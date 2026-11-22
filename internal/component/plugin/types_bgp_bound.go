// Design: docs/architecture/api/process-protocol.md -- bound forwarding completion.
package plugin

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ReactorBoundForwardCoordinator is the captured variant of the existing cache
// and stored-route operations. It MUST use their shared evaluator and writer.
// The owner is captured from the actual authenticated Process, never wire data.
// Its zero owner and missing destination constraint MUST refuse admission.
type ReactorBoundForwardCoordinator interface {
	SendRefreshBound(context.Context, *selector.Selector, uint16, uint8, Sender) error
	ForwardUpdatesBound(context.Context, []uint64, []rpc.ForwardTarget, string, Sender, SelectedOwner) error
	RelayStoredRouteBound(context.Context, netip.Addr, []rpc.StoredRoute, Sender, SelectedOwner) error
}
