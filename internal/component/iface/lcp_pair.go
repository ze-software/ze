// Design: docs/research/vpp-deployment-reference.md -- Linux Control Plane pair readback
// Related: backend.go -- optional capabilities of the selected interface backend

package iface

import "context"

// LCPPair is an actual data-plane/control-plane pairing read from a backend.
// DataplaneIndex and ControlPlaneIndex belong to the userspace data plane;
// LinuxIndex belongs to ControlDevice in Namespace. Empty Namespace denotes
// the data-plane process's namespace, which callers MUST verify they can reach.
// This is a readback DTO, not a resource owner or an instruction to create a pair.
type LCPPair struct {
	DataplaneIndex    uint32
	ControlPlaneIndex uint32
	LinuxIndex        int
	ControlDevice     string
	Namespace         string
}

// LCPPairReader is an optional capability of an interface backend that creates
// Linux Control Plane pairs. GetLCPPair reads the actual pairing for the
// resolved data-plane device name; it MUST NOT invent a Linux name or index
// from the configured name. Missing pairs and incomplete reads return errors.
// The caller MUST first resolve the device through the same backend's link
// read; an unresolved native name is a lookup failure, not an absent pair.
// Calls require a nonnil context and are safe for concurrent use.
type LCPPairReader interface {
	GetLCPPair(context.Context, string) (LCPPair, error)
}
