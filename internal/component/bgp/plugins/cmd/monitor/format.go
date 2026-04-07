// Design: docs/architecture/api/commands.md — BGP monitor visual text formatting
// Overview: doc.go — package doc + YANG import
// Related: monitor.go — monitor command handler

package monitor

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxDisplayPrefixes = 5

// FormatMonitorLine renders a JSON event into a visual one-liner for terminal display.
// Returns the raw string unchanged if JSON parsing fails.
func FormatMonitorLine(raw string) string {
	var ev monitorEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return raw
	}

	dir := formatDirection(ev.BGP.Message.Direction)
	peer := ev.BGP.Peer.Address
	asn := fmt.Sprintf("AS%d", ev.BGP.Peer.Remote.AS)

	switch ev.BGP.Message.Type {
	case "update":
		return formatUpdate(dir, peer, asn, ev)
	case "state":
		return formatState(peer, asn, ev)
	case "keepalive":
		return fmt.Sprintf("%s KALIVE %s %s", dir, peer, asn)
	case "eor":
		fam := ev.BGP.EOR.Family
		return fmt.Sprintf("---- EOR    %s %s %s", peer, asn, fam)
	case "open":
		return formatOpen(dir, peer, asn, ev)
	case "notification":
		return formatNotification(dir, peer, asn, ev)
	case "refresh":
		return fmt.Sprintf("%s RFRSH  %s %s", dir, peer, asn)
	case "negotiated":
		return fmt.Sprintf("---- NEGOT  %s %s", peer, asn)
	}

	return raw
}

func formatDirection(dir string) string {
	switch dir {
	case "received":
		return "recv"
	case "sent":
		return "sent"
	}
	return "----"
}

func formatUpdate(dir, peer, asn string, ev monitorEvent) string {
	var parts []string

	for _, fam := range knownFamilies {
		entries, ok := ev.BGP.Update.NLRI[fam]
		if !ok {
			continue
		}
		for _, entry := range entries {
			prefix := "+"
			if entry.Action == "del" {
				prefix = "-"
			}

			nlris := entry.NLRI
			truncated := 0
			if len(nlris) > maxDisplayPrefixes {
				truncated = len(nlris) - maxDisplayPrefixes
				nlris = nlris[:maxDisplayPrefixes]
			}

			for _, n := range nlris {
				parts = append(parts, prefix+n)
			}
			if truncated > 0 {
				parts = append(parts, fmt.Sprintf("(+%d more)", truncated))
			}

			if entry.NextHop != "" {
				parts = append(parts, "nhop="+entry.NextHop)
			}
		}
	}

	return fmt.Sprintf("%s UPDATE %s %s %s", dir, peer, asn, strings.Join(parts, " "))
}

func formatState(peer, asn string, ev monitorEvent) string {
	state := ev.BGP.State
	if ev.BGP.Reason != "" {
		state += " (" + ev.BGP.Reason + ")"
	}
	return fmt.Sprintf("---- STATE  %s %s %s", peer, asn, state)
}

func formatOpen(dir, peer, asn string, ev monitorEvent) string {
	hold := ev.BGP.Open.Timer.HoldTime
	id := ev.BGP.Open.RouterID
	return fmt.Sprintf("%s OPEN   %s %s hold=%d id=%s", dir, peer, asn, hold, id)
}

func formatNotification(dir, peer, asn string, ev monitorEvent) string {
	code := ev.BGP.Notification.Code
	subcode := ev.BGP.Notification.Subcode
	return fmt.Sprintf("%s NOTIF  %s %s %d/%d", dir, peer, asn, code, subcode)
}

// knownFamilies lists address families to scan in JSON events.
var knownFamilies = []string{
	"ipv4/unicast", "ipv6/unicast",
	"ipv4/mpls-vpn", "ipv6/mpls-vpn",
	"ipv4/flow", "ipv6/flow",
	"l2vpn/evpn", "bgp-ls/bgp-ls",
}

// monitorEvent is the minimal structure for parsing ze-bgp JSON events.
// Only fields needed for text rendering are included.
// All detail fields (state, reason, eor, open, notification, update.nlri)
// are inside the "bgp" wrapper, matching production ze-bgp JSON format.
type monitorEvent struct {
	BGP struct {
		Peer struct {
			Address string `json:"address"`
			Remote  struct {
				AS uint32 `json:"as"`
			} `json:"remote"`
		} `json:"peer"`
		Message struct {
			Direction string `json:"direction"`
			Type      string `json:"type"`
		} `json:"message"`

		State  string `json:"state,omitempty"`
		Reason string `json:"reason,omitempty"`

		EOR struct {
			Family string `json:"family"`
		} `json:"eor"`

		Open struct {
			Timer struct {
				HoldTime int `json:"hold-time"`
			} `json:"timer"`
			RouterID string `json:"router-id"`
		} `json:"open"`

		Notification struct {
			Code    int `json:"code"`
			Subcode int `json:"subcode"`
		} `json:"notification"`

		Update struct {
			NLRI map[string][]nlriEntry `json:"nlri"`
		} `json:"update"`
	} `json:"bgp"`
}

type nlriEntry struct {
	Action  string   `json:"action"`
	NLRI    []string `json:"nlri"`
	NextHop string   `json:"next-hop,omitempty"`
}
