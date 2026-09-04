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
- Config fetch with version hashing: clients only download when config changes
- Two-phase config change: hub notifies, client fetches when ready
- Partition resilience: clients cache config locally and start from cache when hub is unreachable
- Exponential backoff reconnect with jitter (1s to 60s cap)
- Heartbeat liveness detection (30s interval, 90s timeout)
- CLI overrides: `--server`, `--name`, `--token` flags for troubleshooting
- Managed mode toggle: `meta/instance/managed` blob flag controls hub connection

<!-- source: internal/component/managed/client.go -- RunManagedClient lifecycle -->
<!-- source: internal/component/managed/tls.go -- clientTLSConfig, the three trust anchors in order -->
<!-- source: internal/component/plugin/server/managed.go -- hub-side config handlers -->
<!-- source: internal/component/plugin/server/managed_serve.go -- managedCertificate, the leaf the hub serves -->
<!-- source: pkg/fleet/ -- version hash and RPC envelope types -->
