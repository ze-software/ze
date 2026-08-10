// Design: docs/architecture/api/commands.md — BGP monitor visual text formatting
// Overview: doc.go — package doc + YANG import
// Related: monitor.go — monitor command handler

package monitor

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const maxDisplayPrefixes = 5

// formatMonitorLine renders a JSON event into a visual one-liner for terminal display.
// Returns the raw string unchanged if JSON parsing fails.
func formatMonitorLine(raw string) string {
	var ev monitorEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return raw
	}

	dir := formatDirection(ev.BGP.Message.Direction)
	peer := ev.BGP.Peer.Remote.Address
	asn := textbuf.StrInt("AS", int64(ev.BGP.Peer.Remote.AS))

	switch ev.BGP.Message.Type {
	case "update":
		return formatUpdate(dir, peer, asn, ev)
	case "state":
		return formatState(peer, asn, ev)
	case "keepalive":
		var tb textbuf.Buffer
		return tb.Str(dir).Str(" KALIVE ").Str(peer).Byte(' ').Str(asn).String()
	case "eor":
		fam := ev.BGP.EOR.Family
		var tb textbuf.Buffer
		return tb.Str("---- EOR    ").Str(peer).Byte(' ').Str(asn).Byte(' ').Str(fam).String()
	case "open":
		return formatOpen(dir, peer, asn, ev)
	case "notification":
		return formatNotification(dir, peer, asn, ev)
	case "refresh":
		var tb textbuf.Buffer
		return tb.Str(dir).Str(" RFRSH  ").Str(peer).Byte(' ').Str(asn).String()
	case "negotiated":
		var tb textbuf.Buffer
		return tb.Str("---- NEGOT  ").Str(peer).Byte(' ').Str(asn).String()
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

			var tb textbuf.Buffer
			for _, n := range nlris {
				parts = append(parts, tb.Reset().Str(prefix).Str(n).String())
			}
			if truncated > 0 {
				parts = append(parts, textbuf.StrIntStr("(+", int64(truncated), " more)"))
			}

			if entry.NextHop != "" {
				parts = append(parts, tb.Reset().Str("nhop=").Str(entry.NextHop).String())
			}
		}
	}

	var tb textbuf.Buffer
	return tb.Str(dir).Str(" UPDATE ").Str(peer).Byte(' ').Str(asn).Byte(' ').Join(parts, " ").String()
}

func formatState(peer, asn string, ev monitorEvent) string {
	var tb textbuf.Buffer
	tb.Str("---- STATE  ").Str(peer).Byte(' ').Str(asn).Byte(' ').Str(ev.BGP.State)
	if ev.BGP.Reason != "" {
		tb.Str(" (").Str(ev.BGP.Reason).Byte(')')
	}
	return tb.String()
}

func formatOpen(dir, peer, asn string, ev monitorEvent) string {
	hold := ev.BGP.Open.Timer.HoldTime
	id := ev.BGP.Open.RouterID
	var b textbuf.Buffer
	return b.Reset().Str(dir).Str(" OPEN   ").Str(peer).Byte(' ').Str(asn).Str(" hold=").Int(int64(hold)).Str(" id=").Str(id).String()
}

func formatNotification(dir, peer, asn string, ev monitorEvent) string {
	code := ev.BGP.Notification.Code
	subcode := ev.BGP.Notification.Subcode
	var b textbuf.Buffer
	return b.Reset().Str(dir).Str(" NOTIF  ").Str(peer).Byte(' ').Str(asn).Byte(' ').Int(int64(code)).Byte('/').Int(int64(subcode)).String()
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
			Remote struct {
				Address string `json:"address"`
				AS      uint32 `json:"as"`
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
