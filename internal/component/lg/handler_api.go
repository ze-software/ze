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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/report"

	"github.com/ze-software/ze/internal/core/bgp/asn"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	fieldPeers = "peers"
	fieldState = "state"
	fieldTable = "table"
)

// The engine commands the status endpoint reads. Each one is a path the YANG
// command tree declares, and TestStatusCommandsAreDeclared walks that tree and
// fails when one of them stops being a command. The endpoint asked for
// `bgp status` until 2026-09-20, which the daemon has never served: the
// dispatch failed on every request and the endpoint answered 200 with an empty
// router id and an empty version (plan/journal/zero-value-as-valid-answer.md,
// 2026-09-17).
const (
	// cmdBGPOverview answers the router id, the local AS and the peer rows.
	cmdBGPOverview = "show bgp"
	// cmdShowVersion answers the running version and build date.
	cmdShowVersion = "show version"
	// cmdShowUptime answers the daemon start time.
	cmdShowUptime = "show uptime"
	// cmdShowReloadStatus answers when the last config reload finished.
	cmdShowReloadStatus = "show reload-status"
	// cmdShowBMPPeers answers the peers the BMP plugin monitors.
	cmdShowBMPPeers = "show bmp peers"
)

// handleAPIStatus returns router status in birdwatcher format (GET /api/looking-glass/status).
//
// Four commands answer it, because no single command holds the whole identity:
// `show bgp` carries the router id, `show version` the release, `show uptime`
// the start time, and `show reload-status` the last configuration change.
//
// Every one of them must answer. The router id and the version ARE the
// identity this endpoint publishes, so an answer that carries neither is a
// refusal rather than a status object (ai/rules/principles.md).
func (s *LGServer) handleAPIStatus(w http.ResponseWriter, _ *http.Request) {
	overview, ok := s.engineAnswer(w, cmdBGPOverview)
	if !ok {
		return
	}
	release, ok := s.engineAnswer(w, cmdShowVersion)
	if !ok {
		return
	}
	started, ok := s.engineAnswer(w, cmdShowUptime)
	if !ok {
		return
	}
	reloaded, ok := s.engineAnswer(w, cmdShowReloadStatus)
	if !ok {
		return
	}

	routerID := getStr(overview, "router-id")
	if routerID == "" {
		writeJSONError(w, http.StatusBadGateway, cmdBGPOverview+": answer carries no router-id")
		return
	}

	running := getStr(release, "version")
	if running == "" {
		writeJSONError(w, http.StatusBadGateway, cmdShowVersion+": answer carries no version")
		return
	}

	// last_reconfig stays empty until the daemon processes its first reload,
	// which is what `show reload-status` reports for a router nobody has
	// reloaded. That empty is the engine's answer, not a missing one.
	writeJSON(w, transformStatus(routerID, running,
		getStr(started, "start-time"), getStr(reloaded, "last-reload-at")))
}

// engineAnswer dispatches one command and returns the answer it parsed.
//
// A failed dispatch is not an answer, and neither is an error envelope or a
// reply that is not JSON at all: a handler that read only the keys it wants
// would render whatever the zero value of each key looks like, with HTTP 200
// over it. This writes the refusal itself and reports false, naming the command
// that failed so an operator reading the body knows which one
// (ai/rules/principles.md).
func (s *LGServer) engineAnswer(w http.ResponseWriter, command string) (map[string]any, bool) {
	ze, failure := s.ask(command)
	if failure != nil {
		writeEngineFailure(w, failure)
		return nil, false
	}

	return ze, true
}

// writeEngineFailure writes the refusal one failed command owes: its status,
// and a body naming the command and the failure class, never the
// dispatcher's own error (engineFailure.message).
//
// The two codes name two different failures. 503 says the engine produced
// nothing this server could parse. 502 says the engine answered, and its answer
// was an error.
func writeEngineFailure(w http.ResponseWriter, failure *engineFailure) {
	writeJSONError(w, failure.status, failure.message())
}

// peerRoutesAnswer dispatches a peer-scoped routes command and returns the
// answer, or writes the refusal and reports false.
//
// An error envelope answers 404 carrying the engine's refusal: bgp-rib refuses
// with one, and these endpoints have always read that refusal as the named
// peer having nothing to list. Any other failure is an engine failure and
// answers as engineAnswer does.
func (s *LGServer) peerRoutesAnswer(w http.ResponseWriter, command string) (map[string]any, bool) {
	ze, failure := s.ask(command)
	if failure == nil {
		return ze, true
	}

	if failure.envelope {
		writeJSONError(w, http.StatusNotFound, failure.refusal)
		return nil, false
	}

	writeEngineFailure(w, failure)
	return nil, false
}

// handleAPIProtocols returns the peer list in birdwatcher format (GET /api/looking-glass/protocols/bgp).
func (s *LGServer) handleAPIProtocols(w http.ResponseWriter, _ *http.Request) {
	zeData, ok := s.engineAnswer(w, cmdBGPOverview)
	if !ok {
		return
	}

	writeJSON(w, transformProtocols(zeData))
}

// handleAPIProtocolsShort returns short protocol status in birdwatcher format.
func (s *LGServer) handleAPIProtocolsShort(w http.ResponseWriter, _ *http.Request) {
	zeData, ok := s.engineAnswer(w, cmdBGPOverview)
	if !ok {
		return
	}

	writeJSON(w, transformProtocolsShort(zeData))
}

// serveRoutesForPeer runs "show bgp rib received peer <peer>" and writes the
// birdwatcher routes envelope, applying pagination when the client requested
// it. Shared by the protocol/{name} and peer/{peer} endpoints: peer/{peer}
// passes its address through, protocol/{name} passes the address
// protocolSession resolved its name to.
//
// Both endpoints answer the routes learned from the session, so the query names
// bgp-rib's `received` scope. With no scope keyword the plugin answers
// `sent-received` (parsePipelineArgs), and the routes Ze sent the peer were
// listed as learned from it. routes/export/{name} answers the `sent` scope.
func (s *LGServer) serveRoutesForPeer(w http.ResponseWriter, r *http.Request, peer string) {
	limit, offset, present, ok := parsePagination(w, r)
	if !ok {
		return
	}

	var tb textbuf.Buffer
	zeData, ok := s.peerRoutesAnswer(w, tb.Str("show bgp rib received peer ").Str(peer).String())
	if !ok {
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

	address, ok := s.protocolSession(w, name)
	if !ok {
		return
	}

	s.serveRoutesForPeer(w, r, address)
}

// protocolSession resolves a birdwatcher protocol id to the address of the
// session protocols/bgp lists under it, or writes the refusal and reports
// false.
//
// bgp-rib keys every session by its address, while protocols/bgp names each
// one by its configured name (protocolID). Sending the name on to bgp-rib
// matched no session, so a listed protocol answered 200 with no routes, and so
// did a name no session carries.
//
// Two ids name a session: the id protocols/bgp lists it under (protocolID),
// and its address. The address is unique to one session and the endpoint
// answered it before protocols/bgp listed configured names, so a client that
// built its URL from the address keeps working. When a configured name equals
// another session's address, the name wins: it is the id protocols/bgp lists.
// An id matching neither is 404 naming it.
func (s *LGServer) protocolSession(w http.ResponseWriter, id string) (string, bool) {
	overview, ok := s.engineAnswer(w, cmdBGPOverview)
	if !ok {
		return "", false
	}

	var byAddress string
	for _, p := range summaryPeers(overview) {
		peer, isPeer := p.(map[string]any)
		if !isPeer {
			continue
		}
		address := peerAddress(peer)
		if protocolID(peer) != id {
			if address == id {
				byAddress = address
			}
			continue
		}
		if address == "" {
			writeJSONError(w, http.StatusBadGateway, cmdBGPOverview+": session "+strconv.Quote(id)+" carries no address")
			return "", false
		}
		return address, true
	}
	if byAddress != "" {
		return byAddress, true
	}

	writeJSONError(w, http.StatusNotFound, "no BGP session is named "+strconv.Quote(id)+
		": GET /api/looking-glass/protocols/bgp lists the protocol names and addresses")
	return "", false
}

// protocolID is the name protocols/bgp lists a session under: its configured
// name, else its address. protocolSession resolves the same id back.
func protocolID(peer map[string]any) string {
	if name := getStr(peer, "name"); name != "" {
		return name
	}
	return peerAddress(peer)
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

	// bgp-rib's best-path walk takes the family as a filter: a bare family
	// after `best` is an unknown keyword (parseBestPipelineArgs).
	var tb textbuf.Buffer
	zeData, ok := s.engineAnswer(w, tb.Str("show bgp rib best family ").Str(fam).String())
	if !ok {
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
	zeData, ok := s.engineAnswer(w, cmdShowBMPPeers)
	if !ok {
		return
	}

	bw, left := transformBMPProtocols(zeData)
	if left.total() > 0 {
		// The BMP plugin reports one distinct, routable rib-peer for every
		// peer it records, so any row left out is a producer defect or a
		// router address the looking glass cannot carry in a command.
		s.logger.Warn("looking glass left BMP peers out of protocols/bmp: "+
			"a client cannot list them or query their routes through routes/bmp; "+
			"compare the rib-peer values `show bmp peers` reports and report the rows",
			"no-rib-peer", left.keyless, "duplicate-rib-peer", left.duplicate,
			"unsafe-rib-peer", left.unroutable)
	}
	writeJSON(w, bw)
}

// bmpProtocolsLeftOut counts the `show bmp peers` rows transformBMPProtocols
// could not name, by reason. Every row counted is missing from protocols/bmp.
type bmpProtocolsLeftOut struct {
	keyless    int // no rib-peer: nothing routes/bmp/{name} could answer for
	duplicate  int // a rib-peer an earlier row already named
	unroutable int // a rib-peer isValidBMPPeerKey refuses, so routes/bmp answers 400
}

func (l bmpProtocolsLeftOut) total() int {
	return l.keyless + l.duplicate + l.unroutable
}

// handleAPIBMPRoutes returns routes from a BMP-monitored peer (GET /api/looking-glass/routes/bmp/{name}).
//
// The name is the RIB key protocols/bmp lists, the rib-peer of `show bmp peers`,
// and it is passed to the RIB unchanged: the looking glass never parses or
// rebuilds it, so the BMP plugin stays its only author.
func (s *LGServer) handleAPIBMPRoutes(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "peer name required")
		return
	}

	if !isValidBMPPeerKey(name) {
		writeJSONError(w, http.StatusBadRequest, "invalid peer name")
		return
	}

	var tb textbuf.Buffer
	zeData, ok := s.peerRoutesAnswer(w, tb.Str("show bgp rib protocol bmp peer ").Str(name).String())
	if !ok {
		return
	}

	bw := transformRoutes(zeData, name)
	writeJSON(w, bw)
}

// transformBMPProtocols converts BMP peer data to birdwatcher protocols format.
//
// Each protocol is named by the peer's rib-peer, the key the RIB stores its
// routes under, so the name a client reads here is the one routes/bmp/{name}
// answers for. A row is left out and counted in the second result when it has
// no rib-peer, when an earlier row already took its rib-peer (overwriting it
// would hide one peer in silence), or when isValidBMPPeerKey refuses it, so
// every name listed passes the one validator routes/bmp/{name} applies.
func transformBMPProtocols(ze map[string]any) (map[string]any, bmpProtocolsLeftOut) {
	peers, _ := ze[fieldPeers].([]any)

	protocols := make(map[string]any)
	var left bmpProtocolsLeftOut
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}

		name := getStr(peer, "rib-peer")
		if name == "" {
			left.keyless++
			continue
		}
		if _, taken := protocols[name]; taken {
			left.duplicate++
			continue
		}
		if !isValidBMPPeerKey(name) {
			left.unroutable++
			continue
		}
		state := peerState(getBool(peer, "up"))

		protocol := map[string]any{
			"bird_protocol":   name,
			fieldState:        state,
			"description":     "BMP monitored",
			fieldTable:        "bmp",
			"routes_received": 0,
			"routes_imported": 0,
			"routes_exported": 0,
			"routes_filtered": 0,
			// Always false: these four are placeholders and no source is
			// consulted for a BMP-monitored peer at all.
			"routes_counts_available": false,
			"routes": map[string]any{
				"imported":  0,
				"filtered":  0,
				"exported":  0,
				"preferred": 0,
			},
		}
		setNeighborAS(protocol, peer, "peer-as", name)
		protocols[name] = protocol
	}

	return apiEnvelope("protocols", protocols), left
}

// handleAPIRoutesFiltered returns filtered routes per peer.
// Ze does not track import-filtered routes (BIRD's "import keep filtered on").
// Returns an empty route list for API compatibility.
func (s *LGServer) handleAPIRoutesFiltered(w http.ResponseWriter, _ *http.Request) {
	result := apiEnvelope("routes", make([]any, 0))
	result["routes_count"] = 0
	writeJSON(w, result)
}

// handleAPIRoutesExport returns the routes Ze sent a peer: bgp-rib's `sent`
// scope, its Adj-RIB-Out for that peer.
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

	address, ok := s.protocolSession(w, name)
	if !ok {
		return
	}

	var tb textbuf.Buffer
	zeData, ok := s.peerRoutesAnswer(w, tb.Str("show bgp rib sent peer ").Str(address).String())
	if !ok {
		return
	}

	bw := transformRoutes(zeData, address)
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

// handleAPIRoutesCount returns the count of routes learned from a protocol,
// the same `received` scope routes/protocol/{name} lists (serveRoutesForPeer).
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

	address, ok := s.protocolSession(w, name)
	if !ok {
		return
	}

	// `count` is a terminal, and bgp-rib refuses anything after a terminal
	// (parsePipelineArgs), so the scope and the peer selector come first.
	var tb textbuf.Buffer
	command := tb.Str("show bgp rib received peer ").Str(address).Str(" count").String()
	zeData, ok := s.engineAnswer(w, command)
	if !ok {
		return
	}

	// An answer with no count is not a count of zero.
	count, ok := numValue(zeData, "count")
	if !ok {
		writeJSONError(w, http.StatusBadGateway, command+": answer carries no count")
		return
	}
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

	// The routes learned from peers: bgp-rib's `received` scope. With no scope
	// keyword the plugin answers `sent-received` (parsePipelineArgs), and a
	// route Ze sent a peer reads as learned from that peer.
	var tb textbuf.Buffer
	zeData, ok := s.engineAnswer(w, tb.Str("show bgp rib received prefix ").Str(prefix).String())
	if !ok {
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
			return map[string]any{fieldPeers: arr}
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
		if !isPeerNameRune(c) {
			return false
		}
	}
	return true
}

// isPeerNameRune reports whether c may appear in a peer name: a letter, a
// digit, '-', '_', '.' or ':'.
func isPeerNameRune(c rune) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return c == '-' || c == '_' || c == '.' || c == ':'
}

// isValidBMPPeerKey checks that a BMP peer key holds only the characters the
// BMP plugin's keys are made of (bmpCompositeKey, bgp/plugins/bmp/bmp.go): the
// peer-name set, plus the brackets net.Conn.RemoteAddr puts round an IPv6
// router, and the zone a link-local IPv6 router carries inside them
// ("[fe80::1%eth0]:4000"). The key is embedded in a dispatched command, so the
// set excludes every character the command tokenizer or pipe parser reads:
// space, tab, double quote, backslash and '|'.
//
// A zone is admitted only as '%' followed by one or more letters, digits, '-',
// '_' or '.', closed by ']'. transformBMPProtocols applies this same check, so
// protocols/bmp never lists a name routes/bmp/{name} refuses.
func isValidBMPPeerKey(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > 255 {
		return false
	}
	zone := -1 // runes read in the zone, or -1 outside one
	for _, c := range name {
		if zone >= 0 {
			if c != ']' {
				if !isZoneRune(c) {
					return false
				}
				zone++
				continue
			}
			if zone == 0 {
				return false
			}
			zone = -1
			continue
		}
		if c == '%' {
			zone = 0
			continue
		}
		if c == '[' || c == ']' {
			continue
		}
		if !isPeerNameRune(c) {
			return false
		}
	}
	return zone < 0
}

// isZoneRune reports whether c may appear in an IPv6 zone of a BMP peer key:
// a letter, a digit, '-', '_' or '.'.
func isZoneRune(c rune) bool {
	return c != ':' && isPeerNameRune(c)
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

// transformStatus builds the birdwatcher status object from the four facts
// handleAPIStatus read, each one named for the command that answered it.
//
// The two clock reads are one read: current_server and server_time are the same
// instant, and two calls to time.Now can straddle a second boundary and publish
// two timestamps for one response.
func transformStatus(routerID, release, startTime, lastReload string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	return apiEnvelope("status", map[string]any{
		"router_id":      routerID,
		"current_server": now,
		"server_time":    now,
		"last_reboot":    startTime,
		"last_reconfig":  lastReload,
		"message":        "Ze BGP daemon",
		"version":        release,
	})
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
		name := protocolID(peer)

		received := getNum(peer, "routes-received")
		accepted := getNum(peer, "routes-accepted")
		sent := getNum(peer, "routes-sent")
		filtered := getNum(peer, "routes-filtered")

		protocol := map[string]any{
			"bird_protocol":    name,
			fieldState:         getStr(peer, fieldState),
			"state_changed":    getStr(peer, "state-changed"),
			"neighbor_address": addr,
			"description":      getStr(peer, "description"),
			"last_error":       getStr(peer, "last-error"),
			fieldTable:         "master",
			// Flat fields for simple consumers.
			"routes_received": received,
			"routes_imported": accepted,
			"routes_exported": sent,
			"routes_filtered": filtered,
			// Says whether the four counts above mean anything. See
			// routeCountsAvailable: the zeros are kept for compatibility even
			// when Ze has no source for them.
			"routes_counts_available": routeCountsAvailable(peer),
			"uptime":                  uptimeSeconds(peer),
			// Nested routes object for Alice-LG.
			"routes": map[string]any{
				"imported":  accepted,
				"filtered":  filtered,
				"exported":  sent,
				"preferred": accepted,
			},
		}
		setNeighborAS(protocol, peer, "remote-as", name)
		protocols[name] = protocol
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

		name := protocolID(peer)

		protocols[name] = map[string]any{
			"proto":    "BGP",
			fieldTable: "master",
			fieldState: getStr(peer, fieldState),
			"since":    getStr(peer, "state-changed"),
			"info":     getStr(peer, fieldState),
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
				"origin": getStr(route, "origin"),
				// The birdwatcher-compatible API declares as_path an array of
				// NUMBERS (docs/architecture/api/birdwatcher-compat.md), and
				// bgp/as-notation does NOT reach it. The route row it is built
				// from carries strings under a dotted notation, so the numbers
				// are put back here. This is a machine contract, like the
				// plugin event stream. A consumer that parses an integer must
				// not fail because an operator changed a display preference.
				"as_path":           asPathNumbers(getVal(route, "as-path")),
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

// summaryPeers extracts the peer rows from a "show bgp" response.
//
// handleBgpSummary (internal/component/bgp/plugins/cmd/peer/summary.go) answers
// the aggregates and the rows as siblings, so the rows live at ze["peers"].
// parseJSON promotes a bare JSON array to the same key, so a producer that
// answers only the rows lands here too. Mirrors the navigation in
// handler_ui.go.
func summaryPeers(ze map[string]any) []any {
	peers, _ := ze[fieldPeers].([]any)
	return peers
}

// peerAddress returns a peer's IP. handleBgpSummary emits it as "address"
// (summary.go); "peer-address" is accepted as a fallback for other
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

// routeCountsAvailable reports whether the peer summary carried route counts at
// all, as opposed to Ze being unable to produce them.
//
// The producer is deliberate here and this preserves its decision.
// fetchRibRouteCounts (internal/component/bgp/plugins/cmd/peer/summary.go)
// OMITS routes-received and its siblings when the bgp-rib plugin is not loaded,
// and records that they are "never faked to 0". getNum then returns 0 for the
// missing key, so the transform below publishes a zero the producer refused to
// invent, and an operator cannot tell "this peer sent no routes" from "Ze cannot
// tell you" (ai/rules/evidence.md: a zero value must never be a valid-looking
// answer).
//
// Upstream birdwatcher solves this by omitting the key: its parser returns
// before writing when BIRD prints "---" for a count (alice-lg/birdwatcher,
// bird/parser.go, setChangeCount). Ze keeps the zero instead, by owner decision
// of 2026-08-05, because this is a compatibility surface and Alice-LG's reaction
// to an absent count cannot be tested from here. The truth travels beside it in
// routes_counts_available. Both conventions are recorded in
// docs/architecture/api/birdwatcher-compat.md so the trade can be revisited.
//
// The test is whether the count READS as a number, not whether the key is
// there. A key holding a value this transform cannot read publishes the same
// fabricated zero as an absent key, so key presence alone would answer
// "available" over four zeros: ok proves the key exists, never that the value
// is usable (ai/rules/evidence.md). uptimeSeconds above exists because the
// engine sends uptime as a duration string and getNum answered 0 for every real
// response, so this summary payload has produced that mismatch once already.
//
// routes-received is the probe key: mergeRibRouteCounts emits all three counts
// together or none, so one key answers for the set.
func routeCountsAvailable(m map[string]any) bool {
	_, ok := numValue(m, "routes-received")
	return ok
}

// getNum extracts a numeric value from a map, returning 0 when the key is
// absent or its value is not a number. A caller that must tell those two apart
// from a real 0 calls numValue.
func getNum(m map[string]any, key string) float64 {
	n, _ := numValue(m, key)
	return n
}

// numValue reads a numeric field and reports whether it was present and
// readable. A value wrapped as {"value": N} unwraps first, the same shape
// getStr and getBool accept.
func numValue(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}

	if wrapped, ok := v.(map[string]any); ok {
		if val, ok := wrapped["value"]; ok {
			v = val
		}
	}

	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint32:
		return float64(n), true
	}

	return 0, false
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

// asPathNumbers answers the AS path as the numbers the birdwatcher-compatible
// API declares, whatever notation the route row carried.
//
// A value that is not an AS path is returned unchanged, so a caller sees the
// same thing it saw before this function existed. An element that names no AS
// number is dropped rather than answered as 0.
func asPathNumbers(value any) any {
	path, ok := value.([]any)
	if !ok {
		return value
	}
	numbers := make([]any, 0, len(path))
	for _, element := range path {
		if number, ok := asn.FromJSON(element); ok {
			numbers = append(numbers, number)
		}
	}
	return numbers
}

// setNeighborAS writes the birdwatcher `neighbor_as` of one protocol, read
// from the named AS number field of a Ze peer row.
//
// The row carries the spelling bgp/as-notation selected, so under a dotted
// notation it is the string "1.10". getNum has no case for a string and
// answers 0. RFC 7607 Section 1 reserves AS 0, so a consumer would read a
// syntactically valid AS number that is wrong for every peer
// (ai/rules/principles.md: a silently wrong value must not be reachable).
//
// The key is ALWAYS written. birdwatcher declares it mandatory
// (docs/architecture/api/birdwatcher-compat.md), and a client decoding into a
// struct reads an absent key as 0 anyway, so omitting it buys nothing.
//
// A row that names no readable AS number writes 0 and RAISES A WARNING, so the
// fault reaches an operator rather than passing as data. That case is a Ze
// defect in the row. No notation an operator picks can cause it, because
// asn.Of writes the peer rows (../bgp/plugins/cmd/peer/summary.go).
//
// The report bus rather than a log line, because this runs once per peer per
// request and a looking glass is polled every few seconds. The bus keys an
// active warning on (source, code, subject). A broken row therefore refreshes
// ONE entry in `show warnings` however often it is read, and a repaired row
// clears it on the next request.
func setNeighborAS(protocol, peer map[string]any, key, name string) {
	value := getVal(peer, key)
	number, ok := asn.FromJSON(value)
	if ok {
		report.ClearWarning(reportSourceLG, reportCodeUnreadableAS, name)
	} else {
		report.RaiseWarning(reportSourceLG, reportCodeUnreadableAS, name,
			"a peer row carries no readable AS number, so its neighbor_as is 0",
			map[string]any{"field": key, "value": fmt.Sprint(value)})
	}
	protocol["neighbor_as"] = number
}

// The report bus identity of the fault above. reportCodeUnreadableAS names the
// condition rather than the field, because the two callers read two different
// fields for one answer.
const (
	reportSourceLG         = "looking-glass"
	reportCodeUnreadableAS = "lg-unreadable-as-number"
)
