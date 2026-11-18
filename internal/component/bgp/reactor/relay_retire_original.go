// Design: docs/architecture/api/process-protocol.md -- exact original-source retirement.
// Related: session_ownership.go -- existing expected SENT receipt authority.
package reactor

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// retireOriginalStoredRoute cannot announce or reconstruct source facts. Its
// authority is only an exact acknowledged SENT slot of the named original
// owner/physical Session. Unknown originals settle absence without a registry.
func (a *reactorAPIAdapter) retireOriginalStoredRoute(ctx context.Context, peers []*Peer, route *rpc.StoredRoute, sender plugin.Sender, bound *boundForwardRequest) error {
	if route.SourceOwner == 0 {
		return errRelayOriginalSource
	}
	if route.MsgID == 0 {
		return errRelayOriginalSource
	}
	if route.SourceConnectionID == 0 {
		return errRelayOriginalSource
	}
	if !route.Withdraw {
		return errRelayOriginalSource
	}
	if bound == nil {
		if !sender.IsOperator() {
			return errRelayOriginalSource
		}
	}
	fam, known := family.LookupFamily(route.Family)
	if !known {
		return errRelayFamily
	}
	raw, err := hex.DecodeString(route.NLRIHex)
	if err != nil {
		return errRelayHex
	}
	received := route.PathID
	switch route.NLRIFraming {
	case rpc.NLRIFramingPrefixOnly:
	case rpc.NLRIFramingSourceWire:
		// A released Session's framing cannot be recovered from byte guesses.
		// Missing-path producers MUST supply PrefixOnly plus the original PathID,
		// including zero, from their stored native key.
		return errRelayNLRIFraming
	case rpc.NLRIFramingUnrecorded:
		return errRelayNLRIFraming
	default:
		return errRelayNLRIFraming
	}
	var native fwdPathKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if err := fwdPathKeyFor(&native, fam, 0, raw, true, scratch[:]); err != nil {
		return err
	}
	for _, peer := range peers {
		session := peer.currentSession()
		if session == nil {
			return errAdjOutSession
		}
		if bound != nil {
			if err := bound.owner.Current(); err != nil {
				return err
			}
			if err := destinationCurrent(peer, session, sender); err != nil {
				return err
			}
		}
		owner := fwdItem{peer: peer, session: session, authority: adjOutExpected}
		owner.destinationOwner, owner.destinationSession = sender.Destination()
		err := session.sendUpdateCounted(ctx, &message.Update{}, nil, false, &owner, func(s *Session, _ *message.Update) error {
			if bound != nil {
				if err := bound.owner.Current(); err != nil {
					return err
				}
				if err := destinationCurrent(peer, s, sender); err != nil {
					return err
				}
			}
			if s.adjOut == nil {
				return nil
			}
			// Count is this current table's population, not arbitrary input or history.
			s.adjOut.mu.Lock()
			limit := len(s.adjOut.routes)
			s.adjOut.mu.Unlock()
			for range limit {
				key, path, wire, found, err := s.originalStoredRetirement(route, native, received)
				if err != nil {
					return err
				}
				if !found {
					return nil
				}
				item := owner
				item.expectedMessage = path.revision
				item.receivedPeer = path.source
				item.receivedGeneration = path.generation
				item.sourcePolicyEpoch = path.policyEpoch
				item.sourceReceipt = path.sourceReceipt
				item.sourceMessageID = path.message
				item.sourcePeerStr = path.source.addrString
				s.sentForward = &item
				if err := s.writeSelectedWithdrawal(key.path.family, wire); err != nil {
					return err
				}
			}
			_, _, _, unresolved, err := s.originalStoredRetirement(route, native, received)
			if err != nil {
				return err
			}
			if unresolved {
				return errors.New("original source retirement did not settle its current SENT slots")
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Session) originalStoredRetirement(route *rpc.StoredRoute, native fwdPathKey, received uint32) (adjOutKey, adjOutPath, []byte, bool, error) {
	s.adjOut.mu.Lock()
	defer s.adjOut.mu.Unlock()
	if s.adjOut.session != s {
		return adjOutKey{}, adjOutPath{}, nil, false, errAdjOutSession
	}
	for key, entry := range s.adjOut.routes {
		if entry.owner.source == nil {
			continue
		}
		if entry.selection.owner.Done() != nil {
			continue
		} // A selected stream has its own exact deciding owner.
		if entry.owner.source.sourceOwner != route.SourceOwner {
			continue
		}
		if entry.owner.connectionID != route.SourceConnectionID {
			continue
		}
		if entry.owner.received != received {
			continue
		}
		if entry.owner.source.addrString != route.SourcePeer {
			continue
		}
		if route.SourceReceipt != 0 {
			if entry.owner.sourceReceipt != route.SourceReceipt {
				continue
			}
		}
		candidate := key.path
		candidate.received = 0
		if candidate != native {
			continue
		}
		if route.MsgID != 0 {
			if entry.owner.message != route.MsgID {
				continue
			}
		}
		wire, err := adjOutWithdrawalNLRI(key, entry)
		path := entry.owner
		path.revision = entry.revision
		return key, path, wire, true, err
	}
	return adjOutKey{}, adjOutPath{}, nil, false, nil
}
