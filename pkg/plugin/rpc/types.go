// Package rpc defines the canonical wire-format types for the ze plugin RPC protocol.
//
// Both the engine (internal/plugin) and the SDK (pkg/plugin/sdk) import these types
// to ensure a single source of truth for the RPC message structures.
//
// The two-socket architecture uses these types in JSON-RPC messages:
//   - Socket A (plugin → engine): declare-registration, declare-capabilities, ready
//   - Socket B (engine → plugin): configure, share-registry, deliver-event, bye
package rpc

// DeclareRegistrationInput is the input for ze-plugin-engine:declare-registration (Stage 1).
type DeclareRegistrationInput struct {
	Families    []FamilyDecl  `json:"families,omitempty"`
	Commands    []CommandDecl `json:"commands,omitempty"`
	WantsConfig []string      `json:"wants-config,omitempty"`
	Schema      *SchemaDecl   `json:"schema,omitempty"`
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

// EncodeNLRIInput is the input for ze-plugin-callback:encode-nlri.
type EncodeNLRIInput struct {
	Family string   `json:"family"`
	Args   []string `json:"args,omitempty"`
}

// DecodeNLRIInput is the input for ze-plugin-callback:decode-nlri.
type DecodeNLRIInput struct {
	Family string `json:"family"`
	Hex    string `json:"hex"`
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

// SubscribeEventsInput is the input for ze-plugin-engine:subscribe-events.
type SubscribeEventsInput struct {
	Events []string `json:"events,omitempty"`
	Peers  []string `json:"peers,omitempty"`
	Format string   `json:"format,omitempty"`
}

// ByeInput is the input for ze-plugin-callback:bye (shutdown).
type ByeInput struct {
	Reason string `json:"reason,omitempty"`
}
