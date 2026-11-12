// Design: docs/architecture/testing/interop.md -- the TUN device a lab Ze running IPsec holds
// Related: pppdevice.go -- the PPP device the PPP labs hand their containers the same way
//
// Ze's XFRM dataplane creates the ze-espform TUN device for every Child SA
// that receives UDP-encapsulated ESP, and refuses to install the SA when it
// cannot open the TUN clone device (openESPFormTUN,
// internal/component/ike/dataplane/espform_tun_linux.go). A container sees
// only the device nodes it was given, so every lab container that runs Ze
// with `vpn ipsec` holds the host's /dev/net/tun, declared here once.

package interoplab

// TUNDevice is the host's TUN clone device, which module tun provides; Ze's
// kernel builds it in (CONFIG_TUN=y, gokrazy/kernel/runtime.config). Naming
// a new interface on it with TUNSETIFF needs CAP_NET_ADMIN in the container's
// network namespace, which every lab peer that runs IPsec already holds for
// XFRM, and setting the new device's sysctls needs a writable /proc/sys
// (NetSysctlWriteArguments).
const TUNDevice = "/dev/net/tun"

// TUNDeviceArguments hands a container the host's TUN clone device.
func TUNDeviceArguments() []string {
	return []string{"--device", TUNDevice}
}
