package rib

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// TestParseASNotation proves the bgp/as-notation leaf selects the notation.
// An absent leaf selects asplain, and a token nobody defined is refused rather
// than read as a default. The method runs one config subtree per case.
//
// VALIDATES: parseASNotation over the three tokens, absence, and a typo.
// PREVENTS: a misspelled notation being accepted and rendering asplain, which
// tells the operator their edit took effect when it did not.
func TestParseASNotation(t *testing.T) {
	accepted := []struct {
		config map[string]any
		want   asn.Notation
	}{
		{map[string]any{}, asn.NotationPlain},
		{map[string]any{"as-notation": "asplain"}, asn.NotationPlain},
		{map[string]any{"as-notation": "asdot"}, asn.NotationDot},
		{map[string]any{"as-notation": "asdot+"}, asn.NotationDotPlus},
	}
	restore := asn.Configured()
	t.Cleanup(func() { asn.Configure(restore.String()) }) //nolint:errcheck // a configured notation always parses

	for _, tc := range accepted {
		if err := configureASNotation(tc.config); err != nil {
			t.Fatalf("configureASNotation(%v): unexpected error %v", tc.config, err)
		}
		if got := asn.Configured(); got != tc.want {
			t.Errorf("configureASNotation(%v) recorded %v, want %v", tc.config, got, tc.want)
		}
	}

	for _, config := range []map[string]any{
		{"as-notation": "dot"},
		{"as-notation": ""},
		{"as-notation": 3},
	} {
		if err := configureASNotation(config); err == nil {
			t.Errorf("configureASNotation(%v) accepted a value that names no notation", config)
		}
	}
}

// TestASPathListRendersConfiguredNotation proves a route row's as-path is
// written in the notation the configuration selected. It also proves asplain
// writes the JSON numbers every release before the leaf existed wrote. The
// method marshals one path per notation.
//
// VALIDATES: asPathList.MarshalJSON reads configuredASNotation.
// PREVENTS: the notation reaching the config and not the row an operator
// reads, which is an unwired setting.
func TestASPathListRendersConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { asn.Configure(restore.String()) }) //nolint:errcheck // a configured notation always parses

	tests := []struct {
		notation asn.Notation
		path     asPathList
		want     string
	}{
		{asn.NotationPlain, asPathList{65546, 100}, `[65546,100]`},
		{asn.NotationDot, asPathList{65546, 100}, `["1.10","100"]`},
		{asn.NotationDotPlus, asPathList{65546, 100}, `["1.10","0.100"]`},
		{asn.NotationDot, asPathList{}, `[]`},
	}
	for _, tt := range tests {
		configure(t, tt.notation)
		got, err := json.Marshal(tt.path)
		if err != nil {
			t.Fatalf("marshal %v in %s: %v", tt.path, tt.notation, err)
		}
		if string(got) != tt.want {
			t.Errorf("marshal %v in %s = %s, want %s", tt.path, tt.notation, got, tt.want)
		}
	}
}

// TestRouteMapASPathHonorsNotation proves the notation reaches the row builder
// that `show bgp rib` answers with, not only the list type. The method builds
// an Adj-RIB-Out row and marshals the whole route map.
//
// VALIDATES: enrichRouteMapFromRoute wraps the AS path in asPathList.
// PREVENTS: a row that keeps writing decimal numbers while the leaf says asdot.
func TestRouteMapASPathHonorsNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { asn.Configure(restore.String()) }) //nolint:errcheck // a configured notation always parses

	route := &Route{ASPath: []uint32{65546, 100}}

	configure(t, asn.NotationPlain)
	plain := marshalRouteASPath(t, route)
	if plain != `[65546,100]` {
		t.Fatalf("asplain as-path = %s, want [65546,100]", plain)
	}

	configure(t, asn.NotationDot)
	dotted := marshalRouteASPath(t, route)
	if dotted != `["1.10","100"]` {
		t.Fatalf("asdot as-path = %s, want [\"1.10\",\"100\"]", dotted)
	}
}

// configure records one notation for the rest of a test.
func configure(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}

// marshalRouteASPath returns the JSON the as-path attribute value renders as
// for one Adj-RIB-Out route.
func marshalRouteASPath(t *testing.T, route *Route) string {
	t.Helper()
	routeMap := map[string]any{}
	enrichRouteMapFromRoute(routeMap, route)
	attr, ok := routeMap["as-path"].(map[string]any)
	if !ok {
		t.Fatalf("route map holds no as-path attribute: %v", routeMap)
	}
	encoded, err := json.Marshal(attr["value"])
	if err != nil {
		t.Fatalf("marshal as-path value: %v", err)
	}
	return string(encoded)
}
