// Design: docs/plugin-development/metrics.md -- metrics of a forked plugin
// Related: dispatch_metrics.go -- the engine side under test
package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// forkedMetricsPlugin connects an SDK plugin carrying publisher to s over the
// JSON transport a forked plugin uses: no DirectBridge, every call framed on
// the connection and served by the engineOps table.
func forkedMetricsPlugin(t *testing.T, s *Server, owner string, publisher *sdk.MetricsPublisher) *sdk.Plugin {
	t.Helper()
	proc := process.NewProcess(plugin.PluginConfig{Name: owner})
	client, engine := net.Pipe()
	p := sdk.NewWithConn(owner, sdk.ConnWithMetrics(client, publisher))
	pc := plugipc.NewPluginConn(engine, engine)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			req, err := pc.ReadRequest(s.ctx)
			if err != nil {
				return
			}
			s.dispatchPluginRPC(proc, pc, req)
		}
	}()
	t.Cleanup(func() {
		assert.NoError(t, p.Close())
		assert.NoError(t, engine.Close())
		<-done
	})
	return p
}

func metricsTestServer(t *testing.T, reg metrics.Registry) *Server {
	t.Helper()
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	s.SetMetricsRegistry(reg)
	return s
}

// exposition returns the sample and TYPE lines of reg whose metric name
// starts with prefix, as a scrape of the daemon's endpoint shows them.
func exposition(t *testing.T, reg *metrics.PrometheusRegistry, prefix string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody))
	var lines []string
	scanner := bufio.NewScanner(rec.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) || strings.HasPrefix(line, "# TYPE "+prefix) {
			lines = append(lines, line)
		}
	}
	require.NoError(t, scanner.Err())
	return lines
}

// exerciseEveryKind drives every Registry method and every handle method a
// plugin can call, with values whose per-bucket means are exact.
func exerciseEveryKind(reg metrics.Registry) {
	events := reg.Counter("ze_fwdtest_events_total", "Events handled.")
	events.Add(3)
	events.Inc()

	depth := reg.Gauge("ze_fwdtest_queue_depth", "Queue depth.")
	depth.Set(7)
	depth.Dec()
	depth.Add(2)

	perPeer := reg.CounterVec("ze_fwdtest_peer_events_total", "Events per peer.", []string{"peer"})
	perPeer.With("a").Add(2)
	perPeer.With("b").Inc()
	perPeer.Delete("b")
	perPeer.With("c")

	sessions := reg.GaugeVec("ze_fwdtest_sessions", "Sessions per state.", []string{"state", "family"})
	sessions.With("up", "ipv4/unicast").Set(4)
	sessions.With("down", "ipv6/unicast").Inc()

	batch := reg.Histogram("ze_fwdtest_batch_routes", "Routes per batch.", []float64{1, 4, 16})
	for _, v := range []float64{0.5, 0.5, 3, 3, 20} {
		batch.Observe(v)
	}

	latency := reg.HistogramVec("ze_fwdtest_peer_latency_seconds", "Latency per peer.", nil, []string{"peer"})
	latency.With("a").Observe(0.25)
	latency.With("a").Observe(0.25)
	latency.With("b").Observe(2)
}

// VALIDATES: a plugin run forked declares and updates metrics of every kind
// the Registry offers, and after it publishes, the daemon's registry shows
// each under the same name with the same values as the same calls made
// in-process against the daemon's registry.
// PREVENTS: the forked plugin registering no counters at all, with no error
// and no log line (ConfigureMetrics reached a registry only in-process).
func TestForkedPluginMetricsMatchInProcess(t *testing.T) {
	inProcess := metrics.NewPrometheusRegistry()
	exerciseEveryKind(inProcess)

	daemon := metrics.NewPrometheusRegistry()
	s := metricsTestServer(t, daemon)
	publisher := sdk.NewMetricsPublisher()
	exerciseEveryKind(publisher)
	p := forkedMetricsPlugin(t, s, "fwdtest", publisher)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, p.FlushMetrics(ctx))

	want := exposition(t, inProcess, "ze_fwdtest_")
	require.NotEmpty(t, want)
	assert.Equal(t, want, exposition(t, daemon, "ze_fwdtest_"))
}

// VALIDATES: a forked plugin that restarts declares its metrics again
// without a second registration or a panic, and its counter carries on from
// the daemon's value; a name declared with another shape is refused and the
// daemon's metric is left as it was.
// PREVENTS: a restarted plugin panicking the daemon (Prometheus panics on a
// second collector), and a plugin turning another metric's kind.
func TestForkedPluginMetricsSurviveARestart(t *testing.T) {
	daemon := metrics.NewPrometheusRegistry()
	s := metricsTestServer(t, daemon)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first := sdk.NewMetricsPublisher()
	first.Counter("ze_fwdtest_restart_total", "Restarts.").Add(3)
	require.NoError(t, forkedMetricsPlugin(t, s, "fwdtest", first).FlushMetrics(ctx))

	second := sdk.NewMetricsPublisher()
	second.Counter("ze_fwdtest_restart_total", "Restarts.").Add(2)
	require.NoError(t, forkedMetricsPlugin(t, s, "fwdtest", second).FlushMetrics(ctx))

	other := sdk.NewMetricsPublisher()
	other.Gauge("ze_fwdtest_restart_total", "Not a counter.").Set(99)
	require.NoError(t, forkedMetricsPlugin(t, s, "fwdother", other).FlushMetrics(ctx))

	assert.Equal(t, []string{
		"# TYPE ze_fwdtest_restart_total counter",
		"ze_fwdtest_restart_total 5",
	}, exposition(t, daemon, "ze_fwdtest_restart_total"))
}

// VALIDATES: a publish that arrives before the daemon has a metrics registry
// is answered unavailable and applies nothing; the plugin keeps the changes,
// and the next publish after the registry exists carries all of them.
// PREVENTS: increments made during startup vanishing on the forked path.
func TestForkedPluginMetricsWaitForTheRegistry(t *testing.T) {
	s := metricsTestServer(t, nil)
	publisher := sdk.NewMetricsPublisher()
	counter := publisher.Counter("ze_fwdtest_early_total", "Early events.")
	counter.Add(4)
	p := forkedMetricsPlugin(t, s, "fwdtest", publisher)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, p.FlushMetrics(ctx))

	daemon := metrics.NewPrometheusRegistry()
	s.SetMetricsRegistry(daemon)
	counter.Inc()
	require.NoError(t, p.FlushMetrics(ctx))

	assert.Equal(t, []string{
		"# TYPE ze_fwdtest_early_total counter",
		"ze_fwdtest_early_total 5",
	}, exposition(t, daemon, "ze_fwdtest_early_total"))
}
