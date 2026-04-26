// Design: plan/spec-web-4-interfaces.md -- IP Routes page
// Related: workbench_table.go -- Reusable table component
// Related: page_ip_addresses.go -- IP Addresses page (sibling)

package web

import (
	"fmt"
	"html/template"
	"net/http"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// routeDisplayLimit caps the number of routes shown in the web UI to prevent
// unbounded memory usage on full-DFZ boxes.
const routeDisplayLimit = 1000

// routeFlag computes the flag string for a route row.
// A = active (always for kernel routes), S = static, B = bgp,
// C = connected (kernel), D = dynamic (non-static).
func routeFlag(route iface.KernelRoute) (string, string) {
	switch route.Protocol {
	case "static":
		return "A S", ""
	case "bgp":
		return "A B", ""
	case "kernel":
		return "A C", flagClassGreen
	default:
		return "A D", ""
	}
}

// BuildRouteTableData constructs a WorkbenchTableData from kernel routes.
func BuildRouteTableData(routes []iface.KernelRoute, filterProtocol string) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "destination", Label: "Destination", Sortable: true},
		{Key: "gateway", Label: "Gateway", Sortable: true},
		{Key: "metric", Label: "Metric", Sortable: true},
		{Key: "protocol", Label: "Protocol", Sortable: true},
		{Key: "device", Label: "Interface", Sortable: true},
		{Key: "family", Label: "Family", Sortable: true},
	}

	var rows []WorkbenchTableRow
	for _, route := range routes {
		if filterProtocol != "" && route.Protocol != filterProtocol {
			continue
		}

		flags, flagClass := routeFlag(route)

		gw := route.NextHop
		if gw == "" {
			gw = "-"
		}
		dev := route.Device
		if dev == "" {
			dev = "-"
		}

		rows = append(rows, WorkbenchTableRow{
			Key:       route.Destination,
			Flags:     flags,
			FlagClass: flagClass,
			Cells: []string{
				route.Destination,
				gw,
				fmt.Sprintf("%d", route.Metric),
				route.Protocol,
				dev,
				route.Family,
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Routes",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No routes found.",
		EmptyHint:    "Connected routes appear automatically when interfaces have addresses.",
	}
}

// HandleRoutesPage renders the IP routes table content for the workbench.
func HandleRoutesPage(renderer *Renderer, r *http.Request) template.HTML {
	filterProtocol := r.URL.Query().Get("protocol")
	filterPrefix := r.URL.Query().Get("prefix")

	routes, err := iface.ListKernelRoutes(filterPrefix, routeDisplayLimit)
	if err != nil {
		tableData := BuildRouteTableData(nil, filterProtocol)
		return renderer.RenderFragment("workbench_table", tableData)
	}

	tableData := BuildRouteTableData(routes, filterProtocol)
	return renderer.RenderFragment("workbench_table", tableData)
}
