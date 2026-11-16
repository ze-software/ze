// Design: docs/architecture/core-design.md -- BGP policy final disposition.
// Related: policy.go -- configured and absent operator policy snapshots.

package filterapi

// ExportKind identifies the engine authority carrying one route to a peer.
// The engine MUST derive it from authenticated ownership and provenance,
// never from plugin-supplied metadata or the absence of source-peer facts.
// It is a per-route fact, so compiled policies MUST NOT retain it.
type ExportKind uint8

const (
	// ExportUnknown grants no export authority, including the zero value.
	ExportUnknown ExportKind = iota
	// ExportOrdinaryLearned is an elected received route under ordinary output.
	ExportOrdinaryLearned
	// ExportExplicit is an admitted local origin or a route an eligible,
	// explicitly attached route-server/reflector owner carries.
	ExportExplicit
)

// Known reports whether kind names a supported authority class. It does not
// authenticate provenance; the caller derives that before supplying the kind.
// Unknown and out-of-range values MUST fail closed at the export boundary.
func (kind ExportKind) Known() bool {
	return kind == ExportOrdinaryLearned || kind == ExportExplicit
}

// UnconfiguredExportAccepts answers the absent export policy's end disposition.
// External sessions retain RFC 8212's rejection for every kind. On an internal
// session, only explicit local/role output survives; ordinary learned output
// requires an operator policy. Missing provenance never becomes explicit.
func UnconfiguredExportAccepts(external bool, kind ExportKind) bool {
	if !UnconfiguredAccepts(external) {
		return false
	}
	return kind == ExportExplicit
}
