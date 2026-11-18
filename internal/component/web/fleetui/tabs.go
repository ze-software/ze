// Design: docs/architecture/fleet-interface.md -- the registered tabs of each kind of object
// Related: url.go -- the address parser that accepts only a registered tab
// Related: register_tabs.go -- the tabs the fleet interface registers itself
//
// The tabs of the estate, of a group node and of a device, and the pages of
// the administration, are a registered set. A page family registers its tab
// from the init() of its own register file, and the address parser, the tab
// row and the administration menu read the set. So a later feature adds a
// tab without an edit here.

package fleetui

import (
	"errors"
	"slices"
	"sync"
)

// TabDashboard is the name of the default tab of every object. An address
// that names an object's page and no tab is its dashboard.
const TabDashboard = "dashboard"

// Tab is one tab of an object, or one page of the administration.
type Tab struct {
	// Name is the tab's segment in an address.
	Name string
	// Title is what the tab row shows.
	Title string
}

// The ways a tab registration is refused. Each is a defect of the package
// that registers.
var (
	errTabKind      = errors.New("this kind of object has no tabs")
	errTabUnnamed   = errors.New("the tab has no name or no title")
	errTabDuplicate = errors.New("the tab is already registered for this kind of object")
)

// tabRegistry holds the tabs of each kind of object, in registration order.
// The zero value is an empty registry, ready for use. Safe for concurrent
// use.
type tabRegistry struct {
	mu   sync.RWMutex
	tabs map[ObjectKind][]Tab
}

// register adds one tab to a kind. It refuses a kind that takes no tab, a tab
// with an empty name or title, and a name the kind already holds.
func (registry *tabRegistry) register(kind ObjectKind, tab Tab) error {
	if !kind.tabbed() {
		return errTabKind
	}
	if tab.Name == "" {
		return errTabUnnamed
	}
	if tab.Title == "" {
		return errTabUnnamed
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if slices.ContainsFunc(registry.tabs[kind], func(held Tab) bool { return held.Name == tab.Name }) {
		return errTabDuplicate
	}
	if registry.tabs == nil {
		registry.tabs = make(map[ObjectKind][]Tab)
	}
	registry.tabs[kind] = append(registry.tabs[kind], tab)

	return nil
}

// list answers the caller's own copy of the tabs of a kind, in registration
// order.
func (registry *tabRegistry) list(kind ObjectKind) []Tab {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	return slices.Clone(registry.tabs[kind])
}

// holds reports whether name is a tab of kind.
func (registry *tabRegistry) holds(kind ObjectKind, name string) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	return slices.ContainsFunc(registry.tabs[kind], func(held Tab) bool { return held.Name == name })
}

// objectTabs is the tab registry of this build. Register files fill it from
// init(), and nothing removes from it.
var objectTabs tabRegistry

// MustRegisterTab adds a tab to a kind of object: ObjectEstate, ObjectGroup
// and ObjectDevice take tabs, and ObjectAdmin takes the administration pages.
// Call it from init() in a register file.
//
// It panics on a kind that takes no tab, an unnamed tab and a duplicate. Each
// is a defect of the build's composition, found at process start.
func MustRegisterTab(kind ObjectKind, tab Tab) {
	if err := objectTabs.register(kind, tab); err != nil {
		panic("BUG: fleetui.MustRegisterTab: " + kind.String() + " " + tab.Name + ": " + err.Error())
	}
}

// Tabs answers the tabs of a kind of object in the order they registered,
// which is the order the tab row shows them in.
func Tabs(kind ObjectKind) []Tab {
	return objectTabs.list(kind)
}
