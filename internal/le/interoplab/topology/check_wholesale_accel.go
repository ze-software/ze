// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: register_wholesale_accel.go -- the scenario this checker judges.
// Related: check_access.go -- the access-chain readers reused here.
package topology

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkAccessWholesaleAccel proves access-wholesale-accel-lns
// (spec-home-cpe-12, user story 1): the Ze CPE, already at LCP Opened with
// the realm-relaying Ze LAC, renegotiates LCP when accel-ppp, the LNS, sends
// its own Configure-Request (RFC 1661 Section 4.3, Opened: RCR), answers
// accel-ppp's CHAP re-challenge, and comes up with the address accel-ppp
// hands it in IPCP, then pings across the session.
func checkAccessWholesaleAccel(ctx context.Context, check *interoplab.CheckContext) error {
	subscriber := netip.MustParseAddr(subscriberAddress)
	if _, err := waitCPEAddress(ctx, check, "accel-ppp's "+subscriberAddress,
		func(address netip.Addr) bool { return address == subscriber }); err != nil {
		return err
	}
	log, err := check.Lab.Query(ctx, nodeAccelLNS, []string{commandCat, accelLNSLog}, nil)
	if err != nil {
		return fmt.Errorf("read accel-ppp's log: %w", err)
	}
	if err := accelRenegotiationVerdict(log, subscriberUser); err != nil {
		return err
	}
	return checkPingAcross(ctx, check, nodeAccelLNS, subscriber, pingCount)
}

// accelRenegotiationVerdict reads accel-ppp's verbose PPP log. The CPE was
// Opened with the LAC before the call reached accel-ppp, so an LCP
// Configure-Request accel-ppp received is the CPE's renegotiation, and a
// Configure-Ack is the CPE accepting accel-ppp's own request. The CHAP
// Response naming user and the Success that follows are the re-challenge
// answered. Each line is printed by accel-ppp 1.14.0: ppp_lcp.c lcp_recv_conf_req
// and lcp_recv_conf_ack ("recv [LCP ConfReq", "recv [LCP ConfAck"),
// auth_chap_md5.c chap_recv_response and chap_send_success.
func accelRenegotiationVerdict(log, user string) error {
	confReq := false
	confAck := false
	response := false
	success := false
	for line := range strings.SplitSeq(log, "\n") {
		switch {
		case strings.Contains(line, "recv [LCP ConfReq"):
			confReq = true
		case strings.Contains(line, "recv [LCP ConfAck"):
			confAck = true
		case strings.Contains(line, "recv [CHAP Response") && strings.Contains(line, `name="`+user+`"`):
			response = true
		case strings.Contains(line, "send [CHAP Success"):
			success = true
		}
	}
	if !confReq {
		return fmt.Errorf("accel-ppp received no LCP Configure-Request, so the CPE never renegotiated LCP with it:\n%s", interoplab.OutputExcerpt(log))
	}
	if !confAck {
		return fmt.Errorf("accel-ppp received no LCP Configure-Ack, so the CPE never accepted its Configure-Request:\n%s", interoplab.OutputExcerpt(log))
	}
	if !response {
		return fmt.Errorf("accel-ppp received no CHAP Response naming %s, so the CPE never answered its re-challenge:\n%s", user, interoplab.OutputExcerpt(log))
	}
	if !success {
		return fmt.Errorf("accel-ppp sent no CHAP Success:\n%s", interoplab.OutputExcerpt(log))
	}
	return nil
}
