package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsingSuiteHonorsTmpfsMode verifies the PARSE suite materializes a tmpfs
// block with the mode the block declares.
//
// VALIDATES: setupWorkDir writes through tmpfs.WriteTo, so `mode=755` and the
// executable default of a `.sh` path both reach disk.
// PREVENTS: the defect this test was written for. setupWorkDir open-coded the
// write loop with a fixed 0o644, so `exec=./script.sh` in a test/parse/*.ci
// failed with "fork/exec: permission denied" -- the directive was parsed,
// accepted and silently ignored, while every other suite honored it.
func TestParsingSuiteHonorsTmpfsMode(t *testing.T) {
	tmpDir := t.TempDir()

	ciContent := `tmpfs=driver.sh:mode=755:terminator=EOF_SH
#!/bin/sh
exit 0
EOF_SH

tmpfs=router.conf:terminator=EOF_CONF
bgp {
}
EOF_CONF

cmd=foreground:seq=1:exec=./driver.sh
expect=exit:code=0
`
	ciFile := filepath.Join(tmpDir, "mode-honored.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	pt := NewParsingTests(tmpDir)
	test, err := pt.parseCIFile(ciFile)
	require.NoError(t, err)

	r := NewParsingRunner(pt, tmpDir, "ze")
	workDir, err := r.setupWorkDir(test)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(workDir) })

	script, err := os.Stat(filepath.Join(workDir, "driver.sh"))
	require.NoError(t, err)
	assert.NotZero(t, script.Mode().Perm()&0o100, "a mode=755 script must be executable by its owner")

	conf, err := os.Stat(filepath.Join(workDir, "router.conf"))
	require.NoError(t, err)
	assert.Zero(t, conf.Mode().Perm()&0o111, "a block with no mode= keeps the non-executable default")
}

// TestParseTmpfsInCI verifies tmpfs blocks are parsed from .ci files.
//
// VALIDATES: tmpfs blocks extracted and stored in TmpfsFiles map.
// PREVENTS: tmpfs blocks ignored or lost during parsing.
func TestParseTmpfsInCI(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .ci file with tmpfs blocks
	ciContent := `tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533
    peer-as 65533
}
EOF_CONF

option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
`
	ciFile := filepath.Join(tmpDir, "test-tmpfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	// Parse the test
	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
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
    local-as 65533
}
EOF_CONF

option=asn:value=65533
`
	ciFile := filepath.Join(tmpDir, "multi-tmpfs.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
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
	_, err := et.parseAndAdd(ciFile)
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
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	tests := et.Registered()
	require.Len(t, tests, 1)
	r := tests[0]

	// TmpfsFiles should be nil or empty when no tmpfs blocks
	assert.Empty(t, r.TmpfsFiles)
	// Config should still be parsed from option:file
	assert.Equal(t, confFile, r.ConfigFile)
}
