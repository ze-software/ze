// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- logical consumer fixtures

package iface

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// logicalNativeWriter adapts EXISTING logical recorder assertions at the test
// boundary. It never represents a CreatedUnit or a native binding receipt.
// A nil recorder is the old lifecycle-only fixture, which asserted no writes.
// Device strings are recorded only here; production receives no device string.
type logicalNativeWriter struct {
	backend Backend
	device  string
}

func (w logicalNativeWriter) SetMTU(_ context.Context, mtu int) error {
	if w.backend == nil {
		return nil
	}
	return w.backend.SetMTU(w.device, mtu)
}
func (w logicalNativeWriter) SetAdminUp(context.Context) error {
	if w.backend == nil {
		return nil
	}
	return w.backend.SetAdminUp(w.device)
}
func (logicalNativeWriter) SetAdminDown(context.Context) error {
	panic("unexpected logical native down")
}
func (logicalNativeWriter) SetAlias(context.Context, string) error {
	panic("unexpected logical native alias")
}
func (w logicalNativeWriter) AddAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	if w.backend == nil {
		return nil
	}
	if local == peer && local.Bits() == 64 {
		return w.backend.AddAddress(w.device, local.String())
	}
	return w.backend.AddAddressP2P(w.device, local.String(), peer.String())
}
func (w logicalNativeWriter) RemoveAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	if w.backend == nil {
		return nil
	}
	if local == peer && local.Bits() == 64 {
		return w.backend.RemoveAddress(w.device, local.String())
	}
	return w.backend.RemoveAddressP2P(w.device, local.String(), peer.String())
}
func (w logicalNativeWriter) AddRoute(_ context.Context, destination netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	if w.backend == nil {
		return nil
	}
	return w.backend.AddRoute(w.device, destination.String(), gateway.String(), metric, proto)
}
func (w logicalNativeWriter) RemoveRoute(_ context.Context, destination netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	if w.backend == nil {
		return nil
	}
	return w.backend.RemoveRoute(w.device, destination.String(), gateway.String(), metric, proto)
}
func (logicalNativeWriter) Settle(context.Context) error { return nil }
func (logicalNativeWriter) Close() error                 { return nil }
func (logicalNativeWriter) ClosePending() bool           { return false }

var _ NativePPPWriter = logicalNativeWriter{}
