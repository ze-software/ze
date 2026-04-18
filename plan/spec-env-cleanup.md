# Spec: env-cleanup

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/config-design.md` - every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var
4. `.claude/rules/go-standards.md` - "Environment Variables: internal/core/env only"
5. `.claude/rules/compatibility.md` - pre-release, no users, no compat shims
6. `internal/component/config/environment.go` - 904L file being trimmed to ~80L
7. `internal/core/env/env.go` + `registry.go` - env registry semantics
8. `internal/core/slogutil/slogutil.go` `ApplyLogConfig` - precedent for YANG config-block -> env var plumbing
9. `internal/component/config/environment_extract.go` - config tree -> map extractor
10. `~/Code/github.com/exa-networks/exabgp/src/exabgp/environment/config.py` - ExaBGP upstream key inventory

## Task

Purge the ExaBGP-compat env surface from Ze. Every env var registered today must be justified by a live consumer. Three categories:

| Category | Action |
|----------|--------|
| **delete** | Dead, no consumer, no future plan |
| **keep** | Live, Ze-native purpose; YANG path may be renamed into ze-native shape |
| **rename** | Live consumer, new ze-native key name |
| **add** | New Ze-native knob replacing a dropped ExaBGP one |

Pre-release posture (`rules/compatibility.md`): no legacy warner, no shim code, no user migration bridge. The `ze exabgp migrate` tool is the only exception because its audience is ExaBGP users migrating in.

**Decisions baked in (2026-04-18 design session):**

1. pprof env var is `ze.pprof` (top-level, not `ze.bgp.pprof`) - pairs with `ze.chaos.pprof`
2. `ze.bgp.tcp.attempts` / `reactor.MaxSessions` / `reactor.sessionCount` deleted entirely; `cmd/ze-test/bgp.go` switches to SIGTERM after session end
3. PID file env var is `ze.pid.file` (mirrors existing `ze.ready.file`)
4. `ze.bgp.announce.delay` unit is a `time.Duration` string (matches every existing duration knob)
5. Migration output: surviving keys emit YANG block with ze-native name; dropped keys emit `# <key> -- no longer supported` comment
6. `ze.log.l2tp` kept AND wired (switch `l2tp/subsystem.go:80` from `slog.Default().With(...)` to `slogutil.Logger("l2tp")`)
7. `exabgp.api.ack` YANG path `environment/exabgp/api/ack`; sets OS env var `exabgp.api.ack`; bridge subprocess reads via `os.Getenv` (subprocess pattern already documented at `cmd/ze/exabgp/main.go:170-172`)
8. No legacy env warner (no users pre-release)
9. One spec, no umbrella, no phases
10. Phantom L2TP knobs (`ze.l2tp.enabled`, `max-tunnels`, `hello-interval`, `shared-secret`, `max-sessions`) deleted (wiring infrastructure not built)

## Required Reading

### Architecture / Rules

- [ ] `.claude/rules/config-design.md`
  → Constraint: every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var
- [ ] `.claude/rules/go-standards.md` "Environment Variables" section
  → Constraint: all env access via `internal/core/env`; `Private`/`Secret` flags govern visibility
- [ ] `.claude/rules/compatibility.md`
  → Constraint: pre-release, no users, no compat shims under `internal/`; plugin API is the only post-release frozen surface
- [ ] `.claude/rules/no-layering.md`
  → Constraint: delete the old before adding the new
- [ ] `.claude/rules/naming.md`
  → Constraint: ze-native names over ExaBGP-legacy naming where nothing depends on the old form

### RFC Summaries

No RFC work in this spec.

**Key insights:**
- ExaBGP's authoritative keys live in `~/Code/github.com/exa-networks/exabgp/src/exabgp/environment/config.py` (10 sections, ~60 keys). Ze mirrors 8 but only two real consumers in the entire `Environment` struct: `TCP.Attempts` (being deleted per decision 2) and `Debug.Pprof` (being renamed to `ze.pprof`).
- `env.MustRegister` silently overwrites on duplicate key (last-wins). `IsRegistered` consults wildcard prefix patterns, so `ze.bgp.<section>.<option>` makes every `ze.bgp.*` key pass the registration guard.
- Privilege drop (`internal/core/privilege/drop.go:35-36`) reads `ze.user`/`ze.group` directly. Config-side `environment { daemon { user "bgp"; } }` must plumb into that env var.
- Slogutil (`slogutil.ApplyLogConfig`) is the precedent: it consumes `log { level X; ... }` from `ExtractEnvironment` output and calls `env.Set("ze.log.<x>", value)`. Same pattern extends to `daemon.user` -> `ze.user`, `daemon.pid` -> `ze.pid.file`, `pprof` -> `ze.pprof`, `bgp.openwait` -> `ze.bgp.openwait`, `bgp.announce-delay` -> `ze.bgp.announce.delay`, `exabgp.api.ack` -> `exabgp.api.ack`.
- The exabgp bridge is a subprocess. `cmd/ze/exabgp/main.go:170-172` comments that it uses `os.Getenv` because it runs before ze's env registry is initialized. When the parent ze process calls `env.Set` it writes through to `os.Setenv`, so the child inherits it via `os.Environ()`.
- OS env > config > YANG default is the standard priority (`rules/go-standards.md` logging section). The same rule applies to all YANG-plumbed env vars.

## Current Behavior

**Source files read (MUST before writing):**

- [ ] `internal/component/config/environment.go` (904L) - registrations, `Environment` struct, `envOptions` parser table, validators, `LoadEnvironment*`
- [ ] `internal/core/env/env.go` (219L) - `IsRegistered`, FATAL on unregistered, cache semantics
- [ ] `internal/core/env/registry.go` (77L) - `MustRegister`, wildcard prefix handling
- [ ] `internal/core/slogutil/slogutil.go` (~500L) - `ApplyLogConfig` (precedent for YANG -> env plumbing)
- [ ] `internal/exabgp/migration/env.go` (242L) - ExaBGP INI -> Ze config converter
- [ ] `internal/component/bgp/config/loader_create.go:71,188,202` - `LoadEnvironmentWithConfig`, `env.TCP.Attempts` consumer (deleted), `env.Debug.Pprof` consumer (renamed)
- [ ] `internal/component/bgp/reactor/reactor.go:133-136` - `MaxSessions` field doc ("useful for testing")
- [ ] `internal/component/bgp/reactor/reactor_notify.go:132-140` - `MaxSessions` shutdown logic (deleted)
- [ ] `internal/component/bgp/reactor/reactor.go` - registers `ze.cache.safety.valve`, `ze.buf.*` (duplicated in environment.go - dedup)
- [ ] `internal/component/bgp/reactor/session_connection.go` - OPEN read path (new `ze.bgp.openwait` consumer)
- [ ] `internal/component/l2tp/config.go` (~200L) - L2TP env registrations (five phantoms + one ze.log.l2tp)
- [ ] `internal/component/l2tp/subsystem.go:80` - `slog.Default().With("subsystem", SubsystemName)` to switch to `slogutil.Logger`
- [ ] `cmd/ze/hub/main.go` - every `env.Get` in hub startup (ze.web.*, ze.mcp.*, ze.api-server.*, ze.looking-glass.*, ze.gokrazy.*, ze.ready.file); needs PID file writer
- [ ] `cmd/ze-test/bgp.go:509-516` - test harness relies on `ze_bgp_tcp_attempts=1`; must switch to SIGTERM
- [ ] `cmd/ze/exabgp/main.go:170-172` - bridge subprocess `os.Getenv` pattern
- [ ] `internal/exabgp/bridge/bridge.go:479,593` - `zebgpToPluginWithScanner`, `pluginToZebgp`
- [ ] `internal/exabgp/bridge/bridge_muxconn.go:100-143` - `pendingResponses` (register, signal, wait)
- [ ] `cmd/ze/main.go` + `cmd/ze/internal/ssh/client/client.go` - duplicate `ze.config.dir` registration
- [ ] `internal/component/hub/schema/ze-hub-conf.yang` - `environment/{daemon,log,api,debug,chaos}` leaves
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - `environment/{tcp,bgp,cache,reactor}` augments
- [ ] `internal/component/config/environment_extract.go` - `extractSections` list

**Behavior to preserve:**

- Every live env consumer keeps working: SSH server+client, managed mode, gokrazy proxy, DNS resolver, forward pool tuning, route-server tuning, L2TP real knobs, BFD test-parallel, plugin SDK auth, slogutil per-subsystem levels, report caps, ze-chaos binary orchestration, reactor speed/cache/update-grouping/chaos injection.
- `ze-test peer --port N` continues working (via renamed `ze.test.bgp.port`).
- `ze exabgp migrate` keeps consuming ExaBGP INI files.
- `slogutil.ApplyLogConfig` behaviour unchanged: `log { level X; destination Y; <subsystem> Z; }` continues to work.
- `ze-test bgp <N>` continues working — the test harness replaces its `ze_bgp_tcp_attempts=1` trick with explicit SIGTERM after session completion.

**Behavior to change:**

- `bgp.openwait` becomes live under `ze.bgp.openwait`, wired to OPEN read deadline.
- `daemon.pid` YANG leaf stays; config loader plumbs its value into the new `ze.pid.file` env var; hub startup writes PID file on start, removes it on shutdown.
- `tcp.delay` deleted; `ze.bgp.announce.delay` added (duration string) wired to reactor startup gate.
- `daemon.user` YANG leaf stays; config loader plumbs its value into the existing `ze.user` env var (same mechanism as log).
- `debug.pprof` deleted; `ze.pprof` added (top-level YANG `environment/pprof`) wired to `startPprofServer` in hub lifecycle.
- `tcp.port` (test infra) renamed to `ze.test.bgp.port` (Private).
- `reactor.MaxSessions` and `reactor.sessionCount` deleted; test harness uses SIGTERM.
- `ze.log.l2tp` stays; `l2tp/subsystem.go:80` switched to `slogutil.Logger("l2tp")` so the env var actually controls log level.
- New `exabgp.api.ack` YANG leaf (`environment/exabgp/api/ack`, default true); config loader writes OS env; bridge subprocess reads via `os.Getenv`; bridge emits `done\n` / `error ...\n` to plugin stdin when ack mode is on.
- Every other ExaBGP-compat registration and YANG leaf deleted outright.

## Full Env Var Inventory

Notation:
- **keep** — live consumer, no rename
- **rename** — keep consumer, new key name
- **delete** — remove registration + YANG leaf + struct field + setter
- **add** — new key, new YANG leaf
- **dedup** — remove duplicate registration, one canonical site
- **plumb** — keep YANG leaf, config loader maps value into a DIFFERENT env var name (pattern already used by `slogutil.ApplyLogConfig`)
- **wire** — keep registration, complete the consumer wiring

### `ze.bgp.*` ExaBGP compat surface

| Env key | YANG leaf | Wired? | Action | Notes |
|---|---|---|---|---|
| `ze.bgp.<section>.<option>` (wildcard) | — | N/A | **delete** | Wildcard defeats registration guard |
| `ze.bgp.daemon.pid` | `environment/daemon/pid` | no | **plumb** → `ze.pid.file` | YANG leaf stays; loader sets `ze.pid.file` env var; hub writes/removes PID file |
| `ze.bgp.daemon.user` | `environment/daemon/user` | no | **plumb** → `ze.user` | YANG leaf stays; loader sets `ze.user` env var (already consumed by `privilege/drop.go:35`) |
| `ze.bgp.daemon.daemonize` | `environment/daemon/daemonize` | no | **delete** | Ze doesn't fork (systemd/gokrazy own the process) |
| `ze.bgp.daemon.drop` | `environment/daemon/drop` | no | **delete** | Implicit in Ze (drops iff `ze.user` set) |
| `ze.bgp.daemon.umask` | `environment/daemon/umask` | no | **delete** | Documented replacement: `UMask=` in systemd unit file |
| `ze.bgp.log.level` | `environment/log/level` | no | **delete** | Slogutil already maps `log { level X; }` → `ze.log` |
| `ze.bgp.log.destination` | `environment/log/destination` | no | **delete** | Slogutil owns this mapping |
| `ze.bgp.log.short` | `environment/log/short` | no | **delete** | Ze's slog formatter has no short-mode switch |
| `ze.bgp.tcp.attempts` | `environment/tcp/attempts` | YES → `MaxSessions` | **delete entirely** | ExaBGP-debug only; update `cmd/ze-test/bgp.go` to SIGTERM |
| `ze.bgp.tcp.delay` | `environment/tcp/delay` | no | **delete**, add `ze.bgp.announce.delay` | New ze-native knob, staged announcement gate |
| `ze.bgp.tcp.acl` | `environment/tcp/acl` | no | **delete** | Upstream marks "experimental, unimplemented" |
| `ze.bgp.tcp.port` | — (env-only) | YES (test only) | **rename** → `ze.test.bgp.port` (Private) | Name makes test-only scope obvious |
| `ze.bgp.bgp.openwait` | `environment/bgp/openwait` | no | **rename** → `ze.bgp.openwait` (YANG `environment/bgp/openwait`) | Wire to OPEN read deadline |
| `ze.bgp.cache.attributes` | `environment/cache/attributes` | no | **delete** | Ze always caches (pool dedup is architectural) |
| `ze.bgp.reactor.speed` | `environment/reactor/speed` | YES | **keep** | |
| `ze.bgp.reactor.cache-ttl` | `environment/reactor/cache-ttl` | YES | **keep** | |
| `ze.bgp.reactor.cache-max` | `environment/reactor/cache-max` | YES | **keep** | |
| `ze.bgp.reactor.update-groups` | `environment/reactor/update-groups` | YES | **keep** | |
| `ze.bgp.chaos.seed` | `environment/chaos/seed` | YES | **keep** | |
| `ze.bgp.chaos.rate` | `environment/chaos/rate` | YES | **keep** | |
| `ze.bgp.api.ack` | `environment/api/ack` | no | **delete** | Replaced by separately-namespaced `exabgp.api.ack` (decision 7) |
| `ze.bgp.api.chunk` | `environment/api/chunk` | no | **delete** | |
| `ze.bgp.api.encoder` | `environment/api/encoder` | no | **delete** | |
| `ze.bgp.api.compact` | — | no | **delete** | |
| `ze.bgp.api.respawn` | `environment/api/respawn` | no | **delete** | |
| `ze.bgp.api.terminate` | — | no | **delete** | |
| `ze.bgp.api.cli` | `environment/api/cli` | no | **delete** | Ze uses SSH CLI, not named pipes |
| `ze.bgp.debug.pprof` | `environment/debug/pprof` | YES → `startPprofServer` | **rename** → `ze.pprof`, YANG `environment/pprof` | Pprof is process-wide, not BGP-specific |
| `ze.bgp.debug.pdb` | `environment/debug/pdb` | no | **delete** | Python debugger, N/A in Go |
| `ze.bgp.debug.memory` | `environment/debug/memory` | no | **delete** | Go has `go tool pprof` |
| `ze.bgp.debug.configuration` | `environment/debug/configuration` | no | **delete** | Ze default is fail-fast |
| `ze.bgp.debug.selfcheck` | `environment/debug/selfcheck` | no | **delete** | |
| `ze.bgp.debug.route` | `environment/debug/route` | no | **delete** | |
| `ze.bgp.debug.defensive` | `environment/debug/defensive` | no | **delete** | Replaced by `ze.bgp.chaos.*` |
| `ze.bgp.debug.rotate` | `environment/debug/rotate` | no | **delete** | |
| `ze.bgp.debug.timing` | `environment/debug/timing` | no | **delete** | |

### `ze.log.*` (slogutil owns)

| Env key | YANG leaf | Wired? | Action | Notes |
|---|---|---|---|---|
| `ze.log` | `log { level X; }` | YES | **keep** | |
| `ze.log.<subsystem>` (wildcard) | `log { <subsystem> X; }` | YES | **keep** | |
| `ze.log.backend` | `log/backend` | YES | **keep** | |
| `ze.log.destination` | `log/destination` | YES | **keep** | |
| `ze.log.relay` | `log/relay` | YES | **keep** | |
| `ze.log.color` | — (CLI) | YES | **keep** | |
| `ze.log.l2tp` | covered by wildcard | no (phantom) | **wire** | Switch `l2tp/subsystem.go:80` to `slogutil.Logger("l2tp")` |

### Reactor/session tuning (`ze.fwd.*`, `ze.rs.*`, `ze.buf.*`, `ze.cache.*`, `ze.metrics.*`)

| Env key | Wired? | Action |
|---|---|---|
| `ze.fwd.chan.size` | YES | keep |
| `ze.fwd.write.deadline` | YES | keep |
| `ze.fwd.pool.size` | YES | keep |
| `ze.fwd.pool.maxbytes` | YES | keep |
| `ze.fwd.batch.limit` | YES | keep |
| `ze.fwd.teardown.grace` | YES | keep |
| `ze.fwd.pool.headroom` | YES | keep |
| `ze.rs.chan.size` | YES | keep |
| `ze.rs.fwd.senders` | YES | keep |
| `ze.buf.read.size` | YES | **dedup** — registered in env.go AND reactor.go; keep reactor.go |
| `ze.buf.write.size` | YES | **dedup** — same |
| `ze.cache.safety.valve` | YES | **dedup** — same |
| `ze.metrics.interval` | YES | keep |

### L2TP (`ze.l2tp.*`)

| Env key | Wired? | Action | Notes |
|---|---|---|---|
| `ze.l2tp.enabled` | no (phantom) | **delete** | `ExtractParameters` reads YANG tree only; no `env.Get` call |
| `ze.l2tp.hello-interval` | no (phantom) | **delete** | Same |
| `ze.l2tp.max-sessions` | no (phantom) | **delete** | Same |
| `ze.l2tp.max-tunnels` | no (phantom) | **delete** | Same |
| `ze.l2tp.shared-secret` | no (phantom) | **delete** | Same — silent-auth-fail footgun if kept |
| `ze.l2tp.skip-kernel-probe` | YES | keep (Private) | Test-only bypass |
| `ze.l2tp.auth.timeout` | YES | keep | |
| `ze.l2tp.auth.reauth-interval` | YES | keep | |
| `ze.l2tp.ncp.enable-ipcp` | YES | keep | |
| `ze.l2tp.ncp.enable-ipv6cp` | YES | keep | |
| `ze.l2tp.ncp.ip-timeout` | YES | keep | |
| `ze.bfd.test-parallel` | YES | keep (Private) | Test-only |

### DNS / report / plugin / service endpoints / managed / SSH / chaos binary / infra

All keys in these families have live consumers today. **keep** unchanged except `ze.config.dir` **dedup** (remove duplicate registration in `cmd/ze/internal/ssh/client/client.go`, keep `cmd/ze/main.go`).

### New keys

| New env key | YANG leaf | Consumer | Notes |
|---|---|---|---|
| `ze.bgp.openwait` | `environment/bgp/openwait` | `session_connection.go` OPEN read deadline | Default 120s, range 1-3600 |
| `ze.bgp.announce.delay` | `environment/bgp/announce-delay` | Reactor startup gate before first UPDATE | Duration string, default `0s`, range 0-1h |
| `ze.pid.file` | `environment/daemon/pid` (existing leaf renamed-plumbed) | Hub startup writes PID, shutdown removes | Default empty (no PID file) |
| `ze.pprof` | `environment/pprof` (moved from `environment/debug/pprof`) | `loader_create.go:202` → `startPprofServer` | Default empty, format `addr:port` |
| `ze.test.bgp.port` | — (env-only, Private) | `cmd/ze-test/peer.go`, `peers.go:585` | Renamed from `ze.bgp.tcp.port`, default 179 |
| `exabgp.api.ack` | `environment/exabgp/api/ack` | `internal/exabgp/bridge/` — bridge subprocess reads via `os.Getenv` | Default `true` |

## Data Flow

### Entry Point

- OS environment variables (shell, systemd EnvironmentFile, container runtime).
- YANG config `environment { ... }` block resolved via `ExtractEnvironment` then plumbed into OS env vars by `ApplyLogConfig` (log keys) and a new `ApplyEnvConfig` (everything else).
- `exabgp.env` INI file parsed by `ze exabgp migrate` into Ze YANG config text.

### Transformation Path

1. `cmd/ze/hub/main.go` start → `init()` of every package populates the env registry via `env.MustRegister`.
2. Config file → `parser.Parse` → `Tree`.
3. `config.ExtractEnvironment(tree)` walks `environment { ... }` into `map[section]map[option]value`.
4. `slogutil.ApplyLogConfig(envValues)` sets `ze.log.*` env vars when not already in OS env (OS wins).
5. NEW: `config.ApplyEnvConfig(envValues)` sets `ze.pid.file`, `ze.user`, `ze.pprof`, `ze.bgp.openwait`, `ze.bgp.announce.delay`, `exabgp.api.ack` under the same "OS wins" rule.
6. Hub consumers (`startPprofServer`, PID file writer, privilege drop, exabgp bridge launch) call `env.Get*` against the now-populated values.
7. `session_connection.go` uses `env.GetDuration("ze.bgp.openwait", 120*time.Second)` on each OPEN read deadline arming.
8. Reactor uses `env.GetDuration("ze.bgp.announce.delay", 0)` at startup to gate first UPDATE emission.
9. Bridge subprocess inherits `os.Environ()`; calls `os.Getenv("exabgp.api.ack")` directly because it runs before ze's env registry is initialized (`cmd/ze/exabgp/main.go:170-172`).

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Shell → process | `os.Environ()` snapshot | env.Get cache-rebuild test |
| env package → consumer | `env.Get*` | FATAL-on-unregistered test |
| Config file → env | `ApplyLogConfig` + new `ApplyEnvConfig(configValues)` | new unit test + existing slogutil suite |
| Parent process → bridge subprocess | `os.Environ()` inheritance | `.ci` test launches bridge with env set, asserts behaviour |
| ExaBGP INI → Ze config text | `internal/exabgp/migration/env.go` | migration `.ci` tests |

### Integration Points

- `internal/core/env` registry — every binary's `init()` populates it.
- `internal/core/slogutil/slogutil.go` `ApplyLogConfig` — precedent pattern, no change to the log path.
- `internal/component/config/apply_env.go` — NEW sibling of `environment_extract.go` that maps surviving YANG `environment/` sections to env-var names.
- `internal/component/bgp/config/loader_create.go:188,202` — changes from reading `env.TCP.Attempts` / `env.Debug.Pprof` (struct) to direct `env.GetInt` / `env.Get` on renamed keys.
- `cmd/ze/hub/main.go` — new PID file writer + new bridge-ack plumbing callsite.
- `internal/component/config/environment_extract.go` — `extractSections` list updated: remove `tcp`, `cache`, `api`, `debug`; add `exabgp`; `pprof` is a direct leaf not a section (handled as special case or the extractor returns leaf values for `environment/` root too).

### Architectural Verification

- [ ] No bypassed layers — every ze-process consumer goes through `env.Get*`; bridge subprocess uses `os.Getenv` per documented exception.
- [ ] No unintended coupling — `internal/component/config/environment.go` trims to `ParseCompoundListen`/`ListenEndpoint`/`ResolveConfigPath`/`DefaultSocketPath` only (~80 lines).
- [ ] No duplicated functionality — `Environment` struct, `envOptions` table, validators, `LoadEnvironment*` disappear entirely.
- [ ] No legacy warner, no migration shim, no compat wrappers.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|---|---|---|---|
| `ZE_BGP_OPENWAIT=N` at startup | → | OPEN read timeout in `session_connection.go` | `test/plugin/openwait-timeout.ci` |
| `environment { bgp { openwait 60; } }` in config | → | OS env `ze.bgp.openwait=60` after load → OPEN read timeout | `test/parse/openwait-config.ci` |
| `ZE_PID_FILE=/path` at startup | → | PID file writer in `cmd/ze/hub/main.go` | `test/parse/pid-file.ci` |
| `environment { daemon { pid "/path"; } }` in config | → | OS env `ze.pid.file=/path` → PID file writer | `test/parse/pid-file-config.ci` |
| `environment { daemon { user "zeuser"; } }` in config | → | OS env `ze.user=zeuser` → privilege drop | `test/parse/daemon-user-config.ci` |
| `ZE_BGP_ANNOUNCE_DELAY=5s` at reactor ready | → | Startup gate in `reactor.go` | `test/plugin/announce-delay.ci` |
| `environment { pprof ":6060"; }` in config | → | OS env `ze.pprof=:6060` → `startPprofServer` | `test/parse/pprof-config.ci` |
| `ZE_TEST_BGP_PORT=1179` | → | Peer port override in `peers.go:585` | existing `ze-test peer` tests |
| `ZE_LOG_L2TP=debug` | → | `slogutil.Logger("l2tp")` emits DEBUG lines | `test/plugin/l2tp-log-level.ci` |
| Bridge started with default env | → | Bridge emits `done\n` on plugin stdin | `test/exabgp/bridge-ack-default.ci` |
| `EXABGP_API_ACK=false` at bridge start | → | Bridge emits zero ack lines | `test/exabgp/bridge-ack-disabled.ci` |
| `environment { exabgp { api { ack false; } } }` in config | → | OS env `exabgp.api.ack=false` → bridge emits zero acks | `test/exabgp/bridge-ack-config.ci` |
| `ze exabgp migrate` on INI with surviving + dropped keys | → | YANG block for survivors + `#` comments for drops | `test/exabgp/migration-env-surviving.ci`, `test/exabgp/migration-env-dropped.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-1 | Every row marked `delete` in the inventory | Registration, YANG leaf, `Environment.*` field, and `envOptions[...]` setter row all removed |
| AC-2 | `ze.bgp.<section>.<option>` wildcard removed | `env.Get("ze.bgp.typo")` aborts with FATAL (registration guard restored) |
| AC-3 | Every row marked `rename` | Old key unregistered; new key registered; consumer reads new key |
| AC-4 | Every row marked `plumb` | YANG leaf stays; config loader sets target env var; consumer reads env var unchanged |
| AC-5 | Every row marked `add` | New key registered; consumer reads it; `.ci` test proves behaviour |
| AC-6 | Every row marked `dedup` | Key registered in exactly one file |
| AC-7 | Every row marked `wire` | Consumer code updated so env var actually controls behaviour; `.ci` test proves |
| AC-8 | `ZE_BGP_OPENWAIT=2` at peer startup, peer never sends OPEN | Session transitions to Idle after ~2s |
| AC-9 | `environment { bgp { openwait 60; } }` in config | `env.Get("ze.bgp.openwait") == "60"` after load |
| AC-10 | `ZE_PID_FILE=/tmp-safe/ze.pid` at hub startup | File exists with PID; removed at clean shutdown |
| AC-11 | `environment { daemon { pid "/tmp-safe/ze.pid"; } }` in config | Same behaviour as AC-10 (config value plumbs to env var) |
| AC-12 | `environment { daemon { user "zeuser"; } }` in config | `env.Get("ze.user") == "zeuser"`, privilege drop sees it |
| AC-13 | `ZE_BGP_ANNOUNCE_DELAY=5s` at reactor startup | First UPDATE emitted at least 5s after reactor Ready |
| AC-14 | `environment { pprof ":6060"; }` in config | pprof HTTP server started on :6060 |
| AC-15 | `ZE_LOG_L2TP=debug` with l2tp active | l2tp logger emits DEBUG lines that would be suppressed at warn |
| AC-16 | Bridge started with default env (no `exabgp.api.ack` set) | Bridge emits `done\n` on plugin stdin after each successful command dispatch |
| AC-17 | Bridge started with `EXABGP_API_ACK=false` | Bridge emits zero ack lines; does not deadlock over N commands |
| AC-18 | Bridge with ack on and Ze returns `error` for a command | Bridge emits `error ...\n` with Ze's error text |
| AC-19 | `environment { exabgp { api { ack false; } } }` in config | Bridge emits zero ack lines (config plumbs to OS env which child inherits) |
| AC-20 | `ze exabgp migrate` on INI with `bgp.openwait = 60` | Output contains `environment { bgp { openwait 60; } }` |
| AC-21 | `ze exabgp migrate` on INI with `tcp.delay = 5` | Output contains `environment { bgp { announce-delay 5m; } }` (minutes → duration, inline comment noting conversion) |
| AC-22 | `ze exabgp migrate` on INI with `daemon.user = bgp` | Output contains `environment { daemon { user "bgp"; } }` |
| AC-23 | `ze exabgp migrate` on INI with `debug.pdb = true` | Output contains `# debug.pdb = true -- no longer supported` |
| AC-24 | `ze exabgp migrate` on INI with `tcp.attempts = 3` | Output contains `# tcp.attempts = 3 -- no longer supported` |
| AC-25 | `ze-test bgp N` (functional test harness) | Test passes without `ze_bgp_tcp_attempts=1`; harness SIGTERMs ze after session completes |
| AC-26 | `grep MustRegister internal/component/config/environment.go` after cleanup | Zero hits |
| AC-27 | `wc -l internal/component/config/environment.go` after cleanup | ≤ 100 lines (down from 904) |
| AC-28 | `make ze-verify-fast` | Passes |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyEnvConfigDaemonPID` | `internal/component/config/apply_env_test.go` | `environment { daemon { pid "/x"; } }` → `env.Get("ze.pid.file") == "/x"` | new |
| `TestApplyEnvConfigDaemonUser` | same | `environment { daemon { user "u"; } }` → `env.Get("ze.user") == "u"` | new |
| `TestApplyEnvConfigPprof` | same | `environment { pprof ":6060"; }` → `env.Get("ze.pprof") == ":6060"` | new |
| `TestApplyEnvConfigBGPOpenwait` | same | `environment { bgp { openwait 60; } }` → `env.Get("ze.bgp.openwait") == "60"` | new |
| `TestApplyEnvConfigBGPAnnounceDelay` | same | `environment { bgp { announce-delay 5s; } }` → `env.Get("ze.bgp.announce.delay") == "5s"` | new |
| `TestApplyEnvConfigExabgpACK` | same | `environment { exabgp { api { ack false; } } }` → `env.Get("exabgp.api.ack") == "false"` | new |
| `TestApplyEnvConfigOSWins` | same | Pre-existing OS env var is NOT overwritten by config | new |
| `TestNoWildcardRegistered` | `internal/core/env/registry_test.go` | After cleanup, `prefixes` contains only `ze.log.`; `ze.bgp.` is absent | new |
| `TestEnvironmentFileShrunk` | `internal/component/config/environment_test.go` | Only 4 exported symbols remain: `ParseCompoundListen`, `ListenEndpoint`, `ResolveConfigPath`, `DefaultSocketPath` | rewrite |
| `TestOpenWaitEnvWired` | `internal/component/bgp/reactor/session_connection_test.go` | OPEN read path calls `env.GetDuration("ze.bgp.openwait", 120*time.Second)` | new |
| `TestAnnounceDelayEnvWired` | `internal/component/bgp/reactor/reactor_test.go` | Startup gate honours `ze.bgp.announce.delay` | new |
| `TestPIDFileWriteRemove` | `cmd/ze/hub/pidfile_test.go` | `ZE_PID_FILE` causes write on start, remove on shutdown | new |
| `TestL2TPLoggerUsesSlogutil` | `internal/component/l2tp/subsystem_test.go` | `Subsystem.logger` is obtained via `slogutil.Logger("l2tp")`, honours `ze.log.l2tp` | new |
| `TestMigrationEmitsYANG` | `internal/exabgp/migration/env_test.go` | Input `bgp.openwait = 60` → output contains `environment { bgp { openwait 60; } }` | rewrite |
| `TestMigrationEmitsDroppedComment` | same | Input `debug.pdb = true` → output contains `# debug.pdb = true -- no longer supported` | new |
| `TestBridgeACKDefault` | `internal/exabgp/bridge/bridge_ack_test.go` | Default env: bridge emits `done\n` on successful dispatch | new |
| `TestBridgeACKDisabled` | same | `EXABGP_API_ACK=false`: bridge emits zero ack lines | new |
| `TestBridgeACKError` | same | Ze returns error: bridge emits `error ...\n` with error text | new |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.bgp.openwait` | 1-3600 seconds | 3600 | 0 | 3601 |
| `ze.bgp.announce.delay` | 0-1h (duration string) | 1h | -1s | 1h1s |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `openwait-timeout.ci` | `test/plugin/openwait-timeout.ci` | `ZE_BGP_OPENWAIT=2`, sink never sends OPEN → Idle within ~2s | new |
| `openwait-config.ci` | `test/parse/openwait-config.ci` | Config block sets openwait → env var has value → timeout applies | new |
| `pid-file.ci` | `test/parse/pid-file.ci` | `ZE_PID_FILE=tmp/test/ze.pid` → file exists with PID, removed on shutdown | new |
| `pid-file-config.ci` | `test/parse/pid-file-config.ci` | Config `daemon { pid "..."; }` → same behaviour | new |
| `daemon-user-config.ci` | `test/parse/daemon-user-config.ci` | Config `daemon { user "zeuser"; }` → `env.Get("ze.user")` returns `zeuser` | new |
| `announce-delay.ci` | `test/plugin/announce-delay.ci` | `ZE_BGP_ANNOUNCE_DELAY=3s` → first UPDATE ≥3s after Ready | new |
| `pprof-config.ci` | `test/parse/pprof-config.ci` | Config `pprof ":PORT"` → HTTP GET `/debug/pprof/` returns 200 | new |
| `l2tp-log-level.ci` | `test/plugin/l2tp-log-level.ci` | `ZE_LOG_L2TP=debug` → DEBUG lines on stderr | new |
| `migration-env-surviving.ci` | `test/exabgp/migration-env-surviving.ci` | INI with surviving keys → YANG blocks in output | new |
| `migration-env-dropped.ci` | `test/exabgp/migration-env-dropped.ci` | INI with dropped keys → `#` comments in output | new |
| `bridge-ack-default.ci` | `test/exabgp/bridge-ack-default.ci` | Default env: helper reads `done\n` within 1s of announce | new |
| `bridge-ack-disabled.ci` | `test/exabgp/bridge-ack-disabled.ci` | `EXABGP_API_ACK=false`: no ack lines, no deadlock over 100 commands | new |
| `bridge-ack-config.ci` | `test/exabgp/bridge-ack-config.ci` | Config `environment { exabgp { api { ack false; } } }` → same as disabled | new |
| `test-harness-sigterm` | existing `ze-test bgp` suite | Harness runs without `ze_bgp_tcp_attempts`; SIGTERMs after session end | amend |

### Future (deferred tests)

None. Every AC has a test.

## Files to Modify

| File | Action |
|------|--------|
| `internal/component/config/environment.go` | Trim from 904L → ~80L; keep only `ParseCompoundListen`, `ListenEndpoint`, `parseOneEndpoint`, `ResolveConfigPath`, `DefaultSocketPath`, hub-infra + non-bgp env registrations that already live here (web/mcp/api-server/lg/gokrazy/dns/forward/rs/buf/cache/metrics) |
| `internal/component/config/environment_test.go` | Drop all rows for deleted options; keep parse/listener tests |
| `internal/component/config/environment_extract.go` | `extractSections` updated: remove `tcp`, `cache`, `api`, `debug`; add `exabgp` and top-level `pprof` handling |
| `internal/component/config/apply_env.go` | NEW — `ApplyEnvConfig(configValues)` sibling of `slogutil.ApplyLogConfig` |
| `internal/component/config/apply_env_test.go` | NEW |
| `internal/component/config/loader.go:106-107` | Call `config.ApplyEnvConfig(envValues)` after `slogutil.ApplyLogConfig(envValues)` |
| `internal/component/hub/schema/ze-hub-conf.yang` | Delete `environment/{daemon/daemonize,daemon/drop,daemon/umask,log/short,api,debug}` leaves; add top-level `environment/pprof`; add `environment/exabgp/api/ack` |
| `internal/component/bgp/schema/ze-bgp-conf.yang` | Delete `environment/{tcp,cache,bgp/openwait}` augments; add `environment/bgp/{openwait,announce-delay}` |
| `internal/component/bgp/config/loader_create.go:71,188,202` | Drop `LoadEnvironmentWithConfig`; delete `MaxSessions` line; replace `env.Debug.Pprof` with `coreenv.Get("ze.pprof")` |
| `internal/component/bgp/reactor/reactor.go:133-136,278` | Delete `MaxSessions` field and `sessionCount` atomic; remove duplicate `ze.cache.safety.valve`, `ze.buf.*` registrations |
| `internal/component/bgp/reactor/reactor_notify.go:132-140` | Delete MaxSessions shutdown block |
| `internal/component/bgp/reactor/session_connection.go` | OPEN read deadline from `env.GetDuration("ze.bgp.openwait", 120*time.Second)` |
| `internal/component/bgp/reactor/reactor.go` startup path | Staged announce gate from `env.GetDuration("ze.bgp.announce.delay", 0)` |
| `cmd/ze/hub/main.go` | PID file write/remove at startup/shutdown |
| `cmd/ze/hub/pidfile.go` | NEW — writer/remover helpers |
| `cmd/ze-test/peer.go`, `cmd/ze-test/bgp.go:509-516` | Rename `ze.bgp.tcp.port` → `ze.test.bgp.port`; replace `ze_bgp_tcp_attempts=1` trick with SIGTERM after session completion |
| `internal/component/bgp/config/peers.go:585` | Rename `ze.bgp.tcp.port` → `ze.test.bgp.port` |
| `internal/component/bgp/config/loader_create.go:50-56` | Rename `envKeyTCPPort` value to `ze.test.bgp.port` |
| `internal/component/l2tp/subsystem.go:80` | Switch from `slog.Default().With("subsystem", SubsystemName)` to `slogutil.Logger(SubsystemName)` |
| `internal/component/l2tp/config.go` | Delete 5 phantom registrations (`enabled`, `max-tunnels`, `hello-interval`, `shared-secret`, `max-sessions`); keep `ze.log.l2tp`, `skip-kernel-probe`, `auth.*`, `ncp.*` |
| `cmd/ze/main.go` | Keep `ze.config.dir` registration |
| `cmd/ze/internal/ssh/client/client.go` | Remove duplicate `ze.config.dir` registration |
| `internal/exabgp/migration/env.go` | Rewrite emission rules: YANG block for survivors (with minutes→duration conversion for `tcp.delay`), `# <key> -- no longer supported` for dropped |
| `internal/exabgp/migration/env_test.go` | Update expected outputs |
| `internal/exabgp/bridge/bridge.go` | Read `os.Getenv("exabgp.api.ack")` once at Bridge construction; extend `zebgpToPluginWithScanner` to emit `done\n` / `error ...\n` on plugin stdin when ack mode is on |
| `internal/exabgp/bridge/bridge_ack.go` | NEW — ack emission helpers |
| `internal/exabgp/bridge/bridge_muxconn.go:100-143` | Extend `pendingResponses` to track success/error text per dispatched command |
| `test/plugin/*.ci`, `test/parse/*.ci`, `test/exabgp/*.ci` | Add new `.ci` tests; update existing ones that assert on old migration output |
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
| 1 | New user-facing feature? | Yes | `docs/features.md` — PID file, announce delay, YANG-driven env plumbing |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` — `environment { ... }` block trimmed |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/environment-variables.md` (NEW) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — ze-test SIGTERM pattern |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — ExaBGP column footnotes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — config→env plumbing pattern |

## Files to Create

- `internal/component/config/apply_env.go`
- `internal/component/config/apply_env_test.go`
- `cmd/ze/hub/pidfile.go`
- `cmd/ze/hub/pidfile_test.go`
- `internal/exabgp/bridge/bridge_ack.go`
- `internal/exabgp/bridge/bridge_ack_test.go`
- `test/plugin/openwait-timeout.ci`
- `test/plugin/announce-delay.ci`
- `test/plugin/l2tp-log-level.ci`
- `test/parse/openwait-config.ci`
- `test/parse/pid-file.ci`
- `test/parse/pid-file-config.ci`
- `test/parse/daemon-user-config.ci`
- `test/parse/pprof-config.ci`
- `test/exabgp/migration-env-surviving.ci`
- `test/exabgp/migration-env-dropped.ci`
- `test/exabgp/bridge-ack-default.ci`
- `test/exabgp/bridge-ack-disabled.ci`
- `test/exabgp/bridge-ack-config.ci`
- `docs/guide/environment-variables.md`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | — |
| 8. Re-verify | — |
| 9. Repeat 6-8 | Max 2 passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | — |
| 13. Present summary | Executive Summary per `rules/planning.md` |

### Implementation Phases

Phases are ordered by dependency. Tests drive each phase.

1. **Phase: Test harness SIGTERM** — before deleting `ze_bgp_tcp_attempts`
   - Tests: amend existing `ze-test bgp` harness tests
   - Files: `cmd/ze-test/bgp.go:509-516`
   - Verify: `ze-test bgp 1` passes without `ze_bgp_tcp_attempts=1`
2. **Phase: `ApplyEnvConfig` scaffold** — plumbing for YANG → env
   - Tests: `TestApplyEnvConfig*` (7 cases)
   - Files: `internal/component/config/apply_env.go` + test, `internal/component/config/loader.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: Delete ExaBGP compat surface** — the big diff
   - Tests: `TestNoWildcardRegistered`, `TestEnvironmentFileShrunk`
   - Files: `environment.go`, `environment_test.go`, `environment_extract.go`, YANG schemas (hub + bgp), `loader_create.go`, `reactor.go`, `reactor_notify.go`, `l2tp/config.go`
   - Verify: `grep MustRegister internal/component/config/environment.go` → 0 hits; `make ze-verify-fast` passes
4. **Phase: Renames** — `tcp.port` → `test.bgp.port`, `debug.pprof` → `pprof`, `bgp.openwait` → `bgp.openwait` (the doubled prefix is removed)
   - Tests: consumer-side unit tests
   - Files: `cmd/ze-test/peer.go`, `cmd/ze-test/bgp.go`, `peers.go`, `loader_create.go`
5. **Phase: New knobs** — openwait, announce-delay, pid.file, pprof
   - Tests: `TestOpenWaitEnvWired`, `TestAnnounceDelayEnvWired`, `TestPIDFileWriteRemove`; functional `.ci` tests
   - Files: `session_connection.go`, `reactor.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/pidfile.go`
6. **Phase: l2tp logger wire** — complete prior intent
   - Tests: `TestL2TPLoggerUsesSlogutil`, `test/plugin/l2tp-log-level.ci`
   - Files: `internal/component/l2tp/subsystem.go:80`
7. **Phase: Bridge ack** — separate component
   - Tests: `TestBridgeACK*` (3 cases); `.ci` tests for default/disabled/config
   - Files: `bridge.go`, `bridge_ack.go` (NEW), `bridge_muxconn.go`
8. **Phase: Migration rewrite**
   - Tests: `TestMigrationEmitsYANG`, `TestMigrationEmitsDroppedComment`; functional `.ci` tests
   - Files: `internal/exabgp/migration/env.go` + test
9. **Phase: Documentation**
   - Files per Documentation Update Checklist
10. **Full verification** → `make ze-verify-fast`
11. **Complete spec** → audit tables, learned summary, delete spec

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | `ApplyEnvConfig` respects OS-wins precedence (matches `ApplyLogConfig`). Bridge ack default is `true`. `announce.delay=0` means "no delay", not "block forever". |
| Naming | YANG leaves match env var names 1:1 (`environment/daemon/pid` ↔ via plumb ↔ `ze.pid.file` — the plumb mapping is the documented exception). `exabgp.api.ack` uses the uncommon top-level prefix (not `ze.*`) because the bridge subprocess reads it directly and upstream-compat is preserved. |
| Data flow | Every YANG leaf under `environment/` has EITHER (a) `ApplyLogConfig` owns it (log keys) OR (b) `ApplyEnvConfig` owns it (everything else). No third path. |
| Rule: no-layering | `Environment` struct, `envOptions` table, validators, `LoadEnvironment*` fully deleted — no shim function or wrapper left behind. |
| Rule: compatibility | No legacy warner, no startup migration hints, no `EXABGP_*` handling in ze code. (The migration tool is a separate binary; migration comments are its output, not runtime code.) |
| Rule: config-design | Every surviving YANG `environment/` leaf has a matching env var registration verified by `ze env registered`. |
| Rule: integration-completeness | Every new env var has a `.ci` test (Wiring Test table). Every `plumb` mapping has a unit test (`TestApplyEnvConfig*`). |
| Reactor concurrency | Removing `MaxSessions`/`sessionCount` touches reactor atomics. Run `make ze-race-reactor` before commit. |
| Bulk-edit safety | Grep every `ze.bgp.debug.pprof`, `env.Debug.Pprof`, `env.TCP.Attempts` consumer before removing — sibling call-site audit (`rules/before-writing-code.md`). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `environment.go` trimmed to ≤ 100 lines | `wc -l internal/component/config/environment.go` |
| Wildcard `ze.bgp.<section>.<option>` unregistered | `grep -c 'ze\.bgp\.<' internal/component/config/environment.go` → 0 |
| `Environment` struct gone | `grep -c 'type Environment ' internal/component/config/environment.go` → 0 |
| `MustRegister` count in environment.go | `grep -c MustRegister internal/component/config/environment.go` → leaves only non-BGP infrastructure keys; zero `ze.bgp.*` registrations |
| `MaxSessions` gone from reactor | `grep -rn 'MaxSessions' internal/component/bgp/reactor/ --include='*.go'` (excluding SSH) → 0 |
| `ze_bgp_tcp_attempts` gone | `grep -rn 'tcp_attempts\|tcp\.attempts' internal/ cmd/ --include='*.go'` → 0 |
| New `.ci` files exist | `ls test/plugin/openwait-timeout.ci test/parse/pid-file.ci test/plugin/announce-delay.ci test/exabgp/bridge-ack-default.ci ...` |
| New Go files exist | `ls internal/component/config/apply_env.go cmd/ze/hub/pidfile.go internal/exabgp/bridge/bridge_ack.go` |
| YANG has `environment/pprof` (hub) and `environment/bgp/{openwait,announce-delay}` (bgp) and `environment/exabgp/api/ack` (hub) | `grep -E 'pprof\|openwait\|announce-delay\|exabgp' internal/component/*/schema/*.yang` |
| L2TP logger uses `slogutil.Logger` | `grep -n 'slogutil\.Logger("l2tp")' internal/component/l2tp/subsystem.go` → 1 hit |
| Migration emits YANG blocks | `bin/ze-test exabgp migrate-env` output review |
| Bridge reads `os.Getenv("exabgp.api.ack")` | `grep -n 'exabgp.api.ack' internal/exabgp/bridge/bridge*.go` |
| `make ze-verify-fast` passes | `tmp/ze-verify.log` shows PASS |
| `make ze-race-reactor` passes | separate invocation output |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| PID file path injection | `ZE_PID_FILE` must be treated as an operator-supplied path; file creation must fail closed on permission errors (no silent skip). Writes with `os.WriteFile(path, ..., 0644)`, removes only the file it wrote. |
| PID file symlink attack | Use `os.OpenFile` with `O_CREATE|O_EXCL|O_WRONLY` so an existing symlink target cannot be overwritten. If the file already exists, fail with a clear error — refuse to reuse. |
| Shared-secret env removal | `ze.l2tp.shared-secret` registration is being deleted. Any lingering `ZE_L2TP_SHARED_SECRET` in an operator's env becomes silently ignored (no users pre-release, but document in the environment-variables.md guide). |
| Bridge ack plumbing | Plugin stdin is under ze's control; emitted `done\n`/`error ...\n` lines must be newline-terminated and length-bounded so a malformed Ze error message cannot inject additional framing. Use `strconv.Quote` or sanitize `\r\n` before emission. |
| Config → env OS env leakage | `ApplyEnvConfig` calls `env.Set` which calls `os.Setenv`. Child processes (bridge, plugins) inherit. Verify no secret-flagged key is plumbed this way without `Secret: true` flag (the flag causes `os.Unsetenv` after first `Get`). Current list: `ze.user`, `ze.pid.file`, `ze.pprof`, `ze.bgp.openwait`, `ze.bgp.announce.delay`, `exabgp.api.ack` — none are secrets. |
| FATAL-on-unregistered | After wildcard removal, `env.Get("ze.bgp.typo")` aborts. Ensure no code path reads a stale `ze.bgp.*` key that was removed from the registry — that would turn the cleanup into a crash on startup. Grep every removed key before deleting its registration. |
| Privilege drop ordering | `environment { daemon { user "u"; } }` → `env.Set("ze.user", "u")` happens in config load, BEFORE privilege drop. Verify ordering: config parse → `ApplyEnvConfig` → `DropConfigFromEnv`. If reversed, the config-set user is ignored. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error from removed `Environment` field | Delete the caller or replace with direct `env.Get*` |
| `env.Get("ze.bgp.xxx")` aborts FATAL | Grep for `ze.bgp.xxx`; if still a valid consumer, fix the rename; if stale, delete the call |
| `.ci` test flakes on timing-dependent `announce-delay` | Use `ze-test` synchronization, not `sleep` |
| `make ze-race-reactor` fails in reactor cleanup | Removing `sessionCount` atomic may have exposed a race; re-add atomic OR redesign shutdown |
| Migration tool output differs in whitespace | Update test fixtures to match canonical YANG format |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Split into 7 phase specs with an umbrella | Single concern; splitting creates half-renamed intermediate states | One spec, commit granularity decided at commit time |
| `tcp.attempts` rename to `ze.bgp.attempts` | Semantic was misleading — knob is test-only reactor shutdown counter, not per-peer retries | Delete entirely; test harness SIGTERMs |
| `daemon.user` / `daemon.pid` delete | Would regress YANG ergonomics; ze's rule is "every env var has a YANG path" | Keep YANG leaf; plumb value to `ze.user` / `ze.pid.file` env vars |
| `ze.log.l2tp` delete | Registration captured intent; previous session left wiring incomplete — completing it is aligned with the spec's goal | Keep + wire: switch `l2tp/subsystem.go:80` to `slogutil.Logger` |
| Legacy env warner at startup | No users pre-release; `compatibility.md` says so explicitly | Delete the warner entirely |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- **Spec recommendations must be verified against code.** Three times in this design session the spec's description of a knob did not match what the code does. Always grep the real consumer before recommending a rename.
- **`slogutil.ApplyLogConfig` is the reference pattern** for config-block → env-var plumbing. Extending it with a sibling `ApplyEnvConfig` is the lowest-risk way to connect new YANG leaves to env vars. Do not invent a different mechanism.
- **Pre-release `compatibility.md`** is load-bearing. Every "warn on legacy" / "softly migrate" / "detect old name" design reflex must be checked against it — if no users exist, no warning is needed.

## Implementation Summary

### What Was Implemented
- [to fill during implementation]

### Bugs Found/Fixed
- [to fill]

### Documentation Updates
- [to fill]

### Deviations from Plan
- [to fill]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| [to fill] | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| [to fill] | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| [to fill] | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| [to fill] | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | | | |

### Fixes applied
- [to fill]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| [to fill] | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| [to fill] | | |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| [to fill] | | |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-28 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes (full gate including fuzz)
- [ ] `make ze-race-reactor` passes (reactor touched)
- [ ] Feature code integrated
- [ ] Critical Review passes

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-env-cleanup.md`
- [ ] Summary included in commit
