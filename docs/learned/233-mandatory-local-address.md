# 233 — Mandatory Local Address

## Objective
Make `local-address` a required field for all BGP peers so TCP source IP selection is always explicit rather than OS-dependent.

## Decisions
- `local-address auto;` remains valid — it is explicit acknowledgment of OS-selected binding, not a silent default.
- Validation in Go (`reactor/config.go`), not pure YANG: `mandatory true;` in YANG only checks presence, but the Go layer can also enforce format and emit a clear error message with the peer address.
- Error message includes the remedy: `peer <addr>: local-address is required (use IP address or "auto")`.

## Patterns
- Follow the `peer-as mandatory true;` YANG pattern for simple presence enforcement.
- When adding a mandatory field: search all test configs and fix them — new validation fires before capability-specific checks, so tests that previously reached capability validation will now fail earlier.

## Gotchas
- Actual peer parsing is in `reactor/config.go` (`parsePeerFromTree`), not `config/bgp.go`. The spec listed the wrong file; verify the call site with grep before editing.
- Two existing functional tests (`graceful-restart-no-process.ci`, `route-refresh-no-process.ci`) broke because they lacked `local-address` — mandatory validation fires first. Fix by adding the field to their test configs.

## Files
- `internal/component/bgp/reactor/config.go` — mandatory check in `parsePeerFromTree()`
- `internal/component/bgp/schema/ze-bgp-conf.yang` — `mandatory true;` on local-address leaf
- `test/parse/missing-local-address.ci` — functional rejection test
