# Spec: selfupdate-manifest-authenticity

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P0 |
| Release reason | Unauthenticated binary and image replacement is a security release blocker: today a manifest server, or anyone holding a device's update token, decides what code a Ze box runs. |
| Scope | protocol |
| Depends | `plan/release/delivery/spec-release-build.md` (phase 6 only: official releases emit update statements through `./le build release`) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner's order (2026-11-15): "improve the self update, find or write a spec".
This spec owns the authenticity of every path by which new code reaches a
running Ze box:

| Path | Entry point | Who decides the code today |
|------|-------------|----------------------------|
| Daemon self-update, automatic | `system update-check` with `auto-apply` | The manifest server: it supplies both the binary and the SHA-256 the binary is checked against |
| Daemon self-update, manual | `update system firmware download` and `apply` | The manifest server, and with no `sha256` in the manifest nobody: the binary is installed unverified |
| Version check, notify only | `system update-check` without `auto-apply` | The manifest server, which can raise a false "newer firmware available" warning |
| Appliance image push | `ze appliance push <name>` into gokrazy's `/update/*` handlers | Anyone holding the device's update token |
| gokrazy code paths beside the push | gokrazy's `/divert` and `/uploadtemp`, and the update handlers through Ze's `/gokrazy/` web proxy | Anyone holding the token, and any Ze web account whose profile grants `update system firmware apply`, because the proxy adds the token to each request its gate forwards. The proxy never forwards `/divert` or `/uploadtemp` |

Findings, each read at the producer on 2026-11-15:

1. `extendedManifest` (`internal/component/config/system/selfupdate.go`)
   carries `version`, `sha256`, `size`, `paused`, `minimum-version` and
   `download-url`, and no signature. `download` compares the binary's digest to
   the manifest's, so the digest and the binary share one authority.
2. `manualDownload` and `manualApply` log "no checksum available, binary will
   not be verified" and carry on.
3. The self-update `http.Client` (`newSelfUpdater`, `newUpdateChecker`) sets no
   `CheckRedirect`, so it follows redirects to any host, `https` to `http`
   included. `internal/core/fetch` `sameOrigin` already refuses that for the
   installer and `ze init --from`.
   Fixed by 5b800849ee: both now fetch through one client whose `CheckRedirect` is `fetch.SameOrigin` (AC-10).
4. `isNewer` (`update.go`) is a second release comparator beside
   `version.CompareReleases` (`internal/core/version/version.go`).
5. `ze update serve` (`internal/plugins/update-cmd/cmd/serve.go`
   `updateServeContent.handler`) computes an unsigned manifest from its own
   running binary. The skeleton named a `cmd/ze` file for it; that file no
   longer exists.
6. `ze appliance push` (`internal/appliance/cmd_push.go` `pushOne`) pins TLS to
   the stored `cert.pem`, sends the token as basic auth, refuses redirects and
   validates GPT, FAT and SquashFS structure (`push_image.go`). Its `.sha256`
   check is optional (`verifyImageChecksum` returns nil when the sidecar is
   absent). The device side is upstream gokrazy (`update.go` in the pinned
   modcache): `/update/root` streams straight into the inactive partition,
   `/update/boot`, `/update/bootonly` and `/update/mbr` write the boot
   partition shared by both slots, and `/divert` runs an uploaded binary. None
   of them checks who built the bytes.
7. The token is gokrazy's HTTP password (`build_update.go` `gokUpdate`
   `HTTPPassword`). It lives on the build host and on the device.
   `internal/component/gokrazy` `handlerWithAuth` adds it to each request its
   gate (`gate.go` `pageFor`) forwards to the management socket: `/update/...`
   only for an account granted `update system firmware apply`, and never
   `/divert`, `/uploadtemp` or `/poweroff`.
8. `buildOne` (`internal/appliance/cmd_build.go`) stamps `ZeVersion: "dev"`
   into both build manifests, so an image carries no release order today.

Correction to the skeleton's original claim, kept: a spoofed server is mostly
answered already, because `ValidateUpdateCheckURL` calls
`config.ValidateFetchURL` (HTTPS, or HTTP to loopback only). What remains is a
compromised or malicious server, a token holder, and trust that ends at the
Web PKI or at the token instead of at the publisher.

Out of this spec, homed elsewhere (see Work Not Done): Secure Boot, dm-verity
and the authenticity of installer media (ISO, PXE install through
`ze install disk`, whose `run` logs "no image checksum available, skipping
verification" and installs) go to
`plan/post-release/features/spec-kernel-lockdown-hardening.md`, because no
running trusted image exists there to anchor a key. Release packages, their GPG
signatures and the daemon release SBOM stay in
`plan/release/delivery/spec-release-build.md` and
`plan/release/delivery/spec-release-distribution.md`. That spec's Excluded list
names appliance images, installer kernels, initrds and ISOs, and since
2026-11-15 it names their owners: this spec for appliance images and the update
statements, the lockdown spec for installer media.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/self-update.md` - the self-updater state machine
  → Decision: the maintenance window gates replacement only; download and verification run at any time, so verification must finish before the held file is recorded as verified
  → Constraint: manual apply and download bypass pause, spread and window; they MUST NOT bypass verification
- [ ] `docs/architecture/config/system-update.md` - one `UpdateBackend` registry, picked by platform
  → Constraint: `backend_ze_distro.go` (`ze_distro`) wraps checker and updater; `backend_ze_appliance.go` is the minimal stub; a gokrazy device answers through `gokrazyBackend`, which probes `/update/features` over the management socket
  → Decision: version comparison stays the release comparator; `isNewer` is replaced by `version.CompareReleases` so there is one
- [ ] `docs/architecture/appliance/ota-push.md` - the push protocol through `github.com/gokrazy/updater`
  → Constraint: a push mock must speak the whole protocol, features probe included, or it passes while the probe is broken
  → Constraint: an image in the field is the contract: a new host must still push to a device that runs the previous image
- [ ] `docs/architecture/appliance/gokrazy-build-pins.md` - pinned gokrazy, `gokrazy/patches/*.patch`
  → Constraint: a runtime change to gokrazy is a reviewed patch against the pinned source, guarded by `TestPinnedGokrazyRuntimeMatchesUpstream`; an upstream bump re-reviews it
- [ ] `docs/architecture/appliance/builder.md` - build manifest, `.sha256` sidecar, secrets at rest
  → Constraint: build secrets go through the instance's encrypted secret files (`secretFilePath`, `WriteSecret`, `readSecret`); the image signing key is one more such secret
- [ ] `docs/architecture/appliance/on-device-installer.md` - `internal/core/fetch`
  → Decision: the install path has no running trusted image to anchor a key, so its authenticity belongs to the boot-chain work, not here
- [ ] `plan/release/delivery/spec-release-build.md` - owner rulings of 2026-09-28
  → Constraint: release checksums are GPG-signed on the maintainer's hardware key, no signing key exists in CI, Sigstore is not used
- [ ] `plan/release/delivery/spec-release-distribution.md` - signing trust domains, SPDX 2.3 SBOMs, package-managed update guard
  → Constraint: one trust domain per primary key; current and next public keys ship before a rotation
- [ ] `plan/post-release/features/spec-kernel-lockdown-hardening.md` - module, kexec and Secure Boot keys
  → Decision: its stable kexec image key signs a PE/COFF kernel for the kernel keyring; it does not authenticate an update channel and is not reused here
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every refusal is an operator error: problem, why, and the exact fix alone on the last line; every Warn line and report raise names a registered doctor code

### RFC Summaries (Scope: protocol)
- No RFC governs this. The signature format is OpenSSH's SSHSIG (`PROTOCOL.sshsig` in OpenSSH), its keys are RFC 8709 Ed25519 SSH keys. Ze enrols no RFC summary for it (`ai/META-DECISIONS.md`: RFC validation scope does not grow); the interop proof is against OpenSSH's `ssh-keygen -Y`.
  → Constraint: the signed blob is the magic `SSHSIG`, namespace, reserved string, hash algorithm and the message hash; a signature verifies only under the namespace it was made with

**Key insights:**
- The device side of the appliance push is gokrazy, not Ze: authenticity there needs a gokrazy patch, and it must keep working when the Ze daemon is down.
- A signed image is worth nothing while `/divert` and `/uploadtemp` run unsigned code for the same token holder.
- No project-operated self-update feed exists; the release key matters for operators who mirror official releases, and operator keys matter for everyone else.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/selfupdate.go` - `check`, `download`, `manualDownload`, `manualApply`, `fetchManifest`, `resolveDownloadURL`, `downloadBinary`, `newSelfUpdater`
- [ ] `internal/component/config/system/update.go` - `UpdateChecker`, `ValidateUpdateCheckURL`, `fetchVersion`, `isNewer`
- [ ] `internal/component/config/system/backend.go`, `backend_ze_distro.go`, `backend_gokrazy.go` - backend registry and the gokrazy feature probe
- [ ] `internal/component/config/system/doctor.go` - `checkUpdateCheckURL`, `codeUpdateCheckUnreachable`
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `container update-check`
- [ ] `internal/plugins/update-cmd/cmd/serve.go` - `serveManifest`, `updateServeContent`
- [ ] `internal/appliance/cmd_push.go` - `pushOne`, `verifyImageChecksum`, `loadDeviceTLS`, `authTransport`, `refusePushRedirect`
- [ ] `internal/appliance/push_image.go` - `readPushImage`, `readPushPartitions`, `readPushBoot`, `readPushRoot`
- [ ] `internal/appliance/manifest.go`, `cmd_build.go` - `BuildManifest`, `writeImageChecksum`, `buildOne`
- [ ] `internal/appliance/build_update.go`, `cmd_init.go` - the update token is gokrazy's `HTTPPassword`, generated at init and stored with `WriteSecret`
- [ ] `internal/appliance/doctor_checks.go` - `applianceDoctorChecks`
- [ ] `internal/component/gokrazy/gokrazy.go`, `gate.go` - `handlerWithAuth` adds the password only to a request `pageFor` forwards for the account's authorization
- [ ] `internal/core/fetch/fetch.go` - `sameOrigin`
- [ ] `internal/core/version/version.go` - `CompareReleases`, `ImageInfo`
- [ ] `internal/core/authorizedkeys/authorizedkeys.go` - OpenSSH public-key line parsing through `golang.org/x/crypto/ssh`
- [ ] `internal/le/test/qemu/install_ota.go` - `executeApplianceOTA`
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go` - the device handlers; `gokrazy/patches/reboot-ack.patch` - the patch shape

**Behavior to preserve:**
- HTTPS-only fetch with the loopback exception (`config.ValidateFetchURL`), for the manifest URL and `download-url`
- Pause, spread, maintenance window, restart policy, `.prev` rollback, the 20-event history under `meta/config/update-history`
- `X-Ze-Arch` request header and the server's 404 on an architecture mismatch
- The push protocol order (root, bootonly, mbr, switch or testboot, reboot), TLS pinning to `cert.pem`, basic auth, redirect refusal, structure validation before contact, and pushing to a device that runs an image from before this spec
- `ze.test.update.running.version` as the only test override of the running release

**Behavior to change:**
- Every self-update and version-check answer is accepted only with a valid signature from a trusted key; the unverified manual path is deleted
- The manifest's signed fields move into a signed statement; only `paused` and `download-url` stay unsigned
- Self-update HTTP follows only same-origin redirects
- `ze update serve` serves a signed statement and refuses to start without one
- `ze appliance build` signs every image and writes its SPDX SBOM; `ze appliance push` refuses an image without a valid signature
- A Ze image's gokrazy refuses any root, boot or MBR write and any switch not covered by a statement its running image trusts, and refuses `/divert` and `/uploadtemp`

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Payload digest check | `selfupdate.go` `download`, `hashFile` | Compares the binary's SHA-256 to the manifest's | Extends: the digest and size now come from the signed statement |
| Unverified manual path | `selfupdate.go` `manualDownload`, `manualApply` | Warns and installs when the manifest has no `sha256` | Replaces: deleted; all three paths call one verify step |
| HTTPS fetch rule | `config.ValidateFetchURL` via `ValidateUpdateCheckURL` | Refuses plain HTTP except loopback | Keeps unchanged: transport protection stays, authenticity is added above it |
| Same-origin redirects | `internal/core/fetch` `SameOrigin`; `system/update.go` `newUpdateClient` | Exported and set as `CheckRedirect` on both the self-updater and update-checker clients (landed in `5b800849ee`) | Keeps unchanged: AC-10 is already implemented; the signed-statement verifier runs on the responses it fetches |
| Release comparator | `version.CompareReleases`, `version.IsValidRelease`; `update.go` `isNewer` | Two comparators for one fact | Replaces `isNewer` with `version.IsValidRelease(running)` + `version.CompareReleases`, preserving the `dev`/`unknown` short-circuit while carrying downgrade refusal |
| Build version metadata | `internal/core/version/version.go` `Release`, `BuildDate` | Stamps release version and build date, with no build-flavor accessor | Extends: adds build-tag-driven `Flavor()` (`distro`, `appliance`, `dev`) so `VerifyBinary` refuses statements built for another flavor |
| YANG custom validators | `internal/component/config/validators_register.go`, `validate_sections.go` `validatedSections` | Registers `ze:validate` callbacks; `system` is already in `validatedSections` | Extends: registers `update-trusted-key` applying the `authorizedkeys` one-line rule and refusing non-Ed25519 keys |
| Map leaf-list extraction | `internal/core/configvalue` `LeafList` | Normalizes `(*Tree).ToMap` leaf-list delivery (absent, bare `string` for 1 member, `[]string`/`[]any` for 2+) into `[]string` | Extends: `ExtractUpdateCheckFromMap` (`system/update.go`) reads `trusted-key` through `configvalue.LeafList` so a single configured key is never dropped |
| Start-time service block isolation | `internal/component/config/service_blocks.go` `RegisterServiceBlock`, `LoadConfigForStart` | Drops registered service blocks whose validation fails at `ze start` under `doctor-service-not-started` so networking stays up; `system/update-check` is not registered today | Extends: registers `system/update-check` via `config.RegisterServiceBlock("system/update-check", "system update-check")` in `system/register.go` so an invalid `trusted-key` in stored config never blocks `ze start` while `ze config validate`, commit, and reload still refuse it |
| Sequential upgrades | manifest `minimum-version` | Refuses a skip past a required release | Extends: moves into the signed statement, so a server cannot strip it |
| Update server | `update-cmd/cmd/serve.go` `updateServeContent` | Serves an unsigned manifest computed from the running binary | Extends: serves the signed statement files beside the binary |
| Version checker | `update.go` `fetchVersion`, `versionManifest` | Reads `version` and raises a warning | Extends: parses the same manifest through the same verifier |
| Device TLS pinning | `cmd_push.go` `loadDeviceTLS` | Authenticates the device to the host | Keeps: it authenticates the channel; the statement authenticates the content |
| Token basic auth | `cmd_push.go` `authTransport` | Authorises a client to talk to gokrazy | Keeps: no longer sufficient to install code |
| Redirect refusal on push | `cmd_push.go` `refusePushRedirect` | Keeps credentials on the named device | Keeps unchanged |
| Image structure validation | `push_image.go` `readPushImage` | Extracts and bounds root, boot and MBR payloads | Extends: the same extraction computes the part digests the statement signs, at build and at push |
| Optional image checksum | `cmd_push.go` `verifyImageChecksum` | Checks a `.sha256` sidecar when present | Replaces for push: the signed statement is mandatory; the sidecar stays only for the install server, owned by the lockdown spec |
| Build manifest | `manifest.go` `BuildManifest`, `/perm/ze/build.json` | Records appliance, timestamp, version (`dev`), arch, image digest | Keeps: the statement is written beside it and carries its own signed fields |
| gokrazy patch series | `gokrazy/patches/`, `TestPinnedGokrazyRuntimeMatchesUpstream` | Applies reviewed diffs to the pinned runtime | Extends: a second patch, `signed-update.patch` |
| gokrazy update handlers | pinned `update.go` | Write any payload a token holder sends; `/divert` runs uploaded code | Extends through the patch; `/divert` and `/uploadtemp` are replaced by a refusal |
| gokrazy feature probe | `backend_gokrazy.go` `probe`, `featureNames` | Reads `/update/features` | Extends: doctor reads the `ze-signed` feature from it |
| Ze's gokrazy web proxy | `internal/component/gokrazy` `handlerWithAuth`, `gate.go` `pageFor` | Forwards a gokrazy page with the password only when the account's profile grants the Ze command that does the same: `/update/...` needs `update system firmware apply`. It never forwards `/divert`, `/uploadtemp` or `/poweroff` (`docs/guide/appliance.md`, "The gokrazy management UI") | Does not apply: the device-side check covers every path that reaches the handlers, a forwarded `/update/...` included |
| SSH public-key parsing | `internal/core/authorizedkeys` `Parse` | Parses one OpenSSH key line at a time with `x/crypto/ssh` | Extends: the `trusted-key` leaf and the image key files are parsed by the same one-line rule |
| Release GPG signing | `spec-release-build.md`, Signing row | GPG on a hardware key signs `checksums.txt` and RPMs for people, `install.sh` and package managers | Does not replace: device statements are a separate trust domain, verified without an OpenPGP stack on the device. GPG stays for release checksums and RPMs (D-1) |
| Release SBOM | `spec-release-distribution.md`, SPDX 2.3 via pinned Syft in CI | SBOMs for daemon packages | Does not apply to images: an operator's build host has no Syft; the format, SPDX 2.3 JSON, is shared |
| Kernel image key | `spec-kernel-lockdown-hardening.md`, Key strategy | Signs the bzImage for `kexec_file_load` | Does not apply: another format and purpose |
| Update doctor checks and codes | `system/doctor.go` `checkUpdateCheckURL`; `system/register_doctor_codes.go` | Probes the URL and registers `system`-owned doctor codes via `diagnostic.Register` | Extends: adds `doctor-update-no-trusted-key`, `doctor-update-signature-invalid` (also registered via `diagnostic.RegisterEventCode`), and `doctor-image-verification-inactive` in `system/register_doctor_codes.go` |
| Self-update report raises | `selfupdate.go` `selfUpdateCode` | Raises and clears `self-update` warnings | Extends: a signature failure is raised under its own registered event code `doctor-update-signature-invalid` with a fix line |
| Appliance secrets and rekey | `internal/appliance` `secretFilePath`, `WriteSecret`, `readSecret`; `cmd_rekey.go` `runRekey`, `rekeyKeyFiles` | Stores `password.hash`, `update.token`, `key.pem`, and `ca-key.pem` encrypted under the passphrase; `runRekey` re-encrypts the hardcoded `secretFiles` and `keyPaths` lists | Extends: stores the per-instance image signing private key via `WriteSecret` beside `update.token` and adds it to `cmd_rekey.go` `rekeyKeyFiles` so `ze appliance rekey` re-encrypts it (skipping when absent on agent-key instances) |
| Appliance host doctor | `internal/appliance/doctor_checks.go` `applianceDoctorChecks`, `register.go` | Host checks for `ze appliance` | Extends: adds `doctor-appliance-no-signing-key` and registers its metadata via `diagnostic.Register` in `internal/appliance/register_doctor_codes.go` |
| QEMU OTA proof | `internal/le/test/qemu/install_ota.go` `executeApplianceOTA` | Builds two images and pushes the second to a real guest | Extends: the `ota-signed` steps |
| Installer fetch digest | `internal/core/fetch`, `internal/install/disk` `run` | Optional SHA-256 from the same server | Does not apply: homed in the lockdown spec |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads; the door covering `pkg/updatesig/verify.go`, `sign.go`, `statement.go` | "A change under `pkg/` is a change to a published contract. A type, a field, a callback name and the bytes on the wire all count." | agrees | `pkg/updatesig` is a deliberately published package with a documented wire contract (OpenSSH SSHSIG and JSON statements), zero `internal/` imports, and all new checks and codes register from owning-package `init()`. |
| `docs/architecture/appliance/self-update.md` | Decisions | `// Design:` and anchor of `selfupdate.go`, `serve.go`, `serve_daemon.go`; anchor of `update.go`, `storage_consumer_restart.go`; modified by this spec | "Manual apply and manual download bypass server-side pause, spread, and the maintenance window. Auto-apply requires a SHA-256 in the manifest; a manual command warns and proceeds without one." | departs | Deletes the unverified manual fallback so every self-update path (automatic, manual download, manual apply) and version check requires a valid Ed25519 SSHSIG over a signed statement from a trusted key. Approved by Thomas on 2026-11-16 (D-1..D-6); `docs/architecture/appliance/self-update.md` is updated in the same work. |
| `docs/architecture/core-design.md` | 20. System Update Backend | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "The system update surface is a single registered backend interface in `internal/component/config/system`. The hub selects `ze-self-update` for normal Linux, systemd, container, and Darwin platforms, and `gokrazy-ab` for gokrazy appliances." | agrees | Keeps the single `UpdateBackend` interface and both registered backends (`ze-self-update` and `gokrazy-ab`), extending `gokrazyBackend` only to expose `ze-signed` from `/update/features`. |
| `ai/doors/appliance.md` | Who depends on it from outside | the door covering `internal/component/config/system/*`, `internal/appliance/*`, `gokrazy/ze/config.json` | "An image already in the field is the contract. This applies to a change to a `/perm/ze/` path, a zefs key the image writes, or the push protocol. Each such change must still work against the device that runs the previous image." | agrees | `ze appliance push` probes `/update/features` and, when `ze-signed` is absent, warns the operator and pushes with the existing protocol sequence (AC-21) so field devices upgrade to signed enforcement. |
| `docs/architecture/config/system-update.md` | The version check | `// Design:` and anchor of `update.go`, `backend_gokrazy.go`; modified by this spec | "**Each request carries an `X-Ze-Arch` header**, so the server answers 404 for a mismatched architecture." | agrees | Preserves `X-Ze-Arch`, HTTPS-only `ValidateUpdateCheckURL`, report-bus warning reporting, and the `dev`/`unknown` short-circuit (via `version.IsValidRelease` + `version.CompareReleases`), while requiring SSHSIG verification of every version manifest. |
| `docs/architecture/resolve.md` | Where the files are read from | anchored to `update.go`, `ze-system-conf.yang` | "A source is HTTPS, or plain HTTP from the host itself." | agrees | Keeps `config.ValidateFetchURL` enforcing HTTPS (with loopback exception) on `update-check url` and manifest `download-url`, and leaves RIR delegation sources untouched. |
| `docs/features/ai-first.md` | Structured Diagnostics | `// Design:` of `system/doctor.go`; modified by this spec | "A transient code a component raises on the report bus (a plugin process that ended unasked, a refused plugin request) is registered by its owner as an event code, so `ze show diagnostic <code>` describes it and `show doctor` reports the event under it." | agrees | Registers `doctor-update-signature-invalid` via `diagnostic.RegisterEventCode` and `diagnostic.Register` in `internal/component/config/system/register_doctor_codes.go`, and registers the other three doctor codes in their owning packages. |
| `docs/architecture/doctor-and-health-checks.md` | Every finding names the command that fixes it | `// Design:` of `system/register_doctor_codes.go`; modified by this spec | "A doctor diagnostic carries `fix`: the command or config statement the operator runs to clear the problem, alone and with no label" | agrees | Every new doctor check (`doctor-update-no-trusted-key`, `doctor-image-verification-inactive`, `doctor-appliance-no-signing-key`) sets `Fix` on its `diagnostic.Diagnostic` literal, and `doctor-update-signature-invalid` passes its fix via `report.RaiseWarningFix`. |
| `docs/architecture/config/yang-config-design.md` | Where a validator actually runs | `// Design:` and anchor of `validators_register.go` | "Declaring a `ze:validate` is not sufficient to make it run. `ValidateCustomSections` iterates `validatedSections`, a list of top-level section names, and checks only inside those, so an annotation under any other section never executes." | agrees | Registers `update-trusted-key` in `validators_register.go` under `system`, which is already in `validatedSections`, so `ze config validate`, commit, and daemon load enforce it uniformly. |
| `ai/doors/config-yang.md` | Boundaries | the door covering `validators_register.go`, `validators_update.go` | "**The generated YANG glue is not hand-edited.** `./le yang glue update` regenerates each `yang/` package's `embed.go` and `register.go`." | agrees | Adds `trusted-key` to `ze-system-conf.yang` with native `max-elements 16` and `length` bounds plus `ze:validate "update-trusted-key"`, and updates generated YANG glue only through `./le yang glue update`. |
| `docs/architecture/api/commands.md` | A command that starts work outliving its request | anchored to `version.go`, `serve_daemon.go` | "A builtin handler can start work that keeps running after it answers: `update serve start` opens a listener and serves it from a goroutine. No config transaction and no plugin process owns that work, so the handler binds it to the daemon's plugin server with `CommandContext.Server.OnStop(fn)` before it starts the goroutine." | agrees | `updateServeOwner.start` refuses missing `.statement`/`.statement.sig` files before opening any listener or registering `OnStop`, keeping the daemon shutdown hook contract unchanged. |
| `docs/architecture/config/syntax.md` | What a reader receives | anchored to `version.go` | "Read all three through `internal/core/configvalue` (`Bool`, `Int`, `LeafList`, `ListEntries`) rather than asserting a type." | agrees | `ExtractUpdateCheckFromMap` (`internal/component/config/system/update.go`) reads the `trusted-key` leaf-list through `configvalue.LeafList` so one member (bare `string` from `(*Tree).ToMap`) and multiple members (`[]string`/`[]any`) both parse identically, while `version.Flavor()` leaves `CompareReleases` and schema stamps unchanged. |
| `docs/architecture/web-interface.md` | Starting the Web Server | anchored to `version.go` | | silent | `web-interface.md` anchors `version.HTTPHeaderHidden` for the `X-Ze-Version` HTTP header; adding `version.Flavor()` changes no web header or route. |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/version/version.go`, `flavor_distro.go`, `flavor_appliance.go`, `flavor_other.go` | "**Never import upward.** Where core needs something a component owns, the component registers into core. Do not add a row to the core import baseline." | agrees | `internal/core/version` adds build-tag `Flavor()` files using standard-library imports only and zero upward imports into `internal/component/` or `internal/plugins/`. |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/update-cmd/cmd/serve.go`, `serve_daemon.go` | "**A plugin owns its whole surface.** Its commands, YANG, doctor checks and help live in its folder." | agrees | `update serve` changes stay inside `internal/plugins/update-cmd/` with no central switch edits or cross-package coupling. |
| `docs/architecture/appliance/builder.md` | Encryption | `// Design:` and anchor of `cmd_build.go`, `manifest.go`, `cmd_init.go`, `main.go`, `cmd_rekey.go`; modified by this spec | "Secrets are encrypted with a key derived by Argon2id and sealed with XChaCha20-Poly1305. Files are written atomically and keys are zeroed after use." | agrees | Stores the per-instance image signing private key through `WriteSecret`/`readSecret` beside `update.token`, adds it to `cmd_rekey.go` `rekeyKeyFiles` so `ze appliance rekey` re-encrypts it alongside `update.token`, `key.pem`, and `ca-key.pem`, zeros key material after use, and writes `.statement`, `.statement.sig`, and `.spdx.json` beside the image. |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Why a module cache is checked in | anchored to `cmd_build.go`, `gokrazy_runtime_patch_test.go`; modified by this spec | "The patch guard checks both instance pins and compares the complete maintained runtime source and assets with upstream after reversing that diff." | agrees | Adds `gokrazy/patches/signed-update.patch` to the single patched runtime tree guarded by `TestPinnedGokrazyRuntimeMatchesUpstream`, keeps one runtime tree (D-4), and adds the `pkg/updatesig` replace to both `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod` and `gokrazy/docker-host/builddir/github.com/gokrazy/gokrazy/go.mod` (rewritten generically by `absolutizeReplaces`), keeping `./le build gosum` and `TestPreparedModulesResolveIdenticallyToTracked` green. |
| `docs/architecture/appliance/ota-push.md` | Decisions | `// Design:` and anchor of `cmd_push.go`; anchor of `cmd_build.go`, `install_ota.go`; modified by this spec | "Stream bounded root, `bootonly`, and MBR boot-code payloads in that order" | agrees | Preserves TLS pinning, basic auth, redirect refusal, structure validation, and root/bootonly/mbr order; verifies `.statement`/`.statement.sig` locally before contact, sends `PUT /update/ze-statement` first when `/update/features` advertises `ze-signed`, and warns on older devices. |
| `docs/architecture/appliance/remote-operations.md` | Constraint the code does not state | anchored to `cmd_push.go` | "Error messages differentiate an unreachable device from a protocol error." | agrees | Device-side statement and digest refusals map to `protocolError` naming the refused part and fix rather than reporting the device unreachable. |
| `docs/architecture/appliance/build-artifacts.md` | Decisions | `// Design:` and anchor of `doctor_checks.go`; anchor of `cmd_init.go`; modified by this spec | "Doctor checks register from the package `init()`, the same way the plugin doctor registers." | agrees | Registers `doctor-appliance-no-signing-key` from `internal/appliance` `init()` beside existing host checks and documents the `.statement`, `.statement.sig`, and `.spdx.json` build artifacts. |
| `docs/architecture/appliance/iso-installer.md` | Decisions | anchored to `doctor_checks.go` | "A missing `.sha256` sidecar is a hard error, with no override flag. An ISO is a durable offline artifact, and enforcement is what stops corrupt media from being handed around." | agrees | `ze appliance build` keeps writing `<img>.sha256` beside every image so `ze appliance iso` retains its mandatory `.sha256` sidecar enforcement unchanged (AC-15); only `ze appliance push` replaces its optional `.sha256` check with mandatory `.statement`/`.statement.sig` verification, while installer-media authenticity stays homed in `spec-kernel-lockdown-hardening.md`. |
| `docs/architecture/appliance/command-provider.md` | Decisions | `// Design:` and anchor of `internal/appliance/register.go` | "Dispatch goes through the registry lookup, so no appliance spelling remains in the central switch." | agrees | New `ze appliance key` and `ze appliance sign` subcommands register inside `internal/appliance/main.go` under the existing root handler with no edits to any central switch. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `gokrazy_runtime_patch_test.go`, `install_ota.go`, `storage_consumer_restart.go` | "**Never import `internal/le` from product code**, not even for a helper." | agrees | Signing and verification live in `pkg/updatesig` and `internal/appliance`; `internal/le` only guards `signed-update.patch` and drives the QEMU `ota-signed` proof. |
| `docs/architecture/testing/qemu-integration.md` | Appliance image update and recovery | `// Design:` and anchor of `install_ota.go`; modified by this spec | "`./le test qemu appliance-ota` is a manual deployment proof. It builds two named images for the host's architecture, boots the first disk, and publishes a host name through Ze's authenticated HTTPS configuration upload API." | agrees | Extends `executeApplianceOTA` with `ota-signed` steps proving AC-17..AC-20 while preserving its existing build, boot, health, and recovery checks. |
| `docs/architecture/testing/ci-workflows.md` | Where each suite runs | anchored to `install_ota.go` | "`./le test qemu appliance-ota` is a manual appliance-delivery proof, outside the scheduled populations above." | agrees | Keeps the extended `ota-signed` OTA proof in the existing manual `./le test qemu appliance-ota` action without changing CI workflows. |
| `docs/architecture/storage-backends.md` | Opening and ownership | `// Design:` of `storage_consumer_restart.go` | | silent | `storage-backends.md` covers the live store format and ownership; this spec only signs the mock manifest in `storageHistoryRestart` and configures `trusted-key` without altering store semantics. |
| `docs/architecture/ddos/ddos-detect-enhancements.md` | Confidence | anchored to `storage_consumer_restart.go` | | silent | `ddos-detect-enhancements.md` anchors `storageDDoSRestart` in `storage_consumer_restart.go`; this spec modifies only `storageHistoryRestart` and leaves DDoS restart behavior unchanged. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Touches no VPP or Linux kernel forwarding feature and leaves the VPP/kernel commit-time backend gate unchanged. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Touches no VPP/kernel mismatch verdict (and respects its 2026-11-26 start-time supersession below). |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Touches no firewall YANG annotation or VPP firewall verifier. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Touches no data-plane backend selector or `ze:backend` value. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Touches no BGP session or export filter-chain code. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Touches no BGP policy chain or `modify` block. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Touches no BGP engine filter compilation or `policychain.Chain`. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Touches no BGP UPDATE forwarding or `wireOverride` path. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Touches no BGP next-hop validation or MP_REACH encoding. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Touches no BGP route origination or forwarding path. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Touches no BGP filter chain or `\| implied` display seam. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Registers `system/update-check` via `config.RegisterServiceBlock("system/update-check", "system update-check")` in `internal/component/config/system/register.go` so `LoadConfigForStart` drops an invalid `system/update-check` block (for example a malformed `trusted-key`) under `doctor-service-not-started` instead of exiting `ze start`, while `ze config validate`, commit, and reload still refuse it as a whole; missing trusted keys or failed manifest signatures also leave `ze start` running and report `doctor-update-no-trusted-key` or `doctor-update-signature-invalid`. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `selfupdate.go`, `update.go`, `ze-system-conf.yang`, `backend_gokrazy.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Follows the same start-resilience rule: `config.RegisterServiceBlock("system/update-check", "system update-check")` keeps `ze start` running when `system/update-check` carries an invalid `trusted-key` and reports `doctor-service-not-started` with its fix, while `ze config validate`, commit, and reload refuse it. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `version.go`, `serve_daemon.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no daemon command subcommand for scope or presentation; `show system update` adds the signing key fingerprint through its existing structured renderer, and `ze appliance key`/`sign` are offline build-host commands. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `version.go`, `serve_daemon.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Adds no command-level pipe override. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `version.go`, `serve_daemon.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Adds no pipe operation and preserves existing pipe semantics on `show system update` and `update serve`. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Daemon: the bytes of `version.json` from the configured `update-check url`, then the binary from `download-url` or `<base>/<goos>/<goarch>`
- Appliance: an image file and its `.statement` and `.statement.sig` on the build host; on the device, HTTP bodies on gokrazy's `/update/*`

### The signed statement

One format, two kinds, signed as exact bytes (no canonical form is needed, because the verifier parses only bytes it has verified). A statement is JSON; unknown fields are refused.

| Field | Type | Kind | Description |
|-------|------|------|-------------|
| `format` | string | both | `ze-update-statement-1` or `ze-image-statement-1`; anything else is refused |
| `version` | string | both | The Ze release the payload carries; for an image, read from the Go build info of the `ze` binary in the root, informational only |
| `os`, `arch` | string | binary | Must equal the running `runtime.GOOS` and `runtime.GOARCH` |
| `flavor` | string | binary | The build flavor (`distro`, `appliance`); must equal the running flavor |
| `sha256`, `size` | string, integer | binary | The binary's digest and exact length; the download is cut at `size` plus one byte |
| `minimum-version` | string | binary | As today, now signed |
| `appliance` | string | image | The instance name the image was built for; must equal the device's |
| `arch` | string | image | The image architecture |
| `issued` | string, UTC timestamp | both | Signing time. For an image it is the downgrade floor: the device refuses a statement not strictly newer than the last it accepted |
| `image` | object: `sha256`, `size` | image | The whole GPT image |
| `root`, `boot`, `mbr` | object: `sha256`, `size` | image | Each push payload as `readPushImage` extracts it, the boot payload as sent, before the device rewrites its command lines |
| `sbom` | object: `sha256`, `name` | image | The SPDX file shipped beside the image |

The signature is an armored SSHSIG over the statement bytes, hash `sha512`, namespace `ze-update@ze-software.net` for binaries and `ze-image@ze-software.net` for images, so a signature of one kind never verifies as the other. Accepted key type: `ssh-ed25519` only (D-6). Any other key type is refused, naming the type.

### The manifest on the wire

| Field | Signed | Description |
|-------|--------|-------------|
| `statement` | it is the signed bytes, base64 | The binary statement |
| `signature` | - | The armored SSHSIG |
| `paused` | no | Server-side hold; a server can always withhold, so signing it buys nothing |
| `download-url` | no | Where to fetch; the signed digest decides acceptance, and the URL still passes `ValidateFetchURL` |

The old top-level `version`, `sha256`, `size` and `minimum-version` are deleted (pre-release: no compatibility form, `ai/META-DECISIONS.md`).

### Trust roots

| Domain | Private key | Signs | Device trust root |
|--------|-------------|-------|-------------------|
| Ze release update key | The maintainer's `ssh-ed25519` key, reached only through an SSH agent on the maintainer's machine, never in CI (D-6): a passphrase-protected key file loaded into `ssh-agent`, or the authentication subkey of the OpenPGP hardware key that signs releases, served by `gpg-agent`'s SSH support | Binary statements of official releases, one per target | Public keys compiled into every build from `pkg/updatesig/releasekeys/` (current and next) |
| Operator update key | The operator's, made by `ze appliance key generate` or `ssh-keygen` | Binary statements for binaries the operator serves | `system update-check trusted-key` leaf-list; when present it replaces the compiled-in release keys |
| Appliance image key | Per appliance instance, generated by `ze appliance init`, stored with `WriteSecret` beside `update.token`, or an agent key the instance names | Image statements | The key file baked into the root filesystem of every image built from the instance (current and next), so the running image judges the next one |

A dev build carries the same compiled-in release keys (they are public) and, like every build, trusts only them or its configured keys. Tests sign with a fixture key and configure it as `trusted-key`.

### Transformation Path (daemon)
1. `ExtractUpdateCheckFromMap` (`internal/component/config/system/update.go`) reads `trusted-key` through `configvalue.LeafList` (`internal/core/configvalue`) so one configured key (bare `string` from `(*Tree).ToMap`) and multiple keys (`[]string`/`[]any`) both normalize uniformly. `fetchManifest` reads the HTTP body (existing bound), with the same-origin redirect policy (`fetch.SameOrigin` on `newUpdateClient`, landed in `5b800849ee`).
2. `updatesig.VerifyBinary` decodes `statement` and `signature`, picks the trusted set (configured `trusted-key` entries when present, else compiled-in release keys), verifies the SSHSIG (namespace, `ssh-ed25519` key type, key membership, Ed25519 signature), then parses the statement strictly.
3. The updater compares `os`, `arch`, `flavor` (via `version.Flavor()`), checks `version.IsValidRelease(running)` so `dev`/`unknown` builds stay "not newer", compares `version` against the running release with `version.CompareReleases` (strictly newer), then checks `minimum-version`.
4. `downloadBinary` stops at the signed `size`; `download` compares the SHA-256 to the signed digest.
5. `attemptStage` re-hashes the held file before the rename, so a file changed during the maintenance wait is refused.
6. `check`, `manualDownload` and `manualApply` share steps 2 to 5; `UpdateChecker` shares steps 2 and 3 for its warning.

### Transformation Path (appliance)
1. `ze appliance build` writes the image, then `writeImageStatement` extracts the payloads with `readPushImage`, hashes them, writes `<img>.spdx.json` from the Go build info of each binary in the root and the kernel pin, writes `<img>.statement`, signs it into `<img>.statement.sig`.
2. `ze appliance push` verifies the signature against the instance's public keys and every digest against the image file it opened, before contacting the device.
3. The push probes `/update/features`. A device advertising `ze-signed` first receives the statement and signature at `PUT /update/ze-statement`. A device without it runs an image from before this spec: the host prints that the device does not verify updates, that this push installs an image that does, and pushes as today.
4. The patched gokrazy verifies the statement against the running root's key file and the persisted floor, then for each payload: root is hashed while it streams and judged at its end; boot, bootonly and MBR are buffered up to the signed size and written only after their digest matches.
5. `/update/switch` and `/update/testboot` refuse unless every listed payload matched in this session; on success the device writes the accepted statement and floor to `/perm/ze/image-statement.json`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Update server to device | HTTPS JSON manifest, binary body | No |
| Build host to device | gokrazy update protocol plus `PUT /update/ze-statement` | No |
| gokrazy init to Ze code | the patch imports the public `pkg/updatesig`, so verification runs when the Ze daemon is down | No |
| Daemon to `ze doctor` | report-bus codes and the gokrazy feature probe | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `cmd/`, `test/`, `gokrazy/`, and `scripts/` on 2026-11-26 for `version.json`, `extendedManifest`, `versionManifest`, `serveManifest`, `minimum-version`, `X-Ze-Arch`, `update-check`, `update serve`, `/update/features`, `/update/root`, `/update/boot`, `/divert`, `/uploadtemp`, `updater.NewTarget`, and `StreamTo`. Positive controls confirmed: `internal/plugins/update-cmd/cmd/serve.go` (`serveManifest`), `internal/component/config/system/selfupdate.go` (`extendedManifest`), `internal/component/config/system/update.go` (`versionManifest`), `internal/appliance/cmd_push.go` (`doPushUpdater`), and `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go`. Additional in-tree carriers discovered: `internal/test/fixture/storage_consumer_restart.go` (`storageHistoryRestart`, serves an unsigned `minimum-version` manifest), `test/ui/update-serve.ci`, `test/ui/update-serve-daemon.ci`, and `internal/component/gokrazy/gate.go` (`/gokrazy/` web proxy).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Self-update manifest (`version.json`) | writer | `internal/plugins/update-cmd/cmd/serve.go` (`updateServeContent.handler`, `serveManifest`) and `serve_daemon.go` (`updateServeOwner.start`) | Encodes JSON manifest on `GET /version.json` with `X-Ze-Arch` check | Change: read `.statement` and `.statement.sig` beside the binary and emit `{statement, signature, paused}`; refuse to start when absent (AC-14) |
| Self-update manifest (`version.json`) | writer | `internal/test/fixture/storage_consumer_restart.go` (`storageHistoryRestart`) | Inline HTTP handler writes unsigned `{"version":"9999.12.31","minimum-version":"9999.12.30",...}` | Change: emit a signed `minimum-version` statement and configure `trusted-key` in the fixture config so `test/plugin/storage-history-restart.ci` continues proving `blocked-minimum-version` persistence |
| Self-update manifest (`version.json`) | writer | `internal/component/config/system/selfupdate_test.go`, `update_test.go`, `internal/plugins/update-cmd/cmd/serve_test.go` | Unit test mock servers serving `/version.json` | Change: sign test manifests with a fixture key; keep explicit unsigned mocks for AC-3 and AC-13 refusal tests |
| Self-update manifest (`version.json`) | writer | Out-of-repo custom HTTPS update servers (`docs/guide/self-update.md`) and future `./le build release` (`spec-release-build.md` phase 6) | Documented custom server JSON format | Change: pre-release clean break (no unsigned fallback); refusal names `ze appliance sign binary` and `docs/guide/self-update.md` documents the signed format |
| Self-update manifest (`version.json`) | reader | `internal/component/config/system/selfupdate.go` (`selfUpdater.fetchManifest`, `check`, `manualDownload`, `manualApply`) | Decodes `extendedManifest` from `GET <url>` | Change: decode signed envelope and verify through `updatesig.VerifyBinary`; delete unsigned `manualDownload`/`manualApply` fallback |
| Self-update manifest (`version.json`) | reader | `internal/component/config/system/update.go` (`UpdateChecker.fetchVersion`, `check`) | Decodes `versionManifest` from `GET <url>` | Change: decode and verify through `updatesig.VerifyBinary`; suppress version warning and raise `doctor-update-signature-invalid` on invalid or missing signature (AC-13) |
| Self-update manifest (`version.json`) | reader | `test/ui/update-serve.ci`, `test/ui/update-serve-daemon.ci` | Functional tests fetching `/version.json` and asserting `contains=version` | Change: provide signed statement sidecars for the test binary and assert signed manifest fields |
| Appliance OTA push (`/update/*`, `/divert`, `/uploadtemp`) | writer | `internal/appliance/cmd_push.go` (`pushOne`, `doPushUpdater`, via vendored `github.com/gokrazy/updater`) | Calls `updater.NewTarget` (`GET /update/features`), `StreamTo` (`root`, `bootonly`, `mbr`), `Switch`/`Testboot`, `Reboot` | Change: verify `.statement`/`.statement.sig` locally before contact; send `PUT /update/ze-statement` first when `/update/features` advertises `ze-signed`; preserve legacy sequence with warning when `ze-signed` is absent (AC-21) |
| Appliance OTA push (`/update/*`, `/divert`, `/uploadtemp`) | writer | `internal/component/gokrazy/gokrazy.go`, `gate.go` (`handlerWithAuth`, `pageFor`) | Forwards `/gokrazy/update/*` with device token for `update system firmware apply`; already refuses `/divert` and `/uploadtemp` | Preserve: proxy stays unchanged; device-side verification enforces signatures on forwarded `/update/*` requests (AC-20, story 4) |
| Appliance OTA push (`/update/*`, `/divert`, `/uploadtemp`) | reader | `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go` via `gokrazy/patches/signed-update.patch` | Device HTTP handlers for `/update/*`, `/divert`, `/uploadtemp` | Change: advertise `ze-signed`, add `PUT /update/ze-statement`, verify root/boot/bootonly/mbr digests and `issued` floor before switch/testboot, refuse `/divert` and `/uploadtemp` (AC-17..AC-20) |
| Appliance OTA push (`/update/*`, `/divert`, `/uploadtemp`) | reader | `internal/component/config/system/backend_gokrazy.go` (`gokrazyBackend.probe`, `featureNames`) | Reads `GET /update/features` over management Unix socket | Extend: read `ze-signed` presence to drive `doctor-image-verification-inactive` (AC-23) |
| Appliance OTA push (`/update/*`, `/divert`, `/uploadtemp`) | reader | `internal/appliance/cmd_push_test.go`, `push_image_test.go`, `internal/component/config/system/backend_ze_distro_test.go`, `internal/le/test/qemu/install_ota.go` | Mock OTA servers and QEMU OTA proof | Change: update whole-protocol mocks to handle `ze-signed` and `PUT /update/ze-statement` (keeping an unverified-device mock for AC-21); extend `install_ota.go` with `ota-signed` steps |
| Appliance OTA push (`/update/*`, `/divert`, `/uploadtemp`) | reader | Out-of-repo field devices running pre-spec appliance images without `ze-signed` | Upstream gokrazy `update.go` without `signed-update.patch` | Preserve field contract: `ze appliance push` prints a warning and pushes using the existing sequence so devices upgrade to a verifying image (AC-21) |

### Integration Points
- `UpdateBackend` (`backend.go`): no new backend; `zeBackend` keeps its two parts
- `config.RegisterServiceBlock("system/update-check", "system update-check")` in `internal/component/config/system/register.go` so `LoadConfigForStart` isolates an invalid `system/update-check` block (such as a malformed `trusted-key`) under `doctor-service-not-started` instead of exiting `ze start`, while `ze config validate`, commit, and reload still refuse it
- doctor registries in `internal/component/config/system/doctor.go` and `internal/appliance/doctor_checks.go`
- owning-package doctor code registration in `internal/component/config/system/register_doctor_codes.go` (`doctor-update-no-trusted-key`, `doctor-update-signature-invalid` plus `diagnostic.RegisterEventCode`, `doctor-image-verification-inactive`) and `internal/appliance/register_doctor_codes.go` (`doctor-appliance-no-signing-key`)
- `internal/component/config/validators_register.go` (`update-trusted-key` registered under `system`, which is already in `validatedSections`)
- `internal/core/configvalue` (`LeafList` for `trusted-key` extraction in `ExtractUpdateCheckFromMap`)
- `internal/core/version/` (`Flavor()` build-tag accessor)
- `internal/appliance/cmd_rekey.go` (`rekeyKeyFiles` includes the per-instance image signing private key)
- `gokrazy/patches/` and the pin guard (`TestPinnedGokrazyRuntimeMatchesUpstream`), plus both `gokrazy/ze` and `gokrazy/docker-host` builddir `go.mod` files

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | One verify function per kind in `pkg/updatesig`; every daemon path and the gokrazy patch call it |
| No unintended coupling (components stay isolated) | Yes | `pkg/updatesig` imports the standard library and `golang.org/x/crypto/ssh` only; no `internal/` import |
| No duplicated functionality (extends existing, does not recreate) | Yes | `isNewer` and the optional checksum paths are deleted; `fetch.SameOrigin` (already exported in `5b800849ee`) and the `authorizedkeys` line rule are reused |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Not a wire hot path; one hash pass over each payload |
| Registration over hardcoding, outbound | Yes | Doctor checks and codes register in their owning packages (`system/register_doctor_codes.go`, `appliance/register_doctor_codes.go`); no switch gains a case |
| Registration over hardcoding, inbound | Yes | Searched `featureNames`, the `NewBackend` factories, the doctor registries, `validators_register.go` (`validatedSections` already includes `system`), and owning-package `register_doctor_codes.go`: each takes the new names by registration; the two statement kinds are two values of one verifier, not a list another file copies |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `x/crypto/ssh` v0.58.0 verifies an `ssh-ed25519` SSHSIG made by OpenSSH, and Ze's SSHSIG verifies with `ssh-keygen -Y verify` | `vendor/golang.org/x/crypto/ssh/keys.go` `ed25519PublicKey.Verify`; the SSHSIG envelope is a small fixed format (OpenSSH `PROTOCOL.sshsig`) | The envelope parse or the signed-data layout needs fixing in `pkg/updatesig` | Unit test over an `ssh-keygen -Y sign` fixture, and `ssh-keygen -Y verify` over a Ze signature (AC-25) | unvalidated |
| A-2 | The gokrazy init build can import `pkg/updatesig` through a `replace` in both `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod` and `gokrazy/docker-host/builddir/github.com/gokrazy/gokrazy/go.mod` (rewritten generically by `absolutizeReplaces`) | Both instance builddirs pin the same patched `github.com/gokrazy/gokrazy` module (`gokrazy/ze/builddir` and `gokrazy/docker-host/builddir`) | The patch instead execs the running root's `ze` for verification (the alternative set aside) | Build a patched image in the QEMU OTA proof, `./le build gosum`, and `TestPreparedModulesResolveIdenticallyToTracked` | unvalidated |
| A-3 | No Ze workflow uses gokrazy `/divert` or `/uploadtemp` | grep of `internal/appliance`, `internal/le`, `docs`, `gokrazy/ze` found none | A Ze workflow that runs `gok run` stops working on Ze images. D-4 accepts that loss: a developer tests a changed binary by building and pushing a signed image from their own instance | Grep at implementation | unvalidated |
| A-4 | The boot payload fits in device memory for buffering | gokrazy boot FAT is bounded by the partition; the statement bounds it again | Buffer to a tmpfs file bounded by the signed size | QEMU OTA on the smallest supported RAM | unvalidated |
| A-5 | The running binary can name its build flavor | `internal/core/version/version.go` exposes `Release()` and `BuildDate()`, with no flavor accessor today | Add build-tag-driven `Flavor()` (`flavor_distro.go`, `flavor_appliance.go`, `flavor_other.go`) in `internal/core/version/` | Unit test per build tag (`ze_distro`, `ze_appliance`, untagged) | confirmed (code) |
| A-6 | The patch may write `/perm/ze/image-statement.json`, outside the daemon's zefs store | `/perm/ze/build.json` is already a raw file under `/perm/ze`; the fs-persistence gate judges Ze daemon code | Store the floor through a zefs key the daemon owns | `./le arch fs-persistence check`; door review | unvalidated |
| A-7 | A Ze web account granted `update system firmware apply` reaches gokrazy's update handlers through `/gokrazy/` | `gate.go` `forwardedPages` maps `/update/` to that command, and `handlerWithAuth` adds the password to each forwarded request | none: the device-side check is designed for it | Code read | confirmed (code) |
| A-8 | There is no project-operated update feed | `spec-release-distribution.md` scope excludes one | The release key also needs a feed's own publication path | Spec read | confirmed (spec) |
| A-9 | `ze appliance sign` can sign through an SSH agent, whether `ssh-agent` holds a key file or `gpg-agent` serves an OpenPGP card's authentication subkey | `x/crypto/ssh/agent` is vendored; both agents speak the same agent protocol | Signing reads a key file only, and the release key cannot stay on the OpenPGP hardware key | Sign through `ssh-agent` in a unit test; sign once through `gpg-agent` on the maintainer's machine | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Every trusted image key is lost: no image can be pushed again | `ze appliance push` refuses with "no signing key" | `ze appliance export` carries the key; docs say to back it up; recovery is a reinstall from installer media |
| R-2 | The signer's clock steps back and a new statement is not newer than the floor | Device refuses with the floor and the statement times | The signer records the last `issued` per instance and refuses to sign below it, naming the clock |
| R-3 | A partial push: root accepted, boot refused | Push fails after root | Switch is refused, the active slot is untouched, the shared boot partition was not written |
| R-4 | A device in the field runs an image from before this spec | Probe lacks `ze-signed` | Host warns and pushes as today; doctor on the device reports verification inactive until the new image runs |
| R-5 | A custom update server serves the old manifest | Device reports the manifest as unsigned | The refusal names the missing field and the signing command; `docs/guide/self-update.md` documents the format |
| R-6 | A gokrazy bump changes `update.go` under the patch | Pin guard goes red | Existing re-review procedure (`gokrazy-build-pins.md`) |
| R-7 | A compromised release key | Out-of-band report | The next release drops it from `releasekeys/`; operators pin their own key, which replaces the release keys |
| R-8 | A held binary is replaced during the maintenance wait | Re-hash at stage differs | Refused at stage, recorded as `failed-checksum` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A verifier that refuses valid statements stops every update: self-update devices stay on their release, appliances cannot be pushed until fixed by an image a trusted key signed or by reinstall. A verifier that accepts invalid ones leaves today's hole |
| How is it reverted? | Daemon: a commit revert. Appliance: a device running the patched gokrazy accepts only signed images, so a revert must itself be pushed as a signed image |
| Who else touches this path? | `spec-release-build.md` (`./le build release`), `spec-release-distribution.md` (package-managed update guard in `backend.go`), `spec-kernel-lockdown-hardening.md` (kernel build), the fault-investigation recorder (`ze start` parent on gokrazy) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `update system firmware download` | → | `selfUpdater.manualDownload` → `updatesig.VerifyBinary` | `test/plugin/self-update-signed-download.ci` |
| `system update-check auto-apply` | → | `selfUpdater.check` → `download` | `TestCheckRefusesUnsignedManifest` |
| `system update-check` without auto-apply | → | `UpdateChecker.fetchVersion` → `updatesig.VerifyBinary` | `TestCheckerIgnoresUnsignedNewerVersion` |
| `ze update serve` | → | `updateServeContent.handler` serving the signed files | `test/ui/update-serve-requires-statement.ci` |
| `ze appliance build <name>` | → | `writeImageStatement`, `writeImageSBOM` | `test/appliance/build-writes-signed-statement.ci` |
| `ze appliance push <name>` | → | `pushOne` → `verifyImageStatement` → `PUT /update/ze-statement` | `TestPushSendsStatementFirst` (whole-protocol mock) |
| gokrazy `/update/*` on a Ze image | → | `signed-update.patch` handlers → `updatesig.VerifyImage` | QEMU appliance OTA proof, `ota-signed` steps |
| `ze doctor` | → | the four new checks | `test/ui/doctor-update-unverified.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Manifest signed by a configured `trusted-key`, newer version, matching arch and flavor | `update system firmware download` verifies the statement and stages the binary; `show system update` reports the staged version and names the signing key's fingerprint | Unsigned `paused` and `download-url` fields remain outside the signature because a server can always withhold and the signed digest gates acceptance |
| AC-2 | Same, with `auto-apply` | The background cycle downloads, verifies signature and digest, and stages the binary while honoring spread and maintenance window | Same as AC-1; existing spread, pause, and maintenance-window scheduling remain preserved |
| AC-3 | Manifest with no `statement` or `signature` | Download and apply return an operator error naming the server and the signing command before any binary request is sent, leave the running binary untouched, and record `failed-signature` in update history | None: no unverified download or manual bypass path survives |
| AC-4 | Signature by a key not in the trusted set | Download and apply return an operator error naming the untrusted key's fingerprint and the `trusted-key` fix before any binary download, and record `failed-signature` in update history | None: signatures from unlisted keys are never accepted |
| AC-5 | Valid signature, statement bytes changed by one byte | Verification returns an operator error naming signature verification failure before any binary download, and records `failed-signature` in update history | None: any single-byte mutation invalidates the exact-bytes SSHSIG |
| AC-6 | Valid signature made under the image namespace (`ze-image@ze-software.net`) | Binary verifier returns an operator error naming the namespace mismatch and records `failed-signature` | None: cross-domain signatures between binary and image namespaces are never accepted |
| AC-7 | Valid statement, binary changed after signing | Download returns an operator error reporting "checksum mismatch", removes the temp file, and records `failed-checksum` in update history | None: only bytes matching both signed `size` and signed `sha256` are retained |
| AC-8 | Valid statement for an older or equal version | Updater returns an operator error reporting "not newer" with the rollback fix, and leaves the running binary untouched | `dev`/`unknown` running builds still short-circuit to "not newer" via `version.IsValidRelease(running)` unless `ze.test.update.running.version` overrides it, preserving test-build safety (`docs/architecture/config/system-update.md`) |
| AC-9 | Valid statement for another arch or flavor | Updater returns an operator error naming both the statement's and running binary's OS/arch or flavor before downloading | None: cross-arch and cross-flavor binaries are never staged |
| AC-10 | Server answers a redirect to another host or from `https` to `http` | Updater and checker refuse before contacting the redirect target, returning an operator error naming the redacted redirect target and fix. Implemented by `5b800849ee`: update fetches use `fetch.SameOrigin`; `TestRedirectLeavingOriginRefused` and `TestUpdateCheckRedirectLeavingOriginRefused` were red before it and green after | Same-origin redirects (up to 10 hops) remain permitted by `fetch.SameOrigin` |
| AC-11 | No trusted key at all (empty compiled-in set and no `trusted-key`) | Every update command returns an operator error naming the `trusted-key` fix; `ze doctor` reports `doctor-update-no-trusted-key` with `set system update-check trusted-key` as its `Fix` | `ze start` still starts normally per "Ze always starts" (`docs/architecture/core-design.md`); only updates fail closed |
| AC-12 | Held binary changed before the maintenance window opens | `attemptStage` re-hashes the held file, refuses staging with "checksum mismatch", removes the tampered file, records `failed-checksum`, and leaves `.prev` and the running binary untouched | None: a modified held file can never be renamed onto the running binary |
| AC-13 | Unsigned newer manifest, check-only mode | `UpdateChecker` records the signature error in `UpdateStatus.LastError`, clears any active `update-available` warning, and raises `doctor-update-signature-invalid` on the report bus with its fix so `show doctor` reports it | Legitimate signed newer manifests still raise `update-available` in check-only mode |
| AC-14 | `ze update serve` or `update serve start` with no statement files beside its binary | Start exits with an operator error naming the missing `.statement`/`.statement.sig` files and the `ze appliance sign binary` command without opening the HTTP listener | None: `update serve` never synthesizes an unsigned manifest |
| AC-15 | `ze appliance build <name>` | Writes `<img>.statement`, `<img>.statement.sig`, and `<img>.spdx.json` beside the image; `ssh-keygen -Y verify` with the instance public key and namespace `ze-image@ze-software.net` exits 0 and accepts the signature | `<img>.sha256` sidecar remains written for the ISO/PXE install server owned by `spec-kernel-lockdown-hardening.md` |
| AC-16 | `ze appliance push` with a missing, invalid, or mismatching statement | Push exits with an operator error naming the missing/invalid statement and `ze appliance sign image` before opening any HTTP connection to the device (mock sees zero requests) | None: `ze appliance push` never falls back to an optional `.sha256` sidecar |
| AC-17 | Patched device receives a correctly signed push | Patched gokrazy accepts `PUT /update/ze-statement`, verifies root, bootonly, and MBR digests, allows switch/testboot, writes `/perm/ze/image-statement.json`, and boots the new image | Existing TLS pinning (`cert.pem`), basic auth token, redirect refusal, and A/B partition switch/testboot flow remain in place |
| AC-18 | Patched device receives root, boot, bootonly, or MBR with no accepted statement, or with a payload whose digest differs | The device handler responds with an HTTP refusal naming the mismatch or missing statement, writes no boot or MBR bytes, refuses `/update/switch` and `/update/testboot`, and continues booting the old slot | None: unsigned or mismatched payloads can never be committed or booted |
| AC-19 | Patched device receives a validly signed statement whose `issued` is not strictly newer than its persisted floor | The device refuses `PUT /update/ze-statement` with an error naming both timestamps; a freshly re-signed older image (strictly newer `issued`) is accepted | Freshly re-signed rollback images with a newer `issued` timestamp remain permitted so operators can intentionally roll back an appliance |
| AC-20 | `/divert` or `/uploadtemp` on a patched device, direct or through Ze's `/gokrazy/` proxy | The device refuses the request with an operator message stating unsigned code is disabled on Ze images and naming `ze appliance build` and `ze appliance push` | None: no unpatched development image or `gok run` bypass survives (D-4) |
| AC-21 | Device running an image from before this spec (`/update/features` lacks `ze-signed`) | `ze appliance push` prints a clear warning that the device does not verify updates and that this push installs a verifying image, then completes the push using the existing protocol order | Legacy unsigned-device push remains permitted for this transition so field devices can be upgraded to signed images (field-image contract in `ai/doors/appliance.md`) |
| AC-22 | SPDX file written by `ze appliance build` | Valid SPDX 2.3 JSON listing every Go module of every Go binary in the root with version and checksum, plus the runtime kernel version; its SHA-256 equals the signed statement's `sbom.sha256` | Non-Go non-kernel static assets without Go build info are outside the Go module inventory; kernel provenance covers the Linux kernel |
| AC-23 | `ze doctor` on a device whose gokrazy `/update/features` lacks `ze-signed` | Reports `doctor-image-verification-inactive` with a `Fix` naming `ze appliance build` and `ze appliance push` | Devices advertising `ze-signed` emit no finding for this check |
| AC-24 | `ze appliance` instance with no signing key | Host `ze doctor` reports `doctor-appliance-no-signing-key` with `Fix: ze appliance key generate <name>` | Instances that hold a signing key or configured agent key emit no finding |
| AC-25 | A signature made by OpenSSH `ssh-keygen -Y sign` with an `ssh-ed25519` key; a signature by any other key type (RSA, ECDSA, `sk-ssh-ed25519`) | Ze verifies the `ssh-ed25519` signature and refuses every other key type naming the rejected type; a signature produced by Ze verifies with `ssh-keygen -Y verify` | None: only `ssh-ed25519` is accepted (D-6) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator signs a binary and serves it to a fleet | `ze appliance sign binary` → `ze update serve` → device `update system firmware apply` | `test/plugin/self-update-signed-download.ci` |
| 2 | Attacker controlling the update server serves its own binary | manifest → verify refuses | `test/plugin/self-update-unsigned-refused.ci` |
| 3 | Operator builds and pushes an appliance image | `ze appliance build` → `push` → patched gokrazy → switch → boot | QEMU `ota-signed` |
| 4 | Web user with the proxied gokrazy UI tries to install an image | `/gokrazy/update/root` → patched handler refuses | QEMU `ota-signed`, proxy step |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyEd25519Fixture` | `pkg/updatesig/verify_test.go` | An `ssh-keygen` Ed25519 signature verifies | |
| `TestVerifyRefusesNonEd25519KeyType` | `pkg/updatesig/verify_test.go` | Any key type other than `ssh-ed25519` (RSA, ECDSA, `sk-ssh-ed25519`) is refused, naming its type (D-6, AC-25) | |
| `TestVerifyRefusesWrongNamespace` | `pkg/updatesig/verify_test.go` | Binary statement signed as image refused, and the reverse | |
| `TestVerifyRefusesUnknownKey` | `pkg/updatesig/verify_test.go` | Key outside the set refused, fingerprint named | |
| `TestVerifyRefusesTamperedStatement` | `pkg/updatesig/verify_test.go` | One changed byte refused | |
| `TestStatementRefusesUnknownField` | `pkg/updatesig/statement_test.go` | Strict parse | |
| `TestSignThenSSHKeygenFormat` | `pkg/updatesig/sign_test.go` | Ze's armor matches the committed `ssh-keygen` fixture layout | |
| `TestCheckRefusesUnsignedManifest` | `internal/component/config/system/selfupdate_test.go` | Auto path refuses, nothing downloaded | |
| `TestManualPathsShareVerification` | `internal/component/config/system/selfupdate_test.go` | Download and apply refuse an unsigned manifest | |
| `TestDownloadStopsAtSignedSize` | `internal/component/config/system/selfupdate_test.go` | Body longer than `size` refused | |
| `TestStageRehashesHeldFile` | `internal/component/config/system/selfupdate_test.go` | AC-12 | |
| `TestRedirectLeavingOriginRefused` | `internal/component/config/system/selfupdate_test.go` | AC-10, the self-updater | red before, green after 5b800849ee (`ze_distro` tag) |
| `TestUpdateCheckRedirectLeavingOriginRefused` | `internal/component/config/system/update_test.go` | AC-10, the update checker | red before, green after 5b800849ee |
| `TestDowngradeRefusedByCompareReleases` | `internal/component/config/system/selfupdate_test.go` | AC-8 | |
| `TestConfiguredKeyReplacesReleaseKeys` | `internal/component/config/system/selfupdate_test.go` | A release-key signature is refused once a `trusted-key` is configured | |
| `TestCheckerIgnoresUnsignedNewerVersion` | `internal/component/config/system/update_test.go` | AC-13 | |
| `TestTrustedKeyLeafValidated` | `internal/component/config/system/update_test.go` | A malformed or non-Ed25519 key line is refused at `ze config validate`, commit, and reload | |
| `TestExtractUpdateCheckTrustedKeySingleAndMultiple` | `internal/component/config/system/update_test.go` | `ExtractUpdateCheckFromMap` reads 0, 1 (bare `string`), and 2+ (`[]string`/`[]any`) `trusted-key` entries via `configvalue.LeafList` | |
| `TestInvalidTrustedKeyExcludedAtStartRefusedAtValidate` | `internal/component/config/system/update_test.go` | `config.RegisterServiceBlock("system/update-check", ...)` lets `LoadConfigForStart` exclude an invalid `trusted-key` block under `doctor-service-not-started` so `ze start` stays up while strict `LoadConfig`, `ze config validate`, commit, and reload refuse it | |
| `TestServeRefusesWithoutStatement` | `internal/plugins/update-cmd/cmd/serve_test.go` | AC-14 | |
| `TestImageStatementMatchesPushExtraction` | `internal/appliance/statement_test.go` | Part digests equal what push sends | |
| `TestSBOMListsBuildInfoModules` | `internal/appliance/sbom_test.go` | AC-22 | |
| `TestRekeyReencryptsImageSigningKey` | `internal/appliance/cmd_rekey_test.go` | `ze appliance rekey` re-encrypts the per-instance image signing private key alongside `update.token` and TLS keys (and skips cleanly on agent-key instances) | |
| `TestPushRefusesBadStatementBeforeContact` | `internal/appliance/cmd_push_test.go` | AC-16 | |
| `TestPushSendsStatementFirst` | `internal/appliance/cmd_push_test.go` | Mock speaking the whole protocol sees the statement before root | |
| `TestPushWarnsOnUnverifyingDevice` | `internal/appliance/cmd_push_test.go` | AC-21 | |
| `TestSignerRefusesIssuedBelowLast` | `internal/appliance/statement_test.go` | R-2 | |
| `TestSignedUpdatePatchApplies` | `internal/le/vendorpatch/gokrazy_runtime_patch_test.go` | The new patch is in the pinned guard | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| statement `size` (binary) | 1 to 500 MiB | 524288000 | 0 | 524288001 |
| download body | exactly `size` bytes | `size` | `size` minus 1 (digest fails) | `size` plus 1 |
| `trusted-key` entries | 0 to 16 | 16 | N/A | 17 |
| boot payload | 1 to the boot partition length | partition length | 0 | partition length plus 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `self-update-signed-download` | `test/plugin/self-update-signed-download.ci` | AC-1 through the daemon form of `update serve` | |
| `self-update-unsigned-refused` | `test/plugin/self-update-unsigned-refused.ci` | AC-3, AC-13 | |
| `self-update-wrong-key-refused` | `test/plugin/self-update-wrong-key-refused.ci` | AC-4 | |
| `self-update-downgrade-refused` | `test/plugin/self-update-downgrade-refused.ci` | AC-8 | |
| `update-serve-requires-statement` | `test/ui/update-serve-requires-statement.ci` | AC-14 | |
| `doctor-update-unverified` | `test/ui/doctor-update-unverified.ci` | AC-11, AC-13 codes | |
| `build-writes-signed-statement` | `test/appliance/build-writes-signed-statement.ci` | AC-15, AC-22 | |
| `ota-signed` | `internal/le/test/qemu/install_ota.go` steps | AC-17 to AC-21 on a real patched device | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `update-signature-openssh` | `pkg/updatesig/testdata/openssh/` plus the `build-writes-signed-statement` step that runs `ssh-keygen -Y verify` | OpenSSH `ssh-keygen` | AC-25 both directions; the fixture's generating command and OpenSSH version are recorded beside it | |

## Files to Modify
- `internal/component/config/system/selfupdate.go` - one verify step for every path, signed size and digest, re-hash at stage; delete the unverified branch (same-origin client already landed in `5b800849ee`)
- `internal/component/config/system/update.go` - checker verifies via `updatesig.VerifyBinary`; `isNewer` deleted for `version.IsValidRelease` + `version.CompareReleases`; `trusted-key` extraction via `configvalue.LeafList`
- `internal/component/config/system/register.go` - register `system/update-check` via `config.RegisterServiceBlock("system/update-check", "system update-check")` so an invalid `trusted-key` block is excluded at `ze start` under `doctor-service-not-started` instead of exiting the daemon
- `internal/component/config/system/yang/ze-system-conf.yang` - `trusted-key` leaf-list (`max-elements 16`, `length` bound, `ze:validate "update-trusted-key"`), `url` description of the signed manifest
- `internal/component/config/system/doctor.go` - `doctor-update-no-trusted-key`, `doctor-update-signature-invalid` (report bus), and `doctor-image-verification-inactive` (gokrazy feature probe) checks with `Fix` lines
- `internal/component/config/system/register_doctor_codes.go` - owning-package registration via `diagnostic.Register` (and `diagnostic.RegisterEventCode` for `doctor-update-signature-invalid`) for the three system update doctor codes
- `internal/component/config/system/backend_gokrazy.go` - expose `ze-signed` from `/update/features` for the device doctor check
- `internal/component/config/validators_register.go` - register `update-trusted-key` validator (`system` is already in `validatedSections`)
- `internal/core/version/version.go` - expose `Flavor()` accessor for binary statement flavor matching
- `internal/plugins/update-cmd/cmd/serve.go`, `serve_daemon.go` - serve the signed statement files, refuse to start without them
- `internal/appliance/cmd_build.go`, `manifest.go` - statement and SBOM after the image
- `internal/appliance/cmd_push.go` - mandatory statement verification, statement-first push (`PUT /update/ze-statement`), unverifying-device warning
- `internal/appliance/cmd_init.go`, `cmd_rekey.go`, `main.go` - generate the image key; re-encrypt the image signing private key in `rekeyKeyFiles`; `key` and `sign` subcommands
- `internal/appliance/doctor_checks.go`, `internal/appliance/register.go` - `doctor-appliance-no-signing-key` check with `Fix`
- `internal/le/vendorpatch/gokrazy_runtime_patch_test.go` - guard the new `signed-update.patch`
- `internal/le/test/qemu/install_ota.go` - the `ota-signed` steps
- `internal/test/fixture/storage_consumer_restart.go` - emit a signed `minimum-version` statement and configure `trusted-key`
- `gokrazy/ze/config.json`, `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod`, `gokrazy/docker-host/builddir/github.com/gokrazy/gokrazy/go.mod` - key file in root, `pkg/updatesig` replace in both gokrazy instance builddirs (both pin the shared patched `github.com/gokrazy/gokrazy` runtime)
- `docs/architecture/appliance/self-update.md`, `docs/architecture/config/system-update.md`, `docs/architecture/appliance/ota-push.md`, `docs/architecture/appliance/gokrazy-build-pins.md`, `docs/architecture/appliance/builder.md`, `docs/architecture/appliance/build-artifacts.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/testing/qemu-integration.md`, `docs/features/ai-first.md`, `docs/guide/self-update.md`, `docs/guide/appliance.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `features/self-update.md`

## Files to Create
- `pkg/updatesig/verify.go`, `sign.go`, `statement.go` - SSHSIG verify and sign (`ssh-ed25519` only), the two statement kinds
- `pkg/updatesig/releasekeys/` - the release public keys, current and next
- `pkg/updatesig/testdata/openssh/` - `ssh-keygen` fixtures
- `internal/core/version/flavor_distro.go`, `flavor_appliance.go`, `flavor_other.go` - build-tag flavor strings (`distro`, `appliance`, `dev`)
- `internal/component/config/validators_update.go` - `UpdateTrustedKeyValidator` applying the `authorizedkeys` one-line rule and refusing non-Ed25519 keys
- `internal/appliance/statement.go`, `sbom.go`, `cmd_sign.go`, `cmd_key.go`, `register_doctor_codes.go`
- `gokrazy/patches/signed-update.patch`
- the `.ci` files in the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-system-conf.yang` `update-check trusted-key` |
| YANG validation constraints | Yes | `max-elements 16`, `length` bound; key syntax by `ze:validate "update-trusted-key"` |
| YANG custom validators | Yes | `internal/component/config/validators_update.go` registered in `validators_register.go` (`system` is already in `validatedSections`), applying the `authorizedkeys` one-line rule and refusing non-Ed25519 types |
| CLI commands/flags | Yes | `ze appliance key generate`, `key show`, `sign binary`, `sign image` (build-host tooling, `internal/appliance/main.go`) |
| CLI grammar (keyword before value) | Yes | `ze appliance sign binary <path> version <v>`; `ai/rules/cli.md` |
| Editor autocomplete | No | `trusted-key` is free text |
| Functional test for new RPC/API | Yes | Functional Tests table |
| Pipe completeness | N-A | No new daemon command answer; `show system update` gains a field through its existing renderer |
| Env var registration | No | No new env var; the test override `ze.test.update.running.version` already exists |
| Doctor check for runtime dependencies | Yes | `doctor-update-no-trusted-key` (post-config check in `system/doctor.go`), `doctor-image-verification-inactive` (gokrazy probe in `system/doctor.go`), `doctor-appliance-no-signing-key` (pre-config host check in `appliance/doctor_checks.go`); each registers metadata in its owning package (`system/register_doctor_codes.go`, `appliance/register_doctor_codes.go`) with `Fix` and a `.ci` |
| Doctor code per problem class | Yes | Refusals and `Warn` sites carry owning-package registered codes: `doctor-update-no-trusted-key` (post-config), `doctor-update-signature-invalid` (transient report-bus event code registered via `diagnostic.RegisterEventCode` and `diagnostic.Register` in `system/register_doctor_codes.go`, raised with `Fix` via `report.RaiseWarningFix`), `doctor-image-verification-inactive` (post-config/device probe), and `doctor-appliance-no-signing-key` (pre-config host check) |
| Prometheus counters/metrics | No | History and report bus carry the outcome |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/self-update.md` (renders `docs/features.md`): signed updates, image SBOMs, clears defect review row |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (`trusted-key`) |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md`, `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | No daemon RPC added |
| 5 | Plugin added/changed? | Yes | `update-cmd` serve behaviour in `docs/guide/self-update.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/self-update.md` (manifest format, Custom Server, signing), `docs/guide/appliance.md` |
| 7 | Wire format changed? | Yes | The manifest and statement, in `docs/architecture/appliance/self-update.md` |
| 8 | Plugin SDK/protocol changed? | No | `pkg/updatesig` is a new public package, documented in its package doc |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | Yes | QEMU OTA steps in `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares BGP/OSPF routing daemons and has no OS or appliance image-signing table |
| 12 | Internal architecture changed? | Yes | `system-update.md`, `ota-push.md`, `gokrazy-build-pins.md`, `builder.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes beyond doctor codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-selfupdate-manifest-authenticity.md` at implementation; the pages above are the `// Design:` targets |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/self-update.md` Custom Server JSON is rewritten to the signed form |

## Design presentation

### What changes

Every path that brings new code onto a running Ze system — daemon self-update (automatic `auto-apply`, manual `update system firmware download` and `apply`, and passive `system update-check`), `ze update serve`, and appliance OTA image pushes (`ze appliance push` into gokrazy's `/update/*` handlers) — now requires a valid Ed25519 OpenSSH SSHSIG signature over an exact-bytes JSON statement from a key the device trusts. Binary statements (`ze-update-statement-1`, namespace `ze-update@ze-software.net`) bind version, OS, arch, build flavor (`version.Flavor()`), SHA-256, exact size, and `minimum-version`; image statements (`ze-image-statement-1`, namespace `ze-image@ze-software.net`) bind appliance name, arch, monotonic `issued` timestamp floor, whole-image and per-partition (`root`, `boot`, `mbr`) digests/sizes, and the image's SPDX 2.3 SBOM digest. The unverified manual self-update fallback is deleted, `ze update serve` refuses to start without signed statement sidecars, `ze appliance build` signs every image with the instance's key and emits its SPDX 2.3 SBOM, and a second reviewed gokrazy patch (`signed-update.patch`) verifies every pushed payload on the device before switching slots and disables `/divert` and `/uploadtemp` (while `ze appliance push` still upgrades pre-spec field devices with a warning).

### Before and after

| # | Surface | Before | After |
|---|---------|--------|-------|
| 1 | `update system firmware download` / `apply` against an unsigned manifest | Logs "no checksum available, binary will not be verified" and stages or installs the unverified binary | Refuses before downloading any binary with an operator error naming the server and `ze appliance sign binary <path> version <v>`, records `failed-signature` in update history, and leaves the running binary untouched |
| 2 | `system update-check` (check-only) against an unsigned newer manifest | Raises an `update-available` warning claiming newer firmware is available | Suppresses `update-available`, records the signature failure in `UpdateStatus.LastError`, and raises `doctor-update-signature-invalid` on the report bus with its fix so `show doctor` reports it |
| 3 | `system update-check` configuration | Accepts only `url`, `interval`, `auto-apply`, `spread`, `maintenance-window`, `restart`; trusts the manifest server | Adds `trusted-key` leaf-list (up to 16 `ssh-ed25519` public key lines validated by `update-trusted-key`); when configured it replaces compiled-in release keys, and `show system update` displays the signing key's fingerprint |
| 4 | `ze update serve` / `update serve start` | Computes an unsigned `{version, sha256, size, paused}` manifest from its running binary and opens the listener | Reads `.statement` and `.statement.sig` beside the binary and serves `{statement, signature, paused}`; refuses to start without them, naming `ze appliance sign binary` |
| 5 | `ze appliance build <name>` and `ze appliance push <name>` | Writes `<img>` and optional `<img>.sha256`; push skips checksum when sidecar is absent and streams directly to gokrazy | `build` writes `<img>.statement`, `<img>.statement.sig`, and `<img>.spdx.json`; `push` verifies the signature and digests locally before contacting the device, sends `PUT /update/ze-statement` first to devices advertising `ze-signed`, and prints a warning when upgrading an older device lacking `ze-signed` |
| 6 | gokrazy `/update/*`, `/divert`, `/uploadtemp` on a Ze appliance | Installs any bytes sent by a token holder (including through `/gokrazy/update/*`) and executes arbitrary binaries via `/divert` and `/uploadtemp` | Verifies every root, boot, bootonly, and MBR payload against a signed statement trusted by the running root's key file and strictly newer than `/perm/ze/image-statement.json` before allowing switch/testboot, and refuses `/divert` and `/uploadtemp` |

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| minisign (as VyOS uses) | Keys are passphrase-protected files that no SSH agent can hold, so the release update key could not stay on the maintainer's hardware key via `gpg-agent` |
| OpenPGP on the device | Requires adding a third-party OpenPGP module (bringing `circl` and a large parser onto every device); GPG remains for release `checksums.txt` and RPMs (D-1) |
| Sign canonical JSON or sign the binary alone | Signing exact statement bytes needs no JSON canonicalization; signing the binary alone leaves `version`, `flavor`, `os`, `arch`, and `minimum-version` unsigned |
| Sign `paused` and `download-url` | A server can always withhold updates regardless of `paused`, and the signed digest + `ValidateFetchURL` + `fetch.SameOrigin` gate acceptance regardless of `download-url` |
| Additive trust when `trusted-key` is configured | An operator running an internal update feed should not also trust the project release key for that feed; replacement also provides immediate emergency key pinning (D-2) |
| Ze daemon fronting gokrazy `/update/*` or patch execing `ze` | Verification must keep working when the Ze daemon is down for recovery; importing `pkg/updatesig` directly in `signed-update.patch` avoids making a CLI exit code an internal API |
| Keep `/divert` and `/uploadtemp` signed or ship an unpatched dev image | No Ze workflow uses `/divert` (A-3), signing ad-hoc binaries turns every developer into a release signer, and a second unpatched gokrazy runtime tree violates `gokrazy-build-pins.md` (D-4) |
| Support FIDO2 `sk-ssh-ed25519` keys | Thomas confirmed on 2026-11-16 that no FIDO2 hardware key exists; keeping unused sk verification code adds surface with no user (D-6) |
| CycloneDX or Syft for image SBOMs | `spec-release-distribution.md` standardizes on SPDX 2.3 JSON, and operator build hosts do not ship Syft; `debug/buildinfo` plus kernel provenance is self-contained |

### Documents departed from

- `docs/architecture/appliance/self-update.md`, section `Decisions`: currently states `"Manual apply and manual download bypass server-side pause, spread, and the maintenance window. Auto-apply requires a SHA-256 in the manifest; a manual command warns and proceeds without one."` After this work, that page states that manual apply and manual download still bypass pause, spread, and the maintenance window, and that every path (automatic, manual download, manual apply, and version check) requires a valid Ed25519 SSHSIG over a signed statement from a trusted key, with no unverified fallback. Thomas approved this design on 2026-11-16 (D-1..D-6).

### Choices for Thomas

none (Thomas resolved D-1 through D-6 and approved the whole design on 2026-11-16; see `## Owner Decisions` above).

## Implementation Steps

1. **Phase: Wiring** -- `pkg/updatesig` with verify and sign returning "not implemented", called from every entry point in the Wiring Test table; the wiring tests fail for that reason.
2. **Phase: Verifier** -- SSHSIG verify and sign, statements, OpenSSH fixtures (unit tests, A-1).
3. **Phase: Daemon** -- `version.Flavor()`, self-updater and checker verification, `isNewer` deletion, re-hash at stage, `trusted-key` leaf (read via `configvalue.LeafList`), `update-trusted-key` validator, `config.RegisterServiceBlock("system/update-check", ...)` in `system/register.go`, owning-package doctor codes (`system/register_doctor_codes.go`), `update serve`, `storage_consumer_restart.go` fixture update, docs; daemon `.ci` tests. (Same-origin `fetch.SameOrigin` on `newUpdateClient` already landed in `5b800849ee`.)
4. **Phase: Build host** -- image key in `init` and re-encryption in `cmd_rekey.go`, `key` and `sign` subcommands, statement and SBOM in `build`, mandatory statement in `push`, host doctor check and owning-package doctor code registration, docs.
5. **Phase: Device** -- `signed-update.patch`, `pkg/updatesig` replaces in both `gokrazy/ze` and `gokrazy/docker-host` builddir `go.mod` files and key file in root (A-2), pin guard, QEMU `ota-signed`, device doctor check, docs.
6. **Phase: Official releases** -- after `spec-release-build.md` lands `./le build release`: it emits `ze_<v>_<os>_<arch>.ze-update.json` and `.sig` per target, signed with the release update key through `ssh-agent`, and `releasekeys/` holds the published keys.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Fail closed | No path installs, stages or writes a payload before its signature and digest are verified; grep finds no "will not be verified" branch |
| One verifier | `check`, `manualDownload`, `manualApply`, `UpdateChecker`, `pushOne` and the gokrazy patch call `pkg/updatesig`, never a copy |
| Domain separation | The two namespaces and `format` values are checked independently |
| Errors | Each refusal is an `opererr` with the fix alone on the last line |
| Field contract | A push to a device without `ze-signed` still works (AC-21) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Signed statements verify with OpenSSH | `ssh-keygen -Y verify` step in `build-writes-signed-statement.ci` |
| No unverified install path | grep `selfupdate.go` for the removed warning; AC-3 tests |
| Patched gokrazy | `TestPinnedGokrazyRuntimeMatchesUpstream` and QEMU `ota-signed` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Statement parse strict and bounded; armor size bounded; base64 decoded once |
| Resource exhaustion | Download cut at signed size; boot buffer bounded by signed size and partition |
| Replay and downgrade | Version strictly newer (daemon); `issued` strictly above floor (image) |
| Fail-open | A missing key set, a parse error or an unknown key never falls through to "accept" |
| Error leakage | URLs through `redact`; no private key material in logs |
| Key handling | Image key encrypted at rest like `update.token`; release key never in CI |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Operator-facing refusals

Each row gives the first line, the why, and the fix line, per `ai/rules/user-facing-errors.md`.

| Condition | Problem | Why | Fix |
|-----------|---------|-----|-----|
| Unsigned manifest | The update server `<url>` sent no signature, so Ze did not download version `<v>` | Ze installs only binaries signed by a key it trusts | On the update server: `ze appliance sign binary <path> version <v>` |
| Unknown key | The update for `<v>` is signed by key `<fingerprint>`, which this router does not trust | Anyone can sign; only listed keys count | `set system update-check trusted-key "<key line>"` |
| No key | No update key is trusted, so Ze cannot check any update | Without a key every update is refused | `set system update-check trusted-key "<key line>"` |
| Not newer | The signed update offers `<v>`, which is not newer than the running `<r>` | A signed old release could otherwise downgrade the router | `update system firmware rollback` restores the previous binary |
| Push, no statement | Image `<img>` has no signature | The device installs only signed images | `ze appliance sign image <name> <img>` |
| Device refusal | The device refused `<part>`: its digest differs from the signed statement | The bytes changed after signing | `ze appliance build <name>` then `ze appliance push <name>` |
| Divert | gokrazy run and divert are disabled on Ze images | They run code no signature covers | `ze appliance build <name>` and `ze appliance push <name>` |

## Design Insights

- The trust anchor for a pushed image is the running image, so no boot-chain mechanism is needed for the update channel. Secure Boot answers a different attacker, one who writes the disk outside the channel, and stays in the lockdown spec. Installer media have no running image to anchor them, which is why their authenticity goes there too.
- The update token cannot be the content authority: it sits on the device and behind Ze's web proxy. The signing key never leaves the build host.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| SSHSIG (`ssh-keygen -Y`) with Ed25519 keys | minisign, as VyOS signs its nightlies; OpenPGP, as `spec-release-build.md` signs checksums | All three verify offline. SSHSIG needs only the vendored `x/crypto/ssh`, has namespaces for domain separation, its keys fit a config leaf in the form operators already use, OpenSSH is on every host for independent checks, and signing through an SSH agent lets the release key stay on the OpenPGP hardware key the 2026-09-28 ruling uses, through `gpg-agent`'s SSH support (D-6). minisign needs only stdlib plus the vendored `blake2b`, but its keys are password files that no agent can hold. OpenPGP needs a new module (ProtonMail `go-crypto`, which imports `circl` and so needs the owner's approval for that use) and puts a large parser on every device |
| Detached signature over exact statement bytes inside the manifest | Sign the JSON object in canonical form; sign the binary itself | Exact bytes need no canonicalisation; one fetch keeps the existing flow; a signature over the binary alone would leave version and minimum-version unsigned |
| `paused` and `download-url` stay unsigned | Sign the whole manifest | A server can always withhold; the digest decides acceptance whatever the URL |
| Downgrade: version strictly newer for binaries; signing time above a persisted floor for images | A signed expiry or TUF timestamp role | Version order already exists for releases; images are stamped `dev` and built many times per release, so signing time is their order. Freeze protection is not added: the server can withhold either way, and doctor already reports an unreachable server |
| Device-side check in a gokrazy patch importing `pkg/updatesig` | Ze daemon fronts the update endpoints; the patch execs the running `ze` | Verification keeps working when the Ze daemon is down, which today's recovery relies on. An exec makes a CLI exit code an internal API and needs a command no operator should type |
| Refuse `/divert` and `/uploadtemp` on Ze images (D-4) | Require a signature on them too | They exist for `gok run` development; no Ze workflow uses them (A-3), and signing ad-hoc binaries would make every developer a release signer. A developer builds and pushes a signed image from their own instance instead, whose key they hold (D-3). An unpatched development image is set aside: every Ze image builds from the one patched runtime tree, and a second, unpatched copy is what `gokrazy-build-pins.md` forbids |
| Configured `trusted-key` replaces the compiled-in release keys when present (D-2) | Additive trust | An operator who runs their own feed should not also trust the project key for it, and a compromised release key is closed at once by configuring one |
| SPDX 2.3 JSON only, generated from Go build info and the kernel pin | CycloneDX as well, as VyOS ships; Syft | One format, the one `spec-release-distribution.md` chose; an operator's build host has no Syft, and `debug/buildinfo` lists exact module versions and sums |
| The signer lives in `ze appliance` (build-host tooling) | A daemon command | A private key never belongs on the router; `internal/appliance` is already the host-only surface |
| Official release statements are emitted by the same signer from `./le build release` | A second signer inside `le` | One producer for the format |

## Owner Decisions

Thomas handed the five choices only he could make to the main session on
2026-11-15 ("Can you please resolve them?"). Each one is decided, and the
design above follows it.

| # | Decision | Reason |
|---|----------|--------|
| D-1 | Two signing mechanisms: GPG stays for release checksums and RPMs (`spec-release-build.md`), and SSHSIG signs device update statements. `spec-release-build.md` does not change | People and package managers check GPG. A device checks SSHSIG with the vendored `x/crypto/ssh` and carries no OpenPGP stack (Key Design Decisions, first row) |
| D-2 | A configured `trusted-key` replaces the compiled-in release keys | An operator's own feed must not also accept payloads the project signed |
| D-3 | One image signing key per appliance instance | A leaked key covers one device. The cost is one more key to back up per instance (R-1), and it is accepted |
| D-4 | `gok run` and `/divert` (with `/uploadtemp`) are lost on Ze images: the patched device refuses them, as Ze's web proxy already does | They run code that no signature covers. No unpatched development image exists: every Ze image builds from the one patched module cache, and a second unpatched runtime tree is what `docs/architecture/appliance/gokrazy-build-pins.md` forbids. A developer who would have used `gok run` builds and pushes a signed image from their own instance, whose key they already hold (D-3). AC-23 still reports a device running an image from before this spec |
| D-6 | No FIDO2 hardware key exists (Thomas, 2026-11-16). The release update key is a plain `ssh-ed25519` key, reached only through an SSH agent on the maintainer's machine and never in CI: a passphrase-protected key file in `ssh-agent`, or the OpenPGP hardware key's authentication subkey through `gpg-agent`. The verifier accepts `ssh-ed25519` only | sk key support had no key to serve, so it is removed rather than kept unused; the agent-only signer keeps the choice of custody open without a second code path |
| D-5 | The spec stays P0 in `plan/release/fixes/` | Unauthenticated code replacement is a security defect in shipped behavior (Release reason) |

Thomas approved the whole design on 2026-11-16 ("otherwise yes it is
correct"), with one correction: there is no FIDO2 hardware key (D-6). D-4's
open route is closed by removing the unpatched development image from the
design. The spec is `ready`.

## Known Limitations

- Revocation without a release: a compromised release key stays trusted by deployed binaries that have no configured key until the next release drops it. Configuring an operator key closes it at once, because it replaces the release keys.
## Work Not Done

| Item | Destination | State of the destination |
|------|-------------|--------------------------|
| Authenticity of installer media: ISO, installer kernel and initrd, and the image `ze install disk` writes over PXE or HTTP (today an optional SHA-256 from the same server, skipped with a warning when absent) | `plan/post-release/features/spec-kernel-lockdown-hardening.md` | Named in its Task since 2026-11-15 ("Installer media authenticity"), with what checks those bytes today; its verification design and acceptance criteria are owed there before it moves to `ready` |

## RFC Documentation (Scope: protocol)

No RFC. Verification code quotes `PROTOCOL.sshsig` from OpenSSH above the blob layout it checks, with an ASCII diagram of the signed blob.

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
- [ ] Scoped tests of every package and suite the change reaches pass; the whole-tree check is CI's (`ai/META-DECISIONS.md`): `./le test unit pkg path ./pkg/updatesig/`, `./le test unit pkg path ./internal/component/config/system/`, `./le test unit pkg path ./internal/plugins/update-cmd/cmd/`, `./le test unit pkg path ./internal/appliance/`, `./le test unit pkg path ./internal/le/vendorpatch/`, `./le test functional plugin`, `./le test functional ui`, `./le test functional appliance`, `./le test qemu appliance-ota` (the OTA proof), `./le build gosum`
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
- [ ] Interop tests (OpenSSH `ssh-keygen`)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
