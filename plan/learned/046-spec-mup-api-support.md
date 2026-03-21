# 046 — MUP API Support

## Objective

Add MUP SAFI (draft-mpmz-bess-mup-safi, SAFI 85) to the API parser so `announce ipv4/mup ...` commands produce correct MUP UPDATE messages, fixing the `mup4` and `mup6` functional tests.

## Decisions

- Added `"mup"` to `parseSAFI()` dispatch table alongside existing `unicast`, `nlri-mpls`, `mpls-vpn`.
- `MUPRouteSpec` struct in `types.go` carries all MUP-specific fields: `RouteType`, `IsIPv6`, `Prefix`, `Address`, `RD`, `TEID`, `QFI`, `Endpoint`, `Source`, `NextHop`, `ExtCommunity`, `PrefixSID`.
- `AnnounceMUPRoute(peerSelector, MUPRouteSpec)` added to `ReactorInterface` — same pattern as `AnnounceRoute`.
- T1ST/T2ST TEID/QFI/endpoint fields are parsed but not encoded (deferred with TODO comments) — data too unclear from the draft spec.

## Patterns

- The existing `convertMUPRoute()` in `internal/component/config/loader.go` and `buildMUPNLRI()` in `reactor/peer.go` were the reference for how MUP routes should be structured.

## Gotchas

- Static MUP routes from config worked before this spec; only API-injected MUP routes were broken (parser returned "unsupported SAFI" then fell through to unicast handling).
- `parseSAFI()` silently falling through to unicast caused malformed wire format — no error was surfaced to the caller.

## Files

- `internal/component/plugin/route.go` — `SAFINameMUP` constant, `announceMUPImpl()`, `withdrawMUPImpl()`
- `internal/component/plugin/types.go` — `MUPRouteSpec`, `AnnounceMUPRoute` in `ReactorInterface`
- `internal/reactor/reactor.go` — `AnnounceMUPRoute()` implementation
