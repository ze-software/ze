// Design: docs/architecture/api/process-protocol.md -- exact deciding connection.
// Related: selected_current.go -- one comparable authenticated process token.
package plugin

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// WithOriginIntake binds the optional mutation callback before publication.
// A fresh private reader record preserves immutability of any earlier value.
// Neither a name lookup nor the handler itself supplies a destination grant.
func (owner SelectedOwner) WithOriginIntake(handler rpc.OriginIntakeHandler) (SelectedOwner, error) {
	if err := owner.Current(); err != nil {
		return SelectedOwner{}, err
	}
	if owner.currentSelection == nil {
		return SelectedOwner{}, errors.New("origin-intake requires the captured process connection")
	}
	if handler == nil {
		return SelectedOwner{}, errors.New("origin-intake handler is missing")
	}
	if owner.currentSelection.originIntake != nil {
		return SelectedOwner{}, errors.New("origin-intake handler is already bound")
	}
	connection := *owner.currentSelection
	connection.originIntake = handler
	owner.currentSelection = &connection
	return owner, nil
}

func (owner SelectedOwner) HasOriginIntake() bool {
	return owner.currentSelection != nil && owner.currentSelection.originIntake != nil
}

// OriginIntake mutates only the exact authenticated connection captured before
// this owner was published. The callback MUST join cancellation before return.
func (owner SelectedOwner) OriginIntake(ctx context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if !owner.HasOriginIntake() {
		return nil, errors.New("deciding process did not register origin-intake")
	}
	output, err := owner.currentSelection.originIntake(ctx, input)
	if current := owner.Current(); current != nil {
		return nil, errors.Join(err, current)
	}
	if err == nil || output != nil {
		if validation := rpc.ValidateOriginIntakeOutput(input, output); validation != nil {
			return nil, validation
		}
	}
	return output, err
}

type originProducerContextKey struct{}
type originProducerContext struct {
	owner SelectedOwner
	err   error
}

// WithOriginProducerContext is for trusted command ingress only. The original
// authenticated Process token is captured before any selector/queue handoff;
// metadata and a caller-supplied process name cannot manufacture this value.
// An error remains explicit so the engine cannot downgrade it to operator.
func WithOriginProducerContext(ctx context.Context, owner SelectedOwner, err error) context.Context {
	return context.WithValue(ctx, originProducerContextKey{}, originProducerContext{owner: owner, err: err})
}

func OriginProducerFromContext(ctx context.Context) (SelectedOwner, error) {
	value, found := ctx.Value(originProducerContextKey{}).(originProducerContext)
	if !found {
		return SelectedOwner{}, errors.New("origin producer lacks its authenticated process incarnation")
	}
	if value.err != nil {
		return SelectedOwner{}, value.err
	}
	if err := value.owner.Current(); err != nil {
		return SelectedOwner{}, err
	}
	return value.owner, nil
}
