# Spec: BCP 194 child 7 -- a path through a transit provider is a leak

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | spec-bcp194-0-umbrella |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-03 |

**Closure is finished and the commit is BLOCKED, 2026-09-03.** Every closure
section below is filled and the Review Gate is recorded clean over three rounds.
`./le commit create` refuses. Five RFC-tagged units were approved on 2026-09-03
and their rows are in `test/rfc-changed.md`. A SIXTH then surfaced, visible only
once the five cleared: `TestISISAuthAlgorithmEnumAcceptsAll`
(`internal/component/config/isis_auth_algorithm_enum_test.go`, RFC7950-9.6-1).
Its change is one line of setup, `reg.MergeGlobalCompleteFns()` ->
`reg.MergeGlobalCompletions()`, forced by a method rename this spec's second
global registry made necessary. No assertion moved. Only Thomas can approve it.
Status stays `in-progress` because the work is not in git, and a spec that says
`complete` in a working tree nobody committed is the `stale-spec-claims-done`
class. The five units, what changed in each, and the exact steps to finish are in
this session's state file (`./le spec session state current`).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 7 of the BCP 194 spec set. The umbrella is
`plan/spec-bcp194-0-umbrella.md`.

A peer that leaks its transit to you sends you paths that run through its
upstream. Ze's protection today is the max-prefix limit, which does stop the
leak, at the cost of the session: the peer's legitimate routes go with the
leaked ones and the session has to be reset by hand.

RFC 7454 Section 9 recommends the cheaper check, in its own words:

> This loose policy could be combined with filters for specific 2-byte or
> 4-byte AS paths that must not be accepted if advertised by the customer,
> such as upstream transit providers or peer ASNs.

An operator knows which ASNs sell transit and buy none: the Tier-1 set, plus
their own upstreams. An ASN from that set in a path learned from a peer means
the peer is giving you transit it did not sell you. The route is dropped and the
session stays up.

Section 9 states the export half as its own recommendation, and it reads off the
same list:

> Network administrators SHOULD NOT advertise prefixes with upstream AS numbers
> in the AS path to their peering AS unless they intend to provide transit for
> these prefixes.

### Goals

1. A named list of ASNs that must not appear in a path exchanged with a peer,
   declared in config, usable on import and on export. The filter type is
   `reject-asn`, because the type name states the action: this filter only
   ever rejects, unlike `as-path-list` whose entries carry their own action.
2. Each entry says WHERE in the AS_PATH the ASN is unacceptable, from a closed
   vocabulary of three positions plus one shorthand. Simpler than a regex and
   still enough for real policy (owner directive, 2026-09-02).
3. The ASNs are CONFIGURED, never shipped as behavior. Completion makes the
   well-known transit ASNs easy to type and `show` annotates what a list holds,
   so curated data informs the operator and never decides for them (owner
   directive, 2026-09-02).
4. A rejected route names the offending ASN and the position it was found at.
5. An entry accepts an ASN NAME as well as a number, once the name-to-ASN
   feature exists. This spec does not build it and does not block it.
6. Seven ways to say WHERE, settled with the owner 2026-09-03. Six are plain
   leaf-lists directly under the list, with no wrapper keyword; `nth` alone takes
   a number and is therefore the one keyed form:

   | Keyword | The listed ASN is |
   |---------|-------------------|
   | `direct` | the peer we are talking to, prepends collapsed |
   | `indirect` | anything that is NOT the peer |
   | `transit` | past the peer and not the last |
   | `origin` | the last: it announced the route |
   | `anywhere` | at any position |
   | `nth <n>` | at collapsed position n, counted from us, 1-based |
   | `regex` | matched by a pattern over the whole flattened path |

   `direct` and `indirect` are complements and `anywhere` is their union.
   `transit` and `origin` are the two halves of `indirect`. `nth` cuts ACROSS
   that partition rather than joining it, so a token holds a SET of properties
   rather than one label, and a list matches when any listed property holds.
   `direct` is RELATIONAL and `nth 1` is POSITIONAL, which is why both earn a
   place: `[3356 65001]` from AS65001 puts 3356 first without making it the peer.

The list is an unordered REJECT set: a route is rejected if any position rule or
any regex matches. There is no ordering and no first-match-wins, which is what
keeps it different from `as-path-list`, whose entries are ordered and carry their
own accept-or-reject action. Both filter types stay; neither subsumes the other.
6. The check is not optional where a relationship is declared. A peer whose
   RFC 9234 role implies the check, and whose filter chain names no such filter,
   REFUSES the config at commit. The operator states the decision either way, and
   a reader of the config file can see which was made (owner directive,
   2026-09-02).
7. A peer with NO role declared is not bound by goal 6, and Ze says so loudly
   rather than passing silently (owner decision, 2026-09-02).

The two directions are deliberately asymmetric, and the seam points the same way
the operator meaning does. On import `PeerAS` is the sender, so the leading-ASN
exemption has an input to read and a reason to exist. On export `PeerAS` is the
DESTINATION peer (A-2), so the ASN a route was learned from is not available;
goal 4 needs none, which turns the missing input into a non-issue rather than a
constraint. `RFC7454-9-8`'s own exemption, "unless they intend to provide transit
for these prefixes", is expressed by NOT attaching the list to that session, never
by a rule inside the list.

### Not goals

- Replacing max-prefix. It stays as the backstop for a leak this filter's list
  does not name.
- Replacing `bgp-role` (RFC 9234 OTC), which is the standards-based answer for a
  peer that implements it. This filter covers the peers that do not.
- Tearing down the session on a match. `FilterUpdateOutput.Teardown` exists
  (`pkg/plugin/rpc/types.go`) and this filter does not set it.

## The obligation matrix

RFC 9234 names the LOCAL speaker's position, so `customer` means the remote is
our transit provider. Read it the other way round and every cell inverts.

| Local role | Remote is | Import chain | Export chain | Why |
|------------|-----------|--------------|--------------|-----|
| `peer` | a settlement-free peer | REQUIRED | REQUIRED | They must not hand us transit, and we must not hand them any |
| `provider` | our customer | REQUIRED | not required | A customer leaking its other upstream is the case `RFC7454-9-1` addresses by name. We sell them transit, so they get everything outbound |
| `customer` | our transit provider | not required | REQUIRED | They legitimately send us the whole table. We must not offer them transit for a Tier-1, which is `RFC7454-9-8` |
| `rs` | a client of the route server we run | REQUIRED | REQUIRED | A route server relays between members, and a member leaking transit is the `peer` case on both rails |
| `rs-client` | the route server we are a client of | REQUIRED | REQUIRED | Same reason, from the other side |
| absent | undeclared | not required | not required | No relationship is declared, so nothing is implied. Ze says so loudly rather than passing silently (goal 7) |

"REQUIRED" means the config is REFUSED unless that chain names a filter whose
type declares the transit-leak obligation, active or `inactive:`. It does NOT
mean the filter is attached for the operator.

## Owner decisions

| Decision | Answer | Taken |
|----------|--------|-------|
| The concept the list expresses | ASNs that must not be reached via this peer, not Tier-1 membership as such | 2026-09-02 |
| Filter type name | `reject-asn`. The type states the ACTION, because this filter only rejects. `as-path-list` and `prefix-list` are data-shaped names because their entries carry `action accept\|reject`; `remove-private-as` is the existing verb-shaped precedent | 2026-09-02 |
| Chain reference form | The bare list name, `import [ NO-TRANSIT ]`, JunOS style. `canonicalizeOne` already accepts a bare name and resolves it through the policy-container registry, so this needs nothing built | 2026-09-02 |
| Shipped ASN set | NONE. There is no `builtin` leaf and no `except` leaf. The operator configures the ASNs, completion makes them easy to type, and `show` annotates them | 2026-09-02 |
| Where the curated data goes instead | Into completion suggestions and into a `show` annotation column. Stale suggestions are harmless; a stale reject list is not. This is what removed risks R-1, R-3 and R-10 from the design | 2026-09-02 |
| Per-entry position | Each entry says WHERE the ASN is unacceptable. Keyed BY position, with the ASNs as a leaf-list under it, so an ASN set is written once per position rather than repeated per ASN | 2026-09-02 |
| Position vocabulary | Five set-valued keys: `indirect` (transit + origin), `anywhere` (neighbor + transit + origin), `transit`, `origin`, `direct`. Keys are SETS because a YANG list key cannot be a leaf-list, and primitive-only keys would force the same ASNs to be written under two blocks | 2026-09-02 |
| The import exemption | NOT a special case any more. It is the definition of `direct`, which `indirect` excludes. The leading-run skip falls out of the vocabulary instead of being written into the code | 2026-09-02 |
| Direction | Import and export, off one list. On export the filter input carries the DESTINATION peer (A-2), so no ASN is ever `direct` and `indirect` covers the whole path. The no-exemption-on-export rule is a consequence, not a rule | 2026-09-02 |
| Regex escape hatch | `regex`, a sixth key value rather than a separate leaf. Its `asn` values are patterns matched against the whole as-path string, so no position applies to them, which is exactly why `regex` occupies the position slot instead of modifying it. Go RE2 semantics, so linear time and no backtracking, and the 512-character cap `filter_aspath` already uses. A pattern that fails to compile REFUSES the config (owner directive, 2026-09-02) | 2026-09-02 |
| The `asn` leaf-list is union-typed | `uint32` under the five position keys, `string` under `regex`. One leaf name keeps one shape at the cost of a type the schema cannot fully constrain, so the position-appropriate check is made at parse time with a message naming the key and the value | 2026-09-02 |
| How an operator gets the Tier-1 set in | `show bgp reject-asn known transit-free` prints the curated ASNs as a pasteable `asn [ ... ];` block. One command, one paste, and the config file holds NUMBERS. The curated set is still a suggestion made once at authoring time, never a lookup at config load | 2026-09-02 |
| Relationship to `as-path-list` | Neither replaces the other. `reject-asn` is an unordered reject set; `as-path-list` is an ordered accept-or-reject chain with first-match-wins. They are attached the same way and can sit in one chain | 2026-09-02 |
| ASN names | Out of scope here, and this design does not block them: the `asn` leaf-list takes a name later with no other change. Ze's existing `LookupASNName` is the wrong direction, over the network, failing silently, so it is not reusable | 2026-09-02 |
| Home of the work | BCP 194 child 7, inside the set's execution order | 2026-09-02 |
| How the check becomes mandatory | Not by auto-attaching it. The config commit REFUSES a peer whose role implies the check and whose chain names no such filter | 2026-09-02 |
| How an operator opts out | By writing the filter `inactive:` in the chain. `filterChainContains` already counts a deactivated ref as present, so the existing spelling carries the decision and no new keyword is added | 2026-09-02 |
| Which roles are bound | All four cells, not the two originally named: import from `peer`, `provider`, `rs`, `rs-client`; export to `peer`, `customer`, `rs`, `rs-client` | 2026-09-02 |
| A peer with no role | Not bound, and said loudly: one aggregated warning at config load and a `ze doctor` check that enumerates the unbound peers | 2026-09-02 |
| Where the rule runs | Beside `validatePeerProcessCaps` in the BGP peer pipeline, which startup reaches through `CreateReactorFromTree` | 2026-09-02 |
| The `ze:` built-in named set convention | Not implemented here. This plugin ships self-contained; `plan/spec-pol-0-umbrella.md`'s unbuilt `ze:` sets are surfaced separately | 2026-09-02 |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/patterns/plugin.md` and `ai/patterns/registration.md` - the shape a new
  BGP plugin takes.
  → Constraint: a config-named list filter does NOT call `filterapi.Register`.
    It declares `FilterTypes` on the registration and answers `OnFilterUpdate`,
    which is the route `filter_aspath` takes and the route this plugin takes.
  → Decision: `ai/patterns/plugin.md` is WRONG in three places that touch this
    work: its Metrics Pattern snippet does not compile against
    `ConfigureMetrics func(reg metrics.Registry)`, its checklist names a
    `TestAllPluginsRegistered` that does not exist (the real gate is
    `TestRegisteredPluginNames` against `testdata/plugins.snapshot`), and its
    Route Filters section shows only `filterapi.Register` and never mentions
    `FilterTypes`. Repaired in this work.
- [ ] `docs/architecture/config/syntax.md` - the Filter Block and the `inactive:`
  member idiom.
  → Constraint: `inactive:` is a structural flag on a chain member, parsed into
    `filterapi.FilterRef{Name, Inactive}`. A deactivated ref stays in the chain
    and never executes. This is the opt-out spelling.
  → Decision: the same page's claim that mandatory filters (`rfc:otc`) and
    default filters (`rfc:no-self-as`) exist with an `overrides` escape is WRONG
    in every clause. `no-self-as` exists nowhere under `internal/component/bgp/`,
    and `FilterRegistration.Overrides` is written by the plugin server and read by
    no engine code. The real injection mechanism is `prependDefaultFilters`.
    Repaired in this work, because this spec attaches filters to peers.
- [ ] `ai/patterns/functional-test.md` and `docs/architecture/testing/interop.md`
  - the two proof surfaces.
  → Constraint: a `.ci` for a filter decision turns the plugin's log level on with
    `option=env:...`, sends a hand-built UPDATE, and asserts BOTH an
    `expect=stderr:pattern=` for the decision taken AND a
    `reject=stderr:pattern=` for the decision NOT taken. The negative half is
    what makes it discriminating.
  → Constraint: the interop scenario directory is NAMED, never numbered, and its
    RED phase must be observed by reverting the change and REBUILDING the
    artifact the test drives, then recorded.
- [ ] `docs/plugin-development/metrics.md` - the counter naming rule.
  → Constraint: `ze_{scope}_{subject}_{event}_total`, scope being the plugin name
    with a redundant `bgp` prefix dropped, and label values bounded at compile
    time. Peer identity goes in the log line, never in a label.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7454.md` - Section 9, AS Path Filtering. Read against
  `rfc/full/rfc7454.txt` Section 9, which is the authority.
  → Constraint: `RFC7454-9-8` [SHOULD NOT] is the export half of this spec, word
    for word: "Network administrators SHOULD NOT advertise prefixes with upstream
    AS numbers in the AS path to their peering AS unless they intend to provide
    transit for these prefixes". No producer exists.
  → Constraint: the import half is the mechanism the RFC offers for `RFC7454-9-1`
    in its loose form, and the sentence carrying it took no requirement id of its
    own: "This loose policy could be combined with filters for specific 2-byte or
    4-byte AS paths that must not be accepted if advertised by the customer, such
    as upstream transit providers or peer ASNs". `RFC7454-9-1` itself is addressed
    to CUSTOMER sessions. This spec applies the same list to any session that is
    not a transit purchase, which is a superset the RFC neither states nor forbids.
    The spec claims the mechanism, never conformance to 9-1.
  → Decision: RFC 7454 is `non-normative` in `rfc/not-enrolled.txt` (umbrella
    owner decision, 2026-08-08), so no `RFC requirement:` tag and no
    `./le rfc discriminate-record` walk is owed. Code comments still quote the
    section.
  → Constraint: `RFC7454-9-4` (private AS) is child 2's P5 and `RFC7454-9-6`
    (first-AS) is child 2's P4. Neither is this spec. `RFC7454-9-7` (do not
    advertise a nonempty AS path unless providing transit) is a stronger export
    rule this spec does not implement.
- [ ] `rfc/short/rfc9234.md` - the standards-based leak defense Ze already ships
  as `bgp-role`.
  → Decision: this filter does not extend, wrap or depend on `bgp-role`. OTC
    works only where the peer implements RFC 9234; this list works everywhere and
    needs nothing from the peer. Two independent mechanisms, both attachable.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/filter_aspath/filter_aspath.go` - SDK
  filter-update seam a config-named filter list uses. `handleFilterUpdate`
  looks the list up by `in.Filter`, rejects on lookup miss, and returns
  accept or reject only.

- [ ] `internal/core/bgp/attribute/text_append.go` - `(*ASPath).AppendText` is the
  PRODUCER of the as-path field this filter reads.
  → Constraint: an empty path emits no `as-path` token, one ASN emits
    `as-path 65001` unbracketed, several emit `as-path [65001 65002]`. Any reader
    that assumes brackets loses the single-ASN case, which is the direct-peer case.
  → Constraint: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE and AS_CONFED_SET are
    flattened into one list with no marker. A listed ASN inside an AS_SET IS
    caught by a flat scan, which is what this filter wants; a filter that needed
    the segment type would have to declare `raw=true`.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `runIngressPolicyChain`
  and `runEgressPolicyChainASN4` are the two chain entry points.
  → Decision: each supplies its own direction string, so one config-named list
    attaches to the import chain and the export chain independently. The plugin
    declares no `FilterDecl`, exactly as `filter_aspath` does not.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the egress step
  loop and the later AS_PATH prepend.
  → Constraint: the export filter runs BEFORE the local AS is prepended, so the
    text carries the path as stored. Nothing in this spec depends on seeing the
    local AS, and nothing may assume it is absent for a different reason.
- [ ] `internal/component/bgp/config/filter_registry.go` - `BuildFilterRegistry`
  discovers a filter type by the `ze:filter` marker on a YANG list under
  `bgp/policy`.
  → Decision: nothing central enumerates filter types. The new YANG list plus
    `FilterTypes` on the registration is the whole wiring.
- [ ] `internal/component/bgp/plugins/rib/yang/ze-rib.yang` and
  `blackholecfg.Rule.applyDefaultCommunity` - the one shipped-default precedent.
  → Constraint: "A STATED set is taken exactly and the well-known value is NOT
    unioned into it." This spec does not contradict that precedent, because it
    declares no implicit default at all: Ze ships no ASN set, so there is nothing
    for a stated list to replace and the precedent does not bind.
- [ ] `internal/component/resolve/irr/rir_table.go` and `internal/le/ianaasn` -
  the one shipped-data convention.
  → Decision: a generated Go literal carrying `Source:` and `Generated:` in its
    header, refreshed by one deliberate `./le` verb, with no check twin. The
    Tier-1 table borrows the header shape and NOT the refresh verb: there is no
    machine-readable authority to fetch (A-6), so the header says `Curated:` and
    names its sources.
- [ ] `internal/component/bgp/config/peers.go` - `validatePeerProcessCaps` is the
  MODEL for the commit rule, `filterChainContains` is the membership test, and
  `peersAndDynamicGroups` is where the role and the resolved chains are both in
  scope.
  → Decision: `filterChainContains` deliberately counts an `inactive:` ref as
    present. That is how an operator suppresses a `prependDefaultFilters` default
    today, and it is the opt-out spelling this spec adopts rather than inventing
    an `off` keyword.
  → Constraint: `reactor.PeerSettings` carries NO role field. The role is read
    off the resolved tree, never off the settings struct.
- [ ] `internal/component/config/validate_sections.go` - the `validatedSections`
  comment states where BGP validation actually runs.
  → Decision: BGP is excluded from the generic custom-validator walk BECAUSE it
    has a deeper path, and startup reaches that path directly through
    `CreateReactorFromTree`. A rule in the peer pipeline therefore fires at daemon
    startup, at reload, in `ze doctor` (`checkBGPPeerConfig`) and in
    `ze config validate` (`runValidation`), the last two through
    `infra.ValidateBGPPeers`.
  → Constraint: the config editor's `cmdCommit` runs `ValidateTransition`, which
    does NOT reach `infra.ValidateBGPPeers`, so a refusal surfaces one step later
    as `commit failed:` rather than `commit blocked: N issue(s)`. This spec closes
    that gap, because a rule whose purpose is telling the operator what they
    omitted must say it where they typed it.
- [ ] `internal/component/bgp/plugins/role/yang/ze-role.yang` - the role surface.
  → Constraint: the leaf is `role { import <role-type>; }`, and that `import` is
    the RFC 9234 ROLE Ze sends in the OPEN, NOT a filter direction. Nothing here
    may read `role/import` as "the import chain". The enum is `provider`, `rs`,
    `rs-client`, `customer`, `peer`.
  → Constraint: RFC 9234 names the LOCAL speaker's position, so `customer` means
    the remote is our transit provider and `provider` means the remote is our
    customer. The whole direction matrix depends on reading it that way round.
  → Constraint: the role augments peer, group/peer and group. It is NOT settable
    at the `bgp` top level, unlike `filter`. A rule assuming a top-level default
    has nothing to read.

**Key insights:** (minimal context to resume after compaction)
- Direction lives on the config attachment point. `FilterDecl.Direction` is stored
  and read by nothing in the dispatch path.
- `PeerAS` means the sender on import and the destination on export. The import
  exemption has an input; the export half needs none.
- The as-path text flattens every segment type, so one flat scan is correct.
- No per-filter counter exists anywhere in the reactor. Any counting this feature
  wants, it defines in its own plugin.
- The `ze:` built-in named set convention was designed in `plan/spec-pol-0-umbrella.md`
  and never built. This spec ships self-contained and does not implement it
  (owner decision, 2026-09-02).
- `inactive:` is the existing opt-out spelling, and `filterChainContains` already
  treats a deactivated ref as present. Nothing new is built for the opt-out.
- `role/import` is a ROLE, not a direction. Confusing the two inverts the matrix.
- The commit rule lives beside `validatePeerProcessCaps` and reaches startup,
  reload, doctor and `ze config validate` from there.

**Behavior to preserve:**
- `filter_aspath`'s regex semantics and its ordered first-match-wins evaluation.
  This spec touches that plugin only to delete its private copy of the as-path
  field reader and call the shared one.
- The empty-chain accept the umbrella names: a peer with no filters configured
  keeps accepting everything.
- Fail-closed on an unknown filter name and on an IPC error, which
  `policyFilterFunc` already enforces and the new plugin repeats for a list name
  it does not hold.

**Behavior to change:**
- None outside the new plugin, except the shared as-path field reader described
  under Files to Modify.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `bgp { policy { reject-asn NAME { indirect [ N N ]; nth 2 [ N ]; } } }`,
  delivered to the plugin as the BGP subtree on the Stage 2 configure callback.
- Wire: a received UPDATE on a peer whose import chain names the list, or a
  forwarded UPDATE toward a peer whose export chain names it.

### Transformation Path
1. `OnConfigure` parses the BGP subtree, resolves each list to its effective ASN
   set: one map from ASN to the union of the primitive positions its `position`
   blocks expand to. Stored under an `atomic.Pointer` so the hot path reads
   without a lock, the same shape as `filter_aspath`'s `listsByName`.
2. The reactor renders the UPDATE to filter text and calls `CallFilterUpdate`
   with the direction, the peer address and `PeerAS`.
3. The plugin reads the as-path field through the shared reader, scans it left to
   right parsing each decimal in place, and on import skips the leading run of
   tokens equal to `PeerAS` before testing membership.
4. First listed ASN found: reject, log the ASN, the list and the direction,
   and increment the plugin's own counter. No hit: accept.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | `CallFilterUpdate`, filter-update RPC, text-format update | Yes, read at `policyFilterFunc` |
| Config to plugin | Stage 2 configure callback, BGP subtree as JSON | Yes, read at `filter_aspath`'s `OnConfigure` |
| Plugin to operator | log line per reject, Prometheus counter, `show bgp reject-asn` | No |

### Integration Points
- `registry.Register` with `FilterTypes: []string{"reject-asn"}`, which is
  what makes `import [ reject-asn:NAME ]` and a bare `NAME` resolve.
- `ze:filter` on the YANG list, which is how `BuildFilterRegistry` discovers it.
- `ConfigureMetrics` on the registration, the same seam `bgp-role` uses.
- `pluginserver.RegisterRPCs` for the `show` command, the same seam `filter_irr`
  uses.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The filter reaches the engine only through `CallFilterUpdate`, the same seam every config-named filter uses. The rule reaches the config only through the peer pipeline that already validates peers |
| No unintended coupling (components stay isolated) | Yes | `internal/component/bgp/config/` never spells `reject-asn`; it asks the registry which types declare the obligation. Verified by the grep in the Deliverables Checklist |
| No duplicated functionality (extends existing, does not recreate) | Yes | The as-path field reader is LIFTED, not copied: `filter_aspath` loses its private one in the same change. The opt-out reuses `inactive:` and `filterChainContains` rather than adding a keyword |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The scan reads the update text in place and parses decimals off string slices, which do not allocate. The effective set is a prebuilt map read through an `atomic.Pointer`. `TestScanAllocatesNothing` is the gate |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The plugin registers its filter type, its command, its metrics and its obligation. The one field added to `registry.Registration` is a DECLARATION slot every plugin can fill, not a per-feature field: it names no plugin and no filter type. Two central lists still need a hand edit and both are existing exhaustive gates, `TestFilterTypeMappings` and `feature-gates.txt` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `FilterUpdateInput.PeerAS` on an IMPORT call is the ASN of the peer that sent the UPDATE | `reactor_notify.go` passes `peerInfo.PeerAS` into `(*Reactor).runIngressPolicyChain`, which reaches `policyFilterFunc` (`reactor/filter_chain.go`) unchanged | The leading-ASN exemption exempts the wrong ASN, and a session with a listed peer rejects everything it sends | read at the producer, 2026-09-02 | CONFIRMED |
| A-2 | `FilterUpdateInput.PeerAS` on an EXPORT call is the DESTINATION peer's ASN, and the ASN the route was learned from is not in the input at all | `(*Peer).buildForwardFacts` reads `s.PeerAS` off the destination peer's own settings; that `facts.peerAS` is what `runEgressPolicyChain` is given | An export rule phrased "exempt the ASN this was learned from" has no input to read | read at the producer, 2026-09-02 | CONFIRMED |
| A-3 | On EXPORT the rendered as-path carries the path AS STORED: the local AS is prepended after the whole egress step loop closes, so the filter never sees it | the step loop and the later `wireu.ASPathIntent{Prepend: ...}` / `aspathEdit.Record` call, both in `reactor/reactor_api_forward.go` | The export check reads a path one ASN shorter than the peer will receive, which is harmless for a list of foreign ASNs | read at the producer, 2026-09-02 | CONFIRMED |
| A-4 | The as-path text flattens AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE and AS_CONFED_SET into one space-separated list, so a listed ASN inside an AS_SET IS caught by a flat scan | `(*ASPath).AppendText` (`internal/core/bgp/attribute/text_append.go`), whose doc comment states the flattening and names `FilterUpdateInput.Raw` as the only route to segment types | An AS_SET would hide a listed ASN and the filter would pass a leak | read at the producer, 2026-09-02 | CONFIRMED |
| A-5 | Direction is a property of the config ATTACHMENT POINT, not of `FilterDecl`: one list attaches to `filter { import ... }` and to `filter { export ... }` independently, and the engine passes the literal direction | `(*Reactor).runIngressPolicyChain` and `runEgressPolicyChainASN4` (`reactor/filter_ordered.go`) each supply their own direction string; `FilterDecl.Direction` is stored by `internal/component/plugin/server/startup.go` and read by nothing in the dispatch path | The plugin would have to declare its filters at Stage 1, which it cannot do for names that exist only in config | read at the producer, 2026-09-02 | CONFIRMED |
| A-8 | A single-ASN path renders unbracketed (`as-path 65001`), a multi-ASN path bracketed (`as-path [65001 65002]`), and an empty path emits no `as-path` token at all | `(*ASPath).AppendText`, same function: a zero total returns the buffer unchanged and a total of one appends the bare decimal | A parser that assumes brackets drops the single-ASN case, which is the direct-peer case this filter most needs to read correctly | read at the producer, 2026-09-02 | CONFIRMED |
| A-9 | A stated config list REPLACES a shipped default rather than unioning with it | the RFC 7999 precedent in `blackholecfg.Rule.applyDefaultCommunity` and `internal/component/bgp/plugins/rib/yang/ze-rib.yang` | A shipped default would have made "add one ASN" mean retyping the whole set | RESOLVED by design: Ze ships no ASN set at all, so there is no default to replace and the precedent does not bind | CONFIRMED |
| A-10 | `internal/component/bgp/config` may import `internal/component/plugin/registry` to ask which filter types declare the obligation | it ALREADY does, in five files including `redistribution.go` where the chain builders live and `loader_create.go` on the startup path. `internal/component/plugin/registry` imports no bgp package, and its `interfaces.go` comments say that direction was deliberately cut, so there is no cycle | The rule could not ask the registry, and the filter type would have to be spelled in a central package | read at the producer, 2026-09-02 | CONFIRMED |
| A-11 | `peersAndDynamicGroups` sees the deep-merged role, so a role set on a group binds its peers | `ResolveBGPTree` (`internal/component/bgp/config/resolve.go`) builds each peer by `deepMergeMaps` over three layers, bgp defaults then `groupFields` then the peer's own map. Only `peer` is deleted from `groupFields`, so a group's `role` container survives into the resolved peer | An inherited role would not bind, and AC-36 fails, leaving group-configured deployments unguarded | read at the producer, 2026-09-02 | CONFIRMED |
| A-12 | The editor's validation path can call `infra.ValidateBGPPeers` without a cycle or a startup-order problem | `ValidateBGPPeers` (`internal/component/config/infra/bgp.go`) takes a `*config.Tree` and has exactly two non-test callers, `ze config validate` (`cmd_validate.go`) and `ze doctor` (`checks_config.go`). `internal/component/cli/validator.go` calls `config.VerifyPluginConfigContentTransition` with the candidate tree in hand and does NOT call it, which is the gap itself | The editor keeps surfacing the refusal late as `commit failed:`, AC-39 fails, and the feature's usability degrades without its correctness changing | read at the producer, 2026-09-02 | CONFIRMED |
| A-6 | The Tier-1 set has no authoritative publisher, so any curated list is an editorial judgement | Wikipedia's Tier 1 network article lists 14 ASNs and carries no "as of" date; anuragbhatia.com (2026-08-06) names a departure history and calls AS174 repeatedly disputed | If curated data DECIDED anything, Ze would drop legitimate routes on a network that left or never belonged | RESOLVED by design: the curated table feeds completion and the `show` annotation only. `TestCuratedTableHasSourcesAndDate` asserts no config path reads it | CONFIRMED |
| A-7 | An empty or absent list must never mean "accept everything" | `ai/rules/principles.md`, the guard-shaped zero | A typo that empties a list would silently disable a safety filter with nothing red | The MECHANISM in this row was wrong and phase 1 measured it: the schema refuses NEITHER on a daemon path, because `bgp` is outside `validatedSections` and an absent `position` list has no node to walk. Phase 3 moved both refusals into the plugin's own parse (`parseOneList`, `(*rejectList).addBlock`), where `TestParseRefusesListWithNoPosition` and `TestParseRefusesEmptyASNList` hold them | CONFIRMED |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator's list keeps an ASN that no longer sells transit, and Ze drops that network's legitimate routes | Reachability complaints with no BGP-level explanation | Every reject logs the offending ASN, the position it matched at and the list that held it. `show bgp reject-asn` prints each ASN with a curated annotation, so an entry the table no longer calls transit-free is visible. Ze never edits the operator's list |
| R-2 | The filter is attached to a TRANSIT session by mistake, where every path legitimately runs through the upstream, and the entire table is rejected | The session establishes and carries almost no routes | Reject logging names the ASN and the peer. Consider a startup warning where a list is attached to a session whose configured role is `provider` (RFC 9234 role is already in the tree) |
| R-3 | AS174 is in or out depending on the source read | Disagreement between the two sources at authoring time, already observed | No longer a product risk: the curated table only suggests and annotates, so a contested entry is a suggestion the operator accepts or ignores. The annotation carries the word "contested" rather than resolving it |
| R-4 | The leading-ASN exemption is written as "anywhere in the path" by accident, and a leak that re-enters through the peer's own ASN mid-path passes | A unit test for a mid-path occurrence of the peer ASN is missing from the plan | An explicit boundary test: peer ASN leading, peer ASN prepended, peer ASN mid-path, peer ASN both leading and mid-path |
| R-5 | The export half rejects our own originations because the rendered path on export differs from what A-3 assumes | Locally originated routes stop being advertised to peers | A-3 is CONFIRMED at the producer, and AC-14 covers the no-AS_PATH case. A functional test asserts a locally originated route survives an attached export list |
| R-6 | The commit rule refuses a config an operator considers correct, and the message does not tell them what to write | A refusal whose text names only the peer | AC-21 makes the message name the peer, the role, the missing chain AND both ways to satisfy it. `TestRefusalMessageNamesPeerRoleAndChain` asserts the content, not just the refusal |
| R-7 | `role/import` is read as a filter direction rather than as the RFC 9234 role, inverting the whole matrix | The `provider` and `customer` cells behave backwards, which looks correct in a single-role test | `TestObligationMatrixEveryRoleAndDirection` is table-driven over all five roles times both chains, so an inversion cannot pass. The naming trap is stated in Required Reading |
| R-8 | The rule refuses every config in a build where the plugin's feature tag is off, because no filter type declares the obligation | Every BGP config fails to load in a minimal build | AC-34 and `TestNoDeclaringFilterTypeMeansNoEnforcement`: an obligation nothing can discharge is not enforced. This is a guard, and it is named as one |
| R-9 | Reaching `infra.ValidateBGPPeers` from the editor surfaces PRE-EXISTING peer-pipeline refusals that were previously invisible there, and existing configs start failing at commit | The editor blocks commits on configs it used to accept | This is the change working, not failing: those refusals already stopped the daemon loading. Run the `test/ui/` suite and the 12 `test/plugin/` role tests before and after, and report any config that newly blocks |
| R-10 | The curated table drifts and nobody notices | Silence | Downgraded by the design: a stale suggestion costs an operator one keystroke and a stale annotation is cosmetic. `TestCuratedTableHasSourcesAndDate` keeps the provenance from being deleted, and `show` prints the date. No refresh verb is owed |
| R-11 | `direct` is implemented as index zero rather than as the sending peer, which passes every single-role test and accepts the exact leak the filter exists to catch | A test suite that only ever peers with an ASN that is not on the list | `TestDirectIsDefinedByTheSenderNotTheIndex` drives AC-6 specifically, and `TestPositionMatrixEveryKeyEveryIndex` covers both a peer that IS the listed ASN and one that is not |
| R-12 | The five position keys drift from their expansions, so `indirect` quietly starts including `direct` | Nothing, until a leak passes | `TestPositionKeyExpansion` asserts the expansion table itself, so the vocabulary is the thing under test rather than an implementation detail of the matcher |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two different costs. A wrong SCAN silently drops legitimate routes on any session with the list attached, which looks like a reachability problem with no BGP-level explanation. A wrong OBLIGATION RULE refuses configs that are correct, which stops a daemon from loading: worse in the moment, but loud and immediate rather than silent |
| How is it reverted? | The plugin and the scan revert as one commit with no migration: a list nobody attached does nothing. The obligation rule does NOT revert cleanly once configs have been written to satisfy it, because reverting leaves filter refs naming a type the binary no longer knows, which `ValidateFilterNames` then refuses. Revert the rule and the plugin together, or not at all |
| Who else touches this path? | `plan/spec-bcp194-2-role-policy.md` owns gap P9, the role-derived policy seam, and this spec builds the first consumer of the CONFIGURED role. `plan/spec-bcp194-4-prefix.md` owns the shipped-list gap P1. `plan/spec-filter-wire-0-umbrella.md` plans to remove the text round-trip this filter reads, which would change the scan's input from text to wire bytes |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp { policy { reject-asn NO-TRANSIT { indirect [ 3356 ]; } } }` plus a peer `filter { import [ NO-TRANSIT ] }` | → | `handleFilterUpdate` in the new plugin, reached through `CallFilterUpdate` | `TestPathASNFilterReachedFromPeerImportChain` |
| the same list on a peer `filter { export [ ... ] }` | → | the same handler with `direction == "export"` | `TestPathASNFilterReachedFromPeerExportChain` |
| `reject-asn` as a registered filter type | → | `BuildFilterRegistry` discovery of the `ze:filter` marker | `TestFilterTypeMappings` (existing, extended) |
| `show bgp reject-asn` | → | the plugin's `OnExecuteCommand` handler | `TestShowPrintsEffectivePositionPerASN` |
| a `ze.conf` with a `role`-bearing peer and no declaring filter, loaded at daemon startup | → | the new rule in `peersAndDynamicGroups`, reached from `CreateReactorFromTree` | `TestStartupRefusesRoleBearingPeerWithNoLeakFilter` |
| the same config through `ze config validate` | → | the same rule via `infra.ValidateBGPPeers` | `TestZeConfigValidateRefusesRoleBearingPeerWithNoLeakFilter` |
| the same config typed into the config editor and committed | → | the same rule via the editor's validation path | `TestEditorCommitBlocksOnMissingLeakFilter` |
| a plugin declaring the transit-leak obligation at registration | → | the registry lookup the rule uses to learn which filter types discharge it | `TestRegistryReportsTransitLeakFilterTypes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Import, path `[65001 3356 65002]` from peer AS65001, list holds 3356 | Route rejected; the log line names 3356, the list name and `import` |
| AC-2 | Import, path `[65001 65002]` from peer AS65001, list holds 3356 | Route accepted |
| AC-3 | Import, path `[3356 65001]` from peer AS3356, `indirect [ 3356 ];` | Route ACCEPTED: 3356 is `direct` here, and `indirect` excludes `direct` |
| AC-4 | Import, path `[3356 3356 3356 65001]` from peer AS3356, same list | Route accepted: `direct` is the whole leading run, prepends collapsed |
| AC-5 | Import, path `[3356 174 65001]` from peer AS3356, `indirect [ 3356 174 ];` | Route rejected naming 174 at `transit`: only the leading run of the SENDING peer's ASN is `direct` |
| AC-6 | Import, path `[3356 65001]` from peer AS65001, `indirect [ 3356 ];` | Route REJECTED naming 3356 at `transit`. 3356 is at index zero and is still not `direct`, because `direct` is defined by who is sending, not by index. This is the route-server case RFC 7454 §9 names |
| AC-7 | Import, path `[3356 65001 3356]` from peer AS3356, `indirect` | Route rejected naming the trailing 3356 at `origin`: `direct` is positional within the path, not identity-wide |
| AC-8 | Import, path `[65001 3356]` from peer AS65001, `transit [ 3356 ];` | Route ACCEPTED: 3356 is the `origin` here, and this entry names `transit` only |
| AC-9 | The same path and peer with `indirect` | Route rejected naming 3356 at `origin`: `indirect` is transit plus origin |
| AC-10 | Import, path `[65001 3356]` from peer AS65001, `anywhere [ 3356 ];` | Route rejected |
| AC-11 | Import, single-ASN path `as-path 3356` from peer AS65001, `origin [ 3356 ];` | Route rejected: a lone ASN that is not the sender is the origin |
| AC-12 | Import, single-ASN path `as-path 3356` from peer AS3356, `indirect` | Route accepted: the lone ASN is the sender, so it is `direct` |
| AC-13 | Export toward any peer, path carries a listed ASN anywhere including index zero, `indirect` | Route not advertised to that peer. The input carries the destination peer, so no ASN is `direct` and `indirect` covers the whole path. The session is untouched and no NOTIFICATION is sent |
| AC-13a | Import, path `[65001 3491 65002]` from peer AS65001, `nth 2 [ 3491 ];` | Route rejected naming 3491 at `nth 2`: it is the ASN the peer connects to |
| AC-13b | Import, path `[65001 65003 3491]` from peer AS65001, the same list | Route ACCEPTED: 3491 is third, and `nth` is exact rather than at-or-beyond |
| AC-13c | Import, path `[65001 65001 65001 3491]` from peer AS65001, the same list | Route rejected: a run of identical ASNs counts ONCE, so prepending cannot move which rule fires |
| AC-13d | Import, path `[65001 3491 3491 65002]` from peer AS65001, `nth 3 [ 65002 ];` | Route rejected: the collapse rule applies to every run, not only the leading one |
| AC-13e | Import, path `[3356 65001]` from peer AS65001, `nth 1 [ 3356 ];` | Route rejected: `nth 1` is POSITIONAL where `direct` is relational, so it catches a first ASN that is not the peer |
| AC-13f | Import, the same path from peer AS3356, `nth 1 [ 3356 ];` | Route rejected: `nth` asks only where, never whose |
| AC-13g | An ASN listed under both `origin` and `nth 2`, matching at one of them | Rejected once, one counter increment. The cuts overlap and the list is still a set |
| AC-13h | `nth 0`, or an index past the AS_PATH segment cap | Config REFUSED by the schema range, not by Go |
| AC-14 | Export toward peer AS3356, path `[65001 65002]`, list holds 3356 | Route advertised: the destination's own ASN is never consulted |
| AC-15 | A list with every leaf-list empty and no `nth` entry | Config REFUSED: an empty list silently accepts everything, which is the guard-shaped zero `ai/rules/principles.md` forbids |
| AC-16 | An `nth` entry with an empty list | Config REFUSED, naming the index |
| AC-17 | The same ASN under two `position` blocks, for example `indirect` and `origin` | Config accepted; the positions union. `show` prints the effective position set per ASN so the operator sees the union rather than the two blocks |
| AC-18 | An unknown keyword inside a `reject-asn` list | Config refused by the schema as an unknown leaf, not by Go |
| AC-19 | An UPDATE with no AS_PATH attribute at all (locally originated) | Route accepted in both directions |
| AC-20 | Single-ASN path `as-path 3356`, unbracketed | The reader handles the unbracketed form; the decision is AC-11 or AC-12 by peer |
| AC-21 | A listed ASN inside an AS_SET | Route rejected: the text flattens every segment type, so the ASN is scanned like any other. Its POSITION is whatever its index in the flattened list gives it, which the entry's position set is judged against |
| AC-22 | The filter is named in a chain but the plugin holds no list of that name | Route rejected, fail-closed, with a log line naming the unknown list |
| AC-23 | A filter-update arrives before any configure delivery | Route rejected, fail-closed, with a log line saying so |
| AC-24 | `show bgp reject-asn` with two lists configured | Both lists print every ASN with its effective position set and its curated annotation, and the count of peers each is attached to per direction |
| AC-25 | An ASN in a list that the curated table does not know | It prints with an empty annotation column, never omitted and never guessed |
| AC-26 | Any reject | `ze_filter_path_asn_rejects_total` increments with bounded `direction`, `position` and `reason` labels |
| AC-27 | A peer with `role { import peer; }` and neither chain naming a declaring filter | Config REFUSED. The message names the peer, the role, both missing chains, and the two ways to satisfy it |
| AC-28 | The same peer with the filter named in the import chain only | Config REFUSED, naming the export chain alone |
| AC-29 | The same peer with the filter named in both chains | Config accepted |
| AC-30 | The same peer with `inactive: NO-TRANSIT` in both chains | Config accepted: an `inactive:` ref satisfies the obligation, because it records a decision |
| AC-31 | A peer with `role { import customer; }`, filter in the export chain only | Config accepted: the import cell is not required for that role |
| AC-32 | A peer with `role { import provider; }`, filter in the import chain only | Config accepted |
| AC-33 | A peer with `role { import provider; }` and no filter in either chain | Config REFUSED, naming the import chain alone |
| AC-34 | A peer with no `role` block at all | Config ACCEPTED, and one aggregated warning at config load names how many eBGP peers declare no role, with the first few by name |
| AC-35 | A config with peers declaring no role | `ze doctor` reports its own diagnostic code enumerating every unbound peer |
| AC-36 | A role set on a `group` and inherited by its peers | The rule reads the deep-merged value, so an inherited role binds the peer exactly as a directly set one does |
| AC-37 | The chain names a filter of a type that does NOT declare the transit-leak obligation | Config REFUSED: an unrelated filter does not discharge the obligation |
| AC-38 | The chain names a declaring filter whose list name is not defined under `bgp/policy` | Config REFUSED by the existing `ValidateFilterNames` check, before this rule reports anything |
| AC-39 | A refusal raised by this rule, seen from the config editor | The editor prints it at `commit` as a blocked-commit issue naming the peer, not as a later `commit failed:` line |
| AC-40 | The plugin is not built into this binary (its feature tag is off) | The rule finds no declaring filter type in the registry and does NOT refuse any config: an obligation nothing can discharge is not enforced |
| AC-41 | Completion invoked on an `asn` leaf-list inside a `position` block | The well-known transit ASNs are offered with their network names, and any `uint32` remains accepted. A suggestion is never a constraint |
| AC-42 | The same ASN reachable through a `position` key and also matched by a pattern | Rejected once, and the log names whichever rule was evaluated first. The list is a set, so a route rejected twice is still one reject and one counter increment |
| AC-43 | `regex [ "^3356 174 " ]; }` and a path `[3356 174 65001]` | Route rejected, and the log line names the pattern that matched rather than an ASN |
| AC-44 | The same list and a path `[3356 65001 174]` | Route accepted: the pattern asks a shape question and the shape does not hold |
| AC-45 | A list holding a `regex` block and other `position` blocks | A route matching EITHER is rejected. The keys are unioned, with no ordering between them |
| AC-46 | A pattern under `regex` that does not compile | Config REFUSED at load, naming the list and the offending pattern |
| AC-47 | A pattern longer than 512 characters | Config REFUSED, the same cap `filter_aspath` applies |
| AC-48 | A list carrying only `regex` values | Config accepted: AC-15 requires at least one rule, and a pattern is one |
| AC-49 | A pattern matched against a path carrying an AS_SET | The pattern sees the same flattened space-separated string every other reader sees, so an AS_SET member is matchable and no bracket appears in the subject |
| AC-50 | A numeric value under `regex`, or a non-numeric value under any other key | Config REFUSED at parse, naming the key and the value. The union type cannot express this, so the check is explicit and its message says which key was expected to hold what |
| AC-51 | A list holding no `regex` block, measured with `testing.AllocsPerRun` | Zero allocations. The zero-allocation guarantee covers the position path only; a list carrying patterns pays RE2's cost for them, and that is stated rather than hidden |
| AC-52 | Completion invoked on a `position` key | Six keys are offered, `regex` among them, each with help text naming what it expands to or that it takes a pattern |
| AC-53 | `show bgp reject-asn known transit-free` | Prints the curated ASNs as a single `asn [ 174 701 ... ];` line an operator pastes under a `position` block, plus the sources and curated date as comments. The output goes through the pipe operators like any other command |
| AC-54 | The same command with `| json` | The same set as structured data, so a script can build the block instead of a person |
| AC-55 | An operator pastes that block and commits | The config holds the NUMBERS. Nothing in the running config refers to the curated table, and a later change to that table cannot alter this config's behavior |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types `indirect` and completes a set of transit ASNs, attaches the list to a peer's import chain, and the peer leaks transit | completion → config → Stage 2 configure → `runIngressPolicyChain` → `CallFilterUpdate` → reject → route never cached or dispatched | `test/plugin/path-asn-filter-reject.ci` |
| 2 | Same, and the peer sends a legitimate customer route | same path, accept → route reaches the adj-RIB-in | `test/plugin/path-asn-filter-accept.ci` |
| 3 | Peers directly with a Tier-1 that is in the list and receives its routes | import chain, leading-run exemption, accept | `test/plugin/path-asn-filter-peer-is-listed.ci` |
| 4 | Attaches the list to a peer's export chain, and a route learned via a Tier-1 would otherwise be advertised | forward path → egress step loop → `runEgressPolicyChain` → suppress this destination only | `test/plugin/path-asn-filter-export-reject.ci` |
| 5 | Asks the box what the list holds | `show bgp reject-asn` → RPC forward → plugin handler → JSON, rendered through the pipe operators | `test/plugin/path-asn-show.ci` |
| 6 | Runs a real FRR peer that announces a path through a Tier-1 | wire → ze import filter → route absent from ze and never readvertised to BIRD | interop scenario `bgp-path-asn-leak-frr` |
| 7 | Declares a peer's role and forgets the filter | config → `CreateReactorFromTree` → `PeersFromConfigTree` → refusal naming the peer, the role and the missing chain | `test/plugin/path-asn-commit-refused.ci` |
| 8 | Decides deliberately not to run the check on one session | writes `inactive: NO-TRANSIT` → `filterChainContains` counts it → config loads, filter never executes | `test/plugin/path-asn-commit-inactive-accepted.ci` |
| 9 | Runs peers with no role declared | config loads, one aggregated warning, `ze doctor` enumerates them | `test/plugin/path-asn-no-role-warns.ci` |
| 10 | Types the same omission into the config editor | editor `commit` → validation path → blocked-commit issue naming the peer | `test/ui/path-asn-editor-commit-blocked.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPositionKeyExpansion` | `internal/component/bgp/plugins/filter_path_asn/config_test.go` | each of the five keys expands to the right primitive set; the table IS the vocabulary, so a changed expansion cannot pass silently | PASS |
| `TestParseRefusesListWithNoPosition` | same | AC-15 | PASS |
| `TestParseRefusesEmptyASNList` | same | AC-16 | PASS |
| `TestSameASNUnderTwoPositionsUnions` | same | AC-17 | PASS |
| `TestASNBoundaries` | same | the Boundary Tests row for asn values: 0, 23456 and 4294967295 accepted, 4294967296 refused at parse because it fits the union's string arm | PASS |
| `TestPositionMatrixEveryKeyEveryIndex` | `.../match_test.go` | AC-3 to AC-12 as one table: five keys times the positions neighbor, transit, origin, and the lone-ASN case, judged against both a peer that is the ASN and a peer that is not | PASS |
| `TestDirectIsDefinedByTheSenderNotTheIndex` | same | AC-6, the route-server case. This is the single assertion most likely to be got wrong | PASS |
| `TestPrependRunCollapsesToOneNeighbor` | same | AC-4 | PASS |
| `TestExportHasNoNeighborSoViaCoversTheWholePath` | same | AC-13, AC-14 | PASS |
| `TestScanUnbracketedSingleASN` | same | AC-20 | PASS |
| `TestScanAbsentASPath` | same | AC-19 | PASS |
| `TestScanFlattenedASSet` | same | AC-21 | PASS |
| `TestPathLengthBoundaries` | same | the Boundary Tests row for ASNs per scanned path: 0, 1, one full 255-ASN segment, and two of them | PASS |
| `TestScanAllocatesNothing` | same, `testing.AllocsPerRun` | the hot path holds the zero-allocation rule | PASS |
| `TestHandleFilterUpdateUnknownListRejects` | `.../filter_path_asn_test.go` | AC-22 | PASS |
| `TestHandleFilterUpdateBeforeConfigureRejects` | same | AC-23 | PASS |
| `TestRejectIncrementsCounter` | `.../metrics_test.go` | AC-26: every reject path counted under the direction, position and reason that decided it | PASS |
| `TestRejectLabelsAreBounded` | same | AC-26's bounded half: every series exists at 0 from startup and every label value is from the closed vocabulary | PASS |
| `TestSlotsAlignWithPositions` | same | the counter slots ARE the position constants, so a hit needs no mapping table | PASS |
| `TestShowPrintsEffectivePositionPerASN` | `.../command_test.go` | AC-24, AC-25 | PASS |
| `TestShowCountsPeersPerDirection` | same | AC-24's peer counts, over all three spellings of a chain reference | PASS |
| `TestShowByNameAnswersOneList` | same | `show bgp reject-asn name <name>`, and an unknown name answering an ERROR rather than an empty list | PASS |
| `TestCompletionOffersKnownTransitASNs` | `.../complete_test.go` | AC-41, the offering half, driven through `cli.NewCompleter().Complete` | PASS |
| `TestCompletionIsNeverAConstraint` | same | AC-41, the acceptance half: the merged validator has no `ValidateFn`, the editor's value gate accepts an unlisted ASN, and the module walk reports nothing for one | PASS |
| `TestCompletionAnnotatesNothingItDoesNotKnow` | same | AC-25's producer: an ASN outside the table gets an empty annotation, never a guess | PASS |
| `TestCompletionRegistersUnderTheNameTheSchemaCarries` | same | the `ze:validate` name in the YANG and the name the plugin registers are one string | PASS |
| `TestCompletionOffersPositionKeys` | same | AC-52, all six keys, each with the help text its `enum` declares | PASS |
| `TestKnownTransitFreePrintsPasteableBlock` | `.../command_test.go` | AC-53 and AC-55; the printed block is fed back through the config parser in the test, so a format that stops being pasteable goes red | PASS |
| `TestKnownTransitFreeJSONShape` | same | AC-54 | PASS |
| `TestRegexRejectsOnShape` | `.../match_test.go` | AC-43, AC-44 | PASS |
| `TestRegexAndPositionsUnion` | same | AC-45, AC-42 | PASS |
| `TestRegexSubjectIsTheFlattenedString` | same | AC-49 | PASS |
| `TestParseRefusesUncompilablePattern` | `.../config_test.go` | AC-46 | PASS |
| `TestParseRefusesOverlongPattern` | same | AC-47 | PASS |
| `TestParseRefusesValueOfTheWrongKindForItsKey` | same | AC-50, both directions of the mismatch | DELETED with AC-50. Dropping the `position` wrapper made the schema refuse both directions, so the parse check had no reachable failure state. `TestSchemaRefusesANameWhereAnASNBelongs` holds what survives |
| `TestRegexOnlyListIsValid` | same | AC-48 | PASS |
| `TestCuratedTableHasSourcesAndDate` | `.../curated_test.go` | the annotation table cannot lose its provenance header silently | PASS |
| `TestCuratedTableDecidesNothing` | same | the table feeds completion and `show` ONLY: `config.go`, `match.go` and `filter_path_asn.go` do not name it | PASS |
| `TestCuratedTableShape` | same | 15 entries, sorted, unique, each named; AS174 alone is contested | PASS |
| `TestCuratedContestedEntryRecordsTheDispute` | same | R-3: the disagreement over AS174 is recorded, never resolved quietly | PASS |
| `TestCuratedLookupAnswersAbsence` | same | a miss is an answer a caller can tell from a hit | PASS |
| `TestASPathFieldMatchesAppendText` | `internal/component/bgp/filtertext/aspath_test.go` | the reader is the exact inverse of `(*ASPath).AppendText`, round-tripped over generated paths: empty, single, multi, AS_SET, confederation | PASS |
| `TestObligationMatrixEveryRoleAndDirection` | `internal/component/bgp/config/peers_leakfilter_test.go` | AC-27 to AC-33 and AC-34's acceptance; table-driven over the five roles plus the absent one, times the four chain populations, so no cell is untested | PASS |
| `TestInactiveRefSatisfiesObligation` | same | AC-30 | PASS |
| `TestUnrelatedFilterDoesNotSatisfyObligation` | same | AC-37 | PASS |
| `TestInheritedGroupRoleBindsThePeer` | same | AC-36 | PASS |
| `TestPeerWithNoRoleIsAcceptedAndWarnedOnce` | same | AC-34; asserts ONE aggregated line by capturing the config logger, and that an iBGP session is not counted | PASS |
| `TestNoDeclaringFilterTypeMeansNoEnforcement` | same | AC-40, the guard that keeps a feature-gated-out build loadable | PASS |
| `TestRefusalMessageNamesPeerRoleAndChain` | same | AC-27's message content: the peer, the role, the chain, the filter type and the `inactive:` opt-out | PASS |
| `TestDoctorReportsPeersWithNoRole` | `internal/component/bgp/config/doctor_test.go` | AC-35, the engine half: the set `ze doctor` reads is the set the warning names | PASS |
| `TestCheckBGPPeersWithoutRoleReportsEveryPeer` | `internal/component/doctor/checks_config_bgp_peer_test.go` | AC-35, the report half: one warning under `doctor-bgp-peer-no-role`, naming every unbound peer | PASS |
| `TestEditorCommitBlocksOnMissingLeakFilter` | `internal/component/cli/validator_bgp_peer_test.go` | AC-39: `commit` blocks and names the peer, instead of failing one step later at reload | PASS |
| `TestReactorForwardRSDeactivatedExportFilterKeepsTheFastPath` | `internal/component/bgp/reactor/forward_rs_test.go` | a chain of only deactivated refs applies no policy, so the RS fast path keeps the peer. Found by this phase: the obligation makes every role-declaring peer name an export filter, and `inactive:` would otherwise have cost the fast path | PASS |
| `TestRegistryReportsTransitLeakFilterTypes` | `internal/component/plugin/registry/registry_test.go` | the new registration field is readable, and a plugin that declares nothing contributes nothing | PASS |
| `TestRegisterFilterObligationWithoutFilterType` | same | an obligation declared by a plugin owning no filter type is a registration error, never a silent no-op | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `asn` and `except` leaf-list values | 0-4294967295 | 4294967295 | N/A (unsigned) | 4294967296 rejected by YANG `uint32` |
| AS_TRANS in a scanned path | 23456 | 23456 | N/A | N/A |
| ASNs per scanned path | 0-255 (RFC 4271 segment cap per segment, several segments possible) | a path at the maximum UPDATE size | N/A | N/A |
| Effective set size | 0-unbounded | AC-12 covers 0 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `path-asn-filter-reject` | `test/plugin/path-asn-filter-reject.ci` | Story 1; asserts the reject log names the ASN AND `reject=stderr:pattern=` the accept line | PASS |
| `path-asn-filter-accept` | `test/plugin/path-asn-filter-accept.ci` | Story 2 | PASS |
| `path-asn-filter-peer-is-listed` | `test/plugin/path-asn-filter-peer-is-listed.ci` | Story 3 | PASS |
| `path-asn-filter-export-reject` | `test/plugin/path-asn-filter-export-reject.ci` | Story 4 | PASS |
| `path-asn-show` | `test/plugin/path-asn-show.ci` | Story 5: the three answers read back over the same dispatch the CLI uses | PASS |
| `completion-words-reject-asn-show` | `test/ui/completion-words-reject-asn-show.ci` | the YANG command tree publishes `name` and `known transit-free` to Tab | PASS |
| `path-asn-commit-refused` | `test/plugin/path-asn-commit-refused.ci` | Story 7: a role-bearing peer with no leak filter refuses at startup, and the message names the peer and the chain | PASS |
| `path-asn-commit-inactive-accepted` | `test/plugin/path-asn-commit-inactive-accepted.ci` | Story 8: the same config with `inactive:` loads | PASS |
| `path-asn-no-role-warns` | `test/plugin/path-asn-no-role-warns.ci` | Story 9: a peer with no role loads and warns once | PASS |
| `commit-blocked-missing-leak-filter` | `test/editor/lifecycle/commit-blocked-missing-leak-filter.et` | Story 10: the editor blocks at `commit` and names the peer. The file is an `.et` editor-lifecycle test, not the `.ci` this row first named: the editor commit path is driven by the `.et` runner, and a `.ci` cannot type into the editor | PASS |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-path-asn-leak-frr` | `test/interop/scenarios/bgp-path-asn-leak-frr/` | FRR announcing the leak, BIRD as the downstream observer | A path through a Tier-1 from FRR is absent from ze and never reaches BIRD, while a clean path from the same peer arrives at both. Modelled on `bgp-policy-import-export-frr`, whose `opBIRDRouteAbsent` assertion is the shape to reuse | RUN AND GREEN on 2026-09-03. RED forced twice, with the ze image rebuilt for each: `positionsByKey["indirect"]` set to `setDirect` reddens assertion 5 (`peer output unexpectedly contains "10.71.2.0/24"`), and `matchPositions` accepting every listed ASN at every position reddens assertion 3 (BIRD never receives the clean prefix). The two breaks pin the two halves |

## Files to Modify
- `internal/component/bgp/filtertext/aspath.go` (NEW file in an EXISTING package)
  - the as-path field reader. `filtertext` already exists for exactly this
  reason, and its package doc says so: "One reading of the format serves both,
  and a second copy is what lets the two answers drift." It holds
  `CommunityValues` and `HasCommunity`, read by `filter_community_match` and
  `filter_modify`. The as-path reader is the same shape and belongs beside them,
  NOT beside `(*ASPath).AppendText`, which stays the format's producer and is
  read but not edited (design correction found at audit, 2026-09-02).
- `internal/component/bgp/plugins/filter_aspath/match.go` - DELETE the private
  `extractASPathField` and `extractValueUntilNextAttr` and call the shared
  reader. Replacement, not a second path (`ai/rules/no-layering.md`).
- `internal/component/bgp/plugins/filter_aspath/match_test.go` - follow the
  deletion.
- `feature-gates.txt` - one hand-edited line gating the new plugin under `ze_bgp`.
- `internal/component/plugin/all/all_test.go` - `TestFilterTypeMappings`'s
  `expected` map is exhaustive both ways and must gain the new type.
- `docs/architecture/config/syntax.md` - the Filter Block section claims mandatory
  and default filters exist (`rfc:otc`, `rfc:no-self-as`) and that `overrides`
  replaces them. `no-self-as` exists nowhere under `internal/component/bgp/`, and
  `FilterRegistration.Overrides` is written by the plugin server and read by no
  engine code. This spec attaches filters to peers, so the page it makes a reader
  wrong about is repaired here (`ai/rules/documentation.md`).
- `docs/guide/plugins.md` - the as-path filter text is described only as "a
  space-separated decimal string", which loses the unbracketed single-ASN form and
  says nothing about segment-type flattening. Both are load-bearing for this
  plugin, so the page is corrected in the work that depends on it.
- `ai/patterns/plugin.md` - the Metrics Pattern snippet does not compile against
  `ConfigureMetrics func(reg metrics.Registry)`, the checklist names a
  `TestAllPluginsRegistered` that does not exist, and the Route Filters section
  shows only `filterapi.Register` and never mentions `FilterTypes`, which is the
  route this plugin and `filter_aspath` actually take. Repaired here because this
  spec is the next reader it would mislead.
- `internal/component/plugin/registry/registry.go` - one new declaration field on
  `Registration` naming the config obligations a plugin's filter types discharge,
  plus the accessor the rule reads. This is what keeps the filter type's NAME
  inside the plugin: the central rule asks the registry which types qualify, and
  never spells `reject-asn`. The coupling it avoids is the one behind the
  loop-detection defect recorded in `plan/journal/unwired-feature.md`.
- `internal/component/bgp/filterapi/filterapi.go` - the obligation name as a
  const, so the plugin and the rule share one declaration rather than a string
  literal each.
- `internal/component/bgp/config/peers.go` - the rule itself, beside
  `validatePeerProcessCaps` and reusing `filterChainContains`. Also the
  aggregated no-role warning.
- `internal/component/cli/validator.go` - reach `infra.ValidateBGPPeers` from the
  editor's validation path so a refusal reads as `commit blocked: N issue(s)`
  naming the peer, rather than as a later `commit failed:`. This closes a gap
  that today hides EVERY BGP peer-pipeline refusal from the editor, not only
  this rule's.
- `internal/core/diagnostic/codes.go` - the diagnostic code for the doctor check
  that enumerates peers with no declared role.
- The three interop `ze.conf` files carrying a `role` block
  (`test/interop/scenarios/bgp-role-frr/`, `bgp-role-gobgp/`,
  `bgp-role-otc-withdraw-frr/`) and the 14 `.ci` tests that configure a role:
  each gains the filter, or an `inactive:` ref where the test's subject is
  something else. This is the whole blast radius, because the rule binds only a
  peer that DECLARES a role.
- The generated files listed under Integration, regenerated by
  `./le repository generate` and by the snapshot `-update` runs.

## Files to Create
- `internal/component/bgp/plugins/filter_path_asn/register.go`
- `internal/component/bgp/plugins/filter_path_asn/filter_path_asn.go`
- `internal/component/bgp/plugins/filter_path_asn/config.go`
- `internal/component/bgp/plugins/filter_path_asn/match.go`
- `internal/component/bgp/plugins/filter_path_asn/curated.go` - the annotation
  table, a Go literal mapping ASN to network name and a transit-free note, whose
  header names both sources, the curated date, and the AS174 dispute. Read by
  completion and by `show` ONLY; no config or filter path reads it
- `internal/component/bgp/plugins/filter_path_asn/register_completion.go` - the
  `CompleteFn` and `DescribeFn` on the `asn` leaf-list. Named `register*` rather
  than `complete.go`, which the spec first said, because the `pretool-writeedit`
  gate refuses a `Register` call inside `init()` in any other file
- `internal/component/bgp/plugins/filter_path_asn/metrics.go`
- `internal/component/bgp/plugins/filter_path_asn/command.go`
- `internal/component/bgp/plugins/filter_path_asn/register_command.go` - the wire
  methods, the forwarders and the pipe shapes. Named `register*` rather than
  `cmd_path_asn.go`, which the spec first said, for the same `pretool-writeedit`
  reason `register_completion.go` carries
- `internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn.yang`
- `internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn-cmd.yang`
- `internal/component/bgp/plugins/filter_path_asn/yang/doc.go`
- the unit test files named in the TDD plan
- `test/plugin/path-asn-filter-*.ci` and a driver row in
  `internal/test/fixture/plugin_fixture_02.go`
- `test/interop/scenarios/bgp-path-asn-leak-frr/` plus its rows in
  `scenarioOperations` and `scenarioExtras`
- `docs/architecture/bgp/filter-path-asn.md` - the design page each new source
  file's `// Design:` header cites

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn.yang` augments `/bgp:bgp/bgp:policy` with `list reject-asn { ze:filter; key "name"; list position { key "where"; leaf-list asn; } }`, and `ze-filter-path-asn-cmd.yang` carries the `ze:command` node |
| YANG validation constraints | Yes, and they are NOT sufficient on their own | The constraints are all declared: `min-elements 1` on `position` and on `asn`, a `union` of `uint32` and `string(1..512)`, a six-value `enumeration` on `where`. MEASURED at phase 1, which corrected this row: only AC-18 is refused on a daemon path, because `where` is a LIST KEY and `ParseTreeWithYANG` enforces its type. `min-elements` and `length` fire only under `ValidateTreeAllModules`, and `bgp` is absent from `validatedSections` (`internal/component/config/validate_sections.go`) by the same design decision this spec's Required Reading records. AC-15 is refused by NOTHING in the schema at all: an absent list has no node to walk. So AC-15, AC-16 and AC-47 are refused by the PLUGIN's own parse, which is where the TDD plan already put them. The constraints stay because they document the contract and fire wherever the tree IS walked, and `TestPositionBoundsFireWhenTheTreeIsWalked` proves they are live |
| YANG custom validators | No | The cross-node rule this spec needs cannot be a `ze:validate`: its `ValidateFn` receives only a path string and one value, with no siblings and no tree. The rule lives in the BGP peer pipeline instead, which is the seam that can see both the role and the chains |
| CLI commands/flags | Yes | `show bgp reject-asn`, `show bgp reject-asn name <NAME>`, and `show bgp reject-asn known transit-free`, declared in the plugin's `register_command.go` and registered via `pluginserver.RegisterRPCs`. The third is the authoring aid: it prints the curated set as a pasteable config block, which is how an operator gets the Tier-1 ASNs into a list without Ze deciding anything at load time |
| CLI grammar (keyword before value) | Yes | `name <NAME>` puts the keyword before the value, per `ai/rules/cli.md` |
| Editor autocomplete | Yes | Automatic for the `where` enumeration. A `CompleteFn` on the `asn` leaf-list offers the well-known transit ASNs with their network names, reading the curated table. This is the ONLY consumer of that table besides `show`, and it is a suggestion: any `uint32` stays valid (AC-41) |
| Functional test for new RPC/API | Yes | `test/plugin/path-asn-show.ci` drives the new RPC end to end |
| Pipe completeness | Yes | `command.RegisterShape` with `ShapeTab\|ShapeMap\|ShapeDoc` plus `RegisterColumns`, following `filter_irr`'s `registerIRRShapes` |
| Env var registration | N-A | No leaf under `environment/`; the plugin reads no env var |
| Doctor check for runtime dependencies | Yes | Not for a runtime dependency, which this plugin has none of, but for goal 7: a check enumerating eBGP peers that declare no role, with its own code in `internal/core/diagnostic/codes.go`, plus unit and functional tests |
| Prometheus counters/metrics | Yes | `ze_filter_path_asn_rejects_total` with labels `direction` (import, export), `position` (neighbor, transit, origin, regex, unspecified) and `reason` (listed-asn, unknown-list, unconfigured). 14 series, every label value compile-time bounded and every child pre-resolved. `position` was added at phase 5b because AC-26 names it and the two fail-closed rejects need a word for "no position decided this"; `unspecified` is that word, and it is the one the position enum already answers for a value that is not a place. Registered through `ConfigureMetrics` on the registration, following `role/metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | The filter reads AS_PATH and encodes nothing. No new family, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, `docs/features/plugins.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/config-reference.md`, and `docs/architecture/config/syntax.md`, whose Filter Block section is also WRONG today and is repaired here |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` for the new WireMethod, and its Filter Callbacks table, which is silent on `peer-as` meaning the DESTINATION on export and on `FilterDecl.Direction` being ignored |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/features/plugins.md`, `docs/plugin-overview.md`, `docs/DESIGN.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md` gains the list type and the obligation matrix; `docs/guide/bgp-role.md` gains the fact that a declared role now carries a config obligation |
| 7 | Wire format changed? | N-A | The filter reads AS_PATH and encodes nothing new |
| 8 | Plugin SDK/protocol changed? | No | `OnFilterUpdate` and `OnExecuteCommand` are used as they are; nothing under `pkg/plugin` changes. The new field is on the internal `registry.Registration`, not on the SDK, so `docs/architecture/api/process-protocol.md` needs no contract change, only the export-direction clarification named in row 4 |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A as conformance | RFC 7454 is `non-normative` in `rfc/not-enrolled.txt`, so no `rfc/short/` requirement rows change and `docs/features/rfc-status.md` is untouched. Section 9 is quoted above the enforcing code as a comment |
| 10 | Test infrastructure changed? | No | New `.ci` files and one fixture row use existing machinery; `docs/functional-tests.md` enumerates no individual test |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, which lists the existing filter types |
| 12 | Internal architecture changed? | Yes | New `docs/architecture/bgp/filter-path-asn.md`, cited from each new file's `// Design:` header. `docs/architecture/config/yang-config-design.md` is corrected: its "two layers" statement omits the two seams that can see sibling subtrees, one of which this rule uses |
| 13 | Route metadata keys added/changed? | N-A | The filter writes no metadata key |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md`, one Scope-to-Prefix row and one Full Inventory row per metric |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md`, `docs/plugin-overview.md`, plus the `internal/component/plugin/all/testdata/*.snapshot` goldens and the `TestFilterTypeMappings` expected map |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, re-run `./le spec citation anchors spec plan/spec-bcp194-7-transit-asn.md` before closure. Named now: `docs/architecture/core-design.md` (declared by `filter_aspath/match.go`, whose private as-path reader this spec deletes for the shared one) EDITED; `docs/architecture/wire/attributes.md` (declared by `internal/core/bgp/attribute/text_append.go`, which gains the reader beside `(*ASPath).AppendText`) EDITED; `docs/architecture/api/architecture.md` and `docs/features/ai-first.md` (declared by `internal/component/plugin/registry/registry.go`, which gains one declaration field) EDITED, since a new registration field is part of the plugin contract both pages describe; `docs/architecture/config/yang-config-design.md` (declared by the config validation files) EDITED, because its "two layers" statement omits both seams that can see sibling subtrees, which is the statement this spec's rule depends on being wrong |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` and `docs/guide/bgp-policy.md` carry filter-chain examples that gain the new list type. `docs/guide/bgp-role.md`, `docs/guide/bgp-peering.md`, `docs/guide/operations.md` and `docs/guide/route-reflection.md` each show a peer with a `role` block and no leak filter, so every one of those examples now REFUSES to load and must be corrected. Verify each against the parser rather than by eye |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the plugin exists, registers, and is
   reachable from a peer chain while deciding nothing.
   - Tests: `TestPathASNFilterReachedFromPeerImportChain`,
     `TestPathASNFilterReachedFromPeerExportChain`, `TestFilterTypeMappings`
   - Files: `register.go`, `filter_path_asn.go` with a stub
     `handleFilterUpdate` that always accepts, the YANG list, `feature-gates.txt`,
     the generated glue from `./le repository generate`, the snapshot `-update`
   - Verify: the wiring tests reach the handler. They FAIL because the stub
     accepts a path it must reject
2. **Phase: The scan** -- lift the as-path field reader, then decide.
   - Tests: `TestASPathFieldSharedReaderMatchesAppendText` first, then the
     `TestScan*` set and `TestHandleFilterUpdate*`
   - Files: `internal/core/bgp/attribute/text_append.go` gains the reader;
     `filter_aspath/match.go` LOSES its private copy in the same change, never
     alongside it (`ai/rules/no-layering.md`); the new `match.go`
   - Verify: `TestScanAllocatesNothing` holds the hot path at zero allocations,
     and the wiring tests from phase 1 now pass
3. **Phase: Positions and config** -- the vocabulary, then the parse.
   - Tests: `TestPositionKeyExpansion` FIRST, because it fixes the vocabulary the
     matcher is then written against; then `TestParseRefusesListWithNoPosition`,
     `TestParseRefusesEmptyASNList`, `TestSameASNUnderTwoPositionsUnions`,
     `TestPositionMatrixEveryKeyEveryIndex`,
     `TestDirectIsDefinedByTheSenderNotTheIndex`
   - Files: the YANG list and its `min-elements` constraints, `config.go`, the
     position expansion table, `match.go`'s position judgement
   - Verify: AC-3 through AC-18. The functional accept and reject tests pass
3b. **Phase: End-to-end proof** -- the `.ci` tests for user stories 1 to 4, plus
   their driver rows in `internal/test/fixture/plugin_fixture_02.go`. Added
   2026-09-02: the original decomposition left these in Files to Create owned by
   no numbered phase, and phase 3's Verify line assumed them. Unit tests prove the
   algorithm; nothing proved a user reaches it (`ai/rules/principles.md`).
   - Tests: `path-asn-filter-reject`, `-accept`, `-peer-is-listed`,
     `-export-reject`, each asserting the decision taken AND rejecting the
     pattern of the decision not taken
   - Verify: `./le functional plugin`, with the RED phase observed by breaking
     `positionAt` and rebuilt, not assumed
4. **Phase: The obligation** -- registry declaration, then the rule, then the
   editor path. Resolve A-10 before writing the registry lookup and A-11 before
   writing the matrix; if A-10 breaks, STOP and report rather than spelling the
   filter type in `peers.go`.
   - Tests: `TestRegistryReportsTransitLeakFilterTypes`,
     `TestObligationMatrixEveryRoleAndDirection`, `TestInactiveRefSatisfies...`,
     `TestNoDeclaringFilterTypeMeansNoEnforcement`,
     `TestRefusalMessageNamesPeerRoleAndChain`,
     `TestEditorCommitBlocksOnMissingLeakFilter`
   - Files: `registry.go`, `filterapi.go` const, `peers.go`,
     `internal/component/cli/validator.go`
   - Verify: AC-21 through AC-34. Then update the three interop `ze.conf` files
     and the 14 role-bearing `.ci` tests, and report any config that newly blocks
5. **Phase: Visibility** -- the counter, the `show` command, the doctor check.
   - Tests: `TestRejectIncrementsCounter`, `TestShowPrintsEffectivePositionPerASN`,
     `TestDoctorReportsPeersWithNoRole`
   - Files: `metrics.go`, `command.go`, `register_command.go`, the cmd YANG,
     `internal/core/diagnostic/codes.go`
   - Verify: AC-19, AC-20, AC-28, AC-29
6. **Phase: Interop and the pages** -- the scenario, then every doc row.
   - Tests: `bgp-path-asn-leak-frr`, run through the revert-and-see-red walk
     required by `ai/rules/interop-and-goal-validation.md`
   - Files: the scenario directory, `scenarioOperations`, `scenarioExtras`, and
     every page named in the two checklists including the three that are WRONG
     today
   - Verify: the scenario goes RED with the filter reverted and the artifact
     rebuilt, and GREEN with it restored. Record the red

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The leading-run exemption is POSITIONAL: it skips a prefix of the path, never every occurrence of the peer's ASN (AC-7). The export half takes no exemption at all (AC-8) |
| Correctness | `role/import` is read as the RFC 9234 role, never as a chain direction, and `customer` means the remote is our upstream. Check every cell of the matrix against the table in this spec |
| Data flow | The filter type's name appears nowhere in `internal/component/bgp/config/` or any other central package; the rule reaches it only through the registry |
| Data flow | The lifted as-path reader has exactly one definition. `filter_aspath` holds no private copy after phase 2 |
| Naming | The metric is `ze_filter_path_asn_rejects_total`, scope derived by dropping the redundant `bgp` prefix, per `docs/plugin-development/metrics.md` |
| Guard | AC-34's "no declaring type means no enforcement" is a GUARD and is named as one in its comment, not left as an incidental empty-set behavior (`ai/rules/principles.md`) |
| Rule: `ai/rules/documentation.md` | The three pages that are WRONG today are repaired in this work, not reported |
| Rule: `ai/rules/interop-and-goal-validation.md` | The interop scenario's RED phase was observed and recorded, not assumed |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The plugin is registered and gated | `grep filter_path_asn feature-gates.txt` and `grep filter_path_asn internal/component/plugin/all/all_ze_bgp.go` |
| The filter type resolves from config | `TestFilterTypeMappings` passes with the new row |
| No private as-path reader survives | `grep -rn "extractASPathField" internal/component/bgp/plugins/` returns nothing |
| The filter type is not spelled centrally | `grep -rn "reject-asn" internal/component/bgp/config/ internal/component/plugin/` returns nothing outside tests |
| The curated table carries its provenance | `head -20 internal/component/bgp/plugins/filter_path_asn/curated.go` shows both sources and a `Curated:` date |
| The curated table decides nothing | `grep -rn "curated" internal/component/bgp/plugins/filter_path_asn/` names only `curated.go`, `register_completion.go`, `command.go` and their tests, never `config.go` or `match.go` |
| The commit rule refuses | `./le functional plugin` covering `path-asn-commit-refused` |
| The interop scenario discriminates | the recorded RED result from the revert walk |
| Every doc row is answered | `./le spec citation anchors spec plan/spec-bcp194-7-transit-asn.md`, then `./le doc-links check` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The as-path text is attacker-influenced: a peer chooses the ASNs. The scan must not index past the field, must not allocate per token, and must treat a malformed decimal as a non-match rather than a panic or a silent accept |
| Fail-closed | An unknown list name, a filter-update before configure, and an IPC error all REJECT (AC-17, AC-18). None of them may return accept |
| Resource exhaustion | The scan is linear in the path length with no allocation, so a long AS_PATH costs time proportional to what the wire already cost to parse. `TestScanAllocatesNothing` is the guard |
| Authorization failing open | The obligation rule must not be satisfiable by a filter of an unrelated type (AC-31), and must not be silently skipped when the registry lookup returns nothing for a reason other than the feature being absent (AC-34) |
| Error leakage | The refusal message names the peer, the role and the chain. It must not print the effective ASN set or any credential-bearing sibling of the peer block |

Closure verdict, each check run against the producing function:

| Check | Verdict | Evidence |
|-------|---------|----------|
| Input validation | PASS | `nextToken` (`match.go:206`) slices `asPath` and returns `ok == false` once the offset passes the last token, and its doc states next is strictly greater than off whenever ok, which is what bounds every loop it drives. `parseASN` answering `!ok` makes the token a NON-MATCH: `matchPositions` advances the collapsed index and `continue`s, so a malformed decimal neither panics nor accepts |
| Fail-closed | PASS | Three layers. In the plugin, `listsByName.Load() == nil` and an unknown list each `recordReject` and return `FilterReject`. In the engine, `PolicyFilterChain` (`reactor/filter_chain.go:521`) returns `PolicyReject, Failed: true` on an IPC error and on an invalid action. The default is fail-closed at the producer: `(*Server).FilterOnError` (`internal/component/plugin/server/server.go`) returns `OnErrorReject` for a nil process manager, a missing process, a nil registration and any filter that did not ASK for `OnErrorAccept`, and this plugin asks for nothing |
| Resource exhaustion | PASS | The scan is linear in the path with no allocation (`TestScanAllocatesNothing`, `TestNthAllocatesNothing`). `collapsed > nthIndexMax` short-circuits the `nth` lookup, so an attacker-chosen path cannot drive it past the configured bound. Patterns are RE2, which is linear, and `maxPatternLen` caps each at 512 characters at PARSE time, so the cost is bounded by config rather than by the peer |
| Authorization failing open | PASS | `chainNamesFilterType` tests the registry's TYPE, so an unrelated filter cannot discharge the obligation (`TestUnrelatedFilterDoesNotSatisfyObligation`). The one path that skips the rule is the empty-`declaring` guard, which is named as a guard in its comment and covered by `TestNoDeclaringFilterTypeMeansNoEnforcement`. It cannot fire for any reason other than the feature being absent, because `declaring` is `registry.FilterTypesDischarging(filterapi.ObligationTransitLeak)` computed at the call site and passed in |
| Error leakage | PASS | `leakFilterRefusal` formats the peer name, the role, the chain phrase and the declaring type names. It reads no other field of `pair.settings`, so no ASN set and no credential-bearing sibling can reach the message |

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

- The two directions of one operator rule needed different mechanisms, and the
  seam and the meaning agreed on which. On import the sender's ASN is available
  and an exemption is meaningful; on export the input carries the destination
  instead, and no exemption is wanted. A design that forced symmetry would have
  had to invent an input that does not exist.
- The opt-out was already built. `inactive:` plus `filterChainContains` counting
  a deactivated ref as present is a complete "I considered this and chose
  nothing" mechanism, and it was found by asking what already exists rather than
  by designing an `off` keyword. The keyword would have been a second way to say
  the same thing.
- A mandatory check and a hidden check are different features. Auto-attaching
  puts behavior in the daemon that the config file does not show; refusing at
  commit puts the same behavior in the file where a reader, a reviewer and a
  `grep` can all see it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A new plugin rather than a new entry kind in `filter_aspath` | Extend `filter_aspath` with a `contains-asn` entry; document a hand-written regex and ship no code | `filter_aspath`'s identity is an ordered regex list with first-match-wins. An ASN set has no ordering, a different exemption rule, and shipped data that has nothing to do with regexes, so folding it in gives one plugin two ideas. The hand-written regex option loses the curated set, the validation, the visibility and the positional exemption, all of which are the feature |
| Refuse at commit rather than auto-attach | Auto-attach by role with an `off` leaf to disable | Auto-attach hides behavior from the config file and then needs a second keyword to turn off. Refusing at commit keeps the file the single statement of what happens (owner directive, 2026-09-02) |
| `inactive:` as the opt-out | A new `off` or `none` keyword; a `path-asn-check` enumeration leaf | The spelling already exists, already means exactly this, and `filterChainContains` already honours it for the loop-detection default. A new keyword would be a second declaration of one idea |
| The rule lives in the BGP peer pipeline | The plugin's `InProcessConfigVerifier`; both seams with a shared predicate | The peer pipeline is reached by daemon startup through `CreateReactorFromTree`, and by `ze doctor` and `ze config validate` through `infra.ValidateBGPPeers`. The plugin verifier misses startup, which is the door a hand-edited file uses. The editor gap is closed separately and cheaply |
| The plugin declares the obligation; the registry answers who discharges it | Spell `reject-asn` in `peers.go` | A central package naming a plugin's filter type is the coupling that produced the loop-detection defect. One declaration field keeps the name inside the plugin and lets a second implementation qualify later |
| Lift the as-path field reader beside `AppendText` | Duplicate the ~30 lines into the new plugin | Two readers of one text format drift, and the format's producer is `(*ASPath).AppendText`. Reader and writer in one package are tested together against each other |
| Ship no built-in named list at all | Implement pol-0's `ze:` built-in named sets first; ship a `ze:tier1` list here | Those sets were designed and never built, and the two child specs that owned them are gone from `plan/`. Making this spec their vehicle would carry scope that was dropped without a decision. A built-in list also reintroduces a reserved namespace and a list whose contents change under the operator between releases (owner decision, 2026-09-02) |
| Curated data feeds completion and `show`, never the decision | Ship a set the filter reads; ship it opt-in | Behavior derived from a curated set goes wrong silently when the set drifts: a network that leaves the Tier-1 club keeps dropping routes with nothing in the config to point at. Suggestions that drift are harmless, and the operator's list says exactly what it says. This one decision removed three risks from the design (owner directive, 2026-09-02) |
| Keyed BY position, ASNs beneath | One entry per ASN carrying a position leaf-list | An ASN set shares one position far more often than an ASN needs its own, so keying by position writes each set once. A YANG list key cannot be a leaf-list, which is why the five keys are SETS rather than the three primitives (owner directive, 2026-09-02) |
| `direct` is defined by the SENDER, not by index zero | Positional: the first ASN in the path | A path `[3356 65001]` from AS65001 has 3356 at index zero and is still a leak. RFC 7454 §9 names the same distinction for route servers, where the first ASN is a member and the peer is the server. The positional reading would accept the exact case this filter exists to catch |
| `reject-asn` as the filter type | `transit-asn-list`; `reject-transit-asn`; `reject-asn-in-path`; `reject-path-via`; `reject-path` | A data-shaped name suits `as-path-list` because its entries carry `action accept\|reject`. This filter only rejects, so the type states the action, following `remove-private-as`. It names the ASN rather than the path because the ASN is what the operator lists and the `position` key already says where in the path it matters, so `-path` or `-via` in the type would repeat what the key carries. It also avoids asserting that the listed ASNs are transit providers, which is the operator's judgement and not Ze's (owner directive, 2026-09-02) |
| The plugin keeps the name `bgp-filter-path-asn` in `filter_path_asn/` | Rename it to match the config type | The plugin filters on ASNs found in the AS_PATH, which is what the directory says, and `filter_aspath` providing `as-path-list` is the existing precedent for a plugin whose name and filter type differ. It also keeps the metric `ze_filter_path_asn_rejects_total` from reading `reject_asn_rejects` |
| A keyed list whose only non-key child is a leaf-list renders INLINE, everywhere, not only for `nth` | Preserve whichever form the operator wrote; special-case `nth` and leave the other four in the block form | Four existing lists share `nth`'s exact shape: the three BGP community lists, `redistribute/destination/import`, and `system/authentication/tacacs-profile` (`list tacacs-profile { key "level"; leaf level; leaf-list profile; }`). Special-casing would give the config language two spellings for one construct, chosen by which leaf the reader is in. Both forms parse, so no existing config breaks and no golden pinned the block form; only re-rendered output changes. `tacacs-profile 15 [ admin ];` also reads as what the list means, where the block form says "profile" twice. The one cost is a one-time reformat an operator sees on their next commit (owner decision, 2026-09-03) |
| The bare list name in a chain | `<plugin>:NAME` or `<type>:NAME` | JunOS style, and `canonicalizeOne` already accepts it. The prefixed forms stay legal for disambiguation (owner directive, 2026-09-02) |

## Known Limitations
- Nothing detects that the curated annotation table has gone stale. There is no
  machine-readable authority to poll and no refresh verb, only a `Curated:` date
  the `show` command prints. The cost is bounded because the table only suggests
  and annotates.
- No built-in named list ships. An operator gets the Tier-1 ASNs by running
  `show bgp reject-asn known transit-free` and pasting its block, then editing
  it. If pol-0's `ze:` sets ever land, a `ze:tier1` list would be a natural
  addition and nothing here blocks it.
- Completion cannot expand one accepted candidate into several values, so the
  curated set is not reachable as a single `asn tier1<TAB>`. `CompleteFn` returns
  `[]string` and `validateCompletions` (`internal/component/cli/completer.go`)
  makes one `Completion` per string, so acceptance substitutes a single token.
  Changing that would mean changing `Completion` and the editor's acceptance path
  for every completing leaf in the product, which is a separate feature and not
  this spec's to take.
- ASN names are not accepted in place of numbers. The `asn` leaf-list takes one
  later with no other change, and Ze's existing `LookupASNName` cannot serve it:
  wrong direction, over the network, failing silently.
- The zero-allocation guarantee covers the position path only. A list carrying a
  `regex` block pays RE2's matching cost for its patterns. Linear time
  and no backtracking still hold, so the worst case is bounded, but a list of
  patterns is not free the way a list of ASNs is.
- The schema cannot say which arm of the union belongs under which key, so a
  numeric pattern or a textual ASN is caught at parse rather than by YANG. The
  message names the key and the value; the check is explicit for that reason.
- The obligation binds only a peer that DECLARES a role. An operator who omits
  the role is unguarded by construction (goal 7, owner decision). The doctor
  check names them; nothing forces them.
- The filter reads the flattened as-path text, so it cannot tell an AS_SET from
  an AS_SEQUENCE. That is correct for this feature, which wants every ASN
  whatever segment carries it, and it means the plugin can never grow a
  segment-aware rule without declaring `raw=true`.
- `plan/spec-bcp194-4-prefix.md` gap P1 still owes shipped bogon and
  special-purpose PREFIX lists. This spec ships an ASN table and establishes no
  convention that binds P1.

## RFC Documentation (Scope: protocol)

RFC 7454 is Best Current Practice and carries no MUST-level obligation on a
speaker; the umbrella records it in `rfc/not-enrolled.txt` with kind
`non-normative`. Section 9 recommendations are quoted above enforcing code as
`// RFC 7454 Section 9: "<quoted recommendation>"`.

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
- [ ] `./le verify worktree` passes
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Review Gate

| Run | BLOCKER | ISSUE | Notes |
|-----|---------|-------|-------|
| 1 | 1 | 0 | `commit confirmed` bypassed the BGP peer pipeline, so AC-39 held on one commit command and not its sibling |
| 2 | 0 | 3 | Three comments still carrying the retired `neighbor`/`via` vocabulary, one of them also wrong about how many leaves the completion registers on |
| 3 | 0 | 0 | Clean |
| 4 | 0 | 0 | Eight files entered the commit after round 3, found by building a HEAD-plus-ours tree: the completion-suggestion seam (`internal/component/config/yang/validator.go`, `validator_registry.go`, `validator_registry_test.go`, `internal/component/config/yang_schema.go`) and the four one-line call sites of the method it renames. Reviewed here. One NOTE, no BLOCKER, no ISSUE |


## Implementation Summary

### What Was Implemented
- Plugin `bgp-filter-path-asn` (`internal/component/bgp/plugins/filter_path_asn/`)
  providing config filter type `reject-asn`: an UPDATE is rejected when its
  AS_PATH carries a listed ASN at a listed position. `handleFilterUpdate`,
  `matchPositions`, `matchPattern`, `pathShape`, `positionAt`, `parseRejectASNLists`.
- Seven keywords. Six are plain leaf-lists (`direct`, `indirect`, `transit`,
  `origin`, `anywhere`, `regex`); `nth <n>` is the one keyed form because it takes
  a parameter. `direct` is RELATIONAL (matches only when the listed ASN is the
  sending peer) where `nth 1` is POSITIONAL, so both earn a place.
- `nth` counts RUNS, not tokens: a repeated ASN advances the collapsed index once,
  anywhere in the path, so a peer cannot move which rule fires by prepending.
- `filtertext.ASPath` (`internal/component/bgp/filtertext/aspath.go`) is now the
  single reader of the policy filter text format, replacing FOUR copies
  (`filter_aspath`, `filter_aspath_length`, `filter_remove_private_as`, plus the
  bracket knowledge inside `asPathTokens`).
- Config-time obligation rule `validateLeakFilterObligations`
  (`internal/component/bgp/config/peers.go`), beside `validatePeerProcessCaps`:
  a peer whose RFC 9234 role implies the check and whose chain names no
  discharging filter REFUSES the config. `inactive:` satisfies it, because
  `filterChainContains` already counts a deactivated ref as present.
- The filter type's name stays inside the plugin: `Registration.FilterObligations`
  plus `registry.FilterTypesDischarging`, with the obligation name one const at
  `filterapi.ObligationTransitLeak`. `internal/component/bgp/config` contains the
  string `reject-asn` nowhere outside tests.
- `infra.BGPPeersWithoutRole` plus doctor check `doctor-bgp-peer-no-role`, and
  `(*ConfigValidator).bgpPeerErrors` puts `infra.ValidateBGPPeers` on the editor's
  validation path, closing that gap for EVERY peer-pipeline refusal.
- Counter `ze_filter_path_asn_rejects_total`; commands `show bgp reject-asn`,
  `... name <NAME>`, `... known transit-free` (the authoring aid whose printed
  block is fed back through the config parser by its own test).
- Curated Tier-1 table (`curated.go`) feeding COMPLETION and the `show`
  annotation only. Ze ships no ASN set any config or filter path reads.
- Inline rendering for any keyed list whose only non-key child is a leaf-list,
  in both parser and writer, with a round-trip test.
- Interop scenario `bgp-path-asn-leak-frr`: GREEN, red phase observed TWICE
  against rebuilt ze images (breaking `positionsByKey["indirect"]` reddens
  assertion 5; breaking `matchPositions` reddens assertion 3), restored image
  digest identical to the first green run.

### Bugs Found/Fixed
Five PRE-EXISTING defects, none caused by this work, each in its path:
- `reactorForwardRS` (`internal/component/bgp/reactor/forward_rs.go`) skipped the
  route-server fast path for any peer with a non-empty export chain, counting a
  DEACTIVATED ref. The documented `inactive:` opt-out would have silently
  disabled it. Fixed by `hasActiveFilter`; covered by
  `TestReactorForwardRSDeactivatedExportFilterKeepsTheFastPath`.
- `adjRIBRouteObserver02` and `updateReceivedObserver02`
  (`internal/test/fixture/plugin_fixture_02.go`) discarded `Poll`'s answer and
  returned nil, so a rejecting chain and an empty store both read as success. The
  accept half of `aspath-filter-accept`, `-chain` and `-shortform` was vacuous:
  each loads the adj-RIB-in plugin and attached it to no peer. Both observers now
  return an error and the three configs gained the missing attachment.
- `NewCompleter` (`internal/component/cli/completer.go`) built its registry
  without merging the global completions, so EVERY plugin-registered `CompleteFn`
  was dead in the config editor: `mac-address`, `os-device-name`, `ospf-router-id`,
  `ospf-area-id`, `isis-net`, `isis-system-id`. Those six complete for the first
  time. Separately, `listKeyCompletions` never offered an enumeration-keyed list's
  key vocabulary (`enumKeyVocabulary`).
- `docs/features/configuration.md` anchored `internal/component/config/loader.go`
  for `registryName`, which is declared nowhere; the symbol is
  `plugin.RegistryName` in `internal/component/plugin/resolve.go` and `loader.go`
  only calls it. `./le docs-to-code index-check` was red at HEAD for every session
  in the repo; now green over 2,298 code paths.
- `opBIRDRouteAbsent` (`internal/le/interoplab/bgp/check_engine.go`) asked
  `birdc show route for <prefix>`, which exits 1 precisely when BIRD does not hold
  the route — the state the operation exists to observe. The branch returned a
  command failure before `requireAbsentWithProof` ever ran, so that assertion could
  never pass in ANY scenario, `bgp-policy-import-export-frr` included.

### Documentation Updates
Each page below was edited in the PHASE that changed the behavior, never deferred
to closure (`ai/rules/documentation.md`). Pages verified by the closure
documentation review are recorded in Pre-Commit Verification.
- New page `docs/architecture/bgp/filter-path-asn.md`, cited by the `// Design:`
  header of every new source file in the plugin.
- `docs/guide/plugins.md`, `docs/features/plugins.md`, `docs/plugin-overview.md`,
  `docs/DESIGN.md`, `docs/features.md`: the plugin inventories.
- `docs/guide/configuration.md`, `docs/config-reference.md`: the `reject-asn`
  block and its seven keywords.
- `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`: the
  three `show` commands, plus a CORRECTION to the Filter Callbacks table, which
  said `peer`/`peer-as` without stating they mean the DESTINATION on export
  (`runEgressPolicyChainASN4` passes `destAddrStr, destPeerAS`) and was silent on
  `FilterDecl.Direction` gating nothing (one writer at
  `internal/component/plugin/server/startup.go`, no reader).
- `docs/plugin-development/metrics.md`: the counter's scope and inventory rows.
- `docs/guide/bgp-role.md`: a declared role now carries a config obligation.
- `docs/architecture/testing/interop.md`: the scenario, and the birdc trap that
  made `opBIRDRouteAbsent` unable to pass.
- `docs/architecture/config/syntax.md`: the inline leaf-list production, AND a
  repair — its Filter Block claimed mandatory `rfc:otc` and default
  `rfc:no-self-as` filters with an `overrides` escape. Every clause was false:
  `no-self-as` exists nowhere under `internal/component/bgp/`, and
  `FilterRegistration.Overrides` is read by no engine code. The real mechanism is
  `prependDefaultFilters`.
- `docs/architecture/config/yang-config-design.md`: its "two layers" statement
  omitted both seams that can see sibling subtrees, one of which this rule uses.
- `ai/patterns/plugin.md`: three defects repaired — a Metrics Pattern snippet that
  does not compile against `ConfigureMetrics func(reg metrics.Registry)`, a
  checklist naming a `TestAllPluginsRegistered` that does not exist (the real gate
  is `TestRegisteredPluginNames` against `testdata/plugins.snapshot`), and a Route
  Filters section showing only `filterapi.Register` with no mention of
  `FilterTypes`, which is the route this plugin and `filter_aspath` take.
- `docs/features/configuration.md`: the broken source anchor described above.
- `docs/comparison.md`: the filter-type list.

FOUR pages the checklist marked Yes were NOT edited in their implementing phase
and were found empty of the feature by the closure documentation review. They are
written here, and the miss is a Mistake Log row rather than a silent repair:
- `docs/config-reference.md`: a new "Transit-Leak Filter (`reject-asn`)" section
  with the seven-keyword table and a source anchor on the YANG list.
- `docs/guide/bgp-policy.md`: the filter-type list, and a "Rejecting a path
  through a transit provider" section carrying the config, the obligation and the
  two `show` commands.
- `docs/guide/command-catalogue.md`: one row for the three `show bgp reject-asn`
  forms, compared against the five other daemons that table tracks.
- `docs/guide/command-reference.md` was edited in phase; the CATALOGUE was not.

TWO pages carried a `role` block example that the new obligation makes
UNLOADABLE, which row 17 predicted and the implementing phase did not act on:
- `docs/guide/bgp-peering.md` and `docs/guide/route-reflection.md` each showed
  `role { import rs }` with no filter chain. `leakFilterByRole["rs"]` binds both
  chains, so each example now refuses at load. Both gained a `policy` block, both
  chains, and a sentence naming the obligation and the `inactive:` opt-out.

### Deviations from Plan
- The config vocabulary was settled over FOUR owner revisions, and the spec
  records each: `transit-asn-list` with `builtin`/`except` leaves, then
  `reject-path-via` with `position <key> { asn [...] }`, then `reject-path`, then
  `reject-asn` with seven keywords and no wrapper. The parser production for an
  inline leaf-list was written, reverted in full, and re-applied when `nth`
  brought the same shape back; the middle revert was verified empty by `git diff`
  rather than assumed.
- The shipped Tier-1 ASN set was DROPPED as behavior. Curated data now feeds
  completion and a `show` annotation only, on the owner's directive, which removed
  risks R-1, R-3 and R-10 from the design rather than mitigating them.
- AC-50 was DELETED, not satisfied. See the Mistake Log row.
- Phase 3b was added to the spec mid-implementation for the `.ci` tests the
  original decomposition left unowned.
- The as-path reader landed in `internal/component/bgp/filtertext/` rather than
  beside `AppendText`, and retired FOUR copies rather than the one the spec named.
- The interop RED phase was blocked for several hours by a Docker filesystem
  failure that recovered without intervention. No work was reduced for it.

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The Integration Checklist claimed the YANG schema refuses AC-15, AC-16, AC-18 and AC-47 | Only AC-18 is refused on a daemon path, because `where` was a LIST KEY and `ParseTreeWithYANG` enforces its type. `min-elements` and `length` fire only under `ValidateTreeAllModules`, and `bgp` is outside `validatedSections`. AC-15 was refused by nothing at all: an absent list has no node to walk | Phase 1 MEASURED it rather than trusting the row. The spec's own Required Reading already recorded the `validatedSections` exclusion, so the row contradicted a fact the same spec held | Row corrected to say which refusals the schema really makes; the other three moved into the plugin's own parse, where the TDD plan had already put them |
| approach | The as-path reader was specced to live beside `(*ASPath).AppendText` in `internal/core/bgp/attribute/` | `internal/component/bgp/filtertext/` already existed for exactly this purpose, and its package doc states the reason the spec gave: "One reading of the format serves both, and a second copy is what lets the two answers drift" | Found at audit, from `ai/CODE-TO-DOCS.md` listing a `filtertext` package the spec had not consulted | Reader placed in `filtertext` beside its community siblings; `text_append.go` read as the format's producer and not edited |
| approach | The spec left the four `.ci` functional tests in Files to Create owned by NO numbered phase, while phase 3's Verify line assumed them | Unit tests proved the algorithm and nothing proved a user reached it, which `ai/rules/principles.md` forbids calling done | The phase 3 agent refused to expand its own scope silently and reported the gap instead | Phase 3b added to the spec and run; user stories 1 to 4 now have end-to-end proof |
| assumption | AC-50 was written as a parse-time check for "a numeric value under `regex`, or a non-numeric under any other key" | Dropping the `position` wrapper made `regex` its own string leaf-list and the five ASN leaves `uint32`, so the schema refuses both and the criterion's failure state became UNREACHABLE | Followed from the owner's vocabulary change, not from a test | AC-50 and its test DELETED rather than satisfied. A criterion whose state cannot be reached is not a passing criterion |
| approach | I reported that fixing Docker required `colima restart` and would cost another session a five-hour kernel build | The container had already exited, the filesystem recovered on its own, and no restart was needed. The 8MB write probe that had failed with an I/O error ran at 1.2 GB/s | The owner asked why the build would need killing, which was the right question and I had not checked | Cost re-measured before answering; the journal row in `plan/journal/full-disk-false-red.md` now records the recovery as its point. Lesson: re-run the one-line probe before reporting a VM as needing owner action |
| approach | I read "no filter decision line in ze's log" as evidence the filter had not run | The filter was deciding correctly throughout; the scenario simply raises no subsystem to info. Absence of a log line was absence of evidence, not evidence of absence | The interop break proved the filter was live: reverting `positionsByKey["indirect"]` changed the verdict | Diagnosis handed over with the inference labelled as unexplained rather than as a finding |
| approach | I described four unrelated lists as collateral damage of the writer change, and used the word "unrelated" repeatedly | All four have `nth`'s exact shape — a keyed list whose only non-key child is a leaf-list — so they were four instances of the construct being spelled compactly, not bystanders | The owner asked what the change would actually look like, and reading `tacacs-profile` answered it | Recommendation reversed to uniform rendering, with the tacacs case as the worked example |
| approach | The Documentation Update Checklist marked rows 2, 3, 6, 11 and 17 Yes, and each implementing phase edited the pages it was touching anyway | Four pages the checklist named were never edited (`docs/config-reference.md`, `docs/guide/bgp-policy.md`, `docs/guide/command-catalogue.md`, `docs/comparison.md`), and two more carried `role` examples that the new obligation makes unloadable (`docs/guide/bgp-peering.md`, `docs/guide/route-reflection.md`). Row 17 had NAMED those two pages | The closure documentation review grepped every page the checklist listed, rather than trusting the checklist's own Yes | All six written or repaired at closure. Lesson: a checklist row is a claim about a page, so the phase that owns the row greps the page, and a row naming a page as WRONG needs the same grep as a row naming it as MISSING |
| escalation | Nine consecutive server-side API errors killed the closure agent, and I attributed them to the agent's context growing with each resume | A fresh agent with a compact brief failed just as fast, before its first tool call, so context size was not the mechanism | Tested the hypothesis by spawning the fresh agent, which refuted it | Hypothesis retracted; retries continued on the owner's instruction, with record-keeping banked in the main thread and judgement left to a separate context |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Goal 1: a named ASN list usable on import and on export, filter type `reject-asn` | Done | `internal/component/bgp/plugins/filter_path_asn/register.go` (`FilterTypes: []string{"reject-asn"}`), `handleFilterUpdate` in `filter_path_asn.go` | Direction is the attachment point (A-5), so one list serves both chains |
| Goal 2: each entry says WHERE, from a closed vocabulary | Done | `positionsByKey` and `matchPositions` in `match.go` | The vocabulary grew from three plus a shorthand to seven, on the owner's 2026-09-03 revision; the closure is unchanged |
| Goal 3: ASNs are CONFIGURED, never shipped as behavior | Done | `curated.go`, read only by `register_completion.go` and `command.go` | `TestCuratedTableDecidesNothing` asserts no config or filter path reads the table |
| Goal 4: a rejected route names the offending ASN and the position | Done | `handleFilterUpdate` reject path in `filter_path_asn.go`; `matchPositions` returns the position it matched at | AC-1 asserts the ASN, the list name and the direction in the log line |
| Goal 5: an entry accepts an ASN NAME once that feature exists | Not built here, by design | -- | The spec states it does not build and does not block it. The `asn` leaves are `uint32`, so a name is a later type widening, not a redesign |
| Goal 6a: seven ways to say WHERE, six plain leaf-lists plus keyed `nth` | Done | `ze-filter-path-asn.yang` (`leaf-list direct/indirect/transit/origin/anywhere/regex`, `list nth`); `parseRejectASNLists` in `config.go` | `TestPositionKeyExpansion` asserts the expansion table itself |
| Goal 6b: the check is not optional where a relationship is declared | Done | `validateLeakFilterObligations`, `leakFilterByRole`, `chainNamesFilterType` in `internal/component/bgp/config/peers.go` | `TestObligationMatrixEveryRoleAndDirection` covers all five roles by both chains |
| Goal 7: a peer with NO role is not bound, and Ze says so loudly | Done | `rolelessPeers` in `peers.go`; the aggregated WARN at config load; `doctor-bgp-peer-no-role` via `infra.BGPPeersWithoutRole` | One declaration read by both the warning and the doctor check, so the two never enumerate different peers |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPositionMatrixEveryKeyEveryIndex`, `test/plugin/path-asn-filter-reject.ci` | `handleFilterUpdate` logs filter, direction, peer, asn, position, as-path |
| AC-2 | Done | `TestPositionMatrixEveryKeyEveryIndex`, `test/plugin/path-asn-filter-accept.ci` | `matchPositions` returns no hit, so the accept branch runs |
| AC-3 | Done | `TestDirectIsDefinedByTheSenderNotTheIndex` | `pathShape` sets `direct` from `sender.known`; `positionAt` returns `positionDirect` below it |
| AC-4 | Done | `TestPrependRunCollapsesToOneNeighbor` | `pathShape` counts the whole leading run, not one token |
| AC-5 | Done | `TestPositionMatrixEveryKeyEveryIndex` | `inLead` clears at the first non-sender token, so 174 falls to `positionTransit` |
| AC-6 | Done | `TestDirectIsDefinedByTheSenderNotTheIndex`, `test/plugin/path-asn-filter-reject.ci` | R-11's exact case: index zero, not the sender, so not direct |
| AC-7 | Done | `TestPositionMatrixEveryKeyEveryIndex` | `positionAt` is per-index, so the trailing 3356 is `positionOrigin` |
| AC-8 | Done | `TestPositionMatrixEveryKeyEveryIndex` | `positionsByKey["transit"]` is `setTransit` alone |
| AC-9 | Done | `TestPositionKeyExpansion`, `TestPositionMatrixEveryKeyEveryIndex` | `indirect` expands to `setTransit|setOrigin` |
| AC-10 | Done | `TestPositionKeyExpansion` | `anywhere` expands to all three bits |
| AC-11 | Done | `TestScanUnbracketedSingleASN`, `TestPositionMatrixEveryKeyEveryIndex` | `index == tokens-1` with `direct == 0` |
| AC-12 | Done | `TestPositionMatrixEveryKeyEveryIndex` | `positionAt` tests the leading run BEFORE the origin, which is the ordering this AC needs |
| AC-13 | Done | `TestExportHasNoNeighborSoViaCoversTheWholePath`, `test/plugin/path-asn-filter-export-reject.ci` | `senderOf` returns the zero `senderASN` on export; no `Teardown` is set anywhere in the plugin |
| AC-13a | Done | `TestNthCountsCollapsedPositionsFromUs` | `nth[nthKey{index: collapsed, asn: asn}]` |
| AC-13b | Done | `TestNthCountsCollapsedPositionsFromUs` | the map lookup is exact, never at-or-beyond |
| AC-13c | Done | `TestNthCollapsesARunAnywhereInThePath` | `collapsed` advances only when `asn != prevASN` |
| AC-13d | Done | `TestNthCollapsesARunAnywhereInThePath` | the run rule is unconditional on position, not special-cased for the lead |
| AC-13e | Done | `TestNthCutsAcrossThePartition` | `nth` is judged outside `positionAt`, so a non-peer first ASN still matches `nth 1` |
| AC-13f | Done | `TestNthCutsAcrossThePartition` | `nth` never reads `sender` |
| AC-13g | Done | `TestSameASNUnderTwoPositionsUnions`, `TestRejectIncrementsCounter` | `matchPositions` returns on the first hit, so one reject and one increment |
| AC-13h | Done | `TestSchemaRefusesAnNthIndexOutOfRange`, `TestNthBeyondTheIndexBoundDoesNotMatch` | the YANG range on `nth/index`; `nthIndexMax` bounds the runtime side |
| AC-14 | Done | `TestExportHasNoNeighborSoViaCoversTheWholePath` | the destination ASN is never consulted: `senderOf` discards `PeerAS` on export |
| AC-15 | Done | `TestParseRefusesListWithNoPosition` | refused by `parseOneList`, not by the schema. See the first Mistake Log row |
| AC-16 | Done | `TestParseRefusesEmptyASNList`, `TestParseRefusesNthWithNoASN` | `(*rejectList).addASNs` and `addNth` refuse, naming the key or the index |
| AC-17 | Done | `TestSameASNUnderTwoPositionsUnions`, `TestShowPrintsEffectivePositionPerASN` | `positions` is `map[uint32]positionSet`, so two keys OR into one set |
| AC-18 | Done | `TestSchemaRefusesAnUnknownPositionKey` | the only AC the YANG schema refuses on a daemon path |
| AC-19 | Done | `TestScanAbsentASPath` | `nextToken` returns `ok == false` at once, so no position can match |
| AC-20 | Done | `TestScanUnbracketedSingleASN`, `TestASPathFieldMatchesAppendText` | `filtertext.ASPath` reads both the bracketed and the bare form |
| AC-21 | Done | `TestScanFlattenedASSet` | A-4: `(*ASPath).AppendText` flattens every segment type before the filter sees it |
| AC-22 | Done | `TestHandleFilterUpdateUnknownListRejects` | fail-closed with `slotUnknownList` and a WARN naming the list |
| AC-23 | Done | `TestHandleFilterUpdateBeforeConfigureRejects` | fail-closed with `slotUnconfigured`; `listsByName.Load()` nil is the guard |
| AC-24 | Done | `TestShowPrintsEffectivePositionPerASN`, `TestShowCountsPeersPerDirection`, `test/plugin/path-asn-show.ci` | `command.go` |
| AC-25 | Done | `TestCompletionAnnotatesNothingItDoesNotKnow`, `TestCuratedLookupAnswersAbsence` | absent means an empty column, never a guess |
| AC-26 | Done | `TestRejectIncrementsCounter`, `TestRejectLabelsAreBounded`, `TestSlotsAlignWithPositions` | `metrics.go`: every label value is a compile-time constant |
| AC-27 | Done | `TestRefusalMessageNamesPeerRoleAndChain`, `test/plugin/path-asn-commit-refused.ci` | `leakFilterRefusal` names the peer, the role, the chains and both remedies |
| AC-28 | Done | `TestObligationMatrixEveryRoleAndDirection` | the table drives every role by both chains; the `peer` row with import only is one of its cases |
| AC-29 | Done | `TestObligationMatrixEveryRoleAndDirection` | both chains named, so `missingImport` and `missingExport` are both false |
| AC-30 | Done | `TestInactiveRefSatisfiesObligation`, `test/plugin/path-asn-commit-inactive-accepted.ci` | `chainNamesFilterType` counts a deactivated ref on purpose, and says so |
| AC-31 | Done | `TestObligationMatrixEveryRoleAndDirection` | `leakFilterByRole["customer"]` is `{exportChain: true}` |
| AC-32 | Done | `TestObligationMatrixEveryRoleAndDirection` | `leakFilterByRole["provider"]` is `{importChain: true}` |
| AC-33 | Done | `TestObligationMatrixEveryRoleAndDirection`, `TestRefusalMessageNamesPeerRoleAndChain` | `leakFilterRefusal` narrows the message to the one missing chain |
| AC-34 | Done | `TestPeerWithNoRoleIsAcceptedAndWarnedOnce`, `test/plugin/path-asn-no-role-warns.ci` | `rolelessPeers`; the WARN carries `peers=N first=...` (seen in the package run above) |
| AC-35 | Done | `TestDoctorReportsPeersWithNoRole` | `doctor-bgp-peer-no-role` over `infra.BGPPeersWithoutRole`, its code in `internal/core/diagnostic/codes.go` |
| AC-36 | Done | `TestInheritedGroupRoleBindsThePeer` | A-11: `peerRoles` reads the deep-merged map, and pairs by settings pointer |
| AC-37 | Done | `TestUnrelatedFilterDoesNotSatisfyObligation` | `chainNamesFilterType` tests the registry's TYPE, not the ref's presence |
| AC-38 | Done (pre-existing check) | `ValidateFilterNames` in `internal/component/bgp/config/` | the undefined-name refusal fires before this rule reports anything; not re-implemented |
| AC-39 | Done | `TestEditorCommitBlocksOnMissingLeakFilter`, `TestEditorCommitPassesWhenThePeerPipelineAccepts`, `test/editor/lifecycle/commit-blocked-missing-leak-filter.et` | `(*ConfigValidator).bgpPeerErrors` in `internal/component/cli/validator.go` |
| AC-40 | Done | `TestNoDeclaringFilterTypeMeansNoEnforcement` | the named guard at the head of `validateLeakFilterObligations`, commented as one |
| AC-41 | Done | `TestCompletionOffersKnownTransitASNs`, `TestCompletionIsNeverAConstraint` | `register_completion.go`; the leaf type stays `uint32` |
| AC-42 | Done | `TestRegexAndPositionsUnion`, `TestRejectIncrementsCounter` | `handleFilterUpdate` tests positions first and returns, so one reject and one increment |
| AC-43 | Done | `TestRegexRejectsOnShape` | `matchPattern` returns the pattern; the log line names it rather than an ASN |
| AC-44 | Done | `TestRegexRejectsOnShape` | the same test's negative half |
| AC-45 | Done | `TestRegexAndPositionsUnion` | no ordering between the keys: positions then patterns, both rejecting |
| AC-46 | Done | `TestParseRefusesUncompilablePattern` | `(*rejectList).addPatterns` compiles at parse and names the list and the pattern |
| AC-47 | Done | `TestParseRefusesOverlongPattern` | `maxPatternLen` = 512, the cap `filter_aspath` applies |
| AC-48 | Done | `TestRegexOnlyListIsValid`, `TestNthOnlyListIsValid` | a pattern counts as a rule for the AC-15 refusal |
| AC-49 | Done | `TestRegexSubjectIsTheFlattenedString` | `matchPattern` is given the same `asPath` string every other reader sees |
| AC-50 | Changed (DELETED) | `TestSchemaRefusesANameWhereAnASNBelongs` covers the surviving half | Dropping the `position` wrapper made `regex` a string leaf-list and the five ASN leaves `uint32`, so the schema refuses both directions and the criterion's failure state became UNREACHABLE. Recorded in the Mistake Log and in Deviations |
| AC-51 | Done | `TestScanAllocatesNothing`, `TestNthAllocatesNothing` | `testing.AllocsPerRun`; `nextToken` slices `asPath` and allocates nothing |
| AC-52 | Done | `TestCompletionOffersPositionKeys` | seven keys are now offered, not six: the AC's count predates the 2026-09-03 vocabulary revision |
| AC-53 | Done | `TestKnownTransitFreePrintsPasteableBlock`, `TestPasteableLeafIsInTheVocabulary` | the printed block is fed back through the config parser by its own test |
| AC-54 | Done | `TestKnownTransitFreeJSONShape` | `command.RegisterShape` with `ShapeTab|ShapeMap|ShapeDoc` |
| AC-55 | Done | `TestCuratedTableDecidesNothing` | the config holds numbers; no config or filter path reads `curated.go` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 42 unit tests of the Unit Tests table, minus one | Done | `internal/component/bgp/plugins/filter_path_asn/*_test.go`, `internal/component/bgp/filtertext/aspath_test.go`, `internal/component/bgp/config/peers_leakfilter_test.go` and `doctor_test.go`, `internal/component/doctor/checks_config_bgp_peer_test.go`, `internal/component/cli/validator_bgp_peer_test.go`, `internal/component/bgp/reactor/forward_rs_test.go`, `internal/component/plugin/registry/registry_test.go` | `go test -tags "ze_core ze_bgp"` over `filter_path_asn` and `filtertext`: both `ok`, this closure. The eleven obligation and doctor tests were re-run by name and all PASS |
| `TestParseRefusesValueOfTheWrongKindForItsKey` | Changed (DELETED) | -- | Deleted with AC-50; the surviving half is `TestSchemaRefusesANameWhereAnASNBelongs`. Recorded in Deviations and the Mistake Log |
| 24 tests not in the plan, added as the work found what it needed | Done | the same files | `TestNthCountsCollapsedPositionsFromUs`, `TestNthCollapsesARunAnywhereInThePath`, `TestNthCutsAcrossThePartition`, `TestNthBeyondTheIndexBoundDoesNotMatch`, `TestNthAllocatesNothing`, `TestNthOnlyListIsValid`, `TestParseReadsNthEntries`, `TestParseRefusesNthWithNoASN`, `TestShowPrintsNthPositions`, `TestSchemaRefusesAnNthIndexOutOfRange`, `TestSchemaRefusesAnUnknownPositionKey`, `TestSchemaRefusesANameWhereAnASNBelongs`, `TestPositionBoundsFireWhenTheTreeIsWalked`, `TestPasteableLeafIsInTheVocabulary`, `TestFilterInstanceNameStripsEveryPrefixForm`, `TestRolelessPeersFromTreeIsSilentOnAnUnreadableConfig`, `TestEditorCommitPassesWhenThePeerPipelineAccepts`, `TestEditorValidationSkipsTheEngineWithoutABGPBlock`, `TestASPath`, and the five `TestInlineListEntry*` round-trip tests in `internal/component/config/parser_list_test.go` |
| The 10 functional `.ci`/`.et` tests | Done | `test/plugin/path-asn-*.ci`, `test/ui/completion-words-reject-asn-show.ci`, `test/editor/lifecycle/commit-blocked-missing-leak-filter.et` | One row's location changed: the editor test is `.et`, not `.ci`. Recorded in the TDD table itself |
| Interop `bgp-path-asn-leak-frr` | Done | `test/interop/scenarios/bgp-path-asn-leak-frr/` | GREEN, with the RED phase forced twice against rebuilt ze images |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The 12 Files to Create in `filter_path_asn/` plus its `yang/` | Done | All present. `yang/embed.go` and `yang/register.go` were added by `./le yang glue write`, which the plan did not name |
| The unit test files named in the TDD plan | Done | Eight `_test.go` files in the plugin, plus the six outside it |
| `test/plugin/path-asn-*.ci` plus the driver rows in `internal/test/fixture/plugin_fixture_02.go` | Done | Eight `.ci` files; the fixture also gained the two observer fixes recorded in Bugs Found/Fixed |
| `test/interop/scenarios/bgp-path-asn-leak-frr/` plus its rows in `scenarioOperations` and `scenarioExtras` | Done | `bird.conf`, `frr.conf`, `ze.conf`; rows in `internal/le/interoplab/bgp/check_engine.go`, `check_extras.go`, `checkers.go`, `names.go` |
| `docs/architecture/bgp/filter-path-asn.md` | Done | Cited by the `// Design:` header of every new source file in the plugin |
| `internal/component/bgp/filtertext/aspath.go` | Done | New file in an existing package, per the design correction found at audit |
| The four reader copies retired | Done | `filter_aspath/match.go`, `filter_aspath_length/aspath_length.go`, `filter_remove_private_as/private_as.go` and the bracket knowledge in `asPathTokens` all call `filtertext.ASPath` now |
| `feature-gates.txt`, `internal/component/plugin/all/*`, the three snapshots | Done | Regenerated by `./le repository generate` and the snapshot `-update` runs |
| `internal/component/plugin/registry/registry.go`, `internal/component/bgp/filterapi/filterapi.go`, `internal/component/bgp/config/peers.go`, `internal/component/cli/validator.go`, `internal/core/diagnostic/codes.go` | Done | The obligation rule and its seams |
| The three interop `ze.conf` files and the role-bearing `.ci` tests | Done | 3 `ze.conf` plus 14 `.ci` files under `test/plugin/`, each naming the filter or an `inactive:` ref |
| `docs/architecture/config/syntax.md`, `docs/guide/plugins.md`, `ai/patterns/plugin.md` | Done | Each carried a repair as well as an addition; see Bugs Found/Fixed |

### Audit Summary
- **Total items:** 70 requirements and acceptance criteria (7 goals + 63 AC rows), plus 5 test groups and 11 file groups
- **Done:** 68 of 70. Goal 5 is out of scope by the spec's own words and needs no work
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 -- AC-50 and its test DELETED, because dropping the union type made the criterion's failure state unreachable. Recorded in Deviations and in the Mistake Log

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| 1. A named ASN list usable on import and on export | interop + functional | Interop `bgp-path-asn-leak-frr`: FRR announces a path through a Tier-1, the route is absent from ze and never reaches BIRD, while a clean path from the SAME session arrives at both. GREEN 2026-09-03; RED forced twice against rebuilt ze images. Both directions also driven end to end by `test/plugin/path-asn-filter-reject.ci` (import) and `path-asn-filter-export-reject.ci` (export), each through `runIngressPolicyChain` / `runEgressPolicyChain` and `CallFilterUpdate` |
| 2. Each entry says WHERE, from a closed vocabulary | functional + unit | `TestPositionKeyExpansion` asserts the expansion TABLE itself, so the vocabulary is the subject rather than a matcher detail (R-12). `TestPositionMatrixEveryKeyEveryIndex` drives every key at every index, against a peer that IS the listed ASN and one that is not |
| 3. The ASNs are CONFIGURED, never shipped as behavior | data correctness | `TestCuratedTableDecidesNothing`: `config.go`, `match.go` and `filter_path_asn.go` do not name `curated.go`. `TestKnownTransitFreePrintsPasteableBlock` feeds the printed block back through the config parser, so the operator's route from suggestion to config is proven, and what lands in the config is numbers (`TestCuratedTableHasSourcesAndDate`, AC-55) |
| 4. A rejected route names the offending ASN and the position | functional | `test/plugin/path-asn-filter-reject.ci` asserts the reject log names the ASN, and `reject=stderr:pattern=` the accept line, so a filter that accepted everything would go red. `handleFilterUpdate` logs filter, direction, peer, asn, position, as-path, and an `nth` reject additionally logs the collapsed index |
| 5. An entry accepts an ASN NAME once that feature exists | not built, by the spec's own words | The `asn` leaves are `uint32`. Nothing in this spec blocks a later type widening, and nothing here claims the feature |
| 6a. Seven ways to say WHERE | unit + user workflow | `TestPositionKeyExpansion` (the six partition keys), `TestNthCutsAcrossThePartition` and `TestDirectIsDefinedByTheSenderNotTheIndex` (R-11's exact case: `direct` is relational, `nth 1` positional), `TestNthCollapsesARunAnywhereInThePath` (the run-collapse rule a peer could otherwise game by prepending), `TestRegexRejectsOnShape` and `TestRegexSubjectIsTheFlattenedString` (the pattern key). `test/ui/completion-words-reject-asn-show.ci` proves the vocabulary reaches Tab |
| 6b. The check is not optional where a relationship is declared | user workflow, all three entry points | `TestObligationMatrixEveryRoleAndDirection` covers five roles by both chains, so R-7's inversion cannot pass. The three real entry points are each driven: daemon startup by `test/plugin/path-asn-commit-refused.ci`, `ze config validate` by the same file's command, and the config editor by `test/editor/lifecycle/commit-blocked-missing-leak-filter.et`. The deliberate opt-out is proven by `path-asn-commit-inactive-accepted.ci`, which is the paired opposite of the refusal file: only the two `inactive:` lines differ, so a rule that stopped honouring `inactive:` reddens one and a rule that stopped enforcing reddens the other |
| 7. A peer with NO role is not bound, and Ze says so loudly | functional + doctor | `test/plugin/path-asn-no-role-warns.ci`; the WARN is visible in this closure's own package run (`peers=2 first="inherits, restates"`). `TestDoctorReportsPeersWithNoRole` and `TestCheckBGPPeersWithoutRoleReportsEveryPeer` prove the doctor check and the warning read ONE set (`rolelessPeers`), so they can never enumerate different peers |
| Blast-radius goal: the rule must not break a build without the plugin | negative test | `TestNoDeclaringFilterTypeMeansNoEnforcement` (R-8): an obligation nothing can discharge is not enforced, and the guard is commented as a guard rather than left as an incidental empty loop |
| Performance goal: the position path allocates nothing | benchmark | `TestScanAllocatesNothing` and `TestNthAllocatesNothing`, both `testing.AllocsPerRun`. `nextToken` slices `asPath` rather than splitting it. A list carrying patterns pays RE2's cost, which AC-51 states rather than hides |

## Deferrals Resolved

<!-- Closure must leave no dangling row: deferral_unassigned_problems in
     internal/le/commit/prepare.go WARNS (it does not block) on a live row with no
     destination -- act on the warning here, because nothing else will.
     The spec's own shard is git rm'd at closure ONLY when every row in it is
     terminal; a shard still holding a live row outlives its source spec and
     deferral_shard_removal_problems blocks its removal
     (ai/rules/planning.md). Account for every row here.
     If resolving a row empties a FOREIGN shard (its last live row becomes
     terminal), that shard is now residue and this closure removes it too. -->
| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard exists | n/a | The spec header records `Deferral shard \| -`, and `ls plan/deferrals/` at closure names no file for this spec. Nothing was deferred: the audit above records one CHANGE (AC-50 deleted) and no Partial and no Skipped row |
| Goal 5, an ASN NAME in place of a number | out of scope by the spec's own Task section | Not a deferral. The Task states this spec "does not build it and does not block it", so there is no row to carry forward and no destination to name |
| Gap P10 in `plan/spec-bcp194-0-umbrella.md` | done | The umbrella's gap table now points P10 at this spec, and the child row is added. Both directions of RFC 7454 Section 9 have a producer for the first time |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     internal/le/spec/session/review.go record --spec <spec> --rounds <N> ... then check.
     --rounds is the pass count and is required; more than five needs
     --rounds-reason naming the PRODUCT defect a later round found, AND
     --owner-authorised carrying Thomas's word, because more than five passes
     is his decision (owner ruling 2026-08-17). At the cap you stop and ask him;
     you never set that flag on your own initiative. A false statement in this
     record is a NOTE, never a reason for another round (ai/rules/planning.md).
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bcp194-7-transit-asn-d781f6b1-77ce-422d-b72b-965113555cbc.md`, verdict clean |
| `./le spec session review check` | clean |
| Rounds | 4. Round 1 found the BLOCKER below, round 2 the three ISSUEs, round 3 nothing, round 4 covered the eight files a build check added to the commit |
| Reviewer lenses used | wiring and entry points; guard audit and fail-closed paths; removed-behavior audit over every deleted line; security checklist over the attacker-influenced AS_PATH; allocation and hot-path review; the `ze-go-style` pass of step 18 |

The review ran in a single independent context, over the whole diff and the
source it changes, not over the author's account of it. Round 4 exists because
the SCOPE changed, not because a finding did: a `go vet` over a HEAD-plus-ours
tree refused to compile without four files this session had misfiled as another
session's, and their four call sites came with them. A review is owed over what
the commit carries, so the eight were read before the artifact was rewritten. Every finding below
names the producing function and was read there.

### Findings noted, not fixed

| # | Severity | Finding | Location | Why it stands |
|---|----------|---------|----------|---------------|
| N-1 | NOTE | `RegisterSuggestion` overwrites silently when two packages declare the same `ze:validate` name, where `registry.Register` refuses a duplicate `FilterTypes` entry outright | `internal/component/config/yang/validator_registry.go`, `RegisterSuggestion` | `RegisterCompleteFn` beside it has had the same last-wins shape since before this spec, and so does `(*ValidatorRegistry).Register`. Making one of the three refuse and leaving two would be the inconsistency wearing a different face. The whole set wants one decision, which is a spec rather than a line in this one |

### Findings fixed
<!-- Only BLOCKER and ISSUE. NOTEs do not block: record them and proceed.
     Every fix is new code that needs a fresh pass, so re-run until clean. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | AC-39's guard covered ONE of the editor's two commit commands. `cmdCommit` validates through `ValidateTransition`, which reaches `bgpPeerErrors`; `cmdCommitConfirmed` (`commit confirmed <N>`, the safe-commit path with the rollback timer) validated through `Validate`, which does not. A config the daemon refuses was written to `.conf` and failed one step later at reload, which is the exact failure AC-39 exists to prevent. `ai/rules/principles.md`: the sibling path with the same shape is inside the change | `internal/component/cli/model_load.go`, `(*Model).cmdCommitConfirmed` | Both commit commands now take `ValidateTransition`. A commit is a transition, so it gets the transition-aware answer; `Validate` stays the debounced per-keystroke path. Proven by `TestEditorCommitConfirmedBlocksOnMissingLeakFilter` and `TestEditorCommitConfirmedForcedStillBlocksOnAnError`, whose RED phase was OBSERVED: with the line reverted both fail, and both pass restored |
| 2 | ISSUE | A stale comment from the retired vocabulary: "the neighbor position has an input on import and none on export, where `via` therefore covers the whole path". `neighbor` and `via` were the phase-1 names of `direct` and `indirect`, and neither word is in the schema | `internal/component/bgp/plugins/filter_path_asn/filter_path_asn.go`, `senderOf` | Reworded to `direct` and `indirect` (`ai/rules/stale-comments.md`) |
| 3 | ISSUE | The same stale word on the type that carries the sender: "the ASN whose leading run of an AS_PATH is the neighbor position" | `.../match.go`, `senderASN` | Reworded to `direct` |
| 4 | ISSUE | The completion file's header named "the five position leaf-lists ... neighbor, transit, origin, via and all", which is wrong twice: two of the five names are retired, and the file registers on SIX leaves. `ze-filter-path-asn.yang` carries `ze:validate "transit-asn"` at lines 43, 52, 62, 69, 76 and 98, the last being the `asn` leaf-list inside an `nth` entry | `.../register_completion.go` header | Rewritten to name the five position leaf-lists and the `nth` entry's own `asn` list |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in internal/le/commit/prepare.go checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|
| The 12 Go files of `internal/component/bgp/plugins/filter_path_asn/` | Yes | `ls -1` at closure: `command.go command_test.go complete_test.go config.go config_test.go curated.go curated_test.go filter_path_asn.go filter_path_asn_test.go match.go match_test.go metrics.go metrics_test.go register.go register_command.go register_completion.go wiring_test.go` |
| `.../yang/` | Yes | `doc.go embed.go register.go ze-filter-path-asn-cmd.yang ze-filter-path-asn.yang` |
| `internal/component/bgp/filtertext/aspath.go` | Yes | `ls -l`: 1.5K |
| `docs/architecture/bgp/filter-path-asn.md` | Yes | `ls -l`: 20K |
| The 8 `test/plugin/path-asn-*.ci` | Yes | `ls -l`: `path-asn-commit-inactive-accepted.ci` 2.7K, `path-asn-commit-refused.ci` 2.4K, `path-asn-filter-accept.ci` 3.1K, `path-asn-filter-export-reject.ci` 6.9K, `path-asn-filter-peer-is-listed.ci` 3.3K, `path-asn-filter-reject.ci` 3.1K, `path-asn-no-role-warns.ci` 2.7K, `path-asn-show.ci` 2.4K |
| `test/ui/completion-words-reject-asn-show.ci` | Yes | `ls -l`: 579 bytes |
| `test/editor/lifecycle/commit-blocked-missing-leak-filter.et` | Yes | `ls -l`: 1.8K. This is the file the TDD plan named `test/ui/path-asn-editor-commit-blocked.ci`; `ls test/ui/path-asn-editor-commit-blocked.ci` answers "No such file" |
| `test/interop/scenarios/bgp-path-asn-leak-frr/` | Yes | `bird.conf` 442B, `frr.conf` 1.6K, `ze.conf` 2.6K |
| `internal/component/bgp/config/peers_leakfilter_test.go`, `doctor_test.go`, `internal/component/cli/validator_bgp_peer_test.go`, `internal/component/config/parser_list_test.go` | Yes | listed as untracked-new by `git status --porcelain` at closure |


### AC Verified (grep/test)
<!-- Every AC-N, re-checked. Acceptable: test name + pass output, grep showing
     the call, ls showing the file. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-26, AC-41 to AC-55 | the scan, the vocabulary, the fail-closed paths, the counter, `show`, completion and the curated table | `./le job run label unit-pathasn command go test -count=1 -tags "ze_core ze_bgp" ./internal/component/bgp/plugins/filter_path_asn/... ./internal/component/bgp/filtertext/...` at closure: `ok .../filter_path_asn 1.032s`, `ok .../filtertext 0.287s` |
| AC-27 to AC-37, AC-40 | the obligation rule, its matrix, the `inactive:` opt-out, the no-role warning and the feature-gate guard | Re-run by name at closure, all PASS: `TestObligationMatrixEveryRoleAndDirection` 1.54s, `TestRefusalMessageNamesPeerRoleAndChain`, `TestInactiveRefSatisfiesObligation`, `TestUnrelatedFilterDoesNotSatisfyObligation`, `TestInheritedGroupRoleBindsThePeer`, `TestPeerWithNoRoleIsAcceptedAndWarnedOnce`, `TestNoDeclaringFilterTypeMeansNoEnforcement`, `TestRuleReadsTheObligationFromTheRegistry`, `TestDoctorReportsPeersWithNoRole`, `TestFilterInstanceNameStripsEveryPrefixForm`, `TestRolelessPeersFromTreeIsSilentOnAnUnreadableConfig` |
| AC-34's warning text | one aggregated line naming the count and the first few peers | Observed in the closure run's own stderr: `level=WARN msg="eBGP peers declare no RFC 9234 role, so no transit-leak filter is required of them" subsystem=bgp.config peers=6 first="both-programs, injector-only, transit-a, transit-b, transit-c"` |
| AC-38 | the undefined-name refusal is the EXISTING check, not re-implemented | `grep -rn "validateLeakFilterObligations" internal/ --include "*.go"` finds one non-test call site, `peers.go:299`, after `ValidateFilterNames` in the same pipeline |
| AC-39 | the editor reaches `infra.ValidateBGPPeers` | `git diff internal/component/cli/validator.go` shows `result.Errors = append(result.Errors, v.bgpPeerErrors(tree)...)` replacing a discarded tree: `result, _, _ := v.validateCore(candidate)` became `result, tree, _ := ...` |
| AC-50 | DELETED, not satisfied | `grep -rn "func TestParseRefusesValueOfTheWrongKindForItsKey" internal/` returns nothing. `config_test.go:224-238` records why, and `TestSchemaRefusesANameWhereAnASNBelongs` holds the surviving half |
| The obligation rule never names the filter type | `internal/component/bgp/config` contains `reject-asn` nowhere outside tests | `grep -rn "FilterTypesDischarging\|ObligationTransitLeak" internal/ --include "*.go"` shows the rule reading the registry at `peers.go:299-300`; the string lives in `filterapi.go:41` and `filter_path_asn/register.go:22` |
| The reader replaced four copies rather than adding a fifth | `filtertext.ASPath` is the only reader | `grep -rn "filtertext.ASPath" internal/ --include "*.go"` names four call sites (`filter_aspath`, `filter_aspath_length`, `filter_remove_private_as`, `filter_path_asn`); `grep -rn "asPathTokens" internal/ --include "*.go"` returns nothing |


### Wiring Verified (end-to-end)
<!-- Every Wiring Test row: does the .ci exist AND exercise the claimed path?
     Read the file; do not infer it from its name. -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `reject-asn` list plus a peer `filter { import [...] }` | `test/plugin/path-asn-filter-reject.ci` | Read at closure. `cmd=foreground:seq=2:exec=ze --plugin ze.bgp-filter-path-asn --plugin ze.bgp-adj-rib-in -` runs the real daemon; it asserts `reject-asn reject`, `filter=NO-TRANSIT`, `asn=3356`, `position=transit`, `direction=import` and `as-path="65001 3356 65002"`, and carries `reject=stderr:pattern=reject-asn accept` so a filter that rejected everything would NOT satisfy it |
| the same list on a peer `filter { export [...] }` | `test/plugin/path-asn-filter-export-reject.ci` | Read at closure: it drives the forward path, not a second import case. `TestPathASNFilterReachedFromPeerExportChain` (`wiring_test.go:148`) delivers the config with `chain == "export"` and calls with `Direction: "export"` |
| `reject-asn` as a registered filter type | `internal/component/plugin/all/all_test.go` | `TestFilterTypeMappings`'s exhaustive `expected` map carries `"reject-asn": "bgp-filter-path-asn"` at `all_test.go:123` |
| `show bgp reject-asn` | `test/plugin/path-asn-show.ci` | Read at closure: the three answers go over the same RPC dispatch the CLI uses. `test/ui/completion-words-reject-asn-show.ci` proves `name` and `known transit-free` reach Tab |
| a role-bearing `ze.conf` at daemon startup | `test/plugin/path-asn-commit-refused.ci` | Read at closure: it drives `ze config validate -`, the same peer pipeline the daemon runs (`infra.ValidateBGPPeers` -> `PeersFromConfigTree`), and asserts the refusal text. The daemon half is carried by the 17 role-bearing `.ci` files that now name the filter |
| the same config through `ze config validate` | `test/plugin/path-asn-commit-inactive-accepted.ci` | Read at closure: identical peer, role and chains to the refusal file, differing only in the two `inactive:` lines, and it carries `reject=stderr:pattern=requires a filter against a transit leak`. The pair discriminates in both directions |
| the same config in the config editor | `test/editor/lifecycle/commit-blocked-missing-leak-filter.et` | Read at closure. It is an `.et` editor-lifecycle test rather than the `.ci` the plan named, because a `.ci` cannot type into the editor. `TestEditorCommitBlocksOnMissingLeakFilter` covers the same seam in Go |
| a plugin declaring the obligation at registration | `internal/component/plugin/registry/registry_test.go` | `TestRegistryReportsTransitLeakFilterTypes` (`registry_test.go:1258`) and `TestRegisterFilterObligationWithoutFilterType` (`:1306`); the second makes an obligation with no filter type a registration ERROR rather than a silent no-op |
| a peer with no role | `test/plugin/path-asn-no-role-warns.ci` | Read at closure: the config loads and the aggregated warning appears once |


### Assumptions Resolved
<!-- Every A-N. `unvalidated` is not a valid final status. A broken assumption
     needs a Mistake Log row and a Deviations entry. -->
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | CONFIRMED | `reactor_notify.go` passes `peerInfo.PeerAS` into `(*Reactor).runIngressPolicyChain`, read at the producer 2026-09-02. `senderOf` (`filter_path_asn.go:145`) consumes it and marks the sender known only on `import` |
| A-2 | CONFIRMED | `(*Peer).buildForwardFacts` reads `s.PeerAS` off the DESTINATION peer's settings. `senderOf` returns the zero `senderASN` for any direction that is not `import`, so the destination ASN is never treated as the sender. `TestExportHasNoNeighborSoViaCoversTheWholePath` |
| A-3 | CONFIRMED | the egress step loop closes before the `wireu.ASPathIntent{Prepend: ...}` call in `reactor/reactor_api_forward.go`, so the filter reads the path as stored. Harmless for a list of foreign ASNs, as the assumption predicted |
| A-4 | CONFIRMED | `(*ASPath).AppendText` flattens AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE and AS_CONFED_SET. `TestScanFlattenedASSet` and `TestASPathFieldMatchesAppendText` round-trip the reader against that producer |
| A-5 | CONFIRMED | `runIngressPolicyChain` and `runEgressPolicyChainASN4` each supply their own direction string; `FilterDecl.Direction` is written by `internal/component/plugin/server/startup.go` and read by nothing in the dispatch path. That silence is now stated on `docs/architecture/api/commands.md` |
| A-6 | RESOLVED by design | The Tier-1 set has no authoritative publisher, so the curated table DECIDES nothing: it feeds completion and the `show` annotation only. `TestCuratedTableDecidesNothing` asserts no config or filter path reads it, and `TestCuratedContestedEntryRecordsTheDispute` keeps the AS174 disagreement recorded rather than resolved |
| A-7 | CONFIRMED, with its MECHANISM corrected | The assumption held: an empty list must never mean accept-everything. Its stated mechanism did not. Phase 1 MEASURED that the schema refuses neither case on a daemon path, because `bgp` is outside `validatedSections` and an absent list has no node to walk. Both refusals now live in the plugin's own parse: `TestParseRefusesListWithNoPosition` and `TestParseRefusesEmptyASNList`. Recorded as the first Mistake Log row |
| A-8 | CONFIRMED | `(*ASPath).AppendText` emits nothing for an empty path, a bare decimal for one ASN and brackets for several. `TestScanUnbracketedSingleASN` and `TestScanAbsentASPath` |
| A-9 | RESOLVED by design | Ze ships no ASN set, so there is no default to replace and the RFC 7999 precedent does not bind |
| A-10 | CONFIRMED | `internal/component/bgp/config/peers.go` imports `internal/component/plugin/registry` and calls `registry.FilterTypesDischarging` at `peers.go:300`. The package builds, so there is no cycle |
| A-11 | CONFIRMED | `ResolveBGPTree` deep-merges the `role` container into each peer; `peerRoles` reads the resolved map and pairs by settings pointer. `TestInheritedGroupRoleBindsThePeer` |
| A-12 | CONFIRMED | `(*ConfigValidator).bgpPeerErrors` now calls `infra.ValidateBGPPeers` on the editor path with no cycle and no startup-order problem. `TestEditorCommitBlocksOnMissingLeakFilter`, `TestEditorCommitPassesWhenThePeerPipelineAccepts` and `TestEditorValidationSkipsTheEngineWithoutABGPBlock` |


### Documentation Verified
<!-- Every Yes in the Documentation checklist: verify the edited claim against
     source. Every No: paste the grep that proves no update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 1, new user-facing feature | `docs/features.md` was NOT edited; `docs/features/plugins.md` was | `grep -c 'reject-asn' docs/features/plugins.md` is non-zero. `docs/features.md` carries no per-filter-type list, so the plugin inventory row is the whole claim |
| Row 2, config syntax | `docs/guide/configuration.md` (9 mentions), `docs/architecture/config/syntax.md` (the inline leaf-list production plus the Filter Block repair), `docs/config-reference.md` | `docs/config-reference.md` was found EMPTY of `reject-asn` at closure and WRITTEN here: a new "Transit-Leak Filter (`reject-asn`)" section carrying the seven-keyword table and a source anchor on the YANG list. Recorded as a closure documentation-review finding below |
| Row 3, CLI commands | `docs/guide/command-reference.md` carries the three `show` forms; `docs/guide/command-catalogue.md` did not | Catalogue row ADDED at closure, cross-referenced against VyOS, Junos, Nokia, Arista and FRR as that table requires |
| Row 4, API/RPC | `docs/architecture/api/commands.md`: the WireMethods, plus the Filter Callbacks correction | The correction is verified against `runEgressPolicyChainASN4`, which passes `destAddrStr, destPeerAS`, and against `FilterDecl.Direction` having one writer (`internal/component/plugin/server/startup.go`) and no reader |
| Row 5, plugin inventories | `docs/guide/plugins.md`, `docs/features/plugins.md`, `docs/plugin-overview.md`, `docs/DESIGN.md` | Each names `bgp-filter-path-asn`; `git status --porcelain` lists all four modified |
| Row 6, user guide pages | `docs/guide/bgp-role.md` (3 mentions) was edited in the implementing phase; `docs/guide/bgp-policy.md` was NOT | `docs/guide/bgp-policy.md` had no `reject-asn` at closure and gained a "Rejecting a path through a transit provider" section here, plus the filter list row. Recorded as a closure documentation-review finding below |
| Row 11, daemon comparison | `docs/comparison.md` | Found EMPTY of the filter type at closure and edited here: the built-in filter sentence now names `bgp-filter-path-asn`, with a `<!-- source: -->` anchor beside its four siblings |
| Row 12, internal architecture | `docs/architecture/bgp/filter-path-asn.md` (new, 20K); `docs/architecture/config/yang-config-design.md` corrected | Every new plugin source file's `// Design:` header cites the new page |
| Row 14, Prometheus counters | `docs/plugin-development/metrics.md` | Scope-to-Prefix and Full Inventory rows for `ze_filter_path_asn_rejects_total` |
| Row 15, registered inventories | the three `testdata/*.snapshot` goldens and `TestFilterTypeMappings` | `plugins.snapshot` and `yang-providers.snapshot` each gain exactly one line, `bgp-filter-path-asn`. `wire-methods.snapshot` gains `ze-show:reject-asn`, `ze-show:reject-asn-known-transit-free`, `ze-show:reject-asn-name` |
| Row 17, existing examples that carry a `role` block | `docs/guide/bgp-peering.md`, `docs/guide/route-reflection.md`, `docs/guide/operations.md` | `grep -c 'role {'` found one example each. `bgp-peering.md` and `route-reflection.md` both showed `role { import rs }` with NO filter chain, which `leakFilterByRole["rs"]` makes a config that REFUSES to load. Both REPAIRED at closure with a `policy { reject-asn ... }` block, both chains, and a sentence naming the obligation. `operations.md` carries a prose fragment about role mismatch, not a loadable config block, and is left as it is |
| Rows 7, 8, 9, 10, 13 | N-A or No in the checklist | Verified: nothing under `pkg/plugin` changed; `rfc/not-enrolled.txt` still marks RFC 7454 `non-normative`, so no `rfc/short/` row and no `docs/features/rfc-status.md` row is owed; the filter encodes nothing and writes no route metadata key; `docs/functional-tests.md` enumerates no individual test |

## Core Insight
<!-- Optional: the single most important design revelation from this work.
     Not every spec has one. Delete the section if nothing qualifies.
     Feeds the Decisions section of the learned summary. -->
