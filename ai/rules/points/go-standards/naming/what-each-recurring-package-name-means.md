---
kind: table
level:
stage:
---
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
