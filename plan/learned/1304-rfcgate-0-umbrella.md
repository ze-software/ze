# 1304 -- rfcgate-0-umbrella

## Context

`make ze-rfc-check` was green, and a reader took that green to mean Ze's RFC
compliance claims were checked. It did not mean that.

Extraction was unbounded. An RFC counted as covered once someone wrote a
summary, however little of the source that summary captured. A test tag counted
as proof whether or not any pipeline ran the test. An audit verdict was a
free-text field with no schema and no consequence.

The public status page had one guard. That guard fired only when a summary
already admitted a gap.

This umbrella owned the sequencing, the drain policy, and the scope boundary for
four children that close those four holes. It wrote no gate logic itself. Owner
decision D4 fixed the order: machinery first, then a fleet to drain the backlog.

## Decisions

- **Four children, strictly ordered, never two in flight.** 1 extraction, 2
  evidence, 3 audit teeth, 4 ledger. Each landed alone and green.

  Child 4 had to follow child 1, for a reason independent of module conflicts.
  Its four enrolments must carry an extraction sign-off. Child 1's grandfathering
  is scope rather than an allowlist. Landing child 4 first would have enrolled the
  four stems with no bar in existence, and grandfathered them out of the central
  check permanently.
- **No child drains the backlog.** The set ships gates, ratchets, artifact
  formats and published counters. Owner decision D4. A fleet dispatched before
  child 1 would produce records the gate cannot read.
- **The drain quota ships at rate 0** (owner decision D5). The repository has no
  release concept to anchor "N per release" against. There are no tags and no
  VERSION file. A guessed rate is the one failure mode that loses the forcing
  function for good. A rule that reds the gate on unrelated work gets removed
  rather than obeyed.
- **Every register counts toward the quota, and none is summed with another**
  (D6). `rfc2119`, `prose` and `manual-walk` appear in three separate columns. A
  quota that excluded the weaker registers would be unsatisfiable for the 53
  entries that cannot take the `rfc2119` grade. A summed total would let a
  `manual-walk` sign-off read as keyword-verified.
- **The 32 rowless enrolments stay deferred, sequenced BEHIND the annotation
  re-derivation** (OR-3). The judgement that their absence is safe rests on
  `{not-applicable}` annotations that `ai/rules/rfc-compliance.md:56` voided as
  authority. Writing the rows first would bake a void basis into the public page.
- **Every defect the machinery found got a named spec and an owner ruling.** None
  was recorded as a known failure. `ai/rules/no-parking.md` allows a shard only
  for a non-deterministic failure whose mechanism nobody can determine. None of
  these qualified.

## Consequences

### What the numbers did

Three of the four new summary lines report figures WORSE than what the gate
implied before. That is the deliverable.

- The extraction backlog was always 166 of 168 enrolled RFCs. It was invisible.
  Now it is published with a per-entry deficit ranking.
- Interop evidence had no automation at all: 104 scenarios, zero runs. Evidence
  is now tiered, and a nightly-only requirement is marked as such and counted in
  its own column.
- Audit coverage fell from a reported 4.52% to 3.64%. Coverage did not shrink.
  The denominator had been wrong, and eight verdicts sat in no column at all.

The gate now prints four summary lines. Their figures, measured on the closing
tree:

| Line | Figures |
|------|---------|
| `rfc-requirements` | 2721 gated MUST-level requirements, across 168 enrolled RFCs. 2582 test tags resolved |
| `evidence` | unit/verify 2573. functional/verify 7. editor/verify 0. interop/nightly 2 |
| `extraction` | Signed off of 168 enrolled: rfc2119 1, prose 0, manual-walk 1. Unsigned grandfathered backlog 166 |
| `audit` | 49 proven and 3 audited-but-not-proven, of 52 verdicts. 49 of 1345 auditable requirements audited, which is 3.64% |

### What the machinery found the first time anyone looked

Five compliance defects. Each is owned by a named spec, and each carries an owner
ruling.

| Spec | Finding |
|------|---------|
| `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` | 214 unextracted IKEv2 MUSTs, 108 unimplemented. No guard on Message ID exhaustion. DPD probes sent unencrypted |
| `plan/spec-fixit-dns-rfc1035-conformance.md` | 27 MUSTs, six with no code path. No 512-octet bound, no TC bit, and the TTL is never raised against the SOA MINIMUM. Full compliance ruled, including AXFR and IXFR |
| `plan/spec-fixit-isis-hostname-ascii.md` | Ze emits non-ASCII on the IS-IS wire |
| `plan/spec-fixit-bgp-shutdown-cease-notification.md` | Ze sends no NOTIFICATION with error code 6 on SIGTERM. `Session.Close` has no reachable production caller |
| `plan/spec-fixit-bgp-per-family-prefix-enforcement.md` | Three per-family YANG leaves stored as scalars, so one family's setting silently wins |

### What stayed open

- `rfc1035` and `rfc5301` still carry public claims whose MUSTs are unproven.
  Both publish `Partial`, and 34 of their gated rows carry no test.
- `check_unproven_support` does NOT catch that case. It fires only on a summary
  declaring ZERO gated requirements, and both now declare real ones. The claim is
  no longer backed by nothing. It is not backed by proof either.
- The only difference from before is that the debt is declared, counted and
  owned. `rfc/not-enrolled.txt` renders it as DEBT, and two owner rulings commit
  to fixing both fully. That is weaker than "conformant" and much stronger than
  the silence the set started from. Do not let a green gate imply otherwise.
- The drain rate ships at 0 by owner decision D5, so the forcing function is
  inert until the owner arms it. Arming is a one-line edit to
  `rfc/drain-budget.txt`. A quota that ships inert can also stay inert forever,
  and only the owner can close that.

## Gotchas

**The recurring shape, which is the transferable lesson.** The same failure
appeared at every altitude of this work: **prose asserting a property that
nothing checked, with green tests beside it.**

- A docstring claimed a delegation that was never wired. Ten assertions covered
  a dead function.
- A documented authoring path produced a permanently red gate. Its remedy was
  false in all three clauses.
- A guard against dead tokens kept its own uncoupled copy of the token list.
- A gated structural fact read a field off a metric that never carried it.
- A comment asserted that a sibling audit was complete before it was performed.
- An owner ruling's escape clause checked three facts about the artifact and none
  about the document it was supposed to describe.

In every case the fix was cheap. Finding it required someone to drive the path or
grep the call sites, rather than read the claim. Reading the claim is what
produces the green bar.

Other traps worth carrying forward.

- **A gate that reds on correct work gets deleted rather than obeyed.** This
  shaped three separate decisions: the drain rate of 0, the grandfathering of the
  166 and the 32, and AC-12's narrowing to immediate adjacency. Each time, the
  strict reading would have reddened honest work on day one.
- **Arming a guard before you fix what it catches can block the fix.**
  `ze-rfc-check` runs in both verify modes, and `commit_helper.py` refuses a
  commit script over a non-FRESH verify. Landing child 4's guard armed and red
  would have blocked every commit in the repository, including the commits that
  clear it. Owner Ruling OR-2 moved the arming point to the same commit as the
  fix.
- **A percentage that improves can hide a denominator that was wrong.** Audit
  coverage read 4.52% against 974 requirements. The honest figure is 3.64%
  against 1345. Publish the denominator beside the ratio, and say which
  population it counts.
- **`ai/rules/rfc-compliance.md` voided every `{gap}` and `{not-applicable}` as
  authority on 2026-07-27.** Any work that reasons from those annotations
  inherits a void basis. That is why the 32 rows are sequenced behind the
  re-derivation and not before it.
- **`{single-polarity}` is NOT void.** The rule voids `{gap}`,
  `{not-applicable}` and `partial`. `single-polarity` does not appear in it. It is
  a first-class gate annotation, and it still needs the owner's authorisation
  like any annotation. A supervising session got this wrong and framed an
  escalation as though no annotation route existed.
- **No review-gate artifact was recorded for this umbrella.** It carries no code,
  so an artifact over it would pin zero files. `commit_helper.py` runs
  `review_gate.py check` on a closure commit, so one must be recorded before the
  umbrella's commit B or the commit is refused. All four children have clean
  artifacts.

## Files

- `rfc/drain-budget.txt` -- the authored policy. `start 2026-07-29`, `rate 0`
- `rfc/extraction/` -- the sign-off format, its README, and four artifacts
- `rfc/not-enrolled.txt` -- the declared remainder (child 4)
- `scripts/dev/rfc_requirements.py` -- 1,769 lines to 6,488 across the set
- `scripts/dev/rfc_requirements_test.py` -- 665 tests
- `ai/RFC-REQUIREMENTS.md` -- the set's published surface
- `ai/rules/rfc-compliance.md` -- the ratchet table, now six ratchets
- `docs/features/rfc-status.md` -- states which of its own properties are checked
- `test/interop/run_test.go` -- the interop runner fails closed without Docker
- Sibling summaries: `plan/learned/1295-rfcgate-1-extraction.md`,
  `1296-rfcgate-2-evidence.md`, `1297-rfcgate-3-audit-teeth.md`,
  `1303-rfcgate-4-ledger.md`
