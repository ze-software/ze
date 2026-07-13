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
The other [[1112-netlink-ci-harness]] pre-existing failures -- none a
copp/firewall-flush regression:
- **004-cli-show (FIXED, NOT a client storage bug -- my first read was wrong)**:
  `ze cli` reads its SSH LOGIN creds from its OWN zefs store, written by
  `ze init`, independent of the daemon's config storage; `ze.storage.blob=false`
  is irrelevant to it. 004 failed only because the test never set up SSH auth,
  and four layers had to line up. (1) The netns QEMU daemon build lacked `ze_ssh`
  -- the SSH component's `system authentication` / `environment ssh` config
  schema is compiled in ONLY under `//go:build ze_ssh` (`all_ze_ssh.go`), absent
  from `ze_core zetest ze_distro`, so `ze_ssh` was added to the
  `ze-netns-qemu-test` daemon build (it is a default feature-gate in the real
  `bin/ze`; `ze init` is already in `ze_core`, so no `ze_setup` is needed --
  verified by building `ze_core ze_distro ze_ssh` and running the recipe).
  (2) The config needs a
  `system authentication user <bcrypt>` + an `environment ssh` server. (3) The
  driver provisions matching client creds with `ze init` into a sandbox
  `ze.config.dir`, then runs `ze cli --user ... -c` with `ze.ssh.password` +
  `ze.ssh.insecure`. (4) The firewall's OWN `policy drop` input chain silently
  dropped the loopback SSH RETURN traffic -- the SYN-ACK back to the client's
  ephemeral port (not 2222) -- so `ze cli` timed out with no auth logged; accept
  `input-interface lo` (covers both directions). Also the command is
  `show firewall ruleset <name>`; the old `show firewall <table>` never existed
  (004 had never run, so it was never caught). Recipe mirrors
  `test/plugin/ssh-user-login-yang.ci`. Green under netns QEMU (950ms).
- **policy suite / 009-set-element-timeout**: Alpine QEMU minimal-kernel limits,
  not code bugs. policy hits `firewallnft: flush: operation not supported`
  (reproduced identically with ze as root -> kernel nft-feature gap); 009 crashes
  the Alpine kernel on set-element-timeout ops. Both are expected green on a
  full-kernel host via `make ze-netns-test`; there is no per-test "needs full
  kernel" skip annotation (only `skip-os` and `needs-linux`), so netns_qemu.py
  just omits them from the host-safe subset.
