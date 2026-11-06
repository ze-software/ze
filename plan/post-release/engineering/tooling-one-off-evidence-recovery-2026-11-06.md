# Tooling evidence availability after the November 6 cleanup

Earlier session raw evidence became unavailable during an automatic cleanup run
while work on the
[tooling one-off defects spec](spec-tooling-one-off-defects.md) continued.
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
decision and trust contract remain recorded in
[INVENTORY-EXTERNAL-EFFECTS.md](../../../INVENTORY-EXTERNAL-EFFECTS.md).

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
the selected timestamp controls. A complete native GoBGP wire pass remains owed,
and this run supplies no verdict for the inventory method census.

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
`collectMethods`, in `internal/le/repo/inventory/externalreflectiontypes.go`.
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
GoBGP scenario had no result when this record was added. No native pass is
claimed.
