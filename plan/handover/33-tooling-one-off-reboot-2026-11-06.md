# Tooling one-off defects: reboot and another-machine handoff

**Spec:** `plan/post-release/engineering/spec-tooling-one-off-defects.md`
**Branch:** `main`
**Goal:** Complete the approved tooling repairs and their discrimination proofs.
The owner requested a checkpoint, commit and push before stopping this host.
This is a continuation record, not closure: the spec remains in progress.

## Publication at shutdown

Canonical evaluator checkpoint: `6abaa27aa6`. Handoff/source snapshot checkpoint:
`5b5bfd703b`. The configured push destination was checked as SSH to
`github.com:ze-software/ze.git`, with no embedded credentials. There were 134
commits ahead of the recorded upstream after these checkpoints; a normal push
publishes that accumulated branch history, not only this session's two commits.

Automatic approval review rejected the attempted push before execution, stating
that the accumulated commits and diagnostic snapshots were a sensitive payload
to an unverified destination. No push was performed. The destination was then
checked as above; raw logs/captures/profiles/binaries are absent from the source
asset directory. Thomas still needs to authorize publication to that destination
and prepare the native push from his interactive terminal. The required command
is `./le commit push "we want to commit push"`; execute only its printed script.
Do not infer that a local commit or Git bundle is already present on GitHub.

A private incremental Git bundle will be saved beside the evidence archive as
`tooling-reboot.bundle` after the final handoff commit, plus a checksum and
recorded HEAD. It carries the local commits since the recorded upstream. Copy
that bundle and the private evidence archive before switching machines if the
remote still lacks these commits. Clone the repository normally first, then
verify the bundle's prerequisite base before importing it; preserve an existing
dirty checkout and never reset or overwrite it to restore the checkpoint.

Read this record before the older November 3 handover or scratch state. It
supersedes their current-status statements, including private author handoffs
that still describe subsequently tested source as untested.

## What is saved

The root explanation requested by Thomas is committed as `938894dc74`:
`INVENTORY-TRAVERSAL-REVIEW.md`. It explains the repeated traversal, both repair
routes and their correctness obligations. Thomas requested the document for
review; he has **not selected A or B**. Do not implement a fourth performance
attempt while treating the request for that document as approval.

Other earlier commits include `d35b001cd8` (evidence-loss record and retention
repair owner), `ba5e3f3d4f` (timestamp guard scoped to each full TCP tuple), and
`cc8782a1a0` (current proofs and their limits). The new canonical inventory
checkpoint `6abaa27aa6` includes the complete evaluator package, coupling/gated-table
Snapshot consumers, the root external-effects explanation and feature-gate
contract. Its commit message explicitly records the incomplete state.

The adjacent `33-tooling-one-off-reboot-assets/` directory retains source-only
diagnostics, designs and unfinished source snapshots for 140 other task-owned
paths. These are inert `.txt` files: they are **not installed canonical code**.
Its manifest identifies the original canonical paths, base commit and hashes;
its README explains safe reconstruction. Source ownership was independently
checked. Unrelated protocol, appliance, CodeQL and weakening-checker changes
were excluded. On this host the unfinished canonical edits remain in place.
On a fresh clone, reconstruct those snapshots before continuing their tests.
Never overwrite a canonical file whose bytes match neither saved version.

Raw evidence and a private copy of the mixed working tree are outside cache:

```text
/home/thomas/.local/share/ze-evidence/tooling-one-off-defects-01a1a907/recovery-2026-11-06/
```

`reboot-checkpoint/` contains this session's complete scratch and state, a
binary-capable working-tree patch, and untracked-source archive. The surrounding
archive retains profiles, test binary, browser images and native packet proofs.
It is private: raw proof records can contain configuration credentials. Do not
commit or publish it. Copy the whole private archive separately to the other
machine if original proof bytes are required; Git alone does not transfer it.
A reboot preserves it, but moving machines does not make it available there.

Session identity was `01a1a907-3278-7971-98dc-dbc41d4a54a5`; original scratch was
`tmp/session/2026-11-06-080923/scratch`. Restored overlays must use the new main
checkout's absolute paths. Do not execute old overlay JSON unchanged.

## Validated results and remaining failures

The Git-only external-effects contract and bounded trust assumption are
approved. Do not request library copies or add host/standard-library source as
an analyzer input. Ordinary unsafe pointer fabrication, linkname and native
symbol tricks are outside the explicit approved trust boundary.

The latest explicit type-argument repair is implemented in
`externaltypearguments.go` and the IndexExpr branch of `externalexpressions.go`.
It retains deferred reflected type authority without inventing callable
ownership. Before the repair, the 32-case committed probe exposed 16 false-clean
cases. All 32 restored cases and 43 prior regression roots passed with race
instrumentation. Logs are identified by `c464e961` and `cca25cb2`; the exact
source manifest and independent review are retained privately and in diagnostic
source assets where applicable. No new test run was made just for this reboot.

The broader committed-fixture baseline executed 151 distinct roots: 145 PASS,
six FAIL, no skipped roots. It used logs `136049b1`, `5268956f`, `8b67a27c`.
The six unresolved failures are:

1. `TestCommittedAssemblyCanonicalHeaderBindingsRefuse`
2. `TestCommittedAssemblyGeneratedHeaderBindingsRefuse`
3. `TestCommittedAssemblyGeneratedHeadersPreserveFullReport`
4. `TestCommittedAssemblyHeaderEvidenceRetainsSnapshot`
5. `TestCommittedAssemblyGeneratedParameterScopeCannotFallBack`
6. `TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection`

The first five treat an exposed generic type template as a concrete value and
lose its formal type argument. A certified symbolic-template design still needs
work; globally skipping generic types or suppressing unknown formals is unsafe.
The sixth needs both precise scalar projection and a retained range-element
source equation for formal grounding. Static type alone is insufficient.
Diagnoses and independent REWORK reviews are in the saved assets. No fix to
these six failures has been implemented. The explicit type-argument repair is
separate and does not establish their restoration.

The actual repository inventory still has no complete report. Its last observed
run reached the unchanged ten-minute bound; later correctness changes have not
had another actual-root run. Three local performance approaches and a synthetic
traversal diagnostic are explained in the root review document. Do not claim a
new report or a performance improvement from the scoped correctness passes.

Two new source-only probes remain **unexecuted**:

* `inventory-generic-inference-audit`: first establish the actual Go inferred
  interface/reflection premise, then test six committed cases. A source-derived
  hypothesis is not an observed false-clean analyzer result.
* `inventory-range-scalar-tests`: revised seven committed roots and two solver
  roots. The original draft was refused; the revision needs fresh independent
  actual-delta review before MAIN formatting or execution. Preserve descriptor,
  callback, mutation and missing/mixed-formal controls.

The accepted native GoBGP same-suite pair passed with race instrumentation:
`TestNativeGoBGPSecondarySameSuitePair`, log `ecdbab03`. Cut proof `668364761`
and full proof `2540349095` share Ze/BIRD/FRR images; the intended omission
exposes the stale secondary route, and restoration passes. Cleanup passed.
Independent actual review accepted this bounded IPv4 stable-session case. The
private four-file overlay, immutable private patch inputs and review are saved.
This does not replace the missing historical four-peer captures or fifteen
other checker proofs. The earlier ordinary scenario passed its native assertions
but its outer run failed cleanup; minimal removal controls did not reproduce
that stall, so its cause remains unproved.

The Phase 6 controlled partial-handoff baseline now has a genuine RED:
`TestNativeCatalogPartialHandoffCleanup`, log `0519d6ad`, exit 1. Its protected
child remained observable, the packet socket stayed live, and the join receipt
was EOF; the foreign sentinel oracle did not fail. The test-only startup-abort
envelope was independently approved before execution. The final safety fallback
ran after the oracle; no cleanup failure was printed. Retain its journals and
confirm exact fallback settlement before further cleanup experiments. The full
native observer draft is still refused: P1 can kill the gate before an
untransferred child is joined, and P2's sentinel can expire too early. Neither
repair has been implemented. Never run the refused full native roots as-is.

The browser poster run passed (`c908d5b4`); three output WebPs were retained
outside cache before cleanup. Its dormant failure observer does not establish
the historical Save/Fsync cause. The latest documentation check (`0c016d69`)
failed with 142 drift issues; source anchors passed (4,234 paths, 772 packages).
Do not report the overall documentation check as green.

## First five continuation actions

1. Restore only the needed unfinished canonical source from the asset manifest
   after comparing base/current/after bytes. On this host those edits already
   exist. Read `INVENTORY-TRAVERSAL-REVIEW.md` with Thomas and record his A/B
   decision before changing traversal. This is the outstanding product choice.
2. Review the inferred-interface probe's exact delta independently. MAIN alone
   formats the selected copy and runs its runtime premise, then its smallest
   plugin baseline. Preserve genuine baseline evidence before any transfer fix.
3. Review the revised range tests independently, obtain a real positive-case
   baseline, then implement the range source equation and scalar projection.
   Preserve the unchanged failing family witness and all authority negatives.
4. Finish the generic-template design without assuming the ambient pool is a
   complete catalog. Implement and rerun the five named assembly roots only
   after the design preserves genuinely unbound lexical-parameter refusals.
5. Repair P1/P2 in the private Phase 6 observer, review it, restore the controlled
   RED, then prove actual native normal/cancel paths. Promote only reviewed,
   observed source. Phase 2 is needed after these five actions for the broader
   inventory report, causal omissions, coverage, remaining spec phases and close.

Exact test selectors and original admitted recipes are in the diagnostic
HANDOFF files. Recreate the current feature-tag selection from the project's
test tooling; historical `$tooling_test_tags` is a shell variable, not a literal
tag list. Use race instrumentation and `-p=1`, `-count=1`; fixture probes use
their documented ten-minute bound, the controlled handoff root one minute.
Run in the main checkout through native job admission, one CPU job at a time.
No agent runs `./le verify`, a full suite, or development/tests from an export,
worktree or copied tree. Subagents may read/write reviewed source only; all
tests, builds, formatters, Go/gopls, Node and Docker work stays with MAIN.

## Work still owned by the open spec

The detailed phase queues are saved as `phase6-current-proof-queue.md`,
`phase9-current-proof-queue.md` and `phase12-current-proof-queue.md`. Follow the
spec's full acceptance table, not just the five next actions. Phase 7's current
work is complete, but the coarse spec remains 4/13 and is not closure-ready.
P-39 uses the owner-approved separate AS_PATH paired proof while retaining both
GTSM clean/broken/restored and negative packet controls.

Original ze-zeroed archives/boundary traces, June technique/fuzz inventory,
post-OSPF lint log, October naming archive and wider GoBGP captures are still
missing. Fresh greens do not recreate them. Automatic trim removed old Codex
session evidence on November 6; the new private archive avoids that cache.
The separately owned retention repair is not implemented. Keep the main spec
and its Work Not Done owners open until their evidence and review gates pass.

The working tree still contains other sessions' changes. Do not clean, reset,
restore, stash, or stage the whole tree. Use only native `./le commit create`
scripts. Push only through `./le commit push "we want to commit push"`; this
repository requires Thomas's interactive terminal for preparing that push.
An agent must report a terminal refusal, never simulate Thomas's terminal.
