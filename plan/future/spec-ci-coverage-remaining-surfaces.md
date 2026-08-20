# Spec: ci-coverage-remaining-surfaces

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | test coverage |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Write the four `.ci` groups that `spec-finish-ci-coverage` carried as captured
intent and never designed. Each one covers a surface that already ships and is
already unit-tested. None of the four is a defect: nothing is red today, and
nothing here holds the first release (`plan/future/README.md`).

`spec-finish-ci-coverage` closed on 2026-08-17 against its op-1 phase alone. That
phase was the Tier-1 command `.ci`, AC-1 to AC-8, plus the agent-tooling gates T-4
and T-5. The four items below were the rest of its `## Task`. They had no phase,
no acceptance criteria, and no test. This spec is where they live now, so the
umbrella can close and drop none of them.

Every claim below was re-verified at its producer on 2026-08-17. Two names the
2026-07-06 triage listed do not resolve at HEAD. Each one is called out rather
than carried forward as fact.

### Item 1: env-knob `.ci` (0 of ~12 exist)

**What is missing.** No `.ci` anywhere under `test/` names any of these keys.
Verified with `grep -rl <key> test/`, which returns nothing for each:

| Knob | Registered at | Read at |
|------|---------------|---------|
| `ze.bgp.openwait` | `internal/component/config/environment.go` (`ze.bgp.openwait`, default 120) | `internal/component/bgp/reactor/session_connection.go`, `env.GetDuration("ze.bgp.openwait", 120*time.Second)` |
| `ze.bgp.announce.delay` | `internal/component/config/environment.go` (default `0s`) | `internal/component/bgp/reactor/reactor.go`, `env.GetDuration("ze.bgp.announce.delay", 0)` |
| `ze.pid.file` | `internal/component/config/environment.go` | `writePIDFile` (`cmd/ze/hub/pidfile.go`), and `apply_env.go` maps it from the `daemon pid` YANG option |
| `ze.chaos.pprof` | `internal/chaos/orchestrator/cli.go` | the ze-chaos orchestrator, not the daemon |
| `ze.log.l2tp` | `internal/component/l2tp/config.go` | the L2TP subsystem logger |
| ExaBGP `env` migration | `internal/exabgp/migration/env.go` | `bgp.openwait` and `tcp.delay` are translated to `environment { bgp { ... } }` config lines there. This is a MIGRATION surface, not a Ze env knob, so it wants a `test/parse/` case over `ze exabgp migrate` rather than an `option=env` case |
| `bridge-ack` | **nothing.** No `env.MustRegister` and no `env.Get*` call in the tree names a `ze.*ack*` or `ze.*bridge*` key, and `internal/component/iface/` registers no env key at all | - |

**What blocks it: nothing.** `option=env` is parsed (`parseOption`,
`internal/test/runner/record_parse.go`, `case "env":`) and is already used by
the `test/ipsec/*.ci` group, so the runner plumbing the 2026-07-06 row called
missing exists. The taker writes cases, one knob at a time.

**What the taker must decide first.** `bridge-ack` names nothing, so it is either
a knob that was renamed or removed, or a name the triage row got wrong. Re-derive
it or drop the item's seventh row. Do not spend a session hunting it.
`ze.chaos.pprof` belongs to `ze-chaos`, not to the daemon, so its case is a chaos
scenario and not a `test/parse/` or `test/plugin/` `.ci`.

### Item 2: cli-dispatch `.ci` (2 of 3 exist)

**What is missing.** `validate-config` is covered. `set interface create` and
`update peeringdb` are not: no file under `test/` names either command string.
The PeeringDB producer is `internal/component/bgp/plugins/cmd/peer/prefix_update.go`
(`LookupASN`, and the `peeringdbRateLimit` of one second).

**What blocks it.** `update peeringdb` reaches the public PeeringDB API, so a
`.ci` needs either a local HTTP stub the command can be pointed at, or a
recorded fixture. Decide which before writing the case: a test that dials the
real API is a test whose verdict depends on somebody else's uptime.
`set interface create` needs a Linux host with netlink, so it wants
`option=needs-linux` and the netns launch mode.

### Item 3: no-congestion-initial chaos `.ci`

**What is missing.** The scenario itself. Multi-peer chaos orchestration exists
(`mk/test-chaos.mk`, `--peers`), which is what the 2026-07-06 row was waiting on.

**What blocks it: one constraint, not a gap.** `ValidateConfigRangeConflicts`
(`internal/chaos/orchestrator/conflict.go`) derives the per-peer BGP and listen
port ranges from the profile list and delegates to `ValidateRangeConflicts`. It
is called at orchestrator entry (`internal/chaos/orchestrator/run.go`), so a
scenario whose web, metrics, or MCP listener falls inside a derived range is
refused before the run starts. Place those listeners outside the derived ranges.

### Item 4: gRPC-over-wire `.ci`

**What is missing.** A test that speaks gRPC to the daemon's own gRPC listener.
`test/plugin/grpc-execute.ci` does not: its helper calls
`api._call_engine('ze-plugin-engine:dispatch-command', ...)`, which is the plugin
stdio channel. The gRPC transport's handler is exercised in-process, and nothing
puts a gRPC frame on a socket.

**What blocks it: an unresolved tooling question, and it is the reason this item
never started.** No gRPC client is available to a `.ci` helper script:

| Candidate | State at HEAD |
|-----------|---------------|
| `grpcurl` | not on PATH, not installed by `make ze-dev-setup`, and named by no `Makefile`, `mk/`, `scripts/` or `test/` file |
| Python `grpcio` | `python3 -c "import grpc"` fails with `ModuleNotFoundError`. Every `.ci` helper is Python |
| Go `google.golang.org/grpc` | vendored, and already a build dependency |

So the question is which of three shapes the test takes. That is a decision about
test tooling, and the owner MUST make it. A session that picks one for him is how
a dependency arrives that nobody agreed to:

1. Vendor or install `grpcurl` and add it to `make ze-dev-setup` plus a `ze doctor`
   check. It adds a runtime dependency to every developer machine and to CI.
2. Add `grpcio` to the test-side Python requirements. It puts a generated-stub
   build step in front of the functional suite, and the `.ci` helper environment
   currently installs nothing.
3. Write the client in Go, against the already-vendored `google.golang.org/grpc`,
   and drive it from the `.ci` as a built helper binary. It adds no external
   dependency, and it needs the runner to build and place that binary, which is
   the same shim-dir problem `spec-fixit-parse-suite-helper-cannot-invoke-ze`
   describes for `ze` itself.

Option 3 costs no new dependency and is the one the tree already supports at the
library level. It is not chosen here: this spec records the three so the next
reader inherits the question instead of re-deriving it.

## Inherited deferral rows

`spec-finish-ci-coverage` was the catch-all home for deferred `.ci` work, so
twelve live rows in nine other shards named it as their Destination. Commit B of
its closure removes the file, which would leave every one of them pointing at
nothing. No gate would have caught it: the FAIL pass of
`scripts/dev/spec-citation-check.py` globs `plan/spec-*.md` and never reads
`plan/deferrals/`.

Each row's Destination cell now names this spec. Each row stays `deferred` and
undesigned, which is the state it was already in. Only its home changed.

| Shard | Row | Subject |
|-------|-----|---------|
| `plan/deferrals/test-coverage-gaps.md` | 2026-07-10 AC-3 | five IPsec `.ci` exist. Their `vpn-ipsec/sa-up` event assertions were dropped, blocked on two engine gaps |
| `plan/deferrals/test-coverage-gaps.md` | 2026-07-10 AC-3 (`ipsec-dpd-timeout.ci`, deleted) | needs a runner primitive to stop a background daemon mid-test |
| `plan/deferrals/test-coverage-gaps.md` | 2026-07-10 AC-3 (`ipsec-monitor.ci`, deleted) | `monitor vpn ipsec` streaming, blocked on the bgp-locked startup subscription |
| `plan/deferrals/fixit-plugin-event-subscription.md` | 2026-07-19 | functional `.ci` for the SDK-fork end-to-end path |
| `plan/deferrals/fixit-perf-alloc-ci-gate.md` | 2026-07-19 | full `make ze-alloc-check` run, needs a bench log |
| `plan/deferrals/fixit-fuzz-target-discovery.md` | 2026-07-19 | bounded `ze-fuzz-test` mutation run over the newly-enabled IS-IS / OSPF targets |
| `plan/deferrals/fixit-agent-tooling-misleads.md` | 2026-07-19 | functional proof beyond the corrected gates (the row's own What cell says "none") |
| `plan/deferrals/fixit-runner-kill-background.md` | 2026-07-19 | `stop-background.ci` live run |
| `plan/deferrals/fixit-ddos-test-infra.md` | 2026-07-16 (AC-10) | proving packets are FORWARDED needs a receiver on the far veth end |
| `plan/deferrals/fixit-pppoe-orphaned-tests.md` | 2026-07-19 | the `docs/features.md` PPPoE row stays Partial |
| `plan/deferrals/fixit-doc-gate-and-refs.md` | 2026-07-19 | the optional local `check-doc-drift.sh` hook |
| `plan/deferrals/fixit-ipsec-clear-reestablish.md` | 2026-07-19 | strongSwan interop scenarios 10 and 11 need Docker plus charon |

Eight of the twelve are one batch written on 2026-07-19. Each one says the same
thing: a live-server or QEMU run was deferred to CI. Read them as ONE question
about which suites CI runs, not as eight items.

## Provenance

Homed here 2026-08-17 at the closure of `spec-finish-ci-coverage`, which closed
against its op-1 phase only. Thomas has not commissioned any of it. It is written
down so four undesigned items and twelve deferral rows have a home that exists,
rather than to schedule the work.
