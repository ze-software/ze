# Exact Or Reject

**BLOCKING:** If the implementation cannot deliver EXACTLY what the
operator's config asks for, `ze config verify` / `ze config commit`
MUST fail with a clear error. Silent approximation, truncation, or
"best-effort" mapping are bugs.

Rationale: `.claude/rationale/exact-or-reject.md`

## In Practice

| Situation | Wrong (silent) | Right (reject at verify) |
|-----------|----------------|--------------------------|
| Qdisc backend cannot reproduce | Map to closest supported | `qdisc <type>: not supported by backend <name>` |
| Filter type backend cannot match | Skip that filter | `filter <type>: not supported by backend <name>` |
| Backend has fewer slots than classes configured | Discard extras | Error naming capacity + actual count |
| Backend maps N inputs to same output (name truncation) | Second overwrites first | `<name> exceeds <limit>-char limit; shorten or rename` |
| Numeric overflow at backend's wire format | Truncate/wrap | `<value> out of range <lo..hi>` |
| Rate/burst/DSCP outside representable range | Silently clamp | Reject with valid range in message |

## Checklist For Every Backend

```
[ ] Every accepted config path produces backend state matching EXACTLY. No approximation
[ ] Every capacity/limit/bound is checked in the verifier BEFORE Apply time
[ ] Every narrowing numeric input has an explicit range check naming the valid range
[ ] Every name subject to truncation rejects when it would truncate (distinct inputs != same stored name)
[ ] Not-yet-implemented feature rejects with "deferred" message, not quiet ignore
```

## Banned Phrases In Code Comments

| Banned | Usually means |
|--------|---------------|
| "for now we just truncate" | Silent data loss; reject at verify |
| "close enough approximation" | Not the operator's config; reject |
| "MVP only handles the first N" | Classes beyond N silently missing; reject |
| "best-effort translation" | Pick one: exact, or reject |
| "future optimization can batch them" (when un-batched path is wrong) | Fix correctness first |

Caught yourself writing one? Stop. Design it properly, or reject in
the verifier and record in `plan/deferrals.md`.

## Related Rules

- `integration-completeness.md` -- silent wiring gaps. This rule is
  the backend-translation specialization: wired but lossy = not done.
- `anti-rationalization.md` -- "best effort" = "probably fine".
- `deferral-tracking.md` -- deferred features reject at commit AND
  are recorded with a destination spec.

## Mechanical Check

Before marking any backend/translator spec done, for every path that
accepts config and writes state:

1. Lossy field -> pre-check rejects in verifier?
2. Bounded output structure -> capacity check rejects when exceeded?
3. Truncated name -> length check rejects before truncation?
4. Numeric narrowing -> explicit range check with valid range in error?

One "no" = operator intent silently discarded. Fix before commit.
