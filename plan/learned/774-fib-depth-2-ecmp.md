# 774 -- fib-depth-2-ecmp

## Context

Ze has two ECMP mechanisms: (1) bgp-rib multipath, which selects equal-cost siblings from different BGP peers for the same prefix, and (2) sysrib cross-protocol ECMP via ecmpCollect, which groups routes from different protocol sources. The unit tests for ecmpCollect existed, but no functional test proved multipath works end-to-end through the daemon, and no interop test proved Ze's wire encoding is ECMP-compatible with FRR.

## Decisions

- Tested bgp-rib multipath (via bgp rib inject + bgp rib show best) over sysrib ecmpCollect, because the locrib path means sysrib only sees one "bgp" protocol entry per prefix, making cross-protocol ECMP untestable with current BGP-only sources
- Used GoBGP as the second ECMP source in the FRR interop test over a second Ze instance, because the interop framework already supports GoBGP containers
- Added ecmp-paths field to showRIB() output (omitempty) over leaving it opaque, for future cross-protocol ECMP observability

## Consequences

- Functional test proves add/withdraw/grow lifecycle for bgp-rib multipath, catching regressions in SelectMultipath or best-path event propagation
- When a second protocol source starts feeding the locrib (static routes, connected), sysrib ecmpCollect will populate ecmp-paths in rib show output without further code changes
- The interop scenario 34-ecmp-frr proves Ze's UPDATEs are wire-compatible with FRR for ECMP selection (maximum-paths 2 with as-path multipath-relax)

## Gotchas

- sysrib ecmpCollect is keyed by protocol name; all BGP routes are stored under "bgp" via the locrib path, so BGP-only ECMP never triggers ecmpCollect. The spec's data flow originally described this incorrectly.
- The bgp rib withdraw command is a separate dispatch (bgp rib withdraw <peer> <family> <prefix>), not a keyword on bgp rib inject.

## Files

- `internal/component/sysrib/sysrib.go` -- added ecmp-paths to showRIB()
- `test/plugin/fib-ecmp.ci` -- functional test (AC-1, AC-2, AC-3)
- `test/interop/scenarios/34-ecmp-frr/` -- interop test (AC-4): ze.conf, frr.conf, gobgp.toml, announce.py, check.py
