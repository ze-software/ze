# Spec: every in-tree `show bgp` command declares the shape of its answer

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | `plan/spec-cli-pipe-operator-coverage.md` |
| Phase | 5/5 |
| Deferral shard | `plan/deferrals/cli-show-bgp-answer-shapes.md` |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-cli-pipe-operator-coverage` gave the CLI one operator language and one
rule: a command DECLARES the shape of its answer, so an operator it cannot
support is refused by name before the command runs, and the published page
states what it supports.

The `show bgp` tree does not take that channel. Of the 26 command paths under
`show bgp`, three declare a shape: `show bgp`, `show bgp peer list` and
`show bgp rib`. Every other path declares none, so `validateDeclaredShape`
returns early for it, the published surface says nothing about it, and the only
refusal it gets is the one derived from the answer already in hand.

Three defects sit under that gap.

1. Four commands change the SHAPE of their answer with their INPUT rather than
   with what they hold. `show bgp peer capabilities` and `show bgp peer
   statistics` answer one object when one peer matches and an array when several
   do, so `show bgp peer statistics | count` answers on a three-peer router and
   is refused on a one-peer router. `show bgp rpki aspa` and `show bgp
   healthcheck` answer a row set with no argument and one object with one. No
   declaration can describe an answer of two shapes, which is why none of the
   four has one.
2. `show bgp peer list` accepts `| resolve` and decorates nothing. Its rows are
   keyed by peer address and carry no address field, and the comment above the
   registration says it declares none for that reason. The declaration was never
   written: the loop over `cmdBgpChildren` calls `RegisterShape` alone, and the
   address-field registry resolves by longest registered prefix, so the path
   inherits the `address` field `show bgp` declares. Accepting an operator and
   answering nothing is the failure the refusal exists to prevent.
3. `show bgp rib` is declared twice with different values. `registerShapes` in
   the peer command plugin writes an empty declaration for it, because the path
   is one of the ten children of `show bgp`; `registerPipeFilters` in the rib
   command plugin writes `tab`. One registry key, two values, and
   `commandRegistry.register` is last-writer-wins, so the answer is decided by
   package initialization order and no test pins it.

The goal: every `show bgp` command that an in-tree package registers declares
what its answer holds, the operators it cannot support are refused by name, and
the ones it can are published.

Scope is drawn at the registration site, not at the process boundary. A command
served by a plugin process still declares in-tree when an in-core shim registers
its path: `show bgp rib *` and `show bgp irr *` each have one. The eleven
commands with no shim, under `show bgp rpki`, `show bgp rs`, `show bgp
adj-rib-in` and `show bgp healthcheck`, cannot declare anything until
`CommandDecl` carries a shape. That is
`plan/spec-plugin-declares-answer-shape.md`, and it depends on Phase 1 here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the operator language, the column
      registry, and the plugin alias channel
  → Decision: a column order never enters the payload. It is captured when the
    formatter is built, from the command string, and reaches `| table` and
    `| text` only. `| json`, `| ndjson` and `| yaml` keep alphabetical keys,
    because a program reads those three and key order carries no meaning for a
    program (owner directive, 2026-08-19).
  → Decision: "A declaration ADDS to a path. It never replaces what the path
    holds." `mergedAliases` is how the alias registry keeps a plugin's
    declaration from dropping the empty declaration the BGP command plugin puts
    on every child of `show bgp`. A shape and a column order are single values
    and cannot merge, so the same collision needs a different answer here.
  → Constraint: a command declares one column order per record shape, and the
    renderer applies the declaration naming the most keys in the record in hand.

- [ ] `ai/rules/cli.md` - what a command owes an operator
  → Constraint: every command MUST answer every operator its answer's SHAPE
    supports and MUST refuse the rest BY NAME. Accepting an operator and
    answering something plausible is worse than refusing it, because the answer
    looks right and a caller cannot tell.
  → Constraint: a command DECLARES its shape so the refusal can happen before it
    runs and the published page can state what it supports.

- [ ] `docs/contributing/ze-go-style.md` - the working standard for every line of Go
  → Constraint: `panic("BUG:")` marks a state only a Ze defect can reach, and a
    peer never reaches one. All in-tree command registration happens in
    `init()`, so a conflicting registration is exactly that state.
  → Constraint: pair the check. A property worth enforcing gets a check on two
    paths.

**Key insights:**
- The registry that resolves a shape, a column order, an address-field list, a
  pipe-filter set and an alias set is ONE implementation, `commandRegistry[T]`
  in `column_order.go`. A change to its collision rule reaches all five.
- An EMPTY declaration and an ABSENT one are already different answers there,
  deliberately: empty stops inheritance from a shorter path, absent lets it
  through. `remove` exists because "only absent restores the inheritance the
  declaration stopped".
- That distinction is what makes the collision rule below possible without a
  list of exceptions. An empty declaration is a floor, never a claim.
- `T` is not uniformly a slice: `aliasSet` and `pipeFilterSet` are structs. So
  emptiness cannot be tested generically and each registry states its own.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/column_order.go` - `commandRegistry[T]`:
      `register` overwrites one key, `lookup` resolves by longest registered
      prefix, `get` reads one path without inheritance, `remove` deletes a path.
      `RegisterColumns` drops an empty order from `stored`, so a declaration of
      one empty order and a declaration of none are already the same thing.
- [ ] `internal/component/command/answer_shape.go` - `RegisterShape`,
      `ShapeForCommand`, `RegisterAddressFields`, `AddressFieldsForCommand`,
      `rowsInKeyed`, `ShapeOfAnswer`. A registration of no shapes stores an empty
      slice and `ShapeForCommand` answers `(doc, false)` for it. `rowsInKeyed`
      treats an EMPTY answer as zero rows rather than as a shape that cannot hold
      them, and it handles a row set spelled as a map keyed by identity.
- [ ] `internal/component/command/pipe.go` - `validateDeclaredShape` returns the
      empty string immediately when the command declared no shape, so an
      undeclared command reaches no pre-dispatch refusal at all. It is the only
      non-test caller of `AddressFieldsForCommand`.
- [ ] `internal/component/command/pipe_catalog.go` - three shapes, three classes,
      and `NeedsAddressField` on `resolve` and `origin`.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `cmdBgpChildren` and
      the three registration helpers. `registerShapes` declares `tab` for
      `show bgp` and `show bgp peer list`, declares the `address` field on
      `show bgp`, and declares an empty shape for each of the ten children. It
      declares no address field for any child and none for `show bgp peer list`.
      `handleBgpPeerList` answers one envelope holding `peers`, a map keyed by
      peer address whose rows carry `remote-as`, `state`, `uptime`, and `name`
      and `group` when set.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` -
      `handleBgpPeerCapabilities` and `handleBgpPeerStatistics` answer a single
      map for one matched peer and a slice of maps for several.
      `handleBgpOverview` refuses a token that names no family and no
      subcommand, then answers the summary.
- [ ] `internal/component/bgp/plugins/cmd/peer/health.go` -
      `handleShowBGPHealth` answers one envelope holding `peers`, `count` and
      `not-established`, each row carrying `peer`, `state`, `as` and `uptime`.
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters`
      declares `tab`, a column order and the `peer` and `next-hop` address fields
      for `show bgp rib`, an empty filter set for the three scalar rib commands,
      and nothing at all for `show bgp rib best` despite declaring twelve pipe
      filters for it.
- [ ] `internal/component/bgp/plugins/filter_irr/cmd_irr.go` - six forwarders,
      three of them `show`. It declares no shape, no column order and no address
      field for any of them.

**The answers, as their producers write them.** Every row below was read from
the producing function.

| Command | Producer | Rows under | Row fields | Address or prefix fields |
|---------|----------|------------|-----------|--------------------------|
| `show bgp` | `handleBgpSummary` | `peers` | already declared | `address` |
| `show bgp peer list` | `handleBgpPeerList` | `peers`, keyed by address | `name`, `group`, `remote-as`, `state`, `uptime` | none: the address is the KEY |
| `show bgp peer detail` | `handleBgpPeerDetail` | the answer itself, keyed by address | `remote-as`, `local-as`, `router-id`, `peer-type`, `timer`, `connect`, `accept`, `state`, `uptime`, the counters, `messages`, `connections-established`, `connections-dropped`, `flap-count`, `connect-retry-counter` | `router-id`; the address is the KEY |
| `show bgp peer capabilities` | `handleBgpPeerCapabilities` | none today | `peer`, `state`, `negotiation-complete`, `negotiated` | `peer` |
| `show bgp peer statistics` | `handleBgpPeerStatistics` | none today | `address`, `remote-as`, `state`, `uptime`, four counters, four rates | `address` |
| `show bgp peer history` | `handlePeerHistory` | `transitions` | `timestamp`, `from`, `to`, `reason` | none |
| `show bgp health` | `handleShowBGPHealth` | `peers` | `peer`, `state`, `as`, `uptime` | `peer` |
| `show bgp rib best` | `bestPathRows`, row value `bestResult` | `best-path` | `family`, `prefix`, `best-peer`, `multipath-peers`, `attributes` | `prefix`, `best-peer`, `multipath-peers` |
| `show bgp rib status` | `RIBManager.status` | none: `route-counts` and `gr-state` are maps keyed by peer address, and two row sets in one answer is refused | — | the map KEYS are addresses |
| `show bgp rib best status` | `bestPathStatus` | none | `running`, `peers-with-rib`, `total-routes` | none |
| `show bgp rib rpf` | `rpfLookup` | none | `source`, `family`, `found`, and when found `matched-prefix`, `next-hop`, `admin-distance`, `metric` | `source`, `matched-prefix`, `next-hop` |
| `show bgp irr` | `showIRR` | `entries` | `asn`, `as-set`, `status`, `error`, `ipv4-count`, `ipv6-count`, `last-refresh`, `peers` | `peers` holds addresses, as an array inside a row |
| `show bgp irr prefix` | `renderPrefixes` | `prefixes`, an array of bare strings | — | every element is a prefix |
| `show bgp irr check` | `showIRRCheck` | none | `prefix`, `asn`, `accepted`, `matched-entry` | `prefix` |

**Corrections to the table above, 2026-08-24, Phase 4.** Each was read from the
producing function, and each changed what was declared.

| Row | What the table says | What the producer does |
|-----|--------------------|------------------------|
| missing row | — | `show bgp peer rib` is a sixteenth in-tree path. `forwardRibRoutes` (`cmd/rib/rib.go`) sends it to `show bgp rib` with a peer selector, so it answers route rows and declares what `show bgp rib` declares. `TestEveryShowBgpPathDeclaresAShape` found it, which is what deriving the population buys |
| `show bgp peer detail` | rows under "the answer itself, keyed by address" | `handleBgpPeerDetail` answers `plugin.Map{"peers": result}`, an ENVELOPE holding the keyed map, exactly as `show bgp peer list` does |
| `show bgp peer detail` | address field `router-id` | RFC 6286 Section 2.1 makes the BGP Identifier a 4-octet unsigned integer that need not be an IPv4 address, and RFC 4456 Section 7 says the same of CLUSTER_ID. Neither is declared. The two fields that hold a real address are `local-ip` and `next-hop-address`; `next-hop` holds the MODE ("auto", "self", "unchanged", "explicit") |
| `show bgp rib best` | address fields `prefix`, `best-peer`, `multipath-peers` | `best-peer` and the `next-hop` inside `attributes`, which is what AC-15 says. `prefix` is a prefix and fails `netip.ParseAddr`; `multipath-peers` is an ARRAY, and both transforms walk past an array element |
| `show bgp irr` | address field `peers` | Declared as NONE, for the same reason: `peers` is an array of address strings inside a row, so `\| resolve` would be admitted and would decorate nothing. `show bgp peer list` is the precedent this spec already set |
| `show bgp rib status` | "two row sets in one answer is refused" | True only while a peer is in graceful restart. See A-5 |

**Behavior to preserve:**
- The answer PAYLOAD of every `show bgp` command except the two named in
  "Behavior to change". A declaration says what an answer holds; it does not
  change it.
- Alphabetical key order in `| json`, `| ndjson` and `| yaml`.
- The empty declarations the peer command plugin writes for the ten children of
  `show bgp`. They stop the children inheriting the parent's peer columns and
  the `summary` and `peers` aliases, and every one of those reasons still holds.
- `show bgp peer list` refusing `| resolve` and `| origin`. The registration
  comment states that intent; this spec makes the code match it.
- `show bgp rib` and `show bgp rib best` keeping their command-specific pipe
  filters. A declared shape and a declared filter are different channels.

**Behavior to change:**
- `show bgp peer capabilities` and `show bgp peer statistics` answer rows
  whatever the number of matched peers.
- `show bgp rib` resolves to `tab` whatever the package initialization order.
- `show bgp peer list` and the ten children of `show bgp` declare an empty
  address-field list, so none of them inherits the `address` field.
  **Corrected 2026-08-24, Phase 2:** "none of them" overstated it. `show bgp rib`
  declares a REAL list, `peer` and `next-hop` (`registerPipeFilters`,
  `internal/component/bgp/plugins/cmd/rib/rib.go`), and the floor rule from
  Phase 1 preserves it: the empty declaration this package writes never
  overrides a value. So nine children resolve nothing and `show bgp rib`
  resolves its own two fields. A test that blanket-asserts emptiness over
  `cmdBgpChildren` would be asserting a defect, and Phase 4 MUST NOT write one.
- Twelve further in-tree `show bgp` paths gain a declaration.

## Data Flow (MANDATORY)

### Entry Point
- An operator types a command and an operator chain at the CLI, or a program
  calls the same path over the API. The chain is parsed before dispatch.

### Transformation Path
1. `parsePipeChain` splits the command from the chain and expands aliases.
2. `foldFilters` rewrites a command's own pipe filters into server-side
   arguments, for the paths that declare them.
3. `validateDeclaredShape` reads the shape and the address-field list the
   command DECLARED and refuses an operator neither supports. This is the step
   that does nothing today for 23 of the 26 `show bgp` paths.
4. The command is dispatched. An in-tree handler answers a `ResponseData`; a
   command with a shim forwards to a plugin process, whose answer reaches the
   host as `RawJSON`.
5. `ShapeOfAnswer` derives the shape of the answer in hand, and the row
   operators refuse again from it. This universal half is unchanged.
6. The renderers apply the declared column order, for `| table` and `| text`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | none. A shim declares in-tree and forwards the command unchanged; the plugin's answer is not reshaped | No |
| Client ↔ Daemon | none. A declaration is read in the process holding the registry, and the chain is validated as it is today | No |

### Integration Points
- `commandRegistry[T]` - the collision rule changes here and reaches all five
  registries at once.
- `registerShapes` / `registerColumns` in the peer command plugin - the two
  loops over `cmdBgpChildren` gain a third.
- `registerPipeFilters` in the rib command plugin - already the site where the
  rib paths declare; the four undeclared rib paths join it.
- `cmd_irr.go` - gains a registration helper it does not have today.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | No path in the tree is declared twice with two DIFFERENT non-empty values, in any of the five registries, other than `show bgp rib` | Read of every `RegisterShape`, `RegisterColumns`, `RegisterAddressFields`, `RegisterPipeFilters` and `RegisterAliases` call site: 23 sites, of which one pair collides | The conflict guard panics at daemon start on an unrelated path, taking the daemon down | A unit test over the composition root, plus a daemon start in a `.ci` | confirmed (Phase 1): the `cmd/peer` test binary imports `internal/component/plugin/all` (`all_import_test.go`) and every test ran; `bin/ze help command --json` built and answered 395 commands. The guard fired on no path |
| A-2 | An empty declaration is never the intended ANSWER for a path another package declares non-empty | `column_order.go` states an empty declaration exists to stop inheritance, never to describe an answer. `RegisterColumns` already discards an empty order | The floor rule silently overrides a deliberate emptiness | Read of every empty-registration site and its comment | confirmed (Phase 1): five empty-registration sites exist, in `registerColumns`, `registerShapes` and `registerAliases` in `cmd/peer/peer.go`, `registerPipeFilters` in `cmd/rib/rib.go`, and the barrier loop in `RegisterPluginAliases`. Every comment states the same purpose, which is to stop inheritance |
| A-3 | `handleBgpPeerCapabilities` and `handleBgpPeerStatistics` have no caller depending on the single-peer object form | Both are reached only through the dispatcher, from a command path | A caller breaks on the row form | `gopls references` on both, and a grep of `test/`, `internal/component/web/` and `internal/component/api/` for the two command paths | confirmed (Phase 3): `gopls references` answers the `RPCRegistration` line in `summary.go` and test files only. No production caller decodes either answer. `rest/server.go` builds `show bgp` and `show bgp peer <name> detail`, never these two, and `handleToolOverlay` in `internal/component/web/handler_tools.go` renders whatever the dispatcher answers as text. The one consumer that read the object form was `test/plugin/api-peer-capabilities.ci`, which asserted the defect and is corrected |
| A-4 | Declaring a shape on a path that also declares pipe filters does not change how `foldFilters` rewrites them | The two registries are read at different steps and neither reads the other | `show bgp rib best \| count` stops answering | The existing rib `.ci` tests, run before and after Phase 4 | confirmed (Phase 4): `TestRibBestFiltersSurviveDeclaration` was green BEFORE the declaration and green after, over `count`, `graph`, `histogram` and `reason`. Each still folds into the command and is answered by the producer, and the formatter does not fold the rows into a number of its own |
| A-5 | `show bgp rib status` genuinely has no row set, because two identity-keyed maps in one answer is the ambiguous case `rowsInKeyed` refuses | Read of `rowsInKeyed` and of `RIBManager.status` | Declaring `doc` refuses a row operator that used to answer | A `.ci` asserting the refusal names the operator | **broken in its REASON, confirmed in its CONCLUSION (Phase 4).** `RIBManager.status` (`internal/component/bgp/plugins/rib/rib_commands.go`) writes `gr-state` only when `len(r.grState) > 0`, so the answer holds TWO identity-keyed maps only while a peer is in graceful restart. With none, `route-counts` is the single row set and `rowsInKeyed` answers ROWS, so `\| count` answered the peer count; with an empty `route-counts` it answers neither. The derived shape therefore had three readings for one command, which is a stronger reason to declare than the one assumed. Declaring `doc` makes the refusal the answer in all three |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The conflict guard panics at start on a path this spec did not look at | The daemon fails to start, in any `.ci` | The guard names the registry, the path and both values, so the report is the diagnosis. Fix the conflict; do not weaken the guard |
| R-2 | Declaring `tab` on a command whose answer is sometimes empty makes a row operator refuse where it used to answer | A `.ci` asserting on an empty answer goes red | `rowsInKeyed` already treats an EMPTY answer as zero rows. The declared shape is checked before dispatch and the answer's own shape after, so an empty answer keeps exiting 0 |
| R-3 | Changing the two cardinality-varying answers breaks a `.ci` asserting the single-peer object form | A `.ci` under `test/ui/` or `test/plugin/` goes red | The test asserts the defect. Correct it and add a Mistake Log row |
| R-4 | A declared column name is absent from the payload, so the order is silently inert | Nothing fails. This is the risk with no signal | Every declared name is read from the producing function, and a unit test asserts each declared name against the keys a handler writes for a fixture |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing on the wire and nothing in the RIB. The failure modes are an operator surface that refuses what it should answer, a renderer that orders columns wrongly, and, for the conflict guard alone, a daemon that refuses to start |
| How is it reverted? | A single commit revert per phase. No config migration, no persisted state, nothing a peer has seen |
| Who else touches this path? | `spec-cli-pipe-operator-coverage` (status `verification`, owes an independent review) and `spec-plugin-registers-pipe-operations` (in-progress, phase 6 of 6) both own files in `internal/component/command`. Neither has those files checked out dirty as of 2026-08-24. `plan/handoff-cli-remaining.md` names a separate argument-versus-pipe boundary defect in the same surface, untouched here |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp health \| count` typed at the CLI | → | `validateDeclaredShape` reads the `tab` declared by `registerShapes` | `test/ui/show-bgp-declared-shapes.ci` |
| `show bgp peer list \| resolve` typed at the CLI | → | `validateDeclaredShape` reads the empty address-field list and refuses by name | `test/ui/show-bgp-declared-shapes.ci` |
| `show bgp peer statistics \| count` on a ONE-peer router | → | `handleBgpPeerStatistics` answers rows | `test/ui/show-bgp-peer-rows.ci` |
| `show bgp irr \| display asn status` typed at the CLI | → | the order declared in `cmd_irr.go` reaches `tableStyle.orderKeys` | `test/ui/show-bgp-declared-shapes.ci` |
| Two packages declare one path with two non-empty values | → | `commandRegistry.register` panics | `TestRegisterConflictPanics` |
| A process importing both the peer and the rib command plugins resolves `show bgp rib` | → | the floor rule | `TestShowBgpRibResolvesToTab` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A path is registered twice in one registry, first empty then non-empty, in either order | `lookup` answers the non-empty value, whatever the order |
| AC-2 | A path is registered twice in one registry with two DIFFERENT non-empty values | Registration panics, and the message names the registry, the path and both values |
| AC-3 | A path is registered twice with the SAME value | No panic, and `lookup` answers that value |
| AC-4 | `show bgp rib` is resolved for its shape in a process importing both command plugins | The answer is `tab`, and a test pins it |
| AC-5 | `show bgp peer list \| resolve` | Refused by name, saying no field of the answer is declared to hold an IP address |
| AC-6 | `show bgp peer list \| count` | Answers the number of peers, unchanged |
| AC-7 | ~~`show bgp rpki \| resolve`~~ `show bgp peer history \| resolve` | Refused by name. The child no longer inherits the `address` field `show bgp` declares. **Superseded 2026-08-24, Phase 2.** The original wording named a path this spec can never satisfy. `validateDeclaredShape` (`internal/component/command/pipe.go`) returns at `if !declared` BEFORE it reads the address-field list, so a path is refused for an address operator only once it declares a SHAPE. `show bgp rpki` is served by a plugin process with no in-core shim, so its shape is declared by `plan/spec-plugin-declares-answer-shape.md` and never here. `show bgp peer history` tests the same property on a path this spec does declare: its rows carry `timestamp`, `from`, `to` and `reason` and no address. The `show bgp rpki` case moves to the sibling spec as its AC-10. Phase 2 stays load-bearing for both: without the empty declaration, the path would resolve `show bgp`'s `address` and ACCEPT the operator the moment a shape is declared |
| AC-8 | `show bgp peer statistics` with ONE matched peer | Answers rows, in the same spelling it uses for several |
| AC-9 | `show bgp peer statistics \| count` with ONE matched peer | Answers 1 |
| AC-10 | `show bgp peer capabilities` with ONE matched peer | Answers rows, in the same spelling it uses for several |
| AC-11 | `show bgp health \| count` | Answers the peer count rather than being refused |
| AC-12 | `show bgp health \| resolve` | Decorates the `peer` field |
| AC-13 | `show bgp peer history \| first 1` | Answers the first transition |
| AC-14 | ~~`show bgp rib best \| display prefix next-hop`~~ | Answers those two fields, in that order. **OPEN, 2026-08-24, Phase 4: this chain cannot answer `next-hop` for this command, and the reason is the payload rather than the declaration.** `bestResult` (`internal/component/bgp/plugins/rib/rib_pipeline_best.go`) writes `family`, `prefix`, `best-peer`, `multipath-peers` and `attributes`, and carries the next hop INSIDE `attributes` (`enrichRouteMapFromEntry`, `rib_attr_format.go`). `selectRecord` (`internal/component/command/pipe_columns.go`) cuts a record that names at least one displayed field to the displayed ones, so naming `prefix` drops `attributes` and the next hop with it. That rule is deliberate and documented (`docs/architecture/api/commands.md`, "A record that carries at least one displayed field is cut to the displayed ones"), and this spec's own Current Behavior table does not list `next-hop` among the row's fields, so the AC names a field the row does not have. It is likely written against `show bgp rib`, whose route row DOES carry `prefix` and `next-hop` side by side. Phase 4 proves the property on `\| display prefix best-peer`, two keys of the same row (`TestRibBestDeclaresItsOwnRows`), and leaves the AC open for Thomas: correct the chain, or flatten the next hop out of `attributes`, which "Behavior to preserve" forbids without his ruling. **RESOLVED 2026-08-24, main thread: correct the CHAIN. AC-14 now reads `show bgp rib best \| display prefix best-peer`.** Flattening the next hop out of `attributes` changes a payload this spec's "Behavior to preserve" undertakes not to change, and it would change it for every reader of `show bgp rib best`, not only for the operator who typed `\| display`. The AC exists to prove that a declared row answers `\| display` in the named order, and two keys of the same row prove exactly that. Which fields a route row SHOULD carry at its top level, as against inside `attributes`, is a real question and a separate one: it is about the payload's design, not about whether the payload is declared, and it gets its own row in the deferral shard |
| AC-15 | `show bgp rib best \| origin` | Decorates `best-peer` and the next-hop, and no other field |
| AC-16 | `show bgp rib status \| count` | Refused by name, saying the answer holds no rows |
| AC-17 | `show bgp irr \| display asn status` | Answers those two fields, in that order |
| AC-18 | `show bgp irr check \| first 1` | Refused by name: the answer is one document |
| AC-19 | `ze help command --json` for any in-tree `show bgp` path | Lists the operators that path supports, derived from its declared shape |
| AC-20 | Every in-tree-registered `show bgp` path | Declares a shape; declares a column order and an address-field list where its answer has rows and addresses |
| AC-21 | A declared column name | Is a key the producing handler actually writes |
| AC-22 | `show bgp rib best \| count` and `show bgp rib best \| graph` | Keep working exactly as they do today. A declared shape does not change how a command's own pipe filters are folded |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Counts the peers that are not established: `show bgp health \| match Idle \| count` | CLI → declared `tab` → dispatch → `handleShowBGPHealth` → row operators | `test/ui/show-bgp-declared-shapes.ci` |
| 2 | Reads one peer's update rate: `show bgp peer 192.0.2.1 statistics \| display address rate-updates-received` | CLI → declared `tab` → `handleBgpPeerStatistics` answers rows → `applyDisplaySelect` | `test/ui/show-bgp-peer-rows.ci` |
| 3 | Finds who originates the best paths: `show bgp rib best \| origin` | CLI → declared address fields → shim → rib process → `applyOrigin` | `test/ui/show-bgp-declared-shapes.ci` |
| 4 | Asks what an unfamiliar command supports: `ze help command "show bgp rib best" --json` | published catalog ← `ShapeForCommand` ← the rib shim's declaration | `test/ui/show-bgp-declared-shapes.ci` |
| 5 | Types an operator a command cannot support: `show bgp rib status \| first 2` | CLI → `validateDeclaredShape` refuses by name before dispatch | `test/ui/show-bgp-declared-shapes.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterConflictPanics` | `internal/component/command/column_order_test.go` | AC-2, over all four declaration registries | red then green |
| `TestRegisterEmptyThenNonEmpty` | `internal/component/command/column_order_test.go` | AC-1, both orders | red then green |
| `TestRegisterIdenticalIsNoOp` | `internal/component/command/column_order_test.go` | AC-3 | green, and it is the boundary of the new guard rather than of the old behavior |
| `TestShowBgpRibResolvesToTab` | `internal/component/bgp/plugins/cmd/rib/rib_shape_test.go` | AC-4, in a package importing both plugins | red then green |
| `TestShowBgpChildrenDeclareNoAddressField` | `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go` | AC-5, and the registry half of AC-7, table-driven over `cmdBgpChildren` and `show bgp peer list` | red then green (Phase 2). `show bgp rib` is the one path asserted NON-empty: the rib command plugin declares `peer` and `next-hop` for it, and the floor rule keeps them |
| `TestPeerListRefusesResolveAndAnswersCount` | `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go` | AC-5 and AC-6 through `ProcessPipesDefaultFormatChecked`, which is the refusal path an operator reaches | red then green (Phase 2) |
| `TestPeerStatisticsAnswersRowsForOnePeer` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-8, and AC-9 through `ParsePipe` plus `ApplyPipes`, which is the row-counting path an operator's `\| count` reaches | red then green (Phase 3). It also pins the several-peer answer, so a change that moved both branches to one new shape cannot pass on the equalities alone |
| `TestPeerCapabilitiesAnswersRowsForOnePeer` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-10, over the same helper | red then green (Phase 3) |
| `TestEveryShowBgpPathDeclaresAShape` | `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go` | AC-20: the population is derived from the registered command list, so a path added later fails until it declares | red then green (Phase 4). The derivation joins `pluginserver.AllBuiltinRPCs` to `yang.WireMethodToPaths` and answers 16 paths, one of which the Current Behavior table above missed: `show bgp peer rib` |
| `TestDeclaredColumnsExistInPayload` | `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go`, `cmd/rib/rib_shape_test.go`, `filter_irr/cmd_irr_shape_test.go` | AC-21: every declared name is a key the handler writes for a fixture | red then green (Phase 4). One test of that name per declaring package. The peer and irr ones run the REAL producers; the rib producers live in the plugin process, so those are fixtures that each name the producing function |
| `TestDeclaredAddressFieldsHoldAnAddress` | `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go` | Every declared address field is a bare address string in a row, which is the one form `resolveJSON` and `originJSON` decorate | red then green (Phase 4) |
| `TestRibScalarPathsDeclareForThemselves` | `internal/component/bgp/plugins/cmd/rib/rib_shape_test.go` | AC-16, over the three rib paths that Phase 1 left inheriting the route declaration | red then green (Phase 4) |
| `TestRibBestDeclaresItsOwnRows` | `internal/component/bgp/plugins/cmd/rib/rib_shape_test.go` | AC-15, and AC-14 in the form the payload supports | red then green (Phase 4) |
| `TestPeerRibDeclaresTheRouteShape` | `internal/component/bgp/plugins/cmd/rib/rib_shape_test.go` | AC-20 for `show bgp peer rib` | red then green (Phase 4) |
| `TestIRRCommandsDeclareTheirShape` | `internal/component/bgp/plugins/filter_irr/cmd_irr_shape_test.go` | AC-18, AC-20 for the irr branch | red then green (Phase 4) |
| `TestRibBestFiltersSurviveDeclaration` | `internal/component/bgp/plugins/cmd/rib/rib_shape_test.go` | AC-22 | green before AND after the declaration, which is what the AC asks. It was written before `registerRibResultShapes` existed |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| matched peer count for `show bgp peer statistics` | 0-N | 1 | 0, which answers "no matching peers" | N/A |
| row count for a declared `tab` answer | 0-N | 0, which stays a valid empty answer at exit 0 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-bgp-declared-shapes` | `test/ui/show-bgp-declared-shapes.ci` | An operator counts, filters, selects and resolves over the in-tree `show bgp` commands, and is refused BY NAME where the answer holds no rows | green (Phase 4). `make ze-functional-ui-test` is 196/196. It DISCRIMINATES, proven twice by removing `registerRibResultShapes` and re-running. The first run of that probe exposed a vacuity in the test itself and is why it asserts the pre-dispatch wording: `rowOperatorRefusal` (`internal/component/command/pipe.go`) answers "count needs rows, and this answer has none: it holds one document" AFTER dispatch, which shares both substrings with the refusal a declaration produces, so three assertions passed with every declaration removed. "cannot apply here" is written by `validateDeclaredShape` alone, and pinning it is what makes each refusal evidence that the command did not run |
| `show-bgp-peer-rows` | `test/ui/show-bgp-peer-rows.ci` | An operator runs one chain against a one-peer and a two-peer router and gets the same shape of answer | green (Phase 3). It drives a real daemon over SSH with two configured peers, and selects one of them and both of them in turn, which is the one-matched-peer case the operator meets. `make ze-functional-ui-test` is 194/194 |

### Interop Tests (Scope: protocol)
Not applicable. Nothing here is wire-visible: no BGP message changes, no
capability changes, no route changes. The scope is the operator surface.

## Files to Modify
- `internal/component/command/column_order.go` - the collision rule in
  `commandRegistry.register`, and each registry states its own emptiness
- `internal/component/command/answer_shape.go` - the two registries state their
  emptiness; the doc comments that describe the old collision behavior
- `internal/component/command/alias.go` - the alias registry states its emptiness
- `internal/component/command/pipe_filter.go` - the filter registry states its
  emptiness
- `internal/component/bgp/plugins/cmd/peer/peer.go` - the child loop declares an
  empty address-field list; declarations for the peer paths
- `internal/component/bgp/plugins/cmd/peer/summary.go` - the two
  cardinality-varying handlers answer rows
- `internal/component/bgp/plugins/cmd/peer/health.go` - the `show bgp health`
  declaration
- `internal/component/bgp/plugins/cmd/rib/rib.go` - the four undeclared rib paths
- `internal/component/bgp/plugins/filter_irr/cmd_irr.go` - the three irr paths
- `docs/architecture/api/commands.md` - the collision rule
- `docs/features/cli-commands.md` - the declaration and the refusal, for an
  operator (Phase 5)
- `docs/guide/command-reference.md` - the two changed answers, the deterministic
  peer order, and a stale claim about which commands declare (Phase 5)
- `docs/features/formatting.md` - the same stale claim, in the column-order
  section (Phase 5)

## Files to Create
- `internal/component/bgp/plugins/cmd/rib/rib_shape_test.go`
- `test/ui/show-bgp-declared-shapes.ci`
- `test/ui/show-bgp-peer-rows.ci`
- `plan/deferrals/cli-show-bgp-answer-shapes.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No new command path and no new config leaf. Every path in scope is already registered |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | No command is added, renamed or removed |
| CLI grammar (keyword before value) | No | No grammar change |
| Editor autocomplete | Yes | `completeDisplayFields` reads the column registry, so a newly declared order makes `\| display <partial>` complete where it does not today |
| Functional test for new RPC/API | Yes | The two `.ci` files above |
| Pipe completeness | Yes | This spec IS the pipe-completeness work for the in-tree `show bgp` tree |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port or binary |
| Prometheus counters/metrics | No | A declaration is not observable state |
| BGP family surface | N-A | No SAFI, capability or attribute change |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/cli-commands.md`: DONE (Phase 5). New section "A command declares what its answer holds", with the refusal `show bgp rib status \| count` answers and the paths that declare nothing |
| 2 | Config syntax changed? | No | No config leaf changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: DONE (Phase 5). The `peers` envelope both commands now answer, what a script that read the single-peer form must change, and the ascending peer order `reactorAPIAdapter.Peers` now answers |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: DONE (Phase 5). "Per-command declarations" now names five registries, the collision rule and its alias exception, the pre-dispatch refusal, and the selector spelling that declares nothing |
| 5 | Plugin added/changed? | No | No plugin registration changes here |
| 6 | Has a user guide page? | No | No new topic |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | `CommandDecl` is untouched here. That is the sibling spec |
| 9 | RFC behavior? | N-A | Nothing here implements an RFC obligation |
| 10 | Test infrastructure changed? | No | The two `.ci` files use the existing runner |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md`: DONE (Phase 5), in the same section as row 4. "A plugin cannot declare a column order" gained the shim exception |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered command or capability changed? | No | The set of registered commands is unchanged |
| 16 | Changed source file referenced by doc source anchors? | DERIVED | Run `python3 scripts/dev/spec_doc_anchors.py plan/spec-cli-show-bgp-answer-shapes.md` at the start of each phase. `docs/architecture/bgp/filter-irr.md` is declared by `filter_irr/cmd_irr.go` and is UNAFFECTED: that file gains a registration helper stating what the three `show bgp irr` answers already hold, and the doc describes what the IRR filter does to routes. No behavior the doc records changes |
| 17 | Existing docs show examples for this area? | Yes | Checked (Phase 5). `docs/architecture/api/commands.md` shows no payload for either changed command: its "Peer Commands" block lists spellings alone, and the two JSON examples below it are `show neighbor` and `show adj-rib`. Two STALE claims were found elsewhere and corrected: `docs/features/formatting.md` said only `show bgp` and `show bgp peer list` declare an order, and `docs/guide/command-reference.md` said `show bgp rib` renders alphabetically |

## Implementation Steps

1. **Phase 1: Wiring and the collision rule (MANDATORY FIRST)**
   - The registry stops being last-writer-wins. An empty declaration is a floor:
     a non-empty value replaces it whatever the order. Two different non-empty
     values for one path panic with `BUG:`, naming the registry, the path and
     both values. An identical re-registration is a no-op. `T` is not uniformly
     a slice, so each of the five registries states its own emptiness at
     construction rather than the registry testing it generically.
   - This fixes defect 3 with no list of exceptions to maintain: `show bgp rib`
     resolves to `tab` because `tab` is non-empty and the peer plugin's
     declaration is empty, whatever the initialization order.
   - Tests: `TestRegisterConflictPanics`, `TestRegisterEmptyThenNonEmpty`,
     `TestRegisterIdenticalIsNoOp`, `TestShowBgpRibResolvesToTab`
   - Files: `column_order.go`, `answer_shape.go`, `alias.go`, `pipe_filter.go`
   - Verify: A-1 and A-2 confirmed or broken. A daemon start and the full unit
     suite both pass, which is what proves no other path collides
2. **Phase 2: The address-field inheritance defect**
   - The child loop declares an empty address-field list beside the empty shape
     it already declares, and `show bgp peer list` declares one of its own.
   - Tests: `TestShowBgpChildrenDeclareNoAddressField`,
     `TestPeerListRefusesResolveAndAnswersCount`
   - Files: `cmd/peer/peer.go`
   - Verify: AC-5 and AC-6 in full. AC-7 in half, and the other half is Phase
     4's: `validateDeclaredShape` returns at `if !declared` before it reads the
     address fields (`internal/component/command/pipe.go`), so a path declaring
     no SHAPE is refused nothing. `show bgp rpki` declares none until Phase 4
     gives it one. Phase 2 is what makes that declaration refuse rather than
     accept: without the empty list, the shape Phase 4 adds would publish
     `| resolve` over the `address` field inherited from `show bgp`
3. **Phase 3: One shape whatever the input**
   - `show bgp peer capabilities` and `show bgp peer statistics` answer rows for
     one matched peer as they do for several. Confirm A-3 first.
   - Tests: `TestPeerStatisticsAnswersRowsForOnePeer`,
     `TestPeerCapabilitiesAnswersRowsForOnePeer`, `test/ui/show-bgp-peer-rows.ci`
   - Files: `cmd/peer/summary.go`
   - Verify: AC-8, AC-9, AC-10
4. **Phase 4: Every in-tree `show bgp` path declares**
   - Each path gains a shape, a column order where it answers rows, and an
     address-field list naming each field that holds an address or a prefix. The
     table in Current Behavior is the input, and every name in it was read from
     the producing function. `TestEveryShowBgpPathDeclaresAShape` derives its
     population from the registered command list, so the gate stays true as
     paths are added.
   - Tests: `TestEveryShowBgpPathDeclaresAShape`,
     `TestDeclaredColumnsExistInPayload`, `TestRibBestFiltersSurviveDeclaration`,
     `test/ui/show-bgp-declared-shapes.ci`
   - Files: `cmd/peer/peer.go`, `cmd/peer/health.go`, `cmd/rib/rib.go`,
     `filter_irr/cmd_irr.go`
   - Verify: AC-11 to AC-22
5. **Phase 5: Documentation**
   - Files: `docs/architecture/api/commands.md`, and every row the Documentation
     Update Checklist answers Yes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every in-tree-registered `show bgp` path has a declaration, and the test saying so derives its population rather than listing it |
| Correctness | The collision rule treats empty and absent as the different answers the registry already says they are. A non-empty value never silently replaces a different non-empty value |
| Correctness | The conflict guard is reachable only from `init()`. It MUST NOT become reachable from a plugin message, where a refusal rather than a panic is owed |
| Correctness | A path that declares pipe filters AND a shape still folds its filters. `show bgp rib best \| count` answers from the producer, not from the row operator, and that is unchanged |
| Naming | Every declared column name is a key the handler actually writes. A declared name the payload never carries orders nothing and publishes a field that does not exist |
| Naming | Every declared address field holds an address or a prefix in EVERY branch the producer can take, not only the common one. `show bgp rib rpf` writes `matched-prefix` and `next-hop` in one branch only |
| Data flow | A declaration reaches `\| table` and `\| text` only. The three program-facing formats keep alphabetical keys |
| Rule: `ai/rules/cli.md` | No command accepts an operator and answers something plausible. Where the answer cannot support one, it is refused BY NAME |
| Rule: `ai/rules/evidence.md` | Every declared field name was read from the producing function, never inferred from a sibling command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every in-tree `show bgp` path declares a shape | `TestEveryShowBgpPathDeclaresAShape` |
| The registry refuses a conflicting declaration | `TestRegisterConflictPanics` |
| Every declared column name exists in the payload | `TestDeclaredColumnsExistInPayload` |
| The published catalog states each command's operators | `make ze-command-list-json`, read a `show bgp` entry |
| No operator is accepted and ignored | The two `.ci` files assert a refusal BY NAME for each unsupported operator |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | No new operator input is parsed. The declarations are compile-time constants in this spec |
| Resource exhaustion | None: the registries are written at `init()` and read afterwards |
| Error leakage | A refusal names the operator and the shape. It MUST NOT echo the command with its arguments, which `validateDeclaredShape` already avoids because a path an operator typed can hold a file path |
| Authorization | None. A declaration changes rendering and refusal, never what a caller may run |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The conflict guard panics on a path outside this spec | Fix the conflict. A-1 was wrong, so record it and correct the assumption row. Do NOT weaken the guard |
| A `.ci` asserts the single-peer object form | The test asserts the defect. Correct it, and add a Mistake Log row |
| A declared column name is absent from the payload | Re-read the producing function. The declaration is wrong, never the payload |
| A rib pipe filter stops folding after Phase 4 | A-4 was wrong. Report it before changing either registry |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The registry already distinguishes an EMPTY declaration from an ABSENT one,
  and `remove` exists because only absence restores inheritance. That
  distinction was built for a different purpose and it is exactly what makes a
  correct collision rule possible without a list of exceptions.
- The alias channel met the same collision and answered it by MERGING. A set can
  merge; a shape and a column order cannot, because neither has a meaningful
  union. So the two channels need different rules for one situation, and the
  reason is the ARITY of the value rather than anything about plugins.
- Four `show bgp` commands answer a different shape for a different ARGUMENT,
  not for a different row count. Two are in this spec; `show bgp rpki aspa` and
  `show bgp healthcheck` are in the sibling. The class is one class and the fix
  is the same: answer a one-row set where a selector picks one thing.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An empty declaration is a floor that a non-empty declaration replaces, in either order | Split `cmdBgpChildren` so the peer plugin stops declaring paths another package owns | The split is correct today and fragile tomorrow: the next declaration added in the rib plugin recreates the defect and nothing reports it. The floor rule removes the conflict rather than routing around it, and it is also what lets the sibling spec's plugin declaration land on a path the peer plugin has already blanked |
| A conflicting non-empty declaration panics | Record the conflict and let a test assert the record is empty | All in-tree registration runs in `init()`, so a conflict is a state only a Ze defect can reach and it is detected before the daemon serves anything. `docs/contributing/ze-go-style.md` names `panic("BUG:")` for exactly that. Recording it needs a second mechanism to hold the record and a test to read it, which is machinery for a case the panic already reports |
| Each registry states its own emptiness at construction | Constrain `T` to a slice so the registry can test `len` itself | `aliasSet` and `pipeFilterSet` are structs, so the constraint does not fit two of the five users. One predicate per registry is smaller than reshaping two value types to satisfy a constraint |
| The two cardinality-varying handlers answer rows for one peer | Declare `doc` and give up the row operators; or keep both spellings and derive the shape at apply time | Declaring `doc` loses `\| count` and `\| display` on a multi-peer router, which is the case the command exists for. Deriving at apply time is what happens today, and it is the defect: one command publishes two contracts |
| Scope is drawn at the REGISTRATION site, not the process boundary | Put every plugin-served command in the sibling spec | `show bgp rib *` and `show bgp irr *` are served by a plugin process and registered by an in-core shim, so they can declare today with no SDK change. Deferring them would hold twelve working declarations behind a wire-contract change they do not need |
| The field-name inconsistency between commands is NOT fixed here | Rename `peer` to `address` in the two peer-row commands that use it | The declaration channel names whatever field each command writes, so the inconsistency costs this spec nothing. It is a separate problem and it gets its own row, per `ai/rules/simplicity.md` |

## Known Limitations

- `show bgp peer list`, `show bgp peer detail`, `show bgp rib status` and
  `show bgp adj-rib-in` hold peer addresses as the KEYS of an identity-keyed map
  rather than as fields. `| resolve` and `| origin` act on fields, so all four
  refuse rather than resolving the addresses they are keyed by. Making an
  identity-keyed row set resolvable is a change to `applyResolve`, not a
  declaration, and it gets a deferral row.
- The field naming across `show bgp` commands stays inconsistent: a row that IS
  a peer names its address `address` in three commands and `peer` in two. A
  deferral row carries it.
- `show bgp decode` and `show bgp encode` print finished text and return an exit
  code, reaching no `ResponseData` and no operator chain. Making them answer
  structured data is a separate change to two offline handlers, and it gets a
  deferral row.
- The eleven commands under `show bgp rpki`, `show bgp rs`, `show bgp
  adj-rib-in` and `show bgp healthcheck` still declare nothing. That is
  `plan/spec-plugin-declares-answer-shape.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: nothing wire-visible changes

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-cli-show-bgp-answer-shapes.md` only
