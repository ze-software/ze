// Design: docs/architecture/l2tp/bng-5-pppoe.md -- confirmed native rate ownership
// VALIDATES: cancellation before publication creates no pending native owner.
package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"
)

// A canceled request has no authority to reserve the owner's only transaction
// slot. A later live preparation must remain possible without a cleanup guess.
func TestVPPPolicerCanceledPreparationRetainsNoTransaction(t *testing.T) {
	owner, wire := rateOwnerFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	change, err := owner.Prepare(ctx, 8000, 16000, 1500, 1500)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled preparation = %v", err)
	}
	if change != nil {
		t.Fatal("canceled request received a prepared capability")
	}
	if owner.pending != nil {
		t.Fatal("canceled request reserved the pending slot")
	}
	if len(wire.calls) != 0 {
		t.Fatalf("canceled preparation reached native state: %v", wire.calls)
	}
	if _, err := owner.Prepare(t.Context(), 8000, 16000, 1500, 1500); err != nil {
		t.Fatalf("later live preparation failed: %v", err)
	}
}

// Cancellation and stream-open failures happen before SendMsg can run. They
// must remain errors without inventing an uncertain native allocation.
func TestVPPPolicerBeforeSendFailureRetainsNoNativeOwnership(t *testing.T) {
	for _, failure := range []string{"canceled", "open-failed", "nil-stream"} {
		t.Run(failure, func(t *testing.T) {
			ctx := t.Context()
			want := errors.New("stream unavailable")
			if failure == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			}
			client := &binaryAPI{open: func(context.Context) (api.Stream, error) {
				if failure == "canceled" {
					t.Fatal("canceled request opened a stream")
				}
				if failure == "nil-stream" {
					return nil, nil //nolint:nilnil // Deliberately invalid opener result; the test proves a nil stream without an error is refused.
				}
				return nil, want
			}}
			plan, err := newPolicerPlan("ze-pppoe/test/rx", 8000, 1500)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := client.createPolicer(ctx, plan)
			if err == nil {
				t.Fatal("failed preparation was accepted")
			}
			if failure != "nil-stream" {
				if !errors.Is(err, want) {
					t.Fatalf("failure identity = %v", err)
				}
			}
			if receipt.confirmed {
				t.Fatal("unsubmitted request was confirmed")
			}
			if receipt.uncertain {
				t.Fatal("unsubmitted request reserved an uncertain native allocation")
			}
		})
	}
}

// A send failure may occur after partial transmission. It must retain the
// opposite outcome from a pre-send refusal, even when both return no reply.
func TestVPPPolicerSendFailureStillRetainsUnknownOwnership(t *testing.T) {
	want := errors.New("send completion lost")
	stream := &rateSendFailureStream{testStream: &testStream{}, failure: want}
	client := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		stream.ctx = ctx
		return stream, nil
	}}
	plan, err := newPolicerPlan("ze-pppoe/test/rx", 8000, 1500)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := client.createPolicer(t.Context(), plan)
	if !errors.Is(err, want) {
		t.Fatalf("send error = %v", err)
	}
	if !receipt.uncertain {
		t.Fatal("partially sent request lost its native ownership obligation")
	}
	if receipt.confirmed {
		t.Fatal("unacknowledged request was confirmed")
	}
	if len(stream.sent) != 1 {
		t.Fatalf("send attempts = %d", len(stream.sent))
	}
	if stream.closed != 1 {
		t.Fatalf("stream closes = %d", stream.closed)
	}
}

type rateSendFailureStream struct {
	*testStream
	failure error
}

func (s *rateSendFailureStream) SendMsg(message api.Message) error {
	s.sent = append(s.sent, message)
	return s.failure
}
