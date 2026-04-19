// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: cmd_validate.go -- runValidation consults firewallDefaultBackend
//
// On non-Linux platforms the firewall plugin has no default backend
// (internal/component/firewall/default_other.go). Returning "" triggers
// the walker's empty-backend rejection, matching what the daemon
// reports on startup when the user has not set `firewall.backend`.

//go:build !linux

package config

func firewallDefaultBackend() string { return "" }
