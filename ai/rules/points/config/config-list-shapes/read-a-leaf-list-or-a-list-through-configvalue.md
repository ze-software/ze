---
kind: directive
level: MUST
stage:
---
**A YANG `leaf-list` MUST be read with `configvalue.LeafList`, and a YANG `list`
with `configvalue.ListEntries` (`internal/core/configvalue`). A slice MUST NOT
be asserted on either.** `m["k"].([]any)` succeeds on a leaf-list at two members
and fails at one, so the option works when the operator writes two values and
vanishes when they write one. On a list it fails at every count, so the option
never applies at all.

**A local helper that coerces a leaf-list or a list to STRINGS or to entries
MUST NOT be written.** One helper per package is how four readers came to handle
three different subsets of the shapes above, each a wrong model for the next
reader to copy.

**A coercion whose RESULT TYPE `configvalue` does not produce MAY stay local, and
it MUST accept all three leaf-list shapes.** Two exist and both are named here,
so a reader can tell a permitted local coercion from a rediscovered copy.

| Local coercion | Why it cannot delegate |
|----------------|------------------------|
| `configInstanceIDs` (`internal/plugins/ospf/config.go`) | yields `[]uint8`, and `LeafList` yields strings |
| `keyedList` (`internal/plugins/isis/config.go`) | sorts a list numerically for `key-id`, an order `ListEntries` does not offer |

**A list declared `ordered-by user` MUST be read with `configorder.Entries`
(`internal/core/configorder`), and MUST NOT be read through `ListEntries`.**
`ListEntries` sorts by key, so for first-match-wins semantics it substitutes a
lexical order for the configured one, which turns a loud refusal into a silently
wrong evaluation order. `serverEntries`
(`internal/component/l2tp/plugins/authradius/config.go`) was that failure in the
tree until 2026-08-23: it sorted an `ordered-by user` `list server` by name for
what its comment called deterministic failover ordering, so RADIUS failover ran
alphabetically.

| Reader | List kind | What it returns |
|--------|-----------|-----------------|
| `configvalue.ListEntries` | a list whose evaluation does not depend on order | entries sorted by key |
| `configorder.Entries` | a list declared `ordered-by user` | entries in the operator's order, or an error |

**The order is carried by the LOWERING, so a plugin's config MUST be lowered
with `Tree.ToPluginMap` and MUST NOT be lowered with `Tree.ToMap`.**
`ToPluginMap` emits the entry order of every list holding two or more entries,
beside the list, under `configorder.OrderKey(listName)`. `ToMap` stays the
general-purpose lowering and carries no order, because gNMI, `ze config show`,
the web config handler, the support bundle and `ValidateTreeAllModules` all read
its output, and every key in it MUST be a name the YANG schema declares.

**A multi-entry ordered list delivered with NO order MUST be refused, and a
reader MUST NOT sort as a fallback.** `configorder.Entries` makes that refusal
for every caller, so no reader spells it. One entry needs no order and is
answered without the key, which is why the key stays out of the payload for the
common case.
