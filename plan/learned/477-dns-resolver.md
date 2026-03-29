# 477 -- DNS Resolver Component

## Context

Ze had no centralized DNS resolution service. Components needing DNS queries (e.g., the decorator
framework for Team Cymru ASN-to-name lookups) would each need their own resolver setup. A shared
DNS resolver component under `internal/component/dns/` provides cached, configured DNS queries
for all Ze components.

## Decisions

- Chose `github.com/miekg/dns` (the library CoreDNS is built on) over Go's `net.Resolver` because
  miekg/dns gives control over query type, server selection, and response TTL extraction that
  `net.Resolver` does not expose.
- Placed under `internal/component/dns/` (not a plugin) because DNS resolution is cross-cutting
  infrastructure, not specific to any subsystem.
- YANG config under `environment/dns` following the same pattern as `environment/web`,
  `environment/ssh`, `environment/mcp`.
- Cache uses simple LRU with mutex over a fancier concurrent map because the cache is not a hot
  path (DNS queries are infrequent compared to BGP UPDATE processing).
- `ResolverConfig` named to avoid collision with existing `Config` types in other components
  (hook enforced uniqueness).
- NXDOMAIN returns empty results (no error) to simplify caller code -- callers check for empty
  results rather than distinguishing error types.

## Consequences

- Any component can now resolve DNS by receiving a `*dns.Resolver` instance. The decorator
  framework (`spec-decorator.md`) is the immediate consumer.
- Adding the miekg/dns dependency pulls in `golang.org/x/net` and upgrades `golang.org/x/tools`
  and `golang.org/x/mod`.
- New YANG modules that define containers under `environment` must be explicitly loaded in
  `internal/component/config/yang_schema.go` (the `GetEntry` + `Define` pattern). Registration
  alone is not enough.

## Gotchas

- YANG schema registration via `init()` + `yang.RegisterModule()` makes the schema available to
  the loader, but the config parser requires an explicit `loader.GetEntry()` + `schema.Define()`
  block in `yang_schema.go` for each module. Without this, config validation rejects the new
  section as "unknown field."
- The `check-existing-patterns.sh` hook rejects type names that exist in other packages (e.g.,
  `Config`), even though Go allows same-named types in different packages.
- The `block-silent-ignore.sh` hook rejects `default:` cases in switch statements. Use explicit
  type cases only and omit default.
- Race detector caught a data race in test code accessing a counter from both the test goroutine
  and the DNS server handler goroutine. Used `atomic.Int32` to fix.

## Files

- `internal/component/dns/cache.go` -- in-memory cache with TTL and LRU eviction
- `internal/component/dns/cache_test.go` -- 7 cache tests
- `internal/component/dns/resolver.go` -- miekg/dns client with cache integration
- `internal/component/dns/resolver_test.go` -- 10 resolver tests (local DNS server)
- `internal/component/dns/schema/ze-dns-conf.yang` -- YANG config schema
- `internal/component/dns/schema/embed.go` -- embedded YANG file
- `internal/component/dns/schema/register.go` -- YANG module registration
- `internal/component/dns/schema/schema_test.go` -- 2 schema validation tests
- `internal/component/config/yang_schema.go` -- added DNS module loading
- `internal/component/plugin/all/all.go` -- regenerated with DNS schema import
- `test/parse/dns-config.ci` -- functional test for config parsing
