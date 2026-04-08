// Design: docs/architecture/api/process-protocol.md — plugin process management
// Detail: event_monitor.go — monitor event streaming handler

package server

import (
	"context"
	"io"
	"sort"
	"strings"
	"sync"
)

// StreamingHandler handles streaming commands (e.g., monitor).
// ctx is the session context, s is the plugin server, w is the output writer,
// username is the authenticated SSH user (for authorization), args are command arguments.
type StreamingHandler func(ctx context.Context, s *Server, w io.Writer, username string, args []string) error

// streamingHandlers maps command prefix to handler. Multiple streaming commands
// can coexist (e.g., "monitor event", "monitor bgp"). Protected by streamingHandlersMu.
var (
	streamingHandlersMu sync.RWMutex
	streamingHandlers   = make(map[string]StreamingHandler)
)

// RegisterStreamingHandler registers a streaming command handler for a prefix.
// The prefix is matched case-insensitively against command input.
// Called from plugin init() functions.
func RegisterStreamingHandler(prefix string, h StreamingHandler) {
	if strings.TrimSpace(prefix) == "" {
		logger().Error("RegisterStreamingHandler called with empty prefix, ignoring")
		return
	}
	if h == nil {
		logger().Error("RegisterStreamingHandler called with nil handler", "prefix", prefix)
		return
	}
	key := strings.ToLower(prefix)
	streamingHandlersMu.Lock()
	if _, exists := streamingHandlers[key]; exists {
		logger().Warn("duplicate streaming handler prefix, overwriting", "prefix", prefix)
	}
	streamingHandlers[key] = h
	streamingHandlersMu.Unlock()
}

// GetStreamingHandlerForCommand returns the handler and extracted args for a command.
// Matches the longest registered prefix. Returns (nil, nil) if no prefix matches.
func GetStreamingHandlerForCommand(input string) (StreamingHandler, []string) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)

	streamingHandlersMu.RLock()
	defer streamingHandlersMu.RUnlock()

	var bestPrefix string
	var bestHandler StreamingHandler

	for prefix, handler := range streamingHandlers {
		if lower == prefix || strings.HasPrefix(lower, prefix+" ") {
			if len(prefix) > len(bestPrefix) {
				bestPrefix = prefix
				bestHandler = handler
			}
		}
	}

	if bestHandler == nil {
		return nil, nil
	}

	// Extract args after the matched prefix from the original trimmed input
	// (not the lowered version) to preserve case of peer selectors and arguments.
	if len(trimmed) <= len(bestPrefix) {
		return bestHandler, nil
	}
	rest := strings.TrimSpace(trimmed[len(bestPrefix):])
	if rest == "" {
		return bestHandler, nil
	}
	return bestHandler, strings.Fields(rest)
}

// IsStreamingCommand returns true if the input matches any registered streaming prefix.
func IsStreamingCommand(input string) bool {
	h, _ := GetStreamingHandlerForCommand(input)
	return h != nil
}

// StreamingPrefixes returns the registered streaming command prefixes, sorted.
func StreamingPrefixes() []string {
	streamingHandlersMu.RLock()
	defer streamingHandlersMu.RUnlock()
	prefixes := make([]string, 0, len(streamingHandlers))
	for p := range streamingHandlers {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)
	return prefixes
}

// monitorEventFormatter is a registered function that transforms raw JSON event lines
// into compact human-readable one-liners for terminal display.
// Set via RegisterMonitorEventFormatter from plugin init(). Returns raw input on failure.
var (
	monitorEventFormatterMu sync.RWMutex
	monitorEventFormatter   func(string) string
)

// RegisterMonitorEventFormatter registers the function that formats raw JSON event
// lines into compact one-liners for monitor streaming output (both CLI and TUI).
// Called from the monitor plugin's init().
func RegisterMonitorEventFormatter(fn func(string) string) {
	monitorEventFormatterMu.Lock()
	monitorEventFormatter = fn
	monitorEventFormatterMu.Unlock()
}

// MonitorEventFormatter returns the registered event formatter, or nil if none is registered.
func MonitorEventFormatter() func(string) string {
	monitorEventFormatterMu.RLock()
	defer monitorEventFormatterMu.RUnlock()
	return monitorEventFormatter
}

// version is ze application version string, set by main at startup via SetVersion.
var version = "dev"

// buildDate is the build date string, set by main at startup via SetVersion.
var buildDate = "unknown"

// SetVersion sets the application version and build date (called from main).
func SetVersion(v, d string) {
	version = v
	buildDate = d
}

// GetVersion returns the current version and build date.
func GetVersion() (string, string) {
	return version, buildDate
}

// APIVersion is the IPC protocol version.
const APIVersion = "0.1.0"

// Command source constants.
const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
	cmdPlugin     = "plugin" // "plugin" token in command strings like "ze plugin <name>"
)

// RPCRegistration maps a YANG RPC wire method to its handler function.
// The CLI command name is derived from the YANG command tree (-cmd.yang modules)
// via yang.WireMethodToPath(). It is not stored in the registration.
// Help text comes from YANG descriptions. Read-only classification comes from
// the verb position in the command tree (show/validate/monitor = read-only).
type RPCRegistration struct {
	WireMethod       string  // "module:rpc-name" format (e.g., "ze-bgp:peer-list")
	Handler          Handler // Handler function
	RequiresSelector bool    // True if peer commands must have explicit selector (not default "*")
	PluginCommand    string  // If set, this builtin proxies to a runtime plugin command (e.g., "bgp rib show")
}
