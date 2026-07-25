// Design: docs/architecture/web-interface.md -- Birdwatcher REST API handlers
// Overview: server.go -- LG server and route registration
// Related: handler_ui.go -- HTMX web UI handlers
//
// Implements the birdwatcher API consumed by Alice-LG and other looking glass frontends.
// Reference: https://github.com/alice-lg/birdwatcher
// API spec: https://github.com/alice-lg/birdwatcher/blob/master/endpoints.go
// Alice-LG: https://github.com/alice-lg/alice-lg
//
// Field names use snake_case (birdwatcher convention), NOT ze's kebab-case.
// See .claude/rules/json-format.md for the exception note.

package lg

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// handleAPIStatus returns router status in birdwatcher format (GET /api/looking-glass/status).
func (s *LGServer) handleAPIStatus(w http.ResponseWriter, _ *http.Request) {
	result := s.query("bgp status")

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformStatus(zeData)
	writeJSON(w, bw)
}

// handleAPIProtocols returns the peer list in birdwatcher format (GET /api/looking-glass/protocols/bgp).
func (s *LGServer) handleAPIProtocols(w http.ResponseWriter, _ *http.Request) {
	result := s.query("show bgp summary")

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformProtocols(zeData)
	writeJSON(w, bw)
}

// handleAPIProtocolsShort returns short protocol status in birdwatcher format.
func (s *LGServer) handleAPIProtocolsShort(w http.ResponseWriter, _ *http.Request) {
	result := s.query("show bgp summary")

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformProtocolsShort(zeData)
	writeJSON(w, bw)
}

// serveRoutesForPeer runs "show bgp rib peer <peer>" and writes the birdwatcher
// routes envelope, applying pagination when the client requested it. Shared by
// the protocol/{name} and peer/{peer} endpoints, which differ only in their path
// parameter name and validation message.
func (s *LGServer) serveRoutesForPeer(w http.ResponseWriter, r *http.Request, peer string) {
	limit, offset, present, ok := parsePagination(w, r)
	if !ok {
		return
	}

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib peer ").Str(peer).String())

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	if errMsg, ok := zeData["error"].(string); ok {
		writeJSONError(w, http.StatusNotFound, errMsg)
		return
	}

	bw := transformRoutes(zeData, peer)
	if present {
		paginateRoutes(bw, limit, offset)
	}
	writeJSON(w, bw)
}

// handleAPIRoutesProtocol returns routes from a named peer in birdwatcher format.
func (s *LGServer) handleAPIRoutesProtocol(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "peer name required")
		return
	}

	if !isValidPeerName(name) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer name")
		return
	}

	s.serveRoutesForPeer(w, r, name)
}

// handleAPIRoutesPeer returns routes from a peer by IP address in birdwatcher format.
func (s *LGServer) handleAPIRoutesPeer(w http.ResponseWriter, r *http.Request) {
	peer := r.PathValue("peer")
	if peer == "" {
		writeJSONError(w, http.StatusBadRequest, "peer address required")
		return
	}

	if !isValidPeerName(peer) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer address")
		return
	}

	s.serveRoutesForPeer(w, r, peer)
}

// handleAPIRoutesTable returns best routes by address family.
func (s *LGServer) handleAPIRoutesTable(w http.ResponseWriter, r *http.Request) {
	fam := r.PathValue("family")
	if fam == "" {
		writeJSONError(w, http.StatusBadRequest, "family required")
		return
	}

	if !isValidFamily(fam) {
		writeJSONError(w, http.StatusBadRequest, "invalid address family")
		return
	}

	limit, offset, present, ok := parsePagination(w, r)
	if !ok {
		return
	}

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib best ").Str(fam).String())

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformRoutes(zeData, "")
	if present {
		paginateRoutes(bw, limit, offset)
	}
	writeJSON(w, bw)
}

// handleAPIBMPProtocols returns BMP-monitored peers (GET /api/looking-glass/protocols/bmp).
func (s *LGServer) handleAPIBMPProtocols(w http.ResponseWriter, _ *http.Request) {
	result := s.query("show bmp peers")

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformBMPProtocols(zeData)
	writeJSON(w, bw)
}

// handleAPIBMPRoutes returns routes from a BMP-monitored peer (GET /api/looking-glass/routes/bmp/{name}).
func (s *LGServer) handleAPIBMPRoutes(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "peer name required")
		return
	}

	if !isValidPeerName(name) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer name")
		return
	}

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib-protocol bmp ").Str(name).String())

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	if errMsg, ok := zeData["error"].(string); ok {
		writeJSONError(w, http.StatusNotFound, errMsg)
		return
	}

	bw := transformRoutes(zeData, name)
	writeJSON(w, bw)
}

// transformBMPProtocols converts BMP peer data to birdwatcher protocols format.
func transformBMPProtocols(ze map[string]any) map[string]any {
	peers, _ := ze["peers"].([]any)

	protocols := make(map[string]any)
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		router := getStr(peer, "router")
		peerAS := getNum(peer, "peer-as")
		bgpID := getStr(peer, "peer-bgp-id")
		isUp := getBool(peer, "up")

		state := peerState(isUp)

		var tb textbuf.Buffer
		name := tb.Str(router).Byte(':').Str(bgpID).String()
		protocols[name] = map[string]any{
			"bird_protocol":   name,
			"state":           state,
			"neighbor_as":     peerAS,
			"description":     "BMP monitored",
			"table":           "bmp",
			"routes_received": 0,
			"routes_imported": 0,
			"routes_exported": 0,
			"routes_filtered": 0,
			"routes": map[string]any{
				"imported":  0,
				"filtered":  0,
				"exported":  0,
				"preferred": 0,
			},
		}
	}

	return apiEnvelope("protocols", protocols)
}

// handleAPIRoutesFiltered returns filtered routes per peer.
// Ze does not track import-filtered routes (BIRD's "import keep filtered on").
// Returns an empty route list for API compatibility.
func (s *LGServer) handleAPIRoutesFiltered(w http.ResponseWriter, _ *http.Request) {
	result := apiEnvelope("routes", make([]any, 0))
	result["routes_count"] = 0
	writeJSON(w, result)
}

// handleAPIRoutesExport returns exported routes per peer.
func (s *LGServer) handleAPIRoutesExport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "peer name required")
		return
	}

	if !isValidPeerName(name) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer name")
		return
	}

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib sent peer ").Str(name).String())

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	if errMsg, ok := zeData["error"].(string); ok {
		writeJSONError(w, http.StatusNotFound, errMsg)
		return
	}

	bw := transformRoutes(zeData, name)
	writeJSON(w, bw)
}

// handleAPIRoutesNoExport returns not-exported routes per peer.
// Ze does not track export-filtered routes separately.
// Returns an empty route list for API compatibility.
func (s *LGServer) handleAPIRoutesNoExport(w http.ResponseWriter, _ *http.Request) {
	result := apiEnvelope("routes", make([]any, 0))
	result["routes_count"] = 0
	writeJSON(w, result)
}

// handleAPIRoutesCount returns the route count for a protocol.
func (s *LGServer) handleAPIRoutesCount(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "peer name required")
		return
	}

	if !isValidPeerName(name) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer name")
		return
	}

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib count peer ").Str(name).String())

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	count := getNum(zeData, "count")
	writeJSON(w, apiEnvelope("routes", int(count)))
}

// handleAPIRoutesPrefix searches routes by prefix (birdwatcher: /routes/prefix?prefix=...).
func (s *LGServer) handleAPIRoutesPrefix(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		writeJSONError(w, http.StatusBadRequest, "prefix query parameter required")
		return
	}

	if !isValidPrefix(prefix) {
		writeJSONError(w, http.StatusBadRequest, "invalid prefix")
		return
	}

	limit, offset, present, ok := parsePagination(w, r)
	if !ok {
		return
	}

	var tb textbuf.Buffer
	result := s.query(tb.Str("show bgp rib prefix ").Str(prefix).String())

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformRoutes(zeData, "")
	if present {
		paginateRoutes(bw, limit, offset)
	}
	writeJSON(w, bw)
}

// handleAPIRoutesSearch is an alias for handleAPIRoutesPrefix (ze-specific path).
func (s *LGServer) handleAPIRoutesSearch(w http.ResponseWriter, r *http.Request) {
	s.handleAPIRoutesPrefix(w, r)
}

// writeJSON writes a JSON response with Content-Type header.
func writeJSON(w http.ResponseWriter, data map[string]any) {
	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		lgLogger.Warn("json encode error", "error", err)
	}
}

// parseJSON parses a JSON string into a map. Returns nil on failure.
// When the engine returns a JSON array (e.g., peer summary), it is wrapped
// as {"peers": arr}. Non-empty invalid JSON is logged as a warning.
func parseJSON(s string) map[string]any {
	if s == "" {
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		// Try parsing as array (peer summary returns array).
		var arr []any
		if arrErr := json.Unmarshal([]byte(s), &arr); arrErr == nil {
			return map[string]any{"peers": arr}
		}

		lgLogger.Warn("failed to parse engine response as JSON", "error", err, "length", len(s))
		return nil
	}

	return result
}

// isValidPeerName checks that a peer name contains only safe characters.
func isValidPeerName(name string) bool {
	if name == "" || len(name) > 255 {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' && c != ':' {
			return false
		}
	}
	return true
}

// isValidFamily checks that a family string is in "afi/safi" format.
func isValidFamily(family string) bool {
	parts := strings.SplitN(family, "/", 2)
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[1] != "" && isValidPeerName(parts[0]) && isValidPeerName(parts[1])
}

// isValidPrefix checks that a prefix looks like an IP or CIDR notation.
func isValidPrefix(prefix string) bool {
	if prefix == "" || len(prefix) > 50 {
		return false
	}
	for _, c := range prefix {
		if (c < '0' || c > '9') && c != '.' && c != ':' && c != '/' && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// apiEnvelope wraps a payload with the standard birdwatcher api metadata.
func apiEnvelope(key string, value any) map[string]any {
	return map[string]any{
		"api": map[string]any{
			"Version":           "Ze Looking Glass",
			"result_from_cache": false,
		},
		key: value,
	}
}

// maxPageLimit caps the per-request pagination window. A request asking for
// more than this is rejected with 400 rather than silently clamped, so a client
// never believes it received a full page when it did not (spec AC-11 boundary).
const maxPageLimit = 100000

// parsePagination reads the optional limit/offset query params shared by the
// route-list endpoints. present is true when the client supplied either param;
// when it is false the caller MUST leave the response byte-identical to the
// unpaginated default response (R-5). ok is false when a param is malformed,
// in which case a 400 has already been written to w.
func parsePagination(w http.ResponseWriter, r *http.Request) (limit, offset int, present, ok bool) {
	q := r.URL.Query()
	limStr := q.Get("limit")
	offStr := q.Get("offset")
	if limStr == "" && offStr == "" {
		return 0, 0, false, true
	}
	if limStr != "" {
		n, err := strconv.Atoi(limStr)
		if err != nil || n < 0 || n > maxPageLimit {
			writeJSONError(w, http.StatusBadRequest, "invalid limit (expected 0..100000)")
			return 0, 0, false, false
		}
		limit = n
	}
	if offStr != "" {
		n, err := strconv.Atoi(offStr)
		if err != nil || n < 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid offset (expected >= 0)")
			return 0, 0, false, false
		}
		offset = n
	}
	return limit, offset, true, true
}

// paginateRoutes windows the "routes" array of a birdwatcher envelope in place.
// offset skips leading routes (clamped to the list length so an over-large
// offset yields an empty page, not a panic); limit caps the page size (limit <=
// 0 means "no cap", returning all routes from offset). routes_count is reset to
// the returned page length and a "pagination" object carries the pre-slice total
// so a client can compute how many pages exist. Applied only when the client
// requested pagination, keeping the default response unchanged.
func paginateRoutes(bw map[string]any, limit, offset int) {
	routes, _ := bw["routes"].([]any)
	total := len(routes)
	start := min(offset, total)
	end := total
	if limit > 0 {
		end = min(end, start+limit)
	}
	page := routes[start:end]
	bw["routes"] = page
	bw["routes_count"] = len(page)
	bw["pagination"] = map[string]any{
		"total_results": total,
		"offset":        offset,
		"limit":         limit,
	}
}

// transformStatus converts Ze bgp status JSON to birdwatcher status format.
func transformStatus(ze map[string]any) map[string]any {
	result := apiEnvelope("status", map[string]any{
		"router_id":      getStr(ze, "router-id"),
		"current_server": time.Now().UTC().Format(time.RFC3339),
		"server_time":    time.Now().UTC().Format(time.RFC3339),
		"last_reboot":    getStr(ze, "start-time"),
		"last_reconfig":  getStr(ze, "last-config-change"),
		"message":        "Ze BGP daemon",
		"version":        getStr(ze, "version"),
	})
	return result
}

// transformProtocols converts Ze peer summary to birdwatcher protocols format.
func transformProtocols(ze map[string]any) map[string]any {
	peers := summaryPeers(ze)

	protocols := make(map[string]any)
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		addr := peerAddress(peer)
		name := getStr(peer, "name")
		if name == "" {
			name = addr
		}

		received := getNum(peer, "routes-received")
		accepted := getNum(peer, "routes-accepted")
		sent := getNum(peer, "routes-sent")
		filtered := getNum(peer, "routes-filtered")

		protocols[name] = map[string]any{
			"bird_protocol":    name,
			"state":            getStr(peer, "state"),
			"state_changed":    getStr(peer, "state-changed"),
			"neighbor_address": addr,
			"neighbor_as":      getNum(peer, "remote-as"),
			"description":      getStr(peer, "description"),
			"last_error":       getStr(peer, "last-error"),
			"table":            "master",
			// Flat fields for simple consumers.
			"routes_received": received,
			"routes_imported": accepted,
			"routes_exported": sent,
			"routes_filtered": filtered,
			"uptime":          uptimeSeconds(peer),
			// Nested routes object for Alice-LG.
			"routes": map[string]any{
				"imported":  accepted,
				"filtered":  filtered,
				"exported":  sent,
				"preferred": accepted,
			},
		}
	}

	return apiEnvelope("protocols", protocols)
}

// transformProtocolsShort converts Ze peer summary to birdwatcher short protocols format.
func transformProtocolsShort(ze map[string]any) map[string]any {
	peers := summaryPeers(ze)

	protocols := make(map[string]any)
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		name := getStr(peer, "name")
		if name == "" {
			name = peerAddress(peer)
		}

		protocols[name] = map[string]any{
			"proto": "BGP",
			"table": "master",
			"state": getStr(peer, "state"),
			"since": getStr(peer, "state-changed"),
			"info":  getStr(peer, "state"),
		}
	}

	return apiEnvelope("protocols", protocols)
}

// transformRoutes converts Ze route data to birdwatcher routes format.
func transformRoutes(ze map[string]any, peerName string) map[string]any {
	routes := extractRoutes(ze)

	bwRoutes := make([]any, 0, len(routes))
	for _, r := range routes {
		route, ok := r.(map[string]any)
		if !ok {
			continue
		}

		bwRoute := map[string]any{
			"network":       getStr(route, "prefix"),
			"gateway":       getStr(route, "next-hop"),
			"metric":        getNum(route, "med"),
			"interface":     "",
			"from_protocol": peerName,
			"age":           getNum(route, "age"),
			"learnt_from":   getStr(route, "peer-address"),
			"primary":       getBool(route, "best"),
			"bgp": map[string]any{
				"origin":            getStr(route, "origin"),
				"as_path":           getVal(route, "as-path"),
				"next_hop":          getStr(route, "next-hop"),
				"local_pref":        getNum(route, "local-preference"),
				"med":               getNum(route, "med"),
				"communities":       transformCommunities(getVal(route, "community")),
				"large_communities": transformLargeCommunities(getVal(route, "large-community")),
				"ext_communities":   getVal(route, "extended-community"),
			},
		}

		if from := getStr(route, "peer-address"); from != "" {
			bwRoute["from_protocol"] = from
		}

		bwRoutes = append(bwRoutes, bwRoute)
	}

	result := apiEnvelope("routes", bwRoutes)
	result["routes_count"] = len(bwRoutes)
	return result
}

// transformCommunities converts Ze community strings ("65000:100" or well-known
// names like "NO_EXPORT") to birdwatcher integer-pair format ([[65000, 100], ...]).
func transformCommunities(v any) any {
	arr, ok := v.([]any)
	if !ok || arr == nil {
		return nil
	}

	var result []any
	for _, c := range arr {
		s, ok := c.(string)
		if !ok {
			result = append(result, c)
			continue
		}

		if pair, ok := wellKnownCommunityPair(s); ok {
			result = append(result, pair)
			continue
		}

		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			result = append(result, c)
			continue
		}

		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			result = append(result, c)
			continue
		}

		result = append(result, []any{major, minor})
	}

	return result
}

var wellKnownCommunities = map[string][2]int{
	"no-export":                  {65535, 65281},
	"no-advertise":               {65535, 65282},
	"no-export-subconfed":        {65535, 65283},
	"nopeer":                     {65535, 65284},
	"graceful-shutdown":          {65535, 0},
	"accept-own":                 {65535, 1},
	"route-filter-translated-v4": {65535, 2},
	"route-filter-v4":            {65535, 3},
	"route-filter-translated-v6": {65535, 4},
	"route-filter-v6":            {65535, 5},
	"llgr-stale":                 {65535, 6},
	"no-llgr":                    {65535, 7},
	"accept-own-nexthop":         {65535, 8},
	"standby-pe":                 {65535, 9},
	"blackhole":                  {65535, 666},
}

func wellKnownCommunityPair(name string) ([]any, bool) {
	pair, ok := wellKnownCommunities[name]
	if !ok {
		return nil, false
	}
	return []any{pair[0], pair[1]}, true
}

// transformLargeCommunities converts Ze large community strings ("65000:0:100") to
// birdwatcher integer-triple format ([[65000, 0, 100], ...]).
func transformLargeCommunities(v any) any {
	arr, ok := v.([]any)
	if !ok || arr == nil {
		return nil
	}

	var result []any
	for _, c := range arr {
		s, ok := c.(string)
		if !ok {
			result = append(result, c)
			continue
		}

		parts := strings.SplitN(s, ":", 3)
		if len(parts) != 3 {
			result = append(result, c)
			continue
		}

		admin, err1 := strconv.Atoi(parts[0])
		val1, err2 := strconv.Atoi(parts[1])
		val2, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			result = append(result, c)
			continue
		}

		result = append(result, []any{admin, val1, val2})
	}

	return result
}

func peerState(up bool) string {
	if up {
		return "up"
	}
	return "down"
}

// getStr extracts a string value from a map, returning empty string if missing or nil.
func getStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}

	if wrapped, ok := v.(map[string]any); ok {
		if val, ok := wrapped["value"]; ok {
			v = val
		}
	}

	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

// summaryPeers extracts the peer rows from a "show bgp summary" response.
//
// handleBgpSummary wraps its payload in a "summary" envelope
// (internal/component/bgp/plugins/cmd/peer/summary.go:152), so the rows live at
// ze["summary"]["peers"]. The flat form is still accepted: parseJSON promotes a
// bare JSON array to ze["peers"], and tests build maps that shape.
// Mirrors the navigation in handler_ui.go.
func summaryPeers(ze map[string]any) []any {
	if summary, ok := ze["summary"].(map[string]any); ok {
		if peers, ok := summary["peers"].([]any); ok {
			return peers
		}
	}
	peers, _ := ze["peers"].([]any)
	return peers
}

// peerAddress returns a peer's IP. handleBgpSummary emits it as "address"
// (summary.go:113); "peer-address" is accepted as a fallback for other
// producers. Mirrors the fallback in handler_ui.go.
func peerAddress(peer map[string]any) string {
	if addr := getStr(peer, "address"); addr != "" {
		return addr
	}
	return getStr(peer, "peer-address")
}

// uptimeSeconds returns a peer's uptime in seconds for the birdwatcher
// "uptime" field, which Alice-LG reads as a number.
//
// The engine emits uptime as a Go duration string ("6m10s"), so getNum -- whose
// type switch has no string case -- returned 0 for every real response. Parse
// the duration here; numeric producers still pass straight through.
func uptimeSeconds(peer map[string]any) float64 {
	if s, ok := peer["uptime"].(string); ok {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0
		}
		return d.Seconds()
	}
	return getNum(peer, "uptime")
}

// getNum extracts a numeric value from a map, returning 0 if missing.
func getNum(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}

	if wrapped, ok := v.(map[string]any); ok {
		if val, ok := wrapped["value"]; ok {
			v = val
		}
	}

	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint32:
		return float64(n)
	}

	return 0
}

// getBool extracts a boolean value from a map, returning false if missing.
func getBool(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}

	if wrapped, ok := v.(map[string]any); ok {
		if val, ok := wrapped["value"]; ok {
			v = val
		}
	}

	b, ok := v.(bool)
	if ok {
		return b
	}

	return false
}

// unwrapRouteAttrs replaces flag-annotated attribute values in a route map
// with their inner "value", so consumers see plain types.
func unwrapRouteAttrs(rm map[string]any) {
	for k, v := range rm {
		if m, ok := v.(map[string]any); ok {
			if val, ok := m["value"]; ok {
				rm[k] = val
			}
		}
	}
}

// getVal extracts a raw value from a map, unwrapping flag-annotated attributes.
func getVal(m map[string]any, key string) any {
	v, ok := m[key]
	if !ok {
		return nil
	}

	if wrapped, ok := v.(map[string]any); ok {
		if val, ok := wrapped["value"]; ok {
			return val
		}
	}

	return v
}
