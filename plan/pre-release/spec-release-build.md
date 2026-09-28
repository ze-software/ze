# Spec: release-build

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Add `./le build release`, which turns a signed CalVer tag into every release
artifact, modelled on tuios's GoReleaser setup but implemented in Go inside le.

Agreed scope (owner, 2026-09-28):

| Item | Decision |
|------|----------|
| Target matrix | The tuios matrix (linux, darwin, windows, freebsd, openbsd x amd64, arm64, arm, 386, minus windows/arm, windows/arm64, freebsd/arm, openbsd/arm), declared once. Targets that do not compile today are listed as unsupported with the reason and refused by name. On 2026-09-28 only linux and darwin on amd64 and arm64 compiled |
| Version | CalVer with a four-digit year, `YYYY.MM.DD` (owner, 2026-09-28, replacing the two-digit `YY.MM.DD`), never `vX.Y.Z`. A same-day rebuild needs a counter (owner, 2026-09-28): proposed `YYYY.MM.DD.N`, dot-separated because a hyphen means the package revision in dpkg and is forbidden in an RPM version. This reverses spec-release-distribution's "no correction suffix" decision and needs the runtime version parser to accept it; exact form settled at the DESIGN gate |
| Artifacts | Per-target archives, `checksums.txt`, a changelog grouped by commit prefix, `.deb` and `.rpm` for the Linux targets, a Homebrew formula for a `ze-software` tap, and `install.sh` |
| Packaging tool | nFPM used as a Go library; no GoReleaser, no external packaging binary |
| Signing | GPG, a release subkey on a hardware key, signs `checksums.txt` on the maintainer's machine. No signing key exists in CI. Sigstore is not used |
| Verification | The maintainer builds locally and uploads a draft GitHub release. A read-only CI job rebuilds from the tag and compares every artifact hash. `le` publishes the draft only once that check is green |
| Downloads | `.deb` and `.rpm` are attached to the GitHub release for direct download. No APT or DNF repository in this spec |
| install.sh | Served from `ze-software.net`, carries the release public key, verifies the signed checksums before installing |
| Homebrew | The config folder must not resolve inside the versioned Cellar folder |
| Relation to `plan/pre-release/spec-release-distribution.md` | That spec stays as the later layer: it stops building artifacts, consumes this spec's output, and adds repositories, the full package lifecycle, nightlies and retention. Its Homebrew and macOS exclusion is removed |

Reference implementation studied: tuios (github.com/Gaurav-Gosain/tuios), its
`.goreleaser.yml`, `install.sh` and `.github/workflows/release.yml`, cloned to
`tmp/research/tuios` on 2026-09-27.

## Required Reading

### Architecture Docs and Rules
- [ ] `ai/INDEX.md` "Add a development tool" - how an le command is added
  → Constraint: one package per command at the path the name predicts, so the command lives at `internal/le/build/release/`, exposes `Answer(args) (any, int)` and a `leaction.New` action table, registers through `leroot.Register`, `leroot.RegisterActions` and `leroot.RegisterShape`, and is blank-imported once from `internal/le/register.go`. Template: `internal/le/build/installer/actions.go` and `register.go`
  → Constraint: every verb that writes carries `Writes: true` in the action table
- [ ] `ai/rules/go-standards.md` - external commands
  → Constraint: the exec ban applies to shipped code only; `internal/le/` may run `git`, `gh` and `gpg`. `git` is already run (`internal/le/repo/bootstraps/tracked.go`); nothing in le runs `gh` or `gpg` today
- [ ] `ai/rules/platform-linux.md` - host versus target binaries
  → Constraint: the release binaries are target binaries and are cross-compiled; le itself is never cross-compiled
- [ ] `docs/guide/self-update.md`, `docs/architecture/config/system-update.md`, `docs/architecture/appliance/self-update.md` - the update backends
  → Constraint: backend choice is by platform only (`NewBackend` in `internal/component/config/system/backend.go`): gokrazy A/B or in-place self-update. Nothing knows about package managers
- [ ] `docs/guide/ze-install.md` - `ze install local` and `ze install systemd`
  → Constraint: `ze install systemd` refuses unless `<config>/database` exists, so it assumes `ze init` already ran
- [ ] `website/AI.md` - how ze-software.net is produced
  → Constraint: `./le site build` stages the site from `website/` into the Pages checkout `../gh-pages`; everything there is generated from this repository, and `website/CNAME` is `ze-software.net`. install.sh needs a source under `website/` or a producer in `internal/le/site`
- [ ] `plan/pre-release/spec-release-distribution.md` - the heavier release design this spec runs ahead of
  → Decision: reuse its package-managed update marker idea and its nFPM-only-for-assembly decision; defer its repositories, bootstrap, transaction state machine and nightlies
  → Constraint: its steps 8 and 9 name `/etc/ze/database.zefs` as the live store. That is wrong: the live store is the directory `/etc/ze/database/` and `database.zefs` is the import/export blob (`internal/component/config/storage/open.go`, `treeName` and `blobName`). Correct it when its scope is amended

### External References
- [ ] nFPM v2.47.0 (github.com/goreleaser/nfpm/v2), MIT - deb and rpm assembly as a Go library
  → Constraint: build `nfpm.Info` in Go and call the packager registered by the `deb` and `rpm` packages; no YAML
  → Constraint: pin v2.47.0. nFPM main (2026-09-15) moved RPM from google/rpmpack v0.7.1 to an unreleased backend, so reproducibility must be re-proved on any bump
  → Constraint: it brings about 38 third-party modules into `vendor/`, including go-git through goreleaser/chglog, ProtonMail/go-crypto, klauspost/compress, pgzip, ulikunitz/xz, cavaliergopher/cpio, sprig and mergo
  → Constraint: output is byte-identical across builders only when `MTime` is fixed to the commit time, `RPM.BuildHost` is a constant (it defaults to `os.Hostname()`), and every file is an explicit content entry with fixed mode, mtime, owner and group (tree and glob entries take on-disk mtime and umask-dependent mode)
- [ ] Fedora change "Enforcing signature checking by default" (Accepted for Fedora 45) - https://fedoraproject.org/wiki/Changes/Enforcing_signature_checking_by_default
  → Constraint: "rpm will refuse to install such packages, unless explicitly overridden with --nosignature". An unsigned `.rpm` does not install on Fedora 45 without that flag. RPM 6.0 upstream defaults to the same
  → Constraint: rpmpack puts signatures only in the RPM signature header; the lead, main header and payload are identical to the unsigned build, so a comparison can skip the signature header
- [ ] Go reproducible builds - https://go.dev/blog/rebuild, https://go.dev/doc/go1.24
  → Constraint: with `CGO_ENABLED=0`, `-trimpath`, the same toolchain, vendored modules and identical flags, a darwin host and a linux host produce the same bytes for the same GOOS/GOARCH. Every GOxxx variable (GOAMD64, GOARM, GOARM64) must be pinned, since build info records them
  → Constraint: `-buildvcs` records revision, time and a dirty flag, and since Go 1.24 stamps a pseudo-version with `+dirty`; the build uses `-buildvcs=false` and stamps version and commit through `-X`
  → Constraint: the internal linker ad-hoc signs darwin binaries with the fixed identifier `a.out`, which is deterministic
- [ ] Go `archive/tar`, `archive/zip`, `compress/gzip`
  → Constraint: archives are reproducible only with explicit headers: USTAR format, mtime truncated to seconds, uid and gid 0, fixed user and group names, explicit mode, sorted entries, a zero gzip header time. `tar.FileInfoHeader` and `AddFS` copy host metadata and are not used
- [ ] GitHub releases - https://docs.github.com/en/rest/releases/releases, immutable releases (GA 2025-10-28)
  → Constraint: "Only users with push access will receive listings for draft releases". A `contents: read` CI token cannot see the draft, so CI cannot compare against it; CI publishes its own hash list and le compares
  → Constraint: once published, an immutable release's assets cannot be added, changed or deleted and its tag cannot move; the draft, upload, publish order is the supported flow
- [ ] Homebrew - https://docs.brew.sh/Formula-Cookbook, https://goreleaser.com/deprecations/
  → Constraint: only a formula has a `service do` block (`brew services`, launchd on macOS). Casks have no service and are quarantined on download, so an unsigned binary is blocked by Gatekeeper; formula downloads are not quarantined
  → Constraint: one formula can serve macOS and Linux with per-OS and per-arch URLs and hashes
- [ ] install scripts of k3s, rustup, deno, tailscale, docker
  → Constraint: none verifies a detached signature itself. `gpg` is absent on stock macOS and many minimal Linux images, so a GPG-verifying install.sh needs `gpgv` or `gpg` and a clear refusal when neither exists

**Key insights:**
- The release build must not use `gotoolchain.New()`'s clock: version comes from the tag, build date from the tagged commit's committer time, or the CI rebuild never matches
- CI cannot see a draft release with a read-only token; the comparison runs in le against a hash list CI publishes
- Fedora 45 refuses unsigned RPMs, so RPMs must be signed, and the hash comparison for RPMs skips the signature header
- Homebrew must be a formula, not a cask, for `brew services` and to avoid quarantine
- Only linux and darwin on amd64 and arm64 compile today

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/go/toolchain/gotoolchain.go` - builds with `-X main.version` and `-X main.buildDate`; `New()` sets version to the local date as `06.01.02` and build date to the current second
  → Constraint: not reproducible; the release build needs a constructor that takes version and date explicitly
- [ ] `internal/le/repo/featuretags/daemontags.go` - `DaemonBuildTags(root, DaemonBase)` reads `feature-gates.txt`; `DaemonBase` is `ze_core ze_distro`
  → Constraint: the only source of the default tag set; the quickstart list is a derived copy
- [ ] `internal/le/build/compile/build.go` - the shared build runner behind `le build installer` and the ze-host build, through `gaterun.Run`
  → Constraint: extend this runner; do not write a second one
- [ ] `internal/core/version/version.go` - `IsValidRelease` accepts any 8-character string with dots at positions 2 and 5; `CompareReleases` compares strings
  → Constraint: `YYYY.MM.DD` and `YYYY.MM.DD.N` fail it today, and a string compare sorts `26.09.28` above `2026.09.28`
- [ ] `cmd/ze/main.go` - `version = "dev"`, `buildDate = "unknown"`, set by ldflags
- [ ] `internal/core/paths/paths.go` - `ConfigDirFromBinary` maps `<prefix>/bin/ze` to `<prefix>/etc/ze`, `/usr`, `/usr/local` and `/` to `/etc/ze`; `DefaultConfigDir` applies `ze.config.dir`, then resolves symlinks on the executable
  → Constraint: under Homebrew the binary resolves to `<brew>/Cellar/ze/<ver>/bin/ze`, so the config lands in the versioned folder that upgrades remove. Seven `DefaultConfigDir` callers and four `ConfigDirFromBinary` callers (systemd install, local install and uninstall, the exabgp test CLI) depend on it
- [ ] `internal/plugins/systemd/unit.go` - `buildUnitFile(unitSpec{BinaryPath, ConfigDir})` builds the unit as a string: `User=ze`, `ExecStart` runs `ze start`, `ZE_CONFIG_DIR`, `XDG_RUNTIME_DIR=/run/ze`, ambient NET_ADMIN, NET_RAW, NET_BIND_SERVICE, `ProtectSystem=true`; installed at `/etc/systemd/system/ze.service`
  → Constraint: the package unit is generated from this function, so the unit is declared once; the package ships it at the vendor path `/usr/lib/systemd/system/ze.service`
- [ ] `internal/plugins/systemd/cmd_install.go` - checks `<config>/database`, creates the `ze` user and group, chowns the store, writes the unit, reloads, enables
- [ ] `internal/plugins/init/main.go` - `ze init` reads credentials from a terminal or stdin; no unattended mode
  → Constraint: a package cannot initialise Ze; it installs, creates the user and `/etc/ze`, and does not enable or start the service
- [ ] `internal/component/config/system/backend.go`, `selfupdate.go` - self-update resolves its target through `os.Executable` and replaces it by rename
  → Constraint: under the packaged unit `/usr` is read-only and the operator sees "self-update not supported on read-only filesystem"; under Homebrew the Cellar is user-writable and self-update would replace a brew-owned file. No install-method marker exists
- [ ] `internal/le/verify/engine/status.go` - `CheckCertificate(root, nil)` returns Fresh and GitSHA for the local verify certificate at `tmp/ze-verify.status`, keyed on the tree hash including untracked files
- [ ] `.github/workflows/verify.yml` - runs on every push with `contents: read`; actions pinned by major version; Go from `go.mod`
  → Constraint: `internal/le/workflowcheck/workflowcheck_test.go` pins workflow privileges and requires every le action a workflow calls to resolve through the registry

**Behavior to preserve:**
- `ze install local` and `ze install systemd` keep working for source installs
- Non-release builds (`./le` builds, `go install`) keep stamping a version and date
- In-place self-update keeps working for a binary installed by hand or by install.sh

**Behavior to change:**
- Version format becomes `YYYY.MM.DD` with an optional `.N` same-day counter, parsed strictly and compared numerically
- Config directory resolution learns the Homebrew Cellar layout
- Self-update refuses, with a message naming the package manager, when Ze was installed by deb, rpm or Homebrew

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The maintainer runs `./le build release` on a signed tag at a clean checkout

### Transformation Path
1. Resolve the tag: strict CalVer parse, tag signature, clean tree, commit time
2. Extract the tagged tree with `git archive` so nothing untracked enters the build
3. Build `ze` for every supported target with the daemon tag set, stamped version, commit and date
4. Assemble archives, deb and rpm, the Homebrew formula and install.sh from the built binaries
5. Write `checksums.txt`; GPG signs it and the RPMs on the maintainer's hardware key
6. Create a draft GitHub release on the tag and upload every artifact
7. CI, triggered by the tag, rebuilds steps 2 to 4 with read-only access and uploads its hash list as a workflow artifact
8. le downloads CI's hash list, compares every artifact (RPMs past the signature header), and publishes the draft only on a full match

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Maintainer machine → GitHub | `gh release create --draft`, `gh release upload` under the maintainer's login | No |
| GitHub tag → CI | tag push triggers a `contents: read` workflow running `./le build release` | No |
| CI → maintainer | workflow artifact holding the hash list, fetched with `gh run download` | No |
| Release → user | direct download, `.deb`/`.rpm`, Homebrew tap, install.sh from ze-software.net | No |

### Integration Points
- `internal/le/build/compile` - the shared build runner, extended for the target matrix
- `internal/le/go/toolchain` - version and date stamping, extended with explicit values
- `internal/le/repo/featuretags` - the daemon tag set
- `internal/core/version` - the release format
- `internal/core/paths` - config directory resolution
- `internal/component/config/system` - update backend selection
- `internal/plugins/systemd` - the unit text
- `internal/le/site` and `website/` - install.sh publication

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Builds go through the shared `internal/le/build/compile` runner and the toolchain's ldflags, not a second runner |
| No unintended coupling | Yes | le imports the systemd plugin's unit builder only; ze gains no dependency on le |
| No duplicated functionality | Yes | The unit, the tag set and the version parser are reused, not copied |
| Zero-copy preserved where applicable | N-A | No hot path |
| Registration over hardcoding, outbound | Yes | The release command registers through `leroot.Register`; the package-managed backend registers in the update backend factory map |
| Registration over hardcoding, inbound | Yes | Target lists in install.sh, the formula and the packages derive from the one target table; backend selection derives from the install-method file, not a platform switch |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A macOS host and a Linux CI runner produce byte-identical binaries, archives, debs and rpms | Go rebuild blog; nFPM and rpmpack source read, not run | The CI comparison never matches and nothing can publish | A two-host build of every target compared byte for byte, before the design is final | unvalidated |
| A-2 | The maintainer's hardware GPG key can sign both `checksums.txt` and RPMs from le | owner decision; nFPM `SignFn` accepts a callback | RPM signing needs a different path | Signing a test RPM through `gpg --detach-sign` on the hardware key | unvalidated |
| A-3 | No released binary exists whose version uses the two-digit year | `git tag` is empty; no release workflow | A deployed device compares versions wrongly and never updates | `git tag`; asking the owner about lab devices on self-update | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Devices running current builds stamped `YY.MM.DD` sort every `YYYY.MM.DD` release as older and never self-update | a lab device reports no update available | `CompareReleases` parses both forms and compares numerically, and treats a two-digit year as 20YY |
| R-2 | nFPM pulls about 38 modules into `vendor/`, including go-git | vendor diff size; `govulncheck` findings | Pin v2.47.0; size the vendor diff at design; hand-writing deb and rpm is the rejected alternative |
| R-3 | A future nFPM or Go bump changes bytes between the maintainer and CI | the CI comparison fails on an unchanged tag | Both sides build with the pinned toolchain and vendored modules; the failure blocks publishing, never publishes a mismatch |
| R-4 | install.sh served from ze-software.net is replaced by someone who controls that site | install.sh hash differs from the one published in the release notes | Publish install.sh's hash in the signed release; the Pages site is generated from this repository only |
| R-5 | Unsigned binaries on macOS are blocked by Gatekeeper | "cannot be opened because the developer cannot be verified" | Homebrew formula (not quarantined); install.sh removes the quarantine attribute it set by downloading |
| R-6 | Stored configs carry a `# ze-schema: YY.MM.DD` stamp, and `internal/component/config/stamp.go` refuses a config stamped newer than the binary | a config written by a current build is refused, or a rollback is misjudged, after the format change | The shared comparison reads both forms, so a `26.09.28` stamp orders correctly against `2026.09.29`; covered by `TestCompareReleasesLegacyTwoDigitYear` |
| R-7 | An upload stops halfway, leaving a draft with some assets | `draft` exits non-zero; the draft lists fewer assets than `checksums.txt` | `draft` is re-runnable while the release is a draft: it replaces assets with the same name. `publish` refuses a draft whose asset set differs from `checksums.txt` |
| R-8 | The release is published, then the Homebrew tap push fails | `publish` reports the tap step failed | `publish` on an already published release that matches skips publishing and retries only the tap push |
| R-9 | le compares against a hash list from another run, commit or tag | the hash list's recorded tag or commit differs | The CI hash list records tag, commit, Go version and run id; `publish` refuses any mismatch and names the field |
| R-10 | The GPG card is absent or `gh` is not logged in | the first gpg or gh call fails | `build` checks the signing key and `draft`/`publish` check `gh auth status` before any work, and stop with the fix named |
| R-11 | A same-day correction is tagged `.N` while the earlier release stays published | two releases on one date | Intended: immutable releases cannot be replaced; `.N` sorts after the plain date in dpkg, rpm, Homebrew and Ze's own comparison |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Runtime: a wrong version comparison makes devices skip updates or refuse a config; a wrong Cellar rule moves the config folder; a wrong marker check blocks self-update for hand installs. Tooling: a bad release artifact reaches users, but only after the CI rebuild matched it |
| How is it reverted? | Runtime changes revert as commits. A published release cannot be changed (immutable); it is superseded by a `.N` release |
| Who else touches this path? | `internal/core/version` is read by config stamping and migration; `internal/core/paths` by 11 callers; the update backend by the hub. spec-release-distribution builds on this spec's output |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le build release targets` | → | the release command's target table | `test/ui/le-build-release-targets.ci` |
| `./le build release build tag <T>` | → | the build, archive, package, formula, checksum and sign stages | `TestReleaseBuildProducesEveryArtifact` (fake signer, local git fixture) |
| `./le build release rebuild tag <T>` in `.github/workflows/release-rebuild.yml` | → | the unsigned rebuild and hash list | `TestReleaseWorkflowCallsRegisteredActions` in `internal/le/workflowcheck/workflowcheck_test.go` plus `TestRebuildHashListMatchesBuild` |
| `./le build release publish tag <T>` | → | the comparison and publish gate | `TestPublishRefusesMismatch`, `TestPublishPublishesOnMatch` (fake GitHub) |
| `ze` started from a Homebrew Cellar path | → | config folder resolution | `TestConfigDirFromBinaryHomebrewCellar` |
| `ze` update check or apply with an install-method file present | → | the package-managed update backend | `TestNewBackendSelectsPackageManaged`, `test/ui/update-refused-package-managed.ci` |
| `ze doctor` | → | the install-method doctor check | `TestDoctorReportsInstallMethod` |
| `./le site build` | → | install.sh and the public key on ze-software.net | `TestSiteBuildPublishesInstallScriptAndKey` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le build release targets` | Lists all 17 matrix pairs (5 OS x 4 arch minus the 4 tuios exclusions) once each, each marked supported or unsupported; every unsupported row names the build failure that holds it back. On 2026-09-28 supported is exactly linux and darwin on amd64 and arm64 |
| AC-2 | A build asks for an unsupported target | Refused before any build, naming the target and its recorded reason |
| AC-3 | Release strings `2026.09.28`, `2026.09.28.1`, `2026.09.28.12` | Accepted |
| AC-4 | Release strings `26.09.28` as a new tag, `2026.9.28`, `2026.09.28.0`, `2026.09.28.01`, `2026.13.01`, `2026.02.30`, `v2026.09.28`, `2026.09.28-1` | Refused as a tag, with the error naming the expected form |
| AC-5 | Comparing releases | Numeric by year, month, day, then counter (absent counter sorts first): `2026.09.28` < `2026.09.28.1` < `2026.09.28.2` < `2026.09.29`; `2026.10.01` > `2026.09.30` |
| AC-6 | Comparing a legacy two-digit stamp | `26.09.28` compares equal to `2026.09.28` and below `2026.09.29`; a config stamped `26.09.28` is accepted by a `2026.09.29` binary; `dev` and unparseable still sort oldest |
| AC-7 | A non-release build (`./le` builds, the test runner) | Stamps `YYYY.MM.DD` from the date, as today but with a four-digit year |
| AC-8 | `build` on a dirty tree, a tag that does not exist, an unsigned or unverifiable tag, a tag dated after today UTC, or a commit whose GitHub verify workflow did not succeed | Refused before building, each with its own message naming the tag, the commit and the fix |
| AC-9 | `build tag <T>` succeeds | `dist/<T>/` holds, for each supported target, one archive (zip for windows, tar.gz otherwise); for each supported Linux target one `.deb` and one signed `.rpm`; plus `checksums.txt`, `checksums.txt.asc`, `ze.rb`, `install.sh` and `CHANGELOG.md` |
| AC-10 | Running `ze version` from any built binary | Prints `<T>`, the 12-character commit and the commit's UTC committer time as build date |
| AC-11 | Each binary | Built from a `git archive` of the tag with `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, the daemon tag set from `feature-gates.txt`, and pinned GOAMD64, GOARM, GOARM64 and GO386 |
| AC-12 | Building the same tag twice, in different directories, with different umasks | Every artifact except `checksums.txt.asc` is byte-identical, and every RPM is identical after its signature header |
| AC-13 | Archive contents | Exactly `ze` (or `ze.exe`), `LICENSE` and `README.md` under a top folder `ze_<T>_<os>_<arch>/`, mode 0755 for the binary and 0644 otherwise, owner root |
| AC-14 | `.deb` and `.rpm` contents | Exactly `/usr/bin/ze` (0755), `/usr/lib/systemd/system/ze.service` (0644, byte-equal to the systemd plugin's unit for `/usr/bin/ze` and `/etc/ze`), `/usr/share/ze/install-method` holding `deb` or `rpm`, and the licence under `/usr/share/doc/ze/`; version field `<T>`, architecture mapped per format (amd64/x86_64, arm64/aarch64, arm/armhf/armv7hl, 386/i386/i686) |
| AC-15 | Installing the `.deb` on Debian 12 or the `.rpm` on Fedora 45 (key imported) | Creates a locked system user and group `ze`, creates `/etc/ze` mode 0700 owned `ze`, reloads systemd when it runs, prints the `ze init` and `systemctl enable --now ze` next steps, and does not start or enable the service |
| AC-16 | Removing the package | Stops and disables the service, leaves `/etc/ze` and the `ze` user in place |
| AC-17 | The unsigned `.rpm` on Fedora 45 | Refused by `rpm -i`, which the smoke test asserts, proving the signature is what makes the signed one install |
| AC-18 | `checksums.txt` and its signature | Lists the SHA-256 of every other asset, sorted; `gpgv` verifies `checksums.txt.asc` with the key in `website/release/ze-release.asc`; a different key fails |
| AC-19 | Homebrew formula `ze.rb` | Has one URL and SHA-256 per supported macOS and Linux target, installs `ze` and `share/ze/install-method` holding `homebrew`, and has a `service do` block running `ze start` with keep-alive |
| AC-20 | `ze` started from `<brew>/Cellar/ze/<ver>/bin/ze` | Resolves its config folder to `<brew>/etc/ze`, and so does the crash folder; `ze.config.dir` still overrides |
| AC-21 | Update check, download or apply when `<prefix>/share/ze/install-method` next to the resolved binary holds `deb`, `rpm` or `homebrew` | Refused with a message naming the install method and the upgrade path (the release page for deb and rpm, `brew upgrade ze` for homebrew); the binary is never touched |
| AC-22 | The install-method file holds any other value, or cannot be read | Update refused, naming the file and its content or error |
| AC-23 | No install-method file | In-place self-update behaves exactly as before |
| AC-24 | `ze doctor` | Reports the install method (`deb`, `rpm`, `homebrew`, or `direct` when absent); an unknown or unreadable file is an error with its own diagnostic code |
| AC-25 | `CHANGELOG.md` | Lists commits since the previous release tag, grouped by the subject prefix before the colon, groups sorted by name; the first release holds one line saying it is the first |
| AC-26 | `draft tag <T>` | Creates a draft release on the tag with `CHANGELOG.md` as notes and uploads every asset; re-running replaces assets while it is a draft; refuses if the release is already published |
| AC-27 | Tag push reaches GitHub | `release-rebuild.yml` runs with `contents: read` only, runs `./le build release rebuild tag <T>`, and uploads a hash list recording tag, commit, Go version, run id and the SHA-256 of every unsigned artifact (RPMs hashed after the signature header) |
| AC-28 | `publish tag <T>` with a matching hash list | Publishes the draft and pushes `ze.rb` to `ze-software/homebrew-tap` |
| AC-29 | `publish tag <T>` with any mismatch: an artifact hash, a missing or extra artifact, tag, commit, or Go version | Refused, naming each differing artifact or field with both values; the draft stays a draft |
| AC-30 | `publish` on a release already published and matching | Skips publishing and retries the tap push only |
| AC-31 | `./le site build` | Writes `install.sh` and `release/ze-release.asc` into the site; the key text in both comes from `website/release/ze-release.asc` |
| AC-32 | `curl -fsSL https://ze-software.net/install.sh \| sh` on linux amd64/arm64 or darwin arm64 | Installs the latest release to `/usr/local/bin/ze` (or `ZE_INSTALL_DIR`), after verifying the signature with `gpgv` or `gpg` and the archive hash; clears the macOS quarantine attribute; `ZE_VERSION` selects a release |
| AC-33 | install.sh with a tampered archive, a tampered `checksums.txt`, no `gpgv`/`gpg`, or an unsupported platform | Refuses before installing anything, naming the cause and, for a missing tool, the install command |
| AC-34 | Every target list in install.sh, the formula and the packages | Derived from the one target table; a target added there appears in each without another edit |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Maintainer releases | signed tag → `build` → `draft` → CI `rebuild` → `publish` → tap | Rehearsal against a scratch repository with a test key, recorded in Goal Validation |
| 2 | Debian user installs the `.deb` | download → `apt install ./ze_<T>_amd64.deb` → user, `/etc/ze` → `ze init` → `systemctl enable --now ze` | Debian 12 smoke test in `release-rebuild.yml` |
| 3 | Fedora user installs the `.rpm` | `rpm --import` key → `dnf install ./ze-<T>-1.x86_64.rpm` → same as 2 | Fedora 45 smoke test in `release-rebuild.yml` |
| 4 | macOS user installs with Homebrew | `brew install ze-software/tap/ze` → config in `<brew>/etc/ze` → `brew services start ze` | macOS smoke test installing `ze.rb` rewritten to local file URLs |
| 5 | Linux or macOS user installs with curl | install.sh → GitHub latest → verify → install | `TestInstallScriptInstallsVerifiedRelease` against a local HTTP fixture, run on Linux and macOS |
| 6 | A packaged user runs self-update | install-method file → package-managed backend → refusal naming the upgrade path | `test/ui/update-refused-package-managed.ci` |
| 7 | A tampered artifact reaches publish | CI hash list differs → publish refused | `TestPublishRefusesMismatch` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRelease` | `internal/core/version/version_test.go` | AC-3, AC-4 | |
| `TestCompareReleasesNumeric` | `internal/core/version/version_test.go` | AC-5 | |
| `TestCompareReleasesLegacyTwoDigitYear` | `internal/core/version/version_test.go` | AC-6 | |
| `TestDevStampFourDigitYear` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-7 | |
| `TestReleaseToolchainStampsFromTag` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-10, AC-11 | |
| `TestConfigDirFromBinaryHomebrewCellar` | `internal/core/paths/paths_test.go` | AC-20 | |
| `TestNewBackendSelectsPackageManaged` | `internal/component/config/system/backend_test.go` | AC-21, AC-23 | |
| `TestPackageManagedRefusesUnknownMethod` | `internal/component/config/system/backend_test.go` | AC-22 | |
| `TestDoctorReportsInstallMethod` | `internal/component/config/system/doctor_test.go` | AC-24 | |
| `TestTargetTableCoversMatrix` | `internal/le/build/release/targets_test.go` | AC-1 | |
| `TestUnsupportedTargetRefusedByName` | `internal/le/build/release/targets_test.go` | AC-2 | |
| `TestBuildRefusesUnreleasableTag` | `internal/le/build/release/gate_test.go` | AC-8 (each refusal) | |
| `TestReleaseBuildProducesEveryArtifact` | `internal/le/build/release/build_test.go` | AC-9 | |
| `TestArtifactsReproducible` | `internal/le/build/release/build_test.go` | AC-12 | |
| `TestArchiveLayout` | `internal/le/build/release/archive_test.go` | AC-13 | |
| `TestPackagePayload` | `internal/le/build/release/packages_test.go` | AC-14 | |
| `TestPackageUnitMatchesSystemdBuilder` | `internal/le/build/release/packages_test.go` | AC-14 | |
| `TestRPMHashSkipsSignatureHeader` | `internal/le/build/release/compare_test.go` | AC-12, AC-27 | |
| `TestChecksumsCoverEveryAsset` | `internal/le/build/release/checksums_test.go` | AC-18 | |
| `TestFormulaCoversSupportedTargets` | `internal/le/build/release/formula_test.go` | AC-19, AC-34 | |
| `TestChangelogGroupsByPrefix`, `TestChangelogFirstRelease` | `internal/le/build/release/changelog_test.go` | AC-25 | |
| `TestDraftUploadsAndIsRerunnable` | `internal/le/build/release/github_test.go` | AC-26 | |
| `TestRebuildHashListMatchesBuild` | `internal/le/build/release/compare_test.go` | AC-27 | |
| `TestPublishPublishesOnMatch`, `TestPublishRefusesMismatch`, `TestPublishRetriesTapOnly` | `internal/le/build/release/github_test.go` | AC-28, AC-29, AC-30 | |
| `TestSiteBuildPublishesInstallScriptAndKey` | `internal/le/site/build_test.go` | AC-31 | |
| `TestInstallScriptInstallsVerifiedRelease`, `TestInstallScriptRefuses` | `internal/le/build/release/installscript_test.go` | AC-32, AC-33, AC-34 | |
| `TestReleaseWorkflowCallsRegisteredActions` | `internal/le/workflowcheck/workflowcheck_test.go` | AC-27 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| month | 01-12 | 12 | 00 | 13 |
| day | 01-last day of month | 29 on 2028-02, 28 on 2026-02 | 00 | 29 on 2026-02, 32 |
| same-day counter | 1-999 | 999 | 0 | 1000 |
| year | 2026-9999 | 9999 | 2025 | 10000 (five digits) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-build-release-targets` | `test/ui/le-build-release-targets.ci` | The maintainer lists targets and sees supported and unsupported with reasons | |
| `update-refused-package-managed` | `test/ui/update-refused-package-managed.ci` | A packaged ze refuses self-update and names the upgrade path | |
| Package smoke tests | `.github/workflows/release-rebuild.yml`, run through `./le build release smoke` | Debian 12 and Fedora 45 install, user and folder created, service not started, remove keeps `/etc/ze`; unsigned RPM refused; macOS formula install | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol behavior changes.

## Files to Modify
- `internal/core/version/version.go` - strict `YYYY.MM.DD[.N]` parsing, numeric comparison, legacy two-digit read, commit stamp
- `internal/component/config/stamp.go`, `internal/component/config/migration/evolution.go` - comments and error text naming the new format
- `internal/le/go/toolchain/gotoolchain.go` - four-digit dev stamp; a release mode taking version, date and commit explicitly with `-trimpath` and `-buildvcs=false`
- `internal/test/runner/runner.go` - the test runner's own dev stamp uses the four-digit year
- `cmd/ze/main.go`, `cmd/ze/dispatch.go` - a `commit` ldflag variable passed to the version stamp
- `internal/core/paths/paths.go` - the Homebrew Cellar rule in `ConfigDirFromBinary`
- `internal/component/config/system/backend.go`, `selfupdate.go` - install-method lookup and backend selection
- `internal/plugins/systemd/unit.go` - export the unit builder so le generates the package unit from it
- `internal/le/build/compile/build.go` - the target matrix and release build mode
- `internal/le/register.go` - blank import of the release command
- `internal/le/site/build.go` - install.sh and the public key
- `internal/le/workflowcheck/workflowcheck_test.go` - the new workflow's privileges
- `go.mod`, `go.sum`, `vendor/` - nFPM v2.47.0
- `ai/INDEX.md` - the le inventory row
- `plan/pre-release/spec-release-distribution.md` - scope amended to consume this spec's output; the `database.zefs` statement corrected
- `docs/guide/quickstart.md` - installation by package, Homebrew and curl
- `docs/guide/self-update.md`, `docs/architecture/config/system-update.md` - the package-managed refusal
- `docs/guide/ze-install.md` - how package installs relate to `ze install`
- `docs/guide/status.md` - the Packaging row
- `docs/architecture/config/syntax.md`, `docs/config-reference.md` - the release format in the schema stamp

## Files to Create
- `internal/le/build/release/` - `actions.go`, `register.go`, `targets.go`, `gate.go`, `build.go`, `archive.go`, `packages.go`, `checksums.go`, `sign.go`, `formula.go`, `changelog.go`, `github.go`, `compare.go`, `installscript.go`, `smoke.go`, and their tests
- `internal/le/build/release/scripts/` - the deb and rpm post-install, pre-remove and post-remove scripts, embedded
- `internal/le/build/release/install.sh.tmpl` - the install script template
- `internal/component/config/system/backend_package.go` - the package-managed backend
- `internal/component/config/system/doctor.go` (or the existing doctor file of that package) - the install-method check
- `website/release/ze-release.asc` - the release public key (the owner supplies it)
- `.github/workflows/release-rebuild.yml` - the read-only rebuild and smoke tests
- `docs/contributing/releasing.md` - the maintainer runbook: key setup, tagging, build, draft, publish, tap repository creation
- `test/ui/le-build-release-targets.ci`, `test/ui/update-refused-package-managed.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | Yes | `./le build release` in `internal/le/build/release/`; no `ze` command added |
| CLI grammar (keyword before value) | Yes | `tag <T>`, `repo <owner/name>`; checked by `./le cli grammar` |
| Editor autocomplete | N-A | le command, no editor surface |
| Functional test for new RPC/API | Yes | `test/ui/le-build-release-targets.ci`, `test/ui/update-refused-package-managed.ci` |
| Pipe completeness | Yes | `targets` output goes through the shared pipe layer like other le answers |
| Env var registration | N-A | No `ze.*` env var added; install.sh's `ZE_VERSION` and `ZE_INSTALL_DIR` belong to the script, not to ze |
| Doctor check for runtime dependencies | Yes | The install-method file: a check and diagnostic codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | N-A | No runtime state to observe |
| BGP family surface | N-A | No BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (installation channels) |
| 2 | Config syntax changed? | No | The config stamp format changes only its version string; no syntax |
| 3 | CLI command added/changed? | Yes | `docs/contributing/releasing.md` for le; `docs/guide/command-reference.md` unaffected (no ze command) |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | The systemd plugin exports its unit builder; behavior unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/quickstart.md`, `docs/guide/self-update.md`, `docs/guide/ze-install.md` |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | The smoke tests live in a workflow and le, not the functional test harness |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/system-update.md` (backend selection) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` Packaging row |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Full list DERIVED by `./le spec citation anchors spec plan/pre-release/spec-release-build.md` at implementation. Declared by `// Design:` headers of changed files: `docs/architecture/config/syntax.md` (stamp.go, evolution.go) is AFFECTED: its `# ze-schema:` example must show a release in the new form; `docs/architecture/cli/plugin-modes.md` (unit.go) unaffected, exporting the unit builder changes no mode; `docs/architecture/core-design.md` (gotoolchain.go, compile/build.go, register.go) unaffected, le gains a command in the existing pattern; `docs/architecture/system-architecture.md` (cmd/ze/main.go, dispatch.go) unaffected, the entry point gains one stamped value; `docs/architecture/testing/ci-format.md` (runner.go) unaffected, only the dev stamp's year width changes; `docs/features/ai-first.md` unaffected, no agent-facing surface changes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/quickstart.md` install commands; the `YY.MM.DD` format in `docs/config-reference.md` and `docs/architecture/config/system-update.md` |

## Implementation Steps

1. **Phase: Wiring** -- register `./le build release` with the `targets` verb and stubs for the others
   - Tests: `test/ui/le-build-release-targets.ci`, `TestTargetTableCoversMatrix`
   - Files: `internal/le/build/release/actions.go`, `register.go`, `targets.go`, `internal/le/register.go`, `ai/INDEX.md`
   - Verify: the command lists the matrix; the other verbs exist and fail as stubs
2. **Phase: Version format** -- strict `YYYY.MM.DD[.N]`, numeric comparison, legacy read, four-digit dev stamp, commit stamp
   - Tests: `TestParseRelease`, `TestCompareReleasesNumeric`, `TestCompareReleasesLegacyTwoDigitYear`, `TestDevStampFourDigitYear`
   - Files: `internal/core/version/version.go`, `gotoolchain.go`, `runner.go`, `cmd/ze/main.go`, `dispatch.go`, config stamp and evolution
   - Verify: every stored `YY.MM.DD` stamp still orders correctly
3. **Phase: Install location** -- Homebrew Cellar config rule; install-method file, package-managed backend, doctor check
   - Tests: `TestConfigDirFromBinaryHomebrewCellar`, `TestNewBackendSelectsPackageManaged`, `TestPackageManagedRefusesUnknownMethod`, `TestDoctorReportsInstallMethod`, `test/ui/update-refused-package-managed.ci`
   - Files: `paths.go`, `backend.go`, `selfupdate.go`, `backend_package.go`, doctor file, `codes.go`, self-update docs
4. **Phase: Binaries and archives** -- release toolchain mode, `git archive` build tree, target matrix, archives, checksums, gate checks
   - FIRST (owner, 2026-09-28): validate A-1 before anything else in this phase. Build linux/amd64 and darwin/arm64 binaries, one archive and a sample unsigned `.deb` and `.rpm` on the macOS host and inside a Linux environment, and compare SHA-256. A mismatch stops the spec and returns to DESIGN before phases 5 to 9 start
   - Tests: `TestReleaseToolchainStampsFromTag`, `TestBuildRefusesUnreleasableTag`, `TestArchiveLayout`, `TestArtifactsReproducible`, `TestChecksumsCoverEveryAsset`
   - Files: `gotoolchain.go`, `compile/build.go`, `gate.go`, `build.go`, `archive.go`, `checksums.go`
5. **Phase: Packages and signing** -- nFPM vendored, deb and rpm, embedded scripts, unit from the systemd plugin, GPG signing of RPMs and `checksums.txt`, RPM hashing past the signature header
   - Tests: `TestPackagePayload`, `TestPackageUnitMatchesSystemdBuilder`, `TestRPMHashSkipsSignatureHeader`
   - Files: `go.mod`, `vendor/`, `packages.go`, `scripts/`, `sign.go`, `compare.go`, `unit.go`
6. **Phase: Formula and changelog**
   - Tests: `TestFormulaCoversSupportedTargets`, `TestChangelogGroupsByPrefix`, `TestChangelogFirstRelease`
   - Files: `formula.go`, `changelog.go`
7. **Phase: GitHub** -- `draft`, the read-only `release-rebuild.yml`, `publish` with comparison and tap push, smoke tests
   - Tests: `TestDraftUploadsAndIsRerunnable`, `TestRebuildHashListMatchesBuild`, `TestPublishPublishesOnMatch`, `TestPublishRefusesMismatch`, `TestPublishRetriesTapOnly`, `TestReleaseWorkflowCallsRegisteredActions`
   - Files: `github.go`, `smoke.go`, `.github/workflows/release-rebuild.yml`, `workflowcheck_test.go`
8. **Phase: install.sh and site**
   - Tests: `TestInstallScriptInstallsVerifiedRelease`, `TestInstallScriptRefuses`, `TestSiteBuildPublishesInstallScriptAndKey`
   - Files: `install.sh.tmpl`, `installscript.go`, `internal/le/site/build.go`, `website/release/ze-release.asc`
9. **Phase: Rehearsal and docs** -- full release against a scratch repository with a test key; runbook; distribution spec amended
   - Files: `docs/contributing/releasing.md`, user docs, `plan/pre-release/spec-release-distribution.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Reproducibility | No artifact byte depends on the clock, hostname, umask, directory, user, or file-system order |
| Single declaration | Targets, tag set, unit text and public key are each declared once; every other surface derives from them |
| Correctness | Version comparison is numeric and reads the legacy form; the install-method check fails closed |
| Rule: never destroy work | `build` writes only under `dist/<T>/` and refuses to overwrite a different existing build of the same tag |
| Rule: git safety | le never pushes a git ref: the tag is pushed by the owner; the tap is updated through the GitHub contents API by `publish`, which only the owner runs |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `./le build release` with `targets`, `build`, `draft`, `rebuild`, `publish`, `smoke` | `./le build release` lists them |
| Reproducible artifacts | `TestArtifactsReproducible` and a green `release-rebuild.yml` run on the rehearsal tag |
| Package-managed refusal | `test/ui/update-refused-package-managed.ci` |
| install.sh on the site | `./le site build` output holds `install.sh` and `release/ze-release.asc` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Signing key custody | The private key never leaves the hardware key; nothing in le or CI reads a private key file |
| CI privileges | `release-rebuild.yml` has `contents: read` and no secrets |
| Hash list binding | `publish` checks tag, commit and run before trusting any hash |
| install.sh | Verifies the signature before the hash and the hash before installing; refuses rather than skips when a tool is missing; never runs a downloaded file before both checks pass |
| Package scripts | Create only the `ze` user, `/etc/ze` and systemd state; no network, no secrets, no service start |
| Install-method file | An unreadable or unknown value refuses updates rather than allowing them |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A two-builder comparison does not match | Find the varying byte; A-1 is broken → DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The CI witness cannot see a draft release, so the comparison lives with the party that can: the maintainer's le
- The same binary goes into every channel, so anything that must differ per channel (install method) lives in a file beside the binary, never in the binary

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Maintainer builds and signs, CI rebuilds as a read-only witness | CI builds and maintainer verifies; GoReleaser end to end | Owner choice; no key in CI, no write token where tagged code runs |
| `YYYY.MM.DD[.N]` with numeric comparison and legacy two-digit read | `YY.MM.DD` only (distribution spec); semver | Owner choice; `.N` sorts correctly in dpkg, rpm, Homebrew and Ze |
| GPG on a hardware key | Sigstore keyless; both | Owner choice; rpm and apt users know GPG |
| Sign RPMs as well as `checksums.txt` | Unsigned RPMs | Fedora 45 refuses unsigned RPMs |
| Homebrew formula | Cask | Only a formula has `brew services`; casks are quarantined |
| nFPM v2.47.0 as a library | GoReleaser, hand-written deb and rpm | Owner choice; avoids a second build model; pinned because its RPM backend is changing |
| Install-method file beside the binary | Build flag; env var | One binary for every channel keeps the hash comparison possible |
| Homebrew rule in `ConfigDirFromBinary` | Formula wrapper setting `ze.config.dir` | Covers all 11 callers and keeps the binary a binary |
| Release gate is a green GitHub verify run on the tagged commit | Local verify certificate; none | Owner choice; works from any machine |
| Package installs but does not start ze | Automatic init | `ze init` has no unattended mode; the unattended bootstrap stays in spec-release-distribution |

## Known Limitations

- Windows, FreeBSD, OpenBSD and every 32-bit target are listed but unsupported until Ze compiles there; each port is separate work
- No APT or DNF repository, no nightly channel, no package lifecycle beyond install and remove: spec-release-distribution
- macOS binaries are not signed or notarised by Apple; Homebrew and install.sh avoid the quarantine block, a direct browser download does not
- install.sh needs `gpgv` or `gpg` on the machine

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-release-build.md` only, in the same `./le commit create` script
