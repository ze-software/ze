# 860: Generic ze:required Enforcement and Bare-Form Migration

## Context

`ze:required` was designed as a list-level extension with a path argument (510), enforced
only for BGP peers via the hardcoded `CheckRequiredFields`. Non-BGP lists with `ze:required`
were enforced only by the web add form. A second form, bare `ze:required;` on ipsec/pki/l2tp
leaves (13 sites), was silently dropped at parse time and enforced nowhere.

## What Changed

Two coordinated changes:

**Part A: bare form migrated to `mandatory true`.** All 13 bare `ze:required;` annotations
(ipsec 10, pki 2, l2tp 1) replaced with YANG-native `mandatory true`, which is already
enforced at `ze config validate` via the `yangSectionsToValidate` loop. Parse-time rejection
added in `yangToList` and `yangToLeaf` so bare `ze:required;` can never silently no-op again.

**Part B: path form generalized.** `config.CheckRequired` walks the full schema tree, finds
every `ListNode` with `.Required`, descends into the matching config data, and checks each
list entry for the required descendant path. BGP is excluded (handled by
`CheckRequiredFields` after inheritance merge). The walker is wired at `ze config validate`
(`cmd_validate.go`) and editor commit (`cli/validator.go`).

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| `ValidateTreeAllModules` over single-module `ValidateTree` | l2tp spans 5 YANG modules (l2tp-conf, l2tp-auth-conf, etc.) each augmenting `container l2tp`. `ValidateTree` takes one module; scanning all `-conf` modules for the section's top-level container handles multi-module sections generically |
| Walk-time anchor discovery over pre-indexed anchor map | The anchor (the list node carrying `ze:required`) is found by descending the schema tree in lockstep with the data map, building the config path as it goes. Matches the existing `walkNodeRelated` pattern, avoids a second data structure |
| `HasNonBGPRequired` guard before `tree.ToMap()` | `ToMap()` is not free; most configs have no non-BGP `ze:required` paths. The guard avoids the allocation when there is nothing to check |
| Exclude BGP from generic walker via `delete(nonBGPData, "bgp")` | BGP has group-to-peer inheritance (`ResolveBGPTree`); the generic walker operates on unresolved `tree.ToMap()` data. BGP keeps its existing `CheckRequiredFields` path which runs post-inheritance |
| Bare `ze:required;` rejected at schema parse, not silently dropped | Project rule: "no silent ignore." Bare form is off-design (510: "a leaf cannot know if it's at group level or peer level"). Migration to `mandatory true` is the correct fix; rejection prevents reintroduction |

## Consequences

- Any list in any YANG module can declare `ze:required "path/to/field"` and get enforcement at validate, editor, and startup with no Go code changes (the original 510 design intent, now delivered).
- `ze config validate` enforces `mandatory true` for vpn, pki, and l2tp sections (added to `yangSectionsToValidate` alongside the switch to `ValidateTreeAllModules`).
- Editor validation no longer returns early when BGP is absent, so non-BGP configs get YANG validation.

## Gotchas

- `ValidateTree` assumes a 1:1 section-to-module mapping (via `MapPrefixToModule`). Sections like l2tp that span multiple modules need `ValidateTreeAllModules`, which scans all loaded `-conf` modules for the section's top-level container name.
- The generic `CheckRequired` operates on `tree.ToMap()` (pre-inheritance). This is correct for non-BGP sections (no inheritance), but would produce false positives for BGP where group defaults satisfy requirements. The BGP exclusion is load-bearing.
- `sortedMapKeys` already existed in `serialize_blame.go` with a different type signature (`map[string]*MetaTree`). The generic version needed a distinct name (`sortedAnyMapKeys`).

## Files

- `internal/component/config/required.go` (NEW) -- generic `CheckRequired`, `HasNonBGPRequired`, anchor-scoped walker
- `internal/component/config/required_test.go` (NEW) -- 6 unit tests
- `internal/component/config/yang_schema.go` -- bare `ze:required;` rejection in `yangToList` and `yangToLeaf`
- `internal/component/config/yang_schema_test.go` -- `TestBareRequiredRejectedAtLoad`
- `internal/component/config/yang/validator.go` -- `ValidateTreeAllModules`
- `internal/component/config/cli/cmd_validate.go` -- vpn/pki/l2tp in `yangSectionsToValidate`, switched to `ValidateTreeAllModules`, generic required check
- `internal/component/cli/validator.go` -- removed BGP-absent early return, generic required check
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` -- 10x `ze:required;` -> `mandatory true`
- `internal/component/pki/yang/ze-pki-conf.yang` -- 2x `ze:required;` -> `mandatory true`
- `internal/component/l2tp/plugins/auth_radius/yang/ze-l2tp-auth-radius-conf.yang` -- 1x `ze:required;` -> `mandatory true`
- `docs/features/configuration.md`, `docs/architecture/config/syntax.md` -- reconciled ze:required description
