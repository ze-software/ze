// Design: docs/architecture/api/process-protocol.md — plugin process management
// Detail: event_monitor.go — monitor event streaming handler

package server

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
)

// StreamingHandler handles streaming commands (e.g., monitor).
// ctx is the session context, s is the plugin server, w is the output writer,
// username is the authenticated SSH user (for authorization), and args is the
// value ValidateArgs returned for the words after the streaming prefix, so a
// handler never sees a token its command's model refuses
// (GetStreamingHandlerForCommand).
type StreamingHandler func(ctx context.Context, s *Server, w io.Writer, username string, args command.ValidatedArgs) error

// MonitorProvider creates a TUI monitor session for a streaming command.
type MonitorProvider struct {
	Prefix   string
	CreateFn func(ctx context.Context, args []string) (eventCh <-chan string, renderFn func(w, h int) string, cancel func(), err error)
}

var (
	monitorProvidersMu sync.RWMutex
	monitorProviders   = make(map[string]MonitorProvider)
)

// RegisterMonitorProvider registers a TUI monitor provider for a streaming prefix.
func RegisterMonitorProvider(p MonitorProvider) {
	key := strings.ToLower(strings.TrimSpace(p.Prefix))
	monitorProvidersMu.Lock()
	monitorProviders[key] = p
	monitorProvidersMu.Unlock()
}

// GetMonitorProvider returns a provider for the given command input, or nil.
func GetMonitorProvider(input string) (*MonitorProvider, []string) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)

	monitorProvidersMu.RLock()
	defer monitorProvidersMu.RUnlock()

	var bestPrefix string
	var bestProvider *MonitorProvider
	for prefix := range monitorProviders {
		if matchesPrefix(lower, prefix) {
			if len(prefix) > len(bestPrefix) {
				bestPrefix = prefix
				p := monitorProviders[prefix]
				bestProvider = &p
			}
		}
	}
	if bestProvider == nil {
		return nil, nil
	}
	if len(trimmed) <= len(bestPrefix) {
		return bestProvider, nil
	}
	rest := strings.TrimSpace(trimmed[len(bestPrefix):])
	if rest == "" {
		return bestProvider, nil
	}
	return bestProvider, strings.Fields(rest)
}

func matchesPrefix(input, prefix string) bool {
	return input == prefix || (strings.HasPrefix(input, prefix) && len(input) > len(prefix) && input[len(prefix)] == ' ')
}

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

// UnregisterStreamingHandler removes a previously registered streaming handler.
func UnregisterStreamingHandler(prefix string) {
	key := strings.ToLower(strings.TrimSpace(prefix))
	streamingHandlersMu.Lock()
	delete(streamingHandlers, key)
	streamingHandlersMu.Unlock()
}

// GetStreamingHandlerForCommand answers the handler for a streaming command and
// the arguments after its prefix, judged against the leaves the model declares
// for that prefix (route R8). Matches the longest registered prefix. handler is
// nil when no prefix matches, and when the arguments are refused: the refusal
// is then err, and the caller MUST report it rather than run anything. A caller
// MUST invoke the handler with the value answered.
func GetStreamingHandlerForCommand(input string) (StreamingHandler, command.ValidatedArgs, error) {
	handler, prefix, args := matchStreamingCommand(input)
	if handler == nil {
		return nil, command.ValidatedArgs{}, nil
	}
	validated, err := command.ValidateModelArgs(prefix, args, nil)
	if err != nil {
		return nil, command.ValidatedArgs{}, err
	}
	return handler, validated, nil
}

// matchStreamingCommand answers the handler registered at the longest prefix
// of input, that prefix as registered (lowercased), and the raw words after it.
// It judges nothing: it answers whether a command IS a streaming command, and
// GetStreamingHandlerForCommand judges the words before anything runs.
func matchStreamingCommand(input string) (handler StreamingHandler, prefix string, args []string) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)

	streamingHandlersMu.RLock()
	defer streamingHandlersMu.RUnlock()

	for candidate, h := range streamingHandlers {
		if lower == candidate || strings.HasPrefix(lower, candidate+" ") {
			if len(candidate) > len(prefix) {
				prefix = candidate
				handler = h
			}
		}
	}

	if handler == nil {
		return nil, "", nil
	}

	// Extract args after the matched prefix from the original trimmed input
	// (not the lowered version) to preserve case of peer selectors and arguments.
	if len(trimmed) <= len(prefix) {
		return handler, prefix, nil
	}
	rest := strings.TrimSpace(trimmed[len(prefix):])
	if rest == "" {
		return handler, prefix, nil
	}
	return handler, prefix, strings.Fields(rest)
}

// IsStreamingCommand returns true if the input matches any registered streaming
// prefix, whatever its arguments: a refused argument is reported by the
// streaming route, not by a fall-through to another one.
func IsStreamingCommand(input string) bool {
	h, _, _ := matchStreamingCommand(input)
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
	slices.Sort(prefixes)
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
	// registrar is the import path of the package that called RegisterRPCs.
	// Only RegisterRPCs writes it, and the field is private so that neither a
	// struct literal nor a caller holding the slice AllBuiltinRPCs returns can
	// claim another package's identity. OwnerPrefix derives from it the only
	// wire-method prefix that package may declare. Its zero value, "", means
	// the registration never passed RegisterRPCs, and OwnerPrefix refuses it.
	registrar string
}

// Registrar answers the import path of the package that registered r through
// RegisterRPCs, or "" for a registration that never passed it.
func (r *RPCRegistration) Registrar() string {
	return r.registrar
}
