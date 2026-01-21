// Package slogutil provides per-subsystem logging configuration for Ze BGP.
//
// Engine subsystems use Logger() which reads from ze.log.bgp.<subsystem> env vars.
// Plugin processes use LoggerWithLevel() which takes level from CLI --log-level flag.
//
// Environment variables:
//   - ze.log.bgp.<subsystem>=<level> - enable subsystem at level (debug/info/warn/err)
//   - ze.log.bgp.backend=<backend> - log output (stderr/stdout/syslog)
//   - ze.log.bgp.destination=<addr> - syslog address (when backend=syslog)
//   - ze.log.bgp.plugin=enabled - relay plugin stderr to engine logs
//
// Shell-compatible: ze_log_bgp_<subsystem> also works (dot replaced with underscore).
package slogutil

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// getLogEnv returns the log environment variable value.
// Checks both dot notation (ze.log.bgp.key) and underscore (ze_log_bgp_key).
// Dot notation takes precedence.
func getLogEnv(key string) string {
	// Dot notation first (higher priority)
	dotKey := "ze.log.bgp." + key
	if v := os.Getenv(dotKey); v != "" {
		return v
	}
	// Underscore notation (shell-compatible)
	underKey := strings.ReplaceAll(dotKey, ".", "_")
	return os.Getenv(underKey)
}

// Logger returns a logger for an engine subsystem.
// Each subsystem gets its own logger instance to allow independent enable/disable.
// Reads ze.log.bgp.<subsystem> for level, ze.log.bgp.backend for output.
// Returns a discard logger if subsystem is not enabled.
func Logger(subsystem string) *slog.Logger {
	v := getLogEnv(subsystem)
	if v == "" {
		return slog.New(discardHandler{})
	}
	lvl, enabled := parseLevel(v)
	if !enabled {
		return slog.New(discardHandler{})
	}
	handler := createHandler(lvl)
	return slog.New(handler).With("subsystem", subsystem)
}

// LoggerWithLevel returns a logger for plugins (level from CLI --log-level flag).
// Plugins ALWAYS write to stderr (stdout = protocol messages).
func LoggerWithLevel(subsystem, level string) *slog.Logger {
	lvl, enabled := parseLevel(level)
	if !enabled {
		return slog.New(discardHandler{})
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler).With("subsystem", subsystem)
}

// LoggerWithOutput returns a logger that writes to a specific output.
// Used for testing and custom output destinations.
func LoggerWithOutput(subsystem, level string, w io.Writer) *slog.Logger {
	lvl, enabled := parseLevel(level)
	if !enabled {
		return slog.New(discardHandler{})
	}
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler).With("subsystem", subsystem)
}

// IsPluginRelayEnabled checks if plugin stderr should be relayed.
// Reads ze.log.bgp.plugin (enabled/disabled).
func IsPluginRelayEnabled() bool {
	v := getLogEnv("plugin")
	return strings.ToLower(v) == "enabled"
}

// createHandler creates a slog.Handler based on ze.log.bgp.backend setting.
func createHandler(level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	backend := getLogEnv("backend")
	switch strings.ToLower(backend) {
	case "stdout":
		return slog.NewTextHandler(os.Stdout, opts)
	case "syslog":
		return newSyslogHandler(opts)
	default: // stderr (default)
		return slog.NewTextHandler(os.Stderr, opts)
	}
}

// parseLevel parses a log level string.
// Returns (level, enabled). enabled=false means logging should be disabled.
// Level strings are case-insensitive: disabled, debug, info, warn/warning, err/error.
func parseLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(s) {
	case "disabled":
		return slog.LevelInfo, false
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "err", "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false // unknown = disabled
	}
}

// DiscardLogger returns a logger that discards all output.
// Use this as a default logger for plugins before SetLogger() is called.
func DiscardLogger() *slog.Logger {
	return slog.New(discardHandler{})
}

// discardHandler discards all log records.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }
