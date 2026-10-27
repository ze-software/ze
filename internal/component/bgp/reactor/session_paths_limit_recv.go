// Design: docs/guide/add-path.md -- the receive ceiling under Ze's own PATHS-LIMIT
// RFC: rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt -- Section 3
// Related: session_paths_limit.go -- the admission state, section filter and journal shared with send
package reactor

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// pathsLimitRefusedHeadroom is how many refused identifiers one route tracks
// beyond its admitted ones. A peer that ignores the limit cannot decide how
// much memory the audit costs: the tracked set per route never exceeds the
// limit plus this headroom (ExaBGP's IncomingRIB.track_path uses the same 1).
const pathsLimitRefusedHeadroom = 1

// initPathsLimitRecv builds the receive ceiling at OPEN negotiation, on the
// session read goroutine that is its only reader. A family is limited only
// where Ze advertised a nonzero PATHS-LIMIT and ADD-PATH receive is negotiated:
// a limit configured for a send-only family enforces nothing.
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "An AFI/SAFI tuple MUST be
// ignored if the same tuple was not received in the ADD-PATH capability."
func (s *Session) initPathsLimitRecv(encoding *capability.EncodingCaps) {
	s.pathsLimitRecv = nil
	if encoding == nil {
		return
	}
	s.pathsLimitRecv = newPathsLimitAdmission(encoding.PathsLimitRecv, encoding.AddPathMode, capability.AddPathReceive)
	s.warnPathsLimitUnenforced(encoding.PathsLimitRecv, encoding.AddPathMode, capability.AddPathReceive)
}

// receiveCeiling admits one received UPDATE against Ze's advertised
// PATHS-LIMIT and answers the UPDATE to deliver, the journal of what it
// admitted, and whether anything is left to deliver.
//
// An UPDATE that refuses nothing is delivered as it arrived, with no copy. One
// that refuses a path is rewritten into one fresh copy with the refused NLRIs
// removed and the lengths fixed. The rewrite keeps the session's ContextID,
// because the attributes are untouched and the bytes are still encoded in that
// context. An UPDATE whose every announcement was refused, and that withdraws
// nothing, answers false with a nil journal: it is consumed, never delivered
// as an empty body that reads as End-of-RIB. A failed admission is logged,
// rolled back and also answers false, so no path is admitted uncounted.
//
// When it answers true, the caller MUST either call rollback on the journal if
// the UPDATE is then dropped or ends the session, or reportPathsLimitRefusals
// if it is delivered, and MUST call release on the journal in both cases.
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "An implementation SHOULD
// provide a configuration knob to specify the maximum number of paths to
// accept from a sender."
func (s *Session) receiveCeiling(update *wireu.WireUpdate) (*wireu.WireUpdate, *pathsLimitChanges, bool) {
	payload := update.Payload()

	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	dropped, err := s.pathsLimitRecv.admit(payload, changes)
	if err != nil {
		changes.rollback()
		changes.release()
		s.logReceiveCeilingFailure(err, receiveCeilingDropped)
		return nil, nil, false
	}
	if dropped == 0 {
		return update, changes, true
	}

	// Rolling back and filtering replays the same decisions into the copy, so
	// only an UPDATE that refused a path pays for the rewrite.
	changes.rollback()
	changes.release()
	changes, _ = pathsLimitScratch.Get().(*pathsLimitChanges)
	rewritten := make([]byte, len(payload))
	n, _, err := s.pathsLimitRecv.filter(rewritten, payload, changes)
	if err != nil {
		changes.rollback()
		changes.release()
		s.logReceiveCeilingFailure(err, receiveCeilingDropped)
		return nil, nil, false
	}
	if n == 0 {
		s.reportPathsLimitRefusals(changes)
		changes.release()
		return nil, nil, false
	}
	admitted := wireu.NewWireUpdate(rewritten[:n:n], update.SourceCtxID())
	admitted.SetSourceID(update.SourceID())
	return admitted, changes, true
}

// settleReceiveCeiling ends the journal receiveCeiling answered, once the
// prefix limits decided the UPDATE's fate. A dropped UPDATE reached no RIB, so
// its admissions are rolled back, as checkPrefixLimits rolls back its own
// count; a delivered one has its refusals counted and warned. It MUST be
// called exactly once for each journal receiveCeiling answered.
func (s *Session) settleReceiveCeiling(changes *pathsLimitChanges, dropped bool) {
	if dropped {
		changes.rollback()
	} else {
		s.reportPathsLimitRefusals(changes)
	}
	changes.release()
}

// receiveCeilingWithdrawals frees the slots a withdraw-only UPDATE releases.
// The RFC 7606 treat-as-withdraw extra bodies are dispatched on their own, so
// their withdrawals are final whatever happens to the primary body. A
// withdrawal always passes the ceiling, so the body is never rewritten.
func (s *Session) receiveCeilingWithdrawals(body []byte) {
	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	if _, err := s.pathsLimitRecv.admit(body, changes); err != nil {
		changes.rollback()
		s.logReceiveCeilingFailure(err, receiveCeilingSlotsKept)
	}
	changes.release()
}

// What a failed receive admission did to the UPDATE, for the operator.
const (
	receiveCeilingDropped   = "the UPDATE is dropped: none of its routes is installed or withdrawn, so no path is admitted uncounted"
	receiveCeilingSlotsKept = "the withdrawal is still delivered, but the paths it names keep their slots until the session ends"
)

func (s *Session) logReceiveCeilingFailure(err error, effect string) {
	sessionLogger().Error("PATHS-LIMIT: cannot count the paths of a received UPDATE",
		"peer", s.settings.Address,
		"error", err,
		"effect", effect,
		"fix", "remove 'limit { receive N; }' from the add-path block of this family, and report the error")
}

// admit applies one UPDATE to the admission without copying it: every
// withdrawal first, then every announcement, in the order filter walks them,
// which it MUST keep. It answers how many announcements were refused. A caller
// that must remove them from the bytes rolls the journal back and runs filter,
// which replays the same decisions.
func (a pathsLimitAdmission) admit(body []byte, changes *pathsLimitChanges) (int, error) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return 0, err
	}
	if _, _, err := a.section(nil, sections.Withdrawn(body), family.IPv4Unicast, true, changes); err != nil {
		return 0, err
	}
	iter := attribute.NewAttrIterator(sections.Attrs(body))
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		if code != attribute.AttrMPUnreachNLRI {
			continue
		}
		fam, off, err := pathsLimitMP(value, false)
		if err != nil {
			return 0, err
		}
		if _, _, err := a.section(nil, value[off:], fam, true, changes); err != nil {
			return 0, err
		}
	}
	if iter.Remaining() != 0 {
		return 0, wire.ErrUpdateTruncated
	}

	dropped := 0
	iter.Reset()
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		if code != attribute.AttrMPReachNLRI {
			continue
		}
		fam, off, err := pathsLimitMP(value, true)
		if err != nil {
			return 0, err
		}
		_, removed, err := a.section(nil, value[off:], fam, false, changes)
		if err != nil {
			return 0, err
		}
		dropped += removed
	}
	_, removed, err := a.section(nil, sections.NLRI(body), family.IPv4Unicast, false, changes)
	if err != nil {
		return 0, err
	}
	return dropped + removed, nil
}

// reportPathsLimitRefusals counts every path the receive ceiling refused in one
// message and warns once for each route whose refused set was empty before it.
// The receive caller MUST call it once, after the message's admission is final
// and before releasing the journal. It MUST NOT be called after rollback: a
// rolled-back refusal is not final and its route warns again on replay.
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "An implementation SHOULD
// provide a configuration knob to specify the maximum number of paths to
// accept from a sender."
func (s *Session) reportPathsLimitRefusals(changes *pathsLimitChanges) {
	if changes.refusals == 0 {
		return
	}
	for i := range changes.entries[:changes.n] {
		change := &changes.entries[i]
		if change.kind != pathsLimitRefusalTracked && change.kind != pathsLimitRefusalCounted {
			continue
		}
		if s.prefixMetrics != nil {
			s.prefixMetrics.pathsLimitRefused.With(s.peerLabel(), change.family.family.String()).Inc()
		}
		if !change.warn {
			continue
		}
		sessionLogger().Warn("PATHS-LIMIT: peer sent more paths for one route than Ze advertised it would accept; the excess path is dropped",
			"peer", s.settings.Address,
			"family", change.family.family.String(),
			"limit", change.family.limit,
			"route-key", hex.EncodeToString([]byte(change.prefix.key)),
			"path-id", change.id,
			"fix", "raise 'add-path limit receive' for this family, or have the peer honor the PATHS-LIMIT capability",
			"repeats", "suppressed for this route until the peer withdraws the refused path")
	}
}
