// Design: docs/architecture/testing/interop.md -- checker prerequisites require observed peer state.
// Related: checkers.go -- checkESPFormChange rejects a missing Ze XFRM snapshot.
// Related: ipsec_test.go -- scriptedLab and the full form-disagreement traffic control.

package ipsec

import (
	"context"
	"strings"
	"testing"
)

// TestESPFormChangeRejectsMissingZeState drives the actual checker after both
// handshake prerequisites succeed. An empty Ze dump must refuse the proof.
func TestESPFormChangeRejectsMissingZeState(t *testing.T) {
	for _, test := range []struct {
		name  string
		state string
	}{
		{"empty", ""},
		{"whitespace", " \t\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := scriptedLab()
			fake.answer(swanPeer, []string{"swanctl", "--list-sas"},
				"ze: ESTABLISHED\nze-child: INSTALLED\n")
			fake.answer(zePeer, []string{"ip", "xfrm", "state"}, test.state)
			err := checkESPFormChange(context.Background(), checkerFixture(fake))
			if got := fake.indexes[fakeCommandKey(swanPeer, []string{"swanctl", "--list-sas"})]; got != 2 {
				t.Fatalf("strongSwan readiness reads = %d, want 2", got)
			}
			if got := fake.indexes[fakeCommandKey(zePeer, []string{"ip", "xfrm", "state"})]; got != 1 {
				t.Fatalf("Ze state reads = %d, want 1", got)
			}
			if err == nil {
				t.Fatal("form-change checker accepted a missing Ze XFRM state snapshot")
			}
		})
	}
}

// TestESPFormChangeAcceptsPresentZeState reaches the next independent refusal
// with an ESP-in-UDP state, so rejecting every snapshot cannot satisfy the tests.
func TestESPFormChangeAcceptsPresentZeState(t *testing.T) {
	fake := scriptedLab()
	fake.answer(swanPeer, []string{"swanctl", "--list-sas"},
		"ze: ESTABLISHED\nze-child: INSTALLED, TUNNEL-in-UDP\n")
	fake.answer(zePeer, []string{"ip", "xfrm", "state"},
		"src 172.28.0.3 dst 172.28.0.2\n\tproto esp spi 0x2 encap type espinudp sport 4500 dport 4500\n")
	err := checkESPFormChange(context.Background(), checkerFixture(fake))
	if got := fake.indexes[fakeCommandKey(swanPeer, []string{"swanctl", "--list-sas"})]; got != 3 {
		t.Fatalf("strongSwan reads after the Ze state prerequisite = %d, want 3", got)
	}
	if got := fake.indexes[fakeCommandKey(zePeer, []string{"ip", "xfrm", "state"})]; got != 1 {
		t.Fatalf("Ze state reads = %d, want 1", got)
	}
	if err == nil {
		t.Fatal("form-change checker accepted peers using the same ESP form")
	}
	if !strings.Contains(err.Error(), "strongSwan sends UDP-encapsulated ESP; forms agree") {
		t.Fatalf("valid Ze state did not reach the independent form check: %v", err)
	}
}
