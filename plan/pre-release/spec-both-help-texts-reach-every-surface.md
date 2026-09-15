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
| Daemon to site build | `show yang tree --config \| json` (`runYANGConfigTree`) | No |
| Daemon to API client | `ze help ai --json`, `CommandSchema` | No |
| Hub to MCP client | tool `inputSchema` | No |
| Hub to browser | templ views | No |

### Integration Points
- `argDefFor` - fills the two new `ArgDef` fields from the leaf entry.
- `ExtractRPCs` - fills `LeafMeta.Description` from the leaf entry.
- `buildLeafField` - renders the explanation under the input, the summary in the tooltip.
- `writeConfigChildMirror` - prints the explanation as a paragraph under the node line.
- value completion `Yenum` branch - reads the declared summary through the same walk `getListKeyEntry` uses.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The rename commit lands first: `ShortHelp` is the summary and `Description` the explanation on every downstream struct | owner directive 2026-09-15 | this spec's field names are wrong | `gopls references` on `command.Node.ShortHelp` before implementation | unvalidated |
| A-2 | The goyang AST `*yang.Enum` keeps its `Extensions` after the loader resolves the module, with the keyword `ze:help` or `<prefix>:help` | `getListKeyEntry` reads it today | enum summaries cannot be read from the resolved tree | `TestEnumValueCompletionShowsDeclaredSummary` | unvalidated |
| A-3 | JSON Schema `title` is read by MCP clients as the short label | JSON Schema core vocabulary | clients show only `description` | inspection of one MCP client, recorded in Design Insights | unvalidated |
| A-4 | A leaf under an rpc `input` reaches `LeafMeta` through `extractLeaves` for every rpc the hub serves | `rpc.go` | some API leaves keep one text | `TestExtractRPCsCarriesBothLeafTexts` | unvalidated |

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
| AC-5 | A node declares one text only | every surface prints the one it has and an empty string for the other; nothing derives one from the other |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Every carrier holds the pair as two fields | one field holding "summary. explanation" joined | a reader that needs one text would have to split a string; the YANG declares two, so the carrier does |
| JSON Schema maps summary to `title`, explanation to `description` | the explanation alone in `description` | JSON Schema defines `title` as the short label; both texts reach the client and neither is dropped |
| Enum value summaries are resolved from the AST once at load | walking the AST on every completion | completion runs on every keystroke; `getListKeyEntry` already pays the walk once |
| `AnalysisNode` gains `Values` with name and summary | a separate values command | the site reads one JSON tree; a second command is a second artifact to keep fresh |

## Known Limitations
- The explanation of a `-cmd` verb container that another module owns is filled at merge and stays a single text per merged node, as today.

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
