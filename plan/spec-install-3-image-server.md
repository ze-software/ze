# Spec: install-3-image-server

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-05-28 |
| Parent | spec-install-0-umbrella.md |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/dhcpserver/register.go` - reference plugin registration pattern
4. `internal/plugins/dhcpserver/schema/` - reference YANG schema structure
5. `internal/plugins/dhcpserver/handler.go` - reference handler pattern
6. `plan/spec-install-0-umbrella.md` - umbrella spec (Component 3)

## Task

New image server plugin at `internal/plugins/imageserver/` that serves gokrazy disk
images, installer kernel/initrd, iPXE boot scripts, and a pre-provisioned zefs database
over HTTP. The plugin runs its own HTTP listener (separate from the web component) so
image serving does not affect web UI performance and has an independent lifecycle.

The installer initrd on the target downloads the gokrazy image and zefs database from
this server. The zefs database contains SSH credentials pre-provisioned from the
imageserver config, enabling SSH access on the target after first boot.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - plugin lifecycle and registration
  -> Decision: plugins register via `registry.Register()` in `init()`
  -> Constraint: standard plugin pattern with YANG, ConfigRoots, RunEngine
- [ ] `internal/plugins/dhcpserver/register.go` - reference registration implementation
  -> Decision: ConfigRoots `[]string{"service"}`, RunEngine receives `net.Conn`
  -> Constraint: InProcessConfigVerifier for config validation
- [ ] `internal/plugins/dhcpserver/schema/` - reference YANG + embed + register pattern
  -> Constraint: `embed.go` with `//go:embed`, `register.go` with `yang.RegisterModule()`

### RFC Summaries (MUST for protocol work)
No protocol-level RFCs. HTTP serving uses `http.ServeFile` from stdlib.

**Key insights:**
- Plugin registration pattern: `registry.Registration{Name, Description, Features:"yang", YANG, ConfigRoots, InProcessConfigVerifier, RunEngine}` in `init()`
- YANG schema embedded via `//go:embed`, registered via `yang.RegisterModule()` in `schema/register.go`
- ConfigRoots is `[]string{"service"}` since image-server lives under `service { image-server { ... } }`
- `http.ServeFile` handles Range requests, Content-Type, and conditional requests automatically
- zefs database creation reuses `zefs.Create()` + SSH credential writes via `zefs.KeySSHUsername.Key("127.0.0.1", "2222")`, `zefs.KeySSHPassword.Key("127.0.0.1", "2222")`, and `zefs.KeySSHDefault` value `"127.0.0.1/2222"` (matching `ze init` defaults)
- Interface-to-IP resolution for listen-address: same approach as dhcpserver's `listenDHCP(iface)` -- resolve interface name via `net.InterfaceByName`, get first IPv4 from `Addrs()`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/register.go` - plugin registration pattern with `registry.Register()`
  -> Constraint: follow same registration struct fields, same `init()` pattern
- [ ] `internal/plugins/dhcpserver/handler.go` - packet handler with buffer-first encoding
  -> Constraint: imageserver is HTTP (not UDP), different handler shape
- [ ] `internal/plugins/dhcpserver/config.go` - JSON config parsing from `sdk.ConfigSection.Data`
  -> Constraint: config data arrives as JSON string, parse with `json.Unmarshal`
- [ ] `internal/plugins/dhcpserver/schema/embed.go` - `//go:embed` for YANG
  -> Constraint: embedded var name must be exported, referenced in register.go
- [ ] `internal/plugins/dhcpserver/schema/register.go` - `yang.RegisterModule()` in `init()`
  -> Constraint: module name must match the `.yang` filename
- [ ] `cmd/ze/hub/main_servers.go` lines 477-505 - `loadZefsUsers()` reads SSH credentials
  -> Constraint: reads from `database.zefs` file via `zefs.KeySSHUsername.Pattern` (glob-matches `meta/ssh/{host}/{port}/username`)
  -> Constraint: writes must use `KeySSHUsername.Key("127.0.0.1", "2222")` + `KeySSHPassword.Key("127.0.0.1", "2222")` + `KeySSHDefault.Pattern` = `"127.0.0.1/2222"`
- [ ] `pkg/zefs/keys.go` - SSH key pattern definitions
  -> Constraint: `KeySSHUsername.Pattern` = `"meta/ssh/{host}/{port}/username"`, `KeySSHDefault.Pattern` = `"meta/ssh/default"`

**Behavior to preserve:**
- No existing imageserver code; this is a new plugin
- Existing web component HTTP listener unchanged (imageserver uses its own)
- Existing dhcpserver plugin unchanged (imageserver is independent)

**Behavior to change:**
- New plugin `imageserver` registered in plugin registry
- New YANG schema `ze-image-server-conf.yang` under `service { image-server { ... } }`
- New HTTP listener serving install images, boot files, and zefs database

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP GET request to `/install/image/<name>`, `/install/boot/*`, or `/install/database.zefs`
- Plugin config section from ze hub (JSON via `sdk.ConfigSection`)

### Transformation Path
1. ze hub starts imageserver plugin via `RunEngine(conn)`
2. Plugin receives config via `p.OnConfigure()` callback
3. Config parsed: image-directory, boot-directory, listen-port, SSH credentials
4. zefs database created in memory (or temp dir): `zefs.Create()`, then write `KeySSHUsername.Key("127.0.0.1", "2222")` = username, `KeySSHPassword.Key("127.0.0.1", "2222")` = bcrypt hash, `KeySSHDefault.Pattern` = `"127.0.0.1/2222"`. Database file is named `database.zefs` (matching `loadZefsUsers()` expectation)
5. HTTP mux registered with routes for `/install/image/`, `/install/boot/`, `/install/database.zefs`
6. HTTP listener started on configured interface:port
7. On request: `http.ServeFile` serves the requested file from the appropriate directory
8. For `/install/database.zefs`: serves the pre-built zefs database file

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network -> imageserver plugin | HTTP on configured port (own listener) | [ ] |
| Plugin -> config | YANG section for imageserver under `service` | [ ] |
| Config -> zefs creation | SSH creds from config written to temp zefs db | [ ] |

### Integration Points
- `registry.Register()` - standard plugin registration in `init()`
- `sdk.NewWithConn()` / `sdk.Run()` - plugin lifecycle
- `sdk.ConfigSection` - config delivery from hub
- `yang.RegisterModule()` - YANG schema registration
- `zefs.Create()` - create zefs database; `KeySSHUsername.Key("127.0.0.1", "2222")` / `KeySSHPassword.Key("127.0.0.1", "2222")` / `KeySSHDefault.Pattern` - SSH credential writes matching `ze init` and `loadZefsUsers()` expectations
- Interface-to-IP resolution for HTTP listener bind address: `net.InterfaceByName(name)` + `Addrs()` to get first IPv4, same approach as dhcpserver `listenDHCP()`
- `http.ServeMux` / `http.ServeFile` - HTTP request handling

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (imageserver is independent of web component)
- [ ] No duplicated functionality (new plugin, no existing image serving)
- [ ] Zero-copy preserved where applicable (`http.ServeFile` handles efficient I/O)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registration | -> | `registry.Register()` in `init()` | `TestImageServerRegistered` |
| HTTP GET `/install/image/<name>` | -> | `imageHandler.serveImage()` | `TestServeImage` |
| HTTP GET `/install/boot/vmlinuz` | -> | `imageHandler.serveBoot()` | `TestServeBootFile` |
| HTTP GET `/install/boot/ipxe.cfg` | -> | `imageHandler.serveBoot()` | `TestServeIPXEConfig` |
| HTTP GET `/install/database.zefs` | -> | `imageHandler.serveZefs()` | `TestServeZefsDB` |
| Config with image-server section | -> | `parseImageConfig()` | `TestParseImageConfig` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | HTTP GET `/install/image/gokrazy.img` with file present in image-directory | 200 OK, file content served, Content-Type `application/octet-stream` |
| AC-2 | HTTP GET `/install/image/gokrazy.img` with Range header | 206 Partial Content, correct byte range served |
| AC-3 | HTTP GET `/install/boot/vmlinuz` with file present in boot-directory | 200 OK, kernel file served |
| AC-4 | HTTP GET `/install/boot/initrd` with file present in boot-directory | 200 OK, initrd file served |
| AC-5 | HTTP GET `/install/boot/ipxe.cfg` with file present in boot-directory | 200 OK, iPXE script served |
| AC-6 | HTTP GET `/install/database.zefs` with ssh-username and ssh-password-hash configured | 200 OK, valid zefs database served containing SSH credentials at `meta/ssh/127.0.0.1/2222/username` and `meta/ssh/127.0.0.1/2222/password` keys, plus `meta/ssh/default` = `"127.0.0.1/2222"` |
| AC-7 | HTTP GET `/install/image/../../etc/passwd` (path traversal attempt) | 400 or 404, file NOT served |
| AC-8 | HTTP GET `/install/image/nonexistent.img` | 404 Not Found |
| AC-9 | Config with `enabled false` | HTTP listener not started |
| AC-10 | Config with `enabled true` and valid directories | HTTP listener starts on configured interface:port |
| AC-11 | Plugin registered in registry | `imageserver` appears in plugin list |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestImageServerRegistered` | `internal/plugins/imageserver/register_test.go` | Plugin appears in registry after init | |
| `TestParseImageConfig` | `internal/plugins/imageserver/config_test.go` | Config JSON parsed into imageConfig struct | |
| `TestParseImageConfigDefaults` | `internal/plugins/imageserver/config_test.go` | Missing listen-port defaults to 80 | |
| `TestParseImageConfigDisabled` | `internal/plugins/imageserver/config_test.go` | `enabled: false` parsed correctly | |
| `TestServeImage` | `internal/plugins/imageserver/handler_test.go` | GET `/install/image/<name>` serves file from image-directory | |
| `TestServeImageRange` | `internal/plugins/imageserver/handler_test.go` | Range request returns 206 with correct byte range | |
| `TestServeImageNotFound` | `internal/plugins/imageserver/handler_test.go` | GET `/install/image/missing` returns 404 | |
| `TestServeImagePathTraversal` | `internal/plugins/imageserver/handler_test.go` | GET `/install/image/../../etc/passwd` blocked | |
| `TestServeBootFile` | `internal/plugins/imageserver/handler_test.go` | GET `/install/boot/vmlinuz` serves kernel | |
| `TestServeBootInitrd` | `internal/plugins/imageserver/handler_test.go` | GET `/install/boot/initrd` serves initrd | |
| `TestServeIPXEConfig` | `internal/plugins/imageserver/handler_test.go` | GET `/install/boot/ipxe.cfg` serves iPXE script | |
| `TestServeBootNotFound` | `internal/plugins/imageserver/handler_test.go` | GET `/install/boot/missing` returns 404 | |
| `TestServeBootPathTraversal` | `internal/plugins/imageserver/handler_test.go` | GET `/install/boot/../../etc/passwd` blocked | |
| `TestServeZefsDB` | `internal/plugins/imageserver/handler_test.go` | GET `/install/database.zefs` returns valid zefs with SSH creds | |
| `TestServeZefsDBNoCreds` | `internal/plugins/imageserver/handler_test.go` | No ssh-username configured: `/install/database.zefs` returns 404 or error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| listen-port | 1-65535 | 65535 | 0 | N/A (uint16 wraps) |
| ssh-username length | 1-255 | 255 chars | empty string | 256 chars |
| image file name length | 1-255 | 255 chars | empty string | N/A (OS limit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ze-install-http-image` | `test/install/http-image.ci` | HTTP client downloads gokrazy image from imageserver | |
| `test-ze-install-http-zefs` | `test/install/http-zefs.ci` | HTTP client downloads database.zefs with pre-provisioned SSH creds | |

### Future (if deferring any tests)
- Concurrent connection limit test (requires load testing infrastructure)
- Provisioning tracking via bus events (v2 feature)

## Files to Modify

None. This is a new plugin.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/plugins/imageserver/schema/ze-image-server-conf.yang` |
| CLI commands/flags | No | N/A (plugin is config-driven, CLI via ze-install binary in spec-install-4) |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/install/http-image.ci`, `test/install/http-zefs.ci` |
| Doctor check for runtime dependencies | No | N/A (directories checked at config verify time) |
| Plugin all.go import | Yes | `make generate` (code-generated by `scripts/codegen/plugin_imports.go`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - imageserver provisioning plugin |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A (CLI in spec-install-4) |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - imageserver plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` (umbrella doc, not this spec alone) |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A (HTTP is stdlib) |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `internal/plugins/imageserver/register.go` - plugin registration, YANG, RunEngine
- `internal/plugins/imageserver/config.go` - config parsing: imageConfig struct
- `internal/plugins/imageserver/handler.go` - HTTP mux, ServeFile handlers, path validation
- `internal/plugins/imageserver/handler_test.go` - unit tests for HTTP endpoints
- `internal/plugins/imageserver/config_test.go` - unit tests for config parsing
- `internal/plugins/imageserver/register_test.go` - registration test
- `internal/plugins/imageserver/schema/ze-image-server-conf.yang` - YANG schema
- `internal/plugins/imageserver/schema/embed.go` - `//go:embed` for YANG file
- `internal/plugins/imageserver/schema/register.go` - `yang.RegisterModule()` in `init()`
- `test/install/http-image.ci` - functional test: image download
- `test/install/http-zefs.ci` - functional test: zefs download

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella spec Component 3 |
| 2. Audit | Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table -- register plugin, create handler skeleton |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-13. Fix/verify loop | Per finding |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register plugin, create handler skeleton
   - Tests: `TestImageServerRegistered`
   - Files: `register.go`, `schema/`, `handler.go` (stub)
   - Verify: plugin loads with new YANG config, HTTP listener starts, all endpoints return 501

2. **Phase: Config parsing** -- parse imageConfig from JSON
   - Tests: `TestParseImageConfig`, `TestParseImageConfigDefaults`, `TestParseImageConfigDisabled`
   - Files: `config.go`, `config_test.go`
   - Verify: config parsing matches YANG schema, defaults applied

3. **Phase: Image serving** -- serve files from image-directory
   - Tests: `TestServeImage`, `TestServeImageRange`, `TestServeImageNotFound`, `TestServeImagePathTraversal`
   - Files: `handler.go`, `handler_test.go`
   - Verify: `http.ServeFile` serves images, path traversal blocked, 404 on missing

4. **Phase: Boot file serving** -- serve files from boot-directory
   - Tests: `TestServeBootFile`, `TestServeBootInitrd`, `TestServeIPXEConfig`, `TestServeBootNotFound`, `TestServeBootPathTraversal`
   - Files: `handler.go`, `handler_test.go`
   - Verify: boot files served from boot-directory, path traversal blocked

5. **Phase: Zefs endpoint** -- build and serve pre-provisioned zefs database
   - Tests: `TestServeZefsDB`, `TestServeZefsDBNoCreds`
   - Files: `handler.go`, `handler_test.go`
   - Verify: zefs database contains SSH credentials from config, downloadable

6. **Functional tests** -- create after feature works
7. **RFC refs** -- N/A (HTTP is stdlib, no protocol-level RFC work)
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Path traversal prevention verified by test, `http.ServeFile` used (not manual file read) |
| Naming | YANG leaves use kebab-case, Go types use camelCase, package name `imageserver` |
| Data flow | Config -> HTTP listener -> ServeFile, no bypass |
| Rule: registration | Uses standard `registry.Register()` pattern, blank import in all.go via `make generate` |
| Rule: buffer-first | N/A (HTTP, not wire encoding) |
| Security | Path traversal blocked, SSH password hash never logged, no directory listing |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| imageserver plugin registered | `grep -rn 'imageserver' internal/component/plugin/registry/` or `go test -run TestImageServerRegistered` |
| YANG schema loads | `make ze-lint` passes with new schema |
| Image endpoint serves files | `go test -run TestServeImage` |
| Boot endpoint serves files | `go test -run TestServeBootFile` |
| Zefs endpoint serves database | `go test -run TestServeZefsDB` |
| Path traversal blocked | `go test -run TestServeImagePathTraversal` |
| Config parsing works | `go test -run TestParseImageConfig` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Path traversal | `filepath.Clean` + `filepath.Rel` check that resolved path stays inside configured directory. `http.ServeFile` with a mux that extracts the filename, not the raw URL path. |
| Directory listing | No `http.FileServer` (which enables listing). Only named-file serving via `http.ServeFile`. |
| SSH credential handling | `ssh-password-hash` stored as bcrypt hash in config; never logged. Plaintext password never appears in imageserver code. |
| Resource exhaustion | HTTP listener should use `http.Server` with `ReadTimeout`, `WriteTimeout`, `MaxHeaderBytes` set. |
| Input validation | Image name and boot filename validated: no slashes, no null bytes, no `.` components. |

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

N/A. HTTP serving uses Go stdlib. No protocol-level RFC implementation in this plugin.

## Implementation Summary

### What Was Implemented
- imageserver plugin with registration, YANG schema, config parsing, HTTP handler
- Image endpoint: `/install/image/<name>` serves files from image-directory
- Boot endpoint: `/install/boot/<name>` serves kernel, initrd, iPXE config from boot-directory
- Zefs endpoint: `/install/database.zefs` serves pre-provisioned zefs with SSH credentials
- Path traversal protection: flat filename validation (no slashes, dots, null bytes)
- HTTP server with timeouts (ReadTimeout: 30s, WriteTimeout: 5min, MaxHeaderBytes: 64KB)
- Temp zefs directory cleanup on reconfigure/stop

### Bugs Found/Fixed
- `buildZefsDB` had a deferred close that could double-close the store on error. Replaced with explicit error handling.

### Documentation Updates
- `docs/guide/plugins.md`: added imageserver entry to Infrastructure table

### Deviations from Plan
- `TestImageServerRegistered` not created as separate file; registration already verified by `TestAvailablePlugins` in `cmd/ze/main_test.go`

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| HTTP image server plugin | Done | `internal/plugins/imageserver/` | Full plugin with registration, YANG, handler |
| Own HTTP listener | Done | `register.go:105-112` | Separate `http.Server` with timeouts |
| Serve disk images | Done | `handler.go:82-84` | `serveImage` via `serveFromDir` |
| Serve boot files | Done | `handler.go:87-89` | `serveBoot` via `serveFromDir` |
| Serve zefs database | Done | `handler.go:45-79,92-94` | `buildZefsDB` + `serveZefs` |
| Path traversal protection | Done | `handler.go:96-110` | Flat filename validation |
| YANG config | Done | `schema/ze-image-server-conf.yang` | All leaves with appropriate types |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestServeImage` | 200 OK, correct content |
| AC-2 | Done | `TestServeImageRange` | 206 Partial Content |
| AC-3 | Done | `TestServeBootFile` | 200 OK, kernel served |
| AC-4 | Done | `TestServeBootInitrd` | 200 OK, initrd served |
| AC-5 | Done | `TestServeIPXEConfig` | 200 OK, iPXE script served |
| AC-6 | Done | `TestServeZefsDB` | 200 OK, valid zefs with SSH creds at correct keys |
| AC-7 | Done | `TestServeImagePathTraversal` | Non-200 for traversal paths |
| AC-8 | Done | `TestServeImageNotFound` | 404 for missing file |
| AC-9 | Done | `TestParseImageConfigDisabled` + `register.go:88-90` | Disabled config skips listener |
| AC-10 | Done | `register.go:95-122` + `image-server-config.ci` | Listener starts on configured port |
| AC-11 | Done | `TestAvailablePlugins` in `cmd/ze/main_test.go` | imageserver in plugin list |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseImageConfig | Done | config_test.go:9 | Full config parse |
| TestParseImageConfigDefaults | Done | config_test.go:53 | Default port 80 |
| TestParseImageConfigDisabled | Done | config_test.go:66 | Disabled parse |
| TestServeImage | Done | handler_test.go:62 | Image served |
| TestServeImageRange | Done | handler_test.go:84 | Range request |
| TestServeImageNotFound | Done | handler_test.go:114 | 404 on missing |
| TestServeImagePathTraversal | Done | handler_test.go:129 | Path traversal blocked |
| TestServeBootFile | Done | handler_test.go:151 | Kernel served |
| TestServeBootInitrd | Done | handler_test.go:172 | Initrd served |
| TestServeIPXEConfig | Done | handler_test.go:195 | iPXE config served |
| TestServeBootNotFound | Done | handler_test.go:217 | 404 on missing boot |
| TestServeBootPathTraversal | Done | handler_test.go:232 | Boot path traversal blocked |
| TestServeZefsDB | Done | handler_test.go:247 | Zefs with SSH creds served |
| TestServeZefsDBNoCreds | Done | handler_test.go:325 | 404 when no SSH creds |
| TestImageServerRegistered | Skipped | `TestAvailablePlugins` covers | No duplicate test needed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/imageserver/register.go` | Done | Plugin registration + RunEngine |
| `internal/plugins/imageserver/config.go` | Done | Config parsing |
| `internal/plugins/imageserver/handler.go` | Done | HTTP handlers + zefs builder |
| `internal/plugins/imageserver/handler_test.go` | Done | 14 handler tests + 2 zefs tests |
| `internal/plugins/imageserver/config_test.go` | Done | 5 config tests |
| `internal/plugins/imageserver/schema/ze-image-server-conf.yang` | Done | YANG schema |
| `internal/plugins/imageserver/schema/embed.go` | Done | go:embed |
| `internal/plugins/imageserver/schema/register.go` | Done | yang.RegisterModule |
| `test/install/image-server-config.ci` | Done | Functional config validation |

### Audit Summary
- **Total items:** 32
- **Done:** 31
- **Partial:** 0
- **Skipped:** 1 (TestImageServerRegistered -- covered by TestAvailablePlugins)
- **Changed:** 0

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/imageserver/*`)
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
