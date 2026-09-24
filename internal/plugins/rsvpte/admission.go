// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE bandwidth admission control
// Related: wire.go -- FlowSpec carries bandwidth parameters
// Related: fsm.go -- LSP tracks bandwidth reservation
//
// RFC 3209 Section 4.7: Admission control checks available bandwidth
// per interface before accepting a reservation.
//
// SE reservations share the session's maximum on each link. FF reservations
// charge each sender independently. Rates at this boundary are bits per second;
// the signaling engine converts the RFC 2210 byte-rate once on receipt.
package rsvpte

import (
	"errors"
	"net/netip"
	"sync"
)

var errAdmissionDenied = errors.New("rsvp-te: admission denied, insufficient bandwidth")

// sessionID identifies an RSVP SESSION for SE reservation sharing. It excludes
// the SENDER (addr/LSP_ID) so the replacement LSP in a make-before-break shares
// the reservation of the LSP it supersedes.
type sessionID struct {
	endpoint netip.Addr
	tunnelID uint16
	extID    uint32
}

func sessionFromIPv4(s sessionIPv4) sessionID {
	return sessionID{endpoint: s.TunnelEndpoint, tunnelID: s.TunnelID, extID: s.ExtTunnelID}
}

func sessionFromKey(k lspKey) sessionID {
	return sessionID{endpoint: k.TunnelEndpoint, tunnelID: k.TunnelID, extID: k.ExtTunnelID}
}

type reservationCharge struct {
	style     uint32
	bandwidth float64
}

type sessionReservation struct {
	holders map[lspKey]reservationCharge
}

func (sr *sessionReservation) footprint() float64 {
	var shared, fixed float64
	for _, holder := range sr.holders {
		if holder.style == StyleSharedExplicit {
			shared = max(shared, holder.bandwidth)
		} else {
			fixed += holder.bandwidth
		}
	}
	return shared + fixed
}

// interfaceBandwidth tracks bandwidth state for one interface.
//
// Bandwidth is accounted in float64: RSVP FlowSpec carries IEEE 32-bit
// floats on the wire (RFC 2210), but at realistic link rates (1e9-1e12
// bytes/s) the float32 ULP exceeds 1, so accumulating reservations in
// float32 silently rounds small reservations away and admission control
// fails to reject oversubscription. Values are widened from float32 at the
// config/wire boundary.
type interfaceBandwidth struct {
	MaxBandwidth      float64
	MaxReservable     float64
	ReservedBandwidth float64
}

// Available returns the remaining reservable bandwidth.
func (ib *interfaceBandwidth) Available() float64 {
	avail := ib.MaxReservable - ib.ReservedBandwidth
	if avail < 0 {
		return 0
	}
	return avail
}

// admissionController manages per-interface bandwidth accounting.
type admissionController struct {
	mu         sync.Mutex
	interfaces map[string]*interfaceBandwidth
	// sessions holds SE reservation sharing state: interface -> session ->
	// multiset of per-LSP reservations. Only the session's footprint (max) is
	// counted in InterfaceBandwidth.ReservedBandwidth.
	sessions map[string]map[sessionID]*sessionReservation
}

// newAdmissionController creates an admission controller.
func newAdmissionController() *admissionController {
	return &admissionController{
		interfaces: make(map[string]*interfaceBandwidth),
		sessions:   make(map[string]map[sessionID]*sessionReservation),
	}
}

// reserve replaces exactly one sender's charge. A failed increase leaves the
// old charge untouched; callers MUST restore it if native installation fails.
func (ac *admissionController) reserve(iface string, key lspKey, style uint32, bandwidth float64) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib := ac.interfaces[iface]
	if ib == nil {
		return nil
	}
	session := sessionFromKey(key)
	per := ac.sessions[iface]
	if per == nil {
		per = make(map[sessionID]*sessionReservation)
		ac.sessions[iface] = per
	}
	sr := per[session]
	if sr == nil {
		sr = &sessionReservation{holders: make(map[lspKey]reservationCharge)}
		per[session] = sr
	}
	before := sr.footprint()
	old, existed := sr.holders[key]
	sr.holders[key] = reservationCharge{style: style, bandwidth: bandwidth}
	delta := sr.footprint() - before
	if delta > 0 && ib.ReservedBandwidth+delta > ib.MaxReservable {
		if existed {
			sr.holders[key] = old
		} else {
			delete(sr.holders, key)
		}
		if len(sr.holders) == 0 {
			delete(per, session)
		}
		return errAdmissionDenied
	}
	ib.ReservedBandwidth += delta
	return nil
}

// release removes only this sender, never a different equal-rate holder.
func (ac *admissionController) release(iface string, key lspKey) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib := ac.interfaces[iface]
	if ib == nil {
		return
	}
	per := ac.sessions[iface]
	session := sessionFromKey(key)
	sr := per[session]
	if sr == nil {
		return
	}
	before := sr.footprint()
	delete(sr.holders, key)
	ib.ReservedBandwidth -= before - sr.footprint()
	if len(sr.holders) == 0 {
		delete(per, session)
	}
}

// setInterface configures bandwidth limits for an interface. It is also called on
// every config reload, so it must NOT discard the live reserved bandwidth or the
// per-session reservations: for an existing interface it updates only the limits.
// Replacing the struct (zeroing ReservedBandwidth) while LSPs hold reservations
// would make the link admit past MaxReservable until every pre-reload LSP drains.
func (ac *admissionController) setInterface(name string, maxBW, maxReservable float64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ib, ok := ac.interfaces[name]; ok {
		ib.MaxBandwidth = maxBW
		ib.MaxReservable = maxReservable
		return // keep ReservedBandwidth and the live sessions
	}
	ac.interfaces[name] = &interfaceBandwidth{
		MaxBandwidth:  maxBW,
		MaxReservable: maxReservable,
	}
}

// removeInterface drops every trace of an interface the operator took out of the
// config: its bandwidth limits and its per-session reservation records. It returns
// the number of session records dropped so the caller can report what the reload
// discarded; an unknown name drops nothing and returns 0.
//
// It removes both maps together, and that pairing is the invariant. ReleaseSession
// looks the session up under the interface, so an LSP torn down after its interface
// left the config finds no record and releases nothing. Dropping only one of the two
// would leave the other to decrement a limit its reservation was never counted in.
//
// The LSPs that reserved against the interface are NOT torn down. RFC 2205 Section
// 2.4 initiates a teardown "by an application in an end system (sender or receiver),
// or by a router as the result of state timeout or service preemption", and an
// operator withdrawing a link's bandwidth declaration is none of those: the traffic
// still flows, only ze's accounting for it stops. Tearing the LSPs down would make a
// config commit drop live traffic. The cost is that an interface removed and added
// back starts its accounting at zero while pre-removal LSPs still hold their
// bandwidth, so the link under-counts until they drain.
func (ac *admissionController) removeInterface(name string) int {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	dropped := len(ac.sessions[name])
	delete(ac.interfaces, name)
	delete(ac.sessions, name)
	return dropped
}

// Reserve attempts to reserve bandwidth on an interface.
func (ac *admissionController) Reserve(iface string, bandwidth float64) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib, ok := ac.interfaces[iface]
	if !ok {
		return nil
	}
	if ib.ReservedBandwidth+bandwidth > ib.MaxReservable {
		return errAdmissionDenied
	}
	ib.ReservedBandwidth += bandwidth
	return nil
}

// Release returns reserved bandwidth to an interface.
func (ac *admissionController) Release(iface string, bandwidth float64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib, ok := ac.interfaces[iface]
	if !ok {
		return
	}
	ib.ReservedBandwidth -= bandwidth
	if ib.ReservedBandwidth < 0 {
		ib.ReservedBandwidth = 0
	}
}

// GetInterface returns bandwidth state for an interface.
func (ac *admissionController) GetInterface(name string) (interfaceBandwidth, bool) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib, ok := ac.interfaces[name]
	if !ok {
		return interfaceBandwidth{}, false
	}
	return *ib, true
}

// allInterfaces returns bandwidth state for all interfaces.
func (ac *admissionController) allInterfaces() map[string]interfaceBandwidth {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	out := make(map[string]interfaceBandwidth, len(ac.interfaces))
	for k, v := range ac.interfaces {
		out[k] = *v
	}
	return out
}
