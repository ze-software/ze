// Design: docs/architecture/bgp/structural-forwarding.md -- opaque treatment ownership.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// BenchmarkForwardOpaqueNormalization measures the treatment of shared input and
// an already-owned materialization, including resetting the owned input between
// iterations. The payload must change: one opaque attribute needs Partial and
// another must be removed. Neither case can measure an identity fast path.
func BenchmarkForwardOpaqueNormalization(b *testing.B) {
	attrs := append(makeAttr(0xc0, 241, bytes.Repeat([]byte{0x55}, 200)), makeAttr(0x80, 242, []byte{0x66})...)
	payload := buildUpdatePayload(attrs, []byte{24, 203, 0, 115})
	for _, owned := range []bool{false, true} {
		name := "shared"
		if owned {
			name = "owned"
		}
		b.Run(name, func(b *testing.B) {
			input := bytes.Clone(payload)
			base := wireu.NewWireUpdate(input, 0)
			var owner ReceivedUpdate
			if !owned {
				var warm fwdParseCache
				// RFC 4271 Section 5: warm the reusable pool outside measurement.
				if _, err := warm.forwardWire(base, false, &owner); err != nil {
					b.Fatal(err)
				}
				owner.returnFwdHandles()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if owned {
					copy(input, payload)
				}
				var output []byte
				if owned {
					// RFC 4271 Section 5: unpublished materializations compact in place.
					effective, _, err := forwardOpaquePayload(input, true)
					if err != nil {
						b.Fatal(err)
					}
					output = effective
				} else {
					var cache fwdParseCache
					// RFC 4271 Section 5: shared input retains its received bytes.
					out, err := cache.forwardWire(base, false, &owner)
					if err != nil {
						b.Fatal(err)
					}
					output = out.Payload()
				}
				if len(output) != len(payload)-4 {
					b.Fatal("opaque treatment did not remove the non-transitive attribute")
				}
				owner.returnFwdHandles()
			}
		})
	}
}
