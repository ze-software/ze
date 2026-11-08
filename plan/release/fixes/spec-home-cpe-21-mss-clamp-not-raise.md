# Spec: home-cpe-21-mss-clamp-not-raise

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | Release validation and documentation: the reported defect (a smaller MSS is raised) does not reach the wire on Linux, because the kernel refuses to raise it, but no test proves the clamp on the state Ze installs and the policy-routing YANG text describes an unconditional rewrite |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-07 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 21).

RFC gaps: none. RFC 9293 Sections 3.7.1 and 3.7.2 and RFC 6691 explain why raising is
harmful; Ze is a middlebox here and holds no requirement row.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Finding that reshapes this spec

The umbrella records "`tcp-mss` and `tcp-mss-set` write the value without
comparing; a clamp writes min(current, value)", and asked for P1. Read at the
producer, Ze does emit an unconditional write (`lowerSetTCPMSS`,
`internal/plugins/firewall/nft/lower_linux.go`: an immediate into a
register, then an `exthdr` TCP-option write of kind 2, offset 2, length 2). But
the kernel that executes that write compares first. Linux
`net/netfilter/nft_exthdr.c`, `nft_exthdr_tcp_set_eval`, read from the
upstream tree on 2026-10-31 and re-read at the stable tag `v7.3` (the runtime
kernel Ze builds, `kernel.version` in the runtime-kernel cache) on 2026-11-02:

> `case TCPOPT_MSS:` / `/* increase can cause connection to stall */` /
> `if (ntohs(old.v16) <= ntohs(new.v16)) return;`

The guard sits inside the `case 2:` arm of `switch (priv->len)`, so it applies
only to a two-byte write. The `case 4:` arm compares old and new for equality
and nothing else. The no-raise outcome therefore depends on the expression Ze
installs: the kernel guards a two-byte write of the MSS value, and Ze's
lowering is what makes the write two bytes. This is why the no-raise proof has
a discriminating break in Ze's code (TDD Test Plan) and is not a
`kernel-producer` case: that escape is for an outcome no state Ze installs
decides, and here the installed write length decides it.

So on the nftables backend the stack already writes min(current, value): a SYN
carrying MSS 1300 through `tcp-mss-set 1400` leaves at 1300. The VPP backend
refuses the action (`internal/plugins/firewall/vpp/verify.go`, the
`firewall.SetTCPMSS` case of `verifyAction`). No path raises an MSS. Per `ai/META-DECISIONS.md`, a requirement the operating system
meets on Ze's behalf still owes a test that checks the OS behaviour on the
state Ze installs; until it exists this is a gap in testing, not in
correctness. Priority is therefore P2, not P1, and the umbrella's capability
row ("rewrites rather than clamps") should be corrected by its owner.

## Task

1. Prove, on Ze's kernel, that the rule Ze installs for `tcp-mss` (policy
   routing) and `tcp-mss-set` (firewall) lowers a larger MSS to the configured
   value and leaves a smaller one alone, for IPv4 and IPv6. The two halves are
   separate proofs with separate breaks: the lowering proof (AC-1) goes red when
   the rule is absent, and the no-raise proof (AC-2) goes red when Ze's lowering
   writes four bytes instead of two, which escapes the kernel's guard.
2. Make every description say what the stack does: a clamp that never raises.
   The policy-routing leaf says today "Ze writes this value into the TCP MSS
   option ... and it does not read the value already there", which a reader
   takes as "raises a smaller MSS".
3. Decide whether set semantics (raise) are needed: they are not (Key Design
   Decisions).

## Required Reading

### Architecture Docs
- [ ] `docs/guide/firewall.md`, the term actions table, row `tcp-mss-set` - "`tcp-mss-set` | Clamp TCP MSS | `tcp-mss-set 1400;`"
  → Constraint: correct as written; gains one sentence that a smaller MSS is left alone and only SYN segments carry the option
- [ ] `docs/guide/policy-routing.md`, the actions table and the MSS example - "`tcp-mss N` | Clamp TCP MSS to N bytes"
  → Constraint: correct as written; same sentence added
- [ ] `ai/doors/system.md` and the firewall door - before the first code edit

### RFC Summaries (Scope: protocol)
- [ ] no `rfc/short/` summary exists for RFC 9293, RFC 879 or RFC 6691, and none is written (RFC validation scope does not grow; no profile lists them for a middlebox). The texts are in `rfc/full/rfc9293.txt` and `rfc/full/rfc6691.txt`
  → Constraint: RFC 9293 Section 3.7.2: "The TCP MSS Option specifies an upper bound for the size of packets that can be received (see [43])." Raising the value tells the sender it may send segments the receiver did not offer to take; lowering it only shrinks segments
  → Constraint: RFC 6691 Section 2: "When calculating the value to put in the TCP MSS option, the MTU value SHOULD be decreased by only the size of the fixed IP and TCP headers and SHOULD NOT be decreased to account for any possible IP or TCP options" - the operator's value for a 1492-byte PPPoE MTU is 1452 (IPv4) and 1432 (IPv6), which the guide's example states

**Key insights:**
- The kernel guard covers a two-byte write of option kind 2 only (the `case 2:` arm of `switch (priv->len)`). A four-byte write over the same option (kind, length and value from offset 0) meets no guard and can raise the MSS. Ze's lowering writes two bytes at offset 2, so the installed rule never raises.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerSetTCPMSS` emits the write with no comparison; its comment says "clamps"
- [ ] `internal/plugins/policyroute/translate.go` - `allocator.buildActions` appends `firewall.SetTCPMSS` when `TCPMSS > 0`, before any terminal action
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - leaf `tcp-mss` description and help "Rewrite the TCP MSS option to this value in bytes."
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - leaf `tcp-mss-set`, help "Clamp TCP Maximum Segment Size option to the given value"
- [ ] `internal/component/firewall/validate.go` - `validateAction`, the `SetTCPMSS` case: family check (ip, ip6, inet)
- [ ] `internal/plugins/firewall/vpp/verify.go` - `verifyAction`, the `firewall.SetTCPMSS` case: VPP refusal
- [ ] `test/policy/policy-tcp-mss.ci` - asserts the rule is installed, not the packet effect
- [ ] Linux `net/netfilter/nft_exthdr.c` `nft_exthdr_tcp_set_eval` - the guard quoted above

**Behavior to preserve:**
- Rule shape and the leaf names; the VPP refusal; the family check.

**Behavior to change:**
- Descriptions and help text only; tests added. No lowering change.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall MSS action | `internal/component/firewall/model.go` `SetTCPMSS`; parsed in `internal/component/firewall/config.go` (`tcp-mss-set`) | the one action both config surfaces lower to | does not apply: unchanged; this spec proves its effect and corrects its text |
| nft MSS lowering | `internal/plugins/firewall/nft/lower_linux.go` `lowerSetTCPMSS` | emits an `immediate` into a register and an `exthdr` TCP-option write of kind 2, offset 2, length 2, with no comparison | extends: only its comment, which gains the kernel guard it relies on; the expression is unchanged |
| Kernel MSS guard | Linux `net/netfilter/nft_exthdr.c` `nft_exthdr_tcp_set_eval` | skips the write when the packet's MSS is already less than or equal to the value | does not apply as code: Ze relies on it, and the netns test proves it on the rule Ze installs (`ai/META-DECISIONS.md`, a requirement the OS meets on Ze's behalf still owes a test) |
| Policy-routing MSS | `internal/plugins/policyroute/translate.go` `allocator.buildActions`; `docs/architecture/policyroute/policy-routing.md` | appends `firewall.SetTCPMSS` when `TCPMSS > 0`, before any terminal action | does not apply: unchanged; the new `.ci` drives it end to end |
| Family check | `internal/component/firewall/validate.go` (the `SetTCPMSS` case) | refuses `tcp-mss-set` outside families ip, ip6 and inet | does not apply: preserved |
| VPP refusal | `internal/plugins/firewall/vpp/verify.go` (the `firewall.SetTCPMSS` case); `docs/architecture/firewall/fw-6-firewall-vpp.md` | refuses the action at verify on the VPP backend | does not apply: preserved |
| nft netns integration harness | `internal/plugins/firewall/nft/integration_linux_test.go` (named netns setup through `netns.NewNamed`) | applies tables inside a throwaway namespace on a real kernel | extends: the forwarding proof is a new test in the same file and harness, no second harness |
| Policy MSS functional test | `test/policy/policy-tcp-mss.ci` | asserts the ruleset carries `tcp option maxseg size set 1436`, not the packet effect | extends: kept; `policy-tcp-mss-clamp.ci` adds the packet effect |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `policy-route ... tcp-mss N`, `firewall table ... term ... tcp-mss-set N`.

### Transformation Path
1. Parse into `firewall.SetTCPMSS{Size}` (policy routing through `buildActions`, firewall through the `tcp-mss-set` branch of `parseThenBlock` in `config.go`).
2. Lower to nft `immediate` plus `exthdr` TCP-option write.
3. Kernel: the write is skipped when the packet's MSS is already less than or equal to the value.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | nftables rule | Yes (source read); test owed |
| Kernel to wire | rewritten SYN | No: the test of this spec |

### Integration Points
- `internal/plugins/firewall/nft/integration_linux_test.go` - existing netns harness the new test extends.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | no code path change |
| No unintended coupling | Yes | no code path change |
| No duplicated functionality | Yes | no new action |
| Zero-copy preserved | N-A | no code path change |
| Registration over hardcoding, outbound | N-A | no new command or handler |
| Registration over hardcoding, inbound | N-A | no new name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Ze's kernel (7.2.9, 7.3 runtime builds) carries the MSS guard in `nft_exthdr_tcp_set_eval`, for two-byte writes only | upstream source read 2026-10-31; `v7.3` stable tag read 2026-11-02 (same guard, inside `case 2:`) | Ze raises MSS: the spec becomes the P1 fix (lower a `tcp option maxseg size gt N` comparison before the write) | the netns test of AC-2 on Ze's kernel | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A-1 false on some kernel Ze supports (distro kernels via the systemd install) | AC-2 red on a distro VM | lower an explicit comparison then; it costs one `exthdr` load and one `cmp` and is harmless on kernels that already guard |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | nothing new: tests and text only |
| How is it reverted? | single commit revert |
| Who else touches this path? | child 9 (clamp-to-PMTU) adds a sibling action in the same lowering |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| firewall `tcp-mss-set 1400` in a forward chain | → | `lowerSetTCPMSS` rule in the kernel, SYN through a veth pair | `TestTCPMSSSetClampsNeverRaises` in `internal/plugins/firewall/nft/integration_linux_test.go` |
| policy-route `tcp-mss 1400` | → | `buildActions` then the same lowering | `test/policy/policy-tcp-mss-clamp.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SYN with MSS 1460 through `tcp-mss-set 1400`, IPv4 and IPv6 | forwarded SYN carries 1400 |
| AC-2 | SYN with MSS 1300 through `tcp-mss-set 1400` | forwarded SYN carries 1300 |
| AC-3 | SYN with no MSS option | forwarded unchanged (no option added) |
| AC-4 | The same three through policy-routing `tcp-mss 1400` | same results |
| AC-5 | `ze schema` help and description of both leaves | say the value is an upper bound: a larger MSS is lowered to it, a smaller one is left alone |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Clamps LAN-to-WAN SYNs to 1452 for a PPPoE line | config → nft rule → kernel | `TestTCPMSSSetClampsNeverRaises` and lab `lan-services` (child 11) |
| 2 | A LAN host that already advertises 1200 (its own tunnel) keeps 1200 | same | AC-2 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerSetTCPMSS` (existing, kept) | `internal/plugins/firewall/nft/lower_linux_test.go` | expression shape | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `tcp-mss-set`, `tcp-mss` | 1-65535 | 65535 | 0 | 65536 (YANG range) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTCPMSSSetClampsNeverRaises` | `internal/plugins/firewall/nft/integration_linux_test.go` | AC-1..AC-3, IPv4 and IPv6, two netns joined through a forwarding netns, SYN crafted with a raw socket, MSS read on the far side. Two proofs, two breaks (below) | |

**Red walks, one per proof.** Each break is made in Ze's producing code, the
test is run red on Ze's kernel, then the code is restored and the run is green.
No `RFC requirement:` tag is involved (no requirement row), so the result is
recorded in this spec's closure, not through `./le rfc record`.

| Proof | Break (producer) | Expected red | Why it discriminates |
|-------|------------------|--------------|----------------------|
| Lowering (AC-1) | the term's `SetTCPMSS` action is not lowered (`lowerAction` returns no expressions for it) | the 1460 SYN arrives as 1460 | a correct stack writes 1400; only an absent write leaves 1460 |
| No raise (AC-2) | `lowerSetTCPMSS` writes four bytes from option offset 0 (kind 2, length 4, the value) instead of two bytes at offset 2 | the 1300 SYN arrives as 1400 | the four-byte write takes the `case 4:` arm of `nft_exthdr_tcp_set_eval`, which has no MSS guard, so only the two-byte shape Ze installs keeps 1300 |

The earlier proposal, changing the configured value from 1400 to 1500, is not
a break: 1300 stays 1300 under both values on a correct stack and on a broken
one, so it cannot turn AC-2 red.
| `policy-tcp-mss-clamp` | `test/policy/policy-tcp-mss-clamp.ci` | AC-4 through config | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lan-services` MSS step | owned by child 11 | a third-party TCP client and server | MSS 1400 written on SYN, smaller left alone | |

## Files to Modify
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - `tcp-mss` description and help
- `internal/component/firewall/yang/ze-firewall-conf.yang` - `tcp-mss-set` description
- `internal/plugins/firewall/nft/lower_linux.go` - `lowerSetTCPMSS` comment names the kernel guard it relies on
- `docs/guide/firewall.md`, `docs/guide/policy-routing.md` - one sentence each
- `docs/architecture/core-design.md` - declared by `lower_linux.go` (nftables expression lowering); it says nothing about the MSS action, and the expression is unchanged, so nothing becomes wrong
- `docs/architecture/policyroute/policy-routing.md` - declared by `translate.go`; "`tcp-mss N` ... adjusts MSS" stays true and `translate.go` is unchanged, so nothing becomes wrong
- `docs/architecture/firewall/fw-6-firewall-vpp.md` - declared by `internal/plugins/firewall/vpp/verify.go`; the VPP refusal is preserved, so nothing becomes wrong

## Files to Create
- `test/policy/policy-tcp-mss-clamp.ci`
- test function in the existing `integration_linux_test.go` (no new file)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | descriptions only |
| YANG validation constraints | No | unchanged |
| YANG custom validators | No | unchanged |
| CLI commands/flags | N-A | none |
| CLI grammar | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | none |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | No | no new dependency |
| Prometheus counters/metrics | No | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | none |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, `docs/guide/policy-routing.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed? | No | none |
| 10 | Test infrastructure changed? | No | existing harness |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-21-mss-clamp-not-raise.md` |
| 17 | Existing examples? | Yes | the `policy-routing.md` MSS example value checked against RFC 6691 arithmetic |

## Implementation Steps

1. **Phase: Wiring** - the netns test installs the rule from a parsed config term and sends one SYN; it fails before the assertions exist.
2. **Phase: Proof** - AC-1..AC-3 both families; the policy-routing `.ci`; if AC-2 is red, A-1 is broken: route to DESIGN and lower the comparison (R-1).
3. **Phase: Text** - YANG descriptions, lowering comment, guides.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | the test reads the forwarded SYN, not the ruleset |
| Rule: stale-comments | `lowerSetTCPMSS` comment and both YANG descriptions match the stack behaviour |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Clamp proven | `TestTCPMSSSetClampsNeverRaises` green on Ze's kernel |
| Text | grep of both YANG files shows no "does not read the value already there" |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | unchanged range 1-65535 |

### Failure Routing

| Failure | Route To |
|---------|----------|
| AC-2 red | A-1 broken: DESIGN, lower an explicit comparison |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A survey that reads only Ze's lowering concludes "rewrite"; the stack's answer is the kernel's. Conformance and behaviour claims are judged on what the whole stack produces (`ai/rules/rfc-compliance.md`).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/design-claim-never-measured.md` | 2026-11-08 | the comment above `lowerSetTCPMSS` and the AC-2 break: the kernel refused the four-byte write in child 11's lab |

Before 2026-11-08 no journal row recorded this (grep of `plan/journal/` for `tcp-mss`, `TCPMSS`, `MSS`).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No set (raise) semantics | a `mode set` leaf | the kernel cannot raise through `exthdr` set; nobody asked for it; RFC 9293 Section 3.7.2 makes the MSS an upper bound the receiver chose |
| Keep the lowering as is | lower an explicit `gt` comparison now | redundant on every kernel Ze builds; kept as the fallback if A-1 fails on a supported distro kernel |
| Leaf names unchanged | rename `tcp-mss-set` to `tcp-mss-clamp` | a rename breaks configurations for a word; the description carries the meaning |

## Known Limitations

- VPP backend: no MSS action; unchanged, refused at verify.
- Clamp to the route MTU is child 9 (`spec-home-cpe-9-mss-clamp-to-pmtu.md`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | the spec cited Section 3.7.1 for the upper-bound sentence | it is in 3.7.2 | closure review against rfc/full/rfc9293.txt | citations corrected |

## Evidence

| Item | Result |
|------|--------|
| `TestTCPMSSSetClampsNeverRaises` | passes on Ze's 7.3 kernel in QEMU |
| `TestTCPMSSClampToPMTU` | passes on Ze's 7.3 kernel in QEMU |
| `test/policy/policy-tcp-mss-clamp.ci` | passes on Ze's kernel in QEMU |
| AC-1 red walk | recorded 2026-11-07: with `lowerSetTCPMSS` not called (the action not lowered), `TestTCPMSSSetClampsNeverRaises` went red; restored, green |
| AC-2 red walk | still owed. The four-byte write break of the TDD Test Plan was run in child 11's `lan-services` lab on 2026-11-08 (walk `c11-mss-raises`): the kernel refused the rule (`firewallnft: flush: ... netlink receive: operation not supported`), so it cannot prove the no-raise clause. Child 11's replacement break, a transport-header payload write of the register over the MSS value (walk `c11-mss-raises-payload`), is the candidate for this walk too |

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Existing Mechanisms table names every mechanism this spec relies on or extends
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le test unit plugins`; `./le test integration firewall`; on Ze's kernel, `./le test qemu run kernel tmp/kernel/build/vmlinuz packages "nftables iproute2 libcap" timeout 1200s command 'go test -tags integration -count=1 -run TestTCPMSSSetClampsNeverRaises ./internal/plugins/firewall/nft/'`; and `./le test policy` for `policy-tcp-mss-clamp`; then `./le verify changed` over the closing commits
- [ ] Both red walks of the TDD Test Plan run red and green, outputs pasted
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests: YANG range unchanged (1-65535)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: the `lan-services` MSS step of child 11

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
