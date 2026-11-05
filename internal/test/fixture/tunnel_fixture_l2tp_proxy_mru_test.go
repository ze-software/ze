// Design: docs/guide/l2tp.md -- real proxy-session transmit-cap proof.

package fixture

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestTunnelL2TPPPPData(t *testing.T) {
	control := []byte{9, 7, 0, 4}
	plain := []byte{0, 2, 0, 1, 0, 2, 0xc0, 0x21, 9, 7, 0, 4}
	compressed := []byte{0, 2, 0, 1, 0, 2, 0x21, 0x45, 0, 0, 20}
	addressControl := []byte{0, 2, 0, 1, 0, 2, 0xff, 3, 0xc0, 0x21, 9, 7, 0, 4}
	optional := []byte{0x4a, 2, 0, 0, 0, 1, 0, 2, 0, 3, 0, 4, 0, 3, 0, 0, 0, 0xc0, 0x21, 9, 7, 0, 4}
	binary.BigEndian.PutUint16(optional[2:4], uint16(len(optional)))
	badLength := append([]byte(nil), optional...)
	binary.BigEndian.PutUint16(badLength[2:4], uint16(len(badLength)+1))
	badOffset := append([]byte(nil), optional...)
	binary.BigEndian.PutUint16(badOffset[12:14], 65535)
	for _, tc := range []struct {
		name        string
		packet      []byte
		protocol    uint16
		information []byte
		valid       bool
	}{
		{"accounting LCP", plain, 0xc021, control, true},
		{"compressed IPv4 protocol", compressed, 0x21, []byte{0x45, 0, 0, 20}, true},
		{"PPP address and control", addressControl, 0xc021, control, true},
		{"L2TP optional header fields", optional, 0xc021, control, true},
		{"control is not PPP data", []byte{0x80, 2, 0, 0, 0, 0}, 0, nil, false},
		{"missing protocol", plain[:6], 0, nil, false},
		{"truncated two-octet protocol", plain[:7], 0, nil, false},
		{"truncated address and control", addressControl[:8], 0, nil, false},
		{"declared length exceeds carrier", badLength, 0, nil, false},
		{"offset exceeds carrier", badOffset, 0, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			protocol, information, ok := tunnelL2TPPPPData(tc.packet)
			if ok != tc.valid || protocol != tc.protocol || !bytes.Equal(information, tc.information) {
				t.Fatalf("PPP data = %#x/%x/%t, want %#x/%x/%t", protocol, information, ok, tc.protocol, tc.information, tc.valid)
			}
		})
	}
}
