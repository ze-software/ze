// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- bounded peer-option negotiation.
// Related: rfc1661_max_failure_test.go -- default budget and Ack renewal.

package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestClientLCPMaxFailureRefusesMissingBudget checks refusal before any wire
// write, with an operator error naming the actual client and a usable repair.
func TestClientLCPMaxFailureRefusesMissingBudget(t *testing.T) {
	for _, maximum := range []int{0, 256} {
		w := &timedLCPWriter{}
		_, err := negotiateLCP(w, testFrameReader(t, make(chan readFrame), nil), make([]byte, ppp.MaxFrameBufLen), clientMaxFailureConfig(maximum), clientMagic, nil, make(chan struct{}), slog.Default(), new(uint8))
		var refusal *opererr.Error
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want operator refusal", err)
		}
		if refusal.Fix() != "set interface pppoe-client pppoe0 lcp max-failure 5" {
			t.Fatalf("repair = %q", refusal.Fix())
		}
		w.mu.Lock()
		count := len(w.frames)
		w.mu.Unlock()
		if count != 0 {
			t.Fatalf("invalid budget wrote %d frames", count)
		}
	}
}

// TestClientLCPMaxFailureRejectsOriginalInstances covers the normal,
// malformed and authentication-specific Nak paths after a one-Nak budget.
func TestClientLCPMaxFailureRejectsOriginalInstances(t *testing.T) {
	badMRU := []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0}
	goodMRU := []byte{ppp.LCPOptMRU, 4, 0x05, 0xd4}
	for _, tc := range []struct {
		name              string
		request, rejected []byte
	}{
		{"bad-first", append(bytes.Clone(badMRU), goodMRU...), badMRU},
		{"malformed-after-acceptable", append(bytes.Clone(goodMRU), ppp.LCPOptMRU, 1), []byte{ppp.LCPOptMRU, 1}},
		{"magic-zero", []byte{ppp.LCPOptMagic, 6, 0, 0, 0, 0}, []byte{ppp.LCPOptMagic, 6, 0, 0, 0, 0}},
		{"unsupported-auth", []byte{ppp.LCPOptAuthProto, 5, 0xc2, 0x23, 0x81}, []byte{ppp.LCPOptAuthProto, 5, 0xc2, 0x23, 0x81}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := &timedLCPWriter{}
				frames := make(chan readFrame, 1)
				stop := make(chan struct{})
				done := startConfiguredLCP(t, w, frames, stop, clientMaxFailureConfig(1))
				defer func() { close(stop); <-done }()
				synctest.Wait()
				for sent := range 2 {
					id := uint8(30 + sent)
					frames <- serverFrame(ppp.LCPConfigureRequest, id, tc.request)
					synctest.Wait()
					reply := clientMaxFailureReply(t, w, id)
					if sent == 0 {
						if reply.Code != ppp.LCPConfigureNak {
							t.Fatalf("first Code = %d, want Nak", reply.Code)
						}
					} else {
						if reply.Code != ppp.LCPConfigureReject {
							t.Fatalf("second Code = %d, want Reject", reply.Code)
						}
						if !bytes.Equal(reply.Data, tc.rejected) {
							t.Fatalf("Reject = %x, want original instance %x", reply.Data, tc.rejected)
						}
					}
				}
			})
		})
	}
}

// TestClientLCPMaxFailurePeerAckDoesNotRenewBudget distinguishes receiving an
// Ack from sending one: RCA renews Max-Configure, not our sent-Nak budget.
func TestClientLCPMaxFailurePeerAckDoesNotRenewBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		done := startConfiguredLCP(t, w, frames, stop, clientMaxFailureConfig(1))
		defer func() { close(stop); <-done }()
		synctest.Wait()
		requests, _ := w.requests()
		if len(requests) != 1 {
			t.Fatalf("initial requests = %d, want one", len(requests))
		}
		bad := []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0}
		frames <- serverFrame(ppp.LCPConfigureRequest, 20, bad)
		synctest.Wait()
		if reply := clientMaxFailureReply(t, w, 20); reply.Code != ppp.LCPConfigureNak {
			t.Fatalf("first reply = %d, want Nak", reply.Code)
		}
		request := requests[0]
		frames <- serverFrame(ppp.LCPConfigureAck, request[3], bytes.Clone(request[6:]))
		synctest.Wait()
		frames <- serverFrame(ppp.LCPConfigureRequest, 21, bad)
		synctest.Wait()
		if reply := clientMaxFailureReply(t, w, 21); reply.Code != ppp.LCPConfigureReject {
			t.Fatalf("reply after receiving Ack = %d, want exhausted-budget Reject", reply.Code)
		}
	})
}

// TestClientLCPMaxFailureKeepsAcceptableFirstAuth proves an unsupported
// duplicate cannot replace the first authentication method the policy accepted.
func TestClientLCPMaxFailureKeepsAcceptableFirstAuth(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		done := startConfiguredLCP(t, w, frames, stop, clientMaxFailureConfig(1))
		defer func() { close(stop); <-done }()
		synctest.Wait()
		bad := []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0}
		frames <- serverFrame(ppp.LCPConfigureRequest, 20, bad)
		synctest.Wait()
		options := []byte{ppp.LCPOptAuthProto, 4, 0xc0, 0x23, ppp.LCPOptAuthProto, 5, 0xc2, 0x23, 0x81}
		frames <- serverFrame(ppp.LCPConfigureRequest, 21, options)
		synctest.Wait()
		reply := clientMaxFailureReply(t, w, 21)
		if reply.Code != ppp.LCPConfigureAck {
			t.Fatalf("first supported Auth-Protocol received Code %d, want Ack", reply.Code)
		}
		if !bytes.Equal(reply.Data, options) {
			t.Fatalf("Ack options = %x, want unchanged %x", reply.Data, options)
		}
	})
}

// TestClientLCPMaxFailureDoesNotSpendSuppressedReplies uses a ten-byte reply
// buffer: a Magic Nak cannot fit, but an MRU Nak can. Only the latter spends.
func TestClientLCPMaxFailureDoesNotSpendSuppressedReplies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		done := make(chan error, 1)
		reader := testFrameReader(t, frames, nil)
		go func() {
			_, err := negotiateLCP(w, reader, make([]byte, 10), clientMaxFailureConfig(1), clientMagic, nil, stop, slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
			done <- err
		}()
		defer func() { close(stop); <-done }()
		synctest.Wait()
		for id := uint8(20); id < 22; id++ {
			frames <- serverFrame(ppp.LCPConfigureRequest, id, []byte{ppp.LCPOptMagic, 6, 0, 0, 0, 0})
			synctest.Wait()
			w.mu.Lock()
			count := len(w.frames)
			w.mu.Unlock()
			if count != 1 {
				t.Fatalf("oversized response wrote %d frames, want initial request only", count)
			}
		}
		for id := uint8(22); id < 24; id++ {
			frames <- serverFrame(ppp.LCPConfigureRequest, id, []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0})
			synctest.Wait()
			reply := clientMaxFailureReply(t, w, id)
			want := ppp.LCPConfigureNak
			if id == 23 {
				want = ppp.LCPConfigureReject
			}
			if reply.Code != want {
				t.Fatalf("reply %d = %d, want %d", id, reply.Code, want)
			}
		}
	})
}
