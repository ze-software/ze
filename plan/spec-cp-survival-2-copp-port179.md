# Spec: Control-Plane Policing for BGP (Gap B)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cp-survival-0-umbrella |
| Phase | 7/7 |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/firewall/model.go` (Limit action, HookInput, DestinationPort)
4. `internal/plugins/policyroute/register.go` (precedent: policy plugin → firewall.RegisterTables)
5. `internal/component/firewall/yang/ze-firewall-conf.yang` (input hook + limit-rate)

## Task

Provide a turnkey **control-plane policing (CoPP)** construct that protects host-bound traffic to
the BGP listen port (TCP/179) so a DDoS aimed at the router's own address cannot saturate the CPU
and starve BGP processing — the failure mode that drops keepalives and prevents a FlowSpec/RTBH
signal from being sent.

The firewall already has every building block (input-hook chains, `Limit` action, destination-port
match, nft backend, YANG `rate-spec`). What is missing is a **safe, opinionated construct** so
operators get CoPP without hand-rolling an input chain — and without the lock-out risk of doing it
wrong. This spec adds a `copp` system plugin (mirroring how `policyroute` lowers policy to firewall
tables) that generates a safe input chain: trusted peers and established sessions pass at full rate;
unknown new TCP/179 connections are rate-limited.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - CoPP is domain policy over the firewall datapath
  → Constraint: implement as a plugin (`internal/plugins/copp`) that calls `firewall.RegisterTables`; removing the plugin removes all CoPP. No "copp" spelling in firewall/core.
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` - new config
  → Decision: CoPP is operational policy → YANG config, kebab-case leaves.
- [ ] `ai/rules/doctor-checks.md` - new listen-path dependency
  → Constraint: if CoPP introduces a runtime dependency (nft table presence), add a doctor check.

**Key insights:**
- The dangerous mistake is rate-limiting your *real* peers. The generated chain must exempt
  established/related connections and an operator-supplied trusted-source prefix list; only
  unknown new connections to 179 are policed.
- `policyroute` is the exact precedent: a system plugin that translates its own config into
  `firewall` tables/terms via `RegisterTables` and an nft backend — copy its shape.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go:106-159` - `ChainHook` enum incl. `HookInput` (line 111) = locally-destined traffic.
  → Constraint: CoPP uses `HookInput`; base chain needs Type/Hook/Policy/Priority set (validated 651-672).
- [ ] `internal/component/firewall/model.go:294-307,326-327` - `MatchSourcePort`/`MatchDestinationPort` (`Ranges []PortRange`), `MatchProtocol`, `MatchConnState`.
  → Constraint: match `tcp dport 179`, `ct state established,related`, and source-prefix for trusted peers.
- [ ] `internal/component/firewall/model.go:524-530` - `Limit{Rate, Unit, Dimension, Over, Burst}`.
  → Constraint: rate-limit new connections; `Dimension` must be set (packets) or lowering errors.
- [ ] `internal/plugins/firewall/nft/lower_linux.go:103-121,604-625` - `lowerHook` (HookInput→ChainHookInput) and `lowerLimit` (expr.Limit). 
  → Constraint: backend already lowers everything CoPP needs; no nft backend changes.
- [ ] `internal/plugins/policyroute/register.go:181-218` - translates config → `firewall.RegisterTables()` + ip rules.
  → Constraint: copp mirrors this; registers tables only (no ip rules).
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang:38-49,169-171,388-399` - `chain-hook` enum incl `input`, `destination-port`, `limit-rate{rate,burst}` with `rate-spec` pattern.
  → Constraint: CoPP YANG is a thin policy schema; the heavy firewall schema is reused via generated tables, not duplicated.
- [ ] `internal/plugins/host/host.go` + `internal/component/host` - hardware inventory only; NO existing control-plane protection.
  → Constraint: CoPP is net-new; nothing to extend or replace.

**Behavior to preserve:**
- Operator-authored firewall config keeps working unchanged. CoPP adds a separate, named table; it does not modify user tables.
- No CoPP config ⇒ no CoPP table installed.

**Behavior to change:** Add opt-in CoPP. When enabled, a managed input chain is installed.

## Data Flow (MANDATORY)

### Entry Point
- Config `control-plane-protection { bgp { rate; burst; trusted-source [..]; policy } }`.

### Transformation Path
1. `copp` plugin parses its config → an internal `CoppPolicy` (port set, rate, burst, trusted prefixes, over-policy).
2. Translate to `firewall.Table` with one `HookInput` chain and ordered terms:
   a. `ct state established,related → accept`
   b. `tcp dport 179 source-in <trusted> → accept`
   c. `tcp dport 179 ct state new → limit-rate → accept` (over-limit falls through)
   d. chain policy (drop or accept per config; default: continue/accept to avoid lock-out)
3. `firewall.RegisterTables(tables)` → nft backend lowers to a real input chain.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| copp config ↔ copp plugin | YANG → CoppPolicy | [ ] |
| copp plugin ↔ firewall | `firewall.RegisterTables` | [ ] |
| firewall ↔ kernel | nft `lower_linux.go` (existing) | [ ] |

### Integration Points
- `internal/component/firewall` `RegisterTables` — copp registers its managed table here
- `internal/plugins/firewall/nft/lower_linux.go` — existing lowering, unchanged
- `internal/component/bgp` listen port — default protected port (179); if a non-default listen port is configured, CoPP protects it too (config leaf, not BGP coupling)

### Architectural Verification
- [ ] No bypassed layers (copp → firewall → nft, same as policyroute)
- [ ] No unintended coupling (copp does not import bgp internals; trusted peers via explicit config)
- [ ] No duplicated functionality (reuses firewall Limit/HookInput; no new nft code)
- [ ] Zero-copy preserved (N/A — control-plane config)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `firewall.RegisterTables` accepts a fully-formed base input chain from a plugin and the nft backend applies it | policyroute uses it (register.go:181-218) | copp can't install rules | grep RegisterTables signature + policyroute test | confirmed |
| A-2 | An input-hook table installed by copp coexists with operator firewall tables without conflict | nft multi-table model; firewall component owns table ownership | rule collisions | registry.go mergeSameNameTables: unique table name = no collision | confirmed |
| A-3 | Rate-limiting only ct-state-new TCP/179 (not established) is sufficient CoPP for connection-flood; volumetric data-plane floods are gap C/upstream's job | CoPP scope = host-bound connection setup | under-protection claim | doc the boundary; not a code bug | confirmed (design boundary) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | CoPP chain drops the operator's live BGP/SSH on apply (lock-out) | sessions reset at commit | first term accepts established,related; default chain policy is non-drop unless explicitly set; document commit-confirm |
| R-2 | Trusted-source list omitted ⇒ a busy legit peer reconnecting rapidly gets rate-limited | peer flaps post-apply | established-accept covers steady state; doc strongly recommends trusted-source = peer addresses |
| R-3 | Over-limit policy `drop` plus a NAT'd peer behind shared IP starves that peer | one peer can't connect | per-source not global limit where possible; doc the NAT caveat |
| R-4 | CoPP table priority collides with a user input chain at the same hook priority | undefined order | copp uses a documented fixed priority; validate against user tables; doctor check |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `control-plane-protection { bgp { rate "100/second" } }` | → | copp parse → translate → `firewall.RegisterTables` → nft input chain | `copp-bgp.ci` (nft readback shows limit on dport 179) |
| trusted-source prefix configured | → | accept term before limit term | `copp-trusted.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `control-plane-protection { bgp { rate "100/second" burst 20 } }` | An nft input-hook chain exists with `tcp dport 179 ... limit rate 100/second burst 20 ... accept` |
| AC-2 | Above config applied while a BGP session is established | The established session is NOT reset (first term accepts established,related) |
| AC-3 | `trusted-source [192.0.2.0/24]` configured | A term accepts `tcp dport 179` from 192.0.2.0/24 at full rate, ordered before the limit term |
| AC-4 | No `control-plane-protection` config | No copp nft table is installed (regression guard) |
| AC-5 | copp config removed | The copp-managed table is withdrawn cleanly; operator firewall tables untouched |
| AC-6 | Non-default BGP listen port configured in copp (`protected-port 1790`) | The generated chain protects 1790 instead of/in addition to 179 |
| AC-7 | `ze doctor` after enabling copp | Doctor check confirms the copp input chain is present in the datapath |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables CoPP and a flood of new TCP/179 from random sources is rate-limited while the real peer keeps its session | config → copp → firewall → nft input chain | `copp-bgp.ci` (nft readback) + a netns flood test asserting peer survives |
| 2 | lists trusted peer prefixes so they bypass the limit | trusted-source term ordering | `copp-trusted.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCoppParseConfig` | `internal/plugins/copp/config_test.go` | YANG → CoppPolicy | |
| `TestCoppTranslateOrder` | `internal/plugins/copp/translate_test.go` | term order: established → trusted → limit | |
| `TestCoppTableWithdraw` | `internal/plugins/copp/register_test.go` | removing config withdraws the table | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rate (per rate-spec) | ≥1/unit | 1/second | 0 | N/A (uint64) |
| burst | 0-uint32 | 4294967295 | N/A | N/A |
| protected-port | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `copp-bgp` | `test/firewall/copp-bgp.ci` | enable CoPP; nft list shows limit on tcp dport 179 | |
| `copp-trusted` | `test/firewall/copp-trusted.ci` | trusted-source accepted before limit term | |
| `copp-withdraw` | `test/firewall/copp-withdraw.ci` | removing config drops the table, user tables intact | |
| `TestCoppFloodNetns` | `internal/plugins/firewall/nft/copp_integration_linux_test.go` | netns: new-conn flood limited, established survives (QEMU) | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | CoPP is host-local datapath, not a wire protocol; no interop needed | |

## Files to Modify
- `internal/component/plugin/all/all.go` - generated registry (via `make generate`) to include copp
- `docs/features.md` - CoPP feature row
- `docs/guide/command-reference.md` / config guide - CoPP config syntax

## Files to Create
- `internal/plugins/copp/register.go` - plugin registration, config apply → `firewall.RegisterTables`
- `internal/plugins/copp/config.go` - parse `control-plane-protection` config → CoppPolicy
- `internal/plugins/copp/translate.go` - CoppPolicy → `firewall.Table` (ordered terms)
- `internal/plugins/copp/model.go` - CoppPolicy type
- `internal/plugins/copp/doctor.go` - doctor check: copp input chain present
- `internal/plugins/copp/yang/ze-copp-conf.yang` - config schema (rate, burst, trusted-source, protected-port, over-policy)
- `internal/plugins/copp/*_test.go` - unit tests
- `test/firewall/copp-bgp.ci`, `test/firewall/copp-trusted.ci`, `test/firewall/copp-withdraw.ci` - functional tests
- `internal/plugins/firewall/nft/copp_integration_linux_test.go` - netns flood test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create, TDD plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — copp plugin skeleton + YANG + registration; empty translate; failing `copp-bgp.ci`.
2. **Phase: translate** — CoppPolicy → firewall.Table with ordered terms; AC-1, AC-3 (ordering).
3. **Phase: lifecycle** — apply/withdraw via RegisterTables; AC-4, AC-5.
4. **Phase: safety** — established-accept first; default non-drop policy; AC-2; lock-out guard.
5. **Phase: port + doctor** — protected-port leaf; doctor check; AC-6, AC-7.
6. **Phase: tests** — netns flood integration; functional .ci.
7. **Full verification** → `make ze-verify-changed` + `make generate` (registry).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC-N has file:line |
| Correctness | term order (established → trusted → limit); Limit.Dimension set |
| Safety | apply never drops established sessions; default policy non-drop |
| Rule: plugin-self-containment | no "copp" string in firewall/core; removal clean |
| Doctor checks | copp chain presence check registered |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Lock-out | cannot strand the operator; established accepted |
| Input validation | port/rate/prefix bounds; reject malformed trusted-source |
| Resource exhaustion | the limit itself is the DoS protection; ensure burst/rate sane defaults |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit
(Filled at closure.)

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/copp`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-2-copp-port179.md`
