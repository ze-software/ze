# Spec: which protocols reach the FIB is per-protocol and operator-settable

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | spec-connected-static-reach-the-locrib (the Loc-RIB producers and the single FIB writer) |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner directive, 2026-09-06: "every backend should come with a default of fib
kernel which can be changed per protocol (ie: to not import BGP routes into the
kernel but import OSPF and not ISIS)."

The first half is built and committed: a route producer declares
`Registration.NeedsDataPlane`, a FIB plugin declares the data plane it programs,
and the engine loads the writer for the data plane `interface { backend }`
selects, so a config with static routes and no `fib { }` block programs them.

The second half has no surface at all. Nothing in Ze lets an operator say which
PROTOCOLS reach the data plane. Every path the system RIB selects is published
to the FIB plugin, whatever produced it, so an operator who wants OSPF in the
kernel and BGP out of it has no way to say so and no way to see that they
cannot.

Goal: an operator names, per protocol, whether that protocol's routes are
written to the FIB; the default is that they are; and the vocabulary derives
from the protocol registry rather than from a hand-written list.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the cross-protocol RIB and the
      publish path this filter would sit on
- [ ] `docs/architecture/rib/unified-locrib.md` - `Path.Source` is a
      `redistevents.ProtocolID` and half of `Path.key`, so the system RIB
      already knows which protocol produced every path it selects
- [ ] `docs/architecture/config/yang-config-design.md` - the YANG config
      surface this spec adds a leaf-list to, and where `ze:validate` and
      `ze:help` are declared
- [ ] `docs/guide/redistribution.md` - the `redistribute { import }`
      vocabulary, which is the OTHER place in Ze that keys a decision on a
      protocol name
- [ ] `plan/immediate/spec-connected-static-reach-the-locrib.md` - the writer
      resolution this builds on, and the "OS-installed winner produces a
      withdraw" branch that already suppresses one class of FIB write

### RFC Summaries (Scope: config)
- [ ] N-A. No RFC governs which protocols an implementation programs into its
      forwarding table.

**Key insights:**
- Three surfaces key on a protocol name, and they are three STAGES of one
  pipeline rather than three copies of one decision (owner, 2026-09-06):
  selection asks which candidate path for a prefix WINS (`rib { distance { } }`
  feeding `selectBest`), redistribution asks which routes protocol A OFFERS to
  protocol B (`ImportRule`), and FIB programming asks which of the routes that
  WON are WRITTEN. The third does not exist. It MUST NOT be folded into
  distance, because no distance value means "wins the contest and is not
  installed", which is exactly the state the owner asked for.
- A withheld route is NOT a dropped route (owner, 2026-09-06). It stays in the
  Loc-RIB, stays selectable against other protocols on distance, stays
  redistributable, and stays on the plugin API bus. Only the FIB write is
  declined. That is the property BIRD and FRR do not have, because neither has a
  plugin bus to keep serving, and it is why this is not a port of either.
- The use case that justifies it: a controller or route-collector deployment
  that wants BGP in the RIB and on the bus without programming a single kernel
  route. Ze cannot express that today.
- Separating route selection from route programming is a natural way to think
  about a router, and few implementations expose it (owner, 2026-09-06). So an
  operator already holds the concept; what they lack is a place their previous
  tools let them express it. The `ze:help` names the thing plainly and says what
  each value does, in the vocabulary a network engineer already owns -- RIB,
  FIB, selection, programming -- with no justification and no analogy. The
  `docs/guide/` section says what the default is, what changing it does, and
  what withholding a protocol leaves intact. It owes no tutorial.
- The existing per-protocol vocabulary is already incomplete against the
  registry. `redistevents.RegisterProtocol` has ten non-test callers: `kernel`,
  `connected`, `isis`, `as112`, `static`, `ospf`, `ipsec`, `l2tp`, `bgp` and
  `bmp`. `internal/component/sysrib/yang/ze-rib-conf.yang` declares six distance
  leaves: `connected`, `static`, `ebgp`, `ibgp`, `ospf`, `isis`. So four
  registered protocols have no distance leaf, and `ebgp`/`ibgp` are a split the
  registry does not carry. A hand-written list of FIB-import leaves would repeat
  that, and go stale the same way.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/sysrib/sysrib.go` - `recomputeBest` selects the winner
      for a prefix and returns an `outgoingChange`; `publishChanges` emits the
      batch on `(system-rib, best-change)`. No branch anywhere consults the
      winner's protocol to decide WHETHER to publish, except the OS-installed
      branch the connected half added.
- [ ] `internal/core/rib/locrib/candidate.go` - `Path.Source` is a
      `redistevents.ProtocolID`, so the producing protocol is already carried on
      every path and is part of the key.
- [ ] `internal/core/redistevents/registry.go` - `RegisterProtocol(name)`
      allocates the ID; `ProtocolName` and `ProtocolIDOf` map between the two.
      This is the only complete list of protocols Ze has.
- [ ] `internal/component/config/redistribute/route.go` - `ImportRule` matches
      `Source`, `Destination`, `Families`, and `Tag` when `MatchTag` is set. Its
      destination is a PROTOCOL that imports, never the FIB.
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - the six distance
      leaves, hand-written, each with its own `ze:help`.

**Behavior to preserve:**
- The default: every protocol's routes reach the FIB. An operator who writes
  nothing gets what they get today.
- The single-writer property the static half established: one plugin owns a
  main-table entry.
- An operator's explicit `fib { }` block still decides the writer.

**Behavior to change:**
- A named protocol can be excluded from the FIB write.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rib { fib-withhold [ bgp isis ] }`, a leaf-list beside `rib { distance }`.

### Transformation Path
1. `parseFIBImportConfig` resolves the config to a permission set COMPLETE over
   `redistevents.ProtocolNames()`.
2. `recomputeBest` and `cascadeRecompute` consult `fibPermitted` on the winner's
   protocol, beside the existing `osInstalled` branch.
3. A withheld winner goes to `recordWithheldWinner`: it enters `s.best` as any
   other winner, and the change is either nothing or a Withdraw of what Ze had
   programmed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config to sysrib | `publishFIBImport`, in the same verify / configure / apply / rollback callbacks that publish the distance table | Yes -- `internal/component/sysrib/register.go` |
| sysrib to FIB plugins | `(system-rib, best-change)`. A Withdraw is owed only where Ze has an install outstanding, which `programmedByZe` answers | Yes -- FOUR producers, and the count is load-bearing: `recomputeBest`, `cascadeRecompute`, `replayBest` and the permission sweep through `publishFIBImport`. All four take what to program from `fibEntry` |

### Integration Points
- `internal/component/sysrib` configure callback, which already resolves a
  per-protocol table from config and publishes it through a seam.
- `internal/core/redistevents` registry, the only complete protocol list.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The config reaches sysrib through the configure callbacks that already carry the distance table, and reaches the writers through the one `BestChange` emission they already subscribe to. No writer changed |
| No unintended coupling (components stay isolated) | Yes | `internal/component/config` gains one import of `internal/core/redistevents`, a component-to-core direction `./le tier check` passes clean on. No FIB plugin learned a protocol name |
| No duplicated functionality (extends existing, does not recreate) | Yes | It IS a third protocol-keyed surface, and D-1's rationale says why: the three are stages of one pipeline, not copies of one decision. It reuses the stage machinery rather than recreating it -- the same configure callbacks, the same emission, and `recordWithheldWinner` mirroring `recordOSInstalledWinner` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding and no buffer path. The permission set is a map read once per selected prefix |
| Registration over hardcoding | Yes | The vocabulary is `redistevents.ProtocolNames()`, reached through the `registered-protocol` validator and its `CompleteFn`. No protocol name is written in the YANG, in the validator, or in sysrib. A protocol that registers is refused nowhere and completes everywhere, with no list to edit |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The winner's protocol is available at the point the FIB write is decided | `Path.Source` is a `redistevents.ProtocolID` and part of `Path.key` | the filter needs a new field on the change | reading `recomputeBest` and `outgoingChange` | confirmed for the SUPPRESSING side, broken for the SUBSCRIBING side (2026-09-08). `changeToBatch` converts the ID to a name, `recomputeBest` holds `winner.protocol`, and both stamp `outgoingChange.Protocol` on Add and Update. All five Withdraw branches in `sysrib.go` build `&outgoingChange{Action, Prefix}` and stamp NO protocol, so a WRITER cannot tell which protocol a withdraw belonged to |
| A-2 | An excluded protocol's routes still belong in the system RIB, and only the FIB write is suppressed | owner, 2026-09-06: the withheld routes stay on the plugin API, in the Loc-RIB, redistributable and selectable | `show rib` loses the routes, or a plugin stops seeing them | a test that reads the Loc-RIB, the bus and `show rib` with the protocol withheld | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Three places keying on a protocol name drift apart | a protocol named in one and absent from the others | the design question below is settled BEFORE code |
| R-2 | A hand-written leaf list repeats the gap the distance leaves already have | a protocol with no leaf silently defaults | derive from `redistevents` or generate the YANG from it |
| R-3 | The gate is placed where it DISCARDS the path rather than declining to program it | a withheld protocol's routes vanish from `show rib`, from redistribution, or from the plugin bus | RETIRED. `recordWithheldWinner` writes the winner into `s.best` and `s.lastECMP` exactly as `recordOSInstalledWinner` does, so selection is untouched, and `TestFIBWithholdLeavesNoKernelRoute` reads winner `bgp` back out of `show rib` for the withheld prefix while the kernel holds nothing for it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The kernel forwarding table. An over-broad exclusion blackholes every prefix a protocol carries |
| How is it reverted? | Single commit revert; the leaves default to "permitted", so a reverted tree forwards exactly as before |
| Who else touches this path? | `spec-connected-static-reach-the-locrib` owns the publish path and the writer resolution; `spec-fib-depth` owns `BestChangeEntry.TableID` |

## The decision this spec exists to take

The semantics are settled. The owner has answered where the gate sits (at the
RIB-to-FIB boundary, after selection) and what withholding means (the route
lives on everywhere except the kernel). What is open is the SHAPE of the
operator's surface, and the two candidates come from the two peers.

| # | Decision | The options |
|---|----------|-------------|
| D-1 | Whether the filter belongs TO THE FIB WRITER or to a central per-protocol table | (a) BIRD's model: a property of the writer, so a leaf on the `fib { kernel { } }` block, and "which routes this writer accepts" is the writer's own question; (b) FRR's model: a central table keyed by protocol, so a container beside `rib { distance { } }` that every writer reads |

**D-1 ANSWERED (owner, 2026-09-08): (b), the central table.** The setting sits
beside `rib { distance { } }` and sysrib declines the write BEFORE
`BestChange.Emit`, so the suppression exists once and every writer sees the same
filtered stream. The accepted cost is that the setting is global to every FIB:
"BGP into VPP but not into the kernel" is NOT expressible, and a spec that wants
it must reopen D-1 rather than add a per-writer override beside this table.

Two consequences bind the implementation:
- No writer needs to change. `fibkernel.go`, `fibvpp.go` and `fibp4.go` keep
  the code they have, and the anonymous withdraw (A-1) stops being a problem
  because the only component that must know the protocol is the one that
  already does.
- The vocabulary was still owed when D-1 was taken, because copying the
  `rib { distance { } }` shape means copying its hand-written leaf list, which
  is the failure R-2 names. It is delivered: `registered-protocol` reads
  `redistevents.ProtocolNames()`, so no protocol name is written by hand in the
  YANG, the validator or sysrib.

(a) fits Ze's structure, because the FIB writer is already a plugin that owns
its own config container and its own YANG. (b) puts the answer in one place for
an operator running two writers. Ze can run two writers at once
(`test/plugin/fib-vpp-coexist-with-fib-kernel.ci`), which is the fact that makes
this a real choice rather than a spelling.

What the code makes cheap, measured 2026-09-08:

- THREE writers subscribe to ONE emission. `publishChanges` calls
  `sysribevents.BestChange.Emit`, and `fibkernel.go`, `fibvpp.go` and `fibp4.go`
  each `BestChange.Subscribe(eb, f.processEvent)` on the same batch pointer. So
  (b) is necessarily global to every FIB, and (a) cannot suppress at publish:
  each writer holds the gate itself.
- Under (a) each writer needs a prefix-to-protocol map of its own, because the
  withdraw it must emit when a protocol becomes suppressed carries no protocol
  (A-1). Under (b) sysrib never needs one: it already holds `winner.protocol`
  and already produces that exact withdraw in `recordOSInstalledWinner`.
- (b) inherits a working per-protocol config reader.
  `parseAdminDistanceConfig` discovers the protocol SET from
  `config.YANGSchema().Lookup("rib/distance")` and `config.ApplyDefaults`, never
  from a Go list, and publishes through `publishDistances` into
  `distance.Set`, on every configure AND every rollback. (a) has no equivalent:
  a FIB plugin's YANG is hand-written per plugin, one module each.
- Both peers put the filter on the writer, and neither peer has two writers.
  FRR scopes it per-VRF because zebra is the only writer there; BIRD hangs it
  off `protocol kernel` for the same reason. So the peer evidence is silent on
  the question that makes this a choice in Ze, and (a) MUST NOT be justified by
  citing it.
- R-2 is unsolved under BOTH options. Nothing turns
  `redistevents.RegisterProtocol` into a config vocabulary. The nearest thing,
  `redistribute { source }`, validates against a SECOND registry filled by
  hand-written `redistribute.RegisterSource` calls that are independent of the
  first, so the two lists can silently disagree; its sibling `destination` leaf
  validates against nothing and defers to `ze doctor`. Whichever way D-1 goes,
  the vocabulary derivation is new work, not a thing to inherit.

### Peer evidence

| Peer | Shape | Verified |
|------|-------|----------|
| BIRD 2.14 | an export filter on the `kernel` protocol, matching the read-only enumerated `source` attribute: `protocol kernel { ipv4 { export filter F; }; }` with `if source = RTS_BGP then reject;` | MEASURED, 2026-09-07, `bird -p -c` on this host: the config parses with exit 0, and the same config with `RTS_BGP` replaced by an undefined symbol is refused with a syntax error, so the parse is not permissive |
| FRR 10.3.1 | `ip protocol <proto> route-map <map>` and `ipv6 protocol <proto> route-map <map>`, applied in zebra. The protocol enum is closed at the parser and differs per AFI (`rip`/`ospf`/`eigrp` on v4, `ripng`/`ospf6` on v6), and `route-map` is `mandatory true` in `frr-zebra.yang`, so there is no boolean form. The setting is per-VRF, not global: `filter-protocol` augments `/frr-vrf:lib/frr-vrf:vrf` | MEASURED, 2026-09-08, `quay.io/frrouting/frr:10.3.1` on this host with `--cap-add NET_ADMIN,NET_RAW,SYS_ADMIN`. `vtysh -c "conf t" -c "ip protocol bgp route-map FILTER-BGP"` exits 0 and the line appears in `show running-config` and `show ip protocol`. Negative control, run before mgmtd started: `ip protocol frobnicate route-map X` is refused with `% Unknown command`, exit 1. The command is mgmtd-backed and is NOT in zebra's compiled command table, so `zebra -f <conf> -C` refuses it (`EC 100663304 No such command`); a later reader who re-tests through `-C` will wrongly conclude the spelling is wrong |

Neither peer's SEMANTICS transfer. In both, a route the kernel filter rejects is
simply not in the kernel and there is nothing else consuming it. In Ze it is
still on the plugin bus. Copy the spelling where it fits; do not copy the
meaning.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `rib { fib-withhold [ bgp ] }` | → | `fibPermitted`, gating `publishChanges` and `replayBest` | `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci` |
| an operator withholding one protocol on a booted appliance | → | the whole chain, ending at netlink | `TestFIBWithholdLeavesNoKernelRoute` in `internal/plugins/fib/kernel/`, reading `ip route` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config that names no protocol | Every protocol's routes reach the FIB, exactly as today |
| AC-2 | A config excluding one protocol, and a prefix only that protocol offers | No kernel route exists for the prefix, and the path is still in the Loc-RIB, still in `show rib`, still redistributable, and still delivered on the plugin bus |
| AC-3 | A config excluding one protocol, and a prefix two protocols offer | The excluded protocol still WINS selection when its distance is lower, and the prefix is not programmed. Selection and programming are independent |
| AC-6 | A controller deployment: BGP withheld, every peer's routes in the RIB | Not one kernel route is programmed, and a plugin attached to the bus receives every route |
| AC-4 | A protocol that registers and that no leaf names | It defaults to permitted, and the vocabulary shows it without an edit to a hand-written list |
| AC-5 | A config naming a protocol nothing registered | The commit is refused and the message names the registered protocols |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Excludes BGP from the kernel while keeping OSPF | config -> sysrib -> publish decision -> fib-kernel | `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci` and `TestFIBWithholdLeavesNoKernelRoute` |

## Goal Validation

One row per goal in the Task section. The evidence is what proves the goal is
ACHIEVED, not that the code runs.

| Goal | Evidence |
|------|----------|
| An operator names, per protocol, whether that protocol's routes are written to the FIB | `ze config validate` accepts `rib { fib-withhold [ bgp isis ] }` on the shipped daemon flavor, measured 2026-09-08. The user workflow over the whole path is `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci` |
| The default is that they are | `TestFIBImportDefaultPermitsEveryRegisteredProtocol`. The default is the map SAYING permitted for every registered protocol, not a missing key: `parseFIBImportConfig` is complete over `redistevents.ProtocolNames()` |
| The vocabulary derives from the protocol registry, not a hand-written list | No protocol name appears in the YANG, the validator or sysrib. `TestRegisteredProtocolValidatorRefusesAnUnknownName` proves refusal names the registered set; `TestFibWithholdCompletionOffersRegisteredProtocols` proves a registered protocol is offered. The build-dependence measured under Known Limitations is the same property seen from the other side |
| The kernel actually stops forwarding on a withheld route | `TestFIBWithholdLeavesNoKernelRoute` (`internal/plugins/fib/kernel/`, QEMU, `integration && linux`) reads the namespace's real route table through netlink. It asserts the KEPT protocol's prefix is programmed first, so the withheld protocol's absence cannot be a dead chain. RED observed with the `recomputeBest` gate removed: "the withheld protocol programmed 10.98.0.0/24"; GREEN with it restored. `test/plugin/fib-withhold-controller-programs-no-route.ci` puts the same question to a TABLE: 200 withheld prefixes and one permitted prefix arrive in one route-install batch, the permitted one is read out of `ip route show proto 250` first as the proof the chain programs anything at all, and not one withheld prefix is in that output. It carries `option=needs-linux:caps=net-admin`, so the darwin runner skips it and it runs in the Linux guest. OBSERVED PASS 2026-09-09, in QEMU on this host (`uname -r` 7.2.0, aarch64, `id -u` 0): `1/1 PASS 283 fib-withhold-controller-programs-no-route`, 2.2s, log `tmp/session/2026-09-08-aadf4270-6df0-4868-a389-fd00fc12d262/scratch/qemu-fibwithhold.log` |
| A withheld route is not a dropped route | The same QEMU test puts `show rib` to the running plugin and reads winner `bgp` for the withheld prefix. That surface is sysrib's own answer rather than the Loc-RIB the paths were inserted into, so it proves selection survived the gate. At TABLE scale that is `TestWithholdingBGPProgramsNoneOfAWholeTable` (AC-6): 512 withheld prefixes produce not one change on `(system-rib, best-change)`, every one of them is won by `bgp` in `s.best`, and `show rib` answers for all 528 prefixes the two protocols carry. The 16 permitted prefixes in the same run publish their adds, so a sysrib that published nothing could not pass it. RED observed twice, 2026-09-09: with `fibPermits` forced to permit, 512 of 512 withheld prefixes reached the FIB stream; with `publishChanges` cut, the permitted control published 0 of 16 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFIBImportDefaultPermitsEveryRegisteredProtocol` | `internal/component/sysrib/fibimport_test.go` | AC-1 | PASS 2026-09-09 |
| `TestWithheldWinnerPublishesNoAdd` | `internal/component/sysrib/fibimport_test.go` | AC-2 | PASS 2026-09-09 |
| `TestWithheldProtocolStillWinsSelection` | `internal/component/sysrib/fibimport_test.go` | AC-3 | PASS 2026-09-09 |
| `TestWithholdingBGPProgramsNoneOfAWholeTable` | `internal/component/sysrib/fibimport_test.go` | AC-6 | PASS 2026-09-09, RED forced in both directions |
| `TestFIBImportPermitsAProtocolRegisteredAfterConfigure` | `internal/component/sysrib/fibimport_test.go` | AC-4 (runtime) | PASS 2026-09-09 |
| `TestFibWithholdCompletionOffersRegisteredProtocols` | `internal/component/cli` | AC-4 (vocabulary) | PASS 2026-09-09 |
| `TestRegisteredProtocolValidatorRefusesAnUnknownName` | `internal/component/config` | AC-5 | PASS 2026-09-09 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the leaves are boolean or a name list, not numeric | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-withhold-one-protocol-keeps-the-other.ci` | `test/plugin/` | the owner's own example: BGP withheld, OSPF kept | PASS 2026-09-09, in the QEMU Linux guest (kernel 7.2.0, uid 0): `1/1 PASS 284`, 1.5s |
| `fib-withhold-controller-programs-no-route.ci` | `test/plugin/` | the deployment the feature exists for: a whole BGP table in the RIB, and not one prefix of it in the kernel | PASS 2026-09-09, same guest run: `1/1 PASS 283`, 2.2s |
| `TestFIBWithholdLeavesNoKernelRoute` | `internal/plugins/fib/kernel/` (`integration && linux`) | the kernel agrees: `ip route` on a booted appliance | PASS in the guest, with the RED forced and observed first |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible behavior changes. The external system is the Linux FIB, and the QEMU test is the equivalent proof | N-A |

## The spelling (D-2, settled by the code)

D-1 says the table is central. What the operator types follows from three
measured facts, so it is recorded here rather than put to the owner.

```
rib {
    distance {
        ebgp 20
    }
    fib-withhold [ bgp isis ]
}
```

A `leaf-list fib-withhold` of `type string`, carrying
`ze:validate "registered-protocol"`, sibling of `container distance` in
`internal/component/sysrib/yang/ze-rib-conf.yang`. It names what is WITHHELD,
so the polarity is in the name: a list called `fib-import` would not say whether
the names in it are the permitted set or the excluded one.

| Rejected | Reason |
|----------|--------|
| A container of per-protocol boolean leaves, copying the `distance` shape | It is a hand-written protocol list, which is R-2 |
| A YANG list keyed by protocol, the `redistribute { destination }` shape | AC-4 fails on the VOCABULARY half. `enumKeyVocabulary` returns nil unless the key leaf's type is a YANG enum, so a `string` key with a validator completes only the keys the operator already typed. Refusal would still work: `ze:validate` runs on a list key and on each leaf-list item alike |
| Generating the YANG from the registry | Nothing in the tree generates a `.yang`; `glue.Write` emits only `embed.go` and `register.go`. A generator plus its gate, for what one validator answers |
| A new `internal/core/rib/fibimport` seam mirroring `distance` | No producer stamps this and sysrib is its only reader |

No `default` statement is written, and none is possible: `applyChildDefault`
fills a default only for a `*LeafNode`, so a leaf-list absent from the config is
an empty list. Absent, empty and "permit everything" are therefore the same
state, which is what AC-1 wants.

**The default is a named branch, not a map miss.** `parseFIBImportConfig`
returns a map COMPLETE over `redistevents.ProtocolNames()`, holding `true` for
every protocol the operator did not withhold. The read site returns on an
explicit `!known` branch, logged once per protocol, before any boolean is read,
so an unregistered name can never be mistaken for a withhold. This is the
`(value, known)` shape `redistevents.OSInstalled` already documents for the
same hazard, and it fails OPEN for the reason that function gives: reading
unknown as "do not program" blackholes every route from a protocol that forgot
to register. It satisfies the Security Review row.

**Where it gates.** In the three PRODUCERS of an Add or an Update --
`recomputeBest`, `cascadeRecompute` and `replayBest` -- and NOT in
`publishChanges`, which this section named until the implementation showed why
it cannot work there. `publishChanges` receives a list of changes and cannot
tell "Ze had programmed this prefix" from "this prefix is new", so it cannot
emit the Withdraw that is owed when a withheld protocol takes a prefix off a
programmed one. That would leave the kernel forwarding on a path the RIB no
longer selects. `recomputeBest` can tell, because it holds `prev` and the
install state, so the gate lives beside the `osInstalled` branch that already
makes the same kind of decision.

A Withdraw carries no protocol (A-1), so it cannot be filtered ON the protocol.
This section said until round 2 that passing one is therefore "always safe",
and that was wrong: a Withdraw for a prefix Ze never installed makes the kernel
writer call `RouteDel` on a route that is not there, which answers ESRCH and
raises a FIB-sync failure per prefix. With the whole BGP table withheld, that is
one error for every route a peer withdraws, in the deployment this feature was
built for. The real rule is the one the protocol was never needed for: a
Withdraw is owed exactly where Ze has an install OUTSTANDING, which
`programmedByZe` answers.

On reconfigure the sweep compares the OLD permission set against the NEW one and
acts only on the prefixes whose permission CHANGED.

## Files to Modify

- `internal/component/sysrib/yang/ze-rib-conf.yang` - the leaf-list, its
  revision, its `description` and its `ze:help`
- `internal/component/sysrib/sysrib.go` - `fibPermitted`, the two emit sites,
  the reconfigure sweep
- `internal/component/sysrib/register.go` - parse on verify, configure and
  rollback, beside `publishDistances`
- `internal/core/redistevents/registry.go` - `ProtocolNames`
- `internal/component/config/validators.go` and
  `internal/component/config/validators_register.go` - the
  `registered-protocol` validator, whose error names the registered set (AC-5)
- `internal/component/config/validate_sections.go` - add `rib` to
  `validatedSections`, or the annotation is inert. The rib module declares no
  other `ze:validate`, so the blast radius is this one leaf-list
- `docs/guide/configuration.md`, `docs/features.md`,
  `docs/architecture/core-design.md`

## Files to Create

- `internal/component/sysrib/fibimport_test.go`
- `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci`
- an `integration && linux` test in `internal/plugins/fib/kernel/`, a package
  `internal/le/qemu/alltests.go` already lists, so the runner needs no edit

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | the leaf D-1 chooses |
| YANG validation constraints | Yes | AC-5 refuses a name nothing registered |
| CLI commands/flags | No | no new command; `show rib` already prints the source |
| Doctor check for runtime dependencies | Unknown | an exclusion that leaves a prefix unrouted may owe one |
| BGP family surface | N-A | no family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | the guide page D-1 chooses |
| 6 | Has a user guide page? | Yes | a `docs/guide/` section: the default, what changing it does, and what withholding a protocol leaves intact -- the RIB, the plugin bus, redistribution and selection. A reader of that page already knows why a router might want this |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |

## Implementation Steps

1. **Phase: take D-1 with the owner**, after measuring FRR's spelling. Nothing
   below is implementable first.
2. **Phase: the vocabulary.** Derive the protocol names from `redistevents`
   rather than hand-listing them, and prove a newly registered protocol appears
   without an edit.
3. **Phase: the publish decision**, with the kernel read as the evidence.
4. **Phase: docs.**

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Registration over hardcoding | No hand-written protocol list is added; the vocabulary derives from `redistevents` |
| Correctness: the default | A protocol nothing names is PERMITTED, and that default is a named branch rather than a zero value |
| Rule: `ai/rules/principles.md` | The number of places keying on a protocol name did not go from two to three without D-1 saying why |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| the default is unchanged behavior | the existing sysrib and static suites pass untouched |
| the operator can reach it | a `.ci` covering the owner's own example |
| the kernel agrees | a QEMU test reading `ip route` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Fail-closed | A protocol whose permission cannot be resolved MUST be permitted and logged, never silently excluded: a silent exclusion blackholes every prefix that protocol carries |

### Failure Routing

| Failure | Route To |
|---------|----------|
| D-1 unanswered | STOP. This spec cannot start |
| Test fails on behavior mismatch | Re-read Current Behavior. If misunderstood, RESEARCH |

## Design Insights

- The system RIB already holds everything the filter needs. `Path.Source` is the
  producing protocol and it is part of the key, so no new field crosses any
  boundary for the filter itself. What is missing is the operator's vocabulary,
  not the plumbing.
- The connected half of `spec-connected-static-reach-the-locrib` already built
  one branch that suppresses a FIB write for a protocol: an OS-installed winner
  produces a withdraw rather than an install. That is a per-protocol write
  decision reached by DECLARATION rather than by config, and it is the nearest
  thing in the tree to what this spec asks for.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The gate sits at the RIB-to-FIB boundary, after selection | (a) a distance value meaning "never install"; (b) a filter at Loc-RIB insertion; (c) a redistribution rule with the FIB as destination | Owner, 2026-09-06: distance decides priority for resolution in the RIB, which then sends the routes to the FIB, so the two are sequential stages. No distance value means "wins and is not installed". (b) and (c) both make the route ABSENT rather than unwritten, which loses the plugin bus, `show rib` and redistribution -- the whole point of the feature |
| This is a spec rather than an extension of `spec-connected-static-reach-the-locrib` | folding it into the static work | It needs a config subtree that does not exist, a semantics decision nobody has taken (D-2), and an edit to the publish path that spec had already committed. Widening a nearly-green commit is how a day's work becomes unreviewable |

## Known Limitations

- `rib { distance { } }` names six protocols where ten register, and splits
  `bgp` into `ebgp`/`ibgp` where the registry does not. This spec must not
  repeat that, and repairing it is not in scope here.
- The vocabulary is a property of the BUILD, because the registry is populated
  by package init and every protocol sits behind a feature tag. Measured
  2026-09-08 with `ze config validate` over the documented example: a
  `ze_core ze_distro` build holds `connected`, `kernel` and `static` only and
  REFUSES `fib-withhold [ bgp isis ]`; the shipped daemon flavor, which adds
  every feature tag in `.golangci.yml`, accepts it. That is coherent -- a build
  with `ze_bgp` compiled out carries no BGP routes to withhold, and refusing
  tells the operator rather than accepting a line that would do nothing -- but
  it means one config file is not portable across two builds of Ze. Deriving
  the vocabulary from the registry is what makes this true, and a hand-written
  list would have hidden it rather than fixed it.
- The setting is global to every FIB writer, which D-1 accepted. See the D-1
  paragraph for what reopening it would cost.

## RFC Documentation (Scope: config)

N-A. No RFC governs which protocols an implementation programs into its
forwarding table.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] D-1 answered by the owner
- [ ] FRR's spelling MEASURED rather than cited from memory

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `rib { fib-withhold [ bgp isis ] }`, a `leaf-list` of `type string` beside
  `container distance` in `internal/component/sysrib/yang/ze-rib-conf.yang`,
  carrying `ze:validate "registered-protocol"`.
- `registered-protocol`, a config validator (`internal/component/config/validators.go`,
  registered in `validators_register.go`) whose vocabulary and whose refusal
  message both come from `redistevents.ProtocolNames()`. `rib` was added to
  `validatedSections` (`internal/component/config/validate_sections.go`), without
  which the annotation is inert.
- `redistevents.ProtocolNames` (`internal/core/redistevents/registry.go`), the
  registry's own answer for the complete protocol set.
- `internal/component/sysrib/fibimport.go`: `parseFIBImportConfig` (a permission
  map COMPLETE over `ProtocolNames()`), `fibPermits` and `(*sysRIB).fibPermitted`
  (the fail-open read), `recordWithheldWinner`, `fibChange`, `applyFIBImport`,
  `groupPermissionChanged`, `fibStateChange` (the reconfigure sweep) and
  `publishFIBImport`, wired from `internal/component/sysrib/register.go` on
  verify, configure, apply and rollback.
- `fibEntry` in `internal/component/sysrib/sysrib.go`: the one answer to what the
  FIB owes for a prefix, returning a named verdict (`fibPathReachable`,
  `fibPathUnreachable`, `fibPathForbidden`) rather than a bool, taken by all four
  producers of a best-change batch: `recomputeBest`, `cascadeRecompute`,
  `replayBest` and the permission sweep.
- `programmedByZe`, the question that replaced `resolvedNH[key].IsValid()` as the
  test for "Ze has an install outstanding for this prefix".
- Tests: `fibimport_test.go`, `sysrib_fibentry_test.go`, `sysrib_programmed_test.go`,
  `sysrib_replay_test.go`, `sysrib_srv6_test.go` (the package's first SRv6 tests),
  `internal/component/cli/fib_withhold_completion_test.go`,
  `internal/component/config/registered_protocol_validate_test.go`,
  `internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go`,
  two `.ci` scenarios and their three fixture files.

### Bugs Found/Fixed
Every one is a Review Gate finding; the round tables below carry the consequence
of each, and `plan/learned/012-fix-the-question-not-the-site.md` carries the class.

- `resolvedNH[key].IsValid()` read as "Ze programmed this prefix" (B-1, B2-1).
  `resolveNextHop` returns an invalid address unchanged, so an interface-only
  next-hop is programmed with an invalid entry. Fixed by `programmedByZe`, and
  covered by `sysrib_programmed_test.go`.
- `recordWithheldWinner` dropped next-hop tracking a later permitted winner
  assumes (B-2). Fixed; `trackNextHops` and `untrackNextHops` are now paired.
- The reconfigure sweep inferred "newly permitted" from `resolvedNH` and never
  `Untrack`ed what it `Track`ed (I-1, I-2, B2-4). Fixed in `fibStateChange`.
- `ecmpCollect` applied no permission test, so a withheld protocol's gateway rode
  into a permitted winner's multipath group and forwarded traffic (I-3). Fixed in
  `internal/component/sysrib/ecmp.go`.
- The regroup branch compared an unfiltered collector against a filtered
  `lastECMP` (B2-3). Fixed.
- `replayBest` carried the winner's next-hop after an ECMP promotion, so a FIB
  plugin restart re-programmed the prefix over a dead gateway (I3-1, I4-1). Fixed
  by routing replay through `fibEntry`; `TestReplayCarriesTheProgrammedEntry` is
  the positive assertion the negative-only test lacked.
- The permission sweep could not put back what it took away (B5-1): withhold then
  permit published nothing, permanently, and withholding a protocol that was
  merely an equal-cost MEMBER withdrew the prefix instead of regrouping it. Fixed
  by making `fibStateChange` self-contained on the LIVE rule.
- Two of the sweep's five outcomes had zero coverage, measured with
  `-covermode=count` (I6-1). Fixed by `TestWithholdingAnUnreachableMemberPublishesNothing`.

### Documentation Updates
All landed in commit `000e70eec`, before this closure.
- `docs/guide/configuration.md` (+194) - the operator section: the default, what
  changing it does, and what withholding leaves intact.
- `docs/architecture/core-design.md` (+109) - where the gate sits and what the
  four producers take from `fibEntry`.
- `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md` -
  the leaf-list spelling and the `ze:validate` annotation.
- `docs/features.md` - the user-facing feature row.
- `docs/architecture/isis/isis-9-spf-rib.md` - the publish path it describes.
- `./le doc check verify` FAILS on this tree, and not on this work. Read from
  `tmp/session/2026-09-08-aadf4270-6df0-4868-a389-fd00fc12d262/scratch/doc-check-verify.log`:
  8 summary-rule breaks and 4 unresolved source anchors, naming
  `ze-rib-api:command-complete`, `ze-rib-api:command-help`,
  `ze-policyroute-conf:policy/route/interface`, `docs/architecture/api/commands.md`,
  `docs/architecture/exabgp-bridge.md` and `docs/architecture/firewall/firewall-irr.md`.
  Not one names a `fib-withhold`, `sysrib` or `ze-rib-conf` surface.

### Deviations from Plan
| What the spec said | What was built | Why |
|---|---|---|
| The gate sits in `publishChanges` | It sits in the four PRODUCERS of an Add or an Update | `publishChanges` receives a change list and cannot tell "Ze had programmed this" from "this is new", so it cannot emit the Withdraw a withheld winner owes. The spec text was corrected in place under "Where it gates" |
| "A Withdraw carries no protocol, so passing one is always safe" | A Withdraw is owed exactly where `programmedByZe` says an install is outstanding | A Withdraw for a prefix Ze never installed makes the kernel writer answer ESRCH, one FIB-sync failure per withdrawn route, in the deployment this feature exists for |
| `fibEntry` returns a bool | It returns a named verdict | Making the live path obey a bare "not owed" reddened five tests, two of them named by `test/ospf/ospf-route-install.ci`: those scenarios load no `connected` plugin, so refusing there IS the black hole the test exists to prevent |
| One `.ci` in the Wiring Test table | Two | The controller deployment (AC-6) needed a TABLE-scale scenario, not a single prefix |
| A container of per-protocol boolean leaves was one candidate spelling | A leaf-list naming what is WITHHELD | A container copies the hand-written protocol list `rib { distance }` already has, which is R-2 |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed the winner's protocol is available wherever the FIB write is decided | True on the SUPPRESSING side (`recomputeBest` holds `winner.protocol`), false on the SUBSCRIBING side: all five Withdraw branches build `&outgoingChange{Action, Prefix}` and stamp no protocol | Reading `outgoingChange` construction while designing the writer-side option (a) of D-1 | D-1 (b) makes it moot: the only component that must know the protocol is the one that already does. Recorded on the A-1 row |
| approach | The design named `publishChanges` as the choke point, in four sections | It cannot emit the Withdraw a withheld winner owes, because it holds no install state | Implementation, when the withdraw case was written | Route abandoned; the gate moved to the three live producers plus the sweep, and the spec prose was corrected rather than left standing |
| escalation | A defect found mid-work was journaled as "outside the problem in hand", judged against the FEATURE'S NAME | The scope test is the OPERATION that broke. The broken operation was "let this protocol reach the FIB again", which is the feature's own inverse, so it was in scope | Review round 6 rejected the round-5 scope call | Fixed as B5-1. `plan/learned/012-fix-the-question-not-the-site.md` records it and proposes a one-sentence addition to `ai/rules/completion.md`, which is Thomas's to accept |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An operator names, per protocol, whether that protocol's routes are written to the FIB | Done | `internal/component/sysrib/yang/ze-rib-conf.yang` (`fib-withhold`), `internal/component/sysrib/fibimport.go` | Reached end to end by `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci` |
| The default is that they are | Done | `parseFIBImportConfig`, `fibPermits`, `(*sysRIB).fibPermitted` (`internal/component/sysrib/fibimport.go`) | Complete over `ProtocolNames()`; the unknown branch fails OPEN and logs once per protocol |
| The vocabulary derives from the protocol registry | Done | `redistevents.ProtocolNames` (`internal/core/redistevents/registry.go`), `registered-protocol` (`internal/component/config/validators.go`) | No protocol name in the YANG, the validator or sysrib |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFIBImportDefaultPermitsEveryRegisteredProtocol` | Absent, empty and "permit everything" are one state, which is what the leaf-list gives |
| AC-2 | Done | `TestWithheldWinnerPublishesNoAdd`; `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci`; `TestFIBWithholdLeavesNoKernelRoute` | The `.ci` asserts `OK: 10.98.0.0/24 is won by bgp and is not programmed` |
| AC-3 | Done | `TestWithheldProtocolStillWinsSelection` | `recordWithheldWinner` writes `s.best` and `s.lastECMP` as any other winner |
| AC-4 | Done | `TestFIBImportPermitsAProtocolRegisteredAfterConfigure` (runtime), `TestFibWithholdCompletionOffersRegisteredProtocols` (vocabulary) | |
| AC-5 | Done | `TestRegisteredProtocolValidatorRefusesAnUnknownName` | The refusal message names the registered set |
| AC-6 | Done | `TestWithholdingBGPProgramsNoneOfAWholeTable` (512 withheld, 16 permitted); `test/plugin/fib-withhold-controller-programs-no-route.ci` (`OK: 200 bgp prefixes are in the rib and none is programmed`) | RED forced in both directions, 2026-09-09 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The seven planned unit tests | Done | `internal/component/sysrib/fibimport_test.go`, `internal/component/cli/`, `internal/component/config/` | All seven PASS, 2026-09-09 |
| `fib-withhold-one-protocol-keeps-the-other.ci` | Done | `test/plugin/` | PASS in the QEMU Linux guest, 1.5s |
| `fib-withhold-controller-programs-no-route.ci` | Done | `test/plugin/` | PASS in the same guest run, 2.2s |
| `TestFIBWithholdLeavesNoKernelRoute` | Done | `internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go` | PASS in the guest; RED forced first with the `recomputeBest` gate removed |
| Boundary tests | N-A | - | The leaf-list is a name list, not numeric |
| Interop scenario | N-A | - | No wire-visible change; the external system is the Linux FIB and the QEMU test is that proof |
| Tests added beyond the plan | Changed | `sysrib_fibentry_test.go`, `sysrib_programmed_test.go`, `sysrib_replay_test.go`, `sysrib_srv6_test.go` | Six SRv6 tests, the package's first, and the positive replay assertion round 3 said was owed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/sysrib/yang/ze-rib-conf.yang` | Done | +46 |
| `internal/component/sysrib/sysrib.go` | Done | `fibEntry`, `programmedByZe`, the four producers |
| `internal/component/sysrib/register.go` | Done | Parse on verify, configure, apply and rollback |
| `internal/core/redistevents/registry.go` | Done | `ProtocolNames` |
| `internal/component/config/validators.go`, `validators_register.go` | Done | The `registered-protocol` validator |
| `internal/component/config/validate_sections.go` | Done | `rib` added to `validatedSections` |
| `internal/component/sysrib/fibimport.go` | Changed | Not in the Files to Create list; the new surface earned its own file rather than growing `sysrib.go` |
| `internal/component/sysrib/ecmp.go` | Changed | Not in the plan; I-3 required the permission test in `ecmpCollect` |
| `internal/component/sysrib/fibimport_test.go` | Done | 986 lines |
| Both `.ci` files and their fixtures | Done | `internal/test/fixture/fib_withhold_fixture.go`, `fib_withhold_controller_fixture.go`, `register_fib_withhold.go` |
| The `integration && linux` kernel test | Done | `internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go` |
| `docs/guide/configuration.md`, `docs/features.md`, `docs/architecture/core-design.md` | Done | Plus three pages the plan did not name |

### Audit Summary
- **Total items:** 30 (3 requirements, 6 acceptance criteria, 7 planned tests, 14 planned files)
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (`fibimport.go` as a new file, `ecmp.go` added to the diff, four test files beyond the plan). All three ADD to the plan and none removes anything from it, so none needs user approval.

## Goal Validation (BLOCKING)

The `## Goal Validation` section above records the same goals with the design-time
reasoning. This table is the closure re-check: one row per Task goal, with the
evidence observed on 2026-09-09.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator names, per protocol, whether that protocol's routes are written to the FIB | functional (user workflow over the whole path) | `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci`, PASS in the QEMU Linux guest (`uname -r` 7.2.0, aarch64, `id -u` 0): `1.5s 1/1 PASS 284`. It drives `rib { fib-withhold [ bgp ] }` on a real `ze -` daemon with fib-kernel loaded, and asserts `OK: 10.98.0.0/24 is won by bgp and is not programmed`. Log: `tmp/session/2026-09-08-aadf4270-6df0-4868-a389-fd00fc12d262/scratch/qemu-fibwithhold.log` |
| The default is that they are | functional (data correctness) | `TestFIBImportDefaultPermitsEveryRegisteredProtocol` PASS. The producer is `parseFIBImportConfig` (`internal/component/sysrib/fibimport.go`), whose map is complete over `redistevents.ProtocolNames()`, and `fibPermitted` returns permitted on an explicit `!declared` branch, logged once per protocol |
| The vocabulary derives from the protocol registry, not a hand-written list | functional (both polarities) | `TestRegisteredProtocolValidatorRefusesAnUnknownName` PASS: the refusal names the registered set. `TestFibWithholdCompletionOffersRegisteredProtocols` PASS: a registered protocol is offered by the editor. Both run 2026-09-09 |
| The kernel actually stops forwarding on a withheld route | functional (kernel state read through netlink) | `TestFIBWithholdLeavesNoKernelRoute` PASS in the guest, with RED observed first: `qemu-withhold-red.log` holds `--- FAIL: TestFIBWithholdLeavesNoKernelRoute` with the `recomputeBest` gate removed, `qemu-withhold-green.log` holds `--- PASS` with it restored. The whole `internal/plugins/fib/kernel` package runs green in the guest: 68 top-level tests, 88 including subtests, 0 failures |
| A withheld route is not a dropped route (the controller deployment) | functional (table scale, both polarities) | `TestWithholdingBGPProgramsNoneOfAWholeTable` PASS: 512 withheld BGP prefixes produce no change on `(system-rib, best-change)` while 16 permitted OSPF prefixes publish their adds, and `show rib` answers for all 528. RED forced twice on 2026-09-09, in both directions. `test/plugin/fib-withhold-controller-programs-no-route.ci` PASS in the guest, `2.2s 1/1 PASS 283`, asserting `OK: 200 bgp prefixes are in the rib and none is programmed` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A promoted ECMP member is programmed with the WINNER's label stack and SRv6 SID | An MPLS misforward, identical at HEAD (`git show HEAD:internal/component/sysrib/sysrib.go` carries the same `Labels: best.labels` beside `Interface: ecmpPaths[0].Interface`). Withholding a protocol is not broken by it, so the owner-directed route for a defect walked into is a journal row (`ai/rules/completion.md`) | `plan/journal/helper-bypassed-by-an-open-coded-copy.md`, row of 2026-09-09 |
| `cascadeRecompute`'s RESOLVER-driven path still disagrees with the live rule about an unreachable gateway | Pre-existing at HEAD, and about reachability NEWS rather than about which protocols reach the FIB. The SWEEP half of the same disagreement WAS in scope and is fixed (B5-1) | `plan/journal/guard-added-to-one-half-of-a-pair.md`, rows of 2026-09-08 |
| RFC 9252 Section 5, SRv6 SID resolvability before best-path (`RFC9252-5-2`) | Found as review finding I4-2: a code comment quoted a MUST the code does not enforce while the ledger records it as a `{gap}`. The comment was corrected here; making the behavior exist is a protocol change of its own, routed by owner decision | `plan/immediate/spec-srv6-bestpath-resolvability.md` (Status `design`) |
| `applyRouteInstall`'s doc comment describes a register-on-demand its callee deliberately refuses | Met on the way through the dispatch path; outside the sysrib publish decision | `plan/journal/comment-describes-superseded-behaviour.md`, row of 2026-09-08 |
| `rib { distance { } }` names six protocols where ten register, and splits `bgp` into `ebgp`/`ibgp` where the registry does not | Named out of scope in Known Limitations before implementation started. This spec did not repeat the gap. Repairing the distance leaves is separable | No spec owns it. It is an item for Thomas, not a decision this closure may take |

### The two rule changes `plan/learned/012` proposes

Not part of this closure, and NOT edited here. They are Thomas's to accept or reject.

| Rule | Proposed addition |
|------|-------------------|
| `ai/rules/completion.md`, beside the scope question | Answer it against the OPERATION that broke, never against the feature's name. A feature's name covers its inverse, its undo and its disable path |
| `ai/rules/testing.md`, beside the positive-and-negative directive | New branches owe a MEASURED coverage figure, not an inferred one. Run the package under `-covermode=count` |

## Review Gate

<!-- Filled at implementation time by /ze-review (BLOCKING before closure).
     Never delete this section. -->

### Round 1
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| The whole change, independent reviewer, 2026-09-08 | gate coverage, withdraw safety, state coherence, the guard, locking, test discrimination, prose | 2 | 4 | 4 |

Every finding traced to ONE root cause the feature inherited rather than
introduced: `resolvedNH[key].IsValid()` was read as "Ze programmed this
prefix", and it is not. `resolveNextHop` returns an invalid address unchanged,
so an interface-only next-hop is programmed with an invalid entry that
`IsValid` cannot tell from an absent one. The struct comment asserted the
opposite, and the new code trusted it.

| # | Level | Finding | Consequence |
|---|-------|---------|-------------|
| B-1 | BLOCKER | `resolvedNH` presence test | `fib-withhold [ static ]` leaves every interface-only static route in the kernel, and a permitted device-only route emits a spurious Add on every apply, which the kernel writer answers with EEXIST and a FIB-sync error |
| B-2 | BLOCKER | `recordWithheldWinner` drops next-hop tracking a later permitted winner assumes | A prefix promoted back to a permitted protocol on the SAME next-hop is programmed with no tracking entry, so it is never re-evaluated when that next-hop dies |
| I-1 | ISSUE | the sweep infers "newly permitted" from `resolvedNH` | Any `rib` apply resurrects a route withdrawn for an unresolvable next-hop |
| I-2 | ISSUE | the sweep's withdraw branch never `Untrack`s what its add branch `Track`s | withheld to permitted to withheld leaks a resolver entry `showNHTable` prints |
| I-3 | ISSUE | `ecmpCollect` applies no permission test | A withheld protocol's gateway rides into a permitted winner's multipath group and forwards traffic. The feature fails for the operator who most needs it |
| I-4 | ISSUE | docs name `publishChanges` as the filter | It filters nothing; a maintainer adding a fourth emit site would believe the choke point is covered |

The lens that found none: the guard itself. `fibPermitted` fails open
correctly, the permission map is complete over `ProtocolNames()` on the seed,
configure, apply and rollback paths, and no branch can read a withhold out of
"not configured yet". Lock ordering is `mu` then `fibMu` on every path.

### Round 2
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| The round 1 fixes, independent reviewer, 2026-09-08 | the fixes themselves, plus what they could have broken | 1 | 4 | 4 |

Round 1's six findings were each fixed where round 1 POINTED, and round 2 found
the same class alive on sibling paths. That is the finding worth keeping: a
review that names a site gets that site fixed, and the defect class survives
wherever the reviewer did not look.

| # | Level | Finding | Consequence |
|---|-------|---------|-------------|
| B2-1 | BLOCKER | The `len(protocols) == 0` branch of `recomputeBest` still reads `prev != nil` as "Ze programmed it" | With BGP withheld, EVERY BGP withdraw emits a Withdraw for a route Ze never installed. The kernel writer answers ESRCH and raises a FIB-sync failure per prefix, so the route-collector deployment this feature exists for produces one error per withdrawn route |
| B2-2 | ISSUE | `cascadeRecompute` kept the `IsValid` test where the stored address really can be invalid, after an ECMP promotion picks a device-only member | A group whose next-hops have all gone emits no Withdraw, and the kernel forwards on it forever |
| B2-3 | ISSUE | The regroup branch compares the unfiltered collector against a `lastECMP` the cascade may have written with the filtered one | An unrelated apply puts an unreachable member back into the kernel multipath |
| B2-4 | ISSUE | Both sides of the sweep still mishandle a prefix withdrawn for an unreachable next-hop | I-1 and I-2 surviving on the one path their fixes did not cover |
| B2-5 | ISSUE | The guide named `bgp-rib/best-change` as the stream that still carries a withheld route | True for BGP, false for IS-IS, OSPF, static and connected, which never use that topic. My sentence, and the section's own example withholds `isis` |

The one that matters beyond this spec is B2-1, because the false statement was
in the DESIGN prose before it was in the code: "a withdraw carries no protocol,
so passing one is always safe" reads like a proof and is not one. The protocol
was never the question. Whether Ze has an install outstanding is.

### Round 3
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| The round 2 fixes, independent reviewer, 2026-09-08 | the class sweep, the new consolidation, replay, shared state, the display, discrimination, docs | 0 | 4 | 5 |

**The pattern broke.** Rounds 1 and 2 fixed the site the reviewer named. Round 3
replaced the QUESTION -- `programmedByZe` -- and then walked its callers. The
reviewer enumerated every Withdraw, Add and Update construction and every
`resolvedNH` read in the package and confirmed the list: seven Withdraws, all
behind an install test, and exactly two value reads, both guarded. All five of
round 2's findings are fixed and none moved.

| # | Level | Finding | Consequence |
|---|-------|---------|-------------|
| I3-1 | ISSUE | `replayBest`'s membership test is right and its PAYLOAD still reads a proxy | After an ECMP promotion the replay carries the WINNER's next-hop, which the resolver declared unreachable, and the group with the promoted member removed. A FIB plugin restart broadcasts a replay request, so a restart re-programs the prefix over the dead gateway |
| I3-2 | ISSUE | `show ecmp-groups` still describes itself as showing "the paths that share its load" | It now answers the RIB, so it lists members that share no load and prefixes with no install |
| I3-3 | ISSUE | `core-design.md` says a protocol "the resolved table" does not name is permitted | The fail-open branch reads the permission set. Two different structures, one sentence |
| I3-4 | ISSUE | `core-design.md` says replay hands over no prefix whose next-hop stopped resolving | False on the promotion branch, which is I3-1 |

Two structural observations the reviewer left as NOTEs, both worth more than
their level. `TestReplaySkipsAPrefixZeDoesNotProgram` is the only test that
names `replayBest` and it is a NEGATIVE, so deleting the body of `replayBest`
leaves the package green: the fix for I3-1 owes a positive assertion. And
`lastECMP` now has no reader outside replay, because both display commands
answer the RIB, so nothing in Ze reports the group the FIB was actually told.

### Round 4
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| The round 3 fixes, independent reviewer, 2026-09-08 | the new consolidation, the SRv6 move, the Add/Update change, the class sweep, discrimination, docs | 0 | 4 | 7 |

All four of round 3's ISSUEs fixed, none moved. The new findings were all one
shape: `fibEntry` became the single answer for the cascade and the replay, and
`recomputeBest` was left computing its own.

| # | Level | Finding | Consequence |
|---|-------|---------|-------------|
| I4-1 | ISSUE | The consolidation left out the third producer | Live and replay described the same prefix differently: raw versus resolved member addresses, and a live change re-emitting a member the cascade had dropped. Replay also began REFUSING a prefix sysrib records as programmed |
| I4-2 | ISSUE | The RFC 9252 comment quoted a MUST the code does not enforce, while the ledger records it as a `{gap}` | Two declarations of one fact disagreeing, with the comment the wrong one. Routed to `plan/immediate/spec-srv6-bestpath-resolvability.md` by owner decision |
| I4-3 | ISSUE | The Add-vs-Update comment's premise was false | It asked what Update maps to and never what ADD maps to. `RouteAdd` carries `NLM_F_EXCL` and fails EEXIST, which `mplsentry.go` documents deliberately, so the verb IS visible |
| I4-4 | ISSUE | `TestReplayCarriesTheProgrammedEntry` did not discriminate | Every path in it was connected, so resolved and raw were equal and the expectation was byte-identical to the code it replaced |

The fix for I4-1 was NOT the one the review prescribed, and the deviation is the
useful record. Making the live path obey `fibEntry`'s "not owed" verdict
reddened five tests, two of which `test/ospf/ospf-route-install.ci` runs by
name and one mirroring a functional test whose stated purpose is preventing a
silent black hole: those scenarios load no `connected` plugin, so no covering
route exists and every OSPF, IS-IS and forked next-hop is unresolvable in the
Loc-RIB. Refusing there IS the black hole. The answer was to replace the bool
with a named verdict, `fibPathReachable` / `fibPathUnreachable` /
`fibPathForbidden`, where an unreachable verdict still carries the producer's
target on the live path and reads as a path LOST on a cascade.

### Round 5
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| The round 4 fixes, independent reviewer, 2026-09-08 | the verdict type, the unreachable-but-program rule, payload convergence, the SRv6 tests, the class sweep, the RFC comment, docs and YANG | 1 | 2 | 5 |

| # | Level | Finding | Consequence |
|---|-------|---------|-------------|
| B5-1 | BLOCKER | The permission sweep could not put back what it took away | `fibStateChange` delegated to `cascadeRecompute`, which reads an unreachable verdict as a path LOST. Withhold-then-permit on a prefix whose gateway the Loc-RIB does not cover published NOTHING, permanently: `recomputeBest`'s no-op test ignores `programmed`, so a re-announcement returns nil, and a cascade only fires when a covering route changes. Worse, withholding a protocol that was merely an equal-cost MEMBER of another winner's group WITHDREW that prefix instead of regrouping it |
| I5-1 | ISSUE | The same disagreement lives on `cascadeRecompute`'s resolver-driven path | Pre-existing at HEAD, and about reachability news rather than about which protocols reach the FIB. Journaled, not fixed |
| I5-2 | ISSUE | This Review Gate stopped at round 3 | Rounds 4 and 5 existed nowhere a closure agent could read. Fixed by these two sections |

### Round 6
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| The round 5 fix, independent reviewer, 2026-09-09 | the rewritten sweep, what it writes, the class sweep, the hand-written doc paragraph, discrimination, coverage | 0 | 2 | 4 |

Verdict: ready to close once I6-1 has a test and I6-2 is journaled. Both done.
B5-1 is fixed at the root rather than moved: `fibStateChange` is self-contained
and no longer calls `cascadeRecompute` at all.

| # | Level | Finding | Outcome |
|---|-------|---------|---------|
| I6-1 | ISSUE | Two of the sweep's five outcomes had ZERO coverage, measured with `-covermode=count` rather than inferred. Deleting the no-op guard left the package green | FIXED. `TestWithholdingAnUnreachableMemberPublishesNothing` takes the guard's count from 0 to 1, and without the guard the sweep republishes, byte for byte, the entry the cascade already emitted |
| I6-2 | ISSUE | A promoted ECMP member is programmed with the WINNER's label stack and SRv6 SID. `fibChange` stamps both from the winner and the promotion overrides only `Interface` and `Weight`, so a cross-protocol member becomes the primary next-hop under a stack belonging to another route: an MPLS misforward | JOURNALED, not fixed. Identical at HEAD. This change did not cause it and did widen it, from one emitter to four, by consolidating the payload -- which is the same consolidation working in the fixer's favour |

The two scope calls in this spec went opposite ways and both were right the
second time. B5-1 was journaled and should not have been. I6-2 is journaled and
should be, and the difference is the test: ask which OPERATION breaks, never
whether the defect sits in a file the feature's name covers. Withholding a
protocol is broken by B5-1 and is untouched by I6-2, whose victim is MPLS
forwarding.

**B5-1 is the round I got wrong, and the record matters more than the fix.** I
had already met that divergence, in the previous round's report, and journaled
it as "outside the problem in hand, which is which protocols reach the FIB".
The reviewer rejected that scope call: the operation that fails IS "let this
protocol reach the FIB again", on code this spec introduced, and
`ai/rules/completion.md` returns a recorded defect on the path in hand to
scope. The scope test had been applied to the FEATURE's NAME rather than to the
OPERATION that broke, and the two came apart because the broken operation was
the feature's own inverse. The sweep now takes the LIVE rule, which is the one
with tests and a `.ci` scenario behind it; matching the cascade was the
tidier-looking direction that drops routes.

### Closure record

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/per-protocol-fib-import-aadf4270-6df0-4868-a389-fd00fc12d262.md` |
| `./le spec session review check` | clean -- `review_gate: OK (0 code files, clean, hashes match ...)`. It reads 0 code files because the code is already committed as `000e70eec`; the artifact pins the eight source hashes the reviewers read |
| Rounds | 6. Round 6 was authorised by Thomas on 2026-09-09 after he read the per-round finding counts. The product defect that earned it: the ECMP promotion programs the promoted member with the WINNER's label stack and SRv6 SID, an MPLS misforward, and `-covermode=count` showed two of the permission sweep's five outcomes at zero |
| Reviewer lenses used | gate coverage, withdraw safety, state coherence, the guard itself, lock ordering, test discrimination, the class sweep over sibling paths, payload convergence, the SRv6 branches, the RFC comment, YANG, docs and prose |
| Final verdict | 0 BLOCKER. Round 6's two ISSUEs are closed: I6-1 has a test, I6-2 is journaled |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| B-1 | BLOCKER | `resolvedNH` presence read as "Ze programmed this prefix" | `internal/component/sysrib/sysrib.go` | `programmedByZe`, the question asked once |
| B-2 | BLOCKER | `recordWithheldWinner` dropped next-hop tracking a later permitted winner assumes | `internal/component/sysrib/fibimport.go` | Paired `trackNextHops` / `untrackNextHops` |
| I-1 | ISSUE | The sweep inferred "newly permitted" from `resolvedNH` | `fibimport.go` | The sweep reads the permission sets it is comparing |
| I-2 | ISSUE | The sweep's withdraw branch never `Untrack`ed what its add branch `Track`ed | `fibimport.go` | Both branches paired |
| I-3 | ISSUE | `ecmpCollect` applied no permission test, so a withheld gateway forwarded traffic inside a permitted winner's group | `internal/component/sysrib/ecmp.go` | The permission test moved into the collector |
| I-4 | ISSUE | Docs named `publishChanges` as the filter | `docs/architecture/core-design.md` | Corrected to the four producers |
| B2-1 | BLOCKER | The `len(protocols) == 0` branch still read `prev != nil` as "Ze programmed it", so every withheld withdraw raised an ESRCH FIB-sync failure | `sysrib.go` | `programmedByZe` on that branch too |
| B2-2 | ISSUE | `cascadeRecompute` kept the `IsValid` test where the stored address really can be invalid | `sysrib.go` | Same question, same answer |
| B2-3 | ISSUE | The regroup branch compared an unfiltered collector against a filtered `lastECMP` | `ecmp.go` | Both sides filtered |
| B2-4 | ISSUE | I-1 and I-2 surviving on the path their fixes did not cover | `fibimport.go` | The class swept, not the site |
| B2-5 | ISSUE | The guide named `bgp-rib/best-change` as the stream that still carries a withheld route: false for IS-IS, OSPF, static and connected | `docs/guide/configuration.md` | Sentence corrected |
| I3-1 | ISSUE | `replayBest`'s payload read a proxy, so a FIB plugin restart re-programmed the prefix over a dead gateway | `sysrib.go` | Replay takes `fibEntry`; `TestReplayCarriesTheProgrammedEntry` is the positive assertion |
| I3-2 | ISSUE | `show ecmp-groups` described itself as showing "the paths that share its load" while it answers the RIB | `internal/component/sysrib` | Text corrected |
| I3-3 | ISSUE | `core-design.md` said a protocol "the resolved table" does not name is permitted: two structures, one sentence | `docs/architecture/core-design.md` | Corrected to the permission set |
| I3-4 | ISSUE | `core-design.md` claimed replay hands over no prefix whose next-hop stopped resolving | `docs/architecture/core-design.md` | Corrected with I3-1 |
| I4-1 | ISSUE | The consolidation left `recomputeBest` computing its own entry, so live and replay described one prefix differently | `sysrib.go` | All four producers take `fibEntry`, which returns a named verdict |
| I4-2 | ISSUE | An RFC 9252 comment quoted a MUST the code does not enforce while the ledger records it as a `{gap}` | `internal/component/sysrib` | Comment corrected; the behavior is owned by `plan/immediate/spec-srv6-bestpath-resolvability.md` |
| I4-3 | ISSUE | The Add-versus-Update comment's premise was false: `RouteAdd` carries `NLM_F_EXCL`, so the verb IS visible | `sysrib.go` | Comment corrected |
| I4-4 | ISSUE | `TestReplayCarriesTheProgrammedEntry` did not discriminate: every path in it was connected | `sysrib_replay_test.go` | Rewritten over a path where resolved and raw differ |
| B5-1 | BLOCKER | The permission sweep could not put back what it took away, permanently, and withholding a group MEMBER withdrew the prefix instead of regrouping it | `fibimport.go` | `fibStateChange` is self-contained on the LIVE rule and no longer calls `cascadeRecompute` |
| I5-1 | ISSUE | The same disagreement lives on `cascadeRecompute`'s resolver-driven path | `sysrib.go` | Journaled: `plan/journal/guard-added-to-one-half-of-a-pair.md`. Pre-existing at HEAD and about reachability news |
| I5-2 | ISSUE | This Review Gate stopped at round 3, so rounds 4 and 5 existed nowhere a closure agent could read | this spec | The round 4 and 5 sections |
| I6-1 | ISSUE | Two of the sweep's five outcomes had zero coverage, measured with `-covermode=count`; deleting the no-op guard left the package green | `fibimport.go` | `TestWithholdingAnUnreachableMemberPublishesNothing` takes the guard's count from 0 to 1 |
| I6-2 | ISSUE | A promoted ECMP member is programmed with the winner's label stack and SRv6 SID: an MPLS misforward | `sysrib.go` | Journaled: `plan/journal/helper-bypassed-by-an-open-coded-copy.md`. Identical at HEAD, and no part of withholding a protocol is broken by it |

## Pre-Commit Verification

### Files Exist (ls)
`ls -la`, run 2026-09-09 in this closure. Sizes in bytes as reported.

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/sysrib/fibimport.go` | Yes | 17K |
| `internal/component/sysrib/fibimport_test.go` | Yes | 38K |
| `internal/component/sysrib/sysrib_fibentry_test.go` | Yes | 6.8K |
| `internal/component/sysrib/sysrib_programmed_test.go` | Yes | 5.2K |
| `internal/component/sysrib/sysrib_replay_test.go` | Yes | 5.9K |
| `internal/component/sysrib/sysrib_srv6_test.go` | Yes | 14K; six `func Test` declarations, the package's first SRv6 tests |
| `internal/component/cli/fib_withhold_completion_test.go` | Yes | 1.5K |
| `internal/component/config/registered_protocol_validate_test.go` | Yes | 3.3K |
| `internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go` | Yes | 14K |
| `internal/test/fixture/fib_withhold_fixture.go` | Yes | 5.5K |
| `internal/test/fixture/fib_withhold_controller_fixture.go` | Yes | 7.6K |
| `internal/test/fixture/register_fib_withhold.go` | Yes | 536 |
| `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci` | Yes | 2.3K |
| `test/plugin/fib-withhold-controller-programs-no-route.ci` | Yes | 2.4K |

### AC Verified (grep/test)
Every row re-run in this closure on 2026-09-09, not carried from the audit.

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A config naming no protocol permits every protocol | `--- PASS: TestFIBImportDefaultPermitsEveryRegisteredProtocol (0.00s)` |
| AC-2 | A withheld protocol's only prefix is not programmed and is still in the RIB | `--- PASS: TestWithheldWinnerPublishesNoAdd (0.00s)`; `.ci` `1/1 PASS 284 fib-withhold-one-protocol-keeps-the-other`, 1.5s, in the guest |
| AC-3 | The withheld protocol still WINS selection | `--- PASS: TestWithheldProtocolStillWinsSelection (0.00s)` |
| AC-4 | A protocol no leaf names defaults to permitted, and the vocabulary shows it | `--- PASS: TestFIBImportPermitsAProtocolRegisteredAfterConfigure (0.00s)`; `--- PASS: TestFibWithholdCompletionOffersRegisteredProtocols (0.03s)` |
| AC-5 | A name nothing registered is refused, naming the registered set | `--- PASS: TestRegisteredProtocolValidatorRefusesAnUnknownName (0.07s)` |
| AC-6 | A whole withheld table reaches the RIB and no kernel route | `--- PASS: TestWithholdingBGPProgramsNoneOfAWholeTable (0.00s)`; `.ci` `1/1 PASS 283 fib-withhold-controller-programs-no-route`, 2.2s, in the guest |

### Wiring Verified (end-to-end)
Both `.ci` files read in this closure, not inferred from their names.

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `rib { fib-withhold [ bgp ] }` beside `rib { distance }` | `test/plugin/fib-withhold-one-protocol-keeps-the-other.ci` | Yes. The config block carries `fib-withhold [ bgp ]`, loads `internal rib` and `internal fib-kernel`, and runs a real `ze -` daemon (`cmd=foreground:seq=1:exec=ze -`). It asserts `expect=stderr:contains=OK: 10.98.0.0/24 is won by bgp and is not programmed` and rejects `panic` and `fatal error`. `option=needs-linux:caps=net-admin`, so the check reads the kernel table for proto 250 |
| the whole chain to netlink, on a booted appliance | `TestFIBWithholdLeavesNoKernelRoute` (`internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go`) | Yes. Reads the namespace's real route table. It asserts the KEPT protocol's prefix is programmed FIRST, so the withheld protocol's absence cannot be a dead chain |
| the controller deployment: a whole table withheld | `test/plugin/fib-withhold-controller-programs-no-route.ci` | Yes. Same daemon path, `fib-withhold [ bgp ]`, 200 withheld prefixes and one permitted control, asserting `OK: 200 bgp prefixes are in the rib and none is programmed` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken (partially) | Confirmed on the SUPPRESSING side, broken on the SUBSCRIBING side: all five Withdraw branches in `sysrib.go` build `&outgoingChange{Action, Prefix}` and stamp no protocol. D-1 (b) makes it moot. Mistake Log row written |
| A-2 | confirmed | `TestWithheldProtocolStillWinsSelection` and `TestWithholdingBGPProgramsNoneOfAWholeTable` read the withheld winner back out of `s.best` and `show rib`; `TestFIBWithholdLeavesNoKernelRoute` reads `show rib` from the running plugin and finds winner `bgp` for a prefix the kernel does not hold |
| R-3 | retired | `recordWithheldWinner` writes `s.best` and `s.lastECMP` exactly as `recordOSInstalledWinner` does |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature (`docs/features.md`) | Landed in `000e70eec` (+6/-) | Yes |
| 2. Config syntax (`docs/guide/configuration.md`, `docs/architecture/config/syntax.md`) | Landed in `000e70eec` (+194, +10). The documented spelling is the `leaf-list fib-withhold` in `internal/component/sysrib/yang/ze-rib-conf.yang`, and `ze config validate` accepts it on the shipped daemon flavor | Yes |
| 6. User guide section | `docs/guide/configuration.md`: the default, what changing it does, and what withholding leaves intact. B2-5 corrected its claim about which bus topic still carries a withheld route | Yes |
| 12. Internal architecture (`docs/architecture/core-design.md`) | +109. I-4, I3-3 and I3-4 each corrected a statement in it against the producing function | Yes |
| CLI commands/flags | No new command. `show rib` and `show ecmp-groups` already print the source; I3-2 corrected the `show ecmp-groups` self-description | Yes |
| Doctor check for a runtime dependency | No. The feature adds no file path, socket, kernel module, listen port, external binary or certificate. It reads config already parsed and declines a write Ze would otherwise make | No, with reason |
| RFC status row | No. `./le rfc index-update` was not run, because this change proves no new RFC behavior. Review finding I4-2 corrected a comment that OVER-claimed RFC 9252 Section 5, moving the claim back into line with the `{gap}` the ledger already records; the behavior itself is owned by `plan/immediate/spec-srv6-bestpath-resolvability.md` | No, with reason |
| `./le doc check verify` | FAILED on this tree, on surfaces this work does not touch: 8 summary-rule breaks (`ze-rib-api:command-complete`, `ze-rib-api:command-help`, `ze-policyroute-conf:policy/route/interface` among them) and 4 unresolved source anchors (`docs/architecture/api/commands.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/firewall/firewall-irr.md` twice). Grepped: not one failure names a `fib-withhold`, `sysrib` or `ze-rib-conf` surface | Verified as NOT this work |

### The gate that is NOT met: `./le verify worktree`

**Status: RED, and it stays red. It is recorded unmet rather than claimed.**

Run in this closure on 2026-09-09. Its lint stage fails with 50 issues, all
`typecheck`, and every one of them is in a single package this spec never
touched: `internal/component/bgp/plugins/rib`. The message is
`too many arguments in call to peer1RIB.Insert -- have (family.Family, []byte,
[]byte, bool), want (family.Family, []byte, []byte)`. `git status --porcelain`
shows 17 modified and 2 untracked files in that package, so another session is
mid-edit on `Insert`'s signature and its test files have not caught up.
`./le verify status check` reports `STALE: last verify failed (exit=1, at
2026-09-05T00:11:54Z)`, four days before this work, so the red is not this
work's.

`ai/rules/principles.md` says to judge your own change by the evidence your own
change produced and to leave another session's work alone. The per-scope
evidence that stands in the whole-tree gate's place, all observed 2026-09-09:

| Scope | Result |
|-------|--------|
| `./le verify lint run scope ./internal/component/sysrib/...` | 0 issues, both flavors (darwin and `GOOS=linux` with the integration tag) |
| `go test ./internal/component/sysrib/...` (via `./le job run`) | `ok github.com/ze-software/ze/internal/component/sysrib 1.518s`, `ok .../sysrib/events 0.351s` |
| The seven planned unit tests plus the replay pair | 8 of 8 PASS |
| `internal/component/cli`, `internal/component/config` | `ok`, both named tests PASS |
| `internal/plugins/fib/kernel` in the QEMU guest | 68 top-level tests, 88 including subtests, 0 failures |
| Both `.ci` scenarios in the QEMU guest | PASS, 1.5s and 2.2s |
| `go vet` over `./internal/... ./cmd/...` with `ze_core ze_bgp ze_test` | One finding, `internal/core/textbuf/textbuf.go:186: possible misuse of unsafe.Pointer`, pre-existing and unrelated |

One caveat is recorded rather than hidden: the QEMU guest built its DUT from the
WORKING TREE, which carries other sessions' uncommitted edits, and not from
`000e70eec` alone. The `.ci` results therefore prove the feature works in a tree
that CONTAINS the feature; they do not isolate it from every other uncommitted
hunk in the checkout.

## Core Insight

`resolvedNH` was a next-hop CACHE, and this is the first code in the package to
ask it a different question: has Ze programmed this prefix? A cache answers
"what did I last resolve", and reading that as "what did I last install" is
right almost everywhere, which is why the struct comment asserted it and why
nine of the twenty-four review findings trace to it. `resolveNextHop` returns
an invalid address unchanged, so an interface-only next-hop is programmed while
its entry is invalid, and no validity test can separate that from absent.

The lesson generalises past this package: when a new caller asks an old
structure a question it was not built to answer, the structure keeps answering.
`programmedByZe` is that question given a name and a single implementation, and
naming it is what let round 3 walk the callers instead of fixing the site the
reviewer happened to point at.
