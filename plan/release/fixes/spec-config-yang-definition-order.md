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
| Updated | 2026-10-28 |

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
- goyang keeps statement order: `Entry.Node.Statement().SubStatements()` is the source order, `Statement.Location()` gives file:line:col, `Entry.Uses` holds each `uses` with its grouping `Entry`, `Entry.Augmented` holds the augments merged in, and `yang.RootNode(child.Node)` names the module a child was declared in.
- The positional one-line list entry (`parseListInlineEntry`, `serializeListMultiBlock`) assigns values by `ListNode.Children()` order, so a list whose `uses` or augmented children move changes what a positional line MEANS. That is the riskiest point (A-1).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang_schema.go` - YANG-to-schema converter
  → Constraint: `sortedKeys` orders a `map[string]*gyang.Entry` by name; it is the order source of `flattenChildren` (containers), `flattenChoiceCases` (cases and their children), `yangToFlex`, `yangToInlineListWithKey`, `PluginOnlySchema` and the per-module loop of `schemaFromResolved`
  → Constraint: `yangChildOrder` (lists only) walks the list statement's substatements for `leaf`, `leaf-list`, `container`, `list`, `choice`, `anyxml`, then appends every other `Dir` child sorted by name: `uses` children and augmented children land there, mixed, alphabetically; an entry with no `Node` falls back to `sortedKeys` entirely
  → Constraint: `yangToList` flattens a `choice` child through `flattenChoiceCases`, so case order inside a list is alphabetical too
  → Constraint: `schemaFromResolved` sorts module names and visits only `-conf` modules; `Schema.Define` merges a same-named top-level container with `mergeContainer`, which appends the later module's new children after the existing ones. Module-name order is therefore already the merge order, which matches D-1's augment clause
- [ ] `internal/component/config/schema.go` - `Container`, `List`, `Flex`, `InlineList` append children in argument order; `ContainerNode.Children()`, `ListNode.Children()`, `FlexNode.Children()`, `InlineListNode.Children()`, `Schema.Children()` return that slice; `mergeContainer` appends
  → Constraint: no constructor sorts, so the converter is the only place order is decided
- [ ] `internal/component/config/serialize.go`, `serialize_set.go`, `serialize_annotated.go`, `serialize_blame.go`, `flatten.go` - every printer ranges `node.Children()` for leaves and containers; `serializeExtraValues` and `serializeFreeform` sort the values the schema does not name (D-3, unchanged)
  → Constraint: a `ze:flatten` container (`hasFlattenExtension`, one module uses it) prints each child as `<name> <child ...>` in `Children()` order, so its lines reorder too
- [ ] `internal/component/config/parser_list.go` - `parseListInlineEntry` assigns positional tokens by `node.Children()` index, the last child absorbing the rest
  → Constraint: a change of a list's `Children()` order changes the meaning of every positional line written for that list; `entriesArePositional` and `allChildrenAreLeaves` (`serialize.go`) decide which lists print that form
- [ ] `internal/component/config/parser_freeform.go` - `parseInlineList` reads fields by keyword, not position; Flex likewise
  → Constraint: Flex and InlineList field order changes print order only, never parse meaning
- [ ] `internal/component/cli/completer.go` - `getSortedChildren` builds the completion menu from goyang `effectiveChildren` and sorts it with `slices.Sort`; it does not read schema order
  → Decision: unchanged (D-5): completion is a menu of what may be typed, not a print of configuration
- [ ] `internal/component/web/fragment.go`, `handler_config_walk.go`, `page_bgp_policy.go` - the web editor builds form fields and tree rows by ranging `provider.Children()` / `schema.Children()`
  → Constraint: web form field order follows definition order after this spec with no web change; web tests asserting field order are expectations to refresh
- [ ] `internal/component/cli/diff_tree.go` - `diffTreeNode` ranges `walker.Children()`, so the annotated diff follows the new order with no change
- [ ] `internal/component/config/schema_defaults.go`, `prune.go`, `mask.go`, `secret_text.go`, `password_hash.go`, `listener.go`, `list_key.go`, `setparser.go`, `parser.go` - non-printer `Children()` consumers
  → Constraint: each must be order-independent (A-3); `list_key.go` takes key order from the YANG `key` statement, not `Children()`
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

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Schema build: `YANGSchema`, `YANGSchemaWithPlugins`, `PluginOnlySchema`, over the embedded, registered and external plugin YANG modules.

### Transformation Path
1. The loader resolves the modules; goyang merges `uses` and `augment` into each `Entry.Dir` and keeps the statements.
2. One order function, given a goyang entry, returns its data children in D-1 order (the replacement for `sortedKeys` in the converter and for `yangChildOrder`).
3. `yangToContainer`, `yangToList`, `yangToFlex`, `yangToInlineListWithKey`, `flattenChildren`, `flattenChoiceCases`, `PluginOnlySchema` and `schemaFromResolved` pass children to the constructors in that order.
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
| A-1 | Moving a list's `uses` and augmented children to their D-1 positions is acceptable for the positional one-line entry form, because ze is pre-release and the page already documents definition order | `syntax.md` "assigned positionally in YANG definition order"; `ai/rules/cli.md`: an unreleased grammar is replaced outright; committed files are set-meta, never positional | A hand-written positional line in a shipped example, test or doc is read into the wrong leaves with no error | Phase 0 lists every list for which `allChildrenAreLeaves` holds and whose `Children()` order changes; every positional line in `test/`, `docs/`, `etc/` and the `.ci` corpus for those lists is found and rewritten in the same phase; `TestPositionalListOrderChanges` pins the list | unvalidated |
| A-2 | goyang gives every data child a `Node` whose module and statement position are recoverable, for direct, `uses` (including imported groupings and nested `uses`), `choice`/`case` and `augment` children | goyang `Entry.Node`, `Entry.Uses`, `Entry.Augmented`, `yang.RootNode`, `Statement.Location` | Some children have no position and fall back to name order | `TestEveryChildHasDefinitionPosition` walks the full schema (all registered modules) and fails on any child the order function cannot place; the fallback is then a named, tested set | unvalidated |
| A-3 | No non-printer `Children()` consumer depends on child order for correctness (defaults, validation, masking, listener detection, prune) | `schema_defaults.go` and `ApplyDefaults` are name-keyed; `prune.go` compares per child | A validation reports a different first error, or defaults apply differently | Scoped config, cli and web package tests after Phase 1; review each consumer that stops at its first match | unvalidated |
| A-4 | Two builds with different `ze_*` tags or external plugin modules give the same relative order to the children both carry | D-1 orders augmented children by module name; `schemaFromResolved` sorts module names; `mergeContainer` appends in that order | The same config prints differently on two builds, and archive digests differ between them | `TestDefinitionOrderStableAcrossModuleSets`: build the schema from the full set and from a subset, and assert the shared children keep their relative order | unvalidated |
| A-5 | Augment order within one module is recoverable as the source position of each `augment` statement | `Statement.Location` carries file:line:col | Two augments of one target from one module print in an arbitrary order | Covered by `TestAugmentedChildrenByModuleThenSource` with a fixture module holding two augments of one target | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Every printed block and every committed set-meta file reorders once; every archive (`ChangeTracker.HasChanged`) and history digest (`contentDigest`) fires once on the first save after the change | Archive entry on first boot | Expected, one time; named in the commit body; AC-9 proves a second save changes no byte |
| R-2 | Many `.et`, `.ci`, `.wb` and Go golden expectations change from alphabetical to definition order | Scoped suites red after Phase 1 | Phase 2 refreshes each, and every refreshed expectation is judged by an independent `ze-test-judge` agent (owner meta decision on test changes); the commit carries one approval line per test. An expectation is refreshed only where the input's leaf order differs from the old alphabetical order |
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
| Who else touches this path? | The parent `spec-ze-config-fmt` (corpus gate); the dependency `spec-config-list-entries-operator-order` (printers, lands first) |

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
| `TestPositionalListOrderChanges` | `internal/component/config/yang_schema_order_test.go` | A-1: names every positional-eligible list whose order changed, and AC-10 round trip for each | |
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
- `internal/component/config/yang_schema.go` - one D-1 order function replaces `yangChildOrder` and the converter's `sortedKeys` calls in `flattenChildren`, `flattenChoiceCases`, `yangToFlex`, `yangToInlineListWithKey`, `PluginOnlySchema` and `schemaFromResolved`; `sortedKeys` is deleted unless a caller with no schema order remains
- `docs/architecture/config/syntax.md` - the D-1 child-order table next to the list-order paragraph; the positional paragraph names the D-1 rule
- `docs/architecture/config/yang-config-design.md` - one sentence pointing at the rule in `syntax.md`
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

Each phase is sized at about 60 tool calls.

0. **Phase: Measure and wire** -- run the scoped config, cli and web package tests and the `test/parse` and `test/editor` suites on HEAD and keep the result; write the four Wiring Test rows, `TestEveryChildHasDefinitionPosition` and `TestPositionalListOrderChanges` with the fixture modules; record in the spec the lists A-1 names and the positional fixtures that use them.
   - Verify: the order tests fail for the stated reason (alphabetical order)
1. **Phase: One order function** -- the D-1 function over goyang statements (direct, `uses` recursive, `choice`/`case`, augment by module then source position), wired into every converter call site; `yangChildOrder` and the converter's `sortedKeys` uses deleted; docs `syntax.md` and `yang-config-design.md` updated in the same phase.
   - Tests: AC-1 to AC-6, AC-11, AC-14
   - Verify: unit tests pass; A-2, A-4 and A-5 marked validated or broken with evidence
2. **Phase: Expectations and positional fixtures** -- rerun the scoped suites; refresh each red expectation whose input order differs from alphabetical, send every refreshed expectation to an independent `ze-test-judge` agent, and rewrite each positional fixture A-1 named; `docs/` examples reordered (row 17).
   - Tests: AC-7, AC-8, AC-9, AC-10, AC-12, AC-13
   - Verify: every refresh carries its approval line; A-1 and A-3 marked validated or broken
3. **Phase: Close-out** -- corpus round trip green, doc anchors run, Critical Review.

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits. The full verification is Thomas's release check
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
