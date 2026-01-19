package functional

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCIExpectBGP verifies parsing of expect=bgp lines.
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

// TestParseCIExpectJSON verifies parsing of expect=json lines.
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

// TestParseCIOptionEnv verifies parsing of option=env lines.
//
// VALIDATES: option=env:var=X:value=Y is parsed correctly.
// PREVENTS: Environment variables not being set for tests.
func TestParseCIOptionEnv(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
option=env:var=zebgp.log.server:value=debug
option=env:var=zebgp.log.filter:value=info
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	// Should have 2 env vars in KEY=VALUE format
	assert.Equal(t, []string{"zebgp.log.server=debug", "zebgp.log.filter=info"}, rec.EnvVars)
}

// TestParseCIOptionFile verifies parsing of option=file lines.
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

// TestParseCIActionNotification verifies parsing of action=notification lines.
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

	// Notification should be in Expects for testpeer to process (new format).
	found := false
	for _, exp := range rec.Expects {
		if exp == "action=notification:conn=1:seq=2:text=session ending" {
			found = true
			break
		}
	}
	assert.True(t, found, "notification action should be in Expects")
}

// TestParseCICmdAPI verifies parsing of cmd=api documentation lines.
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

// TestParseCIRejectSyslog verifies parsing of reject=syslog lines.
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

// TestParseCIMissingConn verifies error when conn is missing from expect=bgp.
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

// TestParseCIMissingSeq verifies error when seq is missing from expect=bgp.
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
