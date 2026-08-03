// Design: ai/rules/repo-maintenance.md -- doctor check platform probe

//go:build !linux

package ifacera

// readIPv6Forwarding reports that the forwarding state is unknown. Only Linux
// exposes net.ipv6.conf.<device>.forwarding, and the Router Advertisement
// sender itself is Linux only, so there is nothing to read here. Returning
// known=false keeps the check silent rather than warning about a state it
// never saw.
func readIPv6Forwarding(string) (on, known bool) {
	return false, false
}
