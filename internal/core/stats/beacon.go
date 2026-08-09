// Design: docs/architecture/traffic/traffic-analysis-layers.md -- coarse beaconing (interval regularity)

package stats

// beaconFloorSeconds is the smallest beacon period observable at the 1s
// observation tick (Nyquist): a mean interval below this is treated as no signal.
const beaconFloorSeconds = 2.0

// minBeaconIntervals is the fewest inter-arrival gaps needed before regularity is
// meaningful.
const minBeaconIntervals = 3

// IntervalRegularity scores how clock-like a series of event inter-arrival
// intervals (seconds, in order) is, on [0,1]: 1.0 is perfectly periodic, 0.0 is
// irregular or below the observable floor. It is 1 minus the coefficient of
// variation (StdDev/Mean) of the intervals, clamped to [0,1]. Fewer than
// minBeaconIntervals intervals, a non-positive mean, or a mean below
// beaconFloorSeconds yields 0 (insufficient evidence or below the 1s-tick floor).
func IntervalRegularity(intervals []float64) float64 {
	if len(intervals) < minBeaconIntervals {
		return 0
	}
	m := Mean(intervals)
	if m < beaconFloorSeconds {
		return 0
	}
	r := 1 - StdDev(intervals)/m
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}
