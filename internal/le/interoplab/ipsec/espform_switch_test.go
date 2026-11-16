// Design: docs/architecture/testing/interop.md -- a form change must retain one live Child SA.
// Related: checkers.go -- checkESPFormChange; peerform/action.go -- the peer-local command contract.
// Related: espform_change_snapshot_test.go -- missing and initial-form prerequisites remain required.

package ipsec

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const espFormSwitchProgram = "/usr/bin/ipsec-peerform"

// TestESPFormSwitchKeepsOneSA accepts unchanged mismatch counters only after the
// real checker drives bare, UDP and bare probes with four advancing directions.
func TestESPFormSwitchKeepsOneSA(t *testing.T) {
	fixture := newESPFormSwitchFixture(t)
	if err := checkESPFormChange(context.Background(), fixture.lab()); err != nil {
		t.Fatalf("same-SA bare/UDP/bare delivery with unchanged mismatch counter refused: %v", err)
	}
	fixture.requireSwitches(t)
}

// TestESPFormSwitchRequiresInitialDisagreement preserves both starting-form
// refusals before any peer-local migration or traffic stimulus is permitted.
func TestESPFormSwitchRequiresInitialDisagreement(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*espFormSwitchFixture)
		want   string
	}{
		{"Ze has no UDP template", func(fixture *espFormSwitchFixture) {
			fixture.zeState = espFormSwitchState("bare")
		}, "ze inbound state carries no ESP-in-UDP template"},
		{"strongSwan already uses UDP", func(fixture *espFormSwitchFixture) {
			fixture.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED, TUNNEL-in-UDP\n")
		}, "strongSwan sends UDP-encapsulated ESP"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newESPFormSwitchFixture(t)
			test.change(fixture)
			err := checkESPFormChange(context.Background(), fixture.lab())
			if err == nil {
				t.Fatal("a missing starting form disagreement was accepted")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("starting-form refusal = %v, want %q", err, test.want)
			}
			if len(fixture.actions) != 0 {
				t.Fatal("the checker stimulated a peer before verifying the starting forms")
			}
		})
	}
}

// TestESPFormSwitchRejectsOutboundOnlyZeTemplate keeps the receive prerequisite
// directional: a populated outbound UDP template cannot stand in for inbound.
func TestESPFormSwitchRejectsOutboundOnlyZeTemplate(t *testing.T) {
	fixture := newESPFormSwitchFixture(t)
	fixture.zeState = strings.Replace(espFormSwitchState("bare"),
		"spi 0x1 reqid 1 mode tunnel",
		"spi 0x1 reqid 1 mode tunnel encap type espinudp sport 4500 dport 4500", 1)
	err := checkESPFormChange(context.Background(), fixture.lab())
	if err == nil {
		t.Fatal("form-switch checker accepted Ze's outbound-only UDP template as its inbound prerequisite")
	}
	if !strings.Contains(err.Error(), "ze inbound state carries no ESP-in-UDP template") {
		t.Fatalf("outbound-only UDP template did not produce its own refusal: %v", err)
	}
	if len(fixture.actions) != 0 {
		t.Fatal("the checker stimulated a peer before refusing Ze's bare inbound state")
	}
}

// TestESPFormSwitchRejectsAStalledDirection pairs the useful round trip with
// each missing receiver or sender, separately in every form phase.
func TestESPFormSwitchRejectsAStalledDirection(t *testing.T) {
	for phase := range 3 {
		for _, direction := range espFormSwitchDirections() {
			t.Run(strconv.Itoa(phase)+"/"+direction.summary, func(t *testing.T) {
				fixture := newESPFormSwitchFixture(t)
				fixture.stall = &espFormSwitchStall{phase: phase, direction: direction}
				err := checkESPFormChange(context.Background(), fixture.lab())
				if err == nil {
					t.Fatal("a form phase passed with a directed ESP counter unchanged")
				}
				if !strings.Contains(err.Error(), direction.summary) {
					t.Fatalf("stalled direction did not produce its own refusal: %v", err)
				}
			})
		}
	}
}

// TestESPFormSwitchRejectsInvalidProbeEvidence changes one field in the UDP
// wire observation. The checker must refuse a different SA, form or sequence.
func TestESPFormSwitchRejectsInvalidProbeEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*espFormSwitchReceipt)
	}{
		{"action", func(reply *espFormSwitchReceipt) { reply.Action = "udp" }},
		{"source", func(reply *espFormSwitchReceipt) { reply.Source = zeIP }},
		{"target", func(reply *espFormSwitchReceipt) { reply.Target = swanIP }},
		{"spi", func(reply *espFormSwitchReceipt) { reply.SPI = 9 }},
		{"reqid", func(reply *espFormSwitchReceipt) { reply.ReqID = 9 }},
		{"form", func(reply *espFormSwitchReceipt) { reply.Form = "bare" }},
		{"sequence repeated", func(reply *espFormSwitchReceipt) { reply.Sequence = 101 }},
		{"sequence reset", func(reply *espFormSwitchReceipt) { reply.Sequence = 1 }},
		{"sequence outside non-ESN", func(reply *espFormSwitchReceipt) { reply.Sequence = 1 << 32 }},
		{"reset sequence", func(reply *espFormSwitchReceipt) { reply.Sequence = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newESPFormSwitchFixture(t)
			reply := espFormSwitchObservation("probe", "udp", 102)
			test.change(&reply)
			fixture.answerReceipt(t, espFormSwitchProbe(2), reply)
			if err := checkESPFormChange(context.Background(), fixture.lab()); err == nil {
				t.Fatal("invalid peer wire evidence accepted")
			}
			if got := fixture.probes; got != 2 {
				t.Fatalf("invalid UDP evidence was not reached: observed %d probes, want 2", got)
			}
		})
	}
}

// TestESPFormSwitchRejectsInvalidMigrationEvidence requires the atomic change
// receipt to retain the requested identity, form and live replay sequence.
func TestESPFormSwitchRejectsInvalidMigrationEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*espFormSwitchReceipt)
	}{
		{"action", func(reply *espFormSwitchReceipt) { reply.Action = "bare" }},
		{"source", func(reply *espFormSwitchReceipt) { reply.Source = zeIP }},
		{"target", func(reply *espFormSwitchReceipt) { reply.Target = swanIP }},
		{"spi", func(reply *espFormSwitchReceipt) { reply.SPI = 9 }},
		{"reqid", func(reply *espFormSwitchReceipt) { reply.ReqID = 9 }},
		{"form", func(reply *espFormSwitchReceipt) { reply.Form = "bare" }},
		{"replay reset", func(reply *espFormSwitchReceipt) { reply.Sequence = 100 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newESPFormSwitchFixture(t)
			reply := espFormSwitchObservation("udp", "udp", 101)
			test.change(&reply)
			fixture.answerReceipt(t, espFormSwitchUDP(), reply)
			if err := checkESPFormChange(context.Background(), fixture.lab()); err == nil {
				t.Fatal("invalid atomic migration receipt accepted")
			}
			key := fakeCommandKey(swanPeer, espFormSwitchUDP())
			if got := fixture.indexes[key]; got != 1 {
				t.Fatalf("invalid migration receipt was not reached exactly once: %d", got)
			}
			if fixture.probes != 1 {
				t.Fatal("the checker probed UDP before refusing its invalid migration receipt")
			}
		})
	}
}

// TestESPFormSwitchRejectsARefusedOperation requires acknowledged peer changes
// and a successful wire-and-delivery probe, through the existing Lab.Exec seam.
func TestESPFormSwitchRejectsARefusedOperation(t *testing.T) {
	for _, argv := range [][]string{espFormSwitchUDP(), espFormSwitchBare(), espFormSwitchProbe(2)} {
		t.Run(strings.Join(argv[1:], "_"), func(t *testing.T) {
			fixture := newESPFormSwitchFixture(t)
			key := fakeCommandKey(swanPeer, argv)
			fixture.responses[key] = []interoplab.CommandResult{{ExitCode: 1, Stderr: "peer operation refused"}}
			if err := checkESPFormChange(context.Background(), fixture.lab()); err == nil {
				t.Fatal("refused peer operation accepted")
			}
			if got := fixture.indexes[key]; got != 1 {
				t.Fatalf("refused operation was not reached exactly once: %d", got)
			}
		})
	}
}

// TestESPFormSwitchRejectsUnreadableProbeEvidence refuses absent or malformed
// helper output after the checker has reached the UDP probe.
func TestESPFormSwitchRejectsUnreadableProbeEvidence(t *testing.T) {
	for _, output := range []string{"", "{}", "{broken"} {
		t.Run(output, func(t *testing.T) {
			fixture := newESPFormSwitchFixture(t)
			fixture.answer(swanPeer, espFormSwitchProbe(2), output)
			if err := checkESPFormChange(context.Background(), fixture.lab()); err == nil {
				t.Fatal("unreadable peer wire evidence accepted")
			}
			if got := fixture.probes; got != 2 {
				t.Fatalf("unreadable UDP evidence was not reached: observed %d probes, want 2", got)
			}
		})
	}
}

// TestESPFormSwitchRetainsSafetyChecks keeps the counter-read, raw-drop and
// peer-log refusals beside the new same-SA phase proof.
func TestESPFormSwitchRetainsSafetyChecks(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*espFormSwitchFixture)
	}{
		{"missing mismatch counter", func(fixture *espFormSwitchFixture) {
			fixture.answer(zePeer, []string{"cat", "/proc/net/xfrm_stat"}, "XfrmInNoPols 0\n")
		}},
		{"raw socket drops", func(fixture *espFormSwitchFixture) {
			fixture.answer(zePeer, []string{"cat", "/proc/net/raw"}, espFormSwitchRaw(0), espFormSwitchRaw(1))
		}},
		{"peer DELETE", func(fixture *espFormSwitchFixture) {
			fixture.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: "received DELETE for ESP CHILD_SA\n"}
		}},
		{"peer REKEY", func(fixture *espFormSwitchFixture) {
			fixture.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: "REKEY_SA\n"}
		}},
		{"changed Ze SPI", func(fixture *espFormSwitchFixture) {
			fixture.zeAfter = strings.ReplaceAll(fixture.zeState, "spi 0x2", "spi 0x9")
		}},
		{"changed Ze reqid", func(fixture *espFormSwitchFixture) {
			fixture.zeAfter = strings.ReplaceAll(fixture.zeState, "reqid 1", "reqid 9")
		}},
		{"changed Ze selector", func(fixture *espFormSwitchFixture) {
			fixture.zeAfter = strings.ReplaceAll(fixture.zeState, "src "+swanIP+"/32", "src 0.0.0.0/0")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newESPFormSwitchFixture(t)
			test.change(fixture)
			if err := checkESPFormChange(context.Background(), fixture.lab()); err == nil {
				t.Fatal("unsafe form-switch evidence accepted")
			}
		})
	}
}

type espFormSwitchReceipt struct {
	Action   string `json:"action"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	SPI      uint32 `json:"spi"`
	ReqID    uint32 `json:"reqid"`
	Form     string `json:"form"`
	Sequence uint64 `json:"sequence"`
}

type espFormSwitchStall struct {
	phase     int
	direction espDirection
}

// espFormSwitchFixture reuses the lab command seam. Only a helper probe moves
// its counters, so ordinary unprotected pings cannot satisfy the positive case.
type espFormSwitchFixture struct {
	*fakeCheckerLab
	form    string
	probes  int
	actions []string
	stall   *espFormSwitchStall
	zeState string
	zeAfter string
}

func newESPFormSwitchFixture(t *testing.T) *espFormSwitchFixture {
	t.Helper()
	fixture := &espFormSwitchFixture{fakeCheckerLab: scriptedLab(), form: "bare"}
	fixture.zeState = espFormSwitchState("udp")
	fixture.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
	fixture.answer(zePeer, []string{"cat", "/proc/net/raw"}, espFormSwitchRaw(0))
	fixture.answer(zePeer, []string{"cat", "/proc/net/xfrm_stat"}, "XfrmInStateMismatch 5\n")
	fixture.answer(zePeer, []string{"ping", "-c", "3", "-W", "2", swanIP}, losslessPing(3))
	fixture.answer(swanPeer, []string{"ping", "-c", "3", "-W", "2", zeIP}, losslessPing(3))
	fixture.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: "charon started\n"}
	fixture.answerReceipt(t, espFormSwitchProbe(1), espFormSwitchObservation("probe", "bare", 101))
	fixture.answerReceipt(t, espFormSwitchUDP(), espFormSwitchObservation("udp", "udp", 101))
	fixture.answerReceipt(t, espFormSwitchProbe(2), espFormSwitchObservation("probe", "udp", 102))
	fixture.answerReceipt(t, espFormSwitchBare(), espFormSwitchObservation("bare", "bare", 102))
	fixture.answerReceipt(t, espFormSwitchProbe(3), espFormSwitchObservation("probe", "bare", 103))
	return fixture
}

func (fixture *espFormSwitchFixture) lab() *scenarioLab {
	lab := checkerFixture(fixture.fakeCheckerLab)
	lab.check.Lab = fixture
	return lab
}

func (fixture *espFormSwitchFixture) Exec(ctx context.Context, peer string, argv []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if slices.Equal(argv, []string{"ip", "xfrm", "state"}) {
		fixture.indexes[fakeCommandKey(peer, argv)]++
		state := espFormSwitchState(fixture.form)
		if peer == zePeer {
			state = fixture.zeSnapshot()
		}
		return interoplab.CommandResult{Stdout: state}, nil
	}
	if slices.Equal(argv, []string{"ip", "xfrm", "state", "list", "src", swanIP, "dst", zeIP}) {
		fixture.indexes[fakeCommandKey(peer, argv)]++
		if peer != zePeer {
			return interoplab.CommandResult{}, errors.New("unexpected directed peer state query")
		}
		state := fixture.zeSnapshot()
		start := strings.Index(state, "src "+swanIP+" dst "+zeIP+"\n")
		if start < 0 {
			return interoplab.CommandResult{}, nil
		}
		return interoplab.CommandResult{Stdout: state[start:]}, nil
	}
	if slices.Equal(argv, []string{"ip", "-s", "xfrm", "state"}) {
		fixture.indexes[fakeCommandKey(peer, argv)]++
		outbound, inbound := zeEncrypts, zeDecrypts
		if peer == swanPeer {
			outbound, inbound = swanDecrypts, swanEncrypts
		}
		return interoplab.CommandResult{Stdout: espDump(fixture.bytes(outbound), fixture.bytes(inbound))}, nil
	}
	result, err := fixture.fakeCheckerLab.Exec(ctx, peer, argv, environment)
	if err != nil {
		return result, err
	}
	if peer != swanPeer {
		return result, nil
	}
	if len(argv) < 2 {
		return result, nil
	}
	if argv[0] != espFormSwitchProgram {
		return result, nil
	}
	fixture.actions = append(fixture.actions, strings.Join(argv[1:], " "))
	switch argv[1] {
	case "probe":
		fixture.probes++
	case "udp":
		fixture.form = "udp"
	case "bare":
		fixture.form = "bare"
	default:
		return interoplab.CommandResult{}, errors.New("unexpected peer helper action")
	}
	return result, nil
}

func (fixture *espFormSwitchFixture) zeSnapshot() string {
	if fixture.probes > 0 {
		if fixture.zeAfter != "" {
			return fixture.zeAfter
		}
	}
	return fixture.zeState
}

func (fixture *espFormSwitchFixture) Query(ctx context.Context, peer string, argv []string, environment []interoplab.EnvironmentVariable) (string, error) {
	result, err := fixture.Exec(ctx, peer, argv, environment)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", errors.New("query returned no output")
	}
	return result.Stdout, nil
}

func (fixture *espFormSwitchFixture) bytes(direction espDirection) int {
	advances := fixture.probes
	if fixture.stall != nil {
		if direction == fixture.stall.direction {
			if fixture.probes > fixture.stall.phase {
				advances--
			}
		}
	}
	return 100 + 10*advances
}

func (fixture *espFormSwitchFixture) answerReceipt(t *testing.T, argv []string, reply espFormSwitchReceipt) {
	t.Helper()
	encoded, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	fixture.answer(swanPeer, argv, string(encoded))
}

func (fixture *espFormSwitchFixture) requireSwitches(t *testing.T) {
	t.Helper()
	want := make([]string, 0, 5)
	for _, argv := range [][]string{espFormSwitchProbe(1), espFormSwitchUDP(), espFormSwitchProbe(2), espFormSwitchBare(), espFormSwitchProbe(3)} {
		want = append(want, strings.Join(argv[1:], " "))
		if got := fixture.indexes[fakeCommandKey(swanPeer, argv)]; got != 1 {
			t.Errorf("peer helper %v ran %d times, want once", argv, got)
		}
	}
	if !slices.Equal(fixture.actions, want) {
		t.Fatalf("peer operations = %v, want ordered bare/UDP/bare delivery probes %v", fixture.actions, want)
	}
	if fixture.form != "bare" {
		t.Fatalf("peer final form = %q, want bare", fixture.form)
	}
}

func espFormSwitchObservation(action, form string, sequence uint64) espFormSwitchReceipt {
	return espFormSwitchReceipt{Action: action, Source: swanIP, Target: zeIP, SPI: 2, ReqID: 1, Form: form, Sequence: sequence}
}

func espFormSwitchDirections() []espDirection {
	return []espDirection{zeEncrypts, swanDecrypts, swanEncrypts, zeDecrypts}
}

func espFormSwitchProbe(sequence int) []string {
	return []string{espFormSwitchProgram, "probe", "eth0", swanIP, zeIP, "0x2", "4301", strconv.Itoa(sequence)}
}

func espFormSwitchUDP() []string {
	return []string{espFormSwitchProgram, "udp", swanIP, zeIP, "0x2", "4500", "4500"}
}

func espFormSwitchBare() []string {
	return []string{espFormSwitchProgram, "bare", swanIP, zeIP, "0x2"}
}

func espFormSwitchState(form string) string {
	state := xfrmSA(zeIP, swanIP, "0x1", 0) + "\tsel src " + zeIP + "/32 dst " + swanIP + "/32\n" +
		xfrmSA(swanIP, zeIP, "0x2", 0) + "\tsel src " + swanIP + "/32 dst " + zeIP + "/32\n"
	if form == "udp" {
		state = strings.Replace(state, "spi 0x2 reqid 1 mode tunnel", "spi 0x2 reqid 1 mode tunnel encap type espinudp sport 4500 dport 4500", 1)
	}
	return state
}

func espFormSwitchRaw(drops int) string {
	return "   1: 00000000:0032 00000000:0000 07 00000000:00000000 00:00000000 00000000 0 0 12345 2 0000000000000000 " + strconv.Itoa(drops) + "\n"
}
