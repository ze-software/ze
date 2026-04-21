# Architecture Summary Rationale

Why: `ai/rules/architecture-summary.md`

## System Diagram

```
+-------------------------------------------------------------------+
|                          ZeBGP ENGINE                              |
|   +--------+  +--------+  +--------+                              |
|   | Peer 1 |  | Peer 2 |  | Peer N |  (BGP sessions)             |
|   |  FSM   |  |  FSM   |  |  FSM   |                              |
|   +---+----+  +---+----+  +---+----+                              |
|       +------------+----------+                                    |
|                    v                                               |
|             +-----------+                                          |
|             |  Reactor  |  (event loop, BGP cache)                 |
|             +-----------+                                          |
+-------------------------------------------------------------------+
                          |               ^
        JSON events (down)|               | commands (up)
        + base64 wire     |               | update/forward/withdraw
                          v               |
====== PROCESS BOUNDARY (stdin/stdout pipes) ========================
                          |               ^
                          v               |
                   +-------------+
                   |   Plugin    |  (Go/Python/Rust/etc.)
                   |  (RIB/RR)  |
                   +-------------+
```

## Why Negotiated Capabilities Matter (byte-level examples)
- Same wire bytes parse differently based on negotiated caps
- `AS_PATH [00 01 FD E8]` = ASN 65000 (ASN4) or two ASNs 1, 64488 (ASN2)
- NLRI `[00 00 00 01 18 0a 00 00]` = path-id + prefix (ADD-PATH) or two prefixes (no ADD-PATH)
- ContextID identifies encoding context for zero-copy decisions. Same ContextID = same caps = can forward wire bytes unchanged.

## Negotiated Capabilities Struct
```
Negotiated (per-peer) -- see internal/bgp/capability/negotiated.go
  ASN4            bool              -- 4-byte ASN support
  AddPath         map[Family]Mode   -- Receive/Send/Both
  ExtendedMsg     bool              -- 65535 byte messages
  ExtendedNextHop map[Family]AFI    -- Per-family NH mapping
  Families()      []Family          -- Negotiated families
  GracefulRestart *GracefulRestart  -- RFC 4724 GR state
  RouteRefresh    bool              -- RFC 2918 support
```

## WireUpdate vs RIB Diagram

```
WireUpdate (transport)          RIB (storage)
+---------------------+        +-----------------------------------+
| Attributes (shared) |        | NLRI 10.0.0.0/24 -> origin_ref --+--> Pool: IGP
| NLRI: 10.0.0.0/24   | -----> |                    aspath_ref --+--> Pool: [65001]
| NLRI: 10.0.1.0/24   |        |                    localpref --+--> Pool: 100
| NLRI: 10.0.2.0/24   |        | NLRI 10.0.1.0/24 -> (same) ----+--> (shared)
+---------------------+        +-----------------------------------+
```

## Reference Pointers
- Full API syntax: `docs/architecture/api/update-syntax.md`
- Full core design: `docs/architecture/core-design.md`
