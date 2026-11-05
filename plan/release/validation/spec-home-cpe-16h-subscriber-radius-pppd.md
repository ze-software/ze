# Spec: Home CPE 16h subscriber RADIUS pppd alignment

| Field | Value |
|-------|-------|
| Status | in-progress |
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
This skeleton records existing pins; it does not assert that pppd2.5.5 remains the latest release.
Previously reproduced hashes and reported race/interop results are historical; no images or scenarios were rerun for this split.

## Required Reading

- [ ] `ai/META-DECISIONS.md`, third-party lab release policy.
  → Constraint: both labs use the latest stable upstream release rather than a distribution's stale package.
- [ ] `docs/architecture/testing/interop.md`, FreeRADIUS subscriber suite; `docs/labs/l2tp-interop.md`, peer-source basis.
  → Constraint: real PPP kernel sessions and independent RADIUS accounting/authorization evidence remain connected.
- [ ] Upstream release metadata, archive build instructions and source for the chosen version.
  → Constraint: check release status at implementation time and verify the exact bytes and plugin ABI paths being installed.

## Current Behavior

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
| A-1 | The currently chosen archive source remains available. | Existing download routes differ. | Resolve stable release from upstream, fetch exact archive and reproduce hash; unvalidated. |
| R-1 | New archive lacks generated configure or changes ABI paths. | Build failure or plugin load failure. | Read release instructions, update build inputs and measure installed paths. |
| R-2 | One suite remains on an old image layer. | Runtime version differs despite matching Dockerfile pins. | Rebuild both and retain image/version evidence before semantic tests. |

## Scope and Next Action

This spec owns peer-image alignment for 16e/16f/16g, not their checker defects or production MRU policy.
Complete upstream release verification, exact affected-scenario inventory, plugin-path census and template checklists before `ready`. No upstream release check or image build ran while drafting this skeleton.

## Review Gate

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|

## Implementation Handoff (2026-11-04)

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
