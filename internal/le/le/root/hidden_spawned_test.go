// Related: hidden.go -- the hidden set this test pins against what the tests spawn

package leroot_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// spawnedRun matches a daemon config's plugin `run` statement that starts an
// le mock, as a functional test declares it:
// `run "le mock engine-steps ./engine-steps.json"`.
var spawnedRun = regexp.MustCompile(`^\s*run\s+"(?:[^"]*\s)?le (mock [a-z][a-z-]*)`)

// VALIDATES: AC-23 over the real registry -- an le mock that a test daemon
// spawns from its config is hidden, and every other mock is not. "Spawned" is
// read from what the functional tests declare: a `run "le mock <name>"`
// statement in a test/**/*.ci config, which is how Ze, not a person, starts
// the helper.
// PREVENTS: a spawned helper whose register.go dropped leroot.RegisterHidden,
// so help, completion and the menu offer a person a command only Ze types,
// and a mock a person starts by hand hidden from them.
func TestSpawnedMocksAreExactlyTheHiddenOnes(t *testing.T) {
	spawned := spawnedMocks(t)
	if len(spawned) == 0 {
		t.Fatal("no functional test spawns an le mock from its config, so the hidden set is unpinned")
	}
	registered := make(map[string]bool, 16)
	for _, command := range leroot.Commands() {
		if !strings.HasPrefix(command.Name, "mock ") {
			continue
		}
		registered[command.Name] = true
		if spawned[command.Name] && !leroot.Hidden(command.Name) {
			t.Errorf("`le %s` is spawned by a test daemon's config and is not hidden: its register.go MUST call leroot.RegisterHidden", command.Name)
		}
		if !spawned[command.Name] && leroot.Hidden(command.Name) {
			t.Errorf("`le %s` is hidden and no test daemon spawns it, so a person who starts it finds no help", command.Name)
		}
	}
	for name := range spawned {
		if !registered[name] {
			t.Errorf("a functional test spawns `le %s`, which le does not register", name)
		}
	}
}

// spawnedMocks answers every le mock a test/**/*.ci config starts from a
// plugin `run` statement.
func spawnedMocks(t *testing.T) map[string]bool {
	t.Helper()
	root := repositoryRoot(t)
	spawned := make(map[string]bool, 4)
	err := filepath.WalkDir(filepath.Join(root, "test"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".ci" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(string(body), "\n") {
			if match := spawnedRun.FindStringSubmatch(line); match != nil {
				spawned[match[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the functional tests: %v", err)
	}
	return spawned
}

// repositoryRoot answers the directory holding go.mod above the test's own.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's directory")
		}
		dir = parent
	}
}
