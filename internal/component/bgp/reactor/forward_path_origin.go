// Design: docs/architecture/bgp/structural-forwarding.md -- original mapping ownership.
// Related: source_receipt.go -- immutable ingress facts; forward_path_id.go -- mapping lock.
package reactor

import (
	"errors"
	"github.com/ze-software/ze/internal/core/source"
)

// fwdPathSource is a stable logical peer, including two distinct configured
// peers whose shared address gives them the same registry SourceID.
// Its zero value names no learned source. It grants no forwarding authority.
type fwdPathSource struct {
	sourceID  source.SourceID
	peerOwner uint64
}

// fwdPathOrigin is the immutable origin of one received generation, not the
// cache/replay ID or a current Session. Only newFwdPathOrigin constructs runtime
// values. Its zero value is invalid; callers MUST propagate constructor failure.
// The monotonic physical birth and ingress IDs already exist; no history is kept.
type fwdPathOrigin struct {
	source       fwdPathSource
	connectionID uint64
	messageID    uint64
}

// newFwdPathOrigin resolves original facts in one leaf receipt-store read.
// The caller MUST pass the original ReceivedUpdate, never an effective wire
// copy: policy and section copies do not carry the original validation MsgID.
func newFwdPathOrigin(update *ReceivedUpdate) (fwdPathOrigin, error) {
	if update == nil {
		return fwdPathOrigin{}, errors.New("path identifier origin: received update is missing")
	}
	if update.WireUpdate == nil {
		return fwdPathOrigin{}, errors.New("path identifier origin: received wire is missing")
	}
	if update.SourceReceipt == 0 {
		return fwdPathOrigin{}, errors.New("path identifier origin: original source receipt is missing")
	}
	if update.SourceOwner == 0 {
		return fwdPathOrigin{}, errors.New("path identifier origin: original peer owner is missing")
	}
	messageID := update.sourceMessageID()
	// Zero is the existing unversioned removal generation; it never supersedes
	// a versioned announcement on this connection and grants no authority.
	sourceReceipts.mu.Lock()
	defer sourceReceipts.mu.Unlock()
	record := sourceReceipts.entries[update.SourceReceipt]
	if record == nil {
		return fwdPathOrigin{}, errors.New("path identifier origin: original source receipt was released")
	}
	if record.facts.peerOwner != update.SourceOwner {
		return fwdPathOrigin{}, errors.New("path identifier origin: receipt belongs to another peer owner")
	}
	if record.facts.connectionID == 0 {
		return fwdPathOrigin{}, errors.New("path identifier origin: original physical connection is missing")
	}
	if record.facts.source.srcID != update.WireUpdate.SourceID() {
		return fwdPathOrigin{}, errors.New("path identifier origin: receipt belongs to another source")
	}
	return fwdPathOrigin{source: fwdPathSource{sourceID: record.facts.source.srcID, peerOwner: record.facts.peerOwner}, connectionID: record.facts.connectionID, messageID: messageID}, nil
}

// fwdPathMapping holds one bounded latest original stamp beside the current ID.
// A late replay can use an existing identifier without regressing that stamp.
// Its zero ID is valid; map membership, not the value, establishes presence.
type fwdPathMapping struct {
	id      uint32
	origin  fwdPathOrigin
	release *fwdPathRelease
}

// fwdPathRelease is one current compare-release claim, invalidated on every
// allocation/reuse. Nonzero size gives concurrently live claims distinct identity.
// One row holds at most one claim; no completed claim or history is retained.
type fwdPathRelease struct{ _ byte }

func (mapping fwdPathMapping) withOrigin(origin fwdPathOrigin) fwdPathMapping {
	mapping.release = nil
	if origin.connectionID < mapping.origin.connectionID {
		return mapping
	}
	if origin.connectionID == mapping.origin.connectionID {
		if origin.messageID < mapping.origin.messageID {
			return mapping
		}
	}
	mapping.origin = origin
	return mapping
}

// releasedBy admits only a withdrawal at least as new as this mapping. A
// reconnect takes ownership away from an old retained withdrawal; a newer
// withdrawal can retire an obsolete mapping once its original presence is gone.
func (mapping fwdPathMapping) releasedBy(origin fwdPathOrigin) bool {
	if origin.connectionID < mapping.origin.connectionID {
		return false
	}
	if origin.connectionID > mapping.origin.connectionID {
		return true
	}
	return mapping.origin.messageID <= origin.messageID
}
