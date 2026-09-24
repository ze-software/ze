// Design: docs/guide/rpki.md -- retained-route revalidation across config transactions
package rpki

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

func sameRPKIPolicy(a, b *rpkiConfig) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ValidationTimeout == b.ValidationTimeout && a.ASPAValidation == b.ASPAValidation &&
		a.ASPAInvalidAction == b.ASPAInvalidAction && a.ASPAUnknownAction == b.ASPAUnknownAction &&
		a.OriginInvalidAction == b.OriginInvalidAction && a.OriginNotFoundAction == b.OriginNotFoundAction &&
		maps.EqualFunc(a.PeerActions, b.PeerActions, func(a, b peerActionSet) bool {
			return a.OriginInvalid == b.OriginInvalid && a.OriginNotFound == b.OriginNotFound &&
				a.ASPAInvalid == b.ASPAInvalid && a.ASPAUnknown == b.ASPAUnknown &&
				a.ASPAMode == b.ASPAMode && a.BlackholeExempt == b.BlackholeExempt &&
				slices.Equal(a.BlackholeCommunities, b.BlackholeCommunities)
		})
}

// replaceConfig stops the old transport before installing another generation.
// Policy changes fence old queued decisions and re-evaluate retained received
// attributes while Adj-RIB-In holds those routes pending. Changing only the
// transport or its PKI identity does not withdraw and reannounce the whole RIB.
func (rp *rPKIPlugin) replaceConfig(previous, next *rpkiConfig) error {
	if rp.dataLease != nil {
		rp.dataLease.expire(time.Now())
	}
	wasActive := previous != nil && len(previous.CacheServers) != 0
	active := next != nil && len(next.CacheServers) != 0
	if wasActive && active && sameRPKIPolicy(previous, next) {
		rp.startSessions(next)
		return nil
	}
	if !wasActive && !active {
		rp.startSessions(next)
		return nil
	}
	rp.stopSessions()
	// Producers hold validationMu while taking verdicts and enqueueing them.
	// Acquiring it first lets the worker drain backpressure before dispatchMu
	// fences its last RPC. No stale policy decision crosses this transition.
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	rp.dispatchMu.Lock()
	defer rp.dispatchMu.Unlock()
	rp.validationGeneration.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if !active {
		if _, _, err := rp.plugin.DispatchCommandArgs(ctx, "request bgp adj-rib-in disable-validation", nil, ""); err != nil {
			return fmt.Errorf("rpki: disable validation: %w", err)
		}
		rp.startSessions(next)
		rp.aspaTracker.clear()
		rp.originTracker.clear()
		return nil
	}
	timeout := uint16(30)
	if next.ValidationTimeout != 0 {
		timeout = next.ValidationTimeout
	}
	args := []string{"refresh", "timeout", strconv.FormatUint(uint64(timeout), 10)}
	_, data, err := rp.plugin.DispatchCommandArgs(ctx, "request bgp adj-rib-in enable-validation", args, "")
	if err != nil {
		return fmt.Errorf("rpki: enable validation: %w", err)
	}
	rp.startSessions(next)
	rp.aspaTracker.clear()
	rp.originTracker.clear()
	return rp.revalidateConfigSnapshot(ctx, data)
}

type configRouteSnapshot struct {
	Peer       string `json:"peer"`
	PeerName   string `json:"peer-name"`
	PeerGroup  string `json:"peer-group"`
	PeerAS     uint32 `json:"peer-as"`
	LocalAS    uint32 `json:"local-as"`
	Family     string `json:"family"`
	Prefix     string `json:"prefix"`
	Attributes string `json:"attr-hex"`
	PathID     uint32 `json:"path-id"`
	MsgID      uint64 `json:"msg-id"`
}

// revalidateConfigSnapshot is a config-time replay of authoritative received
// data, not of prior acceptance decisions or outbound modified attributes.
// MsgID remains the received UPDATE generation; Adj-RIB-In rejects a snapshot
// overtaken by a newer UPDATE or its already buffered decision.
func (rp *rPKIPlugin) revalidateConfigSnapshot(ctx context.Context, data json.RawMessage) error {
	var snapshot struct {
		Routes []configRouteSnapshot `json:"routes"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("rpki: decode retained route snapshot: %w", err)
	}
	batch := make([]validationRequest, 0, maxBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		_, err := rp.plugin.BatchValidate(ctx, rp.buildDecisions(batch))
		batch = batch[:0]
		return err
	}
	v4, v6 := rp.cache.Count()
	updates := make(map[rpkiUpdateKey]*rpkiUpdateResults)
	for _, route := range snapshot.Routes {
		fam, ok := family.LookupFamily(route.Family)
		if !ok {
			return fmt.Errorf("rpki: retained route has unknown family %q", route.Family)
		}
		raw, err := hex.DecodeString(route.Attributes)
		if err != nil || len(raw) == 0 {
			return errors.New("rpki: retained route has invalid raw attributes")
		}
		attrs := attribute.NewAttributesWire(raw, bgpctx.APIContextID)
		path := rpkiASPathFromWire(attrs)
		originAS := rpkiOriginASFromASPath(path, route.LocalAS)
		origin := rp.cache.Validate(route.Prefix, originAS)
		aspa := aspaStateNone
		var normalized []uint32
		mode := rp.aspaModeForPeer(route.Peer, route.PeerName, route.PeerGroup)
		if rp.aspaEnabled.Load() && aspaAppliesTo(fam) && route.PeerAS != route.LocalAS {
			aspa = ASPAInvalid
			if path != nil {
				aspa, normalized = aspaStateForPath(rp.aspaCache, path.Segments, mode)
			}
		}
		blackhole := rp.carriesAgreedBlackhole(route.Peer, route.PeerGroup, attrs)
		key := routeKey{peerAddr: route.Peer, family: route.Family, prefix: route.Prefix, pathID: route.PathID}
		tracked := originRoute{
			peerGroup: route.PeerGroup, peerName: route.PeerName, peerASN: route.PeerAS,
			originAS: originAS, state: origin, aspaState: aspa, blackhole: blackhole,
			msgID: route.MsgID, unavailable: v4+v6 == 0,
		}
		rp.originTracker.Track(key, tracked)
		collectRPKIUpdate(updates, key, tracked)
		if aspa != aspaStateNone {
			rp.aspaTracker.Track(trackedRoute{
				key: key, peerName: route.PeerName, peerGroup: route.PeerGroup, peerASN: route.PeerAS,
				msgID: route.MsgID, path: normalized, aspaState: aspa, mode: mode,
			})
		}
		batch = append(batch, validationRequest{
			peerAddr: route.Peer, peerGroup: route.PeerGroup, family: route.Family, prefix: route.Prefix,
			pathID: route.PathID, msgID: route.MsgID, state: origin, aspaState: aspa,
			originAS: originAS, blackhole: blackhole, generation: rp.validationGeneration.Load(),
		})
		if len(batch) == maxBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	// Startup can miss the first live UPDATE. Publish its retained verdicts
	// together: the decorator consumes only one secondary per peer and MsgID.
	rp.emitRPKIUpdates(updates)
	return nil
}
