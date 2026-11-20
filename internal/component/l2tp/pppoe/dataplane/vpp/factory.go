// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP session creation before PADS
// Related: prepare.go -- leased access snapshot; session.go -- lifetime owner
// API: https://github.com/FDio/vpp, src/plugins/pppoe/pppoe.api with the retained
// VyOS patches; GoVPP v0.13.0 bindings generated from the pinned patched source.
package vpp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// Name is the exact interface/session backend name selected by registration.
const Name = "vpp"

type factoryOps struct {
	access   func(context.Context, dataplane.Request) (preparedSessionAccess, error)
	socket   func(context.Context, dataplane.Request) (*controlSocket, error)
	counters func(*binaryAPI) counterReader
}

func openWithOps(ctx context.Context, request dataplane.Request, ops factoryOps) (opened dataplane.OpenedSession, err error) {
	if ctx == nil {
		return nil, errors.New("pppoe-vpp: session requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	access, err := ops.access(ctx, request)
	if err != nil {
		return nil, err
	}
	defer access.release()
	reservation, err := reserveMAC(request.PeerMAC, access.api)
	if err != nil {
		return nil, err
	}
	var socket *controlSocket
	var owner *session
	var receipt creation
	var table uint32
	transferred := false
	defer func() {
		failure := recover()
		if transferred {
			if failure != nil {
				panic(failure)
			}
			return
		}
		var cleanup error
		if owner != nil {
			cleanup = owner.Close()
		} else if socket != nil {
			cleanup = socket.Close()
		}
		if receipt.confirmed && owner == nil {
			// A malformed confirmed index cannot safely address its gates.
			// Keep the exact SID/MAC receipt instead of recycling an unproven
			// forwarding owner or targeting local0/the invalid index.
			cleanup = errors.Join(cleanup, errors.New("pppoe-vpp: confirmed session has no valid index for quarantine"))
		}
		if receipt.uncertain {
			reservation.retain(pendingMutation{api: access.api, request: request, table: table, access: access.dpIndex, local: access.localMAC, failure: errors.Join(err, cleanup)})
		}
		if receipt.confirmed {
			if cleanup != nil {
				reservation.retain(pendingMutation{api: access.api, request: request, table: table, access: access.dpIndex, local: access.localMAC, session: owner, failure: errors.Join(err, cleanup)})
			}
		}
		reservation.release()
		if failure != nil {
			if cleanup == nil {
				panic(failure)
			}
			cause, ok := failure.(error)
			if !ok {
				cause = fmt.Errorf("pppoe-vpp: construction panicked: %v", failure)
			}
			panic(errors.Join(cause, cleanup))
		}
		err = errors.Join(err, cleanup)
	}()
	tables, err := access.api.tables(ctx, access.dpIndex)
	if err != nil {
		return nil, err
	}
	table = tables[0]
	source, err := access.api.routeSource(ctx)
	if err != nil {
		return nil, err
	}
	socket, err = ops.socket(ctx, request)
	if err != nil {
		return nil, err
	}
	if socket == nil {
		return nil, errors.New("pppoe-vpp: control socket factory returned no owner")
	}
	// RFC 2516 Section 5.4: "When the Access Concentrator receives a PADR packet, it prepares to begin a PPP session."
	receipt, err = access.api.create(ctx, request.SessionID, request.PeerMAC, table)
	if receipt.confirmed && receipt.index != 0 && receipt.index != noInterface {
		owner = &session{identity: ppp.SessionIdentity{NativeIndex: receipt.index}, api: access.api, socket: socket, tables: tables, source: source, sid: request.SessionID, mac: request.PeerMAC, routes: make(map[sessionRoute]struct{}), addresses: make(map[netip.Prefix]sessionRoute), release: reservation.release}
		owner.retain = func(failure error) {
			reservation.retain(pendingMutation{api: access.api, request: request, table: table, access: access.dpIndex, local: access.localMAC, session: owner, failure: failure})
		}
	}
	if err != nil {
		return nil, err
	}
	// Both receive arcs of each family must be closed before PADS, including
	// a recycled native interface. Rollback uses the same quarantine fence.
	for family := range 2 {
		if err = access.api.setIP(ctx, receipt.index, family == 1, false); err != nil {
			return nil, err
		}
	}
	if err = access.api.verifySession(ctx, receipt.index, access.dpIndex, table, request.SessionID, request.PeerMAC, access.localMAC); err != nil {
		return nil, err
	}
	name, err := access.api.interfaceName(ctx, receipt.index)
	if err != nil {
		return nil, err
	}
	if err = access.api.setAdmin(ctx, receipt.index, false); err != nil {
		return nil, err
	}
	for family, selected := range tables {
		if err = access.api.setTable(ctx, receipt.index, selected, family == 1); err != nil {
			return nil, err
		}
	}
	mtu := min(ppp.MaxFrameLen, access.mtu-8)
	if mtu < ppp.MinMRU {
		return nil, errors.New("pppoe-vpp: access MTU cannot carry the minimum PPP payload")
	}
	if err = access.api.setMTU(ctx, receipt.index, mtu); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	owner.identity.InterfaceName = name
	owner.readCounters = ops.counters(access.api)
	// VPP has confirmed the created index above, so every dump from now on
	// carries its row; an earlier one cannot answer for this session.
	owner.created = time.Now()
	// NewOpenedSession consumes both roles even if validation or an accessor
	// panics. The allocation rollback above relinquishes authority before it.
	transferred = true
	return dataplane.NewOpenedSession(owner, owner, owner)
}

func validateRequest(request dataplane.Request) error {
	if request.AccessName == "" {
		return errors.New("pppoe-vpp: session has no logical access name")
	}
	if request.Device == "" {
		return errors.New("pppoe-vpp: session has no data-plane device")
	}
	if strings.ContainsRune(request.Device, 0) {
		return errors.New("pppoe-vpp: data-plane device contains NUL")
	}
	if request.ControlDevice == "" {
		return errors.New("pppoe-vpp: session has no Linux control device")
	}
	if len(request.ControlDevice) > 15 {
		return errors.New("pppoe-vpp: control device exceeds IFNAMSIZ")
	}
	if strings.ContainsRune(request.ControlDevice, 0) {
		return errors.New("pppoe-vpp: control device contains NUL")
	}
	if strings.ContainsAny(request.Namespace, "/\\\x00") {
		return errors.New("pppoe-vpp: namespace contains a path separator or NUL")
	}
	if request.SessionID == 0 {
		return errors.New("pppoe-vpp: zero discovery session ID")
	}
	if request.SessionID == ^uint16(0) {
		return errors.New("pppoe-vpp: reserved session ID")
	}
	if request.PeerMAC == [6]byte{} {
		return errors.New("pppoe-vpp: peer MAC is absent")
	}
	if request.PeerMAC[0]&1 != 0 {
		return errors.New("pppoe-vpp: peer MAC is not unicast")
	}
	return nil
}

// Upstream link_table is keyed by MAC alone, so even distinct access devices
// cannot admit one client MAC concurrently. This registry owns only reservations
// made by this provider; it never treats a foreign VPP session as its own.
var macReservations = struct {
	sync.Mutex
	held map[[6]byte]*macReservation
}{held: make(map[[6]byte]*macReservation)}

// pendingMutation retains the exact failed owner or ambiguous create receipt.
// Reconciliation must inspect it before releasing the MAC: another admission
// cannot safely reuse a key whose forwarding mutation remains unresolved.
type pendingMutation struct {
	api     *binaryAPI
	request dataplane.Request
	table   uint32
	access  uint32
	local   [6]byte
	session *session
	failure error
}
type macReservation struct {
	mac     [6]byte
	api     *binaryAPI
	pending *pendingMutation
}

func reserveMAC(mac [6]byte, client *binaryAPI) (*macReservation, error) {
	macReservations.Lock()
	defer macReservations.Unlock()
	if client == nil {
		return nil, errors.New("pppoe-vpp: session reservation has no API owner")
	}
	if client.processExited() {
		return nil, errors.New("pppoe-vpp: session reservation belongs to a retired VPP process")
	}
	if _, exists := macReservations.held[mac]; exists {
		return nil, errors.New("pppoe-vpp: client MAC already belongs to a live or constructing session")
	}
	const sessionMax = 65534
	if len(macReservations.held) >= sessionMax {
		return nil, errors.New("pppoe-vpp: session reservation limit reached")
	}
	reservation := &macReservation{mac: mac, api: client}
	macReservations.held[mac] = reservation
	return reservation, nil
}

func (r *macReservation) release() {
	macReservations.Lock()
	defer macReservations.Unlock()
	if r.pending != nil {
		return
	}
	if macReservations.held[r.mac] == r {
		delete(macReservations.held, r.mac)
	}
}
func (r *macReservation) retain(failure pendingMutation) {
	macReservations.Lock()
	defer macReservations.Unlock()
	if failure.api.processExited() {
		r.pending = nil
		clearSessionReconciliation(&failure)
		return
	}
	r.pending = &failure
	reportSessionReconciliation(&failure, failure.failure)
}

// captureCounters binds the session to the API owner and generation it was
// created under; the stats owner refuses any other generation's snapshot.
func captureCounters(client *binaryAPI) counterReader {
	return func(index uint32, notBefore time.Time) (api.InterfaceCounters, error) {
		return vppcomp.InterfaceCountersForGeneration(client.connector, client.generation, index, notBefore)
	}
}
