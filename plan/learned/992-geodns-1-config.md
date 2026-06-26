# 992 -- geodns-1-config

## Context

The first geodns child ([[991-geodns-0-umbrella]]): model the entire geodns
configuration in YANG and parse/validate it into an in-memory resolver state.
Daemon settings (bind addresses, port, TTL, zones, nameservers, SOA, client-IP
source) plus the per-source host records that the reference kept in a watched
`etc/geodns/<ip>` folder. No listener yet -- this produces a validated, queryable
config the server child binds to.

## Decisions

- **Named `host-set`s + `source { prefix; host-set <name> }`.** A `source` maps a
  CIDR to a host-set by name; many prefixes can point at one set (preserving the
  reference's shared `internal`). Chosen over inlining hosts per source, which would
  duplicate records across prefixes.
- **`type string` host-set reference, verifier-enforced.** ze's YANG loader has no
  `leafref`, so `parseConfig` checks every `source.host-set` resolves to a defined
  host-set; a dangling reference aborts the commit.
- **Optional record `type`, auto-detected per address** (`addrRecord`: IPv4 -> A,
  else AAAA). A single host line with mixed v4/v6 yields both an A and an AAAA,
  matching the reference's `recordIP`. An explicit `type` constrains the family.
- **Listeners use the ze `zt:listener` model** (`list listener { ze:listener; uses
  zt:listener }`, ip+port per named entry), aligning geodns with every other ze
  listener service (gnmi, web, mcp, ssh, api) and getting config-time port-conflict
  detection for free via `ze:listener`. Defaults to 127.0.0.1:5300 and ::1:5300 in
  Go when no listener is configured. (Chosen over a `leaf-list listen-address` +
  scalar port, which was the first cut but diverged from the ze convention.)

## Consequences

- The config JSON the plugin receives encodes scalars as strings, leaf-lists as
  arrays, and list entries as maps keyed by the (hoisted) key leaf -- `parseConfig`
  mirrors the dhcpserver idiom exactly.
- A longest-prefix `matcher` (sorted by prefix bits desc, `netip.Prefix.Contains`)
  resolves a client IP to a host-set; family-aware, so v4 never matches a v6 prefix.
- `resolverState` (config + matcher + SOA serial) is the atomic snapshot the server
  and `show` read; published via `storeState`.

## Gotchas

- A `switch` on `recordKind`/qtype needs a `default` (exhaustive linter); `+` string
  concatenation is banned (use `textbuf.Buffer`); `Close()` errors must be handled
  explicitly even in tests; type assertions need the comma-ok form (errcheck
  check-type-assertions).

## Files

- `internal/plugins/geodns/yang/ze-geodns-conf.yang` -- schema
- `internal/plugins/geodns/config.go` -- parseConfig + validation
- `internal/plugins/geodns/source.go` -- longest-prefix matcher
- `internal/plugins/geodns/record.go`, `state.go` -- record model, atomic snapshot
- `test/parse/geodns-config.ci`, `geodns-invalid-record.ci`
