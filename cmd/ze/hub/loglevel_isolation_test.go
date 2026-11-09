// Design: docs/architecture/config/environment.md -- environment block publishes log levels to the process
// Related: config_source_repaired_test.go -- buildRepairedStore isolates it for the repaired-store runs
// Related: service_web_baseline_test.go -- the baseline test needs the level unset to prove anything

package hub

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// baseLogLevelKey is the base level a config's `environment { log { level } }`
// writes into the process (slogutil envKeyBaseLevel).
const baseLogLevelKey = "ze.log"

// isolateBaseLogLevel gives the test a process with no base log level set, and
// puts back whatever it found when the test ends.
//
// A config whose environment block names a level publishes it process-wide
// (env.Set of ze.log, written only while the key is unset), so a test that
// loads such a config leaks the level into every test that runs after it, and
// a test that asserts the block was not applied needs the key unset to prove
// anything. Every spelling env treats as the same key (case, dot or
// underscore) is saved, removed and restored, and the env cache is reset on
// both sides so env.Get reads the process as it is.
//
// Not safe for parallel tests: it changes the process environment, and
// t.Setenv refuses a parallel test.
func isolateBaseLogLevel(t *testing.T) {
	t.Helper()
	// Cleanups run last-registered first, so this reset runs after every
	// t.Setenv restore below and the next env.Get rereads the restored process.
	t.Cleanup(env.ResetCache)

	spellings := []string{baseLogLevelKey}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name == baseLogLevelKey {
			continue
		}
		if strings.EqualFold(strings.ReplaceAll(name, "_", "."), baseLogLevelKey) {
			spellings = append(spellings, name)
		}
	}
	for _, name := range spellings {
		t.Setenv(name, "") // Records the value t.Cleanup restores, set or unset.
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	env.ResetCache()
}
