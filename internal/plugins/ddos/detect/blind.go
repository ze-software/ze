// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- the blind-detector diagnostic.
// Related: detector.go -- tickRates and tickInfos report each feed sample to observeFeed
// Related: doctor.go -- checkRateFeed turns a finding into a `ze doctor` warning
// Related: register.go -- subscribe arms the watch and publishes the detector to the doctor callback
//
// The detector's only input is the per-interface receive rate from the
// trafficstat service. The flow sources (traffic-usage, flow-export) only characterize
// an attack once it is detected. A feed that delivers nothing, or delivers
// zeros, fills the baseline with zeros: `(*baseline).Ready` counts samples and
// not their values, and the threshold falls to the absolute floor, so the
// detector reads as warmed and armed and can never trigger. This file tells
// "no attack" from "no traffic".
package detect

import (
	"strconv"
	"time"
)

// The names of the two rate feeds subscribe can wire the detector to. The
// operator reads them in the warning, so they name the feed, not the Go call.
const (
	feedTrafficstat   = "trafficstat interface rate feed"
	feedIfaceCounters = "interface counter poll"
)

// feedWatch records what the rate feed delivered. Every field is guarded by
// detector.mu.
type feedWatch struct {
	// now is the clock. newDetector sets time.Now; a test replaces it.
	now func() time.Time
	// name is the feed subscribe wired the detector to. Empty until arm runs,
	// which is the unarmed state rateFeedFinding refuses to judge.
	name    string
	armedAt time.Time
	// external is true when the plugin runs as its own process. The feed is a
	// same-process subscription, so such a detector only sees its own process's
	// empty feed, and rateFeedFinding reports that cause rather than the empty
	// samples it produces.
	external bool
	// lastSampleAt is when the feed last delivered a sample. Zero until the
	// first sample, and then rateFeedFinding measures silence from armedAt.
	lastSampleAt time.Time
	// zeroRun counts consecutive samples with zero received packets and zero
	// received bytes on every interface. The first sample with traffic resets it.
	zeroRun int
	// ifaces is the number of interfaces the last sample carried.
	ifaces int
	// warned is true from the WARN log at the onset of a zero run to the INFO
	// log when traffic returns, so each edge logs once and a long idle period
	// does not repeat the line every second.
	warned bool
}

// feedState is what rateFeedFinding concluded about the rate feed.
type feedState uint8

const (
	// feedStateUnspecified is the zero value and never a conclusion.
	feedStateUnspecified feedState = iota
	// feedStateUnarmed: the detector was never subscribed to a feed.
	feedStateUnarmed
	// feedStateExternal: the plugin runs as an external process, so the
	// same-process feed subscription can never deliver a sample.
	feedStateExternal
	// feedStateSeeing: traffic arrived inside the last window.
	feedStateSeeing
	// feedStateNoSample: no sample arrived for a whole window.
	feedStateNoSample
	// feedStateZeroTraffic: every sample of a whole window was zero.
	feedStateZeroTraffic
	// feedStateNoInterface: every sample of a whole window was zero, and the
	// last one carried no interface at all.
	feedStateNoInterface
)

// feedFinding is one conclusion about the rate feed, with what the message
// to the operator names.
type feedFinding struct {
	state  feedState
	feed   string
	window time.Duration
}

// arm records which feed subscribe wired the detector to and whether the
// plugin runs in the daemon's process, and starts the silence clock. subscribe
// MUST call it after the subscription is made.
func (d *detector) arm(feed string, internal bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.feed.name = feed
	d.feed.external = !internal
	d.feed.armedAt = d.feed.now()
}

// blindSamples is the window, in one-second feed samples, after which a feed
// with no traffic is reported: baseline-window evaluations of check-interval
// samples each, which is the moment the baseline holds nothing but zeros. A
// genuinely idle box therefore warns only once its baseline is already blind.
// Caller holds d.mu.
func (d *detector) blindSamples() int {
	return d.cfg.BaselineWindow * d.evalTicks
}

// observeFeed records one rate feed sample and logs the edges of a zero run:
// WARN when the run first reaches the window, INFO when traffic returns.
// Caller holds d.mu (tickRates and tickInfos do). Logging under d.mu is safe:
// nothing here reaches the event bus.
func (d *detector) observeFeed(maxPps, maxBps float64, ifaces int) {
	d.feed.lastSampleAt = d.feed.now()
	d.feed.ifaces = ifaces
	if d.feed.external {
		// The samples an external process sees are its own empty copy of the
		// feed, not the box's traffic. warnIfExternal logged the cause at
		// start, and rateFeedFinding reports it, so no zero-run edge is logged.
		return
	}

	traffic := maxPps > 0 || maxBps > 0
	if traffic {
		d.feed.zeroRun = 0
		if d.feed.warned {
			d.feed.warned = false
			logger().Info("ddos-detect: the interface rate feed shows traffic again, so the detector can see an attack",
				"feed", d.feed.name)
		}
		return
	}

	d.feed.zeroRun++
	if d.feed.warned {
		return
	}
	window := d.blindSamples()
	if d.feed.zeroRun < window {
		return
	}
	d.feed.warned = true
	f := feedFinding{state: feedStateZeroTraffic, feed: d.feed.name, window: time.Duration(window) * time.Second}
	if ifaces == 0 {
		f.state = feedStateNoInterface
	}
	logger().Warn(blindMessage(f), "feed", d.feed.name)
}

// rateFeedFinding judges the rate feed over the last window. Safe for
// concurrent use: it takes d.mu, so the doctor callback can call it while the
// feed ticks.
func (d *detector) rateFeedFinding() feedFinding {
	d.mu.Lock()
	defer d.mu.Unlock()

	samples := d.blindSamples()
	f := feedFinding{feed: d.feed.name, window: time.Duration(samples) * time.Second}
	if d.feed.name == "" {
		f.state = feedStateUnarmed
		return f
	}
	if d.feed.external {
		f.state = feedStateExternal
		return f
	}
	last := d.feed.lastSampleAt
	if last.IsZero() {
		// No sample yet: the silence runs from the subscription.
		last = d.feed.armedAt
	}
	if d.feed.now().Sub(last) >= f.window {
		f.state = feedStateNoSample
		return f
	}
	if d.feed.zeroRun < samples {
		f.state = feedStateSeeing
		return f
	}
	if d.feed.ifaces == 0 {
		f.state = feedStateNoInterface
		return f
	}
	f.state = feedStateZeroTraffic
	return f
}

// blindMessage is the operator text for a blind finding: what is wrong, why it
// matters, and the command that checks it. The same text serves the doctor
// diagnostic and the WARN log line.
func blindMessage(f feedFinding) string {
	seconds := strconv.Itoa(int(f.window / time.Second))
	// Only the four blind states carry a message; checkRateFeed and observeFeed send no other.
	//exhaustive:ignore
	switch f.state {
	case feedStateExternal:
		return "ddos-detect is enabled but runs as an external plugin process, so it never receives the " +
			f.feed + " (a same-process subscription) and cannot detect an attack." +
			" Remove the `plugin { external ddos-detect { ... } }` block from the config so ze runs ddos-detect internal"
	case feedStateNoSample:
		return "ddos-detect is enabled but has received no sample from the " + f.feed +
			" for " + seconds + "s, so it cannot detect an attack. Check the feed with: ze show interface rate"
	case feedStateNoInterface:
		return "ddos-detect is enabled but the " + f.feed + " has reported no interface for " + seconds +
			"s (one baseline window), so it watches nothing and cannot detect an attack." +
			" Check the interfaces with: ze show interface"
	case feedStateZeroTraffic:
		return "ddos-detect is enabled but the " + f.feed + " has reported zero received packets and bytes" +
			" on every interface for " + seconds + "s (one baseline window). Its baseline holds only zeros," +
			" so it cannot tell an attack from no traffic. An idle lab or standby router shows this;" +
			" otherwise check the rates with: ze show interface rate"
	}
	return "ddos-detect: no blind-detector message for rate feed state " + strconv.Itoa(int(f.state))
}
