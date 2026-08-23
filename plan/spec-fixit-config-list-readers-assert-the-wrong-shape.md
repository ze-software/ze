# Spec: fixit -- config list readers assert the wrong shape

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-23 |

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
| Config tree → in-process component | `map[string]any` from `ToMap`, no marshalling | No |
| Config tree → out-of-process plugin | JSON text into `ParseConfig` | No |

### Integration Points
- `internal/core/configvalue` (new) - imported by both kinds of reader; depends on nothing but the standard library.
- Every migrated reader replaces a local helper call with a `configvalue` call.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

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
