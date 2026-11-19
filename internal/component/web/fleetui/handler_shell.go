// Design: docs/architecture/fleet-interface.md -- the fleet page handler: an address resolved to a page inside the shell
// Related: shell.templ -- the markup this handler renders
// Related: pages.go -- the page registry and PageRequest
// Related: shell_view.go -- the views this handler fills
// Related: shell_states.go -- the states a page shows in place of content
// Related: handler_fragment.go -- the handler of the fragments of a page
// Related: register.go -- the routes that reach these handlers

package fleetui

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"net/http"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The addresses the fleet interface is served at. Every fleet route sits under
// fleetPath, which no other web route uses.
const (
	fleetPath       = "/fleet/"
	fleetAssetsPath = "/fleet/assets/"
	stepPath        = fleetPath + "prototype/step"
	holdPath        = fleetPath + "prototype/hold"
)

// The request headers htmx sends, and the values the page handler reads. A
// request htmx makes for a part of the page is answered with the regions that
// change, and every other request with the whole shell.
const (
	headerHTMXRequest = "HX-Request"
	headerHTMXType    = "HX-Request-Type"
	headerHTMXRestore = "HX-History-Restore-Request"
	htmxTrue          = "true"
	htmxPartial       = "partial"
)

// assetsFS holds the fleet styles and scripts. The package embeds its own, so
// a build without the fleet package carries none of them.
//
//go:embed assets
var assetsFS embed.FS

// pageOutcome is what one address resolved to: the status of the answer, the
// main pane around the content, and the two components the page supplied.
type pageOutcome struct {
	status  int
	title   string
	main    MainView
	panel   SidePanelView
	content templ.Component
	// side is the side panel's content. It is set only while panel.Open.
	side templ.Component
	// selects is false for an address that names no page or no object, so
	// no row of the tree is marked.
	selects bool
}

// fleetHandler answers the pages and the fragments of the fleet interface
// from one source. It holds no state of its own, so it is safe for
// concurrent use.
type fleetHandler struct {
	source Source
	// pages is the registry an address is resolved in.
	pages *pageRegistry
}

// newFleetHandler builds the handler over source and the page registry of
// this build. It takes the source itself, so it cannot be built before one
// is registered.
func newFleetHandler(source Source) *fleetHandler {
	return &fleetHandler{source: source, pages: &registeredPages}
}

// answerRefusal writes an operator error as the whole answer, for a request
// the shell cannot be drawn for. An error that is not an operator error yet
// is wrapped, so the answer always ends with a fix.
func answerRefusal(w http.ResponseWriter, err error, status int) {
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		refusal = opererr.Wrap(err, "The fleet interface could not answer the request.",
			"The request failed before a page could be built, so there is nothing to show.",
			"ze show doctor")
	}

	http.Error(w, refusal.Error(), status)
}

// renderAnswer renders component to memory first, so a failed render answers
// an error instead of half a page under a success status.
func renderAnswer(ctx context.Context, w http.ResponseWriter, status int, component templ.Component) {
	var page bytes.Buffer
	if err := component.Render(ctx, &page); err != nil {
		refusal := opererr.Wrap(err, "The fleet page could not be rendered.",
			"The estate was read, but its page failed to build, so nothing can be shown.",
			"ze show doctor")
		http.Error(w, refusal.Error(), http.StatusInternalServerError)

		return
	}

	contentType := "text/html; charset=utf-8"
	if _, data := component.(jsonData); data {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", headerHTMXRequest)
	w.WriteHeader(status)
	w.Write(page.Bytes()) //nolint:errcheck // the answer is committed; a failed write means the browser left
}

// asksPart reports whether htmx asked for the page to swap it into the
// content region, which a navigation link does. A plain request, and the
// request htmx makes to restore a page from history, want the whole shell.
func asksPart(r *http.Request) bool {
	if r.Header.Get(headerHTMXRequest) != htmxTrue {
		return false
	}
	if r.Header.Get(headerHTMXRestore) == htmxTrue {
		return false
	}

	return r.Header.Get(headerHTMXType) == htmxPartial
}

// ServeHTTP answers one page address. An address that names no page, no
// group or no device answers the not-found state inside the shell, with
// status 404.
func (handler *fleetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	address, addressErr := ParseAddress(r.URL)
	if addressErr != nil {
		// The shell is still drawn, around the estate, so the operator
		// keeps the tree and the header to leave the dead address by.
		address = Address{Kind: ObjectEstate, Tab: TabDashboard, View: ParseViewParams(r.URL.Query())}
	}

	request, err := NewPageRequest(ctx, handler.source, address, web.GetUsernameFromRequest(r))
	if err != nil {
		answerRefusal(w, err, http.StatusServiceUnavailable)
		return
	}
	request.pages = handler.pages

	var outcome *pageOutcome
	if addressErr == nil {
		outcome = resolvePage(ctx, request)
	} else {
		outcome = refusedOutcome(request, addressErr, http.StatusNotFound)
	}

	view, err := shellView(ctx, request, outcome)
	if err != nil {
		answerRefusal(w, err, http.StatusInternalServerError)
		return
	}

	if asksPart(r) {
		renderAnswer(ctx, w, outcome.status, pageUpdate(view, outcome.content, outcome.side))
		return
	}

	renderAnswer(ctx, w, outcome.status, Shell(view, outcome.content, outcome.side))
}

// shellView fills the whole shell around one outcome.
func shellView(ctx context.Context, request *PageRequest, outcome *pageOutcome) (ShellView, error) {
	sidebar, err := sidebarView(request, outcome.selects)
	if err != nil {
		return ShellView{}, err
	}

	main := outcome.main
	main.Title = outcome.title + titleSeparator + productName
	if outcome.selects {
		main.EventsURL = EventsPath
	}

	tasks, err := taskPanelView(ctx, request)
	if err != nil {
		return ShellView{}, err
	}

	return ShellView{
		Header:  headerView(request),
		Sidebar: sidebar,
		Main:    main,
		Panel:   outcome.panel,
		Tasks:   tasks,
	}, nil
}

// refusedOutcome is the outcome of an address the shell cannot show an
// object for: the state that matches err in place of the content, under a
// header that names no object. err MUST be non-nil.
func refusedOutcome(request *PageRequest, err error, status int) *pageOutcome {
	return &pageOutcome{
		status: status,
		title:  notFoundTitle,
		main: MainView{
			Object: ObjectHeaderView{Icon: ObjectIconView{Icon: iconAlert}, Name: notFoundTitle},
			Crumbs: []CrumbView{estateCrumb(request)},
		},
		content: failureHero(request, err),
	}
}

// describeObject fills the object header and the breadcrumb of the object the
// request names. The estate has no breadcrumb. It refuses a group no device is in and a device the estate
// does not hold, with the data layer's own error.
func describeObject(request *PageRequest) (ObjectHeaderView, []CrumbView, error) {
	address := request.Address
	views := request.Snapshot.Views

	switch address.Kind {
	case ObjectEstate:
		name := request.Snapshot.Estate.Name
		header := ObjectHeaderView{Icon: ObjectIconView{Icon: iconEstate}, Name: name}

		// The estate is the root: a breadcrumb would only repeat the name the
		// object header shows directly above it, so it has none.
		return header, nil, nil

	case ObjectGroup:
		members, err := GroupDevices(views, address.Group)
		if err != nil {
			return ObjectHeaderView{}, nil, err
		}

		header := ObjectHeaderView{
			Icon: ObjectIconView{Icon: iconGroup},
			Name: groupTitle(address.Group[len(address.Group)-1]),
		}
		for index := range members {
			if members[index].TasksActive > 0 {
				header.Busy = true
			}
		}

		crumbs := append([]CrumbView{estateCrumb(request)}, groupCrumbs(request, address.Group, true)...)

		return header, crumbs, nil

	case ObjectDevice:
		view, err := FindDevice(views, address.Device)
		if err != nil {
			return ObjectHeaderView{}, nil, err
		}

		header := ObjectHeaderView{
			Icon:   ObjectIconView{Icon: iconDevice, Status: DisplayStatus(view.State), Marked: true},
			Name:   view.Device.Name,
			Stated: true,
			State:  view.State,
			Busy:   view.TasksActive > 0,
		}

		crumbs := append([]CrumbView{estateCrumb(request)}, groupCrumbs(request, devicePath(request, view), false)...)
		crumbs = append(crumbs, CrumbView{
			Title: view.Device.Name, Href: deviceAddress(request, view.Device.Name).String(), Current: true,
		})

		return header, crumbs, nil

	case ObjectTasks:
		header := ObjectHeaderView{Icon: ObjectIconView{Icon: iconTasks}, Name: tasksTitle}
		crumbs := []CrumbView{estateCrumb(request), {Title: tasksTitle, Href: address.Path(), Current: true}}

		return header, crumbs, nil

	case ObjectAdmin:
		header := ObjectHeaderView{Icon: ObjectIconView{Icon: iconAdmin}, Name: adminTitle}
		crumbs := []CrumbView{
			estateCrumb(request),
			{Title: adminTitle, Href: address.Path()},
			{Title: tabTitle(address), Href: address.Path(), Current: true},
		}

		return header, crumbs, nil

	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}
}

// describePage fills the main pane of the page the request names, without
// its content. It answers the refused outcome, whose content is set, for an
// object the estate does not hold and for a tree view the hub does not have.
func describePage(request *PageRequest) *pageOutcome {
	object, crumbs, err := describeObject(request)
	if err != nil {
		return refusedOutcome(request, err, failureStatus(err))
	}

	if request.treeViewRefusal != nil {
		outcome := refusedOutcome(request, request.treeViewRefusal, http.StatusBadRequest)
		outcome.title = object.Name
		outcome.main.Object = object
		outcome.selects = true

		return outcome
	}

	address := request.Address
	object.Menu = objectMenu(request, Address{
		Kind: address.Kind, Group: address.Group, Device: address.Device, Tab: TabDashboard, View: linkView(request),
	}, object.Name)
	object.Primary, object.HasPrimary = objectPrimary(request)

	return &pageOutcome{
		status: http.StatusOK,
		title:  object.Name,
		main: MainView{
			Object:    object,
			Crumbs:    crumbs,
			Tabs:      tabViews(request),
			TabsLabel: object.Name + " tabs",
		},
		selects: true,
	}
}

// builtNothing is the failure of a builder that answered no component and no
// error, which its contract forbids.
func builtNothing() error {
	return opererr.New("This page built nothing to show.",
		"Its builder answered neither content nor an error, which is a defect of the fleet interface, so the pane cannot be drawn.",
		"ze show doctor")
}

// buildRegion runs one builder of a page and answers what the region shows:
// the builder's component, or the state that matches its failure. The status
// is http.StatusOK unless the builder failed.
func buildRegion(ctx context.Context, request *PageRequest, build PageBuilder) (templ.Component, int) {
	component, err := build(ctx, request)
	if err != nil {
		return failureHero(request, err), failureStatus(err)
	}
	if component == nil {
		return failureHero(request, builtNothing()), http.StatusInternalServerError
	}

	return component, http.StatusOK
}

// resolvePage resolves the request to its registered page and builds it. A
// tab no page family registered a page for shows the "not built yet" state,
// which is an answer and no failure.
func resolvePage(ctx context.Context, request *PageRequest) *pageOutcome {
	outcome := describePage(request)
	if outcome.content != nil {
		// The object or the tree view was refused: the state is the content.
		return outcome
	}

	address := request.Address

	page, built := request.pages.lookup(address.Kind, address.Tab)
	if !built {
		outcome.content = notBuiltHero(request)
		return outcome
	}

	outcome.content, outcome.status = buildRegion(ctx, request, page.Content)
	outcome.main.ContentLive = !page.Static

	if page.SidePanel == nil {
		return outcome
	}

	closed := address
	closed.View.Row = ""
	outcome.panel = SidePanelView{Present: true, CloseHref: closed.String()}

	if address.View.Row == "" {
		return outcome
	}

	// The side panel's own failure is shown in the panel, and leaves the
	// status of the page as its content set it.
	outcome.panel.Open = true
	outcome.side, _ = buildRegion(ctx, request, page.SidePanel)

	return outcome
}

// failureStatus answers the status of a page whose object or content failed
// with err. A state that is an operating condition of a device (offline, a
// refused relay, a view its software lacks, no answer in time) is a page
// that rendered, so it answers success.
func failureStatus(err error) int {
	switch FailureOf(err) {
	case FailureNotFound:
		return http.StatusNotFound
	case FailureUnauthorized:
		return http.StatusForbidden
	case FailureOffline, FailureRelayRefused, FailureUnsupported, FailureDeadline:
		return http.StatusOK
	case FailureFailed:
		return http.StatusInternalServerError
	case FailureUnspecified:
		panic("BUG: fleetui: FailureOf answered no failure")
	default:
		panic("BUG: fleetui: FailureOf answered a failure outside its set")
	}
}

// newAssetHandler serves the embedded fleet assets at fleetAssetsPath. Assets
// use no-cache, as the web package's own do, so a browser picks up a new
// binary's files without a hard refresh.
func newAssetHandler() http.Handler {
	// Stripping fleetPath leaves "assets/<file>", which is the file's path
	// in the embedded tree.
	files := http.StripPrefix(fleetPath, http.FileServerFS(assetsFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		files.ServeHTTP(w, r)
	})
}
