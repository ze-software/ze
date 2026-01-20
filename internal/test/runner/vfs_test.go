package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseVFSInCI verifies VFS blocks are parsed from .ci files.
//
// VALIDATES: VFS blocks extracted and stored in VFSFiles map.
// PREVENTS: VFS blocks ignored or lost during parsing.
func TestParseVFSInCI(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .ci file with VFS blocks
	ciContent := `vfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
}
EOF_CONF

option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
`
	ciFile := filepath.Join(tmpDir, "test-vfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	// Parse the test
	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	// Verify VFS files were extracted
	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	require.NotNil(t, r.VFSFiles, "VFSFiles should be populated")
	assert.Contains(t, r.VFSFiles, "peer.conf")
	assert.Contains(t, string(r.VFSFiles["peer.conf"]), "local-as 65533")

	// Verify other lines were parsed
	assert.Equal(t, "65533", r.Extra["asn"])
	assert.Len(t, r.Expects, 1)
}

// TestParseVFSMultipleFiles verifies multiple VFS blocks in one .ci file.
//
// VALIDATES: Multiple VFS files extracted correctly.
// PREVENTS: Only first VFS block parsed, others ignored.
func TestParseVFSMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	ciContent := `vfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
EOF_RULES

vfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
}
EOF_CONF

option=asn:value=65533
`
	ciFile := filepath.Join(tmpDir, "multi-vfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	require.NotNil(t, r.VFSFiles)
	assert.Len(t, r.VFSFiles, 2)
	assert.Contains(t, r.VFSFiles, "rules.ci")
	assert.Contains(t, r.VFSFiles, "peer.conf")
}

// TestParseVFSWithSubdirs verifies VFS paths with subdirectories.
//
// VALIDATES: Paths like scripts/plugin.py stored correctly.
// PREVENTS: Path flattening or directory info lost.
func TestParseVFSWithSubdirs(t *testing.T) {
	tmpDir := t.TempDir()

	ciContent := `vfs=conf/peer.conf:terminator=EOF_CONF
peer config
EOF_CONF

vfs=scripts/plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
print("hello")
EOF_PY

option=asn:value=65533
`
	ciFile := filepath.Join(tmpDir, "subdir-vfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	require.NotNil(t, r.VFSFiles)
	assert.Contains(t, r.VFSFiles, "conf/peer.conf")
	assert.Contains(t, r.VFSFiles, "scripts/plugin.py")
}

// TestParseNoVFS verifies .ci files without VFS still work.
//
// VALIDATES: Backward compatibility with non-VFS .ci files.
// PREVENTS: Regression in existing test parsing.
func TestParseNoVFS(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file for the test to reference
	confContent := "peer 127.0.0.1 { local-as 65533; }"
	confFile := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(confFile, []byte(confContent), 0o600))

	ciContent := `option=file:path=test.conf
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
`
	ciFile := filepath.Join(tmpDir, "no-vfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	// VFSFiles should be nil or empty when no VFS blocks
	assert.Empty(t, r.VFSFiles)
	// Config should still be parsed from option=file
	assert.Equal(t, confFile, r.ConfigFile)
}
