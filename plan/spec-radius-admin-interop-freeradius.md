# Spec: radius-admin-interop-freeradius

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
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
from its own stored password, and `spec-radius-admin-eap` (closed 2026-09-04)
added a multi-round exchange with a Message-Authenticator ze signs and a State
attribute ze echoes. A mock built beside ze's encoder agrees with ze by construction. A
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
| Lab → FreeRADIUS | a pulled image plus a mounted config directory | [x] |
| Ze → FreeRADIUS | a real Access-Request over UDP | [x] |
| Checker → both sides | ze's log and the server's detail log | [x] |

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
| A-1 | A FreeRADIUS peer can be a pulled image | FRR is pulled in the L2TP suite through `ImageBuild{Pull: true}` | A Dockerfile is needed, which is still small | AC-1 | confirmed -- `docker.io/freeradius/freeradius-server:3.2.7` pulls and answers; the ready probe reads `/proc/net/udp` for 0714 |
| A-2 | The admin path needs no kernel module | it is a UDP socket and ze's own listeners | The suite inherits a gate after all, and the L2TP suite becomes the right home | AC-8 | confirmed -- the whole suite runs green on macOS/Docker Desktop, where `l2tp_ppp` is absent; `TestSuiteNeedsNoKernelModule` refuses a module mount, a capability or a privileged peer |
| A-3 | FreeRADIUS can be made to log enough for the checker to read the server's side | its detail log module | The checker asserts ze's side only, which is weaker and must be said | AC-4 | confirmed -- two `linelog` modules (`test/interop-radius/mods-ze-request-log`) write one line per request carrying verdict, User-Name, the PRESENCE of each credential, and the NAS-Identifier. The checker reads BOTH sides; the fallback did not have to be taken |
| A-4 | The CHAP scenario needs a `Cleartext-Password` entry | RFC 2865 Section 2.2 | The scenario proves less than it claims | AC-3 | confirmed -- the cleartext entry accepts CHAP; the same password stored `{sha256}` refuses CHAP and still accepts PAP, which is what `radius-admin-chap-hashed-freeradius` asserts |

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
| the suite's own lab test, following `internal/le/interoplab/l2tp/l2tp_test.go` | `internal/le/interoplab/radius/radius_test.go` | AC-1, the container and probe wiring | PASS |

### Boundary Tests (numeric inputs)
| Input | Boundary | Expected |
|-------|----------|----------|
| ready probe | server not yet listening | the probe waits rather than the scenario failing |
| ready probe | server never listens | a named timeout, not a hang |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the three scenarios below | `test/interop-radius/scenarios/` | see the user stories | PASS |

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
- [x] `./le integration` lists the suite and its scenarios.
- [x] `Discover` names every scenario directory, so none breaks the run.
- [x] A CI job runs it.

### Documentation Update Checklist (BLOCKING)
- [x] `docs/architecture/testing/interop.md` -- the suites table gains a row, and
      "The FreeRADIUS admin-login suite" states the peer, the linelog record, and
      what each scenario asserts on both sides.
- [x] `docs/guide/radius.md` -- the Verification section gains an interop table,
      and the §2.2 paragraph now cites the scenario that proves it.
- [x] `docs/functional-tests.md`.

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
| The suite exists and runs | `./le integration interop-radius` | done -- re-run whole at closure, 2026-09-04: `integration: 1 action(s) passed.`, exit 0. That summary IS the per-scenario verdict: `Sweep.Text` (`internal/le/leaction/leaction.go`) renders a report only when its row failed, and `Suite.Run` (`internal/le/interoplab/lab.go`) sets `Code = 1` when any scenario fails. Four scenarios now, not the three this spec created: `radius-admin-eap-freeradius` joined the suite from `spec-radius-admin-eap` |
| PAP against a real server | `radius-admin-pap-freeradius` | done |
| CHAP against a real server | `radius-admin-chap-freeradius` | done |
| The RFC 2865 Section 2.2 consequence | `radius-admin-chap-hashed-freeradius` | done |
| Discrimination recorded | the recorded REDs | done -- see Discrimination Record below |
| CI job | the CI config diff | done -- the `radius-interop` job in `.github/workflows/evidence-nightly.yml` |

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
- [x] AC-1..AC-10 demonstrated
- [x] Each scenario shown RED against a broken producer, and recorded
- [x] CI runs the suite
- [ ] `./le verify worktree` passes

### TDD
- [x] Tests written
- [x] Tests FAIL (paste output)
- [x] Tests PASS (paste output)

## Discrimination Record (AC-9)

Baseline, all three scenarios, `./le integration interop-radius`, after a forced
`./le --update` so the binary under measurement is the tree on disk:

```
PASS  3 scenario(s)
```

**Break 1 -- the chain no longer stops at an Access-Reject.** Removed the
`errors.Is(err, ErrAuthRejected)` early return from
`(*ChainAuthenticator).Authenticate` (`internal/component/aaa/types.go`), which
is the R-1 defect this suite exists to catch, then restored it:

```
── radius-admin-chap-freeradius ──
  ✗ FAIL: assertion 5: radiusop logged in with the LOCAL password while RADIUS rejected it; the chain fell through
── radius-admin-chap-hashed-freeradius ──
  ✓ PASS
── radius-admin-pap-freeradius ──
  ✗ FAIL: assertion 5: radiusop logged in with the LOCAL password while RADIUS rejected it; the chain fell through
```

`radius-admin-chap-hashed-freeradius` stays green under this break, and that is
correct rather than a gap: the password it sends is not the local account's, so
a chain that fell through still could not authenticate it. It needs its own
break, below.

**Break 2 -- ze puts the wrong credential on the wire.** Set `auth-method pap`
in `test/interop-radius/scenarios/radius-admin-chap-hashed-freeradius/ze.conf`,
so ze sends a User-Password the server CAN verify against its stored hash, then
restored it.

The break was OBSERVED AGAIN at closure, on 2026-09-04, and the record below is
that second observation rather than the first. The reason is that the first one
went stale without anybody touching this spec: commit `40a08b6a54` added an
`eap-message` field to every line of `test/interop-radius/mods-ze-request-log`
and to `recordWant.String` (`internal/le/interoplab/radius/checkers.go`), so the
want string the checker prints gained a field the recorded evidence did not
carry. The break is the same, the assertion it reds at is the same, and the
message a reader would now see is this one:

```
── radius-admin-chap-hashed-freeradius ──
  ✗ FAIL: assertion 1: wait for FreeRADIUS record 'verdict=silent user=localop
    user-password=absent chap-password=present eap-message=absent
    nas-identifier=ze-interop-nas' timed out before the peer became ready
```

The scenario reds at its FIRST assertion rather than at the login under test,
because the control request already carries the wrong credential and the checker
reads the server's record of what ARRIVED. That is the discrimination working
earlier than expected, not a weaker one: no assertion in this scenario survives
ze sending PAP where the config says `chap`.

**What closure re-checked, and what it did not.** Break 2 was re-observed, above.
Break 1 was NOT re-run, and it did not need to be: its producing branch is still
`errors.Is(err, ErrAuthRejected)` in `(*ChainAuthenticator).Authenticate`
(`internal/component/aaa/types.go`), the failure text it recorded is still the
one `assertRejectStopsChain` prints verbatim, and the assertion number 5 is
still the one `checkAcceptedLogin` passes. The same break is also re-proven in
process, with no container, by `TestPAPCheckerPolarities` /
`TestEAPCheckerPolarities` "a chain that falls through to local after
Access-Reject fails", which ran green at closure. Re-running it on the wire
would have meant editing a shared product file that peer sessions read, for a
red whose every component was verifiable by reading.

**The fourth scenario's REDs are recorded in another spec.** This spec created
three, and `radius-admin-eap-freeradius` arrived with commit `40a08b6a54` from
`spec-radius-admin-eap`. Its two forced REDs, an `AttrState` append removed from
`eapCredential` and the same wrong-credential break, are in that spec's own
Discrimination Record, preserved at that commit.

### Closure
- [ ] `plan/future/spec-radius-admin-interop-freeradius.md` removed
- [ ] Citations repointed

---

## Implementation Summary

### What Was Implemented

`internal/le/interoplab/radius/`, a fifth interop suite beside `bgp`, `ipsec`,
`l2tp` and `pppoe`, running ze's operator login against
`docker.io/freeradius/freeradius-server:3.2.7` at an exact tag.

- `radius.go` -- the suite: `suiteFor` declares one pulled server image and one
  built ze image, a `172.27.0.0/24` network nobody else holds, a
  `/proc/net/udp` readiness probe on port 0714 for the server and an `ss`
  probe on 2222 for ze. Its only preflight is `buildZe`, the ze cross-compile.
- `checkers.go` -- four checkers. `checkAcceptedLogin` is the shared walk PAP
  and CHAP both take, `checkCHAPHashed` asserts a REFUSAL and stays separate,
  and `checkEAP` adds the intermediate-round assertion.
- `radius_test.go` -- `labPolicy`, a model of the PRODUCT chain, and the
  polarity tables that break one link at a time with no container.
- `test/interop-radius/` -- the lab-wide FreeRADIUS configuration, and one
  directory per scenario holding `users` and `ze.conf`.
- `internal/le/integration/gates.go` and `actions.go` -- the `interop-radius`
  verb and its `RADIUS_INTEROP_SCENARIO` selector.
- `internal/le/rfc/carriers.go` -- the suite's tree, so its RFC evidence tier
  can resolve from the scheduled workflow.
- `.github/workflows/evidence-nightly.yml` -- the `radius-interop` job.

### Bugs Found/Fixed

- `parseStubLogin` (`radius_test.go`) returned three same-typed results in the
  declared order and built them in another, so every stub login arrived as an
  unknown user and all twelve polarity subtests failed at their first assertion
  with one identical message. `TestParseStubLoginReadsEachFieldInOrder` now
  pins the positions.
- `checkCHAPHashed` read one exit status as evidence of one cause, naming
  FreeRADIUS when ze's chain was at fault. `explainUnexpectedLogin` reads ze's
  log and names the side.
- `buildZe` took its environment from `os.Environ`, which resolves GOCACHE to
  the machine default that nothing here manages. It now takes it from
  `gotoolchain`. Recorded with the ipsec lab's identical line in
  `plan/journal/full-disk-false-red.md`.
- Found at closure: the comment on `assertNoAuthSourceLocal` said its single
  log read was safe because "the failure line above proved the log is being
  written". No such assertion runs above it. `assertRejectStopsChain` calls
  `assertAuthFailureRecorded` AFTER it, and that is what makes the single read
  safe. The comment now says so.

### Documentation Updates

- `docs/architecture/testing/interop.md` -- the suites table row, and "The
  FreeRADIUS admin-login suite" naming the peer, the linelog record and what
  each scenario asserts on both sides. It carries three `<!-- source: -->`
  anchors, into `radius.go`, `checkers.go` and `mods-ze-request-log`.
  Committed by the EAP session, not by `2bea012491`: at that moment the file
  held a peer session's uncommitted IPsec work.
- `docs/guide/radius.md` -- the Verification section's interop table, four
  rows, and the RFC 2865 §2.2 paragraph now citing
  `radius-admin-chap-hashed-freeradius`. It carries anchors into `checkers.go`
  and `test/interop-radius/scenarios/`. Committed in `40a08b6a54`.
- `docs/functional-tests.md` -- the `RADIUS interop` row. Committed in
  `2bea012491`.

### Deviations from Plan

- The spec planned three scenarios and the suite carries four. The fourth,
  `radius-admin-eap-freeradius`, was added by `spec-radius-admin-eap` in
  `40a08b6a54` and carries its own discrimination record there. Nothing this
  spec built was reduced.
- The two documentation files this spec owed were committed by a LATER commit
  rather than by `2bea012491`, because at that moment each held another
  session's uncommitted work in the same file. Neither was deferred past the
  work: both are committed and verified below.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Discrimination Record pasted the checker's want string as evidence, and that string is DERIVED from `recordWant.String`. A later spec added an `eap-message` field to it | The recorded evidence stopped matching what the tree prints, with nothing edited in this spec and no gate able to see it | Closure compared the pasted line against `recordWant.String` in the tree | Break 2 re-observed at closure and the record replaced. Journal row in `plan/journal/claim-outlives-the-evidence-it-cites.md` |
| approach | `buildZe` was written with `append(os.Environ(), ...)`, copying the ipsec lab | GOCACHE then resolves outside everything this repository manages, and a concurrent trim fails the cross-compile as a broken toolchain | Two cross-compile failures in ten minutes, at 96% disk | `gotoolchain.Environment`, and a row in `plan/journal/full-disk-false-red.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A RADIUS proof against a server ze did not write | Done | `internal/le/interoplab/radius/radius.go`, `serverImage` | `docker.io/freeradius/freeradius-server:3.2.7`, pulled, not built from ze code |
| Its own suite, not an extension of the L2TP one | Done | `internal/le/interoplab/radius/`, `suiteFor` | `Preflight` runs `buildZe` only; the L2TP module probe is not inherited |
| The checker reads the SERVER's side | Done | `checkers.go`, `assertServerRecord` | reads `/var/log/freeradius/ze-request.log` inside the FreeRADIUS container |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `serverPeerConfig`, its `ReadyProbe` on `/proc/net/udp` | a probe, not a sleep; `TestSuiteNeedsNoKernelModule` refuses a peer with no probe |
| AC-2 | Done | `checkPAP` -> `checkAcceptedLogin`, assertion 2 | `assertAuthSource(radiusUser, "radius")` over ze's real SSH listener |
| AC-3 | Done | `checkCHAP`, and `radius-admin-chap-freeradius/users` holding `Cleartext-Password` | the server reproduces ze's digest from its own stored password |
| AC-4 | Done | `assertServerRecord`, and `wantRecord` comparing all six fields | the polarity `recordRequests: false` fails, so ze's log alone never satisfies a scenario |
| AC-5 | Done | `assertRejectStopsChain` | it sends the LOCAL account's real password, so a fall-through would succeed |
| AC-6 | Done | `assertFilterIDProfile` | `show bgp` refused with the authorization message for `radiusop`, allowed for the admin-profile control user |
| AC-7 | Done | `checkCHAPHashed`, assertions 2 and 3 | the radclient PAP probe proves the entry first, then CHAP must be refused |
| AC-8 | Done | `TestSuiteNeedsNoKernelModule` | no `/lib/modules` mount, no `Capabilities`, no `--privileged`; `PeerConfig` has no other privilege field |
| AC-9 | Done | Discrimination Record above | break 2 re-observed at closure; break 1 verified at its producer and re-proven in process |
| AC-10 | Done | `.github/workflows/evidence-nightly.yml`, job `radius-interop` | `./le integration interop-radius` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the suite's own lab test | Done | `internal/le/interoplab/radius/radius_test.go` | `ok ... 9.472s` at closure |
| the three scenarios | Done | `test/interop-radius/scenarios/` | whole suite green at closure, exit 0 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/interoplab/radius/` | Done | three files |
| `test/interop-radius/scenarios/radius-admin-pap-freeradius/` | Done | `users`, `ze.conf` |
| `test/interop-radius/scenarios/radius-admin-chap-freeradius/` | Done | `users`, `ze.conf` |
| `test/interop-radius/scenarios/radius-admin-chap-hashed-freeradius/` | Done | `users`, `ze.conf` |
| whatever registers interop suites | Done | `internal/le/integration/gates.go`, `actions.go` |
| the CI configuration | Done | `.github/workflows/evidence-nightly.yml` |
| `docs/architecture/testing/interop.md` | Done | committed by the EAP session |
| `docs/guide/radius.md` | Done | `40a08b6a54` |
| `docs/functional-tests.md` | Done | `2bea012491` |

### Audit Summary
- **Total items:** 24
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the suite carries a fourth scenario a later spec added; recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A server ze did not write can DISAGREE with ze about a CHAP digest | interop | `radius-admin-chap-freeradius` green against `freeradius-server:3.2.7`, and the server's own record shows `verdict=accept ... chap-password=present`. The break that puts a PAP credential on the wire under an `auth-method chap` config reds it, recorded above |
| A green run cannot be a local-bcrypt login | interop + unit polarity | `assertServerRecord` requires FreeRADIUS's own line for the request; `TestPAPCheckerPolarities` "a login with no server record fails, whatever ze logged" is red with `recordRequests: false` |
| The guide's `auth-method chap` tradeoff is real, not asserted | interop | `radius-admin-chap-hashed-freeradius`: the radclient probe gets `Received Access-Accept` for the same entry and the same password over PAP, then the CHAP login is refused and ze authenticates the operator through no backend |
| The suite runs where `l2tp_ppp` is absent | functional | the whole suite ran green at closure on macOS/Docker Desktop, where the module does not exist; `TestSuiteNeedsNoKernelModule` refuses a module mount, a capability or a privileged peer |
| The suite is not written and then never run | CI | the `radius-interop` job, and `interopCarriers` (`internal/le/rfc/carriers.go`) now resolves this tree's tier from that scheduled workflow |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none -- this spec declares `Deferral shard: -` and opened no row | cancelled | `plan/deferrals/` holds no shard for this stem |

`plan/deferrals/radius-chap-eap-admin.md` is NOT this spec's shard and is left
alone. Its one live row covers "CHAP/EAP admin authentication, and admin-session
accounting"; the first half is now built and proven, the second is not, so the
row stays live and is not this closure's to terminate.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/radius-admin-interop-freeradius-f89390ec-889f-4a7a-8172-1e2cfd108a12.md` (5 files, verdict=clean) |
| `./le spec session review check` | `review_gate: OK (3 code files, clean, hashes match ...)` |
| Rounds | 1 |
| Reviewer lenses used | fail-closed / zero-value guards, vacuity (would this pass against a stub), both-sides evidence, RFC 2865 §2.2 and §4.1 claim-versus-assertion, secrets and container exposure, `docs/contributing/ze-go-style.md` control flow and panic reachability, stale comments |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The Discrimination Record's break 2 evidence no longer matched what the tree prints: `recordWant.String` gained an `eap-message` field after the record was written, so the pasted want string was a claim about code that no longer exists | this spec, Discrimination Record | break 2 re-observed on 2026-09-04 and the record replaced with the observed output |
| 2 | ISSUE | The Deliverables row claimed "3 scenarios green"; the suite carries four | this spec, Deliverables Checklist | row replaced with the closure run's own result and the reason there are four |
| 3 | NOTE | `assertNoAuthSourceLocal`'s comment justified its single log read by "the failure line above", and that assertion runs after it, not before | `internal/le/interoplab/radius/checkers.go` | comment rewritten to name the assertion that FOLLOWS, which is what makes the read safe |
| 4 | NOTE | `parseServerRecord` and `recordWant.matches` each hold a six-term compound condition, which `docs/contributing/ze-go-style.md` asks be split | `checkers.go` | not changed. The six terms are one fact repeated over a homogeneous field set rather than three different facts a reader must hold at once, and both functions are covered by `TestParseServerRecord`. Recorded, not fixed |

No BLOCKER. No `panic()` anywhere in the suite, so the style pass's first
question has no site to answer about.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/radius/` | Yes | `checkers.go  radius.go  radius_test.go` |
| `test/interop-radius/scenarios/` | Yes | `radius-admin-chap-freeradius/  radius-admin-chap-hashed-freeradius/  radius-admin-eap-freeradius/  radius-admin-pap-freeradius/` |
| the lab-wide server configuration | Yes | `clients.conf  Dockerfile.ze  mods-ze-request-log  site-default` |
| the CI job | Yes | `grep -n radius-interop .github/workflows/evidence-nightly.yml` answers the `radius-interop:` job header |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a probe, not a sleep | `serverPeerConfig` declares `ReadyProbe{Command: {"sh","-c","grep -q ':0714 ' /proc/net/udp"}, Timeout: 90s, Interval: 1s}` |
| AC-2, AC-3 | a real login over SSH, `source=radius` | whole suite green at closure, exit 0 |
| AC-4 | the server's own record | `assertServerRecord` queries `cat /var/log/freeradius/ze-request.log` on `serverPeer`; the line comes from the `linelog` modules in `test/interop-radius/mods-ze-request-log`, called from `post-auth` in `site-default` |
| AC-5 | the chain stops | `go test ./internal/le/interoplab/radius/` green, including "a chain that falls through to local after Access-Reject fails" |
| AC-6 | Filter-Id decides authorization | `interop-operator` in each `ze.conf` denies `show bgp`; `assertFilterIDProfile` requires the refusal AND requires the same command to work for the admin-profile user |
| AC-7 | RFC 2865 §2.2's consequence | the hashed `users` entry is `{sha256}` over `fixture-chap-secret`, confirmed by recomputing the digest: `ar0ZJw9ixbA6LpsWQYWzA/4cmajW11xulQsp2JTJaZM=` |
| AC-8 | no kernel module | `TestSuiteNeedsNoKernelModule` green, and the suite ran green on a host with no `l2tp_ppp` |
| AC-9 | recorded RED per scenario | Discrimination Record above, break 2 re-observed at closure |
| AC-10 | CI names the suite | the `radius-interop` job |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le integration interop-radius` | not a `.ci`: the scenario `radius-admin-pap-freeradius` | Yes -- `runRADIUSInterop` (`internal/le/integration/actions.go`) calls `interopradius.RunAt`, `Discover` reads `test/interop-radius/scenarios/`, and the checker drives `ze cli` over ze's own SSH listener inside the container |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `ImageBuild{Tag: serverImage, Pull: true}`; the image is present and answered every scenario |
| A-2 | confirmed | `TestSuiteNeedsNoKernelModule`, and the closure run itself on a host without the module |
| A-3 | confirmed | three `linelog` modules write the record; the fallback was never taken |
| A-4 | confirmed | the cleartext entry accepts CHAP, the `{sha256}` entry refuses it and still accepts PAP |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/testing/interop.md` suites table and suite section | its four scenario rows match `scenarioCheckerMap`, and the §2.2 quote matches `rfc/full/rfc2865.txt` | Yes |
| `docs/guide/radius.md` Verification interop table | its four rows match the four checkers | Yes |
| `docs/functional-tests.md` | the `RADIUS interop` row names `RADIUS_INTEROP_SCENARIO` and `./le integration interop-radius`, which `gates.go` serves | Yes |
| feature list, config syntax, CLI reference, API/RPC, plugin SDK, wire format, comparison table | this spec changes no product code and adds no config leaf, command or wire behavior: `git show --stat 2bea012491` touches no `internal/component/` file | Yes, no update owed |
| doctor checks | the suite adds no runtime dependency to the DAEMON: Docker and the FreeRADIUS image are dependencies of a development gate, not of `ze` | Yes, none owed |
| RFC status | no requirement id is added or re-levelled; the suite strengthens evidence behind ids that exist. `internal/le/rfc/carriers.go` registers the tree so its tier can resolve | Yes |

### What was NOT verified at closure, and why
- `./le verify worktree` and `./le verify current mode full` were NOT run.
  `./le verify status check` answers `STALE: no status file (never verified)`,
  and the shared checkout holds several peer sessions' uncommitted work,
  including an `internal/component/bgp/plugins/redistribute_ingress` package
  that currently holds only a test file and therefore does not build. A
  whole-tree run over that tree measures their work, not this spec's. The debt
  row is written by `./le commit create` into
  `plan/verification-debt/5db0ba0d.md`.
- `./le repository check` was run and answers 3 issues, all outside this spec:
  a `docs/guide/configuration.md` anchor into the `redistribute_ingress`
  package another session is retiring, and two exported symbols
  (`ApplyNegotiationConfig`, `AllRPCDocs`) in files those sessions hold
  uncommitted.
- `./le spec citation` was run and is green: 208 specs, 51 baselined dangling,
  10 line-token warnings, none of them this spec.
- `./le doc check verify` was run and exits 1. Not one of its findings names
  RADIUS, this suite or any file this spec touches: `grep -i radius` over the
  whole log returns nothing under a failing line. The findings are CLI catalog
  drift (`show errors`, `create interface address`, `clear vrrp statistics`,
  `explain`) that peer sessions hold uncommitted. Its digest-anchor stage is
  green: 3025 anchors across 23 digests all resolve.
- No `./le spec citation` repoint was owed. `speccitation.Scan` reads only
  `plan/spec-*.md`, and no other spec names this one. The three references that
  do exist are outside that population and stay as records: a row in
  `plan/journal/guard-enumerates-instead-of-subtracting.md`, a section of
  `plan/handover/2026-09-04-radius-and-two-fixits.md`, and five entries in
  `internal/le/doc/check/testdata/doc_citation_baseline.txt` that name paths
  the retired skeleton proposed. Those five become stale baseline WARNINGS when
  the spec goes, never errors: `checkLinks` (`internal/le/doc/check/links.go`)
  puts them in `Warnings`, and `plan/spec-` is in `citationExcludePrefixes`.

## Core Insight

An interop record's evidence can be invalidated by a spec that never touches the
record. The want string a checker prints is DERIVED from the code, so pasting it
into a spec creates a claim about a producer the spec does not own. Here a later
phase of a sibling spec added one field to `recordWant.String`, and the
Discrimination Record silently stopped describing the tree. Nothing could see
it: no gate reads a fenced block, and the spec that owned the record had not
changed. A recorded RED is a fact about a moment, and the closure that reads it
is the only thing standing between that fact and a false claim.
