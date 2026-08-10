// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS web neighbor + database views.
// Related: snapshot_views.go -- the generic read-only live-view implementation wrapped here.
//
// The IS-IS engine runs as a managed plugin subprocess, so the web layer reaches it the
// same way the CLI does: through the CommandDispatcher, which forwards `show isis
// neighbor` / `show isis database` to the engine. This adapter just configures the
// generic snapshot views with the IS-IS command names, titles, and stream paths.

package web //nolint:dupl // parallel per-protocol web adapter mirroring handler_ospf.go; the shared logic is in snapshot_views.go, this file is only IS-IS naming + the public API used by main_servers.go.

import (
	"net/http"
	"time"
)

// isisRefreshInterval is how often the IS-IS SSE stream re-fetches and pushes a snapshot.
const isisRefreshInterval = 3 * time.Second

// ISISHandlers holds the dependencies for the IS-IS web UI handlers. Dispatch is the same
// CommandDispatcher the CLI/admin surfaces use.
type ISISHandlers struct {
	Renderer *Renderer
	Dispatch CommandDispatcher
}

var (
	isisNeighborView = viewSpec{command: "show isis neighbor", title: "IS-IS Neighbors", streamPath: "/isis/neighbors/stream", eventName: "neighbors"}
	isisDatabaseView = viewSpec{command: "show isis database", title: "IS-IS Database", streamPath: "/isis/database/stream", eventName: "database"}
)

// views builds the generic snapshot handler configured for IS-IS.
func (h *ISISHandlers) views() *snapshotHandlers {
	return &snapshotHandlers{
		dispatch:       h.Dispatch,
		errNoDispatch:  errISISDispatchUnavailable,
		unavailableMsg: "isis engine unavailable",
		jsonWarnMsg:    "isis view json write",
		dataID:         "isis-data",
		refresh:        isisRefreshInterval,
	}
}

// handleISISNeighbors serves GET /isis (and /isis/neighbors): the IS-IS adjacency view.
func (h *ISISHandlers) handleISISNeighbors() http.HandlerFunc {
	return h.views().handleView(isisNeighborView)
}

// handleISISDatabase serves GET /isis/database: the IS-IS link-state database view.
func (h *ISISHandlers) handleISISDatabase() http.HandlerFunc {
	return h.views().handleView(isisDatabaseView)
}

// handleISISNeighborsSSE serves GET /isis/neighbors/stream.
func (h *ISISHandlers) handleISISNeighborsSSE() http.HandlerFunc {
	return h.views().sse(isisNeighborView)
}

// handleISISDatabaseSSE serves GET /isis/database/stream.
func (h *ISISHandlers) handleISISDatabaseSSE() http.HandlerFunc {
	return h.views().sse(isisDatabaseView)
}
