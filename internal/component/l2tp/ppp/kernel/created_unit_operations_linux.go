// Design: docs/research/l2tpv2-ze-integration.md -- joined original-unit namespace

//go:build linux

package kernel

import (
	"context"
	"errors"
	"os"
)

// WithOriginalNamespace MUST run original unit socket binding, not a lookup or
// reconstructed owner. It joins the existing namespace worker before returning.
// Close may proceed while operation waits: neither the unit mutex nor an owner
// file borrow is held over that wait. The producer MUST retain partial results
// even when original namespace restoration contributes an error.
func (unit *CreatedUnit) WithOriginalNamespace(ctx context.Context, operation func() error) error {
	if unit == nil {
		return errors.New("ppp: namespace operation has no original unit")
	}
	if ctx == nil {
		return errors.New("ppp: namespace operation has no context")
	}
	if operation == nil {
		return errors.New("ppp: namespace operation is absent")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	unit.mu.Lock()
	if unit.self != unit {
		unit.mu.Unlock()
		return errors.New("ppp: namespace operation refuses a copied unit")
	}
	if unit.closed {
		unit.mu.Unlock()
		return os.ErrClosed
	}
	if !unit.ready {
		unit.mu.Unlock()
		return errors.New("ppp: namespace operation requires confirmed creation")
	}
	namespace := unit.namespace
	start := unit.nativeNamespaceStart
	unit.mu.Unlock()
	result := runOriginalPPPNamespaceWithStart(namespace, start, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		unit.mu.Lock()
		closed := unit.closed
		unchanged := unit.namespace == namespace
		unit.mu.Unlock()
		if closed {
			return os.ErrClosed
		}
		if !unchanged {
			return errors.New("ppp: original unit namespace changed")
		}
		return operation()
	})
	return result.err()
}
