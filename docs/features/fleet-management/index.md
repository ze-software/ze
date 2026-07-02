# Fleet Management

Ze supports centralized configuration for multi-node deployments. A central hub
serves configuration to remote ze instances over TLS.

- Named hub blocks: `server <name> { ip; port; secret; }` for listeners, `client <name> { host; port; secret; }` for outbound
- Per-client secrets: each managed client authenticates with its own token
- Config fetch with version hashing: clients only download when config changes
- Two-phase config change: hub notifies, client fetches when ready
- Partition resilience: clients cache config locally and start from cache when hub is unreachable
- Exponential backoff reconnect with jitter (1s to 60s cap)
- Heartbeat liveness detection (30s interval, 90s timeout)
- CLI overrides: `--server`, `--name`, `--token` flags for troubleshooting
- Managed mode toggle: `meta/instance/managed` blob flag controls hub connection

<!-- source: internal/component/managed/client.go -- RunManagedClient lifecycle -->
<!-- source: internal/component/plugin/server/managed.go -- hub-side config handlers -->
<!-- source: pkg/fleet/ -- version hash and RPC envelope types -->
