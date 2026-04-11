package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAndAdd_EnvVarOutsidePeerBlockAccepted verifies that option=env
// placed above the stdin=peer block is parsed into Record.EnvVars.
//
// VALIDATES: AC-1 — option=env outside peer block populates rec.EnvVars.
// PREVENTS: Regression where the previously-valid placement stops working.
func TestParseAndAdd_EnvVarOutsidePeerBlockAccepted(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "outside.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
option=env:var=ze.log.bgp.server:value=debug
stdin=peer:terminator=EOF_PEER
option=timeout:value=5s
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
EOF_PEER
`
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)
	assert.Equal(t, []string{"ze.log.bgp.server=debug"}, rec.EnvVars)
}

// TestParseAndAdd_EnvVarInsidePeerBlockRejected verifies that option=env
// placed inside the stdin=peer block causes parseAndAdd to return a
// non-nil error whose message names the directive and says "outside".
//
// VALIDATES: AC-2 — parser rejects option=env inside peer block with
// an actionable error referencing the directive text.
// PREVENTS: The silent-drop that masked broken tests for months
// (see plan/learned/545-debug-plugin-test-cluster.md).
func TestParseAndAdd_EnvVarInsidePeerBlockRejected(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "inside.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
stdin=peer:terminator=EOF_PEER
option=timeout:value=5s
option=env:var=ze.log.bgp.server:value=debug
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
EOF_PEER
`
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.Error(t, err, "expected parse error for option=env inside peer block")

	msg := err.Error()
	// Error must quote the directive text so the author sees exactly what to move.
	assert.Contains(t, msg, "option=env:var=ze.log.bgp.server:value=debug",
		"error should name the directive")
	// Error must explain the remedy: move OUTSIDE the peer block.
	assert.Contains(t, msg, "outside",
		"error should tell the author to place the directive outside the peer block")
	// Error must identify the position inside the block so the author can jump to it.
	assert.Contains(t, msg, "stdin=peer block line",
		"error should name the stdin=peer block and line offset")
}

// TestParseAndAdd_OptionTimeoutInsidePeerBlockPasses verifies that
// option=timeout inside the peer block is still accepted — it is
// consumed by ze-peer from its stdin, not by the runner, and must
// continue to pass through unchanged.
//
// VALIDATES: AC-5 — non-env option directives inside peer blocks
// are not rejected by the hardening check.
// PREVENTS: Over-broad rejection that would also kill valid peer
// block directives (timeout, open, update, tcp_connections).
func TestParseAndAdd_OptionTimeoutInsidePeerBlockPasses(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "timeout.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
stdin=peer:terminator=EOF_PEER
option=timeout:value=10s
option=open:value=inspect-open-message
option=update:value=inspect-update-message
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
EOF_PEER
`
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err, "non-env option directives inside peer block must be accepted")
}
