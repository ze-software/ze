# Spec: rfc4861-rfc8106-enrolment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | rfc |
| Depends | - (was `spec-router-advertisement`, delivered and closed 2026-09-07) |
| Phase | 0/1 |
| Handoff | - |
| Updated | 2026-09-07 |

## Task

Ze sends IPv6 Router Advertisements on two paths, and neither RFC behind them is
on the public conformance ledger.

`BuildRA` (`internal/core/ndp/ra.go`) encodes the RFC 4861 Section 4.2 message,
the Section 4.6.1 Source Link-layer Address option, the Section 4.6.2 Prefix
Information option and the RFC 8106 Section 5.1 RDNSS option. `UnsolicitedInterval`,
`SolicitedDelay`, `SolicitedSendTime` and `CeaseWait`
(`internal/core/ndp/schedule.go`) implement the Section 6.2 schedule.
`Sender.run` (`internal/plugins/iface/ra/sender_linux.go`) drives them on a LAN
unit, and `raSenderLoop` (`internal/component/l2tp/ppp/ra_send.go`) drives them
for a PPP subscriber. `raValidate` (`internal/component/iface/config_ra.go`)
enforces the Section 6.2.1 bounds on the router configuration variables.

`rfc/short/rfc4861.md` and `rfc/short/rfc8106.md` do not exist, so no requirement
of either RFC is extracted, no tagged test carries either id, and `./le rfc check`
names neither. `rfc/full/rfc8106.txt` is also absent and has to be fetched first.
`rfc/short/rfc4862.md` is enrolled and covers the SLAAC receive side only.

This spec enrols both. It is the home the closure of `spec-router-advertisement`
points at, and it is the only live record of the item: that spec's Known
Limitations section went with it on 2026-09-07.

## Why this is not `spec-followup-rfc-enrollment`

That spec enrols the summaries that EXIST and are marked "not enrolled" in
`ai/RFC-REQUIREMENTS.md`, ranked nearest-to-enrollable. RFC 4861 and RFC 8106
have no summary at all, so they appear in no rollup and no ranking reaches them.
Nothing about them is visible to the program that spec runs.

## What the work is

- Fetch `rfc/full/rfc8106.txt`
  (`curl -o rfc/full/rfc8106.txt https://www.rfc-editor.org/rfc/rfc8106.txt`).
  `rfc/full/rfc4861.txt` is already in the tree.
- Write both summaries with `/ze-rfc`.
- Extract every requirement, and enrol both stems in `rfc/enrolled.txt`.
- Tag the tests that already prove requirements, and write the tests that are
  missing, in both polarities where the requirement admits one.
- Record the discrimination for each tag with `./le rfc discriminate-record`.
- The two `docs/features/rfc-status.md` rows follow from the `## Meta` table of
  each summary and `./le rfc index-update`. They are never hand-edited.

## The size, measured 2026-09-05

`rfc/full/rfc4861.txt` carries 151 MUST/SHALL occurrences across 97 pages, and
they cover the whole of Neighbor Discovery. Ze produces one part of it, the
router send side. Neighbor Solicitation and Advertisement, Redirect, address
resolution, Neighbor Unreachability Detection and the whole host RA processing
path are performed by the Linux kernel on Ze's behalf.

RFC 8106 is 5 pages and one option.

## The owner decision this spec's design phase owes

The 2026-08-31 owner directive puts a requirement met through a lower layer at
MET rather than `{not-applicable}`, and makes each such requirement owe a test
asserting what Ze installs for the layer below. For most of RFC 4861 what Ze
installs is a handful of `accept_ra` sysctls
(`internal/core/sysctl/known_linux.go`), so what each of those 151 lines is worth
is a judgement about scope rather than a lookup.

`ai/rules/rfc-compliance.md` reserves each of those calls to Thomas: a session
may not write `{gap}`, `{not-applicable}` or `binds-another-role` on its own
authority, and `binds-another-role` is presumed wrong before it is written. Ze
speaks both roles of Neighbor Discovery on a link it advertises on, so that
label is unlikely to be right for much of the document.

The question to put to him is which way each tranche is classified, never
whether the enrolment happens.

## Related

- `docs/features/interfaces.md`, the operator page for what Ze advertises.
- `plan/pre-release/spec-followup-rfc-enrollment.md`, the program this sits
  beside.
- `plan/spec-router-advertisement-options-out-of-scope.md`, the options the
  delivering spec left out. An option Ze does not offer excludes the obligations
  conditional on it, so that list and this enrolment read together.
- `internal/core/ndp/ra.go`, `internal/core/ndp/schedule.go`,
  `internal/plugins/iface/ra/sender_linux.go`,
  `internal/component/l2tp/ppp/ra_send.go`,
  `internal/component/iface/config_ra.go`: the producers every requirement is
  judged against.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` - what a session may and may not decide about a
  requirement, and the shape of the question that goes to Thomas
- [ ] `docs/contributing/rfc-conformance-gates.md` - the enrolment walk, the
  extraction sign-off, the eight ratchets, and the discrimination record
- [ ] `docs/features/interfaces.md` - the operator surface the requirements are
  judged against

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4861: no `rfc/short/rfc4861.md` exists (checked 2026-09-07). The full
  text is `rfc/full/rfc4861.txt`. The summary is the first deliverable, written
  with `/ze-rfc`, and no requirement is quoted from anywhere else until it does.
- [ ] RFC 8106: no summary and no full text (checked 2026-09-07). Fetch
  `rfc/full/rfc8106.txt` first.
- [ ] `rfc/short/rfc4862.md` - enrolled, covers the SLAAC receive side, and
  bounds what this spec adds.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/ndp/ra.go` - `BuildRA`, `RALen`, `writeSourceLinkLayerAddress`,
  `writePrefixOptions`, `writeRDNSS`, `MessageHopLimit`. Every RFC 4861 Section 4
  and RFC 8106 Section 5.1 layout Ze emits is here, each carrying a section
  reference in a comment and none carrying a requirement id.
- [ ] `internal/core/ndp/schedule.go` - `UnsolicitedInterval`, `SolicitedDelay`,
  `SolicitedSendTime`, `CeaseWait`. The Section 6.2 and Section 10 arithmetic,
  shared by both senders. Its header states that RFC 4861 is not enrolled.
- [ ] `internal/component/iface/config_ra.go` - `raValidate` enforces the
  Section 6.2.1 bounds on the router configuration variables.
- [ ] `internal/plugins/iface/ra/sender_linux.go` - `openRASocket`, `Sender.run`,
  `Sender.sendFinal`: the LAN send side.
- [ ] `internal/component/l2tp/ppp/ra_send.go` - `raSender.send`, `raSenderLoop`,
  `stopRASender`: the PPP subscriber send side.
- [ ] `rfc/enrolled.txt` - neither stem appears.

**Behavior to preserve:** every wire behavior above. This spec adds evidence, and
a requirement it finds unmet is a defect fixed at its producer.

**Behavior to change:** none, until a requirement says otherwise.

## Data Flow (MANDATORY)

### Entry Point
`rfc/full/rfc4861.txt` and `rfc/full/rfc8106.txt`, read by `/ze-rfc` into
`rfc/short/<stem>.md`.

### Transformation Path
1. `/ze-rfc` writes each summary, requirement by requirement.
2. Extraction turns each summary into requirement ids `./le rfc check` reads.
3. Each id is bound to a tagged test, or to a reasoned annotation Thomas ruled on.
4. `./le rfc discriminate-record` observes the red for each tag and stores it.
5. `./le rfc index-update` regenerates `docs/features/rfc-status.md` from the
   `## Meta` table of each summary.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RFC text ↔ summary | `/ze-rfc` extraction, signed off | [ ] |
| summary ↔ gate | `rfc/enrolled.txt` plus `./le rfc check` | [ ] |
| requirement ↔ test | `RFC requirement:` tag on a test that fails without the producer | [ ] |
| summary ↔ public ledger | `./le rfc index-update` | [ ] |

### Integration Points
`rfc/short/`, `rfc/full/`, `rfc/enrolled.txt`, `rfc/discrimination/`,
`docs/features/rfc-status.md`, and the test files of the five producers above.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over this tree | → | both stems enrolled, every requirement bound | the gate's own exit status |
| `ze` sends an advertisement | → | the producer each requirement names | the tagged test for that requirement |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named at design time, one per requirement that owes a test | `internal/core/ndp/ra_test.go`, `internal/core/ndp/schedule_test.go`, `internal/component/iface/config_ra_test.go`, `internal/plugins/iface/ra/sender_linux_test.go`, `internal/component/l2tp/ppp/ra_send_test.go` | the MUST named in its `RFC requirement:` tag | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-ra-slaac` | `test/plugin/iface-ra-slaac.ci` | exists; a requirement proven on the wire tags it rather than adding a second one | |

## Files to Modify
- `rfc/enrolled.txt` - both stems
- `docs/features/rfc-status.md` - regenerated, never hand-edited
- the five producer test files named above, for the tags

## Files to Create
- `rfc/full/rfc8106.txt`
- `rfc/short/rfc4861.md`, `rfc/short/rfc8106.md`
- `rfc/discrimination/<stem>.json` per tagged unit

## Implementation Steps

1. Fetch RFC 8106 and write both summaries with `/ze-rfc`.
2. Extract, and put the classification tranches to Thomas.
3. Tag what is proven; write what is not.
4. Record the discrimination for every tag.
5. Enrol both stems and regenerate the ledger.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | no | no config surface changes |
| CLI commands | no | - |
| RFC enrolment | yes | `rfc/enrolled.txt` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | yes | `rfc/short/rfc4861.md`, `rfc/short/rfc8106.md`, then `./le rfc index-update` |
| 11 | Affects daemon comparison? | check | `docs/comparison.md` if a support level moves |

## Checklist

### Goal Gates (MUST pass)
- [ ] Both stems in `rfc/enrolled.txt` and `./le rfc check` names no finding for either
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
