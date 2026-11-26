// Design: docs/architecture/config/yang-config-design.md — original-tree validation.

package config

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

var treeValidators struct {
	sync.RWMutex
	functions []treeValidator
}

type treeValidator struct {
	name      string
	blockPath string
	validate  func(*Tree) error
}

type treeValidatorFailure struct {
	name      string
	blockPath string
	err       error
}

// RegisterTreeValidator registers a component's original-tree validation boundary.
// Callers MUST register during init. A validator MUST NOT modify the tree and MUST
// be safe for concurrent use; it runs after inactive subtrees are pruned and before
// inheritance or defaults can conceal an invalid explicitly configured layer.
func RegisterTreeValidator(name string, validate func(*Tree) error) error {
	return registerTreeValidator(name, "", validate)
}

// RegisterScopedTreeValidator registers an original-tree validator owned by a
// specific service config block path (e.g. "environment/looking-glass/next-hop-range"
// or "l2tp"). Any leading "/" on blockPath is stripped. Strict callers
// (LoadConfig, ReadConfig, validate, commit, reload) refuse on failure;
// LoadConfigForStart drops blockPath from the in-memory tree instead.
// Callers MUST register during init. Safe for concurrent use.
func RegisterScopedTreeValidator(name, blockPath string, validate func(*Tree) error) error {
	normalized := strings.TrimLeft(blockPath, PathSep)
	if normalized == "" {
		return fmt.Errorf("scoped tree validator %q requires a non-empty block path", name)
	}
	return registerTreeValidator(name, normalized, validate)
}

func registerTreeValidator(name, blockPath string, validate func(*Tree) error) error {
	if name == "" {
		return fmt.Errorf("tree validator registration requires a name")
	}
	if validate == nil {
		return fmt.Errorf("tree validator %q has no validation function", name)
	}
	treeValidators.Lock()
	defer treeValidators.Unlock()
	for _, registered := range treeValidators.functions {
		if registered.name == name {
			return fmt.Errorf("tree validator %q is already registered", name)
		}
	}
	treeValidators.functions = append(treeValidators.functions, treeValidator{
		name:      name,
		blockPath: blockPath,
		validate:  validate,
	})
	return nil
}

// validateOriginalTree runs the registered validators before publishing a tree.
// Validators MUST NOT modify the tree or register another validator here.
func validateOriginalTree(tree *Tree) error {
	treeValidators.RLock()
	defer treeValidators.RUnlock()
	for _, registered := range treeValidators.functions {
		if err := registered.validate(tree); err != nil {
			return err
		}
	}
	return nil
}

// validateOriginalTreeAll runs every registered original-tree validator and
// collects all failures together with each validator's owning block path (empty
// when unscoped). Validators MUST NOT modify the tree or register another
// validator here.
func validateOriginalTreeAll(tree *Tree) []treeValidatorFailure {
	treeValidators.RLock()
	defer treeValidators.RUnlock()
	var failures []treeValidatorFailure
	for _, registered := range treeValidators.functions {
		if err := registered.validate(tree); err != nil {
			failures = append(failures, treeValidatorFailure{
				name:      registered.name,
				blockPath: registered.blockPath,
				err:       err,
			})
		}
	}
	return failures
}

// snapshotTreeValidators saves the current original-tree validator registry and
// returns a function that restores it, so unit tests never leak registrations.
func snapshotTreeValidators() func() {
	treeValidators.Lock()
	saved := slices.Clone(treeValidators.functions)
	treeValidators.Unlock()
	return func() {
		treeValidators.Lock()
		treeValidators.functions = saved
		treeValidators.Unlock()
	}
}
