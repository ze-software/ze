# Spec: managed-server-hardening

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 |
| Updated | 2026-09-05 |

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

### Certificate decision (2026-08-29)

Two shapes were on the table for AC-1/AC-2. **The hub serves a pki store
certificate (shape a).**

| Shape | Verdict |
|-------|---------|
| (a) The hub serves a certificate from the pki component, and the client verifies it | **Picked.** `pki.ServerTLSMaterial` already resolves a certificate NAME into serving PEM, and the web, DoT and DoH listeners already take it that way. The managed listener was the only TLS server in ze that could not be given a certificate. The hub gains a `certificate` leaf and nothing else |
| (b) The hub keeps a generated self-signed certificate and persists it, and the client pins its fingerprint | **Rejected.** Persisting the generated pair means a second certificate store beside pki: key material, permissions, regeneration when it expires, and a command to read the fingerprint back. It also leaves the operator with a certificate no CA issued and no way to rotate it through config |

The fingerprint pin stays, on the CLIENT side, as the trust anchor for a hub
certificate no CA in the client's trust store issued. That is what a private
fleet CA produces, and it is the case a system CA pool cannot serve. Pinning is
only meaningful because shape (a) makes the served certificate stable: an
ephemeral certificate changes on every restart, so a pin on it would break at
the first restart. The mechanism is `pluginipc.TLSConfigWithFingerprint`, which
the plugin process rail already uses through `ZE_PLUGIN_CERT_FP`.

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
- A managed client can verify the hub's identity without `tls-insecure` (CA cert or pinned fingerprint). DONE 2026-08-29.

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
| managed client connects with cert verification (no tls-insecure) | → | `clientTLSConfig` validates the hub chain against the pki ca the client names | `test/managed/managed-hub-ca-trust.ci` |
| a server block declares managed clients on the plugin acceptor's address | → | `diagnoseManagedListener` reports `doctor-hub-managed-collision` | `test/ui/doctor-hub-managed-collision.ci`, and `test/ui/doctor-hub-managed-separate.ci` for the control |

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

## AC-4 config-changed: no producer (2026-09-05)

The hub pushes `config-changed` from a storage write observer, and the observer
fires only inside `blobStorage.WriteFile`
(`internal/component/config/storage/blob.go`), which means only for a write
through the running hub's OWN store. No non-test caller writes
`ClientConfigKey(name)`: `ze data`, `ze config import` and `ze config edit` each
open the blob file in a separate process, and a running hub answers every read
from the tree `zefs.BlobStore` loaded when it opened the store
(`BlobStore.readFile`, `pkg/zefs/store.go`). Every in-daemon editor takes one
config path, the daemon's own (`cli.NewEditorWithStorage` callers:
`cmd/ze/hub/session_editor.go`, `cmd/ze/hub/editor_adapter.go`,
`cmd/ze/hub/api.go`, `cmd/ze/hub/service_gnmi.go`).

So `ze_managed_config_changed_pushed_total` can only ever count pushes an
in-process test produced, and an operator changing a client's config has to stop
the hub, write the blob, and start it again.

The question for the owner is which way to fix it, not whether:

| Option | Shape |
|--------|-------|
| (a) A hub-side command that writes a managed client's config through the running daemon | New CLI surface plus its YANG. It makes the documented workflow real, and AC-4 then has a producer to drive |
| (b) The in-daemon editor accepts a second config path, the client blob | Reuses the editor, the draft, the history and the validation the operator already has. It widens `NewEditorWithStorage`'s single-path assumption across four call sites |

The documentation was repaired in this work rather than left claiming the
workflow: `docs/architecture/fleet-config.md` "Config Storage (Hub Side)" and
`docs/guide/fleet-config.md` "Config Management" now say the hub must be stopped.

## Files to Modify
- `internal/component/plugin/server/managed_serve.go` - serves the named pki certificate, fails closed, reports its fingerprint (DONE)
- `internal/component/managed/tls.go` - the client trust decision (DONE)
- `internal/component/managed/client.go` - `CertificateFingerprint` on ClientConfig (DONE)
- `internal/component/plugin/yang/ze-plugin-conf.yang` - `certificate` and `certificate-fingerprint` leaves (DONE)
- `internal/component/plugin/types.go`, `internal/component/config/loader_extract.go` - extraction (DONE)
- `docs/architecture/fleet-config.md` - hub certificate and client trust (DONE)
- `internal/component/plugin/doctor/check_managed_listener.go` and `register.go` + `internal/core/diagnostic/codes.go` (AC-3, DONE)
- `test/ui/doctor-hub-managed-collision.ci`, `test/ui/doctor-hub-managed-separate.ci` (AC-3, DONE)
- `docs/guide/health-checks.md`, `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md` (DONE)

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
- [ ] `./le verify worktree` passes
