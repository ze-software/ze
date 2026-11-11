// Design: docs/architecture/bgp/structural-forwarding.md -- explicit withdrawal refusal.
// Related: forward_build.go -- buildWithdrawalPayload returns bytes or a failure.
package reactor

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestBuildWithdrawalPayloadExplicitOutcomes pairs the useful byte result
// with each refusal class. A malformed layout or unframeable conversion must
// never return the same nil error as a real withdrawal.
func TestBuildWithdrawalPayloadExplicitOutcomes(t *testing.T) {
	legacy := withdrawMergePayload(mergeV4Old, makeAttr(0x40, 1, []byte{0}), mergeV4New)
	labeled := labeledWithdrawPayload(labeledWithdrawFamilies[0],
		labeledAnnounce(nil, 1, nil, 24, []byte{10, 2, 3}), nil)
	malformedLabeled := labeledWithdrawPayload(labeledWithdrawFamilies[0],
		[]byte{56, 0, 0x10, 0x40, 0, 0x11, 0x40, 10}, nil)
	twoFamilies := withdrawMergePayload(nil,
		append(withdrawMergeUnreach(1, 128, []byte{88, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 24, 10, 0, 1}),
			withdrawMergeReach(mergeV6New)...), nil)
	source := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{}, bgpctx.DirectionRecv)
	for _, current := range []struct {
		name    string
		payload []byte
		limit   int
		source  *bgpctx.EncodingContext
		want    []byte
		failure error
	}{
		{name: "legacy output", payload: legacy,
			want: []byte{0, 8, 24, 10, 0, 1, 24, 10, 0, 2, 0, 0}},
		{name: "labeled output", payload: labeled, source: source,
			want: []byte{0, 0, 0, 13, 0x80, 15, 10, 0, 1, 4, 48, 0x80, 0, 0, 10, 2, 3}},
		{name: "truncated layout", payload: truncatedAttrPayload(), failure: errWithdrawalLayout},
		{name: "empty layout", payload: []byte{0, 0, 0, 0}, failure: errWithdrawalLayout},
		{name: "separate families", payload: twoFamilies, failure: errWithdrawalFamilies},
		{name: "small buffer", payload: legacy, limit: 4, failure: errWithdrawalEncoding},
		{name: "unknown source framing", payload: labeled, failure: errWithdrawalEncoding},
		{name: "malformed labeled framing", payload: malformedLabeled, source: source, failure: errWithdrawalEncoding},
	} {
		t.Run(current.name, func(t *testing.T) {
			original := bytes.Clone(current.payload)
			limit := len(current.payload) + 4
			if current.limit != 0 {
				limit = current.limit
			}
			buf := make([]byte, limit)
			n, err := buildWithdrawalPayload(current.payload, buf, current.source)
			if !bytes.Equal(current.payload, original) {
				t.Error("withdrawal conversion changed its source bytes")
			}
			if current.failure != nil {
				if !errors.Is(err, current.failure) {
					t.Errorf("conversion error=%v, want %v", err, current.failure)
				}
				if n != 0 {
					t.Errorf("refused conversion returned %d output bytes", n)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid withdrawal was refused: %v", err)
			}
			if n != len(current.want) {
				t.Fatalf("conversion returned %d bytes, want %d", n, len(current.want))
			}
			if !bytes.Equal(buf[:n], current.want) {
				t.Errorf("withdrawal=%x, want %x", buf[:n], current.want)
			}
		})
	}
}
