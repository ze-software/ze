// Design: docs/architecture/web-interface.md -- HTMX looking glass UI handlers
// Overview: server.go -- LG server and route registration
// Related: handler_api.go -- Birdwatcher REST API handlers

package lg

import (
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
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

// handleUIHelp renders the help page.
func (s *LGServer) handleUIHelp(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":     "Help",
		"ActiveTab": "help",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "help", data)
		return
	}
	s.renderPage(w, "help", data)
}

// handleUISearchForm renders the route search form.
func (s *LGServer) handleUISearchForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":     "Route Search",
		"ActiveTab": "search",
		"Prefix":    "",
		"ASPath":    "",
		"Community": "",
		"Family":    "",
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "search", data)
		return
	}
	s.renderPage(w, "search", data)
}

// handleUISearch processes the route search form with stackable filters.
// All filter fields are optional but at least one must be provided.
// Filters are combined: prefix + aspath + community + family.
func (s *LGServer) handleUISearch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	prefix := r.FormValue("prefix")
	aspath := r.FormValue("aspath")
	community := r.FormValue("community")
	fam := r.FormValue("family")

	if prefix == "" && aspath == "" && community == "" {
		s.renderSearchError(w, r, "enter at least one filter (prefix, AS path, or community)",
			prefix, aspath, community, fam)
		return
	}

	// Validate each provided filter.
	if prefix != "" && !isValidPrefix(prefix) {
		s.renderSearchError(w, r, "invalid prefix format", prefix, aspath, community, fam)
		return
	}
	if aspath != "" && !isValidASPathPattern(aspath) {
		s.renderSearchError(w, r, "invalid AS path pattern", prefix, aspath, community, fam)
		return
	}
	if community != "" && !isValidCommunity(community) {
		s.renderSearchError(w, r, "invalid community format (use ASN:value)", prefix, aspath, community, fam)
		return
	}
	if fam != "" && !isValidFamily(fam) {
		s.renderSearchError(w, r, "invalid address family", prefix, aspath, community, fam)
		return
	}

	// Build pipeline command with all provided filters.
	cmd := "rib show"
	if prefix != "" {
		cmd += " prefix " + prefix
	}
	if aspath != "" {
		cmd += " path " + aspath
	}
	if community != "" {
		cmd += " community " + community
	}
	if fam != "" {
		cmd += " family " + fam
	}

	result := s.query(cmd)
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	if len(routes) > maxSearchResults {
		routes = routes[:maxSearchResults]
	}

	data := map[string]any{
		"Title":     "Route Search",
		"ActiveTab": "search",
		"Prefix":    prefix,
		"ASPath":    aspath,
		"Community": community,
		"Family":    fam,
		"Routes":    routes,
		"Count":     len(routes),
		"Error":     engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "route_results", data)
		return
	}
	s.renderPage(w, "search", data)
}

// renderSearchError renders a validation error within the search results area.
func (s *LGServer) renderSearchError(w http.ResponseWriter, r *http.Request,
	errMsg, prefix, aspath, community, family string) {

	data := map[string]any{
		"Title":     "Route Search",
		"ActiveTab": "search",
		"Prefix":    prefix,
		"ASPath":    aspath,
		"Community": community,
		"Family":    family,
		"Error":     errMsg,
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "route_results", data)
		return
	}
	s.renderPage(w, "search", data)
}

// maxDisplayRoutes is the maximum number of individual routes shown in the browser.
// Larger route tables show a prefix-length summary instead.
const maxDisplayRoutes = 1024

// handleUIPeerRoutes renders a prefix-length summary for a peer's routes.
// Individual routes are only shown when the total count is <= maxDisplayRoutes.
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

	// Get prefix-length summary (fast, constant memory).
	result := s.query(fmt.Sprintf("peer %s rib show prefix-summary", address))
	zeData := parseJSON(result)

	totalCount := 0
	var prefixSummary []map[string]any

	if zeData != nil {
		if _, isErr := zeData["error"].(string); !isErr {
			totalCount = getInt(zeData, "count")
			prefixSummary = flattenPrefixSummary(zeData)
		}
	}

	data := map[string]any{
		"Title":         fmt.Sprintf("Routes from %s", address),
		"ActiveTab":     "peers",
		"Address":       address,
		"Peer":          peerInfo,
		"PrefixSummary": prefixSummary,
		"Count":         totalCount,
		"Error":         engineError(zeData),
	}

	// For small route tables, also fetch individual routes.
	if totalCount > 0 && totalCount <= maxDisplayRoutes {
		routeResult := s.query(fmt.Sprintf("peer %s rib show", address))
		routeData := parseJSON(routeResult)
		if routeData != nil {
			if _, isErr := routeData["error"].(string); !isErr {
				data["Routes"] = extractRoutes(routeData)
			}
		}
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, "peer_routes", data)
		return
	}
	s.renderPage(w, "peer_routes", data)
}

// handleUIPeerDownload streams all routes for a peer as gzip-compressed text.
func (s *LGServer) handleUIPeerDownload(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		http.Error(w, "peer address required", http.StatusBadRequest)
		return
	}

	if !isValidPeerName(address) {
		http.NotFound(w, r)
		return
	}

	result := s.query(fmt.Sprintf("peer %s rib show", address))
	zeData := parseJSON(result)

	if zeData == nil {
		http.Error(w, "engine unavailable", http.StatusServiceUnavailable)
		return
	}
	if errMsg, ok := zeData["error"].(string); ok {
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	routes := extractRoutes(zeData)

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"routes-%s.csv.gz\"", address))

	gz := gzip.NewWriter(w)
	defer func() { _ = gz.Close() }()

	if _, err := fmt.Fprintln(gz, "prefix,next-hop,as-path,origin,local-pref,med"); err != nil {
		return
	}

	for _, r := range routes {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if _, err := fmt.Fprintf(gz, "%s,%s,%s,%s,%s,%s\n",
			getStr(rm, "prefix"),
			getStr(rm, "next-hop"),
			csvQuote(formatASPathPlain(rm)),
			getStr(rm, "origin"),
			getStr(rm, "local-preference"),
			getStr(rm, "med"),
		); err != nil {
			return
		}
	}
}

// flattenPrefixSummary converts the nested prefix-summary JSON into a flat sorted list
// of {Family, Length, Count} entries for template rendering.
func flattenPrefixSummary(ze map[string]any) []map[string]any {
	summary, _ := ze["prefix-summary"].(map[string]any)
	if summary == nil {
		return nil
	}

	var rows []map[string]any
	for fam, byLen := range summary {
		lenMap, ok := byLen.(map[string]any)
		if !ok {
			continue
		}
		for length, count := range lenMap {
			rows = append(rows, map[string]any{
				"Family": fam,
				"Length": length,
				"Count":  count,
			})
		}
	}

	// Sort by family, then numerically by prefix length.
	sort.Slice(rows, func(i, j int) bool {
		fi, _ := rows[i]["Family"].(string)
		fj, _ := rows[j]["Family"].(string)
		if fi != fj {
			return fi < fj
		}
		si, _ := rows[i]["Length"].(string)
		sj, _ := rows[j]["Length"].(string)
		li, _ := strconv.Atoi(si)
		lj, _ := strconv.Atoi(sj)
		return li < lj
	})

	return rows
}

// formatASPathPlain returns the AS path as space-separated ASNs for text export.
func formatASPathPlain(route map[string]any) string {
	v, ok := route["as-path"].([]any)
	if !ok {
		s, _ := route["as-path"].(string)
		return s
	}
	parts := make([]string, len(v))
	for i, a := range v {
		parts[i] = fmt.Sprint(a)
	}
	return strings.Join(parts, " ")
}

// csvQuote wraps a value in double quotes if it contains commas or spaces.
func csvQuote(s string) string {
	if strings.ContainsAny(s, ", \"") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

// getInt returns the integer value for a key, or 0.
func getInt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
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

		// Route counts (NLRI-level) and UPDATE message counts (separate).
		received := getStr(peer, "routes-received")
		accepted := getStr(peer, "routes-accepted")
		sent := getStr(peer, "routes-sent")
		updatesRecv := getStr(peer, "updates-received")
		updatesSent := getStr(peer, "updates-sent")

		remoteAS := getStr(peer, "remote-as")

		entry := map[string]any{
			"Address":         address,
			"RemoteAS":        remoteAS,
			"RemoteASName":    s.resolveASN(remoteAS),
			"State":           getStr(peer, "state"),
			"Uptime":          getStr(peer, "uptime"),
			"RoutesReceived":  received,
			"RoutesAccepted":  accepted,
			"RoutesSent":      sent,
			"UpdatesReceived": updatesRecv,
			"UpdatesSent":     updatesSent,
			"Description":     getStr(peer, "description"),
			"Name":            getStr(peer, "name"),
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
// The RIB pipeline returns {"adj-rib-in": {"peer": [routes...]}, "adj-rib-out": {"peer": [routes...]}}.
// Legacy formats use "routes" or "prefixes" as top-level keys.
func extractRoutes(ze map[string]any) []any {
	if ze == nil {
		return nil
	}

	// Legacy format: flat route list.
	if routes, _ := ze["routes"].([]any); routes != nil {
		return routes
	}
	if routes, _ := ze["prefixes"].([]any); routes != nil {
		return routes
	}

	// RIB pipeline format: adj-rib-in/adj-rib-out grouped by peer.
	var result []any
	for _, ribKey := range []string{"adj-rib-in", "adj-rib-out"} {
		rib, _ := ze[ribKey].(map[string]any)
		for peer, peerRoutes := range rib {
			routes, _ := peerRoutes.([]any)
			for _, r := range routes {
				rm, ok := r.(map[string]any)
				if !ok {
					continue
				}
				if _, has := rm["peer-address"]; !has {
					rm["peer-address"] = peer
				}
				result = append(result, rm)
			}
		}
	}

	return result
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

// engineError returns an error message when the engine is unreachable (nil data)
// or when the response contains an "error" key from a dispatch failure.
func engineError(ze map[string]any) string {
	if ze == nil {
		return "BGP engine unavailable"
	}
	if errMsg, ok := ze["error"].(string); ok {
		return errMsg
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
