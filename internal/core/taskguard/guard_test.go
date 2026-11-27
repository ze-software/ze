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
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
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
// nothing, counts nothing, records nothing, raises no report errors and never
// calls end.
func TestGuardNoPanicIsSilent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

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
	if errs := report.Errors(0); len(errs) != 0 {
		t.Fatalf("report errors raised with no panic: %+v", errs)
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

// TestGuardRaisesReportErrorAndLogsDoctorCode proves a recovered panic logs
// the doctor-internal-defect code and raises a matching error event on the
// operational report bus with structured detail and crash-report fix command.
//
// VALIDATES: AC-5 -- Task.Contain logs "code": report.CodeInternalDefect and
// raises an error event on internal/core/report with source, code, fix, and
// detail fields so show errors, show doctor, and show system triage surface it.
// PREVENTS: recovered network-input panics staying invisible to show errors
// and show doctor's recent-events check.
func TestGuardRaisesReportErrorAndLogsDoctorCode(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	var logs bytes.Buffer
	guard := New("bgp", slog.New(slog.NewJSONHandler(&logs, nil)))
	task := guard.Task("read", testRestart)

	_, err := guardedTask(task, "boom")
	var p *Panic
	if !errors.As(err, &p) {
		t.Fatalf("task returned %v, want *Panic", err)
	}

	records := logRecords(t, &logs)
	if len(records) != 1 {
		t.Fatalf("got %d log lines, want 1", len(records))
	}
	if got := records[0]["code"]; got != report.CodeInternalDefect {
		t.Errorf("log field code = %v, want %q", got, report.CodeInternalDefect)
	}

	errs := report.Errors(0)
	if len(errs) != 1 {
		t.Fatalf("report.Errors(0) returned %d issues, want 1", len(errs))
	}
	issue := errs[0]
	if issue.Source != "bgp" {
		t.Errorf("issue.Source = %q, want %q", issue.Source, "bgp")
	}
	if issue.Code != report.CodeInternalDefect {
		t.Errorf("issue.Code = %q, want %q", issue.Code, report.CodeInternalDefect)
	}
	if issue.Severity != report.SeverityError {
		t.Errorf("issue.Severity = %v, want %v", issue.Severity, report.SeverityError)
	}
	wantFix := "ze show crashes name " + p.Report
	if issue.Fix != wantFix {
		t.Errorf("issue.Fix = %q, want %q", issue.Fix, wantFix)
	}
	wantDetail := map[string]string{
		"task":         "read",
		"unit":         "192.0.2.1",
		"message-type": "UPDATE",
		"panic":        "boom",
		"crash-report": p.Report,
	}
	for k, want := range wantDetail {
		if got := issue.Detail[k]; got != want {
			t.Errorf("issue.Detail[%q] = %v, want %q", k, got, want)
		}
	}
}

// TestGuardReportRaiseFallbackFixWithoutCrashDir proves a recovered panic
// without a writable crash directory still raises an internal-defect error on
// the report bus using the log-inspection fallback fix instead of pointing at
// a nonexistent crash record.
//
// VALIDATES: AC-5 -- when crashDir is empty, report.Errors(0) contains one
// doctor-internal-defect issue with Fix == "ze show log recent level err"
// and no "crash-report" detail key.
// PREVENTS: a report-bus fix command directing operators to a crash file that
// was never written.
func TestGuardReportRaiseFallbackFixWithoutCrashDir(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	restore := crashlog.SetCrashDirForTest("")
	t.Cleanup(restore)

	var logs bytes.Buffer
	guard := New("bgp", slog.New(slog.NewJSONHandler(&logs, nil)))
	task := guard.Task("read", testRestart)

	_, err := guardedTask(task, "boom")
	var p *Panic
	if !errors.As(err, &p) {
		t.Fatalf("task returned %v, want *Panic", err)
	}
	if p.Report != "" {
		t.Fatalf("Report = %q with no crash directory, want empty", p.Report)
	}

	records := logRecords(t, &logs)
	if len(records) != 1 {
		t.Fatalf("got %d log lines, want 1", len(records))
	}
	if got := records[0]["code"]; got != report.CodeInternalDefect {
		t.Errorf("log field code = %v, want %q", got, report.CodeInternalDefect)
	}

	errs := report.Errors(0)
	if len(errs) != 1 {
		t.Fatalf("report.Errors(0) returned %d issues, want 1", len(errs))
	}
	issue := errs[0]
	if issue.Code != report.CodeInternalDefect {
		t.Errorf("issue.Code = %q, want %q", issue.Code, report.CodeInternalDefect)
	}
	if issue.Fix != fallbackFixCommand {
		t.Errorf("issue.Fix = %q, want %q", issue.Fix, fallbackFixCommand)
	}
	if _, hasCrash := issue.Detail["crash-report"]; hasCrash {
		t.Errorf("issue.Detail unexpectedly contains crash-report: %v", issue.Detail["crash-report"])
	}
}

// TestGuardReportRaiseBoundsOversizedFields proves empty component/unit fields
// and oversized multi-byte UTF-8 panic/unit values are bounded cleanly so
// report.RaiseErrorFix always accepts the issue and preserves valid UTF-8.
//
// VALIDATES: AC-5, R-3 -- empty source/subject fall back to "unknown" and
// oversized multi-byte UTF-8 fields truncate at UTF-8 rune boundaries within
// report.SourceMax, report.SubjectMax, report.MessageMax, and report.FixMax.
// PREVENTS: report.validFields silently dropping a recovered panic when a
// panic value or unit identifier is empty or exceeds report bus limits.
func TestGuardReportRaiseBoundsOversizedFields(t *testing.T) {
	t.Run("empty component and unit", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		restore := crashlog.SetCrashDirForTest(t.TempDir())
		t.Cleanup(restore)

		guard := New("", slog.New(slog.DiscardHandler))
		task := guard.Task("read", testRestart)
		func() {
			defer task.Contain(func() Context { return Context{} }, nil)
			panic("nil deref")
		}()

		errs := report.Errors(0)
		if len(errs) != 1 {
			t.Fatalf("report.Errors(0) returned %d issues, want 1", len(errs))
		}
		issue := errs[0]
		if issue.Source != unknownField {
			t.Errorf("issue.Source = %q, want %q", issue.Source, unknownField)
		}
		if issue.Code != report.CodeInternalDefect {
			t.Errorf("issue.Code = %q, want %q", issue.Code, report.CodeInternalDefect)
		}
		if issue.Subject == "" || len(issue.Subject) > report.SubjectMax || !utf8.ValidString(issue.Subject) {
			t.Errorf("issue.Subject invalid: len=%d valid=%v value=%q", len(issue.Subject), utf8.ValidString(issue.Subject), issue.Subject)
		}
		if _, hasMsgType := issue.Detail["message-type"]; hasMsgType {
			t.Errorf("issue.Detail unexpectedly contains message-type: %v", issue.Detail["message-type"])
		}
	})

	t.Run("oversized multi-byte UTF-8 fields", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		restore := crashlog.SetCrashDirForTest(t.TempDir())
		t.Cleanup(restore)

		longComponent := strings.Repeat("é", 100)
		longUnit := strings.Repeat("日", 700)
		longPanic := strings.Repeat("🔥", 600)

		guard := New(longComponent, slog.New(slog.DiscardHandler))
		task := guard.Task("read", testRestart)
		func() {
			defer task.Contain(func() Context { return Context{Unit: longUnit, MessageType: "UPDATE"} }, nil)
			panic(longPanic)
		}()

		errs := report.Errors(0)
		if len(errs) != 1 {
			t.Fatalf("report.Errors(0) returned %d issues, want 1", len(errs))
		}
		issue := errs[0]
		if issue.Source == "" || len(issue.Source) > report.SourceMax || !utf8.ValidString(issue.Source) {
			t.Errorf("issue.Source invalid: len=%d valid=%v", len(issue.Source), utf8.ValidString(issue.Source))
		}
		if issue.Subject == "" || len(issue.Subject) > report.SubjectMax || !utf8.ValidString(issue.Subject) {
			t.Errorf("issue.Subject invalid: len=%d valid=%v", len(issue.Subject), utf8.ValidString(issue.Subject))
		}
		if issue.Message == "" || len(issue.Message) > report.MessageMax || !utf8.ValidString(issue.Message) {
			t.Errorf("issue.Message invalid: len=%d valid=%v", len(issue.Message), utf8.ValidString(issue.Message))
		}
		if issue.Fix == "" || len(issue.Fix) > report.FixMax || !utf8.ValidString(issue.Fix) {
			t.Errorf("issue.Fix invalid: len=%d valid=%v", len(issue.Fix), utf8.ValidString(issue.Fix))
		}
	})

	t.Run("boundReportField boundary cases", func(t *testing.T) {
		if got := boundReportField("", 16, unknownField); got != unknownField {
			t.Errorf("boundReportField empty = %q, want %q", got, unknownField)
		}
		if got := boundReportField("exact", 5, unknownField); got != "exact" {
			t.Errorf("boundReportField exact = %q, want %q", got, "exact")
		}
		if got := boundReportField("🔥", 2, unknownField); got != unknownField {
			t.Errorf("boundReportField rune wider than max = %q, want %q", got, unknownField)
		}
		got := boundReportField("a日b", 3, unknownField)
		if got != "a" || !utf8.ValidString(got) {
			t.Errorf("boundReportField mid-rune = %q (valid=%v), want %q", got, utf8.ValidString(got), "a")
		}
	})
}
