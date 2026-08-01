// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- responder COOKIE challenge
// RFC: rfc/short/rfc7296.md -- COOKIE (Section 2.6)
// Related: doctor.go -- the sibling ike readiness checks and their registration
// Related: cookie.go -- cookieRequired and the count this check reports against

package engine

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// checkIPsecCookieThreshold reports a cookie-threshold the responder can never reach.
//
// cookieRequired challenges an inbound initiation once halfOpenResponderCount meets the
// threshold. That count is DERIVED from responderBusy, and responderBusy is one bool per
// peer session. The count can never exceed the number of peers configured to respond.
//
// A threshold above that number is therefore never met. cookieRequired stays false, and
// the challenge RFC 7296 Section 2.6 offers against state exhaustion is off.
//
// It is silent, and that is what makes it worth a check. The operator raised a defense
// and the config committed. Nothing in ze's own logs says the value cannot be reached
// (ai/rules/fail-closed-guards.md: a guard that neither denies nor speaks does not exist).
//
// It is a WARNING rather than a commit-time rejection, because the value is not wrong in
// itself. An operator who is about to add peers has written a threshold that will start
// working when they do. Only the CURRENT peer set makes it inert, and that changes
// without the leaf changing.
func checkIPsecCookieThreshold(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil || cfg == nil {
		// checkIPsecInterface already reports an unparseable vpn ipsec section, with the
		// same tree and in the same phase. Reporting it twice would make one config error
		// read as two unrelated faults.
		return nil
	}
	// Zero is the default and challenges every initiation, so it is reachable by
	// definition and needs no peer at all.
	if cfg.CookieThreshold == 0 {
		return nil
	}
	reachable := respondingPeerCount(cfg)
	if uint64(cfg.CookieThreshold) <= uint64(reachable) {
		return nil
	}

	var tb textbuf.Buffer
	tb.Str("ipsec cookie-threshold is ").Uint32(cfg.CookieThreshold).
		Str(" and at most ").Int(int64(reachable)).
		Str(" half-open handshakes can exist at once, because each of the ").
		Int(int64(reachable)).
		Str(" peers that accept an inbound initiation holds one slot. The count never " +
			"reaches the threshold, so no inbound IKE_SA_INIT is ever challenged and the " +
			"COOKIE defense of RFC 7296 Section 2.6 is off. Lower cookie-threshold to ").
		Int(int64(reachable)).
		Str(" or below, or leave it at 0 to challenge every initiation.")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ipsec-cookie-threshold",
		Severity: "warning",
		Message:  tb.String(),
	}}
}

// respondingPeerCount counts the peers that can take a half-open responder slot. It reads
// the same ConnectionType the reconciler reads, so the ceiling reported here is the one
// halfOpenResponderCount actually has.
func respondingPeerCount(cfg *ipsec.IPsecConfig) int {
	n := 0
	for name := range cfg.Peers {
		if cfg.Peers[name].ConnectionType == ipsec.ConnectionRespond {
			n++
		}
	}
	return n
}
