// Design: docs/architecture/anomaly/anomaly-4-interop-harness.md -- facts->judgment->response end to end.
package detect

import (
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/trafficfeature"
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
	// test-relax: -short skip only. The verify gate runs `go test` WITHOUT -short
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
