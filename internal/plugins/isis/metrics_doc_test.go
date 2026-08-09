// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- the canonical ze_isis_* metric
// set assertion (AC-10), extended to the OPERATOR-FACING documentation.
// Related: metrics_test.go -- TestISISMetricsRegistered asserts the code registers
//   the canonical set; THIS file asserts docs/plugin-development/metrics.md
//   documents exactly that set (no drift in either direction).
//
// VALIDATES: every ze_isis_* series the engine + transport + redistribution
// consumer register with a real registry appears as a row in the metrics.md
// "Full Inventory" table with the same type and label set, and that table
// documents no ze_isis_* series that is not actually registered.
// PREVENTS: a registered series with no documentation row (the B9 finding: 8 of
// 30 ze_isis_* series -- the four SPF and four transport series -- were missing
// from the inventory table), or a stale documented row for a series that was
// renamed/removed.

package isis

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// docRow is one parsed ze_isis_* row of the metrics.md "Full Inventory" table:
// the metric type token (Counter/Gauge/CounterVec/...) and the sorted label set.
type docRow struct {
	typ    string
	labels []string
}

// docInventoryRowRE matches a markdown table row of the form
// `| `+"`ze_isis_xyz`"+` | <Type> | <labels> | <owner> |` and captures the metric
// name, the type token, and the raw label cell. Backtick-wrapped names and
// pipe-delimited cells are the fixed table shape (see metrics.md "Full Inventory").
var docInventoryRowRE = regexp.MustCompile(
	"^\\|\\s*`(ze_isis_[a-z0-9_]+)`\\s*\\|\\s*([A-Za-z]+)\\s*\\|\\s*([^|]*)\\|",
)

// parseISISDocRows extracts every ze_isis_* row from the metrics.md inventory
// table: name -> {type, sorted labels}. Labels are the comma-separated tokens in
// the third cell (empty cell -> no labels).
func parseISISDocRows(t *testing.T, path string) map[string]docRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metrics doc %s: %v", path, err)
	}
	rows := make(map[string]docRow)
	for line := range strings.SplitSeq(string(data), "\n") {
		m := docInventoryRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, typ, labelCell := m[1], m[2], strings.TrimSpace(m[3])
		var labels []string
		for l := range strings.SplitSeq(labelCell, ",") {
			if l = strings.TrimSpace(l); l != "" {
				labels = append(labels, l)
			}
		}
		sort.Strings(labels)
		rows[name] = docRow{typ: typ, labels: labels}
	}
	return rows
}

// registeredTypeRegistry captures, for each registered series, the metric TYPE
// (Counter/Gauge/CounterVec/...) and its sorted label set -- the two columns the
// inventory table documents -- so the doc rows can be checked against the real
// registration surface.
type registeredTypeRegistry struct {
	metrics.NopRegistry
	rows map[string]docRow
}

func newRegisteredTypeRegistry() *registeredTypeRegistry {
	return &registeredTypeRegistry{rows: make(map[string]docRow)}
}

func (r *registeredTypeRegistry) record(name, typ string, labelNames []string) {
	ls := append([]string(nil), labelNames...)
	sort.Strings(ls)
	r.rows[name] = docRow{typ: typ, labels: ls}
}

func (r *registeredTypeRegistry) Counter(name, help string) metrics.Counter {
	r.record(name, "Counter", nil)
	return r.NopRegistry.Counter(name, help)
}

func (r *registeredTypeRegistry) Gauge(name, help string) metrics.Gauge {
	r.record(name, "Gauge", nil)
	return r.NopRegistry.Gauge(name, help)
}

func (r *registeredTypeRegistry) CounterVec(name, help string, labelNames []string) metrics.CounterVec {
	r.record(name, "CounterVec", labelNames)
	return r.NopRegistry.CounterVec(name, help, labelNames)
}

func (r *registeredTypeRegistry) GaugeVec(name, help string, labelNames []string) metrics.GaugeVec {
	r.record(name, "GaugeVec", labelNames)
	return r.NopRegistry.GaugeVec(name, help, labelNames)
}

func (r *registeredTypeRegistry) Histogram(name, help string, buckets []float64) metrics.Histogram {
	r.record(name, "Histogram", nil)
	return r.NopRegistry.Histogram(name, help, buckets)
}

func (r *registeredTypeRegistry) HistogramVec(name, help string, buckets []float64, labelNames []string) metrics.HistogramVec {
	r.record(name, "HistogramVec", labelNames)
	return r.NopRegistry.HistogramVec(name, help, buckets, labelNames)
}

// TestISISMetricsDocumented asserts the metrics.md "Full Inventory" table
// documents EXACTLY the ze_isis_* series the engine + transport + redistribution
// consumer register, with the same metric type and label set. It is the
// doc-drift guard for the B9 finding (SPF + transport series missing from the
// table).
func TestISISMetricsDocumented(t *testing.T) {
	reg := newRegisteredTypeRegistry()
	wireAllMetrics(reg)

	root := findRepoRootForDoc(t)
	docPath := filepath.Join(root, "docs", "plugin-development", "metrics.md")
	docRows := parseISISDocRows(t, docPath)

	// Every registered ze_isis_* series must have a doc row with the same type and
	// labels (catches the missing-row drift this test exists for).
	for name, got := range reg.rows {
		if !strings.HasPrefix(name, "ze_isis_") {
			continue
		}
		doc, ok := docRows[name]
		if !ok {
			t.Errorf("series %q is registered but missing from %s Full Inventory table", name, docPath)
			continue
		}
		if doc.typ != got.typ {
			t.Errorf("series %q documented type = %q, registered type = %q", name, doc.typ, got.typ)
		}
		if !equalStrings(doc.labels, got.labels) {
			t.Errorf("series %q documented labels = %v, registered labels = %v", name, doc.labels, got.labels)
		}
	}

	// Conversely, every documented ze_isis_* row must correspond to a registered
	// series (catches a stale row for a renamed/removed series).
	for name := range docRows {
		if _, ok := reg.rows[name]; !ok {
			t.Errorf("metrics.md documents ze_isis_* series %q that is not registered", name)
		}
	}
}

// findRepoRootForDoc walks up from the test's working directory to the directory
// holding go.mod (the repo root), so the test can locate docs/ regardless of the
// package the test runs from.
func findRepoRootForDoc(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}
