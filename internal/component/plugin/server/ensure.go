// Design: docs/features/interfaces.md -- Auto-ensure parent resources for compound commands
// Related: command.go -- Dispatcher registration and dispatch

package server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// EnsureStep describes one ancestor resource that must exist before a
// descendant command can execute. Built at registration time from the
// YANG command tree's ze:ensure-exists annotations.
type EnsureStep struct {
	Handler         Handler // Creation handler (idempotent: succeeds if resource exists)
	RollbackHandler Handler // Deletion handler for undo on descendant failure
	WireMethod      string  // Creation handler's wire method, for contract errors
}

// ErrEnsureContract reports that a creation handler on an ensure-exists path did
// not answer the one question the rollback machinery must ask it: did YOU create
// this resource? Without that answer the wrapper cannot tell an auto-created
// parent (safe to delete on failure) from one the operator already owned (must
// never be deleted), so it refuses to continue rather than guess.
var ErrEnsureContract = errors.New("ensure-exists contract violation")

// wrapWithEnsureChain returns a new handler that, before calling the
// leaf handler, ensures each ancestor resource in the chain exists.
// If the leaf handler fails after an ancestor was newly created, the
// wrapper rolls back by calling the rollback handler.
func wrapWithEnsureChain(leaf Handler, chain []EnsureStep) Handler {
	return func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		var rollbacks []func()

		for _, step := range chain {
			resp, err := step.Handler(ctx, nil)
			if err != nil {
				runRollbacks(rollbacks)
				return nil, err
			}
			if resp != nil && resp.Status == plugin.StatusError {
				runRollbacks(rollbacks)
				return resp, nil
			}
			created, cErr := wasCreated(resp)
			if cErr != nil {
				// Fail closed: the layer that knows the answer is missing is the
				// only one that can say so (ai/rules/fail-closed-guards.md). Undo
				// the steps that DID report truthfully, then refuse the leaf.
				runRollbacks(rollbacks)
				return nil, fmt.Errorf("%w: creation handler %q %w", ErrEnsureContract, step.WireMethod, cErr)
			}
			if created {
				rb := step.RollbackHandler
				rollbacks = append(rollbacks, func() {
					if _, rbErr := rb(ctx, nil); rbErr != nil {
						logger().Warn("ensure-exists rollback failed", "error", rbErr)
					}
				})
			}
		}

		resp, err := leaf(ctx, args)
		if err != nil || (resp != nil && resp.Status == plugin.StatusError) {
			runRollbacks(rollbacks)
		}
		return resp, err
	}
}

// wasCreated reports whether a response signals that the resource was newly
// created. Creation handlers set Data["created"] = true when they create a new
// resource and false when it already existed.
//
// It is a guard, so it fails closed and says something (ai/rules/fail-closed-guards.md).
// A missing or non-bool "created" key is NOT read as "not created": that zero value
// is indistinguishable from a truthful false, and silently choosing it disarms
// rollback, which is exactly how a failed compound create would strand a
// half-built interface stack. Both wrong answers are unacceptable here -- assuming
// "created" would delete a pre-existing resource the operator owns, assuming "not
// created" leaks an auto-created one -- so an unreportable state is an error and
// the caller aborts the command.
func wasCreated(resp *plugin.Response) (bool, error) {
	if resp == nil || resp.Data == nil {
		return false, errors.New(`returned no data; expected Data["created"] to be a bool`)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		return false, fmt.Errorf(`returned Data of type %T; expected plugin.Map carrying a "created" bool`, resp.Data)
	}
	v, ok := m["created"]
	if !ok {
		return false, errors.New(`returned no "created" key; a creation handler reachable from a ze:ensure-exists path must report created=true when it created the resource and created=false when it already existed`)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf(`returned "created" of type %T; expected bool`, v)
	}
	return b, nil
}

func runRollbacks(fns []func()) {
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// buildEnsureChain walks the command tree along the given CLI path and
// collects EnsureSteps for every ancestor node with ze:ensure-exists.
// wireToHandler maps WireMethod -> Handler for both creation and rollback lookup.
func buildEnsureChain(tree *command.Node, path string, wireToHandler map[string]Handler) []EnsureStep {
	if tree == nil {
		return nil
	}

	parts := strings.Fields(path)
	if len(parts) < 2 {
		return nil
	}

	var chain []EnsureStep
	node := tree

	// Walk all but the last token (the leaf command itself).
	for _, part := range parts[:len(parts)-1] {
		child, ok := node.Children[part]
		if !ok {
			break
		}
		if child.EnsureExists != "" && child.WireMethod != "" {
			createHandler := wireToHandler[child.WireMethod]
			rollbackHandler := wireToHandler[child.EnsureExists]
			if createHandler != nil && rollbackHandler != nil {
				chain = append(chain, EnsureStep{
					Handler:         createHandler,
					RollbackHandler: rollbackHandler,
					WireMethod:      child.WireMethod,
				})
			}
		}
		node = child
	}
	return chain
}
