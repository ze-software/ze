// Design: model_dashboard.go -- poll-based live view pattern
// Related: model_monitor.go -- channel-drain pattern (50ms poll tick)
// Related: model_enrich.go -- | resolve / | origin enrichment for the | log legend

package cli

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const tracerouteDrainInterval = 50 * time.Millisecond

// TracerouteFactory starts a streaming probe round. The returned channel
// receives individual hop results as ICMP responses arrive. It is closed
// when the round completes (after ~1s). Cancel stops the round early.
type TracerouteFactory func(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error)

// ViewKeyTraceroute is the registry/factory key for the monitor-traceroute live
// view. Consumers inject a TracerouteFactory under this key via SetViewFactory.
const ViewKeyTraceroute = "traceroute"

type traceroutePollMsg struct{}

// traceroutePipedPollMsg triggers a poll of the piped traceroute channel.
type traceroutePipedPollMsg struct{}

func (traceroutePollMsg) isViewMsg()      {}
func (traceroutePipedPollMsg) isViewMsg() {}

// traceroutePathStats holds per-IP statistics at a given TTL.
type traceroutePathStats struct {
	addr  string
	sent  int
	recv  int
	last  float64
	best  float64
	worst float64
	sum   float64
	sumSq float64
}

func newPathStats(addr string) traceroutePathStats {
	return traceroutePathStats{addr: addr, best: math.MaxFloat64}
}

func (p *traceroutePathStats) loss() float64 {
	if p.sent == 0 {
		return 0
	}
	return float64(p.sent-p.recv) / float64(p.sent) * 100
}

func (p *traceroutePathStats) avg() float64 {
	if p.recv == 0 {
		return 0
	}
	return p.sum / float64(p.recv)
}

func (p *traceroutePathStats) stddev() float64 {
	if p.recv < 2 {
		return 0
	}
	n := float64(p.recv)
	variance := (p.sumSq - p.sum*p.sum/n) / (n - 1)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

// tracerouteHop holds all paths seen at a given TTL.
type tracerouteHop struct {
	paths []traceroutePathStats
}

func (h *tracerouteHop) findOrCreate(addr string) *traceroutePathStats {
	for i := range h.paths {
		if h.paths[i].addr == addr {
			return &h.paths[i]
		}
	}
	h.paths = append(h.paths, newPathStats(addr))
	return &h.paths[len(h.paths)-1]
}

// absorbStar merges the "*" path's sent count into the named addr
// and removes the "*" entry. Called when a real IP first appears.
func (h *tracerouteHop) firstRealPath() *traceroutePathStats {
	for i := range h.paths {
		if h.paths[i].addr != "*" {
			return &h.paths[i]
		}
	}
	return nil
}

func (h *tracerouteHop) absorbStar(addr string) {
	starIdx := -1
	for i := range h.paths {
		if h.paths[i].addr == "*" {
			starIdx = i
			break
		}
	}
	if starIdx < 0 {
		return
	}
	starSent := h.paths[starIdx].sent
	h.paths = append(h.paths[:starIdx], h.paths[starIdx+1:]...)

	for i := range h.paths {
		if h.paths[i].addr == addr {
			h.paths[i].sent += starSent
			return
		}
	}
	p := newPathStats(addr)
	p.sent = starSent
	h.paths = append(h.paths, p)
}

type tracerouteState struct {
	target       string
	maxHops      int
	hops         []tracerouteHop
	rounds       int
	lastPollTime time.Time
	pollError    string
	poller       TracerouteFactory
	hopChan      <-chan map[string]any
	cancelRound  context.CancelFunc
}

// traceroutePipedState holds state for piped monitor traceroute (| json, | ndjson, etc.).
// Default (replace): alt screen, each round replaces the display, last output copied on exit.
// With | log: appends each round to scrollback.
type traceroutePipedState struct {
	target        string
	maxHops       int
	hops          []tracerouteHop
	rounds        int
	poller        TracerouteFactory
	formatFn      func(string) string
	logMode       bool
	hasFormatPipe bool
	pipeResolve   bool
	pipeOrigin    bool
	lastOutput    string
	hopChan       <-chan map[string]any
	cancelRound   context.CancelFunc
}

func isTracerouteMonitorCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if strings.ContainsRune(trimmed, '|') {
		return false
	}
	return strings.HasPrefix(trimmed, "monitor traceroute ")
}

func isPipedTracerouteMonitorCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if !strings.ContainsRune(trimmed, '|') {
		return false
	}
	cmd, _ := command.ParsePipe(trimmed)
	return strings.HasPrefix(strings.TrimSpace(cmd), "monitor traceroute ")
}

func parseTracerouteMonitorArgs(input string) (target string, maxHops int, errMsg string) {
	maxHops = 16

	trimmed := strings.TrimSpace(input)
	after := strings.TrimPrefix(trimmed, "monitor traceroute ")
	args := strings.Fields(after)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "max-hops":
			if i+1 < len(args) {
				if n := parsePositiveInt(args[i+1]); n > 0 && n <= 64 {
					maxHops = n
				}
				i++
			}
		default:
			if target == "" {
				target = args[i]
			} else {
				var tb textbuf.Buffer
				return "", maxHops, tb.Str("unexpected argument: ").Str(args[i]).Str(" (use | for pipe operators)").String()
			}
		}
	}
	return target, maxHops, ""
}

func hopInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func hopFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func parsePositiveInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (m *Model) startTraceroute(input string) tea.Cmd {
	factory := m.tracerouteFactory()
	if factory == nil {
		m.statusMessage = "traceroute not available (no daemon connection)"
		return nil
	}

	target, maxHops, argErr := parseTracerouteMonitorArgs(input)
	if argErr != "" {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("monitor traceroute: ").Str(argErr).String()
		return nil
	}
	if target == "" {
		m.statusMessage = "monitor traceroute: missing target address"
		return nil
	}

	m.activeView = &tracerouteView{st: &tracerouteState{
		target:  target,
		maxHops: maxHops,
		poller:  factory,
	}}

	return m.startTracerouteRound()
}

func (m *Model) startTraceroutePiped(input string) tea.Cmd {
	factory := m.tracerouteFactory()
	if factory == nil {
		m.statusMessage = "traceroute not available (no daemon connection)"
		return nil
	}

	cmdStr, formatFn, pipeFlags, pipeErr := command.ProcessPipesDetectLog(input, m.cliFormat)
	if pipeErr != "" {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("pipe error: ").Str(pipeErr).String()
		return nil
	}
	target, maxHops, argErr := parseTracerouteMonitorArgs(cmdStr)
	if argErr != "" {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("monitor traceroute: ").Str(argErr).String()
		return nil
	}
	if target == "" {
		m.statusMessage = "monitor traceroute: missing target address"
		return nil
	}

	m.activeView = &traceroutePipedView{st: &traceroutePipedState{
		target:        target,
		maxHops:       maxHops,
		poller:        factory,
		formatFn:      formatFn,
		logMode:       pipeFlags.Log,
		hasFormatPipe: pipeFlags.HasFormat,
		pipeResolve:   pipeFlags.Resolve,
		pipeOrigin:    pipeFlags.Origin,
	}}

	if pipeFlags.Log {
		m.outputBuf.WriteString("--- monitor traceroute | log (Esc to stop) ---\n")
		m.setViewportTextBottom(m.outputBuf.String())
	}
	m.statusMessage = "monitoring traceroute (Esc to stop)"

	return m.startTraceroutePipedRound()
}

func (m *Model) startTracerouteRound() tea.Cmd {
	ts := m.activeTraceroute()
	if ts == nil || ts.poller == nil {
		return nil
	}

	ch, cancel, err := ts.poller(context.Background(), ts.target, ts.maxHops)
	if err != nil {
		ts.pollError = err.Error()
		return nil
	}

	ts.hopChan = ch
	ts.cancelRound = cancel

	return tea.Tick(tracerouteDrainInterval, func(time.Time) tea.Msg { return traceroutePollMsg{} })
}

func (m *Model) startTraceroutePipedRound() tea.Cmd {
	ps := m.activeTraceroutePiped()
	if ps == nil || ps.poller == nil {
		return nil
	}

	ch, cancel, err := ps.poller(context.Background(), ps.target, ps.maxHops)
	if err != nil {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("traceroute error: ").Err(err).String()
		return nil
	}

	ps.hopChan = ch
	ps.cancelRound = cancel

	return tea.Tick(tracerouteDrainInterval, func(time.Time) tea.Msg { return traceroutePipedPollMsg{} })
}

func (m *Model) stopTraceroute() {
	ts := m.activeTraceroute()
	if ts == nil {
		return
	}
	if ts.cancelRound != nil {
		ts.cancelRound()
	}

	lastRender := m.renderTraceroutePlain()
	m.activeView = nil

	if lastRender != "" {
		if m.outputBuf.Len() > 0 {
			m.outputBuf.WriteString("\n")
		}
		m.outputBuf.WriteString(lastRender)
	}

	if m.hasEditor() {
		m.showConfigContent()
	} else if lastRender != "" {
		m.setViewportText(m.outputBuf.String())
		m.viewport.GotoBottom()
	}
	m.statusMessage = "traceroute stopped"
}

func (m *Model) stopTraceroutePiped() {
	ps := m.activeTraceroutePiped()
	if ps == nil {
		return
	}
	if ps.cancelRound != nil {
		ps.cancelRound()
	}

	lastOutput := ps.lastOutput
	isLog := ps.logMode
	target := ps.target
	rounds := ps.rounds
	m.activeView = nil

	if !isLog && lastOutput != "" {
		if m.outputBuf.Len() > 0 {
			m.outputBuf.WriteString("\n")
		}
		var hb textbuf.Buffer
		hb.Str("Traceroute to ").Str(target).Str("  rounds ").Int(int64(rounds)).Str("\n\n")
		m.outputBuf.WriteString(hb.Slice())
		m.outputBuf.WriteString(lastOutput)
	}

	if m.hasEditor() {
		m.showConfigContent()
	} else if lastOutput != "" || isLog {
		m.setViewportText(m.outputBuf.String())
		m.viewport.GotoBottom()
	}
	m.statusMessage = "traceroute stopped"
}

func drainTracerouteHops(ch <-chan map[string]any) (hops []map[string]any, closed bool) {
	for {
		select {
		case hop, ok := <-ch:
			if !ok {
				return hops, true
			}
			hops = append(hops, hop)
		default:
			return hops, false
		}
	}
}

func applyHop(ts *tracerouteState, hop map[string]any) {
	applyHopTo(&ts.hops, hop)
}

func applyHopTo(hops *[]tracerouteHop, hop map[string]any) {
	idx := hopInt(hop["ttl"])
	if idx < 1 {
		return
	}
	for len(*hops) < idx {
		*hops = append(*hops, tracerouteHop{})
	}

	addr, _ := hop["addr"].(string)
	if addr == "" {
		addr = "*"
	}

	h := &(*hops)[idx-1]

	if addr != "*" {
		h.absorbStar(addr)
	} else if first := h.firstRealPath(); first != nil {
		first.sent++
		return
	}

	p := h.findOrCreate(addr)
	p.sent++

	rtt, hasRTT := hopFloat(hop["rtt-ms"])
	if hasRTT {
		p.recv++
		p.last = rtt
		p.sum += rtt
		p.sumSq += rtt * rtt
		if rtt < p.best {
			p.best = rtt
		}
		if rtt > p.worst {
			p.worst = rtt
		}
	}
}

func (m Model) handleTraceroutePoll() (tea.Model, tea.Cmd) {
	ts := m.activeTraceroute()
	if ts == nil || ts.hopChan == nil {
		return m, nil
	}

	hops, closed := drainTracerouteHops(ts.hopChan)

	for _, hop := range hops {
		applyHop(ts, hop)
	}

	if closed {
		ts.hopChan = nil
		ts.cancelRound = nil
		ts.lastPollTime = time.Now()
		ts.rounds++
		ts.pollError = ""

		if n := len(ts.hops); n > 0 && n < ts.maxHops {
			ts.maxHops = n
		}

		cmd := m.startTracerouteRound()
		return m, cmd
	}

	return m, tea.Tick(tracerouteDrainInterval, func(time.Time) tea.Msg { return traceroutePollMsg{} })
}

func (m Model) handleTraceroutePipedPoll() (tea.Model, tea.Cmd) {
	ps := m.activeTraceroutePiped()
	if ps == nil || ps.hopChan == nil {
		return m, nil
	}

	hops, closed := drainTracerouteHops(ps.hopChan)

	for _, hop := range hops {
		applyHopTo(&ps.hops, hop)
	}

	if closed {
		ps.hopChan = nil
		ps.cancelRound = nil
		ps.rounds++

		if n := len(ps.hops); n > 0 && n < ps.maxHops {
			ps.maxHops = n
		}

		rawJSON := hopsToJSON(ps.hops, ps.rounds)
		formatted := ps.formatFn(rawJSON)
		ps.lastOutput = formatted

		if ps.logMode {
			if ps.hasFormatPipe {
				m.outputBuf.WriteString(strings.TrimRight(formatted, "\n"))
				m.outputBuf.WriteString("\n")
			} else {
				if ps.rounds == 1 || (ps.rounds-1)%tracerouteLogMapEveryN == 0 {
					if m.outputBuf.Len() > 0 {
						m.outputBuf.WriteString("\n")
					}
					m.outputBuf.WriteString(formatTracerouteLogMap(ps.hops, ps.pipeResolve, ps.pipeOrigin))
					m.outputBuf.WriteString(formatTracerouteLogHeader(ps.hops))
				}
				m.outputBuf.WriteString("\n")
				m.outputBuf.WriteString(formatTracerouteLogLine(ps.hops, ps.rounds))
			}
			m.setViewportTextBottom(m.outputBuf.String())
		}

		cmd := m.startTraceroutePipedRound()
		return m, cmd
	}

	return m, tea.Tick(tracerouteDrainInterval, func(time.Time) tea.Msg { return traceroutePipedPollMsg{} })
}

const (
	tracerouteLogColWidth  = 10
	tracerouteLogMapEveryN = 25
)

// formatTracerouteLogMap renders the hop-number-to-IP legend as
// fixed-width entries that wrap at terminal width. Each entry is
// padded to a uniform width so columns stay aligned across rows.
func formatTracerouteLogMap(hops []tracerouteHop, resolve, origin bool) string {
	maxAddrLen := 0
	addrs := make([]string, len(hops))
	for i := range hops {
		h := &hops[i]
		addr := "*"
		if len(h.paths) > 0 && h.paths[0].addr != "" {
			addr = h.paths[0].addr
			addr = enrichAddr(addr, resolve, origin)
		}
		addrs[i] = addr
		if len(addr) > maxAddrLen {
			maxAddrLen = len(addr)
		}
	}

	numW := len(textbuf.StringInt(int64(len(hops))))
	colW := numW + 1 + maxAddrLen + 2

	var sb textbuf.Buffer
	sb.Reset(256)
	col := 0
	for i, addr := range addrs {
		if col > 0 && col+colW > 80 {
			sb.Byte('\n')
			col = 0
		}
		num := textbuf.StringInt(int64(i + 1))
		tbPadLeft(&sb, num, numW)
		sb.Byte(':')
		tbPadLeft(&sb, addr, maxAddrLen)
		sb.Str("  ")
		col += colW
	}
	sb.Byte('\n')
	return sb.String()
}

// formatTracerouteLogHeader renders the column header using hop numbers.
func formatTracerouteLogHeader(hops []tracerouteHop) string {
	var sb textbuf.Buffer
	sb.Reset(128)
	tbPadRight(&sb, "Rnd", 5)
	for i := range hops {
		tbPadRight(&sb, textbuf.StringInt(int64(i+1)), tracerouteLogColWidth)
	}
	return sb.String()
}

// formatTracerouteLogLine renders one compact line per round showing
// the last RTT (or *) at each hop.
func formatTracerouteLogLine(hops []tracerouteHop, round int) string {
	var sb textbuf.Buffer
	sb.Reset(128)
	tbPadRight(&sb, textbuf.StringInt(int64(round)), 5)
	for i := range hops {
		h := &hops[i]
		if len(h.paths) == 0 {
			tbPadRight(&sb, "*", tracerouteLogColWidth)
			continue
		}
		p := &h.paths[0]
		if p.recv == 0 {
			tbPadRight(&sb, "*", tracerouteLogColWidth)
			continue
		}
		tbPadRight(&sb, formatFloat1(p.last)+"ms", tracerouteLogColWidth)
	}
	return sb.String()
}

// hopsToJSON serializes accumulated hop stats as a JSON wrapper suitable for pipe processing.
func hopsToJSON(hops []tracerouteHop, rounds int) string {
	var b textbuf.Buffer
	b.Reset(512)
	b.Str(`{"hops":[`)
	first := true
	for i := range hops {
		h := &hops[i]
		for j := range h.paths {
			p := &h.paths[j]
			if !first {
				b.Byte(',')
			}
			first = false
			b.Str(`{"ttl":`).Int(int64(i + 1))
			b.Str(`,"addr":"`).Str(p.addr).Byte('"')
			b.Str(`,"loss":`).Float2(p.loss())
			b.Str(`,"sent":`).Int(int64(p.sent))
			b.Str(`,"rounds":`).Int(int64(rounds))
			if p.recv > 0 {
				b.Str(`,"last":`).Float2(p.last)
				b.Str(`,"avg":`).Float2(p.avg())
				b.Str(`,"best":`).Float2(p.best)
				b.Str(`,"worst":`).Float2(p.worst)
				b.Str(`,"stdev":`).Float2(p.stddev())
			}
			b.Byte('}')
		}
	}
	b.Str(`]}`)
	return b.String()
}

func (m *Model) handleTracerouteKey(keyStr string) bool {
	if m.activeTraceroute() == nil {
		return false
	}
	switch keyStr {
	case "q", keyCtrlC, keyEsc:
		m.stopTraceroute()
	}
	return true
}

// Rendering

var (
	trHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	trFooterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	trLossOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	trLossWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	trLossBad     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	trDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func formatFloat1(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func tbPadLeft(sb *textbuf.Buffer, s string, width int) {
	for i := len(s); i < width; i++ {
		sb.Byte(' ')
	}
	sb.Str(s)
}

func tbPadRight(sb *textbuf.Buffer, s string, width int) {
	sb.Str(s)
	for i := len(s); i < width; i++ {
		sb.Byte(' ')
	}
}

func tbSpaces(sb *textbuf.Buffer, n int) {
	for range n {
		sb.Byte(' ')
	}
}

func renderPathLine(sb *textbuf.Buffer, hopNum string, p *traceroutePathStats, addrWidth int) {
	addr := p.addr
	if addr == "" {
		addr = "???"
	}

	loss := p.loss()
	var tbL textbuf.Buffer
	lossText := tbL.Str(formatFloat1(loss)).Byte('%').String()
	var lossStyle lipgloss.Style
	switch {
	case loss == 0:
		lossStyle = trLossOK
	case loss < 20:
		lossStyle = trLossWarn
	default:
		lossStyle = trLossBad
	}

	sb.Byte(' ')
	tbPadLeft(sb, hopNum, 3)
	sb.Byte(' ')
	tbPadRight(sb, addr, addrWidth)
	sb.Byte(' ')

	var lossBuf textbuf.Buffer
	tbPadLeft(&lossBuf, lossText, 6)
	sb.Str(lossStyle.Render(lossBuf.String()))

	sb.Byte(' ')
	tbPadLeft(sb, textbuf.StringInt(int64(p.sent)), 4)

	if p.recv > 0 {
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.last), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.avg()), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.best), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.worst), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.stddev()), 6)
	} else {
		for range 5 {
			sb.Byte(' ')
			tbPadLeft(sb, "--", 6)
		}
	}
	sb.Byte('\n')
}

func renderPathLinePlain(sb *textbuf.Buffer, hopNum string, p *traceroutePathStats, addrWidth int) {
	addr := p.addr
	if addr == "" {
		addr = "???"
	}

	var tbL2 textbuf.Buffer
	lossText := tbL2.Str(formatFloat1(p.loss())).Byte('%').String()

	sb.Byte(' ')
	tbPadLeft(sb, hopNum, 3)
	sb.Byte(' ')
	tbPadRight(sb, addr, addrWidth)
	sb.Byte(' ')
	tbPadLeft(sb, lossText, 6)
	sb.Byte(' ')
	tbPadLeft(sb, textbuf.StringInt(int64(p.sent)), 4)

	if p.recv > 0 {
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.last), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.avg()), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.best), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.worst), 6)
		sb.Byte(' ')
		tbPadLeft(sb, formatFloat1(p.stddev()), 6)
	} else {
		for range 5 {
			sb.Byte(' ')
			tbPadLeft(sb, "--", 6)
		}
	}
	sb.Byte('\n')
}

func (m Model) renderTraceroute() string {
	ts := m.activeTraceroute()
	if ts == nil {
		return ""
	}

	width := m.width
	if width <= 0 {
		width = 80
	}

	var sb textbuf.Buffer
	sb.Reset(1024)

	var hb textbuf.Buffer
	hb.Str("Traceroute to ").Str(ts.target).Str("  rounds ").Int(int64(ts.rounds))
	if ts.pollError != "" {
		hb.Str("  ").Str(ts.pollError)
	}
	sb.Str(trHeaderStyle.Render(hb.String()))
	sb.Byte('\n')

	addrWidth := min(39, max(15, width-65))

	var hdrBuf textbuf.Buffer
	hdrBuf.Byte(' ')
	tbPadRight(&hdrBuf, "Hop", 3)
	hdrBuf.Byte(' ')
	tbPadRight(&hdrBuf, "Address", addrWidth)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Loss%", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Snt", 4)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Last", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Avg", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Best", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Wrst", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "StDev", 6)
	hdr := hdrBuf.String()
	if len(hdr) > width {
		hdr = hdr[:width]
	}
	sb.Str(trDimStyle.Render(hdr))
	sb.Byte('\n')

	for i := range ts.hops {
		h := &ts.hops[i]
		hopNum := textbuf.StringInt(int64(i + 1))
		for j := range h.paths {
			label := hopNum
			if j > 0 {
				label = ""
			}
			renderPathLine(&sb, label, &h.paths[j], addrWidth)
		}
		if len(h.paths) == 0 {
			renderPathLine(&sb, hopNum, &traceroutePathStats{addr: "???"}, addrWidth)
		}
	}

	if len(ts.hops) == 0 {
		if ts.pollError != "" {
			var tbE textbuf.Buffer
			sb.Str(trLossBad.Render(tbE.Str("  error: ").Str(ts.pollError).String()))
		} else {
			sb.Str(trDimStyle.Render("  waiting for data..."))
		}
		sb.Byte('\n')
	}

	sb.Byte('\n')
	footer := footerQuitHint
	lastUpdate := ""
	if !ts.lastPollTime.IsZero() {
		ago := time.Since(ts.lastPollTime).Truncate(time.Second)
		var tbA textbuf.Buffer
		lastUpdate = tbA.Str("Last probe: ").Str(ago.String()).Str(" ago").String()
	}
	gap := max(1, width-len(footer)-len(lastUpdate))
	var footBuf textbuf.Buffer
	footBuf.Str(footer)
	tbSpaces(&footBuf, gap)
	footBuf.Str(lastUpdate)
	sb.Str(trFooterStyle.Render(footBuf.String()))

	return sb.String()
}

// renderTraceroutePlain renders the traceroute table without ANSI styling,
// suitable for persisting in the scrollback buffer after leaving alt screen.
func (m Model) renderTraceroutePlain() string {
	ts := m.activeTraceroute()
	if ts == nil || len(ts.hops) == 0 {
		return ""
	}

	width := m.width
	if width <= 0 {
		width = 80
	}

	var sb textbuf.Buffer
	sb.Reset(1024)

	var hb textbuf.Buffer
	hb.Str("Traceroute to ").Str(ts.target).Str("  rounds ").Int(int64(ts.rounds))
	sb.Str(hb.String())
	sb.Byte('\n')

	addrWidth := min(39, max(15, width-65))

	var hdrBuf textbuf.Buffer
	hdrBuf.Byte(' ')
	tbPadRight(&hdrBuf, "Hop", 3)
	hdrBuf.Byte(' ')
	tbPadRight(&hdrBuf, "Address", addrWidth)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Loss%", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Snt", 4)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Last", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Avg", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Best", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "Wrst", 6)
	hdrBuf.Byte(' ')
	tbPadLeft(&hdrBuf, "StDev", 6)
	sb.Str(hdrBuf.String())
	sb.Byte('\n')

	for i := range ts.hops {
		h := &ts.hops[i]
		hopNum := textbuf.StringInt(int64(i + 1))
		for j := range h.paths {
			label := hopNum
			if j > 0 {
				label = ""
			}
			renderPathLinePlain(&sb, label, &h.paths[j], addrWidth)
		}
		if len(h.paths) == 0 {
			renderPathLinePlain(&sb, hopNum, &traceroutePathStats{addr: "???"}, addrWidth)
		}
	}

	return sb.String()
}

// renderTraceroutePiped renders piped traceroute output for alt screen (replace mode).
func (m Model) renderTraceroutePiped() string {
	ps := m.activeTraceroutePiped()
	if ps == nil {
		return ""
	}

	width := m.width
	if width <= 0 {
		width = 80
	}

	var sb textbuf.Buffer
	sb.Reset(1024)

	var hb textbuf.Buffer
	hb.Str("Traceroute to ").Str(ps.target).Str("  rounds ").Int(int64(ps.rounds))
	sb.Str(trHeaderStyle.Render(hb.String()))
	sb.Byte('\n')
	sb.Byte('\n')

	if ps.lastOutput != "" {
		sb.Str(ps.lastOutput)
	} else {
		sb.Str(trDimStyle.Render("  waiting for data..."))
	}
	sb.Byte('\n')
	sb.Byte('\n')

	footer := footerQuitHint
	gap := max(1, width-len(footer))
	var footBuf textbuf.Buffer
	footBuf.Str(footer)
	tbSpaces(&footBuf, gap)
	sb.Str(trFooterStyle.Render(footBuf.String()))

	return sb.String()
}
