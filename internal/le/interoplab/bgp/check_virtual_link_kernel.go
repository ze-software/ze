// Design: docs/architecture/testing/interop.md -- observe native transit forwarding.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func virtualLinkTransitGateway(ctx context.Context, lab interoplab.CheckerLab, v6 bool) (netip.Addr, error) {
	if !v6 {
		return netip.AddrFrom4([4]byte{10, 200, 0, 2}), nil
	}
	gateway, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "transit IPv6 link-local gateway"},
		func(ctx context.Context) (netip.Addr, error) {
			answer, err := lab.Query(ctx, peerFRRTransit, []string{"ip", "-j", "-6", ipObjectAddress, ipActionShow, "dev", "eth1"}, nil)
			if err != nil {
				return netip.Addr{}, err
			}
			var interfaces []struct {
				Addresses []struct {
					Family string `json:"family"`
					Local  string `json:"local"`
					Scope  string `json:"scope"`
				} `json:"addr_info"`
			}
			if err := json.Unmarshal([]byte(answer), &interfaces); err != nil {
				return netip.Addr{}, err
			}
			var gateway netip.Addr
			for _, iface := range interfaces {
				for _, address := range iface.Addresses {
					if address.Family != "inet6" || address.Scope != "link" {
						continue
					}
					parsed, err := netip.ParseAddr(address.Local)
					if err != nil {
						return netip.Addr{}, err
					}
					if !parsed.IsLinkLocalUnicast() {
						return netip.Addr{}, errors.New("transit link-scope address is not link-local unicast")
					}
					if gateway.IsValid() {
						return netip.Addr{}, errors.New("transit interface has ambiguous IPv6 link-local gateways")
					}
					gateway = parsed
				}
			}
			return gateway, nil
		}, func(gateway netip.Addr) bool { return gateway.IsValid() })
	return gateway, err
}

func waitVirtualLinkKernelRoute(ctx context.Context, lab interoplab.CheckerLab, v6 bool, gateway netip.Addr, present bool) error {
	family, prefix := "-4", "10.200.0.4/30"
	if v6 {
		family, prefix = "-6", "2001:db8:200::/64"
	}
	var last string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "Ze kernel transit route via intermediate router"},
		func(ctx context.Context) (bool, error) {
			answer, err := lab.Query(ctx, "ze", []string{"ip", "-j", family, ipObjectRoute, ipActionShow, "exact", prefix}, nil)
			last = answer
			if err != nil {
				return false, err
			}
			return virtualLinkKernelRoutePresent(answer, prefix, gateway)
		}, func(found bool) bool { return found == present })
	return withLastOutput(err, last)
}

func virtualLinkKernelRoutePresent(answer, prefix string, gateway netip.Addr) (bool, error) {
	var routes []struct {
		Destination string `json:"dst"`
		Gateway     string `json:"gateway"`
		Device      string `json:"dev"`
	}
	if err := json.Unmarshal([]byte(answer), &routes); err != nil {
		return false, err
	}
	if routes == nil {
		return false, errors.New("kernel returned no route observation")
	}
	if len(routes) == 0 {
		return false, nil
	}
	if len(routes) != 1 {
		return false, errors.New("kernel has multiple routes for the single-path transit prefix")
	}
	route := routes[0]
	if route.Destination != prefix {
		return false, fmt.Errorf("kernel answered transit prefix %q, want %s", route.Destination, prefix)
	}
	if route.Device != "eth1" {
		return false, fmt.Errorf("kernel transit egress %q, want eth1", route.Device)
	}
	parsed, err := netip.ParseAddr(route.Gateway)
	if err != nil {
		return false, fmt.Errorf("kernel transit gateway: %w", err)
	}
	if parsed != gateway {
		return false, fmt.Errorf("kernel transit gateway %s, want intermediate router %s", parsed, gateway)
	}
	return true, nil
}
