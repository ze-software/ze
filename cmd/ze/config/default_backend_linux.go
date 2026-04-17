// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: cmd_validate.go -- runValidation consults ifaceDefaultBackend
//
// Per-platform default for the interface `backend` leaf. MUST match
// internal/component/iface/default_linux.go (defaultBackendName) so the
// offline CLI and the daemon diagnose the same rejection on a config
// that omits the backend leaf.

//go:build linux

package config

func ifaceDefaultBackend() string { return "netlink" }
