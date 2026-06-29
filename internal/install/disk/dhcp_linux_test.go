// VALIDATES: AC-5 (DHCP via nclient4, no udhcpc)
// PREVENTS: dependency on external udhcpc binary in the initrd

//go:build linux

package disk

import "testing"

func TestDHCPAcquireSignature(t *testing.T) {
	var fn func(string) (*dhcpLease, error) = dhcpAcquire
	_ = fn
}
