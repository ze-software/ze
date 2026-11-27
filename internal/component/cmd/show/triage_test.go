// Design: docs/architecture/diagnostics/production-diagnostics.md -- unit tests for show system triage
// Related: triage.go -- handleShowSystemTriage handler

package show

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestHandleShowSystemTriagePopulated verifies handleShowSystemTriage returns
// StatusDone with all ten top-level lowercase kebab-case triage sections,
// populated sources, warn/error-only log filtering, and clean pipe rendering.
//
// VALIDATES: AC-1, AC-4 -- show system triage aggregates version, health,
// doctor, errors, warnings, crashes, plugins, reload, recent-logs (warn/error
// only), and runtime, and renders cleanly through json, yaml, and table pipes.
// PREVENTS: missing or non-kebab-case triage keys, info/debug log leakage into
// recent-logs, or responses that fail pipe rendering.
func TestHandleShowSystemTriagePopulated(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	crashDir := t.TempDir()
	restoreCrashDir := crashlog.SetCrashDirForTest(crashDir)
	t.Cleanup(restoreCrashDir)

	crashFile := "crash-20261127-060000.log"
	require.NoError(t, os.WriteFile(filepath.Join(crashDir, crashFile), []byte("panic: sample"), 0o600))

	report.RaiseError("bgp", "notification-received", "192.0.2.1", "peer sent cease", nil)
	report.RaiseWarning("bgp", "prefix-threshold", "192.0.2.2", "approaching max prefixes", nil)

	const logSub = "triage.test.populated"
	log := slogutil.Logger(logSub)
	require.NoError(t, slogutil.SetLevel(logSub, "debug"))
	t.Cleanup(func() {
		_ = slogutil.SetLevel(logSub, "warn") //nolint:errcheck // best-effort test cleanup
	})

	const (
		debugMsg = "triage-populated-debug-hidden"
		infoMsg  = "triage-populated-info-hidden"
		warnMsg  = "triage-populated-warn-visible"
		errorMsg = "triage-populated-error-visible"
	)
	log.Debug(debugMsg)
	log.Info(infoMsg)
	log.Warn(warnMsg, "peer", "192.0.2.10")
	log.Error(errorMsg, "peer", "192.0.2.11")

	// Confirm the debug and info entries actually reached the global ring so
	// their absence below proves triageRecentLogs filtered them out.
	require.True(t, ringContainsMessage(slogutil.GlobalLogRing().Recent(0), debugMsg))
	require.True(t, ringContainsMessage(slogutil.GlobalLogRing().Recent(0), infoMsg))

	srv, err := pluginserver.NewServer(nil, nil)
	require.NoError(t, err)
	require.NoError(t, srv.Dispatcher().Register("show doctor", func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"status": "ok", "checks": []any{}},
		}, nil
	}, "test show doctor"))
	srv.MarkReloadProcessed(true)

	ctx := &pluginserver.CommandContext{Server: srv}
	resp, err := handleShowSystemTriage(ctx, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be plugin.Map, got %T", resp.Data)

	sections := []string{
		"version",
		"health",
		"doctor",
		"errors",
		"warnings",
		"crashes",
		"plugins",
		"reload",
		"recent-logs",
		"runtime",
	}
	for _, key := range sections {
		_, present := data[key]
		assert.True(t, present, "missing top-level triage key %q", key)
	}

	versionSec := requireTriageMap(t, data, "version")
	assert.NotEmpty(t, versionSec["release"])
	assert.NotEmpty(t, versionSec["build-date"])
	goVer, ok := versionSec["go"].(string)
	require.True(t, ok, "version.go should be string, got %T", versionSec["go"])
	assert.True(t, strings.HasPrefix(goVer, "go"), "version.go=%q should start with go", goVer)
	assert.Equal(t, runtime.GOOS, versionSec["os"])
	assert.Equal(t, runtime.GOARCH, versionSec["arch"])

	healthSec := requireTriageMap(t, data, "health")
	assert.NotEmpty(t, healthSec["status"])
	_, hasCheckedAt := healthSec["checked-at"]
	assert.True(t, hasCheckedAt, "health section must include checked-at")
	_, hasComponents := healthSec["components"]
	assert.True(t, hasComponents, "health section must include components")

	doctorSec := requireTriageMap(t, data, "doctor")
	assert.Equal(t, true, doctorSec["available"])
	assert.Equal(t, "ok", doctorSec["status"])

	errorsSec := requireTriageMap(t, data, "errors")
	assert.Equal(t, 1, errorsSec["count"])
	assert.Equal(t, 1, errorsSec["total-count"])
	assert.Equal(t, false, errorsSec["truncated"])

	warningsSec := requireTriageMap(t, data, "warnings")
	assert.Equal(t, 1, warningsSec["count"])
	assert.Equal(t, 1, warningsSec["total-count"])
	assert.Equal(t, false, warningsSec["truncated"])

	crashesSec := requireTriageMap(t, data, "crashes")
	assert.Equal(t, 1, crashesSec["count"])
	assert.Equal(t, 1, crashesSec["total-count"])
	assert.Equal(t, false, crashesSec["truncated"])
	assert.Equal(t, crashDir, crashesSec["dir"])

	reloadSec := requireTriageMap(t, data, "reload")
	assert.Equal(t, true, reloadSec["available"])
	assert.Equal(t, uint64(1), reloadSec["generation"])
	assert.Equal(t, pluginserver.ReloadOutcomeApplied, reloadSec["last-outcome"])
	assert.NotEmpty(t, reloadSec["last-reload-at"])

	logsSec := requireTriageMap(t, data, "recent-logs")
	logRows, ok := logsSec["entries"].([]map[string]any)
	require.True(t, ok, "recent-logs.entries should be []map[string]any, got %T", logsSec["entries"])
	assert.True(t, triageRowsContainMessage(logRows, warnMsg), "warn log must appear in recent-logs")
	assert.True(t, triageRowsContainMessage(logRows, errorMsg), "error log must appear in recent-logs")
	assert.False(t, triageRowsContainMessage(logRows, debugMsg), "debug log must be excluded from recent-logs")
	assert.False(t, triageRowsContainMessage(logRows, infoMsg), "info log must be excluded from recent-logs")

	runtimeSec := requireTriageMap(t, data, "runtime")
	goroutines, ok := runtimeSec["num-goroutines"].(int)
	require.True(t, ok, "runtime.num-goroutines should be int, got %T", runtimeSec["num-goroutines"])
	assert.GreaterOrEqual(t, goroutines, 1)

	rawJSON, err := json.Marshal(data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rawJSON, &decoded))
	assertKebabCaseKeys(t, decoded, "")

	for _, pipeLine := range []string{
		"show system triage | json",
		"show system triage | yaml",
		"show system triage | table",
	} {
		_, format, parseErr := command.ProcessPipesDefaultFormatResult(pipeLine, "")
		require.Empty(t, parseErr, "pipe parse failed for %q", pipeLine)
		rendered, applyErr := format(string(rawJSON))
		assert.Empty(t, applyErr, "pipe render failed for %q", pipeLine)
		assert.NotEmpty(t, rendered, "pipe render empty for %q", pipeLine)
	}
}

// TestHandleShowSystemTriageUnavailableSources verifies that missing daemon
// handles or a failing doctor dispatch degrade only the affected sections with
// explicit available=false markers while returning the remaining sections.
//
// VALIDATES: AC-2, R-2, R-4 -- nil CommandContext and failing `show doctor`
// dispatch return StatusDone with available:false plus reason/error on
// unavailable sections instead of failing the whole command or inventing empty
// healthy state.
// PREVENTS: offline triage panicking or reporting empty plugins/reload/doctor
// as healthy, or a doctor failure hiding health, errors, crashes, and logs.
func TestHandleShowSystemTriageUnavailableSources(t *testing.T) {
	t.Run("nil context degrades doctor plugins and reload", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		resp, err := handleShowSystemTriage(nil, commandtest.Args())
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, plugin.StatusDone, resp.Status)

		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "Data should be plugin.Map, got %T", resp.Data)

		doctorSec := requireTriageMap(t, data, "doctor")
		assert.Equal(t, false, doctorSec["available"])
		assert.NotEmpty(t, doctorSec["error"])

		pluginsSec := requireTriageMap(t, data, "plugins")
		assert.Equal(t, false, pluginsSec["available"])
		assert.NotEmpty(t, pluginsSec["reason"])

		reloadSec := requireTriageMap(t, data, "reload")
		assert.Equal(t, false, reloadSec["available"])
		assert.NotEmpty(t, reloadSec["reason"])

		// Remaining sections stay populated even offline.
		assert.NotEmpty(t, requireTriageMap(t, data, "version")["go"])
		assert.NotEmpty(t, requireTriageMap(t, data, "health")["status"])
		assert.GreaterOrEqual(t, requireTriageMap(t, data, "runtime")["num-goroutines"], 1)
		assert.NotNil(t, requireTriageMap(t, data, "errors")["errors"])
		assert.NotNil(t, requireTriageMap(t, data, "warnings")["warnings"])
		assert.NotNil(t, requireTriageMap(t, data, "crashes")["crashes"])
		assert.NotNil(t, requireTriageMap(t, data, "recent-logs")["entries"])
	})

	t.Run("failing doctor dispatch isolates failure to doctor section", func(t *testing.T) {
		srv, err := pluginserver.NewServer(nil, nil)
		require.NoError(t, err)
		require.NoError(t, srv.Dispatcher().Register("show doctor", func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
			return &plugin.Response{Status: plugin.StatusError, Error: "doctor check timed out"}, nil
		}, "test failing show doctor"))
		srv.MarkReloadProcessed(false)

		ctx := &pluginserver.CommandContext{Server: srv}
		resp, err := handleShowSystemTriage(ctx, commandtest.Args())
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, plugin.StatusDone, resp.Status)

		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "Data should be plugin.Map, got %T", resp.Data)

		doctorSec := requireTriageMap(t, data, "doctor")
		assert.Equal(t, false, doctorSec["available"])
		assert.Contains(t, doctorSec["error"], "doctor check timed out")

		reloadSec := requireTriageMap(t, data, "reload")
		assert.Equal(t, true, reloadSec["available"])
		assert.Equal(t, uint64(1), reloadSec["generation"])
		assert.Equal(t, pluginserver.ReloadOutcomeFailed, reloadSec["last-outcome"])
	})
}

// TestHandleShowSystemTriageCapsAndTruncation verifies every list section
// enforces its cap, reports pre-truncation total-count and truncated flags at
// empty, exact-cap, and over-cap boundaries, and encodes empty lists as [].
//
// VALIDATES: AC-3, R-1 -- errors (32), warnings (64), crashes (16), and
// recent-logs (32) are bounded newest-first with accurate total-count and
// truncated markers and non-nil empty JSON arrays.
// PREVENTS: unbounded triage payloads overflowing MCP context windows or null
// JSON arrays when a list is empty.
func TestHandleShowSystemTriageCapsAndTruncation(t *testing.T) {
	t.Run("empty lists encode as JSON arrays and not truncated", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
		t.Cleanup(restoreCrashDir)

		resp, err := handleShowSystemTriage(nil, commandtest.Args())
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)

		rawJSON, err := json.Marshal(resp.Data)
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(rawJSON, &decoded))

		for _, tc := range []struct {
			section string
			listKey string
		}{
			{section: "errors", listKey: "errors"},
			{section: "warnings", listKey: "warnings"},
			{section: "crashes", listKey: "crashes"},
		} {
			sec, ok := decoded[tc.section].(map[string]any)
			require.True(t, ok, "%s should decode as object", tc.section)
			arr, ok := sec[tc.listKey].([]any)
			require.True(t, ok, "%s.%s must encode as JSON array [], got %T", tc.section, tc.listKey, sec[tc.listKey])
			assert.Empty(t, arr)
			assert.Equal(t, float64(0), sec["count"])
			assert.Equal(t, float64(0), sec["total-count"])
			assert.Equal(t, false, sec["truncated"])
		}
	})

	t.Run("exact cap is not truncated", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		crashDir := t.TempDir()
		restoreCrashDir := crashlog.SetCrashDirForTest(crashDir)
		t.Cleanup(restoreCrashDir)

		var tb textbuf.Buffer
		for i := range triageMaxErrors {
			subject := tb.Reset().Str("err-exact-").Int(int64(i)).String()
			report.RaiseError("bgp", "exact-err", subject, "exact error", nil)
		}
		for i := range triageMaxWarnings {
			subject := tb.Reset().Str("warn-exact-").Int(int64(i)).String()
			report.RaiseWarning("bgp", "exact-warn", subject, "exact warning", nil)
		}
		for i := range triageMaxCrashes {
			name := tb.Reset().Str("crash-20261127-").Int(int64(100000 + i)).Str(".log").String()
			require.NoError(t, os.WriteFile(filepath.Join(crashDir, name), []byte("panic"), 0o600))
		}

		resp, err := handleShowSystemTriage(nil, commandtest.Args())
		require.NoError(t, err)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)

		errorsSec := requireTriageMap(t, data, "errors")
		assert.Equal(t, triageMaxErrors, errorsSec["count"])
		assert.Equal(t, triageMaxErrors, errorsSec["total-count"])
		assert.Equal(t, false, errorsSec["truncated"])

		warningsSec := requireTriageMap(t, data, "warnings")
		assert.Equal(t, triageMaxWarnings, warningsSec["count"])
		assert.Equal(t, triageMaxWarnings, warningsSec["total-count"])
		assert.Equal(t, false, warningsSec["truncated"])

		crashesSec := requireTriageMap(t, data, "crashes")
		assert.Equal(t, triageMaxCrashes, crashesSec["count"])
		assert.Equal(t, triageMaxCrashes, crashesSec["total-count"])
		assert.Equal(t, false, crashesSec["truncated"])
	})

	t.Run("over cap truncates newest-first with full total-count", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		crashDir := t.TempDir()
		restoreCrashDir := crashlog.SetCrashDirForTest(crashDir)
		t.Cleanup(restoreCrashDir)

		var tb textbuf.Buffer
		totalErrors := triageMaxErrors + 3
		for i := range totalErrors {
			subject := tb.Reset().Str("err-over-").Int(int64(i)).String()
			report.RaiseError("bgp", "over-err", subject, "over error", nil)
		}

		totalWarnings := triageMaxWarnings + 3
		for i := range totalWarnings {
			subject := tb.Reset().Str("warn-over-").Int(int64(i)).String()
			report.RaiseWarning("bgp", "over-warn", subject, "over warning", nil)
		}

		totalCrashes := triageMaxCrashes + 2
		newestCrash := ""
		for i := range totalCrashes {
			name := tb.Reset().Str("crash-20261127-").Int(int64(200000 + i)).Str(".log").String()
			newestCrash = name
			require.NoError(t, os.WriteFile(filepath.Join(crashDir, name), []byte("panic"), 0o600))
		}

		log := slogutil.Logger("triage.test.caps")
		totalLogs := triageMaxLogs + 4
		newestLogMsg := ""
		for i := range totalLogs {
			newestLogMsg = tb.Reset().Str("triage-cap-log-").Int(int64(i)).String()
			if i%2 == 0 {
				log.Warn(newestLogMsg)
			} else {
				log.Error(newestLogMsg)
			}
		}

		resp, err := handleShowSystemTriage(nil, commandtest.Args())
		require.NoError(t, err)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)

		errorsSec := requireTriageMap(t, data, "errors")
		errs, ok := errorsSec["errors"].([]report.Issue)
		require.True(t, ok)
		assert.Len(t, errs, triageMaxErrors)
		assert.Equal(t, triageMaxErrors, errorsSec["count"])
		assert.Equal(t, totalErrors, errorsSec["total-count"])
		assert.Equal(t, true, errorsSec["truncated"])
		assert.Equal(t, tb.Reset().Str("err-over-").Int(int64(totalErrors-1)).String(), errs[0].Subject)

		warningsSec := requireTriageMap(t, data, "warnings")
		warns, ok := warningsSec["warnings"].([]report.Issue)
		require.True(t, ok)
		assert.Len(t, warns, triageMaxWarnings)
		assert.Equal(t, triageMaxWarnings, warningsSec["count"])
		assert.Equal(t, totalWarnings, warningsSec["total-count"])
		assert.Equal(t, true, warningsSec["truncated"])

		crashesSec := requireTriageMap(t, data, "crashes")
		crashes, ok := crashesSec["crashes"].([]map[string]any)
		require.True(t, ok)
		assert.Len(t, crashes, triageMaxCrashes)
		assert.Equal(t, triageMaxCrashes, crashesSec["count"])
		assert.Equal(t, totalCrashes, crashesSec["total-count"])
		assert.Equal(t, true, crashesSec["truncated"])
		assert.Equal(t, newestCrash, crashes[0]["name"])

		logsSec := requireTriageMap(t, data, "recent-logs")
		entries, ok := logsSec["entries"].([]map[string]any)
		require.True(t, ok)
		assert.Len(t, entries, triageMaxLogs)
		assert.Equal(t, triageMaxLogs, logsSec["count"])
		totalCount, ok := logsSec["total-count"].(int)
		require.True(t, ok)
		assert.GreaterOrEqual(t, totalCount, totalLogs)
		assert.Equal(t, true, logsSec["truncated"])
		assert.Equal(t, newestLogMsg, entries[0]["message"])
	})
}

// TestHandleShowSystemTriageSurfacesTaskguardPanic proves end-to-end in-process
// that a panic recovered by taskguard.Task.Contain appears in triage errors,
// crashes, and recent-logs under report.CodeInternalDefect.
//
// VALIDATES: AC-1, AC-5 -- a recovered task panic is immediately visible in
// show system triage errors, crashes, and recent-logs with doctor-internal-defect.
// PREVENTS: contained network-input panics staying invisible to live triage.
func TestHandleShowSystemTriageSurfacesTaskguardPanic(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restoreCrashDir)

	log := slogutil.Logger("triage.test.taskguard")
	guard := taskguard.New("bgp", log)
	task := guard.Task("read", "the session reconnects after backoff")

	var caught *taskguard.Panic
	func() {
		defer task.Contain(
			func() taskguard.Context {
				return taskguard.Context{Unit: "192.0.2.99", MessageType: "UPDATE"}
			},
			func(p *taskguard.Panic) {
				caught = p
			},
		)
		panic("triage taskguard boom")
	}()
	require.NotNil(t, caught, "taskguard must recover the panic")
	require.NotEmpty(t, caught.Report, "taskguard must write a recovered crash file")

	resp, err := handleShowSystemTriage(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be plugin.Map, got %T", resp.Data)

	errorsSec := requireTriageMap(t, data, "errors")
	errs, ok := errorsSec["errors"].([]report.Issue)
	require.True(t, ok, "errors.errors should be []report.Issue, got %T", errorsSec["errors"])
	foundErr := false
	for i := range errs {
		if errs[i].Code == report.CodeInternalDefect && errs[i].Source == "bgp" {
			foundErr = true
			assert.Contains(t, errs[i].Message, "triage taskguard boom")
			assert.Equal(t, "ze show crashes name "+caught.Report, errs[i].Fix)
			break
		}
	}
	assert.True(t, foundErr, "triage errors must contain the taskguard doctor-internal-defect issue")

	crashesSec := requireTriageMap(t, data, "crashes")
	crashes, ok := crashesSec["crashes"].([]map[string]any)
	require.True(t, ok, "crashes.crashes should be []map[string]any, got %T", crashesSec["crashes"])
	foundCrash := false
	for _, row := range crashes {
		if row["name"] == caught.Report {
			foundCrash = true
			assert.Equal(t, crashlog.KindRecovered, row["kind"])
			break
		}
	}
	assert.True(t, foundCrash, "triage crashes must include recovered crash file %q", caught.Report)

	logsSec := requireTriageMap(t, data, "recent-logs")
	entries, ok := logsSec["entries"].([]map[string]any)
	require.True(t, ok, "recent-logs.entries should be []map[string]any, got %T", logsSec["entries"])
	foundLog := false
	for _, row := range entries {
		if row["level"] != "error" {
			continue
		}
		attrs, ok := row["attributes"].(map[string]any)
		if !ok {
			continue
		}
		if attrs["code"] == report.CodeInternalDefect && attrs["panic"] == "triage taskguard boom" {
			foundLog = true
			assert.Equal(t, "192.0.2.99", attrs["unit"])
			break
		}
	}
	assert.True(t, foundLog, "triage recent-logs must include the ERROR entry carrying code=%s", report.CodeInternalDefect)
}

// requireTriageMap extracts a named nested map section from a triage response.
func requireTriageMap(t *testing.T, data plugin.Map, key string) map[string]any {
	t.Helper()
	raw, exists := data[key]
	require.True(t, exists, "missing triage section %q", key)
	sec, ok := raw.(map[string]any)
	require.True(t, ok, "triage section %q is %T, want map[string]any", key, raw)
	return sec
}

// ringContainsMessage reports whether any ring entry carries message.
func ringContainsMessage(entries []slogutil.LogEntry, message string) bool {
	for i := range entries {
		if entries[i].Message == message {
			return true
		}
	}
	return false
}

// triageRowsContainMessage reports whether any formatted triage log row carries
// message.
func triageRowsContainMessage(rows []map[string]any, message string) bool {
	for _, row := range rows {
		if row["message"] == message {
			return true
		}
	}
	return false
}

// assertKebabCaseKeys checks that every triage schema JSON object key under v
// is lowercase kebab-case (letters, digits, and hyphens only). Free-form log
// attributes and report detail maps are skipped after checking their own key.
func assertKebabCaseKeys(t *testing.T, v any, path string) {
	t.Helper()
	switch typed := v.(type) {
	case map[string]any:
		for k, child := range typed {
			assert.Equal(t, strings.ToLower(k), k, "key %q at %s must be lowercase", k, path)
			assert.False(t, strings.Contains(k, "_"), "key %q at %s must use kebab-case, not snake_case", k, path)
			if k == "attributes" {
				continue
			}
			if k == "detail" {
				continue
			}
			assertKebabCaseKeys(t, child, path+"."+k)
		}
	case []any:
		for _, item := range typed {
			assertKebabCaseKeys(t, item, path+"[]")
		}
	}
}
