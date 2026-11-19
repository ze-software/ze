// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- exact native CPE writes
// Related: pppoe_client.go -- sole session-result consumer

package iface

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

const pppoeNativeWait = 5 * time.Second

// nativeWrite grants no identity from a name: native was supplied by this
// session's original creator. The timeout bounds shutdown during a native ACK.
func (c *PPPoEClient) nativeWrite(write func(context.Context) error) (result error) {
	if err := c.beginNativeWrite(); err != nil {
		return err
	}
	defer func() { c.finishNativeWrite(result) }()
	ctx, cancel := context.WithTimeout(context.Background(), pppoeNativeWait)
	defer cancel()
	return invokePPPoENativeWrite(ctx, write)
}

func (c *PPPoEClient) addNativePair(local, peer netip.Addr) error {
	return c.nativeWrite(func(ctx context.Context) error {
		return c.native.AddAddressP2P(ctx, netip.PrefixFrom(local, 32), netip.PrefixFrom(peer, 32))
	})
}

func (c *PPPoEClient) removeNativePair(local, peer netip.Addr) error {
	return c.nativeWrite(func(ctx context.Context) error {
		return c.native.RemoveAddressP2P(ctx, netip.PrefixFrom(local, 32), netip.PrefixFrom(peer, 32))
	})
}

func (c *PPPoEClient) addNativeRoute(peer netip.Addr) error {
	return c.nativeWrite(func(ctx context.Context) error {
		return c.native.AddRoute(ctx, netip.PrefixFrom(netip.IPv4Unspecified(), 0), peer, c.cfg.RoutePriority, rtproto.Iface)
	})
}

func (c *PPPoEClient) removeNativeRoute(peer netip.Addr) error {
	return c.nativeWrite(func(ctx context.Context) error {
		return c.native.RemoveRoute(ctx, netip.PrefixFrom(netip.IPv4Unspecified(), 0), peer, c.cfg.RoutePriority, rtproto.Iface)
	})
}

// A settled ordinary route rejection preserves the existing warning/retry
// contract. Uncertain writes and lost authority stop this result immediately.
// These markers MUST come from the real provider; an errno is not a verdict.
func pppoeNativeRefused(err error) bool {
	return errors.Is(err, ErrNativePPPPending) || errors.Is(err, ErrNativePPPUnavailable)
}
