# Spec: A web config field can lose the value the operator typed last

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P1 |
| Release reason | Incorrect operator-facing behaviour: an operator who edits one config field in the web UI can end with a value they did not type last, with no sign that the last value was not saved |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A web config leaf field saves itself on blur, on Enter and one second after the
last keystroke. On 2026-10-10 `test/web/interface-mac-override.wb` failed about
1 run in 6 under the stress runner and passed alone: the last `/config/set`
POST the server took carried an empty value, after the test cleared the field,
paused for over a second, and typed. The order of the POSTs was never captured,
so the mechanism is not known. A browser-side attempt to reject a stale swap in
`cli.js` still failed 1 run in 4.

Commit d674241c52 (2026-10-26) then changed the field for another defect: a
save the field issues while its own save is in flight is now dropped. Its
commit body states the cost: "a newer value sent inside that window is
discarded and the field shows the saved one". The 2026-10-10 failure has not
been re-run since, so whether it still happens, and by which path, is unknown.

The goal: after any sequence of edits to one field, the value the operator
sees is the value the server holds, and the last value typed is either saved
or shown as not saved.

This row was owned by `plan/release/fixes/spec-single-owner-lifecycle.md`. A
design pass found no server-side lifecycle or owner in it, so it is not that
spec's root cause.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/le-tooling.md`
- [ ] `docs/architecture/web-components.md` - the paragraph on the three save triggers and `hx-sync`
  → Constraint: markup is `.templ`, with no inline script or style
- [ ] `docs/architecture/web-interface.md` - the `// Design:` page of the field template and the set handler

**Key insights:**
- The mechanism is unknown. The reproduction comes before any design.

## Current Behavior (MANDATORY)

**Source files read:** (read on 2026-11-11)
- [ ] `internal/component/web/input_text.templ` - `inputText` posts to `configSetURL` on `blur changed, ze-enter, input changed delay:1s`, with `hx-sync` set to `fieldSaveSync`, target `closest .ze-field`, swap `outerHTML`. `input_number.templ` and `component_list_table.templ` carry the same trigger and the same `hx-sync` (grep, not read whole)
- [ ] `internal/component/web/view.go` - `fieldSaveSync` is `this:drop`. Its comment says htmx 4's default for an element is `queue first`, which sent one edit twice
- [ ] `internal/component/web/assets/htmx.min.js` - the sync queue's `admit` answers `dropped` for strategy `drop` when the element already has a request in flight. An `outerHTML` swap cleans up the replaced element, which clears the pending `delay` timer of each of its triggers, then restores focus and the caret to the element with the same id
- [ ] `internal/component/web/handler_config_form.go` - `HandleConfigSetWithAuthorizer` reads the form fields `leaf` and `value` and nothing else: no version, etag or sequence. On success it renders the field from the value this request posted (`buildFieldMetaFromLeaf` with `effectiveValue`), not from a read of the editor after the set. A refusal goes out through `WriteOOBError` with a 400 or 409
- [ ] `internal/component/web/assets/cli.js` - `initEnterSubmit` sends `ze-enter`. The file listens to `htmx:after:settle` and `htmx:after:request` only: nothing runs before a request or before a swap, and nothing aborts a request

**Behavior to preserve:**
- One edit reaches the editor once, and a refused value lists one error (`test/web/error-panel-toggle.wb`, the defect d674241c52 fixed)

**Behavior to change:**
- To be stated once the reproduction shows the mechanism

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-field request sync | `internal/component/web/view.go` `fieldSaveSync`, used by `inputText`, `inputNumber` and the list table cell | Drops a save the field issues while its own save is in flight | Undecided until the reproduction: it is one suspected place where a newer value is lost |
| Field-replacing answer | `internal/component/web/handler_config_form.go` `HandleConfigSetWithAuthorizer`, with `hx-target="closest .ze-field"` and `hx-swap="outerHTML"` | Every successful save redraws the whole field from the value that request posted | Undecided until the reproduction: a redraw that arrives while the operator types replaces the input they are typing in |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types in a config leaf field of the web UI. The browser sends a form POST to `/config/set/<yang-path>/` with `leaf` and `value`.

### Transformation Path
1. htmx admits or drops the request through the field's sync queue (`fieldSaveSync`).
2. `HandleConfigSetWithAuthorizer` validates the value and calls `EditorManager.SetValue` or `DeleteValue` on the user's draft.
3. The answer carries the redrawn field and the out-of-band commit bar, and htmx replaces `closest .ze-field` with it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser ↔ web handler | form POST, HTML fragment answer | Read at source on 2026-11-11, not observed in a browser |

### Changed-Wire Census

Not filled at skeleton: whether the POST or its answer changes depends on the
mechanism the reproduction shows.

### Integration Points
- `EditorManager.SetValue` and `DeleteValue` - the draft the save lands in (not read for this skeleton)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | not judged at skeleton |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Clear a leaf field, pause over one second, type | → | `inputText` save triggers and `HandleConfigSetWithAuthorizer` | `test/web/interface-mac-override.wb` under the stress runner, the test that failed on 2026-10-10 |

## Reproduction owed

No design is written and no further acceptance criterion is fixed until this is
done, because the mechanism was never captured.

1. Run `test/web/interface-mac-override.wb` under the stress runner at HEAD and record whether it still fails after d674241c52.
2. Capture, for one failing run, the order of `/config/set` POSTs the server took for the field, each with its value, and which of the field's saves the browser dropped.
3. Answer the question the journal row carries: does a value typed while the field's own save is in flight get lost, and what does the operator then see.

Two readings of the code, neither observed in a browser. The later save is
dropped because the empty save is still in flight. Or the empty save's answer
redraws the field while the operator types: the new input holds the empty
value, and the replaced input's pending one-second timer is cleared with it,
so the typed text is neither shown nor sent.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Any sequence of edits to one web config field, including clear, pause over one second, type | The value the field shows is the value the server holds, and the last value typed is either saved or shown as not saved | A save that repeats the value already in flight may still be dropped: d674241c52 needs it so one edit reaches the editor once |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named once the reproduction shows which side holds the defect | `internal/component/web/` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `interface-mac-override` | `test/web/interface-mac-override.wb` | The value typed after a clear and a pause is the value saved and shown, under the stress runner | |

## Files to Modify
- `internal/component/web/input_text.templ`, `internal/component/web/view.go`, `internal/component/web/handler_config_form.go` - the three producers; which of them changes is decided by the reproduction
- `docs/architecture/web-components.md` - the save-trigger paragraph, when the behavior changes

## Implementation Steps

1. **Phase: Reproduction** -- the three steps under "Reproduction owed"; the spec then moves to design with the mechanism named.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `late-write-lands-on-the-successor` | 2026-10-10 | web leaf field (`inputText`, `fieldSaveSync`, `/config/set`): a value typed during the field's own save |

## Related specs

| Spec | Relation |
|------|----------|
| `plan/release/fixes/spec-web-cli-config-one-off-defects.md` | Owns the other web UI one-off journal rows. This row would sit there, but that spec's edit gate asks for sections and source reads covering all of its rows |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Scoped runs: `./le test unit pkg path ./internal/component/web/`, `./le test functional web`, and `./le test stress-repro run suite web test 48` (the command the 2026-10-10 journal row recorded; not run for this skeleton)
