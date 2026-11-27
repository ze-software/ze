// Design: docs/architecture/doctor-and-health-checks.md -- web editor idle eviction discard failure reporting.
// Related: editor.go -- EditorManager.evictInactive; doctor_codes.go -- codeDoctorWebEditorEvictDiscardFailed.

package web

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

type nilDiscardEditor struct {
	recoveryBlockedEditor
}

func (nilDiscardEditor) Discard() error { return nil }

func countWebReportErrorsByCode(code string) (int, *report.Issue) {
	errs := report.Errors(0)
	var count int
	var first *report.Issue
	for i := range errs {
		if errs[i].Code == code {
			count++
			if first == nil {
				issue := errs[i]
				first = &issue
			}
		}
	}
	return count, first
}

// TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent verifies that an
// error from Discard during EditorManager.evictInactive still removes the idle
// session from the cache, logs a coded Warn with fix "ze show doctor", raises
// a rate-limited secret-free event on the report bus, and stays silent when
// Discard returns nil.
//
// VALIDATES: AC-4 -- evictInactive reports discard failures under doctor-web-editor-evict-discard-failed.
// PREVENTS: idle eviction discard errors hiding behind a Debug line or leaking secret text onto the report bus.
func TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent(t *testing.T) {
	meta := diagnostic.Lookup(codeDoctorWebEditorEvictDiscardFailed)
	require.NotNil(t, meta)
	assert.Equal(t, "doctor-web-editor-evict-discard-failed", meta.Code)
	assert.NotEmpty(t, meta.Title)
	assert.NotEmpty(t, meta.Description)
	assert.NotEmpty(t, meta.Examples)
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorWebEditorEvictDiscardFailed))

	t.Run("discard failure frees slot, warns, and raises secret-free doctor event", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		origLog := serverLogger
		var buf bytes.Buffer
		serverLogger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Cleanup(func() { serverLogger = origLog })

		const leakedToken = "evict-discard-raw-secret-token-999" //nolint:gosec // fixture value, not a credential
		mgr := NewEditorManager(nil, "", nil, nil, nil)
		mgr.maxSessions = 1
		mgr.idleTimeout = time.Hour

		idleAt := time.Now().Add(-2 * time.Hour)
		mgr.sessions["alice"] = &userSession{
			editor:       blockedRecoveryEditor(errors.New("draft discard failed: " + leakedToken)),
			lastActivity: idleAt,
		}

		mgr.mu.Lock()
		mgr.evictInactive()
		_, stillCached := mgr.sessions["alice"]
		mgr.mu.Unlock()

		assert.False(t, stillCached, "idle session must be evicted even when Discard fails")

		logged := buf.String()
		assert.Contains(t, logged, "level=WARN")
		assert.Contains(t, logged, "code=doctor-web-editor-evict-discard-failed")
		assert.Contains(t, logged, `fix="ze show doctor"`)
		assert.Contains(t, logged, "user=alice")
		assert.Contains(t, logged, "draft discard failed")

		count, issue := countWebReportErrorsByCode(codeDoctorWebEditorEvictDiscardFailed)
		require.Equal(t, 1, count)
		require.NotNil(t, issue)
		assert.Equal(t, "ze show doctor", issue.Fix)
		assert.NotEmpty(t, issue.Message)
		assert.NotContains(t, issue.Message, leakedToken)

		// A second eviction failure inside the 1-minute window frees its slot
		// without raising a duplicate event on the report bus.
		mgr.sessions["bob"] = &userSession{
			editor:       blockedRecoveryEditor(errors.New("second discard failure")),
			lastActivity: idleAt,
		}
		mgr.mu.Lock()
		mgr.evictInactive()
		_, bobCached := mgr.sessions["bob"]
		mgr.mu.Unlock()

		assert.False(t, bobCached)
		countAfter, _ := countWebReportErrorsByCode(codeDoctorWebEditorEvictDiscardFailed)
		assert.Equal(t, 1, countAfter)
	})

	t.Run("nil discard frees slot and stays silent", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		origLog := serverLogger
		var buf bytes.Buffer
		serverLogger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Cleanup(func() { serverLogger = origLog })

		mgr := NewEditorManager(nil, "", nil, nil, nil)
		mgr.idleTimeout = time.Hour
		mgr.sessions["clean-user"] = &userSession{
			editor:       nilDiscardEditor{},
			lastActivity: time.Now().Add(-2 * time.Hour),
		}

		mgr.mu.Lock()
		mgr.evictInactive()
		_, stillCached := mgr.sessions["clean-user"]
		mgr.mu.Unlock()

		assert.False(t, stillCached)
		assert.Empty(t, buf.String())
		count, _ := countWebReportErrorsByCode(codeDoctorWebEditorEvictDiscardFailed)
		assert.Zero(t, count)
	})
}
