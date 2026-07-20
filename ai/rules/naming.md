# Naming

**When:** naming anything in Ze (identifiers, files, commands, config)
**Severity:** advisory

## Directives

Rationale: `ai/rationale/naming.md`

"Ze" = "The" with a French accent. Use "ze" where "the" works grammatically.

| Context | Use |
|---------|-----|
| CLI binary | `ze` |
| BGP config YANG | `ze-bgp-conf` |
| BGP JSON format | `ze-bgp` |
| Go variables | `ZeBGPConf*` |
| YANG suffixes | config `-conf`, API `-api` |

Config-specific naming (YANG leaves, env var keys, Go struct fields):
`ai/rules/config-naming.md`

## Package-Naming Glossary

What the recurring package names mean, verified against each package's own doc
comment (`ai/PACKAGE-MAP.md`). When creating a NEW package, pick the term whose
definition matches your concern; do not coin a new synonym. The glossary
documents existing packages -- it does not force renames (existing protocols
keep their vocabulary; renames need an explicit user-approved shortlist).

| Term | Means (as a package name) | Canonical examples |
|------|---------------------------|--------------------|
| `packet` | The protocol's wire codec: parse + encode its PDUs/TLVs at the serialization boundary. Preferred term for new protocol codecs. | `component/bfd/packet` (BFD Control codec), `plugins/isis/packet` (PDU/TLV codec, "the protocol's serialization boundary") |
| `message` | Same role as `packet` for protocols whose RFC unit is the "message"; BGP legacy vocabulary. New protocols use `packet`. | `component/bgp/message` (OPEN/UPDATE/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH) |
| `wire` | Wire-level primitives or raw-byte containers shared between layers -- NOT a full codec: buffer writers, raw-packet handoff types. Exception: `ike/wire` is a full codec (predates this glossary). | `core/bgp/wire` (zero-allocation buffer writing), `plugins/ospf/wire` (AF-neutral RawPacket transport->engine handoff) |
| `session` | Per-peer/per-neighbor protocol state: state machine, timers, negotiation for ONE conversation. | `component/bfd/session` (per-session FSM, timer arithmetic, Poll/Final) |
| `fsm` | The RFC-defined state machine when the RFC names it that. | `component/bgp/fsm` (RFC 4271 Section 8) |
| `engine` | The protocol's runtime: the long-lived loop that owns sessions and executes the protocol. Preferred term for new protocol runtimes. | `component/bfd/engine` (express-loop runtime), `component/ike/engine` |
| `transport` | Socket I/O delivering wire bytes to/from the engine; may include an in-memory loopback for tests. | `component/bfd/transport` (UDP I/O + loopback), `plugins/isis/transport` |
| `reactor` | BGP-specific, historical: THE BGP event loop (peer sessions, wire events, plugin dispatch). Do not reuse for new protocols -- use `engine`. | `component/bgp/reactor` |
| `wireu` | "wire UPDATE": lazy-parsed BGP UPDATE messages with zero-copy iterators. Kept name (user decision 2026-07-08, spec-layout-3); a new package with this concern would spell it out. | `component/bgp/wireu` |

<!-- source: ai/PACKAGE-MAP.md -- generated package responsibilities backing each definition -->

### The `cli` / `cmd` / `command` trio (`internal/component/`)

| Package | Is | Use it when |
|---------|----|-------------|
| `component/cli` | The unified interactive TUI: config editing, CLI, and SSH sessions. | Adding an interactive surface or TUI behavior |
| `component/cmd` | A namespace of top-level CLI VERB implementations -- one subpackage per verb (`clear`, `delete`, `log`, `meta`, `metrics`, `monitor`, `set`, `show`, `subscribe`, `update`). | Adding or extending a top-level verb: `component/cmd/<verb>` |
| `component/command` | Shared types and logic for operational command execution (grammar, registry) consumed by the other two. | Adding command plumbing that more than one verb or surface needs |

### The four rib-named packages

| Package | Is |
|---------|----|
| `core/rib/` | Namespace, no root package: `locrib` (unified sharded Loc-RIB arbitrating best paths across protocols), `routeinstall` (RouteSink used by forked route-installing plugins in place of a direct Loc-RIB write), `store` (generic prefix-keyed route store on a BART trie) |
| `component/bgp/rib` | BGP's own RIB with per-attribute-type deduplication (the BGP Adj-RIB layer, distinct from the protocol-neutral Loc-RIB) |
| `core/routingtable` | Maps routing-table NAMES to kernel table IDs (the mapping types) |
| `plugins/routingtable` | The named-routing-table registry engine; wraps and re-exports `core/routingtable` so consumers keep a single import path |

<!-- source: internal/plugins/routingtable/registry.go -- re-export of core/routingtable -->
