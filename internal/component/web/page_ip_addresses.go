// Design: plan/spec-web-4-interfaces.md -- IP Addresses page
// Related: workbench_table.go -- Reusable table component
// Related: page_interfaces.go -- Interface page (sibling)

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// AddressRow holds one row of the IP addresses table.
type AddressRow struct {
	Address       string // CIDR notation
	Network       string // network address from CIDR
	InterfaceUnit string // interface name (and unit if applicable)
	Family        string // "IPv4" or "IPv6"
}

// NetworkFromCIDR computes the network address from a CIDR string.
// For example, "10.0.0.5/24" returns "10.0.0.0/24".
// Returns the input unchanged if parsing fails.
func NetworkFromCIDR(cidr string) string {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return cidr
	}
	return prefix.Masked().String()
}

// BuildAddressTableData constructs a WorkbenchTableData from interface
// information, collecting all addresses across all interfaces.
func BuildAddressTableData(infos []iface.InterfaceInfo, filterIface, filterProtocol string) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "address", Label: "Address", Sortable: true},
		{Key: "network", Label: "Network", Sortable: true},
		{Key: "interface", Label: "Interface", Sortable: true},
		{Key: "family", Label: "Protocol", Sortable: true},
	}

	var rows []WorkbenchTableRow
	for _, info := range infos {
		if filterIface != "" && info.Name != filterIface {
			continue
		}
		for _, addr := range info.Addresses {
			cidr := fmt.Sprintf("%s/%d", addr.Address, addr.PrefixLength)
			family := addrFamily(addr)

			if filterProtocol != "" && !strings.EqualFold(family, filterProtocol) {
				continue
			}

			network := NetworkFromCIDR(cidr)

			rows = append(rows, WorkbenchTableRow{
				Key:   cidr,
				Cells: []string{cidr, network, info.Name, family},
				Actions: []WorkbenchRowAction{
					{Label: "Go to Interface", URL: fmt.Sprintf("/show/iface/detail/%s", info.Name)},
				},
			})
		}
	}

	return WorkbenchTableData{
		Title:        "IP Addresses",
		AddURL:       "/show/ip/addresses/add",
		AddLabel:     "Add Address",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No IP addresses configured.",
		EmptyHint:    "Add an address to an interface to enable L3 connectivity.",
	}
}

// addrFamily returns "IPv4" or "IPv6" based on the address info.
func addrFamily(addr iface.AddrInfo) string {
	if addr.Family == "ipv6" || strings.Contains(addr.Address, ":") {
		return "IPv6"
	}
	return "IPv4"
}

// HandleAddressesPage renders the IP addresses table content for the workbench.
func HandleAddressesPage(renderer *Renderer, r *http.Request) template.HTML {
	filterIface := r.URL.Query().Get("interface")
	filterProtocol := r.URL.Query().Get("protocol")

	infos, err := iface.ListInterfaces()
	if err != nil {
		tableData := BuildAddressTableData(nil, filterIface, filterProtocol)
		return renderer.RenderFragment("workbench_table", tableData)
	}

	tableData := BuildAddressTableData(infos, filterIface, filterProtocol)
	return renderer.RenderFragment("workbench_table", tableData)
}
