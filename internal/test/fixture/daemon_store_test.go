// Design: docs/functional-tests.md -- the DUT's store
// Related: daemon_store.go -- daemonRootEnvironment and daemonOverrides

package fixture

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// selectorSpellings are the spellings of the store selector the tests below
// write and read: dotted, uppercase, lowercase with underscores, mixed case.
var selectorSpellings = []string{"ze.test.storage", "ZE_TEST_STORAGE", "ze_test_storage", "Ze.Test-Storage"}

// TestDaemonRootEnvironmentReplacesEveryRootSpelling proves that
// daemonRootEnvironment hands a child root as its only config root, whatever
// spelling and order base carried the inherited root in, and leaves the store
// selector exactly as base carried it.
// Method: build base environments that hold the operator root under the
// dotted, uppercase, underscore and mixed-case spellings in several orders,
// beside two spellings of the memory selector and an unrelated variable. Judge
// each result the way the child reads it, with env.LookupIn and with
// paths.DefaultConfigDir over the result, count its root entries, and check
// every selector entry and the unrelated one survive unchanged.
func TestDaemonRootEnvironmentReplacesEveryRootSpelling(t *testing.T) {
	root := t.TempDir()
	operator := t.TempDir()
	kept := []string{
		"PATH=/usr/bin",
		"ze.test.storage=" + runner.DUTStoreMemory,
		"ZE_TEST_STORAGE=" + runner.DUTStoreMemory,
	}
	orders := [][]string{
		{"ze.config.dir"},
		{"ZE_CONFIG_DIR"},
		{"ze_config_dir"},
		{"Ze.Config-Dir"},
		{"ze.config.dir", "ZE_CONFIG_DIR"},
		{"ZE_CONFIG_DIR", "ze.config.dir"},
		{"Ze.Config-Dir", "ze_config_dir", "ZE_CONFIG_DIR", "ze.config.dir"},
	}
	for _, names := range orders {
		label := strings.Join(names, ",")
		base := []string{kept[0]}
		for _, name := range names {
			base = append(base, name+"="+operator)
		}
		base = append(base, kept[1:]...)
		got := daemonRootEnvironment(base, root)

		// MUTATION: append ZE_CONFIG_DIR=root onto base instead of replacing
		// the inherited spellings, and every row keeps a second root entry,
		// and each row that carries the dotted spelling resolves the operator.
		if value := env.LookupIn(got, envConfigDirDotted); value != root {
			t.Errorf("%s: env.LookupIn reads root %q, want %q", label, value, root)
		}
		if daemonRoot := daemonConfigDirUnder(t, got); daemonRoot != root {
			t.Errorf("%s: paths.DefaultConfigDir resolves %q, want %q", label, daemonRoot, root)
		}
		roots := 0
		for _, entry := range got {
			key, value, _ := strings.Cut(entry, "=")
			if environmentKeyReading(key) != environmentKeyReading(envConfigDirDotted) {
				continue
			}
			roots++
			if value != root {
				t.Errorf("%s: root entry %q survives, want only %s=%s", label, entry, envConfigDirDotted, root)
			}
		}
		if roots != 1 {
			t.Errorf("%s: %d root entries in %v, want exactly one", label, roots, got)
		}

		// MUTATION: build the result through daemonEnvironment's disk arm, or
		// name the selector among the overrides, and the selector entries
		// below are gone or emptied.
		for _, entry := range kept {
			if !slices.Contains(got, entry) {
				t.Errorf("%s: %q is gone from %v, want it untouched", label, entry, got)
			}
		}
		if selected := env.LookupIn(got, runner.DUTStoreKey); selected != runner.DUTStoreMemory {
			t.Errorf("%s: the selector reads %q, want the inherited %q", label, selected, runner.DUTStoreMemory)
		}
	}
}

// TestDaemonOverridesKeepTheStoreOnDiskUnderTheRoot proves that
// daemonOverrides, handed to a builder that takes a map, gives the daemon root
// as its config root and no memory selector under any spelling, so the daemon
// keeps its store on disk under root as daemonEnvironment's disk arm does.
// Method: put the operator root and the memory selector in this process's
// environment under several spellings, build the child environment through
// extra1Environment and miscEnvironment, and through childEnvironment over a
// base that adds the mixed-case spellings. Judge each as the daemon reads it:
// every selector entry empty, env.LookupIn of every selector spelling empty,
// paths.DefaultConfigDir answering root, and requireDaemonRoot accepting root
// and refusing the operator root.
func TestDaemonOverridesKeepTheStoreOnDiskUnderTheRoot(t *testing.T) {
	root := t.TempDir()
	operator := t.TempDir()
	for _, name := range []string{"ze.config.dir", "ZE_CONFIG_DIR"} {
		t.Setenv(name, operator)
	}
	for _, name := range selectorSpellings[:3] {
		t.Setenv(name, runner.DUTStoreMemory)
	}
	base := append(os.Environ(), "Ze.Test-Storage="+runner.DUTStoreMemory, "Ze.Config-Dir="+operator)
	builders := []struct {
		name        string
		environment []string
	}{
		{"extra1Environment", extra1Environment(daemonOverrides(root))},
		{"miscEnvironment", miscEnvironment(daemonOverrides(root))},
		{"childEnvironment", childEnvironment(base, daemonOverrides(root))},
	}
	for _, builder := range builders {
		// MUTATION: drop ze.test.storage from daemonOverrides, and the
		// inherited memory selector survives under every spelling: the
		// daemon would keep an absent store in memory.
		for _, entry := range builder.environment {
			key, value, _ := strings.Cut(entry, "=")
			if environmentKeyReading(key) == environmentKeyReading(runner.DUTStoreKey) && value != "" {
				t.Errorf("%s: selector entry %q survives, want it empty", builder.name, entry)
			}
		}
		for _, spelling := range selectorSpellings {
			if selected := env.LookupIn(builder.environment, spelling); selected != "" {
				t.Errorf("%s: %s reads %q, want empty", builder.name, spelling, selected)
			}
		}
		if err := requireDaemonRoot(builder.environment, root); err != nil {
			t.Errorf("%s: the daemon's own root refused: %v", builder.name, err)
		}

		// MUTATION: drop the root from daemonOverrides, and the inherited
		// operator root is what the daemon resolves.
		if err := requireDaemonRoot(builder.environment, operator); err == nil {
			t.Errorf("%s: the inherited root %s accepted, want root %s", builder.name, operator, root)
		}
		if daemonRoot := daemonConfigDirUnder(t, builder.environment); daemonRoot != root {
			t.Errorf("%s: paths.DefaultConfigDir resolves %q, want %q", builder.name, daemonRoot, root)
		}
	}
}
