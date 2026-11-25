# Spec: fleet-8 -- Device Samples and Hub-Side Metric History

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | fleet, protocol |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (the hub's metric history of its devices) in the first release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-4-inventory-health.md |
| Phase | 8/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `plan/post-release/features/spec-fleet-0-umbrella.md`,
`plan/release/features/spec-fleet-4-inventory-health.md`, `internal/component/l2tp/cqm.go` (a
bounded bucket ring), `docs/architecture/fleet-config.md`.

## Task

Let the hub keep a week of history for every device. Managed devices report a
small, fixed set of figures at a regular interval; the hub keeps them in a
bounded store that survives a restart, and answers queries for a series, for
one device or summed over a set of devices, as commands on every command
surface (CLI, SSH, MCP, REST, gRPC, the web command line).

This is the data behind the resource alarms
(`plan/release/features/spec-fleet-ui-5-alarm-list.md`) and,
after the first release, behind every chart and every "latest value" in the
fleet web interface.

### Owner ruling of 2026-11-23: commands in the first release, pages after it

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the
first release are reached through commands on every command surface, and that
their web pages and views come after the first release (`ai/META-DECISIONS.md`,
"Goal and scope"). This spec carries no chart, page or web view: the charts are
`plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md`'s, and
the two parts of the fleet interface that `spec-fleet-1-device-registry.md` had
given this spec moved with that work: `HistoryPart` to `fleet-ui-3` (its
AC-9) and `LatestSamplePart` to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`
(its AC-17). Where a rule below said what a chart shows, the command's answer
now says it (AC-11), and AC-10 proves every query on every command surface.

### What this changes in the fleet umbrella

`plan/post-release/features/spec-fleet-0-umbrella.md` listed "External metrics aggregation" as out of
scope: operators use Prometheus, and the fleet provides scrape targets, not a
metrics store. The owner changed that on 2026-11-18 for one bounded purpose:
the hub keeps about a week of a fixed sample set so its own dashboards can draw
charts. The earlier decision still holds for everything else:

| Still true | Detail |
|------------|--------|
| Not a metrics store | The hub keeps a fixed set of figures at coarse resolution. It does not store arbitrary metrics, labels or queries |
| Prometheus stays the detailed source | Every device still exports its full metrics. Anything beyond the fixed set, or older than the retention, is the operator's Prometheus |
| Devices report, the hub aggregates | Samples are pushed by the device on the managed channel, as health and inventory are in `fleet-4` |

### The sample set

Proposed, to be settled at the design gate. Each figure is registered by the
feature that owns it, so the list is a registry, not a table in the fleet code.

| Figure | Unit | Owner |
|--------|------|-------|
| CPU used | percent | host |
| Memory used, memory total | bytes | host |
| Storage used, storage total (root store) | bytes | host |
| Load, one minute | number | host |
| Uptime | seconds | host |
| Interfaces up, interfaces configured | count | interfaces |
| Traffic in, traffic out, summed over physical interfaces | bits per second | interfaces |
| BGP peers established, peers configured | count | BGP |
| Routes held | count | BGP |
| Subscriber sessions | count | L2TP and PPPoE, when present |

A figure whose owner is compiled out or not configured is absent from the
report, and the queries answer it as absent rather than as zero (the charts of
`fleet-ui-3` show that answer after the first release).

### Reporting

| Rule | Proposal |
|------|----------|
| Verb | `sample-report` on fleet-4's report dispatcher. The authenticated managed session identifies the device. One report carries a bounded set of registered, finite numeric values in the units above. An absent optional owner supplies no value. |
| Interval | One minute by default. The hub advertises its configured interval in the successful config fetch at connect. |
| While offline | Keep the last sample and its hub receipt time through disconnect. The device does not buffer. An offline period has no series points. Reconnect starts a new sampling interval and never inserts the retained last value as a fresh sample. |
| Size | A report is a few hundred bytes, far inside the frame bound of the channel |
| Rate | Admit one report per device per configured UTC interval, including across reconnects. Rejected attempts leave the last good value, timestamp and next eligible interval intact. Authenticate the current session before applying any report. |
| Older devices | A device that has never sent samples has no history. Say that no samples have been received. Only describe reporting as unsupported when the device has explicitly reported that fact. |

### The history store

It extends the shape of the L2TP quality ring: fixed buckets, a pooled ring per
series, retention bounded by configuration.

| Property | Proposal |
|----------|----------|
| Resolutions | A one-minute bucket for one day and a fifteen-minute bucket for seven days. Keep the number and positions of contributing minutes, sum and maximum in a coarse bucket. Also retain its per-minute contributions in a bounded packed array for set maxima: two different devices can peak in different minutes. Derive an estate peak from the aligned contributions actually observed together. A bucket with no measured values is a gap. A partially measured bucket carries its coverage. Queries crossing the one-day boundary count each period once. |
| Per series | One ring per device and figure |
| Persistence | Written to the hub's state store on a timer and at shutdown, reloaded at start. With healthy storage an unclean stop can lose data since its last successful checkpoint. Failed checkpoints expose their age. |
| Budget | Proposed defaults are 1 MiB per device and 2 GiB across the hub. Charge the complete persisted representation, including indices, retained latest values and write headroom. Shorten retention oldest first under either cap. Publish the effective retention and the capacity-pressure fact consumed by the History full alarm. |
| Removal | Undeclaration marks the device record RemovedAt. Keep its history through disconnection, undeclaration and re-declaration of the remembered identity. When that raw device record is actually deleted, fence its history writer and erase its shards and latest observation. A subsequently created identity starts empty. |
| Retention | Propose one to seven days, default seven. Reserve space for the latest value of each declared device before accepting a smaller cap. Refuse an impossible configuration with the minimum required space. |

### Queries

Every query is a command, so the same data is available on every command
surface (CLI, SSH, MCP, REST, gRPC, the web command line), and after the first
release to the web pages, which read it through the dispatcher.

| Query | Answers |
|-------|---------|
| Latest samples of one device | The most recent figure of each kind, with its age |
| Latest samples of a set | For the estate or a set of labels: the sum or the average, as the figure's kind requires, and the count of devices contributing |
| One series | For a figure, a device or a set, and a range: buckets reduced to at most a requested number of points, each with average and maximum, gaps preserved |
| Ranking | The devices with the highest latest value of a figure, in a set, to a requested count |
| Store status | Retention in force, size against the cap, oldest sample, devices using most |

### Key design decisions (to confirm at the design gate)

| Decision | Alternative | Reason |
|----------|-------------|--------|
| The device pushes samples | The hub scrapes each device's metrics endpoint | The managed channel already reaches devices behind address translation; the metrics endpoint does not |
| A fixed, registered sample set | Forwarding the full metrics registry | A bounded store needs a bounded set. The full registry is what Prometheus is for |
| Two resolutions | One resolution for the whole week | Coarse buckets bound week-long query and index costs. Their packed minute contributions support exact peaks for an arbitrary authorized device set |
| Persisted on a timer | Written per sample; memory only | The owner chose persisted and bounded. A timer bounds the writes; memory only would empty every chart at each hub upgrade |
| Sums are computed at query time | Stored aggregate series per label | Labels change; stored aggregates would be wrong after a relabel |
| The cap shortens retention | Refusing new samples at the cap | Recent history is worth more than old history |

### Risks to resolve in design

| Risk | Question |
|------|----------|
| Store size | The per-device budget times the largest estate must fit the hub's store. The figures above must be measured, not assumed |
| Write load | A timer write of every ring must not stall the hub's other use of the store |
| Query cost | A sum over a thousand devices and a week must answer in bounded time |
| Clock difference | Stamp each accepted report using the hub's receipt time; use UTC slots for queries. Device time is diagnostic only. A backwards hub wall-clock step cannot replace an accepted slot or create duplicate points. |
| Transport behaviour | The channel enforces a write deadline; a report that cannot be written is a disconnect, handled as one (`plan/post-release/features/spec-fleet-0-umbrella.md`, post-wave corrections) |
| Counter figures | Traffic is reported as a rate computed on the device, so a device restart cannot produce a false spike |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected device | Its figures arrive at the interval and are answered by the latest-samples query | A figure whose owner is absent is missing from the answer, because the device does not have it |
| AC-2 | A week of reports | The series query answers the fine resolution for the last day and the coarse for the last week | None |
| AC-3 | The hub restarts | The history is present after restart, less at most one timer period | None |
| AC-4 | A device offline for an hour | The series shows a gap for that hour | None |
| AC-5 | The store reaches its cap | Retention shortens, oldest first, the alarm is raised, and the store does not exceed the cap | Recent samples are always kept |
| AC-6 | A set query over the estate | The sum or average over the devices that report, with the count contributing | Devices that do not report are left out and counted as such |
| AC-7 | A device's retained registry record is deleted | Its history is gone, including after restart. Undeclaring the device alone preserves the remembered record and its history | None |
| AC-8 | Reports faster than the interval allows | They are refused and counted; the store does not grow faster than configured | None |
| AC-9 | A standalone device started on its normal local configuration | `show fleet status` reports its standalone mode and ordinary device reads succeed. Instrument the sender, sampler clock and history-store open to prove zero calls from fleet sampling. A managed peer negotiating with an old hub still receives and applies the normal config-fetch reply. | Local device commands and ordinary configuration fetch/ACK remain available independently of sample history. |
| AC-10 | Each query of the Queries table asked on the CLI, SSH, MCP, REST, gRPC and the web command line | Each surface lists the command and the daemon dispatcher serves it there (`TestEveryCommandOnEverySurface`, `cmd/ze/hub/surface_parity_test.go`); every surface's answer is the one the hub's handler gives the dispatcher. Added by the owner ruling of 2026-11-23 | None |
| AC-11 | A device that sends no reports, and a figure whose owner is absent on a reporting device | Latest and series identify no reports received or a missing figure. A reported zero is returned as a measurement. | An explicit unsupported capability may identify older device software. |
| AC-12 | A device disconnects, stays offline for an hour, then reconnects | Latest retains its previous measurement and age, including across a hub restart. Series contains no new measurement in the outage. The first qualifying observation after reconnect has its own receipt time. | An older sample may still appear in a query range that includes its actual receipt time. |
| AC-13 | A rejected, repeated, excessively large or malformed report, or a report from a replaced session | Reject the whole submission before changing history. A valid later report still succeeds. Another device's history is unchanged. | None |
| AC-14 | A range crosses resolutions or includes reports from devices with missing figures | Return counts and coverage, preserve gaps and compute averages using contributing samples. Summable counters use registered sum rules. Capacity ratios use summed numerators and denominators from the same reporting devices. | An absent denominator, such as zero installed capacity, returns unavailable. |
| AC-15 | A checkpoint fails while new samples arrive, or a device is removed during checkpoint | Keep dirty data eligible for retry, expose the failed checkpoint through doctor, and prevent a late flush from resurrecting a removed device. On restart load the last complete checkpoint and preserve its timestamps. | Successful committed shards remain readable even if another shard's write failed. |

No criterion of this spec names a chart, a page or a web view. The charts over
this history are `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md`'s,
its history administration page is
`plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md`'s.

## Existing Mechanisms

| Mechanism | Source | Use here |
|-----------|--------|----------|
| Managed session and registered reports | `pkg/fleet/envelope.go`, `internal/component/plugin/server/managed_serve.go`, fleet-4's proposed report dispatcher and producer registration | Today the managed server handles config fetch and ACK explicitly. Fleet-4 must first land its registered producer and receiver extension. Fleet-8 then registers the sample sender and its receiver there. A device sends samples over its existing authenticated session. The hub rejects replaced sessions and checks report bounds before storing. |
| CQM series | `internal/component/l2tp/cqm.go`, `sampleRing` | Its wraparound and ordered snapshot illustrate fixed storage. The L2TP bucket contains session and RTT fields, so the fleet ring owns its own numeric bucket. |
| Local readings | `internal/component/telemetry/collector/cpu_linux.go`, `memory_linux.go`, `loadavg_linux.go`, `uptime_linux.go` | These exporter-side collectors demonstrate the procfs/statfs readings, units and delta semantics. The always-available host sampler reads the same underlying OS APIs in the host package. It never imports the exporter or its collector package. Each optional feature registers only figures it can supply. The managed package does not import BGP, L2TP or a platform collector. |
| Device registry | `internal/component/plugin/server/managed_registry.go`, `ManagedRegistrySnapshot` | Declared clients, labels and session transitions supply membership. The snapshot contains no caller permissions. A name only present in a report cannot create a device. |
| Persistent state | `pkg/zefs/keys.go`, the borrowed hub store | Register a key family for bounded, independently replaceable shards. Write raw state keys instead of creating config-file version history on every flush. |
| Typed command authorization | `internal/component/plugin/server/command.go`, `CommandContext`, `Dispatcher.isAuthorizedCommandArgs`; `internal/component/aaa/command_args.go`, `CanonicalCommandTokens` | Add a request-bound local scope projection. The daemon authorizes the actual history command by its normal provider, including TACACS+, once. Project device names against the authenticated session's local profile generation using the canonical `show fleet device name <value>` read tokens. A grant to the aggregate command delegates the resulting local device set; remote installations that restrict individual device reads configure that same scope in the mapped profile before granting an aggregate. Fail closed if a remote profile cannot be resolved. The projection never calls the remote server per device. An absent or denied device gives the same operator answer. |
| Owning processes | `internal/component/plugin/server/stage1_commands.go`, `command.go`, `SendExecuteCommandAnswer` | Plugins already register commands across their actual process boundary. Each optional producer registers a read-only structured source command in its owning package; dispatch to the registered owning process obtains a bounded scalar snapshot. See Data Flow for discovery, local and forked transport, and loss of a producer. |
| Declared lifecycle, persistence and diagnostics | `ApplyManagedListeners`, `loadDeviceRecords`, `forgetDeviceRecordLocked`, `ManagedDoctorSnapshot` in the managed server | Observe declaration changes separately from actual deletion. Keep peak storage bounded when writing shards. Attach the age and failure of the latest checkpoint to existing managed doctor evidence. |
| Hub settings and command schema | `internal/component/config/loader_extract.go`, `ExtractHubConfig`; `cmd/ze/hub/managed_server.go`, `startManagedServer`, `prepareManagedReload`; `internal/component/managed/register.go` | Feed validated settings into the service during start and accepted reload. Register result shapes, columns, command-specific pipes and daemon handlers beside fleet-1’s commands. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The daemon command supplies one authorized answer to all transports. |
| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to internal/component/managed/yang/ze-managed-cmd.yang | "Caller identity is injected only by trusted transport wiring." | agrees | Apply device visibility from the authenticated caller before computing totals or enumerating devices. |
| `docs/architecture/fleet-config.md` | RPCs | anchored to internal/component/managed/yang/ze-managed-cmd.yang | "All RPCs are scoped to that name implicitly." | agrees | Bind each input to the registered authenticated session and reject stale sessions. The payload cannot select a different device. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering internal/component/managed/yang/ze-managed-cmd.yang | "No always-on file imports a gated package." | agrees | Discover optional producers by registration. The hub and managed client import no optional dataplane feature to obtain its measurements. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | Declare operator settings and bounds through YANG and its existing schema validation. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/yang/ze-plugin-conf.yang | "The SDK stays generic." | agrees | Hub-specific state belongs to the managed service. A forked producer supplies facts across its supported boundary. |
| `docs/architecture/zefs-format.md` | Key Registry | the // Design: page of pkg/zefs/keys.go | "All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`" | agrees | Register the raw key families. Check each encoded shard against its ValueMax before WriteKey, and enforce the byte bound before decoding data read from storage. The registry documents the bound; the history writer enforces it. |
| `docs/architecture/container/image-store.md` | State keys | anchored to pkg/zefs/keys.go | — | silent | The container store owns its image and project records. Fleet uses separate registered state keys and never presents a fleet reading as image state. |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | anchored to pkg/zefs/keys.go | "`List(prefix)` fails when the prefix ends in a slash." | agrees | Use storage.Storage.ListKeys with the registered directory prefix including its trailing slash. The adapter translates to the backend. Never pass that slash to low-level ZeFS.List. |
| `docs/architecture/irr-owner.md` | Who may touch which key | anchored to pkg/zefs/keys.go | — | silent | IRR grants dataset and interest keys to named routing consumers. These grants do not cover fleet state, which belongs to the managed hub. |
| `docs/architecture/storage-backends.md` | Durable publication | anchored to pkg/zefs/keys.go | "Sync failures are returned to the caller." | agrees | Retain dirty state on failure, expose degraded durability and retry publication without marking an uncommitted record flushed. |
| `ai/doors/core.md` | Boundaries | the door covering pkg/zefs/keys.go | "Runtime state goes through the daemon's one store." | agrees | Use the hub’s borrowed store and registered keys. Optional device producers never open the hub store. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-history-reconnect.ci | "Functional `.ci` files live under `test/<suite>/`" | agrees | The managed suite drives authenticated devices and the same commands served to operators. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Command paths identify the subject. Presentation and generic filtering use the command registry’s existing pipe operations. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | These commands return structured data and inherit generic filtering and output shapes. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Reading or piping an answer never triggers a device sample, a state transition or a configuration write. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Existing authenticated sessions feed fleet state. The certificate reload path and its live socket continue to operate independently. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | Use the same per-device identity and elapsed threshold interval; the first release keeps visibility as command data. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | A fresh receipt after reconnect is distinct from the last known value kept during disconnect. Terminate the old resource occurrence and require new qualifying samples. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The release command carries the state. Transient page notifications belong to fleet-ui-9 and are scoped to the viewer. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | These queries never change a rollout or cause a device to receive configuration. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | The later device tab consumes the same authorized per-device facts. All first-release answers are available through commands. |
| `docs/architecture/api/architecture.md` | Command parameter values | Hub and managed service wiring | "The command model then accepts or refuses that value." | agrees | The handler and command model validate the requested subject and query inputs on every surface. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | Hub and managed service wiring | "Each component under `internal/component/` is independently removable." | agrees | Only the feature that owns a measurement contributes it. The managed service consumes registered observations. |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Hub and managed service wiring | "The listeners keep their sockets, and the next handshake serves the new certificate." | agrees | A certificate-only reload leaves device sessions, declaration membership and their timing intact. |
| `ai/doors/config-yang.md` | Where it sits | Hub and managed service wiring | "The schema is spread across components." | agrees | Declare the typed settings in the owning module and have the config loader carry them to the hub. |
| `docs/architecture/hub-architecture.md` | SSH session model factory | Hub and managed service wiring | "Both owners finish before the hub returns its store to the launcher." | agrees | The managed service drains outstanding state writers before returning the borrowed store. |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | Hub and managed service wiring | "Its producer raises it on the report bus" | agrees | Publish degraded evidence and its recovery through the existing managed doctor and report mechanisms. |
| `ai/rules/repo-maintenance.md` | Directives | Hub and managed service wiring | "the owning `docs/architecture/` page for a contract" | agrees | Update the fleet architecture page with the new lifecycle, command and storage contracts. |
| `docs/architecture/api/architecture.md` | Design decisions | Decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Fleet reads register their result shapes and do not alter configuration display, including inline implied annotations. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Offline provisioning remains available to the authorized device editor. Reading fleet observations cannot modify or apply device configuration. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | The hub consumes existing observations across certificate rotation. It does not change any listener protocol, certificate source or socket. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | The hub consumes existing observations across certificate rotation. It does not change any listener protocol, certificate source or socket. |
| `docs/architecture/api/process-protocol.md` | Wire Format | Hub and source wiring | "Every message is a single newline-terminated line:" | agrees | The source commands use the existing execute-command transport. Their bounded replies follow its usual record and terminator framing. |
| `docs/architecture/api/ipc_protocol.md` | Overview | Hub and source wiring | "Ze uses correlated, bidirectional RPC between the engine and plugins." | agrees | Target the current owning process, correlate the answer with that request and discard a late response. |
| `docs/architecture/api/json-format.md` | Field naming | Hub and source wiring | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Use kebab-case for figure names and the optional interval field. |
| `docs/architecture/cli/command-verbs.md` | Read verbs and their guarantee | Hub and source wiring | "That promise is what lets a read-only operator run it" | agrees | Reading history and sampling an existing local counter cannot install device configuration or alter routing. |
| `docs/architecture/cli/error-surface.md` | The rule | Hub and source wiring | "A fault goes to the error zone." | agrees | Return query and permission errors through the existing command error path. A missing optional reading is a fact of the answer. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | Hub and source wiring | "Plugins decide what they need." | agrees | Prepare bounds before committing a new configuration and preserve the old effective settings when the candidate fails. |
| `docs/architecture/encoding-context.md` | Overview | Hub and source wiring | — | silent | The figures count held routes and peers. They never read or produce an encoded UPDATE. |
| `docs/architecture/plugin-manager-wiring.md` | Solution: Two-Phase Startup | Hub and source wiring | "The server is constructed in the hub before Engine.Start." | agrees | Discover live source owners after startup and discard results from a replaced owner. |
| `docs/architecture/web-interface.md` | Admin command visibility | Hub and source wiring | "Admin command discovery omits schema-hidden nodes and their descendants." | agrees | Fleet read commands reach the shared web dispatcher under their declared visibility and execution authorization. |
| `ai/doors/api-surfaces.md` | Where it sits | Hub and source wiring | "They are thin adapters." | agrees | All six command surfaces receive the authorized daemon’s answer. No web handler reads the history store. |
| `ai/doors/forwarding.md` | Where it sits | Hub and source wiring | "Components with a pluggable backend." | agrees | Measure physical links from the configured plane. A backend without a measurement contributes no value. |
| `ai/doors/bgp-plugins.md` | Where it sits | Hub and source wiring | "Each is an edge plugin of the BGP component." | agrees | The peer and route-count producers live with their BGP owners. When those owners are absent, their figures are unavailable. |
| `ai/doors/access-security.md` | Where it sits | Hub and source wiring | "Components, each removable." | agrees | The L2TP and PPPoE sources register only when present, and the daemon checks device-read authorization on the original caller’s policy. |
| `docs/architecture/update-building.md` | Design decisions | Decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/appliance/remote-operations.md` | Decisions | Managed composition and producing owners | — | silent | The sampler is a managed-channel reader of device observations. It never pushes an appliance image or configuration. |
| `docs/architecture/behavior/signals.md` | Supported Signals | Managed composition and producing owners | — | silent | Sampling introduces no process signal. Existing startup and shutdown own the history worker. |
| `docs/architecture/cli/command-completion.md` | What Tab and `?` reveal | Managed composition and producing owners | "Ordinary requests return the candidate array." | agrees | Public sample commands appear in ordinary discovery and designated hidden source commands stay out of its candidate list. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | Managed composition and producing owners | "YANG defines format. Extensions declare behavior. Implementation executes behavior." | agrees | Each producer declares its exact ze:command and registers its handler. |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | Managed composition and producing owners | — | silent | History checkpoints own registered persistent state; the panic crash descriptor and reporter remain independent. |
| `docs/architecture/pki/pki-store.md` | Decisions | Managed composition and producing owners | "A consumer names an entry, and the config checks the name." | agrees | Use the existing authenticated managed channel and its configured certificate. |
| `docs/architecture/system-architecture.md` | Build personalities | Managed composition and producing owners | "The repository builds `ze` and `le` from the one `cmd/ze` codebase." | agrees | Install the producer dispatcher in the runtime composition root, keeping excluded feature packages out of the stripped binary. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | Managed composition and producing owners | "All lines use key=value format with `:` separators:" | agrees | Drive real managed clients and command answers through the existing managed CI fixture. |
| `docs/architecture/aaa-tacacs.md` | A registry, not a hardcoded chain | Managed composition and producing owners | "The hub composes them in priority order" | agrees | Remote authorization judges the actual submitted command. Device-name projection uses the explicitly mapped local profiles bound to that login; missing mapping denies the result. |
| `ai/patterns/registration.md` | The Model | Managed composition and producing owners | "Registry.Register(...)" | agrees | Each producer registers through its own package and is discovered from the live command registry. |
| `ai/doors/system-plugins.md` | Where it sits | Managed composition and producing owners | "Two shapes share the directory." | agrees | The host command plugin owns its command registrations. It calls the independent host measurement component. |
| `docs/architecture/api/wire-format.md` | Transport | Managed composition and producing owners | "Messages are UTF-8 lines terminated by exactly one newline byte (0x0A)." | agrees | The plugin source uses the existing framed, correlated execute-command transport. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | Managed composition and producing owners | — | silent | The source reads the live RIB owner’s input count. It does not choose an encoding or reuse a route pool for fleet history. |
| `docs/architecture/route-selection.md` | Overview | Managed composition and producing owners | — | silent | The reported total counts the RIB owner’s inbound paths. It neither filters candidates nor selects best paths. |
| `docs/architecture/update-building.md` | Three Paths for UPDATE Messages | Managed composition and producing owners | — | silent | A count of retained routes observes the RIB without parsing, formatting or emitting a BGP UPDATE. |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | Managed composition and producing owners | "The session is up." | agrees | Count peers whose observed FSM state is Established. The observation sends no message and starts no session. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Current Ze State | Managed composition and producing owners | "provides a no-op backend for tests and disabled telemetry." | agrees | Device snapshots read underlying sources and work independently of the optional exporter and registry backend. |
| `docs/architecture/rib-transition.md` | Executive Summary | Managed composition and producing owners | "all RIB data and logic lives in API programs" | agrees | The bgp-rib plugin supplies its own total. The managed client never holds routes or imports a RIB. |
| `docs/architecture/rib/forward-handle.md` | The decisions | Managed composition and producing owners | — | silent | The history producer reads a count. It never captures, forwards or releases a best-change handle. |
| `docs/architecture/appliance/builder.md` | Secrets at rest | AAA lifecycle source anchor | — | silent | The profile projection observes a live accepted generation. It never writes an appliance credential or an image. |
| `docs/features/interfaces.md` | Interface Discovery | Physical interface owner | "Results are sorted by type then name." | agrees | The sampler determines physical membership from the owning interface view. It consumes that observation without changing discovery, initial configuration or ownership. |
| `docs/features/interfaces.md` | Design decisions | Decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | A read-only interface sampler cannot claim a device or answer the initial provisioning prompt on an operator’s behalf. |
| `docs/features/interfaces.md` | Design decisions | Decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | A read-only interface sampler cannot claim a device or answer the initial provisioning prompt on an operator’s behalf. |

## Data Flow

1. On a managed session, fleet-4's sender obtains one snapshot from the registered metric producers. Percentage and rate producers skip their first value until they have a valid baseline. Counter rollback, restart and interface membership changes reset the affected rate baseline. Standalone startup creates no sender.
2. The hub bounds decoding before allocation. Propose 8 KiB per frame payload and at most 16 named figures. Reject duplicate names, a non-finite or out-of-domain value, or an unknown schema version with an error on that request. The authenticated client name never comes from the payload. Receipt and disconnect events are ordered against the current session.
3. Insert each point into its UTC slot. Keep the latest accepted report separately so evicting an old bucket does not erase the last-seen value. Retain no forward-filled or zero-filled points. On registry record deletion cancel its outstanding writes and delete both the latest record and its shards.
4. Propose a checkpoint every five minutes plus a final joined flush on graceful shutdown. Flush changed bounded shards independently of the ingest lock, with per-device generation checks and atomic replacements. An unclean stop can lose the unflushed interval under healthy storage. A write failure exposes the age of the last successful flush; the five-minute target does not describe a failed disk.
5. Register three read subjects, `show fleet sample latest`, `show fleet sample series` and `show fleet sample storage`. Latest returns one row per device with its named figures, receipt time and availability. For example, `show fleet sample latest | rank cpu-used | first 10` orders the authorized rows by their reported CPU value descending and breaks ties by device name, then takes ten. The command-specific rank pipe drops rows missing that figure. Register label and device filters and a reduce pipe for sums, means and contributor counts. Each pipe acts on the preceding answer, so a label filter before reduction limits the contributing set. Device, range, figure and label selection restrict the requested data; paging and formatting operate on the returned data. Apply the caller's device visibility before summing, ranking or counting. A set answer states how many members supplied the figure and how many were missing or stale.
6. For totals, sum only readings with matching units. CPU use and load are means over reporting devices. A device's uptime is meaningful individually; a set answer can report mean uptime and its contributor count. Across time, combine sums and counts before computing a mean. Derive connected-device counts from the hub's own connection observations if requested by dashboards; a device cannot report that estate-wide fact. Devices do not synthesize health samples during disconnect.

### Query grammar and returned rows

Declare `latest`, `series` and `storage` as three argument-free YANG `ze:command` subjects. Their registered Go handlers declare the operations below with RegisterPipeFilters. None of these operators is a positional command argument or a YANG input leaf. Authenticated device visibility is applied before the logical input to the first pipe.

| Subject and pipe | Argument and effect | Returned shape |
|---|---|---|
| Latest or series: `device`, `label`, `figure` | Each takes ArgText. Device matches an exact registry name; label takes one `key=value` and repeated labels intersect; figure selects one registered figure including its availability. | Latest retains one observation row per authorized matching device. Series retains timestamped per-device, per-figure rows with UTC bounds, sum, count, observed-minute coverage and maximum. Each row carries the labels at query time. |
| Series: `from`, `until` | Each takes an RFC 3339 UTC instant (ArgText). The range is half open. Defaults span the retained horizon through the query’s captured hub time. Reject an empty or reversed range. | Per-device series rows clipped to the requested, supported resolution; publish actual bounds and partial coverage. |
| Latest: `reduce` | No argument. Fold the selected observations using each figure’s registered aggregation rule. | A table with one row per selected figure, containing figure, unit, value, contributor count and missing count. The table form is stable for one device or an estate. |
| Series: `reduce` | No argument. Fold authorized devices for each figure and aligned UTC slot. | One row per figure and slot with bounds, value, sample count, contributor count, coverage and maximum. Use summed source observations and weights for means and ratios. |
| Series: `points` | ArgCount, limited to 1–2,112. Combine adjacent UTC buckets for each series into at most that many points, including explicit empty buckets. | The same bucket table, with weighted averages, peak and coverage over each wider bucket. |
| Latest: `rank` | ArgText naming a figure; sort measured rows descending then by registry name. Follow with the generic `first` for a top-N query. | A reordered table of contributing devices. |

For example, `show fleet sample series | figure cpu-used | label region=west | from 2026-11-24T00:00:00Z | until 2026-11-25T00:00:00Z | reduce | points 300` produces the data for a west-region chart. Latest and series work on the same authorized pinned snapshot. The handler streams or pushes down filters and aggregates only where their ordering yields exactly the answer of the written pipeline. At most 4,096 result rows and 4 MiB of serialized data may leave the history handler in one answer; these proposed output bounds are independent of the number of stored observations it may fold. A query exceeding either bound receives an explicit refinement error before sending partial rows. Device and label selection on an already reduced result is refused because the result no longer contains device identities. No pipe starts a measurement, changes configuration or widens the authorized set.

### Device producers and hub lifecycle

Each contributing owner declares its read-only structured command under `show fleet source <owner>` using the existing command registration, including its owning plugin’s Stage 1 declaration when it is a separate process. The managed sender discovers registered source commands by that exact prefix from the device dispatcher. It invokes each live source once per negotiated tick with its own bounded deadline and a cancellation context tied to this managed session. In-process handlers return their structured answer directly; a forked owner receives the existing execute-command request and returns its structured answer over its PluginConn. No callback or Go pointer crosses processes. A stopped or replaced source, an old owner without the command or a timeout contributes no reading. The later sender drops late replies by session/tick generation. Mark every source command hidden in its owning ze:command schema and Stage 1 declaration. Hidden controls completion only. A typed invocation still requires ordinary read-only command authorization; the sampler invokes only the registered read-only source for its owning group. The bgp-rib command uses its actual -cmd YANG path and cmd/rib forwarding proxy; the owner declares the same command in Stage 1 and executes its read. For iface, keep the sampling implementation in the iface plugin and the plugin/server import in iface/cmd; its registered proxy reaches commandDecls/executeCommand in rate_command.go. Enumerate from the full registry (including Hidden), never the operator-discovery list. The real hub injects its current server dispatcher into ClientConfig at runYANGConfig’s newManagedClientOwner callback. Standalone startup never invokes the commands. A mixed builtin/forked test exercises the same source in both modes, with telemetry export disabled.

The Linux host source uses procfs and statfs directly from internal/component/host for CPU, memory, load, uptime and root filesystem totals. The ordinary host command module exposes it. With ze_telemetry absent, no internal/component/telemetry/collector symbol is linked. On an unsupported platform the same host registration returns only readings the platform can provide. Convert KiB to bytes at the source. The interface owner reads configured/current physical interface membership and sums physical octet deltas only across a stable membership, converting to bits per elapsed second. BGP's peer summary source counts established and configured peers, and the configured bgp-rib plugin supplies the total current inbound paths from RIBManager’s existing per-peer input counts (the source of routesIn). An ordinary bgp router loads this owner via ConfigRoots. Never require or add the distinct bgp-adj-rib-in validator or count the same routes twice. L2TP’s service snapshot supplies its subscriber session count. PPPoE’s access-concentrator snapshot supplies its subscriber sessions; exclude outbound PPPoE client interfaces. Keep these as distinct contributions on the device and sum those present into the single wire subscriber figure. A configured producer can report a real zero. Limit every source answer to its declared figures and units, reject duplicate ownership except that explicit additive pair, and reject a malformed measurement without representing it as a measured zero. The fixed versioned wire vocabulary bounds what the hub accepts. It does not cause a missing feature to run.

At the hub, ExtractHubConfig carries sampling settings into startup and the prepared reload plan. Validate retention, interval, per-device and global byte caps before applying the generation. Cancel and join outstanding checkpoint and report work before releasing the borrowed store. On actual record deletion, serialize the delete fence and erase operation against queued writes, persist deletion intent until all shards are deleted and recover interrupted deletion before loading history. Record removal followed by recreation of a name cannot reuse the deleted generation. Enumerate keys with storage.Storage.ListKeys using the registered directory prefix with its slash. Verify each shard against its registered maximum before writing and before decoding. The registered `show fleet sample storage` read permission is specifically the grant for the hub-wide cap, utilization, oldest retained point and checkpoint state. Its handler runs only after that exact command is authorized, and filters every named device row through the same local scope. Latest and series authorize their own subjects and never return hub-wide usage. A caller without the storage grant receives the ordinary authorization refusal. In a policy-free installation normal dispatch permits reads. Across SSH, web, REST, gRPC, CLI and MCP, liveLocalAuthorizer, liveLocalProfileAuthorizer, acceptedLocalGenerationAuthorizer, acceptedLocalGenerationProfileFallback and the live bundle/bound-profile wrappers forward the projection with the same accepted generation, principal and resolved names. A bound local profile evaluates the name tokens locally. A TACACS+-authenticated reader must have an explicitly resolved local profile; use that bound mapping for local membership after the one remote command decision. A retired or absent AAA bundle denies, as do unsupported scope providers and unknown remote profile names. Preserve the existing explicit no-RBAC mode only for a live bundle whose selected authorizer is nil. Never treat a missing remote mapping as no RBAC. No positive scope survives an authorization generation change. Test mixed allow/deny names for both providers and count at most one remote authorization request for 4,096 declared devices.

### Sizing and recovery evidence

At default resolution one continuously reporting series requires 1,440 fine and 672 coarse slots. The proposed roster contains 15 scalar figures if aggregate subscriber sessions are one figure. At 24 bytes per slot, the example base payload is 760,320 bytes per device before packed minute contributions, per-series metadata, key overhead, latest values and temporary writes. As a worst-case control, 672 coarse buckets × 15 minute values × 15 figures × 8 bytes can add 1,209,600 bytes per device before compression. The 1 MiB default cannot guarantee seven full days at that density. Measure actual data and report the shorter effective retention; keep the hard ceiling. These are estimates. Qualify the 1 MiB and 2 GiB defaults against actual stored bytes, peak heap, restart reads and checkpoint writes with 2,000 connected clients, then repeat at the registry's 4,096 declared-client limit with sparse data. The default must report shortened retention accurately when space pressure prevents a full week. A hub must not preallocate a full ring for every possible offline client.

### Changed-Wire Census

**Search evidence:** Searched Go, test fixtures, templates, shell and JS for ConfigFetchResponse, config-fetch, sample-report, show fleet sample and the old/new interval key. The existing managed server and independent test fixture both write fetch replies; fetchConfigWithin decodes them. The sample method, source commands and history records are new.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|---|---|---|---|---|
| config-fetch reply | writer | plugin/server/managed.go HandleConfigFetch → managed_serve.go handleRequest; internal/test/fixture/misc_fixture_managed.go managedConfigFetchResponse; fake hubs in managed/client*_test.go, cmd/ze/hub/managed_server_test.go and pkg/fleet/envelope_test.go | Both current and changed-config success replies exist; fixtures use the older response. | Advertise seconds on either success. Keep older writers as compatibility cases. |
| same | reader | managed/client.go fetchConfigWithin, fetchAgainstBaseline and refetch paths; managed/handler.go ValidateConfig; cmd_fleet.go hubAnswerOf; plugin/server/managed_registry.go recordFetch; older external devices | A typed fleet.ConfigFetchResponse feeds existing config, status, hold and registry logic. | Read the optional interval once per accepted reply and leave other readers’ meanings intact; omission disables sampling. |
| sample-report and receipt | writer | managed/sample.go session sender; plugin/server/managed_history.go registered receiver returns RPC success or error | Fleet-4 provides the prerequisite managed report extension. | Only negotiated sessions send bounded, versioned observations and expect the corresponding receipt. |
| same | reader | plugin/server/managed_history.go current-session receiver; managed/sample.go sender handles receipt; old external managed servers | Older managed_serve.go returns an unknown-method error. | A method refusal disables this session’s sender. Invalid data or rate refusals reject only that sample. |
| device-local source command answers | writer | owner handlers in host, iface, BGP peer and bgp-rib, L2TP and PPPoE; plugin/server/command.go forwards execute-command | The process registry carries remotely owned command answers. | Use bounded scalar snapshots, exact units and the current owning process. |
| same | reader | plugin/server/fleet_sample_sources.go collector feeding managed/sample.go | One read-only source per live owner, usable across the fork boundary. | A missing or late owner contributes no figure. |
| fleet sample command answers | writer | managed/cmd_history.go registered latest, series and storage handlers | History answers introduce contributor counts, gaps, coverage, peaks and caller-scoped usage. | Keep stable structured output for all command surfaces. |
| same | reader | CLI, SSH, REST, gRPC, MCP and web command dispatchers; fleetui/history provider and fleetui estate latest provider after release; external command clients | cmd/ze/hub/surface_parity_test.go exercises dispatcher presentation; source_history.go and source_devices.go name downstream views. | Represent an absent reading separately from measured zero. |
| hub history shards and latest records | writer | plugin/server/managed_history.go checkpoint, deletion fence and shutdown flush | Registered raw storage keys persist between binaries. | Bound the encoded version and bytes; atomically replace complete shards. |
| same | reader | plugin/server/managed_history.go startup recovery and deletion replay | Read through borrowed storage.Storage under the registered prefix. | An empty old store yields no samples; reject malformed or unsupported versions and expose the failure. |


Propose an optional `sample-interval-seconds` field on `pkg/fleet/envelope.go`’s `ConfigFetchResponse`. `ManagedConfigService.HandleConfigFetch` sets it on both the changed-config and current answers when sampling is enabled. The managed client consumes it on its session’s successful fetch, including the current answer. An older client ignores the extra JSON field. An older hub omits it, so a newer device leaves the sender idle. If a peer advertises sampling but rejects `sample-report` as an unknown method, disable sampling for that session and leave configuration fetch and ACK available. A content or rate refusal rejects that report only. Negotiate again on reconnect. The mux framing, config hashes, ACK contents and hold/revert decisions retain their current meaning.

Exercise both directions with the older JSON shape and the existing simulated managed peer in `internal/component/managed/client_test.go`; include a current-config reply, sample refusal and reconnect. The three daemon subjects and their pipe queries are consumed through CLI, SSH, REST, gRPC, MCP and the web command line. Their registered answer carries units, missing values, authorization scope and time coverage on each surface.

## Wiring Test

| Entry point | Observation |
|-------------|-------------|
| `test/managed/fleet-history-reconnect.ci` | A real managed device reports a changing figure. The hub commands return its prior value after disconnect, an actual hole in the series and a new value after reconnect. |
| `test/managed/fleet-history-cap.ci` | A low configured cap evicts old history, keeps recent points and signals pressure. Removing the device and restarting the hub confirms it stays removed. |
| Named Go tests | Use an injected clock for rate limits, interval boundaries, a clock step and weighted rollup. A failing store controls checkpoint failure and removal races. Size tests measure encoded bytes and peak allocations with the stated estate. |

## Files to Modify
- `cmd/ze/hub/aaa_lifecycle.go` -- propagate the local projection through live, bound-profile, accepted-generation, profile-fallback and bundle authorizer wrappers used by every command surface
- `cmd/ze/hub/main.go` -- inject the local dispatcher into ClientConfig in the managed owner’s per-generation wiring
- `internal/component/aaa/command_args.go` -- define a local, request-bound device-scope projection protocol separately from per-command authorization
- `internal/component/aaa/login_profiles.go` -- preserve the accepted identity generation when providing local scope
- `internal/component/authz/register.go` -- implement projection using the existing Store.AuthorizeWithProfiles and the caller’s authenticated profile binding
- `internal/component/tacacs/authorizer.go` -- delegate the local projection to the session-bound mapped profile; reserve remote authorization for the requested command
- `internal/plugins/host-cmd/yang/ze-host-cmd.yang` -- hidden read-only host producer RPC binding
- `internal/component/iface/rate_command.go` -- add the iface producer to commandDecls and executeCommand for built-in and forked owners
- `internal/component/iface/cmd/register_rate.go` -- register the typed daemon RPC forwarding the hidden command to the live iface owner
- `internal/component/iface/cmd/interface_rate.go` -- use its dispatcher context to forward the source read to the iface plugin
- `internal/component/iface/yang/ze-iface-show-cmd.yang` -- hidden read-only physical-interface producer RPC binding
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` -- hidden read-only peer counts binding
- `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` -- hidden read-only ze:command for the typed source in BuildCommandTree
- `internal/component/bgp/plugins/cmd/rib/rib.go` -- register the builtin proxy for this source and forward it to the owning bgp-rib process
- `internal/component/bgp/plugins/rib/rib_commands.go` -- declare and register the hidden source with doRegisterBuiltinCommands
- `internal/component/bgp/plugins/rib/rib.go` -- expose the same owned source through commandDecls/Stage 1 and OnExecuteCommand in the forked plugin
- `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` -- hidden read-only L2TP source binding
- `internal/component/l2tp/pppoe/cmd/yang/ze-pppoe-cmd.yang` -- hidden read-only access-concentrator source binding
- `internal/component/plugin/types.go` -- carry interval, retention and budgets in HubConfig
- `internal/component/config/loader_extract.go` -- extract and validate the managed history settings
- `cmd/ze/hub/managed_server.go` -- carry effective settings to server startup and reload preparation
- `internal/component/plugin/server/managed_apply.go` -- apply the prepared settings and preserve remembered identities across undeclaration
- `internal/component/plugin/server/managed_serve.go` -- use fleet-4’s registered receiver, borrowed store and joined service lifecycle
- `internal/component/plugin/server/managed_record_store.go` -- fence history when the actual retained device record is forgotten
- `internal/component/plugin/server/managed_doctor.go` -- expose checkpoint age and failure
- `internal/component/plugin/doctor/check_managed_runtime.go` -- report the affected history check and recovery
- `internal/component/managed/register.go` -- register handlers, shapes, columns and filters for the three read subjects
- `pkg/fleet/envelope.go` -- add the optional advertised interval to both forms of the config-fetch response
- `internal/component/plugin/server/managed.go` -- advertise the configured interval on successful fetch
- `internal/component/managed/client.go` -- consume the advertised interval for the current managed session
- `internal/component/managed/client_test.go` -- prove old/new peer decoding, no-report fallback and reconnect negotiation
- `internal/component/managed/yang/ze-managed-cmd.yang` -- register the query command shapes
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- declare report interval, retention and budgets
- `pkg/zefs/keys.go` -- register history shard and latest-value key families
- `docs/architecture/fleet-config.md` -- specify sample wire and command semantics

## Files to Create
- `internal/component/plugin/server/fleet_visibility.go` -- authorize a named device read via the request's dispatcher policy
- `internal/component/plugin/server/fleet_sample_sources.go` -- enumerate and call live read-only source commands with bounded lifetimes, validating response owner and schema
- `internal/component/host/fleet_samples_linux.go` -- ungated Linux CPU, memory, load, uptime and filesystem sampler
- `internal/component/host/fleet_samples_other.go` -- platform counterpart which reports only supported readings
- `internal/plugins/host-cmd/cmd/fleet_samples.go` -- host command handler bound to the local host sampling API
- `internal/component/iface/fleet_samples.go` -- register physical interface counts and the traffic rate source
- `internal/component/bgp/plugins/cmd/peer/fleet_samples.go` -- register the configured/established peer counts
- `internal/component/bgp/plugins/rib/fleet_samples.go` -- register the owning route-count source
- `internal/component/l2tp/pppoe/cmd/fleet_samples.go` -- project live PPPoE access-concentrator session counts from its own snapshot
- `internal/component/l2tp/cmd/fleet_samples.go` -- register the active L2TP subscriber contribution
- `internal/component/managed/managed_sample_sources_test.go` -- check mixed builtin/forked sources and the absent-data behavior
- `pkg/fleet/sample.go` -- public frame definition, named figures and bounds
- `internal/component/managed/sample.go` -- registered figures and the managed sender
- `internal/component/plugin/server/managed_history.go` -- report receiver, rings, admission, checkpoint and retention
- `internal/component/managed/cmd_history.go` -- daemon query registrations
- `internal/component/plugin/server/managed_history_test.go` -- rate, rollup, persistence and sizing evidence
- `test/managed/fleet-history-reconnect.ci` -- disconnect and restart with retained latest values
- `test/managed/fleet-history-cap.ci` -- capacity pressure and registry removal

## Design presentation

### What changes

The hub can answer what a device last reported and how its readings changed over a week, including across a hub restart. Operators can rank devices, ask for a label group's traffic and see how much history fits their configured budget.

### Before and after

If edge-01 reports 80% CPU at 10:00 and disconnects until 11:00, latest reports 80% with its age. The series has no measurements during the outage. After reconnect a fresh sample can report 25%. Neither the chart nor the alarm evaluator may treat the offline hour as an hour at 80%.

### Alternatives set aside

Polling Prometheus endpoints from the hub would require inbound access to each device. Sending the full metrics registry would make storage depend on arbitrary label cardinality. The fixed set over the managed session bounds the report, while long-term and high-resolution monitoring remain available to operators through their existing Prometheus deployment.

### Documents departed from

None. The bounded history for fleet commands implements Thomas's 2026-11-18 exception to the old umbrella exclusion of external metrics aggregation.

### Choices for Thomas

Granting latest or series to a remote operator delegates its device scope to that login’s local profile; granting the storage command additionally reveals hub-wide capacity. Latest and series use command-specific pipes for narrowing and aggregation, including UTC bounds and a point budget on series. The proposed capacity policy trades older history for a hard byte ceiling. The proposed five-minute checkpoint trades crash recovery precision for fewer writes. The estimates above explain the starting defaults; these limits are proposals until the estate-size measurements and owner review.

## Implementation Steps

1. Wire one registered local producer through the managed session to the real hub query. Prove permission scoping, reconnect retention, actual series gaps and the absent-versus-zero distinction.
2. Implement fine/coarse aggregation, bounded reduction, sequence checks and reset-aware rates. Register each supported producer in its owning package and verify compiled-out producers supply no data.
3. Add checkpoint/restart, deletion fencing, capacity accounting and doctor coverage. Run the storage and estate sizing checks before finalizing defaults. Register shared commands and verify every command surface.
