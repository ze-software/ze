# 506 -- Resolution Component

## Context

DNS, Team Cymru, PeeringDB, and IRR resolution services were scattered across
`internal/component/dns/`, `internal/component/bgp/peeringdb/`,
`internal/component/bgp/irr/`, and `internal/component/web/decorator_asn.go`.
Hub startup created two independent DNS resolvers (web UI and looking glass) and
PeeringDB created a new client per invocation. This made resolution code hard to
find and prevented sharing DNS instances.

## Decisions

- Consolidated all resolvers under `internal/component/resolve/` with typed
  sub-packages (dns, cymru, peeringdb, irr, cache) over keeping them scattered
  across bgp/ and component/ because resolution is a cross-cutting concern used
  by web, LG, and CLI commands.
- Created a generic TTL cache (`resolve/cache/`) shared by Cymru, PeeringDB,
  and IRR over per-resolver caching because all three have the same 1h TTL
  pattern. DNS keeps its own TTL-from-response LRU cache.
- Cymru extracted from decorator_asn.go into its own package over keeping it
  embedded in the web decorator because the LG graph also needs ASN resolution.
- Hub creates one `Resolvers` struct with a single shared DNS instance over
  the previous pattern of two independent DNS resolvers.
- `NewASNNameDecoratorFromCymru` added to web package as bridge between the
  Decorator interface and the Cymru resolver over modifying the Decorator
  interface itself, preserving backwards compatibility.

## Consequences

- All resolution code lives under `resolve/`, making it discoverable.
- Single DNS resolver instance shared by web UI and looking glass.
- Cymru resolver is reusable by any future consumer (not locked to web decorator).
- PeeringDB client is persistent in the resolve tree (no per-invocation creation).
- `ze resolve` CLI commands can be added in a follow-up spec by wiring to the
  existing resolver instances.
- IRR `rir_table.go` generation script updated to point to new location.

## Gotchas

- The `check-existing-patterns.sh` hook flags `New` and `Resolver` as duplicates
  across packages. These are independent types in different packages and are valid.
- goimports removes import aliases (like `resolveDNS`) when they are not yet used
  in the file. Must add import and usage in the same edit.
- The `parseASNForDecorator` helper in hub/main.go converts string ASN to uint32
  because the LG's DecorateASN callback receives a string but Cymru takes uint32.

## Files

- `internal/component/resolve/resolvers.go` -- Resolvers container
- `internal/component/resolve/cache/cache.go` -- Generic TTL cache
- `internal/component/resolve/dns/` -- DNS resolver (moved from component/dns/)
- `internal/component/resolve/cymru/cymru.go` -- Cymru ASN resolver (extracted from web/decorator_asn.go)
- `internal/component/resolve/peeringdb/client.go` -- PeeringDB client (moved from bgp/peeringdb/)
- `internal/component/resolve/irr/` -- IRR whois client (moved from bgp/irr/)
- `cmd/ze/hub/main.go` -- newResolvers(), updated startWebServer/startLGServer
- `internal/component/web/decorator_asn.go` -- Added NewASNNameDecoratorFromCymru
- `internal/component/plugin/all/all.go` -- Updated schema import path
- `docs/architecture/resolve.md` -- Architecture documentation
