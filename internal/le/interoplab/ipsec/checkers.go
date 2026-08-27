// Design: test/interop-ipsec/scenarios/*/check.py -- complete strongSwan assertions.
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
	"eap-tls":                        checkEAPTLS,
	"eap-tls13":                      checkEAPTLS13,
	"esp-form-change":                checkESPFormChange,
	"initiator-rekey-answer-narrows": checkInitiatorRekeyAnswerNarrows,
	"invalid-ke-retry":               checkInvalidKERetry,
	"ipsec-bgp-redistribute-frr":     checkIPsecBGPRedistributeFRR,
	"peer-reload-narrowing":          checkPeerReloadNarrowing,
	"psk-site-to-site":               checkPSKSiteToSite,
	"responder-accepts-reinit":       checkResponderAcceptsReinit,
	"responder-eap-mschapv2":         checkResponderEAPMSCHAPv2,
	"responder-eap-tls13":            checkResponderEAPTLS13,
	"responder-ike-rekey":            checkResponderIKERekey,
	"responder-psk":                  checkResponderPSK,
	"responder-raises-child-rekey":   checkResponderRaisesChildRekey,
}

// ScenarioNames returns every typed checker name in lexical producer order.
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
