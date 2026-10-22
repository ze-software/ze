# Spec: release-repositories-and-nightlies

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | Only the normal distro binary and declared support files ship; no appliance/host/test artifacts leak |
| Depends | `spec-release-distribution.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |
| Priority | P3 |
| Release reason | Retained later capability or corpus expansion; shipped correctness and required release proof stay with the companion. |

## Task

Own the optional APT/DNF repositories, nightly channel, channel transitions, hosted storage and retention layer that consumes the release-build and safe-package outputs. This is existing approved later-layer work, not authorization to change package safety or to publish before its evidence and audit dependencies close.

## Acceptance Criteria

Original criterion ids from `plan/release/delivery/spec-release-distribution.md` are retained for this slice only; read the full inherited design and dated evidence in its Historical source record. Where a criterion spans both slices, the ownership table below is the exact boundary.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-4 | Daily nightly schedule with unchanged source | No tag/release/snapshot/pointer is created; exact date/source digest-bound skip is retained and retry cannot create a second identity |
| AC-5 | Daily nightly schedule with changed source | Exactly one immutable nightly per actual UTC run date has a signed GitHub tag, `prerelease=true`/`make_latest=false`, is nightly-only in repositories, and cannot replace `/releases/latest`; if no stable exists, latest stays absent. Three overlapping scheduled dates remain queued and each records its exact result |
| AC-20 | Native EVR/channel transition | Native tools prove ordering; same-day stable is ordinary upgrade; newer-nightly-to-older-stable uses exact APT `--allow-downgrades` and DNF `distro-sync`, preserves state/policy, and incompatible-state recovery restores nightly |
| AC-21 | Package evidence matrix | Debian 12, Ubuntu 24.04, Rocky 9, current pinned Fedora pass amd64; Debian/Rocky pass native arm64; both booted VMs plus containers execute every transaction stage/DEB unwind, RPM preset, install, active/inactive/disabled/masked upgrade, failure/retry, remove/erase, repository refresh/reinstall, and DEB purge |
| AC-22 | Build/evidence/attestation workflows | SHA-pinned protected workflows; candidate build/evidence runs have no persisted token or write authority; isolated recorder owns checks write and publishes the versioned recorder result/check routing contract; workflow_run derives canonical names before candidate data, binds exact API artifact IDs/run/attempt outside manifests, executes no candidate code, and every identity/name/ID/race negative passes |
| AC-23 | Attestation set | Custom upstream provenance plus matching SPDX bind each binary and one canonically named aggregate bundle; wrong predicate/subject/attester/upstream run-attempt/ref/SHA/artifact/architecture or missing/duplicate pair rejects |
| AC-24 | Publication request/recovery archives | Exact evidence category set, recorder-bound check/result, dependency closure, and Actions attestations verify; immutable input archive precedes signing, exact final-byte archive precedes public staging, release-specific verification plus locked/restorable `final-attestation` archive precedes either repository, and every freshness refresh is archived/restored; mismatch/skip/stale/archive/lock/restore failure rejects at its exact barrier |
| AC-25 | Signing trust/rotation | Separate offline primaries and purpose-specific online subkeys enforce client, tag, retention-plan, and emergency trust domains; each clean client keyring has only current/next applicable roots; overlap/expiry/revocation/freeze/replacement, extra/swapped roots, wrong keys, and out-of-band fingerprint mismatch reject |
| AC-26 | GitHub draft | Exact outer set uploads to draft and downloads identically; stable publishes once with `prerelease=false`/`make_latest=true`, nightly with `prerelease=true`/`make_latest=false` while latest remains stable; `gh release verify` and every `gh release verify-asset` close actual public bytes; no replacement/move |
| AC-27 | Direct downloads and envelope | Payload manifest closes non-envelope assets, checksum covers payload plus manifest, four envelope names are exact, independent signatures, Actions provenance, and release-specific verification cover the right objects; missing/extra/tampered payload/envelope rejects |
| AC-28 | APT install/freshness | Fresh HTTPS Signed-By client verifies InRelease/by-hash/Packages/DEB, exact suite/version, Date/Valid-Until and future skew; clean-keyring mutation of Release/InRelease/index/package digest, expired/future replay, wrong/extra/swapped key/fingerprint, or package rejects and 12-hour refresh preserves availability |
| AC-29 | DNF/YUM install/freshness boundary | One mirrorlist names one combined snapshot; amd64/aarch64 clean clients verify RPM signature, repomd signature/XML, metadata and package digests; every tamper/wrong-arch/wrong-extra-swapped-key/fingerprint case rejects; honest-cache staleness is at most 16 minutes and malicious TLS/CDN replay remains outside native cryptographic guarantees |
| AC-30 | Publisher activation/replay | APT then RPM are independently atomic with signed monotonic generation/expected predecessor; RPM-before-APT rejects, APT failure leaves both old, RPM failure may leave new APT/old RPM, retries never rebuild/resign, and replay/downgrade/invalid emergency authorization rejects without claiming native DNF enforcement |
| AC-31 | Storage/CDN/archive isolation | Exactly five buckets and three one-bucket public domains enforce routing/lifecycle; prefix locks, admin-key absence, conditional pointers, public-origin match, private non-routing, clean restore of every archive class, and denied cross-bucket operations pass |
| AC-32 | Nightly retention | At least 30 full days plus 7-day grace from trusted `public_activated_at` and expired object locks are both required; delayed staging cannot shorten availability; broker rechecks chrony before each mint/child, validates the signed exact plan, and mints five-minute exact-object delete-only sessions; retention Apps delete only nightly releases/tags; read/list/put/copy/unlisted/stable/bootstrap/current access denies |
| AC-33 | Stable retention | Stable public immutable objects, releases, snapshots, signed envelopes/attestations/records, and input/final/final-attestation/refresh private archives remain indefinitely unless security/legal runbook acts |
| AC-34 | Operations | Every clock/cadence/timeout/threshold/dedup/recovery/freeze rule, dispatch/App/key/activation/replay/CDN/APT expiry/DNF staleness/archive/credential/restore/retention scenario produces deterministic alerts and append-only evidence |
| AC-35 | Documentation | Users download and verify the repository installer before execution, bootstrap and separately remove source/key trust in fail-closed order, install/verify/switch/downgrade/upgrade/remove/purge/reinstall, understand service/update/replay limits and recover; maintainers have source-anchored dispatch/sign/publish/freeze/rollback/rotation/restore runbooks |
| AC-36 | First public activation, stable or nightly | A protected, exact-candidate dependency-closure record proves every dependency/finding/evidence item complete; infrastructure is live; staging promotion and every archive clean restore succeed; each primary and independent GitHub/APT/RPM canary failure freezes completion, documentation mutation, and subsequent publication until all six legs pass, after which README/status/spec closure may proceed |

## Implementation Steps

Execute the inherited repository/nightly/retention implementation after safe artifacts exist. Cross-cutting criteria below apply only to the added channels; consume rather than reimplement the direct package safety owner. Every original repository refresh, channel transition, trust rotation, archive, canary and recovery obligation remains required here.

## Work Not Done

The companion retains all other obligations and the complete historical source record. This split is neither closure nor a claim of missing implementation; original approvals, dependencies, artifact requirements and proof standards remain in force.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| AC-1–AC-3, AC-6–AC-19 | `plan/release/delivery/spec-release-distribution.md` | Candidate identity, approved artifacts, reproducibility, package lifecycle and self-update protection; use the owner amendment where it supersedes mechanisms |
| AC-4–AC-5, AC-20, AC-28–AC-32 | `plan/post-release/features/spec-release-repositories-and-nightlies.md` | Nightlies, repository channel changes, APT/DNF trust, hosting and nightly retention |
| AC-21–AC-27, AC-33–AC-36: direct-artifact/package portions | `plan/release/delivery/spec-release-distribution.md` | Package test matrix, trust/provenance, immutable direct publication, stable retention, monitoring/restoration/freeze rules, documentation and first direct activation |
| AC-21–AC-27, AC-33–AC-36: repository/nightly portions | `plan/post-release/features/spec-release-repositories-and-nightlies.md` | Additional channel-specific evidence, trust, publication, repository refresh/reinstall, retention, operations, documentation and activation |

## Required Reading

- `plan/release/delivery/spec-release-distribution.md`: full original source record, design, tests, decisions and evidence; apply them to this file’s owned slice only.
