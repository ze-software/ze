# 003 — Pool Completion

## Objective

Implement the pool package for zero-copy byte deduplication of BGP attributes and NLRI. Prerequisite blocking all wire format work.

## Decisions

Mechanical implementation following pre-existing architecture document (`pool-architecture.md`). No design decisions in this spec.

## Patterns

None beyond what the architecture doc covers.

## Gotchas

None.

## Files

- `internal/pool/` — handle, pool, compaction, scheduler, debug, metrics, benchmarks
