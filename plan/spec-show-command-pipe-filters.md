# Spec: show-command-pipe-filters

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-05-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/planning.md` for spec workflow.
3. `ai/patterns/cli-command.md` for online command structure.
4. `ai/rules/cli-grammar.md` for action before identifier.
5. `ai/rules/pipe-completeness.md` for global pipe behavior.
6. `ai/rules/derive-not-hardcode.md` for registry-derived completion and help.
7. `internal/component/command/pipe.go` for current generic pipe parsing and formatting.
8. `internal/component/command/completer.go` for current global pipe completion.
9. `internal/component/bgp/plugins/rib/rib_pipeline.go` and `rib_pipeline_best.go` for current RIB iterator pipelines.
10. `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` for current peer and summary command tree.
11. `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` for current `show bgp rib` tree.

## Task

Redesign Ze's BGP operational show surface and pipe filtering so the command grammar follows Ze conventions and large route outputs are filtered while being generated.

The intended user-facing model is:

| Intent | Target Surface |
|--------|----------------|
| BGP summary | `show summary` |
| Peer list | `show peer list` |
| Peer detail | `show peer detail <selector>` |
| Peer capabilities | `show peer capabilities <selector>` |
| Peer statistics | `show peer statistics <selector>` |
| Peer history | `show peer history <selector>` |
| BGP RIB routes | `show bgp rib` |
| Received BGP routes | `show bgp rib | received` |
| Advertised BGP routes | `show bgp rib | advertised` |
| Best BGP routes | `show bgp rib best` |
| RIB status | `show bgp rib status` |

Command-specific pipe filters, such as `received`, `advertised`, `peer`, `family`, `prefix`, `path`, and `community`, must be registered by the command that accepts them. The generic pipe framework keeps owning generic pipes such as `match`, `count`, `json`, `ndjson`, `table`, `text`, `yaml`, `resolve`, `origin`, `log`, and `no-more`.

The route command must stream data through selection and formatting. It must not build the complete RIB output in memory and then discard most of it after client-side filtering.

No implementation type shape is mandated by this spec. The implementation must choose an internal representation that satisfies the functional contract and keeps command-specific knowledge out of generic pipe code.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/api/commands.md` - command verb taxonomy and current API command conventions.
  -> Decision: user-facing read-only commands use the `show` verb.
  -> Constraint: legacy noun-first commands may remain internal dispatch, but user-facing commands must be verb-first.
- [ ] `docs/architecture/cli/plugin-modes.md` - plugin command modes and registration boundaries.
  -> Decision: command handlers remain responsible for domain behavior, including domain-specific route filters.
  -> Constraint: generic CLI code must not import or encode BGP plugin behavior.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage and iterator expectations.
  -> Decision: RIB route show operations must use iterator-style generation and apply filters before serialization.
  -> Constraint: no full route materialization for filtering where an iterator can decide earlier.

### Rules and Patterns

- [ ] `ai/patterns/registration.md` - registration architecture.
  -> Decision: command-specific pipe filters are registered metadata, not hardcoded pipe behavior.
  -> Constraint: help, completion, validation, and handler dispatch must derive from the same registration source.
- [ ] `ai/patterns/cli-command.md` - online command structure.
  -> Decision: YANG command tree must mirror user-facing command paths.
  -> Constraint: online command registration still maps YANG command paths to RPC handlers.
- [ ] `ai/rules/cli-grammar.md` - action before identifier.
  -> Decision: peer introspection becomes `show peer detail <selector>`, not `show peer <selector> detail`.
  -> Constraint: identifiers appear only after an action keyword.
- [ ] `ai/rules/pipe-completeness.md` - pipe completeness.
  -> Decision: generic pipes remain available for every output-producing command.
  -> Constraint: command-specific filters are additive to, not replacements for, generic pipe support.
- [ ] `ai/rules/derive-not-hardcode.md` - derive, never hardcode.
  -> Decision: pipe autocomplete must derive command-specific filters from command metadata.
  -> Constraint: no duplicate filter lists in pipe code, completion code, docs, and handlers.
- [ ] `ai/rules/data-flow-tracing.md` - data flow tracing.
  -> Decision: the spec must trace command input to route stream to formatter.
  -> Constraint: data should not bypass command registration or RIB iterator layers.

### RFC Summaries

- [ ] None required for this spec.
  -> Constraint: this spec changes CLI and data-flow behavior, not BGP wire protocol behavior.

**Key insights:**
- The generic pipe framework owns global pipe operators.
- Command handlers own command-specific pipe filters.
- Completion after a pipe must combine generic pipe operators with command-specific filters for the already parsed base command.
- RIB route output must be streamed through source selection, filters, and formatting.
- `match` is a generic grep-like pipe. It should work for every output command and must not become a BGP-specific filter.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe.go` - parses pipe segments, applies generic pipes, and currently contains an incorrect hardcoded RIB folding path in this worktree.
  -> Constraint: remove BGP command-name knowledge from this generic file during implementation.
- [ ] `internal/component/command/completer.go` - `PipeOperators` and `CompletePipe` currently provide global pipe autocomplete only.
  -> Constraint: generic pipe completion must remain global; command-specific completions must be added from command metadata.
- [ ] `internal/component/command/node.go` - command tree nodes currently carry command path metadata, argument definitions, dynamic children, and value hints.
  -> Constraint: command-specific pipe metadata needs to attach to command metadata without duplicating handler-specific lists.
- [ ] `internal/component/config/yang/command.go` - builds the command tree from `-cmd` YANG modules, extracts wire methods, descriptions, task support, backend constraints, and argument definitions.
  -> Constraint: if command-specific pipe metadata is represented in command schemas, it must be extracted here and projected into command nodes.
- [ ] `internal/component/plugin/server/rpc_register.go` - stores RPC registrations added by command packages at init time.
  -> Constraint: command-specific pipe filter registration may attach to this registration path or a neighboring registry, but must remain command-owned.
- [ ] `internal/component/plugin/server/command.go` - loads YANG paths into dispatcher commands and validates YANG-derived command arguments.
  -> Constraint: dispatch must receive command-specific pipe selections before handler output generation when the command supports streaming filters.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - current RIB show pipeline has scope keywords `sent`, `received`, and `sent-received`, filter stages, and terminals, then returns JSON strings.
  -> Constraint: preserve the existing RIB iterator filtering behavior, but expose user-facing `advertised` rather than `sent`.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - current best-path pipeline supports filters and a `reason` terminal.
  -> Constraint: preserve best-path filters and `reason`, but register them as command-specific pipe filters for `show bgp rib best`.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - native RIB plugin commands include `bgp rib show` and `bgp rib show best`.
  -> Constraint: native internal commands may remain implementation details, but user-facing paths must use `show ...`.
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - CLI proxy maps RIB API commands to native plugin command strings.
  -> Constraint: proxy forwarding must pass command-specific selections without reconstructing ad-hoc command strings that lose streaming semantics.
- [ ] `internal/component/bgp/plugins/cmd/rib/schema/ze-rib-cmd.yang` - current `rib routes`, `rib best`, and status command tree.
  -> Constraint: target user-facing command set should avoid exposing noun-first RIB routes as the primary path.
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - current `show bgp rib`, `show bgp rib best`, and status aliases.
  -> Constraint: this is the right user-facing tree for RIB show commands.
- [ ] `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - current top-level `summary` and `peer list/detail/capabilities/statistics` command tree.
  -> Constraint: migrate read-only peer and summary commands to `show summary` and `show peer ...`.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - current peer handlers register `peer list`, `peer detail`, and `show bgp peer` paths.
  -> Constraint: existing peer selector behavior should be preserved while changing user-facing grammar.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - current BGP summary handler supports optional family filtering and returns aggregate plus peer rows.
  -> Constraint: preserve summary output shape and family filtering unless the command-specific pipe filter design explicitly replaces the family argument.

**Behavior to preserve:**
- Generic pipes work on output-producing commands: `match`, `count`, display formats, `resolve`, `origin`, `log`, and `no-more`.
- `| match <text>` remains a generic grep-like pipe for all output commands.
- RIB filters currently available internally, including received routes, sent routes, family, prefix, AS path, community, match, count, prefix summary, graph, and best-path reason, remain available where applicable after grammar cleanup.
- Peer selectors continue to accept wildcard, IP address, peer name, and ASN selector semantics where currently supported.
- Existing JSON keys remain kebab-case and typed.

**Behavior to change:**
- User-facing BGP summary becomes `show summary`.
- User-facing read-only peer commands become `show peer ...`.
- User-facing RIB show commands use `show bgp rib` and `show bgp rib best`.
- `bgp rib show received` is no longer the user-facing grammar.
- `received` and `advertised` are command-specific pipe filters for `show bgp rib`.
- Generic pipe code must not hardcode BGP command names or BGP filter names.
- RIB route output must stream through generation, filtering, and formatting without materializing the full output just to filter it.

## Vendor And Current Command Comparison

| Intent | Ze Current | Ze Target | Junos | Nokia SR OS | Cisco IOS/XE | VyOS |
|--------|------------|-----------|-------|-------------|--------------|------|
| BGP summary | `summary` | `show summary` | `show bgp summary` | `show router bgp summary` | `show ip bgp summary` | `show bgp summary` |
| Peer list | `peer list` | `show peer list` | `show bgp neighbor` | `show router bgp neighbor` | `show ip bgp neighbors` | `show bgp neighbors` |
| Peer detail | `peer detail`, `show bgp peer` | `show peer detail <selector>` | `show bgp neighbor <peer>` | `show router bgp neighbor <peer>` | `show ip bgp neighbors <peer>` | `show bgp neighbors <peer>` |
| Peer capabilities | `peer capabilities` | `show peer capabilities <selector>` | neighbor detail | neighbor detail | neighbor detail | neighbor detail |
| Peer statistics | `peer statistics` | `show peer statistics <selector>` | neighbor counters | neighbor counters | neighbor counters | statistics variants |
| Peer history | `show bgp peer-history` | `show peer history <selector>` | No direct match verified | No direct match verified | No direct match verified | No direct match verified |
| All RIB routes | `show bgp rib`, `rib routes`, `bgp rib show` | `show bgp rib` | `show route protocol bgp` | `show router bgp routes` | `show ip bgp` | `show bgp` |
| Received routes | `bgp rib show received` | `show bgp rib | received` | `show route receive-protocol bgp <peer>` | neighbor received-routes | neighbor received-routes | neighbor received-routes |
| Advertised routes | `bgp rib show sent` | `show bgp rib | advertised` | `show route advertising-protocol bgp <peer>` | neighbor advertised-routes | neighbor advertised-routes | neighbor advertised-routes |
| Best routes | `show bgp rib best`, `rib best`, `bgp rib show best` | `show bgp rib best` | `show route protocol bgp` | `show router bgp routes` | `show ip bgp` | `show bgp` |
| Prefix filter | internal `prefix` pipeline arg | `show bgp rib | prefix <prefix>` | route prefix lookup | route prefix filters | `show ip bgp <prefix>` | prefix forms |
| AS path filter | internal `path` and `aspath` pipeline args | `show bgp rib | path <pattern>` | `show route aspath-regex` | `routes aspath-regex` | `show ip bgp regexp` | `show bgp regexp` |
| Community filter | internal `community` pipeline arg | `show bgp rib | community <value>` | `show route community` | `routes community` | `show ip bgp community` | `show bgp community` |
| Text grep | generic `| match` exists | generic `| match <text>` | implementation-specific pipes | implementation-specific pipes | implementation-specific filtering | VyOS-style pipe behavior |

Vendor command rows are normalized by intent. This spec does not attempt to clone vendor spelling. Ze keeps a smaller command surface and uses registered command-specific pipe filters for route selection.

## Target Command Surface

### Summary And Peer Introspection

| Command | Purpose | Notes |
|---------|---------|-------|
| `show summary` | BGP summary with aggregate and peer rows | Replaces current user-facing `summary` path. |
| `show peer list` | Brief peer list | Replaces current user-facing `peer list` path. |
| `show peer detail <selector>` | Detailed peer state for selected peers | Action keyword `detail` precedes identifier. |
| `show peer capabilities <selector>` | Negotiated capabilities for selected peers | Action keyword precedes selector. |
| `show peer statistics <selector>` | Per-peer update counters and rates | Action keyword precedes selector. |
| `show peer history <selector>` | FSM transition history | Replaces `show bgp peer-history` as the primary path. |

### BGP RIB Commands

| Command | Purpose | Command-Specific Pipe Filters |
|---------|---------|-------------------------------|
| `show bgp rib` | Stream BGP RIB routes | `received`, `advertised`, `peer`, `family`, `prefix`, `path`, `community`, `count`, `prefix-summary`, `graph` |
| `show bgp rib best` | Stream best-path entries | `peer`, `family`, `prefix`, `path`, `community`, `count`, `prefix-summary`, `graph`, `reason` |
| `show bgp rib status` | RIB status summary | None unless explicitly added later. Generic pipes still apply. |
| `show bgp rib best status` | Best-path computation status | None unless explicitly added later. Generic pipes still apply. |

`advertised` is the user-facing name for Adj-RIB-Out route selection. Internal code may still call this sent routes, but the operator-facing filter name is `advertised`.

## Command-Specific Pipe Filter Contract

This spec defines behavior, not a Go type shape.

| Requirement | Contract |
|-------------|----------|
| Ownership | A command registers only the filters it accepts. |
| Generic pipes | The generic pipe framework registers global pipes once. |
| Completion | Completion after a pipe combines global pipes with filters registered by the base command. |
| Validation | Unknown command-specific filters are rejected using the base command's registered filter set. |
| Dispatch | Accepted command-specific filters reach the handler before output generation starts. |
| Streaming | A handler that declares streaming filters applies them while iterating data. |
| Help | Command help can list command-specific filters from the same metadata used by completion and validation. |
| No hardcoding | Generic pipe code must not contain command names such as BGP RIB commands. |

### Generic Pipe Operators

| Pipe | Owner | Applies To | Streaming Requirement |
|------|-------|------------|-----------------------|
| `match <text>` | Pipe framework | Rendered output lines | Must be able to filter line-by-line for streaming commands. |
| `first <n>` | Pipe framework by default, command-specific override allowed | Rendered output lines | Generic: truncate after N lines. RIB override: stop iterator after N entries (zero wasted generation). |
| `last <n>` | Pipe framework by default, command-specific override allowed | Rendered output lines | Generic: ring buffer of last N rendered lines. RIB override: ring buffer of N `RouteItem`s during iteration, only the last N get serialized. Avoids generating JSON for the full RIB. With `AllSorted`, `last N` deterministically returns the highest N prefixes numerically. |
| `count` | Pipe framework by default, command-specific override allowed | Structured output stream | For RIB route commands, count must be pushed into the command so routes need not be serialized. |
| `json` | Pipe framework | Output formatting | Must not force full route materialization when a streaming JSON representation is supported. |
| `ndjson` | Pipe framework | Output formatting | Preferred streaming format for large route sets. |
| `table` | Pipe framework | Output formatting | May buffer only what the table renderer fundamentally needs. |
| `text` | Pipe framework | Output formatting | Should support line-by-line streaming. |
| `yaml` | Pipe framework | Output formatting | May require buffering if YAML output needs a complete document. |
| `resolve` | Pipe framework | Structured output transform | Must remain generic. |
| `origin` | Pipe framework | Structured output transform | Must remain generic. |
| `log` | Pipe framework | Display mode | Remains display behavior. |
| `no-more` | Pipe framework | Display mode | Remains display behavior. |

### Command-Specific RIB Filters

| Filter | Applies To | Argument | Generation Effect |
|--------|------------|----------|-------------------|
| `received` | `show bgp rib` | none | Select Adj-RIB-In source only. |
| `advertised` | `show bgp rib` | none | Select Adj-RIB-Out source only. |
| `peer` | `show bgp rib`, `show bgp rib best` | selector | Iterate only matching peers where possible. |
| `family` | `show bgp rib`, `show bgp rib best` | AFI/SAFI | Drop non-matching families during iteration. |
| `prefix` | `show bgp rib`, `show bgp rib best` | prefix or pattern | Drop non-matching prefixes during iteration. |
| `path` | `show bgp rib`, `show bgp rib best` | AS path pattern | Drop non-matching AS paths during iteration. |
| `community` | `show bgp rib`, `show bgp rib best` | community | Drop non-matching communities during iteration. |
| `first` | `show bgp rib`, `show bgp rib best` | N (integer) | Stop iterator after N matching entries. No wasted generation. |
| `last` | `show bgp rib`, `show bgp rib best` | N (integer) | Ring buffer of N `RouteItem`s during iteration; only last N serialized. With sorted iteration, returns highest N prefixes. |
| `count` | `show bgp rib`, `show bgp rib best` | none | Count streamed matches without serializing route objects. |
| `prefix-summary` | `show bgp rib`, `show bgp rib best` | none | Aggregate streamed matches by family and prefix length. |
| `graph` | `show bgp rib`, `show bgp rib best` | none | Consume streamed matches to produce topology graph. |
| `match` | `show bgp rib`, `show bgp rib best` | text | Cross-field structured match during iteration (peer, prefix, AS path, communities). |
| `reason` | `show bgp rib best` | none | Explain best-path decision for surviving best-path entries. |

`match`, `first`, `last`, and `count` are registered as command-specific filters for RIB commands because the RIB pipeline can do more than the generic pipe: `match` does cross-field structured matching (peer, prefix, AS path, communities) instead of text grep, `first` stops the iterator early, `last` uses a ring buffer of N route entries so only the survivors get serialized, and `count` avoids serializing route objects entirely. For commands that do not register these, the generic client-side behavior applies.

## Data Flow (MANDATORY)

### Entry Point

| Entry | Format | Notes |
|-------|--------|-------|
| Interactive CLI command | User command line | Parsed by command mode before daemon dispatch. |
| SSH exec command | User command line | Parsed by shell client before daemon dispatch. |
| Command tree completion | Partial user command | Uses command tree plus pipe metadata. |
| RIB plugin command | Command selection and filters | Must receive route selection before generating rows. |

### Transformation Path

1. User enters a base command and optional pipes.
2. CLI parser identifies the base command using the command tree.
3. Pipe parser splits generic pipes from command-specific filters using the base command's registered metadata.
4. Completion after a pipe uses global pipe metadata plus the base command's command-specific filter metadata.
5. Dispatcher invokes the command handler with command-specific selections available before output generation.
6. RIB handler builds a route iterator source using selections such as received, advertised, peer, and family.
7. RIB handler applies remaining command-specific filters while iterating.
8. Formatter receives a stream of matching rows or terminal aggregate events.
9. Generic pipes such as `match`, display formats, `resolve`, and `origin` run as stream-capable stages where possible.
10. Output is written to the caller without storing the complete unfiltered RIB output.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI input to command tree | Base command resolved against `command.Node` tree | [ ] |
| Command tree to pipe metadata | Command-specific filter metadata found from resolved command | [ ] |
| Pipe parser to dispatcher | Command-specific selections passed with command invocation | [ ] |
| Dispatcher to RIB plugin proxy | Selection metadata preserved across the proxy boundary | [ ] |
| RIB plugin to formatter | Route entries streamed through filters and terminals | [ ] |
| Formatter to generic pipes | Generic pipes applied without command-specific knowledge | [ ] |

### Integration Points

- `command.Node` or equivalent command metadata carrier: stores command-specific pipe filter metadata for completion and help.
- `pluginserver.RPCRegistration` or neighboring command registry: command-owned registration source for command-specific filters.
- `command.CompletePipe` or successor: completes global pipe operators plus filters for the resolved command.
- `command.ProcessPipes*` or successor: separates generic pipes from command-specific filters without hardcoded command names.
- `pluginserver.Dispatcher`: passes command-specific selections to handlers before output generation.
- BGP RIB proxy handler: forwards selection metadata without lossy command-string reconstruction.
- BGP RIB iterator pipeline: applies received, advertised, peer, family, prefix, path, community, count, and best reason while streaming.

### Architectural Verification

- [ ] No bypassed layers: command-specific filters flow through command registration, not through hardcoded generic pipe checks.
- [ ] No unintended coupling: `internal/component/command` does not contain BGP command names or BGP filter semantics.
- [ ] No duplicated functionality: completion, validation, help, and handler dispatch derive from one command-owned filter registration.
- [ ] Zero-copy and streaming preserved where applicable: RIB route iteration does not build full unfiltered output before filtering.

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `show bgp rib | received` | -> | RIB filter selection reaches RIB iterator source | `TestShowPipelineReceivedScope` |
| `show bgp rib | advertised` | -> | RIB filter selection reaches Adj-RIB-Out source | `TestShowPipelineAdvertisedScope` |
| Pipe autocomplete after `show bgp rib |` | -> | Global pipes plus RIB filters are suggested | `TestCommandModePipeCompletion_WithFilters` |
| Pipe autocomplete after `show version |` | -> | Only global pipes are suggested | `TestCommandModePipeCompletion_NoFilters` |
| `show peer detail <selector>` | -> | Peer detail handler receives selector after action keyword | `TestDispatchShowPeerDetail` |
| `show summary` | -> | Current summary handler reachable through show path | `TestDispatchShowSummary` |
| `show bgp rib | match <text>` | -> | Generic match filters rendered output without BGP-specific code | `TestFilterMatch` |
| `show bgp rib | received | count` | -> | Count terminal runs without serializing full route output | `TestHandleCommandRibShowCount` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Generic pipe package inspected after implementation | No hardcoded BGP command names or BGP filter names remain in generic pipe folding logic. |
| AC-2 | `show summary` invoked | Current BGP summary response is returned through the show command path. |
| AC-3 | `show peer list` invoked | Brief peer list is returned. |
| AC-4 | `show peer detail <selector>` invoked | Peer detail is returned for matching selector, with action before identifier. |
| AC-5 | `show peer capabilities <selector>` invoked | Capabilities are returned for matching selector, with action before identifier. |
| AC-6 | `show peer statistics <selector>` invoked | Statistics are returned for matching selector, with action before identifier. |
| AC-7 | `show bgp rib | received` invoked | Only received routes are generated and streamed. |
| AC-8 | `show bgp rib | advertised` invoked | Only advertised routes are generated and streamed. |
| AC-9 | `show bgp rib | received | peer <selector>` invoked | Route iteration is restricted to received routes for matching peers before serialization. |
| AC-10 | `show bgp rib | advertised | count` invoked | Count is computed without serializing advertised route objects. |
| AC-11 | Completion after `show bgp rib |` | Suggestions include global pipes plus RIB-specific filters. |
| AC-12 | Completion after a command without registered filters | Suggestions include global pipes only. |
| AC-13 | `show bgp rib | match <text>` invoked | `match` is folded server-side for RIB commands (cross-field structured match); stays client-side generic grep for unregistered commands. |
| AC-14 | Unknown command-specific filter used after `show bgp rib |` | Command reports a validation error derived from registered filters. |
| AC-15 | `show bgp rib status | received` invoked | `received` is rejected because status did not register RIB route filters. |
| AC-16 | Help for `show bgp rib` requested | Help lists command-specific filters from registration metadata. |
| AC-17 | Existing internal RIB plugin commands used by components | Internal functionality remains available or is migrated with explicit call-site updates. |
| AC-18 | Documentation generated or updated | Command reference shows target command surface and pipe-filter model. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommandModePipeCompletion_WithFilters` | `internal/component/command/completer_test.go` | Resolved command contributes registered pipe completions. | pass |
| `TestCommandModePipeCompletion_NoFilters` | `internal/component/command/completer_test.go` | Commands without filter metadata only show global pipes. | pass |
| `TestCommandModePipeCompletion` | `internal/component/command/completer_test.go` | Generic pipe operators remain available for all output commands. | pass |
| `TestFoldFilters` | `internal/component/command/pipe_test.go` | Parser separates generic pipes from registered filters. | pass |
| `TestFoldFiltersValidationErrors` | `internal/component/command/pipe_test.go` | Unknown filter rejected from metadata. | pass |
| `TestProcessPipesChecked_InvalidFilter` | `internal/component/command/pipe_test.go` | Invalid filter rejected before dispatch. | pass |
| `TestDispatchShowPeerDetail` | `internal/component/bgp/plugins/cmd/peer/dispatch_test.go` | `show peer detail <selector>` reaches peer detail handler. | pass |
| `TestDispatchShowSummary` | `internal/component/bgp/plugins/cmd/peer/dispatch_test.go` | `show summary` reaches summary handler. | pass |
| `TestShowPipelineReceivedScope` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Received routes selected at source. | pass |
| `TestShowPipelineAdvertisedScope` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Advertised routes selected at source. | pass |
| `TestHandleCommandRibShowCount` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Count drains iterator without serializing routes. | pass |
| `TestCommandHelp_PipeFilters` | `internal/component/cmd/meta/meta_test.go` | Help derives filters from registration. | pass |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Not applicable | No new numeric input range in this spec | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-summary.ci` | `test/plugin/` | Operator runs `show summary` and sees current summary data. | missing |
| `show-peer-detail.ci` | `test/plugin/` | Operator runs `show peer detail <selector>` and sees peer detail. | missing |
| `show-rib-received.ci` | `test/plugin/` | Operator runs `show bgp rib | received` and receives only Adj-RIB-In routes. | missing |
| `show-rib-advertised.ci` | `test/plugin/` | Operator runs `show bgp rib | advertised` and receives only Adj-RIB-Out routes. | missing |
| `show-rib-match.ci` | `test/plugin/` | Operator runs `show bgp rib | match <text>` and gets grep-like line filtering. | missing |
| `show-rib-count.ci` | `test/plugin/` | Operator runs `show bgp rib | received | count` and gets count without full output. | missing |
| `show-rib-reject.ci` | `test/plugin/` | Operator runs `show bgp rib status | received` and gets validation error. | missing |
| `show-rib-complete.et` | `test/editor/` | Completion after `show bgp rib |` includes registered RIB filters. | missing |

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not required | N/A | N/A | CLI grammar and route output filtering only. No BGP wire behavior changes. | |

### Future (if deferring any tests)

- No deferrals planned. Functional route filtering and autocomplete tests are required for completion.

## Files to Modify

- `internal/component/command/node.go` - carry command-specific pipe filter metadata or equivalent command metadata.
- `internal/component/command/completer.go` - complete global pipes plus resolved command-specific filters.
- `internal/component/command/pipe.go` - remove hardcoded RIB folding and parse command-specific filters through metadata.
- `internal/component/command/pipe_test.go` - replace hardcoded RIB folding tests with metadata-driven tests.
- `internal/component/config/yang/command.go` - if command-specific filters are declared in command schemas, extract metadata into command tree nodes.
- `internal/component/config/yang/command_test.go` - verify command-specific filter metadata is derived, not duplicated.
- `internal/component/plugin/server/rpc_register.go` - if filters are registered alongside RPCs, add registration metadata there or in a sibling registry.
- `internal/component/plugin/server/command.go` - pass command-specific selections to handlers before output generation.
- `internal/component/plugin/server/command_test.go` - verify dispatch and validation behavior.
- `internal/component/bgp/plugins/cmd/rib/schema/ze-rib-cmd.yang` - de-emphasize or remove noun-first user-facing RIB paths if implementation chooses to remove aliases.
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - ensure `show bgp rib` and child status commands are canonical.
- `internal/component/bgp/plugins/cmd/rib/rib.go` - adapt proxy forwarding to preserve command-specific selections.
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - expose received and advertised selection through streaming route iteration.
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - expose best-path filters and reason through registered command-specific filters.
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - migrate read-only peer and summary commands to show paths or move them to a show command schema.
- `internal/component/bgp/plugins/cmd/peer/peer.go` - register `show peer ...` handlers and preserve selector behavior.
- `internal/component/bgp/plugins/cmd/peer/summary.go` - register `show summary` handler path.
- `docs/guide/command-reference.md` - document target command surface and pipe-filter model.
- `docs/guide/command-catalogue.md` - update vendor comparison if it remains the command roadmap reference.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang`, `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang`, RIB command schemas as needed. |
| YANG validation constraints | Yes | New command filter declarations, if represented in YANG, need typed argument metadata. |
| YANG custom validators | Maybe | Dynamic peer/family completions may need validator or value hint integration. |
| CLI commands/flags | Yes | Online command tree and CLI pipe parsing. |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md` applies to `show peer ...`. |
| Editor autocomplete | Yes | Pipe autocomplete must include command-specific filters for resolved command only. |
| Functional test for new RPC/API | Yes | `test/plugin/` or `test/ui/`, plus editor tests for completion. |
| Pipe completeness | Yes | Every output command must retain generic pipe support. |
| Env var registration | No | No environment config leaves. |
| Doctor check for runtime dependencies | No | No new runtime dependency. |
| Prometheus counters/metrics | No | No new observable subsystem state. |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` if command overview lists operational CLI capabilities. |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if command metadata model changes. |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/command-catalogue.md` if kept as roadmap. |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | Maybe | `docs/architecture/api/process-protocol.md` only if command-specific selections cross plugin protocol as a new payload field. |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it tracks command parity. |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |

## Files to Create

- `test/plugin/show-summary.ci` - end-to-end summary through show path.
- `test/plugin/show-peer-detail.ci` - end-to-end peer detail with selector.
- `test/plugin/show-rib-received.ci` - received-only route stream.
- `test/plugin/show-rib-advertised.ci` - advertised-only route stream.
- `test/plugin/show-rib-match.ci` - generic match on RIB output.
- `test/plugin/show-rib-count.ci` - streaming count without serialization.
- `test/plugin/show-rib-reject.ci` - validation rejection for wrong command.
- `test/editor/show-rib-complete.et` - pipe completion with registered RIB filters.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | Targeted tests, `make ze-lint-changed`, `make ze-verify` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run verification |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Re-run verification |
| 14. Present summary | Executive Summary Report per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring** - introduce filter metadata and failing tests.
   - Tests: `TestCommandModePipeCompletion_WithFilters`, `TestShowPipelineReceivedScope`.
   - Files: `pipe_filter.go`, completer, dispatcher tests.
   - Verify: tests fail because metadata is not yet connected.
2. **Phase: Remove hardcoded pipe folding** - delete BGP knowledge from generic pipe code.
   - Tests: `TestFoldFilters`, `TestFoldFiltersValidationErrors`.
   - Files: `internal/component/command/pipe.go`, `pipe_test.go`.
   - Verify: generic pipe package has no BGP command names.
3. **Phase: Command grammar migration** - expose `show summary`, `show peer ...`, canonical RIB show paths.
   - Tests: `TestDispatchShowSummary`, `TestDispatchShowPeerDetail`, existing peer tests updated.
   - Files: peer command schema, show command schema, peer handlers, command docs.
   - Verify: action-before-identifier grammar is used for peer selectors.
4. **Phase: Streaming RIB filters** - apply received/advertised selection before serialization.
   - Tests: `TestShowPipelineReceivedScope`, `TestShowPipelineAdvertisedScope`, `TestHandleCommandRibShowCount`.
   - Files: RIB proxy, RIB pipeline, best pipeline if needed.
   - Verify: received and advertised streams are selected at source.
5. **Phase: Completion and help** - derive pipe autocomplete and help from metadata.
   - Tests: `TestCommandModePipeCompletion_WithFilters`, `TestCommandHelp_PipeFilters`.
   - Files: completer, help renderer, docs.
   - Verify: RIB filters appear only for RIB route commands.
6. **Phase: Functional tests** - add end-user `.ci` and `.et` tests.
   - Tests: `show-summary.ci`, `show-peer-detail.ci`, `show-rib-received.ci`, `show-rib-advertised.ci`, `show-rib-match.ci`, `show-rib-count.ci`, `show-rib-reject.ci`, `show-rib-complete.et`.
   - Verify: functional tests exercise the user-facing command spelling.
7. **Phase: Documentation and verification** - update command reference and run final gates.
   - Tests: package tests, functional tests, `make ze-lint-changed`, `make ze-verify`.
   - Verify: no undocumented command behavior changes remain.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every target command in the Target Command Surface table is wired and tested. |
| Correctness | Received routes map to Adj-RIB-In and advertised routes map to Adj-RIB-Out. |
| Streaming | RIB filters are applied before serialization and count does not serialize route rows. |
| Naming | User-facing filter is `advertised`, not `sent`; JSON keys remain kebab-case. |
| Data flow | Command-specific filters come from command metadata and reach handlers before output generation. |
| CLI grammar | Peer selectors appear after `list`, `detail`, `capabilities`, `statistics`, or `history`. |
| Pipe completeness | Generic pipes remain available to all output-producing commands. |
| No hardcoding | Generic pipe code contains no BGP command names or RIB filter lists. |
| Documentation | Command reference documents target spelling and pipe-filter model. |
| Backward compatibility | Any retained legacy aliases are explicitly registered and documented as aliases or deprecated paths. |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `show summary` command path | Dispatch or functional test proves handler is reachable. |
| `show peer ...` command paths | Dispatch tests prove each path reaches the expected handler. |
| `show bgp rib | received` | Functional or unit test proves only received routes are streamed. |
| `show bgp rib | advertised` | Functional or unit test proves only advertised routes are streamed. |
| Command-specific pipe completion | Editor or completer test proves RIB filters appear only on RIB route commands. |
| Generic `match` preserved | Test proves `match` works without command-specific registration. |
| No BGP hardcoding in pipe package | Grep generic pipe package for BGP RIB command names and route filter names. |
| Documentation updated | Diff includes command reference and command catalogue updates where applicable. |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Filter arguments such as peer, family, prefix, path, and community reject invalid or oversized inputs before heavy iteration. |
| Resource exhaustion | Route streaming does not accumulate unbounded route slices for filtering or count. |
| Error leakage | Validation errors report accepted filter names from metadata without dumping internal structures. |
| Authorization | Command rewrite does not bypass existing command authorization or accounting paths. |
| Cross-boundary integrity | Command-specific selections crossing proxy or plugin boundaries are typed or validated, not string-spliced blindly. |

## Review Gate

Not yet run. The spec remains open because functional tests and lint verification are not clean.

## Implementation Audit

| AC | Status | Implementation Evidence | Test / Verification Evidence |
|----|--------|-------------------------|------------------------------|
| AC-1 | Covered in implementation | `internal/component/command/pipe.go` folds command filters through `lookupPipeFilters`, with no BGP command-name switch. | `grep internal/component/command` finds `received` only in tests/fixtures, not pipe implementation. |
| AC-2 | Covered in unit/dispatch tests | `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` maps `show summary` to `ze-bgp:summary`. | `TestDispatchShowSummary`; `go test -race ./internal/component/bgp/plugins/cmd/peer`. |
| AC-3 | Covered in unit/dispatch tests | `show peer list` alias maps to `ze-bgp:peer-list`. | `TestDispatchShowPeerList`; `go test -race ./internal/component/bgp/plugins/cmd/peer`. |
| AC-4 | Covered in unit/dispatch tests | `cmdutil.ExtractSelector` accepts declared positional args and peer detail uses `filterPeersByArgs`. | `TestDispatchShowPeerDetail`; `go test -race ./cmd/ze/internal/cmdutil ./internal/component/bgp/plugins/cmd/peer`. |
| AC-5 | Covered in unit/dispatch tests | `show peer capabilities <selector>` maps to `ze-bgp:peer-capabilities`. | `TestDispatchShowPeerCapabilities`; `go test -race ./internal/component/bgp/plugins/cmd/peer`. |
| AC-6 | Covered in unit/dispatch tests | `show peer statistics <selector>` maps to `ze-bgp:peer-statistics`. | `TestDispatchShowPeerStatistics`; `go test -race ./internal/component/bgp/plugins/cmd/peer`. |
| AC-7 | Covered in RIB pipeline tests | `rib_pipeline.go` maps `received` to Adj-RIB-In source selection. | `TestShowPipelineReceivedScope`; `go test -race ./internal/component/bgp/plugins/rib`. |
| AC-8 | Covered in RIB pipeline tests | `rib_pipeline.go` maps `advertised` to Adj-RIB-Out source selection. | `TestShowPipelineAdvertisedScope`; `go test -race ./internal/component/bgp/plugins/rib`. |
| AC-9 | Covered in RIB pipeline tests | `peer <selector>` is parsed as a command-specific RIB filter and applied during source selection. | `TestShowPipelinePeerFilter`; `go test -race ./internal/component/bgp/plugins/rib`. |
| AC-10 | Covered in RIB pipeline tests | `count` terminal drains the iterator and returns count metadata without route row JSON. | `TestTerminalCount`, `TestHandleCommandRibShowCount`; `go test -race ./internal/component/bgp/plugins/rib`. |
| AC-11 | Covered in completion tests | `CompletePipeForCommand` appends `filterSuggestions` from registered filters. | `TestCommandModePipeCompletion_WithFilters`; `go test -race ./internal/component/command`. |
| AC-12 | Covered in completion tests | Commands without registered filters only use `PipeOperators`. | `TestCommandModePipeCompletion_NoFilters`; `go test -race ./internal/component/command`. |
| AC-13 | Covered in pipe folding tests | `match` folded server-side for RIB (cross-field structured match); stays client-side for unregistered commands. | `TestFoldFilters` "match folded server-side" + "match stays client-side" cases; `go test -race ./internal/component/command`. |
| AC-14 | Covered in pipe validation tests | Unknown registered-command filters produce `pipeInvalid` with valid names from `PipeFilter` metadata. | `TestFoldFiltersValidationErrors`, `TestProcessPipesChecked_InvalidFilter`; `go test -race ./internal/component/command`. |
| AC-15 | Covered in pipe folding tests | Empty registrations on status commands shadow the route command filter set. | `TestFoldFilters` status case; `go test -race ./internal/component/command`. |
| AC-16 | Covered in meta command tests | `command help` includes `pipe-filters` derived from `command.PipeFiltersForCommand`. | `TestCommandHelp_PipeFilters`; `go test -race ./internal/component/cmd/meta`. |
| AC-17 | Covered by preserved registrations/tests | Existing internal RIB command strings remain registered as aliases and `sent` remains accepted internally. | Existing RIB pipeline tests for `sent`; `go test -race ./internal/component/bgp/plugins/rib ./internal/component/bgp/plugins/cmd/rib`. |
| AC-18 | Covered in docs diff | Command reference, command catalogue, and API commands architecture docs now show canonical `show` paths and command-specific RIB filters. | Manual doc update in `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/architecture/api/commands.md`. |

Functional `.ci` / `.et` coverage is still pending for the new user-facing paths and pipe completion. The work is not complete until those tests exist or the user explicitly reduces scope.

## Pre-Commit Verification

| Command | Result |
|---------|--------|
| `go test -race ./internal/component/command ./internal/component/cmd/meta ./internal/component/bgp/plugins/rib ./internal/component/bgp/plugins/cmd/rib ./internal/component/bgp/plugins/cmd/peer ./cmd/ze/internal/cmdutil ./cmd/ze/cli` | Pass |
| `go test -race ./internal/component/config/yang` | Pass |
| `go test -race ./internal/component/plugin/server` | Fails: pre-existing/unrelated `RPC ze-show:host-platform has no YANG path mapping` in `TestEveryRPCHasYANGPath`. |
| `go test -race ./internal/component/cli` | Fails: `TestCmdOptionChangesRedirectsToPipe` expected an error and got nil. This is outside the pipe-filter code touched here, but the package is affected by the strict pipe validation call-site change. |
| `make ze-lint-changed` | Fails on unrelated changed-package issues: `internal/component/cli/model_load.go` gocritic, `internal/component/cli/model_commands.go` unparam, and `internal/component/config/system/backend_test.go` modernize/noctx. |

Pre-commit verification is not clean. Do not close this spec yet.

## Documentation Updates

| File | Update |
|------|--------|
| `docs/guide/command-reference.md` | Replaced BGP summary and peer read-only examples with `show summary` and `show peer ...`; documented canonical RIB show and pipe-filter examples. |
| `docs/guide/command-catalogue.md` | Updated BGP command catalogue rows from noun-first forms to `show` forms, including `advertised` Adj-RIB-Out pipe spelling. |
| `docs/architecture/api/commands.md` | Updated peer/RIB operational command sections and documented command-specific RIB filters versus generic pipes. |

## Deviations

Known issue before implementation: the current worktree contains an incorrect hardcoded RIB pipe folding change in `internal/component/command/pipe.go`. This spec requires removing that design and replacing it with command-owned metadata.

Current implementation keeps legacy internal/user aliases registered while adding canonical `show` paths. This follows `ai/rules/cli-grammar.md` backward-compatibility guidance for grammar fixes, but the spec's open legacy-alias question still needs an explicit final policy before closure.

`sent` remains accepted by the internal RIB pipeline and tests. It is not registered as a user-facing command-specific pipe filter, so `show bgp rib | sent` is rejected by the generic pipe fold while `bgp rib show sent` remains available internally.

No user-approved deferrals at spec creation time. Functional coverage and clean verification remain pending, so this spec is not ready to close.

## Open Questions

| Question | Default In This Spec | Needs User Decision? |
|----------|----------------------|----------------------|
| Should legacy `peer list`, `peer detail`, and `summary` remain as aliases? | No backward compatibility unless explicitly requested. | Yes |
| Should `sent` remain as an alias for `advertised`? | No, user-facing term is `advertised`. | Yes |
| Should family filtering for `show summary` remain positional or move to pipe metadata? | Preserve current summary family argument for now. | Yes |
| Should `show bgp rib | json` stream a JSON array or prefer `ndjson` for large outputs? | Preserve explicit JSON behavior; prefer `ndjson` for streaming where user chooses it. | Yes |
| Where should command-specific pipe metadata be registered? | Command-owned registry keyed by wire method or command node, exact storage to decide during design review. | Yes |

## Implementation Summary

Implemented so far:

| Area | Summary |
|------|---------|
| Command-owned pipe metadata | Added `PipeFilter` registration in `internal/component/command/pipe_filter.go`, command-scoped lookup, completion suggestions, and help accessors. |
| Generic pipe folding | Replaced BGP hardcoding with registry-driven folding. Unknown filters, missing args, and extra args on flag filters are now validation errors. |
| CLI validation | Added checked pipe-processing entry points and used them in `ze cli -c` and operational command mode so invalid pipes can stop before dispatch. |
| RIB filter registration | RIB command package registers `received`, `advertised`, `peer`, `family`, `prefix`, `path`, `community`, `count`, `prefix-summary`, `graph`, and best-path `reason`. |
| RIB pipeline | `advertised` selects Adj-RIB-Out, conflicting direction filters reject, and `peer <selector>` constrains route selection. |
| Show grammar | YANG aliases and dispatch support expose `show summary`, `show peer list`, `show peer detail <selector>`, `show peer capabilities <selector>`, `show peer statistics <selector>`, and `show peer history <selector>`. |
| Command help | `command help` can include `pipe-filters` derived from the same registration metadata used by completion and folding. |
| Documentation | Updated command reference, command catalogue, and API command architecture docs for canonical show paths and RIB pipe filters. |

Remaining before closure:

| Item | Reason |
|------|--------|
| Functional `.ci` tests for `show summary`, `show peer ...`, and `show bgp rib | received/advertised` | Required by `ai/rules/functional-test-gate.md` for changed user-facing behavior. |
| Editor `.et` or equivalent functional completion test for `show bgp rib |` | Required by the spec's functional test table. |
| Clean `make ze-lint-changed` | Currently blocked by unrelated changed-package lint issues. |
| Clean targeted plugin-server wiring test | Currently blocked by unrelated `ze-show:host-platform` YANG mapping failure. |
| Final legacy alias policy | The implementation keeps aliases; spec closure needs a final documented decision. |
