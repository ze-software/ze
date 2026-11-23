# Spec: fleet-8 -- Device Samples and Hub-Side Metric History

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | fleet, protocol |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (the hub's metric history of its devices) in the first release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-4-inventory-health.md |
| Phase | 8/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `plan/post-release/features/spec-fleet-0-umbrella.md`,
`plan/release/features/spec-fleet-4-inventory-health.md`, `internal/component/l2tp/cqm.go` (the only
bounded time series Ze has), `docs/architecture/fleet-config.md`.

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
| Verb | A new verb on the managed channel, device to hub, carrying one timestamped set of figures |
| Interval | One minute, configurable on the hub and told to the device at connect |
| While offline | The device does not buffer. The hub's history shows a gap, which is the truth |
| Size | A report is a few hundred bytes, far inside the frame bound of the channel |
| Rate | The hub refuses reports arriving faster than the interval allows and counts the refusals |
| Older devices | A device that does not send reports has no history; the queries answer that its software does not report |

### The history store

It extends the shape of the L2TP quality ring: fixed buckets, a pooled ring per
series, retention bounded by configuration.

| Property | Proposal |
|----------|----------|
| Resolutions | Fine: one bucket per minute, kept one day. Coarse: one bucket per fifteen minutes, kept seven days. A coarse bucket holds the average and the maximum of its fine buckets |
| Per series | One ring per device and figure |
| Persistence | Written to the hub's state store on a timer and at shutdown, reloaded at start. A hub that stops without notice loses at most one timer period |
| Budget | A stated maximum per device and for the whole store. When the store reaches its cap it shortens retention, oldest first, and raises the "History full" alarm; it never grows past the cap |
| Removal | A device removed from the registry has its history removed with it |
| Retention | Configurable, with the seven days as default and a stated maximum |

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
| Two resolutions | One resolution for the whole week | A week at one minute is ten times the storage for detail no week-long chart can show |
| Persisted on a timer | Written per sample; memory only | The owner chose persisted and bounded. A timer bounds the writes; memory only would empty every chart at each hub upgrade |
| Sums are computed at query time | Stored aggregate series per label | Labels change; stored aggregates would be wrong after a relabel |
| The cap shortens retention | Refusing new samples at the cap | Recent history is worth more than old history |

### Risks to resolve in design

| Risk | Question |
|------|----------|
| Store size | The per-device budget times the largest estate must fit the hub's store. The figures above must be measured, not assumed |
| Write load | A timer write of every ring must not stall the hub's other use of the store |
| Query cost | A sum over a thousand devices and a week must answer in bounded time |
| Clock difference | A device's clock may differ from the hub's. The hub stamps samples on arrival, or corrects by the offset measured at connect |
| Transport behaviour | The channel enforces a write deadline; a report that cannot be written is a disconnect, handled as one (`plan/post-release/features/spec-fleet-0-umbrella.md`, post-wave corrections) |
| Counter figures | Traffic is reported as a rate computed on the device, so a device restart cannot produce a false spike |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected device | Its figures arrive at the interval and are answered by the latest-samples query | A figure whose owner is absent is missing from the answer, because the device does not have it |
| AC-2 | A week of reports | The series query answers the fine resolution for the last day and the coarse for the last week | None |
| AC-3 | The hub restarts | The history is present after restart, less at most one timer period | None |
| AC-4 | A device offline for an hour | The series shows a gap for that hour | None |
| AC-5 | The store reaches its cap | Retention shortens, oldest first, the alarm is raised, and the store does not exceed the cap | Recent samples are always kept |
| AC-6 | A set query over the estate | The sum or average over the devices that report, with the count contributing | Devices that do not report are left out and counted as such |
| AC-7 | A device removed from the registry | Its history is gone | None |
| AC-8 | Reports faster than the interval allows | They are refused and counted; the store does not grow faster than configured | None |
| AC-9 | A standalone device | No reporting, no store, no overhead | None |
| AC-10 | Each query of the Queries table asked on the CLI, SSH, MCP, REST, gRPC and the web command line | Each surface lists the command and the daemon dispatcher serves it there (`TestEveryCommandOnEverySurface`, `cmd/ze/hub/surface_parity_test.go`); every surface's answer is the one the hub's handler gives the dispatcher. Added by the owner ruling of 2026-11-23 | None |
| AC-11 | A device that sends no reports, and a figure whose owner is absent on a reporting device | The latest-samples and series queries answer that the device's software does not report, or that the figure is absent, never a zero or an empty series read as data. Replaces the chart rules "its charts say so" (owner ruling of 2026-11-23) | None |

No criterion of this spec names a chart, a page or a web view. The charts over
this history are `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md`'s,
its history administration page is
`plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md`'s.
