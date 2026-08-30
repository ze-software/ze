// Design: docs/architecture/mcp/overview.md -- server/discover capability advertisement
// Related: streamable_tools.go -- the ok() helper that stamps resultType and serverInfo
// Related: streamable.go -- routes server/discover to this handler
// Related: caching.go -- supplies this result's ttlMs and cacheScope
// Related: apps.go -- defines the extension identifier advertised here

// server/discover for MCP 2026-07-28.
//
// With the initialize handshake gone, server/discover is the one call a client
// can make to learn what a server speaks. The client makes that call before it
// commits to anything else. The call is optional for the client, and it is
// mandatory to implement for the server.

package mcp

import (
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// serverVersion is the version this server reports as its own implementation
// version in io.modelcontextprotocol/serverInfo. It is NOT a protocol version:
// the protocol version is ProtocolVersion.
const serverVersion = "2.0.0"

// defaultServerName is the serverInfo name in command-registry mode. Provider
// mode reports the provider's own name instead.
const defaultServerName = "ze-mcp"

// serverDiscover answers server/discover.
//
// MCP 2026-07-28 basic/versioning Section "Protocol Version Negotiation":
// "Servers MUST implement server/discover. Clients MAY call it before sending
// any other requests to learn the server's supported versions up front."
//
// The DiscoverResult carries supportedVersions, capabilities and the optional
// instructions string. serverInfo rides in the result's _meta rather than at
// the top of result. ok() stamps it there, together with resultType.
//
// MCP 2026-07-28 server/discover Section "Data Types":
// "_meta['io.modelcontextprotocol/serverInfo']: Name and version of the server
// software. Servers SHOULD include this field."
//
// ttlMs and cacheScope are the CacheableResult fields DiscoverResult inherits.
// runMethod stamps them from cacheTTLByMethod (caching.go) rather than this
// handler. The four cacheable surfaces therefore cannot drift apart.
// server/discover is registry-derived, so it carries the same 60 s freshness
// as tools/list.
func (s *Streamable) serverDiscover(req *request) *response {
	return s.ok(req.ID, map[string]any{
		"supportedVersions": slices.Clone(supportedProtocolVersions),
		"capabilities":      serverCapabilities(),
		"instructions":      s.instructions(),
	})
}

// serverCapabilities reports what this server offers, in the specification's
// ServerCapabilities shape.
//
// tools and resources are served in both command-registry and Provider mode
// (the resource set is walked from the embedded UI filesystem at construction,
// independent of the tool surface).
//
// `extensions` names io.modelcontextprotocol/ui, the MCP Apps extension. Tool
// descriptors carry _meta.ui.resourceUri, and resources/read serves the ui://
// assets that field points at. A surface a client can use therefore backs the
// claim. The extension defines `mimeTypes` as something the CLIENT declares to
// constrain what it can render, so this server declares no settings of its own.
//
// `extensions` also names io.modelcontextprotocol/tasks, the Tasks extension.
// This server serves tasks/get, tasks/update and tasks/cancel. It also returns
// a CreateTaskResult with resultType "task" for every command its YANG
// annotates `ze:task-support required`. A surface a client can use therefore
// backs this claim too.
//
// This advertisement is not optional bookkeeping.
//
// MCP 2026-07-28 basic/index Section "ResultType": "The set of supported
// ResultType values MUST be created from the set defined in the core protocol
// and include any additional values of supported extensions that are
// advertised via capabilities."
//
// A client is entitled to reject a `resultType` it does not recognize. And a
// client recognizes "task" only because this map says the extension is
// supported. An earlier revision of this function advertised an empty
// extension set while the server already served tasks/*. That revision claimed
// non-support and served at the same time.
//
// Both settings objects are empty because MCP 2026-07-28 basic/versioning
// Section "Extension Negotiation" says "an empty object indicates support with
// no additional settings", and neither extension defines a server-side settings
// member. An invented member would assert a schema the extension does not
// specify (ai/rules/evidence.md).
func serverCapabilities() map[string]any {
	return map[string]any{
		"tools":     map[string]any{},
		"resources": map[string]any{},
		"extensions": map[string]any{
			extensionUI:    map[string]any{},
			extensionTasks: map[string]any{},
		},
	}
}

// instructions is the natural-language guidance server/discover returns.
//
// MCP 2026-07-28 server/discover Section "Data Types": "instructions: Optional
// natural-language guidance for LLMs on how to use this server effectively."
// It names the entry-point tool rather than restating tool descriptions, which
// the specification asks servers not to duplicate.
func (s *Streamable) instructions() string {
	if s.cfg.Provider != nil {
		var tb textbuf.Buffer
		return tb.Str(s.cfg.Provider.ServerName()).
			Str(" MCP server. Call tools/list to discover the tools it offers.").String()
	}
	return "Ze network OS. Call the ze_reference tool first to discover this instance's commands, plugins and address families, then ze_execute to run a ze CLI command."
}

// serverInfo is the Implementation object this server reports in every result's
// _meta.
//
// MCP 2026-07-28 server/discover Section "Data Types" says serverInfo "is
// self-reported by the server and is not verified by the protocol", and is
// "intended for display, logging, and debugging" only. Nothing here therefore
// reaches an authorization decision.
func (s *Streamable) serverInfo() map[string]any {
	name := defaultServerName
	if s.cfg.Provider != nil {
		name = s.cfg.Provider.ServerName()
	}
	return map[string]any{
		toolKeyName: name,
		"version":   serverVersion,
	}
}
