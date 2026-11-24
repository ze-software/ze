// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- real VPP ECMP and stale IP route sweep proofs
// Overview: vppevidence.go -- the shared container and ordered scenario dispatch
// Related: vppevidencescenarios.go -- vppStaticFIBConfig and the fixed prefixes and next-hops
//
// fib-ecmp proves that fib-vpp installs a multi-path ECMP route (NPaths > 1)
// in real VPP under FIB source ze-fib-vpp, updates it in place when one member
// is withdrawn by reload, and removes it when withdrawn completely.
//
// fib-stale-sweep proves that a ze-fib-vpp IP route left behind in VPP after
// an abrupt daemon exit survives the stop and is removed by the restarted
// daemon's stale IP route sweep (routesource.go) after labelSweepDelay while a
// live control route programmed by the new run stays intact.
package testdeployment

import (
	"net/netip"
	"slices"
	"strings"
)

const (
	vppFIBECMPConfigFile        = "fib-ecmp.conf"
	vppFIBStaleBeforeConfigFile = "fib-stale-before.conf"
	vppFIBStaleAfterConfigFile  = "fib-stale-after.conf"
	vppShowIPFIBCommand         = "show ip fib"
)

func (v *VPP) createAddressedLoopback(container, cidr string) error {
	iface, err := v.createLoopback(container)
	if err != nil {
		return err
	}
	if _, err := v.query(container, vppCommand("set interface ip address", iface, cidr)); err != nil {
		return err
	}
	return nil
}

func (v *VPP) runFIBECMP(container, work string) (VPPScenarioReport, error) {
	checks := make([]VPPCheck, 0, 3)
	if err := v.createAddressedLoopback(container, VPPECMPLoopAddressA); err != nil {
		return VPPScenarioReport{}, err
	}
	if err := v.createAddressedLoopback(container, VPPECMPLoopAddressB); err != nil {
		return VPPScenarioReport{}, err
	}

	twoPath := vppStaticFIBConfig([]vppStaticRoute{
		{prefix: VPPECMPPrefix, nextHops: []string{VPPECMPNextHopA, VPPECMPNextHopB}},
	})
	if err := v.writeConfig(work, vppFIBECMPConfigFile, twoPath); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon, seen, err := v.startEvidenceDaemon(container, vppFIBECMPConfigFile, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer v.stopVPPProcess(daemon, seen)

	query := vppCommand(vppShowIPFIBCommand, VPPECMPPrefix)
	wantBoth := []string{VPPECMPNextHopA, VPPECMPNextHopB}
	ok, text, err := v.awaitQuery(container, query, true, v.vppApplyWait(), func(out string) bool {
		return vppFIBHasSourceNextHops(out, VPPECMPPrefix, wantBoth)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("ecmp-installed",
			tbDetail("real VPP FIB does not hold ", VPPECMPPrefix, " under ", VPPFIBSourceName,
				" with both next-hops ", VPPECMPNextHopA, " and ", VPPECMPNextHopB), text, seen))
		return finishVPPScenario(VPPScenarioFIBECMP, checks), nil
	}
	checks = append(checks, passVPPCheck("ecmp-installed",
		tbDetail("real VPP FIB holds ", VPPECMPPrefix, " under ", VPPFIBSourceName,
			" with ECMP paths ", VPPECMPNextHopA, " and ", VPPECMPNextHopB), tailLines(text)...))

	singlePath := vppStaticFIBConfig([]vppStaticRoute{
		{prefix: VPPECMPPrefix, nextHops: []string{VPPECMPNextHopA}},
	})
	if err := v.reloadEvidenceConfig(container, vppFIBECMPConfigFile, singlePath, daemon); err != nil {
		return VPPScenarioReport{}, err
	}
	wantOne := []string{VPPECMPNextHopA}
	ok, text, err = v.awaitQuery(container, query, true, vppWithdrawWait, func(out string) bool {
		return vppFIBHasSourceNextHops(out, VPPECMPPrefix, wantOne)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("member-withdrawn",
			tbDetail("real VPP FIB did not update ", VPPECMPPrefix, " under ", VPPFIBSourceName,
				" to surviving path ", VPPECMPNextHopA, " alone"), text, seen))
		return finishVPPScenario(VPPScenarioFIBECMP, checks), nil
	}
	checks = append(checks, passVPPCheck("member-withdrawn",
		tbDetail("real VPP FIB updated ", VPPECMPPrefix, " under ", VPPFIBSourceName,
			" to retain surviving path ", VPPECMPNextHopA, " after removing ", VPPECMPNextHopB), tailLines(text)...))

	if err := v.reloadEvidenceConfig(container, vppFIBECMPConfigFile, vppStaticFIBConfig(nil), daemon); err != nil {
		return VPPScenarioReport{}, err
	}
	ok, text, err = v.awaitQuery(container, query, false, vppWithdrawWait, func(out string) bool {
		return vppFIBHasSource(out, VPPECMPPrefix)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("withdrawn",
			tbDetail("real VPP FIB still holds ", VPPECMPPrefix, " under ", VPPFIBSourceName,
				" after the reload that removed the static route"), text, seen))
		return finishVPPScenario(VPPScenarioFIBECMP, checks), nil
	}
	checks = append(checks, passVPPCheck("withdrawn",
		tbDetail("real VPP FIB withdrew ", VPPECMPPrefix, " under ", VPPFIBSourceName)))
	return finishVPPScenario(VPPScenarioFIBECMP, checks), nil
}

func (v *VPP) runFIBStaleSweep(container, work string) (VPPScenarioReport, error) {
	checks := make([]VPPCheck, 0, 5)
	if err := v.createAddressedLoopback(container, VPPStaleSweepLoopAddress); err != nil {
		return VPPScenarioReport{}, err
	}

	beforeCfg := vppStaticFIBConfig([]vppStaticRoute{
		{prefix: VPPStaleSweepPrefix, nextHops: []string{VPPStaleSweepNextHop}},
	})
	if err := v.writeConfig(work, vppFIBStaleBeforeConfigFile, beforeCfg); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon, seen, err := v.startEvidenceDaemon(container, vppFIBStaleBeforeConfigFile, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}

	staleQuery := vppCommand(vppShowIPFIBCommand, VPPStaleSweepPrefix)
	wantHop := []string{VPPStaleSweepNextHop}
	ok, text, err := v.awaitQuery(container, staleQuery, true, v.vppApplyWait(), func(out string) bool {
		return vppFIBHasSourceNextHops(out, VPPStaleSweepPrefix, wantHop)
	})
	if err != nil {
		v.stopVPPProcess(daemon, seen)
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("installed",
			tbDetail("real VPP FIB does not hold ", VPPStaleSweepPrefix, " under ", VPPFIBSourceName,
				" via ", VPPStaleSweepNextHop), text, seen))
		v.stopVPPProcess(daemon, seen)
		return finishVPPScenario(VPPScenarioFIBStaleSweep, checks), nil
	}
	checks = append(checks, passVPPCheck("installed",
		tbDetail("real VPP FIB holds ", VPPStaleSweepPrefix, " under ", VPPFIBSourceName,
			" via ", VPPStaleSweepNextHop), tailLines(text)...))

	// Kill Ze abruptly so static's graceful shutdown cannot withdraw the route
	// before exit, leaving a genuine ze-fib-vpp orphan behind in VPP.
	v.killVPPProcess(daemon, seen)
	if err := v.checkDaemonsStopped(); err != nil {
		return VPPScenarioReport{}, err
	}

	text, err = v.query(container, staleQuery)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !vppFIBHasSourceNextHops(text, VPPStaleSweepPrefix, wantHop) {
		checks = append(checks, failVPPCheck("survived-stop",
			tbDetail("route ", VPPStaleSweepPrefix, " did not remain in real VPP FIB under ",
				VPPFIBSourceName, " after ze exited"), text, seen))
		return finishVPPScenario(VPPScenarioFIBStaleSweep, checks), nil
	}
	checks = append(checks, passVPPCheck("survived-stop",
		tbDetail("stale route ", VPPStaleSweepPrefix, " remained in real VPP FIB under ",
			VPPFIBSourceName, " after ze exited")))

	afterCfg := vppStaticFIBConfig([]vppStaticRoute{
		{prefix: VPPStaleSweepControlPrefix, nextHops: []string{VPPStaleSweepNextHop}},
	})
	if err := v.writeConfig(work, vppFIBStaleAfterConfigFile, afterCfg); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon2, seen2, err := v.startEvidenceDaemon(container, vppFIBStaleAfterConfigFile, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer v.stopVPPProcess(daemon2, seen2)

	controlQuery := vppCommand(vppShowIPFIBCommand, VPPStaleSweepControlPrefix)
	ok, text, err = v.awaitQuery(container, controlQuery, true, v.vppApplyWait(), func(out string) bool {
		return vppFIBHasSourceNextHops(out, VPPStaleSweepControlPrefix, wantHop)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("control-installed",
			tbDetail("restarted ze did not program live control route ", VPPStaleSweepControlPrefix,
				" under ", VPPFIBSourceName), text, seen2))
		return finishVPPScenario(VPPScenarioFIBStaleSweep, checks), nil
	}
	checks = append(checks, passVPPCheck("control-installed",
		tbDetail("restarted ze programmed live control route ", VPPStaleSweepControlPrefix,
			" under ", VPPFIBSourceName, " via ", VPPStaleSweepNextHop)))

	ok, text, err = v.awaitQuery(container, staleQuery, false, vppStaleSweepWait, func(out string) bool {
		return vppFIBHasSource(out, VPPStaleSweepPrefix)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("stale-swept",
			tbDetail("real VPP FIB still holds stale route ", VPPStaleSweepPrefix, " under ",
				VPPFIBSourceName, " after the stale sweep window"), text, seen2))
		return finishVPPScenario(VPPScenarioFIBStaleSweep, checks), nil
	}
	checks = append(checks, passVPPCheck("stale-swept",
		tbDetail("real VPP stale IP route sweep removed ", VPPStaleSweepPrefix, " under ",
			VPPFIBSourceName, " across ze restart")))

	text, err = v.query(container, controlQuery)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !vppFIBHasSourceNextHops(text, VPPStaleSweepControlPrefix, wantHop) {
		checks = append(checks, failVPPCheck("control-preserved",
			tbDetail("live control route ", VPPStaleSweepControlPrefix, " disappeared from real VPP FIB under ",
				VPPFIBSourceName, " during the stale sweep"), text, seen2))
		return finishVPPScenario(VPPScenarioFIBStaleSweep, checks), nil
	}
	checks = append(checks, passVPPCheck("control-preserved",
		tbDetail("live control route ", VPPStaleSweepControlPrefix, " stayed present in real VPP FIB under ",
			VPPFIBSourceName, " after the stale sweep")))

	// Withdraw the control route cleanly so subsequent scenarios inherit a clean FIB.
	if err := v.reloadEvidenceConfig(container, vppFIBStaleAfterConfigFile, vppStaticFIBConfig(nil), daemon2); err != nil {
		return VPPScenarioReport{}, err
	}
	_, _, err = v.awaitQuery(container, controlQuery, false, vppWithdrawWait, func(out string) bool {
		return vppFIBHasSource(out, VPPStaleSweepControlPrefix)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	return finishVPPScenario(VPPScenarioFIBStaleSweep, checks), nil
}

// vppFIBSourceNextHops parses `show ip fib <prefix>` output from VPP's
// format_fib_entry (src/vnet/fib/fib_entry.c, FDio/vpp v26.06) and returns the
// sorted next-hop IP addresses contributed by source for the exact prefix, and
// whether source contributes to that prefix at all.
//
// Only the two-space source block whose name matches source before the
// `forwarding:` DPO section is read, so another prefix (including the 0.0.0.0/0
// LPM fallback VPP prints after a route is removed), another FIB source, or
// the forwarding chain cannot answer for source.
func vppFIBSourceNextHops(output, prefix, source string) ([]string, bool) {
	normalized := strings.ReplaceAll(output, "\r\n", "\n")
	inEntry := false
	inSource := false
	expectNextHop := false
	foundSource := false
	var nextHops []string

	for line := range strings.SplitSeq(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if line[0] != ' ' {
			if line[0] != '\t' {
				before, _, isHeader := strings.Cut(line, " fib:")
				inEntry = isHeader && strings.TrimSpace(before) == prefix
				inSource = false
				expectNextHop = false
				continue
			}
		}
		if !inEntry {
			continue
		}
		if strings.HasPrefix(trimmed, "forwarding:") {
			inSource = false
			expectNextHop = false
			continue
		}
		if strings.HasPrefix(line, "  ") {
			if !strings.HasPrefix(line, "   ") {
				inSource = false
				expectNextHop = false
				fields := strings.Fields(trimmed)
				if len(fields) >= 2 {
					if strings.HasPrefix(fields[1], "refs:") {
						if fields[0] == source {
							inSource = true
							foundSource = true
						}
					}
				}
				continue
			}
		}
		if !inSource {
			continue
		}
		if strings.HasPrefix(trimmed, "path:[") {
			expectNextHop = true
			continue
		}
		if expectNextHop {
			expectNextHop = false
			if hop, ok := vppFIBPathNextHop(trimmed); ok {
				nextHops = append(nextHops, hop)
			}
		}
	}
	slices.Sort(nextHops)
	return nextHops, foundSource
}

// vppFIBPathNextHop extracts the next-hop IP address from the detail line VPP
// prints below a `path:[...]` header: `via <ip> in fib:0 ...` for a recursive
// path or `<ip> <iface>` for an attached-nexthop path.
func vppFIBPathNextHop(trimmed string) (string, bool) {
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", false
	}
	candidate := fields[0]
	if fields[0] == "via" {
		if len(fields) < 2 {
			return "", false
		}
		candidate = fields[1]
	}
	addr, err := netip.ParseAddr(candidate)
	if err != nil {
		return "", false
	}
	return addr.String(), true
}

// vppFIBHasSource reports whether `show ip fib <prefix>` lists
// VPPFIBSourceName as a contributor to the exact prefix.
func vppFIBHasSource(output, prefix string) bool {
	_, found := vppFIBSourceNextHops(output, prefix, VPPFIBSourceName)
	return found
}

// vppFIBHasSourceNextHops reports whether `show ip fib <prefix>` lists
// VPPFIBSourceName as a contributor to the exact prefix with the exact set of
// next-hop addresses in want.
func vppFIBHasSourceNextHops(output, prefix string, want []string) bool {
	got, found := vppFIBSourceNextHops(output, prefix, VPPFIBSourceName)
	if !found {
		return false
	}
	expected := slices.Clone(want)
	slices.Sort(expected)
	return slices.Equal(got, expected)
}
