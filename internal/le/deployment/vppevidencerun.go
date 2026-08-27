// Design: docs/architecture/testing/interop.md -- the eight real VPP proof scenarios
// Overview: vppevidence.go -- the shared container and ordered scenario dispatch
//
// Each function below is the Go producer for one function in effective-vpp.py.
// Each query failure is an operating error. A successful query that does not
// contain the required state is a failed check in the structured report.
package deployment

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type ipsecProbeIDs struct {
	SPD      string
	SAD      string
	CloseSPI string
	CloseSPD string
}

func (i ipsecProbeIDs) complete() bool {
	if i.SPD == "" {
		return false
	}
	if i.SAD == "" {
		return false
	}
	if i.CloseSPI == "" {
		return false
	}
	return i.CloseSPD != ""
}

func parseIPsecProbe(output string) (ipsecProbeIDs, error) {
	if strings.Contains(output, "SKIP") {
		return ipsecProbeIDs{}, errors.New("the IPsec probe skipped; it must program VPP")
	}
	values := make(map[string]string, 4)
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.HasPrefix(line, VPPIPsecReportPrefix) {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, VPPIPsecReportPrefix)), "=")
		if ok {
			values[key] = value
		}
	}
	ids := ipsecProbeIDs{
		SPD:      values["spd-id"],
		SAD:      values["sad-id"],
		CloseSPI: values["close-removed-spi"],
		CloseSPD: values["close-removed-spd-id"],
	}
	if !ids.complete() {
		var tb textbuf.Buffer
		return ipsecProbeIDs{}, errors.New(tb.Str("the IPsec probe reported incomplete ids: ").Str(output).String())
	}
	return ids, nil
}

func (v *VPP) runIPsec(container, _ string) (VPPScenarioReport, error) {
	checks := make([]VPPCheck, 0, 5)
	interfaces, err := v.query(container, "show interface")
	if err != nil {
		return VPPScenarioReport{}, err
	}
	index, err := parseVPPInterfaceIndex(interfaces, v.iface)
	if err != nil {
		return VPPScenarioReport{}, err
	}

	var tb textbuf.Buffer
	api := tb.Str("ZE_VPP_IPSEC_API_SOCKET=/run/vpp/api.sock").String()
	swIndex := tb.Reset().Str("ZE_VPP_IPSEC_SW_IF_INDEX=").Int(int64(index)).String()
	probe := tb.Reset().Str("/src/").Str(filepath.ToSlash(vppIPsecProbeRel(v.Goarch))).String()
	output, ok := v.dockerText(
		dockerExec,
		dockerEnv, api,
		dockerEnv, swIndex,
		container,
		probe,
		"-test.run", "TestVPPRealDataplaneInstalls",
		"-test.v",
	)
	if !ok {
		checks = append(checks, failVPPCheck("probe", "the IKE dataplane backend could not program a real VPP", output, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	ids, err := parseIPsecProbe(output)
	if err != nil {
		checks = append(checks, failVPPCheck("probe-report", err.Error(), output, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	checks = append(checks, passVPPCheck("close-cleanup",
		tb.Reset().Str("Close removed the SA ").Str(ids.CloseSPI).Str(" and SPD ").Str(ids.CloseSPD).
			Str(" it installed, so a restart leaves no orphan").String()))

	summary, err := v.query(container, "show ipsec sa")
	if err != nil {
		return VPPScenarioReport{}, err
	}
	for _, needle := range []string{
		tb.Reset().Str("spi ").Uint(VPPIPsecSPI).String(),
		tb.Reset().Str("spi ").Uint(VPPIPsecInboundSPI).String(),
		"protocol:esp", "tunnel",
	} {
		if strings.Contains(summary, needle) {
			continue
		}
		checks = append(checks, failVPPCheck("sa-list",
			tb.Reset().Str("real VPP SA list does not report ").Quoted(needle).String(), summary, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	inbound := make([]string, 0, 1)
	for line := range strings.SplitSeq(summary, "\n") {
		if strings.Contains(line, "inbound") {
			inbound = append(inbound, line)
		}
	}
	inboundSPI := tb.Reset().Str("spi ").Uint(VPPIPsecInboundSPI).String()
	if len(inbound) != 1 {
		checks = append(checks, failVPPCheck("sa-direction",
			tb.Reset().Str("exactly one SA must carry the inbound flag, and it must be spi ").
				Uint(VPPIPsecInboundSPI).String(), summary, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	if !strings.Contains(inbound[0], inboundSPI) {
		checks = append(checks, failVPPCheck("sa-direction",
			tb.Reset().Str("exactly one SA must carry the inbound flag, and it must be spi ").
				Uint(VPPIPsecInboundSPI).String(), summary, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	checks = append(checks, passVPPCheck("sa-list",
		"real VPP holds both SAs ze installed, and flags one of them inbound", tailLines(summary)...))

	saIndex := ""
	outboundSPI := tb.Reset().Str("spi ").Uint(VPPIPsecSPI).String()
	for line := range strings.SplitSeq(summary, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		if !strings.Contains(line, outboundSPI) {
			continue
		}
		end := strings.IndexByte(line, ']')
		if end > 1 {
			saIndex = line[1:end]
			break
		}
	}
	if saIndex == "" {
		checks = append(checks, failVPPCheck("sa-index",
			tb.Reset().Str("no runtime index for spi ").Uint(VPPIPsecSPI).String(), summary, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	detail, err := v.query(container, vppCommand("show ipsec sa", saIndex))
	if err != nil {
		return VPPScenarioReport{}, err
	}
	for _, needle := range []string{
		tb.Reset().Str("salt ").Str(VPPIPsecSalt).String(),
		"aes-gcm-256", VPPIPsecCipherKey, "integrity alg none", "encap-copy-ecn", "decap-copy-ecn",
	} {
		if strings.Contains(detail, needle) {
			continue
		}
		checks = append(checks, failVPPCheck("sa-detail",
			tb.Reset().Str("real VPP SA does not report ").Quoted(needle).String(), detail, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	checks = append(checks,
		passVPPCheck("aead-key", "real VPP reports the AEAD cipher key and its salt in their own fields"),
		passVPPCheck("ecn-flags", "real VPP holds the ECN copy flags RFC 7296 Section 2.24 requires", tailLines(detail)...),
	)

	spd, err := v.query(container, "show ipsec all")
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !strings.Contains(spd, tb.Reset().Str("spd ").Str(ids.SPD).String()) {
		checks = append(checks, failVPPCheck("spd", tb.Reset().Str("real VPP holds no SPD ").Str(ids.SPD).String(), spd, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	iface := v.iface
	if !strings.Contains(spd, tb.Reset().Str(ids.SPD).Str(" -> ").Str(iface).String()) {
		checks = append(checks, failVPPCheck("spd-binding",
			tb.Reset().Str("SPD ").Str(ids.SPD).Str(" is not bound to ").Str(iface).String(), spd, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	outbound := make([]string, 0, 2)
	for line := range strings.SplitSeq(spd, "\n") {
		if strings.Contains(line, "type ip4-outbound") {
			outbound = append(outbound, strings.TrimSpace(line))
		}
	}
	validPolicies := len(outbound) == 2
	if validPolicies {
		validPolicies = strings.Contains(outbound[0], "priority -100 action bypass")
	}
	if validPolicies {
		validPolicies = strings.Contains(outbound[1], "priority -2000 action protect")
	}
	if validPolicies {
		validPolicies = strings.Contains(outbound[1], "protocol any")
	}
	if !validPolicies {
		checks = append(checks, failVPPCheck("spd-policies", "the SPD policies have the wrong count, order, action, or protocol", spd, nil))
		return finishVPPScenario(VPPScenarioIPsec, checks), nil
	}
	checks = append(checks, passVPPCheck("spd-policies",
		tb.Reset().Str("real VPP holds SPD ").Str(ids.SPD).Str(" bound to ").Str(iface).
			Str(", with both policies").String(), tailLines(spd)...))
	return finishVPPScenario(VPPScenarioIPsec, checks), nil
}

func (v *VPP) runIPv4FIB(container, work string) (VPPScenarioReport, error) {
	return v.runFIB(container, work, false)
}

func (v *VPP) runMPLSFIB(container, work string) (VPPScenarioReport, error) {
	return v.runFIB(container, work, true)
}

func (v *VPP) runFIB(container, work string, mpls bool) (VPPScenarioReport, error) {
	scenario := VPPScenarioIPv4FIB
	peerFile := "peer-script"
	configFile := "fib.conf"
	prefix := VPPFIBPrefix
	label := 0
	if mpls {
		scenario = VPPScenarioMPLSFIB
		peerFile = "mpls-peer-script"
		configFile = "mpls-fib.conf"
		prefix = VPPMPLSPrefix
		label = VPPMPLSLabel
	}
	port, err := v.freePort()
	if err != nil {
		return VPPScenarioReport{}, err
	}
	var tb textbuf.Buffer
	peerBody := tb.Str("option=tcp_connections:value=1\noption=update:value=send-route:prefix=").
		Str(prefix).Str(":next-hop=").Str(VPPNextHop).Str(":origin-as=65001")
	if mpls {
		peerBody.Str(":label=").Int(int64(label))
	}
	peerBody.Byte('\n')
	if err := v.writeConfig(work, peerFile, peerBody.String()); err != nil {
		return VPPScenarioReport{}, err
	}
	if err := v.writeConfig(work, configFile, vppFIBConfig(mpls)); err != nil {
		return VPPScenarioReport{}, err
	}

	peerSeen := newCollector(vppPeerReadyLine)
	// #nosec G204 -- docker is fixed; the container, built binary, numeric port, and peer file are produced by this closed FIB scenario.
	peerCmd := exec.CommandContext(context.Background(), "docker",
		dockerExec, container,
		tb.Reset().Str("/src/").Str(filepath.ToSlash(vppTestRel(v.Goarch))).String(),
		"peer", "--mode", "sink", "--port", tb.Reset().Int(int64(port)).String(),
		tb.Reset().Str(vppMount).Byte('/').Str(peerFile).String(),
	)
	peer, err := startWatched(peerCmd, "peer> ", peerSeen, v.Progress)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer stopVPPProcess(peer, peerSeen)
	if !await(peerSeen, vppPeerReadyLine, peer, vppPeerWait) {
		return VPPScenarioReport{}, errors.New("ze-test peer did not start")
	}

	daemon, daemonSeen, err := v.startEvidenceDaemon(container, configFile, port)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer stopVPPProcess(daemon, daemonSeen)

	query := vppCommand("show ip fib", prefix)
	present := func(text string) bool {
		if !strings.Contains(text, prefix) {
			return false
		}
		if !mpls {
			return true
		}
		if !strings.Contains(strings.ToLower(text), "label") {
			return false
		}
		return strings.Contains(text, tb.Reset().Int(int64(label)).String())
	}
	ok, last, err := v.awaitQuery(container, query, true, vppApplyWait, present)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	checks := make([]VPPCheck, 0, 2)
	if !ok {
		detail := "real VPP FIB route not observed"
		if mpls {
			detail = "real VPP MPLS label push route not observed"
		}
		checks = append(checks, failVPPCheck("installed", detail, last, daemonSeen))
		return finishVPPScenario(scenario, checks), nil
	}
	installDetail := tb.Reset().Str("real VPP FIB contains ").Str(prefix)
	if mpls {
		installDetail.Str(" with MPLS label ").Int(int64(label))
	}
	checks = append(checks, passVPPCheck("installed", installDetail.String()))

	if _, ok := v.containerText(container, "pkill", "-TERM", "-f", filepath.Base(vppTestRel(v.Goarch))); !ok {
		return VPPScenarioReport{}, errors.New("failed to stop the ze-test peer")
	}
	waitForExit(peer, vppPeerWait)

	ok, last, err = v.awaitQuery(container, query, false, vppWithdrawWait, present)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		detail := "real VPP FIB route was not withdrawn"
		if mpls {
			detail = "real VPP MPLS route was not withdrawn"
		}
		checks = append(checks, failVPPCheck("withdrawn", detail, last, daemonSeen))
		return finishVPPScenario(scenario, checks), nil
	}
	withdrawDetail := tb.Reset().Str("real VPP FIB withdrew ").Str(prefix)
	if mpls {
		withdrawDetail.Reset().Str("real VPP FIB withdrew MPLS route ").Str(prefix)
	}
	checks = append(checks, passVPPCheck("withdrawn", withdrawDetail.String()))
	return finishVPPScenario(scenario, checks), nil
}

func waitForExit(proc *running, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		if proc.exited() {
			return
		}
		if !time.Now().Before(deadline) {
			return
		}
		time.Sleep(pollInterval)
	}
}

func (v *VPP) runTrafficInterface(container, work string) (VPPScenarioReport, error) {
	iface := v.iface
	name := vppPolicerName(iface, VPPTrafficPolicerClass)
	file := "traffic.conf"
	if err := v.writeConfig(work, file, vppTrafficConfig(iface, VPPScenarioTrafficInterface, true)); err != nil {
		return VPPScenarioReport{}, err
	}
	checks := make([]VPPCheck, 0, 3)

	daemon, seen, err := v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !v.waitDaemonLine(seen, daemon, vppTrafficLogLine) {
		checks = append(checks, failVPPCheck("applied", "traffic-control apply log not observed", "", seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
	}
	ok, text, err := v.awaitPolicer(container, name, true, vppQueryWait)
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("applied", vppMissingPolicer(name, "not observed after apply"), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
	}
	ok, text, err = v.awaitQuery(container, vppCommand("show interface features", iface), true, vppQueryWait,
		func(value string) bool { return strings.Contains(strings.ToLower(value), "policer") })
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("bound", tbDetail("real VPP policer feature not observed on ", iface), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
	}
	checks = append(checks, passVPPCheck("applied", vppPolicerDetail(name, "exists and is bound to", iface)))
	stopVPPProcess(daemon, seen)

	daemon, seen, err = v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	ok, text, err = v.awaitPolicer(container, name, true, vppApplyWait)
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("restart", vppMissingPolicer(name, "missing after ze restart with same config"), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
	}
	ok, text, err = v.awaitQuery(container, vppCommand("show interface features", iface), true, vppQueryWait,
		func(value string) bool { return strings.Contains(strings.ToLower(value), "policer") })
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("restart-bound", tbDetail("real VPP policer feature not observed on ", iface, " after ze restart"), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
	}
	checks = append(checks, passVPPCheck("restart", tbDetail("real VPP traffic policer ", name, " survived ze restart with same config")))
	stopVPPProcess(daemon, seen)

	if err := v.writeConfig(work, file, vppTrafficConfig(iface, VPPScenarioTrafficInterface, false)); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon, seen, err = v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer stopVPPProcess(daemon, seen)
	ok, text, err = v.awaitPolicer(container, name, false, vppReconcileWait)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("cleanup", vppMissingPolicer(name, "orphan survived ze restart cleanup"), text, seen))
		return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
	}
	checks = append(checks, passVPPCheck("cleanup", tbDetail("real VPP startup cleanup removed orphan traffic policer ", name)))
	return finishVPPScenario(VPPScenarioTrafficInterface, checks), nil
}

func (v *VPP) runTrafficProtocol(container, work string) (VPPScenarioReport, error) {
	return v.runTrafficClassify(container, work, VPPScenarioTrafficProtocol, "traffic-proto.conf",
		[]string{VPPTrafficProtocolClass})
}

func (v *VPP) runTrafficDSCP(container, work string) (VPPScenarioReport, error) {
	return v.runTrafficClassify(container, work, VPPScenarioTrafficDSCP, "traffic-dscp.conf",
		[]string{VPPTrafficDSCPClass})
}

func (v *VPP) runTrafficMultiClass(container, work string) (VPPScenarioReport, error) {
	return v.runTrafficClassify(container, work, VPPScenarioTrafficMultiClass, "traffic-mc.conf",
		[]string{VPPTrafficMultiClassA, VPPTrafficMultiClassB})
}

func (v *VPP) runTrafficClassify(container, work, scenario, file string, classes []string) (VPPScenarioReport, error) {
	iface := v.iface
	names := make([]string, 0, len(classes))
	for _, class := range classes {
		names = append(names, vppPolicerName(iface, class))
	}
	if err := v.writeConfig(work, file, vppTrafficConfig(iface, scenario, true)); err != nil {
		return VPPScenarioReport{}, err
	}
	checks := make([]VPPCheck, 0, 2)
	daemon, seen, err := v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !v.waitDaemonLine(seen, daemon, vppTrafficLogLine) {
		checks = append(checks, failVPPCheck("applied", vppTrafficApplyMissing(scenario), "", seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(scenario, checks), nil
	}
	for _, name := range names {
		ok, text, queryErr := v.awaitPolicer(container, name, true, vppQueryWait)
		if queryErr != nil {
			stopVPPProcess(daemon, seen)
			return VPPScenarioReport{}, queryErr
		}
		if !ok {
			checks = append(checks, failVPPCheck("applied", vppClassifyPolicerMissing(scenario, name), text, seen))
			stopVPPProcess(daemon, seen)
			return finishVPPScenario(scenario, checks), nil
		}
	}
	if scenario != VPPScenarioTrafficMultiClass {
		ok, text, queryErr := v.awaitQuery(container, "show classify tables", true, vppQueryWait,
			func(value string) bool { return !strings.Contains(value, "No classifier tables configured") })
		if queryErr != nil {
			stopVPPProcess(daemon, seen)
			return VPPScenarioReport{}, queryErr
		}
		if !ok {
			checks = append(checks, failVPPCheck("classify-tables", vppClassifyTablesMissing(scenario), text, seen))
			stopVPPProcess(daemon, seen)
			return finishVPPScenario(scenario, checks), nil
		}
	}
	ok, text, queryErr := v.awaitQuery(container, vppCommand("show interface features", iface), true, vppQueryWait,
		func(value string) bool { return strings.Contains(strings.ToLower(value), "policer-classify") })
	if queryErr != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, queryErr
	}
	if !ok {
		checks = append(checks, failVPPCheck("classify-binding", vppClassifyBindingMissing(scenario, iface), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(scenario, checks), nil
	}
	checks = append(checks, passVPPCheck("applied", vppClassifyApplied(scenario, iface, names)))
	stopVPPProcess(daemon, seen)

	if err := v.writeConfig(work, file, vppTrafficConfig(iface, scenario, false)); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon, seen, err = v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer stopVPPProcess(daemon, seen)
	for _, name := range names {
		ok, text, queryErr = v.awaitPolicer(container, name, false, vppReconcileWait)
		if queryErr != nil {
			return VPPScenarioReport{}, queryErr
		}
		if !ok {
			checks = append(checks, failVPPCheck("cleanup", vppClassifyCleanupMissing(scenario, name), text, seen))
			return finishVPPScenario(scenario, checks), nil
		}
	}
	checks = append(checks, passVPPCheck("cleanup", vppClassifyCleanup(scenario, names)))
	return finishVPPScenario(scenario, checks), nil
}

func (v *VPP) awaitPolicer(container, name string, want bool, timeout time.Duration) (bool, string, error) {
	return v.awaitQuery(container, "show policer", want, timeout,
		func(text string) bool { return strings.Contains(text, name) })
}

func (v *VPP) runFirewall(container, work string) (VPPScenarioReport, error) {
	iface := v.iface
	file := "firewall.conf"
	if err := v.writeConfig(work, file, vppFirewallConfig(true)); err != nil {
		return VPPScenarioReport{}, err
	}
	checks := make([]VPPCheck, 0, 3)
	daemon, seen, err := v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !v.waitDaemonLine(seen, daemon, vppFirewallLine) {
		checks = append(checks, failVPPCheck("applied", "firewall config apply log not observed", "", seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioFirewall, checks), nil
	}
	ok, text, err := v.awaitACL(container, true, vppQueryWait)
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("applied", tbDetail("real VPP ACL with tag ", VPPFirewallACLTag, " not observed after apply"), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioFirewall, checks), nil
	}
	ok, text, err = v.awaitQuery(container, vppCommand("show acl-plugin interface", iface), true, vppQueryWait,
		vppACLBound)
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("bound", tbDetail("real VPP ACL not bound to interface ", iface), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioFirewall, checks), nil
	}
	checks = append(checks, passVPPCheck("applied", tbDetail("real VPP firewall ACL ", VPPFirewallACLTag, " exists and is bound to ", iface)))
	stopVPPProcess(daemon, seen)

	daemon, seen, err = v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	ok, text, err = v.awaitACL(container, true, vppApplyWait)
	if err != nil {
		stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("restart", tbDetail("real VPP ACL ", VPPFirewallACLTag, " missing after ze restart with same config"), text, seen))
		stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioFirewall, checks), nil
	}
	checks = append(checks, passVPPCheck("restart", tbDetail("real VPP firewall ACL ", VPPFirewallACLTag, " survived ze restart with same config")))
	stopVPPProcess(daemon, seen)

	if err := v.writeConfig(work, file, vppFirewallConfig(false)); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon, seen, err = v.startEvidenceDaemon(container, file, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer stopVPPProcess(daemon, seen)
	ok, text, err = v.awaitACL(container, false, vppReconcileWait)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("cleanup", tbDetail("real VPP orphan ACL ", VPPFirewallACLTag, " survived ze restart cleanup"), text, seen))
		return finishVPPScenario(VPPScenarioFirewall, checks), nil
	}
	checks = append(checks, passVPPCheck("cleanup", tbDetail("real VPP startup cleanup removed orphan firewall ACL ", VPPFirewallACLTag)))
	return finishVPPScenario(VPPScenarioFirewall, checks), nil
}

func (v *VPP) awaitACL(container string, want bool, timeout time.Duration) (bool, string, error) {
	return v.awaitQuery(container, "show acl-plugin acl", want, timeout,
		func(text string) bool { return strings.Contains(text, VPPFirewallACLTag) })
}

func vppACLBound(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "input acl") {
		return true
	}
	return strings.Contains(lower, "inbound")
}

func tbDetail(parts ...string) string {
	var tb textbuf.Buffer
	for _, part := range parts {
		tb.Str(part)
	}
	return tb.String()
}

func vppMissingPolicer(name, suffix string) string {
	return tbDetail("real VPP policer ", name, " ", suffix)
}

func vppPolicerDetail(name, action, iface string) string {
	return tbDetail("real VPP traffic policer ", name, " ", action, " ", iface)
}

func vppTrafficApplyMissing(scenario string) string {
	switch scenario {
	case VPPScenarioTrafficProtocol:
		return "traffic-control protocol apply log not observed"
	case VPPScenarioTrafficDSCP:
		return "traffic-control dscp apply log not observed"
	case VPPScenarioTrafficMultiClass:
		return "traffic-control multi-class apply log not observed"
	}
	return "traffic-control apply log not observed"
}

func vppClassifyPolicerMissing(scenario, name string) string {
	kind := "multi-class"
	if scenario == VPPScenarioTrafficProtocol {
		kind = "protocol-filter"
	}
	if scenario == VPPScenarioTrafficDSCP {
		kind = "dscp-filter"
	}
	return tbDetail("real VPP ", kind, " policer ", name, " not observed")
}

func vppClassifyTablesMissing(scenario string) string {
	if scenario == VPPScenarioTrafficDSCP {
		return "real VPP dscp classify tables not observed"
	}
	return "real VPP classify tables not observed after apply"
}

func vppClassifyBindingMissing(scenario, iface string) string {
	if scenario == VPPScenarioTrafficProtocol {
		return tbDetail("policer-classify feature not bound on ", iface, " (R-1: table never attached)")
	}
	if scenario == VPPScenarioTrafficDSCP {
		return tbDetail("policer-classify feature not bound on ", iface, " for dscp (R-1)")
	}
	return tbDetail("policer-classify feature not bound on ", iface, " for multi-class")
}

const vppClassJoin = " + "

func vppClassifyApplied(scenario, iface string, names []string) string {
	if scenario == VPPScenarioTrafficProtocol {
		return tbDetail("real VPP protocol filter programmed classify tables bound to ", iface,
			" steering to policer ", names[0])
	}
	if scenario == VPPScenarioTrafficDSCP {
		return tbDetail("real VPP dscp filter programmed classify tables bound to ", iface,
			" steering to policer ", names[0])
	}
	return tbDetail("real VPP multi-class steering: policers ", names[0], vppClassJoin, names[1],
		" exist, classify bound to ", iface)
}

func vppClassifyCleanupMissing(scenario, name string) string {
	kind := "multi-class"
	if scenario == VPPScenarioTrafficProtocol {
		kind = "protocol-filter"
	}
	if scenario == VPPScenarioTrafficDSCP {
		kind = "dscp-filter"
	}
	return tbDetail("real VPP ", kind, " policer ", name, " survived removal")
}

func vppClassifyCleanup(scenario string, names []string) string {
	if scenario == VPPScenarioTrafficProtocol {
		return tbDetail("real VPP protocol-filter policer ", names[0], " removed on reconcile")
	}
	if scenario == VPPScenarioTrafficDSCP {
		return tbDetail("real VPP dscp-filter policer ", names[0], " removed on reconcile")
	}
	return tbDetail("real VPP multi-class policers ", names[0], vppClassJoin, names[1], " removed on reconcile")
}
