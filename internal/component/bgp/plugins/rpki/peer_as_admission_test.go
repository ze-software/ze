// Design: docs/guide/rpki.md -- validation requires the negotiated peer identity.
// Related: rpki.go -- handleEvent admits JSON UPDATEs before validation.

package rpki

import (
	"encoding/json"
	"strings"
	"testing"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestJSONUpdateRequiresPeerAS drives the active JSON handler with an authorized
// route. Unknown or malformed identity must stop before validation, tracking and
// event emission; the known-AS control must produce all three real outputs.
func TestJSONUpdateRequiresPeerAS(t *testing.T) {
	for _, test := range []struct {
		name    string
		member  string
		refusal string
	}{
		{name: "known", member: `,"as":100`},
		{name: "missing", refusal: "unknown-peer-as"},
		{name: "zero", member: `,"as":0`, refusal: "unknown-peer-as"},
		{name: "null", member: `,"as":null`, refusal: "unknown-peer-as"},
		{name: "malformed", member: `,"as":"invalid"`, refusal: "malformed-peer-as"},
		{name: "out-of-range", member: `,"as":4294967296`, refusal: "malformed-peer-as"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := metrics.NewPrometheusRegistry()
			previous := rpkiMetricsPtr.Load()
			SetMetricsRegistry(reg)
			t.Cleanup(func() { rpkiMetricsPtr.Store(previous) })
			rp, bridge := aspaRolePlugin(t, "provider")
			rp.cache.Add(makeVRP("10.0.0.0/24", 24, 300))
			rp.aspaCache.Set(200, []uint32{100})
			rp.aspaCache.Set(300, []uint32{200})
			var emitted []json.RawMessage
			bridge.SetEmitEvent(func(_, _, _, _, raw string) (int, error) {
				emitted = append(emitted, json.RawMessage(raw))
				return 1, nil
			})
			rp.handleEvent(&bgp.Event{
				Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 71},
				Peer: json.RawMessage(`{"name":"ix-192.0.2.50","group":"ix",` +
					`"local":{"as":65000},"remote":{"address":"192.0.2.50"` + test.member + `}}`),
				ASPath: []uint32{100, 200, 300},
				FamilyOps: map[family.Family][]bgp.FamilyOperation{
					family.IPv4Unicast: {{Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}}},
				},
			})
			requests := drainRequests(rp.validateCh)
			if test.refusal != "" {
				if len(requests) != 0 {
					t.Errorf("unusable peer AS produced validation requests: %+v", requests)
				}
				if got := rp.buildDecisions(requests); len(got) != 0 {
					t.Errorf("unusable peer AS produced route decisions: %+v", got)
				}
				if got := rp.originTracker.count(); got != 0 {
					t.Errorf("unusable peer AS retained %d origin routes", got)
				}
				if got := rp.aspaTracker.count(); got != 0 {
					t.Errorf("unusable peer AS retained %d ASPA routes", got)
				}
				if len(emitted) != 0 {
					t.Errorf("unusable peer AS emitted validation events: %s", emitted)
				}
				want := `ze_rpki_updates_refused_total{reason="` + test.refusal + `"} 1`
				out := scrapeRPKIMetrics(t, reg)
				if !strings.Contains(out, want) {
					t.Errorf("refusal was not counted with its cause; want %s in metrics:\n%s", want, out)
				}
				if strings.Contains(out, "ze_rpki_validation_outcomes_total{") {
					t.Errorf("refused UPDATE contributed per-prefix validation outcomes:\n%s", out)
				}
				return
			}
			if len(requests) != 1 {
				t.Fatalf("known AS produced %d requests, want one", len(requests))
			}
			if got := requests[0]; got.state != ValidationValid || got.aspaState != ASPAValid {
				t.Errorf("authorized route did not validate: %+v", got)
			}
			decisions := rp.buildDecisions(requests)
			if len(decisions) != 1 {
				t.Fatalf("known AS produced %d decisions, want one", len(decisions))
			}
			if !decisions[0].Accept {
				t.Error("authorized route was not accepted")
			}
			key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
			origin, ok := rp.originTracker.routes[key]
			if !ok {
				t.Fatal("known AS did not retain the origin route")
			}
			if origin.peerASN != 100 {
				t.Errorf("origin tracker retained AS %d, want 100", origin.peerASN)
			}
			path, ok := rp.aspaTracker.routes[key]
			if !ok {
				t.Fatal("known AS did not retain the ASPA route")
			}
			if path.peerASN != 100 {
				t.Errorf("ASPA tracker retained AS %d, want 100", path.peerASN)
			}
			if len(emitted) != 1 {
				t.Fatalf("known AS emitted %d events, want one", len(emitted))
			}
			var event struct {
				BGP struct {
					Peer bgp.PeerInfoJSON `json:"peer"`
					RPKI struct {
						Prefixes map[string]string `json:"ipv4/unicast"`
						ASPA     string            `json:"aspa-state"`
					} `json:"rpki"`
				} `json:"bgp"`
			}
			if err := json.Unmarshal(emitted[0], &event); err != nil {
				t.Fatal(err)
			}
			if event.BGP.Peer.Remote.AS != 100 {
				t.Errorf("event named AS %d, want 100", event.BGP.Peer.Remote.AS)
			}
			if got := event.BGP.RPKI.Prefixes["10.0.0.0/24"]; got != "valid" {
				t.Errorf("emitted origin verdict %q, want valid", got)
			}
			if event.BGP.RPKI.ASPA != "valid" {
				t.Errorf("emitted ASPA verdict %q, want valid", event.BGP.RPKI.ASPA)
			}
		})
	}
}

// TestJSONPeerDownWithoutASClearsTrackedRoutes first admits a real JSON UPDATE,
// then removes its retained routes on a state-down event with no negotiated AS.
func TestJSONPeerDownWithoutASClearsTrackedRoutes(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	previous := rpkiMetricsPtr.Load()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { rpkiMetricsPtr.Store(previous) })
	rp, bridge := aspaRolePlugin(t, "provider")
	rp.cache.Add(makeVRP("10.0.0.0/24", 24, 300))
	rp.aspaCache.Set(200, []uint32{100})
	rp.aspaCache.Set(300, []uint32{200})
	emitted := 0
	bridge.SetEmitEvent(func(_, _, _, _, _ string) (int, error) {
		emitted++
		return 1, nil
	})
	rp.handleEvent(&bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 71},
		Peer: json.RawMessage(`{"name":"ix-192.0.2.50","group":"ix",` +
			`"local":{"as":65000},"remote":{"address":"192.0.2.50","as":100}}`),
		ASPath: []uint32{100, 200, 300},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {{Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}}},
		},
	})
	requests := drainRequests(rp.validateCh)
	if len(requests) != 1 {
		t.Fatalf("known UPDATE produced %d requests, want one", len(requests))
	}
	if rp.originTracker.count() != 1 {
		t.Fatal("known UPDATE did not populate the origin tracker")
	}
	if rp.aspaTracker.count() != 1 {
		t.Fatal("known UPDATE did not populate the ASPA tracker")
	}
	if emitted != 1 {
		t.Fatalf("known UPDATE emitted %d events, want one", emitted)
	}
	rp.handleEvent(&bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindState},
		Peer:    json.RawMessage(`{"remote":{"address":"192.0.2.50"}}`),
		State:   "down",
	})
	if got := rp.originTracker.count(); got != 0 {
		t.Errorf("state-down without AS retained %d origin routes", got)
	}
	if got := rp.aspaTracker.count(); got != 0 {
		t.Errorf("state-down without AS retained %d ASPA routes", got)
	}
	if got := drainRequests(rp.validateCh); len(got) != 0 {
		t.Errorf("state-down enqueued validation requests: %+v", got)
	}
	if emitted != 1 {
		t.Errorf("state-down emitted %d total validation events, want one", emitted)
	}
	if out := scrapeRPKIMetrics(t, reg); strings.Contains(out, "ze_rpki_updates_refused_total{") {
		t.Errorf("state-down was counted as a refused UPDATE:\n%s", out)
	}
}
