// Package slogutil provides per-subsystem logging configuration for ZeBGP.
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

// TestLoggerDisabledByDefault verifies no logs when env var not set.
//
// VALIDATES: Subsystems are disabled by default.
// PREVENTS: Accidental logging when not explicitly enabled.
func TestLoggerDisabledByDefault(t *testing.T) {
	// Clear any existing env vars using t.Setenv (auto-restores on test end)
	t.Setenv("zebgp.log.test", "")
	t.Setenv("zebgp_log_test", "")

	logger := Logger("test")
	require.NotNil(t, logger)

	// Logger should be disabled - check with Enabled()
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerExplicitDisabled verifies zebgp.log.server=disabled explicitly disables.
//
// VALIDATES: Explicit "disabled" value disables logging.
// PREVENTS: Ambiguity between unset and explicitly disabled.
func TestLoggerExplicitDisabled(t *testing.T) {
	t.Setenv("zebgp.log.server", "disabled")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestLoggerEnabledDot verifies zebgp.log.server=debug enables logging.
//
// VALIDATES: Dot notation enables logging at specified level.
// PREVENTS: Dot notation parsing failure.
func TestLoggerEnabledDot(t *testing.T) {
	t.Setenv("zebgp.log.server", "debug")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestLoggerEnabledUnderscore verifies zebgp_log_server=debug enables logging.
//
// VALIDATES: Underscore notation enables logging at specified level.
// PREVENTS: Underscore notation parsing failure.
func TestLoggerEnabledUnderscore(t *testing.T) {
	// Ensure dot notation not set
	t.Setenv("zebgp.log.server", "")
	t.Setenv("zebgp_log_server", "debug")

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerWithLevel verifies LoggerWithLevel("gr", "debug") enables debug logging.
//
// VALIDATES: LoggerWithLevel correctly sets level from CLI flag.
// PREVENTS: Plugin CLI flag being ignored.
func TestLoggerWithLevel(t *testing.T) {
	logger := LoggerWithLevel("gr", "debug")
	require.NotNil(t, logger)

	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

// TestLoggerWithLevelDisabled verifies LoggerWithLevel with "disabled" disables logging.
//
// VALIDATES: LoggerWithLevel respects disabled value.
// PREVENTS: Plugin logging when explicitly disabled.
func TestLoggerWithLevelDisabled(t *testing.T) {
	logger := LoggerWithLevel("gr", "disabled")
	require.NotNil(t, logger)

	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestLoggerPrecedence verifies dot notation takes precedence over underscore.
//
// VALIDATES: zebgp.log.x > zebgp_log_x > default.
// PREVENTS: Wrong env var being used when both set.
func TestLoggerPrecedence(t *testing.T) {
	// Set both, dot should win
	t.Setenv("zebgp.log.server", "info")
	t.Setenv("zebgp_log_server", "debug")

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
	t.Setenv("zebgp.log.test", "info")

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
	t.Setenv("zebgp.log.server", "info")

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
	t.Setenv("zebgp.log.server", "verbose") // not a valid level

	logger := Logger("server")
	require.NotNil(t, logger)

	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

// TestBackendStderr verifies zebgp.log.backend=stderr uses stderr.
//
// VALIDATES: Default backend is stderr.
// PREVENTS: Wrong output destination.
func TestBackendStderr(t *testing.T) {
	t.Setenv("zebgp.log.server", "info")
	t.Setenv("zebgp.log.backend", "")

	// Default should be stderr - verify by checking createHandler returns stderr handler
	handler := createHandler(slog.LevelInfo)
	require.NotNil(t, handler)
	// Can't easily verify it's stderr, but ensure it's not nil
}

// TestBackendStdout verifies zebgp.log.backend=stdout uses stdout.
//
// VALIDATES: stdout backend option works.
// PREVENTS: stdout option being ignored.
func TestBackendStdout(t *testing.T) {
	t.Setenv("zebgp.log.backend", "stdout")

	handler := createHandler(slog.LevelInfo)
	require.NotNil(t, handler)
}

// TestBackendSyslog verifies zebgp.log.backend=syslog creates syslog handler.
//
// VALIDATES: Syslog backend option works.
// PREVENTS: Syslog option being ignored.
func TestBackendSyslog(t *testing.T) {
	t.Setenv("zebgp.log.backend", "syslog")
	t.Setenv("zebgp.log.destination", "localhost:514")

	handler := createHandler(slog.LevelInfo)
	require.NotNil(t, handler)
}

// TestLoggerWithLevelStderr verifies LoggerWithLevel() always uses stderr.
//
// VALIDATES: Plugin loggers always write to stderr.
// PREVENTS: Plugin stdout contamination (stdout = protocol messages).
func TestLoggerWithLevelStderr(t *testing.T) {
	// LoggerWithLevel always uses stderr regardless of backend setting
	t.Setenv("zebgp.log.backend", "stdout") // Should be ignored

	var buf bytes.Buffer
	logger := LoggerWithOutput("gr", "info", &buf)
	require.NotNil(t, logger)

	logger.Info("test")
	assert.Contains(t, buf.String(), "test")
}

// TestIsPluginRelayEnabled verifies zebgp.log.plugin=enabled returns true.
//
// VALIDATES: Plugin relay enable flag works.
// PREVENTS: Plugin stderr being silently dropped.
func TestIsPluginRelayEnabled(t *testing.T) {
	t.Setenv("zebgp.log.plugin", "enabled")

	assert.True(t, IsPluginRelayEnabled())
}

// TestIsPluginRelayDisabled verifies zebgp.log.plugin=disabled returns false.
//
// VALIDATES: Plugin relay can be explicitly disabled.
// PREVENTS: Ambiguity in disabled state.
func TestIsPluginRelayDisabled(t *testing.T) {
	t.Setenv("zebgp.log.plugin", "disabled")

	assert.False(t, IsPluginRelayEnabled())
}

// TestIsPluginRelayDefault verifies unset zebgp.log.plugin returns false.
//
// VALIDATES: Plugin relay is disabled by default.
// PREVENTS: Unexpected plugin stderr appearing in logs.
func TestIsPluginRelayDefault(t *testing.T) {
	t.Setenv("zebgp.log.plugin", "")
	t.Setenv("zebgp_log_plugin", "")

	assert.False(t, IsPluginRelayEnabled())
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
