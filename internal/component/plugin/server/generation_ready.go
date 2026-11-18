// Design: docs/architecture/api/process-protocol.md -- runtime owner replacement.
// Related: register_engine_op.go -- generic exact-generation teardown registration.
package server

import (
	"context"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/callsite"
)

// ProcessGenerationReady runs after the full handshake and initialization
// joined and runtime dispatch is live. It MUST use this exact Process and MUST
// join its bounded work before returning. Initial startup and an immediate
// restart own their joins. A deferred transaction completion owns its join
// after peer admission. No per-event goroutine is created.
type ProcessGenerationReady func(context.Context, *Server, *process.Process) error

var processGenerationReady []ProcessGenerationReady // engineOperationMu

func RegisterProcessGenerationReady(ready ProcessGenerationReady) error {
	return registry.WithRegistration("process generation ready", callsite.Package(2), func() error {
		if ready == nil {
			return errors.New("process generation ready has no handler")
		}
		engineOperationMu.Lock()
		defer engineOperationMu.Unlock()
		processGenerationReady = append(processGenerationReady, ready)
		return nil
	})
}

func (s *Server) runProcessGenerationReady(ctx context.Context, proc *process.Process) error {
	engineOperationMu.RLock()
	hooks := slices.Clone(processGenerationReady)
	engineOperationMu.RUnlock()
	for _, ready := range hooks {
		if err := ready(ctx, s, proc); err != nil {
			return err
		}
	}
	return nil
}
