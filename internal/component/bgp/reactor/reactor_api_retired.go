// Design: docs/architecture/behavior/peer-lifecycle.md -- API announcements across a session seal.
// Related: reactor_api_batch.go -- shared batch building and queue admission.
// Related: peer_send.go -- synchronous split progress and ownership lifetime.

package reactor

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
)

var errAnnounceQueueBoundary = errors.New("unsent announcement boundary is unknown")

// queueRetiredAnnounce gives ordinary API input admitted before the seal to
// the existing next-session queue. Captured receipts and initial replay MUST
// retain their session authority; they never become new local origination.
// The caller MUST keep update's build buffers and queue attributes borrowed
// until return. QueueAnnounce takes ownership of each retained route.
func (a *reactorAPIAdapter) queueRetiredAnnounce(ctx context.Context, target announceTarget, update *message.Update, batch bgptypes.NLRIBatch, facts announceFacts, attrs []attribute.Attribute, userASPath *attribute.ASPath, cause error) error {
	if batch.SentOwnerMessage != 0 {
		return cause
	}
	if batch.InitialReplay != 0 {
		return cause
	}
	if !errors.Is(cause, errAdjOutSession) {
		// A completed teardown reaches the sender's state or connection check
		// before its sealed-writer check. Only this captured session's permanent
		// seal makes either early error a retirement, rather than an ordinary
		// not-ready peer whose existing error policy stays unchanged.
		if target.session == nil {
			return cause
		}
		if !target.session.tearingDown.Load() {
			return cause
		}
		if !errors.Is(cause, ErrInvalidState) {
			if !errors.Is(cause, ErrNotConnected) {
				return cause
			}
		}
	}
	var progress *splitUpdateFailure
	if !errors.As(cause, &progress) {
		return announceQueueBoundaryFailure(cause)
	}
	if progress.partial {
		// Export policy can replace or reorder prefixes within this callback.
		// Already committed final output has sent history, but a source suffix
		// cannot be recovered by counting it. Report the partial failure rather
		// than duplicating that output or silently claiming full acceptance.
		return announceQueueBoundaryFailure(cause)
	}
	remaining, err := retiredAnnounceSuffix(update, batch, facts, progress.completed)
	if err != nil {
		return announceQueueBoundaryFailure(err)
	}
	batch.NLRIs = remaining
	_, err = a.queueAnnounceForPeer(ctx, target.peer, batch, attrs, userASPath, facts.isIBGP)
	if err != nil {
		return opererr.Wrap(err,
			"BGP announcement could not be queued for the next session",
			"Some routes can already be queued. The peer's queue refused the remaining routes. Check the session, then repeat the original announcement after it reconnects.",
			"show bgp peer list")
	}
	return nil
}

// announceQueueBoundaryFailure preserves the refusal and its evidence while
// giving the operator one next step. It runs only after retirement made the
// unsent source routes unknown; it never changes a successful send's result.
func announceQueueBoundaryFailure(cause error) error {
	if !errors.Is(cause, errAnnounceQueueBoundary) {
		cause = errors.Join(errAnnounceQueueBoundary, cause)
	}
	return opererr.Wrap(cause,
		"BGP announcement could not be completed",
		"Ze cannot identify the unsent original routes. Some routes can already have reached the peer. Check the session, then repeat the original announcement after it reconnects.",
		"show bgp peer list")
}

// retiredAnnounceSuffix reuses the same splitter only after a failed write.
// Completed callbacks are mapped to the original NLRI bytes, then to the
// caller's typed NLRIs; the common successful send does no second wire walk.
// Both walks are bounded by this UPDATE's build buffers and batch length.
// The caller MUST retain update's borrowed storage through this synchronous use.
func retiredAnnounceSuffix(update *message.Update, batch bgptypes.NLRIBatch, facts announceFacts, completed int) ([]nlri.NLRI, error) {
	if completed == 0 {
		return batch.NLRIs, nil
	}
	maxSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, facts.extended))
	splitter := message.GetSplitter()
	defer message.PutSplitter(splitter)
	seen := 0
	consumed := 0
	if err := splitter.Split(update, maxSize, func(family.Family) bool { return facts.addPath }, func(chunk *message.Update) error {
		if seen < completed {
			n, err := announceChunkNLRIBytes(chunk, batch.Family)
			if err != nil {
				return err
			}
			consumed += n
		}
		seen++
		return nil
	}); err != nil {
		return nil, err
	}
	if completed > seen {
		return nil, errAnnounceQueueBoundary
	}
	offset := 0
	for consumed > 0 && offset < len(batch.NLRIs) {
		size := nlri.LenWithContext(batch.NLRIs[offset], facts.addPath)
		if size <= 0 {
			return nil, errAnnounceQueueBoundary
		}
		if size > consumed {
			return nil, errAnnounceQueueBoundary
		}
		consumed -= size
		offset++
	}
	if consumed != 0 {
		return nil, errAnnounceQueueBoundary
	}
	return batch.NLRIs[offset:], nil
}

// announceChunkNLRIBytes reads only the announcement framing the builder made.
// RFC 4271 Section 4.3: legacy NLRI follows the path-attribute block.
// RFC 4760 Section 3: MP_REACH carries AFI(2), SAFI(1), next-hop length(1),
// next-hop, reserved(1), then the NLRI. pathsLimitMP owns that framing reader.
func announceChunkNLRIBytes(chunk *message.Update, fam family.Family) (int, error) {
	if len(chunk.NLRI) != 0 {
		if fam != family.IPv4Unicast {
			return 0, errAnnounceQueueBoundary
		}
		return len(chunk.NLRI), nil
	}
	iter := attribute.NewAttrIterator(chunk.PathAttributes)
	for {
		code, _, value, ok := iter.Next()
		if !ok {
			return 0, errAnnounceQueueBoundary
		}
		if code == attribute.AttrMPReachNLRI {
			// RFC 4760 Section 3.
			chunkFamily, offset, err := pathsLimitMP(value, true)
			if err != nil {
				return 0, errAnnounceQueueBoundary
			}
			if chunkFamily != fam {
				return 0, errAnnounceQueueBoundary
			}
			return len(value) - offset, nil
		}
	}
}
