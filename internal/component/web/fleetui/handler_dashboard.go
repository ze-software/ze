// Design: docs/architecture/fleet-interface.md -- the dashboards of the estate, of a group node and of a device
// Related: handler_dashboard_cards.go -- the cards of the estate and group dashboards
// Related: handler_dashboard_device.go -- the cards of the device dashboard
// Related: handler_chart.go -- the data of the chart cards
// Related: register_dashboard_pages.go -- the pages, the fragments and the cards it registers
// Related: page_dashboard.templ -- the markup of a dashboard and of its own cards
//
// A dashboard is an ordered list of registered cards, each with its width.
// The page draws every card in place, and each card is also a fragment of its
// own (dashboard-card, item the card's name), so a card is built on its own,
// fails on its own and is fetched again on its own: a card whose source
// fails shows the failed state with a retry that asks for that card alone,
// and every other card renders.

package fleetui

import (
	"context"
	"sync"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The fragments the dashboards register.
const (
	// fragmentDashboardCard is one card of a dashboard, named by its item.
	fragmentDashboardCard = "dashboard-card"
	// fragmentDashboardChart is the JSON of one chart, named by its item.
	fragmentDashboardChart = "dashboard-chart"
)

// The tabs and administration pages the cards of a dashboard link to.
const (
	dashTabDevices    = "devices"
	dashTabAlarms     = "alarms"
	dashTabRollouts   = "rollouts"
	dashTabConfig     = "config"
	dashTabRouting    = "routing"
	dashTabInterfaces = "interfaces"
	dashTabAudit      = "audit"
)

// dashboardSlotPrefix starts the id of the element that holds one card.
const dashboardSlotPrefix = "fleet-card-"

// dashboardCardBuilder builds one card over the devices of the dashboard's
// object. frame holds the card's title and width as the card was
// registered; the builder adds its links and badge. It answers the card, or
// the source's operator error: the card then shows the failed state and the
// others render. It MUST NOT answer a nil component with a nil error.
type dashboardCardBuilder func(ctx context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error)

// dashboardCard is one registered card of a dashboard.
type dashboardCard struct {
	// Name names the card in its fragment address and in the id of its
	// slot. It is unique among the cards of one kind of object.
	Name string
	// Title is the card's title. The failed state of the card keeps it.
	Title string
	// Span is the card's width in grid columns.
	Span int
	// Live marks a card showing what a device reports now. The dashboard of
	// an offline device leaves it out, because it would show stale values as
	// current, and shows the offline state in its place.
	Live bool
	// Build builds the card.
	Build dashboardCardBuilder
}

// dashboardRegistry holds the cards of each kind of object's dashboard, in
// registration order, which is the order the dashboard draws them in. The
// zero value is an empty registry. Safe for concurrent use.
type dashboardRegistry struct {
	mu    sync.RWMutex
	cards map[ObjectKind][]dashboardCard
}

// dashboardCards is the card registry of this build. The register file fills
// it from init(), and nothing removes from it.
var dashboardCards dashboardRegistry

// The subjects a refused card registration names.
const (
	bugDashboardCard = "the dashboard card"
	dashboardSubject = "the fleet dashboards"
)

// mustRegisterDashboardCard adds a card at the end of the dashboard of kind.
// Call it from init() in a register file. It panics on a card with no name,
// no builder or a width outside the grid, and on a second card of one name
// for one kind: each is a defect of the build's composition.
func mustRegisterDashboardCard(kind ObjectKind, card dashboardCard) {
	if card.Name == "" {
		registrationBug(bugDashboardCard, card.Name, dashboardSubject, bugUnnamed)
	}
	if card.Build == nil {
		registrationBug(bugDashboardCard, card.Name, dashboardSubject, bugNoBuilder)
	}
	if card.Span < 1 {
		registrationBug(bugDashboardCard, card.Name, dashboardSubject, "is narrower than one column")
	}
	if card.Span > dashboardColumns(kind) {
		registrationBug(bugDashboardCard, card.Name, dashboardSubject, "is wider than its grid")
	}

	dashboardCards.mu.Lock()
	defer dashboardCards.mu.Unlock()

	for _, held := range dashboardCards.cards[kind] {
		if held.Name == card.Name {
			registrationBug(bugDashboardCard, card.Name, dashboardSubject, bugTwice)
		}
	}
	if dashboardCards.cards == nil {
		dashboardCards.cards = make(map[ObjectKind][]dashboardCard)
	}
	dashboardCards.cards[kind] = append(dashboardCards.cards[kind], card)
}

// of answers the cards of the dashboard of kind, in order.
func (registry *dashboardRegistry) of(kind ObjectKind) []dashboardCard {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	return registry.cards[kind]
}

// lookup answers the card of the dashboard of kind named name, or false.
func (registry *dashboardRegistry) lookup(kind ObjectKind, name string) (dashboardCard, bool) {
	for _, card := range registry.of(kind) {
		if card.Name == name {
			return card, true
		}
	}

	return dashboardCard{}, false
}

// dashboardColumns answers the width of the grid of a kind of object's
// dashboard: eight columns for a device, twelve for the estate and a group.
func dashboardColumns(kind ObjectKind) int {
	if kind == ObjectDevice {
		return cardGridDevice
	}

	return cardGridEstate
}

// dashboardScope is what one dashboard counts: the devices of its object and
// their open alarms. Every card of the dashboard reads the same scope, so
// the cards of one answer agree.
type dashboardScope struct {
	// object is the object's own address, with the tree view of the
	// request and no other view parameter. Every link of a card starts from
	// it.
	object Address
	// views holds the devices of the object: every device for the estate,
	// the devices of the node for a group, the device for a device.
	views []DeviceView
	// alarms holds the open alarms of those devices.
	alarms []Alarm
	// device is the device of a device dashboard. It is nil otherwise.
	device *DeviceView
}

// dashboardScopeOf answers the scope of the dashboard request asks for. It
// fails for a group no device is in and for a device the hub does not know.
func dashboardScopeOf(request *PageRequest) (dashboardScope, error) {
	address := request.Address
	snapshot := request.Snapshot

	object := Address{Kind: address.Kind, Group: address.Group, Device: address.Device, Tab: TabDashboard, View: linkView(request)}

	switch address.Kind {
	case ObjectEstate:
		return dashboardScope{object: object, views: snapshot.Views, alarms: snapshot.Alarms}, nil
	case ObjectGroup:
		views, err := GroupDevices(snapshot.Views, address.Group)
		if err != nil {
			return dashboardScope{}, err
		}

		return dashboardScope{object: object, views: views, alarms: ScopeAlarms(snapshot.Alarms, views)}, nil
	case ObjectDevice:
		device, err := FindDevice(snapshot.Views, address.Device)
		if err != nil {
			return dashboardScope{}, err
		}
		views := []DeviceView{*device}

		return dashboardScope{object: object, views: views, alarms: ScopeAlarms(snapshot.Alarms, views), device: &views[0]}, nil
	case ObjectUnspecified, ObjectTasks, ObjectAdmin:
		return dashboardScope{}, dashboardKindRefused(address.Kind)
	}

	return dashboardScope{}, dashboardKindRefused(address.Kind)
}

// dashboardKindRefused is the refusal of a dashboard asked for a kind of
// object that has none.
func dashboardKindRefused(kind ObjectKind) error {
	problem := textbuf.New().Str("The fleet object ").Quoted(kind.String()).Str(" has no dashboard.").Slice()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"Only the estate, a group node and a device have a dashboard, so there is nothing to draw.",
		"open "+fleetPath)
}

// offline reports whether the dashboard is a device's and the device is not
// connected now.
func (scope *dashboardScope) offline() bool {
	if scope.device == nil {
		return false
	}

	return !scope.device.Device.Connection.Connected
}

// tab answers the address of a tab of the scope's object, filtered by
// filter, under the request's tree view.
func (scope *dashboardScope) tab(tab, filter string) Address {
	address := scope.object
	address.Tab = tab
	address.View.Filter = filter

	return address
}

// devicesWith answers the devices tab of the scope's object filtered to the
// devices whose field holds value: the rows a count of a card stands for.
func (scope *dashboardScope) devicesWith(field, value string) string {
	return scope.tab(dashTabDevices, SetFilterTerm("", field, value)).String()
}

// names answers the names of the scope's devices, as a history question
// takes them.
func (scope *dashboardScope) names() []string {
	names := make([]string, 0, len(scope.views))
	for index := range scope.views {
		names = append(names, scope.views[index].Device.Name)
	}

	return names
}

// dashboardRange answers the chart range of the request. A request built
// without one shows the default range.
func dashboardRange(request *PageRequest) ChartRange {
	if request.Address.View.Range.named() {
		return request.Address.View.Range
	}

	return ChartRangeDefault
}

// dashboardContent builds the content of a dashboard tab: the range
// selector and every card of the object's kind, each built on its own.
func dashboardContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	scope, err := dashboardScopeOf(request)
	if err != nil {
		return nil, err
	}

	cards := dashboardCards.of(request.Address.Kind)
	offline := scope.offline()

	slots := make([]templ.Component, 0, len(cards)+1)
	placed := false
	for index := range cards {
		card := &cards[index]
		if !card.Live {
			slots = append(slots, renderDashboardCard(ctx, request, &scope, card))
			continue
		}
		if !offline {
			slots = append(slots, renderDashboardCard(ctx, request, &scope, card))
			continue
		}
		// The offline state stands where the first live card would.
		if !placed {
			slots = append(slots, dashboardOfflineSlot(request, &scope))
			placed = true
		}
	}

	return Dashboard(dashboardView(request), slots), nil
}

// dashboardView answers the frame of a dashboard: its grid width and its
// range selector, whose links keep the rest of the address.
func dashboardView(request *PageRequest) DashboardView {
	current := dashboardRange(request)

	ranges := make([]DashboardChoiceView, 0, len(ChartRanges()))
	for _, chartRange := range ChartRanges() {
		target := request.Address
		target.View.Range = chartRange
		ranges = append(ranges, DashboardChoiceView{
			Label: dashboardRangeLabel(chartRange), Href: target.String(), Current: chartRange == current,
		})
	}

	return DashboardView{Columns: dashboardColumns(request.Address.Kind), Ranges: ranges}
}

// dashboardRangeLabel answers the words of a range in the selector.
func dashboardRangeLabel(chartRange ChartRange) string {
	switch chartRange {
	case ChartRangeHour:
		return "1 hour"
	case ChartRangeDay:
		return "1 day"
	case ChartRangeWeek:
		return "1 week"
	case ChartRangeUnspecified:
		panic("BUG: fleetui: the range selector lists the invalid range")
	}

	panic("BUG: fleetui: the range selector lists a range outside its set")
}

// dashboardSlotView answers the slot of one card: the id the card's retry
// replaces, and the address of the card's own fragment.
func dashboardSlotView(request *PageRequest, name string) DashboardSlotView {
	return DashboardSlotView{
		ID:       dashboardSlotPrefix + name,
		Fragment: FragmentLink(fragmentDashboardCard, request.Address, name),
	}
}

// renderDashboardCard builds one card in its slot. A card that fails shows
// the failed state in its own place.
func renderDashboardCard(ctx context.Context, request *PageRequest, scope *dashboardScope, card *dashboardCard) templ.Component {
	slot := dashboardSlotView(request, card.Name)

	component, err := card.Build(ctx, request, scope, CardView{Title: card.Title, Span: card.Span})
	if err != nil {
		return dashboardSlot(slot, dashboardFailedCard(dashboardFailedView(request, card, slot, err)))
	}
	if component == nil {
		return dashboardSlot(slot, dashboardFailedCard(dashboardFailedView(request, card, slot, builtNothing())))
	}

	return dashboardSlot(slot, component)
}

// dashboardFailedView answers the failed state of one card: its frame and
// title, the state FailureOf names, and a retry that asks for the card
// alone. A failure the operator can simply retry offers that retry in place
// of the page's own.
func dashboardFailedView(request *PageRequest, card *dashboardCard, slot DashboardSlotView, err error) DashboardFailedView {
	hero := failureHeroView(request, err)
	if hero.Kind == HeroFailed {
		hero.Action = nil
	}

	return DashboardFailedView{
		Card:      CardView{Title: card.Title, Span: card.Span},
		Hero:      hero,
		RetryHref: request.Address.String(),
		Fragment:  slot.Fragment,
		Target:    "#" + slot.ID,
	}
}

// dashboardOfflineName is the name of the slot of the offline state.
const dashboardOfflineName = "offline"

// dashboardOfflineSlot answers the full-width state an offline device's
// dashboard shows in place of its live cards: when the device was last seen,
// and why its live values are not shown.
func dashboardOfflineSlot(request *PageRequest, scope *dashboardScope) templ.Component {
	device := scope.device
	name := device.Device.Name

	detail := "It has never connected, so it has reported nothing."
	if device.SeenKnown {
		detail = textbuf.New().
			Str("It was last seen ").Str(RelativeTime(device.SeenAgo)).
			Str(", at ").Str(dashboardClock(device.Device.Connection.LastSeen)).
			Str(". What it reports live is not shown, because it would be stale; the charts end at its last sample.").
			Slice()
	}

	view := DashboardOfflineView{
		Card: CardView{Title: "Device offline", Span: cardGridDevice},
		Hero: StateHeroView{
			Kind:     HeroOffline,
			Sentence: textbuf.New().Str(name).Str(" is offline.").Slice(),
			Detail:   detail,
			Action:   deviceAction(request, tabDeviceTasks, "Open the device's tasks", iconTasks),
		},
	}

	return dashboardSlot(dashboardSlotView(request, dashboardOfflineName), dashboardOffline(view))
}

// dashboardCardFragment answers one card of a dashboard on its own: the
// fragment a card's retry asks for.
func dashboardCardFragment(ctx context.Context, request *PageRequest, item string) (templ.Component, error) {
	card, held := dashboardCards.lookup(request.Address.Kind, item)
	if !held {
		return nil, dashboardCardUnknown(item)
	}

	scope, err := dashboardScopeOf(request)
	if err != nil {
		return nil, err
	}
	if card.Live && scope.offline() {
		return dashboardOfflineSlot(request, &scope), nil
	}

	return renderDashboardCard(ctx, request, &scope, &card), nil
}

// dashboardCardUnknown is the refusal of a card the dashboard does not have.
func dashboardCardUnknown(name string) error {
	problem := textbuf.New().Str("This dashboard has no card named ").Quoted(name).Str(".").Slice()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"The address asks for a card that does not exist, so there is nothing to draw.",
		"open the dashboard again")
}
