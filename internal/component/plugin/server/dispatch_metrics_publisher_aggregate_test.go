// Design: docs/plugin-development/metrics.md -- finite forked metric aggregates.
// Related: dispatch_metrics_publisher_regression_test.go -- real receiver helpers.
package server

import (
	"context"
	"math"
	"net"
	"sync"
	"testing"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestMetricsPublisherFiniteAggregateAdmission drives finite inputs through
// the actual receiver. A finite MaxFloat64 aggregate is accepted; an operation
// that would make its pending representation infinite is refused atomically,
// without poisoning either another label or an unrelated metric. The expected
// in-process registry receives exactly the representable, admitted operations:
// native Prometheus itself permits Inf, which this RPC cannot encode.
func TestMetricsPublisherFiniteAggregateAdmission(t *testing.T) {
	for index, kind := range [4]rpc.MetricKind{rpc.MetricCounterVec, rpc.MetricGaugeVec, rpc.MetricHistogramVec, rpc.MetricHistogramVec} {
		sign, label := 1.0, string(kind)
		if index == 3 {
			sign, label = -1, label+"/negative"
		}
		t.Run(label, func(t *testing.T) {
			const name = "ze_publisher_aggregate_events"
			daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
			publisher := sdk.NewMetricsPublisher()
			v := publisherRegressionVector(publisher, kind, name)
			expected := publisherRegressionVector(want, kind, name)
			overflow, expectedOverflow := v.with("overflow"), expected.with("overflow")
			overflow(sign * math.MaxFloat64 / 2)
			expectedOverflow(sign * math.MaxFloat64 / 2)
			overflow(sign * math.MaxFloat64 / 2)
			expectedOverflow(sign * math.MaxFloat64 / 2)
			overflow(sign * math.MaxFloat64 / 2) // The aggregate, not this finite input, cannot travel.
			v.with("healthy")(2)
			expected.with("healthy")(2)
			neighbor := publisher.Counter("ze_publisher_aggregate_neighbor", "Healthy neighbor.")
			expectedNeighbor := want.Counter("ze_publisher_aggregate_neighbor", "Healthy neighbor.")
			neighbor.Inc()
			expectedNeighbor.Inc()
			p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-aggregate", publisher)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("finite inputs produced an unpublishable aggregate: %v", err)
			}
			assertPublisherExposition(t, want, daemon, "ze_publisher_aggregate_")

			// A successful acknowledgement releases the pending reservation.
			// Counter/histogram deltas can again reach MaxFloat64 even when the
			// daemon's lifetime total becomes Inf, just as it does in-process.
			if kind == rpc.MetricGaugeVec {
				publisher.GaugeVec(name, "Publisher regression events.", []string{"peer"}).With("overflow").Set(7)
				want.GaugeVec(name, "Publisher regression events.", []string{"peer"}).With("overflow").Set(7)
			} else {
				overflow(sign * math.MaxFloat64)
				expectedOverflow(sign * math.MaxFloat64)
			}
			v.with("healthy")(3)
			expected.with("healthy")(3)
			neighbor.Inc()
			expectedNeighbor.Inc()
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("healthy updates stopped after aggregate refusal: %v", err)
			}
			assertPublisherExposition(t, want, daemon, "ze_publisher_aggregate_")
		})
	}
}

// TestMetricsPublisherFailedRestoreStaysFinite pauses a finite snapshot in
// flight. A concurrent finite addition is admitted, while another whose sum
// with that still-unacknowledged snapshot would overflow is refused. After an
// actual unavailable answer or an RPC error, restoration must preserve every
// admitted value and histogram observation and let healthy neighbors advance.
func TestMetricsPublisherFailedRestoreStaysFinite(t *testing.T) {
	for index, kind := range [4]rpc.MetricKind{rpc.MetricCounterVec, rpc.MetricGaugeVec, rpc.MetricHistogramVec, rpc.MetricHistogramVec} {
		sign, label := 1.0, string(kind)
		if index == 3 {
			sign, label = -1, label+"/negative"
		}
		for _, failure := range []string{"unavailable", "error"} {
			t.Run(label+"/"+failure, func(t *testing.T) {
				const name = "ze_publisher_restore_aggregate_events"
				daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
				s := metricsTestServer(t, nil)
				publisher := sdk.NewMetricsPublisher()
				v := publisherRegressionVector(publisher, kind, name)
				expected := publisherRegressionVector(want, kind, name)
				overflow, expectedOverflow := v.with("overflow"), expected.with("overflow")
				overflow(sign * math.MaxFloat64 / 2)
				expectedOverflow(sign * math.MaxFloat64 / 2)
				healthy, expectedHealthy := v.with("healthy"), expected.with("healthy")
				healthy(1)
				expectedHealthy(1)
				neighbor := publisher.Counter("ze_publisher_restore_aggregate_neighbor", "Healthy neighbor.")
				expectedNeighbor := want.Counter("ze_publisher_restore_aggregate_neighbor", "Healthy neighbor.")
				neighbor.Inc()
				expectedNeighbor.Inc()
				client, engine := net.Pipe()
				p := sdk.NewWithConn("publisher-aggregate-restore", sdk.ConnWithMetrics(client, publisher))
				pc := plugipc.NewPluginConn(engine, engine)
				proc := process.NewProcess(plugin.PluginConfig{Name: "publisher-aggregate-restore"})
				entered, resume, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
				release := sync.OnceFunc(func() { close(resume) })
				go func() {
					defer close(stopped)
					first := true
					for {
						req, err := pc.ReadRequest(s.ctx)
						if err != nil {
							return
						}
						if first {
							first = false
							close(entered)
							<-resume
							if failure == "error" {
								if err := pc.SendError(context.Background(), req.ID, "metrics receiver unavailable"); err != nil {
									t.Error(err)
									return
								}
								continue
							}
						}
						s.dispatchPluginRPC(proc, pc, req)
					}
				}()
				t.Cleanup(func() {
					release()
					if err := p.Close(); err != nil {
						t.Error(err)
					}
					if err := engine.Close(); err != nil {
						t.Error(err)
					}
					<-stopped
				})
				firstDone := make(chan error, 1)
				go func() { firstDone <- p.FlushMetrics(t.Context()) }()
				select {
				case <-entered:
				case err := <-firstDone:
					t.Fatalf("finite snapshot did not reach the receiver: %v", err)
				case <-t.Context().Done():
					t.Fatal(t.Context().Err())
				}
				overflow(sign * math.MaxFloat64 / 4)
				expectedOverflow(sign * math.MaxFloat64 / 4)
				overflow(sign * math.MaxFloat64 / 2) // Pending alone fits, but failed restoration would not.
				healthy(2)
				expectedHealthy(2)
				neighbor.Add(2)
				expectedNeighbor.Add(2)
				release()
				if err := <-firstDone; (err != nil) != (failure == "error") {
					t.Fatalf("unexpected failed-publish answer: %v", err)
				}
				s.SetMetricsRegistry(daemon)
				// Restoration releases its reservation before a later update.
				// Four observations have an exact per-bucket replay mean.
				for range 2 {
					overflow(sign * math.MaxFloat64 / 8)
					expectedOverflow(sign * math.MaxFloat64 / 8)
				}
				if err := p.FlushMetrics(t.Context()); err != nil {
					t.Fatalf("failed restoration poisoned a valid publication: %v", err)
				}
				assertPublisherExposition(t, want, daemon, "ze_publisher_restore_aggregate_")
				// Successful recovery must release the replacement reservation
				// too, so a new finite delta can use the whole wire range.
				if kind == rpc.MetricGaugeVec {
					publisher.GaugeVec(name, "Publisher regression events.", []string{"peer"}).With("overflow").Set(7)
					want.GaugeVec(name, "Publisher regression events.", []string{"peer"}).With("overflow").Set(7)
				} else {
					overflow(sign * math.MaxFloat64)
					expectedOverflow(sign * math.MaxFloat64)
				}
				healthy(3)
				expectedHealthy(3)
				neighbor.Inc()
				expectedNeighbor.Inc()
				if err := p.FlushMetrics(t.Context()); err != nil {
					t.Fatalf("healthy updates stopped after failed restoration: %v", err)
				}
				assertPublisherExposition(t, want, daemon, "ze_publisher_restore_aggregate_")
			})
		}
	}
}
