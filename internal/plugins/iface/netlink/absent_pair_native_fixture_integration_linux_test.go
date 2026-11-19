//go:build integration && linux

// Design: docs/features/interfaces.md -- exact point-to-point removal
// Related: pppoe_absent_pair_integration_linux_test.go -- real Linux survivor

package ifacenetlink

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// absentPairNativeWriter supplies the test-owned dummy to the actual consumer.
// This proves exact Linux pair removal, not original PPP binding or identity.
// Only the explicit fixture can select ppp7; no public Created grant is forged.
type absentPairNativeWriter struct{ backend *absentPairBackend }

func (w absentPairNativeWriter) SetMTU(_ context.Context, mtu int) error {
	return w.backend.SetMTU("ppp7", mtu)
}
func (w absentPairNativeWriter) SetAdminUp(context.Context) error {
	return w.backend.SetAdminUp("ppp7")
}
func (absentPairNativeWriter) SetAdminDown(context.Context) error {
	panic("unexpected fixture admin down")
}
func (absentPairNativeWriter) SetAlias(context.Context, string) error {
	panic("unexpected fixture alias")
}
func (w absentPairNativeWriter) AddAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	return w.backend.AddAddressP2P("ppp7", local.String(), peer.String())
}
func (w absentPairNativeWriter) RemoveAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	return w.backend.RemoveAddressP2P("ppp7", local.String(), peer.String())
}
func (absentPairNativeWriter) AddRoute(context.Context, netip.Prefix, netip.Addr, int, rtproto.Proto) error {
	panic("unexpected fixture route add")
}
func (absentPairNativeWriter) RemoveRoute(context.Context, netip.Prefix, netip.Addr, int, rtproto.Proto) error {
	panic("unexpected fixture route remove")
}
func (absentPairNativeWriter) Settle(context.Context) error { return nil }
func (absentPairNativeWriter) Close() error                 { return nil }
func (absentPairNativeWriter) ClosePending() bool           { return false }

var _ iface.NativePPPWriter = absentPairNativeWriter{}
