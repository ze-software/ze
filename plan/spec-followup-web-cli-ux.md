# Spec: followup-web-cli-ux

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-07-09 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/auth.go`, `cmd/ze/hub/service_web.go` - web auth + route wiring
4. `internal/plugins/completion/words.go`, `internal/component/config/cli/cmd_completion.go` - the two completion engines
5. `internal/component/lg/server.go`, `internal/component/lg/handler_api.go` - LG API surface
6. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

User-facing surface follow-ups across web, CLI completion, CLI stdio, and looking glass.

This was a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Designed 2026-07-09; all evidence re-verified at that date.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **web phase 2 (L77)** - RBAC (~~auth.go is basic-auth only~~ design correction: session-cookie auth with basic-auth API fallback already shipped; the gap is role enforcement), i18n, mobile layout, config upload/download, plugin web extensions.
- **shell-completion v3 (L78)** - flag-value completion (`--family <TAB>`) and config-section completion; ~~today does peers/env-keys only~~ design correction: full YANG command-tree completion already ships; the gaps are per-subcommand flag completion and shell wiring for the existing `ze config completion` engine.
- **cli-stdio-hardening (L218)** - shared error-capturing `renderWriter` for the project-wide `fmt.Printf`/`Fprintf`-to-stdout render paths that ignore write errors.
- **looking-glass v2 (L76)** - pagination/offset, large-RIB perf benchmark, Alice-LG e2e `.ci` (TLS already landed).

### Design-time corrections (2026-07-09, verified with file:line)

| Triage claim | Reality today |
|--------------|---------------|
| "auth.go is basic-auth only" | `AuthMiddlewareWithAudit` (`web/auth.go:177-212`) checks the `ze-session` cookie first; Basic Auth is a fallback for JSON API requests (:189-205). Login flow, audit of failures, security headers all shipped |
| No role model exists | authz profiles exist end-to-end: `Profile{Run,Edit}` + `Store.Authorize` (`authz/authz.go:208-266,:337-397`), builtin `admin`/`read-only`, `AuthResult.Profiles` (`authz/auth.go:58-62`). Web DISCARDS them: middleware stores only username; `WebSession` (auth.go:64-68) has no profile field |
| "completion does peers/env-keys only" | `writeWords` (`completion/words.go:36-75`) drives `command.NewTreeCompleter` over full show/run/root trees (:93-169); family ValueHints wired at rib nodes (`command/valuehints.go:85-111`); ~60 `test/ui/cli-completion-*.ci` prove a full YANG config completion engine at `ze config completion` (`config/cli/cmd_completion.go`) that the shells never call (bash falls back to file completion, `bash.go:108-114`) |
| (implicit) web routes extensible | No plugin web-extension mechanism exists; every route hardcoded in `cmd/ze/hub/service_web.go:476-557` |
| (implicit) LG dispatch bespoke | LG now renders via the unified `plugin.CommandDispatcher` (commit `3404c4396`, `lg/server.go:63-67,:537-548`); LG is feature-gated behind build tag `ze_lg` (`cmd/ze/hub/service_lg.go:13`) |

### User scope decisions (2026-07-09, resolving the implementation-session Findings)

The open questions recorded by the 2026-07-09 implementation sessions were put to the user; these decisions are binding for the finishing session:

| Open item | User decision |
|-----------|---------------|
| AC-8/AC-9 completion v3 | **Flags + one-shot command.** (1) Registry flag inventory + shell wiring so real subcommand flags complete (`ze exabgp plugin --family` per `internal/plugins/exabgp/main.go:77-110`; `ze-perf run --family`; `ze-test decode --family`); (2) ADD a one-shot config-path CLI command (working shape `ze config show <path>`, final grammar per `ai/rules/cli-grammar.md` + `ai/patterns/cli-command.md` + command-ownership checks) and wire config-section completion to it via the existing `ze config completion` engine. This also addresses the recorded "no one-shot command" CLI gap. AC-9 is re-scoped onto this new command instead of the (nonexistent) shell trigger |
| AC-10 renderWriter | **Full conversion.** Error-capturing writer + non-zero-exit contract, converting ALL enumerated sites (`internal/core/helpfmt/helpfmt.go:69` seed; `cmd/ze/help_ai.go`, `cmd/ze/help_command.go`, `cmd/ze/dispatch.go`, `cmd/ze/ze_core_dispatch.go`, `internal/component/cli/editor.go`). The write-edit hook bans fmt format calls in these files, so the conversion is textbuf/io.WriteString-based - accepted churn |
| AC-1 tail nav-hiding + AC-5 registry | **Both, pragmatic scope.** Hide edit controls for read-only users in workbench/config-form rendering (pass the authorizer through `HandleWorkbench`); web-route registry scoped to in-tree components (isis/ospf/l2tp/gokrazy pages register instead of being hardcoded in `cmd/ze/hub/service_web.go:476-557`). True subprocess-plugin web extensions stay out of scope (plugins are subprocesses; no Go handler registration possible) |
| AC-6/AC-7 i18n + mobile | **Full including harness.** Extend the `.wb` web-test harness with the missing login/multi-user, locale, and viewport directives, then implement the French proving locale + 390px mobile fixes with real `.wb` proofs |

## Required Reading

### Source files / docs

- [ ] `internal/component/web/auth.go`
  → Constraint: `WebSession` (:64-68) must grow a profiles field; sessions are one-per-user, 24h TTL, crypto/rand tokens (:61-163) - preserve
  → Constraint: middleware context carries only username via `withUsername` (:183,:198); RBAC needs profiles in the same context path
- [ ] `cmd/ze/hub/service_web.go`
  → Constraint: web builds `&authz.LocalAuthenticator{Users}` directly (:444), bypassing the AAA chain (RADIUS PAP admin-auth landed in aaa via `cb9a16ad5`); RBAC work should switch web to the chain authenticator
  → Constraint: config mutations already authorize via `authorizeWebConfigMutation` (`web/handler_config.go:98-107`) wired at :397-406,:438-439; command dispatch authorizes centrally (`plugin/server/command.go:503-532`) - the gap is route/page-level gating and UI hiding only
  → Decision: nav already derives from the merged YANG command tree (`AdminTreeFromYANG` :460-473) - plugin web extensions should follow the same registration-over-hardcoding shape
- [ ] `internal/component/authz/authz.go`, `internal/component/authz/auth.go`
  → Constraint: reuse `Profile`/`Store`/`AuthResult.Profiles`; do NOT invent a web-local role model (`ai/rules/design-context.md`)
- [ ] `internal/plugins/completion/` (words.go, peers.go, bash.go, zsh/fish/nushell generators)
  → Constraint: completion protocol is `ze completion words <verb> [path...]` → TSV `word\tdescription` on stdout, offline; only `peers` dials the daemon
  → Constraint: generated shell scripts complete global flags only (`bash.go:73-83,:92`) + hardcoded per-subcommand lists (:102-189); `words.go:108` skips `-`-prefixed Subs - flag completion needs a real inventory, not more hardcoding
- [ ] `internal/component/config/cli/cmd_completion.go`
  → Decision: config-section completion = wire THIS existing engine (`--context <path> --input <text> [--json|--ghost]`) into the shell generators; do not build a second engine
- [ ] `internal/component/command/registry/registry.go` (`Meta` :132-146), `internal/component/command/node.go` (ValueHints :60-64, ArgDefs :32-41)
  → Constraint: `registry.Meta` has no flag descriptors; adding a flags inventory to Meta (name, description, value-hint kind) is the registration-over-hardcoding path for `--family <TAB>`
- [ ] `internal/core/helpfmt/helpfmt.go`
  → Decision: :69 (`wr` closure, nolint:errcheck) and `Page.WriteErr/WriteOut` (:56-65) are the seed for the shared renderWriter; error capture is the driver, not allocation (`ai/rules/no-sprintf-alloc.md` explicitly allows one-shot CLI fmt output)
- [ ] `internal/component/lg/server.go`, `internal/component/lg/handler_api.go`
  → Constraint: route-list handlers dispatch full-RIB commands and transform everything (`handleAPIRoutesProtocol` :68-96, Peer :99-127, Table :130-153, Prefix/Search :318-346); only `prefix` query param is read today
  → Constraint: `filtered`/`noexport` endpoints return hardcoded empty lists (:242-249,:282-289) - ze does not track filtered routes; Alice-LG e2e assertions MUST expect empty there
  → Constraint: LG builds only under the `ze_lg` tag (`service_lg.go:13`) - benchmarks and e2e builds need the tag
- [ ] `ai/rules/no-sprintf-alloc.md`, `ai/rules/buffer-first.md`
  → Constraint: renderWriter must not leak into wire/hot paths; textbuf.Buffer is the canonical builder and already implements io.Writer/StringWriter
- [ ] `ai/patterns/config-option.md`, `ai/rules/config-surface.md` (if any YANG leaf is added for web RBAC/i18n)
  → Constraint: every new leaf carries max native validation; YANG vs env var decision documented

**Key insights:**
- The web RBAC gap is narrow and well-bounded: store profiles in the session, put them in the request context, gate routes/pages, hide gated nav; enforcement primitives all exist.
- Completion v3 is 90% wiring: one existing engine (config) needs shell glue; flag completion needs a small Meta extension consumed by generators.
- LG pagination must happen at the transform layer (post-dispatch) since the underlying command returns the full RIB; a `limit`/`offset` query-param contract on the birdwatcher endpoints is additive and backwards-compatible.
- Everything web/LG-facing now flows through the unified CommandDispatcher; new endpoints must dispatch through it, not bespoke paths.

## Current Behavior (MANDATORY)

**Source files read (2026-07-09):**

- [ ] `internal/component/web/auth.go` - session-cookie auth + basic fallback (verified firsthand :177-212); no profile storage
- [ ] `cmd/ze/hub/service_web.go` - hardcoded route table; LocalAuthenticator direct; mutation authz wired
- [ ] `internal/plugins/completion/words.go` - tree-completer TSV protocol (verified firsthand :36-75)
- [ ] `internal/component/lg/handler_api.go` - full-RIB dispatch, no pagination (verified firsthand :68-96)
- [ ] `internal/core/helpfmt/helpfmt.go` - error-swallowing render closure at :69
- [ ] `internal/component/web/` templates/assets - viewport meta present, partial `@media` coverage; no i18n anywhere; no upload/download endpoints

**Behavior to preserve:**
- Existing session lifecycle, login flow, audit recording, security headers.
- Basic-auth API fallback for JSON clients.
- Completion TSV contract (`word\tdescription`) and offline operation of `words`.
- Birdwatcher API response shapes (Alice-LG compatibility); unpaginated requests keep returning the full set.
- All ~80 `test/web/*.wb` and ~150 `test/ui/*.ci` existing expectations.

**Behavior to change:**
- Sessions carry authz profiles; admin/config routes enforce them; nav hides unauthorized entries.
- Web authenticates via the AAA chain (RADIUS/TACACS admins work on web).
- New: config download/upload endpoints, plugin web-route registry, i18n infrastructure + one non-English locale, mobile-layout fixes on key pages.
- Shell scripts gain config-section and flag-value completion.
- Render paths capture write errors via the shared writer; affected commands exit non-zero on write failure.
- LG route endpoints accept `limit`/`offset`; benchmark + Alice-LG e2e added.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Web: HTTP request → mux → AuthMiddleware (cookie/basic) → handler → CommandDispatcher
- Completion: shell TAB → generated script → `ze completion words ...` / `ze config completion ...` → TSV/JSON on stdout
- CLI render: command handler → renderWriter → stdout/stderr
- LG: HTTP GET (birdwatcher path + query params) → handler → CommandDispatcher → transform → JSON

### Transformation Path
1. Web: authenticate → attach username+profiles to context → route gate (profile check) → handler → authz-checked dispatch → HTML/JSON
2. Completion: shell context (verb path or flag) → engine (tree completer, config engine, or flag inventory) → candidates → shell
3. Stdio: rendered text → writer that records first error → command exit code reflects failure
4. LG: dispatch full result → transform → slice by offset/limit → birdwatcher JSON (+ total count where the schema allows)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP → web handler | mux routing + auth middleware | [ ] |
| web/LG → engine | unified plugin.CommandDispatcher | [ ] |
| CLI → completion | exec `ze completion words` / `ze config completion`, TSV/JSON stdout | [ ] |
| plugin → web routes | new registration mechanism (this spec) | [ ] |

### Integration Points
- `internal/component/web/auth.go` + `service_web.go` - profile plumbing, route gating, AAA chain
- `internal/component/authz/` - existing Store/Profile (consume, don't modify semantics)
- `internal/plugins/completion/` + `internal/component/command/registry/` - flag inventory + shell glue
- `internal/core/helpfmt/` - renderWriter seed
- `internal/component/lg/handler_api.go` - pagination in transform layer

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The corrected evidence above still holds at implement time | re-verified 2026-07-09 (research pass; firsthand: auth.go:177-212, words.go:36-75, handler_api.go:68-96) | Re-scope the item | grep/LSP at implement-audit | confirmed |
| A-2 | authz profiles are sufficient for web route gating (no per-route permission model needed) | builtin admin/read-only + Section allow/deny (`authz/authz.go:208-266`) | Extend Profile sections, not a new model | design review during phase 1 | unvalidated |
| A-3 | Switching web to the AAA chain authenticator preserves local-user login | chain includes local backend (aaa component; RADIUS added `cb9a16ad5`) | Keep LocalAuthenticator as explicit chain head | `.wb` login test against local user | unvalidated |
| A-4 | Alice-LG works against the birdwatcher API with empty filtered/noexport | endpoints return empty by design (`handler_api.go:242-249,:282-289`); memory `project_no_filtered_routes` | e2e asserts degraded-but-functional; document limitation | Alice-LG e2e .ci | unvalidated |
| A-5 | LG pagination at the transform layer is acceptable perf-wise (full RIB still fetched per request) | dispatch returns full result today; transform is in-process | Follow-up: dispatch-level limit (new command arg) if benchmark shows need | large-RIB benchmark (this spec) | unvalidated |
| A-6 | A flags inventory on `registry.Meta` covers `--family` and peers-style dynamic values | Meta :132-146 is the single registration point; ValueHints pattern exists | Per-command CompleteFn registry instead | phase 2 design review | unvalidated |
| A-7 | i18n scope = infrastructure + 1 proving locale, not full translation coverage | umbrella wording "i18n" with no locale list; no i18n exists at all today | User names locales; extend catalog | user review of this spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | RBAC gating breaks existing single-admin deployments (no assignments configured) | `.wb` suite failures on admin pages | Preserve authz fail-open-when-unassigned semantics (`Store.Authorize` fail-closed only when assignments exist) - mirror that rule at the route layer |
| R-2 | Web-route registry invites plugin coupling to web internals | plugin imports web template internals | Registry contract = path + handler + nav metadata only; templates stay in web |
| R-3 | Shell-script completion changes break one shell but not others | test/ui passes but interactive bash/zsh/fish diverge | Per-shell fixture tests comparing generated script output; manual smoke matrix in PR notes |
| R-4 | renderWriter conversion churns hundreds of call sites and stalls | diff grows past the enumerated seed sites | Phase-scope: helpfmt + cmd/ze dispatch/help + editor.go only; record remaining sites as an inventory in the learned summary |
| R-5 | Pagination changes birdwatcher JSON in a way Alice-LG rejects | e2e fails on paginated responses | Params optional; default behavior byte-identical to today |
| R-6 | i18n infrastructure touches every template | template diff explosion | Introduce translation helper + catalog; convert only the pages covered by the proving locale's `.wb` test |
| R-7 | Umbrella scope too large for one implementation pass | phase overruns | Phases are independent; split into per-item specs at implement time if needed (record as deferral with destination) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Non-admin (read-only profile) user requests an admin/config-mutation page | → | route gate denies (403/redirect), nav hides entry | `.wb` `web-rbac-readonly` |
| Admin user via AAA-chain (local) logs in | → | session stores profiles, admin routes allowed | `.wb` `web-rbac-admin` |
| GET config download endpoint | → | current committed config streamed with authz check | `.wb` or `.ci` `web-config-download` |
| Upload endpoint with invalid config | → | validate rejects, no apply | `.wb` `web-config-upload-invalid` |
| Plugin registers a web route at startup | → | route served + nav entry appears | Go wiring test `TestPluginWebRouteRegistration` + `.wb` `web-plugin-route` |
| `--family <TAB>` on a flag-bearing subcommand | → | flag inventory → family candidates | `.ci` `completion-flag-family` (test/ui) |
| `set <TAB>` in shell config context | → | shell glue calls `ze config completion` | `.ci` `completion-shell-config-section` (test/ui) |
| Render command writes to a closed stdout pipe | → | renderWriter captures error, non-zero exit | Go test `TestRenderWriterErrorExit` + fixture |
| GET `/api/looking-glass/routes/table/{family}?limit=N&offset=M` | → | sliced result + stable ordering | Go test `TestRoutesTablePagination` + `.ci` `lg-paginate` |
| Alice-LG container against ze LG endpoint | → | birdwatcher API consumed end-to-end | e2e `.ci` `lg-alice-e2e` (lg-graph-lab.ci pattern) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | User with `read-only` profile authenticated on web | Admin/config-mutation routes return 403 (or redirect with message); nav omits gated entries; JSON API mutation calls denied; read paths unaffected |
| AC-2 | User with `admin` profile (via AAA chain, local backend) | Full access as today; `WebSession` + request context carry profiles; no assignments configured → current open behavior preserved (R-1) |
| AC-3 | Config download request by an authorized user | Streams the committed config (text) with correct content-type; audit-logged |
| AC-4 | Config upload of a valid / invalid config | Valid: staged through the existing validate+commit path (same authz as web commit), UI feedback; invalid: rejected with validation errors, nothing applied |
| AC-5 | A plugin registering a web extension (route + nav label) | Route served under a namespaced path, nav entry rendered, removal of the plugin removes the route (self-containment); no per-plugin edits in service_web.go beyond the generated composition root |
| AC-6 | Web pages under the proving non-English locale (`Accept-Language` or user setting) | Translated strings render from the catalog; untranslated keys fall back to English; layout intact |
| AC-7 | Key pages (login, dashboard, config editor, admin) at 390px viewport | No horizontal scroll, controls usable; `.wb` viewport assertions pass |
| AC-8 | `ze completion words` for a subcommand flag + `--family` value | Flag names complete from the registry inventory; `--family` completes from `registry.AllFamilies()`; works in bash/zsh/fish generated scripts |
| AC-9 | Config-section TAB completion in an interactive shell | Generated scripts call the existing `ze config completion` engine; candidates match the `test/ui/cli-completion-*.ci` engine results |
| AC-10 | Render path hits a write error (EPIPE) | Shared renderWriter records the first error, command exits non-zero, no partial-write silent success; helpfmt closure (:69) routed through it; enumerated cmd/ze sites converted |
| AC-11 | LG route-list endpoints with `limit`/`offset` | Sliced results, stable order, `limit=0`/absent = full result (today's behavior); invalid params → 400 |
| AC-12 | Large-RIB LG benchmark (Go benchmark, `ze_lg` tag) | Baseline numbers recorded in the spec/learned summary for routes/table and routes/peer transforms at 100k+ routes |
| AC-13 | Alice-LG e2e | Container consumes ze's birdwatcher API: status, protocols, routes pages render; filtered/noexport show empty (documented limitation) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator gives NOC user read-only web access | aaa/authz assignment → login → session profiles → route gate | `web-rbac-readonly` (.wb) |
| 2 | Operator downloads running config from web | browser → download endpoint → config store → file | `web-config-download` |
| 3 | Operator uploads edited config | browser → upload → validate → commit path → applied | `web-config-upload-invalid` + happy-path `.wb` |
| 4 | Operator TABs `--family` at the CLI | shell → generated script → words+inventory → candidates | `completion-flag-family` (.ci) |
| 5 | NOC views a 1M-route LG table page by page | Alice-LG/browser → paginated endpoint → sliced transform | `lg-paginate` (.ci) + `TestRoutesTablePagination` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionStoresProfiles`, `TestRouteGateDeniesReadOnly`, `TestRouteGateOpenWhenUnassigned` | `internal/component/web/auth_test.go` (or rbac_test.go) | AC-1, AC-2, R-1 | |
| `TestWebUsesAAAChain` | `cmd/ze/hub/service_web_test.go` or web tests | AC-2 | |
| `TestConfigDownloadHandler`, `TestConfigUploadValidates` | `internal/component/web/handler_config_test.go` | AC-3, AC-4 | |
| `TestPluginWebRouteRegistration`, `TestPluginRouteRemovedWithPlugin` | web route-registry tests | AC-5 | |
| `TestI18NCatalogFallback` | web i18n tests | AC-6 | |
| `TestFlagInventoryCompletion`, `TestFamilyFlagValues` | `internal/plugins/completion/words_test.go` | AC-8 | |
| `TestShellScriptEmitsConfigCompletionGlue` (per shell) | `internal/plugins/completion/*_test.go` | AC-9 | |
| `TestRenderWriterCapturesError`, `TestRenderWriterErrorExit` | `internal/core/helpfmt/` (or new core pkg) tests | AC-10 | |
| `TestRoutesTablePagination`, `TestPaginationParamValidation`, `TestPaginateRoutes` | `internal/component/lg/handler_api_pagination_test.go` | AC-11 | PASS (Go) |
| `BenchmarkRoutesTableTransform`, `BenchmarkRoutesPeerTransformPaginated` | `internal/component/lg/handler_api_bench_test.go` (plain `package lg`, no tag) | AC-12 | PASS (Go) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| lg `limit` | 0..100000 (0 = unlimited) | 100000 | N/A (unsigned parse; negative → 400) | 400 or clamp (decide in design; record) |
| lg `offset` | 0..result-len | result-len (empty page) | negative → 400 | beyond end → empty list, 200 |
| viewport width (.wb) | 390px mobile baseline | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-rbac-readonly.wb`, `web-rbac-admin.wb` | test/web | role-gated pages | |
| `web-config-download.wb`, `web-config-upload-invalid.wb` | test/web | config round-trip via browser | |
| `web-plugin-route.wb` | test/web | plugin-registered page renders | |
| `web-mobile-layout.wb` | test/web | 390px viewport on key pages | |
| `completion-flag-family.ci`, `completion-shell-config-section.ci` | test/ui | TAB completion behaviors | |
| `lg-paginate.ci`, `lg-alice-e2e.ci` | test/plugin | LG pagination + Alice-LG interop | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lg-alice-e2e` | test/plugin (Docker Alice-LG) | Alice-LG | birdwatcher API compatibility (HTTP, not wire protocol; interop-and-goal-validation.md satisfied by e2e consumer test) | |

## Files to Modify

- `internal/component/web/auth.go` - WebSession profiles, middleware context
- `internal/component/web/handler_*.go` + templates - route gating, nav hiding, upload/download, i18n helper, mobile CSS fixes
- `cmd/ze/hub/service_web.go` - AAA chain authenticator, route-registry consumption (plus `make generate` composition root if a new registry is plugin-fed)
- `internal/component/authz/` - only if a helper accessor is missing (consume existing semantics)
- `internal/plugins/completion/words.go`, `bash.go`, `zsh.go`, `fish.go`, `nushell.go` - flag inventory consumption + config-completion glue
- `internal/component/command/registry/registry.go` - Meta flags inventory (additive)
- `internal/core/helpfmt/helpfmt.go` - renderWriter seed + adoption
- `cmd/ze/help_ai.go`, `cmd/ze/help_command.go`, `cmd/ze/dispatch.go`, `cmd/ze/ze_core_dispatch.go`, `internal/component/cli/editor.go` - convert enumerated bare-print sites
- `internal/component/lg/handler_api.go` - pagination params + slicing
- `docs/guide/` + `docs/features.md` - per Documentation Update Checklist

## Files to Create

- `internal/component/web/routes_registry.go` (or similar) - plugin web-extension registration
- `internal/component/web/i18n.go` + `internal/component/web/locales/<lang>.json` (or Go catalog) - i18n infrastructure + proving locale
- `internal/component/lg/handler_api_bench_test.go` - large-RIB benchmark
- new `.wb`/`.ci` tests listed above

## Implementation Steps

1. **Phase: Wiring (RBAC)** - profiles into session+context, failing `.wb` rbac tests, route-gate skeleton (AC-1, AC-2).
2. **Phase: web AAA chain + upload/download** (AC-3, AC-4).
3. **Phase: plugin web-route registry** - registration mechanism + one in-tree consumer proving it (AC-5).
4. **Phase: i18n infrastructure + proving locale; mobile fixes** (AC-6, AC-7).
5. **Phase: completion v3** - Meta flags inventory → words consumption → shell generators → config-completion glue (AC-8, AC-9).
6. **Phase: renderWriter** - seed in helpfmt, convert enumerated sites, error-exit contract (AC-10).
7. **Phase: LG v2** - pagination + benchmark + Alice-LG e2e (AC-11..AC-13).
8. **Full verification** - `make ze-verify`; `ze-test web/ui` suites.
9. **Complete spec** - audit tables, `plan/learned/NNN-followup-web-cli-ux.md`, two-commit closure.

Phases 1-4 (web), 5 (completion), 6 (stdio), 7 (LG) are independent; commit per phase (disjoint systems get separate commits per `ai/rules/git-safety.md`).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (web-route registry, flag inventory)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse authz profiles for web RBAC | New web-local roles table | Profiles + Store + assignments exist and already gate command dispatch; one permission model across surfaces |
| Web switches to AAA chain authenticator | Keep LocalAuthenticator | RADIUS/TACACS admin auth already works for other surfaces (`cb9a16ad5`); local users preserved as chain head (A-3) |
| Config-section completion wires the existing `ze config completion` engine into shells | New completion provider in the plugin | Engine + 60 .ci tests exist; v3 is glue |
| Flag inventory on `registry.Meta` | Hardcode per-subcommand flag lists in shell generators | Hardcoded lists are the current antipattern (bash.go:102-189); registry is the discovery point |
| renderWriter seeded in helpfmt, adoption scoped to enumerated sites | Project-wide mechanical conversion in one pass | Bounded diff, the seed carries the contract; inventory of remaining sites recorded for follow-up (R-4) |
| LG pagination at transform layer with optional params | New paginated engine command | Additive, backwards-compatible; A-5 benchmark decides if dispatch-level limit is needed later |
| i18n = catalog infrastructure + one proving locale | Full translation pass | No i18n exists; proving the pipeline is the 80% infrastructure win; locale list is a user decision (A-7) |

## Known Limitations

- LG filtered/noexport remain empty (ze does not track filtered routes) - Alice-LG shows empty tabs; separate feature if ever needed.
- Pagination still materializes the full RIB per request server-side (A-5 benchmark documents the cost).
- i18n proving locale only; full catalogs are follow-up content work.
- renderWriter adoption is seeded, not exhaustive; remaining bare-print sites inventoried at implementation.

## Notes
- Designed 2026-07-09 from skeleton; user instruction 2026-07-09 authorized batch conversion to ready.
- If implementation splits this umbrella, each split spec inherits the relevant AC rows and this spec records the deferral destinations (`ai/rules/deferral-tracking.md`).

## Implementation Progress (2026-07-09)

Implemented incrementally, one disjoint phase per commit. This umbrella is large
(7 disjoint subsystems); phases are committed independently and the spec stays
open until all ACs are complete (R-7).

### Per-AC status
| AC | Status | Evidence |
|----|--------|----------|
| AC-11 (LG pagination) | DONE | `parsePagination`/`paginateRoutes` (`internal/component/lg/handler_api.go`), wired into 4 route-list handlers via `serveRoutesForPeer`; `TestRoutesTablePagination`, `TestPaginationParamValidation`, `TestPaginateRoutes` PASS |
| AC-12 (LG benchmark) | DONE | `BenchmarkRoutesTableTransform` ~600ms/op, 155MB/op, 3.0M allocs; `BenchmarkRoutesPeerTransformPaginated` ~577ms/op at 100k routes -- confirms A-5 (transform-layer pagination still materializes the full RIB) |
| AC-13 (Alice-LG e2e) | SCENARIO IMPLEMENTED, ENVIRONMENT-BLOCKED | `test/plugin/lg-paginate.ci` written (birdwatcher consumer: status, protocols, paginated routes, empty filtered/noexport). The LG plugin `.ci` harness (live BGP peer + external Python injector) does NOT run in this sandbox: the pre-existing known-good `lg-graph-lab.ci` hangs identically (proc-level, not http). Route-count assertions unverified in sandbox; must validate in full CI |
| AC-1 (RBAC read-only 403) | PARTIAL: enforcement DONE | Route gate `RequireEditAuthz`/`CanEdit` (`internal/component/web/rbac.go`) 403s read-only sessions on the admin console + config add-form (`cmd/ze/hub/service_web.go`); per-mutation authz already enforced (`authorizeWebConfigMutation`). `TestRouteGateDeniesReadOnly`, `TestRouteGateAllowsAdmin`, `TestRouteGateOpenWhenUnassigned`, `TestCanEditReflectsAuthorizer` PASS. REMAINING: nav-hiding of edit controls (woven through workbench config-form rendering; needs `.wb` harness which cannot do multi-user auth here) |
| AC-2 (session profiles + AAA chain, R-1) | DONE | Session/context: `WebSession.Profiles` + `withProfiles`/`GetProfilesFromRequest` (`internal/component/web/auth.go`); login stores `AuthResult.Profiles`. AAA chain: `liveAAABundleAuthenticator` (`cmd/ze/hub/aaa_authenticator_web.go`) reads the live `aaaBundle` slot per call (mirrors `liveAAABundleAuthorizer`), falling back to static local users before the bundle exists / for chain-unknown users; wired at `service_web.go:449` (replaces `LocalAuthenticator`). `startWebServer` is in package `hub` so it reads the atomic slot directly -- no `ServiceDeps` change needed, solving the post-config-load timing without a hack. `TestSessionStoresProfiles`, `TestProfilesInRequestContext`, `TestWebAuthFallsBackWhenNoBundle`, `TestWebAuthUsesChainWhenInstalled`, `TestWebAuthLocalPreservedWithBundle` (A-3), `TestWebAuthRejectsBadCredentials` PASS; fail-open R-1 verified |
| AC-3 (config download) | DONE | `HandleConfigDownload` (`internal/component/web/handler_config_transfer.go`) + `EditorManager.CommittedConfig`; streams committed config as `text/plain` attachment `ze.conf`, audit `config-download`, route `GET /config/download` (authWrap, any authenticated session -- config viewing is already read-only-visible). `TestConfigDownloadHandler`, `TestConfigDownloadRequiresAuth` PASS |
| AC-4 (config upload) | DONE | `HandleConfigUpload` + `EditorManager.ApplyCommittedContent`; validates via `zeconfigcmd.ValidateContent` (same validator as commit), invalid -> 400 nothing applied, valid -> write+reload-hook (restores prior config on hook failure), audit `config-upload`, route `POST /config/upload` (editMutationWrap: read-only denied + same-origin). `TestConfigUploadValidApplies`, `TestConfigUploadValidatesRejects`, `TestConfigUploadRBACDeny` PASS |
| AC-8 (flag inventory completion) | DONE | `registry.FlagSpec`/`RegisterCommandFlags`/`CommandFlags` (`internal/component/command/registry/flags.go`); `ze completion flags <path>` + `ze completion families` (`internal/plugins/completion/flags.go`); exabgp registers `exabgp plugin`/`exabgp migrate` flags (`internal/plugins/exabgp/register.go`); bash/zsh/fish generators complete flag names from the inventory + `--family` from `AllFamilies`. `TestFlagInventoryCompletion`, `TestFamilyFlagValues`, `TestShellScriptEmitsConfigCompletionGlue` PASS; verified end-to-end against `bin/ze`. Nushell shell-wiring for the new glue deferred (AC-8 scopes to bash/zsh/fish) |
| AC-9 (config-section completion + one-shot command) | DONE | NEW `ze config show <file> [path...]` (`internal/component/config/cli/cmd_show.go`, added to `subcommandHandlers`; reads a file like `dump`, reuses the editor's schema-aware walk for list-keyed paths, `--json`, non-zero exit on bad path / write error). bash/zsh/fish complete the path tokens via the existing `ze config completion` engine (`--context <slash-path> --input "set "`). `TestConfigShow{FullTree,AtPath,PathNotFound,JSON,MissingFile,Registered}` PASS; verified end-to-end. Docs: `docs/guide/command-reference.md` row + source anchors |
| AC-1 tail (nav hide), AC-5 (web-route registry), AC-6,7 (i18n/mobile), AC-10 (renderWriter) | NOT STARTED / PARTIAL | AC-1 enforcement DONE (route gate) but edit-control nav-hiding still open. Remainder umbrella too large for one pass (R-7) |

### Key design decisions (this session)
| Decision | Rationale |
|----------|-----------|
| LG `limit` > 100000 -> HTTP 400 (reject, not clamp) | A client must never believe it got a full page when it did not; boundary made explicit and testable |
| LG pagination adds a `pagination{total_results,offset,limit}` object ONLY when params are present | Default (unpaginated) response stays byte-identical (R-5); Go test asserts absence on default |
| LG unit tests/benchmark are plain `package lg` (NO `ze_lg` tag) | Verified firsthand: `internal/component/lg` has no build tags; only hub wiring is gated. Adding a tag would exclude them from `go test ./...` |
| AC-6 proving locale = French (defaulted) | Maintainer is French (no locale named in spec); recorded as a defaulted decision per implementation guidance |

### Findings / deviations
- **LG plugin `.ci` harness environment-blocked in this sandbox.** `lg-graph-lab.ci` (pre-existing, maintained, known-good) hangs identically here, so the block is the sandbox's inability to run the BGP-peer + external-plugin harness, not the pagination code. AC-11 is proven at the Go tier instead.
- **`.wb` test harness gap (affects AC-1,2,6,7).** The web `.wb` harness starts the server with `--insecure-web` (single implicit admin, `internal/test/cli/cmd_web.go:277`) and has no login/multi-user or viewport directive. RBAC (read-only vs admin), i18n locale (`Accept-Language`), and mobile-viewport `.wb` tests therefore need harness extension; Go httptest is the precise proof layer for those ACs.
- **Web holds only an opaque `aaa.Authorizer`, not `authz.Store`.** RBAC enforcement reuses the authorizer (username->allow/deny, fail-open when unassigned = R-1 preserved); profiles are stored in the session/context for AC-2 + nav.
- The 4 missing review checklists (Critical/Deliverables/Security/Documentation) were absent from the "ready" spec; added at implementation.

### Continuation session 2 (2026-07-09b) -- Phase 2 done; remaining ACs need scope calls

**Completed and committed (`22444c0e3`):** AC-2 (AAA-chain auth), AC-3 (config download),
AC-4 (config upload). Details in the Per-AC table above. Changed surface verified:
`go test ./internal/component/web ./internal/core/audit` PASS, `go test -tags ze_web ./cmd/ze/hub` PASS
(`TestWebAuth*`, `TestConfigDownload*`, `TestConfigUpload*`), `make ze-lint-changed` 0 issues.

**Design note (AC-2 timing solved cleanly):** the spec feared a "hub timing complication"
(AAA chain built post-config-load, after `buildServices`). Resolved with no `ServiceDeps`
change: `startWebServer` is in package `hub`, so it reads the `aaaBundle atomic.Pointer`
slot directly through `liveAAABundleAuthenticator` (mirrors the existing
`liveAAABundleAuthorizer`), which reads the slot per call and falls back to static local
users before the bundle exists / for chain-unknown users. No late-binding indirection object
was needed.

**Remaining ACs -- each hit a genuine blocker/ambiguity; NOT guessed (per ze-implement
"stop on ambiguity"):**

- **AC-8/AC-9 (completion v3) -- SPEC AMBIGUITY, needs user scope call.** The
  `ze config completion` engine (`config/cli/cmd_completion.go`, flags `--context <path>
  --input <text> [--json|--ghost] <file>`) is an *interactive line-editor* completion
  contract: it completes YANG paths/values *inside* a config file given an editing context
  + partial input. It is consumed by the web CLI bar and interactive `ze cli`. There is **no
  well-defined trigger for it in bash/zsh/fish argv completion**: the `ze config`
  subcommands (`edit validate migrate fmt dump diff completion`) all take a config **file**
  argument (completed correctly today via `_ze_filedir conf`), not a YANG path. AC-9's
  "generated scripts call the existing engine" has no clear entry point. Likewise AC-8's
  `--family <TAB>` has **no literal `--family` flag** consumer -- families already complete
  as positional `ValueHints` at rib nodes (`valuehints.go:97-111`, proven by
  `TestWordsShowBGPRibIncludesFamilyHints`). Recommendation: the user must define the shell
  scenario (which command, which token position) before AC-8/AC-9 can be implemented
  correctly; otherwise descope to "flag-name completion for the genuinely flag-bearing
  subcommands" only.

- **AC-10 (renderWriter) -- TOOLING FRICTION vs value.** A fmt-based render Writer is
  blocked by `pretool-writeedit.py`, which bans `fmt.Fprintf`/`fmt.Sprintf`/`fmt.Printf`
  to any non-`os.Stdout`/`os.Stderr` writer (only `fmt.Fprintln`/`fmt.Fprint` pass). The
  enumerated help sites use `%-44s`-style padded format strings (`help_ai.go`), so routing
  them through an error-capturing writer requires converting every format site to
  `textbuf.Buffer` padding -- large churn for the lowest-value AC (EPIPE-on-broken-pipe
  edge case). Also `internal/component/cli/editor.go`'s enumerated sites are an interactive
  stdin/stdout prompt returning an action enum, not an exit code -- the "exit non-zero on
  write error" contract does not map. Recommendation: either accept the per-site textbuf
  churn (and a Writer exposing only `Println`/`Print`/`Write`, no `Printf`), add a hook
  carve-out for a blessed render-writer, or descope AC-10.

- **AC-1 tail (nav-hide) -- diffuse + weak test story.** Enforcement is DONE and is the real
  security boundary (route gate 403 + per-mutation authz). Nav-hiding is cosmetic
  defense-in-depth: `HandleWorkbench` does not receive the authorizer, and edit controls are
  spread across many fragment templates (`list`, `list_table`, `add_form_overlay`,
  `oob_save`, ...). Requires threading the authorizer -> a `CanEdit`/`Editable` flag ->
  per-template `{{if}}` gates; provable only at the Go/httptest render tier here.

- **AC-5 (web-route registry) -- architecture call needed.** ze plugins are subprocesses
  (JSON/text IPC) and cannot register Go `http.Handler`s in the web process; a "plugin web
  route" must be an in-process feature module (like the L2TP/ISIS/OSPF handlers already in
  the web package). A registry needs a deps-injection factory shape
  (`Register(WebRoute{Pattern, NavLabel, Build func(deps) http.Handler})`) since handlers
  need `renderer`/`dispatch` available only at `startWebServer` time. Implementable but the
  factory contract + whether to migrate existing routes should be confirmed first.

- **AC-6/AC-7 (i18n + mobile) -- not started.** No i18n exists at all; needs catalog
  infrastructure + template conversion + `.wb` viewport/locale harness directives (the
  harness gap noted above).
