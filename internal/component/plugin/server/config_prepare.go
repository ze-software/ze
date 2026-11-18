// Design: docs/architecture/config/transaction-protocol.md -- native preparation shares reload acceptance.
// Related: reload_tx.go -- the original outer scope owns the final decision.

package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/callsite"
)

// ConfigPreparation owns resources prepared for one candidate. Complete MUST
// join its bounded work before returning and MUST NOT reenter config reload.
// The server calls it exactly once, with true only after outer publication;
// true MUST select the prepared value even if another owner's cleanup failed.
// Implementations MUST be safe for concurrent completion of different values.
type ConfigPreparation interface {
	Complete(context.Context, bool) error
}

// ConfigPrepare validates and prepares a native owner's candidate under config
// transaction exclusion. The active candidate is read-only: the callback MUST
// NOT mutate or retain its maps, or reenter config reload. It MUST honor ctx;
// a returned preparation receives false on error, while unreturned partial
// resources remain the callback's responsibility. (nil, nil) means unaffected.
// generation increases per server attempt; owners MUST compare accepted, rather
// than merely prepared, generations when completing out-of-order scopes.
type ConfigPrepare func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error)

type configPreparationRegistration struct {
	name    string
	owner   string
	prepare ConfigPrepare
}

type configPrepared struct {
	name  string
	value ConfigPreparation
}

var (
	configPreparationMu sync.RWMutex
	// Bounded by compiled-in owners registering during package initialization.
	configPreparations []configPreparationRegistration
)

// RegisterConfigPreparation declares native preparation from its owner's
// register.go init. Safe for concurrent use; invalid, duplicate and sealed
// declarations preserve the registry. Names follow the YANG identifier shape.
func RegisterConfigPreparation(name string, prepare ConfigPrepare) error {
	owner := callsite.Package(2)
	return registry.WithRegistration("config preparation", name, func() error {
		if !validEngineMethodName(name) {
			return fmt.Errorf("config preparation %q has an invalid name", name)
		}
		if prepare == nil {
			return fmt.Errorf("config preparation %q has no handler", name)
		}
		configPreparationMu.Lock()
		defer configPreparationMu.Unlock()
		for _, registered := range configPreparations {
			if registered.name == name {
				return fmt.Errorf("config preparation %q is already registered by %s", name, registered.owner)
			}
		}
		configPreparations = append(configPreparations, configPreparationRegistration{name, owner, prepare})
		return nil
	})
}

// prepareConfig runs only while the reload owns transaction exclusion. Its
// caller MUST abort returned values if any later step of this attempt fails,
// without discarding earlier valid attempts in the same outer scope.
func (s *Server) prepareConfig(ctx context.Context, tree map[string]any) ([]configPrepared, error) {
	configPreparationMu.RLock()
	registrations := slices.Clone(configPreparations)
	configPreparationMu.RUnlock()
	if len(registrations) == 0 {
		return nil, nil //nolint:nilnil // No registered native owners need preparation.
	}
	s.txLock.configGeneration++
	active := config.ActiveConfigMap(tree)
	prepared := make([]configPrepared, 0, len(registrations))
	for _, registered := range registrations {
		if err := context.Cause(ctx); err != nil {
			return prepared, err
		}
		value, err := registered.prepare(s, ctx, active, s.txLock.configGeneration)
		if err == nil {
			err = context.Cause(ctx)
		}
		if value != nil {
			prepared = append(prepared, configPrepared{registered.name, value})
		}
		if err != nil {
			return prepared, err
		}
	}
	return prepared, nil
}

func completeConfigPreparations(ctx context.Context, prepared []configPrepared, accepted bool) error {
	var result error
	for _, current := range prepared {
		if err := current.value.Complete(ctx, accepted); err != nil {
			result = errors.Join(result, fmt.Errorf("config preparation %s: %w", current.name, err))
		}
	}
	return result
}

func (a *reloadAcceptance) completeConfigPreparations(ctx context.Context, accepted bool) error {
	a.mu.Lock()
	prepared := a.prepared
	a.prepared = nil
	a.mu.Unlock()
	return completeConfigPreparations(ctx, prepared, accepted)
}
