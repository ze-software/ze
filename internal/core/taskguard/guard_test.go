package taskguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/metrics"
)

const testRestart = "the session reconnects after the connect-retry backoff"

// logRecords decodes every JSON log line written to buf.
func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON: %v: %q", err, line)
		}
		records = append(records, record)
	}
	return records
}

// scrape returns the Prometheus text exposition of reg.
func scrape(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody))
	return rec.Body.String()
}

// guardedTask runs a task that defers task.Contain at its top, the way a
// production task does, and panics with value when value is non-nil. It
// returns every *Panic end received and the task's named error result.
func guardedTask(task Task, value any) ([]*Panic, error) {
	var ended []*Panic
	describe := func() Context { return Context{Unit: "192.0.2.1", MessageType: "UPDATE"} }
	run := func() (err error) {
		defer task.Contain(describe, func(p *Panic) {
			ended = append(ended, p)
			err = p
		})
		if value != nil {
			panic(value)
		}
		return nil
	}
	err := run()
	return ended, err
}

// TestGuardRecordsPanic proves AC-2: a recovered panic stops at the boundary,
// writes one ERROR line with every field the operator needs, counts one in
// ze_task_panics_total{component,task}, writes a crash record the listing
// returns, and hands the unit's teardown the panic as an error.
func TestGuardRecordsPanic(t *testing.T) {
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	var logs bytes.Buffer
	guard := New("bgp", slog.New(slog.NewJSONHandler(&logs, nil)))
	reg := metrics.NewPrometheusRegistry()
	guard.SetMetricsRegistry(reg)
	task := guard.Task("read", testRestart)

	ended, err := guardedTask(task, "boom")

	var p *Panic
	if !errors.As(err, &p) {
		t.Fatalf("task returned %v, want the *Panic end set", err)
	}
	if len(ended) != 1 {
		t.Fatalf("end ran %d times, want 1", len(ended))
	}
	if p.Value != "boom" || p.Unit != "192.0.2.1" || p.MessageType != "UPDATE" {
		t.Fatalf("panic = %+v, want value boom for unit 192.0.2.1 on UPDATE", p)
	}
	if !strings.Contains(string(p.Stack), "guardedTask") {
		t.Errorf("stack does not reach the panicking frame:\n%s", p.Stack)
	}

	records := logRecords(t, &logs)
	if len(records) != 1 {
		t.Fatalf("got %d log lines, want 1: %v", len(records), records)
	}
	line := records[0]
	want := map[string]string{
		"level":        "ERROR",
		"component":    "bgp",
		"task":         "read",
		"unit":         "192.0.2.1",
		"message-type": "UPDATE",
		"panic":        "boom",
		"restart":      testRestart,
		"next":         "ze show crashes name " + p.Report,
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("log field %s = %v, want %q", key, line[key], value)
		}
	}
	if msg, _ := line["msg"].(string); !strings.Contains(msg, "internal Ze defect") {
		t.Errorf("log message %q does not say an internal Ze defect ended the unit", msg)
	}
	if stack, _ := line["stack"].(string); !strings.Contains(stack, "guardedTask") {
		t.Errorf("log stack does not reach the panicking frame: %q", stack)
	}

	if got, declared := guard.Panics("read"); got != 1 || !declared {
		t.Errorf("Panics(read) = %d, %v, want 1, true", got, declared)
	}
	if _, declared := guard.Panics("raed"); declared {
		t.Error("Panics answered for a task that was never declared")
	}
	if exposition := scrape(t, reg); !strings.Contains(exposition, `ze_task_panics_total{component="bgp",task="read"} 1`) {
		t.Errorf("metric not incremented once:\n%s", exposition)
	}

	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes returned %d records, want 1", len(crashes))
	}
	if crashes[0].Name != p.Report || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash record %+v, want %s of kind %s", crashes[0], p.Report, crashlog.KindRecovered)
	}
	if report := crashlog.ReadCrash(p.Report); !strings.Contains(report, "Origin: bgp read, unit 192.0.2.1, message UPDATE") {
		t.Errorf("crash record does not name the unit:\n%s", report)
	}
}

// TestGuardNoPanicIsSilent proves AC-9: with no panic the boundary logs
// nothing, counts nothing, records nothing and never calls end.
func TestGuardNoPanicIsSilent(t *testing.T) {
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	var logs bytes.Buffer
	guard := New("bgp", slog.New(slog.NewJSONHandler(&logs, nil)))
	reg := metrics.NewPrometheusRegistry()
	guard.SetMetricsRegistry(reg)
	task := guard.Task("read", testRestart)

	ended, err := guardedTask(task, nil)

	if err != nil {
		t.Fatalf("task returned %v with no panic", err)
	}
	if len(ended) != 0 {
		t.Fatalf("end ran %d times with no panic", len(ended))
	}
	if logs.Len() != 0 {
		t.Fatalf("logged with no panic: %s", logs.String())
	}
	if got, declared := guard.Panics("read"); got != 0 || !declared {
		t.Fatalf("Panics(read) = %d, %v with no panic, want 0, true", got, declared)
	}
	if exposition := scrape(t, reg); !strings.Contains(exposition, `ze_task_panics_total{component="bgp",task="read"} 0`) {
		t.Errorf("declared task has no zero series:\n%s", exposition)
	}
	if crashes := crashlog.ListCrashes(); len(crashes) != 0 {
		t.Fatalf("crash records written with no panic: %+v", crashes)
	}
}

// TestGuardCountsBeforeRegistry proves a panic recovered before the component
// had a metrics registry is not lost: the registry, when it arrives, shows it.
func TestGuardCountsBeforeRegistry(t *testing.T) {
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	guard := New("ospf", slog.New(slog.DiscardHandler))
	task := guard.Task("receive", testRestart)
	_, err := guardedTask(task, 42)
	var p *Panic
	if !errors.As(err, &p) {
		t.Fatalf("task returned %v after a panic, want *Panic", err)
	}
	// A panic value that is neither a string nor an error still reaches the
	// crash record as itself.
	if report := crashlog.ReadCrash(p.Report); !strings.Contains(report, "\n=== Panic ===\n42\n") {
		t.Errorf("integer panic value missing from the crash record:\n%s", report)
	}

	reg := metrics.NewPrometheusRegistry()
	guard.SetMetricsRegistry(reg)
	if exposition := scrape(t, reg); !strings.Contains(exposition, `ze_task_panics_total{component="ospf",task="receive"} 1`) {
		t.Errorf("panic before the registry is missing from the metric:\n%s", exposition)
	}
}

// TestGuardNoCrashDirSaysSo proves the log line tells the operator when no
// crash record could be written, instead of pointing at a record that does not
// exist.
func TestGuardNoCrashDirSaysSo(t *testing.T) {
	restore := crashlog.SetCrashDirForTest("")
	t.Cleanup(restore)

	var logs bytes.Buffer
	guard := New("bgp", slog.New(slog.NewJSONHandler(&logs, nil)))
	task := guard.Task("read", testRestart)
	_, err := guardedTask(task, errors.New("boom"))

	var p *Panic
	if !errors.As(err, &p) {
		t.Fatalf("task returned %v, want *Panic", err)
	}
	if p.Report != "" {
		t.Fatalf("Report = %q with no crash directory", p.Report)
	}
	records := logRecords(t, &logs)
	if len(records) != 1 {
		t.Fatalf("got %d log lines, want 1", len(records))
	}
	if reason, _ := records[0]["crash-record-error"].(string); !strings.Contains(reason, "ze.crash.dir") {
		t.Errorf("log does not say why no record was written: %v", records[0])
	}
	if next, _ := records[0]["next"].(string); strings.Contains(next, "show crashes") {
		t.Errorf("log points at a crash record that was never written: %q", next)
	}
}
