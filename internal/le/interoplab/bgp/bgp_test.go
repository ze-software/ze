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
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

// TestCheckerPopulationMatchesProducer validates that every scenario directory
// has a typed checker, so adding one can never silently shrink the gate.
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
		producer = append(producer, entry.Name())
	}
	sort.Strings(producer)
	checkers := checkers()
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
	}
	sort.Strings(native)
	if strings.Join(native, "\n") != strings.Join(producer, "\n") {
		t.Fatalf("native scenario population differs from producer\nnative: %v\nproducer: %v", native, producer)
	}
}

// TestEveryCheckerFailsClosedWithoutPeerEvidence runs the complete checker
// registry against a lab where every observation fails.
func TestEveryCheckerFailsClosedWithoutPeerEvidence(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	for name, checker := range checkers() {
		if err := checker(ctx, &interoplab.CheckContext{
			Source: interoplab.ScenarioSource{Name: name},
			Lab:    noEvidenceLab{},
		}); err == nil {
			t.Errorf("checker %s passed with no peer evidence", name)
		}
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
		opBIRDRoute, opGoBGPRoute, opFRRCommunity, opFRRNoAS,
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

// TestRFCInteropCheckerBindings pins each RFC carrier to the scenario whose
// foreign-peer observations the native integration action executes.
func TestRFCInteropCheckerBindings(t *testing.T) {
	for _, name := range []string{
		"bgp-addpath-readvertise-collision-frr",
		"bgp-local-pref-strip-gobgp",
		"bgp-med-across-as-gobgp",
		"bgp-med-remove-configured-gobgp",
		"bgp-relay-withdraw-reflector-frr",
		"bgp-relay-withdraw-shape-frr",
		"bgp-rfc2545-linklocal-nexthop-frr",
		"bgp-rfc7606-relay-shape-frr",
		"bgp-rfc7606-typed-nlri-discard",
		"bgp-rfc7999-blackhole-frr",
		"bgp-role-otc-withdraw-frr",
		"bgp-route-server-frr",
		"bgp-self-nexthop-withheld-frr",
		"bgp-wellknown-noexport-frr",
		"isis-p2p-frr",
		"no-family-peer-eor-frr",
		"ospf-stub-nssa-frr",
	} {
		if _, ok := specialCheckers[name]; !ok {
			t.Errorf("native integration registry does not execute RFC checker %s", name)
		}
	}
	for name, checker := range map[string]interoplab.Checker{
		"bgp-addpath-readvertise-collision-frr": checkAddPathReadvertiseCollision,
		"bgp-local-pref-strip-gobgp":            checkLocalPrefStrip,
		"bgp-med-across-as-gobgp":               checkMEDAcrossAS,
		"bgp-med-remove-configured-gobgp":       checkMEDRemovalConfiguration,
		"bgp-relay-withdraw-shape-frr":          checkRelayWithdrawalShape,
		"bgp-rfc2545-linklocal-nexthop-frr":     checkRFC2545NextHops,
		"bgp-rfc7606-relay-shape-frr":           checkRFC7606MixedUpdate,
		"bgp-rfc7606-typed-nlri-discard":        checkRFC7606TypedNLRIDiscard,
		"bgp-rfc7999-blackhole-frr":             checkRFC7999Blackhole,
		"bgp-role-otc-withdraw-frr":             checkOTCWithdrawal,
		"bgp-route-server-frr":                  checkRouteServerASPath,
		"bgp-self-nexthop-withheld-frr":         checkSelfNextHopWithheld,
		"bgp-wellknown-noexport-frr":            checkNoExportBoundary,
		"isis-p2p-frr":                          checkISISDynamicHostname,
		"no-family-peer-eor-frr":                checkNoFamilyEndOfRIB,
		"ospf-stub-nssa-frr":                    checkNSSADefault,
	} {
		t.Run(name, func(t *testing.T) {
			sentinel := errors.New("stop at first foreign-peer query")
			lab := &recordingLab{failure: sentinel}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			err := checker(ctx, &interoplab.CheckContext{
				Network: interoplab.Network{
					IPv4: netip.MustParsePrefix("172.30.44.0/24"),
					IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
				},
				Lab: lab,
			})
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("checker error = %v, want scenario name %q", err, name)
			}
		})
	}
}

// TestRFCInteropStructuredPeerEvidence proves checks that cannot be
// represented as substring operations decode the foreign daemon's exact state.
func TestRFCInteropStructuredPeerEvidence(t *testing.T) {
	t.Run("distinct add-path identifiers", func(t *testing.T) {
		state, err := parseAddPathState(`{"routes":{"10.99.0.0/24":[{"aspath":{"string":"65003"}},{"aspath":{"string":"65004"},"addpathRxId":1}]}}`)
		if err != nil {
			t.Fatal(err)
		}
		if state["65003"] != 0 || state["65004"] != 1 {
			t.Fatalf("Path Identifiers = %v", state)
		}
		if _, err := parseAddPathState(`{"routes":{"10.99.0.0/24":[{"aspath":{"string":"65003"}},{"aspath":{"string":"65004"}}]}}`); err == nil {
			t.Fatal("colliding received Path Identifiers passed")
		}
	})

	t.Run("OTC value", func(t *testing.T) {
		for _, output := range []string{"OTC: 65001", `{"otc":65001}`} {
			value, err := parseOTCValue(output)
			if err != nil {
				t.Fatal(err)
			}
			if value != 65001 {
				t.Fatalf("OTC = %d", value)
			}
		}
		if value, err := parseOTCValue("OTC: 65002"); err != nil || value == 65001 {
			t.Fatalf("wrong OTC verdict: value=%d err=%v", value, err)
		}
	})

	t.Run("RFC2545 next-hop lengths", func(t *testing.T) {
		onLink, err := parseFRRNextHops(`{"paths":[{"nexthops":[{"scope":"global","ip":"fd00:1e:2c::2"},{"scope":"link-local","ip":"fe80::be:ef:2"}]}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := requireNextHopShape(onLink, netip.MustParseAddr("fd00:1e:2c::2"), netip.MustParseAddr("fe80::be:ef:2")); err != nil {
			t.Fatal(err)
		}
		offLink, err := parseFRRNextHops(`{"paths":[{"nexthops":[{"scope":"global","ip":"2001:db8:ffff::1"}]}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := requireNextHopShape(offLink, netip.MustParseAddr("2001:db8:ffff::1"), netip.Addr{}); err != nil {
			t.Fatal(err)
		}
		if err := requireNextHopShape(onLink, netip.MustParseAddr("fd00:1e:2c::2"), netip.Addr{}); err == nil {
			t.Fatal("off-link route accepted a link-local next hop")
		}
	})
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

// TestCheckerFailureKeepsPrimaryCauseWhenLogsFail preserves the former
// scenario-55 exception polarity without executing its deleted probe.
func TestCheckerFailureKeepsPrimaryCauseWhenLogsFail(t *testing.T) {
	cause := errors.New("BIRD route is absent")
	err := checkerFailure(t.Context(), &recordingLab{failure: errors.New("docker logs unavailable")}, "bgp-wire-edit-api-origin-bird", 2, cause)
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "docker logs unavailable") {
		t.Fatalf("checker failure replaced the primary assertion: %v", err)
	}
}

func TestCheckerFailureIncludesAvailablePeerLogs(t *testing.T) {
	cause := errors.New("route is absent")
	err := checkerFailure(t.Context(), &recordingLab{logs: "peer startup clue"}, "scenario", 1, cause)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "peer startup clue") {
		t.Fatalf("checker failure omitted the primary cause or peer logs: %v", err)
	}
}

// TestScenarioPreparerBuildsOrderedPeers validates config discovery, compiled
// helper argv, rebased speaker addressing, and the legacy participant order.
func TestScenarioPreparerBuildsOrderedPeers(t *testing.T) {
	producer := t.TempDir()
	scenario := t.TempDir()
	writeFixture(t, filepath.Join(scenario, "ze.conf"), "bgp {}\n")
	writeFixture(t, filepath.Join(scenario, "frr.conf"), "router bgp 65002\n")
	writeFixture(t, filepath.Join(scenario, "bird.conf"), "protocol device {}\n")
	writeFixture(t, filepath.Join(scenario, "gobgp.toml"), "[global.config]\n")
	writeFixture(t, filepath.Join(scenario, "speaker-args"), "--asn 65010\n")
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
	if joined := strings.Join(peers[1].Command, " "); !strings.Contains(joined, "interop-bgp speaker --connect 172.31.22.2:179") {
		t.Fatalf("compiled speaker command did not use selected network: %s", joined)
	}
	if got := peers[1].Arguments; !slices.Equal(got, []string{"--entrypoint", "ze-test"}) {
		t.Fatalf("speaker entrypoint = %v, want compiled ze-test", got)
	}
	if len(peers[0].Mounts) != 1 || peers[0].Mounts[0].Target != "/etc/ze/bgp.conf" || !peers[0].Mounts[0].ReadOnly {
		t.Fatalf("ze mounts = %+v, want only immutable rendered config", peers[0].Mounts)
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

	t.Run("bgp-addpath-readvertise-collision-frr", func(t *testing.T) {
		negotiated := `{"172.30.0.2":{"neighborCapabilities":{"addPath":{"ipv4Unicast":{"rxAdvertised": true, "rxAdvertisedAndReceived": true}}}}}`
		if !addPathReceiveNegotiated(negotiated) {
			t.Fatal("a negotiated ADD-PATH receive direction was rejected")
		}
		if addPathReceiveNegotiated(`{"addPath":{"ipv4Unicast":{"txAdvertisedAndReceived": true}}}`) {
			t.Fatal("the ADD-PATH send direction passed as the receive direction")
		}
		if addPathReceiveNegotiated(`{"addPath":{"ipv4Unicast":{"rxAdvertised": true, "rxAdvertisedAndReceived": false}}}`) {
			t.Fatal("an advertised but unagreed receive direction passed as negotiated")
		}
		if addPathReceiveNegotiated("") {
			t.Fatal("an empty neighbor answer passed as a negotiated capability")
		}
		live := map[string]uint64{"65003": 0, "65004": 1}
		if !samePathIdentifiers(live, map[string]uint64{"65003": 0, "65004": 1}) {
			t.Fatal("a replay repeating both Path Identifiers was reported as renumbered")
		}
		if samePathIdentifiers(live, map[string]uint64{"65003": 1, "65004": 0}) {
			t.Fatal("a replay that swapped the two Path Identifiers passed")
		}
		if samePathIdentifiers(live, map[string]uint64{"65004": 1}) {
			t.Fatal("a replay that lost one path passed")
		}
	})

	t.Run("bgp-rfc2545-linklocal-nexthop-frr", func(t *testing.T) {
		const (
			onLink    = "2001:db8:5601::/48"
			offLink   = "2001:db8:5602::/48"
			linkLocal = "fe80::be:ef:2"
		)
		installed := "B>* " + onLink + " [20/0] via " + linkLocal + ", eth0, weight 1, 00:00:07\n"
		if err := requireRouteInstalledVia(installed, onLink, linkLocal); err != nil {
			t.Fatalf("a route installed via the link-local next hop failed: %v", err)
		}
		split := "B>* " + onLink + " [20/0] via 2001:db8:ffff::1, eth0\n" +
			"B>* " + offLink + " [20/0] via " + linkLocal + ", eth0\n"
		if err := requireRouteInstalledVia(split, onLink, linkLocal); err == nil {
			t.Fatal("a next hop taken from another route's line passed")
		}
		if err := requireRouteInstalledVia("B>* "+offLink+" [20/0] via "+linkLocal+", eth0\n", onLink, linkLocal); err == nil {
			t.Fatal("an uninstalled route passed as installed")
		}
		if err := requireRouteInstalledVia("", onLink, linkLocal); err == nil {
			t.Fatal("an empty route listing passed as installed")
		}
	})

	t.Run("no-family-peer-eor-frr", func(t *testing.T) {
		const peer = "172.30.0.2"
		decoded := "2026/08/31 03:18:06 BGP: [T1234-56789] " + peer + " rcvd End-of-RIB for IPv4 Unicast from " + peer + "\n"
		if !endOfRIBDecoded(decoded, peer) {
			t.Fatal("FRR's own End-of-RIB decode was rejected")
		}
		if !endOfRIBDecoded("BGP: rcvd End-of-RIB for ipv4 unicast from "+peer+"\n", peer) {
			t.Fatal("a lower-case address family spelling was rejected")
		}
		if endOfRIBDecoded("BGP: "+peer+" sending End-of-RIB for IPv4 Unicast to "+peer+"\n", peer) {
			t.Fatal("the send direction passed as FRR's receive decode")
		}
		split := "BGP: rcvd End-of-RIB for IPv4 Unicast from 172.30.0.9\n" +
			"BGP: " + peer + " went from OpenConfirm to Established\n"
		if endOfRIBDecoded(split, peer) {
			t.Fatal("another peer's marker passed by matching ze on a second line")
		}
		if endOfRIBDecoded("", peer) {
			t.Fatal("an empty log passed as a decoded marker")
		}
		const capabilities = `{%q:{"neighborCapabilities":{"multiprotocolExtensions":{"ipv4Unicast":{%s}}}}}`
		advertised := fmt.Sprintf(capabilities, peer, `"advertised":true`)
		if err := requireMultiprotocolAdvertisedOnly(advertised, peer); err != nil {
			t.Fatalf("FRR advertising the family alone was rejected: %v", err)
		}
		both := fmt.Sprintf(capabilities, peer, `"advertisedAndReceived":true`)
		if requireMultiprotocolAdvertisedOnly(both, peer) == nil {
			t.Fatal("a Multiprotocol capability received from ze passed as absent")
		}
		received := fmt.Sprintf(capabilities, peer, `"advertised":true,"received":true`)
		if requireMultiprotocolAdvertisedOnly(received, peer) == nil {
			t.Fatal("a capability received from ze passed because it was not spelled advertisedAndReceived")
		}
		foreign := fmt.Sprintf(capabilities, "172.30.0.3", `"advertised":true`)
		if requireMultiprotocolAdvertisedOnly(foreign, peer) == nil {
			t.Fatal("another neighbor's capability block passed as the answer about ze")
		}
		if requireMultiprotocolAdvertisedOnly("", peer) == nil {
			t.Fatal("an empty neighbor answer passed as a measured capability")
		}
	})

	t.Run("isis-p2p-frr", func(t *testing.T) {
		const hostname = "ze-p2p"
		if !isisAdjacencyUp(" frr-isis-p2p   eth0   2  Up    27   2020.2020.2020\n") {
			t.Fatal("an Up IS-IS adjacency was reported down")
		}
		if isisAdjacencyUp(" frr-isis-p2p   eth0   2  Down  -    2020.2020.2020\n") {
			t.Fatal("a Down IS-IS adjacency was reported Up")
		}
		if isisAdjacencyUp(" ze-Uplink      eth0   2  Init  27\n") {
			t.Fatal("a neighbor name carrying Up passed as the adjacency state")
		}
		if isisAdjacencyUp("") {
			t.Fatal("an empty neighbor table passed as an Up adjacency")
		}
		database := "IS-IS Level-1 link-state database:\n" +
			"LSP ID          PduLen  SeqNumber   Chksum  Holdtime  ATT/P/OL\n" +
			hostname + ".00-00        85  0x00000003  0x1234      1123  0/0/0\n"
		if !isisDatabaseNamesZe(database) {
			t.Fatal("an LSP rendered by ze's dynamic hostname was rejected")
		}
		if isisDatabaseNamesZe("0000.0000.0002.00-00        85  0x00000003\n") {
			t.Fatal("an LSP rendered by system ID passed as a decoded TLV 137")
		}
		if isisDatabaseNamesZe(hostname + "-2.00-00        85  0x00000003\n") {
			t.Fatal("another router whose name starts with ze's passed as ze's LSP")
		}
		if isisDatabaseNamesZe("") {
			t.Fatal("an empty database passed as a rendered hostname")
		}
	})

	t.Run("ospf-stub-nssa-frr", func(t *testing.T) {
		const (
			defaultRoute = "0.0.0.0/0"
			borderRouter = "172.30.0.2"
		)
		installed := "O>* " + defaultRoute + " [110/20] via " + borderRouter + ", eth0, weight 1, 00:00:12\n"
		if err := requireRouteInstalledVia(installed, defaultRoute, borderRouter); err != nil {
			t.Fatalf("the NSSA default from the border router was rejected: %v", err)
		}
		elsewhere := "O>* " + defaultRoute + " [110/20] via 172.30.0.9, eth0\n" +
			"O>* 10.0.0.0/24 [110/10] via " + borderRouter + ", eth0\n"
		if requireRouteInstalledVia(elsewhere, defaultRoute, borderRouter) == nil {
			t.Fatal("a default from another router passed by matching the border router on a second line")
		}
		if requireRouteInstalledVia("", defaultRoute, borderRouter) == nil {
			t.Fatal("an empty route listing passed as an installed default")
		}
		clean := "       OSPF Router with ID (172.30.0.3)\n\n                AS External Link States\n\n"
		if err := requireNoExternalLSA(clean); err != nil {
			t.Fatalf("an NSSA holding no Type 5 LSA was reported as leaking: %v", err)
		}
		if requireNoExternalLSA(clean+"  LS age: 42\n  Link State ID: 10.9.9.0 (External Network Number)\n") == nil {
			t.Fatal("a Type 5 AS-external LSA in the NSSA passed as absent")
		}
		if requireNoExternalLSA("") == nil {
			t.Fatal("an unanswered database query passed as an absent LSA")
		}
	})

	t.Run("bgp-relay-withdraw-shape-frr", func(t *testing.T) {
		const (
			peer   = "172.30.0.2"
			prefix = "10.10.0.0/24"
			want   = "65001 65004"
		)
		relayed := fmt.Sprintf(`{"prefix":%q,"paths":[{"aspath":{"string":%q}}]}`, prefix, want)
		if err := requireFRRASPath(relayed, want); err != nil {
			t.Fatalf("the AS_PATH ze prepended its own AS to was rejected: %v", err)
		}
		if requireFRRASPath(`{"paths":[{"aspath":{"string":"65004"}}]}`, want) == nil {
			t.Fatal("a path missing ze's prepended AS passed")
		}
		if requireFRRASPath(`{"paths":[{"aspath":{"string":"65001 65004 65005"}}]}`, want) == nil {
			t.Fatal("a longer path carrying the wanted one passed")
		}
		if requireFRRASPath(`{"paths":[{"aspath":{"string":"65001 65004"}},{"aspath":{"string":"65002 65004"}}]}`, want) == nil {
			t.Fatal("the wanted path found beside a second path passed")
		}
		if requireFRRASPath("", want) == nil {
			t.Fatal("an unanswered route query passed as a decoded AS_PATH")
		}
		withdrawn := "2026/08/31 03:18:06 BGP: [T1234-56789] " + peer + " rcvd UPDATE about " + prefix + " IPv4 unicast -- withdrawn\n"
		announced := "BGP: " + peer + " rcvd " + prefix + " IPv4 unicast\n"
		if !frrDecodedWithdrawal(withdrawn, peer, prefix) {
			t.Fatal("FRR's own decode of the withdrawal was rejected")
		}
		if frrDecodedWithdrawal(announced, peer, prefix) {
			t.Fatal("the decode of the announcement passed as a decode of the withdrawal")
		}
		if frrDecodedWithdrawal("BGP: "+peer+" send UPDATE about "+prefix+" IPv4 unicast -- withdrawn\n", peer, prefix) {
			t.Fatal("the send direction passed as FRR's receive decode")
		}
		if frrDecodedWithdrawal("BGP: "+peer+" rcvd UPDATE about "+prefix+" IPv4 unicast\nBGP: withdrawn\n", peer, prefix) {
			t.Fatal("the withdrawn marker matched from a second line")
		}
		if frrDecodedWithdrawal(strings.ReplaceAll(withdrawn, peer, "172.30.0.22"), peer, prefix) {
			t.Fatal("a neighbor whose address carries ze's as a prefix passed as ze")
		}
		if err := requireNoAttributeError(withdrawn, peer, prefix); err != nil {
			t.Fatalf("a withdrawal FRR accepted was reported as refused: %v", err)
		}
		missing := "BGP: [EC 33554482] " + peer + " Missing well-known attribute NEXT_HOP.\n"
		if requireNoAttributeError(withdrawn+missing, peer, prefix) == nil {
			t.Fatal("FRR's missing-attribute verdict passed as acceptance")
		}
		refused := "BGP: [EC 33554455] " + peer + "(Unknown) rcvd UPDATE with errors in attr(s)!! Withdrawing route.\n"
		if requireNoAttributeError(withdrawn+refused, peer, prefix) == nil {
			t.Fatal("FRR's attribute-error verdict passed as acceptance")
		}
		if requireNoAttributeError(announced+refused, peer, prefix) == nil {
			t.Fatal("a log carrying no withdrawal decode passed as an accepted withdrawal")
		}
		if requireNoAttributeError("", peer, prefix) == nil {
			t.Fatal("an unanswered log query passed as an accepted withdrawal")
		}
	})

	t.Run("bgp-rfc7606-relay-shape-frr", func(t *testing.T) {
		const (
			replayed = "10.0.0.0/24"
			want     = "65004"
		)
		transparent := fmt.Sprintf(`{"prefix":%q,"paths":[{"aspath":{"string":%q},"nexthops":[{"ip":"172.30.0.9"}]}]}`, replayed, want)
		if err := requireFRRASPath(transparent, want); err != nil {
			t.Fatalf("a route-server relay that kept the client's own path was rejected: %v", err)
		}
		if requireFRRASPath(`{"paths":[{"aspath":{"string":"65001 65004"}}]}`, want) == nil {
			t.Fatal("a relay that prepended ze's own AS passed as route-server transparency")
		}
		if requireFRRASPath(`{"paths":[{"aspath":{"string":""}}]}`, want) == nil {
			t.Fatal("an empty AS_PATH passed as the client's own path")
		}
		if requireFRRASPath("{}", want) == nil {
			t.Fatal("a prefix FRR does not hold passed as a relayed path")
		}
		const (
			announced = "203.0.113.0/24"
			injector  = "172.30.0.9"
		)
		installed := "B>* " + announced + " [20/0] via " + injector + ", eth0, weight 1, 00:00:09\n"
		if err := requireRouteInstalledVia(installed, announced, injector); err != nil {
			t.Fatalf("the split announce FRR installed was rejected: %v", err)
		}
		if requireRouteInstalledVia("B>* "+announced+" [20/0] via 172.30.0.2, eth0\n", announced, injector) == nil {
			t.Fatal("a route installed through ze's own address passed as the relayed third-party next hop")
		}
		if requireRouteInstalledVia("B>* "+replayed+" [20/0] via "+injector+", eth0\n", announced, injector) == nil {
			t.Fatal("another route's installation passed as the split announce's")
		}
		if requireRouteInstalledVia("", announced, injector) == nil {
			t.Fatal("an unanswered route query passed as an installed route")
		}
	})

	t.Run("bgp-self-nexthop-withheld-frr", func(t *testing.T) {
		const (
			peer     = "172.30.0.2"
			control  = "10.12.0.0/24"
			withheld = "10.11.0.0/24"
			injector = "172.30.0.9"
			frrOwn   = "172.30.0.3"
		)
		decoded := "2026/08/31 03:18:06 BGP: [T1234-56789] " + peer + " rcvd " + control + " IPv4 unicast\n"
		if !frrDecodedPrefix(decoded, peer, control) {
			t.Fatal("FRR's own decode of the control route was rejected")
		}
		if frrDecodedPrefix(decoded, peer, withheld) {
			t.Fatal("the withheld route passed on the control route's decode")
		}
		if !frrDecodedPrefix("BGP: "+peer+" rcvd UPDATE about "+control+" IPv4 unicast -- withdrawn\n", peer, control) {
			t.Fatal("the withdrawn form of a decode was not read as the prefix reaching FRR")
		}
		if frrDecodedPrefix("BGP: "+peer+" rcvd 1"+control+" IPv4 unicast\n", peer, control) {
			t.Fatal("a longer prefix carrying the wanted one passed")
		}
		if frrDecodedPrefix("BGP: 172.30.0.22 rcvd "+control+" IPv4 unicast\n", peer, control) {
			t.Fatal("a neighbor whose address carries ze's as a prefix passed as ze")
		}
		if frrDecodedPrefix("BGP: "+peer+" send "+control+" IPv4 unicast\n", peer, control) {
			t.Fatal("the send direction passed as FRR's receive decode")
		}
		if frrDecodedPrefix("BGP: "+peer+" went from OpenConfirm to Established\nBGP: rcvd "+control+" IPv4 unicast\n", peer, control) {
			t.Fatal("the peer and its decode matched across two log lines")
		}
		if frrDecodedPrefix("", peer, control) {
			t.Fatal("an empty log passed as a decoded prefix")
		}
		third := netip.MustParseAddr(injector)
		if err := requireSoleNextHop([]nextHop{{IP: injector}}, third); err != nil {
			t.Fatalf("the third-party next hop ze relayed was rejected: %v", err)
		}
		if requireSoleNextHop([]nextHop{{IP: injector}, {IP: frrOwn}}, third) == nil {
			t.Fatal("a second next hop beside the wanted one passed")
		}
		if requireSoleNextHop([]nextHop{{IP: frrOwn}}, third) == nil {
			t.Fatal("FRR's own address passed as the third-party next hop")
		}
		if requireSoleNextHop(nil, third) == nil {
			t.Fatal("a route carrying no next hop passed")
		}
		if requireSoleNextHop([]nextHop{{IP: "not-an-address"}}, third) == nil {
			t.Fatal("an unparsable next hop passed")
		}
	})

	t.Run("bgp-rfc7999-blackhole-frr", func(t *testing.T) {
		blackholed := netip.MustParseAddr("10.100.0.1")
		uncovered := netip.MustParseAddr("198.51.100.1")
		unagreed := netip.MustParseAddr("10.200.0.1")
		fib := "172.30.0.0/24 dev eth0 proto kernel scope link src 172.30.0.2\n" +
			"blackhole " + blackholed.String() + "\n" +
			uncovered.String() + " via 172.30.0.3 dev eth0 proto bgp\n" +
			unagreed.String() + " via 172.30.0.4 dev eth0 proto bgp\n"
		if verdict := kernelRouteFor(fib, blackholed); verdict != kernelRouteDiscard {
			t.Fatalf("the honored blackhole read as %s", verdict)
		}
		for _, forwarded := range []netip.Addr{uncovered, unagreed} {
			if verdict := kernelRouteFor(fib, forwarded); verdict != kernelRouteForwarded {
				t.Fatalf("the forwarded %s read as %s", forwarded, verdict)
			}
		}
		if verdict := kernelRouteFor("blackhole "+blackholed.String()+"/32\n", blackholed); verdict != kernelRouteDiscard {
			t.Fatalf("iproute2's /32 spelling of a discard route read as %s", verdict)
		}
		ignored := blackholed.String() + " via 172.30.0.3 dev eth0 proto bgp\n"
		if verdict := kernelRouteFor(ignored, blackholed); verdict == kernelRouteDiscard {
			t.Fatal("a forwarded route for the covered prefix passed as a discard route")
		}
		discarded := "blackhole " + uncovered.String() + "\n"
		if verdict := kernelRouteFor(discarded, uncovered); verdict == kernelRouteForwarded {
			t.Fatal("a discard route for the uncovered prefix passed as forwarded")
		}
		covering := "blackhole 10.100.0.0/24\n"
		if verdict := kernelRouteFor(covering, blackholed); verdict != kernelRouteUnspecified {
			t.Fatalf("a covering discard route answered for the announced host route as %s", verdict)
		}
		if verdict := kernelRouteFor(covering, netip.MustParseAddr("10.100.0.0")); verdict != kernelRouteUnspecified {
			t.Fatalf("a covering discard route answered for the host route at its own network address as %s", verdict)
		}
		longer := "blackhole 10.100.0.10\n"
		if verdict := kernelRouteFor(longer, blackholed); verdict != kernelRouteUnspecified {
			t.Fatalf("a longer address carrying the wanted one answered as %s", verdict)
		}
		split := "blackhole 10.9.9.9\n" + blackholed.String() + " via 172.30.0.3 dev eth0\n"
		if verdict := kernelRouteFor(split, blackholed); verdict != kernelRouteForwarded {
			t.Fatalf("the discard verb matched from another route's line, giving %s", verdict)
		}
		onLink := blackholed.String() + " dev eth0 scope link\n"
		if verdict := kernelRouteFor(onLink, blackholed); verdict != kernelRouteOther {
			t.Fatalf("a route in neither shape read as %s", verdict)
		}
		if verdict := kernelRouteFor("", blackholed); verdict != kernelRouteUnspecified {
			t.Fatalf("an empty route table read as %s", verdict)
		}
	})

	t.Run("bgp-route-server-frr", func(t *testing.T) {
		const (
			prefix = "10.99.0.0/24"
			want   = "65002"
		)
		relayed := prefix + "           unicast [ze_peer 03:18:06.271] * (100) [AS65002i]\n" +
			"\tvia 172.30.0.2 on eth0\n" +
			"\tType: BGP univ\n" +
			"\tBGP.origin: IGP\n" +
			"\tBGP.as_path: " + want + "\n" +
			"\tBGP.next_hop: 172.30.0.2\n"
		if err := requireBIRDASPath(relayed, prefix, want); err != nil {
			t.Fatalf("a relay that kept the client's own path was rejected: %v", err)
		}
		prepended := strings.ReplaceAll(relayed, "BGP.as_path: "+want, "BGP.as_path: 65001 "+want)
		if requireBIRDASPath(prepended, prefix, want) == nil {
			t.Fatal("a relay that prepended ze's own AS passed as route-server transparency")
		}
		appended := strings.ReplaceAll(relayed, "BGP.as_path: "+want, "BGP.as_path: "+want+" 65001")
		if requireBIRDASPath(appended, prefix, want) == nil {
			t.Fatal("a relay that added ze's AS elsewhere in the path passed")
		}
		emptied := strings.ReplaceAll(relayed, "BGP.as_path: "+want, "BGP.as_path:")
		if requireBIRDASPath(emptied, prefix, want) == nil {
			t.Fatal("an empty AS_PATH passed as the client's own path")
		}
		covering := strings.ReplaceAll(relayed, prefix, "10.0.0.0/8")
		if requireBIRDASPath(covering, prefix, want) == nil {
			t.Fatal("BIRD's longest-match answer about a covering route passed as this prefix's")
		}
		longer := strings.ReplaceAll(relayed, prefix, "110.99.0.0/24")
		if requireBIRDASPath(longer, prefix, want) == nil {
			t.Fatal("a longer prefix carrying the wanted one passed as the wanted route")
		}
		if requireBIRDASPath(relayed+relayed, prefix, want) == nil {
			t.Fatal("two paths for one prefix passed the exact-one branch")
		}
		if requireBIRDASPath(relayed, "10.98.0.0/24", want) == nil {
			t.Fatal("an answer about another prefix passed as the answer about the one asked for")
		}
		if requireBIRDASPath(relayed, prefix, "65004") == nil {
			t.Fatal("the predicate read a fixed AS_PATH rather than the one it was given")
		}
		if requireBIRDASPath("", prefix, want) == nil {
			t.Fatal("an unanswered route query passed as a decoded AS_PATH")
		}
	})

	t.Run("bgp-wellknown-noexport-frr", func(t *testing.T) {
		const (
			withheld = "10.10.0.0/24"
			control  = "10.11.0.0/24"
		)
		table := func(prefixes ...string) string {
			routes := make([]string, 0, len(prefixes))
			for _, prefix := range prefixes {
				routes = append(routes, fmt.Sprintf("%q:[{\"prefix\":%q,\"valid\":true}]", prefix, prefix))
			}
			return `{"vrfId":0,"localAS":65002,"routes":{` + strings.Join(routes, ",") + `}}`
		}
		if err := requireRouteWithheld(table(control), withheld, control); err != nil {
			t.Fatalf("a table holding the control route alone was rejected: %v", err)
		}
		if requireRouteWithheld(table(control, withheld), withheld, control) == nil {
			t.Fatal("the external observer learning the NO_EXPORT route passed")
		}
		if requireRouteWithheld(table(withheld), withheld, control) == nil {
			t.Fatal("a table missing the control route passed as a withheld route")
		}
		if requireRouteWithheld(table(), withheld, control) == nil {
			t.Fatal("an empty table passed as proof that ze withheld the route")
		}
		if err := requireRouteWithheld(table(control, "110.10.0.0/24"), withheld, control); err != nil {
			t.Fatalf("a longer prefix carrying the withheld one was read as the withheld route: %v", err)
		}
		if requireRouteWithheld(table("110.11.0.0/24"), withheld, control) == nil {
			t.Fatal("a longer prefix carrying the control one passed as the control route")
		}
		const third = "10.12.0.0/24"
		if err := requireRouteWithheld(table(control), third, control); err != nil {
			t.Fatalf("a prefix the table does not hold was reported as learned: %v", err)
		}
		if requireRouteWithheld(table(control, third), third, control) == nil {
			t.Fatal("the predicate read a fixed withheld prefix rather than the one it was given")
		}
		if err := requireRouteWithheld(table(third), withheld, third); err != nil {
			t.Fatalf("the predicate read a fixed control prefix rather than the one it was given: %v", err)
		}
		if requireRouteWithheld("", withheld, control) == nil {
			t.Fatal("an unanswered table query passed as a withheld route")
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

var errNoPeerEvidence = errors.New("peer produced no evidence")

type noEvidenceLab struct{}

func (noEvidenceLab) Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	return interoplab.CommandResult{}, errNoPeerEvidence
}

func (noEvidenceLab) ExecDetached(context.Context, string, []string, []interoplab.EnvironmentVariable) error {
	return errNoPeerEvidence
}

func (noEvidenceLab) Query(context.Context, string, []string, []interoplab.EnvironmentVariable) (string, error) {
	return "", errNoPeerEvidence
}

func (noEvidenceLab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	return interoplab.LogResult{}, errNoPeerEvidence
}

func (noEvidenceLab) PeerPID(context.Context, string) (int, error) {
	return 0, errNoPeerEvidence
}

func (noEvidenceLab) Signal(context.Context, string, string) error {
	return errNoPeerEvidence
}

func (noEvidenceLab) Pause(context.Context, string) error {
	return errNoPeerEvidence
}

func (noEvidenceLab) Unpause(context.Context, string) error {
	return errNoPeerEvidence
}

func (noEvidenceLab) Start(context.Context, string) error {
	return errNoPeerEvidence
}

func (noEvidenceLab) Stop(context.Context, string, int) error {
	return errNoPeerEvidence
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

var _ interoplab.CheckerLab = (*recordingLab)(nil)
