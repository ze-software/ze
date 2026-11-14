# Spec: numeric-bound-authority-declaration

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision of 2026-11-14 ("Keep half (a) as a release fix, rebuilt on the host-verdict mechanism. Move half (b) post-release, as opt-in for host-owned leaves."). It prevents the defect class; the three known instances are fixed before release by `plan/release/fixes/spec-config-bound-authority.md` |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-config-bound-authority.md` (the host checks a `host` declaration would point at); `plan/release/fixes/spec-yang-single-source-of-truth.md` (it reshapes how a leaf's type is derived and walks the same leaves, so the declaration and its gate are designed on top of that derivation, never beside it) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

This spec is half (b) of the 2026-09-08 design of
`plan/release/fixes/spec-config-bound-authority.md`, split out on 2026-11-14.
It needs a design pass with Thomas before it leaves `skeleton`: the shape is
open (the decisions table below), and his stated lean is recorded, not ruled.

## Task

A numeric config leaf carries a bound, and the bound has an owner. When the
owner is a published standard the bound is safe to write down: a twelve-bit
restart time is 0 to 4095 forever. When the owner is a driver, a kernel or VPP,
the written bound is a COPY of a fact that machine answers for itself, and the
copy is wrong on the next machine.

VyOS shipped `vyos/vyos-1x#5448` on this defect. Its CLI refused a VPP ring
descriptor count of 32768 because two validators held `256-16384` for RX and
`256-8192` for TX, while VPP itself reported `desc 16384 (min 0 max 32768 align
1)` for the Mellanox ConnectX card in the machine. Configuring hardware the
daemon could already drive needed a code change and a release. Ze held the same
shape in the other polarity (ring sizes, DPDK queue counts, interface MTU), and
the release fix repairs those three.

The goal of this spec: Ze cannot hold the class silently. A leaf whose bound the
host owns says so in the schema, and a `./le` gate refuses the shapes that make
a copied bound possible. Nothing runs in the daemon for it.

## Decisions for Thomas

Open. The spec stays `skeleton` until they are answered.

| ID | Decision | Options | What each costs and misses |
|----|----------|---------|----------------------------|
| D-1 | The declaration's shape. Thomas's stated lean on 2026-11-14: "opt-in for host-owned leaves" | S-1, mandatory on every numeric leaf: `ze:authority` with a closed token set `standard`, `policy`, `host`, and the gate refuses a leaf that declares nothing. S-2, opt-in `host`: only a host-owned leaf declares, with two structural rules the gate enforces (a `host` leaf carries the runtime check that judges it, and its static ceiling is its type's maximum). S-3, a heuristic gate with a recorded exception set: no declaration; the gate flags a numeric leaf whose text names a host tell (sysctl, procfs, sysfs, netlink, ethtool, driver, kernel, NIC, VPP, hardware, hugepage, device) and a reviewed exception list records each leaf judged not host-bounded | S-1: about 658 one-line additions (recount owed), touching roughly 90 modules other sessions edit; an absent declaration can never read as a safe answer, which is the zero-value-as-valid-answer rule of `ai/rules/principles.md`. Misses a leaf that declares `policy` over a bound the host owns. S-2: a few lines; misses every host-owned leaf nobody thought to mark, which is the VyOS case exactly, because the absent statement reads as "not host". S-3: no schema change; misses a host leaf whose text names no tell, and the exception list is a second declaration that drifts |
| D-2 | What a `host` declaration points at, now that the release fix judges host bounds in the owners' in-process verifiers, not through `ze:validate` | The gate reads the registration that owns the leaf's root and asks whether its verifier declares the leaf; or the declaration names the check; or the gate does not check the pairing at all | The 2026-09-08 rule "a `host` leaf carries `ze:validate`" no longer fits the release fix's seam; this must be redesigned against what that spec lands |
| D-3 | Whether a `host` leaf's editor completion offers the host's maximum | A completion hook fed by the same host read; or none | Old R-3: widening a ceiling to the type maximum loses the hint the old ceiling gave |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - the YANG system, the `ze:` extensions, the custom validator seam, and "Layer 3: Plugin config verifiers"
  → Constraint: the release fix judges host bounds in the in-process verifier, which "does not run at daemon startup"; a declaration that points at `ze:validate` points at the wrong seam (D-2).
- [ ] `docs/architecture/doctor-and-health-checks.md` - kernelcap's three verdict states
  → Decision: an unreadable host fact never refuses; a declaration changes nothing about that.
- [ ] `plan/release/fixes/spec-config-bound-authority.md` - the three host checks this spec's `host` declaration names
  → Constraint: design after that spec's D-1 and D-2 are answered.
- [ ] `plan/release/fixes/spec-yang-single-source-of-truth.md` - how a leaf's type is derived
  → Constraint: the gate reads the derived type, never a parse of its own.

**Key insights:**
- The bound and its owner are two facts. Ze writes the first and has never written the second.
- The prevention half is one YANG extension (or none, under S-3) and one `le` gate. Nothing runs in the daemon for it.

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-14, for the split; the design pass re-reads them)
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - 35 `ze:` extensions; `validate` "References a custom validator function for runtime validation"
- [ ] `internal/component/config/validators_register.go` - `RegisterValidators` holds 29 registrations
- [ ] `internal/le/config/claims/configclaims.go` - `Audit` loads the schema with `configyang.NewLoader` and walks it
- [ ] `internal/le/config/ports/portdefaults.go` - `Check` compares the Go listener table with the YANG refine defaults (moved from `internal/le/portdefaults`)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `arp-ignore` is `uint8`, `range "0..2"`, default 0, describing levels 0 to 2 only
- [ ] `internal/component/bgp/reactor/session_connection.go` - `socketRecvBufSize` is a Go constant, not a YANG leaf

The population below was measured on 2026-09-08 and must be recounted.

**The population, counted on 2026-09-08, NEEDS A RECOUNT:**

| Measure | Count | How it was derived |
|---------|-------|--------------------|
| `range` statements under `internal/` | 343 | grep for the statement across every `.yang` file |
| Files carrying at least one | 60 | the same grep, listing files |
| On a `leaf` / on a `leaf-list` / on a `typedef` | 337 / 2 / 4 | a brace-counted block walk over each declaration |
| Numeric leaves and leaf-lists in the tree | 564 | leaves whose type is an integer, `decimal64`, or `zt:decimal-2` |
| Of those, carrying a `range` | 339 | |
| Of those, carrying NO bound at all | 225 | the larger under-declaration, and where the VPP queue leaves sat |
| Numeric leaves in a `*-conf.yang` module | 429 | the operator-writable half |
| Numeric leaves in a command or API module | 135 | command arguments an operator also types |
| Of the 343, whose own text names a host tell | 66 | leaf block matching the tell words in D-1 S-3 |
| Of those 66, a true host authority after reading each one | 3 | `ring rx`, `ring tx`, `interface mtu` |
| Host-authority leaves with NO range, found outside the 343 | 2 | VPP `rx-queues`, `tx-queues` |

On 2026-11-14 a rough grep found 658 numeric `type` statements (leaves,
typedefs and command arguments together), 29 validator registrations and 35
`ze:` extensions; the 2026-09-08 figures were 25 and 30. The recount owes the
same method as the table, and the spec quotes its own numbers.

The 63 of 66 that fell out are host-ADJACENT and not host-BOUNDED (2026-09-08
reading, to re-verify): the conntrack timeouts (`1..604800`, 30 leaves) name a
sysctl Ze writes, and the kernel imposes no smaller ceiling, so the seven-day cap
is Ze's own policy. `table-size` and `hash-size` write `nf_conntrack_max` and
`nf_conntrack_buckets`, which the kernel accepts at any `uint32`, so those
bounds are memory policy. `accept-ra`, `arp-announce` and `arp-ignore` copy a
kernel value SET rather than a device limit. The BGP `read-buffer-size` and
write buffer leaves size a userspace buffered reader and writer, not the kernel
socket buffer (`socketRecvBufSize` in
`internal/component/bgp/reactor/session_connection.go` is a separate Go
constant), so their authority is `policy`.

## Existing Mechanisms (MANDATORY)

A skeleton may leave the placeholder; the rows below are the starting point the
design pass extends.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `ze:` extensions | `internal/component/config/yang/modules/ze-extensions.yang` (35 extensions) | Ze's schema annotations, `validate` and `help` among them | Extends under S-1 or S-2: one new extension, uniform with the rest |
| Schema-walking gate | `internal/le/config/claims/configclaims.go` `Audit` | Loads the schema with `configyang.NewLoader` and walks it, with a recorded exception set in its report | Extends: the structural model for the new gate |
| Copy-against-source gate | `internal/le/config/ports/portdefaults.go` `Check` | Compares a hand-written Go table with the YANG that restates it | Extends: the precedent for "a copy names its source and a check compares them" |
| Release host checks | `plan/release/fixes/spec-config-bound-authority.md` | Judge three host-owned bounds in the owners' in-process verifiers | Extends: a `host` declaration names a leaf one of those checks judges (D-2) |
| Type derivation | `plan/release/fixes/spec-yang-single-source-of-truth.md` | Reshapes how a leaf's type and bounds are derived for every consumer | Extends, after it lands: the gate reads the derived type, never a second parse |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer adds or edits a numeric leaf in a `.yang` file and runs `./le config authority check`.

### Transformation Path
1. The gate loads the schema with `configyang.NewLoader`, as the claims gate does.
2. It walks every numeric leaf the chosen shape covers (D-1) and judges its declaration.
3. It answers through the shared renderer, with each refusal naming the module, the leaf path and the fix.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `le` gate ↔ schema | `configyang.NewLoader`, the loader the claims gate uses | No |

### Integration Points
- `internal/le/register.go` - one blank import registers the gate.
- `internal/component/config/yang/modules/ze-extensions.yang` - the extension, under S-1 or S-2.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-4 | `arp-ignore` `range "0..2"` is narrower than the value set the kernel accepts, which the kernel documents as 0 to 8 | `internal/component/iface/yang/ze-iface-conf.yang` declares 0..2 with a description of levels 0, 1 and 2; the kernel's `Documentation/networking/ip-sysctl.rst`, `arp_ignore`, is the counter-claim. UNVERIFIED: no kernel documentation is on the development machine, and the text has never been read in this repository | Ze refuses a legal ARP policy an operator asks for, which is the VyOS defect in Ze's own tree. If it holds, it is a defect in an implemented capability, owed a release fix of its own, not post-release work | Read `arp_ignore` in `Documentation/networking/ip-sysctl.rst` of Ze's kernel source, and the kernel's handling in `arp_ignore()` in `net/ipv4/arp.c`; then classify the leaf, and if it is narrow open the fix | unvalidated |
| A-5 | Every numeric leaf can be classified `standard`, `policy` or `host` with no fourth case (S-1 only) | The 66-leaf hand read on 2026-09-08 produced no residue | A fourth token is needed, and the closed vocabulary grows | The classification pass itself: any leaf that resists the three names is reported before its module is annotated | unvalidated |
| A-6 | The schema loader answers `config false` for a node, so the gate can scope its population without reading file names | `internal/le/config/claims/configclaims.go` walks the loaded tree | The population has to be derived from module file names, a second declaration of what the schema says | Read the loader's node type in the design pass | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-2 | The classification pass (S-1) touches roughly 90 YANG files, several owned by components another session is editing, so the annotation commit collides | A merge conflict, or a session reporting hunks it did not write | One line per leaf, applied module by module, never one commit; a module another session holds is skipped and named. On 2026-09-08 `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` carried 14 ranges and sat under a live IKE session, so it went last |
| R-3 | A `host` leaf whose ceiling is widened to the type maximum loses the completion hint the old ceiling gave | Completion offers an absurd value | D-3: completion fed by the host read offers the real maximum, a better hint than the constant it replaces |
| R-4 | The gate refuses the whole tree until every leaf is annotated (S-1), blocking every other session | The gate goes red repository-wide | The gate lands green: the annotation pass completes before the gate is registered in `internal/le/register.go` |
| R-7 | The declaration is a claim a developer types, so a leaf can declare `policy` over a bound the host owns | Nothing mechanical sees it | The gate cannot decide this, and the spec says so. What it removes is the SILENT case under S-1; under S-2 it does not remove even that |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at run time: the declaration is inert in the daemon. A wrong gate refuses developer commits |
| How is it reverted? | Single commit revert; the gate is registered by one blank import |
| Who else touches this path? | Any session editing a `*-conf.yang` module during an annotation pass (R-2), and `spec-yang-single-source-of-truth` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le config authority check` | → | the gate's `Answer` | `TestAuthorityGateIsRegisteredUnderConfig` |
| A `.yang` numeric leaf the chosen shape covers, with no declaration | → | the gate's schema walk | `TestAuthorityGateRefusesAnUndeclaredNumericLeaf` |

## Acceptance Criteria (candidates)

The 2026-09-08 criteria, preserved as candidates. Each is re-judged in the
design pass against D-1 and D-2; none is agreed.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A numeric leaf in a loaded YANG module carries no `ze:authority` statement (S-1) | `./le config authority check` fails and names the module, the leaf path and the missing statement | A non-numeric leaf, which has no bound to own |
| AC-2 | A `ze:authority` whose first token is not `standard`, `policy` or `host` | The check fails and prints the three tokens it accepts | The three tokens |
| AC-3 | `ze:authority "standard"` with no citation text after the token | The check fails, because a standard authority owes the reader which standard | A `standard` with its citation |
| AC-4 | A leaf declares `ze:authority "host"` and carries no runtime check that judges it (2026-09-08 wording: no `ze:validate` on the same leaf; re-judged under D-2) | The check fails and names the leaf: a host-owned bound with no runtime check declares an owner and enforces nothing | A `host` leaf whose check exists |
| AC-5 | A leaf declares `ze:authority "host"` and its static `range` upper bound is below the maximum its YANG type can carry | The check fails and prints both numbers: a ceiling narrower than the type is a copy of a host fact, the VyOS defect stated mechanically | A lower bound stays, because it is usually a real standard fact (68 for an IPv4 MTU) |
| AC-6 | `./le config authority check` runs over the committed tree | It passes, so every leaf the chosen shape covers carries a declaration | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthorityGateIsRegisteredUnderConfig` | `internal/le/config/authority/authority_test.go` | the gate is reachable | |
| `TestAuthorityGateRefusesAnUndeclaredNumericLeaf` | same | AC-1 over a synthetic module | |
| `TestAuthorityGateRefusesAnUnknownToken` | same | AC-2 | |
| `TestAuthorityGateRefusesAStandardWithNoCitation` | same | AC-3 | |
| `TestAuthorityGateRefusesAHostLeafWithNoCheck` | same | AC-4 | |
| `TestAuthorityGateRefusesAHostCeilingBelowTheTypeMaximum` | same | AC-5, the VyOS shape | |
| `TestAuthorityGatePassesOverTheCommittedTree` | same | AC-6 | |
| `TestAuthorityGateIgnoresANonNumericLeaf` | same | the population boundary | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the gate's selftest | `./le config authority selftest` | each refusal has a synthetic case | |

### Interop Tests (Scope: protocol)
Not applicable: a development gate with no protocol peer.

## Files to Modify
- `internal/component/config/yang/modules/ze-extensions.yang` - the `authority` extension (S-1, S-2)
- `internal/le/register.go` - the blank import for the gate
- every `.yang` module holding a covered numeric leaf - one declaration line per leaf (S-1), or per host leaf (S-2)
- `docs/architecture/config/yang-config-design.md` - the extension, its tokens, and what the gate refuses
- `docs/architecture/core-design.md` - the new gate under `le`

## Files to Create
- `internal/le/config/authority/authority.go`, `register.go`, `report.go` - the schema walk, the refusals, the report shape, following the claims gate

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | one extension; no config leaf added |
| CLI commands/flags | Yes | `./le config authority check` and `selftest` |
| Doctor check for runtime dependencies | N-A | nothing runs in the daemon |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/rules/repo-maintenance.md`, `ai/INDEX.md` for the new gate |

## Known Limitations

- The declaration is a claim a developer writes. A leaf that declares `policy` over a bound the host really owns passes the gate.
- The conntrack timeouts, `table-size` and `hash-size` read as `policy`, so their seven-day and two-million ceilings stay; widening them is a separate question about what an operator should be able to ask for.
- `accept-ra`, `arp-announce` and `arp-ignore` copy a kernel value set rather than a device limit, so they read as `standard` citing the kernel ABI, once A-4 is settled.

## Key Design Decisions (2026-09-08, to re-judge)

| Decision | Alternatives Considered | Rationale (as written then) |
|----------|------------------------|-----------------------------|
| Declare the authority on the LEAF | Derive the population from the APPLIERS, the files that write to procfs, sysfs, device nodes and ethtool | The applier population is not enumerable: the fs-persistence gate's control-file list held 13 kernel and device files and covered neither the netlink MTU write nor any future one. The leaf population is enumerable from the schema loader |
| Mandatory declaration on every numeric leaf | Opt-in: declare only `standard` and `host`, and read an absent statement as `policy` | An absent statement reading as a safe answer is the zero-value-as-valid-answer defect; a forgotten host leaf would be invisible, which is the VyOS failure. Cost: 564 one-line additions. Thomas's 2026-11-14 lean is the opt-in shape, so this row is D-1 |
| A `host` leaf's static ceiling must equal its type maximum | Delete the `range` entirely on a host leaf | The lower bound is usually a real standard fact, so only the ceiling is the copy |
| The gate refuses a `host` leaf with no `ze:validate` | Let the gate resolve the Go function by name with `go/ast` | The schema-local rule is simpler and the Go side is closed by `CheckAllValidatorsRegistered`. Superseded in part: the release fix does not use `ze:validate` (D-2) |
| Enforce through `ze:validate` | A new `hostlimit` package modelled on `kernelcap` | Kept here as history: the release fix found `ze:validate` runs in `LoadConfig` and chose the in-process verifier with `diagnostic.HostVerdict` |

Design insight carried over: the gate cannot find a leaf that declares `policy`
over a host-owned bound. It removes the silent case, not the wrong-answer case.

## Implementation Steps (2026-09-08 phases, to re-plan)

1. Wiring: the gate exists, is registered, and refuses a synthetic module; it is not yet run over the tree.
2. The refusals: token vocabulary, citation rule, runtime-check rule, ceiling rule, each with a synthetic case in a selftest.
3. The classification pass (S-1) or the marking pass (S-2), module by module, reporting any leaf that resists the vocabulary before its module is annotated; the gate joins the checked population only once it passes over the tree.

Evidence for each phase is scoped: `./le test unit pkg path ./internal/le/config/<gate>/` and the gate's own selftest. No `./le verify` command.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] The population recounted, with the method stated
- [ ] D-1, D-2 and D-3 answered by Thomas
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] The agreed acceptance criteria all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread. No agent runs any `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-numeric-bound-authority-declaration.md` only, in the same `./le commit create` script
