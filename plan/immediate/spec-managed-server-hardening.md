# Spec: managed-server-hardening

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | - |
| Phase | - |
| Updated | 2026-09-19 |
| Blocker | Owner choice for the AC-4 in-daemon client-config writer |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. The managed-hub-server record (retired with the learned corpus) - the managed server this hardens
4. `internal/component/plugin/server/managed_serve.go`, `internal/component/managed/client.go`

## Task

This spec owns secure trust for the managed-config hub, a port-collision doctor
check, and a two-daemon proof of both config fetch and change notification.
The trust path, doctor check and fetch proof are implemented, as recorded in
Remaining Work. AC-4's change-notification half remains blocked on the
operator-reachable writer described below.

1. **Secure server-cert verification.** Keep the implemented PKI certificate
   path and client CA verification. The original self-signed-only listener
   required `tls-insecure`; `startManagedServer` now supplies the named
   certificate resolver and local authority to the listener.
2. **Port-collision doctor check.** When a `server` block with `client` entries shares an address
   with the plugin acceptor's bound block, the managed listener cannot bind (handled gracefully:
   it is skipped with an Error log), but managed clients on that block are then dropped by the
   plugin acceptor. Add a `ze doctor` check that flags a server block declaring BOTH plugin usage
   and `client` entries (or a managed block whose address collides with the acceptor).
3. **Two-instance daemon `.ci`.** Add a `test/managed/*.ci` that runs a real `ze` hub serving a
   real `ze` managed client end-to-end (fetch + config-changed), complementing the Go integration
   test `cmd/ze/hub/managed_server_test.go` which exercises `startManagedServer` directly.

### Certificate decision (2026-08-29)

Two shapes were on the table for AC-1/AC-2. **The hub serves a pki store
certificate (shape a).**

| Shape | Verdict |
|-------|---------|
| (a) The hub serves a certificate from the pki component, and the client verifies it | **Picked.** `pki.ServerTLSMaterial` already resolves a certificate NAME into serving PEM, and the web, DoT and DoH listeners already take it that way. The managed listener was the only TLS server in ze that could not be given a certificate. The hub gains a `certificate` leaf and nothing else |
| (b) The hub keeps a generated self-signed certificate and persists it, and the client pins its fingerprint | **Rejected.** Persisting the generated pair means a second certificate store beside pki: key material, permissions, regeneration when it expires, and a command to read the fingerprint back. It also leaves the operator with a certificate no CA issued and no way to rotate it through config |

The August 29 design also proposed a client fingerprint pin. The implementation
recorded on September 5 instead uses `plugin/hub/client/ca` and
`ClientConfig.CA`. `clientTLSConfig` resolves that named PKI CA and refuses an
unresolved name; with no named CA it uses the system trust pool unless the
operator explicitly selects `TLSInsecure`. The current contract and remaining
proof use that CA path.

The defect this closes: `NewManagedServer` always minted a 24-hour self-signed
certificate whose only SAN was 127.0.0.1, and `runConnection` verified against
the system CA pool. No real deployment could connect without
`ze.managed.tls.insecure`, and with it the client sent its token to whatever
answered on the hub address.

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
- [ ] `docs/architecture/api/process-protocol.md` - the plugin hub's own TLS model, declared as the design document by `internal/component/plugin/types.go`
  → Constraint: "Plugin Transport / External Plugins (TLS connect-back)" has the engine create a `PluginAcceptor` holding the certificate authority root that issued the served certificate, and has the forked child validate the engine chain against that root, carried to it as `ZE_PLUGIN_CA_PEM`. The managed listener's trust decision is a second answer to the same question, so this spec states where it diverges rather than inventing a model.
- [ ] The managed-hub-server record (retired with the learned corpus) - what was built and why the dedicated listener

### RFC Summaries (MUST for protocol work)
- [ ] N/A - TLS/cert handling, not an IETF wire protocol.

**Key insights:**
- The managed client verifies against a named PKI CA or the system trust pool; the plugin SDK's fingerprint mechanism is a separate surface.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed_serve.go` - `managedCertificate` serves a named certificate through the injected resolver, or a leaf issued by the injected authority; it refuses missing material.
- [ ] `internal/component/managed/tls.go` - `clientTLSConfig` verifies against a named PKI CA, otherwise uses explicit insecure mode or the system trust pool.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` supplies the certificate resolver and authority and installs the client-config write observer.

**Behavior to preserve:**
- The dedicated managed listener, per-client secret auth, config-fetch/ack/ping, and config-changed push all continue to work unchanged.
- `tls-insecure` remains a valid opt-in for development.

**Behavior to change:**
- A managed client can verify the hub's identity without `tls-insecure` (CA cert or pinned fingerprint). DONE 2026-08-29.

## Data Flow (MANDATORY)

### Entry Point
- Managed client TLS dial to the hub's managed listener (`client.go` `runConnection`).

### Transformation Path
1. The hub serves the named PKI certificate or a leaf issued by its authority.
2. The client resolves its configured PKI CA, or uses the system trust pool.
3. TLS verifies the chain and server name before the client sends its token, unless insecure mode was explicitly selected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub cert material ↔ managed listener | PKI/cert-store load or config-provided cert | [ ] |
| Hub cert identity ↔ client trust | CA chain through the client's named PKI CA or system trust pool | [ ] |

### Integration Points
- `ManagedServerConfig`, `managed.clientTLSConfig`, the hub PKI store, and the `plugin/hub/client/ca` leaf are implemented. The remaining integration is an operator-reachable write through the running hub's store.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| managed client connects with cert verification (no tls-insecure) | → | `clientTLSConfig` validates the hub chain against the pki ca the client names | `test/managed/managed-hub-ca-trust.ci` |
| a server block declares managed clients on the plugin acceptor's address | → | `diagnoseManagedListener` reports `doctor-hub-managed-collision` | `test/ui/doctor-hub-managed-collision.ci`, and `test/ui/doctor-hub-managed-separate.ci` for the control |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Managed client with the hub's CA trusted through the named PKI CA or system pool and `tls-insecure` off | Connects and fetches config; TLS verification passes |
| AC-2 | Managed client with a wrong/absent trust anchor and `tls-insecure` off | Connection refused (verification fails), not silently accepted |
| AC-3 | A `server` block declares both a shared secret (plugins) and `client` entries | `ze doctor` reports a collision/misconfiguration code |
| AC-4 | Real `ze` hub + real `ze` client | End-to-end `config-fetch` + `config-changed` through both daemons |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManagedClientPinsCertificate` | `internal/component/managed/client_tls_test.go` | AC-1 | PASS |
| `TestManagedClientRefusesWrongCertificate` | `internal/component/managed/client_tls_test.go` | AC-2 | PASS |
| `TestManagedClientDefaultFailsClosed` | `internal/component/managed/client_tls_test.go` | AC-2 | PASS |
| `TestManagedClientFingerprintSources` | `internal/component/managed/client_tls_test.go` | AC-1 (env override, hex case) | PASS |
| `TestManagedServerServesConfiguredCertificate` | `internal/component/plugin/server/managed_cert_test.go` | AC-1 | PASS |
| `TestManagedServerFailsClosedOnCertificate` | `internal/component/plugin/server/managed_cert_test.go` | AC-1 | PASS |
| `TestHubTrustLeavesReachTheStructs` | `internal/component/config/hub_certificate_test.go` | AC-1 (both leaves) | PASS |
| `TestHubFingerprintRejectsNonHex` | `internal/component/config/hub_certificate_test.go` | AC-1 (leaf pattern) | PASS |
| `TestHubRefusesDisagreeingCertificates` | `internal/component/config/hub_certificate_test.go` | AC-1 (one certificate per managed server) | PASS |
| `TestManagedListenerCollisionCases` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (each address case, and the documented central hub that must stay silent) | PASS |
| `TestManagedListenerCollisionNamesBothBlocks` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (the message names the block that cannot serve and the one that took the address) | PASS |
| `TestManagedListenerCheckReadsTheConfigTree` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (config text through ExtractHubConfig to the verdict) | PASS |
| `TestManagedListenerCheckIgnoresAForeignTree` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (no tree, a foreign tree, a typed nil tree) | PASS |

Each test was proved to discriminate by reverting the behavior it covers: the
pin branch removed, the pin branch replaced by `InsecureSkipVerify`, the default
branch made insecure, the env lookup and the lowercasing dropped, the server
made to ignore its configured certificate name, the two extraction assignments
dropped, the YANG pattern dropped, and the agreement check dropped. Every
mutation turned exactly the covering test red.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `managed-hub-ca-trust` | `test/managed/managed-hub-ca-trust.ci` | two real daemons: the client verifies the hub chain against the exported root and fetches, and a foreign root does not (AC-1/AC-2/AC-4 fetch half) | PASS |
| `doctor-hub-managed-collision` | `test/ui/doctor-hub-managed-collision.ci` | `ze doctor --json` reports the collision (AC-3) | PASS |
| `doctor-hub-managed-separate` | `test/ui/doctor-hub-managed-separate.ci` | `ze doctor --json` stays silent on the central hub that works (AC-3 control) | PASS |
| AC-4 `config-changed` half | none | BLOCKED: no operator-reachable producer, see "AC-4 config-changed: no producer" |

## Remaining Work (2026-09-05)

| Item | State |
|------|-------|
| AC-1/AC-2 mechanism and tests | Done, in `internal/` |
| Hub wiring: `cmd/ze/hub/managed_server.go` passes `Certificate: blk.Certificate`, `TLSMaterialResolver: zepki.ServerTLSMaterial` and `Authority: caRoot` into `ManagedServerConfig` | DONE. Verified in `startManagedServer` |
| Client wiring: `cmd/ze/ze_core_start.go` `extractManagedClientConfig` passes `CA: cli.CA` into `managed.ClientConfig` | DONE. The design landed as a pki ca anchor rather than a fingerprint pin, so the leaf is `plugin/hub/client/ca` and `ClientConfig.CA` reads it |
| First boot: `fetchInitialConfig` (`cmd/ze/ze_core_start.go`) calls `managed.ClientTLSConfig` rather than building a second `tls.Config` | DONE |
| AC-3 port-collision doctor check | DONE. `internal/component/plugin/doctor/check_managed_listener.go`, code `doctor-hub-managed-collision`, `test/ui/doctor-hub-managed-collision.ci` and `test/ui/doctor-hub-managed-separate.ci` |
| AC-4 two-instance daemon `.ci` | HALF DONE, and the other half is BLOCKED. `test/managed/managed-hub-ca-trust.ci` runs a real `ze` hub and a real `ze` client and proves `config-fetch` end to end, with a foreign-root control. `config-changed` cannot be reached from two daemons: the push fires only on a write through the RUNNING hub's own store, and no operator command performs one. See "AC-4 config-changed: no producer" below |

## AC-4 config-changed: no operator-reachable writer

The 2026-09-05 investigation found that notifications fired only for writes
through the running hub's own blob store. The storage representation has since
changed: `storage.store.WriteFile` writes through the owned tree, and
`guard.Release` invokes the observer after durability and unlock
(`internal/component/config/storage/store.go`). The old claim that the running
store is a `zefs.BlobStore` snapshot no longer describes this path.

The missing producer remains. `startManagedServer`
(`cmd/ze/hub/managed_server.go`) installs the write observer and reads
`ClientConfigKey(name)`, but the callers that write that key are still tests.
The running hub needs an operator entry point that writes a managed client's
configuration through this store. The recorded fetch proof does not prove
that notification path or AC-4 as a whole.

The question for the owner is which way to fix it, not whether:

| Option | Shape |
|--------|-------|
| (a) A hub-side command that writes a managed client's config through the running daemon | New CLI surface plus its YANG. It makes the documented workflow real, and AC-4 then has a producer to drive |
| (b) The in-daemon editor accepts a second config path, the client blob | Reuses the editor, the draft, the history and the validation the operator already has. It widens `NewEditorWithStorage`'s single-path assumption across four call sites |

The documentation was repaired in this work rather than left claiming the
workflow: `docs/architecture/fleet-config.md` "Config Storage (Hub Side)" and
`docs/guide/fleet-config.md` "Config Management" now say the hub must be stopped.

## Files to Modify
- `internal/component/plugin/server/managed_serve.go` - serves the named PKI certificate or authority-issued leaf and fails closed (DONE)
- `internal/component/managed/tls.go` - the client trust decision (DONE)
- `internal/component/managed/client.go` - `CA` on `ClientConfig` (DONE)
- `internal/component/plugin/yang/ze-plugin-conf.yang` - the server `certificate` and client `ca` leaves (DONE)
- `internal/component/plugin/types.go`, `internal/component/config/loader_extract.go` - extraction (DONE)
- `docs/architecture/fleet-config.md` - hub certificate and client trust (DONE)
- `internal/component/plugin/doctor/check_managed_listener.go` and `register.go` + `internal/core/diagnostic/codes.go` (AC-3, DONE)
- `test/ui/doctor-hub-managed-collision.ci`, `test/ui/doctor-hub-managed-separate.ci` (AC-3, DONE)
- `docs/guide/health-checks.md`, `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md` (DONE)

## Implementation Steps
1. Keep the implemented PKI/CA trust path; the remaining owner decision is the client-config writer.
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
- [ ] `./le verify worktree` passes
