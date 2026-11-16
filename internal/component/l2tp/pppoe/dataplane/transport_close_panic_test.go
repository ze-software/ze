// Design: docs/architecture/l2tp/bng-5-pppoe.md -- retained transport cleanup outcomes
package dataplane

import (
	"errors"
	"testing"
)

// A provider panic supplies no confirmation that the transport closed.
type transportClosePanicOwner struct {
	cause any
	calls int
}

func (owner *transportClosePanicOwner) Close() error {
	owner.calls++
	panic(owner.cause)
}

func observeTransportClose(opened OpenedSession) (failure any, err error) {
	defer func() { failure = recover() }()
	err = opened.CloseTransport()
	return nil, err
}

// Exercise the real admitted owner boundary. A failed first close must stay
// failed across copies/repeated calls, without touching the separate PPP owner.
func TestOpenedSessionTransportPanicCannotBecomeCachedSuccess(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause any
	}{
		{"error payload", errors.New("transport provider cleanup panicked")},
		{"text payload", "transport provider cleanup panicked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames, handle, _ := testSessionResources()
			transport := &transportClosePanicOwner{cause: tc.cause}
			opened, err := NewOpenedSession(frames, handle, transport)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handle.Close(); err != nil {
					t.Error(err)
				}
			})
			failure, first := observeTransportClose(opened)
			if failure != nil {
				t.Errorf("transport provider panic escaped the cleanup boundary: %v", failure)
			}
			if first == nil {
				t.Error("unconfirmed transport cleanup returned success")
			}
			if cause, ok := tc.cause.(error); ok {
				if !errors.Is(first, cause) {
					t.Errorf("first cleanup lost the provider cause: %v", first)
				}
			}
			for range 2 {
				cached := opened.CloseTransport()
				if cached == nil {
					t.Error("a later close invented success after the provider panic")
				}
				//nolint:errorlint // Assert exact cached identity, not just a matching wrapped cause.
				if cached != first {
					t.Error("a later close replaced the original cleanup outcome")
				}
			}
			if transport.calls != 1 {
				t.Fatalf("provider close calls = %d, want one", transport.calls)
			}
			if handle.closes != 0 {
				t.Fatal("transport cleanup closed the separate PPP owner")
			}
		})
	}
}
