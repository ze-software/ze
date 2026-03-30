// Design: docs/architecture/web-interface.md -- HTMX looking glass UI handlers
// Overview: server.go -- LG server and route registration
// Related: handler_api.go -- Birdwatcher REST API handlers

package lg

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// maxSearchResults caps the number of routes returned by search queries.
const maxSearchResults = 1000

// handleUIPeers renders the peer dashboard page.
func (s *LGServer) handleUIPeers(w http.ResponseWriter, r *http.Request) {
	result := s.query("summary")
	zeData := parseJSON(result)

	peers := extractPeers(zeData)

	data := map[string]any{
		"Peers": peers,
		"Title": "BGP Peers",
		"Error": engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "peers_content", data)
		return
	}
	s.renderPage(w, "peers", data)
}

// handleUILookupForm renders the route lookup form.
func (s *LGServer) handleUILookupForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "Route Lookup",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "lookup_form", data)
		return
	}
	s.renderPage(w, "lookup", data)
}

// searchParams defines a route search query.
type searchParams struct {
	formField string              // form field name to read
	validate  func(string) bool   // validation function
	errEmpty  string              // error when field is empty
	errBad    string              // error when validation fails
	command   func(string) string // builds the dispatch command
	title     string              // page title
	dataKey   string              // extra data key (e.g., "Prefix", "Pattern")
	page      string              // full page template name
}

// handleSearch is the common handler for route search operations.
func (s *LGServer) handleSearch(w http.ResponseWriter, r *http.Request, p searchParams) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	value := r.FormValue(p.formField)
	if value == "" {
		http.Error(w, p.errEmpty, http.StatusBadRequest)
		return
	}

	if !p.validate(value) {
		http.Error(w, p.errBad, http.StatusBadRequest)
		return
	}

	result := s.query(p.command(value))
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	if len(routes) > maxSearchResults {
		routes = routes[:maxSearchResults]
	}

	data := map[string]any{
		"Title":   p.title,
		p.dataKey: value,
		"Routes":  routes,
		"Count":   len(routes),
		"Error":   engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "route_results", data)
		return
	}
	s.renderPage(w, p.page, data)
}

// handleUILookup processes the route lookup form submission.
func (s *LGServer) handleUILookup(w http.ResponseWriter, r *http.Request) {
	s.handleSearch(w, r, searchParams{
		formField: "prefix",
		validate:  isValidPrefix,
		errEmpty:  "prefix required",
		errBad:    "invalid prefix",
		command:   func(v string) string { return fmt.Sprintf("rib show prefix %s", v) },
		title:     "Route Lookup",
		dataKey:   "Prefix",
		page:      "lookup",
	})
}

// handleUIASPathSearchForm renders the AS path search form.
func (s *LGServer) handleUIASPathSearchForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "AS Path Search",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "aspath_form", data)
		return
	}
	s.renderPage(w, "search_aspath", data)
}

// handleUIASPathSearch processes the AS path search form.
func (s *LGServer) handleUIASPathSearch(w http.ResponseWriter, r *http.Request) {
	s.handleSearch(w, r, searchParams{
		formField: "pattern",
		validate:  isValidASPathPattern,
		errEmpty:  "AS path pattern required",
		errBad:    "invalid AS path pattern",
		command:   func(v string) string { return fmt.Sprintf("rib show aspath %s", v) },
		title:     "AS Path Search",
		dataKey:   "Pattern",
		page:      "search_aspath",
	})
}

// handleUICommunitySearchForm renders the community search form.
func (s *LGServer) handleUICommunitySearchForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "Community Search",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "community_form", data)
		return
	}
	s.renderPage(w, "search_community", data)
}

// handleUICommunitySearch processes the community search form.
func (s *LGServer) handleUICommunitySearch(w http.ResponseWriter, r *http.Request) {
	s.handleSearch(w, r, searchParams{
		formField: "community",
		validate:  isValidCommunity,
		errEmpty:  "community required",
		errBad:    "invalid community",
		command:   func(v string) string { return fmt.Sprintf("rib show community %s", v) },
		title:     "Community Search",
		dataKey:   "Community",
		page:      "search_community",
	})
}

// handleUIPeerRoutes renders routes from a specific peer.
func (s *LGServer) handleUIPeerRoutes(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		http.Error(w, "peer address required", http.StatusBadRequest)
		return
	}

	if !isValidPeerName(address) {
		http.NotFound(w, r)
		return
	}

	// Get peer info first to confirm the peer exists.
	peerResult := s.query("summary")
	peerData := parseJSON(peerResult)
	peerInfo := findPeer(peerData, address)

	if peerInfo == nil {
		http.NotFound(w, r)
		return
	}

	var routes []any
	result := s.query(fmt.Sprintf("peer %s rib show received", address))
	zeData := parseJSON(result)

	if zeData != nil {
		if _, isErr := zeData["error"].(string); !isErr {
			routes = extractRoutes(zeData)
		}
	}

	data := map[string]any{
		"Title":   fmt.Sprintf("Routes from %s", address),
		"Address": address,
		"Peer":    peerInfo,
		"Routes":  routes,
		"Count":   len(routes),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "peer_routes_content", data)
		return
	}
	s.renderPage(w, "peer_routes", data)
}

// handleUIRouteDetail renders expanded route detail for a prefix and peer.
func (s *LGServer) handleUIRouteDetail(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	peer := r.URL.Query().Get("peer")

	if prefix == "" {
		http.Error(w, "prefix required", http.StatusBadRequest)
		return
	}

	if !isValidPrefix(prefix) {
		http.Error(w, "invalid prefix", http.StatusBadRequest)
		return
	}

	if peer != "" && !isValidPeerName(peer) {
		http.Error(w, "invalid peer", http.StatusBadRequest)
		return
	}

	result := s.query(fmt.Sprintf("rib show prefix %s", prefix))
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	// Find the specific route from the given peer.
	var route map[string]any
	for _, r := range routes {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if peer == "" || getStr(rm, "peer-address") == peer {
			route = rm
			break
		}
	}

	if route == nil && len(routes) > 0 {
		if rm, ok := routes[0].(map[string]any); ok {
			route = rm
		}
	}

	data := map[string]any{
		"Route":  route,
		"Prefix": prefix,
	}

	s.renderFragment(w, "route_detail", data)
}

// handleUIEvents serves SSE events for live peer state updates.
func (s *LGServer) handleUIEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Limit concurrent SSE connections to prevent resource exhaustion.
	if s.sseClients.Add(1) > maxSSEClients {
		s.sseClients.Add(-1)
		http.Error(w, "too many SSE clients", http.StatusServiceUnavailable)
		return
	}
	defer s.sseClients.Add(-1)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	ctx := r.Context()

	// Poll peer state every 5 seconds and push updates via SSE.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result := s.query("summary")
			zeData := parseJSON(result)

			if zeData == nil {
				// Engine unavailable; send error event.
				if _, err := fmt.Fprint(w, "event: peer-error\ndata: engine unavailable\n\n"); err != nil {
					return
				}
				flusher.Flush()
				continue
			}

			peers := extractPeers(zeData)
			data := map[string]any{"Peers": peers}
			html := s.renderToString("peers_table_body", data)

			if html == "" {
				continue
			}

			// SSE requires each line prefixed with "data: ".
			// Handle \r\n and bare \r in addition to \n.
			sseData := strings.ReplaceAll(html, "\r\n", "\n")
			sseData = strings.ReplaceAll(sseData, "\r", "\n")
			sseData = strings.TrimRight(sseData, "\n")
			sseData = strings.ReplaceAll(sseData, "\n", "\ndata: ")

			if _, err := fmt.Fprintf(w, "event: peer-update\ndata: %s\n\n", sseData); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleAssets serves static CSS and JS files. Unknown paths return 404.
func (s *LGServer) handleAssets(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/lg/assets/")

	content, contentType := resolveAsset(path)
	if content == "" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if _, err := fmt.Fprint(w, content); err != nil {
		lgLogger.Debug("write asset failed", "path", path, "error", err)
	}
}

// resolveAsset returns the content and content-type for a known asset, or empty strings for unknown.
func resolveAsset(path string) (content, contentType string) {
	if path == "style.css" {
		return lgStyleCSS, "text/css"
	}
	if path == "htmx.min.js" {
		return htmxMinJS, "application/javascript"
	}
	return "", ""
}

// extractPeers converts Ze peer summary data into template-friendly format.
// The summary command returns {"summary": {"peers": [...], ...}}.
func extractPeers(ze map[string]any) []map[string]any {
	if ze == nil {
		return nil
	}

	// Navigate into the "summary" envelope.
	summary, _ := ze["summary"].(map[string]any)
	if summary == nil {
		// Fall back to top-level "peers" for direct array responses.
		summary = ze
	}

	peers, _ := summary["peers"].([]any)
	var result []map[string]any

	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		// The summary handler uses "address"; map to template field "Address".
		address := getStr(peer, "address")
		if address == "" {
			address = getStr(peer, "peer-address")
		}

		// Route counts: prefer NLRI-level if available, fall back to UPDATE message counts.
		received := getStr(peer, "routes-received")
		if received == "" {
			received = getStr(peer, "updates-received")
		}
		accepted := getStr(peer, "routes-accepted")
		sent := getStr(peer, "routes-sent")
		if sent == "" {
			sent = getStr(peer, "updates-sent")
		}

		result = append(result, map[string]any{
			"Address":        address,
			"RemoteAS":       getStr(peer, "remote-as"),
			"State":          getStr(peer, "state"),
			"Uptime":         getStr(peer, "uptime"),
			"RoutesReceived": received,
			"RoutesAccepted": accepted,
			"RoutesSent":     sent,
			"Description":    getStr(peer, "description"),
			"Name":           getStr(peer, "name"),
		})
	}

	return result
}

// extractRoutes converts Ze route data into a slice of route maps.
func extractRoutes(ze map[string]any) []any {
	if ze == nil {
		return nil
	}

	routes, _ := ze["routes"].([]any)
	if routes == nil {
		routes, _ = ze["prefixes"].([]any)
	}

	return routes
}

// findPeer finds a specific peer in the summary data by address.
func findPeer(ze map[string]any, address string) map[string]any {
	if ze == nil {
		return nil
	}

	// Navigate into the "summary" envelope.
	summary, _ := ze["summary"].(map[string]any)
	if summary == nil {
		summary = ze
	}

	peers, _ := summary["peers"].([]any)
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}
		addr := getStr(peer, "address")
		if addr == "" {
			addr = getStr(peer, "peer-address")
		}
		if addr == address || getStr(peer, "name") == address {
			return peer
		}
	}

	return nil
}

// engineError returns an error message when the engine is unreachable (nil data),
// or empty string when data is available.
func engineError(ze map[string]any) string {
	if ze == nil {
		return "BGP engine unavailable"
	}
	return ""
}

// isHTMXRequest checks if the request was made by HTMX (HX-Request header).
func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// isValidASPathPattern checks that an AS path pattern contains only safe characters.
func isValidASPathPattern(pattern string) bool {
	if pattern == "" || len(pattern) > 200 {
		return false
	}
	for _, c := range pattern {
		if (c < '0' || c > '9') && c != ' ' && c != '.' && c != '^' && c != '$' &&
			c != '|' && c != '(' && c != ')' && c != '[' && c != ']' &&
			c != '*' && c != '+' && c != '?' && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// isValidCommunity checks that a community string contains only safe characters.
func isValidCommunity(community string) bool {
	if community == "" || len(community) > 100 {
		return false
	}
	for _, c := range community {
		if (c < '0' || c > '9') && c != ':' && c != ' ' {
			return false
		}
	}
	return true
}
