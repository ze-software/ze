# Web Interface Architecture

The ze web interface is an HTTPS server that renders YANG-driven configuration views using HTMX components. All UI is server-rendered, from typed Go components the templ generator compiles. HTMX handles navigation, auto-save, and error display via out-of-band swaps. `cli.js` handles browser command entry, completion, content loading and stream controls.

For the component design, template filesystem, and interaction flows, see [web-components.md](web-components.md).

All source files in `internal/component/web/` reference this document via `// Design:` comments.

## Source Files

| File | Responsibility |
|------|---------------|
| `server.go` | HTTPS server, TLS config, self-signed cert generation, cert persistence |
| `auth.go` | Session store, auth middleware, login handler, Basic Auth, `GetUsernameFromRequest` |
| `handler.go` | URL parsing, content negotiation, route registration |
| `fragment.go` | HTMX fragment handler, `FragmentData`, `FieldMeta`, sidebar builder |
| `error_fragment.go` | What a refused request is answered with: `WriteOOBError`, which writes the error fragment for the request's target and then the out-of-band error item. The fragment itself, and the middleware that renders one from the plain-text body `http.Error` writes, are shared: `errorfragment.Render` and `errorfragment.Middleware` (`internal/core/errorfragment`). The middleware converts a 4xx or 5xx answer only when the request carries `HX-Request` and the body is text/plain, so a JSON client, a plain browser navigation and a handler's own markup each reach the client untouched. `ServerHandler` (`auth.go`) is the one place it wraps the mux |
| `handler_config.go` | Config set/delete/commit/discard handlers, `ConfigViewData`, `HandleConfigView` |
| `handler_config_walk.go` | Schema + tree walking, `buildConfigViewData`, `populateContainerView` |
| `handler_config_leaf.go` | `buildLeafField`, `leafInputType`, `configViewComponent`, breadcrumbs |
| `handler_admin.go` | Admin command tree navigation and execution. `HandleAdminView` takes a per-request tree source (`AdminCommandTree`). The hub's source, `adminCommandTrees` (`cmd/ze/hub/service_web.go`), answers for each request the YANG command tree (every builtin, local and local-data command under its YANG path) with the plugin commands the registry lists overlaid on a copy (`command.OverlayCommandPaths` of `VisibleCommandEntries()`, which asks the surface predicate `command.ListedOnDaemonSurfaces`), so a plugin command with no YANG node is offered, a hidden one is not, and a plugin that exits is gone on the next request; the shared tree is never written. The command still runs through the dispatcher with the caller's identity, and the route keeps its edit authorization, so an overlaid command is authorized exactly as a builtin is. The finder columns read the tree shape. The command form reads the two help texts of the node it shows: the `ze:help` summary as the lede, the `description` explanation as the body, and one text input for each argument the node declares, each carrying that leaf's two texts. `HandleAdminExecute` takes the same source: it builds the command from the URL path and the posted argument values the node declares (`commandArguments`), through `command.WriteInvocation`, the one declaration of argument placement the MCP tool call shares (`docs/architecture/mcp/overview.md`). A value whose `ArgDef.Anchor()` names a path keyword goes bare after that keyword, inside the path, which is where `anchoredDef` binds a peer selector; every other value is appended after the command as `name value`, in declaration order. An empty value is left out, so the dispatcher's own mandatory check names a missing argument. When the YANG loader fails at hub startup the tree is nil. The admin nav is then empty, and the failure is logged to stderr, rather than a stale static map taking its place. An empty admin nav tells the operator that the hub did not load its command modules. |
| `cli.go` | Legacy `POST /cli` response adapter and browser CLI completion; the adapter and dedicated CLI page share the terminal executor |
| `cli_editor.go` | Adapts editor commands to the shared CLI model through `EditorManager.ConfigCommand` |
| `editor.go` | Per-user `EditorManager`, working tree isolation, change tracking |
| `render.go` | `Renderer`: embedded assets, decorators, and the entry points the hub calls (`RenderLayout`, `RenderLogin`, `RenderWorkbench`, `RenderField`, `RenderDiffModal`) |
| `sse.go` | `EventBroker`, SSE client management (each client under its user), config change broadcast, `SendTo` one user |
| `window_notices.go` | `WindowNotices`: one worker pushing each user the confirm window status and forced-commit discard notices the SSH status line shows |
| `ui_mode.go` | `UIMode` selector: graphical YANG editor by default, experimental Workbench by explicit selection |
| `handler_workbench.go` | workbench shell handler; reuses fragment data path with workbench chrome |
| `workbench_sections.go` | Left-nav section taxonomy (Dashboard/Routing/Logs/...) |
| `workbench_enrich.go` | Promotes any named list to a workbench table; attaches per-row tools and pending markers |
| `handler_tools.go` | `POST /tools/related/run`: resolves `ze:related` descriptors, dispatches via `CommandDispatcher`, renders the overlay |
| `related_resolver.go` | Placeholder substitution for `ze:related` command templates against the user's working tree |

<!-- source: internal/component/web/server.go -- WebServer struct -->
<!-- source: internal/component/web/auth.go -- SessionStore, authMiddleware -->
<!-- source: internal/component/web/fragment.go -- HandleFragment, FragmentData -->
<!-- source: internal/component/web/editor.go -- EditorManager -->

## Component Structure

Every page, fragment and editor is a templ component. A component is a Go
function, so the markup and the view model are checked against each other at
build time. The sources sit in the package directory, named by visual concern:

```
internal/component/web/
  page_*.templ        layout, login, workbench
  component_*.templ   breadcrumb, path_bar, sidebar, detail, finder,
                      list_table, log_table, log_live, add_form_overlay,
                      command_form, command_result, cli_bar, commit_bar,
                      error_panel, diff_modal, notification_error,
                      dashboard_*, oob_*, workbench_*, tool_*
  config_*.templ      breadcrumb, command, command_form, commit, container,
                      list, inline_list, freeform, flex, leaf_input,
                      notification
  input_*.templ       wrapper, bool, enum, number, text
  l2tp_*.templ        list, detail
  notification_banner.templ, terminal.templ
```

A name ending in `_*` above is a family: every file carrying that prefix
belongs to the line it sits on. Each other name is one file.

`./le repo update` compiles each `.templ` into a `*_templ.go` beside it, and
`./le doc check templ-output` refuses a source whose generated file is stale.

Each input type is one file. `fieldInputFor` (`field_input.go`) reads the `fieldInputs` registry, which maps a YANG field type onto the component that edits it. A type nobody registered reaches the text editor by a named rule. No if/else chain in the markup.

<!-- source: internal/component/web/render.go -- NewRenderer, Renderer -->
<!-- source: internal/component/web/field_input.go -- fieldInputs, fieldInputFor -->

## URL Scheme

```
/show/<yang-path>           Full page or HTMX fragment (GET)
/fragment/detail?path=X     HTMX partial: detail + OOB sidebar/breadcrumb (GET)
/config/set/<path>          Save field value (POST, returns OOB commit bar or error)
/config/diff                Diff modal with changes (GET, returns open modal HTML)
/config/diff-close          Close diff modal (GET, returns closed modal HTML)
/config/commit              Apply pending changes (POST)
/config/discard             Revert pending changes (POST)
/config/import              Stage local file or pasted content in the user's candidate (POST)
/cli                        CLI command execution (POST)
/cli/complete?input=X       Tab/? autocomplete (GET, returns JSON)
/cli/terminal               Terminal mode command (POST, returns JSON)
/cli/stream                 Terminal command, with streamed text for live commands and JSON for other commands (POST)
/cli/view/action            Declared control for an active interactive monitor (POST)
/cli/mode                   Toggle CLI/GUI mode (POST)
/admin/<yang-path>          Admin commands (GET browse, POST execute)
/tools/related/run          workbench: execute a related operator tool (POST)
/login                      Authentication (POST)
/assets/                    Static files (CSS, JS)
/                           Redirects to /show/
```

YANG paths use one URL-escaped segment per schema token. A prefix key such as
`192.0.2.0/24` is transported as `192.0.2.0%2F24`; the slash inside the key
never becomes a hierarchy separator. Fragment query parameters and browser CLI
context carry that same escaped path, with the usual query/form escaping around
it. A literal `.` key uses the segment `~.` so browser dot-segment normalization
cannot remove it; an initial `~` in a literal key is doubled. The server decodes
each URI segment once and restores this carrier before schema validation and
candidate editing. Labels and CLI commands continue to show the literal key.

<!-- source: internal/component/web/path_transport.go -- encodeConfigPath, decodeConfigPath -->
<!-- source: internal/component/web/handler.go -- ParseURL, knownPrefixes -->

## Authentication

Reuses SSH user database (`[]ssh.UserConfig`). Two mechanisms:

| Mechanism | When Used | Session Created |
|-----------|-----------|-----------------|
| Session cookie (`ze-session`) | Browser access | Yes (on login) |
| HTTP Basic Auth | JSON API requests | No |

<!-- source: internal/component/web/auth.go -- authMiddleware, parseBasicAuth -->

Session tokens: 32 bytes from `crypto/rand`, hex-encoded. Cookie: `Secure`, `HttpOnly`, `SameSite=Strict`. One session per user, 24h TTL.

A session also ends when the running configuration stops declaring its user. The session records which backend authenticated it (`AuthResult.Source`), and `validateToken` re-checks a session the LOCAL backend granted against the credentials the running configuration declares right now, on every request. An operator removed by a reload loses an open browser tab at once, with no restart and no wait for the TTL. A session a remote backend granted (RADIUS, TACACS+) is not checked against the local list, because that list never granted it.

<!-- source: internal/component/web/auth.go -- SessionStore.validateToken, webSession -->
<!-- source: cmd/ze/hub/main_servers.go -- liveLocalUsers, liveConfigUsers -->

An SSE stream that is already open survives the removal until the client disconnects: it authenticates at connect and then blocks for the life of the connection, so no later request exists to refuse. Every mutation route is a fresh request and is refused.

## Per-User Editor

The `EditorManager` creates independent `cli.Editor` instances per authenticated user.
Before opening or reusing an editor, it validates the account name through the
same `cli.ValidateUser` boundary as SSH and the console. Unsupported names are
refused before either factory runs, so configuration metadata never invents a
different owner by stripping punctuation or path segments.

<!-- source: internal/component/web/editor.go -- EditorManager, GetOrCreate -->

The factory uses `NewIsolatedEditorWithStorage` explicitly. Ordinary stored CLI
editors keep their shared draft view; isolation is a constructor choice, not an
origin-name test. Saving and rebuilding the web candidate preserve that choice.
<!-- source: internal/component/cli/editor.go -- NewIsolatedEditorWithStorage -->
<!-- source: internal/component/cli/editor_view.go -- workingViewMetadata, projectSessionTree -->

Each session has an isolated working tree, change tracking, and serialized access via per-user mutex. Operations: `SetValue`, `DeleteValue`, `ApplyFormEdits`, `Commit`, `Discard`, `Diff`, `ChangeCount`, `Tree`, `ReadTree`.

A newly opened editor resumes this account's pending web intent before it enters
the manager's cache. Recovery includes saved metadata and unsaved assignments
and structural operations. The shared materialized replay supplies the recorded
ordering; the visible candidate is rebuilt from committed configuration and
only the resumed web session's intent. Other users and this account's SSH
sessions keep their own values and ownership. Candidate text excludes their
pending metadata values, including when a leaf is contested.
<!-- source: internal/component/cli/editor_resume.go -- ResumeSession, resumeSession, workingSessionMetadata -->
<!-- source: internal/component/web/editor.go -- GetOrCreate -->

The committed baseline uses the editor constructor's existing parse policy.
Known fields in set or metadata-set text remain inspectable when another field
is obsolete. This does not make that source valid for commit: publication keeps
its strict reader. The draft and change-file parsers keep their existing
refusals; the committed-view retry never applies to pending intent.
<!-- source: internal/component/cli/editor_parse.go -- parseEditingConfig -->

Identity publication holds one storage guard but consists of separate writes.
An error can leave one file rebound. Those records retain their account and
origin, including when the next attempt receives the same session ID. Recovery
can retry them; a refused write, replay or final guard release never caches an
editor or changes committed configuration.
<!-- source: internal/component/cli/editor_resume.go -- ResumeSession, resumeSessionMetadata -->

Explicit discard-all also removes this user's persisted intent from an older
browser session. Browser discard preserves separate SSH edits under the same
account: the shared session discard producer consumes only the resumed browser
session's assignments and structural operations, and retains other owners in
the per-account change file. Saved leaf cleanup removes ownership and restores the shared
draft's affected values from committed configuration plus the surviving saved
owners. Another account's saved value at the same leaf remains pending; reopening
the discarding account starts from its committed value. Cleanup writes the draft
before consuming change-file records, so a storage failure remains retryable.
Its audit detail selects the exact account and `web` origin from metadata and uses the
same masked change renderer as the review; it does not infer ownership from a
formatted session identifier or claim SSH or another user's changes were dropped.
The discard snapshot reads through a private editor even when the user's cache
entry was evicted. It never assigns a session identity, resumes custody or
caches that reader: inspecting the audit detail leaves the stored bytes intact.
One storage guard covers committed parsing and the saved draft and per-user
change-file metadata. The shared materialized-draft planner follows recorded
moves without changing ownership. A failed listing needed for that replay is
also an error. An open, read, parse or final guard-release failure returns
an error, and every discard transport stops before mutation or audit recording.
An exact owner with no stored intent may return an empty detail. When an active
editor is cached, the snapshot takes its user lock after releasing the manager
lock, so it also serializes with that user's edits.
<!-- source: internal/component/cli/editor_pending_snapshot.go -- PendingDiscardChanges, readPendingDiscardChanges -->
<!-- source: internal/component/web/editor.go -- discardDiff, diffChanges -->
<!-- source: internal/component/web/handler_config_commit.go -- HandleConfigDiscardWithAuthorizerAndAudit -->
<!-- source: internal/component/config/change_file.go -- PendingChangeFromSessionEntry, StructuralOp.PendingChange -->

Authenticated Finder and Workbench requests open the user's current editor and
read a tree snapshot under its session lock, including the first request and
the first request after discard. They refuse a configuration-read failure rather
than displaying the older startup tree. Explicit discard removes the user's
persisted browser changes through the shared session discard producer before ending the
cached editor. Idle eviction preserves those persisted changes.
<!-- source: internal/component/web/editor.go -- viewTree, Discard, evictInactive -->
<!-- source: internal/component/web/fragment.go -- HandleFragment -->
<!-- source: internal/component/web/handler_workbench.go -- HandleWorkbench -->

A workbench form save (`/config/form/`) is one edit: `ApplyFormEdits` holds the per-user mutex across every field the form posts, so the review diff and a commit see the whole save or none of it, and two overlapping saves never leave one save's value beside the other's. The fields apply in path order (`parseConfigFormFields` sorts them). A field the editor refuses stops the save there, and the fields sorted before it stay applied, because the editor holds no rollback.

<!-- source: internal/component/web/editor.go -- ApplyFormEdits -->
<!-- source: internal/component/web/handler_config_form.go -- HandleConfigFormWithAuthorizer, parseConfigFormFields -->

**Nothing a request reads is stored on an object other users share.** The
server answers every logged-in user from one process, so one value built at
start and shared by every request (the config completer the hub creates once)
holds only what never changes per user: the schema and the validator registry.
Each request passes its own input into the call. Browser CLI completion
(`/cli/complete`) reads the user's candidate tree through `ReadTree`, which runs
the completion under that user's session mutex, and hands the tree to
`Completer.Complete` as an argument; the completer keeps no tree between calls.
Two users completing at once therefore each see only their own uncommitted
entries.

Each browser command input owns its completion cache and selected candidate.
The shared menu renderer does not hold that state. Enter accepts the visibly
highlighted candidate without executing it; a later Enter submits the command.

<!-- source: internal/component/web/cli.go -- completeCLIInput -->
<!-- source: internal/component/web/editor.go -- ReadTree -->
<!-- source: internal/component/cli/completer.go -- Completer, Complete, completion -->

`Commit` detects conflicts when two users modify the same leaf and returns `CommitResult` with conflict details. Limits: 50 concurrent sessions, 1 hour idle timeout.

The manager separately owns at most 50 per-user command-history owners, using
the same idle timeout. Editor discard and eviction leave recall intact. An idle
history owner is retired only when no call holds it; its username and capacity
remain reserved until publication drains, so an old writer cannot overwrite a
new owner's snapshot. All owners share the existing history keys and the
manager's borrowed store. `Close` seals history admission and drains accepted
writes without discarding candidate intent; the service MUST join HTTP handlers
before calling it, and MUST finish it before releasing the store.
<!-- source: internal/component/web/editor_history.go -- HistorySnapshot, RecordHistory, Close -->

Authenticated `GET /cli?history=1&mode=config|operational` returns a bounded,
masked `entries` array with private/no-store caching. Invalid mode and failed
reads are explicit errors, rather than empty successful recall. The three CLI
POST routes record the command text once before execution. An optional opaque
`submission` id has a bounded per-user receipt window, allowing a file/paste
continuation to reuse its initial receipt even when another command intervened.
The receipt stores no command or uploaded content. Stream fallback carries the
same in-request receipt. Recall failures do not prevent execution: they travel
in `X-Ze-History-Warning`, and terminal JSON also carries `history-warning`.
<!-- source: internal/component/web/cli_history.go -- recordCLIHistory -->

Failed submission recording, including an invalid receipt, raises
`doctor-web-history-unavailable`. An actual JSON delivery failure raises
`doctor-web-history-response`. Each class raises once per minute from
`web-history`, without commands, usernames or raw errors. Both tell the
operator to open `/cli` and reconnect. Shared storage failures retain their
own read and write codes.
<!-- source: internal/component/web/register_cli_history_diagnostic.go -- reportBrowserHistoryUnavailable, reportBrowserHistoryResponseFailure -->

A failed history snapshot owns a browser recall refusal while retaining the
entire storage error as its cause. If both modes failed, both diagnostics and
both `errors.Is` causes remain available; the final recovery action follows
all of that evidence.
<!-- source: internal/component/web/editor_history.go -- historySnapshotFailure -->
The effective mode and control-command eligibility come from the shared CLI
history policy, including automatic entry into configuration mode.
<!-- source: internal/component/web/editor_history.go -- HistorySnapshot, RecordHistory, Close -->
<!-- source: internal/component/web/cli_history.go -- recordCLIHistory, writeCLIHistory -->

Both browser command inputs fetch this projection when they open, after a
submission, and on the first Up press. Each input keeps separate navigation
positions and unfinished drafts for configuration and operational mode. A newer
request, input change, or mode change prevents a late recall response from
replacing the current input. Completion menus keep priority over recall arrows.
The browser never adds submitted commands to its recall projection or saves
recall in localStorage.
<!-- source: internal/component/web/assets/cli.js -- commandRecall, init, initTerminal -->

Each Enter creates one opaque submission receipt. A bar-to-terminal load handoff
and its content continuation reuse that receipt. The terminal marks only its
automatic opening `show` with `history=0`. Streams refresh recall when their
headers arrive, so an ongoing monitor does not delay recall. Recall warnings
remain visible beside execution feedback while commands continue to run.
<!-- source: internal/component/web/assets/cli.js -- newSubmission, init, submitTermCommand -->

Editors with a reload hook commit transactionally: `CommitSessionCandidate` stages a candidate version, the hook (`reloadAfterCommit`) reloads the daemons and promotes the candidate. Editors without a hook commit through the shared configuration store. The web editor manager, SSH session factory and `ze start --cli` console borrow the daemon's one owning store handle.
An explicit-file daemon keeps that file authoritative: publication checks for external changes, updates the file and records stored history. Bare `ze start` uses the stored active configuration.
<!-- source: cmd/ze/hub/session_editor.go -- newSessionEditor reload notifier wiring, attachedConsoleEditor -->
<!-- source: cmd/ze/hub/main.go -- sessionReloadHolder late binding -->

Configuration downloads read the daemon's selected source, including external edits to an explicit file. Uploads use the same daemon-owned publication callback and report conflict or reload failures without replacing the committed configuration. Standalone managers stage uploads as candidates and promote them only after acceptance.

The graphical candidate import is a separate authenticated, same-origin POST
at `/config/import`. Native file and paste forms choose root or selected scope,
and merge or replace. The whole form is bounded at 4 MiB before decoding.
The adapter validates choices with the shared load grammar, preserves path
tokens and filename quoting, and supplies the bytes to the normal per-user
configuration command executor. That executor owns typed parsing and atomic
candidate staging. The response refreshes the selected view and OOB commit
status; a removed selected entry returns to its surviving parent collection.
No import publishes configuration before a separate shared commit. Read-only
accounts cannot submit an import; a refused import preserves their candidate
and the submitted form for correction. `GraphicalImportError` supplies the
visible refusal to both this response and the offline corpus's declared-error
check; the corpus must observe a real refusal before accepting its declaration.
<!-- source: internal/component/web/graphical_import.go -- HandleGraphicalImport, readGraphicalImport, GraphicalImportError -->
<!-- source: internal/component/web/editor.go -- SetConfigSource, committedConfig, applyCommittedContent -->

The terminal and the legacy `POST /cli` adapter parse `commit` with the grammar every editor
shares (`contract.ParseCommit`). `commit now`, `commit confirmed`,
`commit accept` and `commit abort` run through `cli.WindowCommit`, the code the
SSH session editor runs, against the daemon's confirmed-commit window that the
hub hands the manager (`SetConfirmWindow`). `commit verify` validates the
user's view and applies nothing. With no daemon window (`--web-only`),
`commit now` applies directly and the window subcommands are refused.
<!-- source: internal/component/web/editor_commit_window.go -- SetConfirmWindow, runCommit, verifyCommit -->
<!-- source: cmd/ze/hub/service_web.go -- SetConfirmWindow -->

The graphical **Confirm Commit** request also uses `runCommit`. An applied
commit keeps its success outcome when session cleanup reports a warning. The
HTMX response closes the review modal, clears the commit bar and displays the
complete success and advisory text in the existing informational notification
bar. A later configuration-change event retains that answer and replaces only
the previous change banner, so repeated events leave at most one banner beside
the advisory. A normal browser request with a warning renders the same text in
the successful page's content, where later notification-bar updates cannot
erase it; clean normal requests keep their redirect to `/`. Neither feedback
path runs the commit again or repeats its success audit and change event.
<!-- source: internal/component/web/handler_config_commit.go -- handleCommitPost -->
<!-- source: internal/component/web/assets/sse-client.js -- connect -->

The dedicated CLI page and the legacy `POST /cli` adapter execute through
`executeTerminalCommand`. The page layout renders no integrated command bar.
The retained adapter answers HTMX requests, while the CLI page reads a JSON
envelope and offers the browser content picker when a load needs local input. Configuration commands other than
commit and discard call `Editor.ExecuteConfigCommand`, which uses the existing
CLI model's command producers. This includes `show` sources and pipes, ordered
leaf-list insertion, list-aware navigation, display options, session visibility
and rollback. Commit and discard keep the web manager's daemon publication and
confirmed-commit coordination. Operational commands use the authenticated
caller's daemon dispatcher; no SSH connection is opened.

Display and session controls follow the CLI model's Enter behavior. `clear`
restores the current candidate view in configuration mode and empties the
operational output; neither form dispatches a daemon command or drops changes.
Only the exact `configure` control enters configuration mode from operational
mode. Trailing words or pipes retain normal command refusal, and configuration
mode refuses `configure` without changing its candidate.
`top clear` and configuration-mode `run clear` retain the current position.
Configuration `exit` switches to operational mode; operational `exit` and
`quit` return to `/show/`. A pending candidate produces a JSON `confirmation`
prompt before leaving; only a repeat submission with `confirm=true` proceeds,
and the candidate remains intact. JSON `navigate` carries the graphical
destination. Both modes render help from `cli.EditorHelpEntries`.
The terminal renders a page-local Confirm/Cancel form and repeats the same
submission receipt with `confirm=true` only after Confirm; Cancel leaves the
output, mode and candidate intact. No native browser dialog is involved.

Bare `stop` and `restart`, including their configuration-mode `top` forms, use
the same form before daemon execution. `contract.LifecycleCommand` is the one
exact-word mapping to `request shutdown` and `request restart`. After Confirm,
the normal authenticated dispatcher owns authorization and accounting, and its
transport-completion action runs after the accepted JSON answer is written.
Confirmation grants no authority. These controls and display `clear` bypass
stream discovery, while an operational command beginning with `clear` retains
normal dispatch. A restart remains the daemon's registered restart operation;
it does not request a host reboot.

<!-- source: internal/component/web/cli_controls.go -- executeTerminalControl, terminalEditorHelp -->
<!-- source: internal/component/cli/contract/lifecycle.go -- LifecycleCommand -->
<!-- source: internal/component/web/cli_stream.go -- cliStreamInput -->

The graphical YANG editor's selected-item delete and discard buttons call those
same candidate producers. Discarding the selected path preserves pending changes
elsewhere; its audit detail contains only the dropped subset. A removed entry
returns to its parent collection. The controls are absent for read-only users,
and forged mutation requests still pass the existing authorization gate.

<!-- source: internal/component/web/graphical_actions.go -- HandleGraphicalAction, graphicalActionCommand, graphicalSurvivingPath -->

Registered operational commands take precedence over automatic entry into the
configuration editor. For example, operational `delete agent skill` reaches
the daemon, while an unregistered operational `delete` of a configuration path
enters config mode. In config mode, `delete` remains an editor operation and
`run` selects an operational command. The shared CLI mode predicate makes the
same decision for mutation authorization and stream routing; daemon dispatch
still applies the authenticated caller's policy.

Operational completion overlays the current plugin registry on each request.
Live builtin and plugin argument values come from `Server.WireArgCompletion` on copied
paths, leaving the shared tree immutable. `GET /cli/complete` accepts `input`,
`path` and `mode`; its normal response is the completion array. With
`explain=true`, it returns `{ "explanation": "...", "found": true }` from the
shared command completer's `Explain`. Unknown input and commands without a long
explanation return an empty explanation and `found=false`. Authentication,
input bounds and path validation apply to both forms.

The operational source is bound after authentication to the request context,
username, remote address and accepted policy generation. It asks the same
system argument-completion RPC as SSH and the attached console. Permission
denial returns HTTP 403, cancellation 408 and a query failure 502 (504 for a
deadline), preserving the failure rather than claiming no runtime values exist.
Unregistered editor conveniences use `ShouldEnterConfigMode` for completion as
they do for Enter.

Completion preserves an actionable producer refusal, including its recovery
action. A plain transport failure gains browser context and a recovery action;
its existing permission, cancellation or timeout HTTP status is preserved.
Invalid submitted modes, stream paths, unauthenticated history reads and web
request admission failures also return the problem, its effect and a final
browser recovery action, with their existing HTTP statuses.

In configuration mode, completion and explanation share the CLI's `LineBehindTop`
selector. Repeated `top` prefixes select the root without changing the raw
argument tail. Root-selected schema completion clears the current path before
reading the same user's candidate tree. `top run` retains the request-bound
operational source used by ordinary `run`.
<!-- source: internal/component/cli/model_commands.go -- LineBehindTop -->
<!-- source: internal/component/web/cli.go -- completeCLIInput, explainCLIInput -->

The terminal checks HTTP status before decoding candidates. A failed query shows
the returned correction in its feedback area. `?` asks the shared explanation
endpoint for the input a selected candidate would complete, or the completed
current input; Tab also reveals that explanation after completions are exhausted.
Explanation queries do not edit or execute the command. Escape dismisses an
explanation before its underlying menu. Input, path, mode and a request sequence
prevent a late completion or explanation response from replacing newer state.
Long help and multiline corrections preserve their line breaks; the help region
is bounded and scrollable.

<!-- source: internal/component/web/assets/cli.js -- completionResponse, initTerminal -->
<!-- source: internal/component/web/component_cli_terminal.templ -- cliConfirmPanel -->

<!-- source: internal/component/cli/model_mode.go -- ShouldEnterConfigMode, IsConfigureCommand -->
<!-- source: internal/component/plugin/server/command.go -- RecognizesCommand -->
<!-- source: cmd/ze/hub/web_completer.go -- Complete, Explain -->
<!-- source: internal/component/plugin/server/arg_completion.go -- WithArgCompletion -->
<!-- source: internal/component/web/cli.go -- HandleCLICompleteWithCommandCompleter -->

The operational `set cli format` command and its completion are produced by
the same CLI functions as SSH. Its completion is offered only while the browser
is in operational mode, including prefixes that also select schema completion.
Configuration mode offers the shared editor's candidates at the current path,
so `set` remains a sole command match before `?` reveals that path's children.
The browser stores only the validated override
in the authenticated user's existing editor session, under its session lock.
An empty override inherits the configured default; reading or changing a
session preference never writes the process environment. Finite operational
answers, including config-mode `run`, pass the override into the existing pipe
processor, where an explicit format pipe takes precedence. Raw streaming
handlers keep their existing text and pipe policy.

The primary CLI projects the caller-local pipe catalog into native controls.
The supported `save` binding offers **Download** for the current successful
finite result. Its `output-complete` field follows the shared editor error,
dispatcher response and pipe-application outcome; the browser never guesses
success from output text. A successful document beginning with `error:` is
still a successful document. Confirmation, content selection, navigation and
refusals do not offer an export. A new request immediately invalidates the
previous result, and downloading never dispatches a second command.

The finite JSON answer keeps display text separate from export bytes.
`output-byte-count` measures only the final selected format before JSON encoding.
Operational warnings remain above the displayed answer and do not enter the
export bytes or consume its size budget. A complete result of at most 4 MiB
also carries `output-bytes`, using the command owner's Blob document with `utf-8` or `base64`
encoding. A successful empty answer has a zero count and a complete empty Blob.
The terminal response stores that owner-encoded document as raw JSON,
so Go callers can decode the response without treating the Blob object as a
byte slice. If encoding fails, the endpoint returns HTTP 500 with the cause and
states that the command already ran; the operator must inspect current router
state before deciding whether to repeat it. No successful complete-result
answer is emitted. The browser checks the count, encoding and decoded length before
offering Download; a missing or malformed carrier never falls back to display
text. An oversized result has an explicit refusal, and no silent prefix can be
downloaded. Each replacement and page exit revokes the previous object URL.
The browser negotiates the byte-exact raw-output frames below and retains at
most 4 MiB of original producer bytes separately from its visible tail. It offers
Download only after a successful terminal frame and clean EOF; producer error,
Stop, missing completion, malformed framing or a limit refusal leaves no complete
artifact. Binary output is saved without a decode/re-encode round trip.
An interactive monitor instead offers a **Current frame snapshot**, containing
only the latest displayed frame. Each replacement frame revokes its previous
download URL; a snapshot never claims to contain the monitor's complete history.
Legacy plaintext responses remain visible without a complete-result download.

<!-- source: internal/component/web/cli_download.go -- cliLocalOutputControls -->
<!-- source: internal/component/web/assets/cli.js -- initTerminal -->

<!-- source: internal/component/cli/session_format.go -- ExecuteSessionFormat, SessionFormatCompletions -->
<!-- source: internal/component/web/cli_format.go -- sessionFormatCommand, executeTerminalOperationalForUser -->

<!-- source: internal/component/web/cli.go -- dispatchCLICommand -->
<!-- source: internal/component/web/cli_terminal.go -- executeTerminalCommand, executeTerminalConfigMode, executeTerminalOperational -->
<!-- source: internal/component/web/cli_editor.go -- executeTerminalEditorCommand -->
<!-- source: internal/component/cli/editor_transport.go -- ExecuteConfigCommand -->

`load terminal absolute|relative merge|replace` opens a paste editor. A `load
file` command opens a browser file chooser: the filename is a label, never
permission to read a file on the daemon. The selected or pasted bytes are posted
as `content`; parsing and candidate merge/replace use `Model.applyLoad` and the
editor's atomic load implementation. Loading changes the candidate and still
requires commit. Content is limited to 4 MiB. Display preferences stay with the
authenticated user's editor. The browser queues commands so each line uses the
position returned by the previous line, and a quoted `?` stays part of its value.

<!-- source: internal/component/cli/editor_transport.go -- ExecuteConfigCommand -->
<!-- source: internal/component/cli/model_load.go -- applyLoad -->
<!-- source: internal/component/web/editor.go -- ConfigCommand -->

Read-only inspection of caller-held configuration is separate from candidate
loading. The config owner registers `show config supplied` operations with
generic `registry.Meta.SuppliedInput` metadata and YANG-bounded `content`
arguments. The primary Finder's **Inspect supplied configuration** disclosure
derives its choices, input count and comparison composition from those
descriptors, and its help, selections and limits from the actual command tree.
It sits outside **Edit**, so a read-only account can inspect when its command
policy allows the selected operation. `GET /config/inspect/form` updates the
native input controls; authenticated, same-origin `POST /config/inspect`
dispatches exactly one currently offered registered command.

Each source accepts a native file selection or pasted text. The adapter refuses
duplicate or unknown fields, conflicting file/paste input and invalid UTF-8 in
comparison sources before JSON composition. Files stay in memory within the
4 MiB form-body limit. The single encoded command has the owner's smaller
shared input bound, including both comparison sources and their JSON framing.
An uploaded filename becomes a basename diagnostic label, never a daemon path.
Neither endpoint records CLI history or opens an editing session. The current
display preference is read without creating a candidate.

Answers use the browser CLI's existing operational formatter with the original
request context and accepted authorizer. A failed validation retains its typed
diagnostic payload beside the original error; displaying that payload does not
mark the result successful or complete. Authorization failures withhold payload
data. The file/paste `load` controls above retain their own candidate staging
behavior and larger input budget.
<!-- source: internal/component/config/cli/register.go -- registerSuppliedCommands -->
<!-- source: internal/component/command/registry/registry.go -- SuppliedInput -->
<!-- source: internal/component/config/cli/config_supplied.go -- suppliedHandler -->
<!-- source: internal/component/web/graphical_supplied_commands.go -- suppliedInspectionCommands, newSuppliedInspectionCommand -->
<!-- source: internal/component/web/graphical_supplied.go -- HandleSuppliedInspectionForm, HandleSuppliedInspection, readSource -->
<!-- source: internal/component/web/cli_format.go -- currentCLIFormat -->
<!-- source: internal/component/web/cli_terminal.go -- executeTerminalOperationalResult -->

The dedicated browser CLI submits through `/cli/stream`. A registered monitor
streams flushed text from the daemon's existing streaming handler; ordinary
commands retain the terminal's JSON response. Authentication, the accepted
authorization policy and command accounting carry through to the producer.
Stop, Ctrl+C, a new submitted command or a disconnected browser cancel the
request and release the monitor subscription. A daemon stop cancels it too.
The server bounds producer output to 4 MiB per stream. The browser retains a
visible tail of at most 524,288 UTF-16 code units and marks removed older output.
A failed producer reports its reason. Live pipes use the shared remote stream
validator and event formatter; invalid chains and remote pathname saves are
refused before the producer starts.
The complete download buffer counts original bytes separately from this tail.

<!-- source: internal/component/web/cli_stream.go -- HandleCLITerminalStream, cliStreamWriter.Write -->
<!-- source: cmd/ze/hub/server_stream.go -- serverStreamDispatcher -->

A raw-output consumer opts in with
`Accept: application/x-ze-cli-output+ndjson`. Each newline-delimited JSON data
frame carries base64 `data` and `done: false`, preserving the original bytes
even when a producer write splits UTF-8. After dispatch returns, a final frame
carries `done: true` and an actionable `error` if production failed. Only a final
frame without an error establishes a complete result; cancellation or EOF
without that frame leaves the result incomplete. Producer text is never parsed
for an error prefix. A refusal before output retains its HTTP error status,
including 403 for denied authorization. A command the streaming dispatcher does
not handle retains its finite JSON response.

Raw frames contain at most 32 KiB of producer bytes and 64 KiB of encoded JSON;
the 4 MiB budget counts producer bytes before base64 encoding. Each frame write
has a five-second socket deadline. Write, flush and deadline failures cancel
production and prevent a successful terminal frame. The request retains no
server transcript or output queue. Clients that do not request this media type
retain the flushed plaintext response and its visible error trailer.

<!-- source: internal/component/web/cli_output.go -- acceptsCLIOutput, cliOutputWriter.Write, cliOutputWriter.finish, cliOutputWriter.frame -->

An interactive consumer opts in with
`Accept: application/x-ze-cli-view+ndjson`. A matching registered CLI view uses
the same owner selector and `RunBrowserView` model as the terminal. Each
newline-delimited JSON frame carries an opaque `handle`, increasing `sequence`,
plain `text`, owner-declared `controls` (`id` and `label`) and `done`. A failed
request that already wrote frames adds an actionable `error` in a final frame.
The dedicated browser CLI offers this media type on every command request; the
server's registered view selector decides whether it applies. Ordinary commands
retain JSON, and other live consumers retain flushed text. The browser displays
only the latest rich frame and clones the button markup from a templ template,
setting owner labels and frame text with `textContent`. Controls post only their
own handle and declared action to `/cli/view/action`, await the response and
show a producer refusal in the feedback area. Stop stays usable while another
action is pending. No protocol-specific selection or sorting lives in JavaScript.

The browser checks the request generation, handle and increasing sequence before
projecting a frame. It decodes split UTF-8 chunks incrementally and limits each
decoded line to the transport's 64 KiB frame budget. End, Stop, abort, a new
command or page removal clears all controls and releases the response reader.
A final structured producer error remains visible when updates end.

Raw and interactive streams share `serverStreamCommand` admission: registry and
schema checks, remote pipe validation, original command authorization and START/STOP
accounting happen before a view factory or handle opens. The hub binds the same
factory composition used by SSH. Dashboard polls and detail requests use the
original accepted caller, policy and request context through the dispatcher;
the web adapter reads no protocol state. Direct probe factories retain their
existing context-cancellation contract.

Raw piped streams use `command.RunStreamPipes`, the same newline event adapter
as SSH exec. It formats each complete event once, retains split events up to
4 MiB and refuses a larger event without truncation. Unpiped raw streams retain
the producer's original bytes. Rich views retain the full command and apply
their registered model's stream pipes.
<!-- source: internal/component/command/pipe_stream.go -- RunStreamPipes, streamPipeWriter.Write -->
<!-- source: cmd/ze/hub/server_stream.go -- serverStreamCommand -->

The authenticated, same-origin `/cli/view/action` form accepts exactly one
`handle` and one `action`. Only a currently declared control belonging to that
user is admitted. Unknown or ended handles return 410, another user's handle
returns 403, malformed or undeclared actions return 400, and a full queue returns
429. A new action request cannot supply a command, config path or replacement
policy. Two tabs receive distinct handles. Stop cancels at admission even when
the queue or a detail RPC is busy.

The server holds at most 64 active handles, eight pending actions per request
and one latest frame per runner. Each encoded frame is limited to 64 KiB,
action forms to 1 KiB and each socket write to five seconds. Disconnect, write
failure, Stop and daemon shutdown cancel the runner; its owned workers join
before the stream removes its handle. No monitor history or durable view
manager is added.

<!-- source: internal/component/web/cli_view.go -- CLIViewTransport, Action, stream, run, write -->
<!-- source: cmd/ze/hub/server_stream.go -- serverStreamAdmission, serverStreamCommand -->
<!-- source: cmd/ze/hub/view_factory.go -- injectViewFactories, dashboardFactoryFromExecutor -->
<!-- source: cmd/ze/hub/view_web.go -- webViewBinding -->
<!-- source: internal/component/web/assets/cli.js -- initTerminal -->

## YANG Schema Integration

The YANG schema drives the entire UI. No hardcoded field lists.

| Schema element | UI rendering |
|---------------|-------------|
| `ContainerNode` | Sidebar heading (clickable, navigable) |
| `ListNode` | Sidebar heading + entry list + add form |
| `LeafNode` type `TypeBool` | Toggle button (on/off) |
| `LeafNode` type `TypeUint16/32` | Number input with min/max |
| `LeafNode` type `TypeIP/IPv4/IPv6` | Text input with pattern validation |
| `LeafNode` type `TypeString` with `Enums` | Select dropdown |
| `LeafNode` type `TypeString` | Text input |
| `MultiLeafNode`, `BracketLeafListNode`, `ValueOrArrayNode` | Member rows with text inputs or schema enum selectors; add, remove, reorder, save and clear actions |
| `FreeformNode` | Entry name and value rows with add, remove, save and clear actions |
| `InlineListNode` | Keyed entry navigation, a typed add form and the entry's schema-backed fields |
| `LeafNode.ShortHelp` | Three renderings of one string. The (i) tooltip on hover, on the field label and the sidebar heading. The `title=` attribute of the label and the input. The placeholder of an input whose leaf is unset and has no default (`fieldPlaceholder`, `view.go`). On the workbench editor it is also the editor's `aria-describedby` target, which is the tooltip element itself (`fieldDescriptionID`, `view.go`) |
| `ContainerNode.ShortHelp` | (i) tooltip on sidebar heading |
| `ListNode.ShortHelp` | (i) tooltip on sidebar heading |
| `LeafNode.Description` | The long explanation, as a block under the editor: `<p class="config-help">` in the config view (`leafInput`) and `<p class="ze-field-help">` in the workbench editor (`fieldWrapper`). A leaf that declares none renders no block. The tooltip keeps the summary, so a multi-line explanation never lands in a `title=` attribute |
| `ContainerNode.Description`, `ListNode.Description` | The long explanation, as a `<p class="config-help">` block in the config view: at the top of the container view (`configContainer`, before the child list) and under the `Entries` heading of the list view (`configList`). `buildConfigViewData` (`handler_config_walk.go`) copies it into `ConfigViewData.Description`. A node that declares none renders no block. The sidebar heading keeps the summary |

<!-- source: internal/component/config/schema.go -- LeafNode, ContainerNode, ListNode -->

A union leaf's `LeafNode` also lists its member types in `Union`, in declared
order. A value the editor submits is judged against each member in turn and is
accepted at the first that takes it (RFC 7950 Section 9.12); a value no member
takes is refused. For enum members, the refusal lists their accepted values
and help; other member types are named in words.
<!-- source: internal/component/config/schema.go -- UnionMember -->
<!-- source: internal/component/config/leaf_value.go -- validateUnionValue -->
<!-- source: internal/component/web/fragment.go -- buildFieldMeta, nodeDescription -->
<!-- source: internal/component/web/handler_config_leaf.go -- buildLeafField -->
<!-- source: internal/component/web/handler_config_walk.go -- buildConfigViewData -->
<!-- source: internal/component/web/view.go -- fieldPlaceholder -->

The live Finder detail path handles every node above, including a direct leaf
URL, which reads its current candidate value. Collection saves use the shared
editor's atomic load replacement under the user's session lock. Invalid enum,
range, length or pattern values refuse the save; repeated members are accepted
only for a multi-word value or a schema-declared sequence. Leaf-list inactive
member markers follow the rows when reordered. A multi-word leaf supports only
whole-setting activation through Configuration actions. Stored secret scalar
values still pass through `maskSecretLeaf` before reaching an input.
<!-- source: internal/component/web/fragment.go -- buildFragmentData, populateFragmentFields -->
<!-- source: internal/component/web/handler_graphical_values.go -- validateGraphicalValues, replaceGraphicalValues -->

## TLS

Self-signed ECDSA P-256 certificate, valid 365 days. `WebCertHosts` is the one declaration of the SAN set: localhost, the two loopback addresses, the listen address, and any extra name the caller gives. When listening on `0.0.0.0`, all non-loopback interface IPs are added so the cert is valid regardless of which IP the client connects to. The appliance build host takes the same list and has its certificate authority issue the leaf instead (`architecture/appliance/builder.md`).

Certificates are persisted in the configuration store (`meta/web/cert`, `meta/web/key`) via the `CertStore` interface. On restart, the existing cert is loaded instead of regenerated, so browsers don't need to re-accept.

TLS handshake errors from browsers rejecting self-signed certs are suppressed in the server error log.

<!-- source: internal/core/selfcert/selfcert.go -- GenerateWebCertWithAddr, LoadOrGenerateCert, WebCertHosts, CertStore -->

## Security Headers

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Strict-Transport-Security: max-age=63072000; includeSubDomains
Cache-Control: no-store
X-Ze-Version: ze/26.04.05 (ac8f5391; go1.26; linux/amd64)
```

No `unsafe-eval`. All scripts are external files. No inline `<script>` blocks.

`X-Ze-Version` is the one header an operator can turn off. `environment {
hide-version true; }` sets `ze.hide-version`, and both HTTP servers read it
through `version.HTTPHeaderHidden` before they write the banner. Every other
header above is unchanged by it, and Ze sends no `Server` header.

<!-- source: internal/component/web/auth.go -- addSecurityHeaders -->
<!-- source: internal/component/lg/server.go -- securityHeaders -->
<!-- source: internal/core/version/version.go -- HTTPHeaderHidden -->

## Starting the Web Server

| Method | Command |
|--------|---------|
| CLI flag | `ze start --web <port>` |
| Config | `environment { web { enabled true; server main { ip 0.0.0.0; port 3443; } } }` |
| Env vars | `ze.web.listen=ip:port`, `ze.web.enabled=true`, `ze.web.insecure=true`, `ze.web.ui-mode=workbench` (explicit experimental Workbench selection) |

Configured and web-only startup call `startWebServer` in
`cmd/ze/hub/service_web.go`. Web-only mode starts the server without a BGP
config for initial setup.
<!-- source: cmd/ze/hub/service_web.go -- startWebServer -->

With no config path, `startWebServer` takes the config name from `resolve.DefaultConfig`. When that call errors (the instance name exists but cannot be read), the server is not started: it prints `warning: web server disabled:` and the error to stderr and returns no server, rather than serving `ze.conf`.

Web-only mode stops in two steps. The first Ctrl+C (SIGINT or SIGTERM) prints
"Shutting down (Ctrl+C again to force)...", closes the listener and gives open
connections up to 3 s to finish, then exits 0. A second Ctrl+C during that wait
stops waiting: the process drains its stderr relay (`crashlog.Flush`, at most
500 ms, and at once when no child process holds the stderr pipe), prints
"forced exit" and exits 1. The daemon's own stop has more phases
(`docs/guide/operations.md`); web-only mode has no drain, so it has only these two.
`test/ui/web-only-interrupt-graceful.ci` and `test/ui/web-only-interrupt-force.ci`
drive both steps through `ze start --web-only`. Their driver holds a connection
inside a request handler, so the shutdown has something to wait for, and the
graceful test times the stop against the 3 s deadline.

After the network grace period, shutdown cancels request contexts and closes
remaining connections, including POST streams, then joins admitted handlers.
Only after that join does the service drain command history. After a separate
teardown allowance, a stuck handler or storage call produces an ERROR diagnostic
while the service continues joining. The caller cannot release the borrowed
store before cleanup finishes, including a second queued mode whose write has
not acquired a storage guard yet. A second stop signal or the service-manager
timeout may force process exit. An overdue cleanup never becomes successful
drain merely because its diagnostic timer expired.
The allowance also raises `doctor-web-cleanup-overdue` on the existing report
bus, once per minute across concurrent shutdown callers. Its fixed subject is
web shutdown; it holds no command, username or uploaded content. The daemon's
doctor recent-events check reports it with
`ze show log recent level err component web.server` as the next diagnostic
command. Shared history read and publication failures use
`doctor-cli-history-read` and `doctor-cli-history-write`, including a write that
recovers before a browser polls its status. These events do not replace the
HTTP warning or the strict cleanup join.
<!-- source: cmd/ze/hub/service_web.go -- webService.Shutdown -->

A network shutdown failure separately raises `doctor-web-network-cleanup`
before history cleanup starts. Its fixed subject is `web-network-cleanup`,
and its next command is `ze show log recent level err component web.server`.
It uses the same minute bound. The returned shutdown error still includes
both network and history failures. Startup cleanup and web-only shutdown
rely on each failure's producer instead of logging the joined error again.
<!-- source: cmd/ze/hub/register_web_history_diagnostic.go -- reportWebNetworkCleanupFailure -->
<!-- source: cmd/ze/hub/service_web.go -- webService.Shutdown, runWebOnly, startWebServer -->

The same owner handles startup failure and normal/web-only shutdown.
<!-- source: internal/component/web/server_lifetime.go -- requestOwners, serveRequest -->
<!-- source: cmd/ze/hub/service_web.go -- webService.Shutdown -->
<!-- source: cmd/ze/hub/service_web.go -- runWebOnly, forceExitOnSignal -->
<!-- source: internal/test/fixture/web_interrupt_fixture.go -- webInterrupt, webInterruptHold -->

## Web modes

The graphical YANG editor is the default UI, internally named `finder`. It
provides schema navigation and graphical configuration controls. The top bar
offers **YANG editor**, **CLI** and **Workbench (experimental)**. CLI opens the
authenticated browser terminal at `/cli`, without requiring SSH access.

The unfinished RouterOS-style Workbench is an explicit alternative. Select it
in the top bar or set `ze.web.ui-mode=workbench` before startup. Set
`ze.web.ui-mode=finder` to select the YANG editor explicitly. An empty or unknown
mode selects the YANG editor. The current `ze-ui-mode` switch cookie preserves
an operator's explicit selection; the retired `ze-ui` cookie is ignored.

### Experimental Workbench layout

| Region | Source |
|--------|--------|
| Top bar (identity, breadcrumb, actions) | `component_workbench_topbar.templ` |
| Left nav (Dashboard / Routing / Logs / ...) | `component_workbench_nav.templ` driven by `workbench_sections.go` |
| Workspace (table + detail) | reuses the existing `detail` fragment so list tables and fields render unchanged inside the new chrome |
| Tool overlay container `#tool-overlays` | `component_tool_overlay.templ`. HTMX `hx-swap="beforeend"` appends each instance as a sibling, so several overlays can pin |
| Commit bar / diff modal / error panel | reused unchanged from Finder; CLI bar removed (available as `/cli` tab) |

### Related-tool execution

`POST /tools/related/run` accepts only `tool_id` and `context_path` (plus an optional `confirm=true` for destructive tools). Raw command text is never trusted from the form. The handler:

1. Walks the schema to the context node and looks up the descriptor by id.
2. Returns a confirmation overlay if the descriptor declares `confirm=...` and the operator has not confirmed.
3. Resolves placeholder substitutions against the user's working tree via `related_resolver.go` (rejects unsafe values, caps depth at 16 segments, caps resolved command at 4096 chars).
4. Dispatches via the standard `CommandDispatcher(command, username, remoteAddr)` so authz, accounting, and peer-selector extraction live in one place.
5. ANSI-strips and 4-MiB-truncates output, splits the first 128 KiB inline and the rest into a `<details>` "Show full output" disclosure, HTML-escapes every leg.

See `spec-web-2-operator-workbench.md` (Argument Wire Format, Resolved-Value Validation, Day-One BGP Related Tools) for the full descriptor grammar and the BGP YANG annotations that ship with the experiment.


## Looking Glass

The looking glass is a separate HTTP server (`internal/component/lg/`) that provides public, read-only access to BGP state. It runs on its own port (default 8443), independent from the authenticated web UI.

All source files in `internal/component/lg/` reference this document via `// Design:` comments.

### LG Source Files

| File | Responsibility |
|------|---------------|
| `server.go` | HTTP server lifecycle, mux setup, TLS, CommandDispatcher. `NewLGServer` builds the one handler chain: security headers over `errorfragment.Middleware` over the bearer gate over the mux |
| `handler_api.go` | Birdwatcher-compatible REST API (JSON, snake_case), input validation |
| `handler_ui.go` | HTMX pages (peers, lookup, search), SSE events, asset serving |
| `handler_graph.go` | AS path topology SVG endpoint |
| `graph.go` | Graph data model (nodes/edges from AS paths), prepending dedup |
| `layout.go` | Layered layout algorithm, SVG rendering |
| `render.go` | templ component rendering, page vs fragment detection |
| `view.go` | One named view-model struct per page, which the components take |
| `*.templ` | The markup, compiled into `*_templ.go` by `./le repo update` |
| `embed.go` | Embedded assets (CSS, HTMX, SSE) via go:embed |
| `auth.go` | Optional bearer-token gate over the whole mux |

<!-- source: internal/component/lg/server.go -- LGServer, NewLGServer -->
<!-- source: internal/component/lg/handler_api.go -- API handlers, birdwatcher transform -->
<!-- source: internal/component/lg/handler_ui.go -- UI handlers, SSE -->
<!-- source: internal/component/lg/graph.go -- buildGraph, extractASPath -->
<!-- source: internal/component/lg/layout.go -- computeLayout, renderGraphSVG -->

### LG TLS

For self-signed TLS, the hub reads existing certificate and key material from
the daemon's store through `LoadOrGenerateCert`. For `ze start <file>`, that
store belongs beside the explicit config file. A `ze init` store in another
directory does not supply that listener's certificate.
<!-- source: cmd/ze/hub/service_lg.go -- buildLGService -->
<!-- source: cmd/ze/hub/cert_store.go -- blobCertStore -->
<!-- source: internal/core/resolve/resolve.go -- StoreDir, StorageFor -->
<!-- source: internal/core/selfcert/selfcert.go -- LoadOrGenerateCert -->

`NewLGServer` takes the PEM material from its caller: a self-signed certificate,
or a leaf-then-intermediates chain from the PKI store. It puts the parsed pair
behind `tls.Config.GetCertificate` and clears `Certificates`, so `crypto/tls`
reads the certificate once for each handshake instead of once for each listener.

`UpdateTLSCertificate` replaces that pair on a running server. A bound listener
is untouched, so an open connection survives the rotation and the next handshake
serves the new chain. Material that does not parse is refused and the previous
certificate keeps serving. The contract every PKI-serving listener follows is
`docs/architecture/pki/tls-listeners.md`.

<!-- source: internal/component/lg/server.go -- getCertificate, UpdateTLSCertificate -->

### LG Refused Requests

A handler refuses with `http.Error`, which writes a bare status line. The chain
in `NewLGServer` carries `errorfragment.Middleware`
(`internal/core/errorfragment`), so a request carrying `HX-Request` receives the
same error fragment the web UI answers with, and htmx can swap it into the
element the request named. Everything else is untouched: the JSON API refuses
through `writeJSONError`, and a plain browser or a script still reads the status
line. `.error-fragment` in `assets/style.css` is what an operator sees.

### LG Bearer Token

The looking glass is open by default, because it is a public read-only surface.
When `token` is set, `bearerAuth` wraps the mux before the security headers, so
a route added later is gated by construction. A request with no
`Authorization` header, a different scheme, or a wrong token gets `401` with
`WWW-Authenticate: Bearer realm="looking glass"`. The compare is
`subtle.ConstantTimeCompare` over SHA-256 digests, so neither the token length
nor its content leaks through timing. RFC 7235 Section 2.1 makes the scheme
case-insensitive, so `bearer <token>` is accepted. The token itself stays
case-sensitive.

<!-- source: internal/component/lg/auth.go -- bearerAuth, bearerTokenMatches -->

### LG URL Scheme

```
/api/looking-glass/status              Router status (JSON, birdwatcher format)
/api/looking-glass/protocols/bgp       Peer list (JSON)
/api/looking-glass/routes/protocol/X   Routes peer X sent Ze (JSON)
/api/looking-glass/routes/export/X     Routes Ze sent peer X (JSON)
/api/looking-glass/routes/table/X      Best routes by family (JSON)
/api/looking-glass/routes/filtered/X   Filtered routes per peer (JSON)
/api/looking-glass/routes/search?prefix=X  Prefix lookup (JSON)
/lg/peers                              Peer dashboard (HTML)
/lg/lookup                             Route lookup form (HTML)
/lg/search                             Unified search: prefix, AS path, community (HTML)
/lg/peer/{address}                     Routes the peer sent Ze, with the received and sent counts side by side (HTML)
/lg/route/detail                       Route detail fragment (HTMX)
/lg/graph?prefix=X                     AS path topology (SVG)
/lg/events                             SSE peer state stream
/lg/assets/                            Static CSS/JS
```

### LG Data Access

All BGP data is queried via `CommandDispatcher` (same `func(string) (string, error)` as the web UI's admin handlers). The LG never imports RIB or peer plugin packages. The dispatcher routes commands to the engine, preserving plugin isolation.

### LG JSON Convention Exception

The birdwatcher API uses `snake_case` JSON keys (`router_id`, `neighbor_address`, `routes_received`) instead of Ze's standard `kebab-case`. This is intentional for compatibility with Alice-LG and other birdwatcher consumers.

### Starting the Looking Glass

| Method | Config |
|--------|--------|
| Config | `environment { looking-glass { enabled true; server main { ip 0.0.0.0; port 8443; } } }` |
| Env vars | `ze.looking-glass.listen=ip:port`, `ze.looking-glass.enabled=true`, `ze.looking-glass.tls=false` (TLS is on by default), `ze.looking-glass.token=<token>` |

The hub builds the looking-glass service with `buildLGService` and starts its
server with `serveLG` in `cmd/ze/hub/service_lg.go`.

<!-- source: cmd/ze/hub/service_lg.go -- buildLGService, serveLG -->
