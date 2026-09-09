# Spec: subscriber-utilisation-has-no-operator-view

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Depends | plan/immediate/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**An operator cannot ask Ze how much traffic one subscriber has passed. The
numbers exist; nothing shows them where an operator looks.**

accel-ppp answers this with one command: `accel-cmd show sessions` lists every
session with its rx and tx counters beside the username and the uptime. Ze has
the same counters, read from the `pppN` netdev over netlink and baseline-corrected
per session, and reaches them from three places. None of the three is where an
operator would look.

`show subscriber`, the access-type-neutral view, prints identity and state and no
counters at all: `sessionBrief` and `sessionFull`
(`internal/component/l2tp/subscriber/cmd/subscriber.go`) carry username, address,
duration and rates, never bytes. `show l2tp session id` prints no counters either,
from `sessionJSON` (`internal/component/l2tp/cmd/l2tp.go`), **while its own YANG
help text promises "traffic counters"**, so the CLI documents a field it does not
emit. The counters live in a separate command, `show l2tp session traffic`, built
by `sessionTrafficAll` and `sessionTrafficRow` in the same file, and that command
has no per-session form: its YANG container declares no id leaf, so an operator
with one subscriber in mind must print every session and filter.

The web UI shows sessions and no counters (`handler_l2tp.go` with
`l2tp_list.templ` and `l2tp_detail.templ`). gNMI has no subscriber path at all.

The goal: an operator asks about one subscriber, by the identity they already
know, and gets that subscriber's traffic, on the surface they are already using.

**This spec does not collect anything.** Collection exists, and extending it to
PPPoE subscribers is the dependency named above. This spec is the views.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the design document `subscriber/cmd/subscriber.go` declares
  → Decision: `show subscriber` is the access-type-neutral view, so it is the primary surface this spec must reach. A counter that appears only under `show l2tp` answers for one access type and is the shape being removed
  → Constraint: the page is silent on what a subscriber view owes an operator, so it gains the statement
- [ ] `docs/guide/l2tp.md` - the design document `internal/component/l2tp/cmd/l2tp.go` declares
  → Constraint: the page never documents `show l2tp session traffic`, so the one command that does show counters today is undiscoverable from the guide
- [ ] `docs/architecture/web-workbench-pages.md` - the design document `view_l2tp.go` declares
  → Constraint: the web view is built from a view model, so a counter reaches the page by being on that model, not by the template reading the registry
- [ ] `ai/rules/cli.md` - the command grammar
  → Constraint: keyword before value, so a per-session form is `... session id <id> traffic` or equivalent, never a bare identifier in `args[0]`
  → Constraint: every command supports all pipe operators and its payload is structured data, so `| json`, `| yaml` and `| table` each render whatever this spec adds
- [ ] `ai/patterns/cli-command.md` - the structural template for a command
  → Constraint: read it before adding the per-session form; a new verb, its YANG node and its handler are one shape

### RFC Summaries (Scope: protocol)
- [ ] Not applicable. This spec adds no protocol behavior: it renders counters the dependency spec already produces. No `rfc/short/` row changes.

**Key insights:**
- The one command that shows counters cannot be asked about one session, and the one command an operator asks about one session does not show counters. Each half of the answer exists in the other command.
- The YANG help promising traffic counters that `sessionJSON` never emits is a defect on its own: an operator reading the help learns something untrue.
- `show subscriber` is the surface that matters most, because it is the only one that already answers for both access types.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/subscriber/cmd/subscriber.go` - `sessionBrief` and `sessionFull` build the `show subscriber` payload from `subscriber.Session`: identity, state, addresses, duration and configured rates, and no byte or packet counters
- [ ] `internal/component/l2tp/cmd/l2tp.go` - `sessionJSON` builds the per-session payload with no counters; `handleSessionTraffic`, `sessionTrafficAll` and `sessionTrafficRow` build the counter view, over every session
- [ ] `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` - the `show l2tp session id` node whose help promises traffic counters, and the `session traffic` container which declares no id leaf
- [ ] `internal/component/web/handler_l2tp.go`, `view_l2tp.go`, `l2tp_list.templ`, `l2tp_detail.templ` - the session list and detail pages and their view model, none carrying a counter field
- [ ] `internal/component/iface/dispatch.go` - `GetStats`, the read every counter surface uses
- [ ] `internal/component/l2tp/subscriber/session.go` - `Session`, the record every view renders

**Behavior to preserve:**
- `show l2tp session traffic` keeps working and keeps its current output shape, because an operator's scripts may parse it.
- `show subscriber`'s existing fields keep their names and their JSON keys.
- Every command keeps working through all pipe operators, with structured payloads.
- The web session list keeps its current columns; counters are added, nothing is displaced.

**Behavior to change:**
- `show subscriber` carries per-session traffic counters.
- A per-session form of the traffic view exists, addressed by the identity an operator has.
- `show l2tp session id` either emits the traffic counters its help promises, or the help stops promising them. The first is what this spec does.
- The web session list and detail pages show counters.
- A gNMI path exposes them.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator command typed at the CLI or issued over the API: `show subscriber`, `show subscriber id <id>`, `show l2tp session id <id>`, or the new per-session traffic form.
- A web request for the session list or a session detail page.
- A gNMI Get or Subscribe for the subscriber path.
- Format at entry: a command string parsed against the YANG command tree, or an HTTP request, or a gNMI path.

### Transformation Path
1. The command tree resolves the verb and its arguments to a handler.
2. The handler reads `subscriber.DefaultRegistry` for the session or sessions named.
3. For each, `iface.GetStats` supplies the baseline-corrected counters for its PPP interface.
4. The handler builds a structured payload, which the pipe operators render as text, JSON, YAML or a table.
5. The web handler builds its view model from the same read; the gNMI path serves the same values.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator → daemon | The CLI over SSH, or the API command dispatcher | No |
| Handler → subscriber registry | `subscriber.DefaultRegistry` | No |
| Handler → kernel | `iface.GetStats` per session | No |
| Daemon → web | The view model behind `l2tp_list.templ` and `l2tp_detail.templ` | No |
| Daemon → gNMI client | The subscriber path, which does not exist yet | No |

### Integration Points
- `sessionBrief` and `sessionFull` (`subscriber/cmd/subscriber.go`) - where the access-type-neutral view gains counters.
- `sessionJSON` (`l2tp/cmd/l2tp.go`) - where the per-session view stops lying about what its help promises.
- `sessionTrafficRow` (`l2tp/cmd/l2tp.go`) - the existing row builder a per-session form reuses rather than duplicating.
- `view_l2tp.go` and its templates - the web view model.
- The gNMI registration - the new path.

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
| A-1 | Reading counters inside a show handler is cheap enough to do per session on a list command | `GetStats` is a netlink read per interface, and the existing traffic command already does it for every session | A list command on a concentrator with thousands of subscribers becomes slow or hammers netlink | Time `show l2tp session traffic` against a populated registry, and read whether `sessionTrafficAll` reads serially | unvalidated |
| A-2 | The counters an operator wants are the same baseline-corrected values accounting sends, not raw interface totals | `applyBaseline` exists precisely so a reused `pppN` does not inflate a session | The CLI and the RADIUS record disagree about one subscriber's usage, which is worse than showing nothing | Read `applyBaseline` and compare what `sessionTrafficRow` and `buildAcctPacket` each report for one session | unvalidated |
| A-3 | The web session pages are reachable in a shipped build and not behind a feature gate that is off by default | `page_l2tp_off.go` exists beside `page_l2tp.go`, which suggests a build-tag pair | The web work lands in code nobody runs | Read the build tags on both files and the gate that selects them | unvalidated |
| A-4 | gNMI has no subscriber or session path today, so this adds a first one rather than extending an existing tree | No l2tp path was found under the gnmi component | The work is an extension with an existing schema to respect | Read the gNMI path registration and list what it serves | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding a counter read to `show subscriber` makes the common case slow, because the command is now doing per-session netlink work it never did | The command takes visibly longer with many sessions | Counters are read once per command rather than once per session where the kernel allows it, and a large-registry test measures the cost before the shape is fixed |
| R-2 | The CLI and the RADIUS record report different numbers for the same subscriber, because one applies the baseline and the other does not | A functional test comparing the two disagrees | A-2 forces the comparison before implementation, and one test asserts the two surfaces agree for one session |
| R-3 | Adding fields to `show subscriber`'s JSON breaks an operator's script that parses it positionally | Nothing signals this until a user reports it | Fields are ADDED, never renamed or reordered, and the JSON keys of existing fields are asserted unchanged by a golden test |
| R-4 | A per-session traffic form duplicates `sessionTrafficRow` rather than reusing it, so the two drift | Two row builders appear in the same file | The new form calls the existing row builder; a review check names this |
| R-5 | The gNMI path is added with no consumer and no test that reaches it, so it becomes an unwired feature | Nothing exercises the path outside a unit test | The Wiring Test table names a test that drives the path end to end, and the work is dropped rather than shipped unwired if that test cannot exist |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator reads a wrong usage figure for a subscriber, or a list command becomes slow on a busy concentrator |
| How is it reverted? | Single commit revert. Additive fields and one new command form, so nothing an operator depended on is removed |
| Who else touches this path? | The dependency spec changes what the counters cover; `plan/spec-l2tp-ipv6-subscriber.md` adds sessions these views must render |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show subscriber` typed at the CLI | → | `sessionBrief` (`subscriber/cmd/subscriber.go`) | `TestShowSubscriberCarriesTraffic` |
| `show l2tp session id <id>` typed at the CLI | → | `sessionJSON` (`l2tp/cmd/l2tp.go`) | `TestShowSessionIDEmitsTheCountersItsHelpPromises` |
| The per-session traffic form typed at the CLI | → | `handleSessionTraffic` → `sessionTrafficRow` | `TestSessionTrafficByID` |
| A web request for the session list | → | the view model in `view_l2tp.go` | `TestWebSessionListShowsTraffic` |
| A gNMI Get for the subscriber path | → | the gNMI path handler | `TestGNMISubscriberTrafficPath` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show subscriber` with sessions of both access types live | Each row carries rx bytes, tx bytes, rx packets, tx packets and the uptime, beside the identity fields it prints today |
| AC-2 | `show subscriber` piped through `json`, `yaml` and `table` | Each renders the counters, because the payload is structured data rather than preformatted text |
| AC-3 | `show l2tp session id <id>` | The output carries the traffic counters its YANG help promises |
| AC-4 | The per-session traffic form, given a session id that exists | Only that session's counters are printed |
| AC-5 | The same form given an id that does not exist | An error naming the id, and a non-zero exit code; never an empty success |
| AC-6 | A session whose counters could not be read | The field says so and is distinguishable from zero, on every surface |
| AC-7 | The web session list and a session detail page | Both show the counters, with the list's existing columns intact |
| AC-8 | A gNMI Get on the subscriber path | Returns the same values the CLI prints for the same session |
| AC-9 | The CLI and the RADIUS accounting record for one session, read at the same moment | Report the same baseline-corrected numbers |
| AC-10 | `show subscriber` against a registry holding many sessions | Completes within the same order of time as it does today, measured rather than asserted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks how much one subscriber has used | `show subscriber id <id>` → registry → `GetStats` → payload | `TestShowSubscriberCarriesTraffic` |
| 2 | Lists every subscriber with usage, sorted, as JSON | `show subscriber \| json` → structured payload | `TestShowSubscriberPipesRenderTraffic` |
| 3 | Reads one subscriber's usage from a dashboard | web request → view model → detail page | `TestWebSessionDetailShowsTraffic` |
| 4 | Polls usage from an automation system | gNMI Get → subscriber path | `TestGNMISubscriberTrafficPath` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowSubscriberCarriesTraffic` | `internal/component/l2tp/subscriber/cmd/subscriber_test.go` | AC-1 | |
| `TestShowSubscriberPipesRenderTraffic` | `internal/component/l2tp/subscriber/cmd/subscriber_test.go` | AC-2 | |
| `TestShowSubscriberKeysUnchanged` | `internal/component/l2tp/subscriber/cmd/subscriber_test.go` | R-3, a golden assertion over the existing JSON keys | |
| `TestShowSessionIDEmitsTheCountersItsHelpPromises` | `internal/component/l2tp/cmd/l2tp_test.go` | AC-3 | |
| `TestSessionTrafficByID` | `internal/component/l2tp/cmd/l2tp_test.go` | AC-4 | |
| `TestSessionTrafficUnknownIDIsAnError` | `internal/component/l2tp/cmd/l2tp_test.go` | AC-5 | |
| `TestUnreadableCounterIsNotZero` | `internal/component/l2tp/subscriber/cmd/subscriber_test.go` | AC-6 | |
| `TestWebSessionListShowsTraffic` | `internal/component/web/handler_l2tp_test.go` | AC-7 | |
| `TestWebSessionDetailShowsTraffic` | `internal/component/web/handler_l2tp_test.go` | AC-7 | |
| `TestGNMISubscriberTrafficPath` | the gNMI component's test file | AC-8 | |
| `TestCLIAndAccountingAgree` | `internal/component/l2tp/cmd/l2tp_test.go` | AC-9 and R-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| session id argument | the id forms the registry accepts | a live session's id | an empty argument is a usage error | an unknown id is an error, not an empty success |
| counter value rendered | 0 to 2^64-1 | the largest the kernel reports | N/A | N/A |
| session count in the registry | 0 upward | the configured `max-sessions` | an empty registry prints a header and no rows, never an error | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `subscriber-traffic-view` | `test/pppoe/subscriber-traffic-view.ci` | An operator connects a subscriber, passes traffic, and reads the usage back from `show subscriber` and from the per-session form | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | This spec adds no wire behavior. The counters it renders are proven against a real peer by the dependency spec's `radius-accounting-pppoe` scenario | N-A |

## Files to Modify
- `internal/component/l2tp/subscriber/cmd/subscriber.go` - `sessionBrief` and `sessionFull` gain the counters
- `internal/component/l2tp/cmd/l2tp.go` - `sessionJSON` emits what the help promises; the per-session traffic form reuses `sessionTrafficRow`
- `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` - the id leaf the traffic container lacks, and the help text that must match what is emitted
- `internal/component/l2tp/subscriber/cmd/yang/` - the subscriber command module, for any new node
- `internal/component/web/view_l2tp.go` - the view model gains the counters
- `internal/component/web/handler_l2tp.go` - the handler fills them
- `internal/component/web/l2tp_list.templ` and `l2tp_detail.templ` - the rendered columns and fields
- `docs/architecture/l2tp/subscriber-session-model.md` - the design document `subscriber/cmd/subscriber.go` declares
- `docs/guide/l2tp.md` - the design document `internal/component/l2tp/cmd/l2tp.go` declares, and the page that never documented the traffic command
- `docs/architecture/web-workbench-pages.md` - the design document `view_l2tp.go` declares
- `docs/architecture/web-interface.md` - the design document `handler_l2tp.go` and the session templates declare
- `docs/guide/command-reference.md` - the new and changed commands

## Files to Create
- `test/pppoe/subscriber-traffic-view.ci` - the operator path, named in `netnsSelections` (`internal/le/qemu/netns_linux.go`), which is an explicit list and not a directory scan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The id leaf on the traffic container, and any new subscriber node |
| YANG validation constraints | Yes | The id leaf takes the type and pattern the registry's ids actually use, not a bare `string` |
| YANG custom validators | Yes if native constraints cannot express a session id; decide when the leaf is written | `ai/patterns/config-option.md` |
| CLI commands/flags | Yes | The per-session traffic form, `internal/component/l2tp/cmd/l2tp.go` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`: the id follows a keyword, and `args[0]` stays a keyword |
| Editor autocomplete | Yes | A session id is a dynamic value, so it needs a `CompleteFn` rather than a static enum |
| Functional test for new RPC/API | Yes | `test/pppoe/subscriber-traffic-view.ci` |
| Pipe completeness | Yes | The payload is structured data so `json`, `yaml` and `table` all render it (`ai/rules/cli.md`) |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency: the netlink read already exists |
| Prometheus counters/metrics | N-A | The dependency spec owns the metrics surface |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: per-subscriber usage reporting |
| 2 | Config syntax changed? | No | No config leaves |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if the command surface is published there |
| 5 | Plugin added/changed? | No | No plugin change |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` and `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | No wire change |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC row changes; the dependency spec owns them |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new `.ci` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: `accel-cmd show sessions` is the comparison point |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/web-workbench-pages.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | The dependency spec owns them |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A new command registers: `docs/guide/status.md` and the command inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-subscriber-utilisation-has-no-operator-view.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` never documented `show l2tp session traffic`, and its examples are checked against the command tree |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- validate the assumptions, then make one surface carry one counter
   - Tests: `TestShowSubscriberCarriesTraffic`
   - Files: `subscriber/cmd/subscriber.go`, and the reading A-1 to A-4 require
   - Verify: all four assumptions flipped with evidence. A-3 decides whether the web work is reachable at all, and A-4 whether gNMI is an addition or an extension. If A-3 says the pages are off by default, raise it before the web phase runs
2. **Phase: the CLI, honestly** -- `show subscriber` and the per-session form, reusing the existing row builder
   - Tests: `TestShowSubscriberPipesRenderTraffic`, `TestShowSubscriberKeysUnchanged`, `TestSessionTrafficByID`, `TestSessionTrafficUnknownIDIsAnError`, `TestUnreadableCounterIsNotZero`
   - Files: `subscriber/cmd/subscriber.go`, `l2tp/cmd/l2tp.go`, both YANG command modules
   - Verify: the grammar check from `ai/rules/cli.md` run mechanically, `args[0]` a keyword in every new form
3. **Phase: the help stops lying** -- `sessionJSON` emits the counters `show l2tp session id`'s help promises
   - Tests: `TestShowSessionIDEmitsTheCountersItsHelpPromises`
   - Files: `l2tp/cmd/l2tp.go`, `ze-l2tp-cmd.yang`
   - Verify: the help text and the emitted payload compared field by field, not read for plausibility
4. **Phase: the web** -- the view model, the handler and both templates
   - Tests: `TestWebSessionListShowsTraffic`, `TestWebSessionDetailShowsTraffic`
   - Files: `view_l2tp.go`, `handler_l2tp.go`, `l2tp_list.templ`, `l2tp_detail.templ`
   - Verify: templates regenerated by their own generator rather than hand-edited, and the existing columns unchanged
5. **Phase: gNMI, or the decision not to** -- the subscriber path
   - Tests: `TestGNMISubscriberTrafficPath`
   - Files: the gNMI component's registration
   - Verify: R-5 holds. A path with no test that reaches it end to end is not shipped: it is dropped, and the drop becomes its own spec named here
6. **Phase: the operator path and the cost** -- the functional test and the measurement A-1 and AC-10 need
   - Tests: `subscriber-traffic-view.ci`, `TestCLIAndAccountingAgree`
   - Files: `test/pppoe/`, `internal/le/qemu/netns_linux.go`
   - Verify: the list command timed against a large registry, the number recorded rather than asserted to be fine

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | An operator can reach a subscriber's usage from the CLI, the web and gNMI, or the missing one is a named spec rather than a silent gap |
| Correctness | The CLI's numbers equal the accounting record's for the same session; an unreadable counter is never rendered as zero |
| Naming | The field names say bytes and packets and direction, and match the names the metrics and the RADIUS attributes already use |
| Data flow | One row builder feeds the all-sessions and per-session forms; the web view model and the CLI payload read the same source |
| Rule: `ai/rules/cli.md` | Keyword before value in every new form; all pipes render the payload |
| Rule: `ai/rules/principles.md` | The unreadable-counter case is distinguishable on every surface, not just the one that was easiest |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `show subscriber` carries counters | `TestShowSubscriberCarriesTraffic` passes |
| A per-session traffic form exists | `TestSessionTrafficByID` passes, and the YANG carries the id leaf |
| The help no longer promises what is not emitted | The help text and the payload compared in `TestShowSessionIDEmitsTheCountersItsHelpPromises` |
| One row builder, not two | `gopls references` on `sessionTrafficRow` shows both forms calling it |
| The web pages show counters | The two web tests pass |
| gNMI serves the path, or its absence is a named spec | The gNMI test, or the spec path written into Known Limitations |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The session id is operator-supplied: it is matched against the registry, never used to build a path or a command |
| Authorization | Subscriber usage is customer data. Confirm the command sits behind the same authorization as the existing subscriber views, and that the web page and the gNMI path do too |
| Error leakage | An unknown id error names the id the operator typed, not the ids that exist |
| Resource exhaustion | A list command reading counters per session is a cost an unauthenticated caller must not be able to trigger; confirm the web and gNMI paths are authenticated |
| Fail-closed guard | An unreadable counter renders as unknown, never as zero |

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

- The counters were reachable from three places and none of them was where an operator asks the question. That is a discoverability defect rather than a missing feature, and it is why the work is mostly rendering.
- A help text promising a field the handler never emits is the cheapest kind of lie to ship and the hardest for a gate to catch, because both halves are individually correct.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Put counters on `show subscriber` first | Extend only `show l2tp session traffic` | `show subscriber` is the access-type-neutral view, so it answers for PPPoE and L2TP alike. A counter reachable only under `show l2tp` repeats the split the dependency spec exists to remove |
| Add a per-session form to the existing traffic command | A new top-level usage command | The row builder, the payload and the pipe handling already exist. A new command would duplicate all three to gain one argument |
| Make `sessionJSON` emit what the help promises | Change the help to stop promising it | The counters exist and an operator was told they would be there. Removing the promise is the smaller change and the worse one |

## Known Limitations
- No historical usage: every surface reports the counters as they are now. A usage history is a store this spec does not build, and is not attempted.
- No shaper-side figures. `internal/component/traffic/model.go` holds rates and burst with no statistics readback, so "bytes dropped by the rate limiter" is unreachable from this work.
- The counters this spec renders cover only the access types the dependency spec reaches. Until that lands, a PPPoE subscriber's row shows what the collection has, which is nothing.

## RFC Documentation (Scope: protocol)

Not applicable. This spec renders values; it implements no protocol behavior and
changes no `rfc/short/` row.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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
