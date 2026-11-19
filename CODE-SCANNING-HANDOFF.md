# Code-scanning work and scope handoff

Recorded 17 November 2026. Thomas instructed this session to stop the broader
BGP work, keep the changes, and record what happened. That work exceeded the
code-scanning task. It must not resume under this task without new authorization.
The continuing scope is **only security alerts #442, #418 and #341**.

## Security changes already made

| Alert | Local change | Commit |
| --- | --- | --- |
| [#442](https://github.com/ze-software/ze/security/code-scanning/442) | Shared CLI file loading enforces daemon filesystem authority and registered configuration roots, excludes private store files, opens a descriptor-bound regular file without following replacement symlinks, and limits reads to 4 MiB. Local-owner and browser import behavior have separate authority paths. | `a7bca7dd9a5c4b0009b87543fb6d34fdb089a559` |
| [#418](https://github.com/ze-software/ze/security/code-scanning/418) | Configuration tail selection scans the existing text rather than allocating storage from the requested count. | `a7bca7dd9a5c4b0009b87543fb6d34fdb089a559` |
| [#341](https://github.com/ze-software/ze/security/code-scanning/341) | JSON first/last selection allocates according to actual available rows, preserving arrays, envelopes and identity maps for oversized counts. | `500e39d8c7a23c1920761e3830c6a68c99e5a325` |

The CLI commit also includes regression tests, documentation, relative stored-name
handling, and demo configuration staging required by the file boundary.

Validation included the complete command and terminal-demo packages, focused CLI
tests, deterministic path-substitution and count-overflow regressions, real SSH
and HTTPS import checks, and 80 successful real-SSH boundary executions.
The recorded count-coverage selection passed 133 named tests/subtests. These are
local results, not evidence that GitHub has scanned the commits.

This session also individually dismissed 171 reports on GitHub: 145 false
positives, 22 cases used in tests, and four cases of protocol-required
cryptography. Exact reasons and dispositions are retained in the evidence below.

The last verified GitHub snapshot (17 November, approximately 14:30 UTC) still
had exactly #442, #418 and #341 open against scan commit
`e7e590078670a41bbfa7f917fafb021d8d72652f`. This session has not pushed the fixes.
Fresh CodeQL confirmation remains outstanding.

After the scope correction, only the three security regression groups were
rerun. All **12 roots and 36 subtests passed** across the command and CLI
packages, using the native race-instrumented runner. No further security source
edits were necessary: the two commits above already contain the fixes.
The exact result is `tmp/session/2026-11-15-120938/scratch/job-unit-pkg-db1c122b.log`
(SHA256 `211d6d2ffe1e5b16f6b07c56388481254ef13857fb7e895a8583cdf5029dd518`).
The live remote `main` still points to the old scan commit. Local `main` also
contains unrelated outgoing commits, so this session has not published it.

## Broader work performed beyond the requested scope

### Committed corpus/test changes

Commit `9045adee3c5ab7bc399993132f525d631fd11cac` changes:

- `internal/component/config/cli/validate_corpus_test.go`: preserve typed load
  errors and distinguish host-only limitations from actual configuration errors.
- `internal/component/config/cli/validate_corpus_home_cpe_test.go`: select and
  restore the Linux schema for the Linux CPE fixture.
- `internal/component/config/cli/validate_corpus_host_test.go`: add host/refusal,
  mixed-error, candidate-preservation and diagnostic-order controls.
- `docs/architecture/testing/runner-architecture.md`: document that handling.

Six focused controls passed and scoped lint reported zero issues. The complete
config/CLI package was not rerun after all other owners' fixture migrations.

### Uncommitted BGP fixture changes left in place

These five files were changed by this session and are deliberately preserved:

| File | Change |
| --- | --- |
| `internal/component/bgp/reactor/rfc7705_live_behavior_test.go` | `lowLiveRouter` now declares its default selecting-RIB attachment explicitly through the configuration parser, preserving existing same-name caller bindings and the receive/send tokens. |
| `internal/test/fixture/policy_final_reload.go` | Diagnostic capture/check of the installed export policy before and after rejecting reload; retains all original wire/session assertions. |
| `internal/test/fixture/rfc8212_reload.go` | Drained scenario boundaries, peer diagnostics, and a DOWN-event check scoped to the reload scenario. |
| `internal/test/fixture/rfc8212_reload_observe.go` | Bounded raw state transcript, lifetime DOWN counter and scenario-window checks. |
| `internal/test/fixture/rfc8212_reload_phase_test.go` | Native-event controls retaining startup removal evidence and refusing a DOWN inside the scenario window. |

Independent judgments and native approval records were obtained for the fixture
changes. This does **not** establish that all affected tests pass.

Three temporary production logging probes in `peer_settings_apply.go`,
`policy_reload.go` and `reactor_api_advertised.go`, and two private logging options,
were removed using the exact inverse of this session's hunks. Original hashes
were verified. No production BGP algorithm fix was applied by this session.
No destructive Git command or restoration from HEAD was used.

## Unresolved problems and actual results

### Final-only export-policy reload restarts a BGP session

The `bgp-policy-final-reload` draft still fails: changing export final accept to
reject restarts peer 127.0.0.2 instead of withdrawing routes on the original
session. The full captured log reports:

```text
peer restart required phase=operation changed=DisseminationOwner
```

The successor session is already rejecting when later hot-swap classification
runs. This is not evidence of a mutable policy alias. Source tracing found that
the per-operation path compares a pure candidate's unresolved owner with live
runtime ownership; full-section reconciliation already resolves that authority.

A corrected production proposal and separate regression-only patch were prepared
and independently source-reviewed. **Both remain unapplied and untested.**
They also address owner preparation in residual `ReconcilePeers` verification.
An earlier private proposal was rejected on that verification gap and superseded.
No claim is made that an entire native transaction bypassed other verifiers.

### Explicit selecting-RIB fixture: 46 of 52 roots pass

After the fixture attachment change, these original failures passed individually:

- `TestPolicyFinalReloadRetainedRoutes`, including its first restoration.
- `TestRSAnnouncementFirstArrivalConverges`, including all eight subcases.

The subsequent 52-root qualification executed all 52 and returned **46 PASS,
6 FAIL**. Assertions were not weakened. Remaining failures:

- `TestRFC7311ReceivedRecursiveAIGPChangeReachesWire`: both recursion depths
  observed two identical control UPDATEs where one unchanged history was expected
  (`rfc7311_recursive_readvertise_test.go:217`).
- `TestRFC7947ReceivedLoopDenialRetained`.
- `TestRFC7947ReceivedImportFinalDenialRetained`.
- `TestRFC7947ReceivedMixedDenialRetainsAnnouncement`.
- `TestRFC7947ReceivedDeniedReplacementSupersedesOldBytes`.
- `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial`.

The five RFC7947 failures concern missing denied routes or retention of the
original pre-policy attributes. Their cause was **not established** before work
stopped. A proposed explanation that the parser replaced other caller bindings
was checked and ruled out: `parseProcessBindingsFromTree` appends bindings.
Whether the failures were introduced or exposed by the fixture change has not
been demonstrated with a controlled baseline comparison.

### RFC8212 observer repair passed its scoped checks

The original observer counted a startup removal of an earlier Peer incarnation
as a reload-generated DOWN. Raw identities and drained event boundaries proved
that distinction. The repaired observer retains the startup transcript and
requires zero DOWNs within the actual scenario window.

- Native-event positive/negative unit controls passed.
- The original owner-failure draft passed, then passed all 80 stress invocations:
  80 zero exits, 80 named passes and 720/720 verification steps passed.
- The other six affected canonical RFC8212 cases each passed once.

Canonical fixture promotion and publication of these broader changes were not
completed by this session.

### Other owners' work

The advertiser task owns the canonical receive-grant migration (14 previous
corpus findings across 11 CI fixtures). This session did not promote those files.
The web task owns editor correctness publication and its lint fixes. That task
reported a fresh unchanged four-branch `TestSessionEmptyEntryCommit` PASS on
Ze Linux (`job-web-linux-empty-entry-proof-020bac90.log`); publication and final
lint were still with that owner. These are separately owned results, not changes
or completion claims by this session.

## Evidence and retained temporary work

The current managed evidence directory is:

```text
/Users/thomas/.codex/state/plugins/codex-security/scans/main/artifacts-1f914f35b697656beec292c2a10987c22625134fa9ce3b6e200e461c3b7c488c/artifacts/code-scanning-review/
```

It contains `assessment.md`, `verification.json`, `dismissals.json`, the original
intake/triage, security regression/coverage evidence, `helper-judgments.json`
(52 individual source judgments and scoped native receipts), and
`rfc8212-stress-evidence.json` (all 80 invocation results).

Current private proposals are under:

```text
tmp/session/2026-11-15-120938/scratch/reload-policy-diagnostic-proposal/
```

- `low-live-router-source-grant.patch`: applied fixture proposal, SHA256
  `8980f0441bdd04b27f154c44c5744100e42fc05a3cd0b4924a9be4352253de86`.
- `rfc8212-phase-window.patch`: applied observer proposal, SHA256
  `db6687b21e4141d28706ce885da06d987823aa268c643c37deb72f83031dc734`.
- `dissemination-operation.patch`: **unapplied** production proposal, SHA256
  `2e83ef2c88fde00ab53e040fb44d444ffd750fe667fbdbf731507944da36a3d2`.
- `dissemination-operation.regression.patch`: **unapplied** tests, SHA256
  `7356ce00ecbd213bb953a08ffba2fc36be02580bd736dbc275bbc84ff161fe9a`.
- Baseline metadata and the exact temporary logging inverse accompany them.

Native logs under `tmp/session/2026-11-15-120938/scratch/` include:

- `job-unit-pkg-62fd222d.log`: authoritative 52-root result and six failures.
- `job-unit-pkg-94530828.log` and `job-unit-pkg-de29036f.log`: the two original
  fixture failures passing individually.
- `job-unit-pkg-365f3913.log`: RFC8212 native-event controls.
- `job-rfc8212-six-canonical-once-57b35d35.log`: six canonical named passes.
- `reload-policy-restart-reason/3-bgp-policy-final-reload/client-stderr.log`:
  complete restart-reason capture. Native failure summaries elide the middle
  of daemon output; use the saved full capture for diagnosis.

These proposals and minimal failure captures are retained because Thomas asked
to preserve and record this work. Remove them when an authorized owner has
adopted or rejected the changes and durable resolution evidence supersedes them.
Do not automatically apply proposals or resume the broader work from this file.
Task-owned disposable commit helpers were removed (about 173 MiB in this
follow-up, in addition to the earlier approximately 176 MiB cleanup). Shared
builds and other sessions' files were preserved.
After the focused security recheck, the completed task-owned named launcher and
private daemon/helper binaries were also removed after checking that none was
running (approximately 431 MiB). Source changes, proposals and retained evidence
were kept as instructed.

## Continuing authorized scope

Only #442, #418 and #341 are now in scope. Refresh their GitHub status, verify
the existing local fixes against those reports, and make any necessary scoped
corrections. Remote closure requires a CodeQL run of the published fixes.
The repository's owner-only push preparation must be followed; no direct push
or manufactured interactive terminal is permitted.
