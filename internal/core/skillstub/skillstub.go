// Design: docs/contributing/claude-code-cheatsheet.md -- Where a Skill Lives

// Package skillstub writes, recognizes and removes a skill stub: a small file
// at <directory>/<name>/<file> that an agent harness lists by its frontmatter
// and that sends the agent to the skill's real instructions.
//
// A stub is OWNED by the caller that wrote it when its first line after the
// frontmatter block is byte-equal to the caller's marker line. The marker is
// the caller's, never this package's, so each caller recognizes only its own
// stubs and leaves every other file where it is.
//
// The stub's own directory and its file are never written through what sits
// on their path. os.MkdirAll and os.WriteFile follow a symlink, so a linked
// <name> directory or a linked file would have the link's target overwritten.
// The directory given to At is the caller's to vouch for: a caller that
// refuses a linked root checks it before it writes, and a caller that follows
// one passes it as it stands.
//
// The package reads no environment variable and names no tool or harness.
// Each refusal is a typed error, and the caller words the message an operator
// reads. Not safe for concurrent use on the same path: two writers race on the
// file system as any two writers do.
package skillstub

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Fence is the line that opens and closes a frontmatter block. A caller that
// tells an operator how to write a frontmatter names it from here.
const Fence = "---"

// Directory and file modes of what Write creates. A stub is the account's own
// file, and nothing outside that account reads it.
const (
	dirMode  os.FileMode = 0o750
	fileMode os.FileMode = 0o600
)

// ErrNotAName says a name or file given to At is not one directory entry, so
// the stub would be written outside its own directory.
var ErrNotAName = errors.New("skillstub: not a single directory entry name")

// DirectoryAtFileError says a directory sits where the stub's file belongs.
// Write refuses it rather than remove it, because the directory could hold
// files the caller did not write.
type DirectoryAtFileError struct {
	// Path is the file path the directory occupies.
	Path string
}

// Error names the path. A caller that shows an operator the refusal words
// its own message from Path.
func (e *DirectoryAtFileError) Error() string {
	return "skillstub: " + e.Path + " is a directory where the stub file belongs"
}

// Path is the place of one stub, <directory>/<name>/<file>. At is its only
// constructor: the zero Path names no stub, and a caller MUST NOT use it.
type Path struct {
	directory string
	name      string
	file      string
}

// At answers the place of one stub. name and file MUST each be one directory
// entry, never empty, "." or "..", and never holding a separator, so that the
// stub stays inside <directory>/<name>.
func At(directory, name, file string) (Path, error) {
	if err := isOneEntry(name); err != nil {
		return Path{}, err
	}
	if err := isOneEntry(file); err != nil {
		return Path{}, err
	}
	return Path{directory: directory, name: name, file: file}, nil
}

// isOneEntry refuses a name that is not one directory entry.
func isOneEntry(name string) error {
	if name == "" {
		return ErrNotAName
	}
	if name == "." {
		return ErrNotAName
	}
	if name == ".." {
		return ErrNotAName
	}
	if strings.ContainsRune(name, '/') {
		return ErrNotAName
	}
	if strings.ContainsRune(name, filepath.Separator) {
		return ErrNotAName
	}
	return nil
}

// File answers the stub file's path, for a caller to report.
func (p Path) File() string {
	return filepath.Join(p.directory, p.name, p.file)
}

// stubDirectory answers the stub's own directory, <directory>/<name>.
func (p Path) stubDirectory() string {
	return filepath.Join(p.directory, p.name)
}

// Write writes body as a regular file at the stub's path, never through what
// already sits there.
//
// An entry at <directory>/<name> that is not a real directory (a link, to a
// directory or not, or a file) is removed, only the entry and never a link's
// target, and a real directory is created in its place. An entry at the file
// path that is neither a regular file nor a directory, such as a link, is
// removed first, so the link's target keeps its bytes. A directory at the file
// path is refused with a *DirectoryAtFileError. An existing regular file is
// overwritten and keeps its mode.
func (p Path) Write(body []byte) error {
	if err := removeNonDirectory(p.stubDirectory()); err != nil {
		return err
	}
	full := p.File()
	info, err := os.Lstat(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if info.IsDir() {
			return &DirectoryAtFileError{Path: full}
		}
		if !info.Mode().IsRegular() {
			if err := os.Remove(full); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(p.stubDirectory(), dirMode); err != nil {
		return err
	}
	return os.WriteFile(full, body, fileMode)
}

// removeNonDirectory removes the entry at path when it exists and is not a
// real directory. Lstat never follows a link, and Remove removes the link,
// never its target. An absent path or a real directory is left for Write.
func removeNonDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && !info.IsDir() {
		return os.Remove(path)
	}
	return nil
}

// OwnedBy reports whether the stub's file is one the holder of marker wrote:
// a regular file, never a symlink, whose first line after its frontmatter
// block is byte-equal to marker. The marker anywhere else in the file does not
// count, so a hand-written file quoting it is never claimed.
//
// A file that is absent is an error, never "not owned": the caller asked about
// a file it found, and absence then says the tree moved under it.
func (p Path) OwnedBy(marker string) (bool, error) {
	full := p.File()
	info, err := os.Lstat(full)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	body, err := os.ReadFile(full) //nolint:gosec // a regular file at the place its caller named
	if err != nil {
		return false, err
	}
	_, rest, found := Frontmatter(body)
	if !found {
		return false, nil
	}
	line, _, _ := bytes.Cut(rest, []byte{'\n'})
	return string(line) == marker, nil
}

// RemoveOwnedBy removes the stub's file when OwnedBy(marker) holds, then its
// directory when that leaves the directory empty. A directory holding another
// file is kept. removed is false, and nothing is touched, when the file is not
// owned by marker.
func (p Path) RemoveOwnedBy(marker string) (removed bool, err error) {
	owned, err := p.OwnedBy(marker)
	if err != nil {
		return false, err
	}
	if !owned {
		return false, nil
	}
	if err := os.Remove(p.File()); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(p.stubDirectory())
	if err != nil {
		return true, err
	}
	if len(entries) > 0 {
		return true, nil
	}
	return true, os.Remove(p.stubDirectory())
}

// Frontmatter answers the frontmatter block that opens source, from its
// opening fence line through its closing fence line and that line's newline,
// and the bytes after it. found is false when source opens with no block, or
// its block is never closed.
func Frontmatter(source []byte) (block, rest []byte, found bool) {
	first, _, hasNewline := bytes.Cut(source, []byte{'\n'})
	if !hasNewline {
		return nil, nil, false
	}
	if !isFence(first) {
		return nil, nil, false
	}
	at := len(first) + 1
	// Bounded by len(source): each pass moves at past one line.
	for at < len(source) {
		line, _, _ := bytes.Cut(source[at:], []byte{'\n'})
		end := at + len(line)
		if end < len(source) {
			end++
		}
		if isFence(line) {
			return source[:end], source[end:], true
		}
		at = end
	}
	return nil, nil, false
}

// isFence reports whether one line is a frontmatter fence.
func isFence(line []byte) bool {
	return string(bytes.TrimSuffix(line, []byte{'\r'})) == Fence
}
