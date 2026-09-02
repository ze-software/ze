// Design: docs/architecture/web-interface.md -- LG template rendering
// Overview: server.go -- LG server and route registration
// Related: view.go -- the structs every component below takes

package lg

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// stateClass maps an FSM state onto the CSS class its peer row carries.
func stateClass(state string) string {
	switch state {
	case "established":
		return "state-up"
	case "idle", "active", "connect", "opensent", "openconfirm":
		return "state-down"
	}

	return "state-unknown"
}

// bmpStateClass maps a BMP peer's state onto its row class. BMP reports up or
// down and nothing else, so this is not stateClass with more cases.
func bmpStateClass(state string) string {
	if state == "up" {
		return "state-up"
	}

	return "state-down"
}

// formatASPath renders an AS path as space-separated ASNs.
func formatASPath(path []string) string {
	return textbuf.Join(path, " ")
}

// formatCommunities renders a community list as a comma-separated string.
func formatCommunities(values []string) string {
	return textbuf.Join(values, ", ")
}

// routeDetailURL is the HTMX target that expands one route row.
func routeDetailURL(r routeRow) string {
	var tb textbuf.Buffer

	return tb.Str("/lg/route/detail?prefix=").Str(r.Prefix).Str("&peer=").Str(r.PeerAddress).String()
}

// formatUptime converts a Go duration string like "6m10.766415s" to "6m 10s".
func formatUptime(v string) string {
	d, err := time.ParseDuration(v)
	if err != nil {
		return v
	}

	total := int(d.Seconds())
	if total < 0 {
		total = -total
	}

	days := total / 86400
	hours := (total % 86400) / 3600
	mins := (total % 3600) / 60
	secs := total % 60

	var b textbuf.Buffer
	switch {
	case days > 0:
		return b.Reset().Int(int64(days)).Str("d ").Int(int64(hours)).Str("h ").Int(int64(mins)).Str("m").String()
	case hours > 0:
		return b.Reset().Int(int64(hours)).Str("h ").Int(int64(mins)).Str("m ").Int(int64(secs)).Str("s").String()
	case mins > 0:
		return b.Reset().Int(int64(mins)).Str("m ").Int(int64(secs)).Str("s").String()
	}

	return b.Reset().Int(int64(secs)).Str("s").String()
}

// formatCount groups a count the looking glass computed itself.
func formatCount(n int) string {
	return formatNum(textbuf.StringInt(int64(n)))
}

// formatNum groups an engine-reported number with commas.
//
// The input is a string because the engine OMITS a count it cannot produce
// (handler_api.go, routeCountsAvailable). An absent count reaches here as an
// empty string and renders as an empty cell, so an operator never reads a zero
// Ze never sent. A value that is not a number is returned unchanged.
func formatNum(v string) string {
	if v == "" {
		return ""
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}

	n := int64(f)
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	digits := textbuf.StringInt(n)
	length := len(digits)

	var result textbuf.Buffer
	if negative {
		result.Byte('-')
	}

	for pos, c := range digits {
		if pos > 0 && (length-pos)%3 == 0 {
			result.Byte(',')
		}

		result.WriteRune(c)
	}

	return result.String()
}

// renderPage writes one full page. Layout and content render into a buffer
// before any byte reaches w, so a render error never produces a partial 200.
func (s *LGServer) renderPage(w http.ResponseWriter, v layoutView, content templ.Component) {
	var page bytes.Buffer
	if err := pageLayout(v, content).Render(context.Background(), &page); err != nil {
		s.logger.Warn("page render error", "title", v.Title, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)

		return
	}

	s.writeHTML(w, page.Bytes())
}

// renderFragment writes one HTMX fragment, with no layout around it. It
// buffers for the same reason renderPage does.
func (s *LGServer) renderFragment(w http.ResponseWriter, content templ.Component) {
	var buf bytes.Buffer
	if err := content.Render(context.Background(), &buf); err != nil {
		s.logger.Warn("fragment render error", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)

		return
	}

	s.writeHTML(w, buf.Bytes())
}

// renderToString renders a component for a caller that must post-process the
// markup. handleUIEvents is the only one: SSE prefixes every line.
func renderToString(content templ.Component) (string, error) {
	var buf textbuf.Buffer
	buf.Reset()
	if err := content.Render(context.Background(), &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// writeHTML sends rendered markup with the headers every looking-glass page
// carries.
func (s *LGServer) writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	if _, err := w.Write(body); err != nil {
		s.logger.Debug("write html failed", "error", err)
	}
}
