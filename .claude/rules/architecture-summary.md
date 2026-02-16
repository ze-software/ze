# Architecture Summary

**READ `docs/architecture/core-design.md` for full details.** This is a condensed always-in-context reference.

## System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ZeBGP ENGINE                                    │
│                                                                             │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐                                    │
│   │ Peer 1  │  │ Peer 2  │  │ Peer N  │   (BGP sessions)                   │
│   │  FSM    │  │  FSM    │  │  FSM    │                                    │
│   └────┬────┘  └────┬────┘  └────┬────┘                                    │
│        │            │            │                                          │
│        └────────────┼────────────┘                                          │
│                     ▼                                                       │
│              ┌─────────────┐                                                │
│              │   Reactor   │  (event loop, BGP cache)                      │
│              └─────────────┘                                                │
└─────────────────────────────────────────────────────────────────────────────┘
                              │                 ▲
          JSON events (down)  │                 │  commands (up)
          + base64 wire bytes │                 │  update/forward/withdraw
                              ▼                 │
═══════════════════════ PROCESS BOUNDARY (stdin/stdout pipes) ════════════════
                              │                 ▲
                              ▼                 │
                      ┌───────────────┐
                      │    Plugin     │  (Go/Python/Rust/etc.)
                      │  (RIB / RR)   │
                      └───────────────┘
```

**Key insight:** Engine passes wire bytes to plugins. Plugins implement RIB, deduplication, policy.

## Peer Context & Negotiated Capabilities

Decoding/encoding BGP messages requires **negotiated capabilities** from OPEN exchange:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Negotiated (per-peer)                        │
│         See internal/bgp/capability/negotiated.go for full struct    │
├─────────────────────────────────────────────────────────────────┤
│ ASN4            bool                 → 4-byte ASN support       │
│ AddPath         map[Family]Mode      → Receive/Send/Both        │
│ ExtendedMsg     bool                 → 65535 byte messages      │
│ ExtendedNextHop map[Family]AFI       → Per-family NH mapping    │
│ Families()      []Family             → Negotiated families      │
│ GracefulRestart *GracefulRestart     → RFC 4724 GR state        │
│ RouteRefresh    bool                 → RFC 2918 support         │
└─────────────────────────────────────────────────────────────────┘
```

**Why it matters:**
- Same wire bytes parse differently based on negotiated caps
- `AS_PATH [00 01 FD E8]` = ASN 65000 (ASN4) or two ASNs 1, 64488 (ASN2)
- NLRI `[00 00 00 01 18 0a 00 00]` = path-id + prefix (ADD-PATH) or two prefixes (no ADD-PATH)

**ContextID:** Identifies encoding context for zero-copy decisions. Same ContextID = same caps = can forward wire bytes unchanged.

**Wire Writing:** All wire types implement `BufWriter` interface:
- `WriteTo(buf, off) int` - write to pre-allocated buffer (caller guarantees capacity)
- `CheckedWriteTo(buf, off) (int, error)` - validates capacity first
- Context-dependent types (NLRI, ASPath) take `*PackContext` for ADD-PATH/ASN4 handling

## BGP UPDATE = Container

```
UPDATE Message (wire bytes)
├── Header (19 bytes)
├── Withdrawn Routes (IPv4 unicast)
├── Path Attributes
│   ├── ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, ...
│   ├── MP_REACH_NLRI (NLRI for non-IPv4-unicast)
│   └── MP_UNREACH_NLRI (withdrawals for non-IPv4-unicast)
└── NLRI (IPv4 unicast only)
```

## WireUpdate vs RIB

```
WireUpdate (transport)              RIB (storage)
┌─────────────────────┐             ┌────────────────────────────────────┐
│ Attributes (shared) │             │ NLRI 10.0.0.0/24 → origin_ref ─────┼─→ Pool: IGP
│ NLRI: 10.0.0.0/24   │   ────▶     │                    aspath_ref ─────┼─→ Pool: [65001]
│ NLRI: 10.0.1.0/24   │             │                    localpref_ref ──┼─→ Pool: 100
│ NLRI: 10.0.2.0/24   │             │ NLRI 10.0.1.0/24 → (same refs) ────┼─→ (shared)
└─────────────────────┘             └────────────────────────────────────┘
```

**Key points:**
- `WireUpdate` = transport (UPDATE bytes, lazy parse via iterators)
- `RIB` = storage (NLRI → attribute refs, NOT WireUpdate)
- Per-attribute-type pools (ORIGIN, AS_PATH, LOCAL_PREF, MED, COMMUNITY, etc.)
- Per-family NLRI pools (`map[Family]*Pool[NLRI]`)
- Next-hop: special encoding (attribute vs MP_REACH_NLRI depending on family)

## API Command Syntax

```
Text mode:   update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24
Binary mode: update hex attr set 400101... nlri ipv4/unicast add 180a00
```

Both produce WireUpdate with wire bytes.
- Full syntax: `docs/architecture/api/update-syntax.md`
- Full design: `docs/architecture/core-design.md`
