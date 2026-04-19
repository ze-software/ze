// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: cmd_validate.go -- runValidation consults firewallDefaultBackend
//
// Per-platform default for the firewall `backend` leaf. MUST match
// internal/component/firewall/default_linux.go (defaultBackendName) so
// the offline CLI and the daemon diagnose the same rejection on a
// config that omits the backend leaf.

//go:build linux

package config

func firewallDefaultBackend() string { return "nft" }
