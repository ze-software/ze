# 014 — Two-Level Route Grouping

## Objective

Fix UPDATE generation so that routes with different AS_PATHs produce separate UPDATE messages, by introducing two-level grouping: first by non-AS_PATH attributes (AttributeGroup), then by AS_PATH within each group (ASPathGroup).

## Decisions

- Two-level structure chosen over storing AS_PATH inside the attribute key, because it enables memory sharing: routes within the same AttributeGroup share the same `[]Attribute` slice reference even though they have different AS_PATHs.
- AS_PATH stored separately in `route.asPath` (not in `route.attributes`), because AS_PATH is modified per-hop (eBGP prepends local AS) while other attributes pass through unchanged.
- `route.asPath` takes precedence over any AS_PATH found in `route.attributes` when building the grouping key.
- Nil AS_PATH = locally originated route (no AS_PATH attribute or empty); kept separate from routes with explicit AS_PATHs.

## Patterns

- Level 1 key: Family + NextHop + Attributes hash (excludes AS_PATH).
- Level 2 key: AS_PATH hash (nil hashes separately from non-nil empty).
- Both levels sorted for deterministic UPDATE ordering.
- eBGP: local AS prepended to AS_PATH at send time; iBGP: AS_PATH preserved as-is.

## Gotchas

- The original `GroupByAttributes()` silently lost AS_PATHs because it searched `group.Attributes` for AS_PATH (wouldn't find it since it's stored separately), then created a fresh empty AS_PATH — causing all routes in a group to share the wrong AS_PATH.
- RFC 4271 §4.3: all NLRIs in one UPDATE share the same path attributes, including AS_PATH — routes with different AS_PATHs cannot be combined.

## Files

- `internal/rib/grouping.go` — AttributeGroup, ASPathGroup, GroupByAttributesTwoLevel()
- `internal/rib/commit.go` — buildGroupedUpdateTwoLevel(), updated Commit() loop
