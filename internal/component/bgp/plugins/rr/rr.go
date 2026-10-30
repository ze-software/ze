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
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/selector"
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
	Address      string
	ASN          uint32
	Up           bool
	ReplayGen    uint64 // Incremented on each state-up, guards stale goroutines
	Families     map[family.Family]bool
	Capabilities map[string]bool
}

// routeReflector implements a BGP Route Reflector plugin (RFC 4456).
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles RFC 4456 forwarding rules, ORIGINATOR_ID injection,
// CLUSTER_LIST prepend, and next-hop rewriting.
type routeReflector struct {
	plugin   *sdk.Plugin
	peers    map[string]*peerState
	mu       sync.RWMutex
	buf      textbuf.Buffer
	stopping atomic.Bool

	// withdrawalMu protects the withdrawals map.
	withdrawalMu sync.Mutex
	// withdrawals tracks announced routes per source peer for withdrawal on peer-down.
	// sourcePeer -> routeKey (family|prefix) -> withdrawalInfo.
	withdrawals map[string]map[string]withdrawalInfo
	// sourcePolicy holds each source's open source-policy transaction
	// (source_policy.go), keyed like withdrawals and guarded by withdrawalMu.
	sourcePolicy map[string]*sourcePolicyTxn
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
	departures        *syncutil.Coalescer[netip.Addr, map[string]withdrawalInfo]
	departuresStopped bool
	// departing counts each source's departures queued or running, so a
	// source-policy prepare can wait for them (awaitDepartures). merged is
	// set by the Coalescer's merge, which runs inside Enqueue, so an enqueue
	// merged into a waiting departure adds no count. departureMu guards both.
	departing map[netip.Addr]*departureWait
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

	// handleStateDown asks retention.Family whether bgp-gr retains a departed
	// source's family. That is a same-process call: bgp-gr publishes its
	// answer into the daemon's memory, so a forked bgp-rr finds no owner and
	// reads every family as not retained. It would then send the departure
	// recovery for routes RFC 4724 Section 4.2 requires kept, with no error
	// and no log line. Refuse to start rather than answer wrong.
	if !p.IsInternal() {
		logger().Error("bgp-rr: refusing to start as an external plugin process -- " +
			"the graceful-restart retention decision (retention.Family) is a same-process call, " +
			"so a forked route reflector would withdraw routes a restarting peer's neighbors must keep; " +
			"configure it as plugin { internal <name> { use bgp-rr } }")
		return 1
	}
	return serveRouteReflector(p)
}

// serveRouteReflector registers the reflector's callbacks on p and runs it
// until the connection ends. runRouteReflector's process check comes first:
// only a test of the SDK callbacks over the wire transport calls it alone.
func serveRouteReflector(p *sdk.Plugin) int {
	downCtx, stopDown := context.WithCancel(context.Background())
	rr := &routeReflector{
		plugin:      p,
		peers:       make(map[string]*peerState),
		withdrawals: make(map[string]map[string]withdrawalInfo),
		downCtx:     downCtx,
	}
	defer rr.stopDepartures()
	defer stopDown()

	// Register structured event handler for DirectBridge delivery (hot path).
	p.OnStructuredEvent(func(events []any) error {
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
					rr.forwards.enter()
					rr.withdrawalMu.Lock()
					rr.updateWithdrawalMapWire(se.PeerAddress, msg)
					rr.withdrawalMu.Unlock()
					rr.forwardUpdate(msg.MessageID)
					rr.forwards.leave()
				}
			case rpc.EventKindState:
				rr.handleStructuredState(se)
			case rpc.EventKindOpen:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					rr.handleStructuredOpen(se, msg)
				}
			case rpc.EventKindUnspecified, rpc.EventKindNotification, rpc.EventKindKeepalive,
				rpc.EventKindRefresh, rpc.EventKindEOR, rpc.EventKindBoRR, rpc.EventKindEoRR,
				rpc.EventKindSent, rpc.EventKindNegotiated, rpc.EventKindCount:
				// These events do not trigger reflection.
			default:
				// The plugin event set is open; unknown events do not trigger reflection.
			}
		}
		return nil
	})

	// Register text event handler for fork-mode delivery (fallback).
	p.OnEvent(func(eventStr string) error {
		rr.dispatchText(eventStr)
		return nil
	})

	// RFC 8212 Section 3: the source-policy lifecycle owner (source_policy.go).
	p.OnSourcePolicy(rr.handleSourcePolicy)

	// Register command handler.
	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		return rr.handleCommand(command)
	})

	// Startup ownership comes from Registration.Claims. Mid-life auto-load
	// and restart deliver this callback to notify an already-running store.
	p.OnAllPluginsReady(func() error {
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

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer rr.stopping.Store(true)
	if bus := validationBus.Load(); bus != nil {
		stop := rr.startValidation(ctx, *bus)
		defer stop()
	}
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
func (rr *routeReflector) forwardUpdate(msgID uint64) {
	if rr.stopping.Load() {
		return
	}
	rr.updateRoute("*", rr.buf.Reset().Str("cached ").Uint(msgID).String())
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
func (rr *routeReflector) recoverRouteSel(sel *selector.Selector, command, source string, generation uint64) {
	ctx := rr.downCtx
	if ctx == nil {
		// Struct-literal unit owners have no running plugin lifecycle.
		ctx = context.Background()
	}
	meta := map[string]any{"recovery-source": source,
		"recovery-generation": strconv.FormatUint(generation, 10)}
	_, _, err := rr.plugin.UpdateRouteSelWithMeta(ctx, sel, command, meta)
	if err != nil { //nolint:gocritic // ifElseChain: switch blocked by block-silent-ignore hook
		if rr.stopping.Load() {
			logger().Debug("update-route failed (shutting down)",
				"peer", sel, "command", command, "error", err)
		} else if isConnectionError(err) {
			logger().Warn("update-route failed (peer disconnected)",
				"peer", sel, "command", command, "error", err)
		} else {
			logger().Error("update-route failed",
				"peer", sel, "command", command, "error", err)
		}
	}
}

// isConnectionError reports whether err indicates the target peer's connection is closed.
func isConnectionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

// handleStructuredState processes state events from DirectBridge.
func (rr *routeReflector) handleStructuredState(se *rpc.StructuredEvent) {
	if se.PeerAddress == "" {
		return
	}

	rr.mu.Lock()
	if rr.peers[se.PeerAddress] == nil {
		rr.peers[se.PeerAddress] = &peerState{Address: se.PeerAddress}
	}
	peer := rr.peers[se.PeerAddress]
	peer.Up = (se.State == rpc.SessionStateUp)
	// A dynamic peer's AS is unknown before its OPEN is negotiated: keep the
	// last one learned rather than recording AS 0.
	if asn, known := se.PeerAS.ASN(); known {
		peer.ASN = asn
	}
	switch se.State {
	case rpc.SessionStateUp:
		if se.Reason == rpc.ReasonAttached {
			// A reload attached this plugin to a session already up: the
			// peer's initial routing update was sent without it, and is not
			// this plugin's to replay.
			rr.mu.Unlock()
			return
		}
		peer.ReplayGen++
		gen := peer.ReplayGen
		rr.mu.Unlock()
		go rr.replayForPeer(se.PeerAddress, gen, se.InitialReplay)
	case rpc.SessionStateDown:
		rr.mu.Unlock()
		rr.handleStateDown(se.PeerAddress, se.SessionGeneration)
	case rpc.SessionStateUnspecified, rpc.SessionStateCount:
		rr.mu.Unlock()
	default:
		// The plugin state set is open; unknown states need no further action.
		rr.mu.Unlock()
	}
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
func (rr *routeReflector) handleStateDown(peerAddr string, generation uint64) {
	rr.withdrawalMu.Lock()
	entries := rr.withdrawals[peerAddr]
	delete(rr.withdrawals, peerAddr)
	rr.dropSourcePolicyLocked(peerAddr)
	rr.withdrawalMu.Unlock()

	if len(entries) == 0 {
		return
	}

	// RFC 4724 Section 4.2: "it MUST retain the routes received from the
	// peer for all the address families that were previously received in
	// the Graceful Restart Capability". Resolve retention here,
	// synchronously, as bgp-rs does: a reconnect or expiry must not change
	// this DOWN decision before the departure worker sends it. The RIB owns a
	// retained family's routes and their eventual purge, so a retained route
	// MUST NOT reach the engine as a departure: the recovery would withdraw
	// it from every destination during the restart window.
	retained := make(map[family.Family]bool)
	for key, info := range entries {
		keep, seen := retained[info.Family]
		if !seen {
			keep = retention.Family(peerAddr, info.Family)
			retained[info.Family] = keep
		}
		if keep {
			delete(entries, key)
			continue
		}
		info.Generation = generation
		entries[key] = info
	}

	if len(entries) == 0 {
		// Every route belonged to a retained family.
		return
	}

	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		logger().Error("invalid peer address in withdrawal", "peer", peerAddr, "error", err)
		return
	}
	if !rr.enqueueDeparture(addr, entries) {
		logger().Debug("peer-down after the departure worker stopped (plugin stopping)", "peer", peerAddr)
	}
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
func (rr *routeReflector) enqueueDeparture(source netip.Addr, entries map[string]withdrawalInfo) bool {
	rr.departureMu.Lock()
	defer rr.departureMu.Unlock()
	if rr.departuresStopped {
		return false
	}
	if rr.departures == nil {
		rr.departures = syncutil.NewCoalescer(departureWorkers,
			func(waiting, next map[string]withdrawalInfo) map[string]withdrawalInfo {
				// Runs inside Enqueue, under departureMu (merged).
				rr.merged = true
				maps.Copy(waiting, next)
				return waiting
			},
			rr.sendDeparture,
		)
	}
	rr.merged = false
	if !rr.departures.Enqueue(source, entries) {
		return false
	}
	if rr.merged {
		return true
	}
	if rr.departing == nil {
		rr.departing = make(map[netip.Addr]*departureWait)
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
func (rr *routeReflector) sendPeerDown(source netip.Addr, entries map[string]withdrawalInfo) {
	byGroup := make(map[withdrawalGroup][]string)
	for _, info := range entries {
		group := withdrawalGroup{family: info.Family, wireForm: info.WireForm, addPath: info.AddPath,
			generation: info.Generation}
		byGroup[group] = append(byGroup[group], info.Prefix)
	}

	excludeSel := selector.ExcludeAddr(source)
	recoverySource := source.String()
	var command textbuf.Buffer
	for group, prefixes := range byGroup {
		// Map iteration is unordered; sorting makes one down emit one byte
		// sequence. Cold path, once per peer-down.
		slices.Sort(prefixes)
		for i := 0; i < len(prefixes); i += withdrawalBatchSize {
			end := min(i+withdrawalBatchSize, len(prefixes))
			if !group.wireForm {
				rr.recoverRouteSel(excludeSel, nlriDelCmd(group.family.String(), prefixes[i:end]), recoverySource, group.generation)
				continue
			}
			// RFC 8277 Section 2.4: "When using an MP_UNREACH_NLRI
			// attribute to withdraw a route whose NLRI was previously
			// specified in an MP_REACH_NLRI attribute, the lengths and
			// values of the respective prefixes must match, and the
			// respective AFI/SAFIs must match." Native bytes keep RD and
			// prefix; RFC 7911 framing keeps the source's identifier,
			// including zero, which the engine maps to the destination's.
			command.Reset().Str("update hex nlri ").Str(group.family.String())
			if group.addPath {
				command.Str(" addpath")
			}
			for _, prefix := range prefixes[i:end] {
				command.Str(" del ").Str(prefix)
			}
			rr.recoverRouteSel(excludeSel, command.String(), recoverySource, group.generation)
		}
	}
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
func (rr *routeReflector) handleStructuredOpen(se *rpc.StructuredEvent, msg *bgptypes.RawMessage) {
	if se.PeerAddress == "" || msg.RawBytes == nil {
		return
	}

	open, err := message.UnpackOpen(msg.RawBytes)
	if err != nil {
		logger().Warn(rrOpenUnreadable, "peer", se.PeerAddress, "error", err)
		return
	}
	// RFC 9072 Section 2: the OPEN's own framing decides the Parameter Length width.
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		logger().Warn(rrOpenUnreadable, "peer", se.PeerAddress, "error", err)
		return
	}

	asn := uint32(open.MyAS)
	families := make(map[family.Family]bool)
	capabilities := make(map[string]bool)
	hasMP := false
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok {
			hasMP = true
			capabilities["multiprotocol"] = true
			families[family.Family{AFI: mp.AFI, SAFI: mp.SAFI}] = true
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

	rr.mu.Lock()
	defer rr.mu.Unlock()

	if rr.peers[se.PeerAddress] == nil {
		rr.peers[se.PeerAddress] = &peerState{Address: se.PeerAddress}
	}
	peer := rr.peers[se.PeerAddress]
	peer.ASN = asn
	peer.Families = families
	peer.Capabilities = capabilities
}

// dispatchText routes text-format events to handlers (fork-mode fallback).
func (rr *routeReflector) dispatchText(text string) {
	if rr.stopping.Load() {
		return
	}

	// Quick parse: "peer <addr> remote as <n> <dir> <type> <id> ..."
	// State:       "peer <addr> remote as <n> state <state>"
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 6 {
		return
	}

	// fields[5] is either "state" or the direction token.
	if fields[5] == eventState {
		if len(fields) >= 7 {
			peerAddr := fields[1]
			state := fields[6]
			var initialReplay, sessionGeneration uint64
			attached := false
			for i := 7; i < len(fields); i++ {
				if fields[i] == "reason" && i+1 < len(fields) && fields[i+1] == rpc.ReasonAttached {
					attached = true
				}
				if fields[i] != "initial-replay" && fields[i] != "session-generation" {
					continue
				}
				keyword := fields[i]
				i++
				if i == len(fields) {
					return
				}
				token, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					return
				}
				if keyword == "session-generation" {
					sessionGeneration = token
					continue
				}
				initialReplay = token
			}

			rr.mu.Lock()
			if rr.peers[peerAddr] == nil {
				rr.peers[peerAddr] = &peerState{Address: peerAddr}
			}
			peer := rr.peers[peerAddr]
			peer.Up = (state == "up")
			switch state {
			case "up":
				if attached {
					// handleStructuredState's reason: no replay on an attached up.
					rr.mu.Unlock()
					return
				}
				peer.ReplayGen++
				gen := peer.ReplayGen
				rr.mu.Unlock()
				go rr.replayForPeer(peerAddr, gen, initialReplay)
			case "down":
				rr.mu.Unlock()
				rr.handleStateDown(peerAddr, sessionGeneration)
			default: // other states (e.g. "connected")
				rr.mu.Unlock()
			}
		}
		return
	}

	// Message events: fields[5]=direction [6]=type [7]=id
	if len(fields) < 8 {
		return
	}

	if fields[6] == eventUpdate {
		msgID, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			return
		}
		sourcePolicyEpoch, err := textparse.SourcePolicyEpoch(text)
		if err != nil {
			logger().Warn("received UPDATE provenance rejected", "peer", fields[1], "error", err)
			ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
			if releaseErr := rr.plugin.ReleaseCached(ctx, []uint64{msgID}); releaseErr != nil {
				logger().Warn("rejected UPDATE cache release failed", "message-id", msgID, "error", releaseErr)
			}
			cancel()
			return
		}
		// Update withdrawal map from text before forwarding.
		peerAddr := fields[1]
		ops := parseTextNLRIOps(text)
		rr.forwards.enter()
		if len(ops) > 0 {
			rr.withdrawalMu.Lock()
			rr.updateWithdrawalMapText(peerAddr, ops, sourcePolicyEpoch)
			rr.withdrawalMu.Unlock()
		}
		rr.forwardUpdate(msgID)
		rr.forwards.leave()
	}
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
func (rr *routeReflector) replayForPeer(peerAddr string, gen, initialReplay uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The mandatory selecting RIB owns FlowSpec replay independently of the
	// optional store. Complete it before EOR and the readiness report on both
	// successful and failed optional-store replay.
	var replayed int
	defer func() {
		replayed += rr.replayFlowSpecs(peerAddr, gen)
		if replayed > 0 {
			rr.sendEOR(peerAddr, gen)
		}
		rr.signalSessionReady(peerAddr, gen, initialReplay)
	}()

	// Full replay from store index 0.
	progress, err := ribevents.RelayReplay(ctx, rr.dispatchCommand, rr.plugin.RelayStoredRoute, peerAddr, 0, "")
	replayed = progress.Replayed
	if err != nil {
		// Optional-store failure does not prevent mandatory FlowSpec replay.
		logger().Warn("replay failed", "peer", peerAddr, "replayed", progress.Replayed, "error", err)
		return
	}
	lastIndex := progress.LastIndex

	// Convergent delta replay: catch routes adj-rib-in received after the
	// full replay snapshot (race between event delivery and replay).
	for i := range replayConvergenceMax {
		if lastIndex == 0 {
			break
		}
		if i > 0 {
			time.Sleep(replayConvergenceDelay)
		}
		delta, deltaErr := ribevents.RelayReplay(ctx, rr.dispatchCommand, rr.plugin.RelayStoredRoute, peerAddr, lastIndex, "")
		replayed += delta.Replayed
		if deltaErr != nil {
			logger().Warn("delta replay failed", "peer", peerAddr, "attempt", i, "error", deltaErr)
			break
		}
		if delta.Replayed == 0 {
			break
		}
		logger().Debug("delta replay caught new routes", "peer", peerAddr, "attempt", i, "replayed", delta.Replayed)
		lastIndex = delta.LastIndex
	}
}

// dispatchCommand sends a command to the engine via the SDK.
func (rr *routeReflector) dispatchCommand(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
	return rr.plugin.DispatchCommandArgs(ctx, command, args, "")
}

// sendEOR sends End-of-RIB markers for each of the peer's negotiated families.
func (rr *routeReflector) sendEOR(peerAddr string, gen uint64) {
	rr.mu.RLock()
	p := rr.peers[peerAddr]
	if p == nil || p.ReplayGen != gen || len(p.Families) == 0 {
		rr.mu.RUnlock()
		return
	}
	families := make([]string, 0, len(p.Families))
	for f := range p.Families {
		families = append(families, f.String())
	}
	rr.mu.RUnlock()

	slices.Sort(families)
	for _, fam := range families {
		// RFC 4724 Section 2: the marker indicates "the completion of the
		// initial routing update". The replay-eor meta has the engine send it
		// only when this session has not already carried the family's marker
		// from the reactor's own initial sync, so the replay does not repeat it.
		rr.updateRouteWithMeta(peerAddr, rrEORCmd(fam), map[string]any{bgptypes.ReplayEORMeta: true})
	}
	logger().Info("sent EOR", "peer", peerAddr, "families", families)
}

// signalSessionReady reports completion of this plugin's initial replay so the
// engine can release live forwards held behind it. End-of-RIB is independent.
// The local generation check rejects known stale work; the captured receipt also
// fences a reconnect after that check at the command consumer.
// initialReplay MUST come from the replay's peer-up event, never a current-session lookup.
func (rr *routeReflector) signalSessionReady(peerAddr string, gen, initialReplay uint64) {
	if initialReplay == 0 {
		logger().Warn("cannot report replay readiness without the peer-up session receipt", "peer", peerAddr)
		return
	}
	rr.mu.RLock()
	p := rr.peers[peerAddr]
	stale := p == nil || p.ReplayGen != gen
	rr.mu.RUnlock()
	if stale {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()
	var tb textbuf.Buffer
	command := tb.Str("request peer ").Str(peerAddr).Str(" plugin session ready session ").Uint(initialReplay).String()
	if _, _, err := rr.plugin.DispatchCommand(ctx, command); err != nil {
		logger().Warn("plugin session ready failed; this peer's live forwards remain fenced",
			"peer", peerAddr, "error", err)
	}
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
		CacheConsumerUnordered: true,
		// SourcePolicyOwner: this plugin keeps per-source state for the peers
		// whose UPDATEs it receives (forwarded routes and the withdrawal map).
		// Run also sets it, with SourcePolicy, from the OnSourcePolicy handler
		// serveRouteReflector registers (source_policy.go); declaring it here
		// keeps the compiled declaration an owner even without that handler,
		// so the engine refuses rather than treats it as an observer
		// (RFC 8212 Section 3).
		SourcePolicyOwner: true,
		Commands:          commandDecls(),
	}
}
