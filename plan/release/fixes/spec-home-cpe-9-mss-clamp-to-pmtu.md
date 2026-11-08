# Spec: home-cpe-9-mss-clamp-to-pmtu

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P3 |
| Release reason | Feature: a home CPE behind PPPoE clamps forwarded SYNs to the route MTU without the operator computing the value; a fixed `tcp-mss-set` already covers the need, so this waits |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-home-cpe-21-mss-clamp-not-raise.md` (its netns test harness and its proof that the two-byte MSS write never raises); `plan/release/fixes/spec-home-cpe-18-interoplab-multi-link-topology.md` (the topology suite and the shared image registry that `pppoe-mss-pmtu` runs on) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 9).

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
- [ ] `docs/guide/firewall.md` - the term actions table (row `tcp-mss-set`), base and regular chains, hooks
  → Decision: the action is a firewall term action beside `tcp-mss-set`, mutually exclusive with it in one term
  → Constraint: a regular chain (no hook) runs only through a jump or goto; the kernel refuses an `rt` TCPMSS expression in a chain reachable from a hook that does not allow it, so verify must walk jumps
- [ ] `docs/guide/policy-routing.md` - policy routing installs its chain at prerouting (`internal/plugins/policyroute/translate.go`, `allocator.translate`, the chain declared at priority -150)
  → Constraint: prerouting cannot use this expression (below), so policy routing does not offer it
- [ ] `plan/release/fixes/spec-home-cpe-21-mss-clamp-not-raise.md` - the kernel guard in `nft_exthdr_tcp_set_eval` ("increase can cause connection to stall") applies to a two-byte MSS write, which is the write this action shares with `lowerSetTCPMSS`, so the route-based write is a clamp too
- [ ] `ai/doors/system.md`, `ai/doors/forwarding.md` and `ai/doors/system-plugins.md` - before the first code edit
- [ ] `docs/architecture/testing/interop.md` - scenario discovery and the vacuity traps, for `pppoe-mss-pmtu`

### RFC Summaries (Scope: protocol)
- [ ] no `rfc/short/` summary for RFC 9293 or RFC 6691, none written (validation scope does not grow); texts in `rfc/full/`
  → Constraint: RFC 6691 Section 2: "When calculating the value to put in the TCP MSS option, the MTU value SHOULD be decreased by only the size of the fixed IP and TCP headers and SHOULD NOT be decreased to account for any possible IP or TCP options" - the kernel's computation (MTU minus 40 for IPv4, minus 60 for IPv6) matches it
  → Constraint: RFC 9293 Section 3.7.1: "The MSS value to be sent in an MSS Option should be equal to the effective MTU minus the fixed IP and TCP headers."

**Key insights (Linux `net/netfilter/nft_rt.c`, read from the upstream tree 2026-10-31 and at the `v7.3` stable tag 2026-11-02):**
- `get_tcpmss` takes the minimum of the packet's route MTU and the MTU of the route back toward the packet's source, then subtracts the fixed IP and TCP header length for the family.
- `nft_rt_validate` allows `NFT_RT_TCPMSS` only at the forward, output and postrouting hooks, and only in the ip, ip6 and inet families; elsewhere the kernel refuses the rule.
- The vendored `github.com/google/nftables` carries `expr.Rt` with the key constant `RtTCPMSS` (`vendor/github.com/google/nftables/expr/rt.go`); no new module.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - leaf `tcp-mss-set` (fixed value); no route-based form
- [ ] `internal/component/firewall/model.go` - `SetTCPMSS{Size}`, one of the types behind the `Action` marker interface
- [ ] `internal/component/firewall/config.go` - `parseThenBlock`, its `tcp-mss-set` branch
- [ ] `internal/component/firewall/validate.go` - `validateAction`, the `SetTCPMSS` family case
- [ ] `internal/component/firewall/backend.go` - `RegisterBackend`, `RegisterVerifier`, `RunVerifier`
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerAction` (type switch; its default arm answers "unsupported action type"), `lowerSetTCPMSS`
- [ ] `internal/plugins/firewall/vpp/verify.go` - `verifyAction` (type switch; its default arm answers "action type not recognized by backend vpp"), the `firewall.SetTCPMSS` refusal

**Behavior to preserve:**
- `tcp-mss-set` unchanged.

**Behavior to change:**
- New action; nothing else.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall backend registration | `internal/component/firewall/backend.go` `RegisterBackend`, `LoadBackend`; `ai/doors/system-plugins.md` "Firewall lowering is a backend, not an engine" | each backend plugin (`firewall/nft`, `firewall/vpp`) registers from its own `init()`; the component owns the model and never names a backend | extends: the nft backend gains the lowering inside its own package; no central list changes |
| Firewall backend verifier registration | `internal/component/firewall/backend.go` `RegisterVerifier`, `RunVerifier` | a backend registers the check that refuses what it cannot lower, run at verify and by `ApplyAll` | extends: the vpp backend's registered verifier refuses the new action |
| Firewall abstract action model | `internal/component/firewall/model.go` (`Action` interface, `SetTCPMSS`); `docs/architecture/core-design.md`, section 14b | the backend-neutral action types every backend lowers | extends: one new action type beside `SetTCPMSS` |
| Term action parsing | `internal/component/firewall/config.go` `parseThenBlock` | turns the `tcp-mss-set` leaf into `SetTCPMSS{Size}` | extends: the presence leaf parses to the new action in the same function |
| Firewall verify | `internal/component/firewall/validate.go` `validateAction` (the `SetTCPMSS` family case, the `Jump` and `Goto` target checks) | refuses `tcp-mss-set` outside ip, ip6 and inet; checks that jump and goto targets exist | extends: the family check, plus exclusivity with `tcp-mss-set` and the hook walk over jumps and gotos |
| nft action lowering | `internal/plugins/firewall/nft/lower_linux.go` `lowerAction`, `lowerSetTCPMSS` | lowers `SetTCPMSS` to an `immediate` plus a two-byte `exthdr` TCP-option write | extends: one arm that loads `rt` TCPMSS into the register and reuses the same `exthdr` write, never a second copy of it |
| Vendored nft route expression | `vendor/github.com/google/nftables/expr/rt.go` `RtTCPMSS` | carries the `NFT_RT_TCPMSS` key | extends: used as is; no new module |
| Kernel MSS guard | Linux `net/netfilter/nft_exthdr.c` `nft_exthdr_tcp_set_eval`, proven on the installed rule by `plan/release/fixes/spec-home-cpe-21-mss-clamp-not-raise.md` | skips a two-byte MSS write that would raise the value | does not apply as code: makes the route-based write a clamp, as for `tcp-mss-set`, because the write is the same two-byte write |
| Backend capability gate | `ze:backend "nft"` in `internal/component/firewall/yang/ze-firewall-conf.yang`; `docs/architecture/core-design.md`, section 14a | refuses at commit a leaf the selected backend does not implement | extends: the new leaf carries the annotation where the gate applies |
| nft netns integration harness | `internal/plugins/firewall/nft/integration_linux_test.go`, extended by child 21's forwarding proof | applies tables in a throwaway namespace on a real kernel | extends: `TestTCPMSSClampToPMTU` reuses child 21's SYN harness |
| Topology interop suite and image registry | child 18 (`./le test integration interop-topology`, images registered by name, accel-ppp registered by the PPPoE suite from `test/interop-pppoe/Dockerfile.accel`) | runs multi-node scenarios with registered checkers | extends: `pppoe-mss-pmtu` is one more named scenario with its own checker |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: firewall term action leaf `tcp-mss-clamp-to-pmtu` (presence, no value).

### Transformation Path
1. Parse to a new abstract action (clamp to route MSS) in `internal/component/firewall/model.go`.
2. Verify: family ip, ip6 or inet; not together with `tcp-mss-set` in one term; the term's chain is a base chain at forward, output or postrouting, or a regular chain reachable only from such chains (jump and goto walked); otherwise refused with an error naming the table, chain, term, the hook found and the fix (move the term to a forward or postrouting chain).
3. Lower (nft): an `rt` expression with key TCPMSS into a register, then the same two-byte `exthdr` TCP-option write `lowerSetTCPMSS` uses, from that register.
4. VPP: refused at verify like `tcp-mss-set`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to firewall model | `parseThenBlock` | No |
| Model to nft | lowering | No |
| nft to kernel | rule add; kernel validates the hook | No |

### Integration Points
- `lowerAction` in `lower_linux.go`, `validateAction` in `validate.go` and `verifyAction` in `internal/plugins/firewall/vpp/verify.go`, each beside its `SetTCPMSS` arm.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | lowering inside the nft backend |
| No unintended coupling | Yes (design) | no other component |
| No duplicated functionality | Yes (design) | shares the `exthdr` write with `lowerSetTCPMSS` |
| Zero-copy preserved | N-A | config path |
| Registration over hardcoding, outbound | Yes for the backends; the action set itself is open (D-1) | the backends register through `RegisterBackend` and `RegisterVerifier`, so no engine or generic config code names this action. The action is part of the firewall component's own model, parsed by the component's own `parseThenBlock` from its own YANG, and lowered by each backend inside its own package. Each backend's type switch fails closed: its default arm refuses an action it does not know, so a backend that never learns the new action refuses it rather than answering with a plausible value. What stays a closed set is the `Action` sum type: adding an action edits the component's model and each backend's switch. Whether that set should become a per-action registry is D-1 below, an architecture decision this spec does not take |
| Registration over hardcoding, inbound | Yes (design) | the verify hook rule derives from the chain's hook, not a name list |

## Open decision

| ID | Question | Options | Evidence | Effect on this spec |
|----|----------|---------|----------|---------------------|
| D-1 | Should firewall actions become a registry (each action declaring its parse, its verify and a lowering per backend) instead of the closed `Action` sum type that every backend lowers by type switch? | (a) keep the closed sum type: the firewall component owns its actions, backends register, each backend switch fails closed; (b) redesign actions as registrations, a firewall-wide change across every existing action (the model's action types, `parseThenBlock`, `validateAction`, both backends) before this action is added | `model.go` `Action` (marker interface); `lowerAction` and `verifyAction` defaults refuse unknown types; `RegisterBackend`, `RegisterVerifier` in `backend.go`; owner rule in `ai/META-DECISIONS.md` ("a design that has the engine or config code name a specific feature, plugin, field or path is wrong") names engine and config code, and this code is the firewall component's own | (a): this spec proceeds as written. (b): a separate firewall spec lands first and this action becomes one registration in it. Answered by Thomas on 2026-11-08: (a), keep the closed set (see Answer below) |

**D-1 answered by Thomas (2026-11-08), a one-off design answer for this spec,
not a meta rule.** Keep the closed set: firewall actions stay the firewall
component's own closed `Action` set; backends register (`RegisterBackend`,
`RegisterVerifier`) and each backend's lowering switch fails closed on an
unknown action. The owner rule on modular design covers engine and config
code, not a component's own model. This spec proceeds as written.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The route MTU of a PPPoE device route is the PPPoE MTU (1492) | kernel `dst_mtu` of a route via `pppN` | the clamp uses 1500 and does nothing | `pppoe-mss-pmtu` (a real PPP device negotiated with accel-ppp); the netns test proves the arithmetic on a 1492-MTU veth | confirmed (Pre-Commit Verification) |
| A-2 | The reverse-route MTU term does not lower the value below the forward path on a CPE | `get_tcpmss` source | a LAN with a smaller MTU clamps more, which is still correct | `TestTCPMSSClampToPMTU` asymmetric-MTU case | confirmed (Pre-Commit Verification) |
| A-3 | The accel-ppp node clamps no MSS of its own | accel-ppp has no MSS option in the image's configuration | the scenario's 1452 could come from accel-ppp | the scenario's red walk: with Ze's term removed, the 1460 SYN arrives as 1460 | confirmed (Pre-Commit Verification) |

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
| Who else touches this path? | child 21 (same lowering, test harness); child 10 (mapping row for `clamp-mss-to-pmtu`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `tcp-mss-clamp-to-pmtu` in a forward chain term | → | parse, verify, lowering, kernel | `TestTCPMSSClampToPMTU` in `internal/plugins/firewall/nft/integration_linux_test.go` |
| same in config through the daemon | → | firewall plugin apply | `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci` |
| same on a Ze PPPoE client whose MTU came from a real PPPoE session | → | full path, route MTU of `pppN` | scenario `pppoe-mss-pmtu` (`./le test integration interop-topology pppoe-mss-pmtu`) |

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
| AC-8 | `pppoe-mss-pmtu`: Ze PPPoE client with `mtu 1492` against accel-ppp, a forward-chain term matching `output-interface ppp*` and TCP SYN with `tcp-mss-clamp-to-pmtu`; a LAN host behind Ze opens TCP to a server beyond accel-ppp with MSS 1460, then with MSS 1200 | a capture on the server shows 1452 on the first SYN and 1200 on the second; the CPE's `pppN` reports MTU 1492 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Converts VyOS `adjust-mss clamp-mss-to-pmtu` on the PPPoE interface to a term matching `output-interface` of the PPPoE interface and TCP SYN, with `tcp-mss-clamp-to-pmtu` | config → firewall → nft → kernel → wire | `firewall-tcp-mss-clamp-to-pmtu.ci`; scenario `pppoe-mss-pmtu` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseTCPMSSClampToPMTU` | `internal/component/firewall/config_test.go` | parse; AC-5 | |
| `TestValidateClampToPMTUHook` | `internal/component/firewall/validate_test.go` | AC-4 including jump and goto paths; AC-6 | |
| `TestLowerClampToPMTU` | `internal/plugins/firewall/nft/lower_linux_test.go` | `rt` TCPMSS then the two-byte `exthdr` write | |
| `TestVPPRefusesClampToPMTU` | `internal/plugins/firewall/vpp/verify_test.go` | AC-7 | |
| `TestMSSPMTUVerdict` | `internal/le/interoplab/topology/check_mss_pmtu_test.go` | AC-8 verdict from capture fixtures: 1452 and 1200 pass, 1460 or a missing SYN refused | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | presence leaf | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTCPMSSClampToPMTU` | `internal/plugins/firewall/nft/integration_linux_test.go` | AC-1..AC-3 and A-2 on Ze's kernel; red walk: the action not lowered, AC-1 goes red (1460 arrives) | |
| `firewall-tcp-mss-clamp-to-pmtu` | `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci` | config to installed rule, and AC-4 refusal text | |

### Interop Tests (Scope: protocol)

This spec owns its own PMTU case. Child 11's `lan-services` runs the converted
CPE configuration with a fixed `tcp-mss-set 1400` and proves only that clamp;
it does not exercise this action.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-mss-pmtu` | `test/interop-topology/scenarios/pppoe-mss-pmtu/` | accel-ppp as PPPoE access concentrator and router (image registered by the PPPoE suite), Linux TCP stacks on the LAN host and the server | AC-8: the route MTU of a PPP device negotiated with third-party software reaches the SYN as 1452; red walk: Ze's term removed, the 1460 SYN arrives as 1460 (this also rules out a clamp on the accel-ppp node, A-3) | |

IPv4 only: the IPv6 arithmetic (AC-2) is proven by `TestTCPMSSClampToPMTU`,
because IPv6 over the PPPoE session depends on child 8.

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang` - new leaf beside `tcp-mss-set`
- `internal/component/firewall/model.go`, `config.go`, `validate.go`
- `internal/plugins/firewall/nft/lower_linux.go`
- `internal/plugins/firewall/vpp/verify.go`
- `docs/guide/firewall.md`
- `docs/architecture/core-design.md` - declared by `model.go`, `config.go`, `validate.go` and `lower_linux.go`. Section 14b states the count of abstract expression types; the new action makes that count wrong, so the sentence is updated to the count the model holds after the change. Section 14a (the `ze:backend` gate) stays true
- `docs/architecture/firewall/fw-6-firewall-vpp.md` - declared by `internal/plugins/firewall/vpp/verify.go`; it describes the per-expression verifier without listing refused actions, so nothing becomes wrong
- `docs/architecture/testing/interop.md` - the topology suite's scenario inventory gains `pppoe-mss-pmtu`

## Files to Create
- `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci`
- `test/interop-topology/scenarios/pppoe-mss-pmtu/` - the Ze CPE, accel-ppp, LAN host and server configuration files
- `internal/le/interoplab/topology/check_mss_pmtu.go`, `check_mss_pmtu_test.go` - the scenario's registered checker and its verdict test

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
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` scenario inventory |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-9-mss-clamp-to-pmtu.md` |
| 17 | Existing examples? | Yes | firewall guide MSS example |

## Implementation Steps

1. **Phase: Wiring** - leaf parses to the new action and reaches `lowerAction`; the integration test fails (no lowering).
2. **Phase: Verify** - hook walk, exclusivity, family, VPP refusal (AC-4..AC-7).
3. **Phase: Lowering** - `rt` TCPMSS plus the shared `exthdr` write (AC-1..AC-3).
4. **Phase: Interop** - `pppoe-mss-pmtu` once child 18's topology suite exists (AC-8), red walk recorded.
5. **Phase: Docs** - guide entry, interop inventory, and the mapping row of child 10 (`clamp-mss-to-pmtu`).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | verify refuses every chain the kernel would refuse, so a commit never fails at netlink |
| Data flow | the `exthdr` write is shared with `lowerSetTCPMSS`, not duplicated |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Clamp to route MTU | `TestTCPMSSClampToPMTU` green on Ze's kernel; `pppoe-mss-pmtu` green |
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
| Own interop scenario `pppoe-mss-pmtu` | rely on child 11's `lan-services` | `lan-services` runs the converted configuration with a fixed `tcp-mss-set 1400`, so it never exercises the route MTU; the PMTU case needs a real PPP device and a term this spec owns |

## Known Limitations

- VPP backend: refused.
- A per-interface shorthand (VyOS `adjust-mss` under the interface) is not offered; the mapping guide (child 10) writes the firewall term.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Existing Mechanisms table names every mechanism this spec extends
- [ ] No code snippets
- [ ] D-1 answered by Thomas

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le job run label unit-pkg command go test -tags integration -count=1 ./internal/component/firewall/`; `./le test unit plugins`; `./le test integration firewall`; on Ze's kernel, `./le test qemu run kernel tmp/kernel/build/vmlinuz packages "nftables iproute2 libcap" timeout 1200s command 'go test -tags integration -count=1 -run TestTCPMSSClampToPMTU ./internal/plugins/firewall/nft/'`; `./le test firewall` for `firewall-tcp-mss-clamp-to-pmtu`; `./le test integration interop-topology pppoe-mss-pmtu`; then `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests: N-A, presence leaf
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: `pppoe-mss-pmtu`, with its red walk

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

## Implementation Summary

### What Was Implemented
- Landed in ba972d0bb6 (code, tests, docs) and d7bbba3daf (the `.ci` promoted). Both carry the trailer `Spec-work: home-cpe-1-logical-interface-name` by a claim mix-up; the subjects name child 9.
- YANG leaf `tcp-mss-clamp-to-pmtu` (`ze-firewall-conf.yang`, `ze:backend "nft"`); `ClampTCPMSSToPMTU` in `model.go`; parsed in `config.go` `parseThenBlock`; verified by `validate.go` `validateClampTCPMSSToPMTU` (family, exclusivity with `tcp-mss-set`, hook walk over jump and goto through `baseChainReaching` and `chainReaches`); lowered by `lower_linux.go` `lowerClampTCPMSSToPMTU` (`rt` TCPMSS, a byteorder hton, then the shared `tcpMSSWrite`); refused by `internal/plugins/firewall/vpp/verify.go` `verifyAction`; rendered by `cmd/show.go` and `web/page_firewall.go`.
- Scenario `pppoe-mss-pmtu`: `register_pppoe_mss_pmtu.go`, `check_mss_pmtu.go` `checkMSSPMTU` and `mssPMTUVerdict`.

### Bugs Found/Fixed
- Byte order: `nft_rt_eval` stores the route MSS in host order and `nft_exthdr_tcp_set_eval` reads a `__be16`, so without the hton a little-endian host never clamps. Covered by `TestTCPMSSClampToPMTU` and `TestLowerClampToPMTU`.

### Documentation Updates
- `docs/guide/firewall.md` actions table row and the PPPoE example; `docs/architecture/testing/interop.md` scenario inventory row `pppoe-mss-pmtu`; `features/firewall.md`; `docs/guide/migrating-from-vyos.md` mapping rows (line 176, conversion lines 10 and 11).
- At closure, `docs/architecture/core-design.md` section 14b: the count was 44 (23 match) and the model now holds 24 match types, because 9c92ef159f added a match without the page. Corrected to 45 (24 match, 13 action, 8 modifier).
- `./le doc check all`: red with 1851 command-availability drift rows, none on this spec's pages (log `tmp/session/2026-11-07-104251/scratch/c9-doccheck.log`).

### Deviations from Plan
- The interop red walk broke the lowering (`lowerAction` answers `nil, nil` for the action), not the term, and the scenario went red at its table check before any probe was sent. The planned "1460 arrives as 1460" observation was not made. A-3 is confirmed from accel-pppd's source instead, and the verdict half is `TestMSSPMTUVerdict`. The comment in `accel-ppp.conf` that claimed otherwise is corrected.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation commits carried another spec's `Spec-work` trailer and left this spec at `design` | the work was this spec's | closure read `git log` | recorded here; status moved through `ready` by `./le spec claim` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Term action clamps SYN MSS to route MTU less headers | Done | `lower_linux.go` `lowerClampTCPMSSToPMTU` | |
| 1460 to 1452 (IPv4) or 1432 (IPv6) at MTU 1492; smaller kept | Done | `integration_linux_test.go` `TestTCPMSSClampToPMTU` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTCPMSSClampToPMTU` | QEMU, Ze's kernel |
| AC-2 | Done | `TestTCPMSSClampToPMTU` | |
| AC-3 | Done | `TestTCPMSSClampToPMTU` | |
| AC-4 | Done | `TestValidateClampToPMTUHook`; `firewall-tcp-mss-clamp-to-pmtu.ci` | |
| AC-5 | Done | `TestValidateClampToPMTUHook` "with tcp-mss-set refuses"; `TestParseTCPMSSClampToPMTU` | |
| AC-6 | Done | `TestValidateClampToPMTUHook` arp, bridge, netdev | |
| AC-7 | Done | `TestVPPRefusesClampToPMTU` | |
| AC-8 | Done | scenario `pppoe-mss-pmtu` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParseTCPMSSClampToPMTU` | Done | `config_test.go` | |
| `TestValidateClampToPMTUHook` | Done | `validate_test.go` | |
| `TestLowerClampToPMTU` | Done | `lower_linux_test.go` | |
| `TestVPPRefusesClampToPMTU` | Done | `internal/plugins/firewall/vpp/verify_test.go` | |
| `TestMSSPMTUVerdict` | Done | `check_mss_pmtu_test.go` | |
| `TestTCPMSSClampToPMTU` | Done | `integration_linux_test.go` | |
| `firewall-tcp-mss-clamp-to-pmtu` | Done | `test/firewall/` | |
| `pppoe-mss-pmtu` | Done | `test/interop-topology/scenarios/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify and Files to Create | Done | all in ba972d0bb6 and d7bbba3daf; `fw-6-firewall-vpp.md` needed no change, as planned |

### Audit Summary
- **Total items:** 20
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the interop red walk, in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A forwarded SYN is clamped to the route MTU less the fixed headers, never raised | kernel integration test | `TestTCPMSSClampToPMTU` PASS in QEMU on Ze's kernel 2026-11-08 10:49 (`rwinv/out-c2-12-qemu/clean-c9.log`); red with the lowering broken: 1460 arrives as 1460 in all four cases (`c9-no-mss-clamp.log`) |
| The same through the daemon from config | functional `.ci` | `firewall-tcp-mss-clamp-to-pmtu` PASS (`rwinv/out-c2-12-netns/clean-guest.log` line 215); red with the lowering broken: the installed table lacks `tcp option maxseg size set rt mtu` (`c9-no-mss-clamp-guest.log` line 38) |
| A PPP MTU negotiated with third-party software reaches the SYN | interop | `pppoe-mss-pmtu` against accel-ppp: pass 2026-11-08 12:20 (`rwinv/out-c2-12-interop/rerun-pppoe-mss.log`); red 12:16 with the lowering broken (`c9-pppoe-mss-broken.log`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC demonstrated | `plan/release/fixes/spec-home-cpe-9-mss-clamp-to-pmtu.md` (this spec) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-9-mss-clamp-to-pmtu-7e292f27-ced0-427f-a7b6-3a931db211f9.md` (28 files, verdict clean) |
| `./le spec review check` | run by `./le commit create` on commit B |
| Rounds | 1 |
| Reviewer lenses used | wiring (every `SetTCPMSS` switch has its `ClampTCPMSSToPMTU` arm: show, web, validate, nft, vpp), guard audit (hook walk fails closed on an unknown hook, bounded BFS with a visited set, cycle test), kernel contract (`nft_rt_validate` hook mask), style, docs |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE. NOTEs: the `accel-ppp.conf` comment claimed a red walk that was not observed (corrected); core-design count stale from another commit (corrected) | | |

Pre-checks: `./le test weakened check base ba972d0bb6~1` clean; `./le repo check` 3 issues, all in other specs' topology checkers (`check_access_verdict.go`, `check_pppoe_dhcpv6_pd*.go`).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci` | yes | ls, 2.7K |
| `test/interop-topology/scenarios/pppoe-mss-pmtu/` | yes | `accel-ppp.conf`, `ze-cpe.conf` |
| `internal/le/interoplab/topology/check_mss_pmtu.go`, `check_mss_pmtu_test.go`, `register_pppoe_mss_pmtu.go` | yes | ls |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-4..AC-7 | refusals | 2026-11-08 closure run: `TestParseTCPMSSClampToPMTU`, `TestValidateClampToPMTUHook`, `TestVPPRefusesClampToPMTU`, `TestMSSPMTUVerdict` PASS (`go test -race` over firewall, vpp, topology) |
| AC-1..AC-3, AC-8 | clamp | the 2026-11-08 runs above. `lower_linux.go` differs from the walk's overlay by the one broken line only; no commit touched `internal/plugins/firewall/nft/` or `internal/component/firewall/` since 2026-11-08T10:00; 9167d499fd changed topology only for `convertedCPE` (CAP_BPF) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| leaf in a forward-chain term through the daemon | `firewall-tcp-mss-clamp-to-pmtu.ci` | read: seq 1 refuses the prerouting placement naming table, chain, term, base chain and fix; seq 3 lists `tcp option maxseg size set rt mtu` in the installed table |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `checkMSSPMTU` requires `mtu 1492` on `pppoe0` and the server captured 1452 |
| A-2 | confirmed | `TestTCPMSSClampToPMTU` ingress MTU 1400 cases give 1360 and 1340 |
| A-3 | confirmed | `grep -rni mss accel-pppd` in the accel-ppp source (06f64b1) finds nothing; `accel-ppp.conf` sets no clamp |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| guide row and example | `validate.go` refusal text and hooks; YANG description | yes |
| core-design 14b count | `model.go`: 24 `matchMarker`, 21 `actionMarker` (13 action, 8 modifier) | yes |
| RFC status | no tags, no `rfc/short/` summary for RFC 6691 or 9293; `./le rfc check` names neither | N-A |
