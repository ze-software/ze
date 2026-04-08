//nolint:goconst // Test values intentionally repeated for clarity
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Blank import triggers init() registration of all plugin YANG modules.
	// Needed by TestParseEnvironmentBlockApplied for the "api" environment field.

	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
)

func TestLoadEnvironmentDefaults(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	// Check daemon defaults
	if env.Daemon.User != "zeuser" {
		t.Errorf("Daemon.User = %q, want %q", env.Daemon.User, "zeuser")
	}
	if !env.Daemon.Drop {
		t.Error("Daemon.Drop should be true by default")
	}
	if env.Daemon.Umask != 0o137 {
		t.Errorf("Daemon.Umask = %o, want %o", env.Daemon.Umask, 0o137)
	}

	// Check log defaults (legacy ExaBGP boolean fields removed from LogEnv struct).
	// Remaining fields: Level, Destination, Short.
	if env.Log.Level != LogLevelInfo {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, LogLevelInfo)
	}
	if env.Log.Destination != "stdout" {
		t.Errorf("Log.Destination = %q, want %q", env.Log.Destination, "stdout")
	}
	if !env.Log.Short {
		t.Error("Log.Short should be true by default")
	}

	// Check BGP defaults (openwait: YANG default 120, ze-bgp-conf.yang)
	if env.BGP.OpenWait != 120 {
		t.Errorf("BGP.OpenWait = %d, want %d", env.BGP.OpenWait, 120)
	}

	// Check API defaults
	if !env.API.ACK {
		t.Error("API.ACK should be true by default")
	}
	if env.API.Encoder != "json" {
		t.Errorf("API.Encoder = %q, want %q", env.API.Encoder, "json")
	}
}

func TestLoadEnvironmentFromEnv(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	// Use t.Setenv for test-scoped env vars
	t.Setenv("ze.bgp.log.level", "DEBUG")
	coreenv.ResetCache()

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "DEBUG")
	}
}

func TestLoadEnvironmentUnderscoreNotation(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	// Use t.Setenv for test-scoped env vars
	t.Setenv("ze_bgp_log_level", "WARNING")
	coreenv.ResetCache()

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "WARNING" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "WARNING")
	}
}

func TestLoadEnvironmentDotPriority(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	// Dot and underscore normalize to the same cache key.
	// Setting both results in non-deterministic ordering (last os.Setenv wins
	// in the normalized cache). Use only one notation per test.
	t.Setenv("ze.bgp.log.level", "DEBUG")
	coreenv.ResetCache()

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want %q (dot notation)", env.Log.Level, "DEBUG")
	}
}

func TestOpenWaitDuration(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	dur := env.OpenWaitDuration()
	want := 120 * time.Second // YANG default: ze-bgp-conf.yang environment > bgp > openwait

	if dur != want {
		t.Errorf("OpenWaitDuration() = %v, want %v", dur, want)
	}
}

func TestSocketPath(t *testing.T) {
	t.Run("default_no_xdg", func(t *testing.T) {
		coreenv.ResetCache()
		t.Cleanup(coreenv.ResetCache)

		t.Setenv("XDG_RUNTIME_DIR", "")
		coreenv.ResetCache()

		env, err := LoadEnvironment()
		if err != nil {
			t.Fatal(err)
		}
		// Non-root without XDG_RUNTIME_DIR falls back to /tmp/ze.socket.
		want := "/tmp/ze.socket"
		if got := env.SocketPath(); got != want {
			t.Errorf("SocketPath() = %q, want %q", got, want)
		}
	})

	t.Run("xdg_runtime_dir", func(t *testing.T) {
		coreenv.ResetCache()
		t.Cleanup(coreenv.ResetCache)

		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
		coreenv.ResetCache()

		env, err := LoadEnvironment()
		if err != nil {
			t.Fatal(err)
		}
		want := "/run/user/1000/ze.socket"
		if got := env.SocketPath(); got != want {
			t.Errorf("SocketPath() = %q, want %q", got, want)
		}
	})

	t.Run("custom_socketpath", func(t *testing.T) {
		coreenv.ResetCache()
		t.Cleanup(coreenv.ResetCache)

		t.Setenv("ze.bgp.api.socketpath", "/custom/path/my.sock")
		coreenv.ResetCache()

		env, err := LoadEnvironment()
		if err != nil {
			t.Fatal(err)
		}
		want := "/custom/path/my.sock"
		if got := env.SocketPath(); got != want {
			t.Errorf("SocketPath() = %q, want %q", got, want)
		}
	})
}

// TestResolveConfigPathTraversal verifies path traversal prevention in XDG config search.
//
// VALIDATES: Config names with ../ do not escape the config directory.
// PREVENTS: Path traversal via crafted config filenames (CWE-22).
func TestResolveConfigPathTraversal(t *testing.T) {
	dir := t.TempDir()

	// Create a file outside the ze/ config dir
	outside := filepath.Join(dir, "secret.conf")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create the ze/ config dir (empty)
	zeDir := filepath.Join(dir, "ze")
	if err := os.MkdirAll(zeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", dir)
	// Clear XDG_CONFIG_DIRS to avoid matching real system files
	t.Setenv("XDG_CONFIG_DIRS", t.TempDir())

	// Attempt traversal — should NOT resolve to the secret file
	result := ResolveConfigPath("../secret.conf")
	if result == outside {
		t.Errorf("path traversal succeeded: resolved to %q", result)
	}
}

// =============================================================================
// Strict Parsing Tests (TDD - should fail until implementation)
// =============================================================================

// TestParseBoolStrict verifies strict boolean parsing with error returns.
//
// VALIDATES: Boolean values are parsed correctly with explicit error on invalid input
// PREVENTS: Silent fallback to false for invalid boolean strings like "maybe".
func TestParseBoolStrict(t *testing.T) {
	tests := []struct {
		input   string
		want    bool
		wantErr bool
	}{
		// Valid true values
		{"true", true, false},
		{"false", false, false},
		{"TRUE", true, false},
		{"False", false, false},
		{"1", true, false},
		{"0", false, false},
		{"yes", true, false},
		{"no", false, false},
		{"on", true, false},
		{"off", false, false},
		{"enable", true, false},
		{"disable", false, false},
		// Invalid values - should error
		{"maybe", false, true},
		{"", false, true},
		{"2", false, true},
		{"enabled", false, true},  // not exact match
		{"disabled", false, true}, // not exact match
		{"yep", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseBoolStrict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBoolStrict(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseBoolStrict(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseIntStrict verifies strict integer parsing with error returns.
//
// VALIDATES: Integer values are parsed correctly with explicit error on invalid input
// PREVENTS: Silent fallback to default for invalid integer strings.
func TestParseIntStrict(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"179", 179, false},
		{"65535", 65535, false},
		{"-1", -1, false},
		// Invalid
		{"abc", 0, true},
		{"", 0, true},
		{"1.5", 0, true},
		{"1,000", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseIntStrict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIntStrict(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseIntStrict(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseFloatStrict verifies strict float parsing with error returns.
//
// VALIDATES: Float values are parsed correctly with explicit error on invalid input
// PREVENTS: Silent fallback to default for invalid float strings.
func TestParseFloatStrict(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"1.5", 1.5, false},
		{"0.1", 0.1, false},
		{"10.0", 10.0, false},
		{"-1.5", -1.5, false},
		// Invalid
		{"abc", 0, true},
		{"", 0, true},
		{"1,5", 0, true}, // European notation not supported
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFloatStrict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloatStrict(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseFloatStrict(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Validation Tests (TDD - should fail until implementation)
// =============================================================================

// TestValidateLogLevel verifies log level enum validation.
//
// VALIDATES: Only valid syslog levels are accepted
// PREVENTS: Typos like "ERROR" (should be "ERR") or "WARN" silently accepted.
func TestValidateLogLevel(t *testing.T) {
	valid := []string{"DEBUG", "debug", "Debug", "INFO", "info", "NOTICE", "WARNING", "ERR", "CRITICAL"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := validateLogLevel(v); err != nil {
				t.Errorf("validateLogLevel(%q) unexpected error: %v", v, err)
			}
		})
	}

	invalid := []string{"TRACE", "ERROR", "WARN", "FATAL", "", "info ", " DEBUG"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := validateLogLevel(v); err == nil {
				t.Errorf("validateLogLevel(%q) expected error", v)
			}
		})
	}
}

// TestValidatePort verifies port validation: 179 (BGP) or >1024 (unprivileged).
//
// VALIDATES: Port 179 or 1025-65535 accepted
// PREVENTS: Privileged ports (except 179) or invalid ports silently accepted.
func TestValidatePort(t *testing.T) {
	valid := []string{"179", "1025", "1179", "65535"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := validatePort(v); err != nil {
				t.Errorf("validatePort(%q) unexpected error: %v", v, err)
			}
		})
	}

	invalid := []string{"0", "1", "178", "180", "1024", "65536", "-1", "abc", ""}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := validatePort(v); err == nil {
				t.Errorf("validatePort(%q) expected error", v)
			}
		})
	}
}

// TestValidateEncoder verifies encoder enum validation.
//
// VALIDATES: Only valid encoder values are accepted
// PREVENTS: Invalid encoders like "xml" silently accepted.
func TestValidateEncoder(t *testing.T) {
	valid := []string{"json", "JSON", "Json", "text", "TEXT", "Text"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := validateEncoder(v); err != nil {
				t.Errorf("validateEncoder(%q) unexpected error: %v", v, err)
			}
		})
	}

	invalid := []string{"xml", "yaml", "binary", ""}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := validateEncoder(v); err == nil {
				t.Errorf("validateEncoder(%q) expected error", v)
			}
		})
	}
}

// TestValidateAttempts verifies attempts range validation.
//
// VALIDATES: Attempts are in valid 0-1000 range
// PREVENTS: Unreasonable attempt values silently accepted.
func TestValidateAttempts(t *testing.T) {
	valid := []string{"0", "1", "100", "1000"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := validateAttempts(v); err != nil {
				t.Errorf("validateAttempts(%q) unexpected error: %v", v, err)
			}
		})
	}

	invalid := []string{"-1", "1001", "10000", "abc"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := validateAttempts(v); err == nil {
				t.Errorf("validateAttempts(%q) expected error", v)
			}
		})
	}
}

// TestValidateOpenWait verifies openwait range validation.
//
// VALIDATES: OpenWait is in valid 1-3600 range
// PREVENTS: Unreasonable wait times silently accepted.
func TestValidateOpenWait(t *testing.T) {
	valid := []string{"1", "60", "120", "3600"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := validateOpenWait(v); err != nil {
				t.Errorf("validateOpenWait(%q) unexpected error: %v", v, err)
			}
		})
	}

	invalid := []string{"0", "-1", "3601", "abc"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := validateOpenWait(v); err == nil {
				t.Errorf("validateOpenWait(%q) expected error", v)
			}
		})
	}
}

// TestValidateSpeed verifies reactor speed range validation.
//
// VALIDATES: Speed is in valid 0.1-10.0 range
// PREVENTS: Unreasonable speed multipliers silently accepted.
func TestValidateSpeed(t *testing.T) {
	valid := []string{"0.1", "1", "1.0", "5.5", "10", "10.0"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := validateSpeed(v); err != nil {
				t.Errorf("validateSpeed(%q) unexpected error: %v", v, err)
			}
		})
	}

	invalid := []string{"0", "0.05", "10.1", "100", "abc"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := validateSpeed(v); err == nil {
				t.Errorf("validateSpeed(%q) expected error", v)
			}
		})
	}
}

// =============================================================================
// SetConfigValue Tests (TDD - should fail until implementation)
// =============================================================================

// TestSetConfigValue verifies setting environment values via table-driven lookup.
//
// VALIDATES: Values are set correctly for all sections/options
// PREVENTS: Typos in section/option names going unnoticed.
func TestSetConfigValue(t *testing.T) {
	tests := []struct {
		section string
		option  string
		value   string
		check   func(env *Environment) bool
	}{
		{"log", "level", "DEBUG", func(e *Environment) bool { return e.Log.Level == "DEBUG" }},
		{"LOG", "LEVEL", "INFO", func(e *Environment) bool { return e.Log.Level == "INFO" }}, // case insensitive
		{"api", "encoder", "text", func(e *Environment) bool { return e.API.Encoder == "text" }},
		{"reactor", "speed", "2.5", func(e *Environment) bool { return e.Reactor.Speed == 2.5 }},
		{"daemon", "user", "zebgp", func(e *Environment) bool { return e.Daemon.User == "zebgp" }},
		{"cache", "attributes", "false", func(e *Environment) bool { return !e.Cache.Attributes }},
		{"debug", "timing", "true", func(e *Environment) bool { return e.Debug.Timing }},
	}

	for _, tt := range tests {
		t.Run(tt.section+"_"+tt.option, func(t *testing.T) {
			env := &Environment{}
			if err := env.loadDefaults(); err != nil {
				t.Fatalf("loadDefaults: %v", err)
			}

			if err := env.SetConfigValue(tt.section, tt.option, tt.value); err != nil {
				t.Errorf("SetConfigValue(%q, %q, %q) error: %v", tt.section, tt.option, tt.value, err)
				return
			}
			if !tt.check(env) {
				t.Errorf("SetConfigValue(%q, %q, %q) did not set correctly", tt.section, tt.option, tt.value)
			}
		})
	}
}

// TestSetConfigValueErrors verifies error handling for invalid inputs.
//
// VALIDATES: Errors returned for unknown sections/options and invalid values
// PREVENTS: Silent acceptance of typos or invalid configurations.
func TestSetConfigValueErrors(t *testing.T) {
	tests := []struct {
		section string
		option  string
		value   string
		errMsg  string
	}{
		{"invalid", "foo", "bar", "unknown environment section"},
		{"log", "invalid_option", "bar", "unknown option"},
		{"log", "level", "BOGUS", "invalid log level"},
		{"api", "encoder", "xml", "invalid encoder"},
	}

	for _, tt := range tests {
		t.Run(tt.section+"_"+tt.option+"_"+tt.value, func(t *testing.T) {
			env := &Environment{}
			if err := env.loadDefaults(); err != nil {
				t.Fatalf("loadDefaults: %v", err)
			}

			err := env.SetConfigValue(tt.section, tt.option, tt.value)
			if err == nil {
				t.Errorf("SetConfigValue(%q, %q, %q) expected error containing %q", tt.section, tt.option, tt.value, tt.errMsg)
				return
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
				t.Errorf("SetConfigValue(%q, %q, %q) error = %q, want containing %q", tt.section, tt.option, tt.value, err.Error(), tt.errMsg)
			}
		})
	}
}

// =============================================================================
// LoadEnvironmentWithConfig Tests (TDD - should fail until implementation)
// =============================================================================

// TestLoadEnvironmentWithConfig verifies combined config + env loading.
//
// VALIDATES: Config values are applied, OS env overrides config
// PREVENTS: Config values being ignored or env not taking priority.
func TestLoadEnvironmentWithConfig(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	cfg := map[string]map[string]string{
		"log": {"level": "DEBUG"},
	}

	env, err := LoadEnvironmentWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want DEBUG", env.Log.Level)
	}
}

// TestConfigPriorityOSEnvWins verifies OS env vars override config values.
//
// VALIDATES: OS environment variables take priority over config file values
// PREVENTS: Config file values overriding explicit OS env var settings.
func TestConfigPriorityOSEnvWins(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	t.Setenv("ze.bgp.log.level", "WARNING")
	coreenv.ResetCache()

	cfg := map[string]map[string]string{
		"log": {"level": "DEBUG"},
	}
	env, err := LoadEnvironmentWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "WARNING" {
		t.Errorf("Log.Level = %q, want WARNING (OS env priority)", env.Log.Level)
	}
}

// TestUnderscoreEnvPriorityOverConfig verifies underscore env vars override config.
//
// VALIDATES: Underscore notation env vars also beat config file values
// PREVENTS: Only dot notation being checked for overrides.
func TestUnderscoreEnvPriorityOverConfig(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	t.Setenv("ze_bgp_log_level", "ERR")
	coreenv.ResetCache()

	cfg := map[string]map[string]string{
		"log": {"level": "DEBUG"},
	}
	env, err := LoadEnvironmentWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "ERR" {
		t.Errorf("Log.Level = %q, want ERR (underscore env priority)", env.Log.Level)
	}
}

// TestAllSectionsConfig verifies all environment sections can be set via config.
//
// VALIDATES: Every section (daemon, log, tcp, bgp, cache, api, reactor, debug) works
// PREVENTS: Missing section implementations.
func TestAllSectionsConfig(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	cfg := map[string]map[string]string{
		"daemon":  {"user": "zebgp", "daemonize": "true"},
		"log":     {"level": "DEBUG", "short": "false"},
		"tcp":     {"attempts": "5"},
		"bgp":     {"openwait": "120"},
		"cache":   {"attributes": "false"},
		"api":     {"encoder": "text", "respawn": "false"},
		"reactor": {"speed": "2.0"},
		"debug":   {"timing": "true"},
	}

	env, err := LoadEnvironmentWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all sections applied
	if env.Daemon.User != "zebgp" {
		t.Error("Daemon.User")
	}
	if !env.Daemon.Daemonize {
		t.Error("Daemon.Daemonize")
	}
	if env.Log.Level != "DEBUG" {
		t.Error("Log.Level")
	}
	if env.Log.Short {
		t.Error("Log.Short should be false")
	}
	if env.TCP.Attempts != 5 {
		t.Error("TCP.Attempts")
	}
	if env.BGP.OpenWait != 120 {
		t.Error("BGP.OpenWait")
	}
	if env.Cache.Attributes {
		t.Error("Cache.Attributes should be false")
	}
	if env.API.Encoder != "text" {
		t.Error("API.Encoder")
	}
	if env.API.Respawn {
		t.Error("API.Respawn should be false")
	}
	if env.Reactor.Speed != 2.0 {
		t.Error("Reactor.Speed")
	}
	if !env.Debug.Timing {
		t.Error("Debug.Timing")
	}
}

// TestLoadEnvironmentWithConfigError verifies errors from config are propagated.
//
// VALIDATES: Invalid config values cause LoadEnvironmentWithConfig to fail
// PREVENTS: Silent acceptance of invalid configuration.
func TestLoadEnvironmentWithConfigError(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	cfg := map[string]map[string]string{
		"bgp": {"openwait": "invalid"},
	}

	_, err := LoadEnvironmentWithConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid openwait")
	}
}

// TestLoadEnvironmentWithConfigNil verifies nil config uses defaults.
//
// VALIDATES: nil config map works (uses only defaults + env vars)
// PREVENTS: Nil pointer panic.
func TestLoadEnvironmentWithConfigNil(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	env, err := LoadEnvironmentWithConfig(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should have defaults
	if env.BGP.OpenWait != 120 {
		t.Errorf("BGP.OpenWait = %d, want 120 (default)", env.BGP.OpenWait)
	}
}

// =============================================================================
// Strict Environment Loading Tests (TDD - should fail until implementation)
// =============================================================================

// TestLoadFromEnvStrictError verifies env var parse errors are propagated.
//
// VALIDATES: Invalid env var values cause startup failure
// PREVENTS: Silent fallback to defaults for typos in env vars (BREAKING CHANGE).
func TestLoadFromEnvStrictError(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	t.Setenv("ze.bgp.bgp.openwait", "not_a_number")
	coreenv.ResetCache()

	_, err := LoadEnvironmentWithConfig(nil)
	if err == nil {
		t.Error("expected error for invalid env var")
	}
	if !strings.Contains(err.Error(), "ze.bgp.bgp.openwait") {
		t.Errorf("error should mention the env var, got: %v", err)
	}
}

// TestLoadFromEnvStrictInvalidEnum verifies enum validation in env vars.
//
// VALIDATES: Invalid enum values in env vars cause startup failure
// PREVENTS: Accepting "ERROR" instead of "ERR" silently.
func TestLoadFromEnvStrictInvalidEnum(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	t.Setenv("ze.bgp.log.level", "BOGUS")
	coreenv.ResetCache()

	_, err := LoadEnvironmentWithConfig(nil)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

// =============================================================================
// Backward Compatibility Tests
// =============================================================================

// TestTCPOnceBackwardCompat verifies tcp.once sets tcp.attempts to 1.
//
// VALIDATES: Legacy tcp.once=true sets attempts=1
// PREVENTS: Breaking existing ExaBGP configs using tcp.once.
func TestTCPOnceBackwardCompat(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	t.Setenv("ze.bgp.tcp.once", "true")
	coreenv.ResetCache()

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.TCP.Attempts != 1 {
		t.Errorf("TCP.Attempts = %d, want 1 (from tcp.once)", env.TCP.Attempts)
	}
}

// TestTCPOnceDoesNotOverrideAttempts verifies tcp.once doesn't override explicit attempts.
//
// VALIDATES: Explicit tcp.attempts takes priority over tcp.once
// PREVENTS: tcp.once=true overwriting tcp.attempts=5.
func TestTCPOnceDoesNotOverrideAttempts(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	cfg := map[string]map[string]string{
		"tcp": {"attempts": "5", "once": "true"},
	}

	env, err := LoadEnvironmentWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// attempts=5 set first, once=true should not override
	if env.TCP.Attempts != 5 {
		t.Errorf("TCP.Attempts = %d, want 5 (once should not override)", env.TCP.Attempts)
	}
}

// TestTCPConnectionsBackwardCompat verifies tcp.connections is alias for tcp.attempts.
//
// VALIDATES: Legacy tcp.connections works as alias for tcp.attempts
// PREVENTS: Breaking existing ExaBGP configs using tcp.connections.
func TestTCPConnectionsBackwardCompat(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	t.Setenv("ze.bgp.tcp.connections", "3")
	coreenv.ResetCache()

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.TCP.Attempts != 3 {
		t.Errorf("TCP.Attempts = %d, want 3 (from tcp.connections)", env.TCP.Attempts)
	}
}

// =============================================================================
// Environment Block Parsing Tests
// =============================================================================

// TestParseEnvironmentBlock verifies environment block is parsed from config.
//
// VALIDATES: Environment block parsed correctly from config string.
// PREVENTS: Environment values being ignored in config parsing.
func TestParseEnvironmentBlock(t *testing.T) {
	input := `
environment {
    log {
        level DEBUG
    }
    tcp {
        attempts 5
    }
}

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.2
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Extract environment values
	envValues := ExtractEnvironment(tree)
	if envValues == nil {
		t.Fatal("ExtractEnvironment returned nil")
		return
	}

	// Check log.level
	if envValues["log"]["level"] != "DEBUG" {
		t.Errorf("log.level = %q, want DEBUG", envValues["log"]["level"])
	}

	// Check tcp.attempts
	if envValues["tcp"]["attempts"] != "5" {
		t.Errorf("tcp.attempts = %q, want 5", envValues["tcp"]["attempts"])
	}
}

// TestParseEnvironmentBlockApplied verifies environment values are applied.
//
// VALIDATES: Environment block values are applied via LoadEnvironmentWithConfig.
// PREVENTS: Config parsing without applying environment values.
func TestParseEnvironmentBlockApplied(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	input := `
environment {
    log {
        level WARNING
    }
    tcp {
        attempts 5
    }
    api {
        encoder text
    }
}

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.2
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)
	env, err := LoadEnvironmentWithConfig(envValues)
	if err != nil {
		t.Fatalf("LoadEnvironmentWithConfig error: %v", err)
	}

	if env.Log.Level != "WARNING" {
		t.Errorf("Log.Level = %q, want WARNING", env.Log.Level)
	}
	if env.TCP.Attempts != 5 {
		t.Errorf("TCP.Attempts = %d, want 5", env.TCP.Attempts)
	}
	if env.API.Encoder != "text" {
		t.Errorf("API.Encoder = %q, want text", env.API.Encoder)
	}
}

// TestParseEmptyEnvironmentBlock verifies empty environment block is valid.
//
// VALIDATES: Empty environment block doesn't cause parsing errors.
// PREVENTS: Crash on empty environment block.
func TestParseEmptyEnvironmentBlock(t *testing.T) {
	input := `
environment { }

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)
	// Empty block should return empty map or nil
	if len(envValues) != 0 {
		t.Errorf("Expected empty environment values, got %v", envValues)
	}
}

// TestParseEmptySectionInEnvironmentBlock verifies empty section within block is valid.
//
// VALIDATES: Empty section like `log { }` doesn't cause parsing errors.
// PREVENTS: Crash on empty section within environment block.
func TestParseEmptySectionInEnvironmentBlock(t *testing.T) {
	input := `
environment {
    log { }
    tcp {
        attempts 5
    }
}

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)
	if envValues == nil {
		t.Fatal("ExtractEnvironment returned nil")
		return
	}

	// Empty log section should not appear in result
	if _, ok := envValues["log"]; ok {
		t.Error("Empty log section should not be in result")
	}

	// tcp.attempts should still be present
	if envValues["tcp"]["attempts"] != "5" {
		t.Errorf("tcp.attempts = %q, want 5", envValues["tcp"]["attempts"])
	}
}

// TestParseMultipleEnvironmentBlocks verifies multiple environment blocks are merged.
//
// VALIDATES: Multiple environment blocks are merged (not overwritten).
// PREVENTS: Confusion about multiple block behavior.
func TestParseMultipleEnvironmentBlocks(t *testing.T) {
	input := `
environment {
    log {
        level DEBUG
    }
}
environment {
    tcp {
        attempts 5
    }
}

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)

	// Parser merges multiple blocks - both values should be present
	if envValues["log"]["level"] != "DEBUG" {
		t.Errorf("log.level = %q, want DEBUG (from first block)", envValues["log"]["level"])
	}
	if envValues["tcp"]["attempts"] != "5" {
		t.Errorf("tcp.attempts = %q, want 5 (from second block)", envValues["tcp"]["attempts"])
	}
}

// =============================================================================
// Compound Listen Parser Tests
// =============================================================================

// TestParseCompoundListen verifies single endpoint parsing.
//
// VALIDATES: AC-1 "ze.web.listen=0.0.0.0:3443" parsed into single endpoint
// PREVENTS: Compound listen parser failing on basic ip:port input.
func TestParseCompoundListen(t *testing.T) {
	endpoints, err := ParseCompoundListen("0.0.0.0:3443")
	if err != nil {
		t.Fatalf("ParseCompoundListen(\"0.0.0.0:3443\") error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].IP != "0.0.0.0" {
		t.Errorf("IP = %q, want %q", endpoints[0].IP, "0.0.0.0")
	}
	if endpoints[0].Port != 3443 {
		t.Errorf("Port = %d, want %d", endpoints[0].Port, 3443)
	}
	if endpoints[0].String() != "0.0.0.0:3443" {
		t.Errorf("String() = %q, want %q", endpoints[0].String(), "0.0.0.0:3443")
	}
}

// TestParseCompoundListenIPv6 verifies IPv6 bracket notation parsing.
//
// VALIDATES: AC-5 "ze.web.listen=[::1]:3443" parsed correctly
// PREVENTS: IPv6 addresses failing to parse due to colons in address.
func TestParseCompoundListenIPv6(t *testing.T) {
	endpoints, err := ParseCompoundListen("[::1]:3443")
	if err != nil {
		t.Fatalf("ParseCompoundListen(\"[::1]:3443\") error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].IP != "::1" {
		t.Errorf("IP = %q, want %q", endpoints[0].IP, "::1")
	}
	if endpoints[0].Port != 3443 {
		t.Errorf("Port = %d, want %d", endpoints[0].Port, 3443)
	}
	if endpoints[0].String() != "[::1]:3443" {
		t.Errorf("String() = %q, want %q", endpoints[0].String(), "[::1]:3443")
	}
}

// TestCompoundListenMulti verifies multi-endpoint parsing.
//
// VALIDATES: AC-2 "ze.web.listen=0.0.0.0:3443,127.0.0.1:8080" parsed into two endpoints
// PREVENTS: Compound format not supporting comma-separated multiple endpoints.
func TestCompoundListenMulti(t *testing.T) {
	endpoints, err := ParseCompoundListen("0.0.0.0:3443,127.0.0.1:8080")
	if err != nil {
		t.Fatalf("ParseCompoundListen multi error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(endpoints))
	}
	if endpoints[0].IP != "0.0.0.0" || endpoints[0].Port != 3443 {
		t.Errorf("endpoint[0] = %v, want 0.0.0.0:3443", endpoints[0])
	}
	if endpoints[1].IP != "127.0.0.1" || endpoints[1].Port != 8080 {
		t.Errorf("endpoint[1] = %v, want 127.0.0.1:8080", endpoints[1])
	}
	if endpoints[0].String() != "0.0.0.0:3443" {
		t.Errorf("endpoints[0].String() = %q, want %q", endpoints[0].String(), "0.0.0.0:3443")
	}
	if endpoints[1].String() != "127.0.0.1:8080" {
		t.Errorf("endpoints[1].String() = %q, want %q", endpoints[1].String(), "127.0.0.1:8080")
	}
}

// TestCompoundListenBoundary verifies port boundary validation.
//
// VALIDATES: Port range 1-65535 is enforced
// PREVENTS: Port 0 or 65536 being silently accepted.
func TestCompoundListenBoundary(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"port_1_valid", "0.0.0.0:1", false},
		{"port_65535_valid", "0.0.0.0:65535", false},
		{"port_0_invalid", "0.0.0.0:0", true},
		{"port_65536_invalid", "0.0.0.0:65536", true},
		{"empty_string", "", true},
		{"missing_port", "0.0.0.0", true},
		{"missing_port_colon", "0.0.0.0:", true},
		{"port_not_number", "0.0.0.0:abc", true},
		{"negative_port", "0.0.0.0:-1", true},
		{"ipv6_port_0", "[::1]:0", true},
		{"ipv6_port_65536", "[::1]:65536", true},
		{"ipv6_no_bracket_close", "[::1:3443", true},
		{"spaces_around", " 0.0.0.0:3443 ", false},
		{"multi_with_invalid", "0.0.0.0:3443,0.0.0.0:0", true},
		{"ipv6_full_valid", "[2001:db8::1]:8443", false},
		{"only_port", ":3443", false},
		{"trailing_comma", "0.0.0.0:3443,", true},
		{"leading_comma", ",0.0.0.0:3443", true},
		{"ipv6_no_colon_after_bracket", "[::1]3443", true},
		{"empty_ipv6_brackets", "[]:3443", true},
		{"hostname_not_ip", "example.com:3443", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCompoundListen(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCompoundListen(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestCompoundListenValues verifies parsed values for valid inputs.
//
// VALIDATES: Parsed IP, port, and String() output for edge-case inputs
// PREVENTS: Parser accepting input but producing wrong field values.
func TestCompoundListenValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantIP   string
		wantPort int
		wantStr  string
	}{
		{"only_port", ":3443", "", 3443, ":3443"},
		{"spaces_trimmed", " 0.0.0.0:3443 ", "0.0.0.0", 3443, "0.0.0.0:3443"},
		{"ipv6_full", "[2001:db8::1]:8443", "2001:db8::1", 8443, "[2001:db8::1]:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints, err := ParseCompoundListen(tt.input)
			if err != nil {
				t.Fatalf("ParseCompoundListen(%q) unexpected error: %v", tt.input, err)
			}
			if len(endpoints) != 1 {
				t.Fatalf("want 1 endpoint, got %d", len(endpoints))
			}
			if endpoints[0].IP != tt.wantIP {
				t.Errorf("IP = %q, want %q", endpoints[0].IP, tt.wantIP)
			}
			if endpoints[0].Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", endpoints[0].Port, tt.wantPort)
			}
			if endpoints[0].String() != tt.wantStr {
				t.Errorf("String() = %q, want %q", endpoints[0].String(), tt.wantStr)
			}
		})
	}
}

// TestParseNoEnvironmentBlock verifies config without environment block works.
//
// VALIDATES: Config without environment block uses defaults.
// PREVENTS: Panic when environment block is missing.
func TestParseNoEnvironmentBlock(t *testing.T) {
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)

	input := `
bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.2
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)
	if envValues != nil {
		t.Errorf("Expected nil environment values, got %v", envValues)
	}

	// Should still work with LoadEnvironmentWithConfig(nil)
	env, err := LoadEnvironmentWithConfig(nil)
	if err != nil {
		t.Fatalf("LoadEnvironmentWithConfig(nil) error: %v", err)
	}

	// Should have defaults
	if env.BGP.OpenWait != 120 {
		t.Errorf("BGP.OpenWait = %d, want 120 (default)", env.BGP.OpenWait)
	}
}
