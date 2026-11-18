// Design: docs/architecture/plugin/rib-storage-design.md — when a stored route last changed
// Related: routeentry.go -- RouteEntry.ChangedAt holds a RouteTime
// Related: familyrib.go -- the insert paths that write it and the equal branches that keep it

package storage

import (
	"math"
	"time"
)

// RouteTime is one reading of the route clock of the RIB that stores the
// route: the whole seconds elapsed since the epoch that RIB chose when it
// started, plus one. The plus one keeps zero free, so the Go zero value is
// never a reading (RouteTimeUnrecorded).
//
// The epoch is the RIB's own start, never the Unix epoch: an age is a
// difference of two readings, and readings taken from one process's monotonic
// clock do not move when the wall clock is stepped. A reading therefore means
// nothing outside the process that took it and is never written to a wire or a
// store. Four bytes hold 136 years of uptime at this resolution.
type RouteTime uint32

// RouteTimeUnrecorded is the zero RouteTime: no reading was taken. A route
// stored without one has no age, and AgeAt refuses to invent one.
const RouteTimeUnrecorded RouteTime = 0

// routeTimeSecondsMax is the last elapsed second a RouteTime can hold, one
// below the type's range because a reading is the elapsed seconds plus one.
const routeTimeSecondsMax = math.MaxUint32 - 1

// RouteTimeSince answers the reading for now on a clock whose epoch is epoch.
// A now before the epoch, or further from it than a RouteTime holds, has no
// reading and answers RouteTimeUnrecorded.
func RouteTimeSince(epoch, now time.Time) RouteTime {
	elapsed := now.Sub(epoch)
	if elapsed < 0 {
		return RouteTimeUnrecorded
	}
	seconds := int64(elapsed / time.Second)
	if seconds > routeTimeSecondsMax {
		return RouteTimeUnrecorded
	}
	return RouteTime(seconds + 1)
}

// AgeAt answers how long before the reading now the reading t was taken, in
// whole seconds. It answers false when either reading was never taken, or when
// now is earlier than t: none of the three has an age, and zero would read as
// a route that changed this second.
func (t RouteTime) AgeAt(now RouteTime) (time.Duration, bool) {
	if t == RouteTimeUnrecorded {
		return 0, false
	}
	if now == RouteTimeUnrecorded {
		return 0, false
	}
	if now < t {
		return 0, false
	}
	return time.Duration(now-t) * time.Second, true
}
