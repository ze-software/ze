# Spec: service-1-systemd

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/patterns/cli-command.md` - offline command pattern
4. `cmd/ze/init/main.go` - existing setup command for reference
5. `internal/core/paths/paths.go` - config directory resolution

## Task

Add `ze service install` and `ze service uninstall` offline CLI commands that manage
ze as a systemd service on standard Linux hosts. `ze service install` writes a
systemd unit file, enables the service, and optionally starts it. `ze service uninstall`
stops, disables, and removes the unit file. This is for non-gokrazy deployments
where ze runs on a standard Linux distribution.

The command must refuse to run on gokrazy (no systemd) and on non-Linux platforms.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/cli-command.md` - offline command structure
  → Decision: offline commands live in `cmd/ze/<domain>/`, dispatch via switch/map
  → Constraint: register.go with cmdregistry.RegisterRoot + MustRegisterLocal
- [ ] `internal/core/paths/paths.go` - config directory resolution
  → Decision: binary path determines config dir via GNU prefix conventions
  → Constraint: /usr/bin/ze -> /etc/ze, /opt/app/bin/ze -> /opt/app/etc/ze
- [ ] `.claude/rules/cli-grammar.md` - action before identifier
  → Constraint: `ze service install`, not `ze install service`

### RFC Summaries (MUST for protocol work)
None. No protocol work.

**Key insights:**
- ze already resolves its own binary path (`os.Executable()`) and config directory
- Offline commands use `cmd/ze/<domain>/main.go` + `cmd_<sub>.go` pattern
- paths.DefaultConfigDir() and paths.ConfigDirFromBinary() provide path resolution
- gokrazy detection: check for systemctl binary existence

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/init/main.go` - bootstrap command, reference for offline CLI pattern
  → Constraint: uses flag.NewFlagSet, helpfmt.Page, returns exit codes
- [ ] `internal/core/paths/paths.go` - ConfigDirFromBinary, DefaultConfigDir
  → Constraint: resolves config dir from binary location, follows GNU prefix
- [ ] `cmd/ze/main.go` lines 459-488 - `case "start"` dispatches to `cmdStart()`
  → Constraint: `ze start` is the daemon command (not `ze hub`); `cmdStart` resolves zefs, calls `hub.Run()`
  → Constraint: requires `ze init` to have been run first (zefs must exist)
- [ ] `cmd/ze/main.go` - main dispatch, imports all subcommand packages
  → Constraint: new package needs blank import + switch case

**Behavior to preserve:**
- Existing `ze init` workflow unchanged
- Existing `ze hub` startup unchanged
- Config directory resolution logic unchanged

**Behavior to change:**
- New `ze service` subcommand with `install`, `uninstall`, `status` actions

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze service install [flags]` CLI invocation
- `ze service uninstall [flags]` CLI invocation

### Transformation Path
1. `ze service install`: check root, check systemctl exists, resolve binary path, resolve config dir
2. Verify zefs exists in config dir (`<config-dir>/database.zefs`); refuse if missing
3. Create `ze` group and `ze` user if they do not exist (detect groupadd vs addgroup)
4. Generate unit file content from resolved paths
5. Write unit file to `/etc/systemd/system/ze.service`
6. `chown ze:ze <config-dir>` and `chown ze:ze <config-dir>/database.zefs`
7. Run `systemctl daemon-reload`
8. Run `systemctl enable ze.service`
9. Optionally run `systemctl start ze.service` (with `--start` flag)

For uninstall:
1. Check root, check systemctl exists
2. Run `systemctl stop ze.service` (if running)
3. Run `systemctl disable ze.service`
4. Remove `/etc/systemd/system/ze.service`
5. Run `systemctl daemon-reload`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> filesystem | Write unit file to /etc/systemd/system/ | [ ] |
| CLI -> systemctl | exec systemctl commands | [ ] |
| CLI -> user management | exec groupadd/useradd or addgroup/adduser | [ ] |
| CLI -> chown | Change ownership of config dir and zefs | [ ] |
| Daemon -> /run/ze/ | RuntimeDirectory creates socket/pid dir | [ ] |
| Operator CLI -> daemon socket | Must resolve /run/ze/ze.socket | [ ] |

### Integration Points
- `internal/core/paths/paths.go` - reuse ConfigDirFromBinary for config path in unit file
- `os.Executable()` - resolve binary path for ExecStart
- `cmd/ze/main.go` - dispatch to service.Run()

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (standalone offline command)
- [ ] No duplicated functionality (no existing systemd management)
- [ ] Zero-copy preserved where applicable (N/A, not a data path)

## Design Decisions

| Decision | Chosen | Over | Reason |
|----------|--------|------|--------|
| CLI noun | `service` | `install` / `systemd` | "service" is generic (could support launchd later); "install" collides with install-0 umbrella PXE meaning |
| Unit file name | `ze.service` | `ze-hub.service` | The binary is `ze`; users think of the service as "ze" |
| Daemon invocation | `ze start` | `ze -f <path>` | `ze start` resolves config from zefs (standard path). `ze -f` is for filesystem config override, not the systemd case. |
| ExecStart | `ze start` | `ze start --config /etc/ze` | `start` is the daemon subcommand (calls `cmdStart` -> `hub.Run`). Config dir resolved from binary location automatically; explicit flag only if override needed |
| Config flag | `--config` passthrough | Hardcoded path | User may install ze in non-standard prefix; unit file should respect the resolved or overridden config dir |
| Filesystem hardening | ProtectHome=true, RuntimeDirectory=ze only | ProtectSystem=strict | ze writes to /proc/sys (sysctl), manages interfaces via netlink, creates sockets in /var/run; strict blocks all of this. Minimal hardening that does not break functionality. |
| Dry-run flag | `--dry-run` prints unit file | No dry-run | Enables functional testing without root/systemd; useful for operator review before install |
| User/group | Dedicated `ze` user + `ze` group | Run as root | Least privilege; capabilities grant the needed powers without full root |
| Restart policy | `on-failure` | `always` | Crash loops on config error should not restart endlessly |
| After target | `network-online.target` | `network.target` | ze needs working network (BGP, SSH, web); network-online.target waits for link |
| Install WantedBy | `multi-user.target` | `default.target` | Standard for system daemons |
| gokrazy detection | Check for `systemctl` binary | Check /proc/1/cmdline | Simpler, works on any non-systemd system, clear error message |
| Purge on uninstall | No | Yes with config removal | Dangerous; config removal is a separate concern. Uninstall only removes the service. |
| Capabilities | CAP_NET_ADMIN, CAP_NET_RAW, CAP_NET_BIND_SERVICE | Full root | Matches `internal/core/privilege/check_linux.go` requiredCaps. Ambient caps so non-root process inherits. Additional caps commented-out in unit file for operator reference. |
| Socket path | XDG_RUNTIME_DIR=/run/ze in unit | Explicit ze.socket.path env | XDG_RUNTIME_DIR is the standard mechanism; `DefaultSocketPath()` already checks it first. RuntimeDirectory=ze ensures /run/ze/ exists and is owned by ze:ze. |
| PID file in unit | Not set (Type=simple) | PIDFile=/run/ze/ze.pid | systemd ignores PIDFile for Type=simple. ze's own pid file is optional via config. |
| Privilege drop | Handled by systemd User=ze | ze's own dropPrivileges() | systemd sets uid before exec. ze's dropPrivileges() is no-op when not root. Config `daemon { user }` should not be set. |

## Unit File Template

The generated unit file:

| Section | Directive | Value | Source |
|---------|-----------|-------|--------|
| Unit | Description | Ze Network OS | Static |
| Unit | After | network-online.target | Static |
| Unit | Wants | network-online.target | Static |
| Service | Type | simple | Static |
| Service | User | ze | Created by install |
| Service | Group | ze | Created by install |
| Service | ExecStart | `<binary-path> start` | `os.Executable()` resolved to absolute; `start` is the daemon subcommand |
| Service | ExecReload | /bin/kill -HUP $MAINPID | ze handles SIGHUP for config reload |
| Service | Restart | on-failure | Static |
| Service | RestartSec | 5 | Static |
| Service | LimitNOFILE | 65536 | Static (BGP needs many FDs) |
| Service | LimitCORE | infinity | Static (crash debugging) |
| Service | WorkingDirectory | `<config-dir>` | `paths.ConfigDirFromBinary()` or `--config` |
| Service | Environment | `ZE_CONFIG_DIR=<config-dir>` | Config dir for ze |
| Service | Environment | `XDG_RUNTIME_DIR=/run/ze` | Socket + runtime files land in /run/ze/ |
| Service | AmbientCapabilities | CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE | From `privilege/check_linux.go` |
| Service | CapabilityBoundingSet | CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE | Restrict to needed caps only |
| Service | NoNewPrivileges | true | Hardening |
| Service | ProtectHome | true | Hardening |
| Service | RuntimeDirectory | ze | systemd creates /run/ze/ owned by ze:ze |
| Install | WantedBy | multi-user.target | Static |

Where `<binary-path>` is the resolved absolute path of the ze binary,
and `<config-dir>` is resolved via `paths.ConfigDirFromBinary()` or
overridden with `--config`.

## User/Group Creation

`ze service install` creates the `ze` system user and group if they do not exist:

| Step | Action | Condition |
|------|--------|-----------|
| 1 | Create group `ze` (system group) | Group does not exist (`getent group ze` fails) |
| 2 | Create user `ze` (system, no home, nologin shell, gid=ze) | User does not exist (`getent passwd ze` fails) |
| 3 | `chown ze:ze <config-dir>` (directory only, not recursive) | Config dir exists |
| 4 | `chown ze:ze <config-dir>/database.zefs` | zefs file exists |

User/group creation uses Go's `os/exec` to call `groupadd`/`useradd` (Debian/RHEL)
or `addgroup`/`adduser` (Alpine/BusyBox). Detection: check which binary exists
in PATH. The nologin shell path is detected at runtime (`/usr/sbin/nologin` or
`/sbin/nologin`, whichever exists).

The user is a system account with no home directory and no login shell.
`ze service uninstall` does NOT remove the user/group (data ownership concern).

**Prerequisite:** `ze init` must have been run before `ze service install`.
Install verifies zefs exists and refuses with a clear error if not.
Install does NOT create the config directory or run `ze init`. The sequence is:
1. Install ze binary (package manager or manual)
2. `sudo ze init` (creates /etc/ze/ and database.zefs)
3. `sudo ze service install` (creates user, unit file, chowns config dir)
4. Edit config via `ze config edit` or SSH

**Existing user:** If `ze` user already exists, install prints its uid/gid and
continues without modification. No attempt to change shell, home, or groups
on an existing user.

## Runtime Path and Socket Interactions

The unit file sets `XDG_RUNTIME_DIR=/run/ze` combined with `RuntimeDirectory=ze`.
This is critical for three reasons:

| Component | Without XDG_RUNTIME_DIR | With XDG_RUNTIME_DIR=/run/ze |
|-----------|------------------------|------------------------------|
| Unix socket (`DefaultSocketPath()`) | Falls to `/tmp/ze.socket` (uid != 0) | `/run/ze/ze.socket` |
| PID file (`writePIDFile()`) | Needs explicit `ze.pid.file` env var | Can use `/run/ze/ze.pid` via config |
| CLI client connection | Must match daemon socket path | Operator sets `XDG_RUNTIME_DIR=/run/ze` or `ze.socket.path` |

**CLI client access:** The `ze` CLI (e.g. `ze show bgp peer *`) connects to the
daemon's Unix socket. When the daemon runs as the `ze` user with
`XDG_RUNTIME_DIR=/run/ze`, the socket is at `/run/ze/ze.socket`. The CLI resolves
the same path via `DefaultSocketPath()`. For the operator's shell, the CLI
needs to know where the socket is. Options (in priority order):
1. Config file `daemon { socket "/run/ze/ze.socket"; }` (recommended)
2. `export XDG_RUNTIME_DIR=/run/ze` in the operator's shell
3. `ze.socket.path=/run/ze/ze.socket` env var

The install command prints a hint about socket access after successful install.

**Config editing:** `ze config edit` opens the same `database.zefs` file. After
install chowns it to `ze:ze`, non-root users cannot write to it. Operators
must use `sudo ze config edit` (consistent with `ze init` also requiring root).
This is the same privilege model as editing `/etc/` configs on any Linux system.

**resolv.conf:** If `system { dns { resolv-conf-path } }` points to a path
like `/etc/resolv.conf`, the `ze` user cannot write it. The default
(`/tmp/resolv.conf`) works. Operators using a custom path must ensure the `ze`
user has write access (e.g., set the path to `/run/ze/resolv.conf`).

**Reload:** The unit file includes `ExecReload=/bin/kill -HUP $MAINPID`,
enabling `systemctl reload ze` which triggers ze's existing SIGHUP handler
for hot config reload. `ze service restart` / `ze service reload` CLI wrappers
are deferred (operators use `systemctl` directly).

**Stopping before re-init:** If the service is running, `ze init --force` detects
the daemon and refuses. Stop the service first: `sudo ze service uninstall` or
`sudo systemctl stop ze` before running `ze init --force`.

**PID file:** `Type=simple` does not use `PIDFile=`. systemd tracks the main
process directly. ze's own `writePIDFile()` is optional and governed by the
`ze.pid.file` env var or config `daemon { pid }`. The unit file does not set
`ze.pid.file`; operators who want a PID file add it to their ze config.

**Privilege drop interaction:** ze has its own `dropPrivileges()` mechanism
via `ze.user`/`ze.group` env vars (set from config `daemon { user }`). With
systemd `User=ze`, the process starts as `ze` directly. `dropPrivileges()`
is a no-op when not root (line 42 of `drop_unix.go`). The config SHOULD NOT
set `daemon { user }` when running under systemd, because:
- If `daemon { user "ze"; }`: no-op (already that user), harmless but redundant
- If `daemon { user "nobody"; }`: fails (cannot setuid without root), daemon refuses to start

The install command prints a warning if the existing config contains `daemon { user }`.

## Capability Coverage

The three capabilities match `internal/core/privilege/check_linux.go` and cover
the core network OS functions:

| Capability | Used by |
|------------|---------|
| CAP_NET_ADMIN | Netlink (interface config, FIB), sysctl writes to `/proc/sys/net/`, conntrack |
| CAP_NET_RAW | Ping, traceroute, raw sockets |
| CAP_NET_BIND_SERVICE | BGP port 179, SSH port 22 if configured below 1024 |

**Known gaps (document in unit file as comments):**

| Feature | May need | When | Failure mode |
|---------|----------|------|--------------|
| Host tuning (CPU governor, IRQ affinity) | CAP_SYS_NICE | If `system { tuning { } }` is configured | Warning logged, startup continues |
| VRF / network namespaces | CAP_SYS_ADMIN | When VRF support is implemented | Error at feature use |
| PPPoE server (raw ethernet) | Already covered by CAP_NET_RAW | Current | N/A |
| RPS/RFS offload (sysfs writes) | CAP_NET_ADMIN (included) | If `iface { offload { } }` is configured | Warning logged |

The generated unit file includes commented-out lines for these additional
capabilities so operators can uncomment as needed.

## CLI Surface

All commands except `--dry-run` and `status` require root. The command checks
`os.Getuid() == 0` and refuses with `error: must be run as root` if not.

| Command | Purpose | Root? |
|---------|---------|-------|
| `ze service install [--config <dir>] [--start] [--force]` | Create ze user/group, write unit file, enable service | Yes |
| `ze service install --dry-run [--config <dir>]` | Print unit file to stdout (no writes) | No |
| `ze service uninstall` | Stop, disable, remove unit file | Yes |
| `ze service status` | Show systemctl status output | No |

| Flag | Default | Purpose |
|------|---------|---------|
| `--config` | auto-resolved | Override config directory in the unit file |
| `--start` | false | Start the service immediately after install |
| `--force` | false | Overwrite existing unit file |
| `--dry-run` | false | Print unit file to stdout without writing, creating user, or calling systemctl |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze service install` CLI | -> | `service.cmdInstall()` writes unit + enables | `TestServiceInstallGeneratesUnit` |
| `ze service uninstall` CLI | -> | `service.cmdUninstall()` removes unit + disables | `TestServiceUninstallRemovesUnit` |
| `ze service status` CLI | -> | `service.cmdStatus()` runs systemctl status | `TestServiceStatusRuns` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze service install` on Linux with systemd | Unit file written to `/etc/systemd/system/ze.service`, `systemctl enable` succeeds |
| AC-2 | `ze service install --start` | Service installed and started (`systemctl start` runs) |
| AC-3 | `ze service uninstall` with installed service | Service stopped, disabled, unit file removed, `daemon-reload` run |
| AC-4 | `ze service install` on non-Linux or no systemctl | Command refuses with clear error message, exit code 1 |
| AC-5 | `ze service status` with installed service | Shows systemctl status output |
| AC-6 | `ze service install --config /custom/path` | Unit file contains `WorkingDirectory=/custom/path` and `ZE_CONFIG_DIR=/custom/path` |
| AC-7 | `ze service install` when unit file already exists | Refuses with error "service already installed" unless `--force` flag |
| AC-8 | Generated unit file content | Contains correct ExecStart with absolute binary path, correct After/Wants/WantedBy |
| AC-9 | `ze service install` when `ze` user/group do not exist | Creates system user `ze` with group `ze`, no home, nologin shell |
| AC-10 | Generated unit file security | Contains User=ze, Group=ze, AmbientCapabilities, CapabilityBoundingSet, NoNewPrivileges=true |
| AC-11 | `ze service install` config dir ownership | Config directory and zefs file chowned to ze:ze (not recursive) |
| AC-12 | `ze service install` without prior `ze init` | Refuses with error "ze init has not been run" (zefs not found) |
| AC-13 | Generated unit file runtime paths | Contains `XDG_RUNTIME_DIR=/run/ze` and `RuntimeDirectory=ze` |
| AC-14 | Generated unit file has no PIDFile directive | Type=simple, no PIDFile line |
| AC-15 | `ze service install` prints socket access hint | Output includes how to connect CLI to daemon socket |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnitFileContent` | `cmd/ze/service/unit_test.go` | Generated unit file has correct sections and values | |
| `TestUnitFileCustomConfig` | `cmd/ze/service/unit_test.go` | `--config` override appears in unit file | |
| `TestUnitFilePrerequisite` | `cmd/ze/service/unit_test.go` | Install refuses when zefs does not exist | |
| `TestServiceInstallGeneratesUnit` | `cmd/ze/service/service_test.go` | Install writes file to expected path | |
| `TestServiceUninstallRemovesUnit` | `cmd/ze/service/service_test.go` | Uninstall removes file from expected path | |
| `TestServiceStatusRuns` | `cmd/ze/service/service_test.go` | Status subcommand dispatches correctly | |
| `TestServiceRefusesNonLinux` | `cmd/ze/service/service_test.go` | Non-Linux returns error | |
| `TestUnitFileCapabilities` | `cmd/ze/service/unit_test.go` | Unit file contains AmbientCapabilities, CapabilityBoundingSet, User=ze, Group=ze | |
| `TestUnitFileHardening` | `cmd/ze/service/unit_test.go` | Unit file contains NoNewPrivileges, ProtectHome, RuntimeDirectory | |
| `TestUnitFileRuntimeDir` | `cmd/ze/service/unit_test.go` | Unit file contains XDG_RUNTIME_DIR=/run/ze, RuntimeDirectory=ze | |
| `TestUnitFileNoPIDFile` | `cmd/ze/service/unit_test.go` | Unit file does NOT contain PIDFile directive | |
| `TestInstallPrintsSocketHint` | `cmd/ze/service/service_test.go` | Install output includes socket access instructions | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

No user-supplied numeric inputs. RestartSec and LimitNOFILE are static.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-service-unit-gen` | `test/service/unit-gen.ci` | `ze service install --dry-run` prints correct unit file content to stdout | |

Note: actual systemd integration (real systemctl, real user creation) requires
QEMU with systemd. The functional test uses `--dry-run` which generates and prints
the unit file without writing or calling systemctl. This validates the end-to-end
path from CLI args through path resolution to unit file output.

### Future (if deferring any tests)
- Integration test requiring actual systemd (needs QEMU): deferred, unit tests mock systemctl
- macOS launchd support: deferred, Linux-only for v1
- Multi-instance support (`--name` flag): deferred, no current use case
- CAP_SYS_ADMIN for VRF/netns: add when VRF support is implemented

## Files to Modify
- `cmd/ze/main.go` - add `case "service"` dispatch + import

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (offline command) |
| CLI commands/flags | Yes | `cmd/ze/service/register.go` |
| Editor autocomplete | No | N/A (offline command) |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - systemd service management |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - ze service |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/installation.md` - systemd setup section |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create
- `cmd/ze/service/main.go` - Run() + dispatch (install/uninstall/status)
- `cmd/ze/service/register.go` - cmdregistry registration
- `cmd/ze/service/cmd_install.go` - install logic: generate unit, write, enable
- `cmd/ze/service/cmd_uninstall.go` - uninstall logic: stop, disable, remove
- `cmd/ze/service/cmd_status.go` - status wrapper for systemctl status
- `cmd/ze/service/unit.go` - unit file generation (template + path resolution)
- `cmd/ze/service/unit_test.go` - unit file content tests
- `cmd/ze/service/service_test.go` - command dispatch tests
- `cmd/ze/service/detect_linux.go` - systemd detection (build-tagged linux)
- `cmd/ze/service/detect_other.go` - non-linux stub returning error
- `test/service/unit-gen.ci` - functional test (dry-run unit file generation)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-13. Fix/verify loop | Per phase |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register command, create stubs
   - Tests: `TestServiceInstallGeneratesUnit`, `TestServiceUninstallRemovesUnit`, `TestServiceStatusRuns`
   - Files: `cmd/ze/service/register.go`, `cmd/ze/service/main.go`, `cmd/ze/main.go`
   - Verify: `ze service install` dispatches to stub, returns "not implemented"

2. **Phase: Unit file generation** -- template + path resolution
   - Tests: `TestUnitFileContent`, `TestUnitFileCustomConfig`, `TestUnitFileCustomName`
   - Files: `cmd/ze/service/unit.go`, `cmd/ze/service/unit_test.go`
   - Verify: unit file generated with correct content for various binary paths

3. **Phase: Install command** -- write file, call systemctl
   - Tests: `TestServiceInstallGeneratesUnit`
   - Files: `cmd/ze/service/cmd_install.go`, `cmd/ze/service/detect_linux.go`, `cmd/ze/service/detect_other.go`
   - Verify: install writes unit file, calls systemctl enable

4. **Phase: Uninstall + status** -- stop/disable/remove, status wrapper
   - Tests: `TestServiceUninstallRemovesUnit`, `TestServiceStatusRuns`
   - Files: `cmd/ze/service/cmd_uninstall.go`, `cmd/ze/service/cmd_status.go`
   - Verify: uninstall removes unit, status shows output

5. **Functional tests** -- create after feature works
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Unit file content matches systemd expectations (valid sections, correct directives) |
| Naming | Subcommand is `service`, actions are `install`/`uninstall`/`status` |
| Data flow | Binary path resolution -> unit file content -> filesystem write -> systemctl |
| Rule: cli-grammar | Action (`install`/`uninstall`/`status`) before any identifier |
| Rule: registration | register.go with RegisterRoot + MustRegisterLocal |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/service/` package exists | `ls cmd/ze/service/` |
| `ze service install` dispatches | `go build ./cmd/ze/ && ./ze service install --help` |
| Unit file generation correct | `go test ./cmd/ze/service/ -run TestUnitFile` |
| Build-tagged for linux/other | `grep -l 'go:build' cmd/ze/service/detect_*.go` |
| Register.go exists | `ls cmd/ze/service/register.go` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | `--config` path must be absolute and exist; zefs must exist in resolved config dir |
| Privilege escalation | Install command requires root (writes /etc/systemd/system/, creates user). Verify and refuse if not root. Service itself runs as `ze` user with minimal capabilities. |
| Path traversal | Unit name must not contain `/` or `..` |
| Command injection | systemctl arguments must not be user-controlled beyond the unit name (validated) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## RFC Documentation

N/A. No protocol work.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`cmd/ze/service/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

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
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
