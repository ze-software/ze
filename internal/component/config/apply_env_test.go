package config

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// resetEnvCache clears the env registry cache before a test run and re-clears
// after. Tests use t.Setenv for pre-existing OS env, but ApplyEnvConfig reads
// the cache -- ResetCache forces re-population from os.Environ().
func resetEnvCache(t *testing.T) {
	t.Helper()
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// TestApplyEnvConfigDaemonPID verifies `environment { daemon { pid "/x"; } }`
// lands as env var `ze.pid.file`.
//
// VALIDATES: AC-10, AC-11 (pid-file.ci, pid-file-config.ci wiring).
// PREVENTS: PID file YANG leaf becoming a no-op if loader skips plumbing.
func TestApplyEnvConfigDaemonPID(t *testing.T) {
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"daemon": {"pid": "/tmp-safe/ze.pid"},
	})

	if got := env.Get("ze.pid.file"); got != "/tmp-safe/ze.pid" {
		t.Errorf("ze.pid.file = %q, want /tmp-safe/ze.pid", got)
	}
}

// TestApplyEnvConfigDaemonUser verifies `environment { daemon { user "u"; } }`
// lands as env var `ze.user` (same mechanism as privilege/drop.go consumes).
//
// VALIDATES: AC-12.
func TestApplyEnvConfigDaemonUser(t *testing.T) {
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"daemon": {"user": "zeuser"},
	})

	if got := env.Get("ze.user"); got != "zeuser" {
		t.Errorf("ze.user = %q, want zeuser", got)
	}
}

// TestApplyEnvConfigPprof verifies `environment { pprof ":6060"; }` (top-level
// leaf under environment) lands as env var `ze.pprof`.
//
// VALIDATES: AC-14.
func TestApplyEnvConfigPprof(t *testing.T) {
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"": {"pprof": ":6060"},
	})

	if got := env.Get("ze.pprof"); got != ":6060" {
		t.Errorf("ze.pprof = %q, want :6060", got)
	}
}

// TestApplyEnvConfigBGPOpenwait verifies `environment { bgp { openwait 60; } }`
// lands as env var `ze.bgp.openwait`.
//
// VALIDATES: AC-9.
func TestApplyEnvConfigBGPOpenwait(t *testing.T) {
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"bgp": {"openwait": "60"},
	})

	if got := env.Get("ze.bgp.openwait"); got != "60" {
		t.Errorf("ze.bgp.openwait = %q, want 60", got)
	}
}

// TestApplyEnvConfigBGPAnnounceDelay verifies `environment { bgp { announce-delay 5s; } }`
// lands as env var `ze.bgp.announce.delay`.
//
// VALIDATES: AC-13.
func TestApplyEnvConfigBGPAnnounceDelay(t *testing.T) {
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"bgp": {"announce-delay": "5s"},
	})

	if got := env.Get("ze.bgp.announce.delay"); got != "5s" {
		t.Errorf("ze.bgp.announce.delay = %q, want 5s", got)
	}
}

// TestApplyEnvConfigExabgpACK verifies `environment { exabgp { api { ack false; } } }`
// lands as OS env var `exabgp.api.ack` so the exabgp bridge subprocess can read it.
//
// VALIDATES: AC-19.
func TestApplyEnvConfigExabgpACK(t *testing.T) {
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"exabgp.api": {"ack": "false"},
	})

	if got := env.Get("exabgp.api.ack"); got != "false" {
		t.Errorf("exabgp.api.ack = %q, want false", got)
	}
}

// TestApplyEnvConfigOSWins verifies a pre-existing OS env var is NOT
// overwritten by a config-file value (same rule as slogutil.ApplyLogConfig).
//
// VALIDATES: config-design.md "OS env > config > default" priority.
func TestApplyEnvConfigOSWins(t *testing.T) {
	t.Setenv("ze.bgp.openwait", "180")
	resetEnvCache(t)

	ApplyEnvConfig(map[string]map[string]string{
		"bgp": {"openwait": "60"},
	})

	if got := env.Get("ze.bgp.openwait"); got != "180" {
		t.Errorf("ze.bgp.openwait = %q, want 180 (OS env wins)", got)
	}
}

// TestEnvPlumbingTableSectionsExtracted verifies every envPlumbingTable
// section can be produced by ExtractEnvironment. Without this check a new
// plumbing entry using a section string like "foo.bar" that ExtractEnvironment
// does not emit (missing from extractSections, or deeper than one level of
// nesting) would silently never fire.
//
// VALIDATES: the soft coupling between ExtractEnvironment and envPlumbingTable.
func TestEnvPlumbingTableSectionsExtracted(t *testing.T) {
	listed := make(map[string]bool, len(extractSections))
	for _, s := range extractSections {
		listed[s] = true
	}
	for _, p := range envPlumbingTable {
		switch {
		case p.section == "":
			// Top-level leaves under environment/; ExtractEnvironment always emits.
		case listed[p.section]:
			// Direct section.
		default:
			// Must be "<listed>.<direct-child>" (one level of nesting).
			parent, _, ok := splitSectionDot(p.section)
			if !ok || !listed[parent] {
				t.Errorf("envPlumbingTable entry %+v uses section %q that ExtractEnvironment cannot produce "+
					"(parent must be \"\" or in extractSections, nesting max 1 level)",
					p, p.section)
			}
		}
	}
}

// splitSectionDot splits "a.b" into ("a", "b", true). Returns (_, _, false)
// if the input has no dot.
func splitSectionDot(s string) (parent, child string, ok bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
