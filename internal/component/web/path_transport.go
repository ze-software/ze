// Design: docs/architecture/web-interface.md -- reversible schema-token paths.

package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// encodeConfigPath serializes schema tokens without giving key bytes a routing
// meaning. The result is also the browser CLI's context transport.
func encodeConfigPath(path []string) string {
	escaped := make([]string, len(path))
	for i, token := range path {
		// Browsers normalize both . and %2E as a path segment. A non-dot
		// carrier survives that normalization; doubling an initial marker
		// keeps literal marker keys distinct after the URI decode as well.
		if token == "." {
			token = "~."
		} else if strings.HasPrefix(token, "~") {
			var carrier textbuf.Buffer
			token = carrier.Byte('~').Str(token).String()
		}
		escaped[i] = url.PathEscape(token)
	}
	return strings.Join(escaped, "/")
}

// decodeConfigPath splits before unescaping, exactly once. Schema resolution
// remains responsible for distinguishing node names from typed list keys.
func decodeConfigPath(path string) ([]string, error) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return nil, nil
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return nil, fmt.Errorf("invalid path encoding: %w", err)
		}
		if decoded == "~." {
			decoded = "."
		} else if strings.HasPrefix(decoded, "~~") {
			decoded = decoded[1:]
		}
		segments[i] = decoded
	}
	if err := ValidatePathSegments(segments); err != nil {
		return nil, err
	}
	return segments, nil
}

// escapedRequestPath rejects inconsistent RawPath values rather than allowing
// EscapedPath's fallback to silently turn a malformed encoding into a key.
func escapedRequestPath(r *http.Request) (string, error) {
	if r.URL.RawPath != "" {
		decoded, err := url.PathUnescape(r.URL.RawPath)
		if err != nil {
			return "", fmt.Errorf("invalid URL path encoding")
		}
		if decoded != r.URL.Path {
			return "", fmt.Errorf("invalid URL path encoding")
		}
	}
	return r.URL.EscapedPath(), nil
}

func fragmentURL(path string) string {
	return "/fragment/detail?path=" + url.QueryEscape(path)
}

// validPathToken checks transport safety, not the YANG identifier grammar:
// list keys have the type and character set declared by their schema.
func validPathToken(token string) bool {
	if !utf8.ValidString(token) {
		return false
	}
	for _, ch := range token {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}
