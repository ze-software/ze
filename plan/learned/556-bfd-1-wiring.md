# 556 -- bfd-1-wiring

## Context

The BFD plugin skeleton landed in `e5a4add9` as deliberately dead code:
the packet codec, session FSM, transport, and engine express loop were
complete and race-clean, but `RunBFDPlugin` was a no-op stub that
returned 0 immediately. The skeleton was not blank-imported by the
generated `internal/component/plugin/all/all.go` and a safety comment
in `register.go` warned future sessions not to run `make generate`
until a wiring spec landed. Stage 1 closes that gap. The user's
constraint was strict reachability proof — operator can place a
`bfd { ... }` block in the config and observe the plugin go from "not
loaded" to "running" — without taking on the harder Stage 2+ work
(privileged UDP bind, multi-VRF, BGP opt-in, FRR interop) which
remains tracked in `plan/deferrals.md`.

## Decisions

- **Copy the SDK lifecycle from `internal/plugins/sysrib/sysrib.go`,
  not from any BGP plugin.** sysrib is the closest structural match: a
  single long-lived goroutine, YANG-driven config, no reactor coupling.
  The BGP plugins all carry coupling to the reactor's session table
  and would have polluted Stage 1 with unnecessary surface area.
- **Per (VRF, mode) `engine.Loop` instances, lazily allocated.** The
  bfd plugin keeps `map[loopKey]*engine.Loop` with `loopKey = {vrf,
  mode}`. `loopFor` creates and starts a loop the first time a session
  for that pair is requested. Stage 1 binds VRF "default" only;
  non-default VRF returns an explicit error pointing at
  `spec-bfd-2-transport-hardening`. Two loops per VRF (single-hop,
  multi-hop) over the alternative of one loop multiplexing both ports
  -- a single Transport interface implementation can only bind one UDP
  port, and aggregating two would have invented a new "MultiTransport"
  abstraction the spec did not need.
- **Profile-only test config (no pinned sessions).** The test
  `bfd-config-load.ci` uses `bfd { enabled true; profile fast { ... } }`
  with NO `single-hop-session` entries. With no sessions, `loopFor` is
  never called, no UDP socket is bound, and the test runs as an
  unprivileged user. Pinned-session coverage and the FRR interop
  handshake test land with `spec-bfd-3-bgp-client`.
- **`SessionHandle.Shutdown` / `Enable` added to the public API now**,
  not deferred. The Stage 1 lifecycle code needs them to honour
  `shutdown true/false` on pinned sessions, and adding them later
  would have meant a contract change in the public surface
  `pkg/plugin/sdk` external plugins compile against. Better to ship
  the contract once.
- **Config parser walks `map[string]any`, not typed structs.** First
  attempt used a json-tagged struct with `*bool` fields and immediately
  exploded on `cannot unmarshal string into Go struct field
  .bfd.enabled of type bool`. The SDK config bridge stringifies every
  leaf value before delivery (consistent with how `iface/config.go`
  handles its own sections). Rewrote the parser to walk a generic map
  and convert strings explicitly via `parseBool` /
  `strconv.ParseUint`. Pattern now matches `parseIfaceConfig`.
- **Two `.ci` tests, not one.** The handoff suggested a single
  `01-standalone-session.ci`. Stage 1 ships `bfd-features.ci` (CLI
  discovery, mirrors `plugin-rib-features.ci`) and
  `bfd-config-load.ci` (full lifecycle, Python orchestrator). They
  fail for different reasons -- broken plugin registration vs broken
  parser/lifecycle -- and are easier to triage separately.
- **Python orchestrator over `.ci` runner timeout-kill.** The runner
  has no clean "wait for daemon stderr pattern then SIGTERM"
  primitive, and `cmd=foreground:exec=ze -:timeout=5s` without an
  observer fails immediately because the runner sees no peer process
  to wait on. The orchestrator (`tmpfs=test-bfd-load.py`) launches ze
  as a subprocess, captures stderr, polls for the four required
  patterns, then SIGTERMs cleanly. Exit 0 means all patterns matched.
- **`ze-bfd-conf` already in the parser's hard-coded YANG module list.**
  This was the missing wiring step the handoff did not mention. The
  parser at `internal/component/config/yang_schema.go YANGSchemaWithPlugins`
  enumerates the modules whose top-level entries become valid config
  keywords. Without an entry for `ze-bfd-conf` the parser would reject
  `bfd { ... }` with "unknown top-level keyword: bfd" before any
  plugin code runs. Investigation showed the iface-tunnel commit
  `2488c4b1` had ALREADY added the block while doing its own work.
  The local Edit reapplied the same content as a no-op (git diff is
  empty). The same gap still exists for `ze-rib-conf` (sysrib) which
  is why `rib { ... }` also does not work as a top-level block today.

## Consequences

- **The bfd plugin is now reachable from a running ze.** An operator
  who places a `bfd { profile fast { ... } }` block in their config
  will see the plugin auto-load via `ConfigRoots: ["bfd"]`, parse the
  YANG, deliver to `OnConfigure`, and start emitting subsystem=bfd
  log records. The path that was dead code in `e5a4add9` is fully
  alive.
- **The Stage 2+ specs can now build on a stable Service contract.**
  `api.SessionHandle` carries Subscribe / Unsubscribe / Key / Shutdown
  / Enable; the `pkg/plugin/sdk` external plugin surface is frozen
  for these. BGP opt-in (`spec-bfd-3-bgp-client`) plugs into
  `Service.EnsureSession` without changing the contract.
- **Pinned sessions remain untested end-to-end.** The lifecycle code
  for `applyPinned` (loop allocation, EnsureSession, refcount
  tracking, Shutdown propagation) is exercised by Go unit tests
  (`engine_test.go`) but not by a `.ci` test that actually binds UDP
  and runs two ze processes. That test depends on the FRR interop
  scaffolding tracked in `spec-bfd-3-bgp-client` and the
  privileged-port handling in `spec-bfd-2-transport-hardening`.
- **Profile parameters are not yet validated against RFC bounds at
  the YANG layer.** `detect-multiplier 0` is rejected by the parser,
  but `desired-min-tx-us 0` is not -- the slow-start floor in the
  session FSM masks the misconfiguration at runtime. Stage 2 should
  add boundary tests once real session establishment exposes the
  failure modes.
- **`ze-rib-conf` (sysrib) still does not parse as a top-level
  keyword.** The fix in `yang_schema.go` is bfd-specific. A future
  cleanup should either generalise the loader to walk every
  registered YANG module's top-level container, or add the missing
  `ze-rib-conf` entry. Tracked as a follow-up note in `plan/deferrals.md`.

## Gotchas

- **`yang_schema.go` hard-codes the module list.** This was the
  surprise of the spec. The handoff said the bfd plugin would
  auto-load via `ConfigRoots: ["bfd"]`, but `ConfigRoots` is the
  *plugin-side* declaration -- the *parser* must independently know
  that `bfd` is a valid top-level keyword. Two separate registrations
  for the same fact. Future plugin authors will hit this; the rule
  should be "every new top-level config block touches both
  `register.go ConfigRoots` AND `yang_schema.go YANGSchemaWithPlugins`."
- **ConfigSection.Data leaves are strings.** Typed-struct unmarshal
  with `*bool` / `*uint32` fails on every leaf because the SDK
  serialiser stringifies the tree. The pattern from `iface/config.go`
  (`map[string]any` walk + `parseBool` / `strconv.ParseUint`) is
  mandatory for any plugin parser that consumes SDK Configure data.
- **Two explicit plugin lists.** `TestAllPluginsRegistered` in
  `internal/component/plugin/all/all_test.go` AND `TestAvailablePlugins`
  in `cmd/ze/main_test.go` are independent hard-coded slices that both
  need updating. The handoff mentioned the first; `make ze-verify`
  caught the second on the first run. Always grep both files when
  adding a plugin.
- **`make ze` is not enough between iterations.** The functional
  tests need `bin/ze-test` rebuilt as well; `make build` covers both.
  First test run after a code change with stale `bin/ze-test` produces
  confusing failures.
- **`go build` is hook-blocked at the repo root.** Use `go vet
  ./internal/plugins/bfd/...` for fast compile checks; use
  `make ze` / `make build` for binaries; never `go build .`.
- **Test runner classifies daemon-only tests as "TYPE: unknown" and
  reports `Client produced no output`** when there is no peer process
  to wait on. The runner only waits for the foreground daemon if
  `expect=exit:code` is set; otherwise it falls through to the empty
  peer-wait loop and immediately terminates the daemon. A Python
  orchestrator that manages the daemon lifecycle as a subprocess is
  the cleanest workaround.
- **`pkill -f "bin/ze -"` in the bash test loop is dangerous** during
  iteration -- it can kill the running daemons spawned by the
  ze-test runner if they overlap. Use targeted process management
  inside the orchestrator instead.

## Files

- `internal/plugins/bfd/api/service.go` -- added Shutdown / Enable to SessionHandle
- `internal/plugins/bfd/engine/engine.go` -- added ErrUnknownSession
- `internal/plugins/bfd/engine/loop.go` -- handle Shutdown / Enable implementations
- `internal/plugins/bfd/register.go` -- removed warning comment, added ConfigureEngineLogger
- `internal/plugins/bfd/bfd.go` -- replaced stub with full SDK lifecycle and runtimeState
- `internal/plugins/bfd/config.go` -- new, walks ConfigSection.Data as map[string]any
- `internal/component/config/yang_schema.go` -- loads ze-bfd-conf module (already present at spec-run time, added by iface-tunnel commit 2488c4b1)
- `internal/component/plugin/all/all.go` -- regenerated with bfd blank imports
- `internal/component/plugin/all/all_test.go` -- bumped expected list
- `cmd/ze/main_test.go` -- bumped expected list
- `test/plugin/bfd-features.ci` -- new, CLI discovery proof
- `test/plugin/bfd-config-load.ci` -- new, lifecycle proof via Python orchestrator
- `plan/deferrals.md` -- BFD Stage 2+ rows appended
