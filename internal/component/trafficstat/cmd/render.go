// Design: docs/architecture/traffic/traffic-usage-monitor.md -- full-screen traffic monitor renderer

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	trafHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	trafLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	trafValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	trafCaution = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	trafDanger  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	trafDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	trafFooter  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

type trafficView struct {
	mu   sync.Mutex
	snap *viewSnapshot
	name string
}

type viewSnapshot struct {
	At         string       `json:"at"`
	Severity   string       `json:"severity"`
	Degraded   bool         `json:"degraded"`
	Interfaces []viewIface  `json:"interfaces"`
	TopSrcIPs  []viewTalker `json:"top-source-ips"`
	TopDstIPs  []viewTalker `json:"top-dest-ips"`
	TopPorts   []viewPort   `json:"top-ports"`
	Protocols  []viewProto  `json:"protocol-mix"`
	History    []float64    `json:"history"`
}

type viewIface struct {
	Name  string  `json:"name"`
	RxBps float64 `json:"rx-bps"`
	TxBps float64 `json:"tx-bps"`
	RxPps float64 `json:"rx-pps"`
	TxPps float64 `json:"tx-pps"`
}

type viewTalker struct {
	Address string  `json:"address"`
	Bps     float64 `json:"bps"`
}

type viewPort struct {
	Port          uint16  `json:"port"`
	Service       string  `json:"service"`
	Proto         uint8   `json:"proto"`
	Bps           float64 `json:"bps"`
	Amplification string  `json:"amplification,omitempty"`
}

type viewProto struct {
	Proto   uint8   `json:"proto"`
	Name    string  `json:"name"`
	Bps     float64 `json:"bps"`
	Percent float64 `json:"percent"`
}

func createTrafficMonitorSession(ctx context.Context, args []string) (eventCh <-chan string, renderFn func(w, h int) string, cancel func(), err error) {
	svc := trafficstat.EnsureGlobal()
	if svc == nil {
		return nil, nil, nil, errors.New("trafficstat service not available")
	}

	var filterName string
	for i, arg := range args {
		if arg == "name" && i+1 < len(args) {
			filterName = args[i+1]
			break
		}
	}

	id := svc.Attach()
	view := newTrafficView(filterName)
	ch := make(chan string, 16)

	childCtx, childCancel := context.WithCancel(ctx)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				snap := svc.Snapshot()
				if snap == nil {
					continue
				}
				data := snapshotJSON(snap, filterName)
				view.update(data)
				select {
				case ch <- data:
				default:
				}
			}
		}
	}()

	cancelFn := func() {
		childCancel()
		svc.Detach(id)
	}
	return ch, view.render, cancelFn, nil
}

func snapshotJSON(snap *trafficstat.Snapshot, filterName string) string {
	m := snapshotToMap(snap, filterName)
	b, _ := json.Marshal(m)
	return string(b)
}

func newTrafficView(filterName string) *trafficView {
	return &trafficView{name: filterName}
}

func (v *trafficView) update(event string) {
	var snap viewSnapshot
	if err := json.Unmarshal([]byte(event), &snap); err != nil {
		return
	}
	v.mu.Lock()
	v.snap = &snap
	v.mu.Unlock()
}

func (v *trafficView) render(width, _ int) string {
	if width <= 0 {
		width = 80
	}

	v.mu.Lock()
	snap := v.snap
	v.mu.Unlock()

	var sb textbuf.Buffer
	sb.Reset(2048)

	var hb textbuf.Buffer
	hb.Str("Traffic Monitor")
	if v.name != "" {
		hb.Str("  name ").Str(v.name)
	}
	sb.Str(trafHeader.Render(hb.String()))
	sb.Byte('\n')

	if snap == nil {
		sb.Str(trafDim.Render("  waiting for data..."))
		sb.Byte('\n')
		sb.Byte('\n')
		sb.Str(footerLine(width))
		return sb.String()
	}

	if snap.Degraded {
		sb.Str(trafDim.Render("  (traffic-usage/flow-export not configured; interface rates only)"))
		sb.Byte('\n')
	}

	sb.Byte('\n')
	sb.Str(severityBadge(snap.Severity))
	sb.Byte('\n')

	if len(snap.Interfaces) > 0 {
		sb.Str(trafLabel.Render("  Interfaces"))
		sb.Byte('\n')
		for _, ie := range snap.Interfaces {
			sb.Str("    ")
			sb.Str(trafValue.Render(ie.Name))
			sb.Str("  ")
			sb.Str(trafLabel.Render("RX "))
			sb.Str(fmtBps(ie.RxBps))
			sb.Str("  ")
			sb.Str(trafLabel.Render("TX "))
			sb.Str(fmtBps(ie.TxBps))
			sb.Str("  ")
			sb.Str(trafLabel.Render("RxPps "))
			sb.Str(fmtPps(ie.RxPps))
			sb.Str("  ")
			sb.Str(trafLabel.Render("TxPps "))
			sb.Str(fmtPps(ie.TxPps))
			sb.Byte('\n')
		}
	}

	if len(snap.TopSrcIPs) > 0 {
		sb.Byte('\n')
		sb.Str(trafLabel.Render("  Top Source IPs"))
		sb.Byte('\n')
		n := min(10, len(snap.TopSrcIPs))
		for i := range n {
			te := &snap.TopSrcIPs[i]
			sb.Str("    ")
			sb.Str(trafValue.Render(te.Address))
			sb.Str("  ")
			sb.Str(fmtBps(te.Bps))
			sb.Byte('\n')
		}
	}

	if len(snap.TopDstIPs) > 0 {
		sb.Byte('\n')
		sb.Str(trafLabel.Render("  Top Dest IPs"))
		sb.Byte('\n')
		n := min(10, len(snap.TopDstIPs))
		for i := range n {
			te := &snap.TopDstIPs[i]
			sb.Str("    ")
			sb.Str(trafValue.Render(te.Address))
			sb.Str("  ")
			sb.Str(fmtBps(te.Bps))
			sb.Byte('\n')
		}
	}

	if len(snap.TopPorts) > 0 {
		sb.Byte('\n')
		sb.Str(trafLabel.Render("  Top Ports"))
		sb.Byte('\n')
		n := min(10, len(snap.TopPorts))
		for i := range n {
			pe := &snap.TopPorts[i]
			sb.Str("    ")
			sb.Str(trafValue.Render(pe.Service))
			if pe.Amplification != "" {
				sb.Str(" ")
				sb.Str(trafCaution.Render(pe.Amplification))
			}
			sb.Str("  ")
			sb.Str(fmtBps(pe.Bps))
			sb.Byte('\n')
		}
	}

	if len(snap.Protocols) > 0 {
		sb.Byte('\n')
		sb.Str(trafLabel.Render("  Protocol Mix"))
		sb.Byte('\n')
		for _, pm := range snap.Protocols {
			sb.Str("    ")
			sb.Str(trafValue.Render(pm.Name))
			sb.Str("  ")
			sb.Str(fmtBps(pm.Bps))
			sb.Str("  ")
			var pctBuf textbuf.Buffer
			sb.Str(trafDim.Render(pctBuf.Float(pm.Percent, 1).Byte('%').String()))
			sb.Byte('\n')
		}
	}

	if len(snap.History) > 1 {
		sb.Byte('\n')
		sb.Str(trafLabel.Render("  History (60s)  "))
		sb.Str(sparkline(snap.History, 30))
		sb.Byte('\n')
	}

	sb.Byte('\n')
	sb.Str(footerLine(width))
	return sb.String()
}

func severityBadge(severity string) string {
	switch severity {
	case "caution":
		return trafCaution.Render("  [CAUTION]")
	case "danger":
		return trafDanger.Render("  [DANGER]")
	default:
		return trafValue.Render("  [NORMAL]")
	}
}

func fmtBps(bps float64) string {
	var tb textbuf.Buffer
	switch {
	case bps >= 1e9:
		tb.Float(bps/1e9, 1).Str(" Gbps")
	case bps >= 1e6:
		tb.Float(bps/1e6, 1).Str(" Mbps")
	case bps >= 1e3:
		tb.Float(bps/1e3, 1).Str(" Kbps")
	default:
		tb.Float(bps, 0).Str(" bps")
	}
	return trafValue.Render(tb.String())
}

func fmtPps(pps float64) string {
	var tb textbuf.Buffer
	switch {
	case pps >= 1e6:
		tb.Float(pps/1e6, 1).Str("M")
	case pps >= 1e3:
		tb.Float(pps/1e3, 1).Str("K")
	default:
		tb.Float(pps, 0)
	}
	return trafValue.Render(tb.String())
}

func footerLine(width int) string {
	footer := "q/Esc Quit"
	gap := max(1, width-len(footer))
	var buf textbuf.Buffer
	buf.Str(footer)
	for range gap {
		buf.Byte(' ')
	}
	return trafFooter.Render(buf.String())
}

const sparkChars = " _.-~*"

func sparkline(data []float64, maxWidth int) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) > maxWidth {
		data = data[len(data)-maxWidth:]
	}
	var hi float64
	for _, v := range data {
		if v > hi {
			hi = v
		}
	}
	levels := len(sparkChars) - 1
	var sb textbuf.Buffer
	for _, v := range data {
		idx := 0
		if hi > 0 {
			idx = min(int(v/hi*float64(levels)), levels)
		}
		sb.Byte(sparkChars[idx])
	}
	return trafDim.Render(sb.String())
}
