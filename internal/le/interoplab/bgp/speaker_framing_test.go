// Design: docs/architecture/testing/interop.md -- exact retained wire evidence.
// Related: check_typed_discard_boundary_test.go -- the gated typed-discard wire tests.

package bgp

import (
	"testing"
	"time"
)

func TestSpeakerFramingRejectsInvalidOuterHeader(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"marker", func(frame []byte) { frame[0] = 0 }},
		{"short length", func(frame []byte) { frame[16], frame[17] = 0, 18 }},
		{"KEEPALIVE body", func(frame []byte) { frame[18] = bgpKeepalive }},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame := speakerMessage(bgpUpdate, []byte{0, 0, 0, 0})
			test.mutate(frame)
			conn := &scriptedSpeakerConn{reads: []scriptedSpeakerRead{{data: frame[:19]}, {data: frame[19:]}}}
			if _, _, _, err := readSpeakerMessage(conn, time.Now().Add(time.Second)); err == nil {
				t.Fatal("invalid BGP outer header passed")
			}
		})
	}
}
