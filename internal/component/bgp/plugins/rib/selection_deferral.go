// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 4724 Section 4.1 per-family route selection deferral
// RFC: rfc/short/rfc4724.md -- Restarting Speaker selection deferral and Selection_Deferral_Timer
// Related: config_tx.go -- parses bgp update-delay max-delay and configured static peers
// Related: rib_structured.go -- defers Phase 3 best-path selection for held families
// Related: rib_validation.go -- routes negotiated and eor JSON events and defers text-path selection

package rib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const selectionDeferralMaxSeconds = 3600

// selectionDeferral defers BGP best-path route selection per address family
// when the Restarting Speaker configures `bgp update-delay max-delay`.
//
// RFC 4724 Section 4.1: "However, it MUST defer route selection for an address
// family until it either (a) receives the End-of-RIB marker from all its peers
// (excluding the ones with the "Restart State" bit set in the received
// capability and excluding the ones that do not advertise the graceful restart
// capability) or (b) the Selection_Deferral_Timer referred to below has
// expired."
//
// RFC 4724 Section 4.1: "To put an upper bound on the amount of time a router
// defers its route selection, an implementation MUST support a (configurable)
// timer that imposes this upper bound."
//
// Safe for concurrent use. When not armed (max-delay == 0), isDeferred takes a
// single atomic load and returns false immediately.
type selectionDeferral struct {
	active atomic.Bool

	mu               sync.Mutex
	armed            bool
	releasedAll      bool
	maxDelay         time.Duration
	timer            clock.Timer
	expected         map[netip.Addr]struct{}
	observed         map[netip.Addr]bool
	excluded         map[netip.Addr]bool
	peerOwed         map[netip.Addr]map[family.Family]struct{}
	knownFamilies    map[family.Family]struct{}
	releasedFamilies map[family.Family]bool
	dirtyFamilies    map[family.Family]bool
	onRelease        func(family.Family)
	onComplete       func()
}

// activeLocked reports whether the deferral hold is armed and not yet released.
// Caller MUST hold d.mu.
func (d *selectionDeferral) activeLocked() bool {
	if !d.armed {
		return false
	}
	return !d.releasedAll
}

// arm starts the per-family selection deferral hold bounded by maxDelay
// (Selection_Deferral_Timer). Returns false when maxDelay <= 0 or already armed.
func (d *selectionDeferral) arm(clk clock.Clock, maxDelay time.Duration, expectedPeers []netip.Addr, onRelease func(family.Family), onComplete func()) bool {
	if maxDelay <= 0 {
		return false
	}
	if clk == nil {
		clk = clock.RealClock{}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.armed {
		return false
	}
	if d.releasedAll {
		return false
	}

	d.armed = true
	d.maxDelay = maxDelay
	d.onRelease = onRelease
	d.onComplete = onComplete
	d.expected = make(map[netip.Addr]struct{}, len(expectedPeers))
	for _, peer := range expectedPeers {
		if peer.IsValid() {
			d.expected[peer] = struct{}{}
		}
	}
	d.observed = make(map[netip.Addr]bool, len(d.expected))
	d.excluded = make(map[netip.Addr]bool, len(d.expected))
	d.peerOwed = make(map[netip.Addr]map[family.Family]struct{}, len(d.expected))
	d.knownFamilies = make(map[family.Family]struct{})
	d.releasedFamilies = make(map[family.Family]bool)
	d.dirtyFamilies = make(map[family.Family]bool)
	d.active.Store(true)

	// RFC 4724 Section 4.1: "To put an upper bound on the amount of time a router
	// defers its route selection, an implementation MUST support a (configurable)
	// timer that imposes this upper bound."
	d.timer = clk.AfterFunc(maxDelay, d.releaseAll)
	return true
}

// isDeferred reports whether route selection for fam is currently deferred.
// When deferred, fam is recorded as dirty so reselection runs upon release.
func (d *selectionDeferral) isDeferred(fam family.Family) bool {
	if !d.active.Load() {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.activeLocked() {
		return false
	}
	if d.releasedFamilies[fam] {
		return false
	}
	d.knownFamilies[fam] = struct{}{}
	d.dirtyFamilies[fam] = true
	return true
}

// isHolding reports whether any family is still under selection deferral.
func (d *selectionDeferral) isHolding() bool {
	return d.active.Load()
}

// onPeerNegotiated records an expected peer's Graceful Restart capability and
// negotiated families, excluding peers without GR or with the Restart State
// (R) bit set per RFC 4724 Section 4.1.
func (d *selectionDeferral) onPeerNegotiated(peer netip.Addr, hasGR, restartState bool, negotiatedFamilies []family.Family) {
	if !d.active.Load() {
		return
	}

	d.mu.Lock()
	if !d.activeLocked() {
		d.mu.Unlock()
		return
	}
	if _, ok := d.expected[peer]; !ok {
		d.mu.Unlock()
		return
	}

	d.observed[peer] = true

	// RFC 4724 Section 4.1: "excluding the ones with the "Restart State" bit set
	// in the received capability and excluding the ones that do not advertise the
	// graceful restart capability"
	if !hasGR || restartState {
		d.excluded[peer] = true
		delete(d.peerOwed, peer)
	} else {
		delete(d.excluded, peer)
		owed := make(map[family.Family]struct{}, len(negotiatedFamilies))
		for _, fam := range negotiatedFamilies {
			d.knownFamilies[fam] = struct{}{}
			if !d.releasedFamilies[fam] {
				owed[fam] = struct{}{}
			}
		}
		d.peerOwed[peer] = owed
	}

	toRun, cb, doneCB := d.evaluateReleasesLocked()
	d.mu.Unlock()

	runReleasedFamilies(cb, toRun)
	if doneCB != nil {
		doneCB()
	}
}

// onPeerEOR records an End-of-RIB marker for (peer, fam) and releases fam as
// soon as all non-excluded expected peers have completed End-of-RIB for fam.
func (d *selectionDeferral) onPeerEOR(peer netip.Addr, fam family.Family) {
	if !d.active.Load() {
		return
	}

	d.mu.Lock()
	if !d.activeLocked() {
		d.mu.Unlock()
		return
	}
	owed, exists := d.peerOwed[peer]
	if !exists {
		d.mu.Unlock()
		return
	}
	if _, isOwed := owed[fam]; !isOwed {
		// Duplicate or unnegotiated family marker does not advance convergence.
		d.mu.Unlock()
		return
	}
	delete(owed, fam)
	d.knownFamilies[fam] = struct{}{}

	toRun, cb, doneCB := d.evaluateReleasesLocked()
	d.mu.Unlock()

	runReleasedFamilies(cb, toRun)
	if doneCB != nil {
		doneCB()
	}
}

// onPeerDown removes a disconnected peer from the observed set so a peer that
// established and dropped before End-of-RIB no longer counts as converged.
func (d *selectionDeferral) onPeerDown(peer netip.Addr) {
	if !d.active.Load() {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.activeLocked() {
		return
	}
	delete(d.observed, peer)
	delete(d.excluded, peer)
	delete(d.peerOwed, peer)
}

// evaluateReleasesLocked checks whether every expected peer has been observed
// and releases each known family for which no expected peer still owes
// End-of-RIB (including route-less families that had zero dirty routes).
// Caller MUST hold d.mu.
func (d *selectionDeferral) evaluateReleasesLocked() ([]family.Family, func(family.Family), func()) {
	if len(d.expected) == 0 {
		return nil, nil, nil
	}
	if len(d.observed) < len(d.expected) {
		return nil, nil, nil
	}

	remainingOwed := 0
	for _, owed := range d.peerOwed {
		remainingOwed += len(owed)
	}

	var toRun []family.Family
	for fam := range d.knownFamilies {
		if d.releasedFamilies[fam] {
			continue
		}
		if d.familyStillOwedLocked(fam) {
			continue
		}
		d.releasedFamilies[fam] = true
		if d.dirtyFamilies[fam] {
			delete(d.dirtyFamilies, fam)
			toRun = append(toRun, fam)
		}
	}

	var doneCB func()
	if remainingOwed == 0 {
		doneCB = d.onComplete
		d.finishLocked()
	}

	return toRun, d.onRelease, doneCB
}

// familyStillOwedLocked reports whether any expected peer still owes End-of-RIB
// for fam. Caller MUST hold d.mu.
func (d *selectionDeferral) familyStillOwedLocked(fam family.Family) bool {
	for _, owed := range d.peerOwed {
		if _, pending := owed[fam]; pending {
			return true
		}
	}
	return false
}

// releaseAll ends deferral for every remaining family when
// Selection_Deferral_Timer expires and triggers reselection for dirty families.
func (d *selectionDeferral) releaseAll() {
	d.mu.Lock()
	if !d.activeLocked() {
		d.mu.Unlock()
		return
	}
	toRun := make([]family.Family, 0, len(d.dirtyFamilies))
	for fam := range d.dirtyFamilies {
		if !d.releasedFamilies[fam] {
			d.releasedFamilies[fam] = true
			toRun = append(toRun, fam)
		}
	}
	cb := d.onRelease
	doneCB := d.onComplete
	d.finishLocked()
	d.mu.Unlock()

	runReleasedFamilies(cb, toRun)
	if doneCB != nil {
		doneCB()
	}
}

// finishLocked stops the timer, marks the hold complete, and releases maps.
// Caller MUST hold d.mu.
func (d *selectionDeferral) finishLocked() {
	d.releasedAll = true
	d.active.Store(false)
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.expected = nil
	d.observed = nil
	d.excluded = nil
	d.peerOwed = nil
	d.knownFamilies = nil
	d.dirtyFamilies = nil
}

// stop cancels any active deferral timer without running reselection.
func (d *selectionDeferral) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.activeLocked() {
		return
	}
	d.finishLocked()
	ribevents.ClearSelectionDeferred()
}

func runReleasedFamilies(cb func(family.Family), families []family.Family) {
	if cb == nil {
		return
	}
	for _, fam := range families {
		cb(fam)
	}
}

// deferClock returns the clock used by selectionDeferral (RealClock in
// production, injectable fake clock in tests).
func (r *RIBManager) deferClock() clock.Clock {
	if r.deferClk != nil {
		return r.deferClk
	}
	return clock.RealClock{}
}

// emitSelectionDeferred notifies downstream FIB writers that BGP armed RFC 4724
// Section 4.1 selection deferral so they do not sweep preserved forwarding state
// before Selection_Deferral_Timer or End-of-RIB completes.
func (r *RIBManager) emitSelectionDeferred(maxDelay time.Duration) {
	// Record active deferral state before emitting so FIB writers starting at
	// Stage 5 (OnStarted) observe deferral armed at Stage 2 (OnConfigure).
	ribevents.NoteSelectionDeferred(maxDelay)
	eb := getEventBus()
	if eb == nil {
		return
	}
	if _, err := ribevents.SelectionDeferred.Emit(eb, &ribevents.SelectionDeferredEvent{MaxDelay: maxDelay}); err != nil {
		logger().Warn("bgp-rib: selection-deferred emit failed", "code", codeDoctorBGPSelectedOutput, "error", err)
	}
}

// emitSelectionComplete notifies downstream FIB writers that BGP finished
// deferred route selection across all families and published its selected
// routes (RFC 4724 Section 4.1), so remaining stale forwarding entries can now
// be swept.
func (r *RIBManager) emitSelectionComplete() {
	ribevents.ClearSelectionDeferred()
	eb := getEventBus()
	if eb == nil {
		return
	}
	if _, err := ribevents.SelectionComplete.Emit(eb, &ribevents.SelectionCompleteEvent{}); err != nil {
		logger().Warn("bgp-rib: selection-complete emit failed", "code", codeDoctorBGPSelectedOutput, "error", err)
	}
}

// reselectDeferredFamily runs best-path selection over every stored prefix in
// fam after selection deferral releases and publishes the resulting BestChange
// batch to sysrib and Loc-RIB.
func (r *RIBManager) reselectDeferredFamily(fam family.Family) {
	var keys []affectedPrefix
	elected := make(map[reselectKey]struct{})
	var scratch [nlrisplit.PrefixKeyScratchSize]byte

	r.peerMu.RLock()
	if r.stopping {
		r.peerMu.RUnlock()
		return
	}
	collect := func(routes *storage.PeerRIB) {
		addPath := routes.AddPathFamilies()
		routes.Iterate(func(entryFam family.Family, nlri []byte, _ storage.RouteEntry) bool {
			if entryFam != fam {
				return true
			}
			key, ok := newReselectKey(entryFam, nlri, addPath[entryFam], scratch[:])
			if ok {
				if _, seen := elected[key]; seen {
					return true
				}
				elected[key] = struct{}{}
			}
			keys = append(keys, affectedPrefix{fam: entryFam, nlriBytes: bytes.Clone(nlri), addPath: addPath[entryFam]})
			return true
		})
	}
	for _, routes := range r.bgpPeers {
		collect(routes)
	}
	for _, routes := range r.ribInPool[bgpProtocolID] {
		collect(routes)
	}
	r.peerMu.RUnlock()

	if len(keys) == 0 {
		return
	}
	changes := make([]bestChangeEntry, 0, len(keys))
	for _, key := range keys {
		change, changed := r.checkBestPathChange(key.fam, key.nlriBytes, key.addPath, nil)
		if changed {
			changes = append(changes, change)
		}
	}
	if len(changes) > 0 {
		publishBestChanges(changes, fam)
	}
}

// parseSelectionDeferralConfig reads `bgp update-delay max-delay` and every
// statically configured peer IP (standalone `bgp.peer.<name>` and grouped
// `bgp.group.<g>.peer.<name>`, skipping dynamic group templates) using the
// canonical configjson traversal helpers.
func parseSelectionDeferralConfig(bgpCfg map[string]any) (time.Duration, []netip.Addr, error) {
	var maxDelay time.Duration
	if udRaw, ok := bgpCfg["update-delay"].(map[string]any); ok {
		if rawVal, present := udRaw["max-delay"]; present {
			secs, err := parseUpdateDelaySecondsValue(rawVal)
			if err != nil {
				return 0, nil, err
			}
			maxDelay = time.Duration(secs) * time.Second
		}
	}

	var peers []netip.Addr
	seen := make(map[netip.Addr]struct{})
	configjson.ForEachPeer(bgpCfg, func(name string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		if origin.Template {
			return
		}
		ipStr := configjson.PeerRemoteIP(peerMap, groupMap)
		if ipStr == configjson.DynamicPeerIP {
			return
		}
		if ipStr == "" {
			ipStr = name
		}
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			return
		}
		if _, dup := seen[addr]; dup {
			return
		}
		seen[addr] = struct{}{}
		peers = append(peers, addr)
	})
	return maxDelay, peers, nil
}

func parseUpdateDelaySecondsValue(raw any) (int, error) {
	switch v := raw.(type) {
	case float64:
		secs := int(v)
		if secs < 0 {
			return 0, fmt.Errorf("bgp rib: update-delay max-delay %d must be non-negative", secs)
		}
		if secs > selectionDeferralMaxSeconds {
			return 0, fmt.Errorf("bgp rib: update-delay max-delay %d exceeds %d", secs, selectionDeferralMaxSeconds)
		}
		return secs, nil
	case string:
		secs, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("bgp rib: update-delay max-delay %q is not an integer: %w", v, err)
		}
		if secs < 0 {
			return 0, fmt.Errorf("bgp rib: update-delay max-delay %d must be non-negative", secs)
		}
		if secs > selectionDeferralMaxSeconds {
			return 0, fmt.Errorf("bgp rib: update-delay max-delay %d exceeds %d", secs, selectionDeferralMaxSeconds)
		}
		return secs, nil
	default:
		return 0, fmt.Errorf("bgp rib: update-delay max-delay has unsupported type %T", raw)
	}
}

// handleSelectionDeferralText intercepts BGP "negotiated" and "eor" JSON events
// on the text/SDK event rail and feeds them into selectionDeferral. Returns
// (true, nil) when handled, (false, nil) for unrelated event kinds, and an
// error if a deferral event carried malformed JSON or missing required fields.
func (r *RIBManager) handleSelectionDeferralText(jsonStr string) (bool, error) {
	if !r.selectionDefer.isHolding() {
		return false, nil
	}

	var outer struct {
		rpc.EventEnvelope
		Type string          `json:"type"`
		BGP  json.RawMessage `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &outer); err != nil {
		return false, err
	}
	body := []byte(jsonStr)
	if len(outer.Payload) > 0 {
		body = outer.Payload
		outer.BGP = nil
		if err := json.Unmarshal(body, &outer); err != nil {
			return false, err
		}
	}
	if len(outer.BGP) > 0 {
		body = outer.BGP
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, err
	}
	msgObj, ok := payload["message"].(map[string]any)
	if !ok {
		return false, nil
	}
	kind, _ := msgObj["type"].(string)
	if kind != "negotiated" && kind != "eor" {
		return false, nil
	}

	peerAddr, ok := extractEventPeerAddr(payload)
	if !ok {
		return true, fmt.Errorf("bgp rib: %s event missing valid peer.remote.address", kind)
	}

	if kind == "negotiated" {
		negObj, hasNeg := payload["negotiated"].(map[string]any)
		if !hasNeg {
			return true, fmt.Errorf("bgp rib: negotiated event for %s missing negotiated object", peerAddr)
		}
		hasGR := false
		restartState := false
		if grObj, foundGR := negObj["graceful-restart"].(map[string]any); foundGR {
			hasGR = true
			restartState, _ = grObj["restart-state"].(bool)
		}
		var families []family.Family
		if rawFams, foundFams := negObj["families"].([]any); foundFams {
			for _, item := range rawFams {
				famStr, _ := item.(string)
				if fam, found := family.LookupFamily(famStr); found {
					families = append(families, fam)
				}
			}
		}
		r.selectionDefer.onPeerNegotiated(peerAddr, hasGR, restartState, families)
		return true, nil
	}
	eorObj, hasEOR := payload["eor"].(map[string]any)
	if !hasEOR {
		return true, fmt.Errorf("bgp rib: eor event for %s missing eor object", peerAddr)
	}
	famStr, _ := eorObj["family"].(string)
	if fam, found := family.LookupFamily(famStr); found {
		r.selectionDefer.onPeerEOR(peerAddr, fam)
	}
	return true, nil
}

func extractEventPeerAddr(payload map[string]any) (netip.Addr, bool) {
	peerObj, ok := payload["peer"].(map[string]any)
	if !ok {
		return netip.Addr{}, false
	}
	remoteObj, ok := peerObj["remote"].(map[string]any)
	if !ok {
		return netip.Addr{}, false
	}
	addrStr, _ := remoteObj["address"].(string)
	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}
