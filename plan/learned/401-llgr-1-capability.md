# 401 -- LLGR Capability Wire + Config

## Objective

Add LLGR capability (code 71) wire decode/encode per RFC 9494, YANG config for long-lived-stale-time, CLI decode, and well-known community constants (LLGR_STALE, NO_LLGR).

## Decisions

- LLGR decode lives in dedicated `gr_llgr.go` (312 lines), separate from GR's cap-64 decode -- keeps concerns isolated while in the same plugin package
- `llgrPeerCap` / `llgrCapFamily` types mirror GR's `grPeerCap` pattern for per-family LLST + F-bit storage
- Well-known communities added to `attribute/community.go` alongside existing constants: `CommunityLLGRStale = 0xFFFF0006`, `CommunityNoLLGR = 0xFFFF0007`
- YANG `long-lived-stale-time` is a leaf under graceful-restart (not a list), augmented into all three capability paths (peer, peer-in-group, group)
- Registration extended: `CapabilityCodes: []uint8{64, 71}` -- single plugin handles both

## Patterns

- Capability wire format: 7-byte tuples (AFI(2) + SAFI(1) + Flags(1) + LLST(3)), no global header (unlike GR's 2-byte header)
- `parseLLGRCapValue()` validates 24-bit range (0-16777215) and clamps -- same validation pattern as GR's restart-time
- `extractLLGRCapabilities()` produces `CapabilityDecl` for code 71, called alongside existing GR extraction in OnConfigure
- CLI decode: `runCLIDecodeLLGR()` and `decodeLLGRMode()` follow the same pattern as GR's RunDecodeMode

## Gotchas

- LLGR MUST be ignored if GR capability (code 64) is not also present -- tested in `TestHandleEventOpenLLGR_NoGR`
- Truncated tuples (<7 bytes remaining) are silently skipped, not errors -- matches RFC tolerance
- LLGR capability has no global header unlike GR, so tuple count is simply `len(value) / 7`
- Community name registration in `register.go` needed for display: `CommunityLLGRStale` mapped to "LLGR_STALE"

## Files

- `internal/component/bgp/plugins/gr/gr_llgr.go` -- all LLGR capability handling (decode, config, CLI)
- `internal/component/bgp/plugins/gr/register.go` -- cap code 71, community name registration
- `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` -- long-lived-stale-time leaf
- `internal/component/bgp/attribute/community.go` -- LLGR_STALE, NO_LLGR constants
- `test/parse/graceful-restart-llgr.ci` -- config parsing functional test
- `test/plugin/plugin-gr-llgr-capa.ci` -- CLI capability decode functional test
