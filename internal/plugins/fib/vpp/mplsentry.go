// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane
// Related: mpls.go -- the GoVPP label routes and MPLS table 0
// Related: register.go -- the plugin-lifetime (mpls-fib, entry) subscription
//
// fib-vpp is the label writer of the vpp data plane, as fib-kernel is of the
// netlink one. It consumes (mpls-fib, entry) with the same validation and the
// same per-source ownership of incoming labels, and acknowledges every batch it
// consumes, so a producer never advertises a label VPP does not forward.
package fibvpp

import (
	"errors"
	"fmt"
	"net/netip"
	"syscall"

	"github.com/ze-software/ze/internal/core/mplsfib"
)

// handleMPLSEntry programs one batch of label entries and acknowledges it. A
// batch answering another writer's replay request is neither programmed nor
// acknowledged. Caller holds the plugin lifecycle lock.
func (f *fibVPP) handleMPLSEntry(batch *mplsfib.EntryBatch) {
	if batch == nil {
		return
	}
	if !batch.Addresses(f.labelReplayToken) {
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
	if f.retired {
		result = errors.New("mpls-fib: the fib-vpp writer was replaced; retry once VPP reconnects")
		return
	}
	if f.mplsBackend == nil {
		result = errors.New("mpls-fib: fib-vpp holds no MPLS backend")
		return
	}
	for i := range batch.Entries {
		result = errors.Join(result, f.applyLabelEntryLocked(&batch.Entries[i]))
	}
}

// applyLabelEntryLocked judges and programs one entry. Caller holds f.mu.
func (f *fibVPP) applyLabelEntryLocked(e *mplsfib.Entry) error {
	if err := mplsfib.ValidateEntry(e); err != nil {
		return err
	}
	if e.Op == mplsfib.OpPush && e.TableID != 0 {
		return fmt.Errorf("mpls-fib: push for %s selects private table %#x, an RSVP-TE facility-backup "+
			"context chosen by packet mark, and VPP has no packet-mark forwarding context", e.FEC, e.TableID)
	}
	switch e.Action {
	case mplsfib.ActionAdd:
		return f.addLabelEntryLocked(e)
	case mplsfib.ActionRemove:
		return f.delLabelEntryLocked(e)
	case mplsfib.ActionRemoveLabelSource:
		return f.delLabelSourceLocked(e.Source)
	case mplsfib.ActionUnspecified:
		return fmt.Errorf("mpls-fib: unsupported entry action %d", e.Action)
	default:
		// Entry actions are open inputs from MPLS forwarding producers.
		return fmt.Errorf("mpls-fib: unsupported entry action %d", e.Action)
	}
}

func (f *fibVPP) addLabelEntryLocked(e *mplsfib.Entry) error {
	switch e.Op {
	case mplsfib.OpPush:
		return f.addLabelPushLocked(e)
	case mplsfib.OpSwap, mplsfib.OpPop:
		owner, installed := f.labelOwners[e.InLabel]
		if installed && owner != e.Source {
			return fmt.Errorf("mpls-fib: in-label %d belongs to source %d, not %d: %w",
				e.InLabel, owner, e.Source, syscall.EEXIST)
		}
		if err := f.ensureMPLSTableLocked(); err != nil {
			return err
		}
		f.warnPathMTULocked(e)
		route := labelRoute{
			inLabel:   e.InLabel,
			outLabels: e.OutLabels,
			nextHop:   e.NextHop,
			fec:       e.FEC,
			ipTableID: f.ipTableID,
		}
		// RFC 3032 Section 2.1
		if err := f.mplsBackend.addLabelRoutes(route); err != nil {
			logger().Error("fib-vpp: mpls swap/pop install failed", "in-label", e.InLabel, "error", err,
				"code", mplsfib.CodeDoctorNoWriter)
			return err
		}
		f.labelOwners[e.InLabel] = e.Source
		f.reclaimStaleLabelLocked(e.InLabel)
	case mplsfib.OpUnspecified:
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	default:
		// Label operations are open inputs from MPLS forwarding producers.
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	}
	f.countLabelInstallLocked()
	return nil
}

// addLabelPushLocked programs a push at table zero: an imposed stack takes the
// labeled route path, an empty one the plain route path.
func (f *fibVPP) addLabelPushLocked(e *mplsfib.Entry) error {
	if !e.FEC.IsValid() {
		return fmt.Errorf("mpls-fib: cannot program push without a valid FEC (got %s)", e.FEC)
	}
	f.warnPathMTULocked(e)
	var err error
	if len(e.OutLabels) > 0 {
		// RFC 3032 Section 2.4.3
		err = f.mplsBackend.addMPLSRoute(e.FEC, e.NextHop, e.OutLabels)
	} else {
		// RFC 3032 Section 2.1: the downstream label was Implicit NULL, so
		// the FEC leaves as plain IP toward the next hop.
		err = f.backend.addRoute(e.FEC, e.NextHop)
	}
	if err != nil {
		logger().Error("fib-vpp: mpls push install failed", "fec", e.FEC, "error", err,
			"code", mplsfib.CodeDoctorNoWriter)
		return err
	}
	f.labelPushes[e.FEC] = len(e.OutLabels) > 0
	f.countLabelInstallLocked()
	return nil
}

func (f *fibVPP) delLabelEntryLocked(e *mplsfib.Entry) error {
	switch e.Op {
	case mplsfib.OpPush:
		if !e.FEC.IsValid() {
			return fmt.Errorf("mpls-fib: cannot remove push without a valid FEC (got %s)", e.FEC)
		}
		if err := f.delLabelPushLocked(e.FEC, len(e.OutLabels) > 0); err != nil {
			return err
		}
		delete(f.labelPushes, e.FEC)
		return nil
	case mplsfib.OpSwap, mplsfib.OpPop:
		owner, installed := f.labelOwners[e.InLabel]
		if !installed {
			return nil
		}
		if owner != e.Source {
			return fmt.Errorf("mpls-fib: in-label %d belongs to source %d, not %d: %w",
				e.InLabel, owner, e.Source, syscall.EEXIST)
		}
		if err := f.mplsBackend.delLabelRoutes(e.InLabel); err != nil {
			logger().Warn("fib-vpp: mpls swap/pop remove failed", "in-label", e.InLabel, "error", err,
				"code", mplsfib.CodeDoctorNoWriter)
			return err
		}
		delete(f.labelOwners, e.InLabel)
		return nil
	case mplsfib.OpUnspecified:
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	default:
		// Label operations are open inputs from MPLS forwarding producers.
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	}
}

// delLabelSourceLocked retires every label source owns, from the retained
// owner map rather than the producer's view. A failed deletion stays owned
// for the next call.
func (f *fibVPP) delLabelSourceLocked(source uint16) error {
	var result error
	for label, owner := range f.labelOwners {
		if owner != source {
			continue
		}
		entry := mplsfib.Entry{Op: mplsfib.OpPop, InLabel: label, Source: source}
		if err := f.delLabelEntryLocked(&entry); err != nil {
			result = errors.Join(result, fmt.Errorf("remove MPLS source %d label %d: %w", source, label, err))
		}
	}
	return result
}

// ensureMPLSTableLocked creates MPLS table 0 once per writer, which is one
// connection: VPP refuses a label route until the table exists
// (fib_api_table_id_decode answers VNET_API_ERROR_NO_SUCH_FIB).
func (f *fibVPP) ensureMPLSTableLocked() error {
	if f.labelTableReady {
		return nil
	}
	if err := f.mplsBackend.createMPLSTable(); err != nil {
		return err
	}
	f.labelTableReady = true
	return nil
}

// warnPathMTULocked logs once per writer that VPP has no per-route MTU, so an
// entry's PathMTU is not installed: labeled frames are bounded by the
// interface MTU alone, as on a kernel without the MPLS MTU patch.
func (f *fibVPP) warnPathMTULocked(e *mplsfib.Entry) {
	if e.PathMTU == 0 {
		return
	}
	if f.pathMTUWarned {
		return
	}
	f.pathMTUWarned = true
	logger().Warn("fib-vpp: VPP has no per-route MTU, so label entries are installed without their path MTU "+
		"and labeled frames are bounded by the interface MTU alone; set the MTU of each MPLS interface to "+
		"the smallest path MTU the LSPs need",
		"in-label", e.InLabel, "fec", e.FEC, "path-mtu", e.PathMTU, "code", codePathMTUUnenforced)
}

func (f *fibVPP) countLabelInstallLocked() {
	if m := fibVPPMetricsPtr.Load(); m != nil {
		m.routeInstalls.Inc()
	}
}

// delLabelPushLocked removes a push entry by the path it was installed on.
func (f *fibVPP) delLabelPushLocked(fec netip.Prefix, labeled bool) error {
	if labeled {
		return f.mplsBackend.delMPLSRoute(fec, nil)
	}
	return f.backend.delRoute(fec)
}

// flushLabelRoutesLocked removes every label route and push entry this writer
// installed. Caller holds f.mu.
func (f *fibVPP) flushLabelRoutesLocked() {
	if f.mplsBackend == nil {
		return
	}
	for label := range f.labelOwners {
		if err := f.mplsBackend.delLabelRoutes(label); err != nil {
			logger().Warn("fib-vpp: flush MPLS label failed; the next start removes it", "in-label", label,
				"error", err, "code", codeStaleLabel)
			continue
		}
		delete(f.labelOwners, label)
	}
	for fec, labeled := range f.labelPushes {
		if err := f.delLabelPushLocked(fec, labeled); err != nil {
			// The next writer's sweep reads MPLS table 0 only, so this
			// labeled IP route stays until its source re-installs or
			// withdraws the FEC.
			logger().Warn("fib-vpp: flush MPLS push failed; the labeled route stays in VPP until its source "+
				"installs or withdraws the FEC again", "fec", fec, "error", err, "code", codeStaleLabel)
			continue
		}
		delete(f.labelPushes, fec)
	}
}
