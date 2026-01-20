// Package vfs provides a Virtual File System for embedding multiple files in a single stream.
//
// VFS format:
//
//	vfs=<path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
//	<content>
//	<TERM>
//
// Example:
//
//	vfs=peer.conf:terminator=EOF_CONF
//	peer 127.0.0.1 {
//	    local-as 65533;
//	}
//	EOF_CONF
package vfs

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// File represents a single file in the VFS.
type File struct {
	Path    string
	Mode    fs.FileMode
	Content []byte
}

// Reader returns an io.Reader for the file content.
func (f *File) Reader() io.Reader {
	return bytes.NewReader(f.Content)
}

// VFS holds parsed virtual filesystem.
type VFS struct {
	Files      []*File
	OtherLines []string // Non-VFS lines (cmd=, option=, expect=, etc.)
}

// New creates an empty VFS for programmatic construction.
func New() *VFS {
	return &VFS{}
}

// AddFile adds a file to the VFS with default mode based on extension.
func (v *VFS) AddFile(path string, content []byte) {
	v.Files = append(v.Files, &File{
		Path:    path,
		Mode:    defaultModeForPath(path),
		Content: content,
	})
}

// AddFileWithMode adds a file to the VFS with explicit mode.
func (v *VFS) AddFileWithMode(path string, content []byte, mode fs.FileMode) {
	v.Files = append(v.Files, &File{
		Path:    path,
		Mode:    mode,
		Content: content,
	})
}

// Lookup returns the file at the given path, or nil if not found.
func (v *VFS) Lookup(path string) *File {
	for _, f := range v.Files {
		if f.Path == path {
			return f
		}
	}
	return nil
}

// ResolveVFSPaths replaces vfs// prefixes with plain paths.
func (v *VFS) ResolveVFSPaths() []string {
	result := make([]string, len(v.OtherLines))
	for i, line := range v.OtherLines {
		result[i] = strings.ReplaceAll(line, "vfs//", "")
	}
	return result
}

// Limits configures parsing limits.
type Limits struct {
	MaxFileSize  int64
	MaxTotalSize int64
	MaxFiles     int
	MaxPathLen   int
	MaxPathDepth int
}

// DefaultLimits returns standard limits.
func DefaultLimits() Limits {
	return Limits{
		MaxFileSize:  DefaultMaxFileSize,
		MaxTotalSize: DefaultMaxTotalSize,
		MaxFiles:     DefaultMaxFiles,
		MaxPathLen:   DefaultMaxPathLen,
		MaxPathDepth: DefaultMaxPathDepth,
	}
}

// Parse reads VFS blocks from reader using default limits.
func Parse(r io.Reader) (*VFS, error) {
	return ParseWithLimits(r, DefaultLimits())
}

// ParseWithLimits reads VFS blocks with custom limits.
func ParseWithLimits(r io.Reader, limits Limits) (*VFS, error) {
	v := &VFS{}
	scanner := bufio.NewScanner(r)
	seenPaths := make(map[string]bool)
	seenTerminators := make(map[string]bool)
	var totalSize int64

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines and comments outside VFS blocks
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for VFS block
		if strings.HasPrefix(trimmed, "vfs=") {
			file, endLineNum, err := parseVFSBlock(scanner, trimmed, lineNum, limits, seenTerminators)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			lineNum = endLineNum

			// Check for duplicate paths
			if seenPaths[file.Path] {
				return nil, fmt.Errorf("line %d: duplicate path %q", lineNum, file.Path)
			}
			seenPaths[file.Path] = true

			// Check limits
			if len(v.Files) >= limits.MaxFiles {
				return nil, fmt.Errorf("line %d: max files exceeded (%d)", lineNum, limits.MaxFiles)
			}
			if int64(len(file.Content)) > limits.MaxFileSize {
				return nil, fmt.Errorf("line %d: file size %d exceeds limit %d", lineNum, len(file.Content), limits.MaxFileSize)
			}
			totalSize += int64(len(file.Content))
			if totalSize > limits.MaxTotalSize {
				return nil, fmt.Errorf("line %d: total size exceeds limit %d", lineNum, limits.MaxTotalSize)
			}

			v.Files = append(v.Files, file)
		} else {
			// Collect non-VFS lines for other consumers
			v.OtherLines = append(v.OtherLines, trimmed)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	return v, nil
}

// validTerminator matches alphanumeric and underscore only.
var validTerminator = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// parseVFSBlock parses a single VFS block starting from the header line.
func parseVFSBlock(scanner *bufio.Scanner, header string, startLine int, limits Limits, seenTerminators map[string]bool) (*File, int, error) {
	// Parse header: vfs=<path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
	// Remove "vfs=" prefix
	rest := strings.TrimPrefix(header, "vfs=")

	// Parse all key=value pairs
	parts := strings.Split(rest, ":")
	if len(parts) == 0 {
		return nil, startLine, fmt.Errorf("invalid vfs header")
	}

	// First part is the path
	path := parts[0]
	if path == "" {
		return nil, startLine, fmt.Errorf("empty path")
	}

	// Validate path length and depth
	if len(path) > limits.MaxPathLen {
		return nil, startLine, fmt.Errorf("path length %d exceeds limit %d", len(path), limits.MaxPathLen)
	}
	depth := strings.Count(path, "/") + 1
	if depth > limits.MaxPathDepth {
		return nil, startLine, fmt.Errorf("path depth %d exceeds limit %d", depth, limits.MaxPathDepth)
	}

	// Parse remaining key=value pairs
	var terminator string
	var modeStr string
	encoding := "text"

	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			return nil, startLine, fmt.Errorf("invalid key-value pair: %q", part)
		}
		key := part[:eqIdx]
		value := part[eqIdx+1:]

		switch key {
		case "terminator":
			terminator = value
		case "mode":
			modeStr = value
		case "encoding":
			encoding = value
		default:
			return nil, startLine, fmt.Errorf("unknown key: %q", key)
		}
	}

	// Validate terminator
	if terminator == "" {
		return nil, startLine, fmt.Errorf("missing or empty terminator")
	}
	if !validTerminator.MatchString(terminator) {
		return nil, startLine, fmt.Errorf("invalid terminator %q: must be alphanumeric and underscore only", terminator)
	}
	if seenTerminators[terminator] {
		return nil, startLine, fmt.Errorf("duplicate terminator %q", terminator)
	}
	seenTerminators[terminator] = true

	// Validate encoding
	if encoding != "text" && encoding != "base64" {
		return nil, startLine, fmt.Errorf("invalid encoding %q: must be 'text' or 'base64'", encoding)
	}

	// Parse mode
	var mode fs.FileMode
	if modeStr != "" {
		modeVal, err := strconv.ParseInt(modeStr, 8, 32)
		if err != nil {
			return nil, startLine, fmt.Errorf("invalid mode %q: %w", modeStr, err)
		}
		// Validate mode is in valid Unix permission range (0-0777)
		if modeVal < 0 || modeVal > 0o777 {
			return nil, startLine, fmt.Errorf("invalid mode %q: must be 0-777 octal", modeStr)
		}
		mode = fs.FileMode(modeVal) //nolint:gosec // Range validated above
	} else {
		mode = defaultModeForPath(path)
	}

	// Read content until terminator
	var content bytes.Buffer
	lineNum := startLine
	found := false
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == terminator {
			found = true
			break
		}
		content.WriteString(line)
		content.WriteByte('\n')
	}

	if !found {
		return nil, lineNum, fmt.Errorf("unterminated VFS block: terminator %q not found", terminator)
	}

	// Decode content
	var contentBytes []byte
	if encoding == "base64" {
		// Join lines and decode
		b64 := strings.ReplaceAll(content.String(), "\n", "")
		var err error
		contentBytes, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, lineNum, fmt.Errorf("base64 decode error: %w", err)
		}
	} else {
		contentBytes = content.Bytes()
	}

	return &File{
		Path:    path,
		Mode:    mode,
		Content: contentBytes,
	}, lineNum, nil
}

// defaultModeForPath returns the default mode based on file extension.
func defaultModeForPath(path string) fs.FileMode {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py", ".sh", ".bash", ".zsh", ".pl", ".rb":
		return 0o755
	default:
		return 0o644
	}
}

// ReadFrom reads VFS from a file path.
func ReadFrom(path string) (*VFS, error) {
	f, err := os.Open(path) //nolint:gosec // Caller controls path
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return Parse(f)
}

// WriteTo creates files in directory.
func (v *VFS) WriteTo(baseDir string) error {
	for _, f := range v.Files {
		fullPath := filepath.Join(baseDir, f.Path)

		// Create parent directories
		// Note: 0755 is appropriate for temp directories used by test runner
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // Temp dir for tests
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}

		// Remove any existing file/symlink to avoid following symlinks
		// Use Lstat to detect symlinks (Stat follows them)
		if info, err := os.Lstat(fullPath); err == nil {
			// File exists - remove it (especially if it's a symlink)
			if info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular() {
				if err := os.Remove(fullPath); err != nil {
					return fmt.Errorf("remove existing %s: %w", f.Path, err)
				}
			}
		}

		// Write file
		if err := os.WriteFile(fullPath, f.Content, f.Mode); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

// WriteToTemp creates temp dir, writes files, returns path and cleanup.
func (v *VFS) WriteToTemp() (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "zebgp-vfs-*")
	if err != nil {
		return "", nil, err
	}

	cleanup = func() {
		_ = os.RemoveAll(dir)
	}

	if err := v.WriteTo(dir); err != nil {
		cleanup()
		return "", nil, err
	}

	return dir, cleanup, nil
}
