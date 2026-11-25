# Spec: fleet-5 -- Staged Rollout

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | spec-fleet-2-config-templates, plan/release/features/spec-fleet-4-inventory-health.md |
| Phase | 5/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/plugin/server/managed.go` -- ManagedConfigService (config-changed dispatch)
4. `spec-fleet-0-umbrella.md` -- umbrella design decisions
5. `plan/release/features/spec-fleet-1-device-registry.md` -- device registry
6. `spec-fleet-2-config-templates.md` -- config templates and groups (dependency)
7. `plan/release/features/spec-fleet-4-inventory-health.md` -- health reporting (dependency)

## Task

Add staged rollout capability: push config changes to a percentage of devices in a group,
monitor ACK results, and automatically pause if the failure rate exceeds a threshold. The
operator can then inspect, resume, or abort the rollout.

Currently config-changed notifications go to all connected devices simultaneously. For a
group of 100 devices, a bad config breaks all 100 at once. Staged rollout limits blast
radius by notifying devices in batches.

This spec depends on config templates (fleet-2, for group targeting) and health reporting
(fleet-4, for rollout health gates). The rollout also supplies the dispatcher-backed `RolloutPart` defined in fleet-ui-2. Fleet-ui-6 renders its results and controls. The client protocol is unchanged; the hub controls
which devices get notified and when.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Hub-side orchestration | Client protocol unchanged. Hub controls notification order and timing |
| State machine | Persist pending, rolling, paused, complete, failed and aborted in ZeFS. Aborted identifies an operator stopping a rollout, as the fleet page's `RolloutStateAborted` does. |
| Failure threshold | Configurable (default 10%). If N% of a batch reject config, rollout pauses |
| Batch size | Percentage-based (e.g., 10% of group). Minimum 1 device per batch |
| Health gate | Optional: require all devices in batch to report healthy before proceeding to next batch |
| Manual control | Operator can pause, resume, abort, or force-complete a rollout |
| Connected-only targeting | A rollout targets connected members because each batch needs observed results. The hub may still author configuration for declared clients while they are offline. They fetch their desired configuration on reconnect, subject to the local-edit hold. See the 2026-11-19 ruling in `spec-fleet-0-umbrella.md`. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- config-changed notification flow
  -> Constraint: rollout controls when config-changed is sent, not how
- [ ] `internal/component/plugin/server/managed_serve.go` -- NotifyConfigChanged schedules a push to the named client
  -> Decision: release each batch to the existing notification worker, which calls BuildConfigChanged to construct the message.

**Key insights:**
- The notification worker reads the client's current configuration when it builds the notice. Batch admission therefore governs which version a device can fetch as well as when it gets notified.
- Each acknowledgement names the offered version when the device can identify it. A held local edit returns `ok=false` and `holding=true`.
- Rollout state must survive hub restart (persist in ZeFS)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- BuildConfigChanged targets one device; no batching or sequencing
  -> Constraint: rollout wraps existing BuildConfigChanged, does not replace it
- [ ] `pkg/fleet/envelope.go` -- config-ack has OK bool and Error string
  -> Decision: rollout tracks per-device ack results for pass/fail counting
- [ ] The device registry from fleet-1 (group queries): not in the tree on 2026-11-23. `plan/release/features/spec-fleet-1-device-registry.md` creates it (its Files to Create); this bullet names its file once it lands
  -> Decision: use fleet-1 for live connection state and fleet-2 for group membership.
  -> Note (2026-11-23): `fleet-1` keeps labels only; `group` is answered by `fleet-2` (`fleet-1`, the row "`group` declared and filtered by `fleet-1`")
- [ ] Template rendering from fleet-2: not in the tree on 2026-11-23. `plan/post-release/features/spec-fleet-2-config-templates.md` creates it (its Files to Create); this bullet names its file once it lands
  -> Decision: rollout triggers template-based config-changed for group

**Behavior to preserve:**
- Direct config-changed (non-rollout) still works for immediate pushes
- config-ack handling unchanged
- Client protocol unchanged

**Behavior to change:**
- New rollout orchestrator intercepts group-wide config-changed
- Operator initiates rollout via CLI: `fleet rollout start --group <g> --batch-percent <n> --failure-threshold <n>`
- Rollout state machine tracks progress, pauses on failure
- `show fleet rollout` shows active/recent rollouts with per-device status

## Existing Mechanisms

| Mechanism | Source | Use in this design |
|-----------|--------|--------------------|
| Config fetch and notification | `internal/component/plugin/server/managed.go`, `HandleConfigFetch` and `BuildConfigChanged` | Both ask the same reader for a device's configuration. Fleet-5 selects the admitted version for both, including after reconnect. |
| Per-device write notification | `cmd/ze/hub/managed_server.go`, `startManagedServer`; `internal/component/plugin/server/managed_serve.go`, `NotifyConfigChanged` | Use the existing nonblocking notification and worker. Regular writes retain immediate delivery, and staged writes release a batch at a time. |
| Result of an offer | `pkg/fleet/envelope.go`, `ConfigAck`; `internal/component/managed/client.go`, `holdFetched` | Count each authenticated target's result for the offered version and keep the distinction between a refusal and a local edit. |
| Public rollout states | `internal/component/web/fleetui/source_config.go`, `RolloutState` | The durable command result distinguishes an operator abort from a failure. The existing web page can show the same states. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The backend owns the command and structured result across command surfaces. The web UI consumes its result with the same user rights. |
| `docs/architecture/fleet-config.md` | Config Storage (Hub Side) | the // Design: page of internal/component/plugin/server/managed.go | "The hub may write the configuration of a declared client at any time, and a client that is away fetches it when it connects." | agrees | A declared offline client retains its desired configuration. Delivery and acknowledgement each require device contact. |
| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to internal/component/plugin/server/managed.go | "Caller identity is injected only by trusted transport wiring." | agrees | Commands use the authenticated dispatcher identity for authorization. Group and device names identify the configuration the operator requested. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/server/managed.go | "The SDK stays generic." | agrees | This feature extends the hub-side managed service. Its commands and state need no feature-specific SDK callback. |
| `docs/architecture/api/architecture.md` | Command parameter values | anchored to internal/component/plugin/server/managed_serve.go | "The shared API engine resolves structured parameters against one current command declaration before authorization and dispatch." | agrees | Group, percentage and threshold are data declared by the rollout command. The usual transport adapters carry them to the same dispatcher. |
| `docs/architecture/core-design.md` | Command engine and composition roots | anchored to internal/component/plugin/server/managed_serve.go | "Product registrations enter through the normal `cmd/ze` build." | agrees | Register rollout commands in the product. The development test harness drives the same command handlers. |
| `docs/architecture/hub-architecture.md` | What We're Building | anchored to internal/component/plugin/server/managed_serve.go | "Each plugin defines its own configuration schema using YANG" | agrees | Configuration settings use the owning schema and the ordinary configuration transaction. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering cmd/ze/hub/managed_server.go | "SIGHUP and a managed push both reach `doReload`" | agrees | Devices apply a managed change through the existing reload route. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-staged-rollout.ci | "Functional `.ci` files live under `test/<suite>/`" | agrees | Drive managed devices through test/managed and the functional test harness. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of internal/component/plugin/server/managed.go | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | The managed listener keeps its configured certificate and reload semantics. The feature uses its already authenticated sessions. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The command names the template, group or rollout subject. Display filtering and formatting use pipes. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The listing answers structured data and inherits the generic data and formatting operations. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | A pipe transforms the returned data. Starting, resuming or aborting work requires its own authorized action. |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | The rollout delivers a device configuration through the normal compiler. The config display retains its existing inline explanations of implied values. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | An admitted configuration still undergoes device validation. A device refusal is a rollout result, so the orchestrator cannot override backend compatibility. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | An admitted configuration still undergoes device validation. A device refusal is a rollout result, so the orchestrator cannot override backend compatibility. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | The device firewall verifier still owns this decision. The hub records the resulting acknowledgement and uses it when evaluating the batch. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | A target version identifies the unmodified device configuration, including its backend selectors. The rollout never translates them. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP compiles and executes the policy on the device. The rollout controls which approved configuration version is offered and never rewrites next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP compiles and executes the policy on the device. The rollout controls which approved configuration version is offered and never rewrites next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | BGP compiles and executes the policy on the device. The rollout controls which approved configuration version is offered and never rewrites next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP compiles and executes the policy on the device. The rollout controls which approved configuration version is offered and never rewrites next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP compiles and executes the policy on the device. The rollout controls which approved configuration version is offered and never rewrites next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP compiles and executes the policy on the device. The rollout controls which approved configuration version is offered and never rewrites next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The device compiler supplies implied filter members. A rollout neither synthesizes those members nor rewrites the device command that displays them. |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Managed listener | "A configured name that does not resolve returns an error and no material." | agrees | Batch admission selects the served version after authentication. An unresolved certificate reference still fails at the listener. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Managed listener | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | The state of a rollout does not change the listener protocols configured for the looking glass. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Managed listener | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The managed listener uses `managedCertificate` at both startup and reload because its unnamed leaf comes from the daemon authority." | agrees | The managed server retains its certificate provider at initial bind and reload throughout a rollout. |

## Data Flow (MANDATORY)

### Entry Point
- Operator runs `fleet rollout start --group region-west --batch-percent 20 --failure-threshold 10`
- Template change with rollout policy triggers automatic rollout

### Transformation Path
1. Operator initiates rollout for group
2. Resolve the group's members through fleet-2 and intersect with the live connections reported by fleet-1. Show the skipped offline count.
3. Partition the connected members into batches and persist each admitted device's target version. A member waiting for a later batch continues to fetch its previously admitted configuration, including after reconnect.
4. Release batch 1's target versions to the fetch path and schedule each client's existing change notification.
5. Count each authenticated device's acknowledgement of its admitted version once. A stale or duplicate acknowledgement cannot advance a batch. Preserve a holding client's refusal reason and local edits.
6. Count pass/fail. If failure rate > threshold: transition to paused
7. If batch passes: proceed to next batch
8. After all batches: transition to complete

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI to Rollout | `fleet rollout start` command | [ ] |
| Rollout to managed server | Release the admitted version to fetch and enqueue the existing change notice for that device | [ ] |
| config-ack to Rollout | Ack results tracked per device | [ ] |
| Rollout to ZeFS | State persisted for crash recovery | [ ] |

### Integration Points
- `ManagedServer.NotifyConfigChanged` schedules a device's push. Its worker constructs the message using `ManagedConfigService.BuildConfigChanged`.
- The fleet-1 registry supplies connectivity. Fleet-2 supplies group membership.
- Health reports from fleet-4 -- optional health gate between batches
- CLI -- `fleet rollout start/pause/resume/abort/show`
- Web -- rollout progress on fleet dashboard

### Architectural Verification
- [ ] No bypassed layers (rollout uses existing config-changed path)
- [ ] No unintended coupling (non-rollout config-changed unaffected)
- [ ] No duplicated functionality (orchestrates existing primitives)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `fleet rollout start --group region-west` | -> | Rollout state machine creates, sends batch 1 | `test/managed/fleet-staged-rollout.ci` |
| All devices in batch ACK ok | -> | Rollout proceeds to next batch | `test/managed/fleet-staged-rollout.ci` |
| Device in batch rejects config (> threshold) | -> | Rollout pauses automatically | `test/managed/fleet-staged-rollout.ci` |
| `fleet rollout resume` | -> | Rollout continues from paused state | `test/managed/fleet-staged-rollout.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `fleet rollout start --group region-west --batch-percent 20` | Rollout created in pending state, first batch notified |
| AC-2 | All devices in batch 1 send config-ack ok | Rollout proceeds to batch 2 |
| AC-3 | 2 of 5 devices in batch reject config (failure-threshold 10%) | Rollout transitions to paused; no more batches sent |
| AC-4 | `fleet rollout resume` on paused rollout | Rollout retries current batch (or proceeds to next if operator chose to skip) |
| AC-5 | `fleet rollout abort` on an active rollout | The rollout records Aborted and releases no further devices. Its status still reports results of already admitted devices and identifies the operator stop after a hub restart. |
| AC-6 | All batches complete successfully | Rollout transitions to complete |
| AC-7 | `show fleet rollout` | Lists active and recent rollouts: group, progress (batch N/M), state, failure count |
| AC-8 | `show fleet rollout detail <id>` | Detail: per-device status (pending/ack-ok/ack-reject/timeout), batch boundaries |
| AC-9 | Hub restarts during active rollout | Rollout state loaded from ZeFS; resumes from last persisted batch |
| AC-10 | Device in batch does not respond within timeout (default 120s) | Counted as failure; timeout configurable |
| AC-11 | Health gate enabled: the device acknowledged the exact admitted version | Only a health observation collected after that ACK can release the next batch. Retained pre-ACK health and a disconnected target leave it pending. A fresh degraded or down observation pauses the rollout. The health wait accommodates the configured Fleet-4 reporting cadence and distinguishes timeout from an unhealthy report |
| AC-12 | Group has 3 devices, batch-percent 50 | Batch 1: 2 devices, Batch 2: 1 device (minimum 1 per batch) |
| AC-13 | Direct config-changed (no rollout) | Still works immediately for non-rollout pushes |
| AC-14 | Rollout for group with no connected devices | Rollout transitions to paused with "no connected devices" reason |
| AC-15 | A later-batch device fetches while an earlier batch is paused, including after reconnect | The fetch still offers its previously admitted version. It first sees the rollout version when its batch is released. |
| AC-16 | A previous version's acknowledgement arrives or an admitted device acknowledges twice | Only the first matching result counts for that batch member. A device holding local edits reports the hold and keeps its configuration. |

A force-completion test must distinguish all three groups: a device already finished, a device already admitted and a device in an unstarted batch. Check the last device’s served version on reconnect and after hub restart as well as the final rollout report.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRolloutStateMachine` | `internal/component/plugin/server/rollout_test.go` | State transitions: pending -> rolling -> complete | |
| `TestRolloutPauseOnFailure` | `internal/component/plugin/server/rollout_test.go` | Pause when failure exceeds threshold | |
| `TestRolloutResume` | `internal/component/plugin/server/rollout_test.go` | Resume from paused state | |
| `TestRolloutAbort` | `internal/component/plugin/server/rollout_test.go` | An operator abort persists the Aborted state and releases no further batch | |
| `TestRolloutBatchPartition` | `internal/component/plugin/server/rollout_test.go` | Correct batch sizing with minimum 1 | |
| `TestRolloutPersist` | `internal/component/plugin/server/rollout_test.go` | State survives reload from ZeFS | |
| `TestRolloutTimeout` | `internal/component/plugin/server/rollout_test.go` | Device timeout counted as failure | |
| `TestRolloutHealthGate` | `internal/component/plugin/server/rollout_test.go` | Degraded health pauses rollout | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-staged-rollout` | `test/managed/fleet-staged-rollout.ci` | Start rollout, devices ACK, rollout completes; also: device rejects, rollout pauses | |

## Files to Modify
- `internal/component/plugin/server/managed.go` -- read an admitted version for both fetch and change notification
- `internal/component/plugin/server/managed_serve.go` -- receive acknowledgements through the authenticated device session
- `cmd/ze/hub/managed_server.go` -- compose the rollout reader and existing write observer
- `docs/architecture/fleet-config.md` -- batch delivery, reconnect behavior and matching acknowledgements

## Files to Create
- `internal/component/plugin/server/rollout.go` -- Rollout type, state machine, batch orchestration, persistence
- `internal/component/plugin/server/rollout_test.go` -- unit tests
- `test/managed/fleet-staged-rollout.ci` -- functional test

## Design presentation

### What changes

A hub operator can deploy an effective configuration to connected members of a group in batches. The hub persists admitted targets, versions and results. It can pause between batches on failures or health checks and recover its position after restart.

### Before and after

An immediate group update makes the new configuration available to every member. In a rollout of twenty connected devices at twenty percent, the first four may fetch their new configuration. A fifth device that reconnects during a pause still fetches its previous version. An operator abort appears as Aborted, and devices that already applied their configurations keep them. Declared offline clients stay available for ordinary configuration editing.

### Alternatives set aside

Delaying only the push notification would let a later device fetch the new version during a pause. Batching on the hub's fetch path also covers reconnects. Putting the state machine on devices would distribute the decision over machines that cannot observe the other members' acknowledgements.

### Documents departed from

None. Version admission applies to a staged rollout. The hub's existing direct configuration path continues to deliver an ordinary change immediately.

### Choices for Thomas

On 2026-11-25 Thomas selected “Leave unstarted devices on their prior version”. Force complete closes the rollout without waiting for running batches. An unstarted target remains pinned to its prior version even when it reconnects. The rollout reports which devices received the change and which were skipped.

## Implementation Steps

### Implementation Phases

First drive a named rollout through the managed server: pause its first batch, reconnect a later-batch device and assert its fetch still serves the prior version. Release that batch and assert a matching ACK advances once.

1. **Phase: Rollout state machine** -- states, transitions, batch partitioning
   - Tests: `TestRolloutStateMachine`, `TestRolloutBatchPartition`, `TestRolloutPauseOnFailure`, `TestRolloutResume`, `TestRolloutAbort`
   - Files: `rollout.go`, `rollout_test.go`
   - Verify: unit tests pass

2. **Phase: Config-ack tracking** -- per-device ack/reject/timeout tracking
   - Tests: `TestRolloutTimeout`
   - Files: `rollout.go`, `managed.go`
   - Verify: ack results drive state transitions

3. **Phase: Persistence + crash recovery** -- rollout state in ZeFS
   - Tests: `TestRolloutPersist`
   - Files: `rollout.go`
   - Verify: rollout survives hub restart

4. **Phase: Health gate** -- optional health check between batches
   - Tests: `TestRolloutHealthGate`
   - Files: `rollout.go`
   - Verify: degraded health pauses rollout

5. **Phase: Commands and rollout data** -- publish the registered controls and structured results on every command surface and wire fleet-ui-2’s RolloutPart to those results. Fleet-ui-6 owns the pages.
   - Tests: `fleet-staged-rollout.ci`
   - Files: rollout command registrations and the gated RolloutPart provider
   - Verify: rollout lifecycle manageable via CLI

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Failure counting uses percentage, not absolute count; batch minimum is 1 |
| Naming | CLI uses `fleet rollout` prefix; states use lowercase |
| Data flow | Command -> orchestrator -> admitted version and per-device notification -> matching ACK -> state transition |
| Rule: no-partial-completion | Rollout with persistence means crash recovery works |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Auth scope | `fleet rollout start` requires admin authorization |
| Concurrent rollouts | Only one active rollout per group at a time |
| Resource exhaustion | Rollout state bounded (max 100 recent rollouts retained) |

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
