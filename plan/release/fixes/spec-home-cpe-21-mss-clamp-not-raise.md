# Spec: home-cpe-21-mss-clamp-not-raise

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | Release validation and documentation: the reported defect (a smaller MSS is raised) does not reach the wire on Linux, because the kernel refuses to raise it, but no test proves the clamp on the state Ze installs and the policy-routing YANG text describes an unconditional rewrite |
| RFC gaps | - (RFC 9293 Section 3.7.1 and RFC 6691 explain why raising is harmful; Ze is a middlebox here and holds no requirement row) |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 21).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Finding that reshapes this spec

The umbrella records "`tcp-mss` and `tcp-mss-set` write the value without
comparing; a clamp writes min(current, value)", and asked for P1. Read at the
producer, Ze does emit an unconditional write (`lowerSetTCPMSS`,
`internal/plugins/firewall/nft/lower_linux.go` l.1416: an immediate into a
register, then an `exthdr` TCP-option write of kind 2, offset 2, length 2). But
the kernel that executes that write compares first. Linux
`net/netfilter/nft_exthdr.c`, `nft_exthdr_tcp_set_eval`, read from the
upstream tree on 2026-10-31:

> `case TCPOPT_MSS:` / `/* increase can cause connection to stall */` /
> `if (ntohs(old.v16) <= ntohs(new.v16)) return;`

So on the nftables backend the stack already writes min(current, value): a SYN
carrying MSS 1300 through `tcp-mss-set 1400` leaves at 1300. The VPP backend
refuses the action (`internal/plugins/firewall/vpp/verify.go` l.353). No path
raises an MSS. Per `ai/META-DECISIONS.md`, a requirement the operating system
meets on Ze's behalf still owes a test that checks the OS behaviour on the
state Ze installs; until it exists this is a gap in testing, not in
correctness. Priority is therefore P2, not P1, and the umbrella's capability
row ("rewrites rather than clamps") should be corrected by its owner.

## Task

1. Prove, on Ze's kernel, that the rule Ze installs for `tcp-mss` (policy
   routing) and `tcp-mss-set` (firewall) lowers a larger MSS to the configured
   value and leaves a smaller one alone, for IPv4 and IPv6.
2. Make every description say what the stack does: a clamp that never raises.
   The policy-routing leaf says today "Ze writes this value into the TCP MSS
   option ... and it does not read the value already there", which a reader
   takes as "raises a smaller MSS".
3. Decide whether set semantics (raise) are needed: they are not (Key Design
   Decisions).

## Required Reading

### Architecture Docs
- [ ] `docs/guide/firewall.md` l.303 - "`tcp-mss-set` | Clamp TCP MSS | `tcp-mss-set 1400;`"
  → Constraint: correct as written; gains one sentence that a smaller MSS is left alone and only SYN segments carry the option
- [ ] `docs/guide/policy-routing.md` l.72, l.185-188 - "`tcp-mss N` | Clamp TCP MSS to N bytes"
  → Constraint: correct as written; same sentence added
- [ ] `ai/doors/system.md` and the firewall door - before the first code edit

### RFC Summaries (Scope: protocol)
- [ ] no `rfc/short/` summary exists for RFC 9293, RFC 879 or RFC 6691, and none is written (RFC validation scope does not grow; no profile lists them for a middlebox). The texts are in `rfc/full/rfc9293.txt` and `rfc/full/rfc6691.txt`
  → Constraint: RFC 9293 Section 3.7.1: "The TCP MSS Option specifies an upper bound for the size of packets that can be received (see [43])." Raising the value tells the sender it may send segments the receiver did not offer to take; lowering it only shrinks segments
  → Constraint: RFC 6691 Section 2: "When calculating the value to put in the TCP MSS option, the MTU value SHOULD be decreased by only the size of the fixed IP and TCP headers and SHOULD NOT be decreased to account for any possible IP or TCP options" - the operator's value for a 1492-byte PPPoE MTU is 1452 (IPv4) and 1432 (IPv6), which the guide's example states

**Key insights:**
- The kernel guard is unconditional for kind 2; no nft expression can raise an MSS through `exthdr` set.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerSetTCPMSS` (l.1416) emits the write with no comparison; its comment says "clamps"
- [ ] `internal/plugins/policyroute/translate.go` - `buildActions` (l.251) appends `firewall.SetTCPMSS` when `TCPMSS > 0`, before any terminal action
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` l.246-258 - leaf `tcp-mss` description and help "Rewrite the TCP MSS option to this value in bytes."
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` l.466-474 - leaf `tcp-mss-set`, help "Clamp TCP Maximum Segment Size option to the given value"
- [ ] `internal/component/firewall/validate.go` l.387 - family check (ip, ip6, inet)
- [ ] `internal/plugins/firewall/vpp/verify.go` l.353 - VPP refusal
- [ ] `test/policy/policy-tcp-mss.ci` - asserts the rule is installed, not the packet effect
- [ ] Linux `net/netfilter/nft_exthdr.c` `nft_exthdr_tcp_set_eval` - the guard quoted above

**Behavior to preserve:**
- Rule shape and the leaf names; the VPP refusal; the family check.

**Behavior to change:**
- Descriptions and help text only; tests added. No lowering change.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `policy-route ... tcp-mss N`, `firewall table ... term ... tcp-mss-set N`.

### Transformation Path
1. Parse into `firewall.SetTCPMSS{Size}` (policy routing through `buildActions`, firewall through `config.go` l.419).
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
| A-1 | Ze's kernel (7.2.9, 7.3 runtime builds) carries the MSS guard in `nft_exthdr_tcp_set_eval` | upstream source read 2026-10-31; the guard predates every kernel Ze builds | Ze raises MSS: the spec becomes the P1 fix (lower a `tcp option maxseg size gt N` comparison before the write) | the netns test of AC-2 on Ze's kernel | unvalidated |

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
| `TestTCPMSSSetClampsNeverRaises` | `internal/plugins/firewall/nft/integration_linux_test.go` | AC-1..AC-3, IPv4 and IPv6, two netns joined through a forwarding netns, SYN crafted with a raw socket, MSS read on the far side. Red phase: lower a set without the kernel's guard is not possible, so the discriminating break is the configured value (1500 must not raise 1300); a second break removes the rule and AC-1 goes red | |
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
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-21-mss-clamp-not-raise.md` |
| 17 | Existing examples? | Yes | `policy-routing.md` l.72 example value checked against RFC 6691 arithmetic |

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

No journal row records this (grep of `plan/journal/` for `tcp-mss`, `TCPMSS`, `MSS`).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No set (raise) semantics | a `mode set` leaf | the kernel cannot raise through `exthdr` set; nobody asked for it; RFC 9293 Section 3.7.1 makes the MSS an upper bound the receiver chose |
| Keep the lowering as is | lower an explicit `gt` comparison now | redundant on every kernel Ze builds; kept as the fallback if A-1 fails on a supported distro kernel |
| Leaf names unchanged | rename `tcp-mss-set` to `tcp-mss-clamp` | a rename breaks configurations for a word; the description carries the meaning |

## Known Limitations

- VPP backend: no MSS action; unchanged, refused at verify.
- Clamp to the route MTU is child 9 (`spec-home-cpe-9-mss-clamp-to-pmtu.md`).
