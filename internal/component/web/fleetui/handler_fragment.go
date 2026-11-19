// Design: docs/architecture/fleet-interface.md -- the fragment handler and the fragments of the shell's own regions
// Related: pages.go -- the fragment registry
// Related: handler_shell.go -- the page handler, whose regions these fragments refresh
// Related: handler_tree.go, handler_search.go -- the tree and the search fragments
//
// A fragment is the inside of one region of a page, fetched on its own:
// /fleet/fragment/<name>/<page path>, with the page's view parameters. The
// shell registers one per region it owns, and a page family registers its
// own the same way. A fragment that fails answers the state that matches the
// failure, under a success status, so the region shows the state.

package fleetui

import (
	"context"
	"net/http"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The names of the fragments the shell registers for its own regions.
const (
	// fragmentTree is the tree, narrowed by the filter text its item holds.
	fragmentTree = "tree"
	// fragmentTreeBranch is the rows under the tree row its item names.
	fragmentTreeBranch = "tree-branch"
	// fragmentObject is the object header, the breadcrumb and the tab row.
	fragmentObject = "object"
	// fragmentContent is the tab's content.
	fragmentContent = "content"
	// fragmentPanel is the side panel's content.
	fragmentPanel = "panel"
	// fragmentTasks is the task panel.
	fragmentTasks = "tasks"
	// fragmentHeader is the Tasks button of the header, with its counts.
	fragmentHeader = "header"
	// fragmentSearch is what the header search found for its item.
	fragmentSearch = "search"
)

// fragmentItemMax is the longest item a fragment address may carry. It is the
// length a filter may have, which is the longest text an item holds.
const fragmentItemMax = FilterLengthMax

// registerShellFragments registers the fragments of the regions the shell
// owns. register.go calls it from init().
func registerShellFragments() {
	MustRegisterFragment(fragmentTree, treeFragment)
	MustRegisterFragment(fragmentTreeBranch, treeBranchFragment)
	MustRegisterFragment(fragmentObject, objectFragment)
	MustRegisterFragment(fragmentContent, contentFragment)
	MustRegisterFragment(fragmentPanel, panelFragment)
	MustRegisterFragment(fragmentTasks, tasksFragment)
	MustRegisterFragment(fragmentHeader, headerFragment)
	MustRegisterFragment(fragmentSearch, searchFragment)
}

// fragmentUnknown is the refusal for a fragment name nothing registered.
func fragmentUnknown(name string) error {
	problem := textbuf.New().Str("The fleet interface has no fragment named ").Quoted(name).Str(".").Slice()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"The address asks for a part of a page that does not exist, so there is nothing to answer.",
		"open "+fleetPath)
}

// itemTooLong is the refusal for a fragment address whose item is longer
// than fragmentItemMax.
func itemTooLong() error {
	return opererr.New("The text sent with this request is too long.",
		"A filter or a search text longer than "+textbuf.StringInt(fragmentItemMax)+" characters is not read, so nothing was searched.",
		"shorten the text to "+textbuf.StringInt(fragmentItemMax)+" characters or fewer")
}

// serveFragment answers one fragment address.
func (handler *fleetHandler) serveFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fragment, err := ParseFragment(r.URL)
	if err != nil {
		answerRefusal(w, err, http.StatusNotFound)
		return
	}

	build, held := registeredFragments.lookup(fragment.Name)
	if !held {
		answerRefusal(w, fragmentUnknown(fragment.Name), http.StatusNotFound)
		return
	}

	if len(fragment.Item) > fragmentItemMax {
		answerRefusal(w, itemTooLong(), http.StatusBadRequest)
		return
	}

	request, err := NewPageRequest(ctx, handler.source, fragment.Page, web.GetUsernameFromRequest(r))
	if err != nil {
		answerRefusal(w, err, http.StatusServiceUnavailable)
		return
	}
	request.pages = handler.pages

	component, err := build(ctx, request, fragment.Item)
	if err != nil {
		component = failureHero(request, err)
	}
	if component == nil {
		component = failureHero(request, builtNothing())
	}

	renderAnswer(ctx, w, http.StatusOK, component)
}

// objectFragment answers the object header, the breadcrumb and the tab row
// of the page.
func objectFragment(_ context.Context, request *PageRequest, _ string) (templ.Component, error) {
	return objectRegion(describePage(request).main), nil
}

// contentFragment answers the content of the page's tab.
func contentFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	return resolvePage(ctx, request).content, nil
}

// panelFragment answers the side panel's content: the details of the row the
// address names, or the sentence a panel with no row shows.
func panelFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	address := request.Address

	page, built := request.pages.lookup(address.Kind, address.Tab)
	if !built {
		return notBuiltHero(request), nil
	}
	if page.SidePanel == nil {
		return panelEmpty(), nil
	}
	if address.View.Row == "" {
		return panelEmpty(), nil
	}

	side, _ := buildRegion(ctx, request, page.SidePanel)

	return side, nil
}

// tasksFragment answers the task panel: as a page first draws it for an
// empty item, and shown on the tab the item names otherwise. The answer is
// the whole panel, which replaces the one on the page.
func tasksFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	view, err := taskPanelItem(request, item)
	if err != nil {
		return nil, err
	}

	return TaskPanel(view), nil
}

// headerFragment answers the Tasks button of the header.
func headerFragment(_ context.Context, request *PageRequest, _ string) (templ.Component, error) {
	return headerTasks(headerTasksView(request)), nil
}
