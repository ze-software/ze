// Design: docs/architecture/api/ipc_protocol.md — plugin RPC types
// Related: message.go — RPC wire message types
//
// Package rpc defines the canonical wire-format types for the ze plugin RPC protocol.
//
// Both the engine (internal/plugin) and the SDK (pkg/plugin/sdk) import these types
// to ensure a single source of truth for the RPC message structures.
//
// RPCs are multiplexed over a single bidirectional connection via MuxConn:
//   - Plugin-initiated: declare-registration, declare-capabilities, ready,
//     update-route, dispatch-command, emit-event, subscribe/unsubscribe-events,
//     decode/encode-nlri, decode-mp-reach, decode-mp-unreach, decode-update
//   - Engine-initiated: configure, share-registry, deliver-event,
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

// Plugin->engine runtime RPC method strings. Single source of truth: the
// engine's method registry (internal/component/plugin/server) and the plugin
// SDK (pkg/plugin/sdk) both reference these, so the string a caller sends and
// the string the engine dispatches on cannot drift (unify-rpc-dispatch, AC-4).
// Each maps to exactly one registry entry from which the JSON socket path, the
// in-process Direct path, and (where a typed descriptor is declared) the
// DirectBridge fast-path slot all derive. Codec RPCs (decode-nlri, encode-nlri,
// ...) route through the plugin registry and are not listed here.
const (
	MethodUpdateRoute         = "ze-plugin-engine:update-route"
	MethodDispatchCommand     = "ze-plugin-engine:dispatch-command"
	MethodDispatchCommandArgs = "ze-plugin-engine:dispatch-command-args"
	MethodSubscribeEvents     = "ze-plugin-engine:subscribe-events"
	MethodUnsubscribeEvents   = "ze-plugin-engine:unsubscribe-events"
	MethodEmitEvent           = "ze-plugin-engine:emit-event"
	MethodForwardCached       = "ze-plugin-engine:forward-cached"
	MethodReleaseCached       = "ze-plugin-engine:release-cached"
	MethodRelayStoredRoute    = "ze-plugin-engine:relay-stored-route"
	MethodRouteInstall        = "ze-plugin-engine:route-install"
	MethodRouteRemove         = "ze-plugin-engine:route-remove"
	MethodInjectWireRoute     = "ze-plugin-engine:inject-wire-route"
	MethodBatchValidate       = "ze-plugin-engine:batch-validate"
)

// DeclareRegistrationInput is the input for ze-plugin-engine:declare-registration (Stage 1).
type DeclareRegistrationInput struct {
	Families               []FamilyDecl          `json:"families,omitempty"`
	Commands               []CommandDecl         `json:"commands,omitempty"`
	Dependencies           []string              `json:"dependencies,omitempty"`
	WantsConfig            []string              `json:"wants-config,omitempty"`
	ConfigOperations       []ConfigOperationDecl `json:"config-operations,omitempty"`
	VerifyBudget           int                   `json:"verify-budget,omitempty"` // Estimated verify time in seconds (0 = trivial).
	ApplyBudget            int                   `json:"apply-budget,omitempty"`  // Estimated apply time in seconds (0 = trivial).
	Schema                 *SchemaDecl           `json:"schema,omitempty"`
	WantsValidateOpen      bool                  `json:"wants-validate-open,omitempty"`
	CacheConsumer          bool                  `json:"cache-consumer,omitempty"`
	CacheConsumerUnordered bool                  `json:"cache-consumer-unordered,omitempty"`
	Filters                []FilterDecl          `json:"filters,omitempty"`
	DoctorChecks           []DoctorCheckDecl     `json:"doctor-checks,omitempty"`
	Enrichers              []EnricherDecl        `json:"enrichers,omitempty"`
}

// EnricherDecl declares a show enricher the plugin provides.
// Declared during Stage 1 registration; invoked at runtime via callback.
type EnricherDecl struct {
	Command string `json:"command"` // Show command path (e.g., "show subscriber detail")
	Key     string `json:"key"`     // Unique enricher key within command (kebab-case)
}

// EnrichShowInput is the input for ze-plugin-callback:enrich-show.
type EnrichShowInput struct {
	Command string         `json:"command"` // Show command path
	Key     string         `json:"key"`     // Enricher key
	Mode    string         `json:"mode"`    // "detail" or "brief"
	Base    map[string]any `json:"base"`    // Base data map to enrich
}

// EnrichShowOutput is the output for ze-plugin-callback:enrich-show.
type EnrichShowOutput struct {
	Data map[string]any `json:"data,omitempty"` // Enrichment data to merge into base
}

// DoctorCheckDecl declares a doctor readiness check the plugin provides.
// Declared during Stage 1 registration; invoked at runtime via callback.
type DoctorCheckDecl struct {
	Name         string           `json:"name"`                   // Check name (kebab-case, e.g. "rpki-cache-reachable")
	Phase        DoctorCheckPhase `json:"phase"`                  // When to run: pre-config, missing-config, post-config
	Order        int              `json:"order,omitempty"`        // Ordering within phase (0-9999)
	Dependencies []string         `json:"dependencies,omitempty"` // Other check names that must run first
	Platforms    []string         `json:"platforms,omitempty"`    // Platform filter (empty = "any")
	Codes        []string         `json:"codes"`                  // Diagnostic codes (must have "doctor-" prefix)
}

// DoctorCheckPhase determines when a doctor check runs relative to config loading.
type DoctorCheckPhase string

const (
	DoctorPhasePreConfig     DoctorCheckPhase = "pre-config"
	DoctorPhaseMissingConfig DoctorCheckPhase = "missing-config"
	DoctorPhasePostConfig    DoctorCheckPhase = "post-config"
)

// Valid reports whether the phase is a known doctor check phase.
func (phase DoctorCheckPhase) Valid() bool {
	switch phase {
	case DoctorPhasePreConfig, DoctorPhaseMissingConfig, DoctorPhasePostConfig:
		return true
	default:
		return false
	}
}

// DoctorCheckInput is the input for ze-plugin-callback:doctor-check.
type DoctorCheckInput struct {
	Name string `json:"name"` // Which declared check to run
}

// DoctorCheckDiagnostic is a single diagnostic result from a plugin doctor check.
type DoctorCheckDiagnostic struct {
	Code     string `json:"code"`     // Diagnostic code (e.g. "doctor-rpki-cache-unreachable")
	Severity string `json:"severity"` // "error" or "warning"
	Message  string `json:"message"`  // Human-readable description
}

// DoctorCheckOutput is the output for ze-plugin-callback:doctor-check.
type DoctorCheckOutput struct {
	Diagnostics []DoctorCheckDiagnostic `json:"diagnostics"`
}

// FamilyDecl declares an address family the plugin handles.
//
// AFI and SAFI are RFC 4760 wire-format numbers required to register the
// family in the engine's nlri registry. The Name is "afi/safi" canonical form
// (e.g., "ipv4/unicast"); the engine derives afiStr/safiStr by splitting on "/".
type FamilyDecl struct {
	Name string `json:"name"` // e.g., "ipv4/unicast"
	Mode string `json:"mode"` // "encode", "decode", or "both"
	AFI  uint16 `json:"afi"`  // RFC 4760 AFI number (e.g., 1 for IPv4)
	SAFI uint8  `json:"safi"` // RFC 4760 SAFI number (e.g., 1 for unicast)
}

// CommandDecl declares a command the plugin provides.
type CommandDecl struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Args            []string `json:"args,omitempty"`
	Completable     bool     `json:"completable,omitempty"`
	Hidden          bool     `json:"hidden,omitempty"`
	DeprecatedNames []string `json:"deprecated-names,omitempty"`
}

// SchemaDecl declares the YANG schema the plugin provides.
type SchemaDecl struct {
	Module    string   `json:"module"`
	Namespace string   `json:"namespace,omitempty"`
	YANGText  string   `json:"yang-text,omitempty"`
	Handlers  []string `json:"handlers,omitempty"`
}

// FilterDecl declares a named route filter the plugin offers.
type FilterDecl struct {
	Name       string          `json:"name"`                 // Filter name (config references as <plugin>:<name>)
	Direction  FilterDirection `json:"direction"`            // import / export / both
	Attributes []string        `json:"attributes,omitempty"` // Attribute names to receive
	NLRI       *bool           `json:"nlri,omitempty"`       // Include NLRI list (default true)
	Raw        bool            `json:"raw,omitempty"`        // Include raw wire bytes
	OnError    OnErrorPolicy   `json:"on-error,omitempty"`   // reject (fail-closed) or accept (fail-open)
	Overrides  []string        `json:"overrides,omitempty"`  // Default filters this filter replaces
}

// FilterUpdateInput is the input for ze-plugin-callback:filter-update (runtime).
type FilterUpdateInput struct {
	Filter    string `json:"filter"`    // Filter name to invoke
	Direction string `json:"direction"` // "import" or "export"
	Peer      string `json:"peer"`      // Peer IP address
	PeerAS    uint32 `json:"peer-as"`   // Peer ASN
	Update    string `json:"update"`    // Text-format attributes and NLRI
	// Raw is the raw BGP UPDATE body (RFC 4271 Section 4.3, without the 19-byte
	// header) delivered when the filter declared raw=true. A []byte field so
	// encoding/json base64-encodes it on the wire (newline-safe, ~33% expansion)
	// instead of the former hex string (2x) with hand-rolled encode/decode -- see
	// InjectWireRouteInput.UpdateBody for the same idiom.
	Raw []byte `json:"raw,omitempty"`
}

// FilterUpdateOutput is the output for ze-plugin-callback:filter-update.
type FilterUpdateOutput struct {
	Action FilterAction `json:"action"`           // Typed decision; wire form is "accept"/"reject"/"modify"
	Update string       `json:"update,omitempty"` // Delta-only modified attributes (only for action=modify)
	// Raw is a full raw UPDATE-body replacement (only for action=modify with
	// raw). []byte so encoding/json base64-encodes it on the wire (see the input
	// field above).
	Raw []byte `json:"raw,omitempty"`

	// Teardown requests the engine terminate the BGP session after the import
	// filter chain, sending a NOTIFICATION with the given code/subcode. Honored
	// only for import (received) UPDATEs; ignored on export. The route itself is
	// dropped (treated as reject). NotifyCode/NotifySubcode default to
	// Cease / Connection Rejected (RFC 4486) when zero.
	Teardown      bool  `json:"teardown,omitempty"`
	NotifyCode    uint8 `json:"notify-code,omitempty"`
	NotifySubcode uint8 `json:"notify-subcode,omitempty"`
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
	Code     uint8       `json:"code"`
	Encoding CapEncoding `json:"encoding,omitempty"` // hex / b64 / text
	Payload  string      `json:"payload,omitempty"`
	Peers    []string    `json:"peers,omitempty"`
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
	JSON json.RawMessage `json:"json"`
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
	Status string          `json:"status"` // "done" or "error"
	Data   json.RawMessage `json:"data,omitempty"`
}

// UpdateRouteInput is the input for ze-plugin-engine:update-route.
type UpdateRouteInput struct {
	PeerSelector string         `json:"peer-selector,omitempty"`
	Command      string         `json:"command"`
	Meta         map[string]any `json:"meta,omitempty"` // Optional route metadata; plumbed to CommandContext.Meta.
}

// UpdateRouteOutput is the output for ze-plugin-engine:update-route.
type UpdateRouteOutput struct {
	Announced uint32 `json:"announced"`
	Withdrawn uint32 `json:"withdrawn"`
}

// ForwardCachedInput is the input for ze-plugin-engine:forward-cached.
// Bypasses the text-command tokenise path used by update-route.
// Destinations are peer IP addresses (strings); the engine parses them once
// at the reactor boundary. Plugin name is taken from the caller process.
type ForwardCachedInput struct {
	IDs          []uint64 `json:"ids"`
	Destinations []string `json:"destinations"`
}

// ReleaseCachedInput is the input for ze-plugin-engine:release-cached.
// Acks the listed IDs for the calling plugin without forwarding to peers.
type ReleaseCachedInput struct {
	IDs []uint64 `json:"ids"`
}

// StoredRoute is one route a plugin holds as raw wire bytes and asks the engine
// to relay on its behalf (adj-rib-in peer-up replay).
//
// It is deliberately NOT a cache id: forward-cached relays an UPDATE the engine
// still holds in recentUpdates, whereas a stored route outlives that cache (the
// consumer-ack valve drops entries within minutes) and is the only copy left when
// a peer establishes long after the UPDATE arrived.
//
// SourcePeer is the peer the route was LEARNED from. The engine needs it to pick
// the same egress transform a live forward would use -- AS_PATH prepend decisions,
// the RFC 9234 role/OTC step and export policy all key off the source. Dropping it
// (as the older "update hex ... add" replay command did) is what let the replay and
// forward rails diverge.
//
// Hex fields are the stored wire bytes: AttrHex is the whole path-attribute
// section as received (it INCLUDES MP_REACH/MP_UNREACH for MP families -- see
// the A-1 note in plan/spec-fixit-bgp-egress-rail-divergence.md), NextHopHex is
// the next hop in wire form, NLRIHex is this route's own NLRI bytes.
type StoredRoute struct {
	SourcePeer string `json:"source-peer"`
	Family     string `json:"family"`
	AttrHex    string `json:"attr-hex"`
	NextHopHex string `json:"next-hop-hex"`
	NLRIHex    string `json:"nlri-hex"`
}

// RelayStoredRouteInput is the input for ze-plugin-engine:relay-stored-route.
// One call carries a whole peer-up replay: every stored route destined for a
// single newly-established peer, so a replay costs one RPC rather than one per
// route. Destination is that peer's IP address string; the engine resolves it at
// the reactor boundary exactly as forward-cached does.
type RelayStoredRouteInput struct {
	Destination string        `json:"destination"`
	Routes      []StoredRoute `json:"routes"`
}

// RouteInstallEntry is one route a forked route-installing plugin (OSPF, IS-IS,
// ...) inserts into the engine's process-wide Loc-RIB. It carries the fields of
// locrib.Path in wire-portable form.
//
// Protocol is the redistevents protocol NAME (e.g. "ospf", "isis"), NOT the
// numeric ProtocolID: ProtocolIDs are allocated per-process by registration
// order, so the engine re-resolves the name to its own ID. AFI/SAFI are the
// numeric family identifiers (stable across processes). Prefix/NextHop/
// BackupNextHop are netip string forms; empty NextHop means "directly connected".
type RouteInstallEntry struct {
	Protocol           string   `json:"protocol"`
	AFI                uint16   `json:"afi"`
	SAFI               uint8    `json:"safi"`
	Prefix             string   `json:"prefix"`
	Instance           uint32   `json:"instance"`
	NextHop            string   `json:"next-hop,omitempty"`
	AdminDistance      uint8    `json:"admin-distance"`
	Metric             uint32   `json:"metric"`
	Labels             []uint32 `json:"labels,omitempty"`
	IsEBGP             bool     `json:"is-ebgp,omitempty"`
	BackupNextHop      string   `json:"backup-next-hop,omitempty"`
	BackupRepairLabels []uint32 `json:"backup-repair-labels,omitempty"`
}

// RouteInstallInput is the input for ze-plugin-engine:route-install. Routes are
// applied as a batch in one call so a whole SPF delta needs a single round-trip.
type RouteInstallInput struct {
	Routes []RouteInstallEntry `json:"routes"`
}

// RouteInstallOutput is the output for ze-plugin-engine:route-install.
type RouteInstallOutput struct {
	Installed uint32 `json:"installed"` // routes applied to the engine Loc-RIB
}

// RouteRemoveEntry identifies one route to withdraw from the engine Loc-RIB by
// its (Protocol, AFI/SAFI, Prefix, Instance) identity. Protocol is the name
// (see RouteInstallEntry).
type RouteRemoveEntry struct {
	Protocol string `json:"protocol"`
	AFI      uint16 `json:"afi"`
	SAFI     uint8  `json:"safi"`
	Prefix   string `json:"prefix"`
	Instance uint32 `json:"instance"`
}

// RouteRemoveInput is the input for ze-plugin-engine:route-remove.
type RouteRemoveInput struct {
	Routes []RouteRemoveEntry `json:"routes"`
}

// RouteRemoveOutput is the output for ze-plugin-engine:route-remove.
type RouteRemoveOutput struct {
	Removed uint32 `json:"removed"` // routes withdrawn from the engine Loc-RIB
}

// InjectWireRouteInput is the JSON-codec fallback input for
// ze-plugin-engine:inject-wire-route. The typed DirectBridge slot
// (InjectWireRouteHandler) is the hot path for in-process plugins; this shape
// gives forked/external plugins (with no typed slot) a defined socket path
// instead of an ad-hoc "bridge not available" error. UpdateBody is the BGP
// UPDATE payload (RFC 4271 Section 4.3, without the 19-byte header); it wire-
// encodes as base64 in JSON.
type InjectWireRouteInput struct {
	Protocol   string `json:"protocol"`
	PeerKey    string `json:"peer-key"`
	UpdateBody []byte `json:"update-body"`
}

// BatchValidateInput is the JSON-codec fallback input for
// ze-plugin-engine:batch-validate. The typed DirectBridge slot
// (BatchValidateHandler) is the hot path for in-process plugins; this shape
// gives forked/external plugins a defined socket path instead of a hand-rolled
// stride-6 string encoding through dispatch-command-args.
type BatchValidateInput struct {
	Decisions []ValidationDecision `json:"decisions"`
}

// DispatchCommandInput is the input for ze-plugin-engine:dispatch-command.
// Plugins use this to invoke commands through the engine's command dispatcher,
// enabling inter-plugin communication via the standard routing mechanism.
type DispatchCommandInput struct {
	Command string `json:"command"`
}

// DispatchCommandArgsInput is the input for ze-plugin-engine:dispatch-command-args.
// Plugins use this to invoke an exact registered command with pre-tokenized
// arguments, avoiding the command-string tokenizer for internal runtime data.
type DispatchCommandArgsInput struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Peer    string   `json:"peer,omitempty"`
}

// DispatchCommandOutput is the serialized cross-process wire projection of the
// unified in-process command-result envelope (internal/component/plugin.Response)
// for ze-plugin-engine:dispatch-command. It preserves the full {status, data,
// error} shape but carries Data as json.RawMessage rather than a typed payload:
// the receiving process decodes the result without the concrete Go type, so the
// raw-JSON field is mandatory. This is deliberately NOT merged into
// plugin.Response -- it is a distinct transport layer at the process boundary,
// sharing the "done"/"error" status vocabulary. Unlike update-route (which
// extracts only route counters), it preserves the complete response.
type DispatchCommandOutput struct {
	Status string          `json:"status"`         // "done" or "error" (see plugin.StatusDone/StatusError)
	Data   json.RawMessage `json:"data,omitempty"` // raw JSON response data (single-decode)
	Error  string          `json:"error,omitempty"`
}

// EmitEventInput is the input for ze-plugin-engine:emit-event.
// Plugins use this to push events into the engine's delivery pipeline,
// enabling plugin-to-plugin event communication (e.g., RPKI validation events).
type EmitEventInput struct {
	Namespace   string `json:"namespace"`    // Event namespace (e.g., "bgp")
	EventType   string `json:"event-type"`   // Event type (e.g., "rpki")
	Direction   string `json:"direction"`    // Direction for subscriber matching (e.g., "received")
	PeerAddress string `json:"peer-address"` // Peer address for subscriber matching
	Event       string `json:"event"`        // Full JSON event string
}

// EmitEventOutput is the output for ze-plugin-engine:emit-event.
type EmitEventOutput struct {
	Delivered int `json:"delivered"` // Number of subscribers that received the event
}

// SubscribeEventsInput is the input for ze-plugin-engine:subscribe-events.
type SubscribeEventsInput struct {
	Events   []string `json:"events,omitempty"`
	Peers    []string `json:"peers,omitempty"`
	Format   string   `json:"format,omitempty"`
	Encoding string   `json:"encoding,omitempty"` // "json" (default) or "text"
	// Namespace names the event namespace these subscriptions belong to.
	// Empty (the default and the wire-compatible value for every pre-existing
	// caller) resolves to the default namespace registered by the owning
	// protocol component ("bgp" today). A non-empty value lets a plugin
	// subscribe to a non-bgp namespace (e.g. "vpn-ipsec") at startup.
	Namespace string `json:"namespace,omitempty"`
	// Envelope, when true, asks the engine to wrap each delivered event in an
	// EventEnvelope ({namespace, event, payload}) so a plugin subscribed to
	// several event types can discriminate which one arrived without parsing
	// payload-specific fields. Empty/false preserves the bare-payload delivery
	// that every pre-existing consumer relies on.
	Envelope bool `json:"envelope,omitempty"`
}

// EventEnvelope is the JSON shape delivered to a subscriber that opted into
// enveloped delivery (SubscribeEventsInput.Envelope). It wraps the bare event
// payload JSON with its (namespace, event) identity so a multi-subscription
// plugin can tell which event arrived -- impossible for two events that share
// a payload type (e.g. vpn-ipsec sa-up vs sa-down). The envelope rides INSIDE
// the delivered event string, so it is transparent to both the deliver-event
// and deliver-batch wire paths (both carry the event as a JSON string).
type EventEnvelope struct {
	Namespace string          `json:"namespace"`
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
}

// ParseEventEnvelope unmarshals an enveloped delivered event string produced
// when the subscription opted into SubscribeEventsInput.Envelope. Plugins that
// did not opt in receive the bare payload and must not call this.
func ParseEventEnvelope(event string) (EventEnvelope, error) {
	var env EventEnvelope
	err := json.Unmarshal([]byte(event), &env)
	return env, err
}

// ReadyInput is the input for ze-plugin-engine:ready (Stage 5).
// The Subscribe field allows plugins to register event subscriptions atomically
// with startup completion, avoiding the race between SignalAPIReady and a
// separate subscribe-events RPC that would arrive after routes are already sent.
type ReadyInput struct {
	Subscribe *SubscribeEventsInput `json:"subscribe,omitempty"`
	Transport string                `json:"transport,omitempty"` // "bridge" for internal plugins; pipe closed after ack
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

// ConfigOperationType identifies one atomic config operation. Wire values are
// kebab-case so external plugin payloads stay stable and readable.
type ConfigOperationType string

const (
	OperationAddInterface       ConfigOperationType = "add-interface"
	OperationRemoveInterface    ConfigOperationType = "remove-interface"
	OperationAddAddress         ConfigOperationType = "add-address"
	OperationRemoveAddress      ConfigOperationType = "remove-address"
	OperationSetProperty        ConfigOperationType = "set-property"
	OperationAddBridgeMember    ConfigOperationType = "add-bridge-member"
	OperationRemoveBridgeMember ConfigOperationType = "remove-bridge-member"
	OperationAddPeer            ConfigOperationType = "add-peer"
	OperationRemovePeer         ConfigOperationType = "remove-peer"
	OperationModifyPeer         ConfigOperationType = "modify-peer"
	OperationAddListener        ConfigOperationType = "add-listener"
	OperationRemoveListener     ConfigOperationType = "remove-listener"
	OperationAddStaticRoute     ConfigOperationType = "add-static-route"
	OperationRemoveStaticRoute  ConfigOperationType = "remove-static-route"
	OperationSetAdminDistance   ConfigOperationType = "set-admin-distance"
	OperationSetSysctl          ConfigOperationType = "set-sysctl"
	OperationStartDHCP          ConfigOperationType = "start-dhcp"
	OperationStopDHCP           ConfigOperationType = "stop-dhcp"
	OperationAddTunnel          ConfigOperationType = "add-tunnel"
	OperationRemoveTunnel       ConfigOperationType = "remove-tunnel"
)

// ResourceKind identifies the resource an operation targets. It is deliberately
// coarse: component-owned decomposers keep detailed semantics in Params while
// the generic solver uses resource keys for ordering.
type ResourceKind string

const (
	ResourceInterface    ResourceKind = "interface"
	ResourceAddress      ResourceKind = "address"
	ResourcePeer         ResourceKind = "peer"
	ResourceListener     ResourceKind = "listener"
	ResourceBridgeMember ResourceKind = "bridge-member"
	ResourceStaticRoute  ResourceKind = "static-route"
	ResourceSysctl       ResourceKind = "sysctl"
	ResourceDHCP         ResourceKind = "dhcp"
	ResourceTunnel       ResourceKind = "tunnel"
)

// ConfigOperationDecl declares operation callback support during Stage 1.
type ConfigOperationDecl struct {
	Root       string                `json:"root"`
	Decompose  bool                  `json:"decompose,omitempty"`
	Operations []ConfigOperationType `json:"operations,omitempty"`
}

// ResourceRef is the solver-visible target for an operation. All fields are
// values so the payload is safe across internal and external plugin boundaries.
type ResourceRef struct {
	Kind      ResourceKind `json:"kind"`
	Name      string       `json:"name,omitempty"`
	Interface string       `json:"interface,omitempty"`
	Address   string       `json:"address,omitempty"`
	Peer      string       `json:"peer,omitempty"`
	Port      uint16       `json:"port,omitempty"`
	Prefix    string       `json:"prefix,omitempty"`
	NextHop   string       `json:"next-hop,omitempty"`
}

// ConfigOperationParams carries operation-specific values. Component-owned
// decomposers interpret only the fields relevant to their operations.
type ConfigOperationParams struct {
	Name      string          `json:"name,omitempty"`
	Interface string          `json:"interface,omitempty"`
	CIDR      string          `json:"cidr,omitempty"`
	Address   string          `json:"address,omitempty"`
	Peer      string          `json:"peer,omitempty"`
	Port      uint16          `json:"port,omitempty"`
	Prefix    string          `json:"prefix,omitempty"`
	NextHop   string          `json:"next-hop,omitempty"`
	Metric    int             `json:"metric,omitempty"`
	Property  string          `json:"property,omitempty"`
	Value     string          `json:"value,omitempty"`
	OldValue  string          `json:"old-value,omitempty"`
	AllowDual bool            `json:"allow-dual,omitempty"`
	Spec      json.RawMessage `json:"spec,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	OldConfig json.RawMessage `json:"old-config,omitempty"`
	Changed   json.RawMessage `json:"changed,omitempty"`
}

// ConfigOperation is one atomic operation in an ordering-sensitive config transaction.
type ConfigOperation struct {
	ID     string                `json:"id"`
	Root   string                `json:"root"`
	Owner  string                `json:"owner"`
	Type   ConfigOperationType   `json:"type"`
	Target ResourceRef           `json:"target"`
	Params ConfigOperationParams `json:"params,omitzero"`
}

// ConfigOperationDecomposeInput is the input for
// ze-plugin-callback:config-operation-decompose.
type ConfigOperationDecomposeInput struct {
	TransactionID string            `json:"transaction-id"`
	Root          string            `json:"root"`
	Active        ConfigSection     `json:"active"`
	Candidate     ConfigSection     `json:"candidate"`
	Diff          ConfigDiffSection `json:"diff"`
}

// ConfigOperationDecomposeOutput is the output for config-operation-decompose.
type ConfigOperationDecomposeOutput struct {
	Status     string            `json:"status"`
	Error      string            `json:"error,omitempty"`
	Operations []ConfigOperation `json:"operations,omitempty"`
}

// ConfigOperationVerifyInput is the input for config-operation-verify.
type ConfigOperationVerifyInput struct {
	TransactionID string          `json:"transaction-id"`
	Operation     ConfigOperation `json:"operation"`
}

// ConfigOperationVerifyOutput is the output for config-operation-verify.
type ConfigOperationVerifyOutput struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ConfigOperationApplyInput is the input for config-operation-apply.
type ConfigOperationApplyInput struct {
	TransactionID string          `json:"transaction-id"`
	Operation     ConfigOperation `json:"operation"`
}

// ConfigOperationReadiness reports side-effect readiness produced by an operation.
type ConfigOperationReadiness struct {
	Namespace string `json:"namespace"`
	EventType string `json:"event-type"`
	Resource  string `json:"resource,omitempty"`
}

// ConfigOperationApplyOutput is the output for config-operation-apply.
type ConfigOperationApplyOutput struct {
	Status    string                     `json:"status"`
	Error     string                     `json:"error,omitempty"`
	Readiness []ConfigOperationReadiness `json:"readiness,omitempty"`
}

// ConfigOperationRollbackInput is the input for config-operation-rollback.
type ConfigOperationRollbackInput struct {
	TransactionID string            `json:"transaction-id"`
	Operations    []ConfigOperation `json:"operations,omitempty"`
}

// ConfigOperationRollbackOutput is the output for config-operation-rollback.
type ConfigOperationRollbackOutput struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ConfigOperationCommitInput is the input for config-operation-commit.
type ConfigOperationCommitInput struct {
	TransactionID string `json:"transaction-id"`
}

// ConfigOperationCommitOutput is the output for config-operation-commit.
type ConfigOperationCommitOutput struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
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
