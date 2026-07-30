// Recovered verbatim from the pre-Phase-1 elicit_test.go (spec-mcp2026-2-mrtr
// Required Reading, "Deleted code this phase must RECOVER, not reinvent").
//
// These eleven functions test the flat-primitive requestedSchema validator,
// which has nothing to do with the deleted session type. They were collateral
// in a type-driven deletion, not obsolete code. They are restored UNMODIFIED,
// which is the check that the validator itself came back unrewritten. An edit
// needed here would mean the validator was reinvented (Phase 1 R-4).

package mcp

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: AC-15 — a requestedSchema that is not an object at the root is rejected.
// PREVENTS: servers passing a bare primitive as the schema and the client seeing a malformed JSON-RPC request.
func TestElicit_SchemaRejectsNonObjectRoot(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
	}{
		{"missing type", map[string]any{"properties": map[string]any{}}},
		{"type=string at root", map[string]any{"type": "string"}},
		{"type=array at root", map[string]any{"type": "array"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateElicitSchema(tt.schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid, got %v", err)
			}
		})
	}
}

// VALIDATES: AC-15 — a nested object property is rejected.
// PREVENTS: the flat-schema rule silently permitting one level of nesting.
func TestElicit_SchemaRejectsNestedObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"profile": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	err := validateElicitSchema(schema)
	if !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "profile") {
		t.Errorf("error should name the offending path; got %v", err)
	}
}

// VALIDATES: AC-15 — an array-of-object property is rejected.
// PREVENTS: accepting an arrays-of-primitives gotcha that the spec also forbids.
func TestElicit_SchemaRejectsArray(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
	if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid, got %v", err)
	}
}

// VALIDATES: AC-15 — oneOf/allOf/anyOf at the property level are rejected.
// PREVENTS: JSON-Schema features the spec intentionally excludes.
func TestElicit_SchemaRejectsComposition(t *testing.T) {
	for _, kw := range []string{"oneOf", "allOf", "anyOf", "$ref", "not"} {
		t.Run(kw, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{kw: []any{map[string]any{"type": "string"}}},
				},
			}
			if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid for %s, got %v", kw, err)
			}
		})
	}
}

// VALIDATES: string primitives with every supported format pass the validator.
// PREVENTS: the validator being too strict and rejecting legitimate schemas.
func TestElicit_SchemaAllowsString(t *testing.T) {
	for _, format := range []string{"", "email", "uri", "date", "date-time"} {
		t.Run("format="+format, func(t *testing.T) {
			prop := map[string]any{
				"type":        "string",
				"title":       "Display",
				"description": "desc",
				"minLength":   float64(1),
				"maxLength":   float64(50),
			}
			if format != "" {
				prop["format"] = format
			}
			schema := map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": prop},
				"required":   []any{"name"},
			}
			if err := validateElicitSchema(schema); err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}

// VALIDATES: number / integer primitives with min/max pass the validator.
// PREVENTS: the validator confusing JSON number with JSON integer.
func TestElicit_SchemaAllowsNumber(t *testing.T) {
	for _, typ := range []string{"number", "integer"} {
		t.Run(typ, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"age": map[string]any{
						"type":    typ,
						"minimum": float64(0),
						"maximum": float64(120),
					},
				},
			}
			if err := validateElicitSchema(schema); err != nil {
				t.Fatalf("%s: %v", typ, err)
			}
		})
	}
}

// VALIDATES: boolean primitive with a default passes the validator.
// PREVENTS: rejecting default values, which the spec explicitly supports.
func TestElicit_SchemaAllowsBoolean(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enabled": map[string]any{
				"type":    "boolean",
				"default": false,
			},
		},
	}
	if err := validateElicitSchema(schema); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

// VALIDATES: enum with enumNames passes the validator.
// PREVENTS: rejecting the enum pattern the spec explicitly supports.
func TestElicit_SchemaAllowsEnum(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"choice": map[string]any{
				"type":      "string",
				"enum":      []any{"a", "b", "c"},
				"enumNames": []any{"Alpha", "Bravo", "Charlie"},
			},
		},
	}
	if err := validateElicitSchema(schema); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

// VALIDATES: unknown primitive type (e.g. "null") is rejected.
// PREVENTS: schema types the spec does not list slipping through.
func TestElicit_SchemaRejectsUnknownType(t *testing.T) {
	for _, typ := range []string{"null", "", "object", "array"} {
		t.Run(typ, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": typ},
				},
			}
			if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid for type=%q, got %v", typ, err)
			}
		})
	}
}

// VALIDATES: nil / empty schema is rejected — elicitation/create requires a schema.
// PREVENTS: a caller passing a nil map and the server sending an empty requestedSchema.
func TestElicit_SchemaRejectsEmpty(t *testing.T) {
	if err := validateElicitSchema(nil); !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid for nil, got %v", err)
	}
	if err := validateElicitSchema(map[string]any{"type": "object"}); !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid for missing properties, got %v", err)
	}
}

// VALIDATES: enum on a non-string-typed property is rejected (MCP spec
// illustrates enum only under type=string).
// PREVENTS: a caller writing {"type":"number","enum":[1,2]} expecting
// it to mean "pick one of these numbers" -- the spec shape is the
// string+enumNames form and the validator must catch the mistake.
func TestElicit_SchemaRejectsEnumOnNonString(t *testing.T) {
	for _, typ := range []string{"number", "integer", "boolean"} {
		t.Run(typ, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{
						"type": typ,
						"enum": []any{1, 2, 3},
					},
				},
			}
			if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid for enum on type=%q, got %v", typ, err)
			}
		})
	}
}
