// Design: docs/architecture/behavior/peer-lifecycle.md — when a passive peer gets its listener, and what a refused one says
// Overview: reactor.go — startListenerForAddressPort, the one place a BGP listening socket is born
// Related: ../../../core/network/listen_error.go — ListenError, the operator error every listen route reports

package reactor

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/opererr"
)

// listenOwnerUnknown is the owner a refusal names when no peer or group of the
// running configuration claims the endpoint any more: the refusal still names
// the subsystem, never a blank.
const listenOwnerUnknown = "bgp"

// listenRefusal is the operator error for a BGP listener on addr:port the
// kernel refused, cause being what the listener factory returned. It names the
// configured address and port and what configured them (listenOwner), and
// network.ListenError gives the why and the fix: CAP_NET_BIND_SERVICE with its
// remedy for a privileged port, the holder of an address in use, the address
// list for an address no interface holds.
// Caller MUST hold r.mu (read or write).
func (r *Reactor) listenRefusal(addr netip.Addr, port int, cause error) *opererr.Error {
	return network.ListenError(network.ListenSite{
		Network:  "tcp",
		Endpoint: netip.AddrPortFrom(addr, uint16(port)), //nolint:gosec // peerListenPort answers a configured TCP port, which fits a uint16
		Owner:    r.listenOwner(addr, port),
	}, cause)
}

// listenOwner names what in the running configuration asks for a listener on
// addr:port, as the operator wrote it: "bgp peer upstream1", "bgp group
// rs-clients". Several claimants share one listener, so it names the first in
// sort order and counts the rest: a map walk alone would name a different one
// on each refusal. It reads the same claim listenerSpecs and listenPortsAt
// read: a passive peer or dynamic group at that local address and listen port.
// Caller MUST hold r.mu (read or write).
func (r *Reactor) listenOwner(addr netip.Addr, port int) string {
	var owners []string
	for _, peer := range r.peers {
		settings := peer.Settings()
		if r.claimsListener(settings, addr, port) {
			owners = append(owners, peerListenOwner(settings))
		}
	}
	for _, dg := range r.dynamicGroups {
		if r.claimsListener(dg.Settings, addr, port) {
			owners = append(owners, "bgp group "+dg.GroupName)
		}
	}
	if len(owners) == 0 {
		return listenOwnerUnknown
	}
	slices.Sort(owners)
	owners = slices.Compact(owners)
	if len(owners) == 1 {
		return owners[0]
	}
	return owners[0] + " and " + strconv.Itoa(len(owners)-1) + " more"
}

// claimsListener reports whether settings asks for a listener on addr:port.
// Caller MUST hold r.mu (read or write).
func (r *Reactor) claimsListener(settings *PeerSettings, addr netip.Addr, port int) bool {
	if settings == nil {
		return false
	}
	if !settings.LocalAddress.IsValid() {
		return false
	}
	if !settings.Connection.IsPassive() {
		return false
	}
	if r.peerListenPort(settings) != port {
		return false
	}
	return sameTransportAddress(settings.LocalAddress, addr)
}

// peerListenOwner names a peer as the operator configured it: a dynamic peer
// by the group whose template made it, a configured peer by its name, or by its
// address when it has none.
func peerListenOwner(settings *PeerSettings) string {
	if settings.IsDynamic {
		return "bgp group " + settings.GroupName
	}
	if settings.Name != "" {
		return "bgp peer " + settings.Name
	}
	return "bgp peer " + settings.Address.String()
}

// logListenRefusal logs a BGP listener that did not start on a path that goes
// on without it: a dynamic group a reload added, and an address the kernel
// reported added. Neither path refuses (the reload is already applied; the
// address event has no caller to answer), so the log line is where the
// operator meets the refusal, with its why, its fix, and the doctor code the
// listener check declares.
//
// It takes no lock, so a caller may hold r.mu.
func logListenRefusal(err error) {
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		// Every refusal startListenerForAddressPort answers is an operator error
		// but the stop's seal, which its callers handle before this. A plain
		// error here is that contract broken, logged as it is rather than lost.
		reactorLogger().Error("bgp listener did not start", "error", err,
			"code", diagnostic.CodeDoctorBGPListen, "fix", "ze doctor")
		return
	}
	reactorLogger().LogAttrs(context.Background(), slog.LevelError, refusal.Problem(),
		refusal.LogAttrs(slog.String("code", diagnostic.CodeDoctorBGPListen))...)
}
