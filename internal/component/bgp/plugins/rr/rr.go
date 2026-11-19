// Design: docs/architecture/core-design.md -- route reflector plugin
// Detail: withdrawal.go -- NLRI tracking and peer-down withdrawal
// RFC: rfc/short/rfc7911.md -- Section 2, a departure is withdrawn under ze's own Path Identifier
//
// RFC 4456: BGP Route Reflection.
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles the RFC 4456 forwarding rules:
//   - Source exclusion (don't send back to source)
//   - Client/non-client filtering (client->all, non-client->clients only)
//   - ORIGINATOR_ID injection (source peer's BGP Identifier)
//   - CLUSTER_LIST prepend (reflector's cluster-id)
//   - Next-hop rewriting (per destination peer settings)

package rr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/syncutil"
	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger configures the package-level logger for the RR plugin.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

const (
	eventUpdate = "update"
	eventState  = "state"
	eventOpen   = "open"

	updateRouteTimeout     = 60 * time.Second
	replayConvergenceMax   = 10
	replayConvergenceDelay = 20 * time.Millisecond
	statusDone             = "done"
	statusError            = "error"

	// NLRI action tokens for withdrawal map tracking.
	actionAdd = "add"
	actionDel = "del"

	// withdrawalBatchSize caps the number of prefixes per withdrawal RPC.
	withdrawalBatchSize = 1000
)

// peerState tracks a connected peer's state and capabilities.
type peerState struct {
	// replayDebt is current work for this original target only, guarded by mu.
	replayDebt        *peerReplayDebt
	replayCalls       uint64 // Guarded by mu; pins this original actor until its admitted replay returns.
	optionalReplayErr error
	sourceOutputDebt  error
	departureOwed     *departureWork
	departureCall     sync.Mutex
	targetOutputDebt  error
	// inventoryRevision and processedMsgID are guarded by withdrawalMu. They
	// describe only this original actor's current receive projection, not a
	// tombstone or a second route inventory.
	inventoryRevision   uint64
	processedMsgID      uint64
	leaseMu             sync.Mutex
	leaseCall           sync.Mutex
	Address             string
	Identity            peerIdentity
	DestinationSession  uint64
	SourceReceipt       uint64
	Leased              bool
	DownSeen            bool
	PublishedGeneration uint64
	RetainPending       bool
	ASN                 uint32
	Up                  bool
	ReplayGen           uint64 // Incremented on each state-up, guards stale goroutines
	Families            map[family.Family]bool
	Capabilities        map[string]bool
}

// routeReflector implements a BGP Route Reflector plugin (RFC 4456).
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles RFC 4456 forwarding rules, ORIGINATOR_ID injection,
// CLUSTER_LIST prepend, and next-hop rewriting.
type routeReflector struct {
	replayRetryMu   sync.Mutex
	replayRetryIdle chan struct{}
	cleanupPending  map[peerIdentity]uint64
	workMu          sync.Mutex
	workErr         error
	receiveGate     string
	replays         replayLifetimes
	plugin          *sdk.Plugin
	// retained holds bgp-gr's retention verdict per peer, recorded from the
	// bgp-gr/retention event and taken by handleStateDown for that peer.
	retained retention.Ledger
	peers    map[peerIdentity]*peerState
	mu       sync.RWMutex
	buf      textbuf.Buffer
	stopping atomic.Bool

	// withdrawalMu protects the withdrawals map.
	withdrawalMu sync.Mutex
	// withdrawals tracks announced routes per source peer for withdrawal on peer-down.
	// Observed address -> original owner, physical connection and native route key -> withdrawalInfo.
	withdrawals map[string]map[withdrawalKey]withdrawalInfo
	// sourcePolicy holds each source's open source-policy transaction
	// (source_policy.go), keyed by original owner and guarded by withdrawalMu.
	sourcePolicy map[uint64]*sourcePolicyTxn
	// forwards counts the received UPDATEs being reflected, and validation
	// is the revalidation worker (nil without an event bus): a source-policy
	// phase waits for both (settleSource).
	forwards   forwardGate
	validation *validationQueue

	// departures holds the workers that send each departed source's routes
	// to the engine (sendPeerDown), up to departureWorkers sources at once,
	// taking sources in the order they went down. enqueueDeparture starts
	// them on the first departure. Event delivery MUST stop and downCtx MUST
	// be canceled before stopDepartures, and stopDepartures MUST return
	// before the plugin connection is released. departureMu guards both
	// fields.
	departureMu       sync.Mutex
	departures        *syncutil.Coalescer[peerIdentity, departureWork]
	departuresStopped bool
	// departing counts each source's departures queued or running, so a
	// source-policy prepare can wait for them (awaitDepartures). merged is
	// set by the Coalescer's merge, which runs inside Enqueue, so an enqueue
	// merged into a waiting departure adds no count. departureMu guards both.
	departing map[peerIdentity]*departureWait
	merged    bool
	// downCtx bounds every peer-down recovery request. It is canceled at
	// shutdown and carries no deadline: the engine keeps a departure's keys
	// owed until it decides them, so a command timeout would only stop the
	// waiter. Nil in struct-literal unit owners, which have no lifecycle.
	downCtx context.Context
}

// runRouteReflector runs the Route Reflector plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runRouteReflector(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-rr", conn)
	defer func() { _ = p.Close() }()

	// bgp-rr runs internal or forked alike: the retention verdict arrives as
	// the bgp-gr/retention event (rr.retained), the receive store's
	// eligibility change and the RIB's FlowSpec lookup through the SDK
	// (dispatchText), so nothing it reads is filled only in the daemon process.
	return serveRouteReflector(p)
}

// serveRouteReflector registers the reflector's callbacks on p and runs it
// until the connection ends.
func serveRouteReflector(p *sdk.Plugin) int {
	downCtx, stopDown := context.WithCancel(context.Background())
	rr := &routeReflector{
		plugin:      p,
		peers:       make(map[peerIdentity]*peerState),
		withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo),
		downCtx:     downCtx,
	}
	defer rr.stopDepartures()
	defer func() {
		stopDown()
		rr.replays.stop()
	}()

	// Register structured event handler for DirectBridge delivery (hot path).
	p.OnStructuredEvent(func(events []any) error {
		var failed error
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok {
				continue
			}
			switch se.EventType {
			case rpc.EventKindUpdate:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					// Update withdrawal map BEFORE forwarding: the forward path can
					// trigger cache eviction which frees the pool buffer backing
					// msg.WireUpdate. Reading WireUpdate after forward is use-after-free.
					if err := rr.reflectWire(se, msg); err != nil {
						var output *forwardOutputError
						if !errors.As(err, &output) {
							failed = errors.Join(failed, err)
						}
					}
				}
			case rpc.EventKindState:
				if err := rr.handleStructuredState(se); err != nil {
					failed = errors.Join(failed, err)
				}
			case rpc.EventKindOpen:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					if err := rr.handleStructuredOpen(se, msg); err != nil {
						failed = errors.Join(failed, err)
					}
				}
			case rpc.EventKindUnspecified, rpc.EventKindNotification, rpc.EventKindKeepalive,
				rpc.EventKindRefresh, rpc.EventKindEOR, rpc.EventKindBoRR, rpc.EventKindEoRR,
				rpc.EventKindSent, rpc.EventKindNegotiated, rpc.EventKindCount:
				// These events do not trigger reflection.
			default:
				// The plugin event set is open; unknown events do not trigger reflection.
			}
		}
		return failed
	})

	// Register text event handler for fork-mode delivery (fallback).
	p.OnEvent(func(text string) error {
		err := rr.dispatchText(text)
		var output *forwardOutputError
		if errors.As(err, &output) {
			return nil
		}
		return err
	})
	p.OnEventSettlement(rr.settleEvents)
	p.OnDoctorCheck(rr.replayDoctorCheck)

	// RFC 8212 Section 3: the source-policy lifecycle owner (source_policy.go).
	p.OnSourcePolicy(rr.handleSourcePolicy)

	// Register command handler.
	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		return rr.handleCommand(command)
	})

	// Startup ownership comes from Registration.Claims. Mid-life auto-load
	// and restart deliver this callback to notify an already-running store.
	// A forked plugin, or one beside a forked bgp-rib, reads the RIB's
	// FlowSpec lookup through the command its flowspec-gate names.
	releaseGateDispatch := ribevents.RegisterGateDispatch(rr.dispatchCommand)
	defer releaseGateDispatch()

	p.OnAllPluginsReady(func() error {
		// Every plugin has started, so the RIB's gate-query subscription is
		// live: ask it to announce its flowspec-gate again, in case it did so
		// before this plugin subscribed.
		if err := rr.askFlowSpecGate(); err != nil {
			logger().Error("bgp-rr: gate-query not sent: FlowSpec replay reads no rule until the RIB announces", "error", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		status, _, err := rr.dispatchCommand(ctx, "request bgp adj-rib-in claim-replay")
		if err == nil && status != statusDone {
			err = fmt.Errorf("adj-rib-in replay claim returned status %q", status)
		}
		if err != nil {
			logger().Warn("could not claim adj-rib-in replay ownership; peer-up routes may be announced twice", "error", err)
		}
		return err
	})

	// Subscribe to received-direction only for UPDATE and OPEN events.
	// Same rationale as bgp-rs: subscribing to "both" for UPDATEs creates
	// a circular deadlock (ForwardUpdate -> onMessageSent -> deliver -> block).
	p.SetStartupSubscriptions([]string{
		eventUpdate + " direction received",
		eventState,
		eventOpen + " direction received",
	}, nil, "")

	p.SetEncoding("text")
	// bgp-gr's per-DOWN retention verdict, enveloped so dispatchText tells it
	// from the BGP text events. RFC 4724 Section 4.2 (handleStateDown).
	p.AddStartupSubscriptionsIn(retention.Namespace, []string{retention.EventType, retention.CleanupEventType})
	// The receive store's eligibility change, through the SDK so a forked
	// bgp-rr hears it too: the daemon's own bus never reaches a subprocess.
	// The selecting RIB's flowspec-gate names the command that answers its
	// FlowSpec lookup, so a forked bgp-rr (or one beside a forked bgp-rib)
	// replays FlowSpec rules rather than withdrawing every one.
	p.AddStartupSubscriptionsIn(ribevents.Namespace, []string{ribevents.EventValidationChange, ribevents.EventFlowSpecGate, ribevents.EventValidationGate})
	p.SetEnvelope(true)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer rr.stopping.Store(true)
	stopValidation := rr.startValidation(ctx)
	defer stopValidation()
	err := p.Run(ctx, routeReflectorRegistration())

	if err != nil {
		logger().Error("rr plugin failed", "error", err)
		return 1
	}

	return 0
}

// forwardUpdate forwards a cached UPDATE to all peers via cache-forward.
// The reactor handles source exclusion, client/non-client filtering (RFC 4456),
// ORIGINATOR_ID, CLUSTER_LIST, and next-hop rewriting.
func (rr *routeReflector) forwardUpdate(ctx context.Context, source peerIdentity, msgID uint64) error {
	if rr.stopping.Load() {
		return rr.plugin.ReleaseCached(ctx, []uint64{msgID})
	}
	targets := rr.targets(source, family.Family{})
	if len(targets) == 0 {
		return rr.plugin.ReleaseCached(ctx, []uint64{msgID})
	}
	captured := make([]rpc.ForwardTarget, 0, len(targets))
	for _, target := range targets {
		captured = append(captured, rpc.ForwardTarget{Destination: target.identity.address.String(),
			DestinationOwner: target.identity.owner, DestinationSession: target.session})
	}
	// One request owns the cache consumer's single ACK, after every captured
	// destination's final writer or joined cancellation has completed.
	err := rr.plugin.ForwardCached(ctx, rpc.ForwardCachedInput{IDs: []uint64{msgID}, Targets: captured})
	var notDispatched *rpc.CachedWriteNotDispatchedError
	if errors.As(err, &notDispatched) {
		err = rr.rejectCached(msgID, err)
	}
	var command textbuf.Buffer
	rr.reportUpdateFailure("*", command.Str("cached ").Uint(msgID).String(), err)
	return err
}

// updateRoute sends a route update command to matching peers via the engine.
func (rr *routeReflector) updateRoute(peerSelector, command string) {
	rr.updateRouteWithMeta(peerSelector, command, nil)
}

// updateRouteWithMeta is updateRoute with route metadata for the engine's
// command context (nil for none).
func (rr *routeReflector) updateRouteWithMeta(peerSelector, command string, meta map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()

	_, _, err := rr.plugin.UpdateRouteWithMeta(ctx, peerSelector, command, meta)
	rr.reportUpdateFailure(peerSelector, command, err)
}

func (rr *routeReflector) reportUpdateFailure(peerSelector, command string, err error) {
	if err != nil { //nolint:gocritic // ifElseChain: switch blocked by block-silent-ignore hook
		if rr.stopping.Load() {
			logger().Debug("update-route failed (shutting down)",
				"peer", peerSelector, "command", command, "error", err)
		} else if isConnectionError(err) {
			logger().Warn("update-route failed (peer disconnected)",
				"peer", peerSelector, "command", command, "error", err)
		} else {
			logger().Error("update-route failed",
				"peer", peerSelector, "command", command, "error", err)
		}
	}
}

// recoverRouteSel sends a departed source's withdrawal as a source-owned DOWN
// request (recovery-source), through the normal UPDATE grammar and a typed
// selector. The engine reconciles each selected destination against bgp-rib:
// it sends the best surviving route in place of the departed one, or the
// withdrawal, under the Path Identifier ze advertised to that destination.
//
// RFC 7911 Section 2: "A BGP speaker that re-advertises a route MUST generate
// its own Path Identifier to be associated with the re-advertised route." The
// command names the source's identifiers, so only the engine, which minted the
// destination's, can withdraw the path that destination holds.
//
// recovery-generation names the session whose end the departure answers, so
// the engine owes only routes that session or an older one of the source
// wrote, never a newer session's route graceful restart retains.
func (rr *routeReflector) recoverRouteSel(ctx context.Context, target peerTarget, command string, source peerIdentity, owner, generation uint64) error {
	meta := map[string]any{"recovery-source": source.address.String(), "recovery-source-owner": strconv.FormatUint(owner, 10),
		"recovery-generation": strconv.FormatUint(generation, 10)}
	_, _, err := rr.plugin.UpdateRouteRequest(ctx, rpc.UpdateRouteInput{PeerSelector: target.identity.address.String(), Command: command, Meta: meta,
		DestinationOwner: target.identity.owner, DestinationSession: target.session})
	return err
}

// isConnectionError reports whether err indicates the target peer's connection is closed.
func isConnectionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

// handleStructuredState processes state events from DirectBridge.
func (rr *routeReflector) handleStructuredState(se *rpc.StructuredEvent) error {
	identity, err := newPeerIdentity(se.PeerAddress, se.PeerOwner, se.ConnectionID)
	if err != nil {
		if se.PeerOwner == 0 {
			return err
		}
		if se.ConnectionID != 0 {
			return err
		}
		if se.State != rpc.SessionStateDown {
			return err
		}
		if se.Reason != rpc.ReasonPeerRemoved {
			return err
		}
		// Deconfiguration may end the owner before it ever opened a session.
		rr.endOwner(se.PeerOwner)
		return nil
	}
	switch se.State {
	case rpc.SessionStateUp:
		if se.SessionGeneration == 0 {
			return errors.New("bgp-rr: UP has no captured source-policy generation")
		}
		if se.DestinationSession == 0 {
			return errors.New("bgp-rr: UP has no captured destination session")
		}
		rr.mu.RLock()
		older := make([]peerIdentity, 0)
		for prior := range rr.peers {
			if prior.owner != identity.owner {
				continue
			}
			if prior.connection > identity.connection {
				rr.mu.RUnlock()
				return nil
			}
			if prior.connection < identity.connection {
				older = append(older, prior)
			}
		}
		rr.mu.RUnlock()
		// The same actor lease lock serializes late page adoption with replacement
		// and DOWN. No old physical actor can acquire after its successor arrives.
		for _, prior := range older {
			rr.mu.RLock()
			state := rr.peers[prior]
			rr.mu.RUnlock()
			if state == nil {
				continue
			}
			rr.mu.RLock()
			alreadyDown := state.DownSeen
			generation := state.PublishedGeneration
			rr.mu.RUnlock()
			if !alreadyDown {
				rr.handleStateDown(prior, generation)
			}
		}
		rr.mu.Lock()
		if rr.peers == nil {
			rr.peers = make(map[peerIdentity]*peerState)
		}
		peer := rr.peers[identity]
		if peer == nil {
			peer = &peerState{Address: se.PeerAddress, Identity: identity}
			rr.peers[identity] = peer
		}
		if peer.Up {
			rr.mu.Unlock()
			return nil
		}
		if peer.DownSeen {
			rr.mu.Unlock()
			return nil
		}
		peer.Up = true
		peer.DestinationSession = se.DestinationSession
		peer.PublishedGeneration = se.SessionGeneration
		if asn, known := se.PeerAS.ASN(); known {
			peer.ASN = asn
		}
		if err := rr.retained.Begin(identity.owner, identity.connection); err != nil {
			peer.Up = false
			rr.mu.Unlock()
			return err
		}
		if se.Reason == rpc.ReasonAttached {
			rr.mu.Unlock()
			return nil
		}
		peer.ReplayGen++
		target := peerTarget{identity: identity, session: peer.DestinationSession, generation: peer.ReplayGen}
		rr.mu.Unlock()
		rr.startPeerReplay(target, se.InitialReplay)
	case rpc.SessionStateDown:
		rr.mu.RLock()
		peer := rr.peers[identity]
		rr.mu.RUnlock()
		if peer == nil {
			return nil
		}
		rr.mu.RLock()
		seen := peer.DownSeen
		rr.mu.RUnlock()
		if seen {
			return nil
		}
		rr.handleStateDown(identity, se.SessionGeneration)
	case rpc.SessionStateUnspecified, rpc.SessionStateCount:
		return errors.New("bgp-rr: state event has no valid state")
	default:
		return errors.New("bgp-rr: state event has an unknown state")
	}
	return nil
}

// handleStateDown hands every route the downed source peer still announces to
// the engine as a source-DOWN recovery (recoverRouteSel), stamped with the
// peer-down's session generation, except the routes of
// a family bgp-gr retains for the source (GR or LLGR). Routes are batched by
// family, wire form and ADD-PATH to minimize RPCs. The engine, not this plugin,
// names what each destination is sent: the best surviving route, or the
// withdrawal under the Path Identifier ze advertised to that destination, which
// differs from the source's own toward a destination that negotiated ADD-PATH
// send (RFC 7911 Section 2). The requests run on the plugin's departure
// workers (enqueueDeparture, sendPeerDown), never on the event delivery
// goroutine.
//
// Concurrency: OnStructuredEvent is called serially by the engine's delivery
// goroutine, so handleStateDown and UPDATE processing for the same peer cannot
// race within the structured path. The withdrawalMu guards against the
// (theoretical) case of text and structured paths processing the same peer
// concurrently.
func (rr *routeReflector) handleStateDown(source peerIdentity, generation uint64) {
	retained, err := rr.retained.Take(source.owner, source.connection)
	if err != nil {
		rr.noteWorkFailure("DOWN retention admission", err)
		return
	}
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		return
	}
	state.leaseMu.Lock()
	rr.mu.Lock()
	state.Up = false
	state.DownSeen = true
	state.optionalReplayErr = nil
	rr.mu.Unlock()
	rr.withdrawalMu.Lock()
	entries := make(map[withdrawalKey]withdrawalInfo)
	for key, info := range rr.withdrawals[source.address.String()] {
		if key.owner != source.owner {
			continue
		}
		if key.connection != source.connection {
			continue
		}
		if retained[info.Family] {
			info.Retained = true
			rr.withdrawals[source.address.String()][key] = info
			continue
		}
		info.Generation = generation
		info.Closing = true
		rr.withdrawals[source.address.String()][key] = info
		entries[key] = info
	}
	if txn := rr.sourcePolicy[source.owner]; txn != nil {
		if txn.actor == source {
			rr.dropSourcePolicyLocked(source.owner)
		}
	}
	kept := false
	for key, info := range rr.withdrawals[source.address.String()] {
		if key.owner != source.owner {
			continue
		}
		if key.connection != source.connection {
			continue
		}
		if info.Retained {
			if !info.Closing {
				kept = true
			}
		}
	}
	rr.withdrawalMu.Unlock()
	rr.mu.Lock()
	state.RetainPending = kept
	rr.mu.Unlock()
	// Even an empty departure owns the normal-DOWN lease release; it cannot
	// remain queued until an unrelated future applied-event barrier.
	if !rr.enqueueDeparture(source, entries) {
		rr.noteWorkFailure("source departure", errors.New("departure worker stopped before admission"))
	}
	rr.oweCleanup(source)
	state.leaseMu.Unlock()
}

// takeWithdrawalEntriesLocked removes only the original RECEIVE owner's entries.
// Caller MUST hold withdrawalMu. Zero is unrecorded, so it selects nothing.
func (rr *routeReflector) takeWithdrawalEntriesLocked(source string, owner, connection uint64) map[withdrawalKey]withdrawalInfo {
	if owner == 0 {
		return nil
	}
	var taken map[withdrawalKey]withdrawalInfo
	entries := rr.withdrawals[source]
	for key, entry := range entries {
		if entry.SourceOwner != owner {
			continue
		}
		if key.connection != connection {
			continue
		}
		if taken == nil {
			taken = make(map[withdrawalKey]withdrawalInfo)
		}
		taken[key] = entry
		delete(entries, key)
	}
	if len(entries) == 0 {
		delete(rr.withdrawals, source)
	}
	return taken
}

// departureWorkers bounds how many sources' departures the route reflector
// sends at once. The engine answers a departure request only once every owed
// key is decided at every destination, so one source's full table, or one
// destination slow to take its writes, holds its worker for as long as that
// takes. Several workers keep that from holding every other source's
// departure behind it, and the bound keeps a burst of downs from opening one
// recovery per source against the engine at once.
const departureWorkers = 8

// enqueueDeparture queues a departed source's routes for the departure
// workers, starting them on first use. A source that goes down again before a
// worker took its waiting departure is merged into it, the later announcement
// of a route winning, so the queue holds at most one departure per source; a
// source's departures are sent one at a time, in order. It never blocks the
// event delivery goroutine: blocking it would hold back the events the
// engine's recovery of an earlier departure may wait on. It answers false,
// queuing nothing, once stopDepartures was called.
func (rr *routeReflector) enqueueDeparture(source peerIdentity, entries map[withdrawalKey]withdrawalInfo) bool {
	// Snapshot before queue admission. A worker MUST NOT discover a successor
	// through current peers after this source's original DOWN was delivered.
	work := departureWork{entries: entries}
	if len(entries) > 0 {
		work.batches = []departureBatch{{entries: entries, targets: rr.targets(source, family.Family{})}}
	}
	rr.departureMu.Lock()
	defer rr.departureMu.Unlock()
	if rr.departuresStopped {
		return false
	}
	if rr.departures == nil {
		rr.departures = syncutil.NewCoalescer(departureWorkers, func(waiting, next departureWork) departureWork {
			rr.merged = true
			merged := make(map[withdrawalKey]withdrawalInfo, len(waiting.entries)+len(next.entries))
			maps.Copy(merged, waiting.entries)
			maps.Copy(merged, next.entries)
			waiting.entries = merged
			waiting.batches = append(waiting.batches, next.batches...)
			return waiting
		}, rr.sendDeparture)
	}
	rr.merged = false
	if !rr.departures.Enqueue(source, work) {
		return false
	}
	if rr.merged {
		return true
	}
	if rr.departing == nil {
		rr.departing = make(map[peerIdentity]*departureWait)
	}
	wait := rr.departing[source]
	if wait == nil {
		wait = &departureWait{idle: make(chan struct{})}
		rr.departing[source] = wait
	}
	wait.count++
	return true
}

// stopDepartures refuses further departures and, when the workers were
// started, stops them after they sent every queued departure. With no
// departure ever queued it starts nothing. MUST be called after downCtx is
// canceled, so a request the engine has not answered ends rather than
// holding shutdown.
func (rr *routeReflector) stopDepartures() {
	rr.departureMu.Lock()
	rr.departuresStopped = true
	departures := rr.departures
	rr.departureMu.Unlock()
	if departures == nil {
		return
	}
	departures.Stop()
}

// withdrawalGroup names one batched peer-down command: every route in it shares
// a family and a command form. VPN routes, and ADD-PATH labeled and unicast
// routes, retain native bytes and the ADD-PATH negotiation, so their group goes
// out as hex.
type withdrawalGroup struct {
	owner    uint64
	family   family.Family
	wireForm bool
	addPath  bool
	// generation is the departed session the group's routes belong to: a
	// merged departure sends each session's routes under its own bound.
	generation uint64
}

// sendPeerDown runs one departure on a departure worker. It sends the
// departed source's routes to every peer except that source, grouped by
// family, wire form and ADD-PATH, at most withdrawalBatchSize routes per
// request to bound command length.
func (rr *routeReflector) sendPeerDown(ctx context.Context, source peerIdentity, work departureWork) error {
	for _, batch := range work.batches {
		if err := rr.sendDepartureBatch(ctx, source, batch); err != nil {
			return err
		}
	}
	return nil
}

func (rr *routeReflector) sendDepartureBatch(ctx context.Context, source peerIdentity, work departureBatch) error {
	byGroup := make(map[withdrawalGroup][]string)
	for _, info := range work.entries {
		group := withdrawalGroup{family: info.Family, wireForm: info.WireForm, addPath: info.AddPath, owner: info.SourceOwner, generation: info.Generation}
		byGroup[group] = append(byGroup[group], info.Prefix)
	}
	for group, prefixes := range byGroup {
		slices.Sort(prefixes)
		for i := 0; i < len(prefixes); i += withdrawalBatchSize {
			end := min(i+withdrawalBatchSize, len(prefixes))
			command := nlriDelCmd(group.family.String(), prefixes[i:end])
			if group.wireForm {
				var text textbuf.Buffer
				text.Str("update hex nlri ").Str(group.family.String())
				if group.addPath {
					text.Str(" addpath")
				}
				for _, prefix := range prefixes[i:end] {
					text.Str(" del ").Str(prefix)
				}
				command = text.String()
			}
			for _, target := range work.targets {
				if err := rr.recoverRouteSel(ctx, target, command, source, group.owner, group.generation); err != nil {
					rr.noteTargetOutputFailure(target, "original source departure", err)
					return err
				}
			}
		}
	}
	return nil
}

// rrOpenUnreadable is the warning for a received OPEN the route reflector
// cannot decode. The reactor refuses the same OPEN, and at the default level
// this is the only line that says why in the operator's terms. The peer address
// and the decoder error go in the record's peer and error attributes.
const rrOpenUnreadable = "The route reflector cannot read this peer's OPEN message, " +
	"so it records no address families for this peer. " +
	"ze refuses this OPEN, so the session with this peer does not come up. " +
	"Next step: check the capabilities configured on this peer, then compare them with the error attribute."

// handleStructuredOpen processes OPEN events from DirectBridge.
// Decodes raw OPEN wire bytes to extract peer capabilities and families.
//
// An OPEN that does not decode records nothing: no ASN, no families and no
// implicit ipv4/unicast, because a default stored for an OPEN nobody could read
// is a wrong answer. An entry from an earlier session of the peer stays, and
// the next accepted OPEN overwrites it; its readers skip it while the peer is
// down.
func (rr *routeReflector) handleStructuredOpen(se *rpc.StructuredEvent, msg *bgptypes.RawMessage) error {
	identity, identityErr := newPeerIdentity(se.PeerAddress, se.PeerOwner, se.ConnectionID)
	if identityErr != nil {
		return identityErr
	}
	if msg.RawBytes == nil {
		return errors.New("bgp-rr: received OPEN is unreadable")
	}

	open, err := message.UnpackOpen(msg.RawBytes)
	if err != nil {
		logger().Warn(rrOpenUnreadable, "peer", se.PeerAddress, "error", err)
		return errors.New("bgp-rr: received OPEN is unreadable")
	}
	// RFC 9072 Section 2: the OPEN's own framing decides the Parameter Length width.
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		logger().Warn(rrOpenUnreadable, "peer", se.PeerAddress, "error", err)
		return errors.New("bgp-rr: received OPEN is unreadable")
	}

	asn := uint32(open.MyAS)
	families := make(map[family.Family]bool)
	capabilities := make(map[string]bool)
	hasMP := false
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok {
			hasMP = true
			capabilities["multiprotocol"] = true
			fam := family.Family{AFI: mp.AFI, SAFI: mp.SAFI}
			if _, known := family.LookupFamily(fam.String()); known {
				families[fam] = true
			}
		}
		if asn4, ok := c.(*capability.ASN4); ok {
			asn = asn4.ASN
			capabilities["asn4"] = true
		}
	}

	// RFC 4760 Section 1: ipv4/unicast is the implicit default only when
	// the peer sends no Multiprotocol capability.
	if !hasMP {
		families[family.IPv4Unicast] = true
	}

	return rr.recordOpen(identity, asn, families, capabilities)
}

func (rr *routeReflector) recordOpen(identity peerIdentity, asn uint32, families map[family.Family]bool, capabilities map[string]bool) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for prior, state := range rr.peers {
		if prior.owner != identity.owner {
			continue
		}
		if prior.connection > identity.connection {
			return nil
		}
		if prior == identity {
			if state.DownSeen {
				return nil
			}
		}
	}
	if rr.peers == nil {
		rr.peers = make(map[peerIdentity]*peerState)
	}

	if rr.peers[identity] == nil {
		rr.peers[identity] = &peerState{Address: identity.address.String(), Identity: identity}
	}
	peer := rr.peers[identity]
	peer.ASN = asn
	peer.Families = families
	peer.Capabilities = capabilities
	return nil
}

// dispatchText routes text-format events to handlers (fork-mode fallback).
func (rr *routeReflector) dispatchText(text string) error {
	if rr.stopping.Load() {
		return errors.New("bgp-rr: event arrived after reflector admission closed")
	}
	if envelope, matched := retention.AcceptsCleanup(text); matched {
		return rr.cleanupSource(envelope)
	}
	if envelope, matched := retention.Accepts(text); matched {
		return rr.retained.Record(envelope)
	}
	if matched, err := ribevents.ApplyFlowSpecGate(text); matched {
		return err
	}
	gate, matched, err := ribevents.DecodeValidationGate(text)
	if err != nil {
		return err
	}
	if matched {
		rr.mu.Lock()
		rr.receiveGate = gate.Command
		rr.mu.Unlock()
		ribevents.NoteValidationGate(gate)
		return nil
	}
	routes, matched, err := ribevents.DecodeValidationChange(text)
	if err != nil {
		return err
	}
	if matched {
		rr.validationChanged(routes)
		return nil
	}
	text, receipt, enveloped, err := bgptypes.UnwrapUpdateText(text)
	if err != nil {
		return err
	}
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 6 {
		return nil
	}
	if fields[0] != "peer" {
		return nil
	}
	if fields[2] != "remote" {
		return errors.New("bgp-rr: event has malformed remote AS header")
	}
	if fields[3] != "as" {
		return errors.New("bgp-rr: event has malformed remote AS header")
	}
	if fields[5] == eventState {
		return rr.textState(fields)
	}
	if len(fields) < 8 {
		return nil
	}
	if fields[5] != "received" {
		return nil
	}
	if fields[6] == eventOpen {
		return rr.textOpen(fields)
	}
	if fields[6] != eventUpdate {
		return nil
	}
	msgID, err := strconv.ParseUint(fields[7], 10, 64)
	if err != nil {
		return err
	}
	epoch, sourceReceipt, owner, connection, err := textparse.SourcePhysicalProvenance(text)
	if err != nil {
		return rr.rejectCached(msgID, err)
	}
	if enveloped {
		if err := receipt.Validate(fields[1], owner, msgID, epoch, sourceReceipt); err != nil {
			return rr.rejectCached(msgID, err)
		}
	}
	return rr.reflectText(fields[1], owner, connection, sourceReceipt, msgID, epoch, text)
}

// handleCommand processes command requests via SDK execute-command callback.
func (rr *routeReflector) handleCommand(command string) (string, any, error) {
	switch command {
	case "show rr status":
		return statusDone, map[string]any{"running": true}, nil
	case "show rr peers":
		return statusDone, rr.peerStatus(), nil
	default: // fail on unknown command
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	}
}

// peerStatus returns peer state.
func (rr *routeReflector) peerStatus() any {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rr.peers))
	for _, p := range rr.peers {
		row := map[string]any{
			"address": p.Address,
			"remote":  map[string]any{"as": p.ASN},
			"up":      p.Up,
		}
		// The families key is absent, never empty, while the peer is down or no
		// OPEN is recorded: a down entry may hold an earlier session's set.
		if p.Up && p.Families != nil {
			row["families"] = recordedFamilyNames(p.Families)
		}
		peers = append(peers, row)
	}

	return map[string]any{"peers": peers}
}

// recordedFamilyNames is the recorded family set as sorted family names.
func recordedFamilyNames(families map[family.Family]bool) []string {
	names := make([]string, 0, len(families))
	for f := range families {
		names = append(names, f.String())
	}
	slices.Sort(names)
	return names
}

// replayForPeer replays existing routes to a newly-connected peer, then sends
// End-of-RIB markers per negotiated family. Runs in a per-peer lifecycle
// goroutine (not blocking the event loop).
//
// The routes are held by the receive store (bgp-adj-rib-in), and this plugin
// relays them itself: it reads them page by page
// (ribevents.ReplayRoutesCommand) and relays each page under its own name. The
// destination grants `send [ update ]` to this plugin, the replay owner, and
// gives the store nothing, so a relay the store sent would be refused.
//
// The gen parameter guards against rapid reconnects: if the peer's ReplayGen
// has changed by the time replay finishes, this goroutine is stale.
// initialReplay MUST be the receipt captured by this replay's peer-up event.
func (rr *routeReflector) replayForPeer(parent context.Context, target peerTarget, initialReplay uint64, done chan struct{}) {
	defer rr.replays.finish(done)
	state, admitted := rr.pinPeerReplay(target)
	if !admitted {
		return
	}
	defer rr.finishPeerReplay(target.identity, state)
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	destination := target.identity.address.String()
	var relayFailure error
	relay := func(ctx context.Context, destination string, routes []rpc.StoredRoute) error {
		err := rr.relayToPeer(ctx, target, destination, routes)
		if err != nil {
			relayFailure = err
		}
		return err
	}
	progress, readFailure := ribevents.RelayReplay(ctx, rr.dispatchCommand, relay, destination, 0, "")
	replayed := progress.Replayed
	last := progress.LastIndex
	if readFailure == nil {
		for range replayConvergenceMax {
			if last == 0 {
				break
			}
			if !rr.targetCurrent(target) {
				return
			}
			delta, err := ribevents.RelayReplay(ctx, rr.dispatchCommand, relay, destination, last, "")
			replayed += delta.Replayed
			if err != nil {
				readFailure = err
				break
			}
			if delta.Replayed == 0 {
				break
			}
			last = delta.LastIndex
		}
	}
	if relayFailure != nil {
		rr.rememberReplay(target, initialReplay, relayFailure)
		return
	}
	if readFailure != nil {
		// An optional PAGE READ refuses no already joined output. In particular,
		// its expired context must not suppress mandatory selecting-RIB replay.
		rr.reportOptionalReplay(parent, target, readFailure)
	}
	if readFailure == nil {
		rr.reportOptionalReplay(parent, target, nil)
	}
	flowCount, failed := rr.replayFlowSpecs(parent, target)
	replayed += flowCount
	if failed != nil {
		rr.rememberReplay(target, initialReplay, failed)
		return
	}
	if replayed > 0 {
		failed = rr.sendEOR(parent, target)
	}
	if failed == nil {
		failed = rr.signalSessionReady(parent, target, initialReplay)
	}
	// These final writes are required. Their unknown completion cannot be
	// converted into a safe eligibility read retry or an optional-store warning.
	rr.noteTargetOutputFailure(target, "peer-UP final End-of-RIB and readiness", failed)
	rr.rememberReplay(target, initialReplay, failed)
}

// dispatchCommand sends a command to the engine via the SDK.
func (rr *routeReflector) dispatchCommand(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
	return rr.plugin.DispatchCommandArgs(ctx, command, args, "")
}

// sendEOR sends End-of-RIB markers for each of the peer's negotiated families.
func (rr *routeReflector) sendEOR(parent context.Context, target peerTarget) error {
	rr.mu.RLock()
	p := rr.peers[target.identity]
	if p == nil {
		rr.mu.RUnlock()
		return nil
	}
	if !p.Up {
		rr.mu.RUnlock()
		return nil
	}
	if p.ReplayGen != target.generation {
		rr.mu.RUnlock()
		return nil
	}
	if p.DestinationSession != target.session {
		rr.mu.RUnlock()
		return nil
	}
	families := make([]string, 0, len(p.Families))
	for fam := range p.Families {
		families = append(families, fam.String())
	}
	rr.mu.RUnlock()
	slices.Sort(families)
	for _, fam := range families {
		ctx, cancel := context.WithTimeout(parent, updateRouteTimeout)
		_, _, err := rr.plugin.UpdateRouteRequest(ctx, rpc.UpdateRouteInput{PeerSelector: target.identity.address.String(),
			Command: rrEORCmd(fam), Meta: map[string]any{bgptypes.ReplayEORMeta: true},
			DestinationOwner: target.identity.owner, DestinationSession: target.session})
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

// signalSessionReady reports completion of this plugin's initial replay so the
// engine can release live forwards held behind it. End-of-RIB is independent.
// The local generation check rejects known stale work; the captured receipt also
// fences a reconnect after that check at the command consumer.
// initialReplay MUST come from the replay's peer-up event, never a current-session lookup.
func (rr *routeReflector) signalSessionReady(parent context.Context, target peerTarget, initialReplay uint64) error {
	if initialReplay == 0 {
		return nil
	}
	if !rr.targetCurrent(target) {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, updateRouteTimeout)
	defer cancel()
	var text textbuf.Buffer
	command := text.Str("request peer ").Str(target.identity.address.String()).Str(" plugin session ready session ").Uint(initialReplay).String()
	_, _, err := rr.plugin.DispatchDestinationCommand(ctx, rpc.DispatchCommandInput{Command: command, DestinationOwner: target.identity.owner, DestinationSession: target.session})
	return err
}

// nlriDelCmd is one text withdrawal of prefixes, each a complete NLRI
// spelling ("prefix 10.0.0.0/24", or a labeled or VPN form with its
// keywords). Each one takes its own del keyword, the form bgp-rs sends. A
// comma-joined list ("del prefix 10.0.0.0/24,prefix 10.0.1.0/24") is read as
// the one prefix "10.0.0.0/24,prefix", so the engine refused every departure
// of more than one route and the client kept them all.
func nlriDelCmd(fam string, prefixes []string) string {
	var b textbuf.Buffer
	// RFC 4271 Section 4.3: IPv4 unicast is withdrawn in the Withdrawn Routes
	// field, ze's `nlri` section; every other family in MP_UNREACH_NLRI (`mp`).
	b.Reset().Str("update text")
	if textparse.LegacyNLRISection(fam, false) {
		b.Str(" nlri")
	} else {
		b.Str(" mp ").Str(fam)
	}
	for _, prefix := range prefixes {
		b.Str(" del ").Str(prefix)
	}
	return b.String()
}

func rrEORCmd(fam string) string {
	var b textbuf.Buffer
	// RFC 4724 Section 2: the IPv4 unicast End-of-RIB is an UPDATE of minimum
	// length (`nlri eor`); any other family's is an empty MP_UNREACH_NLRI.
	b.Reset().Str("update text")
	if textparse.LegacyNLRISection(fam, false) {
		b.Str(" nlri")
	} else {
		b.Str(" mp ").Str(fam)
	}
	return b.Str(" eor").String()
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show rr status", ShortHelp: "Show RR status"},
		{Name: "show rr peers", ShortHelp: "Show peer states"},
	}
}

// routeReflectorRegistration is the Stage 1 declaration bgp-rr sends from Run.
func routeReflectorRegistration() *sdk.Registration {
	return &sdk.Registration{
		CacheConsumer:          true,
		DoctorChecks:           []rpc.DoctorCheckDecl{replayDoctorDeclaration()},
		CacheConsumerUnordered: true,
		// SourcePolicyOwner: this plugin keeps per-source state for the peers
		// whose UPDATEs it receives (forwarded routes and the withdrawal map).
		// Run also sets it, with SourcePolicy, from the OnSourcePolicy handler
		// serveRouteReflector registers (source_policy.go); declaring it here
		// keeps the compiled declaration an owner even without that handler,
		// so the engine refuses rather than treats it as an observer
		// (RFC 8212 Section 3).
		SourcePolicyOwner:   true,
		Claims:              []string{"bgp-peer-up-replay", disseminationRole},
		Dissemination:       disseminationDecls(),
		SignalsSessionReady: true,
		FencesLiveForwards:  true,
		EventSettlement:     true,
		Commands:            commandDecls(),
		// The registry's edges, so a forked reflector takes a DOWN before
		// the stores it depends on, as an internal one does.
		Dependencies: rrDependencies,
	}
}
