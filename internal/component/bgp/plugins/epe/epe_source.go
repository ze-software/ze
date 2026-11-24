// Design: docs/architecture/wire/nlri-bgpls.md -- live EPE segment producer
// RFC: rfc/short/rfc9086.md -- instantiated PeerNode SIDs

package epe

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/bgp/nlri/bgpls"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

var epeSnapshots = linkstateevents.RegisterSource(Name)
var epeGeneration atomic.Uint64

const epeMPLSSource uint16 = 5

type epeSession struct {
	local        linkstateevents.NodeID
	remote       linkstateevents.NodeID
	localAddress netip.Addr
}

// Source instantiates the PeerNode SIDs of the configured peers: it installs one
// MPLS label for each established session and publishes the matching link-state
// snapshot. Safe for concurrent use.
type Source struct {
	mu        sync.Mutex
	bus       ze.EventBus
	config    Config
	sessions  map[netip.Addr]epeSession
	installed map[netip.Addr]uint32
	stopped   bool
	acquired  bool
	retired   bool // Both native labels and the last advertised snapshot are cleared.
	// refusals holds each label the data plane disagrees with EPE about (an
	// add no writer installed, a removal no writer carried out), with the
	// writer's cause, until the data plane holds what EPE wants. The
	// bgp-epe-label-entries doctor check reports it (epe_doctor_label.go).
	refusals mplsfib.Refusals
}

// NewSource returns a Source that emits labels and snapshots on bus.
func NewSource(bus ze.EventBus) *Source {
	return &Source{bus: bus, sessions: make(map[netip.Addr]epeSession), installed: make(map[netip.Addr]uint32)}
}

// Configure replaces the configuration and republishes the snapshot.
func (s *Source) Configure(config Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	if s.stopped {
		s.acquired = false
	}
	s.stopped, s.retired = false, false
	return s.publishLocked()
}

// State applies one BGP peer state event and republishes the snapshot.
func (s *Source) State(event *bgp.Event) error {
	peer, err := netip.ParseAddr(event.GetPeerAddress())
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.GetPeerState() != "up" {
		delete(s.sessions, peer)
		return s.publishLocked()
	}
	var identity struct {
		bgp.PeerInfoJSON
		RouterID string `json:"router-id"`
	}
	if err := json.Unmarshal(event.Peer, &identity); err != nil {
		return err
	}
	if identity.Local == nil {
		return errors.New("EPE state event has no local identity")
	}
	localID, err := netip.ParseAddr(identity.RouterID)
	if err != nil || !localID.Is4() || localID.IsUnspecified() {
		return errors.New("EPE local BGP identifier missing or invalid")
	}
	remoteID, err := netip.ParseAddr(identity.Remote.RouterID)
	if err != nil || !remoteID.Is4() || remoteID.IsUnspecified() {
		return errors.New("EPE remote BGP identifier missing or invalid")
	}
	localAddress, err := netip.ParseAddr(identity.Local.Address)
	if err != nil {
		return err
	}
	if identity.Local.AS == 0 || identity.Remote.AS == 0 {
		return errors.New("EPE state event has reserved AS zero")
	}
	s.sessions[peer] = epeSession{local: linkstateevents.NodeID{ASN: identity.Local.AS, BGPRouterID: localID},
		remote: linkstateevents.NodeID{ASN: identity.Remote.AS, BGPRouterID: remoteID}, localAddress: localAddress}
	return s.publishLocked()
}

// Replay publishes the current snapshot again.
func (s *Source) Replay() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publishLocked()
}

// Stop removes the installed labels and publishes an empty snapshot.
func (s *Source) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.acquired = false
	}
	s.stopped = true
	err := s.publishLocked()
	if err == nil {
		s.retired = true
	}
	return err
}

// liveApply applies a live (token 0) batch to every active label writer.
func (s *Source) liveApply(entries []mplsfib.Entry) error {
	return mplsfib.Apply(s.bus, entries)
}

// publishLocked withdraws obsolete label assignments before installing their
// replacements. The snapshot describes only labels acknowledged by the native
// FIB owner, never hypothetical SIDs attached to ordinary BGP sessions.
func (s *Source) publishLocked() error {
	return s.publishWithApplyLocked(s.liveApply)
}

// publishWithApplyLocked reconciles label assignments and emits the link-state
// snapshot using applyFn for every mpls-fib batch. Live callers pass liveApply
// (token 0); an unacquired ReplayLabels passes ApplyReplay tagged with the
// asking writer's token so the cleanup and initial adds reach that writer
// alone, never a displaced writer still subscribed during a reload window.
// Caller holds s.mu.
func (s *Source) publishWithApplyLocked(applyFn func([]mplsfib.Entry) error) error {
	if s.retired {
		// A successful bye may race the next producer's startup. The deferred
		// stop must not reset labels that the new instance has since acquired.
		return nil
	}
	desired := s.desiredLocked()
	var failures []error
	// The native FIB owner outlives this source object. Reclaim its retained
	// labels before any fresh assignment, including after an error exit.
	if !s.acquired {
		cleanup := []mplsfib.Entry{{Action: mplsfib.ActionRemoveLabelSource, Source: epeMPLSSource}}
		if err := applyFn(cleanup); err != nil {
			s.recordUnacquiredLocked(desired, err)
			snapshot := linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP, Instance: 1}, Generation: epeGeneration.Add(1)}
			_, publishErr := epeSnapshots.Emit(s.bus, &snapshot)
			return errors.Join(err, publishErr)
		}
		clear(s.installed)
		s.acquired = true
	}
	for peer, label := range s.installed {
		if current, retained := desired[peer]; retained && current == label {
			continue
		}
		if err := s.emitLabelWith(applyFn, mplsfib.ActionRemove, label, peer); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(s.installed, peer)
	}
	// A label retained after a failed removal still forwards to its old peer.
	// Reassignment must wait for that removal, including across peer keys.
	occupied := make(map[uint32]netip.Addr, len(s.installed))
	for peer, label := range s.installed {
		occupied[label] = peer
	}
	desiredPeers := make([]netip.Addr, 0, len(desired))
	for peer := range desired {
		desiredPeers = append(desiredPeers, peer)
	}
	slices.SortFunc(desiredPeers, netip.Addr.Compare)
	for _, peer := range desiredPeers {
		label := desired[peer]
		// A failed removal still owns the old label. Do not lose that cleanup
		// obligation by overwriting it with a successful replacement.
		if _, installed := s.installed[peer]; installed {
			continue
		}
		if owner, retained := occupied[label]; retained {
			failures = append(failures, fmt.Errorf("EPE label %d still belongs to peer %s", label, owner))
			continue
		}
		if err := s.emitLabelWith(applyFn, mplsfib.ActionAdd, label, peer); err != nil {
			failures = append(failures, err)
			continue
		}
		s.installed[peer] = label
		occupied[label] = peer
	}
	s.forgetSettledLocked(desired)
	if err := s.emitSnapshotLocked(desired); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

// emitSnapshotLocked builds and emits the link-state snapshot for every peer
// whose desired label is currently recorded in s.installed. Caller holds s.mu.
func (s *Source) emitSnapshotLocked(desired map[netip.Addr]uint32) error {
	snapshot := linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP, Instance: 1}, Generation: epeGeneration.Add(1)}
	peers := make([]netip.Addr, 0, len(desired))
	for peer, label := range desired {
		if installed, present := s.installed[peer]; present && installed == label {
			peers = append(peers, peer)
		}
	}
	slices.SortFunc(peers, netip.Addr.Compare)
	nodes := make(map[[8]byte]struct{})
	for _, peer := range peers {
		session := s.sessions[peer]
		config := s.config.Peers[peer]
		id := session.local.BGPRouterID.As4()
		var key [8]byte
		binary.BigEndian.PutUint32(key[:4], session.local.ASN)
		copy(key[4:], id[:])
		if _, exists := nodes[key]; !exists {
			nodes[key] = struct{}{}
			capabilities := bgpls.LsSRCapabilities{Ranges: []bgpls.LsSrLabelRange{{Range: s.config.Size, FirstSID: s.config.Base, SIDLen: 3}}}
			wire := make([]byte, capabilities.Len())
			capabilities.WriteTo(wire, 0)
			snapshot.Nodes = append(snapshot.Nodes, linkstateevents.Node{ID: session.local,
				Attributes: []linkstateevents.TLV{{Type: bgpls.TLVSRCapabilities, Value: wire[4:]}}})
		}
		// Index encoding refers to the advertised local SRGB. P reflects the
		// persistent operator assignment, while V and L are clear for an index.
		value := []byte{0x10, config.Weight, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(value[4:], config.Index)
		snapshot.Links = append(snapshot.Links, linkstateevents.Link{Local: session.local, Remote: session.remote,
			LocalAddresses: []netip.Addr{session.localAddress}, RemoteAddresses: []netip.Addr{peer},
			Attributes: []linkstateevents.TLV{{Type: 1101, Value: value}}})
	}
	_, err := epeSnapshots.Emit(s.bus, &snapshot)
	return err
}

// desiredLocked answers the label each established, configured peer should
// own; a stopped source wants none. Caller holds s.mu.
func (s *Source) desiredLocked() map[netip.Addr]uint32 {
	desired := make(map[netip.Addr]uint32)
	if s.stopped {
		return desired
	}
	for peer, config := range s.config.Peers {
		if _, up := s.sessions[peer]; up {
			desired[peer] = s.config.Base + config.Index
		}
	}
	return desired
}

// ReplayLabels answers a label writer's (mpls-fib, replay-request), tagged
// with the request's token so only the writer that asked consumes it. While
// the source has not yet acquired its label space (for example on a box booted
// with fib-dummy), it runs publishWithApplyLocked with ApplyReplay so the
// acknowledged RemoveLabelSource cleanup runs first against the asking writer
// alone and the wanted adds follow only once cleanup succeeds. Once acquired,
// it re-applies every label desiredLocked wants (accepted and previously
// refused alike), skipping peers awaiting removal and labels still occupied by
// another peer's retained label, records newly installed labels in s.installed
// and re-emits the link-state snapshot if any label became newly installed.
//
// It runs on the requesting writer's goroutine, synchronously, and takes
// s.mu: the writer MUST NOT hold the lock its own (mpls-fib, entry) handler
// takes while it emits the request.
func (s *Source) ReplayLabels(req *replay.Request) {
	if req == nil {
		return
	}
	if !replay.IsReplay(req.ReplayID) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retired {
		return
	}
	if s.stopped {
		return
	}
	replayApply := func(entries []mplsfib.Entry) error {
		return mplsfib.ApplyReplay(s.bus, req.ReplayID, entries)
	}
	if !s.acquired {
		if err := s.publishWithApplyLocked(replayApply); err != nil {
			epeLogger.Warn("BGP EPE: the forwarding writer that asked for the PeerNode SID labels did not complete initial cleanup or installation, so labeled traffic to those peers is not forwarded; run `ze show doctor` to see why",
				"error", err,
				"code", mplsfib.CodeDoctorNoWriter)
		}
		return
	}
	desired := s.desiredLocked()
	occupied := make(map[uint32]netip.Addr, len(s.installed))
	for peer, label := range s.installed {
		occupied[label] = peer
	}
	peers := make([]netip.Addr, 0, len(desired))
	for peer := range desired {
		peers = append(peers, peer)
	}
	slices.SortFunc(peers, netip.Addr.Compare)
	entries := make([]mplsfib.Entry, 0, len(peers))
	for _, peer := range peers {
		label := desired[peer]
		// A peer whose old label's removal failed still owns that label;
		// wait for publishLocked to finish removing it before adding the new one.
		if cur, exists := s.installed[peer]; exists {
			if cur != label {
				continue
			}
		}
		// A label retained by another peer after a failed removal cannot be
		// reassigned until that removal succeeds.
		if owner, taken := occupied[label]; taken {
			if owner != peer {
				continue
			}
		}
		entries = append(entries, mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop,
			InLabel: label, NextHop: peer, Source: epeMPLSSource})
	}
	if len(entries) == 0 {
		return
	}
	if err := replayApply(entries); err != nil {
		// The writer answers one acknowledgement for the batch, so which
		// labels it refused is unknown: each is recorded with the cause.
		for i := range entries {
			s.refusals.Record(&entries[i], err)
		}
		epeLogger.Warn("BGP EPE: the forwarding writer that asked for the PeerNode SID labels did not install them all, so labeled traffic to those peers is not forwarded; run `ze show doctor` to see why",
			"labels", len(entries), "error", err,
			"code", mplsfib.CodeDoctorNoWriter)
		return
	}
	newlyInstalled := false
	for i := range entries {
		peer := entries[i].NextHop
		label := entries[i].InLabel
		cur, ok := s.installed[peer]
		if !ok {
			s.installed[peer] = label
			newlyInstalled = true
		} else if cur != label {
			s.installed[peer] = label
			newlyInstalled = true
		}
		s.refusals.Clear(&entries[i])
	}
	if newlyInstalled {
		if err := s.emitSnapshotLocked(desired); err != nil {
			epeLogger.Warn("BGP EPE: emitting the link-state snapshot after label replay failed",
				"error", err,
				"code", mplsfib.CodeDoctorNoWriter)
		}
	}
}

// emitLabelWith applies one PeerNode SID label change through applyFn, and
// records or clears its refusal. Caller holds s.mu.
func (s *Source) emitLabelWith(applyFn func([]mplsfib.Entry) error, action mplsfib.Action, label uint32, peer netip.Addr) error {
	entries := []mplsfib.Entry{{Action: action, Op: mplsfib.OpPop,
		InLabel: label, NextHop: peer, Source: epeMPLSSource}}
	if err := applyFn(entries); err != nil {
		s.refusals.Record(&entries[0], err)
		return fmt.Errorf("EPE label %d: %w", label, err)
	}
	s.refusals.Clear(&entries[0])
	return nil
}
