# 628 -- env-cleanup

## Context

Ze inherited a broad ExaBGP-compat env surface: ~60 `ze.bgp.<section>.<option>`
keys registered via a wildcard prefix, an `Environment` struct spanning 9
sections (daemon/log/tcp/bgp/cache/api/reactor/debug/chaos), and 900+ lines of
validators, setters, and `LoadEnvironment*` helpers in
`internal/component/config/environment.go`. Most of that surface had no
live consumer -- only `TCP.Attempts` (a test-shutdown trick) and
`Debug.Pprof` were actually read at runtime. The wildcard registration
defeated the env-registry guard: `env.Get("ze.bgp.typo")` silently succeeded
instead of aborting. Five L2TP env vars were registered but never wired
(`enabled`, `max-tunnels`, `hello-interval`, `shared-secret`,
`max-sessions`). The `ze.log.l2tp` key was registered but `l2tp/subsystem.go`
used `slog.Default().With("subsystem", SubsystemName)` instead of
`slogutil.Logger("l2tp")`, so the env var had no effect. The goal was to
delete the compat surface, complete the wiring for the keys we kept, and
add Ze-native replacements for the rare but real consumers (announce-delay,
PID file, pprof, openwait, bridge ack).

## Decisions

- Delete the `Environment` struct, `envOptions` table, strict parsers, and
  `LoadEnvironmentWithConfig` -- trim `environment.go` from 904 to ~240L
  (registrations + listener helpers only). The YANG block is parsed by the
  standard YANG path; the env vars are plumbed via a sibling of
  `slogutil.ApplyLogConfig` called `config.ApplyEnvConfig`.
- `ApplyEnvConfig` uses a table-driven plumbing list
  (`envPlumbingTable`) mapping YANG `environment/<section>/<option>` to a
  target env var key. Top-level leaves under `environment/` land in the
  empty-string section; nested containers like `environment/exabgp/api`
  land in the dot-joined section `exabgp.api`. OS env always wins over
  the config file value.
- `ze.bgp.tcp.attempts` / `reactor.MaxSessions` / `sessionCount` deleted
  entirely. The `cmd/ze-test/bgp.go` manual-debug harness switches from
  the `ze_bgp_tcp_attempts=1` trick to `cmd.Cancel = SIGTERM`, so Ctrl+C
  produces a clean shutdown instead of a silent kill.
- Rename `ze.bgp.tcp.port` -> `ze.test.bgp.port` (Private) so the
  test-only scope is obvious. The consumer in `peers.go:applyPortOverride`
  and the `internal/test/runner/runner_exec.go` env injection track the
  new name.
- Rename `ze.bgp.debug.pprof` -> `ze.pprof` (top-level YANG leaf,
  process-wide). The loader_create.go pprof call site reads
  `coreenv.Get("ze.pprof")` directly instead of the deleted
  `env.Debug.Pprof` struct field.
- Rename `ze.bgp.bgp.openwait` -> `ze.bgp.openwait` and wire it to the
  OPEN read deadline in `session_connection.go` via `env.GetDuration`.
- Add `ze.bgp.announce.delay` (duration, default 0s) and wire it to the
  reactor's pre-peer-start gate in `StartWithContext` -- selects on
  `r.clock.After(delay)` vs `r.ctx.Done` so tests using a mock clock
  advance deterministically.
- Add `ze.pid.file` + `cmd/ze/hub/pidfile.go` with O_CREATE|O_EXCL|O_WRONLY
  (symlink-attack defense) and a shutdown remover. The YANG leaf
  `environment/daemon/pid` is plumbed via `ApplyEnvConfig`.
- Add `exabgp.api.ack` (OS-native name to preserve the ExaBGP convention;
  bridge subprocess reads it via `os.Getenv`). The bridge's
  `pluginToZebgp` loop registers a pending-response channel for each
  dispatched command, waits for ze's `#<id> ok|error` response, and emits
  `done\n` / `error <sanitized>\n` on plugin stdin. Extended
  `pendingResponses.signal` to carry a `pendingResult{ok, errText}` so
  errors propagate.
- L2TP subsystem constructor now uses `slogutil.Logger(SubsystemName)`;
  phantom L2TP env registrations deleted.
- Migration tool: `internal/exabgp/migration/env.go` rewrite. Surviving
  keys (`daemon.user`, `daemon.pid`, `bgp.openwait`, `tcp.delay`,
  `debug.pprof`, `api.ack`) emit YANG blocks. `tcp.delay` converts
  minutes-to-`Nm` duration inline with a comment. Dropped keys emit
  `# <key> = <value> -- no longer supported`.
- `ze.config.dir` duplicate registration in `cmd/ze/internal/ssh/client/client.go`
  restored after tests revealed the completion package relies on the SSH
  client's registration path without importing `cmd/ze`. Prior learning in
  `plan/learned/476` was correct; the spec's dedup instruction was wrong
  for this specific pair.
- Rejected: a legacy env warner / backward-compat shim. Pre-release, no
  users, no need. Per `rules/compatibility.md` every "warn on legacy"
  reflex was checked against the rule and deleted.
- Rejected: renaming `ze.bgp.chaos.*` or `ze.bgp.reactor.*` at the same
  time. Keys are not ExaBGP-legacy; renaming would just be churn.

## Consequences

- Single code path from YANG `environment { ... }` to OS env vars:
  `slogutil.ApplyLogConfig` owns the `log/*` subtree,
  `config.ApplyEnvConfig` owns everything else. No third path.
- Adding a new env-plumbed YANG leaf is a one-line change to
  `envPlumbingTable` plus a `MustRegister` call -- no struct,
  no validator, no setter row.
- The wildcard `ze.bgp.<section>.<option>` registration is gone, so the
  env registry guard now catches typos in any `ze.bgp.*` key.
- `reactor.Config.MaxSessions` removal reduces reactor concurrency
  surface (no more `sessionCount` atomic + `sessionCountMu` Mutex). The
  `reactor_notify.go` OnPeerClosed path is simpler.
- Migration tool's output is load-bearing for anyone converting an
  ExaBGP INI; the `# -- no longer supported` comment is the operator's
  trail for audit.
- Test `.ci` files no longer need `environment { tcp { attempts N } }`
  to make ze exit -- plugin scripts already dispatch
  `daemon shutdown`, which was always the correct exit path. Bulk-fix
  removed the block from ~30 files.

## Gotchas

- `env.MustRegister` silently overwrites on duplicate key. Two packages
  can both register the same key -- last wins. That was the trap in
  `plan/learned/476` and it bit again during this spec: removing the
  SSH-client copy of `ze.config.dir` broke completion tests, because the
  completion package constructs the SSH client without transitively
  importing `cmd/ze`'s registration. Restored the duplicate with a
  comment pointing to the learned summary.
- `environment_test.go` asked for fields like `env.BGP.OpenWait` and
  `env.TCP.Attempts` via the deleted `Environment` struct. The whole
  file was rewritten to keep only the listener / XDG tests that still
  apply. `test/parse/listener-tcp-port-removed.ci` was left in place --
  it validates that the parser STILL rejects the retired leaf.
- `environment_extract.go` had to grow two new dimensions: top-level
  leaves under `environment/` (for `pprof`) and one level of nesting
  under each listed section (for `exabgp/api`). Without the nesting
  pass, `exabgp.api.ack` would never reach `ApplyEnvConfig`.
- YANG leaf validation: `ze-bgp-conf.yang` now declares
  `openwait { type int32 { range "1..3600"; } }`. The config parser
  enforces the range at parse time; the env-var strict-range check in
  the deleted `validateOpenWait` is no longer needed.
- VPP plugin backend (untracked, another session's WIP) fails to compile
  (`undefined: logger`, sessionKey vs string map type). Not caused by
  this spec, but it breaks `go build ./...`. Noted but not fixed here.
- Several docs (`debugging-tools.md`, `environment-block.md`,
  `environment.md`, `features.md`, new `guide/environment-variables.md`)
  had to be rewritten; the 2026-03-29 "Last Updated" stamp on
  `environment.md` was pointing at a surface that no longer exists.

## Files

### Created

- `internal/component/config/apply_env.go`
- `internal/component/config/apply_env_test.go`
- `cmd/ze/hub/pidfile.go`
- `cmd/ze/hub/pidfile_test.go`
- `internal/exabgp/bridge/bridge_ack.go`
- `internal/exabgp/bridge/bridge_ack_test.go`
- `docs/guide/environment-variables.md`

### Modified

- `internal/component/config/environment.go` (904L -> ~240L)
- `internal/component/config/environment_test.go` (rewrite, kept listener/path tests)
- `internal/component/config/environment_extract.go` (top-leaf + nested-container extraction)
- `internal/component/config/constants.go` (extractSections trimmed)
- `internal/component/config/loader.go` (ApplyEnvConfig call)
- `internal/component/config/schema_defaults_test.go` (new defaults list)
- `internal/component/bgp/config/loader_create.go` (drop env struct consumer, plumb pprof/chaos from coreenv)
- `internal/component/bgp/config/peers.go` (rename comment for ze.test.bgp.port)
- `internal/component/bgp/reactor/reactor.go` (delete MaxSessions, sessionCount; announce-delay gate; drop duplicate registrations)
- `internal/component/bgp/reactor/reactor_notify.go` (delete MaxSessions block)
- `internal/component/bgp/reactor/session_connection.go` (wire ze.bgp.openwait)
- `internal/component/bgp/schema/ze-bgp-conf.yang` (remove tcp/cache augments, add announce-delay)
- `internal/component/hub/schema/ze-hub-conf.yang` (remove api/debug/daemon legacy leaves, add pprof + exabgp/api/ack)
- `internal/component/l2tp/config.go` (delete 5 phantom registrations)
- `internal/component/l2tp/subsystem.go` (slogutil.Logger)
- `internal/exabgp/bridge/bridge.go` (ack mode snapshot, dispatch-ack wait loop)
- `internal/exabgp/bridge/bridge_muxconn.go` (pendingResult on signal/wait)
- `internal/exabgp/bridge/bridge_test.go` (update for new signal/wait signatures)
- `internal/exabgp/migration/env.go` (YANG-emitting migration rewrite)
- `internal/exabgp/migration/env_test.go` (rewrite expectations)
- `cmd/ze/hub/main.go` (PID file writer/remover)
- `cmd/ze/config/cmd_validate.go` (validateEnvironment simplified)
- `cmd/ze-test/bgp.go` (SIGTERM via cmd.Cancel, rename env var)
- `cmd/ze-test/peer.go` (rename ze.bgp.tcp.port -> ze.test.bgp.port)
- `internal/test/runner/runner_exec.go` (rename env var)
- 30+ `.ci` test files (strip `environment { tcp { attempts N } }`, `cache {}`, `api {}`, `debug {}` legacy blocks)
- `docs/architecture/config/environment.md` (rewrite)
- `docs/architecture/config/environment-block.md` (rewrite)
- `docs/guide/environment-variables.md` (new)
- `docs/features.md` (add env link)
- `docs/debugging-tools.md` (remove tcp.attempts trick)
