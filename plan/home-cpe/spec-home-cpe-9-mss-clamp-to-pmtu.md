# Spec: home-cpe-9-mss-clamp-to-pmtu

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P3 |
| Release reason | Feature: a home CPE behind PPPoE clamps forwarded SYNs to the route MTU without the operator computing the value; a fixed `tcp-mss-set` already covers the need, so this waits |
| RFC gaps | - |
| Depends | `plan/home-cpe/spec-home-cpe-21-mss-clamp-not-raise.md` (its netns test harness and its proof that the kernel never raises an MSS) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 9).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Add a firewall term action that clamps the TCP MSS option of a SYN to the MTU
of the route the packet takes, less the fixed IP and TCP headers: the nft
`tcp option maxseg size set rt mtu` idiom, and what VyOS writes for
`adjust-mss clamp-mss-to-pmtu` on a PPPoE interface. With a PPPoE MTU of 1492
a SYN with MSS 1460 leaves at 1452 (IPv4) or 1432 (IPv6); a SYN already below
that leaves unchanged.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/firewall.md` - term actions table (l.303 `tcp-mss-set`), base and regular chains, hooks
  → Decision: the action is a firewall term action beside `tcp-mss-set`, mutually exclusive with it in one term
  → Constraint: a regular chain (no hook) runs only through a jump or goto; the kernel refuses an `rt` TCPMSS expression in a chain reachable from a hook that does not allow it, so verify must walk jumps
- [ ] `docs/guide/policy-routing.md` - policy routing installs its chain at prerouting (`internal/plugins/policyroute/translate.go` l.48-52, priority -150)
  → Constraint: prerouting cannot use this expression (below), so policy routing does not offer it
- [ ] `plan/home-cpe/spec-home-cpe-21-mss-clamp-not-raise.md` - the kernel guard in `nft_exthdr_tcp_set_eval` ("increase can cause connection to stall") makes any `exthdr` MSS write a clamp
- [ ] `ai/doors/system.md` and the firewall door - before the first code edit

### RFC Summaries (Scope: protocol)
- [ ] no `rfc/short/` summary for RFC 9293 or RFC 6691, none written (validation scope does not grow); texts in `rfc/full/`
  → Constraint: RFC 6691 Section 2: "When calculating the value to put in the TCP MSS option, the MTU value SHOULD be decreased by only the size of the fixed IP and TCP headers and SHOULD NOT be decreased to account for any possible IP or TCP options" - the kernel's computation (MTU minus 40 for IPv4, minus 60 for IPv6) matches it
  → Constraint: RFC 9293 Section 3.7.1: "The MSS value to be sent in an MSS Option should be equal to the effective MTU minus the fixed IP and TCP headers."

**Key insights (Linux `net/netfilter/nft_rt.c`, read from the upstream tree 2026-10-31):**
- `get_tcpmss` takes the minimum of the packet's route MTU and the MTU of the route back toward the packet's source, then subtracts the fixed IP and TCP header length for the family.
- `nft_rt_validate` allows `NFT_RT_TCPMSS` only at the forward, output and postrouting hooks; elsewhere the rule is refused by the kernel.
- The vendored `github.com/google/nftables` carries `expr.Rt` with key `RtTCPMSS` (`vendor/github.com/google/nftables/expr/rt.go` l.31); no new module.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` l.466 - `tcp-mss-set` (fixed value); no route-based form
- [ ] `internal/component/firewall/model.go` l.669 - `SetTCPMSS{Size}`
- [ ] `internal/component/firewall/config.go` l.419-427 - parse of `tcp-mss-set`
- [ ] `internal/component/firewall/validate.go` l.387 - family check for `SetTCPMSS`
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerSetTCPMSS` (l.1416), action switch (l.656)
- [ ] `internal/plugins/firewall/vpp/verify.go` l.353 - VPP refuses `SetTCPMSS`

**Behavior to preserve:**
- `tcp-mss-set` unchanged.

**Behavior to change:**
- New action; nothing else.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: firewall term action leaf `tcp-mss-clamp-to-pmtu` (presence, no value).

### Transformation Path
1. Parse to a new abstract action (clamp to route MSS) in `internal/component/firewall/model.go`.
2. Verify: family ip, ip6 or inet; not together with `tcp-mss-set` in one term; the term's chain is a base chain at forward, output or postrouting, or a regular chain reachable only from such chains (jump and goto walked); otherwise refused with an error naming the table, chain, term, the hook found and the fix (move the term to a forward or postrouting chain).
3. Lower (nft): an `rt` expression with key TCPMSS into a register, then the same `exthdr` TCP-option write `lowerSetTCPMSS` uses, from that register.
4. VPP: refused at verify like `tcp-mss-set`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to firewall model | `config.go` term action parse | No |
| Model to nft | lowering | No |
| nft to kernel | rule add; kernel validates the hook | No |

### Integration Points
- The firewall action switch in `lower_linux.go` and `validate.go`, beside `SetTCPMSS`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | lowering inside the nft backend |
| No unintended coupling | Yes (design) | no other component |
| No duplicated functionality | Yes (design) | shares the `exthdr` write with `lowerSetTCPMSS` |
| Zero-copy preserved | N-A | config path |
| Registration over hardcoding, outbound | No | firewall actions are a closed sum type lowered by type switch; this adds one arm in each backend, as every firewall action does. Recorded, not changed by this spec |
| Registration over hardcoding, inbound | Yes (design) | the verify hook rule derives from the chain's hook, not a name list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The route MTU of a PPPoE device route is the PPPoE MTU (1492) | kernel `dst_mtu` of a route via `pppN` | the clamp uses 1500 and does nothing | netns test with a 1492 MTU device | unvalidated |
| A-2 | The reverse-route MTU term does not lower the value below the forward path on a CPE | `get_tcpmss` source | a LAN with a smaller MTU clamps more, which is still correct | test with asymmetric MTUs | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Jump walk in verify misses a path and the kernel refuses the ruleset at commit | commit error from netlink | verify walks jumps and gotos; a test covers a regular chain reached from prerouting |
| R-2 | Operator puts the term in an input chain | verify refusal | error names the fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a ruleset the kernel refuses, so the whole firewall commit fails; or SYNs unclamped |
| How is it reverted? | single commit revert; configs using the leaf must drop it |
| Who else touches this path? | child 21 (same lowering, test harness) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `tcp-mss-clamp-to-pmtu` in a forward chain term | → | parse, verify, lowering, kernel | `TestTCPMSSClampToPMTU` in `internal/plugins/firewall/nft/integration_linux_test.go` |
| same in config through the daemon | → | firewall plugin apply | `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Forward chain term, egress device MTU 1492, SYN MSS 1460, IPv4 | forwarded SYN carries 1452 |
| AC-2 | Same, IPv6 | 1432 |
| AC-3 | SYN MSS 1200 | unchanged |
| AC-4 | Term in a prerouting or input chain, or in a regular chain jumped to from one | verify refuses, naming table, chain, term, hook, and the fix |
| AC-5 | Term with both `tcp-mss-set` and `tcp-mss-clamp-to-pmtu` | verify refuses, naming both leaves |
| AC-6 | Table family bridge, arp or netdev | verify refuses (as for `tcp-mss-set`) |
| AC-7 | Backend vpp | verify refuses, naming the backend |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Converts VyOS `adjust-mss clamp-mss-to-pmtu` on the PPPoE interface to a forward-chain term matching `oifname` of the PPPoE interface and TCP SYN | config → firewall → nft → kernel | `firewall-tcp-mss-clamp-to-pmtu.ci`; lab `lan-services` (child 11) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseTCPMSSClampToPMTU` | `internal/component/firewall/config_test.go` | parse; AC-5 | |
| `TestValidateClampToPMTUHook` | `internal/component/firewall/validate_test.go` | AC-4 including jump and goto paths; AC-6 | |
| `TestLowerClampToPMTU` | `internal/plugins/firewall/nft/lower_linux_test.go` | `rt` TCPMSS then `exthdr` write | |
| `TestVPPRefusesClampToPMTU` | `internal/plugins/firewall/vpp/verify_test.go` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | presence leaf | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTCPMSSClampToPMTU` | `internal/plugins/firewall/nft/integration_linux_test.go` | AC-1..AC-3 on Ze's kernel; red phase: remove the term, AC-1 goes red | |
| `firewall-tcp-mss-clamp-to-pmtu` | `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci` | config to installed rule, and AC-4 refusal text | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lan-services` MSS step | owned by child 11 | third-party TCP client and server across the PPPoE path | the clamp reaches the far side as 1452 | |

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang` - new leaf beside `tcp-mss-set`
- `internal/component/firewall/model.go`, `config.go`, `validate.go`
- `internal/plugins/firewall/nft/lower_linux.go`
- `internal/plugins/firewall/vpp/verify.go`
- `docs/guide/firewall.md`

## Files to Create
- `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | new leaf |
| YANG validation constraints | Yes | presence; exclusivity with `tcp-mss-set` expressed in YANG where a `must` is supported, else verify |
| YANG custom validators | Yes | hook walk in `validate.go` |
| CLI commands/flags | N-A | none |
| CLI grammar | N-A | none |
| Editor autocomplete | Yes | automatic |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | No | `NFT_RT` is part of nf_tables core on Ze's kernel; a distro kernel check belongs to the kernel-modules enrolment if it is modular |
| Prometheus counters/metrics | No | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` firewall entry |
| 2 | Config syntax changed? | Yes | `docs/guide/firewall.md` actions table |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, with the PPPoE example |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-9-mss-clamp-to-pmtu.md` |
| 17 | Existing examples? | Yes | firewall guide MSS example |

## Implementation Steps

1. **Phase: Wiring** - leaf parses to the new action and reaches the nft action switch; the integration test fails (no lowering).
2. **Phase: Verify** - hook walk, exclusivity, family, VPP refusal (AC-4..AC-7).
3. **Phase: Lowering** - `rt` TCPMSS plus the shared `exthdr` write (AC-1..AC-3).
4. **Phase: Docs** - guide entry and the VyOS mapping note for child 10.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | verify refuses every chain the kernel would refuse, so a commit never fails at netlink |
| Data flow | the `exthdr` write is shared with `lowerSetTCPMSS`, not duplicated |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Clamp to route MTU | `TestTCPMSSClampToPMTU` green on Ze's kernel |
| Refusals | `validate_test.go` cases |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | a refused ruleset leaves the previous ruleset in place (existing firewall commit behaviour) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Kernel refuses a ruleset verify accepted | DESIGN: the hook walk is incomplete |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

None (grep of `plan/journal/` for MSS).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| New presence leaf `tcp-mss-clamp-to-pmtu` | a union on `tcp-mss-set` (number or keyword); a per-interface `adjust-mss` leaf as VyOS has | a separate leaf keeps the existing leaf's type; a per-interface leaf would be a second producer of firewall rules outside the firewall config |
| Firewall only, not policy routing | offer it in policy routing | policy routing runs at prerouting, where the kernel refuses the expression |
| Verify walks jumps | trust the kernel's refusal | the kernel's refusal fails the whole commit with a netlink error the operator cannot map to a term |

## Known Limitations

- VPP backend: refused.
- A per-interface shorthand (VyOS `adjust-mss` under the interface) is not offered; the mapping guide (child 10) writes the firewall term.
