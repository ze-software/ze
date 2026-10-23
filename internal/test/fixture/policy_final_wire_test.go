// Design: docs/functional-tests.md -- final-policy reload assertions judge echoed wire state.
package fixture

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

const policyFinalAlternateAttrs = "4001010040020E02030000FDEB0000FDEB0000FDEB400304C000020380040400000009C00804FDEB0021"

func policyFinalEchoEvent(action, attrs string) string {
	return fmt.Sprintf(`{"bgp":{"message":{"type":"update","direction":"received"},"peer":{"remote":{"address":"127.0.0.2"}},"raw":{"attributes":%q},"update":{"nlri":{"ipv4/unicast":[{"action":%q,"nlri":["10.72.3.0/24"]}]}}}}`, attrs, action)
}

// VALIDATES: absent-key withdrawals are harmless, received withdrawals remove
// installed state, and restoration requires a new received advertisement.
func TestPolicyFinalWireStateRestoration(t *testing.T) {
	state := &policyFinalWireState{routes: make(map[string]bool)}
	if err := state.receive(policyFinalEchoEvent("del", "")); err != nil {
		t.Fatal(err)
	}
	if state.failure != nil || len(state.routes) != 0 {
		t.Fatalf("absent-key withdrawal: routes=%v failure=%v", state.routes, state.failure)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := state.wait(ctx, "missing alternate", "10.72.3.0/24"); err == nil {
		t.Fatal("withdrawal was accepted as alternate restoration")
	}
	announcement := policyFinalEchoEvent("add", policyFinalAlternateAttrs)
	if err := state.receive(strings.Replace(announcement, `"received"`, `"sent"`, 1)); err != nil {
		t.Fatal(err)
	}
	if len(state.routes) != 0 {
		t.Fatal("daemon sent event was accepted as receiver proof")
	}
	if err := state.receive(announcement); err != nil {
		t.Fatal(err)
	}
	if err := state.wait(t.Context(), "restored alternate", "10.72.3.0/24"); err != nil {
		t.Fatal(err)
	}
	if err := state.receive(policyFinalEchoEvent("del", "")); err != nil {
		t.Fatal(err)
	}
	if err := state.wait(t.Context(), "received withdrawal removed installed alternate"); err != nil {
		t.Fatal(err)
	}
	if err := state.wait(ctx, "withdrawn alternate", "10.72.3.0/24"); err == nil {
		t.Fatal("withdrawn advertisement remained available as restoration proof")
	}
	if err := state.receive(announcement); err != nil {
		t.Fatal(err)
	}
	if err := state.wait(t.Context(), "new advertisement restored alternate", "10.72.3.0/24"); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: every wire announcement retains the original attributes. A later
// good advertisement cannot erase a transient denied-path leak or corruption.
func TestPolicyFinalWireStateRejectsWrongAttributes(t *testing.T) {
	for _, tc := range []struct {
		name string
		from string
		to   string
	}{
		{"denied ASN", "0000FDEB0000FDEB0000FDEB", "0000FDEB0000FDEB0000FDF2"},
		{"next hop", "C0000203", "C0000201"},
		{"MED", "00000009", "00000008"},
		{"community", "FDEB0021", "FDE90063"},
		{"origin missing", "40010100", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &policyFinalWireState{routes: make(map[string]bool)}
			attrs := strings.Replace(policyFinalAlternateAttrs, tc.from, tc.to, 1)
			if err := state.receive(policyFinalEchoEvent("add", attrs)); err != nil {
				t.Fatal(err)
			}
			if state.failure == nil {
				t.Fatal("wrong original attributes were accepted")
			}
			if err := state.receive(policyFinalEchoEvent("add", policyFinalAlternateAttrs)); err != nil {
				t.Fatal(err)
			}
			if err := state.wait(t.Context(), "after corrupt announcement", "10.72.3.0/24"); err == nil {
				t.Fatal("later valid announcement erased the wire violation")
			}
		})
	}
}

// VALIDATES: the live preferred-path probe still forbids withdrawing the
// already-installed alternate, even though restoration allows an absent-key del.
func TestPolicyFinalWireStateProtectsInstalledAlternate(t *testing.T) {
	state := &policyFinalWireState{routes: map[string]bool{"10.72.3.0/24": true}, protectAlternate: true}
	if err := state.receive(policyFinalEchoEvent("del", "")); err != nil {
		t.Fatal(err)
	}
	if state.failure == nil {
		t.Fatal("preferred rejection withdrew the installed alternate without failure")
	}
}
