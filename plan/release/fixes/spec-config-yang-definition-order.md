# Spec: config-yang-definition-order

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P2 |
| Release reason | Every printer of the operator's configuration (`show`, `ze config fmt`, the set form, the committed set-meta file, the web editor) follows the schema's child order, and the YANG-to-schema converter makes that order alphabetical, where the owner decided YANG definition order (D-1). It is a release fix, not post-release work, for two reasons. The shipped printer contradicts the decided printing rule and `docs/architecture/config/syntax.md`, which already states that a list's positional values follow YANG definition order while the code puts `uses` and augmented children in name order. And `plan/README.md` requires configuration syntax to stabilize before release: made after release, the change rewrites every operator's committed file and fires every archive and history digest once, on the upgrade. Made before release, it costs one reorder of files no release has shipped |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement owner decision D-1 (Thomas, 2026-10-28, the Junos model, recorded on Q-2
of `plan/post-release/engineering/spec-ze-config-fmt.md` and split out of
the closed `spec-config-list-entries-operator-order`): the leaves and
containers inside a block print in YANG DEFINITION order, not alphabetically.

| Child's origin | Position it takes |
|----------------|-------------------|
| Declared directly in the block | Its position in the YANG source |
| From a `grouping` through `uses` | The `uses` statement's position, in the grouping's own order (recursively for a `uses` inside the grouping) |
| A `choice` | The `choice` statement's position; its cases in source order; each case's data nodes in source order |
| Added by another module through `augment` | After the block's own children, ordered by module name, then by augment order within that module, then by the augment's own statement order |
| Contributed by another `-conf` module to a shared container (`Schema.Define`, `mergeContainer`) | The same rule as augment: after the first module's children, in module-name order |

Free-form and unknown-field values (no schema) stay sorted (owner decision D-3).
Editor completion keeps its alphabetical menu (see D-5 below).

The symptom today: a YANG module that declares `remote` before `local` in a
container prints `local` first in every printer, and every block reads in an order
nobody wrote.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the config syntax and printer page (`yang_schema.go` declares it as `// Design:`)
  → Constraint: it already says positional list values are "assigned positionally in YANG definition order"; the code puts `uses` and augmented children of a list in name order (`yangChildOrder` remainder). After this spec the page and the code agree, and the page gains the D-1 table for container children
  → Constraint: the list-order paragraph is rewritten by the list-order spec; this spec adds the child-order statement next to it, not a second copy of the list rule
- [ ] `docs/architecture/config/yang-config-design.md` - YANG-to-schema design (`completer.go` declares it)
  → Constraint: the page states no child order today; it gains one sentence pointing at the D-1 rule in `syntax.md` (one declaration, `ai/rules/principles.md`)
- [ ] `docs/architecture/web-components.md` - web editor fragments (`fragment.go` declares it)
  → Constraint: form field order follows `provider.Children()`; if the page states an order it is updated, otherwise unaffected
- [ ] `docs/architecture/config/syntax.md`, "What a reader receives" - delivered prerequisite
  → Decision: list entry order and the D-4 comparison rule are implemented; this spec changes no list-entry order and no comparison
- [ ] `plan/post-release/engineering/spec-ze-config-fmt.md` - parent; being edited by another agent, read only
  → Constraint: its corpus gate (`TestFormatCorpusRoundTrip`) is the idempotence proof this spec reruns, not a second gate
- [ ] `docs/contributing/ze-go-style.md` - precondition for the implementing agent's first Go edit

**Key insights:**
- Every printer already iterates the schema node's `Children()` (`serialize.go`, `serialize_set.go`, `serialize_annotated.go`, `serialize_blame.go`, `flatten.go`), and `Children()` returns the `order` slice the converter built (`Container`, `List`, `Flex`, `InlineList` constructors append `FieldDef`s in the order given). So the whole change is the order the converter hands those constructors: no printer changes.
- goyang keeps statement order: `Entry.Node.Statement().SubStatements()` is the source order and `Statement.Location()` gives file:line:col. `Entry.Dir` is a map, so the order is never read from the merged entry: it is rebuilt from the statements. What goyang keeps for each origin (vendored goyang, `pkg/yang/entry.go` and `node.go`, read 2026-11-18):
  - Direct child: its `Node` is a substatement of the parent's statement.
  - `uses` child: `Entry.Uses` is EMPTY in Ze. goyang fills it only under `ParseOptions.StoreUses`, and the loader builds its set with a bare `yang.NewModules()` (`config/yang/loader.go`, `NewLoader`). A child merged through `uses` keeps the `Node` of its statement INSIDE THE GROUPING, so `yang.RootNode(child.Node)` names the grouping's module, not the using one. Its position is recovered by walking the parent statement and resolving each `uses` substatement with the exported `yang.FindGrouping(usesNode, name, seen)`, recursively for a `uses` inside the grouping and across an import prefix. No capture is needed, and no loader option.
  - `choice`/`case` child: the `choice` and `case` statements sit in the parent's substatements; a shorthand case is a data statement directly under `choice`.
  - Augmented child: `Entry.Augment` (`entry.go`) appends one `shallowDup` of each applied augment to the TARGET's `Entry.Augmented`. Its `Node` is the `augment` statement, so the augmenting module is `yang.RootNode(aug.Node)` (its `belongs-to` module for a submodule, as goyang's own `module()` does), its position is `aug.Node.Statement().Location()`, and its `Dir` keys name the children it added; their order is the augment statement's substatements, with `uses` resolved as above. Each merged child also keeps its own `Node` and carries the augmenting module's namespace (`Entry.merge` sets `namespace`), so `Entry.InstantiatingModule()` answers the same module. The order in which goyang appends to `Augmented` follows its own processing order and is never used: D-1 sorts by module name and source position.
- The positional one-line list entry (`parseListInlineEntry`, `serializeListMultiBlock`) assigns values by `ListNode.Children()` order, so a list whose `uses` or augmented children move changes what a positional line MEANS. The PARSE accepts a positional line for every keyed list, whatever its children; only the PRINT is limited to a keyed list of at most two leaf children (`usesMultiBlockForm`, `entriesArePositional`). That is the riskiest point (A-1); the measurement of 2026-11-18 is in A-1.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang_schema.go` - YANG-to-schema converter
  → Constraint: `sortedKeys` orders a `map[string]*gyang.Entry` by name; it is the order source of `flattenChildren` (containers), `flattenChoiceCases` (cases and their children), `yangToFlex`, `yangToInlineListWithKey`, the per-module loop of `schemaFromResolved`, and `LoadYANGModule` (the top level of the ExaBGP migration schema, `exaBGPSchema` in `internal/exabgp/migration/schema.go`, whose nodes go through the same `yangToNode`)
  → Constraint (2026-11-18): `PluginOnlySchema` no longer calls `sortedKeys`. Since 2026-11-16 it is cut from `YANGSchema`: it defines the full schema's own nodes for each name of `firstPassRoots`, in that list's order, so its nodes inherit the converter's order and it needs no change. It serves the first-pass parse (`first_pass.go`) and no printer
  → Constraint (2026-11-18): every road that lowers a `yang.Resolved` to a `config.Schema` goes through `schemaFromResolved`: `YANGSchema` (through `resolvedCache`), `YANGSchemaWithPlugins` (module texts, through `loadYANGModules` and `JoinExternalModules`), `SchemaOf` (the editor completer's set), and `SchemaWithDeclaredPlugins` (`declared_schemas.go`: publishes the external modules, then answers `YANGSchema`). An external plugin's module is joined by `yang.Join` (`config/yang/join.go`, `resolveJoined`), which resolves the compiled-in texts and the external texts in ONE goyang resolve, so an external augment reaches the converter with the same `Augmented`, `Node` and statement facts as a compiled-in one. `LoadYANGModule` is the only other producer
  → Constraint: `yangChildOrder` (lists only) walks the list statement's substatements for `leaf`, `leaf-list`, `container`, `list`, `choice`, `anyxml`, then appends every other `Dir` child sorted by name: `uses` children and augmented children land there, mixed, alphabetically; an entry with no `Node` falls back to `sortedKeys` entirely
  → Constraint: `yangToList` flattens a `choice` child through `flattenChoiceCases`, so case order inside a list is alphabetical too
  → Constraint: `schemaFromResolved` sorts module names and visits only `-conf` modules; `Schema.Define` merges a same-named top-level container with `mergeContainer`, which appends the later module's new children after the existing ones. Module-name order is therefore already the merge order, which matches D-1's augment clause
- [ ] `internal/component/config/schema.go` - `Container`, `List`, `Flex`, `InlineList` append children in argument order; `ContainerNode.Children()`, `ListNode.Children()`, `FlexNode.Children()`, `InlineListNode.Children()`, `Schema.Children()` return that slice; `mergeContainer` appends
  → Constraint: no constructor sorts, so the converter is the only place order is decided
- [ ] `internal/component/config/serialize.go`, `serialize_set.go`, `serialize_annotated.go`, `serialize_blame.go`, `flatten.go` - every printer ranges `node.Children()` for leaves and containers; `serializeExtraValues` and `serializeFreeform` sort the values the schema does not name (D-3, unchanged)
  → Constraint: a `ze:flatten` container (`hasFlattenExtension`, one module uses it) prints each child as `<name> <child ...>` in `Children()` order, so its lines reorder too
- [ ] `internal/component/config/parser_list.go` - `parseListInlineEntry` assigns positional tokens by `node.Children()` index, the last child absorbing the rest
  → Constraint: a change of a list's `Children()` order changes the meaning of every positional line written for that list. `parseList` takes `key v1 v2 ...;` for EVERY keyed list; `usesMultiBlockForm` (`flatten.go`: a key, at most two children, all leaves) and `entriesArePositional` (`serialize.go`) decide only which lists PRINT that form
- [ ] `internal/component/config/parser_freeform.go` - `parseInlineList` reads fields by keyword, not position; Flex likewise
  → Constraint: Flex and InlineList field order changes print order only, never parse meaning
- [ ] `internal/component/cli/completer.go` - `getSortedChildren` builds the completion menu from goyang `effectiveChildren` and sorts it with `slices.Sort`; it does not read schema order
  → Decision: unchanged (D-5): completion is a menu of what may be typed, not a print of configuration
- [ ] `internal/component/web/fragment.go`, `handler_config_walk.go`, `page_bgp_policy.go` - the web editor builds form fields and tree rows by ranging `provider.Children()` / `schema.Children()`
  → Constraint: web form field order follows definition order after this spec with no web change; web tests asserting field order are expectations to refresh
- [ ] `internal/component/cli/diff_tree.go` - `diffTreeNode` ranges `walker.Children()`, so the annotated diff follows the new order with no change
- [ ] `internal/component/config/schema_defaults.go`, `prune.go`, `mask.go`, `secret_text.go`, `password_hash.go`, `listener.go`, `list_key.go`, `setparser.go`, `parser.go` - non-printer `Children()` consumers
  → Constraint: each must be order-independent (A-3); `list_key.go` takes key order from the YANG `key` statement, not `Children()`
  → Constraint (2026-11-18): four consumers the design-time list missed put `Children()` order into text a user reads, so they are A-3 cases too: `containerValueFix` (`parser_diagnostics.go`) writes the fix of a container refusal with its children in `Children()` order; `KeywordHint` and `UnknownKeywordHint` (`compiled_out_root.go`, `retired.go`, called from `parser.go` and `parser_list.go`) pass `Children()` to `suggest.Command`, whose tie between two equally close names follows that order; the editor's own hint (`editor_commands.go`, `lister.Children()`); and the policy registry (`internal/component/policy/registry.go`, `policySchema.Children()`), which collects the policy lists in that order. `configuredInterfaceKinds` (iface) sorts its own copy and is unaffected. No `.ci`, `.et` or Go expectation in the tree quotes a container fix listing two children (grep of 2026-11-18)
- [ ] `internal/component/gnmi/get.go` - `treeToTypedValue` marshals `ToMap` with `encoding/json` (keys sorted); unaffected
- [ ] `internal/component/plugin/cli/test_cmd.go` - ranges schema children for plugin test output; its expectations may reorder

**Behavior to preserve:**
- What every file parses to: a hierarchical or set file parses to the same tree before and after, for every non-positional statement.
- Free-form and unknown-field values print sorted (D-3).
- List entry order and the D-4 comparison rule from the dependency spec.
- Completion menus stay alphabetical (D-5).
- Formatting is idempotent: `fmt` of `fmt` output changes no byte.
- `ToMap`, `ToPluginMap`, gNMI encoding: unchanged.

**Behavior to change:**
- Container, list, Flex, InlineList, choice/case and top-level children take the D-1 positions in every printer and the web editor.
- A list's positional value order follows D-1, so it now includes `uses` and augmented children at their D-1 positions (A-1).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| List definition order | `config/yang_schema.go` `yangChildOrder` | Direct data statements of a list in source order, then every other `Dir` child (`uses`, augment) by name | Replaces: the D-1 order function generalizes it to every node and every origin, and `yangChildOrder` is deleted (no-layering) |
| Name order of the converter | `config/yang_schema.go` `sortedKeys`, `flattenChildren`, `flattenChoiceCases`; `yangToFlex`, `yangToInlineListWithKey`, `schemaFromResolved`, `LoadYANGModule` | Order every container, case, Flex and InlineList child and every module's top level by name | Replaces: each call site takes the D-1 function; `sortedKeys` goes with its last caller |
| Shared-root merge | `config/schema.go` `Schema.Define`, `mergeContainer`; modules visited in name order by `schemaFromResolved` | Appends a later `-conf` module's new children after the earlier module's | Extends: unchanged, its module-name append order already is D-1's rule for merged children |
| Schema child order as the one print order | `ContainerNode.Children()` and the other node kinds (`config/schema.go`), read by every printer, the web editor and the diff tree | Printers range `Children()` | Extends: order is decided once at schema build, no printer changes |
| Unsorted values | `serializeExtraValues`, `serializeFreeform` (`config/serialize.go`) | Sort the values no schema names | Does not apply: stays sorted (D-3) |
| Completion menu | `internal/component/cli/completer.go` `getSortedChildren` | Builds the menu from goyang and sorts it | Does not apply: stays alphabetical (D-5), pinned by AC-12 |
| The external-module join | `config/yang/join.go` `Join`, `resolveJoined`; `config/yang_schema.go` `JoinExternalModules`, `PublishExternalModules` | One goyang resolve of the compiled-in and external texts, lowered by `schemaFromResolved` | Extends: the D-1 function reads the joined set's statements like the compiled-in set's, so an external module's children take the same rule (A-4) |
| Statement position facts of goyang | vendored goyang `Entry.Node`, `Entry.Augmented`, `Statement.SubStatements`, `Statement.Location`, `yang.RootNode`, `yang.FindGrouping` | Kept on every processed entry | Extends: the order function reads them; no new capture, no loader option |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Schema build: `schemaFromResolved`, reached from `YANGSchema`, `YANGSchemaWithPlugins`, `SchemaOf` and `SchemaWithDeclaredPlugins`, over the embedded, registered and joined external plugin YANG modules; and `LoadYANGModule` for the ExaBGP migration schema. `PluginOnlySchema` cuts its nodes from `YANGSchema` and inherits their order.

### Transformation Path
1. The loader resolves the modules (or `yang.Join` resolves them with the external texts); goyang merges `uses` and `augment` into each `Entry.Dir` and keeps the statements and `Entry.Augmented`.
2. One order function, given a goyang entry, returns its data children in D-1 order (the replacement for `sortedKeys` in the converter and for `yangChildOrder`): the entry's own substatements in source order, a `uses` expanded at its position through `yang.FindGrouping`, a `choice` expanded at its position case by case; then the entries of `Entry.Augmented` sorted by (augmenting module name, augment statement position), each expanded the same way. Its sort key is made of source facts only (module name, statement position), so it is stable across module subsets and is what the build-time compiler must carry (`plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md`, OD-4).
3. `yangToContainer`, `yangToList`, `yangToFlex`, `yangToInlineListWithKey`, `schemaFromResolved` and `LoadYANGModule` pass children to the constructors in that order; `flattenChildren` and `flattenChoiceCases` become views of it or are deleted.
4. Printers, the web editor and the diff tree range `Children()` and so follow it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG source ↔ schema | goyang statements and `Entry.Uses` / `Entry.Augmented` | Yes (goyang `Entry` fields at `entry.go`, `Statement.SubStatements` and `Location` at `parse.go`) |
| Schema ↔ printers | `Children()` order | Yes (grep: every printer ranges `Children()`) |
| Schema ↔ web | `provider.Children()` | Yes (`fragment.go`, `handler_config_walk.go`) |
| Schema ↔ completion | Not crossed: completion reads goyang and sorts | Yes (`getSortedChildren`) |

### Integration Points
- `sortedKeys`, `flattenChildren`, `flattenChoiceCases`, `yangChildOrder` (`yang_schema.go`) - replaced by one D-1 order function; `sortedKeys` survives only where no schema order exists, if anywhere.
- `mergeContainer` (`schema.go`) - unchanged; its append order is already D-1's module-name order.
- `plan/release/fixes/spec-config-yang-one-definer-per-node.md` (design) - neither spec needs the other first: the D-1 function orders a merged root (first module by name, then the others' new children by module name) and an augmented one (the definer, then the augmenters by module name) under one rule. The two orders differ only where the definer that spec chooses is not the first `-conf` module by name (`interface`: `ze-cos-conf` is visited before `ze-iface-conf`), so landing this spec first means that spec's conversion reorders those roots once more, and its A-3 ("the print order of a shared root's children does not change") and its AC-4 byte comparison must expect the definer's children first. This spec lands first: it is `ready`, a release fix, and the build-time compiler needs its order.
- `plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` (OD-4) - that spec carries this order into its compiled form; this spec states the sort key it must carry (Transformation Path, step 2).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Order decided once, at schema build; printers untouched |
| No unintended coupling (components stay isolated) | Yes | The change stays in `internal/component/config/yang_schema.go` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `yangChildOrder` and the `sortedKeys` call sites are replaced by one function, not joined by a second |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Schema build only, once per module set (`yangSchemaCache`) |
| Registration over hardcoding, outbound | Yes | No new command, view or handler |
| Registration over hardcoding, inbound | Yes | Order derived from the YANG statements and module names; no module, container or leaf name appears in the order function |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Moving a list's `uses` and augmented children to their D-1 positions is acceptable for the positional one-line entry form, because ze is pre-release and the page already documents definition order | `syntax.md` "assigned positionally in YANG definition order"; `ai/rules/cli.md`: an unreleased grammar is replaced outright; committed files are set-meta, never positional | A hand-written positional line in a shipped example, test or doc is read into the wrong leaves with no error | Order, measured 2026-11-18 by a reader of the 235 shipped `.yang` files that modeled today's order and D-1's (approximate: it does not reproduce goyang's merge of the shared `interface` lists, and 8 VRRP augments into them went unplaced): 19 of 223 lists and about 305 of 570 containers change order. The 19 lists: `bgp peer`, `bgp group`, `bgp group peer`, `plugin hub server`, `policy match`, `static table route`, `service dhcp-server shared-network subnet`, `interface tunnel`, `wireguard`, `xfrm`, `pppoe-client`, and `ethernet`, `veth`, `dummy`, `bridge` with their `unit` lists (the class-of-service augment). A changed order is not a changed meaning, and meaning is what A-1 is about. Meaning, measured by the guard `TestPositionalListOrderChangesInShippedLists`: the parse accepts the positional form for every keyed list, so for each one the guard compares the slots a positional line reaches under the old rule (rebuilt in the test: the list's own data statements in source order, then every other child by name) and under `Children()` (one value to each child before the last while each takes a value, the rest or a bracket to the last child). It fails on a list whose slots differ and that `positionalMeaningChangesReviewed` does not name with its reason, and on a named list whose slots no longer differ. Its run of 2026-11-18 names 8 lists, slots before → after: `interface bridge` stp, member, description, disable → stp, member, description, mtu, os-name, disable; `interface dummy` and `interface ethernet` description, disable → description, mtu, os-name, disable; `interface veth` peer, description, disable → peer, description, mtu, os-name, disable; `interface xfrm` if-id, dev, description, disable, mtu, os-name → if-id, dev, description, mtu, os-name, disable; `interface pppoe-client` source-interface, last os-name → source-interface, last disable; `plugin hub server` secret, certificate, last port → ip, port, secret, certificate; `policy match` action, family, next-hop, interface, last tag → action, family, next-hop, interface, last route-type. What a line written for the old order does now, read from the leaf types (`parseListInlineEntry` and `setInlineLastChild` check each value against the child it lands on, through `validateInlineListChildValue`): `bridge`, `dummy`, `ethernet`, `veth` and `xfrm` refuse it, because the old `disable` value can only be `true` (type empty) and now lands on `mtu` (uint16 68..16000), as in `ethernet eth0 uplink true;`, while a line too short to reach `disable` reads the same. `pppoe-client` refuses it: only a bracket reaches its last child (`authentication`, a container, refuses a second plain value in both orders), and `disable` refuses `pppoe-client ppp0 eth0 [ wan0 ];`, which gave `os-name` the string `[ wan0 ]`. `policy match` refuses it in the schema the guard loads: only a bracket reaches its last child (`metric`, a container, refuses a fifth plain value in both orders), and the `route-type` enumeration refuses `match m1 [ 100 ];`, which gave `tag` the member 100. `plugin hub server` refuses every usable old line but one shape: the secret (32 characters at least, `minTokenLength`) now lands on `ip` (`zt:ip-address`) and the certificate name on `port` (uint16); a secret that is itself an IPv6 literal of 32 to 39 characters, with a numeric certificate or none, is accepted with another meaning: `server main 2001:0db8:0000:0000:0000:0000:0000:0001 443;` read secret `2001:0db8:0000:0000:0000:0000:0000:0001` and certificate `443`, and now reads ip `2001:0db8:0000:0000:0000:0000:0000:0001` and port 443 with no secret. `server main 10.0.0.1 1790;` parses in both orders, but its old reading (secret `10.0.0.1`) was refused at load as shorter than 32 characters. The guard sees only the modules the `config` test binary links, and that set lacks the five BGP filter plugins whose augments extend `policy match` (`ze-filter-aspath`, `ze-filter-aspath-length`, `ze-filter-community-match`, `ze-filter-path-asn`, `ze-filter-prefix`): in a build that carries them the D-1 last child is `ze-filter-prefix`'s `prefix-list`, a string leaf, so `match m1 [ 100 ];` parses as prefix-list `[ 100 ]` and commit refuses it as naming no prefix-list (`prefixListReference`). Census of 2026-11-18: a ripgrep over `test/`, `examples/`, `docs/`, `etc/`, `website/`, `internal/` testdata and Go strings, for each of the 8 lists written as its name, a key, then a token that is not `{`, found no Ze config writing a positional line for any of them, so no file in the repository changes meaning and no fixture is rewritten. The other lists of the 19 reorder without changing meaning. `bgp peer` reorders (first child `attach` before, `connection` after; last child the `update` list before, the last augmenter's by module name after, the `rpki` container today), yet its first slot is a container in both orders and its last child takes no value in either, so every positional value is refused either way and only the refusal names another child (`TestParserRefusesInlineValueOnContainer`, refreshed with the judge's approval). The 8 listener `server` lists printed positionally (`environment` `api-server grpc`, `api-server rest`, `gnmi`, `looking-glass`, `mcp`, `ssh`, `web`, and `telemetry prometheus`, each `server`) take both `ip` and `port` from `zt:listener` (`ze-types.yang`, `ip` declared before `port`), and read `ip port` under the old rule (by name) and under D-1 (grouping order) alike | unvalidated |
| A-2 | goyang gives every data child a `Node` whose module and statement position are recoverable, for direct, `uses` (including imported groupings and nested `uses`), `choice`/`case` and `augment` children | goyang `Entry.Node`, `Entry.Augmented`, `yang.RootNode`, `yang.FindGrouping`, `Statement.Location` (Key insights). Not `Entry.Uses`: Ze never sets `ParseOptions.StoreUses`, so it is empty | Some children have no position and fall back to name order | `TestEveryChildHasDefinitionPosition` walks the full schema (all registered modules) and fails on any child the order function cannot place; the fallback is then a named, tested set. A `uses` that carries its own `augment` substatement adds children the walk does not meet at the `uses` position: the test names them if any shipped module has one | unvalidated |
| A-3 | No non-printer `Children()` consumer depends on child order for correctness (defaults, validation, masking, listener detection, prune) | `schema_defaults.go` and `ApplyDefaults` are name-keyed; `prune.go` compares per child | A validation reports a different first error, or defaults apply differently | Scoped config, cli and web package tests after Phase 1; review each consumer that stops at its first match | unvalidated |
| A-4 | Two builds with different `ze_*` tags or external plugin modules, and two starts with the same external plugins, give the same relative order to the children both carry | D-1 orders augmented children by module name, then source position, a key made of the module texts alone; `schemaFromResolved` sorts module names; `mergeContainer` appends in that order; an external plugin's module is joined in the same goyang resolve (`resolveJoined`) and lowered by the same `schemaFromResolved`, and its module name takes part in the same sort. Removing a module from a set removes its children and moves no other child | The same config prints differently on two builds, and archive digests differ between them | `TestDefinitionOrderStableAcrossModuleSets`: build the schema from the full set and from a subset, and assert the shared children keep their relative order | unvalidated |
| A-5 | Augment order within one module is recoverable as the source position of each `augment` statement | `Statement.Location` carries file:line:col | Two augments of one target from one module print in an arbitrary order | Covered by `TestAugmentedChildrenByModuleThenSource` with a fixture module holding two augments of one target | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Every printed block and every committed set-meta file reorders once; every archive (`ChangeTracker.HasChanged`) and history digest (`contentDigest`) fires once on the first save after the change | Archive entry on first boot | Expected, one time; named in the commit body; AC-9 proves a second save changes no byte |
| R-2 | Go expectations and the web handler goldens change from alphabetical to definition order; few `.ci` and `.et` files do. Measured 2026-11-18 by grep: 62 `.ci` files run a config printer (`test/parse` 26, `test/ui` 17, `test/plugin` 14, `test/l2tp` 2, `test/ipsec`, `test/pppoe`, `test/runner` 1 each), and 46 `.ci`/`.et` files carry a multi-line or `(?s)` expectation (`test/plugin` 31, `test/parse` 11, `test/editor` 2, `test/policy`, `test/ui` 1 each); read one by one, none asserts the relative order of two sibling children (they pin entry order, one sub-block, a refusal or an nft rule), so the expected `.ci`/`.et` refresh is zero. The 141 `.wb` files address fields by selector, none by position. Go: 157 test files call a printer, 33 of them name a YANG-built schema (`config` 13, `cli` 10, `config/cli` 5, `web`, `bfd`, `bgp/config`, `pki`, `ospf` 1 each; the 46 `cli` printer tests reach it through the editor too); the 219 goldens under `internal/component/web/testdata/golden` are rendered over `config.YANGSchema()` (`webGoldenParts`), and those showing a config container or list reorder. RFC-tagged: of the printer-calling tests only `internal/plugins/ospf/rfc5709_key_persistence_test.go` prints config, as set lines, which are path-addressed; no RFC-tagged `.ci` runs a printer. The exact set is the red set of the Phase 1 runs | Scoped suites red after Phase 1 | Phase 2 refreshes each, and every refreshed expectation is judged by an independent `ze-test-judge` agent (owner meta decision on test changes); the commit carries one approval line per test. An expectation is refreshed only where the input's leaf order differs from the old alphabetical order |
| R-3 | A positional line changes meaning (A-1) | `TestPositionalListOrderChanges` lists a list | Rewrite every positional fixture for that list in the same phase; the `syntax.md` positional paragraph names the D-1 rule |
| R-4 | Editor completion and printed order now differ (alphabetical menu, definition-order print) | Review comment or user report | D-5: completion stays alphabetical by decision; `TestCompletionOrderUnchanged` pins it so a later change is deliberate |
| R-5 | The web editor's form fields reorder, and a `.wb` test that clicks the Nth field hits another one | `.wb` red in Phase 2 | Refresh with the judge; a test addressing a field by position is rewritten to address it by name, judged the same way |
| R-6 | Builds with different plugin sets print the same tree differently | `TestDefinitionOrderStableAcrossModuleSets` red | Module-name ordering of augmented and merged children (D-1), never registration or iteration order |
| R-7 | `choice`/`case` flattening places case children at the wrong position (alphabetical case order survives in `flattenChoiceCases`) | `TestChoiceCasesDefinitionOrder` red | The order function places the `choice` at its statement position and walks cases in source order |
| R-8 | `ze:flatten` and inline (`InlineList`, Flex) printers reorder their tokens, and the parent's corpus gate fails idempotence | `TestFormatCorpusRoundTrip` red | Both forms parse by keyword (`parseInlineList`), so only print order moves; the corpus gate must stay green unchanged |
| R-9 | The set printers (`serializeSetContainer` via `parent.Children()`, the set-meta committed file) reorder their lines, and a set-meta parse depends on line order | Set-meta round-trip test red | Set lines are path-addressed, so order is display only; AC-7 round-trips the committed file |
| R-10 | Top-level order changes (`schemaFromResolved` per-module loop) and the root block order of every file moves | `show` of a full config | Top level follows the same rule (module-name order, definition order inside a module); AC-6 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Display order of every printed configuration and the web editor; the meaning of positional one-line list entries for the lists A-1 names |
| How is it reverted? | Single commit revert of Phase 1 plus its expectation refresh; no config migration, because every non-positional statement parses to the same tree in either order |
| Who else touches this path? | The parent `spec-ze-config-fmt` (corpus gate); the closed dependency `spec-config-list-entries-operator-order` (printers, landed); `spec-config-yang-one-definer-per-node` (shared roots, Integration Points); `spec-config-yang-compiled-at-build-time` (carries this order, OD-4), whose join, `checkStructure` and staging commits touch `config/yang/` and not the converter |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config fmt` on a file using a container whose YANG declares `remote` before `local` | → | `configFmtBytes` → `Serialize` → `ContainerNode.Children()` built by the converter | `TestConfigFmtDefinitionOrder` (`internal/component/config/cli/cmd_fmt_test.go`) and `test/parse/config-fmt-definition-order.ci` |
| Editor `show` and `show | display set` | → | `Editor.WorkingContent` → `Serialize` / `SerializeSet` | `test/editor/commands/show-definition-order.et` |
| Web editor form for the same container | → | `fragment.go` → `provider.Children()` | `TestWebFormFieldsDefinitionOrder` (`internal/component/web/fragment_order_test.go`) |
| Schema build over the full registered module set | → | `schemaFromResolved` → the D-1 order function | `TestEveryChildHasDefinitionPosition` (`internal/component/config/yang_schema_order_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A fixture module whose container declares leaf `remote`, then leaf `local`, then container `timers` | `Children()` is remote, local, timers; `ze config fmt`, `show` and `show | display set` print them in that order |
| AC-2 | A container with leaf `a`, then `uses g` where grouping `g` declares `z` then `y`, then leaf `b` | Children a, z, y, b |
| AC-3 | A grouping that itself `uses` another grouping, and a grouping imported from another module | Nested and imported grouping children sit at the outer `uses` position, in source order |
| AC-4 | A container with leaf `a`, then `choice c` with case `k2` (leaf `q`) declared before case `k1` (leaf `p`), then leaf `b` | Children a, q, p, b, in containers and in lists |
| AC-5 | Container `x` in module `m-b`, augmented by module `m-z` (leaf `z1`) and module `m-a` (leaves `a2`, `a1` in that source order), and augmented twice by `m-a` | `x`'s own children first, then `m-a`'s children in augment-statement order and statement order within each augment, then `m-z`'s |
| AC-6 | Two `-conf` modules contributing children to one top-level container, and the root block | The first module (by name) keeps its definition order, the other's new children follow it; top-level blocks follow module-name order, definition order within a module |
| AC-7 | A committed set-meta file written before the change, loaded, saved, loaded and saved again | First save reorders lines to D-1 order and parses to the same tree; second save changes no byte |
| AC-8 | Free-form and unknown-field values in any block | Still printed sorted (D-3) |
| AC-9 | Every carrier in `TestFormatCorpusRoundTrip` | `fmt` of `fmt` output changes no byte; parse of old and new output gives equal trees |
| AC-10 | A list whose positional children include `uses` or augmented children (each list `TestPositionalListOrderChanges` names) | A positional line written in the new D-1 order parses into the right leaves, and its printed form round-trips |
| AC-11 | The same schema built from the full module set and from a subset missing one augmenting module | Shared children keep their relative order |
| AC-12 | Editor completion after `set bgp ` and in any container | Same alphabetical menu as before (D-5) |
| AC-13 | Web editor form for the AC-1 container | Fields remote, local, timers in that order |
| AC-14 | Every data child of the full registered schema | The order function places each one by source position; a child it cannot place fails `TestEveryChildHasDefinitionPosition` by name |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze config fmt` on a config and reads blocks in the order the YANG documents them | file → parser → `Serialize` → converter order | `test/parse/config-fmt-definition-order.ci` |
| 2 | Opens the editor, runs `show` and `show | display set` | editor → `Serialize` / `SerializeSet` | `test/editor/commands/show-definition-order.et` |
| 3 | Upgrades, commits once, commits again | set-meta file → parse → save | `test/editor/workflow/commit-definition-order-stable.et` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestContainerChildrenDefinitionOrder` | `internal/component/config/yang_schema_order_test.go` | AC-1 | |
| `TestUsesChildrenAtUsesPosition` | `internal/component/config/yang_schema_order_test.go` | AC-2, AC-3 | |
| `TestChoiceCasesDefinitionOrder` | `internal/component/config/yang_schema_order_test.go` | AC-4 for containers and lists | |
| `TestAugmentedChildrenByModuleThenSource` | `internal/component/config/yang_schema_order_test.go` | AC-5 | |
| `TestMergedTopLevelDefinitionOrder` | `internal/component/config/yang_schema_order_test.go` | AC-6 | |
| `TestDefinitionOrderStableAcrossModuleSets` | `internal/component/config/yang_schema_order_test.go` | AC-11 | |
| `TestEveryChildHasDefinitionPosition` | `internal/component/config/yang_schema_order_test.go` | AC-14 | |
| `TestPositionalListOrderChangesRoundTrip` | `internal/component/config/yang_schema_order_test.go` | AC-10 round trip over a fixture list | |
| `TestPositionalListOrderChangesInFullBuild` | `internal/component/plugin/all/positional_list_order_test.go` | A-1: names, with its before and after slots, every list of the full build whose positional meaning changed | |
| `TestSetMetaResaveStable` | `internal/component/config/serialize_set_order_test.go` | AC-7 | |
| `TestFreeformStaysSorted` | `internal/component/config/serialize_set_order_test.go` | AC-8 | |
| `TestConfigFmtDefinitionOrder` | `internal/component/config/cli/cmd_fmt_test.go` | AC-1 through the command | |
| `TestFormatCorpusRoundTrip` (rerun, unchanged) | `internal/component/config/cli/fmt_corpus_test.go` | AC-9 | |
| `TestCompletionOrderUnchanged` | `internal/component/cli/completer_order_test.go` | AC-12 | |
| `TestWebFormFieldsDefinitionOrder` | `internal/component/web/fragment_order_test.go` | AC-13 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-fmt-definition-order` | `test/parse/config-fmt-definition-order.ci` | AC-1, AC-9 through the `ze config fmt` binary | |
| `show-definition-order` | `test/editor/commands/show-definition-order.et` | AC-1, AC-12 | |
| `commit-definition-order-stable` | `test/editor/workflow/commit-definition-order-stable.et` | AC-7 | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change; config printing only.

## Files to Modify
- `internal/component/config/yang_schema.go` - one D-1 order function replaces `yangChildOrder` and the converter's `sortedKeys` calls in `flattenChildren`, `flattenChoiceCases`, `yangToFlex`, `yangToInlineListWithKey`, `schemaFromResolved` and `LoadYANGModule`; `sortedKeys` is deleted, since no caller with no schema order remains. `PluginOnlySchema` needs no change (Current Behavior). The order function may live in a file of its own beside it (`yang_schema_order.go`), with the same `// Design:` header
- `docs/architecture/config/syntax.md` - the D-1 child-order table next to the list-order paragraph; the positional paragraph names the D-1 rule
- `docs/architecture/config/yang-config-design.md` - one sentence pointing at the rule in `syntax.md`. The build-time compiler spec is editing this page in the same branch: the sentence is added after that edit lands, never over it
- `docs/guide/configuration.md` - any statement or example of printed order; the "Bind order" row is about entries, not children, and is checked, not assumed
- Expectations: every `.et`, `.ci`, `.wb` and Go golden test whose printed leaf order changes, and every positional fixture A-1 names, each judged (R-2)

## Files to Create
- `internal/component/config/yang_schema_order_test.go` (with fixture YANG modules under `internal/component/config/testdata/yang-order/`)
- `internal/component/config/serialize_set_order_test.go`
- `internal/component/cli/completer_order_test.go`
- `internal/component/web/fragment_order_test.go`
- `test/parse/config-fmt-definition-order.ci`
- `test/editor/commands/show-definition-order.et`
- `test/editor/workflow/commit-definition-order-stable.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No module changes; the converter's child order changes. Test fixture modules live in `testdata` only |
| YANG validation constraints | No | No leaf added |
| YANG custom validators | No | None |
| CLI commands/flags | No | No command added |
| CLI grammar (keyword before value) | No | No grammar change; the positional value order is documented config syntax (A-1), not command grammar |
| Editor autocomplete | Yes | Deliberately unchanged (D-5); `TestCompletionOrderUnchanged` pins it |
| Functional test for new RPC/API | No | No RPC; functional tests above cover the printers |
| Pipe completeness | No | `show` pipes unchanged |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | No runtime dependency |
| Prometheus counters/metrics | No | None |
| BGP family surface (new SAFI / capability / attribute) | No | Not BGP family work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Display order of an existing feature |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`: printed child order, and the positional value order for the lists A-1 names |
| 3 | CLI command added/changed? | No | Output order only |
| 4 | API/RPC added/changed? | No | gNMI and `ToMap` unchanged |
| 5 | Plugin added/changed? | No | None; external plugin modules follow the same order rule |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, `docs/guide/config-editor.md`: examples of `show` output are checked against the new order |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | `ToPluginMap` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | Fixture modules only |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/fixes/spec-config-yang-definition-order.md` at the start of Phase 1 and update every page it names; the run at design time named `docs/architecture/core-design.md` (mentions `yang_schema.go`): checked for a child-order statement and edited if one exists |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every `docs/` example of printed config with two or more leaves in one block: reorder it to what the printer now prints, verified by running `ze config fmt` over it |

## Implementation Steps

Each work package lands as one commit that compiles and stands on its own (`ai/META-DECISIONS.md`, "Ze is pre-release"): a red the package causes and does not repair is named in its commit body, and a refreshed expectation is never committed without its `ze-test-judge` approval line. Each is sized at about 60 tool calls. A subagent edits and lists the commands; the main thread runs them, one at a time.

0. **WP-0: Baseline, then the order tests (no commit of its own).** The main thread runs the scoped package tests of `./internal/component/config/`, `./internal/component/config/cli/`, `./internal/component/cli/`, `./internal/component/web/`, `./internal/component/plugin/cli/`, `./internal/exabgp/migration/` and the `parse`, `editor` and `web` functional suites on HEAD and keeps each result, so a later red is known to be this spec's. The agent writes `yang_schema_order_test.go` with the fixture modules under `testdata/yang-order/` (AC-1 to AC-6, AC-11, AC-14) and `TestPositionalListOrderChanges`; they run red for the stated reason (name order) before WP-1 and ride in WP-1's commit.
1. **WP-1: One order function (commit 1).** The D-1 function over goyang statements (Transformation Path, step 2), wired into every converter call site; `yangChildOrder`, `flattenChildren`'s name sort, `flattenChoiceCases`'s name sort and `sortedKeys` deleted; `syntax.md` gains the D-1 table and its positional paragraph names the rule, in the same commit (`ai/rules/documentation.md`). The red Go expectations of `./internal/component/config/` are refreshed in this commit when the judge approves them, so the package it changes is green; the reds of other packages are named in the body and are WP-2's.
   - Tests: AC-1 to AC-6, AC-11, AC-14; A-2, A-4, A-5 marked validated or broken with evidence
   - Built 2026-11-18 (uncommitted, owed runs not yet made), WP-0 and WP-1 together: `definitionOrder` in `config/yang_schema_order.go` (with `augmentsInOrder`, `augmentKeyOf`, `statementPosition`); `sortedKeys`, `flattenChildren`, `flattenChoiceCases` and `yangChildOrder` deleted from `yang_schema.go`; `yangToContainer`, `yangToList`, `yangToFlex`, `yangToInlineListWithKey`, `schemaFromResolved` and `LoadYANGModule` call it. A child the walk cannot place is a schema build error naming its path, not a name-order fallback, so no name sort survives on a schema path; `LoadYANGModule` now resets and answers the build errors and `exaBGPSchema` reads them. Tests in `config/yang_schema_order_test.go` over seven fixture modules in `testdata/yang-order/`: `TestContainerChildrenDefinitionOrder`, `TestUsesChildrenAtUsesPosition`, `TestChoiceCasesDefinitionOrder`, `TestAugmentedChildrenByModuleThenSource`, `TestMergedTopLevelDefinitionOrder`, `TestDefinitionOrderStableAcrossModuleSets`, `TestEveryChildHasDefinitionPosition`, `TestDefinitionPositionRefusesUnplacedChild`, `TestPositionalListOrderChangesRoundTrip`, `TestPositionalListOrderChangesInShippedLists`. `syntax.md` gains "Child order" and its positional paragraphs name it (the `peer A "x"` example's first child is now `connection`). A-2, A-4, A-5 stay `unvalidated` until the owed runs
2. **WP-2: Go expectations and web goldens (commit 2, or one per package when a judge refuses one).** Rerun the scoped packages of WP-0; refresh each red expectation whose input order differs from name order; regenerate the web handler goldens; every refreshed file goes to an independent `ze-test-judge` agent, and the commit carries one approval line per test. `TestCompletionOrderUnchanged` (AC-12) and `TestWebFormFieldsDefinitionOrder` (AC-13) are written here.
   - Tests: AC-12, AC-13; A-3 marked validated or broken
3. **WP-3: The commands, the files and the docs (commit 3).** `TestConfigFmtDefinitionOrder`, `TestSetMetaResaveStable`, `TestFreeformStaysSorted`, the three functional tests, any positional fixture for a list `TestPositionalListOrderChangesInFullBuild` names (none: the census of 2026-11-18 in A-1 and in "Built so far" found no Ze config writing a positional line for any of its 9 lists), the `docs/` examples (Documentation row 17), `docs/guide/configuration.md` and `docs/guide/config-editor.md`, and the sentence in `yang-config-design.md`.
   - Tests: AC-7, AC-8, AC-9, AC-10; A-1 marked validated or broken
4. **Close-out.** Corpus round trip green, doc anchors run (`./le spec citation anchors spec plan/release/fixes/spec-config-yang-definition-order.md`), Critical Review, then `/ze-close`.

Owed commands, run by the main thread (the form is `docs/contributing/running-commands.md`, "A bare `go test` is not `./le test unit`"):

| WP | Command | What it proves |
|----|---------|----------------|
| 0, 1 | `./le job run label le-test quiet command ./le test unit pkg path ./internal/component/config/ run 'DefinitionOrder\|DefinitionPosition\|UsesChildrenAtUsesPosition\|ChoiceCases\|AugmentedChildren\|MergedTopLevel\|PositionalListOrderChanges'` | Red before WP-1 (name order), green after |
| 1 | `./le job run label le-test quiet command ./le test unit pkg path ./internal/component/config/` and the same for `./internal/exabgp/migration/` | The converter package and the ExaBGP schema, whose `LoadYANGModule` changed |
| 1 | `./le go lint run scope "./internal/component/config/"` | The changed-code lint |
| 2 | `./le job run label le-test quiet command ./le test unit pkg path ./internal/component/config/cli/`, then `./internal/component/cli/`, `./internal/component/web/`, `./internal/component/plugin/cli/` | The reds to refresh, then green after the judged refresh |
| 2 | The web golden regeneration: the `-update-golden` flag of `internal/test/golden` on the `./internal/component/web/` package, through `./le job run label unit-pkg quiet command go test -race -tags "<the feature tags>" ./internal/component/web/ -run TestWebHandlerGoldenOutput -update-golden`, the long form the page names for a flag `./le test unit pkg` does not take | The goldens' new content, which the judge reads |
| 3 | `./le test functional parse`, `./le test functional editor`, `./le test functional web` | The three functional tests and the unchanged `.ci`, `.et`, `.wb` corpus |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | All three user stories pass end to end |
| Correctness | No `sortedKeys` (or any name sort) remains on a schema child-order path in `yang_schema.go`; grep the file |
| Correctness | The order function names no module, container or leaf |
| Correctness | Every positional-eligible list whose order changed is named in the spec and every fixture using it was rewritten (A-1) |
| Correctness | Two module sets give the same relative order (AC-11) |
| Data flow | No printer changed: `git diff` touches no `serialize*.go` or `flatten.go` |
| Rule: no-layering | `yangChildOrder` is deleted, not kept beside the new function |
| Rule: test changes | Every refreshed expectation carries a `ze-test-judge` approval line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| D-1 order at schema build | `TestEveryChildHasDefinitionPosition` and the AC unit tests pass |
| Printed order | `test/parse/config-fmt-definition-order.ci` passes |
| Stable re-save | `test/editor/workflow/commit-definition-order-stable.et` passes |
| Docs | `syntax.md` states the D-1 table; `yang-config-design.md` points at it |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None added; a positional line for an A-1 list must still be type-checked per leaf after the reorder |
| Secret exposure | Masked printers keep masking; one AC-1 case prints a sensitive leaf through `MaskSecrets` |

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

## Built so far (2026-11-18)

Evidence named here comes from the commit message of `0290d55afe` ("config: a block's children follow YANG definition order") unless a row says "written, run owed": those were written after it, uncommitted, and their runs are listed as owed below.

### By work package
| WP | Built | Evidence | Remains |
|----|-------|----------|---------|
| WP-0 | The order tests and the fixture modules under `testdata/yang-order/` | Observed red with the old converter: the eight order tests, the positional guard, the judged parser test | The baseline runs of the WP-0 package list are not recorded in the commit |
| WP-1 | `definitionOrder` and its helpers; `sortedKeys`, `flattenChildren`, `flattenChoiceCases`, `yangChildOrder` deleted; `LoadYANGModule` returns its build errors; a choice inside a Flex, an InlineList or at a module's top level is no longer dropped; `syntax.md` "Child order"; `yang-config-design.md` repointed from `flattenChoiceCases` | `./le test unit pkg` green on `internal/component/config` and `internal/exabgp/migration` | Nothing in the package. The config-package positional guard it shipped sees only the modules the config tests link, so the full build gets a second guard (next row) |
| WP-1 follow-up | Two positional guards over two module sets, both kept. `TestPositionalListOrderChangesInShippedLists` (config package) covers the schema WITHOUT the `ze-cos-conf` merge and the BGP filter augments, which is what a build that drops those gated features carries (`ze-stripped`, `ze-stripped-flow`): there the interface lists' own children and the end of `route-source-conditions` decide the positional slots. `TestPositionalListOrderChangesInFullBuild` (`internal/component/plugin/all/positional_list_order_test.go`, built under `ze_bgp && ze_cos`) covers the full build: its previous-order model merges every module that defines a list, in module-name order, as `Schema.Define` does, it asks the parser which children take a value (a probe of `refuseInlineSlot`) instead of copying `inlineSlotTakesValue`, and its table pins each list's slots, not only its name. Neither set's table answers for the other, so neither guard replaces the other | Full-build guard written, run owed. The independent `ze-test-judge` refused removing the config-package guard, so it stays committed beside the new one | The full-build guard's run |
| WP-2 | The three web handler goldens and the judged Go expectations (`TestFileModeLoadTreeMergeMatchesPrevious`, `TestFileModeLoadAbsoluteMergeNestedBlocks`, `TestParserRefusesInlineValueOnContainer`); `TestCompletionOrderUnchanged` and `TestWebFormFieldsDefinitionOrder` written | Goldens green; `TestFileModeLoad*` green in `internal/component/cli`. The two new tests: written, run owed | Whole-package runs of `config/cli`, `cli`, `web`, `plugin/cli`, and the judged refresh of any red they show. Known reds at the base commit `8fb9a9295e`, not this spec's: the blank optional form field (`TestAdminExecuteAppendsPostedArguments`, `TestWebHandlerGoldenOutput/post-config-inspect*`, `TestBrowserSuppliedInspectionPasteReachesItsOwner`) and `TestRepositoryConfigsValidate` (18 findings) |
| WP-3 | `TestConfigFmtDefinitionOrder` (`config/cli/cmd_fmt_test.go`), `TestSetMetaResaveStable` and `TestFreeformStaysSorted` (`config/serialize_set_order_test.go`); the `yang-config-design.md` sentence pointing at "Child order", and its completion section naming the alphabetical menu | Written, run owed | The draft runs, stress runs and promotion of the three functional tests (next row); the doc anchors run |
| WP-3 functional | Drafted under `ze-test`, not yet promoted: `test/draft/parse/config-fmt-definition-order.ci` (a file written in name order; `ze config fmt` prints `bgp` router-id, session, peer, the peer's connection, session, filter, then the augmented `rpki` (ze-rpki), `system` host before domain; `-w` then `--check` exits 0, and a fourth run keeps the order), `test/draft/editor/commands/show-definition-order.et` (the same file in the editor: `show` and `show \| format config` in that order; the `set` menu in `bgp` keeps peer, router-id, session), `test/draft/editor/workflow/commit-definition-order-stable.et` (a set-meta file in name order, committed twice: the stored `file/active/test.conf` takes router-id, session, peer, connection, session, filter and host, domain on the first save, keeps every statement, and keeps the order on the second). Two editor expectations they need, added to the harness (`internal/component/cli/testing/expect.go`) with their unit tests (`expect_order_test.go`) and rows in `docs/architecture/testing/ci-format.md`: `completion:order=` (relative order of named menu items; `contains`, `excludes` and `exact` judge membership only) and `key:path=...:pattern=` (a regular expression over one stored value, refused beside `glob`) | Written, run owed | Draft runs, stress runs, promotion to `test/parse/`, `test/editor/commands/`, `test/editor/workflow/` |
| WP-3 docs census | Row 17 searched 2026-11-18: every fenced block in `docs/**/*.md` holding a `{` block whose three preceding lines name output (prints, displays, output, shows, produces, writes, returns, result, saved, formatted, `ze config fmt`/`dump`/`show`, `` `show` ``) was read, 15 non-code blocks; every line introducing output (`prints:`, `displays:`, `output:`, `shows:`, `becomes:` and the like, `ze config fmt`, `ze config dump`, `show \| format config`, `display set`) was read; `docs/guide/config-editor.md` and `docs/guide/configuration.md` hold no printed `show` or `fmt` output. Every config block found is an example of what an operator writes, a protocol trace, or JSON | No page presents a config block as Ze's output in an order the change made wrong, so no page changed | Nothing |

### By acceptance criterion
| AC | Test | State |
|----|------|-------|
| AC-1 | `TestContainerChildrenDefinitionOrder` (Serialize); `TestConfigFmtDefinitionOrder` (the command) | First green at `0290d55afe`; second written, run owed. `show` and `show \| format config` (the editor's set form): `show-definition-order.et` drafted, run owed; the command end to end: `config-fmt-definition-order.ci` drafted, run owed |
| AC-2, AC-3 | `TestUsesChildrenAtUsesPosition` | Green |
| AC-4 | `TestChoiceCasesDefinitionOrder` | Green |
| AC-5 | `TestAugmentedChildrenByModuleThenSource` | Green |
| AC-6 | `TestMergedTopLevelDefinitionOrder`, `TestMergedTopLevelDefinitionOrderThroughInclude` | Green |
| AC-7 | `TestSetMetaResaveStable` | Written, run owed; `commit-definition-order-stable.et` drafted, run owed (order and statements of the stored file over two saves; byte equality is the unit test's) |
| AC-8 | `TestFreeformStaysSorted` | Written, run owed |
| AC-9 | `TestFormatCorpusRoundTrip` (unchanged) | Not run since `0290d55afe` (the commit names no `config/cli` package run); `config-fmt-definition-order.ci` drafted (`-w` then `--check` exit 0), run owed |
| AC-10 | `TestPositionalListOrderChangesRoundTrip`; `TestPositionalListOrderChangesInShippedLists` (config package, the schema without the `ze-cos-conf` merge and the BGP filter augments); `TestPositionalListOrderChangesInFullBuild` (the full build) | First two green; third written, run owed |
| AC-11 | `TestDefinitionOrderStableAcrossModuleSets` | Green |
| AC-12 | `TestCompletionOrderUnchanged`; `show-definition-order.et` (`completion:order=peer,router-id,session`) | Both written, runs owed |
| AC-13 | `TestWebFormFieldsDefinitionOrder` (leaves remote, local, bind of a module joined through `YANGSchemaWithPlugins`; the container `timers` is not a form field) | Written, run owed |
| AC-14 | `TestEveryChildHasDefinitionPosition`, `TestDefinitionPositionRefusesUnplacedChild` | Green over the config test binary's modules. Over the full registered set the schema build is exercised by `TestPositionalListOrderChangesInFullBuild`, which fails on a child the order cannot place: run owed |

### Assumptions
| ID | Now | By what |
|----|-----|---------|
| A-1 | Measured for the full build, not yet run | The full-build reading of the YANG (below) names 9 lists; no Ze config in the repository writes a positional line for any of them. It is confirmed when `TestPositionalListOrderChangesInFullBuild` passes with that table |
| A-2 | Confirmed for the config test binary's modules; full set pending | `TestEveryChildHasDefinitionPosition` green; no module those tests link has a child the walk cannot place |
| A-3 | Open | Only the config package ran whole; `cli`, `web`, `config/cli`, `plugin/cli` ran in part (above) |
| A-4 | Confirmed | `TestDefinitionOrderStableAcrossModuleSets` green |
| A-5 | Confirmed | `TestAugmentedChildrenByModuleThenSource` green, two augments of one target from one module |

### The full build's positional table (read from the YANG, 2026-11-18)
In a full build `ze-cos-conf` defines `interface ethernet`, `dummy`, `veth` and `bridge` too and is merged before `ze-iface-conf` (module-name order), so those lists start with the CoS module's children; the five BGP filter plugins augment `policy match`; and `static table route`, whose module the config tests never linked, carries a choice. Slots before → after, as the test spells them:

| List | Before → after | An old-order line now |
|------|----------------|-----------------------|
| `interface bridge`, `dummy`, `ethernet`, `veth` | `last:os-name` → `class-of-service` | Refused: before, every plain value landed on `unit` (a list) and only a bracket parsed, given to `os-name`; the bracket now lands on `offload`, a container. A one-value line refused before now sets `class-of-service` |
| `interface pppoe-client` | `source-interface last:os-name` → `source-interface last:disable` | Refused (as in A-1) |
| `interface xfrm` | `if-id dev description disable mtu os-name` → `if-id dev description mtu os-name disable` | Refused (as in A-1) |
| `plugin hub server` | `secret certificate last:port` → `ip port secret certificate` | Refused but for the IPv6-literal secret, accepted with another meaning (as in A-1) |
| `policy match` | `action family next-hop interface last:tag` → `action family next-hop interface last:prefix-list` | Accepted with another meaning by the parse (`match m1 [ 100 ];` gave `tag` the member 100, now gives `prefix-list` the text `[ 100 ]`), refused at commit as naming no prefix-list (`prefixListReference`) |
| `static table route` | `description metric distance tag blackhole last:reject` → `description metric distance tag last:reject` | Refused: the fifth plain value set the presence container `blackhole` (cases expanded by name) and now lands on `next`, a container |

A build without `ze_cos` reads the four interface lists as the config-package table did (`stp member description disable` style slots of the interface module alone); a build without `ze_bgp` has no filter augments. The guard runs only where both tags are built, which the default test tag set is. Census for the lists the full build adds or changes (the four interface lists, `policy match`, `static table route`): ripgrep over `test/`, `examples/`, `docs/`, `etc/`, `website/`, `internal/` testdata and Go strings for the list name, a key, then a token that is not `{`: no Ze config hit. The hits were set-form paths (`set interface ethernet eth9 unit 0 ...`), the CLI command `ze interface create veth ze0 ze1`, and other daemons' configs (VyOS, FRR, BIRD, the RSVP-TE lab's route files).

## Design Insights

| ID | Design point | Finding | Recommendation | Decided by |
|----|--------------|---------|----------------|------------|
| D-1 | Child order | Converter sorts by name; only list children partly follow definition order | YANG definition order per the Task table | Thomas, 2026-10-28 |
| D-3 | Free-form and unknown-field values | No schema order exists for them | Stay sorted | Thomas, 2026-10-28 |
| D-5 | Completion order | `getSortedChildren` sorts the menu from goyang and never read schema order; D-1 decides printing | Keep the alphabetical menu; an operator scans a menu for a name | Derived from D-1's scope (printing); pinned by AC-12 so a change is deliberate |
| D-6 | Positional value order | The positional one-line form reads values by `Children()` index, so D-1 moves values for lists with `uses` or augmented children | One order for print and parse, as `syntax.md` already documents; the affected fixtures are rewritten (A-1) | Derived from D-1 and `ai/rules/cli.md` (an unreleased grammar is replaced outright) |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Owner decision D-1 (Thomas, 2026-10-28): leaves and containers in a block print in YANG definition order; `uses` children at the `uses` position in grouping order; augmented children after the block's own, by module name, then augment order within a module | Alphabetical, as today | The Junos model; module-name ordering keeps two builds from printing one tree differently |
| Owner decision D-3 (Thomas, 2026-10-28): free-form and unknown-field values stay sorted | Recorded order | No schema order exists |
| Owner decision D-4 (Thomas, 2026-10-28): a reorder of a plain list is no change, of an `ordered-by user` list a change | - | Owned by the dependency spec; this spec changes no comparison |
| Order decided once, at schema build | Sort in each printer | Every printer already follows `Children()`; one place means no printer can disagree |
| Children a module contributes to a shared container follow the augment rule | A separate rule for `mergeContainer` | It is the same situation (another module adds children) and `mergeContainer` already appends in module-name order |

## Known Limitations
- Editor completion stays alphabetical (D-5).
- gNMI Get and `ToMap` JSON stay key-sorted by `encoding/json`; they are not configuration renderings.
- The ExaBGP migration printer (`internal/exabgp/migration/migrate_serialize.go`) is hand-written and keeps its own value order.

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
- [ ] The targeted tests for what changed pass: the scoped package tests, and the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command; the full verification is Thomas's release check and GitHub CI's
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-yang-definition-order.md` only, in the same `./le commit create` script

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-11-18 |
| Checked at | `0bd72d74ab` (branch `worktree-yang-compiled`, 9 commits past main's `8fb9a9295e`) |
| Verdict | CURRENT |
| Goal holds | Thomas confirmed D-1 on 2026-11-18 ("Yes, I keep the order of the definition") |
| Not built | `sortedKeys` and `yangChildOrder` still in `config/yang_schema.go`; none of the spec's new tests or files exists; `TestFormatCorpusRoundTrip` exists (`config/cli/fmt_corpus_test.go`) |
| Targets | Every function the spec names exists and behaves as stated, except `PluginOnlySchema`, cut from `YANGSchema` since 2026-11-16 with no `sortedKeys` of its own |
| New since the baseline | The external-module join (`yang.Join`, `JoinExternalModules`, `PublishExternalModules`, `SchemaWithDeclaredPlugins`) lowers through `schemaFromResolved`, so it adds no producer; `LoadYANGModule` (ExaBGP migration schema) was missed at design time |
| Facts corrected | `Entry.Uses` is empty in Ze (no `StoreUses`); the positional parse reaches every keyed list; four more `Children()` consumers write user-visible text; R-2 and A-1 measured |
| Nothing undone | No removal by `git log -S` of an order function; the design replaces `yangChildOrder` rather than adding beside it |

The goal is the owner's decision, reconfirmed today, and the shipped converter still sorts by name, so the defect occurs at HEAD (`flattenChildren`, `yangChildOrder`). The design (order decided once, at schema build, from goyang's statements) fits today's tree better than it did on 2026-10-28: the external-plugin work joins every external module in one goyang resolve and lowers it through the same `schemaFromResolved`, so one order function covers compiled-in and external modules alike and A-4 needs no new mechanism. The spec had no Existing Mechanisms table; it was added from the mechanisms its own Integration Points already extended or replaced, which is the analysis the table records, so the verdict is CURRENT and not REWORK. The stale references were repointed above without changing what the spec asks for: `PluginOnlySchema` left the call-site list, `LoadYANGModule` joined it, the `Entry.Uses` claim was replaced by the `FindGrouping` route, the refresh and positional measurements replaced the estimates, the `./le verify changed` gate was removed (`ai/META-DECISIONS.md`: no agent runs it), and the phases became work packages that each land as a commit. What would overturn the verdict: a shipped module whose children goyang does not let the order function place (A-2, `TestEveryChildHasDefinitionPosition`), or an owner ruling that the one-definer conversion must land first.
