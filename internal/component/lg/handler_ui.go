// Design: docs/architecture/web-interface.md -- HTMX looking glass UI handlers
// Overview: server.go -- LG server and route registration
// Related: handler_api.go -- Birdwatcher REST API handlers

package lg

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

// maxSearchResults caps the number of routes returned by search queries.
const maxSearchResults = 1000

// maxFormBytes limits POST body size to prevent memory exhaustion.
const maxFormBytes = 4096

// handleUIPeers renders the peer dashboard page.
func (s *LGServer) handleUIPeers(w http.ResponseWriter, r *http.Request) {
	result := s.query("summary")
	zeData := parseJSON(result)

	peers := s.extractPeers(zeData)

	data := map[string]any{
		"Peers":     peers,
		"Title":     "BGP Peers",
		"ActiveTab": "peers",
		"Error":     engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "peers", data)
		return
	}
	s.renderPage(w, "peers", data)
}

// handleUILookupForm renders the route lookup form.
func (s *LGServer) handleUILookupForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":     "Route Lookup",
		"ActiveTab": "lookup",
		"Family":    "",
		"Prefix":    "",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "lookup", data)
		return
	}
	s.renderPage(w, "lookup", data)
}

// handleUILookup processes the route lookup form submission.
func (s *LGServer) handleUILookup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	prefix := r.FormValue("prefix")
	if prefix == "" {
		http.Error(w, "prefix required", http.StatusBadRequest)
		return
	}

	if !isValidPrefix(prefix) {
		http.Error(w, "invalid prefix", http.StatusBadRequest)
		return
	}

	family := r.FormValue("family")
	if family != "" && !isValidFamily(family) {
		http.Error(w, "invalid family", http.StatusBadRequest)
		return
	}

	cmd := fmt.Sprintf("rib show prefix %s", prefix)
	if family != "" {
		cmd = fmt.Sprintf("rib show prefix %s family %s", prefix, family)
	}

	result := s.query(cmd)
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	if len(routes) > maxSearchResults {
		routes = routes[:maxSearchResults]
	}

	data := map[string]any{
		"Title":     "Route Lookup",
		"ActiveTab": "lookup",
		"Prefix":    prefix,
		"Family":    family,
		"Routes":    routes,
		"Count":     len(routes),
		"Error":     engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "route_results", data)
		return
	}
	s.renderPage(w, "lookup", data)
}

// handleUISearchForm renders the unified search form.
func (s *LGServer) handleUISearchForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":      "Route Search",
		"ActiveTab":  "search",
		"SearchType": "",
		"Query":      "",
		"Family":     "",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "search", data)
		return
	}
	s.renderPage(w, "search", data)
}

// handleUISearch processes the unified search form.
func (s *LGServer) handleUISearch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	searchType := r.FormValue("type")
	query := r.FormValue("query")
	family := r.FormValue("family")

	if query == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}

	if family != "" && !isValidFamily(family) {
		http.Error(w, "invalid family", http.StatusBadRequest)
		return
	}

	var cmd string

	switch searchType {
	case "aspath":
		if !isValidASPathPattern(query) {
			http.Error(w, "invalid AS path pattern", http.StatusBadRequest)
			return
		}
		cmd = fmt.Sprintf("rib show aspath %s", query)
	case "community":
		if !isValidCommunity(query) {
			http.Error(w, "invalid community", http.StatusBadRequest)
			return
		}
		cmd = fmt.Sprintf("rib show community %s", query)
	default: // prefix
		if !isValidPrefix(query) {
			http.Error(w, "invalid prefix", http.StatusBadRequest)
			return
		}
		cmd = fmt.Sprintf("rib show prefix %s", query)
	}

	// Append family filter if specified.
	if family != "" {
		cmd = fmt.Sprintf("%s family %s", cmd, family)
	}

	result := s.query(cmd)
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	if len(routes) > maxSearchResults {
		routes = routes[:maxSearchResults]
	}

	data := map[string]any{
		"Title":      "Route Search",
		"ActiveTab":  "search",
		"SearchType": searchType,
		"Query":      query,
		"Family":     family,
		"Routes":     routes,
		"Count":      len(routes),
		"Error":      engineError(zeData),
	}

	// For prefix searches, include the prefix for the graph.
	if searchType == "" || searchType == "prefix" {
		data["Prefix"] = query
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "route_results", data)
		return
	}
	s.renderPage(w, "search", data)
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

	// Decorate peer AS name.
	if remoteAS := getStr(peerInfo, "remote-as"); remoteAS != "" {
		peerInfo["remote-as-name"] = s.resolveASN(remoteAS)
	}

	var routes []any
	result := s.query(fmt.Sprintf("peer %s rib show", address))
	zeData := parseJSON(result)

	if zeData != nil {
		if _, isErr := zeData["error"].(string); !isErr {
			routes = extractRoutes(zeData)
		}
	}

	data := map[string]any{
		"Title":     fmt.Sprintf("Routes from %s", address),
		"ActiveTab": "peers",
		"Address":   address,
		"Peer":      peerInfo,
		"Routes":    routes,
		"Count":     len(routes),
		"Error":     engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "peer_routes", data)
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

	// Disable write timeout for SSE (long-lived connection).
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		s.logger.Debug("SSE: cannot clear write deadline", "error", err)
	}

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

			peers := s.extractPeers(zeData)
			data := map[string]any{"Peers": peers}
			html := s.renderToString("peers_table_body", data)

			if html == "" {
				continue
			}

			// SSE requires each line prefixed with "data: ".
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

// extractPeers converts Ze peer summary data into template-friendly format
// and decorates ASN names.
func (s *LGServer) extractPeers(ze map[string]any) []map[string]any {
	if ze == nil {
		return nil
	}

	// Navigate into the "summary" envelope.
	summary, _ := ze["summary"].(map[string]any)
	if summary == nil {
		summary = ze
	}

	peers, _ := summary["peers"].([]any)
	var result []map[string]any

	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

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

		remoteAS := getStr(peer, "remote-as")

		entry := map[string]any{
			"Address":        address,
			"RemoteAS":       remoteAS,
			"RemoteASName":   s.resolveASN(remoteAS),
			"State":          getStr(peer, "state"),
			"Uptime":         getStr(peer, "uptime"),
			"RoutesReceived": received,
			"RoutesAccepted": accepted,
			"RoutesSent":     sent,
			"Description":    getStr(peer, "description"),
			"Name":           getStr(peer, "name"),
		}

		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		addrI, _ := result[i]["Address"].(string)
		addrJ, _ := result[j]["Address"].(string)
		ipI := net.ParseIP(addrI)
		ipJ := net.ParseIP(addrJ)
		if ipI == nil || ipJ == nil {
			return addrI < addrJ
		}
		return string(ipI.To16()) < string(ipJ.To16())
	})

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
