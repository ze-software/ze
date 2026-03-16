# 273 — Code Hygiene Fixes

## Objective

Fix plugin import violations, MUP helper duplication, and a naming collision in infrastructure code before a planned file-split restructuring, then fix a golangci-lint v1→v2 config migration that had silently disabled all linter settings.

## Decisions

- MUP helpers (`writeMUPPrefix`, `mupPrefixLen`, `teidFieldLen`) moved to `bgp-nlri-mup/helpers.go` as exported symbols — the MUP plugin package is the natural owner of MUP-specific utilities.
- `config.splitPrefix` renamed to `expandPrefix` (not consolidated with `route.splitPrefix`) — the two have deliberately different contracts: config version is lenient (returns original on failure), route version is strict (returns error). Different contracts are not duplication.
- Config cannot import plugin packages per plugin-design rule — shared utilities must live in the plugin package, not config.
- `reactor.go` still imports `labeled` and `vpn` plugin packages because `QueueWithdraw` needs typed `nlri.NLRI` objects, not hex strings. Removing requires a future `registry.BuildNLRI(family, args) (nlri.NLRI, error)` API.

## Patterns

- Registry text API (`EncodeNLRIByFamily`) returns hex strings — insufficient for withdrawal functions that need typed `nlri.NLRI` objects. Registry API gap identified; requires future spec.
- golangci-lint v1 used `linters-settings:` key; v2 uses `linters.settings:`. Silently ignoring misconfiguration exposed ~2300+ lint issues across the codebase when fixed.

## Gotchas

- AC grep patterns (`bgp-flowspec`) did not match the actual import path (`bgp-nlri-flowspec`) — grep AC patterns must match actual import strings verbatim.
- `update_build.go` and `encode.go` were assumed to have plugin imports — they were already clean. Spec overestimated scope.
- The golangci-lint v1→v2 migration was not in the original spec; it was discovered during Phase 1 work and required fixing ~150 files to satisfy the newly-functional linter settings.

## Files

- `internal/component/bgp/plugins/nlri/mup/helpers.go` — `WriteMUPPrefix`, `MUPPrefixLen`, `TEIDFieldLen` (created)
- `internal/component/bgp/reactor/reactor.go`, `internal/component/config/loader.go`, `peers.go` — updated
- `.golangci.yml` — v1→v2 migration (`linters-settings:` → `linters.settings:`)
- 150+ files — gosec G115, gocritic, gofmt, octal literal, errcheck fixes
