// Design: docs/architecture/testing/interop.md -- complete typed assertions for every checked-in strongSwan scenario.
// Detail: helpers.go -- bounded observations and fail-closed protocol queries.
package ipsec

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

type scenarioChecker func(context.Context, *scenarioLab) error

var scenarioCheckers = map[string]scenarioChecker{
	"child-rekey":                    checkChildRekey,
	"child-rekey-narrowing":          checkChildRekeyNarrowing,
	"clear-reestablish":              checkClearReestablish,
	"cookie-challenge":               checkCookieChallenge,
	"delete-while-window-held":       checkDeleteWhileWindowHeld,
	"eap-mschapv2":                   checkEAPMSCHAPv2,
	"eap-nak-method-negotiation":     checkEAPNakMethodNegotiation,
	"eap-tls":                        checkEAPTLS,
	"eap-tls13":                      checkEAPTLS13,
	"esn-both-offered":               checkESNBothOffered,
	"esn-extended-only-refused":      checkESNExtendedOnlyRefused,
	"esp-form-change":                checkESPFormChange,
	"initiator-rekey-answer-narrows": checkInitiatorRekeyAnswerNarrows,
	"invalid-ke-retry":               checkInvalidKERetry,
	"ipsec-bgp-redistribute-frr":     checkIPsecBGPRedistributeFRR,
	"natt-transport-inner-checksum":  checkNATTTransportInnerChecksum,
	"natt-tunnel-inner-checksum":     checkNATTTunnelInnerChecksum,
	"peer-reload-narrowing":          checkPeerReloadNarrowing,
	"psk-site-to-site":               checkPSKSiteToSite,
	"responder-accepts-reinit":       checkResponderAcceptsReinit,
	"responder-eap-mschapv2":         checkResponderEAPMSCHAPv2,
	"responder-eap-tls13":            checkResponderEAPTLS13,
	"responder-ike-rekey":            checkResponderIKERekey,
	"responder-psk":                  checkResponderPSK,
	"responder-raises-child-rekey":   checkResponderRaisesChildRekey,
}

// ScenarioNames returns every typed checker name in lexical selection order.
func ScenarioNames() []string {
	names := make([]string, 0, len(scenarioCheckers))
	for name := range scenarioCheckers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func checkerAdapters() map[string]interoplab.Checker {
	adapters := make(map[string]interoplab.Checker, len(scenarioCheckers))
	for name := range scenarioCheckers {
		adapters[name] = func(context.Context, *interoplab.CheckContext) error { return nil }
	}
	return adapters
}

func establish(ctx context.Context, lab *scenarioLab) error {
	if err := lab.waitSA(ctx, lab.timeout); err != nil {
		return err
	}
	return lab.waitChild(ctx, "")
}

func requireContains(text, needle, message string) error {
	if !strings.Contains(text, needle) {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func checkPSKSiteToSite(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if err := lab.checkXFRMCount(ctx, swanPeer, 2); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not flow through the XFRM tunnel")
}

func checkEAPMSCHAPv2(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	state, err := lab.xfrmState(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(espSPIPattern.FindAllStringSubmatch(state, -1)) == 0 {
		return nil
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if err := lab.checkXFRMCount(ctx, swanPeer, 2); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not flow through the EAP-MSCHAPv2 tunnel")
}

// eapNakFacts are strongSwan's own record of the refusal, in the order the
// exchange produces them.
//
// "initiating EAP_MD5 method" is charon offering Type 4: eap_authenticator.c
// logs "initiating %N method (id 0x%02X)" through eap_type_names. It is the
// positive evidence the absence assertions below need, because an exchange that
// never reached an EAP method Request would install no XFRM SA either.
//
// "EAP/RES/NAK" is charon's parsed-message summary for ze's Response: message.c
// renders an EAP payload as the payload name, the code short name and the type
// short name, so a Type-3 Response reads EAP/RES/NAK. charon decoded ze's octets
// to reach it, which is what makes it a fact about ze's bytes rather than about
// ze's intent.
//
// The scenario's strongswan.conf sets `flush_line = yes`, so both lines reach
// `docker logs` when charon writes them rather than when its stdio buffer fills.
// Without it a short exchange leaves them invisible and the wait below fails on
// charon's buffer instead of on charon's exchange.
var eapNakFacts = [...]string{
	"initiating EAP_MD5 method",
	"EAP/RES/NAK",
}

// checkEAPNakMethodNegotiation proves ze answers a Request for an authentication
// Type it does not run with a Nak another implementation parses as one.
//
// strongSwan is the authenticator and offers eap-md5, which RFC 7296 Section
// 2.16 puts out of ze's scope. ze runs eap-mschapv2 and answers with the legacy
// Nak of RFC 3748 Section 5.3.1.
//
// The scenario asserts the refusal rather than a tunnel, and the peer is the
// reason: the strongSwan image ships no eap-dynamic plugin, which is the only
// charon plugin that answers a received Nak by offering another method. So
// charon ends the exchange, and no XFRM SA appears at either end. That absence
// is read only after the two positive facts above, so a scenario that never got
// as far as EAP fails here instead of passing on an empty kernel.
func checkEAPNakMethodNegotiation(ctx context.Context, lab *scenarioLab) error {
	for _, fact := range eapNakFacts {
		if err := lab.waitLog(ctx, swanPeer, fact, lab.timeout); err != nil {
			return err
		}
	}
	if err := lab.checkXFRMCount(ctx, zePeer, 0); err != nil {
		return err
	}
	return lab.checkXFRMCount(ctx, swanPeer, 0)
}

var eapTLSRefusalFacts = [...]string{
	"cannot export the RFC 5216 Section 2.3 MSK",
	"for peer CN=172.28.0.3",
	"on TLS 1.2",
	"RFC 7627 extended master secret",
	"Move the peer to TLS 1.3 (RFC 9190)",
	"add RFC 7627 to its TLS 1.2 stack",
	"configure another EAP method",
	"crypto/tls: ExportKeyingMaterial is unavailable",
}

func checkEAPTLS(ctx context.Context, lab *scenarioLab) error {
	if err := lab.waitLog(ctx, swanPeer, "negotiated TLS 1.2", lab.timeout); err != nil {
		return err
	}
	if err := lab.waitLog(ctx, swanPeer, "EAP method EAP_TLS succeeded, MSK established", lab.timeout); err != nil {
		return err
	}
	if err := lab.waitLog(ctx, zePeer, eapTLSRefusalFacts[0], lab.timeout); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	for _, fact := range eapTLSRefusalFacts {
		if !strings.Contains(logs, fact) {
			return fmt.Errorf("ze's EAP-TLS refusal does not state: %s", fact)
		}
	}
	if err := lab.checkXFRMCount(ctx, zePeer, 0); err != nil {
		return err
	}
	return lab.checkXFRMCount(ctx, swanPeer, 0)
}

func checkEAPTLS13(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireContains(logs, "negotiated TLS 1.3", "strongSwan never logged negotiated TLS 1.3"); err != nil {
		return err
	}
	if strings.Contains(logs, "negotiated TLS 1.2") {
		return fmt.Errorf("strongSwan logged negotiated TLS 1.2; the version pin did not hold")
	}
	if err := requireContains(logs, "sending TLS cert request", "strongSwan's certificate_authorities list was empty"); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "no traffic through the TLS 1.3 EAP-TLS tunnel")
}

func checkResponderPSK(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	spis, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(spis) == 0 {
		return nil
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if err := lab.checkXFRMCount(ctx, swanPeer, 2); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "no ESP traffic through the responder tunnel")
}

// RFC requirement: RFC9190-2.5-1 positive -- charon's client_process reads the
// protected success indication, requires it to be exactly one octet equal to
// 0, and logs this line only after it has accepted it
// (src/libcharon/plugins/eap_tls/eap_tls.c). Its presence is another
// implementation confirming, on the wire, that Ze sent the encrypted TLS
// record with application data 0x00 the section demands.
// RFC requirement: RFC9190-2.5-1 negative -- the same client refuses to
// produce an MSK when the indication is absent, and says so:
// get_msk logs "missing protected success indication for EAP-TLS with TLS
// 1.3" and returns FAILED. MEASURED 2026-08-12 with tlsMethod.indicateSuccess
// reverted: this line appears and wait_sa_established above times out, so the
// assertion below is what names the cause rather than leaving a bare timeout.
func checkResponderEAPTLS13(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if !strings.Contains(logs, "negotiated TLS 1.3") {
		return fmt.Errorf("strongSwan never logged negotiated TLS 1.3")
	}
	if strings.Contains(logs, "negotiated TLS 1.2") {
		return fmt.Errorf("strongSwan logged negotiated TLS 1.2")
	}
	if !strings.Contains(logs, "received protected success indication via TLS") {
		return fmt.Errorf("strongSwan never accepted the RFC 9190 Section 2.5 protected success indication")
	}
	if strings.Contains(logs, "missing protected success indication") {
		return fmt.Errorf("strongSwan reports a missing protected success indication")
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	_, err = lab.waitXFRM(ctx, zePeer)
	return err
}

func checkResponderEAPMSCHAPv2(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.exec(ctx, swanPeer, "swanctl", "--terminate", "--ike", "ze"); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "first responder EAP SA termination",
	}, func(probe context.Context) (string, error) { return lab.listSAs(probe) }, func(output string) bool {
		return !strings.Contains(output, "ESTABLISHED")
	})
	if err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	retransmitsBefore := strings.Count(swanLogs, "retransmit 1 of request")
	reprocessedBefore := strings.Count(zeLogs, "EAP round missing EAP payload")
	if _, err := lab.exec(ctx, swanPeer, "iptables", "-I", "INPUT", "1", "-s", zeIP, "-p", "udp", "--sport", "4500", "-j", "DROP"); err != nil {
		return err
	}
	defer lab.execQuiet(context.Background(), swanPeer, "iptables", "-D", "INPUT", "-s", zeIP, "-p", "udp", "--sport", "4500", "-j", "DROP")
	if err := lab.check.Lab.ExecDetached(ctx, swanPeer, []string{"swanctl", "--initiate", "--child", "ze-child"}, nil); err != nil {
		return err
	}
	if err := waitDuration(ctx, 8*time.Second); err != nil {
		return err
	}
	lab.execQuiet(ctx, swanPeer, "iptables", "-D", "INPUT", "-s", zeIP, "-p", "udp", "--sport", "4500", "-j", "DROP")
	zeLogs, err = lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if strings.Count(zeLogs, "EAP round missing EAP payload") > reprocessedBefore {
		return fmt.Errorf("ze re-processed retransmitted IKE_AUTH instead of replaying its cached response")
	}
	if err := lab.waitSA(ctx, 60*time.Second); err != nil {
		return err
	}
	if err := lab.waitChild(ctx, ""); err != nil {
		return err
	}
	swanLogs, err = lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if strings.Count(swanLogs, "retransmit 1 of request") <= retransmitsBefore {
		return fmt.Errorf("strongSwan retransmitted nothing during the blackout; duplicate replay assertion is vacuous")
	}
	return nil
}

func checkChildRekey(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	initial, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := waitRekeyAndDelete(ctx, lab, 90*time.Second, false); err != nil {
		return err
	}
	if len(initial) == 0 {
		return nil
	}
	if err := waitDuration(ctx, 3*time.Second); err != nil {
		return err
	}
	after, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(after) == 0 || sameStrings(initial, after) {
		return fmt.Errorf("ESP SPIs did not change after rekey")
	}
	return lab.verifyTunnelTraffic(ctx, "no ESP traffic after the rekey")
}

func checkResponderRaisesChildRekey(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	initial, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := waitRekeyAndDelete(ctx, lab, 120*time.Second, true); err != nil {
		return err
	}
	if len(initial) == 0 {
		return nil
	}
	if err := waitDuration(ctx, 3*time.Second); err != nil {
		return err
	}
	after, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(after) == 0 || sameStrings(initial, after) {
		return fmt.Errorf("ESP SPIs did not change after responder-role Ze rekey")
	}
	return lab.verifyTunnelTraffic(ctx, "no ESP traffic after the rekey Ze raised")
}

func waitRekeyAndDelete(ctx context.Context, lab *scenarioLab, timeout time.Duration, responderRole bool) error {
	outOfWindow := regexp.MustCompile(`expected \d+, ignored`)
	logs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: "strongSwan CREATE_CHILD_SA rekey and old-SA delete",
	}, func(probe context.Context) (string, error) { return lab.logs(probe, swanPeer) }, func(logs string) bool {
		rejected := outOfWindow.MatchString(logs) || strings.Contains(logs, "INVALID_MESSAGE_ID")
		completed := strings.Contains(logs, "parsed CREATE_CHILD_SA request") &&
			strings.Contains(logs, "REKEY_SA") &&
			strings.Contains(logs, "received DELETE for ESP CHILD_SA")
		return rejected || completed
	})
	if err != nil {
		if !strings.Contains(logs, "parsed CREATE_CHILD_SA request") || !strings.Contains(logs, "REKEY_SA") {
			return fmt.Errorf("strongSwan never parsed a CREATE_CHILD_SA REKEY_SA request from Ze: %w", err)
		}
		if !strings.Contains(logs, "received DELETE for ESP CHILD_SA") {
			return fmt.Errorf("strongSwan never received Ze's Delete for the old ESP SA: %w", err)
		}
		return err
	}
	if outOfWindow.MatchString(logs) || strings.Contains(logs, "INVALID_MESSAGE_ID") {
		role := "initiator-role"
		if responderRole {
			role = "responder-role"
		}
		return fmt.Errorf("strongSwan rejected a Message ID from %s Ze", role)
	}
	return nil
}

func checkResponderIKERekey(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	logs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 120 * time.Second, Interval: 3 * time.Second, Description: "strongSwan peer-initiated IKE-SA rekey",
	}, func(probe context.Context) (string, error) { return lab.logs(probe, swanPeer) }, func(logs string) bool {
		failed := strings.Contains(logs, "INVALID_SYNTAX") || strings.Contains(logs, "no IKE config found")
		completed := strings.Contains(logs, "rekeyed") && strings.Contains(logs, "IKE_SA ze[")
		return failed || completed
	})
	if err != nil {
		return err
	}
	if strings.Contains(logs, "INVALID_SYNTAX") || strings.Contains(logs, "no IKE config found") {
		return fmt.Errorf("strongSwan reported an error during IKE rekey")
	}
	sas, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	return requireContains(sas, "ESTABLISHED", "strongSwan IKE SA is not ESTABLISHED after the rekey")
}

func checkClearReestablish(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	before, err := lab.espSPIs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return fmt.Errorf("empty ESP SPI snapshot before clear")
	}
	if _, err := lab.zeCLI(ctx, "clear vpn ipsec sa"); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: 2 * time.Second, Description: "new ESP SA after clear",
	}, func(probe context.Context) (map[string]struct{}, error) { return lab.espSPIs(probe, swanPeer) }, func(after map[string]struct{}) bool {
		return newStrings(before, after)
	})
	if err != nil {
		return err
	}
	return lab.waitSA(ctx, lab.timeout)
}

func checkResponderAcceptsReinit(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	before, err := lab.espSPIs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return fmt.Errorf("empty ESP SPI snapshot before re-init")
	}
	if err := lab.breakLink(ctx); err != nil {
		return err
	}
	lab.execQuiet(ctx, swanPeer, "swanctl", "--terminate", "--ike", "ze")
	lab.restoreLink(ctx)
	if _, err := lab.exec(ctx, swanPeer, "swanctl", "--initiate", "--child", "ze-child"); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: 2 * time.Second, Description: "new ESP SA after responder re-init",
	}, func(probe context.Context) (map[string]struct{}, error) { return lab.espSPIs(probe, swanPeer) }, func(after map[string]struct{}) bool {
		return newStrings(before, after)
	})
	if err != nil {
		return err
	}
	return lab.waitSA(ctx, lab.timeout)
}

func checkCookieChallenge(ctx context.Context, lab *scenarioLab) error {
	if err := lab.waitLog(ctx, swanPeer, "parsed IKE_SA_INIT response 0 [ N(COOKIE) ]", lab.timeout); err != nil {
		return fmt.Errorf("strongSwan never parsed a COOKIE notification from Ze: %w", err)
	}
	if err := lab.waitLog(ctx, swanPeer, "generating IKE_SA_INIT request 0 [ N(COOKIE) SA KE No", lab.timeout); err != nil {
		return fmt.Errorf("strongSwan did not rebuild IKE_SA_INIT with COOKIE first: %w", err)
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not flow through the tunnel built after the COOKIE challenge")
}

func checkInvalidKERetry(ctx context.Context, lab *scenarioLab) error {
	if err := lab.waitLog(ctx, zePeer, "retrying IKE_SA_INIT", lab.timeout); err != nil {
		return err
	}
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if !strings.Contains(zeLogs, "cause=invalid-ke-payload") {
		return fmt.Errorf("ze retried IKE_SA_INIT for another cause")
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if !strings.Contains(swanLogs, "generating IKE_SA_INIT response 0 [ N(INVAL_KE) ]") {
		return fmt.Errorf("strongSwan never sent INVALID_KE_PAYLOAD; retry assertion is vacuous")
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not flow through the retried IKE_SA_INIT tunnel")
}

func checkChildRekeyNarrowing(ctx context.Context, lab *scenarioLab) error {
	const wideLocal, wideRemote = "10.1.0.0/24", "10.2.0.0/24"
	const narrowLocal = "10.1.0.0/25"
	if err := lab.waitSA(ctx, lab.timeout); err != nil {
		return err
	}
	if err := lab.waitChild(ctx, "ze-child"); err != nil {
		return err
	}
	if err := lab.waitChildSelectors(ctx, wideLocal, wideRemote); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if err := lab.waitPolicyPair(ctx, peer, wideLocal, wideRemote, 90*time.Second); err != nil {
			return err
		}
	}
	if err := lab.assertZeSelectors(ctx, wideRemote, wideLocal); err != nil {
		return err
	}
	narrowConfig := "connections {\n    ze {\n        children {\n            ze-child {\n                local_ts = 10.1.0.0/25\n            }\n        }\n    }\n}\n"
	var tb textbuf.Buffer
	command := tb.Str("cat > /etc/swanctl/conf.d/zz-narrow.conf <<'EOF'\n").
		Str(narrowConfig).Str("EOF").String()
	if _, err := lab.exec(ctx, swanPeer, "sh", "-c", command); err != nil {
		return err
	}
	loaded, err := lab.query(ctx, swanPeer, "swanctl", "--load-conns")
	if err != nil {
		return err
	}
	if !strings.Contains(loaded, "loaded connection 'ze'") {
		return fmt.Errorf("strongSwan did not load narrowed connection")
	}
	if err := lab.waitLog(ctx, swanPeer, "deleting IKE_SA ze[", lab.timeout); err != nil {
		return err
	}
	if err := lab.waitChildSelectors(ctx, narrowLocal, wideRemote); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if err := lab.waitPolicyPair(ctx, peer, narrowLocal, wideRemote, 90*time.Second); err != nil {
			return err
		}
	}
	zePairs, err := lab.espPolicyPairs(ctx, zePeer)
	if err != nil {
		return err
	}
	swanPairs, err := lab.espPolicyPairs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if !sameStrings(zePairs, swanPairs) {
		return fmt.Errorf("the two ends hold different SPDs: Ze %v, strongSwan %v", zePairs, swanPairs)
	}
	return lab.assertZeSelectors(ctx, wideRemote, narrowLocal)
}

func checkPeerReloadNarrowing(ctx context.Context, lab *scenarioLab) error {
	const wideLocal, wideRemote = "10.1.0.0/24", "10.2.0.0/24"
	const narrowRemote = "10.2.0.0/25"
	if err := lab.waitSA(ctx, lab.timeout); err != nil {
		return err
	}
	if err := lab.waitChild(ctx, "ze-child"); err != nil {
		return err
	}
	if err := lab.waitChildSelectors(ctx, wideLocal, wideRemote); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if err := lab.waitPolicyPair(ctx, peer, wideLocal, wideRemote, 90*time.Second); err != nil {
			return err
		}
	}
	if err := lab.assertZeSelectors(ctx, wideRemote, wideLocal); err != nil {
		return err
	}
	if err := lab.reloadZe(ctx, filepath.Join(lab.check.Source.Directory, "ze-narrowed.conf")); err != nil {
		return err
	}
	if err := lab.waitChildSelectors(ctx, wideLocal, narrowRemote); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if err := lab.waitPolicyPair(ctx, peer, wideLocal, narrowRemote, 90*time.Second); err != nil {
			return err
		}
	}
	zePairs, err := lab.espPolicyPairs(ctx, zePeer)
	if err != nil {
		return err
	}
	swanPairs, err := lab.espPolicyPairs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if !sameStrings(zePairs, swanPairs) {
		return fmt.Errorf("the two ends hold different SPDs")
	}
	return lab.assertZeSelectors(ctx, narrowRemote, wideLocal)
}

func checkInitiatorRekeyAnswerNarrows(ctx context.Context, lab *scenarioLab) error {
	const zeHalf = "10.2.0.0/24"
	const peerHalf = "10.1.0.0/24"
	const narrowedHalf = "10.2.0.0/25"
	if err := lab.waitSA(ctx, lab.timeout); err != nil {
		return err
	}
	if err := lab.waitChild(ctx, "ze-child"); err != nil {
		return err
	}
	var tb textbuf.Buffer
	if err := lab.waitLog(ctx, swanPeer, tb.Str(peerHalf).Str(" === ").Str(zeHalf).Slice(), lab.timeout); err != nil {
		return err
	}
	if err := lab.waitPolicyPair(ctx, zePeer, zeHalf, peerHalf, 60*time.Second); err != nil {
		return err
	}
	if err := lab.assertZeSelectors(ctx, zeHalf, peerHalf); err != nil {
		return err
	}
	before, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return fmt.Errorf("ze holds no ESP state before narrowed rekey answer")
	}
	if err := lab.waitLog(ctx, swanPeer,
		tb.Reset().Str(peerHalf).Str(" === ").Str(narrowedHalf).Slice(), lab.timeout); err != nil {
		return err
	}
	refusal := tb.Reset().Str("narrows the scope in use ").Str(zeHalf).Str(" <-> ").
		Str(peerHalf).Str(" down to ").Str(narrowedHalf).Str(" <-> ").Str(peerHalf).String()
	if err := lab.waitLog(ctx, zePeer, refusal, lab.timeout); err != nil {
		return err
	}
	after, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if !sameStrings(before, after) {
		return fmt.Errorf("ze installed a replacement Child SA for a narrowed answer")
	}
	pairs, err := lab.espPolicyPairs(ctx, zePeer)
	if err != nil {
		return err
	}
	if len(pairs) != 1 {
		return fmt.Errorf("ze holds unexpected ESP policies: %v", pairs)
	}
	if _, ok := pairs[policyPair(zeHalf, peerHalf)]; !ok {
		return fmt.Errorf("ze no longer holds the original ESP policy")
	}
	return lab.assertZeSelectors(ctx, zeHalf, peerHalf)
}

func checkDeleteWhileWindowHeld(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	established := time.Now()
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	rekeysBefore := strings.Count(logs, "CREATE_CHILD_SA request")
	if err := lab.breakLink(ctx); err != nil {
		return err
	}
	defer lab.restoreLink(context.Background())
	informationals := strings.Count(logs, "parsed INFORMATIONAL request")
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 40 * time.Second, Interval: time.Second, Description: "unanswered Ze liveness probe at strongSwan",
	}, func(probe context.Context) (string, error) { return lab.logs(probe, swanPeer) }, func(current string) bool {
		return strings.Count(current, "parsed INFORMATIONAL request") > informationals
	})
	if err != nil {
		return fmt.Errorf("request window was never proven held: %w", err)
	}
	for time.Since(established) < 20*time.Second {
		current, readErr := lab.logs(ctx, swanPeer)
		if readErr != nil {
			return readErr
		}
		if strings.Count(current, "CREATE_CHILD_SA request") > rekeysBefore {
			return fmt.Errorf("ze sent CREATE_CHILD_SA while its liveness probe was unanswered")
		}
		if err := waitDuration(ctx, time.Second); err != nil {
			return err
		}
	}
	lab.restoreLink(ctx)
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 12 * time.Second, Interval: time.Second, Description: "deferred rekey after request window frees",
	}, func(probe context.Context) (string, error) { return lab.logs(probe, swanPeer) }, func(current string) bool {
		return strings.Count(current, "CREATE_CHILD_SA request") > rekeysBefore && strings.Contains(current, "REKEY_SA")
	})
	if err != nil {
		return err
	}
	return lab.waitLog(ctx, swanPeer, "received DELETE for ESP CHILD_SA", 15*time.Second)
}

// checkESNExtendedOnlyRefused holds ze to its answer when a real peer asks for a Child SA
// it cannot key. What the refusal prevents is the quiet failure: an accepted proposal
// answered with value 0 leaves strongSwan counting 64 bits of sequence number against a
// peer counting 32, so the tunnel establishes and its anti-replay window drops every
// packet ze sends.
//
// RFC requirement: RFC7296-2.7-1 negative -- strongSwan 5.9.14 offers
// esp_proposals = aes256gcm16-esn, the "single ESN transform with value 1" of RFC 7296
// Section 3.3.2, and ze answers no suite: "The responder MUST accept a single proposal or
// reject them all and return an error. The error is given in a notification of type
// NO_PROPOSAL_CHOSEN" (Section 2.7). MEASURED 2026-08-29 with the ESN comparison in
// espProposalMatches reverted: ze accepts that proposal, logs no refusal, and this
// scenario times out, so the assertion names the acceptance rather than a bare absence.
func checkESNExtendedOnlyRefused(ctx context.Context, lab *scenarioLab) error {
	// The refusal ze REACHED is the positive fact, and the pre-fix responder reaches it
	// on no run at all.
	if err := lab.waitLog(ctx, zePeer, "NO_PROPOSAL_CHOSEN", lab.timeout); err != nil {
		return err
	}
	// Nothing was keyed on either side, so no traffic can be silently dropped later.
	sas, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(sas, "INSTALLED") {
		return fmt.Errorf("strongSwan reports an installed CHILD_SA after ze refused the proposal: %s", sas)
	}
	if err := lab.checkXFRMCount(ctx, swanPeer, 0); err != nil {
		return err
	}
	return lab.checkXFRMCount(ctx, zePeer, 0)
}

// checkESNBothOffered is what makes the refusal above a selection rather than a blanket
// rejection of Transform Type 5. RFC 7296 Section 3.3.2 calls an offer of both values the
// usual one from an initiator that supports Extended Sequence Numbers.
//
// RFC requirement: RFC7296-2.7-1 positive -- the same peer offering both values gets one
// of them back: "The accepted cryptographic suite MUST contain exactly one transform of
// each type included in the proposal" (Section 2.7). Traffic is what proves the two ends
// selected the SAME one, because a peer that keyed 64-bit sequence numbers against ze's
// 32-bit state would establish this tunnel and carry nothing through it.
func checkESNBothOffered(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	state, err := lab.xfrmState(ctx, swanPeer)
	if err != nil {
		return err
	}
	if strings.Contains(state, "esn") {
		return fmt.Errorf("strongSwan installed an ESN state after ze answered value 0: %s", state)
	}
	return lab.verifyTunnelTraffic(ctx, "no traffic after ze selected non-extended sequence numbers")
}

func checkESPFormChange(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	zeState, err := lab.xfrmState(ctx, zePeer)
	if err != nil {
		return err
	}
	if strings.TrimSpace(zeState) == "" {
		return nil
	}
	if !strings.Contains(zeState, "encap type espinudp") {
		return fmt.Errorf("ze inbound state carries no ESP-in-UDP template; forms do not disagree")
	}
	sas, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(sas, "TUNNEL-in-UDP") {
		return fmt.Errorf("strongSwan sends UDP-encapsulated ESP; forms agree")
	}
	dropsBefore, err := rawESPDrops(ctx, lab)
	if err != nil {
		return err
	}
	spisBefore, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	mismatchBefore, err := xfrmStat(ctx, lab, zePeer, "XfrmInStateMismatch")
	if err != nil {
		return err
	}
	zeBefore, err := lab.xfrmCounters(ctx, zePeer)
	if err != nil {
		return err
	}
	swanBefore, err := lab.xfrmCounters(ctx, swanPeer)
	if err != nil {
		return err
	}
	for _, direction := range []struct{ peer, target string }{{zePeer, swanIP}, {swanPeer, zeIP}} {
		output := lab.ping(ctx, direction.peer, direction.target, 3)
		loss := regexp.MustCompile(`(\d+)% packet loss`).FindStringSubmatch(output)
		if len(loss) != 2 {
			return fmt.Errorf("%s to %s did not carry lossless ESP: %s", direction.peer, direction.target, output)
		}
		if loss[1] != "0" {
			return fmt.Errorf("%s to %s did not carry lossless ESP: %s", direction.peer, direction.target, output)
		}
	}
	mismatchAfter, err := xfrmStat(ctx, lab, zePeer, "XfrmInStateMismatch")
	if err != nil {
		return err
	}
	if mismatchAfter <= mismatchBefore {
		return fmt.Errorf("XfrmInStateMismatch did not rise; userspace receive path was not exercised")
	}
	zeAfter, err := lab.xfrmCounters(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := assertESPAdvanced(zeBefore, zeAfter, "Ze accepted no ESP across the form disagreement"); err != nil {
		return err
	}
	swanAfter, err := lab.xfrmCounters(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := assertESPAdvanced(swanBefore, swanAfter, "strongSwan accepted no ESP from Ze"); err != nil {
		return err
	}
	dropsAfter, err := rawESPDrops(ctx, lab)
	if err != nil {
		return err
	}
	if dropsAfter > dropsBefore {
		return fmt.Errorf("kernel dropped %d packets on Ze raw ESP socket", dropsAfter-dropsBefore)
	}
	if err := waitDuration(ctx, 2*time.Second); err != nil {
		return err
	}
	spisAfter, err := lab.espSPIs(ctx, zePeer)
	if err != nil {
		return err
	}
	if !sameStrings(spisBefore, spisAfter) {
		return fmt.Errorf("child SA was rekeyed or replaced during form disagreement")
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if strings.Contains(logs, "received DELETE for ESP CHILD_SA") {
		return fmt.Errorf("ze deleted or rekeyed Child SA rather than serving both ESP forms")
	}
	if strings.Contains(logs, "REKEY_SA") {
		return fmt.Errorf("ze deleted or rekeyed Child SA rather than serving both ESP forms")
	}
	return nil
}

func xfrmStat(ctx context.Context, lab *scenarioLab, peer, name string) (uint64, error) {
	output, err := lab.query(ctx, peer, "cat", "/proc/net/xfrm_stat")
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != name {
			continue
		}
		return strconv.ParseUint(fields[1], 10, 64)
	}
	return 0, fmt.Errorf("%s absent from /proc/net/xfrm_stat in %s", name, peer)
}

func rawESPDrops(ctx context.Context, lab *scenarioLab) (uint64, error) {
	output, err := lab.query(ctx, zePeer, "cat", "/proc/net/raw")
	if err != nil {
		return 0, err
	}
	pattern := regexp.MustCompile(`^\s*\d+:\s+\S+:0032\s`)
	for line := range strings.SplitSeq(output, "\n") {
		if !pattern.MatchString(line) {
			continue
		}
		fields := strings.Fields(line)
		return strconv.ParseUint(fields[len(fields)-1], 10, 64)
	}
	return 0, fmt.Errorf("ze holds no raw ESP socket; receive watcher never ran")
}

func checkIPsecBGPRedistributeFRR(ctx context.Context, lab *scenarioLab) error {
	const remoteSite = "10.200.0.0/24"
	if err := lab.waitFRRSession(ctx, zeIP); err != nil {
		return err
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	var tb textbuf.Buffer
	description := tb.Str("Ze XFRM policy for ").Str(remoteSite).String()
	if _, err := lab.waitOutput(ctx, zePeer, []string{"ip", "xfrm", "policy"}, 30*time.Second, description, func(output string) bool {
		return strings.Contains(output, "10.200.0.0")
	}); err != nil {
		return err
	}
	if err := lab.waitFRRRoute(ctx, remoteSite, true); err != nil {
		return err
	}
	if err := lab.check.Lab.Stop(ctx, swanPeer, 10); err != nil {
		return err
	}
	if err := lab.waitFRRRoute(ctx, remoteSite, false); err != nil {
		return err
	}
	output, err := lab.frrOutput(ctx, tb.Reset().Str("show bgp neighbor ").Str(zeIP).Slice())
	if err != nil {
		return err
	}
	return requireContains(output, "BGP state = Established", "BGP session dropped after tunnel down")
}

func waitDuration(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// nattMarker is the payload the natt-* scenarios carry over the Child SA. It is compared
// byte for byte, so a decapsulated packet that arrives truncated or altered fails the
// scenario rather than passing on its length alone.
const nattMarker = "ZE-NATT-FLOW"

// innerChecksumProbe names one crafted datagram and the two counters that answer what a
// receiving stack did with it.
//
// accepted moves when the packet reached the transport layer: a UDP datagram for a closed
// port is counted in Udp.NoPorts, and a TCP SYN for a closed port is answered with a reset
// counted in Tcp.OutRsts. refused moves when the stack dropped the packet for its
// checksum. Exactly one of the two moves for each probe, so neither verdict is read from
// an absence (ai/rules/evidence.md).
//
// Both ports are closed on purpose. A closed port is answered by the stack itself, so no
// listener has to survive between probes for the counters to mean what they say.
type innerChecksumProbe struct {
	protocol string
	port     int
	accepted string
	refused  string
}

var innerChecksumProbes = [...]innerChecksumProbe{
	{protocol: "udp", port: 5555, accepted: "Udp.NoPorts", refused: "Udp.InCsumErrors"},
	{protocol: "tcp", port: 5556, accepted: "Tcp.OutRsts", refused: "Tcp.InCsumErrors"},
}

// checkNATTTransportInnerChecksum proves the UDP-encapsulated TRANSPORT-mode receive path
// that RFC 3948 Section 3.1.2 governs.
//
// RFC 3948 Section 3.1.2 (rfc/full/rfc3948.txt): "When a transport mode has been used to
// transmit packets, contained TCP or UDP headers will have incorrect checksums due to the
// change of parts of the IP header during transit. ... Depending on local policy, one of
// the following MUST be done". Its third alternative reads: "If the protocol header after
// the ESP header is a UDP header, set the checksum field to zero in the UDP header. If
// the protocol after the ESP header is a TCP header, and if there is an option to flag to
// the stack that the TCP checksum does not need to be computed, then that flag MAY be
// used. This SHOULD only be done for transport mode, and if the packet is integrity
// protected."
//
// Ze installs the Child SA and delegates ESP-in-UDP decapsulation to Linux XFRM
// (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go). Three things are
// therefore asserted, and the last is what makes the first two evidence:
//
//  1. Ze's own state carries transport mode AND the ESP-in-UDP template. Those two
//     together are the whole of what puts the kernel on Section 3.1.2's path, and
//     createFirstChildSA (internal/component/ike/engine/child.go) decides both.
//  2. A TCP flow and a UDP flow cross the SA with their payload intact.
//  3. A datagram carrying a DELIBERATELY WRONG inner checksum is still delivered to Ze's
//     stack. A wrong inner checksum is what a NAT leaves behind on a transport-mode flow,
//     so its acceptance here is the observable form of the third alternative.
//     natt-tunnel-inner-checksum is the control: the same sender, tool and flag against a
//     Child SA that differs in its MODE alone is REFUSED there.
//
// The lab has no address-translating middlebox, so strongSwan's encap = yes supplies the
// NAT-T machinery (a faked NAT_DETECTION_SOURCE_IP hash) and nping --badsum supplies the
// wrong checksum the translation would have caused.
//
// natt-tunnel-inner-checksum is the opposite half of the same measurement, and the two
// scenarios must not be changed apart.
//
// RFC requirement: RFC3948-3.1.2-1 positive -- a datagram whose inner checksum a NAT would
// have invalidated is delivered by Ze's stack after transport-mode ESP-in-UDP
// decapsulation, which is the third alternative of Section 3.1.2 in observable form.
// RFC requirement: RFC3948-3.1.2-2 negative -- the "Tunnel mode TCP checksums MUST be
// verified" sentence is scoped to tunnel mode, and this run measures that a TRANSPORT-mode
// SA does not verify. natt-tunnel-inner-checksum carries the positive.
func checkNATTTransportInnerChecksum(ctx context.Context, lab *scenarioLab) error {
	state, err := establishNATT(ctx, lab, "mode transport")
	if err != nil {
		return err
	}
	if err := requireContains(state, "mode transport",
		"ze's INBOUND state is not transport mode, so RFC 3948 Section 3.1.2 never applies: "+state); err != nil {
		return err
	}
	if err := lab.deliverMarker(ctx, "tcp", 5001); err != nil {
		return err
	}
	if err := lab.deliverMarker(ctx, "udp", 5002); err != nil {
		return err
	}
	return lab.checkInnerChecksums(ctx, true)
}

// checkNATTTunnelInnerChecksum proves the other half of RFC 3948 Section 3.1.2.
//
// RFC 3948 Section 3.1.2, inside the third alternative: "Tunnel mode TCP checksums MUST be
// verified." The skip the transport-mode scenario observes is therefore bounded, and this
// scenario is where the bound is measured: the same crafted datagram, over a Child SA that
// differs from the other scenario in its MODE alone, has to be REFUSED.
//
// Ze decides that mode in createFirstChildSA (internal/component/ike/engine/child.go) and
// maps it to XFRM_MODE_TUNNEL in kernelXFRMMode (internal/component/ike/dataplane/
// dataplane.go). Installing tunnel mode as transport would make Linux skip the check this
// requirement demands, and this scenario is what would see it.
//
// RFC requirement: RFC3948-3.1.2-2 positive -- a TCP segment and a UDP datagram whose inner
// checksum is wrong are REFUSED after tunnel-mode ESP-in-UDP decapsulation, so the checksum
// was verified.
// RFC requirement: RFC3948-3.1.2-1 negative -- the transport-mode skip of Section 3.1.2 is
// not applied here: over a tunnel-mode SA that differs in its mode alone, the same crafted
// datagram is dropped.
func checkNATTTunnelInnerChecksum(ctx context.Context, lab *scenarioLab) error {
	state, err := establishNATT(ctx, lab, "mode tunnel")
	if err != nil {
		return err
	}
	if err := requireContains(state, "mode tunnel",
		"ze's INBOUND state is not tunnel mode, so this scenario measures the wrong mode: "+state); err != nil {
		return err
	}
	if err := lab.deliverMarker(ctx, "tcp", 5001); err != nil {
		return err
	}
	if err := lab.deliverMarker(ctx, "udp", 5002); err != nil {
		return err
	}
	return lab.checkInnerChecksums(ctx, false)
}

// establishNATT brings both peers up and returns Ze's XFRM state once it carries the
// ESP-in-UDP template RFC 3948 Section 2.1 defines. The mode is named by the caller and
// appears in the failure text, because a scenario that measured the other mode would
// otherwise report a passing SA and a meaningless verdict.
func establishNATT(ctx context.Context, lab *scenarioLab, mode string) (string, error) {
	if err := establish(ctx, lab); err != nil {
		return "", err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return "", err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return "", err
	}
	state, err := lab.inboundXFRMState(ctx, zePeer, swanIP, zeIP)
	if err != nil {
		return "", err
	}
	if err := requireContains(state, "encap type espinudp sport 4500 dport 4500",
		"ze's INBOUND state carries no ESP-in-UDP template beside "+mode+", so the kernel never reaches RFC 3948 Section 3.1.2: "+state); err != nil {
		return "", err
	}
	return state, nil
}

// checkInnerChecksums sends each crafted probe over the Child SA twice, once with a correct
// inner checksum and once with a corrupt one, and reads Ze's stack counters for the verdict.
//
// decapsulatorSkips states what RFC 3948 Section 3.1.2 asks of the mode under test: true
// for transport mode, where the corrupt datagram is accepted, and false for tunnel mode,
// where it MUST be refused.
//
// THE CONTROL IS THE OTHER SCENARIO, and it has to be, because "the corrupt datagram was
// accepted" would also be the reading if nping had never corrupted anything. The tunnel
// scenario runs the same sender, the same tool and the same flag against the same receiver,
// and differs from the transport scenario in the Child SA's MODE alone. Its REJECTION is
// what proves the corruption is real, so a --badsum that silently did nothing turns that
// scenario red rather than turning this one falsely green.
//
// A control inside one scenario was tried first and removed: sending the same crafted
// datagram from strongSwan to its own address measures the kernel's local delivery path,
// whose checksum handling is not the decapsulation path under test, and it did not answer
// the same way inside the lab as it did on its own.
func (l *scenarioLab) checkInnerChecksums(ctx context.Context, decapsulatorSkips bool) error {
	for _, probe := range innerChecksumProbes {
		if err := l.assertProbeVerdict(ctx, probe, zePeer, zeIP, false, probe.accepted, probe.refused); err != nil {
			return err
		}
		expected, forbidden := probe.refused, probe.accepted
		if decapsulatorSkips {
			expected, forbidden = probe.accepted, probe.refused
		}
		if err := l.assertProbeVerdict(ctx, probe, zePeer, zeIP, true, expected, forbidden); err != nil {
			return err
		}
	}
	return nil
}

// assertProbeVerdict sends one crafted datagram, waits for expected to move, and refuses
// the run when forbidden moved as well. A counter absent from either snapshot is an error,
// so a misspelled name cannot read as "it did not move".
func (l *scenarioLab) assertProbeVerdict(ctx context.Context, probe innerChecksumProbe, observed, target string, badChecksum bool, expected, forbidden string) error {
	before, err := l.snmpCounters(ctx, observed)
	if err != nil {
		return err
	}
	if _, err := l.craftedProbe(ctx, probe.protocol, probe.port, target, badChecksum); err != nil {
		return err
	}

	checksum := "a correct inner checksum"
	if badChecksum {
		checksum = "a corrupt inner checksum"
	}
	var tb textbuf.Buffer
	description := tb.Str(observed).Str(" counter ").Str(expected).Str(" to move for one ").
		Str(probe.protocol).Str(" datagram with ").Str(checksum).String()
	text, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: probeWaitTimeout, Interval: time.Second, Description: description,
	}, func(poll context.Context) (string, error) {
		return l.exec(poll, observed, "cat", "/proc/net/snmp")
	}, func(text string) bool {
		after, parseErr := parseSNMPCounters(text)
		if parseErr != nil {
			return false
		}
		moved, ok := counterDelta(before, after, expected)
		return ok && moved > 0
	})
	if err != nil {
		return err
	}

	after, err := parseSNMPCounters(text)
	if err != nil {
		return fmt.Errorf("%s /proc/net/snmp after the %s probe: %w", observed, probe.protocol, err)
	}
	moved, ok := counterDelta(before, after, forbidden)
	if !ok {
		return fmt.Errorf("%s /proc/net/snmp carries no %s counter, so the %s probe proves nothing",
			observed, forbidden, probe.protocol)
	}
	if moved != 0 {
		return fmt.Errorf("%s answered the %s datagram with %s on both counters: %s advanced by %d as well",
			observed, probe.protocol, checksum, forbidden, moved)
	}
	return nil
}
