// Design: docs/architecture/fleet-interface.md -- the Routing tab of a device
// Related: register_device_pages.go -- registers the page and its section fragment
// Related: page_device.templ -- deviceTab, deviceSubTabs and deviceSection, which draw it
// Related: source_views.go -- DevicePeers, DeviceFamilies and DeviceAdjacencies
//
// The Routing tab holds a row of sub-tabs, BGP first. A sub-tab is shown only
// when the device answers its question: a protocol the device does not run
// has no sub-tab, rather than an empty one. A question the device's software
// does not know keeps its sub-tab, which shows that state. The sub-tab is a
// view parameter of the address, so each has an address that reproduces it.

package fleetui

import (
	"cmp"
	"context"
	"strings"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// titleRouting is the title of the Routing tab's one section.
const titleRouting = "Routing"

// The sub-tabs of the Routing tab that are always BGP's. Every other routing
// protocol the device answers for has a sub-tab keyed by its name.
const (
	subPeers     = "bgp-peers"
	subSummary   = "bgp-summary"
	subProtocols = "protocols"
)

// The titles of the BGP sub-tabs.
const (
	titlePeers     = "BGP peers"
	titleSummary   = "BGP summary"
	titleProtocols = "Other protocols"
)

// peersSubject names the rows of the peers table in a refusal.
const peersSubject = "peers"

// The names of the peer fields a filter term and a sort name, beyond the
// devices table's fieldName, fieldState and fieldGroup.
const (
	fieldAS       = "as"
	fieldSince    = "since"
	fieldReceived = "received"
	fieldSent     = "sent"
)

// The filter grammar and the sortable columns of the peers table.
// registerPeerTable fills both from init().
var (
	peerFilters = NewFilterSchema[Peer](peersSubject, peerContains, nil)
	peerColumns = NewColumnSet[Peer](peersSubject)
)

// registerPeerTable registers the fields and the sortable columns of the
// peers table. The first column, the peer, is the default sort.
func registerPeerTable() {
	peerFilters.MustRegisterField(TextField(fieldState, nil, func(row *Peer) string { return row.State }))
	peerFilters.MustRegisterField(TextField(fieldGroup, nil, func(row *Peer) string { return row.Group }))

	for _, column := range []Column[Peer]{
		{Key: fieldName, Compare: func(a, b *Peer) int { return cmp.Compare(peerKey(a), peerKey(b)) }},
		{Key: fieldGroup, Compare: func(a, b *Peer) int { return cmp.Compare(a.Group, b.Group) }},
		{Key: fieldAS, Compare: func(a, b *Peer) int { return cmp.Compare(a.RemoteAS, b.RemoteAS) }},
		{Key: fieldState, Compare: func(a, b *Peer) int { return cmp.Compare(a.State, b.State) }},
		{Key: fieldSince, Compare: func(a, b *Peer) int { return a.Since.Compare(b.Since) }},
		{Key: fieldReceived, Compare: func(a, b *Peer) int { return cmp.Compare(a.PrefixesReceived, b.PrefixesReceived) }},
		{Key: fieldSent, Compare: func(a, b *Peer) int { return cmp.Compare(a.PrefixesSent, b.PrefixesSent) }},
	} {
		peerColumns.MustRegister(column)
	}
}

// peerKey answers the name a peer's row is known by: its name, or its address
// for a peer the device names none.
func peerKey(row *Peer) string {
	if row.Name != "" {
		return row.Name
	}

	return row.Address.String()
}

// peerContains reports whether a peer's name, address, group or last error
// holds needle, which is lower case.
func peerContains(row *Peer, needle string) bool {
	for _, text := range []string{row.Name, row.Address.String(), row.Group, row.LastError} {
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}

	return false
}

// routingAnswers holds what the device answered to the Routing tab's three
// questions, each with its own failure.
type routingAnswers struct {
	peers        []Peer
	peersErr     error
	families     []FamilySummary
	familiesErr  error
	protocols    []ProtocolAdjacencies
	protocolsErr error
}

// askRouting asks the device the Routing tab's three questions.
func askRouting(ctx context.Context, request *PageRequest, name string) *routingAnswers {
	answers := &routingAnswers{}
	answers.peers, answers.peersErr = request.Source.DevicePeers(ctx, name)
	answers.families, answers.familiesErr = request.Source.DeviceFamilies(ctx, name)
	answers.protocols, answers.protocolsErr = request.Source.DeviceAdjacencies(ctx, name)

	return answers
}

// blocksDevice reports whether a failure is one of the device as a whole,
// which every question of the tab would meet: the tab then shows it once, in
// place of its sub-tabs.
func blocksDevice(err error) bool {
	switch FailureOf(err) {
	case FailureOffline, FailureRelayRefused, FailureUnauthorized, FailureNotFound:
		return true
	case FailureUnsupported, FailureDeadline, FailureFailed:
		return false
	case FailureUnspecified:
		panic("BUG: fleetui: FailureOf answered no failure")
	default:
		panic("BUG: fleetui: FailureOf answered a failure outside its set")
	}
}

// subTabs answers the keys and the titles of the sub-tabs the device's
// answers show, BGP first. A question that failed keeps its sub-tab, which
// shows the failure; a question answered with no row has none.
func (answers *routingAnswers) subTabs() (keys, titles []string) {
	if answers.peersErr != nil || len(answers.peers) > 0 {
		keys, titles = append(keys, subPeers), append(titles, titlePeers)
	}
	if answers.familiesErr != nil || len(answers.families) > 0 {
		keys, titles = append(keys, subSummary), append(titles, titleSummary)
	}
	if answers.protocolsErr != nil && FailureOf(answers.protocolsErr) != FailureUnsupported {
		keys, titles = append(keys, subProtocols), append(titles, titleProtocols)
	}
	for _, protocol := range answers.protocols[:min(len(answers.protocols), ProtocolsMax)] {
		keys, titles = append(keys, protocolKey(protocol.Protocol)), append(titles, protocol.Protocol)
	}

	return keys, titles
}

// protocolKey answers the sub-tab key of a protocol the device names.
func protocolKey(protocol string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(protocol)), " ", "-")
}

// routingContent builds the Routing tab: its sub-tabs and the section of the
// one the address names.
func routingContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	answers := askRouting(ctx, request, view.Device.Name)
	subTabs, body := routingBody(request, view, answers)

	return deviceTab(DeviceTabView{
		Label:   "Routing of " + view.Device.Name,
		SubTabs: subTabs,
		Sections: []DeviceSectionView{{
			Title: titleRouting,
			URL:   sectionLink(request, fragmentDeviceRouting),
			Wide:  true,
			Body:  body,
		}},
	}), nil
}

// routingFragment answers the section of the Routing tab.
func routingFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	_, body := routingBody(request, view, askRouting(ctx, request, view.Device.Name))

	return body, nil
}

// routingBody answers the row of sub-tabs, absent when the device answered
// none, and the section's content for the sub-tab the address names.
func routingBody(request *PageRequest, view *DeviceView, answers *routingAnswers) (*DeviceSubTabsView, templ.Component) {
	if answers.peersErr != nil && blocksDevice(answers.peersErr) {
		return nil, sectionState(request, titleRouting, answers.peersErr)
	}

	keys, titles := answers.subTabs()
	if len(keys) == 0 {
		return nil, sectionEmpty(titleRouting, view.Device.Name+" runs no routing protocol the hub can show.")
	}

	current := request.Address.View.Sub
	if current == "" {
		current = keys[0]
	}
	subTabs := subTabsView(request.Address, "Routing views", keys, titles, current)

	switch current {
	case subPeers:
		if answers.peersErr != nil {
			return subTabs, sectionState(request, titlePeers, answers.peersErr)
		}
		return subTabs, peersTable(request, view, answers.peers)
	case subSummary:
		if answers.familiesErr != nil {
			return subTabs, sectionState(request, titleSummary, answers.familiesErr)
		}
		return subTabs, familiesTable(view, answers.families)
	case subProtocols:
		if answers.protocolsErr != nil {
			return subTabs, sectionState(request, titleProtocols, answers.protocolsErr)
		}
	}

	for index := range answers.protocols {
		protocol := &answers.protocols[index]
		if protocolKey(protocol.Protocol) == current {
			return subTabs, adjacencyTable(request, view, protocol)
		}
	}

	return subTabs, sectionState(request, titleRouting, subTabUnknown("Routing", current, keys))
}

// peerStatus answers how a peer's session state is drawn.
func peerStatus(row *Peer) Status {
	switch {
	case row.State == "":
		return Status{Tone: ToneNeutral, Word: enumWordUnspecified}
	case row.Established:
		return Status{Tone: ToneSuccess, Word: row.State}
	default:
		return Status{Tone: ToneWarning, Word: row.State}
	}
}

// peersTable builds the BGP peers table, which filters, sorts and pages
// through its address.
func peersTable(request *PageRequest, view *DeviceView, peers []Peer) templ.Component {
	peers = peers[:min(len(peers), PeersMax)]
	address := request.Address
	now := request.Snapshot.Now

	query := &QueryRowView{
		ID:          "fleet-peers-query",
		Action:      address.Path(),
		Text:        address.View.Filter,
		Placeholder: "Filter: a peer, an address, state:idle, group:transit",
		Hidden:      carriedFields(address, paramFilter, paramPage, paramRow),
	}

	matched := peers
	filter, err := peerFilters.Parse(address.View.Filter, nil)
	if err == nil {
		matched = filter.Apply(peers)
	} else {
		query.Refusal = refusalOf(err).Problem()
	}

	sorted, err := peerColumns.Sort(matched, address.View.Sort)
	if err != nil {
		sorted = matched
		query.Refusal = refusalOf(err).Problem()
	}

	page := Paginate(len(sorted), address.View.Page, address.View.Size)
	query.Matches = matchesText(len(sorted), len(peers), peersSubject)

	shown := PageRows(sorted, page)
	rows := make([]RowView, 0, len(shown))
	for index := range shown {
		row := &shown[index]
		key := peerKey(row)
		rows = append(rows, RowView{
			ID:      key,
			Name:    key,
			EyeHref: openRowAddress(address, key),
			Opened:  address.View.Row == key,
			Cells: []CellView{
				MonoCell(key),
				TextCell(row.Group),
				NumberCell(textbuf.StringUint32(row.RemoteAS)),
				StatusCell(peerStatus(row)),
				sinceCell(now, row.Since),
				NumberCell(uintText(row.PrefixesReceived)),
				NumberCell(uintText(row.PrefixesSent)),
				TextCell(row.LastError),
			},
		})
	}

	return TableSection(TableSectionView{
		Title: titlePeers,
		Query: query,
		Pager: pagerView(address, peersSubject, page),
		Table: TableView{
			Name: "BGP peers of " + view.Device.Name,
			Headers: []ColumnHeaderView{
				sortHeader(address, "Peer", "kind-address", fieldName, false),
				sortHeader(address, "Group", "kind-text", fieldGroup, false),
				sortHeader(address, "Remote AS", "kind-number", fieldAS, true),
				sortHeader(address, "State", "kind-state", fieldState, false),
				sortHeader(address, "Since", "kind-time", fieldSince, false),
				sortHeader(address, "Received", "kind-number", fieldReceived, true),
				sortHeader(address, "Sent", "kind-number", fieldSent, true),
				{Label: "Last error", Kind: "kind-text"},
			},
			Rows:     rows,
			Empty:    &StateHeroView{Kind: HeroNoData, Sentence: "No peer matches the filter."},
			Controls: true,
		},
	})
}

// familiesTable builds the BGP summary: one row per address family.
func familiesTable(view *DeviceView, families []FamilySummary) templ.Component {
	families = families[:min(len(families), FamiliesMax)]

	rows := make([]RowView, 0, len(families))
	for _, family := range families {
		rows = append(rows, RowView{
			ID:   family.Family,
			Name: family.Family,
			Cells: []CellView{
				MonoCell(family.Family),
				NumberCell(textbuf.StringInt(int64(family.PeersEstablished))),
				NumberCell(uintText(family.Routes)),
				NumberCell(uintText(family.BestRoutes)),
			},
		})
	}

	return TableSection(TableSectionView{
		Title: titleSummary,
		Table: TableView{
			Name: "BGP summary of " + view.Device.Name,
			Headers: []ColumnHeaderView{
				{Label: "Family", Kind: "kind-text"},
				{Label: "Peers established", Kind: "kind-number", Numeric: true},
				{Label: "Routes", Kind: "kind-number", Numeric: true},
				{Label: "Best routes", Kind: "kind-number", Numeric: true},
			},
			Rows: rows,
		},
	})
}

// adjacencyTable builds the table of one other routing protocol: its
// adjacencies or neighbors with their state.
func adjacencyTable(request *PageRequest, view *DeviceView, protocol *ProtocolAdjacencies) templ.Component {
	adjacencies := protocol.Rows[:min(len(protocol.Rows), AdjacenciesMax)]
	now := request.Snapshot.Now

	rows := make([]RowView, 0, len(adjacencies))
	for _, adjacency := range adjacencies {
		rows = append(rows, RowView{
			ID:   adjacency.Neighbor,
			Name: adjacency.Neighbor,
			Cells: []CellView{
				MonoCell(adjacency.Neighbor),
				MonoCell(adjacency.Interface),
				StatusCell(AdjacencyStatus(&adjacency)),
				sinceCell(now, adjacency.Since),
			},
		})
	}

	return TableSection(TableSectionView{
		Title: protocol.Protocol,
		Table: TableView{
			Name: protocol.Protocol + " of " + view.Device.Name,
			Headers: []ColumnHeaderView{
				{Label: "Neighbor", Kind: "kind-address"},
				{Label: "Interface", Kind: "kind-text"},
				{Label: "State", Kind: "kind-state"},
				{Label: "Since", Kind: "kind-time"},
			},
			Rows: rows,
			Empty: &StateHeroView{
				Kind: HeroNoData, Sentence: view.Device.Name + " runs " + protocol.Protocol + " with no adjacency up.",
			},
		},
	})
}

// peerPanel builds the side panel of the BGP peer the address names: its
// session and its prefix counts, as far as the device's answer carries them.
func peerPanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	if sub := request.Address.View.Sub; sub != "" && sub != subPeers {
		return panelEmpty(), nil
	}

	peers, err := request.Source.DevicePeers(ctx, view.Device.Name)
	if err != nil {
		return nil, err
	}

	name := request.Address.View.Row
	for index := range peers {
		row := &peers[index]
		if peerKey(row) != name {
			continue
		}

		since := ""
		if !row.Since.IsZero() {
			since = RelativeTime(request.Snapshot.Now.Sub(row.Since)) + " (" + exactTime(row.Since) + ")"
		}

		return PanelContent(PanelView{
			Title:     name,
			CloseHref: closedAddress(request.Address),
			Icon:      ObjectIconView{Icon: iconDevice, Status: peerStatus(row), Marked: true},
			Cards: []PanelCardView{
				{Title: "Session", Rows: []ValueView{
					{Label: "Address", Text: row.Address.String(), Mono: true, Copy: true},
					{Label: "Group", Text: row.Group},
					{Label: "Remote AS", Text: textbuf.StringUint32(row.RemoteAS), Mono: true},
					{Label: "State", Status: peerStatus(row)},
					{Label: "Since", Text: since},
					{Label: "Last error", Text: row.LastError},
				}},
				{Title: "Prefixes", Rows: []ValueView{
					{Label: "Received", Text: uintText(row.PrefixesReceived)},
					{Label: "Sent", Text: uintText(row.PrefixesSent)},
				}},
			},
		}), nil
	}

	problem := textbuf.New().Str(view.Device.Name).Str(" has no BGP peer named ").Quoted(name).Str(".").String()

	return nil, opererr.Wrap(ErrObjectUnknown, problem,
		"The address names a peer the device did not report, so there is nothing to show.",
		"open "+closedAddress(request.Address))
}
