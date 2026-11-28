# Spec: Daemon runtime one-off defects (3) - Plugin dispatch error preservation, SDK goroutine guard, completion RPC, and IPC auth cleanup

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Fixes five open plugin-infra/SDK defects: `dispatchCommandPeerResponse` discarding response data on error, unguarded plugin goroutines lacking an SDK recovery starter, plugin `Completable` commands missing from `ze cli` completion rows, plugin completion overloading `ExecuteCommand`, and dead test-only `ipc.Authenticate` functions accepting empty tokens |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the five open `internal/component/plugin/` and `pkg/plugin/` journal rows routed from the umbrella.

## Task

Fix five plugin-infrastructure and SDK defects verified open at HEAD on 2026-11-26:

1. **Preserve `*plugin.Response` beside `dispatchErr` (`error-path-discards-data-already-received`, 2026-09-08).** In `(*Server).dispatchCommandPeerResponse` (`internal/component/plugin/server/dispatch.go` L938-953), when `s.dispatcher.Dispatch(cmdCtx, command)` returns both a non-nil `resp` and a non-nil `dispatchErr`, the function currently returns `nil, dispatchErr`, throwing away `resp.Data`, `resp.Error`, `resp.Warnings`, and `resp.RecordAnswerMetadata()`. Return `resp, dispatchErr` when `resp != nil` (preserving the existing logging and `refusePluginRequest` behavior) and ensure its callers (`dispatchCommandResponse`, `handleDispatchCommand`) forward the response's records, status, error sentence, and warnings rather than dropping them.
2. **SDK recovered goroutine starter and remaining unguarded plugin goroutines (`unguarded-goroutine-kills-the-process`, 2026-09-02).** Add a recovered goroutine helper on `pkg/plugin/sdk` (`(*Plugin).Go(task string, fn func())` plus a standalone `sdk.GoRecovered(log *slog.Logger, task string, fn func())`) that defers `recover()`, logs the task name, panic value, and `debug.Stack()` to stderr, and — when called via `(*Plugin).Go` after handshake — asynchronously enqueues a report-bus error via `p.RaiseIssue` through a non-blocking bounded issue queue (never calling `internal/core/report` directly and never blocking a lock-holding loop; code registered with `diagnostic.RegisterEventCode`), keeping the plugin process alive. Guard the remaining raw `go func()` plugin workers identified at HEAD (including `internal/plugins/flowexport/enrichbgp.go`, `internal/plugins/ddos/local/register.go`, `internal/plugins/imageserver/register.go`, `internal/plugins/provision/main.go`, `internal/plugins/exabgp/`, `internal/component/bgp/plugins/healthcheck/hooks.go`, `rib/rib_source_lease.go`, `ls_export/export_state.go`, and `rpki/rtr_session.go`) through either the SDK helper or existing `taskguard`.
3. **Expose plugin `Completable` commands on `ze cli` (`unwired-feature`, 2026-11-11).** Producer inspection showed that SSH `WireArgCompletion` / `WithArgCompletion` (`internal/component/plugin/server/arg_completion.go` L107-120) already walks `d.Registry().All()` for `Completable`. The remaining gap is `commandRows` (`internal/component/plugin/server/system.go` L419), which omits `Completable: cmd.Completable` on registry rows, so `ze cli`'s `wireDaemonArgCompletion` never hangs value hints on plugin `Completable` commands. Set `Completable: cmd.Completable` in `commandRows` and add tests through both SSH and `ze cli`.
4. **Dedicated plugin completion RPC instead of overloading `ExecuteCommand` (`field-carries-two-meanings`, 2026-11-15).** `handleArgComplete` (`internal/component/plugin/server/system.go` L660-661) currently sends `rpc.ExecuteCommandInput{Serial: serial, Command: cmd.Name, Args: completedArgs, Peer: partial}` via `conn.SendExecuteCommand`, making a Tab press indistinguishable from executing the command (so a mutating command like `update irr` would run on Tab). Because Ze is pre-release (`ai/META-DECISIONS.md`), add a dedicated `ze-plugin-callback:complete-command` RPC in `internal/core/ipc/yang/ze-plugin-callback.yang`, `pkg/plugin/rpc` (`MethodCompleteCommand`, `CompleteCommandInput`, `CompleteCommandOutput`, `MuxConn.SendCompleteCommand`, `DirectBridge` slot), and `pkg/plugin/sdk` (`(*Plugin).OnCompleteCommand`), and route `handleArgComplete` through it on both socket and `DirectBridge` transports with zero backward-compat fallback onto `ExecuteCommand`.
5. **Delete test-only `ipc.Authenticate` and `authenticateWithName` (`production-function-only-tests-call`, 2026-11-11).** Both functions in `internal/component/plugin/ipc/tls.go` have zero production callers (only `tls_test.go`) and still authenticate an empty token against an empty expected token. Delete both functions and migrate `internal/component/plugin/ipc/tls_test.go` onto `AuthenticateWithLookup` (which already refuses an empty expected token).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, public `pkg/` API, process boundary
- [ ] `ai/doors/plugin-infra.md` - wire protocol, DirectBridge parity, Stage 1 declarations
- [ ] `docs/architecture/api/process-protocol.md` - callback RPCs, `report-issue`, wire format, `## Design decisions`
- [ ] `docs/architecture/api/ipc_protocol.md` - RPC types and framing
- [ ] `docs/architecture/api/commands.md` - command answers and `## Design decisions`
- [ ] `docs/architecture/cli/command-completion.md` - argument completion across SSH and `ze cli`
- [ ] `docs/architecture/plugin/plugin-system.md` - process boundary, SDK stays generic, `## Design decisions`
- [ ] `ai/rules/goroutine-lifecycle.md` - worker lifecycles and panic recovery

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `internal/component/plugin/server/dispatch.go` - `dispatchCommandPeerResponse` (L938-953) returns `nil, dispatchErr` when `s.dispatcher.Dispatch` returns a non-nil `dispatchErr`, discarding any non-nil `resp`.
- [ ] `pkg/plugin/sdk/` - has no recovered goroutine starter (`sdk_dispatch.go` recovers command dispatch and bridge callbacks only); `pkg/plugin/sdk/report.go` already provides `(*Plugin).RaiseIssue` for process-boundary-safe issue reporting.
- [ ] `internal/component/plugin/server/arg_completion.go` - `argumentPaths` (L107-120) already includes `registered.Completable` commands for SSH `WireArgCompletion`/`WithArgCompletion`.
- [ ] `internal/component/plugin/server/system.go` - `commandRows` (L418-426) omits `Completable: cmd.Completable` on registry rows, and `handleArgComplete` (L654-670) calls `conn.SendExecuteCommand` with `Peer: partial`.
- [ ] `internal/component/plugin/ipc/tls.go` - `Authenticate` (L85) and `authenticateWithName` remain exported/present despite having callers only in `tls_test.go`, and compare SHA-256 digests with no empty-token guard.

**Behavior to preserve:**
- `ErrSilent` in `dispatchCommandPeerResponse` still answers `&plugin.Response{Status: plugin.StatusDone}, nil`.
- Builtin argument completion (`RPCRegistration.Complete`) continues to win collisions over plugin commands and enforce `isAuthorizedCommandArgs`.
- `AuthenticateWithLookup` keeps constant-time SHA-256 comparison and empty-expected-token refusal.
- The SDK stays generic (`plugin-system.md`) and carries no plugin-specific code.

**Behavior to change:**
- `dispatchCommandPeerResponse` returns `resp, dispatchErr` when `resp != nil` so callers preserve records, status, error text, and warnings.
- `pkg/plugin/sdk` provides `(*Plugin).Go` / `sdk.GoRecovered` with panic recovery, stderr stack logging, and non-blocking `RaiseIssue` reporting under a registered diagnostic event code; remaining raw `go func()` plugin workers are guarded.
- `commandRows` sets `Completable: cmd.Completable` on plugin registry rows.
- `handleArgComplete` invokes the new `ze-plugin-callback:complete-command` RPC (`SendCompleteCommand` / `DirectBridge` / `sdk.OnCompleteCommand`) instead of `SendExecuteCommand`.
- `ipc.Authenticate` and `authenticateWithName` are deleted and their tests run against `AuthenticateWithLookup`.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin command dispatch | `internal/component/plugin/server/dispatch.go` `dispatchCommandPeerResponse`, `responseToDispatchOutput`; `pkg/plugin/sdk/sdk_engine.go` `answerValue` | `answerValue` and `responseToDispatchOutput` already preserve completed record answers beside failure (`cdb4156509`), but `dispatchCommandPeerResponse` still drops `resp` on `dispatchErr != nil` | Extends: returns `resp, dispatchErr` so records, status, error, and warnings survive the top server layer |
| Plugin report bus RPC | `pkg/plugin/sdk/report.go` `(*Plugin).RaiseIssue`; `internal/component/plugin/server/dispatch_report.go`; `internal/core/diagnostic/registry.go` `RegisterEventCode` | Delivers plugin issues across the process boundary to the daemon's report bus and doctor | Extends: `(*Plugin).Go` raises a registered panic event code asynchronously via a non-blocking queue calling `RaiseIssue` (never direct `internal/core/report`) |
| Task panic guard | `internal/core/taskguard/guard.go` `Guard`, `Task.Contain` | Recovers panics in network-input loops inside in-tree packages | Does not replace: stays for in-tree network loops; `pkg/plugin/sdk` adds a public generic `(*Plugin).Go` helper usable by both in-tree and third-party external plugins |
| Argument completion wiring and RPC | `internal/component/plugin/server/arg_completion.go` `WireArgCompletion`, `argumentPaths`; `system.go` `commandRows`, `handleArgComplete`; `internal/core/ipc/yang/ze-plugin-callback.yang` | `argumentPaths` already walks plugin `Completable` commands for SSH, but `commandRows` omits `Completable` for `ze cli` and `handleArgComplete` overloads `execute-command` | Extends: sets `Completable: cmd.Completable` in `commandRows` and adds `ze-plugin-callback:complete-command` on both socket and `DirectBridge` |
| Plugin TLS auth | `internal/component/plugin/ipc/tls.go` `AuthenticateWithLookup`, `Authenticate`, `authenticateWithName` | Production uses `AuthenticateWithLookup` (which refuses empty expected tokens); `Authenticate` and `authenticateWithName` are test-only and accept empty tokens | Replaces: deletes `Authenticate` and `authenticateWithName` and migrates `tls_test.go` onto `AuthenticateWithLookup` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "DirectBridge is an in-process optimization of the same protocol, not a second protocol: an external plugin gets the same behavior over RPC." | agrees | `ze-plugin-callback:complete-command` and `(*Plugin).Go` panic reporting (`RaiseIssue`) both work identically over TLS RPC and `DirectBridge`. |
| `ai/doors/plugin-infra.md` | Where it sits | the door covering `internal/component/plugin/`, `pkg/plugin/rpc/`, `pkg/plugin/sdk/` | "It is infrastructure and cannot be compiled out." | agrees | Keeps all RPC, DirectBridge, SDK, and server dispatch changes inside plugin infrastructure with zero BGP imports and no backward-compat shims. |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/ipc/yang/ze-plugin-callback.yang` and `internal/core/diagnostic/codes.go` | "A core package is a library with no config-driven lifecycle and no registry side effect" | agrees | Adds the `complete-command` callback YANG RPC and diagnostic event code cleanly inside core leaf packages. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/plugin/dispatch-error-preserves-records.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | Adds the functional `.ci` test under `test/plugin/`. |
| `docs/architecture/api/process-protocol.md` | Overview | `// Design:` page of `dispatch.go`, `system.go`, `tls.go`, `bridge.go`, `sdk*.go`; modified by this spec | "Ze plugins communicate with the engine via newline-framed YANG RPCs over a" | agrees | Adds `ze-plugin-callback:complete-command` in YANG, `rpc`, and `sdk` on both socket and `DirectBridge` transports, preserves response data on error, and updates this page in the same change. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Existing engine-op authorization (`checkEngineOpBusSend`) stays unchanged in `dispatchCommandPeerResponse`; `complete-command` is an engine-to-plugin callback RPC, not an engine-op. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Event delivery queues are untouched; only command-completion RPC and error-path response preservation change. |
| `docs/architecture/cli/command-completion.md` | The producers | `// Design:` page of `arg_completion.go`; modified by this spec | "Every producer asks one predicate what its surface owes a command:" | agrees | Sets `Completable: cmd.Completable` in `commandRows` and routes `handleArgComplete` through `ze-plugin-callback:complete-command` so SSH and `ze cli` complete plugin commands without executing them, updating this page in the same change. |
| `docs/plugin-development/handlers.md` | Registration | modified by this spec | "Register handlers before calling `Run()`:" | agrees | Documents `OnCompleteCommand` and `(*Plugin).Go` registration/usage for external plugin authors in the same change. |
| `docs/architecture/api/commands.md` | Overview | `// Design:` page of `pkg/plugin/rpc/types.go` and `pkg/plugin/sdk/sdk_callbacks.go` | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Preserves command dispatch paths and record answer envelopes while keeping response data alongside errors in `dispatchCommandPeerResponse`. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Pipe grammar and execution are untouched; Tab completion uses a dedicated `complete-command` RPC rather than overloading command execution. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Pipe override contracts are untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Pipe operator semantics are untouched. |
| `docs/architecture/api/ipc_protocol.md` | Overview | `// Design:` page of `pkg/plugin/rpc/types.go` | "Ze uses correlated, bidirectional RPC between the engine and plugins." | agrees | Adds `CompleteCommandInput` / `CompleteCommandOutput` correlated by `serial` on the bidirectional RPC channel. |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `pkg/plugin/sdk/` and `internal/component/plugin/` | "Callback abstraction for external plugins" | agrees | Keeps `pkg/plugin/sdk` generic (`(*Plugin).Go` and `OnCompleteCommand` carry no plugin-specific logic) and preserves process-boundary parity via `RaiseIssue`. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Query-mode declaration purity is untouched; `(*Plugin).Go` runs only during live plugin execution, never in declare mode. |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `internal/component/plugin/server/system.go` | silent (page covers API transport architecture and bus participant filters, not `complete-command` callback RPC or `ipc.Authenticate` deletion) | silent | Preserves all API transport and bus participant isolation rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Implied-config rendering is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Bus participant registrations are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Bus send destinations are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Bus receive sources are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus allow-lists are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Bus filter compilation is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | No bus arbitration is added. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Bus display commands are untouched. |
| `docs/architecture/core-design.md` | Executive Summary | anchored to `pkg/plugin/rpc/types.go` | silent (page covers overall architecture and sections 14-21, not `complete-command` callback RPC or `ipc.Authenticate` cleanup) | silent | Preserves core modularity and tier rules. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Data-plane compatibility validation is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Data-plane mismatch verdicts are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | VPP firewall verifier rules are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Data-plane selector names are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP egress next-hop rewriting is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP next-hop authoring forms are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | `pkg/plugin/rpc.FilterInsertion` is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP raw-override provenance is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP IPv4/IPv6 next-hop validation is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP originated vs forwarded next-hop rules are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Implied-filter seam is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Adds no startup exit or refusal; `(*Plugin).Go` keeps a plugin alive on worker panic and reports the fault through the report bus and doctor. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Data-plane mismatch handling at start is untouched. |
| `docs/architecture/fleet-config.md` | Vision | anchored to `internal/component/plugin/ipc/tls.go` | silent (page covers fleet config push/pull, not test-only `ipc.Authenticate` deletion) | silent | Fleet hub uses `AuthenticateWithLookup`, which stays unchanged. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Managed listener certificate rotation is untouched. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Fleet web editing is untouched. |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to `internal/component/plugin/server/dispatch.go` | silent (page covers event history ring taps, not `dispatchCommandPeerResponse` error-path preservation) | silent | Event history ring behavior is untouched. |
| `docs/architecture/doctor-and-health-checks.md` | BGP policy absences in the supplied configuration | anchored to `internal/component/plugin/server/dispatch.go` and `internal/core/diagnostic/codes.go` | silent (page covers doctor check taxonomy; new panic event code registers via `diagnostic.RegisterEventCode` per existing rule) | silent | New plugin-goroutine-panic diagnostic code follows `diagnostic.RegisterEventCode` and raises on the report bus via `RaiseIssue`. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `internal/component/plugin/server/dispatch.go` | silent (page covers RIB pool and wire design, not plugin dispatch error preservation or completion RPC) | silent | RIB storage is untouched. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `internal/component/plugin/server/dispatch.go` | silent (page covers `.ci` test syntax, not plugin RPC internals) | silent | New `.ci` test follows standard `.ci` syntax. |
| `docs/architecture/hub-architecture.md` | Vision | anchored to `internal/component/plugin/server/system.go` | silent (page covers hub process orchestration, not `complete-command` RPC or `ipc.Authenticate` cleanup) | silent | Hub orchestration is untouched. |
| `docs/architecture/web-interface.md` | Component Structure | anchored to `internal/component/plugin/server/system.go` | silent (page covers web templ components, not plugin completion RPC) | silent | Web UI is untouched. |
| `docs/features/ai-first.md` | Shared Registrations | anchored to `internal/core/diagnostic/codes.go` | silent (page covers shared command/diagnostic registration vision, not plugin completion RPC) | silent | Registers the panic event code in the shared diagnostic registry. |
| `docs/architecture/api/wire-format.md` | Transport | anchored to `pkg/plugin/rpc/types.go` | "Messages are UTF-8 lines terminated by exactly one newline byte (0x0A)." | agrees | `ze-plugin-callback:complete-command` uses the standard single-line newline-framed JSON RPC format. |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `pkg/plugin/rpc/types.go` | silent (page covers config apply tier ordering, not completion RPC or SDK goroutine recovery) | silent | Config apply ordering is untouched. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `pkg/plugin/rpc/types.go` | silent (page covers verify/apply/rollback config transactions, not Tab completion RPC) | silent | Config transaction RPCs are untouched. |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `pkg/plugin/rpc/types.go` | silent (page covers firewall IRR plugin decisions, not completion RPC or SDK goroutine helper) | silent | Firewall IRR plugin is untouched. |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `pkg/plugin/rpc/types.go` | silent (page covers `route-install` RPC, not `complete-command` RPC) | silent | Route-install RPC is untouched. |
| `docs/architecture/plugin-manager-wiring.md` | Problem | anchored to `pkg/plugin/rpc/types.go` | silent (page covers plugin manager startup wiring, not completion RPC) | silent | Startup wiring is untouched. |
| `docs/architecture/system-architecture.md` | System RIB process boundary | anchored to `pkg/plugin/rpc/types.go` | silent (page covers Loc-RIB selection boundary, not completion RPC) | silent | System RIB boundary is untouched. |
| `docs/architecture/update-cache.md` | Purpose | anchored to `pkg/plugin/rpc/types.go` | silent (page covers BGP UPDATE cache, not completion RPC or SDK goroutines) | silent | UPDATE cache RPCs are untouched. |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `pkg/plugin/rpc/types.go` | silent (page covers BGP NLRI wire encoding, not plugin completion RPC) | silent | NLRI codecs are untouched. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Redistribution source identity | anchored to `pkg/plugin/rpc/types.go` | silent (page covers IPsec XFRM redistribution, not plugin completion RPC) | silent | IPsec XFRM paths are untouched. |
| `docs/architecture/api/json-format.md` | Overview | anchored to `pkg/plugin/sdk/sdk.go` | silent (page covers BGP JSON event format, not plugin completion RPC) | silent | JSON event format is untouched. |
| `docs/architecture/plugin/show-enrichers.md` | The decisions | anchored to `pkg/plugin/sdk/sdk.go` | silent (page covers Stage 1 show enrichers, not `complete-command` RPC or `(*Plugin).Go`) | silent | Show enrichers are untouched. |
| `docs/architecture/cli/plugin-modes.md` | Offline flag completion | anchored to `pkg/plugin/sdk/sdk_callbacks.go` | silent (page covers offline plugin CLI modes and flag completion, not live daemon `complete-command` RPC) | silent | Offline flag completion is untouched. |
| `docs/architecture/module-tiers.md` | The three tiers | anchored to `pkg/plugin/sdk/sdk_callbacks.go` | silent (page covers component/plugin/core tiers, not `complete-command` RPC) | silent | Module tier placement and import direction remain clean. |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `pkg/plugin/sdk/sdk_callbacks.go` | silent (page covers Loc-RIB `ForwardHandle`, not completion RPC) | silent | `ForwardHandle` is untouched. |
| `docs/architecture/firewall/firewall-domain-group.md` | Why a plugin | anchored to `pkg/plugin/sdk/sdk_dispatch.go` | silent (page covers `firewall-domain` plugin placement, not completion RPC) | silent | `firewall-domain` is untouched. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `pkg/plugin/sdk/sdk_dispatch.go` | silent (page covers IKEv2 engine decisions, not completion RPC or SDK goroutine recovery) | silent | IKEv2 engine is untouched. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Plugin-to-daemon `dispatch-command` RPC returning both response data and an error.
- Any plugin background goroutine launched through `(*Plugin).Go` or `sdk.GoRecovered`.
- Tab completion on SSH (`WireArgCompletion`) and `ze cli` (`system:command-list` -> `wireDaemonArgCompletion` -> `handleArgComplete`).
- External plugin TLS handshake authentication (`AuthenticateWithLookup`).

### Transformation Path
1. `s.dispatcher.Dispatch` returns `(resp, dispatchErr)` -> `dispatchCommandPeerResponse` logs/reports error and returns `(resp, dispatchErr)` when `resp != nil` -> `handleDispatchCommand` preserves `resp`'s records, status, error, and warnings.
2. `p.Go(task, fn)` launches goroutine with `defer` recover -> on panic, logs task/value/stack to stderr and enqueues a non-blocking `sdk.Issue` (registered diagnostic event code) delivered via `p.RaiseIssue` -> worker exits cleanly while plugin stays up.
3. `commandRows` emits `Completable: cmd.Completable` -> `ze cli` wires `ArgumentHints` -> Tab calls `handleArgComplete` -> calls `conn.SendCompleteCommand` (`ze-plugin-callback:complete-command`) -> plugin `OnCompleteCommand` returns `[]Completion` without ever triggering `OnExecuteCommand`.
4. `tls_test.go` exercises `AuthenticateWithLookup` directly; dead `Authenticate` and `authenticateWithName` no longer exist.

### Boundaries Crossed
- Public `pkg/plugin/rpc` and `pkg/plugin/sdk` contract (`CompleteCommandInput`/`Output`, `OnCompleteCommand`, `(*Plugin).Go`) updated together with `internal/core/ipc/yang/ze-plugin-callback.yang` and `docs/architecture/api/process-protocol.md` + `docs/plugin-development/handlers.md` (`ai/doors/system.md`: deliberate contract change, pre-release with zero backward-compat shim).
- Process boundary: panic reports from `(*Plugin).Go` cross via `RaiseIssue` RPC, never direct `internal/core/report` calls, and never synchronously on a lock-holding loop.

### Integration Points
- `internal/core/ipc/yang/ze-plugin-callback.yang` (`rpc complete-command`).
- `pkg/plugin/rpc` (`MethodCompleteCommand`, `MuxConn.SendCompleteCommand`, `DirectBridge` complete-command slot).
- `pkg/plugin/sdk` (`OnCompleteCommand`, `(*Plugin).Go`, `sdk.GoRecovered`).
- `internal/component/plugin/server` (`dispatch.go`, `system.go`, `arg_completion.go`).
- `internal/core/diagnostic` (`RegisterEventCode` for plugin goroutine panic code).

### Changed-Wire Census

**Search evidence:** Searched `internal/component/plugin/`, `pkg/plugin/`, `internal/core/ipc/`, and `internal/component/cli/` (excluding `*_test.go`) with `git grep -n -E` for `handleArgComplete|SendExecuteCommand|MethodExecuteCommand|execute-command|commandRows|wireDaemonArgCompletion|decodeCommandList|system command list`. Positive controls confirmed: `internal/component/plugin/server/system.go:670` (`handleArgComplete` calling `conn.SendExecuteCommand`), `internal/component/plugin/ipc/rpc.go:404,476` (`methodExecuteCommand`, `PluginConn.SendExecuteCommand`), `pkg/plugin/rpc/bridge.go:642,673` (`ExecuteCommandHandler`, `DirectBridge.ExecuteCommand`), `pkg/plugin/sdk/sdk_dispatch.go:25,338` (`callbackExecuteCommand`, `answerExecuteCommand`), `internal/core/ipc/yang/ze-plugin-callback.yang:947` (`rpc execute-command`), `internal/component/plugin/server/system.go:399` (`commandRows`), and `internal/component/cli/client/inject.go:53,90` (`decodeCommandList`, `wireDaemonArgCompletion`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-plugin-callback:complete-command` | writer | `internal/component/plugin/server/system.go` `handleArgComplete`; `internal/component/plugin/ipc/rpc.go` `PluginConn.SendCompleteCommand`; `pkg/plugin/rpc/bridge.go` `DirectBridge` complete-command slot | `handleArgComplete` (L587, L670) currently overloads `conn.SendExecuteCommand` (`methodExecuteCommand` in `ipc/rpc.go:404`, `DirectBridge.ExecuteCommand` in `bridge.go:673`) | Change: switch `handleArgComplete` to `SendCompleteCommand` sending `rpc.CompleteCommandInput` (`serial`, `command`, `args`, `partial`) over socket RPC and `DirectBridge` with zero fallback onto `execute-command` |
| `ze-plugin-callback:complete-command` | reader | `pkg/plugin/sdk/sdk_callbacks.go` `OnCompleteCommand`; `pkg/plugin/sdk/sdk_dispatch.go` callback dispatcher; `internal/core/ipc/yang/ze-plugin-callback.yang` `rpc complete-command` | `sdk_dispatch.go:25,338` and `ze-plugin-callback.yang:947` currently declare only `execute-command` for plugin command callbacks | Change: add `rpc complete-command` in YANG (regenerated via `./le yang glue update`) and dispatch to `sdk.OnCompleteCommand`, returning `rpc.CompleteCommandOutput` (`completions` list of `{value, short-help}`) |
| `ze-plugin-callback:complete-command` | reader | External third-party plugins (outside repository) speaking the process protocol over TLS RPC | `ai/doors/system.md` and `ai/doors/plugin-infra.md` list external plugins speaking `ze-plugin-callback.yang` RPCs | Change (pre-release contract update): document `ze-plugin-callback:complete-command` and `OnCompleteCommand` in `docs/architecture/api/process-protocol.md` and `docs/plugin-development/handlers.md` in the same change |
| `ze-plugin-engine:system-command-list` | writer | `internal/component/plugin/server/system.go` `commandRows` | `system.go:399` emits builtin and plugin registry rows for `system command list` while omitting `Completable: cmd.Completable` on plugin rows | Change: populate the existing `completable` boolean field on plugin registry rows from `cmd.Completable` |
| `ze-plugin-engine:system-command-list` | reader | `internal/component/cli/client/main.go` + `internal/component/cli/client/inject.go` `decodeCommandList`, `wireDaemonArgCompletion` | `main.go:1081,1087,1129,1206,1214,1250` queries `system command list`, decodes `completable` in `inject.go:53`, and wires hints in `inject.go:90` | Preserve: reader already decodes `completable` and hangs live daemon argument hints on every command marked `completable: true` |

### Architectural Verification
- [ ] `ze-plugin-callback:complete-command` works identically over TLS socket (`MuxConn`) and in-process (`DirectBridge`)
- [ ] Tab on a plugin `Completable` command never invokes `OnExecuteCommand`
- [ ] `(*Plugin).Go` reports recovered panics via `RaiseIssue` (never direct `internal/core/report`) under a diagnostic code registered with `diagnostic.RegisterEventCode`
- [ ] `./le plugin boundary check`, `./le plugin declarations check`, and `./le doc yang-contract command-contract` pass

## Risks & Assumptions

### Assumptions
- Ze is pre-release (`ai/META-DECISIONS.md`), so adding `ze-plugin-callback:complete-command` directly without an `ExecuteCommand` fallback is the required pattern.
- `ze-plugin-callback.yang` regeneration runs via `./le yang glue update` (never hand-edited).

### Risks
- Changing `dispatchCommandPeerResponse` to return `(resp, dispatchErr)` requires auditing every caller of `dispatchCommandPeerResponse` and `dispatchCommandResponse` so none treats `resp != nil && err != nil` as a nil-deref or double-send.
- `(*Plugin).Go` must bound its asynchronous issue-reporting queue so a panic storm in a plugin goroutine cannot spawn unbounded goroutines or block the recovered goroutine's exit.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin command handler returning both records and error | → | `(*Server).dispatchCommandPeerResponse` | `TestDispatchCommandResponsePreservesDataOnError` (`internal/component/plugin/server/dispatch_test.go`) + `.ci` under `test/plugin/` |
| Panicking worker launched via `p.Go("worker", fn)` | → | `(*Plugin).Go` + `RaiseIssue` | `TestPluginGoRecoversPanicAndRaisesIssue` (`pkg/plugin/sdk/sdk_goroutine_test.go`) on both socket and DirectBridge |
| Tab completion via SSH and `ze cli` on a plugin `Completable` command | → | `commandRows` + `handleArgComplete` -> `SendCompleteCommand` -> `OnCompleteCommand` | `TestPluginCompletableUsesCompleteCommandRPC` (`internal/component/plugin/server/arg_completion_test.go`) proving `OnExecuteCommand` call count is 0 |
| Empty expected token in `internal/component/plugin/ipc` | → | `AuthenticateWithLookup` (after deleting `Authenticate`/`authenticateWithName`) | `internal/component/plugin/ipc/tls_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A dispatched plugin command returns both a non-nil `*plugin.Response` (holding records, status, error text, or warnings) and a non-nil `dispatchErr` (`error-path-discards-data-already-received`, 2026-09-08) | `dispatchCommandPeerResponse` returns `(resp, dispatchErr)` and its callers deliver the response's records, status, error sentence, and warnings to the operator, verified by `TestDispatchCommandResponsePreservesDataOnError` and `test/plugin/dispatch-error-preserves-records.ci` | `ErrSilent` continues answering `&plugin.Response{Status: plugin.StatusDone}, nil`, and a nil `*plugin.Response` with a non-nil `dispatchErr` continues returning `(nil, dispatchErr)` |
| AC-2 | A background goroutine launched via `(*Plugin).Go` or `sdk.GoRecovered` panics (`unguarded-goroutine-kills-the-process`, 2026-09-02) | The helper recovers the panic, writes the task name, panic value, and `debug.Stack()` to stderr, enqueues an asynchronous `RaiseIssue` report under the registered diagnostic event code when connected, and keeps the plugin process serving; all listed raw `go func()` plugin workers are wrapped through the SDK helper or `taskguard` | Panics occurring before Stage 1 handshake completes (or in standalone `sdk.GoRecovered`) log to stderr and skip `RaiseIssue` because no daemon RPC channel is open yet; in-tree network-input loops already guarded by `taskguard` keep using `taskguard` |
| AC-3 | Daemon builds `system command list` rows (`commandRows`) and serves Tab completion over SSH and `ze cli` for a plugin command declaring `Completable: true` (`unwired-feature`, 2026-11-11) | `commandRows` sets `Completable: cmd.Completable` on plugin registry rows, and both SSH (`WireArgCompletion`) and `ze cli` (`wireDaemonArgCompletion`) present Tab argument hints returned by the plugin | Builtin commands (`RPCRegistration.Complete`) continue winning name collisions over plugin commands and unauthorized callers remain filtered by `isAuthorizedCommandArgs` |
| AC-4 | Operator presses Tab on a plugin `Completable` command over SSH or `ze cli` on either socket RPC or `DirectBridge` (`field-carries-two-meanings`, 2026-11-15) | `handleArgComplete` dispatches `ze-plugin-callback:complete-command` (`SendCompleteCommand` / `DirectBridge` slot) to `sdk.OnCompleteCommand`, returning `[]Completion` while keeping `OnExecuteCommand` call count at 0 | None; Ze is pre-release so completion never falls back onto `ze-plugin-callback:execute-command` |
| AC-5 | Auditing `internal/component/plugin/ipc/tls.go` and running `internal/component/plugin/ipc/tls_test.go` with valid, invalid, and empty expected tokens (`production-function-only-tests-call`, 2026-11-11) | `AuthenticateWithLookup` is the sole IPC authenticator in `tls.go`, accepts valid constant-time SHA-256 tokens, rejects empty expected tokens with an auth error, and passes all migrated tests in `tls_test.go` after deleting `ipc.Authenticate` and `authenticateWithName` | None; neither `ipc.Authenticate` nor `authenticateWithName` has production callers, so zero legacy auth entry points remain |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestDispatchCommandResponsePreservesDataOnError` | `internal/component/plugin/server/dispatch_test.go` | AC-1 |
| `TestPluginGoRecoversPanicAndRaisesIssue` | `pkg/plugin/sdk/sdk_goroutine_test.go` | AC-2 |
| `TestCommandRowsMarksPluginCompletable` | `internal/component/plugin/server/system_test.go` | AC-3 |
| `TestPluginCompletableUsesCompleteCommandRPC` | `internal/component/plugin/server/arg_completion_test.go` | AC-3, AC-4 |
| `TestCompleteCommandDirectBridgeAndSocket` | `pkg/plugin/sdk/sdk_complete_test.go` | AC-4 |
| Updated auth tests on `AuthenticateWithLookup` | `internal/component/plugin/ipc/tls_test.go` | AC-5 |

### Functional Tests

| Test | AC |
|------|----|
| `.ci` under `test/plugin/` asserting a plugin command returning both records and an error delivers both to the CLI | AC-1 |
| `.ci` under `test/plugin/` verifying Tab completion on a forked `Completable` plugin calls `complete-command` without executing the command | AC-3, AC-4 |

## Files to Modify
- `internal/component/plugin/server/dispatch.go`
- `internal/component/plugin/server/dispatch_test.go`
- `internal/component/plugin/server/system.go`
- `internal/component/plugin/server/arg_completion.go`
- `internal/component/plugin/ipc/tls.go`
- `internal/component/plugin/ipc/tls_test.go`
- `internal/core/ipc/yang/ze-plugin-callback.yang`
- `internal/core/diagnostic/codes.go` (register plugin goroutine panic event code)
- `pkg/plugin/rpc/types.go`
- `pkg/plugin/rpc/mux.go`
- `pkg/plugin/rpc/bridge.go`
- `pkg/plugin/sdk/sdk.go`
- `pkg/plugin/sdk/sdk_callbacks.go`
- `pkg/plugin/sdk/sdk_dispatch.go`
- `docs/architecture/api/process-protocol.md`
- `docs/architecture/cli/command-completion.md`
- `docs/plugin-development/handlers.md`

## Files to Create
- `pkg/plugin/sdk/sdk_goroutine.go`
- `pkg/plugin/sdk/sdk_goroutine_test.go`
- `pkg/plugin/sdk/sdk_complete_test.go`
- `test/plugin/dispatch-error-preserves-records.ci`

### Integration Checklist
- [ ] Error-path preservation verified through `dispatchCommandPeerResponse` -> `handleDispatchCommand` -> CLI
- [ ] `ze-plugin-callback:complete-command` verified on both socket RPC and `DirectBridge`
- [ ] YANG glue regenerated via `./le yang glue update`
- [ ] `(*Plugin).Go` verified on both internal and forked plugins with zero direct `internal/core/report` calls

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/api/process-protocol.md` - document `ze-plugin-callback:complete-command`
- [ ] `docs/architecture/cli/command-completion.md` - document plugin `Completable` path on SSH and `ze cli`
- [ ] `docs/plugin-development/handlers.md` - document `OnCompleteCommand` and `(*Plugin).Go`

## Design presentation

### What changes
- `dispatchCommandPeerResponse` preserves non-nil `*plugin.Response` beside `dispatchErr` so callers retain records, status, error text, and warnings.
- `pkg/plugin/sdk` adds `(*Plugin).Go` and `sdk.GoRecovered` for panic containment with stderr logging and non-blocking `RaiseIssue` reporting under a registered diagnostic code, and guards remaining unguarded plugin goroutines.
- `commandRows` populates `Completable: cmd.Completable` on registry rows, and `handleArgComplete` moves from overloaded `ExecuteCommand` to a dedicated `ze-plugin-callback:complete-command` RPC on both socket and `DirectBridge`.
- Test-only `ipc.Authenticate` and `authenticateWithName` are deleted and `tls_test.go` moves to `AuthenticateWithLookup`.

### Before and after
- **Before:** A handler returning both records and a Go error lost its entire response in `dispatchCommandPeerResponse`; plugin-spawned goroutines crashed the plugin process on panic; `ze cli` skipped plugin `Completable` commands; Tab on a plugin command sent `ExecuteCommand`; `ipc.Authenticate` and `authenticateWithName` lingered with empty-token bypasses.
- **After:** Records and errors both reach the operator; panics in SDK-managed plugin goroutines are contained, logged, and raised via `RaiseIssue`; SSH and `ze cli` both Tab-complete plugin `Completable` commands through a dedicated `complete-command` RPC that never executes the command; `AuthenticateWithLookup` is the single IPC authenticator.

### Alternatives set aside
- **Adding a `complete: true` boolean flag to `ExecuteCommandInput`:** Rejected because `field-carries-two-meanings` explicitly warns against overloading execution with completion; a dedicated `complete-command` RPC keeps execution and completion separate in YANG, `rpc`, and `sdk`.
- **Keeping `ipc.Authenticate` with an added empty-token check:** Rejected because it has zero production callers; deleting dead production helpers eliminates the maintenance surface entirely.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Write red unit tests for AC-1 to AC-5 and confirm each fails at HEAD.
2. Fix `dispatchCommandPeerResponse` and audit every caller to preserve non-nil `*plugin.Response` beside `dispatchErr`.
3. Register the plugin-goroutine-panic diagnostic event code, implement `pkg/plugin/sdk/sdk_goroutine.go` (`(*Plugin).Go` with bounded async `RaiseIssue` queue and `sdk.GoRecovered`), and guard remaining raw `go func()` plugin workers.
4. Add `rpc complete-command` to `ze-plugin-callback.yang`, run `./le yang glue update`, wire `pkg/plugin/rpc` + `DirectBridge` + `pkg/plugin/sdk` `OnCompleteCommand`, set `Completable: cmd.Completable` in `commandRows`, and switch `handleArgComplete` to `SendCompleteCommand`.
5. Delete `ipc.Authenticate` and `authenticateWithName` from `internal/component/plugin/ipc/tls.go` and migrate `tls_test.go` to `AuthenticateWithLookup`.
6. Update docs, verify scoped tests and plugin/YANG gates, and mark the five owned journal rows `fixed:` in `plan/journal/`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `error-path-discards-data-already-received` | 2026-09-08 | `(*Server).dispatchCommandResponse` |
| `unguarded-goroutine-kills-the-process` | 2026-09-02 | plugin goroutines |
| `unwired-feature` | 2026-11-11 | plugin `Completable` commands get no Tab completion: `WireArgCompletion` and `commandRows` cover builtins only |
| `field-carries-two-meanings` | 2026-11-15 | plugin `Completable` completion request is an `ExecuteCommand`, indistinguishable from running the command |
| `production-function-only-tests-call` | 2026-11-11 | `ipc.Authenticate`, `authenticateWithName` test-only, still accept an empty expected token |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Children 1, 2, and 4 through 7.

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] All five owned journal rows marked `fixed:` in `plan/journal/`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label plugin-infra-fixes command go test -race ./internal/component/plugin/... ./pkg/plugin/...`, `./le plugin boundary check`, `./le plugin declarations check`, `./le yang glue check`, `./le test functional plugin`
