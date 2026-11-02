// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route
// Related: route.go — the route a handler answers on, which decides the rule
// Related: local_data.go — the handlers that read a path the caller names
//
// caller_path.go decides which file a local-data handler may read for the
// caller it answers. On the operator's own `ze` process the path is the
// operator's to name, whatever it is, and `-` is the operator's standard
// input. On the daemon route the path names a file on the daemon's host, read
// with the daemon's identity, for a caller whose authorization (a word match
// over the command text) says nothing about filesystem locations. So the
// daemon reads only inside the directories it already reads configuration
// from, and never a standard input it does not share with the caller.

package command

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// callerPathDirs holds the function that answers the directories the daemon
// route may read a named file from. The config owner registers it, because the
// command layer knows no configuration location of its own.
var callerPathDirs struct {
	mu     sync.RWMutex
	source func() []string
}

// RegisterCallerPathDirs names the function that answers the directories a
// daemon-route handler may read a caller-named file from. It is called once,
// from the init of the package that owns the configuration locations; a second
// registration is a Ze defect and panics.
func RegisterCallerPathDirs(source func() []string) {
	callerPathDirs.mu.Lock()
	defer callerPathDirs.mu.Unlock()
	if source == nil {
		panic("BUG: command.RegisterCallerPathDirs: nil source")
	}
	if callerPathDirs.source != nil {
		panic("BUG: command.RegisterCallerPathDirs: registered twice")
	}
	callerPathDirs.source = source
}

// confinedDirs answers the registered directories, each made absolute and
// with its symbolic links resolved, so a resolved file path can be compared
// with them. A directory that is the filesystem root is left out: confining a
// read to `/` confines nothing, and a daemon started from `/` has it as its
// working directory. A directory that does not exist is left out too, since no
// file is inside it. No registration answers no directory, so every path is
// refused rather than every path allowed.
func confinedDirs() []string {
	callerPathDirs.mu.RLock()
	source := callerPathDirs.source
	callerPathDirs.mu.RUnlock()
	if source == nil {
		return nil
	}
	var dirs []string
	for _, dir := range source() {
		if dir == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue // a directory that does not exist holds no file
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			continue
		}
		if filepath.Dir(resolved) == resolved {
			continue // the filesystem root confines nothing
		}
		if slices.Contains(dirs, resolved) {
			continue
		}
		dirs = append(dirs, resolved)
	}
	return dirs
}

// CallerPath answers the path a local-data handler reads for the file its
// caller named, or the refusal the handler answers instead.
//
// On RouteLocal the path is answered unchanged, `-` included: the operator's
// own process reads the operator's files and standard input. On every other
// route `-` is refused, and a named path is answered only when, made absolute
// and with every symbolic link resolved, it lies inside one of the registered
// directories; the resolved path is answered, so the read cannot follow a link
// out again. A path outside them, a `..` that climbs out, a link that points
// out and a file that does not exist are refused alike, so the refusal tells a
// caller nothing about files outside the directories.
//
// commandPath and tokens are the command and the words the caller typed after
// it: the refusal names the local `ze` command that reads any file.
func CallerPath(route Route, commandPath string, tokens []string, path string) (string, error) {
	if route.CallerLocal() {
		return path, nil
	}
	if cliio.IsStdin(path) {
		return "", callerPathRefusal(commandPath, tokens,
			"reads standard input, which reaches only the ze process the operator runs")
	}
	dirs := confinedDirs()
	resolved, err := resolveCallerPath(path)
	if err != nil {
		return "", callerPathOutside(commandPath, tokens, path, dirs)
	}
	for _, dir := range dirs {
		if inside(dir, resolved) {
			return resolved, nil
		}
	}
	return "", callerPathOutside(commandPath, tokens, path, dirs)
}

// resolveCallerPath makes path absolute, against the daemon's working
// directory, and resolves every symbolic link in it.
func resolveCallerPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	return resolved, nil
}

// inside reports whether path lies under dir. Both are absolute and clean.
func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// callerPathOutside is the refusal of a named path the daemon route will not
// read. It names the directories the daemon does read from, so the operator
// can move the file there, and the local command that reads any file.
func callerPathOutside(commandPath string, tokens []string, path string, dirs []string) error {
	var reason textbuf.Buffer
	reason.Str("reads ").Quoted(path).Str(" on the daemon's host, and the daemon reads only a file inside ")
	if len(dirs) == 0 {
		reason.Str("the configuration directories, none of which exists on this host")
	} else {
		reason.Str("the configuration directories (").Str(strings.Join(dirs, ", ")).Byte(')')
	}
	return callerPathRefusal(commandPath, tokens, reason.String())
}

// callerPathRefusal builds the one-line refusal both cases share: what the
// command needs, and the local `ze` command to run where the input is.
func callerPathRefusal(commandPath string, tokens []string, reason string) error {
	var tb textbuf.Buffer
	tb.Str(commandPath).Byte(' ').Str(reason).Str("; run 'ze ").Str(commandPath)
	for _, token := range tokens {
		tb.Byte(' ').Str(token)
	}
	tb.Str("' on the host that holds the file")
	return errors.New(tb.String())
}
