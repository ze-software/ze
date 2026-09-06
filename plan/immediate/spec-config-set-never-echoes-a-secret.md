# Spec: a command that acknowledges what the operator typed never echoes a secret

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | implemented in the working tree, UNCOMMITTED |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Backfilled. The work was commissioned straight from a journal row and
     skipped the spec step. Status is in-progress: the product code exists in
     the working tree and closure has not run. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze config set ... plaintext-password hunter2` wrote the operator's RAW value
back on stderr, in the success line and in the dry-run line. So did the SSH CLI's
`set` and `insert`, the web terminal's `set`, and the adoption prompt of
`ze config edit`. A commit conflict published the credential ANOTHER operator
typed.

Evidence is the 2026-09-06 row in `plan/journal/secret-echoed-to-the-client.md`,
the ninth producer in a class that was reopened four times on 2026-08-15. It is
not restated here. What that row establishes and this spec turns into criteria:
every mask the class built cleans a TREE, a DIFF or an ERROR MESSAGE, and this is
none of those. It is the command's own acknowledgement of what the operator
typed, written before any tree exists to mask.

Goal: a command that echoes a value the operator typed asks the SCHEMA whether
that value is a secret, and one predicate answers for every such command.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - the mask family and its predicate
  → Constraint: `config.LeafHoldsSecret` answers on `Sensitive` or `Bcrypt`, and
    `ze:ephemeral` is deliberately NOT in it: it says whether a value is
    persisted, which is a different question.
- [ ] `docs/architecture/config/syntax.md` - the `set` command grammar
  → Constraint: the operator types a TOKEN path, which steps over a list key, so
    the mask needs a schema-only walk rather than a tree lookup.
- [ ] `ai/rules/evidence.md` - guards fail closed
  → Decision: an unresolved path and a nil schema both answer the placeholder.
    `Editor.DisplayTreeAtPath` failed OPEN here once, and the row records it.

**Key insights:**
- Every fix in this class needs the OPPOSITE polarity beside it. A mask that
  fails closed on an unresolved path would hide EVERY value and satisfy the
  "never echoes a secret" assertion on its own.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/cli/cmd_set.go` - `cmdSetImpl`, the success line, the dry-run line and the refusal
- [ ] `internal/component/config/mask.go` - `secretAtPath`, `DisplayValueAtPath`, `DisplayMessageAtPath`, `MaskSecretInMessage`, `LeafHoldsSecret`
- [ ] `internal/component/config/schema.go` - `LookupTokenPath`, the schema-only walk over an operator token path
- [ ] `internal/component/cli/model_commands_edit.go` - `Model.cmdSet` and `Model.cmdInsert`, their status lines and refusals
- [ ] `internal/component/cli/editor_commit.go`, `internal/component/cli/editor_draft.go` - the four `Conflict` constructions
- [ ] `internal/component/web/cli_terminal.go` - `executeTerminalSet`, which answered the value into the response body
- [ ] `internal/component/config/cli/cmd_edit.go` - the adoption prompt, through `PendingChange.Summary`
- [ ] `internal/component/web/secret.go` - `maskSecretInMessage`, now delegating
- [ ] `internal/component/authz/yang/ze-authz-conf.yang` - the `plaintext-password` leaf
- [ ] `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang` - the other `plaintext-password` leaf

**Behavior to preserve:**
- A value the schema does NOT mark is still echoed in full. An operator setting a
  hold-time must still read it back.
- The PATH stays in the clear in a conflict report, so the report still says
  which leaf is contested.
- `config.MaskBcrypt` stays narrow, because `ze config dump` calls it before it
  writes `$9$`.

**Behavior to change:**
- `config.DisplayValueAtPath` answers the text a command may ECHO.
  `config.DisplayMessageAtPath` answers the sentence that REFUSES it.
- Both read `secretAtPath`, which reads `LeafHoldsSecret` through
  `Schema.LookupTokenPath`. Both FAIL CLOSED.
- `config.MaskSecretInMessage` is the web's `maskSecretInMessage` body promoted
  whole, and the web function delegates, so the `%q` escape rule this class
  learned on 2026-08-15 has ONE home.
- `Conflict.MyValue`, `OtherValue` and `PreviousValue` are masked at their four
  constructions, which closes all six renderers.
- Both `plaintext-password` leaves carry `ze:sensitive` beside `ze:ephemeral`.

## Data Flow (MANDATORY)

### Entry Point
- `ze config set <token path> <value>` on the command line.
- `set <token path> <value>` typed in the SSH CLI's config mode.
- `set ...` typed in the web CLI terminal.
- Entry format in every case is a token path plus a raw operator value.

### Transformation Path
1. The command parses the token path and the value.
2. `Schema.LookupTokenPath` resolves the token path to a schema node, stepping
   over a list key.
3. `secretAtPath` asks `LeafHoldsSecret` whether that node is `Sensitive` or
   `Bcrypt`, and answers true for a nil schema or an unresolved path.
4. `DisplayValueAtPath` answers the placeholder or the value;
   `DisplayMessageAtPath` answers the refusal sentence with the value removed,
   including its `%q`-escaped spelling.
5. The command writes the answer to stderr, to the model status line, or into the
   web response body.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| operator ↔ stderr | the acknowledgement line | Yes, `TestConfigSetNeverEchoesASecret` |
| operator ↔ SSH session | the model status line | Yes, `TestSSHCLISetNeverEchoesASecret` |
| operator ↔ web response body | `executeTerminalSet` | Yes, `TestWebTerminalSetNeverEchoesASecret` |
| operator ↔ another operator | the commit conflict report | Yes, `TestCommitConflictNeverEchoesASecret` |
| CLI ↔ audit sink and log | `Model.recordConfigCommit` takes the masked `Editor.Diff()`; `cmdSetImpl` records no audit entry | Yes, read at the producer |

### Integration Points
- `internal/component/config/mask.go` - the one home for the display predicate.
- `internal/component/web/secret.go` - delegates rather than holding a second body.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | all eight sinks route through the two new functions |
| No unintended coupling | Yes | `web` and `cli` both call `config`, and neither calls the other |
| No duplicated functionality | Yes | the web's message mask is PROMOTED, not copied (`ai/rules/no-layering.md`) |
| Zero-copy preserved where applicable | N-A | display path |
| Registration over hardcoding | No | the eight sinks are found by reading the code, not by a registry. A ninth sink is a ninth thing to remember, which is what this class has done nine times |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every write-only password leaf is marked `ze:sensitive` | `TestAWriteOnlyPasswordLeafIsMarkedSensitive` walks every YANG module | the mask hides everything except the password that prompted the row | that test | confirmed: both `plaintext-password` leaves now carry the marking |
| A-2 | The eight sinks are the whole population | read at each producer during the fix | a ninth sink still echoes | grep plus the review; a NINTH was found during the work and folded in | confirmed for the eight plus the conflict |
| A-3 | `LookupTokenPath` resolves the same path the command acts on | it is the schema-only sibling of the walk `set` uses | the mask answers about a different leaf | asserted from the code; no test drives a divergence | UNVALIDATED |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A mask that fails closed hides every value | the operator cannot read back a hold-time | each fix carries the OPPOSITE polarity test, `...StillEchoesAValueTheSchemaDoesNotMark` |
| R-2 | A tenth sink exists | this class has been reopened four times already | the population is enumerated in the journal row and in the AC table |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | an operator's credential reaches a terminal, a scrollback, a web response body, or another operator's conflict report |
| How is it reverted? | not yet landed. Once landed, a single commit revert |
| Who else touches this path? | the web mask family, and any future command that acknowledges a typed value |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config set ... plaintext-password <v>` | → | `cmdSetImpl` through `config.DisplayValueAtPath` | `TestConfigSetNeverEchoesASecret` (`internal/component/config/cli/cmd_set_secret_test.go`) |
| `set ...` in the SSH CLI config mode | → | `Model.cmdSet` through `config.DisplayValueAtPath` | `TestSSHCLISetNeverEchoesASecret` (`internal/component/cli/model_commands_edit_secret_test.go`) |
| `set ...` in the web CLI terminal | → | `executeTerminalSet` | `TestWebTerminalSetNeverEchoesASecret` (`internal/component/web/cli_terminal_secret_test.go`) |
| a commit conflict on a secret leaf | → | the four `Conflict` constructions | `TestCommitConflictNeverEchoesASecret` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config set` on a `ze:sensitive` leaf | the success line and the dry-run line carry the placeholder, never the value or a distinctive tail of it |
| AC-2 | the same set is REFUSED | the refusal sentence carries neither the raw value nor its `%q`-escaped spelling |
| AC-3 | `ze config set` on a leaf the schema does NOT mark | the value is echoed in full |
| AC-4 | `set` and `insert` in the SSH CLI config mode | AC-1 and AC-3 hold on the status line and the refusal |
| AC-5 | `set` in the web CLI terminal | AC-1 and AC-3 hold in the response body |
| AC-6 | `ze config edit` adoption prompt | `PendingChange.Summary` requires a schema, so no unmasked spelling of it exists |
| AC-7 | a commit conflict over a secret leaf | all six renderers carry the placeholder for all three values, and the PATH stays in the clear |
| AC-8 | a nil schema, or a token path the schema does not resolve | the placeholder is answered, never the value |
| AC-9 | every YANG module in the tree | no `plaintext-` leaf is unmarked |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigSetNeverEchoesASecret`, `TestConfigSetRefusalNeverEchoesASecret`, `TestConfigSetStillEchoesAValueTheSchemaDoesNotMark` | `internal/component/config/cli/cmd_set_secret_test.go` | AC-1, AC-2, AC-3 | written, uncommitted, each observed red against the unfixed producer |
| `TestSSHCLISetNeverEchoesASecret`, `TestSSHCLISetStillEchoesAValueTheSchemaDoesNotMark`, `TestCommitConflictNeverEchoesASecret`, `TestPendingChangeSummaryNeverEchoesASecret` | `internal/component/cli/model_commands_edit_secret_test.go` | AC-4, AC-6, AC-7 | written, uncommitted, each observed red |
| `TestWebTerminalSetNeverEchoesASecret`, `TestWebTerminalSetStillEchoesAValueTheSchemaDoesNotMark` | `internal/component/web/cli_terminal_secret_test.go` | AC-5 | written, uncommitted, observed red |
| `TestAWriteOnlyPasswordLeafIsMarkedSensitive` | existing | AC-9, A-1 | passes |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| token path depth | 1..n | n | 0, which resolves nothing and therefore masks | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| a `.ci` driving `ze config set` on a marked leaf and reading stderr | not written | the operator sets a password and reads the acknowledgement | MISSING. See "What Remains" |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | CLI display, no wire-visible behavior | N-A |

## Files to Modify
- `internal/component/config/mask.go` - the two new display functions and `secretAtPath`
- `internal/component/config/schema.go` - `LookupTokenPath`
- `internal/component/config/cli/cmd_set.go` - the three sinks in `cmdSetImpl`
- `internal/component/config/cli/cmd_edit.go` - the adoption prompt
- `internal/component/cli/model_commands_edit.go` - `cmdSet` and `cmdInsert`
- `internal/component/cli/editor_commit.go`, `editor_draft.go` - the four `Conflict` constructions
- `internal/component/web/cli_terminal.go` - `executeTerminalSet`
- `internal/component/web/secret.go` - delegate to `config.MaskSecretInMessage`
- `internal/component/authz/yang/ze-authz-conf.yang`, `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang` - mark the two leaves

## Files to Create
- `internal/component/config/cli/cmd_set_secret_test.go`
- `internal/component/cli/model_commands_edit_secret_test.go`
- `internal/component/web/cli_terminal_secret_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | both `plaintext-password` leaves gained `ze:sensitive` beside `ze:ephemeral` |
| YANG validation constraints | No | no constraint changed |
| CLI commands/flags | No | `ze config set` keeps its grammar; only its output changed |
| Editor autocomplete | No | unchanged |
| Functional test for new RPC/API | Yes, MISSING | no `.ci` drives the echo path |
| Pipe completeness | No | the acknowledgement is stderr, outside the pipe surface |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | No | the grammar is unchanged; only the acknowledgement is |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, where `ze config set` output is shown |
| 16 | Any changed source file referenced by existing doc anchors? | Yes | `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` is the `// Design:` anchor of `mask.go` and MUST name the two new display functions. `docs/architecture/config/syntax.md` anchors `schema.go`, `cmd_set.go` and `cmd_edit.go`. `docs/architecture/config/yang-config-design.md` anchors `model_commands_edit.go`, `editor_commit.go` and `editor_draft.go`. `docs/architecture/web-interface.md` anchors `cli_terminal.go` and `secret.go` |
| 1, 4-15, 17 | - | No | no other operator-facing surface changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - `Schema.LookupTokenPath` and
   `secretAtPath`, with a failing test that an unresolved path masks.
2. **Phase: The pair** - `DisplayValueAtPath` and `DisplayMessageAtPath`, and
   promote the web's message mask into `config.MaskSecretInMessage`.
3. **Phase: The eight sinks** - route each one through the pair. Each gets BOTH
   polarities: never-echoes, and still-echoes-an-unmarked-value.
4. **Phase: The conflict** - mask at the four constructions, keeping the path clear.
5. **Phase: Functional test** - a `.ci` over `ze config set`.
6. **Phase: Documentation** - the anchor pages named above.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every one of the eight sinks, plus the conflict, has a test |
| Correctness | each test asserts on a distinctive TAIL as well as the whole value, because the whole is absent from an escaped leak too |
| Feature completeness | the opposite polarity exists for every sink, so a fail-closed mask cannot pass alone |
| Data flow | one predicate, read from the schema; no second spelling of "is this a secret" |
| Rule: `ai/rules/evidence.md` | the guard fails closed, and the zero value is not a valid-looking answer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no raw value in an acknowledgement | `go test ./internal/component/config/cli/ ./internal/component/cli/ ./internal/component/web/ -run Secret` |
| both password leaves are marked | `go test ./... -run TestAWriteOnlyPasswordLeafIsMarkedSensitive` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | the refusal is written with `%q`, so the mask must remove the escaped spelling as well as the raw one |
| Authorization failing open | a nil schema or an unresolved path must mask, never publish |
| Log and audit sinks | read at the producer: `cmdSetImpl` records no audit entry, `Model.recordConfigCommit` takes the masked diff, and the SSH exec log is redacted by `loggedCommand` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A never-echoes test passes but a still-echoes test fails | the mask is too wide; it is hiding unmarked values |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- A mask must know the FORMAT VERB that wrote the message it cleans, not only the
  value. That is the 2026-08-15 lesson, and promoting the web's body is what
  gives it one home.
- Nine producers over three passes is what a per-sink fix costs. The population
  is found by reading code, and the tenth sink will be found the same way.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A new pair for the ECHO shape | extend `config.MaskSecrets` | every existing mask cleans a tree, a diff or an error message; the acknowledgement is written before a tree exists |
| Fail closed on an unresolved path | fail open, as `Editor.DisplayTreeAtPath` did | the journal row records the fail-open as its own defect |
| Mask the conflict at its four constructions | mask at the six renderers | four sites close six, and a seventh renderer inherits the fix |

## Known Limitations
- `cmdInsert` has no red-provable case: `ze:sensitive` and `ze:bcrypt` are fields
  of `LeafNode` alone, so no leaf-list can be marked and its secret polarity is
  unreachable by construction.
- One sink stays OPEN and is a row of its own in
  `plan/journal/secret-echoed-to-the-client.md`: `TranscriptWriter.Record`
  (`internal/component/cli/transcript.go`) writes the operator's raw command line
  to the session transcript. `redact.Command` already answers that shape.
- A committed config file still holds every `ze:sensitive` value in the clear.
  `Editor.WorkingContent` serializes the tree and no serializer re-encodes `$9$`.
  Cleartext at rest is a product question for the owner, not a defect here.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

## Current Condition and What Remains

| Item | State |
|------|-------|
| Product code | UNCOMMITTED in the working tree at the time of writing: `cmd_set.go`, `cmd_edit.go`, `mask.go`, `schema.go`, `model_commands_edit.go`, `editor_commit.go`, `editor_draft.go`, `cli_terminal.go`, `secret.go`, plus the two YANG modules. No SHA can be cited |
| Journal row | written, `plan/journal/secret-echoed-to-the-client.md`, marked FIXED 2026-09-06 against work that has not landed |
| PROVEN | AC-1 through AC-7 for every producer except `cmdInsert`. Each fix was observed RED against the unfixed producer, and each carries the opposite polarity, so a fail-closed mask cannot satisfy it alone. AC-9 by the YANG walk |
| ASSERTED, not proven | AC-8 in the nil-schema arm. `TestDisplayTreeAtPathFailsClosed` covers the sibling function; no test named here drives `DisplayValueAtPath` with a nil schema. A-3 is unvalidated. `cmdInsert`'s polarity is unreachable by construction, stated in Known Limitations |
| Remains | (1) LAND IT; (2) the missing `.ci` over `ze config set`, since no functional test reaches the echo path; (3) a nil-schema test for `DisplayValueAtPath`; (4) the documentation anchors; (5) the transcript sink, which is its own journal row; (6) closure sections |
