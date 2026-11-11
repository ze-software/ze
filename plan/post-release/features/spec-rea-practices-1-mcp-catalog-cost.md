# Spec: rea-practices-1-mcp-catalog-cost

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P3 |
| Release reason | Improves how well agents can use the MCP surface and proves real clients load it; it fixes no incorrect routing or config behaviour, so it does not hold the first release |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-11 |

Parent: `plan/post-release/engineering/spec-rea-practices-0-umbrella.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

WIP cap exception: Thomas granted a one-off claim above the cap for this spec
on 2026-11-09 (`ZE_SPEC_WIP_CAP=25`). It applies to this spec only.

## Task

Every operator command reaches MCP as a tool derived from the command
registry (`ai/META-DECISIONS.md`: every operator command runs through the
daemon on every command surface, MCP included). The catalog therefore grows
with every plugin, and nothing measures what it costs an agent's context or
whether a real agent client can load it at all.

REA (`/Users/thomas/Code/github.com/morluto/rea`) measures exactly this:
catalog cost through a real client's reported usage
(REA `docs/mcp-catalog-evaluation.md`), structural limits real clients
enforce (REA `docs/reference/mcp-contracts.md`, "Compact input schema
profile": a 13,312-byte per-tool input schema budget measured below
Moonshot's cap, a ten-level depth limit, no bare `enum` without `type`, no
`oneOf`/`allOf`), and per-client verification lanes
(REA `docs/development/client-verification.md`).

Thomas approved on 2026-11-08: measure first, then decide on any reduction.

Goals:

| # | Goal |
|---|------|
| G-1 | The size and shape of the catalog a running daemon serves are measured and printed, from the real `tools/list` answer |
| G-2 | A functional test refuses a catalog that breaks the structural limits real clients are known to enforce |
| G-3 | A documented lane, run by Thomas, records which real agent clients (Claude Code, Codex) connect to Ze's MCP server, call a tool, and what context they report the catalog costs |
| G-4 | A tool whose definition fails to build is reported, never silently dropped from the list |

Out of scope, decided at the design gate: a compact schema profile. Ze's
input schemas are flat, so the defects REA's profile works around are not
produced today (see Current Behavior). Any reduction of the catalog is
designed only after G-1 and G-3 produce numbers, in a spec of its own. If the
G-3 lane shows that a current client cannot connect, that connectivity defect
also gets its own spec (R-1).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - design all subsystems share
  → Constraint: a command is declared once and reaches every interface; MCP derives its tools at call time, so nothing in this spec may add a per-command MCP entry or a hand-kept tool list
- [ ] `ai/doors/api-surfaces.md` - MCP is a thin adapter over the dispatcher
  → Constraint: never add a command in an adapter and never copy a fact the YANG model or registry holds; the measurement reads the served answer, it does not re-derive tools
- [ ] `ai/doors/le-tooling.md` - the probe lives in the test harness
  → Constraint: product code never imports `internal/le`; the probe lives in `internal/test/cli`, reached by `le test mcp`, and a unit run goes through `./le job run label unit-pkg command go test -tags ...`
- [ ] `ai/doors/hub-engine.md` - the daemon assembles the catalog's metadata
  → Constraint: `cmd/ze/hub` is always on and reaches MCP only through the gated `service_mcp.go`; this spec changes nothing there, and the measurement must run against a daemon started the normal way (`ze --mcp $PORT`)
- [ ] `docs/architecture/mcp/overview.md` - the MCP profile, tool derivation
  → Constraint: only revision `2026-07-28` (stateless Streamable HTTP) is accepted; `initialize` is answered with an error that names the supported version, so a client speaking an earlier revision cannot connect
  → Decision: tool names are `ze_<prefix_joined_with_underscores>`, plus handcrafted `ze_execute` and `ze_reference`; the server's `instructions` point an agent at `ze_reference` first, so the real-client lane uses `ze_reference` as its call
- [ ] `docs/guide/mcp/overview.md` - the operator guide, `// Design:` page of `tools.go`
  → Constraint: the guide tells a reader to run `tools/list` against a live daemon to see the inventory, and states the 60 s `ttlMs`; the G-4 error answer must be described there in the reader's terms
- [ ] `ai/digests/mcp.md` - traced request flow
  → Constraint: `tools/list` is assembled from `commandMetaSource` (`cmd/ze/hub/command_meta.go`) in the running daemon; an in-process copy would skip YANG parameter, task-support and UI enrichment, so the measurement must use a running daemon
- [ ] `docs/functional-tests.md` - "le test mcp" verbs
  → Decision: the new verb is documented beside `tools-order-stable`, in the same verb list
- [ ] `docs/architecture/testing/ci-format.md` - the `le test mcp` harness command (`// Design:` of `cmd_mcp.go`)
  → Constraint: a probe line is one output line a `.ci` asserts with `expect=stdout:pattern=`; colon-bearing values need `pattern=` (`test/plugin/mcp-tools-list-argument-texts.ci`)

### RFC Summaries (Scope: protocol)
- N-A: MCP is not an RFC; no RFC behaviour changes.

**Key insights:**
- Ze's per-tool input schema is depth 2 (object, then flat properties), with `type` always beside `enum`; REA's structural defects are not produced today, so the structural test is a regression guard, not a fix
- The unmeasured cost is tool count times description length; the `action` enum description joins every action's short help
- The real-client question may be connectivity (revision `2026-07-28` only), not size

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/tools.go` - `groupCommands` groups commands by longest shared prefix (two or more commands); `generateTools` builds one tool per group with `buildToolDef`; `buildToolDef` emits an object schema with `action` (`type` string plus `enum`, description joining each action's short help), typed YANG parameters (`addYANGParams`), a generic `arguments` string when no typed parameter exists, and `peer` only on selector-taking groups. When `json.Marshal` of the schema fails it returns nil, and `generateTools` skips a nil tool with no log line and no error
- [ ] `internal/component/mcp/streamable_tools.go` - `allTools` returns handcrafted tools then generated ones, or a provider's tools in Provider mode; a `CommandLister` error is returned, never answered as an empty list
- [ ] `internal/component/mcp/caching.go` - `tools/list` carries `ttlMs` and `cacheScope` hints, so a client need not re-list each turn
- [ ] `internal/test/cli/cmd_mcp.go` - `le test mcp` script verbs, among them `wait-tool`, `tools-order-stable` (byte comparison of repeated `tools/list` answers) and `probe`
- [ ] `internal/le/test/mcp/register.go` - registers `le test mcp`, forwarding to `cli.CmdMcp`
- [ ] `test/plugin/mcp-tools-list-argument-texts.ci` - pattern for starting `ze --mcp $PORT` and driving `le test mcp`
- [ ] `test/plugin/mcp-tools-list-deterministic-order.ci` - existing catalog-shape test using `tools-order-stable`

**Behavior to preserve:**
- Tool names, grouping, schemas and descriptions are unchanged by this spec: measurement only
- `tools-order-stable` output and every existing `le test mcp` verb
- Provider mode (`le chaos run`) tool lists are untouched

**Behavior to change:**
- A tool whose schema fails to encode is reported (G-4) instead of vanishing

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `le test mcp` script client | `internal/test/cli/cmd_mcp.go` `CmdMcp` | Speaks revision `2026-07-28` to a running daemon and runs script verbs | Extends: adds the `tools-size` verb beside `tools-order-stable` |
| `tools-order-stable` verb | `internal/test/cli/cmd_mcp.go`, `docs/functional-tests.md` | Byte-compares repeated `tools/list` answers | Extends its pattern: one verb, one result line a `.ci` asserts |
| Tool derivation from the registry | `internal/component/mcp/tools.go` `groupCommands`, `generateTools`, `buildToolDef` | Builds the catalog at call time | Extends: `buildToolDef` reports an encode failure instead of returning nil in silence; derivation otherwise unchanged |
| Functional test runner with daemon | `test/plugin/*.ci`, `cmd=background ... ze --mcp $PORT` | Starts a daemon and drives the probe | Extends: one new `.ci` asserts the structural limits |
| Caching hints | `internal/component/mcp/caching.go` | Lets a client keep the list across turns | Does not apply: it reduces how often the list is fetched, not what it costs per load |
| Provider mode | `StreamableConfig.Provider`, `allTools`; `internal/chaos/mcp/tools.go` | Replaces the tool surface for `le chaos run` | Does not apply: not the operator catalog |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `tools/list` JSON-RPC request over HTTP POST to a running daemon's MCP listener, sent by `le test mcp` (G-1, G-2) or by a real agent client (G-3)

### Transformation Path
1. The daemon's `commandMetaSource` walks the dispatcher and YANG to build `CommandInfo` values
2. `groupCommands` and `generateTools` build the tool definitions; `allTools` prepends the handcrafted tools
3. The answer is serialized and returned
4. `tools-size` parses the answer and computes, per tool: serialized tool bytes, serialized `inputSchema` bytes, schema nesting depth, count of `enum` members without a sibling `type`, presence of `oneOf`/`allOf`/`anyOf`; and in total: tool count and total bytes
5. It prints one summary line and one line per tool that breaks a limit

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test client ↔ daemon | MCP `2026-07-28` HTTP POST | No |
| Real agent client ↔ daemon (G-3) | The client's own MCP HTTP transport | No |

### Changed-Wire Census

**Search evidence:** scope `internal/` and `cmd/`, non-test Go files, term `tools/list`. Producers: `internal/component/mcp/streamable_tools.go` `allTools` (operator catalog) and `internal/chaos/mcp/tools.go` (Provider mode). Readers in the tree: `internal/test/cli/cmd_mcp_client.go` and `cmd_mcp_calls.go` (`le test mcp`). Readers outside the tree: every MCP client. The answer format does not change; the one change is that a tool which cannot be encoded now turns the answer into a JSON-RPC error instead of being omitted.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `tools/list` result | writer | `internal/component/mcp/streamable_tools.go` `allTools` | Only producer of the operator catalog | Preserved; G-4 adds an error answer for a tool that cannot be encoded |
| `tools/list` result | writer | `internal/chaos/mcp/tools.go` (Provider mode) | Separate provider catalog | Preserved: not touched |
| `tools/list` result | reader | `internal/test/cli/cmd_mcp_client.go` (`le test mcp`) | Parses the list for `wait-tool`, `tools-order-stable` | Extended: `tools-size` reads the same answer; existing verbs unchanged |
| `tools/list` result | reader | External MCP clients (Claude Code, Codex, others) | Outside the repository | Preserved: same format; an encode failure, which no known tool hits today, now arrives as a named JSON-RPC error |

### Integration Points
- `CmdMcp` verb dispatch - new `tools-size` verb
- `generateTools` / `buildToolDef` - error return in place of the silent nil

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Measurement reads the served answer of a running daemon |
| No unintended coupling (components stay isolated) | Yes | The verb lives in the test client; no product import of `internal/le` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `le test mcp`, not a new client |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test client only; the product change is an error path |
| Registration over hardcoding, outbound | Yes | No tool, command or list is added |
| Registration over hardcoding, inbound | Yes | The limits judge every served tool; no list of tool names is kept |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's served input schemas are at most depth 2 and always carry `type` beside `enum` | `buildToolDef` and `addYANGParams` in `internal/component/mcp/tools.go` | The structural `.ci` goes red at first run, which is then a found defect to fix under this spec | First run of `mcp-tools-list-structural-limits.ci` | confirmed (2026-11-09: max-depth=2, bare-enum=0, combinators=0 over 142 served tools) |
| A-2 | Claude Code and Codex, at their versions on the day of the lane, speak MCP `2026-07-28` | Not known; Ze accepts only that revision (`docs/architecture/mcp/overview.md`) | The clients cannot connect; the lane records the refusal and a connectivity spec is opened (R-1) | The G-3 lane's first run | broken for Codex: 0.167.1 opens with the `2025-06-18` `initialize` handshake, which Ze refuses with `-32020` (lane run 2026-11-11); confirmed for Claude Code 2.1.322 |
| A-3 | Each client reports token usage for a headless run (Claude Code JSON output, Codex JSON events) | REA records Codex usage this way (REA `docs/mcp-catalog-evaluation.md`) | The token column records "not reported" with the client version; size is still measured by G-1 | The G-3 lane's first run | confirmed (2026-11-11: both clients reported input token usage, lane Results table) |
| A-4 | The 13,312-byte and ten-level limits are the right budgets | REA's measurements for Moonshot and strict-depth clients | A tighter client limit exists; the budget is lowered when a client refusal is observed | Lane results and REA's contract page | confirmed as far as observed (2026-11-11: Claude Code loaded the 142-tool catalog with no structural refusal; Codex never reached `tools/list`) |
| A-5 | A loopback daemon with a bearer token configured is enough for a real client to authenticate | `docs/architecture/mcp/overview.md`, bearer auth mode | The lane needs OAuth setup; documented in the lane page | The G-3 lane's first run | confirmed (2026-11-11: Claude Code authenticated with an `Authorization: Bearer` header alone) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Real clients cannot connect because Ze accepts only `2026-07-28` | Lane records a protocol-version refusal | Record it, open a spec in `plan/release/fixes/` (every operator command is owed on MCP, so an MCP no current client can reach is a shipped defect), and keep this spec's measurement goals |
| R-2 | The lane spends model credits or is started by an agent | A lane run with no owner order | The lane page says agents never start it and CI never runs it; it is a manual procedure Thomas runs |
| R-3 | The structural `.ci` passes vacuously (no tools served) | Summary line shows a tool count of 0 or only the handcrafted two | The `.ci` asserts a tool count above the handcrafted two, and the red phase is recorded by breaking a limit on purpose |
| R-4 | Reporting the encode failure turns a whole `tools/list` into an error, hiding every other tool | A client sees an error instead of a list | Design choice recorded in Key Design Decisions: the list fails with an error naming the tool, because a silently partial list is the defect `ai/rules/principles.md` forbids |
| R-5 | `docs/functional-tests.md` carries another session's uncommitted hunks | `git diff` of the file at commit time | Commit own hunks with `hunks <path> <patch-file>` if the foreign hunks are unsafe (`ai/rules/git-safety.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false red in the new `.ci`; or, for G-4, a `tools/list` that errors where it used to omit a tool |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any spec editing `internal/component/mcp/tools.go` or `internal/test/cli/cmd_mcp.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le test mcp` script line `tools-size` against `ze --mcp $PORT` | → | `CmdMcp` verb `tools-size` | `test/plugin/mcp-tools-list-structural-limits.ci` |
| `tools/list` with a command group whose schema cannot encode | → | `generateTools` error path | `TestGenerateToolsReportsEncodeFailure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `tools-size` against a daemon with the full plugin set | Prints one line `tools-size count=<n> bytes=<total> largest=<tool> largest-bytes=<n> max-schema-bytes=<n> max-depth=<n> bare-enum=<n> combinators=<n>`, every value computed from the served answer | None |
| AC-2 | `tools-size` with a tool whose input schema exceeds 13,312 bytes, or depth above 10, or an `enum` without `type`, or any `oneOf`/`allOf`/`anyOf` | Prints one `tools-size violation tool=<name> limit=<limit> value=<value>` line per breach, after the summary | None |
| AC-3 | `mcp-tools-list-structural-limits.ci` | The summary line reports a tool count above the two handcrafted tools, and the test passes with the summary as the only `tools-size` line | No violation line, because the served catalog meets every limit (A-1) |
| AC-4 | Red phase for AC-3 | With one limit lowered below the served value (for example the per-tool budget set under the largest tool's size), the `.ci` goes red and names that tool; the red is recorded in the spec | None |
| AC-5 | A command group whose schema cannot be encoded | `tools/list` answers a JSON-RPC error naming the tool and the encode error | Other tools are not served in that answer, by design (R-4) |
| AC-6 | The real-client lane page | Names the steps, the clients (Claude Code, Codex), what is recorded (client and version, connected yes/no, tool called yes/no, reported token usage with and without Ze's server, date), where results are stored, and that only Thomas starts it | None |
| AC-7 | The lane's first run, by Thomas | The lane page's result table holds one row per client, with its version, whether it connected, whether `ze_reference` was called, and the token usage it reported or the reason it reported none | A client that cannot connect is a valid row (R-1) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A developer adds a plugin with a large command group | registry → `tools/list` → `tools-size` limits | `test/plugin/mcp-tools-list-structural-limits.ci` |
| 2 | Thomas checks whether Claude Code can use Ze over MCP | lane → daemon → client calls `ze_reference` → recorded result | AC-7 result row |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToolsSizeSummary` | `internal/test/cli/cmd_mcp_test.go` | AC-1: counts, bytes, largest, depth from a canned `tools/list` answer | |
| `TestToolsSizeViolations` | `internal/test/cli/cmd_mcp_test.go` | AC-2: each of the four limits produces one violation line; a schema at exactly the limit produces none | |
| `TestGenerateToolsReportsEncodeFailure` | `internal/component/mcp/tools_test.go` | AC-5: an unencodable schema yields an error naming the tool | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Input schema bytes | 0-13312 | 13312 | N/A | 13313 |
| Schema depth | 1-10 | 10 | N/A | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-tools-list-structural-limits` | `test/plugin/mcp-tools-list-structural-limits.ci` | A client loading Ze's catalog gets no tool a strict client would refuse | |

### Interop Tests (Scope: protocol)
- N-A in the protocol sense: MCP behaviour does not change. The real-client lane (AC-6, AC-7) is the interop evidence with other implementations.

## Files to Modify
- `internal/test/cli/cmd_mcp.go` - `tools-size` verb and its help text
- `internal/component/mcp/tools.go` - `generateTools` / `buildToolDef` error path (G-4)
- `internal/component/mcp/streamable_tools.go` - `allTools` returns the error
- `docs/functional-tests.md` - document `tools-size` beside `tools-order-stable`
- `docs/architecture/mcp/overview.md` - catalog limits, the `tools/list` error path, a pointer to the lane
- `docs/guide/mcp/overview.md` - the `// Design:` page of `tools.go`; its "tool inventory" section gains the error answer a client sees when a tool cannot be built, and a pointer to the client verification results
- `docs/architecture/testing/ci-format.md` - the new verb, if the page lists `le test mcp` verbs
- `ai/digests/mcp.md` - `tools/list` error path
- `features/mcp-integration.md` - evidence of real-client verification once AC-7 has a row

## Files to Create
- `test/plugin/mcp-tools-list-structural-limits.ci` - AC-3
- `docs/contributing/mcp-client-verification.md` - the real-client lane (AC-6) and its result table (AC-7)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | `le test mcp` gains a script verb, not a CLI flag |
| CLI grammar (keyword before value) | N-A | No CLI command |
| Editor autocomplete | N-A | No YANG change |
| Functional test for new RPC/API | Yes | `test/plugin/mcp-tools-list-structural-limits.ci` |
| Pipe completeness | N-A | No operator command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency added |
| Prometheus counters/metrics | N-A | No observable runtime state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/mcp-integration.md` (evidence row for real clients, after AC-7) |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/mcp/overview.md` (`tools/list` error path, catalog limits) |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | MCP has no user guide change |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `ai/digests/mcp.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared and edited: `docs/guide/mcp/overview.md`, `docs/functional-tests.md`. Mentioned and re-read at implementation, expected unaffected because tool names, schemas and the dispatch path do not change: `docs/architecture/api/commands.md`, `docs/comparison.md`, `docs/features/ai-first.md`, `docs/features/mcp-integration.md`, `docs/guide/mcp/elicitation.md`, `docs/guide/mcp/tasks.md` (from `./le spec citation anchors spec plan/post-release/features/spec-rea-practices-1-mcp-catalog-cost.md`, 2026-11-08) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md` verb list stays accurate |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `tools-size` verb registered in the probe, printing a stub line; failing `.ci` and unit tests
   - Tests: `mcp-tools-list-structural-limits`, `TestToolsSizeSummary`
   - Files: `internal/test/cli/cmd_mcp.go`, the new `.ci`
   - Verify: the `.ci` reaches the verb and fails on the stub
2. **Phase: Measurement** -- summary and violation lines (AC-1, AC-2, AC-3); record the red phase (AC-4); record the first measured figures in this spec's Design Insights
   - Tests: `TestToolsSizeSummary`, `TestToolsSizeViolations`, the `.ci`
   - Files: `internal/test/cli/cmd_mcp.go`, `docs/functional-tests.md`
3. **Phase: Encode failure surfaces** -- AC-5
   - Tests: `TestGenerateToolsReportsEncodeFailure`
   - Files: `internal/component/mcp/tools.go`, `internal/component/mcp/streamable_tools.go`, `ai/digests/mcp.md`, `docs/architecture/mcp/overview.md`
4. **Phase: Real-client lane page** -- AC-6; Thomas runs it (AC-7) and the result is recorded; a connectivity failure opens the R-1 spec
   - Files: `docs/contributing/mcp-client-verification.md`, `features/mcp-integration.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Both user stories have a working path |
| Correctness | Bytes are measured on the served JSON, not on a re-encoding; depth counts nested `properties`/`items` levels |
| Naming | Output keys follow the existing `tools-order` line style |
| Data flow | Nothing re-derives tools outside the daemon |
| Rule: `ai/rules/principles.md` | No silently partial tool list remains |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `tools-size` verb | `./le test functional plugin` running `mcp-tools-list-structural-limits` |
| Encode failure reported | `TestGenerateToolsReportsEncodeFailure` |
| Lane page | `ls docs/contributing/mcp-client-verification.md` and `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The verb parses an untrusted server answer: malformed JSON fails with a message, never a panic |
| Error leakage | The AC-5 error names the tool and the encode error only, no internal state |
| Credentials | The lane page keeps the bearer token out of recorded results |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| The structural `.ci` is red on first run | A-1 broken: a real served tool breaks a limit; fix the producing code under this spec |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

First measured catalog, 2026-11-09, served by `ze --mcp $PORT` with the full
plugin set and the `.ci`'s one-peer BGP config:

| Figure | Value |
|--------|-------|
| Tools | 142 |
| Whole `tools` array | 109,108 bytes |
| Largest tool | `ze_show_ospf`, 5,670 bytes |
| Largest input schema | 4,662 bytes (35% of the 13,312-byte budget) |
| Schema depth | 2 |
| Bare enums, combinators | 0, 0 |

The structural limits are far from binding; the open question for a reduction
spec is total catalog bytes (about 109 KB), which only the G-3 lane can turn
into tokens.

AC-4 red phase, 2026-11-09: with `toolSchemaBytesMax` lowered to 100 in
`internal/test/cli/cmd_mcp_calls.go`, `./le --name session test bgp plugin
mcp-tools-list-structural-limits` failed with `stdout_mismatch`, "output
unexpectedly contains \"tools-size violation\"", printing one violation per
tool (for example `tools-size violation tool=ze_clear_bgp limit=schema-bytes
value=868`). The limit was restored to 13312 and the test rerun green.

AC-5 rests on a test seam: no value the registry carries can make the schema
encoder fail (every schema value is a string, a string list or a map of
those), so `TestGenerateToolsReportsEncodeFailure` swaps `encodeToolSchema` in
`internal/component/mcp/tools.go`. The race-detector run of the whole package
passed with the swap in place.

The `.ci` runs `le test mcp`, which the launcher builds from HEAD, so before
the commit lands it passes only under `./le --name session` (a working-tree
build); the HEAD build sends `tools-size` to the daemon as a command and gets
`unknown command`.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Measure through `le test mcp` against a running daemon | An in-process `le` report building the catalog | The in-process path skips the daemon's YANG and dispatcher enrichment, so it would measure a copy |
| Measure first, no compact profile | Build REA's compact profile now | Ze's schemas are flat; the profile's defects are not produced. A reduction waits for figures |
| Report total size, cap structure only | Cap total catalog bytes now | No figure exists to set a cap from |
| Encode failure fails the whole `tools/list` with a named error | Log and omit the tool | An omitted tool is a silently wrong answer (`ai/rules/principles.md`) |
| Real-client lane run only by Thomas | A CI job | It spends model credits under his account |

## Known Limitations
- Token cost is measured only by the owner-run lane; the `.ci` measures bytes, and bytes do not establish tokens (REA `docs/mcp-catalog-evaluation.md`)
- Catalog reduction, and any connectivity fix the lane reveals, are their own specs

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
- [ ] Targeted runs pass in the main checkout: `./le test functional plugin` (the new `.ci` and the existing `mcp-*` tests), and the unit tests of `internal/test/cli` and `internal/component/mcp` run through `./le job run label unit-pkg command go test -tags ...`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the two numeric limits
- [ ] Functional `.ci` test for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/features/spec-rea-practices-1-mcp-catalog-cost.md` only

## Implementation Summary

### What Was Implemented
- `le test mcp` verb `tools-size` (`internal/test/cli/cmd_mcp.go` `sizeDirective`; `internal/test/cli/cmd_mcp_calls.go` `measureToolsSize`, `measureSchemaShape`, `toolsSize.render`), measuring the served `tools/list` bytes
- `test/plugin/mcp-tools-list-structural-limits.ci`, asserting the summary line with a tool count of at least 3 and no violation line
- `generateTools` and `buildToolDef` (`internal/component/mcp/tools.go`) return an error naming the tool when its input schema does not encode; `allTools` (`internal/component/mcp/streamable_tools.go`) answers it as a JSON-RPC error instead of a list without the tool
- The owner-run real-client lane `docs/contributing/mcp-client-verification.md`, and its first round of results (2026-11-11)
- Landed in a6199d11d8; the lane page's daemon start command was fixed in 32b19eeeeb

### Bugs Found/Fixed
- A tool whose schema failed to encode vanished from `tools/list` with no log line and no error (`buildToolDef` returned nil, `generateTools` skipped it). Now an error; covered by `TestGenerateToolsReportsEncodeFailure`

### Documentation Updates
- a6199d11d8: `ai/digests/mcp.md`, `docs/architecture/mcp/overview.md`, `docs/functional-tests.md`, `docs/guide/mcp/overview.md`, `docs/contributing/README.md`, `ai/INDEX.md`, `docs/contributing/mcp-client-verification.md`
- Closure: `docs/contributing/mcp-client-verification.md` Results table (two client rows, written from Thomas's run), `docs/features/mcp-integration.md` sections "Client compatibility" (anchor `headers.go -- initializeEraError`) and the `tools-size` paragraph under "Testing" (anchors `cmd_mcp.go -- sizeDirective`, `cmd_mcp_calls.go -- measureToolsSize`), `features/mcp-integration.md` Extra criteria pointer and Doc review
- `./le doc check all` 2026-11-11: only red is drift of `../wiki/command-catalog.md` and `../gh-pages/data/cli-commands.json` against the live command catalog, outside this repository and not from this change; link, anchor and digest checks pass

### Deviations from Plan
- R-1's mitigation said a client that cannot connect opens a fix spec. Thomas decided on 2026-11-11 that Ze keeps revision `2026-07-28` only and Codex is recorded as unsupported until it updates, so no connectivity spec is opened
- `docs/architecture/testing/ci-format.md` gained nothing: it lists no `le test mcp` script verbs (grep for `tools-order-stable` finds none), so the "if the page lists" condition does not hold
- `features/mcp-integration.md` Extra criteria pointed at `test/interop/mcp-third-party-client.md`, which does not exist, and named an `initialize` the revision does not have. It now reads "a third-party MCP client connects and completes tools/call" and points at the lane page. The declared level stays experimental

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed current clients speak `2026-07-28` | Codex 0.167.1 opens with the `2025-06-18` `initialize` handshake and cannot connect | Lane round 1, 2026-11-11 | Recorded as a valid AC-7 row; Thomas decided Codex is unsupported until it updates |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| G-1 measure and print catalog size and shape | Done | `internal/test/cli/cmd_mcp_calls.go` `measureToolsSize` | From the served answer |
| G-2 functional test refuses a catalog breaking limits | Done | `test/plugin/mcp-tools-list-structural-limits.ci` | Red recorded (AC-4) |
| G-3 documented owner-run lane with results | Done | `docs/contributing/mcp-client-verification.md` | Two rows |
| G-4 build failure reported, never dropped | Done | `internal/component/mcp/tools.go` `generateTools`, `buildToolDef`; `streamable_tools.go` `allTools` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestToolsSizeSummary`; `.ci` summary pattern | |
| AC-2 | Done | `TestToolsSizeViolations` | Each limit at and above its bound |
| AC-3 | Done | `mcp-tools-list-structural-limits.ci` | `count=([3-9]\|[1-9][0-9]+)`, `reject=stdout:contains=tools-size violation` |
| AC-4 | Done | Design Insights, red phase 2026-11-09 | `toolSchemaBytesMax` lowered to 100, `stdout_mismatch` |
| AC-5 | Done | `TestGenerateToolsReportsEncodeFailure` | Through `generateTools` and through `dispatchMethod` |
| AC-6 | Done | `docs/contributing/mcp-client-verification.md` | Steps, clients, recorded fields, results, owner-only |
| AC-7 | Done | Lane Results table | Claude Code 2.1.322 connected and called `ze_reference`; Codex 0.167.1 could not connect, a valid row under AC-7's survivor rule |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestToolsSizeSummary` | Done | `internal/test/cli/cmd_mcp_test.go` | |
| `TestToolsSizeViolations` | Done | `internal/test/cli/cmd_mcp_test.go` | Boundary 13312/13313 and 10/11 |
| `TestGenerateToolsReportsEncodeFailure` | Done | `internal/component/mcp/tools_test.go` | |
| `mcp-tools-list-structural-limits` | Done | `test/plugin/mcp-tools-list-structural-limits.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/cli/cmd_mcp.go` | Done | Verb and help text |
| `internal/component/mcp/tools.go` | Done | |
| `internal/component/mcp/streamable_tools.go` | Done | |
| `docs/functional-tests.md` | Done | |
| `docs/architecture/mcp/overview.md` | Done | |
| `docs/guide/mcp/overview.md` | Done | |
| `docs/architecture/testing/ci-format.md` | Changed | Lists no verbs; see Deviations |
| `ai/digests/mcp.md` | Done | |
| `features/mcp-integration.md` | Done | Closure commit |
| `test/plugin/mcp-tools-list-structural-limits.ci` | Done | |
| `docs/contributing/mcp-client-verification.md` | Done | |

### Audit Summary
- **Total items:** 30
- **Done:** 29
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`ci-format.md`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| G-1 | functional | `mcp-tools-list-structural-limits.ci` against `ze --mcp $PORT`: 142 tools, 109,108 bytes, largest `ze_show_ospf` 5,670 bytes, max schema 4,662 bytes, depth 2 (Design Insights, 2026-11-09) |
| G-2 | functional, with recorded red | Same `.ci`; with `toolSchemaBytesMax` at 100 it failed `stdout_mismatch` naming each tool, green again at 13312 (AC-4) |
| G-3 | owner-run interop lane | `docs/contributing/mcp-client-verification.md` Results: Claude Code 2.1.322 connected, called `ze_reference`, 76,211 vs 166,973 input tokens; Codex 0.167.1 refused at the `initialize` handshake |
| G-4 | unit through dispatch | `TestGenerateToolsReportsEncodeFailure`: no partial list, error wraps `*json.UnsupportedValueError` and names `ze_show_probe`, and `dispatchMethod` answers `tools/list` with a JSON-RPC error |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every AC is met; catalog reduction was out of scope by design until figures exist, and Codex connectivity is not owed by Thomas's 2026-11-11 decision | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rea-practices-1-mcp-catalog-cost-ad9def1f-df41-4fcc-bc2d-9569a17e6b8c.md` (14 files: the closure's six and the eight code and test files of a6199d11d8, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | wiring, functional coverage, removed-behavior audit, logic and edge cases of the schema walk, security (untrusted server answer, error leakage, token in results), style, docs drift, citers |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | No BLOCKER or ISSUE. NOTE: `./le test weakened check` reports six weakenings, all in `internal/le/repo/inventory/*_test.go`, another session's work and not in this diff. NOTE: `features/mcp-integration.md` held a dangling Extra-criteria path; repointed in this closure | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/mcp-tools-list-structural-limits.ci` | yes | 54 lines in a6199d11d8 |
| `docs/contributing/mcp-client-verification.md` | yes | 138 lines in a6199d11d8, Results rows in the working tree |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-3 | Summary line asserted | `.ci` line 53 `expect=stdout:pattern=tools-size count=...` |
| AC-2 | Violation lines | `toolsSize.add` appends one `toolsSizeViolation` per broken limit |
| AC-5 | Error, no partial list | `generateTools` returns `nil, err`; `allTools` wraps it; `dispatchMethod` answers `s.fail(..., err.Error())` |
| AC-6, AC-7 | Lane and rows | `docs/contributing/mcp-client-verification.md` Results, two rows dated 2026-11-11 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `le test mcp` line `tools-size` | `test/plugin/mcp-tools-list-structural-limits.ci` | Read: line 21 sends `tools-size`, line 50 starts `ze --mcp $PORT`; `CmdMcp` `directive` calls `sizeDirective` |
| `tools/list` with an unencodable schema | `TestGenerateToolsReportsEncodeFailure` (unit) | Read: drives `dispatchMethod` with `methodToolsList` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | max-depth=2, bare-enum=0, combinators=0 over 142 tools |
| A-2 | broken for Codex, confirmed for Claude Code | Lane Results rows; Mistake Log row |
| A-3 | confirmed | Both rows carry reported input tokens |
| A-4 | confirmed as far as observed | Claude Code loaded the catalog with no structural refusal |
| A-5 | confirmed | Claude Code connected with the bearer header |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature declaration and page | `features/mcp-integration.md`, `docs/features/mcp-integration.md`; `-32020` read in `headers.go` `initializeEraError` and its header comment | yes |
| API/RPC: `tools/list` error path | `streamable_tools.go` `allTools` | yes |
| Test infrastructure | `docs/functional-tests.md` (a6199d11d8); `ci-format.md` lists no verbs | yes |
| Digest | `ai/digests/mcp.md` (a6199d11d8); `./le doc check all` digest anchors all resolve | yes |
