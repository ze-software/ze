# Spec: dropped-error-9-api-cli-config

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | 8 remaining errors across 7 files in the command pipe resolver, `config dump`, the looking glass, and the web editor are caught and logged only below WARN: a failed reverse DNS lookup hides a DNS resolver outage behind a debug line, a failed `$9$` secret encode in `config dump` silently substitutes a placeholder behind a debug line instead of returning an operator error, a failed idle web-editor discard hides behind a debug line, and 5 looking-glass HTTP response writes lack guard annotations |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Doors: `ai/doors/system.md`,
`ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md`,
`ai/doors/core.md`, `ai/doors/le-tooling.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under
`internal/component/{api,web,lg,gnmi,mcp,telemetry}`,
`internal/component/{command,cmd,cli,ssh}` and `internal/component/config`
(outside `config/system`, closed in child 8). Reconciled against HEAD on
2026-11-27: **8 unannotated instances in 7 files** (command 1, config CLI 1,
looking glass 5, web 1; 0 annotated):

- `internal/component/command/pipe_resolve.go`: 1 instance (`ReverseLookup` line 60)
- `internal/component/config/cli/cmd_dump.go`: 1 instance (`maskValue` line 104)
- `internal/component/lg/handler_graph.go`: 1 instance (`writeText` line 302)
- `internal/component/lg/handler_ui.go`: 1 instance (`LGServer.handleUIEvents` line 697)
- `internal/component/lg/render.go`: 1 instance (`LGServer.writeHTML` line 192)
- `internal/component/lg/server.go`: 2 instances (`writeJSONError` line 983; `writeSVG` line 1009)
- `internal/component/web/editor.go`: 1 instance (`EditorManager.evictInactive` line 922)

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error (`maskValue` in `config dump`: refuse the dump with an `opererr.Error` instead of silently substituting a placeholder when `$9$` encoding fails, while never emitting the plaintext secret) |
| 2 | Going on is right and an operator can act | Log at `Warn` with what failed, why it matters, and the exact fix (`ai/rules/user-facing-errors.md`), carrying a literal or constant `code` attribute naming a doctor code registered via `diagnostic.Register` and `diagnostic.RegisterEventCode`, and raise a rate-limited event on `internal/core/report` under that same code (`report.RaiseErrorFixOnce`) so `show doctor` and `show errors` surface it without flooding |
| 3 | Going on is right and nobody can act (a public HTTP response write to a disconnected client, or clearing a write deadline unsupported on HTTP/2 or test recorders) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above, naming a visible non-enclosing dependent symbol |

**`doctor-coverage` commit gate requirement:** `./le arch doctor-coverage check`
and `./le commit create` refuse any added or changed `Warn` or `Error` log line
under `internal/component/` that lacks a covered `code` attribute. Every Answer-2
site here (`internal/component/command/pipe_resolve.go` and
`internal/component/web/editor.go`) MUST:

1. Register its code's metadata with `diagnostic.Register` in the owning package
   (`internal/component/command/register_doctor_codes.go` for
   `doctor-resolve-ptr-failed`, and `internal/component/web/doctor_codes.go` for
   `doctor-web-editor-evict-discard-failed`) so `ze show diagnostic <code>`
   describes it.
2. Declare the transient event code from `init()` with
   `diagnostic.RegisterEventCode(diagnostic.EventCode{Code: <Const>})`
   (`internal/component/command/register_doctor_codes.go` and
   `internal/component/web/register_cli_history_diagnostic.go`) so the
   `recent-events` check and `./le arch doctor-coverage check` cover it.
3. Raise the event on the report bus (`report.RaiseErrorFixOnce`, 1-minute window)
   under that exact code and log at `Warn` with `"code", <Const>` and `"fix"`
   naming the operator's exact next command.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts and operator message rules
  → Constraint: every operator-facing message names what failed, why it matters, and the exact fix on every interface
- [ ] `ai/doors/api-surfaces.md` - web and looking glass boundaries
  → Constraint: the surfaces are thin adapters; the LG JSON contract (`docs/architecture/api/birdwatcher-compat.md`) is normative and no response-write annotation changes any envelope or status code
- [ ] `ai/doors/commands-cli.md` - command model and pipe framework
  → Constraint: a handler answers data, never text; `| resolve` enriches rows without altering or aborting the upstream command
- [ ] `ai/doors/config-yang.md` - config syntax, secret masking, and dump producers
  → Constraint: `config dump` never prints an unmasked secret on any path; a masking failure refuses the dump with an operator error rather than printing plaintext or silently substituting a placeholder
- [ ] `ai/doors/core.md` - core diagnostic and report-bus boundaries
  → Constraint: `internal/core/diagnostic` and `internal/core/report` are leaf packages imported downward by components; event codes register from the owning component's `init()`
- [ ] `ai/doors/le-tooling.md` - refuse-by-default and doctor-coverage gates
  → Constraint: product code never imports `internal/le`; scoped package tests run via `./le test unit pkg path <dir>/`
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Event codes"
  → Constraint: every new `Warn` under `internal/component/` names a registered doctor code in its `code` attribute and every site carrying a registered event code raises it on `internal/core/report`
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, exact fix, and declared doctor `code`
- [ ] `docs/contributing/ze-go-style.md` - "Every error is handled"
  → Constraint: return, count, or log at `Warn`; an annotation names a visible dependent guard distinct from the enclosing function

**Key insights:**
- Reading `internal/component/resolve/dns/resolver.go` (`Resolve`, `ResolvePTR`, `query`) shows `ResolvePTR` returns `(nil, nil)` for NXDOMAIN or empty answers and returns a non-nil error ONLY when DNS resolution itself failed (`no DNS server configured`, invalid address, or every configured server timed out / failed transport). Meanwhile `test/plugin/rib-declared-order-refused.ci` runs `show bgp rib | resolve | count | json` expecting `exit=0`, and `internal/component/cli/model_enrich.go` (`enrichAddr`) requires `| resolve | log` to preserve bare addresses when a hop has no PTR. Therefore `ReverseLookup` is **Answer 2** (rate-limited `Warn` + `report.RaiseErrorFixOnce` under `doctor-resolve-ptr-failed`, returning `""`), NOT Answer 1.
- Reading `internal/component/config/cli/cmd_dump.go` (`maskValue`, `maskMapValues`, `printTreeMap`, `resolveDumpBytes`) confirms **Assumption A-2**: today `maskValue` already returns `config.SecretDataPlaceholder` (`/* SECRET-DATA */`) on error, never plaintext `v`, and `secret.Encode` fails only if `crypto/rand.Read` fails. Still, silently substituting the placeholder hides an encoding failure when the operator asked for `$9$` encoding; returning `(string, error)` from `maskValue` (Answer 1) refuses the dump cleanly with an `opererr.Error` while still returning `config.SecretDataPlaceholder` as the string value so plaintext can never leak.
- Reading `internal/component/web/editor.go` (`evictInactive`), `internal/component/cli/editor_commands.go` (`Editor.Discard`), `internal/component/web/recovery_blocked_editor.go` (`recoveryBlockedEditor.Discard`), and `internal/component/web/editor_history.go` (`retireHistory`) shows `evictInactive` must still delete the idle session from `m.sessions` so capacity is freed for new users, and matching `editor_history.go`'s idle retirement pattern, any `Discard()` error should be reported at `Warn` + `report.RaiseErrorFixOnce` under `doctor-web-editor-evict-discard-failed` (Answer 2). Because `*cli.Editor.Discard()` returns `nil`, normal idle eviction stays completely quiet.
- Reading all 5 looking-glass sites (`handler_graph.go:302`, `handler_ui.go:697`, `render.go:192`, `server.go:983`, `server.go:1009`) plus `internal/core/errorfragment/errorfragment.go:217` (`writer.Unwrap`) confirms all 5 are **Answer 3**: four write already-rendered HTTP bodies after headers/status are sent (failing only when the public client disconnected or timed out), and `rc.SetWriteDeadline(time.Time{})` in `handleUIEvents` fails only on HTTP/2 / test recorders (`http.ErrNotSupported`) or already-closed connections, where `writeStreamMessage` at line 729 already terminates the stream on write failure. Warning on public client disconnects would let external clients flood daemon logs and `show doctor`.

## Current Behavior (MANDATORY)

**Source files read (HEAD 2026-11-27):**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `belowWarn`, `drops`
- [ ] `internal/le/arch/refusebydefault/annotation.go` - `parseGuard`, `namesEnclosing`, `visibleNames`, `addPackageNames`
- [ ] `internal/le/arch/doctorcoverage/doctorcoverage.go` - `siteRoots` (`internal/component`, `internal/plugins`, `cmd/ze`), `StatusCovered`
- [ ] `internal/core/diagnostic/event_codes.go` - `EventCode`, `RegisterEventCode`, `describedCode`
- [ ] `internal/core/report/report.go` - `RaiseErrorFixOnce` (returns `bool`, 1-minute ring-backed rate limit)
- [ ] `internal/component/command/pipe_resolve.go` - `ReverseLookup` (line 60), `resolveJSON`, `applyResolve`
- [ ] `internal/component/command/pipe.go` and `pipe_records.go` - `ApplyPipes` (`pipeResolve`), `transformPositionalAddressItem`, `recordsTransformed`
- [ ] `internal/component/cli/model_enrich.go` - `enrichAddr` (`monitor ping | resolve | log`, `monitor traceroute | resolve | log`)
- [ ] `internal/component/resolve/dns/resolver.go` - `Resolve`, `ResolvePTR`, `query`
- [ ] `internal/component/config/cli/cmd_dump.go` - `cmdDump`, `maskMapValues`, `maskValue` (line 104), `printConfig`, `printTreeMap`, `resolveDumpBytes`
- [ ] `internal/component/config/cli/config_data.go`, `config_supplied.go`, `inspect_route.go` - `dataDump`, `suppliedDump`, `dumpAsking`
- [ ] `internal/component/config/secret/secret.go` and `internal/component/config/secret_text.go` - `secret.Encode`, `config.EncodeSecrets`
- [ ] `internal/component/lg/handler_graph.go` - `handleGraph`, `writeEmpty`, `writeText` (line 302)
- [ ] `internal/component/lg/handler_ui.go` - `LGServer.handleUIEvents` (line 697), `writeStreamMessage`
- [ ] `internal/component/lg/render.go` - `LGServer.renderPage`, `LGServer.renderFragment`, `LGServer.writeHTML` (line 192)
- [ ] `internal/component/lg/server.go` - `LGServer.registerRoutes`, `writeJSONError` (line 983), `writeSVG` (line 1009)
- [ ] `internal/core/errorfragment/errorfragment.go` - `writer.Unwrap`
- [ ] `internal/component/web/editor.go` - `EditorManager.GetOrCreate`, `EditorManager.Discard`, `EditorManager.evictInactive` (line 922)
- [ ] `internal/component/web/editor_history.go`, `doctor_codes.go`, `register_cli_history_diagnostic.go`, `recovery_blocked_editor.go`, `handler_config_recovery.go` - web doctor codes, event-code registration, `retireHistory`, `blockedRecoveryEditor`
- [ ] `internal/component/cli/editor_commands.go` - `Editor.Discard`

**HEAD disposition of all 8 instances (verified 2026-11-27):**

| # | File and Line (HEAD) | Function | Resolution | Doctor Code or Guard Symbol |
|---|----------------------|----------|------------|-----------------------------|
| 1 | `internal/component/command/pipe_resolve.go:60` | `ReverseLookup` | **Answer 2** | `codeDoctorResolvePTRFailed` (`doctor-resolve-ptr-failed`): registered via `diagnostic.Register` + `diagnostic.RegisterEventCode` in `internal/component/command/register_doctor_codes.go`; on `r.ResolvePTR(ip)` error, calls `report.RaiseErrorFixOnce("command-resolve", codeDoctorResolvePTRFailed, "ptr-lookup", msg, fix, time.Minute)` and emits `resolveLog.Warn(msg, "code", codeDoctorResolvePTRFailed, "fix", fix, "ip", ip, "error", err)` when raised, with fix `ze show system dns` |
| 2 | `internal/component/config/cli/cmd_dump.go:104` | `maskValue` | **Answer 1** | Return `(string, error)` (always returning `config.SecretDataPlaceholder` beside a non-nil `opererr.Wrap` error on `encodeSecret` failure, never plaintext `v`); propagate through `maskMapValues`, `resolveDumpBytes`, `printTreeMap`, and `printConfig` so `ze config dump`, `show config dump`, and `show config supplied dump` refuse the dump cleanly with what, why, and fix `ze config dump --strip-private` |
| 3 | `internal/component/lg/handler_graph.go:302` | `writeText` | **Answer 3** | `guards handleGraph` (`handleGraph` declared in `handler_graph.go`, distinct from `writeText`): HTTP text write fails only when the public client disconnected or timed out after headers were sent; no operator action exists |
| 4 | `internal/component/lg/handler_ui.go:697` | `LGServer.handleUIEvents` | **Answer 3** | `guards writeStreamMessage` (`writeStreamMessage` declared in `handler_ui.go`, distinct from `LGServer.handleUIEvents`): `SetWriteDeadline` returns `http.ErrNotSupported` on HTTP/2 or test recorders and fails on already-closed connections; `writeStreamMessage` ends the stream on any subsequent write error |
| 5 | `internal/component/lg/render.go:192` | `LGServer.writeHTML` | **Answer 3** | `guards renderPage` (`renderPage` declared in `render.go`, distinct from `LGServer.writeHTML`): template rendering already succeeded into a buffer before `writeHTML`; `w.Write` fails only when the public client disconnected or timed out |
| 6 | `internal/component/lg/server.go:983` | `writeJSONError` | **Answer 3** | `guards registerRoutes` (`registerRoutes` declared in `server.go`, distinct from `writeJSONError`): HTTP error status and JSON Content-Type header are already written; `fmt.Fprintf` fails only when the public client disconnected or timed out |
| 7 | `internal/component/lg/server.go:1009` | `writeSVG` | **Answer 3** | `guards handleGraph` (`handleGraph` declared in sibling non-test file `handler_graph.go` of package `lg`, visible via `addPackageNames`, distinct from `writeSVG`): SVG rendering already succeeded; `fmt.Fprint` fails only when the public client disconnected or timed out |
| 8 | `internal/component/web/editor.go:922` | `EditorManager.evictInactive` | **Answer 2** | `codeDoctorWebEditorEvictDiscardFailed` (`doctor-web-editor-evict-discard-failed`): registered via `diagnostic.Register` in `internal/component/web/doctor_codes.go` and `diagnostic.RegisterEventCode` in `internal/component/web/register_cli_history_diagnostic.go`; still deletes the idle session from `m.sessions`, calls `report.RaiseErrorFixOnce("web-editor", codeDoctorWebEditorEvictDiscardFailed, "evict-discard", msg, fix, time.Minute)` (fixed message without secrets), and emits `serverLogger.Warn(msg, "code", codeDoctorWebEditorEvictDiscardFailed, "fix", fix, "user", name, "error", discardErr)` with fix `ze show doctor` |

**Behavior to preserve:**
- Every surface's answer format, the birdwatcher LG JSON contract, and all looking-glass HTTP/HTTPS/TLS listener rules.
- `| resolve` still enriches all resolvable rows and leaves `<key>-name` empty (`""`) on an unreachable reverse-DNS lookup without aborting the upstream command or breaking `monitor ping | resolve | log` / `show bgp rib | resolve | count`.
- `config dump` never prints an unmasked secret on any path, including when `$9$` encoding fails.
- Idle web editor eviction (`evictInactive`) continues to free session slots and preserve persisted user drafts.

**Behavior to change:**
- `ReverseLookup` (`internal/component/command/pipe_resolve.go:60`) replaces its hidden `Debug` line with a rate-limited `Warn` carrying `doctor-resolve-ptr-failed`, a `fix`, and a `report.RaiseErrorFixOnce` event.
- `maskValue` (`internal/component/config/cli/cmd_dump.go:104`) propagates `$9$` encoding failure as an operator error through `maskMapValues`, `resolveDumpBytes`, `printTreeMap`, and `printConfig`, while keeping `config.SecretDataPlaceholder` as the returned string value.
- `EditorManager.evictInactive` (`internal/component/web/editor.go:922`) replaces its hidden `Debug` line with a coded `Warn` carrying `doctor-web-editor-evict-discard-failed`, a `fix`, and a `report.RaiseErrorFixOnce` event, while still deleting the evicted session from `m.sessions`.
- All 5 looking-glass HTTP response-write / write-deadline drop sites carry verified `// refuse-by-default: guards <Symbol> -- <why>` annotations.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError`; `./le arch refuse-by-default report` | Lists the 8 unannotated instances under this child's paths | Applies unchanged: acceptance check AC-1 |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` `parseGuard`, `namesEnclosing` | Accepts `refuse-by-default: guards <Symbol> -- <why>` naming a visible non-enclosing symbol | Applies to the 5 LG Answer-3 sites (AC-5) |
| Doctor-coverage commit gate | `internal/le/arch/doctorcoverage/doctorcoverage.go` `Scan`, `siteRoots`; `docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code" | Refuses any added `Warn`/`Error` log call or report-bus raise under `internal/component/` without a covered doctor `code` | Applies unchanged: every Answer-2 `Warn` added here carries a registered event code (AC-2, AC-4) |
| Transient event-code registration | `internal/core/diagnostic/event_codes.go` `RegisterEventCode`, `EventCode`; `internal/component/doctor/checks_recent_events.go` | Lets owning packages declare transient doctor codes described via `diagnostic.Register` so `show doctor` and `doctor-coverage` cover them | Extends: `command` registers `doctor-resolve-ptr-failed` and `web` registers `doctor-web-editor-evict-discard-failed` |
| Rate-limited report bus | `internal/core/report/report.go` `RaiseErrorFixOnce` | Raises a transient error event at most once per window using the error ring as its ledger and returns `bool` | Extends: used by `ReverseLookup` and `EditorManager.evictInactive` so bursts do not flood logs or evict unrelated events |
| Web doctor code registry | `internal/component/web/doctor_codes.go` `doctorDiagnosticCodes`, `registerDoctorCodes`; `internal/component/web/register_cli_history_diagnostic.go` `init` | Registers web-owned doctor code metadata and transient event codes in alphabetical-init-safe order | Extends: adds `codeDoctorWebEditorEvictDiscardFailed` beside the existing two web history codes |
| Operator error wrapping | `internal/core/opererr/opererr.go` `opererr.Wrap`; `docs/architecture/cli/error-surface.md` | Formats what failed, why it matters, and the exact fix across all CLI and daemon surfaces | Extends: `maskValue` / `resolveDumpBytes` wraps `$9$` encode failure as an operator error (AC-3) |
| Config secret masking | `internal/component/config/cli/cmd_dump.go` `maskValue`, `maskMapValues`; `internal/component/config/secret/secret.go` `Encode`; `config.SecretDataPlaceholder` | Replaces sensitive leaves with `$9$` or `/* SECRET-DATA */` in `ze config dump`, `show config dump`, and `show config supplied dump` | Extends: `maskValue` returns `(string, error)` on encode failure instead of silently substituting the placeholder behind `slog.Debug` |
| HTTP ResponseController unwrap | `internal/core/errorfragment/errorfragment.go` `writer.Unwrap` | Exposes the underlying `http.ResponseWriter` to `http.NewResponseController` on HTTP/1.1 | Applies unchanged: explains why `SetWriteDeadline` in `handleUIEvents` fails only on unsupported writers (HTTP/2, test recorders) or closed connections |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Every message a user sees names the problem, why, and the exact fix" | agrees | Every Answer-1 operator error (`maskValue`) and Answer-2 warning/event (`ReverseLookup`, `EditorManager.evictInactive`) states what failed, why it matters, and the exact command to run next (`ze config dump --strip-private`, `ze show system dns`, `ze show doctor`) |
| `docs/architecture/api/commands.md` | Local-data commands (answered in the caller's own process, and by the daemon) | anchored to `internal/component/command/pipe_resolve.go`; anchored to `internal/component/config/cli/cmd_dump.go` | "A handler that cannot answer returns a nil payload and its operator error in the diagnostic (`command.LocalRefusal`): what went wrong, why it prevented an answer, and the recovery action." | agrees | When `maskValue` fails inside `resolveDumpBytes`, `dataDump` and `suppliedDump` return `command.LocalRefusal` with the operator error and a nil payload; `resolveJSON` keeps its declared address-field selection unchanged |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `\| resolve` remains a pipe operation after the command path; reporting PTR resolver failures through a coded `Warn` and report-bus event adds no subcommand and rewrites no command line |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `ReverseLookup` keeps the exact generic `\| resolve` answer shape (`<key>-name` sibling field) unchanged on lookup error while surfacing the resolver failure via doctor/log |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `\| resolve` continues to extend the upstream command's answer without altering or aborting the upstream command when a reverse DNS lookup fails |
| `ai/doors/commands-cli.md` | Boundaries | the door covering `internal/component/command/pipe_resolve.go` | "A handler answers data, never text." | agrees | `pipe_resolve.go` continues to operate on structured JSON/positional records and emits its resolver failure warning only to the logger and report bus, never injecting text into command payload data |
| `docs/architecture/config/syntax.md` | Overview | the `// Design:` page of `internal/component/config/cli/cmd_dump.go` | "File-based dump, validation, formatting, graph and comparison commands also have caller-supplied forms under `show config supplied`. They receive bounded base64 input through the normal command dispatcher and use the same native syntax and byte producers as the file forms." | agrees | Propagating the `maskValue` error from `resolveDumpBytes` ensures both file-based `ze config dump` / `show config dump` and caller-supplied `show config supplied dump` share the exact same fail-closed masking behavior |
| `docs/architecture/config/yang-config-design.md` | Extensions: `ze-extensions.yang` | anchored to `internal/component/config/cli/cmd_dump.go` | "Leaf holds sensitive data. Display obfuscates it with JunOS-compatible `$9$` encoding" | agrees | `maskValue` continues to obfuscate `ze:sensitive` leaves with `$9$` in `DisplayEncode` mode and now fails closed with an operator error (never emitting plaintext) if encoding fails |
| `docs/architecture/testing/ci-format.md` | Port Substitution | anchored to `internal/component/config/cli/cmd_dump.go` | N/A | silent | `ci-format.md` specifies `.ci` test syntax and notes that `ze config dump` reads stdin and resolves a `bgp {}` block; it does not specify `maskValue`'s error-handling behavior on `$9$` encoding failure |
| `ai/doors/config-yang.md` | Who depends on it from outside | the door covering `internal/component/config/cli/cmd_dump.go` | "The config grammar the YANG schema accepts, the `set` form, the error text, the `ze config validate` verdict" | agrees | No YANG leaf or config grammar changes; a `config dump` masking failure now produces a structured operator error with problem, cause, and exact fix (`ze config dump --strip-private`) |
| `docs/architecture/web-interface.md` | Per-User Editor | the `// Design:` page of `internal/component/lg/handler_graph.go`, `handler_ui.go`, `render.go`, `server.go`, and `internal/component/web/editor.go` | "Idle eviction preserves those persisted changes." | agrees | `EditorManager.evictInactive` keeps calling `us.editor.Discard()` (preserving persisted drafts) and deleting the idle session from `m.sessions`, while upgrading any discard error from a hidden `Debug` line to a coded `Warn` and report-bus event; all 5 LG response-write/SSE sites preserve their exact HTML, text, SVG, JSON, and SSE behavior |
| `docs/architecture/api/birdwatcher-compat.md` | 2. Transport and framing | anchored to `internal/component/lg/handler_graph.go`, `handler_ui.go`, `server.go` | "The server MUST answer a request under `/api/looking-glass/` that matches no endpoint with a JSON error body, and MUST NOT fall through to an HTML error page." | agrees | Annotating `writeJSONError` (and the other 4 LG response helpers) changes no HTTP status code, header, or JSON/HTML/SVG/text body |
| `ai/doors/api-surfaces.md` | Where it sits | the door covering `internal/component/lg/*` and `internal/component/web/*` | "None of them owns a command or reads protocol state." | agrees | No command, route, or protocol import is added; LG gets 5 guard annotations and web adds one transient doctor event code inside `internal/component/web/` |
| `docs/architecture/pki/tls-listeners.md` | Decision: each looking-glass listener serves HTTPS or HTTP | anchored to `internal/component/lg/server.go` | N/A | silent | `tls-listeners.md` governs PKI certificate selection, rotation, and per-listener HTTP/HTTPS transport; this spec only annotates response-write error handling in `writeJSONError` and `writeSVG` and changes no TLS or listener code |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | This spec modifies only `writeJSONError` and `writeSVG` in `lg/server.go` with guard annotations; per-`server` `tls` selection, mixed HTTP/HTTPS serving, and RFC 8522 TLS enforcement are untouched |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | This spec does not touch `listenerTLSMaterial`, `managedCertificate`, or any TLS startup/reload path |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | This spec changes no certificate startup or reload behavior; web and looking-glass startup/reload handling remains exactly as ruled |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | `// Design:` page of `internal/component/web/doctor_codes.go`, `internal/component/web/register_cli_history_diagnostic.go`, `internal/component/command/register_doctor_codes.go`; modified by this spec | "A site is covered when its code is one that a doctor check declares." | agrees | Both new transient Answer-2 codes (`doctor-resolve-ptr-failed`, `doctor-web-editor-evict-discard-failed`) register `CodeMeta` via `diagnostic.Register` and `diagnostic.EventCode` via `diagnostic.RegisterEventCode`, emit `"code", <Const>` on their `Warn` lines, raise on `internal/core/report` via `report.RaiseErrorFixOnce`, and are added to the `### Event codes` table in this page |
| `docs/architecture/diagnostics/production-diagnostics.md` | Structured log fields | anchored to `internal/component/web/register_cli_history_diagnostic.go` | "Log producers remain responsible for excluding secrets, just as they are for the configured text/syslog backend." | agrees | The `doctor-web-editor-evict-discard-failed` report-bus raise uses a fixed operator message without config values, secrets, or raw parser text, matching the existing web-history diagnostic pattern |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator command running `| resolve` (`show bgp peer list | resolve`, `show bgp rib | resolve`, `monitor ping | resolve | log`) or `ze config dump` / `show config dump` / `show config supplied dump`.
- A public browser or birdwatcher HTTP/SSE request to the looking glass (`internal/component/lg/`).
- A web-editor session admission hitting capacity (`EditorManager.GetOrCreate` → `evictInactive`).

### Transformation Path
1. **Site 1 (`pipe_resolve.go:60`, Answer 2):** `ReverseLookup` calls `r.ResolvePTR(ip)`. On non-nil `err`, it calls `report.RaiseErrorFixOnce` under `doctor-resolve-ptr-failed` (1-minute window); when raised, it logs `resolveLog.Warn` with `"code", codeDoctorResolvePTRFailed`, `"fix", "ze show system dns"`, `"ip"`, and `"error"`. It returns `""` so the upstream command's rows remain intact.
2. **Site 2 (`cmd_dump.go:104`, Answer 1):** `maskValue` calls `encodeSecret(v)` (`secret.Encode`). On error, it returns `config.SecretDataPlaceholder` plus an `opererr.Wrap` error naming the failure, why no dump was produced, and `ze config dump --strip-private`. `maskMapValues` and `resolveDumpBytes` return the error to `dataDump`, `suppliedDump`, and `dumpAsking` (which refuse with exit 1 and nil payload), and `printTreeMap` / `printConfig` return the error to `cmdDump` (which prints to stderr and exits 1). Plaintext is never returned or printed.
3. **Sites 3–7 (LG, Answer 3):** `writeText`, `LGServer.handleUIEvents`, `LGServer.writeHTML`, `writeJSONError`, and `writeSVG` retain their `Debug` log on write / `SetWriteDeadline` error with a verified `// refuse-by-default: guards <Symbol> -- <why>` annotation directly above each `if`.
4. **Site 8 (`web/editor.go:922`, Answer 2):** `EditorManager.evictInactive` calls `us.editor.Discard()`. On non-nil `discardErr`, it calls `report.RaiseErrorFixOnce` under `doctor-web-editor-evict-discard-failed` (1-minute window) with a fixed secret-free operator message and logs `serverLogger.Warn` with `"code", codeDoctorWebEditorEvictDiscardFailed`, `"fix", "ze show doctor"`, `"user", name`, and `"error", discardErr`, then still unlocks and deletes `m.sessions[name]`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `command` / `web` → `core/diagnostic` + `core/report` | `init()` registers `CodeMeta` + `EventCode` (`internal/component/resolve/cmd/register_doctor_codes.go` for `command.CodeDoctorResolvePTRFailed` via A-3 fallback; `internal/component/web/doctor_codes.go` and `register_cli_history_diagnostic.go` for `codeDoctorWebEditorEvictDiscardFailed`); failure sites emit coded `Warn` and call `report.RaiseErrorFixOnce` | Yes (`TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce`, `TestReverseLookupDoctorCodeRegistered`, `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent`, `./le arch doctor-coverage check` 0 findings) |
| `config/cli` → operator (local CLI & daemon surfaces) | `resolveDumpBytes` / `printConfig` returns `*opererr.Error` via `command.LocalRefusal` / stderr exit 1 | Yes (`TestDumpMaskFailureNeverPrintsTheSecret` PASS under `-race`) |
| `lg` → public HTTP/SSE client | Response bytes unchanged; client disconnect drops remain annotated at `Debug` | Yes (`./le arch refuse-by-default report` 0 findings / 5 annotated, `internal/component/lg` unit tests PASS, `test/ui/lg-peer-table-flat-payload.ci` PASS) |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`; terms `ReverseLookup`, `applyResolve`, `maskValue`, `maskMapValues`, `printConfig`, `printTreeMap`, `resolveDumpBytes`, `writeJSONError`, `writeSVG`, `writeText`, `writeHTML`, `evictInactive`, `doctor-resolve-ptr-failed`, `doctor-web-editor-evict-discard-failed`. Positive controls: `pipe_resolve.go:123`, `pipe_records.go:637`, and `cli/model_enrich.go:31` call `ReverseLookup`; `cmd_dump.go:91`, `cmd_dump.go:171`, `cmd_dump.go:287`, `config_supplied.go:98`, and `inspect_route.go:379` call `maskValue` / `maskMapValues` / `resolveDumpBytes`; `web/editor.go:134` and `discard_snapshot_test.go:43` call `evictInactive`. Findings: no external wire contract changes; two new transient doctor event codes appear in `show doctor` / `show errors` / `ze show diagnostic`, and `config dump` returns an operator error rather than a masked placeholder if `$9$` encoding ever fails.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze config dump` / `show config dump` / `show config supplied dump` failure output | writer | `internal/component/config/cli/cmd_dump.go` (`maskValue`, `maskMapValues`, `resolveDumpBytes`, `printConfig`, `cmdDump`) | Previously substituted `/* SECRET-DATA */` on `secret.Encode` error behind `slog.Debug` | Change: returns `*opererr.Error` with exit 1 and nil payload (never emits plaintext); normal `$9$` and `--strip-private` outputs are byte-identical |
| `ze config dump` / `show config dump` / `show config supplied dump` failure output | reader | `internal/component/config/cli/config_data.go` (`dataDump`), `config_supplied.go` (`suppliedDump`), `inspect_route.go` (`dumpAsking`) | All three already check `err != nil` from `resolveDumpBytes` and return `command.LocalRefusal` | Preserve: existing error propagation handles the new error with zero caller changes outside `cmd_dump.go` |
| Report bus + `show doctor` / `ze show diagnostic` event codes | writer | `internal/component/command/pipe_resolve.go` (`ReverseLookup`) + `register_doctor_codes.go`; `internal/component/web/editor.go` (`evictInactive`) + `doctor_codes.go` + `register_cli_history_diagnostic.go` | New codes `doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed` | Additive: registered with `diagnostic.Register` and `diagnostic.RegisterEventCode`, raised via `report.RaiseErrorFixOnce` |
| Report bus + `show doctor` / `ze show diagnostic` event codes | reader | `internal/component/doctor/checks_recent_events.go` (`recentEventsDiagnostic`), `show errors`, `show log recent`, `ze show diagnostic` | Generic readers over `diagnostic.Lookup` and `report.Errors` | Preserve: new codes render automatically with their title, description, examples, and fix |
| `\| resolve` JSON/positional answer shape | writer and reader | `internal/component/command/pipe_resolve.go` (`resolveJSON`), `pipe_records.go` (`transformPositionalAddressItem`), `cli/model_enrich.go` (`enrichAddr`), `test/plugin/rib-declared-order-refused.ci`, `test/ui/monitor-ping-pipe-resolve-log.ci` | `ReverseLookup` still returns `""` on error | Preserve: zero change to JSON fields, positional rows, or exit codes |
| Looking glass HTTP JSON/HTML/SVG/text/SSE responses | writer and reader | `internal/component/lg/handler_graph.go`, `handler_ui.go`, `render.go`, `server.go`; Alice-LG / birdwatcher clients and browsers | Only `// refuse-by-default: guards ...` comments added | Preserve: byte-identical responses |

### Integration Points
- `internal/component/command/register_doctor_codes.go` (`init()`) registers `doctor-resolve-ptr-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`).
- `internal/component/web/doctor_codes.go` and `internal/component/web/register_cli_history_diagnostic.go` (`init()`) register `doctor-web-editor-evict-discard-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`).
- `docs/architecture/doctor-and-health-checks.md` (`### Event codes`) documents both transient event codes with source anchors.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `maskValue` errors leave through `resolveDumpBytes` → `command.LocalRefusal` / `cmdDump`; Answer-2 failures emit coded `Warn` + `report.RaiseErrorFixOnce` into `recent-events` |
| No unintended coupling (components stay isolated) | Yes | Only downward imports to leaf packages `internal/core/diagnostic` and `internal/core/report` (no cross-component or `internal/le` import) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `report.RaiseErrorFixOnce`, `opererr.Wrap`, `config.SecretDataPlaceholder`, and `web`'s existing `doctor_codes.go` / `register_cli_history_diagnostic.go` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control and error paths only; `ReverseLookup` warns at most once per minute |
| Registration over hardcoding, outbound | Yes | Both new doctor codes register from their owning component's `init()` via `diagnostic.Register` and `diagnostic.RegisterEventCode` |
| Registration over hardcoding, inbound | Yes | `./le arch doctor-coverage` and `recent-events` discover both codes from declarations and registries with no central code list edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The exact HEAD population under child 9's paths is 8 unannotated `error-dropped-below-warn` instances across 7 files | Filtered `./le arch refuse-by-default report '|' json` run on 2026-11-27 | Re-run the report and update the table before the first edit | Re-verified at implementation start and closure (AC-1) | confirmed |
| A-2 | `maskValue` today never prints an unmasked secret on `secret.Encode` failure (it returns `config.SecretDataPlaceholder`) | Direct read of `internal/component/config/cli/cmd_dump.go:98-114` and `internal/component/config/secret/secret.go:109-143` on 2026-11-27 | Would be a P0 secret leak requiring immediate fix | Confirmed by reading `cmd_dump.go:104-107`; locked in by `TestDumpMaskFailureNeverPrintsTheSecret` | confirmed |
| A-3 | `internal/component/command` is allowed by `./le arch tier check` to import `internal/core/diagnostic` and `internal/core/report` | `ai/doors/core.md` ("Everything depends on it. Components, plugins and the public SDK all import core") | Move doctor-code registration to a sibling command helper if a cycle exists | `./le arch tier check` and `./le test unit pkg path ./internal/component/command/` (`internal/core/diagnostic` imports `plugin/registry` → `command`; resolved via fallback by registering `command.CodeDoctorResolvePTRFailed` in `internal/component/resolve/cmd/register_doctor_codes.go`, unconditionally linked in `all.go:152` and `cmd/ze/hub/main.go:48`) | broken (resolved via fallback) |
| A-4 | `ze show system dns` is a valid registered YANG command path for `doctor-resolve-ptr-failed`'s `fix` | `docs/features.md:21` ("`show system dns` prints the learned records and the selection") and `internal/plugins/system-dns-cmd` | Substitute the exact registered DNS diagnostic command from the YANG command tree | Confirmed in `internal/plugins/system-dns-cmd/yang/ze-system-dns-cmd.yang:32` (`ze:command "ze-system-dns:show-system-dns"` under `show > system > dns`) and `internal/plugins/system-dns-cmd/cmd/show_dns.go:30-40` (`showSystemDNS`) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A multi-thousand-row `show bgp rib \| resolve` with no DNS server floods daemon WARN logs or evicts unrelated events from the report bus ring | Unit test calling `ReverseLookup` repeatedly with a failing resolver produces more than 1 `Warn` or more than 1 bus event | Gate both the bus raise and the `resolveLog.Warn` emission on `report.RaiseErrorFixOnce(..., time.Minute)` returning `true` |
| R-2 | `maskValue` returning an error leaks the plaintext secret in the returned string or wrapped error message | `TestDumpMaskFailureNeverPrintsTheSecret` inspects the returned value, error text, and dump output | Always return `config.SecretDataPlaceholder` as the string value beside the error and never interpolate `v` into the `opererr.Wrap` text |
| R-3 | `web/register.go` runs alphabetically before `web/register_cli_history_diagnostic.go`, so registering an event code in `register.go` before `registerDoctorCodes()` panics at startup | `go test ./internal/component/web/` panics in `init()` | Register `codeDoctorWebEditorEvictDiscardFailed` in `register_cli_history_diagnostic.go` directly after `registerDoctorCodes()` |
| R-4 | A guard annotation in `lg/` names an enclosing function or an invisible symbol | `./le arch refuse-by-default check` reports `annotation-names-no-guard` | All 5 guard symbols (`handleGraph`, `writeStreamMessage`, `renderPage`, `registerRoutes`, `handleGraph`) are pre-verified non-enclosing symbols declared in the file or a sibling non-test file of package `lg` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A `Warn` without a covered doctor code fails `./le commit create`, an init-order bug panics package `web` at startup, or a broken `maskValue` path leaks a secret or breaks `ze config dump` |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md`, `spec-doctor-coverage-8-commands-cli.md`, `spec-doctor-coverage-9-config-yang.md` (same packages; no overlapping code names) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over child 9's paths | Filtered report reports 0 `error-dropped-below-warn` and 0 `annotation-names-no-guard` |
| `./le arch doctor-coverage check` | → | `Scan` / `judge` over added `Warn` and `report.RaiseErrorFixOnce` sites | Gate exits 0 with both new event codes covered |
| `\| resolve` with failing `PTRResolver` | → | `ReverseLookup` (`pipe_resolve.go`) | `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` (`internal/component/command/pipe_resolve_test.go`) |
| `ze config dump` / `resolveDumpBytes` with failing secret encoder | → | `maskValue`, `maskMapValues`, `resolveDumpBytes`, `printConfig` (`cmd_dump.go`) | `TestDumpMaskFailureNeverPrintsTheSecret` (`internal/component/config/cli/cmd_dump_test.go`) |
| `EditorManager.GetOrCreate` evicting an idle session whose `Discard` fails | → | `EditorManager.evictInactive` (`web/editor.go`) | `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` (`internal/component/web/editor_evict_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to child 9's paths (`internal/component/{api,web,lg,gnmi,mcp,telemetry,command,cmd,cli,ssh}` and `internal/component/config` outside `config/system`) | 0 `findings` entries with rule `error-dropped-below-warn` and 0 with rule `annotation-names-no-guard`; `./le arch refuse-by-default check` exits 0 | The 5 LG HTTP response-write / `SetWriteDeadline` sites appear in `annotated` (not `findings`), because continuing after a disconnected public client or unsupported deadline clear is correct and no operator action exists |
| AC-2 | `ReverseLookup` calls a `PTRResolver` that returns an error (once, and repeatedly within 1 minute) | Returns `""` so upstream rows stay intact; logs `Warn` with `"code", "doctor-resolve-ptr-failed"`, `"fix", "ze show system dns"`, `"ip"`, and `"error"` and raises on `internal/core/report` under `doctor-resolve-ptr-failed` once per 1-minute window; `diagnostic.Lookup("doctor-resolve-ptr-failed")` returns non-nil metadata and `diagnostic.EventCodes()` contains it; `./le arch doctor-coverage check` exits 0 | NXDOMAIN / empty-answer calls (`ResolvePTR` returning `nil, nil`) log nothing and raise nothing because absence of a PTR record is not a resolver failure |
| AC-3 | `maskValue` / `resolveDumpBytes` / `printConfig` runs in `DisplayEncode` mode when `encodeSecret` returns an error | Refuses with a non-nil `*opererr.Error` naming what failed, why no dump was produced, and fix `ze config dump --strip-private`; the returned string value is `config.SecretDataPlaceholder`; the plaintext secret never appears in the returned value, error text, or dump output; normal `DisplayEncode` and `DisplayStrip` dumps remain unchanged | `DisplayStrip` and `DisplayPlain` never call `encodeSecret` and are unaffected |
| AC-4 | `EditorManager.evictInactive` evicts an idle session whose `editor.Discard()` returns an error | Eviction still removes the idle session from `m.sessions` (freeing capacity for `GetOrCreate`), logs `Warn` with `"code", "doctor-web-editor-evict-discard-failed"`, `"fix", "ze show doctor"`, `"user"`, and `"error"`, and raises a secret-free event on `internal/core/report` under `doctor-web-editor-evict-discard-failed` via `report.RaiseErrorFixOnce`; `diagnostic.Lookup("doctor-web-editor-evict-discard-failed")` returns non-nil metadata and `diagnostic.EventCodes()` contains it; `./le arch doctor-coverage check` exits 0 | Normal `*cli.Editor` sessions return `nil` from `Discard()` and emit no warning or bus event on idle eviction |
| AC-5 | Each of the 5 LG annotations (`handler_graph.go:302`, `handler_ui.go:697`, `render.go:192`, `server.go:983`, `server.go:1009`) | Each carries `// refuse-by-default: guards <Symbol> -- <why>` on the line above the `if`, naming a visible non-enclosing dependent symbol (`handleGraph`, `writeStreamMessage`, `renderPage`, `registerRoutes`, `handleGraph`) and stating why continuing is right; LG HTTP responses remain unchanged | None |
| AC-6 | Scoped package tests (`-race`), architecture gates, doc checks, and reached `.ci` functional tests (`test/ui/monitor-ping-pipe-resolve-log.ci`, `test/plugin/rib-declared-order-refused.ci`, `test/ui/lg-peer-table-flat-payload.ci`) | All pass green when run in the main checkout by the main thread | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` | `internal/component/command/pipe_resolve_test.go` | AC-2 (returns `""`, emits `Warn` with `doctor-resolve-ptr-failed` and fix, raises once per 1-minute window on repeated errors, NXDOMAIN `nil, nil` stays silent) | PASS (`-race`) |
| `TestReverseLookupDoctorCodeRegistered` | `internal/component/command/pipe_resolve_diagnostic_test.go` | AC-2 (`diagnostic.Lookup` and `diagnostic.EventCodes` include `doctor-resolve-ptr-failed` when `resolve/cmd` is linked; split into `package command_test` due to A-3 cycle) | PASS (`-race`) |
| `TestDumpMaskFailureNeverPrintsTheSecret` | `internal/component/config/cli/cmd_dump_test.go` | AC-3 (`maskValue`, `resolveDumpBytes`, and `printConfig` return `*opererr.Error` with fix `ze config dump --strip-private`, return `SecretDataPlaceholder`, and never leak the plaintext secret in value, error, or output) | PASS (`-race`) |
| `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` | `internal/component/web/editor_evict_test.go` | AC-4 (eviction frees the session slot, emits `Warn` with `doctor-web-editor-evict-discard-failed` and `ze show doctor`, raises secret-free event on `report`, `diagnostic.Lookup` and `diagnostic.EventCodes` include the code; nil `Discard()` stays silent) | PASS (`-race`) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unannotated `error-dropped-below-warn` findings under child 9's paths | 0 | 0 | N/A | 1 |
| Uncovered `doctor-coverage` sites added by this change | 0 | 0 | N/A | 1 |
| `ReverseLookup` WARN + bus raises for N consecutive PTR errors inside 1 minute | 1 | 1 | 0 | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `monitor-ping-pipe-resolve-log.ci` | `test/ui/monitor-ping-pipe-resolve-log.ci` | `monitor ping \| resolve \| log` enriches with PTR and keeps working unchanged | PASS |
| `rib-declared-order-refused.ci` | `test/plugin/rib-declared-order-refused.ci` | `show bgp rib \| resolve \| count \| json` exits 0 without aborting on unconfigured DNS | PASS |
| `lg-peer-table-flat-payload.ci` | `test/ui/lg-peer-table-flat-payload.ci` | Looking glass HTTP UI/API responses remain unchanged | PASS |

### Interop Tests (Scope: protocol)
N-A: no protocol or wire-format change.

## Files to Modify
- `internal/component/command/pipe_resolve.go` - 1 site: `ReverseLookup` line 60 (Answer 2: coded rate-limited `Warn` + `report.RaiseErrorFixOnce`)
- `internal/component/config/cli/cmd_dump.go` - 1 site: `maskValue` line 104 (Answer 1: return `(string, error)` via `encodeSecret` seam, propagate through `maskMapValues`, `resolveDumpBytes`, `printTreeMap`, `printConfig`, `cmdDump`)
- `internal/component/lg/handler_graph.go` - 1 site: `writeText` line 302 (Answer 3: `guards handleGraph`)
- `internal/component/lg/handler_ui.go` - 1 site: `LGServer.handleUIEvents` line 697 (Answer 3: `guards writeStreamMessage`)
- `internal/component/lg/render.go` - 1 site: `LGServer.writeHTML` line 192 (Answer 3: `guards renderPage`)
- `internal/component/lg/server.go` - 2 sites: `writeJSONError` line 983 (`guards registerRoutes`); `writeSVG` line 1009 (`guards handleGraph`)
- `internal/component/web/editor.go` - 1 site: `EditorManager.evictInactive` line 922 (Answer 2: coded `Warn` + `report.RaiseErrorFixOnce`, still deletes idle session)
- `internal/component/web/doctor_codes.go` - add `codeDoctorWebEditorEvictDiscardFailed` and its `CodeMeta`
- `internal/component/web/register_cli_history_diagnostic.go` - register `codeDoctorWebEditorEvictDiscardFailed` via `diagnostic.RegisterEventCode` after `registerDoctorCodes()`
- `docs/architecture/doctor-and-health-checks.md` - document `doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed` in `### Event codes` with source anchors

Lines are those of HEAD on 2026-11-27; re-run `./le arch refuse-by-default report` before the first edit if lines drift.

## Files to Create
- `internal/component/command/register_doctor_codes.go` - register `codeDoctorResolvePTRFailed` (`CodeMeta` via `diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/component/command/pipe_resolve_test.go` - unit tests for AC-2 (`TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce`)
- `internal/component/config/cli/cmd_dump_test.go` - unit tests for AC-3 (`TestDumpMaskFailureNeverPrintsTheSecret`)
- `internal/component/web/editor_evict_test.go` - unit tests for AC-4 (`TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command added; `config dump` returns an operator error on `$9$` encode failure |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | Yes | `\| resolve` keeps enriching rows on PTR failure while reporting the DNS failure once per minute under `doctor-resolve-ptr-failed` |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | `doctor-resolve-ptr-failed` (`command/register_doctor_codes.go`) and `doctor-web-editor-evict-discard-failed` (`web/doctor_codes.go`, `web/register_cli_history_diagnostic.go`) registered with `diagnostic.Register` + `diagnostic.RegisterEventCode` and raised on `internal/core/report` |
| Prometheus counters/metrics | N-A | No counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (`### Event codes`: add `doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed` with source anchors) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (two new registered doctor event codes) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec design documents` and `./le spec citation anchors`: `commands.md`, `syntax.md`, `yang-config-design.md`, `ci-format.md`, `web-interface.md`, `birdwatcher-compat.md`, `tls-listeners.md`, `doctor-and-health-checks.md`, `production-diagnostics.md`; only `doctor-and-health-checks.md` requires an edit |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

This spec resolves all 8 remaining unannotated `error-dropped-below-warn`
findings across `internal/component/command/`, `internal/component/config/cli/`,
`internal/component/lg/`, and `internal/component/web/` (reconciled against HEAD
on 2026-11-27):

- **1 Answer-1 error return (`cmd_dump.go:104`, `maskValue`):** when `$9$`
  encoding fails in `DisplayEncode` mode, `maskValue` returns
  `config.SecretDataPlaceholder` plus an `*opererr.Error` (never plaintext `v`),
  propagated through `maskMapValues`, `resolveDumpBytes`, `printTreeMap`, and
  `printConfig` so `ze config dump`, `show config dump`, and
  `show config supplied dump` refuse the dump cleanly with fix
  `ze config dump --strip-private`.
- **2 Answer-2 doctor-covered warnings (`pipe_resolve.go:60` `ReverseLookup`,
  `web/editor.go:922` `EditorManager.evictInactive`):** both replace hidden
  `Debug` lines with `Warn` + `report.RaiseErrorFixOnce` (1-minute window)
  carrying newly registered transient doctor event codes
  (`doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed`) so
  operators see resolver outages and idle-eviction discard failures in
  `ze show doctor`, `ze show errors`, and default-level logs without log
  flooding.
- **5 Answer-3 guard annotations (all 5 looking-glass sites):** `writeText`,
  `LGServer.handleUIEvents`, `LGServer.writeHTML`, `writeJSONError`, and
  `writeSVG` receive verified `// refuse-by-default: guards <Symbol> -- <why>`
  annotations naming visible non-enclosing dependent symbols (`handleGraph`,
  `writeStreamMessage`, `renderPage`, `registerRoutes`, `handleGraph`).

### Before and after

1. **Reverse DNS pipe failure (`pipe_resolve.go:60` `ReverseLookup`):**
   - Before: when the system DNS resolver returns an error (no DNS server configured or all servers timed out), `ReverseLookup` writes only a `Debug` line hidden at the default WARN level and returns `""`, leaving the DNS outage invisible in logs and `show doctor`.
   - After: `ReverseLookup` still returns `""` so upstream rows and monitors are never aborted, and records the outage once per minute via `report.RaiseErrorFixOnce` + `resolveLog.Warn` under `doctor-resolve-ptr-failed` with fix `ze show system dns`.
2. **Secret encoding failure in `config dump` (`cmd_dump.go:104` `maskValue`):**
   - Before: `maskValue` logs `slog.Debug("secret encode failed", ...)` and silently substitutes `/* SECRET-DATA */`, hiding the `$9$` encoding failure from the operator.
   - After: `maskValue` returns `config.SecretDataPlaceholder` and an `*opererr.Error`, refusing `ze config dump`, `show config dump`, and `show config supplied dump` cleanly with fix `ze config dump --strip-private` while still guaranteeing plaintext is never emitted.
3. **Idle web-editor eviction discard failure (`web/editor.go:922` `EditorManager.evictInactive`):**
   - Before: if `us.editor.Discard()` returns an error while evicting an idle session at capacity, `evictInactive` logs only a `Debug` line and drops it.
   - After: `evictInactive` still deletes the idle session from `m.sessions` to free capacity and records any non-nil discard error via `report.RaiseErrorFixOnce` (secret-free message) + `serverLogger.Warn` under `doctor-web-editor-evict-discard-failed` with fix `ze show doctor`; normal `*cli.Editor` evictions stay silent.
4. **Looking-glass HTTP response writes and SSE deadline clear (5 sites in `internal/component/lg/`):**
   - Before: 5 unannotated `error-dropped-below-warn` findings in `./le arch refuse-by-default report`.
   - After: 5 annotated sites naming their verified dependent guards (`handleGraph`, `writeStreamMessage`, `renderPage`, `registerRoutes`, `handleGraph`), 0 unannotated findings, and byte-identical HTTP/SSE behavior.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Make `ReverseLookup` Answer 1 (abort `\| resolve` and return an error to the command caller) | Breaks `show bgp rib \| resolve \| count \| json` (`test/plugin/rib-declared-order-refused.ci:111`), breaks `monitor ping \| resolve \| log` / `monitor traceroute \| resolve \| log` (`cli/model_enrich.go`), and would destroy an entire peer table when a single peer IP has a lame reverse-DNS delegation |
| Keep `maskValue` returning `config.SecretDataPlaceholder` with an Answer-3 annotation or Answer-2 `Warn` | Violates Answer-1 priority: `config dump` answers an interactive/RPC operator request; when `$9$` encoding fails, silently substituting `/* SECRET-DATA */` hides the fault rather than refusing the command with `--strip-private` as the explicit fix |
| Annotate `EditorManager.evictInactive` as Answer 3 instead of Answer 2 | `GetOrCreate` does not guard `Discard()` failures, and `EditorManager`'s own idle history retirement (`editor_history.go:147`) already treats idle cleanup failures as doctor-coded `Warn` events; Answer 2 is silent in normal operation (`*cli.Editor.Discard()` returns `nil`) and truthful if a `contract.Editor.Discard()` ever fails |
| Raise the 5 looking-glass HTTP response-write / `SetWriteDeadline` sites to `Warn` | Public browsers and birdwatcher clients disconnect routinely and HTTP/2 writers return `http.ErrNotSupported` for `SetWriteDeadline`; warning on those would let external traffic flood daemon logs and `show doctor` with unactionable noise |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring and red tests** -- re-verify the 8 HEAD findings with `./le arch refuse-by-default report '|' json`, verify `ze show system dns` against the YANG command tree (A-4), and write the three unit tests (`TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce`, `TestDumpMaskFailureNeverPrintsTheSecret`, `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent`) failing against current HEAD.
2. **Phase 2: Resolve Answer 1 and Answer 2 sites** -- implement `command/register_doctor_codes.go` + `pipe_resolve.go`, `cmd_dump.go` error propagation + `encodeSecret` seam, and `web/doctor_codes.go` + `web/register_cli_history_diagnostic.go` + `web/editor.go`; update `docs/architecture/doctor-and-health-checks.md` (`### Event codes`).
3. **Phase 3: Resolve Answer 3 sites and prove** -- add the 5 verified guard annotations in `internal/component/lg/`, verify all unit tests pass under `-race`, and run all gates (`refuse-by-default`, `doctor-coverage`, `arch tier check`, `go style check`, `doc check all`, and scoped `.ci` tests) from the main thread in the main checkout.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 8 HEAD instances resolved; filtered `refuse-by-default report` shows 0 unannotated `error-dropped-below-warn` and 0 `annotation-names-no-guard` |
| Correctness | No unmasked secret on any `config dump` path; `\| resolve` never aborts upstream rows and rate-limits warnings to 1 per minute; `evictInactive` still deletes the idle session |
| Doctor coverage | Both Answer-2 sites emit `"code", <Const>` matching registered `CodeMeta` + `EventCode`, raise via `report.RaiseErrorFixOnce`, and pass `./le arch doctor-coverage check` |
| Rule: user-facing-errors | Every returned error and `Warn` names what failed, why, and the exact fix command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero unannotated `error-dropped-below-warn` findings under child 9's paths | `./le arch refuse-by-default report '|' json` filtered to child 9's paths + `./le arch refuse-by-default check` |
| Doctor coverage clean | `./le arch doctor-coverage check` |
| Architecture tier clean | `./le arch tier check` |
| Scoped package tests (`-race`) | `./le test unit pkg path ./internal/component/command/`, `./le test unit pkg path ./internal/component/config/cli/`, `./le test unit pkg path ./internal/component/lg/`, `./le test unit pkg path ./internal/component/web/` |
| Scoped functional `.ci` tests | `test/ui/monitor-ping-pipe-resolve-log.ci`, `test/plugin/rib-declared-order-refused.ci`, `test/ui/lg-peer-table-flat-payload.ci` |
| Docs and style clean | `./le doc check all` and `./le go style check` on changed Go files |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret leakage in `config dump` | `maskValue` failure never returns or logs plaintext `v` and never puts `v` inside the `opererr.Error` message |
| Secret leakage in web editor eviction | `doctor-web-editor-evict-discard-failed` bus event carries a fixed operator message with no config text or raw parser output |
| Log/doctor DoS from public clients | All 5 public LG write/SSE sites remain Answer 3 (`Debug` + guard annotation); `ReverseLookup` and `evictInactive` rate-limit via `report.RaiseErrorFixOnce` (1-minute window) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint or gate failure (`refuse-by-default`, `doctor-coverage`, `tier`) | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Re-reading `internal/component/resolve/dns/resolver.go` and `test/plugin/rib-declared-order-refused.ci` overturned the initial 2026-11-02 sketch that classified `ReverseLookup` as Answer 1: `| resolve` enriches every row of a command answer and live monitors, so failing the whole command on one unreachable reverse-DNS zone is wrong, whereas a rate-limited doctor-coded `Warn` + report-bus event exposes DNS outages without breaking callers.
- Re-reading `internal/component/web/editor_history.go` (`retireHistory`) showed `EditorManager` already treats idle cleanup failures as doctor-coded `Warn` events, making Answer 2 the consistent choice for `EditorManager.evictInactive` as well.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella (`plan/release/fixes/spec-dropped-error-0-umbrella.md`).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One child for three small doors | One child per door | 8 instances across 7 files; reading the three doors together is the cheaper split |
| `ReverseLookup` is Answer 2 (rate-limited coded `Warn` + `report.RaiseErrorFixOnce`) | Answer 1 (abort `\| resolve` command on PTR error) | `ResolvePTR` returns `(nil, nil)` for NXDOMAIN and errors only on resolver failure; aborting the command breaks `show bgp rib \| resolve \| count` and `monitor ping \| resolve \| log`, while a 1-minute rate-limited doctor event alerts the operator to DNS failure without flooding |
| `maskValue` is Answer 1 (return `(string, error)` via `encodeSecret` seam) | Answer 2 (`Warn`) or Answer 3 (annotation) | `config dump` answers an operator command requesting `$9$` encoding; silently substituting `/* SECRET-DATA */` hides an encode failure, whereas returning an `*opererr.Error` alongside `SecretDataPlaceholder` fails closed without ever leaking plaintext |
| `EditorManager.evictInactive` is Answer 2 (coded `Warn` + `report.RaiseErrorFixOnce`, still deletes idle session) | Answer 3 (guard annotation) | Matches `EditorManager.retireHistory` (`editor_history.go:147`); silent in normal operation (`*cli.Editor.Discard()` returns `nil`) and visible in `show doctor` if any `contract.Editor.Discard()` fails |
| All 5 LG HTTP write / `SetWriteDeadline` sites are Answer 3 | Raise to `Warn` | Public client disconnects and HTTP/2 `ErrNotSupported` are unactionable; warning on public HTTP traffic would allow log and doctor flooding |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions and Permitted Survivors and Why
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package tests (`-race`), architecture gates (`refuse-by-default`, `doctor-coverage`, `tier`), doc checks, and reached `.ci` functional tests pass in the main checkout
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-9-api-cli-config.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- **Site 1 (Answer 2 — `internal/component/command/pipe_resolve.go`, `internal/component/resolve/cmd/register_doctor_codes.go`):** Exported `CodeDoctorResolvePTRFailed` (`doctor-resolve-ptr-failed`) in `pipe_resolve.go`, replaced the hidden `Debug` line in `ReverseLookup` with a rate-limited `report.RaiseErrorFixOnce` (1-minute window, message bounded by `report.FitMessage`) + `resolveLog.Warn` with `"code", CodeDoctorResolvePTRFailed`, `"fix", "ze show system dns"`, `"ip"`, and `"error"` while still returning `""` for upstream rows and staying silent on NXDOMAIN (`nil, nil`). Registered `CodeMeta` (`diagnostic.Register`) and `EventCode` (`diagnostic.RegisterEventCode`) in `internal/component/resolve/cmd/register_doctor_codes.go` (unconditionally linked by `internal/component/plugin/all/all.go:152` and `cmd/ze/hub/main.go:48`). Covered by `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` (`pipe_resolve_test.go`) and `TestReverseLookupDoctorCodeRegistered` (`pipe_resolve_diagnostic_test.go`).
- **Site 2 (Answer 1 — `internal/component/config/cli/cmd_dump.go`):** Added `dumpEncodeSecret = secret.Encode` test seam, changed `maskValue` to return `(string, error)` returning `config.SecretDataPlaceholder` plus an `*opererr.Error` (`opererr.Wrap`) with fix `ze config dump --strip-private` and zero plaintext secret interpolation on `$9$` encode failure, and propagated the error through `maskMapValues`, `resolveDumpBytes`, `printTreeMap`, `printConfig`, and `cmdDump` (exiting 1 with `command.LocalErrorLine(err)` on stderr, and `command.LocalRefusal` through `dataDump`/`suppliedDump`/`dumpAsking`). Covered by `TestDumpMaskFailureNeverPrintsTheSecret` (`cmd_dump_test.go`).
- **Sites 3–7 (Answer 3 — `internal/component/lg/handler_graph.go`, `handler_ui.go`, `render.go`, `server.go`):** Added verified `// refuse-by-default: guards <Symbol> -- <why>` annotations above all 5 looking-glass HTTP response-write / `SetWriteDeadline` drop sites (`writeText` guarding `handleGraph`, `LGServer.handleUIEvents` guarding `writeStreamMessage`, `LGServer.writeHTML` guarding `renderPage`, `writeJSONError` guarding `registerRoutes`, `writeSVG` guarding `handleGraph`).
- **Site 8 (Answer 2 — `internal/component/web/editor.go`, `doctor_codes.go`, `register_cli_history_diagnostic.go`):** Added `codeDoctorWebEditorEvictDiscardFailed` (`doctor-web-editor-evict-discard-failed`) and `webShowDoctorCommand` (`ze show doctor`) in `doctor_codes.go`, registered its `EventCode` in `register_cli_history_diagnostic.go` after `registerDoctorCodes()`, and updated `EditorManager.evictInactive` to call `report.RaiseErrorFixOnce` (1-minute window, fixed secret-free message) and `serverLogger.Warn` with `"code"`, `"fix"`, `"user"`, and `"error"` while still unlocking and deleting the idle session from `m.sessions`. Covered by `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` (`editor_evict_test.go`).
- **Documentation & News Fragment:** Documented `doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed` with 5 source anchors in `docs/architecture/doctor-and-health-checks.md` (`### Event codes`) and wrote `website/changes/fragments/dropped-error-9-api-cli-config.md` (`ste check: OK`).

### Bugs Found/Fixed
- Scoped lint flagged `goconst` for repeated `"ze show doctor"` literals in `internal/component/web/doctor_codes.go` and `editor.go`; deduplicated via package constant `webShowDoctorCommand = "ze show doctor"`.

### Documentation Updates
- `docs/architecture/doctor-and-health-checks.md` (`### Event codes`, lines 1563–1564 and 1574–1578): added `doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed` rows and 5 source anchors (`internal/component/resolve/cmd/register_doctor_codes.go -- doctorDiagnosticCodes, init`; `internal/component/command/pipe_resolve.go -- CodeDoctorResolvePTRFailed, ReverseLookup`; `internal/component/web/doctor_codes.go -- codeDoctorWebEditorEvictDiscardFailed, doctorDiagnosticCodes`; `internal/component/web/register_cli_history_diagnostic.go -- init`; `internal/component/web/editor.go -- EditorManager.evictInactive`). Note: a sibling session has 2 uncommitted rows directly above ours in this shared file, so Commit A carries only our rows/anchors via `hunks docs/architecture/doctor-and-health-checks.md tmp/session/2026-11-27-145824/scratch/doctor-and-health-checks-child9.patch`.
- `website/changes/fragments/dropped-error-9-api-cli-config.md`: created operator-facing news fragment (`./le doc ste check file website/changes/fragments/dropped-error-9-api-cli-config.md`: `ste check: OK`).
- `./le doc check all`: all 5 new source anchors in `docs/architecture/doctor-and-health-checks.md` resolved cleanly; the only failures in the tree-wide run are pre-existing/unrelated (`internal/le/cli/catalog/catalog.go:0` `plugin format` drift, 1 unrelated YANG summary rule, and pre-existing line 1668 `RegisterPluginEventTypes` anchor).

### Deviations from Plan
- **Assumption A-3 fallback (`command` → `diagnostic` import cycle):** The plan originally proposed creating `internal/component/command/register_doctor_codes.go`, with A-3's fallback specifying moving registration to a sibling command helper if a cycle exists. Because `internal/core/diagnostic` imports `internal/component/plugin/registry` which imports `internal/component/command`, importing `diagnostic` from `command` creates an import cycle. Applied the planned A-3 fallback: exported `command.CodeDoctorResolvePTRFailed` in `internal/component/command/pipe_resolve.go`, registered its `CodeMeta` and `EventCode` in `internal/component/resolve/cmd/register_doctor_codes.go` (unconditionally linked by `internal/component/plugin/all/all.go:152` and `cmd/ze/hub/main.go:48`), and placed the registration assertion in `internal/component/command/pipe_resolve_diagnostic_test.go` (`package command_test`) so it can blank-import `resolve/cmd` without a cycle.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Assumed A-3 (`internal/component/command` could directly import `internal/core/diagnostic`) | `internal/core/diagnostic` imports `internal/component/plugin/registry`, which imports `internal/component/command`, forming an import cycle | Compilation during implementation of `command/register_doctor_codes.go` | Executed A-3 fallback: exported `command.CodeDoctorResolvePTRFailed`, registered in `internal/component/resolve/cmd/register_doctor_codes.go`, and added `pipe_resolve_diagnostic_test.go` in `package command_test` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Resolve Site 1 (`pipe_resolve.go:60` `ReverseLookup`, Answer 2) | Done | `internal/component/command/pipe_resolve.go:25-31,64-86`; `internal/component/resolve/cmd/register_doctor_codes.go:18-38` | Coded rate-limited `Warn` + `report.RaiseErrorFixOnce` (`doctor-resolve-ptr-failed`, fix `ze show system dns`); returns `""` |
| Resolve Site 2 (`cmd_dump.go:104` `maskValue`, Answer 1) | Done | `internal/component/config/cli/cmd_dump.go:63-66,72,87-201,312-314` | Returns `(string, error)` with `SecretDataPlaceholder` + `*opererr.Error` (`ze config dump --strip-private`), propagated through `maskMapValues`, `resolveDumpBytes`, `printTreeMap`, `printConfig`, `cmdDump` |
| Resolve Sites 3–7 (5 LG sites, Answer 3) | Done | `internal/component/lg/handler_graph.go:302`, `handler_ui.go:697`, `render.go:192`, `server.go:983,1010` | All 5 carry verified `// refuse-by-default: guards <Symbol> -- <why>` annotations |
| Resolve Site 8 (`web/editor.go:922` `evictInactive`, Answer 2) | Done | `internal/component/web/editor.go:916-942`; `doctor_codes.go:10-44`; `register_cli_history_diagnostic.go:28-30` | Coded `Warn` + rate-limited secret-free `report.RaiseErrorFixOnce` (`doctor-web-editor-evict-discard-failed`, fix `ze show doctor`); still deletes idle session |
| Register both Answer-2 doctor codes (`diagnostic.Register` + `RegisterEventCode`) | Done | `internal/component/resolve/cmd/register_doctor_codes.go:18-38`; `internal/component/web/doctor_codes.go:32-44`; `internal/component/web/register_cli_history_diagnostic.go:28-30` | Verified by unit tests and `./le arch doctor-coverage check` (0 findings) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le arch refuse-by-default report '\|' json` filtered to child 9 paths: 0 `error-dropped-below-warn`, 0 `annotation-names-no-guard`, 5/5 LG annotated | Confirmed by main thread |
| AC-2 | Done | `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` (`pipe_resolve_test.go`), `TestReverseLookupDoctorCodeRegistered` (`pipe_resolve_diagnostic_test.go`), `./le arch doctor-coverage check` (0 findings) | PASS under `-race` |
| AC-3 | Done | `TestDumpMaskFailureNeverPrintsTheSecret` (`cmd_dump_test.go`) | PASS under `-race`; verified no plaintext secret in value, error, diag, or stdout |
| AC-4 | Done | `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` (`editor_evict_test.go`), `./le arch doctor-coverage check` (0 findings) | PASS under `-race`; verified slot freed and report-bus event secret-free |
| AC-5 | Done | `internal/component/lg/{handler_graph.go:302,handler_ui.go:697,render.go:192,server.go:983,server.go:1010}`; `internal/component/lg` unit tests PASS; `test/ui/lg-peer-table-flat-payload.ci` PASS | All 5 annotations verified by `refuse-by-default` |
| AC-6 | Done | Unit tests (`command`, `resolve/cmd`, `config/cli`, `lg`, `web`) PASS under `-race`; `.ci` tests (`monitor-ping-pipe-resolve-log.ci`, `rib-declared-order-refused.ci`, `lg-peer-table-flat-payload.ci`) PASS; `doctor-coverage`, `tier`, `go style`, scoped lint, `test weakened` clean | All confirmed in main checkout |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` | Done | `internal/component/command/pipe_resolve_test.go:51` | PASS (`-race`) |
| `TestReverseLookupDoctorCodeRegistered` | Done | `internal/component/command/pipe_resolve_diagnostic_test.go:24` | Added in `command_test` for A-3 fallback; PASS (`-race`) |
| `TestDumpMaskFailureNeverPrintsTheSecret` | Done | `internal/component/config/cli/cmd_dump_test.go:76` | PASS (`-race`) |
| `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` | Done | `internal/component/web/editor_evict_test.go:51` | PASS (`-race`) |
| `monitor-ping-pipe-resolve-log.ci` | Done | `test/ui/monitor-ping-pipe-resolve-log.ci` | PASS |
| `rib-declared-order-refused.ci` | Done | `test/plugin/rib-declared-order-refused.ci` | PASS |
| `lg-peer-table-flat-payload.ci` | Done | `test/ui/lg-peer-table-flat-payload.ci` | PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/command/pipe_resolve.go` | Done | Modified (`CodeDoctorResolvePTRFailed`, `ReverseLookup`) |
| `internal/component/command/register_doctor_codes.go` | Changed | Replaced via A-3 fallback by `internal/component/resolve/cmd/register_doctor_codes.go` (recorded in Deviations) |
| `internal/component/resolve/cmd/register_doctor_codes.go` | Done | Modified (A-3 fallback registration for `command.CodeDoctorResolvePTRFailed`) |
| `internal/component/command/pipe_resolve_test.go` | Done | Created |
| `internal/component/command/pipe_resolve_diagnostic_test.go` | Done | Created (`package command_test` for A-3 fallback registration check) |
| `internal/component/config/cli/cmd_dump.go` | Done | Modified (`dumpEncodeSecret`, `maskValue`, `maskMapValues`, `printTreeMap`, `printConfig`, `resolveDumpBytes`, `cmdDump`) |
| `internal/component/config/cli/cmd_dump_test.go` | Done | Created |
| `internal/component/lg/handler_graph.go` | Done | Modified (guard annotation on `writeText`) |
| `internal/component/lg/handler_ui.go` | Done | Modified (guard annotation on `LGServer.handleUIEvents`) |
| `internal/component/lg/render.go` | Done | Modified (guard annotation on `LGServer.writeHTML`) |
| `internal/component/lg/server.go` | Done | Modified (guard annotations on `writeJSONError`, `writeSVG`) |
| `internal/component/web/doctor_codes.go` | Done | Modified (`codeDoctorWebEditorEvictDiscardFailed`, `webShowDoctorCommand`) |
| `internal/component/web/register_cli_history_diagnostic.go` | Done | Modified (`diagnostic.RegisterEventCode` for eviction code) |
| `internal/component/web/editor.go` | Done | Modified (`EditorManager.evictInactive`) |
| `internal/component/web/editor_evict_test.go` | Done | Created |
| `docs/architecture/doctor-and-health-checks.md` | Done | Modified (2 rows + 5 source anchors; committed via `hunks` patch due to sibling rows) |

### Audit Summary
- **Total items:** 34 (5 requirements + 6 ACs + 7 tests + 16 files)
- **Done:** 33
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`internal/component/command/register_doctor_codes.go` → `internal/component/resolve/cmd/register_doctor_codes.go` per A-3 fallback, recorded in Deviations and Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Eliminate all 8 unannotated `error-dropped-below-warn` instances across child 9's paths | Static architecture gate | `./le arch refuse-by-default report '\|' json` filtered to child 9 paths reports 0 `error-dropped-below-warn`, 0 `annotation-names-no-guard`, and 5/5 LG sites in `annotated` |
| Surface reverse DNS lookup failures in `\| resolve` at `Warn` + `show doctor` without flooding or aborting upstream rows | Unit + functional tests + doctor gate | `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` and `TestReverseLookupDoctorCodeRegistered` PASS under `-race`; `test/ui/monitor-ping-pipe-resolve-log.ci` and `test/plugin/rib-declared-order-refused.ci` PASS; `./le arch doctor-coverage check` reports 0 findings |
| Refuse `config dump` cleanly with an operator error when `$9$` secret encoding fails without ever leaking plaintext | Unit test (`-race`) | `TestDumpMaskFailureNeverPrintsTheSecret` (`internal/component/config/cli/cmd_dump_test.go`) PASS under `-race` across `maskValue`, `resolveDumpBytes`, `printConfig`, and normal `DisplayEncode`/`DisplayStrip` |
| Surface idle web-editor eviction `Discard()` failures at `Warn` + secret-free `show doctor` event while freeing session capacity | Unit test (`-race`) + doctor gate | `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` (`internal/component/web/editor_evict_test.go`) PASS under `-race`; `./le arch doctor-coverage check` reports 0 findings |
| Annotate all 5 LG HTTP response-write / `SetWriteDeadline` sites without changing HTTP/SSE responses | Static gate + unit + functional test | `internal/component/lg` unit tests PASS under `-race`; `test/ui/lg-peer-table-flat-payload.ci` PASS; 5/5 sites accepted in `refuse-by-default` `annotated` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All 8 in-scope instances, tests, docs, and gates are complete; tree-wide acceptance remains owned by the umbrella spec | `plan/release/fixes/spec-dropped-error-0-umbrella.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dropped-error-9-api-cli-config-b5a10001-1111-4000-8000-000000000001.md` |
| `./le spec review check` | `OK (14 code files, clean, hashes match tmp/review/dropped-error-9-api-cli-config-b5a10001-1111-4000-8000-000000000001.md)` |
| Rounds | 1 |
| Reviewer lenses used | independent ze-work closure reviewer: logic+wiring, security+edge-cases, spec-completeness+design-docs+go-style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | Assumption A-3 fell back to registering `command.CodeDoctorResolvePTRFailed` in `internal/component/resolve/cmd/register_doctor_codes.go` due to the `command -> diagnostic -> plugin/registry -> command` import cycle; unconditionally linked via `all.go:152` and `cmd/ze/hub/main.go:48` | `internal/component/resolve/cmd/register_doctor_codes.go:18-38` | Recorded in Deviations from Plan, Mistake Log, and Assumptions Resolved; 0 BLOCKER, 0 ISSUE |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/command/pipe_resolve_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 3904 Nov 27 15:08 internal/component/command/pipe_resolve_test.go` |
| `internal/component/command/pipe_resolve_diagnostic_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 1299 Nov 27 15:08 internal/component/command/pipe_resolve_diagnostic_test.go` |
| `internal/component/resolve/cmd/register_doctor_codes.go` | Yes | `-rw-rw-rw- 1 thomas thomas 2187 Nov 27 15:08 internal/component/resolve/cmd/register_doctor_codes.go` |
| `internal/component/config/cli/cmd_dump_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 5644 Nov 27 16:09 internal/component/config/cli/cmd_dump_test.go` |
| `internal/component/web/editor_evict_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 4773 Nov 27 15:04 internal/component/web/editor_evict_test.go` |
| `test/ui/monitor-ping-pipe-resolve-log.ci` | Yes | `-rw-r--r-- 1 thomas thomas 1900 Nov 4 16:42 test/ui/monitor-ping-pipe-resolve-log.ci` |
| `test/plugin/rib-declared-order-refused.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3923 Nov 24 19:37 test/plugin/rib-declared-order-refused.ci` |
| `test/ui/lg-peer-table-flat-payload.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3171 Nov 23 23:24 test/ui/lg-peer-table-flat-payload.ci` |
| `website/changes/fragments/dropped-error-9-api-cli-config.md` | Yes | Created and passed `./le doc ste check file website/changes/fragments/dropped-error-9-api-cli-config.md` (`ste check: OK`) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 0 unannotated `error-dropped-below-warn` and 0 `annotation-names-no-guard` under child 9 paths | Filtered `./le arch refuse-by-default report '\|' json`: 0 `error-dropped-below-warn`, 0 `annotation-names-no-guard`, 5/5 LG annotated |
| AC-2 | `ReverseLookup` returns `""`, warns + raises `doctor-resolve-ptr-failed` once/min with `ze show system dns`, silent on NXDOMAIN, code registered | Direct read of `pipe_resolve.go:25-31,64-86` and `resolve/cmd/register_doctor_codes.go:18-38`; `TestReverseLookupFailureWarnsAndRaisesDoctorEventOnce` and `TestReverseLookupDoctorCodeRegistered` PASS under `-race`; `./le arch doctor-coverage check` 0 findings |
| AC-3 | `maskValue` / `resolveDumpBytes` / `printConfig` return `*opererr.Error` (`ze config dump --strip-private`) and `SecretDataPlaceholder`, never plaintext | Direct read of `cmd_dump.go:63-66,72,87-201,312-314`, `config_data.go:136-139`, `config_supplied.go:98-101`, `inspect_route.go:379-383`; `TestDumpMaskFailureNeverPrintsTheSecret` PASS under `-race` |
| AC-4 | `evictInactive` frees idle slot, warns + raises secret-free `doctor-web-editor-evict-discard-failed` with `ze show doctor`, code registered | Direct read of `web/editor.go:916-942`, `web/doctor_codes.go:10-44`, `web/register_cli_history_diagnostic.go:28-30`; `TestEvictInactiveDiscardFailureWarnsAndRaisesDoctorEvent` PASS under `-race`; `./le arch doctor-coverage check` 0 findings |
| AC-5 | 5 LG sites carry verified `// refuse-by-default: guards <Symbol> -- <why>` annotations | Direct read of `handler_graph.go:302`, `handler_ui.go:697`, `render.go:192`, `server.go:983,1010`; `internal/component/lg` unit tests PASS; `test/ui/lg-peer-table-flat-payload.ci` PASS |
| AC-6 | Scoped unit tests (`-race`), architecture gates, doc checks, and `.ci` tests green | Main thread confirmed PASS for `command`, `resolve/cmd`, `config/cli`, `lg`, `web` unit tests, all 3 `.ci` tests, `refuse-by-default`, `doctor-coverage`, `tier`, `go style`, scoped lint, and `test weakened` (0 findings on our files) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `monitor ping 192.0.2.1 \| resolve \| log` (`ReverseLookup`) | `test/ui/monitor-ping-pipe-resolve-log.ci` | Read lines 22–28, 45–46: drives real TUI model with fake PTR resolver (`ping-target.test`); PASS |
| `show bgp rib \| resolve \| count \| json` (`ReverseLookup` via daemon pipe) | `test/plugin/rib-declared-order-refused.ci` | Read lines 109–113: runs `ze cli -c "show bgp rib \| resolve \| count \| json"` against real daemon expecting exit 0 and count 5; PASS |
| Looking glass HTTP API (`writeJSONError`, `LGServer`) | `test/ui/lg-peer-table-flat-payload.ci` | Read lines 94–103: starts real `ze` with LG enabled and verifies `/api/looking-glass/protocols/bgp` and `/protocols/short` HTTP 200; PASS |
| `resolve/cmd` doctor code registration (`CodeDoctorResolvePTRFailed`) | `internal/component/plugin/all/all.go:152`; `cmd/ze/hub/main.go:48` | Verified unconditional blank import in `all.go:152` and direct import in `hub/main.go:48`; proven by `TestReverseLookupDoctorCodeRegistered` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Filtered `./le arch refuse-by-default report '\|' json` confirmed exact 8 instances across 7 files before edits and 0 unannotated findings after |
| A-2 | confirmed | Direct read of `cmd_dump.go:111-129` confirmed `maskValue` returns `config.SecretDataPlaceholder`, never plaintext `v`; locked in by `TestDumpMaskFailureNeverPrintsTheSecret` |
| A-3 | broken (resolved via fallback) | `internal/core/diagnostic` imports `plugin/registry` → `command` (cycle). Resolved via A-3 fallback: registered `command.CodeDoctorResolvePTRFailed` in `internal/component/resolve/cmd/register_doctor_codes.go` (unconditionally linked in `all.go:152` and `hub/main.go:48`) |
| A-4 | confirmed | `internal/plugins/system-dns-cmd/yang/ze-system-dns-cmd.yang:32` declares `ze:command "ze-system-dns:show-system-dns"` under `show > system > dns`, handled by `showSystemDNS` in `internal/plugins/system-dns-cmd/cmd/show_dns.go:30-40` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/doctor-and-health-checks.md` (`### Event codes`, rows `doctor-resolve-ptr-failed` and `doctor-web-editor-evict-discard-failed`) | Verified against `pipe_resolve.go:25-31,64-86`, `resolve/cmd/register_doctor_codes.go:18-38`, `web/doctor_codes.go:10-44`, `web/register_cli_history_diagnostic.go:28-30`, `web/editor.go:916-942`; all 5 source anchors checked by `./le doc check all` (0 anchor failures on our lines) | Yes |
| Other anchored docs (`commands.md`, `syntax.md`, `yang-config-design.md`, `ci-format.md`, `web-interface.md`, `birdwatcher-compat.md`, `tls-listeners.md`, `production-diagnostics.md`, `guide/looking-glass.md`, `guide/web-interface.md`) | Grepped `docs/` for `<!-- source:` matching all changed Go files; verified existing anchored claims (`resolveJSON`, `resolveDumpBytes`, `EditorManager`, `Discard`, LG handlers) remain accurate with no stale claim | Yes |
| News fragment (`website/changes/fragments/dropped-error-9-api-cli-config.md`) | Verified against `website/changes/fragments/README.md` (front matter + 6 fixed headings) and `./le doc ste check file website/changes/fragments/dropped-error-9-api-cli-config.md` (`ste check: OK`) | Yes |
