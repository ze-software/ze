// Design: docs/architecture/api/architecture.md -- API schema generation
// Related: types.go -- CommandMeta/ParamMeta used to build schemas

package api

import (
	"encoding/json"
	"strings"
)

// The OpenAPI 3.1 and JSON Schema vocabulary this file writes. Each constant is
// the exact token the generated document carries.
const (
	schemaKeyContent     = "content"
	schemaKeyDescription = "description"
	schemaKeyProperties  = "properties"
	schemaKeyRequired    = "required"
	schemaKeySchema      = "schema"
	schemaKeyType        = "type"

	schemaTypeInteger = "integer"
	schemaTypeObject  = "object"
	schemaTypeString  = "string"

	mediaTypeJSON = "application/json"
)

// yangTypeToJSON maps YANG type names to JSON Schema types.
var yangTypeToJSON = map[string]string{
	"uint8":   schemaTypeInteger,
	"uint16":  schemaTypeInteger,
	"uint32":  schemaTypeInteger,
	"uint64":  schemaTypeInteger,
	"int8":    schemaTypeInteger,
	"int16":   schemaTypeInteger,
	"int32":   schemaTypeInteger,
	"int64":   schemaTypeInteger,
	"boolean": "boolean",
}

// jsonSchemaType converts a YANG type name to a JSON Schema type.
func jsonSchemaType(yangType string) string {
	if t, ok := yangTypeToJSON[yangType]; ok {
		return t
	}
	return schemaTypeString
}

// CommandSchema generates a JSON Schema for a single command's parameters.
func CommandSchema(cmd CommandMeta) map[string]any {
	properties := make(map[string]any, len(cmd.Params))
	var required []string

	for _, p := range cmd.Params {
		prop := map[string]any{
			schemaKeyType: jsonSchemaType(p.Type),
		}
		if p.Description != "" {
			prop[schemaKeyDescription] = p.Description
		}
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	schema := map[string]any{
		schemaKeyType:       schemaTypeObject,
		schemaKeyProperties: properties,
	}
	if len(required) > 0 {
		schema[schemaKeyRequired] = required
	}
	return schema
}

// OpenAPISchema generates an OpenAPI 3.1 specification from the command list.
// The schema is built once at startup and cached by the caller.
func OpenAPISchema(commands []CommandMeta) ([]byte, error) {
	paths := make(map[string]any, len(commands))

	for _, cmd := range commands {
		pathKey := "/api/v1/execute/" + strings.ReplaceAll(cmd.Name, " ", "/")

		operation := map[string]any{
			"summary":     cmd.Description,
			"operationId": operationID(cmd.Name),
			"tags":        []string{commandTag(cmd.Name)},
			"responses": map[string]any{
				"200": map[string]any{
					schemaKeyDescription: "Command result",
					schemaKeyContent: map[string]any{
						mediaTypeJSON: map[string]any{
							schemaKeySchema: map[string]any{
								"$ref": "#/components/schemas/ExecResult",
							},
						},
					},
				},
			},
		}

		if len(cmd.Params) > 0 {
			operation["requestBody"] = map[string]any{
				schemaKeyContent: map[string]any{
					mediaTypeJSON: map[string]any{
						schemaKeySchema: CommandSchema(cmd),
					},
				},
			}
		}

		method := "get"
		if !cmd.ReadOnly {
			method = "post"
		}

		paths[pathKey] = map[string]any{
			method: operation,
		}
	}

	// Also add the generic execute endpoint.
	paths["/api/v1/execute"] = map[string]any{
		"post": map[string]any{
			"summary":     "Execute any command",
			"operationId": "execute",
			"tags":        []string{"execute"},
			"requestBody": map[string]any{
				schemaKeyRequired: true,
				schemaKeyContent: map[string]any{
					mediaTypeJSON: map[string]any{
						schemaKeySchema: map[string]any{
							schemaKeyType: schemaTypeObject,
							schemaKeyProperties: map[string]any{
								"command": map[string]any{
									schemaKeyType:        schemaTypeString,
									schemaKeyDescription: "Command to execute",
								},
								"params": map[string]any{
									schemaKeyType:        schemaTypeObject,
									schemaKeyDescription: "Command parameters",
								},
							},
							schemaKeyRequired: []string{"command"},
						},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					schemaKeyDescription: "Command result",
					schemaKeyContent: map[string]any{
						mediaTypeJSON: map[string]any{
							schemaKeySchema: map[string]any{
								"$ref": "#/components/schemas/ExecResult",
							},
						},
					},
				},
			},
		},
	}

	spec := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Ze API",
			"version": "1.0.0",
		},
		"paths": paths,
		"components": map[string]any{
			"schemas": map[string]any{
				"ExecResult": map[string]any{
					schemaKeyType: schemaTypeObject,
					schemaKeyProperties: map[string]any{
						"status": map[string]any{
							schemaKeyType: schemaTypeString,
							"enum":        []string{"done", "error"},
						},
						"data": map[string]any{
							schemaKeyDescription: "Response payload",
						},
						"error": map[string]any{
							schemaKeyType:        schemaTypeString,
							schemaKeyDescription: "Error message",
						},
					},
					schemaKeyRequired: []string{"status"},
				},
			},
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					schemaKeyType: "http",
					"scheme":      "bearer",
				},
			},
		},
		"security": []map[string]any{
			{"bearerAuth": []string{}},
		},
	}

	return json.MarshalIndent(spec, "", "  ")
}

// operationID converts a command name to an operationId.
// "show bgp rib" -> "showBgpRib".
func operationID(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(words[0])
	for _, w := range words[1:] {
		if w != "" {
			b.WriteString(strings.ToUpper(w[:1]))
			b.WriteString(w[1:])
		}
	}
	return b.String()
}

// commandTag extracts the first word as the OpenAPI tag.
// "show bgp rib" -> "show".
func commandTag(name string) string {
	word, _, _ := strings.Cut(name, " ")
	return word
}
