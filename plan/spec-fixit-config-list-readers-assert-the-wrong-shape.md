# Spec: fixit -- config list readers assert the wrong shape

| Field | Value |
|-------|-------|
| Status | done |
| Scope | config |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A YANG `leaf-list` and a YANG `list` each reach a config reader in a shape that
is not a slice of `any`. Five readers assert `[]any` on such a value. The
assertion fails, the `if` body never runs, and the operator's setting is
discarded with no error, no warning and no log line.

The failure has two modes.

| Mode | Node type | When the assertion fails | What the operator sees |
|------|-----------|--------------------------|------------------------|
| 1 | `leaf-list` | exactly one member | the option works with two values and vanishes with one |
| 2 | `list` | every count | the option never applies at all |

The goal is to remove the class: give the codebase one place that knows what
`Tree.ToMap` emits for each node type, route every leaf-list and list reader
through it, and delete the local spellings that taught the wrong shape.

The headline is a security defect. `anomaly/shape` reads its `allowlist`
leaf-list with a `[]any` assertion. An operator who allowlists exactly one
prefix, such as their own management network, gets no allowlist, and the
responder arms a shaping action against that network.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config.md` - the rule that already names the sibling defect (a native-type assertion on a delivered config value)
  → Constraint: every coercion of a delivered config value MUST accept the string form. A leaf-list carrying one member IS the string form of a list, so the same directive reaches it.
  → Decision: `scripts/checks/config_string_coercion.go` scans `internal/**/config.go` for numeric and bool assertions only. It does not see a `[]any` assertion, and it does not scan `register.go`, so neither the pool sites nor this class are mechanically gated today.
- [ ] `ai/rules/architecture.md` - package placement by dependency direction
  → Constraint: a helper used by `internal/component/*` and `internal/plugins/*` alike belongs in the leaf tier, `internal/core/`. Neither of the other two tiers can be imported by both.
  → Decision: abstract at 2+ use cases. Five defect sites plus five existing hand-rolled spellings is well past the threshold, so one shared helper is the root fix rather than an unasked abstraction.
- [ ] `ai/rules/simplicity.md` - the shape of the fix
  → Constraint: the fix MUST be at the ROOT of the defect. The root is the absence of one place that knows the producer's shapes, so patching five call sites and leaving five local spellings alive would leave the copy-source live.
- [ ] `ai/rules/no-layering.md` - replacing X with Y
  → Constraint: when `configvalue.LeafList` becomes the canonical spelling, the local copies MUST be deleted rather than left beside it.
- [ ] `ai/rules/interop-and-goal-validation.md` - vacuity traps
  → Constraint: a multi-member test passes with the bug in place and proves nothing. Every leaf-list test MUST exercise the SINGLE-member case.

**Key insights:** (minimal context to resume after compaction)
- `Tree.ToMap` collapses a one-member leaf-list to a bare string. That one branch is the whole defect class.
- A YANG `list` is never a slice at any count. It is a map keyed by the list key, and the key is not a field inside the entry.
- Two delivery paths exist. In-process readers get `[]string` at 2+ members; JSON-delivered readers get `[]any`. A correct helper accepts both, plus the bare string.
- This class has been fixed twice before, locally, in `l2tp/pppoe` and in `component/iface`. Both fixes left a local helper behind, and neither stopped the next reader.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/tree.go` - `(*Tree).ToMap` walks `t.multiValues` for leaf-lists and `t.lists` for lists. For a leaf-list it calls `activeMembersLocked`, which returns `[]string`, then switches on the count: zero omits the key, exactly one assigns `active[0]` (a bare `string`), and the default assigns `active` (a `[]string`). For a list it assigns a `map[string]any` of key to entry `ToMap`, at every count.
- [ ] `internal/plugins/anomaly/shape/config.go` - `ParseConfig` unmarshals its JSON argument, then asserts `m["allowlist"].([]any)`.
- [ ] `internal/plugins/anomaly/shape/responder.go` - `(*responder).onDetected` calls `allowlisted(entity, r.cfg.Allowlist)` and returns without arming when it is true.
- [ ] `internal/plugins/anomaly/shape/match.go` - `allowlisted` walks the configured prefixes.
- [ ] `internal/component/l2tp/plugins/pool/register.go` - `parseFullPoolConfig` unmarshals JSON, then asserts `poolBlock["named-pool"].([]any)` and `poolBlock["named-ipv6-pool"].([]any)`. `parseNamedPools` and `parseNamedIPv6Pools` take `[]any` and read `entry["name"]`. `handleIPRequest` refuses the session with `named pool <x> not configured` when the named table is nil.
- [ ] `internal/plugins/ospf/config.go` - `parseRouterInformation` asserts `m["scope"].([]any)`, then substitutes both `OpaqueScopeArea` and `OpaqueScopeAS` when the resulting list is empty and the feature is enabled. `configInstanceIDs`, in the same file, coerces the same producer correctly and documents the scalar case.
- [ ] `internal/component/firewall/config.go` - `parseFlowtable` asserts `m["device"].([]any)`.
- [ ] `internal/component/l2tp/pppoe/config.go` - `ExtractParameters` and `cfgStrings`. The `ExtractParameters` doc comment records this class being fixed here once already, and names the trap that two unit tests passed throughout because both built the map in a shape no producer emits.
- [ ] `internal/component/iface/config.go` - `parseStringList`, and the comment above the bridge `member` read recording this class being fixed here once already.
- [ ] `internal/plugins/ldp/register.go` - `configLeafList`.
- [ ] `internal/plugins/isis/config.go` - `configLeafList`, byte-identical to the LDP one, and `keyedList` for the YANG-list form.
- [ ] `internal/component/bgp/plugins/filter_community/config.go` - `anySliceToStrings`, the only local spelling that already handles all three shapes.

**Behavior to preserve:**
- `Tree.ToMap`'s output shapes. This spec changes readers only. The producer is not touched.
- Every reader's behavior at 2+ members, which is correct today.
- `filter_community.uint32List` keeps its two-stage read: string forms first, then a numeric `[]any`.
- `internal/component/l2tp/pppoe` keeps its own inline coercion of the `interface` YANG list.
- `internal/plugins/isis` keeps `keyedList`, which sorts numerically for `key-id` and has no counterpart in the shared helper.

**Behavior to change:**
- Five readers stop discarding the operator's value. Named in the AC table.

### Sites triaged and EXCLUDED (each confirmed at its producing function)

| Site | Symbol / key | Why it is not a defect |
|------|--------------|------------------------|
| `internal/plugins/policyroute/config.go` | `parsePolicyRoute`, `interface` | reads the `string` form before the `[]any` form; both shapes handled |
| `internal/plugins/copp/config.go` | `parsePorts`, `protected-port` | reads the `string` form first |
| `internal/plugins/copp/config.go` | `parseTrustedSources`, `trusted-source` | reads the `string` form first |
| `internal/component/l2tp/plugins/authlocal/register.go` | `parseUsersFromTree`, `user` (list) | falls back to `map[string]any` and reads the key as the name |
| `internal/component/bgp/plugins/filter_community/config.go` | `uint32List` | calls the string coercion first, which handles the bare string |
| `internal/component/bgp/plugins/role/config.go` | `parseExportTokens`, `export` | reads the `string` form before the `[]any` form |
| `internal/component/mcp/apps.go` | `clientSupportsUIApps`, `mimeTypes` | reads the MCP client's `initialize` capabilities, produced by the peer, not by `Tree.ToMap` |
| `internal/exabgp/bridge/bridge_event.go` | `convertNegotiated`, `families` / `send` / `receive` | reads the negotiated-session JSON whose producer is `peer.go`'s `caps["families"] = famStrs`, a marshaled Go slice, not the config tree |
| `internal/component/lg/*`, `internal/component/bgp/cli/*`, `internal/test/*`, `internal/component/command/pipe.go`, `internal/component/bgp/plugins/gr/gr.go`, `internal/component/bgp/plugins/nlri/flowspec/plugin_decode.go` | various | none reads a config tree; each reads a JSON document a Go marshaller produced |
| `internal/component/iface/config.go` | `parseStringList` and its six call sites | already accepts the bare string; migrated here for one spelling, not because it is broken |

### Wrongly excluded, corrected after review: the three BGP filter entry readers

`parsePrefixListEntries` (`filter_prefix/config.go`), `parseAsPathEntries`
(`filter_aspath/config.go`) and `parseCommunityEntries`
(`filter_community_match/config.go`) were listed above as handling the map form.
That is true and it is not sufficient, so the row was wrong.

Re-derived at the producer: the chain is `(*Tree).ToMap`, `ExtractConfigSubtree`,
`json.Marshal`, then `configjson.ParseBGPSubtree`, which is a plain unmarshal
that restructures nothing. So `entry` is a `map[string]any` at every count and
the `[]any` branch in all three is unreachable. The map branch then refuses two
or more entries, to protect the first-match-wins order a Go map cannot carry.
**A prefix-list with two entries cannot load.**

This is NOT fixed here, and the reason is that both available shortcuts are worse
than the defect:

| Shortcut | Why it is refused |
|----------|-------------------|
| Route through `ListEntries` | it sorts by key, so a lexical order would silently replace the operator's configured order in a first-match-wins evaluation. A loud refusal becomes a wrong answer. This is not a hypothetical: `serverEntries` (`internal/component/l2tp/plugins/authradius/config.go`) already took that route, sorting the keyed map by name "for deterministic server (failover) ordering", and its `list server` is declared `ordered-by user` (`yang/ze-l2tp-auth-radius-conf.yang`). RADIUS failover therefore follows alphabetical order rather than the order the operator wrote, with nothing to say so |
| Delete the unreachable `[]any` branch | its fixture is the ONLY multi-entry coverage these three have, so deleting the branch deletes the coverage (`ai/rules/testing.md`) |

The root is upstream of all three and outside this spec's blast radius: `entry`
is declared `ordered-by user`, `ToMap` lowers every list to an unordered map, and
eleven other lists carry the same declaration. Recorded as a journal row in
`plan/journal/green-that-could-not-have-been-red.md`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config text, for example `allowlist 10.0.0.0/8` inside `anomaly { shape { ... } }`.
- Format at entry: ze config syntax (set-command equivalent).

### Transformation Path
1. Parse: `internal/component/config/parser.go` builds a `*Tree`; a leaf-list lands in `t.multiValues`, a list in `t.lists`.
2. Lower: `(*Tree).ToMap` renders the tree as `map[string]any`. A leaf-list becomes a bare `string` (one member) or a `[]string` (2+). A list becomes `map[string]any` keyed by the list key.
3. Deliver, by one of two routes:
   - In-process: the component receives that `map[string]any` unchanged, so a 2+ leaf-list is `[]string`.
   - Out-of-process: the value is marshaled to JSON and the plugin's `ParseConfig` unmarshals it, so a 2+ leaf-list is `[]any`.
4. Read: the plugin or component coerces the value into its typed config struct.
5. Apply: the typed value reaches the feature (the responder's allowlist guard, the flowtable's device set, the OSPF opaque scopes, the pool table).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → in-process component | `map[string]any` from `ToMap`, no marshalling | Yes -- `TestToMapLeafListMemberCountShapes` asserts the three leaf-list shapes and the one list shape `(*Tree).ToMap` emits, and `TestLeafListCoercesEveryProducerShape` reads all three |
| Config tree → out-of-process plugin | JSON text into `ParseConfig` | Yes -- `TestParseConfigAllowlistSingleEntry` and `TestParseFullPoolConfigReadsNamedPoolMap` drive the readers with a JSON string, which is the shape the marshalled map arrives in |

### Integration Points
- `internal/core/configvalue` (new) - imported by both kinds of reader; depends on nothing but the standard library.
- Every migrated reader replaces a local helper call with a `configvalue` call.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Nothing is added to the path. `(*Tree).ToMap` has no diff in this spec, and each reader keeps its own entry point. Only the coercion at the end of the path moves into one function |
| No unintended coupling (components stay isolated) | Yes | `internal/core/configvalue` imports the standard library alone (`sort`). Nine packages across `internal/component/*` and `internal/plugins/*` import it, and none reaches another through it. `make ze-repository-tracked-build-check` compiles all six flavors green |
| No duplicated functionality (extends existing, does not recreate) | Yes | The spelling count went from five to one. `parseStringList`, `cfgStrings`, `configLeafList` (two copies) and `anySliceToStrings` are deleted, and a grep for their definitions under `internal/` returns nothing |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config load is a cold path, read once at commit. `LeafList` copies deliberately, so a caller cannot alias the delivered slice; `TestLeafListDoesNotAliasTheCallersSlice` pins it |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No registry is touched. `LeafList` and `ListEntries` take a value and return a value. Neither knows a plugin name, and no per-feature branch is added anywhere |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Tree.ToMap` emits a one-member leaf-list as a bare `string` | `internal/component/config/tree.go`, `(*Tree).ToMap`, read | the whole class is misdiagnosed and no fix is owed | `TestToMapLeafListMemberCountShapes` in the config package | confirmed |
| A-2 | `Tree.ToMap` emits a YANG `list` as `map[string]any` at every count, never a slice | `(*Tree).ToMap`, read; corroborated by `authlocal.parseUsersFromTree` and `pppoe.ExtractParameters`, which both read the map form | the pool sites are mode 1 rather than mode 2 and need different tests | same test, list arm | confirmed |
| A-3 | `anomaly/shape`, `ospf`, `firewall` and `l2tp/pool` all receive config as JSON, so `[]any` (not `[]string`) is their 2+ shape | a `json.Unmarshal` call in each of `ParseConfig`, `ospf` config resolution, `firewall` config parse and `parseFullPoolConfig` | the tests feed a shape no producer emits, the exact trap recorded in `pppoe.ExtractParameters` | the reader tests drive `ParseConfig` and `parseFullPoolConfig` with a JSON string | confirmed |
| A-4 | `internal/core/` is importable from `internal/plugins/*` and `internal/component/*` alike | `ai/rules/architecture.md` tier rule; existing imports of `internal/core/textbuf` from both trees | the helper cannot live in core and needs another home | `make ze-tier-check` inside `make ze-lint-changed` | confirmed |
| A-5 | Migrating `iface` off `parseStringList` changes no behavior an operator can reach | `parseStringList` returns a one-element slice holding the empty string for an empty string input; `ToMap` omits a leaf-list with zero active members, so an empty member is unreachable from config | a bridge, tunnel or address list gains or loses an empty entry | the iface package's existing tests, re-run | confirmed |
| A-6 | `test/l2tp/radius-framed-ip.ci` passes today while the named pool is never configured | its `expect` lines check the exit code, one stdout line and the substring `l2tp-pool: configured`, none of which reads the named-pool count | the site is not broken, or the `.ci` is already red for another reason | run the `.ci` before the fix and read the log | confirmed: under the reverted coercion the suite goes red on the new `named-pools=1` line ONLY, while the pre-existing `l2tp-pool: configured` line still passes |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A migrated reader silently changes behavior at 2+ members | that package's existing tests go red | run each migrated package's full unit suite, not only the new tests |
| R-2 | The shared helper returns an empty non-nil slice where a caller distinguishes nil from empty | a caller's length guard flips | the helper returns nil for every empty result; asserted in its own test |
| R-3 | A concurrent session edits one of the five defect files under this work | a build error naming a symbol this spec did not touch | check `git status` for the file before editing it; never restore a shared file with a git verb |
| R-4 | The OSPF fix changes a router's flooding scope | a router-information LSA floods at one scope where it flooded at two | this IS the fix: one configured scope must mean one scope. Ze has not shipped a release (`ai/rules/config.md`), so no deployment depends on the wrong value |
| R-5 | The `.ci` strengthening makes an already-slow suite red for an unrelated reason | the `.ci` fails on a line this spec did not add | run that one `.ci` before and after, and read both logs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An anomaly responder shapes traffic to a protected network; an L2TP subscriber whose RADIUS profile names a pool is refused; a firewall flowtable offloads nothing; an OSPF router floods router-information at the wrong scope. All four are already broken; a wrong fix keeps one of them broken or breaks the 2+ case that works today. |
| How is it reverted? | Single commit revert. No config migration, no on-disk state, no wire format. |
| Who else touches this path? | A concurrent session is editing `internal/plugins/anomaly/shape/match.go`, `responder.go` and `testsupport.go`, and `internal/component/firewall/registry.go` and `backend.go`, in this checkout. This spec touches `anomaly/shape/config.go` and `firewall/config.go`, which that session has not modified. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `allowlist 10.0.0.0/8` (one member) in `anomaly { shape }` | → | `ParseConfig` → `Config.Allowlist` → `(*responder).onDetected` → `allowlisted` | `TestOnDetectedSkipsSingleAllowlistedPrefix` |
| `named-pool gold { ... }` in `l2tp { pool }` | → | `parseFullPoolConfig` → `poolConfigResult.namedPools` → `handleIPRequest` | `TestHandleIPRequestAcceptsFramedPoolFromKeyedList` |
| `named-ipv6-pool gold { ... }` in `l2tp { pool }` | → | `parseFullPoolConfig` → `poolConfigResult.namedV6Pools` | `TestParseFullPoolConfigReadsNamedIPv6PoolMap` |
| `scope area` (one member) in `ospf { router-information }` | → | `parseRouterInformation` → `routerInformationConfig.Scopes` | `TestParseRouterInformationSingleScope` |
| `device eth0` (one member) in a firewall flowtable | → | `parseFlowtable` → `Flowtable.Devices` | `TestParseFlowtableSingleDevice` |
| a `named-pool` in a running daemon's config | → | the l2tp pool plugin's startup log | `test/l2tp/radius-framed-ip.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `anomaly/shape` config names exactly one `allowlist` prefix | `Config.Allowlist` holds that one prefix |
| AC-2 | one `allowlist` prefix is configured, and an anomaly is detected for an entity inside it | the responder arms nothing |
| AC-3 | `l2tp` config declares one `named-pool` | that pool is present in the parsed result under its config key as its name |
| AC-4 | `l2tp` config declares two `named-pool` entries | both are present, each under its own key |
| AC-5 | a session's RADIUS profile names a configured `named-pool` | the IP request is accepted and served from that pool's range and gateway |
| AC-6 | `l2tp` config declares one or more `named-ipv6-pool` entries | each is present in the parsed result under its key |
| AC-7 | `ospf` `router-information` is enabled with exactly one `scope` | that one scope is configured, and the other is NOT added |
| AC-8 | a firewall flowtable names exactly one `device` | the flowtable holds that one device |
| AC-9 | a leaf-list value arrives as a bare string, a `[]string`, or a `[]any` | the shared reader returns the same `[]string` for all three, and nil for an absent, empty or wrongly typed value |
| AC-10 | a YANG list value arrives as a key-to-entry map | the shared reader returns one entry per key, carrying the key and the entry fields, ordered by key |
| AC-11 | `Tree.ToMap` renders a leaf-list at zero, one and two members and a list at one and two entries | the emitted shapes are absent, `string`, `[]string`, and `map[string]any` respectively |
| AC-12 | the repository is searched for a leaf-list coercion helper | exactly one exists, in `internal/core/configvalue` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | allowlists their single management prefix, then a burst is detected from it | config → `ToMap` → JSON → `ParseConfig` → `Config.Allowlist` → `onDetected` → `allowlisted` | `TestOnDetectedSkipsSingleAllowlistedPrefix` |
| 2 | configures one named L2TP pool and connects a subscriber whose RADIUS profile names it | config → `ToMap` → JSON → `parseFullPoolConfig` → `namedPools` → `handleIPRequest` | `TestHandleIPRequestAcceptsFramedPoolFromKeyedList`, `test/l2tp/radius-framed-ip.ci` |
| 3 | enables OSPF router-information for the area scope only | config → `ToMap` → JSON → `parseRouterInformation` → `Scopes` | `TestParseRouterInformationSingleScope` |
| 4 | adds one device to a firewall flowtable | config → `ToMap` → JSON → `parseFlowtable` → `Flowtable.Devices` | `TestParseFlowtableSingleDevice` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLeafListCoercesEveryProducerShape` | `internal/core/configvalue/configvalue_test.go` | AC-9: bare string, `[]string`, `[]any`, nil, empty, wrong type | |
| `TestListEntriesReadsKeyedMap` | `internal/core/configvalue/configvalue_test.go` | AC-10: one and several entries, key carried, ordered by key, non-map values skipped | |
| `TestToMapLeafListMemberCountShapes` | `internal/component/config/tomap_shape_test.go` | AC-11: the producer contract the helper is written against | |
| `TestParseConfigAllowlistSingleEntry` | `internal/plugins/anomaly/shape/config_allowlist_test.go` | AC-1 | |
| `TestOnDetectedSkipsSingleAllowlistedPrefix` | `internal/plugins/anomaly/shape/config_allowlist_test.go` | AC-2 | |
| `TestParseFullPoolConfigReadsNamedPoolMap` | `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | AC-3, AC-4 | |
| `TestHandleIPRequestAcceptsFramedPoolFromKeyedList` | `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | AC-5 | |
| `TestParseFullPoolConfigReadsNamedIPv6PoolMap` | `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | AC-6 | |
| `TestParseRouterInformationSingleScope` | `internal/plugins/ospf/router_information_scope_test.go` | AC-7 | |
| `TestParseFlowtableSingleDevice` | `internal/component/firewall/flowtable_device_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| leaf-list member count | 0..n | 2 | N/A | N/A |
| list entry count | 0..n | 2 | N/A | N/A |

The only boundary this class has is the member count, and it is not numeric
input: zero, one and two are the three cases, and one is the case that fails
today. No numeric leaf's range changes in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-framed-ip` | `test/l2tp/radius-framed-ip.ci` | operator configures one named pool; the daemon reports it configured, by count | |

The `.ci` already declares a `named-pool` and already passes with the pool
discarded, because its assertions never read the count. One assertion is added
so it discriminates.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-sr-frr` | `test/interop/scenarios/` | FRR | RUN. Its `ze.conf` declares exactly `scope area`, so it is the discriminating case for the OSPF fix. Under the fix FRR still saw what it expects: `FRR OSPF adjacency is Full` and `Ze advertised SR-Algorithm 0, SRGB and its node Prefix-SID` both passed. It then failed on a later step for an environment reason this change cannot reach: `mpls binary in the Ze container: ABSENT`, `kernel MPLS (/proc/sys/net/mpls): ABSENT` | run, RI assertions pass |
| `ospf-ti-lfa-frr` | `test/interop/scenarios/` | FRR | RUN. Also declares `scope area`. Failed before any router-information LSA was exchanged: adjacency stuck at `ExStart` with repeated `failed to set into network namespace ... operation not permitted`. A container-privilege failure on this machine, so it proves nothing either way about the scope change | run, unproven |
| `ospfv3-sr-frr` | `test/interop/scenarios/` | FRR | RUN. Failed at config load: `unknown field in ipv6: router-information`. Its `ze.conf` puts the container under `address-family ipv6`, and the schema declares it once, directly under `ospf`. The scenario has never exercised OSPFv3 router-information at all. Recorded in `plan/journal/documentation-shows-config-the-parser-refuses.md` | run, pre-existing red |

**This spec DOES change operator-visible behavior on upgrade, and the earlier
claim that it did not was wrong.** An OSPF router whose config names exactly one
`scope` advertised router-information at BOTH area and AS scope before this
change, because the emptied scope list fell through to the enabled-with-no-scope
default. It now advertises at the one configured scope. That is the fix working
and it is a real change to what the router floods, so an operator who relied on
the second scope arriving must add it explicitly. No wire FORMAT changes: the LSA
encoder is untouched, and only the set of scopes it is asked to originate moves.

## Files to Modify
- `internal/plugins/anomaly/shape/config.go` - `allowlist` read via `configvalue.LeafList`
- `internal/plugins/ospf/config.go` - `scope` read via `configvalue.LeafList`
- `internal/component/firewall/config.go` - `device` read via `configvalue.LeafList`
- `internal/component/l2tp/plugins/pool/register.go` - `named-pool` and `named-ipv6-pool` read via `configvalue.ListEntries`; `parseNamedPools` and `parseNamedIPv6Pools` take entries and use the key as the name
- `internal/component/iface/config.go` - delete `parseStringList`, five call sites use `configvalue.LeafList`
- `internal/component/iface/config_ra.go` - one call site
- `internal/plugins/ldp/register.go` - delete `configLeafList`, one call site
- `internal/plugins/isis/config.go` - delete `configLeafList`, one call site
- `internal/component/l2tp/pppoe/config.go` - delete `cfgStrings`, two call sites
- `internal/component/bgp/plugins/filter_community/config.go` - delete `anySliceToStrings`, six call sites
- `test/l2tp/radius-framed-ip.ci` - one assertion so the named pool is actually checked
- `ai/rules/config.md` - a directive naming the leaf-list and list shapes and the helper that reads them

## Files to Create
- `internal/core/configvalue/configvalue.go` - `LeafList` and `ListEntries`
- `internal/core/configvalue/configvalue_test.go` - the shape contract
- `internal/component/config/tomap_shape_test.go` - the producer contract
- `internal/plugins/anomaly/shape/config_allowlist_test.go`
- `internal/component/l2tp/plugins/pool/named_pool_shape_test.go`
- `internal/plugins/ospf/router_information_scope_test.go`
- `internal/component/firewall/flowtable_device_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No leaf, container or list is added, renamed or retyped. Every node this spec reads already exists |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command surface changes |
| CLI grammar (keyword before value) | N-A | No command surface changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/l2tp/radius-framed-ip.ci` gains the assertion that discriminates |
| Pipe completeness | N-A | No command output changes |
| Env var registration | N-A | No `environment/` leaf added |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, module, binary or certificate is added. The change is a pure in-process coercion |
| Prometheus counters/metrics | N-A | No new observable state. The anomaly responder's existing arm-refused counter is unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Nothing is added. Four existing documented options start working |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` if it states what `ToMap` emits per node type; verified at implementation |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | no plugin is added, removed or re-registered |
| 6 | Has a user guide page? | No | the affected options are already documented; their documented meaning is what this restores |
| 7 | Wire format changed? | No | no encoder is touched |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin` and `pkg/ze` are untouched. The config JSON on the wire is unchanged; only its reader changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7770 scope selection was already implemented; this makes a configured value reach it. No `rfc/short/` row changes level or polarity |
| 10 | Test infrastructure changed? | No | no runner, tag or harness changes |
| 11 | Affects daemon comparison? | No | no feature is gained or lost against another daemon |
| 12 | Internal architecture changed? | Yes | a new leaf package `internal/core/configvalue`; named in `ai/rules/config.md` so the next reader finds it |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED by `python3 scripts/dev/spec_doc_anchors.py plan/spec-fixit-config-list-readers-assert-the-wrong-shape.md`. Eight design docs are DECLARED by the changed files and each is unaffected, for one reason: this spec changes how a value is READ out of the delivered config map, and none of these documents describes that coercion. `docs/architecture/anomaly/anomaly-2-shape.md` describes the shaping responder and its allowlist semantics, which are restored rather than changed. `docs/architecture/ospf/ospf-4-component-config.md` and `docs/architecture/isis/isis-4-component-config.md` describe the config resolvers' outputs, which are unchanged. `docs/architecture/l2tp/bng-5-pppoe.md` and `docs/research/l2tpv2-ze-integration.md` describe the BNG session and pool model, not the parse. `docs/architecture/ldp/mpls-ldp.md` describes LDP protocol behavior. `docs/architecture/core-design.md` describes the config pipeline at the `File -> Tree -> map` level, which is the producer this spec deliberately does not touch. `docs/features/interfaces.md` lists interface features, none gained or lost. Advisory `<!-- source: -->` mentions (`docs/guide/configuration.md`, `docs/guide/ospf.md`, `docs/guide/firewall.md`, `docs/features.md`, and the four architecture pages that mention rather than declare) are unaffected for the same reason, and row 17 covers whether any of them shows a broken example |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verified at implementation: the check is that no doc documents the broken behavior as intended |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create the shared reader and pin the producer contract it is written against
   - Tests: `TestLeafListCoercesEveryProducerShape`, `TestListEntriesReadsKeyedMap`, `TestToMapLeafListMemberCountShapes`
   - Files: `internal/core/configvalue/configvalue.go`, `internal/core/configvalue/configvalue_test.go`, `internal/component/config/tomap_shape_test.go`
   - Verify: the producer test states, as an assertion, what `ToMap` emits at zero, one and two leaf-list members and at one and two list entries. The helper handles exactly those shapes. `make ze-tier-check` accepts the package placement
2. **Phase: the security site** -- `anomaly/shape` allowlist
   - Tests: `TestParseConfigAllowlistSingleEntry`, `TestOnDetectedSkipsSingleAllowlistedPrefix`
   - Files: `internal/plugins/anomaly/shape/config.go`, `internal/plugins/anomaly/shape/config_allowlist_test.go`
   - Verify: both tests fail against the current reader and pass after it. The responder test drives `onDetected`, not `allowlisted` alone, so it proves the guard is reached
3. **Phase: the YANG-list sites** -- l2tp named pools
   - Tests: `TestParseFullPoolConfigReadsNamedPoolMap`, `TestParseFullPoolConfigReadsNamedIPv6PoolMap`, `TestHandleIPRequestAcceptsFramedPoolFromKeyedList`
   - Files: `internal/component/l2tp/plugins/pool/register.go`, `internal/component/l2tp/plugins/pool/named_pool_shape_test.go`, `test/l2tp/radius-framed-ip.ci`
   - Verify: `parseNamedPools` and `parseNamedIPv6Pools` read the name from the key, since a keyed list does not carry its key as a field
4. **Phase: the remaining leaf-list sites** -- OSPF scope, firewall device
   - Tests: `TestParseRouterInformationSingleScope`, `TestParseFlowtableSingleDevice`
   - Files: `internal/plugins/ospf/config.go`, `internal/component/firewall/config.go`, and their two new test files
   - Verify: the OSPF test asserts the single configured scope AND asserts the second scope is absent, because the current code produces a wrong value rather than an empty one
5. **Phase: one spelling** -- delete the local helpers
   - Tests: each migrated package's existing suite, re-run
   - Files: `internal/component/iface/config.go`, `internal/component/iface/config_ra.go`, `internal/plugins/ldp/register.go`, `internal/plugins/isis/config.go`, `internal/component/l2tp/pppoe/config.go`, `internal/component/bgp/plugins/filter_community/config.go`
   - Verify: a grep for the four local helper names finds no definition left. Every migrated package is green
6. **Phase: discovery** -- make the next reader unable to get it wrong
   - Tests: none (documentation)
   - Files: `ai/rules/config.md`, `docs/architecture/config/syntax.md` if applicable
   - Verify: the rule names the three leaf-list shapes, the one list shape, and `internal/core/configvalue` as the only reader

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has an implementation and a test, each named by file and symbol |
| Feature completeness | Each of the four user stories runs end to end, and story 1 reaches `onDetected`, not just `ParseConfig` |
| Correctness | The single-member case is asserted for every leaf-list site. A multi-member-only test would pass with the bug in place |
| Correctness | The YANG-list sites read the entry NAME from the map key. A keyed list entry does not carry its key as a field, so reading a `name` field would leave every pool nameless |
| Correctness | OSPF asserts the absent scope as well as the present one, because the defect substitutes a wrong value rather than an empty one |
| Naming | `LeafList` and `ListEntries` name the YANG node types they read, so the call site says which node type it is reading |
| Data flow | The producer is untouched. `internal/component/config/tree.go` has no diff |
| Rule: `no-layering.md` | No local leaf-list coercion helper survives beside the shared one |
| Rule: `simplicity.md` | Two functions, no options, no interface, no registry. `ListEntries` is separate from `LeafList` because a list and a leaf-list return different types |
| Rule: `evidence.md` | Every excluded site's exclusion names the producing function that makes it correct |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/configvalue` exists and is in the leaf tier | `make ze-tier-check` |
| No local leaf-list coercion helper survives | a grep for `func parseStringList`, `func configLeafList`, `func cfgStrings` and `func anySliceToStrings` under `internal/` returns nothing |
| No config-tree reader asserts `[]any` on a leaf-list or list value | re-run the sweep for `.([]any)` under `internal/` and confirm every remaining hit is on the excluded list above |
| Every AC has a test | `make ze-unit-pkg-test` for each affected package |
| The `.ci` discriminates | run it with the fix reverted and observe red |
| Lint | `make ze-lint-changed` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Fail-closed guard | `allowlisted` decides whether to arm a shaping action. A reader that drops the allowlist makes the guard fail OPEN. The test must prove the guard is reached from `onDetected`, not that `allowlisted` returns true when handed a list |
| Input validation | `LeafList` must not panic on any `any`. A wrongly typed value returns nil, and every caller already treats nil as "not configured" |
| Resource exhaustion | `ListEntries` allocates one slice sized by the map. Config size is operator-bounded and read once at commit |
| Authorization failing open | The l2tp pool path fails CLOSED today (a session is refused). The fix must not turn a missing pool into an accepted session: the miss on a name that is not in the table stays a refusal |

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

## Design Insights

- The class survives because the wrong shape is invisible at the call site. Asserting `[]any` on a map value reads as a list read, and nothing about it says the producer collapses one member to a scalar. Naming the helper after the YANG node type puts the question back where the operator's config answers it.
- Two prior fixes of this same class, in `l2tp/pppoe` and in `component/iface`, each ended with a local helper. Both wrote an accurate comment explaining the shape, and neither comment was where the next reader was looking. A local fix to a shared-producer defect does not generalize by itself.
- The two failure modes have different operator experiences, and that matters for the tests. Mode 1 is intermittent by member count, so the operator concludes their config is subtly wrong. Mode 2 is total, so they conclude the feature does not work. The OSPF site is worse than both: it substitutes a plausible wrong value.
- `Tree.ToMap` collapsing at exactly one member is a design choice, not an accident: it keeps the JSON of a single-value leaf-list scalar. Changing the producer would break every reader that already handles the scalar. The reader side is where the fix belongs.
- Every one of the five sites had test coverage, and every one of those tests fed a shape the producer cannot emit: a JSON array of one for the leaf-lists, an array of entries each carrying a `name` field for the lists. A hand-built fixture is a second, silent claim about the producer, and when a reader and its test share the same wrong claim they agree with each other forever. This is what `pppoe.ExtractParameters` already recorded, in the same words, about the same class.
- The `.ci` failed the same way for the same reason: it configured a named pool and asserted the substring `l2tp-pool: configured`, a line the daemon logs whether the pool table has one entry or none. Asserting a COUNT rather than an event is what made it discriminate.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One shared helper in `internal/core/configvalue` | fix each of the five sites locally, as the two prior fixes did | five defect sites and five existing local spellings is far past the 2-use-case threshold in `ai/rules/architecture.md`, and the local route has already been taken twice without stopping the next reader |
| Two functions, `LeafList` and `ListEntries` | one function returning `any`, letting the caller switch | a leaf-list yields `[]string` and a list yields keyed entries. One function returning both would need a type switch at every call site, which is the machinery this removes |
| Signature `LeafList(v any) []string` | `LeafList(m map[string]any, key string) []string` | the map form gives no extra safety (an absent key is nil, and nil yields nil) and does not serve `filter_community.uint32List`, which holds a value with no map in scope |
| Fix the reader, not the producer | make `ToMap` always emit a slice for a leaf-list | the scalar form is load-bearing for every reader that already handles it, including the two prior fixes; changing the producer has a far larger blast radius |
| Delete the local helpers | leave them, since each is correct on its own delivery path | `ai/rules/no-layering.md`: a canonical spelling beside five local ones is the layering the rule bans, and three of the five handle a different subset of shapes, so each is a wrong model for the next reader to copy |
| Leave `isis.keyedList` and `pppoe`'s inline `interface` walk | migrate them to `ListEntries` too | `keyedList` sorts numerically for `key-id`, which the shared function does not do; adding that option would be machinery for one caller. The `pppoe` walk is correct and carries the comment recording the prior fix |
| No mechanical gate in this spec | extend `scripts/checks/config_string_coercion.go` to flag `[]any` assertions | a `[]any` assertion is legitimate wherever the map or string form is ALSO handled, which is seven of the excluded sites, and the existing check scans only `internal/**/config.go`, so it cannot see `register.go`. A gate with that false-positive rate would teach people to allowlist rather than to fix. Deleting the local spellings removes the copy-source instead |

## Known Limitations

- `internal/plugins/isis` keeps `keyedList`, and `internal/component/l2tp/pppoe` keeps its inline `interface` list walk. Both are correct; neither is a leaf-list reader. Named in Key Design Decisions.
- No mechanical check refuses a future `[]any` assertion on a config-tree value. The controls are the shared helper, the rule text, and the absence of a local spelling to copy.
- `filter_community.uint32List` keeps its two-stage read. Its second stage handles a numeric `[]any`, which `LeafList` does not produce, so both stages are still needed.
- **`internal/plugins/ospf/ri_test.go` carries the tenth and largest instance of the array-of-one fixture shape, and it is deliberately NOT corrected here.** `riCfg` builds `"scope":[...]` unconditionally and has sixteen single-scope callers, so it is the biggest single copy-source of the shape left in the tree. Two facts put it outside this spec. It is RFC-tagged (RFC7770-2.4-1, 2.4-2, 2.6-2), so touching it makes `scripts/dev/audit-test-relaxation.py` report an eighth finding whose only remedy is a row in `test/rfc-changed.md`, and that row is an owner approval nobody else may write. And correcting it buys this spec no proof: `parseRouterInformation` substitutes area and AS when an enabled feature lists no scope, so a presence assertion passes on that superset and a corrected fixture would discriminate nothing. It belongs in a commit that can carry the owner row, together with the sixteen callers.

## RFC Documentation (Scope: protocol)

N-A. No RFC-enforcing code is added or changed. The OSPF fix makes an already
implemented RFC 7770 scope selection receive the operator's configured value;
the code that acts on that value, and its RFC comments, are unchanged.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
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

The feature work landed as commit `0cc2cf949`, "fix(config): read a leaf-list and
a list through one reader". Closure adds one doc-comment fix and this record.

### What Was Implemented
- `internal/core/configvalue`: `LeafList(any) []string` and
  `ListEntries(any) []ListEntry`, one place that knows every shape
  `(*Tree).ToMap` and the JSON delivery after it can emit.
- Five readers stopped asserting `[]any`: `ParseConfig`
  (`internal/plugins/anomaly/shape/config.go`), `parseFullPoolConfig`,
  `parseNamedPools` and `parseNamedIPv6Pools`
  (`internal/component/l2tp/plugins/pool/register.go`), `parseRouterInformation`
  (`internal/plugins/ospf/config.go`) and `parseFlowtable`
  (`internal/component/firewall/config.go`).
- Five local coercion spellings deleted: `parseStringList` (iface), `cfgStrings`
  (pppoe), `configLeafList` (ldp and isis, byte-identical copies) and
  `anySliceToStrings` (filter_community). Their call sites now read through
  `configvalue`.
- `(*Tree).ToMap` is untouched. The producer contract it holds is now asserted
  rather than assumed, by `internal/component/config/tomap_shape_test.go`.
- `test/l2tp/radius-framed-ip.ci` gained `expect=stderr:contains=named-pools=1`,
  the assertion that tells a configured pool table from an empty one.

### Bugs Found/Fixed
- The headline is a fail-open guard. `allowlisted` decides whether the shaping
  responder arms, and a one-prefix allowlist parsed empty, so the guard never
  ran and the responder armed against the network the operator protected.
  Covered by `TestOnDetectedSkipsSingleAllowlistedPrefix`, which drives
  `(*responder).onDetected` rather than `allowlisted`, and by
  `TestOnDetectedArmsASourceOutsideTheSingleAllowlistedPrefix`, its negative
  half.
- Nine test fixtures across five packages fed a shape no producer emits. Each is
  corrected to the producer's shape, with no assertion removed.
  `python3 scripts/dev/audit-test-relaxation.py 0cc2cf949~1` reports no
  `[DELETED]` and no `[WEAKENED]` finding on any file this spec touched.
- Found and NOT fixed here: three BGP filter entry readers cannot load a
  two-entry list. Recorded in
  `plan/journal/green-that-could-not-have-been-red.md` and owned by
  `spec-fixit-ordered-list-loses-its-order`.

### Documentation Updates
- `docs/architecture/config/syntax.md` gained "What a reader receives": the
  shape table per node type and per delivery path, with source anchors on
  `(*Tree).ToMap` and on `LeafList, ListEntries`.
- `ai/rules/config.md` gained the `config-list-shapes` point group, three
  points, listed in `ai/rules/points/config/manifest.md`.
- `ai/PACKAGE-MAP.md` gained the `internal/core/configvalue` row.
- `python3 scripts/dev/spec_doc_anchors.py plan/spec-fixit-config-list-readers-assert-the-wrong-shape.md`
  exits 0 and names no unnamed DECLARED document. It notes four documents that
  merely MENTION a changed file (`docs/DESIGN.md`,
  `docs/architecture/iface/logical-name-resolution.md`,
  `docs/architecture/traffic/cos-plugin.md`,
  `docs/architecture/traffic/cp-survival-2-copp-port179.md`); none describes the
  coercion of a delivered config value, so none is stale.
- `make ze-repository-check` reports 9 issues, none on a file this spec touched.
  All 9 are uncommitted exported symbols in `internal/component/command/`,
  `internal/component/iface/iface.go` and `internal/component/web/testing/`,
  owned by other sessions.

### Deviations from Plan
- The spec planned no operator-visible change and then corrected itself. An OSPF
  router whose config names one `scope` advertised router-information at BOTH
  area and AS before, and advertises at the one configured scope now. The spec
  body states it; it is repeated here so a reader of the closure meets it too.
- Commit `0cc2cf949` also carried the `parseChain` change in
  `internal/component/firewall/config.go`, which belongs to
  `spec-fixit-ordered-list-loses-its-order`, and the `ordered-by user`
  paragraphs of the rule point and of `docs/architecture/config/syntax.md`. That
  content names `internal/core/configorder` while the package itself was
  untracked, so `main` did not build from a fresh clone until `d687efe7e` landed
  it. Recorded as a journal row.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Commit `0cc2cf949`'s `--file` list named `internal/component/firewall/config.go`, and the path carried a sibling spec's `configorder` hunk as well as this spec's `parseFlowtable` hunk. Its message states "the rule point names no symbol from the unlanded ordering work", which is false for the content it committed | A `--file` list fixes WHEN a path is staged, never WHOSE content is in it (`ai/rules/git-safety.md`). The check that would have caught it, `make ze-repository-tracked-build-check`, is required by that rule immediately after a commit carrying `.go`, and it was not run | Reported by the main thread at closure, after `d687efe7e` had already repaired it by landing the package | Journal row in `plan/journal/gate-excludes-part-of-its-population.md`. Re-verified at closure: `make ze-repository-tracked-build-check` is green on all six flavors, and every package imported by every file this commit touched has tracked `.go` files |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One place knows what `Tree.ToMap` emits per node type | Done | `internal/core/configvalue/configvalue.go`, `LeafList` and `ListEntries` | The package doc states all four shapes and both delivery paths |
| Every leaf-list and list reader routes through it | Done | Fifteen call sites across nine packages | `grep -rn "configvalue.LeafList\|configvalue.ListEntries" internal/` |
| The local spellings that taught the wrong shape are deleted | Done | iface, pppoe, ldp, isis, filter_community | A grep for the four definitions returns nothing |
| The security site stops failing open | Done | `internal/plugins/anomaly/shape/config.go`, `ParseConfig` | Proven from `(*responder).onDetected`, both polarities |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestParseConfigAllowlistSingleEntry` | Four cases: one member, two, absent, one unparseable |
| AC-2 | Done | `TestOnDetectedSkipsSingleAllowlistedPrefix` | Drives `onDetected`; asserts no term installed and `armedCount` zero |
| AC-3 | Done | `TestParseFullPoolConfigReadsNamedPoolMap` | The name is read from the list KEY |
| AC-4 | Done | `TestParseFullPoolConfigReadsNamedPoolMap` two-entry case, `TestParseNamedPoolConfig` | |
| AC-5 | Done | `TestHandleIPRequestAcceptsFramedPoolFromKeyedList` | Range and gateway asserted |
| AC-6 | Done | `TestParseFullPoolConfigReadsNamedIPv6PoolMap`, `TestParseNamedIPv6PoolConfig` | |
| AC-7 | Done | `TestParseRouterInformationSingleScope` | Asserts the configured scope AND the absence of the other |
| AC-8 | Done | `TestParseFlowtableSingleDevice` | |
| AC-9 | Done | `TestLeafListCoercesEveryProducerShape`, `TestLeafListDoesNotAliasTheCallersSlice` | Bare string, `[]string`, `[]any`, nil, empty, wrong type |
| AC-10 | Done | `TestListEntriesReadsKeyedMap` | Key carried, ordered by key, non-map bodies skipped |
| AC-11 | Done | `TestToMapLeafListMemberCountShapes`, `TestToMapListIsAlwaysAKeyedMap`, `TestToMapKeyedListOmitsTheKeyLeafFromTheEntry` | Driven through the real `*Tree` |
| AC-12 | Done | A grep for the four deleted helper definitions returns nothing | Exactly one coercion survives, in `internal/core/configvalue` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLeafListCoercesEveryProducerShape` | Done | `internal/core/configvalue/configvalue_test.go` | |
| `TestListEntriesReadsKeyedMap` | Done | `internal/core/configvalue/configvalue_test.go` | |
| `TestToMapLeafListMemberCountShapes` | Done | `internal/component/config/tomap_shape_test.go` | |
| `TestParseConfigAllowlistSingleEntry` | Done | `internal/plugins/anomaly/shape/config_allowlist_test.go` | |
| `TestOnDetectedSkipsSingleAllowlistedPrefix` | Done | `internal/plugins/anomaly/shape/config_allowlist_test.go` | |
| `TestParseFullPoolConfigReadsNamedPoolMap` | Done | `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | |
| `TestHandleIPRequestAcceptsFramedPoolFromKeyedList` | Done | `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | |
| `TestParseFullPoolConfigReadsNamedIPv6PoolMap` | Done | `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | |
| `TestParseRouterInformationSingleScope` | Done | `internal/plugins/ospf/router_information_scope_test.go` | |
| `TestParseFlowtableSingleDevice` | Done | `internal/component/firewall/flowtable_device_test.go` | |
| `radius-framed-ip` | Done | `test/l2tp/radius-framed-ip.ci` | `make ze-functional-l2tp-test`: 23/23 PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in "Files to Modify" and "Files to Create" | Done | All 39 paths of `0cc2cf949` are tracked, verified with `git ls-files --error-unmatch` over `git show --name-only` |
| Three test files not in the plan | Changed | `internal/plugins/ospf/origination_v6_ri_test.go`, `ri_functional_test.go` and `ri_show_test.go`: fixture-shape corrections the plan did not anticipate |

### Audit Summary
- **Total items:** 12 AC, 11 tests, 4 requirements
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (three extra fixture files, recorded in Files from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Remove the class: one place knows the producer's shapes | Producer-contract test, plus a spelling count | `TestToMapLeafListMemberCountShapes` drives a real `*Tree` and asserts `absent / string / []string / map[string]any`. A grep for `func parseStringList`, `func configLeafList`, `func cfgStrings` and `func anySliceToStrings` under `internal/` returns nothing, so exactly one coercion survives |
| The anomaly allowlist reaches the arming guard | Security negative test, driven from the entry point | `make ze-unit-pkg-test PKG=./internal/plugins/anomaly/shape RUN='TestParseConfigAllowlistSingleEntry\|TestOnDetectedSkipsSingleAllowlistedPrefix\|TestOnDetectedArmsASourceOutsideTheSingleAllowlistedPrefix'` gives `ok github.com/ze-software/ze/internal/plugins/anomaly/shape 1.056s`. The pair discriminates: one asserts nothing is armed inside the prefix, the other that a source outside it IS armed, so a reader returning a match-everything allowlist fails |
| A named L2TP pool serves a subscriber | Functional test through the daemon | `make ze-functional-l2tp-test` gives `1.6s 13/23 PASS 13 radius-framed-ip`, 23/23 overall. The `.ci` asserts `named-pools=1`, a count the daemon logs only when the table is non-empty. The pre-existing `l2tp-pool: configured` line passed with the table empty |
| OSPF advertises at the one configured scope | Interop scenario, plus a two-sided unit test | `test/interop/scenarios/ospf-sr-frr` declares exactly `scope area`; under the fix FRR reported `FRR OSPF adjacency is Full` and `Ze advertised SR-Algorithm 0, SRGB and its node Prefix-SID` (recorded in the spec's Interop Tests table). `TestParseRouterInformationSingleScope` asserts the configured scope AND the absence of the second |
| A firewall flowtable holds its one device | Unit test at the reader | `make ze-unit-pkg-test PKG=./internal/component/firewall RUN=TestParseFlowtableSingleDevice` gives `ok github.com/ze-software/ze/internal/component/firewall 1.052s`. No daemon-level assertion exists: `show firewall` reads `Backend.ListTables`, an nft readback that needs Linux and root, and `test/` holds no flowtable `.ci` at all. Named as NOTE R1-2 |
| No fixture certifies a path production cannot reach | Relaxation audit over the commit | `python3 scripts/dev/audit-test-relaxation.py 0cc2cf949~1` reports zero `[DELETED]` and zero `[WEAKENED]` findings on any file this spec touched, while nine fixtures moved to the producer's shape |
| The tree still builds from what git holds | Tracked-build check | `make ze-repository-tracked-build-check` gives `tracked-build: OK (every flavor of the committed tree compiles)`, six flavors, 648 to 649 packages each |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec metadata declares `Deferral shard: -` | n-a | `ls plan/deferrals/` holds no `fixit-config-list-readers-assert-the-wrong-shape.md`, so commit A removes no shard |
| The three BGP filter entry readers that cannot load a two-entry list | deferred | Homed at `spec-fixit-ordered-list-loses-its-order`, which has since landed `internal/core/configorder` (`d687efe7e`) and the `ordered-by user` obligation in `ai/rules/config.md`. Recorded as a row in `plan/journal/green-that-could-not-have-been-red.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-config-list-readers-assert-the-wrong-shape-9ad8358c-695f-41be-8019-5d92ba08f8e6.md` |
| `review_gate.py check` | clean -- `review_gate: OK (28 code files, clean, hashes match ...)`, and `OK (1 code files, ...)` over commit A's own code file |
| Rounds | 2 |
| Reviewer lenses used | wiring and functional-test coverage; removed-behavior and test-rewrite; logic, guard audit and security; allocation and performance; documentation drift; project rules and the `ze-style` pass; simplicity and altitude; interop and goal validation |

Round 1 read the full diff of `0cc2cf949` from source and produced one ISSUE and
four NOTEs. Round 2 read the fix that ISSUE produced and found nothing, so the
final run is 0 BLOCKER, 0 ISSUE.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R1-1 | ISSUE | `configorder`'s package doc names `configvalue.ListEntries` as the reader for every unordered list, and `ListEntries` names `configorder` nowhere. A reader who lands on `ListEntries` first meets "sorted by key so that two reads of one config produce one order" with nothing to say that this order is WRONG for an `ordered-by user` list, which is the defect `spec-fixit-ordered-list-loses-its-order` exists to remove. The obligation is stated on one side of a pair (`docs/contributing/ze-style.md`) | `internal/core/configvalue/configvalue.go`, `ListEntries` doc comment | The doc comment now refuses an `ordered-by user` list, says why sorting is wrong for one, and names `configorder.Entries`. `ListEntry.Fields` also gained the aliasing contract its `configorder.Entry.Map` counterpart already carries: it is the delivered map, and a caller MUST NOT write to it. Verified at both call sites that neither `parseIPv4Pool` nor `parseIPv6PDPool` writes to the map it is handed |

### Findings recorded, not blocking
| # | Severity | Finding | Disposition |
|---|----------|---------|-------------|
| R1-2 | NOTE | The firewall flowtable and the anomaly allowlist have no daemon-level test, because neither value is observable from any operator surface: `handleShowAnomalyShape` (`internal/plugins/anomaly/shape/show.go`) does not report the allowlist, and `show firewall` reads an nft readback needing Linux and root. `test/` holds no flowtable `.ci` at all | Making them observable is a CLI surface change, so it is a feature and not a test. The unobservability is itself the reason the defect stayed silent, and it is what the journal row records. The path's daemon-level proof is `test/l2tp/radius-framed-ip.ci` and the `ospf-sr-frr` interop scenario, which exercise the same producer and the same shared reader |
| R1-3 | NOTE | `LeafList` returns nil for a bare empty string where `parseStringList` returned a one-element slice holding it. For `ipv4/address` and `ipv6/address` that would turn a loud `netip.ParsePrefix` error into silence | Unreachable for five of the six migrated leaf-lists: `address` (both), `allowed-ips`, `sysctl-profile` and rdnss `server` each carry a YANG `pattern` or typedef that no empty string matches, and validation runs before `ToMap` delivers. The sixth, bridge `member`, is a bare `type string`: it previously tried to enslave an interface named `""` and now enslaves nothing. A-5 is confirmed on this evidence rather than on the "zero active members" reasoning the spec states |
| R1-4 | NOTE | Commit `0cc2cf949`'s message states "the rule point names no symbol from the unlanded ordering work". The rule point it committed names `configorder.Entries`, `internal/core/configorder`, `Tree.ToPluginMap` and `configorder.OrderKey` | A false statement in a landed commit message, not in the product. History is not rewritten. Recorded in the Mistake Log and as a journal row |
| R1-5 | NOTE | `internal/plugins/ospf/ri_test.go` still carries `riCfg`, which builds a `scope` array unconditionally for sixteen single-scope callers | The spec's Known Limitations names it and states why: the file is RFC-tagged, so correcting it needs an owner row in `test/rfc-changed.md`, and it would discriminate nothing while `parseRouterInformation` substitutes both scopes for an enabled feature listing none |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/configvalue/configvalue.go` | Yes | `-rw-r--r-- 1 thomas thomas 4609 Aug 24 02:56` |
| `internal/core/configvalue/configvalue_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 4592 Aug 23 09:47` |
| `internal/component/config/tomap_shape_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 5352 Aug 23 09:49` |
| `internal/plugins/anomaly/shape/config_allowlist_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 3658 Aug 23 08:21` |
| `internal/component/l2tp/plugins/pool/named_pool_shape_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 5282 Aug 23 08:28` |
| `internal/plugins/ospf/router_information_scope_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 4281 Aug 23 09:50` |
| `internal/component/firewall/flowtable_device_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 1227 Aug 23 08:33` |
| `test/l2tp/radius-framed-ip.ci` | Yes | `-rw-r--r-- 1 thomas thomas 7551 Aug 23 08:29` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | the one-prefix allowlist reaches the arming guard | `make ze-unit-pkg-test PKG=./internal/plugins/anomaly/shape RUN='TestParseConfigAllowlistSingleEntry\|TestOnDetectedSkipsSingleAllowlistedPrefix\|TestOnDetectedArmsASourceOutsideTheSingleAllowlistedPrefix'` gives `ok github.com/ze-software/ze/internal/plugins/anomaly/shape 1.056s` |
| AC-3..AC-6 | every named pool is parsed and served | `make ze-unit-pkg-test PKG=./internal/component/l2tp/plugins/pool RUN='TestParseFullPoolConfigReadsNamedPoolMap\|TestParseFullPoolConfigReadsNamedIPv6PoolMap\|TestParseFullPoolConfigRejectsAnEmptyPoolName\|TestHandleIPRequestAcceptsFramedPoolFromKeyedList'` gives `ok github.com/ze-software/ze/internal/component/l2tp/plugins/pool 1.063s` |
| AC-7 | one configured scope means one scope | `make ze-unit-pkg-test PKG=./internal/plugins/ospf RUN='TestParseRouterInformationSingleScope\|TestParseRouterInformationNoScopeKeepsTheDefault\|TestConfigInstanceIDsReadsEveryProducerShape\|TestConfigInstanceIDsRejectsOutOfRange'` gives `ok github.com/ze-software/ze/internal/plugins/ospf 1.517s` |
| AC-8 | one device means one device | `make ze-unit-pkg-test PKG=./internal/component/firewall RUN=TestParseFlowtableSingleDevice` gives `ok github.com/ze-software/ze/internal/component/firewall 1.052s` |
| AC-9, AC-10 | the shared reader accepts every producer shape | `make ze-unit-pkg-test PKG=./internal/core/configvalue RUN='TestLeafListCoercesEveryProducerShape\|TestLeafListDoesNotAliasTheCallersSlice\|TestListEntriesReadsKeyedMap'` gives `ok github.com/ze-software/ze/internal/core/configvalue 1.031s` |
| AC-11 | the producer contract the reader is written against | `make ze-unit-pkg-test PKG=./internal/component/config RUN='TestToMapLeafListMemberCountShapes\|TestToMapKeyedListOmitsTheKeyLeafFromTheEntry\|TestToMapListIsAlwaysAKeyedMap'` gives `ok github.com/ze-software/ze/internal/component/config 1.187s` |
| AC-12 | exactly one coercion helper exists | `grep -rn "func parseStringList\|func configLeafList\|func cfgStrings\|func anySliceToStrings" internal/` returns nothing |
| all | no migrated package regressed | The eleven affected package trees run green together: `ok` for `configvalue`, `config` (95.077s), `anomaly/shape`, `l2tp/plugins/pool`, `ospf` and its 13 subpackages, `firewall`, `iface`, `l2tp/pppoe`, `isis` (76.963s) and its 10 subpackages, `ldp` and `filter_community` |
| all | lint is clean over every package this spec changed | `golangci-lint run` over the eleven packages reports `0 issues.` twice: once for `./internal/core/configvalue` alone, after the closure doc-comment fix, and once for the other ten together. `make ze-lint-changed` was not the entry point, because "changed" reads the working tree and this checkout carries five other sessions' edits; the packages were named explicitly instead |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `named-pool gold { ... }` in `l2tp { pool }` | `test/l2tp/radius-framed-ip.ci` | Yes. Read the file: it declares `named-pool gold` in the `ze-bgp` config block and asserts `expect=stderr:contains=named-pools=1`. `make ze-functional-l2tp-test` gives `1.6s 13/23 PASS 13 radius-framed-ip`, suite 23/23 |
| `allowlist <prefix>` (one member) in `anomaly { shape }` | none | Unit path only, driven from `(*responder).onDetected`. No daemon assertion is available: `handleShowAnomalyShape` does not report the allowlist. NOTE R1-2 |
| `scope area` (one member) in `ospf { router-information }` | `test/interop/scenarios/ospf-sr-frr` | Yes. Its `ze.conf` declares exactly `scope area`, and its RI assertions passed under the fix (spec Interop Tests table) |
| `device eth0` (one member) in a firewall flowtable | none | Unit path only. `show firewall` reads an nft readback needing Linux and root, and `grep -rln flowtable test/` returns nothing. NOTE R1-2 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Read `(*Tree).toMap` (`internal/component/config/tree.go`): its leaf-list arm switches on the active member count and assigns `active[0]`, a bare `string`, at exactly one. `TestToMapLeafListMemberCountShapes` asserts it |
| A-2 | confirmed | Same function: every entry of `t.lists` becomes a `map[string]any` at every count. `TestToMapListIsAlwaysAKeyedMap` asserts it |
| A-3 | confirmed | Each of the four readers takes a JSON string and unmarshals it, so the 2+ shape they meet is `[]any`. The AC tests drive them with JSON |
| A-4 | confirmed | `internal/core/configvalue` is imported by nine packages across `internal/component/*` and `internal/plugins/*`. `make ze-repository-tracked-build-check` compiles all six flavors green |
| A-5 | confirmed, on different evidence | The spec argues from "ToMap omits a leaf-list with zero active members", which is a different case from an empty MEMBER. The correct basis is the YANG type: `address` (v4 and v6), `allowed-ips`, `sysctl-profile` and rdnss `server` each carry a `pattern` or typedef no empty string matches, and validation runs before delivery. Bridge `member` is a bare `type string` and is the one case where behaviour moves, from enslaving an interface named `""` to enslaving nothing. NOTE R1-3 |
| A-6 | confirmed | `test/l2tp/radius-framed-ip.ci` passes with the `named-pools=1` line added, and the spec records that the suite goes red on that line alone under the reverted coercion |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 2, config syntax | `docs/architecture/config/syntax.md`'s shape table matches `(*Tree).toMap`, read at source, and the section carries source anchors on `(*Tree).ToMap`, `(*Tree).ToPluginMap`, `LeafList, ListEntries` and `Entries, OrderKey` | Yes |
| Row 12, internal architecture | `ai/PACKAGE-MAP.md` carries the `internal/core/configvalue` row; `ai/rules/config.md` and `ai/rules/points/config/config-list-shapes/` name `LeafList` and `ListEntries` as the obligation; `ai/rules/points/config/manifest.md` lists all three points | Yes |
| Row 16, source anchors on changed files | `python3 scripts/dev/spec_doc_anchors.py plan/spec-fixit-config-list-readers-assert-the-wrong-shape.md` exits 0, naming no unnamed DECLARED document. Its four MENTION-only notes are `docs/DESIGN.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/traffic/cos-plugin.md` and `docs/architecture/traffic/cp-survival-2-copp-port179.md`; none describes the coercion of a delivered config value | Yes |
| Row 17, no doc shows the broken behavior as intended | The four options this spec restores are documented with their intended meaning, which is what the fix delivers. The one config the parser refuses is `test/interop/scenarios/ospfv3-sr-frr/ze.conf`, already recorded in `plan/journal/documentation-shows-config-the-parser-refuses.md` | Yes |
| Rows 1, 3 to 11, 13 to 15: No | `make ze-repository-check` reports no stale anchor, no unwired export and no CLI-handler gap on any file this spec touched. Its nine findings are all uncommitted work in other sessions' packages | Yes |
| `make ze-doc-verify` | Not run at closure. `make ze-repository-check` covers the stale-anchor and wiring subset, and `spec_doc_anchors.py` covers the declared-document subset. The full target reads the whole working tree, which carries five other sessions' uncommitted docs | Partial, stated |

**Verification debt carried by this spec's commit.**
`plan/verification-debt/acb7c2cd.md` holds two open rows for
`fix(config): read a leaf-list and a list through one reader`: verify-status was
not FRESH-green, and no full `ze-precommit-verify` was recorded over the
commit's Go. Both stay open. A push is refused while they are open
(`ai/rules/git-safety.md`), which is where that debt is owed; the commits
themselves stand.

## Core Insight

A fixture is a second claim about the producer, and when a reader and its test
make the same wrong claim they agree with each other forever. Every one of the
five defect sites had a green test, and each one hand-built the value in a shape
`(*Tree).ToMap` cannot emit, so the green certified a path production never
takes. The habit that breaks the loop costs one test: assert the PRODUCER's
contract once, driven through the real producer, then derive every fixture from
that shape rather than from what the node type suggests.
