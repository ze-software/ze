// Design: docs/architecture/config/syntax.md — config parsing and loading
// Detail: environment_extract.go — extraction of environment values from config tree
//
// Package config provides configuration parsing for ze.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Env var prefix registration for all ze.bgp.* environment variables.
// The pattern ze.bgp.<section>.<option> matches any key starting with "ze.bgp.".
var _ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.<section>.<option>", Type: "string", Description: "BGP environment configuration (section.option)"})

// Environment constants.
const (
	LogLevelInfo = "INFO"
	EncoderText  = "text"
	EncoderJSON  = "json"
)

// Environment holds all environment-based configuration.
// This provides ExaBGP-compatible environment variable support.
//
// Variable format: ze.bgp.<section>.<option> or ze_bgp_<section>_<option>
// Priority: ze.bgp.x.y > ze_bgp_x_y > default.
type Environment struct {
	Daemon  DaemonEnv
	Log     LogEnv
	TCP     TCPEnv
	BGP     BGPEnv
	Cache   CacheEnv
	API     APIEnv
	Reactor ReactorEnv
	Debug   DebugEnv
	Chaos   ChaosEnv
}

// DaemonEnv holds daemon-related settings.
// Privilege dropping uses ze.user/ze.group env vars (see internal/core/privilege/).
type DaemonEnv struct {
	PID       string // PID file location
	User      string // User (legacy config compat; prefer ze.user env var)
	Daemonize bool   // Run in background
	Drop      bool   // Drop privileges (legacy config compat; prefer ze.user env var)
	Umask     int    // Umask for files (octal)
}

// LogEnv holds logging-related settings.
type LogEnv struct {
	Enable        bool   // Enable logging
	Level         string // Syslog level: DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL
	Destination   string // stdout, stderr, syslog, or filename
	All           bool   // Debug everything
	Configuration bool   // Log config parsing
	Reactor       bool   // Log signals, reloads
	Daemon        bool   // Log pid, forking
	Processes     bool   // Log process handling
	Network       bool   // Log TCP/IP, network state
	Statistics    bool   // Log packet statistics
	Packets       bool   // Log BGP packets
	RIB           bool   // Log local route changes
	Message       bool   // Log route announcements
	Timers        bool   // Log keepalive timers
	Routes        bool   // Log received routes
	Parser        bool   // Log message parsing
	Short         bool   // Short log format
}

// TCPEnv holds TCP-related settings.
type TCPEnv struct {
	Attempts int  // Max connection attempts (0 = unlimited)
	Delay    int  // Delay announcements by N minutes
	Port     int  // Port to bind
	ACL      bool // Experimental ACL
	// NOTE: Bind was removed - listeners are now derived from peer LocalAddress.
	// See spec-listener-per-local-address.md for details.
}

// BGPEnv holds BGP-related settings.
type BGPEnv struct {
	Connect  *bool // Initiate outbound connections (nil = not set, use default true)
	Accept   *bool // Accept inbound connections (nil = not set, use default true)
	OpenWait int   // Seconds to wait for OPEN
}

// CacheEnv holds caching-related settings.
type CacheEnv struct {
	Attributes bool // Cache attributes
}

// APIEnv holds API-related settings.
type APIEnv struct {
	ACK       bool   // Acknowledge API commands
	Chunk     int    // Max lines before yield
	Encoder   string // Encoder type: json
	Compact   bool   // Compact JSON for INET
	Respawn   bool   // Respawn dead processes
	Terminate bool   // Terminate if process dies
	CLI       bool   // Create CLI named pipe
}

// ReactorEnv holds reactor-related settings.
type ReactorEnv struct {
	Speed    float64 // Reactor loop time multiplier
	CacheTTL int     // Update cache TTL in seconds (0=immediate, default 60)
	CacheMax int     // Update cache max entries (0=unlimited, default 1000000)
}

// ChaosEnv holds chaos fault injection settings.
// When Seed is non-zero, Ze wraps its Clock, Dialer, and ListenerFactory
// with chaos-injecting wrappers that introduce seed-driven random failures.
type ChaosEnv struct {
	Seed int64   // PRNG seed (0 = disabled)
	Rate float64 // Fault probability per operation (0.0-1.0, default 0.1)
}

// DebugEnv holds debug-related settings.
type DebugEnv struct {
	PDB           bool   // Enable pdb on errors (N/A in Go)
	Memory        bool   // Memory debug
	Configuration bool   // Raise on config errors
	SelfCheck     bool   // Self-check config
	Route         string // Decode route from config
	Defensive     bool   // Generate random faults
	Rotate        bool   // Rotate config on reload
	Timing        bool   // Reactor timing analysis
	Pprof         string // pprof HTTP server address (e.g. ":6060")
}

// LoadEnvironment loads configuration from environment variables.
// Returns error for invalid env var values (BREAKING CHANGE from silent ignore).
// Use LoadEnvironmentWithConfig(nil) for the same behavior with explicit error handling.
func LoadEnvironment() (*Environment, error) {
	return LoadEnvironmentWithConfig(nil)
}

// loadDefaults sets default values from YANG schema (single source of truth).
// YANG sources: ze-hub-conf.yang (daemon, log) and ze-bgp-conf.yang augment (tcp, bgp, cache, api, reactor, chaos).
// Returns error if any required YANG default is missing -- that is a schema bug.
func (e *Environment) loadDefaults() error {
	schema := YANGSchema()
	if schema == nil {
		return fmt.Errorf("YANG schema failed to load")
	}

	var err error

	// Daemon (ze-hub-conf.yang > environment > daemon)
	if e.Daemon.User, err = SchemaDefaultString(schema, "environment.daemon.user"); err != nil {
		return err
	}
	if e.Daemon.Drop, err = SchemaDefaultBool(schema, "environment.daemon.drop"); err != nil {
		return err
	}
	e.Daemon.Umask = 0o137 // octal -- YANG stores "0137" string, parsed separately

	// Log (ze-hub-conf.yang > environment > log)
	if e.Log.Enable, err = SchemaDefaultBool(schema, "environment.log.enable"); err != nil {
		return err
	}
	if e.Log.Level, err = SchemaDefaultString(schema, "environment.log.level"); err != nil {
		return err
	}
	if e.Log.Destination, err = SchemaDefaultString(schema, "environment.log.destination"); err != nil {
		return err
	}
	if e.Log.Configuration, err = SchemaDefaultBool(schema, "environment.log.configuration"); err != nil {
		return err
	}
	if e.Log.Reactor, err = SchemaDefaultBool(schema, "environment.log.reactor"); err != nil {
		return err
	}
	if e.Log.Daemon, err = SchemaDefaultBool(schema, "environment.log.daemon"); err != nil {
		return err
	}
	if e.Log.Processes, err = SchemaDefaultBool(schema, "environment.log.processes"); err != nil {
		return err
	}
	if e.Log.Network, err = SchemaDefaultBool(schema, "environment.log.network"); err != nil {
		return err
	}
	if e.Log.Statistics, err = SchemaDefaultBool(schema, "environment.log.statistics"); err != nil {
		return err
	}
	if e.Log.Short, err = SchemaDefaultBool(schema, "environment.log.short"); err != nil {
		return err
	}

	// TCP (ze-bgp-conf.yang augment > environment > tcp)
	if e.TCP.Port, err = SchemaDefaultInt(schema, "environment.tcp.port"); err != nil {
		return err
	}

	// BGP (ze-bgp-conf.yang augment > environment > bgp)
	if e.BGP.OpenWait, err = SchemaDefaultInt(schema, "environment.bgp.openwait"); err != nil {
		return err
	}

	// Cache (ze-bgp-conf.yang augment > environment > cache)
	if e.Cache.Attributes, err = SchemaDefaultBool(schema, "environment.cache.attributes"); err != nil {
		return err
	}

	// API (ze-bgp-conf.yang augment > environment > api)
	if e.API.ACK, err = SchemaDefaultBool(schema, "environment.api.ack"); err != nil {
		return err
	}
	if e.API.Chunk, err = SchemaDefaultInt(schema, "environment.api.chunk"); err != nil {
		return err
	}
	if e.API.Encoder, err = SchemaDefaultString(schema, "environment.api.encoder"); err != nil {
		return err
	}
	if e.API.Respawn, err = SchemaDefaultBool(schema, "environment.api.respawn"); err != nil {
		return err
	}
	if e.API.CLI, err = SchemaDefaultBool(schema, "environment.api.cli"); err != nil {
		return err
	}

	// Reactor (ze-bgp-conf.yang augment > environment > reactor)
	if e.Reactor.Speed, err = SchemaDefaultFloat64(schema, "environment.reactor.speed"); err != nil {
		return err
	}
	if e.Reactor.CacheTTL, err = SchemaDefaultInt(schema, "environment.reactor.cache-ttl"); err != nil {
		return err
	}
	if e.Reactor.CacheMax, err = SchemaDefaultInt(schema, "environment.reactor.cache-max"); err != nil {
		return err
	}

	// Chaos (ze-bgp-conf.yang augment > environment > chaos)
	if e.Chaos.Rate, err = SchemaDefaultFloat64(schema, "environment.chaos.rate"); err != nil {
		return err
	}

	return nil
}

// OpenWaitDuration returns the OpenWait as a time.Duration.
func (e *Environment) OpenWaitDuration() time.Duration {
	return time.Duration(e.BGP.OpenWait) * time.Second
}

// SocketPath returns the full path to the API socket.
// Can be overridden with ze.bgp.api.socketpath or ze_bgp_api_socketpath env var.
// Otherwise uses DefaultSocketPath() cascade: XDG_RUNTIME_DIR → /var/run → /tmp.
func (e *Environment) SocketPath() string {
	if path := getEnv("api", "socketpath"); path != "" {
		return path
	}
	return DefaultSocketPath()
}

// DefaultSocketPath returns the default socket path using XDG conventions.
// Resolution order:
//  1. $XDG_RUNTIME_DIR/ze.socket (per-user runtime dir)
//  2. /var/run/ze.socket (system runtime dir, when running as root)
//  3. /tmp/ze.socket (fallback, always writable)
func DefaultSocketPath() string {
	const socketName = "ze.socket"

	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/" + socketName
	}
	if os.Getuid() == 0 {
		return "/var/run/" + socketName
	}
	return "/tmp/" + socketName
}

// ResolveConfigPath searches for a config file using XDG conventions.
// Search order:
//  1. Path as given (absolute or relative to cwd)
//  2. $XDG_CONFIG_HOME/ze/<name> (defaults to ~/.config/ze/)
//  3. Each dir in $XDG_CONFIG_DIRS/ze/<name> (defaults to /etc/xdg/ze/)
//
// Returns the original path unchanged if no XDG match is found,
// letting the caller produce the appropriate "file not found" error.
func ResolveConfigPath(path string) string {
	// Absolute paths and stdin are used as-is.
	if path == "-" || strings.HasPrefix(path, "/") {
		return path
	}

	// If it exists relative to cwd, use it.
	if _, err := os.Stat(path); err == nil {
		return path
	}

	name := path // treat as a filename to search for

	// $XDG_CONFIG_HOME/ze/<name> (default: ~/.config/ze/)
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		if home := os.Getenv("HOME"); home != "" {
			configHome = home + "/.config"
		}
	}
	if configHome != "" {
		base := filepath.Join(configHome, "ze")
		candidate := filepath.Join(base, name)
		if strings.HasPrefix(candidate, base+string(filepath.Separator)) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// $XDG_CONFIG_DIRS/ze/<name> (default: /etc/xdg/ze/)
	configDirs := os.Getenv("XDG_CONFIG_DIRS")
	if configDirs == "" {
		configDirs = "/etc/xdg"
	}
	for dir := range strings.SplitSeq(configDirs, ":") {
		if dir == "" {
			continue
		}
		base := filepath.Join(dir, "ze")
		candidate := filepath.Join(base, name)
		if strings.HasPrefix(candidate, base+string(filepath.Separator)) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Nothing found — return original so caller gets a clear error.
	return path
}

// getEnv returns the environment variable value.
// Checks dot, lowercase underscore, and uppercase underscore notation.
func getEnv(section, option string) string {
	return env.Get("ze.bgp." + section + "." + option)
}

// =============================================================================
// Strict Parsing Functions (return errors instead of silent defaults)
// =============================================================================

// ParseBoolStrict parses a boolean value strictly (case-insensitive).
// Accepts: true/false/yes/no/on/off/enable/disable/1/0.
// Returns error for unrecognized values instead of defaulting to false.
func ParseBoolStrict(value string) (bool, error) {
	v := strings.ToLower(value)
	switch v {
	case "1", "true", "yes", "on", "enable":
		return true, nil
	case "0", configFalse, "no", "off", "disable":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q: must be true/false/yes/no/on/off/enable/disable/1/0", value)
	}
}

// parseIntStrict parses an integer strictly.
func parseIntStrict(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("invalid integer: empty string")
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", value, err)
	}
	return n, nil
}

// parseFloatStrict parses a float strictly.
func parseFloatStrict(value string) (float64, error) {
	if value == "" {
		return 0, fmt.Errorf("invalid float: empty string")
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float %q: %w", value, err)
	}
	return f, nil
}

// parseOctalStrict parses an octal integer strictly.
func parseOctalStrict(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("invalid octal: empty string")
	}
	v := strings.TrimPrefix(value, "0")
	n, err := strconv.ParseInt(v, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid octal %q: %w", value, err)
	}
	return int(n), nil
}

// =============================================================================
// Validation Functions
// =============================================================================

// validateLogLevel checks log level is valid.
// Does NOT trim whitespace - strict validation.
func validateLogLevel(value string) error {
	valid := map[string]bool{
		"DEBUG": true, "INFO": true, "NOTICE": true,
		"WARNING": true, "ERR": true, "CRITICAL": true,
	}
	v := strings.ToUpper(value)
	if !valid[v] {
		return fmt.Errorf("invalid log level %q: must be DEBUG, INFO, NOTICE, WARNING, ERR, or CRITICAL", value)
	}
	return nil
}

// validatePort checks port is valid for BGP: 179 (standard) or >1024 (unprivileged).
func validatePort(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", value, err)
	}
	if n == 179 || (n > 1024 && n <= 65535) {
		return nil
	}
	return fmt.Errorf("port %d invalid: must be 179 or 1025-65535", n)
}

// validateEncoder checks encoder is valid.
// Does NOT trim whitespace - strict validation.
func validateEncoder(value string) error {
	valid := map[string]bool{EncoderJSON: true, EncoderText: true}
	v := strings.ToLower(value)
	if !valid[v] {
		return fmt.Errorf("invalid encoder %q: must be %s or %s", value, EncoderJSON, EncoderText)
	}
	return nil
}

// validateAttempts checks attempts is in valid range (0-1000).
func validateAttempts(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid attempts %q: %w", value, err)
	}
	if n < 0 || n > 1000 {
		return fmt.Errorf("attempts %d out of range: must be 0-1000", n)
	}
	return nil
}

// validateOpenWait checks openwait is in valid range (1-3600 seconds).
func validateOpenWait(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid openwait %q: %w", value, err)
	}
	if n < 1 || n > 3600 {
		return fmt.Errorf("openwait %d out of range: must be 1-3600", n)
	}
	return nil
}

// validateChaosRate checks chaos rate is in valid range (0.0-1.0).
func validateChaosRate(value string) error {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid chaos rate %q: %w", value, err)
	}
	if f < 0 || f > 1.0 {
		return fmt.Errorf("chaos rate %.2f out of range: must be 0.0-1.0", f)
	}
	return nil
}

// validateCacheTTL checks cache TTL is in valid range (0-3600 seconds).
func validateCacheTTL(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid cache-ttl %q: %w", value, err)
	}
	if n < 0 || n > 3600 {
		return fmt.Errorf("cache-ttl %d out of range: must be 0-3600", n)
	}
	return nil
}

// validateCacheMax checks cache max is non-negative.
func validateCacheMax(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid cache-max %q: %w", value, err)
	}
	if n < 0 {
		return fmt.Errorf("cache-max %d must be >= 0", n)
	}
	return nil
}

// validateSpeed checks reactor speed is in valid range (0.1-10.0).
func validateSpeed(value string) error {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid speed %q: %w", value, err)
	}
	if f < 0.1 || f > 10.0 {
		return fmt.Errorf("speed %.2f out of range: must be 0.1-10.0", f)
	}
	return nil
}

// =============================================================================
// Table-Driven Configuration Setters
// =============================================================================

// envOption defines how to set an environment option.
type envOption struct {
	setter   func(env *Environment, value string) error
	validate func(value string) error // optional
}

// setBoolField creates a setter function for boolean fields.
func setBoolField(getter func(e *Environment) *bool) func(env *Environment, value string) error {
	return func(env *Environment, value string) error {
		b, err := ParseBoolStrict(value)
		if err != nil {
			return err
		}
		*getter(env) = b
		return nil
	}
}

// setBoolPtrField creates a setter for optional boolean fields (*bool).
func setBoolPtrField(getter func(e *Environment) **bool) func(env *Environment, value string) error {
	return func(env *Environment, value string) error {
		b, err := ParseBoolStrict(value)
		if err != nil {
			return err
		}
		*getter(env) = &b
		return nil
	}
}

// setIntField creates a setter function for integer fields.
func setIntField(getter func(e *Environment) *int) func(env *Environment, value string) error {
	return func(env *Environment, value string) error {
		n, err := parseIntStrict(value)
		if err != nil {
			return err
		}
		*getter(env) = n
		return nil
	}
}

// envOptions maps section.option to its setter and validator.
//
//nolint:gochecknoglobals // Table-driven configuration, intentionally global
var envOptions = map[string]map[string]envOption{
	"daemon": {
		"pid":       {setter: func(e *Environment, v string) error { e.Daemon.PID = v; return nil }},
		"user":      {setter: func(e *Environment, v string) error { e.Daemon.User = v; return nil }},
		"daemonize": {setter: setBoolField(func(e *Environment) *bool { return &e.Daemon.Daemonize })},
		"drop":      {setter: setBoolField(func(e *Environment) *bool { return &e.Daemon.Drop })},
		"umask": {setter: func(e *Environment, v string) error {
			n, err := parseOctalStrict(v)
			if err != nil {
				return err
			}
			e.Daemon.Umask = n
			return nil
		}},
	},
	"log": {
		"level":         {setter: func(e *Environment, v string) error { e.Log.Level = strings.ToUpper(v); return nil }, validate: validateLogLevel},
		"enable":        {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Enable })},
		"destination":   {setter: func(e *Environment, v string) error { e.Log.Destination = v; return nil }},
		"all":           {setter: setBoolField(func(e *Environment) *bool { return &e.Log.All })},
		"configuration": {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Configuration })},
		"reactor":       {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Reactor })},
		"daemon":        {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Daemon })},
		"processes":     {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Processes })},
		"network":       {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Network })},
		"statistics":    {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Statistics })},
		"packets":       {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Packets })},
		"rib":           {setter: setBoolField(func(e *Environment) *bool { return &e.Log.RIB })},
		"message":       {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Message })},
		"timers":        {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Timers })},
		"routes":        {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Routes })},
		"parser":        {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Parser })},
		"short":         {setter: setBoolField(func(e *Environment) *bool { return &e.Log.Short })},
	},
	"tcp": {
		"port":     {setter: setIntField(func(e *Environment) *int { return &e.TCP.Port }), validate: validatePort},
		"attempts": {setter: setIntField(func(e *Environment) *int { return &e.TCP.Attempts }), validate: validateAttempts},
		"delay":    {setter: setIntField(func(e *Environment) *int { return &e.TCP.Delay })},
		"acl":      {setter: setBoolField(func(e *Environment) *bool { return &e.TCP.ACL })},
		// Backward compatibility aliases (ExaBGP legacy)
		"once": {setter: func(e *Environment, v string) error {
			b, err := ParseBoolStrict(v)
			if err != nil {
				return err
			}
			if b && e.TCP.Attempts == 0 {
				e.TCP.Attempts = 1
			}
			return nil
		}},
		"connections": {setter: setIntField(func(e *Environment) *int { return &e.TCP.Attempts }), validate: validateAttempts},
	},
	"bgp": {
		"connect":  {setter: setBoolPtrField(func(e *Environment) **bool { return &e.BGP.Connect })},
		"accept":   {setter: setBoolPtrField(func(e *Environment) **bool { return &e.BGP.Accept })},
		"openwait": {setter: setIntField(func(e *Environment) *int { return &e.BGP.OpenWait }), validate: validateOpenWait},
	},
	"cache": {
		"attributes": {setter: setBoolField(func(e *Environment) *bool { return &e.Cache.Attributes })},
	},
	"api": {
		"ack":       {setter: setBoolField(func(e *Environment) *bool { return &e.API.ACK })},
		"chunk":     {setter: setIntField(func(e *Environment) *int { return &e.API.Chunk })},
		"encoder":   {setter: func(e *Environment, v string) error { e.API.Encoder = strings.ToLower(v); return nil }, validate: validateEncoder},
		"compact":   {setter: setBoolField(func(e *Environment) *bool { return &e.API.Compact })},
		"respawn":   {setter: setBoolField(func(e *Environment) *bool { return &e.API.Respawn })},
		"terminate": {setter: setBoolField(func(e *Environment) *bool { return &e.API.Terminate })},
		"cli":       {setter: setBoolField(func(e *Environment) *bool { return &e.API.CLI })},
	},
	"reactor": {
		"speed": {
			setter: func(e *Environment, v string) error {
				f, err := parseFloatStrict(v)
				if err != nil {
					return err
				}
				e.Reactor.Speed = f
				return nil
			},
			validate: validateSpeed,
		},
		"cache-ttl": {setter: setIntField(func(e *Environment) *int { return &e.Reactor.CacheTTL }), validate: validateCacheTTL},
		"cache-max": {setter: setIntField(func(e *Environment) *int { return &e.Reactor.CacheMax }), validate: validateCacheMax},
	},
	"debug": {
		"pdb":           {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.PDB })},
		"memory":        {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.Memory })},
		"configuration": {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.Configuration })},
		"selfcheck":     {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.SelfCheck })},
		"route":         {setter: func(e *Environment, v string) error { e.Debug.Route = v; return nil }},
		"defensive":     {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.Defensive })},
		"rotate":        {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.Rotate })},
		"timing":        {setter: setBoolField(func(e *Environment) *bool { return &e.Debug.Timing })},
		"pprof":         {setter: func(e *Environment, v string) error { e.Debug.Pprof = v; return nil }},
	},
	"chaos": {
		"seed": {
			setter: func(e *Environment, v string) error {
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid chaos seed %q: %w", v, err)
				}
				e.Chaos.Seed = n
				return nil
			},
		},
		"rate": {
			setter: func(e *Environment, v string) error {
				f, err := parseFloatStrict(v)
				if err != nil {
					return err
				}
				e.Chaos.Rate = f
				return nil
			},
			validate: validateChaosRate,
		},
	},
}

// ErrUnknownOption indicates an unknown option was encountered.
// This is used to distinguish from other errors when allowing unknown log options.
var ErrUnknownOption = fmt.Errorf("unknown option")

// SetConfigValue applies a single config value from the environment block.
// Returns ErrUnknownOption for unknown options, or other errors for parse/validation failure.
func (e *Environment) SetConfigValue(section, option, value string) error {
	section = strings.ToLower(section)
	option = strings.ToLower(option)

	sectionOpts, ok := envOptions[section]
	if !ok {
		return fmt.Errorf("unknown environment section: %s", section)
	}

	opt, ok := sectionOpts[option]
	if !ok {
		return fmt.Errorf("%w: %s.%s", ErrUnknownOption, section, option)
	}

	// Validate if validator exists
	if opt.validate != nil {
		if err := opt.validate(value); err != nil {
			return err
		}
	}

	// Set the value
	return opt.setter(e, value)
}

// loadFromEnvStrict loads values from environment variables with strict validation.
// Returns error on any parse failure instead of silently using defaults.
func (e *Environment) loadFromEnvStrict() error {
	for section, opts := range envOptions {
		for option := range opts {
			value := getEnv(section, option)
			if value == "" {
				continue
			}
			if err := e.SetConfigValue(section, option, value); err != nil {
				return fmt.Errorf("env var ze.bgp.%s.%s: %w", section, option, err)
			}
		}
	}
	return nil
}

// LoadEnvironmentWithConfig loads env: defaults → config block → OS env.
// The configValues map is section -> option -> value from parsed config.
//
// Unknown options in the "log" section are allowed - they're interpreted as
// subsystem log levels (e.g., "gr debug" → ze.log.gr=debug) and handled by
// slogutil.ApplyLogConfig() separately.
func LoadEnvironmentWithConfig(configValues map[string]map[string]string) (*Environment, error) {
	env := &Environment{}
	if err := env.loadDefaults(); err != nil {
		return nil, fmt.Errorf("load YANG defaults: %w", err)
	}

	// Apply config file values
	for section, options := range configValues {
		for option, value := range options {
			if err := env.SetConfigValue(section, option, value); err != nil {
				// Allow unknown options in "log" section - they're subsystem log levels
				// handled by slogutil.ApplyLogConfig() (e.g., "gr debug" → ze.log.gr=debug)
				if errors.Is(err, ErrUnknownOption) && section == "log" {
					continue
				}
				return nil, fmt.Errorf("config environment.%s.%s: %w", section, option, err)
			}
		}
	}

	// OS env vars override config (with strict validation)
	if err := env.loadFromEnvStrict(); err != nil {
		return nil, err
	}

	return env, nil
}
