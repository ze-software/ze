# Fleet Management

Ze supports centralized configuration for multi-node deployments. A central hub
serves configuration to remote ze instances over TLS.

- Named hub blocks: `server <name> { ip; port; secret; }` for listeners, `client <name> { host; port; secret; ca; }` for outbound
- Per-client secrets: each managed client authenticates with its own token
- Hub authentication by issuer: `ca <name>` names a `pki ca` entry holding the
  hub's certificate authority root, which the operator exports from the hub with
  `show pki local-ca pem`. The client validates the hub's chain against that root
  and against nothing else, so a hub that reissues its certificate stays
  reachable with no client change. A name that resolves to nothing is an error,
  never a fall-through to another anchor
- Hub device registry: `show fleet devices` lists live sessions, current configuration states and retained history, and `| label site=lon` filters the table by a declared label. `show fleet device name edge-01` includes fetch and acknowledgement details.
- Inventory and health reporting: managed clients push bounded hardware inventory (`inventory-report`, on connect and every 24 hours; excluding raw SMART readings and block-device serials) and component health (`health-report`, on connect and every 5 minutes; matching `/health`) to the hub. The hub persists the latest accepted snapshots across disconnect and restart and serves `show fleet device inventory name <name>`, `show fleet device health name <name>`, and `show fleet health` on every command surface.
- Device metric sampling and hub-side history: managed clients push a fixed set of 15 operational figures (`sample-report`) at the hub-configured `bus hub history interval` (default 60s). The hub retains a bounded dual-resolution store (1-minute buckets for 1 day, 15-minute buckets for up to 7 days with aligned per-minute peaks; default 1 MiB/device and 2 GiB/hub) persisted across restarts and serves caller-scoped `show fleet sample latest`, `show fleet sample series`, and `show fleet sample storage` on every command surface.
- Hub-derived fleet alarms and episode history: the hub derives open alarm occurrences across eight built-in kinds (`device-offline`, `component-down`, `component-degraded`, `config-diverged`, `config-rejected`, `resource-high`, `software-behind`, `history-full`) plus an open kind registration seam, tuned under `bus { hub { alarms { ... } } }` and persisted in bounded ZeFS shards (`meta/fleet/alarm/device/{name}`, `meta/fleet/alarm/hub`). Caller-scoped `show fleet alarm` and `show fleet alarm history device <name>` answer identically on every command surface with snapshot-bound continuation pagination and full authorized counts.
- Config fetch with version hashing: clients only download when config changes
- Two-phase config change: hub notifies, client fetches when ready
- Partition resilience: clients cache config locally and start from cache when hub is unreachable
- Exponential backoff reconnect with jitter (1s to 60s cap)
- Heartbeat liveness detection (30s interval, 90s timeout)
- CLI overrides: `--server`, `--name`, `--token` flags for troubleshooting
- Managed mode toggle: `meta/instance/managed` blob flag controls hub connection

<!-- source: internal/component/managed/client.go -- RunManagedClient lifecycle -->
<!-- source: internal/component/managed/tls.go -- clientTLSConfig, the three trust anchors in order -->
<!-- source: internal/component/managed/report.go -- startSessionReporter, ProjectFleetHealth -->
<!-- source: internal/component/host/fleet_inventory.go -- ProjectFleetInventory, collectFleetInventory -->
<!-- source: internal/component/plugin/server/managed.go -- hub-side config handlers -->
<!-- source: internal/component/plugin/server/managed_serve.go -- managedCertificate, the leaf the hub serves -->
<!-- source: internal/component/plugin/server/fleet_report.go -- handleReport, FleetHealthSummary -->
<!-- source: internal/component/managed/cmd_devices.go -- handleFleetDevices, handleFleetDevice -->
<!-- source: internal/component/managed/cmd_reports.go -- handleFleetDeviceInventory, handleFleetDeviceHealth, handleFleetHealth -->
<!-- source: internal/component/managed/cmd_history.go -- handleFleetSampleLatest, handleFleetSampleSeries, handleFleetSampleStorage -->
<!-- source: internal/component/managed/cmd_alarms.go -- handleFleetAlarm, handleFleetAlarmHistory -->
<!-- source: internal/component/managed/sample.go -- sessionSampler, FleetSampleMetrics -->
<!-- source: internal/component/plugin/server/managed_history.go -- ManagedHistoryStore, Ingest, QueryLatest, QuerySeries, QueryStorage -->
<!-- source: internal/component/plugin/server/managed_alarms.go -- ManagedAlarmStore, QueryOpen, QueryHistory -->
<!-- source: internal/component/plugin/server/managed_alarms_kinds.go -- AlarmKindRegistry, RegisterAlarmKind -->
<!-- source: internal/component/plugin/server/managed_alarms_store.go -- LoadFromStore, FlushOnce, FlushFinal -->
<!-- source: pkg/fleet/ -- version hash, RPC envelope, inventory/health report types, and sample vocabulary -->
