// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- VPN IPsec web page
// Related: page_l2tp.go -- Table page pattern
// Related: page_interfaces.go -- HTMX auto-refresh partial pattern
// Related: page_vpn_ipsec_off.go -- ze_ike-off stub counterpart

//go:build ze_ike

package web

import (
	"html/template"
	"net/http"
	"time"

	"github.com/ze-software/ze/internal/component/ike/engine"
)

func buildIPsecSATableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "peer-name", Label: labelPeer, Sortable: true},
		{Key: colState, Label: labelState, Sortable: true},
		{Key: "initiator-spi", Label: "Initiator SPI"},
		{Key: "responder-spi", Label: "Responder SPI"},
		{Key: "encryption", Label: "Encryption"},
		{Key: "uptime", Label: labelUptime, Sortable: true},
	}

	table := engine.ActiveTable()
	if table == nil {
		return WorkbenchTableData{
			Title:        "IPsec Security Associations",
			Columns:      columns,
			Rows:         nil,
			EmptyMessage: "IKE engine is not running.",
			EmptyHint:    "Enable the IPsec VPN subsystem in the configuration.",
		}
	}

	sas := table.All()
	now := time.Now()

	rows := make([]WorkbenchTableRow, 0, len(sas))
	for _, sa := range sas {
		iSPI := engine.SPIHex(sa.InitiatorSPI)
		rSPI := engine.SPIHex(sa.ResponderSPI)
		encr := sa.Proposal.Encryption.ID.String()
		uptime := ""
		if sa.State == engine.StateEstablished && !sa.EstablishedAt.IsZero() {
			uptime = formatDuration(now.Sub(sa.EstablishedAt))
		}

		rows = append(rows, WorkbenchTableRow{
			Key: iSPI,
			Cells: []string{
				sa.PeerName,
				sa.State.String(),
				iSPI,
				rSPI,
				encr,
				uptime,
			},
		})
	}

	return WorkbenchTableData{
		Title:        "IPsec Security Associations",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No active IPsec Security Associations.",
		EmptyHint:    "Configure VPN peers to establish IPsec tunnels.",
	}
}

// handleIPsecPage renders the VPN IPsec SA table page.
func handleIPsecPage(renderer *Renderer) template.HTML {
	tableData := buildIPsecSATableData()
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

func renderVPNPageContent(renderer *Renderer, _ *http.Request, path []string) (template.HTML, bool) {
	if len(path) == 0 {
		return "", false
	}
	if path[0] == "ipsec" {
		return handleIPsecPage(renderer), true
	}
	return "", false
}
