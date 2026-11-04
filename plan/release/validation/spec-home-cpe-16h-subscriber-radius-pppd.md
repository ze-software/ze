# Spec: Home CPE 16h subscriber RADIUS pppd alignment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling, docs |
| Priority | P2 |
| Release reason | Subscriber RADIUS validation uses a stale pppd pin despite a paired-image update requirement. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

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
