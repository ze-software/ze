// Design: docs/architecture/api/process-protocol.md -- original source-session authority.
// Related: peer_session_factory.go -- immutable callback construction.
// Related: peer_session_end.go -- ordered event-delivery release.
package reactor

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The authority clock spans Reactor replacement. Its zero value starts no
// worker or connection in declaration mode, and released entries leave no history.
var sourceReceipts sourceReceiptStore

// captureSourceReceipt captures negotiated facts of this exact Session before
// its UP can reach a deciding process. Repeated capture of the same Session is
// idempotent. The caller MUST call closeSourceReceipt after its ordered DOWN.
func (s *Session) captureSourceReceipt(peer *Peer, store *sourceReceiptStore) error {
	if s.sourceReceipt.Load() != nil {
		return nil
	}
	s.mu.RLock()
	receiveContext := s.recvCtxID
	s.mu.RUnlock()
	if receiveContext == 0 {
		return errors.New("source receipt: negotiated receive encoding is missing")
	}
	contextID := fwdContextIDWithASN4(receiveContext, true)
	context := bgpctx.Registry.Get(contextID)
	if context == nil {
		return errors.New("source receipt: negotiated receive encoding is unknown")
	}
	if !context.ASN4() {
		return errors.New("source receipt: normalized four-octet receive encoding could not be registered")
	}
	src := relaySourceForPeer(peer, contextID, s.advertisedLLGR())
	facts, err := newSourceReceiptFacts(&src, s.connectionID)
	if err != nil {
		return err
	}
	live, err := store.issue(facts)
	if err != nil {
		return err
	}
	if !s.sourceReceipt.CompareAndSwap(nil, live) {
		live.close()
	}
	return nil
}

// sourceReceiptID names only this Session. Before capture it returns the
// explicitly unrecorded zero, never the current Peer's replacement receipt.
func (s *Session) sourceReceiptID() uint64 {
	if live := s.sourceReceipt.Load(); live != nil {
		return live.ID()
	}
	return 0
}

// closeSourceReceipt MUST follow captureSourceReceipt after receive delivery
// and its synchronous DOWN have settled, including failure and panic paths.
// The immutable identity remains inspectable; only its delivery reference ends.
func (s *Session) closeSourceReceipt() {
	if live := s.sourceReceipt.Load(); live != nil {
		live.close()
	}
}

// sourceMessageCallback preserves the public callback and any observer wrapper.
// It binds received UPDATEs to this Session before their buffer can transfer.
// The factory MUST install it before TCP; sent provenance comes from its writer.
func (s *Session) sourceMessageCallback(next MessageCallback) MessageCallback {
	if next == nil {
		return nil
	}
	return func(_ *Session, address netip.Addr, kind msgtype.MessageType, body []byte, update *wireu.WireUpdate, context bgpctx.ContextID,
		direction rpc.MessageDirection, buffer BufHandle, meta map[string]any, source string, id uint64) bool {
		if direction == rpc.DirectionReceived {
			if kind == msgtype.TypeUPDATE {
				if update != nil {
					update.SetSourceReceipt(s.sourceReceiptID())
					if live := s.sourceReceipt.Load(); live != nil {
						update.SetSourceOwner(live.record.facts.peerOwner)
					}
				}
			}
		}
		return next(s, address, kind, body, update, context, direction, buffer, meta, source, id)
	}
}

// messageReceiverForPeer binds lifecycle and policy facts to an exact configured
// Peer, because two sessions can share an address and use different TCP ports.
// The caller installs this once before constructing the Peer's first Session.
func (r *Reactor) messageReceiverForPeer(peer *Peer) MessageCallback {
	return func(captured *Session, address netip.Addr, kind msgtype.MessageType, body []byte, update *wireu.WireUpdate, context bgpctx.ContextID,
		direction rpc.MessageDirection, buffer BufHandle, meta map[string]any, source string, id uint64) bool {
		if kind == msgtype.TypeROUTEREFRESH {
			if direction == rpc.DirectionReceived {
				// RFC 7313 Section 4 / RFC 2918 Section 3: non-Cap-70 Reserved octet is ignored.
				body = captured.normalizeReceivedRouteRefresh(body)
			}
		}
		return r.notifyMessageReceiverFrom(peer, captured, address, kind, body, update, context, direction, buffer, meta, source, id)
	}
}
