# 186 — Hostname Plugin

## Objective
Extract FQDN/hostname capability (code 73) from core BGP code into a self-contained plugin with its own YANG schema, using JSON config delivery.

## Decisions
- YANG embedded as a string constant in `hostname.go`, not a separate file — avoids the `embed.go` boilerplate for a single small schema.
- Config delivered as `config json bgp {...}` (whole peer tree) at Stage 2, not pattern-matched field-by-field. Plugin extracts `capability.hostname.{host,domain}` itself.
- No auto-injection: the plugin ONLY activates when `--plugin ze.hostname` is passed. Unknown fields in config fail validation without the plugin.
- In-process execution (`ze.hostname`) uses a goroutine + socket pair, not a subprocess.
- FQDN struct kept in core `capability.go` for parsing received peer OPENs — only encoding/advertising moves to the plugin.

## Patterns
- Plugin YANG augments the core schema (`/ze-bgp:bgp/ze-bgp:peer/ze-bgp:capability`) — core parser accepts new fields; plugin interprets them.
- Legacy syntax (`peer X { host-name foo; }`) supported via augment at peer level; normalized to same wire encoding.
- Decode mode: `ze bgp decode --plugin hostname --open <hex>` dispatched via `--plugin` flag in decode.go, not a separate RPC.

## Gotchas
- Config tree is delivered wrapped as `{"bgp":{...}}`; plugin must unwrap before accessing peer fields.
- Capability decode belongs in plugins, not core `decode.go` — keep the decode dispatch extensible.

## Files
- `internal/component/plugin/hostname/hostname.go` — plugin implementation + embedded YANG
- `internal/component/plugin/inprocess.go` — registers in-process runner
- `cmd/ze/bgp/plugin_hostname.go` — CLI entry with `--yang` flag
