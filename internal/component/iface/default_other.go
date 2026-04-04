// Design: docs/features/interfaces.md -- Per-OS backend default

//go:build !linux

package iface

// defaultBackendName is empty on non-Linux: no backend is available by default.
const defaultBackendName = ""
