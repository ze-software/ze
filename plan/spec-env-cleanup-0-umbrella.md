# Spec: env-cleanup-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | 0 (umbrella) |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` — workflow rules
3. `.claude/rules/config-design.md` — env var registration rule
4. `.claude/rules/go-standards.md` — "Environment Variables: internal/core/env only" section
5. `internal/component/config/environment.go` — the parser being deleted
6. `internal/core/env/env.go` + `registry.go` — env registry semantics
7. `~/Code/github.com/exa-networks/exabgp/src/exabgp/environment/config.py` — ExaBGP upstream (authoritative list of keys we break compat with)

## Task

Purge the ExaBGP-compat env surface from Ze. Every env var registered today must be justified by a live consumer. Three categories: **delete** (dead, no consumer), **keep** (live, Ze-native purpose), **add** (replaces a deleted ExaBGP knob that operators genuinely need). At startup, warn loudly when legacy env vars are detected in `os.Environ` so migrating users notice instead of silently losing behaviour.

## Required Reading

### Architecture / Rules

- [ ] `.claude/rules/config-design.md` — env var / YANG pairing rule
  → Constraint: every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var
- [ ] `.claude/rules/go-standards.md` "Environment Variables" section
  → Constraint: all env access via `internal/core/env`, `Private`/`Secret` flags govern visibility
- [ ] `.claude/rules/compatibility.md`
  → Constraint: pre-release, no compat shims under `internal/`; plugin API is the only post-release frozen surface
- [ ] `.claude/rules/no-layering.md`
  → Constraint: delete the old before adding the new

### RFC Summaries

No RFC work in this spec.

**Key insights:**
- ExaBGP's authoritative env keys live in `~/Code/github.com/exa-networks/exabgp/src/exabgp/environment/config.py`. 10 sections, ~60 keys. Ze currently mirrors 8 sections but only two consumers in all of `Environment` struct fields.
- `env.MustRegister` silently overwrites on duplicate key (last-wins). `IsRegistered` consults wildcard prefix patterns stored at registration time, so `ze.bgp.<section>.<option>` makes every `ze.bgp.*` key pass the registration guard.
- Privilege drop (`internal/core/privilege/drop.go`) reads `ze.user`/`ze.group` directly, not `Environment.Daemon.User`.
- Slogutil (`slogutil.ApplyLogConfig`) maps the config `log { ... }` block straight to `ze.log.*` env vars, bypassing `Environment.Log` entirely.

## Current Behavior

**Source files read:** (must read BEFORE writing this spec)

- [ ] `internal/component/config/environment.go` — registrations, `Environment` struct, `envOptions` parser table, validators, LoadEnvironment* (904L)
- [ ] `internal/core/env/env.go` — `IsRegistered`, FATAL on unregistered, cache semantics (150L)
- [ ] `internal/core/env/registry.go` — `MustRegister`, wildcard prefix handling (77L)
- [ ] `internal/core/slogutil/slogutil.go` — `ApplyLogConfig`: config `log { ... }` → `ze.log.*` env vars (~500L)
- [ ] `internal/exabgp/migration/env.go` — ExaBGP INI → Ze config text converter (242L)
- [ ] `internal/component/bgp/config/loader_create.go` — two live `Environment.*` consumers (`TCP.Attempts → MaxSessions` at :188, `Debug.Pprof → startPprofServer` at :202)
- [ ] `cmd/ze/hub/main.go` — every `env.Get` in the hub startup path (ze.web.*, ze.mcp.*, ze.api-server.*, ze.looking-glass.*, ze.gokrazy.*, ze.ready.file)
- [ ] `internal/core/privilege/drop.go` — `ze.user`/`ze.group` consumer (the intended replacement for removed `ze.bgp.daemon.user`)
- [ ] `internal/component/bgp/reactor/reactor.go` — registers `ze.cache.safety.valve`, `ze.buf.*` (duplicated in environment.go)
- [ ] `internal/component/bgp/reactor/session_connection.go` — OPEN read path (new `ze.bgp.openwait` consumer)
- [ ] `internal/component/l2tp/config.go` — L2TP env registrations
- [ ] `pkg/plugin/sdk/sdk.go` — plugin SDK env registrations
- [ ] `cmd/ze/main.go` + `cmd/ze/internal/ssh/client/client.go` — duplicate `ze.config.dir` registration
- [ ] `~/Code/github.com/exa-networks/exabgp/src/exabgp/environment/config.py` — upstream ExaBGP env key inventory

**Behavior to preserve:**

- Every live env consumer keeps working: SSH server+client, managed mode, gokrazy proxy, DNS resolver, forward pool tuning, route-server tuning, L2TP subsystem, BFD test-parallel, plugin SDK auth, slogutil per-subsystem levels, report caps, ze-chaos binary orchestration, reactor speed/cache/update-grouping/chaos injection.
- `ze-test` peer tooling keeps working (its test BGP port override).
- `ze exabgp migrate` keeps consuming ExaBGP INI files and emits valid Ze config — but no longer emits config blocks that Ze would silently ignore.
- `slogutil.ApplyLogConfig` behaviour unchanged: `log { level X; destination Y; <subsystem> Z; }` continues to work.

**Behavior to change (user-authorised, 2026-04-18):**

- `bgp.openwait` becomes a live Ze knob under `ze.bgp.openwait`, wired to OPEN read deadline.
- `daemon.pid` becomes a live Ze knob under `ze.pid.file`, written at hub startup, removed at shutdown.
- `tcp.delay` becomes a live Ze knob under `ze.bgp.announce.delay`, gating first UPDATE at reactor startup.
- `daemon.user` is deleted — migrating users are pointed to `ze.user` via warning log.
- `daemon.umask` is deleted — documented replacement: `UMask=` in systemd unit file.
- Every other ExaBGP-compat registration and YANG leaf is deleted outright.
- At hub startup, scan `os.Environ` for legacy keys and log one WARN per hit.

## Full Env Var Inventory (REVIEW TABLE)

Notation:

- **Action** — `keep`, `rename` (keep consumer, change key name), `delete` (remove registration + YANG + struct field + setter + add to legacy warner), `add` (new key), `dedup` (remove duplicate registration).
- **Wired?** — does a production consumer `env.Get*` it today?

### `ze.bgp.*` — ExaBGP compat surface (targeted for elimination)

| Env key | YANG leaf | Wired? | Action | New name / notes |
|---|---|---|---|---|
| `ze.bgp.<section>.<option>` | — (wildcard) | N/A | delete | Wildcard defeats registration guard |
| `ze.bgp.daemon.pid` | `environment/daemon/pid` | no | delete, add `ze.pid.file` | Hub writes PID file on start, removes on shutdown |
| `ze.bgp.daemon.user` | `environment/daemon/user` | no (migration emits config, Ze ignores) | delete | Covered by `ze.user` (`privilege/drop.go`). Migration comment: `# daemon.user=X -- set ZE_USER=X` |
| `ze.bgp.daemon.daemonize` | `environment/daemon/daemonize` | no | delete | Ze doesn't fork (systemd/gokrazy own the process) |
| `ze.bgp.daemon.drop` | `environment/daemon/drop` | no | delete | Implicit in Ze (drops iff `ze.user` set) |
| `ze.bgp.daemon.umask` | `environment/daemon/umask` | no | delete | Documented replacement: `UMask=` in systemd unit file |
| `ze.bgp.log.level` | `environment/log/level` | no (slogutil consumes `ze.log`) | delete | Slogutil already maps `log { level X; }` → `ze.log` |
| `ze.bgp.log.destination` | `environment/log/destination` | no (slogutil consumes `ze.log.destination`) | delete | Slogutil owns this mapping |
| `ze.bgp.log.short` | `environment/log/short` | no | delete | Ze's slog formatter has no short-mode switch |
| `ze.bgp.tcp.attempts` | `environment/tcp/attempts` | YES → `MaxSessions` (loader_create.go:188) | rename → `ze.bgp.attempts` (YANG `environment/bgp/attempts`) | Knob stays, name loses ExaBGP `tcp.*` scope |
| `ze.bgp.tcp.delay` | `environment/tcp/delay` | no | delete, add `ze.bgp.announce.delay` | New Ze-native knob, staged announcement gate |
| `ze.bgp.tcp.acl` | `environment/tcp/acl` | no (upstream marks "experimental, unimplemented") | delete | No consumer upstream or in Ze |
| `ze.bgp.tcp.port` | — (env-only) | YES (test infra only) | rename → `ze.test.bgp.port` (Private) | Name makes test-only scope obvious |
| `ze.bgp.bgp.openwait` | `environment/bgp/openwait` | no (OpenWaitDuration never called) | delete, add `ze.bgp.openwait` | New Ze-native knob wired to OPEN read deadline |
| `ze.bgp.cache.attributes` | `environment/cache/attributes` | no | delete | Ze always caches (pool dedup is architectural) |
| `ze.bgp.reactor.speed` | `environment/reactor/speed` | YES → `Reactor.Speed` | keep | Ze-native reactor tuning |
| `ze.bgp.reactor.cache-ttl` | `environment/reactor/cache-ttl` | YES | keep | Ze UPDATE cache TTL |
| `ze.bgp.reactor.cache-max` | `environment/reactor/cache-max` | YES | keep | Ze UPDATE cache max entries |
| `ze.bgp.reactor.update-groups` | `environment/reactor/update-groups` | YES → `update_group.go:65` | keep | Cross-peer UPDATE grouping switch |
| `ze.bgp.chaos.seed` | `environment/chaos/seed` | YES → `childmode.go:193` | keep | Ze-native chaos injection |
| `ze.bgp.chaos.rate` | `environment/chaos/rate` | YES → `childmode.go:194` | keep | Same |
| `ze.bgp.api.ack` | `environment/api/ack` | no | delete | ExaBGP `api.ack` semantic lives at the bridge (`exabgp.api.ack`, Phase 7) — the Ze-scope fork was never consumed. Unrelated to live `bgp plugin ack sync\|async` CLI (announce-before-wire mode) |
| `ze.bgp.api.chunk` | `environment/api/chunk` | no | delete | Ze's API paginates differently |
| `ze.bgp.api.encoder` | `environment/api/encoder` | no | delete | Ze is JSON-only |
| `ze.bgp.api.compact` | — (env-only) | no | delete | Ze's JSON format is its own |
| `ze.bgp.api.respawn` | `environment/api/respawn` | no | delete | Ze's plugin lifecycle is different |
| `ze.bgp.api.terminate` | — (env-only) | no | delete | Same |
| `ze.bgp.api.cli` | `environment/api/cli` | no | delete | Ze uses SSH CLI, not named pipes |
| `ze.bgp.debug.pprof` | `environment/debug/pprof` | YES → `startPprofServer` (loader_create.go:202) | rename → `ze.pprof`, YANG `environment/pprof` | Pprof is Go-wide, not BGP-specific |
| `ze.bgp.debug.pdb` | `environment/debug/pdb` | no | delete | Python debugger, N/A in Go |
| `ze.bgp.debug.memory` | `environment/debug/memory` | no | delete | Go has `go tool pprof` |
| `ze.bgp.debug.configuration` | `environment/debug/configuration` | no | delete | Ze default is fail-fast on config errors |
| `ze.bgp.debug.selfcheck` | `environment/debug/selfcheck` | no | delete | Upstream unused (0 refs) |
| `ze.bgp.debug.route` | `environment/debug/route` | no | delete | Upstream unused (0 refs) |
| `ze.bgp.debug.defensive` | `environment/debug/defensive` | no | delete | Replaced by `ze.bgp.chaos.*` |
| `ze.bgp.debug.rotate` | `environment/debug/rotate` | no | delete | Ze has commit/rollback |
| `ze.bgp.debug.timing` | `environment/debug/timing` | no | delete | Slog + `ze.metrics.interval` cover timing |

### `ze.log.*` — logging (slogutil owns)

| Env key | YANG leaf | Wired? | Action | Notes |
|---|---|---|---|---|
| `ze.log` | — (from `log { level X; }`) | YES — slogutil base level | keep | |
| `ze.log.<subsystem>` | `environment/log/<unknown>` via `ze:allow-unknown-fields` | YES — per-subsystem level | keep | Load-bearing wildcard |
| `ze.log.backend` | `environment/log/backend` | YES → slogutil | keep | stderr/stdout/syslog |
| `ze.log.destination` | `environment/log/destination` | YES → slogutil | keep | Syslog host or filename |
| `ze.log.relay` | `environment/log/relay` | YES → plugin stderr relay | keep | |
| `ze.log.color` | — (env-only, CLI control) | YES | keep | |
| `ze.log.l2tp` | — (redundant with wildcard) | YES | dedup | Delete explicit entry; wildcard covers it |

### Reactor + session tuning (`ze.fwd.*`, `ze.rs.*`, `ze.buf.*`, `ze.cache.*`, `ze.metrics.*`)

| Env key | Wired? | Action | Notes |
|---|---|---|---|
| `ze.fwd.chan.size` | YES → `reactor.go:333` | keep | |
| `ze.fwd.write.deadline` | YES → `forward_pool.go:82` | keep | |
| `ze.fwd.pool.size` | YES → `reactor.go:337` | keep | |
| `ze.fwd.pool.maxbytes` | YES → `reactor.go:345` | keep | |
| `ze.fwd.batch.limit` | YES → `reactor.go:340` | keep | |
| `ze.fwd.teardown.grace` | YES → `reactor.go:407` | keep | |
| `ze.fwd.pool.headroom` | YES → `reactor.go:381` | keep | |
| `ze.rs.chan.size` | YES → `rs/server.go:194` | keep | |
| `ze.rs.fwd.senders` | YES → `rs/server.go:396` | keep | |
| `ze.buf.read.size` | YES → `session_connection.go:268` | dedup | Registered in env.go AND reactor.go; keep reactor.go |
| `ze.buf.write.size` | YES → `session_connection.go:269` | dedup | Same |
| `ze.cache.safety.valve` | YES → `reactor.go:470` | dedup | Same |
| `ze.metrics.interval` | YES → `reactor_metrics.go:20` | keep | |

### Protocol subsystems (`ze.l2tp.*`, `ze.bfd.*`)

| Env key | Wired? | Action | Notes |
|---|---|---|---|
| `ze.l2tp.enabled` | **no** | delete | Registered in `config.go:27` with a comment claiming it overrides YANG, but `ExtractParameters` reads only `tree.GetContainer("l2tp").Get("enabled")` — no `env.Get` call. Set the YANG leaf; the env var is a phantom knob. |
| `ze.l2tp.hello-interval` | **no** | delete | Same phantom pattern — config tree is the only source |
| `ze.l2tp.max-sessions` | **no** | delete | Same |
| `ze.l2tp.max-tunnels` | **no** | delete | Same |
| `ze.l2tp.shared-secret` | **no** | delete | Same. If a user sets `ZE_L2TP_SHARED_SECRET`, nothing reads it — silent auth failure. Legacy warner must flag this one loudly. |
| `ze.l2tp.skip-kernel-probe` | YES → `subsystem.go:137` | keep (Private) | Test-only bypass |
| `ze.l2tp.auth.timeout` | YES → `reactor.go:791` | keep | PPP auth phase timeout |
| `ze.l2tp.auth.reauth-interval` | YES → `reactor.go:807` | keep | PPP periodic re-auth |
| `ze.l2tp.ncp.enable-ipcp` | YES → `reactor.go:812` | keep | |
| `ze.l2tp.ncp.enable-ipv6cp` | YES → `reactor.go:813` | keep | |
| `ze.l2tp.ncp.ip-timeout` | YES → `reactor.go:815` | keep | |
| `ze.bfd.test-parallel` | YES → `bfd/transport/udp_linux.go:54` | keep (Private) | Test-only SO_REUSEPORT |

Note: the five phantom L2TP knobs (`enabled`, `hello-interval`, `max-sessions`, `max-tunnels`, `shared-secret`) were audited by reading `internal/component/l2tp/config.go` `ExtractParameters` — it reads the YANG tree only, no `env.Get` call. Deleting the registrations restores honesty to `ze env list` output and prevents the silent-auth-fail case where a user sets `ZE_L2TP_SHARED_SECRET` and sees nothing happen.

### DNS / report / plugin infrastructure

| Env key | Wired? | Action |
|---|---|---|
| `ze.dns.server` | YES | keep |
| `ze.dns.timeout` | YES | keep |
| `ze.dns.cache-size` | YES | keep |
| `ze.dns.cache-ttl` | YES | keep |
| `ze.report.warnings.max` | YES → `report.go:193` | keep |
| `ze.report.errors.max` | YES → `report.go:194` | keep |
| `ze.plugin.hub.host` | YES → plugin SDK | keep |
| `ze.plugin.hub.port` | YES → plugin SDK | keep |
| `ze.plugin.hub.token` | YES (Private+Secret) | keep |
| `ze.plugin.cert.fp` | YES | keep |
| `ze.plugin.name` | YES → `cli.go:30` | keep |
| `ze.plugin.stage.timeout` | YES → `server.go:46` | keep |
| `ze.plugin.delivery.timeout` | YES → `delivery.go:97` | keep |

### Service endpoints + SSH

| Env key | Wired? | Action |
|---|---|---|
| `ze.web.enabled` | YES | keep |
| `ze.web.listen` | YES | keep |
| `ze.web.insecure` | YES | keep |
| `ze.mcp.enabled` | YES | keep |
| `ze.mcp.listen` | YES | keep |
| `ze.mcp.token` | YES (Secret) | keep |
| `ze.api-server.rest.enabled` | YES | keep |
| `ze.api-server.rest.listen` | YES | keep |
| `ze.api-server.grpc.enabled` | YES | keep |
| `ze.api-server.grpc.listen` | YES | keep |
| `ze.api-server.token` | YES (Secret) | keep |
| `ze.looking-glass.enabled` | YES | keep |
| `ze.looking-glass.listen` | YES | keep |
| `ze.looking-glass.tls` | YES | keep |
| `ze.gokrazy.enabled` | YES | keep |
| `ze.gokrazy.socket` | YES | keep |
| `ze.ssh.ephemeral` | YES → `infra_setup.go:67` | keep |
| `ze.ssh.host` | YES → ssh client | keep |
| `ze.ssh.port` | YES → ssh client | keep |
| `ze.ssh.username` | YES → ssh client | keep |
| `ze.ssh.password` | YES (Secret) → ssh client | keep |

### Managed mode, infra, `ze.chaos.*` binary

| Env key | Wired? | Action |
|---|---|---|
| `ze.managed.server` | YES | keep |
| `ze.managed.name` | YES | keep |
| `ze.managed.token` | YES (verify Secret flag) | keep |
| `ze.managed.connect.timeout` | YES | keep |
| `ze.child.mode` | YES → `childmode.go:41` | keep (Private) |
| `ze.config.dir` | YES | dedup — keep main.go, drop ssh client copy |
| `ze.storage.blob` | YES | keep |
| `ze.ready.file` | YES → `hub/main.go:628` | keep |
| `ze.user` | YES → `privilege/drop.go:35` | keep |
| `ze.group` | YES → `privilege/drop.go:36` | keep |
| `ze.chaos.bgp.port` | YES → `ze-chaos/main.go:92` | keep |
| `ze.chaos.listen.base` | YES | keep |
| `ze.chaos.ssh.port` | YES | keep |
| `ze.chaos.web.ui.port` | YES | keep |
| `ze.chaos.lg.port` | YES | keep |
| `ze.chaos.web` | YES | keep |
| `ze.chaos.metrics` | YES | keep |
| `ze.chaos.pprof` | YES | keep |

### New keys to add

| New env key | YANG leaf | Consumer | Notes |
|---|---|---|---|
| `ze.bgp.openwait` | `environment/bgp/openwait` (new, BGP augment) | `session_connection.go` OPEN read deadline | Default 120s, range 1-3600 |
| `ze.bgp.announce.delay` | `environment/bgp/announce-delay` (new) | Reactor startup gate before first UPDATE | Default 0s (no delay) |
| `ze.bgp.attempts` | `environment/bgp/attempts` (renamed from `tcp/attempts`) | `loader_create.go:188` → `MaxSessions` | Default 0 (unlimited) |
| `ze.pid.file` | — (env-only) | Hub startup writes PID, shutdown removes | Default empty (no PID file) |
| `ze.pprof` | `environment/pprof` (moved from `debug/pprof`) | `loader_create.go:202` → `startPprofServer` | Default empty, format `addr:port` |
| `ze.test.bgp.port` | — (env-only, Private) | `ze-test peer`, `peers.go:585` | Renamed from `ze.bgp.tcp.port`, default 179 |
| `exabgp.api.ack` | — (env-only, Private) | `internal/exabgp/bridge/` — emits `done\n`/`error ...\n` to plugin stdin on `true` | Default `true` (matches ExaBGP upstream default). Registered in bridge package. Keeps upstream name so user's existing env/INI file works unchanged. |

## Data Flow

### Entry Point

- OS environment variables set by user shell, systemd unit file, or container runtime.
- `exabgp.env` INI file parsed by `ze exabgp migrate` → Ze config text on disk.

### Transformation Path

1. User sets env var (e.g. `ZE_BGP_OPENWAIT=60`).
2. Binary starts; `init()` runs; every registration call in the binary populates the central registry.
3. Hub startup calls `LogLegacyEnvWarnings(logger)` — each legacy key present in `os.Environ()` gets one WARN line.
4. Consumer calls `env.GetDuration("ze.bgp.openwait", 120*time.Second)` at first use; `env.Get` aborts FATAL on unregistered keys.
5. Value reaches `session_connection.go` OPEN read deadline; session enforces timeout.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Shell → process | `os.Environ()` snapshot | env.Get cache-rebuild test |
| env package → consumer | `env.GetDuration` / `env.GetInt` / `env.Get` | FATAL-on-unregistered test |
| Config file → env | `slogutil.ApplyLogConfig(configValues)` | existing slogutil test suite |
| ExaBGP INI → Ze config text | `internal/exabgp/migration/env.go` `mapEnvKnownKey` | migration `.ci` tests |

### Integration Points

- `internal/core/env` registry — every binary's `init()` populates it; `LogLegacyEnvWarnings` reads from it.
- `internal/core/slogutil/slogutil.go` `ApplyLogConfig` — untouched by this spec; continues to consume `log { ... }` config.
- `internal/component/bgp/config/loader_create.go` — changes from reading `Environment.TCP.Attempts` / `Environment.Debug.Pprof` to direct `env.GetInt` / `env.Get` calls on the renamed keys.
- `cmd/ze/hub/main.go` — new PID file writer + legacy env warner hook at startup.

### Architectural Verification

- [ ] No bypassed layers — every consumer goes through `env.Get*`
- [ ] No unintended coupling — `internal/component/config/environment.go` trims to `ParseCompoundListen`/`ListenEndpoint`/`ResolveConfigPath`/`DefaultSocketPath` only
- [ ] No duplicated functionality — `Environment` struct, `envOptions` table, validators disappear

## Wiring Test

| Entry Point | → | Feature Code | Test |
|---|---|---|---|
| `ZE_BGP_OPENWAIT=N` at startup | → | OPEN read timeout in `session_connection.go` | `test/plugin/openwait-timeout.ci` |
| `ZE_PID_FILE=/path` at startup | → | PID file writer in `cmd/ze/hub/main.go` | `test/parse/pid-file.ci` |
| `ZE_BGP_ANNOUNCE_DELAY=N` at reactor ready | → | Startup gate in `reactor.go` | `test/plugin/announce-delay.ci` |
| Legacy env var in `os.Environ()` | → | `LogLegacyEnvWarnings` | `test/parse/legacy-env-warning.ci` |
| `ZE_BGP_ATTEMPTS=3` | → | `MaxSessions` in `loader_create.go` | `test/parse/attempts-override.ci` |
| `ZE_TEST_BGP_PORT=1179` | → | Peer port override in `peers.go:585` | existing `ze-test peer` tests |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|---|---|---|
| AC-1 | Every row marked `delete` in the inventory | Registration, YANG leaf, `Environment.*` field, and `envOptions[...]` setter row all removed |
| AC-2 | `ze.bgp.<section>.<option>` wildcard removed | `env.Get("ze.bgp.typo")` aborts with FATAL (registration guard restored) |
| AC-3 | Every row marked `rename` | Old key no longer registered; new key registered; consumer reads new key; legacy warner fires on old |
| AC-4 | Every row marked `add` | New key registered; consumer reads it; `.ci` test proves behaviour |
| AC-5 | Every row marked `dedup` | Key registered in exactly one file |
| AC-6 | Hub startup with `EXABGP_BGP_OPENWAIT=60` in env | WARN log line naming the key and pointing to `ze exabgp migrate` |
| AC-7 | Hub startup with `ZE_BGP_DAEMON_USER=root` in env | WARN log line pointing to `ZE_USER` |
| AC-8 | Unit test scans `legacyEnvKeys` table | Every entry returns `false` from `env.IsRegistered` |
| AC-9 | `ze exabgp migrate` on ExaBGP INI with `daemon.user = bgp` | Output contains `# daemon.user = bgp -- set ZE_USER=bgp` (comment line, not a parsed Ze block) |
| AC-10 | `ze exabgp migrate` on ExaBGP INI with `bgp.openwait = 60` | Output contains `# bgp.openwait = 60 -- set ZE_BGP_OPENWAIT=60` |
| AC-10b | Bridge started with default env (no `exabgp.api.ack` set) | Bridge emits `done\n` on plugin stdin after each successful command dispatch |
| AC-10c | Bridge started with `EXABGP_API_ACK=false` | Bridge emits zero ack lines on plugin stdin |
| AC-10d | Bridge with ack on and Ze returns `error` for a command | Bridge emits `error ...\n` on plugin stdin with Ze's error text |
| AC-11 | `ZE_BGP_OPENWAIT=2` at peer startup, peer never sends OPEN | Session transitions to Idle after ~2s |
| AC-12 | `ZE_PID_FILE=/tmp-safe/ze.pid` at hub startup | File exists with PID; removed at clean shutdown |
| AC-13 | `ZE_BGP_ANNOUNCE_DELAY=5s` at reactor startup | First UPDATE emitted at least 5s after reactor Ready |
| AC-14 | `grep MustRegister internal/component/config/environment.go` after cleanup | Zero hits |
| AC-15 | `make ze-verify-fast` | Passes |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLegacyEnvKeysNotRegistered` | `internal/component/config/legacy_env_test.go` | Every entry in `legacyEnvKeys` is NOT in the live registry (prevents re-registering something we just removed) | new |
| `TestLogLegacyEnvWarningsFires` | `internal/component/config/legacy_env_test.go` | Given `t.Setenv("EXABGP_BGP_OPENWAIT", "60")`, warner logs exactly one WARN line with the key | new |
| `TestLogLegacyEnvWarningsQuiet` | `internal/component/config/legacy_env_test.go` | Given a clean env, warner logs zero lines | new |
| `TestNoWildcardRegistered` | `internal/core/env/registry_test.go` | After cleanup, `prefixes` slice contains only `ze.log.` (the subsystem wildcard); `ze.bgp.` is absent | new |
| `TestEnvironmentStructDeleted` | build check | `internal/component/config/environment.go` compiles without `Environment`, `envOptions`, `LoadEnvironment`, validators | build |
| `TestOpenWaitEnvWired` | `internal/component/bgp/reactor/session_connection_test.go` | `env.GetDuration("ze.bgp.openwait", ...)` is read by the OPEN read path | new |
| `TestAnnounceDelayEnvWired` | `internal/component/bgp/reactor/reactor_test.go` | Startup gate honours `ze.bgp.announce.delay` | new |
| `TestPIDFileWriteRemove` | `cmd/ze/hub/pidfile_test.go` | `ZE_PID_FILE` causes write on start, remove on shutdown | new |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.bgp.openwait` | 1-3600 seconds | 3600 | 0 | 3601 |
| `ze.bgp.announce.delay` | 0-3600 seconds (0 = no delay) | 3600 | -1 | 3601 |
| `ze.bgp.attempts` | 0-1000 (0 = unlimited) | 1000 | -1 | 1001 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `openwait-timeout.ci` | `test/plugin/openwait-timeout.ci` | Start ze with `ZE_BGP_OPENWAIT=2`; connect a sink that accepts TCP but never sends OPEN; session transitions Idle within ~2s | new |
| `pid-file.ci` | `test/parse/pid-file.ci` | Start ze with `ZE_PID_FILE=tmp/test/ze.pid`; assert file exists with valid PID; trigger shutdown; assert file removed | new |
| `announce-delay.ci` | `test/plugin/announce-delay.ci` | Start ze with `ZE_BGP_ANNOUNCE_DELAY=3s`; assert first UPDATE emitted ≥3s after reactor Ready | new |
| `attempts-override.ci` | `test/parse/attempts-override.ci` | Start ze with `ZE_BGP_ATTEMPTS=3`; connect failing peer 3 times; assert peer goes to permanent Idle | new |
| `legacy-env-warning.ci` | `test/parse/legacy-env-warning.ci` | Start ze with `EXABGP_BGP_OPENWAIT=60` and `ZE_BGP_DAEMON_USER=root`; assert stderr has WARN lines naming both keys with the Ze replacement advice | new |
| `migration-output.ci` | `test/exabgp/migration-env-comments.ci` | Feed ExaBGP INI with `daemon.user = bgp` and `bgp.openwait = 60`; assert migrated config contains the two expected comment lines | new |
| `bridge-ack-default.ci` | `test/exabgp/bridge-ack-default.ci` | Launch `ze exabgp plugin` with a helper that writes `neighbor X announce route Y` then `readline()` on stdin; assert helper reads `done\n` within 1s | new |
| `bridge-ack-disabled.ci` | `test/exabgp/bridge-ack-disabled.ci` | Launch `ze exabgp plugin` with `EXABGP_API_ACK=false`; helper writes commands and never reads stdin (except for events); assert no ack lines appear on stdin and bridge does not deadlock over N commands | new |

### Future (deferred tests)

None. Every AC has a test.

## Files to Modify

| File | Action |
|------|--------|
| `internal/component/config/environment.go` | Trim ~800 lines; keep `ParseCompoundListen` / `ListenEndpoint` / `ResolveConfigPath` / `DefaultSocketPath` only (file stays for compound-listen helpers) |
| `internal/component/config/environment_test.go` | Drop table rows for deleted options |
| `internal/component/config/legacy_env.go` | NEW — `LogLegacyEnvWarnings` + `legacyEnvKeys` map |
| `internal/component/config/legacy_env_test.go` | NEW |
| `internal/component/hub/schema/ze-hub-conf.yang` | Delete `environment/{daemon,api,debug,log/short,cache}` leaves; add `environment/pprof` |
| `internal/component/bgp/schema/ze-bgp-conf.yang` | Delete `environment/{tcp,cache,bgp/openwait}` augments; add `environment/bgp/{attempts,openwait,announce-delay}` |
| `internal/component/bgp/config/loader_create.go` | Replace `env.TCP.Attempts` + `env.Debug.Pprof` with direct `env.GetInt("ze.bgp.attempts", 0)` + `env.Get("ze.pprof")` |
| `internal/component/bgp/reactor/session_connection.go` | OPEN read deadline driven by `env.GetDuration("ze.bgp.openwait", 120*time.Second)` |
| `internal/component/bgp/reactor/reactor.go` | Staged announcement gate driven by `env.GetDuration("ze.bgp.announce.delay", 0)`; remove duplicate `ze.cache.safety.valve`, `ze.buf.*` registrations |
| `cmd/ze/hub/main.go` | PID file write/remove; call `LogLegacyEnvWarnings` at startup |
| `cmd/ze/hub/pidfile.go` | NEW — small writer/remover helpers |
| `cmd/ze-test/peer.go` | Rename `ze.bgp.tcp.port` → `ze.test.bgp.port` |
| `internal/component/bgp/config/peers.go` | Same rename in consumer |
| `internal/component/bgp/config/loader_create.go` | Same rename in registration |
| `internal/core/slogutil/slogutil.go` | Deregister explicit `ze.log.l2tp` (wildcard covers it) |
| `cmd/ze/main.go` | Keep `ze.config.dir` registration here |
| `cmd/ze/internal/ssh/client/client.go` | Remove duplicate `ze.config.dir` registration |
| `internal/exabgp/migration/env.go` | Rewrite `mapEnvKnownKey` cases to emit `# comment` lines with Ze-native replacement advice; `api.ack` case emits confirming comment pointing to bridge behaviour (no injection needed) |
| `internal/exabgp/migration/env_test.go` | Update expected outputs |
| `internal/exabgp/bridge/bridge.go` + new `bridge_ack.go` | Register `exabgp.api.ack` (Private, default true); extend `pendingResponses` to track every dispatched command; emit `done\n`/`error ...\n` to plugin stdin on Ze response when ack mode is on |
| `internal/exabgp/bridge/bridge_muxconn.go` | Extend `pendingResponses` API: `register(id, ackCh)` where ackCh receives the success/error text, or `registerFlush(id)` (existing no-payload path) |
| `test/exabgp/*.ci` | Update any tests that assert on old migration output; add the two new bridge-ack tests |
| `docs/guide/environment-variables.md` | NEW — authoritative env var reference |
| `docs/guide/exabgp-migration.md` | Add "env var changes" section |

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/bgp/schema/ze-bgp-conf.yang`, `internal/component/hub/schema/ze-hub-conf.yang` |
| CLI commands/flags | No | unchanged |
| Editor autocomplete | Yes (automatic) | YANG-driven |
| Functional tests | Yes | `test/plugin/*.ci`, `test/parse/*.ci`, `test/exabgp/*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — PID file, announce delay |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/environment-variables.md` (NEW) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — ExaBGP column footnotes |
| 12 | Affects architecture doc? | Yes | `docs/architecture/core-design.md` |
| 13 | Route metadata? | No | |

## Implementation Steps

Phases 3–5 can run in parallel once Phase 1 lands.

| Phase | Child spec | Scope | Prereq |
|---|---|---|---|
| 1 | `spec-env-cleanup-1-delete.md` | Delete every `delete`/`dedup` row. Remove `Environment` struct, `envOptions`, `LoadEnvironment*`, validators. Rename `ze.bgp.tcp.attempts`/`ze.bgp.debug.pprof`/`ze.bgp.tcp.port`. Remove wildcard. Remove YANG leaves. Update `environment_test.go` | none |
| 2 | `spec-env-cleanup-2-warner.md` | Add `LogLegacyEnvWarnings` + tests; wire into hub startup | Phase 1 |
| 3 | `spec-env-cleanup-3-openwait.md` | Add `ze.bgp.openwait` + YANG leaf; wire OPEN read deadline; `.ci` test | Phase 1 |
| 4 | `spec-env-cleanup-4-pidfile.md` | Add `ze.pid.file`; write/remove PID in hub lifecycle; `.ci` test | Phase 1 |
| 5 | `spec-env-cleanup-5-announce-delay.md` | Add `ze.bgp.announce.delay` + YANG leaf; reactor startup gate; `.ci` test | Phase 1 |
| 6 | `spec-env-cleanup-6-migration.md` | Update `internal/exabgp/migration/env.go` emit rules + migration `.ci` tests | Phases 3, 4, 5 |
| 7 | `spec-env-cleanup-7-bridge-ack.md` | Register `exabgp.api.ack` (Private, default true) in `internal/exabgp/bridge/`. Extend `pendingResponses` to track every dispatched command (not just flushes). In `zebgpToPluginWithScanner`, route `ok`/`error` responses to emit `done\n`/`error ...\n` on plugin stdin when ack mode is on. Two `.ci` tests: (a) default mode — helper does `readline()` after announce and receives `done`; (b) `EXABGP_API_ACK=false` — asserts zero ack lines emitted. Legacy env warner exempts `exabgp.api.ack` from the warn-list. | Phase 1 |

## Checklist

Per-phase:

- [ ] Tests written (unit + functional, per phase's ACs)
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `make ze-test` clean for the touched package
- [ ] `make ze-verify-fast` passes
- [ ] Spec file updated with deferrals (if any)
- [ ] Docs updated per the Documentation Update Checklist
- [ ] Learned summary drafted in `plan/learned/NNN-env-cleanup-N.md`

Umbrella-level (after all 6 phases close):

- [ ] Zero `exabgp`/`ze.bgp.api.`/`ze.bgp.debug.` (except pprof rename)/`ze.bgp.daemon.` registrations remain
- [ ] `ze env list` output reviewed end-to-end — no surprises
- [ ] Migration tool `.ci` tests cover every removed ExaBGP key
- [ ] `docs/guide/environment-variables.md` is the single source of truth
- [ ] Learned summary aggregates the six phase summaries

## Open questions for reviewer

1. **Rename `ze.bgp.debug.pprof` → `ze.pprof` (top-level) vs `ze.bgp.pprof`?** Consumer lives in `cmd/ze` (hub-wide). `ze.pprof` matches `ze.chaos.pprof` for the ze-chaos binary. Recommendation: `ze.pprof`.
2. **`ze.bgp.tcp.attempts` → `ze.bgp.attempts` or `ze.bgp.peer.max-attempts`?** Current semantic: "stop session after N connection attempts" per peer. Recommendation: `ze.bgp.attempts` (shorter).
3. **`ze.pid.file` vs `ze.pid`?** Systemd pattern is `PIDFile=`. Recommendation: `ze.pid.file`.
4. **`ze.bgp.announce.delay` unit — seconds integer or `time.Duration` string?** ExaBGP was minutes-modulo (quirky). Recommendation: `time.Duration` string (`"5s"`, `"2m"`) matching every other Ze duration knob.
5. **Migration tool comment format — retain original ExaBGP key text or advice only?** Recommendation: both — `# exabgp.bgp.openwait = 60 -- set ZE_BGP_OPENWAIT=60`.
6. **`ze.log.l2tp` explicit registration — keep for discoverability or delete (wildcard covers)?** Recommendation: delete.

Answer these and the six phase specs can be written.
