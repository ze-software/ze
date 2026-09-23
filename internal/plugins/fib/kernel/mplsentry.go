// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- MPLS forwarding-entry programming
// Related: fibkernel.go -- run() subscribes handleMPLSEntry to (mpls-fib, entry)
// Related: richroute.go -- push reuses the rich-route IP+label path
//
// fib-kernel is the single owner of kernel forwarding state. It receives MPLS
// label-switching entries from label-distribution sources (RSVP-TE, LDP) on the
// (mpls-fib, entry) topic and programs them: push reuses the rich-route path (an
// IP route with an imposed label stack), swap/pop use AF_MPLS routes keyed by
// the incoming label. The AF_MPLS programming lives in mplsentry_linux.go.
// Unsupported backends reject entries through the synchronous acknowledgment.
package fibkernel

import (
	"errors"
	"fmt"
	"net/netip"
	"syscall"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

// mplsBackend programs AF_MPLS swap/pop entries (in-label keyed). Implemented by
// the netlink backend on Linux; stubbed elsewhere.
type mplsBackend interface {
	// addMPLSSwap installs an AF_MPLS route: packets arriving with inLabel are
	// forwarded to nextHop with outLabels imposed (a single-element stack is a
	// swap; an empty stack is a pop / disposition). replace is true only when
	// this producer already owns the incoming label.
	addMPLSSwap(inLabel uint32, outLabels []uint32, nextHop netip.Addr, pathMTU uint32, replace bool) error
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
	count := len(f.mplsInstalled) + len(f.mplsSwaps)
	if backend, ok := f.backend.(mplsContextBackend); ok {
		count += backend.mplsContextCount()
	}
	return count
}

// handleMPLSEntry programs a batch of MPLS forwarding entries from a label
// distribution source.
func (f *fibKernel) handleMPLSEntry(batch *mplsfibevents.EntryBatch) {
	if batch == nil {
		return
	}
	f.mu.Lock()
	var result error
	defer func() {
		f.mu.Unlock()
		if batch.Acknowledge != nil {
			batch.Acknowledge(result)
		}
	}()
	if f.stopped {
		result = errors.New("mpls-fib: forwarding owner stopped")
		return
	}

	rb := f.asRichBackend()
	mb := f.asMPLSBackend()
	for i := range batch.Entries {
		e := &batch.Entries[i]
		if e.TableID != 0 {
			if e.Op != mplsfibevents.OpPush {
				result = errors.Join(result, errors.New("mpls-fib: swap/pop require table zero"))
				continue
			}
			if e.TableID&mplsContextMask != mplsContextMark {
				result = errors.Join(result, fmt.Errorf("mpls-fib: private table %#x is outside the bypass namespace", e.TableID))
				continue
			}
		}
		switch e.Action {
		case mplsfibevents.ActionAdd:
			result = errors.Join(result, f.addMPLSEntryLocked(e, rb, mb))
		case mplsfibevents.ActionRemove:
			result = errors.Join(result, f.delMPLSEntryLocked(e, rb, mb))
		default:
			result = errors.Join(result, fmt.Errorf("mpls-fib: unsupported entry action %d", e.Action))
		}
	}
}

func (f *fibKernel) addMPLSEntryLocked(e *mplsfibevents.Entry, rb richRouteBackend, mb mplsBackend) error {
	switch e.Op {
	case mplsfibevents.OpPush:
		if err := validateMPLSLabels(e.OutLabels); err != nil {
			logger().Error("fib-kernel: mpls push validation failed", "fec", e.FEC, "error", err)
			f.recordMPLSAddErrorLocked()
			return err
		}
		if rb == nil || !e.FEC.IsValid() {
			logger().Warn("fib-kernel: cannot program mpls push (no rich backend or invalid FEC)", "fec", e.FEC)
			return fmt.Errorf("mpls-fib: cannot program push: backend available=%t, FEC=%s", rb != nil, e.FEC)
		}
		if err := f.addMPLSPushLocked(e, rb); err != nil {
			logger().Error("fib-kernel: mpls push install failed", "fec", e.FEC, "table", e.TableID, "error", err)
			f.recordMPLSAddErrorLocked()
			return err
		}
	case mplsfibevents.OpSwap, mplsfibevents.OpPop:
		if e.InLabel > maxMPLSLabel {
			logger().Error("fib-kernel: mpls in-label exceeds 20-bit maximum", "in-label", e.InLabel)
			f.recordMPLSAddErrorLocked()
			return fmt.Errorf("mpls-fib: in-label %d exceeds 20 bits", e.InLabel)
		}
		if e.Op == mplsfibevents.OpSwap {
			if err := validateMPLSLabels(e.OutLabels); err != nil {
				logger().Error("fib-kernel: mpls swap validation failed", "in-label", e.InLabel, "error", err)
				f.recordMPLSAddErrorLocked()
				return err
			}
		}
		if mb == nil {
			logger().Warn("fib-kernel: cannot program mpls swap/pop (no AF_MPLS backend)", "in-label", e.InLabel)
			return errors.New("mpls-fib: no AF_MPLS backend")
		}
		owner, installed := f.mplsSwaps[e.InLabel]
		if installed && owner != e.Source {
			return fmt.Errorf("mpls-fib: in-label %d belongs to source %d, not %d: %w",
				e.InLabel, owner, e.Source, syscall.EEXIST)
		}
		if err := mb.addMPLSSwap(e.InLabel, e.OutLabels, e.NextHop, e.PathMTU, installed); err != nil {
			logger().Error("fib-kernel: mpls swap/pop install failed", "in-label", e.InLabel, "error", err)
			f.recordMPLSAddErrorLocked()
			return err
		}
		f.mplsSwaps[e.InLabel] = e.Source
	default:
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	}
	if m := fibMetricsPtr.Load(); m != nil {
		m.mplsInstalls.Inc()
		m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
	}
	return nil
}

func (f *fibKernel) delMPLSEntryLocked(e *mplsfibevents.Entry, rb richRouteBackend, mb mplsBackend) error {
	switch e.Op {
	case mplsfibevents.OpPush:
		if e.TableID != 0 {
			backend, ok := f.backend.(mplsContextBackend)
			if !ok {
				return errors.New("mpls-fib: backend cannot remove private push contexts")
			}
			if !e.FEC.IsValid() {
				return errors.New("mpls-fib: cannot remove push without valid FEC")
			}
			if err := backend.delMPLSContext(e.FEC.Masked(), e.TableID); err != nil {
				return err
			}
			break
		}
		if rb != nil && e.FEC.IsValid() {
			if err := rb.delRichRoute(e.FEC, 0); err != nil {
				logger().Warn("fib-kernel: mpls push remove failed", "fec", e.FEC, "error", err)
				return err
			}
		}
		if rb == nil || !e.FEC.IsValid() {
			return errors.New("mpls-fib: cannot remove push without backend and valid FEC")
		}
		delete(f.mplsInstalled, e.FEC.String())
	case mplsfibevents.OpSwap, mplsfibevents.OpPop:
		owner, installed := f.mplsSwaps[e.InLabel]
		if !installed {
			break
		}
		if owner != e.Source {
			return fmt.Errorf("mpls-fib: in-label %d belongs to source %d, not %d: %w",
				e.InLabel, owner, e.Source, syscall.EEXIST)
		}
		if mb != nil {
			if err := mb.delMPLSSwap(e.InLabel); err != nil {
				logger().Warn("fib-kernel: mpls swap/pop remove failed", "in-label", e.InLabel, "error", err)
				return err
			}
		}
		if mb == nil {
			return errors.New("mpls-fib: cannot remove label without AF_MPLS backend")
		}
		delete(f.mplsSwaps, e.InLabel)
	default:
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	}
	if m := fibMetricsPtr.Load(); m != nil {
		m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
	}
	return nil
}

// recordMPLSAddErrorLocked bumps the backend-error counter for a failed MPLS
// install (the only error path here is add; removes are best-effort).
func (f *fibKernel) recordMPLSAddErrorLocked() {
	if m := fibMetricsPtr.Load(); m != nil {
		m.errors.With("add").Inc()
	}
}
