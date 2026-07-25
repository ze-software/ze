// Design: plan/learned/921-mpls-rsvp-te.md -- MPLS forwarding-entry programming
// Related: fibkernel.go -- run() subscribes handleMPLSEntry to (mpls-fib, entry)
// Related: richroute.go -- push reuses the rich-route IP+label path
//
// fib-kernel is the single owner of kernel forwarding state. It receives MPLS
// label-switching entries from label-distribution sources (RSVP-TE, LDP) on the
// (mpls-fib, entry) topic and programs them: push reuses the rich-route path (an
// IP route with an imposed label stack), swap/pop use AF_MPLS routes keyed by
// the incoming label. The AF_MPLS programming lives in mplsentry_linux.go; on
// non-Linux the backend does not implement mplsBackend, so asMPLSBackend returns
// nil and swap/pop entries are skipped (no kernel MPLS dataplane there).
package fibkernel

import (
	"net/netip"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

// mplsBackend programs AF_MPLS swap/pop entries (in-label keyed). Implemented by
// the netlink backend on Linux; stubbed elsewhere.
type mplsBackend interface {
	// addMPLSSwap installs an AF_MPLS route: packets arriving with inLabel are
	// forwarded to nextHop with outLabels imposed (a single-element stack is a
	// swap; an empty stack is a pop / disposition).
	addMPLSSwap(inLabel uint32, outLabels []uint32, nextHop netip.Addr) error
	// delMPLSSwap removes the AF_MPLS route for inLabel.
	delMPLSSwap(inLabel uint32) error
}

// asMPLSBackend returns the mplsBackend if the active backend supports it.
func (f *fibKernel) asMPLSBackend() mplsBackend {
	if mb, ok := f.backend.(mplsBackend); ok {
		return mb
	}
	return nil
}

// mplsCountLocked returns the total MPLS forwarding entries (push + swap/pop)
// for the gauge. Caller holds f.mu.
func (f *fibKernel) mplsCountLocked() int {
	return len(f.mplsInstalled) + len(f.mplsSwaps)
}

// handleMPLSEntry programs a batch of MPLS forwarding entries from a label
// distribution source.
func (f *fibKernel) handleMPLSEntry(batch *mplsfibevents.EntryBatch) {
	if batch == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	rb := f.asRichBackend()
	mb := f.asMPLSBackend()
	for i := range batch.Entries {
		e := &batch.Entries[i]
		switch e.Action {
		case mplsfibevents.ActionAdd:
			f.addMPLSEntryLocked(e, rb, mb)
		case mplsfibevents.ActionRemove:
			f.delMPLSEntryLocked(e, rb, mb)
		case mplsfibevents.ActionUnspecified:
			logger().Warn("fib-kernel: mpls entry with unspecified action", "op", e.Op)
		}
	}
}

func (f *fibKernel) addMPLSEntryLocked(e *mplsfibevents.Entry, rb richRouteBackend, mb mplsBackend) {
	switch e.Op {
	case mplsfibevents.OpPush:
		if err := validateMPLSLabels(e.OutLabels); err != nil {
			logger().Error("fib-kernel: mpls push validation failed", "fec", e.FEC, "error", err)
			f.recordMPLSAddErrorLocked()
			return
		}
		if rb == nil || !e.FEC.IsValid() {
			logger().Warn("fib-kernel: cannot program mpls push (no rich backend or invalid FEC)", "fec", e.FEC)
			return
		}
		// Use Replace only to relabel a push this owner already installed; for a
		// first install use Add. The MPLS push path bypasses sysrib's best-path
		// arbitration and shares the FIB with other writers (sysrib/BGP, static,
		// connected). RouteReplace keys on prefix, not protocol, so an unconditional
		// replace would clobber a foreign route for the same prefix; Add fails EEXIST
		// instead, leaving the other route intact. A genuine relabel of ze's own push
		// (mplsInstalled already set) is safe to Replace so the new label takes effect.
		key := e.FEC.String()
		rr := RichRoute{Prefix: e.FEC, NextHop: e.NextHop, Labels: e.OutLabels}
		var perr error
		if f.mplsInstalled[key] {
			perr = rb.replaceRichRoute(rr)
		} else {
			perr = rb.addRichRoute(rr)
		}
		if perr != nil {
			logger().Error("fib-kernel: mpls push install failed", "fec", e.FEC, "error", perr)
			f.recordMPLSAddErrorLocked()
			return
		}
		f.mplsInstalled[key] = true
	case mplsfibevents.OpSwap, mplsfibevents.OpPop:
		if e.InLabel > maxMPLSLabel {
			logger().Error("fib-kernel: mpls in-label exceeds 20-bit maximum", "in-label", e.InLabel)
			f.recordMPLSAddErrorLocked()
			return
		}
		if e.Op == mplsfibevents.OpSwap {
			if err := validateMPLSLabels(e.OutLabels); err != nil {
				logger().Error("fib-kernel: mpls swap validation failed", "in-label", e.InLabel, "error", err)
				f.recordMPLSAddErrorLocked()
				return
			}
		}
		if mb == nil {
			logger().Warn("fib-kernel: cannot program mpls swap/pop (no AF_MPLS backend)", "in-label", e.InLabel)
			return
		}
		if err := mb.addMPLSSwap(e.InLabel, e.OutLabels, e.NextHop); err != nil {
			logger().Error("fib-kernel: mpls swap/pop install failed", "in-label", e.InLabel, "error", err)
			f.recordMPLSAddErrorLocked()
			return
		}
		f.mplsSwaps[e.InLabel] = true
	case mplsfibevents.OpUnspecified:
		logger().Warn("fib-kernel: mpls entry with unspecified op", "in-label", e.InLabel)
		return
	}
	if m := fibMetricsPtr.Load(); m != nil {
		m.mplsInstalls.Inc()
		m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
	}
}

func (f *fibKernel) delMPLSEntryLocked(e *mplsfibevents.Entry, rb richRouteBackend, mb mplsBackend) {
	switch e.Op {
	case mplsfibevents.OpPush:
		if rb != nil && e.FEC.IsValid() {
			if err := rb.delRichRoute(e.FEC, 0); err != nil {
				logger().Warn("fib-kernel: mpls push remove failed", "fec", e.FEC, "error", err)
			}
		}
		delete(f.mplsInstalled, e.FEC.String())
	case mplsfibevents.OpSwap, mplsfibevents.OpPop:
		if mb != nil {
			if err := mb.delMPLSSwap(e.InLabel); err != nil {
				logger().Warn("fib-kernel: mpls swap/pop remove failed", "in-label", e.InLabel, "error", err)
			}
		}
		delete(f.mplsSwaps, e.InLabel)
	case mplsfibevents.OpUnspecified:
		return
	}
	if m := fibMetricsPtr.Load(); m != nil {
		m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
	}
}

// recordMPLSAddErrorLocked bumps the backend-error counter for a failed MPLS
// install (the only error path here is add; removes are best-effort).
func (f *fibKernel) recordMPLSAddErrorLocked() {
	if m := fibMetricsPtr.Load(); m != nil {
		m.errors.With("add").Inc()
	}
}
