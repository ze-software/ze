# 510 -- YANG Required and Suggest Extensions

## Context

BGP peers could be created via web UI or config file without essential fields (remote IP, local AS, remote AS). Missing fields were only discovered at BGP session startup, not at config validation time. There was no YANG-level mechanism to declare "this field must have a value after inheritance merges bgp/group/peer levels." The web creation form showed only unique constraint fields with no guidance about what's actually needed.

## Decisions

- Chose list-level extensions (`ze:required`, `ze:suggest`) over leaf-level, because inheritance context belongs to the list, not the leaf. A leaf cannot know if it's at group level (optional) or peer level (required post-resolve).
- Chose `CheckRequiredFields` as a separate exported function over embedding it in `ResolveBGPTree`, because `ResolveBGPTree` takes only `*config.Tree` (5 callers), and `plugins.go`/`cmd_dump.go`/`cmd_diff.go` don't need field validation. Only `peers.go` and `cmd_validate.go` call it.
- Replaced the ad-hoc `session/asn/remote` check in `validator.go` with a generic loop over `ListNode.Required`, checking both the merged peer tree and the bgp-level tree for inheritance.
- Runtime guards in `reactor/config.go` and `bgp/plugins/cmd/peer/` left unchanged -- they serve as last-resort runtime checks, not config validation.

## Consequences

- `ze config validate` now catches incomplete peers at validation time with clear error messages naming the peer and missing field.
- Web creation form shows required fields (accent color, `*` marker) and suggested fields (muted) with inherited defaults pre-filled from group/bgp level.
- New required fields can be added to any list by adding `ze:required "path/to/field"` in YANG -- no Go code changes needed.
- The `CheckRequiredFields` call in `PeersFromConfigTree` means daemon startup also validates required fields.
- Editor validation produces warnings (not errors) for missing required fields, so editing is never blocked.

## Gotchas

- `connection/remote/ip` is in the shared `peer-fields` grouping (not augmented/peer-only as initially assumed). It IS structurally inheritable from group level, though the `unique` constraint makes that impractical.
- `YANGSchema()` is not cached -- re-parses on every call. Cannot be called from inside `ResolveBGPTree`.
- The validator's `mergeGroupDefaults` is shallow at depth > 2 (pre-existing). The bgpTree fallback in the required check masks this for current fields but could cause false warnings for future deeply-nested group-only required fields.
- `hasNestedValue` treats non-string values as "present" (correct for current pipeline where ToMap produces strings, but would need revisiting if non-string values appear in resolved maps).
- `resolveParentDefaults` must handle the tree's container/list distinction: containers via `GetContainer`, list entries via `GetList` + key lookup.

## Files

- `internal/component/config/yang/modules/ze-extensions.yang` -- extension definitions
- `internal/component/config/schema.go` -- `ListNode.Required`, `ListNode.Suggest`
- `internal/component/config/yang_schema.go` -- extension parsing in `yangToList`
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- `ze:required`/`ze:suggest` on peer lists
- `internal/component/bgp/config/resolve.go` -- `CheckRequiredFields`, `hasNestedValue`
- `internal/component/bgp/config/peers.go` -- wiring into `PeersFromConfigTree`
- `cmd/ze/config/cmd_validate.go` -- wiring into `ze config validate`
- `internal/component/cli/validator.go` -- generic required loop in `validatePeer`
- `internal/component/web/handler_config.go` -- form inheritance resolution, POST enforcement
- `internal/component/web/fragment.go` -- `collectRequiredFields`, `collectSuggestFields`
- `internal/component/web/templates/component/add_form_overlay.html` -- visual grouping
- `internal/component/web/assets/style.css` -- required/suggest field styles
- `test/parse/required-field-{missing,inherited,all-present}.ci` -- functional tests
