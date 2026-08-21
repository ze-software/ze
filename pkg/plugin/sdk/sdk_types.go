// Design: docs/architecture/api/process-protocol.md — re-exported RPC type aliases
// Overview: sdk.go — plugin SDK core
//
// These type aliases re-export canonical types from pkg/plugin/rpc and
// pkg/plugin so that external plugin authors only need to import the sdk
// package. This decouples the public SDK surface from the internal RPC wire
// types — rpc can be restructured without breaking plugin code.
//
// For the canonical type definitions and field documentation, see:
//   pkg/plugin/rpc/types.go
//   pkg/plugin/records.go

package sdk

import (
	"github.com/ze-software/ze/pkg/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Row is one row of a record answer, as the bytes it appends to a buffer the
// caller owns rather than the bytes it allocates.
type Row = plugin.Row

// Record is one line of a record answer: a row the command produced, or a row
// it rejected while the walk continued.
type Record = plugin.Record

// Records is the walk a command handler answers with instead of the collection,
// so one row at a time reaches the wire.
type Records = plugin.Records

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

// ConfigOperationDecl declares operation callback support during Stage 1.
type ConfigOperationDecl = rpc.ConfigOperationDecl

// ConfigOperationType identifies one atomic config operation.
type ConfigOperationType = rpc.ConfigOperationType

// ConfigOperation is one atomic operation in an ordering-sensitive config transaction.
type ConfigOperation = rpc.ConfigOperation

// ConfigOperationParams carries operation-specific values.
type ConfigOperationParams = rpc.ConfigOperationParams

// ResourceKind identifies the resource an operation targets.
type ResourceKind = rpc.ResourceKind

// ResourceRef is the solver-visible target for an operation.
type ResourceRef = rpc.ResourceRef

// Config operation type values.
const (
	OperationAddInterface       = rpc.OperationAddInterface
	OperationRemoveInterface    = rpc.OperationRemoveInterface
	OperationAddAddress         = rpc.OperationAddAddress
	OperationRemoveAddress      = rpc.OperationRemoveAddress
	OperationSetProperty        = rpc.OperationSetProperty
	OperationAddBridgeMember    = rpc.OperationAddBridgeMember
	OperationRemoveBridgeMember = rpc.OperationRemoveBridgeMember
	OperationAddPeer            = rpc.OperationAddPeer
	OperationRemovePeer         = rpc.OperationRemovePeer
	OperationModifyPeer         = rpc.OperationModifyPeer
	OperationAddListener        = rpc.OperationAddListener
	OperationRemoveListener     = rpc.OperationRemoveListener
	OperationAddStaticRoute     = rpc.OperationAddStaticRoute
	OperationRemoveStaticRoute  = rpc.OperationRemoveStaticRoute
	OperationSetAdminDistance   = rpc.OperationSetAdminDistance
	OperationSetSysctl          = rpc.OperationSetSysctl
	OperationStartDHCP          = rpc.OperationStartDHCP
	OperationStopDHCP           = rpc.OperationStopDHCP
	OperationAddTunnel          = rpc.OperationAddTunnel
	OperationRemoveTunnel       = rpc.OperationRemoveTunnel
)

// Resource kind values.
const (
	ResourceInterface    = rpc.ResourceInterface
	ResourceAddress      = rpc.ResourceAddress
	ResourcePeer         = rpc.ResourcePeer
	ResourceListener     = rpc.ResourceListener
	ResourceBridgeMember = rpc.ResourceBridgeMember
	ResourceStaticRoute  = rpc.ResourceStaticRoute
	ResourceSysctl       = rpc.ResourceSysctl
	ResourceDHCP         = rpc.ResourceDHCP
	ResourceTunnel       = rpc.ResourceTunnel
)

// ConfigOperationDecomposeInput is the input for config-operation-decompose.
type ConfigOperationDecomposeInput = rpc.ConfigOperationDecomposeInput

// ConfigOperationDecomposeOutput is the output for config-operation-decompose.
type ConfigOperationDecomposeOutput = rpc.ConfigOperationDecomposeOutput

// ConfigOperationVerifyInput is the input for config-operation-verify.
type ConfigOperationVerifyInput = rpc.ConfigOperationVerifyInput

// ConfigOperationVerifyOutput is the output for config-operation-verify.
type ConfigOperationVerifyOutput = rpc.ConfigOperationVerifyOutput

// ConfigOperationApplyInput is the input for config-operation-apply.
type ConfigOperationApplyInput = rpc.ConfigOperationApplyInput

// ConfigOperationApplyOutput is the output for config-operation-apply.
type ConfigOperationApplyOutput = rpc.ConfigOperationApplyOutput

// ConfigOperationRollbackInput is the input for config-operation-rollback.
type ConfigOperationRollbackInput = rpc.ConfigOperationRollbackInput

// ConfigOperationRollbackOutput is the output for config-operation-rollback.
type ConfigOperationRollbackOutput = rpc.ConfigOperationRollbackOutput

// ConfigOperationCommitInput is the input for config-operation-commit.
type ConfigOperationCommitInput = rpc.ConfigOperationCommitInput

// ConfigOperationCommitOutput is the output for config-operation-commit.
type ConfigOperationCommitOutput = rpc.ConfigOperationCommitOutput

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

// PipeDecl declares a pipe alias for one of the plugin's own commands.
type PipeDecl = rpc.PipeDecl

// EnricherDecl declares a show enricher the plugin provides.
type EnricherDecl = rpc.EnricherDecl

// EnrichShowInput is the input for enrich-show (runtime callback).
type EnrichShowInput = rpc.EnrichShowInput

// EnrichShowOutput is the output for enrich-show (runtime callback).
type EnrichShowOutput = rpc.EnrichShowOutput

// DoctorCheckDecl declares a doctor readiness check the plugin provides.
type DoctorCheckDecl = rpc.DoctorCheckDecl

// DoctorCheckPhase determines when a doctor check runs relative to config loading.
type DoctorCheckPhase = rpc.DoctorCheckPhase

// DoctorCheckPhase values: wire form is "pre-config", "missing-config", "post-config".
const (
	DoctorPhasePreConfig     = rpc.DoctorPhasePreConfig
	DoctorPhaseMissingConfig = rpc.DoctorPhaseMissingConfig
	DoctorPhasePostConfig    = rpc.DoctorPhasePostConfig
)

// DoctorCheckDiagnostic is a single diagnostic result from a plugin doctor check.
type DoctorCheckDiagnostic = rpc.DoctorCheckDiagnostic

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
