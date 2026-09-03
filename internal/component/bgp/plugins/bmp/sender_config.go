// RFC: rfc/short/rfc8671.md
// Design: docs/architecture/core-design.md -- BMP plugin lifecycle
//
// Overview: bmp.go -- the plugin the configuration below is installed on
// Related: sender.go -- the per-collector session this file starts and stops
// Related: bmp_events.go -- bounceMonitoredPeers, the Peer Down/Peer Up pair a
// behavior change owes a session that stays up
//
// The BMP sender's configuration: what the `bgp bmp sender` subtree parses to,
// what ze compares one of them against another for, and the collector sessions
// that comparison starts, stops and leaves alone.

package bmp

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strconv"
)

// statisticsTimeoutOff is the YANG default of the statistics-timeout leaf: no
// periodic Statistics Report. Carried as the string the config tree delivers,
// because that is what senderConfig holds and what one configuration is
// compared against another as.
const statisticsTimeoutOff = "0"

// senderConfig holds parsed sender configuration from bgp { bmp { sender { ... } } }.
// YANG list with key is delivered as a map keyed by the key value.
type senderConfig struct {
	Collectors            map[string]collectorConfig `json:"collector"`
	RouteMonitoringPolicy string                     `json:"route-monitoring-policy"`
	RouteMirroring        string                     `json:"route-mirroring"`
	StatisticsTimeout     string                     `json:"statistics-timeout"`
	LocRIB                string                     `json:"loc-rib"` // RFC 9069 Loc-RIB monitoring (PeerType=3)

	// identity is the router's own ASN and router-id, read from `bgp router-id`
	// and `bgp session asn local` rather than from this subtree, and carried
	// here because it arrives on the same config rails and is installed by the
	// same apply. nil when the section carried neither leaf.
	//
	// Not a JSON field: parseSenderConfig fills it from the enclosing `bgp`
	// container (bgpSenderSection), which is where the two leaves live.
	identity *localIdentity
}

type collectorConfig struct {
	Address       string `json:"address"`
	Port          string `json:"port"`
	SourceAddress string `json:"source-address"`
}

// parseSenderConfig extracts BMP sender config from the bgp section JSON.
// The JSON is {"bgp": {"bmp": {"sender": {...}}}} (wrapped by ExtractConfigSubtree).
// Returns a default configuration (no collectors) when BMP sender is not configured.
//
// Every leaf the subtree leaves out is filled from its YANG default here, and
// nowhere else. A reload compares two parsed configurations to decide whether a
// BMP session's behavior changed (applySenderConfig), so a leaf the operator
// deleted has to arrive as the value ze will act under rather than as an empty
// string, or the comparison reads a deletion as a change and the sender acts on
// a value that is not the one in force.
func parseSenderConfig(data string) (*senderConfig, error) {
	var sec bgpSenderSection
	if err := json.Unmarshal([]byte(data), &sec); err != nil {
		return nil, fmt.Errorf("bmp sender config: %w", err)
	}
	if sec.BGP == nil {
		return defaultSenderConfig(), nil
	}

	identity := parseLocalIdentity(sec)

	if sec.BGP.BMP == nil || sec.BGP.BMP.Sender == nil {
		cfg := defaultSenderConfig()
		cfg.identity = identity
		return cfg, nil
	}
	cfg := sec.BGP.BMP.Sender
	cfg.identity = identity
	if cfg.RouteMonitoringPolicy == "" {
		cfg.RouteMonitoringPolicy = policyAll
	}
	if cfg.StatisticsTimeout == "" {
		cfg.StatisticsTimeout = statisticsTimeoutOff
	}
	return cfg, nil
}

// parseLocalIdentity reads the router's own ASN and router-id out of the `bgp`
// container, and returns nil when the section carries neither in a usable form.
//
// RFC 9069 Section 5.1 is what needs them: "Peer Autonomous System (AS): Set to
// the primary router BGP autonomous system number (ASN)", and "Peer BGP ID: Set
// the ID to the router-id of the VRF instance if VRF is used; otherwise, set to
// the global instance router-id." Both leaves are `mandatory true`, so a
// configured ze always answers; the nil is what a plugin that has been handed
// no `bgp` section yet returns, and it makes the Loc-RIB path DECLINE rather
// than emit a header the collector would read as AS 0 from router 0.0.0.0.
//
// The router-id is refused when it is zero or not IPv4: RFC 6286 Section 2.1
// defines the BGP Identifier as "a 4-octet, unsigned, non-zero integer", and
// the YANG leaf carries `ze:validate "nonzero-ipv4"` for the same reason. The
// ASN is refused when it is zero, which the `zt:asn` type also rejects. Both
// refusals return nil rather than a half-filled identity: an identity with one
// real field and one zero is the silently-wrong value this function exists to
// keep off the wire (ai/rules/principles.md).
func parseLocalIdentity(sec bgpSenderSection) *localIdentity {
	if sec.BGP.Session == nil || sec.BGP.Session.ASN == nil {
		return nil
	}
	asn, err := strconv.ParseUint(sec.BGP.Session.ASN.Local, 10, 32)
	if err != nil || asn == 0 {
		return nil
	}
	addr, err := netip.ParseAddr(sec.BGP.RouterID)
	if err != nil || !addr.Is4() {
		return nil
	}
	octets := addr.As4()
	routerID := binary.BigEndian.Uint32(octets[:])
	if routerID == 0 {
		return nil
	}
	return &localIdentity{asn: uint32(asn), routerID: routerID}
}

// defaultSenderConfig is the sender configuration ze runs under before any
// config arrives, and the one a `bgp` subtree carrying no BMP sender means: no
// collector, no Route Mirroring, no Loc-RIB monitoring, and the two leaves that
// have a non-empty YANG default at that default.
//
// It is what the plugin boots with, so the first configuration to arrive is
// compared against a real configuration rather than against an empty struct
// that names a policy ze never ran under.
func defaultSenderConfig() *senderConfig {
	return &senderConfig{
		RouteMonitoringPolicy: policyAll,
		StatisticsTimeout:     statisticsTimeoutOff,
	}
}

// senderBehavior is the part of a sender configuration that decides what an
// established BMP session carries. Two configurations with the same
// senderBehavior put the same messages on the same collector socket, so a
// reload between them alters no behavior and owes the collector nothing.
//
// A comparable struct rather than a hand-written field-by-field test: the
// failure mode is a fifth sender leaf that this struct does not carry, and one
// constructor is the one place a reader has to check for it.
type senderBehavior struct {
	policy     string
	mirroring  bool
	locRIB     bool
	statistics string
}

// identityChanged reports whether a reload moved the router's own identity.
//
// It is deliberately NOT a senderBehavior field. senderBehavior decides what
// every peer on a session carries, and a change to it bounces all of them
// (bounceMonitoredPeers). The identity decides what the LOC-RIB emulated peer
// carries and nothing else: a monitored BGP peer's per-peer header is built
// from that peer's own address and ASN (peerHeaderFromEvent), so bouncing the
// monitored peers because the router learned its own router-id would cost every
// collector a re-announcement of state the change did not touch.
//
// RFC 9069 Section 6.1.1 is why the Loc-RIB peer IS affected: "The BMP receiver
// identifies the Loc-RIB by the peer header distinguisher and BGP ID", so a new
// BGP ID is a new peer at the collector and Section 6.1.3 owes it the bounce.
func identityChanged(previous, current *senderConfig) bool {
	switch {
	case previous.identity == nil && current.identity == nil:
		return false
	case previous.identity == nil || current.identity == nil:
		return true
	default:
		return *previous.identity != *current.identity
	}
}

// behaviorOf reads the behavior a parsed sender configuration asks for.
// parseSenderConfig has already filled the absent leaves from their YANG
// defaults, so a leaf the operator deleted compares equal to the same leaf
// written at its default.
//
// statistics-timeout is carried although nothing reads it yet: the leaf is
// declared sender behavior, the periodic Statistics Report timer it configures
// is not implemented (plan/journal/unwired-feature.md, 2026-08-31), and a timer
// added later would otherwise change what a session carries with no bounce
// behind it.
func behaviorOf(cfg *senderConfig) senderBehavior {
	return senderBehavior{
		policy:     cfg.RouteMonitoringPolicy,
		mirroring:  cfg.RouteMirroring == yangTrue,
		locRIB:     cfg.LocRIB == yangTrue,
		statistics: cfg.StatisticsTimeout,
	}
}

// applySenderConfig installs one parsed sender configuration on the running
// plugin and tells the collectors what changed. Every config rail ends here:
// the Stage-2 configure callback at startup, and the config-apply and
// config-rollback callbacks of a reload that carries the `bgp` root
// (registerCallbacks).
//
// previous is the configuration in force and current the one replacing it. The
// two are compared rather than applied blind, because RFC 8671 Section 7.2
// binds a CHANGE: "In case of any change that results in the alteration of
// behavior of an existing BMP session (i.e., changes to filtering and table
// names), the session MUST be bounced with a Peer Down/Peer Up sequence." The
// plugin is handed the whole `bgp` root, so it is told about every neighbor,
// policy and timer the operator edits; acting on all of them would cost every
// collector a bounce for a change no collector can see.
//
// The two comparisons decide different things:
//
//   - The collector set decides which sessions exist. A collector the reload
//     removed, or that now points at another address, loses its session
//     (Termination, then close); a collector with no session gets one. A
//     collector the reload did not touch keeps its session, and keeps
//     everything the collector on the far end has learned on it.
//   - The behavior leaves decide what a surviving session carries, so a change
//     to one is the alteration Section 7.2 names. The BMP session stays up and
//     each peer reported on it is bounced instead (bounceMonitoredPeers).
//
// Loc-RIB monitoring is decided on both sides of that bounce, because its two
// halves belong on different sides of it. Turning it off owes the collectors
// the RFC 9069 Peer Down before their peers are re-announced. Turning it on
// subscribes and asks the RIB for a replay, which reaches every session that is
// connected by then. Both calls are no-ops at startup, where nothing is
// subscribed and no Loc-RIB Peer Up has been sent.
func (bp *BMPPlugin) applySenderConfig(previous, current *senderConfig) {
	behaviorChanged := behaviorOf(previous) != behaviorOf(current)
	movedIdentity := identityChanged(previous, current)
	if !behaviorChanged && !movedIdentity && maps.Equal(previous.Collectors, current.Collectors) {
		return
	}

	// RFC 9069 Section 6.1.3: "In case of any change that results in the
	// alteration of behavior of an existing BMP session, i.e., changes to
	// filtering and table names, the session MUST be bounced with a Peer Down /
	// Peer Up sequence." bounceMonitoredPeers below does that for the monitored
	// BGP peers; the Loc-RIB emulated peer is not one of them, so its half of
	// the bounce is here. Monitoring switched OFF owes the Peer Down and no
	// Peer Up; monitoring that STAYS on across a behavior change owes both,
	// which is what giving the per-session guard back arranges: the Peer Up is
	// re-sent below, under the identity and the policy now in force.
	locRIBBounce := current.LocRIB == yangTrue && (behaviorChanged || movedIdentity)
	if current.LocRIB != yangTrue {
		bp.stopLocRIB()
	}
	if current.LocRIB != yangTrue || locRIBBounce {
		bp.sendLocRIBPeerDown()
	}

	// The new identity is installed AFTER that Peer Down and BEFORE the Peer Up
	// that follows it, and the order is what makes the bounce a bounce. RFC 9069
	// Section 6.1.1 has the collector identify the Loc-RIB "by the peer header
	// distinguisher and BGP ID", so a Peer Down carrying the incoming router-id
	// would name a peer the collector never had up, and the peer it does have up
	// would stay up for ever.
	//
	// Reaching here means the identity is either unchanged or part of what
	// changed: movedIdentity is one of the three conditions the early return
	// above tests, so an identity change never takes it.
	bp.setLocalIdentity(current.identity)

	bp.setSenderPolicy(current.RouteMonitoringPolicy, current.RouteMirroring == yangTrue)
	kept := bp.syncSenders(current)

	if behaviorChanged {
		bp.bounceMonitoredPeers(kept)
	}

	// RFC 9069 Loc-RIB monitoring: subscribe to best-change once the sessions
	// exist. startLocRIB is idempotent, so a reload that leaves monitoring on
	// keeps its one subscription and asks for no second replay; a session this
	// reload started asks for its own dump when it connects (syncSenders,
	// ss.onConnected).
	if current.LocRIB == yangTrue {
		bp.startLocRIB()
		if locRIBBounce {
			// startLocRIB is idempotent, so a subscription that survived the
			// reload asks for no replay and nothing else would re-announce the
			// peer the Peer Down above took away.
			bp.ensureLocRIBPeerUp(kept)
		}
	}
}

// syncSenders reconciles the live collector sessions with cfg and reports the
// sessions that survived, in the order they were held in.
//
// A collector cfg still names at the same address keeps its session, so the BMP
// session on that socket CONTINUES -- which is the premise RFC 8671 Section 7.2
// rests on, and the reason the returned sessions are the ones owed a peer
// bounce. A collector cfg no longer names, or that now points somewhere else,
// has its session stopped: senderSession.stop writes the RFC 7854 Section 4.5
// Termination and closes the socket, which ends that session rather than
// altering it. A collector with no session gets a new one.
//
// Idempotent: called with the configuration already in force it starts nothing,
// stops nothing, and returns every session unchanged.
//
// The sessions to stop are collected under bp.mu and stopped outside it,
// because stop() can wait on a collector's socket and no other plugin path
// should have to wait behind it.
func (bp *BMPPlugin) syncSenders(cfg *senderConfig) []*senderSession {
	bp.mu.Lock()

	kept := make([]*senderSession, 0, len(cfg.Collectors))
	var stale []*senderSession
	for _, ss := range bp.senders {
		col, named := cfg.Collectors[ss.name]
		if named && ss.targets(col) {
			kept = append(kept, ss)
			continue
		}
		stale = append(stale, ss)
	}

	// A fresh slice rather than an append to bp.senders: the old slice is
	// handed to event handlers under a read lock and is read without one after
	// that (handleStructuredEvent), so it must never be written in place.
	live := slices.Clone(kept)
	for name, col := range cfg.Collectors {
		if slices.ContainsFunc(kept, func(ss *senderSession) bool { return ss.name == name }) {
			continue
		}
		ss := newSenderSession(name, col)
		// Every connection this session makes is a NEW BMP session and starts
		// from scratch: Peer Up for the peers that are up (queued in the same
		// critical section that publishes the connection, so nothing precedes
		// them), then a full fresh dump.
		ss.onPrimed = func() { bp.primeSender(ss) }
		ss.onConnected = func() { bp.requestLocRIBDump(ss) }
		live = append(live, ss)
		bp.sessions.Go(ss.run)
		logger().Info("bmp: sender started", "collector", name, "address", col.Address, "port", col.Port)
	}
	bp.senders = live
	bp.mu.Unlock()

	for _, ss := range stale {
		ss.stop()
		logger().Info("bmp: sender stopped", "collector", ss.name, "reason", "collector no longer configured at this address")
	}

	return kept
}

// setSenderPolicy publishes the two config leaves that decide what a reactor
// event produces: which direction is streamed as Route Monitoring, and whether
// Route Mirroring is on.
//
// Both are published in ONE write lock because handleStructuredEvent snapshots
// them together: an event must be processed under a single configuration, never
// under the policy from one and the mirroring flag from the next.
func (bp *BMPPlugin) setSenderPolicy(policy string, mirroring bool) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.routeMonitorPolicy = policy
	bp.routeMirroring = mirroring
}

// stopSenders stops all sender sessions.
//
// The session list is detached under the lock and the sessions are stopped
// outside it: stop() now writes the RFC 7854 Section 4.5 Termination message
// before closing, and no other plugin path should have to wait behind a
// collector's socket for that.
func (bp *BMPPlugin) stopSenders() {
	bp.mu.Lock()
	senders := bp.senders
	bp.senders = nil
	bp.mu.Unlock()

	for _, ss := range senders {
		ss.stop()
	}
}
