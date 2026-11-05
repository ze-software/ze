// Design: docs/plugin-development/metrics.md -- bounded forked metric publication.
// Related: dispatch_metrics.go -- validated daemon receiver.
package server

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// publisherVector gives each vector kind the same lifecycle exercise without
// replacing the registry or the receiver with a test implementation.
type publisherVector struct {
	with   func(string) func(float64)
	delete func(...string) bool
}

func publisherRegressionVector(reg metrics.Registry, kind rpc.MetricKind, name string) publisherVector {
	switch kind {
	case rpc.MetricCounterVec:
		v := reg.CounterVec(name, "Publisher regression events.", []string{"peer"})
		return publisherVector{with: func(label string) func(float64) { return v.With(label).Add }, delete: v.Delete}
	case rpc.MetricGaugeVec:
		v := reg.GaugeVec(name, "Publisher regression events.", []string{"peer"})
		return publisherVector{with: func(label string) func(float64) { return v.With(label).Add }, delete: v.Delete}
	case rpc.MetricHistogramVec:
		v := reg.HistogramVec(name, "Publisher regression events.", []float64{1, 10, 100}, []string{"peer"})
		return publisherVector{with: func(label string) func(float64) { return v.With(label).Observe }, delete: v.Delete}
	case rpc.MetricCounter, rpc.MetricGauge, rpc.MetricHistogram:
		panic("BUG: publisher regression needs a vector kind")
	}
	panic("BUG: publisher regression received an unknown kind")
}

var publisherRegressionKinds = []rpc.MetricKind{
	rpc.MetricCounterVec, rpc.MetricGaugeVec, rpc.MetricHistogramVec,
}

// TestMetricsPublisherLabelAdmission reaches the real JSON receiver with a
// 256-octet label and a refused 257-octet label, then proves unrelated updates
// continue to advance instead of repeatedly restoring an invalid whole batch.
func TestMetricsPublisherLabelAdmission(t *testing.T) {
	for _, kind := range publisherRegressionKinds {
		t.Run(string(kind), func(t *testing.T) {
			const name = "ze_publisher_label_events"
			daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
			publisher := sdk.NewMetricsPublisher()
			v := publisherRegressionVector(publisher, kind, name)
			expected := publisherRegressionVector(want, kind, name)
			p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-labels", publisher)
			boundary := strings.Repeat("é", rpc.MetricLabelValueMax/2)
			invalid := boundary + "x"
			v.with(boundary)(3)
			expected.with(boundary)(3)
			bad := v.with(invalid)
			bad(99)
			v.with("valid")(2)
			expected.with("valid")(2)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("invalid label poisoned valid metrics at the receiver: %v", err)
			}
			if v.delete(invalid) {
				t.Error("an oversized label was admitted as a live series")
			}
			bad(100)
			v.with("valid")(5)
			expected.with("valid")(5)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("valid metrics stopped advancing after invalid label refusal: %v", err)
			}
			assertPublisherExposition(t, want, daemon, name)
		})
	}
}

// TestMetricsPublisherChurnStaysPublishable performs more deletions than one
// real publish can carry, both on one label and on never-published unique
// labels. It compares the surviving values with an in-process registry.
func TestMetricsPublisherChurnStaysPublishable(t *testing.T) {
	for _, kind := range publisherRegressionKinds {
		t.Run(string(kind), func(t *testing.T) {
			const name = "ze_publisher_churn_events"
			daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
			publisher := sdk.NewMetricsPublisher()
			v := publisherRegressionVector(publisher, kind, name)
			expected := publisherRegressionVector(want, kind, name)
			p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-churn", publisher)
			v.with("again")(10)
			expected.with("again")(10)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatal(err)
			}
			for i := range rpc.MetricsPublishMax + 1 {
				for _, label := range []string{"again", strconv.Itoa(i)} {
					old, expectedOld := v.with(label), expected.with(label)
					old(2)
					expectedOld(2)
					if !v.delete(label) || !expected.delete(label) {
						t.Fatalf("deleting a live series failed at churn %d", i)
					}
					old(99)
					expectedOld(99)
				}
			}
			// Registering the family again must preserve its pending deletion.
			v = publisherRegressionVector(publisher, kind, name)
			v.with("again")(4)
			expected.with("again")(4)
			v.with("unrelated")(7)
			expected.with("unrelated")(7)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("bounded live-series churn produced an unpublishable batch: %v", err)
			}
			v.with("unrelated")(3)
			expected.with("unrelated")(3)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("publisher did not recover after churn: %v", err)
			}
			assertPublisherExposition(t, want, daemon, name)
		})
	}
}

func assertPublisherExposition(t *testing.T, want, daemon *metrics.PrometheusRegistry, prefix string) {
	t.Helper()
	expected, got := exposition(t, want, prefix), exposition(t, daemon, prefix)
	if strings.Join(expected, "\n") != strings.Join(got, "\n") {
		t.Fatalf("published metrics differ from in-process metrics:\nwant %v\ngot  %v", expected, got)
	}
}

// TestMetricsPublisherRestorePreservesRecreation pauses a real RPC before its
// receiver applies it, answers unavailable, or fails. It recreates in-flight
// series and compares recovery, including pending declarations, in-process.
func TestMetricsPublisherRestorePreservesRecreation(t *testing.T) {
	for _, kind := range publisherRegressionKinds {
		for _, failure := range []string{"unavailable", "error", "initial-unavailable", "applied"} {
			t.Run(string(kind)+"/"+failure, func(t *testing.T) {
				const name = "ze_publisher_restore_events"
				daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
				s := metricsTestServer(t, daemon)
				publisher := sdk.NewMetricsPublisher()
				v := publisherRegressionVector(publisher, kind, name)
				expected := publisherRegressionVector(want, kind, name)
				client, engine := net.Pipe()
				p := sdk.NewWithConn("publisher-restore", sdk.ConnWithMetrics(client, publisher))
				pc := plugipc.NewPluginConn(engine, engine)
				proc := process.NewProcess(plugin.PluginConfig{Name: "publisher-restore"})
				t.Cleanup(func() {
					if err := p.Close(); err != nil {
						t.Error(err)
					}
					if err := engine.Close(); err != nil {
						t.Error(err)
					}
				})
				flush := func(beforeAnswer func(), reject bool) error {
					done := make(chan error, 1)
					go func() { done <- p.FlushMetrics(t.Context()) }()
					req, err := pc.ReadRequest(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					var in rpc.MetricsPublishInput
					if err := json.Unmarshal(req.Params, &in); err != nil {
						t.Fatal(err)
					}
					if err := rpc.ValidateMetricsPublish(&in); err != nil {
						t.Fatalf("publisher sent an invalid request: %v", err)
					}
					if beforeAnswer != nil {
						beforeAnswer()
					}
					if reject {
						if err := pc.SendError(context.Background(), req.ID, "metrics receiver unavailable"); err != nil {
							t.Fatal(err)
						}
					} else {
						s.dispatchPluginRPC(proc, pc, req)
					}
					return <-done
				}
				v.with("again")(10)
				expected.with("again")(10)
				keep, expectedKeep := v.with("keep"), expected.with("keep")
				keep(2)
				expectedKeep(2)
				if failure != "initial-unavailable" {
					if err := flush(nil, false); err != nil {
						t.Fatal(err)
					}
				}
				v.delete("again")
				expected.delete("again")
				old, expectedOld := v.with("again"), expected.with("again")
				old(20)
				expectedOld(20)
				keep(3)
				expectedKeep(3)
				if failure != "applied" {
					s.SetMetricsRegistry(nil)
				}
				err := flush(func() {
					old(7)
					expectedOld(7)
					v.delete("again")
					expected.delete("again")
					v = publisherRegressionVector(publisher, kind, name)
					v.with("again")(4)
					expected.with("again")(4)
					old(99)
					expectedOld(99)
					keep(5)
					expectedKeep(5)
				}, failure == "error")
				if (err != nil) != (failure == "error") {
					t.Fatalf("unexpected failed-publish answer: %v", err)
				}
				s.SetMetricsRegistry(daemon)
				if err := flush(nil, false); err != nil {
					t.Fatalf("restored publish failed: %v", err)
				}
				assertPublisherExposition(t, want, daemon, name)
			})
		}
	}
}

// TestMetricsPublisherFullCapacityReplacement proves a batch remains accepted
// with every name declared and every labeled-series slot replaced before its
// flush. Receiver limits, not a private queue representation, are the oracle.
func TestMetricsPublisherFullCapacityReplacement(t *testing.T) {
	const name = "ze_publisher_capacity_events"
	daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
	publisher := sdk.NewMetricsPublisher()
	v := publisher.CounterVec(name, "Capacity events.", []string{"peer"})
	expected := want.CounterVec(name, "Capacity events.", []string{"peer"})
	for i := range rpc.MetricsDeclaredMax - 1 {
		scalar := "ze_publisher_capacity_scalar_" + strconv.Itoa(i)
		publisher.Counter(scalar, "Scalar events.").Inc()
		want.Counter(scalar, "Scalar events.").Inc()
	}
	for i := range rpc.MetricsSeriesMax {
		label := strconv.Itoa(i)
		v.With(label).Add(2)
		expected.With(label).Add(2)
	}
	p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-capacity", publisher)
	if err := p.FlushMetrics(t.Context()); err != nil {
		t.Fatalf("full-capacity initial publish failed: %v", err)
	}
	for i := range rpc.MetricsSeriesMax {
		label := strconv.Itoa(i)
		if !v.Delete(label) {
			t.Fatalf("full-capacity deletion failed for %s", label)
		}
		expected.Delete(label)
	}
	for i := range rpc.MetricsSeriesMax {
		label := "new-" + strconv.Itoa(i)
		v.With(label).Add(3)
		expected.With(label).Add(3)
	}
	// Neither declaration nor series admission may leak beyond the receiver's
	// actual limits or poison the valid replacement batch.
	publisher.Counter("ze_publisher_capacity_overflow", "Refused scalar.").Inc()
	v.With("overflow").Inc()
	if err := p.FlushMetrics(t.Context()); err != nil {
		t.Fatalf("full-capacity replacement publish failed: %v", err)
	}
	assertPublisherExposition(t, want, daemon, "ze_publisher_capacity_")
}

// TestMetricsPublisherEncodedBatchBoundary sends valid labels whose JSON
// escaping makes their combined request exceed the real frame bound. It
// proves finite split publication, delete-before-replacement ordering, and
// recovery after a refused tail without replaying an acknowledged prefix.
func TestMetricsPublisherEncodedBatchBoundary(t *testing.T) {
	for _, failTail := range []bool{false, true} {
		t.Run(strconv.FormatBool(failTail), func(t *testing.T) {
			const name = "ze_publisher_encoded_events"
			const series = rpc.MetricsSeriesMax / 4
			daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
			s := metricsTestServer(t, daemon)
			publisher := sdk.NewMetricsPublisher()
			labels := make([]string, rpc.MetricLabelNamesMax)
			for i := range labels {
				labels[i] = "label_" + strconv.Itoa(i)
			}
			v := publisher.CounterVec(name, "Encoded-boundary events.", labels)
			expected := want.CounterVec(name, "Encoded-boundary events.", labels)
			values := make([][]string, series)
			var fixture rpc.MetricsPublishInput
			for i := range values {
				values[i] = make([]string, len(labels))
				for j := range labels {
					values[i][j] = strings.Repeat("<", rpc.MetricLabelValueMax)
				}
				id := strconv.Itoa(i)
				values[i][0] = id + values[i][0][len(id):]
				v.With(values[i]...).Add(2)
				expected.With(values[i]...).Add(2)
				fixture.Update = append(fixture.Update, rpc.MetricUpdate{Name: name, Values: values[i], Value: 2})
			}
			encoded, err := json.Marshal(&fixture)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) <= rpc.MaxMessageSize {
				t.Fatal("fixture does not exceed the actual encoded request limit")
			}
			if err := rpc.ValidateMetricsPublish(&fixture); err != nil {
				t.Fatalf("fixture does not meet structural receiver limits: %v", err)
			}
			publisher.Counter("ze_publisher_encoded_unrelated", "Unrelated events.").Add(7)
			want.Counter("ze_publisher_encoded_unrelated", "Unrelated events.").Add(7)

			client, engine := net.Pipe()
			p := sdk.NewWithConn("publisher-encoded", sdk.ConnWithMetrics(client, publisher))
			pc := plugipc.NewPluginConn(engine, engine)
			proc := process.NewProcess(plugin.PluginConfig{Name: "publisher-encoded"})
			done := make(chan struct{})
			go func() {
				defer close(done)
				appliedUpdates, rejected := false, false
				for {
					req, err := pc.ReadRequest(s.ctx)
					if err != nil {
						return
					}
					var in rpc.MetricsPublishInput
					if err := json.Unmarshal(req.Params, &in); err != nil {
						t.Error(err)
						return
					}
					if failTail && appliedUpdates && !rejected && len(in.Update) > 0 {
						rejected = true
						if err := pc.SendError(s.ctx, req.ID, "metrics tail unavailable"); err != nil {
							t.Error(err)
							return
						}
						continue
					}
					s.dispatchPluginRPC(proc, pc, req)
					if len(in.Update) > 0 {
						appliedUpdates = true
					}
				}
			}()
			t.Cleanup(func() {
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
				<-done
			})
			err = p.FlushMetrics(t.Context())
			if failTail {
				if err == nil {
					t.Fatal("a refused tail was not reported")
				}
				prefix := len(exposition(t, daemon, name)) - 1
				if prefix < 1 || prefix >= series {
					t.Fatalf("failed tail did not leave an acknowledged proper prefix: %d series", prefix)
				}
				if err := p.FlushMetrics(t.Context()); err != nil {
					t.Fatalf("unsent or refused tail did not recover: %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid encoded-wide metrics poisoned publication: %v", err)
			}
			assertPublisherExposition(t, want, daemon, "ze_publisher_encoded_")
			for i := range values {
				old, expectedOld := v.With(values[i]...), expected.With(values[i]...)
				if !v.Delete(values[i]...) {
					t.Fatalf("deletion failed for encoded-wide series %d", i)
				}
				expected.Delete(values[i]...)
				old.Add(99)
				expectedOld.Add(99)
				v.With(values[i]...).Add(3)
				expected.With(values[i]...).Add(3)
			}
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("encoded-wide replacement did not publish: %v", err)
			}
			assertPublisherExposition(t, want, daemon, "ze_publisher_encoded_")
		})
	}
}

// TestMetricsPublisherDeclarationRepresentability refuses only a declaration
// that cannot fit even alone. A singleton beyond the widest-ID fit proof but
// within its actual first frame remains accepted; neither blocks other metrics.
func TestMetricsPublisherDeclarationRepresentability(t *testing.T) {
	for _, representable := range []bool{false, true} {
		t.Run(strconv.FormatBool(representable), func(t *testing.T) {
			const name = "ze_publisher_declaration_events"
			decl := rpc.MetricDeclaration{
				Name: name, Help: "Declaration boundary.", Kind: rpc.MetricCounterVec,
				Labels: []string{"a"},
			}
			in := rpc.MetricsPublishInput{Declare: []rpc.MetricDeclaration{decl}}
			base, err := json.Marshal(&in)
			if err != nil {
				t.Fatal(err)
			}
			target := rpc.MaxMessageSize + 1
			if representable {
				prefix := len(rpc.AppendRequest(nil, math.MaxUint64, rpc.MethodMetricsPublish, json.RawMessage("0"))) - 1
				target = rpc.MaxMessageSize - prefix + 1
			}
			decl.Labels[0] = strings.Repeat("a", target-len(base)+1)
			in.Declare[0] = decl
			encoded, err := json.Marshal(&in)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != target {
				t.Fatalf("singleton fixture encodes to %d octets, want %d", len(encoded), target)
			}
			if err := rpc.ValidateMetricsPublish(&in); err != nil {
				t.Fatalf("declaration is structurally invalid: %v", err)
			}
			if representable {
				if len(rpc.AppendRequest(nil, 1, rpc.MethodMetricsPublish, encoded)) > rpc.MaxMessageSize {
					t.Fatal("near-boundary declaration does not fit its actual first frame")
				}
			}
			daemon := metrics.NewPrometheusRegistry()
			publisher := sdk.NewMetricsPublisher()
			v := publisher.CounterVec(name, decl.Help, decl.Labels)
			if !representable {
				v.With("detached").Inc()
			}
			unrelated := publisher.Counter("ze_publisher_declaration_unrelated", "Unrelated events.")
			unrelated.Add(7)
			p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-declaration", publisher)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("singleton declaration poisoned unrelated publication: %v", err)
			}
			shape, exists := daemon.Shape(name)
			if exists != representable {
				t.Fatalf("daemon declaration exists=%t, want %t", exists, representable)
			}
			if representable {
				if len(shape.LabelNames) != 1 {
					t.Fatal("accepted declaration lost its label name")
				}
				if shape.LabelNames[0] != decl.Labels[0] {
					t.Fatal("accepted declaration changed its label name")
				}
			}
			unrelated.Add(3)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("unrelated metric stopped advancing: %v", err)
			}
			want := metrics.NewPrometheusRegistry()
			want.Counter("ze_publisher_declaration_unrelated", "Unrelated events.").Add(10)
			assertPublisherExposition(t, want, daemon, "ze_publisher_declaration_unrelated")
		})
	}
}
