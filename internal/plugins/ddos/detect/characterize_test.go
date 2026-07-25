package detect

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

// frec builds a flowRecord (destined to the victim) for classifier tests.
func frec(proto uint8, src string, sport, dport uint16, pkts uint64, tcpState uint8) flowRecord {
	return flowRecord{
		SrcAddr:  netip.MustParseAddr(src),
		DstAddr:  netip.MustParseAddr(victimIP),
		SrcPort:  sport,
		DstPort:  dport,
		Protocol: proto,
		Packets:  pkts,
		TCPState: tcpState,
	}
}

// VALIDATES: parseTopDestination picks the highest-byte destination as a host
// prefix and rejects empty/absent/malformed/bad-IP input (Phase 1 target fill).
// PREVENTS: a malformed or empty trafficusage response producing a bogus or
// panicking target prefix instead of a clean fallback.
func TestParseTopDestination(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string // prefix string; "" means expect ok=false
	}{
		{"dominant dst", `{"egress-ips":[{"ip":"203.0.113.5","bytes":9000},{"ip":"203.0.113.6","bytes":100}]}`, "203.0.113.5/32"},
		{"single dst", `{"egress-ips":[{"ip":"198.51.100.7","bytes":1}]}`, "198.51.100.7/32"},
		{"order independent", `{"egress-ips":[{"ip":"203.0.113.6","bytes":100},{"ip":"203.0.113.5","bytes":9000}]}`, "203.0.113.5/32"},
		{"empty list", `{"egress-ips":[]}`, ""},
		{"field absent", `{"interface":"xe0","ingress-ports":[]}`, ""},
		{"not-configured", `{"status":"not-configured"}`, ""},
		{"malformed json", `not json`, ""},
		{"bad ip", `{"egress-ips":[{"ip":"not-an-ip","bytes":5}]}`, ""},
		{"blank ip skipped", `{"egress-ips":[{"ip":"","bytes":9999},{"ip":"203.0.113.8","bytes":3}]}`, "203.0.113.8/32"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseTopDestination(json.RawMessage(tc.data))
			if tc.want == "" {
				if ok {
					t.Fatalf("expected ok=false, got %v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected ok=true with %s", tc.want)
			}
			if got.String() != tc.want {
				t.Errorf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestCharacterizeTargetQueriesAndParses(t *testing.T) {
	var gotCmd string
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		gotCmd = cmd
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.9","bytes":5000}]}`), nil
	})
	prefix, ok := d.characterizeTarget(context.Background(), "xe0")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if prefix.String() != "203.0.113.9/32" {
		t.Errorf("prefix: got %s want 203.0.113.9/32", prefix)
	}
	if gotCmd != "show traffic usage name xe0" {
		t.Errorf("command: got %q want \"show traffic usage name xe0\"", gotCmd)
	}
}

func TestCharacterizeTargetFallbackOnError(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return "", nil, errors.New("ErrUnknownCommand")
	})
	if _, ok := d.characterizeTarget(context.Background(), "xe0"); ok {
		t.Error("expected ok=false on dispatch error")
	}
}

// transitUsage models test/plugin/ddos-transit-forward-drop.ci's topology in the
// shape `show traffic usage` (no argument) answers with: an ARRAY of interface
// objects. The flood arrives on the veth peer zdd0p and is forwarded out zdd0, so
// only zdd0 has an egress destination; zdd0p records the SOURCE it saw ingressing
// (trafficusage/program_linux.go:88-97) and carries no egress-ips at all.
const transitUsage = `[` +
	`{"interface":"zdd0p","ingress-ips":[{"ip":"203.0.113.1","bytes":90000}]},` +
	`{"interface":"zdd0","egress-ips":[{"ip":"203.0.113.9","bytes":90000},{"ip":"203.0.113.4","bytes":10}]}` +
	`]`

// VALIDATES: when the ATTACKED interface reports a destination, the box-wide
// fallback is never consulted and the resolved victim is byte-identical to the
// pre-fallback behavior -- exactly one dispatch, the per-interface one.
// PREVENTS: the fallback silently widening the victim pick for topologies that
// already resolve one (every loopback deployment and test), which would let a
// busier unrelated destination outrank the real victim on the attacked link.
func TestCharacterizeTargetPrefersAttackedInterface(t *testing.T) {
	var cmds []string
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		cmds = append(cmds, cmd)
		// The attacked interface resolves; the box-wide view would pick a
		// DIFFERENT, busier address, so consulting it would be observable.
		if cmd == "show traffic usage name lo" {
			return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"127.0.0.4","bytes":5000}]}`), nil
		}
		return statusDone, json.RawMessage(`[{"interface":"eth0","egress-ips":[{"ip":"198.51.100.1","bytes":999999}]}]`), nil
	})

	prefix, ok := d.characterizeTarget(context.Background(), "lo")
	if !ok {
		t.Fatal("expected ok=true from the attacked interface")
	}
	if prefix.String() != "127.0.0.4/32" {
		t.Errorf("victim: got %s want 127.0.0.4/32 (the attacked interface's own destination)", prefix)
	}
	if len(cmds) != 1 || cmds[0] != "show traffic usage name lo" {
		t.Errorf("expected exactly one per-interface dispatch, got %q", cmds)
	}
}

// VALIDATES: when the attacked interface reports NO destination, the victim is
// resolved box-wide from the interface the attack is forwarded OUT of.
// PREVENTS: the forwarded-attack blind spot -- the detector attributes to the
// top-RECEIVE interface (detector.go:200-226) while egress-ips holds destinations
// of packets LEAVING an interface (trafficusage/program_linux.go:88-97), so on an
// in-A/out-B path the victim can never be in A's egress-ips. Before the fallback
// this left AttackDetected with an empty target on every transit topology, which
// is how ddos-transit-forward-drop.ci failed in QEMU.
func TestCharacterizeTargetBoxWideFallbackResolvesForwardedVictim(t *testing.T) {
	var cmds []string
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		cmds = append(cmds, cmd)
		// zdd0p is monitored and answers cleanly -- it simply has no egress
		// destination, which is the whole defect. Not an error path.
		if cmd == "show traffic usage name zdd0p" {
			return statusDone, json.RawMessage(`{"interface":"zdd0p","ingress-ips":[{"ip":"203.0.113.1","bytes":90000}]}`), nil
		}
		return statusDone, json.RawMessage(transitUsage), nil
	})

	prefix, ok := d.characterizeTarget(context.Background(), "zdd0p")
	if !ok {
		t.Fatal("expected the box-wide fallback to resolve the forwarded victim")
	}
	if prefix.String() != "203.0.113.9/32" {
		t.Errorf("victim: got %s want 203.0.113.9/32 (top destination on the egress interface)", prefix)
	}
	if len(cmds) != 2 || cmds[1] != "show traffic usage" {
		t.Errorf("expected the per-interface query then the box-wide one, got %q", cmds)
	}
}

// VALIDATES: neither scope reporting a destination still yields ok=false.
// PREVENTS: the fallback inventing a victim from an empty box, which would defeat
// applyMitigation's unresolved-victim guard and reintroduce a bogus drop.
func TestCharacterizeTargetBoxWideFallbackStillFails(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		if cmd == "show traffic usage" {
			return statusDone, json.RawMessage(`[{"interface":"zdd0p","ingress-ips":[{"ip":"203.0.113.1","bytes":1}]}]`), nil
		}
		return statusDone, json.RawMessage(`{"interface":"zdd0p"}`), nil
	})
	if prefix, ok := d.characterizeTarget(context.Background(), "zdd0p"); ok {
		t.Errorf("expected ok=false when no interface reports a destination, got %s", prefix)
	}
}

// VALIDATES: parseTopDestinationAcrossInterfaces ranks destinations across every
// interface in the argument-less array response and rejects malformed/empty input.
// PREVENTS: the array shape (show.go returns a Slice with no argument, a Map with
// `name`) being parsed as the object shape and silently yielding no victim.
func TestParseTopDestinationAcrossInterfaces(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string // prefix string; "" means expect ok=false
	}{
		{"forwarded victim", transitUsage, "203.0.113.9/32"},
		{"ranks across interfaces", `[{"egress-ips":[{"ip":"203.0.113.5","bytes":10}]},{"egress-ips":[{"ip":"203.0.113.6","bytes":9000}]}]`, "203.0.113.6/32"},
		{"order independent", `[{"egress-ips":[{"ip":"203.0.113.6","bytes":9000}]},{"egress-ips":[{"ip":"203.0.113.5","bytes":10}]}]`, "203.0.113.6/32"},
		{"skips interfaces with none", `[{"interface":"a"},{"egress-ips":[{"ip":"198.51.100.7","bytes":1}]}]`, "198.51.100.7/32"},
		{"empty array", `[]`, ""},
		{"no egress anywhere", `[{"interface":"a"},{"interface":"b","ingress-ips":[{"ip":"10.0.0.1","bytes":9}]}]`, ""},
		{"object shape rejected", `{"egress-ips":[{"ip":"203.0.113.5","bytes":9000}]}`, ""},
		{"malformed json", `not json`, ""},
		{"bad ip", `[{"egress-ips":[{"ip":"not-an-ip","bytes":5}]}]`, ""},
		{"blank ip skipped", `[{"egress-ips":[{"ip":"","bytes":9999},{"ip":"203.0.113.8","bytes":3}]}]`, "203.0.113.8/32"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseTopDestinationAcrossInterfaces(json.RawMessage(tc.data))
			if tc.want == "" {
				if ok {
					t.Fatalf("expected ok=false, got %v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected ok=true with %s", tc.want)
			}
			if got.String() != tc.want {
				t.Errorf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestCharacterizeTargetFallbackOnNonDone(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return "error", nil, nil
	})
	if _, ok := d.characterizeTarget(context.Background(), "xe0"); ok {
		t.Error("expected ok=false when status != done")
	}
}

func TestCharacterizeTargetFallbackOnNilDispatch(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), nil)
	if _, ok := d.characterizeTarget(context.Background(), "xe0"); ok {
		t.Error("expected ok=false when no dispatch is wired")
	}
}

// floodInto drives the detector from idle to an active attack on iface "xe0".
func floodInto(d *detector) {
	var cumPkts uint64
	for range 20 {
		cumPkts += 50
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cumPkts}}})
	}
	for range 5 {
		cumPkts += 100000
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cumPkts}}})
	}
	d.wg.Wait()
}

func floodConfig() *Config {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 1
	cfg.AbsoluteFloor = 100
	cfg.BaselineWindow = 10
	cfg.StartupGrace = 0
	return cfg
}

// AC-1: a flood produces an AttackDetected carrying a valid target prefix.
func TestDetectorEmitsTargetOnTrigger(t *testing.T) {
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.42","bytes":1000000}]}`), nil
	})

	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	floodInto(d)

	if detected == nil {
		t.Fatal("AttackDetected not emitted")
	}
	if !detected.Target.DstPrefix.IsValid() {
		t.Fatal("expected a valid target prefix")
	}
	if detected.Target.DstPrefix.String() != "203.0.113.42/32" {
		t.Errorf("target: got %s want 203.0.113.42/32", detected.Target.DstPrefix)
	}
}

// AC-10: with no reachable source the detector still emits, with an empty target
// and the generic family -- behavior no worse than before characterization.
func TestDetectorFallbackWhenSourceAbsent(t *testing.T) {
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return "", nil, errors.New("ErrUnknownCommand")
	})

	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	floodInto(d)

	if detected == nil {
		t.Fatal("AttackDetected must still be emitted on fallback")
	}
	if detected.Target.DstPrefix.IsValid() {
		t.Errorf("expected empty target on source-absent, got %s", detected.Target.DstPrefix)
	}
	if detected.Family != ddosevent.FamilyGenericFlood {
		t.Errorf("family: got %s want %s", detected.Family, ddosevent.FamilyGenericFlood)
	}
}

// VALIDATES: Ongoing is suppressed until the asynchronous Detected has been
// emitted, preserving Detected-before-Ongoing ordering for subscribers.
// PREVENTS: the ordering regression from making Detected async -- a slow flow
// query must not let an Ongoing reach subscribers before the attack's Detected.
func TestOngoingGatedUntilDetected(t *testing.T) {
	bus := newDTestBus()
	var mu sync.Mutex
	var order []string
	ddosevent.Detected.Subscribe(bus, func(_ *ddosevent.AttackDetected) {
		mu.Lock()
		order = append(order, "detected")
		mu.Unlock()
	})
	ddosevent.Ongoing.Subscribe(bus, func(_ *ddosevent.AttackOngoing) {
		mu.Lock()
		order = append(order, "ongoing")
		mu.Unlock()
	})

	release := make(chan struct{})
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		<-release // simulate a slow flow query so Detected is delayed
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.1","bytes":1}]}`), nil
	})

	var cum uint64
	for range 20 { // baseline
		cum += 50
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cum}}})
	}
	for range 5 { // flood: activate tick + ticks that would emit Ongoing
		cum += 100000
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cum}}})
	}

	// Detected is still blocked in the goroutine, so no event may have fired.
	mu.Lock()
	pre := append([]string(nil), order...)
	mu.Unlock()
	if len(pre) != 0 {
		t.Fatalf("no event may precede Detected, got %v", pre)
	}

	close(release)
	d.wg.Wait()

	cum += 100000 // one more active tick now that Detected has landed
	d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cum}}})

	mu.Lock()
	defer mu.Unlock()
	if len(order) == 0 || order[0] != "detected" {
		t.Fatalf("Detected must be delivered first, got %v", order)
	}
}

const victimIP = "203.0.113.42"

// TestClassifyFlows covers the family heuristics (AC-3..AC-6) and the narrowest
// vector built for each (proto / discriminating ports / TCP flags). DstPrefix is
// the caller's responsibility, so it stays zero here.
func TestClassifyFlows(t *testing.T) {
	cases := []struct {
		name       string
		flows      []flowRecord
		wantFamily ddosevent.AttackFamily
		wantProto  uint8
		wantSrc    uint16
		wantDst    uint16
		wantFlags  uint8
	}{
		{
			name: "reflection", // AC-3: UDP dominant, dominant source port is a reflector
			flows: []flowRecord{
				frec(protoUDP, "198.51.100.1", 53, 40000, 100, 0),
				frec(protoUDP, "198.51.100.2", 53, 40001, 120, 0),
				frec(protoUDP, "198.51.100.3", 53, 40002, 90, 0),
			},
			wantFamily: ddosevent.FamilyReflection, wantProto: protoUDP, wantSrc: 53,
		},
		{
			name: "syn-flood", // AC-4: TCP dominant AND half-open majority
			flows: []flowRecord{
				frec(protoTCP, "198.51.100.1", 4000, 80, 1, 2), // SYN_RECV
				frec(protoTCP, "198.51.100.2", 4001, 80, 1, 2),
				frec(protoTCP, "198.51.100.3", 4002, 80, 1, 1), // SYN_SENT
			},
			wantFamily: ddosevent.FamilySYNFlood, wantProto: protoTCP, wantDst: 80, wantFlags: tcpFlagSYN,
		},
		{
			name: "icmp-flood", // AC-5: ICMP dominant
			flows: []flowRecord{
				frec(protoICMP, "198.51.100.1", 0, 0, 200, 0),
				frec(protoICMP, "198.51.100.2", 0, 0, 180, 0),
				frec(protoUDP, "198.51.100.3", 1234, 5678, 10, 0),
			},
			wantFamily: ddosevent.FamilyICMPFlood, wantProto: protoICMP,
		},
		{
			name: "udp-flood", // UDP dominant, non-reflection source port, dominant dst port
			flows: []flowRecord{
				frec(protoUDP, "198.51.100.1", 40000, 80, 100, 0),
				frec(protoUDP, "198.51.100.2", 40001, 80, 100, 0),
				frec(protoUDP, "198.51.100.3", 40002, 80, 100, 0),
			},
			wantFamily: ddosevent.FamilyUDPFlood, wantProto: protoUDP, wantDst: 80,
		},
		{
			name: "generic", // AC-6: mixed proto, no dominance -> generic, no proto pinned
			flows: []flowRecord{
				frec(protoUDP, "198.51.100.1", 1111, 2222, 100, 0),
				frec(protoTCP, "198.51.100.2", 3333, 4444, 100, 3), // ESTABLISHED
			},
			wantFamily: ddosevent.FamilyGenericFlood,
		},
		{
			name: "tcp-non-syn-is-generic", // TCP dominant but established -> not SYN, stays generic
			flows: []flowRecord{
				frec(protoTCP, "198.51.100.1", 5000, 443, 100, 3),
				frec(protoTCP, "198.51.100.2", 5001, 443, 100, 3),
				frec(protoTCP, "198.51.100.3", 5002, 443, 100, 3),
			},
			wantFamily: ddosevent.FamilyGenericFlood, wantProto: protoTCP, wantDst: 443,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			family, vec, _, _ := classifyFlows(tc.flows, 10)
			if family != tc.wantFamily {
				t.Errorf("family: got %q want %q", family, tc.wantFamily)
			}
			if vec.Proto != tc.wantProto {
				t.Errorf("proto: got %d want %d", vec.Proto, tc.wantProto)
			}
			if vec.SrcPort != tc.wantSrc {
				t.Errorf("src-port: got %d want %d", vec.SrcPort, tc.wantSrc)
			}
			if vec.DstPort != tc.wantDst {
				t.Errorf("dst-port: got %d want %d", vec.DstPort, tc.wantDst)
			}
			if vec.TCPFlags != tc.wantFlags {
				t.Errorf("tcp-flags: got %#x want %#x", vec.TCPFlags, tc.wantFlags)
			}
			if vec.DstPrefix.IsValid() {
				t.Errorf("classifyFlows must leave DstPrefix to the caller, got %s", vec.DstPrefix)
			}
		})
	}
}

// TestTopSourcesRanking validates A-2: sources rank by packet volume descending,
// capped at topN, ties broken deterministically by address.
func TestTopSourcesRanking(t *testing.T) {
	flows := []flowRecord{
		frec(protoUDP, "198.51.100.10", 53, 1, 50, 0),
		frec(protoUDP, "198.51.100.20", 53, 1, 500, 0),
		frec(protoUDP, "198.51.100.30", 53, 1, 300, 0),
		frec(protoUDP, "198.51.100.20", 53, 1, 100, 0), // .20 total 600
	}
	_, _, top, _ := classifyFlows(flows, 2)
	if len(top) != 2 {
		t.Fatalf("top len = %d, want 2 (capped at topN)", len(top))
	}
	if top[0].String() != "198.51.100.20" {
		t.Errorf("top[0] = %s, want 198.51.100.20 (600 pkts)", top[0])
	}
	if top[1].String() != "198.51.100.30" {
		t.Errorf("top[1] = %s, want 198.51.100.30 (300 pkts)", top[1])
	}
}

// TestSourceEntropy validates the distributed/concentrated annotation: a single
// source is 0 bits, N equal sources is log2(N).
func TestSourceEntropy(t *testing.T) {
	a := netip.MustParseAddr("198.51.100.1")
	b := netip.MustParseAddr("198.51.100.2")
	c := netip.MustParseAddr("198.51.100.3")
	d := netip.MustParseAddr("198.51.100.4")

	if h := sourceEntropy(map[netip.Addr]uint64{a: 10}, 10); h != 0 {
		t.Errorf("single source entropy = %v, want 0", h)
	}
	if h := sourceEntropy(map[netip.Addr]uint64{a: 5, b: 5}, 10); math.Abs(h-1.0) > 1e-9 {
		t.Errorf("two equal sources entropy = %v, want 1.0", h)
	}
	if h := sourceEntropy(map[netip.Addr]uint64{a: 1, b: 1, c: 1, d: 1}, 4); math.Abs(h-2.0) > 1e-9 {
		t.Errorf("four equal sources entropy = %v, want 2.0", h)
	}
	// Concentrated (one dominant) has lower entropy than uniform.
	concentrated := sourceEntropy(map[netip.Addr]uint64{a: 97, b: 1, c: 1, d: 1}, 100)
	uniform := sourceEntropy(map[netip.Addr]uint64{a: 25, b: 25, c: 25, d: 25}, 100)
	if concentrated >= uniform {
		t.Errorf("concentrated entropy %v should be < uniform %v", concentrated, uniform)
	}
}

// TestCharacterizeEmitsCharacterized drives the full trigger path with a dispatch
// that answers both sources and asserts a populated AttackCharacterized reaches
// the bus (AC-3, AC-12, AC-13, Wiring: AttackCharacterized on bus).
func TestCharacterizeEmitsCharacterized(t *testing.T) {
	flowJSON := `[` +
		`{"src-addr":"198.51.100.1","dst-addr":"203.0.113.42","src-port":53,"dst-port":40000,"protocol":17,"packets":100,"tcp-state":0},` +
		`{"src-addr":"198.51.100.2","dst-addr":"203.0.113.42","src-port":53,"dst-port":40001,"protocol":17,"packets":120,"tcp-state":0},` +
		`{"src-addr":"198.51.100.3","dst-addr":"203.0.113.42","src-port":53,"dst-port":40002,"protocol":17,"packets":90,"tcp-state":0}]`

	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		switch {
		case strings.HasPrefix(cmd, "show traffic usage"):
			return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.42","bytes":1000000}]}`), nil
		case strings.HasPrefix(cmd, "show flow recent"):
			return statusDone, json.RawMessage(flowJSON), nil
		}
		return "error", nil, errors.New("unexpected command: " + cmd)
	})

	var ch *ddosevent.AttackCharacterized
	ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) { ch = e })

	floodInto(d)

	if ch == nil {
		t.Fatal("AttackCharacterized not emitted")
	}
	if ch.Family != ddosevent.FamilyReflection {
		t.Errorf("family: got %q want reflection", ch.Family)
	}
	if ch.Target.DstPrefix.String() != "203.0.113.42/32" {
		t.Errorf("target: got %s want 203.0.113.42/32", ch.Target.DstPrefix)
	}
	if ch.Target.Proto != protoUDP || ch.Target.SrcPort != 53 {
		t.Errorf("vector: got proto=%d src-port=%d want 17/53", ch.Target.Proto, ch.Target.SrcPort)
	}
	if ch.Severity != ddosevent.SeverityCritical { // peak >> threshold
		t.Errorf("severity: got %q want critical", ch.Severity)
	}
	if len(ch.TopSources) == 0 {
		t.Error("expected populated TopSources")
	}
	if ch.SourceEntropy <= 0 {
		t.Errorf("expected positive source entropy, got %v", ch.SourceEntropy)
	}
	// A clear reflection flood well above threshold: base 25 + ratio 30 + specific
	// family 25 + reflection 10 = 90 (entropy of 3 sources is below the default 2.0
	// threshold, so no distributed bonus).
	if ch.Confidence < 90 {
		t.Errorf("expected high confidence for a clear reflection flood, got %d", ch.Confidence)
	}
}

// TestCharacterizeSkipsWhenNoFlowSource proves that with trafficusage present but
// flow-recent absent, AttackDetected still fires (coarse) and AttackCharacterized
// is skipped -- never worse than Phase 1.
func TestCharacterizeSkipsWhenNoFlowSource(t *testing.T) {
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		if strings.HasPrefix(cmd, "show traffic usage") {
			return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.42","bytes":1000000}]}`), nil
		}
		return "", nil, errors.New("ErrUnknownCommand") // flow-recent absent
	})

	var detected *ddosevent.AttackDetected
	var ch *ddosevent.AttackCharacterized
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })
	ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) { ch = e })

	floodInto(d)

	if detected == nil || !detected.Target.DstPrefix.IsValid() {
		t.Fatal("AttackDetected with valid target must still fire")
	}
	if ch != nil {
		t.Errorf("AttackCharacterized must be skipped when no flow source, got %+v", ch)
	}
}

// TestCharacterizeDerivesVictimFromFlows covers the flow-only path (AC-12): with
// trafficusage absent, the victim (here IPv6, invisible to trafficusage) is
// derived from the dominant destination in the recent-flow ring.
func TestCharacterizeDerivesVictimFromFlows(t *testing.T) {
	flowJSON := `[` +
		`{"src-addr":"2001:db8:aaaa::1","dst-addr":"2001:db8::1","src-port":123,"dst-port":40000,"protocol":17,"packets":100,"tcp-state":0},` +
		`{"src-addr":"2001:db8:aaaa::2","dst-addr":"2001:db8::1","src-port":123,"dst-port":40001,"protocol":17,"packets":120,"tcp-state":0},` +
		`{"src-addr":"2001:db8:aaaa::3","dst-addr":"2001:db8::1","src-port":123,"dst-port":40002,"protocol":17,"packets":90,"tcp-state":0}]`

	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		if strings.HasPrefix(cmd, "show flow recent") {
			return statusDone, json.RawMessage(flowJSON), nil
		}
		return "", nil, errors.New("ErrUnknownCommand") // trafficusage absent
	})

	var ch *ddosevent.AttackCharacterized
	ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) { ch = e })

	floodInto(d)

	if ch == nil {
		t.Fatal("AttackCharacterized not emitted from flow-only path")
	}
	if ch.Target.DstPrefix.String() != "2001:db8::1/128" {
		t.Errorf("derived victim: got %s want 2001:db8::1/128", ch.Target.DstPrefix)
	}
	if ch.Family != ddosevent.FamilyReflection || ch.Target.SrcPort != 123 {
		t.Errorf("expected reflection on NTP src-port 123, got family=%s src-port=%d", ch.Family, ch.Target.SrcPort)
	}
	if len(ch.TopSources) == 0 || !ch.TopSources[0].Is6() {
		t.Errorf("expected IPv6 top sources, got %v", ch.TopSources)
	}
}

// VALIDATES: characterizeFromFlows polls the recent-flow ring within the
// characterize budget: an initially-empty ring (the periodic conntrack dump has
// not yet landed the attack) is retried until it warms, then the attack is
// classified. AC-9 depends on this -- at production active-timeout (60s) the ring
// holds pre-attack state at confirm, so a single-shot read always fell back to
// generic-flood and confidence was never computed.
// PREVENTS: the confidence path silently degrading to generic-flood whenever the
// flow ring lags the attack by less than the characterize budget.
func TestCharacterizeRetriesUntilRingWarms(t *testing.T) {
	reflection := `[` +
		`{"src-addr":"198.51.100.1","dst-addr":"203.0.113.42","src-port":53,"dst-port":40000,"protocol":17,"packets":100,"tcp-state":0},` +
		`{"src-addr":"198.51.100.2","dst-addr":"203.0.113.42","src-port":53,"dst-port":40001,"protocol":17,"packets":120,"tcp-state":0},` +
		`{"src-addr":"198.51.100.3","dst-addr":"203.0.113.42","src-port":53,"dst-port":40002,"protocol":17,"packets":90,"tcp-state":0}]`

	// Shrink the poll interval so the test does not pace on the 150ms production
	// cadence; restore it after.
	saved := characterizeRetryInterval
	characterizeRetryInterval = time.Millisecond
	defer func() { characterizeRetryInterval = saved }()

	var mu sync.Mutex
	flowCalls := 0
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		switch {
		case strings.HasPrefix(cmd, "show traffic usage"):
			return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.42","bytes":1000000}]}`), nil
		case strings.HasPrefix(cmd, "show flow recent"):
			mu.Lock()
			flowCalls++
			n := flowCalls
			mu.Unlock()
			if n < 3 {
				return statusDone, json.RawMessage(`[]`), nil // ring not warm yet
			}
			return statusDone, json.RawMessage(reflection), nil
		}
		return "error", nil, errors.New("unexpected command: " + cmd)
	})

	var ch *ddosevent.AttackCharacterized
	ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) { ch = e })

	floodInto(d)

	if ch == nil {
		t.Fatal("AttackCharacterized not emitted after the ring warmed")
	}
	if ch.Family != ddosevent.FamilyReflection {
		t.Errorf("family: got %q want reflection", ch.Family)
	}
	mu.Lock()
	defer mu.Unlock()
	if flowCalls < 3 {
		t.Errorf("expected the ring to be polled until warm (>=3 calls), got %d", flowCalls)
	}
}

// VALIDATES: a hard flow-source absence (dispatch error) falls back immediately
// rather than polling for the whole characterize budget.
// PREVENTS: every trigger with no flow source stalling the characterization
// goroutine for characterize-timeout before emitting the coarse fallback.
func TestCharacterizeNoRetryOnSourceAbsent(t *testing.T) {
	saved := characterizeRetryInterval
	characterizeRetryInterval = time.Millisecond
	defer func() { characterizeRetryInterval = saved }()

	var mu sync.Mutex
	flowCalls := 0
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		if strings.HasPrefix(cmd, "show traffic usage") {
			return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.42","bytes":1000000}]}`), nil
		}
		mu.Lock()
		flowCalls++
		mu.Unlock()
		return "", nil, errors.New("ErrUnknownCommand") // flow source absent
	})

	var ch *ddosevent.AttackCharacterized
	ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) { ch = e })

	floodInto(d)

	if ch != nil {
		t.Errorf("AttackCharacterized must be skipped on source-absent, got %+v", ch)
	}
	mu.Lock()
	defer mu.Unlock()
	if flowCalls != 1 {
		t.Errorf("source-absent must not retry: flow queries = %d, want 1", flowCalls)
	}
}

// VALIDATES: a characterization that completes AFTER the attack has cleared does
// not emit a stale Detected (the generation guard drops it).
// PREVENTS: a stuck local drop installed by a late Detected with no matching
// Cleared to remove it (max-mitigation-duration is not enforced in ddos-local).
func TestNoStaleDetectedAfterClear(t *testing.T) {
	bus := newDTestBus()
	var mu sync.Mutex
	var events []string
	ddosevent.Detected.Subscribe(bus, func(_ *ddosevent.AttackDetected) {
		mu.Lock()
		events = append(events, "detected")
		mu.Unlock()
	})
	ddosevent.Cleared.Subscribe(bus, func(_ *ddosevent.AttackCleared) {
		mu.Lock()
		events = append(events, "cleared")
		mu.Unlock()
	})

	release := make(chan struct{})
	cfg := floodConfig()
	cfg.ClearConsecutive = 1 // clear quickly so it races the blocked query
	d := newDetector(cfg, bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		<-release // the attack will clear before this returns
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.1","bytes":1}]}`), nil
	})

	tick := func(p uint64) {
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: p}}})
	}

	var cum uint64
	for range 20 { // baseline
		cum += 50
		tick(cum)
	}
	cum += 100000 // flood: activate, spawns the (blocked) characterization goroutine
	tick(cum)
	tick(cum) // delta 0 -> below: active -> clearing
	tick(cum) // delta 0 -> below: clearing -> idle -> Cleared emitted

	close(release) // characterization completes now, after Cleared
	d.wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	for _, e := range events {
		if e == "detected" {
			t.Fatalf("stale Detected emitted after clear: %v", events)
		}
	}
	if len(events) == 0 || events[len(events)-1] != "cleared" {
		t.Fatalf("expected a Cleared and no Detected, got %v", events)
	}
}

// TestNoStaleCharacterizedAfterClear is the AttackCharacterized counterpart: a slow
// flow-recent query that returns after the attack cleared must not emit a stale
// Characterized. This is the race the review flagged -- without holding d.mu
// across the generation check and the emit, a Cleared could slip between them and
// ddos-local would install a drop with no matching Cleared to remove it.
func TestNoStaleCharacterizedAfterClear(t *testing.T) {
	bus := newDTestBus()
	var mu sync.Mutex
	var ch *ddosevent.AttackCharacterized
	var cleared bool
	ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) {
		mu.Lock()
		ch = e
		mu.Unlock()
	})
	ddosevent.Cleared.Subscribe(bus, func(_ *ddosevent.AttackCleared) {
		mu.Lock()
		cleared = true
		mu.Unlock()
	})

	release := make(chan struct{})
	cfg := floodConfig()
	cfg.ClearConsecutive = 1 // clear quickly so it races the blocked flow query
	d := newDetector(cfg, bus, func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		if strings.HasPrefix(cmd, "show traffic usage") {
			return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.1","bytes":1}]}`), nil
		}
		<-release // flow-recent blocks until the attack has cleared
		return statusDone, json.RawMessage(`[{"src-addr":"198.51.100.1","dst-addr":"203.0.113.1","src-port":53,"dst-port":1,"protocol":17,"packets":10,"tcp-state":0}]`), nil
	})

	tick := func(p uint64) {
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: p}}})
	}

	var cum uint64
	for range 20 { // baseline
		cum += 50
		tick(cum)
	}
	cum += 100000 // flood: activate; goroutine emits Detected then blocks in flow-recent
	tick(cum)

	// Wait for Detected to land (goroutine set detectedEmitted before blocking).
	for !d.detectedEmitted.Load() {
		runtime.Gosched()
	}

	tick(cum) // below: active -> clearing
	tick(cum) // below: clearing -> idle -> Cleared (attackGen advanced)

	close(release) // flow-recent returns now, after Cleared
	d.wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if !cleared {
		t.Fatal("expected the attack to have cleared")
	}
	if ch != nil {
		t.Fatalf("stale Characterized emitted after clear: %+v", ch)
	}
}

// TestStopFencesCharacterization proves Stop() prevents a later trigger from
// spawning a characterization goroutine (the WaitGroup Add-during-Wait fence).
func TestStopFencesCharacterization(t *testing.T) {
	bus := newDTestBus()
	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.1","bytes":1}]}`), nil
	})

	d.Stop() // stop before any attack

	// A flood after Stop must not spawn characterization or emit -- onAttackStart
	// sees d.stopped and returns without wg.Go, and wg.Wait stays at zero.
	floodInto(d)

	if detected != nil {
		t.Errorf("no Detected should be emitted after Stop, got %+v", detected)
	}
}
