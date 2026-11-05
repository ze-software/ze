// Design: docs/plugin-development/metrics.md -- lossless forked label identity.
// Related: dispatch_metrics_publisher_regression_test.go -- real receiver helpers.
package server

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestMetricsPublisherUTF8LabelIdentity proves JSON normalization cannot merge
// a malformed label with a legitimate replacement-character label, or let a
// malformed-label deletion erase that healthy series. Invalid With is refused
// before admission; native Prometheus also rejects malformed UTF-8 rather than
// assigning it another label's identity. All accepted values use the actual
// forked receiver and are compared with the in-process registry.
func TestMetricsPublisherUTF8LabelIdentity(t *testing.T) {
	for _, kind := range publisherRegressionKinds {
		t.Run(string(kind), func(t *testing.T) {
			const name = "ze_publisher_identity_events"
			daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
			publisher := sdk.NewMetricsPublisher()
			v := publisherRegressionVector(publisher, kind, name)
			expected := publisherRegressionVector(want, kind, name)
			invalid := v.with("\xff")
			invalid(1)
			v.with("\ufffd")(2)
			expected.with("\ufffd")(2)
			v.with("healthy")(1)
			expected.with("healthy")(1)
			neighbor := publisher.Counter("ze_publisher_identity_neighbor", "Healthy neighbor.")
			expectedNeighbor := want.Counter("ze_publisher_identity_neighbor", "Healthy neighbor.")
			neighbor.Inc()
			expectedNeighbor.Inc()
			p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-identity", publisher)
			check := func() {
				expectedLines := exposition(t, want, "ze_publisher_identity_")
				actualLines := exposition(t, daemon, "ze_publisher_identity_")
				if strings.Join(expectedLines, "\n") != strings.Join(actualLines, "\n") {
					t.Errorf("malformed label changed a healthy wire identity:\nwant %v\ngot  %v", expectedLines, actualLines)
				}
			}
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("malformed label prevented valid publication: %v", err)
			}
			check()
			if v.delete("\xff") {
				t.Error("malformed UTF-8 label was admitted as a deletable series")
			}
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("malformed-label deletion poisoned publication: %v", err)
			}
			check()
			invalid(99)
			v.with("\ufffd")(3)
			expected.with("\ufffd")(3)
			v.with("healthy")(2)
			expected.with("healthy")(2)
			neighbor.Add(3)
			expectedNeighbor.Add(3)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("healthy labels stopped advancing after malformed input: %v", err)
			}
			check()
		})
	}
}

// TestMetricsPublisherDeleteLabelArity preserves the valid empty-string label
// when Delete has no arguments or too many. It then proves that the matching
// deletion still works, its old handle stays detached, and recreation starts
// from a fresh value. Prometheus DeleteLabelValues returns false on an arity
// mismatch; no malformed call may acquire a valid series' empty map key.
func TestMetricsPublisherDeleteLabelArity(t *testing.T) {
	for _, kind := range publisherRegressionKinds {
		t.Run(string(kind), func(t *testing.T) {
			const name = "ze_publisher_empty_identity_events"
			daemon, want := metrics.NewPrometheusRegistry(), metrics.NewPrometheusRegistry()
			publisher := sdk.NewMetricsPublisher()
			v := publisherRegressionVector(publisher, kind, name)
			expected := publisherRegressionVector(want, kind, name)
			empty, expectedEmpty := v.with(""), expected.with("")
			empty(5)
			expectedEmpty(5)
			v.with("healthy")(3)
			expected.with("healthy")(3)
			p := forkedMetricsPlugin(t, metricsTestServer(t, daemon), "publisher-empty-identity", publisher)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatal(err)
			}
			if v.delete() {
				t.Error("Delete with zero values deleted a valid empty-string label")
			}
			if v.delete("", "") {
				t.Error("Delete with too many values deleted a valid series")
			}
			empty(2)
			expectedEmpty(2)
			v.with("healthy")(1)
			expected.with("healthy")(1)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("arity-mismatched deletion prevented valid publication: %v", err)
			}
			actualLines := exposition(t, daemon, name)
			expectedLines := exposition(t, want, name)
			if strings.Join(expectedLines, "\n") != strings.Join(actualLines, "\n") {
				t.Errorf("arity mismatch changed a valid empty-label series:\nwant %v\ngot  %v", expectedLines, actualLines)
			}
			if !v.delete("") {
				t.Error("correct-arity deletion did not remove its live series")
			}
			if !expected.delete("") {
				t.Fatal("in-process control did not delete its live series")
			}
			empty(99)
			expectedEmpty(99)
			v.with("")(4)
			expected.with("")(4)
			if err := p.FlushMetrics(t.Context()); err != nil {
				t.Fatalf("empty-label recreation did not publish: %v", err)
			}
			assertPublisherExposition(t, want, daemon, name)
		})
	}
}
