# ZeBGP Documentation Index

## Architecture Docs

Read when working on specific areas:

| Area | Doc |
|------|-----|
| Wire formats | `zebgp/wire/MESSAGES.md` |
| NLRI types | `zebgp/wire/NLRI.md` |
| Attributes | `zebgp/wire/ATTRIBUTES.md` |
| Capabilities | `zebgp/wire/CAPABILITIES.md` |
| UPDATE building | `zebgp/UPDATE_BUILDING.md` |
| Memory pools | `zebgp/POOL_ARCHITECTURE.md` |
| Zero-copy | `zebgp/ENCODING_CONTEXT.md` |
| ExaBGP mapping | `zebgp/EXABGP_CODE_MAP.md` |
| ExaBGP compat | `zebgp/EXABGP_COMPATIBILITY.md` |
| FSM | `zebgp/behavior/FSM.md` |
| API | `zebgp/api/ARCHITECTURE.md` |
| API Capabilities | `zebgp/api/CAPABILITY_CONTRACT.md` |
| Config syntax | `zebgp/config/SYNTAX.md` |

## Rules (auto-loaded by path)

| Rule | Applies To |
|------|------------|
| `rules/planning.md` | `*` (non-trivial features) |
| `rules/tdd.md` | `**/*.go` |
| `rules/go-standards.md` | `**/*.go` |
| `rules/rfc-compliance.md` | `pkg/bgp/**/*.go` |
| `rules/git-safety.md` | `*` |
| `rules/config-design.md` | Config changes |

## Edge Cases

| Topic | Doc |
|-------|-----|
| ASN4 handling | `zebgp/edge-cases/AS4.md` |
| ADD-PATH | `zebgp/edge-cases/ADDPATH.md` |
| Extended messages | `zebgp/edge-cases/EXTENDED_MESSAGE.md` |

## RFC Summaries

Implementation-ready RFC summaries in `zebgp/rfc/`. Use keyword table to find relevant RFCs.

### By Topic

| Topic | RFCs to Read |
|-------|--------------|
| **Core BGP** | `rfc4271.md` (base), `rfc4760.md` (MP-BGP) |
| **OPEN message** | `rfc4271.md`, `rfc5492.md` (capabilities), `rfc9072.md` (extended params) |
| **UPDATE message** | `rfc4271.md`, `rfc4760.md` (MP_REACH/UNREACH) |
| **NOTIFICATION** | `rfc4271.md`, `rfc8203.md`/`rfc9003.md` (shutdown message) |
| **KEEPALIVE** | `rfc4271.md` |
| **ROUTE-REFRESH** | `rfc2918.md`, `rfc7313.md` (enhanced) |
| **Error handling** | `rfc7606.md` (revised), `rfc4271.md` §6 |
| **FSM/state machine** | `rfc4271.md` §8, `rfc4724.md` (graceful restart) |

### Attributes

| Attribute | RFCs |
|-----------|------|
| AS_PATH, AS4_PATH | `rfc4271.md`, `rfc6793.md` (4-byte AS) |
| NEXT_HOP, MP_REACH | `rfc4271.md`, `rfc4760.md`, `rfc8950.md` (IPv6 NH) |
| COMMUNITIES | `rfc1997.md` |
| EXTENDED_COMMUNITIES | `rfc4360.md`, `rfc5701.md` (IPv6) |
| LARGE_COMMUNITIES | `rfc8092.md`, `rfc8195.md` (usage) |
| OTC (Only to Customer) | `rfc9234.md` |

### Capabilities

| Capability | RFCs |
|------------|------|
| Multiprotocol (code 1) | `rfc4760.md` |
| Route Refresh (code 2) | `rfc2918.md` |
| 4-byte AS (code 65) | `rfc6793.md` |
| ADD-PATH (code 69) | `rfc7911.md` |
| Extended NH (code 5) | `rfc8950.md` (obsoletes `rfc5549.md`) |
| Graceful Restart (code 64) | `rfc4724.md` |
| Extended Message (code 6) | `rfc8654.md` |
| BGP Role (code 9) | `rfc9234.md` |
| Multiple Labels (code 8) | `rfc8277.md` |

### AFI/SAFI Families

| Family | RFCs |
|--------|------|
| IPv4/IPv6 Unicast | `rfc4271.md`, `rfc4760.md` |
| Labeled Unicast (SAFI 4) | `rfc8277.md`, `rfc3032.md` (MPLS) |
| VPN-IPv4/IPv6 (SAFI 128) | `rfc4364.md`, `rfc4659.md` |
| FlowSpec (SAFI 133/134) | `rfc8955.md` (obsoletes `rfc5575.md`), `rfc8956.md` (IPv6) |
| EVPN (SAFI 70) | `rfc7432.md`, `rfc9136.md` (RT-5) |
| VPLS (SAFI 65) | `rfc4761.md` |
| RT Constraint (SAFI 132) | `rfc4684.md` |
| BGP-LS (AFI 16388) | `rfc7752.md`, `rfc9085.md` (SR), `rfc9514.md` (SRv6) |

### Keyword → RFC Mapping

| Keywords | Primary RFC | Related |
|----------|-------------|---------|
| open, capability, negotiate | `rfc5492.md` | `rfc9072.md` |
| update, nlri, prefix, route | `rfc4271.md` | `rfc4760.md` |
| notification, error, cease | `rfc4271.md` | `rfc7606.md`, `rfc9003.md` |
| keepalive, hold timer | `rfc4271.md` | |
| route-refresh, orf | `rfc2918.md` | `rfc7313.md` |
| community, well-known | `rfc1997.md` | |
| extended community, RT, RD | `rfc4360.md` | `rfc5701.md` |
| large community | `rfc8092.md` | `rfc8195.md` |
| 4-byte AS, ASN4, AS4 | `rfc6793.md` | `rfc4271.md` |
| add-path, path-id | `rfc7911.md` | |
| graceful restart, GR | `rfc4724.md` | |
| extended message, >4096 | `rfc8654.md` | |
| label, mpls, labeled | `rfc8277.md` | `rfc3032.md` |
| vpn, l3vpn, mpls-vpn | `rfc4364.md` | `rfc4659.md` |
| flowspec, traffic filter | `rfc8955.md` | `rfc8956.md` |
| evpn, mac-ip, ethernet | `rfc7432.md` | `rfc9136.md` |
| vpls, l2vpn | `rfc4761.md` | |
| bgp-ls, link-state | `rfc7752.md` | `rfc9085.md`, `rfc9514.md` |
| segment routing, sr, sid | `rfc9085.md` | `rfc9514.md` |
| srv6 | `rfc9514.md` | |
| role, otc, route leak | `rfc9234.md` | |
| ipv6 next hop | `rfc8950.md` | |
| shutdown, reset, admin | `rfc9003.md` | `rfc8203.md` |
| treat-as-withdraw | `rfc7606.md` | |

### Obsoleted RFCs (Do Not Use)

| Obsoleted | Replacement | Notes |
|-----------|-------------|-------|
| `rfc5549.md` | `rfc8950.md` | IPv4 NLRI with IPv6 NH |
| `rfc5575.md` | `rfc8955.md` | FlowSpec |
| `rfc7752.md` | RFC 9552 (no summary yet) | BGP-LS |
| `rfc8203.md` | `rfc9003.md` | Admin Shutdown (255 vs 128 bytes) |

## Reference

- Current state: `plan/CLAUDE_CONTINUATION.md`
- RFC folder: `rfc/`
- RFC summaries: `zebgp/rfc/`
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
