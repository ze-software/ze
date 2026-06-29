// Design: plan/spec-installer-initrd-pure-go.md -- single-shot DHCPv4 via nclient4

//go:build linux

package disk

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
)

const dhcpTimeout = 10 * time.Second

func dhcpAcquire(ifName string) (*dhcpLease, error) {
	slog.Info("dhcp: starting DORA", "iface", ifName, "timeout", dhcpTimeout)
	client, err := nclient4.New(ifName)
	if err != nil {
		return nil, fmt.Errorf("dhcp client on %s: %w", ifName, err)
	}
	defer client.Close() //nolint:errcheck // single-shot

	ctx, cancel := context.WithTimeout(context.Background(), dhcpTimeout)
	defer cancel()

	lease, err := client.Request(ctx)
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
