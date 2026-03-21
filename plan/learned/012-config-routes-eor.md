# 012 — Config Routes and End-of-RIB

## Objective

Design the implicit commit for config routes at session establishment (group routes by attributes, send grouped UPDATEs, then send EOR for each negotiated family). This spec was superseded by spec 013 before implementation.

## Decisions

Mechanical refactor, no design decisions — the spec was superseded before code was written.

## Patterns

- EOR for IPv4 unicast = empty UPDATE (Withdrawn=0, PathAttr=0, NLRI=empty) per RFC 4724.
- EOR for other families = UPDATE with MP_UNREACH_NLRI (AFI/SAFI, no withdrawn NLRIs).
- `commit end` sends EOR only for families that had routes in that commit.

## Gotchas

- This spec was superseded by `013-unified-commit-system.md` which unified config and API commit paths into a single `CommitService`. The EOR wire format and detection logic documented here are correct and were carried forward.

## Files

Design spec only — implementation landed in spec 013.
