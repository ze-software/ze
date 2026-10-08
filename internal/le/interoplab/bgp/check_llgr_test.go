// Design: docs/architecture/testing/interop.md -- LLGR lifecycle oracle boundaries.
package bgp

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func llgrCheckerBranches(t *testing.T) {
	t.Run("route-evidence", llgrFRRRouteEvidence)
	t.Run("session-evidence", llgrFRRSessionEvidence)
	t.Run("original-deadlines", llgrOriginalDeadlineWindows)
	t.Run("family-section", llgrFRRFamilyEvidence)
	t.Run("received-origin", llgrFRRReceivedOrigin)
	t.Run("session-generation", llgrFRRSessionGeneration)
}

// llgrFRRRouteEvidence checks FRR's actual detailed-path schema, including
// wrong-source, invalid-route and empty-query cases that could fake retention.
func llgrFRRRouteEvidence(t *testing.T) {
	const route = `{"prefix":"198.51.94.0/24","paths":[{"valid":true,"peer":{"peerId":"10.254.7.2"},"aspath":{"string":"65001 65004"},"community":{"string":"65004:94 llgr-stale","list":["65004:94","llgrStale"]}}]}`
	for _, test := range []struct {
		name, output          string
		present, stale, valid bool
	}{
		{"retained", route, true, true, true},
		{"initial", strings.Replace(route, `,"llgrStale"`, "", 1), true, false, true},
		{"removed", `{}`, false, false, true},
		{"null-is-not-absence", `null`, false, false, false},
		{"error-is-not-absence", `{"error":"unknown command"}`, false, false, false},
		{"no-path-is-not-absence", `{"prefix":"198.51.94.0/24"}`, false, false, false},
		{"wrong-prefix", strings.Replace(route, "198.51.94.0", "198.51.95.0", 1), false, false, false},
		{"wrong-peer", strings.Replace(route, "10.254.7.2", "172.30.0.2", 1), false, false, false},
		{"wrong-source-as", strings.Replace(route, "65001 65004", "65001 65005", 1), false, false, false},
		{"missing-marker", strings.Replace(route, `"65004:94",`, "", 1), false, false, false},
		{"invalid-route", strings.Replace(route, `"valid":true`, `"valid":false`, 1), false, false, false},
		{"stale-in-unrelated-field", strings.Replace(strings.Replace(route, `,"llgrStale"`, "", 1), `"prefix":`, `"note":"llgrStale","prefix":`, 1), true, false, true},
		{"text-token-is-not-array-token", strings.Replace(route, `"llgrStale"`, `"llgr-stale"`, 1), true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			present, stale, err := llgrRouteState(test.output, llgrIPv4Prefix, "10.254.7.2", "65001 65004", "65004:94")
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if err == nil && (present != test.present || stale != test.stale) {
				t.Fatalf("present/stale=%v/%v want %v/%v", present, stale, test.present, test.stale)
			}
		})
	}
}

// llgrFRRSessionEvidence refuses absent counters and one-way capability claims.
// Family evidence is checked separately in the LLGR text section.
func llgrFRRSessionEvidence(t *testing.T) {
	const session = llgrFRRSessionFixture
	for _, test := range []struct {
		name, output string
		valid        bool
	}{
		{"bilateral", session, true},
		{"missing-counter", strings.Replace(session, `"connectionsDropped":0,`, "", 1), false},
		{"recomputed-epoch", strings.Replace(session, `"bgpTimerUpEstablishedEpoch":123`, `"bgpTimerUpEstablishedEpoch":124`, 1), true},
		{"no-derived-epoch", strings.Replace(session, `"bgpTimerUpEstablishedEpoch":123,`, "", 1), true},
		{"not-bilateral", strings.Replace(session, "advertisedAndReceived", "received", 1), false},
		{"reset", strings.Replace(session, "Established\"", "Active\"", 1), false},
		{"wrong-network", strings.Replace(session, "10.254.7.2", "172.30.0.2", 1), false},
		{"empty", `{}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity, err := llgrParseFRRSession(test.output, "10.254.7.2")
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if test.valid && identity != (llgrFRRIdentity{Established: 1, Dropped: 0}) {
				t.Fatalf("wrong identity: %+v", identity)
			}
		})
	}
}

// llgrOriginalDeadlineWindows checks both polarities of the immediate,
// conventional, original-expiry and zero-LLST observations without wall sleeps.
func llgrOriginalDeadlineWindows(t *testing.T) {
	loss := time.Unix(100, 0)
	down := loss.Add(time.Second)
	for _, test := range []struct {
		name                  string
		at, enter, expire     time.Duration
		present, stale, valid bool
	}{
		{"immediate-stale", 5, 0, 40, true, true, true},
		{"missing-immediate-stale", 5, 0, 40, true, false, false},
		{"retained-before-expiry", 36, 0, 40, true, true, true},
		{"early-loss", 36, 0, 40, false, false, false},
		{"original-expiry", 45, 0, 40, false, false, true},
		{"rearmed-at-sibling-entry", 45, 0, 40, true, true, false},
		{"conventional-first", 10, 20, 60, true, false, true},
		{"conventional-marked-early", 10, 20, 60, true, true, false},
		{"control-enters", 25, 20, 60, true, true, true},
		{"control-not-marked", 25, 20, 60, true, false, false},
		{"control-survives", 50, 20, 60, true, true, true},
		{"peer-wide-purge", 50, 20, 60, false, false, false},
		{"control-expires", 65, 20, 60, false, false, true},
		{"control-overretained", 65, 20, 60, true, true, false},
		{"zero-removed", 1, 0, 0, false, false, true},
		{"zero-retained", 1, 0, 0, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			at := loss.Add(test.at * time.Second)
			err := llgrCheckWindow(at, at, loss, down, test.enter*time.Second, test.expire*time.Second, test.present, test.stale)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
		})
	}
}

// TestLLGRSourceCapabilityBytes proves the input speaker does not mirror Ze's
// GR families or LLST: both asymmetry and the zero-LLST control are literal wire.
func TestLLGRSourceCapabilityBytes(t *testing.T) {
	for _, test := range []struct {
		zero         bool
		asn          uint
		capabilities string
	}{
		{false, 65004, "01040001000141040000fdec0104000200014006001400020100470e0001010000002800020100000028"},
		{true, 65005, "01040001000141040000fded40020014470700010100000000"},
	} {
		frame, err := llgrSourceOpen(speakerOptions{asn: test.asn, routerID: "10.254.7.10"}, test.zero)
		if err != nil {
			t.Fatal(err)
		}
		if int(binary.BigEndian.Uint16(frame[16:18])) != len(frame) {
			t.Fatal("OPEN length does not match its frame")
		}
		if got := hex.EncodeToString(frame[31:]); got != test.capabilities {
			t.Fatalf("capabilities=%s want %s", got, test.capabilities)
		}
		if frame[28] != byte(len(frame)-29) || frame[29] != 2 || frame[30] != byte(len(frame)-31) {
			t.Fatal("OPEN optional parameter framing is invalid")
		}
	}
}

// TestLLGRSourceSelectedNetwork checks the network-bearing octets of both
// families without using Ze's UPDATE encoder to produce the expected values.
func TestLLGRSourceSelectedNetwork(t *testing.T) {
	frames := llgrSourceUpdates(netip.MustParseAddr("10.254.7.10"), netip.MustParseAddr("fd00:fe:7::a"), 65004, false)
	if len(frames) != 4 {
		t.Fatalf("got %d frames, want IPv4 UPDATE/EOR and IPv6 UPDATE/EOR", len(frames))
	}
	for index, frame := range frames {
		if int(binary.BigEndian.Uint16(frame[16:18])) != len(frame) {
			t.Fatalf("frame %d has wrong length", index)
		}
	}
	v4 := hex.EncodeToString(frames[0][19:])
	if !strings.HasSuffix(v4, "4003040afe070a18c6335e") {
		t.Fatalf("IPv4 NEXT_HOP or NLRI is wrong: %s", v4)
	}
	v6 := hex.EncodeToString(frames[2][19:])
	if !strings.Contains(v6, "00020110fd0000fe00070000000000000000000a003020010db80094") {
		t.Fatalf("IPv6 MP_REACH does not use selected network: %s", v6)
	}
	for _, frame := range frames {
		if strings.Contains(hex.EncodeToString(frame), "ffff0006") {
			t.Fatal("source must never originate LLGR_STALE")
		}
	}
}

// TestLLGRSourceRenderedNetwork drives the actual scenario through the native
// renderer, so the IPv6 capability cannot silently run on an IPv4-only lab.
func TestLLGRSourceRenderedNetwork(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "test", "interop", "scenarios", scenarioGracefulRestartFRR)
	dualStack, err := needsIPv6(source)
	if err != nil {
		t.Fatal(err)
	}
	if !dualStack {
		t.Fatal("LLGR scenario did not request a dual-stack Docker network")
	}
	network := interoplab.Network{
		IPv4: netip.MustParsePrefix("10.254.7.0/24"),
		IPv6: netip.MustParsePrefix("fd00:fe:7::/64"),
	}
	target := t.TempDir()
	if err := renderScenario(source, target, network); err != nil {
		t.Fatal(err)
	}
	args, err := readArguments(target, "speaker-args")
	if err != nil {
		t.Fatal(err)
	}
	options, err := parseSpeakerOptions(args)
	if err != nil {
		t.Fatal(err)
	}
	if options.connect != "10.254.7.2:179" {
		t.Fatalf("rendered source connects to %s", options.connect)
	}
	if options.routerID != "10.254.7.10" {
		t.Fatalf("rendered source ID is %s", options.routerID)
	}
	if options.sourceNextHopV6 != "fd00:fe:7::a" {
		t.Fatalf("rendered IPv6 next hop is %s", options.sourceNextHopV6)
	}
	if speakerRunners[options.test] == nil {
		t.Fatalf("rendered source oracle %s is not registered", options.test)
	}
}

// FRR 10.3.1 emits an empty ByPeer map without ENHE negotiation; it is not
// the LLGR tuple oracle. Its text section is the family evidence below.
const llgrFRRSessionFixture = `{"10.254.7.2":{"bgpState":"Established","connectionsEstablished":1,"connectionsDropped":0,"bgpTimerUpEstablishedEpoch":123,"neighborCapabilities":{"longLivedGracefulRestart":"advertisedAndReceived","longLivedGracefulRestartByPeer":{}}}}`

const llgrFRRFamilyFixture = "    Long-lived Graceful Restart: advertised and received\n      Address families by peer:\n           IPv4 Unicast\n           IPv6 Unicast\n    Route refresh: advertised and received\n"

func llgrFRRFamilyEvidence(t *testing.T) {
	for _, test := range []struct {
		name, output string
		valid        bool
	}{
		{"both-families", llgrFRRFamilyFixture, true},
		{"no-section", "", false},
		{"missing-v4", strings.Replace(llgrFRRFamilyFixture, "           IPv4 Unicast\n", "", 1), false},
		{"missing-v6", strings.Replace(llgrFRRFamilyFixture, "           IPv6 Unicast\n", "", 1), false},
		{"one-way", strings.Replace(llgrFRRFamilyFixture, "advertised and received", "received", 1), false},
		{"only-ENHE", strings.Replace(llgrFRRFamilyFixture, "Long-lived Graceful Restart", "Extended nexthop", 1), false},
		{"only-GR", strings.Replace(llgrFRRFamilyFixture, "Long-lived Graceful Restart", "Graceful Restart", 1), false},
		{"family-in-next-section", strings.Replace(llgrFRRFamilyFixture, "           IPv6 Unicast\n", "", 1) + "      Address families by peer:\n           IPv6 Unicast\n", false},
		{"family-before-LLGR", "    Multiprotocol: advertised and received\n      Address families by peer:\n           IPv6 Unicast\n" + strings.Replace(llgrFRRFamilyFixture, "           IPv6 Unicast\n", "", 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := llgrFRRFamilies(test.output); (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
		})
	}
}

func llgrFRRSessionGeneration(t *testing.T) {
	for _, test := range []struct {
		name, output string
		valid        bool
	}{
		{"unchanged", llgrFRRSessionFixture, true},
		{"epoch-rounding", strings.Replace(llgrFRRSessionFixture, ":123", ":124", 1), true},
		{"new-establishment", strings.Replace(llgrFRRSessionFixture, `"connectionsEstablished":1`, `"connectionsEstablished":2`, 1), false},
		{"new-drop", strings.Replace(llgrFRRSessionFixture, `"connectionsDropped":0`, `"connectionsDropped":1`, 1), false},
		{"not-established", strings.Replace(llgrFRRSessionFixture, `"Established"`, `"Active"`, 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := &interoplab.CheckContext{
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("10.254.7.0/24")},
				Lab:     &recordingLab{output: test.output},
			}
			err := llgrRequireSession(t.Context(), check, llgrFRRIdentity{Established: 1})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
		})
	}
}

func llgrFRRReceivedOrigin(t *testing.T) {
	const received = `[{"peer":"10.254.7.3","direction":"received","family":"ipv4/unicast","prefix":"10.20.0.0/24","next-hop":"10.254.7.3"}]`
	for _, test := range []struct {
		name, output   string
		present, valid bool
	}{
		{"FRR-origin", received, true, true},
		{"empty", `[]`, false, true},
		{"missing", `{}`, false, false},
		{"null", `null`, false, false},
		{"RPC-envelope", `{"routes":[]}`, false, false},
		{"native-first-source", strings.Replace(received, `"peer":"10.254.7.3"`, `"peer":"10.254.7.10"`, 1), false, true},
		{"native-second-source", strings.Replace(received, `"peer":"10.254.7.3"`, `"peer":"10.254.7.11"`, 1), false, true},
		{"outbound", strings.Replace(received, `"received"`, `"sent"`, 1), false, true},
		{"wrong-family", strings.Replace(received, `"ipv4/unicast"`, `"ipv4/multicast"`, 1), false, true},
		{"wrong-prefix", strings.Replace(received, "10.20.0.0/24", llgrIPv4Prefix, 1), false, true},
		{"wrong-next-hop", strings.Replace(received, `"next-hop":"10.254.7.3"`, `"next-hop":"10.254.7.10"`, 1), false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			present, err := llgrReceivedRoutesPresent(test.output, "10.254.7.3", []llgrReceivedExpectation{{"10.20.0.0/24", "ipv4/unicast", "10.254.7.3"}})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if present != test.present {
				t.Fatalf("present=%v want=%v", present, test.present)
			}
		})
	}
}

// TestLLGRReceivedFenceRetainsEvidence keeps the actual completed result when
// the last exec consumes the remaining deadline. An observation failure must
// remain a failure, but its earlier CLI or parser evidence must not disappear.
func TestLLGRReceivedFenceRetainsEvidence(t *testing.T) {
	const received = `[{"peer":"10.254.7.3","direction":"received","family":"ipv4/unicast","prefix":"10.20.0.0/24","next-hop":"10.254.7.3"}]`
	const empty = `[]`
	const missing = `null`
	for _, test := range []struct {
		name    string
		replies []llgrReceivedProbeReply
		want    []string
		pass    bool
	}{
		{
			name:    "received",
			replies: []llgrReceivedProbeReply{{result: interoplab.CommandResult{Stdout: received}}},
			pass:    true,
		},
		{
			name:    "parser-error-before-deadline",
			replies: []llgrReceivedProbeReply{{result: interoplab.CommandResult{Stdout: missing}}},
			want:    []string{"never measured peer state", "context deadline exceeded", "available=true", fmt.Sprintf("stdout=%q", missing), "ze received RIB query omitted routes"},
		},
		{
			name: "failed-cli-payload-before-deadline",
			replies: []llgrReceivedProbeReply{{
				result: interoplab.CommandResult{Stdout: "refused command payload", Stderr: "permission denied", ExitCode: 1},
				err:    errors.New("CLI refused"),
			}},
			want: []string{"never measured peer state", "context deadline exceeded", "available=true", `exit=1 stdout="refused command payload" stderr="permission denied" error=CLI refused`},
		},
		{
			name: "measured-rib-before-parser-error-and-deadline",
			replies: []llgrReceivedProbeReply{
				{result: interoplab.CommandResult{Stdout: empty}},
				{result: interoplab.CommandResult{Stdout: missing}},
			},
			want: []string{"timed out before the peer became ready", fmt.Sprintf("stdout=%q", missing), fmt.Sprintf("last measured RIB=%q", empty)},
		},
		{
			name: "no-completed-observation",
			want: []string{"never measured peer state", "context deadline exceeded", "available=false", `last measured RIB=""`},
		},
		{
			name:    "empty-stdout-is-not-a-measurement",
			replies: []llgrReceivedProbeReply{{}},
			want:    []string{"never measured peer state", "peer ze query returned no output", `last measured RIB=""`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				lab := &llgrReceivedProbeLab{replies: test.replies}
				check := &interoplab.CheckContext{
					Network: interoplab.Network{IPv4: netip.MustParsePrefix("10.254.7.0/24")},
					Lab:     lab,
				}
				started := time.Now()
				err := llgrReceivedFence(t.Context(), check, 3)
				if test.pass {
					if err != nil || lab.attempts != 1 {
						t.Fatalf("received route: attempts=%d error=%v", lab.attempts, err)
					}
					return
				}
				if err == nil {
					t.Fatal("failed observation was accepted as received-route evidence")
				}
				for _, want := range test.want {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("diagnostic omits %q: %v", want, err)
					}
				}
				if elapsed := time.Since(started); elapsed != 30*time.Second {
					t.Errorf("receipt window changed: %s", elapsed)
				}
				if lab.attempts != len(test.replies)+1 {
					t.Errorf("diagnostics added probes: attempts=%d replies=%d", lab.attempts, len(test.replies))
				}
			})
		})
	}
}

type llgrReceivedProbeReply struct {
	result interoplab.CommandResult
	err    error
}

type llgrReceivedProbeLab struct {
	noEvidenceLab
	replies  []llgrReceivedProbeReply
	attempts int
}

func (lab *llgrReceivedProbeLab) Exec(ctx context.Context, _ string, _ []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	index := lab.attempts
	lab.attempts++
	if index < len(lab.replies) {
		return lab.replies[index].result, lab.replies[index].err
	}
	<-ctx.Done()
	return interoplab.CommandResult{ExitCode: 124}, ctx.Err()
}

// TestLLGRCheckerRejectsTerminalObservationStall runs the actual observation
// loop in virtual time. A successful but stalled query must not hide an early
// withdrawal once both earlier positive observations have already succeeded.
func TestLLGRCheckerRejectsTerminalObservationStall(t *testing.T) {
	for _, mode := range []string{"healthy", "late-session", "late-baseline", "terminal-route", "terminal-session"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				lab := &llgrClockLab{started: time.Now(), mode: mode}
				check := &interoplab.CheckContext{
					Network: interoplab.Network{IPv4: netip.MustParsePrefix("10.254.7.0/24")},
					Lab:     lab,
				}
				err := llgrObserveExpiry(t.Context(), check, llgrFRRIdentity{Established: 1}, lab.started, lab.started)
				if mode == "healthy" {
					if err != nil {
						t.Fatalf("healthy lifecycle failed: %v", err)
					}
					return
				}
				if !lab.stalled {
					t.Fatalf("test did not reach the %s blocking query: %v", mode, err)
				}
				if err == nil {
					t.Fatal("stalled final cycle falsely certified the original expiry")
				}
				if !strings.Contains(err.Error(), "observation interval") {
					t.Fatalf("wrong failure after blocking query: %v", err)
				}
			})
		})
	}
}

// llgrClockLab reports source-faithful FRR documents on the virtual clock.
// All methods other than Query inherit noEvidenceLab's fail-closed behavior.
type llgrClockLab struct {
	noEvidenceLab
	started          time.Time
	mode             string
	stalled          bool
	terminalRoutes   int
	terminalSessions int
}

func (lab *llgrClockLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	if peer != peerFRR {
		return "", fmt.Errorf("unexpected observation peer %s", peer)
	}
	if len(command) != 3 {
		return "", fmt.Errorf("unexpected observation command %v", command)
	}
	elapsed := time.Since(lab.started)
	session := strings.HasPrefix(command[2], "show bgp neighbor ")
	baseline := strings.Contains(command[2], injectPrefixFirst)
	ipv6 := strings.Contains(command[2], llgrIPv6Prefix)
	if elapsed > llgrRestartTime+llgrStaleTime+llgrObservationMargin {
		if session {
			lab.terminalSessions++
		}
		if ipv6 {
			lab.terminalRoutes++
		}
	}
	stall := false
	switch lab.mode {
	case "healthy":
	case "late-session":
		stall = session && elapsed >= 50*time.Second
	case "late-baseline":
		stall = baseline && elapsed >= 50*time.Second
	case "terminal-route":
		stall = ipv6 && lab.terminalRoutes == 2
	case "terminal-session":
		stall = session && lab.terminalSessions == 2
	default:
		return "", fmt.Errorf("unknown observation mode %s", lab.mode)
	}
	if stall && !lab.stalled {
		lab.stalled = true
		// Advance virtual time inside the successful query, not between
		// iterations: the old loop-entry-only guard missed this interval.
		<-time.After(20 * time.Second)
	}
	if session {
		return llgrFRRSessionFixture, nil
	}
	if baseline {
		return `{"prefix":"10.10.0.0/24","paths":[{}]}`, nil
	}
	if strings.Contains(command[2], llgrZeroPrefix) {
		return `{}`, nil
	}
	elapsed = time.Since(lab.started)
	prefix := llgrIPv4Prefix
	expiry := llgrStaleTime
	stale := true
	if ipv6 {
		prefix = llgrIPv6Prefix
		expiry = llgrRestartTime + llgrStaleTime
		stale = elapsed >= llgrRestartTime
		if lab.mode == "late-session" || lab.mode == "late-baseline" {
			expiry = 51 * time.Second
		}
	} else if !strings.Contains(command[2], llgrIPv4Prefix) {
		return "", fmt.Errorf("unexpected observation command %v", command)
	}
	if elapsed >= expiry {
		return `{}`, nil
	}
	communities := `"65004:94"`
	if stale {
		communities += `,"llgrStale"`
	}
	return fmt.Sprintf(`{"prefix":%q,"paths":[{"valid":true,"peer":{"peerId":"10.254.7.2"},"aspath":{"string":"65001 65004"},"community":{"list":[%s]}}]}`, prefix, communities), nil
}
