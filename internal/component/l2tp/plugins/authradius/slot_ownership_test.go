// Related: register.go -- activateRadiusConfig claims the auth slot
// Related: ../authlocal/register.go -- claims it at init as the base handler
// Related: ../../subscriber/handler_registry.go -- the single-owner slot

// An EXTERNAL test package on purpose. A package-level variable in an internal
// test file is initialized BEFORE the package's own init() runs, so it could
// never observe what init() did. Here both plugins are imported, so both are
// fully initialized before this file's first line runs.
package l2tpauthradius_test

import (
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/l2tp/plugins/authlocal"
	_ "github.com/ze-software/ze/internal/component/l2tp/plugins/authradius"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

// VALIDATES: with RADIUS compiled in but not configured, the auth slot belongs
// to l2tp-auth-local, so a subscriber is judged against the operator's local
// users.
// PREVENTS: the BNG rejecting every authenticated subscriber. The slot holds
// ONE handler; both plugins used to claim it in init(), so the owner was link
// order. RADIUS won, and with no RADIUS server configured it answered every
// CHAP Response "no RADIUS client". PPPoE and L2TP alike could complete LCP and
// then fail auth with a message naming a server the operator never configured.
// The messages tell the two handlers apart: "no users configured" is local's
// (it holds the slot and found no user), "no RADIUS client" is RADIUS's.
func TestUnconfiguredRadiusLeavesTheAuthSlotToLocal(t *testing.T) {
	handler := subscriber.GetAuthHandler()
	if handler == nil {
		t.Fatal("no auth handler registered; l2tp-auth-local must claim the slot at init")
	}

	result := handler(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 1,
		Method:    ppp.AuthMethodCHAPMD5,
		Username:  "alice",
	}, func(bool, string, []byte) error { return nil })

	if strings.Contains(result.Message, "RADIUS") {
		t.Fatalf("auth slot answered %q: an unconfigured RADIUS plugin still owns it, "+
			"so no local credential can ever be checked", result.Message)
	}
	if result.Accept {
		t.Errorf("local auth with no users configured must reject, got accept (%q)", result.Message)
	}
}
