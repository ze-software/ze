// Design: docs/architecture/web-interface.md -- RFC 8522 Looking Glass Command Set API
// Overview: server.go -- LG server and route registration
// Related: handler_api.go -- Birdwatcher REST API handlers
// Related: prefix_lookup.go -- prefix and address validation for route queries
// RFC 8522 -- see rfc/short/rfc8522.md

package lg

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// rfc8522Prefix is the Well-Known URI prefix registered by RFC 8522 Section 5.1.
	rfc8522Prefix   = "/.well-known/looking-glass"
	rfc8522V1Prefix = "/.well-known/looking-glass/v1/"

	// JSend status values defined by RFC 8522 Section 2.3 and Appendix A.
	jsendSuccess = "success"
	jsendFail    = "fail"
	jsendError   = "error"

	// Supported output media types for the RFC 8522 format query parameter.
	rfc8522FormatText = "text/plain"
	rfc8522FormatJSON = "application/json"

	// Command prefixes dispatched through CommandDispatcher for RFC 8522 endpoints.
	cmdShowPing        = "show ping"
	cmdShowTraceroute  = "show traceroute"
	cmdShowRoute       = "show route"
	cmdShowRouteLookup = "show route lookup"
	cmdShowBGPPeer     = "show bgp peer"

	// Default and safety bounds for RFC 8522 query parameters and abuse mitigation.
	defaultRFC8522RuntimeSec      = 30.0
	maxRFC8522RuntimeSec          = 300.0
	defaultRFC8522RateLimitPerMin = 60
	maxRFC8522RateLimitClients    = 1024
	rfc8522RateWindow             = time.Minute
	maxRFC8522HostLen             = 253
	rfc8522AFIv4                  = 1
	rfc8522AFIv6                  = 2
	rfc8522SAFIUnicast            = 1
)

// rfc8522RateEntry tracks fixed-window request counts for one client IP.
type rfc8522RateEntry struct {
	windowStart time.Time
	count       int
}

// rfc8522RateLimiter bounds request rate per client IP to mitigate DDoS abuse
// of scripted looking-glass calls. Safe for concurrent use.
// RFC 8522 Section 6.1: "It is recommended that implementers of the Looking Glass API take steps to mitigate the above described abuse."
type rfc8522RateLimiter struct {
	mu          sync.Mutex
	limitPerMin int
	clients     map[string]rfc8522RateEntry
}

// newRFC8522RateLimiter creates a bounded per-IP rate limiter for RFC 8522 calls.
func newRFC8522RateLimiter(limitPerMin int) *rfc8522RateLimiter {
	limit := limitPerMin
	if limit <= 0 {
		limit = defaultRFC8522RateLimitPerMin
	}
	return &rfc8522RateLimiter{
		limitPerMin: limit,
		clients:     make(map[string]rfc8522RateEntry),
	}
}

// allow reports whether clientIP may issue another RFC 8522 request at now.
// RFC 8522 Section 6.1: "It is recommended that implementers of the Looking Glass API take steps to mitigate the above described abuse."
func (l *rfc8522RateLimiter) allow(clientIP string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.clients) >= maxRFC8522RateLimitClients {
		for k, entry := range l.clients {
			if now.Sub(entry.windowStart) >= rfc8522RateWindow {
				delete(l.clients, k)
			}
		}
		if len(l.clients) >= maxRFC8522RateLimitClients {
			// Evict oldest window entry to keep the map strictly bounded.
			var oldestKey string
			var oldestTime time.Time
			first := true
			for k, entry := range l.clients {
				if first || entry.windowStart.Before(oldestTime) {
					oldestKey = k
					oldestTime = entry.windowStart
					first = false
				}
			}
			if !first {
				delete(l.clients, oldestKey)
			}
		}
	}

	entry, exists := l.clients[clientIP]
	if !exists {
		l.clients[clientIP] = rfc8522RateEntry{windowStart: now, count: 1}
		return true
	}
	if now.Sub(entry.windowStart) >= rfc8522RateWindow {
		l.clients[clientIP] = rfc8522RateEntry{windowStart: now, count: 1}
		return true
	}
	if entry.count >= l.limitPerMin {
		return false
	}
	entry.count++
	l.clients[clientIP] = entry
	return true
}

// rfc8522Protocol holds the parsed protocol query parameter (IANA AFI and optional SAFI).
type rfc8522Protocol struct {
	explicit bool
	afi      int
	hasSAFI  bool
	safi     int
}

// rfc8522Query holds validated RFC 8522 Section 2.2 query parameters.
type rfc8522Query struct {
	protocol   rfc8522Protocol
	runtimeSec float64
	format     string
}

// rfc8522CommandInfo is one entry in the RFC 8522 Section 3.3.3 commands array.
type rfc8522CommandInfo struct {
	Href        string `json:"href"`
	Arguments   string `json:"arguments"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

// isRFC8522Path reports whether path sits under the RFC 8522 well-known prefix.
// RFC 8522 Section 2: "The path component of the resource URI must use the prefix of \".well- known/looking-glass\" (see Section 5.1) to stay namespace neutral."
func isRFC8522Path(path string) bool {
	if path == rfc8522Prefix {
		return true
	}
	return strings.HasPrefix(path, rfc8522Prefix+"/")
}

// handleRFC8522 serves the RFC 8522 Looking Glass Command Set under
// /.well-known/looking-glass/v1/. Every request is stateless HTTP GET over
// HTTPS, dispatched strictly from a closed allowlist of predefined functions
// through CommandDispatcher (never directly between client and router), and
// answered in JSend JSON.
//
// RFC 8522 Section 2: "The path component of the resource URI must use the prefix of \".well- known/looking-glass\" (see Section 5.1) to stay namespace neutral."
// RFC 8522 Section 2: "The client must not be able to execute random commands on the targeting router."
// RFC 8522 Section 2: "There must not be any direct communication between the client and the router."
// RFC 8522 Section 6: "The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit."
func (s *LGServer) handleRFC8522(w http.ResponseWriter, r *http.Request) {
	// RFC 8522 Section 6: "The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit."
	if r.TLS == nil {
		writeRFC8522Error(w, http.StatusBadRequest, "HTTPS is required for the RFC 8522 Looking Glass API", nil)
		return
	}

	// RFC 8522 Section 2: "There must not be any direct communication between the client and the router."
	if r.Header.Get("Upgrade") != "" {
		writeRFC8522Error(w, http.StatusBadRequest, "connection upgrades are not permitted on the Looking Glass API", nil)
		return
	}

	// RFC 8522 Section 6.1: "It is recommended that implementers of the Looking Glass API take steps to mitigate the above described abuse."
	if !s.rfc8522Limiter.allow(rfc8522ClientIP(r), time.Now()) {
		writeRFC8522Error(w, http.StatusTooManyRequests, "rate limit exceeded; retry later", nil)
		return
	}

	// RFC 8522 Section 3: all Looking Glass functions are read-only HTTP GET calls.
	if r.Method != http.MethodGet {
		writeRFC8522Error(w, http.StatusBadRequest, "only HTTP GET is supported by the Looking Glass API", nil)
		return
	}

	// Parse and validate query parameters case-insensitively (RFC 8522 Section 2, 2.2).
	q, errMsg := s.parseRFC8522Query(r)
	if errMsg != "" {
		writeRFC8522Error(w, http.StatusBadRequest, errMsg, nil)
		return
	}

	// Verify exact well-known v1 prefix before matching predefined functions.
	if !strings.HasPrefix(strings.ToLower(r.URL.Path), rfc8522V1Prefix) {
		writeRFC8522Error(w, http.StatusBadRequest, "unknown Looking Glass API path; expected /.well-known/looking-glass/v1/<function>", nil)
		return
	}

	rel := r.URL.Path[len(rfc8522V1Prefix):]
	relLower := strings.ToLower(rel)

	// RFC 8522 Section 2: dispatch strictly through the closed predefined function table.
	switch {
	case relLower == "routers":
		s.handleRFC8522Routers(w, q)
	case strings.HasPrefix(relLower, "routers/"):
		s.handleRFC8522RouterDetails(w, r, rel[len("routers/"):], q)
	case relLower == "cmd":
		s.handleRFC8522Commands(w, r, q)
	case strings.HasPrefix(relLower, "ping/"):
		s.handleRFC8522Ping(w, r, rel[len("ping/"):], q)
	case strings.HasPrefix(relLower, "traceroute/"):
		s.handleRFC8522Traceroute(w, r, rel[len("traceroute/"):], q)
	case strings.HasPrefix(relLower, "show/route/"):
		s.handleRFC8522ShowRoute(w, r, rel[len("show/route/"):], q)
	case relLower == "show/bgp/summary":
		s.handleRFC8522ShowBGPSummary(w, r, q)
	case strings.HasPrefix(relLower, "show/bgp/neighbors/"):
		s.handleRFC8522ShowBGPNeighbors(w, r, rel[len("show/bgp/neighbors/"):], q)
	case strings.HasPrefix(relLower, "show/bgp/"):
		s.handleRFC8522ShowBGP(w, r, rel[len("show/bgp/"):], q)
	default:
		writeRFC8522Error(w, http.StatusBadRequest, "unsupported Looking Glass command; GET /.well-known/looking-glass/v1/cmd lists available commands", nil)
	}
}

// rfc8522ClientIP extracts the host IP from r.RemoteAddr for rate limiting.
func rfc8522ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// parseRFC8522Query parses and validates the RFC 8522 Section 2.2 query
// parameters case-insensitively. The "random" parameter is explicitly ignored.
//
// RFC 8522 Section 2.2: "The server must ignore its value."
func (s *LGServer) parseRFC8522Query(r *http.Request) (rfc8522Query, string) {
	q := rfc8522Query{
		protocol: rfc8522Protocol{
			afi:     rfc8522AFIv4,
			hasSAFI: true,
			safi:    rfc8522SAFIUnicast,
		},
		runtimeSec: defaultRFC8522RuntimeSec,
		format:     rfc8522FormatText,
	}

	raw := r.URL.Query()
	for k, vals := range raw {
		key := strings.ToLower(k)
		val := ""
		if len(vals) > 0 {
			val = vals[0]
		}
		switch key {
		case "random":
			// RFC 8522 Section 2.2: "The server must ignore its value."
			continue
		case "protocol":
			proto, errMsg := parseRFC8522Protocol(val)
			if errMsg != "" {
				return q, errMsg
			}
			q.protocol = proto
		case "router":
			if !strings.EqualFold(strings.TrimSpace(val), s.hostname) {
				return q, "unknown router " + strconv.Quote(val)
			}
		case "routerindex":
			idx, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return q, "invalid routerindex " + strconv.Quote(val) + ": must be a non-negative integer"
			}
			if idx != 0 {
				return q, "unknown routerindex " + strconv.Itoa(idx) + ": router index out of range"
			}
		case "vrf":
			vrfLower := strings.ToLower(strings.TrimSpace(val))
			if vrfLower != "" && vrfLower != "default" && vrfLower != "main" {
				return q, "unsupported vrf " + strconv.Quote(val)
			}
		case "runtime":
			rt, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
			if err != nil {
				return q, "invalid runtime " + strconv.Quote(val) + ": must be a non-negative number of seconds"
			}
			if math.IsNaN(rt) {
				return q, "invalid runtime " + strconv.Quote(val) + ": must be a non-negative number of seconds"
			}
			if math.IsInf(rt, 0) {
				return q, "invalid runtime " + strconv.Quote(val) + ": must be a non-negative number of seconds"
			}
			if rt < 0 {
				return q, "invalid runtime " + strconv.Quote(val) + ": must be >= 0"
			}
			if rt > maxRFC8522RuntimeSec {
				return q, "invalid runtime " + strconv.Quote(val) + ": exceeds maximum of 300 seconds"
			}
			q.runtimeSec = rt
		case "format":
			selected, errMsg := negotiateRFC8522Format(val)
			if errMsg != "" {
				return q, errMsg
			}
			q.format = selected
		default:
			return q, "unknown query parameter " + strconv.Quote(k)
		}
	}

	return q, ""
}

// parseRFC8522Protocol parses an IANA AFI[,SAFI] protocol query value.
func parseRFC8522Protocol(val string) (rfc8522Protocol, string) {
	parts := strings.Split(strings.TrimSpace(val), ",")
	if len(parts) == 0 {
		return rfc8522Protocol{}, "invalid protocol " + strconv.Quote(val) + ": expected <afi> or <afi>,<safi>"
	}
	if len(parts) > 2 {
		return rfc8522Protocol{}, "invalid protocol " + strconv.Quote(val) + ": expected <afi> or <afi>,<safi>"
	}
	afi, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return rfc8522Protocol{}, "invalid protocol AFI in " + strconv.Quote(val)
	}
	if afi <= 0 {
		return rfc8522Protocol{}, "invalid protocol AFI in " + strconv.Quote(val)
	}
	out := rfc8522Protocol{explicit: true, afi: afi}
	if len(parts) == 2 {
		safi, safiErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if safiErr != nil {
			return rfc8522Protocol{}, "invalid protocol SAFI in " + strconv.Quote(val)
		}
		if safi <= 0 {
			return rfc8522Protocol{}, "invalid protocol SAFI in " + strconv.Quote(val)
		}
		out.hasSAFI = true
		out.safi = safi
	}
	return out, ""
}

// negotiateRFC8522Format selects the first supported media type from a
// comma-separated preference list.
func negotiateRFC8522Format(val string) (string, string) {
	if strings.TrimSpace(val) == "" {
		return rfc8522FormatText, ""
	}
	for _, raw := range strings.Split(val, ",") {
		candidate := strings.ToLower(strings.TrimSpace(raw))
		switch candidate {
		case rfc8522FormatText:
			return rfc8522FormatText, ""
		case rfc8522FormatJSON:
			return rfc8522FormatJSON, ""
		}
	}
	return "", "unsupported format " + strconv.Quote(val) + ": supported formats are text/plain, application/json"
}

// checkProtocolMatchesAddr verifies that an explicitly requested IPv4/IPv6 AFI
// agrees with the address literal's family.
func checkProtocolMatchesAddr(proto rfc8522Protocol, addr netip.Addr) string {
	if !proto.explicit {
		return ""
	}
	if proto.afi == rfc8522AFIv4 && !addr.Is4() {
		return "address " + addr.String() + " does not match protocol AFI 1 (IPv4)"
	}
	if proto.afi == rfc8522AFIv6 && !addr.Is6() {
		return "address " + addr.String() + " does not match protocol AFI 2 (IPv6)"
	}
	if proto.afi != rfc8522AFIv4 && proto.afi != rfc8522AFIv6 {
		return "unsupported protocol AFI " + strconv.Itoa(proto.afi) + " for IP address command"
	}
	return ""
}

// validateRFC8522Host validates a ping or traceroute target as either a bare IP
// address or a strict DNS hostname with no whitespace, shell, or pipe characters.
//
// RFC 8522 Section 2: "The client must not be able to execute random commands on the targeting router."
func validateRFC8522Host(host string) (string, netip.Addr, string) {
	if host == "" {
		return "", netip.Addr{}, "target host is required"
	}
	if len(host) > maxRFC8522HostLen {
		return "", netip.Addr{}, "target host exceeds 253 characters"
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Zone() != "" {
			return "", netip.Addr{}, "target address must not carry a zone identifier"
		}
		return addr.String(), addr, ""
	}
	if !isValidRFC8522Hostname(host) {
		return "", netip.Addr{}, "invalid target host " + strconv.Quote(host)
	}
	return host, netip.Addr{}, ""
}

// isValidRFC8522Hostname validates a DNS hostname against RFC 1123 label syntax.
func isValidRFC8522Hostname(host string) bool {
	if host == "" {
		return false
	}
	if len(host) > maxRFC8522HostLen {
		return false
	}
	if host[0] == '-' {
		return false
	}
	if host[0] == '.' {
		return false
	}
	if host[len(host)-1] == '-' {
		return false
	}
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	if len(labels) == 0 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 {
			return false
		}
		if len(label) > 63 {
			return false
		}
		if label[0] == '-' {
			return false
		}
		if label[len(label)-1] == '-' {
			return false
		}
		for i := range len(label) {
			c := label[i]
			isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
			if !isAlphaNum && c != '-' {
				return false
			}
		}
	}
	return true
}

// rfc8522DispatchCtx derives a command context bounded by q.runtimeSec (when > 0).
func rfc8522DispatchCtx(parent context.Context, q rfc8522Query) (context.Context, context.CancelFunc) {
	if q.runtimeSec <= 0 {
		return parent, func() {}
	}
	dur := time.Duration(q.runtimeSec * float64(time.Second))
	return context.WithTimeout(parent, dur)
}

// rfc8522BaseData populates the mandatory RFC 8522 Section 2.3.1 fields inside
// the JSend "data" object: performed_at (UTC ISO 8601) and runtime (seconds).
//
// RFC 8522 Section 2.3.1: "It must also contain a \"data\" object including the following information:"
// RFC 8522 Section 2.3.1: "Combined date and time in UTC ISO 8601 [iso8601] indicating when the operation finished. This information must be present."
// RFC 8522 Section 2.3.1: "Amount of seconds (wallclock) used to run the command. This information must be present."
// RFC 8522 Section 2.3.1: "Adding more information to the response is permitted and must be placed inside the \"data\" object."
func (s *LGServer) rfc8522BaseData(start time.Time) map[string]any {
	now := time.Now().UTC()
	elapsed := math.Round(now.Sub(start).Seconds()*1000) / 1000
	if elapsed < 0 {
		elapsed = 0
	}
	return map[string]any{
		"performed_at": now.Format(time.RFC3339),
		"runtime":      elapsed,
		"router":       s.hostname,
	}
}

// writeRFC8522Success writes a JSend "success" response (HTTP 200).
//
// RFC 8522 Section 2.3: "The response text must comply with the JSON syntax specification JSend, which is briefly explained in Appendix A."
// RFC 8522 Section 2.3: "Consequently, every response must contain a \"status\" field of either \"success\", \"fail\", or \"error\" as explained in the following sections."
// RFC 8522 Section 2.3.1: "A successful response must set the \"status\" field to \"success\"."
// RFC 8522 Section 2.3.1: "The HTTP status code should be 200."
func writeRFC8522Success(w http.ResponseWriter, data map[string]any) {
	writeRFC8522JSON(w, http.StatusOK, map[string]any{
		"status": jsendSuccess,
		"data":   data,
	})
}

// writeRFC8522Fail writes a JSend "fail" response (HTTP 200) when the router
// executed the command and it failed operationally (e.g. 0% ping replies or no
// matching route).
//
// RFC 8522 Section 2.3.2: "The response message must set the \"status\" field to \"fail\" and must contain the \"data\" object with command-specific content listed in Section 2.3.1."
func writeRFC8522Fail(w http.ResponseWriter, data map[string]any) {
	writeRFC8522JSON(w, http.StatusOK, map[string]any{
		"status": jsendFail,
		"data":   data,
	})
}

// writeRFC8522Error writes a JSend "error" response with an appropriate HTTP
// status code (400 client, 500 server, 502 router, 504 timeout) and optional
// "data" object carrying required Section 2.3.1 timing metadata.
//
// RFC 8522 Section 2.3.3: "The response message must set the \"status\" field to \"error\" and must contain the \"message\" key, which keeps a meaningful message, explaining what went wrong."
// RFC 8522 Section 2.3.3: "The response may contain the \"data\" key with required values listed in Section 2.3.1."
// RFC 8522 Section 2.3.3: "The HTTP status code should be 400 in case of a client-side error, 500 in case of a server-side error, or 502 for errors occurring on the target router."
// RFC 8522 Section 2.3.3: "Code 504 should be used when a command timed out."
func writeRFC8522Error(w http.ResponseWriter, httpStatus int, message string, data map[string]any) {
	body := map[string]any{
		"status":  jsendError,
		"message": message,
	}
	if data != nil {
		body["data"] = data
	}
	writeRFC8522JSON(w, httpStatus, body)
}

// writeRFC8522JSON encodes body as JSON with Content-Type: application/json.
func writeRFC8522JSON(w http.ResponseWriter, status int, body map[string]any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		w.Header().Set("Content-Type", rfc8522FormatJSON)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","message":"failed to encode response"}`)) //nolint:errcheck // best-effort fallback on marshal failure
		return
	}
	w.Header().Set("Content-Type", rfc8522FormatJSON)
	w.WriteHeader(status)
	_, _ = w.Write(encoded) //nolint:errcheck // client closed connection after response headers were sent
}

// writeRFC8522EngineFailure maps an engineFailure to an RFC 8522 JSend error:
// 504 on timeout, 502 on router/engine error.
func (s *LGServer) writeRFC8522EngineFailure(w http.ResponseWriter, start time.Time, failure *engineFailure) {
	data := s.rfc8522BaseData(start)
	if failure.timedOut {
		writeRFC8522Error(w, http.StatusGatewayTimeout, "command timed out on router", data)
		return
	}
	writeRFC8522Error(w, http.StatusBadGateway, failure.command+": "+failure.class, data)
}

// handleRFC8522Routers implements GET /.well-known/looking-glass/v1/routers.
//
// RFC 8522 Section 3.3: "The following organizational commands must be included in the implementation."
func (s *LGServer) handleRFC8522Routers(w http.ResponseWriter, _ rfc8522Query) {
	start := time.Now().UTC()
	data := s.rfc8522BaseData(start)
	data["routers"] = []string{s.hostname}
	writeRFC8522Success(w, data)
}

// handleRFC8522RouterDetails implements GET /.well-known/looking-glass/v1/routers/{number}.
//
// RFC 8522 Section 3.3: "The following organizational commands must be included in the implementation."
// RFC 8522 Section 3.3.2: "The response must contain the router's hostname and router index."
func (s *LGServer) handleRFC8522RouterDetails(w http.ResponseWriter, r *http.Request, rawNumber string, q rfc8522Query) {
	start := time.Now().UTC()
	if strings.Contains(rawNumber, "/") {
		writeRFC8522Error(w, http.StatusBadRequest, "invalid router index "+strconv.Quote(rawNumber), nil)
		return
	}
	idx, err := strconv.Atoi(rawNumber)
	if err != nil {
		writeRFC8522Error(w, http.StatusBadRequest, "invalid router index "+strconv.Quote(rawNumber)+": must be a non-negative integer", nil)
		return
	}
	if idx != 0 {
		writeRFC8522Error(w, http.StatusBadRequest, "unknown router index "+strconv.Itoa(idx), nil)
		return
	}

	data := s.rfc8522BaseData(start)
	data["id"] = 0
	data["name"] = s.hostname
	data["format"] = rfc8522FormatText + "," + rfc8522FormatJSON
	data["vendor"] = "Ze"

	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()
	if overview, failure := s.askCtx(ctx, cmdBGPOverview); failure == nil {
		if localAS, ok := numValue(overview, "local-as"); ok && localAS > 0 {
			data["autonomous_system"] = uint32(localAS)
		}
	}

	writeRFC8522Success(w, data)
}

// handleRFC8522Commands implements GET /.well-known/looking-glass/v1/cmd.
//
// RFC 8522 Section 3.3: "The following organizational commands must be included in the implementation."
func (s *LGServer) handleRFC8522Commands(w http.ResponseWriter, r *http.Request, _ rfc8522Query) {
	start := time.Now().UTC()
	base := "https://" + r.Host + rfc8522V1Prefix
	if r.Host == "" {
		base = rfc8522V1Prefix
	}

	commands := []rfc8522CommandInfo{
		{
			Href:        base + "ping",
			Arguments:   "{host}",
			Description: "Send ICMP echo requests to validate reachability and measure round-trip time",
			Command:     "ping",
		},
		{
			Href:        base + "traceroute",
			Arguments:   "{host}",
			Description: "Trace route to destination host",
			Command:     "traceroute",
		},
		{
			Href:        base + "show/route",
			Arguments:   "{addr}",
			Description: "Print records from IP routing table",
			Command:     "show route",
		},
		{
			Href:        base + "show/bgp",
			Arguments:   "{addr}",
			Description: "Display matching records from the BGP routing table",
			Command:     "show bgp",
		},
		{
			Href:        base + "show/bgp/summary",
			Arguments:   "",
			Description: "Print a summary of BGP neighbor status",
			Command:     "show bgp summary",
		},
		{
			Href:        base + "show/bgp/neighbors",
			Arguments:   "{addr}",
			Description: "Display detailed information on a BGP neighbor",
			Command:     "show bgp neighbors",
		},
		{
			Href:        base + "routers",
			Arguments:   "",
			Description: "List routers available for command execution",
			Command:     "routers",
		},
		{
			Href:        base + "routers",
			Arguments:   "{number}",
			Description: "List details for a selected router",
			Command:     "routers",
		},
		{
			Href:        base + "cmd",
			Arguments:   "",
			Description: "List available Looking Glass commands",
			Command:     "cmd",
		},
	}

	data := s.rfc8522BaseData(start)
	data["commands"] = commands
	writeRFC8522Success(w, data)
}

// handleRFC8522Ping implements GET /.well-known/looking-glass/v1/ping/{host}.
//
// RFC 8522 Section 3.1.1: "Implementation of the ping command is mandatory."
func (s *LGServer) handleRFC8522Ping(w http.ResponseWriter, r *http.Request, rawHost string, q rfc8522Query) {
	start := time.Now().UTC()
	if strings.Contains(rawHost, "/") {
		writeRFC8522Error(w, http.StatusBadRequest, "invalid ping host "+strconv.Quote(rawHost), nil)
		return
	}
	host, addr, errMsg := validateRFC8522Host(rawHost)
	if errMsg != "" {
		writeRFC8522Error(w, http.StatusBadRequest, errMsg, nil)
		return
	}
	if addr.IsValid() {
		if protoErr := checkProtocolMatchesAddr(q.protocol, addr); protoErr != "" {
			writeRFC8522Error(w, http.StatusBadRequest, protoErr, nil)
			return
		}
	}

	var tb textbuf.Buffer
	tb.Str(cmdShowPing).Byte(' ').Str(host)
	if q.runtimeSec > 0 && q.runtimeSec < 5 {
		timeoutSec := max(int64(math.Ceil(q.runtimeSec)), 1)
		tb.Str(" timeout ").Int(timeoutSec).Byte('s')
	}

	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()
	zeData, failure := s.askCtx(ctx, tb.String())
	if failure != nil {
		s.writeRFC8522EngineFailure(w, start, failure)
		return
	}

	sent, _ := numValue(zeData, "sent")
	received, _ := numValue(zeData, "received")
	lossPercent, hasLoss := numValue(zeData, "loss-percent")
	rate := 0
	if hasLoss {
		rate = int(math.Round(100 - lossPercent))
	} else if sent > 0 {
		rate = int(math.Round(received / sent * 100))
	}
	if rate < 0 {
		rate = 0
	}
	if rate > 100 {
		rate = 100
	}

	data := s.rfc8522BaseData(start)
	data["format"] = q.format
	data["rate"] = rate
	if minRTT, ok := numValue(zeData, "min-rtt-ms"); ok {
		data["min"] = minRTT
	}
	if avgRTT, ok := numValue(zeData, "avg-rtt-ms"); ok {
		data["avg"] = avgRTT
	}
	if maxRTT, ok := numValue(zeData, "max-rtt-ms"); ok {
		data["max"] = maxRTT
	}
	if q.format == rfc8522FormatJSON {
		data["output"] = zeData
	} else {
		data["output"] = formatRFC8522PingLines(host, zeData, int(sent), int(received), rate)
	}

	if received <= 0 {
		writeRFC8522Fail(w, data)
		return
	}
	writeRFC8522Success(w, data)
}

// formatRFC8522PingLines renders a show ping JSON payload into plain text lines.
func formatRFC8522PingLines(host string, zeData map[string]any, sent, received, rate int) []string {
	dest := getStr(zeData, "destination")
	if dest == "" {
		dest = host
	}
	lines := make([]string, 0, 3)
	var tb textbuf.Buffer
	tb.Str("Sending ").Int(int64(sent)).Str(" ICMP Echos to ").Str(dest)
	lines = append(lines, tb.String())

	if replies, ok := zeData["replies"].([]any); ok && len(replies) > 0 {
		var dots textbuf.Buffer
		for _, item := range replies {
			rep, isMap := item.(map[string]any)
			if !isMap {
				continue
			}
			if getStr(rep, "status") == "ok" {
				dots.Byte('!')
			} else {
				dots.Byte('.')
			}
		}
		if dots.Len() > 0 {
			lines = append(lines, dots.String())
		}
	}

	tb.Reset()
	tb.Str("Success rate is ").Int(int64(rate)).Str(" percent (").Int(int64(received)).Byte('/').Int(int64(sent)).Byte(')')
	lines = append(lines, tb.String())
	return lines
}

// handleRFC8522Traceroute implements GET /.well-known/looking-glass/v1/traceroute/{host}.
//
// RFC 8522 Section 3.1.2: optional traceroute diagnostic command.
func (s *LGServer) handleRFC8522Traceroute(w http.ResponseWriter, r *http.Request, rawHost string, q rfc8522Query) {
	start := time.Now().UTC()
	if strings.Contains(rawHost, "/") {
		writeRFC8522Error(w, http.StatusBadRequest, "invalid traceroute host "+strconv.Quote(rawHost), nil)
		return
	}
	host, addr, errMsg := validateRFC8522Host(rawHost)
	if errMsg != "" {
		writeRFC8522Error(w, http.StatusBadRequest, errMsg, nil)
		return
	}
	if addr.IsValid() {
		if protoErr := checkProtocolMatchesAddr(q.protocol, addr); protoErr != "" {
			writeRFC8522Error(w, http.StatusBadRequest, protoErr, nil)
			return
		}
	}

	var tb textbuf.Buffer
	tb.Str(cmdShowTraceroute).Byte(' ').Str(host)

	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()
	zeData, failure := s.askCtx(ctx, tb.String())
	if failure != nil {
		s.writeRFC8522EngineFailure(w, start, failure)
		return
	}

	hops, _ := zeData["hops"].([]any)
	data := s.rfc8522BaseData(start)
	data["format"] = q.format
	if q.format == rfc8522FormatJSON {
		data["output"] = zeData
	} else {
		data["output"] = formatRFC8522TracerouteLines(host, hops)
	}

	if len(hops) == 0 {
		writeRFC8522Fail(w, data)
		return
	}
	writeRFC8522Success(w, data)
}

// formatRFC8522TracerouteLines renders a show traceroute hops slice into text lines.
func formatRFC8522TracerouteLines(host string, hops []any) []string {
	lines := make([]string, 0, len(hops)+1)
	lines = append(lines, "Tracing the route to "+host)
	var tb textbuf.Buffer
	for _, item := range hops {
		hop, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tb.Reset()
		if ttl, hasTTL := numValue(hop, "ttl"); hasTTL {
			tb.Int(int64(ttl)).Byte(' ')
		}
		hopAddr := getStr(hop, "address")
		if hopAddr == "" {
			hopAddr = "*"
		}
		tb.Str(hopAddr)
		if rtt, hasRTT := numValue(hop, "rtt-ms"); hasRTT {
			tb.Byte(' ').Str(strconv.FormatFloat(rtt, 'f', 2, 64)).Str(" msec")
		}
		lines = append(lines, tb.String())
	}
	return lines
}

// handleRFC8522ShowRoute implements GET /.well-known/looking-glass/v1/show/route/{addr}.
// A CIDR prefix queries `show route <cidr>`; a bare IP address queries
// `show route lookup <ip>`.
//
// RFC 8522 Section 3.2.1: "Implementation of the \"show route\" command is mandatory."
func (s *LGServer) handleRFC8522ShowRoute(w http.ResponseWriter, r *http.Request, rawAddr string, q rfc8522Query) {
	start := time.Now().UTC()
	lookup, parseErr := parsePrefixLookup(rawAddr)
	if parseErr != nil {
		writeRFC8522Error(w, http.StatusBadRequest, parseErr.Error(), nil)
		return
	}

	if protoErr := checkProtocolMatchesLookup(q.protocol, lookup); protoErr != "" {
		writeRFC8522Error(w, http.StatusBadRequest, protoErr, nil)
		return
	}

	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()

	var tb textbuf.Buffer
	if lookup.keyword == filterExact {
		tb.Str(cmdShowRoute).Byte(' ').Str(lookup.value)
		zeData, failure := s.askCtx(ctx, tb.String())
		if failure != nil {
			s.writeRFC8522EngineFailure(w, start, failure)
			return
		}
		routes, _ := zeData["routes"].([]any)
		data := s.rfc8522BaseData(start)
		data["format"] = q.format
		if q.format == rfc8522FormatJSON {
			data["output"] = zeData
		} else {
			data["output"] = formatRFC8522KernelRouteLines(routes)
		}
		if len(routes) == 0 {
			writeRFC8522Fail(w, data)
			return
		}
		writeRFC8522Success(w, data)
		return
	}

	tb.Str(cmdShowRouteLookup).Byte(' ').Str(lookup.value)
	zeData, failure := s.askCtx(ctx, tb.String())
	if failure != nil {
		if failure.envelope && isNoRouteRefusal(failure.refusal) {
			data := s.rfc8522BaseData(start)
			data["format"] = q.format
			if q.format == rfc8522FormatJSON {
				data["output"] = map[string]any{"destination": lookup.value, "found": false}
			} else {
				data["output"] = []string{"% Network not in table"}
			}
			writeRFC8522Fail(w, data)
			return
		}
		s.writeRFC8522EngineFailure(w, start, failure)
		return
	}

	data := s.rfc8522BaseData(start)
	data["format"] = q.format
	if q.format == rfc8522FormatJSON {
		data["output"] = zeData
	} else {
		data["output"] = formatRFC8522KernelRouteLines([]any{zeData})
	}
	writeRFC8522Success(w, data)
}

// isNoRouteRefusal reports whether a `show route lookup` error envelope
// indicates that no route exists in the routing table (JSend "fail").
func isNoRouteRefusal(refusal string) bool {
	lower := strings.ToLower(refusal)
	if strings.Contains(lower, "no route") {
		return true
	}
	if strings.Contains(lower, "not found") {
		return true
	}
	return strings.Contains(lower, "unreachable")
}

// checkProtocolMatchesLookup validates an explicit protocol AFI against a
// parsed prefix or address lookup.
func checkProtocolMatchesLookup(proto rfc8522Protocol, lookup prefixLookup) string {
	if !proto.explicit {
		return ""
	}
	if lookup.keyword == filterExact {
		pfx, err := netip.ParsePrefix(lookup.value)
		if err != nil {
			return ""
		}
		return checkProtocolMatchesAddr(proto, pfx.Addr())
	}
	addr, err := netip.ParseAddr(lookup.value)
	if err != nil {
		return ""
	}
	return checkProtocolMatchesAddr(proto, addr)
}

// formatRFC8522KernelRouteLines formats kernel routing table entries into text lines.
func formatRFC8522KernelRouteLines(routes []any) []string {
	if len(routes) == 0 {
		return []string{"% Network not in table"}
	}
	lines := make([]string, 0, len(routes))
	var tb textbuf.Buffer
	for _, item := range routes {
		rt, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tb.Reset()
		dst := getStr(rt, "dst")
		if dst == "" {
			dst = getStr(rt, "prefix")
		}
		if dst == "" {
			dst = getStr(rt, "destination")
		}
		proto := getStr(rt, "protocol")
		if proto != "" {
			tb.Str(proto).Byte(' ')
		}
		if dst != "" {
			tb.Str(dst)
		}
		if gw := getStr(rt, "gateway"); gw != "" {
			tb.Str(" via ").Str(gw)
		}
		if iface := getStr(rt, "interface"); iface != "" {
			tb.Str(" dev ").Str(iface)
		}
		if metric, hasMetric := numValue(rt, "metric"); hasMetric {
			tb.Str(" metric ").Int(int64(metric))
		}
		if tb.Len() > 0 {
			lines = append(lines, tb.String())
		}
	}
	return lines
}

// handleRFC8522ShowBGP implements GET /.well-known/looking-glass/v1/show/bgp/{addr}.
//
// RFC 8522 Section 3.2.2: optional BGP table lookup command.
func (s *LGServer) handleRFC8522ShowBGP(w http.ResponseWriter, r *http.Request, rawAddr string, q rfc8522Query) {
	start := time.Now().UTC()
	lookup, parseErr := parsePrefixLookup(rawAddr)
	if parseErr != nil {
		writeRFC8522Error(w, http.StatusBadRequest, parseErr.Error(), nil)
		return
	}
	if protoErr := checkProtocolMatchesLookup(q.protocol, lookup); protoErr != "" {
		writeRFC8522Error(w, http.StatusBadRequest, protoErr, nil)
		return
	}

	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()
	zeData, failure := s.askCtx(ctx, lookup.receivedCommand())
	if failure != nil {
		s.writeRFC8522EngineFailure(w, start, failure)
		return
	}

	routes := extractRoutes(zeData)
	data := s.rfc8522BaseData(start)
	data["format"] = q.format
	if q.format == rfc8522FormatJSON {
		data["output"] = zeData
	} else {
		data["output"] = formatRFC8522BGPRouteLines(lookup.value, routes)
	}
	if len(routes) == 0 {
		writeRFC8522Fail(w, data)
		return
	}
	writeRFC8522Success(w, data)
}

// formatRFC8522BGPRouteLines renders BGP RIB routes into human-readable lines.
func formatRFC8522BGPRouteLines(target string, routes []any) []string {
	if len(routes) == 0 {
		return []string{"% Network not in table: " + target}
	}
	lines := make([]string, 0, len(routes)+1)
	lines = append(lines, "BGP routing table entry for "+target)
	var tb textbuf.Buffer
	for _, item := range routes {
		rt, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tb.Reset()
		tb.Str(getStr(rt, "prefix"))
		if nh := getStr(rt, "next-hop"); nh != "" {
			tb.Str(" next-hop ").Str(nh)
		}
		if peer := getStr(rt, "peer"); peer != "" {
			tb.Str(" from ").Str(peer)
		}
		if path := formatASPathAny(rt["as-path"]); path != "" {
			tb.Str(" as-path ").Str(path)
		}
		if origin := getStr(rt, "origin"); origin != "" {
			tb.Str(" origin ").Str(origin)
		}
		lines = append(lines, tb.String())
	}
	return lines
}

// formatASPathAny renders a route's as-path (string or slice) as space-separated text.
func formatASPathAny(v any) string {
	switch p := v.(type) {
	case string:
		return p
	case []any:
		var tb textbuf.Buffer
		for i, item := range p {
			if i > 0 {
				tb.Byte(' ')
			}
			switch n := item.(type) {
			case float64:
				tb.Uint(uint64(n))
			case string:
				tb.Str(n)
			}
		}
		return tb.String()
	default:
		return ""
	}
}

// handleRFC8522ShowBGPSummary implements GET /.well-known/looking-glass/v1/show/bgp/summary.
//
// RFC 8522 Section 3.2.3: optional BGP summary command.
func (s *LGServer) handleRFC8522ShowBGPSummary(w http.ResponseWriter, r *http.Request, q rfc8522Query) {
	start := time.Now().UTC()
	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()

	zeData, failure := s.askCtx(ctx, cmdBGPOverview)
	if failure != nil {
		s.writeRFC8522EngineFailure(w, start, failure)
		return
	}

	data := s.rfc8522BaseData(start)
	data["format"] = q.format
	if q.format == rfc8522FormatJSON {
		data["output"] = zeData
	} else {
		data["output"] = formatRFC8522BGPSummaryLines(zeData)
	}
	writeRFC8522Success(w, data)
}

// formatRFC8522BGPSummaryLines renders a `show bgp` overview into text lines.
func formatRFC8522BGPSummaryLines(zeData map[string]any) []string {
	peers := summaryPeers(zeData)
	lines := make([]string, 0, len(peers)+2)
	var tb textbuf.Buffer
	tb.Str("BGP router identifier ").Str(getStr(zeData, "router-id"))
	if localAS, ok := numValue(zeData, "local-as"); ok {
		tb.Str(", local AS number ").Uint(uint64(localAS))
	}
	lines = append(lines, tb.String())
	lines = append(lines, "Neighbor AS State Up/Down PfxRcd")
	for _, p := range peers {
		peer, ok := p.(map[string]any)
		if !ok {
			continue
		}
		tb.Reset()
		tb.Str(peerAddress(peer))
		if remoteAS, hasAS := numValue(peer, "remote-as"); hasAS {
			tb.Byte(' ').Uint(uint64(remoteAS))
		}
		tb.Byte(' ').Str(getStr(peer, fieldState))
		if up := getStr(peer, "uptime"); up != "" {
			tb.Byte(' ').Str(up)
		}
		if pfx, hasPfx := numValue(peer, "routes-received"); hasPfx {
			tb.Byte(' ').Int(int64(pfx))
		}
		lines = append(lines, tb.String())
	}
	return lines
}

// handleRFC8522ShowBGPNeighbors implements GET /.well-known/looking-glass/v1/show/bgp/neighbors/{addr}.
//
// RFC 8522 Section 3.2.4: optional BGP neighbor detail command.
func (s *LGServer) handleRFC8522ShowBGPNeighbors(w http.ResponseWriter, r *http.Request, rawAddr string, q rfc8522Query) {
	start := time.Now().UTC()
	if strings.Contains(rawAddr, "/") {
		writeRFC8522Error(w, http.StatusBadRequest, "invalid neighbor address "+strconv.Quote(rawAddr), nil)
		return
	}
	addr, err := netip.ParseAddr(rawAddr)
	if err != nil {
		writeRFC8522Error(w, http.StatusBadRequest, "invalid neighbor address "+strconv.Quote(rawAddr), nil)
		return
	}
	if addr.Zone() != "" {
		writeRFC8522Error(w, http.StatusBadRequest, "neighbor address must not carry a zone identifier", nil)
		return
	}
	if protoErr := checkProtocolMatchesAddr(q.protocol, addr); protoErr != "" {
		writeRFC8522Error(w, http.StatusBadRequest, protoErr, nil)
		return
	}

	var tb textbuf.Buffer
	tb.Str(cmdShowBGPPeer).Byte(' ').Str(addr.String()).Str(" detail")

	ctx, cancel := rfc8522DispatchCtx(r.Context(), q)
	defer cancel()
	zeData, failure := s.askCtx(ctx, tb.String())
	if failure != nil {
		if failure.envelope {
			data := s.rfc8522BaseData(start)
			data["format"] = q.format
			if q.format == rfc8522FormatJSON {
				data["output"] = map[string]any{"neighbor": addr.String(), "found": false}
			} else {
				data["output"] = []string{"% No such neighbor " + addr.String()}
			}
			writeRFC8522Fail(w, data)
			return
		}
		s.writeRFC8522EngineFailure(w, start, failure)
		return
	}

	peersMap, _ := zeData[fieldPeers].(map[string]any)
	peerObj, found := peersMap[addr.String()].(map[string]any)
	data := s.rfc8522BaseData(start)
	data["format"] = q.format
	if !found {
		if q.format == rfc8522FormatJSON {
			data["output"] = map[string]any{"neighbor": addr.String(), "found": false}
		} else {
			data["output"] = []string{"% No such neighbor " + addr.String()}
		}
		writeRFC8522Fail(w, data)
		return
	}

	if q.format == rfc8522FormatJSON {
		data["output"] = peerObj
	} else {
		data["output"] = formatRFC8522NeighborLines(addr.String(), peerObj)
	}
	writeRFC8522Success(w, data)
}

// formatRFC8522NeighborLines renders a BGP peer detail object into text lines.
func formatRFC8522NeighborLines(addr string, peer map[string]any) []string {
	lines := make([]string, 0, 3)
	var tb textbuf.Buffer
	tb.Str("BGP neighbor is ").Str(addr)
	if remoteAS, ok := numValue(peer, "remote-as"); ok {
		tb.Str(", remote AS ").Uint(uint64(remoteAS))
	}
	if peerType := getStr(peer, "peer-type"); peerType != "" {
		tb.Str(", ").Str(peerType).Str(" link")
	}
	lines = append(lines, tb.String())

	tb.Reset()
	tb.Str("  BGP router ID ").Str(getStr(peer, "router-id"))
	lines = append(lines, tb.String())

	tb.Reset()
	tb.Str("  BGP state = ").Str(getStr(peer, fieldState))
	if up := getStr(peer, "uptime"); up != "" {
		tb.Str(", up for ").Str(up)
	}
	lines = append(lines, tb.String())
	return lines
}
