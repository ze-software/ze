package slogutil

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

// TestColorizeLineLevelColors verifies each level gets the correct color.
//
// VALIDATES: Level values are wrapped in severity-specific ANSI colors.
// PREVENTS: All levels looking the same in terminal output.
func TestColorizeLineLevelColors(t *testing.T) {
	tests := []struct {
		level     slog.Level
		label     string
		wantColor string
	}{
		{slog.LevelDebug, "DEBUG", ""},
		{slog.LevelInfo, "INFO", ""},
		{slog.LevelWarn, "WARN", ansiLogPalette.warning},
		{slog.LevelError, "ERROR", ansiLogPalette.failure},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			line := "time=2025-01-18T12:00:00Z level=" + tt.label + " msg=test\n"
			result := ansiLogPalette.colorizeLine(line, tt.level)
			if tt.wantColor == "" {
				assert.Contains(t, result, ansiLogPalette.key+"level="+ansiReset+tt.label+" ")
				return
			}
			assert.Contains(t, result, tt.wantColor+tt.label+ansiReset)
		})
	}
}

// TestColorizeLineStylesKeys verifies all key= prefixes are blue.
//
// VALIDATES: Key prefixes have a consistent foreground color.
// PREVENTS: Keys and values having identical styling.
func TestColorizeLineStylesKeys(t *testing.T) {
	line := "time=2025-01-18T12:00:00Z level=INFO msg=hello subsystem=bgp peer=192.168.1.1\n"
	result := ansiLogPalette.colorizeLine(line, slog.LevelInfo)

	assert.Contains(t, result, ansiLogPalette.key+"time="+ansiReset)
	assert.Contains(t, result, ansiLogPalette.key+"msg="+ansiReset)
	assert.Contains(t, result, ansiLogPalette.key+"subsystem="+ansiReset)
	assert.Contains(t, result, ansiLogPalette.key+"peer="+ansiReset)
}

// TestColorizeLinePreservesValues verifies value styling preserves the text.
//
// VALIDATES: The date and clock use separate colors and the timestamp retains its text.
// PREVENTS: Values being garbled or wrapped in unexpected codes.
func TestColorizeLinePreservesValues(t *testing.T) {
	line := `time=2025-01-18T12:00:00.123+05:30 level=INFO msg="hello world" subsystem=bgp` + "\n"
	result := ansiLogPalette.colorizeLine(line, slog.LevelInfo)

	assert.Contains(t, result, ansiLogPalette.date+"2025-01-18"+ansiReset+"T"+ansiLogPalette.clock+"12:00:00.123"+ansiReset+"+05:30")
	assert.Contains(t, result, ansiLogPalette.key+"msg="+ansiReset+`"hello world"`)
	assert.Contains(t, result, ansiLogPalette.key+"subsystem="+ansiReset+ansiLogPalette.identity+"bgp"+ansiReset)
	assert.Equal(t, line, ansi.Strip(result))
}

func TestColorizeLineStylesStateAndQuotedKeys(t *testing.T) {
	line := `level=WARN msg="status=down" status=down "a=b"=42` + "\n"
	result := ansiLogPalette.colorizeLine(line, slog.LevelWarn)
	assert.Contains(t, result, ansiLogPalette.key+"msg="+ansiReset+`"status=down"`)
	assert.Contains(t, result, ansiLogPalette.failure+"down"+ansiReset)
	assert.Contains(t, result, ansiLogPalette.key+`"a=b"=`+ansiReset+"42")
}

// VALIDATES: Plugin stderr relays recover the severity and data of a colored record.
func TestParseColoredLogRecord(t *testing.T) {
	palette := logPaletteFor("15;0", colorprofile.TrueColor)
	line := palette.colorizeLine(`level=WARN msg="peer changed" state=down`, slog.LevelWarn)
	level, message, attrs := ParseLogLine(line)
	assert.Equal(t, slog.LevelWarn, level)
	assert.Equal(t, "peer changed", message)
	assert.Equal(t, []any{"state", "down"}, attrs)
}

// VALIDATES: Log colors follow the advertised background and color depth.
func TestLogPaletteFollowsTerminalProfile(t *testing.T) {
	dark := logPaletteFor("15;0", colorprofile.TrueColor)
	light := logPaletteFor("0;15", colorprofile.TrueColor)
	assert.Equal(t, "\x1b[38;2;197;181;244m", dark.date)
	assert.Equal(t, "\x1b[38;2;98;217;244m", dark.clock)
	assert.Equal(t, "\x1b[38;2;89;66;140m", light.date)
	assert.Equal(t, "\x1b[38;2;0;97;116m", light.clock)
	assert.True(t, strings.HasPrefix(logPaletteFor("15;0", colorprofile.ANSI256).clock, "\x1b[38;5;"))
	assert.Equal(t, ansiLogPalette, logPaletteFor("", colorprofile.TrueColor))
}

// VALIDATES: A mid-tone background can fall back to the terminal foreground.
func TestLogPaletteRechecksContrastAfterReduction(t *testing.T) {
	palette := logPaletteFor("15;244", colorprofile.ANSI256)
	assert.Empty(t, palette.key)
	assert.Equal(t, "\x1b[1m", palette.warning)
}

// VALIDATES: Plugin stderr relays recognize styled severity with a plain key.
func TestParseColoredLogRecordWithPlainKey(t *testing.T) {
	palette := logPaletteFor("15;244", colorprofile.ANSI256)
	line := palette.colorizeLine(`level=WARN msg="peer changed"`, slog.LevelWarn)
	assert.True(t, strings.HasPrefix(line, "level="))
	level, message, _ := ParseLogLine(line)
	assert.Equal(t, slog.LevelWarn, level)
	assert.Equal(t, "peer changed", message)
}

// TestColorizeLineQuotedValueWithEquals verifies quoted values containing = are handled.
//
// VALIDATES: Equals signs inside quoted values don't break key= parsing.
// PREVENTS: Quoted error messages being split at internal = signs.
func TestColorizeLineQuotedValueWithEquals(t *testing.T) {
	line := `time=2025-01-18T12:00:00Z level=ERROR msg="failed" error="key=value inside"` + "\n"
	result := ansiLogPalette.colorizeLine(line, slog.LevelError)

	assert.Contains(t, result, `"key=value inside"`)
	assert.Contains(t, result, ansiLogPalette.key+"error="+ansiReset)
}

// TestColorizeLineEmptyLine verifies empty input returns empty output.
//
// VALIDATES: Empty string is returned unchanged.
// PREVENTS: Panic on empty input.
func TestColorizeLineEmptyLine(t *testing.T) {
	assert.Equal(t, "", ansiLogPalette.colorizeLine("", slog.LevelInfo))
}

// TestColorizeLinePreservesNewline verifies trailing newline is preserved.
//
// VALIDATES: Output retains trailing newline from input.
// PREVENTS: Log lines missing their terminator.
func TestColorizeLinePreservesNewline(t *testing.T) {
	line := "time=2025-01-18T12:00:00Z level=INFO msg=test\n"
	result := ansiLogPalette.colorizeLine(line, slog.LevelInfo)
	assert.True(t, strings.HasSuffix(result, "\n"))
}

// TestColorHandlerOutput verifies the full handler produces colored output.
//
// VALIDATES: colorHandler wraps TextHandler output with ANSI codes.
// PREVENTS: Handler silently dropping colors or content.
func TestColorHandlerOutput(t *testing.T) {
	var buf bytes.Buffer
	handler := newColorHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler).With("subsystem", "test")

	logger.Info("hello", "key", "value")

	output := buf.String()
	assert.Contains(t, output, ansiLogPalette.clock)
	assert.Contains(t, output, ansiReset)
	assert.Contains(t, output, ansiLogPalette.identity+"test"+ansiReset)
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")
}

// VALIDATES: A color override applies to console handlers created before it.
func TestConsoleHandlerUsesCurrentColorOverride(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	require.NoError(t, os.Unsetenv("NO_COLOR"))
	t.Setenv("TERM", "xterm")
	t.Setenv("COLORFGBG", "")
	t.Setenv("ze.log.color", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	var buf bytes.Buffer
	handler := newConsoleHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(handler)

	require.NoError(t, env.Set("ze.log.color", "true"))
	logger.Warn("forced color")
	assert.Contains(t, buf.String(), ansiLogPalette.warning+"WARN"+ansiReset)

	buf.Reset()
	require.NoError(t, env.Set("ze.log.color", "false"))
	logger.Warn("plain output")
	assert.NotContains(t, buf.String(), "\x1b")
	assert.Contains(t, buf.String(), `level=WARN msg="plain output"`)
}

// TestColorHandlerWithAttrs verifies WithAttrs carries pre-defined attrs through.
//
// VALIDATES: Attributes added via WithAttrs appear in colored output.
// PREVENTS: Pre-defined attributes being lost after colorization.
func TestColorHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := newColorHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler2 := handler.WithAttrs([]slog.Attr{slog.String("subsystem", "bgp")})
	logger := slog.New(handler2)

	logger.Info("test")

	output := buf.String()
	assert.Contains(t, output, "subsystem")
	assert.Contains(t, output, "bgp")
}

// TestColorHandlerEnabled verifies level filtering works through colorHandler.
//
// VALIDATES: Enabled() delegates to inner TextHandler correctly.
// PREVENTS: Color handler accepting all levels regardless of config.
func TestColorHandlerEnabled(t *testing.T) {
	handler := newColorHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})

	assert.True(t, handler.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))
	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, handler.Enabled(context.Background(), slog.LevelDebug))
}

// TestUseColorNonFile verifies non-file writers never get colors.
//
// VALIDATES: bytes.Buffer (not *os.File) returns false.
// PREVENTS: Colors in test output or pipe destinations.
func TestUseColorNonFile(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, UseColor(&buf))
}

// TestUseColorNoColorEnv verifies NO_COLOR env var disables colors.
//
// VALIDATES: NO_COLOR convention (no-color.org) is respected.
// PREVENTS: Colors appearing when user explicitly disabled them.
func TestUseColorNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assert.False(t, UseColor(os.Stderr))
}

// TestLevelColor verifies color mapping for each severity level.
//
// VALIDATES: Warning and error use dedicated accents while routine levels are plain.
// PREVENTS: Wrong color for a severity level.
func TestLevelColor(t *testing.T) {
	assert.Empty(t, ansiLogPalette.valueStyle(slog.LevelKey, "DEBUG", slog.LevelDebug))
	assert.Empty(t, ansiLogPalette.valueStyle(slog.LevelKey, "INFO", slog.LevelInfo))
	assert.Equal(t, ansiLogPalette.warning, ansiLogPalette.valueStyle(slog.LevelKey, "WARN", slog.LevelWarn))
	assert.Equal(t, ansiLogPalette.failure, ansiLogPalette.valueStyle(slog.LevelKey, "ERROR", slog.LevelError))
}

// TestColorHandlerWithGroup verifies WithGroup prefixes keys correctly.
//
// VALIDATES: Group-prefixed keys are colorized.
// PREVENTS: WithGroup breaking the color handler chain.
func TestColorHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := newColorHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	grouped := handler.WithGroup("grp")
	logger := slog.New(grouped)

	logger.Info("test", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "grp.key")
	assert.Contains(t, output, "value")
}

// TestUseColorTermDumb verifies TERM=dumb disables colors even when ze.log.color=true.
//
// VALIDATES: AC-2: TERM=dumb disables color output, overriding ze.log.color.
// PREVENTS: ANSI codes sent to terminals that cannot display them.
func TestUseColorTermDumb(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("ze.log.color", "true")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	var buf bytes.Buffer
	assert.False(t, UseColor(&buf))
}

// TestUseColorZeLogColorFalse verifies ze.log.color=false disables colors.
//
// VALIDATES: AC-3: ze.log.color=false forces color off regardless of TTY.
// PREVENTS: Color output when explicitly disabled via ze env var.
func TestUseColorZeLogColorFalse(t *testing.T) {
	t.Setenv("ze.log.color", "false")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	assert.False(t, UseColor(os.Stderr))
}

// TestUseColorZeLogColorTrue verifies ze.log.color=true enables colors on non-TTY.
//
// VALIDATES: AC-4: ze.log.color=true forces color on regardless of TTY.
// PREVENTS: Color being suppressed when user explicitly requested it.
func TestUseColorZeLogColorTrue(t *testing.T) {
	oldNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	oldTerm, hadTerm := os.LookupEnv("TERM")
	require.NoError(t, os.Unsetenv("NO_COLOR"))
	require.NoError(t, os.Unsetenv("TERM"))
	t.Cleanup(func() {
		if hadNoColor {
			require.NoError(t, os.Setenv("NO_COLOR", oldNoColor))
		} else {
			require.NoError(t, os.Unsetenv("NO_COLOR"))
		}
		if hadTerm {
			require.NoError(t, os.Setenv("TERM", oldTerm))
		} else {
			require.NoError(t, os.Unsetenv("TERM"))
		}
		env.ResetCache()
	})
	t.Setenv("ze.log.color", "true")
	env.ResetCache()
	var buf bytes.Buffer
	assert.True(t, UseColor(&buf))
}

// TestUseColorNoColorEnvBeatsZeLogColor verifies NO_COLOR wins over ze.log.color=true.
//
// VALIDATES: AC-7: NO_COLOR (system convention) takes precedence over ze.log.color.
// PREVENTS: Ze env var overriding the system-standard NO_COLOR convention.
func TestUseColorNoColorEnvBeatsZeLogColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("ze.log.color", "true")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	var buf bytes.Buffer
	assert.False(t, UseColor(&buf))
}

// TestUseColorTermDumbAlone verifies TERM=dumb disables colors without ze.log.color set.
//
// VALIDATES: AC-2 in isolation: TERM=dumb alone is sufficient to disable color.
// PREVENTS: TERM=dumb check being accidentally removed without test failure.
func TestUseColorTermDumbAlone(t *testing.T) {
	t.Setenv("TERM", "dumb")
	assert.False(t, UseColor(os.Stderr))
}

// TestUseColorZeLogColorEmpty verifies empty ze.log.color falls through to TTY detection.
//
// VALIDATES: The v != "" guard in UseColor skips to TTY detection for empty values.
// PREVENTS: Removing the empty-string guard silently breaking TTY fallback.
func TestUseColorZeLogColorEmpty(t *testing.T) {
	t.Setenv("ze.log.color", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	var buf bytes.Buffer
	assert.False(t, UseColor(&buf)) // non-TTY buffer, falls through to TTY check
}

// TestUseColorZeLogColorInvalid verifies unrecognized ze.log.color values disable color.
//
// VALIDATES: Non-boolean values like "maybe" are treated as color-off by env.IsEnabled.
// PREVENTS: Unrecognized values silently enabling color output.
func TestUseColorZeLogColorInvalid(t *testing.T) {
	t.Setenv("ze.log.color", "maybe")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	var buf bytes.Buffer
	assert.False(t, UseColor(&buf))
}
