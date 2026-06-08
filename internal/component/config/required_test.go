package config

import (
	"testing"
)

// VALIDATES: AC-1 — ze:required path-form on a list entry missing a descendant reports violation.
// VALIDATES: AC-2 — enforcement works for any list, not just BGP.
// PREVENTS: regression where ze:required is only checked for bgp/peer.
func TestCheckRequiredGenericPerAnchor(t *testing.T) {
	schema := NewSchema()
	schema.Define("comp", Container(
		Field("mylist", listWithRequired(
			[][]string{{"sub", "leaf"}},
			Field("sub", Container(
				Field("leaf", Leaf(TypeString)),
			)),
		)),
	))

	tests := []struct {
		name       string
		data       map[string]any
		wantCount  int
		wantAnchor string
		wantField  string
		wantEntry  string
	}{
		{
			name: "missing required descendant",
			data: map[string]any{
				"comp": map[string]any{
					"mylist": map[string]any{
						"entry1": map[string]any{},
					},
				},
			},
			wantCount:  1,
			wantAnchor: "comp/mylist",
			wantField:  "sub/leaf",
			wantEntry:  "entry1",
		},
		{
			name: "present required descendant",
			data: map[string]any{
				"comp": map[string]any{
					"mylist": map[string]any{
						"entry1": map[string]any{
							"sub": map[string]any{
								"leaf": "value",
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "multiple entries one missing",
			data: map[string]any{
				"comp": map[string]any{
					"mylist": map[string]any{
						"ok": map[string]any{
							"sub": map[string]any{"leaf": "v"},
						},
						"bad": map[string]any{},
					},
				},
			},
			wantCount:  1,
			wantAnchor: "comp/mylist",
			wantField:  "sub/leaf",
			wantEntry:  "bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := CheckRequired(schema, tt.data)
			if len(violations) != tt.wantCount {
				t.Fatalf("got %d violations, want %d: %+v", len(violations), tt.wantCount, violations)
			}
			if tt.wantCount > 0 {
				v := violations[0]
				if v.AnchorPath != tt.wantAnchor {
					t.Errorf("anchor = %q, want %q", v.AnchorPath, tt.wantAnchor)
				}
				if v.FieldPath != tt.wantField {
					t.Errorf("field = %q, want %q", v.FieldPath, tt.wantField)
				}
				if v.EntryKey != tt.wantEntry {
					t.Errorf("entry = %q, want %q", v.EntryKey, tt.wantEntry)
				}
			}
		})
	}
}

// VALIDATES: AC-3 — anchor node absent means no requirement reported.
// PREVENTS: false positives when optional parent containers are not configured.
func TestCheckRequiredAnchorAbsentSkips(t *testing.T) {
	schema := NewSchema()
	schema.Define("comp", Container(
		Field("mylist", listWithRequired(
			[][]string{{"sub", "leaf"}},
			Field("sub", Container(
				Field("leaf", Leaf(TypeString)),
			)),
		)),
	))

	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "empty data",
			data: map[string]any{},
		},
		{
			name: "parent container absent",
			data: map[string]any{
				"other": map[string]any{},
			},
		},
		{
			name: "list absent",
			data: map[string]any{
				"comp": map[string]any{},
			},
		},
		{
			name: "list present but empty",
			data: map[string]any{
				"comp": map[string]any{
					"mylist": map[string]any{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := CheckRequired(schema, tt.data)
			if len(violations) != 0 {
				t.Fatalf("got %d violations, want 0: %+v", len(violations), violations)
			}
		})
	}
}

// VALIDATES: AC-7 — BGP peer with all required fields reports no violations.
// PREVENTS: regression on existing valid BGP configs.
func TestCheckRequiredBGPShape(t *testing.T) {
	schema := NewSchema()
	schema.Define("bgp", Container(
		Field("peer", listWithRequired(
			[][]string{
				{"connection", "remote", "ip"},
				{"session", "asn", "local"},
				{"session", "asn", "remote"},
			},
			Field("connection", Container(
				Field("remote", Container(
					Field("ip", Leaf(TypeString)),
				)),
			)),
			Field("session", Container(
				Field("asn", Container(
					Field("local", Leaf(TypeString)),
					Field("remote", Leaf(TypeString)),
				)),
			)),
		)),
	))

	tests := []struct {
		name      string
		data      map[string]any
		wantCount int
	}{
		{
			name: "all present",
			data: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"london": map[string]any{
							"connection": map[string]any{
								"remote": map[string]any{"ip": "1.2.3.4"},
							},
							"session": map[string]any{
								"asn": map[string]any{"local": "65000", "remote": "65001"},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "missing remote ip",
			data: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"london": map[string]any{
							"session": map[string]any{
								"asn": map[string]any{"local": "65000", "remote": "65001"},
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "missing all three",
			data: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"london": map[string]any{},
					},
				},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := CheckRequired(schema, tt.data)
			if len(violations) != tt.wantCount {
				t.Fatalf("got %d violations, want %d: %+v", len(violations), tt.wantCount, violations)
			}
		})
	}
}

// VALIDATES: CheckRequired skips empty-path entries gracefully.
// PREVENTS: crash if a bare ze:required; slips past schema build rejection.
func TestCheckRequiredSkipsEmptyPath(t *testing.T) {
	schema := NewSchema()
	schema.Define("test", Container(
		Field("mylist", listWithRequired(
			[][]string{{""}},
			Field("leaf", Leaf(TypeString)),
		)),
	))

	// A bare required (empty-string path) should be treated as if no required
	// fields exist — the ListNode.Required should not contain an entry with
	// empty first element. This is tested at the YANG parse level (yang_schema.go).
	// Here we verify that CheckRequired handles it gracefully if one slips through.
	data := map[string]any{
		"test": map[string]any{
			"mylist": map[string]any{
				"e1": map[string]any{},
			},
		},
	}
	violations := CheckRequired(schema, data)
	// Empty-path required entries should be skipped, not crash.
	for _, v := range violations {
		if v.FieldPath == "" {
			t.Errorf("empty FieldPath should not produce a violation: %+v", v)
		}
	}
}

// VALIDATES: nested lists — required on a list inside another list.
// PREVENTS: walker stopping at the first list level.
func TestCheckRequiredNestedList(t *testing.T) {
	schema := NewSchema()
	schema.Define("ipsec", Container(
		Field("esp-group", List(TypeString,
			Field("proposal", listWithRequired(
				[][]string{{"encryption"}},
				Field("encryption", Leaf(TypeString)),
				Field("hash", Leaf(TypeString)),
			)),
		)),
	))

	data := map[string]any{
		"ipsec": map[string]any{
			"esp-group": map[string]any{
				"mygroup": map[string]any{
					"proposal": map[string]any{
						"1": map[string]any{},
					},
				},
			},
		},
	}

	violations := CheckRequired(schema, data)
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(violations), violations)
	}
	v := violations[0]
	if v.AnchorPath != "ipsec/esp-group/proposal" {
		t.Errorf("anchor = %q, want %q", v.AnchorPath, "ipsec/esp-group/proposal")
	}
	if v.FieldPath != "encryption" {
		t.Errorf("field = %q, want %q", v.FieldPath, "encryption")
	}
	if v.EntryKey != "1" {
		t.Errorf("entry = %q, want %q", v.EntryKey, "1")
	}
}

// listWithRequired creates a ListNode with Required fields pre-set.
func listWithRequired(required [][]string, fields ...FieldDef) *ListNode {
	l := List(TypeString, fields...)
	l.Required = required
	return l
}
