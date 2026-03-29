# Spec: plugin-tls-hardening

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - plugin IPC protocol
4. `internal/component/plugin/ipc/tls.go` - TLS auth (engine-side)
5. `pkg/plugin/sdk/sdk.go` - SDK TLS client (plugin-side)
6. `internal/component/plugin/process/process.go` - process startup, env var passing
7. `internal/component/plugin/manager/manager.go` - acceptor creation, token generation

## Task

Harden the TLS plugin transport with three changes:
1. **Per-plugin tokens with name binding** -- each external plugin gets its own random token; the engine validates the presented name matches the expected name for that token
2. **Certificate fingerprint pinning** -- the engine passes the server cert SHA-256 fingerprint to the plugin via env var; the plugin verifies it during TLS handshake instead of `InsecureSkipVerify`
3. **Auto-clear secret env vars** -- `env.Get()` calls `os.Unsetenv` after first read for any var registered with `Secret: true` (value stays in cache); removes secrets from `/proc/<pid>/environ`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/api/process-protocol.md` - plugin protocol lifecycle, 5-stage startup, auth stage 0
  -> Decision: Auth is stage 0 (`#0 auth {"token":"...","name":"..."}`), before 5-stage startup
  -> Constraint: Single connection model -- same conn for auth and all subsequent RPCs
- [ ] `.claude/rules/plugin-design.md` - plugin transport section, registration fields
  -> Decision: External plugins connect back via TLS, auth via token in env var
  -> Constraint: Internal plugins use net.Pipe, not TLS -- TLS hardening is external-only

### RFC Summaries (MUST for protocol work)
- N/A -- this is internal transport security, not a BGP protocol feature

**Key insights:**
- Auth frame: `#0 auth {"token":"...","name":"..."}` -- name is self-declared, not verified against expectation
- `Authenticate()` accepts any valid plugin name if the shared token matches -- no name binding
- `AuthenticateWithLookup()` already supports per-name secrets but is only used for fleet/managed client connections, not for forked plugins
- All forked plugins get the same token from `acceptor.Token()` via `ZE_PLUGIN_HUB_TOKEN` env var
- `InsecureSkipVerify: true` in `sdk.go:150` -- no cert verification at all
- Cert is ephemeral (P-256, 24h), regenerated each engine restart -- fingerprint must be passed at spawn time
- Manager auto-generates 32-byte random token when no hub config exists (`manager.go:193-202`)
- Token visible in `/proc/<pid>/environ` until process exits -- `Secret` flag on `EnvEntry` will auto-clear after first read
- `EnvEntry.Private` means "hidden from user-facing output" (existing); `EnvEntry.Secret` means "cleared from OS env after first read" (new)

## Current Behavior (MANDATORY)

**Source files read:**
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/plugin/ipc/tls.go` (456L) - Authenticate, AuthenticateWithLookup, SendAuth, GenerateSelfSignedCert, StartListeners, PluginAcceptor
  -> Constraint: readLineRaw byte-by-byte to avoid buffering ahead -- preserve this
  -> Constraint: writeErrorRaw and FormatOK write directly to conn (no rpc.Conn) -- preserve this
  -> Constraint: constant-time token comparison -- preserve this
- [ ] `pkg/plugin/sdk/sdk.go` (350L) - NewFromTLSEnv reads ze.plugin.hub.{host,port,token} env vars, connects TLS with InsecureSkipVerify, calls SendAuth, reads OK
  -> Constraint: env vars registered via env.MustRegister -- new vars must follow this pattern
  -> Constraint: auth response read uses rpc.NewConn + ReadRequest before creating MuxConn
- [ ] `internal/component/plugin/process/process.go` (688L) - startExternal sets ZE_PLUGIN_HUB_{HOST,PORT,TOKEN} and ZE_PLUGIN_NAME env vars, calls acceptor.WaitForPlugin
  -> Constraint: rawConn stored for later InitConns -- preserve this lifecycle
- [ ] `internal/component/plugin/manager/manager.go` (280L) - ensureAcceptor creates cert, listener, PluginAcceptor with server.Secret; auto-generates 32-byte token if no config
  -> Constraint: acceptor is shared across all spawn phases (explicit + auto-load)
- [ ] `internal/component/plugin/ipc/tls_test.go` (593L) - 14 tests covering auth success/failure/timeout/malformed, acceptor lifecycle, per-client secrets
  -> Constraint: existing tests must continue to pass
- [ ] `internal/component/plugin/types.go` - HubServerConfig{Host, Port, Secret, Clients}, HubConfig{Servers, Clients}
  -> Constraint: HubServerConfig.Clients already provides per-name secret lookup for fleet

**Behavior to preserve:**
- `Authenticate()` function signature and behavior for non-name-bound callers (fleet uses `AuthenticateWithLookup`)
- `AuthenticateWithLookup()` fallback from per-client to shared secret
- `SendAuth()` frame format: `#0 auth {"token":"...","name":"..."}`
- `PluginAcceptor.Token()` API (used by `startExternal`)
- `GenerateSelfSignedCert()` ephemeral cert generation
- Constant-time comparison for tokens
- `validPluginName` regex validation
- 10-second auth timeout in acceptor `handleConn`
- All existing tests in `tls_test.go`
- Internal plugin path (net.Pipe) is completely unaffected

**Behavior to change:**
- Engine generates a unique random token per plugin name (not one shared token)
- Engine passes expected name alongside expected token to `Authenticate` so it can verify name binding
- `env.Get()` for `Secret: true` vars calls `os.Unsetenv` after first read (value stays in env cache)
- SDK verifies server cert fingerprint from `ZE_PLUGIN_CERT_FP` env var during TLS handshake
- `InsecureSkipVerify` only used when `ZE_PLUGIN_CERT_FP` is not set (backwards compat for manual external plugins)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config: `plugin { hub { server local { host 127.0.0.1; port 12700; secret "..."; } } }` parsed into `HubServerConfig`
- Runtime (new): engine generates per-plugin token + cert fingerprint at process creation time

### Transformation Path
1. `ensureAcceptor` generates ephemeral TLS cert and starts listener (unchanged)
2. `ensureAcceptor` computes cert SHA-256 fingerprint from the DER-encoded cert (new)
3. `PluginAcceptor` stores per-plugin token map: name -> random token (new)
4. `startExternal` gets token for this plugin name from acceptor (changed: per-plugin, not shared)
5. Token passed via `ZE_PLUGIN_HUB_TOKEN` env var (unchanged), but `env.Get()` auto-clears it from OS env after first read because `Secret: true` (new)
6. Cert fingerprint passed via `ZE_PLUGIN_CERT_FP` env var (new)
7. Plugin SDK uses cert fingerprint for TLS verification (new: `VerifyConnection` callback)
8. Plugin connects via TLS, sends `#0 auth {"token":"...","name":"..."}` (unchanged format)
9. Engine validates: token matches per-plugin expected token AND name matches expected name for that token (changed)
10. On success: connection routed to WaitForPlugin caller (unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin (token) | Env var `ZE_PLUGIN_HUB_TOKEN` (auto-cleared after read) | [ ] |
| Engine -> Plugin (cert FP) | Env var `ZE_PLUGIN_CERT_FP` | [ ] |
| Plugin -> Engine (auth) | `#0 auth {"token":"...","name":"..."}` over TLS (unchanged) | [ ] |

### Integration Points
- `PluginAcceptor` -- add `TokenForPlugin(name)` method to generate/return per-plugin tokens
- `PluginAcceptor` -- add `CertFingerprint()` method to return hex-encoded SHA-256 of server cert
- `startExternal` -- pass per-plugin token + cert fingerprint via env vars
- `NewFromTLSEnv` -- use cert fingerprint for TLS verification
- `Authenticate` -- add name binding: accept `expectedName` parameter, verify `params.Name == expectedName`
- `env` package -- add `Secret bool` to `EnvEntry`; `Get()` calls `os.Unsetenv` for `Secret: true` vars after first read
- Token registration changed to `Private: true, Secret: true`
- `env.MustRegister` -- register `ze.plugin.cert.fp` env var

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (auth is not hot path -- N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with external plugin | -> | Per-plugin token generated, written to file, plugin reads and authenticates | `test/plugin/tls-per-plugin-token.ci` |
| SDK connects to engine with cert FP | -> | TLS handshake verifies fingerprint | `TestCertFingerprintVerification` in `tls_test.go` |
| Plugin A's token used with plugin B's name | -> | Auth rejected (name binding) | `TestPerPluginTokenNameBinding` in `tls_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Engine spawns external plugin | Plugin receives a unique random token (different from other plugins) via `ZE_PLUGIN_HUB_TOKEN` |
| AC-2 | `env.Get()` on a `Secret: true` var | Value returned from cache; `os.Unsetenv` called so var no longer in `/proc/<pid>/environ` |
| AC-3 | Plugin sends auth with correct per-plugin token and matching name | Auth succeeds, connection established |
| AC-4 | Plugin sends auth with correct per-plugin token but wrong name | Auth rejected (name binding violation) |
| AC-5 | Plugin sends auth with another plugin's token | Auth rejected (wrong token for this name) |
| AC-6 | `ZE_PLUGIN_CERT_FP` is set | SDK verifies server cert SHA-256 matches; `InsecureSkipVerify` is NOT set |
| AC-7 | `ZE_PLUGIN_CERT_FP` is set but cert doesn't match | TLS handshake fails (connection refused) |
| AC-8 | `ZE_PLUGIN_CERT_FP` is not set | SDK falls back to `InsecureSkipVerify` (backwards compat for manual external plugins) |
| AC-9 | Internal plugins (net.Pipe) | Completely unaffected -- no TLS, no tokens |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerPluginTokenNameBinding` | `internal/component/plugin/ipc/tls_test.go` | AC-4: correct token + wrong name rejected | |
| `TestPerPluginTokenWrongToken` | `internal/component/plugin/ipc/tls_test.go` | AC-5: another plugin's token rejected | |
| `TestCertFingerprintComputation` | `internal/component/plugin/ipc/tls_test.go` | Cert fingerprint is stable SHA-256 hex of DER cert | |
| `TestCertFingerprintVerification` | `internal/component/plugin/ipc/tls_test.go` | AC-6: TLS connects when fingerprint matches | |
| `TestCertFingerprintMismatch` | `internal/component/plugin/ipc/tls_test.go` | AC-7: TLS refuses when fingerprint doesn't match | |
| `TestCertFingerprintFallback` | `internal/component/plugin/ipc/tls_test.go` | AC-8: InsecureSkipVerify when no fingerprint | |
| `TestSecretEnvVarCleared` | `internal/core/env/env_test.go` | AC-2: Secret var removed from OS env after first Get() | |
| `TestSecretEnvVarCachedAfterClear` | `internal/core/env/env_test.go` | AC-2: subsequent Get() still returns cached value | |
| `TestTokenForPluginUniqueness` | `internal/component/plugin/ipc/tls_test.go` | AC-1: different plugins get different tokens | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no new numeric inputs. Token is a string, fingerprint is a hex string.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tls-per-plugin-token` | `test/plugin/tls-per-plugin-token.ci` | External plugin starts with per-plugin token via temp file, authenticates, completes startup | |

### Future (if deferring any tests)
- Certificate rotation (cert expires after 24h, new cert on restart) -- out of scope, existing behavior

## Files to Modify
- `internal/core/env/registry.go` - Add `Secret bool` field to `EnvEntry`
- `internal/core/env/env.go` - `Get()` calls `os.Unsetenv` for `Secret: true` vars after first read
- `internal/component/plugin/ipc/tls.go` - Add `AuthenticateWithName`, `CertFingerprint`, `TokenForPlugin` functions; modify `PluginAcceptor` to track per-plugin tokens and cert fingerprint
- `pkg/plugin/sdk/sdk.go` - Modify `NewFromTLSEnv` to verify cert fingerprint; update token registration to `Secret: true`; register `ze.plugin.cert.fp`
- `internal/component/plugin/process/process.go` - Modify `startExternal` to pass per-plugin token + `ZE_PLUGIN_CERT_FP` env vars
- `internal/component/plugin/manager/manager.go` - Pass cert to acceptor for fingerprint computation; remove shared `Token()` usage in favor of per-plugin tokens

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | No | - |
| CLI usage/help text | No | - |
| API commands doc | No | - |
| Plugin SDK docs | Yes | `docs/plugin-development/README.md` -- update env var documentation |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/plugin/tls-per-plugin-token.ci` |

## Files to Create
- `test/plugin/tls-per-plugin-token.ci` - Functional test: external plugin with per-plugin token auth

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/README.md` -- document `ZE_PLUGIN_CERT_FP` env var, note token auto-cleared from env |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `.claude/rules/plugin-design.md` Transport section -- document per-plugin tokens and cert pinning; `docs/architecture/api/process-protocol.md` -- update auth section; `.claude/rules/go-standards.md` -- document `Secret` field on `EnvEntry`; `docs/architecture/config/environment.md` -- document `Private` vs `Secret` distinction |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Per-plugin token generation and name-bound auth** -- Engine-side changes
   - Add `AuthenticateWithName(ctx, conn, expectedToken, expectedName)` to `tls.go`
   - Add `TokenForPlugin(name)` to `PluginAcceptor` (generates and stores per-plugin random tokens)
   - Modify `PluginAcceptor.handleConn` to validate name binding when per-plugin tokens are in use
   - Tests: `TestPerPluginTokenNameBinding`, `TestPerPluginTokenWrongToken`, `TestTokenForPluginUniqueness`
   - Files: `tls.go`, `tls_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Auto-clear secret env vars** -- env package enhancement
   - Add `Secret bool` field to `EnvEntry` struct
   - Modify `Get()` to call `os.Unsetenv` after first read when `Secret: true` (value stays in cache)
   - Update token registration: `Private: true, Secret: true`
   - Tests: `TestSecretEnvVarCleared`, `TestSecretEnvVarCachedAfterClear`
   - Files: `internal/core/env/registry.go`, `internal/core/env/env.go`, `internal/core/env/env_test.go`, `pkg/plugin/sdk/sdk.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Certificate fingerprint pinning** -- Cert verification
   - Add `CertFingerprint(cert tls.Certificate)` to `tls.go` (SHA-256 of DER, hex-encoded)
   - Store fingerprint in acceptor, add `CertFingerprint()` getter
   - Modify `startExternal` to pass `ZE_PLUGIN_CERT_FP` env var
   - Modify `NewFromTLSEnv` to build `tls.Config` with `VerifyConnection` callback when fingerprint is set
   - Register `ze.plugin.cert.fp` via `env.MustRegister`
   - Tests: `TestCertFingerprintComputation`, `TestCertFingerprintVerification`, `TestCertFingerprintMismatch`, `TestCertFingerprintFallback`
   - Files: `tls.go`, `sdk.go`, `process.go`, `manager.go`, `tls_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Manager wiring** -- Connect all pieces
   - Pass cert to acceptor constructor (for fingerprint computation)
   - Modify `startExternal` to get per-plugin token from acceptor + cert fingerprint
   - Update `manager.go` `ensureAcceptor` to pass cert
   - Files: `manager.go`, `process.go`
   - Verify: existing manager tests still pass

5. **Functional tests** -- Create `test/plugin/tls-per-plugin-token.ci` after feature works
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Token comparison still uses constant-time; fingerprint is SHA-256 of raw DER; temp file has 0600 perms |
| Naming | Env vars follow `ze.plugin.*` pattern; function names describe action not destination |
| Data flow | Token flows: engine generates -> temp file -> plugin reads -> plugin sends -> engine verifies against per-plugin store |
| Rule: no-layering | `InsecureSkipVerify` path retained only for backwards compat when `ZE_PLUGIN_CERT_FP` is absent |
| Rule: go-standards | New env vars registered via `env.MustRegister`; no `os.Getenv` for Ze vars |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Per-plugin tokens generated | `TestTokenForPluginUniqueness` passes |
| Name binding enforced | `TestPerPluginTokenNameBinding` passes |
| Secret env vars auto-cleared | `TestSecretEnvVarCleared` passes |
| Cert fingerprint pinning | `TestCertFingerprintVerification` passes |
| Backwards compat (no FP) | `TestCertFingerprintFallback` passes |
| Functional test exists | `ls test/plugin/tls-per-plugin-token.ci` |
| All existing tests pass | `make ze-verify` clean |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Token entropy | Per-plugin tokens must use `crypto/rand`, at least 32 bytes |
| Constant-time comparison | All token comparisons use `subtle.ConstantTimeCompare` |
| Secret env auto-clear | `env.Get()` calls `os.Unsetenv` for `Secret: true` vars; verify with `os.Getenv` returning empty after first read |
| Fingerprint algorithm | SHA-256 of raw DER cert bytes (not PEM, not the public key alone) |
| Env var leakage | Token cleared from OS env after first read; cert FP in env is not secret |
| Timing oracle | Name validation before token comparison? No -- validate name format, then do token comparison (constant-time), then check name binding |
| Error messages | Auth errors must not leak expected token or expected name |

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

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

- `AuthenticateWithLookup` already exists and does per-name secret lookup, but only for fleet clients. The per-plugin token feature extends the same pattern to forked plugins.
- The `PluginAcceptor.handleConn` already routes by name via `pending` map. Name binding adds a second check: the token itself is tied to the name.
- `tls.Config.VerifyConnection` callback runs after TLS handshake completes but before any application data. It receives `tls.ConnectionState` with `PeerCertificates` (server certs). This is the right hook for fingerprint verification -- no need to implement a custom `VerifyPeerCertificate`.
- `EnvEntry.Private` means "hidden from user-facing output" (e.g., `ze env list`). `EnvEntry.Secret` means "contains sensitive data; cleared from OS environment after first read." A var can be both (token is `Private: true, Secret: true`). Cert fingerprint is neither (not secret, shown to user).

## RFC Documentation

N/A -- internal transport security, not BGP protocol.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
