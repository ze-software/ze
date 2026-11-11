// Design: docs/architecture/fleet-config.md -- one address cannot carry both the plugin acceptor and the managed listener
// Related: acceptor.go -- hubAcceptorServer binds Servers[0]
// Related: server/managed_serve.go -- ManagedServer.Start tells a collision from any other bind failure
// Related: doctor/check_managed_listener.go -- the local check that reports the collision from the config

package plugin

import "net"

// HubAddressesCollide reports whether two hub server blocks ask the kernel for
// the same listening socket. The local collision check and the managed
// server's bind both judge with it, so the two agree on which failure is the
// collision.
//
// The port has to match, and the hosts have to overlap. Two named hosts overlap
// when they are equal. An unspecified host (0.0.0.0, ::, or no ip leaf at all)
// covers every address on the machine. It therefore overlaps any host on that
// port, and tls.Listen refuses the second bind with EADDRINUSE either way round.
//
// Port 0 is the one case a shared block does not collide. The kernel picks a
// free port for each listener. Such a hub still cannot be reached by a remote
// managed client. No client config can name a port the kernel has not chosen
// yet. That is a different defect and this predicate does not claim it.
func HubAddressesCollide(first, second HubServerConfig) bool {
	if first.Port != second.Port {
		return false
	}
	if first.Port == 0 {
		return false
	}
	if first.Host == second.Host {
		return true
	}
	return hubHostIsUnspecified(first.Host) || hubHostIsUnspecified(second.Host)
}

// hubHostIsUnspecified reports whether a host names every address on the
// machine. An empty ip leaf reaches tls.Listen as ":port", which binds the
// wildcard, so it counts as unspecified too.
func hubHostIsUnspecified(host string) bool {
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false // A name, not an address. Resolution is the kernel's job at bind time.
	}
	return ip.IsUnspecified()
}
