# Fleet alarm conditions

The first-release alarm design covers the conditions a fleet hub can derive from device declarations, managed sessions, health reports and samples. The alarm list and history are proposed in Fleet UI-5. Fleet UI-8 adds same-device causes. Their commands are the first-release operator surface. [Alarm list design](../../plan/release/features/spec-fleet-ui-5-alarm-list.md) · [Alarm cause design](../../plan/release/features/spec-fleet-ui-8-alarm-causes.md)

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| For a declared device that has never connected, Thomas chose “After the configured offline grace period from declaration”. | Waiting for the first authenticated connection before starting the timer, or restarting the grace period at every hub restart or unrelated configuration reload | plan/release/features/spec-fleet-ui-5-alarm-list.md | 2026-11-25 |

## Configuration

Alarm delays, resource thresholds, software-behind threshold, and episode retention live under `bus { hub { alarms { ... } } }`. The hub extracts and validates these values into `plugin.HubAlarmsConfig`, applying YANG defaults whenever `bus { hub { ... } }` omits the `alarms` block.

| Leaf | Type | Range | Default | Purpose |
|------|------|-------|---------|---------|
| `offline-delay` | `uint32` (seconds) | `1..86400` | `300` | Grace delay after declaration or disconnect before opening `device-offline` |
| `resource-cpu-percent` | `uint8` (percent) | `1..100` | `90` | CPU usage threshold for `resource-high` |
| `resource-memory-percent` | `uint8` (percent) | `1..100` | `90` | Memory usage threshold for `resource-high` |
| `resource-storage-percent` | `uint8` (percent) | `1..100` | `90` | Storage usage threshold for `resource-high` |
| `resource-duration` | `uint32` (seconds) | `1..86400` | `300` | Sustained duration at or above threshold before opening `resource-high` |
| `software-behind-versions` | `uint16` | `1..1000` | `1` | Ordered versions behind the newest estate release before opening `software-behind` |
| `retention-days` | `uint8` (days) | `1..30` | `7` | Retention horizon for closed alarm episodes |
| `max-episodes-per-device` | `uint16` | `1..512` | `512` | Maximum closed alarm episodes retained per managed device |
| `max-episodes-hub` | `uint32` | `1..65536` | `65536` | Maximum closed alarm episodes retained across the hub |
<!-- source: internal/component/plugin/yang/ze-plugin-conf.yang -- container alarms -->
<!-- source: internal/component/plugin/types.go -- HubAlarmsConfig, DefaultHubAlarmsConfig, ValidateHubAlarmsConfig -->
<!-- source: internal/component/config/loader_extract.go -- ExtractHubConfig, extractHubAlarmsConfig -->

## Alarm kinds

An alarm is a derived condition true right now; nothing is acknowledged or silenced. Kinds register into an open hub registry so later features add a condition without editing a central list. Eight kinds ship in the first release; `relay-not-granted` joins through that same registration seam when `fleet-9` supplies relay capability and an explicit grant state (an unavailable provider or unknown grant opens no alarm).

| Kind | Scope | Severity | Opens when |
|------|-------|----------|------------|
| `device-offline` | Device | Critical | No managed connection for longer than `offline-delay` since declaration or last disconnect |
| `component-down` | Device | Critical | Latest device health report names a component down |
| `component-degraded` | Device | Warning | Latest device health report names a component degraded |
| `config-diverged` | Device | Warning | Device holds a locally edited configuration |
| `config-rejected` | Device | Warning | Device refused the configuration the hub sent |
| `resource-high` | Device | Warning | Fresh CPU, memory, or storage samples stay at or above threshold for `resource-duration` |
| `software-behind` | Device | Informational | Device runs a version at least `software-behind-versions` behind the newest ordered estate version |
| `history-full` | Hub | Warning | Metric history reached its size cap and is dropping samples early |
| `relay-not-granted` (registered by `fleet-9`) | Device | Informational | A relay-capable device reports an explicit absent grant |

## Device offline

Record the time the hub first accepts a device declaration. If no connection arrives before the configured delay expires, the hub opens Device offline at the end of that interval. Connecting before expiry cancels the pending opening. A device that has connected before uses its last disconnect as the start of the next interval. Removing a declaration ends its tracking. Re-adding the same name begins a new interval. A restart preserves the recorded start of a continuing declaration.

For a declaration already present when this facility is first enabled, begin timing at the first durable observation of that declaration. Its actual historical declaration time may be unknown. [Planned acceptance test](../../plan/release/features/spec-fleet-ui-5-alarm-list.md)

## Occurrence identity and reconnect semantics

Each alarm occurrence is identified by its registered kind, target device (or hub for `history-full`), subject (distinguishing two down components or CPU versus memory pressure on one device), and a monotonic opening identifier. Repeated reports of an already open condition preserve its identifier and opening time; only a false-to-true transition starts a new occurrence, and reading commands never synthesizes transitions.

Disconnect preserves the latest health report and metric sample with their original receipt times. An open `component-down` or `component-degraded` occurrence stays open across disconnect and keeps its opening time if the component is still unhealthy after reconnect. An open `resource-high` occurrence stays open while the device is offline, closes at reconnect, and requires fresh post-reconnect samples spanning `resource-duration` to open a new occurrence. Omitted resource figures or zero capacity totals reset pending qualification timers without closing an already-open `resource-high` occurrence. A known absent relay grant stays visible while offline.

Removing a device declaration closes its session- and telemetry-driven alarms (`device-offline`, `software-behind`, `component-down`, `component-degraded`, `resource-high`, and `config-rejected`) and clears pending resource qualification timers. `config-diverged` remains governed by whether the hub still stores a held configuration copy for that device.

## Storage keys and retention bounds

The hub persists declaration timing, active occurrence identity, sequence state, and bounded episode history in its borrowed daemon store under two private ZeFS key families. The writer checks encoded shard sizes against each key's `ValueMax` before `WriteKey` and bounds reads before decoding.

| Constant | Key pattern | `ValueMax` | Contents |
|----------|-------------|------------|----------|
| `KeyFleetAlarmDevice` (`FleetAlarmDevicePrefix`) | `meta/fleet/alarm/device/{name}` | 256 KiB (`FleetAlarmDeviceValueMax`) | Declaration timing, active occurrences, and bounded per-device closed episode history (JSON) |
| `KeyFleetAlarmHub` | `meta/fleet/alarm/hub` | 64 KiB (`FleetAlarmHubValueMax`) | Hub-scoped active occurrences, occurrence sequence state, and bounded hub closed episode history (JSON) |
<!-- source: pkg/zefs/keys.go -- FleetAlarmDeviceValueMax, FleetAlarmHubValueMax, FleetAlarmDevicePrefix, KeyFleetAlarmDevice, KeyFleetAlarmHub -->

Closed episodes are bounded by `retention-days` (default 7, max 30), `max-episodes-per-device` (default 512), and `max-episodes-hub` (default 65536). Under pressure, the oldest closed episode evicts first while active occurrence identities stay intact, and query answers expose the earliest retained time and truncation state.
<!-- source: internal/component/plugin/yang/ze-plugin-conf.yang -- container alarms -->

## Operator commands

Two read-only daemon commands publish the alarm model uniformly on CLI, SSH, REST, gRPC, MCP, and the web command surface. Both authorize caller device scope before filtering, counting, sorting, or issuing a continuation cursor, and both answer structured data compatible with generic pipe operations.

| Command | Wire command | Contract |
|---------|--------------|----------|
| `show fleet alarm` | `ze-managed:show-fleet-alarm` | Returns bounded pages (up to 1,024 rows) of open alarms with stable occurrence IDs, full authorized totals and severity/visibility counts, and a snapshot-bound continuation cursor |
| `show fleet alarm history device <name>` | `ze-managed:show-fleet-alarm-history` | Returns bounded pages (up to 512 rows) of one authorized device's alarm openings and clearings with earliest retained time, truncation flag, and continuation cursor |
<!-- source: internal/component/managed/yang/ze-managed-cmd.yang -- container alarm -->
