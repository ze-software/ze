// Design: docs/architecture/core-design.md -- Per-OS default backend for traffic control

//go:build linux

package traffic

// defaultBackendName is the traffic backend used when the config does not
// specify one. Linux ships with the tc (iproute2) backend.
const defaultBackendName = "tc"
