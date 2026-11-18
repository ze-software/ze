// Design: docs/architecture/plugin/rib-storage-design.md — the route clock and the age a received row carries
// Overview: rib_pipeline.go — the show pipeline whose received rows carry the age
// Related: storage/routetime.go — RouteTime, the reading a stored route keeps
// Related: rib_structured.go — the received path that reads the clock once per message
package rib

import (
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/clock"
)

// rowKeyAge names the member a received row says how long ago its route last
// changed with: a whole number of seconds.
const rowKeyAge = "age"

// routeClock reads the clock a RIBManager stamps its routes with and answers
// each reading as a storage.RouteTime, relative to the epoch it took when it
// was built. The epoch is this process's start of the RIB, so readings do not
// survive a restart: every route is relearned then and is new.
//
// The zero routeClock holds no clock and every reading of it is
// storage.RouteTimeUnrecorded. A RIBManager built without newRIBManager
// therefore stores routes with no age, never with an age read from a clock
// nobody chose.
//
// Safe for concurrent use: it is never written after construction.
type routeClock struct {
	source clock.Clock
	epoch  time.Time
}

// newRouteClock starts a route clock on source, with the epoch at this call.
func newRouteClock(source clock.Clock) routeClock {
	return routeClock{source: source, epoch: source.Now()}
}

// now answers the current reading. A handler that stores a received message
// calls it once for the message and hands the reading to every route of it,
// so the clock is not read per route.
func (c routeClock) now() storage.RouteTime {
	if c.source == nil {
		return storage.RouteTimeUnrecorded
	}
	return storage.RouteTimeSince(c.epoch, c.source.Now())
}

// markReadAt stamps the received rows a source just buffered with one reading
// of the route clock. It is called after the walk that read their entries, so
// the reading is never earlier than a time one of those entries holds.
func (r *RIBManager) markReadAt(items []RouteItem) {
	readAt := r.routeClock.now()
	for i := range items {
		items[i].readAt = readAt
	}
}

// writeRouteAge puts on a received row the seconds between changedAt, the
// reading the stored route keeps, and readAt, the reading taken when the row's
// source read the route. A route with no age writes nothing: a reader then
// knows the RIB could not say, which a zero would hide behind "changed this
// second".
func writeRouteAge(row map[string]any, changedAt, readAt storage.RouteTime) {
	age, known := changedAt.AgeAt(readAt)
	if !known {
		return
	}
	row[rowKeyAge] = int64(age / time.Second)
}
