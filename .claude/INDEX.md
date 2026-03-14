# Ze Documentation Index

## Architecture Docs

| Area | Doc |
|------|-----|
| **Core Design** | `docs/architecture/core-design.md` **(START HERE)** |
| **Hub Architecture** | `docs/architecture/hub-architecture.md` |
| Buffer-first | `docs/architecture/buffer-architecture.md` |
| Wire formats | `docs/architecture/wire/messages.md` |
| NLRI types | `docs/architecture/wire/nlri.md` |
| Attributes | `docs/architecture/wire/attributes.md` |
| Capabilities | `docs/architecture/wire/capabilities.md` |
| UPDATE building | `docs/architecture/update-building.md` |
| Memory pools | `docs/architecture/pool-architecture.md` |
| Zero-copy | `docs/architecture/encoding-context.md` |
| RIB transition | `docs/architecture/rib-transition.md` |
| Route types | `docs/architecture/route-types.md` |
| FSM | `docs/architecture/behavior/fsm.md` |
| API | `docs/architecture/api/architecture.md` |
| API Capabilities | `docs/architecture/api/capability-contract.md` |
| Text format | `docs/architecture/api/text-format.md` |
| Text parser | `docs/architecture/api/text-parser.md` |
| Text coverage | `docs/architecture/api/text-coverage.md` |
| Config syntax | `docs/architecture/config/syntax.md` |
| YANG design | `docs/architecture/config/yang-config-design.md` |
| ZeFS format | `docs/architecture/zefs-format.md` |
| ExaBGP mapping | `docs/exabgp/exabgp-code-map.md` |
| ExaBGP compat | `docs/exabgp/exabgp-differences.md` |

## Keyword → Architecture Doc

| Keywords | Docs |
|----------|------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md`, `rules/buffer-first.md` |
| encode, Pack, WriteTo, alloc | `rules/buffer-first.md`, `buffer-architecture.md` |
| UPDATE, message, build, route | `core-design.md`, `update-building.md`, `encoding-context.md` |
| attribute, AS_PATH, NEXT_HOP, MED | `core-design.md`, `wire/attributes.md`, `update-building.md` |
| community, ext community, large community | `wire/attributes.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `core-design.md`, `wire/nlri.md` |
| multiprotocol, AFI, SAFI | `wire/nlri.md`, `wire/capabilities.md` |
| capability, OPEN, negotiate | `wire/capabilities.md` |
| pool, memory, dedup, zero-copy | `core-design.md`, `pool-architecture.md`, `encoding-context.md` |
| forward, reflect, wire cache | `core-design.md`, `encoding-context.md`, `update-building.md` |
| route, rib, storage | `core-design.md`, `route-types.md`, `rib-transition.md` |
| FSM, state, session, peer | `behavior/fsm.md` |
| API, command, announce, withdraw | `api/architecture.md`, `api/capability-contract.md` |
| text format, IPC, formatter, parser | `api/text-format.md`, `api/text-parser.md`, `api/text-coverage.md` |
| config, load | `config/syntax.md` |
| zefs, blob, netcapstring, storage | `zefs-format.md` |
| FlowSpec | `wire/nlri.md`, `wire/nlri-flowspec.md` |
| VPN, L3VPN, MPLS-VPN, 6PE | `wire/nlri.md` |
| EVPN, MAC-IP | `wire/nlri.md`, `wire/nlri-evpn.md` |
| BGP-LS, link-state | `wire/nlri-bgpls.md` |
| ExaBGP | `exabgp/exabgp-code-map.md`, `exabgp/exabgp-differences.md` |
| ASN4, AS4 | `edge-cases/as4.md` |
| ADD-PATH | `edge-cases/addpath.md` |
| extended message | `edge-cases/extended-message.md` |
| test, functional, .ci | `functional-tests.md`, `testing/ci-format.md` |

All architecture docs in `docs/architecture/` unless noted.

## Keyword → RFC

| Keywords | Primary RFC | Related |
|----------|-------------|---------|
| open, capability | `rfc5492` | `rfc9072` |
| update, nlri, prefix | `rfc4271` | `rfc4760` |
| multiprotocol, mp-bgp | `rfc4760` | |
| notification, error | `rfc4271` | `rfc7606`, `rfc9003` |
| route-refresh | `rfc2918` | `rfc7313` |
| community | `rfc1997` | |
| extended community, RT | `rfc4360` | `rfc5701` |
| large community | `rfc8092` | `rfc8195` |
| 4-byte AS, ASN4 | `rfc6793` | |
| add-path | `rfc7911` | |
| graceful restart | `rfc4724` | |
| extended message | `rfc8654` | |
| label, mpls | `rfc8277` | `rfc3032` |
| vpn, l3vpn, 6pe | `rfc4364` | `rfc4659`, `rfc4798` |
| flowspec | `rfc8955` | `rfc8956` |
| evpn | `rfc7432` | `rfc9136` |
| vpls | `rfc4761` | `rfc4762` |
| bgp-ls | `rfc7752` | `rfc9085`, `rfc9514` |
| role, otc | `rfc9234` | |
| ipv6 next hop | `rfc8950` | |
| shutdown | `rfc9003` | `rfc8203` |
| treat-as-withdraw | `rfc7606` | |

RFC summaries: `rfc/short/`. Full RFCs: `rfc/full/`.

## Session State

Track in `.claude/session-state.md` (not committed). Template: `.claude/session-state.md.template`.
