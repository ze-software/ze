# 680 -- config-4-container-inactive

## Context

Ze's config deactivation feature had an asymmetry: leaf-level inactive
state was stored in `Tree.inactiveValues` (a map on the parent), while
container/list-entry inactive state was stored as an auto-injected
`inactive` boolean leaf in the YANG schema. The injected leaf polluted
autocomplete, required skip guards in every serializer, and made the
schema lie about what fields a container actually has.

The spec-config-3-deactivate learned summary (654) noted this asymmetry
and deferred unification.

## Decisions

- **Bool field on `*Tree`** for container/list-entry inactive state. Containers
  and list entries ARE Trees, so they carry their own flag. Leaves remain on the
  parent's `inactiveValues` map (they have no Tree of their own). This creates a
  justified asymmetry: leaves use parent-side state, structural nodes use self-side
  state, matching how the data model stores them.
- **Delete `isInactiveTree` helper**, replace all 14 call sites with `.IsInactive()`.
  Direct method call on the node is clearer than a package-level function.
- **Delete YANG injection entirely**: `inactiveLeaf()`, both injection sites
  (containers and structural lists), and the `hasStructuralChildren` positional-list
  guard. Exported the guard as `(*ListNode).HasStructuralChildren()` since
  `cmd_deactivate.go` needs it for positional-list rejection.
- **On-disk format unchanged**: `inactive:` prefix syntax, `inactive <path>` set-format
  keyword. No migration needed. The storage mechanism is internal-only.
- **No backward-compat concern**: the injected leaf only existed in memory. Config
  files never contained `inactive true` as a child value.

## Consequences

- YANG schema no longer advertises `inactive` as a valid child of every container
  and list. Autocomplete is clean.
- All serializer skip guards (`if name == InactiveLeafName { continue }`) removed.
  Serializers are simpler.
- `canInlineContainer` no longer needs to subtract the injected leaf from the
  value count.
- `TreeEqual` compares the bool field. Clone copies it.
- `emitSetInactiveStructural` uses `.IsInactive()` instead of reading a value from
  the tree's value map.
- Net: 94 lines added, 198 removed.

## Gotchas

- `serializeContainerInline` holds `child.mu.RLock()` already, so it accesses
  `child.inactive` directly (not via `IsInactive()` which would double-lock).
- `mergeFrom` does not propagate the `inactive` bool. This is correct: the parser
  sets the flag AFTER merge via `applyInactive`, so the merged result gets the
  flag from the last declaration. This matches the old behavior where `mergeFrom`
  would not overwrite existing values.
- Tests that used `inactive enable` as a literal child value needed rewriting to
  use the `inactive:` prefix syntax. The old form only worked because the YANG
  injection made "inactive" a valid field.

## Files

- `internal/component/config/tree.go` -- new `inactive bool`, `SetInactive`, `IsInactive`, Clone copies it
- `internal/component/config/yang_schema.go` -- deleted `inactiveLeaf()`, both injection sites; exported `HasStructuralChildren` as method on `*ListNode`
- `internal/component/config/serialize.go` -- deleted `isInactiveTree`; removed skip guards; `canInlineContainer` simplified; `serializeContainerInline` checks `child.inactive`
- `internal/component/config/serialize_annotated.go`, `serialize_blame.go` -- removed skip guards, `.IsInactive()` calls
- `internal/component/config/serialize_set.go` -- `emitSetInactiveStructural` uses `.IsInactive()`; removed skip guards
- `internal/component/config/serialize_test.go` -- `TreeEqual` compares `inactive` field
- `internal/component/config/parser.go` -- `applyInactive` calls `SetInactive(true)` for containers/list-entries
- `internal/component/config/parser_freeform.go`, `parser_list.go` -- `.IsInactive()` for duplicate-key checks
- `internal/component/config/setparser.go` -- `walkAndMarkInactive` calls `SetInactive(true)`
- `internal/component/config/prune.go` -- `.IsInactive()` replaces `isInactiveTree`
- `internal/component/cli/editor_commands.go` -- `DeactivatePath`/`ActivatePath` use `SetInactive`/`IsInactive`
- `internal/component/config/cli/cmd_deactivate.go` -- positional-list check uses `HasStructuralChildren()`
