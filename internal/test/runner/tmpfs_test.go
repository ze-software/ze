package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTmpfsInCI verifies tmpfs blocks are parsed from .ci files.
//
// VALIDATES: tmpfs blocks extracted and stored in TmpfsFiles map.
// PREVENTS: tmpfs blocks ignored or lost during parsing.
func TestParseTmpfsInCI(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .ci file with tmpfs blocks
	ciContent := `tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
}
EOF_CONF

option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
`
	ciFile := filepath.Join(tmpDir, "test-tmpfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	// Parse the test
	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	// Verify tmpfs files were extracted
	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	require.NotNil(t, r.TmpfsFiles, "TmpfsFiles should be populated")
	assert.Contains(t, r.TmpfsFiles, "peer.conf")
	assert.Contains(t, string(r.TmpfsFiles["peer.conf"]), "local-as 65533")

	// Verify other lines were parsed
	assert.Equal(t, "65533", r.Extra["asn"])
	assert.Len(t, r.Expects, 1)
}

// TestParseTmpfsMultipleFiles verifies multiple tmpfs blocks in one .ci file.
//
// VALIDATES: Multiple tmpfs files extracted correctly.
// PREVENTS: Only first tmpfs block parsed, others ignored.
func TestParseTmpfsMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	ciContent := `tmpfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
EOF_RULES

tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
}
EOF_CONF

option=asn:value=65533
`
	ciFile := filepath.Join(tmpDir, "multi-tmpfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	require.NotNil(t, r.TmpfsFiles)
	assert.Len(t, r.TmpfsFiles, 2)
	assert.Contains(t, r.TmpfsFiles, "rules.ci")
	assert.Contains(t, r.TmpfsFiles, "peer.conf")
}

// TestParseTmpfsWithSubdirs verifies tmpfs paths with subdirectories.
//
// VALIDATES: Paths like scripts/plugin.py stored correctly.
// PREVENTS: Path flattening or directory info lost.
func TestParseTmpfsWithSubdirs(t *testing.T) {
	tmpDir := t.TempDir()

	ciContent := `tmpfs=conf/peer.conf:terminator=EOF_CONF
peer config
EOF_CONF

tmpfs=scripts/plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
print("hello")
EOF_PY

option=asn:value=65533
`
	ciFile := filepath.Join(tmpDir, "subdir-tmpfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	require.NotNil(t, r.TmpfsFiles)
	assert.Contains(t, r.TmpfsFiles, "conf/peer.conf")
	assert.Contains(t, r.TmpfsFiles, "scripts/plugin.py")
}

// TestParseNoTmpfs verifies .ci files without tmpfs still work.
//
// VALIDATES: Backward compatibility with non-tmpfs .ci files.
// PREVENTS: Regression in existing test parsing.
func TestParseNoTmpfs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file for the test to reference
	confContent := "peer 127.0.0.1 { local-as 65533; }"
	confFile := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(confFile, []byte(confContent), 0o600))

	ciContent := `option=file:path=test.conf
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
`
	ciFile := filepath.Join(tmpDir, "no-tmpfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	// TmpfsFiles should be nil or empty when no tmpfs blocks
	assert.Empty(t, r.TmpfsFiles)
	// Config should still be parsed from option:file
	assert.Equal(t, confFile, r.ConfigFile)
}
