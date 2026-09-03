# Spec: radius-admin-interop-freeradius

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `internal/le/interoplab/lab.go` and `docker.go` -- the lab machinery this reuses.
3. `internal/le/interoplab/l2tp/l2tp.go` -- the closest existing suite, and the model for image builds, container specs and ready probes.
4. `internal/component/radius/authenticator.go` -- `(*radiusAuthenticator).Authenticate` and `(*radiusAuthenticator).credential`, the code under test.
5. `docs/guide/radius.md` -- the admin config surface the scenario drives.

## Task

No RADIUS behavior in ze is proven against a RADIUS server ze did not write.
Every proof runs against a mock: `test/plugin/aaa-radius-admin.ci` and
`aaa-radius-fallback.ci` drive `internal/test/mock/radius/radius.go`, and the
L2TP interop suite's peer is `internal/le/interoplab/l2tp/radiusmock/`, which is
ze's own Go program in a container.

That was acceptable while the admin backend sent one credential. It is not now.
Commit `0971cff50d` added CHAP, so ze computes a digest a server must reproduce
from its own stored password, and `plan/spec-radius-admin-eap.md` adds a
multi-round exchange with a Message-Authenticator ze signs and a State attribute
ze echoes. A mock built beside ze's encoder agrees with ze by construction. A
real server is the only thing that can disagree.

**The skeleton this replaces was wrong about the cost, and the correction is why
this now runs.** Written 2026-07-08, it called a RADIUS peer "a distinct
infrastructure build" because the harness of the day was `test/interop/interop.py`,
a Python BGP-peer harness with no notion of another daemon type. That harness was
retired on 2026-08-28. The Go lab that replaced it registers peers
declaratively: `interoplab.ImageBuild` takes either a `Dockerfile` and a
`Context` or `Pull: true` with a tag, which is how `quay.io/frrouting/frr:10.3.1`
arrives in the L2TP suite today. A FreeRADIUS peer is a pulled image, a config
directory and a checker.

### Its own suite, not an extension of the L2TP one

`internal/le/interoplab/radius/`, beside `bgp`, `ipsec`, `l2tp` and `pppoe`.

The reason is a gate, not tidiness. The L2TP suite probes for the `l2tp_ppp` or
`pppol2tp` kernel module and refuses to run without it
(`internal/le/interoplab/l2tp/l2tp.go`), which is correct for a suite that
carries PPP sessions over L2TP. Admin login carries none: it is ze's SSH or web
listener, a UDP socket, and a RADIUS server. Hosting these scenarios in the L2TP
suite would inherit a kernel dependency they do not have, and the practical cost
is that they would stop running anywhere the module is absent, which is where
most of this repository's development happens.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` -- scenario naming, the discrimination
  walk, and the four vacuity traps.
  → Constraint: the scenario directory is NAMED with no numeric prefix, and the
  strengthened assertion owes a forced RED before it can be claimed to discriminate.
- [ ] `docs/guide/radius.md` -- the admin config surface, the chain order, and the
  profile mapping this proves.
- [ ] `ai/rules/interop-and-goal-validation.md` -- what an interop test owes.
- [ ] `ai/rules/platform-linux.md` -- what a Linux-only proof owes, and why
  "needs hardware" is not a reason to skip.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2865.md` -- Access-Request, Access-Accept, Access-Reject,
  Filter-Id (Section 5.11), CHAP-Password (Section 5.3), CHAP-Challenge (Section 5.40).
  → Constraint: the scenarios assert what the SERVER accepted, not what ze built.
- [ ] RFC 2865 Section 2.2 -- CHAP requires the server to hold the password in
  cleartext.
  → Constraint: the FreeRADIUS user entry for the CHAP scenario is a
  `Cleartext-Password`, and the PAP scenario proves a hashed entry still works,
  because that difference is the operator tradeoff ze's guide documents.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/le/interoplab/l2tp/l2tp.go` -- `imageBuilds` returns
  `[]interoplab.ImageBuild`, mixing built images (`Dockerfile` plus `Context`)
  with pulled ones (`Pull: true`, used for FRR). Container specs carry `Image`,
  `Host` and a `ReadyProbe`; the RADIUS peer's probe greps `/proc/net/udp` for
  port 0714.
- [x] `internal/le/interoplab/l2tp/radiusmock/Dockerfile` -- a two-stage Go build
  of ze's own mock. This is the thing a real server replaces.
- [x] `internal/le/interoplab/discover.go` -- `Discover` refuses a scenario
  directory the suite's registry does not name, so an empty directory breaks the
  run rather than being skipped.
- [x] `internal/component/radius/authenticator.go` -- `(*radiusAuthenticator).credential`
  selects one credential per method and never appends both.

**Behavior to preserve:** every existing suite; the `.ci` mock proofs, which stay
and remain the fast feedback loop.

**Behavior to change:** none in the product. This spec adds a suite.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le integration` selecting the new suite, then a scenario that logs in to ze
over SSH with credentials FreeRADIUS holds.

### Transformation Path
1. The lab pulls a FreeRADIUS image and starts it with the scenario's config.
2. It starts ze with a `system authentication radius` block pointing at it.
3. The scenario drives a login through ze's real SSH listener.
4. The checker asserts what ze concluded AND what the server logged, so a login
   that succeeded for the wrong reason fails.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Lab → FreeRADIUS | a pulled image plus a mounted config directory | [ ] |
| Ze → FreeRADIUS | a real Access-Request over UDP | [ ] |
| Checker → both sides | ze's log and the server's detail log | [ ] |

### Integration Points
- `internal/le/interoplab/radius/` -- the new suite: lab, containers, checkers.
- `test/interop-radius/scenarios/<name>/` -- the scenario configs.
- Whatever registers a suite with `./le integration`.

### Architectural Verification
The suite follows the four that exist rather than inventing a shape. Nothing in
`internal/component/radius` changes, which is the point: an interop test that
required a product change would be testing the change.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A FreeRADIUS peer can be a pulled image | FRR is pulled in the L2TP suite through `ImageBuild{Pull: true}` | A Dockerfile is needed, which is still small | AC-1 | unvalidated |
| A-2 | The admin path needs no kernel module | it is a UDP socket and ze's own listeners | The suite inherits a gate after all, and the L2TP suite becomes the right home | AC-8 | unvalidated |
| A-3 | FreeRADIUS can be made to log enough for the checker to read the server's side | its detail log module | The checker asserts ze's side only, which is weaker and must be said | AC-4 | unvalidated |
| A-4 | The CHAP scenario needs a `Cleartext-Password` entry | RFC 2865 Section 2.2 | The scenario proves less than it claims | AC-3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The scenario passes because ze fell through to local bcrypt | a green run with no RADIUS traffic | AC-5 asserts `source=radius` in ze's log AND a matching entry on the server; a fall-through fails both |
| R-2 | The suite is written and never run in CI | no CI job names it | The CI wiring is a deliverable, not a follow-up |
| R-3 | A pulled image version drifts and the scenario changes meaning | a sudden failure with no ze change | The tag is pinned exactly, as `quay.io/frrouting/frr:10.3.1` is |

## Blast Radius

New files only, plus whatever registry line makes `./le integration` aware of the
suite, plus a CI job. No product code changes.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le integration` naming the new suite | → | a FreeRADIUS container answers a real Access-Request | the `radius-admin-pap-freeradius` scenario |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The suite runs | A real FreeRADIUS container starts and becomes ready under a probe, not a sleep |
| AC-2 | `radius-admin-pap-freeradius` | An operator logs in over ze's SSH listener; FreeRADIUS returns Access-Accept; ze's log says `source=radius` |
| AC-3 | `radius-admin-chap-freeradius` | `auth-method chap`, a `Cleartext-Password` user entry, and a successful login. This is the proof commit `0971cff50d` owes: a server ze did not write reproduces ze's digest |
| AC-4 | Both scenarios | The checker reads the SERVER's record of the request as well as ze's log, so a login satisfied by any other backend fails |
| AC-5 | A wrong password | Access-Reject, the chain stops, and ze does NOT fall through to a local account |
| AC-6 | Filter-Id in the Access-Accept | The named profile is attached, asserted through ze's own authorization surface |
| AC-7 | `auth-method chap` against a user entry the server stores HASHED | The login is rejected, which is RFC 2865 Section 2.2's consequence and the operator tradeoff `docs/guide/radius.md` documents. A negative scenario, and the one that proves the guide is not lying |
| AC-8 | The suite on a host with no `l2tp_ppp` | It runs. Nothing here needs a kernel module |
| AC-9 | Discrimination | Each scenario is shown RED against a broken producer, per `docs/architecture/testing/interop.md`, and the RED is recorded |
| AC-10 | CI | A job runs this suite, named in the CI config |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Points ze at a real RADIUS server and logs in | SSH → AAA chain → Access-Request → Accept → profiles | `radius-admin-pap-freeradius` |
| 2 | Switches to `auth-method chap` and logs in | the same, with a CHAP credential | `radius-admin-chap-freeradius` |
| 3 | Switches to CHAP against a server storing hashes | the same, rejected | `radius-admin-chap-hashed-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the suite's own lab test, following `internal/le/interoplab/l2tp/l2tp_test.go` | `internal/le/interoplab/radius/radius_test.go` | AC-1, the container and probe wiring | |

### Boundary Tests (numeric inputs)
| Input | Boundary | Expected |
|-------|----------|----------|
| ready probe | server not yet listening | the probe waits rather than the scenario failing |
| ready probe | server never listens | a named timeout, not a hang |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the three scenarios below | `test/interop-radius/scenarios/` | see the user stories | |

### Interop Tests (Scope: protocol)
| Scenario | Peer implementation | Asserts |
|----------|--------------------|---------|
| `radius-admin-pap-freeradius` | FreeRADIUS, pinned tag | Access-Accept over a real server, Filter-Id to profile |
| `radius-admin-chap-freeradius` | FreeRADIUS with `Cleartext-Password` | The server reproduces ze's CHAP digest |
| `radius-admin-chap-hashed-freeradius` | FreeRADIUS with a hashed entry | Rejected, per RFC 2865 Section 2.2 |

## Files to Modify
- whatever registers interop suites with `./le integration`
- the CI configuration
- `docs/architecture/testing/interop.md` -- the new suite and its scenarios
- `docs/guide/radius.md` -- the Verification table gains the interop rows
- `docs/functional-tests.md`

## Files to Create
- `internal/le/interoplab/radius/` -- lab, containers, checkers, and its test
- `test/interop-radius/scenarios/radius-admin-pap-freeradius/`
- `test/interop-radius/scenarios/radius-admin-chap-freeradius/`
- `test/interop-radius/scenarios/radius-admin-chap-hashed-freeradius/`

### Integration Checklist
- [ ] `./le integration` lists the suite and its scenarios.
- [ ] `Discover` names every scenario directory, so none breaks the run.
- [ ] A CI job runs it.

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/testing/interop.md` -- the suite, its scenarios, its
      peer, and what each asserts.
- [ ] `docs/guide/radius.md` -- the Verification table, which today names two
      `.ci` tests and no interop.
- [ ] `docs/functional-tests.md`.

## Implementation Steps

### Implementation Phases
1. **Phase: The suite skeleton.** Lab, one pulled FreeRADIUS image, a ready
   probe, and one scenario that does nothing but prove the container answers.
2. **Phase: PAP.** AC-2, AC-4, AC-5, AC-6.
3. **Phase: CHAP.** AC-3 and AC-7, the second being the one that proves the
   guide's tradeoff is real.
4. **Phase: Discrimination.** Force each scenario RED and record it.
5. **Phase: CI and docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Not a mock | The peer is a pulled FreeRADIUS image, not ze's own program |
| Both sides | The checker reads the server's record, not only ze's log |
| No fall-through | A green run cannot be a local-bcrypt login |
| No kernel gate | The suite runs where `l2tp_ppp` is absent |
| Pinned | The image tag is exact |

### Deliverables Checklist
| Deliverable | Verification method | Status |
|-------------|--------------------|--------|
| The suite exists and runs | `./le integration` naming it | |
| PAP against a real server | `radius-admin-pap-freeradius` | |
| CHAP against a real server | `radius-admin-chap-freeradius` | |
| The RFC 2865 Section 2.2 consequence | `radius-admin-chap-hashed-freeradius` | |
| Discrimination recorded | the recorded REDs | |
| CI job | the CI config diff | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fixture credentials | The shared secret and passwords are fixture values, marked as such, and never resemble real ones |
| Cleartext storage | The CHAP scenario's `Cleartext-Password` is a fixture and the guide does not read as a recommendation to store real passwords that way |
| Container exposure | The lab's containers bind to the lab network only |

### Failure Routing
| Failure | Route |
|---------|-------|
| The image cannot be pulled | A named failure at lab setup, not a scenario failure |
| The server never becomes ready | The probe's named timeout |
| Ze falls through to local | AC-5 fails, which is the point |

## Design Insights

Every RADIUS proof ze has agrees with ze because ze wrote the other end. That was
tolerable for one credential and stops being tolerable the moment ze computes a
digest or signs a Message-Authenticator, because those are exactly the places
where an implementation can be self-consistently wrong.

## Key Design Decisions

| Decision | Why | What it forecloses |
|----------|-----|--------------------|
| Its own suite | The L2TP suite gates on a kernel module admin login does not need | Sharing the L2TP lab's containers |
| A pulled image | FRR is pulled today, so the machinery exists and the peer is not ze's code | Pinning a FreeRADIUS build ze controls |
| A hashed-password negative scenario | It proves the tradeoff the guide documents rather than asserting it | Nothing |
| Keep the `.ci` mock tests | They are the fast loop, and interop is the slow one | Nothing |

## Known Limitations

- The subscriber RADIUS path keeps its own mock peer. Pointing it at FreeRADIUS
  is a separate piece of work, and it inherits the L2TP suite's kernel gate.

## RFC Documentation (Scope: protocol)
- RFC 2865 Sections 2.2, 5.3, 5.11 and 5.40, all enrolled. This spec adds no
  requirement ids: it strengthens the evidence behind ids that exist.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] The lab machinery was read, so the cost estimate is grounded rather than
      inherited from the retired harness.
- [x] The L2TP suite's kernel gate was read at the producer.
- [x] The owner approved running this spec on 2026-09-04.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Each scenario shown RED against a broken producer, and recorded
- [ ] CI runs the suite
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] `plan/future/spec-radius-admin-interop-freeradius.md` removed
- [ ] Citations repointed
