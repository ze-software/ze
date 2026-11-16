// Design: docs/architecture/api/process-protocol.md -- captured selecting connection.
// Related: selected.go -- immutable authenticated process lifetime.
package plugin

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedCurrentReader is immutable after binding. A private pointer keeps
// SelectedOwner comparable while its exact connection reader remains captured.
type selectedCurrentReader struct { read rpc.CurrentSelectionHandler }

// WithCurrentSelection returns one bound owner before its Process publishes it.
// It MUST be called once before the owner enters a lease or selected writer;
// old copies are not rebound. A lifetime-only owner remains valid for cleanup
// but cannot answer a current-selection query. Safe for concurrent readers.
func (owner SelectedOwner) WithCurrentSelection(read rpc.CurrentSelectionHandler) (SelectedOwner, error) {
	if err := owner.Current(); err != nil { return SelectedOwner{}, err }
	if read == nil { return SelectedOwner{}, errors.New("current-selection reader is missing; capture the selecting connection before publishing its owner") }
	if owner.currentSelection != nil { return SelectedOwner{}, errors.New("current-selection reader is already bound; use the captured owner without rebinding") }
	owner.currentSelection = &selectedCurrentReader{read: read}
	return owner, nil
}

// CurrentSelection reads the captured connection without looking up a process
// name or taking its manager lock. The final writer may call it under its own
// socket lock. Lifetime and complete reply identity are checked on both sides.
func (owner SelectedOwner) CurrentSelection(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
	ctx, cancel := rpc.CurrentSelectionCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil { return nil, err }
	if err := owner.Current(); err != nil { return nil, err }
	if owner.currentSelection == nil { return nil, errors.New("current-selection reader is unbound; capture the selecting connection before writing routes") }
	output, err := owner.currentSelection.read(ctx, input)
	if err != nil { return nil, err }
	if err := owner.Current(); err != nil { return nil, err }
	if err := ctx.Err(); err != nil { return nil, err }
	if err := output.Validate(input); err != nil { return nil, err }
	return output, nil
}
