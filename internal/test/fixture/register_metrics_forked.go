// Design: docs/plugin-development/metrics.md -- a plugin run in its own process
// Related: plugin_fixture_11.go -- dispatchMap11 and the in-process metrics observer

package fixture

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/metrics-forked", metricsForked)
}

const (
	// forkedMetricName is the counter the forked observer owns. It carries the
	// ze_ prefix the engine requires of a published name (pkg/plugin/rpc).
	forkedMetricName = "ze_fixture_forked_events_total"
	// forkedMetricInterval is the plugin's chosen cadence, the shortest the SDK
	// accepts, so the interval publish lands well inside the poll budget.
	forkedMetricInterval = 100 * time.Millisecond
)

// metricsForked proves that a plugin ze forks publishes its own counter into
// the daemon's registry, under the name it gave it, at the cadence it chose.
//
// The observer IS the forked plugin: ze runs it from `plugin { external ... }`
// as its own process, so the counter lives in that process and reaches the
// daemon only over ze-plugin-engine:metrics-publish. Three states are read from
// `show metrics name`, which the daemon answers from its own registry: the
// declaration at zero (sent at once, before any update), the value after an
// increment the plugin's interval alone carries, and the value after a second
// increment FlushMetrics carries. The last read is one dispatch with no poll,
// because FlushMetrics returns only once the engine has applied the update.
func metricsForked(ctx context.Context, _ []string) error {
	var events metrics.Counter
	setup := func(p *sdk.Plugin) error {
		if err := p.SetMetricsInterval(forkedMetricInterval); err != nil {
			return fmt.Errorf("set metrics interval: %w", err)
		}
		events = p.MetricsRegistry().Counter(forkedMetricName, "Events the forked fixture counted.")
		return nil
	}
	return observeConfigured(ctx, "plugin-metrics-forked", &sdk.Registration{}, setup, func(ctx context.Context, p *sdk.Plugin) error {
		if err := waitForkedMetric(ctx, p, 0); err != nil {
			return fmt.Errorf("declaration: %w", err)
		}
		events.Add(3)
		if err := waitForkedMetric(ctx, p, 3); err != nil {
			return fmt.Errorf("interval publish: %w", err)
		}
		events.Add(2)
		if err := p.FlushMetrics(ctx); err != nil {
			return fmt.Errorf("flush metrics: %w", err)
		}
		series, err := forkedMetricSeries(ctx, p)
		if err != nil {
			return err
		}
		if want := forkedMetricLine(5); !slices.Contains(series, want) {
			return fmt.Errorf("after FlushMetrics the daemon shows %q, want %q", series, want)
		}
		fmt.Fprintln(os.Stderr, "OK: "+forkedMetricLine(5)+" after FlushMetrics")
		// Returning dispatches `request shutdown`; the peer still expects the
		// initial-sync End-of-RIB, which a Cease would overtake (metricsOwned).
		return fixture10WaitEOR(ctx, p, "*", 60)
	})
}

// waitForkedMetric polls the daemon until its exposition of the forked counter
// is exactly value, and returns an error naming the last series it saw.
func waitForkedMetric(ctx context.Context, p *sdk.Plugin, value int) error {
	want := forkedMetricLine(value)
	var series []string
	var err error
	ok := Poll(ctx, 40, 250*time.Millisecond, func() bool {
		series, err = forkedMetricSeries(ctx, p)
		return err == nil && slices.Contains(series, want)
	})
	if ok {
		fmt.Fprintln(os.Stderr, "OK: "+want)
		return nil
	}
	if err != nil {
		return fmt.Errorf("want %q: %w", want, err)
	}
	return fmt.Errorf("the daemon shows %q, want %q", series, want)
}

// forkedMetricSeries answers the exposition lines the daemon holds for the
// forked counter.
func forkedMetricSeries(ctx context.Context, p *sdk.Plugin) ([]string, error) {
	status, data, err := dispatchMap11(ctx, p, "show metrics name "+forkedMetricName)
	if err != nil {
		return nil, err
	}
	if status != statusDone {
		return nil, fmt.Errorf("show metrics name %s: status=%s", forkedMetricName, status)
	}
	raw, ok := data["series"].([]any)
	if !ok {
		return nil, fmt.Errorf("show metrics name %s: series is %T, want a list", forkedMetricName, data["series"])
	}
	series := make([]string, 0, len(raw))
	for _, item := range raw {
		line, isText := item.(string)
		if !isText {
			return nil, fmt.Errorf("show metrics name %s: series item is %T, want text", forkedMetricName, item)
		}
		series = append(series, line)
	}
	return series, nil
}

// forkedMetricLine is the exposition line of the forked counter at value.
func forkedMetricLine(value int) string {
	return forkedMetricName + " " + strconv.Itoa(value)
}
