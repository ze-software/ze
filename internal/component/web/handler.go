// Design: docs/architecture/web-interface.md -- URL routing and content negotiation
// Related: handler_config.go -- Config tree view and edit handlers
// Related: cli.go -- CLI bar and terminal mode handlers
// Related: handler_admin.go -- Admin command handlers

// Package web provides the HTTP handler layer for ze's web interface.
//
// URLs follow a verb-first three-tier scheme:
//
//	/show/<yang-path>       -- view tier (GET)
//	/monitor/<yang-path>    -- view tier with auto-poll (GET)
//	/config/<verb>/<path>   -- config tier (edit/set/delete/commit/discard/compare)
//	/admin/<yang-path>      -- admin tier (POST)
//	/login                  -- authentication (POST, no auth required)
//	/assets/                -- static assets (GET, no auth required)
//	/                       -- redirects to /show/
package web

import (
	"fmt"
	"net/http"
	"strings"
)

// Tier represents the authorization tier of a request.
type Tier int

const (
	// TierView is the read-only view tier (show, monitor).
	TierView Tier = iota
	// TierConfig is the configuration tier (edit, set, delete, commit, discard, compare).
	TierConfig
	// TierAdmin is the administrative tier (operational mutations).
	TierAdmin
)

// Content format constants.
const (
	formatJSON = "json"
	formatHTML = "html"
)

// ParsedURL holds the decomposed parts of a request URL.
type ParsedURL struct {
	// Tier is the authorization tier derived from the URL prefix.
	Tier Tier
	// Verb is the action: "show", "monitor", "edit", "set", "delete", "commit", "discard", "compare", or "admin".
	Verb string
	// Path contains the YANG path segments after the verb (and config verb, if config tier).
	Path []string
	// Format is "html" or "json", determined by content negotiation.
	Format string
}

// configVerbs is the set of valid verbs under /config/.
var configVerbs = map[string]bool{
	"edit":     true,
	"set":      true,
	"add":      true,
	"add-form": true,
	"changes":  true,
	"delete":   true,
	"rename":   true,
	"commit":   true,
	"discard":  true,
	"compare":  true,
}

// knownPrefixes maps top-level URL prefixes to their handler logic.
var knownPrefixes = map[string]bool{
	"show":    true,
	"monitor": true,
	"config":  true,
	"admin":   true,
	"portal":  true,
	"login":   true,
	"assets":  true,
}

// ParseURL parses an HTTP request URL into a ParsedURL.
// It extracts the tier, verb, YANG path segments, and negotiated format.
// Returns an error for unrecognized prefixes or invalid config verbs.
func ParseURL(r *http.Request) (ParsedURL, error) {
	format := NegotiateContentType(r)

	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return ParsedURL{Tier: TierView, Verb: "show", Format: format}, nil
	}

	parts := strings.SplitN(path, "/", 2)
	prefix := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}

	if !knownPrefixes[prefix] {
		return ParsedURL{}, fmt.Errorf("unknown URL prefix: %q", prefix)
	}

	var segments []string
	if rest != "" {
		segments = strings.Split(rest, "/")
	}

	switch prefix {
	case "show":
		if err := ValidatePathSegments(segments); err != nil {
			return ParsedURL{}, err
		}
		return ParsedURL{Tier: TierView, Verb: "show", Path: segments, Format: format}, nil

	case "monitor":
		if err := ValidatePathSegments(segments); err != nil {
			return ParsedURL{}, err
		}
		return ParsedURL{Tier: TierView, Verb: "monitor", Path: segments, Format: format}, nil

	case "config":
		return parseConfigURL(segments, format)

	case "admin":
		if err := ValidatePathSegments(segments); err != nil {
			return ParsedURL{}, err
		}
		return ParsedURL{Tier: TierAdmin, Verb: "admin", Path: segments, Format: format}, nil

	case "portal":
		return ParsedURL{Tier: TierView, Verb: "portal", Path: segments, Format: format}, nil

	case "login":
		return ParsedURL{Verb: "login", Format: format}, nil

	case "assets":
		return ParsedURL{Verb: "assets", Path: segments, Format: format}, nil
	}

	// Unreachable: knownPrefixes check above guarantees prefix is in the switch.
	return ParsedURL{}, fmt.Errorf("unknown URL prefix: %q", prefix)
}

// parseConfigURL handles /config/<verb>/<yang-path> URLs.
func parseConfigURL(segments []string, format string) (ParsedURL, error) {
	if len(segments) == 0 {
		return ParsedURL{}, fmt.Errorf("missing config verb, expected /config/<verb>/<path>")
	}

	verb := segments[0]
	if !configVerbs[verb] {
		return ParsedURL{}, fmt.Errorf("unknown config verb: %q", verb)
	}

	yangSegments := segments[1:]
	if err := ValidatePathSegments(yangSegments); err != nil {
		return ParsedURL{}, err
	}

	return ParsedURL{Tier: TierConfig, Verb: verb, Path: yangSegments, Format: format}, nil
}

// ValidatePathSegments rejects path segments that are unsafe or invalid
// as YANG identifiers. It checks for path traversal (..), empty segments,
// null bytes, and characters outside the set [a-zA-Z0-9._:-].
func ValidatePathSegments(segments []string) error {
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("empty path segment (double slash)")
		}
		if seg == ".." {
			return fmt.Errorf("path traversal not allowed")
		}
		if strings.ContainsRune(seg, 0) {
			return fmt.Errorf("null byte in path segment")
		}
		for _, ch := range seg {
			if !isYANGIdentChar(ch) {
				return fmt.Errorf("invalid character %q in path segment %q", ch, seg)
			}
		}
	}
	return nil
}

// isYANGIdentChar returns true for characters valid in YANG identifiers
// plus IP address characters: [a-zA-Z0-9._:-].
func isYANGIdentChar(ch rune) bool {
	if ch >= 'a' && ch <= 'z' {
		return true
	}
	if ch >= 'A' && ch <= 'Z' {
		return true
	}
	if ch >= '0' && ch <= '9' {
		return true
	}
	return ch == '.' || ch == '_' || ch == ':' || ch == '-'
}

// NegotiateContentType determines the response format from the request.
// It checks the ?format= query parameter first (URL wins), then the Accept header.
// Returns "json" or "html". Default is "html".
func NegotiateContentType(r *http.Request) string {
	// URL parameter takes precedence.
	if f := r.URL.Query().Get("format"); f == formatJSON {
		return formatJSON
	}

	// Fall back to Accept header. HTML wins when both are present.
	accept := r.Header.Get("Accept")
	if accept != "" && strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		return formatJSON
	}

	return formatHTML
}

// RegisterRoutes registers route patterns on the given ServeMux.
// The auth handler wraps routes that require authentication.
// The assets handler serves static files under /assets/.
func RegisterRoutes(mux *http.ServeMux, auth, assets http.Handler) {
	mux.Handle("/assets/", assets)
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		auth.ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)
			return
		}
		auth.ServeHTTP(w, r)
	})
}

// RegisterCLIRoutes registers CLI bar endpoint handlers on the given ServeMux.
// All routes require authentication (callers use GetUsernameFromRequest).
// The auth middleware must wrap these handlers before registration.
func RegisterCLIRoutes(mux *http.ServeMux, authWrap func(http.HandlerFunc) http.Handler, cliCmd, cliComplete, cliTerminal, cliMode http.HandlerFunc) {
	mux.Handle("/cli", authWrap(cliCmd))
	mux.Handle("/cli/complete", authWrap(cliComplete))
	mux.Handle("/cli/terminal", authWrap(cliTerminal))
	mux.Handle("/cli/mode", authWrap(cliMode))
}
