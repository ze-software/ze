# Spec: appliance-1-builder

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/patterns/cli-command.md` - CLI command structure
4. `docs/guide/appliance.md` - current appliance docs
5. `cmd/ze/init/main.go` - ze init implementation
6. `cmd/ze/data/main.go` - ze data implementation
7. `Makefile` lines 575-690 - ze-gokrazy target

## Task

Replace the unintuitive `make ze-gokrazy USER=x PASS=y CERTNAME=z` workflow with a `ze appliance` CLI command that:

1. Reads configuration from a JSON file (or prompts interactively on first run)
2. Persists secrets encrypted at rest, protected by an optional passphrase
3. Stores passwords only as non-reversible hashes (bcrypt); plaintext passwords never persist on disk
4. Derives the ZeFS blob deterministically from config + secrets at build time (immutable infrastructure: the blob is an artifact, not the source of truth)
5. Invokes gok to build full images or push OTA A/B updates to running devices
6. Runs from a bastion server over a dedicated management network to manage a fleet
7. Supports both self-signed and CA-signed TLS certificates with rotation
8. Supports batch init and batch build for fleet provisioning from a manifest file
9. Supports arm64 in addition to amd64 (when gokrazy provides an ARM kernel)
10. Provides a passphrase agent (`ze appliance unlock`) to avoid repeated prompts during fleet operations
11. Pushes OTA updates to running devices via gokrazy HTTP update API (`ze appliance push`)
12. Writes SHA-256 checksums alongside images; embeds a build manifest in ZeFS for auditability
13. Supports SSH authorized keys in addition to password authentication
14. Previews merged config without building (`ze appliance config --merged`)
15. Uses a separate per-device update token for gokrazy OTA (not the admin password)
16. Pushes config changes to running devices via SSH without full image rebuild (`ze appliance config-push`), with automatic revert on validation failure
17. Supports parallel OTA push to multiple devices (`push --all --parallel N`)
18. Exports and imports entire appliance directories for bastion disaster recovery (`ze appliance export/import`)
19. Embeds last-known-good config hash in ZeFS; device auto-reverts pushed config on validation failure

**Immutable infrastructure model:** the appliance directory (config + encrypted secrets) is the source of truth. The ZeFS blob and disk image are derived artifacts, regenerated from scratch on every build. For structural changes (new binary, new cert, new password), you rebuild and redeploy. For config-only changes (BGP peers, firewall rules, interface settings), `ze appliance config-push` provides a lightweight path that updates the running device's config via SSH without a full rebuild, with automatic revert if the new config fails validation. `ze fleet` (separate spec) adds orchestration, coordination, and monitoring on top.

**Security model:** if the bastion is compromised and the appliance directory is copied, the encrypted secrets are useless without the passphrase. The passphrase is never stored on disk; it is provided via `ze appliance unlock` (passphrase agent, recommended) or interactive prompt. The agent holds the derived key in a short-lived process via Unix domain socket, never on disk. `ZE_APPLIANCE_PASSPHRASE` env var exists for CI convenience but is NOT suitable for production (visible in `/proc/<pid>/environ`). Config-push uses SSH public key authentication (the operator's key, already baked into the appliance); no additional credentials are transmitted. The device validates the pushed config before applying and auto-reverts on failure.

**Auth model:** each appliance has one superuser account baked into the ZeFS database, with password and/or SSH authorized keys. Additional users are managed at runtime via ze's config (local accounts) or RADIUS (future). The superuser can be disabled (`credentials.admin_enabled: false`) for RADIUS-only deployments; the credentials remain in ZeFS for emergency serial console recovery but SSH/web login is rejected. The gokrazy HTTP update interface uses a separate per-device update token (randomly generated at init, stored encrypted in secrets/); this limits blast radius if the admin password leaks.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/appliance.md` - full appliance guide
  → Constraint: image layout is fixed (root squashfs + /perm ext4), ZeFS goes in /perm/ze/database.zefs
  → Decision: gokrazy config lives in gokrazy/ze/config.json, seed config baked into root fs
- [ ] `docs/architecture/zefs-format.md` - ZeFS blob store format
  → Constraint: keys use registered patterns from pkg/zefs

### Source Files
- [ ] `cmd/ze/init/main.go` - current bootstrap (credentials + TLS cert generation)
  → Constraint: reuse bcrypt hashing, zeweb.GenerateWebCertWithNames; not reimplement
  → Constraint: RunWithReader exists but couples reading + DB creation; appliance needs separate steps
- [ ] `cmd/ze/data/main.go` - ZeFS CLI (write/import/ls/cat/rm)
  → Constraint: reuse openOrCreateStore / zefs.Create patterns
- [ ] `Makefile` ze-gokrazy target (lines 641-690) - current build orchestration
  → Decision: the new command replaces the shell logic inside the Makefile target
- [ ] `internal/component/web/server.go:307-314` - GenerateWebCertWithNames(addr, names)
  → Constraint: returns (certPEM, keyPEM, error); takes listen addr + extra DNS SANs
  → Constraint: `certValidityDuration = 365 * 24 * time.Hour` (line 43) -- certs expire after 1 year
  → Decision: appliance cert generation must use a longer validity (10 years) or accept a duration parameter

**Key insights:**
- `ze init` already does credential creation + TLS cert; appliance composes on top
- `ze data write` already injects arbitrary keys into ZeFS; appliance uses zefs package directly
- No TOML dependency exists; JSON is the natural choice (gokrazy/ze/config.json is already JSON)
- The cert caching logic in the Makefile (CERTNAME -> tmp/gokrazy/certs/) is the exact pain point
- Self-signed cert validity is only 365 days; needs a longer default or `renew-cert` command
- `ze init --managed` writes `meta/instance/managed`; appliance must do the same
- All appliances currently share one `gokrazy/ze/config.json` (one hostname, one update password); must patch both Hostname and Update.HTTPPassword per-appliance
- Gokrazy update password uses a separate per-device random token (not the admin password) to limit blast radius; token stored encrypted in secrets/update.token, injected into temp config.json at build time
- ZeFS is a derived artifact, not the source of truth; the appliance dir is the source of truth
- database.zefs contains plaintext secrets (runtime need); must be treated as sensitive, auto-deleted after image inject, never left on bastion
- `golang.org/x/crypto` v0.50.0 already a dependency; chacha20 vendored. Need to vendor argon2 + chacha20poly1305 for secrets encryption
- Bastion deployment: `ze appliance` commands run on a management server, push to devices over management network
- Config layering (base + overlay) uses ze's `set`/`delete` command semantics; later commands override earlier; build validates merged config
- ARM support: gokrazy supports arm64; if an ARM kernel package exists, `image.arch: "arm64"` should work with cross-compilation
- Gokrazy HTTP update API allows OTA push without physical access; `ze appliance push` wraps this
- Image integrity: SHA-256 checksum alongside image; build manifest in ZeFS for runtime auditability
- SSH authorized keys: ze's SSH server supports public key auth; appliance should support baking in authorized_keys
- Passphrase agent pattern (like ssh-agent): hold derived key in memory via Unix domain socket, avoid repeated prompts
- Config-push via SSH closes the day-2 usability gap: config changes take seconds, not minutes (no rebuild)
- Last-known-good hash in ZeFS provides a revert target when pushed configs fail at runtime
- Export/import archives are the bastion's backup strategy; encrypted even if appliance secrets are not
- Parallel push transforms fleet update from O(N*T) to O(N/P*T) where P = parallelism

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/init/main.go` - bootstraps ZeFS with SSH creds, optional TLS cert via prompts or stdin pipe
- [ ] `cmd/ze/data/main.go` - manual blob store manipulation (write, import, ls, cat, rm)
- [ ] `Makefile:641-690` - ze-gokrazy orchestrates: init, cert cache, data write, gok, ext4 inject
- [ ] `gokrazy/ze/config.json` - gokrazy instance config (packages, env vars, kernel)
- [ ] `gokrazy/ze/ze.conf` - seed config template (set commands for ssh, web, ntp, dhcp)

**Behavior to preserve:**
- ZeFS key layout (meta/ssh/username, meta/ssh/password, meta/ssh/host, meta/ssh/port, meta/instance/name, meta/web/cert, meta/web/key, file/template/ze.conf)
- gok invocation: `GOARCH=<arch> bin/gok --parent_dir gokrazy -i ze overwrite --full <img> --target_storage_bytes <size>`
- ext4 partition inject: dd extract /perm, debugfs mkdir ze, debugfs write database.zefs, dd write back
- gokrazy/ze/config.json structure unchanged
- bcrypt password hashing (same cost factor)

**Behavior to change:**
- Credential+cert+config assembly: from Makefile shell + env vars to structured config file
- Secret persistence: from ad-hoc tmp/gokrazy/certs/<certname>/ to named appliance directory
- User interface: from `make ze-gokrazy USER=x PASS=y CERTNAME=z` to `ze appliance build [name]`

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze appliance init <name>` (interactive wizard) or `ze appliance build <name>` (from config)
- Configuration from `<appliance-dir>/<name>/appliance.json`
- Appliance directory resolved: `--dir` flag > `ZE_APPLIANCE_DIR` env > `~/.config/ze/appliances/`

### Transformation Path
1. Resolve appliance directory and load `<name>/appliance.json` into ApplianceConfig struct
2. Obtain passphrase: from agent socket (if `ze appliance unlock` active), interactive prompt, or env var (with warning)
3. Decrypt secrets in memory: password.hash, tls/key.pem, update.token (tls/cert.pem is plaintext)
4. If `config_base` set, load shared ze.conf; overlay per-appliance ze.conf on top; validate merged config
5. Create ZeFS database via `zefs.Create()`: write SSH creds, SSH authorized keys, TLS cert+key, managed flag, admin-disabled flag, seed config, build manifest
6. Zero decrypted secrets from memory
7. (build) Decrypt update token from secrets/update.token
8. (build) Generate per-appliance gokrazy config.json (patch Hostname + Update.HTTPPassword from update token)
9. Write database to `<name>/database.zefs`; delete immediately unless `--keep` flag
10. (build) Invoke gok subprocess for image build, format /perm, inject database.zefs, delete database.zefs, clean temp config
11. (build) Write `<name>/ze-<timestamp>.img.sha256` checksum; write `<name>/build.json` manifest
12. (build) Zero update token from memory
13. (push) Push image to device via gokrazy HTTP update API using update token over TLS
14. (config-push) Resolve merged config, SSH to device, upload to /perm/ze/config-pushed.conf, device validates + applies or auto-reverts
15. (export) Tar appliance dir(s) into encrypted archive; (import) decrypt + extract to target dir

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> Go struct | encoding/json Unmarshal | [ ] |
| Encrypted secrets -> memory | Argon2id KDF + ChaCha20-Poly1305 AEAD decrypt; plaintext only in memory | [ ] |
| Decrypted secrets -> ZeFS | In-memory plaintext -> zefs.WriteFile; zeroed after | [ ] |
| ZeFS -> disk image | dd + debugfs (same as current Makefile) | [ ] |
| Go -> gok subprocess (build) | exec.Command with GOARCH env, --full flag | [ ] |
| Base config + overlay -> seed ze.conf | Read base, append per-appliance overrides, validate merged config | [ ] |
| appliance.json -> gokrazy config.json | JSON patch Hostname + Update.HTTPPassword (from decrypted update token); temp file deleted after gok | [ ] |
| Update token -> gokrazy config | Decrypted from secrets/update.token, injected into temp config.json, zeroed after | [ ] |
| Passphrase -> agent socket | `ze appliance unlock` derives key via Argon2id, holds in memory, serves via Unix socket | [ ] |
| Image -> checksum | SHA-256 of final .img written to .img.sha256 | [ ] |
| Build metadata -> build.json | Config hash, timestamp, ze version, arch written alongside image | [ ] |
| Image -> device (push) | gokrazy HTTP update API over TLS; update token as HTTP basic auth password | [ ] |
| Config -> device (config-push) | SSH exec: upload merged ze.conf to /perm/ze/config-pushed.conf; device validates, applies, or reverts | [ ] |
| Appliance dir -> archive (export) | tar + ChaCha20-Poly1305 AEAD encryption of entire appliance dir; passphrase required | [ ] |
| Archive -> appliance dir (import) | AEAD decrypt + tar extract; validates structure before overwriting | [ ] |
| Last-known-good hash -> ZeFS | SHA-256 of validated seed config written to meta/config/last-known-good at build time | [ ] |

### Integration Points
- `internal/component/web` - zeweb.GenerateWebCertWithNames for TLS cert generation (needs duration parameter)
- `golang.org/x/crypto/bcrypt` - password hashing (same as cmd/ze/init)
- `golang.org/x/crypto/argon2` - KDF for passphrase -> encryption key (needs vendoring)
- `golang.org/x/crypto/chacha20poly1305` - AEAD for secret encryption (needs vendoring)
- `pkg/zefs` - BlobStore Create/Open/WriteFile/Close
- `bin/gok` - external subprocess for image build
- `gokrazy/ze/config.json` - base template, patched per-appliance (hostname)

### Architectural Verification
- [ ] No bypassed layers (reuses zefs package, web cert generation)
- [ ] No unintended coupling (appliance is offline CLI, no daemon dependency)
- [ ] No duplicated functionality (composes existing init/data primitives)
- [ ] Zero-copy preserved where applicable (N/A, offline tool)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance init lab` CLI | -> | `cmd/ze/appliance/cmd_init.go:cmdInit()` | `TestInitCreatesApplianceDir` |
| `ze appliance build lab` CLI | -> | `cmd/ze/appliance/cmd_build.go:cmdBuild()` | `TestBuildProducesZeFS` |
| `ze appliance assemble lab` CLI | -> | `cmd/ze/appliance/cmd_assemble.go:cmdAssemble()` | `TestAssembleProducesZeFS` |
| `ze appliance passwd lab` CLI | -> | `cmd/ze/appliance/cmd_passwd.go:cmdPasswd()` | `TestPasswdUpdatesHash` |
| `ze appliance replace-cert lab` CLI | -> | `cmd/ze/appliance/cmd_cert.go:cmdReplaceCert()` | `TestReplaceCertUpdatesSecrets` |
| `ze appliance rekey lab` CLI | -> | `cmd/ze/appliance/cmd_rekey.go:cmdRekey()` | `TestRekeyChangesEncryption` |
| `ze appliance clone lab lab2` CLI | -> | `cmd/ze/appliance/cmd_clone.go:cmdClone()` | `TestCloneCopiesConfigNotSecrets` |
| `ze appliance list` CLI | -> | `cmd/ze/appliance/cmd_list.go:cmdList()` | `TestListShowsAppliances` |
| `ze appliance show lab` CLI | -> | `cmd/ze/appliance/cmd_show.go:cmdShow()` | `TestShowDisplaysConfigAndCertExpiry` |
| `ze appliance init --batch m.json` CLI | -> | `cmd/ze/appliance/cmd_init.go:cmdBatchInit()` | `TestBatchInitCreatesMultiple` |
| `ze appliance unlock` CLI | -> | `cmd/ze/appliance/cmd_unlock.go:cmdUnlock()` | `TestUnlockStartsAgent` |
| `ze appliance push lab` CLI | -> | `cmd/ze/appliance/cmd_push.go:cmdPush()` | `TestPushSendsImage` |
| `ze appliance config lab --merged` CLI | -> | `cmd/ze/appliance/cmd_config.go:cmdConfig()` | `TestConfigMergedOutput` |
| `ze appliance build --all` CLI | -> | `cmd/ze/appliance/cmd_build.go:cmdBuildAll()` | `TestBuildAllIteratesAppliances` |
| `ze appliance config-push lab` CLI | -> | `cmd/ze/appliance/cmd_config_push.go:cmdConfigPush()` | `TestConfigPushUploadsConfig` |
| `ze appliance export lab` CLI | -> | `cmd/ze/appliance/cmd_export.go:cmdExport()` | `TestExportCreatesArchive` |
| `ze appliance import archive.ze` CLI | -> | `cmd/ze/appliance/cmd_import.go:cmdImport()` | `TestImportRestoresAppliance` |
| `cmd/ze/main.go` dispatch | -> | `cmd/ze/appliance/main.go:Run()` | `TestMainDispatchAppliance` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze appliance init lab` with piped stdin (user/pass/host/port/certname) | Creates dir with appliance.json, secrets/tls/cert.pem, secrets/tls/key.pem, secrets/password.hash |
| AC-2 | `ze appliance build lab` after init | Produces database.zefs with keys: meta/ssh/username, meta/ssh/password, meta/ssh/host, meta/ssh/port, meta/instance/name, meta/instance/managed, meta/web/cert, meta/web/key, file/template/ze.conf |
| AC-3 | Two consecutive `ze appliance build lab` | cert.pem SHA256 unchanged between builds |
| AC-4 | `ze appliance init lab` with all defaults accepted | appliance.json has defaults: admin, 0.0.0.0, 22, 8080, amd64, 2G, managed=false, validity_years=10 |
| AC-5 | `ze appliance list` with two appliances configured | Output lists both names with hostname and target |
| AC-6 | `ze appliance build lab` with missing cert.pem | Exit code 1, stderr contains "missing secrets/tls/cert.pem" |
| AC-7 | `ze appliance build lab` with config_base + per-appliance ze.conf | database.zefs file/template/ze.conf = base + overlay concatenated |
| AC-8 | `ze appliance init lab --config input.json` | Produces same result as interactive with equivalent input |
| AC-9 | `ze appliance passwd lab` with new password piped | secrets/password.hash changes; appliance.json unchanged |
| AC-10 | `ze appliance replace-cert lab --cert ca.pem --key ca.key` | secrets/tls/ updated with provided files |
| AC-11 | `ze appliance replace-cert lab` (no flags) | Regenerates self-signed cert using tls.cert_name from config |
| AC-12 | `ze appliance clone lab lab2` | Creates lab2/ with copied appliance.json (name/hostname patched), no secrets copied |
| AC-13 | `ze appliance show lab` | Output includes TLS cert expiry date |
| AC-14 | `ze appliance assemble lab` | Produces database.zefs without invoking gok (fast path) |
| AC-15 | `ze appliance build lab` with identity.hostname="edge-01" | Generated gokrazy config.json has Hostname="edge-01" |
| AC-16 | `ze appliance init lab --cert ext.pem --key ext.key` | CA-signed cert copied into secrets/tls/, key encrypted, no self-signed generated |
| AC-17 | `ze appliance run lab` with qemu ports configured | QEMU launched with appliance-specific port forwarding |
| AC-18 | `ze appliance init lab` with passphrase set | `secrets/.encrypted` marker created; password.hash and key.pem are encrypted; cert.pem is plaintext |
| AC-19 | `ze appliance init lab` with empty passphrase | `secrets/.encrypted` absent; all files plaintext |
| AC-20 | `ze appliance assemble lab` with wrong passphrase | Exit code 1, stderr "error: decryption failed" (no partial output) |
| AC-21 | `ze appliance rekey lab` | All encrypted secrets re-encrypted with new passphrase; decrypted content identical |
| AC-22 | Copy entire appliance dir to another machine, attempt `ze appliance assemble` without passphrase | Fails; database.zefs not produced |
| AC-23 | `ze appliance assemble lab` with correct passphrase | database.zefs produced; decrypted secrets never written to any file on disk |
| AC-24 | `ze appliance build lab` | Prompts for admin password; temp config.json has correct Update.HTTPPassword; database.zefs deleted after image inject |
| AC-25 | `ze appliance build lab` with wrong admin password | Exit code 1, stderr "error: admin password does not match stored hash" |
| AC-26 | `ze appliance build lab` with `ZE_APPLIANCE_SSH_PASSWORD` env var | Prints warning about env var; uses password from env |
| AC-27 | `ze appliance init lab` with `admin_enabled: false` | appliance.json has `credentials.admin_enabled: false`; assemble produces `meta/instance/admin-disabled` in ZeFS |
| AC-28 | `ze appliance init --batch manifest.json` with 3 entries | Creates 3 appliance directories, each with config + encrypted secrets |
| AC-29 | `ze appliance build lab` produces image | Image filename is `ze-<timestamp>.img`, not `ze.img` |
| AC-30 | `ze appliance run lab` with port 2222 already in use | Exit code 1, stderr lists conflicting port and process |
| AC-31 | `ze appliance build lab` with `image.arch: "arm64"` | gok invoked with `GOARCH=arm64` |
| AC-32 | `ze appliance assemble lab` with config_base + overlay containing `delete` command | Merged config removes the deleted setting; validation passes |
| AC-33 | `ze appliance assemble lab` with invalid merged config | Exit code 1, stderr identifies which file (base or overlay) and line caused the error |
| AC-34 | `ze appliance build lab` with `ZE_APPLIANCE_PASSPHRASE` env var | Prints "WARNING: passphrase from environment variable (not recommended for production)" |
| AC-35 | `ze appliance unlock` with correct passphrase | Agent starts; subsequent commands skip passphrase prompt; agent PID printed |
| AC-36 | `ze appliance unlock --duration 15m` | Agent exits after 15 minutes; key material zeroed |
| AC-37 | `ze appliance build lab` with agent running | No passphrase prompt; passphrase obtained from agent socket |
| AC-38 | `ze appliance unlock --stop` | Running agent stopped; socket removed |
| AC-39 | `ze appliance push lab` after build | Image pushed to device via gokrazy HTTP update API; device reboots to new partition |
| AC-40 | `ze appliance push lab --image ze-20260427-143022.img` | Specific image pushed (rollback to previous build) |
| AC-41 | `ze appliance push lab` with device unreachable | Exit code 1, stderr "error: device edge-01 unreachable at <hostname>:<port>" |
| AC-42 | `ze appliance build --all` with 3 appliances | All 3 images built sequentially; summary printed |
| AC-43 | `ze appliance push --all` with 3 appliances | All 3 devices updated; per-device status printed |
| AC-44 | `ze appliance build lab` produces checksum | `ze-<timestamp>.img.sha256` written alongside image; matches `sha256sum` output |
| AC-45 | `ze appliance build lab` produces build manifest | `build.json` written: config_hash, timestamp, ze_version, arch, image_sha256 |
| AC-46 | `ze appliance config lab --merged` | Prints effective config (base + overlay merged); no build performed |
| AC-47 | `ze appliance config lab --merged` with config_base | Output shows base settings overridden by overlay |
| AC-48 | `ze appliance init lab` with `credentials.ssh_authorized_keys` | Keys written to `secrets/authorized_keys`; baked into ZeFS as `meta/ssh/authorized_keys` |
| AC-49 | `ze appliance init lab` generates update token | Random 32-byte token generated; stored encrypted in `secrets/update.token`; not the admin password |
| AC-50 | `ze appliance show lab` with `managed: true` | Output explains: "managed: fleet mode (accepts remote config push, reports to hub)" |
| AC-51 | `ze appliance init --batch manifest.json` with `"password": "generate"` | Each appliance gets a unique random password; passwords printed to stdout (sealed output) |
| AC-52 | `ze appliance assemble lab` without `--keep` | database.zefs auto-deleted after assembly; warning printed |
| AC-53 | `ze appliance assemble lab --keep` | database.zefs retained; sensitivity warning printed |
| AC-54 | `ze appliance push lab` with wrong update token | Exit code 1, stderr "error: device rejected update (401 Unauthorized)" |
| AC-55 | `ze appliance config-push lab` with valid config | Config uploaded to device via SSH; device validates and applies; bastion prints "config applied to edge-01" |
| AC-56 | `ze appliance config-push lab` with invalid config | Device validates pushed config, rejects it, keeps previous config; bastion prints "error: device rejected config (validation failed: <detail>)" |
| AC-57 | `ze appliance config-push lab` with device unreachable | Exit code 1, stderr "error: device edge-01 unreachable at <address>:<port>" |
| AC-58 | `ze appliance config-push lab --dry-run` | Prints merged config that would be pushed; no SSH connection made |
| AC-59 | `ze appliance config-push --all` with 3 appliances | All 3 devices updated; per-device status printed |
| AC-60 | `ze appliance config-push lab` after config change | Device applies new config; old config saved as /perm/ze/config-previous.conf for manual recovery |
| AC-61 | `ze appliance push --all --parallel 4` with 8 appliances | 4 concurrent uploads; all 8 devices updated; per-device status printed |
| AC-62 | `ze appliance push --all --parallel 1` | Sequential push (same as without --parallel) |
| AC-63 | `ze appliance config-push --all --parallel 4` | 4 concurrent SSH sessions; per-device status printed |
| AC-64 | `ze appliance export lab` | Creates `lab.ze.enc` archive containing appliance.json + secrets/ + ze.conf; encrypted with passphrase |
| AC-65 | `ze appliance export --all` | Creates `appliances-<timestamp>.ze.enc` archive containing all appliance directories |
| AC-66 | `ze appliance import lab.ze.enc` | Restores appliance directory from archive; prompts before overwriting existing |
| AC-67 | `ze appliance import lab.ze.enc` with wrong passphrase | Exit code 1, stderr "error: decryption failed" |
| AC-68 | `ze appliance import lab.ze.enc --dir /new/bastion` | Restores to specified directory (bastion migration) |
| AC-69 | `ze appliance export lab` without passphrase set | Exit code 1, stderr "error: export requires encryption passphrase (archives always encrypted)" |
| AC-70 | `ze appliance build lab` produces last-known-good hash | ZeFS contains `meta/config/last-known-good` with SHA-256 of validated seed config |
| AC-71 | `ze appliance config-push lab` with config that passes validation but causes runtime failure | Device detects failure (health check timeout), reverts to last-known-good config from ZeFS seed, prints revert reason to device log |
| AC-72 | `ze appliance config-push lab` updates last-known-good | After device confirms applied config is healthy, device updates /perm/ze/last-known-good with new config hash |
| AC-73 | Device boots with no config-pushed.conf | Uses ZeFS seed config (last-known-good baseline); normal boot path unchanged |
| AC-74 | Device boots with config-pushed.conf that fails validation | Ignores pushed config, uses ZeFS seed config, logs warning |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigDefaults` | `cmd/ze/appliance/config_test.go` | Default values populated correctly (incl managed=false, validity_years=10) | |
| `TestConfigMarshalRoundtrip` | `cmd/ze/appliance/config_test.go` | JSON marshal/unmarshal preserves all fields including new ones | |
| `TestConfigValidation` | `cmd/ze/appliance/config_test.go` | Port range, name chars, size minimum validated | |
| `TestInitCreatesApplianceDir` | `cmd/ze/appliance/cmd_init_test.go` | Init creates dir structure with config + secrets | |
| `TestInitPasswordNeverInJSON` | `cmd/ze/appliance/cmd_init_test.go` | appliance.json never contains password field | |
| `TestInitGeneratesCert` | `cmd/ze/appliance/cmd_init_test.go` | TLS cert/key generated with correct DNS SAN and 10-year validity | |
| `TestInitWithCACert` | `cmd/ze/appliance/cmd_init_test.go` | --cert/--key copies provided files into secrets/tls/ | |
| `TestInitFromConfigFile` | `cmd/ze/appliance/cmd_init_test.go` | --config flag reads pre-filled JSON | |
| `TestInitManagedFlag` | `cmd/ze/appliance/cmd_init_test.go` | managed=true in config produces meta/instance/managed=true | |
| `TestAssembleProducesZeFS` | `cmd/ze/appliance/cmd_assemble_test.go` | database.zefs created with all expected keys incl managed | |
| `TestAssembleReusesExistingCert` | `cmd/ze/appliance/cmd_assemble_test.go` | Cert unchanged between assembles | |
| `TestAssembleMissingSecretsFails` | `cmd/ze/appliance/cmd_assemble_test.go` | Clear error when secrets dir incomplete | |
| `TestAssembleConfigLayering` | `cmd/ze/appliance/cmd_assemble_test.go` | config_base + per-appliance ze.conf concatenated | |
| `TestAssembleDefaultZeConf` | `cmd/ze/appliance/cmd_assemble_test.go` | Falls back to gokrazy/ze/ze.conf when no config_base and no local ze.conf | |
| `TestAssembleHostnamePatch` | `cmd/ze/appliance/cmd_assemble_test.go` | Generated gokrazy config.json has patched Hostname | |
| `TestBuildProducesZeFS` | `cmd/ze/appliance/cmd_build_test.go` | Full build produces database.zefs + invokes gok (mocked) | |
| `TestPasswdUpdatesHash` | `cmd/ze/appliance/cmd_passwd_test.go` | New password hashed and written; old hash replaced | |
| `TestReplaceCertUpdatesSecrets` | `cmd/ze/appliance/cmd_cert_test.go` | CA cert copied into secrets/tls/ | |
| `TestReplaceCertRegenerates` | `cmd/ze/appliance/cmd_cert_test.go` | No --cert flag regenerates self-signed with config cert_name | |
| `TestCloneCopiesConfigNotSecrets` | `cmd/ze/appliance/cmd_clone_test.go` | Config copied with patched name/hostname; secrets dir not copied | |
| `TestListShowsAppliances` | `cmd/ze/appliance/cmd_list_test.go` | Lists directories with hostname + target columns | |
| `TestShowDisplaysConfigAndCertExpiry` | `cmd/ze/appliance/cmd_show_test.go` | Prints config summary + cert expiry; no secrets shown | |
| `TestMainDispatchAppliance` | `cmd/ze/appliance/main_test.go` | Run() dispatches to correct subcommand | |
| `TestDirResolution` | `cmd/ze/appliance/main_test.go` | --dir flag > env > default ~/.config/ze/appliances/ | |
| `TestEncryptDecryptRoundtrip` | `cmd/ze/appliance/crypto_test.go` | Argon2id + ChaCha20-Poly1305 encrypt/decrypt produces identical plaintext | |
| `TestEncryptedInitCreatesMarker` | `cmd/ze/appliance/cmd_init_test.go` | Passphrase set -> .encrypted marker; key.pem encrypted; cert.pem plaintext | |
| `TestPlaintextInitNoMarker` | `cmd/ze/appliance/cmd_init_test.go` | Empty passphrase -> no .encrypted; all files plaintext | |
| `TestAssembleWrongPassphraseFails` | `cmd/ze/appliance/cmd_assemble_test.go` | Wrong passphrase -> AEAD auth error, exit 1, no partial ZeFS | |
| `TestAssembleDecryptedSecretsNotOnDisk` | `cmd/ze/appliance/cmd_assemble_test.go` | After assemble, no plaintext secret files exist outside database.zefs | |
| `TestRekeyChangesEncryption` | `cmd/ze/appliance/cmd_rekey_test.go` | Rekey with new passphrase; old passphrase fails; new succeeds; content identical | |
| `TestRekeyPlaintextToEncrypted` | `cmd/ze/appliance/cmd_rekey_test.go` | Rekey on unencrypted appliance adds encryption | |
| `TestRekeyEncryptedToPlaintext` | `cmd/ze/appliance/cmd_rekey_test.go` | Rekey with empty new passphrase removes encryption | |
| `TestBatchInitCreatesMultiple` | `cmd/ze/appliance/cmd_init_test.go` | Batch manifest creates N appliance dirs with config + secrets | |
| `TestBatchInitMissingEnvFails` | `cmd/ze/appliance/cmd_init_test.go` | Batch without ZE_APPLIANCE_SSH_PASSWORD env var fails | |
| `TestBuildPromptsAdminPassword` | `cmd/ze/appliance/cmd_build_test.go` | Build verifies admin password against stored bcrypt hash | |
| `TestBuildWrongAdminPasswordFails` | `cmd/ze/appliance/cmd_build_test.go` | Wrong admin password -> exit 1, clear error | |
| `TestBuildDeletesDatabaseZeFS` | `cmd/ze/appliance/cmd_build_test.go` | After build, database.zefs does not exist in appliance dir | |
| `TestBuildTimestampedImage` | `cmd/ze/appliance/cmd_build_test.go` | Image filename matches ze-YYYYMMDD-HHMMSS.img pattern | |
| `TestBuildPatchesUpdatePassword` | `cmd/ze/appliance/cmd_build_test.go` | Temp gokrazy config.json has Update.HTTPPassword set | |
| `TestInitAdminDisabled` | `cmd/ze/appliance/cmd_init_test.go` | admin_enabled=false produces meta/instance/admin-disabled in ZeFS | |
| `TestAssembleConfigValidation` | `cmd/ze/appliance/cmd_assemble_test.go` | Invalid merged config fails with file + line identification | |
| `TestAssembleDeleteOverride` | `cmd/ze/appliance/cmd_assemble_test.go` | Overlay `delete` command removes base setting | |
| `TestRunDetectsPortConflict` | `cmd/ze/appliance/cmd_run_test.go` | Port in use -> clear error with port number and process | |
| `TestBuildArm64` | `cmd/ze/appliance/cmd_build_test.go` | arch=arm64 invokes gok with GOARCH=arm64 | |
| `TestPassphraseEnvVarWarning` | `cmd/ze/appliance/crypto_test.go` | Passphrase from env var produces warning on stderr | |
| `TestUnlockStartsAgent` | `cmd/ze/appliance/cmd_unlock_test.go` | Agent starts, creates socket, responds to key requests | |
| `TestUnlockDurationExpiry` | `cmd/ze/appliance/cmd_unlock_test.go` | Agent exits after configured duration; socket removed | |
| `TestUnlockStop` | `cmd/ze/appliance/cmd_unlock_test.go` | --stop terminates running agent | |
| `TestAgentPassphraseResolution` | `cmd/ze/appliance/crypto_test.go` | Passphrase resolved: agent > interactive > env var (in priority order) | |
| `TestPushSendsImage` | `cmd/ze/appliance/cmd_push_test.go` | Push sends image to gokrazy HTTP update endpoint (mocked) | |
| `TestPushUnreachableDevice` | `cmd/ze/appliance/cmd_push_test.go` | Unreachable device -> clear error with hostname | |
| `TestPushWrongToken` | `cmd/ze/appliance/cmd_push_test.go` | 401 response -> clear error about update token | |
| `TestPushSpecificImage` | `cmd/ze/appliance/cmd_push_test.go` | --image flag pushes specific image (rollback) | |
| `TestBuildAllIteratesAppliances` | `cmd/ze/appliance/cmd_build_test.go` | --all builds every appliance in dir; summary printed | |
| `TestPushAllIteratesAppliances` | `cmd/ze/appliance/cmd_push_test.go` | --all pushes to every appliance; per-device status | |
| `TestBuildWritesChecksum` | `cmd/ze/appliance/cmd_build_test.go` | .img.sha256 written; content matches SHA-256 of image | |
| `TestBuildWritesManifest` | `cmd/ze/appliance/cmd_build_test.go` | build.json written with config_hash, timestamp, ze_version, arch | |
| `TestConfigMergedOutput` | `cmd/ze/appliance/cmd_config_test.go` | --merged prints effective config after base + overlay | |
| `TestConfigMergedWithDelete` | `cmd/ze/appliance/cmd_config_test.go` | Overlay delete removes base setting from merged output | |
| `TestInitGeneratesUpdateToken` | `cmd/ze/appliance/cmd_init_test.go` | Init generates random 32-byte update token in secrets/ | |
| `TestInitWithAuthorizedKeys` | `cmd/ze/appliance/cmd_init_test.go` | SSH authorized keys written to secrets/authorized_keys | |
| `TestAssembleIncludesAuthorizedKeys` | `cmd/ze/appliance/cmd_assemble_test.go` | ZeFS contains meta/ssh/authorized_keys | |
| `TestAssembleAutoDeleteZeFS` | `cmd/ze/appliance/cmd_assemble_test.go` | database.zefs auto-deleted after assemble (no --keep) | |
| `TestAssembleKeepRetainsZeFS` | `cmd/ze/appliance/cmd_assemble_test.go` | --keep flag retains database.zefs | |
| `TestBatchInitPerDevicePasswords` | `cmd/ze/appliance/cmd_init_test.go` | password=generate creates unique password per device; passwords printed | |
| `TestBuildManifestInZeFS` | `cmd/ze/appliance/cmd_build_test.go` | ZeFS contains meta/build/manifest with build metadata | |
| `TestShowManagedExplanation` | `cmd/ze/appliance/cmd_show_test.go` | managed=true shows explanation of fleet mode behavior | |
| `TestConfigPushUploadsConfig` | `cmd/ze/appliance/cmd_config_push_test.go` | Config pushed to device via SSH (mocked); device confirms apply | |
| `TestConfigPushInvalidConfigReverts` | `cmd/ze/appliance/cmd_config_push_test.go` | Device rejects invalid config; previous config retained | |
| `TestConfigPushUnreachableDevice` | `cmd/ze/appliance/cmd_config_push_test.go` | Unreachable device -> clear error with address | |
| `TestConfigPushDryRun` | `cmd/ze/appliance/cmd_config_push_test.go` | --dry-run prints config without connecting | |
| `TestConfigPushAllDevices` | `cmd/ze/appliance/cmd_config_push_test.go` | --all iterates all appliances with device.address set | |
| `TestConfigPushSavesPrevious` | `cmd/ze/appliance/cmd_config_push_test.go` | Device saves old config as config-previous.conf before applying | |
| `TestPushAllParallel` | `cmd/ze/appliance/cmd_push_test.go` | --parallel 4 runs 4 concurrent uploads; all succeed | |
| `TestPushAllParallelPartialFailure` | `cmd/ze/appliance/cmd_push_test.go` | --parallel with 1 failure: other devices succeed; failure reported at end | |
| `TestPushAllParallelDefault` | `cmd/ze/appliance/cmd_push_test.go` | --parallel 1 is equivalent to sequential push | |
| `TestConfigPushAllParallel` | `cmd/ze/appliance/cmd_config_push_test.go` | --parallel 4 runs 4 concurrent SSH sessions | |
| `TestExportCreatesArchive` | `cmd/ze/appliance/cmd_export_test.go` | Export produces encrypted .ze.enc file containing appliance dir | |
| `TestExportAllCreatesArchive` | `cmd/ze/appliance/cmd_export_test.go` | --all exports all appliances into single archive | |
| `TestExportRequiresPassphrase` | `cmd/ze/appliance/cmd_export_test.go` | Export without passphrase fails (archives always encrypted) | |
| `TestImportRestoresAppliance` | `cmd/ze/appliance/cmd_import_test.go` | Import decrypts and restores appliance directory | |
| `TestImportWrongPassphraseFails` | `cmd/ze/appliance/cmd_import_test.go` | Wrong passphrase -> AEAD auth error, exit 1 | |
| `TestImportPromptsBeforeOverwrite` | `cmd/ze/appliance/cmd_import_test.go` | Existing appliance dir -> prompt for confirmation | |
| `TestImportToNewDir` | `cmd/ze/appliance/cmd_import_test.go` | --dir flag restores to specified directory | |
| `TestExportImportRoundtrip` | `cmd/ze/appliance/cmd_export_test.go` | Export then import produces identical directory tree | |
| `TestBuildWritesLastKnownGood` | `cmd/ze/appliance/cmd_build_test.go` | ZeFS contains meta/config/last-known-good with SHA-256 of seed config | |
| `TestAssembleWritesLastKnownGood` | `cmd/ze/appliance/cmd_assemble_test.go` | ZeFS contains meta/config/last-known-good after assemble | |
| `TestLastKnownGoodHashMatchesSeedConfig` | `cmd/ze/appliance/cmd_build_test.go` | Hash in meta/config/last-known-good matches SHA-256 of file/template/ze.conf content | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| image.size_bytes | 512MB - 64GB | 68719476736 | 536870911 (below 512MB) | N/A (warn only) |
| ssh.port | 1-65535 | 65535 | 0 | 65536 |
| web.port | 1-65535 | 65535 | 0 | 65536 |
| tls.validity_years | 1-25 | 25 | 0 | N/A (warn only) |
| qemu.ssh_port | 1024-65535 | 65535 | 1023 | 65536 |
| --parallel N | 1-64 | 64 | 0 | N/A (clamped to device count) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-appliance-init-assemble` | `test/appliance/init-assemble.ci` | User inits then assembles; database.zefs has correct keys | |
| `test-appliance-passwd` | `test/appliance/passwd.ci` | User changes password; new hash in database.zefs | |
| `test-appliance-clone` | `test/appliance/clone.ci` | Clone + init secrets; both appliances build independently | |
| `test-appliance-config-merged` | `test/appliance/config-merged.ci` | Config preview matches what assemble produces | |
| `test-appliance-batch-init` | `test/appliance/batch-init.ci` | Batch init creates multiple appliances with unique credentials | |
| `test-appliance-unlock-cycle` | `test/appliance/unlock-cycle.ci` | Unlock, build, push sequence without repeated prompts | |
| `test-appliance-export-import` | `test/appliance/export-import.ci` | Export appliance, import to new dir, assemble from imported copy | |
| `test-appliance-config-push` | `test/appliance/config-push.ci` | Config-push to test device (mocked SSH); verify config applied | |

### Future (if deferring any tests)
- Full image build (gok invocation + ext4 inject) requires e2fsprogs + gok binary; functional test covers ZeFS assembly only
- OTA push test requires running gokrazy instance; manual verification only
- Passphrase agent integration test with concurrent builds requires multi-process coordination
- Config-push integration test requires running ze device with SSH; mocked in unit tests
- Parallel push stress test (high N) requires multiple running gokrazy instances
- Last-known-good revert test requires a device that detects runtime config failure (health check); mocked in unit tests

## Files to Modify
- `cmd/ze/main.go` - add "appliance" case to dispatch switch + import
- `internal/component/web/server.go` - add GenerateWebCertWithDuration (or accept duration parameter) for configurable cert validity
- `Makefile` - simplify ze-gokrazy targets to delegate to `ze appliance`
- `.gitignore` - add `appliances/` pattern

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (offline command) |
| CLI commands/flags | Yes | `cmd/ze/appliance/main.go`, `cmd/ze/main.go` |
| Editor autocomplete | No | N/A (offline command) |
| Functional test for new RPC/API | No | N/A (offline command) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` - add ze appliance section |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md` - CLI reference |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - already exists, extend it |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create
- `cmd/ze/appliance/main.go` - Run() + dispatch + --dir flag + dir resolution
- `cmd/ze/appliance/cmd_init.go` - Interactive wizard + config/secret generation + --cert/--key + --config
- `cmd/ze/appliance/cmd_assemble.go` - ZeFS assembly: config layering, hostname patch, managed flag
- `cmd/ze/appliance/cmd_build.go` - Full build: assemble + gok + ext4 inject
- `cmd/ze/appliance/cmd_passwd.go` - Password rotation (prompt + hash + write)
- `cmd/ze/appliance/cmd_cert.go` - TLS cert replacement (--cert/--key or regenerate self-signed)
- `cmd/ze/appliance/cmd_clone.go` - Copy config with patched identity, no secrets
- `cmd/ze/appliance/cmd_list.go` - List appliances with hostname/target columns
- `cmd/ze/appliance/cmd_show.go` - Show config + cert expiry, no secrets
- `cmd/ze/appliance/cmd_run.go` - QEMU boot with per-appliance port forwarding
- `cmd/ze/appliance/register.go` - cmdregistry registration
- `cmd/ze/appliance/config.go` - ApplianceConfig struct + JSON + defaults + validation
- `cmd/ze/appliance/crypto.go` - Argon2id KDF + ChaCha20-Poly1305 AEAD encrypt/decrypt, secret read/write helpers
- `cmd/ze/appliance/resolve.go` - Appliance dir resolution (--dir > env > default)
- `cmd/ze/appliance/cmd_rekey.go` - Change encryption passphrase (decrypt all, re-encrypt)
- `cmd/ze/appliance/cmd_unlock.go` - Passphrase agent: start/stop, Unix domain socket, duration-limited
- `cmd/ze/appliance/cmd_push.go` - OTA push via gokrazy HTTP update API (single device or --all)
- `cmd/ze/appliance/cmd_config.go` - Config preview: --merged shows effective config after layering
- `cmd/ze/appliance/agent.go` - Agent protocol: derive key, serve via socket, auto-expire, memory zeroing
- `cmd/ze/appliance/manifest.go` - Build manifest: config hash, timestamp, ze version, arch, image SHA-256
- `cmd/ze/appliance/config_test.go` - Config unit tests
- `cmd/ze/appliance/cmd_init_test.go` - Init command tests
- `cmd/ze/appliance/cmd_assemble_test.go` - Assemble command tests
- `cmd/ze/appliance/cmd_build_test.go` - Build command tests
- `cmd/ze/appliance/cmd_passwd_test.go` - Passwd command tests
- `cmd/ze/appliance/cmd_cert_test.go` - Cert command tests
- `cmd/ze/appliance/cmd_clone_test.go` - Clone command tests
- `cmd/ze/appliance/cmd_list_test.go` - List command tests
- `cmd/ze/appliance/cmd_show_test.go` - Show command tests
- `cmd/ze/appliance/cmd_rekey_test.go` - Rekey command tests
- `cmd/ze/appliance/crypto_test.go` - Encryption roundtrip + boundary tests
- `cmd/ze/appliance/main_test.go` - Dispatch + dir resolution tests
- `cmd/ze/appliance/cmd_run_test.go` - QEMU port conflict detection tests
- `cmd/ze/appliance/cmd_unlock_test.go` - Agent start/stop/expiry tests
- `cmd/ze/appliance/cmd_push_test.go` - OTA push tests (mocked HTTP)
- `cmd/ze/appliance/cmd_config_test.go` - Config preview tests
- `cmd/ze/appliance/agent_test.go` - Agent protocol + socket tests
- `cmd/ze/appliance/manifest_test.go` - Build manifest generation tests
- `cmd/ze/appliance/cmd_config_push.go` - Config push to running device via SSH without rebuild
- `cmd/ze/appliance/cmd_config_push_test.go` - Config push tests (mocked SSH)
- `cmd/ze/appliance/cmd_export.go` - Export appliance dir to encrypted archive
- `cmd/ze/appliance/cmd_export_test.go` - Export tests
- `cmd/ze/appliance/cmd_import.go` - Import appliance dir from encrypted archive
- `cmd/ze/appliance/cmd_import_test.go` - Import tests
- `cmd/ze/appliance/parallel.go` - Parallel execution helper for --parallel N (push, config-push)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Config + dir resolution + types** -- ApplianceConfig struct, JSON marshal/unmarshal, defaults, validation, dir resolution (--dir > env > default)
   - Tests: `TestConfigDefaults`, `TestConfigMarshalRoundtrip`, `TestConfigValidation`, `TestDirResolution`
   - Files: `cmd/ze/appliance/config.go`, `cmd/ze/appliance/resolve.go`, `cmd/ze/appliance/config_test.go`, `cmd/ze/appliance/main_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Crypto primitives + passphrase agent** -- Argon2id KDF + ChaCha20-Poly1305 AEAD encrypt/decrypt; secret file read/write helpers that handle both encrypted and plaintext mode; passphrase agent (Unix domain socket, duration-limited, memory zeroing on exit)
   - Tests: `TestEncryptDecryptRoundtrip`, boundary tests for empty passphrase / corrupt ciphertext / wrong passphrase, `TestUnlockStartsAgent`, `TestUnlockDurationExpiry`, `TestUnlockStop`, `TestAgentPassphraseResolution`
   - Files: `cmd/ze/appliance/crypto.go`, `cmd/ze/appliance/agent.go`, `cmd/ze/appliance/cmd_unlock.go`, `cmd/ze/appliance/crypto_test.go`, `cmd/ze/appliance/agent_test.go`, `cmd/ze/appliance/cmd_unlock_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Prereq: vendor `golang.org/x/crypto/argon2` and `golang.org/x/crypto/chacha20poly1305`

3. **Phase: Init wizard** -- Interactive + non-interactive init, passphrase prompt, password hashing, cert generation (10-year validity), secret encryption, --cert/--key for CA certs, --config for batch, managed flag, update token generation, SSH authorized keys
   - Tests: `TestInitCreatesApplianceDir`, `TestInitPasswordNeverInJSON`, `TestInitGeneratesCert`, `TestInitWithCACert`, `TestInitFromConfigFile`, `TestInitManagedFlag`, `TestEncryptedInitCreatesMarker`, `TestPlaintextInitNoMarker`, `TestInitGeneratesUpdateToken`, `TestInitWithAuthorizedKeys`
   - Files: `cmd/ze/appliance/cmd_init.go`, `cmd/ze/appliance/cmd_init_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Prereq: GenerateWebCertWithDuration or equivalent in internal/component/web/server.go

4. **Phase: Assemble (ZeFS + config layering + decrypt)** -- Read config, decrypt secrets via agent or prompt, config_base layering, hostname patch, assemble database.zefs, auto-delete unless --keep, zero secrets from memory
   - Tests: `TestAssembleProducesZeFS`, `TestAssembleReusesExistingCert`, `TestAssembleMissingSecretsFails`, `TestAssembleConfigLayering`, `TestAssembleDefaultZeConf`, `TestAssembleHostnamePatch`, `TestAssembleWrongPassphraseFails`, `TestAssembleDecryptedSecretsNotOnDisk`, `TestAssembleAutoDeleteZeFS`, `TestAssembleKeepRetainsZeFS`, `TestAssembleIncludesAuthorizedKeys`
   - Files: `cmd/ze/appliance/cmd_assemble.go`, `cmd/ze/appliance/cmd_assemble_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Day-2 operations** -- passwd, replace-cert, rekey, clone (all passphrase-aware)
   - Tests: `TestPasswdUpdatesHash`, `TestReplaceCertUpdatesSecrets`, `TestReplaceCertRegenerates`, `TestCloneCopiesConfigNotSecrets`, `TestRekeyChangesEncryption`, `TestRekeyPlaintextToEncrypted`, `TestRekeyEncryptedToPlaintext`
   - Files: `cmd/ze/appliance/cmd_passwd.go`, `cmd/ze/appliance/cmd_cert.go`, `cmd/ze/appliance/cmd_rekey.go`, `cmd/ze/appliance/cmd_clone.go` + tests
   - Verify: tests fail -> implement -> tests pass

6. **Phase: List/Show + dispatch** -- List with hostname/target columns, show with cert expiry, main.go dispatch, register.go
   - Tests: `TestListShowsAppliances`, `TestShowDisplaysConfigAndCertExpiry`, `TestMainDispatchAppliance`
   - Files: `cmd/ze/appliance/cmd_list.go`, `cmd/ze/appliance/cmd_show.go`, `cmd/ze/appliance/main.go`, `cmd/ze/appliance/register.go`, `cmd/ze/main.go`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Image build + run + integrity** -- gok invocation, ext4 inject, database.zefs cleanup, timestamped images, SHA-256 checksums, build manifest (build.json + ZeFS meta/build/manifest), QEMU with per-appliance ports + conflict detection, ARM support, --all flag for batch builds
   - Tests: `TestBuildWritesChecksum`, `TestBuildWritesManifest`, `TestBuildManifestInZeFS`, `TestBuildAllIteratesAppliances`; manual tests for e2fsprogs + gok + QEMU
   - Files: `cmd/ze/appliance/cmd_build.go`, `cmd/ze/appliance/cmd_run.go`, `cmd/ze/appliance/manifest.go`, `Makefile`
   - Verify: `make ze-gokrazy APPLIANCE=default` produces bootable image + checksum + manifest; `ze appliance run` boots in QEMU

8. **Phase: OTA push** -- Push image to device via gokrazy HTTP update API, TLS verification (self-signed cert from secrets/), update token auth, --image for rollback, --all for fleet push
   - Tests: `TestPushSendsImage`, `TestPushUnreachableDevice`, `TestPushWrongToken`, `TestPushSpecificImage`, `TestPushAllIteratesAppliances`
   - Files: `cmd/ze/appliance/cmd_push.go`, `cmd/ze/appliance/cmd_push_test.go`
   - Verify: tests fail -> implement -> tests pass

9. **Phase: Config preview** -- `ze appliance config <name> --merged` shows effective config after base + overlay; no build needed
   - Tests: `TestConfigMergedOutput`, `TestConfigMergedWithDelete`
   - Files: `cmd/ze/appliance/cmd_config.go`, `cmd/ze/appliance/cmd_config_test.go`
   - Verify: tests fail -> implement -> tests pass

10. **Phase: Batch init** -- `--batch <manifest.json>`, per-device password generation (`"password": "generate"`), env var requirements, independent crypto state per appliance
   - Tests: `TestBatchInitCreatesMultiple`, `TestBatchInitMissingEnvFails`, `TestBatchInitPerDevicePasswords`
   - Files: update `cmd/ze/appliance/cmd_init.go`
   - Verify: tests fail -> implement -> tests pass

11. **Phase: Config push** -- `ze appliance config-push <name>` pushes merged ze.conf to running device via SSH; device validates, applies, or auto-reverts; saves previous config for manual recovery; last-known-good hash embedded in ZeFS at build time
    - Tests: `TestConfigPushUploadsConfig`, `TestConfigPushInvalidConfigReverts`, `TestConfigPushUnreachableDevice`, `TestConfigPushDryRun`, `TestConfigPushAllDevices`, `TestConfigPushSavesPrevious`, `TestBuildWritesLastKnownGood`, `TestAssembleWritesLastKnownGood`, `TestLastKnownGoodHashMatchesSeedConfig`
    - Files: `cmd/ze/appliance/cmd_config_push.go`, `cmd/ze/appliance/cmd_config_push_test.go`
    - Verify: tests fail -> implement -> tests pass

12. **Phase: Parallel operations** -- `--parallel N` flag for `push --all` and `config-push --all`; bounded worker pool; per-device status; continues on individual failure
    - Tests: `TestPushAllParallel`, `TestPushAllParallelPartialFailure`, `TestPushAllParallelDefault`, `TestConfigPushAllParallel`
    - Files: `cmd/ze/appliance/parallel.go`, update `cmd/ze/appliance/cmd_push.go`, `cmd/ze/appliance/cmd_config_push.go`
    - Verify: tests fail -> implement -> tests pass

13. **Phase: Export/import** -- `ze appliance export` creates encrypted archive of appliance dir; `ze appliance import` restores from archive; bastion disaster recovery
    - Tests: `TestExportCreatesArchive`, `TestExportAllCreatesArchive`, `TestExportRequiresPassphrase`, `TestImportRestoresAppliance`, `TestImportWrongPassphraseFails`, `TestImportPromptsBeforeOverwrite`, `TestImportToNewDir`, `TestExportImportRoundtrip`
    - Files: `cmd/ze/appliance/cmd_export.go`, `cmd/ze/appliance/cmd_import.go`, `cmd/ze/appliance/cmd_export_test.go`, `cmd/ze/appliance/cmd_import_test.go`
    - Verify: tests fail -> implement -> tests pass

14. **Functional tests** -> Create after feature works. Cover user-visible behavior.
15. **Full verification** -> `make ze-verify-fast` (lint + all ze tests)
16. **Complete spec** -> Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-74 has implementation with file:line |
| Correctness | Password hash stored in secrets/, never in appliance.json; cert reused across builds; managed flag in ZeFS |
| Naming | JSON keys use snake_case (matching existing gokrazy/ze/config.json convention) |
| Data flow | Secrets only flow from secrets/ dir, never from config JSON; config_base resolved relative to appliance dir |
| Rule: no-layering | No duplicate of ze init logic; compose zefs + zeweb directly |
| Rule: derive-not-hardcode | ZeFS keys from zefs.Key* constants, not hardcoded strings |
| Cert validity | Self-signed certs use configurable validity (default 10 years), not the 365-day constant |
| Encryption | Passphrase set -> all sensitive secrets encrypted; passphrase empty -> plaintext; no mixed state |
| Memory hygiene | Decrypted secrets zeroed after use; passphrase zeroed after KDF |
| Config layering | Base + overlay produces correct concatenation; missing base = error if referenced |
| Hostname patch | gokrazy config.json Hostname field patched per-appliance; original file not modified |
| Dir isolation | --dir flag works; env var works; default works; appliance data never written to source tree |
| database.zefs cleanup | `build` auto-deletes database.zefs after image inject; `assemble` auto-deletes unless `--keep` |
| Update token isolation | Update token is separate from admin password; randomly generated at init; stored encrypted |
| Gokrazy update auth | Update token (not admin password) injected into temp config.json; temp deleted after gok; token zeroed |
| Image versioning | Images timestamped; `run` picks most recent; `--image` overrides |
| Batch init | Manifest creates N independent appliances; each has unique salt/nonce; env vars required |
| ARM support | arm64 arch triggers GOARCH=arm64; fails clearly if no ARM kernel package |
| Port conflict detection | `run` checks all ports before QEMU launch; clear error on conflict |
| Env var warnings | Passphrase and password env vars produce stderr warnings |
| Passphrase agent | Agent starts/stops cleanly; socket removed on stop; key zeroed on expiry; commands use agent when available |
| OTA push | Push uses update token (not admin password); TLS verified against stored cert; --image for rollback |
| Image integrity | SHA-256 checksum written alongside image; build manifest in build.json and ZeFS meta/build/manifest |
| Config preview | `config --merged` shows effective config; matches what assemble would produce |
| SSH authorized keys | Keys in config baked into ZeFS; password auth still works alongside |
| Batch build | `build --all` iterates all appliances; fails clearly if any single build fails |
| Managed flag documented | `show` explains what managed mode does at runtime |
| Config push | `config-push` connects via SSH; uploads merged config; device validates before applying |
| Config push revert | Device auto-reverts to previous config on validation failure; previous config saved |
| Config push dry-run | `--dry-run` prints config without SSH connection |
| Last-known-good | Build writes meta/config/last-known-good with SHA-256 of validated seed config |
| Device boot with pushed config | Device loads config-pushed.conf from /perm if present; validates against last-known-good; reverts on failure |
| Parallel push | `push --all --parallel N` runs N concurrent uploads; per-device status; partial failure handling |
| Parallel config-push | `config-push --all --parallel N` runs N concurrent SSH sessions |
| Export single | `export <name>` creates encrypted .ze.enc archive |
| Export all | `export --all` creates single archive with all appliances |
| Export requires passphrase | Archives are always encrypted; no unencrypted export |
| Import restore | `import <archive>` decrypts and restores appliance dir |
| Import to new dir | `--dir` flag allows import to different bastion |
| Export/import roundtrip | Export then import produces byte-identical appliance dir |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/appliance/` package compiles | `go build ./cmd/ze/appliance/` |
| `ze appliance init` produces directory | `ls ~/.config/ze/appliances/test-*/appliance.json` or `--dir` equivalent |
| `ze appliance assemble` produces database.zefs | `bin/ze data ls --path <dir>/test-*/database.zefs` shows all expected keys |
| `ze appliance passwd` updates hash | `diff` before/after secrets/password.hash |
| `ze appliance replace-cert` updates cert | `openssl x509 -in secrets/tls/cert.pem -noout -enddate` shows new expiry |
| `ze appliance clone` produces new config | `diff` original and clone appliance.json shows only identity diff |
| `ze appliance list` outputs names + columns | `bin/ze appliance list --dir <testdir>` |
| `ze appliance show` includes cert expiry | Output contains "expires:" line |
| Tests pass | `go test ./cmd/ze/appliance/...` |
| main.go dispatch wired | `grep -n 'appliance' cmd/ze/main.go` |
| Managed flag in ZeFS | `bin/ze data cat --path <db> meta/instance/managed` |
| Hostname in gokrazy config | Build produces temporary config.json with correct Hostname |
| Update token in gokrazy config | Temp config.json has Update.HTTPPassword from update token (not admin password) |
| Image checksum | `sha256sum -c <name>/ze-*.img.sha256` passes |
| Build manifest | `cat <name>/build.json` shows config_hash, timestamp, ze_version, arch |
| Passphrase agent | `ze appliance unlock` starts agent; subsequent commands skip prompt |
| OTA push | `ze appliance push <name>` pushes to device (manual verification with running gokrazy) |
| Config preview | `ze appliance config <name> --merged` outputs effective config |
| SSH authorized keys | `bin/ze data cat --path <db> meta/ssh/authorized_keys` shows keys |
| Batch build | `ze appliance build --all --dir <testdir>` builds all appliances |
| Config push | `ze appliance config-push <name>` pushes config to running device; device validates and applies |
| Config push dry-run | `ze appliance config-push <name> --dry-run` prints merged config |
| Parallel push | `ze appliance push --all --parallel 4` pushes to 4 devices concurrently |
| Export | `ze appliance export <name>` produces .ze.enc archive |
| Import | `ze appliance import <archive>` restores appliance dir from archive |
| Last-known-good in ZeFS | `bin/ze data cat --path <db> meta/config/last-known-good` shows SHA-256 hash |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| **Encryption at rest** | All files listed as "Encrypted: Yes" in design are encrypted when passphrase set; .encrypted marker correct |
| **KDF parameters** | Argon2id: time=3, memory=64MB, threads=4, keyLen=32. Verify these are not accidentally weakened |
| **AEAD correctness** | ChaCha20-Poly1305 nonce is unique per file (random 12B); salt is unique per file (random 16B) |
| **Memory hygiene** | Decrypted secrets zeroed from []byte slices after use (overwrite with zeros, not just nil) |
| **Passphrase never on disk** | grep entire appliance dir for passphrase; verify it appears in no file, no log, no temp file |
| **Partial write safety** | Encrypt to temp file + atomic rename; never leave half-encrypted secret on crash |
| **Wrong passphrase** | AEAD authentication fails cleanly; no partial decryption output; no oracle (constant-time compare) |
| Input validation | Appliance name: reject path traversal (../, absolute paths, special chars outside [a-zA-Z0-9._-]) |
| Secret handling | SSH password plaintext never written to disk; bcrypt hash encrypted at rest |
| File permissions | secrets/ created 0700; encrypted files 0600; database.zefs 0600 |
| Config file trust | appliance.json is untrusted input; validate all fields before use |
| Subprocess safety | gok path not user-controlled; args properly quoted; no shell injection via identity.hostname |
| Cert import | --cert/--key validated (exist, readable, valid PEM) before copying + encrypting key |
| Config base path | config_base resolved relative to appliance dir; reject absolute paths or traversal |
| Temp file cleanup | Per-appliance config.json deleted after gok invocation |
| **No plaintext leak** | database.zefs contains plaintext secrets (by design, it's the runtime artifact); verify it is auto-deleted after image inject in `build`; verify `assemble` auto-deletes unless `--keep` |
| **Update token isolation** | Gokrazy update uses separate random token, not admin password; token in temp config.json; temp deleted after gok; token zeroed |
| **Agent socket security** | Socket created 0600 in `$XDG_RUNTIME_DIR` or `/tmp`; only owner can connect; key material zeroed on agent exit/expiry |
| **Image integrity** | SHA-256 checksum matches image; build manifest not tamperable (informational, not signed) |
| **OTA push TLS** | Push verifies device TLS cert against stored cert.pem; rejects unknown certs; update token sent via HTTP basic auth |
| **Env var warning** | `ZE_APPLIANCE_PASSPHRASE` and `ZE_APPLIANCE_SSH_PASSWORD` env vars print warning on stderr when detected |
| **admin_enabled flag** | When false, verify ze auth layer rejects SSH/web login for the superuser; verify serial console still works |
| **Batch init isolation** | Each appliance in a batch gets independent salt/nonce for encryption; no shared crypto state between appliances |
| **Batch init per-device passwords** | `"password": "generate"` produces unique random password per device; passwords printed once, never stored in plaintext |
| **SSH authorized keys** | Keys validated as proper SSH public key format before writing to ZeFS |
| **Config push SSH auth** | Config-push uses SSH public key auth (operator's key in authorized_keys); no password transmitted over SSH |
| **Config push no secret transfer** | Config-push transmits ze.conf only (routing config); no passwords, no TLS keys, no tokens cross the SSH channel |
| **Config push revert safety** | Device saves previous config before applying new; auto-reverts on validation failure; no config loss |
| **Config push validation** | Device validates pushed config (parse + semantic check) before applying; invalid config never activates |
| **Parallel push isolation** | Each goroutine in parallel push has independent TLS connection, independent update token decryption; no shared mutable state |
| **Export archive encryption** | Archives always encrypted (no `--no-encrypt`); uses same AEAD as secrets; archive passphrase can differ from secrets passphrase |
| **Export archive integrity** | Archive includes HMAC of contents; import verifies before extracting |
| **Import overwrite protection** | Import prompts before overwriting existing appliance dir; `--force` skips prompt |
| **Last-known-good integrity** | Hash in meta/config/last-known-good is SHA-256 of the validated seed config; device trusts this as the fallback |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design

### Threat Model

**Deployment:** a bastion server on a dedicated management network runs `ze appliance` (build-time) and `ze fleet` (runtime) commands. The bastion stores appliance configurations and encrypted secrets. `ze appliance` handles definition, compilation, and image creation. `ze fleet` (separate spec) handles OTA updates, group operations, and device monitoring.

**Threat:** the bastion is compromised. The attacker copies the entire `<appliance-dir>/` tree.

**Goal:** the copied data is useless without the passphrase. The attacker cannot:
- Extract SSH passwords (even the bcrypt hashes are encrypted)
- Extract TLS private keys
- Build a valid image (assemble requires decrypted secrets)

**What the attacker CAN see** (unencrypted, by design):
- appliance.json: hostnames, IP addresses, port numbers, architecture (network topology, not credentials)
- ze.conf: BGP config, interface config (routing policy, not access)
- TLS cert.pem (public certificate, not the private key)

**Non-goals:**
- Protecting against an attacker with the passphrase (that is access control, not encryption)
- Protecting the running device at runtime (out of scope; the device has decrypted secrets in memory)
- Full key escrow or split-key recovery (see Disaster Recovery section for passphrase loss procedures)

**`ZE_APPLIANCE_PASSPHRASE` env var risk:** this env var exists for development and CI convenience only. In production, always use the interactive prompt. The env var is visible to any process with the same UID via `/proc/<pid>/environ` on Linux. CI pipelines should use short-lived secrets injection (e.g., Vault, CI secrets) and clear the variable immediately after use. The `ze appliance` commands print a warning when the env var is detected: "WARNING: passphrase from environment variable (not recommended for production)".

### Secrets Encryption

Sensitive files in `secrets/` are encrypted at rest using **Argon2id + ChaCha20-Poly1305** (both from `golang.org/x/crypto`, already a dependency).

**Encrypted files:**
- `secrets/password.hash` (bcrypt hash of SSH password)
- `secrets/tls/key.pem` (TLS private key)

**Not encrypted** (public or non-sensitive):
- `appliance.json` (network config, no credentials)
- `ze.conf` (routing config)
- `secrets/tls/cert.pem` (public certificate)

**Encryption envelope:**

```
[16B salt][12B nonce][encrypted payload][16B Poly1305 tag]
```

- KDF: Argon2id(passphrase, salt, time=3, memory=64MB, threads=4) -> 32-byte key
- AEAD: ChaCha20-Poly1305(key, nonce, plaintext) -> ciphertext + tag
- Salt: random per-file, stored as prefix
- Nonce: random per-file, stored after salt

**Passphrase lifecycle:**
- Set during `ze appliance init` (prompted: "Encryption passphrase (empty for none):")
- Provided at build/assemble time via (in priority order):
  1. Passphrase agent (`ze appliance unlock`, recommended for production fleet ops)
  2. Interactive prompt (production, single-device operations)
  3. `ZE_APPLIANCE_PASSPHRASE` env var (CI only, prints warning, see Threat Model for risks)
- Never stored on disk, never written to any file, never logged
- If empty, secrets are stored in plaintext (dev/test convenience)
- A marker file `secrets/.encrypted` indicates secrets are encrypted (vs plaintext)
- Commands print a warning when passphrase is sourced from the env var

**Passphrase change:** `ze appliance rekey <name>` decrypts all secrets with old passphrase, re-encrypts with new one.

### Appliance Directory Location

Appliance definitions are **user data, not source code**. They live on the bastion, not in the Ze source tree.

Resolution order:
1. `--dir <path>` flag (explicit, per-invocation)
2. `ZE_APPLIANCE_DIR` environment variable
3. `~/.config/ze/appliances/` (XDG default)

The Ze repo has `appliances/` gitignored for development convenience only.

### Filesystem Layout

```
<appliance-dir>/                     # ~/.config/ze/appliances/ on the bastion
  _shared/                           # optional shared base configs
    ze.conf                          # base seed config for all appliances at a site/role
  <name>/
    appliance.json                   # config (no credentials, safe to version-control)
    ze.conf                          # per-device config overrides (optional, appended to base)
    secrets/                         # 0700 permissions
      .encrypted                     # marker: secrets are encrypted (absent = plaintext mode)
      tls/
        cert.pem                     # public cert (NOT encrypted)
        key.pem                      # private key (encrypted if passphrase set)
      password.hash                  # bcrypt hash (encrypted if passphrase set)
      update.token                   # gokrazy OTA update token (encrypted if passphrase set)
      authorized_keys                # SSH public keys (optional, NOT encrypted)
    database.zefs                    # derived artifact (auto-deleted after build or assemble)
    ze-20260427-143022.img           # derived artifact (timestamped, rebuilt by gok)
    ze-20260427-143022.img.sha256    # SHA-256 checksum of image
    build.json                       # build manifest (config hash, timestamp, version, arch)
```

**Derived artifacts:** database.zefs is auto-deleted after both `build` and `assemble` (contains plaintext secrets); use `assemble --keep` to retain for debugging. Images are timestamped (`ze-<YYYYMMDD>-<HHMMSS>.img`) with a matching `.sha256` checksum and `build.json` manifest. Previous images are kept for rollback via `ze appliance push <name> --image <old.img>`. The operator manages old images manually (or via a future `ze appliance prune` command). Both can be regenerated from config + secrets by `ze appliance assemble` / `ze appliance build`.

### Config File (appliance.json)

```json
{
  "identity": {
    "name": "edge-01",
    "hostname": "edge-01"
  },
  "credentials": {
    "username": "admin",
    "admin_enabled": true,
    "ssh_authorized_keys": [
      "ssh-ed25519 AAAA... operator@bastion"
    ]
  },
  "ssh": {
    "host": "0.0.0.0",
    "port": "22"
  },
  "web": {
    "enabled": true,
    "host": "0.0.0.0",
    "port": "8080"
  },
  "tls": {
    "cert_name": "edge-01.example.com",
    "cert_file": "",
    "key_file": "",
    "validity_years": 10
  },
  "managed": false,
  "device": {
    "address": "10.0.100.1",
    "update_port": 443
  },
  "config_base": "../_shared/ze.conf",
  "image": {
    "arch": "amd64",
    "size_bytes": 2147483648
  },
  "qemu": {
    "ssh_port": 2222,
    "web_port": 28080,
    "gokrazy_port": 18080
  }
}
```

**Field semantics:**

| Field | Purpose | Default | Encrypted |
|-------|---------|---------|-----------|
| `identity.name` | Instance name -> `meta/instance/name` | dir name | No |
| `identity.hostname` | Gokrazy device hostname (patched into config.json) | `"ze"` | No |
| `credentials.username` | SSH username | `"admin"` | No |
| `credentials.admin_enabled` | Enable superuser SSH/web login (false = RADIUS-only, serial console recovery only) | `true` | No |
| `credentials.ssh_authorized_keys` | SSH public keys for key-based authentication (optional, in addition to password) | `[]` | No |
| `ssh.host`, `ssh.port` | SSH listen address | `"0.0.0.0"`, `"22"` | No |
| `web.enabled` | Enable web UI in seed config | `true` | No |
| `web.host`, `web.port` | Web listen address | `"0.0.0.0"`, `"8080"` | No |
| `tls.cert_name` | DNS SAN for self-signed cert generation | `""` (IP-only) | No |
| `tls.cert_file`, `tls.key_file` | External cert paths (source for import) | `""` | No |
| `tls.validity_years` | Self-signed cert lifetime in years | `10` | No |
| `managed` | Fleet mode: device accepts remote config push from hub, reports status to hub (see Managed Mode section) | `false` | No |
| `device.address` | Device management IP (for OTA push; on management network) | `""` | No |
| `device.update_port` | Gokrazy HTTPS update port on the device | `443` | No |
| `config_base` | Path to shared base ze.conf (relative to appliance dir) | `""` (use gokrazy/ze/ze.conf) | No |
| `image.arch` | Target CPU architecture | `"amd64"` | No |
| `image.size_bytes` | Disk image size | `2147483648` (2G) | No |
| `qemu.*` | QEMU port forwarding | `2222`, `28080`, `18080` | No |

**Credentials that live in `secrets/` (encrypted at rest):**

| File | Content | Encrypted |
|------|---------|-----------|
| `secrets/password.hash` | bcrypt hash of SSH password | Yes |
| `secrets/tls/key.pem` | TLS private key | Yes |
| `secrets/tls/cert.pem` | TLS public certificate | No |
| `secrets/update.token` | Gokrazy OTA update token (random 32 bytes, base64-encoded) | Yes |
| `secrets/authorized_keys` | SSH public keys (copied from config at init) | No |

The SSH password plaintext is never stored anywhere. It is prompted, hashed with bcrypt, the hash is encrypted with the passphrase, and the plaintext is zeroed from memory.

The gokrazy update token is a separate random credential generated at `ze appliance init`. It is not the admin password. This limits blast radius: if the admin password leaks, the attacker can log in to the device but cannot push arbitrary firmware. If the update token leaks, the attacker can push firmware but cannot log in interactively. Both are encrypted at rest. SSH transport keys for fleet operations belong in `ze fleet`, not here.

### ze fleet Dependencies (out of scope, documented for completeness)

The following operational workflows belong in a separate `ze fleet` spec but are tightly coupled to the appliance model. `ze appliance push` and `ze appliance config-push` handle single-device and batch operations; `ze fleet` adds orchestration, coordination, and monitoring on top.

| Workflow | Description |
|----------|-------------|
| ~~Config push~~ | ~~Modify local ze.conf, push to running device without full image rebuild~~ **Moved to this spec** as `ze appliance config-push` |
| Config pull | Pull running device's config back to the appliance dir |
| Mandatory resync | Before any config change, either push or pull to ensure local and remote are in sync; no silent overwrite |
| Config drift detection | Compare appliance dir config vs running device config; report differences |
| Group operations | Apply changes to multiple devices by tag/group (beyond `--all`) |
| Cert expiry monitoring | Fleet-wide view of cert expiry dates |
| Rollout coordination | Staged rollout with health checks between groups |

### Config Layering (config_base)

When `config_base` is set, the build reads the base file first, then appends the per-appliance `ze.conf` (if it exists). Ze's config parser applies commands sequentially: later `set` commands override earlier ones, and `delete` commands remove settings from the base. This allows 20 routers to share a common base with per-device overrides:

```
_shared/ze.conf:      set environment log level info
                      set environment web enabled true
                      set environment ssh enabled true

edge-01/ze.conf:      set bgp local-as 65001
                      set bgp router-id 10.0.0.1
                      delete environment ssh
```

Result written to ZeFS `file/template/ze.conf`: base + overlay concatenated. The overlay can override any base setting via `set` or remove one via `delete`.

**Build-time validation:** after concatenation, the merged config is parsed and validated. If the merged config has syntax errors or contradictions, the build fails with a clear error pointing to the offending line and which file (base or overlay) it came from.

If `config_base` is empty and no per-appliance `ze.conf` exists, falls back to `gokrazy/ze/ze.conf` from the Ze source tree. If `config_base` is set but the file does not exist, build fails with a clear error.

### Per-Appliance Gokrazy Config

Each build generates a temporary copy of `gokrazy/ze/config.json` with two fields patched:
- `"Hostname"` from `identity.hostname` (device identity in gokrazy web UI, logs, mDNS)
- `"Update.HTTPPassword"` from the decrypted update token (per-device random token, not the admin password)

The update token is generated at `ze appliance init` (32 random bytes, base64-encoded) and stored encrypted in `secrets/update.token`. At build time, it is decrypted, injected into the temp config.json, and zeroed from memory after gok completes.

This is intentionally a different credential from the admin password. The admin password controls interactive SSH/web access. The update token controls firmware updates. Separating them limits blast radius: a leaked admin password does not grant firmware push capability, and a leaked update token does not grant interactive access.

The original `gokrazy/ze/config.json` is a read-only template. The temporary copy is used for gok invocation and deleted after.

### Passphrase Agent (ze appliance unlock)

Typing the passphrase once per command is tolerable for single-device operations. For fleet operations (build 30 devices, push 30 devices), it becomes a blocking UX problem that pushes operators toward the insecure env var path.

**Solution:** `ze appliance unlock` starts a background agent process that:

1. Prompts for the passphrase once
2. Derives the encryption key via Argon2id (same parameters as direct decryption)
3. Holds the derived key (not the passphrase) in memory
4. Listens on a Unix domain socket at `$XDG_RUNTIME_DIR/ze-appliance-agent.sock` (fallback: `/tmp/ze-appliance-agent-$UID.sock`)
5. Responds to key requests from `ze appliance` commands running under the same UID
6. Auto-exits after the configured duration (default 30 minutes; `--duration` flag)
7. Zeroes key material from memory on exit

**Security properties:**
- Socket created with 0600 permissions; only the owner UID can connect
- The passphrase itself is never stored; only the derived 32-byte key is held
- The key is zeroed from memory on agent exit, expiry, or `ze appliance unlock --stop`
- The agent process has no network access; it only listens on a local Unix socket
- If the agent is killed (SIGKILL), the OS reclaims the memory; no persistent state

**Protocol:** the agent serves a simple request/response over the socket. Commands send a "decrypt" request with the encrypted file content; the agent decrypts using its held key and returns the plaintext. The plaintext is zeroed from the agent's buffer after each response. This avoids sending the raw key over the socket.

**Interaction with `--all` flags:** `ze appliance build --all` and `ze appliance push --all` require an active agent. They refuse to run with interactive prompts because prompting N times defeats the purpose of batch operations.

### OTA Push (ze appliance push)

Gokrazy devices expose an HTTPS update API that accepts a full system image. `ze appliance push` wraps this to close the gap between "image built" and "device updated" without waiting for the full `ze fleet` spec.

**Flow:**
1. Resolve appliance directory and read `appliance.json`
2. Obtain device address from `device.address` and port from `device.update_port`
3. Obtain passphrase (agent > prompt > env var)
4. Decrypt update token from `secrets/update.token`
5. Select image: most recent `ze-*.img` in appliance dir, or `--image` flag for explicit selection (rollback)
6. Verify image checksum against `.img.sha256` (if present)
7. Load device TLS certificate from `secrets/tls/cert.pem` for TLS verification (trust the cert we generated)
8. HTTP PUT to `https://<address>:<port>/update` with HTTP basic auth (user: empty, password: update token)
9. Stream image; print progress
10. On success, device reboots to new partition (gokrazy A/B scheme)
11. Zero update token from memory
12. Print: "Pushed <image> to <hostname> (<address>)"

**Rollback:** gokrazy uses A/B partitions. After a push, the device boots from the newly written partition. If the new image fails to boot, gokrazy falls back to the previous partition automatically. To explicitly rollback, push an older image: `ze appliance push lab --image ze-20260426-120000.img`.

**`--all` flag:** iterates all appliances with a `device.address` set. Requires passphrase agent. Prints per-device status. Continues on failure (reports failed devices at end). Default is sequential; use `--parallel N` (see Parallel Operations section) for concurrent uploads. For coordinated staged rollouts with health checks between stages, use `ze fleet`.

**Error handling:**
- Device unreachable: "error: device <hostname> unreachable at <address>:<port>"
- Auth rejected (wrong token): "error: device rejected update (401 Unauthorized); regenerate update token with `ze appliance init`"
- Checksum mismatch: "error: image checksum mismatch; rebuild with `ze appliance build`"

### Image Integrity

Every `ze appliance build` produces three artifacts:

1. **`ze-<timestamp>.img`** -- the disk image
2. **`ze-<timestamp>.img.sha256`** -- SHA-256 checksum: `<hash>  ze-<timestamp>.img`
3. **`build.json`** -- build manifest (overwritten each build; previous versions in git if the operator versions the appliance dir):

```json
{
  "appliance": "edge-01",
  "timestamp": "2026-04-27T14:30:22Z",
  "ze_version": "0.8.3",
  "arch": "amd64",
  "config_hash": "sha256:abc123...",
  "image": "ze-20260427-143022.img",
  "image_sha256": "def456..."
}
```

Additionally, `meta/build/manifest` is written into ZeFS (same JSON content). This allows a running device to report its build metadata (useful for fleet inventory and drift detection).

`ze appliance push` verifies the checksum before pushing. The manifest is informational, not cryptographically signed. For environments requiring tamper detection, the operator should GPG-sign the checksum file externally.

### Config Preview (ze appliance config)

`ze appliance config <name> --merged` resolves the config layering (base + overlay) and prints the effective config to stdout, without building anything. This allows the operator to verify what the device will receive before committing to a build.

**Flow:**
1. Read `appliance.json` and resolve `config_base`
2. Read base ze.conf (if set) and per-appliance ze.conf
3. Concatenate and parse (same logic as assemble)
4. Print the effective config to stdout
5. If validation fails, print errors identifying which file and line

**No passphrase required.** Config files are not encrypted. This command is safe to run at any time.

### Config Push (ze appliance config-push)

The gap between "edit ze.conf" and "device running new config" should not require a full image rebuild. For config-only changes (BGP peers, firewall rules, interface settings), `ze appliance config-push` pushes the merged ze.conf to the running device via SSH and asks the device to validate and apply it.

**This is not a replacement for `ze appliance build`.** Structural changes (new binary, new cert, new password, new kernel, new packages) still require a full rebuild. Config-push only updates the ze.conf on the running device's persistent partition.

**Flow:**
1. Read `appliance.json` and resolve `config_base`
2. Merge base + per-appliance ze.conf (same logic as assemble/config --merged)
3. Validate merged config locally (syntax + semantic check)
4. SSH to device at `device.address` using the operator's SSH key (from `credentials.ssh_authorized_keys`)
5. Upload merged config to `/perm/ze/config-staged.conf` (temporary staging location)
6. Execute remote validation: `ze config validate /perm/ze/config-staged.conf`
7. If validation passes: save current config as `/perm/ze/config-previous.conf`, move staged to `/perm/ze/config-pushed.conf`, reload config
8. If validation fails: delete staged config, report error to bastion with detail from device
9. Print: "config applied to <hostname>" or "error: device rejected config (validation failed: <detail>)"

**Device-side config loading priority:**
1. `/perm/ze/config-pushed.conf` (if present and valid)
2. ZeFS `file/template/ze.conf` (seed config from last build, the last-known-good baseline)

**Automatic revert:** if the device detects a pushed config causes a runtime failure (health check timeout, BGP session flap within 30 seconds of apply, interface down), it reverts to the previous config (`config-previous.conf`) and logs the revert reason. If `config-previous.conf` is also problematic, it falls back to the ZeFS seed config (the last-known-good baseline from the last build).

**`--dry-run` flag:** prints the merged config that would be pushed, without connecting to the device. Equivalent to `ze appliance config --merged` but framed as a pre-push check.

**`--all` flag:** iterates all appliances with a `device.address` set. Requires passphrase agent (for SSH key decryption if encrypted). Supports `--parallel N` for concurrent SSH sessions. Prints per-device status. Continues on failure (reports failed devices at end).

**No passphrase required** for config-push itself (ze.conf is not encrypted). SSH authentication uses the operator's public key (already baked into the device). If the operator's SSH key is passphrase-protected, the operator's SSH agent handles that (not Ze's passphrase agent).

**Relationship to `ze fleet`:** config-push is a point-to-point operation (bastion to one device, or bastion to N devices in parallel). `ze fleet` adds orchestration on top: staged rollout (push to 10%, wait, push to 50%, wait, push to 100%), health checks between stages, group targeting by tags, and mandatory resync to prevent silent config drift. Config-push is the building block; fleet is the coordinator.

### Last-Known-Good Config

Every `ze appliance build` and `ze appliance assemble` writes a SHA-256 hash of the validated seed config into ZeFS at `meta/config/last-known-good`. This hash represents the config state that was validated at build time and is known to be correct.

**Purpose:** the last-known-good hash establishes a revert target. If a config-push delivers a config that causes runtime problems, the device has an authoritative answer to "what config was I built with?" and can revert to it.

**Device boot behavior:**
1. Read ZeFS seed config (`file/template/ze.conf`) and compute its SHA-256
2. Verify hash matches `meta/config/last-known-good` (integrity check)
3. If `/perm/ze/config-pushed.conf` exists, validate it
4. If pushed config is valid, use it; otherwise log warning, delete it, use seed config
5. Write effective config hash to `/perm/ze/config-active-hash` (for fleet drift detection)

**After a successful config-push:** the device updates `/perm/ze/last-known-good-pushed` with the SHA-256 of the newly applied pushed config. This creates a two-tier revert chain:
- Tier 1: revert to `config-previous.conf` (the config before the most recent push)
- Tier 2: revert to ZeFS seed config (the config from the last full build)

**The ZeFS seed config is immutable.** It cannot be modified by config-push. It is the ultimate fallback. This is the key advantage of the immutable infrastructure model: no matter how many config-pushes go wrong, the device can always return to a known-good state by ignoring `/perm/ze/config-pushed.conf`.

### Parallel Operations

`push --all` and `config-push --all` support a `--parallel N` flag that runs N concurrent operations. The default is sequential (N=1) for backwards compatibility and to avoid overwhelming narrow management links.

**Implementation:** a bounded worker pool of N goroutines. Each goroutine handles one device independently: decrypts update token (push) or reads config (config-push), connects, transfers, reports status. No shared mutable state between goroutines.

**Concurrency limits:**
- `--parallel 1` (default): sequential, identical to current behavior
- `--parallel N` (N > 1): N concurrent connections
- Maximum: clamped to the number of devices (no point in 64 goroutines for 8 devices)
- Boundary: 1-64 (64 is generous; real bottleneck is management network bandwidth)

**Error handling:** continues on individual device failure. Collects results from all devices. Prints per-device status as each completes (not buffered until end). At the end, prints summary: "N succeeded, M failed" with the list of failed devices and their error messages.

**Passphrase agent required** for `--all` operations (both sequential and parallel). Interactive prompts are refused for batch operations.

**Bandwidth consideration:** for OTA push over WAN links, `--parallel` should be tuned to the available bandwidth. Pushing 4 x 2GB images over a 100 Mbps link is 64 GB of concurrent transfer. The operator is responsible for choosing an appropriate N. Config-push is lightweight (a few KB), so high parallelism is fine.

### Export / Import (Bastion Disaster Recovery)

The bastion is a single point of failure for fleet management. If the bastion disk fails, is compromised, or is destroyed, all appliance configs and encrypted secrets are lost. Running devices continue to work, but the operator cannot rebuild, push updates, or rotate credentials.

`ze appliance export` creates an encrypted archive of one or all appliance directories. `ze appliance import` restores from an archive on a fresh bastion.

**Export flow:**
1. Resolve appliance directory
2. Obtain passphrase for the archive (agent > prompt > env var; archive always encrypted, even if appliance secrets are not)
3. For single appliance: tar the `<name>/` directory (appliance.json + secrets/ + ze.conf + build.json; excludes images and database.zefs)
4. For `--all`: tar all appliance directories (including `_shared/` if present)
5. Encrypt the tar with Argon2id + ChaCha20-Poly1305 (same envelope as secrets, but the passphrase can be different from the appliance secrets passphrase)
6. Write `<name>.ze.enc` (single) or `appliances-<timestamp>.ze.enc` (all)
7. Print: "exported to <filename> (encrypted, <size>)"

**Import flow:**
1. Obtain passphrase for the archive
2. Decrypt and verify AEAD tag (wrong passphrase = clear error, no partial extraction)
3. Validate archive structure (must contain appliance.json per directory)
4. If target appliance dir exists, prompt: "Overwrite <name>? [y/N]" (or `--force` to skip)
5. Extract to target directory (--dir flag for bastion migration)
6. Print: "imported <N> appliance(s) to <dir>"

**What is included in the archive:**
- `appliance.json` (config)
- `secrets/` directory (encrypted secrets, including .encrypted marker)
- `ze.conf` (per-appliance config overrides)
- `build.json` (last build manifest, informational)

**What is excluded:**
- `*.img` files (large, can be rebuilt)
- `database.zefs` (derived artifact, can be reassembled)
- `*.img.sha256` files (tied to excluded images)

**Archive format:** `[16B salt][12B nonce][encrypted tar][16B Poly1305 tag]`. Same envelope as individual secret files, but the payload is a tar archive. The archive is a single file for easy transfer to offline storage, USB drives, or remote backup systems.

**Operational recommendation:** run `ze appliance export --all` after any fleet change (init, passwd, replace-cert, rekey) and store the archive on separate media. Automate this with a cron job. The archive is the bastion's backup; the passphrase is the recovery key.

### SSH Authorized Keys

In addition to password authentication, ze's SSH server supports public key authentication. For a fleet managed from a bastion, key-based auth is more practical: the operator uses their SSH key instead of typing a password into each device.

**Config:**
```json
"credentials": {
  "username": "admin",
  "ssh_authorized_keys": [
    "ssh-ed25519 AAAA... operator@bastion",
    "ssh-ed25519 AAAA... backup@bastion"
  ]
}
```

**Storage:** authorized keys are written to `secrets/authorized_keys` (not encrypted, public keys are not sensitive) and baked into ZeFS as `meta/ssh/authorized_keys`.

**Password + keys coexist.** An appliance can have both password auth and key auth enabled. The password is still required for: web UI login, serial console recovery, and as the fallback when key auth fails.

### Managed Mode

The `managed` flag (`meta/instance/managed` in ZeFS) controls whether the device operates as a standalone router or as part of a centrally managed fleet:

| Behavior | `managed: false` (default) | `managed: true` |
|----------|---------------------------|-----------------|
| Config source | Local only (seed config in ZeFS) | Accepts remote config push from hub |
| Hub reporting | None | Reports status, health, config hash to hub |
| OTA updates | Manual via `ze appliance push` | Accepts push from `ze fleet` |
| Config editing | Full local CLI access | Local edits allowed but flagged as drift |

`ze appliance show` includes this explanation when displaying managed status, so the operator understands what they are enabling.

### Disaster Recovery

**Passphrase loss** is the highest-severity failure mode. Without the passphrase, encrypted secrets cannot be decrypted, which means no builds, no pushes, and no credential recovery.

**Impact assessment:**
- All appliances sharing the lost passphrase are affected
- Running devices continue to work (they have decrypted secrets in ZeFS at runtime)
- No new builds, OTA updates, or password rotations are possible
- Recovery requires physical access to every affected device

**Recovery procedure:**

1. **Devices are still running and accessible via SSH/web:** pull the running config from each device (via `ze fleet pull` or manual export). This recovers the ze.conf but not the SSH password hash or TLS private key.

2. **For each affected appliance:**
   - `ze appliance init <name> --force` re-initializes with a new passphrase, new password, new TLS cert, new update token
   - Rebuild and push the new image (requires the device to still be reachable via the old update token, which is gone)
   - If the device is unreachable: physical access required (SD card reflash or serial console)

3. **Prevention:**
   - Use different passphrases for different security zones (e.g., separate passphrase for edge routers vs. core routers)
   - Store the passphrase in a hardware security module, password manager, or sealed envelope in a safe
   - Consider `ze appliance rekey` to rotate passphrases periodically (proves you still have the current one)
   - **Use `ze appliance export --all` regularly** to create encrypted backups of all appliance directories. Store the archive on a separate system (or offline media). If the bastion is lost, `ze appliance import` on a fresh machine restores full fleet management capability. The archive is encrypted with its own passphrase (can be the same or different from the secrets passphrase).

**Bastion loss** (disk failure, compromise, fire) is the second-highest severity failure mode. Without the appliance directories, the operator cannot rebuild images, push updates, or rotate credentials. Running devices continue to work but become unmanageable.

**Recovery from bastion loss:**
1. Provision a new bastion
2. `ze appliance import <archive> --dir /path/to/appliances` restores from the most recent export
3. Verify with `ze appliance list` and `ze appliance show <name>` for each appliance
4. Resume normal operations (build, push, config-push)

**This spec does not implement split-key or threshold recovery** for the secrets passphrase. The passphrase is a single secret. If it is lost and the devices are unreachable, physical access is required. This tradeoff is intentional: split-key schemes add complexity and operational burden that is disproportionate for the typical deployment size (tens of devices, not thousands).

### CLI Commands

| Command | Purpose |
|---------|---------|
| `ze appliance init <name>` | Interactive wizard: creates dir, generates + encrypts secrets (including update token), writes config |
| `ze appliance init --batch <manifest.json>` | Batch init: creates multiple appliances from a manifest (supports per-device password generation) |
| `ze appliance unlock [--duration 30m]` | Start passphrase agent: derives key, holds in memory via Unix socket, auto-expires |
| `ze appliance unlock --stop` | Stop running passphrase agent |
| `ze appliance build <name>` | Full image: assemble ZeFS + gok + ext4 inject + checksum + manifest |
| `ze appliance build --all` | Build all appliances in the directory sequentially |
| `ze appliance assemble <name>` | Fast path: ZeFS only; auto-deletes database.zefs (use `--keep` to retain) |
| `ze appliance push <name>` | OTA push: send most recent image to device via gokrazy HTTP update API |
| `ze appliance push <name> --image <file>` | Push specific image (rollback to previous build) |
| `ze appliance push --all [--parallel N]` | Push most recent image to all devices (default sequential; --parallel for concurrent uploads) |
| `ze appliance config <name> --merged` | Preview effective config after base + overlay layering (no build) |
| `ze appliance config-push <name>` | Push merged ze.conf to running device via SSH; device validates + applies or auto-reverts |
| `ze appliance config-push <name> --dry-run` | Print merged config that would be pushed; no SSH connection |
| `ze appliance config-push --all [--parallel N]` | Push config to all devices with device.address set |
| `ze appliance export <name>` | Export appliance dir to encrypted archive (.ze.enc) |
| `ze appliance export --all` | Export all appliance dirs to single encrypted archive |
| `ze appliance import <archive>` | Restore appliance dir from encrypted archive; prompts before overwrite |
| `ze appliance passwd <name>` | Change SSH password (requires passphrase, re-encrypts hash) |
| `ze appliance replace-cert <name>` | Replace TLS cert (--cert/--key or regenerate; requires passphrase) |
| `ze appliance rekey <name>` | Change encryption passphrase (decrypt all, re-encrypt with new) |
| `ze appliance clone <src> <dst>` | Copy config (not secrets); new appliance needs its own init |
| `ze appliance list` | List appliances with hostname, arch columns |
| `ze appliance show <name>` | Show config summary + cert expiry + managed mode explanation (no passphrase needed) |
| `ze appliance run <name>` | Boot in QEMU with per-appliance port forwarding; detects port conflicts before launch |

Device groups, config drift detection, mandatory resync, and staged rollout coordination belong in a separate `ze fleet` spec. Config push to individual devices is handled here; fleet-level orchestration (health checks between groups, staged rollout percentages) is `ze fleet` scope.

All commands accept `--dir <path>` to override the appliance directory.

**Passphrase handling:** commands that need secrets resolve the passphrase in priority order: (1) passphrase agent socket (if `ze appliance unlock` is active), (2) interactive prompt, (3) `ZE_APPLIANCE_PASSPHRASE` env var (CI only, prints warning). If `secrets/.encrypted` does not exist, no passphrase is required. For fleet operations, `ze appliance unlock` is the recommended workflow: unlock once, then build/push multiple devices without repeated prompts.

**Batch init manifest format:**
```json
[
  {"name": "edge-01", "hostname": "edge-01", "address": "10.0.100.1", "config_base": "../_shared/ze.conf"},
  {"name": "edge-02", "hostname": "edge-02", "address": "10.0.100.2", "config_base": "../_shared/ze.conf"}
]
```
Each entry creates an appliance directory. Fields not specified use defaults. Passphrase comes from env var or agent.

**Password modes:**
- `ZE_APPLIANCE_SSH_PASSWORD` env var: all appliances share the same password (simple fleet scenario)
- `"password": "generate"` in manifest: each appliance gets a unique random 24-character password; passwords are printed to stdout in a sealed table (appliance name + password) for the operator to record securely. The table is printed once and never stored.

All appliances in a batch share the same encryption passphrase (fleet provisioning scenario). Each gets an independent update token and independent salt/nonce for encryption.

### Wizard Flow (ze appliance init)

1. "Encryption passphrase (empty for no encryption):" (hidden input)
2. "SSH username [admin]:"
3. "SSH password:" (hidden input)
4. "SSH authorized keys file (empty to skip):" (path to authorized_keys file, optional)
5. "SSH listen address [0.0.0.0:22]:"
6. "Enable web UI [Y/n]:"
7. "Web listen port [8080]:"
8. "TLS certificate hostname (e.g. router.local):" (or `--cert`/`--key` for CA-signed)
9. "Device management address (for OTA push):" (IP on management network)
10. "Hostname [<name>]:" (gokrazy device hostname)
11. "Architecture (amd64/arm64) [amd64]:"
12. "Fleet managed [y/N]:"
13. "Enable admin login [Y/n]:" (default yes; no = RADIUS-only, serial console recovery only)

After prompts: hash SSH password with bcrypt, generate TLS cert (10-year validity), generate random update token (32 bytes), encrypt sensitive secrets with passphrase (if set), write appliance.json + secrets.

Non-interactive: `ze appliance init <name> --config <file.json>` reads pre-filled JSON. Password + passphrase provided via env vars `ZE_APPLIANCE_PASSPHRASE` and `ZE_APPLIANCE_SSH_PASSWORD`.

### Build Flow (ze appliance build)

1. Resolve appliance directory (--dir > env > default)
2. Read `<name>/appliance.json`
3. Obtain passphrase (agent > prompt > env var)
4. Run assemble step (below, with `--keep` implied so database.zefs persists for image inject)
5. Decrypt update token from `secrets/update.token`
6. Check bin/gok exists; fail with "run: make bin/gok" if missing
7. Generate per-appliance config.json (patch Hostname + Update.HTTPPassword from update token)
8. Invoke: `GOARCH=<arch> bin/gok --parent_dir gokrazy -i ze overwrite --full <img> --target_storage_bytes <size>`
9. Format /perm ext4: mkfs.ext4, dd extract, debugfs mkdir+write, dd inject
10. Clean up temp files (config.json copy, perm.img)
11. Delete `<name>/database.zefs` (derived artifact containing plaintext secrets; must not persist on bastion)
12. Write `<name>/ze-<timestamp>.img.sha256` (SHA-256 checksum of image)
13. Write `<name>/build.json` (manifest: config_hash, timestamp, ze_version, arch, image_sha256)
14. Write `meta/build/manifest` into ZeFS (same metadata, available at runtime for device identification)
15. Zero update token from memory
16. Print: "Image ready: <name>/ze-<timestamp>.img (sha256: <hash>)"

**`--all` flag:** iterates all appliance directories. Requires passphrase agent (refuses to run with interactive prompts to avoid N password entries). Prints per-appliance status. Continues on failure (reports failed appliances at end).

### Assemble Flow (ze appliance assemble)

1. Read `<name>/appliance.json`
2. Obtain passphrase (agent > prompt > env var)
3. Decrypt secrets: password.hash, tls/key.pem, update.token
4. Verify all required secrets present; fail with clear error if missing
5. Resolve seed config: read config_base, append per-appliance ze.conf overlay
6. Validate merged config (parse and check for errors; report which file introduced the error)
7. If `credentials.admin_enabled` is false, write `meta/instance/admin-disabled` flag to ZeFS
8. Create ZeFS database via zefs.Create (all data in memory, decrypted secrets never touch disk):
   - meta/ssh/username, meta/ssh/password (decrypted hash), meta/ssh/host, meta/ssh/port
   - meta/ssh/authorized_keys (if configured)
   - meta/instance/name, meta/instance/managed, meta/instance/admin-disabled (if set)
   - meta/web/cert (plaintext cert.pem), meta/web/key (decrypted key.pem)
   - meta/build/manifest (config hash, timestamp, ze version)
   - file/template/ze.conf (resolved seed config)
   - meta/config/last-known-good (SHA-256 hash of validated seed config)
9. Write to `<name>/database.zefs`
10. Zero decrypted secrets from memory
11. Delete `<name>/database.zefs` unless `--keep` flag is set
12. Print: "database.zefs assembled (contains plaintext secrets, auto-deleted)" or with `--keep`: "WARNING: database.zefs retained (contains plaintext secrets, delete when done)"

**`--keep` flag:** retains database.zefs on disk for debugging or manual inspection. Without `--keep`, database.zefs is written, its SHA-256 is logged for verification, and it is immediately deleted. `ze appliance build` internally calls assemble with `--keep` (it needs the file for image injection) and deletes it after.

### Makefile Integration

```makefile
ze-gokrazy: ze bin/gok
	@bin/ze appliance build --dir appliances $(or $(APPLIANCE),default)

ze-gokrazy-push: ze
	@bin/ze appliance push --dir appliances $(or $(APPLIANCE),default) $(if $(IMG),--image $(IMG),)

ze-gokrazy-run: ze bin/gok
	@bin/ze appliance run --dir appliances $(or $(APPLIANCE),default) $(if $(IMG),--image $(IMG),)

ze-gokrazy-init:
	@bin/ze appliance init --dir appliances $(or $(APPLIANCE),default)

ze-gokrazy-all: ze bin/gok
	@bin/ze appliance build --all --dir appliances

ze-gokrazy-config-push: ze
	@bin/ze appliance config-push --dir appliances $(or $(APPLIANCE),default)

ze-gokrazy-export: ze
	@bin/ze appliance export --all --dir appliances
```

`ze appliance run` uses the most recent `ze-*.img` by default; `--image` overrides for rollback testing. `ze appliance push` sends the most recent image to the device; `--image` overrides for rollback. `ze appliance config-push` pushes config changes without rebuilding.

### .gitignore (Ze repo root)

```
appliances/
```

Development appliance data is ephemeral. Production appliance data lives on the bastion, outside the source tree.

## Resolved Questions

1. ~~Should `appliances/` live at repo root or under `gokrazy/`?~~ **Neither.** Default is `~/.config/ze/appliances/`. Repo root has a gitignored `appliances/` for dev convenience only. `--dir` and `ZE_APPLIANCE_DIR` env override.
2. ~~Should `ze appliance build` require `bin/gok` to exist, or auto-build it?~~ **Require it.** `ze appliance build` checks for `bin/gok` and prints a clear error ("run: make bin/gok") if missing. `ze appliance assemble` does not need gok. Makefile still has `bin/gok` as prerequisite for the `ze-gokrazy` target.
3. ~~Should e2fsprogs path be auto-detected or configurable?~~ **Auto-detect.** Check `PATH` first (Linux), then common brew prefix (`/opt/homebrew/opt/e2fsprogs/sbin/`, `/usr/local/opt/e2fsprogs/sbin/`). Fail with "install e2fsprogs" if not found.
4. ~~Should build and ZeFS assembly be separate?~~ **Yes.** `ze appliance assemble` = fast ZeFS-only path. `ze appliance build` = assemble + gok + ext4. OTA update = `ze fleet` scope (separate spec).
5. ~~Per-appliance gokrazy config.json hostname?~~ **Yes.** Build generates a temp config.json with Hostname patched from `identity.hostname`. Original `gokrazy/ze/config.json` is read-only template.
6. ~~QEMU port conflicts?~~ **Per-appliance qemu section in appliance.json.** Defaults: ssh=2222, web=28080, gokrazy=18080. `ze appliance run` uses these.
7. ~~Interface discovery?~~ **Not applicable at build time.** Seed config uses `dhcp-auto true`. Operators pre-populate interface config in their custom ze.conf for known hardware. Document this.
8. ~~ARM support?~~ **Yes, if gokrazy provides an ARM kernel.** `image.arch` supports `"amd64"` and `"arm64"`. The build cross-compiles via `GOARCH=<arch>`. If no ARM kernel package is available in the gokrazy builddir, build fails with a clear error. ARM testing requires QEMU `qemu-system-aarch64` (auto-detected).
9. ~~Gokrazy update password?~~ **Separate per-device update token.** A random 32-byte token is generated at init and stored encrypted in `secrets/update.token`. This is intentionally not the admin password: separating OTA access from interactive access limits blast radius. If the admin password leaks, the attacker cannot push firmware. If the update token leaks, they cannot log in.
10. ~~Config drift detection?~~ **`ze fleet` scope.** Before any remote config change, `ze fleet` must sync: either push local to device or pull device config to local. Mandatory resync, no silent overwrite. This spec only handles build-time config; runtime config management is out of scope.
11. ~~QEMU port conflicts?~~ **Detect at run time.** `ze appliance run` checks all configured ports (ssh, web, gokrazy) before launching QEMU. If any port is in use, fail with a message listing the conflicting port and which process holds it.
12. ~~Batch provisioning?~~ **`ze appliance init --batch <manifest.json>`** creates multiple appliances from a single manifest. Each entry is a partial appliance config (missing fields use defaults). Supports `"password": "generate"` for per-device unique passwords. Passphrase from env var or agent.
13. ~~Passphrase UX at scale?~~ **Passphrase agent.** `ze appliance unlock` starts a background process holding the derived key in memory via Unix domain socket. Commands check for the agent before prompting. Agent auto-expires (default 30 min). `--all` flags require an active agent.
14. ~~OTA updates in scope?~~ **Yes, single-device and batch.** `ze appliance push` wraps gokrazy's HTTP update API. This closes the gap between "image built" and "device updated" without waiting for `ze fleet`. Fleet-level orchestration (staged rollout, health checks, groups) remains in `ze fleet`.
15. ~~Image integrity?~~ **SHA-256 checksums + build manifest.** Checksums written alongside images; manifest includes config hash, timestamp, version. Build metadata also embedded in ZeFS for runtime device identification. No cryptographic signing (operator can GPG-sign externally).
16. ~~SSH auth?~~ **Password + keys.** Both supported. `credentials.ssh_authorized_keys` in config bakes public keys into ZeFS. Password auth remains for web UI, serial console, and as fallback.
17. ~~Assemble leaves plaintext on disk?~~ **Auto-delete by default.** `ze appliance assemble` deletes database.zefs after writing. `--keep` retains it for debugging with a sensitivity warning.
18. ~~Config preview before build?~~ **`ze appliance config --merged`.** Resolves base + overlay layering and prints effective config. No passphrase needed, no build performed.
19. ~~What does `managed` mean?~~ **Fleet mode.** When true, the device accepts remote config push from the hub, reports status/health, and local config edits are flagged as drift. `ze appliance show` includes this explanation.
20. ~~Passphrase loss?~~ **Documented disaster recovery.** No split-key or threshold recovery (complexity disproportionate to fleet size). Prevention: zone-specific passphrases, secure storage, periodic rekey. Recovery requires physical access to unreachable devices. See Disaster Recovery section.
21. ~~Config changes require full image rebuild?~~ **No.** `ze appliance config-push` pushes ze.conf to the running device via SSH. The device validates and applies, or auto-reverts on failure. Full rebuild is only needed for structural changes (binary, cert, password). This closes the biggest day-2 usability gap vs RouterOS.
22. ~~Sequential fleet operations?~~ **`--parallel N` flag.** `push --all` and `config-push --all` support concurrent operations via a bounded worker pool. Default is sequential (N=1). Maximum is 64. Each goroutine is independent (no shared mutable state).
23. ~~Bastion single point of failure?~~ **`ze appliance export/import`.** Export creates an encrypted archive of appliance directories (config + encrypted secrets, excluding images). Import restores on a fresh bastion. Archives are always encrypted, even if the appliance secrets are not. Operational recommendation: export after every fleet change, store on separate media.
24. ~~Device-side config revert on failure?~~ **Last-known-good.** Build writes SHA-256 of validated seed config to `meta/config/last-known-good` in ZeFS. Device validates pushed config on load; reverts to seed config on failure. Two-tier revert chain: previous pushed config, then ZeFS seed config. The ZeFS seed config is immutable (cannot be modified by config-push).

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-74 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`cmd/ze/appliance/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (`docs/guide/appliance.md`)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-appliance-1-builder.md`
- [ ] Summary included in commit
