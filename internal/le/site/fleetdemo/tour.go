//go:build ze_web && ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: what the tour records, its chunks and its bounds
// Related: render.go -- the render that crawls this tour, splits it and writes it

package sitefleetdemo

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

// The bounds of one recording. Each is a figure the tour is not expected to
// reach: a render that passes one fails naming it, because the tour grew past
// what was planned, and the fix is a decision (narrow the tour) rather than a
// figure to raise in silence. The render's report gives each chunk's figures
// and the bytes of every kind of request (sizeByKind), which is where a
// narrowing starts.
//
// The first render of the full tour, with every page recorded as the whole
// shell, asked about 9,000 requests in about seven minutes, and its estate
// chunk alone was 54,751,363 bytes. Recording a navigation as the region htmx
// asks for, and the narrowing that docs/contributing/gh-pages.md ("Render the
// fleet demonstration") records with its figures, are what bring it under
// these.
const (
	// tourExchangesMax bounds the requests one render asks, the follow-ups
	// and the discovered requests included. The narrowed tour recorded 2,007
	// on 2026-11-19: its tabs and sub-tabs, their fragments, the day charts
	// and the console's suggestions for each of 44 devices, more for each
	// featured one, and the estate, the groups and the administration.
	tourExchangesMax = 5_000
	// chunkExchangesMax bounds one chunk's exchanges. The estate's is the
	// largest, estimated at about 700: its tabs, the devices table's views,
	// the tasks, and every tree branch, which is addressed on the estate.
	chunkExchangesMax = 2_000
	// chunkBytesMax bounds one chunk file, which the host page fetches whole
	// the first time the frame needs it. 16 MiB is the most a visitor should
	// wait for on opening an object; the estate's is the largest, its devices
	// table some 77 kB an answer over about 60 views.
	chunkBytesMax = 16 << 20
	// totalBytesMax bounds the frame and every chunk together. Measured on
	// 2026-11-19 at 87.6 MB, most of it the device tabs (959 partial pages of
	// about 60 kB). The recording lives in the website output's ignored tmp/
	// for the owner's local review and is never published, and a viewer
	// loads one chunk per object it opens (chunkBytesMax bounds that), so
	// the whole is a disk figure, not a download. 128 MiB leaves room for the
	// tour to grow; docs/contributing/gh-pages.md records the figures and the
	// larger lever (the inline icon markup, about half of every table row)
	// for a published demonstration.
	totalBytesMax = 128 << 20
	// discoveryRoundsMax bounds the crawls that ask what the answers name
	// outside hx-get. A tree branch names the branches below it, so a round
	// per tree level is needed, and a tree has at most TreeLevelsMax of them.
	discoveryRoundsMax = fleetui.TreeLevelsMax + 3
)

// The fleet addresses the tour and its chunks know apart.
const (
	fleetPrefix    = "/fleet/"
	fragmentPrefix = fleetPrefix + "fragment/"
	// sidePanelPrefix is the side panel fragment an eye control and the
	// panel's close control ask for in place of the page with or without the
	// row (fleetui.PanelLink).
	sidePanelPrefix = fragmentPrefix + fleetui.SidePanelFragment + "/"
	actionPrefix    = fleetPrefix + "action/"
	// consolePrefix is where the terminal posts a command:
	// /fleet/console/<device>.
	consolePrefix = fleetPrefix + "console/"
	// consoleField is the form field the terminal posts the command in.
	consoleField = "command"

	// devicesTabName is the estate's devices table, whose every view the tour
	// records.
	devicesTabName = "devices"
	// estateDevicesChunk holds every view of the estate's devices table.
	estateDevicesChunk = "estate-devices"
	// The filter fields the tour builds single-term filters from, beside the
	// label keys.
	stateField   = "state"
	versionField = "version"
)

// The attributes the fleet scripts read a request from, outside hx-get.
var (
	// lazyAttribute is a tree row's branch, which the shell script asks
	// through htmx when the row first opens.
	lazyAttribute = regexp.MustCompile(`\bdata-fleet-lazy="([^"]+)"`)
	// chartAttribute is a chart's data, which the chart script fetches with
	// no htmx header.
	chartAttribute = regexp.MustCompile(`\bdata-chart-url="([^"]+)"`)
	// terminalAttribute is where a console posts its commands, with no htmx
	// header, and fillAttribute each command it suggests.
	terminalAttribute = regexp.MustCompile(`\bdata-terminal-url="([^"]+)"`)
	fillAttribute     = regexp.MustCompile(`\bdata-terminal-fill="([^"]+)"`)
)

// bounds are the figures one recording may not pass.
type bounds struct {
	chunkExchanges int
	chunkBytes     int
	totalBytes     int
}

// tourBounds are the bounds a render enforces.
var tourBounds = bounds{chunkExchanges: chunkExchangesMax, chunkBytes: chunkBytesMax, totalBytes: totalBytesMax}

// tour is what one render records: the requests it asks, and the objects
// whose every registered page they open. planTour derives it from the fleet
// package's registries and the sample estate, so a page registered later is
// recorded without a change here.
type tour struct {
	steps []sitelgreplay.Request
	// planned holds the key of every step, so no step is asked twice.
	planned map[string]bool
	// objects are the estate, each group node of the default tree view, each
	// device, the task list and the administration, in that order.
	objects []fleetui.Address
	// featured names the devices recorded at every chart range, with every
	// sort of their tables and with their side panels: the first device by
	// name of each display state. The other devices show the default view of
	// each table at the default range.
	featured map[string]bool
}

// planTour answers the tour over the estate source holds, as far as scope
// reaches.
func planTour(ctx context.Context, source fleetui.Source, account string, scope tourScope) (tour, error) {
	snapshot, err := fleetui.ReadSnapshot(ctx, source)
	if err != nil {
		return tour{}, err
	}
	devices := make([]fleetui.Device, 0, len(snapshot.Views))
	for index := range snapshot.Views {
		devices = append(devices, snapshot.Views[index].Device)
	}
	slices.SortFunc(devices, func(a, b fleetui.Device) int { return strings.Compare(a.Name, b.Name) })

	plan := tour{planned: map[string]bool{}, featured: map[string]bool{}}
	plan.add(sitelgreplay.Request{Method: http.MethodGet, Path: entryPath})

	for _, state := range fleetui.DisplayStates() {
		for index := range devices {
			if fleetui.DeviceDisplayState(&devices[index]) == state {
				plan.featured[devices[index].Name] = true
				break
			}
		}
	}
	opened := devices
	if scope.sampled {
		opened = plan.sample(devices)
	}

	estate := fleetui.Address{Kind: fleetui.ObjectEstate}
	plan.objects = append(plan.objects, estate)
	treeView, err := fleetui.ResolveTreeView(fleetui.TreeViewDefault, snapshot.TreeViews)
	if err != nil {
		return tour{}, err
	}
	tree, err := fleetui.BuildTree(snapshot.Estate.Name, treeView, snapshot.Views)
	if err != nil {
		return tour{}, err
	}
	groups := tree.Root.Children
	if scope.sampled && len(groups) > 1 {
		groups = groups[:1]
	}
	plan.objects = appendGroups(plan.objects, groups)
	for index := range opened {
		plan.objects = append(plan.objects, fleetui.Address{Kind: fleetui.ObjectDevice, Device: opened[index].Name})
	}
	plan.objects = append(plan.objects, fleetui.Address{Kind: fleetui.ObjectTasks}, fleetui.Address{Kind: fleetui.ObjectAdmin})
	pages := objectPages(plan.objects, fleetui.Tabs)
	for index := range pages {
		plan.page(pages[index])
	}

	plan.charts(estate, opened)
	plan.treeViews(snapshot.TreeViews)
	if scope.sampled {
		return plan, nil
	}
	// The devices table shows the whole estate whatever the tour opens, so
	// its pages and its filter terms are counted over every device.
	if err := plan.devicesTable(ctx, source, account, devices); err != nil {
		return tour{}, err
	}

	return plan, nil
}

// sample answers the first featured device and the first ordinary one of
// devices, which are sorted by name. The tour plans only these two; a device
// it does not plan is reached only when a recorded answer links to it (the
// states page's gallery names one), and then holds just those followed
// answers, so featured may keep the others.
func (plan *tour) sample(devices []fleetui.Device) []fleetui.Device {
	var kept []fleetui.Device
	var featured, ordinary bool
	for index := range devices {
		if plan.featured[devices[index].Name] {
			if !featured {
				featured = true
				kept = append(kept, devices[index])
			}
			continue
		}
		if !ordinary {
			ordinary = true
			kept = append(kept, devices[index])
		}
	}
	return kept
}

// appendGroups appends the group node of every node of nodes and of every
// node below them, each before its children.
func appendGroups(objects []fleetui.Address, nodes []fleetui.TreeNode) []fleetui.Address {
	for index := range nodes {
		objects = append(objects, fleetui.Address{Kind: fleetui.ObjectGroup, Group: nodes[index].Path})
		objects = appendGroups(objects, nodes[index].Children)
	}
	return objects
}

// objectPages answers the default view of every registered page of objects:
// each tab tabs answers for the object's kind, or the object itself for a
// kind that takes no tab.
func objectPages(objects []fleetui.Address, tabs func(fleetui.ObjectKind) []fleetui.Tab) []fleetui.Address {
	var pages []fleetui.Address
	for index := range objects {
		object := &objects[index]
		registered := tabs(object.Kind)
		if len(registered) == 0 {
			pages = append(pages, *object)
			continue
		}
		for _, tab := range registered {
			page := *object
			page.Tab = tab.Name
			pages = append(pages, page)
		}
	}
	return pages
}

// unreached answers each registered page of objects whose default view
// holds no exchange, as its tab link asks for it.
func unreached(objects []fleetui.Address, tabs func(fleetui.ObjectKind) []fleetui.Tab, held func(key string) bool) []string {
	var missing []string
	pages := objectPages(objects, tabs)
	for index := range pages {
		request := pageRequest(pages[index])
		key := sitelgreplay.ExchangeKey(request.Method, request.Path, request.Query, nil, request.HTMX)
		if !held(key) {
			missing = append(missing, key)
		}
	}
	return missing
}

// add appends request to the steps unless an earlier step asks it.
func (plan *tour) add(request sitelgreplay.Request) {
	key := sitelgreplay.ExchangeKey(request.Method, request.Path, request.Query, request.Form, request.HTMX)
	if plan.planned[key] {
		return
	}
	plan.planned[key] = true
	plan.steps = append(plan.steps, request)
}

// page adds the request a link of the shell sends for address: htmx asks it
// into the content region.
func (plan *tour) page(address fleetui.Address) {
	plan.add(pageRequest(address))
}

// pageRequest answers the request a link of the shell sends for address.
// Every link of the shell is an hx-get into the content region, and the
// script navigates through htmx too, so each one carries HX-Request.
func pageRequest(address fleetui.Address) sitelgreplay.Request {
	return sitelgreplay.Request{Method: http.MethodGet, Path: address.Path(), Query: fieldsOf(address.View.Query()), HTMX: true}
}

// fieldsOf answers query as fields, by name and then in order.
func fieldsOf(query url.Values) []sitelgreplay.Field {
	names := make([]string, 0, len(query))
	for name := range query {
		names = append(names, name)
	}
	slices.Sort(names)
	var fields []sitelgreplay.Field
	for _, name := range names {
		for _, value := range query[name] {
			fields = append(fields, sitelgreplay.Field{Name: name, Value: value})
		}
	}
	return fields
}

// charts adds the dashboard of the estate and of each featured device at
// every chart range. The dashboard's charts follow from its answer.
func (plan *tour) charts(estate fleetui.Address, devices []fleetui.Device) {
	dashboards := []fleetui.Address{estate}
	for index := range devices {
		if plan.featured[devices[index].Name] {
			dashboards = append(dashboards, fleetui.Address{Kind: fleetui.ObjectDevice, Device: devices[index].Name})
		}
	}
	for index := range dashboards {
		for _, chartRange := range fleetui.ChartRanges() {
			page := dashboards[index]
			page.Tab = fleetui.TabDashboard
			page.View.Range = chartRange
			plan.page(page)
		}
	}
}

// treeViews adds the estate dashboard in each built-in tree view and each
// the hub declares, as the sidebar's view choice opens it.
func (plan *tour) treeViews(declared []fleetui.TreeView) {
	for _, view := range append(fleetui.BuiltinTreeViews(), declared...) {
		plan.page(fleetui.Address{Kind: fleetui.ObjectEstate, Tab: fleetui.TabDashboard, View: fleetui.ViewParams{TreeView: view.Name}})
	}
}

// devicesTable adds every view of the estate's devices table the tour
// records, each at the default of every other part of the view: each
// sortable column both ways, each page size, each page, a single-term filter
// for each state, each version and each label value, and each saved filter.
func (plan *tour) devicesTable(ctx context.Context, source fleetui.Source, account string, devices []fleetui.Device) error {
	if !slices.ContainsFunc(fleetui.Tabs(fleetui.ObjectEstate), func(tab fleetui.Tab) bool { return tab.Name == devicesTabName }) {
		return planFailure("the estate has no " + devicesTabName + " tab")
	}
	fields := fleetui.DeviceFilters().Fields()
	for _, field := range []string{stateField, versionField} {
		if !slices.Contains(fields, field) {
			return planFailure("the devices filter has no " + field + " field")
		}
	}
	// Each view is asked from the table's default view, where its sort
	// link, its pager and its query row are, so the shell answers it as a
	// step that stays on the estate, without the sidebar.
	table := func(view fleetui.ViewParams) {
		step := pageRequest(fleetui.Address{Kind: fleetui.ObjectEstate, Tab: devicesTabName, View: view})
		step.Current = fleetui.Address{Kind: fleetui.ObjectEstate, Tab: devicesTabName}.Path()
		plan.add(step)
	}

	for _, column := range fleetui.DeviceColumns().Keys() {
		table(fleetui.ViewParams{Sort: fleetui.SortSpec{Column: column}})
		table(fleetui.ViewParams{Sort: fleetui.SortSpec{Column: column, Descending: true}})
	}
	// Each page size at its first page, and each page at the default size:
	// no page of another size, which is a cross product (gh-pages.md, "Render
	// the fleet demonstration").
	for _, size := range fleetui.PageSizes() {
		table(fleetui.ViewParams{Size: size})
	}
	for number := 2; number <= fleetui.Paginate(len(devices), 1, fleetui.PageSizeDefault).Pages; number++ {
		table(fleetui.ViewParams{Page: number})
	}

	states, _ := fleetui.DeviceFilters().FieldValues(stateField)
	var versions, labels []string
	for index := range devices {
		versions = append(versions, devices[index].Inventory.SoftwareVersion)
		for _, label := range devices[index].Labels {
			labels = appendLabelTerm(labels, fields, label.Key, label.Value)
		}
	}
	var terms []string
	for _, state := range states {
		terms = appendFilterTerm(terms, stateField, state)
	}
	for _, version := range sortedOnce(versions) {
		terms = appendFilterTerm(terms, versionField, version)
	}
	terms = append(terms, sortedOnce(labels)...)
	saved, err := source.SavedFilters(ctx, account)
	if err != nil {
		return err
	}
	for _, filter := range saved {
		terms = append(terms, filter.Filter)
	}
	for _, term := range terms {
		table(fleetui.ViewParams{Filter: term})
	}
	return nil
}

// sortedOnce answers values sorted, each once.
func sortedOnce(values []string) []string {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}

// planFailure answers the operator error of a tour the fleet package no
// longer matches.
func planFailure(problem string) *opererr.Error {
	return opererr.New("The fleet demonstration's tour cannot be planned: "+problem,
		"The tour records the estate's devices table by the tab and the filter fields the fleet package registers, and one of them is gone.",
		"update the names in internal/le/site/fleetdemo/tour.go to the fleet package's, then "+RenderCommand)
}

// follow decides whether the crawl asks a request an answer names through
// hx-get. It asks every fragment but the side panel, since a page's fragments
// are part of it. Of the page links and the side panels it asks what a reader
// reaches from a recorded view in one step without typing: from the estate
// dashboard every estate view it links; from any other default view the same
// tab's sub-tabs, and its side panels where panels says so; from a featured
// device's views their sorts too; from a recorded side panel, its close
// control. It asks no filtered view, so no combination of filter, sort, page
// and panel is recorded. The crawl names the page each follow-up leaves, so
// the shell answers a step inside one object without the sidebar.
func (plan *tour) follow(parent sitelgreplay.Exchange, asked sitelgreplay.Request) bool {
	if asked.Method != http.MethodGet {
		return false
	}
	panel := strings.HasPrefix(asked.Path, sidePanelPrefix)
	if strings.HasPrefix(asked.Path, fragmentPrefix) && !panel {
		return true
	}
	if strings.HasPrefix(asked.Path, actionPrefix) {
		return false
	}
	target, err := addressOf(asked.Path, asked.Query)
	if err != nil {
		return false
	}
	from, err := addressOf(parent.Path, parent.Query)
	if err != nil {
		return false
	}
	if panel && from.View.Row != "" && target.View.Row == "" {
		// The close control of a recorded panel asks for the panel of the
		// same view with no row.
		closed := from.View
		closed.Row = ""
		return sameObject(from, target) && from.Tab == target.Tab && target.View == closed
	}
	plain := fleetui.ParseViewParams(url.Values{})
	if from.Kind == fleetui.ObjectEstate && from.Tab == fleetui.TabDashboard && from.View == plain {
		return target.Kind == fleetui.ObjectEstate
	}
	if !sameObject(from, target) || from.Tab != target.Tab || target.View.Filter != "" {
		return false
	}
	return viewStep(from.View, target.View, plan.featured[from.Device], plan.panels(from))
}

// panels reports whether the tour opens the side panels of the rows page
// shows. A group's rows are the estate's, whose panels the estate holds, and
// an ordinary device shows the default view of each table only, since its
// panels repeat over 44 devices what the featured devices show (gh-pages.md,
// "Render the fleet demonstration").
func (plan *tour) panels(page fleetui.Address) bool {
	switch page.Kind {
	case fleetui.ObjectGroup:
		return false
	case fleetui.ObjectDevice:
		return plan.featured[page.Device]
	case fleetui.ObjectEstate, fleetui.ObjectTasks, fleetui.ObjectAdmin, fleetui.ObjectUnspecified:
	}
	return true
}

// viewStep reports whether to is one step from the default view from shows,
// or from its sub-tab: another sub-tab, a side panel when rows is set, and a
// sort when sorted is set. A side panel of a sorted view is no step.
func viewStep(from, to fleetui.ViewParams, sorted, rows bool) bool {
	plain := fleetui.ParseViewParams(url.Values{})
	base := plain
	base.Sub, base.Sort = from.Sub, from.Sort
	if from != base {
		return false
	}
	if !sorted && (from.Sort != plain.Sort || to.Sort != from.Sort) {
		return false
	}
	if to.Row != "" && (!rows || to.Sort != plain.Sort) {
		return false
	}
	to.Row, to.Sub, to.Sort = "", from.Sub, from.Sort
	return to == base
}

// sameObject reports whether two addresses name one object.
func sameObject(a, b fleetui.Address) bool {
	return a.Kind == b.Kind && a.Device == b.Device && slices.Equal(a.Group, b.Group)
}

// addressOf answers the page a request is for: the page itself, or the page
// a fragment belongs to.
func addressOf(path string, query []sitelgreplay.Field) (fleetui.Address, error) {
	target, err := url.Parse(path)
	if err != nil {
		return fleetui.Address{}, err
	}
	values := url.Values{}
	for _, field := range query {
		values.Add(field.Name, field.Value)
	}
	target.RawQuery = values.Encode()
	if strings.HasPrefix(path, fragmentPrefix) {
		fragment, err := fleetui.ParseFragment(target)
		return fragment.Page, err
	}
	return fleetui.ParseAddress(target)
}

// discover answers the requests the fleet scripts send from what the
// answers name outside hx-get, in the order the answers name them: each tree
// branch, as htmx asks it; each chart's data and each suggested console
// command, as the scripts fetch them, with no htmx header.
func discover(exchanges []sitelgreplay.Exchange) ([]sitelgreplay.Request, error) {
	var found []sitelgreplay.Request
	for index := range exchanges {
		exchange := &exchanges[index]
		if !strings.HasPrefix(exchange.ContentType, "text/html") {
			continue
		}
		for _, target := range attributeTargets(lazyAttribute, exchange.Body) {
			request, err := targetRequest(target, true)
			if err != nil {
				return nil, err
			}
			found = append(found, request)
		}
		for _, target := range attributeTargets(chartAttribute, exchange.Body) {
			request, err := targetRequest(target, false)
			if err != nil {
				return nil, err
			}
			found = append(found, request)
		}
		for _, terminal := range attributeTargets(terminalAttribute, exchange.Body) {
			for _, command := range attributeTargets(fillAttribute, exchange.Body) {
				found = append(found, sitelgreplay.Request{
					Method: http.MethodPost, Path: terminal,
					Form: []sitelgreplay.Field{{Name: consoleField, Value: command}},
				})
			}
		}
	}
	return found, nil
}

// attributeTargets answers the unescaped value of every match of pattern in
// body, in order.
func attributeTargets(pattern *regexp.Regexp, body string) []string {
	matches := pattern.FindAllStringSubmatch(body, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, html.UnescapeString(match[1]))
	}
	return values
}

// targetRequest answers a GET of a path-and-query an attribute names.
func targetRequest(target string, htmx bool) (sitelgreplay.Request, error) {
	path, raw, _ := strings.Cut(target, "?")
	if !strings.HasPrefix(path, fleetPrefix) {
		return sitelgreplay.Request{}, planFailure("an answer names the request " + strconv.Quote(target) + ", outside " + fleetPrefix)
	}
	request := sitelgreplay.Request{Method: http.MethodGet, Path: path, HTMX: htmx}
	if raw == "" {
		return request, nil
	}
	for pair := range strings.SplitSeq(raw, "&") {
		name, value, _ := strings.Cut(pair, "=")
		decodedName, err := url.QueryUnescape(name)
		if err != nil {
			return sitelgreplay.Request{}, planFailure("an answer names the request " + strconv.Quote(target) + ", whose query does not decode")
		}
		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			return sitelgreplay.Request{}, planFailure("an answer names the request " + strconv.Quote(target) + ", whose query does not decode")
		}
		request.Query = append(request.Query, sitelgreplay.Field{Name: decodedName, Value: decodedValue})
	}
	return request, nil
}

// chunkOf answers the chunk an exchange belongs to: the object its page
// shows. The estate and the task list share the estate's.
func chunkOf(exchange sitelgreplay.Exchange) (string, error) {
	if device, found := strings.CutPrefix(exchange.Path, consolePrefix); found {
		return chunkName("device", device)
	}
	page, err := addressOf(exchange.Path, exchange.Query)
	if err != nil {
		return "", err
	}
	switch page.Kind {
	case fleetui.ObjectEstate, fleetui.ObjectTasks:
		// The devices table's views are most of the estate's bytes (sorts,
		// sizes, pages, filters, their panels), so they travel in a chunk of
		// their own: the estate's dashboard opens without loading them, and
		// they load when a visitor first opens the Devices tab.
		if page.Kind == fleetui.ObjectEstate && page.Tab == devicesTabName {
			return estateDevicesChunk, nil
		}
		return "estate", nil
	case fleetui.ObjectGroup:
		levels := make([]string, 0, len(page.Group))
		for _, level := range page.Group {
			if level.Missing {
				levels = append(levels, level.Key)
				continue
			}
			levels = append(levels, level.Key+"-"+level.Value)
		}
		return chunkName("group", strings.Join(levels, "-"))
	case fleetui.ObjectDevice:
		return chunkName("device", page.Device)
	case fleetui.ObjectAdmin:
		return "admin", nil
	case fleetui.ObjectUnspecified:
	}
	return "", planFailure("the exchange " + exchange.Key + " belongs to no object")
}

// chunkName answers kind and the object's name as a chunk name: lower-case
// letters, digits and single hyphens.
func chunkName(kind, name string) (string, error) {
	var safe strings.Builder
	hyphen := true
	for _, character := range strings.ToLower(name) {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			safe.WriteRune(character)
			hyphen = false
			continue
		}
		if !hyphen {
			safe.WriteByte('-')
			hyphen = true
		}
	}
	text := strings.TrimSuffix(safe.String(), "-")
	if text == "" {
		return "", planFailure("the " + kind + " " + strconv.Quote(name) + " has no letter or digit to name its chunk by")
	}
	return kind + "-" + text, nil
}

// The kinds of request the render's report counts its bytes by.
const (
	kindFullPage    = "full page"
	kindPartialPage = "htmx partial page"
	kindSidePanel   = "side panel fragment"
	kindChart       = "chart data"
	kindConsole     = "console"
	kindPosted      = "posted action"
	// kindFragment prefixes a fragment's registered name: the tree branches
	// are "fragment tree-branch".
	kindFragment = "fragment "

	jsonContentType = "application/json"
)

// exchangeKind answers the kind of request exchange answers: a page as
// htmx or the browser asks it, the side panel fragment an eye or a close
// control asks, any other fragment by its name, a chart's data or a console
// command. A page whose address names a row is a page: a link of another page
// opens it whole.
func exchangeKind(exchange sitelgreplay.Exchange) string {
	if exchange.Method == http.MethodPost {
		if strings.HasPrefix(exchange.Path, consolePrefix) {
			return kindConsole
		}
		return kindPosted
	}
	if name, found := strings.CutPrefix(exchange.Path, fragmentPrefix); found {
		if strings.HasPrefix(exchange.ContentType, jsonContentType) {
			return kindChart
		}
		name, _, _ = strings.Cut(name, "/")
		if name == fleetui.SidePanelFragment {
			return kindSidePanel
		}
		return kindFragment + name
	}
	if exchange.HTMX {
		return kindPartialPage
	}
	return kindFullPage
}

// sizeByKind answers the exchanges and the body bytes of each kind of
// request in exchanges, the largest first.
func sizeByKind(exchanges []sitelgreplay.Exchange) []kindSize {
	byKind := map[string]*kindSize{}
	for index := range exchanges {
		kind := exchangeKind(exchanges[index])
		size, found := byKind[kind]
		if !found {
			size = &kindSize{kind: kind}
			byKind[kind] = size
		}
		size.exchanges++
		size.bytes += len(exchanges[index].Body)
	}
	sizes := make([]kindSize, 0, len(byKind))
	for _, size := range byKind {
		sizes = append(sizes, *size)
	}
	slices.SortFunc(sizes, func(a, b kindSize) int {
		if a.bytes != b.bytes {
			return b.bytes - a.bytes
		}
		return strings.Compare(a.kind, b.kind)
	})
	return sizes
}

// check refuses a split past any bound, naming the bound, the chunk and the
// figure, then what the chunk or the whole holds by kind of request, so the
// failure says what to narrow. frameBytes is the frame document's size, which
// the total counts; sizes holds a chunk's kinds by its name, and the whole's
// under wholeTour.
func (limit bounds) check(chunks []sitelgreplay.Chunk, frameBytes int, sizes map[string][]kindSize) error {
	total := frameBytes
	for _, chunk := range chunks {
		if len(chunk.Keys) > limit.chunkExchanges {
			return boundFailure("chunkExchangesMax", limit.chunkExchanges,
				"chunk "+chunk.Name+" holds "+strconv.Itoa(len(chunk.Keys))+" exchanges"+describeKinds(sizes[chunk.Name]))
		}
		if chunk.Size > limit.chunkBytes {
			return boundFailure("chunkBytesMax", limit.chunkBytes,
				"chunk "+chunk.Name+" is "+strconv.Itoa(chunk.Size)+" bytes"+describeKinds(sizes[chunk.Name]))
		}
		total += chunk.Size
	}
	if total > limit.totalBytes {
		return boundFailure("totalBytesMax", limit.totalBytes,
			"the frame and its chunks are "+strconv.Itoa(total)+" bytes"+describeKinds(sizes[wholeTour]))
	}
	return nil
}

// wholeTour is the name check reads the whole recording's kinds under. No
// chunk is named so: a chunk name is a kind and an object (chunkName).
const wholeTour = "tour"

// describeKinds answers sizes as the clause a bound failure ends with, or
// nothing when there are none.
func describeKinds(sizes []kindSize) string {
	if len(sizes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sizes))
	for _, size := range sizes {
		parts = append(parts, size.kind+" "+strconv.Itoa(size.exchanges)+" answers "+strconv.Itoa(size.bytes)+" bytes")
	}
	return "; by kind of request: " + strings.Join(parts, ", ")
}

// boundFailure answers the operator error of a recording past a bound.
func boundFailure(bound string, figure int, what string) *opererr.Error {
	return opererr.New("The fleet demonstration is past its bound "+bound+" of "+strconv.Itoa(figure)+": "+what,
		"The bounds are the tour's measured size with room to spare, so the tour grew, and nothing was written.",
		"narrow the tour where the kinds above say it grew (docs/contributing/gh-pages.md, \"Render the fleet demonstration\"), or raise "+bound+" in internal/le/site/fleetdemo/tour.go with its reason, then "+RenderCommand)
}

// appendFilterTerm answers terms with the filter that asks field for value,
// as the page's own assisted menus write it. A value the filter grammar cannot
// carry (fleetui.SetFilterTerm answers false) has no link on the page, so the
// tour records no address for it either.
func appendFilterTerm(terms []string, field, value string) []string {
	term, carried := fleetui.SetFilterTerm("", field, value)
	if !carried {
		return terms
	}

	return append(terms, term)
}

// appendLabelTerm answers terms with the filter that asks for the label
// key=value, as the page's Label menu writes it (fleetui.SetLabelFilterTerm,
// over fields, the fields of the devices table): the long form label:key=value
// for a key named like a field, label or has. A pair the filter grammar
// cannot carry has no link on the page, so the tour records no address for
// it either.
func appendLabelTerm(terms, fields []string, key, value string) []string {
	term, carried := fleetui.SetLabelFilterTerm("", fields, key, value)
	if !carried {
		return terms
	}

	return append(terms, term)
}
