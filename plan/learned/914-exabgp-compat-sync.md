# 914: ExaBGP Compatibility Sync + Attribute JSON Registry

## Context

Side-by-side comparison of ze and ExaBGP test data found gaps in both
directions: missing test cases, divergent expected output, and IPv6
extended community rendering falling through to opaque hex. The fix
required adding a proper JSON formatter for attribute code 25, which
exposed that the entire attribute JSON rendering switch was a
centralized dispatch that should use the registration pattern.

## Decisions

- **Attribute JSON rendering uses a registry, not a switch.** Each
  attribute type registers a `JSONFormatter` (key name + value appender)
  via `attribute.RegisterJSONFormatter`. The format package does a
  single registry lookup with mark/truncate for the nil/"not handled"
  fallback. This replaces a 120-line switch in `text_json.go`.

- **Formatter ownership follows AttrModHandler ownership.** If a plugin
  registers `filterapi.RegisterAttrModHandler` for an attribute code, it
  also owns that code's JSON formatter. Core attributes with no dedicated
  plugin register from `attribute/register.go`.

- **Current ownership:** core (origin, next-hop, as-path, MED, local-pref),
  filter_community (community, large-community, extended-community,
  IPv6 extended-community), aigp (AIGP).

- **SR-Policy migration deferred to separate spec.** Ze's ExaBGP config
  migration parses SR-Policy syntax but the route builder treats the
  content as a prefix. Needs SR-Policy-aware NLRI dispatch + Tunnel
  Encapsulation attribute building. Filed as spec-sr-policy-migration.md.

## Consequences

- Adding a new attribute's JSON rendering requires one `RegisterJSONFormatter`
  call in the owner's `register.go` and a named function in the owner's
  `json.go`. No core switch to edit.

- `AppendValue(buf, attr)` receives the caller's buffer directly (zero
  extra allocation). Returns nil to signal "not handled" (AIGP without
  metric), which triggers mark/truncate in the consumer.

## Gotchas

- **Origin unknown values:** `LowerString()` returns `""` for unknown
  origin codes but the old switch used `appendLower(o.String())` which
  produced `"unknown(N)"`. The formatter must use `appendLowerASCII(o.String())`
  to preserve the old behavior.

- **Never pass nil as buf to AppendValue.** `f.AppendValue(nil, attr)`
  forces every formatter to allocate a fresh slice. Pass the caller's
  buf and use mark/truncate for the fallback path.

- **ExaBGP `:json:` lines are documentation only.** The mock bgp server
  skips them (line 1019-1020 of `test/exabgp-compat/bin/bgp`). They are
  not validated. Updates are for reference accuracy.

- **EVPN label bottom-of-stack:** Ze encodes `000001` (S=1), ExaBGP
  encodes `000000` (S=0). Ze is correct per RFC 3032 + RFC 7432.

## Files

None recorded.
