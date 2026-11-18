// Design: docs/functional-tests.md -- framed VPP inventory stub regressions.
// Related: cmd_vpp_stub_readback_test.go -- original readback tests and helpers.
// Upstream: https://github.com/FDio/vpp/blob/v26.06/src/vnet/interface.api
// and src/vnet/ethernet/interface.c; these tests cover inventory, not a VPP FIB.

package cli

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/memclnt"
)

// TestVPPStubZeroIndexDumpsAll checks both all-interface sentinels against
// two retained links, with an absent positive index as the opposite control.
func TestVPPStubZeroIndexDumpsAll(t *testing.T) {
	state := vppStubReadbackState(t)
	first := vppStubNativeCreate(t, state)
	second := vppStubNativeCreate(t, state)
	for _, index := range []interface_types.InterfaceIndex{0, ^interface_types.InterfaceIndex(0)} {
		frames := vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: index})
		if len(frames) != 2 {
			t.Fatalf("dump index %d returned %d links, want 2", index, len(frames))
		}
		for position, want := range []interface_types.InterfaceIndex{first, second} {
			var details interfaces.SwInterfaceDetails
			vppStubReadbackDecode(t, frames[position:position+1], &details)
			if details.SwIfIndex != want {
				t.Fatalf("dump index %d position %d = %d, want %d", index, position, details.SwIfIndex, want)
			}
		}
	}
	if got := len(vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: second + 100})); got != 0 {
		t.Fatalf("absent positive index returned %d links", got)
	}
}

// TestVPPStubNameFilterCaseOverridesIndex checks native case-insensitive
// substring matching while the conflicting index selects the other link.
func TestVPPStubNameFilterCaseOverridesIndex(t *testing.T) {
	state := vppStubReadbackState(t)
	first := vppStubNativeCreate(t, state)
	second := vppStubNativeCreate(t, state)
	var details interfaces.SwInterfaceDetails
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{
		SwIfIndex: first, NameFilterValid: true, NameFilter: "OOP1",
	}), &details)
	if details.SwIfIndex != second {
		t.Fatalf("case-insensitive substring filter returned %d, want %d", details.SwIfIndex, second)
	}
	if got := len(vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{
		SwIfIndex: first, NameFilterValid: true, NameFilter: "absent",
	})); got != 0 {
		t.Fatalf("unmatched name filter returned %d links", got)
	}
}

// TestVPPStubLoopbackFlagsFollowAdminState checks that the native loopback's
// link flag follows admin-up, and that admin-down clears both flags.
func TestVPPStubLoopbackFlagsFollowAdminState(t *testing.T) {
	state := vppStubReadbackState(t)
	index := vppStubNativeCreate(t, state)
	up := interface_types.IF_STATUS_API_FLAG_ADMIN_UP | interface_types.IF_STATUS_API_FLAG_LINK_UP
	for _, flags := range []interface_types.IfStatusFlags{interface_types.IF_STATUS_API_FLAG_ADMIN_UP, 0} {
		vppStubReadbackCall(t, state, &interfaces.SwInterfaceSetFlags{SwIfIndex: index, Flags: flags})
		var details interfaces.SwInterfaceDetails
		vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: index}), &details)
		want := interface_types.IfStatusFlags(0)
		if flags != 0 {
			want = up
		}
		if details.Flags != want {
			t.Fatalf("admin request %d read back flags %d, want %d", flags, details.Flags, want)
		}
	}
}

// TestVPPStubAddressFamiliesReadBackSeparately installs both families and
// checks the exact one-address result of each family-specific dump.
func TestVPPStubAddressFamiliesReadBackSeparately(t *testing.T) {
	state := vppStubReadbackState(t)
	index := vppStubNativeCreate(t, state)
	ipv4, ipv6 := vppStubNativePrefixes()
	for _, prefix := range []ip_types.AddressWithPrefix{ipv4, ipv6} {
		vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{SwIfIndex: index, IsAdd: true, Prefix: prefix})
	}
	for _, prefix := range []ip_types.AddressWithPrefix{ipv4, ipv6} {
		var details ip.IPAddressDetails
		vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &ip.IPAddressDump{
			SwIfIndex: index, IsIPv6: prefix.Address.Af == ip_types.ADDRESS_IP6,
		}), &details)
		if details.Prefix != prefix {
			t.Fatalf("family %d read back %v, want %v", prefix.Address.Af, details.Prefix, prefix)
		}
		if details.SwIfIndex != index {
			t.Fatalf("family %d index = %d, want %d", prefix.Address.Af, details.SwIfIndex, index)
		}
	}
}

// TestVPPStubDumpThenPingPreservesContexts sends a dump and its terminating
// ping through one connection, for both a populated and an empty dump.
func TestVPPStubDumpThenPingPreservesContexts(t *testing.T) {
	state := vppStubReadbackState(t)
	index := vppStubNativeCreate(t, state)
	for _, selected := range []interface_types.InterfaceIndex{index, index + 100} {
		frames, err := vppStubNativeExchange(t, state,
			vppStubNativeRequest{message: &interfaces.SwInterfaceDump{SwIfIndex: selected}, context: 0x10203040},
			vppStubNativeRequest{message: &memclnt.ControlPing{}, context: 0x50607080},
		)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if selected == index {
			want = 2
		}
		if len(frames) != want {
			t.Fatalf("index %d returned %d frames, want %d", selected, len(frames), want)
		}
		if selected == index {
			if frames[0].name != "sw_interface_details" {
				t.Fatalf("first frame = %s, want sw_interface_details", frames[0].name)
			}
			if frames[0].context != 0x10203040 {
				t.Fatalf("detail context = %x, want 10203040", frames[0].context)
			}
		}
		last := frames[len(frames)-1]
		if last.name != "control_ping_reply" {
			t.Fatalf("last frame = %s, want control_ping_reply", last.name)
		}
		if last.context != 0x50607080 {
			t.Fatalf("ping context = %x, want 50607080", last.context)
		}
		var ping memclnt.ControlPingReply
		if err := ping.Unmarshal(last.body); err != nil {
			t.Fatal(err)
		}
		if ping.Retval != 0 {
			t.Fatalf("ping retval = %d, want success", ping.Retval)
		}
	}
}

// TestVPPStubMalformedInventoryBodiesLeaveStateUntouched checks each generated
// decoder's fixed-size boundary and the dump's advertised string length.
func TestVPPStubMalformedInventoryBodiesLeaveStateUntouched(t *testing.T) {
	badName := make([]byte, 9)
	binary.BigEndian.PutUint32(badName[5:9], 1)
	cases := []struct {
		name    string
		message api.Message
		body    []byte
	}{
		{"create", &interfaces.CreateLoopback{}, make([]byte, 5)},
		{"delete", &interfaces.DeleteLoopback{}, make([]byte, 3)},
		{"flags", &interfaces.SwInterfaceSetFlags{}, make([]byte, 7)},
		{"address", &interfaces.SwInterfaceAddDelAddress{}, make([]byte, 23)},
		{"interfaces", &interfaces.SwInterfaceDump{}, make([]byte, 8)},
		{"addresses", &ip.IPAddressDump{}, make([]byte, 4)},
		{"advertised_name", &interfaces.SwInterfaceDump{}, badName},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			state := vppStubReadbackState(t)
			index := vppStubNativeCreate(t, state)
			prefix, _ := vppStubNativePrefixes()
			vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{SwIfIndex: index, IsAdd: true, Prefix: prefix})
			frames, err := vppStubNativeExchange(t, state, vppStubNativeRequest{message: test.message, context: 99, body: test.body})
			if err == nil {
				t.Fatal("malformed body was accepted")
			}
			if len(frames) != 0 {
				t.Fatalf("malformed body returned %d frames", len(frames))
			}
			var link interfaces.SwInterfaceDetails
			vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: 0}), &link)
			if link.SwIfIndex != index {
				t.Fatalf("surviving link = %d, want %d", link.SwIfIndex, index)
			}
			if link.Flags != 0 {
				t.Fatalf("malformed input changed flags to %d", link.Flags)
			}
			var address ip.IPAddressDetails
			vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: index}), &address)
			if address.Prefix != prefix {
				t.Fatalf("malformed input changed address to %v", address.Prefix)
			}
		})
	}
}

// TestVPPStubMissingMutationIndexRefusesWithoutChangingSurvivor checks the
// native invalid-index retval while the real interface and its address survive.
func TestVPPStubMissingMutationIndexRefusesWithoutChangingSurvivor(t *testing.T) {
	state := vppStubReadbackState(t)
	index := vppStubNativeCreate(t, state)
	prefix, _ := vppStubNativePrefixes()
	vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{SwIfIndex: index, IsAdd: true, Prefix: prefix})
	missing := index + 100
	requests := []api.Message{
		&interfaces.DeleteLoopback{SwIfIndex: missing},
		&interfaces.SwInterfaceSetFlags{SwIfIndex: missing, Flags: interface_types.IF_STATUS_API_FLAG_ADMIN_UP},
		&interfaces.SwInterfaceAddDelAddress{SwIfIndex: missing, IsAdd: true, Prefix: prefix},
	}
	for _, request := range requests {
		frames, err := vppStubNativeExchange(t, state, vppStubNativeRequest{message: request, context: 27})
		if err != nil {
			t.Fatal(err)
		}
		if len(frames) != 1 {
			t.Fatalf("%s returned %d acknowledgements", request.GetMessageName(), len(frames))
		}
		if frames[0].name != request.GetMessageName()+"_reply" {
			t.Fatalf("acknowledgement = %s", frames[0].name)
		}
		if frames[0].context != 27 {
			t.Fatalf("acknowledgement context = %d, want 27", frames[0].context)
		}
		if len(frames[0].body) != 4 {
			t.Fatalf("acknowledgement body length = %d, want 4", len(frames[0].body))
		}
		if got := int32(binary.BigEndian.Uint32(frames[0].body)); got != int32(api.INVALID_SW_IF_INDEX) {
			t.Fatalf("%s retval = %d, want %d", request.GetMessageName(), got, api.INVALID_SW_IF_INDEX)
		}
	}
	var link interfaces.SwInterfaceDetails
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: 0}), &link)
	if link.SwIfIndex != index {
		t.Fatalf("surviving index = %d, want %d", link.SwIfIndex, index)
	}
	if link.Flags != 0 {
		t.Fatalf("missing-index request changed survivor flags to %d", link.Flags)
	}
	var address ip.IPAddressDetails
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: index}), &address)
	if address.Prefix != prefix {
		t.Fatalf("surviving address = %v, want %v", address.Prefix, prefix)
	}
}

// TestVPPStubLoopbackMACReadback checks the native default MAC and an explicit
// MAC through independent created interfaces and their generated detail frames.
func TestVPPStubLoopbackMACReadback(t *testing.T) {
	state := vppStubReadbackState(t)
	first := vppStubNativeCreate(t, state)
	var details interfaces.SwInterfaceDetails
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: first}), &details)
	if details.L2Address != [6]byte{0xde, 0xad, 0, 0, 0, 0} {
		t.Fatalf("default MAC = %v, want de:ad:00:00:00:00", details.L2Address)
	}
	request := interfaces.CreateLoopback{}
	request.MacAddress = [6]byte{2, 0, 0, 0, 0, 99}
	var created interfaces.CreateLoopbackReply
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &request), &created)
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: created.SwIfIndex}), &details)
	if details.L2Address != request.MacAddress {
		t.Fatalf("explicit MAC = %v, want %v", details.L2Address, request.MacAddress)
	}
}

// TestVPPStubRecreatedLoopbackHasNoOldAddresses deletes an addressed interface,
// creates another, then checks it cannot inherit the deleted address state.
func TestVPPStubRecreatedLoopbackHasNoOldAddresses(t *testing.T) {
	state := vppStubReadbackState(t)
	index := vppStubNativeCreate(t, state)
	ipv4, ipv6 := vppStubNativePrefixes()
	for _, prefix := range []ip_types.AddressWithPrefix{ipv4, ipv6} {
		vppStubReadbackCall(t, state, &interfaces.SwInterfaceAddDelAddress{SwIfIndex: index, IsAdd: true, Prefix: prefix})
	}
	vppStubReadbackCall(t, state, &interfaces.DeleteLoopback{SwIfIndex: index})
	recreated := vppStubNativeCreate(t, state)
	var details interfaces.SwInterfaceDetails
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.SwInterfaceDump{SwIfIndex: recreated}), &details)
	if details.InterfaceName != "loop0" {
		t.Fatalf("recreated native name = %q, want first free instance loop0", details.InterfaceName)
	}
	for _, ipv6 := range []bool{false, true} {
		if got := len(vppStubReadbackCall(t, state, &ip.IPAddressDump{SwIfIndex: recreated, IsIPv6: ipv6})); got != 0 {
			t.Fatalf("recreated link ipv6=%t inherited %d addresses", ipv6, got)
		}
	}
}

func vppStubNativeCreate(t *testing.T, state *vppStubState) interface_types.InterfaceIndex {
	t.Helper()
	var created interfaces.CreateLoopbackReply
	vppStubReadbackDecode(t, vppStubReadbackCall(t, state, &interfaces.CreateLoopback{}), &created)
	return created.SwIfIndex
}

func vppStubNativePrefixes() (ip_types.AddressWithPrefix, ip_types.AddressWithPrefix) {
	ipv4 := ip_types.AddressWithPrefix{Address: ip_types.Address{Af: ip_types.ADDRESS_IP4}, Len: 24}
	ipv4.Address.Un.SetIP4(ip_types.IP4Address{192, 0, 2, 2})
	ipv6 := ip_types.AddressWithPrefix{Address: ip_types.Address{Af: ip_types.ADDRESS_IP6}, Len: 64}
	ipv6.Address.Un.SetIP6(ip_types.IP6Address{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2})
	return ipv4, ipv6
}

type vppStubNativeRequest struct {
	message api.Message
	context uint32
	// A non-nil body replaces generated bytes for malformed-input tests.
	body []byte
}

type vppStubNativeFrame struct {
	name    string
	context uint32
	body    []byte
}

// vppStubNativeExchange executes all requests synchronously in one in-memory
// connection, then decodes frames. EOF bounds it; malformed bodies return the
// actual dispatch error without recovering a panic or fabricating a reply.
func vppStubNativeExchange(t *testing.T, state *vppStubState, requests ...vppStubNativeRequest) ([]vppStubNativeFrame, error) {
	t.Helper()
	var input bytes.Buffer
	for _, request := range requests {
		body := request.body
		if body == nil {
			encoded, err := request.message.(interface{ Marshal([]byte) ([]byte, error) }).Marshal(nil)
			if err != nil {
				t.Fatal(err)
			}
			body = encoded
		}
		payload, err := state.reply(request.message.GetMessageName(), request.context, body)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeVPPFrame(&input, payload); err != nil {
			t.Fatal(err)
		}
	}
	connection := &vppStubReadbackConnection{Reader: bytes.NewReader(input.Bytes())}
	dispatchError := serveVPPClient(state, connection)
	reader := bufio.NewReader(&connection.written)
	var frames []vppStubNativeFrame
	for {
		frame, err := readVPPFrame(reader)
		if err == io.EOF {
			return frames, dispatchError
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(frame) < 6 {
			t.Fatalf("short reply frame: %d", len(frame))
		}
		frames = append(frames, vppStubNativeFrame{
			name:    state.names[binary.BigEndian.Uint16(frame)],
			context: binary.BigEndian.Uint32(frame[2:6]), body: frame[6:],
		})
	}
}
