// Design: docs/architecture/web-interface.md -- HTMX looking glass UI handlers
// Overview: server.go -- LG server and route registration
// Related: handler_api.go -- Birdwatcher REST API handlers

package lg

import (
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxSearchResults caps the number of routes returned by search queries.
const maxSearchResults = 1000

// maxFormBytes limits POST body size to prevent memory exhaustion.
const maxFormBytes = 4096

// handleUIPeers renders the peer dashboard page.
func (s *LGServer) handleUIPeers(w http.ResponseWriter, r *http.Request) {
	result := s.query("show bgp summary")
	zeData := parseJSON(result)

	bmpResult := s.query("show bmp peers")
	bmpData := parseJSON(bmpResult)

	v := peersView{
		layoutView: layoutView{Title: "Peers", ActiveTab: "peers", Page: pgPeersPage},
		Peers:      s.extractPeers(zeData),
		BMPPeers:   s.extractBMPPeers(bmpData),
		Error:      engineError(zeData),
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, peersPage(v))
		return
	}
	s.renderPage(w, v.layoutView, peersPage(v))
}

// handleUIHelp renders the help page.
func (s *LGServer) handleUIHelp(w http.ResponseWriter, r *http.Request) {
	if isHTMXRequest(r) {
		s.renderFragment(w, helpPage())
		return
	}
	s.renderPage(w, layoutView{Title: "Help", ActiveTab: "help", Page: pgHelpPage}, helpPage())
}

// handleUISearchForm renders the route search form.
func (s *LGServer) handleUISearchForm(w http.ResponseWriter, r *http.Request) {
	v := searchView{layoutView: searchLayout}

	if isHTMXRequest(r) {
		s.renderFragment(w, searchPage(v))
		return
	}
	s.renderPage(w, v.layoutView, searchPage(v))
}

// searchLayout is the chrome every search response carries.
var searchLayout = layoutView{Title: "Route Search", ActiveTab: "search", Page: pgSearchPage}

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
	var tb textbuf.Buffer
	tb.Str("show bgp rib")
	if prefix != "" {
		tb.Str(" prefix ").Str(prefix)
	}
	if aspath != "" {
		tb.Str(" path ").Str(aspath)
	}
	if community != "" {
		tb.Str(" community ").Str(community)
	}
	if fam != "" {
		tb.Str(" family ").Str(fam)
	}

	result := s.query(tb.String())
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	if len(routes) > maxSearchResults {
		routes = routes[:maxSearchResults]
	}

	rows := routeRows(routes)

	v := searchView{
		layoutView: searchLayout,
		Prefix:     prefix,
		ASPath:     aspath,
		Community:  community,
		Family:     fam,
		Routes:     rows,
		Count:      len(rows),
		Error:      engineError(zeData),
	}

	s.renderSearch(w, r, v)
}

// renderSearchError renders a validation error within the search results area.
func (s *LGServer) renderSearchError(w http.ResponseWriter, r *http.Request,
	errMsg, prefix, aspath, community, family string) {

	s.renderSearch(w, r, searchView{
		layoutView: searchLayout,
		Prefix:     prefix,
		ASPath:     aspath,
		Community:  community,
		Family:     family,
		Error:      errMsg,
	})
}

// renderSearch answers a search request. HTMX swaps the results panel alone.
// A plain browser request gets the whole page.
//
// searchPage renders the results panel when there are routes OR an error. A
// validation error therefore reaches the operator on both paths. Until
// 2026-08-14 the page rendered the panel only when there were routes. The
// error banner lives inside that panel, so a bad prefix answered HTTP 200 with
// an empty form (plan/journal/silent-fall-through.md).
func (s *LGServer) renderSearch(w http.ResponseWriter, r *http.Request, v searchView) {
	if isHTMXRequest(r) {
		s.renderFragment(w, routeResults(v))
		return
	}

	s.renderPage(w, v.layoutView, searchPage(v))
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
	peerResult := s.query("show bgp summary")
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

	// Get the prefix-length histogram (fast, constant memory).
	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib histogram peer ").Str(address).String())
	zeData := parseJSON(result)

	totalCount := 0
	var histogram []histogramRow

	if zeData != nil {
		if _, isErr := zeData["error"].(string); !isErr {
			totalCount = getInt(zeData, "count")
			histogram = flattenHistogram(zeData)
		}
	}

	v := peerRoutesView{
		layoutView: layoutView{
			Title:     tb.Reset().Str("Routes from ").Str(address).String(),
			ActiveTab: "peers",
			Page:      pgPeerRoutesPage,
		},
		Address:   address,
		Peer:      peerInfoFrom(peerInfo),
		Histogram: histogram,
		Error:     engineError(zeData),
	}

	// For small route tables, also fetch individual routes.
	if totalCount > 0 && totalCount <= maxDisplayRoutes {
		routeResult := s.query(tb.Reset().Str("show bgp rib peer ").Str(address).String())
		routeData := parseJSON(routeResult)
		if routeData != nil {
			if _, isErr := routeData["error"].(string); !isErr {
				v.Routes = routeRows(extractRoutes(routeData))
			}
		}
	}

	if isHTMXRequest(r) {
		s.renderFragment(w, peerRoutesPage(v))
		return
	}
	s.renderPage(w, v.layoutView, peerRoutesPage(v))
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

	var tb2 textbuf.Buffer
	result := s.query(tb2.Str("show bgp rib peer ").Str(address).String())
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
		"attachment; filename=\"routes-"+address+".csv.gz\"")

	gz := gzip.NewWriter(w)
	defer func() { _ = gz.Close() }()

	if _, err := fmt.Fprintln(gz, "prefix,next-hop,as-path,origin,local-pref,med"); err != nil { //nolint:errcheck // output
		return
	}

	for _, r := range routes {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if _, err := fmt.Fprintf(gz, "%s,%s,%s,%s,%s,%s\n", //nolint:errcheck // output
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

// flattenHistogram converts the nested histogram JSON into a flat
// sorted list of rows the histogram table renders.
func flattenHistogram(ze map[string]any) []histogramRow {
	histogram, _ := ze["histogram"].(map[string]any)
	if histogram == nil {
		return nil
	}

	var rows []histogramRow
	for fam, byLen := range histogram {
		lenMap, ok := byLen.(map[string]any)
		if !ok {
			continue
		}
		for length, count := range lenMap {
			rows = append(rows, histogramRow{
				Family: fam,
				Length: length,
				Count:  scalarString(count),
			})
		}
	}

	// Sort by family, then numerically by prefix length.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Family != rows[j].Family {
			return rows[i].Family < rows[j].Family
		}
		li, _ := strconv.Atoi(rows[i].Length)
		lj, _ := strconv.Atoi(rows[j].Length)

		return li < lj
	})

	return rows
}

// peerInfoFrom types the peer header above a peer's routes. It returns nil for
// a peer the summary does not hold, which is the nil branch
// peerRoutesContent renders.
func peerInfoFrom(peer map[string]any) *peerInfoRow {
	if peer == nil {
		return nil
	}

	return &peerInfoRow{
		State:        getStr(peer, "state"),
		RemoteAS:     getStr(peer, "remote-as"),
		RemoteASName: getStr(peer, "remote-as-name"),
		Description:  getStr(peer, "description"),
	}
}

// routeRows types the decoded RIB JSON extractRoutes yields. The API handlers
// keep the untyped form, because they serialize it back to JSON. Only the
// browser view is typed.
func routeRows(routes []any) []routeRow {
	if len(routes) == 0 {
		return nil
	}

	rows := make([]routeRow, 0, len(routes))
	for _, r := range routes {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}

		rows = append(rows, routeRow{
			Prefix:              getStr(rm, "prefix"),
			NextHop:             getStr(rm, "next-hop"),
			ASPath:              scalarList(rm["as-path"]),
			Origin:              getStr(rm, "origin"),
			LocalPreference:     getStr(rm, "local-preference"),
			MED:                 getStr(rm, "med"),
			PeerAddress:         getStr(rm, "peer-address"),
			Best:                getBool(rm, "best"),
			Communities:         scalarList(rm["community"]),
			LargeCommunities:    scalarList(rm["large-community"]),
			ExtendedCommunities: scalarList(rm["extended-community"]),
		})
	}

	return rows
}

// scalarList renders a decoded JSON array as strings. A value that is not an
// array gives no entries. That is what the AS path and the community column
// showed for a scalar before the port.
func scalarList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(arr))
	for _, a := range arr {
		out = append(out, scalarString(a))
	}

	return out
}

// scalarString renders one decoded JSON scalar.
//
// A JSON number decodes to float64. The %v verb prints a large one in
// exponent form, so AS 4200000000 reached the browser as "4.2e+09" in every
// AS path until 2026-08-14. FormatFloat with precision -1 prints the digits.
// That is what getStr already did for a scalar attribute.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return textbuf.StringInt(int64(t))
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	}

	return fmt.Sprint(v)
}

// formatASPathPlain returns the AS path as space-separated ASNs for text export.
//
// It renders each ASN through scalarString, the same function the browser view
// uses. The CSV download printed AS 4200000000 as "4.2e+09" until 2026-08-14,
// because it read the decoded float64 with the %v verb.
func formatASPathPlain(route map[string]any) string {
	v, ok := route["as-path"].([]any)
	if !ok {
		s, _ := route["as-path"].(string)
		return s
	}
	parts := make([]string, len(v))
	for i, a := range v {
		parts[i] = scalarString(a)
	}
	return textbuf.Join(parts, " ")
}

// csvQuote wraps a value in double quotes if it contains commas or spaces.
func csvQuote(s string) string {
	if strings.ContainsAny(s, ", \"") {
		var tb textbuf.Buffer
		return tb.Byte('"').Str(strings.ReplaceAll(s, "\"", "\"\"")).Byte('"').String()
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

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib prefix ").Str(prefix).String())
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

	v := routeDetailView{Prefix: prefix}
	if route != nil {
		if rows := routeRows([]any{route}); len(rows) == 1 {
			v.Route = &rows[0]
		}
	}

	s.renderFragment(w, routeDetail(v))
}

// handleUIEvents serves the peer table body as a stream of UNNAMED SSE
// messages for live peer state updates.
//
// htmx 4 swaps an unnamed message into the element carrying hx-sse:connect and
// dispatches a NAMED one as a DOM event that swaps nothing, so this stream
// sends one kind of message: the rows the table body must hold. An engine that
// cannot answer is reported inside that same body by peerStreamBody.
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
			// A render error skips the tick. An EMPTY body does not: zero
			// peers is a state the browser must be told about, and the
			// template returned "" for both until 2026-08-14. The tbody
			// carries swapEmpty:true so htmx 4 swaps that empty body.
			html, err := s.peerStreamBody()
			if err != nil {
				s.logger.Warn("SSE: peer table render error", "error", err)

				continue
			}

			if err := writeStreamMessage(w, html); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// peerStreamBody renders the table body that one stream message carries: the
// peer rows, or a single row naming the reason the engine could not answer.
//
// engineError is the producer the peers page reads for its banner, so a
// dispatch failure now reaches a watching operator as well as a reloading one.
// Until this, only a nil response was reported, it was reported as a named
// event nothing consumed, and a dispatch error pushed an empty table instead.
func (s *LGServer) peerStreamBody() (string, error) {
	zeData := parseJSON(s.query("show bgp summary"))

	if message := engineError(zeData); message != "" {
		return renderToString(peersStreamError(message))
	}

	return renderToString(peersTableBody(s.extractPeers(zeData)))
}

// writeStreamMessage writes one UNNAMED server-sent message. Each line of the
// payload needs its own "data: " prefix, or the first newline in the fragment
// ends the message early.
func writeStreamMessage(w io.Writer, html string) error {
	data := strings.ReplaceAll(html, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	data = strings.TrimRight(data, "\n")
	data = strings.ReplaceAll(data, "\n", "\ndata: ")

	var tb textbuf.Buffer

	_, err := io.WriteString(w, tb.Str("data: ").Str(data).Str("\n\n").String())

	return err
}

// extractPeers converts Ze peer summary data into template-friendly format
// and decorates ASN names.
func (s *LGServer) extractPeers(ze map[string]any) []peerRow {
	if ze == nil {
		return nil
	}

	// handleBgpSummary answers the aggregates and the rows as siblings, so the
	// rows are at ze["peers"] (bgp/plugins/cmd/peer/summary.go,
	// handleBgpSummary).
	peers, _ := ze["peers"].([]any)
	var result []peerRow

	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		address := getStr(peer, "address")
		if address == "" {
			address = getStr(peer, "peer-address")
		}

		remoteAS := getStr(peer, "remote-as")

		result = append(result, peerRow{
			Address:      address,
			RemoteAS:     remoteAS,
			RemoteASName: s.resolveASN(remoteAS),
			State:        getStr(peer, "state"),
			Uptime:       getStr(peer, "uptime"),
			// Route counts (NLRI-level) and UPDATE message counts are
			// separate. A count the engine cannot produce stays an empty
			// string. It never becomes a zero.
			RoutesReceived:  getStr(peer, "routes-received"),
			RoutesAccepted:  getStr(peer, "routes-accepted"),
			RoutesSent:      getStr(peer, "routes-sent"),
			UpdatesReceived: getStr(peer, "updates-received"),
			UpdatesSent:     getStr(peer, "updates-sent"),
			Description:     getStr(peer, "description"),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		ipI := net.ParseIP(result[i].Address)
		ipJ := net.ParseIP(result[j].Address)
		if ipI == nil || ipJ == nil {
			return result[i].Address < result[j].Address
		}

		return string(ipI.To16()) < string(ipJ.To16())
	})

	return result
}

// extractBMPPeers converts BMP peer data into template-friendly format.
// Returns nil when BMP is not configured or has no peers.
func (s *LGServer) extractBMPPeers(ze map[string]any) []bmpPeerRow {
	if ze == nil {
		return nil
	}

	peers, _ := ze["peers"].([]any)
	if len(peers) == 0 {
		return nil
	}

	var result []bmpPeerRow
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		peerAS := getStr(peer, "peer-as")

		result = append(result, bmpPeerRow{
			Router:     getStr(peer, "router"),
			PeerAS:     peerAS,
			PeerASName: s.resolveASN(peerAS),
			BGPID:      getStr(peer, "peer-bgp-id"),
			State:      peerState(getBool(peer, "up")),
			IPv6:       getBool(peer, "ipv6"),
		})
	}

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
				unwrapRouteAttrs(rm)
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

	peers, _ := ze["peers"].([]any)
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
