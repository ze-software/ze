# Spec: managed-server-hardening

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. The managed-hub-server record (retired with the learned corpus) - the managed server this hardens
4. `internal/component/plugin/server/managed_serve.go`, `internal/component/managed/client.go`

## Task

Follow-up hardening for the managed-config hub server (landed by `spec-managed-hub-server`),
capturing three gaps surfaced by that spec's `/ze-review`:

1. **Secure server-cert verification.** `ManagedServer` presents a self-signed cert
   (`managed_serve.go` `GenerateSelfSignedCert`). A remote managed client
   (`internal/component/managed/client.go`) verifies against the system CA via `ServerName`
   unless `TLSInsecure`, so today it can only connect with `tls-insecure`. Add a verifiable
   path: either serve the hub's PKI/CA cert, or carry the hub cert fingerprint in the client
   config block and pin it (mirror the plugin SDK's `ze.plugin.cert.fp` pinning). This restores
   the "server certificate verification is the default" posture documented in `fleet-config.md`.
2. **Port-collision doctor check.** When a `server` block with `client` entries shares an address
   with the plugin acceptor's bound block, the managed listener cannot bind (handled gracefully:
   it is skipped with an Error log), but managed clients on that block are then dropped by the
   plugin acceptor. Add a `ze doctor` check that flags a server block declaring BOTH plugin usage
   and `client` entries (or a managed block whose address collides with the acceptor).
3. **Two-instance daemon `.ci`.** Add a `test/managed/*.ci` that runs a real `ze` hub serving a
   real `ze` managed client end-to-end (fetch + config-changed), complementing the Go integration
   test `cmd/ze/hub/managed_server_test.go` which exercises `startManagedServer` directly.

### Post-wave corrections (2026-07-10)

New obligation from the 2026-07 implementation wave (verified against current code): the
plugin RPC connection layer both managed endpoints use gained write-timeout behavior.
`pkg/plugin/rpc/conn.go` applies a default 30s write deadline when the context carries none
(`defaultWriteDeadline`, conn.go; applied in `writeAppended`, conn.go, :309) and
arms a fail-fast write watchdog on transports without `SetWriteDeadline` (armed in `NewConn`
at conn.go; `fireWatchdog` closes the connection, conn.go), with the
`ze_plugin_write_watchdog_total` counter wired in
`internal/component/plugin/server/server.go` and documented in
`docs/plugin-development/metrics.md`.

Relevance to this spec: the managed client wraps its TLS conn in `rpc.NewConn` at
`internal/component/managed/client.go` and the hub's managed listener does the same at
`internal/component/plugin/server/managed_serve.go`. Both are deadline-capable TLS
`net.Conn`s, so they take the 30s-deadline path and the watchdog timer never arms for them
(transport selection at conn.go); the new counter therefore does not observe managed
connections. Obligations: (1) the cert-verification rework (AC-1/AC-2) must keep wrapping the
verified TLS conn in `rpc.NewConn` so the deadline behavior is preserved; (2) the
two-instance daemon `.ci` (AC-4) will implicitly exercise the deadline write path end to end,
and a hang there should be read against this new fail-fast behavior rather than assumed to be
an indefinite block.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` - security section (cert verification default), roles
  → Constraint: cert verification is the documented default; the current self-signed cert breaks it.
- [ ] The managed-hub-server record (retired with the learned corpus) - what was built and why the dedicated listener

### RFC Summaries (MUST for protocol work)
- [ ] N/A - TLS/cert handling, not an IETF wire protocol.

**Key insights:**
- The plugin SDK already pins the acceptor cert via `ze.plugin.cert.fp`; remote managed clients need an analogous config-carried fingerprint or a CA-signed cert.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed_serve.go` - `NewManagedServer` generates a self-signed cert; no fingerprint is surfaced to clients (CertFP was removed as dead code).
- [ ] `internal/component/managed/client.go` - `runConnection` builds `tls.Config{ServerName, InsecureSkipVerify: cfg.TLSInsecure}`; no cert pinning path.
- [ ] `internal/component/plugin/ipc/tls.go` - `CertFingerprint`, `GenerateSelfSignedCert`; the plugin-side pinning pattern to mirror.

**Behavior to preserve:**
- The dedicated managed listener, per-client secret auth, config-fetch/ack/ping, and config-changed push all continue to work unchanged.
- `tls-insecure` remains a valid opt-in for development.

**Behavior to change:**
- A managed client can verify the hub's identity without `tls-insecure` (CA cert or pinned fingerprint).

## Data Flow (MANDATORY)

### Entry Point
- Managed client TLS dial to the hub's managed listener (`client.go` `runConnection`).

### Transformation Path
1. Hub loads/serves a verifiable cert (PKI/CA cert, or self-signed with a published fingerprint).
2. Client obtains the trust anchor (CA in trust store, or `cert-fp` in its config block).
3. Client TLS verifies the server cert (CA chain or pinned fingerprint) before auth.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub cert material ↔ managed listener | PKI/cert-store load or config-provided cert | [ ] |
| Hub cert identity ↔ client trust | CA chain or `cert-fp` carried in client config | [ ] |

### Integration Points
- `ManagedServerConfig` (add an optional cert), `internal/component/managed/client.go` (pinning), the hub PKI/cert store, and the `plugin/hub/client` YANG block (a `cert-fp` leaf).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| managed client connects with cert verification (no tls-insecure) | → | client TLS pin/CA verify against the hub cert | `test/managed/managed-hub-secure.ci` |
| server block with plugins + client entries | → | doctor check flags the collision | doctor functional test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Managed client with the hub cert-fp/CA and `tls-insecure` off | Connects and fetches config; TLS verification passes |
| AC-2 | Managed client with a wrong/absent trust anchor and `tls-insecure` off | Connection refused (verification fails), not silently accepted |
| AC-3 | A `server` block declares both a shared secret (plugins) and `client` entries | `ze doctor` reports a collision/misconfiguration code |
| AC-4 | Real `ze` hub + real `ze` client | End-to-end `config-fetch` + `config-changed` through both daemons |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManagedClientPinsCert` | `internal/component/managed/client_test.go` | AC-1/AC-2 | |
| `TestManagedServerCollisionDoctor` | doctor check test | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `managed-hub-secure` | `test/managed/*.ci` | client verifies the hub cert and fetches (AC-1/AC-4) | |

## Files to Modify
- `internal/component/plugin/server/managed_serve.go` - accept an optional/PKI cert
- `internal/component/managed/client.go` - cert pinning / CA verification
- `internal/component/plugin/*/yang/` - `cert-fp` leaf on the hub client block (if pinning)
- doctor check registration + `internal/core/diagnostic/codes.go`
- `test/managed/managed-hub-secure.ci`

## Implementation Steps
1. Decide the cert approach (PKI/CA cert vs pinned fingerprint) - present to user.
2. Serve a verifiable cert from the managed listener.
3. Wire client-side verification.
4. Add the port-collision doctor check + diagnostic code.
5. Add the two-instance functional `.ci`.
6. Full verification + docs.

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
- [ ] Cert approach decided with the user
- [ ] Secure connection works without `tls-insecure`
- [ ] Doctor check registered
- [ ] Two-instance `.ci` passes
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` passes
