// Design: docs/architecture/mcp/overview.md -- MCP Apps UI extension negotiation
// Related: tools.go -- buildToolDef emits the _meta.ui object this gate admits or removes
// Related: meta.go -- parseClientCapabilities resolves the gate on every request
// Related: discover.go -- advertises the same extension identifier in server capabilities

// MCP Apps as the io.modelcontextprotocol/ui extension (SEP-1865).
//
// Ze has served MCP Apps since the 2026-01-26 draft: a tool descriptor carries
// _meta.ui.resourceUri pointing at a ui:// asset, which the host fetches
// through resources/read and renders in a sandboxed iframe. In 2026-07-28 that
// arrangement became a first-class extension, negotiated through the
// `extensions` capability map rather than assumed.
//
// The payload did NOT change. The MCP Apps overview names "_meta.ui.resourceUri
// field pointing to a ui:// resource", says apps "can also load external
// scripts and resources from origins specified in _meta.ui.csp", and says the
// "_meta.ui object can include permissions to request additional capabilities
// (e.g., microphone, camera) and csp to control what external origins the app
// can load resources from". Those are field-for-field the three keys
// buildToolDef already emits, and the extension's own specification directory
// is still 2026-01-26 -- the draft Ze built against. So the bundle, the ui://
// scheme and the YANG ze:ui-resource walker are all untouched by this file.
// Only the negotiation around them is new.

package mcp

import "strings"

// extensionUI is the specification-registered identifier for MCP Apps.
//
// MCP 2026-07-28 basic/versioning Section "Extension Negotiation": "Extension
// identifiers MUST follow the _meta key naming rules, with a mandatory prefix",
// and names this exact string in its worked example, a client advertising
// {"io.modelcontextprotocol/ui": {"mimeTypes": ["text/html;profile=mcp-app"]}}.
//
// One constant shared by the server's advertisement (discover.go) and the
// client-side gate below, so the two can never name different extensions.
const extensionUI = "io.modelcontextprotocol/ui"

// uiSettingsMIMETypes is the one settings-object member this server reads from
// the client's extension declaration.
const uiSettingsMIMETypes = "mimeTypes"

// uiMediaTypeHTML is the base media type of every bundle Ze serves. The UI
// filesystem holds HTML entry points whose type is sniffed from the file
// extension (resources.go), so this is the whole of what Ze can offer a host.
const uiMediaTypeHTML = "text/html"

// metaKeyUI is the `ui` member of a tool descriptor's `_meta` object.
const metaKeyUI = "ui"

// clientSupportsUIApps resolves whether this request's client declared the MCP
// Apps extension in a form compatible with the bundles Ze serves.
//
// The five cases, and why each answers as it does:
//
//	| Declared io.modelcontextprotocol/ui settings   | Emit _meta.ui |
//	|------------------------------------------------|---------------|
//	| extension absent from `extensions`              | no            |
//	| {} (support, no settings)                       | yes           |
//	| present, no `mimeTypes` key                     | yes           |
//	| `mimeTypes` holds a text/html base type         | yes           |
//	| `mimeTypes` holds no text/html base type        | no            |
//
// An empty or absent settings object means yes because MCP 2026-07-28
// basic/versioning Section "Extension Negotiation" says "Each extension
// specifies the schema of its settings object; an empty object indicates
// support with no additional settings" -- a client that declared the extension
// and constrained nothing accepts what the extension offers.
//
// Matching is on the BASE media type with parameters stripped, so a client
// declaring bare "text/html" is served, as is one declaring
// "text/html;profile=mcp-app". `;profile=mcp-app` is a media-type parameter, so
// bare text/html is the superset; exact string equality would refuse a host
// that can render Ze's bundle perfectly well.
//
// Anything malformed -- a `mimeTypes` that is not an array, entries that are
// not strings, an empty array -- answers no. That is the fail-closed direction
// here: omitting _meta.ui still serves the client a valid tool.
func clientSupportsUIApps(caps map[string]any) bool {
	extensions, present := caps[capabilityExtensionsKey].(map[string]any)
	if !present {
		return false
	}
	settings, declared := extensions[extensionUI].(map[string]any)
	if !declared {
		return false
	}
	raw, constrained := settings[uiSettingsMIMETypes]
	if !constrained {
		return true
	}
	declaredTypes, isList := raw.([]any)
	if !isList {
		return false
	}
	for _, entry := range declaredTypes {
		mediaType, isString := entry.(string)
		if !isString {
			continue
		}
		if baseMediaType(mediaType) == uiMediaTypeHTML {
			return true
		}
	}
	return false
}

// baseMediaType strips any media-type parameters and normalizes case, turning
// "text/html;profile=mcp-app" into "text/html". RFC 9110 makes the type and
// subtype case-insensitive, and everything after the first semicolon is a
// parameter rather than part of the type.
func baseMediaType(mediaType string) string {
	base, _, _ := strings.Cut(mediaType, ";")
	return strings.ToLower(strings.TrimSpace(base))
}

// gateUIMeta removes _meta.ui from every tool descriptor when the client did
// not declare the MCP Apps extension compatibly.
//
// MCP 2026-07-28 basic/versioning Section "Extension Negotiation": "If one
// party supports an extension but the other does not, the supporting party
// MUST either revert to core protocol behavior or reject the request with an
// appropriate error." Ze reverts. A tool descriptor without _meta.ui is a
// valid core descriptor, so omission IS the revert branch, and the tool stays
// listed and callable. Rejecting a whole tools/list because the host cannot
// render HTML panels would break every non-Apps client for no benefit.
//
// Applied here, over the ASSEMBLED list, rather than inside buildToolDef, so
// one gate covers descriptors from both origins: the ones Ze generates from the
// command registry and the ones a ToolProvider hands over. A provider owns its
// descriptor maps and may return the same maps on every call, so nothing is
// mutated in place: the slice and any descriptor that actually loses a key are
// copied, and when the gate is open or no descriptor carries _meta.ui the
// input is returned untouched and nothing is allocated.
func gateUIMeta(tools []map[string]any, supportsUI bool) []map[string]any {
	if supportsUI {
		return tools
	}
	out := tools
	copied := false
	for i, tool := range tools {
		stripped, changed := withoutUIMeta(tool)
		if !changed {
			continue
		}
		if !copied {
			out = make([]map[string]any, len(tools))
			copy(out, tools)
			copied = true
		}
		out[i] = stripped
	}
	return out
}

// withoutUIMeta returns a copy of a tool descriptor with the `ui` member
// removed from its `_meta` object, reporting whether anything was removed. When
// `_meta` holds nothing else afterwards it is dropped entirely rather than left
// as an empty object, so an ungated descriptor is byte-identical to one that
// never had a UI annotation.
func withoutUIMeta(tool map[string]any) (map[string]any, bool) {
	meta, hasMeta := tool[metaKey].(map[string]any)
	if !hasMeta {
		return tool, false
	}
	if _, hasUI := meta[metaKeyUI]; !hasUI {
		return tool, false
	}
	stripped := make(map[string]any, len(tool))
	for key, value := range tool {
		if key != metaKey {
			stripped[key] = value
		}
	}
	if len(meta) > 1 {
		remaining := make(map[string]any, len(meta)-1)
		for key, value := range meta {
			if key != metaKeyUI {
				remaining[key] = value
			}
		}
		stripped[metaKey] = remaining
	}
	return stripped, true
}
