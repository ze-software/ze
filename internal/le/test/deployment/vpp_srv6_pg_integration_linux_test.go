//go:build integration && linux

// Design: docs/architecture/testing/interop.md -- real packet-generator dataplane rail.
// Related: vpp_srv6_wire_integration_linux_test.go -- common independent byte oracle.
// VPP implementation: https://github.com/FDio/vpp/blob/master/src/vnet/pg/cli.c
// Capture writer: https://github.com/FDio/vpp/blob/master/src/vnet/pg/output.c
// RFC 9252 Section 5: "the ingress PE encapsulates the IPv4 or IPv6 customer
// packet in an outer IPv6 header (using H.Encaps or H.Encaps.Red
// flavors specified in [RFC8986]), where the destination address is the
// SRv6 Service SID associated with the related BGP route update."
package testdeployment

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

// Managed VPP deliberately disables unconfigured plugins. Its built-in PG
// interfaces therefore provide actual graph ingress/egress without adding an
// AF_PACKET plugin setting or bypassing the production startup generator.
func vppSRv6PGUnderlay(t *testing.T) {
	t.Helper()
	for _, command := range []string{
		"ip table add 10",
		"ip table add 20",
		"create packet-generator interface pg0 hw-addr " + vppSRv6VPPMAC,
		"create packet-generator interface pg1 hw-addr " + vppSRv6VPPMAC,
		"create packet-generator interface pg2 hw-addr " + vppSRv6VPPMAC,
		"set interface ip table pg0 10",
		"set interface ip table pg1 20",
		"set interface ip address pg0 192.0.2.1/24",
		"set interface ip address pg1 192.0.2.1/24",
		"set interface ip address pg2 " + vppSRv6Source + "/64",
		"set interface state pg0 up",
		"set interface state pg1 up",
		"set interface state pg2 up",
		"set ip neighbor pg2 " + vppSRv6NextHop + " " + vppSRv6HostMAC + " static",
		"ip route add 2001:db8:95::/64 via " + vppSRv6NextHop + " pg2",
		"set sr encaps source addr " + vppSRv6Source,
	} {
		vppSRv6CLI(t, command)
	}
	vppSRv6CLI(t, "show ip fib table 10")
	vppSRv6CLI(t, "show ip fib table 20")
	vppSRv6CLI(t, "show ip6 fib table 0")
}

func vppSRv6PGPacket(t *testing.T, ingress int, destination, wantSID netip.Addr, sequence byte) {
	t.Helper()
	var frame [74]byte
	vppSRv6Frame(frame[:], destination, sequence)
	name := fmt.Sprintf("srv6-%d", sequence)
	input, output := "/run/vpp/"+name+"-in.pcap", "/run/vpp/"+name+"-out.pcap"
	file, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(pcap.WriteFileHeader(file, 65535, pcap.LinkTypeEthernet),
		pcap.WriteRecord(file, time.Now(), frame[:], len(frame)), file.Close()); err != nil {
		t.Fatal(err)
	}
	// Input and output pcaps stay in the harness scratch mount as evidence.
	vppSRv6CLI(t, "packet-generator capture pg2 pcap "+output+" count 64")
	defer vppSRv6CLI(t, "packet-generator capture pg2 disable")
	vppSRv6CLI(t, fmt.Sprintf("packet-generator new { name %s node ethernet-input interface pg%d pcap %s limit 1 }", name, ingress, input))
	defer vppSRv6CLI(t, "packet-generator delete "+name)
	vppSRv6CLI(t, "packet-generator enable-stream "+name)
	vppSRv6PGGenerated(t, name)

	deadline := time.Now().Add(3 * time.Second)
	if !wantSID.IsValid() {
		deadline = time.Now().Add(time.Second)
	}
	for {
		found, err := vppSRv6PGCaptured(output, frame[14:], wantSID)
		if err != nil {
			t.Fatalf("PG ingress pg%d sequence=%d capture=%s: %v", ingress, sequence, output, err)
		}
		if found {
			t.Logf("real PG forwarding: ingress=pg%d sequence=%d outer-dst=%s source=%s no-SRH capture=%s", ingress, sequence, wantSID, vppSRv6Source, output)
			return
		}
		if !time.Now().Before(deadline) {
			if wantSID.IsValid() {
				t.Fatalf("no matching PG egress: ingress=pg%d sequence=%d capture=%s", ingress, sequence, output)
			}
			t.Logf("no forwarded PG datagram after withdrawal: ingress=pg%d generated=1 sequence=%d", ingress, sequence)
			return
		}
		// sleep(poll): the real PG output node writes the capture asynchronously.
		time.Sleep(20 * time.Millisecond)
	}
}

// Requiring one generated input keeps a missing/disabled generator from passing
// a negative forwarding assertion. The stream's own count is independent of
// policy state and of the packet oracle's expected result.
func vppSRv6PGGenerated(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		output := vppSRv6CLIOutput(t, "show packet-generator")
		for line := range strings.SplitSeq(output, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[0] == name {
				count, err := strconv.ParseUint(fields[2], 10, 64)
				if err != nil {
					t.Fatalf("read PG generated count from %q: %v", line, err)
				}
				if count == 1 {
					return
				}
				if count > 1 {
					t.Fatalf("PG generated more than the requested single datagram: %q", line)
				}
			}
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("PG stream %s did not generate its one input: %s", name, output)
		}
		// sleep(poll): observe the actual PG stream generation counter.
		time.Sleep(20 * time.Millisecond)
	}
}

func vppSRv6PGCaptured(path string, inner []byte, wantSID netip.Addr) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil // no packet has reached pg-output yet
	}
	if err != nil {
		return false, err
	}
	reader, err := pcap.NewReader(bytes.NewReader(data))
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return false, nil // the output node has not finished its header write
	}
	if err != nil {
		return false, err
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return false, fmt.Errorf("PG capture link type %d is not Ethernet", reader.LinkType())
	}
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return false, nil // a later output write may append the next record
			}
			return false, err
		}
		if !bytes.Contains(record.Data, inner[28:]) {
			continue // unrelated control traffic is not this unique datagram
		}
		if !wantSID.IsValid() {
			return false, fmt.Errorf("withdrawn route still forwarded datagram: %x", record.Data)
		}
		if err := vppSRv6PacketMatches(record.Data, inner, wantSID); err != nil {
			return false, fmt.Errorf("real PG output %x: %w", record.Data, err)
		}
		return true, nil
	}
}
