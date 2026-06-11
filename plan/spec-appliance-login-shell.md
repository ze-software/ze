# Spec: appliance-login-shell

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-06-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/guide/appliance.md` - appliance architecture
4. `cmd/ze/hub/main_servers.go:594-622` - usersFromZefsDB and local admin auth
5. `gokrazy/modcache/github.com/gokrazy/gokrazy@.../gokrazy.go:358-378` - tryStartShell
6. `gokrazy/modcache/github.com/gokrazy/serial-busybox@.../wrapper.go` - current symlink setup
7. `cmd/ze/ze_core_dispatch.go` - ze personality dispatch (argv[0] check lives here)
8. `cmd/ze/login.go` - login handler

## Task

Gate serial console access on the gokrazy appliance behind authentication.
Currently, serial-busybox provides an unauthenticated busybox shell via
`/tmp/serial-busybox/ash`. Replace the login path so ze authenticates the
operator (username + password against ZeFS local admin credentials) before
granting shell access. The busybox binary is renamed to a non-obvious name
so it cannot be run directly; only ze knows the path and execs it after auth.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/appliance.md` - gokrazy appliance architecture, serial console role
  -> Constraint: serial-busybox is listed in gokrazy/ze/config.json Packages; busybox binary deployed via extrafiles tarballs to /usr/local/bin/busybox
  -> Decision: gokrazy config.json SerialConsole set to "ttyS0,115200"; serial console is emergency-only access
  -> Constraint: root filesystem is read-only SquashFS; binary rename must happen at build time in extrafiles
- [ ] `docs/architecture/hub-architecture.md` - hub startup, infra setup
  -> Constraint: AAA bundle built during hub startup via buildAAABundle; local-admin backend reads from ZeFS

### Source Files
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@.../gokrazy.go:358-378` - tryStartShell
  -> Constraint: gokrazy hardcodes shell discovery paths: /tmp/serial-busybox/ash then /perm/sh
  -> Constraint: runWithCtty uses exec.Command(shell).Run() -- forks a child, does NOT replace gokrazy init. Opens the active TTY device (/sys/class/tty/console/active), sets SysProcAttr.Setsid+Setctty. Child gets stdin/stdout/stderr on the TTY.
  -> Constraint: per-package env vars (from PackageConfig.Environment) are set only on supervised processes, NOT inherited by runWithCtty children. ze.config.dir will be absent when ze is exec'd as ash.
- [ ] `gokrazy/modcache/github.com/gokrazy/serial-busybox@.../wrapper.go` - current wrapper
  -> Constraint: wrapper creates symlink /tmp/serial-busybox/ash -> /usr/local/bin/busybox, calls DontStartOnBoot(), exits. Busybox binary in _gokrazy/extrafiles_*.tar.
- [ ] `cmd/ze/hub/main_servers.go:594-622` - usersFromZefsDB, validateLocalAdminCreds
  -> Constraint: reads meta/auth/local/username and meta/auth/local/password from ZeFS. Returns error on missing/empty. bcrypt hash.
  -> Decision: validation is in hub package, not exported. Login command needs its own ZeFS reader or the validation logic must be extractable.
- [ ] `pkg/zefs/keys.go` - ZeFS key definitions
  -> Constraint: KeyLocalAdminUsername = "meta/auth/local/username", KeyLocalAdminPassword = "meta/auth/local/password" (bcrypt hash, Private)
- [ ] `cmd/ze/main.go` - binary entry point
  -> Constraint: main() calls dispatchMain(os.Args[1:]). Has NO build tag -- runs for all binaries (ze, ze-test, ze-chaos, etc.).
- [ ] `cmd/ze/dispatch.go` - unified binary dispatch
  -> Constraint: binarySetup runs before dispatch. binarySetup is set by ze_core_dispatch.go (ze_core build tag). argv[0] check must be in a ze_core-tagged file, NOT in main.go or dispatch.go (which run for all build tags).
- [ ] `cmd/ze/ze_core_dispatch.go` - ze personality dispatch (ze_core build tag)
  -> Constraint: sets binarySetup and binaryDispatch. binarySetup receives full os.Args and can intercept argv[0] before dispatch proceeds. This is the correct place for argv[0] detection.
- [ ] `cmd/ze/ze_core_dispatch.go` - ze personality dispatch
  -> Constraint: build tag ze_core. Sets binaryDispatch. Handles YANG verbs, config file detection, global flags.
- [ ] `gokrazy/ze/config.json` - gokrazy instance config
  -> Constraint: Packages includes github.com/gokrazy/serial-busybox and codeberg.org/thomas-mangin/ze/cmd/ze. ze.gokrazy.enabled=true in environment.
- [ ] `plan/learned/831-appliance-auth-hardening.md` - auth hardening decisions
  -> Decision: local bootstrap auth uses dedicated meta/auth/local/* keys, not meta/ssh/* keys. Missing creds fail closed in hub startup. bcrypt validation at parse time.

**Key insights:**
- gokrazy hardcodes /tmp/serial-busybox/ash as first shell path; ze must place itself there
- Local admin creds are in ZeFS at meta/auth/local/{username,password}, bcrypt-hashed
- Root FS is read-only SquashFS; binary naming must happen at image build time via extrafiles
- serial-busybox provides both the symlink (wrapper.go) and the binary (extrafiles)
- ze detects argv[0] basename to switch to login mode
- Fail-open when ZeFS is missing: serial console is last-resort recovery

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/ze/config.json` - lists serial-busybox and ze as packages
- [ ] `gokrazy/modcache/.../serial-busybox/.../wrapper.go` - creates symlink, DontStartOnBoot
- [ ] `gokrazy/modcache/.../gokrazy.go:358-378` - tryStartShell checks /tmp/serial-busybox/ash then /perm/sh

**Behavior to preserve:**
- Serial console access works (operator can reach a shell for emergency recovery)
- gokrazy process supervision of ze unaffected
- SSH access path unaffected (separate auth, separate entry point)
- Web UI access path unaffected

**Behavior to change:**
- Serial console access now requires username/password authentication
- github.com/gokrazy/serial-busybox replaced by ze-controlled gokrazy package
- Busybox binary renamed from "busybox" to non-obvious name at build time; not directly runnable
- /tmp/serial-busybox/ash symlink points to ze binary, not busybox
- When ze is invoked as ash, it prompts for credentials before granting shell
- When ZeFS is missing or creds are unreadable, access is granted without auth (fail open for recovery)

## Data Flow (MANDATORY)

### Entry Point
- Operator presses key on serial console (ttyS0, 115200 baud)
- Gokrazy init detects input, calls tryStartShell()
- tryStartShell stats /tmp/serial-busybox/ash, execs it via runWithCtty

### Transformation Path
1. Gokrazy forks ze as child via exec.Command("/tmp/serial-busybox/ash").Run() with TTY on stdin/stdout/stderr
2. Ze binarySetup (ze_core_dispatch.go) checks filepath.Base(os.Args[0]) -- "ash" or "sh" -- routes to login handler before dispatch proceeds
3. Login handler resolves ZeFS path: try ze.config.dir env, fall back to hardcoded /perm/ze (gokrazy default)
4. If ZeFS missing or creds unreadable: log warning to stderr, exec shell binary (fail open)
5. If ZeFS readable: prompt username on stdout, read from stdin
6. Prompt password (no echo via golang.org/x/term.ReadPassword), read from stdin fd
7. bcrypt compare password against stored hash
8. On match: log successful login to stderr, syscall.Exec the renamed busybox binary with "ash" arg (replaces ze process; gokrazy sees child exit when busybox exits)
9. On mismatch: log failed attempt to stderr, brief delay, re-prompt (up to 3 retries)
10. After max retries or Ctrl-C: exit. Gokrazy loops back (gokrazy.go:503-518 for{Read;tryStartShell}), waits for next byte on stdin, then calls tryStartShell again -- re-authenticating every session. Same loop fires after busybox ash exits normally.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gokrazy init -> ze binary | exec via symlink, argv[0] = "ash" | [ ] |
| ze -> ZeFS database | pkg/zefs.Open, ReadFile for local admin keys | [ ] |
| ze -> renamed busybox | syscall.Exec with renamed binary path after auth | [ ] |

### Integration Points
- `pkg/zefs.Open` + `ReadFile` for credential loading
- `golang.org/x/crypto/bcrypt.CompareHashAndPassword` for validation
- `syscall.Exec` for replacing ze process with busybox (preserves controlling terminal)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ze-serial-shell can resolve ze binary path as /user/ze | gokrazy packer places supervised binaries at /user/<name> | Symlink points to wrong path; login handler unreachable | Build test image, verify /user/ze exists | unvalidated |
| A-2 | gokrazy runWithCtty provides stdin/stdout/stderr on the TTY to the forked child | gokrazy.go runWithCtty: opens active TTY, sets SysProcAttr.Setsid+Setctty | Login prompt has no terminal; stdin/stdout broken | Test serial console in QEMU | unvalidated |
| A-3 | Extrafiles tarballs with renamed busybox binary are unpacked correctly by gokrazy packer | gokrazy packer extrafiles logic is path-based, not name-based | Binary not deployed on appliance image | Build test image, verify binary exists at renamed path | unvalidated |
| A-4 | syscall.Exec from ze login into busybox preserves the TTY session | Standard POSIX exec semantics: controlling terminal, session, fd's inherited | Busybox ash has no terminal; line editing broken | Test exec chain in QEMU | unvalidated |
| A-5 | ze.config.dir env var is NOT available when gokrazy execs ze as ash | Per-package env vars only set on supervised processes (supervise.go), not runWithCtty children | N/A -- this is a known fact, not an assumption | Hardcoded fallback /perm/ze in login handler | confirmed |
| A-6 | gokrazy places the ze binary at /user/ze (packer convention) | gokrazy packer layout: /user/<binary-name> for package binaries | ze-serial-shell symlink target is wrong | Build test image, ls /user/ze | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Renamed busybox binary path hardcoded in ze and gokrazy package; if either changes independently, shell access breaks | Auth succeeds but exec fails (file not found) | Path duplicated in login.go and ze-serial-shell/main.go (separate modules, cannot share a constant). Document as maintenance risk; grep for the path on any change. |
| R-2 | ZeFS database path differs between ze startup and ash-invoked ze | Login handler gets wrong config dir | Use same ze.config.dir env var; gokrazy config sets it for all ze invocations |
| R-3 | Terminal echo not properly disabled during password entry | Password visible on screen | Use golang.org/x/term.ReadPassword which disables echo via termios |
| R-4 | Gokrazy package extrafiles tarballs need rebuilding when busybox version changes | Stale or vulnerable busybox binary on appliance | Document rebuild process; pin version in package |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| argv[0] = "ash" or "sh" | -> | login handler dispatch in ze_core_dispatch.go binarySetup | `TestShellArgvDispatch` |
| ze-serial-shell gokrazy package | -> | symlink /tmp/serial-busybox/ash -> /user/ze | `test-appliance-serial-login.ci` (functional, QEMU) |
| serial console input on gokrazy | -> | ze login -> bcrypt check -> exec ze-recovery-shell | `test-appliance-serial-login.ci` (functional, QEMU) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze invoked with argv[0] basename "ash" or "sh" | Dispatches to login handler, prompts for username and password |
| AC-2 | Correct username and password entered at login prompt | Successful login logged to stderr; process replaced with /usr/local/bin/ze-recovery-shell (ash mode); operator gets shell |
| AC-3 | Wrong password entered at login prompt | Rejection message displayed, brief delay, re-prompted; up to 3 retries then exit |
| AC-4 | ZeFS database missing or local admin creds unreadable | Warning logged, shell granted without authentication (fail open for emergency recovery) |
| AC-5 | ze-serial-shell gokrazy package starts | /tmp/serial-busybox/ash symlink created pointing to /user/ze; DontStartOnBoot called |
| AC-6 | Ze invoked with argv[0] basename "ash" but stdin is not a terminal | Exit immediately (not interactive, no login prompt) |
| AC-7 | Busybox binary at original path /usr/local/bin/busybox | Does not exist; binary only at /usr/local/bin/ze-recovery-shell |
| AC-8 | gokrazy config.json updated | serial-busybox replaced by ze-serial-shell package |
| AC-9 | ze.config.dir env var absent (gokrazy serial exec path) | Login handler falls back to hardcoded /perm/ze for ZeFS database path |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShellArgvDispatch` | `cmd/ze/login_test.go` | isShellInvocation("ash") and isShellInvocation("sh") return true; other names return false | |
| `TestLoginValidCredentials` | `cmd/ze/login_test.go` | correct bcrypt password returns success | |
| `TestLoginInvalidCredentials` | `cmd/ze/login_test.go` | wrong password returns error, retry logic works | |
| `TestLoginMissingZeFS` | `cmd/ze/login_test.go` | missing database triggers fail-open path | |
| `TestLoginMissingCreds` | `cmd/ze/login_test.go` | empty username or password in ZeFS triggers fail-open | |
| `TestLoginMaxRetries` | `cmd/ze/login_test.go` | after 3 failed attempts, handler exits | |
| `TestLoginNonTerminal` | `cmd/ze/login_test.go` | non-terminal stdin causes immediate exit | |
| `TestZeFSFallbackPath` | `cmd/ze/login_test.go` | absent ze.config.dir env falls back to /perm/ze | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| retry count | 1-3 | 3 | N/A | 4 (exits) |
| password length | 1-72 | 72 (bcrypt limit) | 0 (empty, fail-open if no creds; reject if creds exist) | 73+ (bcrypt truncates silently) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-appliance-serial-login` | `test/appliance/serial-login.ci` | Operator authenticates on serial console and gets busybox shell | |

### Interop Tests (MANDATORY for protocol features)
N/A - not a protocol feature.

### Future (if deferring any tests)
- Full QEMU integration test for serial console flow may require appliance image build pipeline and QEMU serial port forwarding; deferred if test infrastructure not ready

## Files to Modify
- `cmd/ze/ze_core_dispatch.go` - add argv[0] "ash"/"sh" detection in binarySetup (ze_core build tag)
- `gokrazy/ze/config.json` - replace serial-busybox with ze-serial-shell package
- `gokrazy/ze/builddir/github.com/gokrazy/serial-busybox/go.mod` - remove (replaced)
- `docs/guide/appliance.md` - update serial console and "What's in the image" sections

## Files to Create
- `cmd/ze/login.go` - login handler: open ZeFS, prompt, validate, exec (build tag ze_core, linux)
- `cmd/ze/login_test.go` - unit tests for login logic
- `cmd/ze-serial-shell/main.go` - gokrazy wrapper: creates symlink to ze, DontStartOnBoot
- `cmd/ze-serial-shell/_gokrazy/extrafiles_amd64.tar` - renamed busybox binary (amd64)
- `cmd/ze-serial-shell/_gokrazy/extrafiles_arm64.tar` - renamed busybox binary (arm64)
- `gokrazy/ze/builddir/codeberg.org/thomas-mangin/ze/cmd/ze-serial-shell/go.mod` - gokrazy build dependency
- `test/appliance/serial-login.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A - no config for serial login; uses existing ZeFS local admin creds |
| YANG validation constraints | No | N/A |
| YANG custom validators | No | N/A |
| CLI commands/flags | No | Not a CLI command; invoked via argv[0] dispatch |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| Pipe completeness | No | N/A |
| Env var registration | No | Uses existing ze.config.dir and ze.gokrazy.enabled |
| Doctor check for runtime dependencies | No | Renamed busybox binary guaranteed by ze-serial-shell package |
| Prometheus counters/metrics | No | Emergency serial console; no metrics needed |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` - update serial console description to note authentication |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | Not a user-facing CLI command |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - update "What's in the image" table |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | Minor dispatch addition, not architectural |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | grep docs/ for source anchors referencing ze_core_dispatch.go, config.json |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | docs/guide/appliance.md mentions serial-busybox |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- argv[0] dispatch, symlink setup |
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

1. **Phase: Wiring** -- argv[0] detection and login handler skeleton
   - Tests: `TestShellArgvDispatch`
   - Files: `cmd/ze/ze_core_dispatch.go` (argv[0] check in binarySetup), `cmd/ze/login.go` (skeleton, build tag ze_core && linux)
   - Verify: ze invoked as ash or sh reaches login handler; handler is a stub that exits
   - Note: extract `isShellInvocation(basename string) bool` for testability; test that function, not main()

2. **Phase: Login authentication** -- ZeFS credential loading and bcrypt validation
   - Tests: `TestLoginValidCredentials`, `TestLoginInvalidCredentials`, `TestLoginMissingZeFS`, `TestLoginMissingCreds`, `TestLoginMaxRetries`, `TestLoginNonTerminal`, `TestZeFSFallbackPath`
   - Files: `cmd/ze/login.go`
   - Verify: login handler reads ZeFS (with /perm/ze fallback), validates bcrypt, handles fail-open, enforces retry limit, logs success and failure to stderr
   - Note: tests must mock syscall.Exec and term.ReadPassword for darwin portability

3. **Phase: Exec into shell** -- process replacement after successful auth
   - Tests: unit test validates exec arguments are correct (mock syscall.Exec)
   - Files: `cmd/ze/login.go`
   - Verify: after auth success, process replaced with /usr/local/bin/ze-recovery-shell in ash mode

4. **Phase: Gokrazy package** -- ze-serial-shell replaces serial-busybox
   - Files: `cmd/ze-serial-shell/main.go`, extrafiles tarballs (amd64, arm64), config.json, builddir go.mod
   - Verify: gokrazy config references ze-serial-shell; symlink /tmp/serial-busybox/ash -> /user/ze; busybox at /usr/local/bin/ze-recovery-shell only
   - Note: builddir go.mod must pin gokrazy at v0.0.0-20260218074004-791851666ca2 with same replace directive as serial-busybox's go.mod (replace 2020 gokrazy -> 2026 gokrazy)
   - Note: symlink creation must be idempotent (os.Remove + os.Symlink) since wrapper may run more than once (manual restart via gokrazy web UI)
   - Note: golang.org/x/crypto/bcrypt and golang.org/x/term already in go.mod; no new dependencies needed

5. **Phase: Documentation** -- update appliance guide
   - Files: `docs/guide/appliance.md`
   - Verify: serial console section mentions authentication; "What's in the image" table updated

6. **Functional tests** -- create after feature works
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit, learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | bcrypt comparison uses constant-time compare; fail-open only when ZeFS is genuinely missing, not on wrong password |
| Naming | login handler function and file names follow ze conventions; renamed busybox uses non-obvious name |
| Data flow | credential validation is self-contained in login.go; does not import hub package |
| Security | password not echoed; no password in logs; rate limiting on retries; fail-open only for missing database; busybox not at discoverable path |
| Rule: no-layering | no old unauthenticated path remains; serial-busybox fully replaced |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| login.go exists | `ls cmd/ze/login.go` |
| login_test.go exists | `ls cmd/ze/login_test.go` |
| argv[0] dispatch in ze_core_dispatch.go | `grep -n "ash\|isShellInvocation" cmd/ze/ze_core_dispatch.go` |
| ze-serial-shell package exists | `ls cmd/ze-serial-shell/main.go` |
| ze-serial-shell extrafiles exist | `ls cmd/ze-serial-shell/_gokrazy/extrafiles_amd64.tar` |
| config.json updated | `grep "ze-serial-shell" gokrazy/ze/config.json` |
| serial-busybox removed from config | `! grep "serial-busybox" gokrazy/ze/config.json` |
| renamed binary in extrafiles | `tar tf cmd/ze-serial-shell/_gokrazy/extrafiles_amd64.tar` shows ze-recovery-shell |
| functional test exists | `ls test/appliance/serial-login.ci` |
| appliance docs updated | `grep -i "authenticat" docs/guide/appliance.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Username and password read from terminal; no injection vector (exec args are hardcoded paths, not user input) |
| Password handling | Password not logged, not stored in memory longer than needed, echo disabled during input |
| Fail-open scope | Only when ZeFS database is missing/unreadable; wrong password always rejects (never fail-open on auth error) |
| Rate limiting | Delay between failed attempts prevents brute force on serial console |
| Exec safety | Shell binary path is hardcoded constant, not derived from user input |
| Binary hiding | Busybox not at /usr/local/bin/busybox; only at renamed non-obvious path known to ze |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Embed u-root utilities in ze binary | u-root commands are package main, cannot be imported as libraries; gobusybox AST rewriting + os.Exit in dispatch makes embedding impractical | Keep busybox as shell binary, ze is login gate only |
| Implement minimal builtins with mvdan.cc/sh/v3 | User decided busybox provides sufficient shell; implementing builtins is unnecessary scope | Keep busybox, rename for security |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

u-root (github.com/u-root/u-root) commands cannot be imported as Go libraries due to the package main barrier. The gobusybox build tool works around this with AST source-to-source transformation but introduces os.Exit in dispatch and global state contamination. For embedding shell utilities in another Go binary, the practical options are: (a) fork and refactor specific commands, (b) use mvdan.cc/sh/v3 directly for the shell interpreter with custom builtins, or (c) keep busybox as a separate binary and gate access.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze as login gate, busybox stays as shell | (1) Embed u-root utilities in ze (2) Deploy u-root as separate gokrazy package (3) Implement minimal builtins with mvdan.cc/sh/v3 | u-root cannot be imported as library; embedding requires AST rewriting. Busybox works, just needs auth in front. Simplest change with most value. |
| Fail open when ZeFS missing | Fail closed (no access without creds) | Serial console is last-resort recovery. If database is corrupted, operator must be able to get in to fix it. User decision. |
| Rename busybox binary at build time | (1) Keep at /usr/local/bin/busybox (2) Move at runtime to /tmp | Root FS is read-only SquashFS; rename must be at build time. Keeping original name allows direct access bypassing auth. Rename closes that path. User decision: keep the location, change the name. |
| Own gokrazy package replacing serial-busybox | (1) Keep serial-busybox, overwrite symlink at ze startup (2) Fork serial-busybox | Own package gives full control over binary name and symlink target. No race window. Clean replacement. |
| argv[0] detection in ze_core_dispatch.go binarySetup | (1) Detection in main.go (no build tag, fires on all builds) (2) Separate ze-login binary | argv[0] matches how busybox itself works (multicall binary). Must be in ze_core-tagged file so ze-test/ze-chaos don't trigger login. binarySetup runs before dispatch and has access to os.Args[0]. |

## Known Limitations
- Only local admin credentials supported; RADIUS/TACACS not wired for serial console auth
- bcrypt silently truncates passwords longer than 72 bytes (inherent bcrypt limitation)
- Password input requires a terminal; non-interactive invocations exit immediately
- Ctrl-C during login terminates ze; gokrazy re-triggers shell on next serial input (expected behavior)
- Extrafiles tarballs built for amd64 and arm64 only; ze appliance does not target 386 or arm
- login.go requires linux build tag (syscall.Exec, term.ReadPassword); tests mock these for darwin

## RFC Documentation

N/A - not a protocol feature.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Serial console requires authentication | functional test | test-appliance-serial-login.ci |
| Fail open when ZeFS missing | unit test | TestLoginMissingZeFS |
| Busybox shell accessible after auth | functional test | test-appliance-serial-login.ci |
| Busybox not directly runnable by name | build verification | extrafiles contain ze-recovery-shell only |
| ZeFS path works without per-package env | unit test | TestZeFSFallbackPath |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
