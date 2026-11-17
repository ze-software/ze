// Design: docs/architecture/config/syntax.md -- scoped TCP endpoint identity
// Related: reactor_peers.go -- configured keys retain their original zones
// Related: reactor_connection.go -- socket ownership compares physical zones

package reactor

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// transportZoneIndexMax is net's decimal-zone fallback bound (net/parse.go).
const transportZoneIndexMax = 0xFFFFFF

// transportAddressMatch keeps an unreadable interface table distinct from a
// proven different endpoint. Zero is unspecified and never grants a match.
type transportAddressMatch uint8

const (
	transportAddressUnspecified transportAddressMatch = iota
	transportAddressUnknown
	transportAddressDifferent
	transportAddressSame
)

// sameTransportAddress compares scoped addresses at runtime socket boundaries.
// Configuration construction MUST retain distinct zone spellings until the
// binding boundary. Different spellings alias only when they resolve to the
// same nonzero interface; an unreadable table denies the
// alias. It never changes either address or a configured PeerKey.
func sameTransportAddress(a, b netip.Addr) bool {
	return sameTransportAddressUsing(a, b, net.Interfaces) == transportAddressSame
}

// sameTransportAddress gives ownership decisions a reactor-local interface
// source. The production reader and comparison algorithm remain shared with
// the other socket boundaries above. An unreadable table returns Unknown so
// a caller cannot infer exclusive ownership from a failed alias comparison.
func (r *Reactor) sameTransportAddress(a, b netip.Addr) transportAddressMatch {
	return sameTransportAddressUsing(a, b, r.readInterfaces)
}

func sameTransportAddressUsing(a, b netip.Addr, read func() ([]net.Interface, error)) transportAddressMatch {
	// TCPAddr can present IPv4 in sixteen bytes. Canonicalize comparison
	// values without changing declarations or IPv6 interface scopes.
	a = a.Unmap()
	b = b.Unmap()
	if a == b {
		return transportAddressSame
	}
	if a.WithZone("") != b.WithZone("") {
		return transportAddressDifferent
	}
	if a.Zone() == "" {
		return transportAddressDifferent
	}
	if b.Zone() == "" {
		return transportAddressDifferent
	}
	interfaces, err := readTransportInterfacesFrom(read)
	if err != nil {
		return transportAddressUnknown
	}
	if sameTransportAddressOn(a, b, interfaces) {
		return transportAddressSame
	}
	return transportAddressDifferent
}

// readTransportInterfaces publishes the same bounded diagnostic for socket
// matching and for the physical-identity validation before sockets start.
func readTransportInterfaces() ([]net.Interface, error) {
	return readTransportInterfacesFrom(net.Interfaces)
}

func readTransportInterfacesFrom(read func() ([]net.Interface, error)) ([]net.Interface, error) {
	if read == nil {
		read = net.Interfaces
	}
	interfaces, err := read()
	if err != nil {
		failure := transportScopeDiagnostic(err)
		reactorLogger().Warn(failure.Message, "code", codeTransportScopeUnknown,
			"error", err, "fix", failure.Fix)
		report.RaiseWarningFix(reportSourceBGP, failure.Code, transportScopeSubject,
			failure.Message, failure.Fix)
		return nil, err
	}
	report.ClearWarning(reportSourceBGP, codeTransportScopeUnknown, transportScopeSubject)
	return interfaces, nil
}

// validatePeerTransportsLocked checks physical identity against the current
// kernel interface table at the binding boundary. Startup ordering covers a
// selected provider's handshake, not deferred VPP settlement.
// The caller MUST hold r.mu through this check and the listener pass that
// publishes listenersStarted; later AddPeer calls validate their candidate
// before publication. A nil candidate checks the startup set.
// One immutable listing and one key map keep the pass linear in peer count.
// Settings and their configured PeerKeys are never rewritten.
func (r *Reactor) validatePeerTransportsLocked(candidate *PeerSettings) error {
	settings := make([]*PeerSettings, 0, len(r.peers)+1)
	for _, peer := range r.peers {
		settings = append(settings, peer.Settings())
	}
	if candidate != nil {
		settings = append(settings, candidate)
	}
	scoped := false
	for _, peer := range settings {
		if peer.Address.Zone() != "" {
			scoped = true
		}
		if peer.LocalAddress.Zone() != "" {
			scoped = true
		}
	}
	if !scoped {
		// Exact unscoped duplicate/self identities were checked at AddPeer.
		return nil
	}
	interfaces, err := readTransportInterfaces()
	if err != nil {
		return opererr.Wrap(err, "BGP TCP interface scopes cannot be checked",
			"Peers cannot start until their physical interface identities can be distinguished.", transportScopeFix)
	}
	seen := make(map[netip.AddrPort]*PeerSettings, len(settings))
	for _, peer := range settings {
		remote := transportAddressKeyOn(peer.Address, interfaces)
		local := transportAddressKeyOn(peer.LocalAddress, interfaces)
		if remote == local {
			if !remote.IsLoopback() {
				return opererr.New(fmt.Sprintf("peer %s: address cannot equal local-address", peer.Address),
					"The local and remote TCP addresses select the same physical interface and address.", transportScopeFix)
			}
		}
		key := netip.AddrPortFrom(remote, peer.PeerKey().Port())
		if previous, exists := seen[key]; exists {
			return opererr.Wrap(ErrPeerExists,
				fmt.Sprintf("BGP peers %s and %s use the same remote TCP endpoint", previous.PeerKey(), peer.PeerKey()),
				"Only one configured peer can own a remote address and port on the same physical interface.", transportScopeFix)
		}
		seen[key] = peer
	}
	return nil
}

// transportAddressKeyOn creates a comparison key, never a configured address.
// Resolved nonzero interfaces share one decimal scope key. An unknown name or
// the default scope retains its exact spelling, as sameTransportAddressOn does.
func transportAddressKeyOn(addr netip.Addr, interfaces []net.Interface) netip.Addr {
	if addr.Zone() == "" {
		return addr
	}
	index, known := transportZoneIndex(addr.Zone(), interfaces)
	if !known {
		return addr
	}
	if index == 0 {
		return addr
	}
	return addr.WithZone(strconv.Itoa(index))
}

// sameTransportAddressOn uses one immutable interface listing. This keeps the
// name-first rule independent of the host's current names in its unit proof.
func sameTransportAddressOn(a, b netip.Addr, interfaces []net.Interface) bool {
	if a == b {
		return true
	}
	if a.WithZone("") != b.WithZone("") {
		return false
	}
	aIndex, aKnown := transportZoneIndex(a.Zone(), interfaces)
	if !aKnown {
		return false
	}
	if aIndex == 0 {
		return false
	}
	bIndex, bKnown := transportZoneIndex(b.Zone(), interfaces)
	if !bKnown {
		return false
	}
	return aIndex == bIndex
}

// transportZoneIndex follows Go net's ipv6ZoneCache.index: an interface name
// wins, even when entirely numeric, then dtoi consumes a decimal prefix and
// saturates at transportZoneIndexMax. Unknown names are unresolved, never an
// alias for another unknown name or for the default zone.
func transportZoneIndex(zone string, interfaces []net.Interface) (int, bool) {
	for _, iface := range interfaces {
		if iface.Name == zone {
			return iface.Index, true
		}
	}
	index := 0
	digits := 0
	for i := range len(zone) {
		digit := zone[i]
		if digit < '0' {
			break
		}
		if digit > '9' {
			break
		}
		digits++
		index = index*10 + int(digit-'0')
		if index >= transportZoneIndexMax {
			return transportZoneIndexMax, true
		}
	}
	return index, digits != 0
}

// listenerForAddressPort finds the existing socket under its original map key.
// An address event or a reload may spell its interface differently. The caller
// MUST hold r.mu; a returned key MUST be used to remove or replace that entry.
func (r *Reactor) listenerForAddressPort(addr netip.Addr, port int) (string, *Listener, bool) {
	key := net.JoinHostPort(addr.String(), strconv.Itoa(port))
	if listener, found := r.listeners[key]; found {
		return key, listener, true
	}
	for key, listener := range r.listeners {
		binding := newListenerBinding(netip.Addr{}, port, listener)
		if !binding.bound.IsValid() {
			// Simulation listeners may have no TCP endpoint. Their exact
			// configured key above is authoritative; no alias is inferred.
			continue
		}
		// No query identity participates while choosing the socket: only
		// its original declaration and actual endpoint can establish a match.
		binding = binding.withStoredDeclaration(key)
		if binding.matches(r, addr) == transportAddressSame {
			return key, listener, true
		}
	}
	return key, nil, false
}

// listenerBinding captures one existing socket's identity while r.mu is held.
// The declaration and the actual TCP endpoint can differ after wildcard binding.
// An absent TCP endpoint leaves declaration-only matching. An invalid stored
// key records its error and makes ownership Unknown. The zero value matches no
// valid address. This value owns no resource and is not cached.
type listenerBinding struct {
	address        netip.Addr
	port           int
	bound          netip.Addr
	declaration    netip.Addr
	declarationErr error
}

// newListenerBinding records the observed endpoint, when the socket has one on
// this port. The caller MUST hold r.mu through every use of the returned value.
func newListenerBinding(addr netip.Addr, port int, listener *Listener) listenerBinding {
	binding := listenerBinding{address: addr.Unmap(), port: port}
	if listener == nil {
		return binding
	}
	bound, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return binding
	}
	if bound == nil {
		return binding
	}
	endpoint := bound.AddrPort()
	if int(endpoint.Port()) != port {
		return binding
	}
	binding.bound = endpoint.Addr().Unmap()
	return binding
}

// withStoredDeclaration retains the configured identity of an existing socket.
// An invalid internal key makes the whole claim count Unknown; query and bound
// observations stay available, but cannot justify exclusive or absent ownership.
func (binding listenerBinding) withStoredDeclaration(key string) listenerBinding {
	endpoint, err := netip.ParseAddrPort(key)
	if err != nil {
		binding.declarationErr = err
		return binding
	}
	if int(endpoint.Port()) != binding.port {
		binding.declarationErr = fmt.Errorf("stored listener port %d differs from binding port %d",
			endpoint.Port(), binding.port)
		return binding
	}
	binding.declaration = endpoint.Addr().Unmap()
	return binding
}

// listenerBindingForAddressPort keeps the observed socket when runtime lookup
// established one. The caller MUST hold r.mu until the binding is no longer used.
func (r *Reactor) listenerBindingForAddressPort(addr netip.Addr, port int) listenerBinding {
	key, listener, found := r.listenerForAddressPort(addr, port)
	if !found {
		return newListenerBinding(addr, port, nil)
	}
	return newListenerBinding(addr, port, listener).withStoredDeclaration(key)
}

// matches counts the query, stored declaration or observed endpoint as the same
// binding. A positive identity overrides an unresolved alternate spelling;
// otherwise Unknown stays Unknown. The caller MUST hold r.mu.
func (binding listenerBinding) matches(r *Reactor, candidate netip.Addr) transportAddressMatch {
	if binding.declarationErr != nil {
		return transportAddressUnknown
	}
	match := r.sameTransportAddress(candidate, binding.address)
	if match == transportAddressSame {
		return match
	}
	if binding.declaration.IsValid() {
		declarationMatch := r.sameTransportAddress(candidate, binding.declaration)
		if declarationMatch == transportAddressSame {
			return declarationMatch
		}
		if declarationMatch == transportAddressUnknown {
			match = declarationMatch
		}
	}
	if binding.bound.IsValid() {
		boundMatch := r.sameTransportAddress(candidate, binding.bound)
		if boundMatch == transportAddressSame {
			return boundMatch
		}
		if boundMatch == transportAddressUnknown {
			return boundMatch
		}
	}
	return match
}
