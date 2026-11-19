// Design: docs/architecture/fleet-interface.md -- the golden capture plan of the shared components

package fleetui

import (
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// goldenSpecComponents is the capture plan of the shared components: every
// component_*.templ and icons.templ. Each case renders the component with
// the data the component gallery draws it with, so a fixture is what the
// gallery shows of that component; the gallery itself is one case of
// component_gallery.templ, and its groups are the others.
func goldenSpecComponents(t *testing.T) golden.Spec {
	t.Helper()

	gallery := NewGalleryView()
	marks, controls, cards := gallery.Marks, gallery.Controls, gallery.Cards
	tables, panels, states := gallery.Tables, gallery.Panels, gallery.States
	overlays, console := gallery.Overlays, gallery.Console

	full := tables.Full
	table := full.Table
	task := overlays.Tasks.List.Rows[0]
	donut := cards.Donuts[0].Donuts[0]
	usage := cards.Usage[0]
	link := controls.Links[0]

	one := func(name string, data any) golden.Unit {
		return golden.Unit{Name: name, Variants: []golden.Variant{{Data: data}}}
	}

	return golden.Spec{
		"icons.templ": {
			one("Icon", Icon(marks.Icons[0])),
			one("StatusMark", StatusMark(marks.Statuses[0])),
			one("StatusWord", StatusWord(marks.Statuses[0])),
			one("StateMark", StateMark(marks.States[0])),
			one("StateWord", StateWord(marks.States[0])),
			one("ObjectIcon", ObjectIcon(marks.Objects[1])),
			one("Illustration", Illustration(marks.Illustrations[0])),
		},
		"component_action.templ": {
			one("NavLink", NavLink(NavLinkView{Href: galleryAdminHref, Class: "fleet-sidebar-link", Name: galleryLabels, Current: true})),
			one("actionContent", actionContent(link, actionLookLink)),
			one("actionControl", actionControl(controls.Buttons[0], actionLookButton)),
			one("ActionLink", ActionLink(link)),
			one("PanelLink", PanelLink(PanelLinkView{Href: "/fleet/estate/devices/?id=edge-lon-04", Panel: "/fleet/fragment/side-panel/estate/devices/?id=edge-lon-04", Row: "edge-lon-04", Name: "Details of edge-lon-04"})),
			one("ActionButton", ActionButton(controls.Buttons[0])),
			one("PrimaryButton", PrimaryButton(controls.Primary)),
			one("IconButton", IconButton(controls.Icon)),
			one("menuList", menuList(controls.Menu)),
			one("Menu", Menu(controls.Menu)),
			one("DotsMenu", DotsMenu(controls.Dots)),
			one("Tag", Tag(marks.Tags[0])),
			one("CopyButton", CopyButton(marks.Copy)),
		},
		"component_hero.templ": {
			one("StateHero", StateHero(states.Heroes[0])),
			one("Skeleton", Skeleton(SkeletonView{Rows: 3, Card: true, Span: 2})),
			one("StaleBanner", StaleBanner(states.Stale)),
			one("GoodNews", GoodNews(states.Good)),
			one("Callout", Callout(cards.Callout)),
		},
		"component_terminal.templ": {
			one("Terminal", Terminal(console.Terminal)),
			one("TerminalEntry", TerminalEntry(console.Entries[0])),
			one("Diff", Diff(console.Diff)),
		},
		"component_card.templ": {
			one("CardGrid", CardGrid(CardGridView{Columns: 3})),
			one("Card", Card(cards.Figures.Card)),
			one("Figure", Figure(cards.Figures.Figures[0])),
			one("FigureList", FigureList(FigureListView{Figures: cards.Figures.Figures, Columns: true})),
			one("FiguresCard", FiguresCard(cards.Figures)),
			one("ChartCard", ChartCard(cards.Charts[0])),
		},
		"component_donut.templ": {
			one("donutRing", donutRing(donut)),
			one("legendLine", legendLine(donut.Segments[0])),
			one("Donut", Donut(donut)),
			one("DonutCard", DonutCard(cards.Donuts[0])),
		},
		"component_gallery.templ": {
			one("galleryGroup", galleryGroup(galleryDevices)),
			one("galleryMarksGroup", galleryMarksGroup(marks)),
			one("galleryControlsGroup", galleryControlsGroup(controls)),
			one("galleryCardsGroup", galleryCardsGroup(cards)),
			one("galleryTablesGroup", galleryTablesGroup(tables)),
			one("galleryPanelsGroup", galleryPanelsGroup(panels)),
			one("galleryStatesGroup", galleryStatesGroup(states)),
			one("galleryOverlaysGroup", galleryOverlaysGroup(overlays)),
			one("galleryConsoleGroup", galleryConsoleGroup(console)),
			one("ComponentGallery", ComponentGallery(gallery)),
		},
		"component_panel.templ": {
			one("valueContent", valueContent(ValueView{Label: galleryAddressName, Text: "192.0.2.13"})),
			one("KeyValueList", KeyValueList(KeyValueListView{Rows: []ValueView{{Label: galleryState, Text: "Healthy"}}})),
			one("PanelContent", PanelContent(panels.Row)),
			one("QuickInfo", QuickInfo(cards.QuickInfo)),
		},
		"component_table.templ": {
			one("hiddenFields", hiddenFields([]HiddenFieldView{{Name: paramTreeView, Value: "role"}})),
			one("QueryRow", QueryRow(*full.Query)),
			one("TableActions", TableActions(*full.Bulk)),
			one("SelectionRow", SelectionRow(*full.Bulk)),
			one("pagerStep", pagerStep(pagerStepView{Href: full.Pager.NextHref, Icon: marks.Icons[0], Name: "Next page", Enabled: true})),
			one("Pager", Pager(*full.Pager)),
			one("headerCell", headerCell(table.Headers[0])),
			one("cellText", cellText(galleryCPE, "")),
			one("Cell", Cell(table.Rows[0].Cells[0])),
			one("tableRow", tableRow(table, table.Rows[0])),
			one("Table", Table(table)),
			one("sectionCreate", sectionCreate(full)),
			one("TableSection", TableSection(full)),
		},
		"component_task.templ": {
			one("taskProgress", taskProgress(task)),
			one("taskSummary", taskSummary(task)),
			one("taskEye", taskEye(task)),
			one("TaskRow", TaskRow(task)),
			one("TaskList", TaskList(overlays.NoTasks)),
			one("TaskPanel", TaskPanel(overlays.Tasks)),
			one("AlarmList", AlarmList(cards.Alarms)),
		},
		"component_toast.templ": {
			one("Toast", Toast(overlays.Toasts[0])),
			one("ToastOOB", ToastOOB(overlays.Toasts[0])),
			one("ToastArea", ToastArea()),
			one("confirmBody", confirmBody(overlays.Routine, false)),
			one("confirmDialog", confirmDialog()),
			one("ConfirmPreview", ConfirmPreview(overlays.Destructive)),
		},
		"component_usage.templ": {
			one("usageTrack", usageTrack(usage)),
			one("UsageBar", UsageBar(usage)),
			one("UsageMini", UsageMini(usage)),
			one("UsageGroup", UsageGroup(cards.Group)),
			one("StackedBar", StackedBar(cards.Stacked)),
		},
	}
}
