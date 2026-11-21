// Design: docs/research/l2tpv2-ze-integration.md -- kernel IPv6 control carrier
// Related: ipv6_linux.go -- socket creation in the session's kernel interface

package kernel

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type raSocket interface {
	ReadFrom([]byte) (int, *ipv6.ControlMessage, net.Addr, error)
	WriteTo([]byte, *ipv6.ControlMessage, net.Addr) (int, error)
	SetReadDeadline(time.Time) error
	Close() error
}

type dhcpSocket interface {
	ReadFrom([]byte) (int, net.Addr, error)
	WriteTo([]byte, net.Addr) (int, error)
	SetReadDeadline(time.Time) error
	Close() error
}

// ipv6Carrier owns two dedicated sockets. The service joins both read workers
// before Close; independent read locks keep deadline cancellation scoped to the
// worker that requested it. Close is safe for concurrent use and idempotent.
type ipv6Carrier struct {
	ra       raSocket
	dhcp     dhcpSocket
	ifname   string
	ifindex  int
	raRead   sync.Mutex
	dhcpRead sync.Mutex
	cleanup  *ipv6SocketCleanup
}

var _ ppp.IPv6Carrier = (*ipv6Carrier)(nil)

func newIPv6Carrier(ra raSocket, dhcp dhcpSocket, ifname string, ifindex int) *ipv6Carrier {
	return &ipv6Carrier{ra: ra, dhcp: dhcp, ifname: ifname, ifindex: ifindex,
		cleanup: &ipv6SocketCleanup{ra: ra, dhcp: ipv6DHCPCleanupOwner(dhcp)}}
}

// acquireIPv6Carrier owns partial socket acquisition: once RA opens, failure
// to acquire DHCPv6 closes RA and preserves both the setup and close failures.
// Failed cleanup returns a non-usable carrier with the error. The caller MUST
// retain that owner until its exact cleanup is confirmed.
func acquireIPv6Carrier(ctx context.Context, ifname string, ifindex int, openRA func(context.Context) (raSocket, error), openDHCP func(context.Context) (dhcpSocket, error)) (ppp.IPv6Carrier, error) { //nolint:unparam // ipv6_linux.go passes the real interface name; only the tests, linted alone off Linux, pass "ppp7".
	ra, err := openRA(ctx)
	if err != nil {
		return failedIPv6Acquisition(ra, nil, err)
	}
	dhcp, err := openDHCP(ctx)
	if err != nil {
		return failedIPv6Acquisition(ra, dhcp, err)
	}
	return newIPv6Carrier(ra, dhcp, ifname, ifindex), nil
}

func (c *ipv6Carrier) ReadRS(ctx context.Context) error {
	c.raRead.Lock()
	defer c.raRead.Unlock()
	var payload [1500]byte
	// This worker reads until a valid solicitation or cancellation arrives.
	// Invalid wire messages are silently discarded as RFC 4861 requires.
	for {
		var n int
		var control *ipv6.ControlMessage
		var address net.Addr
		err := carrierRead(ctx, c.ra.SetReadDeadline, func() error {
			var readErr error
			n, control, address, readErr = c.ra.ReadFrom(payload[:])
			return readErr
		})
		if err != nil {
			return err
		}
		if control == nil {
			continue
		}
		if control.IfIndex != c.ifindex {
			continue
		}
		peer, ok := address.(*net.IPAddr)
		if !ok {
			return errors.New("PPP solicitation socket returned a non-IP peer")
		}
		if peer.Zone != "" {
			if peer.Zone != c.ifname {
				continue
			}
		}
		source, ok := netip.AddrFromSlice(peer.IP)
		if !ok {
			continue
		}
		if ppp.ValidRouterSolicitation(payload[:n], source, control.HopLimit) {
			return nil
		}
	}
}

func (c *ipv6Carrier) WriteRA(payload []byte, hopLimit int) error {
	control := &ipv6.ControlMessage{IfIndex: c.ifindex, HopLimit: hopLimit}
	peer := &net.IPAddr{IP: net.IPv6linklocalallnodes, Zone: c.ifname}
	n, err := c.ra.WriteTo(payload, control, peer)
	if err != nil {
		return err
	}
	if n != len(payload) {
		return io.ErrShortWrite
	}
	return nil
}

func (c *ipv6Carrier) ReadDHCPv6(ctx context.Context, payload []byte) (int, netip.AddrPort, error) {
	c.dhcpRead.Lock()
	defer c.dhcpRead.Unlock()
	var n int
	var address net.Addr
	err := carrierRead(ctx, c.dhcp.SetReadDeadline, func() error {
		var readErr error
		n, address, readErr = c.dhcp.ReadFrom(payload)
		return readErr
	})
	if err != nil {
		return 0, netip.AddrPort{}, err
	}
	peer, ok := address.(*net.UDPAddr)
	if !ok {
		return 0, netip.AddrPort{}, errors.New("PPP DHCPv6 socket returned a non-UDP peer")
	}
	addressPort := peer.AddrPort()
	if !addressPort.Addr().Is6() {
		return 0, netip.AddrPort{}, errors.New("PPP DHCPv6 socket returned a non-IPv6 peer")
	}
	if addressPort.Addr().Is4In6() {
		return 0, netip.AddrPort{}, errors.New("PPP DHCPv6 socket returned an IPv4-mapped peer")
	}
	if peer.Zone != "" {
		if peer.Zone != c.ifname {
			return 0, netip.AddrPort{}, errors.New("PPP DHCPv6 peer belongs to another interface")
		}
	}
	if addressPort.Addr().IsLinkLocalUnicast() {
		if peer.Zone == "" {
			addressPort = netip.AddrPortFrom(addressPort.Addr().WithZone(c.ifname), addressPort.Port())
		}
	}
	return n, addressPort, nil
}

func (c *ipv6Carrier) WriteDHCPv6(payload []byte, peer netip.AddrPort) (int, error) {
	if !peer.IsValid() {
		return 0, errors.New("PPP DHCPv6 reply has no peer address")
	}
	if !peer.Addr().Is6() {
		return 0, errors.New("PPP DHCPv6 reply has a non-IPv6 peer")
	}
	if peer.Addr().Is4In6() {
		return 0, errors.New("PPP DHCPv6 reply has an IPv4-mapped peer")
	}
	if peer.Addr().Zone() != "" {
		if peer.Addr().Zone() != c.ifname {
			return 0, errors.New("PPP DHCPv6 reply belongs to another interface")
		}
	}
	if peer.Addr().IsLinkLocalUnicast() {
		if peer.Addr().Zone() == "" {
			peer = netip.AddrPortFrom(peer.Addr().WithZone(c.ifname), peer.Port())
		}
	}
	return c.dhcp.WriteTo(payload, net.UDPAddrFromAddrPort(peer))
}

// Close preserves the first cleanup result. The caller MUST stop and join both
// readers first, and MUST retain a failed owner for RetryIPv6CarrierCleanup.
func (c *ipv6Carrier) Close() error {
	return c.cleanup.Close()
}

// carrierRead interrupts only a read deadline. It never closes the socket,
// because the caller still owes the final Router Advertisement. Waiting for a
// started cancellation callback before clearing the deadline prevents a late
// callback from poisoning the next read of this lane.
func carrierRead(ctx context.Context, setDeadline func(time.Time) error, read func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(ctx, func() {
		interruptErr = setDeadline(time.Now())
		close(done)
	})
	readErr := read()
	if !stop() {
		<-done
	}
	resetErr := setDeadline(time.Time{})
	if err := ctx.Err(); err != nil {
		return errors.Join(err, interruptErr, resetErr)
	}
	return errors.Join(readErr, resetErr)
}

// RetryIPv6CarrierCleanup reconciles only failed captured socket owners. Normal
// Close keeps its original cached result. Caller MUST first stop/join service
// readers and call Close; retry never repeats joins/final RA or closes PPP I/O.
func (c *ipv6Carrier) RetryIPv6CarrierCleanup(ctx context.Context) error { return c.cleanup.retry(ctx) }
