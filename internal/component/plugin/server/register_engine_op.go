// Design: docs/architecture/api/process-protocol.md -- owner-registered engine operations.
// Related: dispatch_registry.go -- the shared JSON and Direct handlers.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/callsite"
	coreipc "github.com/ze-software/ze/internal/core/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// EngineOperationStartup is the admission phase of an owner operation.
// Zero is an unspecified policy and registration refuses it.
type EngineOperationStartup uint8

const (
	EngineOperationStartupUnspecified EngineOperationStartup = iota
	EngineOperationRuntimeOnly
	EngineOperationConfigureSafe
)

// EngineOperation is a raw declaration. RegisterEngineOp validates it before
// publishing an immutable engine operation. Handler receives the authenticated
// process, never a process name supplied by its JSON caller. ConfigureSafe
// handlers MUST finish within ctx and MUST NOT wait for a later startup stage.
type EngineOperation struct {
	Method    string
	Handler   func(*Server, context.Context, *process.Process, json.RawMessage) (any, error)
	Startup   EngineOperationStartup
	TypedWire func(*Server, *process.Process, *rpc.DirectBridge)
	// Native optionally adapts self-contained Go values to the same evaluator
	// as Handler. Owners MUST validate payload types, nils and values before
	// mutation, and MUST NOT return borrowed engine state. Handler is required
	// even when Native is set, so external plugins retain the public operation.
	Native func(*Server, context.Context, *process.Process, any) (any, error)
}

// engineOperationMu protects owner declarations and generation cleanup hooks.
// Runtime lookup only reads immutable operation values after releasing it.
var engineOperationMu sync.RWMutex

// RegisterEngineOp declares a plugin-to-engine operation from its owner's
// register.go init. The namespace comes from the calling package, as for
// RegisterRPCs. RegisterEngineOp MUST run before registry.Seal. Safe for
// concurrent use; duplicate, invalid, and late declarations change nothing.
func RegisterEngineOp(declaration EngineOperation) error {
	return registerEngineOp(callsite.Package(2), declaration)
}

func registerEngineOp(owner string, declaration EngineOperation) error {
	return registry.WithRegistration("engine operation", declaration.Method, func() error {
		prefix, err := OwnerPrefix(owner)
		if err != nil {
			return err
		}
		methodPrefix, methodName, err := coreipc.ParseMethod(declaration.Method)
		if err != nil {
			return fmt.Errorf("invalid engine operation: %w", err)
		}
		if methodPrefix != prefix {
			return fmt.Errorf("engine operation %q belongs to namespace %q", declaration.Method, prefix)
		}
		if !validEngineMethodName(methodName) {
			return fmt.Errorf("engine operation %q has an invalid method name", declaration.Method)
		}
		if declaration.Handler == nil {
			return fmt.Errorf("engine operation %q has no handler", declaration.Method)
		}
		switch declaration.Startup {
		case EngineOperationRuntimeOnly, EngineOperationConfigureSafe:
		case EngineOperationStartupUnspecified:
			return fmt.Errorf("engine operation %q has no startup admission policy", declaration.Method)
		default:
			// Registration is an open declaration boundary, even when supplied by Go.
			return fmt.Errorf("engine operation %q has unknown startup admission %d", declaration.Method, declaration.Startup)
		}
		engineOperationMu.Lock()
		defer engineOperationMu.Unlock()
		if _, exists := engineOpTable[declaration.Method]; exists {
			return fmt.Errorf("engine operation %q is already registered", declaration.Method)
		}
		engineOpTable[declaration.Method] = &engineOp{
			method:    declaration.Method,
			handle:    declaration.Handler,
			typedWire: declaration.TypedWire,
			native:    declaration.Native,
			startup:   declaration.Startup,
		}
		return nil
	})
}

// validEngineMethodName follows the YANG identifier shape after the namespace.
// Registration is the boundary: malformed names must not reach wire dispatch.
func validEngineMethodName(name string) bool {
	for index, character := range name {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character == '_':
		case index > 0 && character >= '0' && character <= '9':
		case index > 0 && character == '-':
		case index > 0 && character == '.':
		default:
			return false
		}
	}
	return name != ""
}

// allEngineOps includes builtins and owner operations for typed bridge wiring.
func allEngineOps() []*engineOp {
	engineOperationMu.RLock()
	defer engineOperationMu.RUnlock()
	ops := make([]*engineOp, 0, len(engineOpTable))
	for _, op := range engineOpTable {
		ops = append(ops, op)
	}
	slices.SortFunc(ops, func(a, b *engineOp) int { return strings.Compare(a.method, b.method) })
	return ops
}

// ProcessGenerationCleanup receives the exact process whose dispatch drained.
// A callback MUST scope state by that identity, not by a reused process name.
type ProcessGenerationCleanup func(*process.Process)

var processGenerationCleanups []ProcessGenerationCleanup

// RegisterProcessGenerationCleanup adds an owner's generation cleanup from init.
// The callback MUST NOT retain caller-owned mutable declarations. Its runtime
// invocation happens after admitted requests drain and before replacement.
// Safe for concurrent use; registration is refused after registry.Seal.
func RegisterProcessGenerationCleanup(cleanup ProcessGenerationCleanup) error {
	return registry.WithRegistration("process generation cleanup", callsite.Package(2), func() error {
		if cleanup == nil {
			return fmt.Errorf("process generation cleanup has no handler")
		}
		engineOperationMu.Lock()
		defer engineOperationMu.Unlock()
		processGenerationCleanups = append(processGenerationCleanups, cleanup)
		return nil
	})
}

func runProcessGenerationCleanups(proc *process.Process) {
	engineOperationMu.RLock()
	hooks := slices.Clone(processGenerationCleanups)
	engineOperationMu.RUnlock()
	for _, cleanup := range hooks {
		cleanup(proc)
	}
}
