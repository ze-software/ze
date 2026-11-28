# Spec: Daemon runtime one-off defects (4) - Hub MCP dormant settings, managed auth CR framing, SIGHUP queueing, and ServesClient readiness

| Field | Value |
|-------|-------|
| Status | design |
| Scope | hub |
| Priority | P1 |
| Release reason | Fixes five open hub and managed-client runtime defects: `ze.mcp.enabled` overwriting a dormant `mcp` block's listener, `managed` `readLine` stripping CR off the auth wire, SIGHUP dropping behind staged SSH/web candidates and racing on transaction completion, and hub start publishing the data RPC target before `ServesClient` is bound |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the five open `cmd/ze/hub/` and `internal/component/managed/` journal rows routed from the umbrella.

## Task

Fix five hub-engine and fleet-client runtime defects verified open at HEAD on 2026-11-26:

1. **Honor dormant `mcp` block listener under `ze.mcp.enabled` without violating the exposure guard or "Ze always starts" (`enabled-gate-discards-settings`, 2026-08-10).** In `cmd/ze/hub/main.go` (L793-795), `env.IsEnabled("ze.mcp.enabled")` eagerly synthesizes `mcpAddrs = []string{"127.0.0.1:8080"}` before reading the config tree, so a dormant `mcp { enabled false; server ... }` block's configured address/port is silently replaced with `127.0.0.1:8080`. Track `mcpEnabledByEnv := env.IsEnabled("ze.mcp.enabled")` separately (mirroring `webEnabled` at L825-839), read `mcpSettings.Servers` from `ExtractMCPSettings(loadResult.Tree)` when `mcpEnabledByEnv && len(mcpAddrs) == 0`, fall back to `127.0.0.1:8080` only when neither CLI/env nor the config block named a server, and pass every resulting MCP endpoint through the existing per-listener boot exposure guard (`RegisterServiceBlock` / per-endpoint exclusion with its `ze doctor` code — never exiting `ze start`, honoring the 2026-11-26 "Ze always starts" ruling in `docs/architecture/core-design.md`). Because `ExtractMCPSettings` already folds the block's `auth-mode`/`token`/`identities`/`oauth`/`tls`, an unauthenticated non-loopback endpoint from a dormant block is excluded and reported while loopback or authenticated endpoints serve normally.
2. **Stop stripping trailing `\r` in fleet hub auth line reader (`guard-added-to-one-half-of-a-pair`, 2026-08-22).** `readLine` (`internal/component/managed/client.go` L1095-1118) still strips a trailing `\r` before `\n`, contradicting the 2026-08-22 owner wire ruling enforced across `rpc.FrameReader`, `ipc.ReadLineRaw`, SSH stream readers, and the ExaBGP bridge: every wire line ends with `\n` alone and no reader strips `\r`. Remove the `\r` strip in `readLine` (or reuse `ipc.ReadLineRaw` semantics) so a `\r\n` auth response is left intact and rejected as malformed framing, and update `TestReadLine` in `internal/component/managed/client_test.go` (recording an independent `ze-test-judge` approval because an existing test case named `CRLF stripped` is replaced).
3. **Queue SIGHUP behind staged SSH/web commit candidates (`silent-fall-through`, 2026-10-19).** In `cmd/ze/hub/main_reload.go` (`handleSIGHUPReload`, L171-182), the retry loop queues only when `errors.Is(err, pluginserver.ErrReloadInProgress)`. When an SSH or web commit has staged a candidate, `stageSIGHUPCandidate` fails with `storage.ErrCandidateExists` ("candidate config already staged"), which breaks out of the loop and drops the SIGHUP, contradicting `docs/architecture/config/transaction-protocol.md`. Queue on `errors.Is(err, storage.ErrCandidateExists)` as well as `pluginserver.ErrReloadInProgress`, waiting for the active commit/candidate slot to clear before replaying the queued SIGHUP once.
4. **Fix `-race` wakeup latency/miss in `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` (`queued-request-waits-for-an-unrelated-trigger`, 2026-09-26).** Audit `handleSIGHUPReload` -> `awaitConfigTransaction` -> `s.ConfigTransactionDone()` (`cmd/ze/hub/main_reload.go` L267-291) and `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` (`cmd/ze/hub/main_reload_test.go` L531+) for the race where `ConfigTransactionDone()` is sampled after a transaction ends or before a second waiter observes the notification, ensuring a queued SIGHUP wakes immediately on transaction completion without waiting for an unrelated trigger or timing out under `-race`.
5. **Gate hub data RPC `ServesClient` until managed server binds (`session-up-before-its-subscriber`, 2026-11-19).** In `cmd/ze/hub/main.go`, `storagecli.InstallDataRPC(dataTarget)` runs at L1301 with `ServesClient == nil`, while `startManagedServer` binds `serving.ServesClient = managedServer.Serves` only at L1823-1832. During that boot window, any config import, restore, or SSH session targeting a declared managed client fails falsely with "this daemon serves no managed client". When the boot config declares managed clients (`len(hubConfig.Clients) > 0`), bind a startup-gated `ServesClient` (or defer client-scoped data RPC target publication / answer a clear "managed server still starting; retry" readiness error until `startManagedServer` publishes the live `Serves` function), and clear the gate atomically once `managedServer` is up (or closed on failure).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, startup boundaries
- [ ] `ai/doors/hub-engine.md` - boot sequence, one reload path, managed client and server
- [ ] `docs/architecture/core-design.md` - Section 18 and `## Design decisions` (2026-11-26 "Ze always starts")
- [ ] `docs/architecture/hub-architecture.md` - daemon startup and reload flow
- [ ] `docs/architecture/fleet-config.md` - managed client/hub auth wire and `## Design decisions`
- [ ] `docs/architecture/config/transaction-protocol.md` - SIGHUP queueing behind active transactions and commits

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `cmd/ze/hub/main.go` - L793-795 sets `mcpAddrs = []string{"127.0.0.1:8080"}` immediately on `ze.mcp.enabled`, bypassing `ExtractMCPSettings`'s `Servers`; L1301 calls `storagecli.InstallDataRPC(dataTarget)` with `ServesClient: nil` before `startManagedServer` at L1823-1832 re-installs `serving.ServesClient`.
- [ ] `internal/component/managed/client.go` - `readLine` (L1095-1118) strips trailing `\r` before `\n`; `client_test.go` L357 `TestReadLine` asserts `CRLF stripped`.
- [ ] `cmd/ze/hub/main_reload.go` - `handleSIGHUPReload` (L171-182) only queues on `errors.Is(err, pluginserver.ErrReloadInProgress)` and drops SIGHUP when `stageSIGHUPCandidate` returns `storage.ErrCandidateExists`; `awaitConfigTransaction` (L267-291) polls `s.ConfigTransactionDone()`.

**Behavior to preserve:**
- "Ze always starts" (`docs/architecture/core-design.md` 2026-11-26): an unauthenticated non-loopback MCP endpoint never exits `ze start`; only the blocked endpoint stays down under `RegisterServiceBlock` and its `ze doctor` code, while valid endpoints and the rest of Ze start normally.
- `ze.mcp.listen` and `--mcp` continue to outrank both `ze.mcp.enabled` and the config block's `server` list.
- SIGHUP still runs through the one reload path (`doReload` / `doReloadContext`) and never loops forever on shutdown (`ctx.Done()` / closed `reloadCh`).
- A hub with NO declared managed clients continues to answer "this daemon serves no managed client" immediately.

**Behavior to change:**
- `ze.mcp.enabled` reads a dormant `mcp` block's `Servers` before falling back to `127.0.0.1:8080`, subject to the existing per-listener exposure guard.
- `managed` `readLine` preserves `\r` bytes instead of stripping them, matching `ipc.ReadLineRaw`.
- `handleSIGHUPReload` queues on `storage.ErrCandidateExists` as well as `ErrReloadInProgress` and wakes reliably when the holding transaction or candidate commit finishes.
- A hub configured with managed clients never answers "this daemon serves no managed client" during startup before `startManagedServer` binds `ServesClient`.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Web dormant-settings split and per-listener exposure guard | `cmd/ze/hub/main.go` `webEnabled`, `ExtractWebSettings`, `RegisterServiceBlock`; `cmd/ze/hub/mgmt_guard.go` | Keeps `webEnabled` separate from `webAddrs`, reads dormant block `Servers` only when enabled, and excludes blocked endpoints without exiting `ze start` | Extends: applies the same `mcpEnabled` split to `ze.mcp.enabled` + `ExtractMCPSettings` while gating every endpoint through the existing exposure guard |
| Raw newline-only auth line reader | `internal/component/plugin/ipc/tls.go` `ReadLineRaw`; `internal/component/managed/client.go` `readLine` | `ipc.ReadLineRaw` reads until `\n` without stripping `\r`; `managed` `readLine` still strips `\r` | Replaces: aligns `managed` `readLine` with `ipc.ReadLineRaw` (no `\r` stripping) |
| One SIGHUP reload worker | `cmd/ze/hub/main_reload.go` `handleSIGHUPReload`, `sighupReload`, `awaitConfigTransaction` | Queues SIGHUP on `ErrReloadInProgress` and waits on `s.ConfigTransactionDone()`, but drops SIGHUP on `storage.ErrCandidateExists` | Extends: queues on `storage.ErrCandidateExists` too and fixes candidate-slot / transaction-done wakeup under `-race` |
| Hub data RPC target publication | `cmd/ze/hub/main.go` `storagecli.InstallDataRPC`, `startManagedServer` | Publishes `dataTarget` with `ServesClient: nil` at L1301 and overwrites it at L1831 after `startManagedServer` returns | Extends: gates `ServesClient` readiness while `startManagedServer` is booting when managed clients are declared |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A config change is a transaction of stream events (verify, apply, rollback) that each plugin answers for its own roots" | agrees | SIGHUP queueing behind `storage.ErrCandidateExists` waits on the active commit/transaction lifecycle rather than bypassing or racing it. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering `cmd/ze/hub/` and `internal/component/managed/` | "SIGHUP and a managed push both reach `doReload`" | agrees | Keeps SIGHUP queueing inside `handleSIGHUPReload` -> `doReloadContext` with no second reload path. |
| `docs/architecture/hub-architecture.md` | Vision | `// Design:` page of `cmd/ze/hub/main.go` and `main_reload.go` | "A **plugin-based configuration system** where Ze acts as a central Hub orchestrating separate processes." | agrees | Repairs MCP dormant-block settings, SIGHUP queueing, and `ServesClient` startup ordering within the single hub runtime. |
| `docs/architecture/config/transaction-protocol.md` | 2. Transaction Phases | modified by this spec | "A config transaction has three runtime phases." | agrees | Queues SIGHUP on active `storage.ErrCandidateExists` commits as well as `ErrReloadInProgress` so SIGHUP obeys the documented transaction serialization, and updates this page in the same change. |
| `docs/architecture/fleet-config.md` | Vision | `// Design:` page of `internal/component/managed/client.go`; modified by this spec | "A managed client's config contains everything: BGP sessions, plugin settings, AND the hub connection info." | agrees | Enforces LF-only auth line framing in `managed` `readLine` and gates `ServesClient` data RPC readiness during hub startup, updating this page in the same change. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Live managed certificate reload is untouched; startup `ServesClient` gating completes before ready publication without altering certificate reload. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Gating `ServesClient` during hub boot prevents false "serves no managed client" refusals on managed-device config imports and sessions. |
| `docs/architecture/core-design.md` | Executive Summary | anchored to `cmd/ze/hub/main.go` | "`ze` and `le` are personalities of the single `cmd/ze` codebase." | agrees | Changes stay cleanly in `cmd/ze/hub/` and `internal/component/managed/` without coupling to `internal/le`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Data-plane compatibility validation is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Data-plane mismatch verdicts at commit, reload, and validate are untouched (and start follows the 2026-11-26 superseding row below). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Firewall VPP verifier rules are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Data-plane selector names are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP export next-hop rewriting is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP next-hop authoring forms are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | BGP built-in chain steps are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP raw-override provenance is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP IPv4/IPv6 next-hop validation is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP originated vs forwarded next-hop rules are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Implied-filter display is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Honoring a dormant `mcp` block's endpoints under `ze.mcp.enabled` gates each endpoint through the per-listener exposure guard (`RegisterServiceBlock` + doctor code) so an unauthenticated non-loopback endpoint stays down while other endpoints and `ze start` still run. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Preserves the start-time non-fatal behavior and keeps reload refusals intact. |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `cmd/ze/hub/main.go` | silent (page covers API transport and bus participant filters, not `ze.mcp.enabled` dormant address precedence or SIGHUP candidate queueing) | silent | API transport and bus filter semantics are unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Implied-config rendering is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Bus participant registrations are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Bus send-destination rules are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Bus receive-source rules are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus allow-lists are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Bus filter compilation is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | No bus arbitration is added. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Bus display output is untouched. |
| `docs/architecture/api/commands.md` | Overview | anchored to `cmd/ze/hub/main.go` | silent (page covers command grammar and dispatch keys, not hub MCP env precedence or SIGHUP reload queueing) | silent | Command paths and answer formats are untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Command pipe grammar is untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Pipe overrides are untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Pipe execution semantics are untouched. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `cmd/ze/hub/main.go` | "Ze plugins communicate with the engine via newline-framed YANG RPCs over a" | agrees | Aligning `managed` `readLine` to strict LF-only framing matches the newline-framed protocol without CR stripping. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Engine operation authorization is untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Event delivery queues are untouched. |
| `docs/architecture/pki/tls-listeners.md` | Decision: the leaf comes first, then the intermediates | anchored to `cmd/ze/hub/main.go` | silent (page covers PKI certificate chain ordering and TLS listeners, not MCP dormant address fallback or SIGHUP candidate queueing) | silent | PKI material resolution and listener TLS handling remain intact. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Looking-glass HTTP/HTTPS per-entry selection is untouched. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | `listenerTLSMaterial` and `managedCertificate` are untouched. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | MCP per-endpoint exclusion under `ze.mcp.enabled` follows the exact same non-fatal start and per-endpoint isolation model. |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to `cmd/ze/hub/main.go` | silent (page covers SSH `ze-config-upload` appliance delivery, not hub MCP env precedence or SIGHUP candidate queueing) | silent | Appliance remote upload paths are untouched. |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `cmd/ze/hub/main.go` | silent (signal table lists SIGHUP reload semantics without detailing internal `ErrCandidateExists` staging lock queueing) | silent | Keeps SIGHUP as the config reload trigger and ensures signals during active commits are queued instead of dropped. |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `cmd/ze/hub/main.go` | silent (page covers CLI Tab completion producers, not hub MCP startup or SIGHUP queueing) | silent | Completion producers are untouched. |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `cmd/ze/hub/main.go` | silent (page covers config file grammar, not hub runtime SIGHUP queueing or managed `readLine`) | silent | Config syntax is untouched. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `cmd/ze/hub/main.go` | silent (page covers YANG module loading and validation layers, not hub SIGHUP queueing or managed auth line framing) | silent | YANG schema loading is untouched. |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | anchored to `cmd/ze/hub/main.go` | silent (page covers stderr/crash output files, not MCP dormant settings or `ServesClient` readiness) | silent | Crash capture is untouched. |
| `docs/architecture/doctor-and-health-checks.md` | BGP policy absences in the supplied configuration | anchored to `cmd/ze/hub/main.go` | silent (page covers doctor check taxonomy, not SIGHUP queueing or managed `readLine`) | silent | Blocked MCP endpoints continue reporting through the existing `RegisterServiceBlock` / doctor check. |
| `docs/architecture/pki/pki-store.md` | Decisions | anchored to `cmd/ze/hub/main.go` | silent (page covers PKI store persistence and local CA, not MCP dormant settings or SIGHUP queueing) | silent | PKI store contracts are untouched. |
| `docs/architecture/storage-backends.md` | The memory encoding | anchored to `cmd/ze/hub/main.go` | silent (page covers tree/blob/memory storage encodings, not SIGHUP queueing or `ServesClient` gating) | silent | Storage backends and `ErrCandidateExists` semantics are preserved unchanged. |
| `docs/architecture/system-architecture.md` | System RIB process boundary | anchored to `cmd/ze/hub/main.go` | silent (page covers system RIB selection boundary, not hub MCP settings or SIGHUP queueing) | silent | System RIB boundary is untouched. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `cmd/ze/hub/main.go` | silent (page covers `.ci` test syntax, not hub runtime code) | silent | Reload `.ci` tests follow standard `.ci` format. |
| `docs/architecture/web-interface.md` | Component Structure | anchored to `cmd/ze/hub/main.go` | silent (page covers web templ UI, not MCP dormant settings or `managed` `readLine`) | silent | Web commit path benefits from SIGHUP queueing behind staged candidates instead of dropping SIGHUP. |
| `docs/architecture/aaa-tacacs.md` | A registry, not a hardcoded chain | anchored to `cmd/ze/hub/main_reload.go` | silent (page covers AAA/TACACS+ backend chaining, not SIGHUP candidate queueing) | silent | AAA reload lifecycle is untouched. |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to `internal/component/managed/client.go` | silent (page covers public `pkg/ze` engine interfaces, not managed auth line CR framing) | silent | Public `pkg/ze` interfaces are untouched. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze start` with `ze.mcp.enabled=1` and a dormant `mcp { enabled false; server ... }` block.
- Managed client TLS connect-back reading the hub's `#0 ok` / `#0 err` auth line.
- `SIGHUP` delivered during an active config transaction or staged SSH/web candidate commit.
- Managed-client config import/restore or SSH session arriving during hub boot before `startManagedServer` completes.

### Transformation Path
1. `ze.mcp.enabled` sets `mcpEnabledByEnv = true` -> `ExtractMCPConfig` / `ExtractMCPSettings` fills `mcpAddrs` from dormant `Servers` when `mcpEnabledByEnv && len(mcpAddrs) == 0` -> defaults to `127.0.0.1:8080` only if still empty -> per-listener exposure guard admits valid endpoints and excludes unauthenticated non-loopback endpoints via `RegisterServiceBlock` without exiting `ze start`.
2. Hub sends `#0 ok\n` -> `managed` `readLine` reads bytes until `\n` without stripping `\r` -> `#0 ok\r\n` reaches `rpc.ParseLine` with `\r` intact and fails framing validation.
3. SIGHUP hits `ErrReloadInProgress` or `storage.ErrCandidateExists` -> `handleSIGHUPReload` logs queueing and awaits transaction / candidate-slot completion (coalescing additional SIGHUPs and honoring `ctx.Done()`) -> replays reload once when the holder finishes.
4. `runYANGConfig` with declared managed clients publishes a readiness-aware `DataRPCTarget` -> requests arriving before `startManagedServer` finishes wait on or receive an explicit starting/not-ready answer rather than "serves no managed client"; once `startManagedServer` starts, `ServesClient` resolves immediately against `managedServer.Serves`.

### Boundaries Crossed
- Hub-to-managed-client auth wire (`internal/component/managed/client.go` reading what `internal/component/plugin/ipc/tls.go` writes).
- Hub startup sequencing between `storagecli.InstallDataRPC` and `startManagedServer`.

### Integration Points
- `cmd/ze/hub/main.go`, `cmd/ze/hub/mgmt_guard.go`, `cmd/ze/hub/main_reload.go`, `cmd/ze/hub/managed_server.go`.
- `internal/component/managed/client.go`.
- `internal/component/config/storage` (`ErrCandidateExists`).

### Changed-Wire Census

**Search evidence:** Searched `internal/component/managed/`, `internal/component/plugin/ipc/`, `internal/component/plugin/server/managed_serve.go`, and `pkg/plugin/sdk/` (excluding `*_test.go`) with `git grep -n -E` for `func readLine|readLine\(|ReadLineRaw|AuthenticateWithLookup|writeErrorRaw`. Positive controls confirmed: `internal/component/plugin/ipc/tls.go:159,241,263` (`AuthenticateWithLookup`, `ReadLineRaw`, `writeErrorRaw`), `internal/component/plugin/server/managed_serve.go:758` (`pluginipc.AuthenticateWithLookup`), `internal/component/managed/client.go:542,1568` (`readLine`), and `pkg/plugin/sdk/sdk.go:520` (`ipc.ReadLineRaw`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `fleet-hub-auth-line` | writer | `internal/component/plugin/ipc/tls.go` `AuthenticateWithLookup`, `writeErrorRaw` (called by `internal/component/plugin/server/managed_serve.go:758`) | `tls.go:159,263` terminates every `#0 ok` / `#0 err` auth response frame with a single `\n` (LF only, no CR) | Preserve: writer already emits strict LF-only frames |
| `fleet-hub-auth-line` | reader | `internal/component/managed/client.go` `readLine` (called at L542) | `client.go:1568` reads auth response bytes until `\n` and currently strips a trailing `\r` | Change: stop stripping `\r` in `readLine` (or reuse `ipc.ReadLineRaw` semantics) so `\r\n` frames reach `rpc.ParseLine` intact and fail framing validation |
| `fleet-hub-auth-line` | reader | `pkg/plugin/sdk/sdk.go` `ipc.ReadLineRaw` (L520) | Sibling TLS connect-back auth reader calls `ipc.ReadLineRaw` (`tls.go:241`), which already enforces LF-only framing without stripping `\r` | Preserve: already complies with strict LF-only auth framing |

### Architectural Verification
- [ ] `ze.mcp.enabled=1` with a dormant unauthenticated non-loopback `mcp` block does NOT exit `ze start` (2026-11-26 "Ze always starts"); it excludes the blocked endpoint via `RegisterServiceBlock` and reports under the doctor code
- [ ] `managed` `readLine` rejects or preserves `\r` verbatim with zero CRLF stripping
- [ ] SIGHUP during an SSH/web commit queues and runs once when the commit finishes
- [ ] Hub startup never returns "serves no managed client" for a declared managed client while `startManagedServer` is still starting

## Risks & Assumptions

### Assumptions
- Changing `TestReadLine` in `internal/component/managed/client_test.go` modifies an existing test assertion (`CRLF stripped`), so implementation MUST obtain an independent `ze-test-judge` approval before editing `client_test.go`.
- If a staged candidate is left abandoned by a crashed editor rather than an active commit, the SIGHUP wait must be bounded or signaled by candidate-lock / mutation-owner completion so `handleSIGHUPReload` never livelocks; if the candidate remains staged with no active holder, return the `ErrCandidateExists` error clearly.

### Risks
- Waiting on `storage.ErrCandidateExists` without distinguishing an active mutation holder from a stale on-disk candidate could spin or hang `handleSIGHUPReload`. Hook the wait onto the daemon's `configMutationOwner` / candidate completion channel (falling through immediately to log the error when no in-flight commit holds the candidate).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze.mcp.enabled=1` with dormant `mcp { enabled false; server ... }` block | → | `cmd/ze/hub/main.go` MCP address resolution + exposure guard | `TestMCPEnabledHonorsDormantBlockUnderExposureGuard` (`cmd/ze/hub/mgmt_guard_test.go`) |
| Managed client auth response containing `\r\n` | → | `internal/component/managed/client.go` `readLine` | `TestReadLinePreservesCR` (`internal/component/managed/client_test.go`) |
| SIGHUP during active SSH/web commit staging a candidate | → | `cmd/ze/hub/main_reload.go` `handleSIGHUPReload` | `TestSIGHUPQueuedBehindStagedCandidateRunsOnce` (`cmd/ze/hub/main_reload_test.go`) + `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` under `-race -count=10` |
| Data RPC targeting a declared managed client before `startManagedServer` binds | → | `cmd/ze/hub/main.go` readiness-gated `DataRPCTarget` | `TestHubDataRPCDoesNotRefuseDeclaredClientBeforeManagedServerStarts` (`cmd/ze/hub/managed_hub_freeze_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Starting `ze start` with `ze.mcp.enabled=1` and a dormant `mcp { enabled false; server ... }` block (`enabled-gate-discards-settings`, 2026-08-10) | Hub reads the dormant block's configured `Servers` and auth/TLS settings, binds loopback or authenticated endpoints on the block's configured addresses (falling back to `127.0.0.1:8080` only when no server is configured), and excludes any unauthenticated non-loopback endpoint via `RegisterServiceBlock` and its doctor code while `ze start` keeps running | Explicit `ze.mcp.listen` and `--mcp` flags continue outranking both `ze.mcp.enabled` and the config block's `server` list |
| AC-2 | Managed client receives an auth response frame containing `\r\n` vs `\n` (`guard-added-to-one-half-of-a-pair`, 2026-08-22) | `managed` `readLine` returns the raw bytes up to `\n` with any `\r` byte preserved verbatim so `#0 ok\n` succeeds and `#0 ok\r\n` fails framing validation in `rpc.ParseLine`, matching `ipc.ReadLineRaw` and `rpc.FrameReader` (`ze-test-judge` approval recorded for updating `TestReadLine`) | None; strict LF-only wire framing applies to every auth line from the hub |
| AC-3 | `SIGHUP` arrives while an active SSH or web commit holds a staged candidate (`storage.ErrCandidateExists`) vs when an abandoned stale candidate exists (`silent-fall-through`, 2026-10-19) | `handleSIGHUPReload` queues the SIGHUP behind an active commit holder and executes the reload once when that commit completes, and logs the `storage.ErrCandidateExists` error immediately when no active mutation holder owns the candidate | Multiple SIGHUPs arriving while one commit is in progress coalesce into a single queued reload, and daemon shutdown (`ctx.Done()`) cancels waiting reloads cleanly |
| AC-4 | Running `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` under `go test -race -count=10 ./cmd/ze/hub/...` (`queued-request-waits-for-an-unrelated-trigger`, 2026-09-26) | Every iteration wakes promptly when the holding transaction completes and applies the queued reload cleanly with zero race warnings or timeouts | None; every queued SIGHUP must wake directly from transaction or candidate completion rather than an unrelated trigger |
| AC-5 | A data RPC (`import`, `restore`, or SSH session) targets a declared managed client during `ze start` before `startManagedServer` finishes binding `ServesClient` (`session-up-before-its-subscriber`, 2026-11-19) | Hub answers with readiness-gated handling (waiting for startup completion or returning an explicit retryable starting error) during the boot window and resolves `managedServer.Serves` immediately once `startManagedServer` binds | Hubs with zero declared managed clients (`len(hubConfig.Clients) == 0`) continue answering "this daemon serves no managed client" immediately |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestMCPEnabledHonorsDormantBlockUnderExposureGuard` | `cmd/ze/hub/mgmt_guard_test.go` | AC-1 |
| `TestReadLine` (updated with `ze-test-judge` approval) | `internal/component/managed/client_test.go` | AC-2 |
| `TestSIGHUPQueuedBehindStagedCandidateRunsOnce` | `cmd/ze/hub/main_reload_test.go` | AC-3 |
| `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` | `cmd/ze/hub/main_reload_test.go` | AC-4 |
| `TestHubDataRPCDoesNotRefuseDeclaredClientBeforeManagedServerStarts` | `cmd/ze/hub/managed_hub_freeze_test.go` | AC-5 |

### Functional Tests

| Test | AC |
|------|----|
| `.ci` under `test/reload/` sending SIGHUP during an active commit and verifying queued reload applies once | AC-3, AC-4 |

## Files to Modify
- `cmd/ze/hub/main.go`
- `cmd/ze/hub/main_reload.go`
- `cmd/ze/hub/main_reload_test.go`
- `cmd/ze/hub/mgmt_guard_test.go`
- `cmd/ze/hub/managed_hub_freeze_test.go`
- `internal/component/managed/client.go`
- `internal/component/managed/client_test.go`
- `docs/architecture/config/transaction-protocol.md`
- `docs/architecture/fleet-config.md`

## Files to Create
- None.

### Integration Checklist
- [ ] `ze.mcp.enabled=1` with dormant `mcp` block tested against both loopback and unauthenticated non-loopback addresses (daemon stays up)
- [ ] SIGHUP queueing tested behind both `ErrReloadInProgress` and `storage.ErrCandidateExists`
- [ ] `ServesClient` readiness verified before and after `startManagedServer`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/config/transaction-protocol.md` - confirm SIGHUP queueing covers both active transactions and staged SSH/web commits
- [ ] `docs/architecture/fleet-config.md` - document LF-only auth line framing and hub `ServesClient` startup readiness

## Design presentation

### What changes
- Separates `mcpEnabledByEnv` from `mcpAddrs` in `cmd/ze/hub/main.go` so `ze.mcp.enabled` reads a dormant `mcp` block's `Servers` before falling back to `127.0.0.1:8080`, gated per-listener by the boot exposure guard without exiting `ze start`.
- Removes `\r` stripping from `internal/component/managed/client.go` `readLine`.
- Extends `handleSIGHUPReload` to queue behind active `storage.ErrCandidateExists` commits and fixes the `-race` transaction-end wakeup.
- Gates `DataRPCTarget.ServesClient` readiness during hub boot when managed clients are declared.

### Before and after
- **Before:** `ze.mcp.enabled=1` silently ignored a dormant `mcp` block's configured port; `managed` `readLine` stripped `\r`; SIGHUP during an SSH/web commit failed with "candidate config already staged" and dropped; `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` could miss wakeup under `-race`; hub boot briefly refused declared managed clients as "serves no managed client".
- **After:** Dormant `mcp` block settings are honored under per-listener exposure gating ("Ze always starts"); `managed` `readLine` enforces strict LF-only wire framing; SIGHUP queues behind both transactions and active commits and wakes promptly; declared managed clients never see a false "serves no managed client" window during boot.

### Alternatives set aside
- **Leaving `ze.mcp.enabled` hardcoded to `127.0.0.1:8080`:** Rejected because it silently discards the operator's configured `mcp` server port while the existing per-listener exposure guard (`RegisterServiceBlock`) already prevents unauthenticated remote exposure safely without exiting `ze start`.
- **Polling `storage.ErrCandidateExists` with a fixed sleep loop:** Rejected under `ai/rules/goroutine-lifecycle.md`; waiting on the mutation owner / transaction completion channel wakes immediately and distinguishes active commits from stale candidates.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Write red unit tests for AC-1, AC-3, AC-4, and AC-5, and obtain `ze-test-judge` approval for updating `TestReadLine` (AC-2).
2. Update `cmd/ze/hub/main.go` MCP `ze.mcp.enabled` address resolution and verify per-listener `RegisterServiceBlock` exclusion.
3. Remove `\r` stripping in `internal/component/managed/client.go` `readLine` and update `client_test.go`.
4. Extend `cmd/ze/hub/main_reload.go` `handleSIGHUPReload` / `awaitConfigTransaction` for active `storage.ErrCandidateExists` queueing and race-free wakeup.
5. Gate `storagecli.InstallDataRPC` `ServesClient` readiness in `cmd/ze/hub/main.go` until `startManagedServer` binds.
6. Update docs, verify scoped tests pass under `-race`, and mark all five owned journal rows `fixed:` in `plan/journal/`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `enabled-gate-discards-settings` | 2026-08-10 | MCP `ze.mcp.enabled` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-22 | fleet hub auth line reader |
| `queued-request-waits-for-an-unrelated-trigger` | 2026-09-26 | `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` |
| `silent-fall-through` | 2026-10-19 | `handleSIGHUPReload` |
| `session-up-before-its-subscriber` | 2026-11-19 | hub start publishes the data RPC target before `ServesClient`: a declared client's config import, restore or SSH session is refused as "serves no managed client" until the managed server runs |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Children 1 through 3 and 5 through 7.

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] `ze-test-judge` approval recorded for updating `TestReadLine`
- [ ] All five owned journal rows marked `fixed:` in `plan/journal/`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label hub-runtime-fixes command go test -race ./cmd/ze/hub/... ./internal/component/managed/...`, `./le test functional reload`
