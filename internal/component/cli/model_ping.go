// Design: docs/architecture/api/commands.md -- monitor ping TUI model
// Related: model_traceroute.go -- same poll/drain/render pattern
// Related: model_monitor.go -- generic monitor session pattern
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

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const pingDrainInterval = 50 * time.Millisecond

// PingFactory starts a continuous ping session. The returned channel
// receives per-reply results until the context is canceled.
//
// count bounds the probes (0 = stream until canceled); size is the ICMP payload
// in bytes (0 = the engine default). Both are passed as primitives rather than a
// ping/cmd options struct so this package keeps its inversion: cli defines the
// contract and never imports the ping engine.
type PingFactory func(ctx context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error)

// ViewKeyPing is the registry/factory key for the monitor-ping live view.
// Consumers inject a PingFactory under this key via SetViewFactory.
const ViewKeyPing = "ping"

type pingPollMsg struct{}
type pingPipedPollMsg struct{}

func (pingPollMsg) isViewMsg()      {}
func (pingPipedPollMsg) isViewMsg() {}

// pingView / pingPipedView are the activeView instances for monitor ping. They
// hold the session state and delegate to the render/poll/stop methods that stay
// in this file (Design 1).
type pingView struct{ st *pingState }
type pingPipedView struct{ st *pingPipedState }

func (v *pingView) update(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(pingPollMsg); ok {
		return m.handlePingPoll()
	}
	return *m, nil
}
func (v *pingView) render(m *Model) string      { return m.renderPingMonitor() }
func (v *pingView) key(m *Model, k string) bool { return m.handlePingMonitorKey(k) }
func (v *pingView) release() {
	if v.st != nil && v.st.cancel != nil {
		v.st.cancel()
	}
}

func (v *pingPipedView) update(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(pingPipedPollMsg); ok {
		return m.handlePingPipedPoll()
	}
	return *m, nil
}

func (v *pingPipedView) render(m *Model) string {
	// In | log mode the piped view appends to the scrollback viewport; return ""
	// so View() falls through to the normal render instead of the alt screen.
	if v.st.logMode {
		return ""
	}
	return m.renderPingMonitorPiped()
}

func (v *pingPipedView) key(m *Model, keyStr string) bool {
	if keyStr == "q" || keyStr == keyCtrlC || keyStr == keyEsc {
		m.stopPingMonitorPiped()
		return true
	}
	// Replace mode (alt screen) absorbs all keys; log mode lets others through.
	return !v.st.logMode
}
func (v *pingPipedView) release() {
	if v.st != nil && v.st.cancel != nil {
		v.st.cancel()
	}
}

// activePing / activePingPiped return the active ping session state, or nil
// when the active view is not a ping view.
func (m *Model) activePing() *pingState {
	if v, ok := m.activeView.(*pingView); ok {
		return v.st
	}
	return nil
}

func (m *Model) activePingPiped() *pingPipedState {
	if v, ok := m.activeView.(*pingPipedView); ok {
		return v.st
	}
	return nil
}

// pingStats tracks running statistics for the ping monitor.
type pingStats struct {
	sent  int
	recv  int
	last  float64
	min   float64
	max   float64
	sum   float64
	sumSq float64
}

func (s *pingStats) loss() float64 {
	if s.sent == 0 {
		return 0
	}
	return float64(s.sent-s.recv) / float64(s.sent) * 100
}

func (s *pingStats) avg() float64 {
	if s.recv == 0 {
		return 0
	}
	return s.sum / float64(s.recv)
}

func (s *pingStats) stddev() float64 {
	if s.recv < 2 {
		return 0
	}
	n := float64(s.recv)
	variance := (s.sumSq - s.sum*s.sum/n) / (n - 1)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

type pingState struct {
	target   string
	interval time.Duration
	timeout  time.Duration
	stats    pingStats
	poller   PingFactory
	replyCh  <-chan map[string]any
	cancel   context.CancelFunc
}

// pingPipedState holds state for piped monitor ping (| json, | log, etc.).
// pipeResolve/pipeOrigin mirror traceroutePipedState: the | log render path
// bypasses ApplyPipes, so data-transform pipes are applied via enrichAddr
// when the target legend is written (see ai/rules/pipe-completeness.md).
type pingPipedState struct {
	target        string
	interval      time.Duration
	timeout       time.Duration
	stats         pingStats
	poller        PingFactory
	formatFn      func(string) string
	logMode       bool
	hasFormatPipe bool
	pipeResolve   bool
	pipeOrigin    bool
	replyCh       <-chan map[string]any
	cancel        context.CancelFunc
}

// Monitor-ping argument bounds.
//
// Duplicated deliberately rather than imported: this package defines the
// PingFactory contract and never imports the ping engine, so the limits are
// restated here. They MUST match internal/component/ping/cmd/ping.go
// (minPingMonitorInterval / maxPingMonitorInterval / maxPingCount /
// maxPingSize), which the offline `monitor ping` parser enforces -- the two
// paths are the same command and must accept the same input. 65507 is the
// largest ICMP payload that still fits a 65535-byte IP datagram after the
// 20-byte IPv4 and 8-byte ICMP headers.
const (
	defaultPingMonitorInterval = time.Second
	defaultPingMonitorTimeout  = 5 * time.Second
	minPingMonitorInterval     = 100 * time.Millisecond
	maxPingMonitorInterval     = 30 * time.Second
	maxPingMonitorCount        = 100
	maxPingMonitorSize         = 65507
)

func isPingMonitorCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if strings.ContainsRune(trimmed, '|') {
		return false
	}
	return strings.HasPrefix(trimmed, "monitor ping ")
}

func isPipedPingMonitorCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if !strings.ContainsRune(trimmed, '|') {
		return false
	}
	cmd, _ := command.ParsePipe(trimmed)
	return strings.HasPrefix(strings.TrimSpace(cmd), "monitor ping ")
}

// parsePingMonitorArgs parses
// `monitor ping <target> [interval <dur>] [timeout <dur>] [count <n>] [size <bytes>]`.
//
// count 0 streams until stopped; size 0 sends the engine's default payload.
// Bounds here must match the offline parser (internal/component/ping/cmd/ping.go
// parseMonitorPingArgs) so the command behaves identically with and without a
// daemon; that parser owns the same limits.
// pingMonitorArgs is the parsed form of a `monitor ping` invocation.
type pingMonitorArgs struct {
	Target   string
	Interval time.Duration
	Timeout  time.Duration
	Count    int // 0 = stream until stopped
	Size     int // 0 = engine default payload
}

func parsePingMonitorArgs(input string) (pingMonitorArgs, string) {
	out := pingMonitorArgs{
		Interval: defaultPingMonitorInterval,
		Timeout:  defaultPingMonitorTimeout,
	}

	trimmed := strings.TrimSpace(input)
	after := strings.TrimPrefix(trimmed, "monitor ping ")
	args := strings.Fields(after)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "interval":
			if i+1 >= len(args) {
				return pingMonitorArgs{}, "interval requires a value"
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < minPingMonitorInterval || d > maxPingMonitorInterval {
				var b textbuf.Buffer
				b.Str("interval must be 100ms-30s, got ").Str(args[i+1])
				return pingMonitorArgs{}, b.String()
			}
			out.Interval = d
			i++
		case "timeout":
			if i+1 >= len(args) {
				return pingMonitorArgs{}, "timeout requires a value"
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > 30*time.Second {
				var b textbuf.Buffer
				b.Str("timeout must be 1s-30s, got ").Str(args[i+1])
				return pingMonitorArgs{}, b.String()
			}
			out.Timeout = d
			i++
		case "count":
			if i+1 >= len(args) {
				return pingMonitorArgs{}, "count requires a value"
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingMonitorCount {
				var b textbuf.Buffer
				b.Str("count must be 1-").Int(int64(maxPingMonitorCount)).Str(", got ").Str(args[i+1])
				return pingMonitorArgs{}, b.String()
			}
			out.Count = n
			i++
		case "size":
			if i+1 >= len(args) {
				return pingMonitorArgs{}, "size requires a value"
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingMonitorSize {
				var b textbuf.Buffer
				b.Str("size must be 1-").Int(int64(maxPingMonitorSize)).Str(", got ").Str(args[i+1])
				return pingMonitorArgs{}, b.String()
			}
			out.Size = n
			i++
		default:
			if out.Target == "" {
				out.Target = args[i]
			} else {
				var tb textbuf.Buffer
				return pingMonitorArgs{}, tb.Str("unexpected argument: ").Str(args[i]).Str(" (use | for pipe operators)").String()
			}
		}
	}
	return out, ""
}

// SetPingFactory sets the factory used to run continuous ping sessions.
// Thin wrapper over the generic keyed factory store (view_registry.go).
func (m *Model) SetPingFactory(f PingFactory) {
	m.SetViewFactory(ViewKeyPing, f)
}

// pingFactory returns the injected PingFactory, or nil when none is registered
// or the stored value is the wrong type. Fail-closed: a type mismatch is a
// misconfiguration, surfaced by the caller as an unavailable status message,
// never a nil-driven silent no-op (ai/rules/fail-closed-guards.md).
func (m *Model) pingFactory() PingFactory {
	raw, present := m.viewFactoryRaw(ViewKeyPing)
	if !present {
		return nil
	}
	f, ok := raw.(PingFactory)
	if !ok {
		return nil
	}
	return f
}

func (m *Model) startPingMonitor(input string) tea.Cmd {
	factory := m.pingFactory()
	if factory == nil {
		m.statusMessage = "ping monitor not available (no daemon connection)"
		return nil
	}

	mp, argErr := parsePingMonitorArgs(input)
	if argErr != "" {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("monitor ping: ").Str(argErr).String()
		return nil
	}
	if mp.Target == "" {
		m.statusMessage = "monitor ping: missing target address"
		return nil
	}

	ch, cancel, err := factory(context.Background(), mp.Target, mp.Interval, mp.Timeout, mp.Count, mp.Size)
	if err != nil {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("monitor ping: ").Err(err).String()
		return nil
	}

	m.activeView = &pingView{st: &pingState{
		target:   mp.Target,
		interval: mp.Interval,
		timeout:  mp.Timeout,
		stats:    pingStats{min: math.MaxFloat64},
		poller:   factory,
		replyCh:  ch,
		cancel:   cancel,
	}}

	return tea.Tick(pingDrainInterval, func(time.Time) tea.Msg { return pingPollMsg{} })
}

func (m *Model) startPingMonitorPiped(input string) tea.Cmd {
	factory := m.pingFactory()
	if factory == nil {
		m.statusMessage = "ping monitor not available (no daemon connection)"
		return nil
	}

	cmdStr, formatFn, pipeFlags, pipeErr := command.ProcessPipesDetectLog(input, m.cliFormat)
	if pipeErr != "" {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("pipe error: ").Str(pipeErr).String()
		return nil
	}
	mp, argErr := parsePingMonitorArgs(cmdStr)
	if argErr != "" {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("monitor ping: ").Str(argErr).String()
		return nil
	}
	if mp.Target == "" {
		m.statusMessage = "monitor ping: missing target address"
		return nil
	}

	ch, cancel, err := factory(context.Background(), mp.Target, mp.Interval, mp.Timeout, mp.Count, mp.Size)
	if err != nil {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("monitor ping: ").Err(err).String()
		return nil
	}

	m.activeView = &pingPipedView{st: &pingPipedState{
		target:        mp.Target,
		interval:      mp.Interval,
		timeout:       mp.Timeout,
		stats:         pingStats{min: math.MaxFloat64},
		poller:        factory,
		formatFn:      formatFn,
		logMode:       pipeFlags.Log,
		hasFormatPipe: pipeFlags.HasFormat,
		pipeResolve:   pipeFlags.Resolve,
		pipeOrigin:    pipeFlags.Origin,
		replyCh:       ch,
		cancel:        cancel,
	}}

	if pipeFlags.Log {
		var hdr textbuf.Buffer
		hdr.Str("--- monitor ping ").Str(mp.Target).Str(" | log (Esc to stop) ---\n")
		m.outputBuf.WriteString(hdr.Slice())
		m.setViewportTextBottom(m.outputBuf.String())
	}
	m.statusMessage = "monitoring ping (Esc to stop)"

	return tea.Tick(pingDrainInterval, func(time.Time) tea.Msg { return pingPipedPollMsg{} })
}

func (m *Model) stopPingMonitor() {
	ps := m.activePing()
	if ps == nil {
		return
	}
	if ps.cancel != nil {
		ps.cancel()
	}

	lastRender := m.renderPingPlain()
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
	m.statusMessage = "ping monitor stopped"
}

func (m *Model) stopPingMonitorPiped() {
	ps := m.activePingPiped()
	if ps == nil {
		return
	}
	if ps.cancel != nil {
		ps.cancel()
	}

	lastOutput := renderPingStatsPlain(ps.target, &ps.stats)
	isLog := ps.logMode
	m.activeView = nil

	if m.outputBuf.Len() > 0 {
		m.outputBuf.WriteString("\n")
	}
	m.outputBuf.WriteString(lastOutput)

	if m.hasEditor() {
		m.showConfigContent()
	} else if lastOutput != "" || isLog {
		m.setViewportText(m.outputBuf.String())
		m.viewport.GotoBottom()
	}
	m.statusMessage = "ping monitor stopped"
}

func drainPingReplies(ch <-chan map[string]any) (replies []map[string]any, closed bool) {
	for {
		select {
		case reply, ok := <-ch:
			if !ok {
				return replies, true
			}
			replies = append(replies, reply)
		default:
			return replies, false
		}
	}
}

func applyPingReply(stats *pingStats, reply map[string]any) {
	stats.sent++
	status, _ := reply["status"].(string)
	if status != "ok" {
		return
	}
	rtt, ok := reply["rtt-ms"].(float64)
	if !ok {
		return
	}
	stats.recv++
	stats.last = rtt
	stats.sum += rtt
	stats.sumSq += rtt * rtt
	if rtt < stats.min {
		stats.min = rtt
	}
	if rtt > stats.max {
		stats.max = rtt
	}
}

func (m Model) handlePingPoll() (tea.Model, tea.Cmd) {
	ps := m.activePing()
	if ps == nil || ps.replyCh == nil {
		return m, nil
	}

	replies, closed := drainPingReplies(ps.replyCh)
	for _, reply := range replies {
		applyPingReply(&ps.stats, reply)
	}

	if closed {
		ps.replyCh = nil
		m.stopPingMonitor()
		return m, nil
	}

	return m, tea.Tick(pingDrainInterval, func(time.Time) tea.Msg { return pingPollMsg{} })
}

func (m Model) handlePingPipedPoll() (tea.Model, tea.Cmd) {
	ps := m.activePingPiped()
	if ps == nil || ps.replyCh == nil {
		return m, nil
	}

	replies, closed := drainPingReplies(ps.replyCh)
	for _, reply := range replies {
		applyPingReply(&ps.stats, reply)

		if ps.logMode {
			var line string
			if ps.hasFormatPipe {
				line = strings.TrimRight(ps.formatFn(pingReplyToJSON(ps.target, reply)), "\n")
			} else {
				if ps.stats.sent == 1 || (ps.stats.sent-1)%25 == 0 {
					if ps.stats.sent > 1 {
						m.outputBuf.WriteString("\n")
					}
					// Data-transform pipes (| resolve, | origin) apply even in
					// the custom | log render path: enrich the target legend.
					m.outputBuf.WriteString("--- ")
					m.outputBuf.WriteString(enrichAddr(ps.target, ps.pipeResolve, ps.pipeOrigin))
					m.outputBuf.WriteString(" ---\n")
				}
				line = formatPingReplyLine(reply)
				if ps.formatFn != nil {
					line = ps.formatFn(line)
				}
			}
			m.outputBuf.WriteString(line)
			m.outputBuf.WriteString("\n")
		}
	}

	if len(replies) > 0 && ps.logMode {
		m.setViewportTextBottom(m.outputBuf.String())
	}

	if closed {
		ps.replyCh = nil
		m.stopPingMonitorPiped()
		return m, nil
	}

	return m, tea.Tick(pingDrainInterval, func(time.Time) tea.Msg { return pingPipedPollMsg{} })
}

func pingReplyToJSON(target string, reply map[string]any) string {
	var b textbuf.Buffer
	b.Reset(64)
	b.Str(`{"target":"`)
	appendJSONString(&b, target)
	b.Str(`","seq":`)
	switch v := reply["seq"].(type) {
	case int:
		b.Int(int64(v))
	case float64:
		b.Int(int64(v))
	default:
		b.Byte('0')
	}
	status, _ := reply["status"].(string)
	b.Str(`,"status":"`)
	appendJSONString(&b, status)
	b.Byte('"')
	if status == "ok" {
		if rtt, ok := reply["rtt-ms"].(float64); ok {
			b.Str(`,"rtt-ms":`)
			b.Str(strconv.FormatFloat(rtt, 'f', 3, 64))
		}
	}
	b.Byte('}')
	return b.String()
}

func appendJSONString(b *textbuf.Buffer, s string) {
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '"':
			b.Str(`\"`)
		case c == '\\':
			b.Str(`\\`)
		case c < 0x20:
			b.Str(`\u00`)
			b.Byte("0123456789abcdef"[c>>4])
			b.Byte("0123456789abcdef"[c&0x0f])
		default:
			b.Byte(c)
		}
	}
}

func formatPingReplyLine(reply map[string]any) string {
	var seq int
	switch v := reply["seq"].(type) {
	case int:
		seq = v
	case float64:
		seq = int(v)
	}
	status, _ := reply["status"].(string)

	var b textbuf.Buffer
	b.Str("seq=").Int(int64(seq))
	if status == "ok" {
		rtt, _ := reply["rtt-ms"].(float64)
		b.Str("  rtt=").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str("ms")
	} else {
		b.Str("  ").Str(status)
	}
	return b.String()
}

func (m *Model) handlePingMonitorKey(keyStr string) bool {
	if m.activePing() == nil {
		return false
	}
	switch keyStr {
	case "q", keyCtrlC, keyEsc:
		m.stopPingMonitor()
	}
	return true
}

// Rendering

var (
	pingHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	pingLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	pingValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	pingLossOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	pingLossWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	pingLossBad     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	pingFooterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	pingDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func (m Model) renderPingMonitor() string {
	ps := m.activePing()
	if ps == nil {
		return ""
	}

	width := m.width
	if width <= 0 {
		width = 80
	}

	var sb textbuf.Buffer
	sb.Reset(512)

	var hb textbuf.Buffer
	hb.Str("Ping ").Str(ps.target)
	if ps.interval != defaultPingMonitorInterval {
		hb.Str("  interval ").Str(ps.interval.String())
	}
	sb.Str(pingHeaderStyle.Render(hb.String()))
	sb.Byte('\n')
	sb.Byte('\n')

	s := &ps.stats
	if s.sent == 0 {
		sb.Str(pingDimStyle.Render("  waiting for data..."))
		sb.Byte('\n')
	} else {
		loss := s.loss()
		var tbL textbuf.Buffer
		lossText := tbL.Str(strconv.FormatFloat(loss, 'f', 1, 64)).Byte('%').String()
		var lossStyle lipgloss.Style
		switch {
		case loss == 0:
			lossStyle = pingLossOK
		case loss < 10:
			lossStyle = pingLossWarn
		default:
			lossStyle = pingLossBad
		}

		sb.Str("  ")
		sb.Str(pingLabelStyle.Render("Sent "))
		sb.Str(pingValueStyle.Render(textbuf.StringInt(int64(s.sent))))
		sb.Str("   ")
		sb.Str(pingLabelStyle.Render("Recv "))
		sb.Str(pingValueStyle.Render(textbuf.StringInt(int64(s.recv))))
		sb.Str("   ")
		sb.Str(pingLabelStyle.Render("Loss "))
		sb.Str(lossStyle.Render(lossText))
		sb.Byte('\n')

		if s.recv > 0 {
			sb.Byte('\n')
			sb.Str("  ")
			sb.Str(pingLabelStyle.Render("Last  "))
			sb.Str(pingValueStyle.Render(fmtMs(s.last)))
			sb.Str("   ")
			sb.Str(pingLabelStyle.Render("Min  "))
			sb.Str(pingValueStyle.Render(fmtMs(s.min)))
			sb.Str("   ")
			sb.Str(pingLabelStyle.Render("Avg  "))
			sb.Str(pingValueStyle.Render(fmtMs(s.avg())))
			sb.Byte('\n')

			sb.Str("  ")
			sb.Str(pingLabelStyle.Render("Max   "))
			sb.Str(pingValueStyle.Render(fmtMs(s.max)))
			sb.Str("   ")
			sb.Str(pingLabelStyle.Render("StDev "))
			sb.Str(pingValueStyle.Render(fmtMs(s.stddev())))
			sb.Byte('\n')
		}
	}

	sb.Byte('\n')
	footer := footerQuitHint
	gap := max(1, width-len(footer))
	var footBuf textbuf.Buffer
	footBuf.Str(footer)
	for range gap {
		footBuf.Byte(' ')
	}
	sb.Str(pingFooterStyle.Render(footBuf.String()))

	return sb.String()
}

func (m Model) renderPingPlain() string {
	ps := m.activePing()
	if ps == nil || ps.stats.sent == 0 {
		return ""
	}
	return renderPingStatsPlain(ps.target, &ps.stats)
}

func renderPingStatsPlain(target string, s *pingStats) string {
	var sb textbuf.Buffer
	sb.Reset(256)

	sb.Str("Ping ").Str(target).Byte('\n')
	sb.Str("  Sent ").Int(int64(s.sent))
	sb.Str("  Recv ").Int(int64(s.recv))
	sb.Str("  Loss ").Str(strconv.FormatFloat(s.loss(), 'f', 1, 64)).Str("%\n")

	if s.recv > 0 {
		sb.Str("  Min ").Str(fmtMs(s.min))
		sb.Str("  Avg ").Str(fmtMs(s.avg()))
		sb.Str("  Max ").Str(fmtMs(s.max))
		sb.Str("  StDev ").Str(fmtMs(s.stddev()))
		sb.Byte('\n')
	}

	return sb.String()
}

func (m Model) renderPingMonitorPiped() string {
	ps := m.activePingPiped()
	if ps == nil {
		return ""
	}

	width := m.width
	if width <= 0 {
		width = 80
	}

	var sb textbuf.Buffer
	sb.Reset(512)

	var hb textbuf.Buffer
	hb.Str("Ping ").Str(ps.target)
	sb.Str(pingHeaderStyle.Render(hb.String()))
	sb.Byte('\n')
	sb.Byte('\n')

	s := &ps.stats
	if s.sent == 0 {
		sb.Str(pingDimStyle.Render("  waiting for data..."))
	} else {
		sb.Str(renderPingStatsPlain(ps.target, s))
	}
	sb.Byte('\n')

	footer := footerQuitHint
	gap := max(1, width-len(footer))
	var footBuf textbuf.Buffer
	footBuf.Str(footer)
	for range gap {
		footBuf.Byte(' ')
	}
	sb.Str(pingFooterStyle.Render(footBuf.String()))

	return sb.String()
}

func fmtMs(v float64) string {
	var tb textbuf.Buffer
	return tb.Str(strconv.FormatFloat(v, 'f', 3, 64)).Str("ms").String()
}
