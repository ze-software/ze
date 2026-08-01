# 893: config-nop-keyword -- set/nop toggle for config deactivation

## Context

Ze config set-format used a two-line model for deactivation: `set <path> <value>` + `inactive <path>`. Toggling activation required adding/removing a line. The new `nop` keyword replaces this with a single-line toggle where changing bytes 0-2 (`set` <-> `nop`) switches activation state.

## Decisions

- **D-1 Keyword:** `nop` ("no operation") chosen over `off`/`rem`/`nil`/`not`. Precise semantics: the line exists but produces no operational effect.
- **D-2 Leaf:** `nop <path> <value>` replaces `set <path> <value>` + `inactive <path>`.
- **D-3 Leaf-list members:** Individual lines per member. Active use `set`, deactivated use `nop`. Bracket form only when all members are active.
- **D-4 Container/list-entry:** Option A -- structural `nop <path>` marker line before children. Children retain their own `set`/`nop` state independently.
- **Backward compat:** `inactive` keyword still parsed but never emitted. One-way migration on save.

## Consequences

- Set-format files are ~30% smaller for heavily deactivated configs (one line vs two per deactivated leaf).
- Activation toggle is a 3-byte in-place edit with no line insertions/deletions.
- Hierarchical format `inactive:` prefix is unaffected.
- Old configs auto-migrate to `nop` on first save.

## Gotchas

- `parseNop` is a two-step operation: `walkAndSet` followed by `markNopInactive`. The inactive marking must be a separate walk because `walkAndSet` returns void (it doesn't expose which node it terminated at).
- For ValueOrArrayNode with deactivated members, `splitInactiveMembers` remains the internal mechanism (the `inactive:` prefix in the multiValues slice). Only the serialized output changes from `inactive` lines to `nop` lines.
- The meta serializer needed `writeMetaLeafLineCmd` (accepting a `cmd` parameter) to avoid duplicating `writeMetaLeafLine`. The `leafLineWriter` callback type retains the old signature for shared helpers (freeform, flex, inline-list) that can't be deactivated.

## Files

None recorded.
