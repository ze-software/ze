# Spec: fixit-eap-tls-escape-hatch-kills-the-daemon

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-24 |

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
   `test/interop-ipsec/scenarios/eap-tls13` and `responder-eap-tls13` both
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

**Where it is measurable today.** `test/interop-ipsec/scenarios/eap-tls/ze-env`
sets the variable, so that scenario fails at `strongSwan SA 'ze' did not reach
ESTABLISHED within 90s` with no IKE packet sent. `eap-tls13` and
`responder-eap-tls13` pass, because TLS 1.3 needs none of this: the export is
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
- [ ] `test/interop-ipsec/scenarios/eap-tls/ze-env` - sets the removed key, so the daemon dies at container start

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
| Ze ↔ peer | the EAP-TLS exchange | NO LONGER, and this change is what invalidated it. "Scenario eap-tls fails with no IKE packet sent" was the pre-fix observation: the daemon died at container start. With the `ze-env` assignment gone ze starts, so the boundary is now crossed and the exchange fails at the MSK export instead. Re-verifying it needs a lab run, which is AC-5 in "Outstanding" |

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
fact AC-2 asks the message to name. On the initiator path, which is the one scenario eap-tls
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
| Who else touches this path? | The IPsec interop lab, which sets the variable for scenario eap-tls |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An EAP-TLS peer negotiating TLS 1.2 without RFC 7627 | → | `exportEAPTLSMSK` | `TestEAPTLSExportRefusalNamesTheCause`. The `.ci` this row first named was dropped: a `.ci` cannot reach the state, because it would need a TLS 1.2 peer that offers no RFC 7627 and Go's own client always offers it, so any Ze-driven peer authenticates. Only the Docker lab has such a peer, which is the row below |
| The interop lab's EAP-TLS scenario against strongSwan 5.9.14 | → | the IKE EAP exchange | `test/interop-ipsec/scenarios/eap-tls`. NOT RUN in this phase, see "Outstanding" |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze` starts with no special environment | The daemon runs. No shipped guidance names a removed GODEBUG key |
| AC-2 | An EAP-TLS peer negotiates TLS 1.2 without RFC 7627 | Ze refuses the peer with an error naming the peer, the TLS version, RFC 7627 and what the operator can do |
| AC-3 | The same peer negotiates TLS 1.3 | Authentication succeeds, exactly as it does today |
| AC-4 | The repository is searched for `tlsunsafeekm` | No file instructs an operator to set it, and any surviving mention explains that it was removed by the toolchain |
| AC-5 | Scenario `eap-tls` runs | It either passes, or it is retired with its reason recorded and the capability gap documented. It does not sit red |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Follows Ze's guidance to reach a legacy EAP-TLS peer | environment → runtime → daemon | `TestEAPTLSExportRefusalNamesTheCause` |
| 2 | Connects a TLS 1.2 strongSwan peer and reads the log to find out why it failed | IKE → EAP → `exportEAPTLSMSK` → log | `test/interop-ipsec/scenarios/eap-tls` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPTLSExportRefusalNamesTheCause` | `internal/component/ike/eap/eap_tls_export_refusal_test.go` | validates AC-2: the refusal is attributed, not passed through raw | green. Discriminates: with `eapTLS12ExportRefused` replaced by the old bare wrap, seven assertions go red |
| `TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret` | same file | keeps AC-2 from reading as "TLS 1.2 never works": a TLS 1.2 session that carries RFC 7627 still exports a real MSK | green |
| `TestEAPTLSAuthenticatorKeepsItsRefusalReason` | same file | AC-2 holds in the AUTHENTICATOR role too, which interop scenarios responder-eap-mschapv2 and responder-eap-tls13 drive. The message is worth nothing if the role that builds it discards it | green. Discriminates: remove `s.err = result.Err` from `handleMethod` and it reds |
| `TestNoShippedGuidanceNamesARemovedGODEBUG` | `cmd/ze/godebug_guidance_test.go` | validates AC-4 structurally, so the next removed key cannot be left in a comment | green. Discriminates on both halves: restoring the `ze-env` assignment reds the first, and stripping the word "removed" from that file reds the second |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Negotiated TLS version | 1.2-1.3 | 1.3 | 1.1 is refused already | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-tls12-refusal-is-attributed` | not created | an operator reads the log after a TLS 1.2 EAP-TLS peer fails, and learns the peer, the TLS version and RFC 7627 rather than a raw crypto/tls string | UNREACHABLE, verified rather than assumed. A `.ci` drives ze against ze, so both TLS endpoints are Go. `makeClientHello` (`crypto/tls`, `handshake_client.go`) sets `extendedMasterSecret: true` unconditionally, and the only branch that clears it is the ECH inner hello, which is TLS 1.3 only. `tls.Config` exposes no knob. So a Go client always offers RFC 7627 on TLS 1.2 and the export always succeeds: the state this test would assert on cannot be produced without a non-Go peer, which is the Docker lab |
| `eap-tls` | `test/interop-ipsec/scenarios/eap-tls` | the TLS 1.2 peer path against a real strongSwan, whatever its resolution | NOT RUN, see "Outstanding" |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `eap-tls` | `test/interop-ipsec/scenarios/` | strongSwan 5.9.14 | the TLS 1.2 without RFC 7627 case, which is the only case in question | |
| `eap-tls13` | `test/interop-ipsec/scenarios/` | strongSwan | the TLS 1.3 path is unaffected, which AC-3 requires | |

## Files to Modify
- `cmd/ze/main.go` - the guidance stops naming a removed key and states what an operator can actually do
- `internal/component/ike/eap` - `exportEAPTLSMSK` attributes the refusal rather than returning a raw `crypto/tls` error
- `test/interop-ipsec/scenarios/eap-tls/ze-env` - stops setting a key that kills the daemon
- `docs/architecture/system-architecture.md` - the design doc `cmd/ze/main.go` declares, if the guidance it summarises changes
- `docs/guide/ipsec.md` - what an operator meeting a TLS 1.2 peer should expect

## Files to Create
- `internal/component/ike/eap/eap_tls_export_refusal_test.go` - the AC-2 proof at the producer, driving a real TLS 1.2 handshake whose export `crypto/tls` refuses
- `cmd/ze/godebug_guidance_test.go` - the AC-4 proof over every tracked file
- `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` - NOT created. The TDD table records why: a `.ci` has two Go endpoints and a Go client always offers RFC 7627, so the state cannot be produced <!-- doc-links: ignore (the row exists to say this file was NOT created and why) -->

## Outstanding (written 2026-08-23; item 1 closed 2026-08-24 in the closure phase)

| # | What | Status |
|---|------|--------|
| 1 | AC-5: scenario `eap-tls` is not run and not resolved | **CLOSED 2026-08-24.** The scenario is repurposed rather than retired, so nothing tracked was deleted and no owner decision was needed. Retiring was the only option that needed his word, and it was also the one that reduces coverage: `eap-tls` is the test named for User Story 2 and for the interop row of the Wiring Test, and `plan/journal/shared-leniency-hides-the-defect.md` records it as the only test whose peer is not a second copy of ze. `check.py` now asserts the completed TLS handshake, the attributed refusal, and that neither end installs an XFRM SA. PASS, and RED under the bare-wrap mutation |
| 2 | The Required Reading row for RFC 7627 is still MUST CREATE | **OPEN, and it is Thomas's decision.** `ai/rules/protocol.md` makes the summary a precondition of design work. Every route to writing it needs him: `check_new_summaries` refuses a NEW summary that declares gated MUSTs and is not in `rfc/enrolled.txt`, enrolment needs every MUST classified, and `ai/rules/rfc-compliance.md` reserves that classification to the owner. `backlog` is closed for the same reason (it is legal for `rfc1035` and `rfc9190` only because both predate that gate), and `non-normative` would be false of a Standards Track document. `make ze-rfc-check` is GREEN: nothing is red, because no summary exists to be judged. One shipped claim rests on the unread text, and it is why the row cannot simply be dropped: `cmd/ze/main.go` states that RFC 7627 "exists to stop the triple handshake attack, and without it exported keying material can collide across sessions". That is protocol semantics cited from memory, which `ai/rules/protocol.md` forbids |

One defect was found on the way, and it is FIXED rather than recorded, because
AC-2 does not hold without it. `handleMethod`
(`internal/component/ike/eap/eap.go`) read `MethodResult.Err` as a boolean and
dropped it, so the RESPONDER half of every EAP method discarded its own
diagnosis while the initiator half logged it. AC-2 is stated unqualified and
interop scenarios responder-eap-mschapv2 and responder-eap-tls13 drive the responder role, so the message this spec
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
| 10 | Test infrastructure changed? | Yes | the interop lab's `ze-env` for scenario eap-tls |
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
   - Files: `cmd/ze/main.go`, `test/interop-ipsec/scenarios/eap-tls/ze-env`
   - Verify: no shipped text instructs an operator to set a removed key
3. **Phase: Attribute the refusal** -- an operator learns why their peer failed
   - Tests: `TestEAPTLSExportRefusalNamesTheCause`
   - Files: `internal/component/ike/eap`
   - Verify: the error names the peer, the TLS version and RFC 7627, and reverting it returns the raw `crypto/tls` text
4. **Phase: Resolve scenario eap-tls** -- pass it, or retire it with the gap recorded
   - Tests: `eap-tls`
   - Verify: the scenario is not red, and whichever outcome it takes is stated in the spec rather than implied by its absence

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The fail-closed default is unchanged, and no package-level `//go:debug` directive was added |
| Rule: `ai/rules/rfc-compliance.md` | RFC 5216 Section 2.3 is a MUST-level derivation. If Ze cannot meet it for TLS 1.2, that is stated in `rfc/short/` and on the public status page rather than left silent |
| Rule: `ai/rules/completion.md` | Scenario eap-tls is not left red, and it is not made green by deleting the assertion |
| Rule: `ai/rules/evidence.md` | The A-1 verdict is read from the Go source and release notes, not inferred from the error text |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No shipped text names a removed GODEBUG key | `TestNoShippedGuidanceNamesARemovedGODEBUG` |
| The refusal is attributable | `TestEAPTLSExportRefusalNamesTheCause` |
| The TLS 1.3 path is untouched | `eap-tls13` and `responder-eap-tls13` |
| Scenario eap-tls is not red | `make ze-interop-ipsec-test` |

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

---

## Implementation Summary

### What Was Implemented

The code landed on 2026-08-23 as commit `d53e73ea9`, "fix(eap): stop telling
operators to set a GODEBUG that kills ze". The closure phase ran on 2026-08-24 and
added the interop resolution AC-5 asks for.

- `cmd/ze/main.go` names no GODEBUG assignment. The header states the fail-closed
  choice, records that Go 1.27 removed the setting, and names the three answers an
  operator has.
- `eapTLS12ExportRefused` and `eapTLSPeerName`
  (`internal/component/ike/eap/eap_tls.go`) attribute the refusal: the peer subject,
  the negotiated version, RFC 7627 and the three remedies, wrapping the `crypto/tls`
  sentence with `%w` rather than replacing it.
- `Session.err`, `Session.Err()` (`internal/component/ike/eap/eap.go`) and the log
  line in `handleResponderEAP` (`internal/component/ike/engine/responder_eap.go`)
  carry that diagnosis on the AUTHENTICATOR half, which had been discarding it.
- `test/interop-ipsec/scenarios/eap-tls/ze-env` sets nothing and says why.
- **Closure phase:** `test/interop-ipsec/scenarios/eap-tls/check.py` is
  repurposed. It asserts the TLS handshake completes, that ze's refusal states all
  eight operator facts, and that neither end installs an XFRM SA.

### Bugs Found/Fixed

- The authenticator half of every EAP method discarded `MethodResult.Err`.
  `handleMethod` read it as a boolean. Covered by
  `TestEAPTLSAuthenticatorKeepsItsRefusalReason`. The row in
  `plan/journal/validated-value-discarded-by-its-caller.md` is updated to `fixed`.
- **Closure phase.** The comment on `indicateSuccess`
  (`internal/component/ike/eap/eap_tls.go`) cited scenario `eap-tls` as the proof
  that a TLS 1.2 exchange concludes with a bare EAP-Success. Ze is the EAP PEER in
  that scenario and `indicateSuccess` runs on the authenticator side, so the citation
  was wrong before this spec touched it and wrong for a second reason after. It now
  names `TestEAPTLS12SendsNoProtectedSuccessIndication`
  (`internal/component/ike/eap/rfc9190_test.go`).

### Documentation Updates

| File | What changed | Anchor |
|------|--------------|--------|
| `docs/features/rfc-status.md` | the RFC 5216 row states that the Section 2.3 export is unreachable on TLS 1.2 without RFC 7627, and that one line of peer config moves the peer to the RFC 9190 path | `eapTLS12ExportRefused` named inline |
| `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` | the 2026-08-01 decision record says the setting was removed by the toolchain | in file |
| `docs/guide/ipsec.md` | the section "EAP-TLS with TLS 1.2 needs RFC 7627", with the three-answer table | `<!-- source: internal/component/ike/eap/eap_tls.go -- exportEAPTLSMSK, eapTLS12ExportRefused -->` |
| `docs/architecture/ike/ipsec-11-interop-eap.md` | **closure phase.** The Proof section states what scenario eap-tls asserts now | `<!-- source: internal/component/ike/eap/eap_tls.go -- exportEAPTLSMSK, eapTLS12ExportRefused -->` |

`docs/guide/ipsec.md` was written by a different session and landed in `5e14f7f51`,
not in `d53e73ea9`. It is recorded here because it discharges checklist row 6, and it
is NOT in this spec's commit list for the same reason.

`make ze-doc-verify` reports two failures and neither is this spec's: a source anchor
in `docs/guide/web-interface.md` naming `liveAAABundleAuthenticator.Authenticate`, and
`ai/rules/config.md` stale against `ai/rules/points/config/`. Both sit in another
session's uncommitted work in this shared checkout.

### Deviations from Plan

| Planned | What happened |
|---------|---------------|
| `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` | NOT created, and the reason is verified rather than assumed. A `.ci` drives ze against ze, and `makeClientHello` (`crypto/tls`) sets `extendedMasterSecret: true` unconditionally on every TLS 1.2 ClientHello, so a Go client always offers RFC 7627 and the export always succeeds. The state cannot be produced without a non-Go peer |
| Scenario eap-tls "either passes or is retired" | It PASSES, by asserting the refusal instead of a tunnel. Retiring was rejected: it deletes tracked work AND it is the option that reduces coverage |
| An RFC 7627 summary created before design work | NOT created. See Outstanding item 2: every route needs the owner. The path is deliberately not spelled, here or in Required Reading: it does not resolve, so `make ze-doc-links-check` and `.claude/hooks/validate-spec.sh` would both read it as rot rather than as the gap this records |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation phase left AC-5 open on the reading that both routes needed the owner, because one of them (retiring) deletes tracked work | Only the RETIRE route needed him. Repurposing deletes nothing and is what the spec's own User Story 2 and Wiring Test row already ask scenario eap-tls to prove | the closure phase re-derived it from the scenario, from `plan/journal/shared-leniency-hides-the-defect.md`, and from a Docker lab run | scenario eap-tls repurposed and green. When two options are on the table and one needs the owner, the question to ask first is whether the OTHER one is simply right |
| assumption | A code comment cited scenario `eap-tls` as proof for authenticator-side behaviour | Ze is the EAP PEER in scenario eap-tls: its `ze.conf` carries `connection-type initiate` and strongSwan's `swanctl.conf` carries `remote { auth = eap-tls }` | reading the scenario's two config files while repurposing the check | citation corrected to the RFC-tagged unit test. A scenario's NAME does not say which ROLE ze plays in it |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Ze must not instruct an operator to do something that stops the daemon | Done | `cmd/ze/main.go` header; `test/interop-ipsec/scenarios/eap-tls/ze-env` | held by `TestNoShippedGuidanceNamesARemovedGODEBUG` (`cmd/ze/godebug_guidance_test.go`) |
| An operator meeting a TLS 1.2 peer without RFC 7627 gets an error naming the peer, the RFC and the options | Done | `eapTLS12ExportRefused` (`internal/component/ike/eap/eap_tls.go`) | proven against a real strongSwan by scenario `eap-tls` |
| The fail-closed default is unchanged | Done | no `//go:debug` directive in `cmd/ze/main.go` | the one line naming that shape is prose about the REJECTED alternative |
| The TLS 1.3 path is untouched | Done | `exportEAPTLSMSK` selects by negotiated version | `eap-tls13` and `responder-eap-tls13` both PASS |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestNoShippedGuidanceNamesARemovedGODEBUG`; the lab starts ze with no special environment in every scenario | ze runs; nothing shipped names a removed key in an assignable form |
| AC-2 | Done | `TestEAPTLSExportRefusalNamesTheCause`, and `eap-tls` against strongSwan 5.9.14 | the interop run is the one that drives the REAL `crypto/tls` branch |
| AC-3 | Done | `eap-tls13` PASS, `responder-eap-tls13` PASS | ESP accepted on both ends in eap-tls13 |
| AC-4 | Done | `TestNoShippedGuidanceNamesARemovedGODEBUG` plus the two fixture tests | see the Review Gate note below: the repository-wide scan is toolchain-derived, so it becomes live for this setting when `go.mod` moves to Go 1.27 |
| AC-5 | Done | `eap-tls` PASS, mutation-verified RED | closed in the closure phase, 2026-08-24 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestEAPTLSExportRefusalNamesTheCause` | Done | `internal/component/ike/eap/eap_tls_export_refusal_test.go` | mutation re-verified 2026-08-24: seven assertions red under the bare wrap |
| `TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret` | Done | same file | green |
| `TestEAPTLSAuthenticatorKeepsItsRefusalReason` | Done | same file | green |
| `TestNoShippedGuidanceNamesARemovedGODEBUG` | Done | `cmd/ze/godebug_guidance_test.go` | green |
| `ipsec-eap-tls12-refusal-is-attributed.ci` | Changed | not created | UNREACHABLE with two Go endpoints; see Deviations |
| `eap-tls` | Done | `test/interop-ipsec/scenarios/eap-tls` | PASS after repurposing |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/main.go` | Done | in `d53e73ea9` |
| `internal/component/ike/eap` | Done | `eap_tls.go`, `eap.go`; `eap_tls.go` edited again in the closure phase for the stale citation |
| `test/interop-ipsec/scenarios/eap-tls/ze-env` | Done | in `d53e73ea9` |
| `docs/architecture/system-architecture.md` | Changed | not edited. The guidance it summarises did not change: it names no GODEBUG and no TLS version condition. `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` is where the fail-closed decision lives and it WAS edited |
| `docs/guide/ipsec.md` | Done | landed in `5e14f7f51` |
| `internal/component/ike/eap/eap_tls_export_refusal_test.go` | Done | created in `d53e73ea9` |
| `cmd/ze/godebug_guidance_test.go` | Done | created in `d53e73ea9` |
| `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` | Changed | NOT created; see Deviations |

### Audit Summary
- **Total items:** 23
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (all recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze's shipped instruction no longer stops the daemon | functional (structural guard over the whole tree) | `TestNoShippedGuidanceNamesARemovedGODEBUG` passes as part of `make ze-unit-pkg-test PKG=./cmd/ze` -> `ok github.com/ze-software/ze/cmd/ze 76.968s`. Its predicate is proven against the defect's own bytes by `TestGODEBUGGuardRedsOnTheDefectItWasWrittenFor` |
| The capability's loss is stated everywhere a reader might assume otherwise, rather than being silent | documentation, source-anchored | `docs/guide/ipsec.md` "EAP-TLS with TLS 1.2 needs RFC 7627"; the RFC 5216 row of `docs/features/rfc-status.md`; `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`; the Proof section of `docs/architecture/ike/ipsec-11-interop-eap.md`. Each carries or is covered by a `<!-- source: ... -->` anchor on `exportEAPTLSMSK, eapTLS12ExportRefused` |
| An operator who meets such a peer learns the peer, the version, RFC 7627 and what to change | interop, against a real non-Go peer | `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=eap-tls` PASS. The daemon logged the line quoted in the fenced block below |
| The refusal is fail-closed: no SA is built from key material ze never derived | interop | scenario eap-tls asserts `check_xfrm_sa_count(ZE_CONTAINER, 0)` and `check_xfrm_sa_count(SWAN_CONTAINER, 0)`, and both pass. strongSwan's OWN EAP method succeeded in the same run (`EAP method EAP_TLS succeeded, MSK established`), so the peer was willing and only ze's refusal kept the SA off the wire |
| The TLS 1.3 path is unaffected | interop | `eap-tls13` PASS with `ESP counters advanced on 0xc1686e0d` on BOTH containers; `responder-eap-tls13` PASS |
| The tests discriminate | mutation | With `eapTLS12ExportRefused` reverted to the bare `fmt.Errorf` wrap: scenario eap-tls goes RED at `Ze log missing: 'cannot export the RFC 5216 Section 2.3 MSK'`, and `TestEAPTLSExportRefusalNamesTheCause` reds on seven assertions. Both re-measured 2026-08-24, and the file was restored to `d53e73ea9` afterwards |

The operator-facing line, read from the ze container during the passing run:

```
level=WARN msg="ike: EAP failed" subsystem=ike peer=swan error="eap-tls: cannot export the RFC 5216 Section 2.3 MSK for peer CN=172.28.0.3 on TLS 1.2. The export needs TLS 1.3, or a TLS 1.2 session that negotiated the RFC 7627 extended master secret. Move the peer to TLS 1.3 (RFC 9190), add RFC 7627 to its TLS 1.2 stack, or configure another EAP method: crypto/tls: ExportKeyingMaterial is unavailable when neither TLS 1.3 nor Extended Master Secret are negotiated; override with GODEBUG=tlsunsafeekm=1"
```

The trailing clause is `crypto/tls`'s own sentence, wrapped rather than replaced.
Go 1.27 drops it: `noEKMBecauseNoEMS` at tag `go1.27.0` returns the same sentence
without an override. So the pasteable form reaches an operator only on a toolchain
where the setting still exists and still works.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec metadata records the Deferral shard as `-`, and no shard for this stem exists under `plan/deferrals/`. There is nothing to remove in commit A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-eap-tls-escape-hatch-kills-the-daemon-9ad8358c-695f-41be-8019-5d92ba08f8e6.md` |
| `review_gate.py check` | clean -- `review_gate: OK (5 code files, clean, hashes match ...)` |
| Rounds | 2 |
| Reviewer lenses used | one closure agent running every lens: wiring, functional-test coverage, documentation drift, removed-behaviour, code comments, logic and guard audit, simplicity, security, interop and goal validation, RFC compliance, ze-style |

The review was independent by PHASE: the diff was written by a previous session
and committed as `d53e73ea9`, and this context judged it from source without
having authored it. It spawned no reader of its own.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | AC-5 unmet: scenario eap-tls was RED. `check.py` asserted an established IKE SA and ESP flow, which ze cannot reach against a stock strongSwan. Measured: `strongSwan SA 'ze' did not reach ESTABLISHED within 60s` | `test/interop-ipsec/scenarios/eap-tls/check.py` | repurposed to assert the handshake, the attributed refusal and the absence of any XFRM SA. PASS, mutation-verified RED |
| 2 | ISSUE | Stale citation: the comment on `indicateSuccess` named scenario eap-tls as proof of authenticator-side TLS 1.2 behaviour, and ze is the PEER there | `internal/component/ike/eap/eap_tls.go`, `indicateSuccess` | the citation names `TestEAPTLS12SendsNoProtectedSuccessIndication` (`internal/component/ike/eap/rfc9190_test.go`) and states why the lab cannot supply that proof |
| 3 | ISSUE | The design doc's Proof section implied scenario eap-tls proves a working EAP-TLS tunnel | `docs/architecture/ike/ipsec-11-interop-eap.md`, Proof | a paragraph stating what eap-tls asserts now, with a source anchor |

Five NOTEs did not block and are recorded in full in the artifact. The two a later
reader most needs:

- `removedGODEBUGSettings` (`cmd/ze/godebug_guidance_test.go`) derives the scan's
  population from the toolchain in use. `go.mod` pins `toolchain go1.26.6`, whose
  `internal/godebugs` table carries one removed setting and it is `x509sha1`. So the
  repository-wide half of `TestNoShippedGuidanceNamesARemovedGODEBUG` does not look
  for `tlsunsafeekm` today, and the AC-4 regression protection becomes live at the Go
  1.27 bump. That is the guard's stated design and it is right: on 1.26.6 the setting
  is not removed, so there is no harm to refuse. The PREDICATE is proven
  toolchain-independently by the two fixture tests.
- `is_test_path` (`scripts/dev/audit-test-relaxation.py`) admits `_test.go`, `.ci` and
  `.et` under `test/`, and the two Python test-file shapes. No interop scenario
  `check.py` is in that population, so a scenario weakened to reach green is invisible
  to the relaxation audit. Recorded as a journal row.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/eap/eap_tls_export_refusal_test.go` | Yes | tracked in `d53e73ea9`, 234 lines added |
| `cmd/ze/godebug_guidance_test.go` | Yes | tracked in `d53e73ea9`, 400 lines added |
| `cmd/ze/testdata/godebug-assignable-forms.tsv` | Yes | 13 rows read by `TestGODEBUGPredicateCoversEveryAssignableForm` |
| `cmd/ze/testdata/godebug-inert-forms.tsv` | Yes | 6 rows, same test |
| `cmd/ze/testdata/godebug-guidance-defect.txt` | Yes | 54 lines, the fixture for `TestGODEBUGGuardRedsOnTheDefectItWasWrittenFor` |
| `test/interop-ipsec/scenarios/eap-tls/check.py` | Yes | rewritten 2026-08-24; `ast.parse` over it returns SYNTAX OK |
| `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` | No, deliberately | see Deviations: two Go endpoints cannot produce the state |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the daemon runs and no shipped guidance names a removed key | `make ze-unit-pkg-test PKG=./cmd/ze` -> `ok github.com/ze-software/ze/cmd/ze 76.968s` |
| AC-2 | the refusal names the peer, the version, RFC 7627 and the remedies | `make ze-unit-pkg-test PKG=./internal/component/ike/eap` -> `ok ... 3.556s`; and the interop log line quoted in Goal Validation |
| AC-3 | TLS 1.3 still authenticates | `python3 test/interop-ipsec/run.py eap-tls13` -> `PASS 1 scenario(s)`; `responder-eap-tls13` -> `PASS 1 scenario(s)` |
| AC-4 | no file instructs an operator to set the setting | a `git ls-files --cached --others --exclude-standard` walk grepped for the assignable form returns no hit outside `vendor/`, `testdata/` and the guard's own test |
| AC-5 | scenario eap-tls is not red | `python3 test/interop-ipsec/run.py eap-tls` -> `PASS 1 scenario(s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An EAP-TLS peer negotiating TLS 1.2 without RFC 7627 | `internal/component/ike/eap/eap_tls_export_refusal_test.go`, `TestEAPTLSExportRefusalNamesTheCause` | Yes. `eapTLS12ExportRefused` has exactly one production caller, `exportEAPTLSMSK` |
| The interop lab's EAP-TLS scenario against strongSwan 5.9.14 | `test/interop-ipsec/scenarios/eap-tls/check.py` | Yes, read and run. It exercises the IKE EAP exchange end to end and asserts on ze's own log |
| The AUTHENTICATOR half keeps its diagnosis | `TestEAPTLSAuthenticatorKeepsItsRefusalReason` | Yes. `Session.Err()` has exactly one production caller, `handleResponderEAP` (`internal/component/ike/engine/responder_eap.go`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Re-verified 2026-08-24 against the released Go 1.27.0 source: the `internal/godebugs` table carries `{Name: "tlsunsafeekm", Removed: 27, Old: one}`, and `noEKMBecauseNoEMS` in `crypto/tls` returns the refusal with no override clause. The module proxy lists `v0.0.1-go1.27.0.linux-amd64`, so 1.27.0 is released |
| A-2 | confirmed | `eapTLS12ExportRefused` builds the message from `cs.Version` and `cs.PeerCertificates`, and the lab run proves it reaches the operator's log through `handleEAPResponse` |
| A-3 | confirmed | Ze has never been released. Recorded for the next reader: `go.mod` pins `toolchain go1.26.6`, and on that toolchain the setting is NOT removed (its table row reads `Changed: 22`) and a binary run with it set exits 0. So the fatal reaches a build made with a locally installed Go 1.27, which `GOTOOLCHAIN=auto` selects over the pinned line, and not a build made from this tree today |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 6. user guide | the "EAP-TLS with TLS 1.2 needs RFC 7627" section of `docs/guide/ipsec.md` names `charon.tls.version_max = 1.3` and the three answers; checked against `eapTLS12ExportRefused`, which lists the same three | Yes |
| 9. RFC behaviour | the RFC 5216 row of `docs/features/rfc-status.md`; `rfc/short/rfc5216.md` deliberately unchanged, because `RFC5216-2.3-1` says ze exports the 64-octet MSK with the RFC label and that is still exactly true. `make ze-rfc-check` -> `rfc-requirements OK: 2966 gated MUST-level requirement(s) across 171 enrolled RFC(s)` | Yes |
| 10. test infrastructure | `test/interop-ipsec/scenarios/eap-tls/ze-env` and the repurposed `check.py`; the `ze-env` comment in `test/interop-ipsec/lab.py` | Yes |
| 11. daemon comparison | grepping `docs/comparison.md` for `EAP-TLS` returns no hit, so there is no parity claim to reconcile | Yes, No applies |
| 12. internal architecture | `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`; the Proof section of `docs/architecture/ike/ipsec-11-interop-eap.md`. `docs/architecture/system-architecture.md` needed no edit: the guidance it summarises names no GODEBUG | Yes |
| 16. anchors on changed files | the `make ze-doc-verify` source-anchor stage reports `checked 2186 code paths, 517 packages` with one failure, and it names `docs/guide/web-interface.md`, another session's file | Yes |
| 16b. `docs/architecture/ike/ipsec-11-interop-eap.md` | answered No at design time and it is Yes now: scenario eap-tls's meaning changed, so its Proof section did go stale | Yes, updated |
| 17. examples setting the variable | the AC-4 walk above finds no assignable form outside `vendor/`, `testdata/` and the guard test | Yes |

## Core Insight

A safe default and its escape hatch are one decision, and a toolchain can remove
half of it with no diff to review. What made this recoverable was that the escape
hatch had a TEST holding it: scenario eap-tls set the variable, so the day the setting
went, the scenario went red and named the file. The guidance in `cmd/ze/main.go` had
no such holder and would have outlived the mechanism indefinitely, which is what the
new guard fixes. The general form: a capability documented in prose and exercised
nowhere is a capability nothing will tell you has gone.
