// Design: docs/architecture/appliance/installer-initrd.md -- single-shot DHCPv4 via nclient4

//go:build linux

package disk

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
)

const dhcpTimeout = 10 * time.Second

// dhcpRequestModifiers are applied to every DISCOVER and REQUEST the installer
// sends. WithBroadcast(true) sets the BOOTP broadcast flag (0x8000).
//
// During DORA the client owns no IP, so it must ask the server to BROADCAST the
// reply. A server that honors a clear flag instead unicasts the OFFER/ACK to
// the offered address (yiaddr) and ARPs for it -- an address nobody answers yet
// -- so the lease is never delivered and the installer never gets network.
// iPXE and the old busybox udhcpc both set this flag; the pure-Go initrd
// regressed by omitting it, which stranded the installer on real hardware.
// Confirmed on the wire: the installer's Flags [none] DISCOVERs drew no reply
// while the server ARPed for the offered IP, whereas iPXE's Flags [Broadcast]
// requests were answered.
var dhcpRequestModifiers = []dhcpv4.Modifier{dhcpv4.WithBroadcast(true)}

func dhcpAcquire(ifName string) (*dhcpLease, error) {
	slog.Info("dhcp: starting DORA", "iface", ifName, "timeout", dhcpTimeout)
	client, err := nclient4.New(ifName)
	if err != nil {
		return nil, fmt.Errorf("dhcp client on %s: %w", ifName, err)
	}
	defer client.Close() //nolint:errcheck // single-shot

	ctx, cancel := context.WithTimeout(context.Background(), dhcpTimeout)
	defer cancel()

	lease, err := client.Request(ctx, dhcpRequestModifiers...)
	if err != nil {
		return nil, fmt.Errorf("dhcp request on %s: %w", ifName, err)
	}

	ack := lease.ACK
	if ack == nil {
		return nil, fmt.Errorf("dhcp on %s: no ACK", ifName)
	}

	ip := ack.YourIPAddr
	mask := ack.SubnetMask()
	if ip == nil || ip.IsUnspecified() {
		return nil, fmt.Errorf("dhcp on %s: no address in ACK", ifName)
	}

	ones, _ := mask.Size()
	if ones == 0 {
		mask = net.CIDRMask(24, 32)
		slog.Warn("dhcp: no subnet mask in ACK, defaulting to /24", "iface", ifName)
	}

	result := &dhcpLease{
		IP:    ip,
		Mask:  mask,
		Iface: ifName,
	}

	routers := ack.Router()
	if len(routers) > 0 {
		result.Router = routers[0]
	}

	slog.Info("dhcp: lease acquired", "iface", ifName, "ip", ip, "router", result.Router)
	return result, nil
}
