package tmpfs

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRejectAbsolutePath verifies absolute paths are rejected.
//
// VALIDATES: Paths starting with / are rejected.
// PREVENTS: Writing to arbitrary filesystem locations like /etc/passwd.
func TestRejectAbsolutePath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"root_etc_passwd", "/etc/passwd"},
		{"root_tmp", "/tmp/malicious"},
		{"double_slash", "//etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "tmpfs=" + tt.path + ":terminator=EOF\ncontent\nEOF\n"
			v, err := Parse(strings.NewReader(input))
			if err == nil {
				// Parse may succeed, but Validate should fail
				err = v.Validate()
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "absolute")
		})
	}
}

// TestRejectParentTraversal verifies parent directory traversal is rejected.
//
// VALIDATES: Paths containing .. are rejected after normalization.
// PREVENTS: Escaping temp directory via ../../etc/passwd attacks.
func TestRejectParentTraversal(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"simple_parent", "../secret"},
		{"double_parent", "../../etc/passwd"},
		{"hidden_parent", "foo/../../../etc/passwd"},
		{"middle_parent", "a/b/../../c/../../../etc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "tmpfs=" + tt.path + ":terminator=EOF\ncontent\nEOF\n"
			v, err := Parse(strings.NewReader(input))
			if err == nil {
				err = v.Validate()
			}
			require.Error(t, err)
			// Should mention either "parent" or "escape" or ".."
			errStr := err.Error()
			assert.True(t, strings.Contains(errStr, "parent") ||
				strings.Contains(errStr, "escape") ||
				strings.Contains(errStr, "..") ||
				strings.Contains(errStr, "traversal"),
				"error should mention path traversal: %s", errStr)
		})
	}
}

// TestRejectPathEscape verifies paths that escape base directory are rejected.
//
// VALIDATES: Normalized paths outside base dir are caught.
// PREVENTS: Subtle escape attempts like foo/../../bar.
func TestRejectPathEscape(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"single_escape", "foo/../../bar"},
		{"deep_escape", "a/b/c/../../../../etc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "tmpfs=" + tt.path + ":terminator=EOF\ncontent\nEOF\n"
			v, err := Parse(strings.NewReader(input))
			if err == nil {
				err = v.Validate()
			}
			require.Error(t, err)
		})
	}
}

// TestRejectHiddenFiles verifies hidden files (starting with .) are rejected.
//
// VALIDATES: Files starting with . are rejected by default.
// PREVENTS: Creating .bashrc, .ssh/authorized_keys, etc.
func TestRejectHiddenFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"hidden_file", ".secret"},
		{"hidden_dir", ".ssh/authorized_keys"},
		{"nested_hidden", "foo/.hidden/bar"},
		{"dotfile", ".bashrc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "tmpfs=" + tt.path + ":terminator=EOF\ncontent\nEOF\n"
			v, err := Parse(strings.NewReader(input))
			if err == nil {
				err = v.Validate()
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "hidden")
		})
	}
}

// TestRejectOversizeFile verifies files exceeding limit are rejected.
//
// VALIDATES: Files larger than MaxFileSize are rejected.
// PREVENTS: Memory exhaustion via huge embedded files.
// BOUNDARY: 1048576 (valid), 1048577 (invalid).
func TestRejectOversizeFile(t *testing.T) {
	tests := []struct {
		name        string
		contentSize int
		maxSize     int64
		shouldErr   bool
	}{
		// Note: parser adds trailing newline, so content of 999 becomes 1000 bytes
		{"at_limit", 999, 1000, false},
		{"over_limit_by_one", 1000, 1000, true},
		{"way_over", 10000, 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Repeat("x", tt.contentSize)
			input := "tmpfs=test.txt:terminator=EOF\n" + content + "\nEOF\n"

			limits := Limits{
				MaxFileSize:  tt.maxSize,
				MaxTotalSize: 10000000, // Large enough
				MaxFiles:     100,
				MaxPathLen:   256,
				MaxPathDepth: 10,
			}

			_, err := ParseWithLimits(strings.NewReader(input), limits)
			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "size")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestRejectTooManyFiles verifies file count limit is enforced.
//
// VALIDATES: More than MaxFiles files are rejected.
// PREVENTS: Resource exhaustion via many small files.
// BOUNDARY: 100 (valid), 101 (invalid).
func TestRejectTooManyFiles(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		maxFiles  int
		shouldErr bool
	}{
		{"at_limit", 5, 5, false},
		{"over_limit", 6, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			for i := 0; i < tt.fileCount; i++ {
				term := "EOF" + strings.Repeat("X", i) // Unique terminators
				sb.WriteString("tmpfs=file" + string(rune('0'+i)) + ".txt:terminator=" + term + "\n")
				sb.WriteString("content\n")
				sb.WriteString(term + "\n\n")
			}

			limits := Limits{
				MaxFileSize:  10000,
				MaxTotalSize: 10000000,
				MaxFiles:     tt.maxFiles,
				MaxPathLen:   256,
				MaxPathDepth: 10,
			}

			_, err := ParseWithLimits(strings.NewReader(sb.String()), limits)
			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "file")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestRejectTotalSizeExceeded verifies total size limit is enforced.
//
// VALIDATES: Combined file sizes exceeding limit are rejected.
// PREVENTS: Splitting large content across many files to bypass per-file limit.
// BOUNDARY: 1048576 (valid), 1048577 (invalid).
func TestRejectTotalSizeExceeded(t *testing.T) {
	// Two files of 600 bytes each, limit 1000 total
	input := `tmpfs=a.txt:terminator=EOF1
` + strings.Repeat("a", 600) + `
EOF1

tmpfs=b.txt:terminator=EOF2
` + strings.Repeat("b", 600) + `
EOF2
`
	limits := Limits{
		MaxFileSize:  1000,
		MaxTotalSize: 1000, // Total is 1200, exceeds limit
		MaxFiles:     100,
		MaxPathLen:   256,
		MaxPathDepth: 10,
	}

	_, err := ParseWithLimits(strings.NewReader(input), limits)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total size")
}

// TestRejectPathLengthExceeded verifies path length limit is enforced.
//
// VALIDATES: Paths longer than MaxPathLen are rejected.
// PREVENTS: Long path attacks, filesystem issues.
// BOUNDARY: 256 (valid), 257 (invalid).
func TestRejectPathLengthExceeded(t *testing.T) {
	tests := []struct {
		name       string
		pathLen    int
		maxPathLen int
		shouldErr  bool
	}{
		{"at_limit", 50, 50, false},
		{"over_limit", 51, 50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := strings.Repeat("a", tt.pathLen) + ".txt"
			input := "tmpfs=" + path + ":terminator=EOF\ncontent\nEOF\n"

			limits := Limits{
				MaxFileSize:  10000,
				MaxTotalSize: 10000000,
				MaxFiles:     100,
				MaxPathLen:   tt.maxPathLen + 4, // +4 for ".txt"
				MaxPathDepth: 10,
			}

			_, err := ParseWithLimits(strings.NewReader(input), limits)
			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "path length")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestRejectPathDepthExceeded verifies path depth limit is enforced.
//
// VALIDATES: Paths deeper than MaxPathDepth are rejected.
// PREVENTS: Deep directory attacks, filesystem issues.
// BOUNDARY: 10 (valid), 11 (invalid).
func TestRejectPathDepthExceeded(t *testing.T) {
	tests := []struct {
		name         string
		depth        int
		maxPathDepth int
		shouldErr    bool
	}{
		{"at_limit", 3, 3, false},
		{"over_limit", 4, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create path with given depth: a/b/c/file.txt (depth 4)
			parts := make([]string, tt.depth-1)
			for i := range parts {
				parts[i] = string(rune('a' + i))
			}
			path := strings.Join(parts, "/") + "/file.txt"
			if tt.depth == 1 {
				path = "file.txt"
			}

			input := "tmpfs=" + path + ":terminator=EOF\ncontent\nEOF\n"

			limits := Limits{
				MaxFileSize:  10000,
				MaxTotalSize: 10000000,
				MaxFiles:     100,
				MaxPathLen:   256,
				MaxPathDepth: tt.maxPathDepth,
			}

			_, err := ParseWithLimits(strings.NewReader(input), limits)
			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "depth")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestWriteToRejectsSymlinkTarget verifies symlink targets are not followed.
//
// VALIDATES: WriteTo creates regular files, not following symlinks.
// PREVENTS: Symlink attacks where attacker places symlink in temp dir.
func TestWriteToRejectsSymlinkTarget(t *testing.T) {
	// This test checks that if a symlink exists at the target path,
	// WriteTo overwrites it with a regular file (doesn't follow it)
	tmpDir := t.TempDir()

	// Create a symlink in temp dir pointing elsewhere
	linkPath := tmpDir + "/test.txt"
	targetPath := "/tmp/ze-test-symlink-target-" + t.Name()
	defer func() { _ = os.Remove(targetPath) }()

	// Some systems may not allow symlinks, skip if so
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skip("cannot create symlink:", err)
	}

	// Parse Tmpfs with same filename as symlink
	input := "tmpfs=test.txt:terminator=EOF\nmalicious content\nEOF\n"
	v, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	// WriteTo should overwrite the symlink, not follow it
	err = v.WriteTo(tmpDir)
	require.NoError(t, err)

	// Verify link was replaced with regular file
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "should be regular file, not symlink")

	// Verify target file was NOT written
	_, err = os.Stat(targetPath)
	assert.True(t, os.IsNotExist(err), "symlink target should not exist")
}

// TestLimitsBoundary verifies boundary values for all limits.
//
// VALIDATES: Limits are enforced at exact boundary values.
// PREVENTS: Off-by-one errors in limit checking.
func TestLimitsBoundary(t *testing.T) {
	t.Run("file_size_boundary", func(t *testing.T) {
		// Content is 999 bytes + newline = 1000 in Content field
		// But we need to account for trailing newline added by parser
		limits := Limits{
			MaxFileSize:  1000,
			MaxTotalSize: 10000,
			MaxFiles:     100,
			MaxPathLen:   256,
			MaxPathDepth: 10,
		}

		// Last valid: exactly at limit
		content := strings.Repeat("x", 999) // 999 + \n = 1000
		input := "tmpfs=test.txt:terminator=EOF\n" + content + "\nEOF\n"
		_, err := ParseWithLimits(strings.NewReader(input), limits)
		require.NoError(t, err, "content at limit should succeed")

		// First invalid: one over
		content = strings.Repeat("x", 1000) // 1000 + \n = 1001
		input = "tmpfs=test.txt:terminator=EOF\n" + content + "\nEOF\n"
		_, err = ParseWithLimits(strings.NewReader(input), limits)
		require.Error(t, err, "content over limit should fail")
	})

	t.Run("file_count_boundary", func(t *testing.T) {
		limits := Limits{
			MaxFileSize:  10000,
			MaxTotalSize: 100000,
			MaxFiles:     3,
			MaxPathLen:   256,
			MaxPathDepth: 10,
		}

		// Last valid: exactly at limit
		input := `tmpfs=a.txt:terminator=EOFA
a
EOFA

tmpfs=b.txt:terminator=EOFB
b
EOFB

tmpfs=c.txt:terminator=EOFC
c
EOFC
`
		_, err := ParseWithLimits(strings.NewReader(input), limits)
		require.NoError(t, err, "3 files at limit should succeed")

		// First invalid: one over
		input += `
tmpfs=d.txt:terminator=EOFD
d
EOFD
`
		_, err = ParseWithLimits(strings.NewReader(input), limits)
		require.Error(t, err, "4 files over limit should fail")
	})
}
