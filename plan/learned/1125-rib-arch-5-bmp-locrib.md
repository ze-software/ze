# 1125 -- rib-arch-5: RFC 9069 BMP Loc-RIB Monitoring (PeerType=3)

## Context

The BMP plugin (`internal/component/bgp/plugins/bmp`) exported per-peer monitoring
(PeerType 0/1/2) but not RFC 9069 Loc-RIB monitoring (PeerType=3): a collector could
see what peers advertised, never the router's own best-path decisions. rib-arch-5 adds
Loc-RIB Route Monitoring built from the RIB's best-change stream, with a Loc-RIB Peer Up
lifecycle.

## Decisions

- **In-process EventBus subscription, not a RIB back-door or a cross-process pull.** bmp
  is an in-process BGP plugin, so it subscribes to the same `ze.EventBus` the rib
  publishes on -- `ribevents.BestChange.Subscribe(bus, cb)` -- exactly like
  `redistribute_egress` (register.go `ConfigureEventBus` -> package `atomic.Pointer`
  holder). The spec's Architectural Verification forbade reaching into rib internals
  (e.g. `reconstructWireAttrs`) for the full attribute set, so the public best-change
  event is the source of truth.
- **Minimal RM, documented fidelity limit.** `BestChangeEntry` carries prefix, next-hop,
  origin-AS and AS_PATH but NOT communities / local-pref, so a Loc-RIB RM reconstructs
  ORIGIN + AS_PATH + NEXT_HOP + NLRI (announce) or withdrawn-routes / MP_UNREACH
  (withdraw) and loses communities. RFC 9069 "Route Monitoring Content" only requires
  ORIGIN/AS_PATH/NEXT_HOP, and AS_PATH may be empty for locally originated routes, so the
  minimal form is compliant. Accepted the fidelity loss rather than couple to rib
  internals.
- **Reuse the attribute encoder; frame inline.** ORIGIN/AS_PATH/NEXT_HOP via
  `attribute.NewBuilder()` (the same encoder `injectRoute` uses), MP_REACH/MP_UNREACH via
  `attribute.NewMPReachNLRI` / `&attribute.MPUnreachNLRI{}` + `attribute.WriteAttrTo`. The
  UPDATE-body framing (withdrawn-len + attr-len + NLRI) is the trivial 4-byte-length
  wrapping every UPDATE builder shares (`wireu.buildUpdatePayload` is unexported), so it
  is built inline in `assembleUpdateBody` -- not a parallel encoder.
- **Broadcast replay for the initial dump.** On enable, bmp emits a broadcast
  `ribevents.ReplayRequest` (mirrors sysrib.go) so an operator turning on Loc-RIB BMP
  on a running router sees the current table (RFC 9069 "Initial dump sends full Loc-RIB
  contents"). The hop is broadcast -> every best-change subscriber (sysrib) re-processes;
  those paths dedup, so it is safe, just redundant work. Verified safe: the `.ci` runs
  the full default plugin set (sysrib+fib) and passes.
- **Local router-id from a cached sent OPEN.** RFC 9069 Peer BGP ID = local router-id.
  There is no local-router-id field on `StructuredEvent` (only the remote peer's), so
  `bgpIdentifierFromSentOpen` reads the BGP Identifier (offset 24) from any cached sent
  OPEN -- no new config surface.

## Consequences

- New config leaf `sender/loc-rib` (boolean, default false) in `ze-bmp-conf.yang`.
- Loc-RIB Peer Up is sent once, lazily, before the first Route Monitoring
  (`ensureLocRIBPeerUp`, guarded by `locRIBUp`); Peer Down best-effort on shutdown.
- Loc-RIB peer header: PeerType=3, Flags=0 (RFC 9069: F=0 in-Loc-RIB, V/L/A/O MUST be 0),
  Peer Address/AS = 0, Peer BGP ID = local router-id.

## Gotchas

- **The decoder could not round-trip its own Loc-RIB Peer Up.** RFC 9069 Loc-RIB Peer Up
  has ZERO-length sent/received OPENs, but `decodePeerUp` unconditionally called
  `extractBGPOpen` twice (each needs >=19 bytes) -- so it rejected the very PDU the sender
  emits. This latent decoder gap only surfaced because the unit test decoded the emitted
  bytes. Fix: `decodePeerUp` skips OPEN extraction for `PeerType==PeerTypeLocRIB`. A
  sender-only feature still needs the decoder to round-trip it (and to ingest Loc-RIB
  Peer Ups from other routers).
- `writeRouteMonitoring(peer, msgType, bgpBody)` takes the UPDATE BODY (no 19-byte BGP
  header); it synthesizes the header. So `buildLocRIBUpdateBody` must return the body
  only, never a full PDU.
- IPv6 announces MUST carry next-hop + NLRI in MP_REACH_NLRI, never the IPv4-only NEXT_HOP
  attribute (`SetNextHopAddr` silently no-ops on a v6 addr, which would have dropped the
  next-hop).

## Files

- `internal/component/bgp/plugins/bmp/bmp_locrib.go` -- header, UPDATE-body reconstruction,
  EventBus subscription, Peer Up/Down lifecycle
- `internal/component/bgp/plugins/bmp/bmp.go` -- `senderConfig.LocRIB`, wiring, `yangTrue`
- `internal/component/bgp/plugins/bmp/register.go` -- `ConfigureEventBus`
- `internal/component/bgp/plugins/bmp/msg.go` -- `decodePeerUp` PeerType=3 zero-length OPENs
- `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` -- `sender/loc-rib` leaf
- `internal/component/bgp/plugins/bmp/bmp_locrib_test.go`, `test/plugin/bmp-locrib.ci`
