// Design: docs/architecture/core-design.md -- Per-OS default backend for traffic control

//go:build !linux

package traffic

// defaultBackendName is empty on non-Linux: no traffic backend is available
// by default. The walker's empty-backend guard fires to tell the operator
// they must configure one explicitly.
const defaultBackendName = ""
