// Design: docs/architecture/web-interface.md -- LG template rendering
// Overview: server.go -- LG server and route registration

package lg

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// templates holds all parsed HTML templates for the LG.
var templates *template.Template

func init() {
	funcMap := template.FuncMap{
		"stateClass": func(state string) string {
			switch state {
			case "established":
				return "state-up"
			case "idle", "active", "connect", "opensent", "openconfirm":
				return "state-down"
			}
			return "state-unknown"
		},
		"formatNum": formatNumCommas,
		"formatASPath": func(v any) string {
			arr, ok := v.([]any)
			if !ok {
				return ""
			}
			var parts []string
			for _, a := range arr {
				parts = append(parts, fmt.Sprintf("%v", a))
			}
			return joinStrings(parts, " ")
		},
		"formatCommunities": func(v any) string {
			arr, ok := v.([]any)
			if !ok {
				return ""
			}
			var parts []string
			for _, a := range arr {
				parts = append(parts, fmt.Sprintf("%v", a))
			}
			return joinStrings(parts, ", ")
		},
	}

	templates = template.Must(template.New("").Option("missingkey=zero").Funcs(funcMap).Parse(allTemplates))
}

// joinStrings joins a string slice with a separator.
func joinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// formatNumCommas formats a value as an integer with comma separators.
// Handles float64, int, int64, and string inputs. Returns the value as-is for non-numeric types.
func formatNumCommas(v any) string {
	var n int64

	switch val := v.(type) {
	case float64:
		n = int64(val)
	case int:
		n = int64(val)
	case int64:
		n = val
	case string:
		// Try to parse numeric strings.
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err != nil {
			return val
		}
		n = int64(f)
	case nil:
		return ""
	case bool:
		// Not a number; render as-is.
		return fmt.Sprintf("%v", val)
	}

	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	// Format with commas by grouping digits in threes.
	s := fmt.Sprintf("%d", n)
	length := len(s)

	var result strings.Builder
	if negative {
		result.WriteByte('-')
	}

	for i, c := range s {
		if i > 0 && (length-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}

	return result.String()
}

// renderPage renders a full HTML page with layout wrapper.
// Both inner content and layout are rendered to buffers before writing to w,
// so a template error never produces a partial 200 response.
func (s *LGServer) renderPage(w http.ResponseWriter, name string, data map[string]any) {
	var content bytes.Buffer
	if err := templates.ExecuteTemplate(&content, name, data); err != nil {
		s.logger.Warn("template render error", "template", name, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	layoutData := map[string]any{
		"Title":   data["Title"],
		"Content": template.HTML(content.String()), //nolint:gosec // pre-rendered trusted template output
	}

	var page bytes.Buffer
	if err := templates.ExecuteTemplate(&page, "layout", layoutData); err != nil {
		s.logger.Warn("layout render error", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(page.Bytes()); err != nil {
		s.logger.Debug("write page failed", "error", err)
	}
}

// renderFragment renders an HTML fragment (no layout wrapper).
// Rendered to buffer first to avoid partial 200 responses on template errors.
func (s *LGServer) renderFragment(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.logger.Warn("fragment render error", "template", name, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(buf.Bytes()); err != nil {
		s.logger.Debug("write fragment failed", "error", err)
	}
}

// renderToString renders a template to a string.
func (s *LGServer) renderToString(name string, data any) string {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.logger.Warn("render to string error", "template", name, "error", err)
		return ""
	}
	return buf.String()
}

// allTemplates contains all LG HTML templates as a single string.
// Using inline templates avoids embed complexity for a focused component.
const allTemplates = `
{{define "layout"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} - Ze Looking Glass</title>
<link rel="stylesheet" href="/lg/assets/style.css">
<script src="/lg/assets/htmx.min.js"></script>
</head>
<body>
<header>
<nav>
<a href="/lg/peers" class="nav-brand">Ze Looking Glass</a>
<a href="/lg/peers">Peers</a>
<a href="/lg/lookup">Lookup</a>
<a href="/lg/search/aspath">AS Path</a>
<a href="/lg/search/community">Community</a>
</nav>
</header>
<main id="content">{{.Content}}</main>
</body>
</html>{{end}}

{{define "error_banner"}}{{if .Error}}<div class="error-banner">{{.Error}}</div>{{end}}{{end}}

{{define "peers"}}
<h1>BGP Peers</h1>
{{template "error_banner" .}}
<div id="peers-table" hx-ext="sse" sse-connect="/lg/events" sse-swap="peer-update">
{{template "peers_content" .}}
</div>
{{end}}

{{define "peers_content"}}
<table class="peers-table">
<thead>
<tr>
<th>Peer</th><th>Remote AS</th><th>State</th><th>Uptime</th>
<th>Received</th><th>Accepted</th><th>Sent</th><th>Description</th>
</tr>
</thead>
<tbody>{{template "peers_table_body" .}}</tbody>
</table>
{{end}}

{{define "peers_table_body"}}{{range .Peers}}
<tr class="{{stateClass .State}}">
<td><a href="/lg/peer/{{.Address}}" hx-get="/lg/peer/{{.Address}}" hx-target="#content" hx-push-url="true">{{.Address}}</a></td>
<td>{{.RemoteAS}}</td>
<td class="state">{{.State}}</td>
<td>{{.Uptime}}</td>
<td>{{formatNum .RoutesReceived}}</td>
<td>{{formatNum .RoutesAccepted}}</td>
<td>{{formatNum .RoutesSent}}</td>
<td>{{.Description}}</td>
</tr>{{end}}
{{end}}

{{define "lookup"}}
<h1>Route Lookup</h1>
{{template "lookup_form" .}}
<div id="results">
{{if .Routes}}{{template "route_results" .}}{{end}}
</div>
{{end}}

{{define "lookup_form"}}
<form hx-post="/lg/lookup" hx-target="#results" class="search-form">
<label for="prefix">Prefix or IP address:</label>
<input type="text" id="prefix" name="prefix" value="{{.Prefix}}" placeholder="10.0.0.0/24 or 10.0.0.1" required>
<button type="submit">Search</button>
</form>
{{end}}

{{define "route_results"}}
{{template "error_banner" .}}
<p>{{.Count}} routes found{{if .Prefix}} for {{.Prefix}}{{end}}</p>
{{if .Prefix}}<a href="/lg/graph?prefix={{.Prefix}}" hx-get="/lg/graph?prefix={{.Prefix}}" hx-target="#graph-container">Show topology</a>{{end}}
<div id="graph-container"></div>
<table class="route-table">
<thead>
<tr>
<th>Prefix</th><th>Next Hop</th><th>AS Path</th><th>Origin</th>
<th>Local Pref</th><th>MED</th><th>Peer</th>
</tr>
</thead>
<tbody>
{{range .Routes}}{{$route := .}}
<tr hx-get="/lg/route/detail?prefix={{index . "prefix"}}&amp;peer={{index . "peer-address"}}" hx-target="next tr" hx-swap="afterend">
<td>{{index . "prefix"}}</td>
<td>{{index . "next-hop"}}</td>
<td>{{formatASPath (index . "as-path")}}</td>
<td>{{index . "origin"}}</td>
<td>{{formatNum (index . "local-preference")}}</td>
<td>{{formatNum (index . "med")}}</td>
<td>{{index . "peer-address"}}</td>
</tr>{{end}}
</tbody>
</table>
{{end}}

{{define "route_detail"}}
<tr class="route-detail">
<td colspan="7">
<div class="detail-panel">
{{if .Route}}
<h3>Route Detail: {{.Prefix}}</h3>
<dl>
<dt>Prefix</dt><dd>{{index .Route "prefix"}}</dd>
<dt>Next Hop</dt><dd>{{index .Route "next-hop"}}</dd>
<dt>Origin</dt><dd>{{index .Route "origin"}}</dd>
<dt>AS Path</dt><dd>{{formatASPath (index .Route "as-path")}}</dd>
<dt>Local Preference</dt><dd>{{formatNum (index .Route "local-preference")}}</dd>
<dt>MED</dt><dd>{{formatNum (index .Route "med")}}</dd>
<dt>Communities</dt><dd>{{formatCommunities (index .Route "community")}}</dd>
<dt>Large Communities</dt><dd>{{formatCommunities (index .Route "large-community")}}</dd>
<dt>Extended Communities</dt><dd>{{formatCommunities (index .Route "extended-community")}}</dd>
<dt>Peer</dt><dd>{{index .Route "peer-address"}}</dd>
</dl>
{{else}}
<p>Route not found</p>
{{end}}
</div>
</td>
</tr>
{{end}}

{{define "search_aspath"}}
<h1>AS Path Search</h1>
{{template "aspath_form" .}}
<div id="results">
{{if .Routes}}{{template "route_results" .}}{{end}}
</div>
{{end}}

{{define "aspath_form"}}
<form hx-post="/lg/search/aspath" hx-target="#results" class="search-form">
<label for="pattern">AS path pattern (regex or space-separated):</label>
<input type="text" id="pattern" name="pattern" value="{{.Pattern}}" placeholder="64500 64501" required>
<button type="submit">Search</button>
</form>
{{end}}

{{define "search_community"}}
<h1>Community Search</h1>
{{template "community_form" .}}
<div id="results">
{{if .Routes}}{{template "route_results" .}}{{end}}
</div>
{{end}}

{{define "community_form"}}
<form hx-post="/lg/search/community" hx-target="#results" class="search-form">
<label for="community">Community (N:N, N:N:N, or extended):</label>
<input type="text" id="community" name="community" value="{{.Community}}" placeholder="65000:100" required>
<button type="submit">Search</button>
</form>
{{end}}

{{define "peer_routes"}}
<h1>{{.Title}}</h1>
{{template "peer_routes_content" .}}
{{end}}

{{define "peer_routes_content"}}
{{if .Peer}}
<div class="peer-info">
<span class="{{stateClass (index .Peer "state")}}">{{index .Peer "state"}}</span>
<span>AS {{index .Peer "remote-as"}}</span>
<span>{{index .Peer "description"}}</span>
</div>
{{end}}
<p>{{.Count}} routes</p>
<table class="route-table">
<thead>
<tr>
<th>Prefix</th><th>Next Hop</th><th>AS Path</th><th>Origin</th>
<th>Local Pref</th><th>MED</th>
</tr>
</thead>
<tbody>
{{range .Routes}}
<tr hx-get="/lg/route/detail?prefix={{index . "prefix"}}&amp;peer={{index . "peer-address"}}" hx-target="next tr" hx-swap="afterend">
<td>{{index . "prefix"}}</td>
<td>{{index . "next-hop"}}</td>
<td>{{formatASPath (index . "as-path")}}</td>
<td>{{index . "origin"}}</td>
<td>{{formatNum (index . "local-preference")}}</td>
<td>{{formatNum (index . "med")}}</td>
</tr>{{end}}
</tbody>
</table>
{{end}}
`
