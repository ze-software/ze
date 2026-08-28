# Spec: unrecognized-field-value-must-fail-loudly

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make every gate that parses a controlled-vocabulary field REFUSE a value it does
not recognize, instead of falling through to a default reading.

**The shape.** A gate reads a field whose values are a closed set. It tests for
the values it knows and treats everything else as one of them, usually the
permissive one. An author who writes a recognized word with an explanation
appended has written an unrecognized value, and the gate acts on the opposite of
what the record says. Nothing reports the mismatch, because from the gate's side
there is no mismatch: it saw a value it did not know and it had a default.

**Why a rule is not enough.** `ai/rules/evidence.md` now carries "Records You
Author", which requires reading a parser before writing the field it reads. That
governs the author. It does nothing for the field already written, and it cannot
be checked by review, because the decorated value and the correct value differ by
punctuation a reader skims past.

**Goal.** A controlled-vocabulary field carries exactly one of its vocabulary's
values, or the gate that reads it fails and names the file, the field, and the
value it could not parse. The failure is the point: an unparseable record must
never be silently sorted into a bucket.

**Scope.** Every gate under `internal/le/` that reads a status, state,
disposition, severity, verdict, or polarity from a Markdown table or a
frontmatter key. The audit is mechanical: find the comparisons against literal
vocabulary words, and check what the surrounding branch does with a value that
matches none of them.

**Not in scope.** Changing any vocabulary, or adding a value to one. This is
about what happens at the edge of a vocabulary, not about its contents.

## Provenance

Written 2026-08-14 at Thomas's instruction, after a session wrote decorated
values into a deferral status column and every gate that read them acted on the
opposite meaning. The root cause was general rather than local: a machine-parsed
field was treated as free-form prose because nothing refused prose. The rule
change covers authors from now on; this covers the fields that already exist and
the authors who will not have read the rule.
