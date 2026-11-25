// Design: docs/architecture/update-building.md -- selected-route final completion.
// Related: reactor_api_forward.go -- the existing destination egress pipeline.
// RFC 4271 Section 9.1.3 -- see rfc/short/rfc4271.md.
package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectRoutes completes a bounded batch against the exact recipient session.
// It holds only that destination's selected stream while its socket can block.
// The global egress dispatch lock is released before any selected final write.
func (a *reactorAPIAdapter) SelectRoutes(ctx context.Context, batch *plugin.SelectedBatch, owner plugin.SelectedOwner) (*rpc.SelectRoutesOutput, error) {
	if batch == nil {
		return nil, errors.New("select-routes: validated batch is missing")
	}
	if a.r.fwdPool == nil {
		return nil, errors.New("select-routes: forwarding workers are unavailable; start the BGP subsystem before reconciling output")
	}
	input := batch.Input()
	ctx, cancel := rpc.SelectedCallContext(ctx, &input)
	defer cancel()
	if err := validateSelectedOwner(owner); err != nil {
		return nil, err
	}
	peer, session, err := a.selectedDestination(batch.Destination(), input.DestinationSession, owner)
	if err != nil {
		return nil, err
	}
	if err := session.selectedMu.LockContext(ctx); err != nil {
		return nil, err
	}
	defer session.selectedMu.Unlock()
	snapshot, kind, err := selectedExportKind(peer, session, owner)
	if err != nil {
		return nil, err
	}
	output := &rpc.SelectRoutesOutput{Results: make([]rpc.SelectedRouteResult, len(input.Routes))}
	for i, route := range input.Routes {
		result, err := a.selectRoute(ctx, peer, session, owner, snapshot, kind, route)
		if err != nil {
			return nil, fmt.Errorf("select-routes: destination %s key %x: %w", input.Destination, route.KeyNLRI, err)
		}
		output.Results[i] = result
	}
	return output, nil
}

func (a *reactorAPIAdapter) selectedDestination(address netip.Addr, receipt uint64, owner plugin.SelectedOwner) (*Peer, *Session, error) {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	// Receipt comparison disambiguates configured peers sharing an address;
	// choosing the first address match would authorize a different TCP session.
	for _, peer := range a.r.peers {
		if peer.Settings().Address != address {
			continue
		}
		session := peer.currentSession()
		if session == nil {
			continue
		}
		if session.initialReplay.Load() != receipt {
			continue
		}
		if peer.State() != PeerStateEstablished {
			return nil, nil, errAdjOutSession
		}
		peers, refused := filterPermittedPeers([]*Peer{peer}, announceOrigin(owner.Sender()))
		if refused != 0 {
			return nil, nil, sendNotPermitted(peerTarget(address), announceOrigin(owner.Sender()))
		}
		return peers[0], session, nil
	}
	return nil, nil, errAdjOutSession
}

func selectedExportKind(peer *Peer, session *Session, owner plugin.SelectedOwner) (*disseminationSnapshot, filterapi.ExportKind, error) {
	snapshot := peer.selectedOwnerSnapshot()
	if snapshot == nil {
		return nil, filterapi.ExportUnknown, errors.New("select-routes: destination has no live dissemination snapshot")
	}
	if snapshot.peer != peer {
		return nil, filterapi.ExportUnknown, errAdjOutSession
	}
	if snapshot.session != session {
		return nil, filterapi.ExportUnknown, errAdjOutSession
	}
	kind, err := snapshot.ExportKind(owner, true)
	return snapshot, kind, err
}

func (a *reactorAPIAdapter) selectRoute(ctx context.Context, peer *Peer, session *Session, owner plugin.SelectedOwner, snapshot *disseminationSnapshot, kind filterapi.ExportKind, route rpc.SelectedRoute) (rpc.SelectedRouteResult, error) {
	fam, key, err := selectedNativeKey(route)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	work := selectedWrite{output: selectedOutput{owner: owner, key: key, revision: route.Revision, kind: kind},
		session: session, destinationSession: session.initialReplay.Load(), snapshot: snapshot, route: route}
	if err := a.selectedCurrent(ctx, &work); err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	// These are open protocol variants, already shape-validated at dispatch.
	switch route.Kind {
	case rpc.SelectedRouteAbsent:
		work.absent = true
		return a.writeSelected(ctx, peer, &work, nil)
	case rpc.SelectedRouteOrigin:
		return a.selectOrigin(ctx, peer, &work, fam, key)
	case rpc.SelectedRouteReceived:
	case rpc.SelectedRouteUnspecified:
		return rpc.SelectedRouteResult{}, errors.New("select-routes: selection kind is unspecified")
	default:
		return rpc.SelectedRouteResult{}, errors.New("select-routes: selection kind is unknown")
	}
	write, err := sourceReceipts.admitWrite(route.SourceReceipt, owner, route.Route.SourcePolicyEpoch)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	defer write.complete()
	work.source = write
	source := write.source()
	if err := selectedOriginalSource(route, &source, fam); err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	if source.info.peer == peer {
		return a.writeSelected(ctx, peer, &work, nil)
	}
	ctx, release := write.writeContext(ctx)
	defer release()
	// Complete provenance before publishing the reconstruction in the shared
	// recent-update cache. Cache readers may observe it as soon as Add returns.
	source.metadata = selectedStaleMetadata(route.StaleLevel)
	var spans []relayAttrSpan
	update, id, _, err := a.buildRelayUpdate([]rpc.StoredRoute{*route.Route}, &source, &spans)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	defer a.r.recentUpdates.Release(id)
	if err := selectedStoredPath(route, fam, key, update.WireUpdate); err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	stage := selectedForwardStage{write: &work}
	defer stage.release(a.r.fwdPool)
	source.info.sender = owner.Sender()
	source.info.selected = &stage
	source.info.flowSpecRedirectRefused = route.FlowSpecRedirectRefused
	// The selecting owner has already published this eligible revision. A
	// second receive/election RPC here could wait on the owner's policy handler
	// which is itself joining this writer. The independent current lookup is
	// its authority; protocol egress still runs through the shared pipeline.
	result := a.stageSelected(update, id, peer, source.info)
	defer result.settle()
	if err := result.outcome(); err != nil {
		if !errors.Is(err, errAllDestinationsSuppressed) {
			return rpc.SelectedRouteResult{}, err
		}
	}
	return a.writeSelected(ctx, peer, &work, stage.items)
}

// stageSelected MUST release the shared dispatch lock before any socket write,
// including error and panic unwinding. The caller retains prepared sections
// until the selected request's final flush has joined.
func (a *reactorAPIAdapter) stageSelected(update *ReceivedUpdate, id uint64, peer *Peer, source forwardSourceInfo) forwardResult {
	a.r.aigp.dispatch.Lock()
	defer a.r.aigp.dispatch.Unlock()
	return a.forwardUpdateSelected(update, id, []*Peer{peer}, source, update.WireUpdate, nil)
}

func (a *reactorAPIAdapter) selectedCurrent(ctx context.Context, work *selectedWrite) error {
	if err := work.output.owner.Current(); err != nil {
		return err
	}
	input := rpc.CurrentSelectionInput{Destination: work.session.settings.Address.String(),
		DestinationSession: work.destinationSession,
		Keys:               []rpc.CurrentSelectionKey{{Family: work.route.Family, KeyNLRI: work.route.KeyNLRI, Revision: work.route.Revision}}}
	answer, err := work.output.owner.CurrentSelection(ctx, input)
	if err != nil {
		return err
	}
	return selectedCurrentRevision(work.route, answer.Results[0], answer.Cut)
}

func selectedNativeKey(route rpc.SelectedRoute) (family.Family, fwdPathKey, error) {
	fam, found := family.LookupFamily(route.Family)
	if !found {
		return fam, fwdPathKey{}, errRelayFamily
	}
	if route.FlowSpecRedirectRefused {
		if route.Kind != rpc.SelectedRouteReceived {
			return fam, fwdPathKey{}, errors.New("select-routes: only a received FlowSpec selection can carry redirect refusal")
		}
		if !ribevents.IsFlowSpec(fam) {
			return fam, fwdPathKey{}, errors.New("select-routes: redirect refusal requires a FlowSpec family")
		}
	}
	split := nlrisplit.Get(fam)
	if split == nil {
		return fam, fwdPathKey{}, errAdjOutProvenance
	}
	count, err := split(route.KeyNLRI, false, nil)
	if err != nil {
		return fam, fwdPathKey{}, err
	}
	if count != 1 {
		return fam, fwdPathKey{}, errors.New("select-routes: native key must name exactly one path")
	}
	var key fwdPathKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if err := fwdPathKeyFor(&key, fam, 0, route.KeyNLRI, false, scratch[:]); err != nil {
		return fam, key, err
	}
	return fam, key, nil
}

// selectedStoredPath validates the bytes the shared reconstruction decoded
// into its retained pool buffer. It never decodes the same hex NLRI again or
// creates a separate per-route byte slice solely to compare the native key.
func selectedStoredPath(route rpc.SelectedRoute, fam family.Family, key fwdPathKey, update *wireu.WireUpdate) error {
	data, err := update.NLRI()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		reach, err := update.MPReach()
		if err != nil {
			return err
		}
		if reach == nil {
			return errors.New("select-routes: reconstructed selection has no advertised native path")
		}
		if reach.Family() != fam {
			return errors.New("select-routes: reconstructed selection changed its family")
		}
		data = reach.NLRIBytes()
	} else if fam != family.IPv4Unicast {
		return errors.New("select-routes: reconstructed selection has the wrong NLRI section")
	}
	count, err := nlrisplit.Get(fam)(data, route.SourceAddPath, nil)
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("select-routes: stored selection must name exactly one path")
	}
	if route.SourceAddPath {
		if binary.BigEndian.Uint32(data[:4]) != route.Route.PathID {
			return errors.New("select-routes: stored source Path Identifier differs from its original framing")
		}
		data = data[4:]
	}
	var actual fwdPathKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if err := fwdPathKeyFor(&actual, fam, 0, data, false, scratch[:]); err != nil {
		return err
	}
	if actual != key {
		return errors.New("select-routes: stored route differs from its native selection key")
	}
	return nil
}

func selectedOriginalSource(route rpc.SelectedRoute, source *relaySource, fam family.Family) error {
	address, err := netip.ParseAddr(route.Route.SourcePeer)
	if err != nil {
		return err
	}
	if address != source.addr {
		return errors.New("select-routes: source receipt belongs to a different original peer")
	}
	encoding := bgpctx.Registry.Get(source.ctxID)
	if encoding == nil {
		return errors.New("select-routes: original source encoding is unavailable")
	}
	if encoding.AddPath(fam) != route.SourceAddPath {
		return errors.New("select-routes: source ADD-PATH presence differs from the issued receipt")
	}
	return nil
}

// These fixed metadata values are shared read-only, like relayRebuiltMeta.
// Callers MUST NOT mutate them or lend them to a mutating filter without a copy.
var (
	selectedFreshMeta = map[string]any{filterapi.MetaKeyRebuilt: true, metaStaleKey: uint8(0)}
	selectedStaleMeta = map[string]any{filterapi.MetaKeyRebuilt: true, metaStaleKey: uint8(1)}
	selectedLLGRMeta  = map[string]any{filterapi.MetaKeyRebuilt: true, metaStaleKey: uint8(2)}
)

func selectedStaleMetadata(level rpc.SelectedStaleLevel) map[string]any {
	// The selected enum has already been validated. Values below are the GR
	// filter's established route-metadata vocabulary, not client metadata.
	switch level {
	case rpc.SelectedFresh:
		return selectedFreshMeta
	case rpc.SelectedStale:
		return selectedStaleMeta
	case rpc.SelectedLLGR:
		return selectedLLGRMeta
	case rpc.SelectedStaleUnspecified:
		return nil
	default:
		return nil
	}
}

// selectedForwardStage owns at most one native selection's prepared sections.
// The source cache retain and per-peer pool/provenance handles remain owned
// until release after the synchronous final writer joins. No worker starts.
type selectedForwardStage struct {
	write *selectedWrite
	items []fwdItem
}

func (stage *selectedForwardStage) release(pool *fwdPool) {
	for i := range stage.items {
		if stage.items[i].done != nil {
			stage.items[i].done()
		}
		pool.releaseItem(&stage.items[i])
	}
	stage.items = nil
}
