# Spec: fleet-2 -- Config Templates and Group Assignment

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | plan/release/features/spec-fleet-1-device-registry.md, plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md (operational adapters) |
| Phase | 2/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- existing fleet config architecture
4. `internal/component/plugin/server/managed.go` -- ManagedConfigService.HandleConfigFetch
5. `spec-fleet-0-umbrella.md` -- umbrella design decisions
6. `plan/release/features/spec-fleet-1-device-registry.md` -- device registry (dependency)

## Task

Add config templates with variable substitution and group-based assignment. Instead of
maintaining N separate config blobs (one per managed device), operators define named
templates with variables (e.g., `{{.DeviceName}}`, `{{.Labels.site}}`). Groups map to
templates. When a device in a group does a config-fetch, the hub renders the template
with that device's variables and returns the result.

Currently each client's config is a separate opaque blob in the hub's ZeFS. Managing
100 devices means maintaining 100 individual files. This spec eliminates that duplication. It also owns the dispatcher-backed `AssignmentPart`, `DeviceConfigPart` and `TemplatePart` defined in fleet-ui-2: the UI reads the same effective configuration bytes and template usage that the hub serves.

The client protocol is unchanged. The client receives a fully rendered config, never
sees templates. A shared hub-side effective-config resolver renders the bytes for fetch, change notification, registry version, commands and divergence handling.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Templates rendered hub-side | Client protocol unchanged; client receives final config |
| Template syntax | Go `text/template` with restricted function set (no shell, no file access) |
| Variable sources | Device name, device labels, group name. No external data sources |
| Assignment model | Group -> template mapping in YANG config. A device's group determines its template |
| Per-device override | Device can still have a direct config blob; overrides template assignment |
| Change propagation | A hub commit changes the desired configuration of declared clients that use the template, including offline clients. The hub notifies connected clients whose effective configuration changed. An offline client fetches it on connect. |
| Protection of a local edit | A device that edited its own configuration since the last hub baseline holds that edit on reconnect. The operator can inspect and resolve the difference through the existing adopt or revert commands. A clean device can apply its newly rendered configuration on the first fetch. See the 2026-11-19 ruling in `spec-fleet-0-umbrella.md`. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- config fetch protocol, version hashing
  -> Constraint: version hash must reflect rendered output, not template source
- [ ] `internal/component/plugin/server/managed.go` -- HandleConfigFetch reads config via ConfigReader
  -> Decision: use the direct configuration when found. The storage reader in `startManagedServer` returns a wrapped `fs.ErrNotExist` for an absent blob. Normalize that miss to the `ConfigReader` contract (`ErrClientConfigNotFound`) before template lookup. Other read errors reach the caller.

**Key insights:**
- ConfigReader is a `func(name string) ([]byte, error)`; template rendering can be injected here
- Version hash computed from rendered bytes ensures client sees "current" only when rendered output matches
- Template change notification requires iterating devices in the affected group

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- HandleConfigFetch calls `s.readConfig(clientName)` then hashes
  -> Constraint: readConfig interface must accommodate template rendering transparently
- [ ] `pkg/fleet/envelope.go` -- ConfigFetchResponse with Version, Config (base64), Status
  -> Constraint: response format unchanged
- [ ] `internal/component/plugin/server/managed_registry.go` -- fleet-1's `ManagedRegistrySnapshot` lists declared clients and their labels. Fleet-2 owns their group assignments.

**Behavior to preserve:**
- Devices with direct config blobs (no template) work exactly as today
- Config-fetch response format unchanged
- Version hash semantics unchanged (hash of what client receives)
- Client protocol unchanged

**Behavior to change:**
- Fetch and change notification read the same effective configuration. A direct blob takes precedence even for a group member.
- When the direct blob is absent, render the group's assigned template with this device's variables.
- Template or group config change triggers config-changed for affected devices
- YANG schema extended with template and group-to-template mapping

## Existing Mechanisms

| Mechanism | Source | Use in this design |
|-----------|--------|--------------------|
| Desired configuration reader | `internal/component/plugin/server/managed.go`, `ConfigReader`, `HandleConfigFetch`, `BuildConfigChanged` | Both managed messages hash the bytes supplied by the same reader. Extend that reader to resolve a group template only when the direct configuration is absent. |
| Change delivery | `internal/component/plugin/server/managed_serve.go`, `NotifyConfigChanged` | Schedule an existing push for each connected client whose effective configuration changed. Reconnection already fetches the current desired bytes. |
| Declared clients and labels | `internal/component/plugin/server/managed_registry.go`, `ManagedRegistrySnapshot` | Read the client's labels from the registry and resolve group and template through fleet-2's configuration. |
| Local edit protection | `internal/component/managed/client.go`, `holdFetched` | The device keeps a local change when offered a rendered configuration just as it does for a direct blob. |
| Model extraction and promotion | `internal/component/config/loader_extract.go`, `ExtractHubConfig` and `extractHubServerConfig`; `cmd/ze/hub/managed_server.go`, `prepareManagedReload`; `internal/component/plugin/server/managed_apply.go`, `PrepareManagedListeners` and `ApplyManagedListeners` | Prepare a single immutable catalogue from the promoted hub configuration for startup, commit and reload. A rejected candidate sends no notification and leaves the previous catalogue active. |
| Divergence inspection, revert and adopt | `internal/component/managed/cmd_divergence_show.go`, `readServedConfig` and `readDivergenceDetail`; `internal/component/managed/cmd_divergence.go`, `revertHeldLocked` and `adoptHeldLocked` | Compare the held blob with the resolved effective bytes. A revert pins the checked version. Adopt writes a direct override only for that device. Keep candidate, freeze, held-hash and authorization checks. |
| Cached desired version | `internal/component/plugin/server/managed_registry_versions.go`, `MarkManagedConfigChanged` and `currentConfigVersion` | A new catalogue or direct blob can change the effective bytes. Invalidate affected client generations before exposing the new catalogue to registry readers. |
| Command declaration and part registrations | `internal/component/managed/register.go` and `yang/ze-managed-cmd.yang`; fleet-ui-2’s AssignmentPart, DeviceConfigPart and TemplatePart | Declare typed reads with the managed command owner. The sample currently exposes the monolithic Source, not these three slots. Fleet-ui-2 is a prerequisite for the planned registration hooks. After its split, link this adapter to those hooks. Include the dependency and its compiled-out behavior in the wiring test. |

| Scoped argument completion | `internal/component/plugin/server/arg_completion.go`, ArgCompleter; `internal/component/plugin/server/system.go`, handleArgComplete; `internal/component/plugin/server/command.go`, RPCRegistration | Existing dynamic completers receive only the server and completed tokens. Extend the registration with an optional caller-aware completer alongside the compatible legacy signature. The server invokes that producer with the trusted CommandContext and validated arguments; never put identity into caller-supplied words. |
| Target permission for raw bytes | `internal/component/cli/config_permission.go`, ConfigPermission; `cmd/ze/hub/session_factory.go`, namedConfig; `internal/component/plugin/server/handler.go`, RPCRegistration.NotReadOnly | The editor gates config set on the named file, while the bare word edit is ungated. Use the same registered permission string and the edit-profile check for the raw show command. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The backend owns the command and structured result across command surfaces. Transport-independent authorization protects the data. The raw config command requires both its NotReadOnly dispatch authorization and the named config-set check. The part relays a refusal without configuration bytes when either check fails. |
| `docs/architecture/fleet-config.md` | Config Storage (Hub Side) | the // Design: page of internal/component/plugin/server/managed.go | "The hub may write the configuration of a declared client at any time, and a client that is away fetches it when it connects." | agrees | A declared offline client retains its desired configuration. Delivery and acknowledgement each require device contact. |
| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to internal/component/plugin/server/managed.go | "Caller identity is injected only by trusted transport wiring." | agrees | Commands use the authenticated dispatcher identity for authorization. Group and device names identify the configuration the operator requested. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/server/managed.go | "The SDK stays generic." | agrees | This feature extends the hub-side managed service. Its commands and state need no feature-specific SDK callback. |
| `docs/architecture/hub-architecture.md` | What We're Building | anchored to cmd/ze/hub/managed_server.go | "Each plugin defines its own configuration schema using YANG" | agrees | Configuration settings use the owning schema and the ordinary configuration transaction. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering cmd/ze/hub/managed_server.go | "SIGHUP and a managed push both reach `doReload`" | agrees | Stage the catalogue through the hub’s existing reload route; devices apply the selected bytes through their existing managed reload route. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | Declare named templates and groups as keyed lists. No new configuration parser is needed. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-template-fetch.ci | "Functional `.ci` files live under `test/<suite>/`" | agrees | Drive managed devices through test/managed and the functional test harness. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of internal/component/plugin/server/managed.go | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | The managed listener keeps its configured certificate and reload semantics. The feature uses its already authenticated sessions. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The three list subjects name templates, groups and assignments; the two single-device subjects require a typed name. Display filtering and formatting use pipes. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The listing answers structured data and inherits the generic data and formatting operations. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | A pipe transforms the returned data. Changing an assignment requires an authorized configuration commit. |
| `ai/rules/repo-maintenance.md` | Directives | the // Design: page of internal/component/managed/register.go | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | Update the command catalogue and feature discovery alongside the new typed reads. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to internal/component/managed/register.go | "A check over such state registers in the `runtime` phase, through the same `diagnostic.RegisterDoctorCheck` every other check uses" | agrees | Use current managed runtime health for unreadable or failed rendering, without adding another check at command registration. |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | anchored to internal/component/config/loader_extract.go | "A configured name that does not resolve returns an error and no material." | agrees | Prepare template and listener candidates without changing live state. A named missing certificate still fails closed and prevents installation. |
| `ai/doors/config-yang.md` | Traps | the door covering internal/component/config/loader_extract.go | "`ze config validate` never calls a live `OnConfigVerify`." | agrees | Extract and validate the catalogue through the offline-capable schema/semantic path. A live managed server is not needed for validation. |
| `docs/architecture/api/process-protocol.md` | Wire Format | the // Design: page of internal/component/plugin/types.go | "Every message is a single newline-terminated line:" | agrees | The hub sends the same existing config-fetch result and config-changed frame. Rendering does not introduce a new process protocol. |
| `docs/architecture/api/architecture.md` | Command parameter values | anchored to internal/component/plugin/types.go | "REST, gRPC and MCP preserve an explicitly supplied empty string." | agrees | Present the same typed device selector on every transport. Reject an explicit empty name with the same model validation. |
| `docs/architecture/api/ipc_protocol.md` | Message Framing | anchored to internal/component/plugin/types.go | "All plugin RPC messages are UTF-8, newline-delimited lines:" | agrees | The managed connection uses the existing correlated request and response framing. Only the bytes selected as the desired configuration change. |
| `docs/architecture/api/json-format.md` | Field naming | anchored to internal/component/plugin/types.go | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Use kebab-case command fields on the wire and map them explicitly into the UI value types. |
| `docs/architecture/cli/command-verbs.md` | Narrowing and shaping: the pipe layer | anchored to internal/component/plugin/types.go | "An owned filter takes the name of the field it selects on." | agrees | Read verbs name subjects. Generic filters and presentation operate on the authorized returned rows. |
| `docs/architecture/cli/error-surface.md` | The operator error | anchored to internal/component/plugin/types.go | "A refusal an operator reads is an `opererr.Error`" | agrees | Return the managed error with its cause and operator action when either direct storage or rendering fails. Sensitive rendered content stays out of the diagnosis. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to internal/component/plugin/types.go | "The hub promotes a candidate after runtime reload succeeds." | agrees | Prepare and validate the catalogue before promotion. A rejected or rolled-back candidate cannot send effective-config notifications. |
| `docs/architecture/core-design.md` | Command engine and composition roots | anchored to internal/component/plugin/types.go | "no second command engine for repository tooling" | agrees | The command owner registers the new reads. The fleet UI depends only on the command dispatcher and its typed facts. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to internal/component/plugin/types.go | "" | silent | HubConfig shares its Go file with BGP RawMessage definitions. Rendering neither creates nor changes negotiated encoding contexts. |
| `docs/architecture/plugin-manager-wiring.md` | Stage 1 Registration: Config Transaction Fields | anchored to internal/component/plugin/types.go | "" | silent | HubConfig shares the types file with process ownership interfaces. No process lifecycle or registration stage changes. |
| `docs/architecture/web-interface.md` | Admin command visibility | anchored to internal/component/plugin/types.go | "Admin command discovery omits schema-hidden nodes and their descendants." | agrees | The new adapter reads through the dispatcher under the authenticated principal. A restricted user cannot discover disallowed configuration bytes. |
| `docs/architecture/aaa-tacacs.md` | Accounting hangs off one dispatch point | anchored to cmd/ze/hub/main_reload.go | — | silent | The template catalogue is a separate prepared configuration consumer. It does not change authentication fallback or reload rejection for a failed backend. |
| `docs/architecture/behavior/signals.md` | Ze Signal Mapping | anchored to cmd/ze/hub/main_reload.go | "`handleSIGHUPReload` (`cmd/ze/hub/main_reload.go`), then `reactor.SignalHandler.OnReload`" | agrees | SIGHUP joins the existing reload path. Template changes take effect and publish notices only after that transaction succeeds. |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to cmd/ze/hub/main_reload.go | — | silent | The existing hub coordinates the reload. Neither the web adapter nor the template renderer takes over manager lifecycle or RPC transport. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering internal/component/web/fleetui/templates/register.go | "Never add a command in an adapter." | agrees | Register with the managed owner. Each optional web part consumes the existing reply through the dispatcher, and no surface bypasses authorization. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Prepare catalogue and PKI candidates in the same transaction. The managed/template change preserves the existing socket and certificate validation policy for every kind of listener. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | Prepare catalogue and PKI candidates in the same transaction. The managed/template change preserves the existing socket and certificate validation policy for every kind of listener. |
| `docs/architecture/api/architecture.md` | Design decisions | Decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Rendering supplies desired bytes, without changing the generic implied-filter presentation or where annotations appear. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The renderer produces configuration text and the existing schema and dataplane validators still own its semantics. It does not synthesize or interpret a BGP, backend or implied-filter decision. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | The assignment and configuration adapters return their own typed facts. The activity, rollout and device page owners retain their recorded behavior. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | The assignment and configuration adapters return their own typed facts. The activity, rollout and device page owners retain their recorded behavior. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The assignment and configuration adapters return their own typed facts. The activity, rollout and device page owners retain their recorded behavior. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | The assignment and configuration adapters return their own typed facts. The activity, rollout and device page owners retain their recorded behavior. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | The assignment and configuration adapters return their own typed facts. The activity, rollout and device page owners retain their recorded behavior. |
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | Published UI adapter contract | "A source answers facts only" | agrees | The three typed parts use dispatcher-backed facts and bounded responses with the caller’s identity. |
| `ai/doors/commands-cli.md` | Boundaries | The new YANG command nodes | "A handler answers data, never text." | agrees | Register tabular facts and device documents once, using the command schema for the typed name selector. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | The proposed web first commit creates a direct override. The template stays shared and selection continues to favor the committed direct bytes. |

| `docs/architecture/cli/command-completion.md` | The producers | The registered argument producer | "Every producer asks one predicate what its surface owes a command" | agrees | Command discovery uses the existing surface rules. The optional name completer receives the trusted caller and checks target visibility for each candidate. |
| `docs/architecture/command-ownership.md` | What the Removal Test Forbids | Generic argument completion | "A plugin's command spelling in generic dispatch" | agrees | The shared dispatcher recognizes a registered caller-aware producer. The managed package owns fleet path names and its own authorization filter. |
| `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` | Decisions | The handler redaction boundary | "A command argument the validator refuses is quoted back" | agrees | The selector holds a declared device name only. Keep the existing credential masking in the shared dispatcher and never suggest or log raw configuration or credentials. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | The managed test suite | "All lines use key=value format" | agrees | Exercise the real CLI commands in managed-suite fixtures and retain the existing functional syntax. |

## Data Flow (MANDATORY)

### Entry Point
- Operator configures `bus { hub { template edge-default { source "...config with {{.DeviceName}}..."; } } }`
- Operator configures `bus { hub { group region-west { template edge-default; } } }`
- Device in group region-west sends config-fetch

### Transformation Path
1. Device sends config-fetch with current version hash
2. Read the device's direct configuration. On success use those bytes. On a storage failure return the error.
3. When the direct configuration is absent, look up the device's group and assigned template. If neither source provides a configuration, return the existing missing-configuration error.
4. Render the template with the device name, group and labels from the same promoted snapshot. A missing variable fails this fetch.
5. Pass the selected bytes to the existing version comparison and managed delivery path. Its held-version and discard-version checks also apply to template clients.

### Model and command contracts

The proposed schema adds sibling keyed lists `bus hub template <name>` and `bus hub group <name>`. A template contains a bounded source leaf. A group has an optional template leafref. Add one optional `group` leafref to each existing `bus hub server <listener> client <name>` entry. A device can belong to one group. When two listeners declare the same name, require the same group and labels across both. Existing clients without the leaf preserve their direct configuration. Deleting a referenced template or group requires removing or updating the references in the same candidate. Treat the group as a named configuration assignment, independent of dashboard tree grouping by label.

| Registered command | Typed selectors and result | Part served |
|-------------------|----------------------------|-------------|
| `show fleet templates` | No positional arguments. Tabular name, assigned groups, count of visible devices effectively using the template, last changed and actor. A direct override excludes that client from effective usage. Read attribution from the last committed catalogue change or report it unavailable. | `Templates(ctx)` returns at most 256 typed Template rows. |
| `show fleet groups` | No positional arguments. Tabular group, assigned template and authorized member count. | Group lookup for targeting and rendered usage. |
| `show fleet assignments` and `show fleet assignment name <device>` | List rows or one document by the YANG-declared device-name leaf, with visible device name, assigned group, effective template and source (direct, template or missing). Return the assigned group even when a direct override supersedes its template. | `Assignments(ctx)` returns at most 4,096 devices. `AssignmentOf(ctx,name)` returns the same answer for one. |
| `show fleet device config name <device>` | The name leaf is required, completed from the caller-visible registry and validated as a device name. Return a document with device, version and effective text, up to the existing 1 MiB limit. Mark the daemon registration NotReadOnly, requiring edit authorization because raw text contains secrets. For a configured policy, require both the resolved raw show command in the edit profile and the existing named permission `config set client-<name>.conf` with isReadOnly false before resolving or rendering. A caller lacking either receives the command’s typed authorization refusal, including in DeviceConfigPart. This part supplies no masked alternate body. | `DeviceConfig(ctx,device)` returns those exact bytes and version. |

Use the dispatcher’s trusted username, remote address and live AAA authorizer. Register the raw configuration RPC with RPCRegistration.NotReadOnly = true; apply the equivalent metadata if a local bridge serves it. For the named-file check construct ClientConfigKey from the validated device name, and use the same named `config set <file>` permission syntax that config-mode set uses for a client file. ConfigPermission does not gate the bare word edit. Selecting this existing set permission as the read-raw threshold is a deliberate proposal; it grants no mutation through the show command. For aggregate reads, check each member against its canonical `show fleet device name <device>` invocation with the existing CommandArgsAuthorizer, argument vector and trusted request principal before projecting rows, summing counts or offering completion. Check the same per-device invocation before returning a single-device assignment, and check the raw-config command in the edit profile and the named config-set permission for full bytes. This design uses command-string and argument authorization already provided by authz.Store, never a presumed device ACL on the store itself. Preserve the system’s NoAuth-configured behavior; with an authenticated policy, a denial or unavailable policy evaluation contributes no row or count. Test permit-all, per-name allow/deny, read-only and no-config-write profiles through all command surfaces.

Complete the device-name arguments using the caller-aware producer. Apply the exact list or raw query's target checks before prefix-matching or publishing choices. A missing trusted caller in a configured authz environment returns no protected suggestions. Leave other registrations on their existing callback. Direct dispatcher calls, CLI tab completion and the web command line must neither suggest nor return an unauthorized client name.

Each command terminates at its subject. Presentation and row filtering use the registered pipes. Use the same daemon handlers through CLI, SSH, REST, MCP, gRPC and the web command line. A restricted caller sees only permitted targets and counts. The optional fleet web adapter registers the three parts once under the operational fleet tags. The sample retains its registrations; builds lacking a part keep fleet-ui-2’s unavailable state.

### Reload, resolution and concurrent changes

1. Lower the candidate to typed hub configuration through ExtractHubConfig. Enforce template count, source length (64 KiB maximum), valid restricted syntax, unique names, leafrefs and duplicate-listener agreement in the same validation used by `ze config validate`, startup, reload and commit. Bound the render before generating more than 1 MiB. Permit only literal text and direct lookups of the named scalar fields or label keys. Walk the parsed syntax tree before rendering and reject unbounded actions, loops, invocation, pipelines, nested templates and dynamic expansion. A syntactically valid template referring to a label absent on one client produces that client’s existing fetch refusal under AC-8.
2. Stage the immutable candidate with the managed listener plan before promotion in the hub reload lifecycle. Publish after successful promotion. Compute affected names from previous and promoted templates, group membership and labels. Enlist direct-blob writes, deletions and adopt in the same per-device generation tracking. Failed candidate or unrelated edit cannot release new bytes or send a notice. The service can rebuild the catalogue from the committed configuration after restart without materializing per-device direct blobs.
3. Detect and publish effective changes by comparing resolved old and new bytes or their absence under a stable generation. Publish the new catalogue and invalidate affected registry generations inside the same critical section, including offline devices. Readers capture the generation with their catalogue and may populate the hash cache only if both are still current after reading. A direct error remains an actionable error. Only a genuine absent direct blob falls through. Removing a direct override exposes the assigned template. Coalesce repeated notifications and compute their advertised version through the same reader used by the next fetch.
4. Both daemon and local divergence readers use the same selection over the committed hub catalogue. A store-only command reconstructs the catalogue from running hub configuration and uses its guarded direct and held reads. Serialize revert against the held record, direct bytes and catalogue generation. Compare both operator-supplied versions before recording the decision. If a concurrent template or label edit changes the effective bytes, refuse the stale revert or void the old offer on fetch. Adopt commits the held bytes to `client-<name>.conf` through CommitConfigWithGuard, updates only that device’s effective version and preserves group membership. Never rewrite the shared template. Restore through restart and test direct selection, blocked staged candidates and an unreadable source.

### Changed-wire census

Managed clients continue to send their current version in config-fetch. HandleConfigFetch and BuildConfigChanged both hash effective bytes, preserving the public fetch, ACK, held and change shapes in the fleet envelope. A direct blob with existing bytes therefore retains its version across upgrades. A new hub with an old client needs no peer feature negotiation. The new typed command answers above reach all surfaces. The existing divergence show, revert and adopt retain their CLI operands and version checks; template-only clients now have a served version and a diff. Validate the projections on both a running hub and a store-only command after shutdown.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to Template Engine | YANG config parsed, templates stored | [ ] |
| Template Engine to HandleConfigFetch | ConfigReader replaced with template-aware reader | [ ] |
| Template change to Notification | Iterate group members, send config-changed | [ ] |

### Integration Points
- `ManagedConfigService.HandleConfigFetch` -- intercept with template-aware ConfigReader
- `DeviceRegistry` from fleet-1 supplies label values. Fleet-2 supplies group membership.
- The promoted managed service invokes the existing coalescing NotifyConfigChanged for each connected device with changed effective bytes.
- The bus hub template and group keyed lists and server/client group leaf bind the model to declared clients.

### Architectural Verification
- [ ] No bypassed layers (template rendering inside HandleConfigFetch path)
- [ ] No unintended coupling (devices without templates unaffected)
- [ ] No duplicated functionality (reuses ConfigReader interface)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with template and group assignment | -> | Template stored, group linked | `test/managed/fleet-template-fetch.ci` |
| Device in group sends config-fetch | -> | Template rendered with device vars | `test/managed/fleet-template-fetch.ci` |
| Template source edited | -> | config-changed sent to group members | `test/managed/fleet-template-change.ci` |
| Device with direct blob (no group) | -> | Blob returned as before | `test/managed/fleet-template-fetch.ci` |
| Normal web build with fleet-ui-2's new part registry, sample build, and build without operational template registration | → | The three real adapter registrations, sample alternatives and unavailable fallback | `internal/component/web/fleetui/templates/source_test.go` checks compiled configurations plus the actual callable parts. |
| A partial name matches both a permitted and a denied client | → | handleArgComplete with trusted metadata and the managed caller-aware producer | `internal/component/managed/cmd_templates_test.go` checks the raw/assignment distinction and empty caller context. |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config with `bus { hub { template edge-default { source "..."; } } }` | Template parsed and stored |
| AC-2 | Template with `{{.DeviceName}}` and device "edge-01" fetches config | Config contains literal "edge-01" at substitution point |
| AC-3 | Template with `{{.Labels.site}}` and device label site=london | Config contains "london" at substitution point |
| AC-4 | Group "region-west" with template "edge-default" | Every member without a direct configuration receives its rendered template through config-fetch |
| AC-5 | Device not in any group, has direct config blob | Blob returned as before (no template rendering) |
| AC-6 | Device in group but also has direct config blob | Direct blob takes precedence over template |
| AC-7 | The hub commits a template edit with connected and offline group members, including one with a direct override | Connected members whose effective bytes changed receive `config-changed`. The offline member receives the new version on reconnect, subject to its existing local-edit hold. The override still serves its own bytes and receives no template-change notification. |
| AC-8 | Template with undefined variable (label not set on device) | The hub returns an error on config-fetch identifying the missing variable and records the rendering failure. The client keeps its active configuration. |
| AC-9 | `show fleet templates` CLI | Lists all templates with usage count (how many groups/devices) |
| AC-10 | `show fleet groups` CLI | Lists all groups with member count and assigned template |
| AC-11 | Two devices produce identical rendered bytes, or a template includes a device-specific value | Identical output yields the same version. A device name substitution that changes the bytes changes the version. |
| AC-12 | Two listener entries for the same client disagree on group; a reference names a missing template; or a malformed template is committed | Validate/reload/start refuse consistently, keep the previously promoted versions and emit no notice. |
| AC-13 | An offline member with a held local edit gets a new template, direct override or group assignment | Registry and divergence diff report the new effective version. A stale pinned revert is refused. An authorized adopt writes the one device’s direct override and leaves other members unchanged. Store-only and daemon answers agree. |
| AC-14 | A restricted caller inspects assignments, config, groups and templates through CLI or a fleet page | All command surfaces and three parts answer the same visible facts. Denied devices do not contribute to counts or expose bytes. A direct override changes effective usage while preserving the declared group. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTemplateRender` | `internal/component/plugin/server/template_test.go` | Basic variable substitution | |
| `TestTemplateRenderLabels` | `internal/component/plugin/server/template_test.go` | Label variable substitution | |
| `TestTemplateRenderUndefinedVar` | `internal/component/plugin/server/template_test.go` | Graceful failure on missing variable | |
| `TestTemplateAwareConfigReader` | `internal/component/plugin/server/template_test.go` | ConfigReader returns rendered config for group member | |
| `TestTemplateAwareConfigReaderDirectBlob` | `internal/component/plugin/server/template_test.go` | A direct blob wins with and without a group assignment. A read error propagates even if a template is available. | |
| `TestTemplateChangeNotification` | `internal/component/plugin/server/template_test.go` | Template edit triggers notification for group members | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-template-fetch` | `test/managed/fleet-template-fetch.ci` | Device in group gets rendered config; direct blob device gets blob | |
| `fleet-template-change` | `test/managed/fleet-template-change.ci` | Template edit triggers config-changed, device re-fetches | |

## Files to Modify
- `internal/component/plugin/server/arg_completion.go`, `internal/component/plugin/server/command.go` and `internal/component/plugin/server/system.go` — add and route the optional caller-aware completer, preserving existing callbacks.
- `docs/architecture/cli/command-completion.md` — document how a registered caller-aware producer filters dynamic name suggestions.
- `internal/component/managed/yang/ze-managed-cmd.yang` and `internal/component/managed/register.go` — declare typed selectors, shapes, fields, completion and RPC registrations.
- `internal/component/config/loader_extract.go` and `internal/component/plugin/types.go` — lower template, group and per-client assignments, validating multi-listener declarations.
- `internal/component/plugin/server/managed_apply.go` and `cmd/ze/hub/main_reload.go` — prepare candidate catalogue and install it on successful promotion.
- `internal/component/plugin/server/managed_registry_versions.go` — invalidate cached effective versions on template, label, assignment and direct changes.
- `internal/component/managed/cmd_divergence.go` and `internal/component/managed/cmd_divergence_show.go` — guarded reads, masked diffs and checked revert use effective bytes. Adopt creates a direct override.
- `internal/component/plugin/server/managed.go` -- template-aware ConfigReader
- `cmd/ze/hub/managed_server.go` -- install the effective configuration reader in the managed service
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- template and group YANG containers
- `docs/architecture/fleet-config.md` -- template selection, effective versions and the direct override

## Files to Create
- `internal/component/managed/cmd_templates.go` — handlers for the typed catalogue, assignments and effective text.
- `internal/component/web/fleetui/templates/register.go`, `internal/component/web/fleetui/templates/source.go` and `cmd/ze/hub/register_fleet_ui_templates.go` — implement and link the three operational parts under fleet web tags.
- `internal/component/managed/cmd_templates_test.go` — typed authorization, store-only divergence, checked revert and adopt with an effective template.
- `internal/component/web/fleetui/templates/source_test.go` — dispatch the correct command per part, check bounds, scope and unavailable states.
- `internal/component/plugin/server/template.go` -- template storage, rendering, change notification
- `internal/component/plugin/server/template_test.go` -- unit tests
- `test/managed/fleet-template-fetch.ci` -- functional test
- `test/managed/fleet-template-change.ci` -- functional test

## Design presentation

### What changes

An operator can assign a template to a group, use each device's name and labels in its configuration, and override a single device by writing its direct configuration. Fetch and change notifications hash the effective bytes each device will receive. Three list commands and two device-specific commands supply the same assignments and versions to operators and the future fleet web parts. Viewing raw desired configuration requires edit authorization; read-only users receive an explicit refusal for that raw-text operation.

### Before and after

Today the operator maintains a separate configuration for every edge router. With a group template, a change to the shared uplink setting updates every member that uses it. An offline member fetches the change when it returns. A router with its own explicit configuration continues to receive that configuration. A router with an emergency local edit shows a hold for the operator to resolve.

### Alternatives set aside

Rendering on the device would require each client to understand the hub's template language. Copying the rendered configuration into each device's direct blob would turn those copies into permanent overrides. Rendering when the effective configuration is read preserves a single group assignment and a deliberate per-device override.

### Documents departed from

None. The existing hub configuration reader still supplies the bytes for both messages.

### Choices for Thomas

The existing design specifies one group per device. The implementation must enforce that assignment and report conflicting configuration at validation, so two templates never compete to supply the same device's configuration.

## Implementation Steps

### Implementation Phases

Before the rendering phases, exercise the real config-fetch with a group template and a direct override in `test/managed/fleet-template-fetch.ci`. Also prove that the change notification and the following fetch name the same effective version.

1. **Phase: Template engine** -- template parsing, variable resolution, rendering
   - Tests: `TestTemplateRender`, `TestTemplateRenderLabels`, `TestTemplateRenderUndefinedVar`
   - Files: `template.go`, `template_test.go`
   - Verify: unit tests pass

2. **Phase: Template-aware ConfigReader** -- intercept HandleConfigFetch with template lookup
   - Tests: `TestTemplateAwareConfigReader`, `TestTemplateAwareConfigReaderDirectBlob`
   - Files: `managed.go`, `template.go`
   - Verify: a member without a direct configuration gets rendered bytes, and a direct configuration wins for grouped and ungrouped devices

3. **Phase: YANG + CLI** -- template and group configuration, `show fleet templates/groups`
   - Tests: `fleet-template-fetch.ci`
   - Files: YANG schema, CLI commands
   - Verify: templates configurable via CLI editor

4. **Phase: Change propagation** -- template edit triggers config-changed
   - Tests: `TestTemplateChangeNotification`, `fleet-template-change.ci`
   - Files: `template.go`, `managed.go`
   - Verify: template edit causes connected devices to re-fetch

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Version hash computed from rendered output, not template source |
| Naming | YANG template/group uses kebab-case |
| Data flow | Config-fetch -> group lookup -> template render -> response |
| Rule: exact-or-reject | Missing template variable rejects, does not silently substitute empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Template injection | Restricted function set in text/template (no shell, file, exec) |
| Input validation | Template source at most 64 KiB and fully rendered configuration at most 1 MiB. Values come from the validated name and labels. |
| Resource exhaustion | Reject looping or invoking syntax at parse time. Bound source, labels and render output before allocation. |

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
