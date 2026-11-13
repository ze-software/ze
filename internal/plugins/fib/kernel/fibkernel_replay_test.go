// Design: docs/guide/plugins.md -- installed-route repair replay.
// Related: fibkernel.go -- processEvent and last successful forwarding state.

package fibkernel

import (
	"errors"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/report"
)

// replayFIBBackend records synchronous processEvent calls. Its constructor
// supplies the required plain backend; its zero value and nil pointers are
// invalid. Nil errors mean success; handled=false requests ordinary replacement.
type replayFIBBackend struct {
	*mockBackend
	richAdded    []RichRoute
	richReplaced []RichRoute
	reasserted   []RichRoute
	handled      bool
	reassertErr  error
	replaceErr   error
}

func newReplayFIBBackend() *replayFIBBackend {
	return &replayFIBBackend{mockBackend: newMockBackend(), handled: true}
}

func (backend *replayFIBBackend) addRichRoute(route RichRoute) error {
	backend.richAdded = append(backend.richAdded, route)
	return nil
}

func (backend *replayFIBBackend) delRichRoute(prefix netip.Prefix, _ uint32) error {
	return backend.delRoute(prefix.String())
}

func (backend *replayFIBBackend) replaceRichRoute(route RichRoute) error {
	backend.richReplaced = append(backend.richReplaced, route)
	return backend.replaceErr
}

func (backend *replayFIBBackend) reassertRichRoute(route RichRoute) (bool, error) {
	backend.reasserted = append(backend.reasserted, route)
	return backend.handled, backend.reassertErr
}

func replayFIBChange() incomingChange {
	return incomingChange{
		Action:    routeaction.Add,
		Prefix:    netip.MustParsePrefix("198.18.85.0/24"),
		NextHop:   netip.MustParseAddr("192.0.2.1"),
		Interface: "eth0", OnLink: true, Weight: 2,
		Protocol: "static", RouteType: sysribevents.RouteTypeUnicast,
		Metric: 100, TableID: 1000, Labels: []uint32{100, 101},
		ECMPPaths: []sysribevents.ECMPPath{{
			NextHop: netip.MustParseAddr("192.0.2.2"), Interface: "eth1",
			OnLink: true, Weight: 3, Labels: []uint32{102, 103},
		}},
		Backup: []sysribevents.ECMPPath{{
			NextHop: netip.MustParseAddr("192.0.2.3"), Interface: "eth2",
			OnLink: true, Weight: 4, Labels: []uint32{104, 105},
		}},
	}
}

func installReplayFIBControl(t *testing.T, change *incomingChange) (*fibKernel, *replayFIBBackend) {
	t.Helper()
	backend := newReplayFIBBackend()
	fib := newFIBKernel(backend)
	fib.processEvent(makeSysribPayload([]incomingChange{*change}))
	if got := fib.installed[change.Prefix.String()]; got != change.NextHop.String() {
		t.Fatalf("useful live control installed next hop %q, want %s", got, change.NextHop)
	}
	if len(backend.reasserted) != 0 {
		t.Fatal("initial live installation used replay reassertion")
	}
	return fib, backend
}

func processFIBReplay(fib *fibKernel, change *incomingChange) {
	batch := makeSysribPayload([]incomingChange{*change})
	batch.ReplayID = replay.Broadcast
	fib.processEvent(batch)
}

// TestFIBKernelReplayReassertsUnchangedForwarding requires a real backend write
// for each eligible installed entry, including plain routes on a rich backend.
func TestFIBKernelReplayReassertsUnchangedForwarding(t *testing.T) {
	for _, name := range []string{"rich", "plain", "different producer"} {
		t.Run(name, func(t *testing.T) {
			change := replayFIBChange()
			if name == "plain" {
				change = incomingChange{Action: routeaction.Add, Prefix: change.Prefix, NextHop: change.NextHop, Protocol: "static"}
			}
			fib, backend := installReplayFIBControl(t, &change)
			if name == "different producer" {
				change.Protocol = "bgp"
			}
			processFIBReplay(fib, &change)
			if len(backend.reasserted) != 1 {
				t.Fatalf("unchanged installed replay attempted %d reassertions, want one actual write", len(backend.reasserted))
			}
			if got := backend.reasserted[0]; !reflect.DeepEqual(got, changeToRichRoute(&change)) {
				t.Errorf("reassertion lost forwarding metadata: got %+v", got)
			}
			if len(backend.richReplaced) != 0 {
				t.Errorf("eligible replay performed %d ordinary rich replacements", len(backend.richReplaced))
			}
			if len(backend.replaced) != 0 {
				t.Errorf("eligible replay performed %d ordinary plain replacements", len(backend.replaced))
			}
			if len(fib.pending) != 0 {
				t.Errorf("successful reassertion retained pending failures: %v", fib.pending)
			}
		})
	}
}

// TestFIBKernelReplayChangedMetadataUsesFullReplacement pins every forwarding
// field, including each member's metadata, to the last successful description.
func TestFIBKernelReplayChangedMetadataUsesFullReplacement(t *testing.T) {
	cases := []struct {
		name   string
		change func(*incomingChange)
	}{
		{"next hop", func(c *incomingChange) { c.NextHop = netip.MustParseAddr("192.0.2.9") }},
		{"interface", func(c *incomingChange) { c.Interface = "eth9" }},
		{"on link", func(c *incomingChange) { c.OnLink = false }},
		{"weight", func(c *incomingChange) { c.Weight++ }},
		{"route type", func(c *incomingChange) { c.RouteType = sysribevents.RouteTypeUnreachable }},
		{"metric", func(c *incomingChange) { c.Metric++ }},
		{"table", func(c *incomingChange) { c.TableID++ }},
		{"label value", func(c *incomingChange) { c.Labels[0]++ }},
		{"label count", func(c *incomingChange) { c.Labels = c.Labels[:1] }},
		{"SRv6 SID", func(c *incomingChange) { c.SRv6SID = netip.MustParseAddr("2001:db8::9") }},
		{"ECMP next hop", func(c *incomingChange) { c.ECMPPaths[0].NextHop = netip.MustParseAddr("192.0.2.9") }},
		{"ECMP interface", func(c *incomingChange) { c.ECMPPaths[0].Interface = "eth9" }},
		{"ECMP on link", func(c *incomingChange) { c.ECMPPaths[0].OnLink = false }},
		{"ECMP weight", func(c *incomingChange) { c.ECMPPaths[0].Weight++ }},
		{"ECMP label value", func(c *incomingChange) { c.ECMPPaths[0].Labels[0]++ }},
		{"ECMP label count", func(c *incomingChange) { c.ECMPPaths[0].Labels = c.ECMPPaths[0].Labels[:1] }},
		{"ECMP count", func(c *incomingChange) { c.ECMPPaths = nil }},
		{"backup next hop", func(c *incomingChange) { c.Backup[0].NextHop = netip.MustParseAddr("192.0.2.9") }},
		{"backup interface", func(c *incomingChange) { c.Backup[0].Interface = "eth9" }},
		{"backup on link", func(c *incomingChange) { c.Backup[0].OnLink = false }},
		{"backup weight", func(c *incomingChange) { c.Backup[0].Weight++ }},
		{"backup label value", func(c *incomingChange) { c.Backup[0].Labels[0]++ }},
		{"backup label count", func(c *incomingChange) { c.Backup[0].Labels = c.Backup[0].Labels[:1] }},
		{"backup count", func(c *incomingChange) { c.Backup = nil }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			change := replayFIBChange()
			fib, backend := installReplayFIBControl(t, &change)
			test.change(&change)
			processFIBReplay(fib, &change)
			if len(backend.reasserted) != 0 {
				t.Errorf("changed forwarding used %d unchanged-route reassertions", len(backend.reasserted))
			}
			if len(backend.richReplaced) != 1 {
				t.Fatalf("changed replay performed %d full replacements, want one", len(backend.richReplaced))
			}
			if got := backend.richReplaced[0]; !reflect.DeepEqual(got, changeToRichRoute(&change)) {
				t.Errorf("full replacement lost changed forwarding metadata: got %+v", got)
			}
		})
	}
}

func TestFIBKernelReplayUncertainStateUsesFullProgramming(t *testing.T) {
	for _, name := range []string{"not installed", "retained", "pending", "no forwarding record"} {
		t.Run(name, func(t *testing.T) {
			change := replayFIBChange()
			fib, backend := installReplayFIBControl(t, &change)
			prefix := change.Prefix.String()
			switch name {
			case "not installed":
				delete(fib.installed, prefix)
			case "retained":
				fib.retained[prefix] = true
			case "pending":
				fib.pending[prefix] = time.Now()
			case "no forwarding record":
				delete(fib.forwarding, prefix)
			}
			processFIBReplay(fib, &change)
			if len(backend.reasserted) != 0 {
				t.Errorf("uncertain route used %d unchanged-route reassertions", len(backend.reasserted))
			}
			if name == "not installed" {
				if len(backend.richAdded) != 2 {
					t.Errorf("unknown route performed %d rich adds, want initial control and exclusive replay add", len(backend.richAdded))
				}
				return
			}
			if len(backend.richReplaced) != 1 {
				t.Errorf("uncertain route performed %d full replacements, want one", len(backend.richReplaced))
			}
		})
	}
}

func TestFIBKernelLiveChangesNeverUseReplayReassertion(t *testing.T) {
	for _, action := range []routeaction.Action{routeaction.Add, routeaction.Update} {
		t.Run(action.String(), func(t *testing.T) {
			change := replayFIBChange()
			fib, backend := installReplayFIBControl(t, &change)
			change.Action = action
			fib.processEvent(makeSysribPayload([]incomingChange{change}))
			if len(backend.reasserted) != 0 {
				t.Errorf("live %s used %d replay reassertions", action, len(backend.reasserted))
			}
			if len(backend.richReplaced) != 1 {
				t.Errorf("live %s performed %d full replacements, want one", action, len(backend.richReplaced))
			}
		})
	}
}

func TestFIBKernelReplayUnhandledUsesFullReplacement(t *testing.T) {
	change := replayFIBChange()
	fib, backend := installReplayFIBControl(t, &change)
	backend.handled = false
	processFIBReplay(fib, &change)
	if len(backend.reasserted) != 1 {
		t.Errorf("eligible replay made %d optional backend attempts, want one", len(backend.reasserted))
	}
	if len(backend.richReplaced) != 1 {
		t.Errorf("unhandled replay performed %d full replacements, want one", len(backend.richReplaced))
	}
}

func TestFIBKernelReplayUnhandledErrorDoesNotReplace(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	change := replayFIBChange()
	fib, backend := installReplayFIBControl(t, &change)
	backend.handled = false
	backend.reassertErr = errors.New("fresh ownership probe failed")
	processFIBReplay(fib, &change)
	if len(backend.reasserted) != 1 {
		t.Errorf("failed replay made %d optional attempts, want one", len(backend.reasserted))
	}
	if len(backend.richReplaced) != 0 {
		t.Errorf("backend error fell through to %d full replacements", len(backend.richReplaced))
	}
	prefix := change.Prefix.String()
	if _, pending := fib.pending[prefix]; !pending {
		t.Error("unhandled backend error did not record pending failure")
	}
	if got := fib.forwarding[prefix]; !reflect.DeepEqual(got, change) {
		t.Errorf("unhandled backend error changed last successful forwarding: %+v", got)
	}
}

func TestFIBKernelReplayReplaceKeepsFullProgramming(t *testing.T) {
	change := replayFIBChange()
	fib, backend := installReplayFIBControl(t, &change)
	change.Action = routeaction.Update
	processFIBReplay(fib, &change)
	if len(backend.reasserted) != 0 {
		t.Errorf("replayed Update used %d unchanged-Add reassertions", len(backend.reasserted))
	}
	if len(backend.richReplaced) != 1 {
		t.Errorf("replayed Update performed %d full replacements, want one", len(backend.richReplaced))
	}
}

func TestFIBKernelReplayFailedReplacementNeedsFullRepair(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	change := replayFIBChange()
	fib, backend := installReplayFIBControl(t, &change)
	failed := cloneForwardingChange(&change)
	failed.Action = routeaction.Update
	failed.NextHop = netip.MustParseAddr("192.0.2.9")
	backend.replaceErr = errors.New("replacement outcome uncertain")
	fib.processEvent(makeSysribPayload([]incomingChange{failed}))
	prefix := change.Prefix.String()
	if _, pending := fib.pending[prefix]; !pending {
		t.Fatal("failed live replacement did not establish pending control")
	}
	if got := fib.forwarding[prefix]; !reflect.DeepEqual(got, change) {
		t.Fatalf("failed live replacement changed last successful forwarding: %+v", got)
	}
	backend.replaceErr = nil
	processFIBReplay(fib, &change)
	if len(backend.reasserted) != 0 {
		t.Errorf("uncertain write used %d unchanged-route reassertions", len(backend.reasserted))
	}
	if len(backend.richReplaced) != 2 {
		t.Errorf("failed replacement and repairing replay performed %d full writes, want two", len(backend.richReplaced))
	}
	if _, pending := fib.pending[prefix]; pending {
		t.Error("successful full repair retained the pending failure")
	}
}

// TestFIBKernelReplayRefusalPreservesSuccessfulForwarding checks the existing
// install-failure path, including its pending state, operator report and metric.
func TestFIBKernelReplayRefusalPreservesSuccessfulForwarding(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	previousMetrics := fibMetricsPtr.Load()
	t.Cleanup(func() { fibMetricsPtr.Store(previousMetrics) })
	registry := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(registry)
	change := replayFIBChange()
	fib, backend := installReplayFIBControl(t, &change)
	prefix := change.Prefix.String()
	lastSuccessful := cloneForwardingChange(&change)
	change.Protocol = "bgp"
	backend.reassertErr = errors.New("fresh ownership reassertion refused")
	processFIBReplay(fib, &change)
	if len(backend.reasserted) != 1 {
		t.Errorf("refused replay made %d reassertion attempts, want one", len(backend.reasserted))
	}
	if len(backend.richReplaced) != 0 {
		t.Errorf("handled refusal fell through to %d ordinary replacements", len(backend.richReplaced))
	}
	if got := fib.forwarding[prefix]; !reflect.DeepEqual(got, lastSuccessful) {
		t.Errorf("failed replay replaced last successful forwarding: got %+v", got)
	}
	if got := fib.installed[prefix]; got != lastSuccessful.NextHop.String() {
		t.Errorf("failed replay lost installed ownership: got next hop %q", got)
	}
	firstFailure, pending := fib.pending[prefix]
	if !pending {
		t.Error("handled replay refusal did not record a pending failure")
	}
	found := false
	for _, failure := range report.Errors(0) {
		if failure.Code != reportCodeFIBSyncFailure {
			continue
		}
		if failure.Subject != prefix {
			continue
		}
		found = true
		if got := failure.Detail[keyOperation]; got != "add" {
			t.Errorf("replay refusal reported operation %v, want add", got)
		}
	}
	if !found {
		t.Error("handled replay refusal did not raise fib-sync-failure")
	}
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), "ze_fibkernel_errors_total{operation=\"add\"} 1\n") {
		t.Error("handled replay refusal did not increment the add failure metric exactly once")
	}
	if !pending {
		return
	}
	// A pending failure makes even the former successful description uncertain.
	backend.replaceErr = errors.New("ordinary repair refused")
	processFIBReplay(fib, &change)
	if len(backend.reasserted) != 1 {
		t.Errorf("pending replay used the optional path again: %d attempts", len(backend.reasserted))
	}
	if len(backend.richReplaced) != 1 {
		t.Errorf("pending replay performed %d full repairs, want one", len(backend.richReplaced))
	}
	if got := fib.pending[prefix]; !got.Equal(firstFailure) {
		t.Errorf("repeated repair failure changed first pending timestamp from %v to %v", firstFailure, got)
	}
}

func TestFIBKernelReplayWithoutOptionalBackendStillRepairs(t *testing.T) {
	backend := newMockBackend()
	fib := newFIBKernel(backend)
	change := incomingChange{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.86.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"), Protocol: "static",
	}
	fib.processEvent(makeSysribPayload([]incomingChange{change}))
	processFIBReplay(fib, &change)
	if got := backend.replaced[change.Prefix.String()]; got != change.NextHop.String() {
		t.Errorf("ordinary repair reached next hop %q, want %s", got, change.NextHop)
	}
}
