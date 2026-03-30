// Design: docs/architecture/web-interface.md -- Birdwatcher REST API handlers
// Overview: server.go -- LG server and route registration
// Related: handler_ui.go -- HTMX web UI handlers

package lg

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	result := s.query(fmt.Sprintf("rib show peer %s", name))

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
func (s *LGServer) handleAPIRoutesFiltered(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "peer name required")
		return
	}

	if !isValidPeerName(name) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer name")
		return
	}

	result := s.query(fmt.Sprintf("rib show peer %s filtered", name))

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

// handleAPIRoutesSearch searches for routes matching a prefix.
func (s *LGServer) handleAPIRoutesSearch(w http.ResponseWriter, r *http.Request) {
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

// transformStatus converts Ze bgp status JSON to birdwatcher status format.
func transformStatus(ze map[string]any) map[string]any {
	return map[string]any{
		"api": map[string]any{
			"Version":         "Ze Looking Glass",
			"ResultFromCache": false,
		},
		"status": map[string]any{
			"router_id":     getStr(ze, "router-id"),
			"server_time":   time.Now().UTC().Format(time.RFC3339),
			"last_reboot":   getStr(ze, "start-time"),
			"last_reconfig": getStr(ze, "last-config-change"),
			"message":       "Ze BGP daemon",
			"version":       getStr(ze, "version"),
		},
	}
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

		protocols[name] = map[string]any{
			"bird_protocol":    name,
			"state":            getStr(peer, "state"),
			"neighbor_address": getStr(peer, "peer-address"),
			"neighbor_as":      getNum(peer, "remote-as"),
			"description":      getStr(peer, "description"),
			"routes_received":  getNum(peer, "routes-received"),
			"routes_imported":  getNum(peer, "routes-accepted"),
			"routes_exported":  getNum(peer, "routes-sent"),
			"routes_filtered":  getNum(peer, "routes-filtered"),
			"uptime":           getNum(peer, "uptime"),
		}
	}

	return map[string]any{
		"api": map[string]any{
			"Version":         "Ze Looking Glass",
			"ResultFromCache": false,
		},
		"protocols": protocols,
	}
}

// transformRoutes converts Ze route data to birdwatcher routes format.
func transformRoutes(ze map[string]any, peerName string) map[string]any {
	routes, _ := ze["routes"].([]any)
	if routes == nil {
		routes, _ = ze["prefixes"].([]any)
	}

	var bwRoutes []any
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
			"bgp": map[string]any{
				"origin":          getStr(route, "origin"),
				"as_path":         route["as-path"],
				"next_hop":        getStr(route, "next-hop"),
				"local_pref":      getNum(route, "local-preference"),
				"med":             getNum(route, "med"),
				"community":       route["community"],
				"large_community": route["large-community"],
				"ext_community":   route["extended-community"],
			},
		}

		if from := getStr(route, "peer-address"); from != "" {
			bwRoute["from_protocol"] = from
		}

		bwRoutes = append(bwRoutes, bwRoute)
	}

	return map[string]any{
		"api": map[string]any{
			"Version":         "Ze Looking Glass",
			"ResultFromCache": false,
		},
		"routes":       bwRoutes,
		"routes_count": len(bwRoutes),
	}
}

// getStr extracts a string value from a map, returning empty string if missing.
func getStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
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
