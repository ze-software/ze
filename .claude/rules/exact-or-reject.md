# Exact Or Reject

**BLOCKING:** If the implementation cannot deliver EXACTLY what the
operator's config asks for, `ze config verify` / `ze config commit` MUST
fail with a clear error. Silent approximation, silent truncation, and
"best-effort" mapping that drops information are bugs.

Rationale: `.claude/rationale/exact-or-reject.md`

## What This Means In Practice

| Situation | Wrong (silent) | Right (reject at verify) |
|-----------|----------------|--------------------------|
| Config references a qdisc the backend cannot reproduce | Map it to the closest supported qdisc and apply | Verifier returns `qdisc <type>: not supported by backend <name>` |
| Config references a filter type the backend cannot match | Skip programming that filter; apply the rest | Verifier returns `filter <type>: not supported by backend <name>` |
| Backend's data structure has fewer slots than classes configured | Discard classes beyond the slot count | Return error naming the capacity and the actual count |
| Backend maps N inputs to the same output due to truncation / name length | Allow the second one to overwrite the first | Verifier returns `<name> exceeds <limit>-char limit; shorten or rename` |
| Numeric overflow at the backend's wire format | Truncate / wrap | Verifier returns `<value> out of range <lo..hi>` |
| Rate / burst / DSCP value outside the backend's representable range | Silently clamp | Reject with the valid range in the message |

## Checklist For Every Backend

```
[ ] Every config path the verifier accepts produces a backend state that
    matches the config EXACTLY. No approximation, no nearest-equivalent.
[ ] Every capacity/limit/bound the backend imposes is checked in the
    verifier BEFORE the config lands, not at Apply time.
[ ] Every numeric input that passes through a narrower type has an
    explicit range check with a message naming the valid range.
[ ] Every name/identifier subject to truncation is rejected when it would
    truncate (two distinct inputs MUST NOT produce the same stored name).
[ ] When a backend feature is not yet implemented but might exist in
    future, the verifier rejects with a clear "deferred" message -- not
    "accepted and quietly ignored."
```

## Banned Phrases In Code Comments

These almost always mean the code is silently wrong:

| Banned | Usually means |
|--------|---------------|
| "for now we just truncate" | Silent data loss; should reject at verify |
| "close enough approximation" | Not the operator's config; reject |
| "MVP only handles the first N" | Classes beyond N silently missing; reject |
| "best-effort translation" | Pick one: exact, or reject |
| "future optimization can batch them" when the un-batched path is wrong | Fix correctness first |

If you catch yourself writing one of these, stop. Either design the
feature properly and support it, or reject it in the verifier and
record the deferral in `plan/deferrals.md`.

## Relationship To Other Rules

- `integration-completeness.md` -- silent wiring gaps. This rule is the
  specialization for backend-level translation: even if the wiring is in
  place, if the translation is lossy, the feature is not done.
- `anti-rationalization.md` -- "good enough" excuses. "Best effort" is
  the backend equivalent of "probably fine".
- `deferral-tracking.md` -- when a feature is deferred, the verifier
  MUST reject at commit time AND the deferral MUST be recorded with a
  destination spec. The operator gets a clear error; the maintainer
  knows where the work is going.

## Mechanical Check

Before marking any backend or translator spec as done, for every code path
that accepts a config and writes state elsewhere:

1. Does every lossy field have a pre-check that rejects in the verifier?
2. Does every bounded-size output structure have a capacity check that
   rejects when exceeded?
3. Does every name that gets truncated somewhere have a length check that
   rejects before the truncation site?
4. Does every numeric narrowing have an explicit range check with the
   valid range in the error message?

One "no" to any of the above means the implementation silently discards
operator intent and must be fixed before commit.
