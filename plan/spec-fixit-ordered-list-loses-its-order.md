# Spec: fixit -- an ordered-by-user list loses its order at the plugin boundary

| Field | Value |
|-------|-------|
| Status | done |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A YANG list declared `ordered-by user` reaches a plugin with its order gone.
`Tree.listOrder` holds the operator's order, and `Tree.ToMap` never reads it.
Every plugin therefore receives a Go map, which has no order, and the reader on
the far side has to invent one.

Five of the eleven order-sensitive lists are broken by this today. Three fail
loudly and two fail silently.

| Broken reader | Symptom |
|---------------|---------|
| `filter_prefix`, `filter_aspath`, `filter_community_match` `entry` | a list of two or more entries is refused at load; the feature is unusable past one entry |
| firewall `term` | rule order is Go map iteration order, so it is randomized per process; a first-match-wins dataplane filters differently on every reload |
| `authradius` `server` | failover order is sorted by name, so the operator's first server is whichever one sorts first |

The firewall case is the severe one. It is silent, it is non-deterministic, and
it changes what a box forwards.

The goal is to carry the order the Tree already holds across the JSON boundary,
and to make every one of the five readers use it. No reader may substitute a
lexical order for the operator's, and no reader may accept a multi-entry
ordered list whose order it was not given.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config.md` - the Config List Shapes section states the delivered shape of a list and bans reading an `ordered-by user` list through `configvalue.ListEntries`
  → Constraint: a `list` is `map[string]any` keyed by the list key at every count, and the key leaf is the map key rather than a field inside the entry. This spec MUST NOT change that
  → Constraint: sorting a keyed map to recover order is banned for `ordered-by user`, because it substitutes a lexical order for the configured one
  → Decision: this rule text says "the operator's entry order is already gone". It becomes false on the plugin path once this spec lands, so the rule gets the ordered counterpart named in it
- [ ] `ai/rules/architecture.md` - tier placement and the sibling call-site audit
  → Constraint: a pure library with no plugin lifecycle belongs in `internal/core/`, so the shared reader lands there
  → Constraint: a guard added to one call site obliges a grep of every other call site in the same commit. Five readers share this defect and all five are in scope
- [ ] `ai/rules/simplicity.md` - the shape of the fix
  → Constraint: the fix goes at the ROOT (the lowering), not as a guard bolted onto each of the five readers
  → Decision: a rejected design MUST be named with the requirement it failed. Five are named in Key Design Decisions

**Key insights:**
- `Tree.listOrder` already holds the operator's order for every list, at every depth. `AddListEntry` maintains it, `GetListOrdered` reads it, and four serializers render the config text from it. Nothing has to be invented, only carried.
- `Tree.ToMap` is read by about forty consumers, including gNMI Get and Subscribe, `ze config show | json`, the web config handler, the support bundle, and `ValidateTreeAllModules`. A new key in its output reaches all of them, and the validator would refuse it. So the order cannot ride in `ToMap`.
- The plugin-facing map is a different map: it is produced by two calls in `cmd/ze/hub` and by the verify path in `plugin_verify.go`, and it flows only to plugins, the reactor, and the config diff.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/tree.go` - `Tree` holds `listOrder`, maintained by `AddListEntry` and read by `GetListOrdered`. `ToMap` walks values, multiValues, containers and lists, and never reads `listOrder`
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfig` lowers with `Tree.ToMap`; `BuildPluginConfigSections` and `buildPluginConfigSectionsTransition` call `ExtractConfigSubtree` then `json.Marshal`
- [ ] `internal/component/plugin/server/startup.go` - `deliverConfigRPC` builds sections from the reactor's config tree
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` takes the new tree from the config loader, diffs it, builds sections from it, and stores it with `SetConfigTree`
- [ ] `internal/component/plugin/server/reload_tx.go` - `marshalOperationRoot` marshals the same two maps for the transaction path
- [ ] `cmd/ze/hub/main.go` - lowers the loaded Tree with `ToMap` into the map that becomes the coordinator's config tree
- [ ] `cmd/ze/hub/main_reload.go` - lowers the reloaded Tree with `ToMap` for the reload path
- [ ] `cmd/ze/hub/main_pki.go` - `configTreeFromMap` rebuilds a Tree out of that map for the PKI parser
- [ ] `internal/component/bgp/plugins/filter_prefix/config.go` - `parsePrefixListEntries` has an unreachable slice branch and a map branch that refuses two or more entries
- [ ] `internal/component/bgp/plugins/filter_aspath/config.go` - `parseAsPathEntries` has the same shape and the same refusal
- [ ] `internal/component/bgp/plugins/filter_community_match/config.go` - `parseCommunityEntries` has the same shape and the same refusal
- [ ] `internal/component/firewall/config.go` - `ParseFirewallConfig` parses a config section; `parseChain` ranges the term map and appends, with no ordering anywhere
- [ ] `internal/component/l2tp/plugins/authradius/config.go` - `serverEntries` sorts the keyed map by name and calls that "deterministic server (failover) ordering"
- [ ] `internal/component/config/tomap_shape_test.go` - `TestToMapListIsAlwaysAKeyedMap` pins the map shape at one and at two entries
- [ ] `internal/component/config/diff.go` - `DiffMaps` walks the map by path, so a value that changes with the order makes a reorder visible to the reload

**Behavior to preserve:**
- `Tree.ToMap` output is unchanged, byte for byte. Every generic consumer of it keeps working, and `TestToMapListIsAlwaysAKeyedMap` keeps passing untouched.
- A list stays a `map[string]any` keyed by the list key at every count, in every lowering. The key leaf stays the map key and never becomes a field inside the entry.
- The three BGP filter readers keep refusing a multi-entry list whose order they were not given. The refusal is the fail-closed guard, and it stays.
- The existing slice-of-entries fixtures keep working. Slice support moves into the shared reader rather than being deleted.
- Single-entry lists are delivered exactly as they are today, with no additional key.

**Behavior to change:**
- The plugin-facing lowering carries the order of every list that has two or more entries.
- Five readers consume that order instead of inventing one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator writes an `ordered-by user` list in the config file, or reorders one from the CLI editor with `insert before` / `insert after`.
- The parser records each entry in the Tree's list map and appends its key to `listOrder`.

### Transformation Path
1. `Tree.ToMap` lowers the tree to `map[string]any`, dropping `listOrder`. This is the loss.
2. `ExtractConfigSubtree` selects one root out of that map and re-wraps it.
3. `json.Marshal` serializes the subtree. Go sorts object keys, so even the incidental order of the map is replaced by a lexical one.
4. The plugin receives the bytes as a config section and unmarshals into `map[string]any`, which has no order.
5. The reader iterates that map and produces the runtime structure: a prefix-list, a firewall chain, a RADIUS server list.

After this spec, stage 1 splits: the general lowering stays as it is, and a
plugin-facing lowering emits the order beside each multi-entry list. Stages 2
to 4 carry that key unchanged. Stage 5 reads it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config Tree -> plugin config map | the plugin-facing lowering emits the order sidecar next to each multi-entry list | No |
| Core -> plugin process (JSON) | the sidecar survives `json.Marshal` and `json.Unmarshal` as a JSON array of strings | No |
| Plugin config map -> plugin runtime state | the shared reader returns entries in operator order, or refuses | No |

### Integration Points
- The Tree's `listOrder` field - the order already recorded, now read by the plugin-facing lowering.
- `BuildPluginConfigSections` and the plugin server's reload path - unchanged; they marshal whatever map they are handed, and the sidecar rides in it.
- `internal/core/configvalue` - untouched. `ListEntries` stays the reader for an unordered list; the new package is its ordered counterpart.

### Architectural Verification
Answered at closure against what shipped. "Holds?" answers the CHECK, so `Yes`
means the property is satisfied.

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the order travels in the config payload the plugin already receives. `Tree.ToPluginMap` (`internal/component/config/tree.go`) writes it, `BuildPluginConfigSections` marshals it unchanged, `configorder.Entries` reads it. No side channel and no second RPC: `git show d687efe7e --stat` adds no RPC and no transport file |
| No unintended coupling (components stay isolated) | Yes | `KeyPrefix` is declared once, in `internal/core/configorder/configorder.go`. A grep for the `"@"` literal over `internal/` and `cmd/` names that declaration and one assertion in the package's own test; every other of the 16 sites calls `configorder.OrderKey` or `configorder.KeyPrefix` |
| No duplicated functionality (extends existing, does not recreate) | Yes | five hand-rolled entry readers collapsed onto one. `parsePrefixListEntries`, `parseAsPathEntries`, `parseCommunityEntries`, `parseChain` and `parseConfigFromTree` each call `configorder.Entries` and none iterates a list map itself |
| Zero-copy preserved where applicable (refs, not copies) | Yes | config lowering is a startup and reload path, not a wire path. `configorder.Entry.Map` is the delivered map rather than a copy, and its doc says a caller MUST NOT write to it |
| Registration over hardcoding | Yes | no per-list branch anywhere. `Tree.toMap` emits the order for every list of two or more entries under one `if withListOrder && len(listMap) > 1`, with no list name in the code |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No plugin rejects an unknown key at container level in its delivered config | grep for `unknown key`, `DisallowUnknownFields`, `unexpected field` over `internal/`: one strict reader exists, `parseFibVPPConfig` in `internal/plugins/fib/vpp/fibvpp.go`, and its container holds only two leaves and no list | that plugin refuses its config at startup | the grep above, plus the functional suite | unvalidated |
| A-2 | The two `cmd/ze/hub` lowering calls are the only producers of the map that reaches a plugin | `deliverConfigRPC` reads the reactor's config tree; `reloadConfig` reads the config loader's map; `runTxCoordinator` is handed those same two maps | a plugin gets a map with no sidecar and refuses a multi-entry ordered list, loudly | a `.ci` that loads a two-entry prefix-list through the daemon | unvalidated |
| A-3 | A YANG node name can never start with `@`, so the sidecar cannot collide with a sibling leaf, container or list | YANG identifiers start with a letter or underscore; the codebase already relies on the same argument for `GroupKeyPrefix` in `internal/component/bgp/configjson/traverse.go` | a config key would be shadowed | a unit test that a container carrying a list still delivers every declared sibling | unvalidated |
| A-4 | Reordering entries with no other edit now shows up in the config diff, so the plugin is reconfigured | `DiffMaps` walks the map by path, and the sidecar value changes when the order changes | a reorder would be applied to the running config but never delivered | a unit test over `DiffMaps` with two orderings of the same list | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Making the firewall refuse a multi-term chain whose order it was not given breaks a config that loads today | the firewall unit tests, and any `.ci` that configures two terms | every production producer is converted in this spec, so the refusal fires only for a hand-built map; the refusal names the chain and says the order was not delivered |
| R-2 | The sidecar reaches a consumer of the coordinator's config tree that walks it generically | `configTreeFromMap` rebuilds a Tree from that map for the PKI parser | the rebuild skips a reserved key; every other consumer of that map reads named keys |
| R-3 | Two packages now read config lists, and a future reader picks the wrong one | a reader that sorts an `ordered-by user` list | `ai/rules/config.md` names both and says which list kind each one serves |
| R-4 | A sixth ordered list is added later and its reader forgets the order | none at author time | the shared reader refuses a multi-entry list with no order, so the new reader fails loudly on its first two-entry config rather than silently |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | plugin config delivery for every plugin. A wrong sidecar shape is loud (a refusal at load), a missing one is loud for the five converted readers, and invisible for every other plugin |
| How is it reverted? | a single commit revert. Nothing persists the sidecar: it is recomputed from the Tree on every load and reload |
| Who else touches this path? | a sibling session is finishing `internal/core/configvalue` and the shape coercion in the same three BGP filter readers. This spec adds a separate package and touches only the entry-list handling in those readers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| operator config with a two-entry prefix-list, loaded by the daemon | → | plugin-facing lowering -> section JSON -> `parsePrefixListEntries` -> `handleFilterUpdate` | `test/plugin/prefix-filter-entry-order.ci` |
| operator config with a two-entry prefix-list, verified at commit | → | `VerifyPluginConfig` -> the filter plugin's verify hook | `TestVerifyPluginConfigDeliversListOrder` |
| firewall chain with two terms | → | plugin-facing lowering -> `ParseFirewallConfig` -> `parseChain` | `TestParseChainTermOrderFollowsTheOperator` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a prefix-list whose entries are written in a non-lexical order, loaded through the daemon | the config loads, and the first entry the operator wrote is the first one evaluated |
| AC-2 | the same prefix-list with the two entries swapped | the opposite match decision, proving the order is read rather than reconstructed |
| AC-3 | an as-path-list and a community-match list, each with two entries | both load, and both evaluate in the operator's order |
| AC-4 | a firewall chain with two terms written in a non-lexical order | `parseChain` returns the terms in the operator's order, on every run |
| AC-5 | a RADIUS server list with two servers written in a non-lexical order | `serverEntries` returns them in the operator's order, not sorted by name |
| AC-6 | a container map with a multi-entry list and no order key | the shared reader returns an error naming the list, and no reader falls back to a sorted or arbitrary order |
| AC-7 | any config, lowered with `Tree.ToMap` | the output is identical to the output before this change: no order key at any depth, at any entry count |
| AC-8 | a single-entry list, on the plugin path | delivered exactly as today, with no order key |
| AC-9 | two configs differing only in the order of one list's entries | `DiffMaps` reports a change, so the plugin is reconfigured |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | writes a prefix-list that rejects a subnet and accepts everything else | config file -> Tree -> plugin lowering -> section JSON -> filter_prefix -> filter decision | `test/plugin/prefix-filter-entry-order.ci` |
| 2 | writes a firewall chain whose first term drops and whose second accepts | config file -> Tree -> plugin lowering -> section JSON -> `ParseFirewallConfig` -> nft rule order | `TestParseChainTermOrderFollowsTheOperator` |
| 3 | reorders a prefix-list from the CLI editor and commits | editor Tree -> commit -> reload -> `DiffMaps` -> plugin reconfigure | `TestDiffMapsSeesAListReorder` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToPluginMapCarriesListOrder` | `internal/component/config/toplugin_order_test.go` | a two-entry list gains its order key, in the operator's order, at every depth | |
| `TestToPluginMapLeavesSingleEntryListsAlone` | `internal/component/config/toplugin_order_test.go` | AC-8: one entry, no order key | |
| `TestToMapIsUnchangedByTheOrderedLowering` | `internal/component/config/toplugin_order_test.go` | AC-7: `ToMap` output carries no order key at any depth | |
| `TestDiffMapsSeesAListReorder` | `internal/component/config/toplugin_order_test.go` | AC-9 | |
| `TestVerifyPluginConfigDeliversListOrder` | `internal/component/config/toplugin_order_test.go` | the verify path and the configure path deliver the same shape | |
| `TestEntriesFollowTheDeliveredOrder` | `internal/core/configorder/configorder_test.go` | the reader returns entries in the delivered order, for the in-process and the post-JSON form of the order value | |
| `TestEntriesRefuseAMultiEntryListWithNoOrder` | `internal/core/configorder/configorder_test.go` | AC-6 | |
| `TestEntriesAcceptASliceOfEntries` | `internal/core/configorder/configorder_test.go` | the slice form keeps working, and the key leaf is read from inside the entry | |
| `TestEntriesToleratesAMalformedOrder` | `internal/core/configorder/configorder_test.go` | an order naming an absent key, and a key the order omits, are both handled without a panic and without a silent drop |  |
| `TestParsePrefixListsTwoEntriesInNonLexicalOrder` | `internal/component/bgp/plugins/filter_prefix/config_test.go` | AC-1, AC-2 | |
| `TestParseAsPathListsTwoEntriesInNonLexicalOrder` | `internal/component/bgp/plugins/filter_aspath/config_test.go` | AC-3 | |
| `TestParseCommunityListsTwoEntriesInNonLexicalOrder` | `internal/component/bgp/plugins/filter_community_match/config_test.go` | AC-3 | |
| `TestParseChainTermOrderFollowsTheOperator` | `internal/component/firewall/config_test.go` | AC-4 | |
| `TestServerEntriesFollowTheOperatorNotTheAlphabet` | `internal/component/l2tp/plugins/authradius/config_test.go` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| list entry count on the plugin path | 0 .. N | 2 is the first count that emits an order key | 1 emits none (valid, no order needed) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-filter-entry-order` | `test/plugin/prefix-filter-entry-order.ci` | an operator writes a two-entry prefix-list in a non-lexical order; the more specific reject entry is written first and wins, and the route is dropped | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | no wire-visible protocol change: this is config lowering, and the existing BGP policy interop scenarios already cover the filter surface | N-A |

## Files to Modify
- `internal/component/config/tree.go` - the plugin-facing lowering, sharing one walker with `ToMap`
- `internal/component/config/plugin_verify.go` - the verify path lowers with the plugin-facing lowering
- `cmd/ze/hub/main.go` - the coordinator's config tree is the plugin-facing lowering
- `cmd/ze/hub/main_reload.go` - the reload path lowers the same way
- `cmd/ze/hub/main_pki.go` - the Tree rebuild skips the reserved key
- `internal/component/bgp/plugins/filter_prefix/config.go` - entries read through the shared reader
- `internal/component/bgp/plugins/filter_aspath/config.go` - the same
- `internal/component/bgp/plugins/filter_community_match/config.go` - the same
- `internal/component/firewall/config.go` - `parseChain` reads terms in the operator's order
- `internal/component/l2tp/plugins/authradius/config.go` - `serverEntries` stops sorting by name
- `ai/rules/config.md` - Config List Shapes names the ordered reader and states the delivered shape

## Files to Create
- `internal/core/configorder/configorder.go` - the reserved key and the one reader for an `ordered-by user` list
- `internal/core/configorder/configorder_test.go` - its tests
- `internal/component/config/toplugin_order_test.go` - the lowering's tests
- `test/plugin/prefix-filter-entry-order.ci` - the functional test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf, container or list; the `ordered-by user` declarations already exist |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | No | no new leaf |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/prefix-filter-entry-order.ci` |
| Pipe completeness | N-A | no command output changes |
| Env var registration | No | no new env var |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module or binary |
| Prometheus counters/metrics | No | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | no SAFI, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the feature was always declared; it did not work. Nothing new to announce |
| 2 | Config syntax changed? | No | the config text is unchanged |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | the config section type is unchanged; only the JSON body gains a key |
| 5 | Plugin added/changed? | No | no plugin added or removed |
| 6 | Has a user guide page? | No | no operator-visible change |
| 7 | Wire format changed? | No | no protocol bytes change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` states what a plugin receives, and it now receives the order key |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC obligation is touched |
| 10 | Test infrastructure changed? | No | one new `.ci` in an existing suite |
| 11 | Affects daemon comparison? | No | no capability claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` describes the config lowering, and `tree.go` declares it in its Design header |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED with `python3 scripts/dev/spec_doc_anchors.py plan/spec-fixit-ordered-list-loses-its-order.md`. Five design docs are declared by the changed files and each is unaffected, because none of them describes the delivered JSON body: `docs/architecture/config/yang-config-design.md` (declared by `tree.go`) describes the YANG-to-Tree modelling, which is unchanged; `docs/architecture/core-design.md` (declared by the three filter `config.go` files and by the firewall `config.go`) describes the component layout, and no component moves; `docs/architecture/hub-architecture.md` (declared by `cmd/ze/hub/main.go`) describes the startup sequence, and the sequence is unchanged because only the lowering call changes; `docs/architecture/pki/tls-listeners.md` (declared by `cmd/ze/hub/main_pki.go`) describes certificate resolution, and the PKI parser reads the same named keys; `docs/research/l2tpv2-ze-integration.md` (declared by the authradius `config.go`) describes the L2TP integration, and the RADIUS server list is unchanged in shape. The one doc that DOES describe the delivered body is `docs/architecture/api/process-protocol.md`, named in row 8 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | any doc showing a single-entry prefix-list example is now safe to show with two entries; checked at implementation time |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the order reaches a plugin
   - Tests: `TestToPluginMapCarriesListOrder`, `TestToMapIsUnchangedByTheOrderedLowering`, `TestVerifyPluginConfigDeliversListOrder`
   - Files: `internal/core/configorder/configorder.go`, `internal/component/config/tree.go`, `internal/component/config/plugin_verify.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`, `cmd/ze/hub/main_pki.go`
   - Verify: the order key appears in a delivered section and nowhere in `ToMap` output
2. **Phase: the three loud readers** -- the reported defect
   - Tests: `TestParsePrefixListsTwoEntriesInNonLexicalOrder` and its two siblings, `test/plugin/prefix-filter-entry-order.ci`
   - Files: the three filter `config.go` files
   - Verify: a two-entry prefix-list loads and evaluates first-match-wins in the operator's order
3. **Phase: the two silent readers** -- the severe defect
   - Tests: `TestParseChainTermOrderFollowsTheOperator`, `TestServerEntriesFollowTheOperatorNotTheAlphabet`
   - Files: `internal/component/firewall/config.go`, `internal/component/l2tp/plugins/authradius/config.go`
   - Verify: term order and failover order both follow the config text
4. **Phase: the rule** -- the next reader picks the right reader
   - Files: `ai/rules/config.md`
   - Verify: the Config List Shapes section names both readers and the list kind each one serves

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC has code and a test |
| Feature completeness | all five broken readers converted, not the three that were reported |
| Correctness | no reader sorts an `ordered-by user` list; a missing order is an error, never a fallback |
| Naming | the reserved key is spelled in exactly one file |
| Data flow | `ToMap` output is unchanged; only the plugin-facing lowering carries the key |
| Rule: `ai/rules/config.md` | the delivered list shape is still a keyed map, and the key leaf is still the map key |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the reserved key is spelled once | a grep for the key literal over `internal/` and `cmd/` names only `internal/core/configorder` |
| no reader sorts an ordered list | `grep -n 'sort\.' internal/component/l2tp/plugins/authradius/config.go` returns nothing for the server list |
| the three refusals still exist | `grep -rn 'first-match-wins' internal/core/configorder/configorder.go` |
| the functional test discriminates | mutate the plugin-facing lowering to skip the order, re-run, observe red, restore from a pristine copy |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the order value arrives from the daemon's own lowering, never from an operator string. A reader must still tolerate a malformed one rather than index out of range: an order naming a key the list does not hold, and a list holding a key the order does not name, are both handled |
| Resource exhaustion | the order key is one string slice per multi-entry list, sized by the entry count that already exists |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check the AC |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

- The Tree never lost the order. Four serializers read `listOrder` and render the config text back in the operator's order, so `ze config show` has always been right. Only the plugin path threw the order away, which is why the defect hid: the surface an operator inspects agreed with what they wrote.
- The refusal in the three BGP filter readers was the correct engineering call at the time, and it is why this defect is a loud failure there and a silent one in the firewall. The refusal is kept, not replaced.
- `serverEntries` in the L2TP RADIUS plugin is the counter-example that settles the design. It met the same problem and sorted the map "for deterministic server (failover) ordering". It is deterministic and it is wrong: the operator's first server is now whichever one sorts first. Determinism is not correctness.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Carry the order as a sibling key beside the list, on a plugin-facing lowering only | emit it from `ToMap` for every consumer | about forty consumers read `ToMap`, and three of them cannot take a new key: gNMI Get and Subscribe would publish a path no YANG module declares, `ze config show \| json` would show it to the operator, and `ValidateTreeAllModules` would refuse the whole config as an unknown node |
| Emit the sidecar for every list with two or more entries | emit it only for lists declared `ordered-by user` | the schema is not reachable at any of the lowering call sites, and building it costs a full YANG load. Marking the Tree at parse time instead would be a second source of truth that a Tree built by another path silently lacks. The entry-count rule needs no schema and cannot fail open |
| A missing order for a multi-entry list is an error | fall back to sorting by key | that is exactly what `serverEntries` does today, and it silently replaced the operator's failover order with a lexical one. A fallback turns a loud defect into a wrong answer |
| Keep slice-of-entries support, inside the shared reader | delete the unreachable slice branch from each of the three filter readers | deleting it drops the only multi-entry fixture those readers have. Moving it into the shared reader keeps the fixture, keeps one copy instead of three, and gives it a test of its own |
| A new `internal/core/configorder` package | add the reader to `internal/core/configvalue` | `configvalue` is being changed by another session right now. The two packages serve different list kinds and `ai/rules/config.md` names which is which |
| Emit the order as a JSON array of keys | emit the entries themselves as a JSON array, the RFC 7951 encoding of a YANG list | the array encoding changes the shape of every list for every plugin at once, and six readers that assert a map would see no entries at all. The firewall would silently lose every term. An additive key changes nothing for a reader that does not look for it |

## Known Limitations

- The order is delivered for every multi-entry list, not only for the eleven declared `ordered-by user`. A reader that does not want it ignores an extra key. The alternative needs the schema at lowering time, and the schema is not there.
- **Corrected at closure:** "every multi-entry list" holds only where the Tree RECORDED a complete order. `entryOrderLocked` returns nil when the recorded order does not account for every entry in the lowered list, and `Tree.toMap` then emits no order key at all, so `configorder.Entries` refuses the list and names it. `Tree.GetList` hands out the live entry map, so a caller CAN add an entry without `AddListEntry` and leave the order short. Completing a short order by any rule would invent an order the operator did not write and a reader could not tell from a real one, so the lowering fails closed instead. `TestToPluginMapEmitsNoOrderItDidNotRecord` pins it.
- Six of the eleven order-sensitive lists were already correct and are untouched: radius and tacacs read the Tree's ordered accessor directly, policyroute carries an explicit `order` leaf, rsvpte sorts its numeric index key, pki `intermediate` is a leaf-list whose order `ToMap` already preserves, and firewall `element` is a set whose order does not matter.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

The implementation landed across four commits before closure ran, so closure
reviewed committed code rather than a working tree.

| Commit | What it carried |
|--------|-----------------|
| `0cc2cf949` | `internal/component/firewall/config.go` -- `parseChain` reads terms with `configorder.Entries`. Landed early, inside a sibling spec's commit, while `internal/core/configorder` was still untracked. That left main unbuildable until `d687efe7e` |
| `d687efe7e` | `internal/core/configorder`, `Tree.ToPluginMap`, `plugin_verify.go`, the three BGP filter readers and `authradius`, `main_pki.go`, `loader_create.go`, `plugin/cli/test_cmd.go`, `test/plugin/prefix-filter-entry-order.ci`, two docs, `ai/rules/config.md` and its point file, `ai/PACKAGE-MAP.md` |
| `e171cc978` | the two review rounds' product fixes: `DeleteList` clears `listOrder`, `DeleteList` and `DeleteContainer` take the lock, `treeFromMap` skips the reserved key |
| `5b24b076f` | the two hub lowering call sites, `cmd/ze/hub/main.go` and `cmd/ze/hub/main_reload.go`, carried as passengers on the AAA session's commit because both files also held its uncommitted work |

### What Was Implemented
- `internal/core/configorder`: `KeyPrefix`, `OrderKey`, `Entry`, `Entries`. One reader for a list declared `ordered-by user`. It answers the delivered map form and the RFC 7951 slice form, and it refuses a multi-entry list delivered with no order rather than sorting one.
- `Tree.ToPluginMap` beside `Tree.ToMap`, both over one walker `Tree.toMap(withListOrder bool)`. `ToMap`'s output is unchanged byte for byte.
- Five readers converted: `parsePrefixListEntries`, `parseAsPathEntries`, `parseCommunityEntries`, `parseChain`, `parseConfigFromTree` (authradius `server`).
- Every producer of a plugin-facing map converted: `VerifyPluginConfig`, `CreateReactorFromTree`, `cmdPluginTest`, the hub's boot and reload lowerings, and the hub's ConfigProvider seeding (the last one found and fixed by this closure).
- Two rebuild-a-Tree-from-a-map sites skip the reserved key: `configTreeFromMap` (PKI) and `treeFromMap` (IKE).

### Bugs Found/Fixed
| Bug | Producing function | Now covered by |
|-----|--------------------|----------------|
| `DeleteList` left `listOrder` holding the deleted list's entries, so a list rewritten under the same name was lowered in the DELETED list's order | `(*Tree).DeleteList` (`internal/component/config/setparser.go`) | `TestToPluginMapDoesNotResurrectADeletedListsOrder` |
| `DeleteList` and `DeleteContainer` mutated the tree with no lock, while every other mutator on the struct takes one | same file | the `-race` runs over `internal/component/config` |
| `treeFromMap`'s `[]any` arm turned a reserved order key into a multi-value leaf no YANG module declares | `treeFromMap` (`internal/component/ike/engine/config.go`) | `internal/component/ike/engine` suite |
| **Found by this closure.** A whole-entry `delete` in the set-format parser removed the entry with a bare map delete, bypassing `RemoveListEntry`, so `listOrder` kept naming it. Writing the same key again appended it a second time, and the delivered order then placed the entry where the operator DELETED it rather than where they wrote it. The duplicate order has the right length and names only keys the list holds, so `entryOrderLocked` passes it. `Tree.GetListOrdered` returned the entry twice for the same reason, which is what the four serializers print from | `(*SetParser).deleteFromList` and `(*SetParser).deleteFromInlineList` (`internal/component/config/setparser.go`) | `TestToPluginMapFollowsAnEntryDeletedAndWrittenAgain` |
| **Found by this closure.** The hub seeded the ConfigProvider with `Tree.ToMap` while the coordinator's config tree was `Tree.ToPluginMap`. A failed reload replays the provider SNAPSHOT into the plugin server, so the recovery path handed every plugin a multi-entry list with no order, which `configorder.Entries` refuses. Boot and reload only ever met on that path, so the divergence was invisible until a reload failed | `runYANGConfig` Phase 2 (`cmd/ze/hub/main.go`), read back by `rollbackReload` -> `snapshotToLoadedTree` (`cmd/ze/hub/main_reload.go`) | `TestLowerForPluginsGivesTheProviderTheOrderARollbackReplays` |

### Documentation Updates
- `docs/architecture/api/process-protocol.md`: a new subsection states that a list of two or more entries arrives with its order beside it, with a worked JSON example rooted at `bgp` (the root `ExtractConfigSubtree` wraps a section body in). Anchors: `internal/core/configorder/configorder.go -- Entries, OrderKey` and `internal/component/config/tree.go -- Tree.ToPluginMap`.
- `docs/architecture/config/syntax.md`: the paragraph that said the entry order "does not survive lowering" is replaced by the two-lowering table and the reader obligation. Anchors updated to name `ToPluginMap` and `configorder`.
- `docs/architecture/core-design.md` (**this closure**): the ConfigProvider bullet now names the lowering it is populated with and why a `ToMap` root would reach a plugin without the order. Anchors: `cmd/ze/hub/main_reload.go -- lowerForPlugins, applyLoadedTreeToProvider, rollbackReload` and `internal/component/config/tree.go -- (*Tree).ToPluginMap`.
- `ai/rules/config.md` and its point file: the reader table names `configorder.Entries` and `configvalue.ListEntries`, states that the lowering carries the order, and names the two permitted sorts whose order is recoverable from the entry data (`parseERO`, `parsePolicyRoute`).
- `ai/PACKAGE-MAP.md`: a row for `internal/core/configorder`.
- `make ze-doc-verify`: two failures, both foreign and both traced. `rules-points: commands.md is stale` comes from another session's uncommitted edit to `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md`. The one anchor finding names `liveAAABundleAuthenticator.Authenticate` in `cmd/ze/hub/aaa_authenticator_web.go`, another session's uncommitted file. `make ze-discovery-index-update` produced no change to `ai/DOCS-TO-CODE.md`, so this spec's new anchors resolve.

### Deviations from Plan
| Planned | Shipped | Why |
|---------|---------|-----|
| `Files to Modify` did not name `internal/component/config/setparser.go` | it changed twice: `DeleteList`/`DeleteContainer` in `e171cc978`, and the two entry-delete paths at closure | `setparser.go` owns three of the four mutators that have to keep `t.lists` and `t.listOrder` in step. Neither was reachable from the file list the spec derived, because both are DELETE paths and the spec traced the WRITE path |
| `Files to Modify` did not name `internal/component/bgp/config/loader_create.go`, `internal/component/plugin/cli/test_cmd.go`, `internal/component/ike/engine/config.go` | all three changed | the sibling-call-site grep the architecture rule requires found two more producers of a plugin-facing map and a second Tree-rebuild site |
| A-2 said the two `cmd/ze/hub` lowering calls are the only producers of a map that reaches a plugin | there were five, and the fifth was missed until closure | see the Assumptions Resolved table |
| The lowering emits an order for every multi-entry list | it emits none for a list whose recorded order does not account for every entry | `Tree.GetList` hands out the live map, so a short order is reachable. Inventing the rest would be indistinguishable from a real order, so the lowering fails closed. Recorded in Known Limitations |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: the two `cmd/ze/hub` lowering calls were taken to be the only producers of the map that reaches a plugin | five producers exist. Three were converted during implementation (`plugin_verify.go`, `loader_create.go`, `test_cmd.go`) without A-2 being revisited. The fifth, the hub's own ConfigProvider seeding, was missed entirely and reaches a plugin through `rollbackReload` | this closure's review round 3, tracing every `.ToMap()` call site in `internal/` and `cmd/` rather than trusting the spec's list | fixed in `lowerForPlugins`; A-2 recorded `broken` |
| approach | the entry-order invariant was fixed one mutator at a time: `AddListEntry` and `RemoveListEntry` were already correct, `DeleteList` was fixed in round 2, and the two entry-delete paths were still wrong | the unit to fix was the INVARIANT, which is "`t.lists` and `t.listOrder` only change together", not the mutator in hand | this closure's review round 3, by probing the set-format parser with a delete-then-rewrite sequence | both remaining paths now call `RemoveListEntry`; journal row in `helper-bypassed-by-an-open-coded-copy` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| carry the order the Tree already holds across the JSON boundary | Done | `(*Tree).ToPluginMap`, `(*Tree).toMap`, `entryOrderLocked` (`internal/component/config/tree.go`) | emitted under `configorder.OrderKey(listName)` beside the list, at every depth |
| every one of the five readers uses it | Done | `parsePrefixListEntries`, `parseAsPathEntries`, `parseCommunityEntries`, `parseChain`, `parseConfigFromTree` | each calls `configorder.Entries`; none iterates the list map itself |
| no reader substitutes a lexical order for the operator's | Done | `configorder.orderFor` | `grep -n 'sort\.' internal/component/l2tp/plugins/authradius/config.go` returns nothing |
| no reader accepts a multi-entry ordered list whose order it was not given | Done | `configorder.orderFor` | the refusal names the list and the count, and points at `Tree.ToPluginMap` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/plugin/prefix-filter-entry-order.ci` | PASS through the real daemon at the closure tree |
| AC-2 | Done | same `.ci`, plus `TestParsePrefixListsTwoEntriesInNonLexicalOrder` | the swapped-order subtest produces the opposite decision |
| AC-3 | Done | `TestParseAsPathListsTwoEntriesInNonLexicalOrder`, `TestParseCommunityListsTwoEntriesInNonLexicalOrder` | both carry the swapped-order subtest |
| AC-4 | Done | `TestParseChainTermOrderFollowsTheOperator` | plus `TestParseChainRefusesTermsWithNoOrder` and `TestParseChainSingleTermNeedsNoOrder` |
| AC-5 | Done | `TestServerEntriesFollowTheOperatorNotTheAlphabet`, `TestParseConfigProductionTwoServersKeepsTheOperatorOrder` | the second runs the whole config-text -> Tree -> `ToPluginMap` -> parse pipeline |
| AC-6 | Done | `TestEntriesRefuseAMultiEntryListWithNoOrder`, `TestServerEntriesRefuseTwoServersWithNoOrder`, `TestParseChainRefusesTermsWithNoOrder` | the refusal is at the shared reader, so no converted reader can fall back |
| AC-7 | Done | `TestToMapIsUnchangedByTheOrderedLowering`, and `TestToMapListIsAlwaysAKeyedMap` untouched | no order key at any depth, at any entry count |
| AC-8 | Done | `TestToPluginMapLeavesSingleEntryListsAlone`, `TestEntriesAcceptASingleEntryWithNoOrder` | |
| AC-9 | Done | `TestDiffMapsSeesAListReorder` | a reorder with no other edit reaches the plugin |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestToPluginMapCarriesListOrder` | Done | `internal/component/config/toplugin_order_test.go` | |
| `TestToPluginMapLeavesSingleEntryListsAlone` | Done | same | |
| `TestToMapIsUnchangedByTheOrderedLowering` | Done | same | |
| `TestDiffMapsSeesAListReorder` | Done | same | |
| `TestVerifyPluginConfigDeliversListOrder` | Done | same | proven to discriminate: reverting `VerifyPluginConfig` to `tree.ToMap()` makes it fail |
| `TestEntriesFollowTheDeliveredOrder` | Done | `internal/core/configorder/configorder_test.go` | covers the `[]string` and the post-JSON `[]any` form |
| `TestEntriesRefuseAMultiEntryListWithNoOrder` | Done | same | |
| `TestEntriesAcceptASliceOfEntries` | Done | same | |
| `TestEntriesToleratesAMalformedOrder` | Done | same | |
| `TestParsePrefixListsTwoEntriesInNonLexicalOrder` | Done | `internal/component/bgp/plugins/filter_prefix/config_test.go` | |
| `TestParseAsPathListsTwoEntriesInNonLexicalOrder` | Done | `internal/component/bgp/plugins/filter_aspath/config_test.go` | |
| `TestParseCommunityListsTwoEntriesInNonLexicalOrder` | Done | `internal/component/bgp/plugins/filter_community_match/config_test.go` | |
| `TestParseChainTermOrderFollowsTheOperator` | Done | `internal/component/firewall/config_test.go` | |
| `TestServerEntriesFollowTheOperatorNotTheAlphabet` | Done | `internal/component/l2tp/plugins/authradius/config_test.go` | |
| `prefix-filter-entry-order` | Done | `test/plugin/prefix-filter-entry-order.ci` | |
| Beyond the plan | Changed | see Notes | eleven more shipped: `TestPluginConfigSectionsCarryListOrderThroughJSON`, `TestToPluginMapEmitsNoOrderItDidNotRecord`, `TestToPluginMapDoesNotResurrectADeletedListsOrder`, `TestToPluginMapFollowsAnEntryDeletedAndWrittenAgain`, `TestEntriesAcceptASingleEntryWithNoOrder`, `TestEntriesAbsentListIsNotAnError`, `TestEntriesRefuseAMalformedList`, `TestOrderKeyCannotCollideWithAYANGNodeName`, `TestParseChainRefusesTermsWithNoOrder`, `TestParseChainSingleTermNeedsNoOrder`, `TestServerEntriesRefuseTwoServersWithNoOrder`, `TestParseConfigProductionTwoServersKeepsTheOperatorOrder`, `TestParsePrefixListsEvaluatesInTheOperatorsOrder`, and `TestLowerForPluginsGivesTheProviderTheOrderARollbackReplays` |
| Boundary test: entry count 0..N | Done | `TestEntriesAbsentListIsNotAnError` (0), `TestEntriesAcceptASingleEntryWithNoOrder` and `TestToPluginMapLeavesSingleEntryListsAlone` (1), `TestToPluginMapCarriesListOrder` (2), `TestToPluginMapEmitsNoOrderItDidNotRecord` (3) | the only numeric input this spec has is the entry count, and 2 is the first count that emits an order key |
| Interop test | Skipped | N-A | no wire-visible change: this is config lowering. The existing BGP policy interop scenarios already cover the filter surface |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/tree.go` | Done | `ToPluginMap`, `toMap`, `entryOrderLocked` |
| `internal/component/config/plugin_verify.go` | Done | also `ExtractConfigSubtree`, which carries the order out of the parent when the extracted node is the ordered list |
| `cmd/ze/hub/main.go` | Done | landed in `5b24b076f`; changed again at closure to seed the ConfigProvider from the same lowering |
| `cmd/ze/hub/main_reload.go` | Done | landed in `5b24b076f`; gained `lowerForPlugins` at closure |
| `cmd/ze/hub/main_pki.go` | Done | `configTreeFromMap` skips the reserved key |
| the three BGP filter `config.go` | Done | |
| `internal/component/firewall/config.go` | Done | landed in `0cc2cf949` |
| `internal/component/l2tp/plugins/authradius/config.go` | Done | `sort.` gone |
| `ai/rules/config.md` | Done | plus its point file, which is the canonical source the rule renders from |
| `internal/core/configorder/configorder.go` + `_test.go` | Done | |
| `internal/component/config/toplugin_order_test.go` | Done | |
| `test/plugin/prefix-filter-entry-order.ci` | Done | |
| Beyond the plan | Changed | `internal/component/config/setparser.go`, `internal/component/bgp/config/loader_create.go`, `internal/component/plugin/cli/test_cmd.go`, `internal/component/ike/engine/config.go`, `docs/architecture/api/process-protocol.md`, `docs/architecture/config/syntax.md`, `docs/architecture/core-design.md`, `ai/PACKAGE-MAP.md`, `cmd/ze/hub/main_reload_order_test.go`. Each is named in Deviations from Plan or Bugs Found/Fixed |

### Audit Summary
- **Total items:** 4 requirements + 9 ACs + 17 test rows + 13 file rows = 43
- **Done:** 41
- **Partial:** 0
- **Skipped:** 1 (the interop row, N-A with the reason the spec's own Interop table states: no wire-visible change)
- **Changed:** 2 (the "Beyond the plan" test row and file row, both recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| carry the order the Tree already holds across the JSON boundary | functional, through the daemon | `ze-test bgp plugin prefix-filter-entry-order` -> `1.9s 1/1 PASS 500 prefix-filter-entry-order`. The `.ci` loads a two-entry prefix-list through the real daemon, sends an UPDATE for `10.0.0.0/24`, and asserts adj-rib-in holds 0 routes plus `prefix-list reject` and `filter=ORDERED` on stderr, with `reject=stderr:pattern=prefix-list accept` |
| the three loud readers stop refusing a multi-entry list | unit, per reader, each with a swapped-order subtest | `TestParsePrefixListsTwoEntriesInNonLexicalOrder`, `TestParseAsPathListsTwoEntriesInNonLexicalOrder`, `TestParseCommunityListsTwoEntriesInNonLexicalOrder`. The fixture writes `10.0.0.0/8` before `0.0.0.0/0`, which is the reverse of the alphabet, so a sorted reader returns the opposite decision rather than a coincidentally correct one |
| the firewall stops evaluating a chain in Go map iteration order | unit, plus a refusal test | `TestParseChainTermOrderFollowsTheOperator` and `TestParseChainRefusesTermsWithNoOrder`. `parseChain` builds `chain.Terms` in the delivered order and the backend emits one nft rule per term in slice order, so `Terms` IS the kernel rule order |
| the RADIUS failover order stops being alphabetical | unit through the production pipeline | `TestParseConfigProductionTwoServersKeepsTheOperatorOrder` drives config text -> Tree -> `ToPluginMap` -> `parseConfigFromTree` with two servers written against the alphabet, and `grep -n 'sort\.' internal/component/l2tp/plugins/authradius/config.go` returns nothing |
| `ToMap` output is unchanged for its forty consumers | unit, negative | `TestToMapIsUnchangedByTheOrderedLowering` walks the whole lowered map at every depth and fails on any key starting with `configorder.KeyPrefix`. `TestToMapListIsAlwaysAKeyedMap` was not edited by this spec |
| a missing order is loud, never filled in | unit, at the shared reader and at the lowering | `TestEntriesRefuseAMultiEntryListWithNoOrder` and `TestToPluginMapEmitsNoOrderItDidNotRecord`. Because the refusal is in `configorder`, a sixth ordered list added later fails on its first two-entry config rather than evaluating in an order nobody chose |
| the order reaches a plugin on EVERY path that delivers config | unit, at the path that was missed | `TestLowerForPluginsGivesTheProviderTheOrderARollbackReplays` asserts both the coordinator's map and the map `rollbackReload` replays. Proven to discriminate: changing `lowerForPlugins` to `tree.ToMap()` fails it with "the map handed to the coordinator carries no entry order" |
| the discrimination is real, not vacuous | mutation of the producer | three separate reverts, each observed red then restored: `VerifyPluginConfig` to `tree.ToMap()`; `deleteFromList`/`deleteFromInlineList` back to the bare map delete; `lowerForPlugins` to `tree.ToMap()` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | the spec metadata sets the Deferral shard field to `-`, and `ls plan/deferrals/` holds no shard for this stem. Nothing to remove in commit B, and no foreign shard was emptied by these resolutions |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ordered-list-loses-its-order-9ad8358c-695f-41be-8019-5d92ba08f8e6.md` |
| `review_gate.py check` | clean |
| Rounds | 5 |
| Reviewer lenses used | round 1-2: logic+wiring, security+edge-cases, the lowering's own risk area (two independent contexts, before closure). Round 3-5: this closure agent, running wiring, removed-behavior, guard-audit, allocation, style, and the sibling-call-site sweep |

### Round scopes (written before each round ran)
| Round | Scope |
|-------|-------|
| 1 | the whole diff of `d687efe7e` |
| 2 | round 1's nine fixes and what they touched |
| 3 | the whole committed diff across `d687efe7e`, `e171cc978`, the two hub hunks of `5b24b076f` and the firewall hunk of `0cc2cf949`, plus every sibling call site: every mutator of `Tree.lists`/`Tree.listOrder` and every `.ToMap()` call site in `internal/` and `cmd/` |
| 4 | round 3's two fixes: `deleteFromList`/`deleteFromInlineList` and `lowerForPlugins` plus its two callers, and the two new tests |
| 5 | round 4's one fix: the clone in `lowerForPlugins` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1-9 | BLOCKER/ISSUE | round 1: 0 BLOCKER, 7 ISSUE, 2 NIT, all nine fixed and confirmed by round 2 | across `d687efe7e` | `d687efe7e` as amended before commit |
| 10 | ISSUE | round 2: `DeleteList` cleared `t.lists` and left `t.listOrder`, so a list rewritten under the same name lowered in the deleted list's order | `(*Tree).DeleteList` (`internal/component/config/setparser.go`) | `e171cc978`, plus `TestToPluginMapDoesNotResurrectADeletedListsOrder` |
| 11 | ISSUE | round 2: `treeFromMap`'s `[]any` arm rebuilt a reserved order key as a multi-value leaf | `treeFromMap` (`internal/component/ike/engine/config.go`) | `e171cc978` |
| 12 | BLOCKER | round 3: a whole-entry `delete` in the set-format parser bypassed `RemoveListEntry`, so a delete-then-rewrite delivered the entry at the position it was DELETED from. The duplicate order passes every check `entryOrderLocked` can make | `(*SetParser).deleteFromList`, `(*SetParser).deleteFromInlineList` (`internal/component/config/setparser.go`) | both call `tree.RemoveListEntry(name, key)`; `TestToPluginMapFollowsAnEntryDeletedAndWrittenAgain` |
| 13 | BLOCKER | round 3: the hub seeded the ConfigProvider with `ToMap` while the coordinator held `ToPluginMap`, so `rollbackReload` replayed an order-less config into the plugin server and `configorder.Entries` refused every multi-entry list on the recovery path | `runYANGConfig` Phase 2 (`cmd/ze/hub/main.go`) | `lowerForPlugins` (`cmd/ze/hub/main_reload.go`); `TestLowerForPluginsGivesTheProviderTheOrderARollbackReplays` |
| 14 | ISSUE | round 4: `lowerForPlugins` handed `Provider.SetRoot` the returned map's own submaps, and `SetRoot` stores the reference. The provider and the coordinator would have been two names for one map, which the reload path does not do | `lowerForPlugins` (`cmd/ze/hub/main_reload.go`) | `cloneStringAnyMap(sub)`, matching `applyLoadedTreeToProvider` |
| 15 | NOTE | round 3: the comment on `aaaAuthenticationChanged` said boot lowers with `ToMap` and a reload with `ToPluginMap`. Finding 13's fix makes both `ToPluginMap`, so the comment described the defect | `aaaAuthenticationChanged` (`cmd/ze/hub/main_reload.go`) | comment rewritten in the same change |

Round 5 found nothing in its scope, and no always-in-scope finding anywhere.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/configorder/configorder.go` | Yes | `-rw-r--r-- 1 thomas thomas 7170 Aug 24 00:58` |
| `internal/core/configorder/configorder_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 9092 Aug 24 00:59` |
| `internal/component/config/toplugin_order_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 13171 Aug 24 01:27` |
| `test/plugin/prefix-filter-entry-order.ci` | Yes | `-rw-r--r-- 1 thomas thomas 4642 Aug 23 17:50` |
| `cmd/ze/hub/main_reload_order_test.go` | Yes | added by this closure for finding 13 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a two-entry prefix-list loads through the daemon and the first entry wins | `ze-test bgp plugin prefix-filter-entry-order` at the closure tree: `1.9s 1/1 PASS 500 prefix-filter-entry-order`, `pass 1/1 100.0%` |
| AC-2 | the swap gives the opposite decision | `TestParsePrefixListsTwoEntriesInNonLexicalOrder` runs both orders as subtests; `internal/component/bgp/plugins/filter_prefix` `ok ... 1.058s` |
| AC-3 | as-path and community lists both load in order | `internal/component/bgp/plugins/filter_aspath` `ok ... 1.065s`, `internal/component/bgp/plugins/filter_community_match` `ok ... 1.048s` |
| AC-4 | `parseChain` returns terms in the operator's order on every run | `internal/component/firewall` `ok ... 1.667s` under `-race` |
| AC-5 | `serverEntries` is not sorted | `grep -n 'sort\.' internal/component/l2tp/plugins/authradius/config.go` -> no output; `internal/component/l2tp/plugins/authradius` `ok ... 3.083s` |
| AC-6 | a multi-entry list with no order is an error, never a fallback | `grep -n 'first-match-wins' internal/core/configorder/configorder.go` -> two hits, in the package doc and in `Entries`'s doc; `internal/core/configorder ok` |
| AC-7 | `ToMap` output is byte-identical | `internal/component/config ok ... 105.897s` under `-race`, `TestToMapIsUnchangedByTheOrderedLowering` and `TestToMapListIsAlwaysAKeyedMap` both in it |
| AC-8 | a single-entry list carries no order key | same suite, `TestToPluginMapLeavesSingleEntryListsAlone` |
| AC-9 | a reorder shows up in `DiffMaps` | same suite, `TestDiffMapsSeesAListReorder` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| operator config with a two-entry prefix-list, loaded by the daemon | `test/plugin/prefix-filter-entry-order.ci` | Yes. Read, not inferred: the `ze-bgp` stanza declares `prefix-list ORDERED` with `entry 10.0.0.0/8 { action reject }` before `entry 0.0.0.0/0 { action accept }`, `ze-peer` sends the UPDATE for `10.0.0.0/24`, and the observer asserts `total-routes == 0` plus three stderr assertions including a `reject=` on `prefix-list accept` |
| operator config with a two-entry prefix-list, verified at commit | `TestVerifyPluginConfigDeliversListOrder` | Yes. It builds sections through `VerifyPluginConfig`'s own path and through `BuildPluginConfigSections`, and asserts both deliver the same order. Reverting `VerifyPluginConfig` to `tree.ToMap()` makes it fail |
| firewall chain with two terms | `TestParseChainTermOrderFollowsTheOperator` | Yes. The fixture builds the delivered JSON with `configorder.OrderKey("term")` beside the term map, so it exercises the shape `ToPluginMap` emits rather than a hand-shaped one |
| a failed reload replaying the provider snapshot | `TestLowerForPluginsGivesTheProviderTheOrderARollbackReplays` | Yes. It calls the production `lowerForPlugins`, then `snapshotProvider` and `snapshotToLoadedTree`, which are the two functions `rollbackReload` composes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | no plugin rejects an unknown key at container level. The one strict reader, `parseFibVPPConfig` (`internal/plugins/fib/vpp/fibvpp.go`), reads a container of two leaves and no list, so no order key can be delivered into it. The functional suites ran green with the sidecar in every delivered payload |
| A-2 | **broken** | five producers exist, not two. `VerifyPluginConfig` (`internal/component/config/plugin_verify.go`), `CreateReactorFromTree` (`internal/component/bgp/config/loader_create.go`), `cmdPluginTest` (`internal/component/plugin/cli/test_cmd.go`), the hub's boot and reload lowerings, and the hub's ConfigProvider seeding, which reaches a plugin through `rollbackReload` -> `snapshotToLoadedTree` -> `Server.ReloadConfig`. The last was missed until closure and is finding 13. Derived by reading every `.ToMap()` call site in `internal/` and `cmd/`; the remaining ones are gNMI, `ze config show`, whole-config validation, the support bundle, the metrics exporter, four plugin doctor checks, `ResolveBGPTree`, and the CLI validator, none of which produces a plugin's config payload |
| A-3 | confirmed | `TestOrderKeyCannotCollideWithAYANGNodeName` pins the property, and RFC 7950 Section 6.2 is cited at the `KeyPrefix` declaration. The whole functional suite delivers `@`-prefixed keys with no collision |
| A-4 | confirmed | `TestDiffMapsSeesAListReorder`: two lowerings that differ only in one list's order produce a non-empty `DiffMaps` result, so `reloadConfig` reconfigures the plugin |

### Deliverables Verified
| Deliverable | Result | Fresh evidence |
|-------------|--------|----------------|
| the reserved key is spelled once | Yes | `grep -rn '"@" *$' --include=*.go internal/ cmd/` names only the `KeyPrefix` declaration in `internal/core/configorder/configorder.go`. The one other literal is `"@entry"` in that package's own `TestOrderKeyCannotCollideWithAYANGNodeName`, which is the assertion that DEFINES the spelling. `internal/component/firewall/config_test.go` builds the key with `configorder.OrderKey("term")` rather than a second literal, and so do the three filter tests and the authradius tests: 16 sites, all through `configorder` |
| no reader sorts an ordered list | Yes | `grep -n 'sort\.' internal/component/l2tp/plugins/authradius/config.go` returns nothing. The two permitted sorts are named in `ai/rules/config.md` with the fact each one sorts on: `parseERO` sorts a numeric hop index that IS the key, and `parsePolicyRoute` sorts an explicit `order` leaf |
| the three refusals still exist | Yes | `grep -n 'first-match-wins' internal/core/configorder/configorder.go` -> the package doc and `Entries`'s doc. The refusal moved from three copies to one owner, and each converted reader wraps it with its own list name |
| the functional test discriminates | Yes | the `.ci` fixture is chosen so the two failure modes are distinguishable: `0.0.0.0/0` sorts BEFORE `10.0.0.0/8`, so a sorted reader accepts the route and a reader with no order refuses the config. Both would break the `total-routes == 0` assertion |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| row 8, Plugin SDK/protocol changed -- `docs/architecture/api/process-protocol.md` | the example is rooted at `bgp` because `ExtractConfigSubtree` wraps a section body in its full path, read from the function; `@entry` sits beside `entry` inside `ORDERED` because `Tree.toMap` writes it into the same `result` map as the list | Yes |
| row 12, Internal architecture changed -- `docs/architecture/config/syntax.md` | the two-lowering table matches `ToMap` and `ToPluginMap`; the "one entry carries no sidecar" claim matches `len(listMap) > 1` | Yes |
| row 12 (added at closure) -- `docs/architecture/core-design.md` | the ConfigProvider bullet's claim traced to `lowerForPlugins` and to `rollbackReload` -> `snapshotToLoadedTree` -> `Server.ReloadConfig` | Yes |
| rows 1-7, 9-11, 13-15 answered No | no config text changes (the `.ci` uses today's syntax unchanged), no command, no RPC type, no plugin added or removed, no protocol bytes, no RFC obligation, no capability claim, no route metadata, no counter, no registry entry. `grep -rn 'ordered-by user' docs/` finds only the two docs this spec edited | Yes |
| row 16, source anchors on changed files | `make ze-discovery-index-update` produced no change to `ai/DOCS-TO-CODE.md`. `make ze-doc-verify`'s anchor pass reports one problem, and it names `cmd/ze/hub/aaa_authenticator_web.go`, another session's uncommitted file | Yes |
| row 17, existing examples for this area | `grep -rn 'prefix-list' docs/guide/` shows single-entry and multi-entry examples; a multi-entry example is now correct where it was refused before, so no doc example needed changing | Yes |

## Core Insight

Determinism is not correctness, and a keyed map is where the two get confused.
`serverEntries` met exactly this problem, sorted the map "for deterministic
server (failover) ordering", and shipped a wrong answer with a stable one's
reputation. The three BGP filter readers met the same problem and refused
instead, which is why the same defect was loud in one place and silent in
another for the same length of time.

The second lesson is about the SHAPE of the invariant. `t.lists` and
`t.listOrder` must only ever change together, and this spec fixed that one
mutator at a time: `AddListEntry` and `RemoveListEntry` were already right,
`DeleteList` was fixed by round 2, and the two entry-delete paths in the
set-format parser were still wrong when closure started. Each fix looked
complete because the mutator in hand was correct afterwards. The unit that was
actually broken was the pair, and the same shape produced finding 13 one layer
up: the plugin-facing lowering was applied to four of the five producers of a
plugin's config map, and the fifth only met the others on the rollback path.
