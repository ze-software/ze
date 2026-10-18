// Design: docs/features/interfaces.md -- IPv6 reverse-path check
// Overview: ifacevpp.go -- vppBackendImpl

package ifacevpp

import (
	"errors"

	"github.com/ze-software/ze/internal/component/iface"
)

// errIPv6RPFUnsupported is the refusal of an IPv6 rpf-check on this backend.
// VPP forwards in its own dataplane, so a Linux nftables check would only see
// the traffic VPP punts to the host, and this backend programs no VPP source
// check. Refusing is the only answer that does not leave the operator
// believing the check is on.
var errIPv6RPFUnsupported = errors.New("ifacevpp: ipv6 rpf-check is not implemented on the vpp backend, " +
	"so the check cannot be enforced; remove `ipv6 rpf-check` from the unit, or run the interface on the netlink backend")

// ReplaceIPv6RPF accepts the empty set, which is what every config without an
// IPv6 rpf-check asks for, and refuses any check.
func (b *vppBackendImpl) ReplaceIPv6RPF(checks []iface.IPv6RPFCheck) error {
	if len(checks) == 0 {
		return nil
	}
	return errIPv6RPFUnsupported
}
