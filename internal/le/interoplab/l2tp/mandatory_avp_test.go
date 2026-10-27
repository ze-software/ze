// Design: docs/architecture/testing/interop.md -- unknown mandatory AVP wire evidence.
package l2tp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// TestMandatoryAVPEvidence requires the exact injected HELLO and matching
// StopCCN, refusing a wrong result, another tunnel, or an unmodified packet.
func TestMandatoryAVPEvidence(t *testing.T) {
	hello := controlPacket(6, 19, 3, 2)
	mutated := append([]byte(nil), hello...)
	mutated = append(mutated, 0x80, 6, 0, 0, 0xff, 0xfe)
	binary.BigEndian.PutUint16(mutated[2:4], uint16(len(mutated)))
	stop := controlPacket(4, 23, 2, 4)
	stop = append(stop, 0x80, 10, 0, 0, 0, 1, 0, 2, 0, 8)
	stop = append(stop, 0x80, 8, 0, 0, 0, 9, 0, 19)
	binary.BigEndian.PutUint16(stop[2:4], uint16(len(stop)))
	for _, tc := range []struct {
		name      string
		change    func([]byte, []byte)
		wantError bool
	}{
		{name: "exact refusal"},
		{name: "wrong result", change: func(_ []byte, s []byte) { s[27] = 1 }, wantError: true},
		{name: "wrong error", change: func(_ []byte, s []byte) { s[29] = 3 }, wantError: true},
		{name: "other tunnel", change: func(_ []byte, s []byte) { s[37] = 20 }, wantError: true},
		{name: "wrong acknowledgement", change: func(_ []byte, s []byte) { s[11] = 3 }, wantError: true},
		{name: "optional instead", change: func(h []byte, _ []byte) { h[20] = 0 }, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := append([]byte(nil), mutated...)
			s := append([]byte(nil), stop...)
			if tc.change != nil {
				tc.change(h, s)
			}
			text := wireLine(t, "lac", hello, h) + wireLine(t, "ze", s, s)
			_, err := mandatoryAVPRefusal(text)
			if (err != nil) != tc.wantError {
				t.Fatalf("refusal error = %v, want error %v", err, tc.wantError)
			}
		})
	}
	if _, err := mandatoryAVPRefusal(wireLine(t, "lac", hello, mutated)); err == nil {
		t.Fatal("missing StopCCN accepted")
	}
	if _, err := mandatoryAVPRefusal("{}"); err == nil {
		t.Fatal("absent wire evidence accepted")
	}
}

// TestOptionalAVPEvidence proves that the positive control records a genuine
// SCCCN changed only by the optional AVP, not a hand-written success marker.
func TestOptionalAVPEvidence(t *testing.T) {
	original := controlPacket(3, 19, 1, 1)
	forwarded := append([]byte(nil), original...)
	forwarded = append(forwarded, 0, 6, 0, 0, 0xff, 0xfe)
	binary.BigEndian.PutUint16(forwarded[2:4], uint16(len(forwarded)))
	if err := optionalAVPEvidence(wireLine(t, "lac", original, forwarded)); err != nil {
		t.Fatal(err)
	}
	forwarded[9]++
	if err := optionalAVPEvidence(wireLine(t, "lac", original, forwarded)); err == nil {
		t.Fatal("sequence number rewrite accepted")
	}
}

func controlPacket(message, tunnel, ns, nr uint16) []byte {
	packet := make([]byte, 20)
	for index, value := range []uint16{0xc802, 20, tunnel, 0, ns, nr, 0x8008, 0, 0, message} {
		binary.BigEndian.PutUint16(packet[index*2:], value)
	}
	return packet
}

func wireLine(t *testing.T, direction string, original, forwarded []byte) string {
	t.Helper()
	value, err := json.Marshal(avpWireObservation{
		Direction: direction, Original: hex.EncodeToString(original), Forwarded: hex.EncodeToString(forwarded),
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(value)) + "\n"
}
