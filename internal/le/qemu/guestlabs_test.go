//go:build linux

package qemu

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGuestEvidenceActionsAreNativeAndGateless(t *testing.T) {
	t.Parallel()
	list := Actions()
	wanted := map[string]bool{
		"vrrp-keepalived-test": false,
		"pppoe-accel-test":     false,
		"netns-test":           false,
		"pppoe-test":           false,
	}
	for _, row := range list.Actions {
		if _, ok := wanted[row.Verb]; !ok {
			continue
		}
		if row.Gate != "" {
			t.Fatalf("guest action %s unexpectedly claims gate %s", row.Verb, row.Gate)
		}
		if len(row.Forks) != 0 {
			t.Fatalf("guest action %s still forks %v", row.Verb, row.Forks)
		}
		wanted[row.Verb] = true
	}
	for verb, found := range wanted {
		if !found {
			t.Errorf("missing guest action %s", verb)
		}
	}
}

func TestClosedSelectionsValidateEveryNameBeforeEffects(t *testing.T) {
	t.Parallel()
	got, err := parseClosedCSV("QS-3,QS-1", "scenario", vrrpScenarioNames, vrrpScenarioNames)
	if err != nil || !slices.Equal(got, []string{"QS-3", "QS-1"}) {
		t.Fatalf("selected %v, %v", got, err)
	}
	for _, raw := range []string{"QS-1,QS-9", "QS-1,QS-1", "QS-1,"} {
		if _, err := parseClosedCSV(raw, "scenario", vrrpScenarioNames, vrrpScenarioNames); err == nil {
			t.Errorf("selection %q was accepted", raw)
		}
	}
	defaults, err := parseClosedCSV("", "suite", netnsSuiteNames, defaultNetnsSuites)
	if err != nil || !slices.Equal(defaults, defaultNetnsSuites) {
		t.Fatalf("defaults %v, %v", defaults, err)
	}
}

func TestGuestVerdictZeroNeverExitsSuccessfully(t *testing.T) {
	t.Parallel()
	if _, code := finishGuestLab(GuestLabReport{Lab: "netns"}, nil); code == 0 {
		t.Fatal("an unspecified verdict exited zero")
	}
	if _, code := finishGuestLab(GuestLabReport{Lab: "netns", Verdict: VerdictFail}, nil); code == 0 {
		t.Fatal("a failed verdict exited zero")
	}
	if _, code := finishGuestLab(GuestLabReport{Lab: "netns", Verdict: VerdictPass}, nil); code != 0 {
		t.Fatalf("pass exited %d", code)
	}
}

func TestGuestFailureReportRetainsArtifacts(t *testing.T) {
	t.Parallel()
	report := GuestLabReport{
		Lab: "vrrp-keepalived", Verdict: VerdictFail,
		Artifacts: []string{"/tmp/effective-vrrp-qs-2/observer.pcap", "/tmp/effective-vrrp-qs-2/ze.conf"},
		Scenarios: []GuestScenario{{Name: vrrpQS2, Verdict: VerdictFail, Artifacts: []string{"observer.pcap"}}},
	}
	if len(report.Artifacts) != 2 || len(report.Scenarios[0].Artifacts) != 1 {
		t.Fatalf("failure lost artifacts: %#v", report)
	}
	pass := GuestLabReport{Lab: "vrrp-keepalived", Verdict: VerdictPass}
	if len(pass.Artifacts) != 0 {
		t.Fatalf("pass retained artifacts: %#v", pass.Artifacts)
	}
}

func TestVRRPConstantsAndConfigBytes(t *testing.T) {
	t.Parallel()
	if vrrpVRID != 10 || vrrpAdvertMS != 1000 || vrrpAdvertCS != 100 || vrrpVirtualMAC != "00:00:5e:00:01:0a" {
		t.Fatal("VRRP wire constants drifted")
	}
	if vrrpCaptureReadyTimeout != 20*time.Second || vrrpZeStartTimeout != 45*time.Second ||
		vrrpZeMasterTimeout != 45*time.Second || vrrpKAStateTimeout != 30*time.Second ||
		vrrpWireEventTimeout != 30*time.Second || vrrpKAGARPTimeout != 25*time.Second ||
		vrrpPingTimeout != 20*time.Second {
		t.Fatal("VRRP wait boundaries drifted")
	}
	if vrrpMasterDown(vrrpZePriority) != 3.21875 || vrrpMasterDown(vrrpKAPriority) != 3.609375 {
		t.Fatal("RFC 9568 timing constants drifted")
	}
	if vrrpQS2PromoteMin != 3 || vrrpQS2PromoteMax != 6 ||
		vrrpQS2PreemptMin != 2.8 || vrrpQS2PreemptMax != 6 ||
		vrrpQS3PromoteMax != 3 || vrrpQS3NoSkewPath != 3.61 {
		t.Fatal("VRRP timing bands drifted")
	}
	names := vrrpNames{zeVeth: "zvz123", kaVeth: "zvk123"}
	config := string(vrrpZeConfig(names))
	for _, exact := range []string{
		"ethernet zvz123 {", "address [ 192.0.2.251/24 ];", "vrid 10;",
		"virtual-address [ 192.0.2.1 ];", "priority 200;", "preempt true;",
		"accept-mode true;", "advertise-interval-milliseconds 1000;",
	} {
		if !strings.Contains(config, exact) {
			t.Errorf("ze config lost %q\n%s", exact, config)
		}
	}
	keepalived := string(vrrpKeepalivedConfig(names, "/root/notify.sh", "/root/ka-state.log", vrrpKAPriority))
	for _, exact := range []string{"vrrp_version 3", "script_user root", "interface zvk123", "advert_int 1", `notify_master "/root/notify.sh MASTER /root/ka-state.log"`} {
		if !strings.Contains(keepalived, exact) {
			t.Errorf("keepalived config lost %q\n%s", exact, keepalived)
		}
	}
}

func TestVRRPCaptureParsersKeepFullWireFields(t *testing.T) {
	t.Parallel()
	lines := []string{
		"1752580000.123456 00:00:5e:00:01:0a > 01:00:5e:00:00:12, ethertype IPv4 (0x0800), length 54: (ttl 255, proto VRRP (112))\n",
		"    192.0.2.251 > 224.0.0.18: VRRPv3, Advertisement, vrid 10, prio 200, intvl 100cs, length 12\n",
		"1752580000.500000 00:00:5e:00:01:0a > ff:ff:ff:ff:ff:ff, ethertype ARP (0x0806), length 42: Request who-has 192.0.2.1 (00:00:5e:00:01:0a) tell 192.0.2.1, length 28\n",
	}
	adverts := parseVRRPAdverts(lines)
	if len(adverts) != 1 || adverts[0].ttl != 255 || adverts[0].interval != 100 || adverts[0].intervalUnit != "cs" {
		t.Fatalf("adverts %#v", adverts)
	}
	garps := parseVRRPGARPs(lines)
	if len(garps) != 1 || garps[0].senderIP != vrrpVIP || garps[0].targetMAC != vrrpVirtualMAC {
		t.Fatalf("garps %#v", garps)
	}
}

func TestPPPoEConfigBytesAndDottedEnvironment(t *testing.T) {
	t.Parallel()
	if pppoeAccelStartupWait != 2*time.Second || pppoeZeSessionTimeout != 75*time.Second ||
		pppoeACSessionTimeout != 30*time.Second || pppoeAddressTimeout != 20*time.Second ||
		pppoeTeardownTimeout != 20*time.Second {
		t.Fatal("PPPoE wait boundaries drifted")
	}
	work := "/tmp/effective-pppoe-accel-x"
	accel := string(pppoeAccelConfig(work, "zpoea1"))
	for _, exact := range []string{
		"auth_chap_md5\n", "thread-count=1\n", "interface=zpoea1\n", "service-name=internet\n",
		"mtu=1492\n", "gw-ip-address=10.11.0.1\n", "10.11.0.2-10\n",
		"chap-secrets=" + work + "/chap-secrets\n",
	} {
		if !strings.Contains(accel, exact) {
			t.Errorf("accel config lost %q", exact)
		}
	}
	ze := string(pppoeZeConfig("zpoez1"))
	for _, exact := range []string{"source-interface zpoez1;", "username alice;", "password s3cr3t;"} {
		if !strings.Contains(ze, exact) {
			t.Errorf("ze config lost %q", exact)
		}
	}
	environ := withGuestEnv([]string{"PATH=/bin"}, map[string]string{"ze.log.interface": "debug", "ze.config.dir": "/tmp/zestate"})
	joined := strings.Join(environ, "\n")
	if !strings.Contains(joined, "ze.log.interface=debug") || !strings.Contains(joined, "ze.config.dir=/tmp/zestate") {
		t.Fatalf("dotted keys changed: %v", environ)
	}
	if strings.Contains(joined, "ze-log-interface") || strings.Contains(joined, "ze-config-dir") {
		t.Fatalf("dotted keys became dashed: %v", environ)
	}
}

func TestPPPoELinkDifferenceDetectsMissingAndDuplicateLinks(t *testing.T) {
	t.Parallel()
	initial := map[string]bool{"ppp0": true}
	if got := newLinks(map[string]bool{"ppp0": true}, initial); len(got) != 0 {
		t.Fatalf("missing-link case returned %v", got)
	}
	got := newLinks(map[string]bool{"ppp0": true, "ppp2": true, "ppp1": true}, initial)
	if !slices.Equal(got, []string{"ppp1", "ppp2"}) {
		t.Fatalf("duplicate-link case returned %v", got)
	}
}

func TestNetnsSelectionsHaveAbsolutePopulations(t *testing.T) {
	t.Parallel()
	want := map[string]int{netnsFirewall: 22, netnsPolicy: 6, netnsOSPF: 8, netnsOSPFv3: 3, netnsPPPoE: 3}
	for suite, count := range want {
		if got := len(netnsSelections[suite]); got != count {
			t.Errorf("%s has %d selectors, want %d", suite, got, count)
		}
	}
}

func TestNetnsRunsAllSuitesAfterRed(t *testing.T) {
	t.Parallel()
	called := make([]string, 0)
	run := func(_ context.Context, _ netnsBinaries, suite string, _ []string) int {
		called = append(called, suite)
		if suite == netnsFirewall {
			return 9
		}
		return 0
	}
	selected := []string{netnsFirewall, netnsPolicy, netnsOSPF}
	results := runEveryNetnsSuite(context.Background(), netnsBinaries{}, selected, run)
	if !slices.Equal(called, selected) || len(results) != len(selected) {
		t.Fatalf("called %v, results %#v", called, results)
	}
	if results[0].Verdict != VerdictFail || results[0].Failure != "9" || results[2].Verdict != VerdictPass {
		t.Fatalf("results %#v", results)
	}
}

func TestGuestWaitIsBounded(t *testing.T) {
	t.Parallel()
	started := time.Now()
	err := waitGuest(context.Background(), 10*time.Millisecond, time.Millisecond, func() (bool, error) { return false, nil })
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("bounded wait returned %v after %s", err, time.Since(started))
	}
}

func TestGuestProcessStopReapsOwnedProcess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	process, _, err := startGuestProcess(ctx, "", []string{"sh", "-c", "sleep 60"}, os.Environ(), "peer> ")
	if err != nil {
		t.Fatalf("start peer: %v", err)
	}
	process.stop()
	if !process.exited() {
		t.Fatal("owned process remained after stop")
	}
}

func TestProbeSkipKeysAreBothRefused(t *testing.T) {
	for _, key := range []string{"ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "1")
			if err := rejectPPPoEProbeSkip(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("probe skip returned %v", err)
			}
		})
	}
	_ = os.Unsetenv("ZE_PPPOE_SKIP_KERNEL_PROBE")
	_ = os.Unsetenv("ze.pppoe.skip-kernel-probe")
}
