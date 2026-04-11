package runner

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/test/syslog"
)

// TestValidateLoggingExpectStderr verifies expect=stderr pattern matching.
//
// VALIDATES: Expected patterns are found in stderr output.
// PREVENTS: False positives/negatives in stderr log verification.
func TestValidateLoggingExpectStderr(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		stderr    string
		wantError bool
		errMsg    string
	}{
		{
			name:      "pattern_found",
			patterns:  []string{"subsystem=server"},
			stderr:    "level=INFO subsystem=server msg=test",
			wantError: false,
		},
		{
			name:      "pattern_not_found",
			patterns:  []string{"subsystem=server"},
			stderr:    "level=INFO subsystem=filter msg=test",
			wantError: true,
			errMsg:    "expect=stderr pattern not found: subsystem=server",
		},
		{
			name:      "regex_pattern_found",
			patterns:  []string{"level=(INFO|DEBUG)"},
			stderr:    "level=DEBUG subsystem=server",
			wantError: false,
		},
		{
			name:      "multiple_patterns_all_found",
			patterns:  []string{"subsystem=server", "level=INFO"},
			stderr:    "level=INFO subsystem=server msg=test",
			wantError: false,
		},
		{
			name:      "multiple_patterns_one_missing",
			patterns:  []string{"subsystem=server", "level=DEBUG"},
			stderr:    "level=INFO subsystem=server msg=test",
			wantError: true,
			errMsg:    "expect=stderr pattern not found: level=DEBUG",
		},
		{
			name:      "invalid_regex",
			patterns:  []string{"[invalid"},
			stderr:    "level=INFO",
			wantError: true,
			errMsg:    "invalid expect=stderr pattern",
		},
		{
			name:      "empty_pattern_matches_anything",
			patterns:  []string{""},
			stderr:    "any text here",
			wantError: false,
		},
		{
			name:      "empty_stderr_no_patterns",
			patterns:  nil,
			stderr:    "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{}
			rec := &Record{ExpectStderr: tt.patterns}

			err := r.validateLogging(rec, tt.stderr, nil)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateLoggingRejectStderr verifies reject=stderr pattern matching.
//
// VALIDATES: Rejected patterns cause failure when found in stderr.
// PREVENTS: Unwanted log messages going undetected.
func TestValidateLoggingRejectStderr(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		stderr    string
		wantError bool
		errMsg    string
	}{
		{
			name:      "pattern_not_found_ok",
			patterns:  []string{"level=ERROR"},
			stderr:    "level=INFO subsystem=server",
			wantError: false,
		},
		{
			name:      "pattern_found_fail",
			patterns:  []string{"level=ERROR"},
			stderr:    "level=ERROR subsystem=server",
			wantError: true,
			errMsg:    "reject=stderr pattern found: level=ERROR",
		},
		{
			name:      "regex_pattern_found",
			patterns:  []string{"error.*fatal"},
			stderr:    "error: something fatal happened",
			wantError: true,
			errMsg:    "reject=stderr pattern found: error.*fatal",
		},
		{
			name:      "invalid_regex",
			patterns:  []string{"(unclosed"},
			stderr:    "level=INFO",
			wantError: true,
			errMsg:    "invalid reject=stderr pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{}
			rec := &Record{RejectStderr: tt.patterns}

			err := r.validateLogging(rec, tt.stderr, nil)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateLoggingObserverFailSentinel verifies the implicit
// ZE-OBSERVER-FAIL sentinel check forces the test to fail regardless of
// explicit expect/reject directives.
//
// VALIDATES: ze_api.runtime_fail() output in ze's relayed stderr fails the
//
//	test, closing the silent-false-positive hole where a Python
//	observer's sys.exit(1) never reached the runner.
//
// PREVENTS: regression to the "observer exit code ignored" state documented
//
//	in the dest-1 handover.
func TestValidateLoggingObserverFailSentinel(t *testing.T) {
	tests := []struct {
		name      string
		stderr    string
		wantError bool
		errSubstr string
	}{
		{
			name:      "sentinel_on_dedicated_line",
			stderr:    "level=INFO subsystem=server msg=ok\ntime=runtime level=ERROR msg=\"ZE-OBSERVER-FAIL: route not found\" subsystem=test.observer\n",
			wantError: true,
			errSubstr: "ZE-OBSERVER-FAIL: route not found",
		},
		{
			name:      "sentinel_alone",
			stderr:    "ZE-OBSERVER-FAIL: any reason\n",
			wantError: true,
			errSubstr: "ZE-OBSERVER-FAIL: any reason",
		},
		{
			name:      "no_sentinel",
			stderr:    "level=INFO subsystem=server msg=ok\nlevel=WARN subsystem=observer msg=benign\n",
			wantError: false,
		},
		{
			name:      "empty_stderr",
			stderr:    "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{}
			rec := &Record{}

			err := r.validateLogging(rec, tt.stderr, nil)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.Contains(t, err.Error(), "observer reported runtime failure")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestExtractObserverFailLine verifies the helper isolates the sentinel line
// from multi-line stderr output so the failure message points at the exact
// failing line instead of dumping the whole buffer.
func TestExtractObserverFailLine(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{
			name:   "sentinel_mid_buffer",
			stderr: "level=INFO msg=a\ntime=x level=ERROR msg=\"ZE-OBSERVER-FAIL: bad\" subsystem=test.observer\nlevel=INFO msg=b\n",
			want:   `time=x level=ERROR msg="ZE-OBSERVER-FAIL: bad" subsystem=test.observer`,
		},
		{
			name:   "sentinel_first_line",
			stderr: "ZE-OBSERVER-FAIL: first\nsecond\n",
			want:   "ZE-OBSERVER-FAIL: first",
		},
		{
			name:   "sentinel_last_line_no_newline",
			stderr: "level=INFO msg=a\nZE-OBSERVER-FAIL: last",
			want:   "ZE-OBSERVER-FAIL: last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := indexSentinel(tt.stderr)
			require.GreaterOrEqual(t, idx, 0)
			got := extractObserverFailLine(tt.stderr, idx)
			assert.Equal(t, tt.want, got)
		})
	}
}

// indexSentinel is a tiny test helper that mirrors the strings.Index call
// used by validateLogging. Defined locally to avoid exposing yet another
// runner symbol for testing.
func indexSentinel(stderr string) int {
	const sentinel = "ZE-OBSERVER-FAIL"
	for i := 0; i+len(sentinel) <= len(stderr); i++ {
		if stderr[i:i+len(sentinel)] == sentinel {
			return i
		}
	}
	return -1
}

// TestValidateLoggingExpectSyslog verifies expect=syslog pattern matching.
//
// VALIDATES: Expected patterns are found in syslog messages.
// PREVENTS: Syslog backend tests silently failing.
func TestValidateLoggingExpectSyslog(t *testing.T) {
	tests := []struct {
		name       string
		patterns   []string
		syslogMsgs []string
		wantError  bool
		errMsg     string
	}{
		{
			name:       "pattern_found",
			patterns:   []string{"subsystem=server"},
			syslogMsgs: []string{"<14>level=INFO subsystem=server msg=test"},
			wantError:  false,
		},
		{
			name:       "pattern_not_found",
			patterns:   []string{"subsystem=server"},
			syslogMsgs: []string{"<14>level=INFO subsystem=filter msg=test"},
			wantError:  true,
			errMsg:     "expect=syslog pattern not found: subsystem=server",
		},
		{
			name:       "multiple_patterns_all_found",
			patterns:   []string{"subsystem=server", "level=INFO"},
			syslogMsgs: []string{"<14>level=INFO subsystem=server msg=test"},
			wantError:  false,
		},
		{
			name:       "pattern_in_syslog_priority_header",
			patterns:   []string{"<14>"},
			syslogMsgs: []string{"<14>level=INFO msg=test"},
			wantError:  false,
		},
		{
			name:       "no_syslog_server",
			patterns:   []string{"subsystem=server"},
			syslogMsgs: nil,   // nil means no syslog server
			wantError:  false, // Should pass - no server to check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{}
			rec := &Record{ExpectSyslog: tt.patterns}

			var syslogSrv *syslog.Server
			if tt.syslogMsgs != nil {
				// Create a syslog server and inject messages
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				syslogSrv = syslog.New(0)
				require.NoError(t, syslogSrv.Start(ctx))
				t.Cleanup(func() { _ = syslogSrv.Close() })

				// Send test messages to syslog server
				for _, msg := range tt.syslogMsgs {
					sendUDPMessage(t, syslogSrv.Port(), msg)
				}
				// Wait for messages to be received
				require.Eventually(t, func() bool {
					return len(syslogSrv.Messages()) >= len(tt.syslogMsgs)
				}, 2*time.Second, time.Millisecond)
			}

			err := r.validateLogging(rec, "", syslogSrv)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateLoggingCombined verifies combined expect/reject patterns.
//
// VALIDATES: All pattern types work together correctly.
// PREVENTS: Interaction bugs between different pattern types.
func TestValidateLoggingCombined(t *testing.T) {
	r := &Runner{}
	rec := &Record{
		ExpectStderr: []string{"subsystem=server"},
		RejectStderr: []string{"level=ERROR"},
	}

	// Both conditions satisfied
	err := r.validateLogging(rec, "level=INFO subsystem=server", nil)
	require.NoError(t, err)

	// Expect passes but reject fails
	err = r.validateLogging(rec, "level=ERROR subsystem=server", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reject=stderr pattern found")
}

// sendUDPMessage sends a UDP message to localhost:port.
func sendUDPMessage(t *testing.T, port int, msg string) {
	t.Helper()
	ctx := context.Background()
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = conn.Write([]byte(msg))
	require.NoError(t, err)
}
