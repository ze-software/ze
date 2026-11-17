// Design: docs/architecture/update-building.md -- joined selected final writes.
// Related: selected_output.go -- only current Adj-RIB-Out slots are indexed.
// RFC 4271 Section 9.1.3 -- see rfc/short/rfc4271.md.
package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"math"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (a *reactorAPIAdapter) writeSelected(ctx context.Context, peer *Peer, work *selectedWrite, items []fwdItem) (rpc.SelectedRouteResult, error) {
	session := work.session
	owner := fwdItem{peer: peer, session: session, authority: adjOutSelected, selected: work}
	var counts updateSendCounts
	err := session.sendUpdateCounted(ctx, &message.Update{}, &counts, false, &owner,
		func(s *Session, _ *message.Update) (result error) {
			// A late failure after a prior body buffered bytes cannot hand a
			// speculative frontier to another writer. Session retirement is
			// already the shared final writer's rollback on partial output.
			defer func() {
				if result != nil {
					if s.writePending {
						s.retireWrite(result)
					}
				}
				s.sentMeta = nil
				s.sentSourcePeerStr = ""
				s.sentSourceMessageID = 0
				s.sentAIGPOrigin = sendOrigin{}
				s.sentAIGPRevision = 0
				s.sentAIGPCostWithheld = false
			}()
			if err := work.current(s); err != nil {
				return err
			}
			if err := a.selectedCurrent(ctx, work); err != nil {
				return err
			}
			if s.selectedAttempt == math.MaxUint64 {
				return errors.New("select-routes: destination write identity exhausted; restart the peer session before replay")
			}
			s.selectedAttempt++
			work.output.attempt = s.selectedAttempt
			if !work.absent {
				facts := peer.forwardFacts()
				if facts == nil {
					return errAdjOutSession
				}
				allowed := facts.exportPolicy.AcceptsExportAtEnd(facts.isEBGP, work.output.kind)
				if allowed {
					for i := range items {
						if err := s.writeSelectedItem(&items[i]); err != nil {
							return err
						}
					}
				}
			}
			s.sentForward = &owner
			return s.retireSelected(work)
		})
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	outcome := rpc.SelectedSuppressed
	if work.absent {
		outcome = rpc.SelectedAbsent
	} else if counts.announcedRoutes != 0 {
		outcome = rpc.SelectedWritten
	}
	return rpc.SelectedRouteResult{Family: work.route.Family, KeyNLRI: work.route.KeyNLRI,
		Revision: work.route.Revision, CurrentRevision: work.route.Revision, Outcome: outcome}, nil
}

// writeSelectedItem consumes the shared egress materialization synchronously.
// The caller MUST hold writeMu and retain every cache/pool handle until flush.
func (s *Session) writeSelectedItem(item *fwdItem) error {
	s.sentForward = item
	s.sentMeta = item.meta
	s.sentSourcePeerStr = item.sourcePeerStr
	s.sentSourceMessageID = item.sourceMessageID
	s.sentAIGPOrigin = item.aigpOrigin
	s.sentAIGPRevision = item.aigpRevision
	s.sentAIGPCostWithheld = item.aigpCostWithheld
	item.writeBody = 0
	if err := s.writePolicyRemovals(item); err != nil {
		return err
	}
	for _, body := range item.rawBodies {
		if err := s.writeRawUpdateBody(body); err != nil {
			return err
		}
		item.writeBody++
	}
	for _, update := range item.updates {
		// Local selected bytes came from the shared local builder and have not
		// passed export policy. Received items were materialized by forwarding.
		write := (*Session).writeUpdatePreFiltered
		if item.originated {
			write = (*Session).writeUpdate
		}
		if err := write(s, update); err != nil {
			return err
		}
		item.writeBody++
	}
	return nil
}

// retireSelected removes exact old output slots after the replacement bodies
// were admitted. A non-ADD-PATH replacement overwrites its old slot directly;
// an ADD-PATH replacement or policy exclusion retires every old identifier.
// The loop is bounded by this one key's actual current output population.
func (s *Session) retireSelected(work *selectedWrite) error {
	if s.adjOut == nil {
		return nil
	}
	s.adjOut.mu.Lock()
	slots, found := s.adjOut.selectedPaths[work.output.key]
	bound := len(slots.rest) + 1
	s.adjOut.mu.Unlock()
	if !found {
		return nil
	}
	defer func() { work.retirement = false }()
	for range bound {
		key, path, raw, found, err := s.selectedRetirement(work)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		work.retirement, work.retireKey = true, key
		item := fwdItem{peer: s.sentForward.peer, session: s, authority: adjOutSelected, selected: work,
			receivedPeer: path.source, receivedGeneration: path.generation,
			sourceReceipt: path.sourceReceipt, sourceMessageID: path.message, sourcePolicyEpoch: path.policyEpoch}
		if path.source != nil {
			item.sourcePeerStr = path.source.addrString
		}
		s.sentForward = &item
		s.sentSourcePeerStr, s.sentSourceMessageID = item.sourcePeerStr, item.sourceMessageID
		if err := s.writeSelectedWithdrawal(key.path.family, raw); err != nil {
			return err
		}
		work.retirement = false
	}
	_, _, _, unresolved, err := s.selectedRetirement(work)
	if err != nil {
		return err
	}
	if unresolved {
		return errors.New("select-routes: final writer did not retire the previous selected output; reconcile the destination session")
	}
	return nil
}

func (s *Session) selectedRetirement(work *selectedWrite) (adjOutKey, adjOutPath, []byte, bool, error) {
	s.adjOut.mu.Lock()
	defer s.adjOut.mu.Unlock()
	slots, found := s.adjOut.selectedPaths[work.output.key]
	if !found {
		return adjOutKey{}, adjOutPath{}, nil, false, nil
	}
	for i := -1; i < len(slots.rest); i++ {
		key := slots.first
		if i >= 0 {
			key = slots.rest[i]
		}
		entry := s.adjOut.routes[key]
		if entry == nil {
			return key, adjOutPath{}, nil, false, errAdjOutProvenance
		}
		if !work.owns(entry.selection) {
			continue
		}
		if work.cleanup != nil {
			if work.cleanup.source != nil {
				if entry.owner.source != work.cleanup.source {
					continue
				}
			}
		}
		if entry.selection.revision > work.output.revision {
			return key, adjOutPath{}, nil, false, errSelectedRevision
		}
		if entry.selection.attempt == work.output.attempt {
			continue
		}
		raw, err := adjOutWithdrawalNLRI(key, entry)
		return key, entry.owner, raw, true, err
	}
	return adjOutKey{}, adjOutPath{}, nil, false, nil
}

func (s *Session) writeSelectedWithdrawal(fam family.Family, raw []byte) error {
	if fam == family.IPv4Unicast {
		return s.writeUpdatePreFiltered(&message.Update{WithdrawnRoutes: raw})
	}
	size := attrHeaderLen(3+len(raw)) + 3 + len(raw)
	buffer := getReadBuf(size > message.MaxMsgLen-message.HeaderLen)
	if buffer.Buf == nil {
		return errRelayBufferPool
	}
	defer returnReadBuffer(buffer)
	if size > len(buffer.Buf) {
		return errRelayTooLarge
	}
	off := writeAttrHeader(buffer.Buf, 0, byte(attribute.FlagOptional), attribute.AttrMPUnreachNLRI, 3+len(raw))
	binary.BigEndian.PutUint16(buffer.Buf[off:off+2], uint16(fam.AFI))
	buffer.Buf[off+2] = byte(fam.SAFI)
	copy(buffer.Buf[off+3:], raw)
	return s.writeUpdatePreFiltered(&message.Update{PathAttributes: buffer.Buf[:size]})
}
