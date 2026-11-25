# Spec: fleet-5 -- Staged Rollout

| Field | Value |
|-------|-------|
| Status | ready |
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
5. `docs/architecture/fleet-config.md`, "Device registry" (`spec-fleet-1-device-registry`, closed 2026-11-25) -- device registry
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
(fleet-4, for rollout health gates). The rollout also supplies the dispatcher-backed `RolloutPart` defined in fleet-ui-2. Fleet-ui-6 renders its results and controls. Fetch, change and ACK retain their public methods. Health-gated rollouts use the optional post-ACK correlation described below. The hub controls
which devices get notified and when.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Hub-side orchestration | Hub controls notification order and timing. The optional post-ACK health proof adds fields to existing replies and reports. |
| State machine | Persist pending, rolling, paused, complete, failed and aborted in ZeFS. Aborted identifies an operator stopping a rollout, as the fleet page's `RolloutStateAborted` does. |
| Failure threshold | Configurable (default 10%). If N% of a batch reject config, rollout pauses |
| Batch size | Percentage-based (e.g., 10% of group). Minimum 1 device per batch |
| Health gate | Optional: require a fresh healthy observation for each admitted target after its successful ACK of the exact named target version. A held, failed, stale or mismatched ACK cannot satisfy the gate. |
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
- [ ] `internal/component/plugin/server/managed_registry.go` — its current authenticated connection determines admission; fleet-2 supplies assigned groups.
  -> Decision: use fleet-1 for live connection state and fleet-2 for group membership.
  -> Note (2026-11-23): `fleet-1` keeps labels only; `group` is answered by `fleet-2` (`fleet-1`, the row "`group` declared and filtered by `fleet-1`")
- [ ] Template rendering from fleet-2: not in the tree on 2026-11-23. `plan/post-release/features/spec-fleet-2-config-templates.md` creates it (its Files to Create); this bullet names its file once it lands
  -> Decision: rollout triggers template-based config-changed for group

**Behavior to preserve:**
- Direct config-changed (non-rollout) still works for immediate pushes
- Ordinary ACKs still update the managed registry and doctor. Gated targets require the additional health confirmation described below.
- Older devices continue to use the same fetch, change and ACK methods.

**Behavior to change:**
- New rollout orchestrator intercepts group-wide config-changed
- Operator initiates rollout via CLI: `request fleet rollout start group <g> target <version-set> batch-percent <n> failure-threshold <n>`
- Rollout state machine tracks progress, pauses on failure
- `show fleet rollout` shows active/recent rollouts with per-device status

## Existing Mechanisms

| Mechanism | Source | Use in this design |
|-----------|--------|--------------------|
| Config fetch and notification | `internal/component/plugin/server/managed.go`, `HandleConfigFetch` and `BuildConfigChanged` | Both ask the same reader for a device's configuration. Fleet-5 selects the admitted version for both, including after reconnect. |
| Per-device write notification | `cmd/ze/hub/managed_server.go`, `startManagedServer`; `internal/component/plugin/server/managed_serve.go`, `NotifyConfigChanged` | Use the existing nonblocking notification and worker. Regular writes retain immediate delivery, and staged writes release a batch at a time. |
| Result of an offer | `pkg/fleet/envelope.go`, `ConfigAck`; `internal/component/managed/client.go`, `holdFetched` | Count each authenticated target's result for the offered version and keep the distinction between a refusal and a local edit. |
| Public rollout states | `internal/component/web/fleetui/source_config.go`, `RolloutState` | The durable command result distinguishes an operator abort from a failure. The existing web page can show the same states. |
| Authenticated targets and result path | `internal/component/plugin/server/managed_registry.go`, `ManagedRegistrySnapshot`; `managed_serve.go`, `handleRequest` and `handleConfigAck` | Freeze connected, authorized target membership at start. Look up the live connection and recorded offered version before counting an ACK. Keep device health, hold, doctor and ordinary ACK bookkeeping operating independently of rollout state. |
| Shared command and optional page registration | `internal/component/managed/yang/ze-managed-cmd.yang` and `internal/component/managed/register.go`; `internal/component/web/fleetui/source_config.go` | Add typed subjects, action leaves and handler registrations at the command owner. Register fleet-ui-2’s RolloutPart once on the gated real build. |
| Durable promotion and target bytes | `pkg/zefs/keys.go` and the managed owner's borrowed statestore | Register bounded rollout headers, target/previous version and result records. Only after the admission record is durable does a fetch or change notification receive that target's new bytes. |
| Post-ACK health | `internal/component/managed/client.go`’s SendConfigAck and fleet-4’s report worker | The managed client currently ignores the ACK result. A capable client reads an optional challenge and collects again. No software release, wall-clock comparison or cached latest-health row substitutes for confirmation of the admitted version. |

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
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Managed listener | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | The managed server retains its certificate provider at initial bind and reload throughout a rollout. |
| `ai/rules/repo-maintenance.md` | Directives | the // Design: page of internal/component/managed/register.go | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | Document the new command subjects and durable records where other operators and developers discover their owners. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to internal/component/managed/register.go | "A check over such state registers in the `runtime` phase, through the same `diagnostic.RegisterDoctorCheck` every other check uses" | agrees | Managed runtime diagnostics continue to see the normal ACK. An unhealthy post-ACK result pauses the rollout and records which version was measured. |
| `docs/architecture/zefs-format.md` | Key Registry | the // Design: page of pkg/zefs/keys.go | "All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`" | agrees | Register bounded raw key families for the manifest, target admissions and results. |
| `docs/architecture/container/image-store.md` | State keys | anchored to pkg/zefs/keys.go | — | silent | The new keys represent reviewed configuration and task state. They do not contain container layers or image records. |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | anchored to pkg/zefs/keys.go | "`List(prefix)` fails when the prefix ends in a slash." | agrees | Resume and eviction enumerate the registered prefix without a trailing slash. |
| `docs/architecture/irr-owner.md` | Who may touch which key | anchored to pkg/zefs/keys.go | — | silent | The managed hub owns the rollout records. Its storage grant does not extend to routing or IRR records. |
| `docs/architecture/storage-backends.md` | Durable publication | anchored to pkg/zefs/keys.go | "Sync failures are returned to the caller." | agrees | A refused durable write never publishes admission or advances a batch. Mark the current operation failed or pending and report the precise durability fault. |
| `ai/doors/core.md` | Boundaries | the door covering pkg/zefs/keys.go | "Runtime state goes through the daemon's one store." | agrees | The managed hub borrows statestore; the core does not depend on fleet. |
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | the // Design: page of internal/component/web/fleetui/source_config.go | "A source answers facts only" | agrees | Publish typed Group, Rollout and Batch facts through the operational adapter. The UI presents the same authorized lifecycle state and skip reasons as the CLI. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering internal/component/web/fleetui/source_config.go | "Never add a command in an adapter." | agrees | The managed owner registers each read and controlled mutation. The web adapter calls the same typed subjects and checks the acting principal. |
| `docs/architecture/fleet-interface.md` | Design decisions | Decision of 2026-11-25 | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | A rollout publishes authenticated and authorized operational facts. The alarm and UI owners apply their recorded visibility, notification and navigation policies. |
| `docs/architecture/fleet-interface.md` | Design decisions | Decision of 2026-11-25 | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | A rollout publishes authenticated and authorized operational facts. The alarm and UI owners apply their recorded visibility, notification and navigation policies. |
| `docs/architecture/fleet-interface.md` | Design decisions | Decision of 2026-11-25 | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | A rollout publishes authenticated and authorized operational facts. The alarm and UI owners apply their recorded visibility, notification and navigation policies. |
| `docs/architecture/fleet-interface.md` | Design decisions | Decision of 2026-11-25 | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | Persist the prior pin and mark unstarted devices skipped. Their later fetch, including after hub restart, must still return the prior version. |
| `docs/architecture/fleet-interface.md` | Design decisions | Decision of 2026-11-25 | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | A rollout publishes authenticated and authorized operational facts. The alarm and UI owners apply their recorded visibility, notification and navigation policies. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | The owner may create desired configuration offline. That alone cannot admit the client to a connected-only rollout or assert an ACK. |

## Data Flow (MANDATORY)

### Entry Point
- Operator runs `request fleet rollout start group region-west target west-v2 batch-percent 20 failure-threshold 10`
- Template change with rollout policy triggers automatic rollout

### Transformation Path
1. Operator initiates rollout for group
2. Resolve the group's members through fleet-2 and intersect with the live connections reported by fleet-1. Show the skipped offline count.
3. Partition the connected members into batches and persist each admitted device's target version. Reject an unknown or stale named target before dispatch. The target names an immutable manifest mapping every selected device to its desired byte hash and content. If a manifest has no member or the bytes no longer match, report the device and refuse the entire start. A member waiting for a later batch continues to fetch its previously admitted configuration, including after reconnect.
4. Release batch 1's target versions to the fetch path and schedule each client's existing change notification.
5. Count each authenticated device's acknowledgement of its admitted version once. A stale or duplicate acknowledgement cannot advance a batch. Preserve a holding client's refusal reason and local edits.
6. Persist the target version, originating session and receipt sequence of the successful ACK. When health gating is enabled, wait for a fleet-4 health observation known to have been collected after that ACK and for the same admitted version. The hub returns a fresh, session-bound challenge in the successful ACK reply for a gated target. The device starts a new health collection after reading that reply. The next accepted health-report includes the optional challenge and applied version. A queued measurement cannot retroactively acquire the challenge; receipt later on the hub by itself cannot distinguish an older queued observation. An older client ignores the optional ACK result and keeps serving its ordinary health data. Report that this target supplied no post-ACK confirmation; it cannot pass before the health deadline. Non-gated rollouts send the ordinary OK reply. A superseded session, reconnect, old healthy record, unknown health or unrelated version cannot release a batch. Re-establish current version and fresh evidence on reconnect or after a hub restart without dispatching the previous batch again.
7. Count failures and pause above the threshold. A fresh down or degraded observation pauses a health-gated batch immediately. Report health timeout separately from configuration ACK timeout. Allow at least the device’s configured reporting interval plus transport allowance for health. Release the next batch only after each required new healthy observation for its acknowledged target version.
8. After all batches: transition to complete

### Command and version contracts

Register the read-only list `show fleet rollout` and one document `show fleet rollout id <id>`. Declare the typed action `request fleet rollout start group <group> target <version-set>` with optional bounded integer leaves `batch-percent` and `failure-threshold` and the configured health gate. The selected target is a hub-side immutable version set, not an unqualified "latest" pointer. The typed preparation action `request fleet rollout prepare group <group>` renders the staged candidate for each selected device, records those bounded bytes beside its prior version, and returns their manifest digest for review. Preparing publishes no delivery or fetch permission. If another writer has already published the candidate outside rollout control it cannot be retroactively staged; refuse with the affected name. A managed template commit subject to rollout policy uses the same staging path before its general change observer could notify a client. The examples use west-v2 as the displayed label; resolving it at start must return and pin the reviewed immutable digest. A stale/changed reviewed target is refused. Validate group membership and authorization on the hub before persisting the rollout.

Pause, resume, abort and force-complete are typed actions under `request fleet rollout`, each with an `id` leaf. Resume rechecks state and dispatches only eligible outstanding work. Keep conditional operator choices explicit if a batch was refused. Each mutation returns the rollout ID, new state and actionable refusal for an invalid transition. Force complete identifies unstarted devices separately, leaves their prior version served on reconnect and keeps already admitted results visible. The list and detail answers return state, progress, prior and target versions, target outcomes, and skipped reasons. Register typed response shapes, parameters and completion once for CLI, SSH, web command line, MCP, REST and gRPC.

Fleet-ui-2’s `RolloutPart` maps `Rollouts(ctx)` to the list and `Rollout(ctx,name)` to one detail, preserving the published bounds of 128 rollouts and 100 batches per rollout. The sample keeps the same contract. The post-release fleet-ui-6 page consumes the operational adapter and sends its action through the same authorized command. Group, member and result visibility scope the returned counts, IDs and names.

### Changed-wire census

Config-fetch, config-changed and config-ack retain their current method names and applied/holding/refused semantics. The config-ack response can optionally carry a health challenge. The fleet-4 report optionally echoes that challenge and the applied version, under the existing bounds. Older senders, receivers and clients that never use the gate continue to exchange the existing frames. The rollout worker compares the active connection, admitted target hash, challenge and accepted post-ACK collection before advancing; it cannot learn this fact from a latest-sample display. A restarted worker obtains fresh evidence for unsettled targets. Exercise old/new mixed peers, stale-generation ACKs, duplicated reports, restart and a health result queued before the ACK.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI to Rollout | `request fleet rollout start` command | [ ] |
| Rollout to managed server | Release the admitted version to fetch and enqueue the existing change notice for that device | [ ] |
| config-ack to Rollout | Ack results tracked per device | [ ] |
| Rollout to ZeFS | State persisted for crash recovery | [ ] |

### Integration Points
- `ManagedServer.NotifyConfigChanged` schedules a device's push. Its worker constructs the message using `ManagedConfigService.BuildConfigChanged`.
- The fleet-1 registry supplies connectivity. Fleet-2 supplies group membership.
- Health reports from fleet-4 -- optional health gate between batches
- CLI -- `request fleet rollout start/pause/resume/abort/force-complete` and the two registered show subjects
- Web -- rollout progress on fleet dashboard

### Architectural Verification
- [ ] No bypassed layers (rollout uses existing config-changed path)
- [ ] No unintended coupling (non-rollout config-changed unaffected)
- [ ] No duplicated functionality (orchestrates existing primitives)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `request fleet rollout start group region-west target west-v2` | -> | Rollout state machine creates, sends batch 1 | `test/managed/fleet-staged-rollout.ci` |
| All devices in batch ACK ok | -> | Rollout proceeds to next batch | `test/managed/fleet-staged-rollout.ci` |
| Device in batch rejects config (> threshold) | -> | Rollout pauses automatically | `test/managed/fleet-staged-rollout.ci` |
| `request fleet rollout resume id <id>` | -> | Rollout continues from paused state | `test/managed/fleet-staged-rollout.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `request fleet rollout start group region-west target west-v2 batch-percent 20` | Rollout created in pending state, first batch notified |
| AC-2 | All devices in batch 1 send config-ack ok | Rollout proceeds to batch 2 |
| AC-3 | 2 of 5 devices in batch reject config (failure-threshold 10%) | Rollout transitions to paused; no more batches sent |
| AC-4 | `request fleet rollout resume id <id>` on paused rollout | Rollout retries current batch (or proceeds to next if operator chose to skip) |
| AC-5 | `request fleet rollout abort id <id>` on an active rollout | The rollout records Aborted and releases no further devices. Its status still reports results of already admitted devices and identifies the operator stop after a hub restart. |
| AC-6 | All batches complete successfully | Rollout transitions to complete |
| AC-7 | `show fleet rollout` | Lists active and recent rollouts: group, progress (batch N/M), state, failure count |
| AC-8 | `show fleet rollout id <id>` | Detail: per-device status (pending/ack-ok/ack-reject/timeout), batch boundaries |
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
| `TestRolloutHealthGate` | `internal/component/plugin/server/rollout_test.go` | Pre-ACK or delayed old health, wrong target version, disconnect and an old session stay pending. Matching fresh healthy passes and fresh down or degraded pauses. | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-staged-rollout` | `test/managed/fleet-staged-rollout.ci` | Start rollout, devices ACK, rollout completes; also: a device rejects and the rollout pauses; a pre-ACK healthy report cannot release a health-gated batch until a matching fresh reading arrives | |

## Files to Modify
- `internal/component/managed/yang/ze-managed-cmd.yang` and `internal/component/managed/register.go` — typed reads, group, version and ID selectors, actions, completions and structured result shapes.
- `internal/component/managed/client.go`, `pkg/fleet/envelope.go` and fleet-4’s planned `pkg/fleet/report.go` — optional ACK response and post-ACK health confirmation.
- `pkg/zefs/keys.go` — register immutable version-set records, bounded rollout and per-device admission/result records.
- `internal/component/web/fleetui/source_config.go` — document the stable RolloutPart contracts and durable target states.
- `internal/component/plugin/server/managed.go` -- read an admitted version for both fetch and change notification
- `internal/component/plugin/server/managed_serve.go` -- receive acknowledgements through the authenticated device session
- `cmd/ze/hub/managed_server.go` -- compose the rollout reader and existing write observer
- `docs/architecture/fleet-config.md` -- batch delivery, reconnect behavior and matching acknowledgements

## Files to Create
- `internal/component/managed/cmd_rollout.go` and `internal/component/managed/cmd_rollout_test.go` — register and serve named target review, start, status and lifecycle commands.
- `internal/component/web/fleetui/rollout/register.go` and `internal/component/web/fleetui/rollout/source.go` — operational RolloutPart adapter over the dispatcher.
- `cmd/ze/hub/register_fleet_ui_rollout.go` — link the operational part on a real fleet web build, excluding sample builds.
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
   - Verify: a fresh unhealthy observation pauses; named-version ACK plus fresh healthy evidence passes, and a delayed pre-ACK report never advances the batch.

5. **Phase: Commands and rollout data** -- publish the registered controls and structured results on every command surface and wire fleet-ui-2’s RolloutPart to those results. Fleet-ui-6 owns the pages.
   - Tests: `fleet-staged-rollout.ci`
   - Files: rollout command registrations and the gated RolloutPart provider
   - Verify: rollout lifecycle manageable via CLI

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Failure counting uses percentage, not absolute count; batch minimum is 1 |
| Naming | Read commands use `show fleet rollout` and actions use `request fleet rollout`; states use lowercase |
| Data flow | Command -> orchestrator -> admitted version and per-device notification -> matching ACK -> state transition |
| Rule: no-partial-completion | Rollout with persistence means crash recovery works |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Auth scope | `request fleet rollout start` requires admin authorization |
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
- [ ] Scoped managed, command and web tests pass in the main checkout.
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
