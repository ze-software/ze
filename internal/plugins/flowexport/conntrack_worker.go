// Design: plan/learned/819-flow-export-2-flow-records.md -- conntrack flow-record lifecycle
// Related: conntrack/reader_linux.go -- netlink conntrack dump
// Related: conntrack/delta.go -- per-flow delta tracking between dumps

package flowexport

import (
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport/conntrack"
)

const (
	// maxConntrackDestroyErrors is how many back-to-back destroy-event read
	// errors are tolerated before the listener backs off, so a broken socket
	// cannot spin the CPU.
	maxConntrackDestroyErrors = 10
	// conntrackDestroyBackoff is the pause inserted after the error threshold.
	conntrackDestroyBackoff = 100 * time.Millisecond
	// conntrackTombstoneGrace is how long a cleanly torn-down flow's delta
	// baseline is retained before SweepTombstones reclaims it. It MUST exceed
	// the longest dump read->process window (so a dump whose snapshot predates
	// the teardown still computes a zero delta instead of re-exporting the full
	// cumulative) and stay well under 2x active-timeout (or it buys nothing over
	// Cleanup). 5s is comfortably above any realistic dump-processing latency.
	// See DeltaTracker.SweepTombstones for the full reasoning.
	conntrackTombstoneGrace = 5 * time.Second
)

// conntrackWorker periodically dumps the kernel conntrack table, computes
// per-flow byte/packet deltas since the last dump, and dispatches per-flow
// records to the exporter's NetFlow v9 / IPFIX collectors.
//
// Platform specifics live in the conntrack package: on non-Linux hosts
// NewReader returns an error and the worker stays idle (logged once).
//
// NOTE (spec 2 C1, resolved): both IPv4 and IPv6 flows are exported. The
// NetFlow v9 / IPFIX encoders carry separate IPv4 and IPv6 per-flow templates
// (template IDs 257 and 258); each address family ships in its own datagram.
type conntrackWorker struct {
	exp     *exporter
	cfg     ConntrackConfig
	reader  *conntrack.Reader
	tracker *conntrack.DeltaTracker

	// destroy listens for NFNLGRP_CONNTRACK_DESTROY events and exports each
	// torn-down flow's residual counters immediately, so short-lived flows
	// that begin and end between two periodic dumps are not lost. Optional:
	// if the listener fails to open the worker still runs in dump-only mode.
	destroy     *conntrack.DestroyListener
	destroyDone chan struct{}

	// refreshCh requests an out-of-band dump between ticker intervals. The DDoS
	// characterizer needs the recent-flow ring to reflect an in-progress attack,
	// but the periodic dump cadence is the operator's active-timeout (up to an
	// hour), so at attack-confirm the ring can hold pre-attack state. Refresh
	// signals here; the run loop performs the dump on its own goroutine so it
	// never races the ticker dump. Buffered depth 1 coalesces a burst of
	// AttackDetected events into a single pending dump.
	refreshCh chan struct{}

	stopped atomic.Bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

func newConntrackWorker(exp *exporter, cfg ConntrackConfig) *conntrackWorker {
	return &conntrackWorker{
		exp:         exp,
		cfg:         cfg,
		tracker:     conntrack.NewDeltaTracker(),
		destroyDone: make(chan struct{}),
		refreshCh:   make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

// Refresh requests an immediate out-of-band conntrack dump so the recent-flow
// ring reflects the current table (e.g. an in-progress attack) without waiting
// for the next active-timeout tick. Non-blocking and coalescing: if a refresh is
// already pending it is a no-op, so a burst of triggers cannot queue a backlog of
// full-table dumps. Safe on a nil worker (conntrack disabled) and after Stop (the
// signal is simply never consumed).
func (w *conntrackWorker) Refresh() {
	if w == nil {
		return
	}
	select {
	case w.refreshCh <- struct{}{}:
	default: // a dump is already pending; coalesce
	}
}

// Start opens the conntrack reader and launches the dump worker. A reader
// failure leaves the worker idle rather than aborting the exporter.
func (w *conntrackWorker) Start() {
	log := loggerPtr.Load()
	// On the gokrazy appliance (ze_appliance), nothing but ze runs at init, so ze
	// must load nf_conntrack, register a tracking hook, and enable accounting
	// itself or the ctnetlink dump reads an empty table. A no-op off the appliance
	// (see conntrack_setup_other.go), where the operator/firewall owns it.
	ensureConntrackTracking(log)
	reader, err := conntrack.NewReader()
	if err != nil {
		log.Warn("flow-export: conntrack reader unavailable; flow records idle", "error", err)
		close(w.doneCh)
		close(w.destroyDone) // never started; keep Stop from blocking on it
		return
	}
	w.reader = reader
	go w.run()

	// Destroy-event listener is best-effort: it needs CAP_NET_ADMIN and the
	// nf_conntrack_netlink module. If it cannot start, the periodic dump still
	// provides flow records (it just cannot capture flows shorter than one
	// dump interval).
	dl, err := conntrack.NewDestroyListener()
	if err != nil {
		log.Info("flow-export: conntrack destroy listener unavailable; dump-only mode", "error", err)
		close(w.destroyDone)
		return
	}
	w.destroy = dl
	go w.runDestroy()
	log.Info("flow-export: conntrack destroy listener active")
}

// runDestroy exports each torn-down flow's residual counters as the events
// arrive. Closing the listener in Stop unblocks Receive; the stopped flag turns
// the resulting error into a clean exit rather than a counted failure.
func (w *conntrackWorker) runDestroy() {
	defer close(w.destroyDone)
	log := loggerPtr.Load()

	consecutiveErrs := 0
	for {
		entries, err := w.destroy.Read()
		if err != nil {
			if w.stopped.Load() {
				return
			}
			consecutiveErrs++
			if consecutiveErrs == maxConntrackDestroyErrors {
				log.Warn("flow-export: repeated conntrack destroy read errors, backing off", "error", err)
			}
			if consecutiveErrs >= maxConntrackDestroyErrors {
				time.Sleep(conntrackDestroyBackoff)
			}
			continue
		}
		consecutiveErrs = 0

		now := time.Now()
		flows := make([]ConntrackFlow, 0, len(entries))
		for i := range entries {
			e := &entries[i]
			// The destroy event carries the flow's final cumulative counters.
			// ComputeDeltaFinal returns the residual since the last dump and
			// tombstones the entry in one locked step. It does NOT hard-delete:
			// a periodic dump whose snapshot predates this teardown may still
			// process the flow, and a delete would make that dump a first
			// observation that re-exports the full total. The tombstone keeps
			// the baseline (so the stale dump computes zero) while letting
			// SweepTombstones reclaim it within conntrackTombstoneGrace instead
			// of waiting for Cleanup at 2x active-timeout. See the SCALING RISK
			// and GENERATION ORDERING notes in conntrack/delta.go.
			d := w.tracker.ComputeDeltaFinal(*e, now)
			if d.Bytes == 0 && d.Packets == 0 {
				continue // fully accounted for by the last dump
			}
			flows = append(flows, w.toFlow(d))
		}
		if len(flows) > 0 {
			w.exp.exportFlows(flows)
		}
		// Reclaim flows tombstoned more than the grace window ago. Sweeping here
		// (rather than on a timer) is self-pacing: a busy box delivers destroy
		// events frequently, so reclaim keeps up with churn; an idle box has
		// few tombstones to begin with.
		w.tracker.SweepTombstones(conntrackTombstoneGrace, now)
		setFlowsActive(float64(w.tracker.Len()))
	}
}

func (w *conntrackWorker) run() {
	defer close(w.doneCh)

	interval := time.Duration(w.cfg.ActiveTimeout) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.dumpAndExport()
		case <-w.refreshCh:
			// Out-of-band dump requested (DDoS characterization). Performed on
			// this goroutine so it serializes with the ticker dump -- the delta
			// tracker makes an extra dump safe (each byte counted once; the
			// export just lands earlier than the scheduled tick).
			w.dumpAndExport()
		case <-w.stopCh:
			return
		}
	}
}

func (w *conntrackWorker) dumpAndExport() {
	log := loggerPtr.Load()
	// Advance the dump generation before snapshotting the kernel table so every
	// entry of this snapshot shares it. The generation lets the delta tracker
	// reject a stale in-flight dump entry that a concurrent destroy event has
	// already superseded (see DeltaTracker.computeLocked).
	gen := w.tracker.BeginDump()
	entries, err := w.reader.Dump()
	if err != nil {
		log.Warn("flow-export: conntrack dump failed", "error", err)
		return
	}

	flows := make([]ConntrackFlow, 0, len(entries))
	for i := range entries {
		e := &entries[i]
		// Both IPv4 and IPv6 flows are exported; the encoders split by
		// address family into separate templates (see note above).
		d := w.tracker.ComputeDelta(*e, gen)
		if d.Bytes == 0 && d.Packets == 0 {
			continue // no new traffic since last dump
		}
		flows = append(flows, w.toFlow(d))
	}

	w.exp.exportFlows(flows)
	setFlowsActive(float64(w.tracker.Len()))
	setRecentRingDrops(float64(w.exp.recentDrops()))

	// GC delta state for flows not seen for two dump intervals.
	w.tracker.Cleanup(2 * time.Duration(w.cfg.ActiveTimeout) * time.Second)
}

// toFlow converts a delta-adjusted conntrack entry into an exportable
// ConntrackFlow. Shared by the periodic dump and the destroy listener so both
// paths timestamp and shape records identically. conntrack may not record a
// start time; fall back to last-seen so the exported flowStart is sane rather
// than a wrapped uint64 from the Go zero time.
func (w *conntrackWorker) toFlow(d conntrack.FlowEntry) ConntrackFlow {
	lastMs := safeUnixMillis(d.LastSeen)
	firstMs := safeUnixMillis(d.StartTime)
	if firstMs == 0 {
		firstMs = lastMs
	}
	return ConntrackFlow{
		SrcAddr:  d.SrcAddr,
		DstAddr:  d.DstAddr,
		SrcPort:  d.SrcPort,
		DstPort:  d.DstPort,
		Protocol: d.Protocol,
		Bytes:    d.Bytes,
		Packets:  d.Packets,
		FirstMs:  firstMs,
		LastMs:   lastMs,
		TCPState: d.TCPState,
	}
}

// safeUnixMillis returns t as Unix milliseconds, or 0 for the zero time or any
// pre-epoch time. conntrack may not record a start time; without this guard
// uint64(time.Time{}.UnixMilli()) would wrap to a huge value and corrupt the
// exported flowStart timestamp.
func safeUnixMillis(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	ms := t.UnixMilli()
	if ms < 0 {
		return 0
	}
	return uint64(ms)
}

// Stop halts the worker, releases the conntrack reader, and shuts down the
// destroy listener. The stopped flag is set before closing the listener so the
// runDestroy goroutine treats the resulting Receive error as a clean exit.
func (w *conntrackWorker) Stop() {
	w.stopped.Store(true)
	close(w.stopCh)
	if w.destroy != nil {
		_ = w.destroy.Close()
	}
	<-w.doneCh
	<-w.destroyDone
	if w.reader != nil {
		_ = w.reader.Close()
	}
}
