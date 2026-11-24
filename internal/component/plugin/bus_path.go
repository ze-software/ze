// Design: docs/architecture/api/architecture.md -- short bus participant path syntax and prefix matching
// Related: bus_config.go -- parsed bus filter blocks referencing participant paths
// Related: types.go -- endpoint configuration and on-exit constants

package plugin

import (
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Canonical bus keywords, path prefixes, and external on-exit values.
const (
	BusKeywordNone        = "none"
	BusEndpointPathPrefix = "/bus/endpoint/"
	OnExitWithdraw        = "withdraw"
	OnExitKeep            = "keep"
	MaxBusPathLen         = 256
)

const busPathFix = "write a short slash-separated participant path such as /bus/endpoint/<name>"

// ValidateBusPathSyntax checks that path is a well-formed short bus participant
// path: non-empty, bounded by MaxBusPathLen, starting with '/', having no
// trailing '/', no empty "//" segment, and no whitespace or control characters.
func ValidateBusPathSyntax(path string) error {
	if path == "" {
		return opererr.New("bus path is empty",
			"every bus path must name a participant or prefix in short slash form",
			busPathFix)
	}
	if len(path) > MaxBusPathLen {
		var tb textbuf.Buffer
		return opererr.New(
			tb.Str("bus path ").Quoted(path).Str(" exceeds maximum length ").Int(MaxBusPathLen).String(),
			"bus paths are bounded to prevent unbounded configuration tokens",
			busPathFix)
	}
	if path[0] != '/' {
		var tb textbuf.Buffer
		return opererr.New(
			tb.Str("bus path ").Quoted(path).Str(" must start with '/'").String(),
			"bus paths use the short slash form anchored at the configuration root",
			busPathFix)
	}
	if path == "/" {
		return opererr.New("bus path \"/\" names no participant or section",
			"a bus path must include at least one segment after the leading '/'",
			busPathFix)
	}
	if path[len(path)-1] == '/' {
		var tb textbuf.Buffer
		return opererr.New(
			tb.Str("bus path ").Quoted(path).Str(" must not end with '/'").String(),
			"bus paths use canonical short slash form without trailing slashes",
			busPathFix)
	}
	if strings.Contains(path, "//") {
		var tb textbuf.Buffer
		return opererr.New(
			tb.Str("bus path ").Quoted(path).Str(" contains an empty segment '//'").String(),
			"every segment of a bus path must be non-empty",
			busPathFix)
	}
	for i := range len(path) {
		c := path[i]
		if c <= 0x20 {
			var tb textbuf.Buffer
			return opererr.New(
				tb.Str("bus path ").Quoted(path).Str(" contains whitespace or a control character").String(),
				"bus path segments must contain printable non-whitespace characters only",
				busPathFix)
		}
		if c == 0x7f {
			var tb textbuf.Buffer
			return opererr.New(
				tb.Str("bus path ").Quoted(path).Str(" contains a control character").String(),
				"bus path segments must contain printable non-whitespace characters only",
				busPathFix)
		}
	}
	return nil
}

// ValidateBusPathOrNone checks that val is either BusKeywordNone ("none") or a
// valid short bus participant path.
func ValidateBusPathOrNone(val string) error {
	if val == BusKeywordNone {
		return nil
	}
	return ValidateBusPathSyntax(val)
}

// PathSegments splits a valid short bus path into its non-empty slash-separated
// segments. Empty strings and bare "/" return nil.
func PathSegments(path string) []string {
	if len(path) <= 1 {
		return nil
	}
	if path[0] != '/' {
		return nil
	}
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	for _, seg := range parts {
		if seg == "" {
			return nil
		}
	}
	return parts
}

// PathCovers reports whether parent equals child or is an exact segment-boundary
// prefix of child. For example, "/a" covers "/a" and "/a/b/c", but does NOT
// cover "/ab".
func PathCovers(parent, child string) bool {
	if len(parent) <= 1 {
		return false
	}
	if len(child) <= 1 {
		return false
	}
	if parent[0] != '/' {
		return false
	}
	if child[0] != '/' {
		return false
	}
	if parent[len(parent)-1] == '/' {
		return false
	}
	if child[len(child)-1] == '/' {
		return false
	}
	if parent == child {
		return true
	}
	if len(child) <= len(parent) {
		return false
	}
	if !strings.HasPrefix(child, parent) {
		return false
	}
	return child[len(parent)] == '/'
}

// EndpointPath returns the canonical bus participant path for an endpoint name.
func EndpointPath(name string) string {
	var tb textbuf.Buffer
	return tb.Str(BusEndpointPathPrefix).Str(name).String()
}
