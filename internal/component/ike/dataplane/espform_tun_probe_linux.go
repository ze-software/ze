// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- both ESP forms on one inbound state: the complete path re-presents on a policy-checked device
// Overview: espform_tun_linux.go -- openESPFormTUNClone and settleESPFormTUNSysctls, the open and the sysctl rule this probe shares
// Related: espform_tun_probe_other.go -- the probe off Linux

//go:build linux

package dataplane

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// ESPFormTUNAvailable reports whether Ze can create its ze-espform interface, which the
// dual-form receiver needs, with IPv4 settings that keep the inbound policy check. It
// answers nil when both hold, and otherwise the same operator error a Child SA install
// would meet, naming the cause and its fix, reworded where it speaks of the interface
// Ze would create.
//
// The engine's configuration apply calls it, through its espFormTUNAvailable seam, on
// every apply that configures a peer (recordESPFormTUN, engine/apply.go), and the
// ipsec-esp-form-fast-path doctor check reports the outcome it recorded, so an operator
// sees the cause before a Child SA that must receive both ESP forms is refused. It asks two questions,
// in the order the receiver meets them:
//   - Can Ze open the TUN clone device /dev/net/tun? It opens the device the way the
//     receiver does, creating a missing node c 10 200 where the kernel allows it
//     (openESPFormTUNClone), and closes it at once.
//   - Would espFormTUNSysctls accept the new interface? predictESPFormTUNSysctls
//     applies the receiver's own rule to the values the interface would copy from
//     net.ipv4.conf.default.
//
// The probe creates no interface and writes no sysctl. An fd that never ran TUNSETIFF
// names no interface, and the sysctl question reads values and tests writability
// without writing (probeESPFormTUNSysctlWritable). The one thing it may create is a
// missing /dev/net/tun node (and /dev/net, when that is missing too), as the receiver's
// first install would.
//
// Safe for concurrent use: each call opens and closes its own fds, and shares no state.
func ESPFormTUNAvailable() error {
	fd, err := openESPFormTUNClone(espFormTUNHostOps)
	if err != nil {
		return err
	}
	if err := unix.Close(fd); err != nil {
		return fmt.Errorf("dataplane: close %s after the probe opened it: %w", espFormTUNClone, err)
	}
	// RFC 4301 Section 5.2
	return predictESPFormTUNSysctls(espFormTUNHostSysctls)
}

// predictESPFormTUNSysctls answers what espFormTUNSysctls would answer for a new
// ze-espform, before the interface exists. A new device starts with a copy of
// net.ipv4.conf.default (net/ipv4/devinet.c, inetdev_init: `memcpy(&in_dev->cnf,
// net->ipv4.devconf_dflt, sizeof(in_dev->cnf));`), so it reads conf/default where the
// receiver reads the device, and tests whether each write the receiver would make is
// refused where the receiver makes it. The decision is settleESPFormTUNSysctls, the
// receiver's own, so the prediction cannot drift from the install.
func predictESPFormTUNSysctls(ops espFormTUNSysctlOps) error {
	// RFC 4301 Section 5.2
	return settleESPFormTUNSysctls(ops, espFormTUNPredicted)
}
