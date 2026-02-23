# Architecture Summary

Rationale: `.claude/rationale/architecture-summary.md`
Full details: `docs/architecture/core-design.md`

## System

```
Engine: Peers (FSM) → Reactor (event loop, BGP cache)
   ║ JSON events + base64 wire bytes (down) / commands (up)
Plugin: RIB, RR, GR, etc. (Go/Python/Rust)
```

Engine passes wire bytes to plugins. Plugins implement RIB, dedup, policy.

## Negotiated Capabilities (per-peer)

| Field | Type | Effect |
|-------|------|--------|
| ASN4 | bool | 4-byte ASN in AS_PATH |
| AddPath | map[Family]Mode | Path-ID prefix in NLRI |
| ExtendedMsg | bool | 65535 byte messages |
| ExtendedNextHop | map[Family]AFI | Per-family NH mapping |
| GracefulRestart | *GR | RFC 4724 state |
| RouteRefresh | bool | RFC 2918 |

Same wire bytes parse differently based on caps. ContextID identifies encoding context for zero-copy.

## Wire Writing

All types implement `BufWriter`: `WriteTo(buf, off) int` or `CheckedWriteTo(buf, off) (int, error)`.
Context-dependent types take `*PackContext` for ADD-PATH/ASN4.

## UPDATE Structure

```
UPDATE = Header (19B) + Withdrawn (IPv4) + Path Attributes
  + MP_REACH_NLRI (non-IPv4 announce) + MP_UNREACH_NLRI (non-IPv4 withdraw)
  + NLRI (IPv4 unicast only)
```

## WireUpdate vs RIB

- WireUpdate = transport (lazy parse via iterators, keeps wire refs)
- RIB = storage (NLRI → attribute refs into per-type pools, NOT WireUpdate)
- Per-attribute-type pools with dedup. Per-family NLRI pools.

## Forward Pool

Current: per-destination-peer workers (goroutine + channel per peer).
Future consideration: shared round-robin pool with dynamic scaling.

## Chaos Simulator

Unbounded event buffer — no events ever dropped. Ring buffer rejected because losing route events breaks convergence counts.

## API Command Syntax

```
Text:   update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24
Binary: update hex attr set 400101... nlri ipv4/unicast add 180a00
```
