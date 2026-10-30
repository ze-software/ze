// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owners
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: ../../../component/bgp/plugins/rib/source_policy.go -- bgp-rib's store-specific phases
// Related: ../../../component/bgp/plugins/adj_rib_in/source_policy.go -- bgp-adj-rib-in's store-specific phases

// Package sourcepolicy is the transaction ledger every source-policy owner
// keeps: the phase a transaction reached, the identity it was prepared for, and
// the recorded outcome of each phase, so a duplicate delivery returns that
// outcome unchanged and a foreign, stale or out-of-order phase is refused
// before the owner changes anything. What a phase does to a store stays in the
// owner; this package decides only whether the phase may run.
package sourcepolicy

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Stage is the last phase a transaction completed. The zero value is no phase:
// a ledger becomes Prepared when its owner records the prepare.
type Stage uint8

const (
	StageUnspecified Stage = iota
	StagePrepared
	StageApplied
	StageAborted
	StageFinalized
)

// outcome is the recorded answer of one completed phase.
type outcome struct {
	target uint64
	err    error
}

// Ledger is one source's transaction state. It is not safe for concurrent use:
// the owner guards it with the lock that guards its store.
type Ledger struct {
	id       string
	source   rpc.SourcePolicyIdentity
	stage    Stage
	outcomes map[rpc.SourcePolicyOperation]outcome
}

// NewLedger starts the ledger of the transaction input prepares. Its stage
// stays Unspecified until the owner records the prepare's outcome.
func NewLedger(input rpc.SourcePolicyInput) Ledger {
	return Ledger{id: input.TransactionID, source: input.Source,
		outcomes: make(map[rpc.SourcePolicyOperation]outcome, 4)}
}

// Source is the captured identity the transaction was prepared for.
func (l *Ledger) Source() rpc.SourcePolicyIdentity { return l.source }

// Stage is the last phase the transaction completed.
func (l *Ledger) Stage() Stage { return l.stage }

// Record stores a completed phase's outcome for duplicate delivery, moves the
// ledger to stage, and returns err.
func (l *Ledger) Record(input rpc.SourcePolicyInput, stage Stage, err error) error {
	l.outcomes[input.Operation] = outcome{target: input.NextPolicyEpoch, err: err}
	l.stage = stage
	return err
}

// Owner is a source-policy owner's plugin name, which opens every refusal it
// returns so an operator reads which store refused.
type Owner string

// Phases are an owner's store-specific handlers, one per operation. Each is
// called with the source peer parsed from the request.
type Phases struct {
	Prepare  func(netip.Addr, rpc.SourcePolicyInput) error
	Apply    func(netip.Addr, rpc.SourcePolicyInput) error
	Abort    func(netip.Addr, rpc.SourcePolicyInput) error
	Finalize func(netip.Addr, rpc.SourcePolicyInput) error
}

// Handle parses Source.Peer as the peer address the received-UPDATE events
// carry and runs the phase input names.
func (o Owner) Handle(input rpc.SourcePolicyInput, phases Phases) error {
	peer, err := netip.ParseAddr(input.Source.Peer)
	if err != nil {
		return errors.New(string(o) + ": source-policy peer " + input.Source.Peer + " is not an IP address: " + err.Error())
	}
	switch input.Operation {
	case rpc.SourcePolicyPrepare:
		return phases.Prepare(peer, input)
	case rpc.SourcePolicyApply:
		return phases.Apply(peer, input)
	case rpc.SourcePolicyAbort:
		return phases.Abort(peer, input)
	case rpc.SourcePolicyFinalize:
		return phases.Finalize(peer, input)
	case rpc.SourcePolicyUnspecified:
		return errors.New(string(o) + ": source-policy operation is unspecified")
	default: // Open set: the request crossed a process boundary.
		return errors.New(string(o) + ": unknown source-policy operation")
	}
}

// Reprepare answers a prepare against the ledger the source already holds,
// which is nil when it holds none. done reports that err is the final answer:
// the recorded outcome of the same transaction's prepare, a refusal of a
// prepare that names another identity under the same transaction, or a
// refusal because another transaction is still open. done false means the
// owner prepares a new transaction, replacing the ledger it holds and
// releasing that ledger's pins.
//
// A Prepared ledger of another transaction is abandoned at any identity, and
// an Applied one whose identity is older than the incoming one is too: the
// coordinator holds one transaction per source at a time and captured the
// incoming identity only after the older transaction ended, so a lost or
// timed-out abort or finalize left it behind. Replacing it releases its pins
// only; a denial an applied ledger recorded in the owner's store stays, so
// nothing becomes eligible. An Applied ledger whose identity is equal to or
// newer than the incoming one still holds the source: the incoming prepare
// may itself be the stale one.
func (o Owner) Reprepare(l *Ledger, input rpc.SourcePolicyInput) (done bool, err error) {
	if l == nil {
		return false, nil
	}
	if l.id == input.TransactionID {
		if l.source != input.Source {
			return true, errors.New(string(o) + ": transaction " + input.TransactionID +
				" names another session or epoch of source " + input.Source.Peer + " than the one it prepared")
		}
		return true, l.outcomes[rpc.SourcePolicyPrepare].err
	}
	if l.stage != StagePrepared && l.stage != StageApplied {
		return false, nil
	}
	// A Prepared ledger of another transaction was never applied: it pins and
	// changes nothing else. The coordinator claims a peer for one transaction
	// at a time (sourcePolicyCoordinator.claim) and released this one's claim
	// before it captured the incoming one, so the ledger was abandoned by a
	// lost target-zero abort, at any identity, the same one included.
	if l.stage == StagePrepared {
		return false, nil
	}
	if Supersedes(input.Source, l.source) {
		return false, nil
	}
	return true, o.held(l, input)
}

// Supersedes reports whether identity next was captured after previous for the
// same source: a later session, or a later policy epoch of the same session.
func Supersedes(next, previous rpc.SourcePolicyIdentity) bool {
	if next.SessionGeneration != previous.SessionGeneration {
		return next.SessionGeneration > previous.SessionGeneration
	}
	return next.PolicyEpoch > previous.PolicyEpoch
}

// held refuses a prepare while another transaction holds the source.
func (o Owner) held(l *Ledger, input rpc.SourcePolicyInput) error {
	return errors.New(string(o) + ": source " + input.Source.Peer + " is held by open transaction " +
		l.id + "; finalize or abort it first")
}

// Admit matches a phase after prepare to the source's ledger, nil when the
// source holds none. It answers a target-zero abort of a transaction the
// source never prepared as a duplicate with no error (neverPrepared). It
// answers a duplicate delivery with the recorded outcome (duplicate true) and
// refuses a foreign, stale or unprepared token, and a phase out of order,
// before the owner changes anything. Finalize is admitted
// only after apply, and a finalized ledger refuses abort, so a stale phase
// cannot undo an accepted decision.
func (o Owner) Admit(l *Ledger, input rpc.SourcePolicyInput) (duplicate bool, err error) {
	if neverPrepared(l, input) {
		// A target-zero abort only releases the pins its transaction's prepare
		// took. The coordinator sends it to an owner whose prepare failed or
		// timed out too, because that prepare may still have pinned: when it
		// did not, the abort's outcome already holds, so it is answered as done.
		return true, nil
	}
	if l == nil {
		return false, errors.New(string(o) + ": source " + input.Source.Peer +
			" has no prepared source-policy transaction (never prepared, or its session ended); prepare the transaction again")
	}
	if l.id != input.TransactionID {
		return false, errors.New(string(o) + ": source " + input.Source.Peer + " belongs to transaction " +
			l.id + ", not " + input.TransactionID + "; a stale or foreign token changes nothing")
	}
	if l.source != input.Source {
		return false, errors.New(string(o) + ": transaction " + input.TransactionID +
			" names another session or epoch of source " + input.Source.Peer + " than the one it prepared")
	}
	if recorded, done := l.outcomes[input.Operation]; done {
		if recorded.target != input.NextPolicyEpoch {
			return false, errors.New(string(o) + ": transaction " + input.TransactionID +
				" already ran this phase with another target epoch")
		}
		return true, recorded.err
	}
	return false, o.ordered(l, input)
}

// neverPrepared reports a target-zero abort of a transaction this source holds
// no ledger for: none at all, or another transaction's.
func neverPrepared(l *Ledger, input rpc.SourcePolicyInput) bool {
	if input.Operation != rpc.SourcePolicyAbort {
		return false
	}
	if input.NextPolicyEpoch != 0 {
		return false
	}
	return l == nil || l.id != input.TransactionID
}

// ordered refuses a first delivery of a phase the ledger's stage forbids.
func (o Owner) ordered(l *Ledger, input rpc.SourcePolicyInput) error {
	switch input.Operation {
	case rpc.SourcePolicyApply:
		if l.stage != StagePrepared {
			return errors.New(string(o) + ": transaction " + input.TransactionID + " cannot apply after abort or finalize")
		}
		return nil
	case rpc.SourcePolicyAbort:
		return o.abortable(l, input)
	case rpc.SourcePolicyFinalize:
		// Abort, never finalize, closes a transaction that was not applied:
		// finalize accepts an applied denial and nothing else.
		if l.stage != StageApplied {
			return errors.New(string(o) + ": transaction " + input.TransactionID +
				" was not applied; abort it instead of finalizing it")
		}
		return nil
	case rpc.SourcePolicyPrepare:
		return errors.New(string(o) + ": transaction " + input.TransactionID + " is already past prepare")
	case rpc.SourcePolicyUnspecified:
		return errors.New(string(o) + ": source-policy operation is unspecified")
	default: // Open set: the request crossed a process boundary.
		return errors.New(string(o) + ": unknown source-policy operation")
	}
}

// abortable refuses a first abort the ledger's stage forbids. A zero target
// aborts a transaction that was prepared and never applied, so it is admitted
// only from Prepared: the owner then releases its pins and changes nothing
// else. A nonzero target restamps the restored paths to the source's advanced
// epoch: after apply it must be newer than the apply target, and from
// Prepared it is admitted too, because a coordinator whose apply failed part
// way has already advanced the epoch and every prepared owner, applied or
// not, must restamp to it. A closed transaction admits no new abort.
func (o Owner) abortable(l *Ledger, input rpc.SourcePolicyInput) error {
	switch l.stage {
	case StagePrepared:
		return nil
	case StageApplied:
		if input.NextPolicyEpoch == 0 {
			return errors.New(string(o) + ": transaction " + input.TransactionID +
				" was applied; its abort needs a target epoch newer than the apply target, not zero")
		}
		if l.outcomes[rpc.SourcePolicyApply].target >= input.NextPolicyEpoch {
			return errors.New(string(o) + ": transaction " + input.TransactionID + " abort target must be newer than its apply target")
		}
		return nil
	case StageFinalized:
		return errors.New(string(o) + ": transaction " + input.TransactionID + " is finalized; a stale abort cannot undo it")
	case StageAborted:
		return errors.New(string(o) + ": transaction " + input.TransactionID + " is already aborted")
	case StageUnspecified:
		return errors.New(string(o) + ": transaction " + input.TransactionID + " was never prepared; nothing to abort")
	default: // Closed set: only this package writes the stage; a new stage must be handled here.
		panic("BUG: unknown source-policy ledger stage")
	}
}
