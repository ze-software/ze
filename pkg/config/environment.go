// Package config provides configuration parsing for ZeBGP.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment holds all environment-based configuration.
// This provides ExaBGP-compatible environment variable support.
//
// Variable format: exabgp.<section>.<option> or exabgp_<section>_<option>
// Priority: exabgp.x.y > exabgp_x_y > default.
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
	Attempts int      // Max connection attempts (0 = unlimited)
	Delay    int      // Delay announcements by N minutes
	Bind     []string // IPs to bind when listening
	Port     int      // Port to bind
	ACL      bool     // Experimental ACL
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
func LoadEnvironment() *Environment {
	env := &Environment{}
	env.loadDefaults()
	env.loadFromEnv()
	return env
}

// loadDefaults sets default values.
func (e *Environment) loadDefaults() {
	// Daemon defaults
	e.Daemon.User = "nobody"
	e.Daemon.Drop = true
	e.Daemon.Umask = 0o137

	// Log defaults
	e.Log.Enable = true
	e.Log.Level = "INFO"
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
	e.API.PipeName = "zebgp"   //nolint:goconst // Default name, not worth a constant
	e.API.SocketName = "zebgp" //nolint:goconst // Default name, not worth a constant

	// Reactor defaults
	e.Reactor.Speed = 1.0
}

// loadFromEnv loads values from environment variables.
func (e *Environment) loadFromEnv() {
	// Daemon
	e.Daemon.PID = getEnvString("daemon", "pid", e.Daemon.PID)
	e.Daemon.User = getEnvString("daemon", "user", e.Daemon.User)
	e.Daemon.Daemonize = getEnvBool("daemon", "daemonize", e.Daemon.Daemonize)
	e.Daemon.Drop = getEnvBool("daemon", "drop", e.Daemon.Drop)
	e.Daemon.Umask = getEnvOctal("daemon", "umask", e.Daemon.Umask)

	// Log
	e.Log.Enable = getEnvBool("log", "enable", e.Log.Enable)
	e.Log.Level = getEnvString("log", "level", e.Log.Level)
	e.Log.Destination = getEnvString("log", "destination", e.Log.Destination)
	e.Log.All = getEnvBool("log", "all", e.Log.All)
	e.Log.Configuration = getEnvBool("log", "configuration", e.Log.Configuration)
	e.Log.Reactor = getEnvBool("log", "reactor", e.Log.Reactor)
	e.Log.Daemon = getEnvBool("log", "daemon", e.Log.Daemon)
	e.Log.Processes = getEnvBool("log", "processes", e.Log.Processes)
	e.Log.Network = getEnvBool("log", "network", e.Log.Network)
	e.Log.Statistics = getEnvBool("log", "statistics", e.Log.Statistics)
	e.Log.Packets = getEnvBool("log", "packets", e.Log.Packets)
	e.Log.RIB = getEnvBool("log", "rib", e.Log.RIB)
	e.Log.Message = getEnvBool("log", "message", e.Log.Message)
	e.Log.Timers = getEnvBool("log", "timers", e.Log.Timers)
	e.Log.Routes = getEnvBool("log", "routes", e.Log.Routes)
	e.Log.Parser = getEnvBool("log", "parser", e.Log.Parser)
	e.Log.Short = getEnvBool("log", "short", e.Log.Short)

	// TCP
	e.TCP.Attempts = getEnvInt("tcp", "attempts", e.TCP.Attempts)
	e.TCP.Delay = getEnvInt("tcp", "delay", e.TCP.Delay)
	e.TCP.Bind = getEnvStringList("tcp", "bind", e.TCP.Bind)
	e.TCP.Port = getEnvInt("tcp", "port", e.TCP.Port)
	e.TCP.ACL = getEnvBool("tcp", "acl", e.TCP.ACL)

	// Backward compatibility: tcp.once -> tcp.attempts
	if getEnvBool("tcp", "once", false) && e.TCP.Attempts == 0 {
		e.TCP.Attempts = 1
	}
	// Backward compatibility: tcp.connections -> tcp.attempts
	if v := getEnv("tcp", "connections"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			e.TCP.Attempts = n
		}
	}

	// BGP
	e.BGP.Passive = getEnvBool("bgp", "passive", e.BGP.Passive)
	e.BGP.OpenWait = getEnvInt("bgp", "openwait", e.BGP.OpenWait)

	// Cache
	e.Cache.Attributes = getEnvBool("cache", "attributes", e.Cache.Attributes)

	// API
	e.API.ACK = getEnvBool("api", "ack", e.API.ACK)
	e.API.Chunk = getEnvInt("api", "chunk", e.API.Chunk)
	e.API.Encoder = getEnvString("api", "encoder", e.API.Encoder)
	e.API.Compact = getEnvBool("api", "compact", e.API.Compact)
	e.API.Respawn = getEnvBool("api", "respawn", e.API.Respawn)
	e.API.Terminate = getEnvBool("api", "terminate", e.API.Terminate)
	e.API.CLI = getEnvBool("api", "cli", e.API.CLI)
	e.API.PipeName = getEnvString("api", "pipename", e.API.PipeName)
	e.API.SocketName = getEnvString("api", "socketname", e.API.SocketName)

	// Reactor
	e.Reactor.Speed = getEnvFloat("reactor", "speed", e.Reactor.Speed)

	// Debug
	e.Debug.PDB = getEnvBool("debug", "pdb", e.Debug.PDB)
	e.Debug.Memory = getEnvBool("debug", "memory", e.Debug.Memory)
	e.Debug.Configuration = getEnvBool("debug", "configuration", e.Debug.Configuration)
	e.Debug.SelfCheck = getEnvBool("debug", "selfcheck", e.Debug.SelfCheck)
	e.Debug.Route = getEnvString("debug", "route", e.Debug.Route)
	e.Debug.Defensive = getEnvBool("debug", "defensive", e.Debug.Defensive)
	e.Debug.Rotate = getEnvBool("debug", "rotate", e.Debug.Rotate)
	e.Debug.Timing = getEnvBool("debug", "timing", e.Debug.Timing)
}

// OpenWaitDuration returns the OpenWait as a time.Duration.
func (e *Environment) OpenWaitDuration() time.Duration {
	return time.Duration(e.BGP.OpenWait) * time.Second
}

// SocketPath returns the full path to the API socket.
func (e *Environment) SocketPath() string {
	return "/var/run/" + e.API.SocketName + ".sock"
}

// getEnv returns the environment variable value with ExaBGP naming.
// Checks both dot notation (exabgp.section.option) and underscore (exabgp_section_option).
func getEnv(section, option string) string {
	// Dot notation first (higher priority)
	dotKey := "exabgp." + section + "." + option
	if v := os.Getenv(dotKey); v != "" {
		return v
	}

	// Underscore notation
	underKey := strings.ReplaceAll(dotKey, ".", "_")
	return os.Getenv(underKey)
}

// getEnvString returns a string value from environment.
func getEnvString(section, option, def string) string {
	if v := getEnv(section, option); v != "" {
		return v
	}
	return def
}

// getEnvBool returns a boolean value from environment.
func getEnvBool(section, option string, def bool) bool {
	v := getEnv(section, option)
	if v == "" {
		return def
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on" || v == "enable" //nolint:goconst // Truthy values
}

// getEnvInt returns an integer value from environment.
func getEnvInt(section, option string, def int) int {
	v := getEnv(section, option)
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return def
}

// getEnvFloat returns a float value from environment.
func getEnvFloat(section, option string, def float64) float64 {
	v := getEnv(section, option)
	if v == "" {
		return def
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return def
}

// getEnvOctal returns an octal integer value from environment.
func getEnvOctal(section, option string, def int) int {
	v := getEnv(section, option)
	if v == "" {
		return def
	}
	// Strip leading 0 if present
	v = strings.TrimPrefix(v, "0")
	if n, err := strconv.ParseInt(v, 8, 32); err == nil {
		return int(n)
	}
	return def
}

// getEnvStringList returns a space-separated list of strings from environment.
func getEnvStringList(section, option string, def []string) []string {
	v := getEnv(section, option)
	if v == "" {
		return def
	}
	return strings.Fields(v)
}
