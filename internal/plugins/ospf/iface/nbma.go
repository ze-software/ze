// Design: docs/architecture/ospf/ospf-5-interface-ism.md -- NBMA + point-to-multipoint Hello send.
// RFC: rfc/short/rfc2328.md (sec 9.4 step 6, sec 9.5, sec 10.1 Attempt), rfc/short/rfc5340.md (sec 2.9 unicast)
//
// The shared ISM core (iface.go) sends Hellos to a multicast group. NBMA has no
// all-routers multicast, so it unicasts a Hello to each statically configured
// neighbor (RFC 2328 App C.6): at HelloInterval to neighbors it has heard from, and
// at the slower PollInterval to silent (Attempt) neighbors. The non-broadcast
// point-to-multipoint variant unicasts to the same configured list every tick with no
// poll gating. The logic is address-family-neutral; only the unicast destination form
// (IPv4 address vs IPv6 link-local) branches on IsV6.
package iface

import (
	"net/netip"
	"time"
)

// afLabel is the address-family metric label value.
func afLabel(isV6 bool) string {
	if isV6 {
		return "ipv6"
	}
	return "ipv4"
}

// nonBroadcastLocked reports whether Hellos are unicast to a configured neighbor list
// rather than sent to the all-routers multicast group: NBMA always, and the
// point-to-multipoint variant that carries an explicit neighbor list (RFC 2328 sec 9.5).
func (i *Interface) nonBroadcastLocked() bool {
	switch i.cfg.NetworkType {
	case NetworkNBMA:
		return true
	case NetworkPointToMultipoint:
		return len(i.cfg.NBMANeighbors) > 0
	default:
		return false
	}
}

// nbmaDestLocked returns the unicast Hello destination for a configured neighbor and
// whether it is known. IPv4 uses the configured address; IPv6 (RFC 5340 sec 2.9) uses
// the configured link-local, else the link-local learned from the neighbor's Hello.
func (i *Interface) nbmaDestLocked(n NBMANeighbor) (netip.Addr, bool) {
	if i.cfg.IsV6 {
		if n.LinkLocal.IsValid() {
			return n.LinkLocal, true
		}
		if hn, ok := i.neighbors[n.RouterID]; ok && hn.Address.IsValid() {
			return hn.Address, true
		}
		return netip.Addr{}, false
	}
	if n.Address.IsValid() {
		return n.Address, true
	}
	return netip.Addr{}, false
}

// nbmaHeardLocked reports whether a Hello has been received from a configured neighbor:
// keyed by Router ID for IPv6, by reachable address for IPv4.
func (i *Interface) nbmaHeardLocked(n NBMANeighbor) bool {
	if i.cfg.IsV6 {
		_, ok := i.neighbors[n.RouterID]
		return ok
	}
	for _, hn := range i.neighbors {
		if hn.Address == n.Address {
			return true
		}
	}
	return false
}

// helloTargetsLocked returns the unicast Hello destinations for this send tick. A
// point-to-multipoint non-broadcast interface sends to every configured neighbor. An
// NBMA interface sends to every heard eligible neighbor at HelloInterval and polls every
// silent (Attempt, RFC 2328 sec 10.1) eligible neighbor whose PollInterval has elapsed.
// Per RFC 2328 sec 9.5.1, an ineligible (Priority 0) neighbor is included only while this
// router is itself DR or BDR; otherwise it is skipped here and reached only by the
// one-shot Start Hello. It records poll times, counts polls, and refreshes the
// configured-neighbor gauge.
func (i *Interface) helloTargetsLocked(now time.Time) []netip.Addr {
	isNBMA := i.cfg.NetworkType == NetworkNBMA
	poll := time.Duration(i.cfg.PollInterval) * time.Second
	af := afLabel(i.cfg.IsV6)
	out := make([]netip.Addr, 0, len(i.cfg.NBMANeighbors))
	for _, n := range i.cfg.NBMANeighbors {
		dst, ok := i.nbmaDestLocked(n)
		if !ok {
			continue
		}
		// RFC 2328 sec 9.5.1: an ineligible (Priority 0) NBMA neighbor receives periodic
		// and poll Hellos only while this router is itself DR or BDR. Otherwise it gets
		// only the one-shot Start Hello from startHelloTargetsLocked (sec 9.4 step 6). PtMP
		// has no DR/BDR election, so this eligibility gating is NBMA-only.
		if isNBMA && n.Priority == 0 && i.state != StateDR && i.state != StateBackup {
			continue
		}
		if !isNBMA || i.nbmaHeardLocked(n) {
			// PtMP: every neighbor every tick. NBMA heard: HelloInterval rate.
			out = append(out, dst)
			continue
		}
		// NBMA silent (Attempt): poll at the slower PollInterval.
		last, seen := i.nbmaLastPoll[dst]
		if !seen || poll <= 0 || now.Sub(last) >= poll {
			i.nbmaLastPoll[dst] = now
			i.metrics.NBMAPolls.With(i.cfg.Name, af).Inc()
			out = append(out, dst)
		}
	}
	if isNBMA {
		i.updateNBMANeighborGaugeLocked(af)
	}
	return out
}

// startHelloTargetsLocked returns the priority-0 (ineligible) configured-neighbor
// destinations that must receive an immediate Start Hello because this NBMA router just
// became DR or BDR (RFC 2328 sec 9.4 step 6). It returns nil unless the election changed
// this router's role and it is now the DR or BDR.
func (i *Interface) startHelloTargetsLocked(elected bool) []netip.Addr {
	if !elected || i.cfg.NetworkType != NetworkNBMA {
		return nil
	}
	if i.state != StateDR && i.state != StateBackup {
		return nil
	}
	var out []netip.Addr
	for _, n := range i.cfg.NBMANeighbors {
		if n.Priority != 0 {
			continue
		}
		if dst, ok := i.nbmaDestLocked(n); ok {
			out = append(out, dst)
		}
	}
	return out
}

// updateNBMANeighborGaugeLocked publishes the configured-neighbor count split by poll
// state (heard vs attempt) for this NBMA interface.
func (i *Interface) updateNBMANeighborGaugeLocked(af string) {
	heard, attempt := 0, 0
	for _, n := range i.cfg.NBMANeighbors {
		if i.nbmaHeardLocked(n) {
			heard++
		} else {
			attempt++
		}
	}
	i.metrics.NBMANeighbors.With(i.cfg.Name, af, "heard").Set(float64(heard))
	i.metrics.NBMANeighbors.With(i.cfg.Name, af, "attempt").Set(float64(attempt))
}

// nbmaNeighborSnapshotsLocked renders the configured NBMA neighbors for `show ospf
// interface`, marking each heard or attempt (RFC 2328 sec 10.1).
func (i *Interface) nbmaNeighborSnapshotsLocked() []NBMANeighborSnapshot {
	if len(i.cfg.NBMANeighbors) == 0 {
		return nil
	}
	out := make([]NBMANeighborSnapshot, 0, len(i.cfg.NBMANeighbors))
	for _, n := range i.cfg.NBMANeighbors {
		id := ""
		switch {
		case i.cfg.IsV6:
			id = n.RouterID.String()
		case n.Address.IsValid():
			id = n.Address.String()
		}
		state := "attempt"
		if i.nbmaHeardLocked(n) {
			state = "heard"
		}
		out = append(out, NBMANeighborSnapshot{Neighbor: id, Priority: n.Priority, State: state})
	}
	return out
}

// setPTMPHostRouteLocked records the point-to-multipoint host-route metric: a PtMP
// interface contributes exactly one host route (its own reachable address) while up.
func (i *Interface) setPTMPHostRouteLocked(active bool) {
	v := 0.0
	if active {
		v = 1
	}
	i.metrics.PTMPHostRoutes.With(i.cfg.Name, afLabel(i.cfg.IsV6)).Set(v)
}

// sendUnicast sends payload to each destination, returning the first send error without
// aborting the fan-out (RFC 2328 sec 13.3: a failed send to one neighbor must not stop
// delivery to the others).
func sendUnicast(sender Sender, name string, dsts []netip.Addr, payload []byte) error {
	var firstErr error
	for _, dst := range dsts {
		if err := sender.SendPacket(name, dst, payload); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
