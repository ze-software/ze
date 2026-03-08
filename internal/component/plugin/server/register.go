// Design: docs/architecture/api/process-protocol.md — RPC registration
// Detail: system.go — system and daemon command handlers
// Detail: session.go — session protocol handlers
// Detail: plugin_rpc.go — plugin introspection handlers
//
// RPC registration uses init()-based self-registration. Each handler file
// registers its own RPCs via init() + RegisterRPCs(). External packages
// (e.g., handler, editor) call RegisterRPCs() from their own init().

package server
