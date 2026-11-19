// Design: docs/architecture/fleet-interface.md -- the header search
// Related: shell_view.go -- SearchResultsView
// Related: handler_fragment.go -- the fragment registry this search answers through
//
// The header search finds devices and pages for typed text. It reads the
// snapshot of its request and the registered tabs, so it asks no device and
// costs the same whatever the devices do.

package fleetui

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/a-h/templ"
)

// searchResultsMax is the most results the header search answers. The first
// one is what the enter key opens.
const searchResultsMax = 10

// searchFragment answers what the header search found for the text the
// fragment address carries as its item.
func searchFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	return searchResults(searchView(request, item)), nil
}

// searchView finds the devices, then the pages, that match text, at most
// searchResultsMax of them. A device matches by its name or by a label
// value, the devices matching by name first. A page matches by its title.
// Empty text finds nothing.
func searchView(request *PageRequest, text string) SearchResultsView {
	view := SearchResultsView{Text: strings.TrimSpace(text)}

	needle := strings.ToLower(view.Text)
	if needle == "" {
		return view
	}

	view.Results = append(view.Results, searchDevices(request, needle)...)
	view.Results = append(view.Results, searchPages(request, needle)...)
	if len(view.Results) > searchResultsMax {
		view.Results = view.Results[:searchResultsMax]
	}

	return view
}

// searchDevices answers the devices whose name or a label value holds
// needle, which MUST be lower case: those whose name holds it first, each
// set in name order, at most searchResultsMax.
func searchDevices(request *PageRequest, needle string) []SearchResultView {
	kept := FilterTree(request.Snapshot.Views, needle)

	// A device view is large, so the matches are ordered through pointers.
	matches := make([]*DeviceView, 0, len(kept))
	for index := range kept {
		matches = append(matches, &kept[index])
	}

	named := func(view *DeviceView) bool {
		return strings.Contains(strings.ToLower(view.Device.Name), needle)
	}
	slices.SortStableFunc(matches, func(a, b *DeviceView) int {
		aNamed, bNamed := named(a), named(b)
		if aNamed != bNamed {
			if aNamed {
				return -1
			}

			return 1
		}

		return cmp.Compare(a.Device.Name, b.Device.Name)
	})

	if len(matches) > searchResultsMax {
		matches = matches[:searchResultsMax]
	}

	results := make([]SearchResultView, 0, len(matches))
	for _, match := range matches {
		results = append(results, SearchResultView{
			Title:  match.Device.Name,
			Detail: "Device, " + match.State.String(),
			Href:   deviceAddress(request, match.Device.Name).String(),
			Icon:   iconDevice,
		})
	}

	return results
}

// searchPages answers the pages whose title holds needle, which MUST be
// lower case: the tabs of the estate, the task list and the administration
// pages, in that order.
func searchPages(request *PageRequest, needle string) []SearchResultView {
	var results []SearchResultView

	holds := func(title string) bool {
		return strings.Contains(strings.ToLower(title), needle)
	}

	for _, tab := range Tabs(ObjectEstate) {
		if !holds(tab.Title) {
			continue
		}
		target := Address{Kind: ObjectEstate, Tab: tab.Name, View: linkView(request)}
		results = append(results, SearchResultView{
			Title: tab.Title, Detail: "Estate page", Href: target.String(), Icon: iconEstate,
		})
	}

	if holds(tasksTitle) {
		target := Address{Kind: ObjectTasks, View: linkView(request)}
		results = append(results, SearchResultView{
			Title: tasksTitle, Detail: "Every task", Href: target.String(), Icon: iconTasks,
		})
	}

	for _, tab := range Tabs(ObjectAdmin) {
		if !holds(tab.Title) {
			continue
		}
		target := Address{Kind: ObjectAdmin, Tab: tab.Name, View: linkView(request)}
		results = append(results, SearchResultView{
			Title: tab.Title, Detail: adminTitle + " page", Href: target.String(), Icon: iconAdmin,
		})
	}

	return results
}
