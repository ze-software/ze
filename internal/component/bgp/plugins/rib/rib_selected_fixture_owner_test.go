// Design: docs/architecture/plugin/rib-storage-design.md -- learned fixture admission.
package rib

import "net/netip"

// destinationUp preserves the existing learned-only fixture's synthetic identity
// assignment. Equality of owner and Session here is a fixture choice, never an
// engine identity rule. New origin tests always supply distinct real fields.
func (d *selectedDispatch) destinationUp(peer netip.Addr, session uint64) error {
	return d.destinationOwnerUp(peer, session, session)
}
