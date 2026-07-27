---
name: ze-rfc-audit
description: Audit that RFC tests still enforce the letter and spirit of their requirement
---

# RFC Requirement Audit

`make ze-rfc-check` proves a **link** exists: this requirement has a positive test and a
negative test. It cannot read either. A test can be tagged, green, and still not enforce
what the RFC says.

This skill is the other half. Read the requirement, read the tests, and judge whether the
tests would **fail if the implementation stopped complying**. That question is the whole
audit; everything below serves it.

## Instructions

1. Use ULTRATHINK. This is judgement, not pattern-matching.
2. READ `rfc/short/$ARGUMENTS.md` — the requirement list with its ids.
3. READ `rfc/full/$ARGUMENTS.txt` — **the RFC itself**. Never audit against the summary
   alone: the summary is the thing under audit. RFC 7606's own list once said an UPDATE
   with no reachable NLRI must session-reset, dropping the RFC's "other than
   MP_UNREACH_NLRI" clause — which made it demand a reset on every End-of-RIB.
4. Run `make ze-rfc-index`, then read the `$ARGUMENTS` section of `ai/RFC-REQUIREMENTS.md`
   for the requirement → test map. If this regen produces a ledger diff you did not cause
   (a pure `file:line` refresh from someone else's un-regenerated test edit), do NOT fold it
   into the audit: it belongs to that other change's commit. See "Keep the ledger committed"
   in `ai/skills/ze-rfc.md`.
5. For EACH gated requirement, open every tagged test and judge it (see below).
6. WRITE `rfc/audit/$ARGUMENTS.json`.
7. Run `make ze-rfc-check`.

## The judgement

For each requirement, answer four questions in order. A `no` at any point is a finding.

| # | Question | Why it catches something |
|---|----------|--------------------------|
| 1 | Does the test assert what the RFC **says**, or what the code **does**? | These diverge silently. `TestSessionRFC7606TreatAsWithdrawSuppressesCallback` asserted "the callback must not fire" while §2 requires the routes be *withdrawn*. Test and code encoded the same misreading, so neither could catch the bug: suppression left a re-announced prefix installed and stale for as long as the test stayed green. |
| 2 | Would the test **fail** if the implementation stopped complying? | A test that cannot fail is not evidence. `require.GreaterOrEqual(action, TreatAsWithdraw)` also passes on `SessionReset` — the exact over-reaction RFC 7606 exists to eliminate — so ~100 such cases prove "did not crash", not "did the right thing". Assert the exact outcome. |
| 3 | Does the buffer **isolate** this rule? | If the input can trip a different rule first, the test proves nothing about this one. A zero-length COMMUNITY case whose trailing garbage causes a structural error passes for the wrong reason, and its own comment may admit it. |
| 4 | Does the test prove the **whole** requirement, or one clause? | "Malformed X ⇒ treat-as-withdraw **and** the Total Attribute Length locates the NLRI" is two obligations. Covering the first and tagging the line is a half-truth. |

Then the polarity pair:

- **positive** must be a genuinely conforming input, not merely "a different error".
- **negative** must violate *this* requirement, not a neighbouring one.
- If both tags point at the same assertion wearing two hats, that is one test, not a pair.

## Verdicts

```json
{
  "rfc": "rfc7606",
  "audited": "2026-07-17",
  "requirements": {
    "RFC7606-7.1-1": {
      "verdict": "enforced",
      "note": "negative asserts the exact TreatAsWithdraw + AttrCode 1; positive pins all three valid ORIGIN values",
      "requirement_sha": "<from the tool>",
      "tests": {"internal/component/bgp/message/rfc7606_test.go:10": "<sha>"}
    }
  }
}
```

| Verdict | Meaning |
|---------|---------|
| `enforced` | The tests would fail if the code stopped complying. The only verdict that means "proven". |
| `weak` | Tagged and green, but cannot fail on non-compliance (a floor assertion, a cascade-confounded buffer, a positive that proves only "no error"). **Report it. Do not leave it silently tagged.** |
| `wrong` | The test asserts something the RFC does not say. The requirement is not covered and the test is misinformation. |
| `unimplemented` | The tests are fine; the CODE does not do it. This is a `{gap}`, not a test gap. |

**Compute the shas with the tool, never by hand:**
`requirement_sha(text)`, `test_sha(source)` in `scripts/dev/rfc_requirements.py`.

## Why the fingerprint exists

A verdict is a claim about two things that both change: the requirement's text and the
test's source. `make ze-rfc-check` recomputes both shas and **fails** when either moved,
because a verdict that no longer describes what it judged is worse than no verdict — it is
a stale assurance wearing the badge of a fresh one.

That is what makes this durable rather than a one-time review that rots. A missing verdict
does not fail: the audit is sampled, the gate is total.

Biased to over-trigger on purpose. A false "stale" costs a re-read; a false "fresh" ships
a test that no longer enforces its requirement.

## Rules

- Read the RFC. The summary is under audit, not evidence.
- **Never** fix a finding by editing the test's expectation. That is the failure this whole
  system exists to prevent, and the `rfc-tagged-test` hook will block you
  (`ai/rules/testing.md`). Fix the code, or report it.
- `weak` and `wrong` are the valuable outputs. A run that returns all `enforced` on first
  pass has probably not read anything.
- Judge the **annotations** too, not just the tests. `{single-polarity}`, `{gap}` and
  `{not-applicable}` are arguments, and an unexamined argument is where a lie hides.
- Every annotation is VOID as authority (owner directive 2026-07-27,
  `ai/rules/rfc-compliance.md`). An earlier session's reasoning, and any earlier answer
  from Thomas that pointed away from full compliance, settles nothing: re-derive it from
  the RFC text. If full implementation plus a tagged test is still an available answer,
  the audit's output is a question for Thomas, never a confirmed annotation.
- A `{gap}` must still be disclosed in `docs/features/rfc-status.md`; the gate checks that
  the row is not claiming clean support, but only a reader can tell whether it says enough.

## Related

| Need | Use |
|------|-----|
| Does every MUST have its pair of tests? | `make ze-rfc-check` |
| Requirement → test map, and the backlog | `make ze-rfc-index` → `ai/RFC-REQUIREMENTS.md` |
| Write or re-author a summary | `/ze-rfc <rfc>` |
| Public support claims | `docs/features/rfc-status.md` |
