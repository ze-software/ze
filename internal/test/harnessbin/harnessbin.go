// Design: docs/architecture/testing/runner-architecture.md -- the harness binary and its variables
// Related: ../runner/runner.go -- the runner that reads these variables

// Package harnessbin names the test harness binary and resolves the variables
// that locate it.
//
// The harness is `le-test`. Until the rename completes (Phases 1 and 2 of
// plan/spec-le-subject-first-command-tree.md) every builder also writes the
// retired name `ze-test` as a hard link to the same file, and each harness
// variable is read under its `le.` key first and its retired `ze.` key second.
//
// Two registered entries, not an `Aliases` entry: env.Get reads an alias
// spelling only when the caller passes the alias key, so an alias on
// `le.test.bin` would never see an environment that sets only `ZE_TEST_BIN`.
// The retired entry carries `Deprecated`, so env.Get prints its one warning
// the first time a value is read from it.
package harnessbin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

// Name is the file name of the harness binary.
const Name = "le-test"

// RetiredName is the file name every builder also writes, as a hard link to
// Name, while callers still exec the harness by its old name.
const RetiredName = "ze-test"

// The harness variables this package owns. Each Key has a RetiredKey that is
// read when the Key is unset.
const (
	KeyTestBin        = "le.test.bin"
	RetiredKeyTestBin = "ze.test.bin"
	KeyNoBuild        = "le.test.no.build"
	RetiredKeyNoBuild = "ze.test.no.build"
)

// The environment spellings a parent process writes for a harness child.
const (
	EnvTestBin = "LE_TEST_BIN"
	EnvNoBuild = "LE_TEST_NO_BUILD"
)

var (
	_ = env.MustRegister(env.EnvEntry{Key: KeyTestBin, Type: "string", Description: "Pre-built le-test harness binary path for the test runner (absolute or repo-relative)"})
	_ = env.MustRegister(env.EnvEntry{Key: RetiredKeyTestBin, Type: "string", Deprecated: EnvTestBin, Description: "Retired spelling of le.test.bin, read when le.test.bin is unset"})
	_ = env.MustRegister(env.EnvEntry{Key: KeyNoBuild, Type: "bool", Description: "Skip the in-process go build and require pre-built test binaries"})
	_ = env.MustRegister(env.EnvEntry{Key: RetiredKeyNoBuild, Type: "bool", Deprecated: EnvNoBuild, Description: "Retired spelling of le.test.no.build, read when le.test.no.build is unset"})
)

// Setting answers a harness variable. Both keys MUST be registered. An empty
// answer means neither spelling is set.
func Setting(key, retiredKey string) string { return env.Get(answering(key, retiredKey)) }

// answering is the one resolver of a harness variable: it names the key whose
// value the caller gets. The key wins when it is set. Otherwise the retired key
// answers, and the read of it through env.Get warns once that it is
// deprecated, because the retired entry is registered with Deprecated.
func answering(key, retiredKey string) string {
	if env.Get(key) != "" {
		return key
	}
	return retiredKey
}

// TestBin answers the harness binary a caller named, or "" when none is named.
func TestBin() string { return Setting(KeyTestBin, RetiredKeyTestBin) }

// NoBuild reports whether the caller asked for pre-built binaries only.
func NoBuild() bool { return env.IsEnabled(answering(KeyNoBuild, RetiredKeyNoBuild)) }

// LinkRetired writes the retired name beside a harness artifact, as a hard
// link to it. The artifact's base name MUST start with Name: `le-test` gains
// `ze-test`, and `le-test-linux-arm64` gains `ze-test-linux-arm64`. A file
// already at the retired path is replaced, because it is a previous build.
func LinkRetired(artifact string) (string, error) {
	base := filepath.Base(artifact)
	suffix, ok := strings.CutPrefix(base, Name)
	if !ok {
		return "", errors.New("harnessbin: " + artifact + " is not a " + Name + " artifact")
	}
	retired := filepath.Join(filepath.Dir(artifact), RetiredName+suffix)
	if err := os.Remove(retired); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Link(artifact, retired); err != nil {
		return "", err
	}
	return retired, nil
}
