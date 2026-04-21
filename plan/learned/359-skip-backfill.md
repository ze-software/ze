# 359 — Skip-and-Backfill Encoding Pattern

## Objective

Audit all Len()-then-WriteTo() call sites, enforce skip-and-backfill as the canonical encoding pattern in hot paths, and expand the encoding allocation hook to prevent regressions.

## Decisions

- Only 1 site needed fixing: `update_split.go` double Len() — added `WriteAttrToWithLen()` variant that accepts pre-computed length
- 13 Len()-then-WriteTo() sites in warm/cold paths left unchanged — acceptable in RIB dedup, formatting, OPEN building, and cached paths
- Hook check uses negative filtering: flags `.Len()` in hot-path files, but only blocks when `WriteAttrTo()` (not `WriteAttrToWithLen` or `WriteAttrToWithContext`) is also present
- `CheckedWriteTo()` pattern explicitly excluded from hook — Len() for capacity guard is a valid safety pattern, not double traversal

## Patterns

- Skip-and-backfill: write fixed bytes → skip length fields (save position) → write payload forward → backfill lengths at saved positions. Avoids Len()-then-WriteTo() double traversal.
- `WriteAttrToWithLen(attr, buf, off, valueLen)` for callers that already know the length — eliminates redundant Len() in WriteAttrTo's header-size decision

## Gotchas

- The hook scope (`update_build*`, `message/pack*`, `reactor_wire*`) deliberately excludes plugin encode.go and CLI decode paths — those are called at human speed where pool allocation is wrong
- `AttributesSize()` summing Len() in attribute.go is kept — it's used for buffer pre-allocation in paths needing known sizes upfront

## Files

- `internal/component/bgp/attribute/attribute.go` — `WriteAttrToWithLen()` function
- `internal/component/bgp/message/update_split.go` — uses pre-computed length instead of double Len()
- `internal/component/bgp/reactor/reactor_wire.go` — skip-and-backfill pattern documentation comment
- `ai/rules/buffer-first.md` — documents pattern, bans Len-then-WriteTo in hot paths
- `.claude/hooks/block-encoding-alloc.sh` — check 6: Len()-then-WriteTo() detection
