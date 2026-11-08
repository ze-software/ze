package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/resolve"
)

// dutRootSpellings are the spellings env.Get reads as ze.config.dir: dotted,
// underscore, and the uppercase form a shell exports.
var dutRootSpellings = []string{"ze.config.dir", "ze_config_dir", "ZE_CONFIG_DIR"}

// TestDUTRootIsOneAcrossSpellings: seed and start of a DUT resolve one config
// root whichever spelling carries it, and an operator root inherited from the
// runner's own environment never reaches the child. Method: build each child
// environment through dutEnv, the producer runTest and runOrchestrated call,
// once from a parent holding an operator root under each spelling and once
// from a .ci declaring its own root under each spelling. The environment must
// hold exactly one spelling of the key. Loaded into this process, it must
// resolve the same root for seed (paths.DefaultConfigDir, which `ze init`
// reads), for a bare or stdin start (resolve.StoreDir("-")), and for a start
// on a config file in that root (resolve.StoreDir(<root>/router.conf)).
//
// MUTATION: (record_dut_store.go, `env.Without(environ, name)`, `env.Without(environ, name+"-unset")`): no earlier spelling is removed, so an inherited operator root survives beside the runner's and a .ci's ZE_CONFIG_DIR sits beside, and loses to, the runner's dotted root; the one-spelling count fails.
// MUTATION: (record_dut_store.go, `environ = withEntry(environ, dutConfigRootKey+"="+root)`, `environ = append(environ, dutConfigRootKey+"="+root)`): an inherited spelling of the root stays in the child, so the inherited cases count two spellings.
func TestDUTRootIsOneAcrossSpellings(t *testing.T) {
	for _, spelling := range dutRootSpellings {
		t.Run("inherited "+spelling, func(t *testing.T) {
			work := t.TempDir()
			operator := t.TempDir()
			rec, err := parseCILines(t, "cmd=foreground:seq=1:exec=true", "expect=exit:code=0")
			require.NoError(t, err)

			child := rec.dutEnv([]string{spelling + "=" + operator, "PATH=/usr/bin"}, work)

			requireOneDUTRoot(t, child, work)
		})
		t.Run("declared "+spelling, func(t *testing.T) {
			work := t.TempDir()
			declared := t.TempDir()
			rec, err := parseCILines(t,
				"option=env:var="+spelling+":value="+declared,
				"cmd=foreground:seq=1:exec=true", "expect=exit:code=0")
			require.NoError(t, err)

			child := rec.dutEnv([]string{"PATH=/usr/bin"}, work)

			requireOneDUTRoot(t, child, declared)
		})
	}
}

// TestChildEnvDropsOperatorRoot: no test child inherits the developer's config
// root under any spelling, so a child that never receives a test root (a
// parsing-suite ze, the BGP peer) cannot reach the operator's store either.
// Method: set an operator root under each spelling in this process, then
// build a child environment through childEnv, which every child goes through.
//
// MUTATION: (harness_exec.go, `return strings.EqualFold(normalized, strings.ReplaceAll(dutConfigRootKey, ".", "_"))`, `return false`): the inherited operator root reaches every child, so each spelling is found in the child environment.
func TestChildEnvDropsOperatorRoot(t *testing.T) {
	for _, spelling := range dutRootSpellings {
		t.Run(spelling, func(t *testing.T) {
			t.Setenv(spelling, t.TempDir())

			for _, entry := range childEnv() {
				name, _, _ := strings.Cut(entry, "=")
				require.False(t, env.InNamespace(name, dutConfigRootKey), "child inherited %q", entry)
			}
		})
	}
}

// requireOneDUTRoot checks that child carries the config root under one
// spelling with value want, then loads child's root into this process and
// requires seed and both forms of start to resolve want.
func requireOneDUTRoot(t *testing.T, child []string, want string) {
	t.Helper()
	var roots []string
	for _, entry := range child {
		name, value, _ := strings.Cut(entry, "=")
		if env.InNamespace(name, dutConfigRootKey) {
			roots = append(roots, entry)
			require.Equal(t, want, value, "child root entry %q", entry)
		}
	}
	require.Len(t, roots, 1, "child holds %v; seed and start can read different spellings", roots)

	for _, spelling := range dutRootSpellings {
		t.Setenv(spelling, "")
		require.NoError(t, os.Unsetenv(spelling))
	}
	name, value, _ := strings.Cut(roots[0], "=")
	require.NoError(t, os.Setenv(name, value))
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	seed := paths.DefaultConfigDir()
	require.Equal(t, want, seed, "seed root (ze init)")
	require.Equal(t, seed, resolve.StoreDir("-"), "bare or stdin start root")
	require.Equal(t, seed, resolve.StoreDir(filepath.Join(want, "router.conf")), "start root of a config in the root")
}
