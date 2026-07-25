// Design: docs/features/interfaces.md -- Auto-ensure parent resources for compound commands
// Related: command.go -- Dispatcher registration and dispatch

package server

import (
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
}

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
			if wasCreated(resp) {
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

// wasCreated checks if a response signals that a resource was newly created.
// Creation handlers set Data["created"] = true when they create a new resource,
// and false when the resource already existed.
func wasCreated(resp *plugin.Response) bool {
	if resp == nil || resp.Data == nil {
		return false
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		return false
	}
	v, ok := m["created"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
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
				})
			}
		}
		node = child
	}
	return chain
}
