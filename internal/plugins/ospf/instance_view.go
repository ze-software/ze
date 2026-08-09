// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- OSPFv3 AF-aware instance listing.
// RFC: rfc/short/rfc5838.md (Section 2: each address family is a separate OSPFv3 instance
// with its own LSDB, identified by its Instance-ID range).
//
// `show ospf ipv6 instance` enumerates every running OSPFv3 address-family instance with its
// address family (derived from the Instance-ID range), Instance ID, area count, and neighbor
// count (AC-12). It degrades cleanly to a single IPv6-unicast instance when only the base
// address family is configured (A-11); it reads the AF identity multi-AF establishes and
// does NOT implement the demux.

package ospf

// v3InstanceRow identifies one running OSPFv3 address-family instance for the AF-aware
// `show ospf ipv6 instance` view.
type v3InstanceRow struct {
	AddressFamily string `json:"address-family"`
	InstanceID    uint8  `json:"instance-id"`
	RouterID      string `json:"router-id"`
	Areas         int    `json:"areas"`
	Neighbors     int    `json:"neighbors"`
}

// instanceListing lists every running OSPFv3 address-family instance with its AF, Instance
// ID, Router ID, area count, and neighbor count (RFC 5838 Section 2).
func (m *v6EngineSet) instanceListing() []v3InstanceRow {
	afs := m.runningAFs()
	out := make([]v3InstanceRow, 0, len(afs))
	for _, af := range afs {
		e, ok := m.engineFor(af)
		if !ok {
			continue
		}
		e.mu.Lock()
		rid := e.cfg.RouterID.String()
		e.mu.Unlock()
		var instanceID uint8
		if e.dispatch != nil {
			instanceID = e.dispatch.currentInstanceID()
		}
		out = append(out, v3InstanceRow{
			AddressFamily: af.String(),
			InstanceID:    instanceID,
			RouterID:      rid,
			Areas:         e.areaCount(),
			Neighbors:     len(e.neighborSnapshot()),
		})
	}
	return out
}

// areaCount returns the number of areas this engine's LSDB holds LSAs for.
func (e *engine) areaCount() int {
	if e.lsdb == nil {
		return 0
	}
	return len(e.lsdb.Snapshot().Areas)
}
