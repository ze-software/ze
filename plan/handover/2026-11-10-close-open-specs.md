# Handover: closing near-done specs (2026-11-10)

**Current status (2026-11-11):** eight specs closed; four handed back blocked;
access remains claimed by another session. All four handback claims are released.
**Branch:** main. No push was performed by this session; Thomas controls pushes.
**Goal:** close the open specs whose implementation is already committed. The
owner asked that locked or blocked specs not stop progress on the others.

## Model-switch checkpoint, 2026-11-11

### Latest continuation after the switch

This subsection supersedes the checkpoint's remaining-work statements below.
No additional spec closed. All execution stayed in the main thread; no push or
`./le verify` ran.

- Static-BFD guest proof **passed**: exactly
  `test/reload/static-bfd-profile-reload-deleted.ci`, 1/1 tests, 1 phase executed,
  none skipped, 2.5 seconds, command/check/lab exit 0. Kernel `7.3.0-ze`, SHA-256
  `1305faf7734c0c309b0b1d97d37951df625f71e548bbdc04f7ff456f27d4c5e0`.
  Console: `tmp/qemu/docker-lab-console-2884467513.log`.
- Thomas explicitly authorized the sixth review. Independent source review is
  CLEAN. Native recording nevertheless refuses noninteractive authorization.
  Thomas must run
  `bash /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-11-014000/scratch/reload-owner-review-record.sh`
  in his terminal. The script pins the accumulated 39-path native population,
  including the lint cleanup; the sixth review's narrow scope remains the
  diagnostic repair. The native record is still absent at this handback.
- `ea8001c256` fixes both reload scoped-lint findings without changing accepted
  inputs: widen the QoS comparison and remove an unused removal wrapper, keeping
  its exact operations in the existing tests. Six focused race controls passed
  (`job-reload-lint-repair-controls-38d67305.log`). The original broader scoped
  lint reported these two findings, with Darwin and capability passes clean.
  The two-package rerun stopped during planning because foreign
  `internal/le/job/quiet.go` changed; no final lint plan was produced. Retry
  after that owner's edits settle. Reload evidence handback is `80c6383866`;
  its claim is released.
- `e7a77d6e73` commits all four independently approved corpus fixture repairs.
  Actual named runs passed: two reload drafts, doctor, and compiled-out feature
  guidance. The last run supplied the freshly built QEMU stripped daemon through
  PATH; the initial host attempt lacked that binary and did not reach assertions.
  Logs: `job-refusal-reload-drafts-191cf58e.log`,
  `job-refusal-ui-drafts-1cebaff3.log` (doctor pass, missing stripped binary),
  `job-refusal-ui-stripped-retry-c751b774.log` (compiled-out fixture pass).
- The refusal audit reproduced an owned pipeline bug: malformed arguments to
  both `show bgp rib` and `show bgp rib best` produced an error payload with
  status `done`. The repair uses the existing `pipelineRefusal` path. Seven
  focused race units passed after the observed red; independent source review
  and five test migrations are approved. Logs:
  `job-refusal-pipeline-ntp-red-0f4a55fa.log` and
  `job-refusal-pipeline-green-191dfb31.log`. The functional fixture still needs
  execution; its latest attempt hit the launcher panic described next.
  Implementation and blocked handback committed as `820476c6ee`.

**Current shared execution blocker:** the current-source launcher panics before
dispatch because the foreign new `internal/le/test/unit/pkg.go` declares an
83-character Help for `test unit pkg path`, over the 60-character limit.
Other descriptions there also need their owner to check the same contract.
The dirty unit files belong to claimed `spec-agent-economy-2-scoped-test`, held
by `f4d858bd-ad8f-4b19-8a8d-50f67a40ccf9+a438d3331bd8f1e98`. Preserve that work.
Its owner must shorten those descriptions and let the named launcher rebuild.
No functional pipeline, corpus, ADD-PATH or LDP verdict is claimed after this
failure; do not use a stale runner to bypass it.

**Scope question pending:** the refusal spec's owned table includes the NTP
omission, but Known Limitations explicitly excludes it. The closed dependency
spec also left it outside its config-only work. The absent-provider regression
failed, while inactive/measured-provider controls passed. Thomas was asked
whether to repair it in this spec or give it a separate spec; no reply yet.
NTP producer patches are unapplied. The regression was moved out of the package
to scratch `ntp-system-regression.go.txt`; retain it with
`ntp-producer-repair.patch`, `ntp-test-migration.patch`, and `ntp-pending-scope.md`
until ownership is resolved.

Next execution: pipeline functional fixture and scoped RIB package tests;
the config corpus after the four landed corrections; ADD-PATH baseline,
negative-overlay and clean rebuild, then LDP; the AC-7 actual refusal report.
Current audit source/test census is in `refusal-independent-evidence-review.md`
and `refusal-remaining-evidence-audit.md` under the same session scratch.

This section supersedes the old blockers below. The owner requested a model
switch after the continuation's source review and focused repairs. No additional
spec closed. No push or `./le verify` command ran. Unrelated dirty files remain.

- QEMU writes now work: a temporary file was created and removed under
  `/home/thomas/.cache/ze-tmp/main/qemu`. The named launcher also builds from
  current source; the old inventory missing-field error did not recur.
- The static-BFD QEMU command built runtime kernel `7.3.0-ze` and the Ze guest
  image, then failed before guest execution because the CPE extraction missed
  `check_vpn.go`'s `cpeIKEName` caller. The shared `homecpe.CPEIdentity` repairs
  that caller. The current launcher and focused topology tests then built and
  passed. **The static-BFD guest assertion still needs its retry.** Keep the
  new kernel and image caches; they avoid repeating the cold build.
- `dd6864599a` shares the existing home-CPE renderer between the topology lab
  and config corpus, adds a negative rendering control, and records the refusal
  spec's blocked handback. Its claim is released. The documentation hunk landed
  concurrently in `ab4a67d631`. Focused race tests passed for `homecpe`, affected
  topology callers, and `TestSweepRendersHomeCPE`; independent source review and
  moved-test judgments passed.
- Reload review round 5 found a real diagnostic defect: a refused restart can
  follow a plugin exit, but `brokenRollbackLine` claimed the plugin still ran.
  The new renderer regression failed, while the real stopped-process control
  passed. Repair and blocked handback `1cddf638d8` keep refusal classification
  and replace the liveness claim with the live status command. All five focused
  race tests passed after the fix. The claim is released. **A sixth review needs
  the owner's approval; none was performed.**
- Lint-tooling is locked by two existing markers:
  `opencode-lint-backlog-20261102+tooling-fix` and
  `1b60310f-ee84-4530-a4fa-5c2a0c43bc6f`. Access remains claimed by
  `644a4e78-0122-4eeb-8642-eb50638a1641+AccessInteropImplement`.
  Inventory remains claimed by `03d43e18-c488-4511-8040-2650b2986bb3`.
  None of these claims was taken or released. Resolve ownership before edits;
  `specOwner.Claim` does not itself enforce exclusivity between markers.
- Route-policy still has no matched Mac Loc-RIB record. Baseline resolves to
  `d04aabe5e0019f83369767ebf0d111a1806d5798`; implementation is
  `fb94aa04977da6f0cb31f08dd46a8f315af0ffc1`. Run
  `BenchmarkShardedInsertSerial`, `BenchmarkShardedInsertParallel`, and
  `BenchmarkShardedManyWritersDistinctShards` on that pair on the same Mac and
  Go toolchain, retaining full output and hardware metadata. Later Loc-RIB
  changes need reconciliation before claiming current-HEAD performance.

### Successor's execution queue

1. Retry `./le test qemu ci test test/reload/static-bfd-profile-reload-deleted.ci`.
   Finish the reload spec's recorded scoped lint/checks. Keep it open until the
   owner authorizes the sixth review and that review passes.
2. Run the four approved fixture drafts through job admission and the current
   named runner: `test bgp reload --draft dns-source-reload
   reload-parse-error-single-prefix`, then `test ui --draft
   config-compiled-out-feature-names-tag doctor-procfs-netlink`.
   `test reload` is not a registered command: the attempted spelling exited 2
   with usage and ran no tests. Promote only after actual passes, then run the
   named `TestRepositoryConfigsValidate` corpus test once.
3. Run ADD-PATH baseline, negative overlay, and clean rebuild; then LDP. The
   refusal spec records the exact scenarios and overlay. The existing overlay
   still changes only `nlri.WriteNLRI` to omit identifier zero. Preserve effective
   Go flags when adding it; this continuation's `go env GOFLAGS` was empty.
   Neither interop scenario ran in this continuation.
4. Recheck foreign ownership before resuming lint-tooling or access. Obtain the
   matched Mac measurements before route-policy closure.

### Retained evidence and drafts

Session: `01a1c842-0a58-7752-bbff-2a3f1633b66f`.
Scratch: `tmp/session/2026-11-11-014000/scratch/`.
Named metadata binary: `bin/le-close-open-nov11/le`; use the launcher with
`--name close-open-nov11` when execution must read current source.
All tests, builds and formatting ran in the main thread.

Observed race logs in that scratch directory are
`job-refusal-cpe-focused-c8e0a05a.log` (pass),
`job-reload-refusal-red-139ad124.log` (expected renderer failure and passing
stopped-process control), and `job-reload-refusal-green-0bb88bcd.log` (all five
named tests pass). The canonical test sources and commands are in the specs;
these session logs aid diagnosis and are not a substitute for retained source.

Keep `test/draft/reload/{dns-source-reload,reload-parse-error-single-prefix}.ci`
and `test/draft/ui/{config-compiled-out-feature-names-tag,doctor-procfs-netlink}.ci`.
Three add expected-refusal metadata; the doctor fixture changes reserved table
3001 to table 100. Runtime assertions are preserved. Native independent
approvals remain in `tmp/commit-weakening-approved-8b5d7643.md` for this session.
Remove draft copies only after their tested promotion. Keep the prior ADD-PATH
overlay until its negative/clean proof completes. No full-tree compile, lint,
documentation gate, or verification ran; the evidence is the scoped runs above.

## Status

### Done (committed)

| Spec | Closing commits |
|------|-----------------|
| spec-lint-backlog-9-config-and-forwarding | `dca392f162`, `74481d95fb` |
| spec-lint-backlog-2-bgp-plugins | `1f4d542608`, `68d7c9e541` |
| spec-lab-docker-cleanup-and-go-test-containers | `e6ce0f9c40`, `091c33f079` |
| spec-lint-backlog-1-bgp-engine | `7c2e0fbdba`, `c7d8391a17` |
| spec-lint-backlog-7-system-and-routing-plugins | `b972ef8113`, `673bcaf92d` |
| spec-lint-backlog-5-hub-and-policy | `0fa6d5f922`, `b1065c830f` |
| spec-lint-backlog-6-access-and-appliance | `7eb86b64a2` (interim), `1b98435bc8`, `84b796f5e3` |
| spec-rfc-verdict-fix-ike-eap | `03447bf159`, `44797abe31` |

IKE/EAP closed independently after `e62246b71e` refreshed 65 proof covers in
30 JSON files. The postcommit RFC check has no owned-stem violations; 63 unrelated
global violations remain. EAP halts prove dependency only. The remaining
selector and Method-Id/Session-Id work belongs to architecture-gaps AC-13 and
RFC9190 AC-10. No RFC9190 enrollment or support upgrade was claimed.

Tooling fix landed on the way, because it blocked every lint closure:
`b032323956` "le: a scoped lint judges only its own inputs". A scoped
`./le go lint run scope` used to abort on any edit anywhere in the tree; it now
refuses only when a path its own population reads changes by content
(`job.ChangedInputs`, `lintPopulation` in `internal/le/go/lint/planinputs.go`).
Independent review CLEAN at round 3; ze-test-judge approved the seven parity
tests whose seam changed.

Owner decisions taken this session:
- lab-docker Ctrl-C proof of `verify evidence release-candidate`: the three
  fake-runner tests were accepted as AC-8 evidence (test-level, not live daemon).
- lab-docker review: a fresh one-round review of the closure files, not a
  re-record of another session's round 10.
- lint-6: the rangeValCopy findings in `ike/engine/unmatched*.go` were fixed
  before release (not left to spec-ipsec-spd-final-discard-vpp-ospf), with
  RFC4301-4.4.1-3 re-recorded from hand-written breaks.

### Repairs completed; closure still blocked

Reload registry isolation landed in `82abdd598d`; SDK rollback state repair
landed in `f768ab3ca7`. VPP alias lookup, monitor teardown and lifecycle harness
repairs landed in `a82960a6e8`, `e4e5f8da3a` and `844f42a7d5` respectively.
The final real VPP run passed five active scenarios; LCP was explicitly skipped
because the image lacks it. The empty-egress no-op mutation failed as required,
then the clean run exited zero. AC-18b is met. Evidence:
`tmp/session/2026-11-10-224742/scratch/reload-vpp-final-clean.log`.
Scoped VPP backend lint passed: native completion record
`tmp/.ze-jobs/lint.2.3953053966.rc` contains zero (2026-11-11 01:14:53 UTC).
The full lint log was removed when the job released; broader tooling lint and
independent review remain owed.

All tests, lint and proof execution ran in the main thread. One agent metadata
command accidentally triggered a named-launcher build collision; subsequent
metadata calls used the existing binary directly, and the main thread retried
the build. Agents continued source work while main-thread jobs ran. No push was run.

### Blocked or locked

| Spec | Current evidence and what clears it |
|------|-------------------------------------|
| `plan/release/validation/spec-lint-backlog-3-test-tooling.md` | Handback `dc7b73d88c`; claim released. Repairs and independent source review are complete. Final scoped lint and structural/documentation checks need the claimed inventory build repaired first. |
| `plan/release/fixes/spec-refuse-instead-of-defaulting.md` | Handback `a261740ab2`; claim released. CoS and forwarding-writer refusals are repaired. Actual absent-CoS CLI, affected Go consumers and all eight parser cases passed. Interop staging cannot build the shared `le` dependency; no ADD-PATH scenario or negative control ran, and LDP remains unrun. The config corpus is separately red with eight errors across five non-CoS fixtures. |
| `plan/post-release/features/spec-route-policy-0-umbrella.md` | Handback `a32bb92747`; claim released. A-4 requires matched Loc-RIB performance results on the owner's Mac: baseline `fb94aa0497^` versus implementation `fb94aa0497`. Preserve revisions, Go version, hardware and full output. The SDK transport benchmark does not answer this obligation. Final review and closure remain owed. |
| `plan/release/fixes/spec-plugin-reload-applies-or-refuses.md` | Handback `dbf47bb319`; claim released. Real VPP proof is complete. Static-BFD guest proof is blocked: `tmp/qemu` resolves under `/home/thomas/.cache/ze-tmp/main/qemu`, outside writable roots. Grant write access to `/home/thomas/.cache/ze-tmp`, then rerun the named guest fixture. Remaining scoped checks and independent review stay owed. |
| `spec-rfc-verdict-fix-access` | Locked by `644a4e78-0122-4eeb-8642-eb50638a1641+AccessInteropImplement`. Its owner holds the six lab acceptance runs and proof work; this continuation did not take the claim or alter its source. |

The shared build blocker is `internal/le/repo/inventory/source.go`: `Snapshot`
lacks `assemblyPredefines`, `headerPresence` and `localHeaders`, and
`sourcePackage` lacks `assembly`. The readers in `assemblyheaders.go`,
`assemblylinkage.go` and `assemblylocalinclude.go` fail compilation.
`spec-inventory-fast-registry-checks.md` is claimed by
`03d43e18-c488-4511-8040-2650b2986bb3`; that session must restore the coherent
dependency before the blocked native checks resume. No stale image or
`NoBuild` bypass was used.

Refusal repair commits are `a6c70ec923`, `2b90f8864b`, `528fdc8fdc` and journal
update `4a66448ad9`; concurrent `12720abe4c` carried its two inventory tests.
The blocked spec names the exact logs, corpus findings and remaining
ADD-PATH baseline/negative/clean-green and LDP commands.

The static-BFD QEMU attempt could not write `tmp/qemu/linux-amd64/bin/ze`;
`tmp/session/2026-11-10-224742/scratch/reload-static-bfd-qemu.log` records the
permission failure. The host run lacked `CAP_NET_ADMIN` and skipped, so it does
not substitute for `test/reload/static-bfd-profile-reload-deleted.ci` guest proof.

### Earlier handover findings, not revalidated by this continuation

These are the original handover's other-session findings. They are retained as
historical context, not current test verdicts or new closure blockers.

- 22 lint findings in IKE (14 hugeParam on a 288-byte peer type, 5 intrange,
  3 goconst) come from another session's uncommitted RFC 4301 DF/ICMP-selector
  work in `internal/component/ike/ipsec/` and new untracked files. That session
  owes its own lint before committing.
- `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage` fails inside another
  session's in-progress RFC 4945 EKU edit of `remote_cert_eku_test.go`.
- Journal rows written this session, each owned by an open spec:
  `TestBridgeSetSTPKernel` NUL byte (parser-refuses-a-form-its-producer-emits,
  owned by spec-doctor-coverage-2-home-cpe-batch);
  `TestPPPoEClientAlreadyAbsentPair` (package-red-not-reproduced, owned by
  spec-ppp-l2tp-aaa-one-off-defects); kernelbuilder raw `docker`
  (gate-red-where-nothing-blocks-on-it, owned by spec-tooling-one-off-defects).
- `TestOrdinaryRefreshDoesNotDoublePrepend` is a deliberate red owned by
  `plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md` (ready).
- `./le spec citation` is red on `spec-oauth-login-0-umbrella` citing the
  missing `spec-mgmt-oauth-bearer-and-web-sso`, another session's.

## Lessons that apply to the next closures

- Every lint-backlog scope gained new findings from later commits by other
  sessions; fix them as closure work. golangci-lint caps repeats at 10 per
  linter (`.golangci.yml` `max-same-issues`), so a fixed batch uncovers the next.
- A scoped lint still refuses, correctly, while another session edits a package
  the scope depends on. Wait for a quiet tree (no dirty Go file touched for a
  few minutes) and run it then; don't retry in a loop.
- `./le --name session ...` builds le from the working tree, so another
  session's partial `internal/le` edit can block it. An existing named binary
  can read metadata, but cannot prove that the current source builds or replace
  an owed native check with a stale result.
- Agents use the existing named binary directly for metadata; builds belong
  to the main thread. Concurrent named-launcher calls collided on the same
  PID-2 build output in this sandbox. Do not start a second launcher build.
- Closure agents run no tests; the main thread runs every test, lint and
  record, one CPU job at a time, and sends results back by message.
- `plan/release/validation/spec-lint-backlog-0-umbrella.md` collects one row per
  child; commit only your own row with `hunks`.
- A commit refused on `tmp/commit-weakening-approved-<ns>.md` or
  `tmp/commit-rfc-approved-<ns>.md` "has no table header" means a stale
  approval file from an older format in that namespace; if its approvals belong
  to a commit that already landed, move it aside.

## Files already handled (no need to re-read)

- `internal/le/go/lint/planinputs.go`, `internal/le/job/inputsnapshot.go`: the
  scoped-lint population and content-change guard, committed in `b032323956`.
- `docs/contributing/running-commands.md`, `docs/architecture/testing/verify-freshness-scope.md`:
  updated for that guard.

## Then

Resume reload after the QEMU scratch path is writable, retaining its completed
VPP evidence. Resume lint3 and refusal after the inventory owner repairs the build; resume route-policy
after the owner supplies the matched Mac evidence. Leave access with its
current owner. Run independent closure where the spec requires it, with every
test, lint and proof run in the main thread. Keep blocked specs open.
