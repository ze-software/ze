# Spec: fixit-eap-tls-escape-hatch-kills-the-daemon

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/4 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze's shipped instruction to an operator now stops the daemon from starting, and
the capability it enabled has no replacement.**

`cmd/ze/main.go` carries the reasoning for a deliberate fail-closed choice and
ends with the escape hatch: an operator who must talk to such a peer is told to
set the `tlsunsafeekm` GODEBUG setting to its old value, "which is a decision
they take knowingly and can audit". The instruction is quoted here without its
assignment form on purpose: that form is what a reader pastes, and
`TestNoShippedGuidanceNamesARemovedGODEBUG` refuses it everywhere. The verbatim
text is preserved in `cmd/ze/testdata/godebug-guidance-defect.txt`, which is that
test's fixture.

Go 1.27 REMOVED that GODEBUG key. A removed key set to its old value is a fatal
error raised before `runtime.main`, so the process dies before any Ze code runs.
Measured on this host with Go 1.27.0:

    fatal error: removed GODEBUG "tlsunsafeekm" set to old value "1" in
    environment (https://go.dev/doc/godebug#go-127)

An operator who follows Ze's own guidance therefore gets a daemon that refuses to
start, with a message naming neither Ze nor EAP-TLS.

**Two things are broken, and the second is the one that matters.**

1. The instruction is wrong. That alone is a documentation fix.
2. **The capability is gone.** RFC 5216 Section 2.3 defines the EAP-TLS MSK as a
   `crypto/tls` ExportKeyingMaterial result. Go refuses that export on a TLS 1.2
   session that did not negotiate RFC 7627, so Ze cannot authenticate such a peer
   at all. strongSwan 5.9.14 is one: it lands on TLS 1.2 by DEFAULT and does not
   negotiate 7627. Correction, 2026-08-23, during implementation: that default is
   not a cap. charon ships `version_max = 1.2` with the comment "default to TLS
   1.2 until 1.3 is stable for use in EAP", and `charon.tls.version_max = 1.3` on
   the same 5.9.14 image reaches an established SA, which
   `test/interop-ipsec/scenarios/06-eap-tls13` and `25-responder-eap-tls13` both
   run. So the peer IS reachable, by one line of PEER config, and every shipped
   string that said otherwise was telling operators to give up on the cheapest
   remedy. Before Go 1.27 an operator could also opt in on the ze side. That half
   is gone with no replacement, and nothing in the tree said so.

**Why the original decision was right and must not simply be reversed.** A
`//go:debug tlsunsafeekm=1` line in `main.go` was written and removed on
2026-08-01. It sets the default for every Ze binary, so it would weaken the
export rule for every user to suit one peer version. The GODEBUG name says unsafe
for a reason: RFC 7627 exists to stop the triple handshake attack, and without it
exported keying material can collide across sessions. Restoring that line trades
every deployment's safety for one peer's compatibility, which is the trade the
owner already refused.

**Where it is measurable today.** `test/interop-ipsec/scenarios/04-eap-tls/ze-env`
sets the variable, so that scenario fails at `strongSwan SA 'ze' did not reach
ESTABLISHED within 90s` with no IKE packet sent. `06-eap-tls13` and
`25-responder-eap-tls13` pass, because TLS 1.3 needs none of this: the export is
always available and RFC 9190 supersedes RFC 5216.

Found while closing `spec-fixit-ipsec-interop-cli-credentials`, which it does not
block. The journal row is in `plan/journal/test-against-broken-path.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - the design doc `cmd/ze/main.go` declares
  → Constraint: one `main()` serves every Ze binary, so anything set there is set for all of them. That is why the package-level directive was refused.
- [ ] `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - records the 2026-08-01 decision and its reasoning
  → Decision: the `go:debug` directive was written and removed deliberately. Re-adding it is not a candidate fix.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5216.md` - EAP-TLS, Section 2.3, the MSK derivation
  → Constraint: the MSK IS an ExportKeyingMaterial result. A Ze that cannot export cannot authenticate, so this is a conformance gap and not a convenience.
- [ ] RFC 7627, Extended Master Secret, and the attack it prevents.
  **MUST CREATE before any design work in this spec** (`ai/rules/protocol.md`):
  the repository holds neither the full text nor a summary for that stem under
  `rfc/`, and no disposition for it in either `rfc/enrolled.txt` or
  `rfc/not-enrolled.txt`. Verified 2026-08-23. The two paths are deliberately not
  spelled here: they do not resolve, and naming them would read as rot to
  `make ze-doc-links-check` rather than as the gap this row records.
  → Constraint: the export refusal is not arbitrary. Without 7627 the exported material can collide across sessions.
  → Finding, and it is why this row is written this way: Ze's fail-closed EAP-TLS
    decision, the 2026-08-01 refusal of the package-level directive, and this
    spec's whole security argument all rest on an RFC the repository has never
    summarised. `rfc/short/rfc5216.md` and `rfc/short/rfc9190.md` both exist. The
    one that justifies REFUSING does not, so the reasoning for the refusal lives
    only in a Go comment.
- [ ] `rfc/short/rfc9190.md` - EAP-TLS 1.3
  → Constraint: it supersedes RFC 5216 for TLS 1.3 and needs no escape hatch. The gap is TLS 1.2 only.

**Key insights:**
- The safe default and the escape hatch were one decision. Go removed half of it and left the half that refuses.
- A capability that silently disappears with a toolchain bump is worse than one that was never offered, because the guidance describing it survives.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - the header comment stating the fail-closed choice and naming the GODEBUG escape hatch
- [ ] `internal/component/ike/eap/eap_tls.go` - `exportEAPTLSMSK` selects by negotiated TLS version and is where the refusal surfaces. Read 2026-08-23: the TLS 1.2 branch already wraps the failure as `eap-tls: export MSK (RFC 5216 Section 2.3): %w`, so the RFC is named and the CAUSE is not. AC-2 is therefore an increment on an existing message rather than a new one, and what it must add is the peer, the negotiated version and RFC 7627. The function's own doc comment records why it returns an error rather than a zero MSK, which is the same fail-closed reasoning AC-2 extends
- [ ] `test/interop-ipsec/scenarios/04-eap-tls/ze-env` - sets the removed key, so the daemon dies at container start

**Runtime evidence:**
- [ ] A Go 1.27.0 binary run with `tlsunsafeekm` set to its old value fatals before `runtime.main`. Reproduced 2026-08-23 with a two-line program, so the failure is the toolchain's and not Ze's.

**Behavior to preserve:**
- The fail-closed default. Ze must not weaken the export rule for every deployment.
- The TLS 1.3 path, which needs no escape hatch and is preferred.
- The 2026-08-01 decision refusing a package-level `//go:debug` directive.

**Behavior to change:**
- Ze must not instruct an operator to do something that stops the daemon.
- An operator meeting a TLS 1.2 peer without RFC 7627 must get an error that names the peer, the RFC and what their options are, rather than a `crypto/tls` refusal or a runtime fatal.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator configures an EAP-TLS IPsec peer and starts `ze`, having set the documented environment variable.
- Format at entry: the process environment, read by the Go runtime before any Ze code.

### Transformation Path
1. The Go runtime parses `GODEBUG` and fatals on a removed key set to its old value. Nothing below runs.
2. Without the variable, the daemon starts and reaches `exportEAPTLSMSK` (`internal/component/ike/eap`).
3. That selects by negotiated TLS version; on a TLS 1.2 session without RFC 7627 the export is refused by `crypto/tls`.
4. The EAP-TLS exchange cannot complete, so the peer never authenticates.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator ↔ runtime | the `GODEBUG` environment variable | Yes, reproduced 2026-08-23 |
| Ze ↔ crypto/tls | `ExportKeyingMaterial` on the negotiated session | Yes. `TestEAPTLSExportRefusalNamesTheCause` drives a real TLS 1.2 handshake whose export `crypto/tls` refuses, and `TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret` drives one it allows |
| Ze ↔ peer | the EAP-TLS exchange | NO LONGER, and this change is what invalidated it. "Scenario 04 fails with no IKE packet sent" was the pre-fix observation: the daemon died at container start. With the `ze-env` assignment gone ze starts, so the boundary is now crossed and the exchange fails at the MSK export instead. Re-verifying it needs a lab run, which is AC-5 in "Outstanding" |

### Integration Points
- `exportEAPTLSMSK` - the single place the export is attempted and the only place that can produce a diagnosable error
- `cmd/ze/main.go` - the guidance that must stop naming a removed key

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The message is built where the refusal arrives, in `exportEAPTLSMSK`, and travels the `MethodResult.Err` / `PeerResult.Err` path the engine already logs. No new channel was opened |
| No unintended coupling (components stay isolated) | Yes | `eapTLS12ExportRefused` reads `tls.ConnectionState` alone. The eap package gained no import and no dependency on the engine |
| No duplicated functionality (extends existing, does not recreate) | Yes | The TLS 1.2 branch's existing wrap was replaced rather than added beside (`ai/rules/no-layering.md`). The test reuses the package's own `newEAPTLSPKI` and the production `verifyServerChain` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | An authentication refusal is a control-plane path taken once for each failed session |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No command, view, family, or handler was added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Go offers no replacement knob for the unsafe EKM export in 1.27 | the removal note at `https://go.dev/doc/godebug#go-127` | a supported knob exists and the fix is to name it instead | read the Go 1.27 release notes and the `crypto/tls` source in GOROOT | confirmed |
| A-2 | The refusal is reachable and diagnosable inside Ze, so a clear error can replace a runtime fatal | `exportEAPTLSMSK` selects by negotiated version, so the TLS 1.2 branch is a place Ze controls | the error cannot be attributed and the fix is documentation only | read `exportEAPTLSMSK` and the `crypto/tls` error it receives | confirmed |
| A-3 | No deployment currently relies on the variable | it kills the daemon on Go 1.27, so any deployment that set it is already down | an operator on an older toolchain is relying on it and a removal surprises them | the release notes for the Go version Ze pins, and `go.mod` | confirmed |

**A-1 evidence, read 2026-08-23 in `$(go env GOROOT)` at Go 1.27.0.** The
`internal/godebugs` table carries the row <!-- doc-links: ignore (a GOROOT path, not a path in this repository) -->

```
{Name: "tlsunsafeekm", Removed: 27, Old: one},
```

and `doc/godebug.md` states the removal. In `crypto/tls`, `Conn.connectionStateLocked`
selects `noEKMBecauseNoEMS` on `c.vers != VersionTLS13 && !c.extMasterSecret`, with no
godebug guard left in the expression, and `noEKMBecauseNoEMS` returns the refusal
unconditionally. `ConnectionState.ekm` is unexported and `ExportKeyingMaterial` is its
only accessor, so no caller outside `crypto/tls` can reach the RFC 5705 PRF another
way, and `tls.Config` exposes no extended-master-secret or export knob. There is no
replacement.

**A-2 evidence.** `exportEAPTLSMSK` (`internal/component/ike/eap/eap_tls.go`) receives
the `crypto/tls` error and holds `cs.Version` and `cs.PeerCertificates`, which is every
fact AC-2 asks the message to name. On the initiator path, which is the one scenario 04
drives (`ze.conf`: `connection-type initiate`), the error reaches the operator through
`handleEAPResponse` (`internal/component/ike/engine/fsm.go`), which logs `result.Err`.

**A-3 evidence.** The fatal was reproduced on this host at Go 1.27.0 with a one-line
program. With `tlsunsafeekm` set to its old value in the environment it exits 2 with

```
fatal error: removed GODEBUG "tlsunsafeekm" set to old value "1" in environment (https://go.dev/doc/godebug#go-127)
```

Ze has never been released, so no deployment exists to be surprised.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fix becomes re-adding the package-level directive, reversing a decision the owner made on security grounds | a diff adds `//go:debug tlsunsafeekm=1` to `cmd/ze/main.go` | the Key Design Decisions table records that as REJECTED before implementation starts |
| R-2 | The capability is genuinely unrecoverable, and the spec closes as documentation only | A-1 confirms no replacement knob | that is an acceptable outcome, stated plainly: Ze cannot authenticate such a peer, `docs/` says so, and the error names it. Silence is what this spec forbids |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator either cannot start the daemon, or cannot reach a TLS 1.2 peer and cannot find out why |
| How is it reverted? | Single commit revert. No config migration, no persisted state |
| Who else touches this path? | The IPsec interop lab, which sets the variable for scenario 04 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An EAP-TLS peer negotiating TLS 1.2 without RFC 7627 | → | `exportEAPTLSMSK` | `TestEAPTLSExportRefusalNamesTheCause`. The `.ci` this row first named was dropped: a `.ci` cannot reach the state, because it would need a TLS 1.2 peer that offers no RFC 7627 and Go's own client always offers it, so any Ze-driven peer authenticates. Only the Docker lab has such a peer, which is the row below |
| The interop lab's EAP-TLS scenario against strongSwan 5.9.14 | → | the IKE EAP exchange | `test/interop-ipsec/scenarios/04-eap-tls`. NOT RUN in this phase, see "Outstanding" |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze` starts with no special environment | The daemon runs. No shipped guidance names a removed GODEBUG key |
| AC-2 | An EAP-TLS peer negotiates TLS 1.2 without RFC 7627 | Ze refuses the peer with an error naming the peer, the TLS version, RFC 7627 and what the operator can do |
| AC-3 | The same peer negotiates TLS 1.3 | Authentication succeeds, exactly as it does today |
| AC-4 | The repository is searched for `tlsunsafeekm` | No file instructs an operator to set it, and any surviving mention explains that it was removed by the toolchain |
| AC-5 | Scenario `04-eap-tls` runs | It either passes, or it is retired with its reason recorded and the capability gap documented. It does not sit red |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Follows Ze's guidance to reach a legacy EAP-TLS peer | environment → runtime → daemon | `TestEAPTLSExportRefusalNamesTheCause` |
| 2 | Connects a TLS 1.2 strongSwan peer and reads the log to find out why it failed | IKE → EAP → `exportEAPTLSMSK` → log | `test/interop-ipsec/scenarios/04-eap-tls` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPTLSExportRefusalNamesTheCause` | `internal/component/ike/eap/eap_tls_export_refusal_test.go` | validates AC-2: the refusal is attributed, not passed through raw | green. Discriminates: with `eapTLS12ExportRefused` replaced by the old bare wrap, seven assertions go red |
| `TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret` | same file | keeps AC-2 from reading as "TLS 1.2 never works": a TLS 1.2 session that carries RFC 7627 still exports a real MSK | green |
| `TestEAPTLSAuthenticatorKeepsItsRefusalReason` | same file | AC-2 holds in the AUTHENTICATOR role too, which interop scenarios 08 and 25 drive. The message is worth nothing if the role that builds it discards it | green. Discriminates: remove `s.err = result.Err` from `handleMethod` and it reds |
| `TestNoShippedGuidanceNamesARemovedGODEBUG` | `cmd/ze/godebug_guidance_test.go` | validates AC-4 structurally, so the next removed key cannot be left in a comment | green. Discriminates on both halves: restoring the `ze-env` assignment reds the first, and stripping the word "removed" from that file reds the second |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Negotiated TLS version | 1.2-1.3 | 1.3 | 1.1 is refused already | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-tls12-refusal-is-attributed` | not created | an operator reads the log after a TLS 1.2 EAP-TLS peer fails, and learns the peer, the TLS version and RFC 7627 rather than a raw crypto/tls string | UNREACHABLE, verified rather than assumed. A `.ci` drives ze against ze, so both TLS endpoints are Go. `makeClientHello` (`crypto/tls`, `handshake_client.go`) sets `extendedMasterSecret: true` unconditionally, and the only branch that clears it is the ECH inner hello, which is TLS 1.3 only. `tls.Config` exposes no knob. So a Go client always offers RFC 7627 on TLS 1.2 and the export always succeeds: the state this test would assert on cannot be produced without a non-Go peer, which is the Docker lab |
| `04-eap-tls` | `test/interop-ipsec/scenarios/04-eap-tls` | the TLS 1.2 peer path against a real strongSwan, whatever its resolution | NOT RUN, see "Outstanding" |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `04-eap-tls` | `test/interop-ipsec/scenarios/` | strongSwan 5.9.14 | the TLS 1.2 without RFC 7627 case, which is the only case in question | |
| `06-eap-tls13` | `test/interop-ipsec/scenarios/` | strongSwan | the TLS 1.3 path is unaffected, which AC-3 requires | |

## Files to Modify
- `cmd/ze/main.go` - the guidance stops naming a removed key and states what an operator can actually do
- `internal/component/ike/eap` - `exportEAPTLSMSK` attributes the refusal rather than returning a raw `crypto/tls` error
- `test/interop-ipsec/scenarios/04-eap-tls/ze-env` - stops setting a key that kills the daemon
- `docs/architecture/system-architecture.md` - the design doc `cmd/ze/main.go` declares, if the guidance it summarises changes
- `docs/guide/ipsec.md` - what an operator meeting a TLS 1.2 peer should expect

## Files to Create
- `internal/component/ike/eap/eap_tls_export_refusal_test.go` - the AC-2 proof at the producer, driving a real TLS 1.2 handshake whose export `crypto/tls` refuses
- `cmd/ze/godebug_guidance_test.go` - the AC-4 proof over every tracked file
- `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` - NOT created. The TDD table records why: a `.ci` has two Go endpoints and a Go client always offers RFC 7627, so the state cannot be produced <!-- doc-links: ignore (the row exists to say this file was NOT created and why) -->

## Outstanding (written 2026-08-23, at the end of the code phase)

Two items are open and neither is a trim. Each needs its own agent.

| # | What | Why it is not done here |
|---|------|-------------------------|
| 1 | AC-5: scenario `04-eap-tls` is not run and not resolved | `ze-env` no longer stops the daemon, so the scenario now fails at the refusal instead of at container start. Resolving it means repurposing `check.py` to assert the attributed refusal, or retiring the scenario. Both need a full Docker lab run to verify, and retiring it deletes tracked work, which needs the owner's word (`ai/rules/never-destroy-work.md`) |
| 2 | The Required Reading row for RFC 7627 is still MUST CREATE | `ai/rules/protocol.md` makes the summary a precondition. Creating it pulls in a disposition in `rfc/enrolled.txt` or `rfc/not-enrolled.txt`, and enrolment would need every MUST classified, which `ai/rules/rfc-compliance.md` reserves to the owner. Ze implements none of RFC 7627: `crypto/tls` does. Nothing written in this phase quotes RFC 7627's normative text, so no claim rests on the unread document |

One defect was found on the way, and it is FIXED rather than recorded, because
AC-2 does not hold without it. `handleMethod`
(`internal/component/ike/eap/eap.go`) read `MethodResult.Err` as a boolean and
dropped it, so the RESPONDER half of every EAP method discarded its own
diagnosis while the initiator half logged it. AC-2 is stated unqualified and
interop scenarios 08 and 25 drive the responder role, so the message this spec
exists to write was being built and thrown away for half of ze's roles.
`Session.err`, `Session.Err()` and the `handleResponderEAP` log line close it,
and `TestEAPTLSAuthenticatorKeepsItsRefusalReason` holds it. The row in
`plan/journal/validated-value-discarded-by-its-caller.md` is updated to
`fixed`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no operator-visible setting changes unless A-1 produces one |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | No | `GODEBUG` is the Go runtime's, not a `ze.` key |
| Doctor check for runtime dependencies | No | The design-time answer was Yes and it is wrong. `ze doctor` is the same binary, so a removed setting in the environment fatals it before `main()` exactly as it fatals the daemon: the check could never run in the case it exists for. The Go runtime already reports that condition, by name and with the release note URL, which is why nothing here can improve on it. The reachable half is the guidance, and `TestNoShippedGuidanceNamesARemovedGODEBUG` owns it |
| Prometheus counters/metrics | No | an authentication refusal is already logged |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a capability is documented as lost, or restored |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, what a TLS 1.2 EAP-TLS peer now does |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | Done in `docs/features/rfc-status.md`, RFC 5216 row: the coverage cell now states that the Section 2.3 export is unreachable on TLS 1.2 without RFC 7627, and that the peer reaches TLS 1.3 with one config change. `rfc/short/rfc5216.md` was deliberately NOT touched. Its `RFC5216-2.3-1` annotation says ze exports the 64-octet MSK with the RFC label, which is still exactly true: ze implements the derivation, and the peer's TLS stack fails to supply its input. Writing a `{gap}` there would claim a conformance failure ze does not have, and would move the count `check_gap_count_agreement` reads. `make ze-rfc-check` is green |
| 10 | Test infrastructure changed? | Yes | the interop lab's `ze-env` for scenario 04 |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it claims EAP-TLS parity with a daemon that still reaches such peers |
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md`, the design doc `cmd/ze/main.go` declares |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `cmd/ze/main.go` and the eap package |
| 16b | `docs/architecture/ike/ipsec-11-interop-eap.md`, the design doc `eap_tls_export_refusal_test.go` declares | No | Unaffected. It names no GODEBUG, no RFC 7627, and no TLS version condition (grepped 2026-08-23 for `godebug`, `tlsunsafe`, `7627`, `extended master`, `TLS 1.2`: zero hits), so nothing in it went stale. The TLS 1.2 limit is documented in `docs/guide/ipsec.md` and `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`, which is where the fail-closed decision already lived |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | any example setting `GODEBUG` |

## Implementation Steps

1. **Phase: Settle A-1 (MANDATORY FIRST)** -- establish whether Go 1.27 offers any supported way to obtain the export
   - Method: the Go 1.27 release notes, the `godebug` removal page, and `crypto/tls` in GOROOT read directly
   - Verify: A-1 flips to `confirmed` or `broken`. **This decides whether the spec restores a capability or documents its loss, so nothing else starts first**
2. **Phase: Stop the guidance killing the daemon** -- whatever A-1 says
   - Tests: `TestNoShippedGuidanceNamesARemovedGODEBUG`
   - Files: `cmd/ze/main.go`, `test/interop-ipsec/scenarios/04-eap-tls/ze-env`
   - Verify: no shipped text instructs an operator to set a removed key
3. **Phase: Attribute the refusal** -- an operator learns why their peer failed
   - Tests: `TestEAPTLSExportRefusalNamesTheCause`
   - Files: `internal/component/ike/eap`
   - Verify: the error names the peer, the TLS version and RFC 7627, and reverting it returns the raw `crypto/tls` text
4. **Phase: Resolve scenario 04** -- pass it, or retire it with the gap recorded
   - Tests: `04-eap-tls`
   - Verify: the scenario is not red, and whichever outcome it takes is stated in the spec rather than implied by its absence

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The fail-closed default is unchanged, and no package-level `//go:debug` directive was added |
| Rule: `ai/rules/rfc-compliance.md` | RFC 5216 Section 2.3 is a MUST-level derivation. If Ze cannot meet it for TLS 1.2, that is stated in `rfc/short/` and on the public status page rather than left silent |
| Rule: `ai/rules/completion.md` | Scenario 04 is not left red, and it is not made green by deleting the assertion |
| Rule: `ai/rules/evidence.md` | The A-1 verdict is read from the Go source and release notes, not inferred from the error text |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No shipped text names a removed GODEBUG key | `TestNoShippedGuidanceNamesARemovedGODEBUG` |
| The refusal is attributable | `TestEAPTLSExportRefusalNamesTheCause` |
| The TLS 1.3 path is untouched | `06-eap-tls13` and `25-responder-eap-tls13` |
| Scenario 04 is not red | `make ze-interop-ipsec-test` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None new: the change is guidance and error attribution |
| Weakened defaults | The whole risk. Any diff that makes the unsafe export the default for every binary reverses a decision taken on 2026-08-01 and must be refused |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A safe default and its escape hatch are one decision. When a toolchain removes the hatch, the default becomes a refusal nobody chose, and the guidance describing the hatch outlives the hatch.
- A capability lost to a dependency bump leaves no diff to review, which is why nothing caught it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Settle whether a replacement exists before choosing a fix | **B. Re-add `//go:debug tlsunsafeekm=1` to `cmd/ze/main.go`.** REJECTED, and recorded here so no implementer reaches for it: it sets the default for every Ze binary and weakens the export rule for every deployment to suit one peer version. The owner removed exactly that line on 2026-08-01. **C. Delete the guidance and say nothing.** REJECTED: a capability that disappears silently is the defect, not the cure | The fix depends on a fact nobody has established. A-1 is one reading of the Go source and release notes, and it decides between restoring a capability and documenting its loss |

## Known Limitations

- If A-1 confirms no replacement exists, Ze cannot authenticate a TLS 1.2 EAP-TLS peer that does not negotiate RFC 7627, and this spec's outcome is to say so everywhere a reader might assume otherwise.

## RFC Documentation (Scope: protocol)

Add `// RFC 5216 Section 2.3: "<quoted requirement>"` above the export site, and
`// RFC 7627: "<quoted requirement>"` above the refusal that cites it. If the
capability is lost, the `rfc/short/rfc5216.md` row records what Ze cannot do and
why, with the producing function named, per `ai/rules/rfc-compliance.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
