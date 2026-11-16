// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device: the start step seam
// Related: ../ze_core_start.go -- cmdStart, which runs the registered steps

// Package startprep holds the steps `ze start` runs before it opens its store
// and before anything enumerates the host's interfaces. A feature registers its
// step from init(), and cmdStart runs every registered step, so the start path
// names no feature: removing the feature's package removes its step and
// nothing else changes (docs/architecture/core-design.md, the registration
// pattern).
package startprep

import "slices"

// step is one registered preparation: the name the registry keys it by, and
// the work.
type step struct {
	name string
	run  func()
}

// steps is the registry, in registration order. Register writes it from
// init() and a test's cleanup removes from it; Run reads it after main starts.
// It is not safe for concurrent use: every writer runs before Run or in a
// test that owns the process.
var steps []step

// Register adds a step that `ze start` runs before it reads its store.
//
// A step MUST NOT stop the start, so it has no error to return: it logs its
// own failures, with what failed, why, and the fix, and returns. A step MUST
// finish its work before it returns, because what `ze start` does next is
// enumerate the interfaces the step may have made appear.
//
// Register MUST be called from init(). An empty name, a nil step and a name
// already registered are Ze defects, so each panics. The returned function
// removes the step; only a test's cleanup calls it.
func Register(name string, run func()) (unregister func()) {
	if name == "" {
		panic("BUG: startprep step registered with no name")
	}
	if run == nil {
		panic("BUG: startprep step " + name + " registered with no work")
	}
	if slices.ContainsFunc(steps, func(s step) bool { return s.name == name }) {
		panic("BUG: startprep step " + name + " registered twice")
	}
	steps = append(steps, step{name: name, run: run})
	return func() {
		steps = slices.DeleteFunc(steps, func(s step) bool { return s.name == name })
	}
}

// Run runs every registered step once, in registration order. A step that
// registers or removes a step while Run runs does not change this pass.
func Run() {
	for _, s := range slices.Clone(steps) {
		s.run()
	}
}

// Names answers the registered step names, in the order Run runs them.
func Names() []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.name)
	}
	return names
}
