// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: cmd_validate.go -- runValidation consults ifaceDefaultBackend
//
// On non-Linux platforms the interface plugin has no default backend
// (internal/component/iface/default_other.go). Returning "" triggers the
// walker's empty-backend rejection, matching what the daemon reports on
// startup when the user has not set `interface.backend`.

//go:build !linux

package config

func ifaceDefaultBackend() string { return "" }
