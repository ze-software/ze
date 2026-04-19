// Design: docs/architecture/core-design.md -- default firewall backend on Linux

//go:build linux

package firewall

// defaultBackendName is the backend chosen when the operator omits the
// `firewall/backend` leaf on Linux. nftables is the only backend that
// implements Apply today; the backend leaf remains mandatory in the
// YANG once more backends exist.
const defaultBackendName = "nft"

// DefaultBackendName exposes defaultBackendName so the offline
// `ze config validate` CLI (cmd/ze/config) can surface the same
// default the daemon uses when computing the backend gate.
func DefaultBackendName() string { return defaultBackendName }
