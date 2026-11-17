// Design: docs/architecture/api/process-protocol.md -- selected refresh authority.
// Related: selected.go -- authenticated process lifetime and selected output.
package plugin

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectedRefresh owns one validated scalar request. Its zero value and nil
// pointer are invalid; construction grants no peer or process authority.
type SelectedRefresh struct {
	destination netip.Addr
	family      family.Family
	input       rpc.SelectRefreshInput
}

func NewSelectedRefresh(input rpc.SelectRefreshInput) (*SelectedRefresh, error) {
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	destination, err := netip.ParseAddr(input.Destination)
	if err != nil {
		return nil, fmt.Errorf("select-refresh destination is invalid: %w", err)
	}
	fam, registered := family.LookupFamily(input.Family)
	if !registered {
		return nil, fmt.Errorf("select-refresh family %q is not registered; use a registered negotiated family", input.Family)
	}
	return &SelectedRefresh{destination: destination, family: fam, input: input}, nil
}

func (request *SelectedRefresh) Destination() netip.Addr       { return request.destination }
func (request *SelectedRefresh) Family() family.Family         { return request.family }
func (request *SelectedRefresh) Input() rpc.SelectRefreshInput { return request.input }

// ReactorSelectedRefresher is optional protocol delegation. An unsupported
// reactor MUST refuse; it MUST NOT return an empty successful receipt.
type ReactorSelectedRefresher interface {
	SelectRefresh(context.Context, *SelectedRefresh, SelectedOwner) (*rpc.SelectRefreshOutput, error)
}
