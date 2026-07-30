package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCIExpectBGP verifies parsing of expect:bgp lines.
//
// VALIDATES: expect=bgp:conn=N:seq=N:hex=... is parsed correctly.
// PREVENTS: BGP message expectations not being captured.
func TestParseCIExpectBGP(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02
expect=bgp:conn=6:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001705`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	// Should have 3 messages, including a connection number above the old four-connection limit.
	assert.Len(t, rec.Messages, 3, "should have 3 messages")

	// First message: conn=1, seq=1 → index 101
	msg1 := rec.getMessage(101)
	require.NotNil(t, msg1, "message 101 should exist")
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304", msg1.RawHex)

	// Second message: conn=1, seq=2 → index 102
	msg2 := rec.getMessage(102)
	require.NotNil(t, msg2, "message 102 should exist")
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02", msg2.RawHex)

	// Sixth connection: conn=6, seq=1 → index 601.
	msg3 := rec.getMessage(601)
	require.NotNil(t, msg3, "message 601 should exist")
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001705", msg3.RawHex)
}

// TestParseCIExpectJSON verifies parsing of expect:json lines.
//
// VALIDATES: expect=json:conn=N:seq=N:json={...} is parsed and linked to same seq.
// PREVENTS: JSON validation not being associated with correct message.
func TestParseCIExpectJSON(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=json:conn=1:seq=1:json={"type":"keepalive"}`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	// Should have 1 message with both hex and json
	assert.Len(t, rec.Messages, 1, "should have 1 message")

	msg := rec.getMessage(101)
	require.NotNil(t, msg)
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304", msg.RawHex)
	assert.Equal(t, `{"type":"keepalive"}`, msg.JSON)
}

// TestParseCIExpectFile verifies parsing of expect:file lines.
//
// VALIDATES: expect=file:path and expect=file:glob checks are captured for post-run validation.
// PREVENTS: daemon filesystem assertions requiring shell scripts in .ci tests.
func TestParseCIExpectFile(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=file:path=meta/config/active:exists=true
expect=file:path=meta/config/candidate:absent=true
expect=file:glob=rollback/ze-bgp-*.conf:count=2
expect=file:glob=rollback/ze-bgp-*.conf:contains=router-id 1.2.3.4`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)
	require.Len(t, rec.FileChecks, 4)
	assert.Equal(t, "meta/config/active", rec.FileChecks[0].Path)
	assert.True(t, rec.FileChecks[0].Exists)
	assert.Equal(t, "meta/config/candidate", rec.FileChecks[1].Path)
	assert.True(t, rec.FileChecks[1].Absent)
	assert.Equal(t, "rollback/ze-bgp-*.conf", rec.FileChecks[2].Glob)
	require.NotNil(t, rec.FileChecks[2].Count)
	assert.Equal(t, 2, *rec.FileChecks[2].Count)
	assert.Equal(t, "router-id 1.2.3.4", rec.FileChecks[3].Contains)
}

// TestParseCIOptionEnv verifies parsing of option:env lines.
//
// VALIDATES: option=env:var=X:value=Y is parsed correctly.
// PREVENTS: Environment variables not being set for tests.
func TestParseCIOptionEnv(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
option=env:var=ze.log.bgp.server:value=debug
option=env:var=ze.log.bgp.filter:value=info
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	// Should have 2 env vars in KEY=VALUE format
	assert.Equal(t, []string{"ze.log.bgp.server=debug", "ze.log.bgp.filter=info"}, rec.EnvVars)
}

// TestParseCIOptionFile verifies parsing of option:file lines.
//
// VALIDATES: option=file:path=X is parsed correctly.
// PREVENTS: Config file not being loaded.
func TestParseCIOptionFile(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "myconfig.conf")

	ciContent := `option=file:path=myconfig.conf
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	assert.Equal(t, filepath.Join(tmpDir, "myconfig.conf"), rec.ConfigFile)
}

// TestParseCIMultiConn verifies parsing of multi-connection tests.
//
// VALIDATES: conn=1 and conn=2 are tracked separately with proper indexing.
// PREVENTS: Multi-connection tests having wrong message ordering.
func TestParseCIMultiConn(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=bgp:conn=1:seq=1:hex=AAAA
expect=bgp:conn=2:seq=1:hex=BBBB
expect=bgp:conn=1:seq=2:hex=CCCC
expect=bgp:conn=2:seq=2:hex=DDDD`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	// Should have 4 messages with proper conn/seq indexing
	// conn=1:seq=1 → index 101, conn=1:seq=2 → index 102
	// conn=2:seq=1 → index 201, conn=2:seq=2 → index 202
	assert.Len(t, rec.Messages, 4, "should have 4 messages")

	assert.Equal(t, "AAAA", rec.getMessage(101).RawHex) // conn=1:seq=1
	assert.Equal(t, "CCCC", rec.getMessage(102).RawHex) // conn=1:seq=2
	assert.Equal(t, "BBBB", rec.getMessage(201).RawHex) // conn=2:seq=1
	assert.Equal(t, "DDDD", rec.getMessage(202).RawHex) // conn=2:seq=2
}

// TestParseCISameSeq verifies that same seq number means unordered acceptance.
//
// VALIDATES: Multiple expects with same conn:seq are stored for unordered matching.
// PREVENTS: Strict ordering when order is unknown.
func TestParseCISameSeq(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	// Two messages with same seq - order unknown, accept either first
	ciContent := `option=file:path=test.conf
expect=bgp:conn=1:seq=1:hex=AAAA
expect=bgp:conn=1:seq=1:hex=BBBB`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	// Both should be stored - implementation decides how to handle unordered
	// For now, we just verify both are captured in Expects
	assert.Len(t, rec.Expects, 2, "should have 2 expects for unordered matching")
}

// TestParseCIActionNotification verifies parsing of action:notification lines.
//
// VALIDATES: action=notification:conn=N:seq=N:text=X is parsed correctly.
// PREVENTS: Notification actions not being recognized.
func TestParseCIActionNotification(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
action=notification:conn=1:seq=2:text=session ending`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	// Notification should be in Expects for testpeer to process.
	found := slices.Contains(rec.Expects, "action=notification:conn=1:seq=2:text=session ending")
	assert.True(t, found, "notification action should be in Expects")
}

// TestParseCICmdAPI verifies parsing of cmd:api documentation lines.
//
// VALIDATES: cmd=api:conn=N:seq=N:text=X is parsed and stored.
// PREVENTS: API command documentation not being captured.
func TestParseCICmdAPI(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	msg := rec.getMessage(101)
	require.NotNil(t, msg)
	assert.Equal(t, "update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24", msg.Cmd)
}

// TestParseCIRejectSyslog verifies parsing of reject:syslog lines.
//
// VALIDATES: reject=syslog:pattern=X is parsed correctly.
// PREVENTS: Syslog rejection patterns not being captured.
func TestParseCIRejectSyslog(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
reject=syslog:pattern=fatal
reject=syslog:pattern=panic
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)

	assert.Equal(t, []string{"fatal", "panic"}, rec.RejectSyslog)
}

// TestParseCIEmptyPatternRejected verifies that an empty pattern= value on any
// expect/reject stderr or syslog directive is rejected at parse time.
//
// VALIDATES: empty patterns fail parsing with a clear message.
// PREVENTS: the false pass where `expect=stderr:pattern=` compiles to a regex
//
//	that matches every string, so the assertion passes vacuously and a typo
//	silently disables a real check.
func TestParseCIEmptyPatternRejected(t *testing.T) {
	cases := []struct {
		name      string
		directive string
		errSubstr string
	}{
		{"expect_stderr", "expect=stderr:pattern=", "expect=stderr:pattern= must not be empty"},
		{"expect_stdout", "expect=stdout:pattern=", "expect=stdout:pattern= must not be empty"},
		{"expect_stdout_invalid", "expect=stdout:pattern=[invalid", "invalid expect=stdout pattern"},
		{"expect_syslog", "expect=syslog:pattern=", "expect=syslog:pattern= must not be empty"},
		{"reject_stderr", "reject=stderr:pattern=", "reject=stderr:pattern= must not be empty"},
		{"reject_stdout", "reject=stdout:pattern=", "reject=stdout:pattern= must not be empty"},
		{"reject_stdout_invalid", "reject=stdout:pattern=[invalid", "invalid reject=stdout pattern"},
		{"reject_syslog", "reject=syslog:pattern=", "reject=syslog:pattern= must not be empty"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ResetNickCounter()
			tmpDir := t.TempDir()
			ciFile := filepath.Join(tmpDir, "test.ci")
			confFile := filepath.Join(tmpDir, "test.conf")

			ciContent := "option=file:path=test.conf\n" + tc.directive +
				"\nexpect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"

			require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
			require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

			et := NewEncodingTests(tmpDir)
			_, err := et.parseAndAdd(ciFile)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errSubstr)
		})
	}
}

// TestParseCIMissingConn verifies error when conn is missing from expect:bgp.
//
// VALIDATES: Missing conn field produces error.
// PREVENTS: Ambiguous message targeting.
func TestParseCIMissingConn(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=bgp:seq=1:hex=FFFFFFFF`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing conn")
}

// TestParseCIMissingSeq verifies error when seq is missing from expect:bgp.
//
// VALIDATES: Missing seq field produces error.
// PREVENTS: Unordered messages when ordering is required.
func TestParseCIMissingSeq(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
expect=bgp:conn=1:hex=FFFFFFFF`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing seq")
}

// TestNextMarker verifies the nextMarker helper finds the earliest marker.
//
// VALIDATES: nextMarker returns the position of the earliest marker after offset.
// PREVENTS: Incorrect value boundary detection in marker-based parsing.
func TestNextMarker(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		offset  int
		markers []string
		want    int
	}{
		{
			name:    "no_markers",
			line:    "hello world",
			offset:  0,
			markers: []string{":foo=", ":bar="},
			want:    11, // len(line)
		},
		{
			name:    "single_marker",
			line:    "abc:foo=123",
			offset:  0,
			markers: []string{":foo="},
			want:    3,
		},
		{
			name:    "earliest_of_two",
			line:    "abc:bar=x:foo=y",
			offset:  0,
			markers: []string{":foo=", ":bar="},
			want:    3, // :bar= at 3 is earlier than :foo= at 9
		},
		{
			name:    "offset_skips_earlier",
			line:    "abc:bar=x:foo=y",
			offset:  4,
			markers: []string{":foo=", ":bar="},
			want:    9, // :foo= at 9 (skipped :bar= at 3)
		},
		{
			name:    "empty_markers",
			line:    "abc:foo=123",
			offset:  0,
			markers: nil,
			want:    11, // len(line)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextMarker(tt.line, tt.offset, tt.markers...)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseCmdExec verifies marker-based parsing of cmd=background/foreground lines.
//
// VALIDATES: parseCmdExec correctly extracts seq, exec (with colons), stdin,
// timeout, and the per-command exit= assertion, in any marker order, rejecting
// exit codes outside 0..255.
// PREVENTS: Colons in exec values being misinterpreted as field delimiters, and
// a malformed exit= being silently ignored -- which would leave the command
// unasserted while the test still looked green.
func TestParseCmdExec(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		line    string
		want    RunCommand
		wantErr string
	}{
		{
			name: "basic_exec",
			mode: "background",
			line: "cmd=background:seq=1:exec=ze-chaos --quiet",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "ze-chaos --quiet"},
		},
		{
			name: "exec_with_colon_port",
			mode: "background",
			line: "cmd=background:seq=1:exec=ze-chaos --web :8000 --quiet",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "ze-chaos --web :8000 --quiet"},
		},
		{
			name: "exec_with_stdin",
			mode: "background",
			line: "cmd=background:seq=1:exec=ze-peer --port 1790:stdin=peer",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "ze-peer --port 1790", Stdin: "peer"},
		},
		{
			name: "foreground_with_timeout",
			mode: "foreground",
			line: "cmd=foreground:seq=2:exec=ze server -:stdin=config:timeout=10s",
			want: RunCommand{Mode: "foreground", Seq: 2, Exec: "ze server -", Stdin: "config", Timeout: "10s"},
		},
		{
			name: "exec_with_colon_and_stdin",
			mode: "background",
			line: "cmd=background:seq=1:exec=ze-chaos --in-process --web :$PORT --duration 10s:stdin=data",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "ze-chaos --in-process --web :$PORT --duration 10s", Stdin: "data"},
		},
		{
			name:    "missing_seq",
			mode:    "background",
			line:    "cmd=background:exec=something",
			wantErr: "missing seq=",
		},
		{
			name:    "missing_exec",
			mode:    "background",
			line:    "cmd=background:seq=1",
			wantErr: "missing exec=",
		},
		{
			name:    "invalid_seq_zero",
			mode:    "background",
			line:    "cmd=background:seq=0:exec=something",
			wantErr: "invalid seq=",
		},
		{
			name: "exit_code",
			mode: "foreground",
			line: "cmd=foreground:seq=1:exec=ze config validate -:exit=1",
			want: RunCommand{Mode: "foreground", Seq: 1, Exec: "ze config validate -", ExitCode: new(1)},
		},
		{
			name: "exit_code_zero",
			mode: "foreground",
			line: "cmd=foreground:seq=1:exec=ze doctor:exit=0",
			want: RunCommand{Mode: "foreground", Seq: 1, Exec: "ze doctor", ExitCode: new(0)},
		},
		{
			name: "exit_code_with_stdin_and_timeout",
			mode: "foreground",
			line: "cmd=foreground:seq=2:exec=ze config validate -:stdin=cfg:timeout=60s:exit=1",
			want: RunCommand{Mode: "foreground", Seq: 2, Exec: "ze config validate -", Stdin: "cfg", Timeout: "60s", ExitCode: new(1)},
		},
		{
			// Marker order must not matter: exit= before stdin= must still
			// terminate exec= and leave stdin= intact.
			name: "exit_code_before_stdin",
			mode: "foreground",
			line: "cmd=foreground:seq=1:exec=ze config validate -:exit=1:stdin=cfg",
			want: RunCommand{Mode: "foreground", Seq: 1, Exec: "ze config validate -", Stdin: "cfg", ExitCode: new(1)},
		},
		{
			name:    "invalid_exit_not_a_number",
			mode:    "foreground",
			line:    "cmd=foreground:seq=1:exec=ze doctor:exit=yes",
			wantErr: "invalid exit=",
		},
		{
			name:    "invalid_exit_negative",
			mode:    "foreground",
			line:    "cmd=foreground:seq=1:exec=ze doctor:exit=-1",
			wantErr: "invalid exit=",
		},
		{
			name:    "invalid_exit_above_255",
			mode:    "foreground",
			line:    "cmd=foreground:seq=1:exec=ze doctor:exit=256",
			wantErr: "invalid exit=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCmdExec(tt.mode, tt.line)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseHTTP verifies marker-based parsing of http= lines.
//
// VALIDATES: parseHTTP correctly handles URLs with colons, optional contains, repeatable
// header= keys (including values that themselves contain colons), and any marker order.
// PREVENTS: Panics on reordered markers, colons in URLs truncating values, a header=
// swallowing a following key's value, and a malformed header= being silently dropped.
// The header-free cases double as the no-regression proof. They compare the WHOLE
// httpCheck, so a stray Headers entry on a line without header= would fail them.
func TestParseHTTP(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    httpCheck
		wantErr string
	}{
		{
			name: "basic_get",
			line: "http=get:seq=1:url=http://127.0.0.1:8000/:status=200",
			want: httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:8000/", Status: 200},
		},
		{
			name: "with_contains",
			line: "http=get:seq=2:url=http://127.0.0.1:8000/peers:status=200:contains=peer-0",
			want: httpCheck{Seq: 2, Method: "get", URL: "http://127.0.0.1:8000/peers", Status: 200, Contains: "peer-0"},
		},
		{
			name: "url_with_query_params",
			line: "http=get:seq=3:url=http://127.0.0.1:9090/cell?src=0&dst=1:status=200",
			want: httpCheck{Seq: 3, Method: "get", URL: "http://127.0.0.1:9090/cell?src=0&dst=1", Status: 200},
		},
		{
			name: "url_with_port_variable",
			line: "http=get:seq=1:url=http://127.0.0.1:$PORT/viz/events:status=200",
			want: httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:$PORT/viz/events", Status: 200},
		},
		{
			name: "post_sendfile_content_type",
			line: "http=post:seq=4:url=https://127.0.0.1:$PORT2/config/set/bgp/:status=200:sendfile=set.form:content-type=application/x-www-form-urlencoded:insecure-tls=true",
			want: httpCheck{Seq: 4, Method: "post", URL: "https://127.0.0.1:$PORT2/config/set/bgp/", Status: 200, SendFile: "set.form", ContentType: "application/x-www-form-urlencoded", InsecureTLS: true},
		},
		{
			name: "single_header",
			line: "http=post:seq=1:url=http://127.0.0.1:$PORT/mcp:status=200:header=MCP-Protocol-Version: 2026-07-28",
			want: httpCheck{Seq: 1, Method: "post", URL: "http://127.0.0.1:$PORT/mcp", Status: 200, Headers: []httpHeader{
				{Name: "MCP-Protocol-Version", Value: "2026-07-28"},
			}},
		},
		{
			name: "two_headers_one_line",
			line: "http=post:seq=2:url=http://127.0.0.1:$PORT/mcp:status=200:header=Mcp-Method: tools/list:header=Mcp-Name: ze",
			want: httpCheck{Seq: 2, Method: "post", URL: "http://127.0.0.1:$PORT/mcp", Status: 200, Headers: []httpHeader{
				{Name: "Mcp-Method", Value: "tools/list"},
				{Name: "Mcp-Name", Value: "ze"},
			}},
		},
		{
			name: "header_with_sendfile_and_content_type",
			line: "http=post:seq=3:url=http://127.0.0.1:$PORT/mcp:status=200:sendfile=call.json:content-type=application/json:header=MCP-Protocol-Version: 2026-07-28:header=Mcp-Method: tools/call:contains=result",
			want: httpCheck{Seq: 3, Method: "post", URL: "http://127.0.0.1:$PORT/mcp", Status: 200, Contains: "result", SendFile: "call.json", ContentType: "application/json", Headers: []httpHeader{
				{Name: "MCP-Protocol-Version", Value: "2026-07-28"},
				{Name: "Mcp-Method", Value: "tools/call"},
			}},
		},
		{
			name: "header_value_with_colon",
			line: "http=get:seq=4:url=http://127.0.0.1:$PORT/:status=200:header=Referer: http://127.0.0.1:8080/page",
			want: httpCheck{Seq: 4, Method: "get", URL: "http://127.0.0.1:$PORT/", Status: 200, Headers: []httpHeader{
				{Name: "Referer", Value: "http://127.0.0.1:8080/page"},
			}},
		},
		{
			name: "header_without_space_after_colon",
			line: "http=get:seq=5:url=http://127.0.0.1:$PORT/:status=200:header=Mcp-Name:ze",
			want: httpCheck{Seq: 5, Method: "get", URL: "http://127.0.0.1:$PORT/", Status: 200, Headers: []httpHeader{
				{Name: "Mcp-Name", Value: "ze"},
			}},
		},
		{
			name:    "header_missing_colon",
			line:    "http=get:seq=1:url=http://host/:status=200:header=MCP-Protocol-Version",
			wantErr: `invalid header="MCP-Protocol-Version"`,
		},
		{
			name:    "header_empty_name",
			line:    "http=get:seq=1:url=http://host/:status=200:header= : value",
			wantErr: "invalid header=",
		},
		{
			name:    "invalid_insecure_tls",
			line:    "http=get:seq=1:url=https://127.0.0.1:$PORT2/:status=200:insecure-tls=maybe",
			wantErr: "invalid insecure-tls=",
		},
		{
			name:    "missing_seq",
			line:    "http=get:url=http://host/:status=200",
			wantErr: "missing seq=",
		},
		{
			name:    "missing_url",
			line:    "http=get:seq=1:status=200",
			wantErr: "missing url=",
		},
		{
			name:    "missing_status",
			line:    "http=get:seq=1:url=http://host/",
			wantErr: "missing status=",
		},
		{
			name:    "invalid_seq_zero",
			line:    "http=get:seq=0:url=http://host/:status=200",
			wantErr: "invalid seq=",
		},
		{
			name:    "invalid_status",
			line:    "http=get:seq=1:url=http://host/:status=abc",
			wantErr: "invalid status=",
		},
		{
			name:    "unsupported_method",
			line:    "http=delete:seq=1:url=http://host/:status=200",
			wantErr: "unsupported HTTP method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			et := NewEncodingTests(t.TempDir())
			rec := newRecord("test")

			// Extract method from the line (http=METHOD:...)
			parts := strings.SplitN(tt.line, ":", 2)
			method := strings.TrimPrefix(parts[0], "http=")

			err := et.parseHTTP(rec, method, tt.line)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, rec.HTTPChecks, 1)
			assert.Equal(t, tt.want, rec.HTTPChecks[0])
		})
	}
}

func TestParseHTTPWait(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		want     httpCheck
		wantErr  string
		wantWait bool // true = stored in HTTPWaits, false = HTTPChecks
	}{
		{
			name:     "basic_wait",
			line:     "http=wait:seq=1:url=http://127.0.0.1:8000/:status=200",
			want:     httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:8000/", Status: 200},
			wantWait: true,
		},
		{
			name:     "wait_with_contains",
			line:     "http=wait:seq=1:url=http://127.0.0.1:8000/graph:status=200:contains=AS2914",
			want:     httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:8000/graph", Status: 200, Contains: "AS2914"},
			wantWait: true,
		},
		{
			name:     "wait_with_timeout",
			line:     "http=wait:seq=1:url=http://127.0.0.1:8000/:status=200:timeout=10s",
			want:     httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:8000/", Status: 200, Timeout: "10s"},
			wantWait: true,
		},
		{
			name:     "wait_with_contains_and_timeout",
			line:     "http=wait:seq=1:url=http://127.0.0.1:$PORT2/lg/graph:status=200:contains=AS2914:timeout=15s",
			want:     httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:$PORT2/lg/graph", Status: 200, Contains: "AS2914", Timeout: "15s"},
			wantWait: true,
		},
		{
			name: "wait_with_header_and_timeout",
			line: "http=wait:seq=1:url=http://127.0.0.1:$PORT/mcp:status=200:header=MCP-Protocol-Version: 2026-07-28:timeout=20s",
			want: httpCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:$PORT/mcp", Status: 200, Timeout: "20s", Headers: []httpHeader{
				{Name: "MCP-Protocol-Version", Value: "2026-07-28"},
			}},
			wantWait: true,
		},
		{
			name:    "wait_invalid_timeout",
			line:    "http=wait:seq=1:url=http://host/:status=200:timeout=bogus",
			wantErr: "invalid timeout=",
		},
		{
			name:    "wait_missing_seq",
			line:    "http=wait:url=http://host/:status=200",
			wantErr: "missing seq=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			et := NewEncodingTests(t.TempDir())
			rec := newRecord("test")

			parts := strings.SplitN(tt.line, ":", 2)
			method := strings.TrimPrefix(parts[0], "http=")

			err := et.parseHTTP(rec, method, tt.line)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantWait {
				require.Len(t, rec.HTTPWaits, 1)
				assert.Empty(t, rec.HTTPChecks)
				assert.Equal(t, tt.want, rec.HTTPWaits[0])
			} else {
				require.Len(t, rec.HTTPChecks, 1)
				assert.Empty(t, rec.HTTPWaits)
				assert.Equal(t, tt.want, rec.HTTPChecks[0])
			}
		})
	}
}

// TestParseCIOptionSkipOS verifies option=skip-os sets SkipReason when the
// current GOOS matches, and leaves it empty otherwise. The list is
// comma-separated; any match wins.
//
// VALIDATES: option=skip-os:value=darwin produces a SKIP on darwin without
// running the test, and a no-op on other OSes. Matches the contract
// documented in .claude/rules/os-specific-tests.md.
// PREVENTS: a feature stubbed on one OS (e.g. IP_RECVTTL on darwin) causing
// spurious test failures when the .ci runs everywhere by default.
func TestParseCIOptionSkipOS(t *testing.T) {
	tests := []struct {
		name       string
		skipValue  string
		wantSkip   bool
		wantReason string
	}{
		{
			name:      "current-os-matches",
			skipValue: runtime.GOOS,
			wantSkip:  true,
		},
		{
			name:      "other-os-single",
			skipValue: "plan9",
			wantSkip:  false,
		},
		{
			name:      "current-os-in-list",
			skipValue: "plan9," + runtime.GOOS + ",solaris",
			wantSkip:  true,
		},
		{
			name:      "other-os-list",
			skipValue: "plan9, solaris",
			wantSkip:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetNickCounter()

			tmpDir := t.TempDir()
			ciFile := filepath.Join(tmpDir, "test.ci")
			confFile := filepath.Join(tmpDir, "test.conf")

			ciContent := "option=file:path=test.conf\noption=skip-os:value=" + tt.skipValue + "\n"
			require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
			require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

			et := NewEncodingTests(tmpDir)
			_, err := et.parseAndAdd(ciFile)
			require.NoError(t, err)

			rec := et.GetByNick("1")
			require.NotNil(t, rec)

			if tt.wantSkip {
				assert.NotEmpty(t, rec.SkipReason, "SkipReason should be set when GOOS matches")
				assert.Contains(t, rec.SkipReason, runtime.GOOS)
			} else {
				assert.Empty(t, rec.SkipReason, "SkipReason should be empty when GOOS does not match")
			}
		})
	}
}

// TestParseCIOptionSkipOSMissingValue verifies that option=skip-os without
// a value= key produces a clear parse error. Silently accepting it would
// let an author write a test that runs everywhere when they intended to
// skip a specific OS.
func TestParseCIOptionSkipOSMissingValue(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := "option=file:path=test.conf\noption=skip-os\n"
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skip-os")
}
