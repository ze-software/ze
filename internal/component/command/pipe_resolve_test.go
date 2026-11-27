// Design: docs/architecture/doctor-and-health-checks.md -- reverse DNS pipe failure reporting and rate limiting.
// Related: pipe_resolve.go -- ReverseLookup, CodeDoctorResolvePTRFailed.

package command

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
)

type stubPTRResolver struct {
	records []string
	err     error
}

func (s stubPTRResolver) ResolvePTR(string) ([]string, error) {
	return s.records, s.err
}

func countReportErrorsByCode(code string) (int, *report.Issue) {
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

// TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce verifies that a DNS
// resolver error in ReverseLookup keeps returning "" for upstream rows, logs a
// coded Warn and raises a report-bus event at most once per 1-minute window,
// and stays silent on NXDOMAIN (nil, nil) or success.
//
// VALIDATES: AC-2 -- ReverseLookup reports DNS failures via doctor-resolve-ptr-failed once per minute.
// PREVENTS: DNS resolver outages hiding behind a Debug log or flooding logs on large | resolve tables.
func TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce(t *testing.T) {
	assert.Equal(t, "doctor-resolve-ptr-failed", CodeDoctorResolvePTRFailed)

	t.Run("resolver failure warns and raises once per minute", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		origLog := resolveLog
		var buf bytes.Buffer
		resolveLog = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Cleanup(func() { resolveLog = origLog })

		SetPTRResolver(stubPTRResolver{err: errors.New("no DNS server configured")})
		t.Cleanup(func() { SetPTRResolver(nil) })

		assert.Empty(t, ReverseLookup("192.0.2.10"))
		assert.Empty(t, ReverseLookup("192.0.2.11"))
		assert.Empty(t, ReverseLookup("192.0.2.12"))

		logged := buf.String()
		assert.Contains(t, logged, "level=WARN")
		assert.Equal(t, 1, strings.Count(logged, "level=WARN"), "expected rate-limited single Warn per minute, got:\n"+logged)
		assert.Contains(t, logged, "code=doctor-resolve-ptr-failed")
		assert.Contains(t, logged, `fix="ze show system dns"`)
		assert.Contains(t, logged, "no DNS server configured")

		count, issue := countReportErrorsByCode(CodeDoctorResolvePTRFailed)
		require.Equal(t, 1, count)
		require.NotNil(t, issue)
		assert.Equal(t, "ze show system dns", issue.Fix)
		assert.NotEmpty(t, issue.Message)
	})

	t.Run("nxdomain stays silent", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		origLog := resolveLog
		var buf bytes.Buffer
		resolveLog = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Cleanup(func() { resolveLog = origLog })

		SetPTRResolver(stubPTRResolver{records: nil, err: nil})
		t.Cleanup(func() { SetPTRResolver(nil) })

		assert.Empty(t, ReverseLookup("192.0.2.20"))
		assert.Empty(t, buf.String())
		count, _ := countReportErrorsByCode(CodeDoctorResolvePTRFailed)
		assert.Zero(t, count)
	})

	t.Run("successful lookup returns trimmed name silently", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		origLog := resolveLog
		var buf bytes.Buffer
		resolveLog = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Cleanup(func() { resolveLog = origLog })

		SetPTRResolver(stubPTRResolver{records: []string{"router1.example.net."}})
		t.Cleanup(func() { SetPTRResolver(nil) })

		assert.Equal(t, "router1.example.net", ReverseLookup("192.0.2.30"))
		assert.Empty(t, buf.String())
		count, _ := countReportErrorsByCode(CodeDoctorResolvePTRFailed)
		assert.Zero(t, count)
	})
}
