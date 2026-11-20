// Design: docs/architecture/api/architecture.md -- API schema generation
// Related: types.go -- CommandMeta/ParamMeta used to build schemas

package api

import (
	"encoding/json"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The OpenAPI 3.1 and JSON Schema vocabulary this file writes. Each constant is
// the exact token the generated document carries.
const (
	schemaKeyContent     = "content"
	schemaKeyDescription = "description"
	schemaKeyProperties  = "properties"
	schemaKeyRequired    = "required"
	schemaKeySchema      = "schema"
	schemaKeyTitle       = "title"
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
// A parameter's summary is the property's `title` and its explanation the
// property's `description`, which is how JSON Schema names a short and a long
// text. Each key is written only when the YANG leaf declares that text.
func CommandSchema(cmd CommandMeta) map[string]any {
	properties := make(map[string]any, len(cmd.Params))
	var required []string

	for _, p := range cmd.Params {
		prop := map[string]any{
			schemaKeyType: jsonSchemaType(p.Type),
		}
		if p.Flag {
			prop[schemaKeyType] = "boolean"
		}
		if p.ShortHelp != "" {
			prop[schemaKeyTitle] = p.ShortHelp
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
//
// Every path it publishes is a route `rest.registerRoutes` serves: a client
// that follows the document never meets a 404. A command is run through the
// one execute route, so a command has no path of its own. It is published as
// a schema component instead, the request body that runs it on that route.
func OpenAPISchema(commands []CommandMeta) ([]byte, error) {
	schemas := make(map[string]any, len(commands)+1)
	for _, cmd := range commands {
		schemas[operationID(cmd.Name)] = commandRequestSchema(cmd)
	}
	schemas["ExecResult"] = map[string]any{
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
	}

	paths := make(map[string]any, 1)
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
								// The one description of the pipe list, which the MCP
								// tool schema publishes too.
								command.PipeListParameter: command.PipeListSchema(),
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
			"schemas": schemas,
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

// commandRequestSchema is the body of POST /api/v1/execute that runs one
// command: `command` fixed to the command's name, its `params`, and the
// optional pipe list every command takes.
//
// JSON Schema defines `title` as the short text and `description` as the long
// one. The two halves map one to one, and neither is derived from the other. A
// command with no explanation carries no description key. An empty one would
// render as a blank paragraph.
func commandRequestSchema(cmd CommandMeta) map[string]any {
	params := CommandSchema(cmd)
	required := []string{"command"}
	if _, ok := params[schemaKeyRequired]; ok {
		required = append(required, "params")
	}
	schema := map[string]any{
		schemaKeyType: schemaTypeObject,
		schemaKeyProperties: map[string]any{
			"command": map[string]any{
				schemaKeyType: schemaTypeString,
				"const":       cmd.Name,
			},
			"params":                  params,
			command.PipeListParameter: command.PipeListSchema(),
		},
		schemaKeyRequired: required,
	}
	if cmd.ShortHelp != "" {
		schema[schemaKeyTitle] = cmd.ShortHelp
	}
	if cmd.Description != "" {
		schema[schemaKeyDescription] = cmd.Description
	}
	return schema
}

// operationID converts a command name to the key of its schema component.
// "show bgp rib" -> "showBgpRib". A command word is lower case, so no key
// collides with a fixed component such as ExecResult.
func operationID(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}
	var b textbuf.Buffer
	b.Reset()
	b.Str(words[0])
	for _, w := range words[1:] {
		if w != "" {
			b.Str(strings.ToUpper(w[:1])).Str(w[1:])
		}
	}
	return b.String()
}
