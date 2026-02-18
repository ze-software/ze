package runner

import (
	"os"
	"path/filepath"
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
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	// Should have 2 messages
	assert.Len(t, rec.Messages, 2, "should have 2 messages")

	// First message: conn=1, seq=1 → index 101
	msg1 := rec.GetMessage(101)
	require.NotNil(t, msg1, "message 101 should exist")
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304", msg1.RawHex)

	// Second message: conn=1, seq=2 → index 102
	msg2 := rec.GetMessage(102)
	require.NotNil(t, msg2, "message 102 should exist")
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02", msg2.RawHex)
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	// Should have 1 message with both hex and json
	assert.Len(t, rec.Messages, 1, "should have 1 message")

	msg := rec.GetMessage(101)
	require.NotNil(t, msg)
	assert.Equal(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304", msg.RawHex)
	assert.Equal(t, `{"type":"keepalive"}`, msg.JSON)
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	// Should have 4 messages with proper conn/seq indexing
	// conn=1:seq=1 → index 101, conn=1:seq=2 → index 102
	// conn=2:seq=1 → index 201, conn=2:seq=2 → index 202
	assert.Len(t, rec.Messages, 4, "should have 4 messages")

	assert.Equal(t, "AAAA", rec.GetMessage(101).RawHex) // conn=1:seq=1
	assert.Equal(t, "CCCC", rec.GetMessage(102).RawHex) // conn=1:seq=2
	assert.Equal(t, "BBBB", rec.GetMessage(201).RawHex) // conn=2:seq=1
	assert.Equal(t, "DDDD", rec.GetMessage(202).RawHex) // conn=2:seq=2
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	msg := rec.GetMessage(101)
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
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	assert.Equal(t, []string{"fatal", "panic"}, rec.RejectSyslog)
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
	err := et.parseAndAdd(ciFile)

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
	err := et.parseAndAdd(ciFile)

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
// VALIDATES: parseCmdExec correctly extracts seq, exec (with colons), stdin, timeout.
// PREVENTS: Colons in exec values being misinterpreted as field delimiters.
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
			line: "cmd=background:seq=1:exec=ze-chaos --web :8080 --quiet",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "ze-chaos --web :8080 --quiet"},
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
// VALIDATES: parseHTTP correctly handles URLs with colons, optional contains, and any marker order.
// PREVENTS: Panics on reordered markers, colons in URLs truncating values.
func TestParseHTTP(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    HTTPCheck
		wantErr string
	}{
		{
			name: "basic_get",
			line: "http=get:seq=1:url=http://127.0.0.1:8080/:status=200",
			want: HTTPCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:8080/", Status: 200},
		},
		{
			name: "with_contains",
			line: "http=get:seq=2:url=http://127.0.0.1:8080/peers:status=200:contains=peer-0",
			want: HTTPCheck{Seq: 2, Method: "get", URL: "http://127.0.0.1:8080/peers", Status: 200, Contains: "peer-0"},
		},
		{
			name: "url_with_query_params",
			line: "http=get:seq=3:url=http://127.0.0.1:9090/cell?src=0&dst=1:status=200",
			want: HTTPCheck{Seq: 3, Method: "get", URL: "http://127.0.0.1:9090/cell?src=0&dst=1", Status: 200},
		},
		{
			name: "url_with_port_variable",
			line: "http=get:seq=1:url=http://127.0.0.1:$PORT/viz/events:status=200",
			want: HTTPCheck{Seq: 1, Method: "get", URL: "http://127.0.0.1:$PORT/viz/events", Status: 200},
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
			rec := NewRecord("test")

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
