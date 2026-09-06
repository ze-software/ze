// RFC: rfc/short/rfc7854.md
// Design: docs/guide/bmp.md -- BMP sender, periodic Statistics Report
//
// Overview: bmp.go -- the plugin that owns the counters reported here
// Related: sender_config.go -- the statistics-timeout leaf this file acts on
// Related: sender.go -- writeStatisticsReport, the encoder these reports use
// Related: bmp_events.go -- duplicateUpdate, which produces the counter
//
// The periodic BMP Statistics Report: the ticker the statistics-timeout leaf
// starts, the counters ze measures, and the per-peer messages one tick writes to
// every collector.

package bmp

import (
	"encoding/binary"
	"time"
)

// Statistics Report stat types (RFC 7854 Section 4.8). Only the types ze
// MEASURES are named. Section 4.8 defines fourteen, and a type ze does not
// measure has no honest value to carry: reporting a zero for it would be a
// count the collector reads as "none happened" where ze counted nothing
// (ai/rules/principles.md).
//
// RFC 7854 Section 4.8: "Stat Type = 13: (32-bit Counter) Number of duplicate
// update messages received." duplicateUpdate is what measures it.
const statTypeDuplicateUpdates uint16 = 13

// statCounterSize is the width of a 32-bit Counter stat value in bytes. RFC
// 7854 Section 4.8: "Although the current specification only specifies 4-byte
// counters and 8-byte gauges as "Stat Data", this does not preclude future
// versions from incorporating more complex TLV-type "Stat Data"." Ze sends the
// 4-byte form, which is the one Stat Type 13 is defined as.
const statCounterSize = 4

// makeStatCounter builds a StatEntry carrying a 32-bit Counter value.
func makeStatCounter(typ uint16, value uint32) StatEntry {
	v := make([]byte, statCounterSize)
	binary.BigEndian.PutUint32(v, value)
	return StatEntry{Type: typ, Value: v}
}

// setStatisticsTimeout installs the periodic Statistics Report interval that
// the `statistics-timeout` leaf asks for, and is the only place a statistics
// ticker starts or stops. Zero stops the ticker and sends no further report,
// which RFC 7854 Section 4.8 permits: "SR messages are optional."
//
// Exactly one ticker is live at a time. A reload that moves the interval closes
// the running ticker's stop channel and starts a ticker of its own, so a
// collector never receives two report streams at two intervals. A reload that
// leaves the interval where it is starts nothing, which keeps the phase of the
// reports ze is already sending.
func (bp *BMPPlugin) setStatisticsTimeout(interval time.Duration) {
	bp.mu.Lock()
	if bp.statisticsInterval == interval {
		bp.mu.Unlock()
		return
	}

	previous := bp.statisticsStop
	bp.statisticsInterval = interval
	bp.statisticsStop = nil
	if interval > 0 {
		stop := make(chan struct{})
		bp.statisticsStop = stop
		bp.sessions.Go(func() { bp.statisticsLoop(interval, stop) })
	}
	bp.mu.Unlock()

	// Outside the lock: the ticker goroutine takes the read lock on every tick,
	// and it must be able to take it while this closure is being handed to it.
	if previous != nil {
		close(previous)
	}
}

// statisticsLoop writes one round of Statistics Reports every interval, until
// the configuration turns the reports off (stop) or the plugin shuts down
// (bp.stopCh).
//
// The loop has no iteration bound, which is the decision rather than an
// omission: a periodic report runs for as long as the operator asks for it, and
// both of its exits are closed channels rather than counters.
func (bp *BMPPlugin) statisticsLoop(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-bp.stopCh:
			return
		case <-stop:
			return
		case <-ticker.C:
			bp.sendStatisticsReports()
		}
	}
}

// sendStatisticsReports writes one Statistics Report per established BGP peer
// to every collector session.
//
// RFC 7854 Section 4.8 leaves the timing to the implementation -- "Transmission
// of SR messages could be timer triggered or event driven ... It is left to the
// implementation to determine transmission timings -- however, configuration
// control should be provided of the timer and/or threshold values" -- and
// `statistics-timeout` is that control.
//
// One message per peer, and no message for a peer ze reports nothing about:
// Section 4.8 ends "SR messages are optional. However, if an SR message is
// transmitted, at least one statistic MUST be carried in it."
//
// The RFC 9069 Loc-RIB emulated peer gets no report. Section 5.6 names the two
// stat types that are relevant to a Loc-RIB, 8 and 10, and both are route
// counts of the Loc-RIB itself, which ze does not hold in the BMP plugin.
func (bp *BMPPlugin) sendStatisticsReports() {
	// The timestamp of a per-peer header describes the event the message
	// carries (RFC 7854 Section 4.2), and the event here is the counter read.
	// One read for the whole round, so every peer's report on one tick carries
	// the instant the round was taken rather than a per-peer wall-clock drift.
	now := uint32(time.Now().Unix()) //nolint:gosec // seconds since the epoch is what RFC 7854 Section 4.2 defines the field as, and it is a uint32 there too

	bp.mu.RLock()
	senders := bp.senders
	reports := make([]statisticsReport, 0, len(bp.peerUps))
	for address, up := range bp.peerUps {
		peer := up.peer
		peer.TimestampSec = now
		peer.TimestampUsec = 0

		// A peer with no counter entry has had no duplicate yet, so zero is a
		// count ze measured rather than one it never took: handleSenderUpdate
		// runs the received-direction detector for every peer while this ticker
		// is running, whatever route-monitoring-policy is in force.
		reports = append(reports, statisticsReport{
			Peer:  peer,
			Stats: []StatEntry{makeStatCounter(statTypeDuplicateUpdates, bp.dedupCount[address])},
		})
	}
	bp.mu.RUnlock()

	for i := range reports {
		for _, ss := range senders {
			if err := ss.writeStatisticsReport(reports[i].Peer, reports[i].Stats); err != nil {
				logger().Debug("bmp: sender statistics report failed", "collector", ss.name, "error", err)
			}
		}
	}
}
