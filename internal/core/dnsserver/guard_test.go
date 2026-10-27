// Design: docs/architecture/dns/server-harness.md -- the per-query panic boundary
// Related: guard.go -- queryGuard and describeQuery under test

package dnsserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// swapQueryGuard replaces queryGuard with a guard that logs JSON into the
// returned buffer, points crash records at a temporary directory, and restores
// both when t ends. Build the handler under test after calling it: a wrapper
// declares its task on queryGuard when it is built.
func swapQueryGuard(t *testing.T) (*taskguard.Guard, *bytes.Buffer) {
	t.Helper()
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := queryGuard
	var logs bytes.Buffer
	queryGuard = taskguard.New("dns", slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() {
		queryGuard = prior
		restoreCrashDir()
	})
	return queryGuard, &logs
}

// requireQueryContained asserts that exactly one query panic with value
// "boom" reached task: one ERROR line naming the component, the task, the
// client and the question, one count, and one recovered crash record.
func requireQueryContained(t *testing.T, guard *taskguard.Guard, logs *bytes.Buffer, task string) {
	t.Helper()
	if count, declared := guard.Panics(task); count != 1 || !declared {
		t.Errorf("Panics(%s) = %d, %v, want 1, true: the panic was not counted", task, count, declared)
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), logs.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, lines[0])
	}
	want := map[string]string{"level": "ERROR", "component": "dns", "task": task, "panic": "boom"}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("log field %s = %v, want %q", key, line[key], value)
		}
	}
	if unit, _ := line["unit"].(string); unit != "query from 192.0.2.1:5353, question example.test. A" {
		t.Errorf("log unit = %q, want the client and the question", unit)
	}

	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes returned %d records, want 1", len(crashes))
	}
	if crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash record kind %q, want %q", crashes[0].Kind, crashlog.KindRecovered)
	}
}

// TestQueryPanicNeverSilent proves a panic in an answer func is reported
// whatever the consumer passed: with a nil log, which is what the old
// onPanic-less callers passed, each wrapper still writes the ERROR line,
// counts the panic in ze_task_panics_total{component="dns"} on the registry
// SetMetricsRegistry received, writes a crash record, and sends no reply.
// METHOD: build each wrapper under a swapped queryGuard, panic inside fn, then
// read the guard, the log, the crash listing and the scraped registry.
// PREVENTS: the per-query recover swallowing a panic with no line, no count
// and no record when the consumer gave it nothing to call.
func TestQueryPanicNeverSilent(t *testing.T) {
	wrappers := []struct {
		task string
		wrap func(*slog.Logger, AnswerFunc) dns.HandlerFunc
	}{
		{taskAuthoritativeQuery, Authoritative},
		{taskRecursiveQuery, Recursive},
	}
	for _, wrapper := range wrappers {
		t.Run(wrapper.task, func(t *testing.T) {
			guard, logs := swapQueryGuard(t)
			priorMetrics := harnessMetricsPtr.Load()
			t.Cleanup(func() { harnessMetricsPtr.Store(priorMetrics) })
			reg := metrics.NewPrometheusRegistry()
			SetMetricsRegistry(reg)

			handler := wrapper.wrap(nil, func(msg, r *dns.Msg, p Peer) bool { panic("boom") })
			q := new(dns.Msg)
			q.SetQuestion("example.test.", dns.TypeA)
			fw := &fakeResponseWriter{}
			handler(fw, q)

			if fw.written != nil {
				t.Error("a reply was written for a query whose answer func panicked, want none")
			}
			requireQueryContained(t, guard, logs, wrapper.task)

			rec := httptest.NewRecorder()
			reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody))
			series := taskguard.MetricName + `{component="dns",task="` + wrapper.task + `"} 1`
			if !strings.Contains(rec.Body.String(), series) {
				t.Errorf("registry does not carry %s:\n%s", series, rec.Body.String())
			}
		})
	}
}
