// Design: docs/architecture/bgp/structural-forwarding.md -- withdrawal refusal and ownership.
// Related: forward_build.go -- fwdWithdrawal.of preserves conversion errors.
package reactor

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestForwardWithdrawalRefusalCarriesCause proves the adapter preserves each
// builder refusal class without adopting a buffer or caching a failed wire.
func TestForwardWithdrawalRefusalCarriesCause(t *testing.T) {
	labeled := labeledWithdrawPayload(labeledWithdrawFamilies[0],
		labeledAnnounce(nil, 1, nil, 24, []byte{10, 2, 3}), nil)
	twoFamilies := withdrawMergePayload(nil,
		append(withdrawMergeUnreach(1, 128, []byte{88, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 24, 10, 0, 1}),
			withdrawMergeReach(mergeV6New)...), nil)
	for _, current := range []struct {
		name    string
		payload []byte
		failure error
	}{
		{name: "unreadable layout", payload: truncatedAttrPayload(), failure: errWithdrawalLayout},
		{name: "separate families", payload: twoFamilies, failure: errWithdrawalFamilies},
		{name: "unknown labeled framing", payload: labeled, failure: errWithdrawalEncoding},
	} {
		t.Run(current.name, func(t *testing.T) {
			original := bytes.Clone(current.payload)
			base := wireu.NewWireUpdate(current.payload, 0)
			var update ReceivedUpdate
			t.Cleanup(update.returnFwdHandles)
			var withdrawals fwdWithdrawal
			_, beforeStd := bufMuxStd.Stats()
			_, beforeExt := bufMuxExt.Stats()
			got, err := withdrawals.of(&update, base)
			if !errors.Is(err, current.failure) {
				t.Errorf("conversion error=%v, want %v", err, current.failure)
			}
			if got != nil {
				t.Error("refused conversion returned a wire")
			}
			if withdrawals.base != nil {
				t.Error("refused conversion cached its source")
			}
			if withdrawals.wire != nil {
				t.Error("refused conversion cached a wire")
			}
			if len(update.fwdHandles) != 0 {
				t.Errorf("refused conversion adopted %d buffers", len(update.fwdHandles))
			}
			_, afterStd := bufMuxStd.Stats()
			if afterStd != beforeStd {
				t.Errorf("standard buffers held=%d, want %d", afterStd, beforeStd)
			}
			_, afterExt := bufMuxExt.Stats()
			if afterExt != beforeExt {
				t.Errorf("extended buffers held=%d, want %d", afterExt, beforeExt)
			}
			if !bytes.Equal(base.Payload(), original) {
				t.Error("refused conversion changed its source bytes")
			}
		})
	}
}

// TestForwardWithdrawalCacheSurvivesRefusal checks actual withdrawal bytes and
// source identity, one adoption for repeated use, and reuse after a failed base.
func TestForwardWithdrawalCacheSurvivesRefusal(t *testing.T) {
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	payload := withdrawMergePayload(mergeV4Old, makeAttr(0x40, 1, []byte{0}), mergeV4New)
	original := bytes.Clone(payload)
	base := wireu.NewWireUpdate(payload, ctxID)
	base.SetSourceID(42)
	var update ReceivedUpdate
	t.Cleanup(update.returnFwdHandles)
	var withdrawals fwdWithdrawal
	got, err := withdrawals.of(&update, base)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("successful conversion returned no wire")
	}
	want := []byte{0, 8, 24, 10, 0, 1, 24, 10, 0, 2, 0, 0}
	if !bytes.Equal(got.Payload(), want) {
		t.Fatalf("withdrawal=%x, want %x", got.Payload(), want)
	}
	if got.SourceCtxID() != ctxID {
		t.Errorf("source context=%d, want %d", got.SourceCtxID(), ctxID)
	}
	if got.SourceID() != 42 {
		t.Errorf("source identity=%d, want 42", got.SourceID())
	}
	for attempt := range 2 {
		if attempt == 1 {
			bad := wireu.NewWireUpdate(truncatedAttrPayload(), ctxID)
			refused, err := withdrawals.of(&update, bad)
			if !errors.Is(err, errWithdrawalLayout) {
				t.Fatalf("refused conversion error=%v, want %v", err, errWithdrawalLayout)
			}
			if refused != nil {
				t.Fatal("refused conversion returned a wire")
			}
		}
		cached, err := withdrawals.of(&update, base)
		if err != nil {
			t.Fatal(err)
		}
		if cached != got {
			t.Fatal("same base did not reuse its cached withdrawal")
		}
		if len(update.fwdHandles) != 1 {
			t.Errorf("cached conversion adopted %d buffers, want 1", len(update.fwdHandles))
		}
		if !bytes.Equal(cached.Payload(), want) {
			t.Errorf("cached withdrawal=%x, want %x", cached.Payload(), want)
		}
	}
	if !bytes.Equal(base.Payload(), original) {
		t.Error("successful conversion changed its source bytes")
	}
}
