// Design: docs/architecture/core-design.md -- Linux route protocol IDs
//
// Package rtproto defines Linux route protocol IDs used to mark Ze-owned
// kernel routes by producer.
package rtproto

// Proto is the rtm_protocol value a route carries in the kernel. A route
// backend takes one from its caller, so the caller states which producer owns
// the route it installs, and which producer's routes a delete is allowed to
// match.
type Proto int

const (
	// Any matches a route whatever its protocol. A delete with Any removes a
	// route this box did not install, so a caller that wants it must name it:
	// an omitted value is never Any by accident. Only a caller that must
	// remove kernel-installed routes uses it, such as the RA default-route
	// cleanup in internal/component/iface.
	Any Proto = 0
	// FIBKernel marks routes installed by the BGP/sysrib FIB kernel plugin.
	FIBKernel = 250
	// Static marks routes installed by the static route plugin.
	Static = 251
	// PolicyRoute marks auto-routes installed by the policy routing plugin.
	PolicyRoute = 252
	// Iface marks routes installed by the interface layer: the DHCPv4 client,
	// the IPv6 RA default-route manager, the PPPoE client, and PPP NCPs.
	// It is deliberately NOT a value IsZe reports true for -- see IsZe.
	Iface Proto = 253
)

// IsZe reports whether protocol is one of Ze's producer-specific route owners.
//
// Iface is absent on purpose. routewatch suppresses events for every protocol
// this reports true for (internal/core/routewatch/routewatch.go, (*Watcher).deliver),
// so adding Iface here would silence DHCP, RA and PPPoE route churn for every
// subscriber.
func IsZe(protocol int) bool {
	switch Proto(protocol) {
	case FIBKernel, Static, PolicyRoute:
		return true
	case Any, Iface:
		return false
	}
	return false
}

// Name returns the display name for a route protocol Ze installs. Membership
// here is rendering only, and is wider than IsZe: a protocol Ze stamps still
// needs a name in route output even when its events stay visible.
func Name(protocol int) (string, bool) {
	name, ok := names[Proto(protocol)]
	return name, ok
}

var names = map[Proto]string{
	FIBKernel:   "ze-fib",
	Static:      "ze-static",
	PolicyRoute: "ze-policy-route",
	Iface:       "ze-iface",
}
