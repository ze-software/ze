package filter_path_asn

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// TestPathASNMatchUsesDaemonMetricsRegistry exercises config lowering through
// the compiled condition and the daemon's real Prometheus registry, without a
// plugin process or a hand-installed metrics hook. A fresh test process owns
// the deferred init callbacks: registry.Snapshot does not retain that state.
func TestPathASNMatchUsesDaemonMetricsRegistry(t *testing.T) {
	const child = "path-asn-daemon-metrics-child"
	if os.Args[len(os.Args)-1] != child {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), binary,
			"-test.run=^TestPathASNMatchUsesDaemonMetricsRegistry$", "--", child)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("daemon metrics ownership regression: %v\n%s", err, output)
		}
		return
	}

	chain := peerChain(t, "import")
	session := policyview.Session{EBGP: true, Sender: 65001, SenderKnown: true}
	// A missing exporter must not affect filtering or permanently bind its
	// early no-op children after a real registry arrives.
	registry.SetMetricsRegistry(nil)
	if outcome := judgeLeak(t, chain, session); outcome.Action != policychain.OutcomeReject {
		t.Fatalf("without telemetry: %+v, want reject", outcome)
	}
	reg := metrics.NewPrometheusRegistry()
	registry.SetMetricsRegistry(reg)
	if got := scrapeRejects(t, reg); len(got) != int(flowCount)*int(slotCount) {
		t.Fatalf("daemon exporter has %d reject series, want %d", len(got), int(flowCount)*int(slotCount))
	}
	if got := rejectTotal(t, scrapeRejects(t, reg)); got != 0 {
		t.Fatalf("new daemon exporter counts %d rejects, want zero", got)
	}
	if outcome := judgeLeak(t, chain, session); outcome.Action != policychain.OutcomeReject {
		t.Fatalf("with telemetry: %+v, want reject", outcome)
	}
	const transit = metricRejects + `{direction="import",position="transit",reason="listed-asn"} 1`
	found := false
	for _, line := range scrapeRejects(t, reg) {
		if line == transit {
			found = true
		}
	}
	if !found {
		t.Fatalf("daemon exporter does not expose %s: %v", transit, scrapeRejects(t, reg))
	}

	var view policyview.View
	view.Reset(pathUpdate(t, 65001, 65002), session)
	defer view.Clear()
	if outcome := chain.Run(&view, new(policychain.RunSets)); outcome.Action != policychain.OutcomeAccept {
		t.Fatalf("clean route: %+v, want accept", outcome)
	}
	registry.SetMetricsRegistry(reg)
	if got := rejectTotal(t, scrapeRejects(t, reg)); got != 1 {
		t.Fatalf("clean route or repeated injection changed count to %d, want one", got)
	}
}
