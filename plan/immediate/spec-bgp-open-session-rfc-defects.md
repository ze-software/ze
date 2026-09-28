# Spec: bgp-open-session-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Four defects in how Ze builds and reports its BGP OPEN exchange, found by the
RFC re-read and the softver interop work of
`spec-rfc-requirement-quote-hand-backfill`. Two are RFC
defects: a peer misframes Ze's OPEN, and a per-peer override gives peers
different BGP Identifiers. Two are Ze behaviour defects that no RFC sentence
states, each recorded by its journal row: a changed capability does not reach
the wire after a reload, and a peer refusing Ze's OPEN leaves no line at the
default log level. Each was confirmed open in HEAD on 2026-09-27.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|--------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9072 Section 2 (RFC9072-2-1) | "The rules for encoding Optional Parameters are unchanged with respect to those given in [RFC4271], except that the Parameter Length field is extended to be a two-octet unsigned integer." | `internal/component/bgp/reactor/session_negotiate.go::buildOptionalParams` | The one-octet form is chosen while `capTotal <= 255`, so a capability total of 254 or 255 yields 256 or 257 octets of Optional Parameters with `ExtendedParams` false; `Open.WriteTo` (`internal/component/bgp/message/open.go`) then writes the extended envelope (optLen above 255) around a parameter with a one-octet length, which a conformant receiver misframes. The boundary is 2 plus capTotal | no verdict; read, not reproduced |
| D4 | RFC 6286 Section 2.1 (RFC6286-2.1-1) | "The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." | `internal/component/bgp/reactor/config.go::parsePeerSettings` ("Router ID from session > router-id (peer-level overrides global)") | A peer-level `router-id` gives that peer a BGP Identifier different from the others'; `docs/features/rfc-status.md` advertises the override as supported. The sentence carries no BCP 14 keyword and sits in the Section 2.1 definition; the row labels it MUST | weak (TestParsePeerFromTreeInvalid asserts only non-zero); OWNER DECISION |

### Ze behaviour defects (no RFC sentence)

Neither row is an RFC defect: no RFC sentence states the behaviour, so neither
carries an RFC tag, a discrimination record or a conformance count. Each is
owed because an operator meets it, and its journal row is its record.

| ID | Journal row | Quote | Producer | What Ze does wrong | Record |
|----|-------------|-------|----------|--------------------|--------|
| D2 | `plan/journal/unwired-feature.md`, 2026-09-27 | none: a reload defect | `sdk.Plugin.Run` capability declaration at stage 3; `internal/component/bgp/plugins/softver/softver.go` `OnConfigure`; same shape in `hostname`, `role`, `llnh`, `gr` | A SIGHUP reload that changes a plugin-declared capability (softver `encoding`) bounces the session, but the new OPEN keeps the startup capability (code reading; wire not captured) | journal row, Open |
| D3 | `plan/journal/silent-fall-through.md`, 2026-09-27 | none: an observability defect | `internal/component/bgp/reactor/session_handlers.go::Session.handleNotification`, the reconnect loop in `peer_run.go` | A NOTIFICATION received in answer to Ze's OPEN is logged only at DEBUG, so a peer that refuses the OPEN never comes up and no line says why (seen in the `frr-software-version` interop scenario) | journal row, Open |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a unit test at capTotal 253, 254, 255 and 256 asserting the envelope and the per-parameter length agree. D2: a test reloading a changed softver encoding and asserting the next OPEN. D3: a test asserting the WARN line |
| Both polarities | D1: 253 stays one-octet with no envelope; 254 upward uses the RFC 9072 form throughout. D4, if ruled a defect: a peer-level router-id refused at config load, the global one accepted |
| Discrimination | `./le rfc discriminate-record` for RFC9072-2-1, and for RFC6286-2.1-1 if D4 is ruled a defect; none for D2 and D3, which carry no RFC tag |
| Real entry point | `.ci` tests for D2 (reload) and D3 (peer refuses OPEN) |
| Interop | D1 against FRR with a capability set sized to 254 octets; D2 and D3 through `frr-software-version` |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | Re-declare capabilities on config apply, or refuse a capability change the reload cannot apply? |
| D4 | Remove the per-peer `router-id` override, or keep it as an owner-approved deviation from RFC 6286 Section 2.1, recorded with its reason, not counted as conformant, and the `docs/features/rfc-status.md` claim corrected? |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9072.md`, `rfc/full/rfc9072.txt` Section 2
- [ ] `rfc/short/rfc6286.md`, `rfc/full/rfc6286.txt` Section 2.1

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `buildOptionalParams`
- [ ] `internal/component/bgp/message/open.go` - `Open.WriteTo`
- [ ] `internal/component/bgp/plugins/softver/softver.go` - `OnConfigure`, `SetCapabilities`
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleNotification`
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerSettings` (router-id)

**Behavior to change:** the rows above.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- session start (OPEN built), SIGHUP reload, a NOTIFICATION received during OpenSent

### Transformation Path
1. plugins declare capabilities; the reactor collects them
2. `buildOptionalParams` then `Open.WriteTo`
3. the peer answers OPEN or NOTIFICATION; the reactor logs and reconnects

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | capability declaration at stage 3 | No |

### Integration Points
- the plugin SDK capability declaration and the reload path

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with a changed softver encoding | → | capability re-declaration | [to fill in design: `.ci`] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | capabilities totalling 254 or 255 octets | OPEN uses the RFC 9072 form with two-octet parameter lengths |
| AC-2 | reload changing a plugin-declared capability | the next OPEN carries the new capability, or the reload is refused with a reason |
| AC-3 | peer answers Ze's OPEN with a NOTIFICATION | one line at WARN or INFO naming the peer, code and subcode |
| AC-4 | the `weak` verdicts of RFC6286-2.1-1 and RFC9072-2-1, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| boundary test over capTotal | `internal/component/bgp/reactor/` | D1 | [to fill in design] |

### Functional Tests
- [to fill in design]

## Files to Modify

- the producers in the defect table and their tests

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination record for RFC9072-2-1; 4. close the two journal rows.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
