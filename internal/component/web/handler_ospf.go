// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- OSPF web neighbor + database views.
// Related: snapshot_views.go -- the generic read-only live-view implementation wrapped here.
//
// The OSPF engine runs as a managed plugin subprocess, so the web layer reaches it the
// same way the CLI does: through the CommandDispatcher, which forwards `show ospf
// neighbor` / `show ospf database` to the engine. This adapter just configures the
// generic snapshot views with the OSPF command names, titles, and stream paths.

package web //nolint:dupl // parallel per-protocol web adapter mirroring handler_isis.go; the shared logic is in snapshot_views.go, this file is only OSPF naming + the public API used by main_servers.go.

import (
	"net/http"
	"time"
)

// ospfRefreshInterval is how often the OSPF SSE stream re-fetches and pushes a snapshot.
const ospfRefreshInterval = 3 * time.Second

// OSPFHandlers holds the dependencies for the OSPF web UI handlers. Dispatch is the same
// CommandDispatcher the CLI/admin surfaces use.
type OSPFHandlers struct {
	Renderer *Renderer
	Dispatch CommandDispatcher
}

var (
	ospfNeighborView = viewSpec{command: "show ospf neighbor", title: "OSPF Neighbors", streamPath: "/ospf/neighbors/stream", eventName: "neighbors"}
	ospfDatabaseView = viewSpec{command: "show ospf database", title: "OSPF Database", streamPath: "/ospf/database/stream", eventName: "database"}
	// spec-ospf-ext-14: read-only opaque (IPv4) and OSPFv3 (IPv6) database views. The inject
	// path is NEVER surfaced on the web (R-6) -- only read-only `show` commands are wired here.
	ospfOpaqueDatabaseView = viewSpec{command: "show ospf database opaque-area", title: "OSPF Opaque Database", streamPath: "/ospf/database/opaque/stream", eventName: "opaque"}
	ospfV3DatabaseView     = viewSpec{command: "show ospf ipv6 database", title: "OSPFv3 Database", streamPath: "/ospfv3/database/stream", eventName: "ospfv3-database"}
)

// views builds the generic snapshot handler configured for OSPF.
func (h *OSPFHandlers) views() *snapshotHandlers {
	return &snapshotHandlers{
		dispatch:       h.Dispatch,
		errNoDispatch:  errOSPFDispatchUnavailable,
		unavailableMsg: "ospf engine unavailable",
		jsonWarnMsg:    "ospf view json write",
		dataID:         "ospf-data",
		refresh:        ospfRefreshInterval,
	}
}

// HandleOSPFNeighbors serves GET /ospf (and /ospf/neighbors): the OSPF adjacency view.
func (h *OSPFHandlers) HandleOSPFNeighbors() http.HandlerFunc {
	return h.views().handleView(ospfNeighborView)
}

// HandleOSPFDatabase serves GET /ospf/database: the OSPF link-state database view.
func (h *OSPFHandlers) HandleOSPFDatabase() http.HandlerFunc {
	return h.views().handleView(ospfDatabaseView)
}

// HandleOSPFNeighborsSSE serves GET /ospf/neighbors/stream.
func (h *OSPFHandlers) HandleOSPFNeighborsSSE() http.HandlerFunc {
	return h.views().sse(ospfNeighborView)
}

// HandleOSPFDatabaseSSE serves GET /ospf/database/stream.
func (h *OSPFHandlers) HandleOSPFDatabaseSSE() http.HandlerFunc {
	return h.views().sse(ospfDatabaseView)
}

// HandleOSPFOpaqueDatabase serves GET /ospf/database/opaque: the read-only IPv4 opaque LSDB.
func (h *OSPFHandlers) HandleOSPFOpaqueDatabase() http.HandlerFunc {
	return h.views().handleView(ospfOpaqueDatabaseView)
}

// HandleOSPFOpaqueDatabaseSSE serves GET /ospf/database/opaque/stream.
func (h *OSPFHandlers) HandleOSPFOpaqueDatabaseSSE() http.HandlerFunc {
	return h.views().sse(ospfOpaqueDatabaseView)
}

// HandleOSPFv3Database serves GET /ospfv3/database: the read-only OSPFv3 (IPv6) LSDB.
func (h *OSPFHandlers) HandleOSPFv3Database() http.HandlerFunc {
	return h.views().handleView(ospfV3DatabaseView)
}

// HandleOSPFv3DatabaseSSE serves GET /ospfv3/database/stream.
func (h *OSPFHandlers) HandleOSPFv3DatabaseSSE() http.HandlerFunc {
	return h.views().sse(ospfV3DatabaseView)
}
