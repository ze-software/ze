// Design: docs/architecture/diagnostics/crash-capture.md -- crash file persistence and rotation

package crashlog

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
)

const (
	crashFilePrefix = "crash-"
	crashFileSuffix = ".log"
	defaultKeep     = 5
	minKeep         = 1
	maxKeep         = 100
)

var crashDirCandidates = []string{
	"/perm/ze/crash",
	"/var/lib/ze/crash",
	"/tmp/ze-crash",
}

func resolveCrashDir() string {
	if explicit := env.Get("ze.crash.dir"); explicit != "" {
		if tryCandidate(explicit) {
			return explicit
		}
	}

	if configDir := paths.DefaultConfigDir(); configDir != "" {
		candidate := filepath.Join(configDir, "crash")
		if tryCandidate(candidate) {
			return candidate
		}
	}

	for _, candidate := range crashDirCandidates {
		if tryCandidate(candidate) {
			return candidate
		}
	}

	return ""
}

func tryCandidate(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	return probeWritable(dir)
}

// probeWritable answers whether this process can create a file in dir.
//
// The probe file's name is UNIQUE per call. A fixed `.probe` made two ze
// processes sharing one directory race each other: the first to finish removed
// the file the second had just created, and the second then read its own
// successful write as a failure and rejected a directory it could write to
// perfectly well. Several ze daemons share a crash directory on every test run
// and on any host that runs more than one.
//
// A leftover probe is possible when the process dies between the create and the
// remove. It is a zero-length dotfile in a crash directory, which is the
// cheapest failure available here: the alternative is a shared name, and that
// one is wrong even when nothing dies.
func probeWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".probe-")
	if err != nil {
		return false
	}
	probe := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(probe)
		return false
	}
	return os.Remove(probe) == nil
}

func parseCrashKeep() int {
	s := env.Get("ze.crash.keep")
	if s == "" {
		return defaultKeep
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultKeep
	}
	if n < minKeep {
		return minKeep
	}
	if n > maxKeep {
		return maxKeep
	}
	return n
}

func writeCrashFile(dir string, keep int, report string) {
	ts := time.Now().UTC().Format("20060102-150405")
	path := filepath.Join(dir, crashFilePrefix+ts+crashFileSuffix)

	if err := os.WriteFile(path, []byte(report), 0o600); err != nil {
		return
	}

	rotateCrashFiles(dir, keep)
}

func rotateCrashFiles(dir string, keep int) {
	files := listCrashFileNames(dir)
	if len(files) <= keep {
		return
	}

	for _, name := range files[:len(files)-keep] {
		os.Remove(filepath.Join(dir, name)) //nolint:errcheck // best-effort rotation
	}
}

func listCrashFileNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, crashFilePrefix) && strings.HasSuffix(name, crashFileSuffix) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
