// Design: docs/architecture/api/process-protocol.md — re-exported RPC type aliases
// Overview: sdk.go — plugin SDK core

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
