# Spec: policyroute-interface-list-matches-no-packet

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.**
`internal/plugins/policyroute/yang/ze-policyroute-conf.yang` declares
`leaf-list interface` under a policy with the description "Ingress interface(s)
to match. A trailing '*' enables prefix (wildcard) matching (e.g. 'l2tp*')." The
node is a `leaf-list` and the description writes the plural, so a reader takes
two entries to mean the policy matches traffic arriving on either interface.
That is the only reading a leaf-list of match criteria supports, because a
packet arrives on exactly one interface.

**What Ze does instead.** Two entries produce a policy that matches no packet.
`buildMatches` (`internal/plugins/policyroute/translate.go`) loops over the
interfaces and appends one `firewall.MatchInputInterface` per entry into the
same `[]firewall.Match`, which the caller puts into a single `firewall.Term`
alongside the rule's other matches. `lowerTermForNFProto`
(`internal/plugins/firewall/nft/lower_linux.go`) then lowers that term into one
nftables rule: it ranges over `term.Matches`, calls `lowerMatch` on each, and
concatenates the returned expressions into one expression list. An nftables rule
ANDs its matches, so the rule asks for a packet whose input interface is both
names at once. No packet satisfies it, so the policy never fires and the
operator's traffic follows the main table instead of the policy route.

This fails silently. The commit succeeds, `show configuration` reads the entries
back, and the rule is installed in the kernel. Nothing logs a warning, and the
only symptom is traffic that does not take the policy route.

**What closing it means.** The implementer chooses between building the behavior
and refusing the leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. Neither is obviously
right, and the refusal available here is narrower than the VRF one, because a
single entry works: the refusal would be a validator that rejects a second
entry, which turns a silent wrong answer into a loud one without giving the
operator the OR the schema declares. Building it means emitting one term per
named interface rather than one term carrying every name, so the nftables rules
OR by being separate rules. The design has to establish what that does to rule
ordering and to the `order` leaf the policy already carries, because one policy
would then produce several rules where it produces one today. The leaf's own
`ze:help` states the current defect and tells the operator to write one policy
per interface, which is the manual form of exactly that fix.

## Progress (2026-09-06, session paused)

An implementation agent ran against this spec and was STOPPED mid-work when the
session hit its budget. What follows is the state of the tree at that moment.

**The evidence for "compiles" or "does not compile" below is the editor's
compiler diagnostics observed as the agents were stopped, not a build this
session ran to completion.** Re-check before you trust it.

**State: least progress of the batch.** The agent spent its time blocked on the
shared checkout not compiling (other sessions' in-flight edits) and was moving to
a throwaway worktree when it stopped. The package type-checks, so little or
nothing was changed here.

**Next step:** start it fresh. Before inventing a shape, read how the firewall
lowering represents alternatives: the fix may be one nft rule per interface
rather than one rule with many matches.

## Progress (2026-09-08, design phase)

**The product fix LANDED before this design phase started.** Commit
`df5b6c25af` ("fix(policyroute): an interface leaf-list becomes one term per
interface") replaced `buildMatches` with `ruleTerms` in
`internal/plugins/policyroute/translate.go` and added two unit tests to
`translate_test.go`. Commit `76ce2d39a8` registered the functional-test driver
`policy/policy-interface-list` in `internal/test/fixture/netfilter_fixture.go`.

So the Task section above, and steps 3, 5 and 6 of the Transformation Path
below, describe the tree BEFORE `df5b6c25af`. They are kept because they record
the defect this spec exists to close. The Current Behavior section describes the
tree as it stands now, and the rest of this spec is the work that remains.

**What remains: no product Go.** The `.ci` functional test the fixture was
registered for was never written, and the fix left four prose surfaces stating
the defect as current behavior. The YANG `ze:help` on the `interface` leaf-list
still tells the operator to write one policy per interface.

## Progress (2026-09-08, implementation phase)

Everything the design phase named as remaining is done, and none of it was
product Go. The `.ci` exists and passes, the vacuity walk forced and recorded its
RED, the `order` unit test exists, the false sentence in the `termName` comment
is corrected, and the four prose surfaces state what the daemon does.

Two things this phase did NOT do are in Work Not Done, and neither is a scope
reduction the author took: one is a file the phase was not allowed to edit, and
one is an assertion nobody wrote. Closure is `/ze-close`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/policyroute/policy-routing.md` - the page `translate.go` declares in its `// Design:` header
  → Decision: every policy merges into ONE nftables table `ze_pr` (filter, prerouting, priority -150) with one chain, and terms are "named `<policy>-<rule>`". A rule that names several interfaces now installs several terms, so that naming sentence is wrong and the page edit belongs to this work.
  → Constraint: `firewall.RegisterTables("policy-routes", tables)` followed by `firewall.ApplyAll()` is the whole route a non-firewall plugin has to the kernel. The registry merges every owner, so no owner's apply deletes another owner's tables.
- [ ] `docs/guide/policy-routing.md` - the operator page for the `interface` leaf-list
  → Constraint: the "Interface binding" section says "The interface match is prepended to every rule in the policy". That is true of the match and silent on the rule count, so it reads as one rule for the whole policy. It has to state that each named interface gets its own rule.
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - the ownership rules of the shared table registry
  → Constraint: Rule 2, a registered table name carries the `ze_` prefix, and `RegisterTables` refuses one that does not. `ze_pr` carries it, so changing how many terms sit inside the table needs no registry change at all.
- [ ] `docs/functional-tests.md` - the `policy` suite and its isolation rules
  → Constraint: every test under `test/policy/` writes the SAME global kernel objects, one `inet ze_pr` table and one global ip-rule table, so the suite runs serial with a per-test namespace (`Name: "policy"`, `Concurrency: serial`, `Namespace: perTest` in `internal/le/qemu/alltests.go`). A new case joins that suite and MUST NOT raise its parallelism.

### RFC Summaries (Scope: protocol)
N/A. Scope is `plugin`. No RFC governs a YANG leaf-list or an nftables lowering.

**Key insights:** (minimal context to resume after compaction)
- nftables ANDs the expressions inside one rule and offers no branch inside one. Alternatives are separate rules. That single fact decides the whole fix shape.
- A term reaches the kernel in slice order (`programChain`, `internal/plugins/firewall/nft/backend_linux.go:230`), and the term name travels as `Rule.UserData` (`:261`), never as a kernel object name.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/policyroute/translate.go` - `translate` builds one `ze_pr` table and one filter chain; `translatePolicy` walks the rules of a policy; `ruleTerms` returns ONE term for each named interface, each carrying that interface's `firewall.MatchInputInterface` first and the rule's own matches after, and ONE term with no interface match where the policy names none; `termName` keeps `<policy>-<rule>` for a single interface and appends the interface's 1-based position for several.
- [ ] `internal/plugins/policyroute/config.go` - `parsePolicyConfig` sorts the policies by name (`slices.Sort(policyNames)`), `parsePolicyRoute` accepts the leaf as either a string or a list and calls `parseIfaceSpec` on each entry, `parseIfaceSpec` strips a trailing `*` and sets `Wildcard` PER ENTRY, and `parsePolicyRoute` sorts the rules by `Order` then by name.
- [ ] `internal/plugins/policyroute/register.go` - `applyPolicies` calls `translate`, `firewall.RegisterTables("policy-routes", ...)`, `firewall.ApplyAll()`, then `rm.applyAll`; `formatPolicies` renders `show policy routes` from the parsed config and re-appends the `*` to a wildcard entry.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerTermForNFProto` concatenates every match's expressions into ONE expression list; `lowerMatch` dispatches `MatchInputInterface` to `lowerIfaceMatch`; `lowerIfaceMatch` emits `expr.Meta{Key: MetaKeyIIFNAME}` plus `expr.Cmp`, sending the unpadded name for a wildcard and the 16-byte IFNAMSIZ-padded name for an exact match.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `programChain` programs the terms in slice order and writes the term name into `Rule.UserData`; `mergeRuleCounters` sums the counters of every rule sharing one name into a single `firewall.TermCounter` row.
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` checks only the `ze_` table-name prefix and stores the owner's tables; `ApplyAll` merges every owner and calls the backend under one process-wide lock. Neither calls `ValidateTables`.
- [ ] `internal/component/firewall/validate.go` - `validateTerm` calls `ValidateName(term.Name)`, and `ValidateTables` has exactly two callers, both in `internal/component/firewall/engine.go` over the firewall engine's OWN parsed `cfg.Tables`. A term registered through `RegisterTables` therefore never reaches that check.
- [ ] `internal/component/firewall/model.go` - `MatchInputInterface{Name, Wildcard}`, and `ValidateName` delegating to `naming.ValidateNodeName` with a 255-byte limit and an alphanumeric, hyphen, underscore and dot character set.
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - the `leaf-list interface` description promising the plural and the trailing `*`, and three `ze:help` texts that still describe the pre-fix behavior.
- [ ] `internal/test/fixture/netfilter_fixture.go`, `internal/test/fixture/netfilter_fixture_policy.go` - `Register("policy/policy-interface-list", policyRuleDump)` at `netfilter_fixture.go:58`, and `policyRuleDump` printing the `ze_pr` table plus one `RULE <line>` per programmed rule once the table carries at least two rules.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A policy naming ONE interface installs one term named `<policy>-<rule>`, exactly as `docs/guide/policy-routing.md` and the YANG `ze:help` describe it today.
- A policy naming NO interface installs one term with no interface match, matching every ingress interface.
- The trailing `*` wildcard stays a per-entry property and stays a prefix compare on the wire.
- One `table N` or `next-hop` rule allocates ONE fwmark and ONE ip rule, whatever the interface count.
- The chain order stays: policies by name, then rules by `order` and name.
- `show policy routes` output is unchanged. It renders the parsed config, not the terms.

**Behavior to change:** (only what the user asked for)
- None in Go. The product change landed in `df5b6c25af`. This spec adds the functional test that proves the operator reaches the behavior, and repairs the prose that still states the defect as current behavior.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config. The `interface` leaf-list under `policy > route <name>`
  (`internal/plugins/policyroute/yang/ze-policyroute-conf.yang`), committed by
  the operator with two or more entries.
- The subtree reaches the plugin process because `runPolicyRoutePlugin`
  (`internal/plugins/policyroute/register.go`) registers `ConfigRoots` and
  `WantsConfig` as `configRoot` (`"policy"`).
- Format at entry: `[]sdk.ConfigSection` at `p.OnConfigVerify` and
  `p.OnConfigure`, each section's `Data` holding the `policy` subtree. Both
  callbacks skip a section whose `Root` is not `configRoot`.
- A second packet-side entry exists and is NOT where the defect lives: the
  installed nftables rule is evaluated by the kernel on every ingress packet.
  That is where the operator observes the symptom.

### Transformation Path
1. `parsePolicyConfig` turns the section data into `[]PolicyRoute`, one per
   `route` list entry, carrying the interface entries as a slice.
2. `applyPolicies` (`register.go`) calls `(*allocator).translate`, which calls
   `(*allocator).translatePolicy` per policy
   (`internal/plugins/policyroute/translate.go`).
3. The per-rule match builder appends one `firewall.MatchInputInterface` per
   interface entry into ONE `[]firewall.Match`, and that slice becomes the
   `Matches` field of a single `firewall.Term`. This is the stage the defect
   lives at: an OR in the schema becomes several matches on one term.
4. `applyPolicies` hands the resulting tables to `firewall.RegisterTables`
   under the owner name `policy-routes`, then calls `firewall.ApplyAll`.
5. `lowerTermForNFProto` (`internal/plugins/firewall/nft/lower_linux.go`)
   ranges over `term.Matches`, calls `lowerMatch` on each, and concatenates
   every returned expression into ONE expression list, which becomes one
   nftables rule.
6. nftables ANDs the expressions of a rule, so the installed rule asks for an
   input interface equal to both names at once. No packet satisfies it, and the
   traffic follows the main table.
7. `rm.applyAll` installs the ip rules and tables, so the routing side is built
   and reachable. Only the rule that would mark the packet never fires.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | The `policy` config subtree arrives as `[]sdk.ConfigSection` at `OnConfigVerify` and `OnConfigure`, each `Data` holding `map[string]any`. The `interface` leaf-list arrives as `[]any` of `string`, or as a bare `string` for one entry, and `parsePolicyRoute` reads both shapes | Yes |
| Plugin ↔ firewall component | In-process Go call, value types only: `firewall.RegisterTables("policy-routes", []firewall.Table)` then `firewall.ApplyAll()`. No pointer crosses | Yes |
| firewall component ↔ kernel | The nft backend lowers each `firewall.Term` to nftables expressions over netlink, one rule per term per address family, with the term name in `Rule.UserData` | Yes |
| Plugin ↔ kernel | `rm.applyAll` adds the ip rules and the auto-allocated routes over netlink. Untouched by this spec: one rule keeps one mark and one ip rule whatever its interface count | Yes |
| Test ↔ kernel | The `.ci` reads the programmed state back with `nft list table inet ze_pr` through the `policy/policy-interface-list` fixture, and asserts on the per-rule `RULE` lines it prints | Yes |

### Integration Points
- `ruleTerms` and `termName` (`internal/plugins/policyroute/translate.go`) - the producers of the per-interface terms. Reached only through `translatePolicy`, which is reached only through `translate`.
- `firewall.RegisterTables` and `firewall.ApplyAll` (`internal/component/firewall/registry.go:97,119`) - the registry the plugin hands its tables to. It checks the table-name prefix and nothing else, so a term count change needs no registry change.
- `lowerIfaceMatch` (`internal/plugins/firewall/nft/lower_linux.go:745`) - lowers each term's single `MatchInputInterface`. It is where the wildcard becomes a prefix compare, and it is unchanged by this spec.
- `mergeRuleCounters` (`internal/plugins/firewall/nft/backend_linux.go:381`) - sums the counters of every kernel rule sharing a term name into one row. It is the reason two terms of one group MUST NOT share a name.
- `fixture.Register("policy/policy-interface-list", policyRuleDump)` (`internal/test/fixture/netfilter_fixture.go:58`) - the functional-test driver, already registered and inert until a `.ci` names it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config to `parsePolicyConfig` to `translate` to `RegisterTables`/`ApplyAll` to the nft backend. The `.ci` drives the whole chain from a config file and reads the kernel back |
| No unintended coupling (components stay isolated) | Yes | The plugin names no nft type. It builds `firewall.Term` values and hands them to the registry; the backend owns the lowering |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ruleTerms` reuses the existing `firewall.MatchInputInterface` and the existing term list. The test reuses `policyRuleDump`, already registered, rather than a new driver |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config-plane translation, run once per commit. `ruleTerms` clones the actions slice per term on purpose: two terms sharing one backing array would alias a mutation |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them (`ai/rules/plugins.md`) | Yes | The fixture registers by name in the plugin-side `init()`, and `fixture.Run` dispatches by name. No central list enumerates the policy fixtures |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | nftables ANDs the expressions inside one rule and ORs across rules, so separate terms are the only way to express an alternative | `lowerTermForNFProto` concatenates every match's expressions into one list (`lower_linux.go:366`), and `programChain` writes one kernel rule per term | The fix shape is wrong and the whole spec restarts at RESEARCH | `test/policy/policy-interface-list.ci` asserting two `RULE` lines, one naming each interface, and no line naming both | confirmed. The `.ci` passes with both `expect=stdout:pattern=RULE .*iifname "..."` lines and both `reject=stdout:pattern` lines green (9/9 steps, QEMU verbose run 2026-09-08) |
| A-2 | An `iifname` match does not need the interface to exist when the rule loads | `lowerIfaceMatch` emits `expr.Meta{Key: MetaKeyIIFNAME}` plus `expr.Cmp`, a compare against packet metadata rather than an index lookup, and `test/policy/policy-boot-apply.ci` already loads `iifname "l2tp*"` with no such interface present | The `.ci` needs `option=netns-link` for each name, which puts it in the netns-only population and changes the count `docs/functional-tests.md` publishes | The `.ci` passing with no `netns-link` option | confirmed. The `.ci` declares no `option=netns-link` and passes in a guest where neither `eth0` nor any `l2tp*` interface exists |
| A-3 | A term registered through `RegisterTables` is never checked by `firewall.ValidateName` | `ValidateTables` has exactly two callers, `engine.go:216` and `engine.go:321`, both over the firewall engine's own `cfg.Tables`. `RegisterTables` checks the table-name prefix only | The `termName` comment's stated reason is right as written, and the position suffix keeps the same justification either way | `grep -rn "ValidateTables(" internal/component/firewall internal/plugins/firewall` returning those two call sites and no third | confirmed. `ValidateTables` has exactly two non-test callers, `engine.go:216` and `engine.go:321`, both over `cfg.Tables`. `ValidateName(term.Name)` is reached only from `validateTerm` (`validate.go:88`), and `RegisterTables` checks the `ze_` prefix alone. The `termName` comment is corrected in this work |
| A-4 | The chain order is fixed before translation: policies by name, then rules by `order` then name | `slices.Sort(policyNames)` (`config.go:75`) and `sort.Slice` on `Order` then `Name` (`config.go:126`) | The `order` acceptance criterion tests the wrong thing | `TestPolicyInterfaceListKeepsRuleOrder` asserting the four term names in sequence | confirmed. The test drives `parsePolicyConfig` with the `order 10` rule written first and reads `steer-early-1`, `-2`, `steer-late-1`, `-2` back |
| A-5 | Nothing reorders `Chain.Terms` between `translate` and the kernel | `programChain` iterates `chain.Terms` by index (`backend_linux.go:230`) | A per-interface group could be split by another rule's terms, and `order` would stop meaning what the help says | The same unit test, plus the `.ci` asserting the `RULE` lines in sequence | confirmed. `TestPolicyInterfaceListKeepsRuleOrder` reads the four terms back in slice order after `translate` |
| A-6 | `show policy routes` is unaffected | `formatPolicies` (`register.go`) renders the parsed `[]PolicyRoute`, never the terms | `test/plugin/policy-routes-show.ci` goes red and the show path joins this spec | broken as a validation METHOD, holds as a fact. `formatPolicies` (`register.go:248`) reads `[]PolicyRoute` and never a term, so no code this spec touches reaches the show path. `test/plugin/policy-routes-show.ci` was NOT run: it times out in the ad-hoc QEMU guest this session used, waiting on `ze cli`, before any assertion. The gate must run it | confirmed at the producer, unrun as a test |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `show firewall ruleset` gains one counter row per interface for a multi-interface rule, so an operator reading per-rule totals now reads per-interface totals | A firewall show test asserting a row count goes red | This is the correct answer, not a regression: the kernel counts each rule separately and `mergeRuleCounters` can only sum rows that share a name. Named in Known Limitations rather than hidden by giving the group one name |
| R-2 | The term name `<policy>-<rule>-<N>` has no length bound, and nothing on the registry path validates it (A-3) | None today. A 255-byte overflow would surface only if a future change routes registry tables through `ValidateTables` | Pre-existing for `<policy>-<rule>` and not made materially worse by the suffix. Recorded in Known Limitations, not fixed here |
| R-3 | The new `.ci` shares `inet ze_pr` and the global ip-rule table with the five existing policy tests | A flaky red under a parallel run | The suite is already serial with a per-test namespace (`alltests.go`). The new test adds no `option=netns-link` and raises no parallelism |
| R-4 | The `.ci` passes against the broken code and proves nothing | The vacuity check finds it green after the revert | `policyRuleDump` waits for at least two rules in `ze_pr`. The pre-fix code emits ONE term for the one-rule config, so the fixture times out and the test goes red. The revert walk is recorded in the Goal Validation evidence |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing new: the product code already shipped. A wrong `.ci` either passes vacuously, which leaves the behavior unproven, or fails and blocks the policy suite. A wrong prose repair leaves the operator reading a `ze:help` that contradicts the daemon |
| How is it reverted? | Single commit revert. The `.ci` and the prose are additive, no config migration, and the leaf-list syntax is identical before and after |
| Who else touches this path? | Any spec changing `translate.go` term naming or the firewall show path. `test/policy/` is shared by five other `.ci` tests, and `internal/plugins/firewall/nft/` is shared with copp, ddos-local, firewall-irr, the FlowSpec bridge and anomaly-shape through the one table registry |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A committed config with two `interface` entries under one `policy > route` | → | `ruleTerms` (`translate.go`), reached through `applyPolicies` and `translate`, then `firewall.RegisterTables` / `ApplyAll` and `programChain` | `test/policy/policy-interface-list.ci` |
| The same config, read back from the kernel | → | `lowerIfaceMatch` (`lower_linux.go`) writing one `iifname` compare per rule | `test/policy/policy-interface-list.ci`, asserting `RULE` lines that each name exactly one interface |
| `[]PolicyRoute` handed to `translate` | → | `termName` (`translate.go`) naming the terms of a group | `TestPolicyInterfaceListOneTermPerInterface` (`internal/plugins/policyroute/translate_test.go`) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | A policy names two interfaces and carries one rule | Two nftables rules are installed. One names the first interface, one names the second, and neither names both. Traffic arriving on either interface takes the policy route | `test/policy/policy-interface-list.ci`, steps 6-9 of the verbose QEMU run: `RULE .*iifname "eth0".*meta mark set` and `RULE .*iifname "l2tp\*".*meta mark set` both matched, and both `reject=stdout:pattern` lines found no rule naming both. Plus `TestPolicyInterfaceListOneTermPerInterface` |
| AC-2 | A policy names ONE interface and carries one rule | Exactly one nftables rule is installed, and its term keeps the name `<policy>-<rule>` with no position suffix | `TestMultiplePoliciesMergedIntoOneTable`, which reads back the unsuffixed `alpha-r1` and `beta-r1`, and `termName` returning `base` when `count == 1` |
| AC-3 | A policy names NO interface | Exactly one nftables rule is installed for each of its rules, carrying no interface match, so it matches every ingress interface | `TestPolicyWithoutInterfaceMatchesEveryIngress`: one term named `any-drop-udp` and zero interface matches |
| AC-4 | A policy names `eth0` and `l2tp*` in one leaf-list | The `eth0` rule compares the full 16-byte interface name, and the `l2tp*` rule compares the 4-byte prefix. Each entry keeps its own wildcard state, and mixing the two forms in one list is accepted | `TestPolicyInterfaceListOneTermPerInterface` asserts `{Name: "eth0"}` and `{Name: "l2tp", Wildcard: true}` on their own terms. The `.ci` proves it at the kernel: nft prints `iifname "eth0"` for the 16-byte compare and `iifname "l2tp*"` for the prefix compare, on two separate rules |
| AC-5 | A policy names two interfaces and carries two rules, `order 0` and `order 10` | Four nftables rules are installed. Both rules of the `order 0` group precede both rules of the `order 10` group, so the `order` leaf keeps deciding evaluation sequence | `TestPolicyInterfaceListKeepsRuleOrder`: `parsePolicyConfig` is given the `order 10` rule first, and the four terms read back `steer-early-1`, `steer-early-2`, `steer-late-1`, `steer-late-2` |
| AC-6 | A policy names two interfaces and one rule selects `table 100` | One fwmark is allocated and one ip rule maps it to table 100. Both nftables rules set that same mark | `TestPolicyInterfaceListOneTermPerInterface` asserts both terms carry the same `SetMark` value and that it equals `result.IPRules[0].Mark`, with `len(result.IPRules) == 1`. The `.ci` asserts `meta mark set` on both `RULE` lines |
| AC-7 | A multi-interface policy is applied and `show firewall ruleset` is read | Each interface's rule is reported as its own counter row. No two rules of the group are summed into one row | `termName` gives the two terms distinct names, so `mergeRuleCounters` (which merges only on an equal name) cannot sum them. Not asserted through `show firewall ruleset` in this work: see Work Not Done |
| AC-8 | An interface name carrying a character a term name may not carry, for example `l2tp*` | The apply succeeds. The term name carries the interface's position, never the interface string, so no interface name can reach a name check | `termName` builds the suffix from `strconv.Itoa(index+1)`, so no operator string enters a term name. The `.ci` commits `interface "l2tp*"` and the apply succeeds (`policy routes applied count=1` in the guest log) |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `interface eth0;` and `interface "l2tp*";` under one policy and commits | config file → `parsePolicyConfig` → `parseIfaceSpec` per entry → `translate` → `ruleTerms` → `RegisterTables` → `ApplyAll` → `programChain` → two kernel rules | `test/policy/policy-interface-list.ci` |
| 2 | Sends traffic that matches the policy on the SECOND named interface | ingress packet → `ze_pr` prerouting chain → the second rule's `iifname` compare → `meta mark set` → ip rule → the policy's routing table | `test/policy/policy-interface-list.ci` asserting `meta mark set` on both `RULE` lines |
| 3 | Reads `show policy routes` after committing a two-interface policy | plugin command → `formatPolicies` over the parsed config | `test/plugin/policy-routes-show.ci` (existing, must stay green) |
| 4 | Reads `show firewall ruleset` to see which interface is carrying the traffic | nft backend → `mergeRuleCounters` → one row per term name | AC-7, covered by the counter assertion in `test/policy/policy-interface-list.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPolicyInterfaceListOneTermPerInterface` | `internal/plugins/policyroute/translate_test.go` | AC-1, AC-4, AC-6, AC-8: two terms, one interface match each, the interface match first, distinct names `wan-mark-1` and `wan-mark-2`, both carrying the one shared mark that the one ip rule looks up | exists (`df5b6c25af`) |
| `TestPolicyWithoutInterfaceMatchesEveryIngress` | `internal/plugins/policyroute/translate_test.go` | AC-3: one term, no interface match | exists (`df5b6c25af`) |
| `TestMultiplePoliciesMergedIntoOneTable` | `internal/plugins/policyroute/translate_test.go` | AC-2 regression: one interface per policy keeps the unsuffixed names `alpha-r1` and `beta-r1` | exists |
| `TestPolicyInterfaceListKeepsRuleOrder` | `internal/plugins/policyroute/translate_test.go` | AC-5: two interfaces and two rules at `order 10` and `order 0` produce four terms in the sequence `p-a-1`, `p-a-2`, `p-b-1`, `p-b-2`, with the `order 0` rule's group first and neither group split | written, passing |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interface leaf-list entry count | 0..unbounded | N-A, no upper bound is declared | N-A | N-A |
| term name length (`<policy>-<rule>-<N>`) | 1..255 by `ValidateName` | 255 | 0 | 256, unreachable on this path because `RegisterTables` never validates a term name (A-3, R-2) |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `policy-interface-list` | `test/policy/policy-interface-list.ci` | An operator names two ingress interfaces on one policy, commits, and both interfaces steer traffic. The test asserts two `RULE` lines, one naming `eth0` and one naming `l2tp*`, and REJECTS any single `RULE` line naming both | written, passing, and proven to discriminate (Vacuity Walk below) |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is `plugin`. No wire protocol and no peer daemon: the counterpart is the Linux kernel, and the `.ci` reads the programmed nftables state back from it | N-A |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - three `ze:help` texts state the pre-fix behavior. The `leaf-list interface` help says "Name ONE interface" and tells the operator to write one policy per interface; the `list rule` help says "Each rule becomes one nftables term named `<policy>-<rule>`"; the route `leaf name` help says the name prefixes every term "as `<policy>-<rule>`". Each states what the daemon now does: one term per named interface, named `<policy>-<rule>` for one interface and `<policy>-<rule>-<N>` for several
- `internal/plugins/policyroute/translate.go` - the `termName` doc comment justifies the position suffix partly on "a term name that fails validation takes down the apply of every firewall owner". A term registered through `RegisterTables` never reaches `ValidateName` (A-3), so that sentence is wrong. The counter-merge reason is correct and is the one to keep, beside the plain point that an interface name in a term name is noise the operator did not ask for
- `internal/plugins/policyroute/translate_test.go` - add `TestPolicyInterfaceListKeepsRuleOrder`
- `docs/architecture/policyroute/policy-routing.md` - "terms named `<policy>-<rule>`" and "The interface wildcard is prepended to every rule" both describe one term per rule. State one term per named interface and the naming rule. This page is declared by `translate.go`'s `// Design:` header
- `docs/guide/policy-routing.md` - the "Interface binding" section states the OR the operator gets, and that each named interface becomes its own nftables rule
- `docs/features.md` - the Policy Routing row says "Interface wildcard binding (e.g., `l2tp*`)" and is silent on several interfaces. Add that a policy naming several interfaces matches traffic on any of them
- `docs/functional-tests.md` - the `test/policy` suite gains a case; check the netns-link population sentence only if the new `.ci` ends up declaring `option=netns-link` (it should not, per A-2)

## Files to Create
- `test/policy/policy-interface-list.ci` - the functional test the already-registered `policy/policy-interface-list` fixture exists to drive

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`, `ze:help` text only. No leaf is added, removed, or retyped, so no migration and no completion change |
| YANG validation constraints | No | The leaf-list stays `type string`. A `pattern` cannot express an interface name plus an optional trailing `*` more usefully than the parser already does, and no numeric or enumerated leaf is touched |
| YANG custom validators | No | Deliberately not added. See the Key Design Decisions row rejecting the refusal validator: the schema declares a leaf-list and the operator is owed the OR |
| CLI commands/flags | No | No command is added or changed. `show policy routes` renders the parsed config through `formatPolicies` and is unaffected (A-6) |
| CLI grammar (keyword before value) | N-A | No command is added |
| Editor autocomplete | No | The leaf is a free string with no `CompleteFn`, unchanged |
| Functional test for new RPC/API | Yes | `test/policy/policy-interface-list.ci` |
| Pipe completeness | N-A | No command output is added |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | No | No new file path, socket, service, module, port, sysctl, netlink surface, binary or certificate. The nft backend and its netlink use already carry their checks |
| Prometheus counters/metrics | No | The firewall apply metrics (`ze_firewall_apply_duration_seconds`, `ze_firewall_apply_timeout_total`) already cover this apply path unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, the Policy Routing row: several named interfaces are an OR |
| 2 | Config syntax changed? | No | The leaf-list syntax is identical. Its MEANING is now what the schema always declared, which is a behavior repair rather than a syntax change. `docs/guide/configuration.md` only links to the guide and needs no edit |
| 3 | CLI command added/changed? | No | No command added or changed |
| 4 | API/RPC added/changed? | No | `docs/architecture/api/commands.md` anchors `cmd_show.go`, which is untouched |
| 5 | Plugin added/changed? | No | `docs/guide/plugins.md` anchors `register.go` for the registration, which is untouched |
| 6 | Has a user guide page? | Yes | `docs/guide/policy-routing.md`, the "Interface binding" section |
| 7 | Wire format changed? | N-A | No wire format |
| 8 | Plugin SDK/protocol changed? | No | No SDK type or transport change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is `plugin`, no RFC governs it |
| 10 | Test infrastructure changed? | No | The fixture and its driver already exist (`netfilter_fixture.go:58`). Only a `.ci` is added, and `docs/functional-tests.md` names the suite rather than each file. The netns-link count sentence stays correct while the new test declares no `option=netns-link` (A-2) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not carry a policy-routing interface row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/policyroute/policy-routing.md`, the term naming and the one-term-per-rule statement |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | The plugin's registration, commands and events are unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by grep over `docs/` for anchors naming the changed files. `translate.go` DECLARES `docs/architecture/policyroute/policy-routing.md` in its `// Design:` header, and that page is named above, so the block is satisfied. Advisory anchors: `docs/functional-tests.md` cites `translate.go` for `policyRoutingTable = "ze_pr"`, unaffected; `docs/guide/configuration.md` and `docs/guide/policy-routing.md` cite the YANG for the schema link and the `table` leaf range, both unaffected. Re-derive with `./le spec citation anchors spec plan/immediate/spec-policyroute-interface-list-matches-no-packet.md` before closure |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/policy-routing.md` already shows a two-entry example (`interface "l2tp*"; interface eth1;`) under prose that reads as one rule. The example is correct and the prose around it is what changes |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the operator reaches the behavior from a config file
   - Tests: `test/policy/policy-interface-list.ci`
   - Files: `test/policy/policy-interface-list.ci`. The driver is already registered at `internal/test/fixture/netfilter_fixture.go:58`, so no Go is written here
   - Shape: `option=needs-linux:caps=net-admin`, no `option=netns-link` (A-2), `firewall { backend nft; }`, one `policy > route` naming `eth0` and `"l2tp*"` with one `table 100` rule, the two `ze.log.*` env options the sibling tests set, then `cmd=foreground:exec=ze-test fixture policy/policy-interface-list`
   - Assertions: `expect=stdout:contains=table inet ze_pr`; one `expect=stdout:pattern=RULE .*iifname "eth0"`; one `expect=stdout:pattern=RULE .*iifname "l2tp\*"`; and the discriminating `reject=stdout:pattern=RULE .*eth0.*l2tp`, which is what tells an OR across two rules from an AND inside one. A whole-table dump cannot tell them apart, which is why `policyRuleDump` prints per-rule lines
   - Verify: run it. It must PASS on the current tree, because the product code already landed. That is not the wiring proof on its own, so Phase 2 supplies the red
2. **Phase: Vacuity walk (BLOCKING, `ai/rules/interop-and-goal-validation.md`)** -- force the red the test never had
   - Method: revert `ruleTerms` to the pre-fix shape (`git show df5b6c25af^:internal/plugins/policyroute/translate.go` gives the old `buildMatches`), rebuild the daemon so the revert takes effect, run the `.ci`, record that it goes RED and how it fails, restore, confirm GREEN
   - Expected red: `policyRuleDump` waits for at least two rules in `ze_pr`. The old code emits ONE term for a one-rule config, so the fixture fails with "policy rules were not programmed" rather than an assertion mismatch. Record that message, because a test that only ever passed proves nothing
   - Verify: the RED output is pasted into the spec's TDD checklist and into the Goal Validation evidence
3. **Phase: the `order` unit test** -- pin what the interface groups do to evaluation sequence
   - Tests: `TestPolicyInterfaceListKeepsRuleOrder`
   - Files: `internal/plugins/policyroute/translate_test.go`
   - Shape: one policy, two interfaces, two rules given in the config with `order 10` before `order 0`. Assert four terms in the sequence `<policy>-<order0rule>-1`, `-2`, `<policy>-<order10rule>-1`, `-2`, so the group is contiguous and the `order` leaf still decides which group runs first
   - Verify: it fails first against a deliberately wrong expectation, then passes
4. **Phase: the prose repair** -- every surface that still states the defect
   - Files: the three `ze:help` texts in `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`, `docs/architecture/policyroute/policy-routing.md`, `docs/guide/policy-routing.md`, `docs/features.md`, and the `termName` comment in `internal/plugins/policyroute/translate.go`
   - Verify: grep the repository for "one policy per interface" and for "`<policy>-<rule>`" and confirm every hit describes the current behavior. Read each edited sentence against `ruleTerms` and `termName`, not against this spec

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a test named in the TDD plan, and AC-1 through AC-8 are each reachable from a config file rather than from a Go literal alone |
| Feature completeness | The `.ci` drives the whole chain: config file, daemon, kernel, read back. No AC is proven by a unit test only |
| Correctness | The discriminating assertion is the NEGATIVE one. A test that only checks both interface names appear somewhere in the dump passes against the broken code, because two matches in one rule print the same substrings as the same two matches in two rules |
| Naming | A term of a single-interface policy keeps `<policy>-<rule>`. No interface name reaches a term name. The suffix is the 1-based position, and the docs say 1-based |
| Data flow | The interface OR is expressed in `ruleTerms` alone. No nft type is named in the plugin, and `lowerIfaceMatch` still sees exactly one interface per term |
| Rule: `ai/rules/documentation.md` | The prose repair lands in the SAME work as the test, not in a follow-up. The YANG `ze:help` is the surface the operator reads at the CLI, so it is not "just a doc" |
| Rule: `ai/rules/interop-and-goal-validation.md` | The revert walk is performed and its RED output is recorded. The `.ci` was written against already-working code, which is the first vacuity trap |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `test/policy/policy-interface-list.ci` exists and runs | `ls test/policy/policy-interface-list.ci` and `./le job run label functional command ze-test policy` |
| The fixture registration is no longer inert | `grep -n "policy-interface-list" test/policy/policy-interface-list.ci internal/test/fixture/netfilter_fixture.go` returning both |
| The order unit test exists and passes | `go test -run TestPolicyInterfaceListKeepsRuleOrder ./internal/plugins/policyroute/` through the registered `./le job run` action |
| No surface still tells the operator to write one policy per interface | `grep -rn "one policy per interface" internal/ docs/` returning nothing |
| No surface still says a rule becomes one term | `grep -rn "becomes one nftables term" internal/plugins/policyroute/yang/` returning nothing |
| The RED phase is recorded | The reverted-build failure message pasted in the TDD checklist |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The interface name is operator input that reaches a term name. It MUST NOT: `termName` uses the position, so no operator string enters a name any layer might validate or interpret |
| Resource exhaustion | The leaf-list has no declared upper bound, and each entry now costs a kernel rule instead of a match. The multiplier is entries times rules per policy. Config-plane only, applied once per commit, and an operator who can write the config can already write that many policies |
| Fail-open | The failure mode this spec closes was fail-OPEN in the operator's favour and fail-CLOSED in the policy's: a policy that matched nothing let traffic follow the main table silently. The `.ci` is what stops that returning unobserved |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Vacuity Walk (BLOCKING, `ai/rules/interop-and-goal-validation.md`)

`test/policy/policy-interface-list.ci` was written against code that already
worked, which is the first vacuity trap. The walk below forced the red it never
had. Every run boots one QEMU Alpine guest, which compiles `ze` and `ze-test`
from the working tree it is given, so a source revert reaches the daemon under
test.

| Step | Tree | Result |
|------|------|--------|
| 1 | `ruleTerms` as it stands | PASS |
| 2 | `ruleTerms` reverted to the pre-fix `buildMatches` shape (`git show df5b6c25af^:internal/plugins/policyroute/translate.go`) | FAIL |
| 3 | fix restored, `git status` clean on `translate.go` | PASS |

### Step 1, GREEN before the revert

```
═══ policy ════════════════════════════════════════════════════════════════════
502ms     0/1      1/1  running  2(500ms)
979ms    1/1  PASS  2  policy-interface-list
pass  1/1  100.0%  982ms
QEMU VM: PASS
```

### Step 2, RED under the reverted code

The daemon applied the policy and installed ONE rule, so the fixture's
"at least two rules in `ze_pr`" predicate never held and it failed on the
observer error rather than on an assertion mismatch. That is what R-4 predicted.

```
6.5s     1/1  FAIL  2  policy-interface-list
fail  0/1  0.0%  6.5s  failed 1 [2]
TEST FAILURE: 2 policy-interface-list
TYPE:    logging_mismatch

ERROR:
observer reported runtime failure: time=runtime level=ERROR msg="ZE-OBSERVER-FAIL: policy rules were not programmed" subsystem=test.observer

CLIENT OUTPUT:
time=2026-09-08T13:35:11.877Z level=INFO msg="firewall backend loaded" subsystem=firewall backend=nft
time=2026-09-08T13:35:11.880Z level=INFO msg="firewall config applied" subsystem=firewall tables=0
time=2026-09-08T13:35:11.889Z level=INFO msg="policy routes applied" subsystem=policy.routes count=1
time=runtime level=ERROR msg="ZE-OBSERVER-FAIL: policy rules were not programmed" subsystem=test.observer
```

The unit tests went red on the same tree, and they name the count directly:

```
--- FAIL: TestPolicyInterfaceListOneTermPerInterface (0.00s)
    translate_test.go:288: expected 2 terms (one per interface), got 1
--- FAIL: TestPolicyInterfaceListKeepsRuleOrder (0.00s)
    translate_test.go:414: expected 4 terms (2 rules x 2 interfaces), got 2
FAIL	github.com/ze-software/ze/internal/plugins/policyroute	0.541s
```

### Step 3, GREEN after restoring, with every step named

```
1.9s     1/1  PASS  2  policy-interface-list
pass  1/1  100.0%  1.9s
STEP TRACE: 2 policy-interface-list
    1 ✓ exec /workspace/bin/ze start ...
    2 ✓ exec /workspace/bin/ze-test fixture policy/policy-interface-list
    3 ✓ expect exit-code
    4 ✓ expect stderr-contains
    5 ✓ expect stdout-contains
    6 ✓ expect stdout-regex
    7 ✓ expect stdout-regex
    8 ✓ expect stdout-reject-regex
    9 ✓ expect stdout-reject-regex
QEMU VM: PASS
```

The walk was re-run after the YANG and documentation edits landed, with the same
9-of-9 result, because the daemon parses the changed schema at startup.

Steps 8 and 9 are the discriminating pair. A whole-table dump prints the same
substrings whether the two interface matches share one rule or sit on two, so
the assertion that separates the shapes is the rejection of a single `RULE` line
carrying both names.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An `interface` leaf-list of two entries matches traffic on either interface, rather than no packet at all | functional test over config, daemon and kernel | `test/policy/policy-interface-list.ci`. Two `RULE` lines, one comparing `iifname "eth0"` and one comparing `iifname "l2tp*"`, each carrying `meta mark set`, and no single rule carrying both names. Nine of nine steps pass, and the Vacuity Walk above records the RED under the reverted producer |
| The failure stops being silent: the behavior the schema declares is now the behavior the kernel gets | functional test, discriminating assertion | The two `reject=stdout:pattern` assertions. They are what the old code fails, and they are what a whole-table `contains` assertion would have passed against |
| The `order` leaf keeps deciding evaluation sequence once one rule installs several kernel rules | unit test over the whole config path | `TestPolicyInterfaceListKeepsRuleOrder`. `parsePolicyConfig` is given the `order 10` rule first and the terms come out `steer-early-1`, `steer-early-2`, `steer-late-1`, `steer-late-2` |
| One `table N` rule still allocates one fwmark and one ip rule, whatever the interface count | unit test | `TestPolicyInterfaceListOneTermPerInterface`: `len(result.IPRules) == 1`, both terms carry an equal `SetMark`, and it equals `result.IPRules[0].Mark` |
| No surface still tells the operator the old, wrong thing | grep over the tree | `grep -rn "one policy per interface" internal/ docs/` returns nothing, `grep -rn "becomes one nftables term" internal/ docs/` returns nothing, and `grep -rn "prepended to every rule" internal/ docs/` returns nothing. The only remaining hits are in this spec, where they record the defect |

## Implementation Summary

### What Was Implemented
- `test/policy/policy-interface-list.ci`, the functional test the already-registered `policy/policy-interface-list` fixture existed to drive. It is the first `.ci` to name that fixture, so the registration at `internal/test/fixture/netfilter_fixture.go:58` is no longer inert.
- `TestPolicyInterfaceListKeepsRuleOrder` (`internal/plugins/policyroute/translate_test.go`), driven through `parsePolicyConfig` rather than through `translate` alone, because the sort by `order` lives in `parsePolicyRoute`.
- The `termName` doc comment in `internal/plugins/policyroute/translate.go`, corrected. No behavior change.
- Four prose surfaces: the `interface` leaf-list `description` and its `ze:help`, the `list rule` `ze:help` and the route `leaf name` `ze:help` (`internal/plugins/policyroute/yang/ze-policyroute-conf.yang`); `docs/architecture/policyroute/policy-routing.md`; `docs/guide/policy-routing.md`; `docs/features.md`.

No product Go changed. `df5b6c25af` had already landed it.

### Bugs Found/Fixed
- The `termName` comment justified the position suffix on "a term name that fails validation takes down the apply of every firewall owner". That is false. `ValidateName(term.Name)` is reached only from `validateTerm` (`internal/component/firewall/validate.go:88`), which only `ValidateTables` calls, and `ValidateTables` has two non-test callers (`internal/component/firewall/engine.go:216` and `:321`), both over the firewall engine's own `cfg.Tables`. `firewall.RegisterTables` checks the `ze_` table-name prefix and nothing else. The comment now states the reason that is true and says plainly that no check stands behind the choice.

### Documentation Updates
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`: three `ze:help` texts and one `description`. `./le yang glue check` reports 154 directories current.
- `docs/architecture/policyroute/policy-routing.md`: the page `translate.go` declares in its `// Design:` header. New "One term for each named interface" section. Anchor added: `<!-- source: internal/plugins/policyroute/translate.go -- ruleTerms, termName -->`.
- `docs/guide/policy-routing.md`: the "Interface binding" section. Anchors added for `ruleTerms`/`termName` and for `mergeRuleCounters`.
- `docs/features.md`: the Policy Routing row.
- `docs/functional-tests.md`: NOT edited. It names the `test/policy` suite rather than each file, and the netns-link population sentence stays correct because the new `.ci` declares no `option=netns-link` (A-2).

### Deviations from Plan
- The vacuity walk ran the `.ci` through `./le qemu run` with a single-test guest script rather than through `./le qemu netns-test suites policy`. That action runs a hardcoded list of six test names (`netnsSelections`, `internal/le/qemu/netns_linux.go`), and the new test is not on it. See Work Not Done.
- The guest run is the guest ROOT namespace rather than the per-test namespace the policy suite uses under `all-tests`. The test does not depend on the namespace: it names two interfaces that exist in neither.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| `policy-interface-list` added to `netnsSelections[netnsPolicy]` (`internal/le/qemu/netns_linux.go`), so `./le qemu netns-test suites policy` reaches it | That file was out of the editing scope this phase was given: another session holds uncommitted work under `internal/le/`. `./le qemu all-tests` finds the test with no registration, so only the focused developer loop misses it | Unhomed. Named here for the main thread to route: it is a one-line edit, not a spec |
| AC-7 asserted through `show firewall ruleset` output | The AC is met by construction (`termName` gives the group's terms distinct names, and `mergeRuleCounters` merges only equal names), and the `.ci` proves the two rules exist separately. No test reads the counter rows back | Unhomed. Named here rather than claimed |
| `test/plugin/policy-routes-show.ci` re-run (A-6) | It times out in the ad-hoc QEMU guest this session used, waiting on `ze cli`, before any assertion. `formatPolicies` reads the parsed config and no code this spec touches reaches it, so the fact holds at the producer | The verification gate runs it |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- **A whole-table dump cannot distinguish an AND from an OR.** Two matches in one rule and the same two matches in two rules print the same set of substrings. Any test that asserts "both names appear" passes against both. The per-rule `RULE` lines `policyRuleDump` prints, plus a `reject=stdout` on a line carrying both names, are what make the assertion discriminate. This generalizes to every firewall `.ci` that cares about which matches share a rule.
- **The term name is `Rule.UserData`, not a kernel object name.** The kernel never parses it, and `RegisterTables` never validates it. Its only consumers are `mergeRuleCounters` and the show paths, so the name's whole job is to be the grouping key for the counters. That reframes the naming decision: the question is not "what is a legal name" but "which rules should share a counter row".
- **`ValidateTables` does not cover the registry path.** It runs on the firewall engine's own parsed config, at `engine.go:216` and `:321`. Six of the seven table producers reach the kernel through `RegisterTables` and are validated by nothing but the `ze_` prefix check. A term with no action, or an unsupported match, fails at the backend instead of at the boundary. Not this spec's problem to fix, and worth a journal row of its own.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One term per named interface, so the nftables rules OR by being separate rules | One term carrying every interface match | nftables ANDs the matches inside a rule and offers no branch inside one, so several interface matches on one term ask for a packet whose input interface is several names at once. Separate rules are the only OR nftables has |
| Build the OR the schema declares | A validator refusing a second leaf-list entry, the way `unimplementedVRFValidator` refuses `vrf` | REJECTED. The schema declares a `leaf-list`, its description writes "interface(s)", and the `list route` help promises the rules apply to "the listed interfaces". A validator refusing the second entry makes the daemon contradict its own published schema, and turns a promise into a rejection instead of keeping it |
| The group's terms are ordered by leaf-list position and sit contiguously where the single term sat | Interleaving the groups, or sorting the terms by interface name | At most one term of a group can match a packet, so their relative order decides nothing. Keeping the group contiguous is what leaves the `order` leaf meaning exactly what its `ze:help` says |
| The suffix is the interface's 1-based position | The interface name; a shared name for the whole group | `mergeRuleCounters` sums every kernel rule sharing a term name into ONE row, so a shared name would report the group's total once per interface. The position rather than the name because an interface name is operator input, it can carry `*` and other characters `ValidateName` refuses, and the name buys the reader nothing the position does not |
| One interface keeps the unsuffixed `<policy>-<rule>` | Always suffixing, including for one interface | Every existing test, doc and `show firewall ruleset` reading names that form. Suffixing unconditionally would rename every single-interface term for no gain |
| The wildcard stays a per-entry property | A per-policy wildcard flag | `parseIfaceSpec` already decides per entry, and `ruleTerms` copies each entry's flag into its own term. A list can mix `eth0` and `l2tp*`, and each keeps its own compare |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- `show firewall ruleset` reports one counter row per interface for a multi-interface rule, rather than one row for the rule. Summing them would need the group to share a term name, which is exactly what `mergeRuleCounters` makes impossible without losing the per-interface figure. The per-interface figure is the more useful one, so this is a deliberate boundary and not outstanding work.
- The term name `<policy>-<rule>-<N>` has no length bound. `<policy>-<rule>` could already exceed the 255 bytes `ValidateName` allows, because each name is independently allowed 255. Nothing on the registry path checks it (A-3), so no failure is reachable today. Recorded here rather than fixed, and it belongs with the `ValidateTables` gap in the third Design Insight.
- The leaf-list has no `max-elements`. Each entry now costs a kernel rule for each rule of the policy, so a large list multiplies the rule count. No bound is added here because none was measured.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
