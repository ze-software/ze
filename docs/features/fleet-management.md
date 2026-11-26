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
<!-- source: pkg/fleet/ -- version hash, RPC envelope, and inventory/health report types -->
