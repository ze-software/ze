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
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
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

// publishLocked withdraws obsolete label assignments before installing their
// replacements. The snapshot describes only labels acknowledged by the native
// FIB owner, never hypothetical SIDs attached to ordinary BGP sessions.
func (s *Source) publishLocked() error {
	if s.retired {
		// A successful bye may race the next producer's startup. The deferred
		// stop must not reset labels that the new instance has since acquired.
		return nil
	}
	desired := make(map[netip.Addr]uint32)
	var failures []error
	if !s.stopped {
		for peer, config := range s.config.Peers {
			if _, up := s.sessions[peer]; up {
				desired[peer] = s.config.Base + config.Index
			}
		}
	}
	// The native FIB owner outlives this source object. Reclaim its retained
	// labels before any fresh assignment, including after an error exit.
	if !s.acquired {
		if err := mplsfib.RemoveLabelSource(s.bus, epeMPLSSource); err != nil {
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
		if err := s.emitLabel(mplsfib.ActionRemove, label, peer); err != nil {
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
	for peer, label := range desired {
		// A failed removal still owns the old label. Do not lose that cleanup
		// obligation by overwriting it with a successful replacement.
		if _, installed := s.installed[peer]; installed {
			continue
		}
		if owner, retained := occupied[label]; retained {
			failures = append(failures, fmt.Errorf("EPE label %d still belongs to peer %s", label, owner))
			continue
		}
		if err := s.emitLabel(mplsfib.ActionAdd, label, peer); err != nil {
			failures = append(failures, err)
			continue
		}
		s.installed[peer] = label
		occupied[label] = peer
	}
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
			capabilities := ls.LsSRCapabilities{Ranges: []ls.LsSrLabelRange{{Range: s.config.Size, FirstSID: s.config.Base, SIDLen: 3}}}
			wire := make([]byte, capabilities.Len())
			capabilities.WriteTo(wire, 0)
			snapshot.Nodes = append(snapshot.Nodes, linkstateevents.Node{ID: session.local,
				Attributes: []linkstateevents.TLV{{Type: ls.TLVSRCapabilities, Value: wire[4:]}}})
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
	if err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func (s *Source) emitLabel(action mplsfib.Action, label uint32, peer netip.Addr) error {
	entries := []mplsfib.Entry{{Action: action, Op: mplsfib.OpPop,
		InLabel: label, NextHop: peer, Source: epeMPLSSource}}
	if err := mplsfib.Apply(s.bus, entries); err != nil {
		return fmt.Errorf("EPE label %d: %w", label, err)
	}
	return nil
}
