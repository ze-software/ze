//nolint:goconst // Test values intentionally repeated for clarity
package config

import (
	"strings"
	"testing"
)

func TestLoadEnvironmentDefaults(t *testing.T) {
	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	// Check daemon defaults
	if env.Daemon.User != "nobody" {
		t.Errorf("Daemon.User = %q, want %q", env.Daemon.User, "nobody")
	}
	if !env.Daemon.Drop {
		t.Error("Daemon.Drop should be true by default")
	}
	if env.Daemon.Umask != 0o137 {
		t.Errorf("Daemon.Umask = %o, want %o", env.Daemon.Umask, 0o137)
	}

	// Check log defaults
	if !env.Log.Enable {
		t.Error("Log.Enable should be true by default")
	}
	if env.Log.Level != LogLevelInfo {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, LogLevelInfo)
	}
	if env.Log.Destination != "stdout" {
		t.Errorf("Log.Destination = %q, want %q", env.Log.Destination, "stdout")
	}
	if !env.Log.Short {
		t.Error("Log.Short should be true by default")
	}

	// Check TCP defaults
	if env.TCP.Port != 179 {
		t.Errorf("TCP.Port = %d, want %d", env.TCP.Port, 179)
	}

	// Check BGP defaults
	if env.BGP.OpenWait != 60 {
		t.Errorf("BGP.OpenWait = %d, want %d", env.BGP.OpenWait, 60)
	}

	// Check API defaults
	if !env.API.ACK {
		t.Error("API.ACK should be true by default")
	}
	if env.API.Encoder != "json" {
		t.Errorf("API.Encoder = %q, want %q", env.API.Encoder, "json")
	}
	const defaultSocketName = "ze-bgp"
	if env.API.SocketName != defaultSocketName {
		t.Errorf("API.SocketName = %q, want %q", env.API.SocketName, defaultSocketName)
	}
}

func TestLoadEnvironmentFromEnv(t *testing.T) {
	// Use t.Setenv for test-scoped env vars
	t.Setenv("ze.bgp.log.level", "DEBUG")
	t.Setenv("ze.bgp.tcp.port", "1179")
	t.Setenv("ze.bgp.bgp.passive", "true")
	t.Setenv("ze.bgp.api.socketname", "test-socket")

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "DEBUG")
	}
	if env.TCP.Port != 1179 {
		t.Errorf("TCP.Port = %d, want %d", env.TCP.Port, 1179)
	}
	if !env.BGP.Passive {
		t.Error("BGP.Passive should be true")
	}
	if env.API.SocketName != "test-socket" {
		t.Errorf("API.SocketName = %q, want %q", env.API.SocketName, "test-socket")
	}
}

func TestLoadEnvironmentUnderscoreNotation(t *testing.T) {
	// Use t.Setenv for test-scoped env vars
	t.Setenv("ze_bgp_log_level", "WARNING")
	t.Setenv("ze_bgp_tcp_port", "2179")

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "WARNING" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "WARNING")
	}
	if env.TCP.Port != 2179 {
		t.Errorf("TCP.Port = %d, want %d", env.TCP.Port, 2179)
	}
}

func TestLoadEnvironmentDotPriority(t *testing.T) {
	// Set both notations - dot should take priority
	t.Setenv("ze.bgp.log.level", "DEBUG")
	t.Setenv("ze_bgp_log_level", "WARNING")

	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want %q (dot notation should take priority)", env.Log.Level, "DEBUG")
	}
}

func TestLoadEnvironmentBooleanValues(t *testing.T) {
	// Test valid boolean values only - invalid values now error (BREAKING CHANGE)
	tests := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"on", true},
		{"enable", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"disable", false},
		// NOTE: "random" removed - now returns error (strict validation)
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("ze.bgp.bgp.passive", tt.value)
			env, err := LoadEnvironment()
			if err != nil {
				t.Fatalf("LoadEnvironment() error = %v", err)
			}

			if env.BGP.Passive != tt.want {
				t.Errorf("BGP.Passive with %q = %v, want %v", tt.value, env.BGP.Passive, tt.want)
			}
		})
	}
}

func TestLoadEnvironmentBooleanInvalidValue(t *testing.T) {
	// Invalid boolean values now error (BREAKING CHANGE from silent false)
	t.Setenv("ze.bgp.bgp.passive", "random")

	_, err := LoadEnvironment()
	if err == nil {
		t.Error("expected error for invalid boolean value 'random'")
	}
}

func TestOpenWaitDuration(t *testing.T) {
	env, err := LoadEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	dur := env.OpenWaitDuration()
	want := 60 * 1000 * 1000 * 1000 // 60 seconds in nanoseconds

	if int64(dur) != int64(want) {
		t.Errorf("OpenWaitDuration() = %v, want 60s", dur)
	}
}

func TestSocketPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		env, err := LoadEnvironment()
		if err != nil {
			t.Fatal(err)
		}
		if env.SocketPath() != "/var/run/ze-bgp.sock" {
			t.Errorf("SocketPath() = %q, want %q", env.SocketPath(), "/var/run/ze-bgp.sock")
		}
	})

	t.Run("custom", func(t *testing.T) {
		t.Setenv("ze.bgp.api.socketname", "custom")
		env, err := LoadEnvironment()
		if err != nil {
			t.Fatal(err)
		}
		if env.SocketPath() != "/var/run/custom.sock" {
			t.Errorf("SocketPath() = %q, want %q", env.SocketPath(), "/var/run/custom.sock")
		}
	})
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
			got, err := parseBoolStrict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBoolStrict(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseBoolStrict(%q) = %v, want %v", tt.input, got, tt.want)
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
		{"tcp", "port", "1179", func(e *Environment) bool { return e.TCP.Port == 1179 }},
		{"bgp", "passive", "true", func(e *Environment) bool { return e.BGP.Passive }},
		{"bgp", "passive", "false", func(e *Environment) bool { return !e.BGP.Passive }},
		{"api", "encoder", "text", func(e *Environment) bool { return e.API.Encoder == "text" }},
		{"reactor", "speed", "2.5", func(e *Environment) bool { return e.Reactor.Speed == 2.5 }},
		{"daemon", "user", "zebgp", func(e *Environment) bool { return e.Daemon.User == "zebgp" }},
		{"cache", "attributes", "false", func(e *Environment) bool { return !e.Cache.Attributes }},
		{"debug", "timing", "true", func(e *Environment) bool { return e.Debug.Timing }},
	}

	for _, tt := range tests {
		t.Run(tt.section+"_"+tt.option, func(t *testing.T) {
			env := &Environment{}
			env.loadDefaults()

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
		{"tcp", "port", "abc", "invalid"},
		{"tcp", "port", "99999", "invalid"},
		{"tcp", "port", "0", "invalid"},
		{"tcp", "port", "1024", "invalid"}, // privileged port (not 179)
		{"log", "level", "BOGUS", "invalid log level"},
		{"api", "encoder", "xml", "invalid encoder"},
		{"bgp", "passive", "maybe", "invalid boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.section+"_"+tt.option+"_"+tt.value, func(t *testing.T) {
			env := &Environment{}
			env.loadDefaults()

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
	cfg := map[string]map[string]string{
		"log": {"level": "DEBUG"},
		"tcp": {"port": "1179"},
	}

	env, err := LoadEnvironmentWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want DEBUG", env.Log.Level)
	}
	if env.TCP.Port != 1179 {
		t.Errorf("TCP.Port = %d, want 1179", env.TCP.Port)
	}
}

// TestConfigPriorityOSEnvWins verifies OS env vars override config values.
//
// VALIDATES: OS environment variables take priority over config file values
// PREVENTS: Config file values overriding explicit OS env var settings.
func TestConfigPriorityOSEnvWins(t *testing.T) {
	t.Setenv("ze.bgp.log.level", "WARNING")

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
	t.Setenv("ze_bgp_log_level", "ERR")

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
	cfg := map[string]map[string]string{
		"daemon":  {"user": "zebgp", "daemonize": "true"},
		"log":     {"level": "DEBUG", "short": "false"},
		"tcp":     {"port": "1179", "attempts": "5"},
		"bgp":     {"passive": "true", "openwait": "120"},
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
	if env.TCP.Port != 1179 {
		t.Error("TCP.Port")
	}
	if env.TCP.Attempts != 5 {
		t.Error("TCP.Attempts")
	}
	if !env.BGP.Passive {
		t.Error("BGP.Passive")
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
	cfg := map[string]map[string]string{
		"tcp": {"port": "invalid"},
	}

	_, err := LoadEnvironmentWithConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

// TestLoadEnvironmentWithConfigNil verifies nil config uses defaults.
//
// VALIDATES: nil config map works (uses only defaults + env vars)
// PREVENTS: Nil pointer panic.
func TestLoadEnvironmentWithConfigNil(t *testing.T) {
	env, err := LoadEnvironmentWithConfig(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should have defaults
	if env.TCP.Port != 179 {
		t.Errorf("TCP.Port = %d, want 179 (default)", env.TCP.Port)
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
	t.Setenv("ze.bgp.tcp.port", "not_a_number")

	_, err := LoadEnvironmentWithConfig(nil)
	if err == nil {
		t.Error("expected error for invalid env var")
	}
	if !strings.Contains(err.Error(), "ze.bgp.tcp.port") {
		t.Errorf("error should mention the env var, got: %v", err)
	}
}

// TestLoadFromEnvStrictInvalidEnum verifies enum validation in env vars.
//
// VALIDATES: Invalid enum values in env vars cause startup failure
// PREVENTS: Accepting "ERROR" instead of "ERR" silently.
func TestLoadFromEnvStrictInvalidEnum(t *testing.T) {
	t.Setenv("ze.bgp.log.level", "BOGUS")

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
	t.Setenv("ze.bgp.tcp.once", "true")

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
	t.Setenv("ze.bgp.tcp.connections", "3")

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
        level DEBUG;
    }
    tcp {
        port 1179;
    }
}

bgp {
    router-id 192.0.2.1;
    local-as 65000;
    peer 192.0.2.2 {
        peer-as 65001;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Extract environment values
	envValues := ExtractEnvironment(tree)
	if envValues == nil {
		t.Fatal("ExtractEnvironment returned nil")
	}

	// Check log.level
	if envValues["log"]["level"] != "DEBUG" {
		t.Errorf("log.level = %q, want DEBUG", envValues["log"]["level"])
	}

	// Check tcp.port
	if envValues["tcp"]["port"] != "1179" {
		t.Errorf("tcp.port = %q, want 1179", envValues["tcp"]["port"])
	}
}

// TestParseEnvironmentBlockApplied verifies environment values are applied.
//
// VALIDATES: Environment block values are applied via LoadEnvironmentWithConfig.
// PREVENTS: Config parsing without applying environment values.
func TestParseEnvironmentBlockApplied(t *testing.T) {
	input := `
environment {
    log {
        level WARNING;
    }
    tcp {
        port 1179;
    }
    api {
        encoder text;
    }
}

bgp {
    router-id 192.0.2.1;
    local-as 65000;
    peer 192.0.2.2 {
        peer-as 65001;
    }
}
`
	p := NewParser(YANGSchema())
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
	if env.TCP.Port != 1179 {
		t.Errorf("TCP.Port = %d, want 1179", env.TCP.Port)
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
    router-id 192.0.2.1;
    local-as 65000;
}
`
	p := NewParser(YANGSchema())
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
        port 1179;
    }
}

bgp {
    router-id 192.0.2.1;
    local-as 65000;
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)
	if envValues == nil {
		t.Fatal("ExtractEnvironment returned nil")
	}

	// Empty log section should not appear in result
	if _, ok := envValues["log"]; ok {
		t.Error("Empty log section should not be in result")
	}

	// tcp.port should still be present
	if envValues["tcp"]["port"] != "1179" {
		t.Errorf("tcp.port = %q, want 1179", envValues["tcp"]["port"])
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
        level DEBUG;
    }
}
environment {
    tcp {
        port 1179;
    }
}

bgp {
    router-id 192.0.2.1;
    local-as 65000;
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	envValues := ExtractEnvironment(tree)

	// Parser merges multiple blocks - both values should be present
	if envValues["log"]["level"] != "DEBUG" {
		t.Errorf("log.level = %q, want DEBUG (from first block)", envValues["log"]["level"])
	}
	if envValues["tcp"]["port"] != "1179" {
		t.Errorf("tcp.port = %q, want 1179 (from second block)", envValues["tcp"]["port"])
	}
}

// TestParseNoEnvironmentBlock verifies config without environment block works.
//
// VALIDATES: Config without environment block uses defaults.
// PREVENTS: Panic when environment block is missing.
func TestParseNoEnvironmentBlock(t *testing.T) {
	input := `
bgp {
    router-id 192.0.2.1;
    local-as 65000;
    peer 192.0.2.2 {
        peer-as 65001;
    }
}
`
	p := NewParser(YANGSchema())
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
	if env.TCP.Port != 179 {
		t.Errorf("TCP.Port = %d, want 179 (default)", env.TCP.Port)
	}
}
