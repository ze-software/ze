# 488 -- Looking Glass

## Context

Ze had no built-in looking glass. Network operators at IXPs need public, read-only visibility into BGP state (peers, routes, AS paths) via browser and API. Existing looking glasses (Alice-LG, Hyperglass) are separate applications that query daemons via birdwatcher API. Ze needed both the birdwatcher-compatible API (for Alice-LG integration) and its own HTMX web UI, running as a component on a separate port with no authentication.

## Decisions

- **Component, not plugin** (over placing in `bgp/plugins/`): the looking glass is infrastructure, not BGP behavior. The "delete the folder" test: deleting `bgp/plugins/` should not remove the looking glass.
- **Separate HTTP server** (over extending the web UI): different access model (public vs authenticated), different port (8443 vs 3443). Operators expose LG to the internet while keeping the web UI firewalled.
- **Server-side SVG** (over client-side JS graph library): zero external dependencies, perfect HTMX fit, sufficient for AS path DAGs (typically 3-10 nodes).
- **Birdwatcher snake_case** (over Ze's kebab-case): de facto standard for looking glass APIs. Alice-LG compatibility requires it. Documented as the only exception in `json-format.md`.
- **CommandDispatcher for data access** (over direct RIB imports): preserves plugin isolation. LG queries the engine the same way the web UI does.
- **Plain HTTP by default** (over mandatory TLS): looking glasses are typically behind reverse proxies. TLS is opt-in. When TLS is explicitly configured but cert loading fails, the server refuses to start (no silent downgrade).
- **Default port 8443** (LG). Web UI moved to 3443.

## Consequences

- Alice-LG can use Ze as a direct data source without any adapter.
- The LG URL namespace (`/lg/*`, `/api/looking-glass/*`) is reserved on the LG port.
- The `environment/looking-glass` YANG block, env vars `ze.looking-glass.*`, and `ExtractLGConfig` follow the same pattern as web/mcp/dns components.
- ASN name resolution wired via `DecorateASN` callback using Team Cymru DNS. Graph nodes and peer tables show organization names.
- Uses real htmx.min.js (v2.0.4) and SSE extension, synced from the same vendor directory as the web UI.

## Gotchas

- **YANG schema registration requires explicit loading** in `yang_schema.go` -- registering via `init()` + `all.go` blank import is necessary but not sufficient. The module must also be loaded by name in `YANGSchemaWithPlugins()`.
- **IPv6 peer addresses contain colons** -- `isValidPeerName` initially rejected them, blocking any IPv6 peer from the LG. Had to add `:` to the allowed charset.
- **`template.Option("missingkey=zero")`** is needed to avoid `<no value>` rendering for missing map keys in Go templates when using `{{index . "key"}}`.
- **`ListenAndServe` must close the ready channel on bind failure** -- without this, `WaitReady` blocks for the full timeout instead of returning immediately.
- **SSE connections need an explicit limit** -- without `maxSSEClients`, each connection holds a goroutine + polls the engine every 5s. 100 connections = 20 queries/second.
- **`go func()` in hub main.go triggers the goroutine-lifecycle hook** -- the hook matches `/hub/` in the path. Workaround: extract the goroutine into a named function (`serveLG`/`serveLGBlocking`).
- **Three deep review passes** found progressively subtle issues: pass 1 caught command injection + SVG injection + template bugs; pass 2 caught IPv6 validation gap + ready channel lifecycle + extractASPath default case; pass 3 confirmed clean security/logic/errors but identified test coverage gaps.

## Files

- `internal/component/lg/` -- 8 source files, 5 test files, YANG schema
- `cmd/ze/hub/main.go` -- LG startup integration, env vars, `startLGServer`
- `internal/component/bgp/config/loader.go` -- `ExtractLGConfig`
- `internal/component/config/yang_schema.go` -- ze-lg-conf module loading
- `internal/component/plugin/all/all.go` -- blank import for schema registration
- `docs/features.md` + `docs/features/*.md` -- restructured into index + 13 feature pages
- `docs/guide/looking-glass.md` -- user guide
- `docs/architecture/web-interface.md` -- LG architecture section
- `.claude/rules/json-format.md` -- snake_case exception
- `test/parse/lg-config.ci`, `test/parse/lg-disabled.ci` -- functional tests
