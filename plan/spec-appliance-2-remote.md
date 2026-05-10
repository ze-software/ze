# Spec: appliance-2-remote

| Field | Value |
|-------|-------|
| Status | design |
| Depends | appliance-1-builder |
| Phase | - |
| Updated | 2026-05-10 |
| Split | Split from appliance-1-builder. Device-side config loading/revert is in appliance-4-device-config. |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/675-appliance-1-builder.md` - learned summary (builder spec deleted after completion)
4. `cmd/ze/appliance/main.go` - dispatch map, extractDirFlag, getBaseDir()
5. `cmd/ze/appliance/crypto.go` - Encrypt/Decrypt/ResolvePassphrase/ReadSecret
6. `cmd/ze/appliance/config.go` - ApplianceConfig (DeviceConfig.Address, DeviceConfig.UpdatePort)
7. `cmd/ze/appliance/cmd_assemble.go` - resolveSeedConfig() for config layering logic
8. `cmd/ze/appliance/agent.go` - passphrase agent (key-on-socket protocol)

## Task

Remote operations for ze appliance: OTA push, config preview, batch init, config-push (bastion side), and parallel fleet operations.

Split from `spec-appliance-1-builder` (design session 2026-05-09). The build-time tooling (init, assemble, build, day-2 ops, list/show, crypto, passphrase agent) ships in appliance-1-builder. This spec adds the remote operations that depend on that foundation.

Device-side behavior (config validation, auto-revert, last-known-good) is tracked separately in `spec-appliance-4-device-config`.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/675-appliance-1-builder.md` - Builder learned summary; design decisions inherited
  -> Decision: key-on-socket agent protocol; kebab-case JSON; spec split into 4 specs
  -> Constraint: gok invocation and QEMU are stubs pending binary availability
- [ ] `docs/guide/appliance.md` - appliance guide (updated by appliance-1-builder)
  -> Constraint: update with push, config, config-push, batch init, parallel operations sections
- [ ] `ai/patterns/cli-command.md` - CLI command structure
  -> Constraint: follow same dispatch pattern as existing appliance commands

### Source Files
- [ ] `cmd/ze/appliance/main.go` (144L) - handlers map (line 21), extractDirFlag, getBaseDir(), stub pattern, usage()
  -> Decision: add push, config, config-push to handlers map; add to usage() Sections
- [ ] `cmd/ze/appliance/crypto.go` (195L) - Encrypt/Decrypt (AEAD envelope), ResolvePassphrase (agent > env > prompt), ReadSecret, ZeroBytes
  -> Decision: push uses ReadSecret to decrypt update.token; --all requires agent (no interactive prompt)
  -> Constraint: ZeroBytes(passphrase) and ZeroBytes(updateToken) after use
- [ ] `cmd/ze/appliance/agent.go` (89L) - RunAgent (key-on-socket), requestKeyFromAgent (reads 32B key)
  -> Constraint: --all operations refuse to run without agent; interactive passphrase not safe for batch
- [ ] `cmd/ze/appliance/config.go` (198L) - ApplianceConfig with DeviceConfig{Address, UpdatePort}, LoadConfig, Validate
  -> Decision: push reads device.address and device.update-port from config; --all iterates appliances with device.address set
- [ ] `cmd/ze/appliance/cmd_init.go` (294L) - runInit, promptConfig, readPasswordValue, readPassphraseForInit
  -> Decision: add --batch flag to runInit; batch reads manifest JSON, iterates entries
- [ ] `cmd/ze/appliance/cmd_assemble.go` (197L) - assembleZeFS, resolveSeedConfig (base + overlay concatenation)
  -> Decision: config preview reuses resolveSeedConfig() directly; config-push merges then pushes
- [ ] `cmd/ze/appliance/cmd_build.go` (142L) - buildAll() directory iteration pattern (line 102)
  -> Decision: push --all and config-push --all reuse same iteration pattern

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/appliance/main.go` (144L) - handlers map dispatches 11 subcommands. extractDirFlag handles --dir/--dir=. getBaseDir() returns resolved path. usage() lists commands in helpfmt.Page.
- [ ] `cmd/ze/appliance/crypto.go` (195L) - Encrypt/Decrypt with XChaCha20-Poly1305. ResolvePassphrase: agent > env(ze.appliance.passphrase) > prompt callback. ReadSecret: read file, decrypt if passphrase provided. ZeroBytes for key zeroing.
- [ ] `cmd/ze/appliance/agent.go` (89L) - RunAgent: listen on Unix socket, serve 32B derived key on each connection, expire after duration. requestKeyFromAgent: dial socket, read 32B key.
- [ ] `cmd/ze/appliance/config.go` (198L) - ApplianceConfig struct. DeviceConfig has Address (string, omitempty) and UpdatePort (int, default 443). LoadConfig unmarshals JSON. SaveConfig writes with atomic rename.
- [ ] `cmd/ze/appliance/cmd_init.go` (294L) - runInit: parse flags (--config, --cert, --key, --managed), create dirs, hash password, generate TLS cert, generate update token, write secrets, save config. No --batch flag yet.
- [ ] `cmd/ze/appliance/cmd_assemble.go` (197L) - resolveSeedConfig(baseDir, name, cfg): reads ConfigBase (absolute or relative path), reads ze.conf overlay, concatenates base + "\n" + overlay. Used by assembleZeFS.
- [ ] `cmd/ze/appliance/cmd_build.go` (142L) - buildAll(): ReadDir, skip _shared/dotfiles, LoadConfig to validate, iterate.
- [ ] `cmd/ze/appliance/manifest.go` (66L) - BuildManifest, ImageTimestamp, ImageFileName, WriteImageChecksum, ConfigHash.

**Appliance directory layout** (relevant for push):

| Path | Role in remote ops |
|------|--------------------|
| appliance.json | DeviceConfig.Address + UpdatePort for push target |
| ze.conf | Per-appliance config overlay (config preview, config-push) |
| ze-*.img | Disk images (push sends these) |
| ze-*.img.sha256 | Checksums (push verifies before sending) |
| build.json | Manifest with image name |
| secrets/update.token | Decrypted for HTTP basic auth during push |
| secrets/tls/cert.pem | Device TLS cert (push verifies against this) |

**Behavior to preserve:**
- All appliance-1-builder functionality unchanged
- Passphrase agent protocol (key-on-socket, 32B derived key)
- Config layering semantics: resolveSeedConfig(base + "\n" + overlay)
- appliance.json schema (kebab-case JSON)
- buildAll() iteration pattern for --all operations

**Behavior to change:**
- Add --batch flag to init (batch manifest JSON input)
- Add push, config, config-push subcommands to dispatch
- Add push, config, config-push to usage() help text

## Data Flow (MANDATORY)

### Entry Point
- `ze appliance push <name>` dispatched via handlers map in main.go
- `ze appliance push --all` iterates appliances with device.address set
- `ze appliance config <name> --merged` dispatched via handlers map
- `ze appliance config-push <name>` dispatched via handlers map
- `ze appliance config-push --all` iterates appliances with device.address set
- `ze appliance init --batch <manifest.json>` extends existing runInit in cmd_init.go

### Transformation Path

**Push flow:**
1. getBaseDir() + LoadConfig(ConfigPath(dir, name)) to load ApplianceConfig
2. Read device.address and device.update-port from config; error if address empty
3. ResolvePassphrase(prompt) if IsEncrypted(dir, name); for --all, ResolvePassphrase(nil) (agent-only)
4. ReadSecret(secretFilePath(dir, name, "update.token"), passphrase) to decrypt update token
5. Select image: --image flag, or most recent ze-*.img in AppliancePath(dir, name)
6. If .sha256 exists, verify checksum before sending
7. Load TLS cert from TLSDir(dir, name)/cert.pem; create TLS config trusting this cert
8. HTTP PUT to https://device.address:device.update-port/update with basic auth (user: "", password: update token)
9. Stream image body; print progress
10. ZeroBytes(updateToken), ZeroBytes(passphrase) after use
11. Print "Pushed <image> to <hostname> (<address>)"

**Config preview flow:**
1. getBaseDir() + LoadConfig(ConfigPath(dir, name))
2. resolveSeedConfig(dir, name, cfg) (reuse from cmd_assemble.go)
3. Print effective config to stdout
4. If config is empty, print warning

**Config push flow:**
1. getBaseDir() + LoadConfig(ConfigPath(dir, name))
2. resolveSeedConfig(dir, name, cfg) to merge base + overlay
3. Validate merged config locally (parse check)
4. If --dry-run: print merged config and exit
5. Read device.address from config; error if empty
6. SSH to device using operator's SSH key (standard ssh agent, not Ze's passphrase agent)
7. Upload merged config to /perm/ze/config-staged.conf
8. Execute remote: ze config validate /perm/ze/config-staged.conf
9. If valid: remote saves previous as config-previous.conf, moves staged to config-pushed.conf
10. If invalid: remote deletes staged, report error to bastion
11. Print "config applied to <hostname>" or error

**Batch init flow:**
1. Parse --batch <manifest.json> flag in runInit
2. json.Unmarshal manifest into slice of batch entries
3. For each entry: construct ApplianceConfig, call existing init logic
4. If password == "generate": generate random password per device, print to stdout

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Bastion -> device (push) | HTTPS PUT, basic auth (empty user, update token as password), TLS verified against stored cert.pem | [ ] |
| Bastion -> device (config-push) | SSH with operator's public key (ssh agent, not Ze agent) | [ ] |
| Manifest JSON -> Go structs | encoding/json Unmarshal into batch entry slice | [ ] |
| Encrypted update token -> memory | ReadSecret(path, passphrase) via ResolvePassphrase | [ ] |
| Config files -> merged config | resolveSeedConfig() from cmd_assemble.go | [ ] |

### Integration Points
- `cmd/ze/appliance/crypto.go:ResolvePassphrase` - passphrase for update token decryption
- `cmd/ze/appliance/crypto.go:ReadSecret` - decrypt update.token file
- `cmd/ze/appliance/crypto.go:ZeroBytes` - zero sensitive data after use
- `cmd/ze/appliance/cmd_assemble.go:resolveSeedConfig` - config layering for preview and push
- `cmd/ze/appliance/config.go:LoadConfig` - load appliance.json
- `cmd/ze/appliance/config.go:DeviceConfig` - device.address and device.update-port
- `cmd/ze/appliance/resolve.go` - ResolveDir, AppliancePath, SecretsDir, TLSDir
- `cmd/ze/appliance/manifest.go` - ImageFileName pattern for finding latest image
- `cmd/ze/appliance/agent.go` - passphrase agent required for --all operations

### Architectural Verification
- [ ] No bypassed layers (reuses crypto, config, resolve from appliance-1-builder)
- [ ] No unintended coupling (push/config/config-push are independent subcommands)
- [ ] No duplicated functionality (config merging reuses resolveSeedConfig)
- [ ] Push TLS verification uses stored cert (not system CA pool)
- [ ] Config-push SSH uses operator's ssh agent (not Ze's passphrase agent)
- [ ] Zero-copy not applicable (offline CLI tool, not wire protocol)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance push lab` CLI | -> | `cmd/ze/appliance/cmd_push.go:cmdPush()` | `TestPushSendsImage` |
| `ze appliance config lab --merged` CLI | -> | `cmd/ze/appliance/cmd_config.go:cmdConfig()` | `TestConfigMergedOutput` |
| `ze appliance config-push lab` CLI | -> | `cmd/ze/appliance/cmd_config_push.go:cmdConfigPush()` | `TestConfigPushUploadsConfig` |
| `ze appliance init --batch m.json` CLI | -> | `cmd/ze/appliance/cmd_init.go:cmdBatchInit()` | `TestBatchInitCreatesMultiple` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-28 | `ze appliance init --batch manifest.json` with 3 entries | Creates 3 appliance directories, each with config + encrypted secrets |
| AC-39 | `ze appliance push lab` after build | Image pushed to device via gokrazy HTTP update API; device reboots to new partition |
| AC-40 | `ze appliance push lab --image ze-20260427-143022.img` | Specific image pushed (rollback to previous build) |
| AC-41 | `ze appliance push lab` with device unreachable | Exit code 1, stderr "error: device edge-01 unreachable at <hostname>:<port>" |
| AC-43 | `ze appliance push --all` with 3 appliances | All 3 devices updated; per-device status printed |
| AC-46 | `ze appliance config lab --merged` | Prints effective config (base + overlay merged); no build performed |
| AC-47 | `ze appliance config lab --merged` with config_base | Output shows base settings overridden by overlay |
| AC-51 | `ze appliance init --batch manifest.json` with `"password": "generate"` | Each appliance gets a unique random password; passwords printed to stdout (sealed output) |
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

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPushSendsImage` | `cmd/ze/appliance/cmd_push_test.go` | Push sends image to gokrazy HTTP update endpoint (mocked) | |
| `TestPushUnreachableDevice` | `cmd/ze/appliance/cmd_push_test.go` | Unreachable device -> clear error with hostname | |
| `TestPushWrongToken` | `cmd/ze/appliance/cmd_push_test.go` | 401 response -> clear error about update token | |
| `TestPushSpecificImage` | `cmd/ze/appliance/cmd_push_test.go` | --image flag pushes specific image (rollback) | |
| `TestPushAllIteratesAppliances` | `cmd/ze/appliance/cmd_push_test.go` | --all pushes to every appliance; per-device status | |
| `TestConfigMergedOutput` | `cmd/ze/appliance/cmd_config_test.go` | --merged prints effective config after base + overlay | |
| `TestConfigMergedWithDelete` | `cmd/ze/appliance/cmd_config_test.go` | Overlay delete removes base setting from merged output | |
| `TestBatchInitCreatesMultiple` | `cmd/ze/appliance/cmd_init_test.go` | Batch manifest creates N appliance dirs with config + secrets | |
| `TestBatchInitMissingEnvFails` | `cmd/ze/appliance/cmd_init_test.go` | Batch without ZE_APPLIANCE_SSH_PASSWORD env var fails | |
| `TestBatchInitPerDevicePasswords` | `cmd/ze/appliance/cmd_init_test.go` | password=generate creates unique password per device; passwords printed | |
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

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| --parallel N | 1-64 | 64 | 0 | N/A (clamped to device count) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-appliance-config-merged` | `test/appliance/config-merged.ci` | Config preview matches what assemble produces | |
| `test-appliance-batch-init` | `test/appliance/batch-init.ci` | Batch init creates multiple appliances with unique credentials | |
| `test-appliance-config-push` | `test/appliance/config-push.ci` | Config-push to test device (mocked SSH); verify config applied | |

### Future (if deferring any tests)
- OTA push test requires running gokrazy instance; manual verification only
- Parallel push stress test (high N) requires multiple running gokrazy instances
- Config-push integration test requires running ze device with SSH; mocked in unit tests

## Files to Modify
- `cmd/ze/appliance/main.go` - Add push, config, config-push dispatch cases
- `cmd/ze/appliance/cmd_init.go` - Add --batch flag and batch init logic
- `cmd/ze/appliance/register.go` - Register new subcommands

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (offline command) |
| CLI commands/flags | Yes | `cmd/ze/appliance/main.go` |
| Editor autocomplete | No | N/A (offline command) |
| Functional test for new RPC/API | No | N/A (offline command) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` - push, config-push, batch init, parallel |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md` - CLI reference |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create
- `cmd/ze/appliance/cmd_push.go` - OTA push via gokrazy HTTP update API (single device or --all)
- `cmd/ze/appliance/cmd_push_test.go` - OTA push tests (mocked HTTP)
- `cmd/ze/appliance/cmd_config.go` - Config preview: --merged shows effective config after layering
- `cmd/ze/appliance/cmd_config_test.go` - Config preview tests
- `cmd/ze/appliance/cmd_config_push.go` - Config push to running device via SSH without rebuild
- `cmd/ze/appliance/cmd_config_push_test.go` - Config push tests (mocked SSH)
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

1. **Phase: OTA push** -- Push image to device via gokrazy HTTP update API, TLS verification (self-signed cert from secrets/), update token auth, --image for rollback, --all for fleet push
   - Tests: `TestPushSendsImage`, `TestPushUnreachableDevice`, `TestPushWrongToken`, `TestPushSpecificImage`, `TestPushAllIteratesAppliances`
   - Files: `cmd/ze/appliance/cmd_push.go`, `cmd/ze/appliance/cmd_push_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Config preview** -- `ze appliance config <name> --merged` shows effective config after base + overlay; no build needed
   - Tests: `TestConfigMergedOutput`, `TestConfigMergedWithDelete`
   - Files: `cmd/ze/appliance/cmd_config.go`, `cmd/ze/appliance/cmd_config_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Batch init** -- `--batch <manifest.json>`, per-device password generation (`"password": "generate"`), env var requirements, independent crypto state per appliance
   - Tests: `TestBatchInitCreatesMultiple`, `TestBatchInitMissingEnvFails`, `TestBatchInitPerDevicePasswords`
   - Files: update `cmd/ze/appliance/cmd_init.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Config push (bastion side)** -- `ze appliance config-push <name>` pushes merged ze.conf to running device via SSH; device validates and applies or rejects; saves previous config for manual recovery
   - Tests: `TestConfigPushUploadsConfig`, `TestConfigPushInvalidConfigReverts`, `TestConfigPushUnreachableDevice`, `TestConfigPushDryRun`, `TestConfigPushAllDevices`, `TestConfigPushSavesPrevious`
   - Files: `cmd/ze/appliance/cmd_config_push.go`, `cmd/ze/appliance/cmd_config_push_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Parallel operations** -- `--parallel N` flag for `push --all` and `config-push --all`; bounded worker pool; per-device status; continues on individual failure
   - Tests: `TestPushAllParallel`, `TestPushAllParallelPartialFailure`, `TestPushAllParallelDefault`, `TestConfigPushAllParallel`
   - Files: `cmd/ze/appliance/parallel.go`, update `cmd/ze/appliance/cmd_push.go`, `cmd/ze/appliance/cmd_config_push.go`
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -> Create after feature works. Cover user-visible behavior.
7. **Full verification** -> `make ze-verify` (lint + all ze tests)
8. **Complete spec** -> Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation with file:line |
| Correctness | Push uses update token (not admin password); TLS verified against stored cert |
| Naming | Subcommand names match spec (push, config, config-push) |
| Data flow | Update token decrypted via agent, never written to disk; config layering reuses assemble logic |
| OTA push TLS | Push verifies device TLS cert against stored cert.pem; rejects unknown certs |
| Config preview accuracy | `config --merged` output matches what assemble would produce |
| Config push safety | No secrets cross the SSH channel; only ze.conf content |
| Parallel isolation | Each goroutine has independent state; no shared mutable data |
| Batch init isolation | Each appliance gets independent salt/nonce; no shared crypto state |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OTA push | `ze appliance push <name>` pushes to device (manual verification with running gokrazy) |
| Config preview | `ze appliance config <name> --merged` outputs effective config |
| Batch init | `ze appliance init --batch <manifest>` creates multiple appliances |
| Config push | `ze appliance config-push <name>` pushes config to running device |
| Config push dry-run | `ze appliance config-push <name> --dry-run` prints merged config |
| Parallel push | `ze appliance push --all --parallel 4` pushes to devices concurrently |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| **OTA push TLS** | Push verifies device TLS cert against stored cert.pem; rejects unknown certs; update token sent via HTTP basic auth |
| **Batch init isolation** | Each appliance in a batch gets independent salt/nonce for encryption; no shared crypto state between appliances |
| **Batch init per-device passwords** | `"password": "generate"` produces unique random password per device; passwords printed once, never stored in plaintext |
| **Config push SSH auth** | Config-push uses SSH public key auth (operator's key in authorized_keys); no password transmitted over SSH |
| **Config push no secret transfer** | Config-push transmits ze.conf only (routing config); no passwords, no TLS keys, no tokens cross the SSH channel |
| **Parallel push isolation** | Each goroutine in parallel push has independent TLS connection, independent update token decryption; no shared mutable state |

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

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Push TLS verifies against stored cert.pem | Device uses self-signed cert generated at init; system CA pool won't trust it; stored cert is the trust anchor |
| Push uses HTTP basic auth (empty user, update token as password) | Matches gokrazy's HTTP update API authentication scheme |
| --all requires passphrase agent (refuses interactive prompt) | Interactive prompt per device is error-prone for batch; ResolvePassphrase(nil) refuses prompt |
| Config preview reuses resolveSeedConfig() from cmd_assemble.go | Same merge logic; no divergence between "what preview shows" and "what assemble builds" |
| Config-push device-side behavior (validation, revert, last-known-good) is in spec-appliance-4-device-config | Separation: this spec owns bastion-side SSH upload; device-side is a different component |
| Batch init manifest supports "password": "generate" | Per-device unique passwords for fleet init; passwords printed once to stdout |
| --parallel N bounded to 1-64, default 1 (sequential) | 1 is safe default; 64 is generous upper bound; real bottleneck is network bandwidth |
| Config-push SSH uses operator's ssh agent, not Ze's passphrase agent | SSH key management is the operator's concern; Ze does not store SSH private keys |
| Push selects most recent image by filename timestamp if --image not given | ze-YYYYMMDD-HHMMSS.img naming convention makes lexicographic sort give chronological order |
| Export push progress as line-per-device status | Immediate feedback; no buffering until all complete |

### OTA Push Protocol

Gokrazy devices expose HTTPS on the update port (default 443):
- Endpoint: `PUT /update`
- Auth: HTTP basic auth, username empty, password is the update token (base64-encoded)
- TLS: self-signed cert generated at init, stored in secrets/tls/cert.pem
- Body: raw disk image
- Success: device reboots to new A/B partition
- A/B fallback: gokrazy handles boot failure automatically (falls back to previous partition)

### Config Push Protocol (Bastion Side)

1. Merge config locally (same as preview)
2. SSH to device at device.address
3. Write merged config to /perm/ze/config-staged.conf
4. Run remote validation: `ze config validate /perm/ze/config-staged.conf`
5. On success: device moves staged to config-pushed.conf, saves old as config-previous.conf
6. On failure: device deletes staged, bastion reports error

Config-push does NOT send any secrets over SSH. Only ze.conf (routing/interface config).

### Parallel Operations

Bounded worker pool pattern:
- `--parallel N` creates N goroutines from a channel of appliance names
- Each goroutine: load config, resolve passphrase (from agent), operate independently
- No shared mutable state between goroutines
- Results printed as each device completes (not buffered)
- Continue on individual failure; summary at end: "N succeeded, M failed"
- Passphrase agent is required (single socket, concurrent reads are safe: each conn gets 32B key)

### Batch Init Manifest Format

JSON array of objects. Each entry maps to an ApplianceConfig with overrides.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Appliance name (passed to init) |
| hostname | string | No | Device hostname (defaults to name) |
| password | string | Yes | SSH password, or "generate" for per-device random |
| device.address | string | No | Device management IP |
| ssh-authorized-keys | string array | No | SSH public keys for operator access |

- "password": "generate" creates a unique random password (printed to stdout, not stored plaintext)
- Missing fields use DefaultConfig(name) defaults
- Each appliance gets independent crypto state (unique salt/nonce)
- Requires ZE_APPLIANCE_SSH_PASSWORD env var OR "password" field per entry

### Image Selection (Push)

When --image is not specified:
1. List files matching `ze-*.img` in AppliancePath(dir, name)
2. Sort by filename (lexicographic = chronological due to timestamp format)
3. Select the last (most recent) entry
4. If no images found, error: "no images found; run `ze appliance build`"

### Scope Boundary

| In this spec (bastion side) | In appliance-4-device-config (device side) |
|----------------------------|--------------------------------------------|
| Push image via HTTPS | Device boots new partition |
| Config-push: SSH upload + remote validate command | Config loading priority (pushed > seed) |
| Config-push: --dry-run preview | Auto-revert on runtime failure |
| Parallel operations | Last-known-good hash verification |
| Batch init | Health check timeout detection |

## Resolved Questions

| # | Question | Answer |
|---|----------|--------|
| Q1 | How does push verify TLS? | Load cert.pem from secrets/tls/ into tls.Config.RootCAs; standard Go TLS verification against that pool |
| Q2 | What SSH library for config-push? | golang.org/x/crypto/ssh; connect to device.address:22 |
| Q3 | Can passphrase agent handle concurrent reads? | Yes. RunAgent accepts connections in a loop; each conn.Write(key) is independent. Safe for parallel --all |
| Q4 | What if batch init has a failure partway through? | Continue remaining entries; report failed entries at end. Already-created appliances are not rolled back. |

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
- [ ] AC-28, AC-39-41, AC-43, AC-46-47, AC-51, AC-54-63 all demonstrated
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
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit
