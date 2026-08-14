// Design: docs/architecture/config/syntax.md -- the boot and SIGHUP load path
// Related: password_hash.go -- ApplyPasswordHashing, the transform LoadConfig runs
// Related: loader.go -- LoadConfig, the producer these tests drive

package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// loadConfigUser returns the "lab" user entry of a tree LoadConfig produced.
func loadConfigUser(t *testing.T, tree *Tree, name string) *Tree {
	t.Helper()
	system := tree.GetContainer("system")
	require.NotNil(t, system, "config carries a system container")
	auth := system.GetContainer("authentication")
	require.NotNil(t, auth, "config carries system authentication")
	entry := auth.GetList("user")[name]
	require.NotNil(t, entry, "config carries user %q", name)
	return entry
}

// warningsNaming returns the config.loader warnings whose message names path.
// The ring is process-global and shared with every other test in this package,
// so the temp path is what makes the filter exact.
func warningsNaming(path string) []string {
	var out []string
	for _, e := range slogutil.GlobalLogRing().Snapshot(0, "WARN", "config.loader") {
		if strings.Contains(e.Message, path) {
			out = append(out, e.Message)
		}
	}
	return out
}

// TestLoadConfigHashesPlaintextPassword: the boot path honors plaintext-password.
//
// VALIDATES: AC-1, AC-4 -- after LoadConfig the canonical password leaf holds a
// bcrypt hash of the operator's plaintext and the plaintext-password leaf is
// gone from the tree, so LocalAuthenticator.CheckPassword can accept the login.
//
// PREVENTS: the defect recorded in plan/journal/silent-fall-through.md on
// 2026-08-14. ApplyPasswordHashing had callers only on the editor commit path,
// so a config FILE carrying plaintext-password loaded with an empty canonical
// leaf, every login for that user was refused, and nothing said why.
func TestLoadConfigHashesPlaintextPassword(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "boot.conf")
	input := `system {
	authentication {
		user lab {
			plaintext-password "labsecret";
		}
	}
}
`

	result, err := LoadConfig(input, configPath, nil)
	require.NoError(t, err)

	lab := loadConfigUser(t, result.Tree, "lab")

	hash, ok := lab.Get("password")
	require.True(t, ok, "the canonical password leaf must be populated at load")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("labsecret")),
		"the stored hash must validate against the plaintext the operator wrote")

	_, plainOK := lab.Get("plaintext-password")
	assert.False(t, plainOK, "the ephemeral plaintext leaf must not survive the load")
}

// TestLoadConfigWarnsPlaintextRemainsOnDisk: the operator is told once.
//
// VALIDATES: AC-3 -- one warning, naming the config file, because the load path
// hashes in memory and never rewrites the operator's file: the secret is still
// on disk where they wrote it.
//
// PREVENTS: a silent transform. Hashing without a warning would leave the
// operator believing the plaintext was consumed, and leave the file readable.
func TestLoadConfigWarnsPlaintextRemainsOnDisk(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "warn.conf")
	input := `system {
	authentication {
		user lab {
			plaintext-password "labsecret";
		}
		user ops {
			plaintext-password "opssecret";
		}
	}
}
`

	_, err := LoadConfig(input, configPath, nil)
	require.NoError(t, err)

	warnings := warningsNaming(configPath)
	require.Len(t, warnings, 1, "two plaintext leaves in one file get ONE warning, not one each")
	assert.NotContains(t, warnings[0], "labsecret", "the warning must never carry the secret")
	assert.NotContains(t, warnings[0], "opssecret", "the warning must never carry the secret")
}

// TestLoadConfigLeavesHashedPasswordAlone: a hashed config pays nothing.
//
// VALIDATES: a file already carrying a bcrypt hash and no plaintext sibling is
// loaded unchanged and warns about nothing (R-5: bcrypt is deliberately slow,
// so a config without a plaintext leaf must not be re-hashed at every boot, and
// R-6: an unstable hash under system would defeat the reload diff).
//
// PREVENTS: re-hashing a hash, which would refuse every login, and a warning
// that fires for every daemon whatever the config says.
func TestLoadConfigLeavesHashedPasswordAlone(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("labsecret"), bcrypt.MinCost)
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "hashed.conf")
	input := `system {
	authentication {
		user lab {
			password "` + string(hash) + `";
		}
	}
}
`

	result, err := LoadConfig(input, configPath, nil)
	require.NoError(t, err)

	lab := loadConfigUser(t, result.Tree, "lab")
	stored, ok := lab.Get("password")
	require.True(t, ok)
	assert.Equal(t, string(hash), stored, "a hash already in the file is left byte-identical")
	assert.Empty(t, warningsNaming(configPath), "nothing was hashed, so nothing is warned about")
}

// TestLoadConfigRefusesMaskedBcryptLeaf: the load path takes BOTH halves of the pair.
//
// VALIDATES: LoadConfig runs RejectMaskedBcryptLeaves before ApplyPasswordHashing,
// which is the order both editor commit sites use
// (internal/component/cli/editor_commit.go, editor_commands.go).
//
// PREVENTS: a config file whose password leaf holds the display placeholder
// loading as if the placeholder were the hash. CheckPassword
// (internal/component/authz/auth.go) accepts a stored hash as a bearer token on a
// trusted-local transport, so the placeholder is a PUBLIC CONSTANT that would
// authenticate. Wiring only the hashing half of the pair created that hole.
func TestLoadConfigRefusesMaskedBcryptLeaf(t *testing.T) {
	// The fixture carries BOTH the masked canonical leaf and a plaintext sibling,
	// so it discriminates the ORDER and not only the presence of the guard.
	// Reject-first fails the load. Hash-first would overwrite the placeholder with
	// a fresh hash, leave nothing for the guard to find, and load clean.
	configPath := filepath.Join(t.TempDir(), "masked.conf")
	input := `system {
	authentication {
		user lab {
			password "` + SecretDataPlaceholder + `";
			plaintext-password "labsecret";
		}
	}
}
`

	_, err := LoadConfig(input, configPath, nil)
	require.Error(t, err, "a masked bcrypt leaf must not load")
	assert.Contains(t, err.Error(), "display placeholder",
		"the error must say why, not just that the config is bad")
	assert.Contains(t, err.Error(), "system.authentication.user.lab.password",
		"the error must name the leaf holding the placeholder, list key included")
}

// TestLoadConfigDropsEmptyPlaintextLeaf: the ephemeral leaf never survives.
//
// VALIDATES: AC-4 for the degenerate input. plaintext-password "" hashes to
// nothing, and the leaf is still ephemeral, so it must not reach the running
// tree where `show config` would display it.
//
// PREVENTS: an empty plaintext leaf being carried into the running config
// because the early return that skips hashing also skipped the delete.
func TestLoadConfigDropsEmptyPlaintextLeaf(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "empty.conf")
	input := `system {
	authentication {
		user lab {
			plaintext-password "";
		}
	}
}
`

	result, err := LoadConfig(input, configPath, nil)
	require.NoError(t, err)

	lab := loadConfigUser(t, result.Tree, "lab")
	_, plainOK := lab.Get("plaintext-password")
	assert.False(t, plainOK, "an empty ephemeral leaf is dropped, not carried")
	assert.Empty(t, warningsNaming(configPath),
		"nothing was hashed, so the operator is told nothing")
}

// TestLoadConfigNamesNoSourceItWasNotGiven: no invented file name.
//
// VALIDATES: a caller that passes no config path gets a warning that says so.
// Four production call sites pass "" (cmd/ze/ze_core_start.go and the managed
// Handler.Validate closure), because their config comes from a blob store or a
// hub push and has no filesystem path at all.
//
// PREVENTS: the warning naming <stdin> for a config that never came from stdin,
// which sent the operator to the wrong place to find the secret.
func TestLoadConfigNamesNoSourceItWasNotGiven(t *testing.T) {
	input := `system {
	authentication {
		user lab {
			plaintext-password "labsecret";
		}
	}
}
`

	_, err := LoadConfig(input, "", nil)
	require.NoError(t, err)

	warnings := warningsNaming("plaintext password in the loaded config")
	require.NotEmpty(t, warnings, "an unnamed source still warns")
	for _, w := range warnings {
		assert.NotContains(t, w, "<stdin>", "a caller that named no source gets no invented name")
		assert.NotContains(t, w, "labsecret", "the warning must never carry the secret")
	}
}

// TestLoadConfigMergesCLIPluginsIntoAPluginOnlyConfig: a config whose only
// top-level block is `plugin` takes the --plugin list like any other config.
//
// VALIDATES: the plugin-only shape, which commit 8d92e9fab moved off the deleted
// orchestrator runtime and onto runYANGConfig, loads with cliPlugins set, and the
// result carries both the plugin the file declares and the plugin the flag names.
//
// PREVENTS: the refusal returning. The orchestrator could not run an in-process
// plugin, so `--plugin` was refused for exactly this config shape; the daemon
// passes it to LoadConfig instead. Every other LoadConfig test here passes nil
// for cliPlugins, so the shape and the flag were never exercised together.
func TestLoadConfigMergesCLIPluginsIntoAPluginOnlyConfig(t *testing.T) {
	input := "plugin { external demo { } }\n"

	result, err := LoadConfig(input, filepath.Join(t.TempDir(), "plugin-only.conf"), []string{"extra-demo"})
	require.NoError(t, err)

	names := make(map[string]bool, len(result.Plugins))
	for _, p := range result.Plugins {
		names[p.Name] = true
	}
	assert.True(t, names["demo"], "the plugin the config declares survives the merge, got %v", names)
	assert.True(t, names["extra-demo"], "the plugin --plugin names is merged in, got %v", names)
}
