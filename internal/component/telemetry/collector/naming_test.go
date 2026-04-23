package collector

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// TestNetdataMetricNaming verifies that collector-registered metrics follow
// the Netdata Prometheus naming convention:
//
//	{prefix}_{context}_{units}_{suffix}{chart="...",dimension="...",family="..."}
func TestNetdataMetricNaming(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()

	stub := &loadAvgStub{}
	stub.Init(reg, "netdata")

	srv := httptest.NewServer(reg.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL) //nolint:noctx // test-only HTTP call
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	output := string(body)

	requiredMetrics := []string{
		`netdata_system_load_load_average{chart="system.load",dimension="load1",family="load"}`,
		`netdata_system_load_load_average{chart="system.load",dimension="load5",family="load"}`,
		`netdata_system_load_load_average{chart="system.load",dimension="load15",family="load"}`,
	}
	for _, want := range requiredMetrics {
		if !strings.Contains(output, want) {
			t.Errorf("missing expected metric line containing: %s", want)
		}
	}

	if !strings.Contains(output, "# HELP netdata_system_load_load_average") {
		t.Error("missing HELP line for system.load")
	}
	if !strings.Contains(output, "# TYPE netdata_system_load_load_average gauge") {
		t.Error("missing TYPE gauge line for system.load")
	}
}

// TestCustomPrefix verifies that a non-default prefix propagates to metric names.
func TestCustomPrefix(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	stub := &loadAvgStub{}
	stub.Init(reg, "myprefix")

	srv := httptest.NewServer(reg.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL) //nolint:noctx // test-only HTTP call
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "myprefix_system_load_load_average") {
		t.Error("custom prefix not applied to metric name")
	}
	if strings.Contains(string(body), "netdata_") {
		t.Error("default prefix leaked when custom prefix was set")
	}
}

// loadAvgStub sets fixed load average values without reading /proc.
type loadAvgStub struct {
	gauge metrics.GaugeVec
}

func (s *loadAvgStub) Name() string { return "loadavg-stub" }

func (s *loadAvgStub) Init(reg metrics.Registry, prefix string) {
	s.gauge = reg.GaugeVec(
		prefix+"_system_load_load_average",
		"System Load Average",
		[]string{"chart", "dimension", "family"},
	)
	s.gauge.With("system.load", "load1", "load").Set(0.5)
	s.gauge.With("system.load", "load5", "load").Set(0.3)
	s.gauge.With("system.load", "load15", "load").Set(0.2)
}

func (s *loadAvgStub) Collect() error { return nil }

// TestManagerCollectInterval verifies the manager respects the tick interval.
func TestManagerCollectInterval(t *testing.T) {
	reg := metrics.NopRegistry{}
	m := NewManager(reg, "test", 50*time.Millisecond, nil)

	count := 0
	m.Register(&countingCollector{collectFn: func() { count++ }})
	m.Start()

	time.Sleep(180 * time.Millisecond)
	m.Stop()

	if count < 3 {
		t.Fatalf("collect called %d times, want >= 3 (1 immediate + 2 ticks)", count)
	}
}

type countingCollector struct {
	collectFn func()
}

func (c *countingCollector) Name() string                      { return "counter" }
func (c *countingCollector) Init(_ metrics.Registry, _ string) {}
func (c *countingCollector) Collect() error                    { c.collectFn(); return nil }
