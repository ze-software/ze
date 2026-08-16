# Spec: Cloud-Init for Gokrazy Appliance

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-install-8-appliance-iso (completed; closed, learned 854) |
| Phase | RESEARCH |
| Updated | 2026-06-03 |

STALE ANCHORS -- re-locate before any implementation (2026-07-22 plan
review): the entire build-integration surface this spec edits has MOVED.
`cmd/ze/install/appliance/` (its cited `config.go`, `cmd_build.go`,
`cmd_assemble.go`, `cmd_init.go`) no longer exists and no `ApplianceConfig`
struct remains anywhere; appliance code now lives under
`internal/appliance/`. `identity.Resolve` is at `:34` (cited `:32`). The
feature itself has not landed and the Depends (install-8, learned 854) is
satisfied, so the design intent stands -- but every file/type anchor must be
re-mapped to the reorganized tree first.

## Re-map notes (2026-07-22)

Anchor re-map performed against the current tree (every location below was read
and verified). The design intent survives; the build-integration surface it
edits has changed shape in five ways an implementer must re-plan around:

1. **CLI surface moved and was renamed.** Appliance tooling is now the
   self-contained command provider `internal/appliance/` (dispatch:
   `internal/appliance/main.go` `Run()`), registered as the root command
   `appliance` via `internal/appliance/register.go`
   (`registry.MustRegisterRootHandler("appliance", ...)`). The grammar is
   `ze appliance build|init|...`, NOT `ze install appliance ...`. Every
   `ze install appliance` spelling in this spec must be read as `ze appliance`.
2. **`ApplianceConfig` dissolved, not moved.** The config struct is now the
   UNEXPORTED `applianceConfig` (`internal/appliance/config.go`), and
   `SaveConfig` is now unexported `saveConfig` (`config.go`);
   `DefaultConfig` (`config.go`), `Validate` (`config.go`) and
   `LoadConfig` (`config.go`) remain. Adding the planned `CloudConfig`
   field is package-internal; there is no exported config surface to extend.
3. **Build runs gok in-process.** `runBuild` (`internal/appliance/cmd_build.go`)
   -> `runGokBuild` (`cmd_build.go`) -> `runGokInProcess` (`cmd_build.go`,
   embeds `github.com/gokrazy/tools/gok`, repo-local `gokrazy/modcache`), then
   `injectZeFS` (`cmd_build.go`) writes the ZeFS database into the /perm
   partition via debugfs. The `--cloud` variant ("omit pre-baked ZeFS") hooks
   into `buildOne` (`cmd_build.go`), which currently always assembles and
   injects the database. (Anchors re-verified 2026-07-23 after the
   origin/main fast-forward to 822029463, which touched `cmd_build.go`.)
4. **No new gokrazy builddir is needed.** `cmd/ze-cloud-init` would live in this
   same Go module, already covered by
   `gokrazy/ze/builddir/github.com/ze-software/ze/` — the cloud variant is a
   `Packages` change to a generated `gokrazy/ze/config.json` variant, not a new
   instance dir. The planned file
   `gokrazy/ze-cloud-init/builddir/github.com/ze-software/ze/go.mod` is
   obsolete as specified.
5. **The pushed-config slot the design targets already exists.** Ze reads
   `/perm/ze/config-pushed.conf` at boot: `cmd/ze/pushed_config.go`
   (`pushedConfigPath`), applied by `checkPushedConfig`
   (`cmd/ze/pushed_config.go`); its comment (`pushed_config.go`) already
   names cloud-init as the intended external writer. The "Config write" step of
   the Data Flow needs no new Ze-side code for the loose-file inbox.

Also verified unchanged: `internal/core/gokrazyutil/gokrazyutil.go` (password
paths at `:37`), `internal/core/identity/identity.go` (`Resolve` now at `:34`),
`gokrazy/ze/config.json` (still the instance config, but `serial-busybox` was
replaced by `cmd/ze-serial-shell`, and per-package `Environment` /
`CommandLineFlags` live under `PackageConfig`). The shell-script installer
initrd `tools/installer-initrd/init` was dissolved into pure Go:
`cmd/ze-installer/main.go` (build tag `ze_installer`) calling
`internal/install/disk.RunInitrd()`, built by `ze appliance initrd`
(`runInitrd`, `internal/appliance/cmd_initrd.go`).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/guide/appliance.md` - appliance architecture, boot flow, /perm layout
4. `internal/core/gokrazyutil/gokrazyutil.go` - gokrazy helper patterns
5. ~~`cmd/ze/install/appliance/config.go` - ApplianceConfig struct~~ (moved 2026-07-22 re-map: now `internal/appliance/config.go` - unexported `applianceConfig` struct)
6. ~~`cmd/ze/install/appliance/cmd_build.go` - image build flow~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_build.go` - `runBuild`)
7. ~~`cmd/ze/install/appliance/cmd_assemble.go` - ZeFS assembly, seed config injection~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_assemble.go` - `runAssemble`; seed config injection at `cmd_assemble.go`)

## Task

Add cloud-init support to the Ze gokrazy appliance so that a pre-built appliance image can boot on a cloud VM (AWS EC2, GCP GCE, or any provider exposing the standard IMDS at 169.254.169.254) and automatically provision itself on first boot without operator intervention.

The gokrazy appliance has no shell, no Python, no package manager. Standard cloud-init (the Python agent) cannot run. Ze must include a Go program that queries the cloud instance metadata service (IMDS) on first boot, extracts configuration data, and writes it to the persistent `/perm` partition. After first boot completes, subsequent boots use the persisted config normally.

## Problem Statement

Today, every Ze appliance carries a pre-baked seed config and ZeFS database injected at build time. This works for bare-metal and managed fleets, but cloud deployments have different needs:

1. **No pre-shared credentials.** Cloud VMs get SSH keys from the metadata service, not from an `appliance init` wizard.
2. **Dynamic identity.** Hostname, instance ID, and network config come from the cloud provider.
3. **User data.** Operators pass Ze config (BGP neighbors, interfaces, policies) via the cloud provider's user-data mechanism.
4. **No physical access.** There is no serial console wizard, no USB drive. The image must self-provision.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/appliance.md` - appliance boot flow, /perm layout, config priority, env vars
  → Constraint: root filesystem is read-only SquashFS; persistent data only on /perm (ext4)
  → Constraint: config priority is pushed > seed; cloud-init must fit this model
  → Constraint: no shell, no PATH; all functionality must be compiled Go
- [ ] `docs/architecture/core-design.md` - component registration, startup sequence
  → Decision: cloud-init agent runs as a separate gokrazy-supervised binary, not compiled into ze

### RFC Summaries (MUST for protocol work)
N/A (not protocol work).

**Key insights:**
- Gokrazy images are immutable root + persistent /perm. Cloud-init writes to /perm only.
- Ze already resolves machine-id from /etc/machine-id, hostname, or crypto/rand (`identity.Resolve()` in `internal/core/identity/identity.go`).
- The appliance config loading priority (pushed > seed) gives us a natural injection point: cloud-init writes to the "pushed" slot.
- Gokrazy supervises processes: a cloud-init binary that exits 0 after first boot is simply not restarted (exit code 125 = "don't restart").
- ~~`ApplianceConfig` (config.go)~~ (moved 2026-07-22 re-map: dissolved into unexported `applianceConfig` at `internal/appliance/config.go`) already has Identity, Credentials, SSH, Web, TLS, Device, Image, QEMU structs (plus Managed, ConfigBase). CloudConfig is a natural addition, now package-internal.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/gokrazyutil/gokrazyutil.go` - reads gokrazy password from /perm/gokr-pw.txt, /etc/gokr-pw.txt, /gokr-pw.txt
- [ ] `internal/core/identity/identity.go` - `Resolve()` machine-id resolution: zefs > /etc/machine-id > hostname > random; `Storage` interface for zefs read/write
- [ ] ~~`cmd/ze/install/appliance/config.go` - `ApplianceConfig` struct (line 61), `DefaultConfig()`, `Validate()`, `LoadConfig()`, `SaveConfig()`~~ (moved 2026-07-22 re-map: now `internal/appliance/config.go` - unexported `applianceConfig` struct at `:85`, `DefaultConfig()` `:107`, `Validate()` `:245`, `LoadConfig()` `:345`, unexported `saveConfig()` `:359`)
- [ ] ~~`cmd/ze/install/appliance/cmd_build.go` - image build: `runBuild()`, gok + ext4 /perm inject with ZeFS~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_build.go` - `runBuild()` `:76`, in-process gok via `runGokInProcess()` `:239`, ext4 /perm ZeFS inject via `injectZeFS()` `:324`)
- [ ] ~~`cmd/ze/install/appliance/cmd_assemble.go` - ZeFS assembly: password hash, TLS cert, seed config at `zefs.KeyFileTemplate.Key("ze.conf")`~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_assemble.go` - `assembleZeFS()` `:73`, seed config at `zefs.KeyFileTemplate.Key("ze.conf")` `:125`)
- [ ] ~~`cmd/ze/install/appliance/cmd_init.go` - `runInit()` appliance init wizard, `runBatchInit()` batch init~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_init.go` - `runInit()` `:40`, `runBatchInit()` `:329`)
- [ ] `gokrazy/ze/config.json` - gokrazy instance config: ~~Packages (serial-busybox, ze), Environment (ze.config.dir=/perm/ze), CommandLineFlags (start)~~ (updated 2026-07-22 re-map: path unchanged; Packages are now `cmd/ze-serial-shell` + `cmd/ze` (serial-busybox replaced), and `Environment` (`ze.config.dir=/perm/ze`) / `CommandLineFlags` (`start`) sit per-package under `PackageConfig`)
- [ ] ~~`tools/installer-initrd/init` - PXE/ISO installer initrd (PID 1 shell script), cmdline params ze.source, ze.server, etc.~~ (moved 2026-07-22 re-map: shell initrd dissolved; replaced by pure-Go PID-1 `cmd/ze-installer/main.go` (build tag `ze_installer`) calling `internal/install/disk.RunInitrd()`, built by `ze appliance initrd` -- `runInitrd` at `internal/appliance/cmd_initrd.go`)

**Behavior to preserve:**
- Bare-metal and on-prem appliances continue to work unchanged (pre-baked ZeFS + seed config)
- Existing config priority: pushed config > seed config
- ZeFS database format and key layout
- Gokrazy process supervision model (exit 125 = don't restart)
- Identity resolution chain: zefs > /etc/machine-id > hostname > random

**Behavior to change:**
- A new Go binary (`ze-cloud-init`) is added to the gokrazy image for cloud builds
- On first boot, ze-cloud-init queries IMDS and writes config + credentials to /perm
- ~~`ze install appliance build`~~ (moved 2026-07-22 re-map: grammar is now `ze appliance build` -- `internal/appliance/main.go` dispatch, `cmd_build.go`) gains a `--cloud` flag to include ze-cloud-init in the image
- ~~`ze install appliance init`~~ (moved 2026-07-22 re-map: grammar is now `ze appliance init` -- `internal/appliance/main.go` dispatch, `cmd_init.go`) gains a `--cloud` flag to configure a cloud-mode appliance

## Data Flow (MANDATORY)

### Entry Point
- Gokrazy starts `ze-cloud-init` as a supervised process alongside `ze`
- ze-cloud-init checks `/perm/ze/cloud-init-done` marker file
- If marker exists and instance-id matches: exit 125 (gokrazy: don't restart)
- If marker absent or instance-id changed: query IMDS, provision, write marker, exit 0

### Transformation Path
1. **IMDS detection**: try well-known metadata endpoints to identify cloud provider
2. **Metadata fetch**: retrieve instance identity (hostname, instance-id, region, SSH keys)
3. **User-data fetch**: retrieve operator-provided Ze config from user-data endpoint
4. **Credential generation**: if no ZeFS exists on /perm, generate a fresh TLS cert and password hash
5. **Config write**: write Ze config to `/perm/ze/config-pushed.conf` (the "pushed" config slot)
6. **Identity write**: write instance-id as machine-id to ZeFS via `identity.Storage` interface
7. **SSH key write**: write SSH authorized_keys to ZeFS
8. **Marker write**: write `/perm/ze/cloud-init-done` with timestamp + instance-id + provider name
9. **Exit 0**: gokrazy restarts ze-cloud-init; it sees the marker and exits 125

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Cloud IMDS → ze-cloud-init | HTTP GET to 169.254.169.254 (or provider-specific endpoint) | [ ] |
| ze-cloud-init → /perm filesystem | Direct file writes (ext4, writable) | [ ] |
| /perm → ze (config loading) | Ze reads /perm/ze/config-pushed.conf at startup | [ ] |
| /perm → ze (identity) | Ze reads machine-id from ZeFS via `identity.Resolve()` | [ ] |

### Integration Points
- `internal/core/identity/identity.go:Resolve()` (~~line 32~~ (moved 2026-07-22 re-map: now `:34`)) - reads machine-id from ZeFS; cloud-init writes it
- Ze config loading - reads pushed config from /perm; cloud-init writes it (verified 2026-07-22: `cmd/ze/pushed_config.go` `pushedConfigPath = "/perm/ze/config-pushed.conf"`, applied by `checkPushedConfig` at `:35`)
- `gokrazy/ze/config.json` - cloud variant includes ze-cloud-init as additional package
- ~~`cmd/ze/install/appliance/cmd_build.go:runBuild()`~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_build.go`) - build flow gains --cloud flag

### Architectural Verification
- [ ] No bypassed layers (cloud-init writes to existing config slots, Ze reads normally)
- [ ] No unintended coupling (ze-cloud-init is a standalone binary; Ze knows nothing about it)
- [ ] No duplicated functionality (reuses pushed-config and ZeFS patterns)
- [ ] Zero-copy preserved where applicable (N/A, this is a first-boot provisioner)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Gokrazy starts ze-cloud-init | → | cmd/ze-cloud-init/main.go:main() | TestCloudInitEntryPoint |
| IMDS response | → | internal/cloudinit/provider.go:Detect() | TestProviderDetection |
| User-data content | → | internal/cloudinit/provision.go:Provision() | TestProvisionFromUserData |
| --cloud flag on build | → | ~~cmd/ze/install/appliance/cmd_build.go~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_build.go`) | TestBuildCloudImage |
| Marker file present | → | cmd/ze-cloud-init/main.go:main() | TestSkipWhenAlreadyProvisioned |

## Design

### Cloud Provider Abstraction

A `Provider` interface abstracts the differences between cloud platforms:

| Method | Signature | Purpose |
|--------|-----------|---------|
| Name | `Name() string` | Human-readable provider name ("aws", "gcp", "generic") |
| Detect | `Detect(ctx context.Context) bool` | Returns true if running on this provider |
| FetchMetadata | `FetchMetadata(ctx context.Context) (*Metadata, error)` | Retrieve instance identity |
| FetchUserData | `FetchUserData(ctx context.Context) ([]byte, error)` | Retrieve operator-provided config |

**Metadata struct fields:**
- InstanceID (string)
- Hostname (string)
- Region (string, optional)
- SSHPublicKeys ([]string)
- NetworkConfig (optional, provider-specific)

**Provider implementations (phased):**

| Phase | Provider | Detection | Metadata endpoint |
|-------|----------|-----------|-------------------|
| 1 | AWS EC2 | IMDSv2 token PUT succeeds | `/latest/meta-data/`, `/latest/user-data` |
| 1 | GCP GCE | `Metadata-Flavor: Google` header present | `/computeMetadata/v1/` |
| 1 | Generic IMDS v1 | `GET http://169.254.169.254/` returns 200 | Standard paths |
| 2 | Azure | `Metadata: true` header | `/metadata/instance?api-version=...` |
| 2 | Oracle Cloud | Standard IMDS | `/opc/v2/instance/` |
| 2 | Vultr/Hetzner/DO | Standard IMDS | Provider-specific paths |
| 3 | NoCloud (ISO/seed) | Mount check for cidata volume label | Files on mounted volume |

Phase 1 providers cover the two largest clouds plus generic. Phase 2 and 3 are future specs.

Detection order: AWS (most specific, IMDSv2 PUT), GCP (header check), Generic (last resort). Each provider gets a 2s detection timeout. First match wins.

### User-Data Format

Ze cloud-init accepts user-data as a Ze config file (the same syntax used in `ze.conf`). No YAML, no MIME multipart. The user-data content is written directly to `/perm/ze/config-pushed.conf`.

If user-data starts with `#cloud-config` (standard cloud-init marker), ze-cloud-init rejects it with a log warning and falls back to the seed config. This prevents operators from accidentally pasting standard cloud-init YAML.

If user-data is empty, the seed config is used.

### First-Boot Sequence

| Step | Action | Failure behavior |
|------|--------|-----------------|
| 1 | ze-cloud-init starts (gokrazy supervision) | N/A |
| 2 | Check /perm/ze/cloud-init-done marker | If exists and instance-id matches: exit 125 |
| 3 | Detect cloud provider (try each in order, 2s timeout each) | No provider detected: log warning, exit 125 |
| 4 | Fetch metadata (5s timeout) | Log error, exit 1 (gokrazy retries) |
| 5 | Fetch user-data (5s timeout) | 404 = no user-data (OK). Other error: log, continue without user-data |
| 6 | If /perm/ze/database.zefs missing: generate TLS cert + random password, write to ZeFS | Exit 1 on failure (gokrazy retries) |
| 7 | Write SSH authorized_keys to ZeFS (from metadata SSH keys) | Log warning if no keys |
| 8 | Write machine-id to ZeFS (from metadata instance-id) | Exit 1 on failure |
| 9 | If user-data is non-empty and valid Ze config: write to /perm/ze/config-pushed.conf | Invalid config: log warning, skip (seed config used) |
| 10 | Write /perm/ze/cloud-init-done marker | Exit 1 on failure |
| 11 | Send SIGHUP to Ze process (config reload) | Log warning if Ze not found (will reload on next restart) |
| 12 | Exit 0 | Gokrazy restarts, step 2 catches marker, exit 125 |

### Race Condition: Ze vs ze-cloud-init

Both start simultaneously under gokrazy. Ze starts with the seed config (or no pushed config if fresh image). ze-cloud-init writes the pushed config.

→ Decision: ze-cloud-init sends SIGHUP to the Ze process after writing config.
Ze already handles SIGHUP as a config reload signal. ze-cloud-init finds Ze's PID by scanning `/proc/*/cmdline` for the ze binary.

Alternative considered: inotify watch on `/perm/ze/config-pushed.conf`. Rejected because it adds coupling and complexity, and is less gokrazy-idiomatic.

### Build-Time Integration

~~`ze install appliance build --cloud`~~ (moved 2026-07-22 re-map: now `ze appliance build --cloud`) (or `appliance.json` field `"cloud": {"enabled": true}`):

1. Generates a gokrazy config.json variant that includes `ze-cloud-init` as an additional Package
2. Omits the pre-baked ZeFS database (no password, no TLS cert). The seed config is still embedded via ExtraFileContents.
3. The image boots without credentials; ze-cloud-init generates them on first boot
4. Optional: `--cloud-provider aws|gcp|generic` sets the provider hint in appliance.json (skips detection at runtime)

### ApplianceConfig Changes

New `CloudConfig` struct added to ~~`ApplianceConfig`~~ (moved 2026-07-22 re-map: the struct is now the unexported `applianceConfig` at `internal/appliance/config.go`; the addition is package-internal):

| Field | Type | JSON key | Default | Purpose |
|-------|------|----------|---------|---------|
| Enabled | bool | `cloud.enabled` | false | Include ze-cloud-init in the image, omit pre-baked ZeFS |
| Provider | string | `cloud.provider` | "" | Provider hint: "aws", "gcp", "generic", or "" for auto-detect |

### Security Considerations

| Concern | Mitigation |
|---------|-----------|
| IMDS SSRF | AWS IMDSv2 (PUT token required). GCP Metadata-Flavor header. |
| User-data is untrusted input | Validated through Ze's config parser before writing. Invalid config rejected. |
| Generated password exposure | Logged once to stdout (gokrazy ring buffer). Plaintext never on disk (bcrypt hash only in ZeFS). |
| Marker file tampering | Marker contains instance-id. Mismatch triggers re-provisioning. |
| User-data size | Capped at 1 MB to prevent OOM. |
| HTTP timeout | All IMDS requests have context deadlines (2s detect, 5s fetch). |
| Secrets in user-data | Operator responsibility. Ze config supports ZeFS key references for sensitive values. |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Fresh cloud image boots on EC2 with user-data containing Ze config | ze-cloud-init detects AWS, fetches metadata + user-data, writes config to /perm, Ze loads it |
| AC-2 | Fresh cloud image boots on GCP with SSH keys in metadata | ze-cloud-init writes SSH authorized_keys, Ze SSH accepts those keys |
| AC-3 | Cloud image boots with no user-data | Ze uses seed config, ze-cloud-init still writes SSH keys and generates credentials |
| AC-4 | Cloud image reboots after successful first boot | ze-cloud-init sees marker file, exits 125 immediately, Ze starts normally |
| AC-5 | User-data contains `#cloud-config` YAML | ze-cloud-init rejects it with warning log, falls back to seed config |
| AC-6 | User-data contains invalid Ze config | ze-cloud-init rejects it with warning log, falls back to seed config |
| AC-7 | IMDS unreachable (not a cloud environment) | ze-cloud-init logs "no cloud provider detected", exits 125 |
| AC-8 | ~~`ze install appliance build --cloud lab`~~ (moved 2026-07-22 re-map: `ze appliance build --cloud lab`) | Image includes ze-cloud-init binary, no pre-baked ZeFS database |
| AC-9 | Image cloned to new instance (different instance-id) | ze-cloud-init re-runs, updates machine-id and SSH keys |
| AC-10 | ze-cloud-init finishes after Ze has started | ze-cloud-init sends SIGHUP to Ze, Ze reloads with the new pushed config |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDetectAWS` | `internal/cloudinit/provider_test.go` | AWS IMDSv2 detection with mock HTTP server | |
| `TestDetectGCP` | `internal/cloudinit/provider_test.go` | GCP detection via Metadata-Flavor header | |
| `TestDetectGeneric` | `internal/cloudinit/provider_test.go` | Generic IMDS v1 detection (fallback) | |
| `TestDetectNone` | `internal/cloudinit/provider_test.go` | No provider when IMDS unreachable | |
| `TestDetectOrder` | `internal/cloudinit/provider_test.go` | AWS checked before GCP before generic | |
| `TestFetchMetadataAWS` | `internal/cloudinit/aws_test.go` | Parses instance-id, hostname, SSH keys from mock IMDS | |
| `TestFetchMetadataGCP` | `internal/cloudinit/gcp_test.go` | Parses instance name, SSH keys from mock IMDS | |
| `TestFetchUserData` | `internal/cloudinit/provider_test.go` | Returns raw user-data bytes | |
| `TestFetchUserDataEmpty` | `internal/cloudinit/provider_test.go` | Returns nil when 404 | |
| `TestFetchUserDataTooLarge` | `internal/cloudinit/provider_test.go` | Rejects user-data over 1 MB | |
| `TestProvisionFresh` | `internal/cloudinit/provision_test.go` | Writes config, SSH keys, machine-id, generates creds, writes marker | |
| `TestProvisionWithZeFS` | `internal/cloudinit/provision_test.go` | Existing ZeFS: writes SSH keys + config, does not regenerate creds | |
| `TestProvisionRejectsCloudConfig` | `internal/cloudinit/provision_test.go` | User-data starting with #cloud-config is rejected | |
| `TestProvisionRejectsInvalidConfig` | `internal/cloudinit/provision_test.go` | Malformed Ze config is rejected, seed config used | |
| `TestMarkerFileSkip` | `internal/cloudinit/provision_test.go` | Marker file present + same instance-id = skip | |
| `TestMarkerFileRerun` | `internal/cloudinit/provision_test.go` | Marker file present + different instance-id = re-run | |
| `TestBuildCloudImage` | ~~`cmd/ze/install/appliance/cmd_build_test.go`~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_build_test.go`) | --cloud flag produces config.json with ze-cloud-init package | |
| `TestCloudApplianceConfig` | ~~`cmd/ze/install/appliance/config_test.go`~~ (moved 2026-07-22 re-map: now `internal/appliance/config_test.go`) | CloudConfig struct validates, serializes, defaults | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IMDS detect timeout | 1s-10s | 10s | 0s | 11s |
| IMDS fetch timeout | 1s-30s | 30s | 0s | 31s |
| User-data max size | 1-1048576 bytes | 1048576 | N/A | 1048577 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-cloud-init-provision` | `test/install/cloud-init-provision.ci` | Mock IMDS server, fresh /perm, verify config + creds written | |
| `test-cloud-init-skip` | `test/install/cloud-init-skip.ci` | Marker file present, verify immediate exit 125 | |
| `test-cloud-init-reject-yaml` | `test/install/cloud-init-reject-yaml.ci` | #cloud-config user-data rejected, seed config preserved | |
| `test-cloud-init-build` | `test/install/cloud-init-build.ci` | --cloud flag produces image with ze-cloud-init package | |
| `test-cloud-init-rerun` | `test/install/cloud-init-rerun.ci` | Changed instance-id triggers re-provisioning | |

### Interop Tests (MANDATORY for protocol features)
N/A (not protocol work). Cloud provider interop validated manually on real cloud infrastructure.

### Future (if deferring any tests)
- Live AWS/GCP integration tests (require cloud credentials, manual execution)
- Azure, Oracle Cloud, Vultr provider implementations (Phase 2 spec)
- NoCloud/cidata provider for air-gapped environments (Phase 3 spec)

## Files to Modify

- ~~`cmd/ze/install/appliance/config.go` - add CloudConfig struct to ApplianceConfig~~ (moved 2026-07-22 re-map: now `internal/appliance/config.go` - add CloudConfig field to the unexported `applianceConfig` at `:85`)
- ~~`cmd/ze/install/appliance/cmd_build.go`~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_build.go`) - --cloud flag, generate cloud gokrazy config.json variant
- ~~`cmd/ze/install/appliance/cmd_init.go`~~ (moved 2026-07-22 re-map: now `internal/appliance/cmd_init.go`) - --cloud flag for init wizard

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A (build-time tooling, not runtime config tree) |
| CLI commands/flags | Yes | `--cloud` and `--cloud-provider` on appliance build/init |
| Doctor check for runtime dependencies | No | ze-cloud-init runs on the appliance, not the build host |
| Prometheus counters/metrics | No | First-boot provisioner, not a long-running service |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add cloud deployment |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - add Cloud Deployment section |
| 12 | Internal architecture changed? | No | Standalone binary, no core architecture changes |

## Files to Create

- `cmd/ze-cloud-init/main.go` - entry point: marker check, provider detect, provision, exit
- `internal/cloudinit/provider.go` - Provider interface, Detect(), provider registry
- `internal/cloudinit/aws.go` - AWS EC2 IMDSv2 provider
- `internal/cloudinit/gcp.go` - GCP GCE provider
- `internal/cloudinit/generic.go` - Generic IMDS v1 provider (fallback)
- `internal/cloudinit/metadata.go` - Metadata struct definition
- `internal/cloudinit/provision.go` - Provision(): orchestrates writes to /perm
- `internal/cloudinit/provider_test.go` - provider detection tests
- `internal/cloudinit/aws_test.go` - AWS-specific tests with mock IMDS
- `internal/cloudinit/gcp_test.go` - GCP-specific tests with mock IMDS
- `internal/cloudinit/provision_test.go` - provisioning logic tests
- ~~`gokrazy/ze-cloud-init/builddir/github.com/ze-software/ze/go.mod` - gokrazy builddir for ze-cloud-init~~ (moved 2026-07-22 re-map: obsolete -- `cmd/ze-cloud-init` is in this module, already covered by the existing `gokrazy/ze/builddir/github.com/ze-software/ze/`; the cloud variant only adds the package to a generated `gokrazy/ze/config.json` variant's `Packages`)
- `test/install/cloud-init-*.ci` - functional tests

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register ze-cloud-init entry point, write failing wiring tests
   - Tests: TestCloudInitEntryPoint, TestProviderDetection
   - Files: cmd/ze-cloud-init/main.go, internal/cloudinit/provider.go
   - Verify: binary compiles, entry point exists, tests fail because logic is stub

2. **Phase: Provider Detection** - implement cloud provider detection with mock HTTP
   - Tests: TestDetectAWS, TestDetectGCP, TestDetectGeneric, TestDetectNone, TestDetectOrder
   - Files: internal/cloudinit/provider.go, aws.go, gcp.go, generic.go
   - Verify: detection works against mock HTTP servers, correct priority order

3. **Phase: Metadata Fetch** - implement metadata + user-data retrieval per provider
   - Tests: TestFetchMetadataAWS, TestFetchMetadataGCP, TestFetchUserData, TestFetchUserDataEmpty, TestFetchUserDataTooLarge
   - Files: internal/cloudinit/aws.go, gcp.go, generic.go, metadata.go
   - Verify: correct parsing of mock IMDS responses, size limit enforced

4. **Phase: Provisioning** - implement /perm writes: config, creds, SSH keys, marker
   - Tests: TestProvisionFresh, TestProvisionWithZeFS, TestProvisionRejectsCloudConfig, TestProvisionRejectsInvalidConfig, TestMarkerFileSkip, TestMarkerFileRerun
   - Files: internal/cloudinit/provision.go
   - Verify: correct files written to mock /perm directory

5. **Phase: Build Integration** - --cloud flag on appliance build/init, CloudConfig
   - Tests: TestBuildCloudImage, TestCloudApplianceConfig
   - Files: ~~cmd/ze/install/appliance/config.go, cmd_build.go, cmd_init.go~~ (moved 2026-07-22 re-map: now internal/appliance/config.go, cmd_build.go, cmd_init.go)
   - Verify: cloud image config includes ze-cloud-init package, omits pre-baked ZeFS

6. **Functional tests** - create .ci tests after feature logic works
7. **Full verification** - `make ze-precommit-verify`
8. **Complete spec** - fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Correctness | IMDSv2 token flow correct (PUT to get token, GET with X-aws-ec2-metadata-token header) |
| Correctness | GCP uses Metadata-Flavor: Google on every request |
| Correctness | SIGHUP delivery finds correct Ze PID via /proc scan |
| Naming | CloudConfig JSON keys use kebab-case per cli.md |
| Data flow | ze-cloud-init writes only to /perm, never to read-only root |
| Security | User-data validated before writing; no arbitrary command execution |
| Security | IMDSv2 used for AWS (prevents SSRF via token requirement) |
| Security | Generated password bcrypt-hashed, plaintext only in gokrazy ring buffer |
| Rule: no-layering | ze-cloud-init is standalone; does not modify Ze's config loading |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| ze-cloud-init binary compiles | `go build ./cmd/ze-cloud-init/` |
| Provider detection works | `go test ./internal/cloudinit/ -run TestDetect` |
| Provisioning writes correct files | `go test ./internal/cloudinit/ -run TestProvision` |
| --cloud flag accepted by build | ~~`go test ./cmd/ze/install/appliance/ -run TestBuildCloud`~~ (moved 2026-07-22 re-map: `go test ./internal/appliance/ -run TestBuildCloud`) |
| CloudConfig in ApplianceConfig | ~~`go test ./cmd/ze/install/appliance/ -run TestCloudAppliance`~~ (moved 2026-07-22 re-map: `go test ./internal/appliance/ -run TestCloudAppliance`) |
| Functional tests exist | `ls test/install/cloud-init-*.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | User-data parsed as Ze config; reject anything that does not parse. Size capped at 1 MB. |
| SSRF prevention | AWS IMDSv2 (PUT token required), GCP Metadata-Flavor header check |
| Credential handling | Password plaintext never on disk; only bcrypt hash in ZeFS |
| File permissions | All /perm/ze/ files written with 0600 |
| Resource exhaustion | User-data 1 MB cap, HTTP timeouts on all IMDS requests |
| Timeout enforcement | Context deadlines: 2s per detection probe, 5s per metadata/user-data fetch |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong, back to design |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate binary (ze-cloud-init) | Compile into ze, gokrazy hook | Gokrazy supervises binaries independently. Ze stays unaware of cloud-init. Clean separation. |
| SIGHUP for config reload | inotify watch, restart Ze, polling | Ze already handles SIGHUP. Minimal coupling. No new infrastructure. |
| Ze config as user-data format | YAML (cloud-init standard), JSON, custom DSL | Ze config syntax is what operators already know. No translation layer. Rejects #cloud-config to prevent confusion. |
| Marker file for idempotency | Systemd-style flag, ZeFS key | Simple, inspectable, gokrazy-friendly. Contains instance-id for clone detection. |
| Auto-detect provider with hint | Require explicit provider | Most users don't want to specify. Detection is deterministic and fast. Build-time hint available for optimization. |
| Exit 125 when done | Stay running and sleep | Gokrazy convention: 125 = "intentionally stopped." Frees memory. |
| Phase 1: AWS + GCP + generic | All providers at once | Covers the two largest clouds. Generic catches most smaller providers. Prevents scope creep. |
| No ZeFS in cloud images | Pre-bake empty ZeFS | Cloud VMs have no pre-shared secrets. Generating at first boot is the cloud-native pattern. Seed config still embedded for basic connectivity. |

## Known Limitations

- Phase 1 covers AWS, GCP, and generic IMDS only. Azure, Oracle Cloud, others are Phase 2.
- NoCloud (ISO/seed volume with cidata label) is Phase 3, requires block device scanning.
- Network config from IMDS is not applied. Ze uses DHCP auto-discovery, which works for default cloud VPC networks. Multi-interface or static IP setups require user-data.
- No support for cloud-init modules (packages, runcmd, write_files). Ze is not a general-purpose OS.
- Generated SSH password is only visible in the gokrazy ring buffer. If the instance restarts before the operator retrieves it, the password is lost. SSH key auth (from IMDS) is the primary access method.
- ze-cloud-init does not handle instance metadata refreshes after first boot (e.g., SSH key rotation via IMDS). Key rotation requires config-push or a rebuilt image.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (Provider interface justified by 3 implementations in Phase 1)
- [ ] No speculative features (Phase 2/3 are explicit future specs, not stubs)
- [ ] Single responsibility per component (ze-cloud-init does provisioning only)
- [ ] Explicit > implicit behavior (marker file, detection logging, #cloud-config rejection)
- [ ] Minimal coupling (Ze knows nothing about cloud-init; writes to existing config slots)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cloud-init.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-install-9-cloud-init.md`
