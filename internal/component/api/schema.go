// Design: docs/architecture/api/architecture.md -- API schema generation
// Related: types.go -- CommandMeta/ParamMeta used to build schemas

package api

import (
	"encoding/json"
	"strings"
)

// yangTypeToJSON maps YANG type names to JSON Schema types.
var yangTypeToJSON = map[string]string{
	"uint8":   "integer",
	"uint16":  "integer",
	"uint32":  "integer",
	"uint64":  "integer",
	"int8":    "integer",
	"int16":   "integer",
	"int32":   "integer",
	"int64":   "integer",
	"boolean": "boolean",
}

// jsonSchemaType converts a YANG type name to a JSON Schema type.
func jsonSchemaType(yangType string) string {
	if t, ok := yangTypeToJSON[yangType]; ok {
		return t
	}
	return "string"
}

// CommandSchema generates a JSON Schema for a single command's parameters.
func CommandSchema(cmd CommandMeta) map[string]any {
	properties := make(map[string]any, len(cmd.Params))
	var required []string

	for _, p := range cmd.Params {
		prop := map[string]any{
			"type": jsonSchemaType(p.Type),
		}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
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
					"description": "Command result",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/ExecResult",
							},
						},
					},
				},
			},
		}

		if len(cmd.Params) > 0 {
			operation["requestBody"] = map[string]any{
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": CommandSchema(cmd),
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
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"command": map[string]any{
									"type":        "string",
									"description": "Command to execute",
								},
								"params": map[string]any{
									"type":        "object",
									"description": "Command parameters",
								},
							},
							"required": []string{"command"},
						},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Command result",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
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
					"type": "object",
					"properties": map[string]any{
						"status": map[string]any{
							"type": "string",
							"enum": []string{"done", "error"},
						},
						"data": map[string]any{
							"description": "Response payload",
						},
						"error": map[string]any{
							"type":        "string",
							"description": "Error message",
						},
					},
					"required": []string{"status"},
				},
			},
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":   "http",
					"scheme": "bearer",
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
// "bgp rib routes" -> "bgpRibRoutes".
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
// "bgp rib routes" -> "bgp".
func commandTag(name string) string {
	word, _, _ := strings.Cut(name, " ")
	return word
}
