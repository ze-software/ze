// Design: docs/architecture/iface/vlan-qos-map.md -- 802.1Q TCI decode/build helpers

package ifacenetlink

import (
	"encoding/binary"
	"fmt"
	"testing"
)

// IEEE 802.1Q: TPID 0x8100, TCI = PCP(3) | DEI(1) | VID(12).
const ethTPID8021Q = 0x8100

// decodeTCI extracts PCP, DEI, and VID from an Ethernet frame carrying an
// 802.1Q tag. The frame must contain at least dst(6) + src(6) + TPID(2) +
// TCI(2) = 18 bytes, and bytes 12-13 must be 0x8100.
func decodeTCI(frame []byte) (pcp, dei uint8, vid uint16, err error) {
	if len(frame) < 18 {
		return 0, 0, 0, fmt.Errorf("frame too short for 802.1Q: %d bytes, need 18", len(frame))
	}
	tpid := binary.BigEndian.Uint16(frame[12:14])
	if tpid != ethTPID8021Q {
		return 0, 0, 0, fmt.Errorf("not 802.1Q: TPID=0x%04x, want 0x%04x", tpid, ethTPID8021Q)
	}
	tci := binary.BigEndian.Uint16(frame[14:16])
	pcp = uint8(tci >> 13)
	dei = uint8((tci >> 12) & 1)
	vid = tci & 0x0FFF
	return pcp, dei, vid, nil
}

// buildTaggedFrame crafts an Ethernet frame with an 802.1Q tag.
// Layout: dst(6) | src(6) | TPID(2) | TCI(2) | innerEtherType(2) | payload.
func buildTaggedFrame(dst, src [6]byte, vid uint16, pcp uint8, innerEtherType uint16, payload []byte) []byte { //nolint:unparam // EtherType is 0x0800 in current tests; kept generic for IPv6 and future use
	frame := make([]byte, 0, 18+len(payload))
	frame = append(frame, dst[:]...)
	frame = append(frame, src[:]...)
	frame = binary.BigEndian.AppendUint16(frame, ethTPID8021Q)
	tci := uint16(pcp)<<13 | (vid & 0x0FFF)
	frame = binary.BigEndian.AppendUint16(frame, tci)
	frame = binary.BigEndian.AppendUint16(frame, innerEtherType)
	frame = append(frame, payload...)
	return frame
}

// VALIDATES: spec-vlan-qos-lab phase 1 -- PCP extraction from raw 802.1Q bytes
// is correct for all valid PCP values (0-7), rejects non-tagged and truncated frames.
// PREVENTS: bit-shift error in TCI decode silently passing positive-only assertions.
func TestDecodeTCI(t *testing.T) {
	tests := []struct {
		name    string
		frame   []byte
		wantPCP uint8
		wantDEI uint8
		wantVID uint16
		wantErr bool
	}{
		{
			name: "PCP6_VID100",
			frame: buildTaggedFrame(
				[6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
				[6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
				100, 6, 0x0800, []byte("test")),
			wantPCP: 6, wantVID: 100,
		},
		{
			name:    "PCP0_VID1",
			frame:   buildTaggedFrame([6]byte{}, [6]byte{}, 1, 0, 0x0800, nil),
			wantPCP: 0, wantVID: 1,
		},
		{
			name:    "PCP7_VID4094",
			frame:   buildTaggedFrame([6]byte{}, [6]byte{}, 4094, 7, 0x0800, nil),
			wantPCP: 7, wantVID: 4094,
		},
		{
			name:    "frame_too_short",
			frame:   []byte{0x00, 0x01, 0x02},
			wantErr: true,
		},
		{
			name:    "not_8021Q",
			frame:   make([]byte, 18),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcp, dei, vid, err := decodeTCI(tt.frame)
			if tt.wantErr {
				if err == nil {
					t.Error("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pcp != tt.wantPCP {
				t.Errorf("PCP = %d, want %d", pcp, tt.wantPCP)
			}
			if dei != tt.wantDEI {
				t.Errorf("DEI = %d, want %d", dei, tt.wantDEI)
			}
			if vid != tt.wantVID {
				t.Errorf("VID = %d, want %d", vid, tt.wantVID)
			}
		})
	}
}

// VALIDATES: spec-vlan-qos-lab phase 1 -- crafted frame round-trips through
// decodeTCI with correct PCP, VID, MACs, EtherType, and payload.
// PREVENTS: frame builder producing malformed 802.1Q that the decoder silently accepts.
func TestBuildTaggedFrame(t *testing.T) {
	dst := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	src := [6]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	payload := []byte("hello")
	frame := buildTaggedFrame(dst, src, 200, 5, 0x0800, payload)

	pcp, _, vid, err := decodeTCI(frame)
	if err != nil {
		t.Fatalf("round-trip decodeTCI: %v", err)
	}
	if pcp != 5 {
		t.Errorf("round-trip PCP = %d, want 5", pcp)
	}
	if vid != 200 {
		t.Errorf("round-trip VID = %d, want 200", vid)
	}

	if frame[0] != 0xaa || frame[6] != 0x11 {
		t.Errorf("MAC headers: dst[0]=%02x src[0]=%02x", frame[0], frame[6])
	}
	etherType := binary.BigEndian.Uint16(frame[16:18])
	if etherType != 0x0800 {
		t.Errorf("EtherType = 0x%04x, want 0x0800", etherType)
	}
	if string(frame[18:23]) != "hello" {
		t.Errorf("payload = %q, want %q", frame[18:23], "hello")
	}
}
