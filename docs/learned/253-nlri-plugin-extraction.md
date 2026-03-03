# 253 — NLRI Plugin Extraction

## Objective

Extract all non-INET NLRI types from `internal/plugins/bgp/nlri/` into 9 self-contained `bgp-nlri-*` plugins (4 tiers), leaving the core `nlri/` as a slim shared-types library with only 4 INET families native.

## Decisions

- `bgp-nlri-*` prefix for all NLRI plugins (including existing evpn, flowspec, vpn, ls) — distinguishes NLRI plugins from behavioral plugins (gr, rib, rr).
- SAFI constants stay in core `nlri/constants.go` — `ParseFamily()` needs them; moving to plugins would create import cycles.
- `EVPNParams` refactored to accept pre-built `NLRI []byte` (matching FlowSpec/MUP pattern) to break the `message/update_build.go` → `bgp-nlri-evpn` import.
- `nlri/ipvpn.go` renamed to `nlri/rd.go` (not deleted) — contained `RouteDistinguisher` types shared by VPN/EVPN/MVPN/MUP; only the `IPVPN` NLRI type was removed.
- `InProcessRouteEncoder` added to registry, `RouteEncoderByFamily()` replaces hardcoded switch in `encode.go`.
- `encode.go` shrunk from 1148 to 253 lines via registry dispatch.

## Patterns

- Pre-built NLRI bytes: build family-specific NLRI bytes before passing to `UpdateBuilder`; decouples message building from NLRI construction.
- Registry dispatch: `registry.RouteEncoderByFamily(family)` eliminates need for infrastructure to import any plugin package.
- Wire consistency test as external package (`package bgp_nlri_labeled_test`): cleanest way to test cross-package wire format agreement.

## Gotchas

- Import cycle in message test files: after moving encode functions to plugins, `message` tests that imported plugin packages created cycles. Fixed with inline NLRI byte construction helpers and external test packages.
- `const label = 100; byte(label<<4)` overflows at compile time — use `label := uint32(100)` (runtime variable).
- `ExtractAttrsFromWire` initially implemented as 7 separate `wire.Get()` calls (each scanning the index) — rewrote to single `wire.All()` + type switch.

## Files

- `internal/plugins/bgp/nlri/` — slim library: `nlri.go`, `inet.go`, `base.go`, `iterator.go`, `wire.go`, `rd.go`, `constants.go`, `helpers.go`
- `internal/plugins/bgp-nlri-evpn/`, `bgp-nlri-flowspec/`, `bgp-nlri-vpn/`, `bgp-nlri-ls/` — renamed from old names
- `internal/plugins/bgp-nlri-labeled/`, `bgp-nlri-mvpn/`, `bgp-nlri-vpls/`, `bgp-nlri-rtc/`, `bgp-nlri-mup/` — 5 new plugins
- `internal/plugins/bgp/message/common_attrs.go` — exported `CommonAttrs` + `ExtractAttrsFromWire`
- `cmd/ze/bgp/encode.go` — 253 lines, registry dispatch, zero `bgp-nlri-*` imports
