// Design: docs/architecture/config/environment.md — structured logging utilities
// Overview: slogutil.go — core logging infrastructure

package slogutil

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Env var registration for color control.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.log.color", Type: "bool", Description: "Force color output on (true) or off (false)"})

// ANSI escape codes for terminal output.
const (
	ansiReset   = "\033[0m"
	ansiDim     = "\033[2m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiBoldRed = "\033[1;31m"
)

// colorHandler wraps slog.TextHandler to add ANSI colors to terminal output.
// Level values get severity-based colors, key= prefixes are dimmed.
type colorHandler struct {
	inner  slog.Handler
	writer io.Writer
	mu     *sync.Mutex
	buf    *bytes.Buffer
}

func newColorHandler(w io.Writer, opts *slog.HandlerOptions) *colorHandler {
	buf := &bytes.Buffer{}
	return &colorHandler{
		inner:  slog.NewTextHandler(buf, opts),
		writer: w,
		mu:     &sync.Mutex{},
		buf:    buf,
	}
}

func (h *colorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.buf.Reset()
	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}

	colored := colorizeLine(h.buf.String(), r.Level)
	_, err := io.WriteString(h.writer, colored)
	return err
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &colorHandler{
		inner:  h.inner.WithAttrs(attrs),
		writer: h.writer,
		mu:     h.mu,
		buf:    h.buf,
	}
}

func (h *colorHandler) WithGroup(name string) slog.Handler {
	return &colorHandler{
		inner:  h.inner.WithGroup(name),
		writer: h.writer,
		mu:     h.mu,
		buf:    h.buf,
	}
}

// colorizeLine adds ANSI color codes to a slog text format line.
// Level values get severity-specific colors, all key= prefixes are dimmed.
func colorizeLine(line string, level slog.Level) string {
	if line == "" {
		return line
	}

	hasNewline := line[len(line)-1] == '\n'
	if hasNewline {
		line = line[:len(line)-1]
	}

	var b strings.Builder
	b.Grow(len(line) + 80)

	pos := 0
	first := true

	for pos < len(line) {
		if !first {
			b.WriteByte(' ')
		}
		first = false

		rest := line[pos:]
		eqIdx := strings.IndexByte(rest, '=')
		if eqIdx == -1 {
			b.WriteString(rest)
			break
		}

		key := rest[:eqIdx]

		b.WriteString(ansiDim)
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(ansiReset)

		valueStart := eqIdx + 1
		var valueEnd int

		if valueStart < len(rest) && rest[valueStart] == '"' {
			end := findClosingQuote(rest, valueStart+1)
			if end == -1 {
				valueEnd = len(rest)
			} else {
				valueEnd = end + 1
			}
		} else {
			spIdx := strings.IndexByte(rest[valueStart:], ' ')
			if spIdx == -1 {
				valueEnd = len(rest)
			} else {
				valueEnd = valueStart + spIdx
			}
		}

		value := rest[valueStart:valueEnd]

		if key == "level" {
			b.WriteString(levelColor(level))
			b.WriteString(value)
			b.WriteString(ansiReset)
		} else {
			b.WriteString(value)
		}

		pos += valueEnd
		if pos < len(line) && line[pos] == ' ' {
			pos++
		}
	}

	if hasNewline {
		b.WriteByte('\n')
	}

	return b.String()
}

// levelColor returns the ANSI color escape code for a log level.
func levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return ansiBoldRed
	case level >= slog.LevelWarn:
		return ansiYellow
	case level >= slog.LevelInfo:
		return ansiGreen
	}
	// DEBUG and below
	return ansiCyan
}

// UseColor reports whether color output should be used for the given writer.
//
// Precedence (first match wins):
//  1. NO_COLOR env var set -> disabled (no-color.org system standard)
//  2. TERM=dumb -> disabled (system standard)
//  3. ze.log.color set -> use its boolean value (set by --color/--no-color flags or env var)
//  4. term.IsTerminal(fd) -> enabled/disabled based on TTY
func UseColor(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if v := env.Get("ze.log.color"); v != "" {
		return env.IsEnabled("ze.log.color")
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
