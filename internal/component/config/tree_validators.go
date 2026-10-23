// Design: docs/architecture/config/yang-config-design.md — original-tree validation.

package config

import (
	"fmt"
	"sync"
)

var treeValidators struct {
	sync.RWMutex
	functions []treeValidator
}

type treeValidator struct {
	name     string
	validate func(*Tree) error
}

// RegisterTreeValidator registers a component's original-tree validation boundary.
// Callers MUST register during init. A validator MUST NOT modify the tree and MUST
// be safe for concurrent use; it runs after inactive subtrees are pruned and before
// inheritance or defaults can conceal an invalid explicitly configured layer.
func RegisterTreeValidator(name string, validate func(*Tree) error) error {
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
	treeValidators.functions = append(treeValidators.functions, treeValidator{name: name, validate: validate})
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
