# Learned: port-defaults

## What worked

- **PortDefault/AddrPortDefault helpers** in the env package are the right abstraction level: they resolve the env override and format the description in one call, keeping flag definitions clean. Two variants (int vs string) cover all ze-chaos flag types.

- **Reusing ValidateListenerConflicts** from config/listener.go for ze-chaos was straightforward because it takes []ListenerEndpoint (decoupled from config tree). Hand-building the endpoint slice from resolved flags works well.

- **env.Get normalization** eliminates the need for double-key loops (dot and underscore variants). The signal/main.go fix collapsed 8 lines per function into 3.

## What was discovered

- **MCP YANG was missing its port default.** Every other listener service (web 3443, LG 8443, SSH 2222) declared both ip and port defaults in YANG. MCP only declared ip; port 8080 was a Go runtime fallback in hub/main.go:243. Adding `refine port { default 8080; }` made all services consistent. The hub/main.go fallback is now redundant defense-in-depth.

- **ze-chaos has two kinds of port flags:** integer ports (--port, --ssh, --web-ui, --lg) and addr:port strings (--web, --pprof, --metrics). This required two different helpers. The spec caught this during research (Wrong Assumptions table).

- **Range base ports (--port, --listen-base) can't be conflict-checked** against single-port listeners without knowing peer count at check time. This is a known gap documented in the spec.

## Patterns to reuse

- **env.PortDefault(key, fallback, desc)** pattern for any future CLI flag that should be env-overridable. Returns (resolvedValue, formattedDescription) for direct use with flag.IntVar/flag.StringVar.

- **Hand-built ListenerEndpoint slices** for conflict detection in tools that don't load YANG/config trees. ValidateListenerConflicts is tool-agnostic.

## Mistakes avoided

- Did not create a new conflict detection system (existing one has 12 tests).
- Did not add env vars for --ze-pprof (debugging flag, rarely used) or --port/--listen-base (range semantics, not single endpoints).
- Did not set a Default for ze.mcp.listen until YANG was fixed first -- the Default must be YANG-sourced, not a lie about a Go constant.
