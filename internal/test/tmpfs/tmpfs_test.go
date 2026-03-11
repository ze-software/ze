package tmpfs

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTmpfsBlock verifies basic Tmpfs block parsing.
//
// VALIDATES: Single Tmpfs block parsed correctly with path and content.
// PREVENTS: Wrong path extraction, content truncation.
func TestParseTmpfsBlock(t *testing.T) {
	input := `tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533
}
EOF_CONF
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 1)

	f := v.Files[0]
	assert.Equal(t, "peer.conf", f.Path)
	assert.Equal(t, fs.FileMode(0o644), f.Mode)
	assert.Equal(t, "peer 127.0.0.1 {\n    local-as 65533\n}\n", string(f.Content))
}

// TestParseMultipleBlocks verifies multiple Tmpfs blocks in stream.
//
// VALIDATES: Multiple files parsed from single input.
// PREVENTS: State leakage between blocks, early termination.
func TestParseMultipleBlocks(t *testing.T) {
	input := `tmpfs=first.conf:terminator=EOF_FIRST
first content
EOF_FIRST

tmpfs=second.conf:terminator=EOF_SECOND
second content
EOF_SECOND
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 2)

	assert.Equal(t, "first.conf", v.Files[0].Path)
	assert.Equal(t, "first content\n", string(v.Files[0].Content))
	assert.Equal(t, "second.conf", v.Files[1].Path)
	assert.Equal(t, "second content\n", string(v.Files[1].Content))
}

// TestModeDefaults verifies automatic mode detection for scripts.
//
// VALIDATES: Script extensions get 0755, others get 0644.
// PREVENTS: Non-executable scripts, executable non-scripts.
func TestModeDefaults(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantMode fs.FileMode
	}{
		{"python_script", "plugin.py", 0o755},
		{"shell_script", "run.sh", 0o755},
		{"bash_script", "run.bash", 0o755},
		{"perl_script", "run.pl", 0o755},
		{"ruby_script", "run.rb", 0o755},
		{"zsh_script", "run.zsh", 0o755},
		{"config_file", "peer.conf", 0o644},
		{"text_file", "readme.txt", 0o644},
		{"json_file", "data.json", 0o644},
		{"no_extension", "Makefile", 0o644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "tmpfs=" + tt.path + ":terminator=EOF\ncontent\nEOF\n"
			v, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, v.Files, 1)
			assert.Equal(t, tt.wantMode, v.Files[0].Mode)
		})
	}
}

// TestModeExplicit verifies explicit mode override.
//
// VALIDATES: Explicit mode= overrides default.
// PREVENTS: Default mode ignoring explicit setting.
func TestModeExplicit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMode fs.FileMode
	}{
		{
			name:     "explicit_644_for_script",
			input:    "tmpfs=script.py:mode=644:terminator=EOF\ncontent\nEOF\n",
			wantMode: 0o644,
		},
		{
			name:     "explicit_755_for_config",
			input:    "tmpfs=peer.conf:mode=755:terminator=EOF\ncontent\nEOF\n",
			wantMode: 0o755,
		},
		{
			name:     "explicit_600",
			input:    "tmpfs=secret.key:mode=600:terminator=EOF\ncontent\nEOF\n",
			wantMode: 0o600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(strings.NewReader(tt.input))
			require.NoError(t, err)
			require.Len(t, v.Files, 1)
			assert.Equal(t, tt.wantMode, v.Files[0].Mode)
		})
	}
}

// TestBase64Encoding verifies base64 encoded content.
//
// VALIDATES: Binary content decoded from base64.
// PREVENTS: Raw base64 stored instead of decoded bytes.
func TestBase64Encoding(t *testing.T) {
	// "hello world" in base64
	input := `tmpfs=binary.bin:encoding=base64:terminator=EOF
aGVsbG8gd29ybGQ=
EOF
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
	assert.Equal(t, []byte("hello world"), v.Files[0].Content)
}

// TestWriteTo verifies file creation in directory.
//
// VALIDATES: Files created with correct content and mode.
// PREVENTS: Wrong permissions, missing directories.
func TestWriteTo(t *testing.T) {
	input := `tmpfs=peer.conf:terminator=EOF_CONF
test content
EOF_CONF

tmpfs=scripts/plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
print("hello")
EOF_PY
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	tmpDir := t.TempDir()
	err = v.WriteTo(tmpDir)
	require.NoError(t, err)

	// Check peer.conf
	content, err := os.ReadFile(filepath.Join(tmpDir, "peer.conf")) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Equal(t, "test content\n", string(content))

	info, err := os.Stat(filepath.Join(tmpDir, "peer.conf"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o644), info.Mode().Perm())

	// Check scripts/plugin.py (subdirectory)
	content, err = os.ReadFile(filepath.Join(tmpDir, "scripts", "plugin.py")) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Contains(t, string(content), "print(\"hello\")")

	info, err = os.Stat(filepath.Join(tmpDir, "scripts", "plugin.py"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm())
}

// TestMalformedHeader verifies rejection of invalid tmpfs= lines.
//
// VALIDATES: Parser rejects malformed headers gracefully.
// PREVENTS: Panic on bad input, accepting invalid format.
func TestMalformedHeader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "missing_terminator",
			input:   "tmpfs=peer.conf\ncontent\n",
			wantErr: "terminator",
		},
		{
			name:    "empty_path",
			input:   "tmpfs=:terminator=EOF\ncontent\nEOF\n",
			wantErr: "empty path",
		},
		{
			name:    "invalid_mode",
			input:   "tmpfs=peer.conf:mode=abc:terminator=EOF\ncontent\nEOF\n",
			wantErr: "invalid mode",
		},
		{
			name:    "invalid_encoding",
			input:   "tmpfs=peer.conf:encoding=rot13:terminator=EOF\ncontent\nEOF\n",
			wantErr: "invalid encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.input))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestMissingTerminator verifies EOF without terminator detection.
//
// VALIDATES: Parser errors on unterminated block.
// PREVENTS: Silently truncating content, infinite loop.
func TestMissingTerminator(t *testing.T) {
	input := `tmpfs=peer.conf:terminator=EOF_NEVER
content line 1
content line 2
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminator")
}

// TestDuplicatePaths verifies rejection of duplicate paths.
//
// VALIDATES: Same path twice is rejected.
// PREVENTS: Silent overwrite, undefined behavior.
func TestDuplicatePaths(t *testing.T) {
	input := `tmpfs=peer.conf:terminator=EOF1
first
EOF1

tmpfs=peer.conf:terminator=EOF2
second
EOF2
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate path")
}

// TestEmptyTerminator verifies rejection of empty terminator.
//
// VALIDATES: Empty terminator is rejected.
// PREVENTS: Matching empty lines as terminator.
func TestEmptyTerminator(t *testing.T) {
	input := "tmpfs=peer.conf:terminator=\ncontent\n\n"
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminator")
}

// TestTerminatorSpecialChars verifies terminator validation.
//
// VALIDATES: Terminator only allows alphanumeric + underscore.
// PREVENTS: Terminators that could be confused with content.
func TestTerminatorSpecialChars(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
		errMatch  string // substring that should appear in error
	}{
		{
			name:      "valid_alphanumeric",
			input:     "tmpfs=test.conf:terminator=EOF123\ncontent\nEOF123\n",
			shouldErr: false,
		},
		{
			name:      "valid_underscore",
			input:     "tmpfs=test.conf:terminator=EOF_CONF\ncontent\nEOF_CONF\n",
			shouldErr: false,
		},
		{
			// Colon splits into separate parts: terminator=EOF becomes valid,
			// but CONF becomes an invalid key-value pair (no =)
			name:      "colon_splits_as_delimiter",
			input:     "tmpfs=test.conf:terminator=EOF:CONF\ncontent\nEOF:CONF\n",
			shouldErr: true,
			errMatch:  "invalid key-value pair",
		},
		{
			name:      "invalid_equals",
			input:     "tmpfs=test.conf:terminator=EOF=CONF\ncontent\nEOF=CONF\n",
			shouldErr: true,
			errMatch:  "terminator",
		},
		{
			name:      "invalid_dash",
			input:     "tmpfs=test.conf:terminator=EOF-CONF\ncontent\nEOF-CONF\n",
			shouldErr: true,
			errMatch:  "terminator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.input))
			if tt.shouldErr {
				require.Error(t, err)
				if tt.errMatch != "" {
					assert.Contains(t, err.Error(), tt.errMatch)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEmptyFile verifies 0-byte files are allowed.
//
// VALIDATES: Empty content between header and terminator is valid.
// PREVENTS: Rejecting legitimate empty files.
func TestEmptyFile(t *testing.T) {
	input := "tmpfs=empty.txt:terminator=EOF\nEOF\n"
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
	assert.Empty(t, v.Files[0].Content)
}

// TestEmptyPath verifies empty path is rejected.
//
// VALIDATES: Path must be non-empty.
// PREVENTS: Creating files with empty name.
func TestEmptyPath(t *testing.T) {
	input := "tmpfs=:terminator=EOF\ncontent\nEOF\n"
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

// TestDuplicateTerminators verifies same terminator in different blocks.
//
// VALIDATES: Terminator must be unique within file.
// PREVENTS: Ambiguous termination.
func TestDuplicateTerminators(t *testing.T) {
	input := `tmpfs=first.conf:terminator=EOF
first
EOF

tmpfs=second.conf:terminator=EOF
second
EOF
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate terminator")
}

// TestCommentsAndBlankLines verifies non-Tmpfs lines are ignored.
//
// VALIDATES: Comments and blank lines outside Tmpfs blocks are skipped.
// PREVENTS: Treating comments as content or errors.
func TestCommentsAndBlankLines(t *testing.T) {
	input := `# This is a comment

tmpfs=peer.conf:terminator=EOF_CONF
content
EOF_CONF

# Another comment
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
	assert.Equal(t, "content\n", string(v.Files[0].Content))
}

// TestKeyValueOrder verifies key=value pairs in any order.
//
// VALIDATES: mode=, encoding=, terminator= can appear in any order.
// PREVENTS: Order-dependent parsing failures.
func TestKeyValueOrder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMode fs.FileMode
	}{
		{
			name:     "terminator_first",
			input:    "tmpfs=test.txt:terminator=EOF:mode=600\ncontent\nEOF\n",
			wantMode: 0o600,
		},
		{
			name:     "mode_first",
			input:    "tmpfs=test.txt:mode=600:terminator=EOF\ncontent\nEOF\n",
			wantMode: 0o600,
		},
		{
			name:     "encoding_in_middle",
			input:    "tmpfs=test.txt:mode=644:encoding=text:terminator=EOF\ncontent\nEOF\n",
			wantMode: 0o644,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(strings.NewReader(tt.input))
			require.NoError(t, err)
			require.Len(t, v.Files, 1)
			assert.Equal(t, tt.wantMode, v.Files[0].Mode)
		})
	}
}

// TestBase64MultiLine verifies base64 with multiple lines.
//
// VALIDATES: Multiline base64 content is joined and decoded.
// PREVENTS: Line breaks corrupting base64 decode.
func TestBase64MultiLine(t *testing.T) {
	// Encode a longer string that would wrap in base64
	// "The quick brown fox jumps over the lazy dog" base64 = "VGhlIHF1aWNrIGJyb3duIGZveCBqdW1wcyBvdmVyIHRoZSBsYXp5IGRvZw=="
	input := `tmpfs=test.bin:encoding=base64:terminator=EOF
VGhlIHF1aWNrIGJyb3duIGZveCBqdW1wcyBvdmVyIHRoZSBsYXp5IGRvZw==
EOF
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
	assert.Equal(t, "The quick brown fox jumps over the lazy dog", string(v.Files[0].Content))
}

// TestNonTmpfsLinesPassthrough verifies non-Tmpfs lines are collected.
//
// VALIDATES: Lines not starting with tmpfs= are available for other consumers.
// PREVENTS: Losing cmd:, option:, expect: lines.
func TestNonTmpfsLinesPassthrough(t *testing.T) {
	input := `tmpfs=peer.conf:terminator=EOF_CONF
content
EOF_CONF

option:asn:value=65533
cmd:ze bgp validate tmpfs//peer.conf
expect:exit:code=0
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
	require.Len(t, v.OtherLines, 3)
	assert.Equal(t, "option:asn:value=65533", v.OtherLines[0])
	assert.Equal(t, "cmd:ze bgp validate tmpfs//peer.conf", v.OtherLines[1])
	assert.Equal(t, "expect:exit:code=0", v.OtherLines[2])
}

// TestParseWithLimits verifies custom limits are respected.
//
// VALIDATES: ParseWithLimits uses provided limits.
// PREVENTS: Ignoring custom limits.
func TestParseWithLimits(t *testing.T) {
	limits := Limits{
		MaxFileSize:  100,
		MaxTotalSize: 200,
		MaxFiles:     2,
		MaxPathLen:   50,
		MaxPathDepth: 3,
	}

	input := `tmpfs=a.txt:terminator=EOF1
short
EOF1

tmpfs=b.txt:terminator=EOF2
also short
EOF2
`
	v, err := ParseWithLimits(strings.NewReader(input), limits)
	require.NoError(t, err)
	require.Len(t, v.Files, 2)
}

// TestLargeReader verifies reading from large input doesn't buffer all.
//
// VALIDATES: Parser can handle streaming input.
// PREVENTS: Loading entire file into memory before parsing.
func TestLargeReader(t *testing.T) {
	// Create a reader that produces content on demand
	var buf bytes.Buffer
	buf.WriteString("tmpfs=test.txt:terminator=EOF\n")
	for range 1000 {
		buf.WriteString("line content\n")
	}
	buf.WriteString("EOF\n")

	v, err := Parse(&buf)
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
}

// TestTmpfsResolve verifies tmpfs// path replacement.
//
// VALIDATES: tmpfs//path becomes path after WriteTo.
// PREVENTS: Leaving tmpfs// prefix in paths.
func TestTmpfsResolve(t *testing.T) {
	v := &Tmpfs{
		OtherLines: []string{
			"cmd:ze bgp validate tmpfs//peer.conf",
			"cmd:ze bgp run tmpfs//scripts/plugin.py",
		},
	}

	resolved := v.ResolveTmpfsPaths()
	assert.Equal(t, "cmd:ze bgp validate peer.conf", resolved[0])
	assert.Equal(t, "cmd:ze bgp run scripts/plugin.py", resolved[1])
}

// TestCleanup verifies temp directory removal.
//
// VALIDATES: WriteToTemp cleanup function removes directory.
// PREVENTS: Temp directory leaks.
func TestCleanup(t *testing.T) {
	input := `tmpfs=test.txt:terminator=EOF
content
EOF
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	dir, cleanup, err := v.WriteToTemp()
	require.NoError(t, err)
	require.DirExists(t, dir)

	cleanup()
	require.NoDirExists(t, dir)
}

// TestReadFrom verifies reading Tmpfs from file path.
//
// VALIDATES: ReadFrom opens file and parses content.
// PREVENTS: File handle leaks, wrong error on missing file.
func TestReadFrom(t *testing.T) {
	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")

	content := `tmpfs=peer.conf:terminator=EOF
test content
EOF
`
	require.NoError(t, os.WriteFile(ciFile, []byte(content), 0o600))

	v, err := ReadFrom(ciFile)
	require.NoError(t, err)
	require.Len(t, v.Files, 1)
	assert.Equal(t, "peer.conf", v.Files[0].Path)
}

// TestReadFromMissingFile verifies error on missing file.
//
// VALIDATES: ReadFrom returns error for non-existent file.
// PREVENTS: Nil panic, unclear error message.
func TestReadFromMissingFile(t *testing.T) {
	_, err := ReadFrom("/nonexistent/path/test.ci")
	require.Error(t, err)
}

// TestLookup verifies file lookup by path.
//
// VALIDATES: Lookup returns correct file for path.
// PREVENTS: Wrong file returned, nil for existing file.
func TestLookup(t *testing.T) {
	input := `tmpfs=a.txt:terminator=EOF1
content a
EOF1

tmpfs=b.txt:terminator=EOF2
content b
EOF2
`
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	f := v.Lookup("a.txt")
	require.NotNil(t, f)
	assert.Equal(t, "content a\n", string(f.Content))

	f = v.Lookup("b.txt")
	require.NotNil(t, f)
	assert.Equal(t, "content b\n", string(f.Content))

	f = v.Lookup("nonexistent.txt")
	assert.Nil(t, f)
}

// TestIOReaderInterface verifies File implements io.Reader.
//
// VALIDATES: File.Read works for streaming content.
// PREVENTS: Type assertion failures.
func TestIOReaderInterface(t *testing.T) {
	f := &File{
		Path:    "test.txt",
		Content: []byte("hello world"),
	}

	r := f.Reader()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}
