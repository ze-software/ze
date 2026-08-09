// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- `show rsvp-te ...` data builders
// RFC: rfc/short/rfc4090.md
// Related: register.go -- OnExecuteCommand dispatches these
// Related: cmd_show.go -- the show RPC proxies that front them
// Related: frr.go -- the RFC 4090 protection state showFastReroute reports
//
// These build the JSON payloads returned for the `show rsvp-te ...` commands.
// They read live LSP/admission state under each LSP's lock. Kept out of
// register.go so that file stays focused on registration and the config pipeline.
package rsvpte

func showSessions(lspTable *lspTable) any {
	type sessionInfo struct {
		TunnelEndpoint string   `json:"tunnel-endpoint"`
		TunnelID       uint16   `json:"tunnel-id"`
		LSPID          uint16   `json:"lsp-id"`
		SenderAddr     string   `json:"sender-address"`
		State          string   `json:"state"`
		Role           string   `json:"role"`
		Bandwidth      float32  `json:"bandwidth"`
		InLabel        uint32   `json:"in-label"`
		OutLabel       uint32   `json:"out-label"`
		ERO            []string `json:"ero,omitempty"`
		RRO            []string `json:"rro,omitempty"`
	}
	lsps := lspTable.All()
	out := make([]sessionInfo, 0, len(lsps))
	for _, lsp := range lsps {
		lsp.mu.Lock()
		info := sessionInfo{
			TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
			TunnelID:       lsp.Key.TunnelID,
			LSPID:          lsp.Key.LSPID,
			SenderAddr:     lsp.Key.SenderAddr.String(),
			State:          lsp.State.String(),
			Role:           lsp.Role.String(),
			Bandwidth:      lsp.Bandwidth,
			InLabel:        lsp.InLabel,
			OutLabel:       lsp.OutLabel,
		}
		if lsp.PSB != nil {
			info.ERO = formatERO(lsp.PSB.ERO)
		}
		if lsp.RSB != nil {
			info.RRO = formatRRO(lsp.RSB.RRO)
		}
		out = append(out, info)
		lsp.mu.Unlock()
	}
	return out
}

func showInterfaces(admission *admissionController) any {
	type ifaceInfo struct {
		Name              string  `json:"name"`
		MaxBandwidth      float64 `json:"max-bandwidth"`
		MaxReservable     float64 `json:"max-reservable"`
		ReservedBandwidth float64 `json:"reserved-bandwidth"`
		Available         float64 `json:"available-bandwidth"`
	}
	ifaces := admission.allInterfaces()
	out := make([]ifaceInfo, 0, len(ifaces))
	for name, ib := range ifaces {
		out = append(out, ifaceInfo{
			Name:              name,
			MaxBandwidth:      ib.MaxBandwidth,
			MaxReservable:     ib.MaxReservable,
			ReservedBandwidth: ib.ReservedBandwidth,
			Available:         ib.Available(),
		})
	}
	return out
}

func showTunnels(lspTable *lspTable) any {
	type tunnelInfo struct {
		TunnelEndpoint string  `json:"tunnel-endpoint"`
		TunnelID       uint16  `json:"tunnel-id"`
		State          string  `json:"state"`
		Bandwidth      float32 `json:"bandwidth"`
		EROHops        int     `json:"ero-hops"`
	}
	lsps := lspTable.All()
	out := make([]tunnelInfo, 0, len(lsps))
	for _, lsp := range lsps {
		lsp.mu.Lock()
		if lsp.Role != RoleIngress {
			lsp.mu.Unlock()
			continue
		}
		eroHops := 0
		if lsp.PSB != nil {
			eroHops = len(lsp.PSB.ERO)
		}
		info := tunnelInfo{
			TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
			TunnelID:       lsp.Key.TunnelID,
			State:          lsp.State.String(),
			Bandwidth:      lsp.Bandwidth,
			EROHops:        eroHops,
		}
		lsp.mu.Unlock()
		out = append(out, info)
	}
	return out
}

// showFastReroute reports RFC 4090 protection state: each configured bypass LSP
// and each protected transit LSP with the bypass it is armed with and whether
// local protection is available / in use.
func showFastReroute(lspTable *lspTable) any {
	type frrInfo struct {
		TunnelEndpoint string `json:"tunnel-endpoint"`
		TunnelID       uint16 `json:"tunnel-id"`
		LSPID          uint16 `json:"lsp-id"`
		Kind           string `json:"kind"`                 // "protected" or "bypass"
		State          string `json:"state"`                // bypass LSP state
		Mode           string `json:"mode,omitempty"`       // "facility" or "one-to-one"
		NodeProtection bool   `json:"node-protection"`      // protecting against the next node
		Available      bool   `json:"protection-available"` // a backup is armed
		InUse          bool   `json:"protection-in-use"`    // traffic is on the backup
		Bypass         string `json:"bypass,omitempty"`     // the bypass LSP protecting this LSP
	}
	lsps := lspTable.All()
	out := make([]frrInfo, 0, len(lsps))
	for _, lsp := range lsps {
		lsp.mu.Lock()
		switch {
		case lsp.IsBypass:
			out = append(out, frrInfo{
				TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
				TunnelID:       lsp.Key.TunnelID,
				LSPID:          lsp.Key.LSPID,
				Kind:           "bypass",
				State:          lsp.State.String(),
			})
		case lsp.Bypass != nil:
			info := frrInfo{
				TunnelEndpoint: lsp.Key.TunnelEndpoint.String(),
				TunnelID:       lsp.Key.TunnelID,
				LSPID:          lsp.Key.LSPID,
				Kind:           "protected",
				State:          lsp.State.String(),
				Mode:           "facility",
				Available:      true,
				InUse:          lsp.ProtectionInUse,
				Bypass:         lsp.Bypass.String(),
			}
			if lsp.PSB != nil && lsp.PSB.Protection != nil {
				if !lsp.PSB.Protection.Facility {
					info.Mode = "one-to-one"
				}
				info.NodeProtection = lsp.PSB.Protection.NodeProtection
			}
			out = append(out, info)
		}
		lsp.mu.Unlock()
	}
	return out
}
