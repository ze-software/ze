// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- merge-point reply identity
package rsvpte

import (
	"context"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

type peerDomain struct {
	source string
	domain linkstateevents.Domain
}

type peerIdentity struct {
	sync.RWMutex
	local []InterfaceAddress
	domains map[peerDomain]map[netip.Addr]map[netip.Addr]struct{}
}

// refreshLocalAddresses runs outside LSP locks. Readers use the owned snapshot
// while deciding ERO membership and choosing a distinct local repair sender.
func (e *engine) refreshLocalAddresses() error {
	addresses, err := e.transport.LocalAddresses()
	if err != nil {
		addresses = nil
	}
	e.peers.Lock()
	e.peers.local = addresses
	e.peers.Unlock()
	return err
}

func (e *engine) isLocalAddress(address netip.Addr) bool {
	return e.isLocalPrefix(netip.PrefixFrom(address, address.BitLen()))
}

func (e *engine) isLocalPrefix(prefix netip.Prefix) bool {
	e.peers.RLock()
	defer e.peers.RUnlock()
	for _, address := range e.peers.local {
		if prefix.Contains(address.Address) {
			return true
		}
	}
	return false
}

// samePeer accepts only a configured address or another address attributed to
// that node by a live native IGP database. Reachable prefixes are deliberately
// excluded: advertising a host route does not make its address the router's.
func (e *engine) samePeer(expected, received netip.Addr) bool {
	if !received.Is4() || received.IsUnspecified() || received.IsMulticast() || received == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return false
	}
	if expected == received {
		return true
	}
	e.peers.RLock()
	defer e.peers.RUnlock()
	for _, domain := range e.peers.domains {
		if _, ok := domain[expected][received]; ok {
			return true
		}
	}
	return false
}

// peerInPrefix tests abstract-node membership using native node addresses,
// not a reachable prefix advertised by the peer.
func (e *engine) peerInPrefix(peer netip.Addr, prefix netip.Prefix) bool {
	if prefix.Contains(peer) {
		return true
	}
	e.peers.RLock()
	defer e.peers.RUnlock()
	for _, domain := range e.peers.domains {
		for address := range domain[peer] {
			if prefix.Contains(address) {
				return true
			}
		}
	}
	return false
}

func (e *engine) updatePeerIdentity(source string, snapshot *linkstateevents.Snapshot) {
	if snapshot == nil {
		return
	}
	switch snapshot.Domain.Protocol {
	case linkstateevents.OSPFv2, linkstateevents.OSPFv3, linkstateevents.ISISLevel1, linkstateevents.ISISLevel2:
	default:
		return
	}
	owners := make(map[netip.Addr]string)
	visitPeerAddresses(snapshot, func(node []byte, address netip.Addr) {
		if len(node) == 0 || !address.Is4() || address.IsUnspecified() || address.IsMulticast() {
			return
		}
		identity := string(node)
		if previous, exists := owners[address]; exists && previous != identity {
			owners[address] = "" // ambiguous ownership is not an alias association
		} else if !exists {
			owners[address] = identity
		}
	})
	byNode := make(map[string]map[netip.Addr]struct{})
	for address, node := range owners {
		if node == "" {
			continue
		}
		addresses := byNode[node]
		if addresses == nil {
			addresses = make(map[netip.Addr]struct{})
			byNode[node] = addresses
		}
		addresses[address] = struct{}{}
	}
	aliases := make(map[netip.Addr]map[netip.Addr]struct{}, len(owners))
	for address, node := range owners {
		if node != "" {
			aliases[address] = byNode[node]
		}
	}
	e.peers.Lock()
	if e.peers.domains == nil {
		e.peers.domains = make(map[peerDomain]map[netip.Addr]map[netip.Addr]struct{})
	}
	key := peerDomain{source: source, domain: snapshot.Domain}
	if len(aliases) == 0 {
		delete(e.peers.domains, key)
	} else {
		e.peers.domains[key] = aliases
	}
	e.peers.Unlock()
}

// visitPeerAddresses reads only node identities and actual interface addresses,
// not TE constraints or paths. The snapshot's storage is borrowed for this call.
func visitPeerAddresses(snapshot *linkstateevents.Snapshot, visit func([]byte, netip.Addr)) {
	for _, node := range snapshot.Nodes {
		if len(node.ID.RouterID) == 4 {
			visit(node.ID.RouterID, netip.AddrFrom4([4]byte(node.ID.RouterID)))
		}
		for _, attr := range node.Attributes {
			// RFC 9552: IPv4 Router-ID attribute (also used by IS-IS sources).
			if attr.Type == 1028 && len(attr.Value) == 4 {
				visit(node.ID.RouterID, netip.AddrFrom4([4]byte(attr.Value)))
			}
		}
	}
	for _, link := range snapshot.Links {
		for _, address := range link.LocalAddresses {
			visit(link.Local.RouterID, address)
		}
		for _, address := range link.RemoteAddresses {
			visit(link.Remote.RouterID, address)
		}
	}
}

func (e *engine) startPeerIdentity(ctx context.Context) {
	bus := getEventBus()
	if bus == nil {
		return
	}
	var unsubscribe []func()
	for _, source := range linkstateevents.Sources() {
		unsubscribe = append(unsubscribe, bus.Subscribe(source, linkstateevents.EventType, func(payload any) {
			if snapshot, ok := payload.(*linkstateevents.Snapshot); ok {
				e.updatePeerIdentity(source, snapshot)
			}
		}))
	}
	go func() {
		<-ctx.Done()
		for _, stop := range unsubscribe {
			stop()
		}
	}()
	e.refreshPeerIdentity()
}

func (e *engine) refreshPeerIdentity() {
	e.peers.Lock()
	clear(e.peers.domains)
	e.peers.Unlock()
	if bus := getEventBus(); bus != nil {
		if _, err := linkstateevents.Request.Emit(bus); err != nil {
			e.log.Warn("rsvp-te: peer identity snapshot request failed", "error", err)
		}
	}
}
