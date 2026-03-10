// Package slogutil provides per-subsystem logging configuration for Ze BGP.
package slogutil

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoggerDefaultWarn verifies WARN level when env var not set.
//
// VALIDATES: Subsystems default to WARN level (shows warnings and errors).
// PREVENTS: Missing important warnings/errors in production.
func TestLoggerDefaultWarn(t *testing.T) {
	// Clear any existing env vars using t.Setenv (auto-restores on test end)
	t.Setenv("ze.log.test", "")
	t.Setenv("ze_log_test", "")
	t.Setenv("ze.log", "")
	t.Setenv("ze_log", "")

	logger := Logger("test")
	require.NotNil(t, logger)

	// Default is WARN - warn and error enabled, info and debug disabled
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelError))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerExplicitDisabled verifies ze.log.server=disabled explicitly disables.
//
// VALIDATES: Explicit "disabled" value disables logging.
// PREVENTS: Ambiguity between unset and explicitly disabled.
func TestLoggerExplicitDisabled(t *testing.T) {
	t.Setenv("ze.log.server", "disabled")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestLoggerEnabledDot verifies ze.log.server=debug enables logging.
//
// VALIDATES: Dot notation enables logging at specified level.
// PREVENTS: Dot notation parsing failure.
func TestLoggerEnabledDot(t *testing.T) {
	t.Setenv("ze.log.server", "debug")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestLoggerEnabledUnderscore verifies ze_log_server=debug enables logging.
//
// VALIDATES: Underscore notation enables logging at specified level.
// PREVENTS: Underscore notation parsing failure.
func TestLoggerEnabledUnderscore(t *testing.T) {
	// Ensure dot notation not set
	t.Setenv("ze.log.server", "")
	t.Setenv("ze_log_server", "debug")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerPrecedence verifies dot notation takes precedence over underscore.
//
// VALIDATES: ze.log.x > ze_log_x > default.
// PREVENTS: Wrong env var being used when both set.
func TestLoggerPrecedence(t *testing.T) {
	// Set both, dot should win
	t.Setenv("ze.log.server", "info")
	t.Setenv("ze_log_server", "debug")

	logger := Logger("server")
	require.NotNil(t, logger)

	// Info level enabled, debug should not be (since we set info, not debug)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerSubsystemAttr verifies subsystem attribute added to logs.
//
// VALIDATES: Logger adds subsystem=<name> to all log messages.
// PREVENTS: Missing subsystem tag in output.
func TestLoggerSubsystemAttr(t *testing.T) {
	t.Setenv("ze.log.test", "info")

	var buf bytes.Buffer
	logger := LoggerWithOutput("test", "info", &buf)
	require.NotNil(t, logger)

	logger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "subsystem=test")
}

// TestParseLevelCaseInsensitive verifies debug, DEBUG, Debug all work.
//
// VALIDATES: Level parsing is case-insensitive.
// PREVENTS: Case sensitivity issues in config.
func TestParseLevelCaseInsensitive(t *testing.T) {
	tests := []string{"debug", "DEBUG", "Debug", "DeBuG"}
	for _, level := range tests {
		t.Run(level, func(t *testing.T) {
			lvl, enabled := parseLevel(level)
			assert.True(t, enabled)
			assert.Equal(t, slog.LevelDebug, lvl)
		})
	}
}

// TestParseLevelAliases verifies err/error, warn/warning both work.
//
// VALIDATES: Both short and long level names work.
// PREVENTS: User confusion about level names.
func TestParseLevelAliases(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"err", slog.LevelError},
		{"error", slog.LevelError},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lvl, enabled := parseLevel(tt.input)
			assert.True(t, enabled)
			assert.Equal(t, tt.want, lvl)
		})
	}
}

// TestLoggerLevelFiltering verifies info level filters out debug messages.
//
// VALIDATES: Level filtering works correctly.
// PREVENTS: Debug logs appearing at info level.
func TestLoggerLevelFiltering(t *testing.T) {
	t.Setenv("ze.log.server", "info")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelError))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerUnknownLevel verifies unknown level value = disabled.
//
// VALIDATES: Unknown level values disable logging.
// PREVENTS: Typos silently enabling logging.
func TestLoggerUnknownLevel(t *testing.T) {
	t.Setenv("ze.log.server", "verbose") // not a valid level

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestBackendStderr verifies ze.log.backend=stderr uses stderr.
//
// VALIDATES: Default backend is stderr.
// PREVENTS: Wrong output destination.
func TestBackendStderr(t *testing.T) {
	t.Setenv("ze.log.server", "info")
	t.Setenv("ze.log.backend", "")

	// Default should be stderr - verify by checking createHandler returns stderr handler
	handler := createHandler(slog.LevelInfo)
	require.NotNil(t, handler)
	// Can't easily verify it's stderr, but ensure it's not nil
}

// TestBackendStdout verifies ze.log.backend=stdout uses stdout.
//
// VALIDATES: stdout backend option works.
// PREVENTS: stdout option being ignored.
func TestBackendStdout(t *testing.T) {
	t.Setenv("ze.log.backend", "stdout")

	handler := createHandler(slog.LevelInfo)
	require.NotNil(t, handler)
}

// TestBackendSyslog verifies ze.log.backend=syslog creates syslog handler.
//
// VALIDATES: Syslog backend option works.
// PREVENTS: Syslog option being ignored.
func TestBackendSyslog(t *testing.T) {
	t.Setenv("ze.log.backend", "syslog")
	t.Setenv("ze.log.destination", "localhost:514")

	handler := createHandler(slog.LevelInfo)
	require.NotNil(t, handler)
}

// TestLoggerWithOutput verifies LoggerWithOutput() writes to provided writer.
//
// VALIDATES: Custom output destination works.
// PREVENTS: Logs going to wrong destination.
func TestLoggerWithOutput(t *testing.T) {
	t.Setenv("ze.log.backend", "stdout") // Should be ignored for LoggerWithOutput

	var buf bytes.Buffer
	logger := LoggerWithOutput("gr", "info", &buf)
	require.NotNil(t, logger)

	logger.Info("test")
	assert.Contains(t, buf.String(), "test")
}

// TestDiscardHandler verifies discardHandler implements slog.Handler correctly.
//
// VALIDATES: discardHandler interface compliance.
// PREVENTS: Panic when using disabled logger.
func TestDiscardHandler(t *testing.T) {
	h := discardHandler{}

	// Enabled should always return false
	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, h.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, h.Enabled(context.Background(), slog.LevelError))

	// Handle should return nil
	assert.NoError(t, h.Handle(context.Background(), slog.Record{}))

	// WithAttrs and WithGroup should return the same handler
	assert.Equal(t, h, h.WithAttrs(nil))
	assert.Equal(t, h, h.WithGroup("test"))
}

// TestParseLogLineValid verifies parsing valid slog text line.
//
// VALIDATES: ParseLogLine extracts level, msg, attrs from valid slog output.
// PREVENTS: Plugin stderr relay losing information.
func TestParseLogLineValid(t *testing.T) {
	line := `time=2025-01-18T12:00:00Z level=DEBUG msg="test message" key=value peer=192.168.1.1`

	level, msg, attrs := ParseLogLine(line)

	assert.Equal(t, slog.LevelDebug, level)
	assert.Equal(t, "test message", msg)
	assert.Contains(t, attrs, "key")
	assert.Contains(t, attrs, "value")
}

// TestParseLogLineAllLevels verifies all log levels are extracted correctly.
//
// VALIDATES: ParseLogLine handles DEBUG/INFO/WARN/ERROR levels.
// PREVENTS: Level mapping errors in relay.
func TestParseLogLineAllLevels(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{`time=... level=DEBUG msg="x"`, slog.LevelDebug},
		{`time=... level=INFO msg="x"`, slog.LevelInfo},
		{`time=... level=WARN msg="x"`, slog.LevelWarn},
		{`time=... level=ERROR msg="x"`, slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.want.String(), func(t *testing.T) {
			level, _, _ := ParseLogLine(tt.input)
			assert.Equal(t, tt.want, level)
		})
	}
}

// TestParseLogLineQuotedMsg verifies quoted message containing spaces.
//
// VALIDATES: ParseLogLine handles quoted messages correctly.
// PREVENTS: Truncated messages in relay.
func TestParseLogLineQuotedMsg(t *testing.T) {
	line := `time=2025-01-18T12:00:00Z level=INFO msg="this is a message with spaces"`

	level, msg, _ := ParseLogLine(line)

	assert.Equal(t, slog.LevelInfo, level)
	assert.Equal(t, "this is a message with spaces", msg)
}

// TestParseLogLineMalformed verifies handling of non-slog text.
//
// VALIDATES: ParseLogLine returns raw line for malformed input.
// PREVENTS: Panic/crash on unexpected plugin stderr (e.g., panic output).
func TestParseLogLineMalformed(t *testing.T) {
	tests := []string{
		"panic: runtime error: index out of range",
		"some random text without slog format",
		"",
		"level=DEBUG", // Missing msg
	}
	for _, line := range tests {
		t.Run(strings.ReplaceAll(line, " ", "_"), func(t *testing.T) {
			level, msg, attrs := ParseLogLine(line)
			// Should return LevelInfo and the raw line
			assert.Equal(t, slog.LevelInfo, level)
			if line != "" {
				assert.Equal(t, line, msg)
			}
			assert.Empty(t, attrs)
		})
	}
}

// TestParseLogLineQuotedAttrWithSpaces verifies attrs with quoted values containing spaces.
//
// VALIDATES: ParseLogLine preserves full quoted attribute values.
// PREVENTS: Relay truncating error messages at first space inside quotes.
func TestParseLogLineQuotedAttrWithSpaces(t *testing.T) {
	line := `time=2025-01-18T12:00:00Z level=ERROR msg="replay failed" peer=127.0.0.2 status="" error="rpc error: unknown command"`

	level, msg, attrs := ParseLogLine(line)

	assert.Equal(t, slog.LevelError, level)
	assert.Equal(t, "replay failed", msg)

	// Find the error attribute value
	for i := 0; i < len(attrs)-1; i += 2 {
		if attrs[i] == "error" {
			assert.Equal(t, "rpc error: unknown command", attrs[i+1],
				"quoted attribute value with spaces should be preserved intact")
			return
		}
	}
	t.Fatal("error attribute not found in parsed attrs")
}

// TestLoggerWithOutputSubsystem verifies LoggerWithOutput includes subsystem.
//
// VALIDATES: LoggerWithOutput adds subsystem attribute.
// PREVENTS: Missing subsystem in test output.
func TestLoggerWithOutputSubsystem(t *testing.T) {
	var buf bytes.Buffer
	logger := LoggerWithOutput("coordinator", "debug", &buf)

	logger.Debug("test message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "subsystem=coordinator")
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

// TestAllLevelsParsing verifies all valid level strings parse correctly.
//
// VALIDATES: All documented level strings work.
// PREVENTS: Undocumented level behavior.
func TestAllLevelsParsing(t *testing.T) {
	tests := []struct {
		input   string
		want    slog.Level
		enabled bool
	}{
		{"disabled", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, true},
		{"info", slog.LevelInfo, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"err", slog.LevelError, true},
		{"error", slog.LevelError, true},
		{"unknown", slog.LevelInfo, false},
		{"", slog.LevelInfo, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, enabled := parseLevel(tt.input)
			assert.Equal(t, tt.want, level)
			assert.Equal(t, tt.enabled, enabled)
		})
	}
}

// =============================================================================
// Hierarchical Logging Tests (ze.log.<path> convention)
// =============================================================================

// TestLoggerHierarchicalSpecificOverridesParent verifies ze.log.bgp.fsm overrides ze.log.bgp.
//
// VALIDATES: Specific subsystem env var overrides parent.
// PREVENTS: Parent level incorrectly overriding specific setting.
func TestLoggerHierarchicalSpecificOverridesParent(t *testing.T) {
	t.Setenv("ze.log.bgp", "debug")
	t.Setenv("ze.log.bgp.fsm", "warn")

	logger := Logger("bgp.fsm")
	require.NotNil(t, logger)

	// Should be warn level (specific), not debug (parent)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerHierarchicalParentLevel verifies ze.log.bgp=debug enables all bgp.* subsystems.
//
// VALIDATES: Parent level applies to all child subsystems.
// PREVENTS: Missing inheritance from parent level.
func TestLoggerHierarchicalParentLevel(t *testing.T) {
	t.Setenv("ze.log.bgp", "debug")
	// Ensure specific is not set
	t.Setenv("ze.log.bgp.fsm", "")
	t.Setenv("ze_log_bgp_fsm", "")

	logger := Logger("bgp.fsm")
	require.NotNil(t, logger)

	// Should inherit debug from parent
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerHierarchicalRootLevel verifies ze.log=debug enables all subsystems.
//
// VALIDATES: Root ze.log level applies to all subsystems.
// PREVENTS: Root level being ignored.
func TestLoggerHierarchicalRootLevel(t *testing.T) {
	t.Setenv("ze.log", "info")
	// Ensure specific and parent are not set
	t.Setenv("ze.log.server", "")
	t.Setenv("ze_log_server", "")

	logger := Logger("server")
	require.NotNil(t, logger)

	// Should inherit from root
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerHierarchicalDotOverridesUnderscore verifies dot notation wins over underscore.
//
// VALIDATES: ze.log.bgp.fsm > ze_log_bgp_fsm at same level.
// PREVENTS: Underscore incorrectly taking precedence.
func TestLoggerHierarchicalDotOverridesUnderscore(t *testing.T) {
	t.Setenv("ze.log.bgp.fsm", "warn")
	t.Setenv("ze_log_bgp_fsm", "debug")

	logger := Logger("bgp.fsm")
	require.NotNil(t, logger)

	// Dot notation should win
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerHierarchicalUnderscoreFallback verifies underscore works when dot not set.
//
// VALIDATES: ze_log_bgp_fsm works when ze.log.bgp.fsm is not set.
// PREVENTS: Underscore notation being completely ignored.
func TestLoggerHierarchicalUnderscoreFallback(t *testing.T) {
	t.Setenv("ze.log.bgp.fsm", "")
	t.Setenv("ze_log_bgp_fsm", "debug")

	logger := Logger("bgp.fsm")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerHierarchicalInvalidLevel verifies invalid level = disabled.
//
// VALIDATES: Unknown level strings disable logging.
// PREVENTS: Typos silently enabling logging.
func TestLoggerHierarchicalInvalidLevel(t *testing.T) {
	t.Setenv("ze.log.bgp.fsm", "verbose") // invalid

	logger := Logger("bgp.fsm")
	require.NotNil(t, logger)

	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestLoggerHierarchicalEmptyEnvDefaultsWarn verifies empty env = WARN level.
//
// VALIDATES: No env vars set = WARN level (default).
// PREVENTS: Missing warnings/errors when not configured.
func TestLoggerHierarchicalEmptyEnvDefaultsWarn(t *testing.T) {
	// Clear all possible env vars
	t.Setenv("ze.log", "")
	t.Setenv("ze_log", "")
	t.Setenv("ze.log.test", "")
	t.Setenv("ze_log_test", "")

	logger := Logger("test")
	require.NotNil(t, logger)

	// Default is WARN - warn enabled, info disabled
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// =============================================================================
// PluginLogger Tests (CLI + Env Var)
// =============================================================================

// TestPluginLoggerCLIOverridesEnv verifies CLI flag takes precedence over env var.
//
// VALIDATES: --log-level=warn overrides ze.log.gr=debug.
// PREVENTS: Env var incorrectly overriding explicit CLI flag.
func TestPluginLoggerCLIOverridesEnv(t *testing.T) {
	t.Setenv("ze.log.gr", "debug")

	logger := PluginLogger("gr", "warn")
	require.NotNil(t, logger)

	// CLI should win
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestPluginLoggerDisabledCLIFallsBackToEnv verifies CLI "disabled" uses env var.
//
// VALIDATES: --log-level=disabled falls back to env var.
// PREVENTS: Plugins unable to use env var when CLI is "disabled".
func TestPluginLoggerDisabledCLIFallsBackToEnv(t *testing.T) {
	t.Setenv("ze.log.gr", "debug")

	logger := PluginLogger("gr", "disabled")
	require.NotNil(t, logger)

	// Should use env var since CLI is "disabled"
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestPluginLoggerEmptyCLIFallsBackToEnv verifies empty CLI uses env var.
//
// VALIDATES: --log-level="" falls back to env var.
// PREVENTS: Empty CLI string breaking env var lookup.
func TestPluginLoggerEmptyCLIFallsBackToEnv(t *testing.T) {
	t.Setenv("ze.log.gr", "info")

	logger := PluginLogger("gr", "")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestPluginLoggerBothEmptyDefaultsWarn verifies both empty = WARN level.
//
// VALIDATES: No CLI flag and no env var = WARN level (default).
// PREVENTS: Missing warnings/errors when not configured.
func TestPluginLoggerBothEmptyDefaultsWarn(t *testing.T) {
	t.Setenv("ze.log.gr", "")
	t.Setenv("ze_log_gr", "")
	t.Setenv("ze.log", "")
	t.Setenv("ze_log", "")

	logger := PluginLogger("gr", "")
	require.NotNil(t, logger)

	// Default is WARN - warn enabled, info disabled
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestPluginLoggerHierarchicalEnv verifies plugin inherits from parent env.
//
// VALIDATES: PluginLogger("gr", "") uses ze.log hierarchy.
// PREVENTS: Plugin ignoring hierarchical env vars.
func TestPluginLoggerHierarchicalEnv(t *testing.T) {
	t.Setenv("ze.log", "warn")
	t.Setenv("ze.log.gr", "")

	logger := PluginLogger("gr", "")
	require.NotNil(t, logger)

	// Should inherit from root
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// =============================================================================
// Relay Level Tests
// =============================================================================

// TestRelayLevel verifies ze.log.relay controls relay output level.
//
// VALIDATES: RelayLevel() returns configured level.
// PREVENTS: Plugin stderr relay not respecting configured level.
func TestRelayLevel(t *testing.T) {
	t.Setenv("ze.log.relay", "debug")

	level, enabled := RelayLevel()
	assert.True(t, enabled)
	assert.Equal(t, slog.LevelDebug, level)
}

// TestRelayLevelDisabled verifies disabled relay.
//
// VALIDATES: ze.log.relay=disabled returns enabled=false.
// PREVENTS: Disabled relay still outputting.
func TestRelayLevelDisabled(t *testing.T) {
	t.Setenv("ze.log.relay", "disabled")

	_, enabled := RelayLevel()
	assert.False(t, enabled)
}

// TestRelayLevelDefault verifies unset relay = WARN level.
//
// VALIDATES: No ze.log.relay = WARN level (default).
// PREVENTS: Missing plugin warnings/errors.
func TestRelayLevelDefault(t *testing.T) {
	t.Setenv("ze.log.relay", "")
	t.Setenv("ze_log_relay", "")

	level, enabled := RelayLevel()
	assert.True(t, enabled)
	assert.Equal(t, slog.LevelWarn, level)
}

// =============================================================================
// ApplyLogConfig Tests (Config File Support)
// =============================================================================

// TestApplyLogConfigBaseLevel verifies "level" maps to ze.log.
//
// VALIDATES: Config "level" key sets ze.log env var.
// PREVENTS: Config file base level being ignored.
func TestApplyLogConfigBaseLevel(t *testing.T) {
	// Clear any existing env var
	t.Setenv("ze.log", "")

	configValues := map[string]map[string]string{
		"log": {"level": "debug"},
	}

	ApplyLogConfig(configValues)

	// Check env var was set
	assert.Equal(t, "debug", getLogEnv("anything"))
}

// TestApplyLogConfigSubsystem verifies "bgp.routes" maps to ze.log.bgp.routes.
//
// VALIDATES: Config subsystem key sets correct env var.
// PREVENTS: Subsystem config not applied.
func TestApplyLogConfigSubsystem(t *testing.T) {
	t.Setenv("ze.log.bgp.routes", "")
	t.Setenv("ze.log.bgp", "")
	t.Setenv("ze.log", "")

	configValues := map[string]map[string]string{
		"log": {"bgp.routes": "debug"},
	}

	ApplyLogConfig(configValues)

	// Check hierarchical lookup finds it
	assert.Equal(t, "debug", getLogEnv("bgp.routes"))
}

// TestApplyLogConfigBackend verifies "backend" maps to ze.log.backend.
//
// VALIDATES: Config backend key sets correct env var.
// PREVENTS: Backend config not applied.
func TestApplyLogConfigBackend(t *testing.T) {
	t.Setenv("ze.log.backend", "")

	configValues := map[string]map[string]string{
		"log": {"backend": "syslog"},
	}

	ApplyLogConfig(configValues)

	// Check special env var was set
	assert.Equal(t, "syslog", getSpecialEnv("backend"))
}

// TestApplyLogConfigDestination verifies "destination" maps to ze.log.destination.
//
// VALIDATES: Config destination key sets correct env var.
// PREVENTS: Syslog destination not applied.
func TestApplyLogConfigDestination(t *testing.T) {
	t.Setenv("ze.log.destination", "")

	configValues := map[string]map[string]string{
		"log": {"destination": "localhost:514"},
	}

	ApplyLogConfig(configValues)

	assert.Equal(t, "localhost:514", getSpecialEnv("destination"))
}

// TestApplyLogConfigRelay verifies "relay" maps to ze.log.relay.
//
// VALIDATES: Config relay key sets correct env var.
// PREVENTS: Relay level config not applied.
func TestApplyLogConfigRelay(t *testing.T) {
	t.Setenv("ze.log.relay", "")

	configValues := map[string]map[string]string{
		"log": {"relay": "info"},
	}

	ApplyLogConfig(configValues)

	assert.Equal(t, "info", getSpecialEnv("relay"))
}

// TestApplyLogConfigOSEnvOverrides verifies OS env vars take precedence.
//
// VALIDATES: OS env var > config file.
// PREVENTS: Config file overriding explicit OS env var.
func TestApplyLogConfigOSEnvOverrides(t *testing.T) {
	// Set OS env var before ApplyLogConfig
	t.Setenv("ze.log.bgp.routes", "warn")

	configValues := map[string]map[string]string{
		"log": {"bgp.routes": "debug"},
	}

	ApplyLogConfig(configValues)

	// OS env var should still be there (not overwritten)
	assert.Equal(t, "warn", getLogEnv("bgp.routes"))
}

// TestApplyLogConfigEmptyLog verifies no crash on empty log section.
//
// VALIDATES: Empty log section is handled gracefully.
// PREVENTS: Nil map panic.
func TestApplyLogConfigEmptyLog(t *testing.T) {
	// Should not panic
	ApplyLogConfig(nil)
	ApplyLogConfig(map[string]map[string]string{})
	ApplyLogConfig(map[string]map[string]string{"other": {"key": "value"}})
}

// TestApplyLogConfigInvalidLevelWarns verifies invalid level produces warning.
//
// VALIDATES: Invalid log level outputs warning to writer.
// PREVENTS: Silent acceptance of typos in config file.
func TestApplyLogConfigInvalidLevelWarns(t *testing.T) {
	t.Setenv("ze.log.badlevel", "")

	var buf bytes.Buffer
	configValues := map[string]map[string]string{
		"log": {"badlevel": "verbose"}, // invalid level
	}

	applyLogConfigTo(configValues, &buf)

	output := buf.String()

	// Should contain warning about invalid level
	assert.Contains(t, output, "warning")
	assert.Contains(t, output, "invalid log level")
	assert.Contains(t, output, "verbose")
}

// TestApplyLogConfigInvalidBackendWarns verifies invalid backend produces warning.
//
// VALIDATES: Invalid backend outputs warning to writer.
// PREVENTS: Silent acceptance of invalid backend in config file.
func TestApplyLogConfigInvalidBackendWarns(t *testing.T) {
	t.Setenv("ze.log.backend", "")

	var buf bytes.Buffer
	configValues := map[string]map[string]string{
		"log": {"backend": "file"}, // invalid backend
	}

	applyLogConfigTo(configValues, &buf)

	output := buf.String()

	// Should contain warning about invalid backend
	assert.Contains(t, output, "warning")
	assert.Contains(t, output, "invalid log backend")
	assert.Contains(t, output, "file")
}

// TestApplyLogConfigMultipleSettings verifies multiple settings work together.
//
// VALIDATES: Multiple log config settings are applied.
// PREVENTS: Only first/last setting being applied.
func TestApplyLogConfigMultipleSettings(t *testing.T) {
	t.Setenv("ze.log", "")
	t.Setenv("ze.log.bgp.routes", "")
	t.Setenv("ze.log.config", "")
	t.Setenv("ze.log.backend", "")

	configValues := map[string]map[string]string{
		"log": {
			"level":      "warn",
			"bgp.routes": "debug",
			"config":     "info",
			"backend":    "stdout",
		},
	}

	ApplyLogConfig(configValues)

	// All settings should be applied
	assert.Equal(t, "warn", getLogEnv("unknown"))       // base level
	assert.Equal(t, "debug", getLogEnv("bgp.routes"))   // specific
	assert.Equal(t, "info", getLogEnv("config"))        // specific
	assert.Equal(t, "stdout", getSpecialEnv("backend")) // special
}

// =============================================================================
// LazyLogger Tests
// =============================================================================

// TestLazyLoggerDeferredCreation verifies logger is created on first use.
//
// VALIDATES: LazyLogger defers logger creation until first call.
// PREVENTS: Package-level loggers ignoring config file settings.
func TestLazyLoggerDeferredCreation(t *testing.T) {
	// Set env var AFTER LazyLogger declaration but BEFORE first use
	t.Setenv("ze.log.lazytest", "debug")

	// Create lazy logger (doesn't read env var yet)
	lazy := LazyLogger("lazytest")

	// First call creates the logger and reads env var
	logger := lazy()
	require.NotNil(t, logger)

	// Should be debug level (from env var set after declaration)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLazyLoggerCachesResult verifies same logger instance returned.
//
// VALIDATES: LazyLogger returns same instance on subsequent calls.
// PREVENTS: Creating new logger instances on every call.
func TestLazyLoggerCachesResult(t *testing.T) {
	t.Setenv("ze.log.cachetest", "info")

	lazy := LazyLogger("cachetest")

	// Multiple calls should return same instance
	logger1 := lazy()
	logger2 := lazy()
	logger3 := lazy()

	assert.Same(t, logger1, logger2)
	assert.Same(t, logger2, logger3)
}

// TestLazyLoggerConfigFileIntegration verifies config file settings work.
//
// VALIDATES: LazyLogger picks up settings from ApplyLogConfig.
// PREVENTS: Config file log settings being ignored by engine loggers.
func TestLazyLoggerConfigFileIntegration(t *testing.T) {
	// Clear env var
	t.Setenv("ze.log.integrated", "")

	// Simulate config loading: first declare lazy logger, then apply config
	lazy := LazyLogger("integrated")

	// Apply config (simulates config file parsing)
	ApplyLogConfig(map[string]map[string]string{
		"log": {"integrated": "debug"},
	})

	// Now first use of lazy logger - should pick up config setting
	logger := lazy()
	require.NotNil(t, logger)

	// Should be debug level from config
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLazyLoggerConcurrentAccess verifies thread safety under contention.
//
// VALIDATES: Multiple goroutines calling lazy() simultaneously get same instance.
// PREVENTS: Race conditions in lazy initialization, duplicate logger creation.
func TestLazyLoggerConcurrentAccess(t *testing.T) {
	t.Setenv("ze.log.concurrent", "info")

	lazy := LazyLogger("concurrent")

	const numGoroutines = 100
	results := make(chan *slog.Logger, numGoroutines)
	start := make(chan struct{})

	// Launch goroutines that all wait for start signal
	for range numGoroutines {
		go func() {
			<-start // Wait for signal
			results <- lazy()
		}()
	}

	// Release all goroutines simultaneously
	close(start)

	// Collect all results
	var loggers []*slog.Logger
	for range numGoroutines {
		loggers = append(loggers, <-results)
	}

	// All goroutines must get the exact same logger instance
	first := loggers[0]
	require.NotNil(t, first)
	for i, l := range loggers {
		assert.Same(t, first, l, "goroutine %d got different logger instance", i)
	}
}

// =============================================================================
// LevelRegistry Tests
// =============================================================================

// TestLevelRegistryTracksLoggers verifies Logger() registers subsystem in LevelRegistry.
//
// VALIDATES: Logger() registers enabled loggers in level registry.
// PREVENTS: ListLevels() returning empty when loggers exist.
func TestLevelRegistryTracksLoggers(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.regtest", "info")
	_ = Logger("regtest")

	levels := ListLevels()
	assert.Contains(t, levels, "regtest")
	assert.Equal(t, "info", levels["regtest"])
}

// TestLevelRegistryListLevels verifies ListLevels() returns all tracked subsystems.
//
// VALIDATES: ListLevels() returns subsystem names with current levels.
// PREVENTS: Missing subsystems in list output.
func TestLevelRegistryListLevels(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.list1", "debug")
	t.Setenv("ze.log.list2", "warn")
	_ = Logger("list1")
	_ = Logger("list2")

	levels := ListLevels()
	assert.Equal(t, "debug", levels["list1"])
	assert.Equal(t, "warn", levels["list2"])
}

// TestLevelRegistrySetLevel verifies SetLevel() changes level and logger reflects it.
//
// VALIDATES: SetLevel() changes level atomically and logger output reflects new level.
// PREVENTS: SetLevel() having no effect on actual logging.
func TestLevelRegistrySetLevel(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.settest", "warn")
	logger := Logger("settest")

	// Initially warn level — debug should be disabled
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))

	// Change to debug
	err := SetLevel("settest", "debug")
	require.NoError(t, err)

	// Now debug should be enabled
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))

	// ListLevels should reflect new level
	levels := ListLevels()
	assert.Equal(t, "debug", levels["settest"])
}

// TestLevelRegistrySetLevelUnknown verifies SetLevel() for unknown subsystem returns error.
//
// VALIDATES: SetLevel() for unknown subsystem returns error.
// PREVENTS: Silent no-op when subsystem name is wrong.
func TestLevelRegistrySetLevelUnknown(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	err := SetLevel("nonexistent", "info")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown subsystem")
}

// TestLazyLoggerRegistered verifies LazyLogger() registers on first call.
//
// VALIDATES: LazyLogger() registers in level registry on first call, not at creation.
// PREVENTS: Uninitialized subsystems appearing in list.
func TestLazyLoggerRegistered(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.lazyregtest", "info")

	lazy := LazyLogger("lazyregtest")

	// Before first call — not registered
	levels := ListLevels()
	assert.NotContains(t, levels, "lazyregtest")

	// First call triggers registration
	_ = lazy()

	levels = ListLevels()
	assert.Contains(t, levels, "lazyregtest")
	assert.Equal(t, "info", levels["lazyregtest"])
}

// TestSetLevelInvalidLevel verifies SetLevel() rejects invalid level strings.
//
// VALIDATES: SetLevel() with invalid level string returns error.
// PREVENTS: Accepting typos silently.
func TestSetLevelInvalidLevel(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.invalidtest", "info")
	_ = Logger("invalidtest")

	err := SetLevel("invalidtest", "badlevel")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid level")
}

// TestDisabledLoggerNotRegistered verifies disabled loggers are not in the registry.
//
// VALIDATES: Disabled loggers (discardHandler) are NOT registered.
// PREVENTS: Users seeing disabled subsystems they can't change.
func TestDisabledLoggerNotRegistered(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.disabledregtest", "disabled")
	_ = Logger("disabledregtest")

	levels := ListLevels()
	assert.NotContains(t, levels, "disabledregtest")
}

// TestDefaultLoggerRegistered verifies Logger() with no env var registers at WARN.
//
// VALIDATES: Default (no env var) loggers are registered at WARN level.
// PREVENTS: Default loggers missing from ListLevels().
func TestDefaultLoggerRegistered(t *testing.T) {
	ResetLevelRegistry()
	defer ResetLevelRegistry()

	t.Setenv("ze.log.defaultregtest", "")
	t.Setenv("ze_log_defaultregtest", "")
	t.Setenv("ze.log", "")
	t.Setenv("ze_log", "")
	_ = Logger("defaultregtest")

	levels := ListLevels()
	assert.Contains(t, levels, "defaultregtest")
	assert.Equal(t, "warn", levels["defaultregtest"])
}
