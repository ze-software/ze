package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

// TestCheckerPopulationMatchesProducer validates that adding any scenario without
// a native checker makes the package fail closed instead of silently shrinking the gate.
func TestCheckerPopulationMatchesProducer(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "test", "interop", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	producer := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(root, "test", "interop", "scenarios", entry.Name(), "check.py")); statErr == nil {
			producer = append(producer, entry.Name())
		}
	}
	sort.Strings(producer)
	checkers := Checkers()
	native := make([]string, 0, len(checkers))
	for name, checker := range checkers {
		if checker == nil {
			t.Errorf("checker %s is nil", name)
		}
		if len(scenarioOperations[name]) == 0 && specialCheckers[name] == nil {
			t.Errorf("checker %s has no typed assertion", name)
		}
		native = append(native, name)
	}
	for name := range specialCheckers {
		if _, generic := scenarioOperations[name]; generic {
			t.Errorf("bespoke checker %s still has a generic fallback", name)
		}
		if _, generic := scenarioExtras[name]; generic {
			t.Errorf("bespoke checker %s still has generic extra operations", name)
		}
		if len(specialAuditOperations[name]) == 0 {
			t.Errorf("bespoke checker %s has no exact audit mapping", name)
		}
	}
	sort.Strings(native)
	if strings.Join(native, "\n") != strings.Join(producer, "\n") {
		t.Fatalf("native scenario population differs from producer\nnative: %v\nproducer: %v", native, producer)
	}
}

// TestEveryScenarioOperationUsesRecorder drives every registered checker operation
// against measured peer output. It catches an operation branch that stops querying.
func TestEveryScenarioOperationUsesRecorder(t *testing.T) {
	network := interoplab.Network{
		Name: "recorded",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	covered := make(map[operationKind]int)
	for name, operations := range scenarioOperations {
		operations := append(append([]operation(nil), operations...), scenarioExtras[name]...)
		t.Run(name, func(t *testing.T) {
			for index := range operations {
				current := &operations[index]
				rewriteOperation(network, current)
				recorder := recorderFor(current)
				if current.kind == opDelayRequireContains {
					current.delay = time.Nanosecond
				}
				err := runOperation(t.Context(), network, recorder, current)
				if err != nil {
					t.Fatalf("operation %d (%d): %v", index+1, current.kind, err)
				}
				if recorder.reads == 0 && current.kind != opExec && current.kind != opSignal && current.kind != opStart {
					t.Fatalf("operation %d read no peer output", index+1)
				}
				covered[current.kind]++
			}
		})
	}
	for _, branch := range []operationKind{
		opFRRSession, opBIRDSession, opGoBGPSession, opFRRRoute, opFRRRouteAbsent,
		opBIRDRoute, opGoBGPRoute, opFRRCommunity, opFRRNoAS, opBIRDNoAS,
		opWaitContains, opWaitContainsAny, opWaitAbsent, opRequireContains, opRequireAbsent,
		opRequireJSONFields, opWaitJSONFields, opExec, opSignal, opStart,
		opWaitLogFields, opWaitLogContains, opDelayRequireContains,
	} {
		if covered[branch] == 0 {
			t.Errorf("operation branch %d has no scenario fixture", branch)
		}
	}
}

// TestEveryScenarioOperationRejectsContradictoryRecorder proves each typed
// operation rejects missing required state or present forbidden state.
func TestEveryScenarioOperationRejectsContradictoryRecorder(t *testing.T) {
	network := interoplab.Network{
		Name: "contradictory",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	for name, operations := range scenarioOperations {
		operations := append(append([]operation(nil), operations...), scenarioExtras[name]...)
		t.Run(name, func(t *testing.T) {
			for index := range operations {
				current := &operations[index]
				rewriteOperation(network, current)
				recorder := contradictoryRecorderFor(current)
				current.timeout = time.Nanosecond
				if current.kind == opDelayRequireContains {
					current.delay = time.Nanosecond
				}
				if err := runOperation(t.Context(), network, recorder, current); err == nil {
					t.Fatalf("operation %d (%d) accepted contradictory peer state", index+1, current.kind)
				}
			}
		})
	}
}

func TestRequiredOperationBranchScenarioMappings(t *testing.T) {
	for branch, scenario := range map[operationKind]string{
		opStart:           "vrrp-mastership-keepalived",
		opWaitContainsAny: "ospf-nbma-frr",
	} {
		found := false
		operations := append(append([]operation(nil), scenarioOperations[scenario]...), scenarioExtras[scenario]...)
		for index := range operations {
			if operations[index].kind == branch {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("operation branch %d has no real mapping in scenario %s", branch, scenario)
		}
	}
}

// TestNegativeAssertionsRequirePositiveProof verifies both failure branches of
// an absence check: the forbidden state and a mechanism that never ran.
func TestNegativeAssertionsRequirePositiveProof(t *testing.T) {
	if err := requireAbsentWithProof("session Established", []string{"NOTIFICATION"}, nil); err == nil {
		t.Fatal("absence assertion without positive proof passed")
	}
	if err := requireAbsentWithProof("session Idle", []string{"NOTIFICATION"}, []string{"Established"}); err == nil {
		t.Fatal("absence assertion passed without proof that the session ran")
	}
	if err := requireAbsentWithProof("Established NOTIFICATION", []string{"NOTIFICATION"}, []string{"Established"}); err == nil {
		t.Fatal("absence assertion accepted the forbidden state")
	}
	if err := requireAbsentWithProof("Established route received", []string{"NOTIFICATION"}, []string{"Established", "route received"}); err != nil {
		t.Fatalf("measured absence failed: %v", err)
	}
}

// TestScenarioPreparerBuildsOrderedPeers validates config discovery, immutable
// mounts, rebased speaker addressing, and the legacy participant order.
func TestScenarioPreparerBuildsOrderedPeers(t *testing.T) {
	producer := t.TempDir()
	scenario := t.TempDir()
	writeFixture(t, filepath.Join(scenario, "ze.conf"), "bgp {}\n")
	writeFixture(t, filepath.Join(scenario, "frr.conf"), "router bgp 65002\n")
	writeFixture(t, filepath.Join(scenario, "bird.conf"), "protocol device {}\n")
	writeFixture(t, filepath.Join(scenario, "gobgp.toml"), "[global.config]\n")
	writeFixture(t, filepath.Join(scenario, "speaker-args"), "--asn 65010\n")
	writeFixture(t, filepath.Join(scenario, "announce.py"), "print('ready')\n")
	if err := os.Mkdir(filepath.Join(producer, "speaker"), 0o750); err != nil {
		t.Fatal(err)
	}
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.31.22.0/24")}
	peers, err := scenarioPeers(producer, scenario, "fixture", network)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(peers))
	for _, peer := range peers {
		got = append(got, peer.Name)
	}
	want := []string{"ze", "speaker", "frr", "bird", "gobgp"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("peer order = %v, want %v", got, want)
	}
	if joined := strings.Join(peers[1].Command, " "); !strings.Contains(joined, "172.31.22.2:179") {
		t.Fatalf("speaker command did not use selected network: %s", joined)
	}
	foundPlugin := false
	for _, mount := range peers[0].Mounts {
		if mount.Target == "/etc/ze/announce.py" && mount.ReadOnly {
			foundPlugin = true
		}
	}
	if !foundPlugin {
		t.Fatal("ze peer did not mount the scenario plugin read-only")
	}
}

// TestSpecialCheckerParsers validates the structured answers used by the two
// non-linear checkers, including the exact ADD-PATH wire comparison token.
func TestSpecialCheckerParsers(t *testing.T) {
	recorder := &recordingLab{output: `{"count":256}`}
	count, err := zeRIBCount(t.Context(), recorder)
	if err != nil || count != 256 {
		t.Fatalf("RIB count = %d, %v", count, err)
	}
	recorder.output = `{"adj-rib-in":{"peer":[{"prefix":"10.0.0.0/24"},{"prefix":"10.0.1.0/24"}]}}`
	count, err = zeRIBDocumentCount(t.Context(), recorder)
	if err != nil || count != 2 {
		t.Fatalf("RIB document count = %d, %v", count, err)
	}
	rows := make([]map[string]string, 50)
	for index := range rows {
		rows[index] = map[string]string{"prefix": fmt.Sprintf("10.0.%d.0/24", index)}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	recorder.output = string(data)
	count, err = zeRIBBestRows(t.Context(), recorder)
	if err != nil || count != 50 {
		t.Fatalf("best rows = %d, %v", count, err)
	}
	logs := "established: yes\nresult: PASS\nnote: update-hex: 0000000000000007180a6300\n"
	update, err := speakerRouteUpdate(logs, "speaker")
	if err != nil || fmt.Sprintf("%x", update) != "0000000000000007180a6300" {
		t.Fatalf("route-bearing update = %x, %v", update, err)
	}
}

// TestBespokeCheckerBranches pins every non-linear checker predicate that a
// generic presence assertion cannot represent.
func TestBespokeCheckerBranches(t *testing.T) {
	t.Run("bfd-frr", func(t *testing.T) {
		if bfdSessionDown("BGP state = Established") {
			t.Fatal("Established BGP session was reported down")
		}
		if !bfdSessionDown("BGP state = Idle") {
			t.Fatal("non-Established BGP session was not reported down")
		}
		if err := requireBFDTeardownBudget(1999 * time.Millisecond); err != nil {
			t.Fatalf("sub-two-second teardown failed: %v", err)
		}
		if err := requireBFDTeardownBudget(2 * time.Second); err == nil {
			t.Fatal("two-second teardown passed the strict budget")
		}
	})

	t.Run("bgp-addpath-rail-agreement-speaker", func(t *testing.T) {
		const update = "0000000000000007180a6300"
		logs := "established: yes\nresult: PASS\nnote: update-hex: " + update + "\n"
		body, err := speakerRouteUpdate(logs, "speaker")
		if err != nil || fmt.Sprintf("%x", body) != update {
			t.Fatalf("valid ADD-PATH UPDATE = %x, %v", body, err)
		}
		if _, err := speakerRouteUpdate(logs+"note: update-hex: "+update+"\n", "speaker"); err == nil {
			t.Fatal("two route-bearing UPDATEs passed the exact-one branch")
		}
		bare := "established: yes\nresult: PASS\nnote: update-hex: 00000000180a6300\n"
		if _, err := speakerRouteUpdate(bare, "speaker"); err == nil {
			t.Fatal("bare, non-ADD-PATH NLRI passed")
		}
	})

	t.Run("bgp-holdtime-deadpeer-frr", func(t *testing.T) {
		if holdNotificationSeen("notification sent\nhold timer expired") {
			t.Fatal("hold notification matched tokens from different log lines")
		}
		if !holdNotificationSeen("NOTIFICATION sent: Hold Timer Expired") {
			t.Fatal("hold notification did not match one complete peer event")
		}
		if frrReceivedHoldExpiry(map[string]any{"lastResetDueTo": "BGP Notification send", "lastNotificationReason": "Hold Timer Expired"}) {
			t.Fatal("FRR-originated notification passed as a received notification")
		}
		if !frrReceivedHoldExpiry(map[string]any{"lastResetDueTo": "BGP Notification receive", "lastNotificationReason": "Hold Timer Expired"}) {
			t.Fatal("received hold expiry was rejected")
		}
	})

	t.Run("bgp-max-prefix-per-family-frr", func(t *testing.T) {
		if !maxPrefixWarnOnlyDecision("prefix count exceeded maximum family=ipv4/unicast teardown=false") {
			t.Fatal("complete warn-only decision was rejected")
		}
		if maxPrefixWarnOnlyDecision("prefix count exceeded maximum\nfamily=ipv4/unicast teardown=false") {
			t.Fatal("decision tokens on separate lines passed")
		}
		if maxPrefixWarnOnlyDecision("prefix count exceeded maximum family=ipv4/unicast teardown=true") {
			t.Fatal("teardown family passed warn-only decision")
		}
	})

	t.Run("bgp-relay-withdraw-reflector-frr", func(t *testing.T) {
		network := interoplab.Network{IPv4: netip.MustParsePrefix("172.30.44.0/24")}
		originator, cluster := reflectorAttributeTokens(network)
		if originator != "800904AC1E2C03" || cluster != "800A04AC1E2C02" {
			t.Fatalf("reflector attributes = %s, %s", originator, cluster)
		}
		if err := requireNoEarlyReflectorWithdrawal("advertisement only"); err != nil {
			t.Fatalf("clean pre-withdrawal log failed: %v", err)
		}
		if err := requireNoEarlyReflectorWithdrawal(":02:0004180a14000000"); err == nil {
			t.Fatal("early withdrawal body passed")
		}
	})

	t.Run("bgp-wire-edit-api-origin-bird", func(t *testing.T) {
		recorder := &recordingLab{output: "10.55.0.0/24\n BGP.community: (65001,100) (65001,200)\n BGP.large_community: (65001, 0, 1)\n"}
		if err := checkBIRDAPICommunities(t.Context(), recorder, "10.55.0.0/24"); err != nil {
			t.Fatalf("scoped community lines failed: %v", err)
		}
		recorder.output = "10.55.0.0/24\n BGP.community: (65001,100)\n other: (65001,200)\n BGP.large_community: (65001, 0, 1)\n"
		if err := checkBIRDAPICommunities(t.Context(), recorder, "10.55.0.0/24"); err == nil {
			t.Fatal("community on the wrong attribute line passed")
		}
		recorder.logs = "ZE-OBSERVER-FAIL: queue rail lost"
		if err := observerFailure(t.Context(), recorder); err == nil {
			t.Fatal("observer failure sentinel passed")
		}
	})

	t.Run("isis-purge-reorig-frr", func(t *testing.T) {
		const golden = "831b010612010000001b0000000000000002000000001000c02c01"
		pdu := buildISISL1Purge([6]byte{0, 0, 0, 0, 0, 2}, 4096, 0, 0)
		if got := fmt.Sprintf("%x", pdu); got != golden {
			t.Fatalf("purge PDU = %s, want %s", got, golden)
		}
		frame, err := buildISISEthernetFrame(net.HardwareAddr{2, 66, 172, 30, 44, 2}, pdu[:])
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", frame[:17]); got != "0180c20000140242ac1e2c02001efefe03" {
			t.Fatalf("802.3/LLC header = %s", got)
		}
		recorder := &recordingLab{}
		called := false
		err = injectISISOwnLSPPurge(t.Context(), recorder, func(pid int, interfaceName string, sent []byte) error {
			called = true
			if pid != 4242 || interfaceName != "eth0" || fmt.Sprintf("%x", sent) != golden {
				t.Fatalf("inject call = pid %d interface %s pdu %x", pid, interfaceName, sent)
			}
			return nil
		})
		if err != nil || !called {
			t.Fatalf("rootless purge injection seam = called %v, err %v", called, err)
		}
		recorder.output = "ze-purge.00-00 83 0x00001001 0xc530 1176 0/0/0"
		row, err := queryISISLSP(t.Context(), recorder)
		if err != nil || row.sequence != 4097 || row.holdtime != 1176 || row.pduLen != 83 {
			t.Fatalf("IS-IS LSP row = %+v, %v", row, err)
		}
		recorder.output = "ze-purge.00-00 27 0x00001000 0xc530 (0) 0/0/0"
		row, err = queryISISLSP(t.Context(), recorder)
		if err != nil || row.holdtime != 0 || row.pduLen != 27 {
			t.Fatalf("purged IS-IS LSP row = %+v, %v", row, err)
		}
	})

	t.Run("ospf-lfa-frr", func(t *testing.T) {
		rows := []map[string]any{{"prefix": "172.30.255.3/32", "next-hops": []any{
			map[string]any{"protected": true, "backup": "172.30.0.3", "repair-labels": []any{"not-a-label"}},
		}}}
		if !fastRerouteProtected(rows, false) {
			t.Fatal("base LFA rejected a protected backup because of unused repair metadata")
		}
	})

	t.Run("ospf-ti-lfa-frr", func(t *testing.T) {
		sr := map[string]any{
			"enabled":     true,
			"srgb":        []any{map[string]any{"lower-bound": float64(16000), "upper-bound": float64(23999)}},
			"prefix-sids": []any{map[string]any{"prefix": "172.30.0.2/32", "index": float64(200)}},
		}
		if !srProgrammed(sr) {
			t.Fatal("configured SRGB and Prefix-SID were rejected")
		}
		sr["enabled"] = false
		if srProgrammed(sr) {
			t.Fatal("disabled SR state passed")
		}
		rows := []map[string]any{{"prefix": "172.30.255.3/32", "next-hops": []any{
			map[string]any{"protected": true, "backup": "172.30.0.3", "repair-labels": []any{float64(16000)}},
		}}}
		if !fastRerouteProtected(rows, true) {
			t.Fatal("protected TI-LFA backup was rejected")
		}
		rows[0]["next-hops"] = []any{map[string]any{"protected": true, "backup": "172.30.0.3", "repair-labels": []any{float64(0x100000)}}}
		if fastRerouteProtected(rows, true) {
			t.Fatal("repair label wider than 20 bits passed")
		}
	})

	t.Run("show-rib-under-frr-load", func(t *testing.T) {
		if ribLoadRoutes != 256 || ribLoadWalkers != 8 || ribLoadWindow != 45*time.Second {
			t.Fatalf("load dimensions changed: routes=%d walkers=%d window=%s", ribLoadRoutes, ribLoadWalkers, ribLoadWindow)
		}
		recorder := &recordingLab{}
		if err := setFRRRedistribution(t.Context(), recorder, false); err != nil {
			t.Fatal(err)
		}
		if err := setFRRRedistribution(t.Context(), recorder, true); err != nil {
			t.Fatal(err)
		}
		if recorder.reads != 2 {
			t.Fatalf("redistribution mutations = %d, want 2", recorder.reads)
		}
	})
}

type recordingLab struct {
	output  string
	logs    string
	reads   int
	failure error
}

func recorderFor(current *operation) *recordingLab {
	recorder := &recordingLab{}
	switch current.kind {
	case opUnspecified:
		recorder.failure = errors.New("checker operation is unspecified")
	case opFRRSession:
		recorder.output = "BGP state = Established"
	case opBIRDSession:
		recorder.output = current.argument + " Established"
	case opGoBGPSession:
		recorder.output = "state: established"
	case opFRRRoute:
		recorder.output = fmt.Sprintf(`{"prefix":%q,"paths":[{}]}`, current.argument)
	case opFRRRouteAbsent:
		recorder.output = `{}`
	case opBIRDRoute, opGoBGPRoute:
		recorder.output = current.argument
	case opBIRDRouteAbsent, opRequireAbsent, opWaitAbsent:
		recorder.output = strings.Join(current.proof, " ")
	case opFRRCommunity:
		recorder.output = current.argument + " " + strings.Join(current.contains, " ")
	case opFRRNoAS:
		recorder.output = current.argument + ` "aspath":{"string":"65000"}`
	case opBIRDNoAS:
		recorder.output = current.argument + " BGP.as_path: 65000"
	case opWaitContains, opWaitContainsAny, opDelayRequireContains, opRequireContains:
		recorder.output = strings.Join(current.contains, " ")
	case opRequireJSONFields, opWaitJSONFields:
		document := make(map[string]any, len(current.fields)+len(current.minimum))
		for key, value := range current.fields {
			document[key] = value
		}
		for key, minimum := range current.minimum {
			document[key] = minimum
		}
		data, err := json.Marshal(document)
		if err != nil {
			panic("BUG: recorder JSON contains only supported scalar values")
		}
		recorder.output = string(data)
	case opWaitLogFields:
		var report strings.Builder
		for key, value := range current.fields {
			fmt.Fprintf(&report, "note: %s: %s\n", key, value)
		}
		for key, minimum := range current.minimum {
			fmt.Fprintf(&report, "note: %s: %d\n", key, minimum)
		}
		recorder.logs = report.String()
	case opWaitLogContains:
		recorder.logs = strings.Join(current.contains, " ")
	case opExec, opSignal, opStart:
		recorder.output = ""
	}
	return recorder
}

func contradictoryRecorderFor(current *operation) *recordingLab {
	recorder := &recordingLab{
		output: "RECORDER-CONTRADICTION",
		logs:   "RECORDER-CONTRADICTION",
	}
	switch current.kind {
	case opUnspecified:
		recorder.failure = errors.New("checker operation is unspecified")
	case opFRRSession:
		recorder.output = "BGP state = Idle"
	case opBIRDSession:
		recorder.output = current.argument + " Idle"
	case opGoBGPSession:
		recorder.output = "state: idle"
	case opFRRRoute:
		recorder.output = `{}`
	case opFRRRouteAbsent:
		recorder.output = fmt.Sprintf(`{"prefix":%q,"paths":[{}]}`, current.argument)
	case opBIRDRoute, opGoBGPRoute:
		recorder.output = "measured peer table without requested prefix"
	case opBIRDRouteAbsent:
		recorder.output = strings.Join(append(append([]string(nil), current.proof...), current.argument), " ")
	case opFRRCommunity:
		recorder.output = current.argument + " without requested community"
	case opFRRNoAS:
		recorder.output = current.argument + " " + current.absent[0]
	case opBIRDNoAS:
		recorder.output = current.argument + " BGP.as_path: " + current.absent[0]
	case opWaitContains, opWaitContainsAny, opDelayRequireContains, opRequireContains:
		recorder.output = "measured output without required values"
	case opRequireAbsent, opWaitAbsent:
		values := append(append([]string(nil), current.proof...), current.absent...)
		recorder.output = strings.Join(values, " ")
	case opRequireJSONFields, opWaitJSONFields:
		recorder.output = `{}`
	case opExec, opSignal, opStart:
		recorder.failure = fmt.Errorf("recorded mutation failed")
	case opWaitLogFields:
		recorder.logs = "note: result: FAIL\n"
	case opWaitLogContains:
		recorder.logs = "measured log without required values"
	}
	return recorder
}

func (r *recordingLab) Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	r.reads++
	if r.failure != nil {
		return interoplab.CommandResult{}, r.failure
	}
	return interoplab.CommandResult{Stdout: r.output}, nil
}

func (r *recordingLab) ExecDetached(context.Context, string, []string, []interoplab.EnvironmentVariable) error {
	r.reads++
	return r.failure
}

func (r *recordingLab) Query(context.Context, string, []string, []interoplab.EnvironmentVariable) (string, error) {
	r.reads++
	if r.failure != nil {
		return "", r.failure
	}
	if strings.TrimSpace(r.output) == "" {
		return "", fmt.Errorf("recorded query returned no output")
	}
	return r.output, nil
}

func (r *recordingLab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	r.reads++
	if r.failure != nil {
		return interoplab.LogResult{}, r.failure
	}
	return interoplab.LogResult{Text: r.logs, Available: true}, nil
}

func (r *recordingLab) PeerPID(context.Context, string) (int, error) {
	r.reads++
	if r.failure != nil {
		return 0, r.failure
	}
	return 4242, nil
}

func (r *recordingLab) Signal(context.Context, string, string) error {
	r.reads++
	return r.failure
}

func (r *recordingLab) Pause(context.Context, string) error {
	r.reads++
	return r.failure
}

func (r *recordingLab) Unpause(context.Context, string) error {
	r.reads++
	return r.failure
}

func (r *recordingLab) Start(context.Context, string) error {
	r.reads++
	return r.failure
}

func (r *recordingLab) Stop(context.Context, string, int) error {
	r.reads++
	return r.failure
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

var _ interoplab.CheckerLab = (*recordingLab)(nil)
