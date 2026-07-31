# Diagnosis Before Fix

**When:** Before changing code to make a symptom go away (failing test, rejected input, error, red gate, broken demo), write the Diagnosis first
**Severity:** blocking

## Directives

Before changing code to make a symptom go away (failing test, rejected input, error, red gate, broken demo), write the Diagnosis first. Editing to silence the symptom before the root cause is named is the defect, not the fix.

## Why

The recurring failure is jumping from symptom to the nearest edit that silences it: rename the command so it stops being rejected, skip or relax the test so it stops failing, special-case the one input that breaks. That fixes where the problem *shows up*, not where it *is*. The cure is to change the success criterion from "symptom gone" to "root cause named and fixed at the owning layer", and to produce the diagnosis BEFORE touching code.

## The Diagnosis (write all five before any edit)

1. **Symptom** — the exact failure, verbatim (error text, rejected input, failing assertion).
2. **Root cause** — traced to the exact function where behavior diverges from intent, named as the file plus the symbol. Read the path; do not guess. If you cannot name it, you have not diagnosed it yet.
3. **Owning layer** — which layer/component owns the correct fix.
4. **Two candidate fixes, labeled** — at least one `[workaround]` and one `[source]`. Name what each changes and what each leaves broken for the next caller.
5. **Why not the workaround** — one sentence on why the local edit is wrong.

Only after the five are written do you implement the `[source]` fix.

## When a check or validation rejects you

Ask the three-way question, not "how do I get past this":

- Is the **check** wrong? (the validation logic is incorrect)
- Is the **input** wrong? (you are doing the wrong thing)
- Is the check's **data/config** incomplete? (the check is right but its allowed-set / table / registry is missing an entry)

The third option is where most "I worked around it" bugs hide. Example: `update bgp irr` rejected because `update` was missing from the registry's allowed-verb set. The verb gate was correct; renaming the command was a workaround. The fix was adding `update` to the registry.

## Altitude

Always ask: am I fixing where the problem **is**, or where it **shows up**? A special case layered on shared infrastructure means the underlying mechanism should be generalized instead. See `ai/rules/no-workarounds-for-missing-behavior.md`.

## Trigger words (stop and write the Diagnosis)

"let me just rename / just skip / just special-case / just adjust the test / add a fallback / quick workaround". The word "just" is the tell. Stop, write the five, then fix the source.

## Related

- `ai/rules/no-workarounds-for-missing-behavior.md` — implement missing behavior at the source.
- `ai/rules/no-test-deletion.md` — a red test means the code is wrong by default; never weaken the test to reach green.
- `ai/rules/anti-rationalization.md` — 3-Fix Rule: after 3 failed attempts, stop and ask.
