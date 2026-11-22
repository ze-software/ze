// Design: docs/architecture/api/process-protocol.md -- captured legacy replay.
package plugin

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (c *Coordinator) boundForwardReactor() (ReactorBoundForwardCoordinator, error) {
	reactor := c.getReactor()
	if reactor == nil {
		return nil, ErrNoReactor
	}
	bound, ok := reactor.(ReactorBoundForwardCoordinator)
	if !ok {
		return nil, errors.New("reactor has no bound forwarding contract; rebuild the BGP reactor")
	}
	return bound, nil
}
func (c *Coordinator) ForwardUpdatesBound(ctx context.Context, ids []uint64, destinations []rpc.ForwardTarget, name string, sender Sender, owner SelectedOwner) error {
	reactor, err := c.boundForwardReactor()
	if err != nil {
		return err
	}
	return reactor.ForwardUpdatesBound(ctx, ids, destinations, name, sender, owner)
}
func (c *Coordinator) RelayStoredRouteBound(ctx context.Context, destination netip.Addr, routes []rpc.StoredRoute, sender Sender, owner SelectedOwner) error {
	reactor, err := c.boundForwardReactor()
	if err != nil {
		return err
	}
	return reactor.RelayStoredRouteBound(ctx, destination, routes, sender, owner)
}

func (c *Coordinator) SendRefreshBound(ctx context.Context, sel *selector.Selector, afi uint16, safi uint8, sender Sender) error {
	reactor, err := c.boundForwardReactor()
	if err != nil {
		return err
	}
	return reactor.SendRefreshBound(ctx, sel, afi, safi, sender)
}
