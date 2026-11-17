// Design: docs/architecture/update-building.md -- authenticated selected writer.
// Related: source_receipt.go -- immutable source authority and revocation join.
// RFC 4271 Section 9.1.3 -- see rfc/short/rfc4271.md.
package reactor

import (
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedWrite borrows one validated selection until its final flush settles.
// Its zero value is invalid. Only the selected request's admission constructs
// it; an absence intentionally carries no received-source write token.
type selectedWrite struct {
	output             selectedOutput
	session            *Session
	destinationSession uint64
	snapshot           *disseminationSnapshot
	source             sourceReceiptWrite
	cleanup            *selectedCleanupPermit
	absent             bool
	retirement         bool
	retireKey          adjOutKey
	route              rpc.SelectedRoute
}

// current runs under the final socket lock, before any ownership mutation.
// The request's independent current-selection lookup additionally fences its
// revision; it MUST run before writing the staged bodies or retiring entries.
func (write *selectedWrite) current(session *Session) error {
	if write == nil {
		return errors.New("select-routes: selected write authority is missing")
	}
	if write.session != session {
		return errAdjOutSession
	}
	if write.destinationSession != session.initialReplay.Load() {
		return errAdjOutSession
	}
	if write.cleanup != nil {
		if write.cleanup.session != session {
			return errAdjOutSession
		}
		if !write.absent {
			return errAdjOutProvenance
		}
		return nil
	}
	if write.snapshot == nil {
		return errors.New("select-routes: destination dissemination snapshot is missing")
	}
	if write.snapshot.session != session {
		return errAdjOutSession
	}
	if write.snapshot.peer.selectedOwnerSnapshot() != write.snapshot {
		return errors.New("select-routes: destination dissemination changed; reconcile through its current configured owner")
	}
	if err := write.output.owner.Current(); err != nil {
		return err
	}
	if write.absent {
		return nil
	}
	return write.source.current()
}

func (write *selectedWrite) owns(output selectedOutput) bool {
	if !write.output.owner.SameIncarnation(output.owner) {
		return false
	}
	return output.key == write.output.key
}

// validateSelectedOwner requires the authenticated process before permission
// lookup. An operator and a zero sender never become an ordinary advertiser.
func validateSelectedOwner(owner plugin.SelectedOwner) error {
	if err := owner.Current(); err != nil {
		return err
	}
	_, named := owner.Sender().Process()
	if !named {
		return errors.New("select-routes: caller is not a selecting process")
	}
	return nil
}
