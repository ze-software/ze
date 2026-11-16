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
| Updated | 2026-11-16 |

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
| Same-origin redirects | `internal/core/fetch` `sameOrigin` | Refuses a redirect leaving scheme or host, for the installer and `ze init --from` | Extends: exported and set as the self-update and checker clients' `CheckRedirect` |
| Release comparator | `version.CompareReleases`; `update.go` `isNewer` | Two comparators for one fact | Replaces `isNewer` with `CompareReleases`, which then carries the downgrade refusal |
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
| Update doctor checks | `system/doctor.go` `checkUpdateCheckURL` | Probes the URL | Extends: two checks beside it, and a device check reading the feature probe |
| Self-update report raises | `selfupdate.go` `selfUpdateCode` | Raises and clears `self-update` warnings | Extends: a signature failure is raised under its own code |
| Appliance secrets | `internal/appliance` `secretFilePath`, `WriteSecret`, `readSecret` | Stores the token, TLS key and passwords, encrypted under the passphrase | Extends: stores the image signing key |
| Appliance host doctor | `internal/appliance/doctor_checks.go` `applianceDoctorChecks` | Host checks for `ze appliance` | Extends: a check for an instance with no signing key |
| QEMU OTA proof | `internal/le/test/qemu/install_ota.go` `executeApplianceOTA` | Builds two images and pushes the second to a real guest | Extends: the `ota-signed` steps |
| Installer fetch digest | `internal/core/fetch`, `internal/install/disk` `run` | Optional SHA-256 from the same server | Does not apply: homed in the lockdown spec |

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
1. `fetchManifest` reads the body (existing bound), with the same-origin redirect policy.
2. `updatesig.VerifyBinary` decodes `statement` and `signature`, picks the trusted set, verifies the SSHSIG (namespace, key membership, user presence), then parses the statement strictly.
3. The updater compares `os`, `arch`, `flavor`, then `version` against the running release with `version.CompareReleases` (strictly newer), then `minimum-version`.
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

### Integration Points
- `UpdateBackend` (`backend.go`): no new backend; `zeBackend` keeps its two parts
- doctor registries in `internal/component/config/system/doctor.go` and `internal/appliance/doctor_checks.go`
- `internal/core/diagnostic/codes.go` for the four new codes
- `gokrazy/patches/` and the pin guard

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | One verify function per kind in `pkg/updatesig`; every daemon path and the gokrazy patch call it |
| No unintended coupling (components stay isolated) | Yes | `pkg/updatesig` imports the standard library and `golang.org/x/crypto/ssh` only; no `internal/` import |
| No duplicated functionality (extends existing, does not recreate) | Yes | `isNewer` and the optional checksum paths are deleted; `sameOrigin` and the `authorizedkeys` line rule are reused |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Not a wire hot path; one hash pass over each payload |
| Registration over hardcoding, outbound | Yes | Doctor checks register in their owning packages; no switch gains a case |
| Registration over hardcoding, inbound | Yes | Searched `featureNames`, the `NewBackend` factories, the doctor registries and `codes.go`: each takes the new names by registration; the two statement kinds are two values of one verifier, not a list another file copies |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `x/crypto/ssh` v0.58.0 verifies an `ssh-ed25519` SSHSIG made by OpenSSH, and Ze's SSHSIG verifies with `ssh-keygen -Y verify` | `vendor/golang.org/x/crypto/ssh/keys.go` `ed25519PublicKey.Verify`; the SSHSIG envelope is a small fixed format (OpenSSH `PROTOCOL.sshsig`) | The envelope parse or the signed-data layout needs fixing in `pkg/updatesig` | Unit test over an `ssh-keygen -Y sign` fixture, and `ssh-keygen -Y verify` over a Ze signature (AC-25) | unvalidated |
| A-2 | The gokrazy init build can import `pkg/updatesig` through a `replace` in `gokrazy/ze/builddir` | `builddir/github.com/gokrazy/gokrazy/go.mod` requires only gokrazy today | The patch instead execs the running root's `ze` for verification (the alternative set aside) | Build a patched image in the QEMU OTA proof | unvalidated |
| A-3 | No Ze workflow uses gokrazy `/divert` or `/uploadtemp` | grep of `internal/appliance`, `internal/le`, `docs`, `gokrazy/ze` found none | A Ze workflow that runs `gok run` stops working on Ze images. D-4 accepts that loss: a developer tests a changed binary by building and pushing a signed image from their own instance | Grep at implementation | unvalidated |
| A-4 | The boot payload fits in device memory for buffering | gokrazy boot FAT is bounded by the partition; the statement bounds it again | Buffer to a tmpfs file bounded by the signed size | QEMU OTA on the smallest supported RAM | unvalidated |
| A-5 | The running binary can name its build flavor | `internal/core/version` exposes release and build date; no flavor accessor found | Add the stamp at build, beside the release stamp | Read `version` and the flavor build files | unvalidated |
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

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Manifest signed by a configured `trusted-key`, newer version, matching arch and flavor | `update system firmware download` stages the binary; `show system update` names the signing key's fingerprint |
| AC-2 | Same, with `auto-apply` | The background cycle downloads, verifies and stages, as today |
| AC-3 | Manifest with no `statement` or `signature` | Download and apply refuse before any binary download; the error names the server and the fix; nothing is staged; history records `failed-signature` |
| AC-4 | Signature by a key not in the trusted set | Refused as AC-3, naming the key fingerprint |
| AC-5 | Valid signature, statement bytes changed by one byte | Refused as AC-3 |
| AC-6 | Valid signature made under the image namespace | Refused: a statement of the wrong kind |
| AC-7 | Valid statement, binary changed after signing | Refused with "checksum mismatch", temp file removed |
| AC-8 | Valid statement for an older or equal version | Refused as "not newer"; a signed old statement cannot downgrade |
| AC-9 | Valid statement for another arch or flavor | Refused, naming both |
| AC-10 | Server answers a redirect to another host | Refused, naming the redirect target (redacted). Implemented by 5b800849ee: update fetches use `fetch.SameOrigin`; `TestRedirectLeavingOriginRefused` and `TestUpdateCheckRedirectLeavingOriginRefused` were red before it and green after |
| AC-11 | No trusted key at all | Every update command refuses with the `trusted-key` fix; doctor reports `doctor-update-no-trusted-key` |
| AC-12 | Held binary changed before the maintenance window opens | Stage refuses; `.prev` and the running binary are untouched |
| AC-13 | Unsigned newer manifest, check-only mode | No "newer firmware available" warning; doctor reports `doctor-update-signature-invalid` |
| AC-14 | `ze update serve` with no statement files beside its binary | Start refuses, naming the command that signs |
| AC-15 | `ze appliance build <name>` | Writes `<img>.statement`, `<img>.statement.sig`, `<img>.spdx.json`; `ssh-keygen -Y verify` with the instance key and namespace `ze-image@ze-software.net` accepts the signature |
| AC-16 | `ze appliance push` with a missing, invalid or mismatching statement | Refused before contacting the device (the mock sees no request) |
| AC-17 | Patched device receives a correctly signed push | Switches and boots the new image; `/perm/ze/image-statement.json` holds its statement |
| AC-18 | Patched device receives root, boot or MBR with no accepted statement, or with a payload whose digest differs | The handler refuses; boot and MBR bytes are not written; switch and testboot refuse; the device keeps booting the old slot |
| AC-19 | Patched device receives a validly signed statement whose `issued` is not newer than its floor | Refused, naming both times; a re-signed old image (new `issued`) is accepted |
| AC-20 | `/divert` or `/uploadtemp` on a patched device, direct or through Ze's `/gokrazy/` proxy | Refused with a message saying unsigned code is disabled on Ze images |
| AC-21 | Device running an image from before this spec | Push succeeds with a printed warning that the device does not verify updates |
| AC-22 | SPDX file | Valid SPDX 2.3 JSON listing every Go module of every Go binary in the root with version and checksum, plus the kernel version; its digest equals the statement's `sbom.sha256` |
| AC-23 | `ze doctor` on a device whose gokrazy lacks `ze-signed` | Reports `doctor-image-verification-inactive` with the push command |
| AC-24 | `ze appliance` instance with no signing key | Host doctor reports `doctor-appliance-no-signing-key` with `ze appliance key generate <name>` |
| AC-25 | A signature made by OpenSSH `ssh-keygen -Y sign` with an Ed25519 key; a signature by any other key type | Ze verifies the Ed25519 one and refuses the other, naming its type; a Ze signature verifies with `ssh-keygen -Y verify` |

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
| `TestVerifySKRequiresUserPresence` | `pkg/updatesig/verify_test.go` | An sk signature without the UP flag is refused | |
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
| `TestTrustedKeyLeafValidated` | `internal/component/config/system/update_test.go` | A malformed or non-Ed25519 key line refused at commit | |
| `TestServeRefusesWithoutStatement` | `internal/plugins/update-cmd/cmd/serve_test.go` | AC-14 | |
| `TestImageStatementMatchesPushExtraction` | `internal/appliance/statement_test.go` | Part digests equal what push sends | |
| `TestSBOMListsBuildInfoModules` | `internal/appliance/sbom_test.go` | AC-22 | |
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
- `internal/component/config/system/selfupdate.go` - one verify step for every path, signed size and digest, re-hash at stage, same-origin client; delete the unverified branch
- `internal/component/config/system/update.go` - checker verifies; `isNewer` deleted for `version.CompareReleases`; `trusted-key` extraction
- `internal/component/config/system/yang/ze-system-conf.yang` - `trusted-key` leaf-list, `url` description of the new manifest
- `internal/component/config/system/doctor.go` - `doctor-update-no-trusted-key`, `doctor-update-signature-invalid`
- `internal/component/config/system/backend_gokrazy.go` - doctor reads `ze-signed`
- `internal/core/fetch/fetch.go` - export the same-origin policy
- `internal/core/diagnostic/codes.go` - four codes
- `internal/plugins/update-cmd/cmd/serve.go`, `serve_daemon.go` - serve the signed files, refuse without them
- `internal/appliance/cmd_build.go`, `manifest.go` - statement and SBOM after the image
- `internal/appliance/cmd_push.go` - mandatory statement, statement-first push, unverifying-device warning
- `internal/appliance/cmd_init.go`, `main.go` - generate the image key; `key` and `sign` subcommands
- `internal/appliance/doctor_checks.go` - `doctor-appliance-no-signing-key`
- `internal/le/vendorpatch/gokrazy_runtime_patch_test.go` - guard the new patch
- `internal/le/test/qemu/install_ota.go` - the `ota-signed` steps
- `gokrazy/ze/config.json`, `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod` - key file in root, `pkg/updatesig` replace
- `docs/architecture/appliance/self-update.md`, `docs/architecture/config/system-update.md`, `docs/architecture/appliance/ota-push.md`, `docs/architecture/appliance/gokrazy-build-pins.md`, `docs/architecture/appliance/builder.md`, `docs/guide/self-update.md`, `docs/guide/appliance.md`
- `docs/architecture/appliance/build-artifacts.md` - declared by `internal/appliance/doctor_checks.go`; gains the `.statement`, `.statement.sig` and `.spdx.json` artifacts beside each image
- `docs/features/ai-first.md` - declared by `internal/core/diagnostic/codes.go` and `internal/component/config/system/doctor.go`; its list of built-in diagnostic codes gains the four new codes

## Files to Create
- `pkg/updatesig/verify.go`, `sign.go`, `statement.go` - SSHSIG verify and sign, the two statement kinds
- `pkg/updatesig/releasekeys/` - the release public keys, current and next
- `pkg/updatesig/testdata/openssh/` - `ssh-keygen` fixtures
- `internal/appliance/statement.go`, `sbom.go`, `cmd_sign.go`, `cmd_key.go`
- `gokrazy/patches/signed-update.patch`
- the `.ci` files in the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-system-conf.yang` `update-check trusted-key` |
| YANG validation constraints | Yes | `max-elements 16`, `length` bound; key syntax by `ze:validate` |
| YANG custom validators | Yes | `ValidateFn` applying the `authorizedkeys` one-line rule and refusing non-Ed25519 types |
| CLI commands/flags | Yes | `ze appliance key generate`, `key show`, `sign binary`, `sign image` (build-host tooling, `internal/appliance/main.go`) |
| CLI grammar (keyword before value) | Yes | `ze appliance sign binary <path> version <v>`; `ai/rules/cli.md` |
| Editor autocomplete | No | `trusted-key` is free text |
| Functional test for new RPC/API | Yes | Functional Tests table |
| Pipe completeness | N-A | No new daemon command answer; `show system update` gains a field through its existing renderer |
| Env var registration | No | No new env var; the test override exists |
| Doctor check for runtime dependencies | Yes | `doctor-update-no-trusted-key` (config check), `doctor-update-signature-invalid` (report bus, raised at check time), `doctor-image-verification-inactive` (device probe), `doctor-appliance-no-signing-key` (host); each in `codes.go` with a `.ci` |
| Prometheus counters/metrics | No | History and report bus carry the outcome |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: signed updates and image SBOMs |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (`trusted-key`) |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md`, `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | No daemon RPC added |
| 5 | Plugin added/changed? | Yes | `update-cmd` serve behaviour in `docs/guide/self-update.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/self-update.md` (manifest format, Custom Server, signing), `docs/guide/appliance.md` |
| 7 | Wire format changed? | Yes | The manifest and statement, in `docs/architecture/appliance/self-update.md` |
| 8 | Plugin SDK/protocol changed? | No | `pkg/updatesig` is a new public package, documented in its package doc |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | Yes | QEMU OTA steps in `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` row on signed updates (VyOS signs images) |
| 12 | Internal architecture changed? | Yes | `system-update.md`, `ota-push.md`, `gokrazy-build-pins.md`, `builder.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes beyond doctor codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-selfupdate-manifest-authenticity.md` at implementation; the pages above are the `// Design:` targets |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/self-update.md` Custom Server JSON is rewritten to the signed form |

## Implementation Steps

1. **Phase: Wiring** -- `pkg/updatesig` with verify and sign returning "not implemented", called from every entry point in the Wiring Test table; the wiring tests fail for that reason.
2. **Phase: Verifier** -- SSHSIG verify and sign, statements, OpenSSH fixtures (unit tests, A-1).
3. **Phase: Daemon** -- self-updater and checker, `isNewer` deletion, same-origin client, re-hash at stage, `trusted-key`, doctor codes, `update serve`, docs; daemon `.ci` tests.
4. **Phase: Build host** -- image key in `init`, `key` and `sign` subcommands, statement and SBOM in `build`, mandatory statement in `push`, host doctor check, docs.
5. **Phase: Device** -- `signed-update.patch`, builddir replace and key file (A-2), pin guard, QEMU `ota-signed`, device doctor check, docs.
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
- [ ] Scoped tests of every package and suite the change reaches pass; the whole-tree check is CI's (`ai/META-DECISIONS.md`): `./le test unit pkg path ./pkg/updatesig/`, `./le test unit pkg path ./internal/component/config/system/`, `./le test unit pkg path ./internal/plugins/update-cmd/cmd/`, `./le test unit pkg path ./internal/appliance/`, `./le test unit pkg path ./internal/le/vendorpatch/`, `./le test functional plugin`, `./le test functional ui`, `./le test functional appliance`, `./le test qemu installer-scenarios` (the OTA proof), `./le build gosum`
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
