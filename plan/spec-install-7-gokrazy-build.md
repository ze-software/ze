# Spec: install-7-gokrazy-build

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-install-0-umbrella |
| Phase | - |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/appliance/main.go` - existing appliance CLI dispatch
4. `cmd/ze/appliance/cmd_build.go` - existing build (shells out to gok)
5. `cmd/ze/appliance/cmd_push.go` - existing OTA push
6. `cmd/ze/install/main.go` - existing install CLI dispatch
7. `plan/learned/675-appliance-1-builder.md` - appliance builder decisions
8. `plan/learned/677-appliance-2-remote.md` - remote operations decisions

## Task

Unify ze's installation and appliance management under `ze install`, and remove the
external dependency on the `gok` binary by vendoring the gokrazy builder and updater
libraries. After this work, `ze` is fully self-contained for the provisioning lifecycle:
build image, PXE-boot target, push updates, manage fleet. No external tooling required.

Currently:
- `ze install local` installs ze binary + systemd unit on Linux
- `ze install remote` starts PXE provisioning servers
- `ze appliance` manages gokrazy fleet (init, build, push, config, export/import, etc.)
- `ze appliance build` shells out to external `bin/gok` binary
- `ze appliance push` implements OTA update with raw HTTP PUT

After:
- `ze install local` (unchanged)
- `ze install remote` (unchanged)
- `ze install appliance init/build/push/...` (moved from `ze appliance`)
- `ze install appliance build` uses vendored gokrazy builder (no external `gok`)
- `ze install appliance push` uses vendored gokrazy updater (cleaner protocol handling)
- `ze appliance` kept as deprecated alias pointing to `ze install appliance`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - section 20: Appliance Config Loading Priority
  -> Decision: pushed config at /perm/ze/config-pushed.conf, validation via config.LoadConfig
  -> Constraint: ZeFS is read-only after build; persistent state on /perm
- [ ] `docs/guide/appliance.md` - user-facing appliance documentation
  -> Constraint: document all CLI changes, keep examples working

### Learned Summaries
- [ ] `plan/learned/675-appliance-1-builder.md` - builder decisions
  -> Decision: kebab-case JSON config, config-base validation at assemble time
  -> Constraint: gok invocation + ext4 inject currently uses external binaries
- [ ] `plan/learned/677-appliance-2-remote.md` - remote operations decisions
  -> Decision: push uses HTTP basic auth (empty user, token as password), TLS cert pinning
  -> Constraint: sshExecFunc is a replaceable function variable for testability
- [ ] `plan/learned/676-appliance-3-recovery.md` - export/import decisions
  -> Decision: tar-in-memory then encrypt atomically, archives always encrypted
- [ ] `plan/learned/678-appliance-4-device-config.md` - device config decisions
  -> Decision: pushed config priority, LKG hash, 30s health revert window
- [ ] `plan/learned/578-gokrazy-3-build.md` - gokrazy build config
  -> Decision: explicit GokrazyPackages (only randomd + heartbeat), ze owns DHCP/NTP
- [ ] `plan/learned/769-install-subcommand.md` - install subcommand decisions
  -> Decision: fork pattern (config gen + fork `ze -`), NUL sentinel

### Source Files
- [ ] `cmd/ze/appliance/main.go` - 16 subcommands dispatch, baseDir resolution, usage
  -> Constraint: handlers map + extractDirFlag pattern must be preserved
- [ ] `cmd/ze/appliance/cmd_build.go` - gok shell-out, ext4 inject, GPT parsing
  -> Constraint: findLastPartition() GPT reader, injectZeFS() ext4 operations reusable
  -> Decision: runExternalFn is a replaceable function variable (testability)
- [ ] `cmd/ze/appliance/cmd_push.go` - HTTP PUT to gokrazy update endpoint
  -> Constraint: TLS cert pinning via stored cert.pem, protocolError type
- [ ] `cmd/ze/appliance/cmd_run.go` - QEMU launch with port conflict detection
  -> Constraint: buildQEMUCommand handles ARM64 vs AMD64, accelerator selection
- [ ] `cmd/ze/appliance/config.go` - ApplianceConfig struct, validation, load/save
  -> Constraint: kebab-case JSON tags throughout
- [ ] `cmd/ze/appliance/crypto.go` - Argon2id + XChaCha20-Poly1305 encryption
  -> Constraint: key zeroing, atomic file writes
- [ ] `cmd/ze/install/main.go` - local/remote dispatch
  -> Decision: Run(args) pattern, helpfmt.Page usage
- [ ] `cmd/ze/install/serve.go` - config gen + fork pattern for PXE
- [ ] `cmd/ze/appliance/register.go` - subcommand registration in main.go dispatch

**Key insights:**
- The appliance subsystem is mature (38 files, 4 completed specs, comprehensive tests)
- gok binary is the ONLY external dependency for image building
- The gokrazy update protocol is simple (HTTP PUT with basic auth)
- All appliance subcommands follow the same pattern: flag parse, load config, resolve passphrase, act
- The ext4 injection (mkfs.ext4 + debugfs) is also an external dependency worth addressing

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/appliance/` (38 files) - full fleet management CLI
  -> Constraint: all existing functionality must be preserved
- [ ] `cmd/ze/install/` (9 files) - local install + PXE provisioning
  -> Constraint: local and remote subcommands unchanged
- [ ] `cmd/ze/main.go` - dispatches both "appliance" and "install" subcommands
  -> Constraint: must update dispatch for nested structure

**Behavior to preserve:**
- All 16 appliance subcommands work identically under new namespace
- Encrypted secrets, passphrase agent, TLS cert management unchanged
- ApplianceConfig JSON format unchanged (no migration needed)
- All existing tests pass under new package structure
- PXE provisioning (ze install remote) unchanged
- Local install (ze install local) unchanged
- Device-side config loading (pushed config, health revert) unchanged

**Behavior to change:**
- `ze appliance <cmd>` becomes `ze install appliance <cmd>`
- `ze appliance` kept as deprecated alias (prints deprecation warning, delegates)
- `ze appliance build` no longer requires external `bin/gok` binary
- `ze appliance push` uses vendored gokrazy updater library (cleaner, testable)
- `cmd/ze/appliance/` package moves to `cmd/ze/install/appliance/`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze install appliance <subcommand>` CLI invocation
- `ze appliance <subcommand>` (deprecated alias, same dispatch)

### Transformation Path
1. `cmd/ze/main.go` dispatches "install" to `install.Run()`
2. `install.Run()` dispatches "appliance" to `appliance.Run()`
3. `appliance.Run()` dispatches to specific handler (build, push, etc.)
4. For build: vendored gokrazy builder produces disk image
5. For push: vendored gokrazy updater pushes to device

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> install package | `install.Run(args)` | [ ] |
| install -> appliance package | `appliance.Run(args[1:])` | [ ] |
| appliance -> gokrazy builder library | Go API call (vendored) | [ ] |
| appliance -> gokrazy updater library | Go API call (vendored) | [ ] |
| appliance -> filesystem | ext4 for /perm injection | [ ] |

### Integration Points
- `cmd/ze/main.go` dispatch table: add "appliance" delegation through "install"
- `cmd/ze/install/main.go`: add "appliance" case in switch
- `cmd/ze/install/appliance/`: moved package with all existing code
- gokrazy builder library: replaces `runExternalFn(gokBinary, ...)` calls
- gokrazy updater library: replaces raw `doPush()` HTTP implementation

### Architectural Verification
- [ ] No bypassed layers (appliance accessed through install, not directly)
- [ ] No unintended coupling (appliance package self-contained, only needs gokrazy libs)
- [ ] No duplicated functionality (deprecated alias delegates, does not copy)
- [ ] Zero-copy preserved where applicable (image streaming for push)

## Child Specs

| Spec | Scope | Depends |
|------|-------|---------|
| `spec-install-7a-namespace.md` | Move `cmd/ze/appliance/` to `cmd/ze/install/appliance/`. Update CLI dispatch. Add deprecated `ze appliance` alias. Update all imports, tests, docs. | - |
| `spec-install-7b-vendor-builder.md` | Vendor `github.com/gokrazy/tools` builder library. Replace `gok` shell-out in `cmd_build.go` with Go API calls. Remove `bin/gok` dependency. Handle cross-compilation and partition table creation. | 7a |
| `spec-install-7c-vendor-updater.md` | Vendor `github.com/gokrazy/updater` library. Replace raw HTTP PUT in `cmd_push.go` with updater API. Cleaner A/B root update, progress reporting, protocol version handling. | 7a |

## Component 1 (spec-install-7a): Namespace Migration

### What Changes

Move `cmd/ze/appliance/` package to `cmd/ze/install/appliance/`. This is primarily a refactoring task:

1. Move all 38 files from `cmd/ze/appliance/` to `cmd/ze/install/appliance/`
2. Update package declaration from `package appliance` (can keep same name)
3. Update all import paths: `codeberg.org/.../cmd/ze/appliance` -> `codeberg.org/.../cmd/ze/install/appliance`
4. Update `cmd/ze/main.go` dispatch: remove direct "appliance" case, let it route through "install"
5. Update `cmd/ze/install/main.go`: add "appliance" case that delegates to `appliance.Run()`
6. Add deprecated alias: `ze appliance` prints warning then delegates to `ze install appliance`
7. Update `cmd/ze/appliance/register.go` -> `cmd/ze/install/appliance/register.go`
8. Update all test files, documentation, and `// Design:` annotations
9. Update `docs/guide/appliance.md` with new command paths

### Deprecated Alias

In `cmd/ze/main.go`, the "appliance" case becomes:

Print to stderr: `warning: "ze appliance" is deprecated, use "ze install appliance"`
Then delegate to `install.Run(append([]string{"appliance"}, args...))`.

### Files

All files in `cmd/ze/appliance/` move to `cmd/ze/install/appliance/`. Import paths update throughout.

## Component 2 (spec-install-7b): Vendor Gokrazy Builder

### Current Build Process

`cmd_build.go:runGokBuild()` shells out to `bin/gok` with:
- `--parent_dir gokrazy` (gokrazy config directory)
- `-i ze` (instance name)
- `overwrite --full <imgPath>` (full disk image output)
- `--target_storage_bytes <size>` (image size)

After gok produces the image, `injectZeFS()` uses external `mkfs.ext4` and `debugfs` to:
1. Find the last GPT partition (findLastPartition, pure Go)
2. Format it as ext4 (shells out to mkfs.ext4)
3. Extract the partition, inject ze/database.zefs via debugfs
4. Write the modified partition back

### Vendored Builder (API Research Complete)

The `github.com/gokrazy/tools/gok` package exposes a minimal but sufficient API:

    type Context struct {
        Stdin  io.Reader
        Stdout io.Writer
        Stderr io.Writer
        Args   []string
    }
    func (c Context) Execute(ctx context.Context) error

This is a library wrapper around the gok CLI. You pass CLI args and it runs the
full gok pipeline internally (cross-compilation, partition table, firmware, A/B root,
ext4 /perm creation). It does NOT expose low-level builder primitives.

Replace `runExternalFn(gokBinary, ...)` with:

    gokCtx := gok.Context{
        Stdout: os.Stdout,
        Stderr: os.Stderr,
        Args:   []string{"--parent_dir", "gokrazy", "-i", "ze", "overwrite", "--full", imgPath, "--target_storage_bytes", sizeStr},
    }
    err := gokCtx.Execute(ctx)

This eliminates the `bin/gok` external binary dependency. The gokrazy config
directory (`gokrazy/ze/config.json`) is still needed as input.

**e2fsprogs dependency remains**: The gok Execute() handles image creation but
the ZeFS injection into /perm still needs mkfs.ext4 + debugfs (the `injectZeFS()`
function). This external dependency can be addressed separately with a pure-Go
ext4 writer in a future spec.

### Risk

The gok.Context API is thin (just Args + Execute), so API stability risk is low.
The risk is in gok's internal behavior changing between versions. Pin to a specific
version in go.mod.

## Component 3 (spec-install-7c): Vendor Gokrazy Updater

### Current Push Process

`cmd_push.go:doPush()` manually constructs an HTTP PUT request:
- Opens image file
- Creates `http.Request` with `PUT` method to `https://<device>/update`
- Sets basic auth (empty user, update token as password)
- Custom TLS config with cert pinning
- Checks response status (401 = auth error, 200 = success)

### Vendored Updater (API Research Complete)

The `github.com/gokrazy/updater` package exposes a rich API:

    func NewTarget(ctx context.Context, baseURL string, httpClient HTTPDoer) (*Target, error)

    // Target methods:
    func (t *Target) StreamTo(ctx context.Context, dest string, r io.Reader) error
    func (t *Target) Put(ctx context.Context, dest string, r io.Reader) error
    func (t *Target) Switch(ctx context.Context) error
    func (t *Target) Reboot(ctx context.Context) error
    func (t *Target) RebootWithoutKexec(ctx context.Context) error
    func (t *Target) Testboot(ctx context.Context) error
    func (t *Target) Divert(ctx context.Context, path, diversion string, serviceFlags, commandLineFlags []string) error
    func (t *Target) Supports(feature ProtocolFeature) bool
    func (t *Target) InstalledEEPROM() EEPROMVersion

    // HTTPDoer interface (satisfied by *http.Client):
    type HTTPDoer interface { Do(*http.Request) (*http.Response, error) }

    // Destinations for StreamTo: "mbr", "root", "boot", "bootonly"

Replace `doPush()` with:

    httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
    target, err := updater.NewTarget(ctx, baseURL, httpClient)
    // Stream root filesystem
    target.StreamTo(ctx, "root", rootReader)
    // Stream boot partition
    target.StreamTo(ctx, "boot", bootReader)
    // Switch A/B and reboot
    target.Switch(ctx)
    target.Reboot(ctx)

The existing TLS cert pinning (`loadDeviceTLS()`) is preserved by passing the
custom `*http.Client` with pinned TLS config as the `HTTPDoer`.

Benefits over current raw HTTP PUT:
- Proper A/B root partition handling (current code just PUTs the whole image)
- Protocol feature detection (`Supports()`)
- Testboot support (boot new version, auto-revert if it fails)
- EEPROM version checking for RPi

## Design Decisions

| Decision | Chosen | Over | Reason |
|----------|--------|------|--------|
| Nested CLI (`ze install appliance`) | Nest under install | Flat (all at `ze install` level) | Keeps 16 fleet management commands grouped, avoids crowding install namespace |
| Deprecated alias | `ze appliance` warns + delegates | Hard break (remove immediately) | Graceful migration, existing scripts keep working |
| Vendor gokrazy builder | Go library import | Keep shelling out to gok | Single binary philosophy, no external dependency |
| Vendor gokrazy updater | Go library import | Keep raw HTTP PUT | Protocol compatibility as gokrazy evolves |
| Child specs | 3 children (namespace, builder, updater) | Single spec | Incremental implementation, each independently testable |
| Package location | `cmd/ze/install/appliance/` | New package name | Preserves `package appliance` name, just moves location |

## Scope Boundaries (v1)

| Limitation | v1 behavior | Future |
|------------|-------------|--------|
| ext4 tools dependency | e2fsprogs (mkfs.ext4, debugfs) still needed for ZeFS injection into /perm | Pure Go ext4 writer |
| gokrazy config directory | `gokrazy/ze/config.json` still needed as input to gok.Execute() | Generate config programmatically |
| gokrazy builder API | gok.Context.Execute() is args-based (not structured). Pin version in go.mod. | Track upstream for richer API |
| Cross-compilation | Requires Go toolchain on the build machine (same as current) | N/A |

## Wiring Test (MANDATORY, NOT deferrable)

Umbrella spec. Wiring tests defined in child specs.

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze install appliance build <name>` | -> | vendored gokrazy builder produces image | `spec-install-7b` |
| `ze install appliance push <name>` | -> | vendored gokrazy updater pushes to device | `spec-install-7c` |
| `ze appliance build <name>` (deprecated) | -> | delegates to `ze install appliance build` | `spec-install-7a` |
| `ze install appliance` (all subcommands) | -> | same behavior as `ze appliance` | `spec-install-7a` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze install appliance --help` | Shows all 16 subcommands with descriptions |
| AC-2 | `ze install appliance init lab` | Creates appliance config identical to current `ze appliance init lab` |
| AC-3 | `ze install appliance build lab` | Builds gokrazy image WITHOUT external `gok` binary |
| AC-4 | `ze install appliance push lab` | OTA update using vendored gokrazy updater library |
| AC-5 | `ze appliance build lab` | Prints deprecation warning, then delegates to `ze install appliance build` |
| AC-6 | All existing appliance tests | Pass under new package location with zero behavior change |
| AC-7 | `ze install --help` | Shows local, remote, and appliance as subcommands |
| AC-8 | `ze install appliance build` with no `bin/gok` present | Succeeds (no external dependency) |
| AC-9 | Image produced by vendored builder | Identical partition layout to gok-produced image (firmware, root A/B, /perm) |
| AC-10 | ZeFS injection into /perm | SSH credentials accessible after boot |

## TDD Test Plan

### Unit Tests

Umbrella spec. Detailed test plans in child specs.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInstallApplianceDispatch` | `cmd/ze/install/appliance/main_test.go` | All 16 subcommands dispatch correctly | |
| `TestDeprecatedAlias` | `cmd/ze/main_test.go` | `ze appliance` warns and delegates | |
| `TestVendoredBuild` | `cmd/ze/install/appliance/cmd_build_test.go` | Image built without gok binary | |
| `TestVendoredPush` | `cmd/ze/install/appliance/cmd_push_test.go` | Push uses updater library | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-install-appliance-help` | `test/install/appliance-help.ci` | `ze install appliance --help` shows all commands | |
| `test-install-appliance-deprecated` | `test/install/appliance-deprecated.ci` | `ze appliance` prints deprecation warning | |

## Files to Modify

Umbrella spec. Detailed file lists in child specs.

- `cmd/ze/main.go` - update dispatch for deprecated alias
- `cmd/ze/install/main.go` - add appliance case
- `cmd/ze/appliance/` -> `cmd/ze/install/appliance/` (move all 38 files)
- `cmd/ze/appliance/cmd_build.go` - replace gok shell-out with vendored builder
- `cmd/ze/appliance/cmd_push.go` - replace HTTP PUT with vendored updater
- `docs/guide/appliance.md` - update command paths
- `go.mod` - add gokrazy/tools and gokrazy/updater dependencies

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | Yes | `cmd/ze/install/main.go`, `cmd/ze/main.go` |
| CLI grammar | Yes | action before identifier maintained |
| Go module dependencies | Yes | `go.mod` (gokrazy/tools, gokrazy/updater) |
| Subcommand registration | Yes | `cmd/ze/install/appliance/register.go` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Namespace change, not new feature |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - new namespace |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - update all command examples |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - section 20 command paths |

## Files to Create

- `cmd/ze/install/appliance/` - moved package (not technically new, but new location)
- `test/install/appliance-help.ci` - help output test
- `test/install/appliance-deprecated.ci` - deprecated alias test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + child specs |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table per child spec |
| 4. Implement | Child specs in order: 7a, 7b, 7c |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist |

### Implementation Phases

1. **Phase: Namespace migration (spec-install-7a)** - move package, update imports, add alias
2. **Phase: Vendor builder (spec-install-7b)** - replace gok shell-out
3. **Phase: Vendor updater (spec-install-7c)** - replace raw HTTP push

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 16 appliance subcommands work under new namespace |
| Correctness | Deprecated alias prints warning AND delegates correctly |
| Naming | `ze install appliance` not `ze install app` or other abbreviations |
| Data flow | CLI dispatch chain: main -> install -> appliance -> handler |
| Backward compat | `ze appliance` still works (with warning) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `ze install appliance -h` prints usage | Run command, check output |
| `ze appliance` prints deprecation | Run command, check stderr |
| Image builds without `bin/gok` | Remove gok, run build, verify success |
| Push uses vendored updater | grep for gokrazy/updater import |
| All existing tests pass | `make ze-unit-test` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Credential handling | Passphrase agent, encrypted secrets unchanged |
| TLS cert pinning | Push still validates against stored cert.pem |
| Path traversal | Import still validates paths |
| Dependency audit | gokrazy/tools and gokrazy/updater reviewed for supply chain risk |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import path errors | Fix in namespace migration (7a) |
| gokrazy builder API mismatch | Research actual API, update spec-install-7b |
| gokrazy updater API mismatch | Research actual API, update spec-install-7c |
| Tests fail after move | Fix import paths, verify no behavior change |

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

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Nest appliance under install | Flat (all subcommands at install level), keep separate | User chose unification. Nesting keeps fleet management grouped. |
| Deprecated alias | Hard break, silent redirect | Graceful migration for existing scripts, visible deprecation. |
| Vendor over shell-out | Keep gok dependency, embed gok as go:embed | Single binary, no external tools. go:embed doesn't help (gok is a compiler). |

## Known Limitations

- e2fsprogs (mkfs.ext4, debugfs) still needed for ZeFS injection into /perm partition after gok builds the image
- gok.Context.Execute() is args-based (string args, not structured API); changes in gok CLI flags could break
- gokrazy config directory (`gokrazy/ze/config.json`) still needed as input; not yet generated programmatically
- Cross-compilation requires Go toolchain on the build machine (same as current)

## RFC Documentation

N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [Umbrella: see child specs]

### Bugs Found/Fixed
- [None yet]

### Documentation Updates
- [None yet]

### Deviations from Plan
- [None yet]

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
