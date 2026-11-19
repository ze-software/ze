// Design: docs/architecture/fleet-interface.md -- the page and fragment registries, and what a builder is handed
// Related: handler_shell.go -- the handlers that resolve an address to a registered page
// Related: handler_fragment.go -- the handler that answers a registered fragment
// Related: tabs.go -- the tabs a page is registered under
// Related: register.go -- the fragments the shell registers for its own regions
//
// The shell knows no page. A page family registers each of its pages here,
// from the init() of its own register file, under the kind of object and the
// tab the page shows. The shell resolves an address to the registered page,
// and a tab nobody registered a page for renders the "not built yet" state,
// so the shell is complete on its own.

package fleetui

import (
	"context"
	"sync"
	"time"

	"github.com/a-h/templ"
)

// Snapshot is the hub-held state one request reads: the estate, the hub's
// clock, and the devices joined with the open alarms and the tasks. It is
// read once per request, so the tree, the header and the page of one answer
// agree with each other.
type Snapshot struct {
	Estate Estate
	// Now is the hub's clock. Every age on the page is measured against it.
	Now time.Time
	// Views holds one view per device of the estate, in the source's order.
	Views  []DeviceView
	Alarms []Alarm
	Tasks  []Task
	// TreeViews holds the tree views the hub declares. The built-in views
	// are not among them.
	TreeViews []TreeView
}

// ReadSnapshot asks source for the hub-held state of one request. A question
// the source cannot answer fails the whole snapshot with the source's own
// operator error: a shell drawn over half an estate would show wrong counts.
func ReadSnapshot(ctx context.Context, source Source) (*Snapshot, error) {
	now, err := source.Now(ctx)
	if err != nil {
		return nil, err
	}

	estate, err := source.Estate(ctx)
	if err != nil {
		return nil, err
	}

	devices, err := source.Devices(ctx)
	if err != nil {
		return nil, err
	}

	alarms, err := source.Alarms(ctx)
	if err != nil {
		return nil, err
	}

	tasks, err := source.Tasks(ctx)
	if err != nil {
		return nil, err
	}

	declared, err := source.TreeViews(ctx)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Estate:    estate,
		Now:       now,
		Views:     DeriveDevices(devices, alarms, tasks, now),
		Alarms:    alarms,
		Tasks:     tasks,
		TreeViews: declared,
	}, nil
}

// PageRequest is what a page builder and a fragment builder are handed: the
// address asked for with its view parameters, the data source, the account
// the request runs as, and the hub-held state already read for the request.
// NewPageRequest builds one. Builders read it and MUST NOT change it.
type PageRequest struct {
	// Address is the page asked for. Address.View holds the view
	// parameters: filter, sort, page, size, row, chart range, tree view.
	Address Address
	// Source answers what the snapshot does not hold: history, device
	// views, configuration, one task.
	Source Source
	// Username is the account the request runs as.
	Username string
	// Snapshot is the hub-held state, read once for this request. Its Now
	// is the clock of the request.
	Snapshot *Snapshot

	// pages is the registry the request's page is looked up in: the one of
	// this build, unless a test supplies its own.
	pages *pageRegistry
	// treeView is the tree view the sidebar draws: the one the address
	// names, or the default one while treeViewRefusal is set.
	treeView TreeView
	// treeViewRefusal is set when the address names a tree view the hub
	// does not have. The page then shows the refusal.
	treeViewRefusal error
}

// NewPageRequest reads the snapshot and resolves the tree view of address. It
// fails only when the source cannot answer the snapshot. A tree view the hub
// does not have is not a failure here: the request then draws the default
// view and the page shows the refusal.
func NewPageRequest(ctx context.Context, source Source, address Address, username string) (*PageRequest, error) {
	snapshot, err := ReadSnapshot(ctx, source)
	if err != nil {
		return nil, err
	}

	request := &PageRequest{
		Address: address, Source: source, Username: username, Snapshot: snapshot, pages: &registeredPages,
	}

	view, err := ResolveTreeView(address.View.TreeView, snapshot.TreeViews)
	if err == nil {
		request.treeView = view
		return request, nil
	}

	fallback, fallbackErr := ResolveTreeView(TreeViewDefault, snapshot.TreeViews)
	if fallbackErr != nil {
		panic("BUG: fleetui: the default tree view is not among the built-in views")
	}
	request.treeView = fallback
	request.treeViewRefusal = err

	return request, nil
}

// PageBuilder builds one region of a page from a request. It answers the
// component to render, or the source's operator error: the shell then shows
// the state that matches the error (FailureOf). It MUST NOT answer a nil
// component with a nil error.
type PageBuilder func(ctx context.Context, request *PageRequest) (templ.Component, error)

// Page is one registered page: what the content region shows for one tab of
// one kind of object.
type Page struct {
	// Kind is the kind of object the page belongs to.
	Kind ObjectKind
	// Tab is the tab, or the administration page, as its address segment
	// names it. It is empty for ObjectTasks, which has no tab.
	Tab string
	// Content builds the tab's content. It is required.
	Content PageBuilder
	// SidePanel builds the details of the row the address names in its id
	// parameter. A nil SidePanel means the page has no side panel. It is
	// called only while the address names a row.
	SidePanel PageBuilder
	// Primary answers the primary action of the object header, the text
	// link with the leading plus sign, or false for none. Nil means none.
	Primary func(request *PageRequest) (ActionView, bool)
	// Actions answers the entries of the Actions menu of the object header
	// and of the object's tree row. Nil means none.
	Actions func(request *PageRequest) []ActionView
	// Static marks a page whose content holds state the browser owns, such
	// as a terminal. A live event then leaves its content region alone.
	Static bool
}

// pageKey names one page in the registry.
type pageKey struct {
	kind ObjectKind
	tab  string
}

// pageRegistry holds the registered pages. The zero value is an empty
// registry, ready for use. Safe for concurrent use.
type pageRegistry struct {
	mu    sync.RWMutex
	pages map[pageKey]Page
}

// registeredPages is the page registry of this build. Register files fill it
// from init(), and nothing removes from it.
var registeredPages pageRegistry

// The subjects a refused registration of this file names.
const (
	bugPage         = "the page"
	bugFragment     = "the fragment"
	pageSubject     = "the fleet pages"
	fragmentSubject = "the fleet fragments"
	bugNoBuilder    = "has no builder"
)

// MustRegisterPage adds a page. Call it from init() in the register file of
// the page family that owns the page. It panics on a page with no kind, a
// page with no content builder and a second page for one kind and tab: each
// is a defect of the build's composition, found at process start.
//
// The tab is not checked against the registered tabs, because the init() of
// another register file may register the tab later. A page under a tab that
// is never registered is simply never reached: the address parser refuses its
// address.
func MustRegisterPage(page Page) {
	name := page.Kind.String() + "/" + page.Tab

	if page.Kind == ObjectUnspecified {
		registrationBug(bugPage, name, pageSubject, "names no kind of object")
	}
	if page.Content == nil {
		registrationBug(bugPage, name, pageSubject, bugNoBuilder)
	}

	key := pageKey{kind: page.Kind, tab: page.Tab}

	registeredPages.mu.Lock()
	defer registeredPages.mu.Unlock()

	if _, held := registeredPages.pages[key]; held {
		registrationBug(bugPage, name, pageSubject, bugTwice)
	}
	if registeredPages.pages == nil {
		registeredPages.pages = make(map[pageKey]Page)
	}
	registeredPages.pages[key] = page
}

// lookup answers the page registered for a kind and a tab, or false when no
// page family registered one.
func (registry *pageRegistry) lookup(kind ObjectKind, tab string) (Page, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	page, held := registry.pages[pageKey{kind: kind, tab: tab}]

	return page, held
}

// FragmentBuilder builds one fragment: a part of a page htmx fetches on its
// own. item is the fragment address's item parameter, which names which one
// for a fragment a page holds several of, and is empty for the others. The
// answer is the markup that goes inside the region the fragment fills. It
// MUST NOT answer a nil component with a nil error.
type FragmentBuilder func(ctx context.Context, request *PageRequest, item string) (templ.Component, error)

// fragmentRegistry holds the registered fragments by name. The zero value is
// an empty registry, ready for use. Safe for concurrent use.
type fragmentRegistry struct {
	mu        sync.RWMutex
	fragments map[string]FragmentBuilder
}

// registeredFragments is the fragment registry of this build.
var registeredFragments fragmentRegistry

// MustRegisterFragment adds a fragment under name, the second segment of its
// address: /fleet/fragment/<name>/<page path>. Call it from init() in a
// register file. It panics on an empty name, a nil builder and a name already
// registered: each is a defect of the build's composition.
func MustRegisterFragment(name string, build FragmentBuilder) {
	if name == "" {
		registrationBug(bugFragment, name, fragmentSubject, bugUnnamed)
	}
	if build == nil {
		registrationBug(bugFragment, name, fragmentSubject, bugNoBuilder)
	}

	registeredFragments.mu.Lock()
	defer registeredFragments.mu.Unlock()

	if _, held := registeredFragments.fragments[name]; held {
		registrationBug(bugFragment, name, fragmentSubject, bugTwice)
	}
	if registeredFragments.fragments == nil {
		registeredFragments.fragments = make(map[string]FragmentBuilder)
	}
	registeredFragments.fragments[name] = build
}

// lookup answers the builder registered under name, or false.
func (registry *fragmentRegistry) lookup(name string) (FragmentBuilder, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	build, held := registry.fragments[name]

	return build, held
}

// FragmentLink answers the address of the fragment name of the page at
// address, with the page's view parameters, for the data attribute a region
// carries. item is empty for a fragment a page holds one of.
func FragmentLink(name string, address Address, item string) string {
	return FragmentAddress{Name: name, Page: address, Item: item}.String()
}
