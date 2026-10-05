// Design: docs/architecture/core-design.md -- work an area runs before le dispatches
// Overview: leroot.go -- the registration adapter every le tool joins through
// Related: dispatch.go -- the dispatch the hooks run ahead of
//
// An area that needs a turn on every le invocation, the scratch package's
// hourly store trim for one, registers a hook here from its own register.go
// and the le root handler runs every hook before it dispatches. The root
// handler therefore names no tool package: the area stays a blank import of
// the composition root like every other.
package leroot

import (
	"slices"
	"sync"
)

// BeforeDispatch is work an area runs before le dispatches any command, native
// hooks included. program is the invocation name the dispatch prints: `le` for
// the le binary, `ze le` for a ze binary carrying le.
//
// A hook MUST NOT print and MUST NOT wait for slow work: it runs ahead of every
// le call, most of which are hook calls that answer in milliseconds, and the
// command's output is the dispatch's alone. It returns nothing, so it cannot
// change the command's exit code; a hook that fails records why itself.
type BeforeDispatch func(program string)

// RegisterBeforeDispatch records the hook one area runs before every dispatch.
// It MUST be called from the area's register.go init(). A nil hook or a second
// hook for one area is a programming error at init, so it panics.
func RegisterBeforeDispatch(area string, hook BeforeDispatch) {
	if hook == nil {
		panic("BUG: leroot.RegisterBeforeDispatch: nil hook; see the init frame above for the area")
	}
	beforeDispatchHooks.Lock()
	defer beforeDispatchHooks.Unlock()
	if beforeDispatchHooks.byArea == nil {
		beforeDispatchHooks.byArea = make(map[string]BeforeDispatch, 4)
	}
	if _, taken := beforeDispatchHooks.byArea[area]; taken {
		panic("BUG: leroot.RegisterBeforeDispatch: a second hook for one area")
	}
	beforeDispatchHooks.byArea[area] = hook
}

// RunBeforeDispatch runs every registered hook, in area-name order, so two
// runs of one build call them in the same order. The le root handler MUST
// call it before Dispatch; Dispatch does not call it, so a test that drives
// Dispatch is the command without the hooks.
func RunBeforeDispatch(program string) {
	beforeDispatchHooks.RLock()
	areas := make([]string, 0, len(beforeDispatchHooks.byArea))
	for area := range beforeDispatchHooks.byArea {
		areas = append(areas, area)
	}
	hooks := make([]BeforeDispatch, 0, len(areas))
	slices.Sort(areas)
	for _, area := range areas {
		hooks = append(hooks, beforeDispatchHooks.byArea[area])
	}
	beforeDispatchHooks.RUnlock()

	for _, hook := range hooks {
		hook(program)
	}
}

// beforeDispatchHooks holds every area's hook. Registration runs in init() and
// the root handler reads it on any goroutine, so the map is guarded. Safe for
// concurrent use.
var beforeDispatchHooks struct {
	sync.RWMutex
	byArea map[string]BeforeDispatch
}
