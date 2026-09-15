# Spec: both-help-texts-reach-every-surface

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli \| config \| docs \| tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every YANG node that an operator reads declares two texts: `ze:help` is the
one-line summary and `description` is the long explanation (owner directive,
2026-09-15, the swap commit that precedes this spec). Three surfaces read both:
the command tree (`mergeYANGEntry`), the RPC summary (`ExtractRPCs`) and the
interactive `?` box (`entryLongHelp`). Every other surface was built with one
text field and copies one string, so the explanation an author writes on a
command argument, an RPC leaf, a notification leaf, a config node in the web
editor, a config node in the published configuration reference, or an enum
value never reaches a reader. The owner wants both texts on every UI and every
documentation surface, so that the 299 enum values and the 692 nodes that today
declare one text can be written and READ.

Goal: each carrier below holds the pair, each reader renders it, and a text
written on any node reaches the page or the screen that names that node.

| Carrier today | Holds | Readers that lose the explanation |
|---------------|-------|-----------------------------------|
| `command.ArgDef` | name, kind, ranges, mandatory: no text at all | `ze help command --json` args, the site command catalog `args`, the wikicatalog `args`, the MCP tool input schema, the web admin command form |
| `yang.LeafMeta` (rpc input, rpc output, notification leaves) | the summary only | `api.ParamMeta`, `api.CommandSchema`, the gRPC schema, `ze help ai --json`, the plugin schema listing |
| `config.LeafNode`, `config.ContainerNode`, `config.ListNode` | one `Description` | the web config editor tooltip, `handler_config_leaf.go`, `fragment.go` |
| `cli.AnalysisNode` (`show yang tree`) | one `Description`, no enum values | the site configuration reference page, its markdown mirror and `llms.txt` |
| enum value completion | the literal `enum value` | every config leaf whose type is an enumeration, except a list key |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - "CLI Help from YANG" names the three carriers and their readers
  → Decision: `GetHelpExtension` is the ONE reader of the `ze:help` statement, and the explanation is the goyang entry's `Description`. A new carrier reads through those two, never through a third reader
  → Constraint: `mergeHelpText` decides each field on its own; a new field on a merged node follows the same three cases
- [ ] `docs/architecture/api/commands.md` - "A leaf's own `ze:help` and `description` reach no surface" is the sentence this spec makes false
  → Decision: `argDefFor` gains the two texts; `ArgDef` gains `ShortHelp` and `Description`
- [ ] `docs/architecture/web-interface.md` - the admin form reads the two texts of the command node it shows
  → Constraint: the config editor form reads a schema node, so the pair lands on `config.LeafNode` and its siblings, not on the handler
- [ ] `docs/contributing/documentation-testing.md` - `./le docvalid help-shape` holds the four corpora to one shape
  → Constraint: the gate counts a node with no `ze:help` as `missing-summary`; the enum row says an enum carries a summary alone and nothing reads its `description`. That row changes when completion reads the summary
- [ ] `ai/rules/cli.md` - every command answers structured data, and `| json` renders it
  → Constraint: a JSON key is kebab-case: `short-help`, `description`, `values`
- [ ] `ai/rules/principles.md` - a central enumeration is recognized by what it answers for a feature it does not name
  → Constraint: the completer's `enum value` literal is a placeholder beside a registry that holds the fact; the fix reads the declaration, never a table

**Key insights:**
- The YANG boundary is two calls: `GetHelpExtension(exts)` for the summary and `entry.Description` for the explanation. Every carrier fills both from those two.
- A JSON Schema has `title` for a short text and `description` for a long one. The API and MCP schemas map `ShortHelp` to `title` and `Description` to `description`.
- The completer resolves an enum's own statements only for a list key (`getListKeyEntry`); the same walk serves every enum leaf.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/command.go` - `argDefFor` reads type and mandatory; `mergeYANGEntry` fills the command node's pair; `GetHelpExtension` reads the summary
- [ ] `internal/component/command/node.go` - `ArgDef` carries no text field
- [ ] `internal/component/config/yang/rpc.go` - `ExtractRPCs` fills `RPCMeta` with the pair and `LeafMeta` with the summary only
- [ ] `internal/component/api/types.go` - `ParamMeta` carries one `Description`; `cmd/ze/hub/api.go` copies it from `LeafMeta`
- [ ] `internal/component/api/schema.go` - `CommandSchema` writes one JSON Schema `description` per property
- [ ] `internal/component/config/yang_schema.go` - `LeafNode`, `ContainerNode`, `ListNode` copy one text at three sites
- [ ] `internal/component/web/handler_config_leaf.go` - `buildLeafField` puts the summary in the `title` tooltip
- [ ] `internal/component/config/yang/cli/tree.go` - `AnalysisNode` copies one text and no enum values
- [ ] `internal/le/site/config.go` - `configurationMirror` and `writeConfigChildMirror` print the one text; `llmsdata.go` `configNode` decodes it
- [ ] `internal/le/site/catalog.go` - `catalogArg` has name, type, values, mandatory
- [ ] `internal/le/wikicatalog/catalog.go` - `extractArgs` copies `ArgDef` without text
- [ ] `internal/component/cli/completer.go` - the `Yenum` branch of value completion writes `enum value`; `getListKeyEntry` reads an enum's declared summary
- [ ] `internal/component/mcp/tools.go` - the tool input schema properties are built from `ArgDef`
- [ ] `internal/le/docvalid/helpshape_schema.go` - `schemaEnums` judges a list-key enum's summary only

**Behavior to preserve:**
- Every JSON key that exists today keeps its name and its meaning after the rename commit (`short-help` summary, `description` explanation).
- The command tree merge cases, the interactive `?` box and Tab box, the help page layout.
- `./le docvalid help-shape` refusals on the four corpora.

**Behavior to change:**
- Each carrier in the Task table gains the missing text (or both), each reader renders it, and enum value completion shows the declared summary.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A YANG module's `ze:help` and `description` statements on a data node, an rpc leaf, a notification leaf, or an enum value.
- Format at entry: goyang `*Entry` (`Exts`, `Description`) and, for an enum value, the AST `*yang.Enum` (`Extensions`, `Description`).

### Transformation Path
1. YANG boundary: `GetHelpExtension` and `entry.Description` in `command.go`, `rpc.go`, `yang_schema.go`, `cli/tree.go`, `completer.go`.
2. Carrier structs: `ArgDef`, `LeafMeta`, `LeafNode`/`ContainerNode`/`ListNode`, `AnalysisNode`, `contract.Completion`.
3. Readers: `ze help command --json`, `ze help ai --json`, `api.CommandSchema`, the gRPC schema, MCP tool schemas, web admin and config forms, `show yang tree --config | json`, the site build, the wikicatalog.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon to site build | `show yang tree --config \| json` (`runYANGConfigTree`) | Yes: `TestConfigurationReferencePrintsExplanation`, `test/ui/yang-tree-json-both-texts.ci` |
| Daemon to API client | `ze help ai --json`, `CommandSchema` | Yes: `TestAPICommandListerCarriesBothLeafTexts`, `TestCommandSchemaWritesTitleAndDescription`, `test/ui/help-ai-json-rpc-leaf-texts.ci` |
| Hub to MCP client | tool `inputSchema` | Yes: `TestBuildParamMetaCarriesBothLeafTexts`, `test/plugin/mcp-tools-list-argument-texts.ci` |
| Hub to browser | templ views | Yes: `TestConfigLeafFormRendersBothTexts`, `TestConfigContainerAndListRenderTheExplanation`, `test/web/admin-command-posts-argument.wb` |

### Integration Points
- `argDefFor` - fills the two new `ArgDef` fields from the leaf entry.
- `ExtractRPCs` - fills `LeafMeta.Description` from the leaf entry.
- `buildLeafField` - renders the explanation under the input, the summary in the tooltip.
- `writeConfigChildMirror` - prints the explanation as a paragraph under the node line.
- value completion `Yenum` branch - reads the declared summary through the same walk `getListKeyEntry` uses.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every carrier reads `GetHelpExtension` and `entry.Description` (`argDefFor`, `extractEntryLeaves`, `walkYANGEntry`); enum values read `yang.EnumValueSummaries` and `yang.EnumValueNames` (`enum.go`) |
| No unintended coupling (components stay isolated) | Yes | the web reads `config.LeafNode`; the site reads the daemon's JSON; MCP reads `ParamInfo` through the hub (`buildParamMeta`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | round 2 #7 folded the three enum walks into `EnumValueNames`; `enumKeyVocabulary` reads `EnumValueSummaries` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | texts are strings copied by reference; the completer caches the summary map per entry (`Completer.enumValueSummaries`) |
| Registration over hardcoding, outbound | Yes | no surface names a module or a node; the `enum value` literal is gone (`valueCompletions`) |
| Registration over hardcoding, inbound | Yes | the help-shape gate judges every enumeration leaf `collectSchema` walks, not a list (`schemaEnums`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The rename commit lands first: `ShortHelp` is the summary and `Description` the explanation on every downstream struct | owner directive 2026-09-15 | this spec's field names are wrong | `gopls references` on `command.Node.ShortHelp` before implementation | confirmed |
| A-2 | The goyang AST `*yang.Enum` keeps its `Extensions` after the loader resolves the module, with the keyword `ze:help` or `<prefix>:help` | `getListKeyEntry` reads it today | enum summaries cannot be read from the resolved tree | `TestEnumValueCompletionShowsDeclaredSummary` | confirmed |
| A-3 | JSON Schema `title` is read by MCP clients as the short label | JSON Schema core vocabulary | clients show only `description` | inspection of one MCP client, recorded in Design Insights | broken |
| A-4 | A leaf under an rpc `input` reaches `LeafMeta` through `extractLeaves` for every rpc the hub serves | `rpc.go` | some API leaves keep one text | `TestExtractRPCsCarriesBothLeafTexts` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A published JSON artifact gains a key a drift fixture does not model | `./le docvalid` catalog drift, `catalogfields_test.go` | model the key in `site/catalog.go` and the docvalid fixture in the same phase |
| R-2 | The site build's `configNode` decoder ignores the new keys and the page silently prints nothing | the configuration reference page unchanged after the build | the site test asserts the explanation of one node appears in the page and the mirror |
| R-3 | The web editor renders a multi-line explanation inside a `title` attribute | the tooltip shows raw newlines | the explanation is a block element under the input, the tooltip keeps the summary |
| R-4 | A merged command node carries `ArgDef` texts from two modules that disagree | `YANG command help text mismatch` log | `argDefFor` runs on the declaring module's leaf; a merged node keeps the first text, as `mergeHelpText` does |
| R-5 | The completer's enum branch allocates per keystroke by walking the AST | `go test -bench` on completion | resolve the enum summaries once at load, on the entry, as `getListKeyEntry` does |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A help page, a JSON envelope or a documentation page shows a wrong or empty text. No route, session or config value is touched |
| How is it reverted? | single commit revert |
| Who else touches this path? | the rename commit that precedes this spec; `plan/immediate/spec-yang-rpc-declarations-with-no-handler.md` reads `RPCMeta` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze help command --json` on a command whose argument leaf declares both texts | → | `argDefFor`, `ArgDef.ShortHelp`, `ArgDef.Description` | `test/ui/help-command-json-argument-texts.ci` |
| `ze help ai --json` on an rpc whose input leaf declares both texts | → | `ExtractRPCs`, `LeafMeta.Description`, `ParamMeta.Description` | `test/ui/help-ai-json-rpc-leaf-texts.ci` |
| `show yang tree --config \| json` on a leaf with both texts and an enum type | → | `AnalysisNode.ShortHelp`, `AnalysisNode.Description`, `AnalysisNode.Values` | `test/ui/yang-tree-json-both-texts.ci` |
| Tab on an enum config leaf in `ze cli` | → | value completion `Yenum` branch reading the declared summary | `test/ui/completion-enum-value-summary.ci` |
| The web config editor form for a leaf | → | `buildLeafField`, `config_leaf_input.templ` | `TestConfigLeafFormRendersBothTexts` in `internal/component/web` |
| The site build | → | `writeConfigChildMirror`, `configNode` | `TestConfigurationReferencePrintsExplanation` in `internal/le/site` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `-cmd` module leaf declares `ze:help` and `description` | `ze help command --json` prints the argument with `short-help` and `description`; the site catalog and the wikicatalog carry the same two keys; the web admin form prints both beside the input |
| AC-2 | An rpc input, rpc output or notification leaf declares both texts | `ze help ai --json` prints both on the leaf; `CommandSchema` writes `title` (summary) and `description` (explanation) on the property; the gRPC schema carries both |
| AC-3 | A config leaf, container or list declares both texts | the web config editor shows the summary in the tooltip and the explanation as a block under the input; `show yang tree --config \| json` prints `short-help` and `description` |
| AC-4 | A config leaf of type enumeration declares a `ze:help` on each value | `show yang tree --config \| json` prints `values` with `name` and `short-help`; the site configuration reference lists the values with their summaries; Tab in `ze cli` shows the declared summary beside each value instead of `enum value` |
| AC-5 | A node declares one text only | every surface prints the one it has and omits the other key; nothing derives one from the other |
| AC-6 | The MCP tool for a command with an argument that declares both texts | the tool `inputSchema` property carries `title` and `description` |
| AC-7 | `./le docvalid help-shape` on the tree | the enum corpus now judges every enum value the completer renders, not only list keys; the report names the count |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads the published configuration reference for `bgp/as-notation` | YANG -> `AnalysisNode` -> `show yang tree` JSON -> `configurationMirror` -> page | `TestConfigurationReferencePrintsExplanation` |
| 2 | Presses Tab after `set bgp as-notation ` in `ze cli` | YANG -> completer `Yenum` branch -> menu row | `test/ui/completion-enum-value-summary.ci` |
| 3 | Opens a leaf in the web config editor | YANG -> `LeafNode` -> `buildLeafField` -> templ | `TestConfigLeafFormRendersBothTexts` |
| 4 | Asks an MCP client what an argument means | YANG -> `ArgDef` -> tool `inputSchema` | `TestToolInputSchemaCarriesArgumentTexts` |
| 5 | Runs `ze help ai --json` | YANG -> `LeafMeta` -> `ParamMeta` -> JSON | `test/ui/help-ai-json-rpc-leaf-texts.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestArgDefForCarriesBothTexts` | `internal/component/config/yang/command_test.go` | `argDefFor` fills `ShortHelp` and `Description` from the leaf | |
| `TestExtractRPCsCarriesBothLeafTexts` | `internal/component/config/yang/rpc_test.go` | input, output and notification leaves carry both | |
| `TestSchemaNodesCarryBothTexts` | `internal/component/config/yang_schema_test.go` | leaf, container and list carry both | |
| `TestAnalysisTreeCarriesBothTextsAndValues` | `internal/component/config/yang/cli/tree_test.go` | `AnalysisNode` carries both and enum values with summaries | |
| `TestEnumValueCompletionShowsDeclaredSummary` | `internal/component/cli/completer_test.go` | the menu row carries the declared summary; a value with none says so | |
| `TestCommandSchemaWritesTitleAndDescription` | `internal/component/api/schema_test.go` | JSON Schema property carries both | |
| `TestToolInputSchemaCarriesArgumentTexts` | `internal/component/mcp/tools_test.go` | MCP property carries both | |
| `TestConfigLeafFormRendersBothTexts` | `internal/component/web/handler_config_leaf_test.go` | the tooltip is the summary and the block is the explanation | |
| `TestConfigurationReferencePrintsExplanation` | `internal/le/site/config_test.go` | page, mirror and llms carry the explanation and the enum values | |
| `TestCatalogArgCarriesBothTexts` | `internal/le/wikicatalog/catalog_test.go` | `args` entries carry both keys | |
| `TestHelpShapeJudgesEveryRenderedEnum` | `internal/le/docvalid/helpshape_schema_test.go` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `help-command-json-argument-texts` | `test/ui/help-command-json-argument-texts.ci` | an operator reads what an argument means from the JSON help | |
| `help-ai-json-rpc-leaf-texts` | `test/ui/help-ai-json-rpc-leaf-texts.ci` | an agent reads what an rpc leaf means | |
| `yang-tree-json-both-texts` | `test/ui/yang-tree-json-both-texts.ci` | the site build reads both texts and enum values | |
| `completion-enum-value-summary` | `test/ui/completion-enum-value-summary.ci` | Tab on an enum leaf shows each value's summary | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no wire-visible change | |

## Files to Modify
- `internal/component/command/node.go` - `ArgDef` gains `ShortHelp` and `Description`
- `internal/component/config/yang/command.go` - `argDefFor` fills them
- `internal/component/config/yang/rpc.go` - `LeafMeta` gains `Description`; `ExtractRPCs` and the notification extractor fill it
- `internal/component/api/types.go`, `internal/component/api/schema.go`, `internal/component/api/grpc/convert.go`, `cmd/ze/hub/api.go` - `ParamMeta` carries both; the schema writes `title` and `description`
- `internal/component/config/yang_schema.go` - `LeafNode`, `ContainerNode`, `ListNode` gain `ShortHelp`
- `internal/component/web/handler_config_leaf.go`, `internal/component/web/config_leaf_input.templ`, `internal/component/web/fragment.go`, `internal/component/web/handler_admin.go` and its templ - render both
- `internal/component/config/yang/cli/tree.go` - `AnalysisNode` gains `ShortHelp` and `Values`
- `internal/le/site/config.go`, `internal/le/site/llmsdata.go`, `internal/le/site/catalog.go` - decode and print both; `catalogArg` gains both
- `internal/le/wikicatalog/catalog.go` - `Argument` gains both
- `internal/component/cli/completer.go` - enum value completion reads the declared summary
- `internal/component/mcp/tools.go` - property `title` and `description`
- `internal/plugins/meta/cmd/help.go` - `args` carry both keys
- `internal/le/docvalid/helpshape_schema.go` - every rendered enum value is judged
- `docs/architecture/api/commands.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/web-interface.md`, `docs/contributing/documentation-testing.md`, `docs/guide/api.md`, `docs/guide/mcp/overview.md` - the pages that state which surface reads which text

## Files to Create
- `test/ui/help-command-json-argument-texts.ci`
- `test/ui/help-ai-json-rpc-leaf-texts.ci`
- `test/ui/yang-tree-json-both-texts.ci`
- `test/ui/completion-enum-value-summary.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new node; the texts already exist in the modules |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no new command; existing JSON envelopes gain keys |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | Yes | `internal/component/cli/completer.go`, enum value rows |
| Functional test for new RPC/API | Yes | the four `.ci` files above |
| Pipe completeness | Yes | the new keys ride the existing structured payloads, so `\| json`, `\| yaml`, `\| table` render them without a change |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` Self-Documenting System row: both texts on every surface |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if it shows `help command --json` output |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: the argument paragraph and `ArgDef` |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md`, `docs/guide/api.md`, `docs/guide/mcp/overview.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | the plugin registration keys were renamed by the preceding commit, not here |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` "CLI Help from YANG", `docs/architecture/web-interface.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/api/architecture.md` (declared by `api/types.go`, `api/schema.go`, `hub/api.go`: the `ParamMeta` and schema paragraphs), `docs/architecture/config/syntax.md` (declared by `yang_schema.go`: unaffected, the node texts are not syntax), `website/AI.md` (declared by `site/catalog.go`: the catalog `args` keys). Run `./le spec citation anchors spec plan/pre-release/spec-both-help-texts-reach-every-surface.md` at implementation start for the rest |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/api.md` JSON examples gain the keys |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the four `.ci` tests and the six unit wiring tests exist and fail
   - Tests: the Wiring Test table
   - Files: the four `.ci` files, the test files named in the Unit Tests table
   - Verify: each fails because the key or the field is absent
2. **Phase: carriers** -- add the fields and fill them at the YANG boundary
   - Tests: `TestArgDefForCarriesBothTexts`, `TestExtractRPCsCarriesBothLeafTexts`, `TestSchemaNodesCarryBothTexts`, `TestAnalysisTreeCarriesBothTextsAndValues`
   - Files: `node.go`, `command.go`, `rpc.go`, `yang_schema.go`, `cli/tree.go`
3. **Phase: readers** -- JSON envelopes, schemas, MCP, web, site, wikicatalog, completion
   - Tests: the rest of the Unit Tests table, the four `.ci` files
   - Files: the remaining Files to Modify
4. **Phase: gate and pages** -- `helpshape_schema.go` judges every rendered enum value; the pages in the Documentation Update Checklist state the new readers
   - Tests: `TestHelpShapeJudgesEveryRenderedEnum`
   - Verify: `./le docvalid help-shape` reports the enum count; `./le doc check verify` is green on the touched pages

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | no surface derives one text from the other; an absent text stays empty |
| Naming | JSON keys `short-help`, `description`, `values`; JSON Schema `title`, `description` |
| Data flow | the YANG boundary is `GetHelpExtension` and `entry.Description` only; no third reader |
| Rule: `ai/rules/principles.md` | the `enum value` literal is gone and nothing else prints a placeholder where a declaration exists |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `ArgDef.ShortHelp`, `ArgDef.Description` | `gopls references` shows the producers and the six readers |
| the four `.ci` tests | `./le` functional route on `test/ui/` |
| the enum completion row | `test/ui/completion-enum-value-summary.ci` |
| the pages | `./le doc check verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the texts are authored in the repository's own modules; the web views escape them as every other text |

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

- `ze help ai --json` published an rpc as `wire-method`, `short-help`, `description` and no leaves at all, and no notifications. AC-2 owes both texts on each leaf, so `aihelp.RPC` gains `input` and `output` (`aihelp.Leaf`: `name`, `type`, `mandatory`, `short-help`, `description`) and `aihelp.Reference` gains `notifications` (`wire-method`, `short-help`, `leaves`), all `omitempty`.
  → Decision: the JSON reference lists the leaves; the text mode of `ze help ai` keeps its one-line-per-leaf listing with the summary, because a paragraph does not fit a listing row.
- `internal/plugins/meta/cmd/help.go` `args` is the usage-hint STRING a plugin sends in its `CommandDecl.Args` (`internal/component/plugin/server/command_registry.go`, `CommandDecl.Args`), never a list of `ArgDef`. The Files to Modify row read it as the `ArgDef` list.
  → Decision: `command help` keeps its `args` string; the argument texts reach the five readers AC-1 names (`ze help command --json`, the site catalog, the wikicatalog, the MCP schema, the web admin form). No key was added to `command help`.
- A-3 is broken. The MCP specification (2025-06-18, "Data Types / Tool") defines `title` for the tool only, every property example carries `description` alone, and no client documents reading a property `title`; the schema is handed to the model as JSON Schema, where `title` is the short label.
  → Decision: `addYANGParams` keeps `title` for the summary and `description` for the explanation. A `description`-only client reads the explanation and loses the summary; joining the two into `description` would derive one text from the other, which AC-5 forbids.
- MCP `ParamInfo` is filled from `yang.LeafMeta` through the hub (`cmd/ze/hub/command_meta.go`, `commandParam`), not from `ArgDef`, so AC-6 rides the `LeafMeta.Description` carrier, and `TestToolInputSchemaCarriesArgumentTexts` drives `allTools` with `ParamInfo`.
  → Decision: the JSON keys on `args`, `input`, `output` and `leaves` are `short-help,omitempty` and `description,omitempty`, the convention every sibling text key on those envelopes already follows (`aihelp.RPC`, `catalogCommand.ShortHelp`, `commandEntry.ShortHelp`). AC-5 is met by never deriving one text from the other; an undeclared text is absent rather than an empty string, as the RPC `description` already is (`docs/guide/mcp/overview.md`).
- The gRPC `ParamInfo` proto carried one `description` holding the summary. It gains `short_help = 5` (`json_name = "short-help"`) and `description` becomes the explanation, and `CommandInfo` gains the same field so the two messages agree. Regenerated with `./le setup proto-generate`.
  → Decision: the wiki argument table gains `Summary` and `Description` columns (`internal/le/wikicatalog/render.go`), and `internal/le/docvalid/command_surfaces.go` expects the six-column header and row, so the wiki and its validator change in one diff. The golden `internal/le/wikicatalog/testdata/multi.md` follows.
- The web admin command form never filled `CommandFormData.Parameters`, so it rendered no argument inputs at all. `commandFormParameters` (`internal/component/web/handler_admin.go`) now lists the node's `ArgDefs` with both texts, and `component_command_form.templ` prints the summary beside the input and the explanation under it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Every carrier holds the pair as two fields | one field holding "summary. explanation" joined | a reader that needs one text would have to split a string; the YANG declares two, so the carrier does |
| JSON Schema maps summary to `title`, explanation to `description` | the explanation alone in `description` | JSON Schema defines `title` as the short label; both texts reach the client and neither is dropped |
| Enum value summaries are resolved from the AST once at load | walking the AST on every completion | completion runs on every keystroke; `getListKeyEntry` already pays the walk once |
| `AnalysisNode` gains `Values` with name and summary | a separate values command | the site reads one JSON tree; a second command is a second artifact to keep fresh |

## Known Limitations
- The explanation of a `-cmd` verb container that another module owns is filled at merge and stays a single text per merged node, as today.

## Implementation Notes (command and API side)

| AC | Producer | Proof (RED, then GREEN) |
|----|----------|-------------------------|
| AC-1 | `argDefFor` (`internal/component/config/yang/command.go`) fills `ArgDef.ShortHelp` from `GetHelpExtension(leaf.Exts)` and `ArgDef.Description` from `leaf.Description`; `extractArgs` (`cmd/ze/help_command.go`), `catalogArg` (`internal/le/site/catalog.go`) and `extractArgs` (`internal/le/wikicatalog/catalog.go`) copy both; `commandFormParameters` (`internal/component/web/handler_admin.go`) lists them for `component_command_form.templ` | `TestArgDefForCarriesBothTexts`: RED `port.ShortHelp undefined (type command.ArgDef has no field or method ShortHelp)`, GREEN `job yang-green: exit 0`. `TestCatalogArgCarriesBothTexts`: RED `the wiki argument row carries neither text`, GREEN `job wiki2: exit 0`. `TestCommandFormPrintsArgumentTexts`: RED `Parameters[0].ShortHelp undefined`, GREEN `job web-green: exit 0`. `test/ui/help-command-json-argument-texts.ci` |
| AC-2 | `extractEntryLeaves` (`internal/component/config/yang/rpc.go`) fills `LeafMeta.Description` from `child.Description`; `apiCommandLister` (`cmd/ze/hub/api.go`) copies to `api.ParamMeta.Description`; `CommandSchema` (`internal/component/api/schema.go`) writes `title` and `description`; `commandMetaToProto` (`internal/component/api/grpc/convert.go`) fills `ParamInfo.ShortHelp` and `Description`; `aihelp.Build` lists `input`, `output` and `notifications` leaves | `TestExtractRPCsCarriesBothLeafTexts`: RED `rpcs[0].Input[0].Description undefined`, GREEN `job yang-green: exit 0`. `TestCommandSchemaWritesTitleAndDescription`: RED `[build failed]` on `ParamMeta.Description`, GREEN `job api-green` api package ok. `TestCommandMetaToProto` (extended) GREEN. `test/ui/help-ai-json-rpc-leaf-texts.ci` |
| AC-5 | every producer above copies each text on its own; no producer reads one to fill the other | the `label` and `owner` cases of `TestCommandSchemaWritesTitleAndDescription` and `TestToolInputSchemaCarriesArgumentTexts`; the `label` leaf of `TestArgDefForCarriesBothTexts`; the notification leaf of `TestExtractRPCsCarriesBothLeafTexts` |
| AC-6 | `addYANGParams` (`internal/component/mcp/tools.go`) writes `title` from `ParamInfo.ShortHelp` and `description` from `ParamInfo.Description`; `cmd/ze/hub/service_mcp.go` and `command_meta.go` carry `Description` from `LeafMeta` | `TestToolInputSchemaCarriesArgumentTexts`: RED `[build failed]` on `ParamInfo.Description`, GREEN `job mcp-green3: exit 0` |

## Implementation Notes (config side)

→ Decision: one reader for an enum value's `ze:help`, `yang.EnumValueSummaries` (`internal/component/config/yang/enum.go`), reads the value's own statements off the parse-tree type statements, a union member by member and a typedef through the chain goyang resolved (the decision below). The completer (`valueCompletions`, `listKeyCompletions`), the analysis tree (`yangEnumValues`) and the help-shape gate (`schemaEnums`) all read through it; `enumKeyVocabulary` lost its own walk.
→ Decision: the completer caches the summaries per leaf entry (`Completer.enumValueSummaries`, `enumSummaries` map under a mutex, bounded by the enumeration leaves the model declares) so a keystroke never walks the AST.
→ Decision: a typedef-borne enumeration resolves through goyang's own resolver rather than a walk of Ze's own: `Type.resolve` (`vendor/github.com/openconfig/goyang/pkg/yang/types.go`) leaves the referenced typedef's type statement at `YangType.Base`, so `collectEnumSummaries` (`enum.go`) follows `Base` from the leaf's statement through every typedef to the builtin, whose `Base` is nil, and reads the `ze:help` off each statement's `enum` list and each union member's on the way. Scope (module, grouping, imported prefix) is what goyang already resolved, so the reader looks nothing up by name. RED `EnumValueSummaries(settings/mode) = map[], want map[active:Open the session passive:Wait for the peer]` (job enum-typedef-red, five subtests), GREEN job enum-typedef-green exit 0; help-shape after: 813 rendered, 813 with a summary, no refusal.
→ Decision: `show yang tree | json` spells the summary `short-help` and the explanation `description`; the `description` key used to carry the summary, and every reader (`internal/le/site/llmsdata.go` `configNode`, `configscript.go`) moved with it. The site's markdown mirror prints the summary on the node line after a colon, the explanation as the line under it, and each enum value as `- \`name\`: summary`; the published fixtures (`published-configuration.md`, `published-configuration-body.html`) carry only the old `description` key, which now renders as an explanation line and so stayed byte-identical, and the new keys are proven by `TestConfigurationReferencePrintsExplanation` over a restated `bgp`.
→ Constraint: the web goldens `internal/component/web/testdata/handler/get-fragment-detail.txt` and `get-fragment-detail-page.txt` were recaptured with `-update-golden`: the workbench detail now carries `<p class="ze-field-help">` under `bgp/as-notation`, which is AC-3's rendering.
→ Constraint: `TestHelpShapeIgnoresAnEnumThatKeysNoList` asserted the behavior AC-7 removes and was replaced by `TestHelpShapeJudgesEveryRenderedEnum`; the fixture's `sockets/state/open` summary is now in cap and the test injects the over-cap one.

| AC | Producer | Proof (RED, then GREEN) |
|----|----------|-------------------------|
| AC-3 | `buildLeafField` (`internal/component/web/handler_config_leaf.go`) and `buildFieldMeta` (`fragment.go`) copy `LeafNode.Description`; `leafInput` (`config_leaf_input.templ`) and `fieldWrapper` (`input_wrapper.templ`) render it as a block, the tooltip keeps `ShortHelp`; `walkYANGEntry` (`internal/component/config/yang/cli/tree.go`) fills `AnalysisNode.Description`, `formatTreeJSON` (`format.go`) writes `short-help` and `description` | `TestConfigLeafFormRendersBothTexts`: RED `field.Description undefined (type LeafField has no field or method Description)`, GREEN `job web-green: exit 0`. `TestAnalysisTreeCarriesBothTextsAndValues`: RED `notation.Description undefined (type *AnalysisNode has no field or method Description)`, GREEN `job enum-green: exit 0`. `TestSchemaNodesCarryBothTexts` held at first run (the nodes carry both since b57ec4ab6b) and stays as the proof. `test/ui/yang-tree-json-both-texts.ci` |
| AC-4 | `yangEnumValues` fills `AnalysisNode.Values`, `formatTreeJSON` writes `values[].name` and `values[].short-help`; `writeConfigChildMirror` and `configscript.go` `valuesFor` (`internal/le/site/`) list them; `valueCompletions` `Yenum` and `Yunion` branches (`internal/component/cli/completer.go`) put `enumValueSummaries(entry)[name]` on the row, the `enum value` literal is gone | `TestEnumValueCompletionShowsDeclaredSummary`: RED `c.enumValueSummaries undefined`, GREEN `job enum-green: exit 0`. `TestConfigurationReferencePrintsExplanation`: RED `notation.ShortHelp undefined (type configNode has no field or method ShortHelp)`, GREEN `job site-green: exit 0`. `test/ui/completion-enum-value-summary.ci` |
| AC-5 | every producer above copies each text on its own; `EnumValueSummaries` omits a value with no `ze:help` and the row, the tree and the reference print an empty summary | the `silent` value of `TestEnumValueCompletionShowsDeclaredSummary`, `TestConfigurationReferencePrintsExplanation` and `TestConfigLeafFormRendersBothTexts` (no block for a leaf with no explanation) |
| AC-7 | `schemaEnums` (`internal/le/docvalid/helpshape_schema.go`) judges every value of every enumeration leaf and union member under the two caps; `HelpShapeReport.SchemaEnumValues` and `SchemaEnumValuesWithSummary` print as `Enum values rendered` and `Enum values with a summary` | `TestHelpShapeJudgesEveryRenderedEnum` was written beside the widened `schemaEnums`, so its own pre-implementation red was not recorded; the discrimination evidence is the OLD assertion going red under the new code, `TestHelpShapeCapsAnEnumButDoesNotDemandADescription: the char cap names [ze-fixture-conf:sockets/binding/family/ipv4 ze-fixture-conf:sockets/state/open], want the enum on the list key` (job docvalid-16158480), which the old `schemaEnums` could not produce. GREEN `job docvalid-2a6c45e5`: every `TestHelpShape*` ok. Over the checkout: 813 rendered, 578 with a summary and 19 refused before the typedef walk and the summary rewrites; 813 rendered, 813 with a summary, none refused after (help-shape-typedef.log) |

## Review Gate

### Round 1 (2026-09-15, independent Opus 5 reader, whole uncommitted diff at d23e6c1b99)

Scope: every file the three implementers changed, both lenses (correctness and wiring; design and simplicity), the four pages and the recorded RED/GREEN lines.

| # | Sev | File / symbol | What is wrong | What settles it |
|---|-----|---------------|---------------|-----------------|
| 1 | BLOCKER | `internal/component/web/handler_admin.go` `commandFormParameters`, `HandleAdminExecute`; `component_command_form.templ` | The form now renders an `<input>` per `ArgDef`, but `HandleAdminExecute` builds the command from the URL path alone and reads no form value: a typed value is dropped in silence. Before the diff `Parameters` was never filled, so no dead input existed | Append the posted values in `ArgDef` order, keyword before value, with a handler test that POSTs and asserts the dispatched string |
| 2 | ISSUE | `internal/le/docvalid/helpshape_schema.go` (the `collectSchema` comment), `helpshape_schema_test.go` (the ArgDef comment), `docs/contributing/documentation-testing.md` (the `-cmd` leaf row) | The new comment says a `-cmd` or `-api` leaf is judged by the command tree corpus: false, `collectSchema` walks `ConfModuleNames()` only and `helpshape.go` reads no `ArgDefs`, `Input` or `Output`. The test comment and the page row still say `ArgDef` holds no text | Judge `ArgDef` summaries and rpc and notification leaf summaries under the two caps, count them, and make the three statements true |
| 3 | ISSUE | `helpshape_schema.go` `schemaEnums` comment | Stale: "an enum reached through a typedef renders with no summary"; `collectEnumSummaries` follows `YangType.Base` | Delete the two lines |
| 4 | ISSUE | `cmd/ze/hub/command_meta.go` `buildParamMeta`, `service_mcp.go` `mcpCommandLister`, `api.go` `apiCommandLister` | The three `Description: p.Description` copies have no test; reverting any one leaves every test green | A hub test over `buildParamMeta` with a leaf declaring both texts; a `test/plugin/mcp-tools-list-*.ci` matching `title` and `description` on one real property |
| 5 | ISSUE | `internal/le/site/testdata/published-yang-config-tree.json`, `published-configuration{.md,-body.html}` | Fixtures keep the old shape (`description` = summary, no `short-help`, no `values`); R-1 names the fixture as same-phase | Regenerate the three from `show yang tree --config \| json` |
| 6 | ISSUE | `docs/architecture/web-interface.md` (the config-editor row); `config_container.templ` | AC-3 names leaf, container AND list; `ContainerNode` and `ListNode` explanations reach no web surface, and the page was rewritten to say so. Scope narrowed without the owner | Render the explanation under the container and list heading, recapture the golden |
| 7 | ISSUE | `config/yang/cli/tree.go` `yangEnumValues`; `docvalid/helpshape_schema.go` `schemaEnums`; `cli/completer.go` `valueCompletions` | "The values an enum leaf or union renders" is derived three times; the gate's population-judged-equals-population-rendered rests on copies agreeing | One `yang.EnumValueNames(entry)` beside `EnumValueSummaries`, read by all three |
| 8 | NOTE | `helpshape_schema_test.go` | `./le commit audit`: `TestHelpShapeIgnoresAnEnumThatKeysNoList` deleted; it asserted what AC-7 removes | The commit carries the accepted row naming `TestHelpShapeJudgesEveryRenderedEnum` |
| 9 | NOTE | `docs/architecture/api/commands.md` (the readers sentence under `LeafMeta`) | "Five readers" lists four | Say four |
| 10 | NOTE | `cli/format.go` `valueJSON`; AC-5 text | AC-5 says "empty string"; every envelope omits the key except `values[].short-help`, which writes `""` | Align the AC wording with the recorded decision; make `valueJSON` omit like its siblings |
| 11 | NOTE | spec A-3; `mcp/tools.go` `addYANGParams` | A-3 still `unvalidated`; a `description`-only MCP client loses the summary on a summary-only leaf | Closure records the inspection or breaks A-3 |
| 12 | NOTE | the four new `test/ui/*.ci` | No run recorded under `tmp/` | Read the parent's ui log once |

AC verdicts: AC-1 proven but weak (#1, #12); AC-2 proven but weak (#4); AC-3 proven for leaf, unmet for container and list (#6); AC-4 proven; AC-5 proven under the omit-key reading (#10); AC-6 proven but weak (#4); AC-7 proven (813/813, #7 on the walk).

### Round 2 (scope: the fixes to #1 to #7, #9, #10 and their sibling call sites)

Evidence (2026-09-15, two fix agents; logs under `tmp/session/2026-09-15-d19fb97b-265c-45eb-a3c1-a3078ceebb08/scratch/`):

| # | What changed (file, symbol) | Test | RED | GREEN |
|---|-----------------------------|------|-----|-------|
| 1 | `web/handler_admin.go` `HandleAdminExecute(renderer, dispatch, tree)`, `commandArguments`: posted values appended in ArgDef order, keyword before value, empty skipped, space quoted, `"` refused 400; `cmd/ze/hub/service_web.go` passes `commandTree` | `TestAdminExecuteAppendsPostedArguments` (5 cases), `TestAdminExecuteRefusesAQuoteInAValue` (`handler_admin_test.go`) | `too many arguments in call to HandleAdminExecute` (job web-red, build failure) | job web-green exit 0 (`fix-web-green.log`); r2: `r2-web-1.log` lists neither test under `--- FAIL` |
| 2 | `docvalid/helpshape.go` `HelpShapeReport.arguments`, `leaves`; `usage.go` calls `arguments` after `node`; `collectRPCs` calls `leaves` for input, output, notifications; surfaces `argument`, `leaf`; `ze-bgp-api.yang` `session-established/asn` summary shortened; `collectSchema` comment, `documentation-testing.md` rows corrected | `TestHelpShapeCapsACommandArgumentSummary`, `TestHelpShapeJudgesAnInheritedArgumentOnce`, `TestHelpShapeCapsAnRPCAndNotificationLeafSummary` (`helpshape_schema_test.go`), replacing `TestHelpShapeIgnoresALeafInACommandModuleButCapsOneInAConfigModule` | `the char cap names [], want the command argument` (job docvalid-red, `fix-docvalid-red.log`): the argument judge and the rpc input/output judge were disabled; the notification judge stayed live, so that RED shows `socket-closed/reason` still capped and proves nothing about it. Round 3 observed the notification red on its own: with the `ExtractNotifications` call to `leaves` disabled, `the char cap names [ze-fixture-api:socket-clear/input/idle], want [ze-fixture-api:socket-clear/input/idle ze-fixture-api:socket-closed/reason]` (`r3-docvalid-notif-red.log`) | job docvalid-green2 exit 0 (`fix-docvalid-green2.log`); r2: `./le --name swap docvalid help-shape` exit 0, 241/241 argument, 265/265 RPC leaf, 20/20 notification leaf texts (`r2-help-shape.log`) |
| 3 | `helpshape_schema.go` `schemaEnums` comment: the two typedef lines deleted | none (comment) | n-a | n-a |
| 4 | `cmd/ze/hub/command_meta.go` `buildParamMeta` (producer unchanged); `test/plugin/mcp-tools-list-argument-texts.ci` written | `TestBuildParamMetaCarriesBothLeafTexts` (`command_meta_test.go`); the `.ci` | no RED run: the producer already existed, the test is the discrimination the review asked for | job fix-hub-green exit 0 (`fix-hub-green.log`); r2 `.ci`: `433/802 PASS mcp-tools-list-argument-texts` in `./le --name swap functional plugin` (`r2-functional-plugin.log`; the suite's 17 reds are peer-exchange mismatches and the assertion-placement rule, none naming this file or a help-text key) |
| 5 | `internal/le/site/testdata/published-yang-config-tree.json` replaced by the tree `./le site config-tree` extracts (`go run` of `./cmd/ze` with the shipped tags, `show yang tree --config \| json`, indexed by `writeYANGConfigTree`); `published-configuration.md` and `published-configuration-body.html` recaptured from `renderConfiguration` over that tree (body: `<main>` with the two `application/json` payloads emptied, as the fixture header says). The two rendered fixtures are now the renderer's own output, not a gh-pages publish | `TestTheConfigurationReferenceReadsAsThePublishedPage`, `TestTheConfigurationPayloadsCarryTheSchemaAndItsOwners`, `TestTheConfigurationMirrorReadsAsThePublishedMirror` | n-a (fixture refresh) | `r2-site.log`: the three pass; the four reds are `TestTheChangesIndex*` (3) and `site/wiki` `TestTheCommittedIndexStatesTheLiveWiki`, both journaled before this spec |
| 6 | `web/handler_config.go` `ConfigViewData.Description`; `handler_config_walk.go` `buildConfigViewData` copies `ContainerNode.Description` and `ListNode.Description`; `config_container.templ` renders `<p class="config-help">` at the top, `config_list.templ` under the `Entries` heading; `docs/architecture/web-interface.md` row rewritten | `TestConfigContainerAndListRenderTheExplanation`, `TestConfigViewDataCarriesTheNodeExplanation` (`handler_config_leaf_test.go`) | r2, both copies disabled and both templ `if` arms forced false: `--- FAIL: TestConfigContainerAndListRenderTheExplanation` (`:75`, `:85`), `--- FAIL: TestConfigViewDataCarriesTheNodeExplanation` (`:112`, `:116`), `r2-web-red.log` | r2: whole web package `ok` after the recapture (`r2-web-green.log`, exit 0); goldens recaptured with `-update-golden` (`r2-web-golden.log`, exit 0): the only new diff is the templ whitespace after each command-form `<input>` from #1's conditional blocks; no golden fixture carries a container or list explanation, so no golden moved for #6; `./le doc check templ-output` exit 0 (`r2-templ-check.log`) |
| 7 | `config/yang/enum.go` `yang.EnumValueNames(entry)`; `cli/tree.go` `yangEnumValues`, `docvalid/helpshape_schema.go` `schemaEnums`, `cli/completer.go` `valueCompletions` read it; the three walks deleted | the existing enum tests over the three readers | n-a (refactor to one producer) | job fix-enum-green exit 0 over config/yang, config/yang/cli, component/cli, component/command, docvalid, wikicatalog (`fix-enum-green.log`); r2: 813/813 enum values (`r2-help-shape.log`) |
| 8 | none (commit audit row is the commit's) | n-a | n-a | n-a |
| 9 | `docs/architecture/api/commands.md`: "Five readers" is "Four readers" | none (prose) | n-a | n-a |
| 10 | `cli/format.go` `valueJSON.ShortHelp` is `short-help,omitempty`, built by conversion; AC-5 row reads "omits the other key" | `TestTreeJSONOmitsAnEmptyValueSummary` (`cli/tree_test.go`) | none recorded | in fix-enum-green |

Gates run once this round: `./le --name swap doc check verify` exit 1 (`r2-doc-verify.log`). The drift stage reports 3770 issues, 3769 naming a file under `../gh-pages` and 1 naming `../wiki/command-catalog.md`, the two published trees the checkout regenerates (`./le site build`, `./le wiki-catalog update`); two of them are this spec's rename reaching published data that has not been regenerated (`../gh-pages/data/cli-commands.json` still carries `long-help`, which the reader no longer accepts; the wiki catalog disagrees with the live catalog on the renamed fields), and the rest are the published catalog lagging the live one. The two anchor claims are `docs/architecture/api/text-format.md:147` (`FamilyIPv4Unicast`) and `docs/features/formatting.md:39` (`validCLIFormats`), neither a page this spec touched; every other stage green. Owed to the main thread: `./le verify lint run`.

#### Round 2 findings (independent Opus 5 reader, a third context)

| # | Sev | File / symbol | What is wrong | What settles it |
|---|-----|---------------|---------------|-----------------|
| 1 | BLOCKER | `internal/le/docvalid/helpshape_schema_test.go`; `helpshape_schema.go` `schema` | The deleted `TestHelpShapeIgnoresALeafInACommandModuleButCapsOneInAConfigModule` also asserted that a CONFIG leaf's over-cap `ze:help` is refused; nothing asserts that now, so deleting `r.judgeCaps(surfaceSchema, ...)` in `schema` leaves every test green, and `./le commit audit` lists the deletion as unexplained | Restore that half as its own test; accepted audit rows for both deletions |
| 2 | ISSUE | `web/handler_admin.go` `commandArguments`; `plugin/server/command.go` `Dispatch`, `validateCommandArgs` | An anchored argument (`selector` on `request bgp withdraw all`) is posted as `selector <value>` after the path; `Dispatch` adopts only a positional selector, so every `RequiresSelector` command answers "requires a selector" for the one input the operator filled. The stub dispatcher in the handler test cannot see it | Print an anchored value after its anchor keyword (`ArgDef.Anchor`); test through the real `Dispatcher` with a `RequiresSelector` command |
| 3 | ISSUE | `test/web/` | No `.wb` posts an admin form value through the daemon; the handler tests stub the dispatcher | One `.wb` that opens a leaf admin command, fills `param-<name>`, submits, and asserts the result card |
| 4 | ISSUE | `cmd/ze/hub/api.go` `apiCommandLister` | The gRPC copy of `Description` has no test through the hub | A hub test over `apiCommandLister`, or a gRPC `.ci` matching both keys on one real param |
| 5 | NOTE | `cli/completer_enum_summary_test.go` | The "served from the cache" assertion compares two equal maps | Assert identity or count the reader's calls, or drop the claim |
| 6 | NOTE | this record | Row 2's RED shows `socket-closed/reason` already capped, so the notification-leaf cap has no observed red; "every drift row names `../gh-pages` or `../wiki`" is not exact | Correct the two sentences |
| 7 | NOTE | `config/yang/enum.go` `EnumValueNames` | Two allocations per call on the per-keystroke completion path | None unless completer latency is measured |

Round 2 does not end clean: 1 BLOCKER, 3 ISSUE.

### Round 3 (scope: the fixes to round 2 #1 to #6 and their sibling call sites)

#### Round 3 evidence

Evidence (2026-09-15, one fix agent; logs under `tmp/session/2026-09-15-d19fb97b-265c-45eb-a3c1-a3078ceebb08/scratch/r3-*.log`):

| # | What changed (file, symbol) | Test | RED | GREEN |
|---|-----------------------------|------|-----|-------|
| 1 | `internal/le/docvalid/helpshape_schema_test.go`: `TestHelpShapeCapsAConfigLeafSummary` puts an over-cap `ze:help` on `sockets/binding/family` in the fixture config module; `test/weakened/a98dde17.md`: one row each for the deleted `TestHelpShapeIgnoresAnEnumThatKeysNoList` and `TestHelpShapeIgnoresALeafInACommandModuleButCapsOneInAConfigModule`, naming the tests that carry their surviving assertions, preamble corrected | `TestHelpShapeCapsAConfigLeafSummary` | `r.judgeCaps(surfaceSchema, ...)` in `schema` commented out: `the char cap names [], want exactly [ze-fixture-conf:sockets/binding/family]` (job r3-docvalid-red); restored from a byte copy | job r3-docvalid-green exit 0, whole package; `./le commit audit` (`r3-commit-audit.log`) |
| 2 | `internal/component/web/handler_admin.go` `commandArguments(path []string, ...)`, `writeArgumentValue`: a value whose `ArgDef.Anchor` names a path keyword is printed bare after that keyword, which is where `matchCommandTokens` binds it (`anchoredDef`); every other value keeps the keyword form after the command. `internal/component/command/node.go` `ArgDef.Anchor` comment: "nothing binds a value by anchor" was stale since `anchoredDef`, rewritten | `TestAdminExecuteBindsAnAnchoredValueThroughTheDispatcher` (`handler_admin_test.go`): real `pluginserver.NewDispatcher` with `send bgp withdraw all` registered `RequiresSelector` and `selector` anchored to `bgp` | anchor placement forced off: `expected: "send bgp 10.0.0.1 withdraw all" actual: "send bgp withdraw all selector 10.0.0.1"`, selector `""`, card `"send bgp withdraw all requires a selector"` (job r3-web-red) | job r3-web-green exit 0, whole web package (165 s) |
| 3 | `test/web/admin-command-posts-argument.wb`: opens `/admin/clear/firewall/domain-group` (the `name` leaf of the `-cmd` module declares both texts), asserts both texts, fills `param-name`, clicks Execute, asserts the card title `clear firewall domain-group name blocked` and the web-only dispatcher's standalone-mode answer. No `.wb` server kind runs the full daemon with the admin UI, so the answer the card carries is `webOnlyDispatcher`'s. The first draft opened `/admin/system/command/help`, whose `name` leaf is an rpc INPUT leaf (`ze-system-api.yang`): `extractArgDefs` reads a node's `ArgDefs` from the `ze:command` container's own leaves, so that page renders no input and the draft failed at `expect=element:text=Command name` (`wb-old-red.log`). Journaled in `plan/journal/command-takes-an-untyped-positional-value.md` | the `.wb` | `wb-old-red.log`: step 3 `expected element with text "Command name" not found in snapshot` | `wb-single.log`: `1/1 PASS`, nine steps; `./le functional web` (`functional-web-3.log`), see the report |
| 4 | `cmd/ze/hub/api.go` `apiCommandLister(src func() []commandMeta)`: takes the meta source as `mcpCommandLister` does; `buildAPIEngine` passes `commandMetaSource(server)` | `TestAPICommandListerCarriesBothLeafTexts` (`api_test.go`) | `Description: p.Description` blanked: `expected: "Only the sockets bound to this port are listed." actual: ""` (`api_test.go:591`, job r3-hub-red); restored from a byte copy | job r3-hub-green exit 0, whole hub package |
| 5 | `internal/component/cli/completer_enum_summary_test.go`: the cache assertion compares the map identity (`reflect.ValueOf(...).UnsafePointer()`) of the two calls and of `c.enumSummaries[entry]` | `TestEnumValueCompletionShowsDeclaredSummary` | cache hit disabled (`held && false`): `the second call must serve the cached map, not a fresh walk` at `:95` and `:97` (job r3-cli-red); restored | job r3-cli-green exit 0, whole cli package |
| 6 | this record: the row 2 RED sentence and the drift sentence under Round 2 rewritten; the notification-leaf red observed and recorded in row 2 | n-a | `r3-docvalid-notif-red.log` | n-a |
| lint | `internal/le/docvalid/helpshape.go` `argumentLabel`: `for i, token := range slices.Backward(tokens)` (modernize `slicesbackward`, `lint4.log`) | existing | n-a | job r3-docvalid-green2 exit 0 |

`gofmt -l` clean over web, command, docvalid, cli, hub. Owed to the main thread: `./le verify lint run`.

#### Round 3 findings (independent Opus 5 reader, a fourth context)

| # | Sev | File / symbol | What is wrong | What settles it |
|---|-----|---------------|---------------|-----------------|
| 1 | ISSUE | `docs/architecture/web-interface.md`, the `handler_admin.go` row | The row says every posted value is appended keyword before value after the command; after round 3's fix an anchored value goes bare after its anchor keyword inside the path | One clause on the row naming the anchored placement (`anchoredDef`) |
| 2 | NOTE | `internal/component/mcp/tools.go`, the tool call builder | Sibling shape of round 2 #2: MCP emits an anchored selector in keyword form after the command, which `Dispatch` refuses. Pre-existing; the spec's goal does not depend on it | Journal row in `plan/journal/guard-demands-what-the-model-cannot-supply.md` (written) |
| 3 | NOTE | Round 3 evidence, the `./le commit audit` cell | "2 unexplained" is expected for an uncommitted tree; the commit names the shard | Nothing owed |

Round 3 does not end clean: 1 ISSUE (#1, one page clause).

### Round 4 (scope: the one clause on `docs/architecture/web-interface.md`)

The clause is rewritten: an anchored value goes bare after its anchor keyword, inside the path, where `anchoredDef` binds a peer selector; every other value is appended after the command as `name value` in declaration order.

Round 4 (independent Opus 5 reader, a fifth context) read the row against `commandArguments`, `writeArgumentValue`, `matchCommandTokens` and `anchoredDef`, and the journal row against the MCP call builder and `Dispatch`: 0 BLOCKER, 0 ISSUE, 1 NOTE ("a peer selector" is the one example of an anchored leaf in the tree today, not the rule). Round 4 ends clean.

### Record

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/both-help-texts-reach-every-surface-d19fb97b-265c-45eb-a3c1-a3078ceebb08.md` (107 files, verdict=clean) |
| `./le spec session review check` | `review_gate: OK (80 code files, clean, hashes match)` |
| Rounds | 4: round 1 found the form input with no reader, round 2 the anchored placement and the deleted config-leaf assertion, round 3 one page clause, round 4 clean |
| Reviewer lenses used | correctness and wiring; design and simplicity; the pages against the producers; each RED line against the log it names |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | the admin form rendered an input per `ArgDef` that `HandleAdminExecute` never read | `internal/component/web/handler_admin.go` | `commandArguments` appends the posted values; `TestAdminExecuteAppendsPostedArguments` |
| 2 | ISSUE | `collectSchema` comment, the test comment and the page said a `-cmd` leaf is judged; nothing judged it | `internal/le/docvalid/helpshape_schema.go`, `docs/contributing/documentation-testing.md` | `HelpShapeReport.arguments` and `leaves` judge arguments and rpc and notification leaves |
| 3 | ISSUE | stale typedef comment on `schemaEnums` | `helpshape_schema.go` | deleted |
| 4 | ISSUE | the three `Description: p.Description` copies in the hub had no test | `cmd/ze/hub/command_meta.go`, `service_mcp.go`, `api.go` | `TestBuildParamMetaCarriesBothLeafTexts`, `TestAPICommandListerCarriesBothLeafTexts`, `test/plugin/mcp-tools-list-argument-texts.ci` |
| 5 | ISSUE | the site fixtures kept the old tree shape | `internal/le/site/testdata/` | regenerated from `show yang tree --config \| json` |
| 6 | ISSUE | container and list explanations reached no web surface | `internal/component/web/handler_config_walk.go`, `config_container.templ`, `config_list.templ` | `ConfigViewData.Description`; `TestConfigContainerAndListRenderTheExplanation` |
| 7 | ISSUE | the enum value population was derived three times | `internal/component/config/yang/enum.go` | `EnumValueNames`, read by the tree, the gate and the completer |
| 8 | BLOCKER | deleting a test also deleted the config-leaf cap assertion, and `r.judgeCaps(surfaceSchema, ...)` could go without a red | `internal/le/docvalid/helpshape_schema_test.go` | `TestHelpShapeCapsAConfigLeafSummary`; `test/weakened/a98dde17.md` |
| 9 | ISSUE | an anchored selector was posted in keyword form, which `Dispatch` refuses | `handler_admin.go` `writeArgumentValue` | bare after its anchor keyword; `TestAdminExecuteBindsAnAnchoredValueThroughTheDispatcher` over the real dispatcher |
| 10 | ISSUE | no `.wb` posted an admin form value | `test/web/admin-command-posts-argument.wb` | written; `2/98 PASS` in `functional-web-3.log` |
| 11 | ISSUE | the gRPC `Description` copy in `apiCommandLister` had no test | `cmd/ze/hub/api.go` | `apiCommandLister(src)`; `TestAPICommandListerCarriesBothLeafTexts` |
| 12 | ISSUE | the `handler_admin.go` page row did not name the anchored placement | `docs/architecture/web-interface.md` | one clause naming `anchoredDef` |

## Executive Summary

Every YANG node an operator reads declares a summary (`ze:help`) and an
explanation (`description`), and until this spec five carriers dropped one or
both before a reader saw them. Now `ArgDef`, `LeafMeta`, the three config
schema nodes, `AnalysisNode` and the enum completion row each hold the pair,
and the readers render it: the CLI JSON help, the site and wiki catalogs, the
MCP tool schema, the web admin form, the API and gRPC schemas, the web config
editor, the published configuration reference and Tab in `ze cli`. The
help-shape gate judges every carrier (813/813 enum values, 241/241 arguments,
265/265 rpc leaves, 20/20 notification leaves, zero refusals).

Risks and observations: R-1 met (the catalog and docvalid fixtures moved with
the keys, `catalogfields_test.go`); R-2 met (`TestConfigurationReferencePrintsExplanation`);
R-3 met (the explanation is a block, the tooltip keeps the summary); R-4 holds
by construction (`argDefFor` runs on the declaring leaf, first text wins); R-5
met (`Completer.enumValueSummaries` caches per entry, identity-asserted). A-3
is broken: no MCP client documents reading a property `title`, so a
`description`-only client shows the explanation and loses the summary; the
product keeps `title` for the summary because deriving one text from the
other is what AC-5 forbids.

## Implementation Summary

### What Was Implemented
- `command.ArgDef` gained `ShortHelp` and `Description`, filled by `argDefFor` (`internal/component/config/yang/command.go`) and read by `extractArgs` (`cmd/ze/help_command.go`), `catalogArg` (`internal/le/site/catalog.go`), `extractArgs` (`internal/le/wikicatalog/catalog.go`) and `commandFormParameters` (`internal/component/web/handler_admin.go`); the admin form now posts the values it renders (`commandArguments`, `writeArgumentValue`).
- `yang.LeafMeta` gained `Description`, filled by `extractEntryLeaves` (`rpc.go`) and carried to `api.ParamMeta` (`apiCommandLister`), the JSON Schema `title`/`description` (`CommandSchema`), the gRPC `ParamInfo.short_help` (`commandMetaToProto`, `api/proto/ze.proto`), the MCP `inputSchema` (`addYANGParams`) and `ze help ai --json` (`aihelp.Build`: `input`, `output`, `notifications`).
- The web config editor renders `LeafNode.Description` as a block under the input (`buildLeafField`, `config_leaf_input.templ`, `input_wrapper.templ`) and the container and list explanation at the top (`buildConfigViewData`, `config_container.templ`, `config_list.templ`).
- `cli.AnalysisNode` gained `ShortHelp`, `Description` and `Values` (`walkYANGEntry`, `yangEnumValues`, `formatTreeJSON`); the site prints them (`writeConfigChildMirror`, `configNode`, `configscript.go`) and the fixtures were regenerated.
- One reader of an enum value's texts, `yang.EnumValueSummaries` and `yang.EnumValueNames` (`enum.go`), following `YangType.Base` through every typedef and union member; the completer (`valueCompletions`, cached in `Completer.enumValueSummaries`), the tree and the help-shape gate read it, and the `enum value` literal is gone.
- `./le docvalid help-shape` judges every rendered enum value, every command argument and every rpc and notification leaf (`schemaEnums`, `HelpShapeReport.arguments`, `leaves`).

### Bugs Found/Fixed
- The admin command form never filled `CommandFormData.Parameters`, so no argument input existed; now rendered and posted (`TestAdminExecuteAppendsPostedArguments`, `TestAdminExecuteBindsAnAnchoredValueThroughTheDispatcher`).
- `ze help ai --json` listed no rpc leaves and no notifications (`TestAIHelp*` over `aihelp.Build`, `test/ui/help-ai-json-rpc-leaf-texts.ci`).
- An enumeration reached through a typedef rendered no summary (`TestEnumValueSummaries*` typedef and union subtests, `enum_test.go`).

### Documentation Updates
- `docs/architecture/api/commands.md` (`argDefFor`, `LeafMeta` readers), `docs/architecture/config/yang-config-design.md` ("CLI Help from YANG"), `docs/architecture/web-interface.md` (admin form and config editor rows), `docs/architecture/mcp/overview.md` and `docs/guide/mcp/overview.md` (`title`/`description`), `docs/contributing/documentation-testing.md` (the help-shape corpora), `docs/guide/api.md` (JSON examples), `docs/guide/config-editor.md`, `docs/features.md` (Self-Documenting System row, at closure).
- `./le doc check verify`: exit 1 (`r2-doc-verify.log`, and `close-doc-verify.log` at closure after the `docs/features.md` edit); every red is drift under `../gh-pages` and `../wiki` (the published trees the checkout regenerates), the two anchor claims on `docs/architecture/api/text-format.md` (`FamilyIPv4Unicast`) and `docs/features/formatting.md` (`validCLIFormats`), pages this spec did not touch, and rules-gate rows of other sessions. The three anchors added to `docs/features.md` resolve (`checked 2561 code paths`, neither named).

### Deviations from Plan
- `internal/plugins/meta/cmd/help.go` `args` was not changed: it is the usage-hint string a plugin registers, not an `ArgDef` list (Design Insights).
- `internal/component/config/yang_schema.go` was not changed: `LeafNode`, `ContainerNode` and `ListNode` carry both texts since b57ec4ab6b; `TestSchemaNodesCarryBothTexts` proves it.
- `TestEnumValueCompletionShowsDeclaredSummary` lives in `internal/component/cli/completer_enum_summary_test.go`, not `completer_test.go`.
- AC-6 rides `LeafMeta` through the hub (`buildParamMeta`), not `ArgDef`: MCP `ParamInfo` is filled from the rpc input leaves.
- AC-5 says an absent text is an omitted key, not an empty string (round 1 #10).
- A-3 broken: `title` stays the summary carrier (see Mistake Log).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The admin form rendered an `<input>` per `ArgDef` and `HandleAdminExecute` read no form value | An input with no reader drops the operator's value in silence | round 1 BLOCKER #1 | `commandArguments` appends the posted values; handler tests POST and assert the dispatched string |
| approach | Posted values were appended `name value` after the command for every `ArgDef` | An anchored value (`ArgDef.Anchor`) binds bare after its anchor keyword inside the path (`anchoredDef`), and `Dispatch` adopts only a positional selector | round 2 ISSUE #2 | `writeArgumentValue`; a test through the real `pluginserver.NewDispatcher` |
| approach | The `web-interface.md` row still said every value is appended after the command | The anchored value goes inside the path | round 3 ISSUE #1 | one clause naming `anchoredDef`; a page edit lands with the code edit, not after |
| assumption | The first `.wb` opened `/admin/system/command/help` expecting a `name` input | That `name` leaf is an rpc INPUT leaf; `extractArgDefs` reads a node's `ArgDefs` from the `ze:command` container's own leaves, so the page renders no input | `wb-old-red.log` step 3 | the `.wb` opens `/admin/clear/firewall/domain-group`; journal row in `plan/journal/command-takes-an-untyped-positional-value.md` |
| assumption | A-3: an MCP client reads a property's JSON Schema `title` as the short label | The MCP specification (2025-06-18, "Data Types / Tool") defines `title` at the TOOL level only, every property example carries `description` alone, and no client documents reading a property `title` | closure inspection of the MCP specification | `title` stays the summary carrier and `description` the explanation, because deriving one from the other is what AC-5 forbids; a `description`-only client loses the summary |
| approach | Documentation Update Checklist row 1 named `docs/features.md` and the implementers left it | The Self-Documenting System row named the two command texts only | closure step 4 grep | one sentence on the row naming the five carriers |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `command.ArgDef` holds the pair; five readers render it | Done | `internal/component/config/yang/command.go` `argDefFor`; `cmd/ze/help_command.go`, `internal/le/site/catalog.go`, `internal/le/wikicatalog/catalog.go`, `internal/component/mcp/tools.go`, `internal/component/web/handler_admin.go` | MCP reads the leaf through `LeafMeta` |
| `yang.LeafMeta` holds the pair; API, schema, gRPC, `ze help ai`, plugin schema listing | Done | `internal/component/config/yang/rpc.go` `extractEntryLeaves`; `cmd/ze/hub/api.go`, `internal/component/api/schema.go`, `internal/component/api/grpc/convert.go`, `internal/component/aihelp/aihelp.go` | |
| config nodes hold the pair; web editor renders both | Done | `internal/component/web/handler_config_leaf.go` `buildLeafField`, `fragment.go` `buildFieldMeta`, `handler_config_walk.go` `buildConfigViewData` | nodes carried both since b57ec4ab6b |
| `AnalysisNode` holds the pair and enum values; site reference, mirror, `llms.txt` | Done | `internal/component/config/yang/cli/tree.go` `walkYANGEntry`, `yangEnumValues`; `internal/le/site/config.go`, `llmsdata.go`, `configscript.go` | |
| enum value completion shows the declared summary | Done | `internal/component/cli/completer.go` `valueCompletions`, `enumValueSummaries` | `enum value` literal gone |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestArgDefForCarriesBothTexts`, `TestCatalogArgCarriesBothTexts`, `TestCommandFormPrintsArgumentTexts`, `test/ui/help-command-json-argument-texts.ci` | |
| AC-2 | Done | `TestExtractRPCsCarriesBothLeafTexts`, `TestCommandSchemaWritesTitleAndDescription`, `TestCommandMetaToProto`, `TestAPICommandListerCarriesBothLeafTexts`, `test/ui/help-ai-json-rpc-leaf-texts.ci` | |
| AC-3 | Done | `TestConfigLeafFormRendersBothTexts`, `TestConfigContainerAndListRenderTheExplanation`, `TestAnalysisTreeCarriesBothTextsAndValues`, `test/ui/yang-tree-json-both-texts.ci` | |
| AC-4 | Done | `TestEnumValueCompletionShowsDeclaredSummary`, `TestConfigurationReferencePrintsExplanation`, `test/ui/completion-enum-value-summary.ci` | |
| AC-5 | Done | the `label` and `owner` cases of the schema and MCP tests; the `silent` value of the completion test; `TestTreeJSONOmitsAnEmptyValueSummary` | an absent text omits the key |
| AC-6 | Done | `TestToolInputSchemaCarriesArgumentTexts`, `TestBuildParamMetaCarriesBothLeafTexts`, `test/plugin/mcp-tools-list-argument-texts.ci` | |
| AC-7 | Done | `TestHelpShapeJudgesEveryRenderedEnum`; `r2-help-shape.log` 813/813 | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestArgDefForCarriesBothTexts` | Done | `internal/component/config/yang/command_test.go:1883` | |
| `TestExtractRPCsCarriesBothLeafTexts` | Done | `internal/component/config/yang/rpc_test.go:129` | |
| `TestSchemaNodesCarryBothTexts` | Done | `internal/component/config/yang_schema_test.go:1502` | held at first run |
| `TestAnalysisTreeCarriesBothTextsAndValues` | Done | `internal/component/config/yang/cli/tree_test.go:286` | |
| `TestEnumValueCompletionShowsDeclaredSummary` | Changed | `internal/component/cli/completer_enum_summary_test.go:66` | its own file |
| `TestCommandSchemaWritesTitleAndDescription` | Done | `internal/component/api/schema_test.go:195` | |
| `TestToolInputSchemaCarriesArgumentTexts` | Done | `internal/component/mcp/tools_test.go:1443` | |
| `TestConfigLeafFormRendersBothTexts` | Done | `internal/component/web/handler_config_leaf_test.go:24` | |
| `TestConfigurationReferencePrintsExplanation` | Done | `internal/le/site/config_test.go:364` | |
| `TestCatalogArgCarriesBothTexts` | Done | `internal/le/wikicatalog/catalog_test.go:156` | |
| `TestHelpShapeJudgesEveryRenderedEnum` | Done | `internal/le/docvalid/helpshape_schema_test.go:106` | |
| the four `test/ui/*.ci` | Done, unrun | `test/ui/` | the ui suite outlasts a subagent's window; owed to the main thread |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/command/node.go` | Done | |
| `internal/component/config/yang/command.go`, `rpc.go`, `cli/tree.go`, `cli/format.go`, `enum.go` | Done | `enum.go` is the one enum reader |
| `internal/component/api/types.go`, `schema.go`, `grpc/convert.go`, `cmd/ze/hub/api.go` | Done | plus `api/proto/ze.proto` |
| `internal/component/config/yang_schema.go` | Skipped | already carries both since b57ec4ab6b; no edit owed |
| `internal/component/web/handler_config_leaf.go`, `config_leaf_input.templ`, `fragment.go`, `handler_admin.go`, `component_command_form.templ` | Done | plus `input_wrapper.templ`, `config_container.templ`, `config_list.templ`, `handler_config_walk.go` |
| `internal/le/site/config.go`, `llmsdata.go`, `catalog.go` | Done | plus `configscript.go`, `commands.go`, `derived.go`, `search.go` |
| `internal/le/wikicatalog/catalog.go` | Done | plus `render.go` |
| `internal/component/cli/completer.go` | Done | |
| `internal/component/mcp/tools.go` | Done | |
| `internal/plugins/meta/cmd/help.go` | Changed | `args` is a usage string; not an `ArgDef` list (Design Insights) |
| `internal/le/docvalid/helpshape_schema.go` | Done | plus `helpshape.go`, `usage.go`, `command_surfaces.go` |
| the six pages | Done | plus `docs/architecture/mcp/overview.md`, `docs/guide/config-editor.md`, `docs/features.md` |
| the four `test/ui/*.ci` | Done | plus `test/plugin/mcp-tools-list-argument-texts.ci`, `test/web/admin-command-posts-argument.wb` |

### Audit Summary
- **Total items:** 5 requirements, 7 ACs, 12 test rows, 13 file rows
- **Done:** 35
- **Partial:** none
- **Skipped:** `yang_schema.go` (no edit owed; the field exists)
- **Changed:** `TestEnumValueCompletionShowsDeclaredSummary` file; `help.go` (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `command.ArgDef` carries both texts to the CLI JSON help, the site and wiki catalogs, the MCP schema and the web admin form | unit and functional | `TestArgDefForCarriesBothTexts` (RED `port.ShortHelp undefined`), `TestCatalogArgCarriesBothTexts`, `TestCommandFormPrintsArgumentTexts`, `test/ui/help-command-json-argument-texts.ci`, `test/web/admin-command-posts-argument.wb` (`2/98 PASS`, `functional-web-3.log`); `r2-help-shape.log`: 241/241 argument texts with a summary |
| `yang.LeafMeta` carries both texts to `ParamMeta`, `CommandSchema`, the gRPC schema and `ze help ai --json` | unit and functional | `TestExtractRPCsCarriesBothLeafTexts` (RED `Input[0].Description undefined`), `TestCommandSchemaWritesTitleAndDescription`, `TestAPICommandListerCarriesBothLeafTexts` (RED `expected: "Only the sockets..." actual: ""`), `TestBuildParamMetaCarriesBothLeafTexts`, `test/ui/help-ai-json-rpc-leaf-texts.ci`, `test/plugin/mcp-tools-list-argument-texts.ci` (`433/802 PASS`); `r2-help-shape.log`: 265/265 RPC and 20/20 notification leaf texts |
| the config schema nodes carry both texts to the web editor | unit | `TestConfigLeafFormRendersBothTexts` (RED `field.Description undefined`), `TestConfigContainerAndListRenderTheExplanation` (RED `r2-web-red.log`), the recaptured `get-fragment-detail.txt` golden carrying `<p class="ze-field-help">`; `r2-help-shape.log`: 3268/3268 config nodes with both texts |
| `cli.AnalysisNode` carries both texts and enum values to the site reference, its mirror and `llms.txt` | unit and functional | `TestAnalysisTreeCarriesBothTextsAndValues` (RED `notation.Description undefined`), `TestConfigurationReferencePrintsExplanation` (RED `notation.ShortHelp undefined`), `test/ui/yang-tree-json-both-texts.ci`; the regenerated `published-yang-config-tree.json` |
| enum value completion shows the declared summary | unit and functional | `TestEnumValueCompletionShowsDeclaredSummary` (RED `c.enumValueSummaries undefined`; cache identity RED `r3-cli-red.log`), `test/ui/completion-enum-value-summary.ci`; `r2-help-shape.log`: 813/813 enum values with a summary, `TestHelpShapeJudgesEveryRenderedEnum` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The MCP tool call builder emits an anchored selector in keyword form after the command, which `Dispatch` refuses | outside the goal: the texts reach the MCP schema; the call builder predates this spec | none: a defect met on the way, one row in `plan/journal/guard-demands-what-the-model-cannot-supply.md` (owner directive 2026-08-10) |
| `internal/plugins/meta/cmd/help.go` `args` gained no text keys | `args` is the usage-hint string a plugin registers (`CommandDecl.Args`), not an `ArgDef` list; the argument texts reach the five readers AC-1 names | none: the Files to Modify row misread the field, recorded in Design Insights |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ui/help-command-json-argument-texts.ci` | Yes | `-rw-rw-r-- 1068 Sep 15 04:19` |
| `test/ui/help-ai-json-rpc-leaf-texts.ci` | Yes | `-rw-rw-r-- 1050 Sep 15 04:19` |
| `test/ui/yang-tree-json-both-texts.ci` | Yes | `-rw-rw-r-- 1293 Sep 15 04:35` |
| `test/ui/completion-enum-value-summary.ci` | Yes | `-rw-rw-r-- 1910 Sep 15 04:35` |
| `test/plugin/mcp-tools-list-argument-texts.ci` | Yes | `-rw-rw-r-- 1829 Sep 15 05:33` |
| `test/web/admin-command-posts-argument.wb` | Yes | `-rw-rw-r-- 2105 Sep 15 06:55` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `argDefFor` fills both | `command.go:652-653`: `def.ShortHelp = GetHelpExtension(leaf.Exts)`, `def.Description = leaf.Description` |
| AC-2 | `extractEntryLeaves` fills both | `rpc.go:137-138`: `ShortHelp: GetHelpExtension(child.Exts)`, `Description: child.Description` |
| AC-3 | web and tree carry both | `TestConfigLeafFormRendersBothTexts` (`handler_config_leaf_test.go:24`), `TestAnalysisTreeCarriesBothTextsAndValues` (`tree_test.go:286`); `r3-web-green.log` (web package ok), `job-enum-green-71744d30.log`/`fix-enum-green.log` (config/yang/cli ok); the tree is unchanged since those runs, so they are not re-run (`ai/rules/pre-release.md`) |
| AC-4 | completion reads the declared summary | `completer.go:965-966`: `yang.EnumValueNames(entry)` then `c.enumValueSummaries(entry)`; no `enum value` literal in `completer.go` (grep: one hit, the `enumKeyVocabulary` comment) |
| AC-5 | nothing derives one text from the other | `addYANGParams` (`tools.go:523-528`) writes each key under its own `!= ""` guard; same shape in `argDefFor`, `extractEntryLeaves` |
| AC-6 | MCP property carries `title` and `description` | `tools.go:523-528`; `TestToolInputSchemaCarriesArgumentTexts` (`tools_test.go:1443`) |
| AC-7 | the gate judges every rendered enum value | `r2-help-shape.log`: `Enum values rendered: 813`, `Enum values with a summary: 813`; `TestHelpShapeJudgesEveryRenderedEnum` (`helpshape_schema_test.go:106`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze help command "show log recent" --json` | `test/ui/help-command-json-argument-texts.ci` | read: asserts `"short-help": "Filter by component name"` and the `description` on the `component` argument; not run (ui suite owed) |
| `ze help ai --json` | `test/ui/help-ai-json-rpc-leaf-texts.ci` | read: asserts `ze-system:command-help` with `"short-help": "Command name"` and its `description`; not run (ui suite owed) |
| `show yang tree --config \| json` | `test/ui/yang-tree-json-both-texts.ci` | read: asserts `short-help`, `description` and `values[].name` and `short-help` for `asplain` and `asdot+`; not run (ui suite owed) |
| Tab on `set bgp as-notation as` | `test/ui/completion-enum-value-summary.ci` | read: `.et` fixture asserts the dropdown and the hint `An AS number of 65536 or more is X.Y`; not run (ui suite owed) |
| the web config editor form | `TestConfigLeafFormRendersBothTexts` | `handler_config_leaf_test.go:24`, in `r3-web-green.log` |
| the site build | `TestConfigurationReferencePrintsExplanation` | `config_test.go:364`, in `r2-site.log` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | commit b57ec4ab6b swapped the fields; `command.Node.ShortHelp` is the summary and `Description` the explanation on every carrier this diff fills |
| A-2 | confirmed | `TestEnumValueSummariesFollowATypedef` (`enum_test.go`) and `TestEnumValueCompletionShowsDeclaredSummary`; 813/813 enum values with a summary over the checkout |
| A-3 | broken | `addYANGParams` (`tools.go:523-528`) writes `title` and `description` from the two texts, but the MCP specification 2025-06-18 ("Data Types / Tool") defines `title` for the tool only and every property example carries `description` alone; no client documents reading a property `title`. Mistake Log row; `title` stays the summary carrier |
| A-4 | confirmed | `TestExtractRPCsCarriesBothLeafTexts` (`rpc_test.go:129`): input, output and notification leaves through `extractEntryLeaves` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1 feature list: `docs/features.md` Self-Documenting System row names the five carriers | `argDefFor`, `extractEntryLeaves`, `walkYANGEntry`, `EnumValueSummaries` | Yes, at closure |
| 3 CLI: `docs/guide/command-reference.md` | grep `help command --json` finds one prose line, no JSON example to change | No update owed |
| 4 API: `docs/architecture/api/commands.md` argument paragraph and `ArgDef` | `argDefFor`; the readers sentence says four | Yes |
| 6 guide: `docs/guide/config-editor.md`, `docs/guide/api.md`, `docs/guide/mcp/overview.md` | `buildLeafField`, `CommandSchema`, `addYANGParams` | Yes |
| 12 architecture: `yang-config-design.md`, `web-interface.md` | `mergeHelpText`, `commandArguments`, `anchoredDef` | Yes (round 4 read the row against the producers) |
| 16 anchors: `docs/architecture/api/architecture.md` (`buildAPIAuthentication`, `Authentication`), `docs/contributing/gh-pages.md` (`catalogFile`, `loadCommandCatalog`) | the anchored symbols are untouched by the diff | No update owed |
| 17 examples: `docs/guide/api.md` | the JSON examples carry `title` and `description` | Yes |

## Core Insight

The YANG boundary is two calls, `GetHelpExtension` and `entry.Description`, and
a carrier that copies one string is where a text is lost. A struct with one
text field looks complete to every reader built on it, so the defect never
shows as a red; it shows as 692 nodes whose explanation nobody could read. The
gate that judges the CARRIERS (813 enum values, 241 arguments, 285 leaves)
rather than the modules is what makes the loss visible.


## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec
- [ ] **Commit B:** remove the spec (commit A preserves it in history)
