// Package slogutil provides per-subsystem logging configuration for Ze BGP.
//
// Environment variables use hierarchical ze.log.<path> convention:
//   - ze.log=<level> - base level for all subsystems
//   - ze.log.bgp=<level> - level for all bgp.* subsystems
//   - ze.log.bgp.fsm=<level> - level for specific subsystem
//   - ze.log.backend=<backend> - log output (stderr/stdout/syslog)
//   - ze.log.destination=<addr> - syslog address (when backend=syslog)
//   - ze.log.relay=<level> - plugin stderr relay level
//
// Priority (highest to lowest):
//  1. CLI flag --log-level (plugin processes only)
//  2. Most specific env var (dot): ze.log.bgp.fsm
//  3. Most specific env var (underscore): ze_log_bgp_fsm
//  4. Parent env var (dot): ze.log.bgp
//  5. Parent env var (underscore): ze_log_bgp
//  6. ... up to ze.log / ze_log
//  7. Default: WARN (shows warnings and errors)
//
// To silence all logging: ze.log=disabled
//
// Shell-compatible: ze_log_bgp_fsm also works (dot replaced with underscore).
package slogutil

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// levelDisabled is the log level string that disables logging.
const levelDisabled = "disabled"

// getLogEnv returns the log level for a subsystem using hierarchical lookup.
// Walks from most specific to least specific: ze.log.bgp.fsm → ze.log.bgp → ze.log
// At each level, dot notation takes precedence over underscore.
func getLogEnv(subsystem string) string {
	// Build path segments: ["bgp", "fsm"] for "bgp.fsm"
	parts := strings.Split(subsystem, ".")

	// Try from most specific to least specific
	for i := len(parts); i >= 0; i-- {
		var key string
		if i == 0 {
			key = "ze.log"
		} else {
			key = "ze.log." + strings.Join(parts[:i], ".")
		}

		// Dot notation first (higher priority)
		if v := os.Getenv(key); v != "" {
			return v
		}

		// Underscore notation (shell-compatible)
		underKey := strings.ReplaceAll(key, ".", "_")
		if v := os.Getenv(underKey); v != "" {
			return v
		}
	}

	return ""
}

// getSpecialEnv returns a special (non-hierarchical) env var value.
// Used for backend, destination, relay which don't use hierarchical lookup.
func getSpecialEnv(key string) string {
	dotKey := "ze.log." + key
	if v := os.Getenv(dotKey); v != "" {
		return v
	}
	underKey := strings.ReplaceAll(dotKey, ".", "_")
	return os.Getenv(underKey)
}

// Logger returns a logger for an engine subsystem.
// Each subsystem gets its own logger instance to allow independent enable/disable.
// Uses hierarchical env var lookup: ze.log.<subsystem> → ze.log.<parent> → ze.log
// Default: WARN level (shows warnings and errors). Use ze.log=disabled to silence.
func Logger(subsystem string) *slog.Logger {
	v := getLogEnv(subsystem)
	if v == "" {
		// Default to WARN level - show warnings and errors
		handler := createHandler(slog.LevelWarn)
		return slog.New(handler).With("subsystem", subsystem)
	}
	lvl, enabled := parseLevel(v)
	if !enabled {
		return slog.New(discardHandler{})
	}
	handler := createHandler(lvl)
	return slog.New(handler).With("subsystem", subsystem)
}

// PluginLogger returns a logger for plugin processes.
// Priority: CLI flag → env var hierarchy → WARN (default).
// If cliLevel is levelDisabled or empty, falls back to env var lookup.
// Plugins ALWAYS write to stderr (stdout = protocol messages).
func PluginLogger(subsystem, cliLevel string) *slog.Logger {
	// CLI flag takes precedence if it's a valid, enabled level
	if cliLevel != "" && cliLevel != levelDisabled {
		lvl, enabled := parseLevel(cliLevel)
		if enabled {
			handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
			return slog.New(handler).With("subsystem", subsystem)
		}
	}

	// Fall back to hierarchical env var lookup
	v := getLogEnv(subsystem)
	if v == "" {
		// Default to WARN level - show warnings and errors
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})
		return slog.New(handler).With("subsystem", subsystem)
	}
	lvl, enabled := parseLevel(v)
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

// RelayLevel returns the plugin stderr relay level and whether it's enabled.
// Reads ze.log.relay for level. Default: WARN (shows warnings and errors).
// Use ze.log.relay=disabled to silence plugin stderr.
func RelayLevel() (slog.Level, bool) {
	v := getSpecialEnv("relay")
	if v == "" {
		// Default to WARN level - show warnings and errors from plugins
		return slog.LevelWarn, true
	}
	return parseLevel(v)
}

// createHandler creates a slog.Handler based on ze.log.backend setting.
func createHandler(level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	backend := getSpecialEnv("backend")
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
	case levelDisabled:
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

// LazyLogger returns a logger that defers creation until first use.
// This allows package-level loggers to pick up config file settings
// that are applied after init() but before first log call.
//
// Usage:
//
//	var configLogger = slogutil.LazyLogger("config")
//
//	func foo() {
//	    configLogger().Debug("message")  // Note the () to get the logger
//	}
func LazyLogger(subsystem string) func() *slog.Logger {
	var logger *slog.Logger
	var once sync.Once
	return func() *slog.Logger {
		once.Do(func() {
			logger = Logger(subsystem)
		})
		return logger
	}
}

// ApplyLogConfig applies log configuration from config file to environment variables.
// This allows log levels to be set via config file instead of only OS env vars.
//
// Config keys map to env vars:
//   - "level" → ze.log (base level for all subsystems)
//   - "bgp.routes" → ze.log.bgp.routes (specific subsystem)
//   - "backend" → ze.log.backend (output destination)
//   - "destination" → ze.log.destination (syslog address)
//   - "relay" → ze.log.relay (plugin stderr relay level)
//
// Priority: OS env var > config file > default (WARN).
// If an OS env var is already set, it is NOT overwritten.
// Invalid log levels generate a warning to stderr.
//
// Call this early in main() before loggers are created.
func ApplyLogConfig(configValues map[string]map[string]string) {
	applyLogConfigTo(configValues, os.Stderr)
}

// applyLogConfigTo applies log configuration, writing warnings to the given writer.
// Used by ApplyLogConfig (with os.Stderr) and tests (with a buffer).
func applyLogConfigTo(configValues map[string]map[string]string, warnWriter io.Writer) {
	if configValues == nil {
		return
	}

	logConfig, ok := configValues["log"]
	if !ok || len(logConfig) == 0 {
		return
	}

	for key, value := range logConfig {
		var envKey string
		isLevel := false

		switch key {
		case "level":
			// Base level for all subsystems
			envKey = "ze.log"
			isLevel = true
		case "backend":
			// Validate backend value
			switch strings.ToLower(value) {
			case "stderr", "stdout", "syslog":
				// valid
			default:
				_, _ = fmt.Fprintf(warnWriter, "warning: invalid log backend %q (must be stderr/stdout/syslog)\n", value)
				continue
			}
			envKey = "ze.log." + key
		case "destination":
			// No validation for destination (address string)
			envKey = "ze.log." + key
		case "relay":
			envKey = "ze.log." + key
			isLevel = true
		default:
			// Subsystem path like "bgp.routes", "config", etc.
			envKey = "ze.log." + key
			isLevel = true
		}

		// Validate log level values
		if isLevel {
			_, valid := parseLevel(value)
			if !valid && strings.ToLower(value) != levelDisabled {
				_, _ = fmt.Fprintf(warnWriter, "warning: invalid log level %q for %s (must be disabled/debug/info/warn/err)\n", value, key)
				continue
			}
		}

		// Priority: OS env var > config file
		// Only set if not already set by OS
		if os.Getenv(envKey) == "" {
			_ = os.Setenv(envKey, value)
		}
	}
}

// discardHandler discards all log records.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }
