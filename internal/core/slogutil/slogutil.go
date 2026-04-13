// Design: docs/architecture/config/environment.md — structured logging utilities
// Detail: color.go — ANSI color formatting for terminal output
//
// Package slogutil provides per-subsystem logging configuration for Ze BGP.
//
// Environment variables use hierarchical ze.log.<path> convention:
//   - ze.log=<level> - base level for all subsystems
//   - ze.log.bgp=<level> - level for all bgp.* subsystems
//   - ze.log.bgp.fsm=<level> - level for specific subsystem
//   - ze.log.backend=<backend> - log output (stderr/stdout/syslog/kmsg)
//   - ze.log.destination=<addr> - syslog address (when backend=syslog)
//   - ze.log.relay=<level> - plugin stderr relay level
//   - ze.log.color=<bool> - force color on/off (overrides TTY detection)
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
	"slices"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Env var registrations for logging.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.log", Type: "string", Default: "warn", Description: "Base log level for all subsystems"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.log.<subsystem>", Type: "string", Description: "Log level for specific subsystem (e.g. ze.log.bgp.fsm)", Private: true})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.log.backend", Type: "string", Default: "stderr", Description: "Log output: stderr, stdout, or syslog (requires ze.log.destination)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.log.destination", Type: "string", Description: "Syslog address when backend=syslog (e.g. localhost:514, /dev/log)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.log.relay", Type: "string", Description: "Plugin stderr relay level (disabled/debug/info/warn/err)"})
)

// SubsystemInfo describes a log subsystem for ze env listing.
type SubsystemInfo struct {
	Name        string
	Description string
}

// subsystemNames tracks all registered subsystem names for ze env listing.
var subsystemNames sync.Map

// subsystemDescriptions maps subsystem names to human-readable descriptions.
//
//nolint:gochecknoglobals // Central registry, intentionally global.
var subsystemDescriptions = map[string]string{
	// bgp.* -- BGP protocol subsystem.
	"bgp.config":           "Configuration parsing and loading",
	"bgp.filter":           "Route filtering (AS loop, originator-ID)",
	"bgp.filter.community": "Community-based route filtering plugin",
	"bgp.gr":               "Graceful restart marker handling",
	"bgp.reactor":          "Reactor event loop and peer management",
	"bgp.reactor.cache":    "UPDATE cache gap-based eviction",
	"bgp.reactor.forward":  "Per-peer forward worker pool",
	"bgp.reactor.peer":     "Peer lifecycle and FSM transitions",
	"bgp.reactor.session":  "Session wire protocol handling",
	"bgp.routes":           "Route processing and announcements",
	"bgp.server":           "TCP server and connection handling",
	"bgp.watchdog":         "Watchdog timer plugin",
	// chaos.* -- Chaos fault injection.
	"chaos":      "Chaos fault injection orchestration",
	"chaos.peer": "Chaos testing simulated peers",
	// cli.* -- CLI and editor.
	"cli.editor.draft": "Config editor draft persistence",
	// hub.* -- Hub process management.
	"hub.managed": "Managed mode client connection",
	"hub.reload":  "Configuration reload handling",
	// plugin.* -- Plugin infrastructure.
	"plugin":             "Plugin process lifecycle and event delivery",
	"plugin.coordinator": "Plugin startup stage coordination",
	"plugin.manager":     "Multi-plugin coordination and respawn",
	"plugin.relay":       "Plugin stderr relay to engine log",
	"plugin.server":      "Plugin RPC server and stage protocol",
	// test.* -- Test infrastructure.
	"test.record": "Test recording infrastructure",
	"test.runner": "Test runner infrastructure",
	// web.* -- Web UI.
	"web.auth":   "Web UI authentication",
	"web.server": "Web UI HTTP server",
}

// registerSubsystem records a subsystem name for discovery by ze env.
func registerSubsystem(name string) {
	subsystemNames.Store(name, true)
}

// Subsystems returns all known subsystem info, sorted alphabetically.
// These are subsystems that have created a logger via Logger() or LazyLogger().
func Subsystems() []SubsystemInfo {
	var infos []SubsystemInfo
	subsystemNames.Range(func(key, _ any) bool {
		if name, ok := key.(string); ok {
			infos = append(infos, SubsystemInfo{
				Name:        name,
				Description: subsystemDescriptions[name],
			})
		}
		return true
	})
	slices.SortFunc(infos, func(a, b SubsystemInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	return infos
}

// Log level and backend string constants.
const (
	levelDisabled = "disabled"
	backendStdout = "stdout"
	backendSyslog = "syslog"
	backendStderr = "stderr"
	backendKmsg   = "kmsg"
)

// levelRegistry tracks subsystem names to their *slog.LevelVar for runtime level changes.
// Only loggers created via Logger() or LazyLogger() are registered (not disabled ones).
var levelRegistry sync.Map // map[string]*slog.LevelVar

// getLogEnv returns the log level for a subsystem using hierarchical lookup.
// Walks from most specific to least specific: ze.log.bgp.fsm → ze.log.bgp → ze.log
// At each level, checks dot, lowercase underscore, and uppercase underscore notation.
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

		if v := env.Get(key); v != "" {
			return v
		}
	}

	return ""
}

// getSpecialEnv returns a special (non-hierarchical) env var value.
// Used for backend, destination, relay which don't use hierarchical lookup.
func getSpecialEnv(key string) string {
	return env.Get("ze.log." + key)
}

// Logger returns a logger for an engine subsystem.
// Each subsystem gets its own logger instance to allow independent enable/disable.
// Uses hierarchical env var lookup: ze.log.<subsystem> → ze.log.<parent> → ze.log
// Default: WARN level (shows warnings and errors). Use ze.log=disabled to silence.
// Enabled loggers are registered in the level registry for runtime level changes.
func Logger(subsystem string) *slog.Logger {
	registerSubsystem(subsystem)
	v := getLogEnv(subsystem)
	if v == "" {
		// Default to WARN level - show warnings and errors
		lv := &slog.LevelVar{}
		lv.Set(slog.LevelWarn)
		levelRegistry.Store(subsystem, lv)
		handler := createHandler(lv)
		return slog.New(handler).With("subsystem", subsystem)
	}
	lvl, enabled := parseLevel(v)
	if !enabled {
		return slog.New(discardHandler{})
	}
	lv := &slog.LevelVar{}
	lv.Set(lvl)
	levelRegistry.Store(subsystem, lv)
	handler := createHandler(lv)
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
// Accepts slog.Leveler so both slog.Level (fixed) and *slog.LevelVar (mutable) work.
// Uses colorHandler for terminal output (auto-detected), plain TextHandler otherwise.
func createHandler(level slog.Leveler) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	backend := getSpecialEnv("backend")

	// Split on comma: "kmsg,stderr" -> ["kmsg", "stderr"]
	parts := strings.Split(strings.ToLower(backend), ",")

	// Single syslog backend (not composable with io.Writer).
	if len(parts) == 1 && strings.TrimSpace(parts[0]) == backendSyslog {
		return newSyslogHandler(opts)
	}

	writers := make([]io.Writer, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if w := writerForBackend(name); w != nil {
			writers = append(writers, w)
		}
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = io.MultiWriter(writers...)
	}

	if UseColor(w) {
		return newColorHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// writerForBackend returns an io.Writer for a backend name, or nil if unknown/empty.
func writerForBackend(name string) io.Writer {
	switch name {
	case "":
		return nil
	case backendStdout:
		return os.Stdout
	case backendStderr:
		return os.Stderr
	case backendKmsg:
		return openKmsg()
	}
	writeWarn(os.Stderr, "warning: unknown log backend %q, ignoring\n", name)
	return nil
}

// kmsgFile holds the singleton /dev/kmsg file descriptor so repeated
// calls to openKmsg (e.g., on config reload) reuse the same fd.
var kmsgFile *os.File

// openKmsg opens /dev/kmsg for writing (once). Falls back to stderr.
func openKmsg() io.Writer {
	if kmsgFile != nil {
		return kmsgFile
	}
	f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0)
	if err != nil {
		return os.Stderr
	}
	kmsgFile = f
	return f
}

var validBackends = map[string]bool{
	backendStderr: true,
	backendStdout: true,
	backendSyslog: true,
	backendKmsg:   true,
}

// validateBackends checks that every comma-separated backend name is known.
func validateBackends(value string) error {
	for part := range strings.SplitSeq(strings.ToLower(value), ",") {
		name := strings.TrimSpace(part)
		if name != "" && !validBackends[name] {
			return fmt.Errorf("invalid log backend %q (must be stderr/stdout/syslog/kmsg)", name)
		}
	}
	return nil
}

// writeWarn writes a warning to w, discarding write errors (logger not yet initialized).
func writeWarn(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...) //nolint:errcheck // pre-logger warning output
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
// The logger is registered in the level registry on first call (inside the Once).
//
// Usage:
//
//	var configLogger = slogutil.LazyLogger("bgp.config")
//
//	func foo() {
//	    configLogger().Debug("message")  // Note the () to get the logger
//	}
func LazyLogger(subsystem string) func() *slog.Logger {
	registerSubsystem(subsystem)
	var logger *slog.Logger
	var once sync.Once
	return func() *slog.Logger {
		once.Do(func() {
			logger = Logger(subsystem) // Logger() handles registration
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
			if err := validateBackends(value); err != nil {
				writeWarn(warnWriter, "warning: %v\n", err)
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
			if !valid && !strings.EqualFold(value, levelDisabled) {
				_, _ = fmt.Fprintf(warnWriter, "warning: invalid log level %q for %s (must be disabled/debug/info/warn/err)\n", value, key)
				continue
			}
		}

		// Priority: OS env var > config file
		// Only set if not already set by OS (any notation)
		if env.Get(envKey) == "" {
			_ = env.Set(envKey, value)
		}
	}
}

// ListLevels returns a map of subsystem names to their current level strings.
// Only includes loggers registered via Logger() or LazyLogger() (not disabled ones).
func ListLevels() map[string]string {
	result := make(map[string]string)
	levelRegistry.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		lv, ok := value.(*slog.LevelVar)
		if !ok {
			return true
		}
		result[name] = LevelString(lv.Level())
		return true
	})
	return result
}

// SetLevel changes the log level for a subsystem at runtime.
// Returns an error if the subsystem is unknown or the level string is invalid.
func SetLevel(subsystem, levelStr string) error {
	lvl, enabled := parseLevel(levelStr)
	if !enabled {
		return fmt.Errorf("invalid level %q (valid: debug, info, warn, err)", levelStr)
	}

	val, ok := levelRegistry.Load(subsystem)
	if !ok {
		return fmt.Errorf("unknown subsystem: %s", subsystem)
	}

	lv, ok := val.(*slog.LevelVar)
	if !ok {
		return fmt.Errorf("unknown subsystem: %s", subsystem)
	}
	lv.Set(lvl)
	return nil
}

// LevelString converts a slog.Level to a human-readable string.
func LevelString(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	}
	// Non-standard level (e.g. custom numeric) — use stdlib formatting.
	return level.String()
}

// ResetLevelRegistry clears all entries from the level registry. Only for use in tests.
func ResetLevelRegistry() {
	levelRegistry.Range(func(key, _ any) bool {
		levelRegistry.Delete(key)
		return true
	})
}

// discardHandler discards all log records.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }
