# Spec: install-7c-vendor-updater

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-install-7a-namespace |
| Phase | - |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/install/appliance/cmd_push.go` - current push implementation
4. `cmd/ze/install/appliance/cmd_push_test.go` - existing tests
5. `gokrazy/tools/vendor/github.com/gokrazy/updater/updater.go` - updater API
6. `plan/learned/677-appliance-2-remote.md` - remote ops decisions

## Task

Replace the raw HTTP PUT in `doPush()` with the gokrazy updater library API.
The updater provides proper A/B root partition handling, hash verification
(CRC32 fast path or SHA256 fallback), protocol feature detection, testboot
support, and partition-aware streaming.

The updater library (`github.com/gokrazy/updater`) is already vendored in
`gokrazy/tools/vendor/`. It has zero external dependencies (stdlib only).

## Required Reading

### Architecture Docs
- [ ] `plan/learned/677-appliance-2-remote.md` - remote ops decisions
  -> Decision: push uses HTTP basic auth (empty user, token as password), TLS cert pinning
  -> Constraint: protocolError type differentiates auth errors from network errors
- [ ] `plan/spec-install-7-gokrazy-build.md` - umbrella spec, Component 3
  -> Decision: use vendored updater.NewTarget() API with StreamTo/Switch/Reboot
  -> Constraint: TLS cert pinning preserved via custom *http.Client as HTTPDoer

### Source Files
- [ ] `cmd/ze/install/appliance/cmd_push.go` - current doPush() does raw HTTP PUT
  -> Constraint: loadDeviceTLS, verifyImageChecksum, resolveImagePath preserved
  -> Constraint: protocolError type, isProtocolError helper preserved
  -> Decision: doPush injectable via doPushFn for testability
- [ ] `cmd/ze/install/appliance/cmd_push_test.go` - 8 existing tests
  -> Constraint: all existing test scenarios must continue passing
- [ ] `gokrazy/tools/vendor/github.com/gokrazy/updater/updater.go` - API
  -> API: NewTarget(ctx, baseURL, HTTPDoer) -> (*Target, error)
  -> API: target.StreamTo(ctx, "root", reader) - streams with hash verification
  -> API: target.Switch(ctx) - switches A/B partition
  -> API: target.Reboot(ctx, opts...) - reboots device
  -> API: target.Testboot(ctx) - marks boot as test (auto-revert)
  -> API: target.Supports(ProtocolFeature) - feature detection
  -> Constraint: NewTarget calls /update/features endpoint (needs network)

**Key insights:**
- updater.StreamTo does hash verification (CRC32 or SHA256) the current code lacks
- updater handles A/B partition correctly; current code just PUTs the whole image
- The HTTPDoer interface is satisfied by *http.Client, preserving TLS cert pinning
- updater.NewTarget probes /update/features; older devices return 404 (handled)
- The updater library has ZERO external deps, all stdlib

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/install/appliance/cmd_push.go` - doPush opens image, HTTP PUT to /update
  -> Constraint: basic auth with empty user, update token as password
  -> Constraint: TLS config from loadDeviceTLS with cert pinning
  -> Constraint: 401 -> protocolError, non-200 -> protocolError, network -> raw error
- [ ] `cmd/ze/install/appliance/cmd_push_test.go` - 8 tests using httptest.NewTLSServer
  -> Constraint: tests verify auth header, body content, parallel push, partial failure

**Behavior to preserve:**
- pushOne/pushAll dispatch, --all/--parallel/--image flags unchanged
- LoadConfig, resolveImagePath, verifyImageChecksum, loadDeviceTLS unchanged
- Encrypted secrets, passphrase agent flow unchanged
- Error message differentiation: "unreachable" vs protocol errors
- All 8 existing test scenarios pass

**Behavior to change:**
- doPush() replaced with updater-based implementation
- Image streamed to "root" destination via StreamTo (hash-verified)
- Switch A/B partition after streaming
- Reboot device after switch
- New --testboot flag for safe updates (auto-revert on failure)
- New --no-reboot flag to skip reboot after push
- Feature detection logged (protocol capabilities)

## Data Flow (MANDATORY)

### Entry Point
- `ze install appliance push <name>` CLI invocation

### Transformation Path
1. pushOne loads config, resolves secrets, resolves image path
2. loadDeviceTLS builds TLS config with cert pinning
3. Build *http.Client with pinned TLS and basic auth transport
4. updater.NewTarget(ctx, baseURL, httpClient) probes features
5. target.StreamTo(ctx, "root", imageReader) streams image with hash
6. target.Switch(ctx) activates the new root partition
7. target.Reboot(ctx) or target.Testboot(ctx) based on flags

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> pushOne | flag parsing, dispatch | [ ] |
| pushOne -> updater.NewTarget | Go API (vendored library) | [ ] |
| updater -> device HTTP | TLS with cert pinning | [ ] |
| updater hash verification | CRC32 or SHA256 per feature | [ ] |

## Architecture

### Integration Approach

The updater library is already vendored in `gokrazy/tools/vendor/`. Rather than
adding it to ze's main go.mod (which would pull deps), we use the same pattern
as spec-install-7b: a thin wrapper program under `gokrazy/tools/cmd/` invoked
via `go run`. However, the push operation is interactive (progress, error handling)
and doesn't benefit from the `go run` indirection.

Instead, copy the single updater.go file (stdlib-only, ~430 lines) into
`cmd/ze/install/appliance/updater/` as a vendored internal copy. This avoids:
- Adding external deps to ze's main go.mod
- The `go run` latency for an interactive command
- API drift since we pin to the exact version already in gokrazy/tools

### Auth Transport

The updater's HTTPDoer interface needs basic auth on every request. Wrap the
TLS-pinned *http.Client with an authTransport that injects the Authorization
header, replacing the per-request SetBasicAuth in the current code.

## Design Decisions

| Decision | Chosen | Over | Reason |
|----------|--------|------|--------|
| Copy updater.go locally | Internal copy in appliance/updater/ | go.mod dependency | Zero new deps, no go run latency, pin exact version |
| Auth via transport wrapper | authTransport wrapping http.Transport | Per-request SetBasicAuth | updater.NewTarget makes its own requests; auth must be on all |
| Keep protocolError | Extend with updater error mapping | Remove | Preserves existing error message differentiation |
| Add --testboot flag | New flag on push | Always testboot | Operator choice; testboot not always available on old devices |
| Add --no-reboot flag | New flag on push | Always reboot | Useful for batch updates where operator reboots at a chosen time |
| Injectable doPushFn | Function variable pattern | Direct call | Matches existing runExternalFn/gokBuildFn testability pattern |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze install appliance push lab` | Image streamed via updater.StreamTo to "root" destination |
| AC-2 | Push to device | Hash verification (CRC32 or SHA256) performed by updater |
| AC-3 | Push succeeds | A/B partition switched via updater.Switch |
| AC-4 | Push succeeds (no --no-reboot) | Device rebooted via updater.Reboot |
| AC-5 | `push --testboot lab` | Uses updater.Testboot instead of Switch |
| AC-6 | `push --no-reboot lab` | Streams and switches but does not reboot |
| AC-7 | Device returns 401 | protocolError with "Unauthorized" message |
| AC-8 | Device unreachable | Error message says "unreachable" (not protocol error) |
| AC-9 | Push with TLS cert pinning | Custom TLS config from loadDeviceTLS used by updater |
| AC-10 | `push --all --parallel 4` | All devices updated via updater (parallel preserved) |
| AC-11 | Existing tests | All 8 existing push tests pass with updater-based mock |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestPushUsesUpdaterStreamTo | cmd_push_test.go | Image sent via StreamTo "root" | |
| TestPushSwitchesPartition | cmd_push_test.go | Switch called after StreamTo | |
| TestPushReboots | cmd_push_test.go | Reboot called after Switch | |
| TestPushTestboot | cmd_push_test.go | --testboot uses Testboot instead of Switch | |
| TestPushNoReboot | cmd_push_test.go | --no-reboot skips Reboot | |
| TestPushHashVerification | cmd_push_test.go | StreamTo response contains valid hash | |
| TestAuthTransport | cmd_push_test.go | Auth header injected on all requests | |

### Existing Tests (must continue passing)

| Test | Validates |
|------|-----------|
| TestPushSendsImage | Image content reaches device |
| TestPushUnreachableDevice | Network error handled |
| TestPushWrongToken | 401 error handled |
| TestPushSpecificImage | --image flag works |
| TestPushAllParallel | --all --parallel N works |
| TestPushAllParallelPartialFailure | Partial failure reported |
| TestPushAllParallelDefault | --parallel 1 works |
| TestPushAllIteratesAppliances | All addressed devices pushed |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze install appliance push lab` | -> | updater.StreamTo + Switch + Reboot | TestPushUsesUpdaterStreamTo |
| `ze install appliance push --testboot lab` | -> | updater.Testboot | TestPushTestboot |

## Files to Modify

- `cmd/ze/install/appliance/cmd_push.go` - replace doPush with updater-based impl
- `cmd/ze/install/appliance/cmd_push_test.go` - update mock server for updater protocol

## Files to Create

- `cmd/ze/install/appliance/updater/updater.go` - local copy of gokrazy updater

## Implementation Steps

### Phase 1: Copy updater library
1. Copy updater.go from gokrazy/tools/vendor to cmd/ze/install/appliance/updater/
2. Adjust package name

### Phase 2: Auth transport
1. Add authTransport type wrapping http.RoundTripper
2. Injects basic auth header on all requests

### Phase 3: Replace doPush
1. Make doPush injectable (doPushFn pattern)
2. New implementation: NewTarget, StreamTo("root"), Switch, Reboot
3. Add --testboot and --no-reboot flags
4. Map updater errors to protocolError where appropriate

### Phase 4: Update tests
1. Update mock server to handle updater protocol (/update/features, /update/root, /update/switch, /reboot)
2. Add new tests for testboot, no-reboot, hash verification
3. Verify all 8 existing tests pass

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] All existing push tests pass
- [ ] `make ze-test` passes
- [ ] Feature code integrated

### Completion (BLOCKING)
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-install-7c-vendor-updater.md`
