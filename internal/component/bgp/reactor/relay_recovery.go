// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package reactor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errRecoveryNotWritten = errors.New("source-DOWN recovery operation was not written")

// recoveryAdmission owns every route and post-policy section from one lookup.
// Producers MUST stage all items before dispatch; the destination worker MUST
// fence the entire operation once under writeMu and complete it once at release.
// No IPC or RIB lookup runs under writeMu. This cold receipt is bounded by the
// lookup response and its policy output, not another persistent route inventory.
type recoveryAdmission struct {
	peer     *Peer
	session  *Session
	sequence uint64
	nlri     []byte
	items    []fwdItem
	ctx      context.Context
	retry    bool
	written  bool
	err      error
	done     chan bool
	sources  map[*Peer]recoverySourceReceipt
}

func (admission *recoveryAdmission) current(session *Session) bool {
	if admission.session != session {
		admission.retry = true
		return false
	}
	if session.tearingDown.Load() {
		admission.retry = true
		return false
	}
	if admission.peer.sentUpdateSequence.Load() != admission.sequence {
		admission.retry = true
		return false
	}
	return true
}

type recoverySourceReceipt struct {
	generation uint64
	cut        receivePublicationCut
}

// recoverySources binds selected bytes to the sessions present before lookup.
// The cold map is bounded by configured peers and covers both lookup transports.
func (a *reactorAPIAdapter) recoverySources() map[*Peer]recoverySourceReceipt {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	sources := make(map[*Peer]recoverySourceReceipt, len(a.r.peers))
	for _, peer := range a.r.peers {
		sources[peer] = recoverySourceReceipt{generation: peer.forwardGeneration.Load(),
			cut: peer.receiveCut()}
	}
	return sources
}

// drainRecoverySources closes the gap between a fast-path wire decision and
// publication of its received event. Only afterward may the process-level
// applied-event barrier make a cold RIB lookup causally current.
func drainRecoverySources(ctx context.Context, sources map[*Peer]recoverySourceReceipt) error {
	for _, source := range sources {
		if err := source.cut.wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// recoverySnapshot takes the sequence only after every preceding writer has
// enqueued its sent event. Taking an atomic value without writeMu could observe
// a writer between incrementing the sequence and enqueueing that event.
func recoverySnapshot(peer *Peer) (*Session, uint64) {
	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()
	if session == nil {
		return nil, 0
	}
	session.writeMu.Lock()
	sequence := peer.sentUpdateSequence.Load()
	session.writeMu.Unlock()
	return session, sequence
}

// recoverNLRIBatch keeps the parser's native identity. Every received path of a
// departed source is gone; the sent inventory, not ingress IDs, owns egress IDs.
// RFC 4271 Section 6: "The local system recalculates its best routes for the
// destinations of the routes marked as invalid."
// Recovery preserves the departed source's native destination identities.
func (a *reactorAPIAdapter) recoverNLRIBatch(ctx context.Context, batch bgptypes.NLRIBatch, destinations []*Peer, sender plugin.Sender) (result error) {
	// A hard ownership failure cannot leave known-invalid routes installed.
	// Capture the affected sessions for failures before admission, then refresh
	// ownership on each attempt. An old attempt MUST NOT retire a reconnect
	// unless recovery has itself started an attempt on that replacement.
	sessions := make([]*Session, len(destinations))
	for i, destination := range destinations {
		sessions[i] = destination.currentSession()
	}
	defer func() {
		if result == nil {
			return
		}
		for i, session := range sessions {
			if session == nil {
				continue
			}
			if destinations[i].currentSession() != session {
				continue
			}
			// AutomaticStop uses the existing safe NOTIFICATION/close path.
			// This is not a graceful-restart TCP drop: obsolete output MUST
			// disappear from the remote rather than be retained as stale.
			_ = session.teardownAutomatic(message.NotifyCeaseOutOfResources, "")
		}
	}()
	if a.r.api == nil || a.r.fwdPool == nil {
		return errServerNotReady
	}
	command := a.r.api.Dispatcher().Registry().Lookup("request bgp rib recovery")
	if command == nil || command.Process == nil {
		return fmt.Errorf("source-DOWN recovery requires the BGP RIB command owner")
	}
	if len(batch.NLRIs) == 0 {
		return nil
	}
	request := ribevents.RecoveryRequest{Source: batch.RecoverySource, Family: batch.Family,
		Cut: batch.RecoveryCut, NLRIs: make([][]byte, 0, len(batch.NLRIs))}
	for i, route := range batch.NLRIs {
		// RFC 7911 Section 3: "In order to carry the Path Identifier in an
		// UPDATE message, the NLRI encoding MUST be extended by prepending
		// the Path Identifier field, which is of four octets."
		addPath := false
		if framing, ok := route.(nlri.AddPathAware); ok {
			addPath = framing.HasAddPath()
		}
		_, prefixOnly := route.(*nlri.INET)
		prefixOnly = batch.Family.SAFI == family.SAFIMPLSLabel && prefixOnly
		if i == 0 {
			request.AddPath, request.PrefixOnly = addPath, prefixOnly
		} else if request.AddPath != addPath || request.PrefixOnly != prefixOnly {
			return fmt.Errorf("source-DOWN recovery batch has mixed NLRI framing")
		}
		raw := make([]byte, nlri.LenWithContext(route, addPath))
		nlri.WriteNLRI(route, raw, 0, addPath)
		request.NLRIs = append(request.NLRIs, raw)
	}
	for i, destination := range destinations {
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			if destination.State() != PeerStateEstablished {
				break
			}
			nc := destination.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				break
			}
			session, sequence := recoverySnapshot(destination)
			if session == nil {
				break
			}
			sessions[i] = session // Own this attempt before its barrier or lookup can fail.
			sources := a.recoverySources()
			if err := drainRecoverySources(ctx, sources); err != nil {
				return err
			}
			// No session or RIB lock is held across this FIFO barrier. A
			// receipt MUST prove application, not merely attempted delivery.
			if err := command.Process.DrainEventsApplied(ctx); err != nil {
				return err
			}
			request.Destination = destination.Settings().Address
			request.SentAddPath = destination.addPathFor(batch.Family)
			// RFC 4271 Section 6: only the RIB chooses withdrawal or replacement.
			retry, err := a.relayRecovery(ctx, request, destination, session, sequence, sender, sources)
			if err != nil {
				return err
			}
			if !retry {
				break
			}
		}
		sessions[i] = nil // This destination's obligation completed or its session retired.
	}
	return nil
}

// relayRecovery keeps election in the RIB and export on the ordinary forward
// rail. No command or delivery barrier is invoked under destination writeMu.
// RFC 4271 Section 6: "Before the invalid routes are deleted from the system,
// it advertises, to its peers, either withdraws for the routes marked as invalid,
// or the new best routes before the invalid routes are deleted from the system."
// Selection and export therefore complete before source ownership is released.
func (a *reactorAPIAdapter) relayRecovery(ctx context.Context, request ribevents.RecoveryRequest, destination *Peer, session *Session, sequence uint64, sender plugin.Sender, sources map[*Peer]recoverySourceReceipt) (bool, error) {
	var routes []ribevents.RecoveryRoute
	var err error
	if owner := ribevents.RecoveryProvider(); owner != nil {
		// RFC 4271 Section 6: use the existing selection producer in-process.
		routes, err = owner.Lookup(request)
	} else {
		if a.r.api == nil {
			return false, errServerNotReady
		}
		identities := make([]string, len(request.NLRIs))
		for i, raw := range request.NLRIs {
			identities[i] = hex.EncodeToString(raw)
		}
		args := []string{request.Source.String(), request.Destination.String(), request.Family.String(),
			strings.Join(identities, ","), strconv.FormatBool(request.AddPath),
			strconv.FormatBool(request.PrefixOnly), strconv.FormatBool(request.SentAddPath), strconv.FormatUint(request.Cut, 10)}
		commandContext := &pluginserver.CommandContext{Server: a.r.api, RequestContext: ctx}
		var response *plugin.Response
		response, err = a.r.api.Dispatcher().ForwardToPlugin(commandContext, "request bgp rib recovery", args, "*")
		if err == nil {
			switch {
			case response == nil:
				err = fmt.Errorf("source-DOWN recovery returned no RIB answer")
			case response.Error != "":
				err = errors.New(response.Error)
			default:
				data, ok := response.Data.(plugin.RawJSON)
				if ok {
					err = json.Unmarshal([]byte(data), &routes)
				} else {
					err = fmt.Errorf("source-DOWN recovery returned an invalid RIB document")
				}
			}
		}
	}
	if err != nil {
		return false, err
	}
	currentSession, currentSequence := recoverySnapshot(destination)
	if currentSession != session || currentSequence != sequence {
		return true, nil
	}
	admission := &recoveryAdmission{peer: destination, session: session, sequence: sequence,
		ctx: ctx, done: make(chan bool, 1), sources: sources}
	// On every pre-dispatch exit this producer MUST release staged references.
	// After dispatch, releaseItem owns them and the single completion instead.
	dispatched := false
	defer func() {
		if !dispatched {
			a.r.fwdPool.releaseRecoveryItems(admission)
		}
	}()
	for _, recovery := range routes {
		admission.nlri = recovery.SentNLRI
		stored := rpc.StoredRoute{SourcePeer: recovery.Source.String(), Family: request.Family.String(),
			NLRIHex: hex.EncodeToString(recovery.NLRI), AttrHex: hex.EncodeToString(recovery.Attributes),
			NextHopHex: hex.EncodeToString(recovery.NextHop), MsgID: recovery.MessageID,
			NLRIFraming: rpc.NLRIFramingSourceWire, Withdraw: recovery.Withdraw}
		// RFC 4271 Section 6: replacement and withdrawal have disjoint owners.
		if err := a.queueRecovery(destination, &stored, request.Family, admission, sources, sender); err != nil {
			if errors.Is(err, errRelayNoSource) {
				return true, nil
			}
			if errors.Is(err, errForwardNoSource) {
				return true, nil
			}
			return false, err
		}
	}
	if len(admission.items) == 0 {
		return false, nil
	}
	// One queue entry prevents siblings from invalidating each other's receipt,
	// and sends all advertised ADD-PATH IDs without restarting the RIB scan.
	dispatched = true
	if !a.r.fwdPool.dispatchOverflow(fwdKey{peerAddr: destination.Settings().PeerKey()},
		fwdItem{peer: destination, recovery: admission}) {
		return false, errForwardPoolStopped
	}
	select {
	case retry := <-admission.done:
		return retry, admission.err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// queueRecovery admits a replacement on its surviving source's ordinary rail,
// or queues an already destination-framed withdrawal behind earlier forwards.
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message."
// Queue order retains earlier forwards before this replacement or withdrawal.
func (a *reactorAPIAdapter) queueRecovery(destination *Peer, route *rpc.StoredRoute, fam family.Family, admission *recoveryAdmission, sources map[*Peer]recoverySourceReceipt, sender plugin.Sender) error {
	if route.Withdraw {
		return a.queueRecoveryWithdrawal(destination, route, fam, admission)
	}
	sourceAddr, err := netip.ParseAddr(route.SourcePeer)
	if err != nil {
		return err
	}
	source := a.resolveRelaySource(sourceAddr)
	if !source.ok {
		return errRelayNoSource
	}
	receipt, present := sources[source.info.peer]
	if !present {
		return errRelayNoSource
	}
	if receipt.generation != source.generation || !receipt.cut.current(source.info.peer) {
		return errRelayNoSource
	}
	var spans []relayAttrSpan
	update, id, _, err := a.buildRelayUpdate([]rpc.StoredRoute{*route}, source, &spans)
	if err != nil {
		return err
	}
	defer a.r.recentUpdates.Release(id)
	source.info.sender, source.info.recovery = sender, admission
	err = a.forwardUpdateCore(update, id, []*Peer{destination}, source.info)
	if errors.Is(err, errAllDestinationsSuppressed) {
		withdraw := rpc.StoredRoute{Family: route.Family, NLRIHex: hex.EncodeToString(admission.nlri), Withdraw: true}
		// RFC 4271 Section 9.1.3: an export-excluded advertised route is withdrawn.
		return a.queueRecoveryWithdrawal(destination, &withdraw, fam, admission)
	}
	return err
}

// queueRecoveryWithdrawal uses destination framing, including the advertised
// ADD-PATH identifier. The session/sequence receipt captures its obsolete owner.
// RFC 7911 Section 3: "The combination of the address prefix and the Path
// Identifier can be used to identify a route advertised by a BGP speaker.".
func (a *reactorAPIAdapter) queueRecoveryWithdrawal(destination *Peer, route *rpc.StoredRoute, fam family.Family, admission *recoveryAdmission) error {
	source := relaySource{addr: destination.Settings().Address, ctxID: destination.sendContextID()}
	update, id, _, err := a.buildRelayWithdrawal(route, source, fam, 0)
	if err != nil {
		return err
	}
	// The staged item owns the cache reference until the operation is released.
	admission.items = append(admission.items, fwdItem{peer: destination,
		session: admission.session, authority: adjOutRecovery,
		rawBodies: [][]byte{update.WireUpdate.Payload()},
		done:      func() { a.r.recentUpdates.Release(id) }})
	return nil
}
