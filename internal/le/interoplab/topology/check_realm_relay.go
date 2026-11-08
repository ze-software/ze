// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: register_wholesale.go -- the realm-relay scenarios these checkers judge.
// Related: check_wholesale.go -- the pppd readers reused here.
// Related: check_wholesale_accel.go -- accel-ppp's log verdict reused here.
package topology

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// Values ze-lac.conf of both realm-relay scenarios declares: its core
// address (`environment l2tp server main ip`), the realm it relays and the
// remote it relays it to.
const (
	lacCoreAddress = "192.0.2.2"
	relayedRealm   = "lab.example.net"
	relayRemote    = "lns"
)

// lacLocalRefusal is the CHAP Failure message the LAC sends a subscriber it
// authenticates locally with no user configured. The l2tp-auth-local plugin
// registers its handler from init() in every build that carries L2TP
// (authlocal/register.go), so with no `authentication local user` the
// handler answers (authlocal/auth.go localAuth.handle); the "no auth handler
// configured" refusal of l2tp/pppoe/drain.go is reached only by a build
// without that plugin. pppd prints the message after "CHAP authentication
// failed: " (ppp 2.5.5 chap.c chap_handle_status).
const lacLocalRefusal = "no users configured"

// lacRelaySession is the part of a LAC session `show l2tp sessions` answers
// that a realm call adds (spec-home-cpe-14 AC-12). A Service-Name relay or a
// local session carries none of these keys, so each is a pointer: absent
// reads nil, never a zero that passes.
type lacRelaySession struct {
	LocalSID      int     `json:"local-sid"`
	RelayRealm    *string `json:"relay-realm"`
	RelayRemote   *string `json:"relay-remote"`
	ProxyLCPSent  *bool   `json:"proxy-lcp-sent"`
	ProxyAuthSent *bool   `json:"proxy-auth-sent"`
}

// checkRealmRelayAccel proves l2tp-lac-realm-relay-accel-ppp (spec-home-cpe-14,
// user stories 1 and 3) against accel-ppp as the LNS: pppd, whose realm the
// LAC relays, comes up with the address accel-ppp hands it after accel-ppp
// ran its own LCP and CHAP with it; the LAC's session is a realm call that
// sent proxy LCP and proxy authentication; pppd pings accel-ppp across the
// session; and pppd-stray is refused by the LAC and never reaches accel-ppp.
func checkRealmRelayAccel(ctx context.Context, check *interoplab.CheckContext) error {
	if err := waitPPPDAddress(ctx, check); err != nil {
		return err
	}
	if err := checkLACRealmSession(ctx, check); err != nil {
		return err
	}
	log, err := check.Lab.Query(ctx, nodeAccelLNS, []string{commandCat, accelLNSLog}, nil)
	if err != nil {
		return fmt.Errorf("read accel-ppp's log: %w", err)
	}
	if err := accelRenegotiationVerdict(log, pppdUser); err != nil {
		return err
	}
	if err := pppdPing(ctx, check, poolGateway); err != nil {
		return err
	}
	// accel-ppp prints a CHAP Response's name as name="<name>"
	// (auth_chap_md5.c chap_recv_response).
	return checkStrayRefused(ctx, check, nodeAccelLNS, accelLNSLog, `name="`+strayUser+`"`)
}

// checkRealmRelayXL2TPD proves l2tp-lac-realm-relay-xl2tpd against xl2tpd and
// pppd as the LNS: the same clauses as checkRealmRelayAccel, with xl2tpd's
// log showing the call it established from the LAC's ICCN and the LNS pppd's
// debug log showing its own LCP and CHAP with the subscriber.
func checkRealmRelayXL2TPD(ctx context.Context, check *interoplab.CheckContext) error {
	if err := waitPPPDAddress(ctx, check); err != nil {
		return err
	}
	if err := checkLACRealmSession(ctx, check); err != nil {
		return err
	}
	xl2tpdLog, err := check.Lab.Query(ctx, nodeXL2TPDLNS, []string{commandCat, xl2tpdLNSLog}, nil)
	if err != nil {
		return fmt.Errorf("read xl2tpd's log: %w", err)
	}
	pppdLNSLog, err := check.Lab.Query(ctx, nodeXL2TPDLNS, []string{commandCat, xl2tpdPPPDLog}, nil)
	if err != nil {
		return fmt.Errorf("read the LNS pppd's log: %w", err)
	}
	if err := xl2tpdLNSVerdict(xl2tpdLog, pppdLNSLog, pppdUser); err != nil {
		return err
	}
	if err := pppdPing(ctx, check, poolGateway); err != nil {
		return err
	}
	// pppd prints a CHAP Response's name as name = "<name>" (ppp 2.5.5
	// chap.c chap_print_pkt, utils.c print_string).
	return checkStrayRefused(ctx, check, nodeXL2TPDLNS, xl2tpdPPPDLog, `name = "`+strayUser+`"`)
}

// checkLACRealmSession reads the LAC's sessions over REST and judges them.
func checkLACRealmSession(ctx context.Context, check *interoplab.CheckContext) error {
	var sessions []lacRelaySession
	if err := restExecute(ctx, check, nodeLAC, "show l2tp sessions", &sessions); err != nil {
		return err
	}
	return lacRealmSessionVerdict(sessions)
}

// lacRealmSessionVerdict accepts only one LAC session, the realm call for
// pppd: it names relayedRealm and relayRemote and reports that its ICCN
// carried the proxy LCP and the proxy authentication AVPs (RFC 2661 Section
// 4.4.5). pppd-stray is authenticated locally, so it holds no L2TP session.
func lacRealmSessionVerdict(sessions []lacRelaySession) error {
	if len(sessions) != 1 {
		return fmt.Errorf("ze-lac holds %d L2TP sessions, want 1: pppd's realm call (pppd-stray's realm is not relayed)", len(sessions))
	}
	session := sessions[0]
	if session.RelayRealm == nil {
		return errors.New("ze-lac's session carries no relay-realm, so it is not a realm call")
	}
	if *session.RelayRealm != relayedRealm {
		return fmt.Errorf("ze-lac's session reports relay-realm %q, want %s", *session.RelayRealm, relayedRealm)
	}
	if session.RelayRemote == nil {
		return errors.New("ze-lac's session carries no relay-remote")
	}
	if *session.RelayRemote != relayRemote {
		return fmt.Errorf("ze-lac's session reports relay-remote %q, want %s", *session.RelayRemote, relayRemote)
	}
	if session.ProxyLCPSent == nil {
		return errors.New("ze-lac's session carries no proxy-lcp-sent")
	}
	if !*session.ProxyLCPSent {
		return errors.New("ze-lac's session reports proxy-lcp-sent false, so its ICCN carried no proxy LCP AVPs")
	}
	if session.ProxyAuthSent == nil {
		return errors.New("ze-lac's session carries no proxy-auth-sent")
	}
	if !*session.ProxyAuthSent {
		return errors.New("ze-lac's session reports proxy-auth-sent false, so its ICCN carried no proxy authentication AVPs for a CHAP subscriber")
	}
	return nil
}

// checkStrayRefused waits until pppd-stray logs the LAC's local refusal, then
// reads the LNS's log and refuses one that holds nameMarker, pppd-stray's
// name as that LNS prints a CHAP Response: a call the LAC should never have
// placed.
func checkStrayRefused(ctx context.Context, check *interoplab.CheckContext, lns, lnsLog, nameMarker string) error {
	if err := pollUntil(ctx, wholesaleWait, "ze-lac refuses pppd-stray", func() error {
		log, err := check.Lab.Query(ctx, nodeStrayPPPD, []string{commandCat, pppdLog}, nil)
		if err != nil {
			return fmt.Errorf("read pppd-stray's log: %w", err)
		}
		return strayRefusedVerdict(log)
	}); err != nil {
		return err
	}
	log, err := check.Lab.Query(ctx, lns, []string{commandCat, lnsLog}, nil)
	if err != nil {
		return fmt.Errorf("read %s's log: %w", lns, err)
	}
	return lnsNeverSawVerdict(log, nameMarker, lns)
}

// strayRefusedVerdict reads pppd-stray's debug log. A CHAP success means a
// subscriber whose realm no binding names was admitted; a CHAP Failure
// without lacLocalRefusal is a refusal some other party sent, such as an LNS
// the LAC wrongly relayed it to.
func strayRefusedVerdict(log string) error {
	if strings.Contains(log, "CHAP authentication succeeded") {
		return fmt.Errorf("pppd-stray logs a CHAP success, so a subscriber whose realm no binding names was admitted:\n%s", interoplab.OutputExcerpt(log))
	}
	if !strings.Contains(log, "CHAP authentication failed: "+lacLocalRefusal) {
		return fmt.Errorf("pppd-stray logs no CHAP Failure carrying %q, the LAC's local refusal:\n%s", lacLocalRefusal, interoplab.OutputExcerpt(log))
	}
	return nil
}

// lnsNeverSawVerdict refuses an LNS log that holds nameMarker.
func lnsNeverSawVerdict(log, nameMarker, lns string) error {
	if strings.Contains(log, nameMarker) {
		return fmt.Errorf("%s's log holds %s, so ze-lac relayed pppd-stray, whose realm no binding names:\n%s", lns, nameMarker, interoplab.OutputExcerpt(log))
	}
	return nil
}

// xl2tpdLNSVerdict reads xl2tpd's log and the debug log of the pppd it
// started. xl2tpd logs "Call established with <peer>" once it accepted the
// LAC's ICCN and started pppd (xl2tpd 1.3.20 control.c control_finish, case
// ICCN). It ignores the proxy AVPs (avp.c: no handler for 26 to 28,
// ignore_avp for 29 to 33), so its pppd runs its own LCP and CHAP with a
// subscriber the LAC already took to LCP Opened: a Configure-Request and a
// Configure-Ack it received are the subscriber's renegotiation (RFC 1661
// Section 4.3), and the CHAP Response naming user with the Success it sent
// are the re-challenge answered. ppp 2.5.5 prints each packet as
// "rcvd [..." or "sent [..." (utils.c dump_packet) and the IPCP peer as
// "remote IP address" (ipcp.c ipcp_up).
func xl2tpdLNSVerdict(xl2tpdLog, pppdLog, user string) error {
	if !strings.Contains(xl2tpdLog, "Call established with "+lacCoreAddress) {
		return fmt.Errorf("xl2tpd logs no call established with ze-lac (%s), so it never accepted the LAC's ICCN:\n%s", lacCoreAddress, interoplab.OutputExcerpt(xl2tpdLog))
	}
	confReq := false
	confAck := false
	response := false
	success := false
	for line := range strings.SplitSeq(pppdLog, "\n") {
		switch {
		case strings.Contains(line, "rcvd [LCP ConfReq"):
			confReq = true
		case strings.Contains(line, "rcvd [LCP ConfAck"):
			confAck = true
		case strings.Contains(line, "rcvd [CHAP Response") && strings.Contains(line, `name = "`+user+`"`):
			response = true
		case strings.Contains(line, "sent [CHAP Success"):
			success = true
		}
	}
	if !confReq {
		return fmt.Errorf("the LNS pppd received no LCP Configure-Request, so the subscriber never renegotiated LCP with it:\n%s", interoplab.OutputExcerpt(pppdLog))
	}
	if !confAck {
		return fmt.Errorf("the LNS pppd received no LCP Configure-Ack, so the subscriber never accepted its Configure-Request:\n%s", interoplab.OutputExcerpt(pppdLog))
	}
	if !response {
		return fmt.Errorf("the LNS pppd received no CHAP Response naming %s, so the subscriber never answered its challenge:\n%s", user, interoplab.OutputExcerpt(pppdLog))
	}
	if !success {
		return fmt.Errorf("the LNS pppd sent no CHAP Success:\n%s", interoplab.OutputExcerpt(pppdLog))
	}
	if !strings.Contains(pppdLog, "remote IP address "+subscriberAddress) {
		return fmt.Errorf("the LNS pppd logs no IPCP peer address %s:\n%s", subscriberAddress, interoplab.OutputExcerpt(pppdLog))
	}
	return nil
}
