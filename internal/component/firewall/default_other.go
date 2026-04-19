// Design: docs/architecture/core-design.md -- default firewall backend on non-Linux

//go:build !linux

package firewall

// defaultBackendName is empty on non-Linux: no firewall backend exists
// so any firewall section in config rejects at verify under
// exact-or-reject (rather than silently being a no-op).
const defaultBackendName = ""

// DefaultBackendName exposes defaultBackendName for the offline
// `ze config validate` CLI. An empty string signals the gate walker
// to surface a "no backend configured" rejection on non-Linux hosts.
func DefaultBackendName() string { return defaultBackendName }
