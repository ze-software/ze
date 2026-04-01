package slogutil

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestColorizeLineLevelColors verifies each level gets the correct color.
//
// VALIDATES: Level values are wrapped in severity-specific ANSI colors.
// PREVENTS: All levels looking the same in terminal output.
func TestColorizeLineLevelColors(t *testing.T) {
	tests := []struct {
		level     slog.Level
		levelStr  string
		wantColor string
	}{
		{slog.LevelDebug, "DEBUG", ansiCyan},
		{slog.LevelInfo, "INFO", ansiGreen},
		{slog.LevelWarn, "WARN", ansiYellow},
		{slog.LevelError, "ERROR", ansiBoldRed},
	}
	for _, tt := range tests {
		t.Run(tt.levelStr, func(t *testing.T) {
			line := "time=2025-01-18T12:00:00Z level=" + tt.levelStr + " msg=test\n"
			result := colorizeLine(line, tt.level)
			assert.Contains(t, result, tt.wantColor+tt.levelStr+ansiReset)
		})
	}
}

// TestColorizeLineDimsKeys verifies all key= prefixes are dimmed.
//
// VALIDATES: Key prefixes are wrapped in dim ANSI codes.
// PREVENTS: Keys and values having identical styling.
func TestColorizeLineDimsKeys(t *testing.T) {
	line := "time=2025-01-18T12:00:00Z level=INFO msg=hello subsystem=bgp peer=192.168.1.1\n"
	result := colorizeLine(line, slog.LevelInfo)

	assert.Contains(t, result, ansiDim+"time="+ansiReset)
	assert.Contains(t, result, ansiDim+"msg="+ansiReset)
	assert.Contains(t, result, ansiDim+"subsystem="+ansiReset)
	assert.Contains(t, result, ansiDim+"peer="+ansiReset)
}

// TestColorizeLinePreservesValues verifies attribute values are not altered.
//
// VALIDATES: Only keys and level get ANSI treatment, values are preserved.
// PREVENTS: Values being garbled or wrapped in unexpected codes.
func TestColorizeLinePreservesValues(t *testing.T) {
	line := `time=2025-01-18T12:00:00Z level=INFO msg="hello world" subsystem=bgp` + "\n"
	result := colorizeLine(line, slog.LevelInfo)

	assert.Contains(t, result, "2025-01-18T12:00:00Z")
	assert.Contains(t, result, `"hello world"`)
	assert.Contains(t, result, ansiDim+"subsystem="+ansiReset+"bgp")
}

// TestColorizeLineQuotedValueWithEquals verifies quoted values containing = are handled.
//
// VALIDATES: Equals signs inside quoted values don't break key= parsing.
// PREVENTS: Quoted error messages being split at internal = signs.
func TestColorizeLineQuotedValueWithEquals(t *testing.T) {
	line := `time=2025-01-18T12:00:00Z level=ERROR msg="failed" error="key=value inside"` + "\n"
	result := colorizeLine(line, slog.LevelError)

	assert.Contains(t, result, `"key=value inside"`)
	assert.Contains(t, result, ansiDim+"error="+ansiReset)
}

// TestColorizeLineEmptyLine verifies empty input returns empty output.
//
// VALIDATES: Empty string is returned unchanged.
// PREVENTS: Panic on empty input.
func TestColorizeLineEmptyLine(t *testing.T) {
	assert.Equal(t, "", colorizeLine("", slog.LevelInfo))
}

// TestColorizeLinePreservesNewline verifies trailing newline is preserved.
//
// VALIDATES: Output retains trailing newline from input.
// PREVENTS: Log lines missing their terminator.
func TestColorizeLinePreservesNewline(t *testing.T) {
	line := "time=2025-01-18T12:00:00Z level=INFO msg=test\n"
	result := colorizeLine(line, slog.LevelInfo)
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
	assert.Contains(t, output, ansiDim)
	assert.Contains(t, output, ansiReset)
	assert.Contains(t, output, ansiGreen)
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")
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
// VALIDATES: Each level maps to a distinct ANSI color.
// PREVENTS: Wrong color for a severity level.
func TestLevelColor(t *testing.T) {
	assert.Equal(t, ansiCyan, levelColor(slog.LevelDebug))
	assert.Equal(t, ansiGreen, levelColor(slog.LevelInfo))
	assert.Equal(t, ansiYellow, levelColor(slog.LevelWarn))
	assert.Equal(t, ansiBoldRed, levelColor(slog.LevelError))
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
	t.Setenv("ze.log.color", "true")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
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
