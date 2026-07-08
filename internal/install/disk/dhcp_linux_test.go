// VALIDATES: AC-5 (DHCP via nclient4, no udhcpc)
// PREVENTS: dependency on external udhcpc binary in the initrd;
// PREVENTS: DORA DISCOVER with a clear BOOTP broadcast flag (0x0000), which lets
// the server unicast the OFFER to the not-yet-owned yiaddr and strands the
// installer with no lease on real hardware.

//go:build linux

package disk

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestDHCPAcquireSignature(t *testing.T) {
	var fn = dhcpAcquire
	_ = fn
}

// TestDHCPRequestsBroadcast asserts the installer's DISCOVER/REQUEST modifiers
// set the BOOTP broadcast flag. A clear flag caused the pure-Go initrd
// regression: the server unicast the OFFER to the offered IP (ARPing for an
// address the client cannot yet answer), so no lease ever arrived.
func TestDHCPRequestsBroadcast(t *testing.T) {
	hwaddr := net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	d, err := dhcpv4.NewDiscovery(hwaddr, dhcpRequestModifiers...)
	if err != nil {
		t.Fatalf("build discovery: %v", err)
	}
	if !d.IsBroadcast() {
		t.Fatal("installer DHCP DISCOVER must set the broadcast flag (0x8000); a clear flag strands DORA when the server unicasts the OFFER to the unconfigured client IP")
	}
}
