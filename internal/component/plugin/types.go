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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"math"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
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

	// VerifyConfig validates protocol-specific settings from the whole
	// configuration tree, the tree SetConfigTree takes, not one protocol's block.
	VerifyConfig(configTree map[string]any) error

	// ApplyConfigDiff applies incremental changes from the whole configuration
	// tree, the tree SetConfigTree takes, not one protocol's block.
	ApplyConfigDiff(configTree map[string]any) error

	// GetConfigTree returns the internal running map. Public consumers MUST project active data.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)

	// ReloadRunning answers the running config a reload compares candidate
	// against. It is GetConfigTree less the runtime state the reactor keeps
	// across a reload that candidate does not declare, which SetConfigTree
	// carries forward. It does not change the running config.
	ReloadRunning(candidate map[string]any) map[string]any

	// CreatedPeers answers that runtime state: the peers created at runtime
	// that no configuration declares yet, by address, with the name each is
	// recorded under. The map is the caller's. A reload's compensation keeps
	// it with the tree it would restore, because SetConfigTree drops the peers
	// the new tree declares and setting the old tree back does not undo that.
	CreatedPeers() map[netip.Addr]string

	// RestoreCreatedPeers marks again the created peers a compensated reload
	// took over, and rebuilds from before, the tree the compensation restores,
	// a created peer the reload removed. It MUST be called before
	// ApplyConfigDiff of the restored tree, whose reconcile reads the
	// configuration file and removes a running peer the file lacks unless it is
	// marked, and again after SetConfigTree of that tree, which drops the mark
	// of every peer the tree declares. A peer of another name running at a
	// recorded address is neither marked nor replaced; the reconcile removes it,
	// and the second call rebuilds the created peer at the freed address. An
	// error is a created peer that could not be rebuilt.
	RestoreCreatedPeers(before map[string]any, created map[netip.Addr]string) error
}

// ReactorStartupCoordinator handles plugin startup protocol signaling.
type ReactorStartupCoordinator interface {
	// SignalAPIReady signals that an API process is ready.
	SignalAPIReady()

	// AddAPIProcessCount adds to the number of API processes to wait for.
	AddAPIProcessCount(count int)

	// SignalPluginStartupComplete signals that all plugin phases are done.
	// An error refuses the config the daemon loaded, and at boot the daemon
	// exits with it.
	SignalPluginStartupComplete() error

	// SignalPeerAPIReady signals that a peer-specific API initialization is
	// complete. sender names the reporting process; initialReplay is the token
	// captured from its peer-UP event. Only explicit operator no-op readiness
	// may omit the token. A stale or missing process receipt is an error.
	SignalPeerAPIReady(ctx context.Context, peerAddr string, sender Sender, initialReplay uint64) error

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

	// GetConfigTree returns the internal running map. Public consumers MUST project active data.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)

	// VerifyConfig validates protocol-specific settings from the whole
	// configuration tree, the tree SetConfigTree takes, not one protocol's block.
	VerifyConfig(configTree map[string]any) error

	// ApplyConfigDiff applies incremental changes from the whole configuration
	// tree, the tree SetConfigTree takes, not one protocol's block.
	ApplyConfigDiff(configTree map[string]any) error

	// SignalPluginStartupComplete signals that all plugin phases are done.
	// An error refuses the config the daemon loaded, and at boot the daemon
	// exits with it.
	SignalPluginStartupComplete() error
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
// A JSON scalar is refused too: the payload MUST be an object or an array,
// or `null`, which an RPC boundary passes for a command with no data.
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
	// A top-level scalar is valid JSON and still finished text: a Go string
	// marshaled at the RPC boundary arrives as `"text"`, and `| yaml` and
	// `| table` unmarshal it to a scalar and print it back.
	if first := bytes.TrimSpace(b); first[0] != '{' && first[0] != '[' && !bytes.Equal(first, []byte("null")) {
		return nil, fmt.Errorf("plugin: response payload must be a JSON object or an array, "+
			"so every format can render it: answer with plugin.Map or plugin.Slice "+
			"(got %d bytes starting %.32q)", len(b), string(b))
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

// Config permission strings: the commands an authorizer is asked about before
// a configuration change. They are declared once here because every surface
// that changes configuration asks with them: the SSH session editor (through
// the verb table in internal/component/cli), the web editor and web terminal,
// the REST and gRPC config sessions and the SSH config upload. Operators'
// profile entries and a TACACS+ server's command authorization match these
// exact strings, so changing one changes what every profile allows.
//
// Every one is judged against the edit section of a profile: none starts with
// a read verb.
const (
	ConfigAuthEdit     = "config edit"
	ConfigAuthSet      = "config set"
	ConfigAuthAdd      = "config add"
	ConfigAuthDelete   = "config delete"
	ConfigAuthRename   = "config rename"
	ConfigAuthCommit   = "config commit"
	ConfigAuthDiscard  = "config discard"
	ConfigAuthRollback = "config rollback"
	ConfigAuthSave     = "config save"
)

// ConfigAuthStrings returns every config permission string above, for a
// reader that needs the whole set: the check that warns about a profile entry
// matching nothing Ze authorizes reads it, so a string added above is known
// there without a second list. A new slice on each call, so no caller can
// change what another one sees.
func ConfigAuthStrings() []string {
	return []string{
		ConfigAuthEdit,
		ConfigAuthSet,
		ConfigAuthAdd,
		ConfigAuthDelete,
		ConfigAuthRename,
		ConfigAuthCommit,
		ConfigAuthDiscard,
		ConfigAuthRollback,
		ConfigAuthSave,
	}
}

// Response represents an API command response.
// Serial is included only if command had #N prefix.
type Response struct {
	Serial  string       `json:"serial,omitempty"`  // Correlation ID (omitted if no prefix)
	Status  string       `json:"status"`            // "done", "error", or "partial"
	Partial bool         `json:"partial,omitempty"` // True for streaming chunks, false for final
	Data    ResponseData `json:"data,omitempty"`    // Typed success payload
	Error   string       `json:"error,omitempty"`   // Error message (set when Status is "error")
	// Warnings are the lines a command states about an answer it still gave,
	// one line each, without the newline: the parse warnings `show config
	// dump` reports for a config it resolved. The local route prints them on
	// stderr before the answer, so every surface that renders a response
	// shows them beside its data; a dropped warning is a silently wrong
	// answer. A failure or refused record outranks a warning's status while
	// these lines still accompany the structured answer.
	Warnings []string `json:"warnings,omitempty"`

	// recordAnswer retains a consumed plugin walk's original outcome beside
	// its document. Only CompleteRecordAnswer publishes an owned snapshot.
	recordAnswer *rpc.ExecuteCommandRecordAnswer

	transportComplete func()
}

// WarningText is the warnings of r as the text the local route prints: one
// line each, joined by newlines, with no final newline. It is empty for a nil
// response and for one that states no warning.
func (r *Response) WarningText() string {
	if r == nil {
		return ""
	}
	return strings.Join(r.Warnings, "\n")
}

// refusalErrorPrefix opens the refusal text a command writes for an
// operator, and every surface's own rendering of an error.
const refusalErrorPrefix = "error: "

// RefusalError is a refusal whose operator text the command wrote whole, as
// the local route prints it on stderr. A local-data handler served by the
// daemon refuses with one (pluginserver localAnswerResponse).
//
// Error answers the reason a program reads, without the "error: " the text
// opens with, which is what Response.Error carries. A surface that prints an
// error for a person prints Line instead of adding its own prefix, so a
// refusal that is a usage page ("usage: show config list") reads as it reads
// on the local route rather than as "error: usage: ...". RefusalLine is that
// test.
type RefusalError struct {
	line string
}

// NewRefusalError builds the refusal whose operator text is diagnostic. The
// final newline the local route prints is dropped, because each surface ends
// its line its own way. diagnostic MUST NOT be empty: a refusal with no text
// has nothing to print verbatim, and its caller reports it as a plain error.
func NewRefusalError(diagnostic string) *RefusalError {
	return &RefusalError{line: strings.TrimSuffix(diagnostic, "\n")}
}

// Error answers the reason without the leading "error: ".
func (e *RefusalError) Error() string {
	return strings.TrimPrefix(e.line, refusalErrorPrefix)
}

// Line answers the operator text exactly as the local route prints it,
// without the final newline.
func (e *RefusalError) Line() string {
	return e.line
}

// RefusalLine answers the operator text of err when err is, or wraps, a
// RefusalError, and false for every other error, which the surface renders
// its own way ("error: " and the text).
func RefusalLine(err error) (string, bool) {
	if refusal, ok := errors.AsType[*RefusalError](err); ok {
		return refusal.Line(), true
	}
	return "", false
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
	Announced  uint32               `json:"announced"`
	Withdrawn  uint32               `json:"withdrawn"`
	Warnings   []string             `json:"warnings,omitempty"`
	Admissions []rpc.RouteAdmission `json:"admissions,omitempty"`
	Failure    string               `json:"failure,omitempty"`
}

// NewResponse creates a new Response with the given status and data.
func NewResponse(status string, data ResponseData) *Response {
	return &Response{
		Status: status,
		Data:   data,
	}
}

// OperatorErrorResponse is the one route an operator error takes into a
// command's error answer. Response.Error is wire, a plain string every
// surface prints, so the operator error is rendered whole into it: the
// problem, the why with its cause, and the "fix: " line last, the text a Go
// error chain carries (docs/architecture/cli/error-surface.md, "The operator
// error"). Taking *opererr.Error rather than error makes the type the proof,
// so `./le arch operator-error` counts every call as typed without reading the
// argument.
//
// e MUST NOT be nil: a refusal with no error is a Ze defect, and an answer
// with an empty Error would reach the operator as a refusal that says nothing.
func OperatorErrorResponse(e *opererr.Error) *Response {
	if e == nil {
		panic("BUG: plugin.OperatorErrorResponse called with a nil operator error")
	}
	return &Response{
		Status: StatusError,
		Error:  e.Error(),
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

	// SealProcessAdmission permanently closes process creation without waiting
	// for SpawnMore. The caller MUST read GetProcessManager after sealing, join
	// that owner, then invoke the returned function to cancel its parent.
	// GetProcessManager MUST remain available while SpawnMore is blocked.
	SealProcessAdmission() func()

	// GetProcessManager returns the most recent ProcessManager.
	// Returns nil if no processes have been spawned.
	GetProcessManager() any
}

// ManagedClientsMax bounds the number of distinct devices a hub can manage
// across all its server blocks. A device may appear on more than one block.
const ManagedClientsMax = 4096

// HubServerConfig holds a named hub server block.
// Extracted from: bus { hub { server <name> { ip; port; secret; } } }.
type HubServerConfig struct {
	Name    string            // Server block name (e.g., "local", "central")
	Host    string            // Listen address
	Port    uint16            // Listen port
	Secret  string            `json:"-"` // Auth token for plugin connections
	Clients map[string]string `json:"-"` // Per-client secrets: name -> secret
	// ClientLabels holds the labels declared for each named client. A client
	// with no labels has no entry.
	ClientLabels map[string]map[string]string
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
// Extracted from: bus { hub { client <name> { host; port; secret; source-address; } } }.
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

// HubHistoryConfig holds hub-side fleet metric sampling and history storage
// settings extracted from: bus { hub { history { ... } } }.
// IntervalSeconds == 0 disables sampling.
type HubHistoryConfig struct {
	IntervalSeconds   int
	RetentionDays     int
	DeviceBudgetBytes int64
	HubBudgetBytes    int64
}

// DefaultHubHistoryConfig returns the default fleet metric sampling and history
// storage settings matching ze-plugin-conf.yang defaults.
func DefaultHubHistoryConfig() HubHistoryConfig {
	return HubHistoryConfig{
		IntervalSeconds:   fleet.DefaultSampleIntervalSeconds,
		RetentionDays:     fleet.DefaultHistoryRetentionDays,
		DeviceBudgetBytes: fleet.DefaultHistoryPerDeviceBytes,
		HubBudgetBytes:    fleet.DefaultHistoryHubBytes,
	}
}

// ValidateHubHistoryConfig validates sampling interval, retention horizon, and
// per-device and hub-wide byte budgets against the number of distinct declared
// managed devices. When HubBudgetBytes is too small to reserve latest-sample
// space for the declared estate, the returned operator error states the exact
// minimum required space in bytes.
func ValidateHubHistoryConfig(cfg HubHistoryConfig, declaredClients int) error {
	var tb textbuf.Buffer
	if declaredClients < 0 {
		what := tb.Reset().Str("invalid declared managed client count ").Int(int64(declaredClients)).String()
		return opererr.New(what, "declared managed client count cannot be negative", "check bus hub server client declarations")
	}
	if cfg.IntervalSeconds < 0 {
		what := tb.Reset().Str("bus hub history interval ").Int(int64(cfg.IntervalSeconds)).Str(" is negative").String()
		return opererr.New(what, "sample interval must be 0 (disabled) or between 1 and 3600 seconds", "set bus hub history interval 60")
	}
	if cfg.IntervalSeconds > 0 {
		if cfg.IntervalSeconds < fleet.MinSampleIntervalSeconds {
			what := tb.Reset().Str("bus hub history interval ").Int(int64(cfg.IntervalSeconds)).Str(" is below minimum ").Int(fleet.MinSampleIntervalSeconds).Str(" seconds").String()
			fix := tb.Reset().Str("set bus hub history interval ").Int(fleet.MinSampleIntervalSeconds).String()
			return opererr.New(what, "sample interval must be 0 (disabled) or at least the minimum interval", fix)
		}
	}
	if cfg.IntervalSeconds > fleet.MaxSampleIntervalSeconds {
		what := tb.Reset().Str("bus hub history interval ").Int(int64(cfg.IntervalSeconds)).Str(" exceeds maximum ").Int(fleet.MaxSampleIntervalSeconds).Str(" seconds").String()
		fix := tb.Reset().Str("set bus hub history interval ").Int(fleet.MaxSampleIntervalSeconds).String()
		return opererr.New(what, "sample interval cannot exceed one hour", fix)
	}
	if cfg.RetentionDays < fleet.MinHistoryRetentionDays {
		what := tb.Reset().Str("bus hub history retention-days ").Int(int64(cfg.RetentionDays)).Str(" is below minimum ").Int(fleet.MinHistoryRetentionDays).String()
		fix := tb.Reset().Str("set bus hub history retention-days ").Int(fleet.MinHistoryRetentionDays).String()
		return opererr.New(what, "history retention must be between 1 and 7 days", fix)
	}
	if cfg.RetentionDays > fleet.MaxHistoryRetentionDays {
		what := tb.Reset().Str("bus hub history retention-days ").Int(int64(cfg.RetentionDays)).Str(" exceeds maximum ").Int(fleet.MaxHistoryRetentionDays).String()
		fix := tb.Reset().Str("set bus hub history retention-days ").Int(fleet.MaxHistoryRetentionDays).String()
		return opererr.New(what, "history retention cannot exceed 7 days", fix)
	}
	const (
		minReserve            = int64(fleet.MinHistoryLatestReservePerDeviceBytes)
		hubHistoryBytesSuffix = " bytes"
	)
	if cfg.DeviceBudgetBytes < minReserve {
		what := tb.Reset().Str("bus hub history device-budget-bytes ").Int(cfg.DeviceBudgetBytes).Str(" is below minimum required ").Int(minReserve).Str(hubHistoryBytesSuffix).String()
		why := tb.Reset().Str("each device requires at least ").Int(minReserve).Str(" bytes to retain its latest observation and metadata").String()
		fix := tb.Reset().Str("set bus hub history device-budget-bytes ").Int(minReserve).String()
		return opererr.New(what, why, fix)
	}
	if int64(declaredClients) > math.MaxInt64/minReserve {
		return opererr.New("bus hub declared managed client count exceeds budget accounting range",
			"hub history reserve calculation overflowed",
			"reduce declared bus hub server client entries")
	}
	minHubBytes := max(minReserve, int64(declaredClients)*minReserve)
	if cfg.HubBudgetBytes < minHubBytes {
		what := tb.Reset().Str("bus hub history hub-budget-bytes ").Int(cfg.HubBudgetBytes).Str(" is below minimum required ").Int(minHubBytes).Str(hubHistoryBytesSuffix).String()
		var why string
		if declaredClients > 0 {
			why = tb.Reset().Str("the hub must reserve at least ").Int(minReserve).Str(" bytes per declared managed device (").Int(int64(declaredClients)).Str(" declared devices require at least ").Int(minHubBytes).Str(" bytes)").String()
		} else {
			why = tb.Reset().Str("the hub history budget must be at least ").Int(minHubBytes).Str(hubHistoryBytesSuffix).String()
		}
		fix := tb.Reset().Str("set bus hub history hub-budget-bytes ").Int(minHubBytes).String()
		return opererr.New(what, why, fix)
	}
	return nil
}

// Default and range constants for bus hub alarm settings matching
// ze-plugin-conf.yang.
const (
	DefaultAlarmOfflineDelaySeconds     = 300
	MinAlarmOfflineDelaySeconds         = 1
	MaxAlarmOfflineDelaySeconds         = 86400
	DefaultAlarmResourcePercent         = 90
	MinAlarmResourcePercent             = 1
	MaxAlarmResourcePercent             = 100
	DefaultAlarmResourceDurationSeconds = 300
	MinAlarmResourceDurationSeconds     = 1
	MaxAlarmResourceDurationSeconds     = 86400
	DefaultAlarmSoftwareBehindVersions  = 1
	MinAlarmSoftwareBehindVersions      = 1
	MaxAlarmSoftwareBehindVersions      = 1000
	DefaultAlarmRetentionDays           = 7
	MinAlarmRetentionDays               = 1
	MaxAlarmRetentionDays               = 30
	DefaultAlarmMaxEpisodesPerDevice    = 512
	MinAlarmMaxEpisodesPerDevice        = 1
	MaxAlarmMaxEpisodesPerDevice        = 512
	DefaultAlarmMaxEpisodesHub          = 65536
	MinAlarmMaxEpisodesHub              = 1
	MaxAlarmMaxEpisodesHub              = 65536

	alarmUnitPercent = "percent"
)

// HubAlarmsConfig holds hub-side fleet alarm threshold, grace delay, and
// history retention settings extracted from: bus { hub { alarms { ... } } }.
type HubAlarmsConfig struct {
	OfflineDelaySeconds     int
	ResourceCPUPercent      int
	ResourceMemoryPercent   int
	ResourceStoragePercent  int
	ResourceDurationSeconds int
	SoftwareBehindVersions  int
	RetentionDays           int
	MaxEpisodesPerDevice    int
	MaxEpisodesHub          int
}

// DefaultHubAlarmsConfig returns the default fleet alarm threshold, delay, and
// history retention settings matching ze-plugin-conf.yang defaults.
func DefaultHubAlarmsConfig() HubAlarmsConfig {
	return HubAlarmsConfig{
		OfflineDelaySeconds:     DefaultAlarmOfflineDelaySeconds,
		ResourceCPUPercent:      DefaultAlarmResourcePercent,
		ResourceMemoryPercent:   DefaultAlarmResourcePercent,
		ResourceStoragePercent:  DefaultAlarmResourcePercent,
		ResourceDurationSeconds: DefaultAlarmResourceDurationSeconds,
		SoftwareBehindVersions:  DefaultAlarmSoftwareBehindVersions,
		RetentionDays:           DefaultAlarmRetentionDays,
		MaxEpisodesPerDevice:    DefaultAlarmMaxEpisodesPerDevice,
		MaxEpisodesHub:          DefaultAlarmMaxEpisodesHub,
	}
}

// ValidateHubAlarmsConfig validates every hub alarm delay, threshold, and
// episode history retention bound, returning an operator error naming the
// exact set bus hub alarms fix command on any out-of-range value.
func ValidateHubAlarmsConfig(cfg HubAlarmsConfig) error {
	if err := validateAlarmIntRange("offline-delay", cfg.OfflineDelaySeconds,
		MaxAlarmOfflineDelaySeconds, "seconds",
		"offline delay must be between 1 and 86400 seconds"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("resource-cpu-percent", cfg.ResourceCPUPercent,
		MaxAlarmResourcePercent, alarmUnitPercent,
		"CPU resource threshold must be between 1 and 100 percent"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("resource-memory-percent", cfg.ResourceMemoryPercent,
		MaxAlarmResourcePercent, alarmUnitPercent,
		"memory resource threshold must be between 1 and 100 percent"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("resource-storage-percent", cfg.ResourceStoragePercent,
		MaxAlarmResourcePercent, alarmUnitPercent,
		"storage resource threshold must be between 1 and 100 percent"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("resource-duration", cfg.ResourceDurationSeconds,
		MaxAlarmResourceDurationSeconds, "seconds",
		"resource duration must be between 1 and 86400 seconds"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("software-behind-versions", cfg.SoftwareBehindVersions,
		MaxAlarmSoftwareBehindVersions, "",
		"software-behind threshold must be between 1 and 1000 versions"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("retention-days", cfg.RetentionDays,
		MaxAlarmRetentionDays, "days",
		"alarm history retention must be between 1 and 30 days"); err != nil {
		return err
	}
	if err := validateAlarmIntRange("max-episodes-per-device", cfg.MaxEpisodesPerDevice,
		MaxAlarmMaxEpisodesPerDevice, "",
		"per-device retained alarm episodes must be between 1 and 512"); err != nil {
		return err
	}
	return validateAlarmIntRange("max-episodes-hub", cfg.MaxEpisodesHub,
		MaxAlarmMaxEpisodesHub, "",
		"hub-wide retained alarm episodes must be between 1 and 65536")
}

func validateAlarmIntRange(leaf string, val, maxVal int, unit, why string) error {
	const minVal = 1
	var tb textbuf.Buffer
	if val < minVal {
		tb.Reset().Str("bus hub alarms ").Str(leaf).Byte(' ').Int(int64(val)).Str(" is below minimum ").Int(int64(minVal))
		if unit != "" {
			tb.Byte(' ').Str(unit)
		}
		what := tb.String()
		fix := tb.Reset().Str("set bus hub alarms ").Str(leaf).Byte(' ').Int(int64(minVal)).String()
		return opererr.New(what, why, fix)
	}
	if val > maxVal {
		tb.Reset().Str("bus hub alarms ").Str(leaf).Byte(' ').Int(int64(val)).Str(" exceeds maximum ").Int(int64(maxVal))
		if unit != "" {
			tb.Byte(' ').Str(unit)
		}
		what := tb.String()
		fix := tb.Reset().Str("set bus hub alarms ").Str(leaf).Byte(' ').Int(int64(maxVal)).String()
		return opererr.New(what, why, fix)
	}
	return nil
}

// Default and range constants for bus hub fleet audit settings matching
// ze-plugin-conf.yang.
const (
	DefaultAuditMaxEntries = 10000
	MinAuditMaxEntries     = 100
	MaxAuditMaxEntries     = 100000
)

// HubAuditConfig holds hub-side fleet audit log retention settings extracted
// from: bus { hub { audit { ... } } }.
type HubAuditConfig struct {
	MaxEntries int
}

// DefaultHubAuditConfig returns the default fleet audit log retention settings
// matching ze-plugin-conf.yang defaults.
func DefaultHubAuditConfig() HubAuditConfig {
	return HubAuditConfig{
		MaxEntries: DefaultAuditMaxEntries,
	}
}

// ValidateHubAuditConfig validates the hub fleet audit retention ceiling,
// returning an operator error naming the exact set bus hub audit fix command
// on any out-of-range value.
func ValidateHubAuditConfig(cfg HubAuditConfig) error {
	var tb textbuf.Buffer
	if cfg.MaxEntries < MinAuditMaxEntries {
		what := tb.Reset().Str("bus hub audit max-entries ").Int(int64(cfg.MaxEntries)).Str(" is below minimum ").Int(MinAuditMaxEntries).String()
		fix := tb.Reset().Str("set bus hub audit max-entries ").Int(MinAuditMaxEntries).String()
		return opererr.New(what, "fleet audit retention must be between 100 and 100000 entries", fix)
	}
	if cfg.MaxEntries > MaxAuditMaxEntries {
		what := tb.Reset().Str("bus hub audit max-entries ").Int(int64(cfg.MaxEntries)).Str(" exceeds maximum ").Int(MaxAuditMaxEntries).String()
		fix := tb.Reset().Str("set bus hub audit max-entries ").Int(MaxAuditMaxEntries).String()
		return opererr.New(what, "fleet audit retention cannot exceed 100000 entries", fix)
	}
	return nil
}

// HubConfig holds bus hub transport, metric history, alarm, and audit configuration.
// Extracted from: bus { hub { server ...; client ...; history { ... }; alarms { ... }; audit { ... } } }.
type HubConfig struct {
	Servers []HubServerConfig // Named server blocks (listeners)
	Clients []HubClientConfig // Hub-level client blocks (outbound)
	History HubHistoryConfig  // Managed device sample interval, retention, and storage budgets
	Alarms  HubAlarmsConfig   // Managed device alarm thresholds, delays, and history retention
	Audit   HubAuditConfig    // Managed fleet audit log retention ceiling
}

// RespawnRequest is what a plugin block's `respawn` leaf asks for. The leaf is
// ExaBGP's spelling, kept because an ExaBGP process block carries it and ze
// accepts the migrated configuration (internal/exabgp/migration/migrate.go).
//
// It has three states rather than two because "the operator wrote nothing" and
// "the operator wrote false" are different answers. Silence leaves the decision
// to the plugin, which declares its own failure policy in Stage 1. A written
// value is an instruction ze either honors or refuses to start with: the
// plugin's declaration is the constraint, and this leaf is a request inside it
// (rpc.FailurePolicy).
type RespawnRequest uint8

const (
	// RespawnUnstated is a block with no `respawn` leaf, which is every internal
	// plugin and every external block that did not write one.
	RespawnUnstated RespawnRequest = 0
	// RespawnAsked is `respawn true`. It agrees with a plugin that declares
	// rpc.FailureRestart, and it stops the daemon against a plugin that does
	// not, because ze cannot honor it.
	RespawnAsked RespawnRequest = 1
	// RespawnDeclined is `respawn false`. It asks for less than any declaration
	// permits, so it never disagrees: a plugin that would have been started
	// again is left stopped instead.
	RespawnDeclined RespawnRequest = 2
)

const (
	// DefaultDeliveryQueueMax is the default lossless FIFO delivery queue
	// ceiling for non-cache-consumer endpoints when delivery-queue-max is unset.
	DefaultDeliveryQueueMax = 65536
	// DefaultCacheConsumerQueueMax is the default lossless FIFO delivery queue
	// ceiling for CacheConsumer forwarding plugins (such as bgp-rs and bgp-rr)
	// when delivery-queue-max is unset.
	DefaultCacheConsumerQueueMax = 256
	// MinDeliveryQueueMax is the minimum permitted value of delivery-queue-max.
	MinDeliveryQueueMax = 64
	// MaxDeliveryQueueMax is the maximum permitted value of delivery-queue-max.
	MaxDeliveryQueueMax = 1048576
)

// PluginConfig holds endpoint process configuration. Bus send/receive filter
// blocks live outside PluginConfig so live filter reloads never trip
// external-process declaration change refusals.
type PluginConfig struct {
	Name             string         // Endpoint identifier.
	CompiledPlugin   string         // Explicit compiled registration name; identifies the compiled plugin regardless of Name or SeparateProcess.
	SeparateProcess  bool           // Run a compiled-in plugin as a separate forked process over TLS connect-back.
	Run              string         // Command to execute (empty for compiled-in plugins unless using legacy Internal+Run spelling).
	Encoder          string         // "json" or "text".
	Respawn          RespawnRequest // What the block's `respawn` leaf asked for.
	OnExit           string         // "" or "withdraw" withdraws originated routes on exit; "keep" keeps them (ExaBGP compatibility).
	WorkDir          string         // Working directory for plugin execution.
	ReceiveUpdate    bool           // Forward received UPDATEs to plugin stdin.
	StageTimeout     time.Duration  // Startup stall timeout: how long a stage may go with no plugin progress (0 = use default 5s).
	DeliveryQueueMax int            // Bounded lossless FIFO delivery queue ceiling (0 = role-appropriate default).
	Internal         bool           // In-daemon compiled flag.
}

// CompiledIdentity returns the compiled registration identity this config
// runs, or empty string when the config is not a compiled-in plugin.
//
// A foreign external program (Run != "" with neither CompiledPlugin nor
// Internal set) NEVER inherits a compiled registration identity from its
// endpoint Name or Run command.
func (p PluginConfig) CompiledIdentity() string {
	if p.CompiledPlugin != "" {
		return strings.TrimPrefix(p.CompiledPlugin, "ze.")
	}
	if p.Internal {
		if p.Run != "" {
			cand := strings.TrimPrefix(p.Run, "ze.")
			if registry.Has(cand) {
				return cand
			}
			if p.Name != "" && registry.Has(p.Name) {
				return p.Name
			}
			return cand
		}
		return p.Name
	}
	if p.Run != "" {
		return ""
	}
	if p.Name != "" && registry.Has(p.Name) {
		return p.Name
	}
	return ""
}

// IsCompiled reports whether this config identifies a compiled-in plugin
// registration, regardless of whether it runs in-daemon or in a separate
// process.
func (p PluginConfig) IsCompiled() bool {
	return p.CompiledIdentity() != ""
}

// RunsInDaemon reports whether this config runs a compiled-in plugin as a
// goroutine inside the daemon process.
func (p PluginConfig) RunsInDaemon() bool {
	return p.IsCompiled() && !p.SeparateProcess
}

// RunsSeparateProcess reports whether this config runs a compiled-in plugin
// as a separate forked process executing Ze's own binary over TLS connect-back.
func (p PluginConfig) RunsSeparateProcess() bool {
	return p.IsCompiled() && p.SeparateProcess
}

// RunsExternalProgram reports whether this config runs a foreign external
// program Ze did not ship. Its identity is its endpoint name alone, and it
// takes nothing from any compiled-in registration that shares that name.
func (p PluginConfig) RunsExternalProgram() bool {
	return !p.IsCompiled() && p.Run != ""
}

// RunsForked reports whether this config spawns an OS child process over TLS
// connect-back, either a compiled-in plugin with SeparateProcess or a foreign
// external program.
func (p PluginConfig) RunsForked() bool {
	return p.RunsSeparateProcess() || p.RunsExternalProgram()
}

// Format constants for process output formatting.
const (
	FormatHex     = "hex"     // Wire bytes as hex string
	FormatBase64  = "base64"  // Positional space-delimited base64 UPDATE lines beside JSON control events.
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
		panic("BUG: unknown wire encoding")
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
	// StatusWarning is a command that completed and states a warning. Every
	// transport carries it: the answer's terminator spells it wrn
	// (rpc.AnswerKindWarning), and DirectBridge carries the status itself.
	StatusWarning = rpc.StatusWarning
)
