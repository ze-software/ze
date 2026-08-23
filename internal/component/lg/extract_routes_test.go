package lg

import "testing"

// TestExtractRoutesNormalizesFlatRows pins the shape `show bgp rib` answers.
//
// It answers ONE envelope under `routes`, one row per route, each row carrying
// `peer` and `direction` as fields (owner ruling, 2026-08-23). extractRoutes
// already had a `routes` branch, written for a different producer that answered
// an already-flat list, and it returned those rows UNTOUCHED. So the day
// `show bgp rib` started answering `routes`, that branch silently became the
// path every RIB answer took: attributes stayed wrapped and no row carried
// `peer-address`, so the looking-glass graph drew nothing and answered
// "No routes found".
//
// Nothing errored. An empty graph reads like a true answer about an empty RIB,
// which is why only a functional test caught it.
//
// VALIDATES: a flat row reaches the graph with the two fields it reads.
// PREVENTS: an added payload key colliding with an existing branch that handles
// it wrongly and quietly.
func TestExtractRoutesNormalizesFlatRows(t *testing.T) {
	got := extractRoutes(map[string]any{
		"routes": []any{
			map[string]any{
				"peer":      "192.0.2.1",
				"direction": "received",
				"prefix":    "10.0.0.0/24",
				"as-path":   map[string]any{"value": "65001"},
			},
		},
	})

	if len(got) != 1 {
		t.Fatalf("extractRoutes answered %d rows, want 1", len(got))
	}
	row, ok := got[0].(map[string]any)
	if !ok {
		t.Fatalf("row is %T, want a record", got[0])
	}
	if row["peer-address"] != "192.0.2.1" {
		t.Errorf("peer-address = %v; a flat row names its peer in `peer` and the "+
			"graph reads `peer-address`", row["peer-address"])
	}
	if wrapped, isMap := row["as-path"].(map[string]any); isMap {
		t.Errorf("the attribute was not unwrapped, still %v", wrapped)
	}
}

// TestExtractRoutesStillHandlesTheGroupedShape keeps the other producers
// working. The grouped shape is not dead: only `show bgp rib` left it.
func TestExtractRoutesStillHandlesTheGroupedShape(t *testing.T) {
	got := extractRoutes(map[string]any{
		"adj-rib-in": map[string]any{
			"198.51.100.7": []any{
				map[string]any{"prefix": "10.0.0.0/24", "as-path": map[string]any{"value": "65002"}},
			},
		},
	})

	if len(got) != 1 {
		t.Fatalf("extractRoutes answered %d rows, want 1", len(got))
	}
	row, ok := got[0].(map[string]any)
	if !ok {
		t.Fatalf("row is %T, want a record", got[0])
	}
	if row["peer-address"] != "198.51.100.7" {
		t.Errorf("peer-address = %v, want the key the row was grouped under", row["peer-address"])
	}
	if wrapped, isMap := row["as-path"].(map[string]any); isMap {
		t.Errorf("the attribute was not unwrapped, still %v", wrapped)
	}
}
