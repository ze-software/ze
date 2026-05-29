// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE bandwidth admission control
// Related: wire.go -- FlowSpec carries bandwidth parameters
// Related: fsm.go -- LSP tracks bandwidth reservation
//
// RFC 3209 Section 4.7: Admission control checks available bandwidth
// per interface before accepting a reservation.
//
// RFC 3209 Section 6.1 (SHARED EXPLICIT): during make-before-break the ingress
// signals a replacement LSP for the same SESSION (new LSP_ID, SE style) that
// must SHARE bandwidth with the LSP it replaces on common links, so admission
// does not double-count the reservation and reject the reroute. ReserveSession/
// ReleaseSession implement this: per interface, per SESSION we keep the multiset
// of the LSPs' reservations and account only the session's MAXIMUM (its link
// footprint) against the interface, not the sum. Two LSPs of one session at the
// same rate therefore consume that rate once, not twice.
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

func sessionFromIPv4(s SessionIPv4) sessionID {
	return sessionID{endpoint: s.TunnelEndpoint, tunnelID: s.TunnelID, extID: s.ExtTunnelID}
}

func sessionFromKey(k LSPKey) sessionID {
	return sessionID{endpoint: k.TunnelEndpoint, tunnelID: k.TunnelID, extID: k.ExtTunnelID}
}

// sessionReservation is the multiset of per-LSP reservations sharing a link for
// one SESSION. The session's interface footprint is the largest holder (SE).
type sessionReservation struct {
	holders []float64
}

func (sr *sessionReservation) footprint() float64 {
	maxBW := 0.0
	for _, h := range sr.holders {
		if h > maxBW {
			maxBW = h
		}
	}
	return maxBW
}

func (sr *sessionReservation) add(bw float64) {
	sr.holders = append(sr.holders, bw)
}

// remove drops one holder equal to bw (the LSP being torn down). If no exact
// match exists it removes the largest holder, keeping the footprint monotonic
// and never under-counting.
func (sr *sessionReservation) remove(bw float64) {
	for i, h := range sr.holders {
		if h == bw {
			sr.holders = append(sr.holders[:i], sr.holders[i+1:]...)
			return
		}
	}
	if len(sr.holders) == 0 {
		return
	}
	maxIdx := 0
	for i, h := range sr.holders {
		if h > sr.holders[maxIdx] {
			maxIdx = i
		}
	}
	sr.holders = append(sr.holders[:maxIdx], sr.holders[maxIdx+1:]...)
}

// InterfaceBandwidth tracks bandwidth state for one interface.
//
// Bandwidth is accounted in float64: RSVP FlowSpec carries IEEE 32-bit
// floats on the wire (RFC 2210), but at realistic link rates (1e9-1e12
// bytes/s) the float32 ULP exceeds 1, so accumulating reservations in
// float32 silently rounds small reservations away and admission control
// fails to reject oversubscription. Values are widened from float32 at the
// config/wire boundary.
type InterfaceBandwidth struct {
	MaxBandwidth      float64
	MaxReservable     float64
	ReservedBandwidth float64
}

// Available returns the remaining reservable bandwidth.
func (ib *InterfaceBandwidth) Available() float64 {
	avail := ib.MaxReservable - ib.ReservedBandwidth
	if avail < 0 {
		return 0
	}
	return avail
}

// AdmissionController manages per-interface bandwidth accounting.
type AdmissionController struct {
	mu         sync.Mutex
	interfaces map[string]*InterfaceBandwidth
	// sessions holds SE reservation sharing state: interface -> session ->
	// multiset of per-LSP reservations. Only the session's footprint (max) is
	// counted in InterfaceBandwidth.ReservedBandwidth.
	sessions map[string]map[sessionID]*sessionReservation
}

// NewAdmissionController creates an admission controller.
func NewAdmissionController() *AdmissionController {
	return &AdmissionController{
		interfaces: make(map[string]*InterfaceBandwidth),
		sessions:   make(map[string]map[sessionID]*sessionReservation),
	}
}

// ReserveSession reserves bandwidth for one LSP of a SESSION on an interface
// using SHARED EXPLICIT semantics: the interface is charged only the increase
// in the session's footprint (max over its LSPs), so a make-before-break
// replacement at the same rate adds nothing. Returns errAdmissionDenied when the
// incremental demand would exceed the reservable limit.
func (ac *AdmissionController) ReserveSession(iface string, sess sessionID, bandwidth float64) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib, ok := ac.interfaces[iface]
	if !ok {
		return nil
	}
	per := ac.sessions[iface]
	if per == nil {
		per = make(map[sessionID]*sessionReservation)
		ac.sessions[iface] = per
	}
	sr := per[sess]
	if sr == nil {
		sr = &sessionReservation{}
		per[sess] = sr
	}
	old := sr.footprint()
	newFootprint := old
	if bandwidth > newFootprint {
		newFootprint = bandwidth
	}
	delta := newFootprint - old
	if ib.ReservedBandwidth+delta > ib.MaxReservable {
		if len(sr.holders) == 0 {
			delete(per, sess)
		}
		return errAdmissionDenied
	}
	sr.add(bandwidth)
	ib.ReservedBandwidth += delta
	return nil
}

// ReleaseSession releases one LSP's reservation for a SESSION, returning to the
// interface only the reduction in the session's footprint (SE). The session's
// shared reservation persists until its last LSP is released.
func (ac *AdmissionController) ReleaseSession(iface string, sess sessionID, bandwidth float64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib, ok := ac.interfaces[iface]
	if !ok {
		return
	}
	per := ac.sessions[iface]
	if per == nil {
		return
	}
	sr := per[sess]
	if sr == nil {
		return
	}
	old := sr.footprint()
	sr.remove(bandwidth)
	newFootprint := sr.footprint()
	ib.ReservedBandwidth -= old - newFootprint
	if ib.ReservedBandwidth < 0 {
		ib.ReservedBandwidth = 0
	}
	if len(sr.holders) == 0 {
		delete(per, sess)
	}
}

// SetInterface configures bandwidth limits for an interface.
func (ac *AdmissionController) SetInterface(name string, maxBW, maxReservable float64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.interfaces[name] = &InterfaceBandwidth{
		MaxBandwidth:  maxBW,
		MaxReservable: maxReservable,
	}
}

// Reserve attempts to reserve bandwidth on an interface.
func (ac *AdmissionController) Reserve(iface string, bandwidth float64) error {
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
func (ac *AdmissionController) Release(iface string, bandwidth float64) {
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
func (ac *AdmissionController) GetInterface(name string) (InterfaceBandwidth, bool) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ib, ok := ac.interfaces[name]
	if !ok {
		return InterfaceBandwidth{}, false
	}
	return *ib, true
}

// AllInterfaces returns bandwidth state for all interfaces.
func (ac *AdmissionController) AllInterfaces() map[string]InterfaceBandwidth {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	out := make(map[string]InterfaceBandwidth, len(ac.interfaces))
	for k, v := range ac.interfaces {
		out[k] = *v
	}
	return out
}
