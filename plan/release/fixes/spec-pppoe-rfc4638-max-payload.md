# Spec: pppoe-rfc4638-max-payload

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | The fttx-edge release profile reaches RFC 4638. Ze's access concentrator echoes PPP-Max-Payload, which the RFC reserves for a server that supports an MRU above 1492, then holds every session at 1492: a shipped false capability claim, plus the BRAS MUSTs of the RFC Ze implements |
| RFC gaps | RFC4638-3-1, RFC4638-4-1, RFC4638-4-2, RFC4638-5.1-2, RFC4638-5.1-3, RFC4638-5.2-2 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make the PPPoE access concentrator's and the PPPoE client's MRU behaviour
match RFC 4638. No owner ruling names an MTU above 1492 unwanted on either
side, so the client row is owed work too (`ai/META-DECISIONS.md`, Goal and
scope).

| Row | Producer | What is missing |
|-----|----------|-----------------|
| RFC4638-4-2 | `internal/component/l2tp/pppoe/discovery.go::BuildPADO`, `BuildPADS` | PADO copies the client's PPP-Max-Payload tag whenever present, PADS never does, and the server cannot honour any value above 1492. The echo claims a capability the session does not have |
| RFC4638-3-1 | `internal/component/l2tp/pppoe/server.go` session start | `MaxMRU: PPPoEMaxMTU` (1492) for every session |
| RFC4638-5.1-2, RFC4638-5.1-3 | same | The tag and the access interface MTU never enter the LCP MRU bound |
| RFC4638-5.2-2 | none | No MRU-sized Echo-Request test exists, so the 1492 fallback it governs does not exist |
| RFC4638-4-1 | `internal/component/l2tp/pppoeclient/dialer.go::pppoeSessionMTU`, the client's PADI and PADR | The client refuses a configured session MTU above 1492 and puts no PPP-Max-Payload tag in PADI or PADR, so it cannot use a larger MRU |

RFC 4638 text, `rfc/full/rfc4638.txt`:

- Section 3: "a BRAS acting as a PPPoE server MUST support PPPoE MRU negotiations larger than 1492 octets in order to limit the amount of fragmented packets in networks similar to those described in Section 1."
- Section 4: "If a PPPoE client wants to use an MTU/MRU higher than 1492 octets, then it MUST include an optional PPP-Max-Payload Tag in the PADI and PADR packets."
- Section 4: "If the PPPoE server can support an MTU/MRU higher than 1492 octets, it MUST respond with an echo of the clients tag in the PADO and PADS packets when the PPP-Max-Payload tag is received from the client."
- Section 5.1: "If the PPP-Max-Payload tag is present and greater than 1492, it MUST be considered along with the server's interface MTU settings when the maximum value is selected for the normal RFC 1661 [2] MRU negotiation which decides the actual MRU to use."
- Section 5.2: "If these packets receive replies, the sending side MUST not send packets bigger than 1492 octets for this session."

The echo is the defect a client meets today; the rest is owed work of the RFC
Ze implements. Design reads accel-ppp (`accel-pppd/ctrl/pppoe/pppoe.c`, the
`ppp-max-payload` handling) and the Linux pppd `rp-pppoe` plugin, and quotes
the deciding functions. RFC 4638 Section 5.2's MRU test is a SHOULD; design
decides whether Ze runs it (making RFC4638-5.2-2 bind) or records it as a
feature outside scope with `excluded-kind: feature-out-of-scope`, per
`ai/rules/rfc-compliance.md`.

## Required Reading

- `rfc/short/rfc4638.md`, `rfc/full/rfc4638.txt` Sections 3 to 5.2
- `rfc/short/rfc2516.md` (PPPoE discovery)
- The PPPoE pages named for these files in `ai/CODE-TO-DOCS.md`

## Current Behavior (MANDATORY)

**Source files read:** (2026-10-26; design re-reads each before any decision)
- [ ] `internal/component/l2tp/pppoe/discovery.go` - `BuildPADO` copies a 2-octet PPP-Max-Payload; `BuildPADS` echoes Service-Name, Host-Uniq, Relay-Session-Id only; `PPPoEMaxMTU` is 1492
- [ ] `internal/component/l2tp/pppoe/server.go` - session start sets `MaxMRU: PPPoEMaxMTU`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- PADI and PADR frames on the access interface

### Transformation Path
1. Discovery parse, tag lookup
2. PADO and PADS build
3. `ppp.StartSession` with `MaxMRU` into LCP negotiation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPPoE discovery to PPP driver | `ppp.StartSession.MaxMRU` | No |

### Integration Points
- The access interface MTU (iface component) bounds the negotiated MRU

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A larger MRU with an intermediate segment that cannot carry it blackholes large packets | The accel-ppp interop scenario stalls on large pings | The Section 5.2 MRU test, or an interface MTU bound that stays at 1500 unless the operator raised it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PADI with PPP-Max-Payload 1500 on a 1508-MTU access interface | → | PADO/PADS echo and LCP MRU 1500 | `test/pppoe/` functional test and a `pppoe-*` interop scenario (to write) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4638Section4EchoOnlyWhenSupported` | `internal/component/l2tp/pppoe/` | AC-1 | |
| `TestRFC4638Section51MRUBound` | `internal/component/l2tp/pppoe/` | AC-2 | |

Each tagged unit carries a discrimination record. Scoped evidence:
`./le test unit` over the changed packages and the named `.ci` test.

## Files to Modify
- `internal/component/l2tp/pppoe/discovery.go` - conditional echo in PADO and PADS
- `internal/component/l2tp/pppoe/server.go` - MRU bound from tag and interface MTU
- `rfc/short/rfc4638.md` - remove the `{gap}` annotations

## Implementation Steps

1. Echo only when the access interface MTU supports the requested payload, in PADO and PADS
2. Carry the bound into `StartSession.MaxMRU`
3. Section 5.2 decision and, if run, the MRU test

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `published-value-drifts-from-the-behavior-it-describes.md` | 2026-10-26 | PADO echoes PPP-Max-Payload while LCP caps the MRU at 1492 |
| `rfc-extension-not-implemented.md` | 2026-10-26 | PPPoE client bounded at 1492, sends no PPP-Max-Payload tag |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | PADO and PADS echo PPP-Max-Payload only when the access interface can carry that payload; otherwise neither carries the tag |
| AC-2 | A client that sent PPP-Max-Payload N above 1492 negotiates an MRU of min(N, interface MTU minus 8) through LCP |
| AC-3 | A client that sent no tag negotiates at most 1492, as today |
| AC-4 | Each row is implemented or excluded with evidence, its `{gap}` removed, with tagged tests and discrimination records |
| AC-5 | The PPPoE client accepts a configured session MTU above 1492 and then carries a PPP-Max-Payload tag in its PADI and PADR; with a session MTU of 1492 or below it sends none |
