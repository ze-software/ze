// Design: docs/architecture/bgp/filter-irr.md -- re-admitting a route the list did not yet authorize
// Overview: filter_irr.go -- handleFilterUpdate holds a rejected prefix, apply wakes the worker after a publish
// Related: readmit_engine.go -- the engine commands the worker sends
// RFC: rfc/short/rfc2918.md -- ROUTE-REFRESH as the way to re-examine a peer's prefixes

package filter_irr

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// heldPrefixMax bounds the prefixes held for re-evaluation, across every
	// peer and every ASN. A prefix rejected while the set is full is not held:
	// it is re-evaluated only when its peer sends it again, as before the set
	// existed, and the first refusal logs a warning.
	heldPrefixMax = 100_000

	// readmitWarnedMax bounds per-peer deduplication entries in readmitter.warned.
	readmitWarnedMax = 256

	readmitWarnReasonConfirm = "confirm"

	// readmitInterval is the worker's timer. Each tick drops what a peer that
	// is no longer established holds, and asks again for what the published
	// lists now admit. It fetches nothing: retrying a fetch that failed is the
	// irr plugin's schedule, and a list it stores reaches the worker through
	// the reader's publish (AC-14).
	readmitInterval = time.Minute

	// readmitCommandTimeout bounds each engine command the worker sends.
	readmitCommandTimeout = 10 * time.Second
)

// heldKey names one prefix a peer announced and the IRR filter rejected.
// family is the family token of the filter text ("ipv4/unicast"), which the
// ROUTE-REFRESH command takes back unchanged.
type heldKey struct {
	peer   netip.Addr
	family string
	prefix netip.Prefix
}

// heldOwner is who may hold a prefix: a peer the config filters under an ASN.
type heldOwner struct {
	peer netip.Addr
	asn  uint32
}

// peerFamily is the unit a ROUTE-REFRESH asks for: one peer, one family.
type peerFamily struct {
	peer   netip.Addr
	family string
}

// peerSession is what the worker needs to know about a peer before it asks the
// peer to send its routes again.
type peerSession struct {
	established  bool
	routeRefresh bool
}

// readmitEngine is the part of the engine the worker reaches. The production
// implementation dispatches commands (readmit_engine.go).
type readmitEngine interface {
	session(ctx context.Context, peer netip.Addr) (peerSession, error)
	refresh(ctx context.Context, peer netip.Addr, family string) error
}

// readmitSource is the part of the filter the worker reads: the list of an
// ASN as last published. The worker has no way to fetch one.
type readmitSource interface {
	// heldList returns the ASN's current list, nil when it has none.
	heldList(asn uint32) *irrPrefixList
}

// readmitter holds the prefixes the IRR filter rejected because the list did
// not authorize them yet, and asks their peer to send them again once it does.
//
// RFC 2918 Section 1: "When the inbound routing policy for a peer changes, all
// prefixes from that peer must be somehow made available and then re-examined
// against the new policy." An IRR list arriving or growing is such a change.
// The rejected route is not stored anywhere ze can replay it from: the import
// chain runs before bgp-adj-rib-in, which stores only accepted routes. So the
// route is made available again by asking the peer for it.
//
// Lifecycle: newReadmitter builds it, run is its one long-lived worker and
// returns when stop closes. track, forget, retain and notify are safe for
// concurrent use with each other and with run.
type readmitter struct {
	engine   readmitEngine
	source   readmitSource
	issues   *issuequeue.Queue
	interval time.Duration

	// wake has depth 1: a burst of list changes collapses into one pass, and a
	// change that arrives during a pass leaves the next one queued.
	wake chan struct{}

	mu     sync.Mutex
	held   map[heldKey]uint32 // value: the ASN whose list rejected the prefix
	full   bool               // the cap refused a prefix and has not drained since
	warned map[netip.Addr]string

	// taking holds the keys take removed for a ROUTE-REFRESH in flight. A
	// verdict on a key while it is in flight settles it, so a failed request
	// does not hold again a prefix the filter accepted or a peer withdrew
	// meanwhile. The value is the ASN, as in held.
	taking map[heldKey]uint32

	// count is len(held), written under mu after every change, so the filter's
	// per-UPDATE check that nothing is held takes no lock.
	count atomic.Int64
}

func newReadmitter(engine readmitEngine, source readmitSource, interval time.Duration) *readmitter {
	return &readmitter{
		engine:   engine,
		source:   source,
		interval: interval,
		wake:     make(chan struct{}, 1),
		held:     make(map[heldKey]uint32),
		warned:   make(map[netip.Addr]string),
		taking:   make(map[heldKey]uint32),
	}
}

// notify asks the worker for a pass. It never blocks.
func (r *readmitter) notify() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// track applies one NLRI block's verdict: a withdrawn or accepted prefix is no
// longer held, and a rejected one is held when its ASN is enrolled.
func (r *readmitter) track(peer netip.Addr, asn uint32, enrolled bool, block *familyNLRI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, prefix := range block.prefixes {
		key := heldKey{peer: peer, family: block.family, prefix: prefix}
		delete(r.taking, key)
		if block.withdraw {
			delete(r.held, key)
			continue
		}
		if block.admitted[i] {
			delete(r.held, key)
			continue
		}
		if enrolled {
			r.insert(key, asn)
			continue
		}
		delete(r.held, key)
	}
	r.changed()
}

// insert holds one key, within the cap. Caller MUST hold r.mu and MUST call
// changed after.
func (r *readmitter) insert(key heldKey, asn uint32) {
	if _, ok := r.held[key]; ok {
		r.held[key] = asn
		return
	}
	if len(r.held) >= heldPrefixMax {
		if !r.full {
			r.full = true
			logger().Warn("irr: too many rejected prefixes held for re-evaluation, holding no more",
				"limit", heldPrefixMax, "peer", key.peer, "prefix", key.prefix,
				"effect", "a prefix rejected now is evaluated again only when its peer sends it again",
				"action", "run `request peer <peer> clear soft` once the IRR lists are complete")
		}
		return
	}
	r.held[key] = asn
}

// forget drops prefixes the peer withdrew, or that the filter has accepted.
func (r *readmitter) forget(peer netip.Addr, family string, prefixes []netip.Prefix) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, prefix := range prefixes {
		key := heldKey{peer: peer, family: family, prefix: prefix}
		delete(r.held, key)
		delete(r.taking, key)
	}
	r.changed()
}

// retain keeps only the prefixes held by an owner the new config still has: a
// reconfigure that removes a peer, or stops filtering it under an ASN, drops
// what it held, even when the ASN stays enrolled through another peer.
func (r *readmitter) retain(owners map[heldOwner]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, asn := range r.held {
		if _, kept := owners[heldOwner{peer: key.peer, asn: asn}]; !kept {
			delete(r.held, key)
		}
	}
	for key, asn := range r.taking {
		if _, kept := owners[heldOwner{peer: key.peer, asn: asn}]; !kept {
			delete(r.taking, key)
		}
	}
	for peer := range r.warned {
		kept := false
		for owner := range owners {
			if owner.peer == peer {
				kept = true
				break
			}
		}
		if !kept {
			delete(r.warned, peer)
		}
	}
	r.changed()
}

// changed publishes the held count and re-arms the cap warning once the set is
// below its limit. Caller MUST hold r.mu, after every change to held.
func (r *readmitter) changed() {
	r.count.Store(int64(len(r.held)))
	if len(r.held) < heldPrefixMax {
		r.full = false
	}
}

// heldCount reports how many prefixes are held. It takes no lock.
func (r *readmitter) heldCount() int {
	return int(r.count.Load())
}

// run is the worker. It returns when stop closes. This loop has no other end
// by design: it lives as long as the filter.
func (r *readmitter) run(stop <-chan struct{}) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-r.wake:
			r.pass(stop)
		case <-ticker.C:
			r.dropIdlePeers(stop)
			r.pass(stop)
		}
	}
}

// pass asks every peer holding a prefix its ASN's list now authorizes to send
// that family again. The held prefixes of a refreshed family are dropped: the
// peer sends them again, and the filter holds again any it still rejects.
func (r *readmitter) pass(stop <-chan struct{}) {
	for pf, keys := range r.due() {
		if isStopped(stop) {
			return
		}
		r.readmit(pf, keys)
	}
}

// due groups by peer and family the held prefixes whose list now admits them.
func (r *readmitter) due() map[peerFamily][]heldKey {
	r.mu.Lock()
	snapshot := make(map[heldKey]uint32, len(r.held))
	maps.Copy(snapshot, r.held)
	r.mu.Unlock()

	lists := make(map[uint32]*irrPrefixList)
	due := make(map[peerFamily][]heldKey)
	for key, asn := range snapshot {
		list, seen := lists[asn]
		if !seen {
			list = r.source.heldList(asn)
			lists[asn] = list
		}
		if list == nil {
			continue
		}
		if !evaluatePrefix(list.entries, key.prefix) {
			continue
		}
		pf := peerFamily{peer: key.peer, family: key.family}
		due[pf] = append(due[pf], key)
	}
	return due
}

// readmit asks one peer to send one family again, for the keys now admitted.
func (r *readmitter) readmit(pf peerFamily, keys []heldKey) {
	ctx, cancel := context.WithTimeout(context.Background(), readmitCommandTimeout)
	defer cancel()

	sess, err := r.engine.session(ctx, pf.peer)
	if errors.Is(err, errReadmitPeerAbsent) {
		// The engine no longer has the peer, so nobody can ask it for anything.
		r.dropPeer(pf.peer)
		return
	}
	if err != nil {
		r.warnOnce(pf.peer, "session", "irr: a route the IRR list now authorizes stays rejected for now: ze cannot read the peer's session",
			"peer", pf.peer, "family", pf.family, "error", err,
			"effect", "the filter tries again every minute",
			"action", "check that `show bgp peer <peer> detail` answers")
		return
	}
	if !sess.established {
		// A peer that comes back up sends every route again, through the import
		// chain, so nothing it held needs asking for.
		r.dropPeer(pf.peer)
		return
	}
	if !sess.routeRefresh {
		// RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to
		// its peer only if it has received the Route Refresh Capability from its
		// peer."
		// The detail answer reports the negotiated capability: ze advertises
		// Route Refresh only when the config enables it, and the peer must
		// advertise it too (reactor config_capabilities.go).
		r.warnOnce(pf.peer, "capability", "irr: a route the IRR list now authorizes stays rejected: Route Refresh is not negotiated with this peer (ze advertises it only with `session { capability { route-refresh enable } }`, and the peer must advertise it too), so ze cannot ask the peer to send the route again",
			"peer", pf.peer, "family", pf.family, "prefixes", len(keys),
			"action", "add `session { capability { route-refresh enable } }` to this peer in ze and enable Route Refresh on the peer's side, then reset the session with `request peer <peer> teardown`; the reset alone also makes the peer send its routes again")
		r.dropKeys(keys)
		return
	}
	// The keys go BEFORE the request: the peer's reply runs through the filter,
	// which holds again any prefix it still rejects, and a drop after the reply
	// would delete that new hold.
	taken := r.take(keys)
	defer r.settle(taken)
	// RFC 2918 Section 4: the capability was checked above, and the engine
	// checks it again before it writes the message.
	if err := r.engine.refresh(ctx, pf.peer, pf.family); err != nil {
		if strings.Contains(err.Error(), sendNotPermittedText) {
			// Asking again cannot work until the operator changes the config.
			r.warnOnce(pf.peer, "permission", "irr: a route the IRR list now authorizes stays rejected: the peer's config does not let bgp-filter-irr ask it to send the route again",
				"peer", pf.peer, "family", pf.family, "prefixes", len(keys), "error", err,
				"action", "allow bgp:refresh from /bus/endpoint/bgp-filter-irr in the peer's bus block, then run `request peer <peer> refresh <family>`")
			return
		}
		r.restore(taken)
		r.warnOnce(pf.peer, "refresh", "irr: a route the IRR list now authorizes stays rejected for now: ze could not ask the peer to send it again",
			"peer", pf.peer, "family", pf.family, "prefixes", len(keys), "error", err,
			"effect", "the filter asks again on the next pass, at most a minute later",
			"action", "check the session with `show bgp peer <peer> detail`")
		return
	}
	// The engine answers success for a peer it skipped because the session went
	// down meanwhile, so the request is reported only once the session reads up.
	sess, err = r.engine.session(ctx, pf.peer)
	if errors.Is(err, errReadmitPeerAbsent) {
		r.dropPeer(pf.peer)
		return
	}
	if err != nil {
		// The request went out and only its confirmation failed. What the peer
		// sends back runs through the filter, which holds again what it still
		// rejects, so nothing the peer holds is dropped on a guess.
		r.reportReadmitConfirmFailed(pf, len(keys), err)
		return
	}
	if !sess.established {
		r.dropPeer(pf.peer)
		return
	}
	r.clearWarning(pf.peer)
	logger().Info("irr: asked the peer to send again routes the IRR list now authorizes",
		"peer", pf.peer, "family", pf.family, "prefixes", len(keys))
}

// dropIdlePeers drops what a peer that is no longer established holds.
func (r *readmitter) dropIdlePeers(stop <-chan struct{}) {
	for _, peer := range r.heldPeers() {
		if isStopped(stop) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), readmitCommandTimeout)
		sess, err := r.engine.session(ctx, peer)
		cancel()
		if errors.Is(err, errReadmitPeerAbsent) {
			r.dropPeer(peer)
			continue
		}
		if err != nil {
			continue
		}
		if !sess.established {
			r.dropPeer(peer)
		}
	}
}

func (r *readmitter) heldPeers() []netip.Addr {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := make(map[netip.Addr]struct{})
	peers := make([]netip.Addr, 0, 4)
	for key := range r.held {
		if _, ok := seen[key.peer]; ok {
			continue
		}
		seen[key.peer] = struct{}{}
		peers = append(peers, key.peer)
	}
	return peers
}

func (r *readmitter) dropKeys(keys []heldKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, key := range keys {
		delete(r.held, key)
	}
	r.changed()
}

// take drops keys, marks them in flight, and returns the ASN each was still
// held for. The caller MUST call settle once the request is over, and restore
// before it when the request failed.
func (r *readmitter) take(keys []heldKey) map[heldKey]uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	taken := make(map[heldKey]uint32, len(keys))
	for _, key := range keys {
		asn, ok := r.held[key]
		if !ok {
			continue
		}
		taken[key] = asn
		delete(r.held, key)
		r.taking[key] = asn
	}
	r.changed()
	return taken
}

// restore holds again what take returned and is still in flight. A key an
// UPDATE judged meanwhile is not: an accept or a withdrawal settled it, and a
// reject held it again under the newer ASN.
func (r *readmitter) restore(taken map[heldKey]uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range taken {
		asn, inFlight := r.taking[key]
		if !inFlight {
			continue
		}
		delete(r.taking, key)
		r.insert(key, asn)
	}
	r.changed()
}

// settle ends the flight of what take returned. MUST be called once for each
// take, after restore when there is one.
func (r *readmitter) settle(taken map[heldKey]uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range taken {
		delete(r.taking, key)
	}
}

func (r *readmitter) dropPeer(peer netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.held {
		if key.peer == peer {
			delete(r.held, key)
		}
	}
	for key := range r.taking {
		if key.peer == peer {
			delete(r.taking, key)
		}
	}
	delete(r.warned, peer)
	r.changed()
}

// warnOnce logs msg the first time a peer meets reason, and again only after
// the reason changed or a refresh to the peer succeeded.
func (r *readmitter) warnOnce(peer netip.Addr, reason, msg string, args ...any) {
	r.mu.Lock()
	repeat := r.warned[peer] == reason
	r.warned[peer] = reason
	r.mu.Unlock()
	if repeat {
		return
	}
	logger().Warn(msg, args...)
}

func (r *readmitter) clearWarning(peer netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.warned, peer)
}

// shouldWarn records reason for peer and reports true when this is a new or
// changed warning condition for that peer. The map is bounded to
// readmitWarnedMax entries and cleared by clearWarning, dropPeer, and retain.
func (r *readmitter) shouldWarn(peer netip.Addr, reason string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.warned == nil {
		r.warned = make(map[netip.Addr]string)
	}
	if r.warned[peer] == reason {
		return false
	}
	if _, exists := r.warned[peer]; !exists {
		if len(r.warned) >= readmitWarnedMax {
			for k := range r.warned {
				delete(r.warned, k)
				break
			}
		}
	}
	r.warned[peer] = reason
	return true
}

// reportReadmitConfirmFailed emits one deduplicated WARN and report-bus error
// event under codeIRRReadmitConfirmFailed when confirming a peer's session
// after a ROUTE-REFRESH fails.
func (r *readmitter) reportReadmitConfirmFailed(pf peerFamily, prefixCount int, err error) {
	if !r.shouldWarn(pf.peer, readmitWarnReasonConfirm) {
		return
	}
	peer := pf.peer.String()
	var fixBuf textbuf.Buffer
	fix := fixBuf.Str("ze show bgp peer ").Str(peer).Str(" detail").String()
	var msgBuf textbuf.Buffer
	msg := fitReportText(
		msgBuf.Str("irr: asked peer ").Str(peer).Str(" to re-send ").Str(pf.family).Str(" routes now authorized by IRR, but confirming peer session state failed: ").Err(err).Str("; held prefixes of other families were kept and any still-rejected prefix will be held again").String(),
		rpc.ReportMessageMax,
	)
	reportErr := r.issues.RaiseError(codeIRRReadmitConfirmFailed, fitReportText(peer, rpc.ReportSubjectMax), msg, fix, func(cbErr error) {
		logger().Warn("irr: failed to report readmission session confirmation failure to daemon",
			"code", codeIRRReadmitConfirmFailed,
			"fix", fix,
			"peer", pf.peer,
			"family", pf.family,
			"error", cbErr)
	})
	logger().Warn(msg,
		"code", codeIRRReadmitConfirmFailed,
		"fix", fix,
		"peer", pf.peer,
		"family", pf.family,
		"prefixes", prefixCount,
		"error", err,
		issuequeue.Outcome(reportErr))
}

// isStopped reports whether stop has closed, without blocking.
func isStopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}
