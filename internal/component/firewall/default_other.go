// Design: docs/architecture/core-design.md -- default firewall backend on non-Linux

//go:build !linux

package firewall

// defaultBackendName is empty on non-Linux: no firewall backend exists
// so any firewall section in config rejects at verify under
// exact-or-reject (rather than silently being a no-op).
const defaultBackendName = ""
