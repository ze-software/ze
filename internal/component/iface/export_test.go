// Design: docs/features/interfaces.md — Interface plugin registration
// Related: resolve.go — the logical-to-OS resolver these seams publish to
// Related: register.go — the DHCP client factory ifacedhcp installs

package iface

import "github.com/ze-software/ze/pkg/ze"

// SetResolverMappingForTest publishes a logical-to-OS name map to the shared
// resolver, the map a config apply publishes through setResolverConfig. It
// exists for the external iface_test package, which cannot run a config apply
// without the whole interface plugin and cannot reach the unexported resolver.
// A test MUST restore the empty mapping in its cleanup, because the resolver is
// process-wide.
func SetResolverMappingForTest(logicalToOS map[string]string) {
	globalResolver.setMapping(logicalToOS, nil)
}

// DHCPClientFactoryForTest returns the factory SetDHCPClientFactory installed,
// the one the interface plugin calls for a unit with dhcp or dhcpv6 enabled.
// It is nil until the ifacedhcp package is linked in.
func DHCPClientFactoryForTest() func(string, string, ze.EventBus, bool, bool, string, string, bool, int, string, int) (DHCPStopper, error) {
	return dhcpClientFactory
}

// BindResolverEventsForTest subscribes the shared resolver to eb, the binding a
// config apply makes through bindResolverEvents, so a test that starts the
// backend's monitor on eb drives Subscribe consumers through the real chain:
// netlink monitor, bus, resolver, subscriber. The returned function removes the
// subscriptions and lets a later test bind again; a test MUST call it in its
// cleanup, because the resolver is process-wide and binds only once.
func BindResolverEventsForTest(eb ze.EventBus) func() {
	globalResolver.bindEvents(eb)
	return func() {
		globalResolver.mu.Lock()
		unsub := globalResolver.unsub
		globalResolver.unsub = nil
		globalResolver.bound = false
		globalResolver.mu.Unlock()
		for _, u := range unsub {
			u()
		}
	}
}
