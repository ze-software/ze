package runner

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/syslog"
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
			// An empty pattern compiles to a regex that matches every string,
			// so it would pass vacuously regardless of stderr. validateLogging
			// must reject it so a typo'd expect=stderr can never falsely pass.
			name:      "empty_pattern_rejected",
			patterns:  []string{""},
			stderr:    "any text here",
			wantError: true,
			errMsg:    "expect=stderr pattern is empty",
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
		{
			// An empty reject pattern matches every string, so it would fail
			// every test unconditionally. Reject it at validation so the
			// directive's intent ("this line must be absent") stays meaningful.
			name:      "empty_pattern_rejected",
			patterns:  []string{""},
			stderr:    "level=INFO",
			wantError: true,
			errMsg:    "reject=stderr pattern is empty",
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

// TestValidateLoggingSyslog verifies expect=syslog and reject=syslog assertions
// against captured syslog output.
//
// VALIDATES: expected patterns must appear and rejected patterns must NOT appear
//
//	in the syslog capture; empty patterns are rejected; and a missing
//	syslog server fails the test instead of passing vacuously.
//
// PREVENTS: the false pass where reject=syslog was parsed and counted as an
//
//	assertion but never actually evaluated, so a forbidden log line could
//	appear and the test would still report PASS.
func TestValidateLoggingSyslog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := syslog.New(0)
	require.NoError(t, srv.Start(ctx))
	t.Cleanup(func() { _ = srv.Close() })

	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", srv.Port()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = conn.Write([]byte("<14>level=ERROR subsystem=server msg=\"boom happened\""))
	require.NoError(t, err)

	// Wait until the message is captured so the assertions below are deterministic.
	require.Eventually(t, func() bool {
		return srv.Match("boom happened")
	}, 2*time.Second, 5*time.Millisecond, "expected syslog message to arrive")

	r := &Runner{}

	t.Run("expect_present_passes", func(t *testing.T) {
		rec := &Record{ExpectSyslog: []string{"boom happened"}}
		require.NoError(t, r.validateLogging(rec, "", srv))
	})

	t.Run("expect_absent_fails", func(t *testing.T) {
		rec := &Record{ExpectSyslog: []string{"never logged"}}
		err := r.validateLogging(rec, "", srv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expect=syslog pattern not found")
	})

	t.Run("reject_absent_passes", func(t *testing.T) {
		rec := &Record{RejectSyslog: []string{"never logged"}}
		require.NoError(t, r.validateLogging(rec, "", srv))
	})

	t.Run("reject_present_fails", func(t *testing.T) {
		// The forbidden line IS in the capture: this must fail. Before the fix
		// RejectSyslog was never checked, so this returned nil (false pass).
		rec := &Record{RejectSyslog: []string{"boom happened"}}
		err := r.validateLogging(rec, "", srv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reject=syslog pattern found")
	})

	t.Run("reject_invalid_regex_fails_closed", func(t *testing.T) {
		// A bad regex must error rather than fail open via Server.Match==false.
		rec := &Record{RejectSyslog: []string{"(unclosed"}}
		err := r.validateLogging(rec, "", srv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reject=syslog pattern")
	})

	t.Run("empty_reject_pattern_rejected", func(t *testing.T) {
		rec := &Record{RejectSyslog: []string{""}}
		err := r.validateLogging(rec, "", srv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reject=syslog pattern is empty")
	})

	t.Run("nil_server_with_assertion_fails", func(t *testing.T) {
		// Syslog assertion declared but no server wired up: fail loudly rather
		// than skip the check and pass vacuously.
		rec := &Record{RejectSyslog: []string{"boom happened"}}
		err := r.validateLogging(rec, "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no syslog server was started")
	})
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
			// A syslog assertion with no server wired up must fail, not pass:
			// the harness starts the capture server whenever expect/reject=syslog
			// is present, so a nil server here means the assertion could never
			// have been evaluated. Failing closed prevents the vacuous pass.
			name:       "no_syslog_server",
			patterns:   []string{"subsystem=server"},
			syslogMsgs: nil, // nil means no syslog server
			wantError:  true,
			errMsg:     "no syslog server was started",
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

// TestValidateJSONJsonOnlyUnsupportedFamily verifies that a json-only message
// expectation (no wire-level expect=bgp:hex=) whose route content is in a family
// the NLRI matcher cannot extract fails loudly instead of being silently skipped.
//
// VALIDATES: json-only expectations in unsupported families fail; wire-hex-backed
//
//	and genuinely content-free expectations still skip without error.
//
// PREVENTS: the latent false pass where the only assertion on a message is a JSON
//
//	body the content matcher cannot key on, so the comparison is skipped and the
//	message is validated by nothing.
func TestValidateJSONJsonOnlyUnsupportedFamily(t *testing.T) {
	r := &Runner{}
	const evpnJSON = `{"l2vpn/evpn":[{"action":"add","nlri":{"string":"rd 1:1 mac aa:bb:cc:dd:ee:ff"}}]}`

	t.Run("json_only_unsupported_family_fails", func(t *testing.T) {
		rec := &Record{Messages: []messageExpect{{Index: 101, JSON: evpnJSON}}}
		err := r.validateJSON(rec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot extract")
	})

	t.Run("wire_hex_backed_skips", func(t *testing.T) {
		// Same content, but an exact wire-byte check backs the message: the JSON
		// match is supplementary, so skipping it is correct (no error).
		rec := &Record{Messages: []messageExpect{{Index: 101, RawHex: "FFFF", JSON: evpnJSON}}}
		require.NoError(t, r.validateJSON(rec))
	})

	t.Run("content_free_json_skips", func(t *testing.T) {
		// Attribute-only JSON carries no route entries (EOR-like): nothing to
		// match, and the guard must not trip on it.
		rec := &Record{Messages: []messageExpect{{Index: 101, JSON: `{"origin":"igp","as-path":[65001]}`}}}
		require.NoError(t, r.validateJSON(rec))
	})
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
