// Design: docs/architecture/web-interface.md -- Birdwatcher REST API handlers
// Overview: server.go -- LG server and route registration
// Related: handler_ui.go -- HTMX web UI handlers

package lg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	result := s.query("summary")

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
	result := s.query("summary")

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformProtocolsShort(zeData)
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

	result := s.query(fmt.Sprintf("peer %s rib show", name))

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

	result := s.query(fmt.Sprintf("peer %s rib show", peer))

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
	writeJSON(w, bw)
}

// handleAPIRoutesTable returns best routes by address family.
func (s *LGServer) handleAPIRoutesTable(w http.ResponseWriter, r *http.Request) {
	family := r.PathValue("family")
	if family == "" {
		writeJSONError(w, http.StatusBadRequest, "family required")
		return
	}

	if !isValidFamily(family) {
		writeJSONError(w, http.StatusBadRequest, "invalid address family")
		return
	}

	result := s.query(fmt.Sprintf("rib best %s", family))

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformRoutes(zeData, "")
	writeJSON(w, bw)
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

	result := s.query(fmt.Sprintf("peer %s rib show sent", name))

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

	result := s.query(fmt.Sprintf("peer %s rib show count", name))

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

	result := s.query(fmt.Sprintf("rib show prefix %s", prefix))

	zeData := parseJSON(result)
	if zeData == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "engine unavailable")
		return
	}

	bw := transformRoutes(zeData, "")
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
	peers, _ := ze["peers"].([]any)

	protocols := make(map[string]any)
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		name := getStr(peer, "name")
		if name == "" {
			name = getStr(peer, "peer-address")
		}

		received := getNum(peer, "routes-received")
		accepted := getNum(peer, "routes-accepted")
		sent := getNum(peer, "routes-sent")
		filtered := getNum(peer, "routes-filtered")

		protocols[name] = map[string]any{
			"bird_protocol":    name,
			"state":            getStr(peer, "state"),
			"state_changed":    getStr(peer, "state-changed"),
			"neighbor_address": getStr(peer, "peer-address"),
			"neighbor_as":      getNum(peer, "remote-as"),
			"description":      getStr(peer, "description"),
			"last_error":       getStr(peer, "last-error"),
			"table":            "master",
			// Flat fields for simple consumers.
			"routes_received": received,
			"routes_imported": accepted,
			"routes_exported": sent,
			"routes_filtered": filtered,
			"uptime":          getNum(peer, "uptime"),
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
	peers, _ := ze["peers"].([]any)

	protocols := make(map[string]any)
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		name := getStr(peer, "name")
		if name == "" {
			name = getStr(peer, "peer-address")
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
	routes, _ := ze["routes"].([]any)
	if routes == nil {
		routes, _ = ze["prefixes"].([]any)
	}

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
				"as_path":           route["as-path"],
				"next_hop":          getStr(route, "next-hop"),
				"local_pref":        getNum(route, "local-preference"),
				"med":               getNum(route, "med"),
				"communities":       transformCommunities(route["community"]),
				"large_communities": transformLargeCommunities(route["large-community"]),
				"ext_communities":   route["extended-community"],
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

// transformCommunities converts Ze community strings ("65000:100") to birdwatcher
// integer-pair format ([[65000, 100], ...]).
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

// getStr extracts a string value from a map, returning empty string if missing or nil.
func getStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}

	s, ok := v.(string)
	if ok {
		return s
	}

	return fmt.Sprintf("%v", v)
}

// getNum extracts a numeric value from a map, returning 0 if missing.
func getNum(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}

	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
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

	b, ok := v.(bool)
	if ok {
		return b
	}

	return false
}
