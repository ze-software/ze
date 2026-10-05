// Design: docs/architecture/bgp/structural-forwarding.md -- opaque treatment ownership.
package reactor

import (
	"bytes"
	"testing"
)

// TestForwardOpaquePayload checks exact bytes for compaction, extended headers,
// identity treatment and a malformed tail, over shared and owned payloads.
// Shared input must remain byte-identical even when later parsing fails.
func TestForwardOpaquePayload(t *testing.T) {
	cases := []struct {
		name      string
		attrs     []byte
		wantAttrs []byte
		malformed bool
	}{
		{
			name:      "drop-before-retained-and-stamped",
			attrs:     []byte{0x80, 242, 1, 0x11, 0x40, 1, 1, 0, 0xc0, 241, 2, 0x22, 0x33},
			wantAttrs: []byte{0x40, 1, 1, 0, 0xe0, 241, 2, 0x22, 0x33},
		},
		{
			name:      "extended-length",
			attrs:     []byte{0xd0, 241, 0, 2, 0x22, 0x33, 0x90, 242, 0, 1, 0x11},
			wantAttrs: []byte{0xf0, 241, 0, 2, 0x22, 0x33},
		},
		{
			name:      "already-normalized",
			attrs:     []byte{0xe0, 241, 2, 0x22, 0x33},
			wantAttrs: []byte{0xe0, 241, 2, 0x22, 0x33},
		},
		{
			name:      "malformed-after-drop",
			attrs:     []byte{0x80, 242, 1, 0x11, 0xc0, 241, 5, 0x22},
			malformed: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, owned := range []bool{false, true} {
				payload := buildUpdatePayload(tc.attrs, []byte{24, 203, 0, 115})
				original := bytes.Clone(payload)
				// RFC 4271 Section 5.
				out, handle, err := forwardOpaquePayload(payload, owned)
				if !owned && !bytes.Equal(payload, original) {
					t.Error("normalization changed shared input")
				}
				if tc.malformed {
					if err == nil {
						t.Error("malformed attribute tail was forwarded")
					}
					returnReadBuffer(handle)
					continue
				}
				if err != nil {
					returnReadBuffer(handle)
					t.Fatal(err)
				}
				if out == nil {
					out = payload
				}
				want := buildUpdatePayload(tc.wantAttrs, []byte{24, 203, 0, 115})
				if !bytes.Equal(out, want) {
					t.Errorf("owned=%v payload=%x, want %x", owned, out, want)
				}
				returnReadBuffer(handle)
			}
		})
	}
}
