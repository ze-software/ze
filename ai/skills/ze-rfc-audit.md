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
      "tests": {"internal/component/bgp/message/rfc7606_test.go:10": "<whole-file sha>"},
      "units": {"internal/component/bgp/message/rfc7606_test.go:10": "<enclosing-func sha>"}
    }
  }
}
```

| Verdict | Meaning |
|---------|---------|
| `enforced` | The tests would fail if the code stopped complying. The only verdict that means "proven". Requires a non-empty `tests` map AND both polarities (or a `{single-polarity}` annotation). |
| `weak` | Tagged and green, but cannot fail on non-compliance (a floor assertion, a cascade-confounded buffer, a positive that proves only "no error"). **Report it. Do not leave it silently tagged.** |
| `wrong` | The test asserts something the RFC does not say. The requirement is not covered and the test is misinformation. |
| `unimplemented` | The tests are fine; the CODE does not do it. This is a `{gap}`, not a test gap. Requires a `code` map and a `{gap}`/`{not-applicable}` annotation. |
| `not-applicable` | No reachable code path can satisfy or violate the requirement — it binds a document's authors, another role, or a layer Ze does not implement. Requires NO cited test (`tests` empty or omitted, either is the same state), a `no_code_path` reason in prose, and a `{not-applicable}` annotation on the checklist line. |

**The five values are a closed enum and `make ze-rfc-check` enforces it.** A sixth word is a
parse error, not a novel verdict. `implemented` sat in `rfc/audit/rfc7606.json` for weeks, and no
code read the field at all.

**`not-applicable` is not a shortcut past `enforced`.** It exists because an obligation on
*future specification authors* (RFC 7606 §8) can be neither satisfied nor violated by code that
runs. A test for it is therefore a contradiction. It costs two committed facts — the reason on
the record and the agreeing annotation — precisely so it stays more expensive than the honest
alternatives.

And per `ai/rules/rfc-compliance.md` the annotation it agrees with is itself VOID as authority.
This verdict says what the CODE can do. It never says that the classification is settled.

| Field | When | What it is |
|-------|------|-----------|
| `requirement_sha` | always | `requirement_sha(text)` of the checklist line |
| `tests` | one entry per tagged test, and empty or omitted on `not-applicable`, which cites none | `{file:line -> whole-file sha}` for each tagged test |
| `units` | one entry per `tests` entry, so empty or omitted whenever `tests` is | `{file:line -> enclosing-unit sha}`. The unit is one top-level Go function (doc comment through closing brace) or the whole file for a `.ci`, a `.et` or an interop `check.py` |
| `code` | `unimplemented` | `{file:line -> enclosing-unit sha}` of the PRODUCING code the note names. Without it the verdict can never go stale |
| `no_code_path` | `not-applicable` | prose stating why no reachable path exists |
| `upgrade_reason` | changing a `weak`/`wrong` verdict to `enforced` with no unit change | what you re-read and why the earlier judgement was wrong |

**Compute the shas with the tool, never by hand:**
`requirement_sha(text)`, `test_sha(source)`, `tagged_unit_shas(tags)` (the `tests` map) and
`unit_shas(keys)` (the `units` and `code` maps) in `scripts/dev/rfc_requirements.py`.

## Recording a finding never fails the build

`weak` is green. `wrong` and `unimplemented` are green too, provided the RFC's row in
`docs/features/rfc-status.md` already admits the RFC is not fully met. The red falls on the
public CLAIM, not on your honesty.

What IS red: deleting a `weak` or `wrong` verdict, upgrading one to `enforced` with nothing
changed, and removing any verdict that existed at HEAD. Audit coverage is monotonic per
requirement id, so a judgement that has been made cannot be un-made by erasing it. If you
believe a finding was wrong, record `upgrade_reason`.

An `unimplemented` verdict is a statement that Ze does not meet a MUST. Under
`ai/rules/rfc-compliance.md` that is a question for Thomas, never a settled deviation. Raise it
with the RFC text and the producing `file:line`. Then ask which way he wants it fixed.

## Why the fingerprint exists

A verdict is a claim about two things that both change: the requirement's text and the
test's source. `make ze-rfc-check` recomputes both shas and **fails** when either moved,
because a verdict that no longer describes what it judged is worse than no verdict — it is
a stale assurance wearing the badge of a fresh one.

That is what makes this durable rather than a one-time review that rots. A missing verdict
does not fail: the audit is sampled, the gate is total.

Biased to over-trigger on purpose. A false "stale" costs a re-read; a false "fresh" ships
a test that no longer enforces its requirement.

### Four states, and only one of them wants you

| State | What moved | What clears it |
|-------|-----------|----------------|
| `fresh` | nothing | — |
| `shifted` | the FILE around a tagged unit: a line shift, a sibling test, an import rewrite. The unit itself is byte-identical, so nothing was re-judged | `make ze-rfc-reseal`, then `make ze-rfc-index`. **No re-reading is asked for** |
| `stale-unit` | the tagged unit itself, or a cited producer in a `code` map | `/ze-rfc-audit <rfc>` — read it again |
| `stale-requirement` | the checklist line's own text | `/ze-rfc-audit <rfc>` — every judgement under it is void |

`shifted` exists because six of the sixteen commits that have touched `rfc/audit/rfc7606.json`
were hand re-stamps in which no verdict changed. Each one cost a human a mechanical proof and a
written note. And each one taught the reflex that re-stamping is what you do when this gate goes
red. That is the failure mode at fleet scale, so the class is now automated away.

`make ze-rfc-reseal` is the ONLY thing that writes `rfc/audit/` without a human editing it.
`make ze-rfc-check` is read-only and `make ze-rfc-index` touches the ledger alone.

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
| Which requirements are audited, proven, or carry a finding | the **Audit coverage** section of `ai/RFC-REQUIREMENTS.md` (derived, never hand-maintained) |
| Clear a `shifted` verdict | `make ze-rfc-reseal`, then `make ze-rfc-index` |
| Write or re-author a summary | `/ze-rfc <rfc>` |
| Public support claims | `docs/features/rfc-status.md` |
