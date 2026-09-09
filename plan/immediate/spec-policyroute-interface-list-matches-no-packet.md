# Spec: policyroute-interface-list-matches-no-packet

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-09-09 |

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
- A term reaches the kernel in slice order (`applyChain`, `internal/plugins/firewall/nft/backend_linux.go`), and the term name travels as `Rule.UserData`, never as a kernel object name.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/policyroute/translate.go` - `translate` builds one `ze_pr` table and one filter chain; `translatePolicy` walks the rules of a policy; `ruleTerms` returns ONE term for each named interface, each carrying that interface's `firewall.MatchInputInterface` first and the rule's own matches after, and ONE term with no interface match where the policy names none; `termName` keeps `<policy>-<rule>` for a single interface and appends the interface's 1-based position for several.
- [ ] `internal/plugins/policyroute/config.go` - `parsePolicyConfig` sorts the policies by name (`slices.Sort(policyNames)`), `parsePolicyRoute` accepts the leaf as either a string or a list and calls `parseIfaceSpec` on each entry, `parseIfaceSpec` strips a trailing `*` and sets `Wildcard` PER ENTRY, and `parsePolicyRoute` sorts the rules by `Order` then by name.
- [ ] `internal/plugins/policyroute/register.go` - `applyPolicies` calls `translate`, `firewall.RegisterTables("policy-routes", ...)`, `firewall.ApplyAll()`, then `rm.applyAll`; `formatPolicies` renders `show policy routes` from the parsed config and re-appends the `*` to a wildcard entry.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerTermForNFProto` concatenates every match's expressions into ONE expression list; `lowerMatch` dispatches `MatchInputInterface` to `lowerIfaceMatch`; `lowerIfaceMatch` emits `expr.Meta{Key: MetaKeyIIFNAME}` plus `expr.Cmp`, sending the unpadded name for a wildcard and the 16-byte IFNAMSIZ-padded name for an exact match.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `applyChain` programs the terms in slice order and writes the term name into `Rule.UserData`; `mergeRuleCounters` sums the counters of every rule sharing one name into a single `firewall.TermCounter` row.
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` checks only the `ze_` table-name prefix and stores the owner's tables; `ApplyAll` merges every owner and calls the backend under one process-wide lock. Neither calls `ValidateTables`.
- [ ] `internal/component/firewall/validate.go` - `validateTerm` calls `ValidateName(term.Name)`, and `ValidateTables` has exactly two callers, both in `internal/component/firewall/engine.go` over the firewall engine's OWN parsed `cfg.Tables`. A term registered through `RegisterTables` therefore never reaches that check.
- [ ] `internal/component/firewall/model.go` - `MatchInputInterface{Name, Wildcard}`, and `ValidateName` delegating to `naming.ValidateNodeName` with a 255-byte limit and an alphanumeric, hyphen, underscore and dot character set.
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - the `leaf-list interface` description promising the plural and the trailing `*`, and three `ze:help` texts that still describe the pre-fix behavior.
- [ ] `internal/test/fixture/netfilter_fixture.go`, `internal/test/fixture/netfilter_fixture_policy.go` - `Register("policy/policy-interface-list", policyRuleDump)` at `netfilter_fixture.go`, and `policyRuleDump` printing the `ze_pr` table plus one `RULE <line>` per programmed rule once the table carries at least two rules.

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
- `Register("policy/policy-interface-list", policyRuleDump)` (`internal/test/fixture/netfilter_fixture.go:58`) - the functional-test driver, already registered and inert until a `.ci` names it.

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
| A-1 | nftables ANDs the expressions inside one rule and ORs across rules, so separate terms are the only way to express an alternative | `lowerTermForNFProto` concatenates every match's expressions into one list (`lower_linux.go:366`), and `applyChain` writes one kernel rule per term | The fix shape is wrong and the whole spec restarts at RESEARCH | `test/policy/policy-interface-list.ci` asserting two `RULE` lines, one naming each interface, and no line naming both | confirmed. The `.ci` passes with both `expect=stdout:pattern=RULE .*iifname "..."` lines and both `reject=stdout:pattern` lines green (10 of 10 steps, QEMU verbose run 2026-09-08, step trace pasted at Vacuity Walk step 5). The "9/9 steps" written here before the review counted the one-rule version of the file, which carried one assertion fewer |
| A-2 | An `iifname` match does not need the interface to exist when the rule loads | `lowerIfaceMatch` emits `expr.Meta{Key: MetaKeyIIFNAME}` plus `expr.Cmp`, a compare against packet metadata rather than an index lookup, and `test/policy/policy-boot-apply.ci` already loads `iifname "l2tp*"` with no such interface present | The `.ci` needs `option=netns-link` for each name, which puts it in the netns-only population and changes the count `docs/functional-tests.md` publishes | The `.ci` passing with no `netns-link` option | confirmed. The `.ci` declares no `option=netns-link` and passes in a guest where neither `eth0` nor any `l2tp*` interface exists |
| A-3 | A term registered through `RegisterTables` is never checked by `firewall.ValidateName` | `ValidateTables` has exactly two callers, `engine.go:216` and `engine.go:321`, both over the firewall engine's own `cfg.Tables`. `RegisterTables` checks the table-name prefix only | The `termName` comment's stated reason is right as written, and the position suffix keeps the same justification either way | `grep -rn "ValidateTables(" internal/component/firewall internal/plugins/firewall` returning those two call sites and no third | confirmed. `ValidateTables` has exactly two non-test callers, `engine.go:216` and `engine.go:321`, both over `cfg.Tables`. `ValidateName(term.Name)` is reached only from `validateTerm` (`validate.go:87`), and `RegisterTables` checks the `ze_` prefix alone. The `termName` comment is corrected in this work |
| A-4 | The chain order is fixed before translation: policies by name, then rules by `order` then name | `slices.Sort(policyNames)` (`config.go:75`) and `sort.Slice` on `Order` then `Name` (`config.go:126`) | The `order` acceptance criterion tests the wrong thing | `TestPolicyInterfaceListKeepsRuleOrder` asserting the four term names in sequence | confirmed. The test drives `parsePolicyConfig` with the `order 10` rule written first and reads `steer-early-1`, `-2`, `steer-late-1`, `-2` back |
| A-5 | Nothing reorders `Chain.Terms` between `translate` and the kernel | `applyChain` iterates `chain.Terms` by index (`backend_linux.go`) | A per-interface group could be split by another rule's terms, and `order` would stop meaning what the help says | The same unit test, plus the `.ci` asserting the `RULE` lines in sequence | confirmed. `TestPolicyInterfaceListKeepsRuleOrder` reads the four terms back in slice order after `translate` |
| A-6 | `show policy routes` is unaffected | `formatPolicies` (`register.go`) renders the parsed `[]PolicyRoute`, never the terms | `test/plugin/policy-routes-show.ci` goes red and the show path joins this spec | confirmed, and now proven by the test. `test/plugin/policy-routes-show.ci` ran in the QEMU guest and PASSES (2.5s), reading `test-pbr` and `allow-http` back through `formatPolicies`. It was red before this work for two reasons that predate the spec and that the interface-list change never reaches: the config declared no SSH server and no user, so `ze cli` stopped at "no credentials for 127.0.0.1:2222: no stored username and none supplied" before dispatching anything, and the daemon then rendered a table while the driver parses JSON. Both are fixed in the `.ci` itself | confirmed, and green |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `show firewall ruleset` gains one counter row for each interface of a multi-interface rule, where an operator read one row before | A firewall show test asserting a row count goes red | This is the correct answer, not a regression: the kernel programs each interface as its own rule, and `mergeRuleCounters` can only sum rows that share a name. The extra rows carry NO new information about traffic, because `applyChain` prepends the counter ahead of the interface match and every row of the chain therefore reports the same chain-wide count (`plan/journal/counter-counts-the-wrong-packets.md`). Named in Known Limitations rather than hidden by giving the group one name |
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
| A committed config with two `interface` entries under one `policy > route` | → | `ruleTerms` (`translate.go`), reached through `applyPolicies` and `translate`, then `firewall.RegisterTables` / `ApplyAll` and `applyChain` | `test/policy/policy-interface-list.ci` |
| The same config, read back from the kernel | → | `lowerIfaceMatch` (`lower_linux.go`) writing one `iifname` compare per rule | `test/policy/policy-interface-list.ci`, asserting `RULE` lines that each name exactly one interface |
| `[]PolicyRoute` handed to `translate` | → | `termName` (`translate.go`) naming the terms of a group | `TestPolicyInterfaceListOneTermPerInterface` (`internal/plugins/policyroute/translate_test.go`) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | A policy names two interfaces and carries one rule | Two nftables rules are installed. One names the first interface, one names the second, and neither names both. Traffic arriving on either interface takes the policy route | `test/policy/policy-interface-list.ci`, steps 6-10 of the verbose QEMU run: `RULE .*iifname "eth0".*meta mark set` and `RULE .*iifname "l2tp\*".*meta mark set` both matched, and both `reject=stdout:pattern` lines found no rule naming both. Plus `TestPolicyInterfaceListOneTermPerInterface` |
| AC-2 | A policy names ONE interface and carries one rule | Exactly one nftables rule is installed, and its term keeps the name `<policy>-<rule>` with no position suffix | `TestMultiplePoliciesMergedIntoOneTable`, which reads back the unsuffixed `alpha-r1` and `beta-r1`, and `termName` returning `base` when `count == 1` |
| AC-3 | A policy names NO interface | Exactly one nftables rule is installed for each of its rules, carrying no interface match, so it matches every ingress interface | `TestPolicyWithoutInterfaceMatchesEveryIngress`: one term named `any-drop-udp` and zero interface matches |
| AC-4 | A policy names `eth0` and `l2tp*` in one leaf-list | The `eth0` rule compares the full 16-byte interface name, and the `l2tp*` rule compares the 4-byte prefix. Each entry keeps its own wildcard state, and mixing the two forms in one list is accepted | `TestPolicyInterfaceListOneTermPerInterface` asserts `{Name: "eth0"}` and `{Name: "l2tp", Wildcard: true}` on their own terms. The `.ci` proves it at the kernel: nft prints `iifname "eth0"` for the 16-byte compare and `iifname "l2tp*"` for the prefix compare, on two separate rules |
| AC-5 | A policy names two interfaces and carries two rules, `order 0` and `order 10` | Four nftables rules are installed. Both rules of the `order 0` group precede both rules of the `order 10` group, so the `order` leaf keeps deciding evaluation sequence | `test/policy/policy-interface-list.ci` proves it at the kernel: the config writes `order 10` first, and the four `RULE` lines read back `eth0 udp dport 53`, `l2tp* udp dport 53`, `eth0 tcp dport`, `l2tp* tcp dport`, in that sequence, under one `expect=stdout:pattern`. The Vacuity Walk records its RED. Plus `TestPolicyInterfaceListKeepsRuleOrder`, which reads `steer-early-1`, `steer-early-2`, `steer-late-1`, `steer-late-2` |
| AC-6 | A policy names two interfaces and one rule selects `table 100` | One fwmark is allocated and one ip rule maps it to table 100. Both nftables rules set that same mark | `TestPolicyInterfaceListOneTermPerInterface` asserts both terms carry the same `SetMark` value and that it equals `result.IPRules[0].Mark`, with `len(result.IPRules) == 1`. The `.ci` asserts `meta mark set` on both `RULE` lines |
| AC-7 | A multi-interface policy is applied and `show firewall ruleset` is read | Each interface's rule is reported as its own counter row, named for that interface's position. No two rules of the group are summed into one row. The AC is about row IDENTITY only: the packets and bytes those rows carry are chain-wide, because `applyChain` prepends the counter ahead of the interface match, and that defect is `plan/journal/counter-counts-the-wrong-packets.md` | `test/policy/policy-interface-list-counters.ci`, PASS in 6.0s in the QEMU guest. It drives `show firewall ruleset pr` with the json pipe over the real `ze cli` SSH path, and asserts `"steer-webmark-1"` and `"steer-webmark-2"` with `reject=stdout:contains="steer-webmark"`, the unsuffixed name a merged group prints. The Vacuity Walk below records its RED: with `termName` returning `base` for every interface, the one row the client printed was `"name": "steer-webmark"` carrying `"packets": 136` |
| AC-8 | An interface name carrying a character a term name may not carry, for example `l2tp*` | The apply succeeds. The term name carries the interface's position, never the interface string, so no interface name can reach a name check | `termName` builds the suffix from `strconv.Itoa(index+1)`, so no operator string enters a term name. The `.ci` commits `interface "l2tp*"` and the apply succeeds (`policy routes applied count=1` in the guest log) |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `interface eth0;` and `interface "l2tp*";` under one policy and commits | config file → `parsePolicyConfig` → `parseIfaceSpec` per entry → `translate` → `ruleTerms` → `RegisterTables` → `ApplyAll` → `applyChain` → two kernel rules | `test/policy/policy-interface-list.ci` |
| 2 | Sends traffic that matches the policy on the SECOND named interface | ingress packet → `ze_pr` prerouting chain → the second rule's `iifname` compare → `meta mark set` → ip rule → the policy's routing table | `test/policy/policy-interface-list.ci` asserting `meta mark set` on both `RULE` lines |
| 3 | Reads `show policy routes` after committing a two-interface policy | plugin command → `formatPolicies` over the parsed config | `test/plugin/policy-routes-show.ci` (existing, must stay green) |
| 4 | Reads `show firewall ruleset` to see which rules a multi-interface policy installed | nft backend → `mergeRuleCounters` → one row for each term name, and `termName` gives each interface its own name | `test/policy/policy-interface-list-counters.ci`, which asserts the row NAMES. The counts in those rows are chain-wide, not per interface: `applyChain` prepends the counter ahead of the interface match, so every rule of the chain counts every packet the chain sees (`plan/journal/counter-counts-the-wrong-packets.md`). The story is "which rules exist", never "which interface carried the traffic" |

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
| `policy-interface-list` | `test/policy/policy-interface-list.ci` | An operator names two ingress interfaces on one policy with two rules, commits, and both interfaces steer traffic for both rules. The test asserts four `RULE` lines in `order` sequence, each naming exactly one interface, and REJECTS any single `RULE` line naming both | written, passing (10 of 10 steps), and every claim-carrying assertion proven to discriminate by an observed RED (Vacuity Walk below) |

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
   - Files: `test/policy/policy-interface-list.ci`. The driver is already registered in `internal/test/fixture/netfilter_fixture.go`, so no Go is written here
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
| The .ci's rejections and its order assertion each have an observed RED | The three breaks in the Vacuity Walk, with their `stdout-regex` and `stdout-reject-regex` failure lines pasted |
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
had, once for each assertion that carries a claim. Every run cross-builds `ze`
and `ze-test` for the guest from the working tree, so a source break reaches the
daemon under test.

The FIRST walk of this test, recorded before the review, broke the right
producer against the wrong config. The `.ci` carried ONE policy rule and
`policyRuleDump` waits for at least two rules in `ze_pr`, so the pre-fix shape
programmed one rule, the fixture died at "policy rules were not programmed", and
the two `reject=stdout:pattern` lines never ran. The test therefore proved one
term per interface and nothing about the rejections, which are the assertions
that tell an AND from an OR. The `.ci` now carries TWO policy rules, `order 10`
and `order 0`. The pre-fix shape then programs two kernel rules, each naming
both interfaces, which is the shape the rejections exist to catch, and the same
config makes AC-5 observable in the kernel.

| Step | Tree | Result |
|------|------|--------|
| 1 | `ruleTerms` as it stands | PASS, 1.4s. That run carried 11 steps: a third rejection, on a tcp `RULE` line preceding a udp one, which step 4 showed no break could turn red and which the positive sequence assertion already covers. It was REMOVED rather than kept unproven, so the final test carries 10 |
| 2 | Break A: `ruleTerms` returns ONE term carrying every interface match, the pre-fix shape | FAIL on the four-rule sequence |
| 3 | Break A, with the sequence assertion commented out so the rejections are reached | FAIL on `RULE .*eth0.*l2tp` |
| 4 | Break B: the same merged term with the interfaces appended in reverse | FAIL on `RULE .*l2tp.*eth0` |
| 5 | `ruleTerms` restored, `git status` clean on `translate.go` | PASS, 1.1s, 10 of 10 steps |

Three breaks, because one break cannot exhibit three reds. The runner evaluates
every `expect=stdout:pattern` before the first `reject=stdout:pattern`, so it
stops at the sequence assertion and never reaches the rejections. And the two
rejections are mirrors of each other: for one leaf-list order exactly one of
them can match, so the second needs a break that emits the names the other way
round.

### Step 1, GREEN before the breaks

```
═══ policy ════════════════════════════════════════════════════════════════════
508ms     0/1      1/1  running  3(504ms)
1.4s     1/1  PASS  3  policy-interface-list
pass  1/1  100.0%  1.4s
```

### Step 2, RED under Break A: the four-rule sequence

Two kernel rules where four are owed, and each one names both interfaces:

```
RULE counter packets 0 bytes 0 iifname "eth0" iifname "l2tp*" udp dport 53 meta mark set 0x00050000
RULE counter packets 0 bytes 0 iifname "eth0" iifname "l2tp*" tcp dport { 80, 443 } meta mark set 0x00050000
```

```
    8 ✗ expect stdout-regex -> stdout does not match regex "RULE .*iifname \"eth0\".*udp dport 53.*\\nRULE .*iifname \"l2tp\\*\".*udp dport 53.*\\nRULE .*iifname \"eth0\".*tcp dport.*\\nRULE .*iifname \"l2tp\\*\".*tcp dport"
```

That one assertion carries AC-5 at the kernel: it reads the `order 0` pair
before the `order 10` pair, with the config writing `order 10` first, so it is
a claim about the sort in `parsePolicyRoute` rather than about the file.

### Step 3, RED under Break A: the first rejection

With the sequence assertion commented out, the same two rules reach the
rejections:

```
    8 ✗ expect stdout-reject-regex -> stdout matches forbidden regex "RULE .*eth0.*l2tp"
```

### Step 4, RED under Break B: the mirror rejection

Break B appends the interfaces in reverse, so the merged rule names `l2tp*`
first:

```
RULE counter packets 0 bytes 0 iifname "l2tp*" iifname "eth0" udp dport 53 meta mark set 0x00050000
RULE counter packets 0 bytes 0 iifname "l2tp*" iifname "eth0" tcp dport { 80, 443 } meta mark set 0x00050000
```

```
    9 ✗ expect stdout-reject-regex -> stdout matches forbidden regex "RULE .*l2tp.*eth0"
```

### Step 5, GREEN after restoring, with every step named

```
1.1s     1/1  PASS  3  policy-interface-list
pass  1/1  100.0%  1.1s
STEP TRACE: 3 policy-interface-list
    1 ✓ exec /workspace/bin/ze-plr start ...
    2 ✓ exec /workspace/bin/ze-test-plr fixture policy/policy-interface-list
    3 ✓ expect exit-code
    4 ✓ expect stderr-contains
    5 ✓ expect stdout-contains
    6 ✓ expect stdout-regex
    7 ✓ expect stdout-regex
    8 ✓ expect stdout-regex
    9 ✓ expect stdout-reject-regex
   10 ✓ expect stdout-reject-regex
QEMU VM: PASS
```

The four kernel rules the restored tree programs, read back with
`nft list table inet ze_pr` in the same guest:

```
counter packets 2 bytes 80 iifname "eth0" udp dport 53 meta mark set 0x00050000
counter packets 2 bytes 80 iifname "l2tp*" udp dport 53 meta mark set 0x00050000
counter packets 2 bytes 80 iifname "eth0" tcp dport { 80, 443 } meta mark set 0x00050000
counter packets 2 bytes 80 iifname "l2tp*" tcp dport { 80, 443 } meta mark set 0x00050000
```

All four counters read the same 2 packets, which is the counter defect in
`plan/journal/counter-counts-the-wrong-packets.md` visible in one dump: the
counter sits ahead of the `iifname` compare, so it counts the chain rather than
the term.


## Vacuity Walk, AC-7 (`test/policy/policy-interface-list-counters.ci`)

The counter-row assertion was also written against code that already worked, so
it owes its own forced red. The producer broken here is `termName`
(`internal/plugins/policyroute/translate.go`), which is what gives a group's
terms distinct names; `mergeRuleCounters`
(`internal/plugins/firewall/nft/backend_linux.go`) merges on an equal name, so
one name for the group is the whole failure. Each step cross-builds `ze` and
`ze-test` for the guest, so the break reaches the daemon under test.

| Step | Tree | Result |
|------|------|--------|
| 1 | `termName` as it stands | PASS, 6.0s |
| 2 | `termName` returning `base` for every interface of a group | FAIL |
| 3 | `termName` restored, `git status` clean on `translate.go` | PASS, 6.4s |

### Step 2, RED under the broken producer

The client reached the daemon and printed the ruleset. The answer carried ONE
term row for the two interfaces, which is the merged shape AC-7 forbids:

```
      "terms": [
        {
          "bytes": 28952,
          "name": "steer-webmark",
          "packets": 136
        },
```

```
    5 x expect stdout-contains -> stdout does not contain "\"steer-webmark-1\""
fail  0/1  0.0%  5.9s  failed 1 [2]
```

`test/policy/policy-interface-list.ci` stayed GREEN through that same step,
which is the second half of the discrimination: the rule COUNT and the interface
matches are unchanged by a break to `termName`, so only the counter-row
assertion can see it. The `termName` break and the `ruleTerms` break therefore
separate the two tests, and neither test can stand in for the other.


## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An `interface` leaf-list of two entries matches traffic on either interface, rather than no packet at all | functional test over config, daemon and kernel | `test/policy/policy-interface-list.ci`. Four `RULE` lines for a two-interface, two-rule policy, each comparing exactly one of `iifname "eth0"` and `iifname "l2tp*"`, each carrying `meta mark set`, and no single rule carrying both names. Ten of ten steps pass, and the Vacuity Walk above records a RED for every claim-carrying assertion |
| The failure stops being silent: the behavior the schema declares is now the behavior the kernel gets | functional test, discriminating assertion | The two `reject=stdout:pattern` assertions, each observed RED under a break that merges the interface matches into one rule (Vacuity Walk steps 3 and 4). They are what the pre-fix code fails, and they are what a whole-table `contains` assertion would have passed against |
| The `order` leaf keeps deciding evaluation sequence once one rule installs several kernel rules | functional test at the kernel, plus a unit test over the whole config path | `test/policy/policy-interface-list.ci`: the config writes `order 10` first, and the four kernel `RULE` lines come back with the `order 0` pair first, under one `expect=stdout:pattern` whose RED is recorded. Plus `TestPolicyInterfaceListKeepsRuleOrder`, whose terms come out `steer-early-1`, `steer-early-2`, `steer-late-1`, `steer-late-2` |
| One `table N` rule still allocates one fwmark and one ip rule, whatever the interface count | unit test | `TestPolicyInterfaceListOneTermPerInterface`: `len(result.IPRules) == 1`, both terms carry an equal `SetMark`, and it equals `result.IPRules[0].Mark` |
| No surface still tells the operator the old, wrong thing | grep over the tree | `grep -rn "one policy per interface" internal/ docs/` returns nothing, `grep -rn "becomes one nftables term" internal/ docs/` returns nothing, and `grep -rn "prepended to every rule" internal/ docs/` returns nothing. The only remaining hits are in this spec, where they record the defect |

## Implementation Summary

### What Was Implemented
- `test/policy/policy-interface-list.ci`, the functional test the already-registered `policy/policy-interface-list` fixture existed to drive. It is the first `.ci` to name that fixture, so the registration in `internal/test/fixture/netfilter_fixture.go` is no longer inert. It carries TWO policy rules, at `order 10` and `order 0`, which is what lets the pre-fix shape program two rules and the rejections discriminate, and what gives AC-5 its kernel proof.
- `TestPolicyInterfaceListKeepsRuleOrder` (`internal/plugins/policyroute/translate_test.go`), driven through `parsePolicyConfig` rather than through `translate` alone, because the sort by `order` lives in `parsePolicyRoute`.
- `test/policy/policy-interface-list-counters.ci`, which drives `show firewall ruleset pr | json` over the real `ze cli` SSH path and asserts that the group's two interfaces are two counter rows. It is the only test in the tree that reaches `mergeRuleCounters` through a client.
- `test/plugin/policy-routes-show.ci`, repaired. It was red before this work for two reasons that predate the spec: the config declared no SSH server and no user, so `ze cli` stopped at "no credentials" before dispatching, and the driver parses JSON while the daemon rendered a table. Both are fixed in the `.ci` itself.
- `netnsSelections[netnsPolicy]` (`internal/le/qemu/netns_linux.go`) names both new tests, so `./le qemu netns-test suites policy` runs them.
- `zeCLIRunsOneCommand` and `zeCLICommandValue` (`internal/test/runner/runner_exec_util.go`), which read the `-c` VALUE and refuse a streaming one. Four daemon cases and one quick case added to `TestIsQuickExitZeCommand`. Without it, `ze cli -c "monitor ..."` is classed quick-exit and awaited to the test deadline.
- The `termName` and `ruleTerms` doc comments in `internal/plugins/policyroute/translate.go`, corrected. No behavior change.
- Two journal rows: `plan/journal/counter-counts-the-wrong-packets.md` (the counter sits ahead of the interface match, so every row of the chain reports the chain's packets) and `plan/journal/one-members-bad-input-fails-every-member.md` (one owner's unusable table fails `ApplyAll` for every owner). Both were walked into here, neither blocks this spec's goal, and neither is fixed here.
- Four prose surfaces: the `interface` leaf-list `description` and its `ze:help`, the `list rule` `ze:help` and the route `leaf name` `ze:help` (`internal/plugins/policyroute/yang/ze-policyroute-conf.yang`); `docs/architecture/policyroute/policy-routing.md`; `docs/guide/policy-routing.md`. The `docs/features.md` sentence is NOT this work's: it landed in `fed8cb4956` ("docs(vrrp): the features page stops promising a pingable VIP"), it is correct, and it is in HEAD.

No product Go changed. `df5b6c25af` had already landed it.

### Bugs Found/Fixed
- The `termName` comment justified the position suffix on "a term name that fails validation takes down the apply of every firewall owner". That is false. `ValidateName(term.Name)` is reached only from `validateTerm` (`internal/component/firewall/validate.go:87`), which only `ValidateTables` calls, and `ValidateTables` has two non-test callers (`internal/component/firewall/engine.go:216` and `:321`), both over the firewall engine's own `cfg.Tables`. `firewall.RegisterTables` checks the `ze_` table-name prefix and nothing else. The comment now states the reason that is true and says plainly that no check stands behind the choice.

### Documentation Updates
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`: three `ze:help` texts and one `description`. `./le yang glue check` reports 154 directories current.
- `docs/architecture/policyroute/policy-routing.md`: the page `translate.go` declares in its `// Design:` header. New "One term for each named interface" section. Anchor added: `<!-- source: internal/plugins/policyroute/translate.go -- ruleTerms, termName -->`.
- `docs/guide/policy-routing.md`: the "Interface binding" section. Anchors added for `ruleTerms`/`termName` and for `mergeRuleCounters`.
- `docs/features.md`: the Policy Routing row.
- `docs/functional-tests.md`: NOT edited. It names the `test/policy` suite rather than each file, and the netns-link population sentence stays correct because the new `.ci` declares no `option=netns-link` (A-2).

### Deviations from Plan
- The vacuity walk ran the `.ci` through `./le qemu run` with a single-test guest script rather than through `./le qemu netns-test suites policy`. That action runs a hardcoded list of six test names (`netnsSelections`, `internal/le/qemu/netns_linux.go`), and the new test is not on it. See Work Not Done.
- The guest run is the guest ROOT namespace rather than the per-test namespace the policy suite uses under `all-tests`. The test does not depend on the namespace: it names two interfaces that exist in neither.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first vacuity walk broke the right producer against a config that could not carry the break. The `.ci` held ONE policy rule, `policyRuleDump` waits for two rules in `ze_pr`, so the pre-fix build programmed one rule and the fixture died before the two `reject=stdout:pattern` assertions ran | Those two rejections are the only assertions that tell an AND inside one rule from an OR across two, and they had never been observed to fail. A typo in either would have left the test permanently green | Round 1 of the independent review, reading the walk against `policyRuleDump` rather than against the walk's own prose | Fixed. The `.ci` carries two policy rules, three separate breaks each redden a different assertion, and the row is journaled in `plan/journal/green-that-could-not-have-been-red.md` |
| assumption | The `termName` doc comment justified the position suffix on "a term name that fails validation takes down the apply of every firewall owner" | No check stands behind it. `ValidateName(term.Name)` is reached only from `validateTerm`, only `ValidateTables` calls that, and its two callers both read the firewall engine's own `cfg.Tables`. A term registered through `RegisterTables` is checked for the `ze_` table prefix and nothing else | A-3, validated by grepping every `ValidateTables` call site | Fixed in the comment, which now names the counter-merge reason that is true and says plainly that no check stands behind the choice |
| approach | The Review Gate's round-1 resolution table recorded NOTE 3 as "not addressed, this round was not allowed to edit `translate.go`" and NOTE 5 as "unhomed" | Both are in HEAD. `89180a9cb` carries the `ruleTerms` clone clause NOTE 3 asked for and the journal row NOTE 5 asked for | The closure audit read `89180a9cb` rather than the table describing it | Fixed in the table. A false statement in the record is a NOTE and earns no further review round (`ai/rules/planning.md`) |
| approach | The spec named the nft backend's rule programmer `programChain` in eight places, including the basis cells of A-1 and A-5 | No such symbol has ever existed in this tree. The producer is `(*backend).applyChain` (`internal/plugins/firewall/nft/backend_linux.go`), and every behavioral claim attached to the invented name is true of it: terms are programmed in slice order and the term name travels in `Rule.UserData` | The closure audit resolved the symbol before repeating the claim | Fixed. The name is corrected everywhere and the claims are re-verified at `applyChain` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Build the OR the schema declares, rather than refusing the second leaf-list entry | Done | `ruleTerms` (`internal/plugins/policyroute/translate.go`) | One term per named interface. The rejected alternative is a Key Design Decisions row |
| Establish what several rules per policy does to rule ordering and to the `order` leaf | Done | `translatePolicy` appends each rule's group contiguously; `parsePolicyRoute` (`config.go`) sorts by `Order` then name | Proven at the kernel by the four-`RULE` sequence assertion and in `TestPolicyInterfaceListKeepsRuleOrder` |
| Stop every surface telling the operator to write one policy per interface | Done | `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`, `docs/guide/policy-routing.md`, `docs/architecture/policyroute/policy-routing.md`, `docs/features.md` | `grep -rn "one policy per interface" internal/ docs/` returns nothing |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/policy/policy-interface-list.ci` (two `expect=stdout:pattern` naming one interface each, two `reject=stdout:pattern` refusing a rule naming both) and `TestPolicyInterfaceListOneTermPerInterface` | Producer `ruleTerms`. RED observed under Break A and Break B |
| AC-2 | Done | `TestMultiplePoliciesMergedIntoOneTable`, reading back `alpha-r1` and `beta-r1` | Producer `termName`, `count == 1` returns `base` |
| AC-3 | Done | `TestPolicyWithoutInterfaceMatchesEveryIngress` | Producer `ruleTerms`, the `len(policy.Interfaces) == 0` branch |
| AC-4 | Done | `TestPolicyInterfaceListOneTermPerInterface` asserts `{Name: "eth0"}` and `{Name: "l2tp", Wildcard: true}` on separate terms; the `.ci` reads `iifname "eth0"` and `iifname "l2tp*"` back from the kernel | Producers `ruleTerms` (copies the per-entry flag) and `lowerIfaceMatch` (pads for an exact name, sends the prefix unpadded) |
| AC-5 | Done | The `.ci`'s four-`RULE` sequence assertion, whose RED is recorded at Vacuity Walk step 2, plus `TestPolicyInterfaceListKeepsRuleOrder` | The config writes `order 10` first, so the assertion is a claim about the sort rather than about the file |
| AC-6 | Done | `TestPolicyInterfaceListOneTermPerInterface`: `len(result.IPRules) == 1`, both terms carry an equal `SetMark`, and it equals `result.IPRules[0].Mark`. The `.ci` asserts `meta mark set` on both `RULE` lines | The mark is allocated in `buildActions`, once per rule, before `ruleTerms` is called |
| AC-7 | Done | `test/policy/policy-interface-list-counters.ci`: `expect=stdout:contains="steer-webmark-1"`, `"steer-webmark-2"`, and `reject=stdout:contains="steer-webmark"` | The wording claims row IDENTITY only. Re-read against the assertions at closure: the test asserts the three names and asserts no count, which is what the AC says. RED observed with `termName` returning `base` for every interface |
| AC-8 | Done | `termName` builds the suffix from `strconv.Itoa(index+1)`; the `.ci` commits `interface "l2tp*"` and asserts `policy routes applied` on stderr | No operator string enters a term name |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPolicyInterfaceListOneTermPerInterface` | Done | `internal/plugins/policyroute/translate_test.go` | Passes. `./le job run label unit command go test ./internal/plugins/policyroute/...` is `ok` |
| `TestPolicyWithoutInterfaceMatchesEveryIngress` | Done | same file | Passes |
| `TestMultiplePoliciesMergedIntoOneTable` | Done | same file | Passes |
| `TestPolicyInterfaceListKeepsRuleOrder` | Done | same file | Passes |
| `policy-interface-list` | Done | `test/policy/policy-interface-list.ci` | 10 of 10 steps, PASS 1.1s in the QEMU guest. Three forced REDs recorded |
| `policy-interface-list-counters` | Done | `test/policy/policy-interface-list-counters.ci` | PASS 6.0s in the QEMU guest. One forced RED recorded |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` | Done | Three `ze:help` texts and one `description` |
| `internal/plugins/policyroute/translate.go` | Done | `termName` and `ruleTerms` doc comments. No behavior change |
| `internal/plugins/policyroute/translate_test.go` | Done | `TestPolicyInterfaceListKeepsRuleOrder` added |
| `docs/architecture/policyroute/policy-routing.md` | Done | New "One term for each named interface" section, with the `ruleTerms, termName` anchor |
| `docs/guide/policy-routing.md` | Done | "Interface binding" rewritten, with the `applyChain, mergeRuleCounters` anchor for the counter sentence |
| `docs/features.md` | Changed | Correct and in HEAD, carried by `fed8cb4956` rather than by this work |
| `docs/functional-tests.md` | Changed | Deliberately not edited. It names the `test/policy` suite rather than each file, and the netns-link population sentence stays correct because neither new `.ci` declares `option=netns-link` |
| `test/policy/policy-interface-list.ci` | Done | Created |

### Audit Summary
- **Total items:** 25 (3 requirements, 8 ACs, 6 tests, 8 files)
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (`docs/features.md`, `docs/functional-tests.md`; both recorded in Deviations and in Documentation Updates)

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Nothing. | | |

The two rows this table carried until the review were false. Both claimed
`internal/le/qemu/netns_linux.go` was left unedited, and both edits are in HEAD:
`a2b98a342` added `policy-interface-list` to `netnsSelections[netnsPolicy]` and
`75281fbae` added `policy-interface-list-counters`. Read back on 2026-09-08, the
`netnsPolicy` selection carries both names.

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
| The suffix is the interface's 1-based position | The interface name; a shared name for the whole group | `mergeRuleCounters` sums every kernel rule sharing a term name into ONE row, so a shared name would merge the group into one row. The position rather than the name because an interface name is operator input, it can carry `*` and other characters `ValidateName` refuses, and the name buys the reader nothing the position does not. The names are distinct WITHIN one rule's group. They are not unique across policies: a second policy can own the same name, and Known Limitations states the condition |
| One interface keeps the unsuffixed `<policy>-<rule>` | Always suffixing, including for one interface | Every existing test, doc and `show firewall ruleset` reading names that form. Suffixing unconditionally would rename every single-interface term for no gain |
| The wildcard stays a per-entry property | A per-policy wildcard flag | `parseIfaceSpec` already decides per entry, and `ruleTerms` copies each entry's flag into its own term. A list can mix `eth0` and `l2tp*`, and each keeps its own compare |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- `show firewall ruleset` reports one counter row for each interface of a multi-interface rule, rather than one row for the rule. Distinct names are what keeps the rows distinct, and a shared name would merge them in `mergeRuleCounters`. The rows say which rules exist. They do not say which interface carried the traffic: `applyChain` prepends the counter ahead of the interface match, so every row reports the packets the whole `ze_pr` chain saw. That is `plan/journal/counter-counts-the-wrong-packets.md` and it is out of scope here, because fixing it changes counter semantics for every firewall owner.
- The position suffix can collide with an operator-chosen rule name, and the colliding rows then merge. The condition: policy P names two or more interfaces and carries rule R, so it emits `P-R-<N>`; a second policy named `P-R` carries a rule named `<N>` and names exactly one interface, so `termName` returns its unsuffixed base `P-R-<N>`. Concretely, policy `steer` with `rule web` and two interfaces emits `steer-web-1` and `steer-web-2`, and policy `steer-web` with `rule 1` and one interface emits `steer-web-1`. Both `leaf name` nodes are `type string` with no `pattern`, and `naming.ValidateNodeName` (`internal/core/naming/validate.go`) accepts a leading digit, so nothing refuses either name. Both terms land in the one `ze_pr` prerouting chain, `mergeRuleCounters` keys on the name, and the second policy's row disappears into the first policy's totals. Forwarding is unaffected: each kernel rule keeps its own matches and actions, and only the reported counter row merges. The base-name collision (`a-b` plus rule `c` against `a` plus rule `b-c`) predates this work, and the suffix is a NEW member of that class, because a synthesized name now competes with an operator-chosen rule name. Recorded rather than fixed: a separator no name can carry would have to be a character `ValidateNodeName` rejects, and every term name would change with it.
- The term name `<policy>-<rule>-<N>` has no length bound. `<policy>-<rule>` could already exceed the 255 bytes `ValidateName` allows, because each name is independently allowed 255. Nothing on the registry path checks it (A-3), so no failure is reachable today. Recorded here rather than fixed, and it belongs with the `ValidateTables` gap in the third Design Insight.
- The leaf-list has no `max-elements`. Each entry now costs a kernel rule for each rule of the policy, so a large list multiplies the rule count. No bound is added here because none was measured.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/policyroute-interface-list-matches-no-packet-0a21e591-d035-4f7c-8d1b-231ab023a478.md` |
| `./le spec session review check` | OK, clean, hashes match |
| Rounds | 2. Round 1 found 1 BLOCKER and 3 ISSUE and fixed all four. Round 2, the closure audit, found 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | Round 1: producer verification, discrimination of the functional assertions, naming collisions, runner classification. Round 2: producer verification, AC-to-test mapping, vacuity discrimination, `ze-go-style` pass, documentation anchors |

Independent review, round 1, 2026-09-08. The reviewer did not write this code.
Scope: `df5b6c25af`, `a2b98a342`, `75281fbae`. Counts: 1 BLOCKER, 3 ISSUE,
5 NOTE.

Verified at the producer rather than from this spec's prose:
`lowerTermForNFProto` (`internal/plugins/firewall/nft/lower_linux.go`)
concatenates every match's expressions into one list, and `(*backend).applyChain`
(`internal/plugins/firewall/nft/backend_linux.go`) emits one kernel rule for each
term in `chain.Terms`, in slice order. Separate terms are separate rules, and
separate rules are the OR. A-1 and A-5 hold. The order chain holds end to end:
`parsePolicyConfig` sorts policies by name, `parsePolicyRoute` sorts rules by
`Order` then name, `translatePolicy` appends each rule's group contiguously, and
`applyChain` programs by index. The wildcard stays a per-entry property:
`parseIfaceSpec` sets it per entry, `ruleTerms` copies it into that entry's own
term, and `lowerIfaceMatch` sends the unpadded prefix for a wildcard and the
16-byte padded name for an exact match.

`go test ./internal/plugins/policyroute/` is green. `./internal/test/runner/` has
two reds and neither is this work: `TestCIAcceptOnlyLint` fails on 29
`test/exabgp-compat/api/*.ci` files this diff does not touch, and
`TestCIMustFailFixturesAllFail/exit-code-is-judged.ci` fails because `/bin/false`
does not exist on darwin.

### BLOCKER

1. **`docs/guide/policy-routing.md`, "Interface binding": the counter sentence is
   false, and this work published it.** The page now says "A multi-interface
   policy therefore reports its traffic per interface rather than as one total."
   `(*backend).applyChain` (`internal/plugins/firewall/nft/backend_linux.go`)
   prepends `&expr.Counter{}` AHEAD of the expressions `lowerTerm` produced, and
   `readRuleCounter` reads that first counter. nftables evaluates a rule's
   expressions in order, so the counter increments for every packet the CHAIN
   sees rather than for the packets the term matched. policyroute declares no
   `counter` action, so `hasCounterExpr` is never true on this path and the
   prepend always happens. The failure: an operator names `eth0` and `l2tp*` on
   one policy, sends traffic on `eth0` only, reads `show firewall ruleset`, and
   sees the SAME packet count on `steer-webmark-1` and `steer-webmark-2`. The
   page tells him to read that as per-interface traffic. The same session
   measured this in a QEMU guest and wrote it up in
   `plan/journal/counter-counts-the-wrong-packets.md` (a `dummy0` rule reporting
   the 3 packets only the `lo` rule matched), and
   `test/policy/policy-interface-list-counters.ci` states it in its own header,
   after the page had already gone out saying the opposite. Fix: correct that
   sentence to say what the rows are (one row for each interface, naming which
   rule the traffic could have taken, with the count chain-wide until the journal
   row is fixed), and correct the two places in this spec carrying the same
   claim: End-to-End User Story 4 ("Reads `show firewall ruleset` to see which
   interface is carrying the traffic") and the AC-7 Expected Behavior wording.
   The counter defect itself is correctly journaled and stays out of scope.

### ISSUE

1. **`termName` (`internal/plugins/policyroute/translate.go`) can produce a name
   an operator's own rule already owns, and `mergeRuleCounters` then sums two
   policies into one row.** `firewall.ValidateName` accepts digits, so `rule 1`
   is a legal rule name. Policy `steer` naming two interfaces with `rule web`
   emits `steer-web-1` and `steer-web-2`; policy `steer-web` naming one interface
   with `rule 1` emits `steer-web-1`. Both land in the one `ze_pr` prerouting
   chain, `mergeRuleCounters` (`internal/plugins/firewall/nft/backend_linux.go`)
   keys on the name, and the second policy's row disappears into the first
   policy's totals. Forwarding is unaffected and the reported number is silently
   wrong. The base-name collision (`a-b` plus `c` against `a` plus `b-c`) is
   older than this change, but the suffix is a NEW member of that class: a
   synthesized name now competes with operator-chosen rule names. The Key Design
   Decisions row states the distinct-name property with no condition on it, and
   Known Limitations does not record it. Fix: record the condition in Known
   Limitations, or find a suffix separator no `ValidateName` name can carry.

2. **The recorded RED for `test/policy/policy-interface-list.ci` never reaches
   the assertions the spec calls discriminating, and AC-5 has no kernel proof.**
   The `.ci` configures ONE rule, and `policyRuleDump`
   (`internal/test/fixture/netfilter_fixture_policy.go`) waits for at least two
   rules in `ze_pr`. Under the reverted `ruleTerms` the pre-fix code emits one
   term, so the fixture fails at "policy rules were not programmed" and steps 8
   and 9, the two `reject=stdout:pattern` lines, never run. Those two patterns
   have therefore never been observed to fail, so a typo in either leaves the
   test permanently green. The producer broken was the right one; the CONFIG is
   what limits the walk. Fix: give the policy a second rule, `order 0` and
   `order 10`. The pre-fix shape then emits two rules, each carrying both
   `iifname` matches, which is what the reject lines exist to catch, and the same
   config makes AC-5 observable in the kernel rather than only in
   `TestPolicyInterfaceListKeepsRuleOrder`. AC-5 is proven by a unit test alone
   today, which this spec's own Critical Review Checklist forbids ("No AC is
   proven by a unit test only").

3. **`zeCLIRunsOneCommand` (`internal/test/runner/runner_exec_util.go`) classes a
   streaming `ze cli -c` as quick-exit.** The daemon guards still come first and
   no config-file or `--web` invocation reaches the new branch, so the shape the
   commit set out to fix is safe. But `runBGP`
   (`internal/component/cli/client/main.go`) routes `-c` to
   `client.StreamMonitor` when `isMonitorCommand` accepts the command, and that
   call streams until the connection drops. `ze cli -c "monitor event"` is a
   daemon carrying `-c`, and `awaitQuickZe` does a bare `proc.Wait()` on it. The
   process is started with `exec.CommandContext(testCtx, ...)`, so the bound is
   the test deadline rather than the forever the function's own comment warns
   about: the step burns the whole budget and the test fails on timeout. No `.ci`
   does this today (14 files use `ze cli -c`, every one a `show` or a
   `validate`), so the finding is prospective. Fix: refuse a `-c` value beginning
   with a registered streaming prefix, and add the case to the daemon list in
   `TestIsQuickExitZeCommand`.

### NOTE

1. **Work Not Done is false on both rows.** Both claim
   `internal/le/qemu/netns_linux.go` was outside the editing scope and left
   unhomed. `a2b98a342` added `policy-interface-list` to
   `netnsSelections[netnsPolicy]` and `75281fbae` added
   `policy-interface-list-counters`. Both are in HEAD, so the table should be
   emptied. A false record does not re-open a round.
2. **`test/policy/policy-interface-list-counters.ci` fences on `sleep 5`.** The
   header explains why a fence is needed (the SSH listener accepts after
   `ze.ready.file` is written) and that the runner has no primitive for it. A
   fixed sleep in an oversubscribed QEMU guest is a flake source, and it spends
   5s of a 40s budget on every run. A retry loop around the client, or a runner
   primitive that waits on a listening port, is the durable answer.
3. **`ruleTerms` clones the actions slice on one branch and not on the other.**
   The per-interface branch writes `slices.Clone(actions)` for each term; the
   no-interface branch passes `actions` through unchanged, which is safe because
   it is the only term. Nothing in the tree appends to `firewall.Term.Actions`,
   so the clone guards a mutation that does not exist today. It is cheap
   config-plane work and it follows the style guide's "do not take an alias", so
   it stays. The asymmetry is worth one clause in the doc comment.
4. **`docs/features.md` is named as this work's fourth prose surface, and its
   Policy Routing sentence landed in `fed8cb4956` ("docs(vrrp): the features page
   stops promising a pingable VIP"), in none of the three commits under review.**
   The text is correct and it is in HEAD. The Implementation Summary should name
   the commit that carries it.
5. **A bare `interface "*";` still takes the whole firewall apply down.**
   `parseIfaceSpec` strips the trailing `*` and leaves `Name` empty,
   `lowerIfaceMatch` returns `errInterfaceNameMustNotBeEmpty`, and that fails
   `applyChain` and therefore `ApplyAll` for every registered owner rather than
   for policy routes alone. Pre-existing and unchanged by this work; AC-8 covers
   `l2tp*` and does not reach it. It is worth a journal row rather than a fix
   here.

### Verdict

Not clean. The BLOCKER is one sentence in an operator-facing page plus two
sentences in this spec, and the session already holds the measurement that
settles it. ISSUE 2 is a config change to one `.ci`. ISSUE 1 and ISSUE 3 are a
recorded limitation and a two-line guard. None of the four touches product Go.

### Round 1 resolution, 2026-09-08

No product Go changed. Every finding was cleared where the review named it.

| Finding | Cleared by | Where |
|---------|-----------|-------|
| BLOCKER 1 | The false counter sentence is replaced. The guide now says a multi-interface policy gets one row for each interface, each naming the rule the traffic can take, and then states plainly that the counts in those rows are NOT per interface: the nft backend puts the counter at the front of the rule, ahead of the interface match, so it counts every packet the `ze_pr` chain sees, and two interfaces on one policy report the same count. Verified at both producers before the sentence was written: `applyChain` prepends `&expr.Counter{}` when `hasCounterExpr` is false, `readRuleCounter` takes the FIRST `expr.Counter`, and `buildActions` emits no counter action, so the prepend always happens on this path. The four kernel rules in the Vacuity Walk show it, all four reading `packets 2 bytes 80`. `applyChain` is untouched | `docs/guide/policy-routing.md`, "Interface binding", plus the `applyChain` source anchor. In this spec: End-to-End User Story 4, AC-7 Expected Behavior, R-1 and the first Known Limitation, all of which carried the same claim |
| ISSUE 1 | The collision condition is recorded, precisely and with its producers named, and the naming scheme is unchanged. The Key Design Decisions row no longer states distinctness without a condition: it says the names are distinct within one rule's group and points at the limitation | `## Known Limitations`, first bullet; `## Key Design Decisions`, the position-suffix row |
| ISSUE 2 | The `.ci` carries two policy rules, `order 10` written first and `order 0` second. The pre-fix shape then programs two kernel rules rather than one, so `policyRuleDump`'s two-rule wait is satisfied and the rejections are reached. AC-5 gained its kernel proof: one `expect=stdout:pattern` reads the four `RULE` lines in `order` sequence. The walk was redone with three breaks, and every claim-carrying assertion now has an observed RED. A third rejection written during this work was REMOVED rather than kept, because no break exhibited its red and the positive sequence assertion already carries the claim | `test/policy/policy-interface-list.ci`; `## Vacuity Walk`, steps 1 to 5 |
| ISSUE 3 | `zeCLIRunsOneCommand` reads the `-c` VALUE and refuses one whose first word is `monitor`, the CLI's whole streaming namespace. A `-c` with no readable value is refused too, which keeps the file's stated property: misclassifying a daemon as quick-exit is the worse failure. The registry is NOT consulted, and the comment says why: ze-test links a subset of the streaming handlers the daemon links, so `pluginserver.StreamingPrefixes()` read there would answer about the wrong process and would accept `ze cli -c "monitor event"`. Four daemon cases and one quick case added; the guard was observed RED against the previous implementation | `internal/test/runner/runner_exec_util.go`, `zeCLIRunsOneCommand` and `zeCLICommandValue`; `TestIsQuickExitZeCommand` |
| NOTE 1 | The Work Not Done table is emptied and says why: both `netnsSelections[netnsPolicy]` registrations are in HEAD (`a2b98a342`, `75281fbae`), read back on 2026-09-08 | `## Work Not Done` |
| NOTE 4 | The Implementation Summary no longer claims `docs/features.md` as this work's surface. It names `fed8cb4956` | `## Implementation Summary`, "What Was Implemented" |
| NOTE 3 | Addressed, and this row said otherwise until the closure audit read the commit. `ruleTerms` carries the clause: "The clone is on the per-interface branch alone. Several terms sharing one actions slice would alias, and no reader can see from a term that its neighbor holds the same backing array; the single term of the no-interface branch has nobody to alias with, so it takes the caller's slice as it is." | `internal/plugins/policyroute/translate.go`, the `ruleTerms` doc comment, in `89180a9cb` |
| NOTE 5 | Recorded, and this row said "unhomed" until the closure audit read the file. A bare `interface "*";` leaves an empty name, `lowerIfaceMatch` refuses it, and `ApplyAll` batches every owner's tables into one `Apply`, so one operator's typo costs copp, ddos-local, flowspec-firewall and the operator's own `firewall {}` block their reconcile. Verified at all three producers and journaled as a class, with both candidate repairs named | `plan/journal/one-members-bad-input-fails-every-member.md`, row of 2026-09-09, in `89180a9cb` |
| NOTE 2 | Not addressed, and separable: the `sleep 5` fence is a runner capability (a primitive that waits on a listening port), not this spec's goal, and the reason it is needed is written in the `.ci`'s own header rather than only here, so it survives this spec's deletion | `test/policy/policy-interface-list-counters.ci`, header |

### Round 2, closure audit, 2026-09-09

Counts: 0 BLOCKER, 0 ISSUE, 3 NOTE. The round read the four commits at their
producers rather than through this spec's prose, and re-derived every claim the
round-1 resolution table makes.

Verified at the producer: `ruleTerms` returns one term per named interface with
the interface match first and `slices.Clone(actions)` per term; `termName`
returns `base` at `count == 1` and appends `strconv.Itoa(index+1)` otherwise;
`(*backend).applyChain` prepends `&expr.Counter{}` when `hasCounterExpr` is
false and programs `chain.Terms` in slice order with the term name in
`Rule.UserData`; `mergeRuleCounters` sums on an equal name; every one of the
five `RegisterStreamingHandler` prefixes in the tree starts with `monitor`, so
`zeCLIStreamingCommand` covers the namespace `isMonitorCommand` routes to
`StreamMonitor`. `netnsSelections[netnsPolicy]` names both new tests, and the
`policy` suite in `internal/le/qemu/alltests.go` runs `allTests`, so both are in
the gating population rather than reachable only by name.

Style pass over the changed Go (`docs/contributing/ze-go-style.md`): no finding.
No `panic` is added, no peer reaches `internal/test/runner`, both new functions
guard and return early, and `zeCLICommandValue` returns `(value, ok)` so a `-c`
with no value cannot be read as an empty command.

Two reds in `internal/test/runner`, neither this work's: `TestCIAcceptOnlyLint`
fails on 5 accept-only `.ci` files and 40 unparseable
`test/exabgp-compat/api/*.ci` files this diff does not touch, and
`TestCIMustFailFixturesAllFail/exit-code-is-judged.ci` fails because
`/bin/false` does not exist on darwin. Neither names a file of this spec
(`ai/rules/pre-release.md`: the red is the instrument, so it is reported rather
than repaired here).

| # | Severity | Finding | Location | Resolution |
|---|----------|---------|----------|------------|
| 1 | NOTE | The round-1 resolution table states NOTE 3 unaddressed and NOTE 5 unhomed. Both are in `89180a9cb`. A false statement in the record is a NOTE and earns no further round (`ai/rules/planning.md`) | `## Review Gate`, round-1 resolution table | Corrected in this closure, one edit |
| 2 | NOTE | The spec named the nft rule programmer `programChain` in eight places. No such symbol exists in the tree; the producer is `(*backend).applyChain`, and every claim attached to the invented name holds at it | `## Required Reading`, `## Current Behavior`, A-1, A-5, Wiring Test, User story 1 | Corrected in this closure, one edit. Recorded in the Mistake Log |
| 3 | NOTE, open | The `list rule > leaf name` `ze:help` says the rule name "is also the second half of the nftables term name `<policy>-<rule>`" and is silent on the `<policy>-<rule>-<N>` form. Not false for a single-interface policy, and the `leaf-list interface` and route `leaf name` helps both carry the full rule | `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`, `list rule > leaf name` | Left as written. One sentence, no behavior claim is wrong, and editing the YANG at closure costs a glue regeneration this commit has no other reason to carry |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/policy/policy-interface-list.ci` | Yes | `ls` returns it; read in full at closure |
| `test/policy/policy-interface-list-counters.ci` | Yes | `ls` returns it; read in full at closure |
| `test/plugin/policy-routes-show.ci` | Yes | In HEAD, repaired by `75281fbae` |
| `plan/journal/counter-counts-the-wrong-packets.md` | Yes | One row, 2026-09-08 |
| `plan/journal/one-members-bad-input-fails-every-member.md` | Yes | One row, 2026-09-09 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Two rules, one per interface, neither naming both | `ruleTerms` read at closure: one `firewall.Term` per `policy.Interfaces` entry, each carrying exactly one `MatchInputInterface`. `test/policy/policy-interface-list.ci` carries both `reject=stdout:pattern` lines |
| AC-2 | One interface keeps `<policy>-<rule>` | `termName` returns `base` when `count == 1`. `TestMultiplePoliciesMergedIntoOneTable` asserts `alpha-r1` and `beta-r1` |
| AC-3 | No interface, one term, no interface match | `ruleTerms` returns one term named `base` on the empty branch. `TestPolicyWithoutInterfaceMatchesEveryIngress` asserts zero interface matches |
| AC-4 | The wildcard stays per entry | `ruleTerms` copies `iface.Wildcard` into that entry's own term; the unit test asserts `{Name: "l2tp", Wildcard: true}` on term 2 alone |
| AC-5 | The `order` leaf still decides the sequence | The `.ci`'s four-`RULE` sequence assertion, red under Break A; `TestPolicyInterfaceListKeepsRuleOrder` reads `steer-early-1`, `-2`, `steer-late-1`, `-2` |
| AC-6 | One mark, one ip rule | `buildActions` is called once per rule, before `ruleTerms`; the unit test asserts `len(result.IPRules) == 1` and equal marks |
| AC-7 | One counter row per interface, named for its position | `mergeRuleCounters` keys on the term name, so distinct names cannot merge. The counters `.ci` asserts both suffixed names and rejects the unsuffixed one |
| AC-8 | No operator string reaches a term name | `termName` builds the suffix with `strconv.Itoa(index+1)`; the `.ci` commits `interface "l2tp*"` and the apply succeeds |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A committed config with two `interface` entries | `test/policy/policy-interface-list.ci` | Yes. Read at closure: the config block declares both entries, and the assertions read the kernel back through `ze-test fixture policy/policy-interface-list` |
| The same config read back from the kernel | `test/policy/policy-interface-list.ci` | Yes. The two `iifname` patterns match per-rule `RULE` lines from `policyRuleDump`, not a whole-table dump |
| `[]PolicyRoute` handed to `translate` | `TestPolicyInterfaceListOneTermPerInterface` | Yes. Asserts the two term names and one interface match each |
| `show firewall ruleset` over the client | `test/policy/policy-interface-list-counters.ci` | Yes. The step runs `ze cli -c` with the json pipe over SSH, so the whole client path runs |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `lowerTermForNFProto` concatenates every match's expressions into one list; `applyChain` writes one kernel rule per term. The `.ci` passes with both rejections green |
| A-2 | confirmed | Neither `.ci` declares `option=netns-link`, and both pass in a guest where neither `eth0` nor any `l2tp*` interface exists |
| A-3 | confirmed | Re-grepped at closure: `ValidateTables` has two non-test callers, both in `engine.go` over `cfg.Tables`; `ValidateName(term.Name)` is reached only from `validateTerm` |
| A-4 | confirmed | `slices.Sort(policyNames)` and the `sort.Slice` on `Order` then `Name`, both in `parsePolicyConfig`/`parsePolicyRoute` and read at closure. `TestPolicyInterfaceListKeepsRuleOrder` passes |
| A-5 | confirmed | `applyChain` iterates `chain.Terms` by index and programs in slice order. Nothing between `translate` and it reorders |
| A-6 | confirmed | `test/plugin/policy-routes-show.ci` passes in the guest, reading `test-pbr` and `allow-http` back through `formatPolicies` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/policy-routing.md`, "Interface binding" | Read against `ruleTerms`, `termName`, `applyChain` and `mergeRuleCounters` at closure. The page states the OR, the one-rule-per-interface count, the two naming forms, and that the counts in those rows are chain-wide | Yes |
| `docs/architecture/policyroute/policy-routing.md`, "One term for each named interface" | Read against `ruleTerms` and `termName`. The naming rule and the contiguous-group statement both hold | Yes |
| `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` | The `leaf-list interface` description and help, the `list rule` help and the route `leaf name` help each state one term per named interface and the two naming forms | Yes |
| `docs/features.md`, Policy Routing row | In HEAD from `fed8cb4956` and correct: "A policy that names several interfaces matches traffic on any of them. Ze installs one nftables rule per named interface" | Yes, not this work's edit |
| Declared design documents | `./le spec citation anchors spec plan/immediate/spec-policyroute-interface-list-matches-no-packet.md` prints nothing, which is its clean answer: no document declared by a `// Design:` header of a named source file, and no `ai/CODE-TO-DOCS.md` mention, is left unnamed | Yes |
| Citation drift | `./le spec citation` reports no warning for this spec after the `applyChain` correction | Yes |
| Categories answered No | `grep -rn "one policy per interface" internal/ docs/`, `grep -rn "becomes one nftables term" internal/ docs/` and `grep -rn "prepended to every rule" internal/ docs/` each return nothing | Yes |

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
