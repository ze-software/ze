# Closing the RFC verdict pass parent and retiring this directory

Written 2026-11-14, after the last child (services) closed in `22b8957163`.
Every child spec is now removed: bgp `3c417fdab2`, bfd `22fb2639f0`, ospf
`3a490b812e`, vrrp `3b30602d2e`, ike-eap `44797abe31`, access `a5ca6d22cd`,
routing `7523c321e8`, services `22b8957163`.

The parent is `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`
(status `blocked`). It is not closable as it stands. This page is the survey
of what closure needs, read at source by an agent that changed nothing. Remove
this page with the directory.

## What blocks closure

| # | Blocker | What clears it |
|---|---------|----------------|
| 1 | AC-11 is unmet. `grep -rlE '"verdict": *"(weak\|wrong)"' rfc/audit` lists 19 weak verdicts and no wrong one. Three have no live spec naming them: RFC9568-7.2-5, RFC5798-7.2-1 (VRRP) and RFC9234-5-9 (BGP role) | Each is named in the acceptance criteria of an owning spec. The other 16 are mentioned by a live spec; whether each sits in an AC table was not checked |
| 2 | The parent has no Deliverables Checklist, no Security Review Checklist and no closure sections | The closing agent adds both checklists, then `plan/templates/TEMPLATE-CLOSURE.md` |
| 3 | It has no `## Existing Mechanisms` section, which an in-progress spec owes | Rows for `auditStamp` and `stampRefusal` (extended), the `checkAuditFindings` upgrade predicate, `./le rfc approve` |

The 16 owned weak ids: RFC2205-3-40, RFC4271-5.1.2-2, SFLOW-V5-x-11,
RFC4271-8.2.2-15, RFC7296-3.13.1-1, RFC7296-3.13.1-2, RFC7950-9.6.4.2-1,
RFC7950-9.1-1, RFC7950-8.3.3-1, RFC5036-3.4.4.1-1, RFC5036-3.4.4.1-2,
RFC4090-6-6, RFC9728-3.3-1, RFC3101-3.2-2, RFC9252-5-3, RFC9494-4.2-6.

## What is wrong in the parent's text

| Place | Defect | Correction |
|-------|--------|------------|
| Line 9, Depends | Names children under `plan/pre-release/`, all removed | Name the closing commits above |
| Lines 567, 568, 679 | Say D-3 "permits" a MUST level that is then reviewed for demotion | D-3 fixes the level: a row whose sentence carries no BCP 14 keyword keeps MUST, and a level changes only where the sentence carries a different keyword (RULINGS R5). No demotion and no owner call arise |
| Lines 776, 777 | A-3 and A-4 read `unvalidated`, while line 183 says A-3 is confirmed | Record each assumption's outcome |
| Line 989, Goal Gates | Asks for `./le verify worktree`, which no agent runs | Cite the CI run |
| AC-1 to AC-6, AC-8, AC-9 | Built, but no run, commit or closing SHA is recorded | Fill the evidence: `mode rejudge` in `audit_stamp.go`, the `TestAuditStampRejudge*` tests, the child closures |

## What closure owes

- No lighter route exists for a parent: commit A preserves, commit B removes.
- No learned entry is owed unless there is a lesson.
- Commit B needs `no-news`: the children carried every operator-visible change.
- A review artifact is owed even with no code. No commit carries
  `Spec-work: rfc-verdict-test-fix-pass`, so the reviewed change set is the
  removal. Whether the recorder accepts a removal alone is unverified.
- The parent has no owned journal rows table. Four open rows mention the stem
  and each already has another owner, so nothing moves.

## Citers of the parent's path

| Citer | Becomes |
|-------|---------|
| `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` line 171 | bare stem, marked closed |
| `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` line 196 | bare stem, marked closed |
| `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` lines 372, 783 | bare stem |
| `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` line 29 | `rfc/corrections/sflow-v5.md` (this spec refuses every edit today, see page 39) |
| `plan/release/fixes/spec-ike-dpd-demand-driven.md` line 397 | bare stem |
| `plan/release/validation/spec-rfc-clause-scoped-proof.md` line 27 | bare stem |

## Retiring this directory (96 tracked files)

Citers from outside, each to repoint when the content moves:

| Citer | What it cites |
|-------|---------------|
| `spec-bgp-fsm-delayopen-and-automatic-start.md` line 99 | RULINGS R47(a), R53(a) |
| `spec-bgp-unowned-rfc-rows.md` line 105 | RULINGS R58(b) |
| `spec-bgp-llgr-per-family-config.md` lines 42, 54 | owner ruling 8(e); `bgp/c27-author.md` |
| `spec-rfc-clause-scoped-proof.md` lines 18, 43 | decision 10; HANDOFF, the briefs, RULINGS, c35 |
| `rfc/corrections/rfc9514.md` line 3 | decision 10 |
| `plan/post-release/engineering/tooling-one-off-evidence.md` line 63 | `data/` |
| `plan/journal/gate-red-where-nothing-blocks-on-it.md` line 10 | `data/vrrp-closure-records.md` |

Proposed homes, not yet agreed with Thomas:

| Content | Home | Why |
|---------|------|-----|
| `RULINGS.md` whole | `plan/records/rfc-verdict-test-fix-pass-rulings.md` | A historical record; keeps every R label resolvable |
| Owner ruling 2 (what a negative is for a generate/accept row) | `ai/META-DECISIONS.md`, "Test changes" | A rule on how agents judge tests, found in no rule or page today. Thomas words it |
| Rulings 3, 7, 8(a); decisions 4, 7, 9 | The design page of each subject (BFD, GR, route-refresh, Prefix-SID, LLGR) | A design decision lives in its subject's page. Whether each page already carries it is unverified |
| Ruling of 2026-10-30 (RTR version 0 is owed) | `rfc/short/rfc6810.md` gap annotations | The ruling says so itself |
| One-off permissions (decisions 1, 2, 11, ruling 6(a)) | Nowhere | Spent |
| `data/` | `plan/records/rfc-verdict-test-fix-pass/data/` | Two citers need it |
| The briefs, session records and 74 per-package handoffs | Removed, residue into `plan/handover/39-residuals-of-closed-handovers.md` | As `935398f8df` did |

The three `plan/handover/enum-switch-exhaustiveness-20261006-*.json` files
index evidence archives of a closed spec. The archives are not in this
checkout. They belong in `plan/records/`, beside
`enum-switch-exhaustiveness-inventory.json`, with their internal paths left
as written.

## Order of work

1. Home the three unowned weak verdicts in owning specs.
2. Retire this directory in a commit of its own: move the records, write the
   design-page entries, repoint the citers above.
3. Bring the parent to in-progress, add the missing sections, correct the
   text, fill the evidence.
4. Commit A (parent plus citer repoints), then commit B (removal, `no-news`).
