package config

import (
	"os"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// environmentBlockOfEveryKind is an `environment` block as ExtractEnvironment
// answers it, holding one leaf of each kind the load applies: log settings
// (one of them a level the load refuses), a nested container, a section and a
// top-level leaf.
func environmentBlockOfEveryKind() map[string]map[string]string {
	return map[string]map[string]string{
		"log":        {"level": "info", "relay": "debug", "bgp.routes": "not-a-level"},
		"exabgp.api": {"ack": "false"},
		"daemon":     {"user": "zeuser", "pid": "/run/ze.pid"},
		"":           {"pprof": ":6060"},
	}
}

// TestEnvironmentEntriesNamesEverySettingOfTheBlock proves EnvironmentEntries
// answers the block's settings under their env keys and leaves out a value
// the load refuses.
func TestEnvironmentEntriesNamesEverySettingOfTheBlock(t *testing.T) {
	entries := EnvironmentEntries(environmentBlockOfEveryKind())

	want := []string{
		"ze.log=info",
		"ze.log.relay=debug",
		"exabgp.api.ack=false",
		"ze.user=zeuser",
		"ze.pid.file=/run/ze.pid",
		"ze.pprof=:6060",
	}
	slices.Sort(entries)
	slices.Sort(want)
	if !slices.Equal(entries, want) {
		t.Errorf("EnvironmentEntries = %q, want %q", entries, want)
	}
	if none := EnvironmentEntries(nil); len(none) != 0 {
		t.Errorf("a config with no environment block answers %q, want no entry", none)
	}
}

// TestForkBeforeTheBlockIsAppliedReadsWhatAForkAfterItReads: a declared
// external plugin is forked before the full parse applies the `environment`
// block to the daemon, and is handed the block as entries laid under the
// daemon's environment. It must read what a plugin forked after the block was
// applied reads, the operator's own environment winning in both.
//
// Method: the same block, in this process. The early child's environment is
// built from the environment as it stands (env.WithDefaults over
// EnvironmentEntries, then env.ShellSafe, which is Process.inheritedEnvironment).
// The block is then applied for real, as LoadConfig applies it, and the late
// child's environment is env.ShellSafe of what the process then holds, which
// is what every fork got before the declared plugins moved ahead of the parse.
// The operator's environment sets two of the block's keys, one of them under
// its shell spelling.
//
// VALIDATES: the two environments are equal, variable for variable.
// PREVENTS: a declared external plugin running on a key's default while the
// config sets it (exabgp.api.ack for an ExaBGP bridge), and a config value
// winning over the operator's environment for a forked plugin alone.
func TestForkBeforeTheBlockIsAppliedReadsWhatAForkAfterItReads(t *testing.T) {
	resetEnvCache(t)
	for _, key := range []string{
		"ze.log", "ze.log.relay", "ze.log.bgp.routes",
		"exabgp.api.ack", "ze.user", "ze.pid.file", "ze.pprof",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	t.Setenv("ze.user", "operator")
	t.Setenv("ZE_LOG_RELAY", "warn")
	env.ResetCache()
	block := environmentBlockOfEveryKind()

	// MUTATION: drop slogutil.LogConfigEntries from EnvironmentEntries, and the
	// early child has no ZE_LOG; drop the guard of env.WithDefaults, and it
	// reads ZE_USER zeuser where the operator set operator.
	early := env.ShellSafe(env.WithDefaults(os.Environ(), EnvironmentEntries(block)))

	slogutil.ApplyLogConfig(block)
	ApplyEnvConfig(block)
	late := env.ShellSafe(os.Environ())

	slices.Sort(early)
	slices.Sort(late)
	if !slices.Equal(early, late) {
		t.Errorf("a plugin forked before the block is applied reads another environment than one forked after it\nonly before: %q\nonly after:  %q",
			entriesOnlyIn(early, late), entriesOnlyIn(late, early))
	}
	for key, want := range map[string]string{
		"exabgp.api.ack":    "false",
		"ze.user":           "operator",
		"ze.log":            "info",
		"ze.log.relay":      "warn",
		"ze.log.bgp.routes": "",
		"ze.pid.file":       "/run/ze.pid",
		"ze.pprof":          ":6060",
	} {
		if got := env.LookupIn(early, key); got != want {
			t.Errorf("a plugin forked before the block is applied reads %s as %q, want %q", key, got, want)
		}
	}
}

// entriesOnlyIn answers the entries of entries that others does not hold.
func entriesOnlyIn(entries, others []string) []string {
	var missing []string
	for _, entry := range entries {
		if !slices.Contains(others, entry) {
			missing = append(missing, entry)
		}
	}
	return missing
}
