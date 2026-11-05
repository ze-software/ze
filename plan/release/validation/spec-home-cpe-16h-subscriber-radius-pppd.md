# Spec: Home CPE 16h subscriber RADIUS pppd alignment

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling, docs |
| Priority | P2 |
| Release reason | Subscriber RADIUS validation uses a stale pppd pin despite a paired-image update requirement. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/1 |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-8 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Align the paired subscriber RADIUS and L2TP LAC images with the latest stable upstream pppd available when implemented.
Verify downloads, hashes, build inputs and runtime plugin paths, then rebuild both images and run their affected scenarios.
The original split recorded existing pins without rerunning images or scenarios.
Central Verification and the closure audit below record the subsequent source,
archive, build and live-session evidence.

## Required Reading

- [x] `ai/META-DECISIONS.md`, third-party lab release policy.
  → Constraint: both labs use the latest stable upstream release rather than a distribution's stale package.
- [x] `docs/architecture/testing/interop.md`, FreeRADIUS subscriber suite; `docs/labs/l2tp-interop.md`, peer-source basis.
  → Constraint: real PPP kernel sessions and independent RADIUS accounting/authorization evidence remain connected.
- [x] Upstream release metadata, archive build instructions and source for the chosen version.
  → Constraint: check release status at implementation time and verify the exact bytes and plugin ABI paths being installed.

## Current Behavior

These rows describe the pre-fix images at the original split. The committed
implementation aligns both images, as the closure audit below records.

| Source checked | Behavior |
|----------------|----------|
| `test/interop-radius-subscriber/Dockerfile.lac` | Pins PPP_VERSION2.5.4 and a Samba archive hash; uses `configure`; comment says to change both LAC Dockerfiles together. |
| `test/interop-l2tp/Dockerfile.lac` | Pins PPP_VERSION2.5.5 and the official GitHub tag archive hash; installs autotools dependencies and runs `autogen.sh`; documents `/usr/lib/pppd/2.5.5`. |
| Both Dockerfiles | Build xl2tpd1.3.20 and derive runtime shared libraries with scanelf. |
| `internal/le/interoplab/radiussubscriber/radiussubscriber.go` and `internal/le/interoplab/l2tp/l2tp.go` | Their lab image declarations select each suite's own LAC Dockerfile. |

## Existing Mechanisms

| Mechanism | Where | Direction |
|-----------|-------|-----------|
| Source-built hash-pinned LAC images | The two `Dockerfile.lac` files | Align the existing build inputs rather than install a second pppd. |
| Runtime dependency discovery | scanelf stages in both Dockerfiles | Preserve derived shared-library dependencies. |
| Real subscriber session proof | Existing L2TP and RADIUS subscriber suites | Reuse the scenarios to prove the rebuilt images operate. |

## Data Flow

Verified upstream release/archive → hash check → build pppd/plugins and xl2tpd → runtime library discovery → suite LAC image → actual PPP/RADIUS sessions.
Trace every suite-owned plugin-path consumer so the version update cannot leave a stale ABI directory.

## Draft Acceptance Criteria

| ID | Input / condition | Expected behavior | Permitted survivors and why |
|----|-------------------|-------------------|-----------------------------|
| AC-1 | Upstream stable release checked at implementation time. | Both LAC builds use that pppd release with verified archive hashes and matching documented versions. | Existing xl2tpd behavior and suite-specific tools remain available. |
| AC-2 | Build both images without reusing the obsolete pppd layer. | Both builds succeed; installed pppd identifies the selected version and loads its PPPoL2TP plugin from the actual ABI path. | Real LAC dial/session establishment remains supported. |
| AC-3 | Execute affected subscriber RADIUS scenarios. | PPP authentication, traffic, accounting, CoA and Disconnect assertions remain valid on the rebuilt peer. | Supported RADIUS decisions and session control continue to operate. |
| AC-4 | Execute affected L2TP policy scenarios. | MRU/CHAP policy checkers run against the rebuilt pinned peer and preserve their positive traffic and negative-auth controls. | Valid CHAP sessions survive; disallowed PAP remains refused. |
| AC-5 | Tampered/unavailable archive or missing plugin. | Build or preflight fails with the cause visible instead of silently using a different installed version. | A verified archive and usable plugin still produce working images. |

## 🧪 TDD Test Plan

| Planned carrier / check | Proof |
|------------------------|-------|
| Paired Docker builds with retained version/hash output | Reproducible source selection and working build procedure. |
| Runtime version/plugin load smoke check in each image | Installed release and ABI-path correctness, not comments alone. |
| `interop-radius-subscriber` affected scenarios | Real subscriber traffic, accounting and dynamic authorization after rebuild. |
| `l2tp-lns-mru-xl2tpd` and `l2tp-lns-chap-only-xl2tpd` | Rebuilt peer preserves policy evidence and working sessions. |

Do not invent protocol discrimination from an image version string. Record the actual rebuilt image identity and each scenario's result; retain existing semantic discrimination obligations.

## Files to Modify

- `test/interop-radius-subscriber/Dockerfile.lac` and `test/interop-l2tp/Dockerfile.lac` — synchronized source pins, build steps and runtime paths.
- Any version-specific plugin-path consumer found by source census in both suites — convert with the image update.
- `docs/labs/l2tp-interop.md` and affected `docs/architecture/testing/interop.md` version/evidence paragraphs — source-correct versions and build procedure; update corresponding wiki coverage.

## Risks & Assumptions

| ID | Risk / assumption | Early signal or basis | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| A-1 | The currently chosen archive source remains available. | Existing download routes differ. | Confirmed: official ppp digest reproduced centrally. Successful paired builds verify both archive pins before extraction, followed by passing version/plugin probes. |
| R-1 | New archive lacks generated configure or changes ABI paths. | Build failure or plugin load failure. | Read release instructions, update build inputs and measure installed paths. |
| R-2 | One suite remains on an old image layer. | Runtime version differs despite matching Dockerfile pins. | Rebuild both and retain image/version evidence before semantic tests. |

## Scope and Next Action

This spec owns peer-image alignment for 16e/16f/16g, not their checker defects or production MRU policy.
Implementation is committed in `3cc99259b0`. The complete paired-image source
census and independent closure audit are below. Native review is recorded
and checked CLEAN. The two closure commits belong to the coordinator.

## Implementation Handoff (2026-11-04)

Historical author handoff. Central Verification supersedes its unrun obligations.

The owner requires central execution: this issue agent authored files and
performed static inspection only. No build, test, formatter, daemon, Docker
command or commit ran here. The main thread owns all execution and review.

| Item | Implementation or retained obligation |
|------|---------------------------------------|
| Upstream release check | The upstream releases page marks v2.5.5 latest: `https://github.com/ppp-project/ppp/releases`. Checked 2026-11-04. |
| Paired image fix | Subscriber RADIUS now uses L2TP's v2.5.5 official tag archive, the existing hash and autotools build procedure. Both retain xl2tpd 1.3.20 and scanelf-derived runtime libraries. |
| Plugin consumers | Static search of both suite trees and their native lab packages found no version-specific consumer besides the L2TP Dockerfile comment; the RADIUS Dockerfile now documents the same ABI directory. Runtime measurement remains owed. |
| Drift regression | New `internal/le/interoplab/lac_images_test.go`, `TestLACPeerReleasePinsAgree`, reads both real Dockerfiles through existing `buildArgument`; compares pppd/xl2tpd release/hash defaults and rejects missing, duplicate or malformed hash declarations. It does not claim network or protocol proof. |
| Documentation | `docs/labs/l2tp-interop.md`, peer-source subsection, describes the paired build and distinguishes historical L2TP results from RADIUS evidence still owed. |
| Archive proof owed | Main thread reproduces SHA256 `5d27c1c048480139de3ba7cdca89cf455da312930473b25205a994e5fa20ef4f` from `https://api.github.com/repos/ppp-project/ppp/tarball/v2.5.5`; this agent reused the existing L2TP pin rather than invent a digest. |
| Runtime proof owed | Rebuild both LAC images, measure pppd version and PPPoL2TP plugin ABI path, then run subscriber RADIUS scenarios and both L2TP policy scenarios. |

### Wiring Test

| Entry point | Producer | Authored test |
|-------------|----------|---------------|
| Suite-owned LAC Dockerfile declarations | Actual paired `ARG` instructions parsed by existing `buildArgument` | `TestLACPeerReleasePinsAgree` |

### Central Verification Required

| Check | Target |
|-------|--------|
| Scoped Go test | `./internal/le/interoplab`, test `TestLACPeerReleasePinsAgree`, using the tagged race recipe under `./le job run`. |
| L2TP peer build and policy runs | Named-build deployment action `docker-l2tp-ppp`, selectors `l2tp-lns-mru-xl2tpd` and `l2tp-lns-chap-only-xl2tpd`. |
| Subscriber RADIUS build and scenarios | Named-build integration action `interop-radius-subscriber`; include authentication, accounting, CoA and Disconnect scenarios. |
| Image evidence | Retain image identity, `pppd --version`, loaded PPPoL2TP plugin and actual plugin ABI directory for each rebuilt peer. |
| Documentation | `./le doc check all`, plus corresponding wiki synchronization in the main thread. |

Archive and runtime assumptions remain unvalidated until the central thread
reports their measured results. This handoff is ready for review, not closure.

### Runtime Smoke Authored for Central Execution (2026-11-05)

`internal/le/interoplab/lac_images_runtime_integration_test.go` adds
`TestLACPeerRuntimeVersions` under the `integration` build tag. It uses
`NewDocker().RunOneShot`, which labels, names and removes every probe through
the native lifecycle. No bare unowned container command is introduced.

The test reads each paired Dockerfile's `PPP_VERSION`, probes the suite-owned
physical tags `ze-l2tp-lac` and `ze-radius-subscriber-lac`, and checks both
version output (including stderr) and the exact PPPoL2TP plugin path. It uses
no network or image pull, has a two-minute context bound, and fails rather
than skips when a runtime probe is unavailable. Root runs this one test after
the native suites rebuild both images; real subscriber scenarios remain the
proof that the installed plugin loads and carries PPP traffic.

This issue agent did not compile, format or run the new test. Central execution
and the resulting image/version/path evidence remain the main thread's work.

## Central Verification (2026-11-05)

This result supersedes the author's unrun handoff above. The coordinator
rebuilt the native images and ran `TestLACPeerReleasePinsAgree` and
`TestLACPeerRuntimeVersions`. Both `ze-l2tp-lac` and
`ze-radius-subscriber-lac` report pppd 2.5.5 and contain
`/usr/lib/pppd/2.5.5/pppol2tp.so`. The official 2.5.5 archive reproduced
SHA256 `5d27c1c048480139de3ba7cdca89cf455da312930473b25205a994e5fa20ef4f`.

The full native subscriber-RADIUS suite passed all four registered scenarios:
Framed-Protocol rejection, accounting restart, CoA disconnect and duplicate
CoA. The L2TP MRU and refusal scenarios also passed with the rebuilt peer.
The issue agent authored the image and runtime-version checks without
compiling or executing them.

Evidence: `job-cpe16-peer-runtime-version-3461699d.log`,
`job-cpe16-live-radius-subscriber-d83f8741.log` and
`job-cpe16-final-l2tp-live-e6b8eb67.log` under
`tmp/session/2026-11-04-222204/scratch/`. The remaining step for this spec is
the separate closure review; no unpublished or skipped run is counted green.

## Deliverables Checklist

| Deliverable | Verification | Result |
|-------------|--------------|--------|
| Current stable pppd in both LAC builds | Compare actual Dockerfile defaults and upstream release metadata. | Done: both select 2.5.5. The coordinator observed the official latest-release URL resolve to v2.5.5 on 2026-11-05. |
| Verified downloads and working build procedure | Read both chained download/checksum/build instructions and upstream `autogen.sh`, `configure.ac` and PPPoL2TP `Makefile.am`. | Done: matching ppp/xl2tpd hashes, generated autotools configuration and versioned plugin installation. |
| Actual rebuilt version and plugin | Inspect `TestLACPeerRuntimeVersions` and its central output. | Done: both subtests PASS, report pppd 2.5.5 and require `/usr/lib/pppd/2.5.5/pppol2tp.so`. |
| Preserve complete subscriber behavior | Read native suite declarations and actual scenario output. | Done: four subscriber-RADIUS scenarios and both L2TP policy scenarios PASS. The permitted CHAP carrier establishes useful traffic. |
| Prevent paired-image drift | Read `TestLACPeerReleasePinsAgree` and the central race result. | Done: both real Dockerfiles supply all four matching release/hash defaults. Missing/duplicate/default/hash-format failures are explicit. |
| Retain accurate documentation | Compare both tests and Dockerfiles with the lab page and subscriber-suite architecture section. | Done: owning lab page describes paired builds and separates source, runtime and protocol proof. |

## Security Review Checklist

| Concern | Source-aware review | Result |
|---------|---------------------|--------|
| Archive substitution or failed download | Both Dockerfiles use HTTPS, `curl -fsSL`, SHA256 verification before extraction, and chained build commands. | Clear: download/checksum failure stops the stage. There is no installed-package fallback. The digest verifies selected bytes, not an upstream signature. |
| Command injection or path traversal | Probe version comes from local Dockerfile defaults. Plugin path is passed as positional `$1` to a fixed shell body in `TestLACPeerRuntimeVersions`. | Clear: the version is not inserted into shell program text. Existing image build paths and contexts remain suite-owned. |
| Unbounded work, memory or leaked resources | Pin test reads two local files and four keys. Runtime test has a two-minute context and 30-second one-shot bounds. `Docker.RunOneShot` owns names, labels and removal. | Clear: no new worker, cache, recursion or peer-controlled allocation. Cancellation cleanup is the existing native lifecycle. |
| Privilege, network or credential expansion | Runtime probes use `Network: none` and `--pull=never`. Dockerfile change adds build-only autotools dependencies. | Clear: no new socket, grant, production dependency, secret or authentication policy. Existing lab kernel grants remain unchanged. |
| False proof from matching comments or tags | Unit test reads actual ARG declarations. Integration test runs pppd and checks the exact plugin file. Native scenarios establish actual sessions. | Clear: file existence is not presented as plugin loading. Historical runs that omitted image IDs are not attributed a later image reference. |

## Documentation Update Checklist

| Category | Update? | Evidence |
|----------|---------|----------|
| Feature declaration | No | Existing subscriber capabilities and maturity are unchanged. The Dockerfile/test diff adds no production role or feature claim. |
| User guide / wiki | No additional edit | `docs/guide/l2tp.md`, `docs/guide/radius.md` and wiki `l2tp.md` describe operator/subscriber behavior, not these image pins. Wiki policy updates already landed in `312639d`. |
| Config syntax | No | No YANG, configuration reader, example syntax or product default changes. |
| CLI reference | No | No command, flag, completion, output contract or exit-code declaration changes. |
| API / RPC | No | No handler, method, event or response type changes. |
| Plugin guide | No | No plugin SDK or lifecycle change. |
| Wire format | No | No Ze encoder, decoder or protocol transition changes. Native scenarios verify compatibility of the rebuilt independent peer. |
| Plugin SDK rules | No | No registration or SDK field changes. |
| RFC compliance | No | Neither new image test carries an RFC requirement tag. This change claims no new requirement coverage and changes no RFC summary or record. |
| Test infrastructure | Yes | `docs/labs/l2tp-interop.md`, Peer-source basis, changed in `3cc99259b0`: paired build procedure and both named tests have source anchors. Closure refreshes the dated upstream-release observation. |
| Comparison | No | Lab dependency alignment changes no product parity claim. |
| Architecture | Review only | `docs/architecture/testing/interop.md`, FreeRADIUS subscriber suite, still matches `radiussubscriber.suiteFor`. No stale image version is asserted there. Foreign working-tree prose is excluded from this issue's commit. |
| Route metadata | No | No metadata producer or reader changes. |
| Prometheus counters | No | No counter or gauge changes. |
| Agent discovery | No | Tests use the existing Go test and native lab entry points. No action, framework, gate or workflow is added. |

The source-anchor census finds both Dockerfiles and both test files only in
the owning lab page. Each factual paragraph there names its actual producer.
The two Go Design headers name that page because it defines the paired-image
and runtime-proof contracts. Both Related headers resolve to existing
`buildArgument` and `Docker.RunOneShot` producers. No new doctor check is owed:
the image replaces an existing lab dependency and adds none to shipped Ze.

## Implementation Summary

### Executive Summary

Both LAC images build the same pppd 2.5.5 and xl2tpd 1.3.20 source pins.
The archive checks, paired declaration test, actual version/plugin probes,
four subscriber-RADIUS scenarios and two L2TP policy scenarios pass.
R-1 is controlled by the upstream autotools procedure and measured plugin path.
R-2 is controlled by native rebuilds and actual version probes. Historical
version/scenario outputs omit immutable image IDs, so no exact-image equality
is claimed for those runs. The later normal refusal build retains its own IDs.

### What Was Implemented

`test/interop-radius-subscriber/Dockerfile.lac` now uses L2TP's official ppp
tag archive, digest and autotools build procedure. Both retain source-built
xl2tpd and scanelf-derived runtime libraries. Each suite keeps its existing
tools. `TestLACPeerReleasePinsAgree` compares the four defaults in both actual
Dockerfiles. `TestLACPeerRuntimeVersions` probes the already-built native
images through `Docker.RunOneShot`. Implementation landed in `3cc99259b0`.

### Bugs Found/Fixed

Subscriber RADIUS retained pppd 2.5.4 after the paired L2TP image moved to
2.5.5. Its source pin, archive route and build procedure now match. The new
declaration test catches a future update to only one image. No product
authentication, MRU or RADIUS decision changes are part of this issue.

### Documentation Updates

The owning lab page documents the actual build and separates declaration,
installed-file and live-session proof. Closure refreshes its upstream latest
check to 2026-11-05. Parent F-8 and the live 16e/16f dependency cells now point
to the durable lab procedure and implementation commit rather than this
soon-removed spec. The architecture subscriber-suite section remains accurate.
The central `./le doc check all` result is disclosed under Central Checks.

### Deviations from Plan

No additional version-specific consumer required conversion. Source census
of both suite trees and native lab packages finds only Dockerfile version
comments and the new test's derived path. The architecture page needed no
version edit because it declares no pppd version. Existing native scenarios
provide the end-to-end carriers, so no new `.ci` is required.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The original handoff retained unrun-proof wording after central execution. | The coordinator had observed both runtime probes and complete native scenarios. | Independent closure compared the spec with actual outputs. | Preserve the author handoff as history and record the observed outcomes and limits here. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Align both peer images with current stable pppd | Done | Both `Dockerfile.lac` defaults and build stages | Official latest-release URL resolves to v2.5.5 on 2026-11-05. |
| Verify downloads, hashes, build inputs and plugin paths | Done | Chained archive stages, upstream autotools producers, both named tests | Source/hash agreement, installed pppd version and exact plugin ABI path are distinct checks. |
| Rebuild both and run affected scenarios | Done | `l2tp.imageBuilds`, `radiussubscriber.suiteFor`, central native outputs | Both version probes, subscriber 4/4 and policy 2/2 PASS. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Actual paired defaults, reproduced ppp digest, both successful checksum-verified builds, official latest-release metadata | No distribution ppp package or alternate installed-version fallback. |
| AC-2 | Done | Native image builds and `TestLACPeerRuntimeVersions` | Both report 2.5.5 and contain the matching plugin. Actual live sessions prove loading separately. |
| AC-3 | Done | `l2tp-radius-framed-protocol-reject`, `radius-acct-restart-freeradius`, `radius-coa-disconnect-freeradius`, `radius-coa-duplicate-freeradius` | Four passed, zero failed. Positive PPP traffic, negative framing, accounting and dynamic authorization remain required by their actual checkers. |
| AC-4 | Done | `l2tp-lns-mru-xl2tpd`, `l2tp-lns-chap-only-xl2tpd` | Correlated 1493→1492 Nak/retry, CHAP/IPCP/traffic and disallowed-PAP refusal pass on the pinned peer. |
| AC-5 | Done | Both Dockerfile chains, `TestLACPeerRuntimeVersions`, `Docker.RunOneShot` | Source-aware negative-path verification: failed curl/hash aborts the stage, absent plugin returns nonzero and fails the test. No archive/plugin fault injection is claimed. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Paired Docker builds | Done | Existing native L2TP/subscriber-RADIUS image declarations | Both rebuild before probes and semantic scenarios. |
| Runtime version/plugin smoke | Done | `lac_images_runtime_integration_test.go`, `TestLACPeerRuntimeVersions` | Both subtests PASS. |
| Affected subscriber-RADIUS scenarios | Done | Four registered scenarios in `test/interop-radius-subscriber/scenarios/` | Complete registered suite PASS. |
| Both L2TP policy scenarios | Done | `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/`, `l2tp-lns-chap-only-xl2tpd/` | Positive traffic and refusal controls PASS. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `test/interop-radius-subscriber/Dockerfile.lac` | Done | Pin, source route, autotools dependencies/procedure and ABI comment aligned in `3cc99259b0`. |
| `test/interop-l2tp/Dockerfile.lac` | Done | Paired-image maintenance comment added. Existing correct release/hash/build retained. |
| `internal/le/interoplab/lac_images_test.go` | Done | Added actual paired declaration regression. |
| `internal/le/interoplab/lac_images_runtime_integration_test.go` | Done | Added native integration-tagged version/plugin probe. |
| `docs/labs/l2tp-interop.md` | Done | Source-anchored paired-image procedure and evidence distinctions. |
| `docs/architecture/testing/interop.md` | Done | Inspected subscriber-suite claims against `suiteFor`. No version-specific assertion requires an edit. |

### Audit Summary

18 items reviewed: 3 Task requirements, 5 ACs, 4 TDD carriers and 6 files.
All are Done. No partial or skipped requirement and no scope reduction.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Both LAC builds use the current stable peer | Source and declaration test | Official latest-release observation on 2026-11-05 plus actual matching 2.5.5 defaults. `TestLACPeerReleasePinsAgree` PASS under race. |
| Verified source produces a usable pppd/plugin installation | Archive/build and runtime integration | The ppp digest was reproduced centrally. Both successful build chains verify the ppp and unchanged xl2tpd pins. `TestLACPeerRuntimeVersions/l2tp` and `/radius-subscriber` PASS with pppd 2.5.5 and the exact ABI path. |
| Rebuilt peers preserve subscriber service | Native interop | Subscriber-RADIUS 4/4 PASS. L2TP MRU and CHAP-only 2/2 PASS, including useful authenticated traffic and independent refusal evidence. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All in-scope image, build, runtime and affected-scenario obligations are met. Wider MRU/checker/RFC work remains in its existing sibling specs and is not a deferred item of 16h. | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-16h-subscriber-radius-pppd-3a525723-d369-4547-9124-fe35463dc0f5.md` |
| `./le spec review check` | Coordinator observed `OK (2 code files, clean, hashes match)` over all nine declared manifest paths. Both Dockerfiles are recorded in the artifact. The native code count includes only the two Go files. |
| Rounds | 1 independent closure round. No code fix required. |
| Reviewer lenses used | Logic+wiring/spec completeness, security+negative paths, style+documentation/source identity. |
| Result | Static review: 0 BLOCKER, 0 ISSUE. Source-only AC-5 verification and missing historical image IDs are disclosed rather than presented as observed fault runs or identity proof. |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | - | No BLOCKER or ISSUE found in the owned implementation. | Four owned Dockerfile/test files and their actual native wiring | No code change. Record-only stale handoff prose was corrected as a NOTE. |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Both LAC Dockerfiles and both new Go test files | Yes | `ls -l` lists all four regular files. Read their contents and the exact `3cc99259b0^..3cc99259b0` diff. |
| Native scenario directories and owning lab page | Yes | Read native image declarations, the complete scenario outputs and source-anchored lab procedure. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Current stable source pins agree | Both actual Dockerfiles declare the same version/hash quartet. The official latest-release URL resolves to v2.5.5 on 2026-11-05. |
| AC-2 | Rebuilt installation uses matching plugin | Actual runtime output has two PASS subtests and pppd 2.5.5 plus `/usr/lib/pppd/2.5.5/pppol2tp.so`. |
| AC-3 | Subscriber semantics survive | Actual native output lists all four registered scenario names as pass and `4 passed, 0 failed`. |
| AC-4 | Both L2TP policy controls survive | Actual output has MRU1493/Nak1492/retry1492, CHAP/IPCP and kernel link followed by PASS. Refusal trace and complete checker also PASS. |
| AC-5 | Failure cannot select a fallback | Read chained `curl`/`sha256sum` stages and the runtime test's `test -f`/fatal-error branch through `RunOneShot`. This is static negative-path evidence. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Existing Go test runner | `TestLACPeerReleasePinsAgree` in `lac_images_test.go` | Reads both real suite Dockerfiles through existing `buildArgument`. Central race run PASS. |
| Integration-tagged Go test runner | `TestLACPeerRuntimeVersions` in `lac_images_runtime_integration_test.go` | Actual suite-owned physical tags, owned `RunOneShot` lifecycle, no pull/network, exact version and plugin assertions. Central two-subtest PASS. |
| Native L2TP/subscriber-RADIUS commands | Existing scenario directories, not new `.ci` carriers | `imageBuilds` and `suiteFor` select the actual paired Dockerfiles. Registered checkers complete real kernel PPP/RADIUS sessions and controls. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Official ppp digest reproduced centrally. Both successful native build chains verify their ppp and xl2tpd archive pins before extraction. Upstream `autogen.sh` generates/configures the tree. Both actual runtime probes and native sessions pass. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Paired release/hash/build and derived runtime libraries | Both actual Dockerfiles | Owning lab paragraphs and anchors match the build instructions. |
| Pin checks versus runtime/plugin/session proof | Both named Go tests, `Docker.RunOneShot`, native suite producers | Lab paragraph states the actual assertion of each check without claiming plugin loading from file existence. |
| Go Design/Related headers | Both test headers and their named source producers | Owning lab page supplies the design contract. `sourcepin.go` `buildArgument` and `docker.go` `RunOneShot` resolve and match the dependency. |
| Architecture, wiki and other checklist categories | `radiussubscriber.suiteFor`, unchanged operator/product diff, wiki `l2tp.md` | Architecture topology and wiki operator behavior stay accurate. No version pin is asserted in the wiki page. No product config/API/SDK/metric change needs a page edit. |

### Central Checks and Limitations

| Check | Directly observed result and scope |
|-------|------------------------------------|
| Declaration regression | `job-cpe16-first-auth-image-c7fe6d80.log`: `TestLACPeerReleasePinsAgree` PASS, owning package 1.068s. Complete later interoplab package race PASS also includes this unit. |
| Runtime integration | `job-cpe16-peer-runtime-version-3461699d.log`: both actual image probes PASS and print pppd 2.5.5 with the exact matching plugin path. This log omits immutable image IDs. |
| Subscriber-RADIUS | `job-cpe16-live-radius-subscriber-d83f8741.log`: full native suite, four passed and zero failed. No skipped scenario. Historical output omits immutable image IDs. |
| L2TP policy | `job-cpe16-final-l2tp-live-e6b8eb67.log`: both policy scenarios pass. Complete later runtime/checker race scope PASS in `job-cpe16-postreview-runtime-race-f3e7163f.log`. |
| Later normal refusal with identity | `job-cpe16g-normal-image-provenance-restored-8cbdeacb.log`: full-build normal refusal passes with LAC `sha256:888be29a76531204e87131f0c36747fd1e80a3802a449e154a3058f8de975a7a`, Ze `sha256:9249679e3b8f1a3af1afa6a48059641c3770787fe6a223a9d2780013829b8925`, RADIUS `sha256:d2e91c0271db10000c1578ef16d38bcd629e73a5ee67c2e3a2cda1be8a470c7a`. These identities belong only to that later run. |
| Broader prechecks | Repo check reports nine foreign unused exports. Weakened check reports ten foreign findings and no 16h test finding. Neither is reported globally green. |
| Documentation | Latest central `./le doc check all` in `job-cpe16-doc-check-provenance-final-baf5a445.log` reports 142 published gh-pages disagreements: one catalog plus 47 mode differences across three published representations. Main/wiki anchors, YANG validation, usage/help, rules, discovery and digest stages pass. Wider site generation is blocked by foreign inventory work. No whole-site-green claim. |
| RFC freshness | This change owns no RFC tag, summary or record. Wider 16a refresh and unrelated client/BGP findings are separate. Closure creates no discrimination record or audit verdict and makes no global RFC-green claim. |

The retained proof is the canonical source and named test/scenario inputs in
`3cc99259b0`, the source-anchored lab procedure and directly observed results
recorded in commit A. Session logs in `tmp/session/2026-11-04-222204/scratch/`
are temporary debugging aids, not the durable proof. The independent closure
agent ran no build, test, formatter, Docker command, native gate or commit.
The coordinator owns execution, artifact recording and both closure commits.

### Source and Command Provenance

| Source/check | Retained observation and limit |
|--------------|--------------------------------|
| Official ppp 2.5.5 archive | The coordinator's retained session state records verification of SHA256 `5d27c1c048480139de3ba7cdca89cf455da312930473b25205a994e5fa20ef4f`. The downloaded archive and extracted source remain in the private `2026-11-04-232740` scratch directory. The original download arguments were not retained. No exact historical curl invocation is claimed. |
| xl2tpd 1.3.20 archive | Both Dockerfiles retain SHA256 `3db95450c5e1efaeea7547af344b5621f4453af3c227f26ec43bcbc79087b045`, originally taken 2026-10-22. Both successful native builds execute checksum verification before extraction. No new standalone xl2tpd hash measurement is claimed. |
| Canonical fetch/build producer | Both actual Dockerfiles fetch ppp from `https://api.github.com/repos/ppp-project/ppp/tarball/v${PPP_VERSION}`. Their `curl -fsSL` and chained `sha256sum -c` steps refuse failed downloads or mismatched bytes before `autogen.sh`. The exact historical download arguments are not required to reproduce these declared build inputs. |
| Runtime probe | `TestLACPeerRuntimeVersions` has the `integration` build constraint. The actual retained run reports both named subtests PASS, pppd 2.5.5 and the exact plugin path. Its original command arguments were not retained, so no historical race flag or complete tag list is asserted. |
| Canonical runtime reproduction | Run the native Go-test job for `./internal/le/interoplab`, with `integration` and the repository's full feature-tag selection, selector `^TestLACPeerRuntimeVersions$`, verbose output and count 1. A race-enabled repetition is a reproduction recipe, not a claim about the original probe's flags. |
| Canonical native scenario reproduction | `./le --name cpe16 test integration interop-radius-subscriber` selects the complete registered subscriber suite. `./le --name cpe16 test deployment docker-l2tp-ppp` with `ZE_L2TP_INTEROP_SCENARIO` selecting each policy scenario reproduces the L2TP controls. These are canonical entry points, not reconstructed historical argument records. |

The archive-command and historical image-ID limits affect attribution detail,
not the observed version/plugin and subscriber-service verdicts. No run is
repeated merely to replace missing historical metadata.
