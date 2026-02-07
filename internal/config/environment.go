// Package config provides configuration parsing for ze.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

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
}

// DaemonEnv holds daemon-related settings.
type DaemonEnv struct {
	PID       string // PID file location
	User      string // User to run as
	Daemonize bool   // Run in background
	Drop      bool   // Drop privileges before forking
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
	Passive  bool // Make all peers passive
	OpenWait int  // Seconds to wait for OPEN
}

// CacheEnv holds caching-related settings.
type CacheEnv struct {
	Attributes bool // Cache attributes
}

// APIEnv holds API-related settings.
type APIEnv struct {
	ACK        bool   // Acknowledge API commands
	Chunk      int    // Max lines before yield
	Encoder    string // Encoder type: json
	Compact    bool   // Compact JSON for INET
	Respawn    bool   // Respawn dead processes
	Terminate  bool   // Terminate if process dies
	CLI        bool   // Create CLI named pipe
	PipeName   string // Name for CLI pipe
	SocketName string // Name for Unix socket
}

// ReactorEnv holds reactor-related settings.
type ReactorEnv struct {
	Speed float64 // Reactor loop time multiplier
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
}

// LoadEnvironment loads configuration from environment variables.
// Returns error for invalid env var values (BREAKING CHANGE from silent ignore).
// Use LoadEnvironmentWithConfig(nil) for the same behavior with explicit error handling.
func LoadEnvironment() (*Environment, error) {
	return LoadEnvironmentWithConfig(nil)
}

// loadDefaults sets default values.
func (e *Environment) loadDefaults() {
	// Daemon defaults
	e.Daemon.User = "nobody"
	e.Daemon.Drop = true
	e.Daemon.Umask = 0o137

	// Log defaults
	e.Log.Enable = true
	e.Log.Level = LogLevelInfo
	e.Log.Destination = "stdout"
	e.Log.Configuration = true
	e.Log.Reactor = true
	e.Log.Daemon = true
	e.Log.Processes = true
	e.Log.Network = true
	e.Log.Statistics = true
	e.Log.Short = true

	// TCP defaults
	e.TCP.Port = 179

	// BGP defaults
	e.BGP.OpenWait = 60

	// Cache defaults
	e.Cache.Attributes = true

	// API defaults
	e.API.ACK = true
	e.API.Chunk = 1
	e.API.Encoder = "json"
	e.API.Respawn = true
	e.API.CLI = true
	e.API.PipeName = "ze-bgp"   //nolint:goconst // Default name, not worth a constant
	e.API.SocketName = "ze-bgp" //nolint:goconst // Default name, not worth a constant

	// Reactor defaults
	e.Reactor.Speed = 1.0
}

// OpenWaitDuration returns the OpenWait as a time.Duration.
func (e *Environment) OpenWaitDuration() time.Duration {
	return time.Duration(e.BGP.OpenWait) * time.Second
}

// SocketPath returns the full path to the API socket.
// Can be overridden with ze.bgp.api.socketpath or ze_bgp_api_socketpath env var.
func (e *Environment) SocketPath() string {
	if path := getEnv("api", "socketpath"); path != "" {
		return path
	}
	return "/var/run/" + e.API.SocketName + ".sock"
}

// getEnv returns the environment variable value.
// Checks both dot notation (ze.bgp.section.option) and underscore (ze_bgp_section_option).
func getEnv(section, option string) string {
	// Dot notation first (higher priority)
	dotKey := "ze.bgp." + section + "." + option
	if v := os.Getenv(dotKey); v != "" {
		return v
	}

	// Underscore notation
	underKey := strings.ReplaceAll(dotKey, ".", "_")
	return os.Getenv(underKey)
}

// =============================================================================
// Strict Parsing Functions (return errors instead of silent defaults)
// =============================================================================

// parseBoolStrict parses a boolean value strictly.
// Returns error for invalid values instead of defaulting to false.
func parseBoolStrict(value string) (bool, error) {
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
		b, err := parseBoolStrict(value)
		if err != nil {
			return err
		}
		*getter(env) = b
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
			b, err := parseBoolStrict(v)
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
		"passive":  {setter: setBoolField(func(e *Environment) *bool { return &e.BGP.Passive })},
		"openwait": {setter: setIntField(func(e *Environment) *int { return &e.BGP.OpenWait }), validate: validateOpenWait},
	},
	"cache": {
		"attributes": {setter: setBoolField(func(e *Environment) *bool { return &e.Cache.Attributes })},
	},
	"api": {
		"ack":        {setter: setBoolField(func(e *Environment) *bool { return &e.API.ACK })},
		"chunk":      {setter: setIntField(func(e *Environment) *int { return &e.API.Chunk })},
		"encoder":    {setter: func(e *Environment, v string) error { e.API.Encoder = strings.ToLower(v); return nil }, validate: validateEncoder},
		"compact":    {setter: setBoolField(func(e *Environment) *bool { return &e.API.Compact })},
		"respawn":    {setter: setBoolField(func(e *Environment) *bool { return &e.API.Respawn })},
		"terminate":  {setter: setBoolField(func(e *Environment) *bool { return &e.API.Terminate })},
		"cli":        {setter: setBoolField(func(e *Environment) *bool { return &e.API.CLI })},
		"pipename":   {setter: func(e *Environment, v string) error { e.API.PipeName = v; return nil }},
		"socketname": {setter: func(e *Environment, v string) error { e.API.SocketName = v; return nil }},
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
	env.loadDefaults()

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
