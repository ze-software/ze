// Design: docs/research/l2tpv2-ze-integration.md -- explicit logical mutation port

package kernel

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// Only logical tests adapt the pre-existing recording backend. This type is
// unavailable to public construction and is never a native receipt producer.
type sessionTestMutationOperations struct {
	backend  InterfaceOperations
	identity ppp.SessionIdentity
	ops      *sessionOps
}

func (operations sessionTestMutationOperations) SetMTU(_ context.Context, mtu int) error {
	return operations.backend.SetMTU(operations.identity.InterfaceName, mtu)
}
func (operations sessionTestMutationOperations) SetAdminUp(context.Context) error {
	return operations.backend.SetAdminUp(operations.identity.InterfaceName)
}
func (operations sessionTestMutationOperations) SetAdminDown(context.Context) error {
	return errors.New("logical session has no admin-down port")
}
func (operations sessionTestMutationOperations) SetAlias(_ context.Context, tag string) error {
	if operations.ops == nil {
		return errors.New("logical session has no native birth stamp port")
	}
	if operations.ops.stampBirth == nil {
		return errors.New("logical session has no native birth stamp port")
	}
	return operations.ops.stampBirth(operations.identity, tag)
}
func (operations sessionTestMutationOperations) AddAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	return operations.backend.AddAddressP2P(operations.identity.InterfaceName, local.String(), peer.String())
}
func (operations sessionTestMutationOperations) RemoveAddressP2P(_ context.Context, local, _ netip.Prefix) error {
	return operations.backend.RemoveAddress(operations.identity.InterfaceName, local.String())
}
func (operations sessionTestMutationOperations) AddRoute(_ context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	return operations.backend.AddRoute(operations.identity.InterfaceName, destination.String(), gateway.String(), metric, protocol)
}
func (operations sessionTestMutationOperations) RemoveRoute(_ context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	return operations.backend.RemoveRoute(operations.identity.InterfaceName, destination.String(), gateway.String(), metric, protocol)
}
func (sessionTestMutationOperations) Settle(context.Context) error { return nil }
func (sessionTestMutationOperations) Close() error                 { return nil }
func (sessionTestMutationOperations) ClosePending() bool           { return false }
