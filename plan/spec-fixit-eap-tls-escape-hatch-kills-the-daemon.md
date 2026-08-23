# Spec: fixit-eap-tls-escape-hatch-kills-the-daemon

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze's shipped instruction to an operator now stops the daemon from starting, and
the capability it enabled has no replacement.**

`cmd/ze/main.go` carries the reasoning for a deliberate fail-closed choice and
ends with the escape hatch: "An operator who must talk to such a peer sets
`GODEBUG=tlsunsafeekm=1` in the environment, which is a decision they take
knowingly and can audit."

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
   at all. strongSwan 5.9.14 is one: it caps at TLS 1.2 and does not negotiate
   7627. Before Go 1.27 an operator could opt in knowingly. Now there is no
   supported way to reach that peer, and nothing in the tree says so.

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
- [ ] A Go 1.27.0 binary run with `GODEBUG=tlsunsafeekm=1` fatals before `runtime.main`. Reproduced 2026-08-23 with a two-line program, so the failure is the toolchain's and not Ze's.

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
| Ze ↔ crypto/tls | `ExportKeyingMaterial` on the negotiated session | No |
| Ze ↔ peer | the EAP-TLS exchange | Yes, by scenario 04 failing with no IKE packet sent |

### Integration Points
- `exportEAPTLSMSK` - the single place the export is attempted and the only place that can produce a diagnosable error
- `cmd/ze/main.go` - the guidance that must stop naming a removed key

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Go offers no replacement knob for the unsafe EKM export in 1.27 | the removal note at `https://go.dev/doc/godebug#go-127` | a supported knob exists and the fix is to name it instead | read the Go 1.27 release notes and the `crypto/tls` source in GOROOT | unvalidated |
| A-2 | The refusal is reachable and diagnosable inside Ze, so a clear error can replace a runtime fatal | `exportEAPTLSMSK` selects by negotiated version, so the TLS 1.2 branch is a place Ze controls | the error cannot be attributed and the fix is documentation only | read `exportEAPTLSMSK` and the `crypto/tls` error it receives | unvalidated |
| A-3 | No deployment currently relies on the variable | it kills the daemon on Go 1.27, so any deployment that set it is already down | an operator on an older toolchain is relying on it and a removal surprises them | the release notes for the Go version Ze pins, and `go.mod` | unvalidated |

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
| An EAP-TLS peer negotiating TLS 1.2 without RFC 7627 | → | `exportEAPTLSMSK` | `TestEAPTLSExportRefusalNamesTheCause`, and `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` for what an operator reads <!-- doc-links: ignore (this spec's AC-2 creates this file; the spec is ready and not yet authorised to run) --> |
| The interop lab's EAP-TLS scenario against strongSwan 5.9.14 | → | the IKE EAP exchange | `test/interop-ipsec/scenarios/04-eap-tls` |

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
| `TestEAPTLSExportRefusalNamesTheCause` | `internal/component/ike/eap/` | validates AC-2: the refusal is attributed, not passed through raw | |
| `TestNoShippedGuidanceNamesARemovedGODEBUG` | `cmd/ze/` | validates AC-4 structurally, so the next removed key cannot be left in a comment | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Negotiated TLS version | 1.2-1.3 | 1.3 | 1.1 is refused already | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-tls12-refusal-is-attributed` | `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` | an operator reads the log after a TLS 1.2 EAP-TLS peer fails, and learns the peer, the TLS version and RFC 7627 rather than a raw crypto/tls string | <!-- doc-links: ignore (this spec's AC-2 creates this file; the spec is ready and not yet authorised to run) --> |
| `04-eap-tls` | `test/interop-ipsec/scenarios/04-eap-tls` | the TLS 1.2 peer path against a real strongSwan, whatever its resolution | |

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
- `test/ipsec/ipsec-eap-tls12-refusal-is-attributed.ci` - the AC-2 proof, and the only functional evidence that does not need the Docker lab <!-- doc-links: ignore (this spec's AC-2 creates this file; the spec is ready and not yet authorised to run) -->
- nothing else until A-1 is settled. Whether a replacement mechanism exists decides whether this spec adds code or removes a claim

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
| Doctor check for runtime dependencies | Yes | a doctor check can name a removed GODEBUG key present in the environment, which is a runtime condition an operator cannot otherwise diagnose |
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
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 5216 Section 2.3 is unreachable for TLS 1.2 without RFC 7627. `rfc/short/rfc5216.md` and the `docs/features/rfc-status.md` row must say so rather than implying the path works |
| 10 | Test infrastructure changed? | Yes | the interop lab's `ze-env` for scenario 04 |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it claims EAP-TLS parity with a daemon that still reaches such peers |
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md`, the design doc `cmd/ze/main.go` declares |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `cmd/ze/main.go` and the eap package |
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
