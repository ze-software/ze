// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- logical connected consumer

package pppoeclient

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// consumerNativeWriter preserves the old logical recorder and setup gate. It
// grants no kernel unit or native proof; real binding is tested separately by
// TestNativeCPEConsumerKeepsOriginalAcrossNameReplacement.
type consumerNativeWriter struct{ backend *changeConsumerBackend }

func (w consumerNativeWriter) SetMTU(_ context.Context, mtu int) error {
	return w.backend.SetMTU("wan", mtu)
}
func (w consumerNativeWriter) SetAdminUp(context.Context) error { return w.backend.SetAdminUp("wan") }
func (consumerNativeWriter) SetAdminDown(context.Context) error { panic("unexpected logical down") }
func (consumerNativeWriter) SetAlias(context.Context, string) error {
	panic("unexpected logical alias")
}
func (w consumerNativeWriter) AddAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	return w.backend.AddAddressP2P("wan", local.String(), peer.String())
}
func (w consumerNativeWriter) RemoveAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	return w.backend.RemoveAddressP2P("wan", local.String(), peer.String())
}
func (w consumerNativeWriter) AddRoute(_ context.Context, destination netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	return w.backend.AddRoute("wan", destination.String(), gateway.String(), metric, proto)
}
func (w consumerNativeWriter) RemoveRoute(_ context.Context, destination netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	return w.backend.RemoveRoute("wan", destination.String(), gateway.String(), metric, proto)
}
func (consumerNativeWriter) Settle(context.Context) error { return nil }
func (consumerNativeWriter) Close() error                 { return nil }
func (consumerNativeWriter) ClosePending() bool           { return false }

var _ iface.NativePPPWriter = consumerNativeWriter{}
