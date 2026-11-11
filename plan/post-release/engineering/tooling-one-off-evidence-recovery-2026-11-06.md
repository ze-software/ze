# Tooling evidence availability after the November 6 cleanup

Earlier session raw evidence became unavailable during an automatic cleanup run
while work on the
tooling one-off defects spec (spec-tooling-one-off-defects, now closed) continued.
Source changes remain in the checkout. Earlier observed results remain recorded
in the [evidence page](tooling-one-off-evidence.md#git-only-evaluator-continuation-on-2026-11-06)
and conversation, but their former session paths cannot supply the original bytes.

## What is available

The main session preserved the following files outside the repository and managed
caches, under this private archive:

```text
/home/thomas/.local/share/ze-evidence/tooling-one-off-defects-01a1a907/recovery-2026-11-06/
```

| Artifact | Availability and use |
|----------|----------------------|
| `evidence-loss-store-trim-last.log` and `evidence-loss-store-trim-stamp` | Preserved cleanup log and stamp. The log records the automatic run and removal counts. |
| `recovered-source/capture-timestamp-before/` and `capture-timestamp-after/` | Exact source snapshots for the two-file timestamp change. These support review of that change. |
| `recovered-source/method-census-after/` and its source-freeze note | Five implementation/documentation files survived. The corresponding before snapshots are unavailable, so review cannot claim an exact before/after comparison. |
| `recovered-working-state.md` | Reconstructed working state, explicitly labeled as reconstructed. It records observations and remaining obligations. |

The former `tmp/session/2026-11-05-001121/` evidence and November 2 state record
are unavailable. Missing files include run logs, omission overlays, profiles,
native captures, proof JSON, and publication selections. A recreated November 5
directory contains later source snapshots only. Its existence does not establish
recovery of the earlier evidence.

The original source-boundary proposal and Git-only design linked from the root
explanation are also unavailable at their old scratch paths. The approved
decision and trust contract were recorded in `INVENTORY-EXTERNAL-EFFECTS.md`,
removed with the proof by `spec-inventory-fast-registry-checks` and still
readable in git history.

## What the cleanup evidence establishes

The preserved log starts at `2026-11-06T07:55:23Z` and reports 63 session
directories removed. It also reports a permission error during removal of
`tmp/session/2026-11-02-121815/`. It does not enumerate every removed directory.
The November 5 originals became unavailable, but the log does not conclusively
identify the remover of each missing directory.

Current source explains how an active Codex session can lose that protection.
`internal/le/procscan/procscan.go` recognizes Claude as the session CLI.
`internal/le/session/reap.go` `judge` protects its own session, process arguments,
valid PID pins, and recent Claude transcripts. It has no corresponding Codex
transcript check. `internal/le/scratch/livetrim.go` `trimSessions` uses the
parent session directory's modification time after that liveness judgment, with
a 24-hour age floor. Writes inside an existing nested directory do not renew
that parent timestamp.

`internal/le/scratch/storetrim.go` `storeTrimBeforeDispatch` can start a detached
trim during an ordinary command when the hourly stamp is due. The evidence does
not establish that closing the other Codex session triggered this run. This
record documents the exposure and loss. It does not claim a retention-code fix.

## Continuing verification

Session scratch remains temporary storage. New retained raw evidence belongs in
the private archive above, accompanied by a durable record of canonical source,
the exact command and selection, observed failures and skips, and artifact
digests computed from the retained bytes. No new test result or artifact digest
is inferred from the reconstructed state. The verification below was observed
by MAIN after recovery and has its own retained log.

Unchanged checks already observed are not rerun merely to renew a timestamp.
Changed implementation and missing raw proofs still require their appropriate
checks. In particular, a fresh native capture can prove the repaired checker,
but cannot recreate the lost GoBGP capture or any missing historical archive.
The spec remains open until its required implementation and evidence exist.

## Restored timestamp controls

On November 6, MAIN ran the three additive timestamp roots against the repaired
checker. The command exited 0. All three roots passed, with no skips. Both
chronological and interleaved independent connections passed. The controls also
passed for a measured epoch fraction and refusal of zero, hidden reversal, and
opposite-direction reversal times.

```sh
tooling_test_tags="ze_le integration $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label tooling-clock-targeted-restored-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -run '^(TestDelayOpenTimestampOrderPerConnection|TestDelayOpenPermitsMeasuredEpochFraction|TestDelayOpenRefusesInvalidConnectionTime)$' \
  -count=1 -timeout=3m -v ./internal/le/interoplab/bgp
```

| Retained input or output | SHA-256 |
|-------------------------|---------|
| `internal/le/interoplab/bgp/check_delayopen_wire.go` | `c6d0a3a2d7c324910474cd57e6e9263021142b8c631006dbe71c39ee7fe2c006` |
| `internal/le/interoplab/bgp/check_delayopen_timestamp_test.go` | `3d0801fe5dd11eed6026e13387835db034f62335f1ca36ab2b6cba349373858e` |
| Archive log `job-tooling-clock-targeted-restored-20261106-992abb87.log` | `794c99cd0b9ee4ace4eb8879649fae40dc971f77deedba36f67194c172a52e52` |

The log is retained under the private archive named above. This result proves
the selected timestamp controls. The complete native GoBGP job remains owed,
with the ordinary assertion and cleanup outcomes recorded below. This run
supplies no verdict for the inventory method census.

Commit `ba5e3f3d4f` landed the clock correction, its timestamp tests, and the
corresponding documentation. It does not close Phase 9 or the tooling spec.

## Inventory method census source controls

MAIN observed all 42 `TestExternal` roots pass after the method census change.
The command exited 0, with no failed or skipped roots. The coverage output
records execution of all 29 blocks in `externalmethodcensus.go`. This is a
source-selected regression result. The complete inventory report and signed
fixture cohort remain owed.

Using the feature tags and session environment shown above, MAIN ran:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label tooling-census-source-cohort-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 -run '^TestExternal' \
  -count=1 -timeout=5m \
  -coverprofile=/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/census-source-cohort.cover \
  -v ./internal/le/repo/inventory
```

MAIN checked all 149 source and configuration entries in the recorded manifest
after the passing cohort. The private archive contains the following outputs
and manifest:

| Archive artifact | SHA-256 |
|------------------|---------|
| `job-tooling-census-source-cohort-20261106-6ff56818.log` | `5d63bd27813ae76f325cf0a8e7ee1fcaf25d3ef539c8f13cb3f11b23eb4944cb` |
| `census-source-cohort.cover` | `ea89552bab0bbb974bef622780eba3f0d812b6ec0bdc178065c065f8671dc71a` |
| `inventory-census-source-inputs-20261106.sha256` | `75dd748773ad7ff4c36be8904d2256809cc73788d8c7f94b35dbb371b773e34f` |

The archived `current-inventory-source/` files support source review only.
Testing ran in the main checkout. The actual-root result follows.

## Actual inventory root still times out

The method-census actual-root run failed at its unchanged ten-minute bound and
produced no inventory report. This was the fourth timed-out actual-root attempt
after the reflection repairs. The three earlier attempts also failed. All 149
source and configuration entries in the recorded manifest still matched after
this run.

MAIN ran this command with the feature tags defined above:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=0 \
  bin/le-tooling-resume-01a1a907/le job run \
  label tooling-census-real-root-20261106 quiet command \
  go test -tags "$tooling_test_tags" -p=1 \
  -run '^TestAnswerAnswersTheInventory$' -count=1 -timeout=10m \
  -cpuprofile=/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/census-real-root.cpu \
  -memprofile=/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/census-real-root.mem \
  -o /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/census-real-root.test \
  -v ./internal/le/repo/inventory
```

The test log names `TestAnswerAnswersTheInventory` and reports
`panic: test timed out after 10m0s`. MAIN then used the matching retained binary
and CPU profile for diagnostic attribution:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=0 \
  bin/le-tooling-resume-01a1a907/le job run \
  label tooling-census-real-root-cpu-attribution-20261106 quiet command \
  go tool pprof -top -cum -nodecount=35 \
  tmp/session/2026-11-06-080923/scratch/census-real-root.test \
  tmp/session/2026-11-06-080923/scratch/census-real-root.cpu
```

The cumulative CPU sample attribution is 85.25% for both
`externalReflectionWalk.collect` and `collectNamed`, and 77.71% for
`collectMethods`, in `internal/le/repo/inventory/externalreflectiontypes.go`. <!-- doc-links: ignore (file deleted by the inventory spec; this record cites it historically) -->
Those nested totals overlap. They describe an incomplete run and establish no
report-speed improvement, throughput figure, or completed-run comparison.

| Archive artifact | SHA-256 |
|------------------|---------|
| `job-tooling-census-real-root-20261106-e57d764f.log` | `ecf200b920008dba70dfea4fafbb802959f57998c0f3d48ad75a60b6423fc6a9` |
| `census-real-root.cpu` | `0582aebe6dc081e434ca0cc4b1a0ce4a6b6a1232dca53cf878a92c4668ea77e1` |
| `census-real-root.test` | `5b875919972831f48ca0944f930d17e2e242f1fec721a1dd9291b7306759fb9b` |
| `census-real-root.mem` | `ef2d1b881218d4b367975ef74a5c2296e882e825a28a3c5fae374ae320d710a7` |
| `job-tooling-census-real-root-cpu-attribution-20261106-6a9289ce.log` | `6dc9294e37fa3c2bdb09ff0d3551bc1b7417a67bea700a30f5c31d0aa90ae142` |

MAIN stopped further inventory patches under the three-fix rule in
`ai/rules/completion.md`. Source-only diagnosis of the analysis model is in
progress. No further inventory fix or complete report is claimed. The separately
tested clock and GoBGP source controls have the result below.

## Broader clock and GoBGP source controls

MAIN observed all 27 selected DelayOpen and GoBGP reload roots pass, with no
failed or skipped roots. The command ran from the main checkout with the same
feature tags:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label tooling-clock-gobgp-affected-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -run '^(TestDelayOpen|TestGoBGPReload)' -count=1 -timeout=5m \
  -coverprofile=/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/clock-gobgp-affected.cover \
  -v ./internal/le/interoplab/bgp
```

| Archive artifact | SHA-256 |
|------------------|---------|
| `job-tooling-clock-gobgp-affected-20261106-71c13f13.log` | `df6e844eda5af4fad8aa7eb8dc6eebfd659f92937b08b0ce215871724a83e67b` |
| `clock-gobgp-affected.cover` | `9d4ecc5a817e4919861617706c2971e78bbf9013542b8e8c7dff6a1067bf32b3` |

This result covers the selected source controls. The fresh native ordinary
GoBGP result is recorded below.

## Inventory traversal decision

Owner choice is pending. Three approaches reduced work inside each descriptor
visit but did not produce the required actual inventory report: cheaper binding
and output construction, completed pure receiver queries, and the ordered method
census. The current actual-root run still times out after ten minutes. This
source-only diagnosis does not approve another repair.

The model being reconsidered is that cheaper individual visits are sufficient.
`externalreflectiontypes.go` `collectNamed` removes a named frame when its active
call returns, so a later sibling can traverse that completed subgraph again.
`collectMethods` traverses a matching method signature even when the same method
is already present in the output. The current depth limit bounds each path,
while branching can still multiply the total visits.

MAIN observed all nine cases of the diagnostic-only
`TestExternalTraversalObservation` overlay pass with race instrumentation.
At depth 12, the single-child chain made 29 collection calls across 29 distinct
structural keys and visited the one method signature once. The branching
diamond made 24,575 calls across 41 structural keys and visited the same one
signature 4,096 times. Both returned the same `Touch` method, without unknowns
or external origins, and recorded no active backedge, scope-bearing visit, or
argument-bearing visit. The recursive control recorded one backedge. Distinct
generic bindings and a one-key diagnostic-storage overflow also passed.

These keys deliberately omit semantic substitutions and active contexts. The
counts demonstrate repeated traversal in the synthetic graph only. They do not
establish equivalent semantic states or dominance in the actual repository run.
No evaluator repair was part of this observation. MAIN ran:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label tooling-traversal-observation-20261106 quiet command \
  go test -race \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/inventory-traversal-observation/overlay.json \
  -tags "$tooling_test_tags" -p=1 -run '^TestExternalTraversalObservation$' \
  -count=1 -timeout=3m -v ./internal/le/repo/inventory
```

The private archive retains the overlay and its notes under
`inventory-traversal-observation/`. Its observed log is
`job-tooling-traversal-observation-20261106-04a11315.log`, SHA-256
`cb2fe2dd269b4634938c36d6acc0fb33826cc2297289c7c520a4e69b94d7babe`.

The private archive contains the fuller proposal in
`inventory-reflection-traversal-model-decision.md`. Both proposed routes retain
the approved Git-only boundary, exact depth and substitution rules, ordered
refusals, independently owned outputs, and all existing proof obligations.

| Choice | Proposed implementation | Proof required before reuse |
|--------|-------------------------|-----------------------------|
| A. Query-local completed-subgraph coverage (recommended) | Within one query and its same monotone output union, skip a completed contribution already represented there. Store coverage facts without cached answer maps. Use the original traversal whenever proof is unavailable. | Exact incoming depth and substitutions, completed traversal, and independence from the current active ancestors. A prior visit in another ancestry alone proves none of these. |
| B. Explicit ordered state graph | Replace recursive execution with ordered continuations and completion states that retain the full relevant active context. This is a larger rewrite. | Equivalent source, depth, substitutions, visibility, root flags and active context. Unproved state identity retains the original operation. |

Route A is recommended as the smaller change with explicit raw fallback.
Its useful coverage of the actual repository remains to be established.
Neither route is implemented or approved by this record.

`ai/rules/completion.md` requires this decision:

> After three failed fixes you MUST stop, report all three approaches, question the mental model, and ask the user which way to fix it.

MAIN has stopped further inventory patches pending the owner's choice between
these routes. The complete actual-root report and signed fixtures remain owed.

## Native ordinary GoBGP assertions pass, cleanup fails

The fresh ordinary scenario passed its initial state, all reload phases, and
wire assertions. The recorded assertion verdict is `<nil>`. The overall job
exited 1 because all five container removal commands exceeded their 30-second
deadlines. This is an assertion pass with a cleanup failure, and the complete
native job remains failed.

The scenario is `bgp-export-reload-gobgp`. Its outer admission command used the
committed launcher while its child rebuilt the working-tree test runner:

```sh
env -u NO_BUILD \
  CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=0 \
  INTEROP_SCENARIO=bgp-export-reload-gobgp \
  ZE_INTEROP_SUFFIX=01a1a907-gobgp-ordinary-recovered \
  bin/builds/20261106T085530Z-926f01f732/le job run \
  label gobgp-native-clock-restored-ordinary-ready-20261106 quiet command \
  ./le --name tooling-resume-01a1a907 test integration interop
```

The private archive contains `bgp-export-reload-gobgp-429617498/` with the proof
and original wire capture. Capture metadata reports 403 packets captured, 405
received by the filter, and zero kernel drops. The applied patch digest matches
`35eed21c951868c3846465c8bbf4fc5c507ffedd93f9c35b4c4e06d03a09b047`.

| Archive artifact | SHA-256 |
|------------------|---------|
| `bgp-export-reload-gobgp-429617498/proof.json` | `b41e9ed8c0c9caa1f9952c07643cb946331ce1ec7e994795c14641cf5cbfaf0e` |
| `bgp-export-reload-gobgp-429617498/wire.pcap` | `5b0d2b5025e968decf9f461aa4bc791c33c4439e2f5cb75e4e40606f45237bcb` |
| `job-gobgp-native-clock-restored-ordinary-ready-20261106-fe673b84.log` | `2feaf9d95e7b48aee59b8ab56ed5f8d0e18ce2ff6785d09c682ad832bc43b90a` |

A later filtered `docker ps` query returned no names. Its suffix and daemon
endpoint binding to this run had not been confirmed, so that observation does
not establish removal of all owned containers. Later exact lifecycle events
identify all five container IDs and owner run `ZEHVVNFUHZL4HB4WFFXACZMOMC`.
They record all five destroy events at 09:28:07–08 UTC, after the owned network's
destroy event at 09:28:04.872 UTC. The private archive retains
`native-ordinary-container-lifecycle.events` and
`native-ordinary-network-lifecycle.events`.

Those positive events establish eventual removal. Their order does not explain
the timeouts. No cleanup fix or clean native pass is claimed. The secondary
scenario's omission and restoration proofs remain owed.

## Actual browser poster inputs retained

MAIN observed `TestWebConfigPosterUsesAuthoredInputs` pass in both the renderer
child and its parent, with no skips. The test ran against the reviewed
three-file Go overlay after refreshing the native demo binaries and renderer
image. Its three captures completed the authored login, edit, save, diff,
confirm, and readback journey. The unchanged assertions require identical bytes
for identical inputs and different bytes when only the hostname changes.

The overlay retained all three actual WebP inputs before the successful cleanup
callbacks. They are in `phase4-save-observation-180032089/` under the private
archive above. Its `manifest.json` records:

| Retained image | Bytes | SHA-256 |
|----------------|-------|---------|
| `first.webp` and `same-input.webp` | 61,282 each | `673b056da7d0671cb629c259c0e294653d0f456d21f4f387897daa4822e3decf` |
| `hostname-only.webp` | 62,834 | `b6653da5578e68e3887addd1bf4d7deb74b5871e87ca7039405f25b7aaea642c` |

The retained log is
`job-phase4-save-owned-goroutines-retained-posters-20261106-c908d5b4.log`.
The failure observer did not run, and the manifest has no failure observations.
This result proves the current poster assertions. It does not explain the
earlier intermittent Save failures or establish an Fsync cause.

## Removal diagnostic ownership control restored

Before native execution, the independent review found a name-reassignment risk
in the diagnostic overlay. Its receipt checks verified a full container ID,
but its later removal still used the name. Reassignment could target another
container between those operations.

MAIN observed both subcases of
`TestOwnedRemovalReceiptSurvivesNameReassignment` fail with the original draft.
The fake probe exercises `ownedRemovalCase.removeReadyContainer` and
`removeUnattemptedContainer`, without Docker or HTTP operations. Both paths
and the observer now use the verified full ID. The unchanged fake assertions
then passed for both `ready` and `setup-cleanup`, with no skips.

The retained baseline and restored logs are
`job-owned-removal-fake-baseline-20261106-e1567663.log` and
`job-owned-removal-fake-restored-20261106-e1567663.log`. The reviewed fixed
source and restored log are also retained in `owned-removal-fake-restored/`
under the private archive above.

This correction applies to the private diagnostic fixture. It does not fix or
explain the native GoBGP cleanup timeout. The inventory repair choice remains
pending, and no further evaluator repair is recorded.

## Native removal controls do not reproduce the timeout

After the fake pass and independent review, MAIN ran the two exact native
roots serially with race instrumentation. Both passed without skips:

| Root | Retained log | Private archive directory |
|------|--------------|---------------------------|
| `TestNativeOwnedRemovalWithoutNetwork` | `job-owned-removal-none-native-20261106-d0687b91.log` | `owned-removal-none-native/case-1222680614/` |
| `TestNativeOwnedRemovalOnOwnedBridge` | `job-owned-removal-bridge-native-20261106-4c91bb5d.log` | `owned-removal-bridge-native/case-2480728650/` |

Each case's `events.jsonl` records raw removal exit 0, the unchanged 30-second
command timeout, and exact-ID absence after removal. The bridge case also
records successful removal of its owned empty bridge. MAIN immediately retained
both cases and logs outside managed scratch, under the private archive above.

These controls did not reproduce the native GoBGP cleanup timeout. They prove
no production correction or cause. The secondary omission result follows.

## Secondary GoBGP omission exposes a stale route

The native `bgp-export-reload-secondary-gobgp` omission run failed in its
`simultaneous` phase. Its runtime patch digest was
`1b7b90213a0875f9a1ef8e8e5cfadb5537bde56a641174651cbb5050871bcc67`.
That candidate retains the BOTH reset and omits the secondary withdrawal
correction. Earlier phases reached the simultaneous check.

At probe 16, secondary FRR still held `10.240.6.0/24` although the phase
required its absence. The independent FRR sink returned an empty route object
for the same prefix. GoBGP's sent bookkeeping toward secondary FRR reported
five prefixes where the receiver contract required four.

The final probe, number 17, also hit the existing 30-second command bound
while querying `10.240.5.0/24`. The failed verdict retains that timeout.
The stale-route observation is a semantic failure, but this run does not prove
that every negative-run control completed cleanly. The capture reports 450
packets captured, 450 received by the filter, zero kernel drops, and stop exit 0.

The private archive retains `gobgp-secondary-cut-native/`, including
`job-gobgp-secondary-cut-native-20261106-fe673b84.log` and
`bgp-export-reload-secondary-gobgp-759034807/` with the proof and wire capture.
The safe projection is
`scratch/gobgp-secondary-paired-proof/cut-actual-safe-projection.json` in the
current session. This is a new successor scenario, not recovery of the missing
historical four-peer proof.

MAIN restored the full patch only after the live file matched the exact cut
candidate. Its digest is again
`35eed21c951868c3846465c8bbf4fc5c507ffedd93f9c35b4c4e06d03a09b047`.
All frozen control hashes matched. The full secondary restoration job
`gobgp-secondary-restored-native-20261106` passed with exit 0 and no cleanup
error. Its assertion verdict is `<nil>`, and all phases passed, including
repeat and restore. The capture reports 454 packets captured, 456 received by
the filter, zero kernel drops, and stop exit 0. The private archive retains
`gobgp-secondary-restored-native/`, including
`job-gobgp-secondary-restored-native-20261106-fe673b84.log` and
`bgp-export-reload-secondary-gobgp-2789421850/` with the proof and wire capture.

Independent review accepted the semantic omission failure and full-patch
positive result, but refused the controlled pair. The Ze and BIRD image IDs
changed between the separate suite runs. Matching Ze binary bytes and FRR
image IDs do not establish equivalent nonproducer filesystems or BIRD inputs.
The old cut-run Ze and BIRD images are unavailable for inspection. This record
does not identify what removed them.

`internal/le/interoplab/docker.go` `Docker.Build` adds the process owner's image
labels to each build. Those labels can change image identity. They do not
prove filesystem equivalence. A source author is preparing one `Suite.Run`
case that shares immutable Ze, BIRD, and FRR image IDs across both runs while
building the cut and full GoBGP images separately. The completed comparison
below resolves that image-identity gap. Wider phase closure remains owed.

## Controlled same-suite secondary comparison

`TestNativeGoBGPSecondarySameSuitePair` passed with race instrumentation in
`job-gobgp-secondary-same-suite-private-inputs-20261106-ecdbab03.log`.
Its one Suite retained identical consumed Ze, BIRD, FRR and sink image IDs.
The omitted-repair case produced the intended stale secondary prefix and sent
count; the full-repair case passed every reload phase. Both captures and both
scenario cleanup paths passed their checks, as did image-retention cleanup.
Independent review accepted this bounded comparison.

The diagnostic used immutable private patch inputs and the unchanged common
assertion body and runtime byte/SHA checks. It wrote no canonical patch or
first-party source. Five default and selected-input provenance test roots
passed first. The actual project patch remained the full `35eed21c…09b047`.

The private archive retains `gobgp-same-suite-private-native/`, containing
`gobgp-same-suite-pair-run-314078206/`, cut proof directory
`bgp-export-reload-secondary-gobgp-668364761/`, and full proof directory
`bgp-export-reload-secondary-gobgp-2540349095/`. The safe report's SHA-256 is
`155d2d837f4b0bb89ea0ebc4eff699885d7b83a83db22264874fd3aa6a1e7064`.
Source snapshots and review are retained separately in
`gobgp-same-suite-private-inputs-source/`.

This proves the new stable-session, non-ADD-PATH IPv4-unicast secondary case.
It does not recover the original four-peer evidence, discharge the fifteen
historical checker proofs, or explain the earlier ordinary cleanup stall.

## Complete committed-fixture baseline

All 151 `TestCommitted` inventory roots executed across three retained runs:
145 passed and six failed. The first batch's ten-minute aggregate alarm
interrupted a root after two seconds; that root passed alone, and the remaining
48 roots then ran separately. The evaluator's 149 source/configuration hashes
matched before and after these runs.

Five failures concern generic template exposure. The sixth is the FlowSpec
family-name projection's ranged-field ownership proof. The private archive
retains all logs and root accounting in `inventory-committed-cohort-baseline/`.
These failures remain open. No evaluator repair was part of this baseline.

## Reboot checkpoint and later scoped results

The reboot handoff of 2026-11-06 is superseded by
[the current continuation record](../../handover/35-tooling-one-off-actual-root-chain-2026-11-09.md);
the old record stays in git history. Its inert source-only diagnostic and
unfinished-source assets now live in
[tooling-one-off-reboot-assets](tooling-one-off-reboot-assets/README.md). Canonical inventory
checkpoint `6abaa27aa6` retains the Git-only evaluator and direct consumers;
it does not close this spec or repair the six failures above.

The explicit type-argument probe subsequently exposed sixteen false-clean cases.
The canonical repair restores all 32 cases and preserves 43 prior roots with
race instrumentation (logs `c464e961`, `cca25cb2`). This is separate from the
generic-template and ranged-field failures. The inferred-interface hypothesis
and revised range tests are source-only, with no runtime result claimed.

The controlled Phase 6 partial-handoff baseline produced the intended RED in
`TestNativeCatalogPartialHandoffCleanup`, log `0519d6ad`: protected child/socket
observations survived old cleanup and the join receipt was EOF. Its test-only
startup-abort envelope was independently approved. The full native observer
still requires the two refused cleanup/lifetime repairs before execution.

The latest documentation check, `0c016d69`, failed with 142 drift issues. Its
source-anchor check passed for 4,234 paths and 772 packages. No complete
repository inventory report or new actual-root performance result exists.

Before reboot, scratch, state, the mixed-tree patch and untracked sources were
also saved privately under `reboot-checkpoint/` in the archive named above.
They contain private evidence and must be transferred separately from Git.
