// Design: docs/architecture/api/ipc_protocol.md — plugin RPC types
// Related: text.go — text format/parse uses RPC types
// Related: message.go — RPC wire message types
//
// Package rpc defines the canonical wire-format types for the ze plugin RPC protocol.
//
// Both the engine (internal/plugin) and the SDK (pkg/plugin/sdk) import these types
// to ensure a single source of truth for the RPC message structures.
//
// The two-socket architecture uses these types in JSON-RPC messages:
//   - Socket A (plugin → engine): declare-registration, declare-capabilities, ready,
//     update-route, dispatch-command, subscribe/unsubscribe-events, decode/encode-nlri,
//     decode-mp-reach, decode-mp-unreach, decode-update
//   - Socket B (engine → plugin): configure, share-registry, deliver-event,
//     decode/encode-nlri, decode-capability, execute-command, bye
package rpc

import "encoding/json"

// Status constants for plugin API responses.
// Defined here so both internal code and pkg/plugin/sdk can use them.
const (
	StatusDone  = "done"
	StatusError = "error"
	StatusOK    = "ok"
)

// DeclareRegistrationInput is the input for ze-plugin-engine:declare-registration (Stage 1).
type DeclareRegistrationInput struct {
	Families               []FamilyDecl  `json:"families,omitempty"`
	Commands               []CommandDecl `json:"commands,omitempty"`
	Dependencies           []string      `json:"dependencies,omitempty"`
	WantsConfig            []string      `json:"wants-config,omitempty"`
	Schema                 *SchemaDecl   `json:"schema,omitempty"`
	WantsValidateOpen      bool          `json:"wants-validate-open,omitempty"`
	CacheConsumer          bool          `json:"cache-consumer,omitempty"`
	CacheConsumerUnordered bool          `json:"cache-consumer-unordered,omitempty"`
}

// FamilyDecl declares an address family the plugin handles.
type FamilyDecl struct {
	Name string `json:"name"` // e.g., "ipv4/unicast"
	Mode string `json:"mode"` // "encode", "decode", or "both"
}

// CommandDecl declares a command the plugin provides.
type CommandDecl struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Args        []string `json:"args,omitempty"`
	Completable bool     `json:"completable,omitempty"`
}

// SchemaDecl declares the YANG schema the plugin provides.
type SchemaDecl struct {
	Module    string   `json:"module"`
	Namespace string   `json:"namespace,omitempty"`
	YANGText  string   `json:"yang-text,omitempty"`
	Handlers  []string `json:"handlers,omitempty"`
}

// ConfigSection is a single config section delivered to the plugin.
type ConfigSection struct {
	Root string `json:"root"` // Config root name (e.g., "bgp")
	Data string `json:"data"` // JSON-encoded config data
}

// ConfigureInput is the input for ze-plugin-callback:configure (Stage 2).
type ConfigureInput struct {
	Sections []ConfigSection `json:"sections"`
}

// DeclareCapabilitiesInput is the input for ze-plugin-engine:declare-capabilities (Stage 3).
type DeclareCapabilitiesInput struct {
	Capabilities []CapabilityDecl `json:"capabilities"`
}

// CapabilityDecl declares a BGP capability for OPEN injection.
type CapabilityDecl struct {
	Code     uint8    `json:"code"`
	Encoding string   `json:"encoding,omitempty"` // "hex", "b64", "text"
	Payload  string   `json:"payload,omitempty"`
	Peers    []string `json:"peers,omitempty"`
}

// RegistryCommand is a command in the shared registry (Stage 4).
type RegistryCommand struct {
	Name     string `json:"name"`
	Plugin   string `json:"plugin"`
	Encoding string `json:"encoding,omitempty"`
}

// ShareRegistryInput is the input for ze-plugin-callback:share-registry (Stage 4).
type ShareRegistryInput struct {
	Commands []RegistryCommand `json:"commands"`
}

// DeliverEventInput is the input for ze-plugin-callback:deliver-event (runtime).
type DeliverEventInput struct {
	Event string `json:"event"`
}

// EncodeNLRIInput is the input for ze-plugin-callback:encode-nlri (engine→plugin)
// and ze-plugin-engine:encode-nlri (plugin→engine).
type EncodeNLRIInput struct {
	Family string   `json:"family"`
	Args   []string `json:"args,omitempty"`
}

// EncodeNLRIOutput is the output for ze-plugin-engine:encode-nlri (plugin→engine).
type EncodeNLRIOutput struct {
	Hex string `json:"hex"`
}

// DecodeNLRIInput is the input for ze-plugin-callback:decode-nlri (engine→plugin)
// and ze-plugin-engine:decode-nlri (plugin→engine).
type DecodeNLRIInput struct {
	Family string `json:"family"`
	Hex    string `json:"hex"`
}

// DecodeNLRIOutput is the output for ze-plugin-engine:decode-nlri (plugin→engine).
type DecodeNLRIOutput struct {
	JSON string `json:"json"`
}

// DecodeMPReachInput is the input for ze-plugin-engine:decode-mp-reach (plugin→engine).
// Hex is the MP_REACH_NLRI attribute value (after TLV header): AFI(2)+SAFI(1)+NHLen(1)+NH+Reserved+NLRI.
type DecodeMPReachInput struct {
	Hex     string `json:"hex"`
	AddPath bool   `json:"add-path,omitempty"`
}

// DecodeMPReachOutput is the output for ze-plugin-engine:decode-mp-reach (plugin→engine).
type DecodeMPReachOutput struct {
	Family  string          `json:"family"`
	NextHop string          `json:"next-hop,omitempty"`
	NLRI    json.RawMessage `json:"nlri"`
}

// DecodeMPUnreachInput is the input for ze-plugin-engine:decode-mp-unreach (plugin→engine).
// Hex is the MP_UNREACH_NLRI attribute value (after TLV header): AFI(2)+SAFI(1)+Withdrawn.
type DecodeMPUnreachInput struct {
	Hex     string `json:"hex"`
	AddPath bool   `json:"add-path,omitempty"`
}

// DecodeMPUnreachOutput is the output for ze-plugin-engine:decode-mp-unreach (plugin→engine).
type DecodeMPUnreachOutput struct {
	Family string          `json:"family"`
	NLRI   json.RawMessage `json:"nlri"`
}

// DecodeUpdateInput is the input for ze-plugin-engine:decode-update (plugin→engine).
// Hex is the UPDATE message body (after 19-byte BGP header): Withdrawn+Attrs+NLRI.
type DecodeUpdateInput struct {
	Hex     string `json:"hex"`
	AddPath bool   `json:"add-path,omitempty"`
}

// DecodeUpdateOutput is the output for ze-plugin-engine:decode-update (plugin→engine).
// JSON contains the ze-bgp format UPDATE, same structure as deliver-event.
type DecodeUpdateOutput struct {
	JSON string `json:"json"`
}

// DecodeCapabilityInput is the input for ze-plugin-callback:decode-capability.
type DecodeCapabilityInput struct {
	Code uint8  `json:"code"`
	Hex  string `json:"hex"`
}

// ExecuteCommandInput is the input for ze-plugin-callback:execute-command.
type ExecuteCommandInput struct {
	Serial  string   `json:"serial"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Peer    string   `json:"peer,omitempty"`
}

// ExecuteCommandOutput is the output for ze-plugin-callback:execute-command.
type ExecuteCommandOutput struct {
	Status string `json:"status"` // "done" or "error"
	Data   string `json:"data,omitempty"`
}

// UpdateRouteInput is the input for ze-plugin-engine:update-route.
type UpdateRouteInput struct {
	PeerSelector string `json:"peer-selector,omitempty"`
	Command      string `json:"command"`
}

// UpdateRouteOutput is the output for ze-plugin-engine:update-route.
type UpdateRouteOutput struct {
	PeersAffected uint32 `json:"peers-affected"`
	RoutesSent    uint32 `json:"routes-sent"`
}

// DispatchCommandInput is the input for ze-plugin-engine:dispatch-command.
// Plugins use this to invoke commands through the engine's command dispatcher,
// enabling inter-plugin communication via the standard routing mechanism.
type DispatchCommandInput struct {
	Command string `json:"command"`
}

// DispatchCommandOutput is the output for ze-plugin-engine:dispatch-command.
// Preserves the full {status, data} response from the dispatcher, unlike
// update-route which extracts only route counters.
type DispatchCommandOutput struct {
	Status string `json:"status"`         // "done" or "error"
	Data   string `json:"data,omitempty"` // JSON-encoded response data
}

// SubscribeEventsInput is the input for ze-plugin-engine:subscribe-events.
type SubscribeEventsInput struct {
	Events   []string `json:"events,omitempty"`
	Peers    []string `json:"peers,omitempty"`
	Format   string   `json:"format,omitempty"`
	Encoding string   `json:"encoding,omitempty"` // "json" (default) or "text"
}

// ReadyInput is the input for ze-plugin-engine:ready (Stage 5).
// The Subscribe field allows plugins to register event subscriptions atomically
// with startup completion, avoiding the race between SignalAPIReady and a
// separate subscribe-events RPC that would arrive after routes are already sent.
type ReadyInput struct {
	Subscribe *SubscribeEventsInput `json:"subscribe,omitempty"`
}

// ConfigVerifyInput is the input for ze-plugin-callback:config-verify.
// The engine sends the full candidate config sections for the plugin to validate.
type ConfigVerifyInput struct {
	Sections []ConfigSection `json:"sections"`
}

// ConfigVerifyOutput is the output for ze-plugin-callback:config-verify.
type ConfigVerifyOutput struct {
	Status string `json:"status"`          // "ok" or "error"
	Error  string `json:"error,omitempty"` // Reason for rejection
}

// ConfigDiffSection describes what changed in a single config root.
type ConfigDiffSection struct {
	Root    string `json:"root"`              // Config root name (e.g., "bgp")
	Added   string `json:"added,omitempty"`   // JSON-encoded added config
	Removed string `json:"removed,omitempty"` // JSON-encoded removed config
	Changed string `json:"changed,omitempty"` // JSON-encoded changed config
}

// ConfigApplyInput is the input for ze-plugin-callback:config-apply.
// The engine sends the diff between old and new config for the plugin to apply.
type ConfigApplyInput struct {
	Sections []ConfigDiffSection `json:"sections"`
}

// ConfigApplyOutput is the output for ze-plugin-callback:config-apply.
type ConfigApplyOutput struct {
	Status string `json:"status"`          // "ok" or "error"
	Error  string `json:"error,omitempty"` // Reason for failure
}

// ByeInput is the input for ze-plugin-callback:bye (shutdown).
type ByeInput struct {
	Reason string `json:"reason,omitempty"`
}

// ValidateOpenCapability is a single capability from an OPEN message,
// represented as code + raw value bytes in hex (no TLV envelope).
type ValidateOpenCapability struct {
	Code uint8  `json:"code"`
	Hex  string `json:"hex"`
}

// ValidateOpenMessage represents one side of the OPEN exchange for validation.
type ValidateOpenMessage struct {
	ASN          uint32                   `json:"asn"`
	RouterID     string                   `json:"router-id"`
	HoldTime     uint16                   `json:"hold-time"`
	Capabilities []ValidateOpenCapability `json:"capabilities,omitempty"`
}

// ValidateOpenInput is the input for ze-plugin-callback:validate-open.
// The engine sends both local and remote OPENs for the plugin to validate.
type ValidateOpenInput struct {
	Peer   string              `json:"peer"`
	Local  ValidateOpenMessage `json:"local"`
	Remote ValidateOpenMessage `json:"remote"`
}

// ValidateOpenOutput is the output for ze-plugin-callback:validate-open.
// A plugin returns Accept=true to allow the session, or Accept=false with
// NOTIFICATION code/subcode to reject it.
type ValidateOpenOutput struct {
	Accept        bool   `json:"accept"`
	NotifyCode    uint8  `json:"notify-code,omitempty"`
	NotifySubcode uint8  `json:"notify-subcode,omitempty"`
	Reason        string `json:"reason,omitempty"`
}
