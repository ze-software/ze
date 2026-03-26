# 433 -- Interop Coverage

## Context

Ze had 21 interop scenarios covering core BGP (session, routes, capabilities, communities, auth, route server) but only tested IPv4 unicast address families with live peers. EVPN, VPN, FlowSpec encoding was validated byte-for-byte via ExaBGP compat tests but never with real daemons. IPv6 was tested only with FRR, not BIRD or GoBGP. Multi-hop eBGP was untested. The goal was to expand the interop matrix to cover all address families across all available vendors.

## Decisions

- **Numbered scenarios 22-32** (over 20-25 originally planned) because scenarios 20-21 were added for RFC 9234 roles after the spec was first written.
- **Expanded to full vendor matrix** (over FRR-only per feature): each feature tested with every vendor that supports it. EVPN/VPN/FlowSpec with both FRR and GoBGP. Multihop with all three. BIRD excluded from EVPN/VPN/FlowSpec (no support).
- **Used `origin:` extended community for VPN** (over `target:` Route Target) because the text API parser only supports `origin`, `redirect`, `rate-limit`. Route Target (`target:`) is not implemented in the text command parser.
- **Used `destination`/`source` component keywords for FlowSpec** (over `destination-ipv4`/`source-ipv4`) because the text API encoder normalizes differently from the config parser.
- **Config format uses new syntax**: `peer <name> { remote { ip; as; } local { ip; as; } family { ... { prefix { maximum N; } } } }`. Old format (`peer <IP> { local-address; local-as; peer-as; }`) is rejected by current parser. Existing scenarios 01-21 still use old format and are broken.

## Consequences

- 11 new scenarios provide triangulation: when a feature fails with both FRR and GoBGP, the bug is in Ze. When it passes with one but fails with another, the issue is vendor-specific.
- 4 scenarios pass (IPv6/GoBGP, multihop/all three vendors), proving the test infrastructure and process plugin delivery work correctly for IPv4 and IPv6 unicast.
- 7 scenarios fail, revealing Ze bugs in EVPN/VPN/FlowSpec capability negotiation and route delivery. These are tracked in `plan/spec-interop-failures.md`.
- Existing scenarios 01-21 need config migration to the new parser format (separate work, not part of this spec).

## Gotchas

- **Text API parser differs from config parser**: `destination-ipv4` works in config blocks but the text API requires `destination`. `=tcp` operator syntax works in config FlowSpec but the text API requires plain `tcp`. The deep review caught these before live testing.
- **Extended community `target:` not supported in text API**: the parser switch only handles `origin`/`redirect`/`rate-limit`. The VPN encode test (`vpn.ci`) uses `target:` in config static blocks (different code path), which was misleading.
- **FlowSpec text API requires explicit `add` keyword**: `nlri ipv4/flow add destination ...` not `nlri ipv4/flow destination ...`. Existing encode tests (`flow-redirect.ci`, `simple-flow.ci`) omit `add` in `cmd=api` commands, which may indicate those tests use a different dispatch path or are silently failing.
- **FlowSpec requires `nhop`** even though the next-hop is semantically unused: without it, `resolveNextHop` returns `ErrNextHopUnset` and the route is silently dropped.
- **Config format changed significantly** since the original interop scenarios were written. Three iterations were needed: flat keys -> `remote{}`/`local{}` blocks, IP peer keys -> named peers, bare families -> `prefix { maximum N; }` blocks.

## Files

- `test/interop/scenarios/22-evpn-frr/` through `32-multihop-ebgp-gobgp/` (44 new files)
- `docs/architecture/testing/interop.md` (inventory + gaps list updated)
- `plan/spec-interop-failures.md` (investigation spec for 7 failing scenarios)
