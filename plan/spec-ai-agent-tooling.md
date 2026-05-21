# Spec: AI Agent Tooling

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/11 |
| Updated | 2026-05-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/planning.md` for workflow rules.
3. `ai/rules/json-format.md` for JSON key naming.
4. `ai/rules/derive-not-hardcode.md` for generated command/help data.
5. `ai/patterns/cli-command.md` and `.claude/rules/cli-grammar.md` for command placement.
6. `docs/features/ai-first.md` and `docs/guide/mcp/overview.md` for existing AI surface.
7. `cmd/ze/config/cmd_validate.go`, `cmd/ze/help_ai.go`, and `internal/component/mcp/tools.go` for the current implementation.

## Task

Create Ze-native agent tooling inspired by Zero's agent-facing contracts. The first milestone is a stable diagnostic and repair-planning loop for `ze config validate --json`, plus discoverable machine-readable help and version-matched Ze skills served by the binary.

The goal is to make agents fix Ze config and command usage from structured facts instead of scraping terminal prose. The implementation must reuse Ze's existing registries, YANG metadata, command registry, MCP tool generation, and OpenAPI metadata rather than creating a parallel AI API surface.

## Required Reading

### Architecture Docs

- [ ] `ai/rules/planning.md` - required workflow for writing this spec.
  Decision: This spec stays in `plan/` until approved and no code is implemented from it yet.
  Constraint: One spec at a time per session; user explicitly said not to select this spec.
- [ ] `ai/rules/design-principles.md` - Ze design principles.
  Decision: Add the smallest cross-cutting diagnostic registry needed for stable codes and explanations.
  Constraint: Explicit behavior, exact-or-reject, no hidden magic, no premature abstraction.
- [ ] `ai/rules/json-format.md` - JSON output conventions.
  Decision: New JSON keys use lower kebab-case, including `fix-safety`, `related`, and `diagnostic-code` where needed.
  Constraint: Do not use camelCase in Ze JSON output.
- [ ] `ai/rules/derive-not-hardcode.md` - generated data rule.
  Decision: `ze help --ai --json` must be derived from YANG, command registries, plugin registries, env registries, and MCP grouping helpers.
  Constraint: Do not create a second hand-maintained command or family list.
- [ ] `ai/rules/exact-or-reject.md` - validation and repair safety rule.
  Decision: Repair plans are plan-only and must label unsafe or lossy suggestions instead of applying them.
  Constraint: If Ze cannot prove a repair preserves intent, the plan marks it `requires-human-review`.
- [ ] `ai/patterns/registration.md` - Ze registration architecture.
  Decision: Diagnostic explanations and skills use registration or embedded inventory, not scattered string switches.
  Constraint: Core discovers contributed data through registries or generated inventories.
- [ ] `ai/patterns/cli-command.md` - offline command pattern.
  Decision: New commands are offline CLI commands under `cmd/ze/` with root command registration.
  Constraint: Root command metadata lives in each package `register.go`; `help --ai` consumes the registry.
- [ ] `.claude/rules/cli-grammar.md` - action before identifier.
  Decision: `ze config fix --plan --json <file>`, `ze explain <diagnostic-code>`, and `ze skills get <name>` keep the action before user-supplied identifiers.
  Constraint: Any new command grammar must not put an ID before the action keyword.
- [ ] `docs/features/ai-first.md` - current AI-first documentation.
  Decision: The spec extends the current premise that the binary describes itself.
  Constraint: MCP and CLI remain views of the same command surface.
- [ ] `docs/guide/mcp/overview.md` - MCP command and tool discovery.
  Decision: `ze help --ai --json` should align with what `tools/list` exposes.
  Constraint: MCP tools are auto-generated from the YANG command registry at request time.
- [ ] `docs/architecture/config/yang-config-design.md` - YANG config, validation, completion, command schema.
  Decision: Config diagnostics should carry YANG path, type, expected value, actual value, and validator name when known.
  Constraint: YANG remains the source of truth for schema, validation, completion, and command metadata.
- [ ] `docs/architecture/api/commands.md` - command dispatch and response contracts.
  Decision: Diagnostic and help JSON should describe the same dispatch keys accepted by the daemon.
  Constraint: User-facing commands use verb-first syntax, and identity is injected by trusted transport wiring only.
- [ ] `docs/architecture/api/json-format.md` - existing IPC JSON output.
  Decision: New diagnostic JSON is a CLI JSON contract and must not be confused with plugin IPC events.
  Constraint: Existing event response envelopes remain unchanged.
- [ ] `docs/architecture/api/architecture.md` - command registry, YANG RPCs, and OpenAPI extraction.
  Decision: Help JSON should reuse existing `CommandMeta`, YANG RPC metadata, and OpenAPI-style parameter extraction where possible.
  Constraint: Do not make the API engine depend on concrete dispatcher or plugin packages.

### RFC Summaries

- [ ] No RFC summaries are required. This spec changes local CLI tooling and JSON contracts only.

**Key insights:**
- Ze already has the right sources of truth: YANG modules, command registry, plugin registry, env registry, MCP tool generation, OpenAPI metadata, config parser, and custom validators.
- The main missing contract is stable, structured diagnostics with explanations and repair plans.
- Existing `ze help --ai` is generated from code but is text-only and still has several static prose sections.
- Existing `ze config validate --json` is hand-rendered and carries only `valid`, `path`, `errors`, `warnings`, and optional `config` summary. Current structs: `validationResult`, `validationError` (Line, Message), `validationWarning` (Line, Message).
- Zero's useful pattern is not syntax. It is: (1) schema-versioned JSON envelopes, (2) stable diagnostic codes with structured facts, (3) explain with both text and JSON, (4) plan-only fix with typed repair IDs and safety levels, (5) topic-specific skills served by the binary, (6) command contract snapshot tests preventing drift.
- Zero's skill system bundles 7 inner skills by topic. Agents load only what they need. The outer SKILL.md is a discovery stub; the binary is the source of truth. Ze should replicate this architecture with 4 initial inner skills.
- Zero uses conformance fixtures to pin diagnostic codes to specific error patterns, preventing regression. Ze should do the same for config validation diagnostics.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/help_ai.go` - implements text `ze help --ai` sections for CLI, API, dispatch keys, MCP tools, plugins, families, services, recipes, common errors, and minimal config.
- [ ] `cmd/ze/config/cmd_validate.go` - implements `ze config validate`; `--json` is manually printed and does not include stable codes, spans, expected/actual facts, help text, repair safety, or related diagnostics.
- [ ] `cmd/ze/config/cmd_completion.go` - implements non-interactive completion with JSON output using `encoding/json` over `contract.Completion` objects with `Text`, `Description`, and `Type` fields.
- [ ] `cmd/ze/config/cmd_validate_test.go` - validates exit codes, stdin handling, parsing failures, `ValidateContent`, plugin verifier integration, and semantic warnings.
- [ ] `cmd/ze/config/main.go` - dispatches config subcommands through `subcommandHandlers` and `storageHandlers` maps.
- [ ] `cmd/ze/config/register.go` - registers `config` root metadata and local shortcuts such as `show config dump` and `validate config`.
- [ ] `cmd/ze/cli/main.go` - exposes `WireToPath`, `YANGCommandTree`, and `AllCLIRPCs` for generated help and command tree construction.
- [ ] `internal/component/mcp/tools.go` - groups command metadata into MCP tools, builds JSON Schema, validates generated action names, and dispatches generated tools to the regular command string path.
- [ ] `internal/component/mcp/tools_test.go` - tests grouping, generated tool schemas, action validation, peer validation, task support, UI resource metadata, and generated dispatch.
- [ ] `internal/component/api/schema.go` - generates OpenAPI 3.1 JSON from `CommandMeta` and `ParamMeta`, with a separate YANG type to JSON Schema type mapping.
- [ ] `internal/component/api/schema_test.go` - tests OpenAPI validity, read-only methods, parameter schema generation, operation IDs, and bearer auth metadata.
- [ ] `internal/component/api/types.go` - defines `CommandMeta`, `ParamMeta`, `ExecResult`, and caller identity types.
- [ ] `internal/component/api/engine.go` - lists commands, describes commands, executes commands, and parses JSON output into `ExecResult.Data`.
- [ ] `internal/component/config/parser.go` - parser has token line and column data available, but `errorf` currently formats plain `line N: message` errors.
- [ ] `internal/component/config/tokenizer.go` - token type includes `Line` and `Col`, so parser diagnostics can become column-aware without retokenizing.
- [ ] `internal/component/config/yang/validator.go` - YANG validation already has typed `ValidationError` with `Path`, `Type`, `Message`, `Expected`, `Got`, and `LineNumber`.
- [ ] `internal/component/config/yang/validator_registry.go` - custom validator registry has names, validation functions, and completion functions; missing validators are detected by walking the YANG tree.
- [ ] `internal/component/config/schema.go` - config schema nodes carry type, default, sensitive, enum, range, pattern, backend, and related tool metadata.
- [ ] `docs/features/ai-first.md` - documents current AI-first design, `ze help --ai`, MCP tools, and the CLI-as-API principle.
- [ ] `docs/guide/mcp/overview.md` - documents MCP startup, authentication, generated tools, `ze_execute`, `ze_commands`, and `ze help --ai` flags.
- [ ] `docs/architecture/config/yang-config-design.md` - documents YANG modules, native validation, custom validators, and completion.
- [ ] `docs/architecture/api/commands.md` - documents dispatch commands, response formats, command registry, and MCP task/resource methods.
- [ ] `docs/architecture/api/json-format.md` - documents plugin IPC JSON events and response envelopes.
- [ ] `docs/architecture/api/architecture.md` - documents command dispatch, YANG RPC extraction, OpenAPI, and generated command metadata.
- [ ] `github.com/vercel-labs/zerolang:skills/zero/SKILL.md` - Thin discovery stub. Tells agents to call `zero skills get zero --full` for version-matched content. Does not contain workflows itself.
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-diagnostics.md` - Diagnostic contract: stable codes, spans with `length`, expected/actual facts, help, fix safety levels including `local-edit`, typed repair IDs, related facts with truncation flag, explain text and JSON, plan-only fix.
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-agent.md` - Agent edit loop: read source, smallest change, `zero check --json`, `zero explain` + `zero fix --plan`, add/update tests, validate with narrowest command.
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-builds.md` - Build workflows: `--json` on build/ship/size/doctor, target readiness, profile names, direct emitters, sysroot facts.
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-testing.md` - Test blocks: inline `test` declarations, `--filter`, `--json` structured results, expected-fail markers (`xfail:`).
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-packages.md` - Package system: `zero.json` manifest, `use` imports, local path dependencies, deterministic lock facts.
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-stdlib.md` - Standard library: target-neutral helpers (mem, codec, parse, time, rand, crypto, json, io), hosted capabilities (args, env, fs, net, http, proc), Maybe pattern, owned resource pattern.
- [ ] `github.com/vercel-labs/zerolang:skill-data/zero-language.md` - Language reference: declarations, types, shapes/enums/choices, errors with `raises`, borrowing (ref/mutref), generics with static parameters.
- [ ] `github.com/vercel-labs/zerolang:docs/articles/diagnostics.md` - Full diagnostic reference: JSON shape with `schemaVersion`, 40+ stable codes (PAR/NAM/IMP/PKG/TAR/BLD/ERR/ABI/BOR/OWN/TYP/PUB/MET/IFC/STC/SHM/RCV/FLD/MAT), repair packets, fix safety levels, common repair patterns with code examples.
- [ ] `github.com/vercel-labs/zerolang:docs/articles/cli-reference.md` - Full CLI: 20+ subcommands, `--json` on check/graph/size/build/ship/test/doctor/explain/fix/dev/time/abi/doc/mem, `--full` on skills get, command contract snapshot validation via `pnpm run command-contracts`.
- [ ] `github.com/vercel-labs/zerolang:AGENTS.md` - Contributor guide: breaking changes acceptable for agent-first goals, use `bin/zero` for local work, `pnpm run conformance` and `pnpm run command-contracts` for validation.
- [ ] `github.com/vercel-labs/zerolang:conformance/agent-surface/classification.json` - Agent-surface conformance fixtures: 18 fixtures classified as semantic-rule, intended-behavior, unsupported-language-feature, diagnostic-surface-gap, backend-limitation-reported-late, metadata-drift-risk. Each pinned to an expected diagnostic code and test runner.

**Behavior to preserve:**
- `ze config validate` exit codes remain `0` for valid config, `1` for invalid config, and `2` for missing or unreadable file.
- `ze config validate -q` remains quiet and returns only the exit code.
- `ze config validate --limit environment` keeps its current behavior.
- `ValidateContent` remains a reusable API for static config validation and returns an aggregated error on invalid input.
- Existing top-level `ze config validate --json` fields `valid`, `path`, `errors`, `warnings`, and `config` remain available unless user approves a breaking JSON version.
- Existing text validation output remains human-readable and does not require agents to parse it.
- `ze help --ai` text output remains available and generated from current registries where it already is.
- MCP generated tools continue to use the same command dispatcher and `ze_execute` escape hatch.
- OpenAPI generation remains a transport-neutral API concern and should not import concrete CLI packages.
- Sensitive config values must not be exposed in diagnostics, repair plans, explanations, or help JSON.

**Behavior to change:**
- Add stable diagnostic records to `ze config validate --json` for parse, YANG, semantic, plugin verifier, MCP, BGP resolution, peer construction, hub config, and listener conflict failures. Include `schema-version` in the JSON envelope.
- Add `ze explain <diagnostic-code>` with both text and `--json` output to return version-matched explanations for emitted diagnostic codes.
- Add `ze config fix --plan --json <config-file>` as a plan-only repair planner. It never edits files.
- Add `ze help --ai --json` with a generated machine-readable reference for commands, RPCs, dispatch keys, plugins, families, services, and MCP tool groups.
- Add `ze skills list` and `ze skills get <name> [--full]` so agents can load version-matched Ze guidance from the installed binary. Bundle topic-specific inner skills (`ze-config`, `ze-diagnostics`, `ze-commands`, `ze-agent`).

## External Research Findings

Research based on zerolang.ai source (github.com/vercel-labs/zerolang v0.1.3, May 2026).

| Source | Finding | Decision for Ze |
|--------|---------|-----------------|
| Zero skill stub (`skills/zero/SKILL.md`) | The installed binary is the authority for its own agent workflows. The SKILL.md in the repo is a thin discovery stub that tells agents to call `zero skills get zero --full` for version-matched content. | Ze should serve skills from the running binary. The repo skill file should be a discovery stub only, directing agents to `ze skills get <name>`. |
| Zero inner skills (`skill-data/`) | Zero bundles 7 topic-specific skills: `zero-agent` (edit loop), `zero-language` (syntax), `zero-diagnostics` (check/explain/fix), `zero-builds` (compile/target), `zero-testing` (test blocks), `zero-packages` (modules/manifests), `zero-stdlib` (library usage). Each has frontmatter (`name`, `description`) and focused content. `zero skills list` discovers them; `zero skills get <name>` loads one. | Ze should bundle topic-specific inner skills: `ze-config` (config syntax/validation), `ze-diagnostics` (check/explain/fix workflow), `ze-commands` (CLI and dispatch reference), and `ze-agent` (edit loop). Start with 3-4 skills; more can be added per component. |
| Zero `--full` flag | `zero skills get zero` returns a compact version; `zero skills get zero --full` returns the complete content. Agents pick the depth they need. | `ze skills get <name>` returns compact content; `ze skills get <name> --full` returns expanded content with examples and repair patterns. |
| Zero diagnostics JSON (`docs/articles/diagnostics.md`) | JSON output includes `schemaVersion: 1` at the top level for forward compatibility, plus `ok`, `diagnostics[]`. Each diagnostic includes `length` (token extent in columns) alongside `line`/`column` for exact span highlighting. | Ze diagnostic JSON should include `schema-version: 1` at top level (kebab-case per Ze rules). Add `length` field to spans for token highlighting when available. |
| Zero diagnostic codes | Codes use short uppercase prefixes: `NAM003`, `TAR002`, `PAR100`, `BOR001`. Codes are stable across compiler versions. | Ze uses lower-kebab codes (`config-parse`, `config-yang-type`). This is intentional: Ze diagnostic codes map to config validation domains, not compiler phases. Keep Ze's naming convention. |
| Zero explain `--json` | `zero explain <code>` has both text and `--json` output modes. Agents prefer JSON for programmatic triage; text is for operators. | Add `ze explain --json <code>` alongside text output. Update AC-7 to cover both modes. |
| Zero fix plan safety levels | `zero fix --plan --json` safety levels include `local-edit` (confined to current scope/file) beyond the base set. Repair objects carry stable IDs like `fix-import-path`, `make-binding-mutable`, `manual-review`. Agents learn repair patterns by ID across sessions. | Add `section-local` safety level for repairs confined to one config section. Use descriptive repair IDs: `add-missing-field`, `fix-type-mismatch`, `resolve-listener-conflict`, `fix-peer-reference`, `manual-review`. |
| Zero `related` field structure | Related entries carry spans and structured facts for multi-point diagnostics (e.g., `borrowTrace.activeBorrows` with root, path, kind, binding, declaration range, and a `truncated` flag when the reporting cap is hit). | Ze `related` entries should carry `path`, `line`, `column`, `length`, `message` for multi-point errors like listener conflicts, duplicate peer addresses, or cross-section reference failures. |
| Zero command contract snapshots (`scripts/snapshot-command-contracts.mts`) | Zero validates CLI JSON contracts with snapshot tests that assert schema version, field presence, type invariants, and structural shape across compiler versions. Prevents accidental contract drift. | Ze should add command contract snapshot tests that validate diagnostic JSON shape, help JSON structure, explain output, and skills output format. |
| Zero agent-surface conformance (`conformance/agent-surface/`) | Zero tracks minimized agent-facing repros with a `classification.json` that records fixture ID, expected diagnostic code, classification (semantic-rule, intended-behavior, diagnostic-surface-gap), and test runner type. | Ze should add agent-surface conformance fixtures: config files that produce known diagnostic codes, pinned in a classification file. Prevents diagnostic code drift. |
| Zero ANSI-free defaults | Zero diagnostics do not include ANSI colors, bold, hyperlinks, or OSC escapes by default. These bytes bloat agent context and break diff comparison. | Ze diagnostic text output should not emit ANSI when stdout is not a terminal. This matches Ze's existing TTY detection for most commands. New commands must follow the same convention. |
| Zero agent edit loop (`skill-data/zero-agent.md`) | Zero documents a specific agent workflow: (1) read source, (2) smallest change, (3) `zero check --json`, (4) `zero explain` + `zero fix --plan`, (5) add/update tests, (6) validate with narrowest command. | Ze's bundled `ze-agent` skill should document the equivalent loop: (1) read config, (2) `ze config validate --json`, (3) `ze explain` for unknown codes, (4) `ze config fix --plan --json` for repair candidates, (5) apply fix, (6) re-validate. |
| Zero doctor | `zero doctor --json` checks host and target readiness with a per-target readiness matrix and `targetToolchains`. | Deferred to a separate spec. Ze could benefit from `ze doctor --json` (check interfaces, kernel modules, VPP readiness, service health). |
| Zero graph and size | `zero graph --json` (dependency/structure) and `zero size --json` (artifact size breakdown) give agents cheap inspection facts. | Deferred. Ze's first gap is diagnostics and discoverability. Config dependency graph and service topology could be a future spec. |

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point

| Entry Point | Input | Format |
|-------------|-------|--------|
| `ze config validate --json <file>` | Config file or stdin | Ze config text |
| `ze config fix --plan --json <file>` | Config file or stdin | Ze config text |
| `ze explain [--json] <diagnostic-code>` | Diagnostic code | CLI token string |
| `ze help --ai --json` | Running binary registrations | In-process registries and YANG metadata |
| `ze skills list [--json]` | None | Embedded skill inventory |
| `ze skills get <name> [--full] [--json]` | Skill name | CLI token string and embedded markdown |

### Transformation Path

1. Config validation reads config bytes from file or stdin.
2. Config validation loads the YANG-derived schema through `config.YANGSchema`.
3. The config parser tokenizes and parses input into `config.Tree`, preserving parser warnings.
4. Inactive nodes are pruned before semantic validation.
5. YANG tree validation walks configured sections using `YANGValidatorWithPlugins`.
6. MCP, plugin verifier, BGP resolution, peer extraction, hub config, and listener checks run in the existing `runValidation` order.
7. Each error or warning becomes a diagnostic record with code, severity, message, path, span, expected/actual facts when known, help text, and optional repair metadata.
8. `ze config validate --json` encodes the validation result with `encoding/json`; text output renders the same diagnostic records.
9. `ze config fix --plan --json` runs the same diagnostic collection, looks up repair planners by diagnostic code, and emits plan-only fixes.
10. `ze explain <diagnostic-code>` looks up the code in the diagnostic registry. Text mode renders a human explanation; `--json` mode returns structured explanation with code, title, description, examples, and related codes.
11. `ze help --ai --json` builds one structured reference from YANG command tree, wire-method mapping, builtin RPC registrations, plugin registry, family registry, env registry, and MCP grouping helpers.
12. `ze skills list` enumerates bundled skill names and descriptions. `ze skills get <name>` returns compact skill content; `--full` returns expanded content with examples and repair patterns. `--json` wraps output in a structured envelope.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> config component | `cmd/ze/config` calls parser, YANG schema, validators, plugin verifier, BGP config helpers | [ ] Unit and functional tests exercise `ze config validate --json` |
| Config -> plugin registry | `VerifyPluginConfig` uses registered in-process plugin verifier hooks | [ ] Existing plugin verifier test extended to assert diagnostic code |
| Config -> BGP config | `ResolveBGPTree`, `ValidateAuthzConfig`, and `PeersFromConfigTree` validate BGP semantics | [ ] Unit tests cover mapped BGP diagnostic codes |
| CLI -> diagnostic registry | `ze explain` reads registered diagnostic explanations by stable code | [ ] Functional test for known and unknown code |
| CLI -> YANG command tree | `ze help --ai --json` reads `YANGCommandTree`, `WireToPath`, and RPC metadata | [ ] Unit test asserts generated JSON contains a YANG-derived command |
| CLI -> plugin and family registries | Help JSON lists plugins and families from registries | [ ] Unit test registers test data or asserts existing builtin family output |
| CLI -> embedded skills | Skills command reads bundled skill inventory | [ ] Functional test lists and reads bundled Ze skill |
| MCP helper reuse | Help JSON uses the same grouping semantics as MCP generated tools where possible | [ ] Unit test compares command group names for a sample list |

### Integration Points

- `cmd/ze/config/cmd_validate.go` - central validation flow and JSON/text output.
- `internal/component/config/parser.go` - parser error construction can expose column data already present on tokens.
- `internal/component/config/yang/validator.go` - YANG errors already have structured path/type/expected/got fields.
- `internal/core/diagnostic` - new leaf registry for diagnostic code metadata, explanations, and repair planner metadata.
- `cmd/ze/explain` - new offline command consuming the diagnostic registry.
- `cmd/ze/skills` - new offline command consuming embedded skill inventory.
- `cmd/ze/help_ai.go` - add JSON output and share data extraction with text output where practical.
- `internal/component/mcp/tools.go` - candidate source for shared command grouping logic or a small exported helper to avoid duplicating MCP grouping behavior.
- `internal/component/api/schema.go` - current OpenAPI parameter schema builder is a reference for JSON Schema conversion but must not gain CLI imports.
- `cmd/ze/internal/cmdregistry` - root command metadata source for new `explain` and `skills` commands.

### Architectural Verification

- [ ] No bypassed layers: config diagnostics are produced from existing validation stages, not a separate parser.
- [ ] No unintended coupling: `internal/core/diagnostic` stays a leaf package with generic types and no imports from components.
- [ ] No duplicated functionality: help JSON uses existing registries and YANG metadata instead of static command lists.
- [ ] Zero-copy preserved where applicable: not applicable, this spec does not touch wire UPDATE paths.

## Diagnostic Contract

### Top-Level Envelope

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `schema-version` | integer | Yes | Contract version for forward compatibility. Starts at `1`. |
| `valid` | boolean | Yes | `true` if config is valid (preserves existing field). |
| `path` | string | Yes | Config file path or `"<stdin>"`. |
| `diagnostics` | array | Yes | Array of diagnostic records (may be empty). |
| `errors` | array | Yes | Backward-compatible error list (existing field, enriched). |
| `warnings` | array | Yes | Backward-compatible warning list (existing field, enriched). |
| `config` | object | When valid | Config summary (existing field, unchanged). |

### Diagnostic Record

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `code` | string | Yes | Stable lower-kebab diagnostic code. |
| `severity` | string | Yes | `error` or `warning`. |
| `message` | string | Yes | Short human-readable summary. |
| `path` | string | When known | Config section path or YANG path to the problem. |
| `line` | integer | When known | One-based source line. |
| `column` | integer | When known | One-based source column. |
| `length` | integer | When known | Token extent in characters, for span highlighting. |
| `expected` | value | When known | Expected value, type, range, enum set, or structured constraint. |
| `actual` | value | When known | Actual input value or type. |
| `help` | string | When known | Concise next action for an operator or agent. |
| `fix-safety` | string | When repair exists | Safety label for a proposed repair. |
| `repair` | object | When repair exists | Repair metadata (see Repair Object below). |
| `related` | array | When useful | Related spans or facts (see Related Entry below). |

### Repair Object

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `id` | string | Yes | Stable repair identifier (e.g., `add-missing-field`, `fix-type-mismatch`, `manual-review`). Agents learn patterns by ID. |
| `summary` | string | Yes | One-line description of the candidate repair. |

### Related Entry

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `path` | string | When known | Config path of the related location. |
| `line` | integer | When known | One-based source line. |
| `column` | integer | When known | One-based source column. |
| `length` | integer | When known | Token extent for span highlighting. |
| `message` | string | Yes | What this related location contributes to the diagnostic. |

### Fix Safety Levels

| Fix Safety | Meaning |
|------------|---------|
| `format-only` | Formatting or trivia only. |
| `section-local` | Repair is confined to a single config section. |
| `behavior-preserving` | Intended not to change runtime behavior. |
| `api-changing` | Command or config API shape may change. |
| `target-changing` | Runtime target, backend, or capability use may change. |
| `requires-human-review` | Ze cannot prove the repair is safe. |

### Initial Repair IDs

| Repair ID | Used by codes | Meaning |
|-----------|---------------|---------|
| `add-missing-field` | `config-yang-missing` | Insert a mandatory config field with its default or required value. |
| `fix-type-mismatch` | `config-yang-type`, `config-yang-enum` | Replace the actual value with one matching the expected type or enum set. |
| `fix-range-value` | `config-yang-range`, `config-yang-length` | Adjust a numeric or length value to fall within the allowed range. |
| `fix-pattern-value` | `config-yang-pattern` | Correct a string to match the required YANG pattern. |
| `fix-peer-reference` | `config-bgp-peer`, `config-bgp-authz` | Correct a peer or authz profile cross-reference. |
| `resolve-listener-conflict` | `config-listener-conflict` | Change address or port on one of the conflicting listeners. |
| `manual-review` | Any code | Ze cannot determine a safe repair; operator must decide. |

| Initial Code | Source | Trigger |
|--------------|--------|---------|
| `config-parse` | Config parser | Syntax, unknown keyword, missing token, invalid scalar value. |
| `config-yang-missing` | YANG validator | Missing mandatory field. |
| `config-yang-type` | YANG validator | Wrong scalar type. |
| `config-yang-range` | YANG validator | Numeric value outside allowed range. |
| `config-yang-pattern` | YANG validator | String value does not match a YANG pattern. |
| `config-yang-enum` | YANG validator | Value is not one of the allowed enum values. |
| `config-yang-length` | YANG validator | String length outside allowed range. |
| `config-yang-cardinality` | YANG validator | List or leaf-list cardinality violation. |
| `config-plugin-verify` | Plugin verifier | In-process plugin config verifier rejects config. |
| `config-mcp-invalid` | MCP semantic validation | MCP auth, bind, OAuth, or TLS consistency failure. |
| `config-bgp-resolve` | BGP config resolver | Template or BGP tree resolution failure. |
| `config-bgp-authz` | BGP authz validation | Authz profile reference failure. |
| `config-bgp-peer` | BGP peer extraction | Peer settings, route extraction, or capability constraint failure. |
| `config-hub-invalid` | Hub config extraction | Plugin hub config extraction failure. |
| `config-listener-conflict` | Listener validation | Two listeners conflict on address or port. |
| `config-warning` | Existing semantic warnings | Warning without a narrower code in this milestone. |

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze config validate --json bad.conf` | -> | diagnostic collection in `cmd/ze/config/cmd_validate.go` | `TestValidateJSONDiagnosticsContract` and `test/ui/config-validate-json-diagnostics.ci` |
| `ze config validate --json bad.conf` with plugin verifier | -> | plugin verifier diagnostic mapping | `TestValidatePluginVerifierDiagnostic` |
| `ze config validate --json` with listener conflict | -> | related entries in diagnostic records | `TestValidateListenerConflictRelated` |
| `ze explain config-parse` | -> | diagnostic registry lookup (text) | `TestExplainKnownDiagnostic` and `test/ui/explain-diagnostic.ci` |
| `ze explain --json config-parse` | -> | diagnostic registry lookup (JSON) | `TestExplainKnownDiagnosticJSON` and `test/ui/explain-diagnostic-json.ci` |
| `ze config fix --plan --json bad.conf` | -> | repair planner over diagnostic records | `TestConfigFixPlanJSON` and `test/ui/config-fix-plan-json.ci` |
| `ze help --ai --json` | -> | generated AI reference builder | `TestAIHelpJSONContract` and `test/ui/help-ai-json.ci` |
| `ze skills list` | -> | embedded skill inventory listing | `TestSkillsListAll` and `test/ui/skills-list-get.ci` |
| `ze skills get ze --full` | -> | embedded skill content retrieval | `TestSkillsGetFull` and `test/ui/skills-list-get.ci` |
| `ze skills get ze-diagnostics` | -> | inner skill retrieval | `TestSkillsGetInnerSkill` and `test/ui/skills-list-get.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config validate --json` receives invalid syntax | Output includes `schema-version:1`, `valid:false`, `diagnostics` array with an error diagnostic carrying stable `code`, `severity`, `message`, and `line`; exit code remains 1. |
| AC-2 | `ze config validate --json` receives a YANG validation failure | Diagnostic includes YANG path, YANG-derived type/category, expected/actual facts when available, `length` when token data exists, and a stable `config-yang-*` code. |
| AC-3 | `ze config validate --json` receives valid config with warnings | Exit code remains 0; warnings appear as diagnostics with `severity:warning` without making `valid` false. |
| AC-4 | `ze config validate --json -q` is used | No JSON is printed and exit-code-only behavior is preserved. |
| AC-5 | Existing callers read `errors`, `warnings`, or `config` from validate JSON | Existing top-level fields still exist alongside new `diagnostics` and `schema-version` fields. |
| AC-6 | `ValidateContent` receives invalid config | It returns an aggregated error whose text remains useful, and the underlying diagnostic collection is not duplicated. |
| AC-7 | `ze explain <known-code>` is called (text mode) | It returns a human-readable explanation for the installed binary's diagnostic code and exits 0. |
| AC-7b | `ze explain --json <known-code>` is called | It returns a structured JSON explanation with `code`, `title`, `description`, `examples`, and `related-codes` fields. |
| AC-8 | `ze explain <unknown-code>` is called | It rejects the code, exits nonzero, and suggests valid codes derived from the diagnostic registry. |
| AC-9 | `ze config fix --plan --json` is called on invalid config | It emits a plan-only JSON response with diagnostics and repair candidates carrying stable `repair.id` values; it does not edit the file. |
| AC-10 | A diagnostic has no safe automatic repair | The repair plan either omits fixes or marks the candidate `requires-human-review` with repair ID `manual-review`. |
| AC-11 | `ze help --ai --json` is called | It emits valid JSON with generated command, RPC, dispatch, plugin, family, service, and MCP tool-group sections. |
| AC-12 | New CLI command or plugin data is registered | Help JSON picks it up from registries without adding a second hand-maintained list. |
| AC-13 | `ze skills list` is called | It lists bundled skill names and short descriptions from the installed binary. `--json` wraps output in a structured array. |
| AC-14 | `ze skills get ze` is called | It returns the bundled Ze skill content for this exact binary version. `--full` returns expanded content. |
| AC-14b | `ze skills get ze-diagnostics` is called | It returns the diagnostic workflow skill covering check, explain, and fix-plan usage. |
| AC-15 | Config contains sensitive values | Diagnostics and repair plans do not print sensitive plaintext in `actual`, `expected`, `help`, `repair`, or `related` fields. |
| AC-16 | `ze config validate --json` has a listener conflict | Diagnostic with code `config-listener-conflict` includes `related` entries pointing to both conflicting listener locations with `line`, `path`, and `message`. |
| AC-17 | Text diagnostic output when stdout is not a terminal | Output contains no ANSI escape sequences, color codes, or OSC sequences. |
| AC-18 | Web UI config commit with invalid pending config | Commit is blocked and the response includes diagnostic records from the same validation pipeline. |
| AC-19 | `ze config validate --json --pending` is called | Validates the pending config from zefs storage and returns diagnostic JSON without committing. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiagnosticRegistryKnownCodes` | `internal/core/diagnostic/registry_test.go` | Initial codes are registered with explanation metadata. | |
| `TestDiagnosticRegistryRejectsDuplicate` | `internal/core/diagnostic/registry_test.go` | Duplicate diagnostic code registration fails or panics according to registry policy. | |
| `TestValidateJSONSchemaVersion` | `cmd/ze/config/cmd_validate_test.go` | JSON output includes `schema-version: 1` at top level. | |
| `TestValidateJSONDiagnosticsContract` | `cmd/ze/config/cmd_validate_test.go` | Invalid config JSON includes stable diagnostic records and preserves exit code. | |
| `TestValidateYANGDiagnosticFacts` | `cmd/ze/config/cmd_validate_test.go` | YANG validation maps path, expected, actual, length, and type into diagnostics. | |
| `TestValidatePluginVerifierDiagnostic` | `cmd/ze/config/cmd_validate_test.go` | Plugin verifier failures map to `config-plugin-verify`. | |
| `TestValidateListenerConflictRelated` | `cmd/ze/config/cmd_validate_test.go` | Listener conflict diagnostic includes `related` entries with both listener locations. | |
| `TestValidateQuietSuppressesJSON` | `cmd/ze/config/cmd_validate_test.go` | `-q --json` remains quiet. | |
| `TestValidateTextNoANSI` | `cmd/ze/config/cmd_validate_test.go` | Text output to non-TTY contains no ANSI escape sequences. | |
| `TestConfigFixPlanJSON` | `cmd/ze/config/cmd_fix_test.go` | Plan-only fix output includes diagnostics with repair IDs and does not write the file. | |
| `TestConfigFixPlanRepairIDs` | `cmd/ze/config/cmd_fix_test.go` | Repair objects carry stable IDs from the Initial Repair IDs table. | |
| `TestExplainKnownDiagnostic` | `cmd/ze/explain/explain_test.go` | Known codes render text explanation. | |
| `TestExplainKnownDiagnosticJSON` | `cmd/ze/explain/explain_test.go` | Known codes with `--json` render structured explanation with `code`, `title`, `description`. | |
| `TestExplainUnknownDiagnostic` | `cmd/ze/explain/explain_test.go` | Unknown codes fail with suggestions from registry. | |
| `TestAIHelpJSONContract` | `cmd/ze/help_ai_test.go` | `--ai --json` emits valid generated JSON with expected sections. | |
| `TestAIHelpJSONDerivedCommands` | `cmd/ze/help_ai_test.go` | JSON command list is derived from YANG and command registry. | |
| `TestSkillsListAll` | `cmd/ze/skills/skills_test.go` | `ze skills list` returns all bundled skill names including inner skills. | |
| `TestSkillsGetCompact` | `cmd/ze/skills/skills_test.go` | `ze skills get ze` returns compact skill content. | |
| `TestSkillsGetFull` | `cmd/ze/skills/skills_test.go` | `ze skills get ze --full` returns expanded skill content. | |
| `TestSkillsGetInnerSkill` | `cmd/ze/skills/skills_test.go` | `ze skills get ze-diagnostics` returns the diagnostics workflow skill. | |
| `TestSkillsGetJSON` | `cmd/ze/skills/skills_test.go` | `ze skills list --json` and `ze skills get ze --json` return structured envelopes. | |
| `TestCommandContractSnapshot` | `cmd/ze/config/cmd_validate_test.go` | Diagnostic JSON structure matches pinned contract: field names, types, and required fields. | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Diagnostic `line` | 0 or positive integer, 0 means unknown internally | Maximum parser line in input | Negative forced through test helper | N/A |
| Diagnostic `column` | 0 or positive integer, 0 means unknown internally | Maximum token column in input | Negative forced through test helper | N/A |
| Diagnostic `length` | 0 or positive integer, 0 means unknown or unavailable | Maximum token length in input | Negative forced through test helper | N/A |
| `schema-version` | Positive integer starting at 1 | Current version (1) | 0 rejected by contract test | N/A (future versions increment) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-validate-json-diagnostics` | `test/ui/config-validate-json-diagnostics.ci` | Agent runs config validation and receives stable diagnostic JSON with `schema-version`, `diagnostics`, and backward-compatible fields. | |
| `config-validate-json-related` | `test/ui/config-validate-json-related.ci` | Agent validates a config with listener conflict and receives `related` entries pointing to both locations. | |
| `config-fix-plan-json` | `test/ui/config-fix-plan-json.ci` | Agent asks for a repair plan with stable repair IDs and verifies no file edit occurred. | |
| `help-ai-json` | `test/ui/help-ai-json.ci` | Agent requests machine-readable help from the binary. | |
| `explain-diagnostic` | `test/ui/explain-diagnostic.ci` | Agent asks for text explanation of a diagnostic code. | |
| `explain-diagnostic-json` | `test/ui/explain-diagnostic-json.ci` | Agent asks for JSON explanation with structured `code`, `title`, `description`, `related-codes`. | |
| `skills-list-get` | `test/ui/skills-list-get.ci` | Agent discovers bundled skills, reads compact and `--full` content, and reads an inner skill (`ze-diagnostics`). | |
| `command-contract-snapshot` | `test/ui/command-contract-snapshot.ci` | Validates that diagnostic JSON, explain JSON, and skills JSON match their pinned contract shapes across builds. | |

### Future (if deferring any tests)

- No test deferrals are planned for this spec.

## Files to Modify

- `cmd/ze/config/cmd_validate.go` - produce diagnostic records, use `encoding/json`, preserve existing exit codes and top-level JSON fields.
- `cmd/ze/config/main.go` - add `fix` subcommand dispatch.
- `cmd/ze/config/register.go` - include `fix` in root metadata if implemented as a `ze config` subcommand.
- `cmd/ze/help_ai.go` - add `--json`, share generated data extraction with existing text output where practical.
- `cmd/ze/main.go` - add dispatch for new root commands if the existing registry fallback does not cover them.
- `internal/component/config/parser.go` - expose structured parser errors with line and column, or wrap existing errors with diagnostic metadata.
- `internal/component/config/yang/validator.go` - preserve structured YANG validation facts for diagnostic mapping.
- `internal/component/mcp/tools.go` - extract or expose grouping helpers only if needed to prevent duplicating MCP grouping logic in help JSON.
- `internal/component/api/schema.go` - no direct dependency on CLI; only touch if sharing a leaf JSON-schema helper is necessary.
- `docs/features/ai-first.md` - document diagnostic JSON, explain, repair plan, help JSON, and skills.
- `docs/guide/mcp/overview.md` - document how MCP users discover help JSON and skills.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | Offline commands only. |
| CLI commands/flags | Yes | `cmd/ze/config/*`, `cmd/ze/explain/*`, `cmd/ze/skills/*`, `cmd/ze/help_ai.go` |
| Editor autocomplete | No | New commands are offline process commands, not config editor commands. |
| Functional test for new RPC/API | Yes | `test/ui/*.ci` for end-user CLI behavior. |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/ai-first.md` |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/mcp/overview.md` and generated help output |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/features/ai-first.md` |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` only if `internal/core/diagnostic` becomes a new cross-cutting core registry. |

## Files to Create

- `internal/core/diagnostic/registry.go` - leaf registry for diagnostic code metadata and explanation text.
- `internal/core/diagnostic/types.go` - diagnostic record and repair-plan data types if they are shared outside config validation.
- `internal/core/diagnostic/registry_test.go` - registry tests.
- `cmd/ze/config/cmd_fix.go` - `ze config fix --plan --json` command.
- `cmd/ze/config/cmd_fix_test.go` - fix-plan tests.
- `cmd/ze/explain/main.go` - `ze explain` command.
- `cmd/ze/explain/register.go` - root command registration.
- `cmd/ze/explain/explain_test.go` - explain command tests.
- `cmd/ze/skills/main.go` - skills command.
- `cmd/ze/skills/register.go` - root command registration.
- `cmd/ze/skills/skills_test.go` - skills tests.
- `cmd/ze/skills/data/ze.md` - bundled Ze skill content (compact discovery stub).
- `cmd/ze/skills/data/ze-full.md` - expanded Ze skill content (served by `--full`).
- `cmd/ze/skills/data/ze-diagnostics.md` - bundled diagnostic workflow: check, explain, fix-plan usage.
- `cmd/ze/skills/data/ze-config.md` - config syntax, validation, and common error patterns.
- `cmd/ze/skills/data/ze-commands.md` - CLI command reference, dispatch keys, and MCP tool groups.
- `cmd/ze/skills/data/ze-agent.md` - agent edit loop: validate, explain, fix-plan, apply, re-validate.
- `test/ui/config-validate-json-diagnostics.ci` - validation JSON functional test.
- `test/ui/config-validate-json-related.ci` - listener conflict `related` entries test.
- `test/ui/config-fix-plan-json.ci` - fix-plan functional test.
- `test/ui/help-ai-json.ci` - help JSON functional test.
- `test/ui/explain-diagnostic.ci` - explain text functional test.
- `test/ui/explain-diagnostic-json.ci` - explain JSON functional test.
- `test/ui/skills-list-get.ci` - skills list, get, get --full, and inner skill functional test.
- `test/ui/command-contract-snapshot.ci` - contract shape validation functional test.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a self-critical review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - add command skeletons and failing wiring tests.
   Tests: wiring tests from the Wiring Test table.
   Files: command registration files, command skeleton files, `.ci` test skeletons.
   Verify: commands are reachable and tests fail because feature logic is not implemented yet.
2. **Phase: Diagnostic Core** - add diagnostic registry, code metadata, explanation lookup, and shared diagnostic record types.
   Tests: `TestDiagnosticRegistryKnownCodes`, `TestDiagnosticRegistryRejectsDuplicate`.
   Files: `internal/core/diagnostic/*`.
   Verify: registry tests fail first, then pass.
3. **Phase: Config Validate Diagnostics** - map existing validation stages to diagnostics and preserve text/JSON compatibility.
   Tests: validate JSON and plugin verifier tests.
   Files: `cmd/ze/config/cmd_validate.go`, parser/YANG helpers if required.
   Verify: invalid syntax, YANG error, warning-only, quiet mode, and plugin verifier paths pass.
4. **Phase: Explain and Fix Plan** - implement `ze explain` (text and `--json`) and plan-only `ze config fix --plan --json`.
   Tests: explain text, explain JSON, fix-plan unit plus functional tests.
   Files: `cmd/ze/explain/*`, `cmd/ze/config/cmd_fix.go`.
   Verify: known codes explain in both modes, unknown codes reject with suggestions, repair plan carries stable IDs and does not edit input.
5. **Phase: Help JSON** - add `ze help --ai --json` from existing registries and shared builders.
   Tests: `TestAIHelpJSONContract`, `TestAIHelpJSONDerivedCommands`, `test/ui/help-ai-json.ci`.
   Files: `cmd/ze/help_ai.go`, optional shared helpers.
   Verify: JSON is valid, keys are kebab-case, command data is derived.
6. **Phase: Bundled Skills** - implement `ze skills list [--json]` and `ze skills get <name> [--full] [--json]` with embedded skill content. Bundle 4 inner skills: `ze-config`, `ze-diagnostics`, `ze-commands`, `ze-agent`.
   Tests: `TestSkillsListAll`, `TestSkillsGetCompact`, `TestSkillsGetFull`, `TestSkillsGetInnerSkill`, `TestSkillsGetJSON`, `test/ui/skills-list-get.ci`.
   Files: `cmd/ze/skills/*`, embedded skill markdown files under `cmd/ze/skills/data/`.
   Verify: skills list shows all 5 skills (ze + 4 inner), compact/full modes return different content, inner skills are retrievable, JSON output is valid.
7. **Phase: Command Contract Snapshots** - add contract snapshot tests that pin diagnostic JSON structure, explain JSON structure, and skills JSON structure. Add agent-surface conformance fixtures with classification.
   Tests: `TestCommandContractSnapshot`, `test/ui/command-contract-snapshot.ci`.
   Verify: contracts match pinned shapes; adding a field does not break existing assertions.
8. **Functional tests** - create and run process-level `.ci` tests for each new user path.
9. **Documentation** - update AI-first and MCP guide docs.
10. **Full verification** - run `make ze-verify` before completion.
11. **Complete spec** - fill audit tables, write learned summary, and delete this spec from `plan/` only when implementation is complete.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-17 has a test and implementation evidence. |
| Correctness | Diagnostics map to the stage that actually produced the error; no generic catch-all hides specific facts when structured data exists. |
| Naming | JSON keys are lower kebab-case; diagnostic codes are stable lower kebab-case; repair IDs are stable lower-kebab. |
| Data flow | Validation still runs through existing parser, YANG validation, semantic checks, plugin verifier, and listener checks. |
| Schema version | `schema-version: 1` present in every JSON envelope (validate, fix-plan). |
| Rule: derive-not-hardcode | Help JSON command, RPC, plugin, family, env, and MCP sections are generated from registries or YANG. |
| Rule: exact-or-reject | Fix plan never edits; unsafe repairs are omitted or marked `requires-human-review` with repair ID `manual-review`. |
| Contract stability | Command contract snapshot tests pin field names, types, and required fields for diagnostic, explain, and skills JSON. |
| Security | Sensitive values are redacted from diagnostic `actual`, `expected`, `help`, repair, and related fields. |
| ANSI-free | Text output to non-TTY stdout contains no ANSI escape sequences. |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Stable diagnostic registry exists | Read `internal/core/diagnostic/registry.go` and run registry tests. |
| `ze config validate --json` emits diagnostics | Run unit test and functional test named in Wiring Test. |
| Existing validation exit codes preserved | Run existing `cmd_validate_test.go` tests. |
| `ze explain` works | Run explain unit and functional tests. |
| `ze config fix --plan --json` works and does not edit files | Run fix-plan tests and inspect fixture before/after assertions. |
| `ze help --ai --json` works | Run help JSON tests and inspect generated sections. |
| `ze skills list/get` works | Run skills tests. |
| Docs updated | Read `docs/features/ai-first.md` and `docs/guide/mcp/overview.md`. |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Sensitive value leakage | Diagnostics must not include decoded `ze:sensitive` or `ze:bcrypt` values in `actual`, `help`, repair plans, or related facts. |
| JSON injection | All JSON output uses `encoding/json`, not string concatenation. |
| Command injection | Repair plan commands are data, not shell strings to execute automatically. |
| Untrusted file paths | Validation and fix plan read only the requested file or stdin and never write in plan mode. |
| Resource exhaustion | Diagnostic collection is bounded by existing validation work and does not retain full config copies in every diagnostic. |
| Registry integrity | Duplicate diagnostic codes and duplicate skill names are rejected at init or test time. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it. |
| Test fails wrong reason | Fix test assertion or setup. |
| Test fails behavior mismatch | Re-read Current Behavior and adjust design only with user approval. |
| Lint failure | Fix inline; if architectural, return to design. |
| Functional test fails | Check AC; if AC is wrong, return to design; if AC is correct, implement. |
| Audit finds missing AC | Return to relevant phase and implement. |
| 3 fix attempts fail | Stop, report all 3 approaches, and ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `tmp/session/selected-spec` could be updated for this new spec. | User explicitly said not to select the spec. | User message during drafting. | This spec must be created without touching `tmp/session/selected-spec`. |
| `spec-arch-3-remote-creds.md` might be an active missing dependency. | It was a stale selected-spec leftover from a closed spec. | Search found learned summary and no live spec. | Do not treat it as blocking this spec. |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Selecting this new spec in `tmp/session/selected-spec`. | User explicitly forbade selecting the spec. | Create only `plan/spec-ai-agent-tooling.md`. |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| None yet. | N/A | N/A | N/A |

## Design Insights

- Ze already has token column data in `internal/component/config/tokenizer.go`; the parser loses it only when formatting plain errors. Adding `length` alongside `line`/`column` enables exact span highlighting in editors and agent UIs.
- YANG validation already has structured facts (`ValidationError` with `Path`, `Type`, `Message`, `Expected`, `Got`, `LineNumber`); diagnostic mapping should preserve those fields instead of reparsing error strings.
- `ze config completion --json` currently emits Go field names from `contract.Completion`; this spec should not copy that pattern for new JSON contracts because current JSON rules require kebab-case.
- `cmd/ze/help_ai.go` contains a mix of generated sections and static prose sections; JSON output should start with generated sections only and avoid pretending static recipes are a stable machine contract.
- MCP and OpenAPI already contain separate YANG type to JSON Schema mappings; any shared helper must stay in a leaf package to avoid coupling the API engine to CLI or MCP.
- Repair plans must be conservative. A useful empty plan is better than a confident lossy edit.
- Zero's `schemaVersion` field is critical for forward compatibility: agents check it before parsing. Ze should include `schema-version: 1` from the start so future changes can bump the version without breaking existing parsers.
- Zero's skill architecture uses a thin SKILL.md as a discovery stub, with the binary serving version-matched content. This means the repo file never goes stale relative to the installed binary. Ze should follow the same pattern.
- Zero's inner skills are organized by concern (language, diagnostics, builds, testing, packages, stdlib, agent), each under 200 lines. This lets agents load only what they need for the current task. Ze should start with 4 inner skills and grow organically.
- Zero's command contract snapshot testing pins JSON field names, types, and structural invariants. This prevents accidental contract drift during refactors. Ze should add this as a test category alongside existing functional tests.
- Zero's agent-surface conformance fixtures classify diagnostic behavior into categories: semantic-rule, intended-behavior, diagnostic-surface-gap, backend-limitation-reported-late. Ze should track config diagnostic fixtures similarly, pinning each to an expected diagnostic code.
- Zero's `related` field serves multi-point diagnostics (borrow conflicts, import cycles). Ze's equivalent is listener conflicts, duplicate peer addresses, and cross-section reference failures. Defining the `related` entry shape upfront avoids ad-hoc structures per diagnostic code.
- Stable repair IDs let agents build pattern libraries across sessions. An agent that sees `add-missing-field` on `config-yang-missing` in one config can reuse that knowledge in another. The IDs should be descriptive enough to be self-documenting.

## RFC Documentation

No RFC comments are required. This spec does not implement protocol behavior.

## Implementation Summary

### What Was Implemented

- Not implemented yet. This spec is in design.

### Bugs Found/Fixed

- Not implemented yet.

### Documentation Updates

- Not implemented yet.

### Deviations from Plan

- Not implemented yet.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan

| File | Status | Notes |
|------|--------|-------|

### Audit Summary

- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

- None yet.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Pre-Spec Verification

| Item | Evidence |
|------|----------|
| Metadata table present | Present immediately after title. |
| INDEX.md checked | `ai/INDEX.md` read; relevant docs selected from keyword table. |
| RFC summaries checked | No protocol RFCs referenced. |
| Template format followed | Template sections preserved; `🧪 TDD Test Plan` present. |
| Checkboxes use `[ ]` | Template markers use `[ ]`, not completed checkboxes. |
| No code snippets | Spec uses prose and tables only. |
| Files to Modify includes feature code | Includes `cmd/ze/*`, `internal/core/diagnostic`, and docs. |
| Current Behavior completed | Source files and behavior preservation listed. |
| Data Flow completed | Entry points, transformation path, boundaries, and integration points listed. |
| AC rows testable | AC-1 through AC-15 define observable outcomes. |
| Required Reading has decisions and constraints | Every required reading row includes both. |
| Research findings captured | Zero source (github.com/vercel-labs/zerolang v0.1.3) read in depth: SKILL.md, 7 skill-data files, diagnostics.md, cli-reference.md, AGENTS.md, agent-surface/classification.json, command-contracts script. Findings captured in External Research Findings (14 rows) and Design Insights (13 bullets). |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-17 (including AC-7b, AC-14b) all demonstrated
- [ ] Wiring Test table complete, every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean, Review Gate section filled with 0 BLOCKER and 0 ISSUE
- [ ] `make ze-test` passes
- [ ] Feature code integrated under `internal/*` and `cmd/*`
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated if `internal/core/diagnostic` is created
- [ ] Critical Review passes

### Quality Gates (SHOULD pass, defer with user approval)

- [ ] RFC constraint comments added if protocol code is unexpectedly touched
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit behavior
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL, paste output
- [ ] Tests PASS, paste output
- [ ] Boundary tests for numeric fields
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING, before ANY commit)

- [ ] Critical Review passes, all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial or skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ai-agent-tooling.md`
- [ ] Summary included in commit
