# CI 41685742426: history repair follow-through

**Spec:** `plan/release/fixes/spec-cli-history-blocks-ui.md` (in progress; not closed).
**Branch:** main. No push is authorized.
**Goal:** Resolve the failures from full CI run https://github.com/ze-software/ze/actions/runs/41685742426 without claiming unobserved full-CI success.

## Status

- Committed history/owner repair: `aa34d659d7`. History no longer writes storage on Model.Update; bounded detached snapshots use the existing coalescer. Model, terminal and daemon owners drain before borrowed-store release. SSH channels own separate closers; Start waits for actual listener registration.
- Actual evidence: held-writer useful-progress red/green, submitted-slice alias mutant red/restored green, channel-owner red/green, real daemon startup-failure red/green, normal daemon shutdown green, 33 selected SSH race roots, four CLI-side consumer packages, constructor refusal, raw native editor admission and three unchanged editor recall cases.
- Final independent lifetime and behavior fix-only reviews are clean. Review verdicts are not a full spec Review Gate or full CI result.
- `job-ci-history-hub-owner-cache-recovery-25969fc6.log` passes both actual daemon paths and selected consumers. Its predecessor passed SSH but lost shared cache entries before hub/config execution; one documented recovery ran only those failed packages.
- `job-ci-history-original-force-verify-named-874110ea.log` passes both current draft force/verify counterparts in a single native run; it is not an 80-iteration load proof. The separate live-carrier run `job-ci-history-original-live-force-verify-c26c4bee.log` still fails: force step 16 waits for “force commits over warnings, never errors”; verify step 18 waits for the prior role/filter diagnostic. Both output deadlines expire. No claim that CI is resolved is justified.

## Remaining, not waived

1. History AC-7: original loaded plugin and reload groups still owe their full 80-iteration runs. Preserve every disk-backed store, status, deadline and assertion; do not promote the remaining drafts on a single pass. The reload route-refresh startup timeout was not proved to share the history cause.
2. History closure: measured new-branch coverage, scoped source-anchor checks, native pinned Review Gate and two-commit closure remain owed. Keep the spec open; do not repeat already-observed passing checks merely to reconfirm them.
3. Pinned-CI RFC publication: six ready paths and five foreign-source-bound holds are enumerated in `tmp/session/2026-11-03-231620/scratch/ci-final-rfc-publication-classification.json`. Do not publish whole-file fingerprints against a producer absent from HEAD. Two held DocSource patches likewise require their actual inventory producers to land first.
4. Full CI confirmation is unobserved. No agent runs `./le verify`; no push was authorized. Other sessions' BGP/config/RPC migrations and their current reds are not this repair's work to alter.

## Files already handled

The complete source/caller census, approvals, concrete commands, prior landings, observed failures and remaining native commands live in `tmp/session/2026-11-03-231620/state/session-state-cli-history-blocks-ui-omp-ci-41685742426-20261103.md` and the spec. The core History/Model, local/headless owners, SSH model/listener owners, daemon OnStop/OnceFunc wiring and real composition regressions were read and independently reviewed; do not restart their design audit.

## Edits

No further product edit is prescribed by the current evidence. Run the remaining exact scenarios first; diagnose a real failure at its producer before editing. Preserve foreign shared-file hunks. The history landing used a selective main.go patch to exclude another session's BGP ConfigData removal.

## Then

Use the retained native `stress-repro` argv from the canonical state: plugin's five named drafts and reload's three named drafts, `any-failure iterations 80 parallel 2 burners 1 timeout 240 minutes 25`, one CPU-heavy job at a time. Exit 0 means a failure was reproduced, not success; read the actual outcome. Run in the main checkout with the patched toolchain and registered feature tags. Do not copy the tree, widen deadlines, weaken assertions or rerun full CI locally.
