// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: register_wholesale.go -- the scenarios these checkers judge.
// Related: check_access.go -- the access-chain readers reused here.
package topology

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// wholesaleWait bounds every state poll of the wholesale checkers: the
// session crosses one LCP negotiation at the LAC, the call setup and a
// second LCP negotiation and CHAP at the LNS.
const wholesaleWait = 90 * time.Second

// proxyLCPDetail is the LNS session detail with the key spec-home-cpe-15 adds.
type proxyLCPDetail struct {
	l2tpSession
	ProxyLCP *string `json:"proxy-lcp"`
}

// checkAccessWholesale proves access-wholesale: a Ze CPE behind a Ze LAC
// relaying by realm comes up through a Ze LNS that ignored the proxy LCP,
// sent its own LCP Configure-Request (MRU 1492, on the core capture),
// re-challenged with CHAP against FreeRADIUS and reports `proxy-lcp
// renegotiated`.
func checkAccessWholesale(ctx context.Context, check *interoplab.CheckContext) error {
	subscriber := netip.MustParseAddr(subscriberAddress)
	if _, err := waitCPEAddress(ctx, check, "Framed-IP-Address "+subscriberAddress,
		func(address netip.Addr) bool { return address == subscriber }); err != nil {
		return err
	}
	if err := waitLogin(ctx, check, subscriberUser); err != nil {
		return err
	}
	session, err := lnsSessionOf(ctx, check, subscriberUser, subscriber)
	if err != nil {
		return err
	}
	if err := checkNegotiated(ctx, check, session); err != nil {
		return err
	}
	if err := checkRenegotiated(ctx, check, session); err != nil {
		return err
	}
	return checkPingAcross(ctx, check, subscriber, pingCount)
}

// checkLCPRenegotiatePPPD proves l2tp-lns-lcp-renegotiate-pppd: pppd, Opened
// with the LAC, renegotiates when ze-lns sends its own Configure-Request
// (RFC 1661 Section 4.3), answers ze-lns's CHAP and gets its address.
func checkLCPRenegotiatePPPD(ctx context.Context, check *interoplab.CheckContext) error {
	subscriber := netip.MustParseAddr(subscriberAddress)
	if err := pollUntil(ctx, wholesaleWait, "pppd holds "+subscriberAddress+" on ppp0", func() error {
		answer, err := check.Lab.Query(ctx, nodePPPD, []string{"ip", "-o", "-4", "address", "show", ipWordDev, "ppp0"}, nil)
		if err != nil {
			return err
		}
		if !strings.Contains(answer, " "+subscriberAddress+" ") && !strings.Contains(answer, " "+subscriberAddress+"/") {
			return fmt.Errorf("ppp0 holds no %s:\n%s", subscriberAddress, answer)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := waitLogin(ctx, check, pppdUser); err != nil {
		return err
	}
	session, err := lnsSessionOf(ctx, check, pppdUser, subscriber)
	if err != nil {
		return err
	}
	if err := checkRenegotiated(ctx, check, session); err != nil {
		return err
	}
	frames, err := readCapture(ctx, check, nodeLNS, coreCapture)
	if err != nil {
		return err
	}
	if err := lcpMRUVerdict(frames, netip.MustParseAddr(lnsCoreAddress), sessionMRU); err != nil {
		return err
	}
	log, err := check.Lab.Query(ctx, nodePPPD, []string{"cat", pppdLog}, nil)
	if err != nil {
		return fmt.Errorf("read pppd's log: %w", err)
	}
	if err := pppdRenegotiationVerdict(log); err != nil {
		return err
	}
	ping, err := check.Lab.Query(ctx, nodePPPD,
		[]string{"ping", "-c", strconv.Itoa(pingCount), "-W", "2", lnsCoreAddress}, nil)
	if err != nil {
		return fmt.Errorf("pppd ping %s: %w\n%s", lnsCoreAddress, err, ping)
	}
	return pingVerdict(ping, pingCount)
}

// checkRenegotiated reads the LNS session detail and judges it.
func checkRenegotiated(ctx context.Context, check *interoplab.CheckContext, session l2tpSession) error {
	var detail proxyLCPDetail
	if err := restExecute(ctx, check, nodeLNS, "show l2tp session id "+strconv.Itoa(session.LocalSID), &detail); err != nil {
		return err
	}
	return renegotiatedVerdict(detail)
}

// renegotiatedVerdict accepts only a session that opened LCP, authenticated
// CHAP-MD5 (the LNS's configured method) and reports proxy-lcp renegotiated.
func renegotiatedVerdict(detail proxyLCPDetail) error {
	if detail.ProxyLCP == nil {
		return errors.New("ze-lns's session detail reports proxy-lcp null, so LCP never opened")
	}
	if *detail.ProxyLCP != "renegotiated" {
		return fmt.Errorf("ze-lns's session detail reports proxy-lcp %s, want renegotiated (proxy-lcp renegotiate)", *detail.ProxyLCP)
	}
	if detail.Auth == nil {
		return errors.New("ze-lns's session detail reports auth-method null, so LCP is not open")
	}
	if *detail.Auth != "chap-md5" {
		return fmt.Errorf("ze-lns's session detail reports auth-method %s, want chap-md5", *detail.Auth)
	}
	return nil
}

// pppdRenegotiationVerdict reads pppd's debug log: one LCP negotiation is
// with the LAC, so a renegotiation by the LNS shows a second Configure-Request
// received and a second Configure-Ack sent, then CHAP and IPCP.
func pppdRenegotiationVerdict(log string) error {
	received := strings.Count(log, "rcvd [LCP ConfReq")
	acked := strings.Count(log, "sent [LCP ConfAck")
	if received < 2 {
		return fmt.Errorf("pppd received %d LCP Configure-Requests, want 2 (the LAC's and ze-lns's renegotiation):\n%s", received, log)
	}
	if acked < 2 {
		return fmt.Errorf("pppd acknowledged %d LCP Configure-Requests, want 2:\n%s", acked, log)
	}
	if !strings.Contains(log, "CHAP authentication succeeded") {
		return fmt.Errorf("pppd logs no CHAP success:\n%s", log)
	}
	if !strings.Contains(log, "local  IP address "+subscriberAddress) {
		return fmt.Errorf("pppd logs no IPCP address %s:\n%s", subscriberAddress, log)
	}
	return nil
}

// pollUntil retries probe every second until it answers nil or wait ends,
// and then returns the last refusal naming what was awaited.
func pollUntil(ctx context.Context, wait time.Duration, what string, probe func() error) error {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		last := probe()
		if last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting until %s: %w (last: %w)", what, ctx.Err(), last)
		case <-deadline.C:
			return fmt.Errorf("waiting until %s for %s: %w", what, wait, last)
		case <-tick.C:
		}
	}
}
