# Spec: vpn-concentrator-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | - (the `vpn-concentrator` profile is `Release: no` in `rfc/profiles/vpn-concentrator.md`, so its RFC work waits until after the first release under the release-timing rule in `ai/META-DECISIONS.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | umbrella: three children, one in design, two skeletons |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas decided on 2026-11-08 (O-2 of `spec-ike-rfc8420-eddsa-conformance.md`,
answered "yes to all recommendations") that the vpn-concentrator work gets an
umbrella spec parenting the three specs that already exist for it.

The `vpn-concentrator` profile is Ze as an IPsec concentrator for remote-access
clients: an IKEv2 responder that authenticates many road-warrior peers, by
certificate or by EAP. The RFCs it lists are declared by the summaries
themselves (`Use cases` and `Optional use cases` Meta rows), never by the
profile file, so this umbrella holds no RFC list of its own: `./le rfc tree
profile vpn-concentrator` prints it.

The umbrella's job is to keep the profile's open work visible as one item, to
order its children, and to close when every child has closed:

| Child | Spec | Work it owns | RFC | Tier in the profile | Status |
|-------|------|--------------|-----|---------------------|--------|
| 1 | `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` | EdDSA (Ed25519, Ed448) in IKEv2 brought onto the RFC ledger: summary, extraction walk, tags, discrimination records and the four missing tests. Ze already implements it | RFC 8420 | optional (owner decision O-1, 2026-11-08) | design |
| 2 | `plan/post-release/features/spec-ike-eap-only-authentication.md` | EAP-only responder authentication: the EAP_ONLY_AUTHENTICATION notify, the Section 4 method list, AUTH from the EAP key alone | RFC 5998 | required | skeleton |
| 3 | `plan/post-release/features/spec-ike-multiple-authentication-eap-rounds.md` | The parts of multiple authentication rounds left unbuilt after the closed `spec-ike-multiple-authentication`: EAP in a second round and the remote-access cases | RFC 4739 | required | skeleton |

Each child keeps its own design, acceptance criteria and evidence. The
umbrella adds no requirement and no code.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` (IKE)
  → Constraint: IKE is the `ze_ike` feature gate; readers of IKE state use the core leaves `ipsecinventory` and `ikeprobe`, never `ike/engine`. Every child stays inside `ike/engine`, `ike/wire` and their core leaves
  → Constraint: the proof suite is `./le test integration interop-ipsec` against strongSwan, and `./le rfc check` for every tagged requirement
- [ ] `ai/META-DECISIONS.md`, "Goal and scope"
  → Constraint: "An RFC for an optional transport or feature is listed for the profile as optional, and it counts toward the profile's percentage only once Thomas brings it into scope. Every RFC a profile lists, required or optional, has a valid summary."
  → Constraint: "RFCs no release profile declares wait until after the first release." vpn-concentrator is `Release: no`, so every child is P3 in `post-release/features`
- [ ] `docs/contributing/rfc-conformance-gates.md`, "Profiles and aggregates"
  → Decision: membership of the profile is the summaries' Meta rows; this umbrella and its children never edit `rfc/profiles/vpn-concentrator.md`
- [ ] `rfc/profiles/vpn-concentrator.md`
  → Decision: `Kind: use-case`, `Release: no`
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/architecture/ike/ipsec-14-responder.md`
  → Constraint: the responder's EAP authenticator and the Digital Signature table are the mechanisms the children extend (Existing Mechanisms)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/auth_signature_scheme.go` - the registered Digital Signature family table child 1 tests; the header still says "no summary: no release profile lists RFC 8420", which child 1 rewrites
- [ ] `internal/component/ike/engine/responder_eap.go` - the responder's EAP authenticator (`eapMethodConfig`), extended by children 2 and 3
- [ ] `internal/component/ike/engine/eap_auth.go` - AUTH from a shared secret for an EAP exchange (`computeAuthFromSharedSecret`), where child 2's EAP-key-only AUTH lands
- [ ] `internal/component/ike/wire/payload_notify.go` - the notify type constants; EAP_ONLY_AUTHENTICATION (16417) is absent (child 2)

**Behavior to preserve:**
- every site-to-site and remote-access behaviour the three children list as preserved

**Behavior to change:**
- none at umbrella level; each child owns its own

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Umbrella specs | `spec-home-cpe-0-umbrella` (closed), `plan/release/validation/spec-doctor-coverage-0-umbrella.md` | A `-0-umbrella` spec lists its children with a Parent line in each child and closes when they close | extends: the same shape for the vpn-concentrator profile |
| Profile membership by summary Meta rows | `internal/le/rfc/meta.go` `readUseCases`, `checkUseCases`; `rfc-conformance-gates.md` "Profiles and aggregates" | A summary declares the profiles it belongs to, required or optional | does not apply as a change: the umbrella lists children, never RFCs; `./le rfc tree profile vpn-concentrator` is the RFC list |
| RFC ledger tooling | `./le rfc check`, `./le rfc record`, `./le rfc extraction create` | Gates summaries, tags and discrimination records | does not apply as a change: each child uses it for its own RFC |
| IKE responder EAP authenticator | `ike/engine/responder_eap.go`, `eap_auth.go` | EAP-MSCHAPv2, EAP-TLS and MD5-Challenge on the responder | does not apply at umbrella level: children 2 and 3 extend it |
| Digital Signature family table | `ike/engine/auth_signature_scheme.go` `signatureSchemes`, `announcedHashAlgos` | Registered signature families answer selection, signing, verification and the notify list | does not apply at umbrella level: child 1 tests it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- None at umbrella level. Each child's entry point is a remote-access client's IKE_SA_INIT and IKE_AUTH reaching Ze's responder, and the RFC ledger for child 1.

### Transformation Path
1. Each child carries its own path, through `ike/engine` on the product side and `internal/le/rfc` on the ledger side.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to strongSwan | IKEv2 on UDP 500 and 4500 | by each child's interop scenarios |

### Changed-Wire Census

**Search evidence:** none at umbrella level; the umbrella changes no code. Each child carries its own census.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| - | - | - | - | - |

### Integration Points
- None at umbrella level.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | N-A | no code |
| No unintended coupling (components stay isolated) | N-A | no code |
| No duplicated functionality (extends existing, does not recreate) | Yes | the umbrella shape copies the existing `-0-umbrella` specs |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no code |
| Registration over hardcoding, outbound | N-A | no code |
| Registration over hardcoding, inbound | Yes | the profile's RFC list stays derived from the summaries; the umbrella copies no RFC list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three children are every open spec that serves the vpn-concentrator profile | grep of `plan/` for `vpn-concentrator` on 2026-11-08: the three children, a journal file, a handover, the generated roadmap and the implementing `spec-home-cpe-5-ike-eddsa.md` (a release fix for vpn-endpoint, not this profile) | a fourth spec gains a Parent line here | the grep re-run when a child closes | validated (grep) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A profile RFC with a gap and no spec | `./le rfc tree profile vpn-concentrator` lists unproven rows no child owns | a new child spec is written and listed in the Task table |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | nothing in the product; the umbrella is planning only |
| How is it reverted? | single commit revert |
| Who else touches this path? | the three children |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| remote-access client IKE_AUTH | → | each child's producer | each child's own Wiring Test table |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Child 1 closed | RFC 8420 is enrolled with `Optional use cases: vpn-concentrator`, and `./le rfc tree profile vpn-concentrator` shows its rows in the optional tier | none |
| AC-2 | Child 2 closed | RFC 5998 rows RFC5998-3-1 to 3-4 and 4-1 are no longer `{gap}` and each has a discrimination record | none |
| AC-3 | Child 3 closed | the RFC 4739 rounds the child names are built and proven against strongSwan | none |
| AC-4 | Every child closed | every `{gap}` row `./le rfc tree profile vpn-concentrator` lists is owned by a spec named in it, so each remaining gap has the spec that schedules it; an unowned row is the failure | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| each child's unit tests | the children | AC-1 to AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| each child's `.ci` and interop scenarios | the children | a remote-access client connects | |

## Files to Modify
- none at umbrella level; each child lists its own

## Files to Create
- none at umbrella level

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | children decide |
| YANG validation constraints | N-A | children decide |
| YANG custom validators | N-A | children decide |
| CLI commands/flags | N-A | children decide |
| CLI grammar (keyword before value) | N-A | children decide |
| Editor autocomplete | N-A | children decide |
| Functional test for new RPC/API | N-A | children decide |
| Pipe completeness | N-A | children decide |
| Env var registration | N-A | children decide |
| Doctor check for runtime dependencies | N-A | children decide |
| Prometheus counters/metrics | N-A | children decide |
| BGP family surface (new SAFI / capability / attribute) | N-A | IKE, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | children |
| 2 | Config syntax changed? | No | children |
| 3 | CLI command added/changed? | No | children |
| 4 | API/RPC added/changed? | No | children |
| 5 | Plugin added/changed? | No | children |
| 6 | Has a user guide page? | No | children |
| 7 | Wire format changed? | No | children |
| 8 | Plugin SDK/protocol changed? | No | children |
| 9 | RFC behavior implemented, changed, or newly proven? | No | children |
| 10 | Test infrastructure changed? | No | children |
| 11 | Affects daemon comparison? | No | children |
| 12 | Internal architecture changed? | No | children |
| 13 | Route metadata keys added/changed? | No | children |
| 14 | Prometheus counters added/changed? | No | children |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | children |
| 16 | Any changed source file referenced by existing doc source anchors? | No | no source changes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | children |

## Implementation Steps

1. **Child 1** - RFC 8420 conformance: needs no product change, so it runs first.
2. **Child 2** - RFC 5998, after its design phase moves it from skeleton.
3. **Child 3** - RFC 4739 EAP rounds, after its design phase; it may share the responder EAP changes child 2 makes, so it follows child 2.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every vpn-concentrator gap row has an owning child |
| Scope | the umbrella edits no profile file and adds no requirement |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Children closed | the three child spec files are removed by their closure commits |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Remote-access authentication | each child keeps the refusal paths its RFC requires |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A child finds a gap outside its RFC | a new child spec listed in the Task table |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The profile is defined by its summaries, so the umbrella orders work and never lists RFCs; a list here would be a second copy of what `./le rfc tree profile vpn-concentrator` prints.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none owned at umbrella level; each child lists its own |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An umbrella parents the three vpn-concentrator specs | siblings joined by Related lines | Thomas decided (O-2 of child 1, 2026-11-08) |
| `post-release/features`, P3 | `release/` | `vpn-concentrator` is `Release: no`; META: RFCs no release profile declares wait until after the first release |
| RFC 8420 is optional in the profile | required | Thomas decided (O-1 of child 1, 2026-11-08) |

## Known Limitations
- Child 2 and child 3 are skeletons: their designs are owed before they move to `ready`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (RFC 8420: written by child 1)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests (N-A: umbrella)
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every child closed
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
- [ ] Each child's targeted tests pass in the main checkout, as each child names them, and `./le rfc check`

### TDD
- [ ] Tests written (in the children)
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** edited spec
- [ ] **Commit B:** `remove plan/post-release/features/spec-vpn-concentrator-0-umbrella.md` only
