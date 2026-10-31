// Design: docs/architecture/config/syntax.md — config migration
// Related: migrate.go — the built-in transformations and the loops that run them

package migration

import (
	"slices"
	"sync"
)

var (
	registeredMu              sync.RWMutex
	registeredTransformations []Transformation
)

// MustRegisterTransformation adds a transformation that a feature package owns,
// so the package that changed a config shape also carries the rewrite of the old
// shape, and migrate.go never names it. Registered transformations run after the
// built-in ones, in registration order: they rewrite blocks under the roots the
// built-in steps have already moved into place. The package calls it from init().
// A second registration under one name is a Ze defect and panics.
func MustRegisterTransformation(t Transformation) {
	registeredMu.Lock()
	defer registeredMu.Unlock()
	for _, existing := range registeredTransformations {
		if existing.Name == t.Name {
			panic("BUG: migration: transformation " + t.Name + " registered twice")
		}
	}
	for _, builtin := range transformations {
		if builtin.Name == t.Name {
			panic("BUG: migration: transformation " + t.Name + " shadows a built-in one")
		}
	}
	registeredTransformations = append(registeredTransformations, t)
}

// allTransformations answers the built-in transformations then the registered
// ones, as a fresh slice the caller may keep.
func allTransformations() []Transformation {
	registeredMu.RLock()
	defer registeredMu.RUnlock()
	return slices.Concat(transformations, registeredTransformations)
}
