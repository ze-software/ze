# Spec: release-distribution

| Field | Value |
|-------|-------|
| Status | design |
| Depends | `spec-release-evidence-gate.md`, `spec-release-audit-0-umbrella.md` |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md` and `ai/rules/completion.md`.
3. `plan/spec-release-evidence-gate.md` and `plan/spec-release-audit-0-umbrella.md`.
4. `Makefile:48-56,115-153`, `mk/test-release.mk`, and `internal/core/version/version.go`.
5. `internal/plugins/init/main.go,158-340`, `internal/plugins/systemd/cmd_install.go`, `internal/plugins/systemd/unit.go`, and `internal/component/doctor/checks_platform.go`.
6. `internal/component/config/system/selfupdate.go,672-719` and `cmd/ze/update_serve.go`.
7. `.github/workflows/codeql.yml`, `.github/workflows/pages.yml`, and `.woodpecker/verify.yml`.
8. `docs/guide/quickstart.md`, `docs/guide/ubuntu-build-install.md`, `docs/guide/ze-install.md`, and `docs/guide/self-update.md`.

## Task

Deliver a production release-distribution system for Ze's normal Linux daemon binary. A maintainer-created signed CalVer tag must produce immutable GitHub Release downloads, cryptographically authenticated DEB artifacts, signed RPM packages, signed APT and DNF/YUM repositories at `packages.ze-software.net`, and machine-verifiable provenance. A daily schedule must produce one immutable nightly per UTC day when `main` changed, with a separate prerelease channel and 30-day retention.

The system must preserve Codeberg as the canonical source repository and GitHub as the official release/download mirror. It must fail closed on mirror divergence, unsigned or unauthorized tags, incomplete release evidence, unverified build bundles, package test failures, or publication errors. The publishing worker must be isolated from the build worker and must never compile or execute code from the candidate checkout.

The package must install only the normal `ze` Linux daemon and its operational support files. It must make a fresh package installation usable without exposing a plaintext bootstrap secret, while preserving existing state during upgrades and ordinary removal. Package-managed binaries must never be replaced behind APT/RPM ownership by Ze's in-place self-update backend.

### Scope boundary

Included:
- Linux `amd64` and `arm64` builds of `cmd/ze` with `ze_core`, `ze_distro`, and the complete default feature-gate set.
- Direct tarball downloads, DEB, RPM, checksums, SPDX SBOMs, input/final release manifests, GitHub attestations, and signed repository metadata.
- Stable and nightly channels.
- Package-first bootstrap, systemd lifecycle, repository installation, upgrade, removal, purge behavior, package-managed update protection, and release operations.
- Codeberg to GitHub fast-forward mirroring and exact signed tag propagation.

Excluded:
- `ze-setup`, `ze-appliance`, `ze-stripped`, `ze-test`, `ze-chaos`, `ze-perf`, `ze-analyze`, standalone installer binaries, appliance images, installer kernels/initrds, ISOs, and containers.
- Homebrew, Snap, Flatpak, APK, Windows, macOS, FreeBSD, and source packages.
- Replacing the existing `ze update-serve` command or publishing a project-operated self-update feed.
- A general-purpose package hosting application or always-running web service.

The excluded binaries are host/developer, test/evidence, or target/appliance artifacts with different build and lifecycle contracts. They may get separate release specs only after their source-supported packaging and end-to-end tests exist.

## Required Reading

### Architecture Docs and Rules

- [ ] `ai/rules/architecture.md` - explicit behavior, simple ownership, and no speculative surfaces.
  -> Decision: use checked-in deterministic release tooling around the existing Make/Go build instead of adding GoReleaser as a second build model.
  -> Constraint: nFPM is limited to native package assembly; it does not own version derivation, release policy, signing, or publication.
- [ ] `ai/rules/architecture.md` - trace source, build, attestation, signing, and publication boundaries.
  -> Decision: separate unprivileged build, attestation, and privileged signing/publication stages with digest-bound manifests between them.
- [ ] `ai/rules/evidence.md` - every behavioral claim needs producing-source evidence.
  -> Constraint: package and release documentation must cite build, init, service, update, and publication producers.
- [ ] `ai/rules/testing.md`, `ai/rules/testing.md`, `ai/rules/testing.md`, `ai/rules/completion.md` - test-first and user-entry coverage.
  -> Decision: package lifecycle is tested through real package managers and systemd, not only archive inspection.
  -> Constraint: `ze init --automatic` needs Go unit tests and an install-suite `.ci` test; repository installation needs distro integration evidence.
- [ ] `ai/rules/platform-linux.md` - Linux-only behavior needs Linux integration evidence.
  -> Decision: container tests cover the distro matrix and booted QEMU VMs prove one DEB and one RPM family systemd lifecycle end to end.
- [ ] `ai/rules/repo-maintenance.md` and `ai/rules/writing.md` - release tooling and user install paths must be discoverable.
  -> Constraint: release targets, runbooks, package guide, architecture doc, indexes, and source anchors ship with the implementation.
- [ ] `ai/rules/repo-maintenance.md` - runtime dependency readiness belongs to the owning component.
  -> Decision: extend the existing doctor systemd check to inspect systemd's effective `FragmentPath`, so vendor units and admin override units are both verified.
- [ ] `ai/rules/completion.md` and `ai/rules/completion.md` - every public entry point must be reachable and tested.
  -> Constraint: no release can be presented as complete until all stable/nightly, package, repository, signing, lifecycle, and recovery ACs pass.
- [ ] `docs/architecture/cli/plugin-modes.md` - offline command ownership and systemd command placement.
  -> Decision: `ze init --automatic` remains in the existing init plugin; package service behavior remains aligned with the existing systemd plugin.
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` test format and install-suite conventions.
  -> Constraint: package bootstrap and command reachability tests live in `test/install/` or `test/ui/` using existing runners.

### Existing Specs

- [ ] `plan/spec-release-evidence-gate.md` - full release-readiness evidence matrix.
  -> Decision: stable publication consumes a successful evidence record for the exact candidate commit rather than duplicating the evidence matrix.
  -> Constraint: this spec cannot publish a public nightly or stable release until the release-evidence spec is complete and package evidence is added to it.
- [ ] `plan/spec-release-audit-0-umbrella.md` - first user-facing release audit.
  -> Decision: implementation and dry-run staging may proceed, but public release activation waits for the umbrella and every blocking child finding to close.
  -> Constraint: a nightly is user-facing and is subject to the same first-publication gate.
- [ ] `plan/spec-release-audit-8-docs-onboarding.md` - release-facing documentation audit.
  -> Decision: installation and release docs added here must also satisfy the docs audit's source-backed onboarding requirements.

### External References

- [ ] GitHub Releases documentation - release assets may each be under 2 GiB, with no total release-size or download-bandwidth limit documented.
  -> Decision: use GitHub Releases for direct downloadable artifacts, never GitHub Pages.
  -> Reference: <https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases>
- [ ] GitHub immutable releases documentation - published tags and assets are locked, and draft-first publication is recommended.
  -> Decision: upload and verify every asset on a draft, then publish once; enable immutable releases on `ze-software/ze` before first activation.
  -> Reference: <https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases>
- [ ] GitHub `actions/attest` documentation - custom predicates and SPDX mode require protected OIDC plus `attestations: write` and `artifact-metadata: write`.
  -> Decision: one `workflow_run` job binds protected workflow and resolved candidate identities separately, records API artifact IDs only after upload, and executes no candidate code.
  -> References: <https://github.com/actions/attest>, <https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations>
- [ ] GitHub Pages limits - Pages is limited to a 1 GiB published site and a 100 GiB/month soft bandwidth limit and is not intended for software distribution.
  -> Decision: Pages remains documentation-only.
  -> Reference: <https://docs.github.com/en/pages/getting-started-with-github-pages/github-pages-limits>
- [ ] nFPM configuration - native DEB/RPM package creation and package script/content metadata.
  -> Decision: pin standalone nFPM and use one reviewed package definition with format-specific script overrides.
  -> Reference: <https://nfpm.goreleaser.com/configuration/>
- [ ] Debian Policy, control fields and maintainer scripts.
  -> Constraint: DEB versions, conffile/state behavior, and maintainer-script exit semantics follow Debian policy.
  -> Reference: <https://www.debian.org/doc/debian-policy/ch-controlfields.html>
- [ ] RPM version comparison and package signing documentation.
  -> Constraint: RPM EVR ordering and package signatures are validated with RPM's own tools.
  -> References: <https://rpm.org/docs/6.0.x/man/rpm-version.7>, <https://rpm.org/docs/6.0.x/man/rpmsign.8>
- [ ] Aptly snapshot publication and S3 endpoints.
  -> Decision: aptly generates APT repository snapshots locally; the Ze publisher controls S3 upload order and activation.
  -> Reference: <https://www.aptly.info/doc/aptly/publish/>
- [ ] `createrepo_c` repository metadata.
  -> Decision: generate immutable RPM snapshots with `createrepo_c`, then sign packages and `repomd.xml` before activation.
  -> Reference: <https://rpm-software-management.github.io/createrepo_c/>
- [ ] Cloudflare R2 custom domains, bucket locks, API tokens, temporary credentials, and S3 compatibility.
  -> Decision: map three custom domains one-to-one to five total buckets, use provider-native prefix locks, and locally mint five-minute exact-object `DeleteObject`/`DeleteObjects` sessions inside a root broker; do not claim ordinary tokens are delete-only or require AWS Object Lock/versioning.
  -> References: <https://developers.cloudflare.com/r2/buckets/public-buckets/>, <https://developers.cloudflare.com/r2/buckets/bucket-locks/>, <https://developers.cloudflare.com/r2/api/tokens/>, <https://developers.cloudflare.com/r2/api/s3/temporary-credentials/>, <https://developers.cloudflare.com/r2/api/s3/api/>

**Key insights:**
- GitHub Releases are the correct direct-download surface. GitHub Packages does not provide APT/RPM repositories, and GitHub Pages is unsuitable for package traffic.
- The existing release build embeds wall-clock values (`Makefile:53-56`), while runtime release comparison accepts only the eight-character `YY.MM.DD` identity (`internal/core/version/version.go`). Release tooling must use the source commit timestamp and keep nightly channel identity outside the embedded release.
- The package-sized product is one statically linked `ze` distro binary plus service/account declarations, completion scripts, install-method marker, license, and metadata. Repository source/key trust is an external ordered bootstrap pair; appliance and host-tool outputs have separate producer contracts.
- Fresh package installation cannot safely call the interactive init flow. The current init path requires a username and plaintext password (`internal/plugins/init/main.go`) and discovers host interfaces (`internal/plugins/init/main.go`). Package bootstrap needs an explicit noninteractive mode with username `admin`, at least 256 bits of CSPRNG entropy, and only loopback management config.
- The current doctor service check reads only `/etc/systemd/system/ze.service` (`internal/component/doctor/checks_platform.go`), so it would miss a package-owned vendor unit. It must discover the effective unit through systemd with a bounded call.
- The current self-updater replaces its running executable in place (`internal/component/config/system/selfupdate.go,875-886`). Package installs need a package-manager marker and backend guard so APT/RPM always owns `/usr/bin/ze`.
- Static object storage can serve APT, RPM, signed release manifests, and audit archives. Only small channel pointers are mutable.
- Signing keys and publication credentials must never enter candidate-executing jobs. A protected default-branch attestation workflow and an isolated VPS publisher verify a closed input bundle before final signing.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/version/version.go` - release identity comparison and build metadata (full line ranges in the entries below).
- [ ] `internal/plugins/init/main.go` - current interactive/piped bootstrap flow (full line ranges below).
- [ ] `internal/component/doctor/checks_platform.go` - current systemd unit check (full line ranges below).
- [ ] `internal/component/config/system/selfupdate.go` - current in-place self-update behavior (full line ranges below).
- [ ] `Makefile:48-56,93-168` - derives feature tags from `feature-gates.txt`, uses wall-clock version/build date, and builds all local binaries.
- [ ] `mk/test-release.mk` - composes the release evidence matrix but performs no packaging, signing, or publishing.
- [ ] `.woodpecker/verify.yml:1-19` - runs `make ze-verify` on pushes and pull requests.
- [ ] `.github/workflows/codeql.yml` - GitHub security analysis only.
- [ ] `.github/workflows/pages.yml` - GitHub Pages publication only.
- [ ] `internal/core/version/version.go` - compares eight-character CalVer releases lexically and exposes build metadata.
- [ ] `internal/plugins/init/main.go,158-340` - interactive/piped bootstrap, bcrypt storage, host-interface discovery, optional TLS, and atomic zefs rename.
- [ ] `internal/plugins/systemd/cmd_install.go` - manual systemd install, account creation, state ownership, admin unit write, daemon-reload, enable, and optional start.
- [ ] `internal/plugins/systemd/unit.go` - current service identity, capabilities, paths, security flags, and restart policy.
- [ ] `internal/component/doctor/checks_platform.go` - validates only the hard-coded admin unit path and its executable/user/group.
- [ ] `internal/core/ssh/client/client.go,385-423` - local super-admin uses the stored bcrypt hash as an opaque SSH token.
- [ ] `internal/component/ssh/ssh.go,161-190` - default SSH listener is loopback `127.0.0.1:2222`.
- [ ] `internal/component/config/system/selfupdate.go,672-719` - static-compatible update manifest fields and architecture-derived download fallback.
- [ ] `internal/component/config/system/backend.go,71-115` and `backend_ze_distro.go,85-133` - platform-only backend selection and self-update operation dispatch.
- [ ] `cmd/ze/hub/main_system.go` - platform/options producer and active backend startup.
- [ ] `internal/plugins/update-cmd/cmd/firmware.go` - user check/download/apply/restart/rollback handlers all consume `system.ActiveBackend()`.
- [ ] `internal/component/config/system/selfupdate.go,774-814,875-886` - manual download writes adjacent temporary state and apply/rollback rename executable bytes.
- [ ] `cmd/ze/update_serve.go` - current single-binary update manifest and download endpoint contract.
- [ ] `docs/guide/quickstart.md` and `README.md` - source build is the only published getting-started path.
- [ ] `docs/guide/ubuntu-build-install.md` - build/install/init/systemd sequence and current source-supported service behavior.
- [ ] `docs/guide/self-update.md` - standalone update-server and manifest contract.

**Current outputs:**
- No repository-defined release tag trigger, nightly build schedule, GitHub Release creation, native package definition, APT/RPM repository, artifact signing, SBOM publication, release retention, or automated Codeberg/GitHub mirroring exists.
- The current GitHub repository has no published releases or tags as observed on 2026-07-10.
- Local `make build` produces multiple binaries; only `bin/ze` is the normal distro daemon (`Makefile:115-153`).

**Behavior to preserve:**
- Codeberg remains the canonical development source and GitHub remains the official public repository/download surface (`README.md`).
- The normal release binary uses `ze_core`, `ze_distro`, and every gate derived from `feature-gates.txt` (`Makefile:48-51,118-120`).
- Runtime embedded release remains exactly `YY.MM.DD`; no `v` prefix or nightly suffix reaches `main.version`.
- The package service remains `ze.service`, runs as `ze:ze`, starts `/usr/bin/ze start`, uses `/etc/ze`, sets `/run/ze`, and retains the existing capability and hardening contract from `buildUnitFile`.
- Existing `database.zefs`, active configuration, credentials, and `/etc/ze` survive upgrades and normal package removal.
- Existing interactive `ze init`, source installation, `ze install systemd`, standalone/source self-update, appliance, and developer build paths keep their behavior; only package-marked installs select the mutation-blocking update backend.
- `make ze-verify` remains the fast pre-commit gate; heavy release and package evidence stays in explicit release targets.

**Behavior to change:**
- Authorized signed tags and the daily schedule produce complete release bundles for Linux `amd64` and `arm64`.
- `ze init --automatic` adds an idempotent, package-safe first-install path.
- Doctor discovers and validates the effective systemd unit regardless of vendor or admin location.
- Native packages, repositories, release manifests, signing, provenance, retention, and operations become first-class repository-defined behavior.
- Quickstart and installation documentation lead release users to verified packages/downloads while retaining source-build instructions for developers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

The channel-specific paths below carry the full detail; this overview maps the
canonical stages first.

### Entry Point
- Stable: a maintainer-created annotated, signed `YY.MM.DD` tag on canonical Codeberg, verified and propagated by the mirror (see Stable Entry Point below).
- Nightly: the protected default-branch schedule `17 2 * * *` (see Nightly Entry Point below).
- Format at entry: a signed git tag object, or a scheduled protected workflow run.

### Transformation Path
1. The mirror verifies the signed tag (or the schedule fires), records its journal entry, and dispatches the protected build workflow.
2. The unprivileged build produces deterministic binaries, tarballs, DEB/RPM inputs, SBOMs, and the closed `input-manifest.json` bundle.
3. The protected attestation workflow binds provenance/SPDX statements to the verified candidate identity.
4. The evidence workflow runs the channel's mandatory category set; a protected recorder emits the recorder-bound result and commit check.
5. The isolated VPS publisher verifies the bundle, archives inputs, signs packages and metadata, publishes the immutable GitHub release, then activates APT and RPM in that order.

### Boundaries Crossed
(Summary; the detailed component/security boundary table is in the second
Boundaries Crossed section further below.)

| Boundary | How | Verified |
|----------|-----|----------|
| Codeberg -> GitHub | fast-forward mirror + exact signed tag propagation | [ ] |
| Build -> attestation | closed `release-input.tar` artifact fetched by API ID | [ ] |
| Attestation/evidence -> publisher | recorder-bound commit check + artifact IDs | [ ] |
| Publisher -> public surfaces | immutable GitHub release; APT/RPM snapshot activation | [ ] |

### Integration Points
(Summary; the full list is in the second Integration Points section further below.)
- `mk/test-release.mk` evidence matrix - consumed as the release evidence category source.
- `internal/plugins/init` (`ze init --automatic`) - the package-safe bootstrap entry point.
- `internal/plugins/systemd` unit contract - vendor unit parity for the packaged service.
- `internal/component/config/system` self-update backend - the package-managed mutation guard.

### Stable Entry Point

1. A maintainer creates an annotated, signed `YY.MM.DD` tag in canonical Codeberg. Exactly one stable source release is permitted per UTC date; a packaging correction uses a corrected commit and new date, never a suffix or moved identity.
2. The mirror uses its NTP-synchronized trusted UTC clock and durable stable high-water mark. The tag date must equal the current UTC date, except the immediately previous date is accepted only through `06:00:00Z` for midnight-boundary retries; future dates, older dates, duplicate dates, and identities not strictly newer than the high-water mark reject before dispatch.
3. It verifies the tag signature against an out-of-band maintainer fingerprint allowlist, verifies the target is reachable from protected Codeberg `main`, fast-forwards GitHub `main`, and pushes the exact annotated tag object to GitHub.
4. Before API dispatch it atomically records a mirror journal entry containing tag/ref object IDs, peeled candidate SHA, protected workflow path/digest, a random dispatch request ID, attempt number, and stage `prepared`; the protected workflow's exact `run-name` includes that request ID. A successful response advances to `submitted`. A timeout or lost response advances to `response-ambiguous`, polls the workflow-runs API for the exact event/run-name/head SHA within a bounded eventual-consistency window, and never advances the stable high-water mark until one run ID is selected. If no run appears it may retry the identical request after the window; if one or more delayed duplicate runs appear, an atomic first-created-run-ID selection is durable and every other run is rejected by recorder/publisher policy. Crash/retry tests cover failure before request, accepted response lost, request not accepted, one delayed run, and duplicate delayed runs without two publishable candidates.
5. The build workflow re-resolves both IDs from both forges and rejects any mismatch before checkout.

### Nightly Entry Point

1. Protected default-branch schedule `17 2 * * *` runs once daily after the five-minute VPS mirror timer. Nightly `workflow_dispatch` is rejected; a failed scheduled run resumes only through GitHub's native re-run of the same run ID, original `created_at` date, original `head_sha`, and incremented attempt.
2. Scheduled workflows intentionally have no GitHub concurrency group, because GitHub retains only one pending member per group and a third arrival can cancel an older pending date. Every scheduled run remains independently queued, verifies GitHub `main` equals canonical Codeberg `main`, derives its UTC identity date from the API-verified original run `created_at`, and carries that date/source digest through a durable publisher compare-and-swap. Forge refs/releases, protected workflow runs/artifacts, and publisher state reject a successful or different in-progress public identity for the date; duplicate same-identity runs may build but only the first durably selected run ID can record evidence or publish. The concurrency test overlaps at least three dates, proves none is cancelled/replaced, and proves serialized, one-per-date FIFO publication despite out-of-order job completion.
3. If no source commit changed since the previous successful nightly, it records a successful date/source digest-bound skip and publishes nothing; a re-run reproduces the same skip.
4. Otherwise identity is `nightly-YY.MM.DD-g<12-hex-sha>` and embedded release is `YY.MM.DD`.
5. After bundle verification, publisher signs the annotated nightly tag with the nightly-tag subkey and pushes the identical object to both forges before the GitHub prerelease.

### Build and Attestation Path

1. Resolve channel, full candidate commit SHA, source commit timestamp, embedded release, architecture map, complete feature-gate set, and deterministic environment.
2. Reject dirty/generated drift, unauthorized refs, invalid/out-of-policy calendar dates, mismatched main branches, a stable target not reachable from protected main, or a release tool/workflow digest not allowed by publisher policy.
3. Build `cmd/ze` for `linux/amd64` and `linux/arm64` with `CGO_ENABLED=0`, `ze_core`, `ze_distro`, every gate derived from `feature-gates.txt`, `-trimpath`, VCS metadata, empty Go build ID, explicit embedded release, and commit timestamp build date.
4. Run `ze --extended-version`, exact-tag build tests, and compiled-inventory checks. Assert release, commit, clean state, Go version, OS, architecture, and the complete expected feature set.
5. Generate deterministic completion files, tarballs, DEB/RPM inputs through pinned nFPM, and per-architecture SPDX 2.3 SBOMs through pinned Syft.
6. Create `input-manifest.json` containing the closed file set, candidate repository/ref/SHA, protected build workflow path/digest/run ID/attempt, the protected-policy-derived packaging artifact name `ze-release-input-v1-run-<run-id>-attempt-<attempt>`, tool identities, modes, sizes, and digests. Transport-assigned GitHub artifact IDs are deliberately excluded; the manifest never claims final RPM or repository bytes.
7. Assert the `/usr/bin/ze` extracted from each DEB/RPM and the `ze` from each tarball have the exact SHA-256 digest recorded for that architecture.
8. Build the same input twice in isolated directories and compare every unsigned output byte for byte.
9. Canonically pack the closed files into one deterministic `release-input.tar`, then upload one GitHub artifact with the protected-policy-derived packaging name containing only that tar. GitHub's transport archive/ID need not be reproducible. The unprivileged build job has no release, repository, object-storage, signing, OIDC, or attestation-write authority.
10. Protected default-branch `release-attest.yml`, triggered by `workflow_run`, derives the three current-attempt names `ze-release-input-v1-run-<run-id>-attempt-<attempt>`, `ze-release-evidence-v1-run-<run-id>-attempt-<attempt>`, and `ze-release-evidence-record-v1-run-<run-id>-attempt-<attempt>` from protected policy before reading candidate data. It verifies the triggering workflow path/digest, conclusion, event, head repository ID, protected workflow `head_sha`/head ref, run ID/attempt, and API artifact inventory. Every artifact name must be canonical for an API-observed attempt from the same run, and all names/IDs are unique. Failed historical native rerun attempts may leave any unique subset of their own three canonical names and are ignored; the current successful attempt must have exactly its complete triplet. It downloads each current artifact by its API-returned ID, verifies run/attempt association, verifies the input and evidence-record internal declarations equal their API names, and rejects swapped names, duplicates, cross-run IDs, cross-attempt IDs, and unknown artifacts. The packaging transport contains only `release-input.tar`; its extracted closed set is verified and the transport ID plus deterministic tar digest are recorded in the predicate, not the input manifest. For stable, it independently resolves the protected annotated tag to the candidate ancestor SHA; for nightly, it binds the scheduled run's original `head_sha` as candidate. Downstream attester `GITHUB_SHA`/`GITHUB_REF` are never candidate identity.
11. Pinned `actions/attest` custom mode emits predicate type `https://ze-software.net/attestation/release-build/v1` for each architecture binary and the aggregate input bundle. The predicate separately records API-verified protected workflow revision/run/attempt/artifact ID and resolved candidate repository/ref/SHA while the OIDC statement retains the protected attester workflow identity.
12. Pinned `actions/attest` SBOM mode emits a matching SPDX predicate for each architecture binary. Publisher policy requires one custom provenance and one SPDX statement with identical binary subject digest and attester run identity, plus exactly one aggregate provenance subject. Tests cover a stable tag ancestor of the dispatch `head_sha`, `main` advancing again before `workflow_run`, rerun artifact inventory, and binding only the verified tag candidate plus original protected workflow revision.

### Evidence Path

1. Protected default-branch `release-build.yml` calls `release-evidence.yml` only through local reusable `workflow_call`, passing required channel, release identity, candidate ref/SHA, caller workflow path/digest/run ID/attempt, and required-set version. `release-evidence.yml` has no PR, push, schedule, or manual trigger; it rejects a non-protected caller/ref or candidate-controlled workflow revision, checks out only the passed candidate as test input, and runs `make ze-release-evidence` on the dedicated reset runner.
2. Stable mandatory category IDs are exactly: `verify`, `chaos`, `fuzz`, `interop`, `ipsec-interop`, `l2tp-interop`, `pppoe-interop`, `functional-extra`, `perf`, `qemu`, `vpp-deployment`, `live`, `release-policy`, `release-repro`, `package-deb-amd64`, `package-deb-arm64`, `package-rpm-amd64`, `package-rpm-arm64`, `package-vm-deb`, `package-vm-rpm`, and `repository-tamper`. None may be skipped.
3. Stable evidence also includes `mutation` as its only advisory category. It must execute and record either pass or advisory-fail; it may not be skipped and cannot substitute for a mandatory category.
4. Nightly mandatory and allowed IDs are exactly: `verify`, `release-policy`, `release-repro`, `package-deb-amd64`, `package-deb-arm64`, `package-rpm-amd64`, `package-rpm-arm64`, `package-vm-deb`, `package-vm-rpm`, and `repository-tamper`; mutation is not a nightly category. VM categories use the nightly smoke profile, while stable uses full lifecycle.
5. The versioned evidence manifest binds repository/ref/SHA, release identity/channel, trusted caller and reusable-workflow paths/digests, caller run ID/attempt, the protected-policy-derived evidence name `ze-release-evidence-v1-run-<run-id>-attempt-<attempt>`, required-set version, and exactly one record per channel-allowed category with classification, status, command, log digest/path, start/end timestamps, runner image, and tool lock digest.
6. Stable publication accepts only the exact stable set with every mandatory status `pass`; nightly accepts only the exact nightly set. Missing, duplicate, unknown, skipped, advisory-only, stale, wrong-caller/SHA/run/attempt/required-set, or digest-mismatched entries reject.
7. The candidate-running reusable job packs only the canonical manifest and declared logs into one evidence artifact under the exact protected-policy-derived name. A fresh protected recorder job executes only trusted default-branch verifier code, lists and downloads that artifact by the API-returned ID, verifies the closed transport set and every log digest, and emits a closed `evidence-recorder-result.json` binding schema version, repository, candidate ref/SHA, caller and reusable workflow paths/digests, run ID/attempt, required-set version, evidence logical name/API ID/manifest and transport digests, and, for stable, the dependency-closure record/digest. It uploads the result alone as `ze-release-evidence-record-v1-run-<run-id>-attempt-<attempt>`, captures that API ID, and writes exactly one successful `ze/release-evidence-v1` candidate commit check from the GitHub Actions App; the check external ID binds repository, run, attempt, result-artifact ID, and result digest. Publisher and workflow-policy negatives cover missing/duplicate/stale/wrong-app checks, candidate-authored checks, wrong run/attempt/name/API ID/digest/required-set/SHA, swapped names, cross-run and cross-attempt artifacts, and every evidence rejection class.

### Dependency Closure Gate

`packaging/publisher/release-dependency-policy.json` is a closed, versioned protected-policy input whose digest is pinned by publisher/workflow policy. Its spec IDs are exactly `spec-release-evidence-gate`, `spec-release-audit-0-umbrella`, `spec-release-audit-1-surface-inventory`, `spec-release-audit-2-bgp-protocol`, `spec-release-audit-3-config-cli`, `spec-release-audit-4-web-lg-api`, `spec-release-audit-5-plugins-rib`, `spec-release-audit-6-system-linux`, `spec-release-audit-7-resilience-security`, and `spec-release-audit-8-docs-onboarding`. Before first activation the finalized policy also enumerates every exact blocking finding ID from the closed child indexes, each required evidence category, and the allowed closure-record schema; placeholders, ranges, category-only summaries, and unknown IDs reject. The current `spec-release-distribution` is intentionally not its own prerequisite. Changes require protected review and a policy-version/digest update; candidate manifests cannot add, remove, or rename dependencies.

For a stable candidate, the protected recorder parses repository files as data without executing candidate code and creates `dependency-closure.json`. The closed record binds schema/policy version and digest, repository and exact candidate SHA, then for every policy entry records the closure commit (which must be an ancestor of the candidate), preserved learned/audit record path and digest, exact zero-open-finding result, and required release-evidence record digests. It rejects any still-present open dependency spec, missing/partial/skipped finding, stale or wrong-SHA record, non-ancestor closure, unknown dependency/finding discovered in the umbrella's closed index, duplicate entry, or evidence mismatch. The publisher and infrastructure preflight require this recorder-bound record before stable signing and again before first public completion; `DependencyClosurePolicyTest.test_exact_candidate_closure` covers success plus every rejection class.

### Signing and Publication Path

1. The VPS publisher polls GitHub Actions with a release-publisher GitHub App installation token and considers only completed, mirror-journal-selected stable runs or publisher-state-selected nightly runs.
2. It queries candidate checks and requires exactly one successful `ze/release-evidence-v1` check from the expected GitHub Actions App whose external ID matches repository/candidate/run/attempt. It downloads `evidence-recorder-result.json` by the check-bound result-artifact API ID, verifies its closed schema/name/digest and all identity fields, then downloads the evidence artifact only by the recorder-bound API ID. It verifies the evidence set plus every provenance/SBOM statement, trusted workflow digest, repository/ref/SHA, run attempt, input bundle digest, input-manifest schema, tool lock, closed file/subject sets, and stable dependency-closure record. It never independently chooses an evidence artifact by candidate-supplied name.
3. It never checks out, compiles, imports, sources, or executes candidate-controlled files. Maintainer scripts are data and were already exercised in disposable package-test environments.
4. Before signing, it archives the verified input bundle, attestation bundles, evidence record/result, dependency-closure record, and tool lock under a unique private immutable input prefix. Stable input archives are indefinite; nightly input archives cannot be pruned until both their object lock expires and at least 37 full days have elapsed since trusted public activation. Archive/retention verification failure blocks signing.
5. It signs each RPM exactly once, confirms the signed RPM still contains the attested architecture binary and expected payload, and generates all final package/repository bytes.
6. `release-manifest.json` is the canonical payload manifest. It lists every non-envelope GitHub payload asset exactly once with name, size, digest, source input digest, architecture, and attestation identity. It excludes itself, `release-manifest.json.asc`, `SHA256SUMS`, and `SHA256SUMS.asc`; its schema names those four expected envelope files without self-digests.
7. `SHA256SUMS` lists every payload asset plus `release-manifest.json`; it excludes itself and both detached signatures. The direct-release key signs the manifest and checksum list independently. The deterministic outer release set is payload subjects plus those four envelope files, and draft/public listing checks reject any missing or extra payload/envelope name.
8. It generates one multi-architecture RPM repository snapshot per channel release plus APT indexes for both architectures, signs APT `Release`/`InRelease` and RPM `repomd.xml`, and verifies all signatures with clean keyrings.
9. Before any public activation, it writes a second unique retention-locked private archive containing every exact final GitHub outer-set byte, complete APT/RPM snapshot bytes, candidate pointer payloads/signatures, release record, and digest inventory. Clean-room restore must reproduce every final digest without publisher disk or secret key access.
10. It creates a GitHub draft release, uploads the exact outer set, downloads every asset back, and verifies the payload manifest, envelope rules, signatures, and digests.
11. It uploads immutable object-store snapshot objects, verifies them over public HTTPS/CDN, and leaves current channel metadata unchanged.
12. It publishes the GitHub draft once as immutable: stable request and API-observed state must be `prerelease=false` and `make_latest=true`; nightly must be `prerelease=true` and `make_latest=false`, and publishing nightly must leave `/releases/latest` resolving to the last stable release. It runs `gh release verify` for the release and `gh release verify-asset` for every downloaded outer-set asset. Because GitHub does not expose a separate attestation-export command in this contract, `release-attestation-record.json` closes the exact verifier version, command/exit status/transcript digests, repository/release/tag/commit IDs, API-observed immutable/prerelease/latest fields, and every asset name/ID/size/digest/path verified. It writes the record and raw transcripts under a new `final-attestation` retention-locked archive key and restores and replays verification against freshly downloaded assets before repository activation. Missing, tampered, unlocked, or unrestorable final-attestation state leaves both repository pointers unchanged; retry resumes from the immutable GitHub release without editing it.
13. It activates APT by replacing `InRelease` last. Only after the same release's APT state is durably active may it activate RPM by replacing the single mirrorlist pointer to the combined snapshot. Each format is independently atomic, but order is strictly APT then RPM; RPM-before-APT is rejected.
14. If APT activation fails, both repositories remain on their previous valid snapshots. If later RPM activation fails, APT remains on the new valid snapshot and RPM remains on its previous valid snapshot. No automatic downgrade occurs; retry continues from durable state without rebuild or resigning.
15. After both formats and both public-network canaries verify, it updates the signed channel/latest manifest and marks the release complete with the trusted `public_activated_at` timestamp used by retention.

### Signing Ownership

| Trust domain | Private-key location | Objects signed | Consumer trust |
|--------------|----------------------|----------------|----------------|
| Maintainer identity | Maintainer hardware/offline key only | Stable Codeberg annotated tags | Mirror fingerprint allowlist published out of band |
| Nightly tag | Dedicated offline primary plus short-lived online subkey in nightly-tag GPG home | Nightly annotated tags on both forges only | Publisher/mirror nightly-key allowlist; not package clients |
| Direct release | Dedicated offline primary plus short-lived online subkey in manifest GPG home | Final checksums, `release-manifest.json`, channel/latest manifest | Public direct-download keyring and fingerprints on GitHub plus `ze-software.net` |
| APT archive | Dedicated offline primary plus short-lived online subkey in APT GPG home | APT `Release`/`InRelease` only | `/usr/share/keyrings/ze-archive-keyring.gpg`, containing current and next APT public primaries |
| RPM archive | Dedicated offline primary plus separate short-lived online subkeys in RPM package/metadata GPG homes | RPM package signatures and detached `repomd.xml` signatures only | `/etc/pki/rpm-gpg/RPM-GPG-KEY-ze`, containing current and next RPM public primaries |
| Emergency operations | Dedicated offline-only primary | Short-lived rollback authorization objects only | Publisher operations-key allowlist; never installed as a package/repository trust root |
| Retention authorization | Dedicated offline primary plus short-lived online subkey in retention GPG home | Exact nightly prune plans after metadata activation | Root credential broker allowlist; not a client/package trust root |
| GitHub Actions attestations | GitHub OIDC/keyless service | Per-binary provenance/SPDX and aggregate input provenance | `gh attestation verify` with repository/workflow policy |
| GitHub immutable release | GitHub release attestation service | Immutable release and each final asset | `gh release verify` plus `gh release verify-asset`; versioned record and raw verifier transcripts archived |

Separate primary keys provide enforceable trust-domain separation that subkey naming alone cannot. Each online secret subkey is exported without its primary secret, stored under a dedicated locked OS account/GPG home, and delivered through a distinct systemd credential. Current and next public primaries ship before signer rotation; overlap tests prove old and new clients. Revocation freezes only the affected trust domain, publishes the revoked primary and replacement fingerprint through GitHub and `ze-software.net`, and rebuilds from archived attested inputs where required.

### Credential and Host Ownership

| Process | Execution identity | Forge permissions | Storage permissions | Signing access |
|---------|--------------------|-------------------|---------------------|----------------|
| Build | Ephemeral GitHub-hosted runner | `actions: read`, `contents: read`; checkout/fetch uses `persist-credentials: false`; no Checks/Attestations/Artifact-metadata/Contents write or OIDC | None | None |
| Attestation | Protected default-branch GitHub runner | `actions: read`, `contents: read`, `id-token: write`, `attestations: write`, `artifact-metadata: write`; no repository contents/release write | None | GitHub OIDC only |
| Evidence execution | Reset isolated runner | `contents: read` only for the trusted checkout step with `persist-credentials: false`; candidate commands receive no token, secret, OIDC, or write authority | None; evidence is an immutable run artifact verified by a separate job | None |
| Evidence recorder | Fresh protected-workflow GitHub runner; never checks out or executes candidate files | `actions: read`, `contents: read`, `checks: write`; binds caller/run/attempt/artifact ID/manifest digest; no release/tag/attestation write | None | None |
| Mirror/dispatch | Locked `ze-mirror` VPS account | Codeberg read-only deploy credential; GitHub mirror App with metadata read, contents read/write, Actions write, no Checks/Attestations administration; protected-main allowlist permits this App only for verified fast-forward | None | None |
| Publisher | Locked `ze-publisher` VPS account | GitHub publisher App with Actions/Checks/Attestations read and Contents read/write for nightly tags/releases, no Actions dispatch; Codeberg credential creates nightly tags while protected main/stable-tag rules reject it | Separate bucket-scoped Object Read & Write tokens for stable public, nightly public, and stable/nightly archive buckets; prefix locks deny immutable overwrite/delete; no bootstrap, retention-parent, or lock-admin token | May invoke only allowlisted no-network signer units; cannot read secret key files |
| Retention | Locked `ze-retention` VPS account | Run-scoped retention App token; policy accepts only expired `nightly-*` release/tag deletion; Codeberg nightly-delete credential; protected main/stable rules reject both | Receives only five-minute locally signed R2 credentials for `DeleteObject`/`DeleteObjects` and the broker-approved exact nightly-public or nightly-archive object keys; no read/list/put/copy/stable/bootstrap/current authority | May request a retention-plan signature but cannot read key files |
| Credential broker | Root-only locked no-network signer plus one networked deletion child | No forge permission | Parent Object Read & Write secrets for only `ze-nightly-public` and `ze-archive-nightly`; validates signed plan against root-readable append-only publisher activation state and monitor records, exact object keys/age/expired lock, then mints exact-object delete-only sessions | Retention public-key verification only |
| Monitor | Locked `ze-monitor` VPS account | Read-only forge/App metadata, Actions, Checks, Attestations, release, and ref access | Read-only public/private inventory and CDN probes | Public-key verification only |
| APT/RPM/direct/nightly/retention signers | One locked OS account and GPG home per online subkey | No network and no forge credential | Read one normalized staging input, write one signature/signed-RPM output, no bucket credential | Own exported secret subkey only through a distinct systemd credential |

Checked-in API-operation allowlists and protected branch/tag policies narrow platform permissions that GitHub/Codeberg cannot express as separate create/delete/tag/release scopes. Preflight probes every intended and denied operation. App/private-key tokens are minted only for one timer run, held in memory/systemd credentials, and revoked/rotated independently.

Candidate code runs only in Build and Evidence execution after authenticated checkout credentials have been removed. The evidence recorder downloads the evidence artifact by API ID, verifies its closed manifest/digests and trusted caller identity with protected workflow code, then writes the commit check; candidate code cannot access or influence its token.

For pruning, `ze-retention` creates a closed exact-key plan only after signed current metadata excludes the expired nightly. The retention signer records it, but the root broker independently re-verifies the signature against root-readable append-only publisher state containing the exact activated metadata bytes/digests and trusted `public_activated_at`, two-network monitor records newer than activation, both object age/lock expiry and at least 37 full days since public activation, nightly-only path grammar, high-water/predecessor state, and complete object inventory. Immediately before every credential-batch mint and again before launching its deletion child, the broker invokes the shared strict `chronyc tracking` parser; stale, malformed, unsynchronized, or threshold-failing time creates no session and performs no forge or object deletion. The broker partitions each bucket's approved keys into deterministic batches of 1-100, locally HS256-signs one R2 session per batch with TTL exactly 300 seconds, actions exactly `DeleteObject`/`DeleteObjects`, and `paths.objectPaths` exactly equal to that batch, then starts one sandboxed networked child with only one session at a time. Parent credentials never enter the retention account/child; raw tests prove sessions cannot read, list, put, copy, target unlisted objects, exceed TTL/batch limits, or access stable/bootstrap buckets.

GitHub rulesets make `main` fast-forward-only and nondeletable, allow only the mirror App to update it, deny publisher/retention Apps, and protect `.github/workflows/**` through the same branch rule. Stable tag pattern `[0-9][0-9].[0-9][0-9].[0-9][0-9]` permits mirror creation only and denies publisher/retention update/delete; nightly pattern `nightly-*` permits publisher creation and retention deletion only. Codeberg protected `main` and stable-tag policy give equivalent denials. Raw preflight proves source-ref create/update/delete, stable-tag update/delete, workflow mutation, administration, and attestation deletion fail for publisher/retention credentials; application-policy tests separately cover the forge API operations whose coarse Contents permission cannot deny, including stable release deletion.

### Publisher State and Replay Protection

| State | Durable evidence | Retry rule |
|-------|------------------|------------|
| discovered | forge tag/ref IDs or nightly source ID | Re-resolve and reject mismatch |
| verified | input/attestation/evidence/check/dependency digests | Never execute candidate files |
| input-archived | unique private input-archive key, inventory, retention proof | Signing blocked until input restore check passes |
| signed | final package/checksum/manifest/snapshot digests | Atomic signed outputs are reused; never sign the same final package twice |
| final-archived | unique private final-archive key, full digest inventory, retention proof | Public staging blocked until clean final-byte restore passes |
| github-staged | draft ID and downloaded outer-set digests | Resume missing uploads only |
| repositories-staged | public snapshot URLs and verified signatures | Active metadata unchanged |
| github-published | immutable release ID and API-observed prerelease/latest fields | Never edit or replace |
| release-attestation-archived | `final-attestation` key, release-specific record/transcript digests, asset/API inventory, lock/restore/replay proof | Both repository activations blocked until exact restore succeeds |
| apt-active | `InRelease` digest and generation | RPM remains old until this same release is durable |
| rpm-active | mirrorlist digest and generation plus matching `apt-active` predecessor | Reject RPM-before-APT; retry only the RPM transition |
| latest-active | signed latest-manifest digest | Only after both repositories and canaries verify |
| complete | append-only release record and trusted `public_activated_at` | No further mutation except nightly retention and metadata freshness refresh |

Every channel activation carries a signed monotonically increasing generation, release identity, and expected predecessor digest. The publisher keeps an append-only high-water mark and rejects non-increasing or unexpected-predecessor activation before signing or writing. APT-first is the only transition graph: `release-attestation-archived -> apt-active -> rpm-active -> latest-active -> complete`. Emergency rollback requires a short-lived object signed by the offline-only release-operations primary that names current and target digests, reason, expiry, and incident ID; it is never an ordinary retry path.

### Package First-Install Path

1. Package manager installs `/usr/bin/ze`, package-manager marker, vendor unit/format-specific preset, sysusers/tmpfiles declarations, transaction-state declaration, completions, license/copyright, and metadata. Repository source and key bundles are a jointly managed external bootstrap pair, never package payload.
2. DEB depends on `systemd`, `init-system-helpers`, and `ca-certificates`. RPM requires `systemd` and `ca-certificates`, with native scriptlet dependencies on systemd helpers.
3. On a fresh package install, `preinst`/`%pre` classifies `/etc/systemd/system/ze.service` with `lstat`: absent is accepted; the exact symlink to `/dev/null` is a preserved native mask; a regular admin unit, any other/broken symlink, directory, device, or other type rejects without changing any file. On upgrade/downgrade of an installed package, any existing admin unit/drop-ins are hashed and preserved byte-identically; package scripts never follow, chown, replace, or delete them.
4. Identity preflight accepts: both `ze` user/group absent; group present with user absent; or an existing locked system user whose primary group is `ze`, home is `/nonexistent`, and shell is `/usr/sbin/nologin` or `/sbin/nologin`. It preserves compatible numeric UID/GID and supplementary memberships. User-without-group, primary-group mismatch, unlocked password, home/shell mismatch, or name collision rejects before ownership changes.
5. Maintainer scripts use `systemd-sysusers`/native helpers after unpack to create any missing locked identity, verify the resulting contract, and create real non-symlink `/etc/ze` mode `0700`.
6. Before unpack or any other package-controlled mutation, `preinst`/`%pre` validates `/var/lib/ze-package` or creates it as an exact root-owned, root-group, non-symlink empty directory mode `0700`. Creation is the sole idempotent prerequisite before the journal can exist. A crash before creation changes nothing; a crash after creation leaves only that safe empty container, which the next pre-hook accepts. Final abort/removal may remove it only when the package is absent, no transaction exists, and it is still empty with the exact owner/mode/type. Tests crash immediately before/after directory creation and reject pre-existing nonempty, symlinked, wrong-owner, or wrong-mode containers.
7. A transaction file `/var/lib/ze-package/transaction.json` mode `0600` records family, old/new EVR, generated target digest, pre-action `ActiveState`/`UnitFileState`, stage, and retry intent before unpack. It is atomically updated, never follows a symlink, survives reboot/failure, and is removed only after successful configuration or final package removal.
8. If `/etc/ze/database.zefs` is absent, postinst invokes `ze init --automatic`. Automatic init uses username `admin`, requests 32 bytes from `crypto/rand`, encodes without truncation, stores only bcrypt/hash-as-token values, validates loopback-only active config, atomically installs mode-`0600` state, and discards plaintext.
9. If state exists, automatic mode requires `/etc/ze` to be a real directory with expected owner/mode and `database.zefs` to be a same-filesystem non-symlink regular file owned `ze:ze` mode `0600`. It opens then `fstat`s the file, verifies the `lstat`/open identity, reads zefs and active config read-only, and runs normal config validation. Only valid state returns success byte-identically; corrupt, unreadable, unsafe-type, symlinked, owner/mode-invalid, or invalid-config state fails without chown/mutation/start.
10. Short entropy reads and injected entropy, bcrypt, config-validation, write, close, rename, identity, state, transaction-container, or journal errors fail closed and remove only newly created temporary files or a provably empty orphan container.
11. All validation and daemon-reload work precedes selection of one final service intent. Native helpers apply the precedence table and reconcile the running executable before retrying, so a target process already started by an ambiguous prior call is never restarted again.

### Package Transaction State Machine

`transaction.json` is a closed schema with `schema=1`, family, operation (`install|upgrade|downgrade|reinstall|remove`), old/new EVR, generated target-binary SHA-256, pre-transaction `ActiveState`/`UnitFileState`, exact intent (`none|start|restart|stop-disable|abort-restore`), retry kind (`none|rpm-reinstall`), stage, triggering native hook/arguments, and created-container/identity/config flags. Hooks normally accept an existing transaction only when family, operation, EVRs, and target digest match. The sole exception is an RPM reinstall bridge: `%pre N>1` from the same package embeds the requested target EVR/digest, requires installed EVR and retained new EVR/digest to equal that target, preserves the original operation/old EVR/snapshot/intent, and changes only retry kind to `rpm-reinstall`. Mismatch or unknown fields/stages fail closed with the native recovery command.

| Forward-only stage | Producer | Durable effect and retry consumer |
|--------------------|----------|-----------------------------------|
| `snapshot-prepared` | old Debian `prerm upgrade` | On an already-installed package, queries and atomically records pre-upgrade `ActiveState`/`UnitFileState`, old/new EVR, and exact native arguments before new preinst/unpack; target digest is the only temporarily-null field |
| `prepared` | new `preinst`/`%pre` | Fresh install records explicit absent/inactive snapshot; Debian upgrade validates `snapshot-prepared` and fills its embedded target digest; RPM records its pre-state directly; all unit/identity/state preflight is complete before unpack |
| `payload-present` | `postinst configure`/`%post` | Package DB and installed binary digest verified; compatible identity/directories created or reused |
| `validated` | same | Target binary's read-only state/config validation passed; all bootstrap temporaries cleaned |
| `reloaded` | same | `daemon-reload` succeeded, or offline/non-systemd policy recorded an explicit no-action result |
| `action-pending` | same | Current mask/policy/enablement re-read and one exact final intent selected |
| `action-running` | same, atomically before `systemctl` | Retry queries `ActiveState`, `MainPID`, and `/proc/<pid>/exe` digest: exact target already active completes without another call; old active invokes the pending restart; inactive with recorded active/start intent invokes start; denied/masked/no-action completes without service mutation |
| `configured` | same | Reconciled result and target digest recorded; transaction removed immediately before successful hook return. A DEB retry with no transaction may reconstruct only from dpkg old-version arguments, current policy, and running target/old digest. RPM never reconstructs an interrupted original transaction from scriptlet count; a journal-free same-EVR `dnf reinstall` creates a new `reinstall` transaction and reconciles current target state without a default start assumption |
| `install-abort-pending` / `install-aborted` | `postrm abort-install` | Verify dpkg is `not-installed`, remove only declared new temporaries and a provably empty orphan transaction container, then remove the matching fresh-install journal; before/after-crash retry repeats the same predicates |
| `upgrade-abort-pending` / `upgrade-abort-cleaned` / `upgrade-aborted` | old `preinst abort-upgrade`, new `postrm abort-upgrade`, old `postinst abort-upgrade` | Exact native form advances only its stage: preserve old process/policy/config/identity, clean only declared new temporaries in new postrm, and let old postinst record terminal restoration without assuming dpkg has already marked the old package configured. The next package pre-hook may remove `upgrade-aborted` only after `dpkg-query` proves that exact old EVR configured; new-unpacked, half-installed, or any mismatched state retains/fails closed |
| `remove-prepared` / `remove-action-running` / `removed` | `prerm remove`/RPM final erase, then `postrm remove` | Pre-remove activity/enablement recorded; stop/disable is reconciled idempotently; final removal clears package transaction state |
| `abort-restore-running` / `abort-restored` | `postinst abort-remove`/`abort-deconfigure` | Restore only recorded pre-remove enable/activity allowed by current mask/policy and reconcile the target process. Because dpkg commits restored state only after the hook returns, retain terminal `abort-restored`; a later pre-hook clears it only after exact configured-EVR proof |

Every transition is write-fsync-rename-fsync-dir atomic. A crash or injected failure retains the last complete stage. Identity/directories created after unpack may remain only in their already-valid locked/empty form; config/state is either the prior byte-identical tree or a fully validated atomic bootstrap. Tests interrupt before and after every transition, each abort producer/consumer, terminal-journal removal, and each service call.

### Service State Precedence

Safety overrides service intent: invalid state/identity/unit, offline/chroot, Debian `policy-rc.d` denial, an RPM administrator preset denial, and a native mask are evaluated first. Otherwise enablement and activity are independent: preserve pre-transaction `UnitFileState`; when pre-transaction `ActiveState=active`, reconcile until the target binary is active without a duplicate successful restart; never start an inactive upgrade.

| Pre-transaction condition | Fresh install result | Upgrade/downgrade/retry result |
|---------------------------|----------------------|--------------------------------|
| Ordinary fresh host, no mask, native preset permits | Apply native enable policy and start after successful bootstrap | N/A |
| `enabled` + `active` | N/A | Remain enabled; if target digest is not active, perform the reconciled restart intent |
| `enabled` + `inactive` | N/A | Remain enabled and inactive |
| `disabled` + `active` | N/A | Remain disabled; if target digest is not active, perform the reconciled restart intent |
| `disabled` + `inactive` | N/A | Remain disabled and inactive |
| exact `/dev/null` mask + inactive | Preserve mask, bootstrap, remain inactive, warn | Preserve mask and inactive state; no restart |
| exact `/dev/null` mask + active | N/A | Preserve mask and currently running process; do not restart automatically; warn that new bytes take effect only after explicit admin action |
| Debian `policy-rc.d` denies action | Configure/enable metadata, remain inactive, succeed | Preserve process/activity and enablement; never bypass denial |
| RPM administrator preset resolves disabled | Bootstrap, remain disabled and inactive, succeed | Current administrator preset/policy participates in the ordinary upgrade precedence; never creates start intent |
| Offline root/chroot or systemd not PID 1 | Bootstrap and enable metadata where helper permits; do not start; succeed | Preserve state/enablement metadata; no service action |

If administrator policy differs from the transaction snapshot on retry, the current mask or a change from enabled to disabled wins; a change from disabled to enabled never creates start intent. Service and package tests cover every table row on DEB and RPM.

### Native Failure and Retry States

| Failure point | On-disk/package state | Process/unit state | Supported retry |
|---------------|-----------------------|--------------------|-----------------|
| `preinst`/`%pre` before unpack | Old or absent payload/database unchanged | Unchanged | Correct cause and rerun package command |
| Fresh DEB post-unpack validation/configuration | New payload unpacked, dpkg unconfigured, temp cleaned, transaction retained | Never started; mask/policy unchanged | `dpkg --configure -a` |
| Fresh RPM `%post` validation/configuration | New payload installed in RPM DB, temp cleaned, transaction retained | Never started; mask/policy unchanged | `dnf reinstall ze` |
| DEB upgrade after unpack, before service action | New binary on disk, dpkg unpacked/unconfigured, state/config unchanged | Previously active process keeps old mapped binary; inactive stays inactive; enablement unchanged | `dpkg --configure -a`; retry validates and reconciles one recorded final intent against process digest |
| RPM upgrade after unpack, before service action | New binary and package installed in RPM DB, state/config unchanged | Previously active process keeps old mapped binary; inactive stays inactive; enablement unchanged | `dnf reinstall ze`; retry validates and reconciles one recorded final intent against process digest |
| DEB/RPM ambiguous or failed restart/start | New binary on disk, format-specific package state above, transaction remains `action-running` | Target may be active despite a timeout, old process may remain, or unit may be failed/inactive | Same native retry; exact target active completes without a call, otherwise recorded active intent retries only if current safety/policy permits |

#### Debian Maintainer-Script Dispatch

All Debian Policy argument forms are explicit; unknown/malformed arguments fail before mutation.

| Script and accepted arguments | Required behavior |
|-------------------------------|-------------------|
| `preinst install [old-version new-version]`; `preinst upgrade old-version new-version` | Preflight and write/resume only `prepared`; never call systemd or depend on unpacked payload. Upgrade requires the matching `snapshot-prepared` record from old `prerm` and fills only its target digest |
| old `preinst abort-upgrade new-version` | No service action or payload assumption; preserve old process/config and write/resume matching `upgrade-abort-pending` only when a matching journal exists, otherwise succeed as a byte-preserving no-op |
| `postinst configure [old-version]` | Sole install/upgrade consumer of `payload-present` through `configured`; reconcile before any retry action |
| old `postinst abort-upgrade new-version` | Preserve old process, policy, config, and identity; if a matching journal exists, write terminal `upgrade-aborted`, otherwise succeed as a byte-preserving no-op. Never remove a journal or claim configured state before dpkg commits the successful return |
| `postinst abort-remove [in-favour package version]`; `postinst abort-deconfigure in-favour package version [removing package version]` | Consume the removal snapshot through terminal `abort-restored`; restore no more than recorded pre-remove intent under current safety policy and leave later exact-state cleanup to a subsequent pre-hook |
| old `prerm upgrade new-version`; new `prerm failed-upgrade old-version new-version`; `prerm deconfigure in-favour package version [removing package version]` | Old `prerm upgrade` uses its configured dependencies to run the bounded systemd/offline classification and atomically create `snapshot-prepared`. If it fails, new `prerm failed-upgrade` may validate and resume only an already-complete matching snapshot; under preinst constraints it never queries systemd or invents missing activity. Missing/partial snapshot fails closed into dpkg's old-postinst unwind. Neither form marks an abort or performs service action. `prerm deconfigure` writes/resumes only its matching removal snapshot |
| `prerm remove [in-favour package version]` | Run only `remove-prepared`/`remove-action-running`; stop/disable once with reconciliation |
| old `postrm upgrade new-version`; new `postrm failed-upgrade old-version new-version`; new `postrm abort-install [old-version new-version]`; new `postrm abort-upgrade old-version new-version` | Normal old `postrm upgrade` and successful recovery `postrm failed-upgrade` retain the continuing forward journal unchanged. `abort-install` consumes only install-abort stages. `abort-upgrade` writes/advances only upgrade-abort stages and never removes them before old-postinst/dpkg restoration completes |
| `postrm remove` | Preserve `/etc/ze` and external repository source/key pair; clear package transaction state |
| `postrm purge` | Remove `/etc/ze` and package-created markers, preserve external repository source/key pair and `ze` identity, clear transaction state |
| `postrm disappear overwriter overwriter-version` | No service mutation; preserve operator state, clean matching package transaction state |

Container failure injection executes every row and both continue-versus-unwind branches, including old-`prerm` snapshot failure recovered by new `prerm failed-upgrade`, old-`postrm` failure recovered by new `postrm failed-upgrade`, `preinst/postinst abort-upgrade`, `postinst abort-remove`, `postinst abort-deconfigure`, `postrm abort-install`, and `postrm disappear`, with crashes before/after each stage and terminal cleanup, and proves exact package DB/process/disk state plus native retry.

#### RPM Scriptlet Dispatch

RPM scriptlets accept only numeric transaction counts. The package embeds its own target EVR and binary digest, queries the installed EVR, and uses the durable journal rather than inferring the original operation from count alone:

| Script/count | Required behavior |
|--------------|-------------------|
| new `%pre 1` | Fresh install preflight; create only `prepared`; no service call or unpacked-payload dependency |
| new `%pre N` where `N>1` | If a nonterminal install/upgrade/downgrade journal targets the same embedded EVR/digest and that EVR is installed, enter only the `rpm-reinstall` retry bridge without overwriting original old EVR, pre-state, stage, or intent. With no journal and installed EVR equal target, create a closed `reinstall` transaction from current policy/state. Otherwise derive upgrade/downgrade from installed versus embedded target EVR and create `prepared` |
| new `%post 1`; new `%post N` where `N>1` | Verify installed target EVR/digest and consume the original or reinstall transaction from `payload-present` through `configured`; fresh path applies preset, every non-fresh path preserves the recorded policy |
| old `%preun 0` | Final erase only: consume `remove-prepared`/`remove-action-running` and reconcile stop/disable |
| old `%preun N` where `N>=1` | Upgrade/downgrade/reinstall: no service action; preserve matching transaction |
| old `%postun 0` | Final erase cleanup: preserve `/etc/ze` and external repository pair, clear transaction |
| old `%postun N` where `N>=1` | Upgrade/downgrade/reinstall: no daemon reload, stop, disable, transaction deletion, or new-payload mutation |

Zero, negative, nonnumeric, missing, overflow, unexpected script/count combinations, and injected scriptlet failures fail closed. Rocky/Fedora tests exercise install, multi-version upgrade/downgrade counts, final erase, fresh-install and upgrade `%post` failures followed by the exact `dnf reinstall ze`, crash after terminal-journal removal, journal-free ordinary reinstall, and prove old postun cannot undo the new package's service state.

Upgrade and downgrade run the newly installed binary's read-only state/config validation before service action. Incompatible state leaves the old process running when possible and the transaction retryable; documentation gives the exact package-manager recovery/reinstall path.

### Upgrade, Removal, and Package-Managed Update Path

1. Upgrade/downgrade verifies repository/package trust, installs the new binary/unit, preserves `/etc/ze`, never bootstraps, validates state read-only, reloads systemd, and follows the transaction/precedence/failure tables.
2. DEB remove and RPM erase stop/disable only on final removal, remove package-owned files and `/var/lib/ze-package`, preserve `/etc/ze`, and leave the externally bootstrapped repository source/key pair together so native refresh and reinstall continue to work.
3. Explicit DEB purge removes `/etc/ze` and package-created markers but preserves the external repository pair and `ze` user/group because external files may use them. RPM has no invented purge; explicit state/account/repository cleanup is documented as separate operator actions.
4. Upgrade preserves administrator unit override; doctor reports effective fragment/drop-ins/executable. Fresh installs warn when `/usr/local/bin/ze` exists and preserve it byte-for-byte.
5. `BackendOptions.InstallMethodPath` defaults to `/usr/share/ze/install-method`. `NewBackend` resolves a regular root-owned `deb`/`rpm` marker before platform selection and chooses registered `BackendPackageManaged`; missing marker preserves the existing platform backend, while unreadable/unsafe/unknown marker fails backend construction.
6. `backend_package_managed.go` registers `BackendPackageManaged`. `Start`/`Stop` are no-ops and `History` is empty. `Status`/`Check` remain non-networked, keep the backend active even with no URL, and use the exact `FirmwareResult.Message` `package-managed installation: run 'apt-get update && apt-get install --only-upgrade ze' on the host` for `deb`, or `package-managed installation: run 'dnf upgrade ze' on the host` for `rpm`. `Download`, `Apply`, `Restart`, and `Rollback` all return the same format-specific result plus `ErrFirmwareUnsupported` and create no staged/`.prev` file.
7. `cmd/ze/hub/main_system.go` passes the marker path; `internal/plugins/update-cmd/cmd/firmware.go` handlers and `handleShowSystemUpdate` in `show.go` expose the same backend/status/error contract. Auto-apply/restart config never constructs or starts `SelfUpdater`; doctor emits registered `doctor-update-package-managed`.
8. Unit, `.ci`, and booted-VM tests invoke real `show system update`, check, manual download, background auto-apply startup, apply, restart, and rollback entry points for both `deb` and `rpm` markers with pre-existing staged/`.prev` files, assert the exact guidance literal at the dispatcher boundary, and assert no new temp/stage file plus unchanged `/usr/bin/ze`.

### Package File Contract

| Path or state | Owner/group | Mode | Contract |
|---------------|-------------|------|----------|
| `/usr/bin/ze` | root/root | `0755` | Exact attested architecture binary; regular file, no set-id bits |
| `/usr/lib/systemd/system/ze.service` | root/root | `0644` | Vendor unit matching generated semantics |
| `/usr/lib/systemd/system-preset/50-ze.preset` in RPM | root/root | `0644` | Enables Ze through native RPM preset policy; administrator `/etc` policy remains authoritative |
| `/usr/lib/sysusers.d/ze.conf`, `/usr/lib/tmpfiles.d/ze.conf` | root/root | `0644` | Account and state-directory declarations |
| `/usr/lib/tmpfiles.d/ze-package.conf` | root/root | `0644` | Declares root-only transaction directory `/var/lib/ze-package` mode `0700` |
| `/usr/share/bash-completion/completions/ze` | root/root | `0644` | Generated Bash completion |
| `/usr/share/zsh/site-functions/_ze` | root/root | `0644` | Generated Zsh completion |
| `/usr/share/fish/vendor_completions.d/ze.fish` | root/root | `0644` | Generated Fish completion; Nushell remains available through `ze completion nushell` because there is no universal system destination |
| `/usr/share/doc/ze/copyright` in DEB; `/usr/share/licenses/ze/LICENSE` in RPM | root/root | `0644` | Format-native AGPL license/copyright payload |
| `/usr/share/ze/install-method` | root/root | `0644` | Format value `deb` or `rpm`; selects package-managed update behavior |
| `/etc/apt/sources.list.d/ze.sources` plus `/usr/share/keyrings/ze-archive-keyring.gpg`; `/etc/yum.repos.d/ze.repo` plus its current/next key file/imports | root/root | `0644` | Coupled external bootstrap state, never package payload; source activates last, source removes first, and package remove/purge/erase preserves both for refresh/reinstall |
| `/etc/ze` | ze/ze | `0700` | Mutable operator state; never package payload content |
| `/etc/ze/database.zefs` and bootstrap temp | ze/ze after postinst | `0600` | Atomic state; temp removed on every failure |
| Generated private host key | ze/ze | `0600` | Preserved on upgrade/remove; removed only by DEB purge |

Unexpected symlinks, devices, set-id bits, world-writable paths, undeclared files, or wrong owner/mode fail package tests.

The production entry point `packaging/repository/install-ze-repository.sh` supports only `install [deb|rpm|auto]` and `remove [deb|rpm|auto]`, requires root, working OS CA trust, a verified HTTPS-capable `curl`, `gpg`, and the native APT or RPM key/query tools, and has no package dependency on Ze. `auto` succeeds only when exactly one supported native family is detected and rejects absent or ambiguous managers. The checked-in script is published under its digest as an immutable bootstrap object with a signed checksum and the exact immutable URL/digest in GitHub release/docs; documentation downloads it without piping to a shell, verifies the signed checksum and out-of-band direct-release fingerprint, then executes the local file. Install downloads the fingerprinted current/next key bundle and matching reviewed source template from `bootstrap.packages.ze-software.net` into root-only temporary files, verifies exact out-of-band primary fingerprints and closed URI/`Signed-By`/`gpgkey` fields, installs the key file/imports first, and renames the source/repo file last. Any failure before the last rename leaves no active Ze source; cleanup removes temporary files but may retain a harmless verified key. Remove renames/removes the source first, proves APT/DNF no longer selects Ze, then removes only the exact expected key file/import fingerprints. It refuses unknown arguments, paths, roots, pre-existing conflicting files, tool failures, fingerprint drift, and partial trust state. `make ze-release-repository-bootstrap-test` and all six package profiles invoke this same script with local signed fixtures and failure injection at every operation; they prove there is never an active source with missing trust, package lifecycle never mutates the pair, and refresh plus reinstall works after package removal.

### Repository Layout and Versioning

- Stable Git identity and embedded release: `YY.MM.DD`; package release/revision is `1`.
- Nightly Git identity: `nightly-YY.MM.DD-g<12-hex-sha>`; embedded release remains `YY.MM.DD`.
- Stable DEB: `YY.MM.DD-1`.
- Stable RPM: Version `YY.MM.DD`, Release `1`.
- Nightly DEB: `YY.MM.DD~nightly.g<sha>-1`.
- Nightly RPM: Version `YY.MM.DD`, Release `0.nightly.g<sha>.1`.
- Native comparison tests prove: previous stable < next-day nightly and same-day nightly < same-day stable. Same-day stable is a normal upgrade; returning immediately from a newer next-day nightly to an older stable is an explicit native downgrade, never an implicit source switch.
- APT channels are `stable` and `nightly`, component `main`, suites/codenames equal the channel, architectures `amd64` and `arm64`, with `Acquire-By-Hash: yes`.
- RPM uses separate `ze-stable` and `ze-nightly` IDs. Each release has one combined immutable repository snapshot containing x86_64/aarch64 packages and one root repodata tree; DNF filters compatible architecture.
- Direct GitHub DEBs are authenticated by the signed payload manifest/checksum envelope and GitHub immutable-release attestation. APT authenticates the same DEB bytes through signed `InRelease`, `Packages` hashes, and package digest. No embedded DEB signature is claimed.

Immediate nightly-to-stable return is documented and tested as a state-preserving downgrade. APT disables the nightly stanza, enables stable, runs `apt-get update`, then runs `apt-get install --allow-downgrades "ze=<stable-deb-version>"` using the exact version from authenticated stable `Packages`. DNF disables `ze-nightly`, enables `ze-stable`, then runs `dnf --disablerepo=ze-nightly --enablerepo=ze-stable distro-sync ze`. Both run target-binary read-only config validation before service action; incompatible state fails in the native retry state and documentation restores the nightly package explicitly.

`<id>` is the Git identity (`YY.MM.DD` or `nightly-YY.MM.DD-g<sha>`). Canonical public asset names are:

| Asset | Canonical name |
|-------|----------------|
| Tarball | `ze_<id>_linux_<goarch>.tar.gz` |
| DEB | `ze_<id>_<debarch>.deb` |
| RPM | `ze-<rpm-version>-<rpm-release>.<rpmarch>.rpm` |
| SPDX SBOM | `ze_<id>_linux_<goarch>.spdx.json` |
| Exported provenance/SPDX attestation bundle | `ze_<id>_linux_<goarch>.attestation.jsonl` |
| Aggregate input provenance attestation | `ze_<id>_release-input.attestation.jsonl` |
| Final asset manifest and signature | `release-manifest.json`, `release-manifest.json.asc` |
| Final checksum list and signature | `SHA256SUMS`, `SHA256SUMS.asc` |

Each tarball has one top directory `ze-<id>-linux-<goarch>/` containing only `ze` mode `0755` and `LICENSE` mode `0644`; names, owner/group zero, timestamps, gzip header, and order are canonical. The payload manifest closes non-envelope assets; its schema closes the four envelope names; GitHub's immutable-release attestation plus `gh release verify`/`verify-asset` closes the complete public outer set.

Public DNS maps one custom domain to each selected public R2 bucket, with no path router and no public/archive origin:

| Public URL/object | Bucket/lifecycle | Signer | Cache and commit contract |
|-------------------|------------------|--------|---------------------------|
| `https://bootstrap.packages.ze-software.net/keys/<fingerprint>/{ze-apt-archive.gpg,RPM-GPG-KEY-ze,ze-release-manifest.asc}` | `ze-bootstrap`, immutable fingerprinted prefixes | Offline key ceremony | `max-age=31536000, immutable`; bucket credential absent from publisher |
| `https://bootstrap.packages.ze-software.net/{keys/<current-name>,config/ze.sources,config/ze.repo}` | `ze-bootstrap`, reviewed current/next bundles/config | Offline key ceremony | `max-age=3600, must-revalidate`; out-of-band fingerprints |
| `https://bootstrap.packages.ze-software.net/install/<script-digest>/{install-ze-repository.sh,SHA256SUMS,SHA256SUMS.asc}` | `ze-bootstrap`, immutable digest prefix | Reviewed offline bootstrap ceremony using direct-release key | `max-age=31536000, immutable`; exact URL/digest copied to GitHub release and docs; publisher has no bucket credential |
| `https://packages.ze-software.net/{releases/stable/<id>/<manifest-digest>/...,apt/pool/stable/...,apt/dists/stable/**/by-hash/...,rpm/snapshots/stable/<id>/<snapshot-digest>/...}` | `ze-stable-public`, indefinite prefix bucket locks | Final objects already signed/attested | `max-age=31536000, immutable`; bucket-scoped writer creates, locks deny overwrite/delete |
| `https://packages.ze-software.net/{apt/dists/stable/{Release,Release.gpg,InRelease,main/binary-<arch>/Packages*},rpm/stable/mirrorlist,channels/stable/release-manifest.json{,.asc}}` | unlocked current prefixes in `ze-stable-public` | APT/direct domains as applicable | `max-age=60, must-revalidate`; conditional predecessor; `InRelease`/mirrorlist are format commit objects |
| `https://nightly.packages.ze-software.net/{releases/nightly/<id>/<manifest-digest>/...,apt/pool/nightly/...,apt/dists/nightly/**/by-hash/...,rpm/snapshots/nightly/<id>/<snapshot-digest>/...}` | `ze-nightly-public`, 37-day prefix bucket locks | Final objects already signed/attested | `max-age=31536000, immutable`; brokered prune only after lock and grace |
| `https://nightly.packages.ze-software.net/{apt/dists/nightly/{Release,Release.gpg,InRelease,main/binary-<arch>/Packages*},rpm/nightly/mirrorlist,channels/nightly/release-manifest.json{,.asc}}` | unlocked current prefixes in `ze-nightly-public` | APT/direct domains as applicable | `max-age=60, must-revalidate`; conditional predecessor; publisher re-signs metadata before prune |
| private `/archive/stable/<id>/<digest>/{input,final,final-attestation,refresh}/...` | `ze-archive-stable`, indefinite prefix bucket lock | Digest inventory signed by direct domain | No public/custom-domain route; bucket writer creates, lock denies overwrite/delete; read-only monitor |
| private `/archive/nightly/<id>/<digest>/{input,final,final-attestation,refresh}/...` | `ze-archive-nightly`, 37-day prefix bucket lock plus 37 days from trusted public activation | Digest inventory signed by direct domain | No public/custom-domain route; brokered prune only after both lock expiry and activation grace |

R2/API tokens are bucket-scoped, not assumed prefix-scoped. The topology is exactly five buckets: bootstrap, stable public, nightly public, stable archive, and nightly archive. Provider-native prefix lock rules, not token claims, protect immutable paths within the two public and two archive buckets; lock-administration credentials remain offline. Preflight proves each custom domain resolves only its selected public bucket, publisher cannot delete/overwrite locked objects, brokered prune cannot access stable/bootstrap/current or unlisted objects, and no public route reaches either archive.

APT ships two deb822 stanzas: stable uses `URIs: https://packages.ze-software.net/apt` with `Suites: stable`; disabled-by-default nightly uses `URIs: https://nightly.packages.ze-software.net/apt` with `Suites: nightly`. Both use `Components: main`, `Architectures: amd64 arm64`, dedicated `Signed-By`, `Check-Date: yes`, `Check-Valid-Until: yes`, `Valid-Until-Max: 172800`, and `Date-Max-Future: 600`. `Release`/`InRelease` contains `Date`, `Valid-Until=Date+48h`, Suite/Codename, Architectures, Components, and Acquire-By-Hash. Every 12 hours, even without a package release, the publisher increments the APT-specific generation, binds the previous `InRelease` digest, signs fresh `Release`/`Release.gpg`/`InRelease`, locks their inventory and bytes under a unique `refresh` archive key, conditionally writes `Release`/`Release.gpg`, and commits with `InRelease` last; pool/by-hash objects and RPM state do not change. Warning begins with 24 hours validity remaining and clients fail closed after expiry or more than 10 minutes future skew.

`/config/ze.repo` defines `ze-stable` and disabled-by-default `ze-nightly`, format-specific mirrorlists, `gpgcheck=1`, `repo_gpgcheck=1`, `metadata_expire=15m`, and `http_caching=none`. Each mirrorlist is exactly one newline-terminated immutable snapshot HTTPS URL. Conforming DNF plus honest CDN/cache state may remain stale for at most 16 minutes; OpenPGP authenticates content but native DNF does not cryptographically reject an actively malicious TLS/CDN replay. Two-network generation monitoring detects ordinary stale state and freezes new publication, while this residual malicious-CDN freeze threat is documented rather than mislabeled as client rollback protection.

### Effective Systemd Unit Discovery

Doctor preserves the existing `ze.test.doctor.service-unit` test override. Without that override on a systemd host, it runs exactly one injected command with a two-second context deadline and 64 KiB combined-output limit:

`systemctl show ze.service --no-pager --property=LoadState --property=FragmentPath --property=DropInPaths --property=ExecStart --property=User --property=Group --property=UnitFileState --property=ActiveState`

The parser requires one `KEY=value` record per requested property, rejects duplicate/unknown/missing/malformed records, uses `FragmentPath` only for fragment file checks, and uses systemd's effective `ExecStart` serialization so drop-in overrides are not missed. `LoadState=not-found` means no installed service. Missing `systemctl`, nonzero exit/systemd unavailable, timeout, oversized output, or malformed output emits new registered `doctor-service-query`; unreadable fragment and invalid effective executable/user/group retain their existing specific codes. Tests inject vendor/admin/drop-in success, not-found, unavailable, nonzero, malformed, duplicate, oversized, and a command blocked immediately beyond the two-second deadline.

### Operations and Monitoring Policy

| Signal/process | Cadence and timeout | Warning | Critical/freeze |
|----------------|---------------------|---------|-----------------|
| Mirror poll | Every 5 minutes; 60-second run timeout | Canonical-to-GitHub lag over 10 minutes | Divergence immediately or lag over 30 minutes; freeze dispatch/publication |
| Trusted VPS clock | `chrony` required; `chronyc tracking` before every identity/sign/activation/prune-plan decision, every deletion-credential mint and child launch, and every 5 minutes | absolute system offset over 250 ms or last update over 5 minutes | Leap status not `Normal`, absolute offset over 1 second, last update over 10 minutes, invalid stratum, or command failure; freeze dispatch/sign/activation/prune and mint no delete session |
| Publisher poll/state | Every 2 minutes; 30-second API calls | One transition pending over 30 minutes | Pending over 60 minutes or unexpected state/predecessor; freeze new publication |
| VPS public/CDN probe | Every 5 minutes; 30 seconds per endpoint | First origin/public digest or generation mismatch | Three consecutive failures or generation over 10 minutes behind origin; freeze new publication |
| Independent GitHub public monitor | `47 * * * *`; 10-minute job timeout | Last successful external probe older than 90 minutes | Older than 2 hours; freeze new publication |
| APT freshness | Re-sign every 12 hours | Less than 24 hours remains before `Valid-Until` | Less than 12 hours or refresh failure twice; freeze and page before client fail-closed deadline |
| DNF honest-cache freshness | Probe current mirrorlist/repomd every 5 minutes | Generation over 5 minutes behind origin | Over 10 minutes or three failures; freeze; native malicious-CDN limitation remains documented |
| Online signing subkeys | Daily | Expiry within 60 days | Expiry within 30 days pages; within 14 days freezes affected trust domain |
| Forge App/private-key rotation | Daily age check | 75 days since rotation | 90 days freezes affected automation until rotation |
| Per-release input/final/final-attestation/refresh archive | Input/final/final-attestation synchronous, every refresh synchronous, plus daily inventory | First missing/digest/lock mismatch | Any repeat or failed restore blocks/freeze before the dependent public state advances |
| Clean-room restore rehearsal | Every 90 days | 75 days since last success | 90 days freezes publication |
| Nightly retention | `17 4 * * *`; 60-minute timeout | Object eligible by both lock expiry and `public_activated_at+37d` remains 6 hours | 24 hours late or deletion-order mismatch; retain rather than delete aggressively |

VPS monitor writes structured JSON to journald and POSTs fixed-schema alerts to one HTTPS webhook URL loaded as `ZE_RELEASE_ALERT_WEBHOOK` through a systemd credential. It deduplicates the same code/resource for 30 minutes, increments occurrence count, and emits one recovery after two consecutive healthy probes. Critical signals block new dispatch/sign/activation but never roll back a valid active repository. Unit tests use an injected clock and fake HTTP/forge/storage clients for every threshold, deduplication, freeze, and recovery transition.

`install_publisher.py` requires but does not rewrite operator NTP sources: it installs/enables `chrony`, orders release units after `chrony.service`/`time-sync.target`, and fails preflight until `chronyc tracking` meets the critical thresholds. `preflight.py`, mirror, publisher, monitor, retention, and the root credential broker share one strict parser and injected-clock seam. Staging feeds normal/stale/unsynchronized/malformed tracking records and proves a healthy signed plan followed by a stale broker clock cannot authorize a CalVer tag, freshness signature, prune plan, delete session/child, forge deletion, object deletion, or activation.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Codeberg source -> GitHub mirror/dispatch | Fast-forward branch, exact tag object, protected default-branch workflow dispatch | Mirror/dispatch integration test |
| Git ref -> release identity | Strict calendar parser | Python unit and boundary tests |
| Source -> binaries | Deterministic Make/Go invocation with exact feature set | Double-build, build-tag, inventory, extended-version tests |
| Binary -> native package | nFPM closed content manifest | Extracted binary digest, path/owner/mode tests |
| Build -> trusted attestation | Artifact IDs consumed by protected `workflow_run` | Workflow-policy and attestation-negative tests |
| GitHub/Codeberg -> VPS automation | Separate mirror, publisher, and nightly-retention App/deploy identities plus protected refs | App-scope, API-allowlist, denied-operation, and token-rotation tests |
| Publisher -> signing keys | Dedicated GPG homes/systemd credentials by signature class | Clean-keyring and key-rotation tests |
| Publisher -> GitHub release | Draft upload, final-manifest download verification, immutable publish | Staging stable/nightly scenarios |
| Publisher -> public/private object storage | Immutable snapshots/audit archive plus monotonic pointer state | Fake-S3, staging, restore, replay tests |
| Object storage -> APT client | HTTPS, `Signed-By`, signed `InRelease`, by-hash indexes | Fresh install and tamper negatives |
| Object storage -> DNF client | HTTPS mirrorlist, signed RPM and repomd | Native amd64/arm64 install and tamper negatives |
| Package manager -> zefs/systemd | `ze init --automatic` plus native scriptlets | `.ci`, container, and booted VM lifecycle tests |
| Package marker -> update backend | Installed method marker and doctor config check | Unit, `.ci`, and package VM mutation-negative tests |
| Vendor unit -> doctor | Bounded effective `FragmentPath` discovery | Doctor unit and functional tests |

### Integration Points

- `feature-gates.txt` and `Makefile:48-51` remain the only source of default feature tags.
- `Makefile:53-56` accepts explicit release metadata from the release builder while preserving local defaults.
- `internal/plugins/init/main.go` owns automatic bootstrap and reuses existing zefs keys, bcrypt, and atomic rename.
- `internal/plugins/systemd/unit.go` remains the operational source for service semantics; a parity test prevents package/vendor unit drift.
- `internal/component/doctor/checks_platform.go` discovers the effective systemd unit with a bounded call and preserves test overrides.
- `internal/component/config/system` detects package ownership, prevents in-place mutation, and exposes a doctor diagnostic for incompatible `auto-apply`.
- `mk/test-release.mk` adds package/repository evidence to the existing release matrix.
- GitHub Actions builds and attests; separate VPS mirror/publisher identities own forge dispatch, final signing, repository activation, and retention.

### Architectural Verification

- [ ] No bypassed layers: maintainers authorize stable releases with signed canonical tags; publisher never fabricates source identity.
- [ ] No unintended coupling: build jobs have no publishing secrets; publisher handles only verified files and metadata.
- [ ] No duplicated build model: feature gates and Go entry point remain sourced from existing repository producers.
- [ ] No mutable stable artifacts: all packages and releases are immutable; only explicit channel pointers change.
- [ ] No secret-bearing package state: package payload never contains `database.zefs`, config, credentials, private keys, or signing material.
- [ ] No silent service shadowing: admin and vendor unit conflicts fail closed.
- [ ] Zero-copy is not applicable to release tooling; runtime hot paths are untouched.
- [ ] Registration over hardcoding remains preserved; the existing init/systemd/doctor owners receive the behavior.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | GitHub immutable releases and release attestations are enabled for `ze-software/ze` | GitHub supports repository/org immutable-release policy and final-release attestation | Published assets could change or final bytes would lack trusted provenance | Preflight API/config check and throwaway draft/publish rehearsal | unvalidated |
| A-2 | Codeberg supports protected `main`, restricted stable tag creation, and deploy-key/App automation needed by the mirror | Canonical forge requirement | Unauthorized source/tag identity could enter the queue | Provisioning checklist plus unauthorized-tag and divergence exercises | unvalidated |
| A-3 | `ubuntu-24.04-arm` remains available for this public GitHub repository | GitHub-hosted runner reference, checked 2026-07-10 | Native arm64 package smoke cannot run there | Workflow dry run; fall back to an isolated self-hosted arm64 runner only with explicit user approval | unvalidated |
| A-4 | The selected R2 deployment supports three one-bucket custom domains, conditional writes, five bucket-scoped credentials, prefix bucket locks, and locally signed temporary credentials with explicit actions/object paths | R2 custom-domain, bucket-lock, API-token, and temporary-credential documentation | Atomic activation, routing, immutable retention, or delete authority would be unsafe | Five-bucket/three-domain staging, prefix-lock, broker-mint, credential-denial, CDN, and restore tests | unvalidated |
| A-5 | nFPM 2.47.0, Syft 1.46.0, Aptly 1.6.3, and createrepo_c 1.2.4 satisfy deterministic and target-format contracts | Latest upstream releases observed 2026-07-10 | Tool lock or repository design changes | Pinned prototype, double-build, native package-manager, and clean-keyring tests | unvalidated |
| A-6 | Current `ze` distro binary remains fully static under both target architectures | `CGO_ENABLED=0` global build contract | Package gains undeclared runtime libraries | `file`, `ldd` negative check, and booted distro tests | unvalidated |
| A-7 | Existing zefs hash-as-token login makes discarded automatic plaintext usable for local administration | `internal/core/ssh/client/client.go` | Fresh package install would be inaccessible | VM install then root `ze status` and `ze cli` test | confirmed |
| A-8 | Vendor-unit, sysusers, tmpfiles, and preset paths work on the declared Debian/RPM families | systemd/FHS conventions | Package scripts fail on a supported distribution | Container matrix and booted QEMU VM tests | unvalidated |
| A-9 | Exact release-evidence check runs can be bound to a source SHA through GitHub checks/artifacts | GitHub Actions model | Publisher cannot machine-verify stable readiness | Evidence workflow prototype and negative SHA/run-attempt mismatch tests | unvalidated |
| A-10 | Separate mirror, publisher, and nightly-retention identities plus protected refs can contain forge authority despite coarse Contents write permission | GitHub App/Codeberg protection model | Credentials would exceed the documented trust boundary | Preflight intended/denied API probes, protected-main push denial, stable-release/tag deletion denial-by-policy, audit-log, and token-rotation exercises | unvalidated |
| A-11 | Package-manager helpers can express the service policy without assuming systemd is PID 1 | Debian and RPM native lifecycle conventions | Offline/chroot install or policy-denied starts fail | Disposable chroot/container and booted VM lifecycle matrix | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Online signing subkey compromise | Unexpected signature, publisher-host alert, unrecognized release record | Offline primary revokes subkey; freeze activation; publish revocation/replacement; rebuild from archived attested inputs |
| R-2 | Canonical and mirror branches diverge | Mirror cannot fast-forward or commit/tag object IDs differ | Fail closed, alert, require human reconciliation; never force-push |
| R-3 | Candidate code exfiltrates credentials | Workflow review or unexpected network/process behavior | Build has no secrets/write scope; attestation is separate; publisher never executes candidate files |
| R-4 | Automatic package start changes host networking | VM sees non-loopback listeners or interface changes | Loopback-only generated config, no interface discovery, negative network-state assertions |
| R-5 | Package scripts destroy state or shadow administrator policy | `/etc/ze` digest changes, unit mask/override changes, unexpected start | Exact state/policy matrix, pre-unpack conflict check, idempotent retry, fail closed |
| R-6 | RPM `%post` failure is mistaken for transactional rollback | DNF reports scriptlet failure with payload installed | Document and test installed-but-inactive retry state; clean temporary state; `dnf reinstall ze` is the supported retry |
| R-7 | APT clients observe mixed metadata | Hash-sum mismatch or 404 after activation | Immutable by-hash objects first, `InRelease` last, low pointer TTL, public preflight fetch |
| R-8 | DNF clients observe mixed metadata or incompatible snapshots | repomd/package mismatch or wrong-architecture package chosen | One combined multi-arch snapshot, one mirrorlist pointer, native amd64/arm64 client tests |
| R-9 | GitHub publishes before one repository activates | Durable state shows `github-published` with one channel generation old | Keep every active format valid, do not auto-rollback, report degraded state, retry only the failed transition |
| R-10 | CDN serves stale mutable pointers | Public endpoint differs from origin generation/ETag | 60-second pointer TTL, revalidation, conditional writes, monitoring from two networks |
| R-11 | Reproducibility breaks in Go, gzip, tar, nFPM, or SBOM output | Double-build digest mismatch | Fixed epoch/canonical metadata, tool lock, deterministic writer, block publication |
| R-12 | Cleanup deletes referenced nightly objects | Install 404 after cleanup | Remove from metadata, activate replacement, wait maximum client/CDN TTL plus grace, then delete |
| R-13 | Current release evidence remains failing | Exact evidence check never succeeds | Keep stable publication disabled; finish dependency specs; never weaken evidence |
| R-14 | Forge Contents write permission is broader than tag/release operation intent | Permission audit shows an App can call an unrelated Contents verb | Three Apps/OS accounts, protected-main actor policy, checked-in API allowlists, run-scoped tokens, audit monitoring, immutable/private recovery records, quarterly rotation |
| R-15 | Packaging defect is found after immutable publication | Install or policy test fails after release | Freeze affected channel, diagnose, publish corrected source under a new date tag, retain original forensic evidence |
| R-16 | Container tests pass while booted systemd fails | QEMU lifecycle differs from container result | Require one Debian-family and one RPM-family booted VM for stable publication |
| R-17 | Direct DEB user assumes embedded package signature | Verification instructions or support report skips manifest verification | State the real trust path, require signed manifest/checksum plus immutable-release attestation, test tamper rejection |
| R-18 | Private archive is incomplete or unrestorable | Backup inventory mismatch or restore rehearsal fails | Archive-before-signing gate, unique immutable keys, provider-native retention lock, quarterly clean-room restore and attestation verification |
| R-19 | Future/stale CalVer blocks ordinary upgrades | Mirror observes date outside trusted window or high-water order | Trusted UTC/current-or-six-hour-boundary policy and future/stale tests before dispatch |
| R-20 | Native DNF cannot cryptographically detect a malicious replay of old signed metadata | Public generation differs selectively while signatures remain valid | Honest-cache bound, TLS, two-network monitoring, short metadata expiry, explicit limitation; do not claim client rollback protection |
| R-21 | Explicit nightly downgrade finds state incompatible with older stable | Target-binary read-only validation fails before restart | Keep old process when possible, preserve state, retain transaction, document exact nightly reinstall recovery |
| R-22 | Root broker parent R2 credential is compromised | Parent token appears outside credential broker or unexpected nightly write/list | Separate parent per nightly bucket, no-network mint boundary, systemd credential, five-minute exact delete sessions, audit every mint, rotate/freeze immediately |
| R-23 | Repository source and key bootstrap become unpaired | Source references missing key or orphan key remains after removal | Download/verify both to root-only temp, install key first and source last; remove source first and key/imports last; refresh/reinstall/removal tests |
| R-24 | VPS clock is stale or unsynchronized | chrony offset/age/leap/stratum threshold fails | Fail closed before date/sign/prune/activation, five-minute monitoring, frozen publication until chrony is healthy |
| R-25 | Crash straddles package service action | Transaction says `action-running` with ambiguous process state | Persist before call, reconcile `MainPID` executable digest/current policy, never default-start, inject every before/after crash point |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Exact mandatory test |
|-------------|----|--------------|----------------------|
| `ze init --automatic` | -> | init automatic bootstrap/state validator | `test/install/package-bootstrap.ci`; six named `TestRunAutomatic*` symbols |
| `ze doctor --json` and `ze explain` | -> | bounded systemctl query plus registered update diagnostic | `test/install/package-doctor-unit.ci`; `TestDoctorSystemctlShow`; `TestDoctorPackageManagedUpdate` |
| `show system update` and every firmware check/download/apply/restart/rollback command | -> | `ActiveBackend` package-managed dispatcher plus package-aware YANG help | guard `.ci`; `TestPackageManagedShowSystemUpdate`; `TestFirmwareHandlersPackageManaged`; `TestFirmwareCommandHelpPackageManaged`; both booted VM full profiles |
| Downloaded `install-ze-repository.sh install|remove` | -> | fingerprint/checksum verification and atomic external source/key pair lifecycle | `make ze-release-repository-bootstrap-test`; all six package full profiles |
| `make ze-release-build CHANNEL=stable REF=<sha>` | -> | deterministic builder/input manifest/packages | `make ze-release-repro-test CHANNEL=stable REF=<sha>` |
| Signed Codeberg stable tag | -> | mirror journal, ambiguous-dispatch reconciliation, trusted build/attest/archive/publish | `MirrorPolicyTest.test_ambiguous_dispatch_reconciliation`; staging `stable` and `stable-dispatch-ambiguity` |
| Protected daily schedule, changed source | -> | nightly build/tag/repositories | `make ze-release-staging-test SCENARIO=nightly-changed` |
| Protected daily schedule, unchanged source | -> | exact skip path | `make ze-release-staging-test SCENARIO=nightly-unchanged` |
| Failed scheduled nightly native re-run or three overlapping dates | -> | independent queued runs, original date/SHA/attempt lineage, durable one-per-date selection | `make ze-release-staging-test SCENARIO=nightly-concurrency-rerun` |
| Protected `workflow_call` evidence | -> | tokenless candidate runner, canonical artifact names, isolated recorder result/check | `WorkflowPolicyTest.test_evidence_reusable_caller_and_token_isolation`; `EvidenceRecorderPolicyTest.test_check_result_routing`; `make ze-release-policy-test` |
| GitHub `workflow_run` | -> | exact canonical current-attempt triplet, API IDs, protected-workflow/candidate identities, custom provenance/SPDX | `ArtifactTransportPolicyTest.test_current_attempt_name_id_matrix`; staging `attestation-main-advance`; three `AttestationPolicyTest` symbols |
| Stable APT install | -> | Signed-By/InRelease/by-hash/DEB/bootstrap/service | `effective-package-install.py --family deb --distro debian-12 --arch amd64 --profile full` |
| Stable DNF install | -> | mirrorlist/combined repodata/signed RPM/bootstrap/service | `effective-package-install.py --family rpm --distro rocky-9 --arch amd64 --profile full` |
| Nightly opt-in and immediate stable return | -> | native EVR plus explicit downgrade | all six package commands; `ReleaseModelTest.test_native_version_order_and_downgrade` |
| Booted Debian VM | -> | full service/lifecycle/failure/update-guard path | `effective-package-vm.py --family deb --distro debian-12 --arch amd64 --profile full` |
| Booted RPM VM | -> | full service/lifecycle/failure/update-guard path | `effective-package-vm.py --family rpm --distro rocky-9 --arch amd64 --profile full` |
| Direct GitHub verification | -> | outer set, signatures, release-specific verification, final-attestation archive | `make ze-release-public-test RELEASE=<id> NETWORK=primary`; independent monitor variant; `PublisherStateTest.test_final_attestation_archive_barrier` |
| Publisher retry/activation/replay | -> | durable strict APT-then-RPM state | `make ze-release-staging-test SCENARIO=failures`; `SCENARIO=freshness-replay`; negative RPM-before-APT unit |
| Nightly retention timer/broker | -> | activation-epoch-plus-lock plan, fresh clock, exact delete sessions, nightly-only deletion | both `RetentionPolicyTest` symbols; both `CredentialPolicyTest` symbols; staging `retention` and `storage-isolation` |
| Trusted VPS clock | -> | chrony-gated tag/sign/prune/mint/child/activation | `MonitorPolicyTest.test_trusted_clock_freezes_authorization`; `CredentialPolicyTest.test_stale_clock_before_each_mint`; staging `monitoring` |
| Monitor and restore | -> | thresholds/alerts/freeze plus input/final/final-attestation/refresh recovery | staging `monitoring`, `backup-restore`, and `archive-failures` |
| `make ze-release-repository-tamper-test` | -> | clean-keyring APT/RPM/package/metadata/key/fingerprint rejection | `RepositoryTamperTest.test_clean_keyring_matrix`; mandatory `repository-tamper` evidence |
| Stable dependency and first-canary gate | -> | exact candidate closure plus six public GitHub/APT/RPM network legs | `DependencyClosurePolicyTest.test_exact_candidate_closure`; `ActivationPolicyTest.test_dependency_canary_completion_gate`; public `canary-failures` scenario |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Stable tag `YY.MM.DD` | Strict calendar parser plus trusted UTC policy accepts current date or previous date only through `06:00:00Z`, requires identity above stable high-water, annotated allowlisted signature, immutable tag, and protected-main reachability; future/stale/duplicate/moved/invalid tags reject before dispatch |
| AC-2 | Canonical and mirror `main` or stable tag objects differ | Mirror fast-forwards GitHub only when possible; divergence/object mismatch fails closed with alert and no force update |
| AC-3 | Authorized stable tag passes mirror verification | Only mirror App dispatches protected default-branch workflow with exact tag/commit IDs; its durable request-ID journal reconciles lost responses and selects one run ID from delayed duplicates without advancing high-water early; tag-push and publisher/retention source-ref attempts cannot enter or alter release workflow |
| AC-4 | Daily nightly schedule with unchanged source | No tag/release/snapshot/pointer is created; exact date/source digest-bound skip is retained and retry cannot create a second identity |
| AC-5 | Daily nightly schedule with changed source | Exactly one immutable nightly per actual UTC run date is signed identically on both forges, `prerelease=true`/`make_latest=false` on GitHub, nightly-only in repositories, and cannot replace `/releases/latest`; three overlapping scheduled dates remain queued and each records its exact result |
| AC-6 | Release build for amd64 and arm64 | Each output is full default static distro binary with exact release, commit, source-timestamp build date, OS, architecture, complete feature inventory, no dirty marker |
| AC-7 | Same inputs/locked tools built twice | Every unsigned binary/archive/package input/manifest/checksum/completion/SPDX output is byte-identical |
| AC-8 | Packaging input bundle | Manifest closes files and binds candidate, protected workflow/run/attempt, protected-policy-derived logical name, tools, modes/sizes/digests while excluding future API IDs; current attempt has the exact canonical input/evidence/recorder triplet and API ID associations; missing/extra/unsafe/final-byte/self-reference/swapped-name/duplicate/cross-run/cross-attempt claims reject |
| AC-9 | Tarball, DEB, signed RPM per architecture | Extracted binary equals that architecture's attested bytes; signing preserves RPM payload; cross-architecture substitution rejects |
| AC-10 | Package inspection | Exact payload paths/types/owners/modes/dependencies/scriptlets/external-trust exclusion/preset/completions/license/marker/transaction declaration match; no repository config/key, daemon config/secret/helper/undeclared payload |
| AC-11 | Fresh automatic init | Admin plus exact 32-byte CSPRNG secret, no plaintext persistence, validated loopback-only active config, restrictive atomic state, all injected failures cleaned |
| AC-12 | Existing or unsafe bootstrap state | Valid real-directory/regular-file owner/mode/zefs/config returns success byte-identically; conflicts, short entropy, corrupt/unreadable/symlink/nonregular/owner-mode-invalid state and every injected error fail without mutation/chown/start |
| AC-13 | Fresh ordinary install and identity adoption | Creates or preserves compatible locked `ze:ze`, rejects every incompatible existing identity before ownership changes, bootstraps, and when native preset permits enables/starts with local CLI and no non-loopback listener |
| AC-14 | Policy/mask/offline install | Exact `/dev/null` mask is preserved while regular/unsafe fresh admin unit rejects pre-unpack; policy-rc.d, RPM administrator preset denial, chroot, and non-PID1 never bypass policy, unmask, enable/start contrary to policy, and yield the exact table result |
| AC-15 | Upgrade/downgrade with existing state/unit policy | Preserves zefs/config/credentials/admin unit/drop-ins byte-identically, never bootstraps, preserves enablement independently, and reconciliation yields at most one successful target-binary final transition according to every ActiveState/UnitFileState row |
| AC-16 | Native transaction/failure/retry matrix | Safe transaction-container creation, every forward and install/upgrade abort stage, crash point, DEB argument/unwind/package-state predicate, RPM hook and reinstall bridge, pre/post-unpack and service-call failure produce exact disk/process/package-db state; retries preserve the original snapshot/EVRs/intent, reconcile target/old process digest, keep service action last, and never default-start |
| AC-17 | Remove/purge/erase | Remove/erase preserve `/etc/ze` and the external source/key pair, permit native refresh/reinstall, and delete package transaction state; DEB purge removes `/etc/ze` but preserves external repository pair and identity; RPM documents separate cleanup |
| AC-18 | Admin unit and `/usr/local` | Fresh regular/unsafe admin unit rejects with no change, native mask succeeds unchanged, upgrade preserves admin fragments/drop-ins, doctor reports effective values, `/usr/local/bin/ze` is only warned about |
| AC-19 | Package-managed update backend | Marker selects registered backend before platform; `show system update` plus every check/status/unsupported handler emits the exact apt/dnf host command literal, creates no stage/temp/prev, and cannot change `/usr/bin/ze`; YANG help and doctor/explain state package ownership and incompatible auto-apply |
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
| AC-34 | Operations | Every clock/cadence/timeout/threshold/dedup/recovery/freeze rule, mirror/App/key/activation/replay/CDN/APT expiry/DNF staleness/archive/credential/restore/retention scenario produces deterministic alerts and append-only evidence |
| AC-35 | Documentation | Users download and verify the repository installer before execution, bootstrap and separately remove source/key trust in fail-closed order, install/verify/switch/downgrade/upgrade/remove/purge/reinstall, understand service/update/replay limits and recover; maintainers have source-anchored mirror/sign/publish/freeze/rollback/rotation/restore runbooks |
| AC-36 | First public activation | A protected, exact-candidate dependency-closure record proves every dependency/finding/evidence item complete; infrastructure is live; staging promotion and every archive clean restore succeed; each primary and independent GitHub/APT/RPM canary failure freezes completion, documentation mutation, and subsequent publication until all six legs pass, after which README/status/spec closure may proceed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Downloads either architecture tarball and verifies it | Signed source -> deterministic binary -> custom provenance/SPDX -> payload/envelope -> immutable release | staging `stable`; both public probes |
| 2 | Adds stable APT and installs Ze | fingerprinted key -> deb822 freshness -> InRelease/by-hash -> DEB -> bootstrap -> systemd -> local CLI | three exact DEB commands; Debian VM |
| 3 | Adds stable DNF and installs Ze | fingerprinted key/repo -> one-line mirrorlist -> combined repodata/signed RPM -> systemd | three exact RPM commands; Rocky VM |
| 4 | Installs in chroot/policy-denied/masked host | native helpers -> safe bootstrap -> precedence table -> no forbidden start/unmask | package-policy unit; both family full profiles |
| 5 | Opts into nightly then returns immediately to older stable | explicit repo disable/enable -> APT allow-downgrade or DNF distro-sync -> target validation -> preserved state | `test_native_version_order_and_downgrade`; both full profiles |
| 6 | Upgrades active/inactive/disabled/masked router | signed snapshot -> transaction -> read-only validation -> precedence -> process-digest reconciliation without duplicate successful restart | both booted VM full profiles |
| 7 | Recovers from package-script failure | durable transaction -> exact package DB/process state -> native configure/reinstall -> final action | package-policy unit; both VM failure stages |
| 8 | Removes/purges/erases and reinstalls Ze | final native removal -> package files/transaction removed -> external source/key pair preserved -> repository refresh/reinstall -> documented config/identity result | six full profiles and both VMs |
| 9 | Runs update commands on packaged Ze | marker -> `BackendPackageManaged` -> status/guidance, no download/stage/rename | guard `.ci`; handler/backend tests; both VMs |
| 10 | Verifies a direct DEB without APT | final attestation + signed payload/checksum envelope -> exact DEB digest | envelope unit; both public probes/tamper cases |
| 11 | Maintainer tags stable on Codeberg | trusted date/signature -> mirror dispatch -> tokenless evidence/attest -> input/final/final-attestation archives -> sign/publish/activate | staging `stable` |
| 12 | Maintainer waits for nightly | mirror -> 02:17 schedule -> changed/unchanged -> signed nightly/prerelease/repos | staging `nightly-changed` and `nightly-unchanged` |
| 13 | Operator recovers after one repository fails | durable per-format state -> valid old/new generations -> failed-format retry -> latest | staging `failures` |
| 14 | Operator rotates/revokes a key or restores publisher | domain key overlap/freeze plus locked input/final/final-attestation/refresh archives -> clean verification | staging `key-rotation` and `backup-restore` |
| 15 | Operator re-runs a failed scheduled nightly | GitHub native same-run retry -> original date/SHA -> incremented attempt -> concurrency/obsolete checks | staging `nightly-concurrency-rerun` |
| 16 | Retention removes expired nightlies | metadata-first activation -> signed exact plan -> brokered exact-object delete sessions -> forge cleanup | staging `retention` and `storage-isolation` |
| 17 | Operator removes Ze repository trust | verified production script removes source first -> proves deselection -> removes matching key bundle/imports -> package state untouched | repository-bootstrap target plus all package profiles |

## 🧪 TDD Test Plan

### Unit Tests

| Test symbol | File | Validates | Status |
|-------------|------|-----------|--------|
| `TestRunAutomaticFresh` | `internal/plugins/init/main_test.go` | Admin, exact entropy, required zefs keys, validated loopback config, atomic restrictive state | |
| `TestRunAutomaticExistingState` | `internal/plugins/init/main_test.go` | Real valid state is read-only validated and byte-identical | |
| `TestRunAutomaticRejectsUnsafeState` | `internal/plugins/init/main_test.go` | Corrupt/unreadable/symlink/nonregular/owner-mode-invalid directory/database rejects without mutation | |
| `TestRunAutomaticRejectsConflicts` | `internal/plugins/init/main_test.go` | Force/managed/web/input conflicts | |
| `TestRunAutomaticFailureCleanup` | `internal/plugins/init/main_test.go` | Short rand and entropy/bcrypt/config/write/close/rename failures clean temporary state | |
| `TestRunAutomaticNoPlaintext` | `internal/plugins/init/main_test.go` | Known plaintext absent from state/config/output/argv fixtures | |
| `TestPackagedUnitMatchesGeneratedUnit` | `internal/plugins/systemd/unit_test.go` | Vendor unit semantic parity for `/usr/bin/ze` and `/etc/ze` | |
| `TestDoctorSystemctlShow` | `internal/component/doctor/checks_platform_systemd_test.go` | Exact command; vendor/admin/drop-in/not-found/unavailable/nonzero/malformed/duplicate/oversize/timeout classifications | |
| `TestDoctorPackageManagedUpdate` | `internal/component/doctor/checks_platform_systemd_test.go` | `doctor-update-package-managed` plus `ze explain` registration | |
| `TestPackageManagedBackendSelection` | `internal/component/config/system/backend_package_managed_test.go` | Safe deb/rpm marker precedence; missing marker fallback; unsafe/unknown marker failure | |
| `TestPackageManagedBackendOperations` | `internal/component/config/system/backend_package_managed_test.go` | Check/status and download/apply/restart/rollback guidance with no files/mutation | |
| `TestPackageManagedDisablesAutoApply` | `internal/component/config/system/backend_package_managed_test.go` | Background SelfUpdater never starts despite auto config | |
| `TestFirmwareHandlersPackageManaged` | `internal/plugins/update-cmd/cmd/firmware_test.go` | Real check/download/apply/restart/rollback handler responses | |
| `TestPackageManagedShowSystemUpdate` | `internal/plugins/update-cmd/cmd/show_test.go` | Both marker values cross `handleShowSystemUpdate` with exact guidance fields/literals | |
| `TestFirmwareCommandHelpPackageManaged` | `internal/plugins/update-cmd/yang/self_containment_test.go` | Command help states apt/dnf backends do not contact, stage, restart, or roll back | |
| `ReleaseModelTest.test_stable_date_policy` | `scripts/release/test_model.py` | Calendar, trusted UTC boundary, strict high-water, one stable/date | |
| `ReleaseModelTest.test_nightly_identity` | `scripts/release/test_model.py` | Actual UTC date, resume identity, one/day, SHA, embedded release | |
| `ReleaseModelTest.test_native_version_order_and_downgrade` | `scripts/release/test_model.py` | dpkg/rpm EVR and explicit APT/DNF return commands | |
| `ArtifactModelTest.test_names_and_architectures` | `scripts/release/test_model.py` | Go/DEB/RPM mapping and canonical names | |
| `ReleaseBuildTest.test_smoke` | `scripts/release/test_build.py` | Full feature set, extended version, tar/DEB/RPM binary parity | |
| `ReleaseBuildTest.test_rejects_cross_architecture_substitution` | `scripts/release/test_build.py` | Wrong Go/DEB/RPM architecture and binary digest substitution reject | |
| `ReleaseBuildTest.test_all_unsigned_output_classes` | `scripts/release/test_build.py` | Policy inventory names and compares every unsigned output class required by AC-7 | |
| `InputManifestTest.test_closed_safe_set` | `scripts/release/test_manifest.py` | Missing/extra/unsafe path/type/mode and input schema | |
| `ReleaseEnvelopeTest.test_non_self_referential_closure` | `scripts/release/test_manifest.py` | Payload set, four envelope names, checksum exclusions, outer set | |
| `ReleaseEnvelopeTest.test_rejects_payload_envelope_tamper_matrix` | `scripts/release/test_manifest.py` | Every missing/extra/tampered payload, aggregate attestation, envelope, and checksum case | |
| `EvidencePolicyTest.test_exact_stable_and_nightly_sets` | `scripts/release/test_evidence.py` | Required/advisory categories and manifest schema | |
| `EvidencePolicyTest.test_rejects_missing_duplicate_skip_stale` | `scripts/release/test_evidence.py` | Every publisher evidence rejection class | |
| `WorkflowPolicyTest.test_evidence_reusable_caller_and_token_isolation` | `scripts/release/test_evidence.py` | workflow_call-only trusted revision, candidate SHA, API artifact ID, recorder split, no persisted token | |
| `EvidenceRecorderPolicyTest.test_check_result_routing` | `scripts/release/test_evidence.py` | Exact check name/App/external ID, recorder-result schema, evidence API ID routing, and all stale/wrong/duplicate negatives | |
| `DependencyClosurePolicyTest.test_exact_candidate_closure` | `scripts/release/test_evidence.py` | Exact policy set, candidate SHA, ancestor commits, zero findings, evidence digests, and missing/partial/stale/unknown negatives | |
| `MirrorPolicyTest.test_trusted_dispatch_and_ref_denials` | `scripts/release/test_mirror.py` | Fast-forward, tag/date/allowlist, rulesets, exact dispatch, denied refs | |
| `MirrorPolicyTest.test_ambiguous_dispatch_reconciliation` | `scripts/release/test_mirror.py` | Prepared/submitted/ambiguous journal, lost response, delayed/duplicate run selection, no premature high-water advance | |
| `AttestationPolicyTest.test_workflow_candidate_identity_races` | `scripts/release/test_attestation.py` | Stable tag ancestor, dispatch `head_sha`, later main advance, API artifact ID outside manifest | |
| `AttestationPolicyTest.test_subject_predicate_pairing` | `scripts/release/test_attestation.py` | Per-binary custom provenance/SPDX pairs and aggregate closed subject | |
| `AttestationPolicyTest.test_rejects_forged_statement_matrix` | `scripts/release/test_attestation.py` | Wrong/missing/duplicate subject, predicate, attester, run/attempt, ref/SHA, artifact, architecture | |
| `ArtifactTransportPolicyTest.test_current_attempt_name_id_matrix` | `scripts/release/test_attestation.py` | Complete protected current triplet, allowed partial historical rerun subsets, swapped/duplicate/cross-run/cross-attempt name-ID rejection | |
| `PublisherStateTest.test_idempotent_transitions` | `scripts/release/test_publish.py` | Durable resume, no repeated build/sign/upload, strict APT-then-RPM path, APT failure keeps both old, RPM failure keeps new APT/old RPM, RPM-before-APT rejection | |
| `PublisherStateTest.test_replay_and_rollback_authorization` | `scripts/release/test_publish.py` | Generation/predecessor/high-water/emergency action | |
| `PublisherStateTest.test_release_visibility_flags` | `scripts/release/test_publish.py` | Stable/nightly `prerelease` and `make_latest` request/API fields plus latest endpoint remains stable | |
| `PublisherStateTest.test_final_attestation_archive_barrier` | `scripts/release/test_publish.py` | Release-specific verifier, locked record, restore, tamper/failure block, and idempotent resume from published GitHub | |
| `PublicCanaryPolicyTest.test_all_leg_failures_freeze` | `scripts/release/test_publish.py` | Primary/independent GitHub/APT/RPM failures block completion/docs/subsequent release until all six pass | |
| `ActivationPolicyTest.test_dependency_canary_completion_gate` | `scripts/release/test_publish.py` | Exact dependency closure plus all public canary results are mandatory completion inputs | |
| `RepositoryPolicyTest.test_apt_activation_and_freshness` | `scripts/release/test_repository.py` | By-hash, InRelease-last, Date/Valid-Until/future skew/refresh | |
| `RepositoryPolicyTest.test_rpm_combined_snapshot` | `scripts/release/test_repository.py` | One repodata, both arches, one-line mirrorlist, 16-minute cache bound | |
| `RepositoryTamperTest.test_clean_keyring_matrix` | `scripts/release/test_repository.py` | APT Release/InRelease/Packages/DEB/date/key/fingerprint and RPM signature/repomd/XML/metadata/arch/key/fingerprint tamper matrix | |
| `StoragePolicyTest.test_bucket_authority_and_locks` | `scripts/release/test_storage.py` | Five buckets, three custom domains, prefix locks, denied stable/bootstrap/cross-bucket actions | |
| `StoragePolicyTest.test_stable_delete_and_private_route_denials` | `scripts/release/test_storage.py` | Stable/bootstrap lock deletion and all public access to private archives reject | |
| `CredentialPolicyTest.test_exact_delete_sessions` | `scripts/release/test_credentials.py` | Parent isolation, five-minute TTL, exact objects, delete-only actions, all read/list/write/cross-bucket denials | |
| `CredentialPolicyTest.test_stale_clock_before_each_mint` | `scripts/release/test_credentials.py` | Healthy signed plan followed by stale/malformed clock yields no session, child, forge, or object deletion | |
| `RetentionPolicyTest.test_metadata_first_and_stable_exclusion` | `scripts/release/test_retention.py` | Metadata first, activation epoch plus lock/grace, exact plan, stable exclusion | |
| `RetentionPolicyTest.test_activation_epoch_and_delayed_staging` | `scripts/release/test_retention.py` | Both object lock and `public_activated_at+37d`, delayed activation, exact threshold, unfinished publication exclusion | |
| `PackagePolicyTest.test_identity_unit_service_and_failure_matrix` | `scripts/release/test_package_policy.py` | Identity/mask/service-state/native failure transaction tables | |
| `PackageTransactionTest.test_all_stages_hooks_and_crash_reconciliation` | `scripts/release/test_package_policy.py` | Every forward stage, DEB argument/unwind branch, RPM hook, crash point, service-call reconciliation | |
| `PackageTransactionTest.test_debian_abort_state_machine` | `scripts/release/test_package_policy.py` | Every closed install/upgrade abort stage, dpkg-state predicate, container edge, and before/after crash | |
| `PackageTransactionTest.test_rpm_reinstall_bridge` | `scripts/release/test_package_policy.py` | Fresh and upgrade `%post` failures preserve original journal through exact DNF reinstall and terminal-removal crash | |
| `PackagePolicyTest.test_payload_allowlist_negative_matrix` | `scripts/release/test_package_policy.py` | Missing/extra/type/owner/mode/dependency/scriptlet/trust/preset/marker/transaction negatives | |
| `RepositoryBootstrapPolicyTest.test_atomic_install_remove_failures` | `scripts/release/test_package_policy.py` | Production bootstrap script dependencies, key-first/source-last, source-first/key-last, and every failure point | |
| `PackagePolicyTest.test_upgrade_policy_digest_matrix` | `scripts/release/test_package_policy.py` | Unit-level AC-15 state/unit/credential/drop-in digests and at-most-one action invariant | |
| `PackagePolicyTest.test_removal_repository_pair_matrix` | `scripts/release/test_package_policy.py` | Unit-level AC-17 remove/purge/erase state and external pair preservation | |
| `PackageMatrixPolicyTest.test_required_cells_and_cases` | `scripts/release/test_package_policy.py` | Unit-level AC-21 exact six container/two VM cells and required case inventory | |
| `DocumentationPolicyTest.test_user_and_operator_contract` | `scripts/release/test_docs.py` | Unit-level AC-35 required commands, trust ordering, limits, and recovery sections | |
| `KeyPolicyTest.test_domain_rotation_revocation` | `scripts/release/test_keys.py` | Separate primaries, overlap, expiry, freeze, revocation, clean keyrings | |
| `MonitorPolicyTest.test_thresholds_dedup_freeze_recovery` | `scripts/release/test_monitor.py` | Every cadence/threshold/webhook/dedup/recovery/freeze transition | |
| `MonitorPolicyTest.test_trusted_clock_freezes_authorization` | `scripts/release/test_monitor.py` | chrony normal/offset/stale/leap/stratum/malformed/command-failure thresholds | |

All Python unit symbols run through `make ze-release-unit-test`, defined as `python3 -m unittest discover -s scripts/release -p 'test_*.py'`; Go symbols run through focused package targets in `mk/release.mk`. `make ze-release-policy-test` runs both plus actionlint, manifest/schema validation, GitHub permission policy, package shell lint, and generated-file drift checks.

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Stable calendar syntax | Gregorian 2000-2099 as `YY.MM.DD` | valid leap day | zero/impossible | unsupported mapping |
| Stable dispatch date | current UTC or prior date through `06:00:00Z`; above high-water | prior at exactly 06:00 | older/high-water-equal | any future date |
| Automatic entropy | exactly 32 bytes before encoding | 32 | short read/31 | N/A |
| Doctor systemctl deadline/output | 2 seconds, 64 KiB | completion just before 2s; 65536 bytes | N/A | 2s blocked; 65537 bytes |
| Trusted VPS clock | chrony Normal, valid stratum, update age 0-600s, absolute offset 0-1s | 1s offset and 600s age | N/A | first instant beyond either bound or non-Normal |
| Nightly SHA | exactly 12 lowercase hex | `ffffffffffff` | 11/non-hex | 13 |
| Release asset size | 1 byte to less than 2 GiB | 2 GiB minus 1 | 0 | 2 GiB |
| Manifest path/outer set | normalized allowlist plus exactly four envelope names | longest configured path/exact set | missing | traversal/extra |
| Evidence categories | stable 21 mandatory + 1 advisory; nightly 10 mandatory | exact unique set | missing/skip | duplicate/unknown |
| Release-build artifact inventory | canonical input/evidence/recorder names per run/attempt | exact unique current triplet plus any unique canonical historical subset | current name missing | duplicate/swapped/unknown/cross-run/cross-attempt name or ID |
| Nightly retention | both object lock expiry and 37 full days from `public_activated_at` | instant before later threshold | either threshold unmet, including delayed activation | eligible retained past policy deadline |
| R2 delete session | TTL fixed 300s; 1-100 exact object paths per batch | 300s/100 objects | 0s/0 objects | 301s/101 objects |
| Storage topology | exactly five buckets and three public custom domains | exact set | one missing | unknown bucket/domain |
| Pointer cache | 1-60 seconds | 60 | 0 | 61 |
| DNF honest-cache window | metadata 15m + pointer 60s | 16 minutes | N/A | 16m plus one instant |
| APT validity/future | 48h validity; 600s future skew | expiry/future threshold minus one instant | N/A | threshold instant |
| Signing-key thresholds | warn 60d, critical 30d, freeze 14d | exactly each pre-threshold instant | N/A | crossing each threshold |
| Architecture set | exactly amd64 and arm64 | both | one missing | third |
| RPM scriptlet count | decimal `0..max-int` with hook-specific allowed ranges | max-int | missing/nonnumeric/negative | overflow |
| Successful nightlies/date | 0 or 1 | 1 | N/A | 2 |
| Activation generation | uint64 strictly current+1 with exact predecessor | current+1 | 0/current | skip/replay/overflow |
| Monitor consecutive failures | warning first, critical third | second | N/A | third |
| Restore cadence | success within 90d, warning at 75d | instant before 90d | N/A | 90d |

### Functional Tests

| Exact command/test | Location | End-user/operator scenario | Status |
|--------------------|----------|----------------------------|--------|
| `make ze-release-install-functional-test` (`bin/ze-test install --all`) | `test/install/package-bootstrap.ci` | Safe automatic bootstrap, existing/unsafe state, no plaintext | |
| same exact install-suite command | `test/install/package-doctor-unit.ci` | Effective unit/drop-in/query diagnostics and `ze explain` | |
| same exact install-suite command | `test/install/package-self-update-guard.ci` | Real update handlers cannot stage/mutate packaged binary | |
| `make ze-ui-test` (`bin/ze-test ui --all`) | `test/ui/init-automatic-help.ci` | Automatic flag and package purpose visible | |
| `make ze-release-repository-bootstrap-test` | `packaging/repository/install-ze-repository.sh`; `scripts/release/test_package_policy.py` | Same production install/remove script, local signed fixtures, tool/conflict/fingerprint/failure matrix | |
| `effective-package-install.py --family deb --distro debian-12 --arch amd64 --profile full` | `scripts/evidence/` | Full DEB container/native-manager lifecycle | |
| `effective-package-install.py --family deb --distro ubuntu-24.04 --arch amd64 --profile full` | same | Ubuntu DEB policy/lifecycle | |
| `effective-package-install.py --family deb --distro debian-12 --arch arm64 --profile full` | same | Native arm64 DEB payload/repository/lifecycle | |
| `effective-package-install.py --family rpm --distro rocky-9 --arch amd64 --profile full` | same | Rocky RPM lifecycle/failure/reinstall | |
| `effective-package-install.py --family rpm --distro fedora-current --arch amd64 --profile full` | same | Pinned Fedora current RPM lifecycle | |
| `effective-package-install.py --family rpm --distro rocky-9 --arch arm64 --profile full` | same | Native arm64 RPM combined repository | |
| `effective-package-vm.py --family deb --distro debian-12 --arch amd64 --profile full` | `scripts/evidence/` | Booted install, active/inactive/disabled/masked upgrade, failures, remove/reinstall/purge | |
| `effective-package-vm.py --family rpm --distro rocky-9 --arch amd64 --profile full` | same | Booted install, state matrix, failures, erase/reinstall | |
| `effective-release-repro.py --channel stable --architectures amd64,arm64` | `scripts/evidence/` | Two isolated builds and exact binary/package/archive parity | |
| `effective-release-staging.py --scenario stable` | `scripts/evidence/` | Stable journal-selected dispatch through dependency closure, all archives, immutable release, strict repository activation | |
| `effective-release-staging.py --scenario stable-dispatch-ambiguity` | same | Before-send, accepted-response-lost, rejected, delayed, duplicate-run journal reconciliation | |
| `effective-release-staging.py --scenario dependency-closure` | same | Exact candidate closure success and missing/partial/stale/wrong-SHA/skipped/unknown rejection | |
| `effective-release-staging.py --scenario stable-policy-negatives` | same | Future/stale/duplicate/moved/invalid stable tags all reject before dispatch | |
| `effective-release-staging.py --scenario mirror-divergence` | same | Branch/tag object divergence and non-fast-forward mirror reject without force | |
| `effective-release-staging.py --scenario nightly-changed` | same | One immutable nightly with exact prerelease/latest API state and prior stable latest preserved | |
| `effective-release-staging.py --scenario nightly-unchanged` | same | Exact skip and no mutation | |
| `effective-release-staging.py --scenario nightly-concurrency-rerun` | same | Native rerun preserves lineage; at least three overlapping dates remain queued, select one run/date, and publish FIFO despite out-of-order completion | |
| `effective-release-staging.py --scenario attestation-main-advance` | same | workflow_run candidate binding race | |
| `effective-release-staging.py --scenario manifest-architecture-negatives` | same | Forged closed manifest/artifact identity and cross-architecture substitutions reject | |
| `effective-release-staging.py --scenario failures` | same | Every durable/final-action/activation transition and retry | |
| `effective-release-staging.py --scenario freshness-replay` | same | APT expiry/future rejection, DNF cache bound, publisher replay rejection | |
| `make ze-release-repository-tamper-test` (`python3 -m unittest scripts.release.test_repository.RepositoryTamperTest.test_clean_keyring_matrix`) | `scripts/release/` | Exact clean-keyring APT/RPM metadata/package/date/architecture/key/fingerprint mutations | |
| `effective-release-staging.py --scenario key-rotation` | same | Every trust-domain overlap/revocation/freeze | |
| `effective-release-staging.py --scenario storage-isolation` | same | Bucket locks and denied stable/bootstrap/cross-bucket operations | |
| `effective-release-staging.py --scenario retention` | same | Both lock and activation-epoch 30+7-day pruning, delayed staging, unfinished exclusion, stale-broker denial, stable denial | |
| `effective-release-staging.py --scenario backup-restore` | same | Input, final, final-attestation, and refresh clean restore | |
| `effective-release-staging.py --scenario archive-failures` | same | Every archive class missing/tampered/unlocked/unrestorable case blocks its exact transition and resumes idempotently | |
| `effective-release-staging.py --scenario monitoring` | same | Thresholds, webhook, dedup, freeze, recovery, including healthy-plan-to-stale-broker denial | |
| `effective-release-public.py --release <id> --network primary` | `scripts/evidence/` | `gh release verify`/`verify-asset`, stable/nightly visibility/latest fields, public APT/RPM install | |
| `effective-release-public.py --release <id> --network independent` | GitHub `release-monitor.yml` | Independent release-specific verification plus generation/signature/install probe | |
| `effective-release-public.py --release <id> --scenario canary-failures` | `scripts/evidence/` | Six isolated primary/independent GitHub/APT/RPM failures each freeze completion/docs/next publication, then all-leg recovery | |

### AC-to-Test Matrix

| AC | Exact mandatory evidence |
|----|--------------------------|
| AC-1 | `ReleaseModelTest.test_stable_date_policy`; `effective-release-staging.py --scenario stable-policy-negatives` |
| AC-2 | `MirrorPolicyTest.test_trusted_dispatch_and_ref_denials`; `effective-release-staging.py --scenario mirror-divergence` |
| AC-3 | Both `MirrorPolicyTest` symbols; staging `stable-dispatch-ambiguity`; raw publisher/retention ref-denial cases |
| AC-4 | `ReleaseModelTest.test_nightly_identity`; staging `nightly-unchanged` |
| AC-5 | `ReleaseModelTest.test_nightly_identity`; `PublisherStateTest.test_release_visibility_flags`; staging `nightly-changed` and `nightly-concurrency-rerun`; both successful public probes |
| AC-6 | `ReleaseBuildTest.test_smoke`; exact reproducibility command |
| AC-7 | `ReleaseBuildTest.test_all_unsigned_output_classes`; exact reproducibility command; `release-repro` evidence category |
| AC-8 | `InputManifestTest.test_closed_safe_set`; `ArtifactTransportPolicyTest.test_current_attempt_name_id_matrix`; staging `manifest-architecture-negatives` |
| AC-9 | both `ReleaseBuildTest` smoke/cross-architecture symbols; staging `manifest-architecture-negatives` |
| AC-10 | six package full profiles; `PackagePolicyTest.test_payload_allowlist_negative_matrix` |
| AC-11 | `TestRunAutomaticFresh`; `TestRunAutomaticFailureCleanup`; `TestRunAutomaticNoPlaintext`; install functional test |
| AC-12 | existing/unsafe/conflict automatic-init units; six package full-profile unsafe-state cases |
| AC-13 | `PackagePolicyTest.test_identity_unit_service_and_failure_matrix`; both exact VM full profiles |
| AC-14 | same package policy unit; six mask/policy/chroot/RPM-preset profiles |
| AC-15 | `PackagePolicyTest.test_upgrade_policy_digest_matrix`; both VM full profiles' active/inactive/disabled/masked digest assertions |
| AC-16 | all three `PackageTransactionTest` symbols; staging `failures`; both VM full profiles |
| AC-17 | `PackagePolicyTest.test_removal_repository_pair_matrix`; six package plus both VM full profiles' remove/purge/erase/reinstall cases |
| AC-18 | `TestDoctorSystemctlShow`; install functional test; both VM conflict/mask/override/`/usr/local` cases |
| AC-19 | three package-backend tests; `TestPackageManagedShowSystemUpdate`; firmware/help tests; guard `.ci`; both VMs |
| AC-20 | `ReleaseModelTest.test_native_version_order_and_downgrade`; both VM same-day upgrade and older-stable recovery |
| AC-21 | `PackageMatrixPolicyTest.test_required_cells_and_cases`; six exact container plus two exact VM full commands |
| AC-22 | workflow/evidence policy target; `EvidenceRecorderPolicyTest.test_check_result_routing`; artifact transport and attestation identity tests; staging `attestation-main-advance` |
| AC-23 | attestation pairing and forged-statement symbols |
| AC-24 | both evidence policy symbols; `PublisherStateTest.test_final_attestation_archive_barrier`; staging `backup-restore` and `archive-failures` |
| AC-25 | key rotation unit; `RepositoryTamperTest.test_clean_keyring_matrix`; staging `key-rotation` |
| AC-26 | envelope closure; `PublisherStateTest.test_release_visibility_flags`; release-specific primary/independent public probes |
| AC-27 | both envelope symbols; both release-specific public probes |
| AC-28 | APT policy unit; repository tamper target; staging `freshness-replay`; three DEB full profiles |
| AC-29 | RPM policy unit; repository tamper target; staging `freshness-replay`; three RPM full profiles |
| AC-30 | both original publisher-state symbols plus strict-order assertions; staging `failures` and `freshness-replay` |
| AC-31 | both storage units; exact delete-session unit; staging `storage-isolation` and `backup-restore` |
| AC-32 | both retention units; both credential units; staging `retention` |
| AC-33 | stable-delete/private-route unit; both retention units; staging `storage-isolation` |
| AC-34 | both monitor units plus stale-clock credential unit; staging `monitoring` |
| AC-35 | `DocumentationPolicyTest.test_user_and_operator_contract`; `make ze-doc-test`; both successful public probes executing guide commands |
| AC-36 | `DependencyClosurePolicyTest.test_exact_candidate_closure`; `ActivationPolicyTest.test_dependency_canary_completion_gate`; public canary-failures scenario; all evidence/staging/successful public modes; signed first-public-canary record |

### Interop Tests

Not protocol work. Native package-manager, GitHub attestation, OpenPGP, S3/R2, CDN, and systemd lifecycle interoperability replace network-protocol interop for this spec.

### Future

None. Every named release, package, repository, trust, bootstrap, recovery, monitoring, and documentation test above is required for completion.


## Files to Modify

- `Makefile` - include release targets and accept explicit deterministic metadata without changing local defaults.
- `mk/test-release.mk` - versioned exact stable/nightly required sets, new release/package categories, canonical evidence manifest, no mandatory skip.
- `internal/plugins/init/main.go`, `register.go`, and `main_test.go` - automatic bootstrap, safe existing-state validation, injected entropy/error seams, CLI help.
- `internal/plugins/systemd/unit.go` and `unit_test.go` - canonical service semantics and vendor-unit parity.
- `internal/component/doctor/checks_platform.go` - exact bounded `systemctl show` contract and package-managed auto-apply diagnostic.
- `internal/component/config/system/backend.go` and existing backend tests - `BackendPackageManaged`, marker option/precedence, safe marker validation.
- `cmd/ze/hub/main_system.go` - pass injectable/default install-method marker path into backend selection.
- `internal/plugins/update-cmd/cmd/firmware.go`, `cmd/show_test.go`, `yang/ze-update-firmware-cmd.yang`, and `yang/self_containment_test.go` - preserve structured package-managed responses, verify `show system update` dispatch for both markers, and make command help explain that package backends do not contact/stage/restart/rollback.
- `internal/core/diagnostic/codes.go` - unconditionally register `doctor-service-query` and `doctor-update-package-managed` with `ze explain` examples.
- `README.md` - package/download quickstart and pre-release notice removal only after public canary.
- `docs/features.md`, `docs/guide/README.md`, `docs/guide/quickstart.md` - package-first availability/navigation.
- `docs/guide/ubuntu-build-install.md`, `docs/guide/ze-install.md` - source versus package install, exact identity/state/service/failure/remove policy.
- `docs/guide/self-update.md` - package-managed backend/commands and no project feed claim.
- `docs/guide/operations.md`, `docs/guide/command-reference.md` - unit query, init, update ownership, alerts/recovery.
- `docs/functional-tests.md` - exact category/matrix/staging/public targets.
- `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`, `ai/PACKAGE-MAP.md`, generated inventories/indexes through owning generators.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Automatic bootstrap writes existing config; package marker is installation state, not operator config |
| CLI flag | Yes | Init command owner, registration/help, `.ci` test |
| CLI/update behavior | Yes | Existing update command entry points select package-managed backend and return precise guidance |
| CLI grammar | Yes | Existing `ze init [options]` and update commands; no identifier grammar change |
| Functional test | Yes | Bootstrap, doctor, update guard, package/repository, staging, and public `.ci`/evidence paths |
| Doctor behavior | Yes | Effective systemd unit plus package-managed auto-apply diagnostic |
| Diagnostic registry | Yes | New registered `doctor-service-query` and `doctor-update-package-managed`; unit/`.ci`/`ze explain` coverage |
| New daemon runtime dependency | No | Packages require systemd/CA certificates; Go daemon adds no external library/process dependency |
| Prometheus metrics | No | Publisher metrics are external operational state, not daemon telemetry |
| Release evidence | Yes | `mk/test-release.mk` and release evidence workflow |
| Make help/discovery | Yes | Release targets, tool index, architecture/runbook links, script inventories |
| Generated code/docs | Yes | Run the owning generators after command, script, and documentation changes |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | Yes | README after canary, features, quickstart, package install guide |
| 2 | Config syntax changed? | No | Existing SSH/update config only; validate generated literal and package-policy diagnostic |
| 3 | CLI command added/changed? | Yes | Command reference, install guide, self-update guide |
| 4 | API/RPC added/changed? | No | No daemon API/RPC change |
| 5 | Plugin added/changed? | Yes | Existing init/systemd/config-system/doctor owners and generated inventories |
| 6 | Has a user guide page? | Yes | `docs/guide/package-install.md` linked from guide index |
| 7 | Wire format changed? | No | No network protocol change |
| 8 | Plugin SDK/protocol changed? | No | No external SDK/process protocol change |
| 9 | RFC behavior changed? | No | No protocol behavior |
| 10 | Test infrastructure changed? | Yes | Functional-tests guide and release architecture |
| 11 | Affects daemon comparison? | No | Distribution is not daemon protocol functionality |
| 12 | Internal architecture changed? | Yes | Release-distribution architecture and operations runbook |
| 13 | Route metadata changed? | No | No route data |
| 14 | Prometheus counters changed? | No | External publisher monitor uses its own operational contract |
| 15 | Runtime inventory changed? | Yes | Update generated command/diagnostic/package maps as required |
| 16 | Changed source referenced by docs? | Yes | Refresh source anchors for Makefile, init, systemd, doctor, config-system, and release scripts |
| 17 | Existing docs show examples? | Yes | Quickstart, Ubuntu install, installation, operations, self-update, command reference |
| 18 | Operator infrastructure changed? | Yes | Provisioning, key ceremony, App scopes, backup/restore, monitoring, failure recovery |

## Files to Create

- `.github/workflows/release-check.yml` - PR/manual release-policy/package validation with no publication.
- `.github/workflows/release-build.yml` - protected default-branch stable-dispatch/nightly build and bundle.
- `.github/workflows/release-attest.yml` - non-executing `workflow_run` custom upstream provenance and SPDX attestations.
- `.github/workflows/release-evidence.yml` - reusable exact-SHA candidate execution with no persisted token plus isolated protected recorder/check job.
- `.github/workflows/release-monitor.yml` - hourly read-only independent public probe.
- `mk/release.mk` - exact unit/repro/policy/repository-bootstrap/repository-tamper/staging/public targets and Make help.
- `internal/component/config/system/backend_package_managed.go`, `backend_package_managed_test.go` - registered marker-selected backend.
- `internal/component/doctor/checks_platform_systemd_test.go` - injected exact systemctl/diagnostic tests.
- `internal/plugins/update-cmd/cmd/firmware_test.go` - real handler package-backend tests.
- `packaging/nfpm.yaml` - exact shared contract with DEB/RPM overrides/dependencies/scripts.
- `packaging/systemd/ze.service`, `packaging/systemd/50-ze.preset` - vendor service/RPM preset.
- `packaging/sysusers/ze.conf`, `packaging/tmpfiles/ze.conf`, `packaging/tmpfiles/ze-package.conf` - identity, config state, root transaction state.
- `packaging/deb/preinstall.sh`, `postinstall.sh`, `preremove.sh`, `postremove.sh` - DEB classification/identity/transaction/lifecycle.
- `packaging/rpm/preinstall.sh`, `postinstall.sh`, `preremove.sh`, `postremove.sh` - RPM classification/identity/transaction/lifecycle.
- `packaging/install-method/deb`, `packaging/install-method/rpm` - backend marker payload.
- `packaging/repository/install-ze-repository.sh`, `ze.sources`, `ze.repo` - production verified install/remove entry point and external ordered source/key templates for stable and disabled nightly domains.
- `packaging/release-tools.lock.json`, `packaging/test-images.json` - pinned actions/tools/images, URLs, hashes/digests.
- `packaging/keys/ze-apt-archive.gpg`, `ze-apt-archive.asc`, `RPM-GPG-KEY-ze`, `ze-release-manifest.asc`, `ze-nightly-tag.asc`, `ze-retention.asc`, `ze-release-operations.asc` - public current/next trust-domain material only.
- `packaging/schemas/input-manifest.schema.json`, `release-manifest.schema.json`, `evidence-manifest.schema.json`, `evidence-recorder-result.schema.json`, `dependency-closure.schema.json`, `release-record.schema.json`, `release-attestation-record.schema.json`, `release-provenance-predicate.schema.json`, `package-transaction.schema.json`, `retention-plan.schema.json` - versioned closed contracts.
- `packaging/publisher/github-app-policy.json`, `storage-policy.json`, `monitor-policy.json`, `release-dependency-policy.json` - exact permissions/API allowlists, five buckets/prefix locks/temporary credentials, clock/alert policy, and pinned dependency/finding set.
- `packaging/publisher/ze-release-mirror.service`, `.timer` - five-minute mirror/dispatch.
- `packaging/publisher/ze-release-publisher.service`, `.timer` - two-minute archive/sign/publish/state driver.
- `packaging/publisher/ze-release-sign@.service` - no-network per-trust-domain signer account/credential.
- `packaging/publisher/ze-release-prune.service`, `.timer`, `ze-release-credential-broker@.service` - daily metadata-first orchestration plus root-only local exact-object delete-credential mint and sandboxed child.
- `packaging/publisher/ze-release-monitor.service`, `.timer` - five-minute clock/readiness/freshness/archive monitor.
- `packaging/publisher/ze-release.conf.example` - non-secret endpoints/paths plus credential names.
- `scripts/release/model.py`, `manifest.py`, `build.py`, `attestation.py`, `evidence.py`, `dependencies.py` - identity/build, protected artifact/check routing, dependency closure, and versioned input/evidence/attestation contracts.
- `scripts/release/repository.py`, `storage.py`, `credentials.py`, `mirror.py`, `publish.py`, `retention.py`, `monitor.py` - repositories/tamper validation, five-bucket/prefix-lock policy, brokered credentials, forge dispatch journal/publisher state, pruning/alerts.
- `scripts/release/preflight.py`, `install_publisher.py` - infrastructure/API/clock/dependency/canary denied probes and idempotent VPS deployment without private material.
- `scripts/release/test_model.py`, `test_build.py`, `test_manifest.py`, `test_evidence.py`, `test_mirror.py`, `test_attestation.py`, `test_repository.py`, `test_storage.py`, `test_credentials.py`, `test_publish.py`, `test_retention.py`, `test_package_policy.py`, `test_keys.py`, `test_monitor.py`, `test_docs.py` - exact Python unit suites.
- `scripts/evidence/effective-package-install.py`, `effective-package-vm.py` - six native-manager cells and two full booted-systemd lifecycles.
- `scripts/evidence/effective-release-repro.py`, `effective-release-staging.py`, `effective-release-public.py` - reproducibility, all staged scenarios, public dual-network canary.
- `test/install/package-bootstrap.ci`, `package-doctor-unit.ci`, `package-self-update-guard.ci`, `test/ui/init-automatic-help.ci` - end-user command wiring.
- `docs/guide/package-install.md` - trust/repo/install/nightly/downgrade/lifecycle/recovery guide.
- `docs/architecture/release-distribution.md` - producers, trust, evidence, buckets, state, freshness/replay boundary.
- `docs/contributing/releasing.md`, `docs/contributing/release-infrastructure.md` - tag/publish/freeze/rollback/rotation/monitor/restore/DNS/App/bucket runbooks.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file and dependencies |
| 2. Audit | Current Behavior, Files to Modify/Create, external tool lock |
| 3. Wiring phase | Wiring Test table, new Make targets, workflow dry-run entry points, failing bootstrap `.ci` |
| 4. Implement TDD | Phases below |
| 5. Full verification | Focused tests, package matrix, QEMU VM, `make ze-verify`, exact release evidence |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Every blocker/issue |
| 8. Re-verify | Repeat exact affected and full gates |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist and staging fault exercises |
| 12. Documentation review | Documentation Update Checklist and source anchors |
| 13. /ze-review gate | Review Gate, zero BLOCKER/ISSUE |
| 14. Present summary + close | First public canary, audit tables, learned summary, two-commit closure |

### Implementation Phases

Each phase uses test-first development and ends with a self-critical review.

1. **Wiring** - add Make/workflow/bootstrap/update-guard entry points and red wiring tests.
   - Tests: bootstrap, doctor, self-update guard, release model smoke, workflow policy/syntax.
   - Verify: every user/automation entry point is reachable and fails only for missing implementation.
2. **Release identity and deterministic build** - strict date/channel model, explicit source metadata, full feature set, closed input manifest, archives, completions, SPDX, and double-build proof.
   - Tests: model/manifest/architecture units, extended-version inventory, byte reproducibility.
   - Verify: both architectures report exact identity and every distribution view contains the same attested bytes.
3. **Package payload, bootstrap, and ownership** - automatic bootstrap/error injection, exact payload/dependencies, external ordered repository trust pair, vendor unit/preset, sysusers/tmpfiles, transaction state machine/maintscript dispatch, package-managed update backend and YANG help.
   - Tests: init/systemd/doctor/config-system units, every transaction/hook/crash branch, and `.ci` tests red first.
   - Verify: no secret/config/repository-trust payload, no in-place update mutation, idempotent bootstrap/retry reconciliation, effective-unit doctor.
4. **Package/repository interoperability** - ephemeral-key signed local APT and combined multi-arch RPM snapshots, service/preset policy matrix, native EVR, container and booted VM evidence.
   - Tests: install/upgrade/remove/purge/erase/reinstall, repository refresh, all Debian unwind arguments, RPM preset/scriptlet failures, chroot/mask/conflict/tamper/cross-arch negatives.
   - Verify: exact state/unit/binary/repository-pair digests and local CLI/network behavior match every AC.
5. **Unprivileged build/evidence and non-executing attestation** - protected default-branch dispatch/schedule, SHA-pinned workflows, no-persist candidate checkouts, isolated evidence recorder, permissions/concurrency, closed artifacts, per-binary and aggregate statements.
   - Tests: actionlint/security policy, unauthorized reusable caller/ref/SHA, token isolation, forged run/attempt/artifact/subject/predicate cases, staging workflow.
   - Verify: candidate build/evidence has no write token, recorder executes only protected code, and attestation executes no candidate code.
6. **Mirror, private archive, and isolated publisher** - out-of-band tag authorization, three forge automation identities, protected refs/API allowlists, attestation/evidence/tool policy, archive-before-sign, trust-domain GPG homes, draft/final verification, durable state.
   - Tests: unit transition matrix, intended/denied App operations, protected-main denial, staging forge/bucket, backup failure and clean restore.
   - Verify: candidate files never execute, credentials are scoped, retries never rebuild or resign.
7. **Independent atomic activation, retention, and monitoring** - APT/RPM commit points, monotonic predecessor state, emergency rollback, three-domain CDN policy, brokered exact-object nightly deletion, trusted clock, key overlap/revocation, monitors/alerts.
   - Tests: strict activation ordering, every fault point, replay/downgrade, cache staleness, activation-epoch-plus-lock 30+7-day boundary, temporary-credential denials, clock thresholds, expiry/rotation.
   - Verify: each format remains valid independently, deletion authority is exact and short-lived, and degraded/unsynchronized state is observable/frozen/retryable.
8. **Infrastructure pre-publication** - provision three DNS domains, five buckets/prefix locks, CDN, forge protections, mirror/publisher/retention Apps, Codeberg credentials, evidence runner/recorder, credential broker, chrony, VPS OS accounts, trust-domain primaries/subkeys/GPG homes, systemd credentials/timers, monitoring, backups.
   - Tests: preflight, least-privilege/anonymous-write and broker denials, custom-domain isolation, public HTTPS/signatures, mirror divergence, clock/key rotation/revocation, restore.
   - Verify: assumptions A-1 through A-11 are confirmed, or return this spec to design before publication.
9. **Documentation and first activation** - source-anchored user/operator docs, exact-candidate dependency closure, one immutable release, release-specific GitHub verification, and primary/independent APT/RPM installs before README status change.
   - Tests: documentation policy/doc tests, six isolated canary-failure freezes, all-six-leg recovery, command copy/paste canary, exact stable release evidence.
   - Verify: dependency record, all ACs/user stories, every archive restore, and both networks' GitHub/APT/RPM legs have fresh durable evidence.
10. **Full verification and closure** - touched tests, lint, docs, matrices, QEMU, release evidence, staging replay/recovery, security/review passes, and fully expanded audit tables.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-36 has implementation and concrete evidence |
| Artifact parity | Tarball, DEB, signed RPM, GitHub, and repository packages contain exact per-architecture attested bytes |
| Identity | Tag, embedded release, EVR, commit, source timestamp, channel, generation, manifest, and predecessor cannot disagree |
| Reproducibility | Two isolated builds cover every unsigned output and locked tool/workflow/image identity |
| Trust separation | Candidate build/evidence, recorder, attestation, mirror, archive, signing, broker, publication, and monitoring credentials are isolated |
| Attestation | Protected workflow revision, resolved candidate, API artifact ID, per-binary provenance/SPDX, and aggregate input provenance have exact subjects/predicates/run identity; final release attests final assets |
| Publisher behavior | No candidate execution, archive before signing, one signing pass, durable idempotent retries, conditional writes |
| Package payload | Exact allowlist, modes, ownership, dependencies, external key/source exclusion, preset/marker/transaction declarations, no state/secret/helper, architecture match |
| Package lifecycle | Every transaction stage, DEB argument/unwind, RPM hook/preset, fresh/upgrade/remove/purge/erase/failure/chroot/policy/mask/admin-override case matches tables |
| Bootstrap safety | 32-byte CSPRNG, loopback-only validated config, no interface discovery/plaintext, atomic restrictive state, all error cleanup |
| Package ownership | Self-update cannot mutate `/usr/bin/ze`; package-manager guidance/YANG and doctor auto-apply diagnostic work |
| Systemd ownership | Fresh conflicts reject before unpack; upgrades preserve admin policy; retry reconciles process digest; doctor lookup is bounded and effective |
| Repository atomicity | APT `InRelease` and one combined-RPM mirrorlist are final per-format commit points; generations are monotonic |
| Native trust | External fail-closed source/key bootstrap order, direct manifest/attestation, APT `Signed-By`, RPM/repodata signatures, clean-keyring and tamper tests work |
| Nightly isolation | One/day changed-source/native-rerun rule, prerelease, separate domain/repo, native EVR, 30-day retention, no stable contamination |
| Stable gate | Exact-SHA mandatory release evidence, release audit closure, archive restore, clock and infrastructure preflight are machine checked |
| Failure recovery | Every partial package/publisher state remains valid/observable; retry reconciles exact continuation; replay/downgrade/unsigned rollback reject |
| Operations | Clock, App/key rotation, broker credentials, expiry, divergence, stalled state, CDN freshness, archive/restore, retention are monitored |
| Scope | Only the normal distro binary and declared support files ship; no appliance/host/test artifacts leak |
| Documentation | Commands, URLs, fingerprints, policies, and failure states match live producers after canary |
| Rule: no-layering | nFPM assembles packages only; release identity/build/publish policy remains repository-owned |
| Rule: no-partial-completion | No release or completion claim while any AC, assumption, dependency, canary, or restore proof remains open |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Deterministic amd64/arm64 closed input bundle | Two isolated builds, inventory, manifest, architecture, and digest comparison |
| DEB and signed RPM exact packages | Native extract, path/type/mode/owner/dependency/signature/binary checks |
| Safe automatic package bootstrap | Unit error matrix, `.ci`, container, booted VM, plaintext/network negatives |
| Package-managed service and update ownership | Unit parity, bounded doctor, service policy matrix, immutable `/usr/bin/ze` test |
| Stable/nightly protected workflows | Trusted dispatch/schedule, unchanged skip, action policy, forged-context negatives |
| Per-binary SPDX/provenance plus aggregate provenance | Offline/GitHub verification for exact subject/predicate/run set |
| Exact Codeberg/GitHub mirror | Branch/tag object identity, unauthorized/divergence negatives |
| Private immutable recovery archive | Archive-before-sign failure gate and clean-room restore |
| Isolated VPS publisher | Sandbox, credential/App scope, candidate no-exec, durable-state/retry tests |
| Signed APT stable/nightly | Fresh installs, by-hash, direct-byte equality, tamper negatives |
| Signed combined RPM stable/nightly | Native amd64/arm64 installs, one-snapshot pointer, tamper/cross-arch negatives |
| Atomic independent activation | Fault injection, monotonic generation/predecessor, replay/downgrade/rollback authorization |
| GitHub direct download trust | Immutable draft flow, release-specific verification/asset verification, final-attestation archive, signed manifest/checksum tamper tests |
| DNS/CDN/public and private storage | HTTPS/cache/conditional/retention/anonymous-write and all-class restore checks |
| Retention, monitoring, and recovery | Delayed-activation 30+7-day simulation, stable exclusion, timers, stale-broker denial, alerts, key/App rotation, backup restore |
| User/operator documentation | Doc tests, source anchors, public command copy/paste canary |
| First immutable public release | Public GitHub verification plus fresh APT and RPM install evidence |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Stable authorization | Annotated allowlisted signature, protected-main reachability, exact object mirroring, trusted dispatch only |
| Nightly authorization | Protected schedule and native same-run re-run only; source-change/one-day/concurrency rule; publisher-created exact tag on both forges |
| Workflow permissions | Explicit minimum per job; candidate build/evidence checkout credentials do not persist and candidate commands have no write/OIDC token; recorder/attester are separate protected jobs |
| Action/tool pinning | Full action SHA; version plus upstream checksum/digest for tools, containers, VM images |
| Artifact substitution | Closed manifest, API transport ID outside deterministic input, exact architecture, separate input/final schemas, subject/predicate verification |
| Candidate execution | Recorder/attestation/publisher/monitor/broker never import, source, execute, or run bundle scripts |
| App/forge/storage credentials | Separate mirror-dispatch, publisher, retention identities and root broker, no PAT, protected refs, checked-in allowlists, run-scoped tokens, exact-object temporary sessions, denial/rotation tests |
| Signing ownership | Separate offline primaries for nightly tag, direct manifest, APT, RPM, retention authorization, and emergency operations; maintainer identity keys remain separate |
| Online signing keys | Primary secrets absent from VPS; purpose-specific expiring subkeys use distinct locked OS accounts/GPG homes/systemd credentials |
| Key lifecycle | Current/next public primaries only in each applicable consumer keyring, overlap, expiry alert, per-domain freeze/revocation/rebuild, out-of-band fingerprints |
| Object storage | Public/private prefix policies, conditional pointer writes, immutable unique keys with provider-native retention locks, anonymous write/list denied, restore proof |
| Package scripts | Fixed paths, quoted inputs, no network, exact dependencies, native helpers, fail closed, idempotent retry |
| Bootstrap secret | Full 256-bit entropy, memory-only plaintext, bcrypt/hash only, no argv/env/output/journal, all errors injected |
| Package ownership | Marker is package-owned; updater cannot rename over `/usr/bin/ze`; deleting marker is unsupported/tamper-detectable |
| Service exposure/policy | Loopback-only initial listener; no policy bypass, unmask, unexpected start, or admin override replacement |
| Filesystem safety | No traversal/symlink/device/set-id/world-writable/undeclared path; pre-unpack conflict check |
| Direct download trust | `gh release verify`/`verify-asset`, archived versioned record/raw transcripts, restored replay, and signed final manifest/checksum verify actual downloadable bytes |
| Repository tampering | Named clean-keyring matrix for APT Release/InRelease/Packages/DEB/date/key/fingerprint and RPM package/repomd/XML/metadata/architecture/key/fingerprint mutations |
| Activation replay | Signed generation/predecessor, high-water state, expired/invalid emergency rollback rejection |
| CDN poisoning | Immutable URLs, short pointer TTL, origin/public digest match, two-network monitoring, HTTPS |
| Replay/idempotency | Conditional release/snapshot/tag creation, transition journal, no duplicate signing or pointer regression |
| Log leakage | Tokens/credentials/plaintext redacted; records contain release IDs/digests and durable incident correlation only |
| Runner/VPS isolation | Evidence runner reset and secretless; publisher OS accounts/key homes/prefix credentials separated |
| Supply-chain recovery | Freeze, revoke/rotate, restore archived inputs, verify attestations/evidence, republish corrected new-date release |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Invalid/unauthorized/duplicate-date tag | Mirror rejects; maintainer creates a valid new date only after fixing source; never moves published identity |
| Mirror divergence or tag-object mismatch | Stop dispatch/publication, alert, human reconciliation; no force push |
| Reproducibility/manifest/architecture mismatch | Discard bundle; deterministic build/tool-lock phase |
| Package payload/bootstrap/service policy failure | Package lifecycle owner; affected format/source support remains blocked |
| RPM `%post` failure | Preserve installed-but-inactive state, cleanup temp, report exact DNF retry; never claim payload rollback |
| Attestation/evidence mismatch | Discard candidate; build/attestation or dependency gate |
| Private archive/restore failure | Stop before signing/publication; repair backup/retention and repeat clean restore |
| Signing/final manifest failure | Preserve verified unsigned input; no release publish or pointer change |
| GitHub draft failure | Resume exact draft uploads; repository pointers remain unchanged |
| GitHub immutable publish followed by repository failure | Keep direct release valid, retain per-format valid generations, report degraded state, retry failed transition |
| APT activation failure | Leave both previous APT and previous RPM generations active; retry APT only; RPM-before-APT remains forbidden |
| RPM activation failure | Leave previous RPM generation active; the same release's APT is already newly valid; retry RPM only |
| Replay/downgrade/invalid rollback action | Reject before signing/write, freeze if suspicious, alert with current/requested generations |
| Public CDN verification failure | Keep active pointer unchanged, retain immutable staged objects, alert/retry after cache diagnosis |
| Cleanup failure | Retain data, alert, resume metadata-first state; never accelerate deletion |
| Monitor/App/key expiry warning | Page operator before threshold; freeze new publication when policy deadline passes |
| Three failed implementation approaches | Stop, record attempts, ask user per planning rule |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| GitHub Pages for package repositories | Pages is documentation hosting with 1 GiB site and 100 GiB/month soft limits, and is not intended for software distribution | GitHub Releases for assets plus S3-compatible object storage/CDN for repositories |
| GitHub Packages for APT/RPM | GitHub Packages does not expose native APT or RPM repositories | Static signed repositories on `packages.ze-software.net` |
| GoReleaser as release orchestrator | Built-in nightly and Cloudsmith publishing features require Pro, and its SemVer-oriented release model duplicates Ze's CalVer/build rules | Checked-in Ze release scripts plus standalone nFPM |
| Always-running package application on VPS | APT/RPM repositories are static files and do not need an application server | Isolated timer-driven signer/publisher writing object storage |
| Building and signing in one GitHub job | Candidate code would execute with signing/publication authority | Unprivileged build, non-executing attestation, isolated VPS publisher |
| Automatic init through piped shell password | Plaintext can leak through argv/env/logs and requires fragile shell behavior | First-class `ze init --automatic` CSPRNG path |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Triple Challenge

| Question | Answer |
|----------|--------|
| Simplicity: is this the minimum system that meets the goal? | Yes. GitHub owns direct immutable downloads and attestations; object storage serves standard static repositories; nFPM only assembles packages; short-lived scripts/timers replace an always-running package service. The extra private archive, state journal, and monitor exist because signed publication is not recoverable or safely operable without them. |
| Uniformity: does this match Ze's existing producers and ownership? | Yes. `feature-gates.txt`/Make remain the build source, init owns zefs bootstrap, systemd owns unit semantics, config-system owns updater mutation, doctor owns readiness diagnostics, and release evidence extends the existing Make category model. No second runtime config or plugin communication mechanism is introduced. |
| Performance: does this harm Ze runtime constraints? | No. Release work is offline. Runtime changes are cold-path startup/command checks for a small package marker and a bounded doctor subprocess. No wire hot path, per-update allocation, or daemon polling loop changes. |

## Design Insights

- Repository distribution is static content; operational complexity belongs in short-lived, idempotent mirror/publisher/monitor processes, not an online package application.
- The most important supply-chain boundary is candidate-code execution versus credentials capable of attesting, signing, archiving, or publishing.
- Native package lifecycle is product behavior because it creates identity, state, listeners, service policy, and a privileged daemon. Archive generation alone is insufficient.
- Ze's eight-character runtime CalVer is a schema identity. Nightly channel and package EVR data remain outside it.
- A package manager owns both bytes and lifecycle. An in-place self-updater behind dpkg/RPM ownership corrupts the installed-state contract even if the replacement binary is authentic.
- Independent APT/RPM activation is safer than pretending a distributed multi-format transaction exists. Durable state and monotonic predecessor checks make partial success explicit and recoverable.

## Core Insight

One attested architecture binary is the immutable root of each architecture's tarball, DEB, and RPM. A closed attested input bundle, archived before signing, is the root of final signed assets, repository snapshots, and reproducible incident recovery.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| GitHub Releases plus S3-compatible repository hosting | GitHub Pages, GitHub Packages, Cloudsmith | Correct direct assets, native package-manager support, static repositories |
| Three one-bucket R2 custom domains over five buckets with prefix locks | One hostname needing a Worker router, seven buckets, VPS filesystem | Provider-native domain routing is implementable; public stable/nightly and private archives retain separate lifecycle while locks protect immutable prefixes |
| Strict stable `YY.MM.DD`, nightly date plus SHA, no correction suffix | SemVer, mutable nightly, `-rN` source-identical corrections | Matches runtime parser and immutable identity; packaging fixes use a new reviewed date release |
| nFPM only for native package assembly | GoReleaser end to end, hand-built ar/cpio | Mature formats without duplicating Ze identity/build/publication policy |
| Protected default-branch dispatch/schedule, tokenless candidate evidence, isolated recorder, and separate `workflow_run` attestation | Tag-push privileged workflow, candidate checks-write, build-job attestation | Candidate content cannot choose privileged workflow or access write tokens; protected jobs bind transport/candidate identity |
| Three forge automation identities plus root R2 credential broker | PAT, shared App, ordinary delete-capable bucket token, account-wide keys | Mirror, publish, and retention differ; local exact-object delete sessions contain destructive authority while parent keys remain isolated |
| Separate offline primary per signing trust domain, with purpose-specific online subkeys and pre-enrolled replacements | One primary with named subkeys, one online key, keyless-only | Separate primaries give APT, RPM, direct manifests, nightly tags, retention plans, and emergency operations enforceable trust boundaries |
| Automatic package bootstrap with loopback-only config | Disabled package, interactive postinst, shell-generated password | Requested usable install; Go CSPRNG path avoids plaintext leakage/public exposure |
| Package-managed update backend guard | Publish unsigned self-update manifest, allow in-place replacement | Keeps dpkg/RPM database, signatures, and rollback ownership truthful while retaining check/report |
| Vendor unit/preset plus semantic parity and bounded effective-unit doctor | Manual installer unit, hard-coded vendor path | Native ownership with administrator policy and no duplicate untested semantics |
| External ordered repository source/key pair preserved across package removal | Keyring only in daemon package, source left dangling, auto-delete on remove | Key-first/source-last activation and source-first/key-last removal keep APT/DNF usable; native reinstall remains functional |
| APT `InRelease` last and one combined-RPM mirrorlist last | In-place metadata overwrite, per-arch RPM pointers | One atomic commit object per format; DNF filters architecture from complete snapshot |
| Independent activation with durable monotonic predecessor state | Cross-format rollback, best-effort pointer writes | No false distributed transaction; every partial state is valid, observable, and retryable |
| Archive verified inputs before signing, exact final bytes before public staging, and release-specific verification before repositories | Trust transient GitHub artifacts, archive inputs only, archive final files only | Inputs permit verified reconstruction; final archive preserves non-reproducible signatures/repository bytes; final-attestation archive preserves GitHub's actual release verification evidence |
| One changed-source nightly per UTC day, 30-day retention | Rolling mutable latest, every-commit nightly | Auditable artifacts, bounded storage, stable per-build URLs |
| Exact stable evidence, smaller nightly gate | Full release evidence nightly, same gate | Stable claims require full proof; daily feedback remains practical without weakening stable |

## Known Limitations

- Initial distribution supports only the normal Linux daemon on `amd64` and `arm64`.
- Package installation requires systemd and does not claim OpenRC, runit, s6, containers without systemd, or non-Linux support.
- GitHub and Codeberg do not expose credentials scoped only to a tag prefix or to release create versus delete. Protected refs, separate Apps/accounts, checked-in API allowlists, run-scoped tokens, audit alerts, and private recovery reduce but do not eliminate a compromised forge-App credential's repository-level blast radius.
- RPM erase preserves `/etc/ze`; RPM has no automatic purge counterpart. Package-created user/group are preserved until an operator proves they are unused.
- Direct DEB verification uses the signed final manifest/checksum plus GitHub immutable-release attestation; no nonstandard embedded DEB signature is provided.
- There is no project-operated self-update feed in this spec. Package-managed installations update through APT/DNF, and in-place mutation is blocked.
- One stable source release is permitted per UTC date. A packaging defect after immutable publication requires a new date release after correction and review.
- Native DNF verifies signed RPM/repodata integrity but does not cryptographically enforce freshness against an actively malicious TLS/CDN replay. The repository bounds honest cache staleness and monitors two networks; this residual freeze threat is explicit.
- Immediate nightly-to-older-stable return is a native package downgrade. Target validation can reject state incompatible with the older stable; the old process/state are preserved where possible and docs restore the nightly package.
- Public activation cannot begin until the protected exact-candidate dependency record, infrastructure preflight, all private archive restores, and staging promotion pass. Primary and independent GitHub/APT/RPM canaries run immediately after first activation. Failure of any one of the six legs freezes completion, README/status/spec mutation, and every subsequent publication; recovery requires fresh success from all six.
- R2 ordinary bucket tokens are not delete-only. A root broker necessarily retains powerful parent write credentials for the two nightly buckets; no-network local minting, exact-object five-minute sessions, systemd isolation, audit, and rotation reduce but cannot eliminate parent-key compromise risk.
- Repository source/key trust is deliberately external to the daemon package and remains after remove/purge/erase so refresh/reinstall works. Operators remove that pair explicitly after removing Ze.

## RFC Documentation

Not applicable. This spec does not add or change a network protocol.

## Implementation Summary

### What Was Implemented

- Fill during implementation.

### Bugs Found/Fixed

- Fill during implementation with a regression test for each bug.

### Documentation Updates

- Fill during implementation with source anchors and `make ze-doc-test` evidence.

### Deviations from Plan

- None at design time. Any deviation requires a recorded decision and matching AC/test/audit update.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| DEB packages | planned | AC-9 through AC-18, AC-28 | Exact payload, lifecycle, policy, trust |
| RPM packages | planned | AC-9 through AC-18, AC-29 | Signed package, combined multi-arch repository, native failure model |
| GitHub release downloads | planned | AC-22 through AC-27 | Immutable, final-attested, signed manifest/checksum |
| Tagged stable automation | planned | AC-1 through AC-3, AC-22 through AC-26 | Protected mirror dispatch, exact identity |
| Automated nightly downloads | planned | AC-4 through AC-5, AC-20, AC-32 | Changed-source one/day, separate channel |
| Maintained APT/RPM sources | planned | AC-20, AC-28 through AC-30 | Signed independent atomic snapshots |
| Package-first usability | planned | AC-10 through AC-19 | Safe bootstrap, service policy, update ownership |
| Reproducibility/provenance | planned | AC-6 through AC-9, AC-22 through AC-24 | Closed inputs, exact subjects, archive |
| DNS/infrastructure/security | planned | AC-25, AC-31, AC-34, AC-36 | Apps, keys, VPS, CDN, storage, monitoring |
| Operational maintenance/recovery | planned | AC-30 through AC-36 | Replay, retention, key/App rotation, backup restore |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-36 | planned | Named unit, boundary, functional, staging, QEMU, and public canary evidence above | Expand to one row per AC during implementation |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All named unit/boundary/functional/staging/public tests | planned | `🧪 TDD Test Plan` | Record each command, run ID, evidence record, and digest independently |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| All Files to Modify/Create | planned | Record actual producer/test/doc locations and approved deviations during implementation |

### Audit Summary

- **Total items:** expand and count during implementation before completion review.
- **Done:** 0 at design time.
- **Partial:** 0.
- **Skipped:** 0.
- **Changed:** 0.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Codeberg remains canonical and mirror failures fail closed | Mirror/security negative E2E | Fast-forward-only exact branch/tag tests; unauthorized/future/stale/divergent/object-mismatch and denied source-ref scenarios |
| Signed stable tag produces immutable direct downloads | Protected-dispatch/public E2E | staging `stable`, exact outer-set final attestation/signatures, both public probes |
| Users install trusted DEB from maintained source | Native integration | Three DEB cells plus Debian VM prove freshness/InRelease/by-hash/exact bytes |
| Users install trusted RPM from maintained source | Native integration | Three RPM cells plus Rocky VM prove combined repodata/signatures/architecture and stated freshness boundary |
| Nightlies are changed-source, automated, immutable, isolated, one/day, and retained | Schedule/retention E2E | changed/unchanged/three-overlap staging, native EVR/downgrade, activation-epoch-plus-lock prune |
| Only normal distro binary and declared support payload ship | Closed inventory/parity | Build smoke, six package inspections, tar allowlist, host/appliance/helper exclusion and cross-architecture negatives |
| Artifacts are reproducible and source-traceable | Repro/attestation/archive | double-build, protected-workflow/candidate races, binary/SPDX pairs, aggregate provenance, input/final/final-attestation/refresh clean restore |
| Candidate code cannot reach privileged authority | Security negatives | no-persist build/evidence permissions, isolated recorder, attestation no-exec, ref rules, App/API denials, signer/broker/storage sandbox |
| Fresh install is usable without plaintext/public exposure | Functional/system | init failure/secret tests, identity/policy matrix, booted local CLI and listener assertions |
| Upgrade, downgrade, and ordinary removal preserve operator state | Lifecycle digests | both VM full profiles compare zefs/config/credential/admin-unit bytes across every transition and failure |
| Package manager retains binary/update ownership | Mutation negatives | check/download/background/apply/restart/rollback leave binary/temp/stage unchanged and give APT/DNF guidance |
| Publication mismatches/failures/replay fail closed | State-machine faults | exact evidence/attestation/envelope/archive rejects; every transition, predecessor, rollback-auth, bucket denial |
| Operators can maintain and recover distribution | Operations rehearsal | deterministic alerts/freezes, key/App rotation/revocation, mirror divergence, retention, exact final-byte restore |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | Fill from `/ze-review-spec` | | |

### Fixes applied

- Fill after review.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review-spec` and final `/ze-review` show 0 BLOCKER, 0 ISSUE.
- [ ] All NOTEs recorded or explicitly none.

## Pre-Commit Verification

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| Every file in Files to Create | pending | Fill with fresh file checks |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-36 | pending | One row per AC before completion |

### Wiring Verified

| Entry Point | Test | Verified |
|-------------|------|----------|
| Every Wiring Test row | pending | Read test and record passing end-to-end evidence |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-11 | pending | No assumption may remain unvalidated |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Package install, release, trust, retention, operations | live source, package, workflow, public endpoint | pending |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1 through AC-36 all demonstrated.
- [ ] Every End-to-End User Story has a working path and passing test.
- [ ] Every Wiring Test row has a concrete passing test.
- [ ] `spec-release-evidence-gate.md` is complete and exact stable evidence passes.
- [ ] `spec-release-audit-0-umbrella.md` and blocking child findings are complete.
- [ ] `/ze-review-spec` and final `/ze-review` are clean.
- [ ] `make ze-verify` and `make ze-lint-changed` pass.
- [ ] `make ze-release-evidence` passes with no mandatory skip on the exact stable SHA.
- [ ] Package container and booted QEMU VM matrices pass for required distro/architecture/policy cases.
- [ ] Staging stable/nightly/attestation-race/failure/freshness/storage/key-rotation/retention/monitoring/restore exercises pass.
- [ ] Infrastructure preflight, dependency closure, raw/API-policy denials, five-bucket/three-domain prefix locks and brokered credentials, clock/monitor thresholds, input/final/final-attestation/refresh archive inventory, and clean-room restore pass.
- [ ] Primary and independent GitHub release verification plus APT/RPM canaries pass, and every isolated failure is proven to freeze completion/documentation/next publication before release-status changes.
- [ ] Documentation/discovery checklists and source anchors are complete.
- [ ] Every assumption A-1 through A-11 is confirmed or the spec returns to design.

### Quality Gates

- [ ] Implementation Audit is expanded to one row per AC/test/file and complete.
- [ ] Mistake Log escalation reviewed.
- [ ] Reproducibility comparison covers every unsigned output and architecture.
- [ ] Attestation/signature negatives cover workflow/run/ref/SHA/subject/predicate/architecture/package/metadata/key/checksum/manifest tampering.
- [ ] Publisher state/replay/failure tests cover every durable transition, the sole APT-then-RPM order, and RPM-before-APT rejection.
- [ ] No TODO, FIXME, stub, placeholder, mock publisher, fake fallback, or optional in-scope test remains.

### Design

- [ ] One canonical release identity, manifest, and publisher generation model.
- [ ] One attested architecture binary per tarball/DEB/RPM distribution view.
- [ ] One closed attested input bundle is locked before signing and every exact final byte is locked before public staging.
- [ ] No candidate code executes with attestation, archive, signing, or publication credentials.
- [ ] Only static repository content is publicly served.
- [ ] Stable public assets and input/final/final-attestation/refresh recovery archives are immutable and retained.
- [ ] Nightly channel is explicit, changed-source one-per-day, and bounded.
- [ ] Package state, transaction, service policy, identity, and update ownership match native manager semantics.
- [ ] Publisher activation is monotonic/predecessor-checked; APT freshness is client-enforced; DNF's residual malicious-CDN replay boundary is explicit.

### TDD

- [ ] Tests written before implementation.
- [ ] Tests fail for the intended missing behavior.
- [ ] Tests pass after implementation.
- [ ] Tests FAIL first and Tests PASS after, with output pasted per phase (`ai/rules/testing.md`).
- [ ] `make ze-test` passes (lint + all ze tests) in addition to the release gates above.
- [ ] Boundary tests cover syntax/trusted date/clock, entropy, doctor deadline/output, SHA, size, path/envelope, workflow artifact/evidence sets, cache/freshness, retention credentials/batches, key/monitor thresholds, scriptlet count, storage/architecture/generation, and one-per-day limits.
- [ ] Functional tests cover every user/operator entry point and every AC maps to exact evidence.
- [ ] Six native package-manager cells and two full booted-systemd lifecycle tests pass.
- [ ] All staging/public evidence records and input/final/final-attestation/refresh archive bytes are digest-bound and retention-verified.
- [ ] Goal Validation table has concrete evidence for every Task promise.

### Completion (BLOCKING - before any commit)

- [ ] Critical Review passes with no failures.
- [ ] No partial or skipped item exists without explicit user approval.
- [ ] Implementation Summary and Implementation Audit are complete.
- [ ] Release dependencies and first public canary are complete.
- [ ] Write learned summary to `plan/learned/NNN-release-distribution.md`.
- [ ] Commit A preserves code, tests, docs, spec, learned summary, and counter bump.
- [ ] Commit B removes only `plan/spec-release-distribution.md` after Commit A is preserved.
