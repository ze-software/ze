package msslab

import (
	"encoding/binary"
	"testing"
)

// VALIDATES: the SYN the lab sends parses back to the option it carries, with
// valid IPv4 and TCP checksums, for both families and for a SYN with no MSS.
// PREVENTS: a lab whose own framing is wrong, which would read as a kernel or
// lowering defect in the netns proofs that use it.
// The method: build each frame, check its checksums by summing to zero, then
// parse it with the same port and with another port.
func TestSYNFrameRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		family Family
		sent   Option
		header int
	}{
		{"ipv4 mss 1460", FamilyIPv4, Option{Present: true, Value: 1460}, 24},
		{"ipv4 no mss", FamilyIPv4, Option{}, 20},
		{"ipv6 mss 1300", FamilyIPv6, Option{Present: true, Value: 1300}, 24},
		{"ipv6 no mss", FamilyIPv6, Option{}, 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			frame, err := synFrame(c.family, c.sent, 40001)
			if err != nil {
				t.Fatalf("synFrame: %v", err)
			}
			ipLen := ipv4HeaderLen
			if c.family == FamilyIPv6 {
				ipLen = ipv6HeaderLen
			}
			ip := frame[ethHeaderLen : ethHeaderLen+ipLen]
			tcp := frame[ethHeaderLen+ipLen:]
			if c.family == FamilyIPv4 {
				if got := checksum(0, ip); got != 0 {
					t.Errorf("IPv4 header checksum does not verify: residue %#04x", got)
				}
			}
			var pseudo uint32
			if c.family == FamilyIPv4 {
				pseudo = sum(sum(0, sourceV4[:]), targetV4[:])
			} else {
				pseudo = sum(sum(0, sourceV6[:]), targetV6[:])
			}
			pseudo += protoTCP + uint32(len(tcp))
			if got := checksum(pseudo, tcp); got != 0 {
				t.Errorf("TCP checksum does not verify: residue %#04x", got)
			}
			if got := binary.BigEndian.Uint16(tcp[0:2]); got != 40001 {
				t.Errorf("source port %d, want 40001", got)
			}
			seen, ok := parseSYN(frame, c.family, 40001)
			if !ok {
				t.Fatal("parseSYN did not match the frame it was built from")
			}
			if seen.MSS != c.sent {
				t.Errorf("parsed MSS %+v, want %+v", seen.MSS, c.sent)
			}
			if seen.HeaderOctets != c.header {
				t.Errorf("TCP header %d octets, want %d", seen.HeaderOctets, c.header)
			}
			if _, ok := parseSYN(frame, c.family, 40002); ok {
				t.Error("parseSYN matched a frame from another source port")
			}
		})
	}
}

// VALIDATES: the option walk skips NOPs and other options before the MSS, and
// answers absent for a truncated or zero-length option rather than looping.
func TestMSSOptionWalk(t *testing.T) {
	cases := []struct {
		name    string
		options []byte
		want    Option
	}{
		{"nop then mss", []byte{1, 1, 2, 4, 0x05, 0x78}, Option{Present: true, Value: 1400}},
		{"sack-permitted then mss", []byte{4, 2, 2, 4, 0x05, 0xb4}, Option{Present: true, Value: 1460}},
		{"end before mss", []byte{0, 0, 2, 4, 0x05, 0xb4}, Option{}},
		{"zero length", []byte{8, 0, 2, 4, 0x05, 0xb4}, Option{}},
		{"truncated mss", []byte{2, 4, 0x05}, Option{}},
		{"none", nil, Option{}},
	}
	for _, c := range cases {
		if got := mssOption(c.options); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}
