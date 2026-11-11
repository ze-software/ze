# Handover: closing near-done specs (2026-11-10)

**Current status (2026-11-11):** eight specs closed; four handed back blocked;
access remains claimed by another session. All four handback claims are released.
**Branch:** main. No push was performed by this session; Thomas controls pushes.
**Goal:** close the open specs whose implementation is already committed. The
owner asked that locked or blocked specs not stop progress on the others.

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
