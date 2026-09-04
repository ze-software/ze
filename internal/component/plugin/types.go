// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin implements ze plugin infrastructure for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer detail, rib routes, announce/withdraw)
//   - JSON encoder for ze-bgp format output
//   - External process management for spawning and communicating with subprocesses
package plugin

import (
	"encoding/json"
	"fmt"
	"iter"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
)

// ReactorConfigurator handles configuration reload, verification, and application.
type ReactorConfigurator interface {
	// Reload reloads the configuration from the config file via reloadFunc.
	Reload() error

	// VerifyConfig validates protocol-specific settings from a config tree.
	VerifyConfig(configTree map[string]any) error

	// ApplyConfigDiff applies incremental changes from a protocol config tree.
	ApplyConfigDiff(configTree map[string]any) error

	// GetConfigTree returns the full config as a map for plugin config delivery.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)
}

// ReactorStartupCoordinator handles plugin startup protocol signaling.
type ReactorStartupCoordinator interface {
	// SignalAPIReady signals that an API process is ready.
	SignalAPIReady()

	// AddAPIProcessCount adds to the number of API processes to wait for.
	AddAPIProcessCount(count int)

	// SignalPluginStartupComplete signals that all plugin phases are done.
	SignalPluginStartupComplete()

	// SignalPeerAPIReady signals that a peer-specific API initialization is
	// complete. sender names the process that reported, because the peer's
	// End-of-RIB waits for a NAMED set of route-pushing processes and a report
	// from a process outside that set must not release it.
	SignalPeerAPIReady(peerAddr string, sender Sender)

	// SetPeerUpBarrier declares how many barrier-declaring plugins
	// (registry.Registration.PeerUpBarrier) a peer's peer-up event is being
	// delivered to. Called before the first delivery of that event.
	SetPeerUpBarrier(peerAddr string, expected int)

	// SignalPeerUpBarrier records that one barrier plugin has taken delivery of
	// a peer's peer-up event. When all expected ones have, the peer's
	// initial-sync End-of-RIB is released.
	//
	// Composed into the interface rather than reached by type assertion, for
	// the reason recorded on ReactorRelayCoordinator: a facade missing the
	// delegation must be a compile error, not a runtime miss that silently
	// drops the barrier.
	SignalPeerUpBarrier(peerAddr string)
}

// ProtocolReactor is the minimal interface any protocol reactor must implement.
// It provides lifecycle management and configuration access that the engine
// and plugin infrastructure use without knowledge of the specific protocol.
//
// Protocol-specific extensions (BGP peers, OSPF neighbors, IS-IS adjacencies)
// are expressed as separate interfaces. Consumers type-assert when they need
// protocol-specific operations.
type ProtocolReactor interface {
	// Stop signals the reactor to shut down.
	Stop()

	// Reload reloads the configuration.
	Reload() error

	// GetConfigTree returns the full config as a map for plugin config delivery.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)

	// VerifyConfig validates protocol-specific settings from a config tree.
	VerifyConfig(configTree map[string]any) error

	// ApplyConfigDiff applies incremental changes from a protocol config tree.
	ApplyConfigDiff(configTree map[string]any) error

	// SignalPluginStartupComplete signals that all plugin phases are done.
	SignalPluginStartupComplete()
}

// ResponseData is the marker interface for typed response payloads.
// Only concrete types that implement this interface can be assigned
// to Response.Data, preventing bare strings from bypassing pipe operators.
type ResponseData interface {
	responseData()
}

// Map is a named map type that satisfies ResponseData.
type Map map[string]any

func (Map) responseData() {}

// DataMarker is embedded in structs to satisfy ResponseData.
type DataMarker struct{}

func (DataMarker) responseData() {}

// Slice is a generic named slice type that satisfies ResponseData.
type Slice[T any] []T

func (Slice[T]) responseData() {}

// RawJSON holds pre-serialized JSON from an RPC boundary. Its payload MUST be
// a JSON value. It is the one ResponseData implementor built on a string, so it
// is the one route by which finished text can reach Response.Data
// (`ai/rules/cli.md`).
type RawJSON string

func (RawJSON) responseData() {}

// MarshalJSON emits the payload verbatim, and refuses one that is not JSON.
// Quoting it would make finished text a valid-looking answer, which is the
// failure a guard must never produce (`ai/rules/evidence.md`). `| json`,
// `| yaml` and `| table` would each hand the reader that same text back.
//
// An empty payload stays `null`. ExecuteCommandOutput.Data
// (`pkg/plugin/rpc/types.go`) is `omitempty`, and a command that answers with
// no data is not an error.
func (r RawJSON) MarshalJSON() ([]byte, error) {
	if r == "" {
		return []byte("null"), nil
	}
	b := []byte(r)
	if !json.Valid(b) {
		return nil, fmt.Errorf("plugin: response payload is not JSON, so no format "+
			"can render it: marshal the value instead of pre-rendering it, or answer "+
			"with plugin.Map or plugin.Slice (got %d bytes starting %.32q)", len(b), string(b))
	}
	return b, nil
}

// Records is the ResponseData a handler answers with when its rows are
// produced on demand instead of built whole before the answer opens. It is the
// one ResponseData implementor that carries no finished collection, so an
// answer nobody wants whole costs only the rows a consumer reads
// (`ai/rules/cli.md` still holds: each row is structured data, so `| json`,
// `| yaml` and `| table` stay three renderings of one payload).
//
// Key names the envelope the rows belong under, and the head line carries it.
// A handler MUST NOT name it rpc.AnswerErrorsKey, which every producer refuses
// (rpc.ErrReservedEnvelopeKey): the rejected rows travel under that name beside
// the rows the command produced, so an envelope of the same name would put the
// two collections under one key and lose one of them.
//
// Rows is pulled one row at a time; a consumer that stops ranging stops the
// walk, which is how `| first 10` bounds a read of a table with a million rows.
//
// Fields names the columns of an answer whose rows share one schema, in column
// order. A handler that declares them yields each row as a JSON array of values
// in that order, and the encoder writes the names once on the head rather than
// on every row. It declares the SCHEMA and not the wire shape: whether that
// answer is streamed at all is decided from how many rows the walk produces
// (WriteAnswer, dispatch.go). A handler that declares no fields yields
// self-describing objects.
//
// A handler that declares fields MUST yield every row as a JSON array carrying
// exactly one value for each name, in the same order. A consumer reads the two
// against each other by POSITION, so a short row would gain a column it never
// carried and a long one would lose a value. Neither is repaired: the row is
// refused at the producer, on the wire path and on the buffered one alike
// (rpc.ErrRowArity, rpc.ErrRowNotPositional). The names reach the operator
// through the same zip on both, so the document a declared schema answers is
// the document the same rows answer when each one carries its own names.
//
// A row appends its own JSON into the buffer the encoder owns (rpc.Row), so a
// walk of a million rows costs no allocation for a row. A handler MAY hand back
// one Row value for every row and refill it in place: the encoder appends it
// before the yield that carried it returns and keeps no reference to it.
type Records struct {
	Key    string
	Fields []string
	Rows   iter.Seq[rpc.RowRecord]
}

func (Records) responseData() {}

// rows is Rows with the empty sequence standing in for a nil one. A Records
// that names an envelope and carries no generator is an empty collection, which
// is what a command that produced nothing answered with; ranging a nil
// iter.Seq panics instead of saying so.
func (r Records) rows() iter.Seq[rpc.RowRecord] {
	if r.Rows == nil {
		return noRecords
	}
	return r.Rows
}

// MarshalJSON collapses the answer into the object a consumer of the record
// path holds once the terminator arrives: the rows the command produced, in
// walk order, under Key, and the rows it rejected under rpc.AnswerErrorsKey
// beside them. With no rejected row it is Key over the items, or a bare array
// when Key is empty, which is the shape a buffered surface has always seen.
//
// It is the buffered half of AC-10 in spec-streaming-answer-protocol,
// so a surface that takes the whole answer as one string (REST, gRPC, web, MCP,
// the looking glass) reads through ResponseJSON what a surface reading records
// reads through WriteAnswer. A commit that applied 97 leaves and rejected 3
// therefore renders both on a web page, rather than the 97 being lost with the
// error.
//
// The collapse itself is rpc.CollapseRecords, so the document a plugin rebuilds
// from an arriving answer is the document this renders from the same rows
// (pkg/plugin/rpc/collapse.go).
//
// This walks the whole answer into memory, which is what a buffered rendering
// is. A caller that must bound the memory takes the record path. The rows are
// held rather than written, so each one is appended into a slice of its own
// (rpc.HeldRecords), which is the allocation a walk that reaches the wire does
// not pay.
//
// Rows is walked once here, so one Records value takes one path, never both.
func (r Records) MarshalJSON() ([]byte, error) {
	return rpc.CollapseRecords(r.Key, r.Fields, rpc.HeldRecords(r.rows()))
}

// UnauthorizedMessage is the canonical operator-facing text for a command
// refused by authorization, shared by every surface that can refuse one (SSH,
// CLI, web, REST, gRPC) so an operator reads the same sentence everywhere.
//
// It must stay distinguishable from an unknown-command error: telling an
// operator "unknown command" when their profile is what blocked them sends
// them hunting for a typo that does not exist.
const UnauthorizedMessage = "command restricted by access control"

// Response represents an API command response.
// Serial is included only if command had #N prefix.
type Response struct {
	Serial  string       `json:"serial,omitempty"`  // Correlation ID (omitted if no prefix)
	Status  string       `json:"status"`            // "done", "error", or "partial"
	Partial bool         `json:"partial,omitempty"` // True for streaming chunks, false for final
	Data    ResponseData `json:"data,omitempty"`    // Typed success payload
	Error   string       `json:"error,omitempty"`   // Error message (set when Status is "error")

	transportComplete func()
}

// OnTransportComplete carries an accepted action to the transport that owns
// delivery of this response. Lifecycle commands use it to keep teardown behind
// the response boundary instead of coupling the command handler to one socket.
func (r *Response) OnTransportComplete(action func()) {
	if r == nil {
		return
	}
	r.transportComplete = action
}

// TakeTransportComplete transfers the completion action to another response
// envelope at a process boundary. The source response no longer owns it.
func (r *Response) TakeTransportComplete() func() {
	if r == nil {
		return nil
	}
	action := r.transportComplete
	r.transportComplete = nil
	return action
}

// TransportComplete runs the accepted action after the transport has completed
// response delivery. Clearing it first makes repeated completion calls harmless.
func (r *Response) TransportComplete() {
	action := r.TakeTransportComplete()
	if action != nil {
		action()
	}
}

// RouteResult is the typed payload for update-route command responses.
type RouteResult struct {
	DataMarker
	Announced uint32   `json:"announced"`
	Withdrawn uint32   `json:"withdrawn"`
	Warnings  []string `json:"warnings,omitempty"`
}

// NewResponse creates a new Response with the given status and data.
func NewResponse(status string, data ResponseData) *Response {
	return &Response{
		Status: status,
		Data:   data,
	}
}

// newErrorResponse creates an error Response with the given message.
func newErrorResponse(message string) *Response {
	return &Response{
		Status: StatusError,
		Error:  message,
	}
}

// ProcessSpawner is the interface for plugin process lifecycle management.
// Implemented by PluginManager. Used by Server to delegate process creation
// instead of creating ProcessManager directly.
type ProcessSpawner interface {
	// SpawnMore spawns additional plugin processes (for auto-load).
	SpawnMore(configs []PluginConfig) error

	// GetProcessManager returns the most recent ProcessManager.
	// Returns nil if no processes have been spawned.
	GetProcessManager() any
}

// HubServerConfig holds a named hub server block.
// Extracted from: plugin { hub { server <name> { host; port; secret; } } }.
type HubServerConfig struct {
	Name    string            // Server block name (e.g., "local", "central")
	Host    string            // Listen address
	Port    uint16            // Listen port
	Secret  string            `json:"-"` // Auth token for plugin connections
	Clients map[string]string `json:"-"` // Per-client secrets: name -> secret
	// Certificate names the pki store certificate the managed-client listener
	// on this block serves. Empty serves a self-signed certificate no client
	// can verify against a CA. Every block that accepts managed clients serves
	// the same certificate: ExtractHubConfig refuses a config where two of them
	// disagree.
	Certificate string
}

// Address returns "host:port" for net.Listen.
func (s HubServerConfig) Address() string {
	var b textbuf.Buffer
	return b.Reset().Str(s.Host).Byte(':').Uint16(s.Port).String()
}

// HubClientConfig holds a hub-level client block (outbound connection).
// Extracted from: plugin { hub { client <name> { host; port; secret; source-address; } } }.
type HubClientConfig struct {
	Name          string // Client identity name
	Host          string // Remote hub address
	Port          uint16 // Remote hub port
	Secret        string `json:"-"` // Auth token
	SourceAddress string // Optional source IP for outbound connection
	// CA names the pki ca entry holding the hub's certificate authority root,
	// for a hub whose certificate no public CA issued.
	CA string
}

// Address returns "host:port" for net.Dial.
func (c HubClientConfig) Address() string {
	var b textbuf.Buffer
	return b.Reset().Str(c.Host).Byte(':').Uint16(c.Port).String()
}

// HubConfig holds plugin transport configuration.
// Extracted from: plugin { hub { server ...; client ...; } }.
type HubConfig struct {
	Servers []HubServerConfig // Named server blocks (listeners)
	Clients []HubClientConfig // Hub-level client blocks (outbound)
}

// PluginConfig holds plugin configuration.
type PluginConfig struct {
	Name           string        // Plugin identifier
	Run            string        // Command to execute (empty for internal plugins)
	Encoder        string        // "json" or "text"
	Respawn        bool          // ExaBGP compat (prefer RespawnEnabled)
	RespawnEnabled bool          // Respawn with limit enforcement (5/60s)
	WorkDir        string        // Working directory for plugin execution
	ReceiveUpdate  bool          // Forward received UPDATEs to plugin stdin
	StageTimeout   time.Duration // Startup stall timeout: how long a stage may go with no plugin progress (0 = use default 5s)
	Internal       bool          // If true, run in-process via goroutine (ze.X plugins)
}

// Format constants for process output formatting.
const (
	FormatHex     = "hex"     // Wire bytes as hex string
	FormatBase64  = "base64"  // Wire bytes as base64
	FormatParsed  = "parsed"  // Decoded/interpreted fields only (default)
	FormatRaw     = "raw"     // Wire bytes only (hex) - alias for FormatHex
	FormatFull    = "full"    // Both parsed content AND raw bytes
	FormatSummary = "summary" // NLRI metadata only (families + announce/withdraw presence)
)

// WireEncoding specifies how wire bytes are encoded in API messages.
// Controls encoding for both inbound (events to process) and outbound (commands from process).
type WireEncoding uint8

// Wire encoding constants.
const (
	WireEncodingHex  WireEncoding = iota // Hex string (default, human-readable)
	WireEncodingB64                      // Base64 (33% overhead, compact)
	WireEncodingText                     // Parsed text (no wire bytes)
)

// Wire encoding name constants.
const (
	wireEncHex = "hex"
	wireEncB64 = "b64"
)

// String returns the encoding name.
func (e WireEncoding) String() string {
	switch e {
	case WireEncodingHex:
		return wireEncHex
	case WireEncodingB64:
		return wireEncB64
	case WireEncodingText:
		return EncodingText
	default:
		return wireEncHex
	}
}

// parseWireEncoding converts a string to WireEncoding.
// Returns error for unknown encodings.
func parseWireEncoding(s string) (WireEncoding, error) {
	switch s {
	case wireEncHex:
		return WireEncodingHex, nil
	case wireEncB64, "base64":
		return WireEncodingB64, nil
	case EncodingText:
		return WireEncodingText, nil
	default:
		return WireEncodingHex, fmt.Errorf("invalid wire encoding: %q (valid: hex, b64, text)", s)
	}
}

// Status constants for API responses.
const (
	StatusDone  = "done"
	StatusError = "error"
	StatusOK    = "ok"
)

// cmdPlugin is the "plugin" token in command strings like "ze plugin <name>".
const cmdPlugin = "plugin"
