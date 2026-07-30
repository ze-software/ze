# Spec: mcp2026-2-mrtr

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-mcp2026-1-stateless-core |
| Phase | 2/4 |
| Deferral shard | `plan/deferrals/mcp2026-2-mrtr.md` |
| Updated | 2026-07-30 |

Parent: `plan/spec-mcp2026-0-umbrella.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace Ze's server-initiated elicitation with the Multi Round-Trip Requests
(MRTR) pattern.

Today a handler calls `session.Elicit(ctx, message, schema)`
(`internal/component/mcp/elicit.go:227`), which upgrades the POST reply sink to
SSE (`elicit.go:252`), writes an `elicitation/create` JSON-RPC **request** frame
to the client (`elicit.go:259`, frame built at `elicit.go:291-302`), and
**blocks the handler goroutine** on a per-elicit channel (`elicit.go:263-269`)
until the client POSTs a correlated JSON-RPC **response** that
`handleElicitResponse` routes back (`streamable.go:603`). The `2026-07-28`
transport forbids both halves of that exchange:

> "The server **MUST NOT** send independent JSON-RPC *requests* on this stream."
> "The client **MUST NOT** send JSON-RPC *responses*."

Under MRTR the server instead **returns** `resultType: "input_required"` with an
`inputRequests` map, terminating the original request. The client gathers the
input and **retries the original request** (with a different JSON-RPC id)
carrying `inputResponses`. The server processing the retry needs nothing beyond
what is in the retry.

**Research collapsed this phase.** The design that this spec carried at
`skeleton` assumed a general continuation-state problem and budgeted an
HMAC/AEAD `requestState` with principal binding, TTL and request-digest. That
assumption is false for Ze, and the correct design is much smaller. See
"The one call site" and Key Design Decisions below.

### The one call site

A tree-wide grep for `\.Elicit(` over `internal/ cmd/`, excluding `_test.go`,
returns exactly one hit: `internal/component/mcp/tools.go:747`, inside the
`ze_execute` handler declared at `internal/component/mcp/tools.go:722`.

| Fact | Evidence |
|------|----------|
| Exactly one reachable elicitation call site | `internal/component/mcp/tools.go:747` (handler declared `tools.go:722`) |
| It elicits one string, named `command` | `tools.go:747-758` builds a single-property flat schema |
| The elicited value is assigned to a **tool argument** and nothing else | `tools.go:766` assigns to `input.Command`, which the retry would carry anyway |
| Nothing else is suspended across the wait | `tools.go:768` dispatches on `input.Command` alone; no other local outlives the `Elicit` call |
| `TaskElicit` is a second producer with **no production caller** | `internal/component/mcp/tasks.go:464`; every other reference is in `internal/component/mcp/task_elicit_test.go` |

The entire state suspended across Ze's only elicitation is one string that is
itself a tool argument. There is no continuation state to carry.

`TaskElicit` (`tasks.go:464`) is exported but unwired: it is dead code under
`ai/rules/wiring-completeness.md`, tested only by its own tests. It is deleted
in this phase, not ported.

**Delete:**

| What | Where |
|------|-------|
| `session.Elicit` and the blocking channel model | `elicit.go:212-302` |
| The elicit correlation map | `session.go:61` (`correlations`), `ResolveElicit` (`session.go:403`), `maxPendingElicits` (`session.go:108`) |
| `handleElicitResponse` and the client-response intake path | `streamable.go:544-605` |
| `elicitation/create` frame construction as a JSON-RPC request | `elicit.go:291-302` |
| `TaskElicit` and its tests (unwired dead code) | `tasks.go:456-513`, `task_elicit_test.go` |
| The POST-to-SSE sink upgrade, if Phase 1 has not already removed it | `reply_sink.go`, `UpgradeCurrentSinkToSSE` |

**Add:**

| What | Requirement |
|------|-------------|
| `InputRequiredResult` | `resultType: "input_required"` plus an `inputRequests` map. Ze emits no `requestState` (Key Design Decision 1) |
| `InputRequests` / `InputResponses` maps | Server-assigned string keys, unique within the request. Ze uses exactly one key for its one elicitation |
| Unsolicited-`requestState` guard | A `requestState` arriving on any request is rejected. Ze never mints one, so no arriving value can be legitimate (Key Design Decision 2) |
| Form-mode capability precondition | The `2026-07-28` elicitation capability carries mode sub-keys. The gate is "client supports **form** mode", not "the `elicitation` key is present" (A-4) |
| Explicit `mode: "form"` on the emitted `ElicitRequest` | New required-with-default parameter in this revision (Key Design Decision 3) |
| Handler re-entrancy | `ze_execute` becomes a function of (arguments plus `inputResponses`), resumable from scratch |
| Retry tolerance | A retry that omits the requested key gets a **new** `InputRequiredResult`, not an error. An explicit `decline` or `cancel` is a terminal answer, not a re-ask |
| Unknown-key tolerance | Unrecognised `InputResponses` entries are ignored |

**Constraints:**

- Ze is a server, so it emits `inputRequests` and consumes `inputResponses`. It never implements the client half.
- Sampling and Roots are Deprecated in `2026-07-28` and Ze implements neither. Only `elicitation/create` entries are emitted. Do not add the other two for completeness.
- `InputRequiredResult` is permitted only on `prompts/get`, `resources/read` and `tools/call`. Ze implements two of those; `prompts/*` is not implemented and stays that way (umbrella Known Limitations).
- URL-mode elicitation is not implemented. Ze's single elicitation asks for a `ze` CLI command, which is not a credential, so form mode is the correct and conformant mode (Known Limitations).
- The elicit schema validator (`elicit.go:102-167`, flat-primitive subset) is preserved rather than rewritten: `ElicitRequest.params.requestedSchema` keeps the same constraint. Its 11 unit tests (`elicit_test.go:99-317`) survive unmodified.
- R-2: the elicitation test estate is restructured, not simply deleted. Mutation-verify the replacement per `ai/rules/functional-test-gate.md`: disable the `InputRequiredResult` producer and confirm the new `.ci` tests go red.

Umbrella AC covered: AC-6.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/mcp/overview.md` - elicitation section, rewritten by this phase
- [x] `ai/rules/fail-closed-guards.md` - an unsolicited `requestState` must deny, never be silently ignored
  → Decision: Ze rejects rather than ignores. "Fail closed or say something" is satisfied by an explicit rejection plus a WARN log, not by a silent drop.
- [x] `ai/rules/error-messages.md` - rejection messages name what failed without echoing attacker bytes
- [x] `ai/rules/design-principles.md` - YAGNI
  → Decision: the `requestState` machinery is not built, because Ze has no state to put in it. See Key Design Decision 1.

### Deleted code this phase must RECOVER, not reinvent (Phase 1 R-4)

**Added 2026-07-29, closing an obligation Phase 1 recorded but never carried
here.** Phase 1's R-4 says "Phase 2's Required Reading must name the deleted
path so the validator is recovered rather than reinvented", and until now it did
not. The path is named below so the recovery is a `git show`, not a rewrite.

Phase 1 deletes `internal/component/mcp/elicit.go` and `elicit_test.go`, because
`(*session).Elicit` (`elicit.go:227`), `TaskElicit` and `handleElicitResponse`
all name the `session` type and cannot compile once it is gone. But **most of
that file has nothing to do with sessions**: the flat-primitive
`requestedSchema` validator is pure, and this phase's own Constraints section
requires it "preserved rather than rewritten", with "its 11 unit tests
(`elicit_test.go:99-317`) surviving unmodified".

- [ ] Recover the validator from Phase 1's commit A, which preserves the file in
  history by design (`ai/rules/spec-preservation.md`):
  `git show <phase-1-commit-A>:internal/component/mcp/elicit.go`
  and the same for `elicit_test.go`. Find the commit with
  `git log --oneline --diff-filter=D -- internal/component/mcp/elicit.go`.
- [ ] Symbols to recover verbatim: `validateElicitSchema` (`:102`),
  `validateElicitProperty` (`:132`), `wrapSchemaErr` (`:172`),
  `elicitSchemaError` (`:181`), `describeType` (`:194`), and the three package
  vars `elicitPrimitiveTypes` (`:72`), `elicitStringFormats` (`:80`),
  `elicitForbiddenKeywords` (`:89`). **None of these references the `session`
  type** — they were collateral in a type-driven deletion, not obsolete code.
- [ ] Do NOT recover `(*session).Elicit` (`:227`), `resolveElicitAction`
  (`:274`) or `buildElicitFrame` (`:291`). Those three are the blocking
  channel model and the server-initiated request frame, which is precisely what
  MRTR replaces.
  → Constraint: recovering the tests unmodified is the check that the validator
  came back byte-identical. If a recovered test needs editing to pass, the
  validator was rewritten rather than recovered, and that is the R-4 failure.

### Protocol Specification (Scope: protocol)
- [x] `https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr` - server requirements 1-8, client requirements 1-4, error handling, security considerations
  → Decision: requirement 6 ("Servers **MUST** include at least one of `inputRequests` or `requestState` in every `InputRequiredResult` response") is satisfied by `inputRequests` alone, so `requestState` is omitted entirely.
  → Constraint: requirement 3 is "The `InputRequiredResult` **MAY** include a `requestState` field", so omitting it is conformant, not a gap.
  → Constraint: requirement 4's integrity obligation is conditional, "**If** a client request contains a `requestState` field". Ze's guard satisfies it by rejecting.
  → Constraint: requirement 7, "Servers **MUST NOT** send an `inputRequests` that the client has not declared support for in its capabilities."
  → Constraint: client requirement 2, "If the `InputRequiredResult` does not contain a `requestState` field, the client **MUST NOT** include one in the retry." This makes an arriving `requestState` a client protocol violation.
- [x] `https://modelcontextprotocol.io/specification/2026-07-28/schema#inputrequiredresult` - field shapes; both `inputRequests` and `requestState` are optional
- [x] `https://modelcontextprotocol.io/specification/2026-07-28/client/elicitation` - `ElicitRequest` shape, capability shape, requested-schema subset
  → Constraint: "Clients that support elicitation **MUST** declare the `elicitation` capability in `_meta.io.modelcontextprotocol/clientCapabilities` on each request", with `form` and `url` sub-keys; an empty object "is equivalent to declaring support for `form` mode only".
  → Constraint: "Servers **MUST NOT** send elicitation requests with modes that are not supported by the client." This is a new gate Ze does not have today.
  → Constraint: "Servers **MUST NOT** use form mode elicitation to request sensitive information such as passwords, API keys, access tokens, or payment credentials."
  → Decision: "Elicitations do not require that the server maintain state about users with the multi round-trip requests mechanism" is the specification's own statement of the model this phase adopts.

**Key insights:** (minimal context to resume after compaction)
- The central research finding is that Ze has **one** reachable elicitation call site (`tools.go:747`) and it suspends **one string that is already a tool argument** (`tools.go:766`). There is no continuation state, so there is nothing for `requestState` to carry.
- Consequently this phase emits `inputRequests` alone and **omits `requestState` entirely**. That is conformant by MRTR server requirement 6, and it deletes the whole HMAC/AEAD, principal-binding, TTL and request-digest design that the skeleton budgeted. Umbrella R-3 (signed-but-unbound state) cannot occur, because no state exists to sign.
- Two genuine requirements were discovered during design and are new work, not carried over: the elicitation capability is now **mode-structured** (the gate is form-mode support, not key presence), and the emitted request carries an explicit `mode` field.
- `TaskElicit` (`tasks.go:464`) is exported dead code with no production caller. It is deleted, and with it `task_elicit_test.go` (185 lines) which is the only thing that exercises it.
- The MCP functional tests live in `test/plugin/`, not `test/mcp/`. `test/mcp/` does not exist. The three existing elicitation tests are `test/plugin/elicitation-accept.ci`, `elicitation-decline.ci` and `elicitation-no-capability.ci`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [x] `internal/component/mcp/elicit.go` (302L) - flat-primitive schema validator (`:102-167`), four typed sentinels (`:28-56`), `session.Elicit` blocking model (`:227-270`), JSON-RPC request frame builder (`:291-302`).
- [x] `internal/component/mcp/session.go` (570L) - `clientElicit` capability bit (`:55`), `correlations` map (`:61`), `maxPendingElicits = 32` (`:108`), `ClientSupportsElicit` (`:356`), `RegisterElicit` cap check (`:384`), `ResolveElicit` (`:403`).
- [x] `internal/component/mcp/reply_sink.go` - the `replySink` interface and the POST-to-SSE upgrade the elicit frame rides.
- [x] `internal/component/mcp/tools.go` (855L) - the single elicit call site (`:747`) inside `ze_execute` (`:722`); capability pre-check (`:737`); elicited value assigned to `input.Command` (`:766`); dispatch (`:768`); `maxRequestBody = 1 << 20` (`:673`); tool description advertising elicitation (`:836`).
- [x] `internal/component/mcp/tasks.go` (530L) - `TaskElicit` (`:464`), a second elicitation producer with no production caller.
- [x] `internal/component/mcp/streamable.go` (808L) - `parseElicitationCapability` bare-presence check (`:720-736`), consumed at `doInitialize` (`:705`); client-response intake (`:544-605`, resolve at `:603`); body cap applied (`:154`, `:412`).
- [x] `internal/component/mcp/elicit_test.go` (676L, 60 assertions, 22 test functions) - 11 schema-validator tests (`:99-317`) that survive; the rest exercise the blocking model.
- [x] `internal/component/mcp/task_elicit_test.go` (185L, 17 assertions, 3 functions) - exercises unwired `TaskElicit` only.
- [x] `test/plugin/elicitation-accept.ci` (57L), `elicitation-decline.ci` (58L), `elicitation-no-capability.ci` (54L) - the real functional coverage.

**Behavior to preserve:**
- The flat-primitive requested-schema validation subset (`elicit.go:102-167`) and its `ErrElicitSchemaInvalid` path, unchanged.
- The `ErrElicitDeclined` and `ErrElicitCanceled` outcome distinction, retargeted from a blocking return value to an `inputResponses` `action` value.
- The user-visible experience: `ze_execute` invoked without a `command` still prompts for one rather than erroring, when the client supports form-mode elicitation.
- The fail-fast path when the client declares no capability (`tools.go:737-739`): an error result naming the missing argument, not a hang.

**Behavior to change:**
- Everything in the Delete and Add tables above.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- HTTP POST `tools/call` (or `resources/read`) whose `params` may carry `inputResponses` from a prior `InputRequiredResult`, and whose `_meta` carries `io.modelcontextprotocol/clientCapabilities`.

### Transformation Path
1. Header and version validation, per-request auth (Phase 1).
2. If `requestState` is present anywhere in the request: reject. Ze never mints one, so the value cannot be verified and the client violated MRTR client requirement 2.
3. Parse form-mode elicitation support from per-request `_meta` `clientCapabilities`.
4. Dispatch the tool handler with arguments plus any `inputResponses`.
5. The handler resolves its effective `command` from, in order: the `command` argument; then the `inputResponses` entry under Ze's fixed key. Absent both, and with form-mode support declared, it returns `resultType: "input_required"` carrying one `elicitation/create` entry. Absent both and without form-mode support, it returns the existing missing-argument error result.
6. An `inputResponses` entry with `action: "decline"` or `"cancel"` is a terminal answer: the handler completes with an error result and does not re-ask.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server to client across a retry | **Nothing crosses.** No continuation state is minted, carried, or echoed. The retry is self-contained and independently authenticated | No |
| Tool handler to transport | Handler returns an input-required outcome instead of blocking a goroutine | No |
| Client `_meta` to capability gate | Per-request `clientCapabilities.elicitation`, read for form-mode support rather than key presence | No |

### Integration Points
- `internal/component/mcp/tools.go` - the single elicit call site becomes re-entrant
- `internal/component/mcp/streamable_tools.go` - `inputResponses` intake and the `requestState` rejection guard
- `internal/test/cli/cmd_mcp.go` - the client half of the retry loop
- Phase 3 reuses `InputRequests` / `InputResponses` for tasks in `input_required`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | To verify at implementation: the capability gate is read once from per-request `_meta` and passed to the handler, never re-parsed inside `tools.go` |
| No unintended coupling | No | To verify at implementation: `mrtr.go` holds the result types and depends on no session type, so Phase 3 can reuse it for tasks |
| No duplicated functionality | No | To verify at implementation: the preserved validator (`elicit.go:102-167`) is called, not reimplemented, by the `ElicitRequest` builder |
| Zero-copy preserved where applicable | No | Not applicable: MCP bodies are JSON decoded into generic maps at a cold control-plane boundary, not a wire hot path |
| Registration over hardcoding | No | To verify at implementation: no per-tool `switch` is added to shared transport code; the input-required outcome is a return value from the existing `toolHandlers` map (`tools.go:721`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every current elicitation call site is expressible as a pure function of (arguments plus `inputResponses`) | Umbrella A-5 | The stateless model does not hold for Ze's handlers and the design changes | Done: `grep '\.Elicit(' internal/ cmd/` excluding tests returns exactly one hit, `tools.go:747`. Its suspended state is one string assigned to a tool argument at `tools.go:766`. The only other producer, `TaskElicit` (`tasks.go:464`), has no production caller and is deleted | confirmed |
| A-2 | Ze has a suitable keyed-MAC primitive available without a new dependency | `crypto/hmac` is stdlib | A dependency question reaches the user (`ai/rules/go-standards.md` bans new third-party imports without asking) | Done: the MCP crypto surface is already pure stdlib (`crypto/subtle` and `crypto/sha256` at `bearer.go:22-23`; `crypto/sha256`, `crypto/sha512`, `crypto/rsa`, `crypto/ecdsa` at `jwt.go:18-21`; `crypto/rand` at `session.go:14`; `crypto/ecdsa`, `crypto/elliptic`, `crypto/rsa` at `jwks.go:19-21`). **Moot for this phase** (no `requestState` is minted); retained because R-1's trigger re-opens it | confirmed |
| A-3 | No Ze elicitation flow needs at-most-once `requestState` semantics | No current elicit flow performs a redemption or irreversible one-shot action | A server-side consumed-state store is needed, reintroducing state | Done with A-1: the single flow (`tools.go:722-773`) prompts for a command string and dispatches it. **Structurally moot**: with no `requestState` minted there is no token to replay, so the at-most-once question cannot arise | confirmed |
| A-4 | The `2026-07-28` elicitation capability is mode-structured, so the correct gate is form-mode support rather than key presence | Elicitation page: the capability carries `form` and `url` sub-keys, an empty object "is equivalent to declaring support for `form` mode only", and "Servers **MUST NOT** send elicitation requests with modes that are not supported by the client" | Ze would emit a form-mode request to a url-only client, violating a MUST | Done: specification text read in full 2026-07-28. Ze's current parser (`streamable.go:720-736`) checks key presence only, so this is new work | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A **future** elicitation flow needs real continuation state, and `requestState` is added without the integrity design this phase deliberately did not build. The result is umbrella R-3 arriving later, unreviewed, in a phase that budgeted nothing for it | Any of these three triggers: (a) a second elicitation call site appears whose suspended state is not already a tool argument; (b) url-mode elicitation is implemented, whose whole point is out-of-band completion the server must correlate on retry; (c) `requestState` appears in a diff at all | The obligation is recorded in Known Limitations and in the `mrtr.go` guard's comment: **the day `requestState` is minted, MRTR server requirement 5 applies in full** (principal, TTL, originating-request digest, each verified, each with a negative test), and A-2 above records that the stdlib primitives are already present. The unsolicited-`requestState` rejection guard (AC-4) is the tripwire: it cannot be relaxed into an accept path without a deliberate edit that review will see |
| R-2 | Elicitation coverage silently thins (umbrella R-2) | The replacement suite is smaller than what it replaces, or passes with the producer disabled | The raw "861 lines deleted" framing is wrong and hid what matters. Measured: `elicit_test.go` is 676L / 60 assertions / 22 functions, of which **11 functions (`:99-317`) test the preserved validator and survive unmodified**; `task_elicit_test.go` is 185L / 17 assertions and tests **unwired** code, so deleting it loses no reachable coverage. The real exposure is the roughly 8 functions that prove the elicitation *decision* (when to prompt, what the prompt contains, decline and cancel outcomes). Each needs an MRTR equivalent, and the mutation check must disable the `InputRequiredResult` producer and confirm the `.ci` tests in `test/plugin/` go red. Record assertion counts before and after |
| R-3 | Key management for `requestState` (ephemeral versus persisted) breaks retries across a daemon restart | A retry after restart fails verification | **Void under Key Design Decision 1**: no key exists, because no state is signed. Retries survive restart trivially, since the retry is a self-contained authenticated request. Re-arms only if R-1 triggers, at which point the zefs-versus-ephemeral question (`ai/rules/zefs-persistence.md`) must be answered before any key is minted |
| R-4 | The re-ask loop becomes unbounded: a client that never supplies the input is prompted forever | A `.ci` test or a client hangs in a prompt cycle | Accepted, and harmless by construction. MRTR server requirement 8 explicitly permits repeated `InputRequiredResult` responses. Each round trip is a fresh, independently authenticated request holding **no** server-side state, so an unanswered prompt costs the server nothing between rounds. This is the strongest practical argument for the stateless design and is recorded as a Design Insight |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Interactive tool calls fail, or `ze_execute` stops prompting for a missing command. The cross-principal replay exposure that the skeleton design carried does not exist here, because no state is minted (Key Design Decision 1) |
| How is it reverted? | Single revert of the phase. Phase 1 leaves elicitation unreachable (it deletes the session type that owns `Elicit`), so reverting returns to that state, not to a working `2025-06-18` elicitation |
| Who else touches this path? | Phase 3 reuses `InputRequests` / `InputResponses` for tasks in `input_required`. `internal/test/cli/cmd_mcp.go` is the only in-tree client |

## Wiring Test (MANDATORY -- NOT deferrable)

Tests live in `test/plugin/`, which is where the existing MCP elicitation `.ci`
tests are (`test/plugin/elicitation-accept.ci` and siblings). `test/mcp/` does
not exist.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Client POSTs `tools/call` for `ze_execute` omitting `command`, then retries with `inputResponses` | → | handler → `InputRequiredResult` (no `requestState`) → retry → merged argument → dispatch | `test/plugin/mcp-mrtr-elicit-roundtrip.ci` |
| Client POSTs a retry carrying an unsolicited `requestState` | → | `requestState` rejection guard in `streamable_tools.go` | `test/plugin/mcp-mrtr-unsolicited-state-rejected.ci` |
| Client declaring no elicitation capability calls `ze_execute` without `command` | → | form-mode capability precondition | `test/plugin/mcp-mrtr-no-capability.ci` |
| Client declaring `elicitation: {"url":{}}` only calls `ze_execute` without `command` | → | form-mode capability precondition (A-4) | `test/plugin/mcp-mrtr-url-only-capability.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `tools/call` for `ze_execute` with empty `command`, client declaring form-mode elicitation | Result carries `resultType: "input_required"` and one `inputRequests` entry whose value is an `elicitation/create` request with `mode: "form"`, and **no `requestState` field is present** |
| AC-2 | Retry carrying `inputResponses` with `action: "accept"` under Ze's key | Final result, `resultType: "complete"`, carrying the dispatched command output |
| AC-3 | Any elicitation flow | No JSON-RPC request frame is ever written to the client, and no client JSON-RPC response is ever accepted. **Evidence completed 2026-07-30:** the second half had no test -- the five `TestStreamable_JSONRPCResponse*` tests that drove the old intake path went with `reply_sink_test.go` in Phase 1, and the refusal survived only as arithmetic (a response has no `method`, and `Mcp-Method` must repeat the body's method). It is now explicit at the producer (`errBodyCarriesNoMethod`, `internal/component/mcp/headers.go`, still `-32020`/400) and asserted by `TestClientJSONRPCResponseIsRefused` (`internal/component/mcp/headers_test.go`, 5 frame shapes x 4 header shapes) and `test/plugin/mcp-client-response-frame-rejected.ci`. The first half's `.ci` needle was vacuous and is corrected in `mcp-mrtr-elicit-roundtrip.ci` |
| AC-4 | Any request carrying a `requestState` field | Rejected with a JSON-RPC error; the tool does not run. The rejection message names the failure class and does not echo the supplied bytes |
| AC-5 | Client declares no `elicitation` capability in per-request `_meta` | No `inputRequests` entry is emitted; the existing missing-argument error result is returned |
| AC-6 | Client declares `elicitation` with `url` mode only | No form-mode entry is emitted (MRTR requirement 7 and the elicitation page's mode MUST NOT); the missing-argument error result is returned |
| AC-7 | Retry omitting the requested `inputResponses` key | A **new** `InputRequiredResult` asking again, not an error |
| AC-8 | Retry whose `inputResponses` entry carries `action: "decline"` or `"cancel"` | Terminal error result naming the outcome. No re-ask, so the loop cannot be driven by a declining client |
| AC-9 | Retry carrying unrecognised `inputResponses` keys alongside the recognised one | Extras ignored, request proceeds |
| AC-10 | `InputRequiredResult` emitted on a method other than `tools/call` or `resources/read` | Never happens (table test over every method `runMethod` dispatches) |
| AC-11 | Every `inputRequests` entry Ze can emit | Requests no credential-shaped field, satisfying "Servers **MUST NOT** use form mode elicitation to request sensitive information" (table test over the emittable set, which has one member) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Invokes `ze_execute` without a command, supplies one when prompted, gets the result | `tools/call` → `InputRequiredResult` → client retry with `inputResponses` → dispatch → final result | `test/plugin/mcp-mrtr-elicit-roundtrip.ci` |
| 2 | Declines the prompt | `tools/call` → `InputRequiredResult` → retry with `action: "decline"` → terminal error result, no re-ask | `test/plugin/mcp-mrtr-elicit-declined.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInputRequiredResultShape` | `internal/component/mcp/mrtr_test.go` | AC-1: `resultType`, one `inputRequests` entry, `mode: "form"`, and the absence of a `requestState` key | |
| `TestInputRequiredResultOmitsRequestState` | `internal/component/mcp/mrtr_test.go` | AC-1 negative: marshalled JSON contains no `requestState` key at all, not merely an empty one | |
| `TestUnsolicitedRequestStateRejected` | `internal/component/mcp/mrtr_test.go` | AC-4, including that the error text does not contain the supplied value | |
| `TestElicitCapabilityFormMode` | `internal/component/mcp/mrtr_test.go` | AC-5 and AC-6: absent, empty object (form), `{"form":{}}`, `{"url":{}}`, `{"form":{},"url":{}}` | |
| `TestResolveCommandFromInputResponses` | `internal/component/mcp/mrtr_test.go` | AC-2, AC-7, AC-8, AC-9: argument present; key present accept; key absent; decline; cancel; extra keys | |
| `TestInputRequiredOnlyOnSupportedMethods` | `internal/component/mcp/mrtr_test.go` | AC-10 | |
| `TestEmittableElicitationsRequestNoSecrets` | `internal/component/mcp/mrtr_test.go` | AC-11 | |
| `TestElicit_Schema*` (11 existing functions) | `internal/component/mcp/elicit_test.go:99-317` | Preserved unmodified; the validator does not change | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `inputResponses` map entries on one retry (client-supplied, so attacker-controlled) | 0 to whatever fits the 1 MB body cap (`maxRequestBody`, `internal/component/mcp/tools.go:673`, applied at `streamable.go:154` and `:412`) | the largest map whose encoded request stays at or under 1 MB | N/A: 0 entries is valid and means "re-ask" (AC-7) | the first map that pushes the body past 1 MB, refused by `http.MaxBytesReader` (`streamable.go:412`) before any handler runs |
| `inputRequests` entries emitted per `InputRequiredResult` (server-produced) | exactly 1 | 1 | 0 would violate MRTR requirement 6, since Ze emits no `requestState` to satisfy the at-least-one-of rule | 2 is unreachable: `ze_execute` asks for one value. Asserted by `TestInputRequiredResultShape` |
| Elicited command string length | 1 byte to the body cap above | a command whose retry body stays at or under 1 MB | empty string, which is treated as "not supplied" and re-asks (AC-7), matching today's `tools.go:763-765` | refused by the body cap, not by a per-field limit |

**No new cardinality bound is added, and this is a deliberate decision.** Today
`maxPendingElicits = 32` (`internal/component/mcp/session.go:108`) bounds the
`correlations` map, which exists because the server holds one entry per
*outstanding* elicitation while a handler goroutine blocks. Under MRTR nothing
outstanding is held: the `InputRequiredResult` is emitted and the request
terminates, so there is no map to grow and no goroutine to pin. The cap is
deleted with the map rather than reimplemented. The only attacker-controlled
quantity that remains is `inputResponses`, which is inbound request body and is
already bounded by `maxRequestBody` (1 MB). Adding a second, narrower cap on a
map that is parsed once and discarded would be YAGNI
(`ai/rules/design-principles.md`).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-mrtr-elicit-roundtrip` | `test/plugin/*.ci` | Prompted for a missing command and get the result. Replaces `elicitation-accept.ci` | |
| `mcp-client-response-frame-rejected` | `test/plugin/*.ci` | A client JSON-RPC response POSTed to `/mcp` is refused with `-32020`/400 and never dispatched (AC-3, second half) | done 2026-07-30 (written late; see the AC-3 row) |
| `mcp-mrtr-elicit-declined` | `test/plugin/*.ci` | Decline the prompt cleanly, no re-ask. Replaces `elicitation-decline.ci` | |
| `mcp-mrtr-unsolicited-state-rejected` | `test/plugin/*.ci` | A `requestState` Ze never issued is refused | |
| `mcp-mrtr-no-capability` | `test/plugin/*.ci` | Client without elicitation is never prompted. Replaces `elicitation-no-capability.ci` | |
| `mcp-mrtr-url-only-capability` | `test/plugin/*.ci` | Client supporting only url mode is never sent a form request | |
| `mcp-mrtr-reask` | `test/plugin/*.ci` | A retry that omits the answer is asked again rather than erroring | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No third-party MCP peer in-tree (umbrella Key Design Decisions). The substitute obligation stands: `internal/test/cli/cmd_mcp.go` implements the retry loop from the MRTR page text, not from Ze's server code | |

## Files to Modify
- `internal/component/mcp/tools.go` - `ze_execute` re-entrancy at `:722-773`; tool description at `:836` no longer describes a server-initiated frame
- `internal/component/mcp/streamable_tools.go` - `inputResponses` intake and the unsolicited-`requestState` guard
- `internal/component/mcp/streamable.go` - `parseElicitationCapability` (`:720-736`) becomes a form-mode check reading per-request `_meta`
- `internal/component/mcp/elicit.go` - reduced to the preserved validator, the sentinels, and `ElicitRequest` construction with `mode`
- `internal/component/mcp/tasks.go` - delete `TaskElicit` (`:456-513`)
- `internal/component/mcp/elicit_test.go` - keep `:99-317` unmodified, retarget the rest
- `internal/test/cli/cmd_mcp.go` - client retry loop
- `docs/guide/mcp/elicitation.md` (**CREATE, not rewrite** -- corrected 2026-07-29: Phase 1 DELETED this page rather than stubbing it, because its two `<!-- source: -->` anchors pointed at `elicit.go` and `session.go` and `code_to_docs.py --check` fails on an anchor to a deleted file. Phase 2 writes it fresh for the MRTR shape), `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md`

## Files to Create
- `internal/component/mcp/mrtr.go` - `InputRequiredResult`, `InputRequests`, `InputResponses`, the form-mode capability gate, and the unsolicited-`requestState` guard
- `internal/component/mcp/mrtr_test.go` - the unit tests above
- `test/plugin/mcp-mrtr-*.ci` - the functional suite above

## Files to Delete
- `internal/component/mcp/task_elicit_test.go` - tests `TaskElicit`, which has no production caller and is deleted with it
- `internal/component/mcp/reply_sink.go` (if Phase 1 has not already)
- Correlation-map members of `session.go` (if any survive Phase 1)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No tunable is added. The `requestState` TTL that would have needed one does not exist (Key Design Decision 1), and no other constant in this phase is operator-relevant |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | Yes | `internal/test/cli/cmd_mcp.go`: the `-elicit` flag (`:33`) describes initialize-time capability declaration and becomes a per-request form-mode declaration; the client gains the retry loop |
| CLI grammar | N-A | No operator-facing command surface |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | Yes | `test/plugin/mcp-mrtr-*.ci` |
| Pipe completeness | N-A | JSON-RPC over HTTP, not a CLI display surface |
| Env var registration | No | No YANG leaf added, so no `env.MustRegister` entry |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or certificate |
| Prometheus counters/metrics | No | The one candidate was a rejected-`requestState` counter. `ai/rules/fail-closed-guards.md` requires the guard to "fail closed or say something": it does both, by returning a JSON-RPC error to the caller and emitting a WARN log. A counter for a condition that should never occur, and that is already visible in the response and the log, is YAGNI |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (the elicitation statement changes from server-initiated to MRTR) |
| 2 | Config syntax changed? | No | No leaf added or removed by this phase; the session-lifetime leaves are Phase 1's |
| 3 | CLI command added/changed? | Yes | `docs/functional-tests.md` for the `ze-test mcp` `-elicit` flag and retry loop |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: `elicitation/create` stops being a server-to-client method and becomes an `inputRequests` value |
| 5 | Plugin added/changed? | No | MCP is a component |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/elicitation.md` -- **created fresh, not rewritten** (corrected 2026-07-29: Phase 1 deleted the page; see Required Reading). Document the MRTR round trip: `resultType: "input_required"` + `inputRequests`, the client retry carrying `inputResponses`, and the form-mode capability precondition. There is no SSE upgrade or correlated-response walkthrough to remove, because there is no page |
| 7 | Wire format changed? | Yes | `docs/architecture/mcp/overview.md` |
| 8 | Plugin SDK/protocol changed? | No | Untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/mcp-integration.md` records MRTR support and the deliberate absence of `requestState` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: the MCP elicitation row names the mechanism, which changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md` (the elicit correlation flow is removed from both) |
| 13 | Route metadata keys added/changed? | N-A | Not routing |
| 14 | Prometheus counters added/changed? | No | None added, per the Integration Checklist decision above |
| 15 | Registered plugin/event/command/capability changed? | No | No registry entries |
| 16 | Changed source referenced by doc source anchors? | Yes | `docs/architecture/mcp/overview.md` anchors `elicit.go`, `session.go` and `tasks.go`, all of which change |
| 17 | Docs show config/CLI/API examples for this area? | Yes | ~~`docs/guide/mcp/elicitation.md` examples are all the old server-initiated shape~~ **corrected 2026-07-29:** that page is gone, so there are no stale examples left to fix. The live surfaces carrying MCP examples are `docs/guide/mcp/overview.md`, `docs/features/mcp-integration.md` and `cmd/ze/help_ai.go`; a new `inputRequests` example is added to the first two |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - accept `inputResponses` in `tools/call` params and route them to a stub that always re-asks; add the unsolicited-`requestState` rejection guard. Failing wiring test first.
2. **Phase: Capability gate** - per-request form-mode parsing replacing `parseElicitationCapability` (`streamable.go:720-736`), TDD across the five capability shapes (A-4).
3. **Phase: `InputRequiredResult`** - types and emission in `mrtr.go`, gated on form-mode support, asserting no `requestState` key is marshalled.
4. **Phase: Handler re-entrancy** - rewrite `ze_execute` (`tools.go:722-773`) as a function of arguments plus `inputResponses`, with the decline and cancel terminal paths distinguished from the missing-key re-ask path.
5. **Phase: Delete the old path** - `session.Elicit`, correlation map, `handleElicitResponse`, `TaskElicit` and `task_elicit_test.go`.
6. **Phase: Consumers and docs** - test-client retry loop, a freshly created `docs/guide/mcp/elicitation.md` (Phase 1 deleted it), overview, digest. Re-add the inbound links Phase 1 removed from `docs/architecture/api/commands.md` and `docs/features/mcp-integration.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No `requestState` key is emitted anywhere, and the marshalled `InputRequiredResult` omits it rather than sending it empty or null |
| Correctness | The unsolicited-`requestState` guard rejects and cannot fall through to a "treat as absent" path |
| Correctness | The capability gate tests form-mode support, not key presence, so a url-only client is never sent a form request |
| Correctness | No JSON-RPC request frame is written to a client anywhere in the tree |
| Correctness | `InputRequiredResult` never emitted on a method the specification forbids it on |
| Data flow | The retry is processable with nothing beyond the retry request; no server-side continuation survives between the two |
| Rule: `ai/rules/fail-closed-guards.md` | An arriving `requestState` denies and says so; a missing `inputResponses` key is distinguishable from a declined one, and only the former re-asks |
| Rule: `ai/rules/no-test-deletion.md` | The 11 preserved validator tests (`elicit_test.go:99-317`) are unmodified; the decision-proving tests have MRTR equivalents; `task_elicit_test.go` is deleted only because its subject is unwired dead code |
| Rule: `ai/rules/wiring-completeness.md` | No exported symbol in `mrtr.go` lacks a non-test caller, which is the defect `TaskElicit` exhibited |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No server-initiated request frames | `grep -rn 'elicitation/create' --include=*.go internal/ \| grep -v _test.go` shows only `ElicitRequest` construction destined for `inputRequests`, never a frame write |
| No client-response intake | `grep -rn 'handleElicitResponse\|ResolveElicit\|maxPendingElicits' --include=*.go internal/` is empty |
| No `requestState` minted | `grep -rn 'requestState' --include=*.go internal/` matches only the rejection guard and its tests |
| Unwired producer gone | `grep -rn 'TaskElicit' --include=*.go internal/` is empty |
| Coverage restructured, not thinned | Assertion counts recorded before (60 in `elicit_test.go`, 17 in `task_elicit_test.go`) and after; mutation check green per R-2 |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| No state, therefore no replay primitive | Confirm by inspection that nothing server-side survives between the initial request and the retry, and that the retry derives identity from per-request authentication rather than from anything the client echoed |
| Unsolicited `requestState` | Rejected, not ignored. The guard denies before dispatch and its error text does not echo the supplied bytes (`ai/rules/error-messages.md`) |
| Capability gate | A url-only or absent capability cannot reach the form-mode emission path |
| Form-mode sensitive data | The single emittable elicitation asks for a `ze` CLI command, not a credential (AC-11) |
| Future-state tripwire | The R-1 obligation is recorded in the guard's own comment, so anyone minting a `requestState` meets MRTR requirement 5 in the same diff |
| Error leakage | Rejection messages name the failure class only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Wrong AC → DESIGN, correct AC → IMPLEMENT |
| A second elicitation call site is discovered that suspends non-argument state | STOP. A-1 is broken and Key Design Decision 1 must be re-taken. Escalate to Thomas |
| A spec MUST cannot be met as designed | STOP. Escalate per `ai/rules/rfc-compliance.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **The hard part evaporated when it was measured.** The umbrella called this the hardest phase and named a "control-flow inversion inside the tool handlers" (umbrella Design Insights). That reading was right about the shape and wrong about the scale: there is one handler, and the value it suspends is a tool argument it would have received anyway. The inversion is real but it is roughly "read the argument from a second place", not a continuation rewrite. Measuring the call sites before designing the mechanism is what shrank the phase.
- **Statelessness is what makes the re-ask loop safe.** MRTR server requirement 8 permits a server to return `InputRequiredResult` repeatedly. That would be an obvious denial-of-service concern for a design holding per-prompt state, which is exactly what `maxPendingElicits = 32` (`session.go:108`) exists to bound today. With no state held between rounds, an unanswered prompt costs the server nothing, so the cap disappears without needing a replacement (R-4).
- **The absence of `requestState` is itself the security property.** Umbrella R-3 feared state that is "signed but unbound", giving a cross-user replay primitive. The elicitation page's security consideration 1, that servers "**MUST** bind elicitation requests to the client and user identity", is satisfied structurally here rather than cryptographically: there is no carried authority to bind or mis-bind, and the retry is authorised on its own per-request credentials (Phase 1). A design with no token cannot leak one.
- **An unsolicited `requestState` is a client protocol violation, not merely unexpected input.** MRTR client requirement 2 says that when the result carries no `requestState`, "the client **MUST NOT** include one in the retry". Ze never emits one, so any arriving value came from a confused or hostile client. Rejecting is both conformant (requirement 4 tells servers to reject state that fails verification, and unverifiable state fails vacuously) and the fail-closed choice.
- **Two real requirements were hiding under the one that vanished.** The capability shape changed (mode sub-keys, A-4) and the request gained a `mode` parameter. Neither is in the umbrella's Task table, both are MUST-adjacent, and both would have been easy to miss while attention was on the crypto that turned out to be unnecessary.
- **A line count is not a coverage measure.** R-2 was framed as "861 lines of elicit tests deleted". Reading them shows 11 of 22 functions in `elicit_test.go` test the preserved validator and need no change at all, while all 185 lines of `task_elicit_test.go` exercise a symbol with no production caller. The genuine exposure is about 8 functions, and naming them is what makes the mutation check meaningful.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Emit `inputRequests` alone and **omit `requestState` entirely** | (a) Implement the full integrity-protected `requestState` anyway, for future-proofing: HMAC or AEAD over a payload carrying principal, TTL and originating-request digest. (b) Emit an unprotected `requestState` carrying the command | Conformant by MRTR server requirement 6, "Servers **MUST** include at least one of `inputRequests` or `requestState` in every `InputRequiredResult` response", which `inputRequests` satisfies on its own; requirement 3 makes `requestState` a MAY. Ze has nothing to put in it: the only elicitation suspends one string that is already a tool argument (`tools.go:747`, `:766`), and the elicitation page states outright that "Elicitations do not require that the server maintain state about users with the multi round-trip requests mechanism". Alternative (a) is rejected on YAGNI (`ai/rules/design-principles.md`): it is a key, a TTL, a digest scheme, four negative tests and a key-lifecycle question (old R-3) built for a caller that does not exist, and unused crypto is a liability rather than an asset. Alternative (b) is rejected outright: an unprotected `requestState` influencing dispatch is exactly umbrella R-3. **Re-opened by R-1's triggers**, and A-2 records that the stdlib primitives are already in the tree when that day comes |
| **Reject** an unsolicited `requestState` rather than ignoring it | Silently ignore it, since a field that influences nothing needs no integrity protection under requirement 4's conditional | Both are arguably conformant, but rejecting is the fail-closed choice (`ai/rules/fail-closed-guards.md`: "fail closed or say something"). Ze never mints state, so every arriving value is unverifiable and fails verification vacuously, which requirement 4 says to reject. Ignoring would also leave a silent accept path that a future `requestState` implementation could inherit without noticing; the rejection is the tripwire that forces R-1's obligation to be met deliberately |
| Emit `mode: "form"` explicitly on the `ElicitRequest` | Omit `mode`, which the specification permits: "For backwards compatibility, servers **MAY** omit the `mode` field for form mode elicitation requests" | Explicit over implicit (`ai/rules/design-principles.md`). Omission relies on every client applying the default correctly, and it makes the url-mode gap invisible at the call site. The cost is one map key |
| Gate on **form-mode support**, not on the presence of the `elicitation` key | Keep the existing bare-presence check (`streamable.go:720-736`), which is what `2025-06-18` needed | The elicitation page states "Servers **MUST NOT** send elicitation requests with modes that are not supported by the client", and the capability now carries `form` and `url` sub-keys. A client declaring `{"url":{}}` supports elicitation but not form mode, so presence is no longer the right question. An empty object still means form-only, so the common case is unchanged |
| Preserve the flat-primitive schema validator unchanged | Widen it to the `2026-07-28` subset, which now permits `oneOf`-titled enums and array multi-select enums | The validator constrains what **Ze emits**, and a server emitting a narrower subset than the specification permits is conformant: it simply does not use the optional richer forms. Ze's one schema is a single string property. Widening would be speculative work on a validator with no caller needing it, and it would put back the `oneOf` and array cases the current code deliberately rejects (`elicit.go:89-91`, `:142-144`). Recorded in Known Limitations |
| Distinguish a missing `inputResponses` key (re-ask) from `decline` or `cancel` (terminal) | Treat any absent value as a re-ask, including after a decline | The MRTR error-handling section says to re-ask when "the client fails to send all the information requested", which is omission, not refusal. Treating a decline as omission would loop a user who has explicitly said no. It also preserves today's outcome distinction (`ErrElicitDeclined`, `ErrElicitCanceled`, `elicit.go:274-285`) instead of collapsing it |
| Tests go in `test/plugin/`, not `test/mcp/` | Create `test/mcp/` as the umbrella and the skeleton both assumed | `test/mcp/` does not exist. The three existing MCP elicitation tests are `test/plugin/elicitation-accept.ci`, `elicitation-decline.ci` and `elicitation-no-capability.ci`, and 19 `.ci` files reference MCP across `test/plugin/`, `test/parse/`, `test/chaos-web/` and `test/ui/`. Following the existing placement keeps the replacements next to what they replace |
| Delete `TaskElicit` rather than porting it to MRTR | Port it, so tasks can elicit under the new pattern | It has no production caller (`tasks.go:464`; every other reference is in its own test file), so porting it would carry unwired code across a rewrite and grow Phase 2 for no user-visible behavior. Phase 3 owns tasks in `input_required` and will build that path against the `mrtr.go` types deliberately, per the umbrella phase table |

## Known Limitations

- **No `requestState` is minted or accepted.** This is conformant (MRTR requirement 6 is satisfied by `inputRequests`, and requirement 3 makes `requestState` a MAY) and is the right design for Ze's single stateless elicitation. The limitation it implies is that any future flow needing genuine continuation state cannot simply start emitting one: R-1 records the trigger conditions and the obligation, which is MRTR requirement 5 in full, principal plus TTL plus originating-request digest, each verified and each with a negative test.
- **URL-mode elicitation is not implemented.** Ze's one elicitation asks for a `ze` CLI command, which is not a credential, so form mode is correct and the "**MUST NOT** use form mode elicitation to request sensitive information" prohibition is satisfied rather than skirted. Url mode would also be the first flow to need real `requestState`, since its completion is out of band, so it is deliberately coupled to R-1's trigger (b).
- **The emitted requested-schema subset is narrower than `2026-07-28` permits.** The revision now allows `oneOf`-titled enums and array multi-select enums; Ze's validator rejects both (`elicit.go:89-91`, `:142-144`) and Ze emits neither. A server that uses fewer optional schema forms than the specification offers is conformant. Widening is available if a future elicitation needs a picker.
- **Sampling and Roots entries are never emitted.** `InputRequests` values may be `ElicitRequest`, `CreateMessageRequest` or `ListRootsRequest`; Ze emits only the first. Both others are Deprecated in this revision and Ze implements neither (umbrella Known Limitations).
- **`prompts/get` remains unimplemented**, so of the three methods permitted to carry an `InputRequiredResult`, Ze can produce one on two. Pre-existing and unrelated to this revision change.

## RFC Documentation (Scope: protocol)

Add `// MCP 2026-07-28 basic/patterns/mrtr Section X: "<quoted requirement>"`
above each enforcing path. At minimum:

| Enforcing path | Requirement to quote |
|----------------|----------------------|
| `InputRequiredResult` construction | Server requirement 6, the at-least-one-of rule, plus a note that `inputRequests` alone satisfies it |
| The unsolicited-`requestState` guard | Server requirement 4 (attacker-controlled input, reject what fails verification) and client requirement 2 (client MUST NOT include one when none was sent). The comment also carries R-1's obligation for whoever mints the first real state |
| The form-mode capability gate | Server requirement 7, plus the elicitation page's "Servers **MUST NOT** send elicitation requests with modes that are not supported by the client" |
| The method restriction | "Servers **MUST NOT** send `InputRequiredResult` responses on any other client requests" |
| The re-ask path | The error-handling clause on missing requested information |
| The unknown-key path | The error-handling clause on unexpected `InputResponses` parameters |
| The emitted `ElicitRequest` | The elicitation page's form-mode sensitive-information prohibition |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
