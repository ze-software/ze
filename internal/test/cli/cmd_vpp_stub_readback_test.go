// Design: docs/functional-tests.md -- VPP stub state readback before monitor startup.
// Related: cmd_vpp_stub.go -- serveVPPClient and acknowledged interface mutations.
// Upstream: https://github.com/FDio/vpp/blob/v26.06/src/vnet/interface.api
// and src/vnet/ip/ip.api; GoVPP carries these requests and replies for Ze.
// Native names: src/vnet/ethernet/interface.c, format_simulated_ethernet_name
// and loopback_instance_alloc; a fresh dynamic loopback is loop0.

package cli

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// TestVPPStubInterfaceDumpReadsCreatedLoopback checks acknowledged creation
// through the same framed dump the interface backend uses to seed its monitor.
func TestVPPStubInterfaceDumpReadsCreatedLoopback(t *testing.T) {
	state := vppStubReadbackState(t)
	created := vppStubReadbackCall(t, state, &interfaces.CreateLoopback{})
	var reply interfaces.CreateLoopbackReply
	vppStubReadbackDecode(t, created, &reply)
	vppStubReadbackCall(t, state, &interfaces.SwInterfaceSetFlags{
		SwIfIndex: reply.SwIfIndex, Flags: interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
	})
	frames := vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)})
	var details interfaces.SwInterfaceDetails
	vppStubReadbackDecode(t, frames, &details)
	if details.SwIfIndex != reply.SwIfIndex {
		t.Fatalf("dump index = %d, want created index %d", details.SwIfIndex, reply.SwIfIndex)
	}
	if details.InterfaceName != "loop0" {
		t.Fatalf("native interface name = %q, want loop0", details.InterfaceName)
	}
	if details.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP == 0 {
		t.Fatal("dump lost acknowledged admin-up state")
	}
}

// TestVPPStubAddressDumpReadsConfirmedAddressBeforeMonitor checks that an
// address installed before any event subscription survives initial readback.
func TestVPPStubAddressDumpReadsConfirmedAddressBeforeMonitor(t *testing.T) {
	state := vppStubReadbackState(t)
	var created interfaces.CreateLoopbackReply
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.CreateLoopback{}), &created)
	prefix := ip_types.AddressWithPrefix{Address: ip_types.Address{Af: ip_types.ADDRESS_IP4}, Len: 24}
	prefix.Address.Un.SetIP4(ip_types.IP4Address{10, 0, 0, 2})
	vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{
		SwIfIndex: created.SwIfIndex, IsAdd: true, Prefix: prefix,
	})
	frames := vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: created.SwIfIndex})
	var details ip.IPAddressDetails
	vppStubReadbackDecode(t, frames, &details)
	if details.SwIfIndex != created.SwIfIndex {
		t.Fatalf("address index = %d, want %d", details.SwIfIndex, created.SwIfIndex)
	}
	if details.Prefix != prefix {
		t.Fatalf("address = %v, want installed %v", details.Prefix, prefix)
	}
	if got := len(vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: created.SwIfIndex, IsIPv6: true})); got != 0 {
		t.Fatalf("IPv6 dump contains %d IPv4 addresses", got)
	}
}

// TestVPPStubInterfaceReadbackFiltersAndRemovesState preserves a second live
// interface while deleting the first, so empty unconditional dumps cannot pass.
func TestVPPStubInterfaceReadbackFiltersAndRemovesState(t *testing.T) {
	state := vppStubReadbackState(t)
	var first, second interfaces.CreateLoopbackReply
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.CreateLoopback{}), &first)
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.CreateLoopback{}), &second)
	prefix := ip_types.AddressWithPrefix{Address: ip_types.Address{Af: ip_types.ADDRESS_IP4}, Len: 24}
	prefix.Address.Un.SetIP4(ip_types.IP4Address{192, 0, 2, 2})
	for _, index := range []interface_types.InterfaceIndex{first.SwIfIndex, second.SwIfIndex} {
		vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{SwIfIndex: index, IsAdd: true, Prefix: prefix})
	}
	filtered := vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{
		SwIfIndex: ^interface_types.InterfaceIndex(0), NameFilterValid: true, NameFilter: "loop1",
	})
	var survivor interfaces.SwInterfaceDetails
	vppStubReadbackDecode(t, filtered, &survivor)
	if survivor.SwIfIndex != second.SwIfIndex {
		t.Fatalf("filtered dump index = %d, want %d", survivor.SwIfIndex, second.SwIfIndex)
	}
	vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{SwIfIndex: first.SwIfIndex, Prefix: prefix})
	if got := len(vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: first.SwIfIndex})); got != 0 {
		t.Fatalf("removed address still appears in %d replies", got)
	}
	var address ip.IPAddressDetails
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: second.SwIfIndex}), &address)
	if address.Prefix != prefix {
		t.Fatalf("surviving address = %v, want %v", address.Prefix, prefix)
	}
	vppStubReadbackCall(t, state, &interfaces.DeleteLoopback{SwIfIndex: first.SwIfIndex})
	if got := len(vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: first.SwIfIndex})); got != 0 {
		t.Fatalf("deleted interface still appears in %d replies", got)
	}
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)}), &survivor)
	if survivor.SwIfIndex != second.SwIfIndex {
		t.Fatalf("surviving interface = %d, want %d", survivor.SwIfIndex, second.SwIfIndex)
	}
}

func vppStubReadbackState(t *testing.T) *vppStubState {
	t.Helper()
	state, err := newVPPStubState(filepath.Join(t.TempDir(), "stub.jsonl"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.log.Close() }) //nolint:errcheck // test owns the log.
	return state
}

// vppStubReadbackCall uses the production frame dispatcher synchronously.
// Its in-memory connection reaches EOF after the request, so no worker or
// timeout is needed to bound the test.
func vppStubReadbackCall(t *testing.T, state *vppStubState, message api.Message) []vppStubReadbackFrame {
	t.Helper()
	encoded, err := message.(interface{ Marshal([]byte) ([]byte, error) }).Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := state.reply(message.GetMessageName(), 101, encoded)
	if err != nil {
		t.Fatal(err)
	}
	var request bytes.Buffer
	if err := writeVPPFrame(&request, payload); err != nil {
		t.Fatal(err)
	}
	connection := &vppStubReadbackConnection{Reader: bytes.NewReader(request.Bytes())}
	if err := serveVPPClient(state, connection); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&connection.written)
	var frames []vppStubReadbackFrame
	for {
		frame, err := readVPPFrame(reader)
		if err == io.EOF {
			name := message.GetMessageName()
			if name == "sw_interface_dump" {
				return frames
			}
			if name == "ip_address_dump" {
				return frames
			}
			if len(frames) != 1 {
				t.Fatalf("%s acknowledgement count = %d, want 1", name, len(frames))
			}
			if frames[0].name != name+"_reply" {
				t.Fatalf("%s acknowledgement = %s", name, frames[0].name)
			}
			if got := int32(binary.BigEndian.Uint32(frames[0].body)); got != 0 {
				t.Fatalf("%s retval = %d, want success", name, got)
			}
			return frames
		}
		if err != nil {
			t.Fatal(err)
		}
		if got := binary.BigEndian.Uint32(frame[2:6]); got != 101 {
			t.Fatalf("reply context = %d, want 101", got)
		}
		frames = append(frames, vppStubReadbackFrame{name: state.names[binary.BigEndian.Uint16(frame)], body: frame[6:]})
	}
}

type vppStubReadbackFrame struct {
	name string
	body []byte
}

func vppStubReadbackDecode(t *testing.T, frames []vppStubReadbackFrame, message api.Message) {
	t.Helper()
	if len(frames) != 1 {
		t.Fatalf("%s reply count = %d, want 1", message.GetMessageName(), len(frames))
	}
	if frames[0].name != message.GetMessageName() {
		t.Fatalf("reply name = %s, want %s", frames[0].name, message.GetMessageName())
	}
	if err := message.(interface{ Unmarshal([]byte) error }).Unmarshal(frames[0].body); err != nil {
		t.Fatal(err)
	}
}

type vppStubReadbackConnection struct {
	*bytes.Reader
	written bytes.Buffer
}

func (c *vppStubReadbackConnection) Write(p []byte) (int, error) { return c.written.Write(p) }
func (*vppStubReadbackConnection) Close() error                  { return nil }
func (*vppStubReadbackConnection) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "stub", Net: "unix"}
}
func (*vppStubReadbackConnection) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "client", Net: "unix"}
}
func (*vppStubReadbackConnection) SetDeadline(time.Time) error      { return nil }
func (*vppStubReadbackConnection) SetReadDeadline(time.Time) error  { return nil }
func (*vppStubReadbackConnection) SetWriteDeadline(time.Time) error { return nil }
