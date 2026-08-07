---
kind: table
level:
stage:
---
| Module | Required? | Holds |
|--------|-----------|-------|
| root package | required | `register.go` registration (`sdk.NewWithConn` engine entry), config plumbing; may BE the engine |
| `packet/` | required | The wire codec: parse + encode the protocol's PDUs/TLVs (glossary: `packet`) |
| `transport/` | required | Socket I/O delivering wire bytes to/from the engine; in-memory loopback for tests welcome |
| `yang/` | required | Embedded + registered YANG schema modules (already uniform across all protocols) |
| engine home | required | The long-lived runtime loop: either the root package (IS-IS, OSPF) or a dedicated `engine/` (BFD, IKE) |
| per-peer state | required when the protocol has per-peer conversations | Named by the protocol's OWN RFC term: `session` (BFD), `adjacency` (IS-IS), `neighbor` (OSPF), `fsm` (BGP, RFC 4271). Do not flatten these to one word -- the RFC name is the discoverable one |
| `types/` | optional | Shared leaf types imported by codec and engine |
| `cli/` or `cmd/` | optional | Operational command handlers (see the glossary trio for which) |
| `redistribute/` | optional | Route redistribution glue |
| domain modules | optional | Protocol concepts named after the RFC concept: `lsdb`, `spf`, `sr`, `crypto`, `eap`, `ipsec`, `auth`, `circuit`, `iface`. Free naming, one concept per package |
| `v<N>/` | optional | Wire-version split (`ospf/v3`): version-specific `packet`/`types`/`transport` under a version dir, shared engine above it |
