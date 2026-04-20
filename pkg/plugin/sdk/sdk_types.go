// Design: docs/architecture/api/process-protocol.md — re-exported RPC type aliases
// Overview: sdk.go — plugin SDK core
//
// These type aliases re-export canonical types from pkg/plugin/rpc so that
// external plugin authors only need to import the sdk package. This decouples
// the public SDK surface from the internal RPC wire types — rpc can be
// restructured without breaking plugin code.
//
// For the canonical type definitions and field documentation, see:
//   pkg/plugin/rpc/types.go

package sdk

import "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"

// Registration is the SDK name for the declare-registration input (Stage 1).
type Registration = rpc.DeclareRegistrationInput

// FamilyDecl declares an address family the plugin handles.
type FamilyDecl = rpc.FamilyDecl

// CommandDecl declares a command the plugin provides.
type CommandDecl = rpc.CommandDecl

// SchemaDecl declares the YANG schema the plugin provides.
type SchemaDecl = rpc.SchemaDecl

// DeclareCapabilitiesInput is the input for declare-capabilities (Stage 3).
type DeclareCapabilitiesInput = rpc.DeclareCapabilitiesInput

// CapabilityDecl declares a BGP capability for OPEN injection.
type CapabilityDecl = rpc.CapabilityDecl

// ConfigSection is a config section delivered during Stage 2.
type ConfigSection = rpc.ConfigSection

// RegistryCommand is a command in the shared registry (Stage 4).
type RegistryCommand = rpc.RegistryCommand

// UpdateRouteOutput is the output for update-route (runtime).
type UpdateRouteOutput = rpc.UpdateRouteOutput

// ExecuteCommandOutput is the output for execute-command (runtime).
type ExecuteCommandOutput = rpc.ExecuteCommandOutput

// DispatchCommandOutput is the output for dispatch-command (runtime).
type DispatchCommandOutput = rpc.DispatchCommandOutput

// ConfigDiffSection describes what changed in a single config root (reload).
type ConfigDiffSection = rpc.ConfigDiffSection

// ConfigVerifyOutput is the output for config-verify (reload).
type ConfigVerifyOutput = rpc.ConfigVerifyOutput

// ConfigApplyOutput is the output for config-apply (reload).
type ConfigApplyOutput = rpc.ConfigApplyOutput

// ValidateOpenInput is the input for validate-open (OPEN validation).
type ValidateOpenInput = rpc.ValidateOpenInput

// ValidateOpenOutput is the output for validate-open (OPEN validation).
type ValidateOpenOutput = rpc.ValidateOpenOutput

// ValidateOpenMessage represents one side of the OPEN exchange.
type ValidateOpenMessage = rpc.ValidateOpenMessage

// ValidateOpenCapability is a single capability from an OPEN message.
type ValidateOpenCapability = rpc.ValidateOpenCapability

// DecodeNLRIOutput is the output for decode-nlri (plugin→engine).
type DecodeNLRIOutput = rpc.DecodeNLRIOutput

// EncodeNLRIOutput is the output for encode-nlri (plugin→engine).
type EncodeNLRIOutput = rpc.EncodeNLRIOutput

// DecodeMPReachOutput is the output for decode-mp-reach (plugin→engine).
type DecodeMPReachOutput = rpc.DecodeMPReachOutput

// DecodeMPUnreachOutput is the output for decode-mp-unreach (plugin→engine).
type DecodeMPUnreachOutput = rpc.DecodeMPUnreachOutput

// DecodeUpdateOutput is the output for decode-update (plugin→engine).
type DecodeUpdateOutput = rpc.DecodeUpdateOutput

// ConnectionHandlerDecl declares a listen socket the plugin wants via fd passing.
type ConnectionHandlerDecl = rpc.ConnectionHandlerDecl

// FilterDecl declares a named route filter the plugin offers.
type FilterDecl = rpc.FilterDecl

// FilterUpdateInput is the input for filter-update (runtime callback).
type FilterUpdateInput = rpc.FilterUpdateInput

// FilterUpdateOutput is the output for filter-update (runtime callback).
type FilterUpdateOutput = rpc.FilterUpdateOutput

// FilterAction is the typed wire decision for a filter-update response.
type FilterAction = rpc.FilterAction

// FilterAction values: wire form is "accept", "reject", "modify".
const (
	FilterUnspecified = rpc.FilterUnspecified
	FilterAccept      = rpc.FilterAccept
	FilterReject      = rpc.FilterReject
	FilterModify      = rpc.FilterModify
)

// FilterDirection is the typed wire direction for a FilterDecl.
type FilterDirection = rpc.FilterDirection

// FilterDirection values: wire form is "import", "export", "both".
const (
	FilterDirectionUnspecified = rpc.FilterDirectionUnspecified
	FilterImport               = rpc.FilterImport
	FilterExport               = rpc.FilterExport
	FilterBoth                 = rpc.FilterBoth
)

// OnErrorPolicy is the typed failure policy for a FilterDecl.
type OnErrorPolicy = rpc.OnErrorPolicy

// OnErrorPolicy values: wire form is "reject", "accept".
const (
	OnErrorUnspecified = rpc.OnErrorUnspecified
	OnErrorReject      = rpc.OnErrorReject
	OnErrorAccept      = rpc.OnErrorAccept
)

// CapEncoding is the typed payload encoding for a CapabilityDecl.
type CapEncoding = rpc.CapEncoding

// CapEncoding values: wire form is "hex", "b64", "text".
const (
	CapEncodingUnspecified = rpc.CapEncodingUnspecified
	CapEncodingHex         = rpc.CapEncodingHex
	CapEncodingBase64      = rpc.CapEncodingBase64
	CapEncodingText        = rpc.CapEncodingText
)
