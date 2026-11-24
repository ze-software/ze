// Design: docs/architecture/api/architecture.md -- tests for short bus path syntax and prefix matching
// Related: bus_path.go -- implementation under test

package plugin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateBusPathSyntax verifies short bus path syntax rules.
//
// VALIDATES: Valid short slash paths pass; empty, relative, trailing-slash,
// double-slash, whitespace, control-char, and overlong paths are refused.
// PREVENTS: Malformed bus participant paths entering config or filter matching.
func TestValidateBusPathSyntax(t *testing.T) {
	valid := []string{
		"/bus/endpoint/collector",
		"/proto/node/edge1",
		"/proto/domain/transit/node/t1",
		"/proto/ip/198.51.100.7",
		"/a",
	}
	for _, p := range valid {
		assert.NoError(t, ValidateBusPathSyntax(p), "expected valid path %q", p)
		assert.NoError(t, ValidateBusPathOrNone(p), "expected valid path-or-none %q", p)
	}

	assert.NoError(t, ValidateBusPathOrNone(BusKeywordNone))
	assert.Error(t, ValidateBusPathSyntax(BusKeywordNone))

	invalid := []string{
		"",
		"/",
		"bus/endpoint/a",
		"/bus/endpoint/",
		"/bus//endpoint",
		"/bus/end point",
		"/bus/\tendpoint",
		"/bus/\nendpoint",
		"/bus/\x7fendpoint",
		"/" + strings.Repeat("a", MaxBusPathLen),
	}
	for _, p := range invalid {
		assert.Error(t, ValidateBusPathSyntax(p), "expected invalid path %q", p)
		assert.Error(t, ValidateBusPathOrNone(p), "expected invalid path-or-none %q", p)
	}
}

// TestPathSegmentsAndCovers verifies segment splitting and exact boundary
// prefix matching.
//
// VALIDATES: PathCovers matches identical paths and exact segment prefixes
// without false matches across partial segment names.
// PREVENTS: /a matching /ab or /bus/endpoint/a matching /bus/endpoint/ab.
func TestPathSegmentsAndCovers(t *testing.T) {
	assert.Equal(t, []string{"bus", "endpoint", "collector"}, PathSegments("/bus/endpoint/collector"))
	assert.Nil(t, PathSegments(""))
	assert.Nil(t, PathSegments("/"))
	assert.Nil(t, PathSegments("relative"))
	assert.Nil(t, PathSegments("/a//b"))

	assert.Equal(t, "/bus/endpoint/collector", EndpointPath("collector"))

	assert.True(t, PathCovers("/a", "/a"))
	assert.True(t, PathCovers("/a", "/a/b"))
	assert.True(t, PathCovers("/a", "/a/b/c"))
	assert.True(t, PathCovers("/bus/endpoint/c", "/bus/endpoint/c"))

	assert.False(t, PathCovers("/a", "/ab"))
	assert.False(t, PathCovers("/bus/endpoint/a", "/bus/endpoint/ab"))
	assert.False(t, PathCovers("/a/b", "/a"))
	assert.False(t, PathCovers("/", "/a"))
	assert.False(t, PathCovers("/a/", "/a/b"))
	assert.False(t, PathCovers("", "/a"))
	require.False(t, PathCovers("a", "a/b"))
}
