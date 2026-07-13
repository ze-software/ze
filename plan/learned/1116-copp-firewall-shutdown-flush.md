# 1116 -- CoPP standalone apply + firewall clean-shutdown flush option

## Context
Follow-up triage from [[1112-netlink-ci-harness]]: the copp-* `.ci` suites
(`copp-bgp`, `copp-trusted`, `copp-withdraw`) never ran on Linux until the netns
harness landed, and their first real run exposed a chain of latent bugs -- none
caught by unit tests, all in the copp -> firewall-registry -> nft path.

## Decisions
- **Backend loads on demand.** A config with only `control-plane-protection`
  (no `firewall {}` block) never loaded the nft backend, so `firewall.ApplyAll`
  returned `errFirewallBackendNotLoaded` and copp's apply failed. Fix at the
  shared altitude: `ApplyAll` loads the OS-default backend
  (`defaultBackendForAutoload`, "nft" on Linux) on demand when a producer
  registers tables but no firewall section loaded one. Benefits every registry
  consumer (copp, policy-routes, ddos-local), not just copp. `LoadBackend` was
  split into `loadBackendLocked` so `ApplyAll` can load atomically under
  `backendsMu`.
- **Producers must carry the `ze_` ownership prefix.** The nft backend uses
  `firewall.Table.Name` verbatim (no prefix added) and only reconciles/deletes
  tables whose kernel name starts with `ze_`. The firewall engine adds the
  prefix in `ParseFirewallConfig` (`config.go: tableNamePrefix + name`);
  policy-routes hardcodes `ze_pr`. copp used bare `"copp"` -> kernel table
  `copp` -> invisible to `nft list table inet ze_copp` AND never withdrawn.
  Fix: `coppTableName = "ze_copp"`. **ddos-local has the same bug** (`tableName
  = "ddos-local"`, no prefix) -- a latent withdraw leak, not yet chased by a
  failing test.
- **Clean-shutdown table teardown is a firewall-engine responsibility, gated by
  a config option.** copp/firewall/policy-routes/ddos-local run in-process
  (`Internal: true`, set in `startup_autoload.go`) and share one `activeBackend`.
  copp's old post-`Run` withdraw raced the firewall engine's `CloseBackend`
  (`ProcessManager.Stop` cancels all plugins at once, no dependency ordering);
  once the backend closed, the table could not be removed (a fresh backend won't
  delete a `ze_*` table not in its own `applied` set). New `flush-on-shutdown`
  boolean leaf (default true) drives `firewall.FlushAllTables` from the engine's
  post-`Run` path -- a single ordered actor holding a live backend, sequential
  before `CloseBackend`, so no race. copp's post-`Run` withdraw was removed;
  config-removal-while-running still withdraws via `OnConfigApply ->
  applyCoppPolicy(nil)`.

## Consequences
- Default behaviour CHANGED: an orderly (SIGTERM) ze stop now removes all
  ze-owned nft tables (firewall + copp + policy-routes + ddos-local). Set
  `firewall { flush-on-shutdown false; }` to use ze as a one-shot provisioner
  (program the rules, exit, leave them running). This is process-lifecycle only
  and unrelated to BGP graceful restart (which never restarts the daemon).
- **Keep-on-crash is automatic**: the teardown only runs on the orderly-stop
  post-`Run` path; SIGKILL/panic/power-loss bypasses it, so tables persist. New
  `flush-crash.ci` (SIGKILL) and `flush-persist.ci` (option=false) lock this in.
- The three copp `.ci` suites plus the two flush tests are in the permanent
  `scripts/evidence/netns_qemu.py` host-safe subset: 20/20 green in QEMU, host
  nft byte-identical.

## Gotchas
- **nft 1.1.1 renders a MASKED source-address match RAW**: `ip saddr
  192.0.2.0/24` becomes `@nh,96,32 & 0xffffff00 == 0xc0000200` (0xc0000200 =
  192.0.2.0). Corrects [[1112-netlink-ci-harness]]'s claim that a literal
  `ip saddr` always renders cooked -- only an UNMASKED literal does. Version-
  robust `.ci` assertions must match either form (`192.0.2.0/24` or the hex).
- `atomic.Bool` zero value is false; `flushOnShutdown` needs a `func init()`
  `Store(true)` for its fail-safe default (with `//nolint:gochecknoinits`).
- The firewall engine parses `flush-on-shutdown` even when otherwise idle only
  if a `firewall` section is present; a copp-only config leaves the package
  default (true), which is why copp-withdraw passes with no firewall block.

## Sibling triage (verified, NOT yet fixed)
The other [[1112-netlink-ci-harness]] pre-existing failures, root-caused here for
a future decision -- none is a copp/firewall-flush regression:
- **004-cli-show**: `ze cli` authenticates to the daemon's SSH CLI server with
  credentials read from `{config.dir}/database.zefs`
  (`ssh/client/client.go: ReadCredentialsForRemote` -> `zefs.Open`). The client's
  `ResolveDBPath` hardcodes the zefs file and, unlike `core/resolve/resolve.go`
  (which falls back to the filesystem store when `ze.storage.blob=false`), does
  NOT honour that env. The functional runner forces `ze.storage.blob=false` for
  daemon configs (`runner_exec.go: zeDaemonShouldForceFileStorage`), so the
  daemon never writes a zefs db and the cli dies with `open database: ... no such
  file or directory`. Setting `ze.storage.blob=false` on the cli does NOT help --
  the ssh client ignores it. A real fix means making the ssh client resolve its
  credential store consistently with the storage backend (mirror the server's
  `storage.Storage` abstraction, ssh.go:387/533) AND ensuring the blob=false
  daemon writes login creds there -- security-sensitive, needs a design call.
- **policy suite / 009-set-element-timeout**: Alpine QEMU minimal-kernel limits,
  not code bugs. policy hits `firewallnft: flush: operation not supported`
  (reproduced identically with ze as root -> kernel nft-feature gap); 009 crashes
  the Alpine kernel on set-element-timeout ops. Both are expected green on a
  full-kernel host via `make ze-netns-test`; there is no per-test "needs full
  kernel" skip annotation (only `skip-os` and `needs-linux`), so netns_qemu.py
  just omits them from the host-safe subset.
