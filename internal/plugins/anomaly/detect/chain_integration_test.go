// Design: docs/architecture/anomaly/anomaly-4-interop-harness.md -- facts->judgment->response end to end.
package detect

import (
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/trafficfeature"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/plugins/anomaly/shape"
	"github.com/ze-software/ze/pkg/ze"
)

// chainTestBus is a synchronous in-process ze.EventBus (mirrors ddosevent's
// testBus): Emit delivers to matching subscribers immediately, in-process.
type chainTestBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newChainTestBus() *chainTestBus { return &chainTestBus{subs: make(map[string][]func(any))} }

func (b *chainTestBus) Emit(ns, et string, payload any) (int, error) {
	b.mu.Lock()
	hs := append([]func(any){}, b.subs[ns+"\x00"+et]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return len(hs), nil
}

func (b *chainTestBus) Subscribe(ns, et string, handler func(any)) func() {
	key := ns + "\x00" + et
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], handler)
	b.mu.Unlock()
	return func() {}
}

var _ ze.EventBus = (*chainTestBus)(nil)

// publishSyntheticFlow sends one flow-byte observation to the global feed, the
// same shape a real flow source (flowexport) publishes.
func publishSyntheticFlow(src, dst string, port uint16, nbytes float64) {
	observation.Global().Publish(observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow:    observation.FlowKey{Src: netip.MustParseAddr(src), Dst: netip.MustParseAddr(dst), DstPort: port},
		Value:   nbytes,
	})
}

// assertNoIncidentOnAxis fails when any incident in the ring sits on the given
// entity axis. It is scoped to one axis on purpose: each chain test asserts about the
// axis it exercises, and an incident on another axis is another test's subject.
func assertNoIncidentOnAxis(t *testing.T, d *detector, kind anomalyevent.EntityKind, phase string) {
	t.Helper()
	incs := d.recentIncidents()
	for i := range incs {
		if incs[i].EntityKind == kind {
			t.Fatalf("a %s incident fired during %s: %+v", kind, phase, incs[i])
		}
	}
}

// publishFlow sends one flow-byte observation with both transport ports and the
// protocol spelled out, which is what a real exporter publishes and what the port
// axis needs to tell a service port from a client socket.
func publishFlow(src, dst string, srcPort, dstPort uint16, proto uint8, nbytes float64) {
	observation.Global().Publish(observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow: observation.FlowKey{
			Src: netip.MustParseAddr(src), Dst: netip.MustParseAddr(dst),
			SrcPort: srcPort, DstPort: dstPort, Proto: proto,
		},
		Value: nbytes,
	})
}

// TestChainDestOutlier proves the DESTINATION axis end to end on real facts: a /24 of
// ordinary receivers plus one target that many sources sweep across many ports flows
// through a real trafficfeature.Service into the real detector, which confirms an
// incident tagged kind=dest with the target's prefix.
//
// It drives real 1s ticks and takes about 15 seconds. Unlike TestChainFactsToResponse
// it has no -short escape, because it is the ONLY functional proof of its acceptance
// criteria and nothing else in the suite exercises the dest axis end to end.
//
// VALIDATES: child-5 AC-1 + AC-4 and user story 1 -- the dest feature vector is
// derived from real observations (not a crafted snapshot) and the judgment layer
// scores it against its dest-prefix cohort.
// PREVENTS: a break anywhere in feed -> dest accumulator -> snapshot -> dest baseline
// -> confirm that per-layer unit tests cannot see, and a dest incident that arrives
// tagged as a source (which the responder would act on).
func TestChainDestOutlier(t *testing.T) {
	tf := trafficfeature.NewService(observation.Global())
	tfID := tf.Attach()
	defer tf.Detach(tfID)

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 2
	d := newDetector(cfg, nil)

	// Ordinary receivers: one client each, on one well-known port, answering.
	type server struct{ addr, client string }
	normals := []server{
		{"198.18.0.1", "198.51.100.1"},
		{"198.18.0.2", "198.51.100.2"},
		{"198.18.0.3", "198.51.100.3"},
		{"198.18.0.4", "198.51.100.4"},
		{"198.18.0.5", "198.51.100.5"},
	}
	const target = "198.18.0.9" // same /24, so cohort rarity measures it against them
	const targetClient = "198.51.100.9"

	served := func(s server) {
		publishFlow(s.client, s.addr, 40000, 443, 6, 1000)
		publishFlow(s.addr, s.client, 443, 40000, 6, 1000)
	}
	tick := func() {
		time.Sleep(1100 * time.Millisecond) // let trafficfeature finalize a 1s window
		d.onTick(tf.Snapshot())
	}

	// Warmup: the target behaves like its neighbors, so the baselines settle and the
	// new-peer flag expires (newPeerTicks=5).
	for range 7 {
		for _, n := range normals {
			served(n)
		}
		served(server{target, targetClient})
		tick()
	}
	// Nothing on the axis under test may fire while every receiver looks alike. The
	// SOURCE axis is not asserted here: a first-sighting sender whose dominant
	// destination port is uncommon fires new-peer plus rare-port on its own, which is
	// the source axis's existing behavior and is not what this test is about.
	assertNoIncidentOnAxis(t, d, anomalyevent.EntityKindDest, "warmup")

	// Attack: the neighbors carry on; the target is swept by many sources across many
	// high ports and answers none of them.
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		for _, n := range normals {
			served(n)
		}
		for i := range 12 {
			publishFlow(netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}).String(),
				target, 40000, uint16(20000+i), 6, 4000)
		}
		tick()
		incs := d.recentIncidents()
		for i := range incs {
			if incs[i].EntityKind == anomalyevent.EntityKindDest &&
				incs[i].Entity == netip.MustParsePrefix(target+"/32") {
				if incs[i].Cohort != "198.18.0.0/24" {
					t.Errorf("dest incident cohort = %q, want 198.18.0.0/24", incs[i].Cohort)
				}
				return // chain proven: real facts -> dest baseline -> kind=dest incident
			}
		}
	}
	t.Fatalf("no kind=dest incident for the swept target; incidents=%+v", d.recentIncidents())
}

// TestChainPortOutlier proves the destination-PORT axis end to end on real facts: a
// service port that suddenly answers many sources far more than they ask it becomes
// its own entity and confirms an incident tagged kind=port, scored against its own
// history alone.
//
// It runs unconditionally for the same reason TestChainDestOutlier does.
//
// VALIDATES: child-5 AC-2 + AC-5 and user story 2 -- the port feature vector is
// derived from real observations and scored cohort-free.
// PREVENTS: a break in feed -> port accumulator -> snapshot -> port baseline, a client
// socket being tracked as a service, and a port incident arriving with no identity.
func TestChainPortOutlier(t *testing.T) {
	tf := trafficfeature.NewService(observation.Global())
	tfID := tf.Attach()
	defer tf.Detach(tfID)

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 2
	d := newDetector(cfg, nil)

	const service = "198.19.0.1"
	const port = 31337 // the port under test, udp
	tick := func() {
		time.Sleep(1100 * time.Millisecond)
		d.onTick(tf.Snapshot())
	}
	// quiet is the service as it normally behaves: one client, answering evenly.
	quiet := func() {
		publishFlow("198.51.100.20", service, 45000, port, 17, 1000)
		publishFlow(service, "198.51.100.20", port, 45000, 17, 1000)
	}

	for range 7 {
		quiet()
		tick()
	}
	// As in TestChainDestOutlier, only the axis under test is asserted: the service
	// answering a client's ephemeral port makes it a first-sighting SOURCE with an
	// uncommon dominant destination port, which the source axis reports on its own.
	assertNoIncidentOnAxis(t, d, anomalyevent.EntityKindPort, "warmup")

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		// A reflection sweep: many sources ask, the service answers each of them
		// twenty times over.
		for i := range 12 {
			client := netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 100)}).String()
			publishFlow(client, service, 45000, port, 17, 500)
			publishFlow(service, client, port, 45000, 17, 10000)
		}
		tick()
		incs := d.recentIncidents()
		for i := range incs {
			inc := &incs[i]
			if inc.EntityKind != anomalyevent.EntityKindPort {
				continue
			}
			if inc.Port != port || inc.Proto != 17 {
				t.Fatalf("port incident identity = %d/%d, want 17/%d", inc.Proto, inc.Port, port)
			}
			if inc.Entity.IsValid() {
				t.Errorf("port incident entity = %v, want the zero prefix", inc.Entity)
			}
			if inc.Cohort != "" {
				t.Errorf("port incident cohort = %q, want empty (cohort-free)", inc.Cohort)
			}
			return // chain proven: real facts -> port baseline -> kind=port incident
		}
	}
	t.Fatalf("no kind=port incident for the reflecting service; incidents=%+v", d.recentIncidents())
}

// TestChainFactsToResponse proves facts->judgment->response in-process: a
// same-/24 normal cohort plus one high-fan-out / rare-port outlier, published as
// real observations, flow through a real trafficfeature.Service into the real
// detector, which confirms an incident and emits it on the bus, and the real
// anomaly/shape responder arms the outlier.
//
// VALIDATES: real feature data (not a crafted snapshot) drives the whole chain,
// end to end, in one test -- the gap the three existing anomaly .ci tests leave.
// PREVENTS: a silent break anywhere in facts->judgment->response that per-layer
// unit tests cannot catch (feed gate, feature vector, scoring, emit, arm).
func TestChainFactsToResponse(t *testing.T) {
	// -short skip only. The verify gate runs `go test` WITHOUT -short
	// (Makefile GO_TEST), so this integration test still runs in CI; -short only
	// lets local iteration skip its ~10s of real 1s ticks. No coverage is dropped.
	if testing.Short() {
		t.Skip("integration test drives real 1s trafficfeature ticks (~10s); run without -short")
	}
	bus := newChainTestBus()

	tf := trafficfeature.NewService(observation.Global())
	tfID := tf.Attach() // starts the service and subscribes to observation.Global()
	defer tf.Detach(tfID)

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 2 // confirm after 2 consecutive above-threshold ticks
	d := newDetector(cfg, bus)

	armedList, stop := shape.SubscribeForTest(bus)
	defer stop()

	// A same-/24 cohort. Each normal is paired 1:1 with a partner in a different
	// /24 so its traffic is BALANCED (in+out -> finite ratio, not exfil) and its
	// fan-out is 1. Only the outlier will later go pure-outbound and fan out.
	type peer struct{ src, partner string }
	normals := []peer{
		{"10.0.0.1", "203.0.113.101"},
		{"10.0.0.2", "203.0.113.102"},
		{"10.0.0.3", "203.0.113.103"},
		{"10.0.0.4", "203.0.113.104"},
		{"10.0.0.5", "203.0.113.105"},
	}
	const outlier = "10.0.0.9" // same /24 as the cohort, so cohort rarity measures it against peers
	const outlierPartner = "203.0.113.109"
	outDests := []string{
		"203.0.113.10", "203.0.113.11", "203.0.113.12", "203.0.113.13",
		"203.0.113.14", "203.0.113.15", "203.0.113.16", "203.0.113.17",
		"203.0.113.18", "203.0.113.19", "203.0.113.20", "203.0.113.21",
	}

	// balanced injects one out and one in flow so src's out/in ratio is ~1.
	balanced := func(src, prt string) {
		publishSyntheticFlow(src, prt, 443, 1000)
		publishSyntheticFlow(prt, src, 443, 1000)
	}
	tick := func() {
		time.Sleep(1100 * time.Millisecond) // let trafficfeature finalize a 1s window
		d.onTick(tf.Snapshot())
	}

	// Warmup: every source (including the future outlier) behaves normally, so
	// baselines establish and the new-peer flag expires (newPeerTicks=5).
	for range 7 {
		for _, n := range normals {
			balanced(n.src, n.partner)
		}
		balanced(outlier, outlierPartner)
		tick()
	}
	if got := len(armedList()); got != 0 {
		t.Fatalf("normal cohort should not arm during warmup, armed=%v", armedList())
	}

	// Attack: normals stay balanced; the outlier goes pure-outbound, fanning out
	// across many dests on distinct high (rare) ports -- exfil + scan behavior.
	const outlierPrefix = "10.0.0.9/32"
	outlierArmed := func() bool { return slices.Contains(armedList(), outlierPrefix) }
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		for _, n := range normals {
			balanced(n.src, n.partner)
		}
		for i, dst := range outDests {
			publishSyntheticFlow(outlier, dst, uint16(10000+i), 5000)
		}
		tick()
		if len(d.recentIncidents()) > 0 && outlierArmed() {
			return // chain proven: real facts -> incident -> the outlier armed
		}
	}

	if len(d.recentIncidents()) == 0 {
		t.Fatalf("no anomaly incident fired from real trafficfeature data")
	}
	t.Fatalf("outlier %s not armed after sustained anomaly; armed=%v", outlier, armedList())
}
