# Spec: fixit-child-sa-rekey-policy

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Every Child SA rekey fails on Linux, and the tunnel stops carrying traffic when
the retired SA hard-expires.**

Ze installs an XFRM policy per Child SA. A rekey is make-before-break, so it
installs the replacement pair while the retired one is still installed. The
replacement's policies are identical to the retired pair's in every field the
kernel reads, because `newRekeyedChild` inherits `TSLocal`, `TSRemote`, `IfID`,
`ReqID`, `Mode` and `Selectors`. The kernel refuses the second install.

The goal is that a Child SA rekey installs new STATES and leaves the policy
alone, and that retiring the old pair removes its states without removing the
policy the live pair depends on.

### The measurement

`TestXFRMSecondInstallOfOneSelectorIsRefused`
(`internal/component/ike/dataplane/xfrm_rekey_policy_integration_linux_test.go`)
runs in the QEMU Alpine VM against a real kernel:

    second install refused, as expected: xfrm: policy add: file exists
    --- PASS

Its second half also proves only ONE policy exists: a second `RemovePolicyParams`
for the same selector fails.

### The chain, measured or read

| Link | How it is known |
|------|-----------------|
| A second policy with one selector is refused `EEXIST`, and only one exists | measured, QEMU, real kernel |
| `netlink.XfrmPolicyAdd` sends `XFRM_MSG_NEWPOLICY`, the exclusive form | read, `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` |
| `newRekeyedChild` inherits every field the policy selector is built from | read, `engine/rekey.go` |
| `installChildSA` installs both policies unconditionally, with no guard | read, `engine/child.go` |
| `isXFRMUnsupported` matches `ENOPROTOOPT`, `EPROTONOSUPPORT`, `EAFNOSUPPORT`, `ENOSYS` only | read, `engine/child.go` |
| `installChildTolerant` returns any error it cannot classify as unsupported | read, `engine/child.go` |

So the replacement's inbound policy install returns `EEXIST`, and
`installChildSA`'s own rollback then calls `RemoveSA` on both freshly installed
states. The rekey exchange fails after the peer has already keyed its side.

### Why the model is wrong, not the check

The policy is treated as owned by a Child SA. The kernel keys a policy on the
selector pair and resolves it to a state through `ReqID` (RFC 4301 Section
4.4.1.2). A replacement pair with the same `ReqID` is picked up by the policy
that is already installed, so the policy never needs reinstalling. It belongs to
the SESSION, and it must go when the session goes, not when one pair of states
is retired.

That also makes today's teardown wrong in the other direction:
`removeChildSAOutgoing` calls `RemovePolicy` with the selector the LIVE pair
still needs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ipsec/` (whichever pages cover Child SA programming)
  → Constraint: the dataplane vocabulary is `dataplane.SPParams` and
    `dataplane.SAParams`; the engine never speaks netlink directly.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - Section 2.8 rekey, Section 1.3.2 CREATE_CHILD_SA
  → Constraint: make-before-break. The replacement must carry traffic before the
    retired pair is deleted, so the policy must never be absent between the two.
- [ ] `rfc/short/rfc4301.md` if present, else `rfc/full/rfc4301.txt` Section 4.4.1.2
  → Constraint: the policy template resolves to a state through the endpoints and
    the request id. That is why one policy serves successive states.

**Key insights:**
- One policy, many successive states. `ReqID` is the join.
- The engine already carries `ReqID` on `ChildSA` and inherits it at rekey, so the
  join is present and correct today. Only the install and remove calls are wrong.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/child.go` - `installChildSA` installs two
  states then two policies, unconditionally. `removeChildSA` removes both
  policies and both states. `installChildTolerant` swallows only "unsupported".
- [ ] `internal/component/ike/engine/rekey.go` - `newRekeyedChild` inherits the
  selector fields. `respondChildRekey` and `applyChildRekeyResponse` both call
  `installChildTolerant` while the retired pair is still installed.
- [ ] `internal/component/ike/engine/delete.go` - `closeDesignatedChildSAs` calls
  `removeChildSA` on the superseded pair when the peer confirms a rekey.
- [ ] `internal/component/ike/engine/established.go` - `cleanupChild` removes both
  the superseded and the live pair at session teardown.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `InstallPolicy` calls
  `XfrmPolicyAdd`; `RemovePolicy` and `RemovePolicyParams` call `XfrmPolicyDel`.

**Behavior to preserve:**
- The first Child SA of a session still installs both policies and both states.
- A session teardown still leaves no XFRM state or policy behind.
- Transport mode still installs a policy with no tunnel endpoints.
- `installChildTolerant` still degrades gracefully where XFRM is absent.

**Behavior to change:**
- A rekey installs states only.
- Retiring a superseded pair removes states only.
- The policy pair is installed once per session and removed once, at teardown.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A CREATE_CHILD_SA rekey arrives (peer-initiated) or fires on ze's own timer.
- Format at entry: an SK-encrypted CREATE_CHILD_SA carrying SA, Ni/Nr, TSi, TSr.

### Transformation Path
1. `respondChildRekey` or `applyChildRekeyResponse` builds the replacement pair
   through `newRekeyedChild`, inheriting the selector fields.
2. `installChildTolerant` calls `installChildSA`, which installs two states and
   then, today, two policies.
3. The peer's Delete for the retired pair reaches `closeDesignatedChildSAs`,
   which calls `removeChildSA` and, today, removes the policies.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ dataplane | `dataplane.Dataplane` interface: `InstallSA`, `RemoveSA`, `InstallPolicy`, `RemovePolicy`, `RemovePolicyParams` | No |
| Dataplane ↔ kernel | netlink XFRM: `XfrmStateAdd`, `XfrmPolicyAdd`, `XfrmPolicyDel` | Yes, measured in QEMU |

### Integration Points
- `ChildSA` (`engine/child.go`) carries the selector fields the policy is built
  from. It is the natural place to record whether this pair owns the policy.
- `PeerSession` (`engine/register.go`) outlives every Child SA of the session,
  so it is the natural owner of the policy lifecycle.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire buffers on this path |
| Registration over hardcoding | N-A | no new command, view, family or handler |

## Key Design Decisions

- **The policy belongs to the session, not to a Child SA.** The alternative,
  reinstalling with `XfrmPolicyUpdate` on every rekey, keeps the wrong ownership
  and leaves the teardown race in place: the retiring pair would still delete a
  policy the live pair needs.
- **`ReqID` is already the join and does not change.** No new field is needed on
  the wire or in config.
- **Fail closed on a policy install that genuinely fails.** Today an `EEXIST` is
  indistinguishable from a real failure. After this change a duplicate install is
  not attempted at all, so any error from `InstallPolicy` is a real one.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every IPsec tunnel. A policy installed too late leaves traffic unprotected; a policy removed too early drops it. |
| How is it reverted? | Single commit revert. No config migration, nothing a peer observes. |
| Who else touches this path? | `spec-fixit-vpp-ipsec-inoperable` (the VPP backend refuses every policy IKE produces, so it is unaffected until `plan/future/spec-ipsec-vpp-policy-interface.md` lands), `plan/spec-ipsec-opaque-selector-port-mask.md` (the same policy selector). |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel resolves an installed policy to a NEW state with the same `ReqID`, so a rekey needs no policy touch | RFC 4301 Section 4.4.1.2; `xfrmPolicyFromParams` writes the template `Reqid` | The replacement carries no traffic and the fix is worse than the defect | A QEMU test that installs a policy, installs state A, replaces it with state B at the same `ReqID`, and asserts the policy still resolves | unvalidated |
| A-2 | Only `createFirstChildSA` and the rekey paths install a Child SA | grep of `installChildSA` and `installChildTolerant` call sites | A third path installs no policy and the tunnel never comes up | grep at implementation time | unvalidated |
| A-3 | Selectors never change across a rekey | `newRekeyedChild` inherits `TSLocal`/`TSRemote`; RFC 7296 Section 2.9.2 forbids narrowing below the scope in use | A rekey that legitimately renegotiates selectors needs the policy replaced, so the session-owned policy is stale | Read `respondChildRekey`'s `narrowChildSelectors` and decide whether it can widen or shift the pair | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A-3 is broken: a rekey CAN change the selectors, so a session-owned policy would be wrong after one | `narrowChildSelectors` returns a pair different from `old.Selectors` | Compare the replacement's selectors against the installed policy's, and replace the policy only when they differ |
| R-2 | Session teardown paths are more numerous than `cleanupChild`, so a policy leaks | An XFRM policy survives a `clear` in the QEMU test | Enumerate every teardown path in the same pass, as the `forgetKeys` work did for keys |
| R-3 | The transport-mode policy has a different shape, so one lifecycle does not fit both | `xfrm_transport_integration_linux_test.go` reddens | Keep the mode on the session-owned record, exactly as `ChildSA` carries it now |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A CREATE_CHILD_SA rekey the peer initiates | → | `respondChildRekey` → `installChildTolerant` | `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` |
| A CREATE_CHILD_SA rekey ze initiates | → | `applyChildRekeyResponse` → `installChildTolerant` | `TestInitiatedChildRekeyInstallsStatesWithoutTouchingThePolicy` |
| The peer's Delete confirming a rekey | → | `closeDesignatedChildSAs` → `removeChildSA` | `TestRetiringASupersededPairKeepsTheLivePolicy` |
| Session teardown | → | `cleanupChild` | `TestSessionTeardownRemovesThePolicyExactlyOnce` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Child SA rekey completes while the retired pair is installed | The replacement's states are installed and no `InstallPolicy` call is made |
| AC-2 | A Child SA rekey completes | The rekey does not fail, and no state is rolled back |
| AC-3 | The peer's Delete retires the superseded pair | The retired pair's states are removed and the policy remains installed |
| AC-4 | The session ends with a live pair and a superseded pair | Every state and both policies are removed, and no second delete is attempted |
| AC-5 | A real Linux kernel, a policy installed, a state replaced at the same `ReqID` | The policy still resolves to the new state (A-1) |
| AC-6 | A policy install fails for a reason other than a duplicate | The Child SA install fails and says why, exactly as today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a site-to-site tunnel past its Child SA lifetime | timer → `initiateChildRekey` → `applyChildRekeyResponse` → states installed → peer Delete → old states removed | `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` plus the QEMU rekey test |
| 2 | Runs a tunnel against strongSwan past the rekey interval | the IPsec interop lab with a short Child SA lifetime | the interop scenario named below |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` | `internal/component/ike/engine/rekey_policy_test.go` | AC-1, AC-2 through a fake dataplane that records every call | |
| `TestRetiringASupersededPairKeepsTheLivePolicy` | `internal/component/ike/engine/rekey_policy_test.go` | AC-3 | |
| `TestSessionTeardownRemovesThePolicyExactlyOnce` | `internal/component/ike/engine/rekey_policy_test.go` | AC-4 | |
| `TestPolicyInstallFailureStillFailsTheChildSA` | `internal/component/ike/engine/rekey_policy_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no new numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-child-rekey-xfrm` | `test/ipsec/ipsec-child-rekey-xfrm.ci` | AC-1 to AC-4 through the daemon: two ze instances rekey a Child SA with the REAL xfrm backend, and the tunnel still has a policy afterwards. `option=needs-linux:caps=net-admin`, so it runs under `make ze-qemu-needs-linux-test` | |
| `TestXFRMPolicyResolvesToAReplacedState` | `internal/component/ike/dataplane/xfrm_rekey_policy_integration_linux_test.go` | AC-5, against a real kernel in QEMU | |
| `TestXFRMSecondInstallOfOneSelectorIsRefused` | same file | the measurement this spec rests on | done |

**Why the existing `.ci` is blind to this.** `test/ipsec/ipsec-child-rekey.ci`
already drives a full Child SA rekey between two ze instances, and it passes. It
sets `option=env:var=ze_test_ike_dataplane:value=noop`, so `installChildTolerant`
returns before any XFRM call. The test proves the protocol exchange and can never
see the dataplane collision. The new `.ci` above is that test with the real
backend and the capability marker the backend needs.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| a short-lifetime rekey scenario | `test/ipsec-interop/` | strongSwan | a tunnel survives a Child SA rekey and keeps carrying traffic | |

## Files to Modify
- `internal/component/ike/engine/child.go` - split the policy install and remove
  out of `installChildSA` and `removeChildSA`.
- `internal/component/ike/engine/rekey.go` - the two rekey paths install states only.
- `internal/component/ike/engine/delete.go` - retiring a superseded pair removes
  states only.
- `internal/component/ike/engine/established.go` - `cleanupChild` owns the policy
  removal, once.
- `internal/component/ike/engine/register.go` - `PeerSession` records the installed
  policy pair.

## Files to Create
- `internal/component/ike/engine/rekey_policy_test.go` - the four unit tests above.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no operator-visible setting changes |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | No | no new runtime dependency; XFRM is already checked |
| Prometheus counters/metrics | No | the failure this removes was silent, and the fix makes it not happen; a counter would count zero |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| Doc | Update? | File / reason |
|-----|---------|---------------|
| Feature list | No | no new feature; a defect is removed |
| User guide | No | no operator-visible change |
| RFC compliance | Yes | `docs/features/rfc-status.md` RFC 7296 row: Section 2.8 rekey now survives on the XFRM backend |
| Architecture | Yes | the Child SA programming page: one policy, successive states |

## Implementation Steps

1. Validate A-1 first, in QEMU. If the kernel does NOT resolve a replaced state
   through the existing policy, this design is wrong and the spec stops here.
2. Validate A-3 by reading `narrowChildSelectors`. If a rekey can change the
   selectors, add the compare-and-replace path R-1 names before going further.
3. Add the policy record to `PeerSession` and the four unit tests, red first.
4. Split the policy calls out of `installChildSA` and `removeChildSA`.
5. Point the two rekey paths and the retire path at the states-only variants.
6. Enumerate every session teardown path, as the `forgetKeys` work did.
7. Run the QEMU integration suite and the strongSwan interop lab.

## Goal Gates

- `make ze-verify` passes.
- `make ze-qemu-integration-test` passes, including the two tests above.
- The strongSwan interop scenario survives a Child SA rekey.

## Quality Gates

- Every AC has a named test whose assertion states the AC's observable behavior.
- Every test is mutation-verified: disabling the production change reddens it.

## RFC Documentation (Scope: protocol)

RFC 7296 Section 2.8 is already enrolled. This spec adds no requirement row: it
makes an implemented obligation actually work. If `make ze-rfc-check` shows a
Section 2.8 row whose evidence is unit-only, this is the work that earns it a
functional or interop tier.

## Known Limitations

The VPP IPsec backend cannot be driven by IKE: it installs SAs and refuses every policy
the engine produces (`spec-fixit-vpp-ipsec-inoperable`, closed 2026-08-10), so this fix is
proven on XFRM only.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] A-1 validated in QEMU before any production edit
- [ ] A-3 validated by reading `narrowChildSelectors`
- [ ] Every teardown path enumerated
- [ ] `make ze-verify` green
- [ ] QEMU integration green
- [ ] Interop scenario green

## Audit 2026-08-02: implemented by a DIFFERENT design. NOT ready to close

Read against the code on 2026-08-02, during the closure of
the rfcgate-1b RFC 7296 pilot spec. This section is a bookkeeping record. It changes
no code and closes nothing.

**The tunnel-level defect is fixed and interop-proven. The spec text is counterfactual.**
Commit `1963345b4` added this spec file and the fix in one commit, and the fix took the
alternative this spec's Key Design Decisions section explicitly REJECTS. There is no
`PeerSession`-owned policy record. What shipped is:

- `xfrmBackend.InstallPolicy` upserts, at `internal/component/ike/dataplane/xfrm_linux.go`
  (`netlink.XfrmPolicyUpdate`). No `XfrmPolicyAdd` remains in production code (verified
  2026-08-02 by a tree-wide grep over `internal/`, `cmd/` and `pkg/`).
- Removal is guarded by a shared-selector comparison: `samePolicySelector`
  (`internal/component/ike/engine/child.go`) feeding `dropPolicy` in
  `removeChildSAExcept` (`child.go`).

**AC verdicts.**

| AC | Verdict | Evidence |
|----|---------|----------|
| AC-1 | NOT landed as written | `installChildSA` still calls `dp.InstallPolicy` unconditionally at `child.go` and `child.go`, on every install including both rekey paths. The AC says "no `InstallPolicy` call is made", which is false today. The HARM it proxied for is gone, cured one layer down by the upsert |
| AC-2 | Landed, by a different mechanism | The duplicate error cannot occur, because UPDPOLICY replaces. `TestXFRMPolicyInstallIsIdempotent` |
| AC-3 | Landed | `removeChildSAExcept`, retired from `closeDesignatedChildSAs`. `TestRetiredChildKeepsThePolicyTheReplacementUses` |
| AC-4 | Partially landed | First clause holds. The second ("no second delete is attempted") is false BY DESIGN: the peer-Delete-then-teardown path repeats `removeChildSA` on the live pair, and the code comment concedes it as harmless |
| AC-5 | NOT landed | `TestXFRMPolicyResolvesToAReplacedState` does not exist. Neither QEMU test installs an XFRM state at all, so neither exercises reqid resolution. **A-1 is still `unvalidated` and still load-bearing**, because the upsert design has one policy serve successive states |
| AC-6 | Landed, behavior unchanged | The install-failure paths still fail closed. No test named for it exists |

**Residual work.** Each line is concrete enough to implement from.

- Rewrite Key Design Decisions and the ACs to state the design that SHIPPED: one upserted
  policy per selector plus a shared-selector guard on removal. Not a `PeerSession`-owned
  policy record.
- Restate AC-1: `InstallPolicy` IS called on rekey and MUST be idempotent.
- Restate AC-4's second clause, or stop the live pair's teardown being repeated.
- Write the AC-5 QEMU test: install a policy, install a state at one request id, replace it
  with a second state at the same request id, assert the policy still resolves. This is what
  validates A-1.
- Write the AC-6 test proving a non-unsupported policy-install error still fails the Child SA
  install and rolls back both states.
- Add a teardown test for the two-child "exactly once" case. The existing test covers only
  the single-child case.
- Audit `reapStalePending` and `cleanupPendingSA` (`engine/established.go`): both tear down a
  pending child with no `keep`, so a pending child sharing the live child's selector and
  interface id can strip the LIVE pair's policy. This is the bug class AC-3 fixes, on a path
  the fix did not reach. **Read, not measured. Unverified.**
- Audit the three responder rollback sites for the same hazard.
- Add a `.ci` exercising the real dataplane. `test/ipsec/ipsec-child-rekey.ci` still pins the
  no-op dataplane and is blind to this whole class, exactly as this spec's own "Why the
  existing `.ci` is blind" paragraph says.
- Reconcile Files to Modify with reality: `dataplane/xfrm_linux.go` was changed and is not
  listed.

Tracked by `plan/deferrals/rfcgate-1b-rfc7296-pilot.md`, which names this spec as the
destination. The spec stays OPEN.

## Audit 2026-08-02, second pass: three more policy-identity residuals

Found while closing the rfcgate-1b RFC 7296 pilot spec, after the audit above was
written. All three sit on the same surface this spec owns: what identifies a policy, and
what may remove one. They are appended rather than merged into the list above, so the
first pass stays as it was written.

The three responder rollback sites are NOT repeated here. The list above already carries
them as "Audit the three responder rollback sites for the same hazard", and they are
`responder.go`, `:821` and `:846`. Each calls `removeChildSA`, which is
`removeChildSAExcept(child, nil, ...)`, so a rollback can remove a policy a surviving
pair still answers to.

**1. `samePolicySelector` flips orientation on a peer-initiated rekey.** `selectorPort`
(`internal/component/ike/engine/child.go`) chooses which side of the selector pair to
read from `child.LocalIsInitiator` at `:732`. `newRekeyedChild` resets that field. So the
same selector compares unequal across a peer-initiated rekey, and `samePolicySelector`
(`child.go`) answers false where it should answer true.

Refuted as a policy DROP: an incorrect false makes `removeChildSAExcept` drop a policy it
should keep, which the upsert then reinstalls. The real consequence is an ORPHANED policy,
left behind because the guard did not recognise the pair that still needs it. Exercising it
needs a peer-proposed port narrowing, which no current test produces. READ, not measured.

**2. Removal and install disagree on what identifies a policy.** Install passes `SPParams`
carrying `Proto`, `IfID`, `ReqID`, `Action` and `Priority`
(`internal/component/ike/dataplane/dataplane.go`). Removal is the three-argument
`RemovePolicy(src, dst *net.IPNet, dir SADir)` (`dataplane.go`), which carries no
ports, no protocol and no interface id.

This predates the `keep` work, so it is not a regression. It matters now because the `keep`
design assumes removal matches install: a guard that decides "this selector is still in
use" is only as good as the identity the removal call can express. A removal keyed on
fewer fields than the install can remove a policy the guard meant to protect.

**3. `resolvePendingAfterOwnerLoop` returns early on a nil `pendingSA`.** The function
(`internal/component/ike/engine/fsm.go`, called at `:330`) returns before it can
consider a `pendingChild`. A non-nil `pendingChild` holding the only claim on a shared
policy is therefore stranded when `pendingSA` is nil.

READ, not measured, and stated that way deliberately. What would settle it is a case that
produces a nil `pendingSA` beside a non-nil `pendingChild`. If that combination cannot
arise, the finding dissolves, and proving it cannot arise is the cheaper answer.

**All three stay OPEN and none is closed by the first pass.** Rows in
`plan/deferrals/rfcgate-1b-rfc7296-pilot.md` name this spec as their destination.
