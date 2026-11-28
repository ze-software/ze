# Spec: release-build

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P2 |
| Release reason | Required release artifacts, safe installation or appliance/runtime-kernel delivery and their outstanding boot evidence. |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Status returned from `ready` to `design` on 2026-11-28: the Task and the acceptance criteria changed (the versioned install below), so the earlier approval no longer covers this design.

## Task

Add `./le build release`, which turns a signed CalVer tag into every release
artifact, modelled on tuios's GoReleaser setup but implemented in Go inside le.

Agreed scope (owner, 2026-09-28; the Version, Artifacts and install.sh rows carry what he changed on 2026-11-28):

| Item | Decision |
|------|----------|
| Target matrix | The tuios matrix (linux, darwin, windows, freebsd, openbsd x amd64, arm64, arm, 386, minus windows/arm, windows/arm64, freebsd/arm, openbsd/arm), declared once. Targets that do not compile today are listed as unsupported with the reason and refused by name. On 2026-09-28 only linux and darwin on amd64 and arm64 compiled |
| Version | CalVer with a two-digit year, `YY.MM.DD`, never `vX.Y.Z`, and a same-day counter, `YY.MM.DD.N` (owner, 2026-11-28, voice-transcribed: "I thought we had settled on a Calver but adding effectively the day counter", "We want only two characters for the year."). This reverses the four-digit year he chose on 2026-09-28. The counter is dot-separated because a hyphen means the package revision in dpkg and is forbidden in an RPM version. Go's module version rules are no constraint: "Ze is not a library, it does not matter." |
| Artifacts | Per-target archives, `checksums.txt`, a changelog (owner, 2026-11-09, replacing "grouped by commit prefix": the news fragments added since the previous release tag, grouped by topic, never commit subjects), `.deb` and `.rpm` for the Linux targets, and a Homebrew formula for a `ze-software` tap. `install.sh` left this list on 2026-11-28 |
| Packaging tool | nFPM used as a Go library; no GoReleaser, no external packaging binary. Confirmed on 2026-11-28: nFPM stays, the package follows Debian Policy |
| Signing | GPG, a release subkey on a hardware key, signs `checksums.txt` on the maintainer's machine. No signing key exists in CI. Sigstore is not used |
| Verification | The maintainer builds locally and uploads a draft GitHub release. A read-only CI job rebuilds from the tag and compares every artifact hash. `le` publishes the draft only once that check is green |
| Downloads | `.deb` and `.rpm` are attached to the GitHub release for direct download. No APT or DNF repository in this spec (confirmed on 2026-11-28: the repository comes after the first release) |
| install.sh | Withdrawn (owner, 2026-11-28, voice-transcribed): "I don't think we need an install.sh; we just need a tar with absolute path for installation, and people can get directly files in the right location.", "I don't want an install.sh , That way we have a script for ze so that can be built inside of it." The Linux tarball holds its files at their final paths, and the `ze` launcher script carries the install and uninstall functions. install.sh was also what verified the signed checksums before installing; who does that now is choice Q-2 |
| Homebrew | The config folder must not resolve inside the versioned Cellar folder |
| Relation to `plan/release/delivery/spec-release-distribution.md` | That spec stays as the later layer: it stops building artifacts, consumes this spec's output, and adds repositories, the full package lifecycle, nightlies and retention. Its Homebrew and macOS exclusion is removed |

Reference implementation studied: tuios (github.com/Gaurav-Gosain/tuios), its
`.goreleaser.yml`, `install.sh` and `.github/workflows/release.yml`, cloned to
`tmp/research/tuios` on 2026-09-27.

### Versioned install (owner, 2026-11-28)

A second goal joined this spec on 2026-11-28. Ze now ships its own VPP build as files (`plan/release/delivery/spec-vpp-embedded-runtime.md`), so one file at `<prefix>/bin/ze` is no longer a whole installation. An installation becomes one directory per release, holding `ze` and its VPP together, and a launcher script at `<prefix>/bin/ze` that starts the right one. Several releases stay installed side by side. This spec owns that layout, the launcher, the listing and the removal of releases, what `ze install`, `ze uninstall`, the systemd unit and the self-updater become, and the tarball, DEB and RPM that carry the layout.

His messages are voice-transcribed and are read for their meaning. Each row below is his word, relayed by the session that asked him and recorded in `plan/handover/64-vpp-ships-as-files-with-ze.md`.

| His words | What it settles here |
|-----------|----------------------|
| "install following the FSB on a linux FS (probably /usr/local/) and then add a script in /usr/local/bin so we can have multiple version installed and change based on a env variable" | An install that is not a package lives under `/usr/local`, as the Filesystem Hierarchy Standard says. `/usr/local/bin/ze` is a script. Several versions are installed at once, and an environment variable chooses among them |
| "old version are kept the ze script providing an uninstall option a bit like we do with ./le" | Installing a release never removes an older one. Removal is an explicit action that the `ze` script provides. The `./le` launcher is the model |
| "lgtm" to this shape: a listing shows the installed versions and the selected one; removing one version removes its directory, `ze` and its VPP together | The behavior of the listing and of the removal. Their spelling is choice Q-8 |
| "same for vpp install it in the same folder as ze and make sure ze will scan that folder" | The VPP files sit in the directory that holds the real `ze` |
| Asked where the Ze folder is on a package install (A: `ze` at `/usr/bin/ze`, folder `/usr/lib/ze`; B: `ze` itself inside the folder with `/usr/bin/ze` a link; C: `/usr/libexec/ze`): "Neither, like B, but the /usr/bin/ze is a script, not a symlink." | The real `ze` program lives inside the Ze folder with its VPP beside it. `<prefix>/bin/ze` is a launcher script that runs it, never a symbolic link |
| Asked whether a package install uses version directories too (A: one layout and one launcher everywhere, with the example `/usr/bin/ze` (the launcher), `/usr/lib/ze/2026.11.28/ze` and `/usr/lib/ze/2026.11.28/vpp/`; B: a second, simpler layout for packages): "a" | The DEB and the RPM install `/usr/bin/ze` (the launcher) and `/usr/lib/ze/<release>/`. One layout, one launcher script. The listing shows the one version, and its removal is refused with the package manager's command |
| Asked whether the selected version is recorded in a file or a link: "It should be the latest one, unless an environment value is set in the shell running it setting another version in the list of valid options." | No marker exists, neither file nor link. The launcher runs the latest installed release, unless the variable names another installed release. This replaced his earlier answer "a" to a `current` marker. That a value naming no installed release is refused with the list is this design's reading of "in the list of valid options" |
| Asked what the systemd service runs (A: the launcher; B: the real program of one version): "A" | The unit runs the launcher. A running daemon keeps the version it started with until it restarts |
| Asked how a bad latest release is rolled back (A: removal refuses only a version a running daemon uses; the operator stops the service, removes the bad release and starts the service; B: removal refuses the latest and the operator pins the old one through the variable): "A" | Removal refuses only a version a running daemon is using, and its refusal gives the commands that work. No rollback command and no pin are needed. This replaced his earlier answer that removal also refuses the selected version |
| Asked how a `/usr/local` Ze is removed completely (A: one explicit command removes every version directory, the launcher and the service unit, and leaves `/etc/ze` alone; B: no command; C: as A and also deletes the configuration): "a" | One command removes the whole installation and keeps the configuration. It refuses while the service runs and names the command that stops it, as the option he chose records (handover 64, L-5). A package install is removed by its package manager |
| Asked whether the commands take the grammar's spelling (A) or keep `ze version list` and `ze version uninstall <v>` as an exception (B): "a" | Every command of this design is spelled as `ai/rules/cli.md` requires. The spellings themselves are owed to him before anything is built: choice Q-8 |
| Asked what "the usual Debian packaging method in repos" changes (A: nFPM stays, the package follows Debian Policy for file locations, maintainer scripts and configuration kept on removal, and the APT and DNF repository stays after the first release; B: the repository in the first release; C: debhelper in place of nFPM): "a" | nFPM stays. The DEB follows Debian Policy. No repository in this spec |
| "ok for having a .deb for installation too follow the usal debian packaging method in repos", "One package", "you can do rpm too" | One package carries `ze` and its VPP, as a DEB and as an RPM |
| Asked who installs a self-update when Ze runs as its service (A: the daemon downloads and verifies, root installs with one command; B: A plus a root-owned helper; C: version directories live where user `ze` can write, fully automatic): "C We do not require root users to run Ze. It could be run as a Ze user, provided the capability required to run the programs is available." | Ze does not require root. The version directories live where the `ze` user can write, and that user installs a new version directory itself, so auto-apply needs no operator. This departs from the page sentence that every program the service runs is owned by root (Design presentation, "Documents departed from"). Which owners the trust check then accepts is choice Q-4 |
| "The automatic update should only be for Linux, for the appliance it should be immutable infrastructure." | No automatic update on the appliance, and nothing on a running appliance installs, replaces or selects a program. This design reads the sentence as covering the whole versioned install: version directories and the launcher are for plain Linux, and the appliance keeps one `ze` and one VPP in its image, replaced with the image through the existing A/B backend. That reading is wider than his sentence, which names the automatic update |
| "I thought we had settled on a Calver but adding effectively the day counter", "We want only two characters for the year." | A release is named `YY.MM.DD`, or `YY.MM.DD.N` for a later release of the same day. A version directory is named by the release it holds. The counter starting at 1, `.0` refused, and a missing counter ordering before `.1` are carried over from this spec's criteria of 2026-09-28, not from his words of 2026-11-28 |
| "I don't think we need an install.sh; we just need a tar with absolute path for installation, and people can get directly files in the right location.", "I don't want an install.sh , That way we have a script for ze so that can be built inside of it." | No install.sh. The Linux tarball's member paths are the final locations. The `ze` launcher script carries the install and uninstall functions. What those functions take and do is Q-1, taken on the design's recommendation as (a) |

Not asked, because his words above already answer it: a self-update installs a new version directory, `ze` and its VPP together, and no in-between state is built (the handover's C-3).

Every example below names a release in the new form, such as `26.11.28` and `26.12.03`.

### What is not in this spec

| Item | Why | Where it lives |
|------|-----|----------------|
| Building the VPP bundle, finding it at start, the appliance image | The sibling spec owns the bundle and the rule that Ze finds VPP beside the `ze` that runs | `plan/release/delivery/spec-vpp-embedded-runtime.md` |
| APT and DNF repositories, nightlies, the full package lifecycle | Owner decision above | `plan/release/delivery/spec-release-distribution.md`, `plan/post-release/features/spec-release-repositories-and-nightlies.md` |
| Any change to how an appliance takes new software | Owner decision above: the appliance is immutable | The existing `gokrazy-ab` backend and `ze appliance push`, unchanged |

### The two kinds of system, kept apart

| | Plain Linux (tarball, DEB, RPM) | Appliance (gokrazy image) |
|-|---------------------------------|---------------------------|
| Where `ze` is | `<prefix>/lib/ze/<release>/ze`, started by the launcher `<prefix>/bin/ze` | `/user/ze`, started by gokrazy. No launcher is placed in the image |
| Versions installed | One or more | One, the image's |
| The version listing | Every installed release | One row, the running release |
| Removing one version, installing, removing everything | Act, or refuse with the package manager's command on a DEB or RPM install | Refuse, and name the image update: `ze appliance push <name>` from the build host |
| Update | Installs a version directory (not on a DEB or RPM install, which refuses as AC-21 says) | The `gokrazy-ab` backend, unchanged |
| How Ze knows which it is | `host.DetectPlatform` answers a platform other than `PlatformGokrazy` | `host.DetectPlatform` answers `PlatformGokrazy` (`internal/component/host/inventory.go`), the detection `NewBackend` already uses. No second detection is added |

His words name Linux. The macOS archive and the Homebrew formula take the same layout, with no `vpp` (Q-10, taken on the design's recommendation as (a)).

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
- [ ] `plan/release/delivery/spec-release-distribution.md` - the heavier release design this spec runs ahead of
  → Decision: reuse its package-managed update marker idea and its nFPM-only-for-assembly decision; defer its repositories, bootstrap, transaction state machine and nightlies
  → Constraint: its steps 8 and 9 name `/etc/ze/database.zefs` as the live store. That is wrong: the live store is the directory `/etc/ze/database/` and `database.zefs` is the import/export blob (`internal/component/config/storage/open.go`, `treeName` and `blobName`). Correct it when its scope is amended

- [ ] `website/changes/fragments/README.md` - the news fragments the release notes are made of
  → Decision: `CHANGELOG.md` is the fragments added since the previous release tag, grouped by topic, never commit subjects (owner, 2026-11-09; AC-25)
  → Constraint: a fragment is read through the one parser in `internal/le/commit/newsfragment.go`; the changelog keeps no second copy of the heading list or of the topic vocabulary (`website/data/topics.json`)
  → Constraint: a fragment's release membership comes from git (`--diff-filter=A` between the tags), never from its front matter, which carries no date

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

### Read for the versioned install (2026-11-28)
- [ ] `ai/doors/system.md`, `ai/doors/core.md`, `ai/doors/system-plugins.md`, `ai/doors/commands-cli.md`, `ai/doors/hub-engine.md`, `ai/doors/appliance.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`, `ai/doors/forwarding.md` - the doors of every tree this design edits
  → Constraint: a core package imports no component and no plugin. The layout code that both the local plugin and the update backend need is a core leaf package
  → Constraint: a command lives with its owner. The install commands stay in `internal/plugins/local`, which already owns `ze install` and `ze uninstall`
  → Constraint: a handler answers data, never text; a command that enumerates is spelled `list`
  → Constraint: a gokrazy device does not replace its own `ze` binary
- [ ] `ai/rules/cli.md`, `docs/architecture/cli/root-namespace-grammar.md`, `docs/architecture/cli/command-verbs.md`, `ai/patterns/cli-command.md` - how a command is spelled and registered
  → Constraint: the verb is chosen by effect: a read is `show`, something that stops existing is `delete`
  → Constraint: an action on one member of a set names it with a typed keyword, and the bare prefix is refused with the explicit form
  → Constraint: a `--flag` belongs to the offline `cmd/ze` tooling only, which is where `ze install` and `ze uninstall` already are
  → Constraint: a grammar change is run through `./le cli grammar`
- [ ] the `le` script at the repository root - the launcher the owner named as the model
  → Constraint: what it gives this design: one directory per build, a name grammar that makes the newest sortable, a dot-prefixed staging directory renamed into place so a half-written directory is never a build, selection by an environment value (`ZE_LE_BUILD_NAME`) that it exports to the process it starts and to that process's children, and a Go copy of the name grammar held equal to the shell copy by a test (`TestLauncherAgreesWithTheResolver`)
  → Constraint: it answers one word itself, `rollback`, before any build is chosen: `rollback_words` reads the words after it, and the launcher moves the newest build aside. That is the precedent for "the ze script providing an uninstall option a bit like we do with ./le"
  → Constraint: its build machinery (the lock, the background build, the smoke test at each first run, setting a failed build aside) has no counterpart in a router's launcher, which runs at every command and at every service start
- [ ] `docs/architecture/cli/plugin-modes.md`, "Native installation commands"; `docs/guide/ze-install.md` - `ze install`, `ze uninstall`, the unit and their refusals as they stand
  → Constraint: the local plugin reaches the bootstrap and the service owner through the install registry (`cmd/ze/install/native.go`), without importing either plugin
  → Constraint: "Service installation requires root ownership of the executable and every existing ancestor." This design departs from that sentence for the version directories
- [ ] `docs/architecture/appliance/self-update.md`, `docs/architecture/config/system-update.md` - the self-updater and the backend registry
  → Constraint: pause, spread, the maintenance window, the restart policy and the history are the updater's, and each survives (Data Flow, "What each part of the self-updater becomes")
- [ ] `plan/release/fixes/spec-selfupdate-manifest-authenticity.md` (ready, P0) - constraints only
  → Constraint: a download happens only after a signed statement verified, the statement signs the size and the SHA-256 of what is downloaded, and a held file is hashed again before it is used. With version directories the signed object changes from one binary to an archive, which that spec must state (Work Not Done)
- [ ] `plan/release/delivery/spec-vpp-embedded-runtime.md` - the sibling spec
  → Constraint: Ze finds VPP in the directory of the `ze` that runs, and the bundle is one archive with a manifest, the same bytes for the appliance, the package and a version directory
  → Constraint: the bundle is built natively in a container, so it cannot be rebuilt byte for byte on two hosts. It enters the release build as an input pinned by its digest
- [ ] `plan/handover/64-vpp-ships-as-files-with-ze.md` - the questions put to the owner, his answers, and what was verified at the producers

**Key insights:**
- `ze install` and `ze uninstall` already exist (`internal/plugins/local`). The owner wants the install and uninstall functions in the launcher script. Two installers of the same files cannot both stay (`ai/rules/no-layering.md`), so how the work divides is choice Q-1
- With no version installed, no `ze` program exists to answer anything: the launcher is the only code that can speak, or act, in that state
- `ze version` ignores every word after it and answers 0, so `ze version list` prints the version line today
- The binary picks its personality from the last element of `argv[0]` and starts its plugin children from `os.Executable`, so the launcher must replace itself with the real `ze` by an absolute path whose last element is `ze`
- A started plugin's PATH holds the inherited PATH first, so a child that runs `ze` by name reaches the launcher. The launcher therefore exports its choice, or a child would run a release newer than its parent
- A development build made by `./le` stamps today's date in the release form (`gotoolchain.New`), so nothing in the stamp tells a development build from the release of the same day: choice Q-9
- The release build must not use `gotoolchain.New()`'s clock: version comes from the tag, build date from the tagged commit's committer time, or the CI rebuild never matches
- CI cannot see a draft release with a read-only token; the comparison runs in le against a hash list CI publishes
- Fedora 45 refuses unsigned RPMs, so RPMs must be signed, and the hash comparison for RPMs skips the signature header

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/go/toolchain/gotoolchain.go` - builds with `-X main.version` and `-X main.buildDate`; `New()` sets version to the local date as `06.01.02` and build date to the current second
  → Constraint: not reproducible; the release build needs a constructor that takes version and date explicitly
- [ ] `internal/le/repo/featuretags/daemontags.go` - `DaemonBuildTags(root, DaemonBase)` reads `feature-gates.txt`; `DaemonBase` is `ze_core ze_distro`
  → Constraint: the only source of the default tag set; the quickstart list is a derived copy
- [ ] `internal/le/build/compile/build.go` - the shared build runner behind `le build installer` and the ze-host build, through `gaterun.Run`
  → Constraint: extend this runner; do not write a second one
- [ ] `internal/core/version/version.go` - `IsValidRelease` accepts any 8-character string with dots at positions 2 and 5; `CompareReleases` compares strings
  → Constraint: `YY.MM.DD.N` fails it today, and a string compare sorts `26.11.28.10` below `26.11.28.9`
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

**Source files read for the versioned install (2026-11-28):**
- [ ] `internal/plugins/local/native.go` - `RunNativeInstall` and `RunNativeUninstall`, the bare `ze install` and `ze uninstall`. `selectNativeTarget` takes `--prefix` (default `/usr/local`), refuses `/usr` and `/`, and names the binary `<prefix>/bin/ze`. `nativeTarget.preflight` and `nativePath` refuse a symbolic link anywhere on the binary or config path. `install` stops the service through the registered service owner, copies the running binary under the store's maintenance lock, and starts the service again; a failure restores the previous binary. `uninstall` asks for confirmation unless `--yes`, stops and removes the service, takes the maintenance lock, removes the binary with `os.Remove` (the file it usually runs from), and keeps the config unless `--purge`
  → Constraint: `packageOwnership` looks for `<prefix>/share/ze/install-method` only when the binary's directory is named `bin`, and nothing in the tree writes that marker
- [ ] `internal/plugins/local/cmd_install.go`, `cmd_uninstall.go`, `register.go` - `ze install local` and `ze uninstall local` copy or remove the one binary with no service and no initialization; `copyInstalledBytes` writes a temporary file beside the target and renames it; the plugin registers its two defaults and two targets with the install and uninstall dispatchers of `cmd/ze`
- [ ] `internal/plugins/systemd/cmd_install.go`, `main.go` `resolveBinaryPath`, `native_trust.go`, `unit.go` - `ze install systemd` resolves the running executable through links and writes it into `ExecStart`; `trustedExecutable` and `trustedNode` refuse a path with a symbolic link, an owner other than root, or group or other write, on the executable and every existing ancestor; `buildUnitFile` writes `User=ze`, `ProtectSystem=true`, `NoNewPrivileges=true` and the capability set of `privilege.DaemonCapabilities`
- [ ] `internal/core/privilege/capabilities.go` - `daemonCapabilities`: `CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_NET_BIND_SERVICE`, `CAP_BPF`. The unit grants exactly these, ambient and bounding
- [ ] `internal/core/paths/paths.go` - `classifyBinary` knows a `bin` or `sbin` parent and gokrazy's `/user`. A `ze` in any other directory has no config directory and no data directory
- [ ] `cmd/ze/ze_core_dispatch.go` - the `version` root handler prints the version, reads only `--extended`, ignores every other word and returns 0
- [ ] `cmd/ze/dispatch.go`, `cmd/ze/le_build_name.go` - `binaryName` reads the last element of `argv[0]`; `ze.le.build.name` is the precedent for a key a launcher exports to the process it starts and to that process's children
- [ ] `internal/component/plugin/childenv.go` - `ChildPathEnv` gives a started plugin the inherited PATH first, then the engine's own directory
- [ ] `internal/component/config/system/selfupdate.go` - `resolveTarget` resolves the running executable; `downloadBinary` writes a temporary file in the executable's own directory; `stageBinary` probes for write access (answering "self-update not supported on read-only filesystem"), hard-links the binary to `.prev` and renames the new file over it; `Rollback` renames `.prev` back; `defaultRestart` re-executes the target path with the old arguments and environment
- [ ] `internal/component/config/storage/maintenance_linux.go` - `AcquireMaintenance` takes the store's exclusive lifetime lock, the one a running daemon holds, and answers `ErrBusy` while a daemon owns the store. `ze install` and `ze uninstall` already use it
- [ ] `internal/component/host/inventory.go` - `DetectPlatform` and `PlatformGokrazy`, the one platform detection
- [ ] `internal/component/vpp/dpdk.go` - `BindAll` loads the vfio kernel modules and rebinds each configured NIC through sysfs before VPP starts
  → Constraint: neither the component nor `docs/guide/vpp.md` states which user starts VPP. The unit's capability set holds no capability that loads a module or writes a root-owned sysfs file. Whether a `ze` that is not root can start VPP is not shown by anything read here (assumption A-9)

**Facts taken from handover 64 and from the earlier draft of this section, not re-read on 2026-11-28:** every row of "Source files read for the versioned install" above. Read again at the producer on 2026-11-28, because a later decision depends on it: the `le` script's `rollback` handling, the development stamp in `gotoolchain.New` (already the first row of this section), `ensureServiceAccount` and `createUser` in `internal/plugins/systemd/cmd_install.go` (they create the `ze` group and user with `groupadd` and `useradd`, or `addgroup` and `adduser`), and the absence of any existing design-decision row on a four-digit year, install.sh or root ownership.

**Behavior to preserve:**
- `ze install systemd` keeps creating the `ze` account and writing the unit
- Non-release builds (`./le` builds, `go install`) keep stamping a version and date, in the form choice Q-9 settles
- What `ze install` does around the files: the refusals before any change, the first-time initialization, the service stop and start, the configuration and credentials kept
- The complete removal keeps asking for confirmation, removing the service, and keeping the configuration unless `--purge`. It no longer stops the service: it refuses while the service runs and names `sudo systemctl stop ze.service` (L-5)
- Pause, spread, the maintenance window, the restart policy and the update history of the self-updater
- The appliance: `gokrazy-ab` stays the only way an appliance takes new software
- `ze version` and `ze version --extended` print what they print today

**Behavior to change:**
- The release form gains the same-day counter: `YY.MM.DD` or `YY.MM.DD.N`, parsed strictly and compared numerically. The year stays two digits
- Config directory resolution learns the Homebrew Cellar layout
- Self-update refuses, with a message naming the package manager, when Ze was installed by deb, rpm or Homebrew
- An installation is a launcher at `<prefix>/bin/ze` and one directory per release under `<prefix>/lib/ze`. The flat layout, a real `ze` at `<prefix>/bin/ze`, is deleted, not kept beside the new one
- The unit's `ExecStart` runs the launcher, and on a `/usr/local` install the unit lets the service write the version directories
- Config and data directory resolution, and the package marker lookup, learn the version directory
- The self-updater downloads a release archive and installs a version directory. The in-place replacement, the `.prev` hard link and the rollback that renamed it back are deleted
- The trust rule for the service's executable accepts a version directory the `ze` user owns (the owners: choice Q-4)
- `ze version` followed by a word it does not know is refused. Today it prints the version and answers 0
- What `ze install`, `ze install local`, `ze uninstall` and `ze uninstall local` become follows Q-1, taken as (a). Their copy of the running binary to `<prefix>/bin/ze` goes in every answer, because that path holds the launcher

## Existing Mechanisms (MANDATORY)

Rows other than the news fragment rows restate this spec's Current Behavior
section above; the symbols were confirmed present on 2026-11-09, and the
behavior each row states is that section's reading, owed a re-read at the
producer when its phase starts.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Go toolchain environment | `internal/le/go/toolchain/gotoolchain.go` `New` | Stamps version and build date through `-X`, from the local clock | Extends: a constructor taking version and date explicitly, so a release build is reproducible |
| Daemon build tags | `internal/le/repo/featuretags/daemontags.go` `DaemonBuildTags` | Derives the daemon `-tags` from `feature-gates.txt` | Extends: the release build reads its tags here, no second list |
| Shared build runner | `internal/le/build/compile/build.go` | Runs the ze-host and installer builds | Extends: the release targets run through it, no second runner |
| Release version parser | `internal/core/version/version.go` `IsValidRelease`, `CompareReleases` | Accepts the two-digit CalVer form by length and compares strings | Extends: `YY.MM.DD` and `YY.MM.DD.N`, parsed strictly and compared numerically |
| Config directory from binary | `internal/core/paths/paths.go` `ConfigDirFromBinary` | Maps `<prefix>/bin/ze` to `<prefix>/etc/ze` | Extends: the Homebrew Cellar layout |
| systemd unit builder | `internal/plugins/systemd/unit.go` `buildUnitFile` | Builds the unit `ze install systemd` writes | Extends: the package unit is generated from it, declared once |
| Update backend choice | `internal/component/config/system/backend.go` `NewBackend` | Picks gokrazy A/B or in-place self-update by platform | Extends: an install-method marker makes self-update refuse for packaged installs |
| Site build | `internal/le/site/build.go` `Build` | Stages `website/` into the Pages artifact | Extends: publishes the release public key. It no longer publishes an `install.sh` (owner, 2026-11-28) |
| News fragments | `website/changes/fragments/README.md`; parsed by `parseNewsFragment` (unexported until this spec's changelog needs it) in `internal/le/commit/newsfragment.go` (spec-weekly-news-fragments, closed) | One operator-facing file per closed change, validated at commit and kept unpublished | Extends: `CHANGELOG.md` is built from the fragments added since the previous release tag (AC-25), read through that one parser |
| Commit subjects as release notes | none: no producer exists | The earlier AC-25 planned a changelog grouped by subject prefix | Replaced before it was built: subjects are a second, weaker account of what fragments carry (owner, 2026-11-09) |
| Native install and removal | `internal/plugins/local/native.go` `RunNativeInstall`, `RunNativeUninstall`, `nativeRuntime.install`, `nativeRuntime.uninstall` | Installs the running binary at `<prefix>/bin/ze` with initialization and a service; removes it, keeping the configuration | Replaces the binary copy: that path now holds the launcher. Extends the rest: the initialization, the `ze` account and the service stay here, and the file work moves to the launcher's functions, which the owner placed in the launcher script (Q-1, taken as (a)). No second installer of the same files is kept |
| Binary-only install and removal | `internal/plugins/local/cmd_install.go` `cmdInstall`, `cmd_uninstall.go` `cmdUninstall` | Copies or removes the one binary, with no service | The same: its copy goes, and what remains follows Q-1 |
| Atomic placement | `internal/plugins/local/cmd_install.go` `copyInstalledBytes` | Writes a temporary file beside the target, then renames it | Extends: a version directory that Ze itself writes (a self-update) is written under a dot-prefixed staging name in the Ze home and renamed |
| Install and uninstall dispatch | `cmd/ze/install/dispatch.go`, `cmd/ze/uninstall/dispatch.go`; `internal/plugins/local/register.go` | A default and named targets, registered by the owning plugin | Extends: the listing is a command of the same plugin. The words the launcher answers itself (Q-1 and Q-8, taken as (a)) are words of these two roots, and bare `ze uninstall` is refused, naming both explicit forms |
| Bootstrap and service owners behind a registry | `cmd/ze/install/native.go` `NativeService`, `RegisterService`, `RegisterBootstrap` | The local plugin stops, installs and removes the service without importing the systemd plugin | Extends: the service owner also answers which executable the service's main process runs, which the removal of one version needs |
| The `ze` account | `internal/plugins/systemd/cmd_install.go` `ensureServiceAccount`, `createUser` | `ze install systemd` creates the `ze` group and user | Extends, under the recommendation of choice Q-3: the same step hands the Ze home to that user. No second place creates the account |
| Trusted service executable | `internal/plugins/systemd/native_trust.go` `trustedExecutable`, `trustedNode`; `internal/plugins/local/native.go` `nativePath` | Refuses a symbolic link, an owner other than root, and group or other write, on the executable and every existing ancestor | Replaces for the version directories: the `ze` user may own them (owner, 2026-11-28). Which owners are accepted, and on which paths, is choice Q-4. A departure (Design presentation) |
| Unit builder, executable and protection | `internal/plugins/systemd/unit.go` `buildUnitFile`, `unitSpec`; `main.go` `resolveBinaryPath` | Writes `ExecStart` from the resolved running executable, with `User=ze` and `ProtectSystem=true` | Extends: `ExecStart` names the launcher (owner, 2026-11-28), and the unit of a `/usr/local` install keeps the Ze home writable for the service |
| Paths from the binary | `internal/core/paths/paths.go` `classifyBinary` | Knows `bin`, `sbin` and gokrazy's `/user` | Extends: a `ze` in a version directory is classified as the launcher beside it would be, so the config and data directories do not move |
| Package marker lookup | `internal/plugins/local/native.go` `packageOwnership`, `packageStateOwnership` | Finds `<prefix>/share/ze/install-method` from a binary in a `bin` directory | Extends: the prefix comes from the same classification, so a `ze` in a version directory finds its marker |
| Store lifetime lock | `internal/component/config/storage/maintenance_linux.go` `AcquireMaintenance` | Refuses while a daemon owns the store; used by `ze install` and `ze uninstall` | Extends: the removal of one version asks it whether a daemon runs. No pid file is added |
| Release order | `internal/core/version/version.go` `CompareReleases` | Orders two release strings | Extends: it is the one declaration of which installed release is the latest. The launcher's shell copy is held equal to it by a test, as `le`'s name grammar is |
| The `./le` launcher | the `le` script; `internal/le/go/toolchain/builds.go`; `docs/contributing/running-commands.md`, "Launcher builds do not overwrite running binaries" | One directory per build, staging and rename, selection by an exported environment value, one word (`rollback`) answered by the script itself, a Go copy of the shell grammar under an agreement test | Extends as a model, not as shared code: the `ze` launcher takes those ideas and none of the build machinery |
| The `ze` script at the repository root | the `ze` script | The development launcher of a checkout: it builds and runs `bin/ze` | Does not apply: it serves a source checkout and is not installed. The product launcher is a new file. Known from the `le` script's comment on their shared function, not read |
| A launcher-owned environment key | `cmd/ze/le_build_name.go` `envLeBuildName` | The launcher exports its choice to the process and its children | Extends: the `ze` launcher exports the release it chose under a key registered beside the code that reads it. Its name is choice Q-7 |
| The `version` root | `cmd/ze/ze_core_dispatch.go`, the `version` root handler; `show version` in `internal/component/cmd/show` | Prints the running version; ignores other words | Extends: the listing is a new command, spelled as choice Q-8 settles. The root refuses a word it does not know |
| In-place self-update | `internal/component/config/system/selfupdate.go` `stageBinary`, `Rollback`, `defaultRestart` | Replaces the running executable by rename, keeps `.prev`, re-executes the same path | Replaces: the download becomes a release archive, the stage becomes the installation of a version directory, the restart re-executes the launcher. `.prev` and its rollback are deleted |
| Update backend registry | `internal/component/config/system/backend.go` `registerBackend`, `NewBackend` | `ze-self-update` off gokrazy, `gokrazy-ab` on it | Extends: unchanged for the appliance, which is what keeps it immutable; the package-managed backend of the earlier design joins the same registry |
| Platform detection | `internal/component/host/inventory.go` `DetectPlatform`, `PlatformGokrazy` | Answers the platform once | Extends: the version commands ask it to refuse on the appliance. No second detection |
| Commands answered in the caller's own process | `internal/component/command` `MustRegisterLocalData`, `MustRegisterLocal`, `RegisterShape`, `RegisterColumns` (`ai/patterns/cli-command.md`); `ai/doors/commands-cli.md`, "Where it sits" | A local-data command is answered in the caller's process and by the daemon through the same handler. An answer about the daemon's host is a plain local command with a builtin RPC on the same path, as `show version` is | Extends: the version listing is one more. It answers about the host it runs on, so it takes the second form, with its rows as data |
| Cached, keyed build artifacts | `docs/architecture/appliance/build-artifacts.md`; the sibling spec's `./le build vpp bundle` | Builds the VPP bundle once per input key | Extends: the release build reads the bundle by digest and never builds it |
| Signed update statements | `plan/release/fixes/spec-selfupdate-manifest-authenticity.md`, "The signed statement" | Designed: an SSHSIG statement signs the size and SHA-256 of the binary a device downloads | Extends: the signed object becomes the release archive. That spec owns the change (Work Not Done) |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | the door every change reads | "A doctor check registered by the package that owns the dependency" | agrees | The two doctor checks are registered by the package that owns the marker and the Ze home |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/version` and `internal/core/paths` | "It imports neither `internal/component/` nor `internal/plugins/`." | agrees | The layout package is a core leaf that the local plugin and the update backend both import |
| `ai/doors/config-yang.md` | Boundaries | the door covering the config stamp files | "**A tunable defaults to a YANG leaf, and env-only is the exception.**" | agrees | The version variable is read by the launcher before any `ze` or config exists, so it cannot be a leaf. No config leaf is added |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/le` and the `.ci` files | "**The rule is directional.** A shipped `ze` never links an `internal/le`" | agrees | The layout and the launcher live in `internal/core`, which the release tooling imports |
| `ai/doors/hub-engine.md` | Where it sits | the door covering `cmd/ze` | "**`cmd/ze` is one program with several personalities.**" | agrees | The launcher starts `ze` by a path whose last element is `ze`, so the personality does not change |
| `ai/doors/appliance.md` | Boundaries | the door covering `internal/component/config/system` | "**A gokrazy device does not replace its own ze binary.**" | agrees | The appliance keeps `gokrazy-ab` and takes no version directory |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/local` and `internal/plugins/systemd` | "**A plugin owns its whole surface.**" | agrees | The listing and the check that no daemon uses a version are the local plugin's, and it reaches the service owner through the install registry |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | the `// Design:` page of the local and systemd plugins; a page this spec modifies | "Service installation requires root ownership of the executable and every existing ancestor." | departs | The owner decided that the `ze` user installs versions, so a version directory that user owns is accepted (the owners are Q-4's, taken as (a), AC-49) |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | the `// Design:` page of the local and systemd plugins; a page this spec modifies | "The bare `ze install` and `ze uninstall` commands dispatch to defaults registered by the local plugin." | departs | Bare `ze uninstall` is refused and names `ze uninstall version <release>` and `ze uninstall all`, because the complete removal needs an explicit word (Q-8, taken as (a)) |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | the `// Design:` page of the local and systemd plugins; a page this spec modifies | "A failed replacement restores the prior binary and resumes a service that was previously active." | departs | The section also says it refuses "before copying the executable". The Go `ze install` copies no executable and replaces no binary: a version reaches the host by extracting the tarball or through the launcher's install function, and the Go command keeps the first-time initialization, the `ze` account and the service (Q-1, taken as (a), AC-52) |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | the `// Design:` page of the local and systemd plugins; a page this spec modifies | "Native removal first asks the service owner to stop the selected service." | departs | The complete removal refuses while the service runs and names `sudo systemctl stop ze.service`; it never stops the service itself (owner, 2026-11-28, L-5; AC-43) |
| `docs/architecture/appliance/self-update.md` | Decisions | the `// Design:` page of `selfupdate.go`; a page this spec modifies | "**Replacement is a hard link then a rename.**" | departs | An update installs a version directory beside the running one. No file is replaced and no `.prev` is kept |
| `docs/architecture/config/system-update.md` | The version check | the `// Design:` page of `backend.go`; a page this spec modifies | "**Version comparison is lexicographic, not semver.**" | departs | With the same-day counter the comparison is numeric, part by part |
| `docs/architecture/config/system-update.md` | The version check | the `// Design:` page of `backend.go`; a page this spec modifies | "`ze update-serve` is the build-infrastructure side: it serves `/version.json` with arch validation and `/<goos>/<goarch>` for the binary." | departs | `<base>/<goos>/<goarch>` serves the release archive for that system, which the update unpacks into a version directory, not one binary. `spec-selfupdate-manifest-authenticity` owns that change (Work Not Done) |
| `docs/config-reference.md` | Firmware Update Check | a page this spec modifies | "(lexicographic comparison, matching ze's `YY.MM.DD` date-based versioning)." | departs | The same: the comparison becomes numeric, and the form gains `.N` |
| `docs/architecture/config/syntax.md` | Basic Syntax Patterns | the `// Design:` page of `stamp.go` and `evolution.go`; a page this spec modifies | "release that wrote them (`YY.MM.DD`):" | agrees | The stamp keeps the two-digit form. The page gains the counter a stamp can carry |
| `docs/architecture/core-design.md` | 20. System Update Backend | the `// Design:` page of the toolchain and build files; anchored to `backend.go` and `selfupdate.go` | "The system update surface is a single registered backend interface in `internal/component/config/system`." | agrees | The package-managed backend joins that registry, and the self-update backend keeps its name |
| `docs/architecture/system-architecture.md` | Capabilities without root | the `// Design:` page of `cmd/ze/main.go`, `dispatch.go` and `ze_core_dispatch.go` | "`privilege.DaemonCapabilities` is the one declaration of the capabilities a non-root daemon runs with" | agrees | Ze runs as the `ze` user with that set. Whether its VPP starts under it is A-9 |
| `docs/architecture/api/commands.md` | Local-data commands (answered in the caller's own process, and by the daemon) | anchored to `internal/core/version/version.go` | | silent | The anchor is about `show version`. How the new listing is served follows that page and is not changed by it |
| `docs/architecture/diagnostics/production-diagnostics.md` | Single-call live triage (`show system triage`) | anchored to `internal/core/version/version.go` | | silent | The triage reads `version.BuildIdentity`, which keeps its fields |
| `docs/architecture/web-interface.md` | Security Headers | anchored to `internal/core/version/version.go` and `internal/core/paths/paths.go` | | silent | The web headers and the backup workspace read nothing this design changes |
| `docs/architecture/testing/qemu-integration.md` | A run boots ze's kernel when it is given one | anchored to `gotoolchain.go` | | silent | The page is about the QEMU harness, not about the stamp |
| `docs/architecture/testing/verify-freshness-scope.md` | One change-set selection | anchored to `gotoolchain.go` | | silent | The page is about what a change set reaches |
| `docs/architecture/testing/ci-format.md` | Options | the `// Design:` page of `internal/test/runner/runner.go` | | silent | Only the runner's own stamp can change, under choice Q-9. The `.ci` format does not |
| `docs/architecture/config/environment.md` | Top-Level Variables | anchored to `runner.go` and `paths.go` | | silent | Not read for a statement on a launcher variable. The phase that registers the variable reads this page first and adds the key to it |
| `docs/architecture/testing/runner-architecture.md` | `.ci` Runner wrapper | anchored to `runner.go` and `compile/build.go` | | silent | The runner's scheduling is not changed |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `cmd/ze/main.go` | | silent | No registration field changes |
| `docs/architecture/cli/tui-launcher.md` | Decisions | anchored to `cmd/ze/ze_core_dispatch.go` | | silent | The page is about what bare `ze` shows at a terminal. The `version` root's refusal does not touch it |
| `docs/architecture/config/transaction-protocol.md` | 5. Plugin Load/Unload During Reload | anchored to `cmd/ze/ze_core_dispatch.go` | | silent | No config transaction changes |
| `docs/architecture/fleet-config.md` | Client Side | anchored to `cmd/ze/ze_core_dispatch.go` | | silent | The managed client is not changed |
| `docs/architecture/container/image-store.md` | Where it is today | anchored to `internal/core/paths/paths.go` | | silent | The image store reads the data directory, which keeps its place for a `ze` in a version directory (AC-45) |
| `docs/architecture/api/process-protocol.md` | Shutdown | anchored to `internal/plugins/systemd/unit.go` | | silent | The unit's stop behavior is not changed, only the program it starts |
| `docs/architecture/behavior/signals.md` | Startup Paths | anchored to `internal/plugins/systemd/unit.go` | | silent | The same: signals and the start path are not changed |
| `website/AI.md` | Layout | the `// Design:` page of `internal/le/site/build.go` | | silent | The site build stages one more file from `website/`, the release key. It no longer needs a producer for `install.sh` |
| `docs/architecture/testing/ci-workflows.md` | What pins the workflow set | anchored to `workflowcheck_test.go` | | silent | The new workflow is pinned by the same test, as the page says every workflow is |
| `docs/features/ai-first.md` | Structured Diagnostics | the `// Design:` page of the doctor file | | silent | Two codes are added in the form the page describes |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The listing's path names the subject and stops, and its rendering is the pipe layer's |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The listing overrides none |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | The listing answers rows as data, and no pipe changes what it does |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Untouched: no config verdict changes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | The launcher refuses only when it has no version to run or the variable names none. Once a `ze` runs, nothing of this design ends `ze start` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Untouched |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | A plugin started in query mode reaches `ze` through the launcher with the exported version, so it is the release of its parent |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Untouched |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Untouched |
| `docs/architecture/config/system-update.md` | Design decisions | decision of 2026-11-28 | "A Ze release is named by calendar version with a two-digit year, YY.MM.DD, and a day counter is added to it when a day has more than one release (YY.MM.DD.N). Go's module version rules do not constrain the name. In Thomas's words (voice-transcribed): "I thought we had settled on a Calver but adding effectively the day counter", "We want only two characters for the year."; on Go's module version rules: "Ze is not a library, it does not matter." This reverses the four-digit year, YYYY.MM.DD, that he chose on 2026-09-28" | agrees | AC-3, AC-4 and AC-5 are that form. A version directory carries it |
| `docs/architecture/appliance/self-update.md` | Design decisions | decision of 2026-11-28 | "The automatic update is for Linux only. The appliance is immutable infrastructure. In Thomas's words (voice-transcribed): "The automatic update should only be for Linux, for the appliance it should be immutable infrastructure."" | agrees | The appliance keeps `gokrazy-ab` and installs nothing (AC-48) |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "An install of Ze that is not a package lives under /usr/local, as the Filesystem Hierarchy Standard says, and several versions are installed side by side. The real ze program lives inside the Ze folder of its version, with its VPP beside it, and the entry in the bin directory is a launcher script, never a symbolic link. A DEB or RPM install has the same layout: the launcher at /usr/bin/ze, and /usr/lib/ze/<release>/ze with /usr/lib/ze/<release>/vpp/ beside it. In Thomas's words (voice-transcribed): "install following the FSB on a linux FS (probably /usr/local/) and then add a script in /usr/local/bin so we can have multiple version installed and change based on a env variable", "same for vpp install it in the same folder as ze and make sure ze will scan that folder"; asked where the Ze folder is on a package install: "Neither, like B, but the /usr/bin/ze is a script, not a symlink."; asked whether a package install uses version directories too, (a) one layout and one launcher everywhere or (b) a second, simpler layout for packages: "a"" | agrees | Data Flow, "The installed layout" |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "The launcher runs the latest installed version, unless an environment variable set in the shell that runs it names another of the installed versions. Nothing else records a selected version, neither a file nor a link. In Thomas's words (voice-transcribed), asked whether the selected version is recorded in a file or in a link: "It should be the latest one, unless an environment value is set in the shell running it setting another version in the list of valid options." This replaced his earlier answer, a current marker" | agrees | Data Flow, "The launcher"; AC-35 to AC-38. The variable's name is choice Q-7 |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "The systemd unit that ze install systemd writes runs the launcher. In Thomas's words (voice-transcribed), asked what the unit executes, (A) the launcher or (B) the real program of one version: "A"" | agrees | AC-44 |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "Old versions are kept: installing a version removes no other. The ze script provides the uninstall, as ./le does. Removing one version removes its whole directory, ze and its VPP together. The removal refuses only a version that a running daemon is using, so a bad latest release is rolled back by stopping the service, removing that release and starting the service. One explicit command removes a /usr/local Ze completely, every version directory, the launcher and the service unit, and leaves /etc/ze alone. A package install is removed by its package manager. In Thomas's words (voice-transcribed): "old version are kept the ze script providing an uninstall option a bit like we do with ./le"; "lgtm" to a listing of the installed versions and to a removal of one version with its directory; asked how a bad latest release is rolled back, (A) removal refuses only a version a running daemon uses, and the operator stops the service, removes the bad release and starts the service, or (B) removal refuses the latest and the operator pins the old one through the variable: "A"; asked how a /usr/local Ze is removed completely, (A) one explicit command removes every version directory, the launcher and the service unit, and leaves /etc/ze alone, (B) no command, or (C) as A and also deletes the configuration: "a". The answer "A" on rollback replaced his earlier answer that the removal also refuses the selected version" | agrees | AC-40, AC-42 and AC-43; the complete removal refuses while the service runs and names `sudo systemctl stop ze.service`. The script's uninstall function does the file work after the installed `ze` made the checks (Q-1, taken as (a), AC-52) |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "The version commands take the spelling the CLI grammar requires (ai/rules/cli.md), with no exception for "ze version list" and "ze version uninstall". In Thomas's words (voice-transcribed), asked whether the commands take the grammar's spelling (A) or keep "ze version list" and "ze version uninstall <version>" as an exception (B): "a"" | agrees | The spellings are put to him as choice Q-8 and run through `./le cli grammar` |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "There is no install.sh. The Linux tarball holds its files under the paths where they are installed, so extracting it puts every file in the right location, and the ze launcher script carries the install and uninstall functions itself. In Thomas's words (voice-transcribed): "I don't think we need an install.sh; we just need a tar with absolute path for installation, and people can get directly files in the right location.", "I don't want an install.sh , That way we have a script for ze so that can be built inside of it." This reverses install.sh as a release artifact, agreed on 2026-09-28" | agrees | Data Flow, "The Linux tarball"; AC-13, AC-31, AC-32 |
| `docs/architecture/cli/plugin-modes.md` | Design decisions | decision of 2026-11-28 | "Ze does not require root to run. It runs as the ze user, provided the capabilities its programs need are available, and the version directories live where the ze user can write, so that user installs a version and an automatic update needs no operator. In Thomas's words (voice-transcribed), asked who installs a self-update when Ze runs as its service, (A) the daemon downloads and verifies and root installs with one command, (B) A plus a root-owned helper, or (C) version directories live where user ze can write, fully automatic: "C We do not require root users to run Ze. It could be run as a Ze user, provided the capability required to run the programs is available."" | agrees | Data Flow, "Who owns what Ze runs"; AC-23, AC-49; A-9 holds the condition he set |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Untouched |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The maintainer runs `./le build release` on a signed tag at a clean checkout
- An operator extracts a release tarball at the root of a Linux host, or installs the DEB or the RPM, and then types `ze`
- A running daemon with `system update-check { auto-apply true; }` finds a newer release

### Transformation Path
1. Resolve the tag: strict CalVer parse, tag signature, clean tree, commit time
2. Extract the tagged tree with `git archive` so nothing untracked enters the build
3. Build `ze` for every supported target with the daemon tag set, stamped version, commit and date
4. Assemble the archives, the deb and the rpm and the Homebrew formula from the built binaries, the launcher and, for Linux, the pinned VPP bundle
5. Write `checksums.txt`; GPG signs it and the RPMs on the maintainer's hardware key
6. Create a draft GitHub release on the tag and upload every artifact
7. CI, triggered by the tag, rebuilds steps 2 to 4 with read-only access and uploads its hash list as a workflow artifact
8. le downloads CI's hash list, compares every artifact (RPMs past the signature header), and publishes the draft only on a full match

### The installed layout

One shape for every install on plain Linux (owner, 2026-11-28). `<prefix>` is `/usr/local` for the tarball and `/usr` for the DEB and the RPM.

| Path | What it is |
|------|------------|
| `<prefix>/bin/ze` | The launcher, a POSIX `sh` script. A regular file, never a link |
| `<prefix>/lib/ze/` | The Ze home. It holds the version directories |
| `<prefix>/lib/ze/<release>/` | One version directory, named by the release string, for example `26.11.28` or `26.11.28.1` |
| `<prefix>/lib/ze/<release>/ze` | The real program |
| `<prefix>/lib/ze/<release>/vpp/` | Ze's VPP bundle, as the sibling spec builds it |
| `<prefix>/lib/ze/<release>/LICENSE`, `README.md` | Ze's licence and a pointer to the documentation |
| `<prefix>/share/ze/install-method` | The package marker of this spec's earlier design (`deb`, `rpm`). Absent on a `/usr/local` install |

The Ze folder of the sibling spec, the directory that holds the programs Ze runs, is the version directory: the directory of the `ze` that runs. That is the owner's "same folder as ze", read literally.

No file records which version is selected. The answer is computed each time, from the directory names.

A name in the Ze home that starts with a dot is never a version. A version directory that Ze itself writes (a self-update) is written under such a name and renamed when it is whole, as `le` stages a build.

Who owns the Ze home and the version directories is settled by the owner's decision that the `ze` user installs versions, and by choices Q-3 and Q-4. In a package every file is the package manager's, owned by root.

The appliance has none of this. gokrazy places `ze` at `/user/ze` and starts it; the image build places the bundle at the Ze folder the image names (sibling spec). No launcher, no Ze home.

### The Linux tarball

The tarball's member paths are the final locations (owner, 2026-11-28).

| Member | Mode |
|--------|------|
| `usr/local/bin/ze` | 0755, the launcher |
| `usr/local/lib/ze/<release>/ze` | 0755 |
| `usr/local/lib/ze/<release>/vpp/` and every file of the bundle | The modes of the bundle's manifest |
| `usr/local/lib/ze/<release>/LICENSE`, `README.md` | 0644 |

His words are "a tar with absolute path". The member names carry no leading slash: `tar` removes one when it extracts and prints a warning, and an archive made without it extracts to the same places with no warning. The operator extracts at the root, so every file lands at its final path: `sudo tar -C / -xzf ze_26.11.28_linux_amd64.tar.gz`. After that command `ze` runs, with nothing moved.

What follows from this shape:

| Consequence | Handled by |
|-------------|------------|
| Extracting a second release adds its directory and writes that release's launcher over `/usr/local/bin/ze`. Extracting an older release puts an older launcher back | The launcher's contract with a version directory is kept small (a directory name, a file named `ze`, one variable), and a test runs every later launcher against a version directory of the first release (AC-41) |
| The extracted files belong to whoever extracts, which is root. The owner decided that the `ze` user installs versions | Choice Q-3 |
| Nothing checks the archive's signature or hash when an operator extracts it by hand | Choice Q-2 |
| `tar` writes each file in place, so for the seconds an extraction lasts, a version directory can exist half written | Risk R-13 |
| The launcher finds the Ze home from its own path, so an archive extracted under another root (`tar -C /srv/ze-root`) works from there | No `--prefix` is needed for the files |

### The launcher

The launcher is one file, the same bytes in every artifact of a release. It is declared once in the repository, and the release build writes that file into the tarball, the DEB and the RPM.

What it does when it starts `ze`, in order:

1. It finds the Ze home: the `lib/ze` directory beside the directory that holds the launcher, from its own path, as `le` finds its root.
2. It lists the versions. A version is an entry of the Ze home that is a real directory, not a link, whose name is a release string (`YY.MM.DD` or `YY.MM.DD.N`), and that holds a regular, executable `ze` that is not a link. Any other entry is ignored.
3. When the version variable (its name is choice Q-7) is set and not empty, it must name one of those versions. Otherwise the launcher refuses and prints the installed versions.
4. When the variable is not set, it takes the latest version: the year, the month, the day and the counter compared as numbers, a release with no counter before the same date with one.
5. It exports the variable with the version it chose, and replaces itself with that version's `ze`, by its absolute path, passing every argument unchanged.

The launcher also carries the install and uninstall functions (owner, 2026-11-28). The `./le` script is the model: it answers `rollback` itself, before it chooses a build. What the `ze` launcher's functions take and do, and what is left to the `ze` program, is Q-1, taken as (a): the launcher does the file work and asks the installed `ze` for what only the program holds (AC-52). One fact holds under every answer: with no version installed there is no `ze` program, so only the launcher can answer in that state.

The launcher checks no owner and no caller identity. Root runs the latest version like every caller, a version the `ze` user installed included (N-6, taken as (b)), because the owner's rows of 2026-11-28 forbid "A version chosen by anything other than version order and that variable" and "Refusing a version directory only because the ze user, and not root, owns it". The risk this leaves is stated in "Who owns what Ze runs" below and in `docs/guide/ze-install.md`. Which owners the service's trust check accepts is Q-4, a separate guard.

| Launcher case | What the operator sees (stderr, exit 2) |
|---------------|------------------------------------------|
| The variable names `26.10.01`, which is not installed | That the variable names 26.10.01, that it is not installed in `/usr/local/lib/ze`, the installed versions, and on the last line the command that works: the same command with the variable unset |
| No version in the Ze home | That no Ze version is installed in `/usr/local/lib/ze`, and the command that installs one: the tarball extraction line for a `/usr/local` install, or the line that installs the package again |

The order of releases is declared once, in `internal/core/version`. The launcher cannot call it, because the launcher is what finds the program that holds it, so it carries a shell copy of the name grammar and of the order. A test gives both the same sets of directory names and requires the same choice, the way `TestLauncherAgreesWithTheResolver` holds `le` to its Go resolver.

The launcher exports its choice so that a child reaching `ze` through PATH (an external plugin whose `run` line names `ze`) runs the release its parent runs, and not a newer one installed since. The updater removes the variable from the environment before it re-executes the launcher, so a restarted daemon runs the latest version.

### Who owns what Ze runs (a departure)

Today every program the service runs, and every directory above it, is owned by root, with no group or other write and no symbolic link (`trustedExecutable`). The owner decided on 2026-11-28 that Ze does not require root and that the `ze` user installs versions. Decided and open, kept apart:

| Point | State |
|-------|-------|
| The version directories live where the `ze` user can write, and that user installs one | Decided |
| The unit of a `/usr/local` install keeps the Ze home writable for the service. `ProtectSystem=true` keeps the rest of `/usr` read-only | A consequence of the decision. The package's unit carries no such line, because a package install does not update itself |
| Who creates the `ze` account and hands the Ze home to it | Choice Q-3 |
| Which owners the trust check accepts, and on which paths | Choice Q-4 |
| Whether root runs a version that the `ze` user installed | Yes: root runs the latest like every caller (N-6, taken as (b)), and the guide states the risk |
| How the launcher at `<prefix>/bin/ze`, which the `ze` user cannot write, follows a self-update | Choice Q-6 |

What the decision costs, stated plainly: a daemon that is compromised can replace the programs it runs, and it already holds every capability the unit grants those programs. A version it writes is what the launcher runs at the next start. Because root runs the latest version like every caller, it is also what the next `sudo ze` runs: a compromised daemon chooses the program root runs, an escalation from the `ze` user to root. This is accepted (N-6 (b)), and `docs/guide/ze-install.md` states it.

### What a non-root Ze needs to run its programs

His condition is "provided the capability required to run the programs is available". The unit grants `CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_NET_BIND_SERVICE` and `CAP_BPF`, ambient and bounding, through `privilege.DaemonCapabilities`. That set is declared for Ze's own work. VPP, started by Ze, inherits it. Before VPP starts, the VPP component loads kernel modules and rebinds NICs through sysfs; VPP itself needs hugepages, locked memory, the vfio devices and its socket directory. None of the four capabilities permits loading a module or writing a root-owned sysfs file. Nothing read for this spec shows a Ze that is not root starting VPP. That is assumption A-9 and risk R-17: until it is proved, "Ze runs as user `ze`" is shown for Ze and not for its VPP.

### Commands

The spellings are Q-8's, taken as (a) on the design's recommendation. Which code serves each row is Q-1's, taken as (a): the launcher's uninstall function removes the files after the installed `ze` made the checks and did the service work.

| What the operator wants | Recommended spelling | Behavior (decided unless marked) |
|-------------------------|----------------------|----------------------------------|
| The installed versions and which one runs | `ze show version list` | One row per version, as data, rendered by the pipe layer. On a package install and on the appliance it shows one row |
| Use another installed version for one command or one shell | The version variable in front of the command | The launcher. No command selects a version, and nothing is stored |
| Remove one version | `ze uninstall version <release>` | Removes the whole directory, `ze` and its VPP together. Refuses only a version a running daemon is using |
| Go back from a bad latest release | No command | Stop the service, remove that release, start the service |
| Remove a `/usr/local` Ze completely | `ze uninstall all` | Every version directory, the launcher and the service unit. `/etc/ze` is left alone. Refuses, changing nothing, while the service runs, and names `sudo systemctl stop ze.service` |
| Install a version from a tarball | `ze install release <tarball>` | The launcher's install function: the installed `ze` verifies the release first, then the version is put in place; with no `ze` installed it only extracts (Q-1 (a), Q-2 (c); AC-33, AC-52). The tarball extraction also installs one, with no command |

The listing answers these fields for each version: the release, whether it is the latest, whether this command would run it, whether a daemon runs it (yes, no or unknown, never a guess), the VPP it carries or `none`, its owner and its path.

The removal of one version refuses, changing nothing, in these cases. Each refusal says what is wrong and why, and ends with the command that works.

| Condition | The refusal says | The command on its last line |
|-----------|------------------|------------------------------|
| No such version | The name given and the installed versions | The listing command |
| No version named | That the command needs a version, and the form | The removal command with an installed release filled in |
| A daemon runs from that version | That the Ze service runs 26.11.28, that removing it under a running daemon breaks its VPP and its plugins, and that the next two commands are the removal and `sudo systemctl start ze.service` | `sudo systemctl stop ze.service` |
| A daemon runs and Ze cannot tell which version it runs (a daemon started by hand) | That a Ze daemon owns `/etc/ze` and its version cannot be read | The command that stops it |
| A DEB or RPM install | That the package manager owns the files | `sudo apt remove ze` or `sudo dnf remove ze` |
| The appliance | That the image holds one version and is replaced whole | `ze appliance push <name>`, run on the build host |
| The caller may not write the directory | Who owns it | The same command under `sudo` |

Removing the only installed version is not refused by his rule, which refuses only a version a running daemon uses. After it the launcher answers the "no version" case above.

How the command knows that a daemon runs from a version, from mechanisms that exist: the store's lifetime lock says whether a daemon of this installation runs (`AcquireMaintenance`, which `ze uninstall` already takes), and the registered service owner says which executable the service's main process runs. With no lock holder, no daemon runs and any version can go. With a lock holder and a service owner that answers, only that version is refused. With a lock holder and no answer, every removal is refused and says why. No pid file is added. A removal renames the directory to a dot-prefixed name first, then deletes it, so a half-removed directory is never a version.

The complete removal refuses, changing nothing, while the service runs (owner, 2026-11-28, L-5: the option he chose includes that it "refuses while the service runs"). The refusal says that the Ze service runs and that the complete removal does not stop it, and its last line is `sudo systemctl stop ze.service`. A daemon started by hand holds the store's lifetime lock the same way, and is refused with the command that stops it, as for the removal of one version. With no daemon running it keeps what `ze uninstall` does today around the files: the confirmation, removing the service, the maintenance lock, and `/etc/ze` kept unless the operator passes `--purge`. Today's command stops the service itself; this design deletes that step from the complete removal, a departure from `docs/architecture/cli/plugin-modes.md` (Design documents).

`ze version` keeps printing the version. A word it does not know is refused with exit 1 and the command the operator may have meant.

### Config, data, marker and unit, once `ze` sits in a version directory

| Reader | Today | After |
|--------|-------|-------|
| `classifyBinary` (`ConfigDirFromBinary`, `DataDirFromBinary`) | `<prefix>/bin/ze` only | `<prefix>/lib/ze/<release>/ze` is classified as `<prefix>/bin/ze`: `/usr/lib/ze/26.11.28/ze` and `/usr/local/lib/ze/26.11.28/ze` give `/etc/ze`, `/opt/ze/lib/ze/26.11.28/ze` gives `/opt/ze/etc/ze` |
| `packageOwnership` | The marker under the prefix of a `bin` directory | The prefix comes from the same classification |
| `resolveBinaryPath` of `ze install systemd` | The resolved executable | The launcher of the prefix the resolved executable belongs to. `ExecStart` runs `<prefix>/bin/ze start` |
| `trustedExecutable` | Root only, on one path | The launcher's path and the version directory's path, each under the rule choice Q-4 settles |
| `nativePath` of `ze install` | No link on the binary and config paths | No link on the launcher path, the Ze home and the config path |

A daemon keeps the version it started with. Installing a version changes nothing for a running process. The next start, by an operator, by the restart policy or by systemd after a failure, runs the latest version.

### What each part of the self-updater becomes

This applies to a `/usr/local` install on plain Linux. A DEB, RPM or Homebrew install refuses updates as AC-21 says, and the appliance keeps `gokrazy-ab`.

| Part today | With version directories |
|------------|--------------------------|
| The signed statement (`spec-selfupdate-manifest-authenticity`) | It signs the size and SHA-256 of the release archive for this OS, architecture and flavor, in place of one binary. Verified before any download, as that spec requires. That spec owns the change (Work Not Done) |
| Download into the binary's directory | The archive is downloaded to a dot-prefixed name in the Ze home, hashed, and kept there until the window opens. The held file is hashed again before use |
| Pause, spread, the maintenance window | Unchanged. They gate the installation of the version directory, as they gated the replacement |
| Stage: write probe, `.prev` hard link, rename | The version directory's members are unpacked into a dot-prefixed staging directory in the Ze home, the new `ze` is asked for its release, and the directory is renamed to that name. The unpack takes only the members under `usr/local/lib/ze/<release>/`, and refuses an entry outside that directory, a link that leaves it, a device, and a set-user or set-group mode. No file of a running version is touched. The archive's launcher member is not written by the daemon (choice Q-6) |
| Restart policy | Unchanged in what decides it. The restart re-executes the launcher of the prefix, with the version variable removed, so the process starts the latest version |
| `.prev` and `update system firmware rollback` | `.prev` is deleted with the in-place replacement. The command stays, because every backend answers it, and the self-update backend answers it with a refusal that gives the three commands of the decided path: stop the service, remove the release, start the service. A daemon cannot remove the directory it runs from |
| History | Unchanged: the same key, the same events, with the installed release |
| A home the daemon cannot write (root-owned files after a first extraction, a read-only file system) | The update stops after verification, the history records it, and `ze doctor` names the directory, its owner and the command that hands it to the service user |

### What the release artifacts carry

| Artifact | Content |
|----------|---------|
| Archive, Linux | The tarball above: the launcher and one version directory, at their final paths |
| Archive, other systems | Choice Q-10 |
| DEB and RPM | `/usr/bin/ze` (the launcher), `/usr/lib/ze/<T>/` with `ze`, `vpp/`, `LICENSE` and `README.md`, the unit, the marker, and the licence and copyright file under `/usr/share/doc/ze/` naming every bundled component. One package (owner) |
| VPP source | The source archive of every LGPL component of the bundle, published as release assets beside the packages and listed in `checksums.txt` |
| `checksums.txt` | Every asset, the source archives included |

The VPP bundle is never built by the release build. It is a native C build, so two hosts do not produce the same bytes, and this spec's publish gate compares bytes. The release build takes the bundle archive and its source archives as inputs, each named by a SHA-256 written in a pin file in the repository. The maintainer's build reads them from the keyed cache the sibling spec fills. The CI rebuild has no cache and no secret, so it needs those bytes from somewhere it can read: both builds fetch them by digest from the assets of a public GitHub release named for that digest (Q-12, taken as (a), AC-56).

On a package upgrade both version directories can exist for a moment: the package manager unpacks the new files before it removes the old ones. A daemon started inside that window may find the new directory's VPP files incomplete; the sibling spec's start check then keeps VPP down and reports it. So the package's post-install script restarts the service when it was running (and only then), after the unpack ended, and a daemon is never left running from a directory the upgrade removed. This order is what `dpkg` and `rpm` are documented to do and was not read in their sources (assumption A-8).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator's shell → launcher → `ze` | The launcher replaces itself with `<prefix>/lib/ze/<release>/ze`, having exported the version variable | No |
| Service user → Ze home | The unit keeps the Ze home writable; the owner rule of `trustedExecutable` (Q-4) | No |
| Update server → Ze home | A signed statement, then an archive checked against it, unpacked under a staging name | No |
| Release tarball → file system | `tar` at the root, by the operator. No Ze code runs | No |
| VPP bundle cache → release build | A pinned SHA-256 per architecture | No |
| Maintainer machine → GitHub | `gh release create --draft`, `gh release upload` under the maintainer's login | No |
| GitHub tag → CI | tag push triggers a `contents: read` workflow running `./le build release` | No |
| CI → maintainer | workflow artifact holding the hash list, fetched with `gh run download` | No |
| Release → user | direct download of the tarball, `.deb` or `.rpm`, and the Homebrew tap | No |

### Integration Points
- `internal/le/build/compile` - the shared build runner, extended for the target matrix
- `internal/le/go/toolchain` - version and date stamping, extended with explicit values
- `internal/le/repo/featuretags` - the daemon tag set
- `internal/core/version` - the release format and the release order the launcher copies
- `internal/core/paths` - config directory resolution from a version directory
- `internal/component/config/system` - update backend selection, and the updater that installs a version directory
- `internal/plugins/local` - the listing, the removal of one version, the complete removal
- `internal/plugins/systemd` - the unit text, the launcher in `ExecStart`, the trust rule
- `internal/le/site` and `website/` - publication of the release public key

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | N-A | No hot path |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A macOS host and a Linux CI runner produce byte-identical binaries, archives, debs and rpms | Go rebuild blog; nFPM and rpmpack source read, not run | The CI comparison never matches and nothing can publish | A two-host build of every target compared byte for byte, before the design is final | unvalidated |
| A-2 | The maintainer's hardware GPG key can sign both `checksums.txt` and RPMs from le | owner decision; nFPM `SignFn` accepts a callback | RPM signing needs a different path | Signing a test RPM through `gpg --detach-sign` on the hardware key | unvalidated |
| A-3 | Every stamp already written (config schema stamps, lab devices) uses the two-digit form `YY.MM.DD`, so the format change adds the counter and breaks no stored value | `IsValidRelease` accepts only that form today; `git tag` is empty | A stored value in another form is refused or misordered | `git tag`; a search of the repository's configs for `ze-schema:` stamps in the version phase | unvalidated |
| A-8 | `dpkg` and `rpm` unpack a package's new files before they remove the old version's files, and run the post-install script after the unpack | Their documented order, not read in their sources | A daemon restart can land between the two | The package smoke tests upgrade a running service and read which version it runs | unvalidated |
| A-9 | A `ze` that is not root, holding the unit's four capabilities, can start its VPP | His condition "provided the capability required to run the programs is available". Not shown by anything read: the VPP component loads kernel modules and rebinds NICs before the start | "Ze runs as user `ze`" holds for Ze and not for its VPP, and the unit needs more capabilities or a root step | The sibling spec's container proof, run as the `ze` user under the unit's capability set | unvalidated |
| A-10 | An archive whose member names carry no leading slash, extracted with `tar -C /`, is what he means by "a tar with absolute path" | His sentence ends "people can get directly files in the right location" | The archive is made with absolute member names, which `tar` strips with a warning | Showing him the extraction line (Design presentation, "Before and after") | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-2 | nFPM pulls about 38 modules into `vendor/`, including go-git | vendor diff size; `govulncheck` findings | Pin v2.47.0; size the vendor diff at design; hand-writing deb and rpm is the rejected alternative |
| R-3 | A future nFPM or Go bump changes bytes between the maintainer and CI | the CI comparison fails on an unchanged tag | Both sides build with the pinned toolchain and vendored modules; the failure blocks publishing, never publishes a mismatch |
| R-5 | Unsigned binaries on macOS are blocked by Gatekeeper | "cannot be opened because the developer cannot be verified" | Homebrew formula (not quarantined). A direct download is quarantined, and nothing clears it now that install.sh is gone: the guide gives the command |
| R-7 | An upload stops halfway, leaving a draft with some assets | `draft` exits non-zero; the draft lists fewer assets than `checksums.txt` | `draft` is re-runnable while the release is a draft: it replaces assets with the same name. `publish` refuses a draft whose asset set differs from `checksums.txt` |
| R-8 | The release is published, then the Homebrew tap push fails | `publish` reports the tap step failed | `publish` on an already published release that matches skips publishing and retries only the tap push |
| R-9 | le compares against a hash list from another run, commit or tag | the hash list's recorded tag or commit differs | The CI hash list records tag, commit, Go version and run id; `publish` refuses any mismatch and names the field |
| R-10 | The GPG card is absent or `gh` is not logged in | the first gpg or gh call fails | `build` checks the signing key and `draft`/`publish` check `gh auth status` before any work, and stop with the fix named |
| R-11 | A same-day correction is tagged `.N` while the earlier release stays published | two releases on one date | Intended: immutable releases cannot be replaced; `.N` sorts after the plain date in dpkg, rpm, Homebrew and Ze's own comparison |
| R-12 | A two-digit year orders wrongly across a century, and reads as a day or a month to a person | `99.12.31` against `00.01.01` | Accepted with his decision: the year is read as 20YY. The parser refuses nothing on that ground |
| R-13 | `tar` writes files in place, so a version directory exists half written for the seconds an extraction lasts. A command or a service start in that window can run a partial `ze` or find a partial VPP | A start that fails during an extraction | The archive orders its members so that the version's `ze` is the last one written, and the launcher counts a directory as a version only when that file is there and executable. A partly written `ze` is still possible for a moment; the guide says to extract while the service is stopped or before the first start. A self-update is not exposed: it stages and renames |
| R-14 | The launcher at `<prefix>/bin/ze` and the latest version directory come from different releases, after a self-update or after an older tarball was extracted | A launcher that does not know a later directory rule | The contract between them is three things only, and a test holds every launcher to it (AC-41). Choice Q-6 settles the refresh |
| R-15 | An operator extracts a tarball by hand and never verifies it | Nothing in Ze sees a hand extraction | Choice Q-2 |
| R-16 | A compromised daemon installs a version that every later start runs | A version directory no release statement accounts for | Stated to the owner with the decision. The self-updater installs only what a signed statement names; Q-4 bounds the owners the service accepts; root runs the latest like every caller (N-6 (b)), and the guide states that risk; `ze doctor` compares each installed version with what it should hold |
| R-17 | A Ze that is not root cannot start its VPP (A-9) | The sibling spec's proof passes as root only | The proof is run as the `ze` user before this design is called proved |
| R-18 | Removing the only installed version leaves a host with a launcher and no `ze` | The launcher's "no version" refusal | The refusal names the command that installs one. His rule refuses only a version a running daemon uses |

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
| `./le site build` | → | the release public key on ze-software.net | `TestSiteBuildPublishesReleaseKey` |
| The Linux tarball extracted at an empty root, then `usr/local/bin/ze version` | → | the launcher finds the Ze home, chooses the version and runs its `ze` | `test/ui/launcher-runs-latest.ci` |
| The launcher with the version variable naming an installed and a missing release | → | the launcher's selection and its refusal | `test/ui/launcher-version-variable.ci`, `TestLauncherAgreesWithReleaseOrder` |
| The version listing typed at a shell | → | the listing command of the local plugin | `test/ui/version-list.ci` |
| The removal of one version typed at a shell | → | the removal and its refusals: the checks in the local plugin, the file work in the launcher (Q-1, taken as (a)) | `test/ui/version-delete.ci` |
| `ze` started from `<prefix>/lib/ze/<release>/ze` | → | `classifyBinary` gives the config and data directories of the prefix | `TestClassifyBinaryVersionDirectory` |
| `ze install systemd` on a versioned install | → | the unit names the launcher, and the trust rule accepts the layout | `TestUnitRunsLauncher`, `TestTrustedExecutableVersionDirectory` |
| `system update-check { auto-apply true; }` against a fixture server | → | the updater verifies, stages and renames a version directory, then restarts through the launcher | `test/plugin/selfupdate-installs-version-directory.ci` |
| The DEB and the RPM installed in a container | → | the package places the launcher and one version directory, and `ze` runs | The package smoke tests, run through `./le build release smoke` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le build release targets` | Lists all 17 matrix pairs (5 OS x 4 arch minus the 4 tuios exclusions) once each, each marked supported or unsupported; every unsupported row names the build failure that holds it back. On 2026-09-28 supported is exactly linux and darwin on amd64 and arm64 |
| AC-2 | A build asks for an unsupported target | Refused before any build, naming the target and its recorded reason |
| AC-3 | Release strings `26.09.28`, `26.09.28.1`, `26.09.28.12` | Accepted |
| AC-4 | Release strings `2026.09.28`, `26.9.28`, `26.09.28.0`, `26.09.28.01`, `26.13.01`, `26.02.30`, `v26.09.28`, `26.09.28-1` | Refused as a tag, with the error naming the expected form |
| AC-5 | Comparing releases | Numeric by year, month, day, then counter (absent counter sorts first): `26.09.28` < `26.09.28.1` < `26.09.28.2` < `26.09.28.10` < `26.09.29`; `26.10.01` > `26.09.30` |
| AC-6 | A stamp written before this change, and a string that is no release | A config stamped `26.09.28` is accepted by a `26.09.29` binary, because the date part did not change form; `dev` and an unparseable string still sort oldest |
| AC-7 | A non-release build (`./le` builds, the test runner) | Stamps a string that is not a release (Q-9, taken as (a)), so a development build of the day a release is made never reads as that release: the launcher never runs a directory of that name, and installing the build as a version is refused, saying that only a release build makes a version. What else reads that stamp (the config schema stamp, the update comparison) is read before this is built, as Q-9 (a) records |
| AC-8 | `build` on a dirty tree, a tag that does not exist, an unsigned or unverifiable tag, a tag dated after today UTC, or a commit whose GitHub verify workflow did not succeed | Refused before building, each with its own message naming the tag, the commit and the fix |
| AC-9 | `build tag <T>` succeeds | `dist/<T>/` holds, for each supported target, one archive (zip for windows, tar.gz otherwise); for each supported Linux target one `.deb` and one signed `.rpm`, and the source archive of every LGPL component of the VPP bundle; plus `checksums.txt`, `checksums.txt.asc`, `ze.rb` and `CHANGELOG.md`. It holds no `install.sh` |
| AC-10 | Running `ze version` from any built binary | Prints `<T>`, the 12-character commit and the commit's UTC committer time as build date |
| AC-11 | Each binary | Built from a `git archive` of the tag with `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, the daemon tag set from `feature-gates.txt`, and pinned GOAMD64, GOARM, GOARM64 and GO386 |
| AC-12 | Building the same tag twice, in different directories, with different umasks | Every artifact except `checksums.txt.asc` is byte-identical, and every RPM is identical after its signature header |
| AC-13 | Linux archive contents | Exactly the launcher at `usr/local/bin/ze` (0755), `usr/local/lib/ze/<T>/ze` (0755), every file of the pinned VPP bundle under `usr/local/lib/ze/<T>/vpp/` with the modes of its manifest, and `LICENSE` and `README.md` (0644) in the version directory; owner root; no member name starts with a slash or leaves `usr/local/`; the version's `ze` is the last member. The macOS archive takes the same layout with no `vpp` (Q-10, taken as (a)) |
| AC-14 | `.deb` and `.rpm` contents | Exactly `/usr/bin/ze` (0755, the launcher, byte-equal to the tarball's), `/usr/lib/ze/<T>/ze` (0755), the pinned VPP bundle under `/usr/lib/ze/<T>/vpp/`, `LICENSE` and `README.md` in the version directory, `/usr/lib/systemd/system/ze.service` (0644, byte-equal to the systemd plugin's unit for the launcher `/usr/bin/ze` and `/etc/ze`), `/usr/share/ze/install-method` holding `deb` or `rpm`, and the licence and copyright file under `/usr/share/doc/ze/` naming every bundled component; version field `<T>`, architecture mapped per format (amd64/x86_64, arm64/aarch64, arm/armhf/armv7hl, 386/i386/i686) |
| AC-15 | Installing the `.deb` on Debian 12 or the `.rpm` on Fedora 45 (key imported) | Creates a locked system user and group `ze`, creates `/etc/ze` mode 0700 owned `ze`, reloads systemd when it runs, prints the `ze init` and `systemctl enable --now ze` next steps, and does not start or enable the service |
| AC-16 | Removing the package | Stops and disables the service, leaves `/etc/ze` and the `ze` user in place |
| AC-17 | The unsigned `.rpm` on Fedora 45 | Refused by `rpm -i`, which the smoke test asserts, proving the signature is what makes the signed one install |
| AC-18 | `checksums.txt` and its signature | Lists the SHA-256 of every other asset, sorted; `gpgv` verifies `checksums.txt.asc` with the key in `website/release/ze-release.asc`; a different key fails |
| AC-19 | Homebrew formula `ze.rb` | Has one URL and SHA-256 per supported macOS and Linux target, installs `ze` and `share/ze/install-method` holding `homebrew`, and has a `service do` block running `ze start` with keep-alive |
| AC-20 | `ze` started from `<brew>/Cellar/ze/<ver>/bin/ze` | Resolves its config folder to `<brew>/etc/ze`, and so does the crash folder; `ze.config.dir` still overrides |
| AC-21 | Update check, download or apply when `<prefix>/share/ze/install-method` next to the resolved binary holds `deb`, `rpm` or `homebrew` | Refused with a message naming the install method and the upgrade path (the release page for deb and rpm, `brew upgrade ze` for homebrew); the binary is never touched |
| AC-22 | The install-method file holds any other value, or cannot be read | Update refused, naming the file and its content or error |
| AC-23 | No install-method file, on a `/usr/local` install whose Ze home the service can write, and a newer release offered by a signed statement | The updater installs `<prefix>/lib/ze/<new>/` beside the running version, changes no file of the running version, and the restart runs `<new>` through the launcher |
| AC-24 | `ze doctor` | Reports the install method (`deb`, `rpm`, `homebrew`, or `direct` when absent); an unknown or unreadable file is an error with its own diagnostic code |
| AC-25 | `CHANGELOG.md` | Lists the news fragments (`website/changes/fragments/*.md`, its `README.md` excluded) added between the previous release tag and `<T>`, as `git log --diff-filter=A` over that directory answers, each read at `<T>` through the one fragment parser `internal/le/commit/newsfragment.go` declares; one entry per fragment, grouped by the fragment's `topic`, groups sorted by name; no line is taken from a commit subject; a fragment that does not parse refuses the build, naming its path; the first release holds one line saying it is the first |
| AC-26 | `draft tag <T>` | Creates a draft release on the tag with `CHANGELOG.md` as notes and uploads every asset; re-running replaces assets while it is a draft; refuses if the release is already published |
| AC-27 | Tag push reaches GitHub | `release-rebuild.yml` runs with `contents: read` only, runs `./le build release rebuild tag <T>`, and uploads a hash list recording tag, commit, Go version, run id and the SHA-256 of every unsigned artifact (RPMs hashed after the signature header) |
| AC-28 | `publish tag <T>` with a matching hash list | Publishes the draft and pushes `ze.rb` to `ze-software/homebrew-tap` |
| AC-29 | `publish tag <T>` with any mismatch: an artifact hash, a missing or extra artifact, tag, commit, or Go version | Refused, naming each differing artifact or field with both values; the draft stays a draft |
| AC-30 | `publish` on a release already published and matching | Skips publishing and retries the tap push only |
| AC-31 | `./le site build` | Writes `release/ze-release.asc` into the site, from `website/release/ze-release.asc`. It writes no `install.sh` |
| AC-32 | `sudo tar -C / -xzf ze_<T>_linux_<arch>.tar.gz` on a Debian 12 and a Fedora host with no Ze | `ze version` prints `<T>` with nothing moved or run in between. Extracting a second, later release leaves the first directory unchanged, and `ze version` prints the later one |
| AC-33 | A tampered tarball or a tampered `checksums.txt`, on a host with no Ze and on a host with one installed | With no Ze, nothing in Ze checks a hand extraction: `docs/guide/quickstart.md` gives the `gpgv` and `sha256sum` lines, and each refuses the tampered file. With a Ze installed, `ze install release <tarball>` has the installed `ze` verify the signature with the release key compiled into it, then the hash, and refuses before any file is placed, naming the check that failed; no `gpgv` is needed (Q-2, taken as (c)). A plain `tar` stays possible and unverified |
| AC-34 | Every target list in the formula and the packages | Derived from the one target table; a target added there appears in each without another edit |
| AC-35 | A Ze home holding `26.11.28`, `26.11.28.1`, `26.11.28.10` and `26.12.03`, and beside them a dot-prefixed directory, a symbolic link named `26.12.31`, a directory `26.12.30` with no `ze`, a file named `current` and a directory `2026.11.28` | The launcher runs `26.12.03`. With that directory gone it runs `26.11.28.10`. No other entry is ever run |
| AC-36 | The version variable names `26.11.28`, which is installed | The launcher runs it, and a child of that process that runs `ze` by name runs `26.11.28` too |
| AC-37 | The version variable names a release that is not installed | Refused with exit 2 and nothing run: the message names the value, the Ze home and the installed versions, and its last line is the command that works |
| AC-38 | A Ze home with no version | Refused with exit 2: the message says no version is installed and gives the command that installs one |
| AC-39 | The version listing on a `/usr/local` install with two versions and a running service, on a package install, and on the appliance | One row per version with the fields Data Flow names, the daemon's version marked; one row on a package install; one row on the appliance. `\| json` and `\| table` render the same rows |
| AC-40 | The removal of one version | The whole directory goes, `ze` and its VPP together, and no other version changes. Each condition of the refusal table in Data Flow refuses with its message and its last-line command, and changes nothing. A version a running daemon uses is refused; the latest version, when no daemon uses it, is removed |
| AC-41 | The launcher of this release, and a version directory laid out as the first release with this layout made it | The launcher runs it. A fixture of that first directory is kept in the tree |
| AC-42 | A service running `26.12.03`, with `26.11.28` also installed: stop the service, remove `26.12.03`, start the service | The daemon runs `26.11.28` |
| AC-43 | The complete removal on a `/usr/local` install with the service stopped, the same with the service running, and on a DEB install | Stopped: every version directory, the launcher and the service unit are gone and `/etc/ze` is byte-identical. Running: refused, nothing changes, the service keeps running, and the last line is `sudo systemctl stop ze.service` (owner, 2026-11-28, L-5). On the DEB install it is refused with `sudo apt remove ze` |
| AC-44 | `ze install systemd` on a `/usr/local` install; then a later version is installed while the daemon runs; then the service restarts | The unit's `ExecStart` is `/usr/local/bin/ze start`; the running daemon keeps its version; after the restart it runs the later one |
| AC-45 | `ze` at `/usr/lib/ze/26.11.28/ze`, `/usr/local/lib/ze/26.11.28/ze` and `/opt/ze/lib/ze/26.11.28/ze` | The config directory is `/etc/ze`, `/etc/ze` and `/opt/ze/etc/ze`, as for a `ze` at the `bin` path of the same prefix, and the package marker is found under the same prefix |
| AC-46 | An update whose archive holds a member outside the version directory, a link that leaves it, a device, or a set-user mode; and a held archive changed before the window opens | Refused before any directory takes a version name; the staging directory is removed; the history records the cause |
| AC-47 | `update system firmware rollback` on the self-update backend | Refused with the three commands of the rollback path. No `.prev` file exists at any point of an update |
| AC-48 | The appliance | The listing answers one row; the removal and the installation are refused and name `ze appliance push <name>`; the image holds no launcher; `auto-apply` installs nothing |
| AC-49 | A version directory owned by the `ze` user, with no group or other write and no link on its path | `ze install systemd` and the start of the service accept it, and so they do a version directory owned by root. A version directory owned by any other user, any group or other write under the Ze home, a link anywhere on the path, and a launcher or a directory above the Ze home not owned by root are refused before bootstrap, service stop or replacement, naming the path and its owner (Q-4, taken as (a)) |
| AC-50 | `ze version list` | Refused with exit 1, naming the listing command. `ze version` and `ze version --extended` print what they print today |
| AC-51 | `ze doctor` on a `/usr/local` install whose Ze home the service cannot write | A finding names the directory, its owner, that an automatic update cannot install there, and the command that fixes it |
| AC-52 | The install and uninstall functions of the launcher (Q-1, taken as (a)) | The install function takes a tarball: with a `ze` installed it has that `ze` verify the release first (AC-33) and refuses on failure, then puts the version in place; with no `ze` it only extracts. The uninstall function removes one version only after the installed `ze` answered that no daemon uses it (AC-40), and removes everything only after that `ze` answered that the service does not run and removed the service, the launcher last (AC-43). The Go `ze install` and `ze uninstall` keep the first-time initialization, the `ze` account and the service, and neither copies nor removes a binary |
| AC-53 | A fresh host after a first extraction by root, then `sudo ze install systemd` | `ze install systemd` creates the `ze` account as today and gives `/usr/local/lib/ze` to it, so the service can install an update. A host on which it never ran keeps root-owned files and does not update itself (Q-3, taken as (a)) |
| AC-54 | Root runs `ze` when the latest version belongs to the `ze` user | The launcher runs that latest version, as for every caller, with no owner check and no notice (N-6, taken as (b)). `docs/guide/ze-install.md` states that a compromised daemon so chooses the program root runs |
| AC-55 | The launcher after a self-update that brought a newer launcher text | The launcher at `<prefix>/bin/ze` is left as it is and runs the new version, because its contract with a version directory stays a directory name, a file named `ze` and one variable (AC-41). `ze doctor` reports a launcher older than the latest version's and gives the root command that refreshes it (Q-6, taken as (a)) |
| AC-56 | The local release build and the CI rebuild package the pinned VPP bundle | Both fetch the bundle archive and its source archives by SHA-256 from the assets of the public GitHub release named for that digest, uploaded once by the maintainer. A fetched file whose digest differs refuses the build, naming both digests, and the packaged bundle is byte-identical in both builds (Q-12, taken as (a)) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Maintainer releases | signed tag → `build` → `draft` → CI `rebuild` → `publish` → tap | Rehearsal against a scratch repository with a test key, recorded in Goal Validation |
| 2 | Debian user installs the `.deb` | download → `apt install ./ze_<T>_amd64.deb` → launcher, version directory, user, `/etc/ze` → `ze init` → `systemctl enable --now ze` | Debian 12 smoke test in `release-rebuild.yml` |
| 3 | Fedora user installs the `.rpm` | `rpm --import` key → `dnf install ./ze-<T>-1.x86_64.rpm` → same as 2 | Fedora 45 smoke test in `release-rebuild.yml` |
| 4 | macOS user installs with Homebrew | `brew install ze-software/tap/ze` → config in `<brew>/etc/ze` → `brew services start ze` | macOS smoke test installing `ze.rb` rewritten to local file URLs |
| 5 | Linux user installs from the tarball | download → `sudo tar -C / -xzf` → `ze version` | `test/ui/launcher-runs-latest.ci` and the tarball step of the smoke tests |
| 6 | A packaged user runs self-update | install-method file → package-managed backend → refusal naming the upgrade path | `test/ui/update-refused-package-managed.ci` |
| 7 | A tampered artifact reaches publish | CI hash list differs → publish refused | `TestPublishRefusesMismatch` |
| 8 | A `/usr/local` host updates itself | signed statement → archive → staged version directory → rename → restart through the launcher | `test/plugin/selfupdate-installs-version-directory.ci` |
| 9 | An operator rolls back a bad release | stop the service → remove the release → start the service | `test/ui/version-delete.ci` (rollback step) |
| 10 | An operator runs one command on an older version | the version variable → the launcher → that version's `ze` | `test/ui/launcher-version-variable.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRelease` | `internal/core/version/version_test.go` | AC-3, AC-4 | |
| `TestCompareReleasesNumeric` | `internal/core/version/version_test.go` | AC-5 | |
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
| `TestChangelogGroupsFragmentsByTopic`, `TestChangelogIgnoresCommitSubjects`, `TestChangelogRefusesUnparsableFragment`, `TestChangelogFirstRelease` | `internal/le/build/release/changelog_test.go` | AC-25: only fragments added in the tag range appear (one added before the previous tag and the README stay out), grouped and sorted by topic; a commit in the range with no fragment adds no line; a malformed fragment refuses, naming its path | |
| `TestDraftUploadsAndIsRerunnable` | `internal/le/build/release/github_test.go` | AC-26 | |
| `TestRebuildHashListMatchesBuild` | `internal/le/build/release/compare_test.go` | AC-27 | |
| `TestPublishPublishesOnMatch`, `TestPublishRefusesMismatch`, `TestPublishRetriesTapOnly` | `internal/le/build/release/github_test.go` | AC-28, AC-29, AC-30 | |
| `TestReleaseWorkflowCallsRegisteredActions` | `internal/le/workflowcheck/workflowcheck_test.go` | AC-27 | |
| `TestSiteBuildPublishesReleaseKey` | `internal/le/site/build_test.go` | AC-31 | |
| `TestArchiveLinuxFinalPaths` | `internal/le/build/release/archive_test.go` | AC-13: member names, modes, order, nothing outside `usr/local/` | |
| `TestLauncherIsOneFile` | `internal/le/build/release/packages_test.go` | AC-13, AC-14: the launcher in the tarball, the DEB and the RPM is byte-equal to the one declared in the repository | |
| `TestLauncherAgreesWithReleaseOrder` | `internal/core/installlayout/launcher_test.go` | AC-35: the launcher script and `CompareReleases` choose the same directory from the same sets of names | |
| `TestLauncherIgnoresWhatIsNotAVersion` | `internal/core/installlayout/launcher_test.go` | AC-35: a dot name, a link, a directory with no `ze`, a file, a four-digit name | |
| `TestLauncherVariable`, `TestLauncherRefusals` | `internal/core/installlayout/launcher_test.go` | AC-36, AC-37, AC-38 | |
| `TestLauncherRunsFirstReleaseLayout` | `internal/core/installlayout/launcher_test.go` | AC-41 | |
| `TestClassifyBinaryVersionDirectory` | `internal/core/paths/paths_test.go` | AC-45 | |
| `TestVersionListRows`, `TestVersionListPackageAndAppliance` | `internal/plugins/local/versions_test.go` | AC-39 | |
| `TestVersionDeleteRemovesDirectory`, `TestVersionDeleteRefusals` | `internal/core/installlayout/launcher_test.go` for the file work, `internal/plugins/local/versions_test.go` for the refusals (Q-1, taken as (a)) | AC-40, AC-48 | |
| `TestUninstallRemovesEveryVersion` | `internal/core/installlayout/launcher_test.go` for the file work, `internal/plugins/local/native_test.go` for the service work (Q-1, taken as (a)) | AC-43 | |
| `TestUnitRunsLauncher` | `internal/plugins/systemd/unit_test.go` | AC-44 | |
| `TestTrustedExecutableVersionDirectory` | `internal/plugins/systemd/native_trust_test.go` | AC-49 | |
| `TestUpdateInstallsVersionDirectory`, `TestUpdateRefusesUnsafeArchive`, `TestUpdateRehashesHeldArchive` | `internal/component/config/system/selfupdate_test.go` | AC-23, AC-46 | |
| `TestRollbackAnswersTheThreeCommands` | `internal/component/config/system/selfupdate_test.go` | AC-47 | |
| `TestVersionRootRefusesUnknownWord` | `cmd/ze/ze_core_dispatch_test.go` | AC-50 | |
| `TestDoctorReportsUnwritableHome` | `internal/component/config/system/doctor_test.go` | AC-51 | |
| `TestLauncherChoosesByOrderOnly` | `internal/core/installlayout/launcher_test.go` | AC-54: run as root over a Ze home whose latest version another user owns, the launcher runs that version and prints nothing; the script reads no owner and no caller identity | |
| `TestUninstallAllRefusesRunningService` | `internal/plugins/local/native_test.go` | AC-43: with the service running the complete removal changes nothing and names `sudo systemctl stop ze.service` | |
| `TestInstallReleaseVerifiesFirst` | `internal/core/installlayout/launcher_test.go` | AC-33, AC-52: a tampered tarball or `checksums.txt` given to the install function with a `ze` installed is refused before any file is placed | |
| `TestInstallSystemdGivesHomeToZe` | `internal/plugins/systemd/native_test.go` | AC-53 | |
| `TestDoctorReportsOlderLauncher` | `internal/component/config/system/doctor_test.go` | AC-55 | |
| `TestReleaseBuildFetchesBundleByDigest` | `internal/le/build/release/build_test.go` | AC-56: a fetched bundle whose digest differs refuses the build, naming both digests | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| year | 00-99, two digits | 99 | N/A (00 is valid) | 100 (three digits), and any four-digit year |
| month | 01-12 | 12 | 00 | 13 |
| day | 01-last day of month | 29 on 28.02, 28 on 26.02 | 00 | 29 on 26.02, 32 |
| same-day counter | 1-999 | 999 | 0 | 1000 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-build-release-targets` | `test/ui/le-build-release-targets.ci` | The maintainer lists targets and sees supported and unsupported with reasons | |
| `update-refused-package-managed` | `test/ui/update-refused-package-managed.ci` | A packaged ze refuses self-update and names the upgrade path | |
| `launcher-runs-latest` | `test/ui/launcher-runs-latest.ci` | A tarball extracted under a scratch root: `ze version` prints its release, and a second extraction makes the later one run | |
| `launcher-version-variable` | `test/ui/launcher-version-variable.ci` | The variable runs an older installed version, and a value that names none is refused with the list | |
| `version-list` | `test/ui/version-list.ci` | The listing shows two installed versions and which one runs | |
| `version-delete` | `test/ui/version-delete.ci` | One version is removed, a version in use is refused with the commands that work, and the rollback path ends on the older version | |
| `selfupdate-installs-version-directory` | `test/plugin/selfupdate-installs-version-directory.ci` | A daemon installs a newer version directory from a fixture server and restarts into it | |
| Package smoke tests | `.github/workflows/release-rebuild.yml`, run through `./le build release smoke` | Debian 12 and Fedora 45: the package installs the launcher and one version directory, `ze version` answers, user and folder created, service not started, remove keeps `/etc/ze`; unsigned RPM refused; the tarball extracted at the root; macOS formula install | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol behavior changes.

## Files to Modify
- `internal/core/version/version.go` - strict `YY.MM.DD[.N]` parsing, numeric comparison, commit stamp
- `internal/component/config/stamp.go`, `internal/component/config/migration/evolution.go` - comments and error text naming the form
- `internal/le/go/toolchain/gotoolchain.go` - a release mode taking version, date and commit explicitly with `-trimpath` and `-buildvcs=false`; the development stamp as choice Q-9 settles
- `internal/test/runner/runner.go` - the test runner's own stamp, as choice Q-9 settles
- `cmd/ze/main.go`, `cmd/ze/dispatch.go` - a `commit` ldflag variable passed to the version stamp
- `cmd/ze/ze_core_dispatch.go` - the `version` root refuses a word it does not know
- `internal/core/paths/paths.go` - the Homebrew Cellar rule in `ConfigDirFromBinary`; a `ze` in a version directory classified as the launcher beside it
- `internal/component/config/system/backend.go`, `selfupdate.go` - install-method lookup and backend selection; the updater installs a version directory; the in-place replacement, `.prev` and its rollback are deleted
- `internal/plugins/systemd/unit.go` - export the unit builder so le generates the package unit from it; `ExecStart` names the launcher; the Ze home stays writable for the service on a `/usr/local` install
- `internal/plugins/systemd/main.go`, `native_trust.go`, `cmd_install.go` - `resolveBinaryPath` answers the launcher; the trust rule for a version directory (choice Q-4); the account and the owner of the Ze home (choice Q-3)
- `internal/plugins/local/native.go`, `cmd_install.go`, `cmd_uninstall.go`, `register.go` - the copy of the running binary to the bin path is deleted; the complete removal refuses while the service runs and removes the service, and the launcher then removes every version directory and itself (Q-1, taken as (a)); the marker lookup uses the path classification; the first-time initialization, the `ze` account and the service stay here
- `cmd/ze/install/native.go` - the service owner answers which executable the service's main process runs
- `internal/le/build/compile/build.go` - the target matrix and release build mode
- `internal/le/register.go` - blank import of the release command
- `internal/le/site/build.go` - the release public key
- `internal/le/workflowcheck/workflowcheck_test.go` - the new workflow's privileges
- `go.mod`, `go.sum`, `vendor/` - nFPM v2.47.0
- `ai/INDEX.md` - the le repo inventory row, and the keywords of the versioned install
- `plan/release/delivery/spec-release-distribution.md` - scope amended to consume this spec's output; the `database.zefs` statement corrected
- `docs/guide/quickstart.md` - installation by package, by tarball and by Homebrew
- `docs/guide/self-update.md`, `docs/architecture/config/system-update.md`, `docs/architecture/appliance/self-update.md` - the package-managed refusal; an update installs a version directory; the release form and its numeric order
- `docs/guide/ze-install.md`, `docs/architecture/cli/plugin-modes.md` - the layout, the launcher, the listing, the removal and its refusal while the service runs, who owns a version directory, and the risk that root runs the latest version like every caller (N-6 (b))
- `docs/guide/command-reference.md` - the version commands
- `docs/guide/status.md` - the Packaging row
- `docs/architecture/config/syntax.md`, `docs/config-reference.md` - the release form in the schema stamp

## Files to Create
- `internal/le/build/release/` - `actions.go`, `register.go`, `targets.go`, `gate.go`, `build.go`, `archive.go`, `packages.go`, `checksums.go`, `sign.go`, `formula.go`, `changelog.go`, `github.go`, `compare.go`, `smoke.go`, and their tests
- `internal/le/build/release/scripts/` - the deb and rpm post-install, pre-remove and post-remove scripts, embedded
- `internal/core/installlayout/` - the layout declared once: the Ze home and version directory names, the grammar of a version directory name, the launcher script as one file, and the tests that hold the launcher to `internal/core/version`
- `internal/plugins/local/versions.go` - the version listing, and the check that no daemon uses a version. The removal of the files lives in the launcher (Q-1, taken as (a))
- `internal/component/config/system/backend_package.go` - the package-managed backend
- `internal/component/config/system/doctor.go` (or the existing doctor file of that package) - the install-method check and the check of a Ze home the service cannot write
- `website/release/ze-release.asc` - the release public key (the owner supplies it)
- `.github/workflows/release-rebuild.yml` - the read-only rebuild and smoke tests
- `docs/contributing/releasing.md` - the maintainer runbook: key setup, tagging, the VPP bundle pin, build, draft, publish, tap repository creation
- `test/ui/le-build-release-targets.ci`, `test/ui/update-refused-package-managed.ci`, `test/ui/launcher-runs-latest.ci`, `test/ui/launcher-version-variable.ci`, `test/ui/version-list.ci`, `test/ui/version-delete.ci`
- `test/plugin/selfupdate-installs-version-directory.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The version listing carries a YANG registration, and so does the program's check that no daemon uses a version, which the launcher's removal asks for (Q-1, taken as (a)). The listing answers about the host, so it is a plain local command with a builtin RPC on the same path, as `show version` is. No config leaf is added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `./le build release` in `internal/le/build/release/`; the version listing in `internal/plugins/local`; the removal, its checks there and its file work in the launcher (Q-1, taken as (a)); the `version` root's refusal |
| CLI grammar (keyword before value) | Yes | `tag <T>`, `repo <owner/name>` for le; the version commands as choice Q-8 settles; both checked by `./le cli grammar` |
| Editor autocomplete | N-A | No config leaf |
| Functional test for new RPC/API | Yes | `test/ui/version-list.ci`, `test/ui/version-delete.ci`, `test/ui/le-build-release-targets.ci`, `test/ui/update-refused-package-managed.ci` |
| Pipe completeness | Yes | The version listing and `targets` answer through the shared pipe layer |
| Env var registration | Yes | The version variable, registered with `env.MustRegister` beside the code that reads it. Its name is choice Q-7. `ZE_INSTALL_DIR` and install.sh's own `ZE_VERSION` are gone with install.sh |
| Doctor check for runtime dependencies | Yes | The install-method file, and a Ze home the service cannot write: checks registered by the package that owns each, with diagnostic codes |
| Doctor code per problem class | Yes | `doctor-install-method-unknown` (the marker holds an unknown value or cannot be read); `doctor-ze-home-not-writable` (an automatic update cannot install a version) |
| Prometheus counters/metrics | N-A | No runtime rate to observe |
| BGP family surface | N-A | No BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | The feature page for installation and update under `features/`, found with `./le feature report` |
| 2 | Config syntax changed? | No | The config stamp keeps its form; a counter can follow it |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the version commands; `docs/contributing/releasing.md` for le |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` for the version listing's answer shape |
| 5 | Plugin added/changed? | Yes | `docs/architecture/cli/plugin-modes.md`: the local plugin and the systemd plugin change what they install |
| 6 | Has a user guide page? | Yes | `docs/guide/quickstart.md`, `docs/guide/self-update.md`, `docs/guide/ze-install.md` |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | The smoke tests live in a workflow and le, not the functional test harness |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/system-update.md` (backend selection, the release order), `docs/architecture/appliance/self-update.md` (what an update installs) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` Packaging row; `docs/architecture/doctor-and-health-checks.md` for the two codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/delivery/spec-release-build.md`. Known now: `docs/architecture/cli/plugin-modes.md` anchors `native.go`, `native_trust.go`, `cmd_install.go` and `cmd_uninstall.go`, and its "Native installation commands" section becomes wrong; `docs/architecture/appliance/self-update.md` anchors `selfupdate.go`, and its "Replacement is a hard link then a rename" becomes wrong; `docs/architecture/config/syntax.md` anchors the stamp |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/quickstart.md` install commands; `docs/guide/ze-install.md`; the `YY.MM.DD` text of `docs/config-reference.md` and `docs/architecture/config/system-update.md`, which gain the counter |

## Design presentation

### What changes

A Ze installation on Linux stops being one file. It becomes a launcher script at `<prefix>/bin/ze` and one directory per release under `<prefix>/lib/ze`, each holding `ze` and its VPP. Several releases stay installed. Typing `ze` runs the latest one, unless an environment variable names another installed one. A package uses the same shape under `/usr`. The tarball holds the files at their final paths, so extracting it at the root installs Ze, and there is no install.sh. A release is named `YY.MM.DD`, with `.N` for a later release of the same day. Ze runs as the `ze` user, and that user installs a new version, so an automatic update needs nobody. The appliance keeps none of this: one version in its image, replaced with the image.

### Before and after

1. **Installing from the tarball.** Before (the design of 2026-09-28): `curl -fsSL https://ze-software.net/install.sh | sh` downloads the release, verifies it and writes `/usr/local/bin/ze`. After: the operator downloads `ze_26.11.28_linux_amd64.tar.gz` and types `sudo tar -C / -xzf ze_26.11.28_linux_amd64.tar.gz`. That writes `/usr/local/bin/ze` (the launcher) and `/usr/local/lib/ze/26.11.28/` with `ze` and `vpp/`. Then `ze version` prints `26.11.28`.

2. **Two releases installed.** Before: not possible, one file. After: the operator extracts `ze_26.12.03_linux_amd64.tar.gz` the same way. `/usr/local/lib/ze/` holds `26.11.28` and `26.12.03`. `ze version` prints `26.12.03`. With the version variable set to `26.11.28` in front of the command, it prints `26.11.28`.

3. **The variable names a release that is not installed.** After, on stderr, exit 2: the variable names 26.10.01, which is not installed in /usr/local/lib/ze; installed: 26.11.28, 26.12.03; and on the last line the same command without the variable.

4. **Rolling back a bad release.** Before: `update system firmware rollback` renames `.prev` over the binary. After: `sudo systemctl stop ze.service`, the removal of release 26.12.03, `sudo systemctl start ze.service`. The daemon runs 26.11.28. Removing 26.12.03 while the daemon still runs it is refused, and the refusal gives those three commands.

5. **An automatic update.** Before: the daemon renames a new binary over itself, which fails under the systemd unit with "self-update not supported on read-only filesystem". After: the daemon, running as `ze`, verifies the signed statement, downloads the archive, writes `/usr/local/lib/ze/26.12.03/` and restarts through the launcher. No operator acts.

6. **A DEB install.** Before (the design of 2026-09-28): the package holds `/usr/bin/ze`, the program. After: `/usr/bin/ze` is the launcher, and `/usr/lib/ze/26.11.28/` holds `ze` and `vpp/`. The listing shows one version. Removing it is refused with `sudo apt remove ze`.

7. **Release names.** Before (the ruling of 2026-09-28): `2026.11.28`, `2026.11.28.1`. After: `26.11.28`, `26.11.28.1`.

8. **The appliance.** Before and after: one `ze` in the image, updated by replacing the image. The version commands answer one row or refuse with `ze appliance push <name>`.

9. **Root after an automatic update.** After: the daemon, as `ze`, installs `/usr/local/lib/ze/26.12.03/`. An administrator types `sudo ze show version`, and the launcher runs 26.12.03, as it would for any caller, with no notice. `docs/guide/ze-install.md` says that a compromised daemon so chooses the program root runs.

10. **Removing everything while the service runs.** Before: `ze uninstall` stops the service, then removes it and the binary. After: `ze uninstall all` is refused, changing nothing, and its last line is `sudo systemctl stop ze.service`. Run after that command, it removes every version directory, the launcher and the unit, and leaves `/etc/ze`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| A `current` link or a marker file naming the selected version | The owner replaced his first answer: the latest runs, unless the variable says otherwise. The install code also refuses every link on the path of a binary it installs |
| `ze` at `<prefix>/bin/ze` with only VPP in a Ze folder | The owner chose the real `ze` inside the folder, behind a launcher script that is not a link |
| A second, flat layout for packages | The owner chose one layout and one launcher everywhere |
| install.sh, served from ze-software.net | Withdrawn by the owner on 2026-11-28 |
| A tarball with one top directory that an installer moves into place | The owner chose member paths that are the final locations |
| A four-digit year | Reversed by the owner on 2026-11-28 |
| Root installs each update, by hand or through a root-owned helper | The owner chose that the `ze` user installs versions |
| A rollback command, or a pin of the previous release | The owner chose the removal of the bad release between a stop and a start |
| debhelper in place of nFPM, or a repository in the first release | The owner chose nFPM, Debian Policy, and the repository after the first release |
| Version directories and a launcher on the appliance | The owner: the appliance is immutable infrastructure |

### Documents departed from

| Document | Section | Statement | Why this design departs |
|----------|---------|-----------|-------------------------|
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | "Service installation requires root ownership of the executable and every existing ancestor." | The owner decided on 2026-11-28 that Ze does not require root and that the `ze` user installs versions, so a version directory the `ze` user owns must be accepted |
| `docs/architecture/appliance/self-update.md` | Decisions | "**Replacement is a hard link then a rename.**" | An update installs a version directory beside the running one, and a rollback is the removal of a release (owner, 2026-11-28). No file is replaced and no `.prev` is kept |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | "The bare `ze install` and `ze uninstall` commands dispatch to defaults registered by the local plugin." | Bare `ze uninstall` is refused and names both explicit forms, because the complete removal needs an explicit word (Q-8, taken as (a)) |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | "It refuses symlink paths, package ownership markers and conflicting services before copying the executable." and "A failed replacement restores the prior binary and resumes a service that was previously active." | The Go `ze install` no longer copies or replaces the binary; versions arrive by the tarball or the launcher's install function (Q-1, taken as (a)) |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | "Native removal first asks the service owner to stop the selected service." | The complete removal refuses while the service runs, naming the command that stops it (owner, 2026-11-28, L-5) |
| `docs/architecture/config/system-update.md` | The version check | "**Version comparison is lexicographic, not semver.**" | The owner added a same-day counter on 2026-11-28. `26.11.28.10` sorts below `26.11.28.9` as text, so the comparison becomes numeric, part by part |
| `docs/architecture/config/system-update.md` | The version check | "`ze update-serve` is the build-infrastructure side: it serves `/version.json` with arch validation and `/<goos>/<goarch>` for the binary." | That URL serves the release archive, unpacked into a version directory. `spec-selfupdate-manifest-authenticity` owns the change |
| `docs/config-reference.md` | Firmware Update Check | "(lexicographic comparison, matching ze's `YY.MM.DD` date-based versioning)." | The same statement, in the operator's reference |

### Choices for Thomas

Each is a question his words leave open. Asked the first of them on 2026-11-28, Thomas answered (voice-transcribed): "I told you what I wanted to do so stop asking me stupid questions". So none was put to him, and each is taken on the design's recommendation, as its Decision cell says, except N-6: the design moved it from its recommendation (a) to (b), because (a) contradicts two of his rows of 2026-11-28. None is his choice in his own words, and he may change any of them.

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| Q-1 | What do the install and uninstall functions of the `ze` script do, and what becomes of today's `ze install` and `ze uninstall`? You said the script carries them, "a bit like we do with ./le", where the script answers `rollback` itself. Today `ze install` (Go, `internal/plugins/local`) copies the running program to `/usr/local/bin/ze`, runs the first-time initialization and sets up the service, and `ze uninstall` removes the service and that file. One fact bounds the answer. Only the `ze` program can tell whether a daemon uses a version (the store's lock and the service owner), stop and remove the service, and verify a release with the key compiled into it. Only the script can act when no `ze` is installed, or remove the launcher itself. Example: a host runs 26.11.28 and the operator has downloaded `ze_26.12.03_linux_amd64.tar.gz` | (a) recommended: the script carries both functions and does the file work, and it asks the installed `ze` for what only the program holds. Its install function takes a tarball: when a `ze` is installed it has that `ze` verify the release first (Q-2), then it puts the version in place; with no `ze` it only extracts. Its uninstall function removes one version once the installed `ze` answered that no daemon uses it, or removes everything once that `ze` answered that the service does not run and removed it, the launcher last (the complete removal refuses while the service runs, L-5). The Go commands keep what is not files: the first-time initialization, the `ze` account and the service. Their copy of the binary is deleted. (b) the script does everything in shell, the checks, the service and the initialization included, and the Go `ze install` and `ze uninstall` are deleted. (c) the `ze` program does all of it, and the script's two functions exist only for a host with no `ze` installed | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-2 | Who verifies the signature and the hash of a release, now that install.sh is gone? install.sh was the step that checked `checksums.txt.asc` and the archive's hash before installing. Example: an operator runs `curl -LO` on the tarball, then `sudo tar -C / -xzf ze_26.11.28_linux_amd64.tar.gz` | (a) the operator, by hand: the guide gives the `gpgv` and `sha256sum` lines, and nothing in Ze checks a hand extraction. (b) the script's install function: it takes the tarball with `checksums.txt` and its signature, verifies with `gpgv`, refuses when `gpgv` is missing or the check fails, then extracts. A plain `tar` stays possible and unverified. (c) recommended: (a) for the first install, when no Ze exists on the host, and from then on the installed `ze` verifies the next release with the release key compiled into it, the way the self-updater verifies its signed statement, so no `gpgv` is needed on a router | (c), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-3 | Who creates the `ze` user and gives it the version directories? After `sudo tar -C / -xzf`, every file belongs to root and no `ze` account exists, so the daemon could not install an update. Today `ze install systemd` creates the `ze` group and user. Example: a fresh Debian host, tarball extracted, then `sudo ze install systemd` | (a) recommended: `ze install systemd`, which already creates the account, also gives `/usr/local/lib/ze` to it. A host with no service keeps root-owned files and does not update itself. (b) the script's install function creates the account and sets the owner at every install. (c) the operator, by hand, from two lines in the guide | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-4 | Which owners does the trust check accept? Today the service's program and every directory above it must be owned by root, with no group or other write and no link. The relayed reading of your answer, not yet confirmed by you: the owner is root or the service user only. Example: `/usr/local/lib/ze/26.12.03/ze` owned by `ze`, mode 0755, is accepted; owned by `thomas`, or mode 0775, it is refused | (a) recommended: under `/usr/local/lib/ze` the owner is root or the `ze` user, nobody else; the launcher and every directory above the Ze home stay root-owned; no group or other write and no link anywhere. (b) any owner is accepted under the Ze home, and only the write bits and links are checked. (c) the owner is the `ze` user only, so a directory root extracted must be handed over before the service starts | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| N-6 | May root run a version that the `ze` user installed? It follows from "the `ze` user installs a version" and "the latest runs". Example: the daemon, as `ze`, updates itself to 26.12.03; an administrator then types `sudo ze show version`, and the root-owned launcher runs the newest directory, which root did not write. A compromised daemon would so choose the program root runs | (a) when root is the caller, the launcher runs only a version root owns, says in one line that a newer version owned by `ze` exists, and names the command that runs it as the `ze` user. (b) root runs whatever is latest, like every caller, and the guide states the risk that a compromised daemon chooses what root runs. The design first recommended (a) | (b), taken by the design on 2026-11-28 because (a) contradicts two of his rows of that day on `docs/architecture/cli/plugin-modes.md`, which forbid "A version chosen by anything other than version order and that variable" and "Refusing a version directory only because the ze user, and not root, owns it". Not his choice in his words: he declined further questions and may change it |
| Q-6 | How does the launcher follow a self-update? The `ze` user cannot write `/usr/local/bin`, so after the daemon installs 26.12.03 the launcher is still the one release 26.11.28 brought. Example: release 26.12.03 ships a changed launcher | (a) recommended: the launcher is left as it is. Its contract with a version directory stays three things (a directory name, a file named `ze`, one variable) and a test holds every launcher to it; `ze doctor` reports a launcher older than the latest version's and gives the root command that refreshes it. (b) `/usr/local/bin/ze` belongs to the `ze` user, which then decides what every user's `ze` runs. (c) a root-owned helper refreshes it | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-9 | What names a version directory, and how is a development build told from a release? A `./le` build stamps today's date in the release form, so on the day 26.11.28 is released a development build also says `26.11.28`. The recommendation relayed to you was: the directory is named by the release string, and a development build is refused as a version. Your recorded words settle the form, not this. Example: a developer runs `ze install` from `bin/ze` on a host that has release 26.11.28 | (a) recommended: a version directory is named by the release of the `ze` in it; only the release build makes one; a development build stamps a string that is not a release and cannot be installed as a version. What else reads that stamp (the config schema stamp, the update comparison) is read before this is built. (b) a development build is installable under its date plus its commit, a name the launcher then has to order | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-7 | What is the environment variable called? Example: `ZE_VERSION=26.11.28 ze show version` | (a) recommended: `ZE_VERSION`, registered as `ze.version`. install.sh used that name for another purpose and is gone. (b) `ZE_RELEASE`, since the code calls the string a release. (c) `ZE_USE_VERSION` | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-8 | The final command spellings, which you asked to see before anything is built. Example session: list the versions, remove 26.11.28, remove everything | The answer follows Q-1, because a word the script answers itself is a word of `install` or `uninstall`, the two roots that exist today. (a) recommended, with Q-1 (a): `ze show version list` for the listing (a read that enumerates a set, answered by the program); `ze uninstall version 26.11.28` to remove one version (a keyword before the value); `ze uninstall all` to remove everything, with bare `ze uninstall` refused and naming both forms, because your rule of 2026-10-17 wants an explicit word for the everything form; `ze install release ze_26.12.03_linux_amd64.tar.gz` to install from a tarball; `ze version` unchanged. Today's bare `ze uninstall` removes the installation, so this changes it. (b) with Q-1 (c): `ze delete version name 26.11.28` for one version, answered by the program (the thing stops existing; a typed keyword before the value), and `ze uninstall` kept as today for the complete removal. (c) either, with `ze show version installed` for the listing | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-10 | Do the macOS archive and the Homebrew formula take the same layout? Your words name Linux. No VPP exists for macOS. Example: `brew install ze-software/tap/ze` | (a) recommended: the same layout everywhere, so the code has one rule for finding its directories: the keg holds `bin/ze` (the launcher) and `lib/ze/<release>/ze`, with no `vpp`. (b) macOS keeps one program at `bin/ze`, and Ze keeps two layouts in its path code | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-12 | Where does the CI rebuild get the pinned VPP bundle? The release is published only when CI rebuilds every artifact to the same bytes. A VPP build is not byte-identical on two hosts, so the bundle is an input named by its SHA-256, and CI has no cache and no secret. Example: `release-rebuild.yml` builds the DEB for tag 26.11.28 | (a) recommended: the bundle archive and its source archives are uploaded once, by the maintainer, as assets of a public GitHub release named for their digest, and both builds fetch them by digest. (b) CI builds its own bundle, and the comparison skips the bundle's bytes, which removes the witness for a third of the package. (c) the archives are committed to the repository, which you ruled out for the bundle on 2026-11-19 | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| Q-11 | How is a nightly named, and can one be installed as a version? Nightlies are after the first release (`plan/post-release/features/spec-release-repositories-and-nightlies.md`). The older design names one `nightly-YY.MM.DD-g<sha>` with the embedded release `YY.MM.DD`. Example: the nightly of 26.12.01 and the release 26.12.01 would both ask for the directory `26.12.01` | (a) recommended: not in the first release. The launcher accepts `YY.MM.DD` and `YY.MM.DD.N` only, and that spec names a nightly before one can be installed. (b) decide the nightly's directory name now | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |

## Implementation Steps

No product code is written before the owner approved this design. The choices the phases name are taken (Choices for Thomas), so no phase waits on one.

1. **Phase: Wiring** -- register `./le build release` with the `targets` verb and stubs for the others
   - Tests: `test/ui/le-build-release-targets.ci`, `TestTargetTableCoversMatrix`
   - Files: `internal/le/build/release/actions.go`, `register.go`, `targets.go`, `internal/le/register.go`, `ai/INDEX.md`
   - Verify: the command lists the matrix; the other verbs exist and fail as stubs
2. **Phase: Version format** -- strict `YY.MM.DD[.N]`, numeric comparison, the commit stamp, the development stamp (Q-9)
   - Tests: `TestParseRelease`, `TestCompareReleasesNumeric`
   - Files: `internal/core/version/version.go`, `gotoolchain.go`, `runner.go`, `cmd/ze/main.go`, `dispatch.go`, config stamp and evolution, `docs/architecture/config/system-update.md`
   - Verify: every stored `YY.MM.DD` stamp still orders correctly (A-3)
3. **Phase: Layout and launcher** -- the layout package, the launcher script, the path classification (Q-1, Q-7, N-6)
   - Tests: `TestLauncherAgreesWithReleaseOrder`, `TestLauncherIgnoresWhatIsNotAVersion`, `TestLauncherVariable`, `TestLauncherRefusals`, `TestLauncherRunsFirstReleaseLayout`, `TestLauncherChoosesByOrderOnly`, `TestClassifyBinaryVersionDirectory`, `test/ui/launcher-runs-latest.ci`, `test/ui/launcher-version-variable.ci`
   - Files: `internal/core/installlayout/`, `internal/core/paths/paths.go`
4. **Phase: Version commands** -- the listing, the removal of one version, the `version` root's refusal (Q-1, Q-8)
   - Tests: `TestVersionListRows`, `TestVersionListPackageAndAppliance`, `TestVersionDeleteRemovesDirectory`, `TestVersionDeleteRefusals`, `TestVersionRootRefusesUnknownWord`, `test/ui/version-list.ci`, `test/ui/version-delete.ci`
   - Files: `internal/plugins/local/versions.go`, `register.go`, `cmd/ze/ze_core_dispatch.go`, `cmd/ze/install/native.go`
   - Verify: `./le cli grammar` accepts the spellings
5. **Phase: Service, owners and removal** -- the launcher in the unit, the trust rule, the account, the complete removal (Q-1, Q-3, Q-4)
   - Tests: `TestUnitRunsLauncher`, `TestTrustedExecutableVersionDirectory`, `TestUninstallRemovesEveryVersion`, `TestUninstallAllRefusesRunningService`, `TestInstallSystemdGivesHomeToZe`
   - Files: `internal/plugins/systemd/unit.go`, `main.go`, `native_trust.go`, `cmd_install.go`, `internal/plugins/local/native.go`, `cmd_install.go`, `cmd_uninstall.go`, `docs/architecture/cli/plugin-modes.md`
6. **Phase: Install location** -- Homebrew Cellar config rule; install-method file, package-managed backend, doctor checks
   - Tests: `TestConfigDirFromBinaryHomebrewCellar`, `TestNewBackendSelectsPackageManaged`, `TestPackageManagedRefusesUnknownMethod`, `TestDoctorReportsInstallMethod`, `TestDoctorReportsUnwritableHome`, `test/ui/update-refused-package-managed.ci`
   - Files: `paths.go`, `backend.go`, `backend_package.go`, doctor file, self-update docs
7. **Phase: The update installs a version directory** -- after `spec-selfupdate-manifest-authenticity` states the signed archive (Q-6)
   - Tests: `TestUpdateInstallsVersionDirectory`, `TestUpdateRefusesUnsafeArchive`, `TestUpdateRehashesHeldArchive`, `TestRollbackAnswersTheThreeCommands`, `test/plugin/selfupdate-installs-version-directory.ci`
   - Files: `selfupdate.go`, `docs/architecture/appliance/self-update.md`, `docs/guide/self-update.md`
8. **Phase: Binaries and archives** -- release toolchain mode, `git archive` build tree, target matrix, the final-path tarball, the pinned bundle input, checksums, gate checks (Q-10, Q-12)
   - FIRST (owner, 2026-09-28): validate A-1 before anything else in this phase. Build linux/amd64 and darwin/arm64 binaries, one archive and a sample unsigned `.deb` and `.rpm` on the macOS host and inside a Linux environment, and compare SHA-256. A mismatch stops the spec and returns to DESIGN before the later phases start
   - Tests: `TestReleaseToolchainStampsFromTag`, `TestBuildRefusesUnreleasableTag`, `TestArchiveLayout`, `TestArchiveLinuxFinalPaths`, `TestArtifactsReproducible`, `TestChecksumsCoverEveryAsset`
   - Files: `gotoolchain.go`, `compile/build.go`, `gate.go`, `build.go`, `archive.go`, `checksums.go`
9. **Phase: Packages and signing** -- nFPM vendored, deb and rpm with the launcher and the version directory, embedded scripts, unit from the systemd plugin, GPG signing of RPMs and `checksums.txt`, RPM hashing past the signature header
   - Tests: `TestPackagePayload`, `TestPackageUnitMatchesSystemdBuilder`, `TestLauncherIsOneFile`, `TestRPMHashSkipsSignatureHeader`
   - Files: `go.mod`, `vendor/`, `packages.go`, `scripts/`, `sign.go`, `compare.go`, `unit.go`
10. **Phase: Formula and changelog**
   - Tests: `TestFormulaCoversSupportedTargets`, `TestChangelogGroupsFragmentsByTopic`, `TestChangelogIgnoresCommitSubjects`, `TestChangelogRefusesUnparsableFragment`, `TestChangelogFirstRelease`
   - Files: `formula.go`, `changelog.go`
11. **Phase: GitHub** -- `draft`, the read-only `release-rebuild.yml`, `publish` with comparison and tap push, smoke tests
   - Tests: `TestDraftUploadsAndIsRerunnable`, `TestRebuildHashListMatchesBuild`, `TestPublishPublishesOnMatch`, `TestPublishRefusesMismatch`, `TestPublishRetriesTapOnly`, `TestReleaseWorkflowCallsRegisteredActions`
   - Files: `github.go`, `smoke.go`, `.github/workflows/release-rebuild.yml`, `workflowcheck_test.go`
12. **Phase: The release key on the site**
   - Tests: `TestSiteBuildPublishesReleaseKey`
   - Files: `internal/le/site/build.go`, `website/release/ze-release.asc`
13. **Phase: Rehearsal and docs** -- full release against a scratch repository with a test key; runbook; distribution spec amended
   - Files: `docs/contributing/releasing.md`, user docs, `plan/release/delivery/spec-release-distribution.md`

Each phase edits the pages its changes make wrong before the next phase starts (`ai/rules/documentation.md`).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Reproducibility | No artifact byte depends on the clock, hostname, umask, directory, user, or file-system order |
| Single declaration | Targets, tag set, unit text and public key are each declared once; every other surface derives from them |
| Correctness | Version comparison is numeric, part by part; the install-method check fails closed |
| One layout | No code path knows a real `ze` at `<prefix>/bin/ze`; the launcher, the tarball, the DEB and the RPM derive the layout from one declaration |
| No stored selection | No file or link records a selected version |
| Operator errors | Every refusal of the launcher and of the version commands says what is wrong, why, and ends with the command that works (`ai/rules/user-facing-errors.md`) |
| Rule: never destroy work | `build` writes only under `dist/<T>/` and refuses to overwrite a different existing build of the same tag |
| Rule: git safety | le never pushes a git ref: the tag is pushed by the owner; the tap is updated through the GitHub contents API by `publish`, which only the owner runs |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `./le build release` with `targets`, `build`, `draft`, `rebuild`, `publish`, `smoke` | `./le build release` lists them |
| Reproducible artifacts | `TestArtifactsReproducible` and a green `release-rebuild.yml` run on the rehearsal tag |
| Package-managed refusal | `test/ui/update-refused-package-managed.ci` |
| The release key on the site | `./le site build` output holds `release/ze-release.asc` and no `install.sh` |
| A tarball that installs by extraction | `test/ui/launcher-runs-latest.ci` |
| An update that installs a version directory | `test/plugin/selfupdate-installs-version-directory.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Signing key custody | The private key never leaves the hardware key; nothing in le or CI reads a private key file |
| CI privileges | `release-rebuild.yml` has `contents: read` and no secrets |
| Hash list binding | `publish` checks tag, commit and run before trusting any hash |
| Verification of a release | Whoever choice Q-2 names verifies the signature before the hash and the hash before any file is placed, and refuses, never skips, when a tool is missing |
| The launcher | It runs only a real directory with a real `ze`, never follows a link in the Ze home, and passes its arguments unchanged. It checks no owner and no caller: root runs the latest version like every caller (N-6, taken as (b)), so a compromised daemon chooses the program root runs, a risk accepted and stated in `docs/guide/ze-install.md`. The owners the service accepts are Q-4's (AC-49) |
| The update's archive | No member lands outside the staged version directory; no device, no set-user or set-group mode; the statement is verified before the download |
| Who can write what Ze runs | The launcher and every directory above the Ze home are root's. What the `ze` user may own is the Ze home and below, as choice Q-4 settles |
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
| `YY.MM.DD[.N]` with numeric comparison | `YYYY.MM.DD[.N]` (2026-09-28); `YY.MM.DD` only (distribution spec); semver | Owner choice, 2026-11-28; `.N` sorts correctly in dpkg, rpm, Homebrew and Ze |
| GPG on a hardware key | Sigstore keyless; both | Owner choice; rpm and apt users know GPG |
| Sign RPMs as well as `checksums.txt` | Unsigned RPMs | Fedora 45 refuses unsigned RPMs |
| Homebrew formula | Cask | Only a formula has `brew services`; casks are quarantined |
| nFPM v2.47.0 as a library | GoReleaser, hand-written deb and rpm, debhelper | Owner choice, confirmed 2026-11-28; avoids a second build model; pinned because its RPM backend is changing |
| Install-method file under the prefix | Build flag; env var | One binary for every channel keeps the hash comparison possible |
| Homebrew rule in `ConfigDirFromBinary` | Formula wrapper setting `ze.config.dir` | Covers all 11 callers and keeps the binary a binary |
| Release gate is a green GitHub verify run on the tagged commit | Local verify certificate; none | Owner choice; works from any machine |
| Package installs but does not start ze | Automatic init | `ze init` has no unattended mode; the unattended bootstrap stays in spec-release-distribution |
| One directory per release, a launcher script, the latest runs unless a variable names another | One file; a `current` link; a marker file | Owner choice, 2026-11-28 |
| The tarball holds final paths; no install.sh | A top directory moved by an installer | Owner choice, 2026-11-28 |
| The `ze` user installs versions | Root installs; a root helper | Owner choice, 2026-11-28 |
| The VPP bundle is a pinned input of the release build | Built by the release build | A native C build is not byte-identical on two hosts, and the publish gate compares bytes |
| The launcher carries a shell copy of the release order, held to the Go one by a test | The launcher calls `ze` to order versions | The launcher is what finds `ze`. The `le` script already works this way |

## Known Limitations

- Windows, FreeBSD, OpenBSD and every 32-bit target are listed but unsupported until Ze compiles there; each port is separate work
- No APT or DNF repository, no nightly channel, no package lifecycle beyond install and remove: spec-release-distribution
- macOS binaries are not signed or notarised by Apple; Homebrew avoids the quarantine block, a direct download does not
- A package install holds one version: the package manager replaces it at each upgrade
- The appliance holds one version and takes none of the version commands
- The launcher needs a POSIX `sh`, and `ls` if choice Q-4 makes it check owners

## Work Not Done

| Item | Why not here | Spec that owns it |
|------|--------------|-------------------|
| Amended on 2026-11-28, as plain consequences: its Task no longer lists install.sh and names the release form `YY.MM.DD[.N]`; its mechanism row for this spec; AC-9 and AC-10 declare the launcher and the version directory with its VPP. Still owed there, in its historical record, which is kept as history: the sentence "The package must install only the normal `ze` Linux daemon and its operational support files", the tarball layout `ze-<id>-linux-<goarch>/` holding only `ze` and `LICENSE`, "no correction suffix", the amendment row naming `YYYY.MM.DD`, and every step that reads `/usr/bin/ze` as the program | That spec owns the package lifecycle. Its historical record is not rewritten | `plan/release/delivery/spec-release-distribution.md` |
| The signed statement signs the size and SHA-256 of a release archive, in place of one binary, and `download-url` and `<base>/<goos>/<goarch>` serve that archive. Its "Behavior to preserve" keeps the `.prev` rollback, which the owner replaced on 2026-11-28 with the removal of a release. Its AC-12 re-hashes a held binary, which becomes a held archive. `ze update serve` serves a manifest and the binary of its own executable, and must serve an archive it is given. Its phase 6 depends on this spec for official statements | That spec is `ready`, P0 and approved. Its approval covers its Task, criteria and design tables, so an edit there needs the owner. Not edited by this work | `plan/release/fixes/spec-selfupdate-manifest-authenticity.md` |
| The container proof of a managed VPP start, run as the `ze` user under the unit's capability set (A-9, R-17) | The proof is the sibling spec's | `plan/release/delivery/spec-vpp-embedded-runtime.md` |
| The name of a nightly, before one can be a version directory (Q-11) | Nightlies are after the first release | `plan/post-release/features/spec-release-repositories-and-nightlies.md` |

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/delivery/spec-release-build.md` only, in the same `./le commit create` script
