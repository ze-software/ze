// Design: docs/architecture/api/process-protocol.md -- current selection validation.
// Related: bridge.go -- delivery lifetime fencing for typed callbacks.
package rpc

import "context"

// CurrentSelectionHandler reads one consistent published desired-selection
// snapshot. It MUST honor ctx and must not call the engine or perform I/O.
// Input is borrowed until return. Output ownership passes to the caller; the
// handler MUST NOT subsequently modify its output or any referenced bytes.
type CurrentSelectionHandler func(context.Context, CurrentSelectionInput) (*CurrentSelectionOutput, error)

// SetCurrentSelection installs the independent typed callback before SetReady.
func (b *DirectBridge) SetCurrentSelection(handler CurrentSelectionHandler) {
	b.currentSelection = handler
	b.hasCurrentSelection.Store(handler != nil)
}

// HasCurrentSelection reports handler support, never a peer permission grant.
func (b *DirectBridge) HasCurrentSelection() bool {
	return b.ready.Load() && b.hasCurrentSelection.Load()
}

// CurrentSelection bypasses the ordinary callback queue. The SDK slot enqueues
// on its independent worker; a blocked execute-command cannot prevent lookup.
func (b *DirectBridge) CurrentSelection(ctx context.Context, input CurrentSelectionInput) (*CurrentSelectionOutput, error) {
	return bridgeReadSnapshot(b, ctx, input, CurrentSelectionCallContext,
		&b.hasCurrentSelection, &b.currentSelection,
		"current-selection handler is not installed; register OnCurrentSelection before Run",
		(*CurrentSelectionOutput).Validate)
}
