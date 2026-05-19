// Design: docs/architecture/api/commands.md -- monitor ping TUI model
// Related: model_traceroute.go -- same poll/drain/render pattern
// Related: model_monitor.go -- generic monitor session pattern

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

const pingDrainInterval = 50 * time.Millisecond

// PingFactory starts a continuous ping session. The returned channel
// receives per-reply results until the context is canceled.
type PingFactory func(ctx context.Context, target string, interval, timeout time.Duration) (<-chan map[string]any, context.CancelFunc, error)

type pingPollMsg struct{}
type pingPipedPollMsg struct{}

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

type pingPipedState struct {
	target   string
	interval time.Duration
	timeout  time.Duration
	stats    pingStats
	poller   PingFactory
	formatFn func(string) string
	logMode  bool
	replyCh  <-chan map[string]any
	cancel   context.CancelFunc
}

const (
	defaultPingMonitorInterval = time.Second
	defaultPingMonitorTimeout  = 5 * time.Second
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

func parsePingMonitorArgs(input string) (target string, interval, timeout time.Duration, errMsg string) {
	interval = defaultPingMonitorInterval
	timeout = defaultPingMonitorTimeout

	trimmed := strings.TrimSpace(input)
	after := strings.TrimPrefix(trimmed, "monitor ping ")
	args := strings.Fields(after)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "interval":
			if i+1 >= len(args) {
				return "", 0, 0, "interval requires a value"
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < 100*time.Millisecond || d > 30*time.Second {
				var b textbuf.Buffer
				b.Str("interval must be 100ms-30s, got ").Str(args[i+1])
				return "", 0, 0, b.String()
			}
			interval = d
			i++
		case "timeout":
			if i+1 >= len(args) {
				return "", 0, 0, "timeout requires a value"
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > 30*time.Second {
				var b textbuf.Buffer
				b.Str("timeout must be 1s-30s, got ").Str(args[i+1])
				return "", 0, 0, b.String()
			}
			timeout = d
			i++
		default:
			if target == "" {
				target = args[i]
			} else {
				return "", 0, 0, "unexpected argument: " + args[i] + " (use | for pipe operators)"
			}
		}
	}
	return target, interval, timeout, ""
}

// SetPingFactory sets the factory used to run continuous ping sessions.
func (m *Model) SetPingFactory(f PingFactory) {
	m.pingFactory = f
}

// IsPingMonitor returns true if the ping monitor is active.
func (m Model) IsPingMonitor() bool {
	return m.pingMonitor != nil
}

// IsPingMonitorPiped returns true if a piped ping session is active.
func (m Model) IsPingMonitorPiped() bool {
	return m.pingMonitorPiped != nil
}

func (m *Model) startPingMonitor(input string) tea.Cmd {
	if m.pingFactory == nil {
		m.statusMessage = "ping monitor not available (no daemon connection)"
		return nil
	}

	target, interval, timeout, argErr := parsePingMonitorArgs(input)
	if argErr != "" {
		m.statusMessage = "monitor ping: " + argErr
		return nil
	}
	if target == "" {
		m.statusMessage = "monitor ping: missing target address"
		return nil
	}

	ch, cancel, err := m.pingFactory(context.Background(), target, interval, timeout)
	if err != nil {
		m.statusMessage = "monitor ping: " + err.Error()
		return nil
	}

	m.pingMonitor = &pingState{
		target:   target,
		interval: interval,
		timeout:  timeout,
		stats:    pingStats{min: math.MaxFloat64},
		poller:   m.pingFactory,
		replyCh:  ch,
		cancel:   cancel,
	}

	return tea.Tick(pingDrainInterval, func(time.Time) tea.Msg { return pingPollMsg{} })
}

func (m *Model) startPingMonitorPiped(input string) tea.Cmd {
	if m.pingFactory == nil {
		m.statusMessage = "ping monitor not available (no daemon connection)"
		return nil
	}

	cmdStr, formatFn, logMode, pipeErr := command.ProcessPipesDetectLog(input)
	if pipeErr != "" {
		m.statusMessage = "pipe error: " + pipeErr
		return nil
	}
	target, interval, timeout, argErr := parsePingMonitorArgs(cmdStr)
	if argErr != "" {
		m.statusMessage = "monitor ping: " + argErr
		return nil
	}
	if target == "" {
		m.statusMessage = "monitor ping: missing target address"
		return nil
	}

	ch, cancel, err := m.pingFactory(context.Background(), target, interval, timeout)
	if err != nil {
		m.statusMessage = "monitor ping: " + err.Error()
		return nil
	}

	m.pingMonitorPiped = &pingPipedState{
		target:   target,
		interval: interval,
		timeout:  timeout,
		stats:    pingStats{min: math.MaxFloat64},
		poller:   m.pingFactory,
		formatFn: formatFn,
		logMode:  logMode,
		replyCh:  ch,
		cancel:   cancel,
	}

	if logMode {
		var hdr textbuf.Buffer
		hdr.Str("--- monitor ping ").Str(target).Str(" | log (Esc to stop) ---\n")
		m.outputBuf.WriteString(hdr.String())
		m.setViewportTextBottom(m.outputBuf.String())
	}
	m.statusMessage = "monitoring ping (Esc to stop)"

	return tea.Tick(pingDrainInterval, func(time.Time) tea.Msg { return pingPipedPollMsg{} })
}

func (m *Model) stopPingMonitor() {
	ps := m.pingMonitor
	if ps == nil {
		return
	}
	if ps.cancel != nil {
		ps.cancel()
	}

	lastRender := m.renderPingPlain()
	m.pingMonitor = nil

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
	ps := m.pingMonitorPiped
	if ps == nil {
		return
	}
	if ps.cancel != nil {
		ps.cancel()
	}

	lastOutput := renderPingStatsPlain(ps.target, &ps.stats)
	isLog := ps.logMode
	m.pingMonitorPiped = nil

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
	ps := m.pingMonitor
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
	ps := m.pingMonitorPiped
	if ps == nil || ps.replyCh == nil {
		return m, nil
	}

	replies, closed := drainPingReplies(ps.replyCh)
	for _, reply := range replies {
		applyPingReply(&ps.stats, reply)

		if ps.logMode {
			if ps.stats.sent == 1 || (ps.stats.sent-1)%25 == 0 {
				if ps.stats.sent > 1 {
					m.outputBuf.WriteString("\n")
				}
				m.outputBuf.WriteString("--- ")
				m.outputBuf.WriteString(ps.target)
				m.outputBuf.WriteString(" ---\n")
			}
			line := formatPingReplyLine(reply)
			if ps.formatFn != nil {
				line = ps.formatFn(line)
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
	if m.pingMonitor == nil {
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
	ps := m.pingMonitor
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
		lossText := strconv.FormatFloat(loss, 'f', 1, 64) + "%"
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
		sb.Str(pingValueStyle.Render(textbuf.Int(int64(s.sent))))
		sb.Str("   ")
		sb.Str(pingLabelStyle.Render("Recv "))
		sb.Str(pingValueStyle.Render(textbuf.Int(int64(s.recv))))
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
	ps := m.pingMonitor
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
	ps := m.pingMonitorPiped
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
	return strconv.FormatFloat(v, 'f', 3, 64) + "ms"
}
