# Spec: web-templ-migration

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/web-templ-migration.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace `html/template` with templ (`github.com/a-h/templ`) as the rendering
engine for the web interface, so that a mismatch between a view model and its
markup is a compile error instead of a blank panel.

**The symptom that motivates this.** `internal/component/web/render.go` has ten
`return ""` paths. `RenderFragment`, `RenderField`, `RenderConfigToHTML` and
`RenderL2TPTemplate` each discard the template execution error and emit empty
HTML. `internal/component/lg/render.go` (`renderPage`) passes its data as
`map[string]any`, so no field name is checked at all. Renaming a field in a
view-model struct today produces an empty region in the config editor and no
error anywhere. For a config editor, an empty region means the operator cannot
see the configuration they came to read.

Written from an assessment Thomas asked for on 2026-08-09.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/web-interface.md` - the current rendering position
  → Constraint: line 3 states "All UI is server-rendered Go templates". That
    claim survives the migration; the mechanism changes, the position does not.
- [ ] `docs/architecture/web-components.md` - fragment and OOB design
  → Decision: "One template per visual concern" and "one HTTP request updates
    multiple components" (HTMX OOB). Both map onto templ components directly.
- [ ] `docs/architecture/web-workbench-pages.md` - the page layer
  → Constraint: pages build view-model structs and delegate rendering. The
    migration must not move markup back into the page files.
- [ ] `ai/rules/no-layering.md` - always-on
  → Constraint: delete X first, then implement Y. A steady state where some
    pages render through `html/template` and others through templ is banned.
    This is what forces per-renderer phasing rather than per-page.
- [ ] `ai/rules/simplicity.md`
  → Constraint: the migration adds machinery (a codegen step, a dependency).
    It must buy correctness, not ergonomics. The ten `return ""` paths are that
    purchase; nothing else here justifies the cost on its own.

**Key insights:**
- The page layer already separates view models from markup. `HandleRoutesPage`
  (`page_ip_routes.go`) builds a `WorkbenchTableData` and calls
  `RenderFragment`. About thirty page files do this and emit no markup at all.
  The migration is therefore mostly a template-language port, not a rewrite.
- There are TWO independent renderers, not one: `NewRenderer`
  (`web/render.go`) and `parseLGTemplates` (`lg/render.go`). They share no code.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/web/render.go` - `NewRenderer` parses 57 templates
  into groups (layout, workbench, login, a config map, a fragments set, an
  l2tp map). `RenderFragment` / `RenderField` / `RenderConfigToHTML` /
  `RenderL2TPTemplate` return `template.HTML` and swallow every error.
- [x] `internal/component/lg/render.go` - `parseLGTemplates`, `renderPage`,
  `renderFragment`, `renderToString`. Second, independent template set.
- [x] `internal/component/web/page_ip_routes.go` - `HandleRoutesPage`, the
  representative page: builds a struct, calls `RenderFragment`, emits no markup.
- [x] `internal/component/web/page_interfaces.go` - `writeKV` escapes both key
  and value with `template.HTMLEscapeString`; `capitalizeFirst` output reaches
  a plain `WorkbenchTableData.Title` field that `html/template` auto-escapes.
- [x] `scripts/dev/code_to_docs.py` - `check_path_exists` validates the doc
  anchor PATH only, never the symbol named after `--`.
- [x] `Makefile` - `generate` (line 188) is four `go run` calls, all from
  vendor, nothing on PATH. `ze-proto-gen` (line 204) is the precedent for a
  generator built from vendor and is deliberately excluded from `generate`
  because it needs `protoc`. `ze-plugin-imports-check` (line 214) is the
  precedent for a `--check` gate that catches stale generated output.
- [x] `tools.go` - the `//go:build tools` vendoring pattern for tool deps.

**Measured surface:**

| Surface | Size |
|---------|------|
| `web` templates | 57 files, 1631 lines |
| `lg` templates | 8 files, 356 lines |
| Literal HTML inside Go, 18 files | ~143 markup lines within 5244 file lines |
| `internal/chaos/web` (own `htmlWriter`, `ze_chaos` tag) | 5906 lines, out of scope |
| Go tests over web + lg | 21617 lines, 40 asserting on HTML substrings |
| `.ci` tests over the web component | 17 (a further 7 under `test/chaos-web/` are out of scope) |

**Behavior to preserve:**
- Every rendered byte. This migration is not user-visible. The 40 tests that
  assert on HTML substrings are the discriminator: a faithful port keeps them
  green, and that is the whole safety net.
- HTMX contract: `hx-swap-oob` OOB swaps (`WriteOOBError`, `HandleFragment`),
  and the SSE `data: ` line prefixing in `writeSSEEvent`.
- The no-inline-script policy that `TestTemplatesAvoidInlineScriptAndStyle`
  and `TestHandleCLIPageAvoidsInlineStyle` enforce over the template set.
- Escaping. It holds today by manual convention with no linter behind it.
  templ escapes by default, which is strictly safer, so the risk on migration
  is DOUBLE-escaping a value that a call site already escaped by hand, not
  under-escaping. Every `template.HTMLEscapeString` call must be deleted as its
  call site is ported.

**Behavior to change:**
- Template execution errors stop being silent. The ten `return ""` paths and
  the string-keyed dispatch maps (`config`, `l2tp`, and `fieldFor`'s runtime
  `input_<type>` lookup) become compile-time resolution or an explicit
  registry, which is this repository's unifying pattern.

## Data Flow (MANDATORY)

### Entry Point
HTTP request to a route registered through `webroute.go` (`RegisterWebRoute`).
Every caller is inside `internal/component/web`; no plugin registers a web
route and no plugin emits HTML.

### Transformation Path
1. Handler builds a view-model struct (`WorkbenchTableData`, `FieldMeta`, ...)
2. Today: `Renderer` executes a named `html/template` and returns
   `template.HTML`. After: the handler calls a typed templ component.
3. Bytes reach the response writer, an SSE `data: ` stream, or an OOB fragment.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `cmd/ze/hub/service_web.go` ↔ web package | calls `RenderFragment("diff_modal")` and writes the bytes itself | Yes, a Renderer consumer outside the package |
| web ↔ lg | none; independent renderers, no shared code | Yes |
| web ↔ `internal/chaos/web` | none; separate surface behind `ze_chaos` | Yes |

### Integration Points
- `tools.go` - the templ generator joins the existing `//go:build tools` set.
- `Makefile` `generate` - four `go run` calls today, all from vendor.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | handlers keep building view models; only the renderer changes. `HandleRoutesPage` shape is unchanged |
| No unintended coupling (components stay isolated) | Yes | `lg` and `web` stay independent; each phase touches one |
| No duplicated functionality (extends existing, does not recreate) | Yes | no-layering forbids two engines coexisting, so each phase REPLACES rather than adds |
| Zero-copy preserved where applicable (refs, not copies) | Yes | templ writes to an `io.Writer`; today's path buffers into `bytes.Buffer` then copies. Not worse, likely better |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `fieldFor`'s runtime `input_<type>` string lookup becomes an explicit component registry, moving TOWARD the pattern, not away |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Marginal vendor cost is about 5 to 6M | measured 2026-08-09: templ v0.3.1020 runtime 3.2M, generator 2.1M, `a-h/parse` 240K; its `x/tools`, `x/net`, `x/mod`, `x/sync` deps are already vendored here | the x/vuln precedent (`Makefile:370`) applies and the dependency is refused | `go mod vendor` on a real branch, `du -sh vendor` before and after, in phase 1 | unvalidated |
| A-2 | Escaping is safe at every call site today | traced every dynamic `.Str(` in the 18 markup files, including `writeKV`, `capitalizeFirst`, `smartHealthLabel`, `formatBytes` | a value gets double-escaped or under-escaped on port | `TestDecorationHTMLEscaped` plus a rendered-output diff per page | unvalidated |
| A-3 | A faithful port keeps the existing tests green | the tests assert rendered HTML, not the engine | the test suite must be rewritten, which removes the safety net and changes the cost by an order of magnitude | phase 2 (`lg`) measures it before phase 3 commits to `web` | unvalidated |
| A-4 | templ can express `fieldFor`'s dynamic dispatch | templ components are ordinary Go functions, so a map of constructors replaces a map of template names | the input-type dispatch needs a different shape | phase 3, proven by `TestRenderFieldResolvesDecoration` staying green | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Generated `*_templ.go` drifts from its `.templ` source | `ze-tracked-build-check` goes red, or worse stays green with stale output | `make ze-templ-generate-check`, modelled on `ze-plugin-imports-check` (`Makefile:214`); the generated file is committed with its consumer |
| R-2 | Doc prose goes stale silently | none: `check_path_exists` validates the anchor PATH only, so the 9 anchors naming `render.go` / `fragment.go` / `lg/layout.go` symbols pass even when those symbols are gone | update the three web architecture docs by hand in each phase |
| R-3 | Mutation testing scans generated code | `gomu` runtime rises, noise in results | add a `_templ.go` pattern to `.gomuignore` (`mk/test-mutation.mk`) |
| R-4 | Big-bang scope collides with no-layering | a phase cannot be committed without two engines live at once | phase per RENDERER, not per page: `lg` complete in one commit, `web` complete in another |
| R-5 | Double-escaping ships unnoticed | an operator sees `&lt;` in a field value | delete every `template.HTMLEscapeString` at each ported call site; `TestDecorationHTMLEscaped` covers the decorated path |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The web config editor renders wrong or blank. No protocol impact, no wire impact, no effect on `ze` running headless. |
| How is it reverted? | Single commit revert per phase, as long as each phase is one renderer. |
| Who else touches this path? | `cmd/ze/hub/service_web.go` consumes `RenderFragment` from outside the package. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| HTTP GET a looking-glass graph page | → | ported templ component replacing `renderPage` / `renderToString` | `TestRenderSVG` and `TestRenderSVGWithNames` (existing test, must stay green) |
| HTTP GET the looking-glass with a token | → | ported `lg` page component | `TestLGTokenMiddleware` (existing test) |
| HTTP GET `/show/interface/` in the workbench | → | ported component replacing `RenderFragment("workbench_table")` | `TestInterfaceTableData_Build` (existing test) |
| HTTP GET the config editor layout | → | ported component replacing `RenderLayout` | `TestRenderLayout` (existing test) |
| HTMX POST deleting a list entry | → | ported `HandleFragment` OOB path | `test/ui/web-delete-list-entry.ci` |
| HTMX POST committing config | → | ported `WriteOOBError` and commit bar | `test/ui/web-commit-transactional.ci` |
| `make generate` with a `.templ` edited but not regenerated | → | the templ generate step | `make ze-templ-generate-check` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A view-model field is renamed without updating its markup | `go build` fails. Today the page renders blank. |
| AC-2 | Every page and fragment reachable in the web UI is requested | Rendered bytes are unchanged from before the migration |
| AC-3 | `make generate` runs on a clean tree | No file changes. Generated output is in sync with its `.templ` sources |
| AC-4 | A package is declared migrated | No `html/template` parse call remains in it (no-layering) |
| AC-5 | A value containing `<script>` reaches any rendered field | It appears escaped exactly once, never twice, never raw |
| AC-6 | The ported template set is scanned for inline script and style | Still none, as today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Deletes a list entry in the config editor | HTTP → `HandleFragment` → templ component → HTMX OOB swap | `test/ui/web-delete-list-entry.ci` |
| 2 | Commits a config change and sees the result | HTTP → commit handler → `WriteOOBError` or commit bar → OOB swap | `test/ui/web-commit-transactional.ci` |
| 3 | Has a rejected commit reported back | HTTP → commit handler → error panel component | `test/ui/web-commit-reject.ci` |
| 4 | Opens a looking-glass AS-path graph | HTTP → `lg` handler → ported SVG component | `TestRenderSVGWithNames` |
| 5 | Signs in to the web UI | HTTP → `RenderLogin` replacement | `TestRenderLogin` and `test/plugin/web-auth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNewRenderer` | `internal/component/web/render_test.go` | the ported renderer constructs; AC-4 | existing, must stay green |
| `TestRenderLayout` | `internal/component/web/render_test.go` | layout bytes unchanged; AC-2 | existing, must stay green |
| `TestRenderLogin`, `TestRenderLoginOverlay` | `internal/component/web/render_test.go` | login page bytes unchanged; AC-2 | existing, must stay green |
| `TestRenderFieldResolvesDecoration` | `internal/component/web/render_test.go` | the input-type dispatch survives; A-4 | existing, must stay green |
| `TestDecorationHTMLEscaped` | `internal/component/web/render_test.go` | AC-5, escaped exactly once | existing, must stay green |
| `TestTemplatesAvoidInlineScriptAndStyle` | `internal/component/web/render_test.go` | AC-6 over the ported set | existing, must stay green |
| `TestRenderSVG`, `TestRenderSVGWithNames` | `internal/component/lg/layout_test.go` | `lg` output unchanged; AC-2 | existing, must stay green |
| `TestRouteTableData_Build`, `TestInterfaceTableData_Build` | `internal/component/web/page_*_test.go` | view models untouched by the port | existing, must stay green |
| `TestTemplComponentTypeSafety` | `internal/component/web/render_test.go` | AC-1: a renamed field fails the build | new |

### Boundary Tests (numeric inputs)
N-A. No numeric input is introduced or changed.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-delete-list-entry` | `test/ui/web-delete-list-entry.ci` | deletes a list entry, OOB swap updates the page | existing, must stay green |
| `web-commit-transactional` | `test/ui/web-commit-transactional.ci` | commits a change end to end | existing, must stay green |
| `web-commit-reject` | `test/ui/web-commit-reject.ci` | a rejected commit reports through the error panel | existing, must stay green |
| `web-tool-decode` | `test/ui/web-tool-decode.ci` | the tools page renders and works | existing, must stay green |
| `web-auth` | `test/plugin/web-auth.ci` | login and session | existing, must stay green |
| `web-startup` | `test/plugin/web-startup.ci` | the server starts and serves | existing, must stay green |
| `ospf-debug-web` | `test/ospf/ospf-debug-web.ci` | a registered per-protocol page renders | existing, must stay green |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling; no wire-visible behavior changes.

## Files to Modify
- `tools.go` - add the templ generator import
- `go.mod`, `go.sum`, `vendor/` - add `github.com/a-h/templ`
- `Makefile` - add the templ call to `generate`, plus `ze-templ-generate-check`
- `mk/test-mutation.mk` or `.gomuignore` - exclude `*_templ.go`
- `internal/component/lg/render.go` - phase 2
- `internal/component/lg/layout.go` - `renderGraphSVG` string building
- `internal/component/web/render.go` - phase 3
- `internal/component/web/fragment.go` - `WriteOOBError`, `HandleFragment`
- `internal/component/web/sse.go` - `notificationBannerTmpl`, `writeSSEEvent`
- `cmd/ze/hub/service_web.go` - external `RenderFragment` consumer
- `docs/architecture/web-interface.md`, `docs/architecture/web-components.md`,
  `docs/architecture/web-workbench-pages.md` - 9 source anchors name symbols
  this deletes

## Files to Create
- `internal/component/lg/templates/*.templ` and generated `*_templ.go`
- `internal/component/web/templates/*.templ` and generated `*_templ.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface changes |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | no CLI surface changes |
| CLI grammar (keyword before value) | N-A | no CLI surface changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new API; existing `.ci` set covers the surface |
| Pipe completeness | N-A | no CLI output |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | No | templ is a BUILD-time generator, not a runtime dependency. `ai/rules/repo-maintenance.md` requires a doctor check for runtime deps only. The generated code's runtime import is a vendored Go library like any other. |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | not user-visible by design (AC-2) |
| 2 | Config syntax changed? | No | no config surface |
| 3 | CLI command added/changed? | No | no CLI surface |
| 4 | API/RPC added/changed? | No | no API surface |
| 5 | Plugin added/changed? | No | no plugin emits HTML or registers a web route |
| 6 | Has a user guide page? | No | mechanism change only |
| 7 | Wire format changed? | No | not a protocol change |
| 8 | Plugin SDK/protocol changed? | No | unchanged |
| 9 | RFC behavior changed? | No | not a protocol change |
| 10 | Test infrastructure changed? | No | the browser harness in `internal/component/web/testing/` keeps its interface |
| 11 | Affects daemon comparison? | No | no feature difference |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-interface.md`, `docs/architecture/web-components.md` |
| 13 | Route metadata keys changed? | No | none |
| 14 | Prometheus counters changed? | No | none |
| 15 | Registered plugin/event/command/inventory changed? | No | `RegisterWebRoute` keeps its signature |
| 16 | Changed files referenced by doc source anchors? | Yes | 9 anchors across the three web docs name `render.go`, `fragment.go`, `lg/layout.go` symbols. `check_path_exists` will NOT catch these: it validates the path only. Update by hand. |
| 17 | Existing docs show examples for this area? | Yes | the layout diagrams in `web-components.md` describe rendered structure; verify each against the ported output |

## Implementation Steps

Phased per RENDERER, because `ai/rules/no-layering.md` forbids two engines
coexisting as a steady state. Each phase is one commit that fully replaces one
renderer.

1. **Phase: Wiring (MANDATORY FIRST)** -- toolchain only, no page ported
   - Tests: `make ze-templ-generate-check` (new, from the Wiring Test table)
   - Files: `tools.go`, `go.mod`, `go.sum`, `vendor/`, `Makefile`, `.gomuignore`
   - Verify: the check target fails while nothing is generated, then passes.
     `make ze-tracked-build-check` stays green. Measure the real vendor delta
     and settle A-1 before going further. If A-1 breaks, STOP and report.
2. **Phase: `lg`** -- 8 templates, 356 lines, one renderer, `map[string]any` data
   - Tests: `TestRenderSVG`, `TestRenderSVGWithNames`, `TestLGTokenMiddleware`
   - Files: `internal/component/lg/render.go`, `lg/layout.go`, `lg/templates/`
   - Verify: rendered bytes unchanged, no `html/template` parse call left in
     the package. This phase settles A-3. If it grates, STOP and report.
3. **Phase: `web`** -- 57 templates, the string-keyed dispatch maps, and
   `fieldFor`'s runtime `input_<type>` lookup
   - Tests: `TestNewRenderer`, `TestRenderLayout`, `TestRenderLogin`,
     `TestRenderFieldResolvesDecoration`, `TestDecorationHTMLEscaped`,
     `TestTemplatesAvoidInlineScriptAndStyle`, then the seven `.ci` tests
   - Files: `web/render.go`, `web/fragment.go`, `web/sse.go`, `web/templates/`,
     `cmd/ze/hub/service_web.go`
   - Verify: rendered bytes unchanged across every reachable page; A-4 settled
4. **Phase: Type-safety proof and docs**
   - Tests: `TestTemplComponentTypeSafety` (new), proving AC-1
   - Files: the three web architecture docs and their 9 source anchors
   - Verify: AC-1 demonstrated, `make ze-doc-test` green
5. **Phase: Go-literal markup (OPTIONAL)** -- the 143 lines across 18 files,
   chiefly `cli_terminal.go` (32), `page_system.go` (19), `page_interfaces.go`
   (19). Record in the deferral shard if not taken.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line. No `html/template` import left in a package declared migrated |
| Feature completeness | Every reachable page renders; no route lost in the port |
| Correctness | Rendered bytes identical: diff before and after, per page |
| Naming | Component names match the template names they replace, so the docs and the layout diagrams stay navigable |
| Escaping | Every `template.HTMLEscapeString` at a ported call site is DELETED, not left alongside templ's automatic escaping |
| Data flow | SSE still prefixes each line with `data: `; OOB swap attributes survive |
| Registration over hardcoding | `fieldFor`'s string dispatch becomes a registry, not a switch in a shared package |
| Rule: no-layering | Each commit leaves exactly one engine live per package |
| Rule: simplicity | The codegen step buys AC-1. If a phase buys nothing but ergonomics, drop it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| templ vendored, generator runs from vendor | `make generate` with no network access |
| No engine coexistence | `grep -rn 'html/template' internal/component/web internal/component/lg` returns nothing after phase 3 |
| Generated output in sync | `make ze-templ-generate-check` |
| Vendor delta measured | `du -sh vendor` before and after, recorded against A-1 |
| Type safety proven | `TestTemplComponentTypeSafety` fails when a field is renamed |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Double-escaping. Today's manual `template.HTMLEscapeString` plus templ's automatic escaping renders `&lt;` visibly to the operator. Every hand-escape at a ported call site must go. |
| Injection | `snapshotPageHTML` uses `JSEscapeString` for two JavaScript contexts. templ has its own script-context rules; port these deliberately, not mechanically. |
| Error leakage | The ten `return ""` paths become real errors. Confirm the new error text reaches the log, never the rendered page. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Rendered bytes differ | Re-read the original template. If the change is intentional → DESIGN |
| Generated file out of sync in CI | Fix the generate wiring in phase 1 |
| Lint failure | Fix inline. If architectural → DESIGN |
| A-1 breaks (vendor cost too high) | STOP at phase 1 and report; do not proceed to phase 2 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The repository has a precedent for a generator built from vendor
  (`ze-proto-gen`, `Makefile:204`) and a precedent for REFUSING a tool on
  vendor-churn grounds (x/vuln, `Makefile:370`). templ sits between them:
  lighter than govulncheck's analysis tree, heavier than nothing. A-1 decides.
- `make generate` today needs no binary on PATH. templ preserves that
  (`go run` from vendor), unlike `ze-proto-gen`, which needs `protoc`.
- `ze-plugin-imports-check` already models the stale-generated-output gate, so
  R-1 needs no new mechanism, only a new target that follows it.
- The doc gate is weaker than it looks. `check_path_exists` reads the path and
  ignores the symbol, so a migration that keeps `render.go` as a thin registry
  passes `make ze-doc-test` with every symbol name in the anchors already dead.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Phase per renderer | Phase per page (strangler fig) | `ai/rules/no-layering.md` forbids two engines as a steady state |
| `lg` first | `web` first | 356 lines against 1631, self-contained, and its `map[string]any` data path has the most to gain. One session buys the decision. |
| Keep the view-model structs | Move data assembly into components | The page layer's separation already works and 30 files depend on it. templ replaces the markup layer only. |

## Known Limitations
- `internal/chaos/web` (5906 lines, its own `htmlWriter` and `escapeHTML`) is
  out of scope. It shares no code with the web component.
- Static assets (8 JS files, 2 CSS) are untouched.
- Phase 5 (Go-literal markup) is optional and may be deferred.

## RFC Documentation (Scope: protocol)
N-A. Scope is tooling.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### Quality Gates
- [ ] `make ze-lint-changed` clean
- [ ] `make ze-doc-test` green after the 9 source anchors are corrected
- [ ] `make ze-tracked-build-check` green after each phase's commit
- [ ] No `html/template` import in any package declared migrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: none introduced)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: scope is tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
