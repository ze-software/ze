# Spec: doctor-daemon-route

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P1 |
| Release reason | Doctor must run plugin checks concurrently in deterministic order without hanging on a slow SSH daemon exchange or leaving contradictory protocol docs. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The local `ze doctor` route to a running daemon (originally designed here on
2026-10-15 after Thomas's ruling "route to the daemon if running") has already
landed on `main` via `internal/component/doctor/local_route.go`
(`answerLocalWith`, `askDaemonDoctor`, `askDaemonOverSSH`,
`doctor-runtime-not-checked`), `internal/component/doctor/answer.go`,
`internal/component/doctor/cmd/show.go` (`HandleDoctor`),
`internal/component/doctor/yang/ze-doctor-cmd.yang` (with `leaf config` and
`leaf kernel-capabilities`), and `docs/architecture/doctor-and-health-checks.md`
("The local route asks the running daemon first").

Three concrete defects from the original specification remain open at HEAD:

1. **Serial plugin doctor checks and non-deterministic ordering (formerly AC-5/AC-6).**
   `collectDoctorChecksFrom` in `internal/component/doctor/cmd/show.go` still
   walks every plugin's declared Stage-1 doctor checks in a serial loop, each
   under a 5 s `perCheckTimeout`, and sorts only by `check.Order` over a Go map
   iteration. With multiple plugins declaring checks, worst-case latency grows
   linearly with plugin count, a single hung plugin delays all plugins behind
   it, and checks with equal `Order` appear in random order.
2. **Unbounded SSH handshake, authentication, and command execution on the local route.**
   `askDaemonOverSSH` in `internal/component/doctor/local_route.go` calls
   `sshclient.ExecCommandRaw`, which calls `dialDaemon` (`ssh.ClientConfig.Timeout`
   bounds TCP dial only, leaving SSH key exchange and password authentication
   unbounded) and `session.Output` with no deadline. A stalled handshake or
   unresponsive daemon hangs `ze doctor` indefinitely instead of falling back to
   local checks with `doctor-runtime-not-checked`.
3. **Stale documentation contradicting the shipped daemon route.**
   `docs/architecture/api/process-protocol.md` (both the Stage-1 `Doctor Check
   Declaration` section and the `doctor-check` callback description) and
   `docs/guide/health-checks.md` (`External plugins (RPC)`) still state that
   offline `ze doctor` does not reach or invoke plugin doctor checks.

This spec narrows cleanly to those three remaining items and preserves the
shipped `local_route.go` / `doctor-runtime-not-checked` / `HandleDoctor` design
unchanged.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - readiness tiers, the `runtime` phase, and "The local route asks the running daemon first"
  → Constraint: `ze doctor` already asks the running daemon for `show doctor | raw` via `answerLocalWith` / `askDaemonOverSSH` and adds one `doctor-runtime-not-checked` warning when no daemon answers; this spec preserves that contract and does NOT introduce a `show doctor runtime` subcommand or `DoctorResult.runtime` field.
  → Decision: document concurrent per-plugin-generation doctor workers, deterministic `(Order, plugin, check)` output ordering, and the 22 s bounded SSH ask in this page.
- [ ] `docs/architecture/api/process-protocol.md` - Stage-1 `doctor-checks` declaration and `doctor-check` callback
  → Constraint: lines 966 and 2152 still claim offline `ze doctor` does not invoke or reach plugin doctor checks; both must be updated to describe the shipped local route and the `RunDoctorChecks` worker path.
- [ ] `docs/guide/health-checks.md` - operator guide for `ze doctor` and `External plugins (RPC)`
  → Constraint: lines 299-300 repeat the stale claim that offline `ze doctor` does not invoke plugin checks; update to agree with `docs/architecture/doctor-and-health-checks.md`.
- [ ] `docs/architecture/api/commands.md` - `// Design:` page of `internal/component/doctor/cmd/show.go`
  → Constraint: `show doctor` remains one YANG command path (`ze-doctor:show-doctor`) and pipe behavior stays unchanged.
- [ ] `docs/architecture/api/ipc_protocol.md` - `// Design:` page of `internal/component/cli/sshclient/answer.go`
  → Constraint: exec-channel answer framing (`EnvAnswerFrame`, `readAnswerFrame`) is preserved unchanged by context-bounded execution.
- [ ] `docs/architecture/system-architecture.md` - `// Design:` page of `internal/component/cli/sshclient/client.go`
  → Constraint: `ExecCommandRaw` remains the JSON raw-pipe SSH transport; adding a context-bounded sibling preserves all existing callers.
- [ ] `ai/rules/goroutine-lifecycle.md` - goroutine ownership and lifecycle rules
  → Constraint: never spawn a goroutine per `show doctor` request; use one long-lived worker per live plugin generation (mirroring `drainCaller` in `internal/component/plugin/server/drain.go`), started and stopped in `superviseProcess`.
- [ ] `ai/rules/user-facing-errors.md` - every operator failure names problem, why, and fix
  → Constraint: queue-wait timeout and SSH ask timeout both name the plugin or daemon address, the bound/cause, and the exact fix command (`ze request plugin restart <plugin>` or `ze cli --user <name> -c "show doctor"`).

**Key insights:**
- `drainCaller` (`internal/component/plugin/server/drain.go`) and `superviseProcess` (`internal/component/plugin/server/failure_policy.go`) already establish Ze's per-generation long-lived worker lifecycle and two-pass fan-out without per-request goroutines.
- The SDK dispatches callbacks serially per plugin (`bridgeEventLoop` and `eventLoop` in `pkg/plugin/sdk/sdk_dispatch.go`), so concurrency belongs across plugins while checks of a single plugin stay serial in declared order.
- `streamCommand` (`internal/component/cli/sshclient/client.go`) already establishes `context.AfterFunc(ctx, func() { client.Close() })` to unblock SSH operations promptly on context cancellation/deadline; combining that with `net.Dialer.DialContext` and a `net.Conn` deadline during `ssh.NewClientConn` bounds TCP connect, handshake, auth, session setup, and command execution under one context.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/doctor/local_route.go` (`answerLocalWith`, `askDaemonDoctor`, `askDaemonOverSSH`, `daemonUnreached`, `decodeDaemonDoctor`, `runtimeNotChecked`) - read 2026-11-26: local `ze doctor` already asks a running daemon for `show doctor` (or `show doctor config <resolved>` when `command.CallerPath` admits the file) via `sshclient.ReachDaemon("")` and `sshclient.ExecCommandRaw(reached.Credentials, line)`. On success it draws the daemon's `DoctorResult`; on failure it runs `runChecks(configPath)` locally and appends one `doctor-runtime-not-checked` warning naming the reason and fix. Gap: `askDaemonOverSSH` calls unbounded `sshclient.ExecCommandRaw` with no overall deadline.
- [ ] `internal/component/doctor/answer.go` (`answerLocal`, `Answer`, `answerDoctor`, `readDoctorRequest`) - read 2026-11-26: `answerLocal` dispatches through `answerLocalWith`; `readDoctorRequest` already enforces keyword-before-value (`config <file>`, `kernel-capabilities`) and refuses bare positional tokens.
- [ ] `internal/component/doctor/cmd/show.go` (`HandleDoctor`, `answerDaemonDoctor`, `doctorCheckSource`, `doctorCheckSourceOf`, `collectDoctorChecksFrom`) - read 2026-11-26: `HandleDoctor` calls `answerDaemonDoctor` which runs `collectDoctorChecksFrom(source, callerName)` before `doctor.RunRuntimeChecks(judged, plugins.codes)`. `collectDoctorChecksFrom` already skips `pluginName == callerName` and filters by `platformMatches`, then sorts only by `pending[i].check.Order < pending[j].check.Order` (leaving equal `Order` non-deterministic across map iteration) and invokes `source.CallDoctorCheck` serially in a loop with `perCheckTimeout` (5 s) each.
- [ ] `internal/component/doctor/yang/ze-doctor-cmd.yang` - read 2026-11-26: revision `2026-11-02` already declares `container show { container doctor { ze:command "ze-doctor:show-doctor"; leaf config; leaf kernel-capabilities; } }`.
- [ ] `internal/component/plugin/server/server.go` (`Server`, `CallDoctorCheck`, `DoctorCheckPlugins`) - read 2026-11-26: `Server` already tracks per-generation `drainCallers map[*process.Process]*drainCaller`; `CallDoctorCheck` calls `conn.SendDoctorCheck` directly with only one production caller (`internal/component/doctor/cmd/show.go`).
- [ ] `internal/component/plugin/server/drain.go` (`drainCaller`, `startDrainCaller`, `DrainRound.Run`) and `internal/component/plugin/server/failure_policy.go` (`superviseProcess`) - read 2026-11-26: `superviseProcess` starts a per-generation worker before serving RPCs and stops and joins it before `applyFailurePolicy`, and `DrainRound.Run` fans out across workers without spawning per-request goroutines.
- [ ] `internal/component/cli/sshclient/client.go` (`ExecCommand`, `ExecCommandRaw`, `streamCommand`, `ProbeDaemon`) and `internal/component/cli/sshclient/answer.go` (`dialDaemon`) - read 2026-11-26: `dialDaemon` uses `ssh.Dial` with `Timeout: dialTimeout` (10 s, TCP connect only, handshake and auth unbounded); `ExecCommand` / `ExecCommandRaw` take no `context.Context` and set no deadline on `NewSession`, `Setenv`, or `Output`.
- [ ] `pkg/plugin/sdk/sdk_dispatch.go` (`bridgeEventLoop`, `eventLoop`) - read 2026-11-26: both internal and external plugin loops process ordinary callbacks (including `ze-plugin-callback:doctor-check`) inline, serially, one at a time.
- [ ] `internal/component/ssh/ssh.go` (`defaultMaxSessions = 8`) - read 2026-11-26: SSH server defaults to 8 concurrent sessions.

**Behavior to preserve:**
- Shipped local-route contract in `internal/component/doctor/local_route.go` and `docs/architecture/doctor-and-health-checks.md`: `ze doctor` asks the daemon for `show doctor | raw` (or `show doctor config <resolved> | raw`), draws the daemon's `DoctorResult` on success, and falls back to local checks plus one `doctor-runtime-not-checked` warning when no daemon answers.
- `ze-doctor-cmd.yang` command tree (`show doctor`, `config <file>`, `kernel-capabilities`), `DoctorResult` JSON schema (`schema-version: 1`, `ready`, `diagnostics`), and `ready` / exit-code semantics.
- `collectDoctorChecksFrom` caller-skip (`pluginName == callerName`), platform filtering (`platformMatches`), truncation at `maxDiagnosticsPerCheck` (64) with `doctor-plugin-excessive-diagnostics`, `doctor.PluginDiagnostic` fix preservation, severity normalization (`normalizeSeverity`), and `ownedCodes` deduplication with `recent-events` (`TestShowDoctorListsAPluginOwnedWarningOnce`).
- Existing `sshclient.ExecCommand` and `sshclient.ExecCommandRaw` signatures and `LoginRefused` detection on handshake/auth errors.

**Behavior to change:**
- Replace the serial `CallDoctorCheck` loop in `collectDoctorChecksFrom` with concurrent per-plugin-generation `doctorWorker` execution owned by `internal/component/plugin/server`, plus deterministic `(Order, plugin name, check name)` sorting.
- Add context-bounded `sshclient.ExecCommandRawContext` (bounding TCP connect, SSH handshake, authentication, session setup, and command output) and call it from `askDaemonOverSSH` with a 22 s deadline.
- Update the stale doctor-check statements in `docs/architecture/api/process-protocol.md` and `docs/guide/health-checks.md`, and document the workers and bounded SSH ask in `docs/architecture/doctor-and-health-checks.md`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Local `ze doctor` daemon route | `internal/component/doctor/local_route.go` (`answerLocalWith`, `askDaemonDoctor`, `askDaemonOverSSH`, `runtimeNotChecked`); `docs/architecture/doctor-and-health-checks.md` ("The local route asks the running daemon first") | Probes and reaches the daemon via `sshclient.ReachDaemon("")`, runs `show doctor \| raw`, draws the daemon's `DoctorResult` on success, or runs local checks plus one `doctor-runtime-not-checked` warning on failure | Extends: `askDaemonOverSSH` switches from unbounded `sshclient.ExecCommandRaw` to context-bounded `sshclient.ExecCommandRawContext` under `daemonDoctorTimeout` (22 s) so a stalled handshake or command degrades cleanly instead of hanging |
| Per-generation plugin worker and fan-out | `internal/component/plugin/server/drain.go` (`drainCaller`, `startDrainCaller`, `DrainRound.Run`) and `internal/component/plugin/server/failure_policy.go` (`superviseProcess`) | Starts one long-lived goroutine per live plugin generation declaring a feature, tracks it by `*process.Process`, fans out requests without per-request goroutines, and stops/joins it before `applyFailurePolicy` | Extends (same pattern): adds a sibling `doctorWorker` (`startDoctorWorker`) in `internal/component/plugin/server/doctor_worker.go` wired into `superviseProcess` for generations declaring `DoctorChecks`, replacing the serial `CallDoctorCheck` path |
| Serial `show doctor` plugin check collector | `internal/component/doctor/cmd/show.go` (`collectDoctorChecksFrom`, `doctorCheckSource`) and `internal/component/plugin/server/server.go` (`CallDoctorCheck`) | Collects plugin checks, skips `callerName`, sorts only by `Order`, and invokes `CallDoctorCheck` serially with 5 s timeout per check | Replaces: replaces `CallDoctorCheck` and the serial loop with `Server.RunDoctorChecks` backed by per-generation `doctorWorker` instances and deterministic `(Order, plugin name, check name)` sorting |
| SSH raw command exec and context teardown | `internal/component/cli/sshclient/client.go` (`ExecCommand`, `ExecCommandRaw`, `streamCommand`) and `internal/component/cli/sshclient/answer.go` (`dialDaemon`, `LoginRefused`) | `ExecCommandRaw` runs `command \| raw` over SSH with `dialTimeout` on TCP connect only; `streamCommand` closes the SSH client via `context.AfterFunc` when its context ends | Extends: adds `ExecCommandRawContext` backed by `execCommandContext` and `dialDaemonContext` (using `net.Dialer.DialContext`, a `net.Conn` deadline over `ssh.NewClientConn`, and `context.AfterFunc` to close the client), while existing `ExecCommand` / `ExecCommandRaw` delegate to it with `context.Background()` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | the door every change reads; covering `internal/component/doctor/` | "A long-lived worker on a channel, never one per event" | agrees | Each live plugin generation declaring `DoctorChecks` owns one long-lived `doctorWorker` on a buffered-16 channel, started and stopped in `superviseProcess`; `show doctor` spawns zero goroutines per request |
| `docs/architecture/api/process-protocol.md` | Engine to Plugin (Callbacks) | `// Design:` and anchors of `server.go`, `failure_policy.go`, `show.go`; modified by this spec | "Engine invokes a plugin's declared doctor check by name." | agrees | Preserves the Stage-1 `doctor-checks` declaration and `ze-plugin-callback:doctor-check` RPC contract while running different plugins' checks concurrently via per-generation workers; updates two stale sentences (lines 966, 2152) that already contradicted shipped `local_route.go` and `docs/architecture/doctor-and-health-checks.md` |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers bus participant topology, delivery filters, and `\| implied`; silent on plugin doctor worker scheduling and SSH client exec deadlines |
| `docs/architecture/api/commands.md` | Dispatch keys are YANG paths | `// Design:` of `show.go`; anchored to `server.go`, `show.go`, `sshclient/answer.go` | "The daemon registers each built-in handler under its YANG command path, derived from the tree" | agrees | `show doctor` remains registered under `ze-doctor:show-doctor` with unchanged YANG path, arguments, and pipe dispatch |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers YANG module loading, validation layers, and config schema publication; silent on plugin doctor worker concurrency and SSH client exec deadlines |
| `docs/architecture/doctor-and-health-checks.md` | The local route asks the running daemon first | `// Design:` of `local_route.go`, `answer_test.go`, `show_unreachable_test.go`; anchored to `server.go`, `failure_policy.go`, `show.go`, `client.go`; modified by this spec | "`ze doctor` answers for the box, and the runtime rows exist only in the running daemon, so the local route asks the daemon before it answers alone" | agrees | Preserves the shipped `local_route.go` / `doctor-runtime-not-checked` design and fix table verbatim (keeping `fixAskAsAccount` (`ze cli --user <name> -c "show doctor"`) for ask/timeout failures), adds the 22 s context deadline to `askDaemonOverSSH`, and documents the concurrent per-generation doctor workers and deterministic sort |
| `docs/architecture/hub-architecture.md` | Overview | anchored to `server.go`, `failure_policy.go` | - | silent | Covers daemon boot sequence and external plugin schema reload rules; silent on plugin doctor worker concurrency and SSH client exec deadlines |
| `docs/architecture/overview.md` | 2. System Architecture | anchored to `internal/component/plugin/server/server.go` | - | silent | High-level Ze architecture overview; silent on plugin doctor worker scheduling and SSH client exec deadlines |
| `docs/architecture/testing/runner-architecture.md` | Layers | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers the `.ci` functional test runner and `ze.test.doctor.probe-timeout` for external probe checks; silent on plugin doctor worker scheduling and SSH client exec deadlines |
| `docs/architecture/update-cache.md` | Purpose | anchored to `internal/component/plugin/server/server.go` | - | silent | Covers BGP UPDATE wire caching and consumer tracking; silent on plugin doctor checks and SSH client exec deadlines |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/server/` | "A compiled-in and an external plugin differ only in how they reach the bus." | agrees | `doctorWorker` starts in `superviseProcess` and invokes `conn.SendDoctorCheck` identically for both internal and forked plugin generations |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `internal/component/plugin/server/failure_policy.go` | - | silent | Covers config verify/apply/rollback transactions; silent on plugin doctor worker scheduling and SSH client exec deadlines |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to `internal/component/plugin/server/failure_policy.go` | - | silent | Covers the ExaBGP wrapper endpoint and command translation; silent on plugin doctor worker scheduling and SSH client exec deadlines |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `internal/component/plugin/server/failure_policy.go` | - | silent | Covers IRR firewall prefix sync and its doctor codes; silent on plugin doctor worker scheduling and SSH client exec deadlines |
| `docs/architecture/system-architecture.md` | Overview | `// Design:` of `internal/component/cli/sshclient/client.go` | - | silent | Legacy hub/orchestrator architecture note; silent on SSH client exec context deadlines and plugin doctor worker scheduling |
| `docs/architecture/api/ipc_protocol.md` | SSH exec channel | `// Design:` of `sshclient/answer.go`; anchored to `client.go`, `answer.go` | "Answer framing on the SSH exec channel is opt-in." | agrees | `execCommandContext` and `ExecCommandRawContext` preserve `EnvAnswerFrame = AnswerFrameEnabled` and `readAnswerFrame` unchanged |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `internal/component/cli/sshclient/client.go` | - | silent | Covers `.ci` functional test syntax and runner directives; silent on plugin doctor worker concurrency and SSH client exec deadlines |
| `ai/doors/commands-cli.md` | Where it sits | the door covering `internal/component/cli/sshclient/` | "A local-data command is answered locally by" | agrees | Preserves `ze doctor`'s local-data registration and `show doctor`'s daemon handler unchanged while bounding the SSH ask in `sshclient` |
| `docs/architecture/api/process-protocol.md` | Design decisions | `// Design:` and anchors of `server.go`, `failure_policy.go`, `show.go`; modified by this spec | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation and changes no plugin authorization profile; reuses existing engine-to-plugin `doctor-check` callback |
| `docs/architecture/api/process-protocol.md` | Design decisions | `// Design:` and anchors of `server.go`, `failure_policy.go`, `show.go`; modified by this spec | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Touches only the per-generation `doctorWorker` request queue for `show doctor` RPCs; engine-to-plugin event delivery remains lossless |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Config rendering and `\| implied` formatting are untouched by this spec |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Bus participant naming and default message registrations are untouched; `doctorWorker` derives only from Stage-1 `DoctorChecks` |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Bus delivery filter `send > to` semantics are untouched |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Bus delivery filter `receive > from` semantics are untouched |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus allow-list enforcement is untouched |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Bus filter compilation and commit validation are untouched |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Bus arbitration is untouched; each plugin runs its own declared doctor checks independently |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `internal/component/plugin/server/server.go` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Bus default/effective display is untouched |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` of `show.go`; anchored to `server.go`, `show.go`, `sshclient/answer.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no subcommand or presentation keyword; `show doctor` keeps its existing command path and `\| raw` remains a pipe operator |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` of `show.go`; anchored to `server.go`, `show.go`, `sshclient/answer.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `show doctor` overrides no generic pipe operator and keeps its existing `DoctorResult` answer format |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` of `show.go`; anchored to `server.go`, `show.go`, `sshclient/answer.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `\| raw` on `askDaemonOverSSH` selects raw JSON rendering of `show doctor`'s answer without altering what `show doctor` executes |
| `docs/architecture/exabgp-bridge.md` | Design decisions | anchored to `internal/component/plugin/server/failure_policy.go` | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | ExaBGP wrapper endpoints and config are untouched; any external endpoint declaring `DoctorChecks` uses the same `doctorWorker` path |
| `docs/architecture/exabgp-bridge.md` | Design decisions | anchored to `internal/component/plugin/server/failure_policy.go` | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | ExaBGP UPDATE encoding and batching are untouched by this spec |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Local `ze doctor [--json] [config <file>]` entering `answerLocal` -> `answerLocalWith` -> `askDaemonOverSSH`.
- Daemon `show doctor [config <file>]` (from SSH, CLI, web, REST, gRPC, MCP, or a plugin `dispatch-command`) entering `HandleDoctor` -> `answerDaemonDoctor` -> `collectDoctorChecksFrom`.

### Transformation Path
1. **Local route (`ze doctor`):** `askDaemonOverSSH` runs `sshclient.ReachDaemon("")`, derives `ctx, cancel := context.WithTimeout(context.Background(), daemonDoctorTimeout)` (22 s), and calls `sshclient.ExecCommandRawContext(ctx, reached.Credentials, line)`.
2. **Bounded SSH exchange (`sshclient`):** `dialDaemonContext` dials TCP with `net.Dialer.DialContext` capped by `min(ctx deadline, dialTimeout)` (10 s), sets `conn.SetDeadline(handshakeDeadline)` on the raw connection before `ssh.NewClientConn` so key exchange and password authentication obey the same bound while preserving `*LoginRefused` classification, clears the connection deadline once `ssh.NewClient` is built, and `execCommandContext` registers `context.AfterFunc(ctx, func() { client.Close() })` across `NewSession`, `Setenv`, and `Output`. If `ctx` expires, `askDaemonOverSSH` returns `*daemonUnanswered` with `fixAskAsAccount` (`ze cli --user <name> -c "show doctor"`, matching `local_route.go:144` and `docs/architecture/doctor-and-health-checks.md:332`), and `answerLocalWith` falls back to local `runChecks` plus `doctor-runtime-not-checked`.
3. **Daemon plugin collection (`HandleDoctor` -> `collectDoctorChecksFrom`):**
   - Queries `source.DoctorCheckPlugins()`, skips `pluginName == callerName` (AC-2), filters checks by `platformMatches`, and groups surviving checks per plugin.
   - Sorts each plugin's checks by `(Order, check.Name)` and builds one deterministic plugin request list ordered by plugin name.
   - Calls `source.RunDoctorChecks(requests, perCheckTimeout, doctorQueueWait)` on `*pluginserver.Server` (implemented in `doctor_worker.go`).
4. **Per-generation doctor workers (`doctor_worker.go`):**
   - When `superviseProcess` starts serving a plugin generation whose Stage-1 registration declares `len(reg.DoctorChecks) > 0`, `startDoctorWorker(proc)` launches one worker reading a buffered-16 request channel (`doctorWorkerQueueDepth = 16`) on `s.wg.Go` and registers it under `s.doctorWorkers[proc]`. When the generation ends, `superviseProcess` removes it, cancels its context, and waits for `worker.done` before `applyFailurePolicy`.
   - `RunDoctorChecks` executes a two-pass fan-out with zero per-request goroutines: Pass 1 looks up each requested plugin's current worker and enqueues its batch non-blockingly (recording immediate error outcomes for stopped/absent workers, and holding any full-queue plugin in a short deferred list that waits up to the shared `queueDeadline = enqueuedAt + doctorQueueWait` only after all non-full plugins are enqueued).
   - Each worker closes the request's `pickedUp` channel on dequeue (or discards the request unrun if `req.expired.Load()` is true), then executes each check in the batch serially with a fresh `context.WithTimeout(workerCtx, perCheckTimeout)` started at pickup of that check, and posts the ordered slice of per-check outcomes to a capacity-1 `reply` channel so a late answer never blocks the worker.
   - Pass 2 collects every submitted plugin's reply: while awaiting pickup it selects on `pickedUp`, `reply`, `worker.done`, and the remaining time until `queueDeadline` (on timeout it sets `req.expired.Store(true)` and records a busy-timeout error naming the plugin and `doctorQueueWait`); once picked up it selects on `reply` and `worker.done` (the worker guarantees every picked-up check finishes within `perCheckTimeout`).
5. **Deterministic assembly (`collectDoctorChecksFrom`):**
   - Maps each check's outcome into a per-check entry retaining `(Order, pluginName, checkName, declaredCodes, output, err)`.
   - Sorts all entries deterministically by `(Order, pluginName, checkName)`.
   - Emits `doctor-plugin-unreachable` (warning, fix `ze request plugin restart <plugin>`) for failed/timed-out checks (adding no declared codes to `ownedCodes`), or records `declaredCodes`, enforces `maxDiagnosticsPerCheck` (64), maps each diagnostic via `doctor.PluginDiagnostic` and `normalizeSeverity`, and appends them to `pluginCheckAnswer.diagnostics`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator process ↔ daemon | SSH exec of `show doctor \| raw` (or `show doctor config <resolved> \| raw`) through `sshclient.ExecCommandRawContext` under 22 s context deadline; `DoctorResult` JSON returned | No |
| Daemon `doctor/cmd` ↔ `plugin/server` | `doctorCheckSource.RunDoctorChecks` dispatching ordered per-plugin batches to generation-scoped `doctorWorker` channels | No |
| Daemon ↔ plugin (internal or forked) | Existing `ze-plugin-callback:doctor-check` RPC via `conn.SendDoctorCheck(ctx, checkName)`, unchanged on wire | No |

### Changed-Wire Census

No wire format, RPC method, YANG node, or JSON envelope changes in this spec. `show doctor | raw` sends the exact existing SSH command line and decodes the unchanged `diagnostic.DoctorResult` (`schema-version: 1`); `ze-plugin-callback:doctor-check` sends the exact existing `rpc.DoctorCheckInput` and reads the unchanged `rpc.DoctorCheckOutput`; `ze-doctor-cmd.yang` is not modified.

**Search evidence:** Searched `internal/component/doctor/`, `internal/component/cli/sshclient/`, `internal/component/plugin/server/`, and `pkg/plugin/rpc/` for `SendDoctorCheck`, `CallDoctorCheck`, `DoctorCheckPlugins`, `ExecCommandRaw`, and `showDoctorCommand`. Positive control: `internal/component/doctor/local_route.go` (`showDoctorCommand = "show doctor"`, `askDaemonOverSSH` calling `sshclient.ExecCommandRaw`, `decodeDaemonDoctor` decoding `diagnostic.DoctorResult`) and `internal/component/plugin/server/server.go:729` (`conn.SendDoctorCheck(ctx, checkName)`). Every wire payload and command string stays byte-for-byte identical.

### Integration Points
- `internal/component/plugin/server/failure_policy.go` (`superviseProcess`) - starts and stops each plugin generation's `doctorWorker` right beside `startDrainCaller`.
- `internal/component/plugin/server/server.go` (`Server`) - holds `doctorWorkersMu` and `doctorWorkers map[*process.Process]*doctorWorker` beside `drainCallers`, and exposes `RunDoctorChecks` replacing `CallDoctorCheck`.
- `internal/component/doctor/cmd/show.go` (`doctorCheckSource`, `collectDoctorChecksFrom`) - submits per-plugin batches through `RunDoctorChecks` and sorts assembled diagnostics by `(Order, plugin name, check name)`.
- `internal/component/cli/sshclient/client.go` and `answer.go` - provides `ExecCommandRawContext` backed by `execCommandContext` and `dialDaemonContext`, reused by `ExecCommand` / `ExecCommandRaw`.
- `internal/component/doctor/local_route.go` (`askDaemonOverSSH`) - calls `sshclient.ExecCommandRawContext` with `daemonDoctorTimeout` (22 s) and preserves `*daemonUnanswered` with `fixAskAsAccount` (`ze cli --user <name> -c "show doctor"`) when `context.DeadlineExceeded` occurs.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Local `ze doctor` continues to reach the daemon only over SSH (`sshclient.ExecCommandRawContext`), and the daemon reaches plugins only via `conn.SendDoctorCheck` inside `plugin/server` |
| No unintended coupling (components stay isolated) | Yes | `doctor/cmd` talks to `plugin/server` through the narrow unexported `doctorCheckSource` interface; `doctor` talks to `sshclient` only as before; no plugin is imported |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ExecCommand` and `ExecCommandRaw` delegate to `execCommandContext`; `doctorWorker` mirrors `drainCaller` in `plugin/server` and replaces `CallDoctorCheck` rather than keeping a serial fallback |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Not a BGP wire/hot path |
| Registration over hardcoding, outbound | Yes | Workers are created strictly from Stage-1 `reg.DoctorChecks` declarations; no plugin name or check name is hardcoded anywhere |
| Registration over hardcoding, inbound | Yes | Searched `internal/component/doctor/` and `internal/component/plugin/server/` for plugin/check name switches: none exist; `DoctorCheckPlugins` and `startDoctorWorker` both derive solely from `proc.Registration().DoctorChecks` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Server.CallDoctorCheck` has no callers outside `internal/component/doctor/cmd/show.go` (and its three test doubles), so replacing it with `Server.RunDoctorChecks` leaves no broken callers or unwired exports | Repo-wide grep of `internal/`, `cmd/`, `pkg/`, `docs/`, and `ai/` on 2026-11-26 found only `server.go:716`, `show.go:68,137`, `show_test.go`, `show_unreachable_test.go`, `show_recent_events_test.go`, and the doc anchor `docs/architecture/api/process-protocol.md:2168` | Any extra caller would fail to compile and be migrated to `RunDoctorChecks` | `go test` across `internal/component/doctor/...` and `internal/component/plugin/server/...` | confirmed |
| A-2 | The SDK processes `ze-plugin-callback:doctor-check` serially per plugin process, so serial execution within one plugin worker avoids premature sibling timeouts | Read `pkg/plugin/sdk/sdk_dispatch.go` (`bridgeEventLoop`, `eventLoop`) on 2026-11-26: both loops invoke ordinary callbacks inline | If SDK ever gained concurrent doctor callbacks, a plugin with many checks would still run safely serially in its worker | `pkg/plugin/sdk/sdk_dispatch.go` lines 194-255 | confirmed |
| A-3 | Setting `conn.SetDeadline(handshakeDeadline)` on the raw TCP `net.Conn` before `ssh.NewClientConn` and clearing it after preserves `*LoginRefused` detection while bounding SSH handshake and password authentication | Read `internal/component/cli/sshclient/answer.go:257-289`: `keyAccepted` atomic bool is flipped in `HostKeyCallback` before authentication; `ssh.NewClientConn` runs both handshake and auth over `conn` | A timeout before `HostKeyCallback` returns a deadline error (not `LoginRefused`); a timeout during auth after host-key acceptance must surface `context.DeadlineExceeded`, NOT `LoginRefused` (check `ctx.Err()` before `keyAccepted.Load()`) | `TestExecCommandRawContextBoundsHandshakeAndExec` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A timeout during password authentication after `HostKeyCallback` ran could be misclassified as `*LoginRefused` if `keyAccepted.Load()` is checked before `ctx.Err()` | Timeout test returning `LoginRefused` instead of `context.DeadlineExceeded` | In `dialDaemonContext`, check `ctx.Err() != nil` before `keyAccepted.Load()` and return `fmt.Errorf("connect to %s: %w", addr, ctx.Err())` |
| R-2 | A worker goroutine leaks if a plugin exits while a check callback is blocked, or if `superviseProcess` exits early | `TestDoctorWorkerLifecycle` hangs on `<-worker.done` or `-race` reports a race on stop | `startDoctorWorker` derives a child context from `s.ctx`; `stopDoctorWorker` cancels it (which cancels any in-flight `SendDoctorCheck(checkCtx, ...)`) and joins `<-worker.done` before `applyFailurePolicy`, identical to `startDrainCaller` |
| R-3 | A full worker queue (16 requests) on one busy plugin blocks enqueueing requests to healthy plugins | Slow/saturated plugin delays fast plugin in Pass 1 | Pass 1 enqueues non-blockingly into every worker first, and only waits for full slots in Pass 1b after all healthy workers have already started |
| R-4 | A plugin generation respawns between `DoctorCheckPlugins()` snapshot and `RunDoctorChecks()` lookup | `doctorWorkers` keyed by `*process.Process` finds no worker for a replaced generation | Look up the current generation via `pm.GetProcess(pluginName)` inside `RunDoctorChecks` (or index active workers by `*process.Process` matched against `pm.GetProcess(pluginName)`); if absent or stopped, return immediate unreachable error rather than blocking |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show doctor` plugin check collection could deadlock, miss diagnostics, leak worker goroutines on plugin restart, or `ze doctor` could fail to connect to a healthy daemon |
| How is it reverted? | Single commit revert; no config, store, YANG, or wire migration is involved |
| Who else touches this path? | Every plugin declaring Stage-1 `DoctorChecks` (`bgp-rib`, `bgp-rr`, `bgp-rs`, `sysrib`, `ldp`, `rsvp-te`, `ospf`, `bgp-epe`, `fib-vpp`, `ike`, `container`, and test fixtures) and every `ze doctor` / `show doctor` caller |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `HandleDoctor` / `collectDoctorChecksFrom` with multiple plugins declaring Stage-1 checks | → | `Server.RunDoctorChecks` → per-generation `doctorWorker` (`startDoctorWorker` in `superviseProcess`) → deterministic `(Order, plugin, check)` sort | `TestCollectDoctorChecksConcurrent` (`internal/component/doctor/cmd/show_test.go`) and `TestDoctorWorkerLifecycle` (`internal/component/plugin/server/doctor_worker_test.go`) |
| `HandleDoctor` / `collectDoctorChecksFrom` dispatched by a check-declaring plugin | → | `collectDoctorChecksFrom` caller-skip before worker hand-off | `TestCollectDoctorChecksConcurrent` caller-skip case (`internal/component/doctor/cmd/show_test.go`) |
| Local `ze doctor` (`answerLocalWith` → `askDaemonOverSSH`) against a stalled SSH listener | → | `sshclient.ExecCommandRawContext` → `dialDaemonContext` (`net.Conn` deadline) + `execCommandContext` (`context.AfterFunc`) → `daemonUnanswered` + `doctor-runtime-not-checked` | `TestExecCommandRawContextBoundsHandshakeAndExec` (`internal/component/cli/sshclient/exec_timeout_test.go`) and `TestAskDaemonOverSSHTimeoutFallsBackLocally` (`internal/component/doctor/answer_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `show doctor` on a daemon where multiple running plugins declare Stage-1 doctor checks (including slow plugins, a plugin with two checks, an unresponsive plugin, checks with equal `Order`, and concurrent `show doctor` callers) | (a) Different plugins run concurrently through one long-lived `doctorWorker` per live plugin generation declaring `DoctorChecks` (started in `superviseProcess` and stopped/joined before `applyFailurePolicy`; plugins declaring none get no worker; zero goroutines spawned per request): two plugins each taking 0.8 of `perCheckTimeout` both answer within one `perCheckTimeout` plus margin, not their sum. (b) Checks of one plugin run serially in `(Order, check.Name)` order, each with a fresh `perCheckTimeout` (5 s) starting at pickup, never at enqueue. (c) Each worker buffers 16 requests (`doctorWorkerQueueDepth = 16`, twice `defaultMaxSessions = 8`): concurrent `show doctor` calls queue without failing, run each check once per request (never cached), and a request not picked up within `doctorQueueWait` (5 s from enqueue) or targeting a stopped worker yields `doctor-plugin-unreachable` naming the plugin, check, and busy/stopped cause with fix `ze request plugin restart <plugin>`, while the worker discards already-expired requests unrun. (d) An unresponsive plugin yields `doctor-plugin-unreachable` at its own bound without delaying other plugins and contributes no code to `ownedCodes` (preserving `recent-events` visibility). (e) Final diagnostics are ordered deterministically by `(check.Order, plugin name, check name)` regardless of map iteration or completion order | A single plugin declaring multiple slow checks can still take up to the sum of its own checks' `perCheckTimeout` bounds, because the plugin SDK dispatches callbacks serially per plugin process (`bridgeEventLoop` and `eventLoop` in `pkg/plugin/sdk/sdk_dispatch.go`); parallelizing within one plugin would only queue behind the SDK loop and expire sibling timeouts prematurely |
| AC-2 | `show doctor` dispatched by a plugin that itself declares a Stage-1 doctor check (`cmdCtx.Process.Name() == pluginName`) | `collectDoctorChecksFrom` skips the calling plugin before submitting any batch to `RunDoctorChecks`, runs every other plugin's checks concurrently through their workers, and returns within one bound with every other plugin's diagnostics and no `doctor-plugin-unreachable` for the caller | None: only the synchronously-blocked calling plugin is omitted, while every other plugin's checks still execute normally |
| AC-3 | Local `ze doctor` (`askDaemonOverSSH`) reaching a listener whose TCP connect succeeds (`ProbeDaemon` passes) but that stalls during SSH handshake, authentication, session setup, or `show doctor` execution | `askDaemonOverSSH` calls `sshclient.ExecCommandRawContext(ctx, reached.Credentials, line)` with `daemonDoctorTimeout` (22 s = `dialTimeout` 10 s + `doctorQueueWait` 5 s + `perCheckTimeout` 5 s + 2 s margin). `dialDaemonContext` bounds TCP dial, SSH key exchange, and password auth via a `net.Conn` deadline capped by `min(ctx deadline, dialTimeout)` (without misclassifying a deadline timeout as `*LoginRefused`), and `execCommandContext` closes the client via `context.AfterFunc` on context expiry. When the deadline expires, `ExecCommandRawContext` returns an error wrapping `context.DeadlineExceeded` promptly, and `askDaemonOverSSH` returns a `*daemonUnanswered` naming the address, command, and bound with `fixAskAsAccount` (`ze cli --user <name> -c "show doctor"`, matching `local_route.go:144` and `docs/architecture/doctor-and-health-checks.md:332`), causing `answerLocalWith` to fall back to local checks plus one `doctor-runtime-not-checked` warning | Existing `sshclient.ExecCommand` and `sshclient.ExecCommandRaw` keep their existing unbounded signatures (delegating to `execCommandContext(context.Background(), ...)`) so unrelated interactive or long-running CLI callers remain unaffected |
| AC-4 | Architecture and operator documentation for plugin doctor checks and the local `ze doctor` route | (a) `docs/architecture/api/process-protocol.md` (Stage-1 `Doctor Check Declaration` and `doctor-check` callback sections) accurately states that the daemon invokes plugin checks for `show doctor` concurrently across plugins via per-generation workers and serially within one plugin, that local `ze doctor` asks the running daemon first over SSH (falling back to local checks plus `doctor-runtime-not-checked` when none answers), and updates the `CallDoctorCheck` anchor to `RunDoctorChecks`. (b) `docs/guide/health-checks.md` (`External plugins (RPC)`) replaces the stale "not during offline `ze doctor`" sentence with the accurate local-route description matching `docs/architecture/doctor-and-health-checks.md`. (c) `docs/architecture/doctor-and-health-checks.md` documents the concurrent per-plugin-generation doctor workers, queue depth (16), `doctorQueueWait` (5 s), `perCheckTimeout` (5 s from pickup), deterministic `(Order, plugin, check)` ordering, and the 22 s bounded SSH ask in `askDaemonOverSSH` with updated source and test anchors | Existing statements in `docs/architecture/doctor-and-health-checks.md` noting that `runChecks` itself runs only the four config phases (never the `runtime` phase locally in the operator's process) remain intact because they accurately describe the fallback path when no daemon answers |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator runs `show doctor` (or `ze doctor` against a running daemon) with multiple plugins declaring runtime checks, one of which is slow or hung, and gets all healthy plugin diagnostics in deterministic order within one check timeout | `show doctor` → `HandleDoctor` → `collectDoctorChecksFrom` → `Server.RunDoctorChecks` → per-generation `doctorWorker` → `conn.SendDoctorCheck` → deterministic `(Order, plugin, check)` sort | `TestCollectDoctorChecksConcurrent`, `TestDoctorWorkerLifecycle`, existing `test/plugin/api-doctor-check.ci` |
| 2 | An operator runs `ze doctor` when a daemon SSH listener accepts TCP connections but hangs during handshake or execution, and receives local readiness checks plus a `doctor-runtime-not-checked` warning instead of an indefinite hang | `ze doctor` → `answerLocalWith` → `askDaemonOverSSH` → `sshclient.ExecCommandRawContext` (22 s deadline) → `daemonUnanswered` (`fix: ze cli --user <name> -c "show doctor"`) → local `runChecks` + `runtimeNotChecked` | `TestExecCommandRawContextBoundsHandshakeAndExec`, `TestAskDaemonOverSSHTimeoutFallsBackLocally` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCollectDoctorChecksConcurrent` (with shortened test timeouts: (1) two plugins each taking 0.8 of `perCheckTimeout` both answer within one bound + margin; (2) one hung plugin yields `doctor-plugin-unreachable` at its bound without delaying a fast plugin and adds no `ownedCodes`; (3) one plugin with two checks runs them serially, never overlapping, each under its own fresh bound; (4) out-of-order completions and equal-`Order` plugins sort deterministically by `(Order, plugin, check)` across repeated runs; (5) two concurrent `show doctor` calls queue against one plugin, run each check once per request with per-check clock starting at pickup, and a third call waiting past `doctorQueueWait` gets `doctor-plugin-unreachable` naming the busy plugin while the worker discards it unrun; (6) caller skip omits `callerName` before worker submission and returns within one bound with no `doctor-plugin-unreachable` for the caller) | `internal/component/doctor/cmd/show_test.go` | AC-1, AC-2 | [ ] |
| `TestDoctorWorkerLifecycle` (through real `superviseProcess` / `runPluginPhase` modeled on `drain_lifecycle_test.go`: plugin declaring `DoctorChecks` gets one worker started with its generation and stopped/joined before `applyFailurePolicy`; plugin declaring none gets no worker; respawn creates a fresh worker with no leaked old worker; submission to a stopped worker returns unreachable immediately; full-queue wait expires at `doctorQueueWait` and discards expired requests unrun; late callback answer never blocks worker exit) | `internal/component/plugin/server/doctor_worker_test.go` | AC-1 | [ ] |
| `TestExecCommandRawContextBoundsHandshakeAndExec` (against a local TCP listener that accepts and stalls before sending any SSH banner, and a fixture that stalls after connect: `ExecCommandRawContext` with a short context deadline returns `context.DeadlineExceeded` promptly without hanging and without misclassifying the timeout as `*LoginRefused`) | `internal/component/cli/sshclient/exec_timeout_test.go` | AC-3 | [ ] |
| `TestAskDaemonOverSSHTimeoutFallsBackLocally` (exercises `askDaemonOverSSH` / `answerLocalWith` with a bounded timeout against a stalled listener or timeout error: returns within the bound, runs local checks, and appends one `doctor-runtime-not-checked` warning naming the timeout and `fix: ze cli --user <name> -c "show doctor"`) | `internal/component/doctor/answer_test.go` | AC-3 | [ ] |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `doctorWorkerQueueDepth` | 1..16 queued requests per plugin worker | 16th queued request accepted immediately without waiting | N/A (fixed constant 16, twice `defaultMaxSessions = 8`) | 17th request waits for a slot up to `doctorQueueWait` (5 s), returning `doctor-plugin-unreachable` if unpicked at expiry |
| `doctorQueueWait` | 5 s from enqueue (`perCheckTimeout`) | request picked up just before `doctorQueueWait` runs with a full fresh `perCheckTimeout` | N/A | request still unpicked at `doctorQueueWait` yields `doctor-plugin-unreachable` and is discarded unrun |
| `perCheckTimeout` | 5 s per check, measured from worker pickup | check finishing just inside 5 s returns its diagnostics and registers its `ownedCodes` | N/A | check exceeding 5 s yields `doctor-plugin-unreachable` and adds no `ownedCodes` |
| `daemonDoctorTimeout` | 22 s (`dialTimeout` 10 s + `doctorQueueWait` 5 s + `perCheckTimeout` 5 s + 2 s margin) | SSH handshake + `show doctor` finishing just inside 22 s draws the daemon's `DoctorResult` | N/A | SSH handshake or `show doctor` exceeding 22 s returns `context.DeadlineExceeded` and degrades to local checks + `doctor-runtime-not-checked` with `fix: ze cli --user <name> -c "show doctor"` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/plugin/api-doctor-check.ci` (existing, re-run unchanged) | `test/plugin/` | Plugin declares a Stage-1 doctor check and `show doctor` returns its diagnostic through the new per-generation `doctorWorker` path | [ ] |
| `test/plugin/doctor-owner-check-show.ci` (existing, re-run unchanged) | `test/plugin/` | `show doctor config <file>` continues to work through `HandleDoctor` | [ ] |

### Interop Tests (Scope: protocol)
N-A: Scope is cli; no wire protocol peer.

## Files to Modify
- `internal/component/plugin/server/server.go` - add `doctorWorkersMu` and `doctorWorkers map[*process.Process]*doctorWorker` beside `drainCallers`, and remove the obsolete serial `CallDoctorCheck` method (replaced by `RunDoctorChecks` in `doctor_worker.go`).
- `internal/component/plugin/server/failure_policy.go` - `superviseProcess` calls `stopDoctorWorker := s.startDoctorWorker(proc)` beside `startDrainCaller` and invokes `stopDoctorWorker()` before `applyFailurePolicy`.
- `internal/component/doctor/cmd/show.go` - replace `CallDoctorCheck` in `doctorCheckSource` with `RunDoctorChecks`; update `collectDoctorChecksFrom` to group checks by plugin (skipping `callerName`), submit all plugins concurrently through `RunDoctorChecks`, preserve `ownedCodes` / truncation / `PluginDiagnostic` / `normalizeSeverity`, and sort final results deterministically by `(Order, plugin name, check name)`.
- `internal/component/doctor/cmd/show_test.go` - adapt `fakeDoctorCheckSource` and `countingDoctorCheckSource` to `RunDoctorChecks` and add `TestCollectDoctorChecksConcurrent`.
- `internal/component/doctor/cmd/show_unreachable_test.go` - adapt `unreachableDoctorSource` to `RunDoctorChecks`.
- `internal/component/doctor/cmd/show_recent_events_test.go` - adapt `owningDoctorCheckSource` to `RunDoctorChecks`.
- `internal/component/cli/sshclient/client.go` - add `ExecCommandRawContext(ctx, creds, command)` backed by `execCommandContext` (registering `context.AfterFunc(ctx, func() { client.Close() })`), and delegate `ExecCommand` / `ExecCommandRaw` to `execCommandContext(context.Background(), ...)`.
- `internal/component/cli/sshclient/answer.go` - add `dialDaemonContext(ctx, creds)` using `net.Dialer.DialContext` and `conn.SetDeadline` over `ssh.NewClientConn` (checking `ctx.Err()` before `keyAccepted.Load()` so timeouts are not misclassified as `*LoginRefused`), and have `dialDaemon` delegate to `dialDaemonContext(context.Background(), creds)`.
- `internal/component/doctor/local_route.go` - add `daemonDoctorTimeout` (22 s), have `askDaemonOverSSH` call `sshclient.ExecCommandRawContext` under that deadline, and preserve `*daemonUnanswered` naming the address, command, and timeout bound with `fixAskAsAccount` (`ze cli --user <name> -c "show doctor"`).
- `internal/component/doctor/answer_test.go` - add `TestAskDaemonOverSSHTimeoutFallsBackLocally`.
- `docs/architecture/api/process-protocol.md` - fix the two stale statements (lines 966 and 2152) claiming offline `ze doctor` does not invoke/reach plugin checks, update the `CallDoctorCheck` anchor to `RunDoctorChecks`, and note concurrent per-generation worker execution.
- `docs/guide/health-checks.md` - fix the stale statement (lines 299-300) claiming offline `ze doctor` does not invoke plugin checks to match `docs/architecture/doctor-and-health-checks.md`.
- `docs/architecture/doctor-and-health-checks.md` - document concurrent per-plugin-generation doctor workers, queue depth and wait bound, deterministic `(Order, plugin, check)` ordering, and the 22 s bounded SSH ask in `askDaemonOverSSH`, with updated source and test anchors.

## Files to Create
- `internal/component/plugin/server/doctor_worker.go` - `doctorWorker`, `startDoctorWorker`, `DoctorCheckPluginRequest`, `DoctorCheckPluginResult`, `DoctorCheckOutcome`, `doctorWorkerQueueDepth = 16`, and `(*Server).RunDoctorChecks` two-pass concurrent fan-out.
- `internal/component/plugin/server/doctor_worker_test.go` - `TestDoctorWorkerLifecycle`.
- `internal/component/cli/sshclient/exec_timeout_test.go` - `TestExecCommandRawContextBoundsHandshakeAndExec`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `ze-doctor-cmd.yang` already declares `show doctor`, `config`, and `kernel-capabilities`; no YANG node changes |
| YANG validation constraints | N-A | No YANG changes |
| YANG custom validators | N-A | No YANG changes |
| CLI commands/flags | No | `ze doctor` and `show doctor` grammar and flags are unchanged |
| CLI grammar (keyword before value) | N-A | No new commands or keywords |
| Editor autocomplete | N-A | No YANG changes |
| Functional test for new RPC/API | No | No new RPC; existing `test/plugin/api-doctor-check.ci` and `test/plugin/doctor-owner-check-show.ci` exercise `show doctor` |
| Pipe completeness | N-A | Output rendering and pipe path unchanged |
| Env var registration | No | No new env var; reuses existing `ze.ssh.*` registrations |
| Doctor check for runtime dependencies | N-A | Improves doctor's own check runner and SSH timeout; adds no runtime dependency |
| Doctor code per problem class | Yes | Reuses existing `doctor-plugin-unreachable` (for busy/stopped/timed-out plugin workers) and `doctor-runtime-not-checked` (for SSH ask timeout), both already registered with exact fixes |
| Prometheus counters/metrics | No | No new observable metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `docs/features.md` already describes the shipped `local_route.go` daemon route |
| 2 | Config syntax changed? | No | No config leaf changed |
| 3 | CLI command added/changed? | No | No command or flag changed |
| 4 | API/RPC added/changed? | No | `docs/architecture/api/commands.md` is unaffected; `show doctor` path and answer shape are unchanged |
| 5 | Plugin added/changed? | No | No plugin added or changed |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` (fix stale offline `ze doctor` sentence in `External plugins (RPC)`) |
| 7 | Wire format changed? | No | No wire format changed |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (fix stale lines 966 and 2152; update `CallDoctorCheck` anchor to `RunDoctorChecks`) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | No | Existing unit and `.ci` harnesses only |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (concurrent per-generation workers, deterministic sort, 22 s bounded SSH ask) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors`: `docs/architecture/doctor-and-health-checks.md` and `docs/architecture/api/process-protocol.md` are updated; `docs/architecture.md`, `docs/architecture/api/architecture.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/firewall/firewall-irr.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/overview.md`, `docs/architecture/system-architecture.md`, `docs/architecture/testing/ci-format.md`, `docs/architecture/testing/runner-architecture.md`, `docs/architecture/update-cache.md`, `docs/features.md`, `docs/features/formatting.md`, `docs/features/plugins.md`, `docs/guide/appliance.md`, `docs/guide/authentication.md`, `docs/guide/command-reference.md`, `docs/guide/ddos-mitigation.md`, `docs/guide/ipsec.md`, `docs/guide/operations.md`, `docs/guide/plugins.md`, `docs/plugin-development/metrics.md`, and `docs/plugin-overview.md` are unaffected because command paths, answer framing, credential loading, plugin failure policies, and `doctor-runtime-not-checked` semantics stay unchanged |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verified `docs/guide/health-checks.md` and `docs/architecture/doctor-and-health-checks.md` examples match the shipped `ze doctor` / `show doctor` output |

## Design presentation

### What changes

This spec finishes the remaining three gaps of `doctor-daemon-route` on top of the already-landed local daemon route (`internal/component/doctor/local_route.go`). First, the daemon runs Stage-1 plugin doctor checks concurrently across plugins using one long-lived `doctorWorker` per live plugin generation (mirroring `drainCaller`), keeping checks of a single plugin serial in declared order and sorting the combined diagnostics deterministically by `(Order, plugin name, check name)`. Second, `ze doctor`'s SSH ask (`askDaemonOverSSH`) switches to a context-bounded `sshclient.ExecCommandRawContext` with a 22 s overall deadline covering TCP connect, SSH handshake, authentication, and command execution so a stalled daemon falls back cleanly to local checks plus `doctor-runtime-not-checked` instead of hanging indefinitely. Third, the stale statements in `docs/architecture/api/process-protocol.md` and `docs/guide/health-checks.md` claiming offline `ze doctor` never reaches plugin checks are corrected, and `docs/architecture/doctor-and-health-checks.md` records the worker concurrency and SSH bound.

### Before and after

1. **Multiple plugins declaring doctor checks on `show doctor`:**
   - Before: `collectDoctorChecksFrom` loops serially over every check (5 s timeout each), so 5 slow plugins can take up to 25 s, one hung plugin stalls every plugin behind it, and equal-`Order` checks appear in random map-iteration order.
   - After: Each plugin's generation-scoped `doctorWorker` runs concurrently with every other plugin's worker; 5 plugins each taking 4 s finish together in ~4 s, a hung plugin yields `doctor-plugin-unreachable` at 5 s without delaying others, and output is deterministically ordered by `(Order, plugin name, check name)`.
2. **Local `ze doctor` hitting a stalled SSH daemon listener:**
   - Before: `ProbeDaemon` succeeds on TCP connect, then `sshclient.ExecCommandRaw` blocks indefinitely during a stalled SSH handshake, auth, or hung `show doctor` command because `ssh.ClientConfig.Timeout` bounds only TCP dial.
   - After: `sshclient.ExecCommandRawContext` expires at 22 s, closes the SSH client, and `askDaemonOverSSH` falls back to local checks with a `doctor-runtime-not-checked` warning naming the 22 s timeout and `fix: ze cli --user <name> -c "show doctor"`.
3. **Protocol and guide documentation:**
   - Before: `docs/architecture/api/process-protocol.md` (lines 966, 2152) and `docs/guide/health-checks.md` (lines 299-300) state that offline `ze doctor` does not reach or invoke plugin doctor checks, contradicting `docs/architecture/doctor-and-health-checks.md`.
   - After: All three pages agree that `ze doctor` asks the running daemon first over SSH and runs local checks plus `doctor-runtime-not-checked` only when no daemon answers.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep the original `show doctor runtime` subcommand and `DoctorResult.runtime` JSON object from 2026-10-15 | Already superseded on `main` by `local_route.go` (`answerLocalWith` sending `show doctor \| raw` and adding `doctor-runtime-not-checked` on fallback), documented in `docs/architecture/doctor-and-health-checks.md`. Reviving the old design would break existing tests and depart from the shipped architecture |
| Spawn a goroutine per plugin check inside `collectDoctorChecksFrom` | Violates `ai/rules/goroutine-lifecycle.md` (no goroutine per request/event) and risks unbounded goroutine pile-ups under concurrent `show doctor` calls; per-generation workers mirror `drainCaller` cleanly |
| Run checks of the SAME plugin concurrently | The plugin SDK processes callbacks serially in one loop (`bridgeEventLoop` / `eventLoop` in `pkg/plugin/sdk/sdk_dispatch.go`), so concurrent callbacks to one plugin would queue inside the SDK and expire sibling `perCheckTimeout` clocks while waiting |
| Add a fixed global pool of N doctor workers in `internal/component/doctor` | A fixed pool reintroduces serial waits whenever plugin count exceeds N and has no plugin generation lifecycle to bind to |
| Leave `sshclient.ExecCommandRaw` unbounded and rely on `ProbeDaemon` | `ProbeDaemon` only checks TCP accept; a wedged SSH server or deadlocked daemon still hangs `ze doctor` indefinitely |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `doctorWorker` skeleton and `Server.RunDoctorChecks` signature wired into `superviseProcess` and `collectDoctorChecksFrom`; add `ExecCommandRawContext` / `dialDaemonContext` skeleton wired into `askDaemonOverSSH`; write the failing wiring tests.
   - Tests: `TestDoctorWorkerLifecycle`, `TestCollectDoctorChecksConcurrent`, `TestExecCommandRawContextBoundsHandshakeAndExec`, `TestAskDaemonOverSSHTimeoutFallsBackLocally`
   - Files: `internal/component/plugin/server/doctor_worker.go`, `server.go`, `failure_policy.go`, `internal/component/doctor/cmd/show.go`, `internal/component/cli/sshclient/client.go`, `answer.go`, `internal/component/doctor/local_route.go`
   - Verify: entry points compile and are reachable; new tests fail against stub implementations.
2. **Phase: Per-generation doctor workers and deterministic collector (AC-1, AC-2)** -- implement `doctorWorker`, `startDoctorWorker`, buffered-16 queue, `doctorQueueWait` with discard-expired-unrun, fresh per-check `perCheckTimeout` at pickup, two-pass fan-out in `Server.RunDoctorChecks`, caller-skip preservation, and `(Order, plugin name, check name)` sorting in `collectDoctorChecksFrom`; remove obsolete `Server.CallDoctorCheck` and update the three test doubles in `internal/component/doctor/cmd/`.
   - Tests: `TestDoctorWorkerLifecycle`, `TestCollectDoctorChecksConcurrent`, existing `internal/component/doctor/cmd/...` tests
   - Files: `internal/component/plugin/server/doctor_worker.go`, `doctor_worker_test.go`, `server.go`, `failure_policy.go`, `internal/component/doctor/cmd/show.go`, `show_test.go`, `show_unreachable_test.go`, `show_recent_events_test.go`
   - Verify: tests fail → implement → tests pass under `-race`.
3. **Phase: Bounded SSH exec on `ze doctor` local route (AC-3)** -- implement `dialDaemonContext` (`net.Dialer.DialContext` + `conn.SetDeadline` over `ssh.NewClientConn` with `ctx.Err()` checked before `keyAccepted.Load()`), `execCommandContext` (`context.AfterFunc` closing client on expiry), and `ExecCommandRawContext`; wire `askDaemonOverSSH` with `daemonDoctorTimeout` (22 s) and timeout fallback wording keeping `fixAskAsAccount`.
   - Tests: `TestExecCommandRawContextBoundsHandshakeAndExec`, `TestAskDaemonOverSSHTimeoutFallsBackLocally`, existing `internal/component/cli/sshclient/...` and `internal/component/doctor/...` tests
   - Files: `internal/component/cli/sshclient/client.go`, `answer.go`, `exec_timeout_test.go`, `internal/component/doctor/local_route.go`, `answer_test.go`
   - Verify: tests fail → implement → tests pass under `-race`.
4. **Phase: Documentation and functional verification (AC-4)** -- update `docs/architecture/api/process-protocol.md`, `docs/guide/health-checks.md`, and `docs/architecture/doctor-and-health-checks.md`; verify anchors and existing functional tests.
   - Tests: `test/plugin/api-doctor-check.ci`, `test/plugin/doctor-owner-check-show.ci`, `./le doc check all`
   - Files: `docs/architecture/api/process-protocol.md`, `docs/guide/health-checks.md`, `docs/architecture/doctor-and-health-checks.md`
   - Verify: stale statements gone; `./le doc check all` and scoped functional tests pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-4 has an implementation at file:line |
| Feature completeness | Both user stories have a working path and passing tests |
| Correctness | Concurrent plugins finish in one bound; equal-`Order` sorting is deterministic; `ownedCodes` excludes failed checks; `ctx.Err()` is checked before `keyAccepted.Load()` so timeouts are not misclassified as `*LoginRefused` |
| Naming | Constants and symbols match surrounding conventions (`doctorWorker`, `doctorWorkerQueueDepth`, `doctorQueueWait`, `daemonDoctorTimeout`, `ExecCommandRawContext`) |
| Data flow | `collectDoctorChecksFrom` submits through `Server.RunDoctorChecks` only; `askDaemonOverSSH` uses `ExecCommandRawContext` only |
| Operator errors and doctor codes | Busy/stopped worker timeout emits `doctor-plugin-unreachable` with `fix: ze request plugin restart <plugin>`; SSH ask timeout emits `doctor-runtime-not-checked` with `fix: ze cli --user <name> -c "show doctor"` |
| Rule: no-layering | Obsolete serial `Server.CallDoctorCheck` and serial loop in `collectDoctorChecksFrom` are deleted, never kept beside the worker path |
| Rule: goroutine-lifecycle | Zero `go func(` per request; `doctorWorker` owns one goroutine per live plugin generation, started and joined in `superviseProcess` with lifecycle documented on the type |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Per-generation `doctorWorker` and `Server.RunDoctorChecks` | `TestDoctorWorkerLifecycle` passes under `-race` |
| Concurrent `collectDoctorChecksFrom` with caller skip and deterministic `(Order, plugin, check)` sort | `TestCollectDoctorChecksConcurrent` passes under `-race` |
| Context-bounded `sshclient.ExecCommandRawContext` wired into `askDaemonOverSSH` (22 s) | `TestExecCommandRawContextBoundsHandshakeAndExec` and `TestAskDaemonOverSSHTimeoutFallsBackLocally` pass under `-race` |
| Stale protocol and guide docs fixed | `grep -n "does not invoke plugin doctor checks\|does not reach plugins\|not during offline" docs/architecture/api/process-protocol.md docs/guide/health-checks.md` returns nothing, and `./le doc check all` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Worker queue capped at 16 with 5 s `doctorQueueWait`; expired requests discarded unrun; SSH ask bounded to 22 s across connect, handshake, auth, and exec |
| Error leakage | Timeout errors name daemon address, command, and bound (never credentials or passwords) |
| Authentication semantics | Host-key verification (`hostKeyCallback`) and `*LoginRefused` classification remain intact on `dialDaemonContext` |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rebase spec onto the shipped `local_route.go` / `doctor-runtime-not-checked` daemon route instead of reviving the superseded `show doctor runtime` / `DoctorResult.runtime` design | Implement the 2026-10-15 `show doctor runtime` and `DoctorResult.runtime` design on top of `main` | `local_route.go` and `docs/architecture/doctor-and-health-checks.md` already implement and document the daemon route on `main`; reviving the old design would break existing contracts and duplicate doctor paths |
| One long-lived `doctorWorker` per live plugin generation in `internal/component/plugin/server`, mirroring `drainCaller` in `drain.go` | (a) Per-request goroutine fan-out in `collectDoctorChecksFrom`; (b) fixed worker pool in `internal/component/doctor`; (c) concurrent checks within a single plugin | Matches `ai/rules/goroutine-lifecycle.md` and the proven `superviseProcess` start/stop lifecycle; keeps per-plugin checks serial to match the SDK's single-loop dispatch while overlapping across plugins |
| Context-bounded `sshclient.ExecCommandRawContext` using `net.Dialer.DialContext`, `conn.SetDeadline` over `ssh.NewClientConn`, and `context.AfterFunc` client close, called by `askDaemonOverSSH` with a 22 s bound | (a) Leave `ExecCommandRaw` unbounded; (b) add a separate SSH transport in `doctor`; (c) change `ExecCommandRaw`'s signature for all callers | Reuses the single `sshclient` transport with zero duplication (`ExecCommand` / `ExecCommandRaw` delegate to `execCommandContext`), bounds the entire exchange including handshake and auth, and keeps unrelated CLI callers unaffected |

## Known Limitations
- A single plugin declaring several slow checks answers in up to the sum of its own checks' `perCheckTimeout` bounds because the SDK processes callbacks serially per plugin process (`pkg/plugin/sdk/sdk_dispatch.go`); if that sum exceeds `daemonDoctorTimeout` (22 s), local `ze doctor` falls back with `doctor-runtime-not-checked` and `fix: ze cli --user <name> -c "show doctor"`.
- A host where no daemon listens or where the caller lacks SSH login credentials continues to receive local checks plus `doctor-runtime-not-checked`, by design (`docs/architecture/doctor-and-health-checks.md`).

## Design Review Log

| Round | Date | Scope | Result | Disposition |
|-------|------|-------|--------|-------------|
| - | 2026-10-15 | split & O-1..O-5, r1..r3 | Original design for local daemon route, `show doctor runtime`, `config <file>`, and concurrent `show doctor` | Original local-route, `config <file>`, and `HandleDoctor` goals subsequently landed on `main` via `local_route.go` and `ze-doctor-cmd.yang` (rev 2026-11-02) |
| rebase | 2026-11-26 | Rebased against HEAD | Narrowed to the three remaining open items: concurrent per-generation plugin doctor workers + deterministic sort (AC-1/AC-2), bounded SSH exec on `askDaemonOverSSH` (AC-3), and stale protocol/guide docs (AC-4); added `Existing Mechanisms`, `Changed-Wire Census`, `Design documents`, and `Design presentation` | Ready for design-judge verification |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (N-A: no RFC)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: Scope cli, no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-doctor-daemon-route.md` only, in the same `./le commit create` script
