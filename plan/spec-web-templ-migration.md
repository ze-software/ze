# Spec: web-templ-migration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Updated | 2026-08-15 |

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
  → Constraint: `renderPage` and `renderFragment` already handle errors
    correctly. Each logs and returns HTTP 500. They are not the sloppy path.
    They never FIRE, because `html/template` returns no error for a missing
    key on a `map[string]any`: it renders an empty value and reports success.
    Measured 2026-08-14 against the stdlib: a missing STRUCT field gives
    `can't evaluate field Titel in type main.View`, a missing MAP key gives
    `err=<nil>` and empty output. So `lg` is the worse surface, not the
    better-handled one, and no error-handling fix can reach it. Only typing
    the data can. This is what AC-8 exists to force.
- [x] `internal/component/web/page_ip_routes.go` - `HandleRoutesPage`, the
  representative page: builds a struct, calls `RenderFragment`, emits no markup.
- [x] `internal/component/web/page_interfaces.go` - `writeKV` escapes both key
  and value with `template.HTMLEscapeString`; `capitalizeFirst` output reaches
  a plain `WorkbenchTableData.Title` field that `html/template` auto-escapes.
- [x] `scripts/dev/code_to_docs.py` - `check_path_exists` validates the doc
  anchor PATH only, never the symbol named after `--`.
- [x] `Makefile` - `generate` (line 188) is three `go run` calls from vendor
  plus one `python3` call, nothing on PATH. Corrected 2026-08-14 in phase 1:
  the fourth line is `python3 scripts/dev/fuzz-targets.py`, not a `go run`.
  `ze-proto-gen` (line 204) is the precedent for a
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
| `cmd/ze/hub/service_web.go` ↔ web package | FOUR entry points, not one: `NewRenderer`, `RenderLogin` (inside `loginRenderer`, which reaches `AuthMiddlewareWithAudit` and `LoginHandlerWithAudit`), `RenderFragment("diff_modal_open")`, `RenderFragment("diff_modal")`. It also passes the `Renderer` into `zeweb.RouteDeps` | Yes, read 2026-08-14. A phase-3 port that covers only `diff_modal` leaves the hub-served LOGIN page broken |
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
| A-1 | Marginal vendor cost is about 5 to 6M | measured 2026-08-09: templ v0.3.1020 runtime 3.2M, generator 2.1M, `a-h/parse` 240K; its `x/tools`, `x/net`, `x/mod`, `x/sync` deps are already vendored here | the x/vuln precedent (`Makefile:370`) applies and the dependency is refused | `go mod vendor` on a real branch, `du -sh vendor` before and after, in phase 1 | **confirmed 2026-08-14.** `du -sk vendor` reads 46688 before and 51672 after, so the delta is 4984K, or 4.9M. That is a 10.7% rise and it clears the 8M stop condition. Largest single addition is `andybalholm/brotli` at 2560K, which templ's dev proxy needs. Then `a-h/templ` itself at 1776K, `a-h/parse` at 128K, and `cenkalti/backoff`, `cli/browser`, `fatih/color`, `mattn/go-colorable` and `natefinch/atomic` at under 50K each. `golang.org/x/net` and `golang.org/x/tools` gained packages and neither is new |
| A-2 | Escaping is safe at every call site today | traced every dynamic `.Str(` in the 18 markup files, including `writeKV`, `capitalizeFirst`, `smartHealthLabel`, `formatBytes` | a value gets double-escaped or under-escaped on port | `TestDecorationHTMLEscaped` plus a rendered-output diff per page | **lg half measured 2026-08-14, web half unvalidated.** `lg` had no hand-escape at any ported call site: its only `template.HTMLEscapeString` calls are in `layout.go` and `layout_nexthop.go`, which build SVG strings and belong to phase 5. So the port introduced no double escape in `lg`, and the fidelity comparison over 29 template units and 37 handler responses saw none. Phase 3 settles the `web` half, where `writeKV` and `capitalizeFirst` live. **Phase 3a measured its own half on 2026-08-14: no ported call site carried a hand-escape.** The 13 templates it ported read `LoginData.Error`, `commitModalData.Diff`, `FieldMeta` and `WorkbenchSection`, and every one of those values reaches its component raw. `LayoutData.Content`, `NotificationHTML` and `CLIPathBar` are `template.HTML` and go out through `templ.Raw`, which escapes nothing, so the `template.HTMLEscapeString` in `handler_config_commit.go` is still required and still correct. `writeKV`, `capitalizeFirst` and `snapshotPageHTML` belong to the templates 3b and phase 5 own. **CONFIRMED 2026-08-14 by phase 3b, and the risk ran the OTHER way.** No ported call site under-escaped. FIVE double-escaped: `handleDashboardEventsPage` and `parseLogOutput` (`page_dashboard.go`), `parseIssueRows` and its plain-text fallback (`page_logs.go`), and `runToolCommand` (`page_tools.go`). Each ran a cell through `template.HTMLEscapeString` and handed the result to markup that escapes again, so the operator read `&#34;` as text. The phase-1 handler capture holds the proof at HEAD: `testdata/handler/nav-show-events.txt` carries `{&amp;#34;result&amp;#34;:&amp;#34;golden&amp;#34;}`. All five hand escapes are deleted. A value reaching a `template.HTML` field keeps its own, because `templ.Raw` escapes nothing: `handler_tools.go` and `handler_config_commit.go` are correct as they stand |
| A-3 | A faithful port keeps the existing tests green | the tests assert rendered HTML, not the engine | the test suite must be rewritten, which removes the safety net and changes the cost by an order of magnitude | phase 2 (`lg`) measures it before phase 3 commits to `web` | **BROKEN 2026-08-14, and it invalidates AC-2 as written.** templ normalizes its output whitespace by design, and no flag turns that off. Three byte changes, each with a named producer. One: the doctype is lowercased unconditionally by `generateDocType` (`vendor/github.com/a-h/templ/generator/generator.go`), which writes `fmt.Sprintf("<!doctype %s>", ...)`. Two: whitespace between two nodes is DROPPED unless both are inline or text, decided by `isInlineOrText` (same file) over the `blockElements` table (`vendor/github.com/a-h/templ/parser/v2/types.go`). Three: whitespace that survives rule two becomes ONE space, because `TrailingSpace` (same file) holds only `SpaceNone`, `SpaceHorizontal` and `SpaceVertical`, and `writeWhitespaceTrailer` writes it only when rule two allows. Measured on a faithful port of `layout.html` rendered with the same data: 1023 bytes before, 1001 after, 26 lines joined into 1, `<!DOCTYPE html>` became `<!doctype html>`, and a new space appeared after each `</a>` in the tab bar. Every `lg` and `web` fixture would move. Byte identity is reachable only by writing each newline as a `{ "\n" }` expression and the doctype through `templ.Raw`, which costs templ the readability that is its remaining benefit. **RESOLVED by Thomas on 2026-08-14: keep templ and prove the port semantically.** A port is proven by rendering the same data through both engines and comparing with whitespace collapsed and the doctype case-folded. That comparison is what AC-2 now requires, because AC-2 always wanted no VISIBLE change and bytes were its proxy. HTML collapses a newline between block elements exactly as it collapses a space, so none of the three changes above reaches a reader. The fixtures are re-baselined to templ's exact output only AFTER that comparison passes, and byte-exact diffing resumes from the new baseline |
| A-4 | templ can express `fieldFor`'s dynamic dispatch | templ components are ordinary Go functions, so a map of constructors replaces a map of template names | the input-type dispatch needs a different shape | phase 3, proven by `TestRenderFieldResolvesDecoration` staying green | **confirmed 2026-08-14.** `fieldInputs` (`internal/component/web/field_input.go`) is a map literal from a field type to a `func(FieldMeta) templ.Component`. `fieldInputFor` reads it and names the text fallback the old lookup reached through a discarded error. Go refuses a duplicate key in a map literal, so two editors for one type is a build failure rather than an init-order race. `TestRenderFieldResolvesDecoration` and `TestDecorationHTMLEscaped` are green, and `TestFieldInputRegistryAnswersEachType` proves each field type reaches the editor the old lookup reached. **Corrected 2026-08-15: the set is NINE, not eleven, and it is DERIVED.** `buildFieldMeta` (`internal/component/web/fragment.go`) is the only non-test producer of a `FieldMeta`. It reads `valueTypeToFieldType` for the leaf's type, which answers eight strings over the `config.ValueType` run, and it overwrites the answer with `enum` for a leaf that declares enums. The earlier claim named eleven strings. Three of them (`number`, `text`, `""`) are produced by nothing, and `int` was missing, which every YANG `int8` through `int64` becomes (`yang_schema.go`). `producedFieldTypes` walks the iota run and fails when a constant is added past `TypeEmpty`, so the test cannot drift from its producer again. Proven to discriminate: dropping the `int` row reddens the test by name. `number` is a registered editor no leaf reaches, before the port and after it, and its row is in `plan/journal/unwired-feature.md` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Generated `*_templ.go` drifts from its `.templ` source | `ze-tracked-build-check` goes red, or worse stays green with stale output | CLOSED 2026-08-14 by `make ze-templ-generate-check`, modelled on `ze-plugin-imports-check`. It runs `templ generate -check -keep-orphaned-files`, and that flag is what makes the run write nothing and delete nothing. Measured both ways in a scratch tree: without it, `-check` removes an orphaned `*_templ.go` and exits 0. Two checks keep the scope complete, both in `scripts/dev/templ_orphan_check.py`: a `.templ` outside `internal/` fails, and so does a `*_templ.go` whose `.templ` is gone, which templ itself no longer reports once the flag is set. It is a prerequisite of `ze-regen-check-readonly`, so `make ze-verify` runs it |
| R-2 | Doc prose goes stale silently | RESOLVED 2026-08-14, and the mitigation inverts. `scripts/dev/code_to_docs.py` gained `check_anchor_symbols` on 2026-08-10 (commit `1307a1170`), so it now verifies the SYMBOL an anchor names, not just the path | the implementer meets a RED `make ze-doc-test`, not silent drift. Budget for it in each phase instead of hand-auditing. The anchors sit in `web-components.md` and `web-interface.md`; `web-workbench-pages.md` carries none |
| R-3 | Mutation testing scans generated code | `gomu` runtime rises, noise in results | CLOSED 2026-08-14. `.gomuignore` carries `*_templ.go` beside `*.pb.go`, under its "Generated code" heading |
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
| HTTP GET a looking-glass graph page | → | `handleGraph` (`internal/component/lg/handler_graph.go`) | `make ze-web-golden-check`, cases `ui-graph-aspath` and `ui-graph-nexthop` in `TestLGHandlerGoldenOutput`. CORRECTED 2026-08-14: the graph page reaches NO template. `handleGraph` calls `writeSVG` over `renderGraphSVG`, a `strings.Builder` path, so it never reached `renderPage` or `renderToString` and the port left it untouched. Its markup is phase 5 |
| HTTP GET the looking-glass with a token | → | ported `lg` page component | `make ze-web-golden-check`, cases `gated-peers-authorized` and `gated-peers-unauthorized` in `TestLGHandlerGoldenOutput`. Both pin the whole body, so the gated page is covered where `TestLGTokenMiddleware` asserts status alone |
| HTTP GET `/show/interface/` in the workbench | → | ported component replacing `RenderFragment("workbench_table")` | NEW test required. `TestInterfaceTableData_Build` asserts struct fields (`data.Title`, `data.Rows[0].Key`) and renders nothing |
| Every template body, before and after the port | → | the TEMPLATE golden capture | `make ze-web-golden-check` (built 2026-08-14). Proven to discriminate: one added space in `lg/templates/error.html` reds three named subtests |
| An HTTP request to every reachable route | → | the HANDLER golden capture | `make ze-web-golden-check` (built 2026-08-14): `TestWebHandlerGoldenOutput` and `TestLGHandlerGoldenOutput`. The template capture does NOT cover this: it executes the parsed template directly and bypasses `RenderLayout`, `RenderWorkbench`, `RenderFragment`, `RenderConfigToHTML`, `RenderField` and `RenderL2TPTemplate`, and its view models are authored in the test rather than built by a handler. Proven to discriminate: a change to `HandleAdminView`'s title composition reds two named handler cases and leaves the template capture green |
| HTTP GET the config editor layout | → | ported component replacing `RenderLayout` | `TestRenderLayout` (existing test) |
| HTMX POST deleting a list entry | → | ported `HandleFragment` OOB path | `test/ui/web-delete-list-entry.ci` |
| HTMX POST committing config | → | ported `WriteOOBError` and commit bar | `test/ui/web-commit-transactional.ci` |
| `make generate` with a `.templ` edited but not regenerated | → | the templ generate step | `make ze-templ-generate-check` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A view-model field is renamed without updating its markup | `go build` fails. Today the page renders blank. |
| AC-2 | Every page and fragment reachable in the web UI is requested | No rendered byte changes EXCEPT templ's own normalization, and a mechanical comparison proves it. Render the same data through both engines, collapse whitespace, case-fold the doctype, fold the attribute-quote delimiter, and fold character references. Then require equality. A port may not be re-baselined until that comparison passes for every unit. FIVE normalizations, not three. templ writes an expression attribute with double quotes only, so a source `hx-vals='{"leaf":"x"}'` becomes `hx-vals="{&#34;...}"`, and `html/template` escapes `+` as `&#43;` where templ does not. Both change the ENCODING and not the decoded value, so a browser reads them identically, which is why folding them is correct rather than convenient. Measured 2026-08-14 over 90 fixtures: 90 of 90 match once all five are folded. **The comparison MUST be runnable, not hand-run.** `golden.NormalizeHTML` had no caller outside its own test when phase 3a re-baselined, so AC-2's evidence was unreproducible. It takes the pre-port ref as a parameter, so it cannot fossilize. **DONE 2026-08-15: `make ze-templ-port-check REF=<sha>`.** `golden.AssertPortFidelity` (`internal/test/golden/portcheck.go`) reads the fixtures at REF with one `git archive`, reads the ones on disk, and compares each pair under `NormalizeHTML`. `TestWebTemplPortFidelity` and `TestLGTemplPortFidelity` call it, and neither is gated or skipped, so both also run under `make ze-unit-test`. A response is split first: its status and its headers are compared byte for byte, and only a body declaring `text/html` is normalized. A unit at REF with no fixture today is a finding, and so is a declared difference that no longer differs. Green against `80f0b8b57` for both packages on 2026-08-15. Proven to discriminate: one class renamed in `component/detail--fields.html` names that unit and prints the byte offset. Byte identity itself is unreachable: A-3 names the three producers. PROVEN FOR `lg` 2026-08-14: a normalizing comparison rendered every unit through templ and compared it against the pre-port bytes phase 1 committed. 29 template units and 36 handler responses matched under whitespace collapse and doctype case folding. The comparison was proven to discriminate on a ported component: one attribute value, one text node and one element name each reddened it. The fixtures were re-baselined only after that. THE INSTRUMENT IS `golden.NormalizeHTML` (`internal/test/golden/normalize.go`), landed 2026-08-14 so phase 3 inherits it. It is a pure function and carries no baseline, so it cannot fossilize. Phase 3 reads its pre-port bytes from git: `git show 80f0b8b57:internal/component/web/testdata/golden/<path>` and `git show 80f0b8b57:internal/component/web/testdata/handler/<name>.txt`, which is the commit that captured them. `TestNormalizeHTMLErasesLayoutOnly` proves the instrument discriminates: an attribute value, a text node, an element name and a dropped attribute each survive normalization as a difference. A newline against a space, and a doctype case change, do not. INSIDE a `<pre>` or a `<textarea>` every byte is content, so that newline DOES survive as a difference, and `TestNormalizeHTMLKeepsWhitespaceInsidePre` proves both halves. templ drops such a newline unless the port writes it as `{ "\n" }`, and a normalizer that collapsed it would call that port faithful. THREE captures are needed and phase 1 owns all three. The TEMPLATE capture pins each template body under fixed test-authored input. The HANDLER capture pins the response bytes of an actual HTTP request. The MARKUP capture pins the HTML the Go builders write, which no template holds. None proves another, and the `strings.Contains` suite proves none of them |
| AC-3 | `make generate` runs on a clean tree | No file changes. Generated output is in sync with its `.templ` sources |
| AC-4 | A package is declared migrated | No `html/template` parse call remains in it (no-layering) |
| AC-5 | A value containing `<script>` reaches any rendered field | It appears escaped exactly once, never twice, never raw |
| AC-6 | The ported template set is scanned for inline script and style | Still none, as today. The scan MUST walk `.templ` files: today's walk filters on a `.html` suffix, so a port that changes the extension makes the check pass over zero files |
| AC-7 | The web and lg packages are scanned for HTML tag literals in Go | None outside a `.templ` file, with ONE stated exception. **DONE 2026-08-15, and the criterion is a TEST now rather than a grep in a checklist: `TestNoGoFileBuildsMarkup` in each package, over `internal/test/markupcheck`.** The set was DERIVED, never taken from a list. 16 files held a literal shaped like a tag and one was a false positive: `handler.go` writes `/config/<verb>/<path>` in an error message. 13 are ported, and `web`'s exemption table is EMPTY. **The exception is a GENERATED DRAWING, stated as a rule rather than a file list.** `renderGraphSVG` (`lg/layout.go`) and `renderNextHopGraphSVG` (`lg/layout_nexthop.go`) write SVG whose every attribute is a coordinate `computeLayout` produced. Porting them buys neither thing this spec exists to buy. They read `layout.Positions[n.ASN].X` in Go, so a renamed field is ALREADY a compile error and the blank-panel failure cannot arise. Their only two interpolated values already go through `template.HTMLEscapeString`. It costs bytes and legibility: measured on a probe 2026-08-15, templ rewrites `<rect x="20" y="20" width="81" height="40"/>` as `<rect ...></rect>`. A graph draws three such elements per node and two per edge, and the response is `image/svg+xml`, which `golden.AssertPortFidelity` compares byte for byte rather than normalizing. `graphEmpty` (`lg/graph_empty.templ`) keeps the exception narrow. It is SVG too, and it IS a document, so it is ported. **The gate is fail-closed both ways.** A file that starts building markup is a finding, and an exemption that stops explaining one is a finding too, so no entry outlives its builder. The scan reads Go string LITERALS, so a tag in a comment is not a finding. It reads the FORM of a tag rather than its name, so the 14 `usage: set <leaf> <value>` strings in `cli_terminal.go` are not findings either. A name-based scan is a chore and a hole at once: `<path>` is an SVG element and also how a usage string names a path, and `<old-name>` carries the hyphen HTML gives a custom element. **HTML's void elements are the one case FORM cannot reach, and 5b closed it.** `<br>` never closes, so a bare `<br>` IS a whole element and `strings.Join(rows, "<br>")` showed the scan nothing. `voidElements` holds HTML's list, minus `command` and `source`, which ze writes as CLI usage text in `cli_terminal.go`. It changes the BARE case alone: each of the sixteen was already a finding once it carried an attribute. Proven to discriminate by `TestReportNamesEachMarkupLiteral` and its seven siblings, each building a throwaway package in `t.TempDir()`. **Two sibling scans joined it in 5b, over the `.templ` sources rather than the Go ones.** `AssertNoInlineScriptOrStyle` refuses what `'self'` refuses, and `AssertAssetsResolve` resolves each `src` and `href` against the served sub-FS. `lg` had neither, and it shipped two `onclick` handlers no browser ran |
| AC-8 | An `lg` page's view data is inspected after the port | It is a named struct. No `map[string]any` reaches a templ component. Renaming one of its fields fails the build, as AC-1 requires for `web`. Without this, porting `lg` to templ buys nothing: an unchecked map key stays unchecked inside a templ component. THE GUARD IS `internal/test/templcheck`, called by `TestLGViewDataIsTyped` and by phase 3 for `web`. It resolves a named type through the package's own declarations, so `type viewData map[string]any` is refused as the map it is, and it refuses a bare `any`. It is fail-closed: a type it cannot resolve is reported. It walks struct fields, nested and embedded ones included, so `struct{ Data map[string]any }` is refused as the map it wraps. That wrapper is the cheapest port of the `map[string]any` `web` builds today, and it defeats AC-8 one dereference in. A field of a type from another package is accepted, because refusing it would refuse `template.HTML`. `TestReportRefusesEachEscape` names every escape and proves each one reds |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Deletes a list entry in the config editor | HTTP → `HandleFragment` → templ component → HTMX OOB swap | `test/ui/web-delete-list-entry.ci` |
| 2 | Commits a config change and sees the result | HTTP → commit handler → `WriteOOBError` or commit bar → OOB swap | `test/ui/web-commit-transactional.ci` |
| 3 | Has a rejected commit reported back | HTTP → commit handler → error panel component | `test/ui/web-commit-reject.ci` |
| 4 | Opens a looking-glass AS-path graph | HTTP → `lg` handler → `renderGraphSVG`, which stays in Go by AC-7 | `make ze-web-golden-check`, cases `ui-graph-aspath` and `ui-graph-nexthop` of `TestLGHandlerGoldenOutput`. CORRECTED 2026-08-15: the row said "ported SVG component" and a NEW test was required. The handler capture phase 1 built IS that test, and it exercises the whole story, which `TestRenderSVGWithNames` could not. The empty-state drawing is covered too, by `graphEmpty` in `TestLGGoldenOutput` |
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
| `TestTemplatesAvoidInlineScriptAndStyle` | `internal/component/web/render_test.go` | AC-6 over the ported set | existing, MUST BE EDITED. Its `fs.WalkDir` skips any path without a `.html` suffix, so after the port it walks zero files and passes vacuously. Change the suffix to `.templ` in the SAME commit that renames the templates, and confirm it still counts the files it visited |
| `TestRenderSVG`, `TestRenderSVGWithNames` | `internal/component/lg/graph_test.go` | `lg` output unchanged; AC-2 | existing, must stay green |
| `TestRouteTableData_Build`, `TestInterfaceTableData_Build` | `internal/component/web/page_*_test.go` | view models untouched by the port | existing, must stay green |
| `TestTemplComponentTypeSafety` | `internal/component/web/templ_typesafety_test.go` | AC-1: a renamed field fails the build | WRITTEN 2026-08-14 in phase 1, because phase 4 needs the mechanism settled. The `go/packages` route was chosen over the `.ci` one: it needs no test runner and it reads structured type errors rather than compiler stderr. The fixture is `testdata/templtypesafety/`, one `.templ` plus its view model. The rename is an OVERLAY, so nothing broken reaches disk and two sessions can run it at once. Proven both ways: `go vet` on the fixture passes, and with `Title` renamed it fails inside `page_templ.go` on the generated read of `v.Title`. Proven to discriminate: markup that stops reading the field reds the test with "renaming the view-model field left the build clean". Phase 4 must repoint it at a real ported component |
| `TestLGViewDataIsTyped` | `internal/component/lg/view_test.go` | AC-8: no `map[string]any` reaches a templ component | DONE 2026-08-14. The rules live in `internal/test/templcheck`, so phase 3 calls the same guard |
| `TestReportRefusesEachEscape`, `TestReportPassesTypedComponents`, `TestReportRefusesAVacuousWalk` | `internal/test/templcheck/templcheck_test.go` | the AC-8 guard refuses a raw map, a named map, a slice of named maps, a pointer to a map, `any`, a foreign type and an undeclared name | DONE 2026-08-14. Each fixture is built in `t.TempDir()`, so no committed copy can drift from the rules |
| `TestNormalizeHTMLErasesLayoutOnly`, `TestNormalizeHTMLKeepsAGreaterThanInAValue` | `internal/test/golden/normalize_test.go` | the AC-2 instrument erases whitespace layout, doctype case, the attribute delimiter and the character-reference spelling, and nothing else | DONE 2026-08-14, EXTENDED 2026-08-15 to the last three normalizations. Four new `differ` rows prove the decode runs ONCE: a value escaped twice, in a text node and in an attribute, an escaped tag against a real one, and an ampersand added to a value |
| `TestWebTemplPortFidelity`, `TestLGTemplPortFidelity` | `internal/component/web/port_check_test.go`, `internal/component/lg/port_check_test.go` | AC-2, made runnable. Every fixture against its pre-port bytes at a REF the caller names | DONE 2026-08-15. `make ze-templ-port-check REF=80f0b8b57` is green over both packages |
| `TestPortFidelityNamesTheUnitThatChanged` and its four siblings | `internal/test/golden/portcheck_test.go` | the AC-2 comparison names a changed unit, refuses a unit that stopped being captured, refuses a stale exemption, splits a response head from its body, and errors on an empty pre-port side | DONE 2026-08-15. Each builds its own throwaway git repository, so no committed fixture can drift from the rules |
| `TestResponseBodyFindingRefusesAnEmptyAnswer` | `internal/test/golden/response_test.go` | the handler captures refuse a response with no body, and exempt only a redirect, a 204, a 304 and a stream | DONE 2026-08-15. The exemptions are derived from the captured status, never declared per case |
| `TestScalarStringRendersALargeASN`, `TestASPathRendersALargeASN` | `internal/component/lg/handler_ui_test.go` | a 4-byte ASN prints its digits in the browser column and in the CSV download | DONE 2026-08-14. Every fixture holds an ASN at or below 65002, where `%v` and `FormatFloat` agree, so no golden capture can see this |
| golden-output capture and diff | `internal/component/web/golden_test.go`, `internal/component/lg/golden_test.go` | AC-2, the only evidence for it | new, phase 1, BEFORE any page is ported |

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
- `tools.go` - add the templ generator import (DONE)
- `go.mod`, `go.sum`, `vendor/` - add `github.com/a-h/templ` (DONE)
- `Makefile` - add the templ call to `generate`, plus `ze-templ-generate-check`
  (DONE, and the check is a prerequisite of `ze-regen-check-readonly`)
- `.gomuignore` - exclude `*_templ.go` (DONE)
- `scripts/status/verify_run_test.go` - the `generate` recipe now holds five
  generators, and `generatorChecks` says which target guards the fifth (DONE)
- `scripts/dev/verify_wiring_docs.py` and its test - route a changed `.templ`
  or `*_templ.go` to the new gate (DONE)
- `internal/component/lg/render.go` - phase 2
- `internal/component/lg/layout.go` - `renderGraphSVG` string building
- `internal/component/web/render.go` - phase 3
- `internal/component/web/fragment.go` - `WriteOOBError`, `HandleFragment`
- `internal/component/web/sse.go` - `notificationBannerTmpl`, `writeSSEEvent`
- `cmd/ze/hub/service_web.go` - external `RenderFragment` consumer
- phase 5, the Go-literal markup. The set is DERIVED by
  `internal/test/markupcheck`, not by the line counts this bullet used to carry:
  16 files hold a tag-shaped literal, one is a false positive, 13 are ported and
  two are the drawing builders AC-7 exempts by rule
- `docs/architecture/web-interface.md`, `docs/architecture/web-components.md`,
  `docs/architecture/web-workbench-pages.md` - 9 source anchors name symbols
  this deletes

## Files to Create
- `internal/component/lg/*.templ` and generated `*_templ.go`, in the PACKAGE
  directory. DONE 2026-08-14. A `templates/` subdirectory is a second Go
  package, so the view models would have to move into it or the import would
  cycle. The decision is recorded under Implementation Steps, phase 2
- `internal/component/web/*.templ` and generated `*_templ.go`. Phase 3 faces the
  same choice over the four subdirectories of `web/templates/`, and `web`
  already keeps its view models in package `web`
- `internal/test/golden/normalize.go` - `NormalizeHTML`, the AC-2 instrument.
  DONE 2026-08-14
- `internal/test/templcheck/` - the AC-8 guard both packages call. DONE
  2026-08-14
- `internal/test/markupcheck/` - the AC-7 guard both packages call, and
  `markup_check_test.go` in each. DONE 2026-08-15
- `internal/component/web/assets/snapshot-live.js` - the snapshot page's live
  view, moved out of an inline script `script-src 'self'` refuses. DONE
  2026-08-15
- `internal/component/web/testdata/templtypesafety/` - the AC-1 fixture, and
  `internal/component/web/templ_typesafety_test.go`, its test (DONE)
- a new `lg` file holding one named view-model struct per page (AC-8), replacing
  the `map[string]any` that `renderPage` takes today
- `internal/component/web/golden_test.go`, `internal/component/lg/golden_test.go`
  and their captured fixtures, created in phase 1 against the UNPORTED tree

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

1. **Phase: Wiring (MANDATORY FIRST)** -- toolchain and EVIDENCE, no page ported
   - Tests: `make ze-templ-generate-check` (new), `make ze-web-golden-check` (new)
   - Files: `tools.go`, `go.mod`, `go.sum`, `vendor/`, `Makefile`, `.gomuignore`,
     `internal/component/web/golden_test.go`, `internal/component/lg/golden_test.go`
   - **Both golden captures happen HERE, against the UNPORTED tree.** They are
     the only things that can prove AC-2, and they are unobtainable once a page
     is ported: the bytes they must compare against no longer exist. Capturing
     late is the one mistake in this spec that cannot be corrected later.
   - **DONE 2026-08-14: the TEMPLATE capture.** 57 web files / 60 templates /
     141 fixtures, 8 lg files / 14 templates / 29 fixtures. It executes the
     parsed template directly, deliberately, because `RenderFragment`,
     `RenderField`, `RenderConfigToHTML` and `RenderL2TPTemplate` each discard
     the execution error and return `""`, so a capture taken through them would
     record an empty fixture as a pass. Proven red then green by hand.
   - **DONE 2026-08-14: the HANDLER capture.** 51 web routes (30 the hub
     registers plus 21 from the registry) and 29 lg routes, captured by 109 web
     fixtures and 37 lg fixtures in `testdata/handler/`. It issues a
     real HTTP request through the mux the hub builds and records the response.
     The route list is DERIVED, never typed: `RegisteredWebRoutes()`, the
     literal patterns in `cmd/ze/hub/service_web.go` and
     `internal/component/lg/server.go`, and `sections()` for the workbench
     pages. A route no case reaches fails the check by name. Proven to
     discriminate at the layer the template capture cannot see: changing
     `HandleAdminView`'s title composition leaves `TestWebGoldenOutput` green
     and reds `TestWebHandlerGoldenOutput` on both admin cases.
   - **DONE 2026-08-14: the MARKUP capture.** `TestWebMarkupGoldenOutput`
     renders the HTML builders that no template holds, over input the test
     fixes, into `internal/component/web/testdata/markup/`.
     `buildHostHardwareHTML` is the first: the handler capture normalizes that
     panel, because its live section list and rows follow what `host.Detect`
     finds on the machine. Proven to discriminate: one byte changed inside the
     builder's markup reds `TestWebMarkupGoldenOutput/host-hardware` and
     `TestWebHandlerGoldenOutput/nav-show-system-hardware`.
   - **DONE 2026-08-14: the TOOLCHAIN.** `github.com/a-h/templ` v0.3.1020 is in
     `go.mod` and vendored. `tools.go` imports `github.com/a-h/templ/cmd/templ`
     under the existing `//go:build tools` pattern, which is what carries the
     generator and the runtime into `vendor/`. `make generate` gained one
     `go run` from vendor, so it still needs no network and nothing on PATH.
     `make ze-templ-generate-check` is the freshness gate and is a prerequisite
     of `ze-regen-check-readonly`. `.gomuignore` excludes `*_templ.go`.
   - **DONE 2026-08-14: the AC-1 mechanism.** A fixture package under
     `internal/component/web/testdata/templtypesafety/`, loaded by
     `go/packages` from `TestTemplComponentTypeSafety`. The rename is applied as
     an OVERLAY, so no broken file is ever written. A file that must not compile
     takes its whole package down, and a package that does not build reports no
     test result, which is why the fixture cannot live in the package under
     test. `testdata` keeps it out of every build and every lint run.
   - Verify: the check targets fail while nothing is generated, then pass.
     `make ze-tracked-build-check` stays green.
   - **A-1 stop condition, stated as a number so it is decidable:** `vendor/`
     is 46M today. STOP and report if the delta exceeds 8M, which is a 17%
     rise for one build-time generator. Below that, proceed and record the
     measurement. The x/vuln precedent (`Makefile`) refuses on the SHAPE of
     the churn rather than a size, so it supplies no threshold; this one is
     set here to make the go/no-go answerable rather than a judgment call
     made under implementation pressure.
2. **Phase: `lg`** -- DONE 2026-08-14. 8 templates became 14 templ components,
   and every view model is a named struct
   - Tests: `TestLGViewDataIsTyped` (AC-8), `TestLGGoldenOutput` and
     `TestLGHandlerGoldenOutput` re-baselined onto templ's exact bytes
   - Files: `internal/component/lg/{layout,error,help,peers,peer_routes,
     route_detail,route_table,search}.templ` and their generated `*_templ.go`,
     `view.go` (new), `render.go`, `handler_ui.go`, `server.go`, `embed.go`.
     `internal/component/lg/templates/` is deleted
   - **The `.templ` files live in the package directory, not in a `templates/`
     subdirectory.** A subdirectory is a second Go package, and the view models
     would have to move into it or the import would cycle. One package keeps
     each struct beside the handler that fills it, which is where `web` already
     keeps its own
   - `lg/layout.go` is NOT touched. It builds SVG with a `strings.Builder` and
     holds no template parse call, so AC-4 is met without it. Its
     `template.HTMLEscapeString` calls belong to phase 5
   - **A named struct per lg page, and `map[string]any` is gone from the render
     path.** `extractPeers` returns `[]peerRow`, `extractBMPPeers` returns
     `[]bmpPeerRow`, `flattenPrefixSummary` returns `[]prefixSummaryRow`, and
     `routeRows` types the decoded RIB for the browser. `extractRoutes` keeps
     its `[]any` result, because the birdwatcher API serializes that value
     straight back to JSON
   - Typing removed three dead keys the maps carried and nothing read:
     `BGPPeers`, `Count` on the peer-routes page, and `Name` on a peer row
   - **Two defects the port replaced the producer of, both fixed.** The AS path
     rendered each ASN with `fmt.Sprint` on the decoded float64, so AS
     4200000000 reached the browser as `4.2e+09`. `scalarString`
     (`handler_ui.go`) uses `FormatFloat` with precision -1, and
     `formatASPathPlain` calls it, so the CSV download prints the digits too.
     The search page rendered its results panel only when there were routes,
     and the error banner lives inside that panel, so a bad filter answered 200
     with an empty form
   - `test/plugin/lg-ui-pages.ci` gained an invalid-prefix POST. Its other
     tokens are supplied by an `h1`, so they held whether or not the banner
     rendered. The new case asserts the banner element and the message inside
     it, and reverting `searchPage` reds it by name
   - Verify: AC-8 proven by `TestLGViewDataIsTyped`, which reads the generated
     components and refuses a map parameter. A-3 is settled: see AC-2
3. **Phase: `web`** -- 57 templates, the string-keyed dispatch maps, and
   `fieldFor`'s runtime `input_<type>` lookup
   - Tests: `TestNewRenderer`, `TestRenderLayout`, `TestRenderLogin`,
     `TestRenderFieldResolvesDecoration`, `TestDecorationHTMLEscaped`,
     `TestTemplatesAvoidInlineScriptAndStyle`, then the seven `.ci` tests
   - Files: `web/render.go`, `web/fragment.go`, `web/sse.go`, `web/templates/`,
     `cmd/ze/hub/service_web.go`
   - **Prove the port before re-baselining any fixture**, the way phase 2 did.
     Render each unit through templ, read the pre-port bytes with
     `git show 80f0b8b57:<path>`, and compare through `golden.NormalizeHTML`.
     AC-2 forbids a re-baseline until that comparison passes for every unit
   - **Call `templcheck.AssertTyped` from a `web` test**, with the component
     count. That is AC-8's guard, and `web` inherits no typing check without it
   - **Write every newline inside a `<pre>` as `{ "\n" }`.** Nine web templates
     carry a `<pre>` and `component/tool_bgp_decode.html` carries a
     `<textarea>`. templ has two raw elements, `style` and `script`
     (`parser/v2/templateparser.go`, `rawElements`), so source layout inside a
     `<pre>` is rewritten like any other whitespace. `commit.html` writes one
     newline after each `</span>` inside `<pre class="commit-diff-output">`, and
     those newlines are the line breaks of the config diff. A naive port drops
     them and renders the diff on one line. `{ "\n" }` reaches the output
     through `templ.EscapeString`, which escapes five characters and no newline
   - **A struct that wraps `map[string]any` does not satisfy AC-8.**
     `handler_admin.go` and `handler_l2tp.go` build that map today.
     `templcheck` walks struct fields, embedded ones included, and refuses the
     map it reaches. Each map becomes named fields
   - `service_web.go` uses FOUR renderer entry points (Boundaries Crossed), not
     the one the table used to name. `RenderLogin` reaches it through
     `loginRenderer`, which both `AuthMiddlewareWithAudit` and
     `LoginHandlerWithAudit` hold. Port all four or sign-in breaks.
   - Verify: golden bytes unchanged across every reachable page; A-4 settled
   - **DONE 2026-08-14, in two packages.** 3a ported the renderer core, the input
     registry, 13 spine templates and the six hub entry points. 3b ported the
     remaining 44: the config map, the l2tp pair, 29 fragments and the two
     unwired root templates. 72 templ components, no `html/template` parse call
     on any runtime path, and `NewRenderer` holds no template at all
   - **The two string-keyed maps are gone.** `configViewComponent`
     (`handler_config_leaf.go`) is a switch over `config.NodeKind` returning a
     component, and it answers nil for the two kinds that could never render.
     The l2tp map is two direct calls. Both dispatches are now resolved by the
     compiler, which is what the Architectural Verification row asked for
   - **The l2tp JSON body is a SEPARATE value from the view.** A Go map
     serializes in sorted key order and a struct in declaration order, so one
     value feeding both would move the response bytes the handler capture pins.
     `view_l2tp.go` holds the view models and the handlers keep their map
   - **AC-2 was proven before any fixture moved.** 97 unit comparisons rendered
     the same data through both engines; 109 handler responses were compared
     against their committed bytes. Both under `golden.NormalizeHTML`, with two
     documented spelling rewrites on the html/template side: `&#43;`, `&#61;`
     and `&#96;`, which `templ.EscapeString` does not produce, and a
     single-quoted attribute value, which `writeExpressionAttribute` never
     writes. Proven to discriminate: one renamed class reddens `log-table-rows`,
     and one dropped `{ "\n" }` inside `<pre>` reddens `commit-diff`
   - **ONE rendered byte moved on purpose**, and it is AC-5: see A-2
3b. **Phase: the interface defect pass** -- DONE 2026-08-15. Ten defects the
   port RECORDED rather than fixed are fixed, each with a test that would have
   caught it. Thomas ruled the ports land byte-faithful first and the defects
   follow as their own commits, so the golden diff shows what changes on screen
   - Files: `component_workbench_form.templ`, `component_workbench_table.templ`,
     `component_finder.templ`, `component_sidebar.templ`,
     `component_oob_error.templ`, `component_error_panel.templ`,
     `component_oob_save.templ`, `component_dashboard_health.templ`,
     `workbench_form.go`, `view_fragment.go`, `view_config.go`, `field_input.go`,
     `rbac.go`, `auth.go`, `server.go`, `handler_config_form.go`,
     `handler_config_leaf.go`, `handler_config_commit.go`,
     `handler_config_entry.go`, `assets/cli.js`, `cmd/ze/hub/service_web.go`
   - **Each fix has its own test, and each test was proven to discriminate.**
     The defect was restored and the named test went red. The four that are not
     obvious from the assertion are the security-header wrapper, the duplicate
     DOM id, the error drawer and the commit-bar pair
   - **The captured bytes moved, deliberately.** 79 fixtures: 23 template and 56
     handler. `make ze-templ-port-check REF=80f0b8b57` names each moved unit and
     the reason, in `webPortTemplates` and `webPortHandlers`
     (`port_check_test.go`). That table is fail-closed. An entry naming a unit
     that stops differing is itself a finding
   - **Two findings the journal rows had not seen.** The security-header row
     said three responses carried none. The capture found FIVE. The extra two
     are `/assets/` on a miss, and the login redirect that hands out the session
     cookie. `HandleConfigChanges` also took no authorizer at all, and it is the
     live path for the commit-bar defect: `refreshCommitBar` (`assets/cli.js`)
     fetches `/config/changes` on every page load
   - Verify: `ze-web-golden-check`, `ze-templ-port-check`,
     `ze-templ-generate-check`, `ze-test-pkg PKG=./internal/component/web`,
     `ze-web-test` (87/87) and `ze-ui-test` (169/169) are green
3c. **Phase: the defect pass reviewed** -- DONE 2026-08-15. Four review
   findings closed. One was a live secret exposure the pass had reported closed
   - **The mask reads the schema, not the field type.** `maskSecretLeaf`
     (`secret.go`) answers on `Sensitive || Bcrypt`. Masking on bcrypt alone
     rendered every `ze:sensitive` leaf in plaintext on the generic config
     path, `l2tp/shared-secret` among them. `formFieldValue` is deleted. It was
     a second rule for one property, and it hid this one
   - **A page holds no leaf node**, so `renderPageContent` (`workbench_pages.go`)
     masks the display tree each page reads. One door, and a page added later
     inherits it
   - Files: `secret.go` (new), `fragment.go`, `handler_config_leaf.go`,
     `handler_config_entry.go`, `workbench_pages.go`, `workbench_form.go`,
     `editor.go`, `page_dashboard.go`, `view_fragment.go`,
     `component_list_table.templ`, `component_workbench_table.templ`,
     `component_workbench_form.templ`, `component_dashboard_health.templ`,
     `config_leaf_input.templ`
   - Tests: `TestNoRenderPathEmitsAStoredSecret` and
     `TestSecretMaskingFollowsTheSchemaMarking` (six render paths each),
     `TestAnUntouchedSecretIsNeitherRewrittenNorDeleted`,
     `TestErrorDrawerWiringHoldsTogether`,
     `TestDashboardHealthDrawsOneCellPerHeaderColumn`,
     `TestCapturedTableRowsCoverTheirHeader`, and the workbench composition
     added to `TestRenderedPageCarriesNoDuplicateDOMID`
   - **No captured byte moved.** The one fixture that would have, the workbench
     form, is fed the placeholder by its builder. That is what a real page now
     passes, so `ze-templ-port-check REF=80f0b8b57` needed no new entry
   - Verify: `ze-web-golden-check`, `ze-templ-port-check`,
     `ze-templ-generate-check`, `ze-test-pkg PKG=./internal/component/web`,
     `ze-web-test` (87/87) and `ze-ui-test` (169/169) are green
3d. **Phase: the masking property closed in both directions** -- DONE
   2026-08-15. Round 2 found the mask still incomplete. Seven findings closed
   - **The property is one sentence.** No path renders a secret the schema
     marks, and no path stores the placeholder as a value. Every finding below
     was that one property failing somewhere
   - **One predicate, exported.** `config.LeafHoldsSecret`
     (`internal/component/config/mask.go`) is the only answer to "does this leaf
     hold a secret". `config.MaskSecrets` is the only whole-tree display mask,
     and `web.maskedDisplayTree` is deleted for it. `MaskBcrypt` stays narrow
     because `ze config dump` calls it before it writes `$9$`, and widening it
     would replace an encoded secret with the placeholder
   - **The web CLI published secrets on two routes.** `serializeTreeAtPath` and
     `serializeSetAtPath` (`cli_terminal.go`) masked through `MaskBcrypt`, and
     `EditorManager.ContentAtPath` (`editor.go`) reached the editor's
     bcrypt-only `DisplayContentAtPath`. The `show` verb needs no config
     authorization, so both reached any authenticated session
   - **The write guard widened with the read half.**
     `config.RejectMaskedBcryptLeaves` keyed on `leaf.Bcrypt`, so a
     `ze config dump --strip` uploaded back stored the placeholder as the
     secret. It reads `config.LeafHoldsSecret` now, and phase 3e renamed it
   - **`ze:ephemeral` is NOT in the predicate.** It answers whether a value is
     persisted. `plaintext-password` holds a cleartext secret, so the two YANG
     modules say so with `ze:sensitive` now
   - **`EditorManager.Compare` is deleted.** It returned the editor's raw text
     diff, secrets included. `EditorManager.Diff` masks each change against the
     schema and is the one diff the web renders
   - **The page mask is lazy and its door is enforced.** `renderPageContent`
     builds the masked tree at most once, and only for a branch that reads
     config. `TestWorkbenchPagesReceiveOnlyTheMaskedTree` reads the function's
     own source, so a page handed the raw tree is a red test
   - Files: `internal/component/config/mask.go`, `secret.go`, `editor.go`,
     `cli_terminal.go`, `workbench_pages.go`, `page_dashboard.go`,
     `internal/component/ssh/yang/ze-ssh-conf.yang`,
     `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang`
   - Tests: `TestNoRenderPathEmitsAStoredSecret` and
     `TestSecretMaskingFollowsTheSchemaMarking` (nine render paths each),
     `TestUploadedConfigWithAMaskedSecretIsRefused`,
     `TestAWriteOnlyPasswordLeafIsMarkedSensitive`,
     `TestWorkbenchPagesReceiveOnlyTheMaskedTree`,
     `TestBalancedBlockReadsPastTextAndComments`,
     `TestMarkupHasClassReadsOneClassInsideTheAttribute`,
     `TestMaskSecretsCoversEverySecretLeaf`,
     `TestRejectMaskedSecretLeavesCoversTheSameLeavesAsTheMask`,
     `TestLeafHoldsSecretReadsTheTwoSecretExtensions`
   - **No captured byte moved**, so `ze-templ-port-check REF=80f0b8b57` needed
     no new entry
   - Verify: `ze-web-golden-check`, `ze-templ-port-check`,
     `ze-templ-generate-check`, `ze-test-pkg` on `./internal/component/web`,
     `./internal/component/config`, `./internal/component/cli`,
     `./internal/component/config/cli` and `./internal/component/ssh`,
     `ze-web-test` (87/87) and `ze-ui-test` (179/179) are green
3e. **Phase: the six interface-defect findings of round 3** -- DONE
   - **A masked compare stated something false.** `compareTreesAtPath`
     (`cli_terminal.go`) diffs two masked texts, so a rotated secret read as the
     same placeholder on each side and the verb answered `(no changes)`. New
     `config.ChangedSecretPaths` and `config.ChangedSecretPathsSubtree`
     (`internal/component/config/mask.go`) walk the two trees together and name
     each leaf that moved. `changedSecretLines` writes one line per leaf, with
     the display placeholder standing in for the value, which is the shape
     `EditorManager.Diff` already writes
   - **The guard's name and its diagnostic code follow its predicate.**
     `config.RejectMaskedSecretLeaves` replaces `RejectMaskedBcryptLeaves` at
     all nine call sites, and the code `config-bcrypt-masked` becomes
     `config-secret-masked`, registered in `internal/core/diagnostic/codes.go`.
     A code is a contract surface that `ze explain` reads
   - **`EditorManager.ContentAtPath` fails closed.** Its fallback called
     `Editor.DisplayContentAtPath`, whose first branch answers the raw working
     content. It now answers nothing and logs at Warn
   - Two comments corrected: `parseConfigFormFields` cited the deleted
     `formFieldValue`, and `cmd/ze/hub/service_web.go` called the config
     download a read path for any authenticated session. `editWrap` gates it
   - The surviving CLI hole is recorded in
     `plan/journal/secret-echoed-to-the-client.md`, with all six producers
   - Files: `internal/component/config/mask.go`,
     `internal/core/diagnostic/codes.go`,
     `internal/component/config/cli/cmd_validate.go`,
     `internal/component/web/{cli_terminal,editor,handler_config_form}.go`,
     `internal/component/cli/{validator,editor_commit,editor_commands}.go`,
     `internal/component/cli/contract/contract.go`, `cmd/ze/hub/service_web.go`
   - Tests: `TestChangedSecretPathsNamesARotatedSecret`,
     `TestChangedSecretPathsReportsOnlyWhatTheMaskHides`,
     `TestChangedSecretPathsSubtreeIsRelativeToItsNode`,
     `TestCompareNamesARotatedSecretAndPublishesNeitherValue`,
     `TestContentAtPathAnswersNothingWhenItCannotMask`, the new
     `cli-terminal-compare` case of `TestNoRenderPathEmitsAStoredSecret` and
     `TestSecretMaskingFollowsTheSchemaMarking`, plus
     `test/parse/bcrypt-placeholder-rejected.ci` seq 2 for the code
   - **No captured byte moved**, so `ze-templ-port-check REF=80f0b8b57` needed
     no new entry
4. **Phase: Type-safety proof and docs**
   - Tests: `TestTemplComponentTypeSafety` (new), proving AC-1
   - Files: the three web architecture docs and their 9 source anchors
   - Verify: AC-1 demonstrated, `make ze-doc-test` green
5. **Phase: Go-literal markup** -- DONE 2026-08-15. 13 files ported into 13 templ
   components' worth of markup, six dead components deleted, and AC-7 made a
   runnable gate
   - Files: `web/{cli,cli_terminal,page_system,page_interfaces,page_bgp_peers,
     page_snapshot,page_l2tp,page_traffic,page_l2tp_off,page_vpn_ipsec_off,
     handler_portal,handler_config_commit,snapshot_views,handler_isis,
     handler_ospf,page_workbench_generic}.go`, `lg/handler_graph.go`, and the new
     `.templ` files beside them. NEW `internal/test/markupcheck`
   - **The set was DERIVED, not read off the spec.** The earlier count of 18
     files and 143 lines was a line-based grep. A literal-aware scan finds 16
     files, one of them a false positive, and it separates markup from the 17
     `usage: <leaf>` strings a line grep counted
   - **The two SVG drawing builders stay in Go, deliberately.** AC-7 states the
     rule and the measurement. `writeEmpty` (`lg/handler_graph.go`) is SVG and a
     document, so it is ported to `graphEmpty`
   - **The snapshot page's inline script was a live defect, and the port could
     not land without fixing it.** `snapshotPageHTML` wrote its `EventSource`
     inline, `setSecurityHeaders` (`auth.go`) puts `script-src 'self'` on every
     response through the whole-mux wrapper, so a browser refused it and the
     IS-IS and OSPF live views never updated. Porting the markup would also have
     reddened `TestTemplatesAvoidInlineScriptAndStyle`, which walks `.templ`
     files for `<script>`. The stream path and the event name are data
     attributes now, read by the new `assets/snapshot-live.js`, which follows
     `log-live.js`. Both `JSEscapeString` calls are gone with the script:
     neither value is JavaScript any more. Eight handler fixtures move, recorded
     in `webPortHandlers`
   - **Six dead components deleted**, each with what proved it dead: `configFlex`
     (no producer builds a `configFlexData`), `terminalPage` and
     `notificationBannerOOB` (ports of a template file `NewRenderer` never
     parsed, whose live markup is now `terminalContent` and `cliNotificationOOB`),
     `cliBar`, `dashboardOverview`, `sidebar` and `sidebarSection` (no caller).
     Each was checked by the component identifier, the old template name, and
     any string that could reach it: the only references were the golden capture
     itself. `FragmentData.Sidebar` survives, because `detail`
     (`component_detail.templ`) reads its length for an empty state
   - **One rendered byte moved that was not intended, and it was caught and
     closed.** Three sibling `if` blocks inside `configViewBody` put two spaces
     before `</main>` in the CLI response, because templ counts an `if` block as
     inline. The three sections are components on one line now, and
     `post-cli-mode` is byte-identical to its pre-port capture
   - Verify: AC-7 by `TestNoGoFileBuildsMarkup` in both packages.
     `ze-templ-port-check REF=80f0b8b57` reports no unexplained difference,
     `ze-web-test` 87/87 and `ze-ui-test` 179/179 green
5b. **Phase: the review's five findings, plus three record defects** -- DONE
   2026-08-15. The gate got the case it could not see, `lg` got the gate it
   never had, and one live outage closed
   - **The AC-7 scan missed HTML's void elements.** `<br>`, `<hr>` and
     `<input>` never close, so `strings.Join(rows, "<br>")` carried no close
     tag, no self-close and no attribute, and the three form rules saw nothing.
     `voidElements` (`markupcheck.go`) holds HTML's list, minus `command` and
     `source`, which ze writes as CLI usage text in `cli_terminal.go`. The list
     changes the BARE case alone: every void element was already a finding once
     it carried an attribute. `TestReportReadsABareVoidElement` and
     `TestReportPassesTheTwoVoidNamesZeWritesAsCLIText` hold both halves
   - **`lg` shipped the same class of outage this phase exists to close.**
     `route_table.templ` carried an `onclick` on each graph-mode button, and
     `lg` answers `default-src 'self'` with no `script-src` beside it
     (`setSecurityHeaders`, `server.go`), so a browser refused the handler and
     the pressed button never gained `.active`. `assets/graph-mode.js` carries
     the class change now, delegated on the document so it survives the HTMX
     swap. Proven live in a browser over the real fixture and the real asset
   - **`lg` had no equivalent of `TestTemplatesAvoidInlineScriptAndStyle`.**
     The scan moved into `markupcheck.AssertNoInlineScriptOrStyle`, and both
     packages call it. Copying the body into `lg` would have been the second
     copy of a check both packages owe
   - **Nothing proved an asset resolved.** `AssertAssetsResolve` resolves each
     `src` and `href` against the SERVED sub-FS, in both packages. Renaming
     `assets/snapshot-live.js` now reds `TestTemplAssetsResolve` instead of
     killing the live view in silence
   - **The three record defects.** `webPortHandlers` said "the ONE response" of
     nine and had absorbed `portSnapshotScript`'s doc comment. `AssertNoMarkup`
     failed only at zero literals, where its siblings use counted floors, so it
     takes a `Floors` now. `Findings` could not see an exemption table GROW,
     which is the one edit that turns a finding green, so `Floors.Exempt` fixes
     the size
   - Verify: `TestGraphModeScriptTargetsTheButtons` (`lg`) holds the class the
     script selects against the class the buttons carry, which is the join no
     other guard reads

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
| templ vendored, generator runs from vendor | `make generate` with no network access. DONE 2026-08-14: `GOPROXY=off make generate` completes, and the templ step reports `Complete [ updates=1 duration=99ms ]` |
| No engine coexistence | `grep -rn 'html/template' internal/component/web internal/component/lg` returns nothing after phase 3 |
| Generated output in sync | `make ze-templ-generate-check`. DONE 2026-08-14, and proven both ways: a one-tag edit to a `.templ` with no regeneration reds it by file name, and the restored pair is green |
| Vendor delta measured | `du -sh vendor` before and after, recorded against A-1. DONE 2026-08-14: 46688K to 51672K, a 4984K delta |
| Type safety proven | `TestTemplComponentTypeSafety` fails when a field is renamed, through the mechanism chosen in phase 1. The mechanism is DONE 2026-08-14 and green over a fixture. Phase 4 must repoint it at a ported component |
| `lg` view data typed (AC-8) | No `map[string]any` in the `lg` render path; `TestLGViewDataIsTyped` green |
| Rendered bytes unchanged (AC-2), markup layer | `make ze-web-golden-check` against the phase-1 template capture |
| Rendered bytes unchanged (AC-2), composition and handler layers | `make ze-web-golden-check`, the handler half. The template capture cannot stand in for it: it bypasses every wrapper phase 3 rewrites |
| Rendered bytes unchanged (AC-2), against the PRE-PORT bytes | `make ze-templ-port-check REF=80f0b8b57`. `ze-web-golden-check` proves the fixtures are the current render. Once a fixture is recaptured it compares the port against itself, and only this target still reads what the port had to preserve |
| A handler that renders nothing cannot be captured | `make ze-web-golden-check`. `golden.AssertResponseHasBody` refuses an empty body on both handler captures, during a capture run as much as a check run. Proven by name: an early return in `HandleAdminView` reds `get-admin`, and one in `handleUIHelp` reds `ui-help` |
| No HTML literals left in Go (AC-7) | `make ze-test-pkg` on either package runs `TestNoGoFileBuildsMarkup`. **A GREP MUST NOT BE THE CHECK.** The one this row used to name matched the double-quoted form alone, found 8 sites of 107, and nothing ran it. The scan reads Go string literals in both forms and judges the FORM of the tag, so it neither misses a raw string nor reds on `usage: set <leaf> <value>`. `lg` carries two named exemptions and `web` carries none |

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
| Keep the view-model structs in `web`. INTRODUCE them in `lg` | Move data assembly into components; or port `lg` markup while keeping its maps | `web` already separates view models from markup and 30 files depend on it, so templ replaces its markup layer only. `lg` has no structs to keep: its render path takes `map[string]any`, which is the reason its defects are invisible. "Keep the view-model structs" read as one rule for both packages would have made phase 2 a no-op purchase. AC-8 splits them. |

## Known Limitations
- `internal/chaos/web` (5906 lines, its own `htmlWriter` and `escapeHTML`) is
  out of scope. It shares no code with the web component.
- Static assets (8 JS files, 2 CSS) are untouched.

Nothing in this spec is deferred. Every phase is in scope, so the metadata
carries no deferral shard.

## RFC Documentation (Scope: protocol)
N-A. Scope is tooling.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated, each by a check that would FAIL if the
      behavior broke (`ai/rules/interop-and-goal-validation.md`). Five ACs
      carried non-discriminating proofs before 2026-08-14; do not reintroduce one
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Nothing deferred: every phase landed, no deferral shard created

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

---

## Implementation Summary

### What Was Implemented

`html/template` is gone from both render paths. 58 `.templ` components in `web`
and 9 in `lg`, each a Go function over a named view model, so a field the markup
misspells is a build failure. The silent empty-HTML paths are gone with the
engine. One remains and it is not silent: `renderComponent`
(`internal/component/web/render.go`) logs at Warn and names the component, for
the page builders that compose markup into `LayoutData.Content` and carry no
error path of their own.

Phases 1, 2, 3, 3b, 3c, 3d, 3e, 5 and 5b landed in earlier commits. This closure
implements phase 4 and commits four files earlier phases wrote and did not carry.

| Landed here | What |
|-------------|------|
| AC-1 repointed at the product | `TestTemplComponentTypeSafety` renames `LayoutData.Title` in `internal/component/web/render.go` and loads package `web` through a `go/packages` overlay. It requires a type error POSITIONED in a `*_templ.go` and naming `Title`. The two-file fixture under `internal/component/web/testdata/` is deleted |
| The `ze:sensitive` marking phase 3d wrote | `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang`. Its sibling module was committed and this one was not, so `TestAWriteOnlyPasswordLeafIsMarkedSensitive` was red at HEAD |
| The doc anchors phase 5 repointed | `docs/guide/web-interface.md`, four anchors naming deleted `templates` files. `make ze-validate` reported all four |
| The pattern doc phase 5 rewrote | `ai/patterns/web-endpoint.md`, the component hierarchy and the two checklist lines |
| 28 spec citations restated | Every tracked file citing this spec by full path now writes the bare stem. Check 5 of `scripts/dev/check_doc_links.py` reads EVERY tracked file, so commit B would have stranded 28 dead path references |

### Bugs Found/Fixed

- **AC-1 was proven over a stand-in.** The only test for it renamed a field in a
  fixture package and never touched a shipped component. Fixed, and journaled.
- **Four files of this spec were never committed.** HEAD was red for
  `TestAWriteOnlyPasswordLeafIsMarkedSensitive` and for four source anchors.
  Fixed by committing them, and journaled: no gate reads the tracked test
  population.
- **`NormalizeHTML` was exported with no caller in any other package.** Not even
  a `_test.go` one, so `check_cross_package_wiring` (`scripts/dev/validate.py`)
  fired on anyone who edited the file. It is `normalizeHTML` now, and the
  exemption entry in `scripts/dev/verify_wiring_docs.py` is deleted rather than
  kept. `AssertPortFidelity` is the entry point the docs name.
- **Two comments named a template the port deleted.** `internal/test/golden/normalize.go`
  and its test cited the pre-port commit template. Both name `configCommit`
  (`internal/component/web/config_commit.templ`) now, which is what carries the
  `{ "\n" }` the comment is about.
- **Two journal rows of this spec carried dead paths.** Rows 9 and 10 of
  `plan/journal/closure-deletes-a-cited-document.md` record a deletion, so the
  paths they name are the point. Each row carries a `doc-links: ignore` marker
  with that reason, and both still parse into five cells.
- **A dead test citation and its baseline pair.** This spec named an `lg` layout
  test file that does not exist. The tests live in
  `internal/component/lg/graph_test.go`. The row is corrected and the
  grandfathering pair is removed from `scripts/dev/doc_citation_baseline.txt`.

### Documentation Updates

| File | What changed | Anchor |
|------|--------------|--------|
| `docs/guide/web-interface.md` | four source anchors repointed from deleted templates to their components | each anchor names the `.templ` that replaced it |
| `ai/patterns/web-endpoint.md` | the template hierarchy became the component hierarchy, and the checklist names `fieldInputs` | prose, no anchor |
| `ai/INDEX.md`, `docs/contributing/testing.md`, `Makefile` | the port-check description names `golden.AssertPortFidelity`, the exported entry point | prose |
| `docs/architecture/web-interface.md`, `docs/architecture/web-components.md` | already committed by the port; every anchor resolves | `python3 scripts/dev/code_to_docs.py --check` reports all references valid |

`ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` are regenerated and committed.
They are generated from the isolated clone, so they describe exactly the tree
these two commits produce. That matters: at HEAD `ai/CODE-TO-DOCS.md` still
listed six deleted template files, and `scripts/dev/check_doc_links.py` reported
each one. It reports ONE reference after this closure, and that one is this
spec's own line 416 naming the deleted `lg` template directory. Commit B deletes
the file that holds it.

Regenerating them in this checkout produces a different answer, because the
generators read another session's uncommitted sources. That is why the commit
script passes `--stale-index-ok`: the committed index is right for the committed
tree and cannot match a working tree holding 700 foreign edits.

### Deviations from Plan

- **A-3 broke and Thomas resolved it.** Byte identity is unreachable, so AC-2 is
  a semantic comparison. Recorded in full under A-3 and AC-2.
- **Phase 4's type-safety half ran at closure, not in phase 4.** Its doc half
  landed with the port commits.
- **Evidence was gathered in an isolated clone of HEAD.** This checkout does not
  build: another session is mid-rename of `ctx.ProcessName` to `ctx.Sender`
  (`internal/component/plugin/server/command.go`), which breaks every consumer in
  `internal/component/bgp/plugins/cmd/`, and `internal/component/web` imports that
  package transitively. The clone holds HEAD plus this closure's files and nothing
  else, so every result below is attributable.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed a faithful port keeps every byte | templ normalizes its own output and no flag turns that off | measured on a ported layout template in phase 2 | AC-2 became a semantic comparison Thomas approved, and `make ze-templ-port-check` makes it runnable |
| approach | phase 1 proved AC-1 over a fixture, expecting phase 4 to repoint it | the port landed and the repoint did not, so AC-1's only proof never read a shipped component | this closure re-read the deliverable row against the test | repointed at package `web`, fixture deleted, journal row written |
| escalation | four files were left uncommitted across three phases | the working tree was green and HEAD was red for every other session | a clean export of HEAD failed a test the working tree passed | committed here, and journaled against `make ze-tracked-build-check`, whose population excludes every test-only input |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A view-model and markup mismatch is a compile error | Done | `internal/component/web/templ_typesafety_test.go` | proven over `page_layout.templ` and `LayoutData.Title` |
| The silent empty-HTML paths are gone | Done | `internal/component/web/render.go` | `NewRenderer` holds no template. The one remaining empty return logs at Warn and names the component |
| `lg` stops passing `map[string]any` | Done | `internal/component/lg/view.go` | `TestLGViewDataIsTyped` refuses a map parameter |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTemplComponentTypeSafety` | renames a real view-model field and requires the error inside a generated component |
| AC-2 | Done | `make ze-templ-port-check REF=80f0b8b57` | green over `web` and `lg`; `make ze-web-golden-check` green |
| AC-3 | Done | `make ze-templ-generate-check` | exit 0 |
| AC-4 | Done | `assertPackageParsesNoTemplate` (`internal/component/web/render_test.go`) | the surviving `html/template` imports are the `template.HTML` TYPE, which the walk allows and the Deliverables grep could not tell apart |
| AC-5 | Done | `TestDecorationHTMLEscaped`, `TestNoRenderPathEmitsAStoredSecret` | nine render paths |
| AC-6 | Done | `TestTemplatesAvoidInlineScriptAndStyle` | walks `.templ` and carries a file floor, so a vacuous walk reds it |
| AC-7 | Done | `TestNoGoFileBuildsMarkup` in both packages | `web` exempts nothing; `lg` exempts the two drawing builders by rule |
| AC-8 | Done | `TestLGViewDataIsTyped`, `TestWebViewDataIsTyped` | `internal/test/templcheck` is the one guard both call |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTemplComponentTypeSafety` | Changed | `internal/component/web/templ_typesafety_test.go` | repointed from the fixture to package `web` |
| `TestNewRenderer`, `TestRenderLayout`, `TestRenderLogin`, `TestRenderFieldResolvesDecoration`, `TestDecorationHTMLEscaped` | Done | `internal/component/web/render_test.go` | green in the package run |
| `TestRenderSVG`, `TestRenderSVGWithNames` | Done | `internal/component/lg/graph_test.go` | green |
| `TestLGViewDataIsTyped` | Done | `internal/component/lg/view_test.go` | green |
| `TestWebTemplPortFidelity`, `TestLGTemplPortFidelity` | Done | `internal/component/web/port_check_test.go`, `internal/component/lg/port_check_test.go` | green against `80f0b8b57` |
| golden captures | Done | `internal/component/web/golden_test.go`, `internal/component/lg/golden_test.go` | `make ze-web-golden-check` green |
| the seven `.ci` | Done | `test/ui/`, `test/plugin/`, `test/ospf/` | `ze-ui-test` 169/169, `ze-web-test` 87/87 |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `tools.go`, `go.mod`, `go.sum`, `vendor/`, `Makefile`, `.gomuignore` | Done | phase 1 |
| the `lg` components, `internal/component/lg/view.go`, `render.go` | Done | phase 2; the `lg` template directory is deleted |
| the `web` components, `internal/component/web/render.go`, `fragment.go`, `sse.go` | Done | phases 3, 3b, 5 |
| `cmd/ze/hub/service_web.go` | Done | all four renderer entry points ported |
| `internal/test/golden/`, `internal/test/templcheck/`, `internal/test/markupcheck/` | Done | the AC-2, AC-8 and AC-7 instruments |
| the three web architecture docs | Done | phase 4 doc half, committed with the port |
| the type-safety fixture under `internal/component/web/testdata/` | Changed | deleted here; the test reads the real package instead |

### Audit Summary
- **Total items:** 24
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the AC-1 test target and its fixture; both in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A view-model and markup mismatch is a compile error, not a blank panel | functional over the real package | `TestTemplComponentTypeSafety`: the clean load of package `web` reports no error, and the same load with `LayoutData.Title` renamed reports one inside `page_layout_templ.go` naming `Title`. Markup that stops reading the field fails the test by its own message |
| No operator-visible byte changed | functional, against the pre-port bytes | `make ze-templ-port-check REF=80f0b8b57` exit 0 over both packages. It reads the fixtures at that ref with `git archive` and compares under the five AC-2 normalizations. `make ze-web-golden-check` exit 0 proves the fixtures ARE the current render |
| The whole web surface still works through the daemon | functional `.ci` and `.wb` | `make ze-web-test` 87/87, `make ze-ui-test` 169/169 with 10 skipped, `make ze-parse-test` 275/275 with 35 skipped |
| No engine coexists | unit | `assertPackageParsesNoTemplate` walks both packages for a parse call and finds none. `NewRenderer` holds no template |
| No secret reaches the client | unit over nine render paths | `TestNoRenderPathEmitsAStoredSecret` and `TestSecretMaskingFollowsTheSchemaMarking`, plus `TestAWriteOnlyPasswordLeafIsMarkedSensitive`, which walks every YANG module and is what the committed marking answers |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The metadata carries no shard and `plan/deferrals/` holds no file for this stem. Nothing was deferred |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/web-templ-migration-bd3cb1c5-21a8-4146-a390-5190a37971e8.md`, 24 code files |
| `review_gate.py check` | OK, clean, hashes match |
| Rounds | 3 |
| Reviewer lenses used | round 1 wiring and documentation drift over the whole uncommitted state; round 2 removed-behaviour and gate population over the round-1 fixes; round 3 the same over the round-2 fixes |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | four source anchors named templates the port deleted, and the repair was written but never committed | `docs/guide/web-interface.md` | committing the file; `make ze-validate` no longer reports them |
| 2 | BLOCKER | AC-1's only proof ran over a fixture package, so no shipped component was ever type-checked by a test | `internal/component/web/templ_typesafety_test.go` | repointed at package `web`; fixture deleted; journal row |
| 3 | BLOCKER | commit B would strand 28 full-path citations of this spec, and check 5 of the doc-link gate reads every tracked file | 20 files, `Makefile` and `test/relax-ceiling.txt` among them | each restated as the bare stem |
| 4 | ISSUE | `TestAWriteOnlyPasswordLeafIsMarkedSensitive` was red at HEAD because one of the two YANG modules was never committed | `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang` | committed; proven by a clean export of HEAD failing without it |
| 5 | ISSUE | an exported symbol with no caller in any other package, so the wiring check fires on anyone who edits the file | `internal/test/golden/normalize.go` | unexported; the exemption entry deleted rather than kept |
| 6 | NOTE | two comments cited a template the port deleted | `internal/test/golden/normalize.go`, `internal/test/golden/normalize_test.go` | both name `configCommit` now |
| 7 | NOTE | two journal rows of this spec carried dead paths a gate reports | `plan/journal/closure-deletes-a-cited-document.md` | `doc-links: ignore` with the reason; both rows still parse into five cells |
| 8 | NOTE | `ExemptionDrift` reads as unwired, and it is not: its caller is `package markupcheck_test`, in the SAME directory, and `_has_cross_pkg_ref` (`scripts/dev/validate.py`) skips a same-directory file before the test-caller inversion can count it | `internal/test/markupcheck/markupcheck.go` | NOT fixed. The symbol is correctly exported and the gate cannot see the caller. `changed_files` reads the working tree against HEAD, so the finding exists only while the file is uncommitted and lands on nobody. Journaled against the same population class |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/golden/normalize.go` | yes | `ls -1` returns it, 14K |
| `internal/test/golden/portcheck.go` | yes | `ls -1` returns it, 12K |
| `internal/test/templcheck/templcheck.go` | yes | `ls -1` returns it, 11K |
| `internal/test/markupcheck/markupcheck.go` | yes | `ls -1` returns it, 13K |
| `internal/component/web/field_input.go`, `internal/component/web/secret.go` | yes | `ls -1` returns both |
| `internal/component/config/mask.go` | yes | `ls -1` returns it, 12K |
| `internal/component/lg/view.go` | yes | `ls -1` returns it |
| `internal/component/web/assets/snapshot-live.js`, `internal/component/lg/assets/graph-mode.js` | yes | `ls -1` returns both |
| the `.templ` sources | yes | 58 under `internal/component/web`, 9 under `internal/component/lg` |
| the seven `.ci` and `test/parse/bcrypt-placeholder-rejected.ci` | yes | `ls -1` returns all eight |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a renamed view-model field fails the build | `TestTemplComponentTypeSafety` passes in the `ze-test-pkg` run of `internal/component/web`, which was `ok` at 266.594s |
| AC-2 | no operator-visible byte moved | `make ze-templ-port-check REF=80f0b8b57` exit 0; `make ze-web-golden-check` exit 0 |
| AC-3 | generated output is in sync | `make ze-templ-generate-check` exit 0, `updates=0` |
| AC-4 | no template parse call in either package | `ze-test-pkg` on both packages green, which runs `assertPackageParsesNoTemplate` |
| AC-5 | a value is escaped exactly once | same package run; `TestNoRenderPathEmitsAStoredSecret` covers nine paths |
| AC-6 | no inline script or style in the ported set | same package run, over `.templ` with a file floor |
| AC-7 | no Go file builds markup | `TestNoGoFileBuildsMarkup` green in `web` and in `lg` |
| AC-8 | no `map[string]any` reaches a component | `TestLGViewDataIsTyped` and `TestWebViewDataIsTyped` green |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| HTMX POST deleting a list entry | `test/ui/web-delete-list-entry.ci` | yes, `ze-ui-test` 169/169 |
| HTMX POST committing config | `test/ui/web-commit-transactional.ci` | yes, same run |
| a rejected commit reported back | `test/ui/web-commit-reject.ci` | yes, same run |
| login and session | `test/plugin/web-auth.ci`, `test/plugin/web-startup.ci` | file read: both drive the daemon over HTTP and assert the login path |
| a looking-glass page with a bad filter | `test/plugin/lg-ui-pages.ci` | file read: it asserts the error banner element and the message inside it |
| a masked secret refused on upload | `test/parse/bcrypt-placeholder-rejected.ci` | yes, `ze-parse-test` 275/275 |
| a `.templ` edited and not regenerated | `make ze-templ-generate-check` | yes, exit 0 |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `du -sk vendor` moved 46688K to 51672K, a 4984K delta against an 8M stop condition |
| A-2 | confirmed | phase 3b found five DOUBLE escapes and no under-escape; all five are deleted |
| A-3 | broken | templ normalizes its own output. Thomas resolved it: keep templ and prove the port semantically. AC-2 carries the five normalizations and `make ze-templ-port-check` runs them |
| A-4 | confirmed | `fieldInputs` is a map from a field type to a component constructor, and `producedFieldTypes` derives the nine types a leaf can reach |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| every source anchor resolves to a file and a symbol | `python3 scripts/dev/code_to_docs.py --check` reports 2077 code paths and all references valid | yes |
| no doc names a deleted template | `make ze-validate` reports no anchor of this spec; the two it still reports name `delivery_graph.go`, which belongs to another session | yes |
| the component hierarchy a new endpoint follows | `ai/patterns/web-endpoint.md` lists `page_*`, `component_*` and `input_*` files that exist | yes |
| the port-check description names an exported symbol | `ai/INDEX.md`, `docs/contributing/testing.md` and `Makefile` name `golden.AssertPortFidelity`, which `gopls symbols` finds in `internal/test/golden/portcheck.go` | yes |
| categories answered No | rows 1 to 11 and 13 to 15 of the Documentation Update Checklist: no config, CLI, API, plugin, wire or RFC surface changed, and `RegisterWebRoute` keeps its signature | yes |

## Core Insight

A proof built before the thing it proves exists will prove the scaffold unless
somebody repoints it. Phase 1 was right to build the AC-1 mechanism against a
fixture, because no component was ported yet, and the deliverable row even said
phase 4 must repoint it. The port landed, the phases that followed each found
real defects, and the one test that was supposed to guarantee the whole engine
change kept passing over two files nobody ships. The scaffold is not the risk.
Forgetting that a green test is aimed somewhere is.
