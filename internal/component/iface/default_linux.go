// Design: docs/features/interfaces.md -- Per-OS backend default

//go:build linux

package iface

// defaultBackendName is the backend used when the config does not specify one.
const defaultBackendName = "netlink"
