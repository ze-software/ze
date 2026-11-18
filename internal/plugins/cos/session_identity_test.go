// Design: docs/architecture/traffic/cos-dynamic.md -- original subscriber identity
//go:build ze_l2tp

package cos

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

func ac2CoSBirth(t *testing.T, text string) subscriberid.L2TPBirth {
	t.Helper()
	birth, err := subscriberid.ParseL2TPBirth(text)
	if err != nil {
		t.Fatal(err)
	}
	return birth
}

func TestCoSPreparedTokenSurvivesForeignBirthDown(t *testing.T) {
	h, _ := setupCoSTransaction(t)
	first := ac2CoSBirth(t, "l2tp-00000000000000000000000000000003")
	foreign := ac2CoSBirth(t, "l2tp-00000000000000000000000000000004")
	peer := netip.MustParseAddr("192.0.2.1")
	prepared, err := h.prepareSessionChange(sessionTransactionRequest{SubscriberID: first.String(), TunnelID: 7, SessionID: 42, Interface: "eth0.42", Profile: "business"}, peer)
	if err != nil {
		t.Fatal(err)
	}
	h.onSessionDown(&l2tpevents.SessionDownPayload{Birth: foreign, SubscriberID: foreign.String(), TunnelID: 7, SessionID: 42})
	if h.pending == nil || h.pending.prepared().token != prepared.Token {
		t.Fatal("foreign equal-label Down invalidated the original transaction")
	}
	if err := h.applySessionChange(prepared.Token, peer); err != nil {
		t.Fatal(err)
	}
	if err := h.commitSessionChange(prepared.Token, peer); err != nil {
		t.Fatal(err)
	}
	if err := h.rollbackSessionToken(prepared.Token, peer); err != nil {
		t.Fatalf("original committed transaction lost compensation: %v", err)
	}
}

func TestCoSPrepareRejectsMissingOrNoncanonicalIdentityBeforeRead(t *testing.T) {
	h, _ := setupCoSTransaction(t)
	reads := 0
	original := h.readLink
	h.readLink = func(name string) (*iface.InterfaceInfo, error) {
		reads++
		return original(name)
	}
	peer := netip.MustParseAddr("192.0.2.1")
	for _, id := range []string{"", "l2tp-7-42", "l2tp-00000000000000000000000000000000"} {
		if _, err := h.prepareSessionChange(sessionTransactionRequest{SubscriberID: id, TunnelID: 7, SessionID: 42, Interface: "eth0.42", Profile: "business"}, peer); err == nil {
			t.Fatalf("invalid carried identity %q admitted", id)
		}
	}
	if reads != 0 {
		t.Fatalf("invalid identity reached native map read %d times", reads)
	}
	valid := ac2CoSBirth(t, "l2tp-00000000000000000000000000000005")
	if _, err := h.prepareSessionChange(sessionTransactionRequest{SubscriberID: valid.String(), TunnelID: 7, SessionID: 42, Interface: "eth0.42", Profile: "business"}, peer); err != nil {
		t.Fatalf("valid original identity refused: %v", err)
	}
	if reads != 1 {
		t.Fatalf("valid prepare read count = %d, want 1", reads)
	}
}

func TestCoSEnricherSelectsCanonicalIDOverNumericLabels(t *testing.T) {
	first := ac2CoSBirth(t, "l2tp-00000000000000000000000000000006")
	second := ac2CoSBirth(t, "l2tp-00000000000000000000000000000007")
	sessionStore.Store(sessionKey{birth: first}, sessionCoSState{profileName: "first"})
	sessionStore.Store(sessionKey{birth: second}, sessionCoSState{profileName: "second"})
	t.Cleanup(func() {
		sessionStore.Delete(sessionKey{birth: first})
		sessionStore.Delete(sessionKey{birth: second})
	})
	base := map[string]any{"id": second.String(), "tunnel-id": uint16(7), "session-id": uint16(42)}
	enrichSubscriberBrief(base)
	if base["cos-profile"] != "second" {
		t.Fatalf("canonical ID selected another CoS owner: %#v", base)
	}
	delete(base, "id")
	delete(base, "cos-profile")
	enrichSubscriberBrief(base)
	if _, found := base["cos-profile"]; found {
		t.Fatal("numeric labels supplied a missing canonical identity")
	}
}
