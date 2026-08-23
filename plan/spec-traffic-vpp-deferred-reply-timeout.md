# Spec: traffic-vpp-deferred-reply-timeout

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | plugin |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/fixit-firewall-concurrency-deadlock.md` |
| Handoff | verify |
| Updated | 2026-08-23 |

<!-- Handoff `verify`: the implementation session commits the work, sets Status to
     `verification` and stops. A later Opus 5 session reviews that commit and closes. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The traffic VPP backend sends binary-API requests on a channel with no reply
deadline, so a VPP that accepts a request and never answers blocks the caller
for the life of the process.

`(*backend).Apply` (`internal/plugins/traffic/vpp/backend_linux.go`) ends with a
call to `applyWithOps` whose ops value is an inline `&govppOps{ch: ch}` literal.
No file in `internal/plugins/traffic/vpp` calls `SetReplyTimeout`, so the channel
keeps `core.DefaultReplyTimeout`. govpp sets that constant to 0 and its own comment
reads "default timeout for replies from VPP is disabled"
(`vendor/go.fd.io/govpp/core/connection.go`, `DefaultReplyTimeout`).

The producer of the block is `receiveReplyInternal`
(`vendor/go.fd.io/govpp/core/channel.go`): it reads `ch.replyTimeout`, and when the
value is at or below zero it substitutes `maxInt64`, which is about 292 years.
`Channel.ReceiveReply` takes no context, so the `ctx` that `Apply` receives from
the plugin lifecycle cannot end the wait. `Apply` holds `b.mu` across the whole
call, so the backend never accepts another apply either.

The firewall VPP backend already carries the fix. `newGovppOps`
(`internal/plugins/firewall/vpp/timeout_linux.go`) calls `SetReplyTimeout` and then
returns the ops value, so no production request can be sent on an unbounded
channel. This spec mirrors that shape into the traffic backend: a constructor that
binds the deadline, an env-var knob with a clamp, and the call site that uses the
constructor instead of the inline literal.

This work was homed from the 2026-08-07 row of
`plan/deferrals/fixit-firewall-concurrency-deadlock.md`.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/traffic/fw-7b-backend-hardening.md` - the `vppOps` seam this change constructs
  → Decision: `vppOps` is the seam that lets the apply path be tested without VPP; `govppOps` is the one production implementation of it.
  → Constraint: the doc's "The `vppOps` seam" section is the surface that must record the deadline, because the constructor is part of the seam contract.
- [ ] `docs/architecture/core-design.md` - "Firewall reconcile concurrency", the sibling record
  → Decision: both firewall backends default to 10s and clamp to 1..60s, and zero is refused because it is each library's spelling of "no deadline".
  → Constraint: `firewall.ErrKernelTimeout` lives on the firewall `Backend` contract. The traffic `Backend` contract has no such sentinel, so this spec MUST NOT import one and MUST NOT invent one.
- [ ] `ai/rules/config.md` - env-var naming and the YANG-versus-env decision
  → Decision: a safety cap that is not tuned in production is env-only, which is why the firewall knobs carry no YANG leaf.
  → Constraint: env keys are dot-separated and lowercase with the prefix `ze.<component>`, so the key here is `ze.traffic.vpp.reply-timeout`, matching `ze.firewall.vpp.reply-timeout`.
- [ ] `ai/rules/platform-linux.md` - what a Linux-only file owes QEMU
  → Constraint: a bare `//go:build linux` test that makes no syscall runs under `go test` on any Linux host, the QEMU VM included. It reaches the VM only if a QEMU target names its package: `ZE_QEMU_INTEGRATION_PKGS` derives from the `//go:build integration && linux` tag, which this package does not carry.
- [ ] `ai/rules/interop-and-goal-validation.md` - "Prove the test discriminates"
  → Constraint: the new test MUST be shown RED with the fix reverted. A test that passes with `SetReplyTimeout` removed proves nothing.

**Key insights:**

- The channel comes from a `sync.Pool` on the shared `Connection`
  (`vendor/go.fd.io/govpp/core/connection.go`, the `channelPool` factory), and
  `(*Channel).Reset` (`vendor/go.fd.io/govpp/core/channel.go`) drains the buffers
  and leaves `replyTimeout` alone. A pooled channel therefore keeps the value its
  previous owner set. Ze runs one `Connector`, so the traffic backend can receive a
  channel the firewall backend bounded at 10s. Today's behavior is not "always
  unbounded": it is unbounded or bounded depending on which plugin used the channel
  last. Binding on every construction is what makes it deterministic.
- The traffic backend is the only remaining VPP consumer named by the deferral row,
  but it is not the only one with the defect. See Known Limitations.

## Current Behavior (MANDATORY)

**Source files read:** (all read firsthand 2026-08-11)

- [ ] `internal/plugins/traffic/vpp/backend_linux.go` - `(*backend).Apply` takes `b.mu`, resolves the connector, waits for the connection with a 5s `waitConnectedTimeout`, calls `conn.NewChannel()`, defers `ch.Close()`, and returns `b.applyWithOps(&govppOps{ch: ch}, desired)`. It is the only `NewChannel()` call site in the package.
- [ ] `internal/plugins/traffic/vpp/ops_linux.go` - defines `govppOps`, a struct whose single field is `ch api.Channel`, and its eleven methods, each one a wrap around a single VPP request with retval decoding. No constructor exists.
- [ ] `internal/plugins/traffic/vpp/ops.go` - defines the `vppOps` interface that `applyWithOps` consumes; tests substitute a `fakeOps`.
- [ ] `internal/plugins/traffic/vpp/apply_test.go` - twenty tests, `//go:build linux`, all reaching `applyWithOps` with a fake, or reaching `Apply` against an unconnected `Connector`. None constructs a `govppOps`.
- [ ] `internal/plugins/firewall/vpp/timeout_linux.go` - the pattern: three constants, one `env.MustRegister`, `vppReplyTimeout` clamping with `min` and `max`, `newGovppOps` setting the deadline before returning, and `asDataplaneTimeout` tagging the error.
- [ ] `internal/plugins/firewall/vpp/timeout_linux_test.go` - the test pattern: a `recordingChannel` that implements `api.Channel` and records the duration, with every other method a panic; `TestVppReplyTimeoutBounds`; `TestNewGovppOpsBindsReplyTimeout`.
- [ ] `internal/component/traffic/register.go` - the callers of `Backend.Apply`: the config-apply handler, and the reload journal, whose rollback arm calls `Apply` a second time.
- [ ] `internal/component/vpp/conn.go` - `(*Connector).NewChannel` delegates to `c.conn.NewAPIChannel()` on the one shared govpp `Connection`.
- [ ] `vendor/go.fd.io/govpp/core/channel.go` - `SetReplyTimeout` writes the field; `receiveReplyInternal` substitutes `maxInt64` for a value at or below zero; `ErrReplyTimeout` is what it returns when the timer fires; `Reset` does not touch the field.
- [ ] `vendor/go.fd.io/govpp/core/connection.go` - `DefaultReplyTimeout` is 0, and the `channelPool` factory installs it on every freshly allocated channel.
- [ ] `mk/test-integration.mk` - `ZE_QEMU_INTEGRATION_PKGS` derives from the `integration && linux` tag; the `ze-qemu-integration-test` recipe appends `./internal/plugins/firewall/vpp/...` explicitly, and the comment above the variable gives the reason: that package's tests are linux-tagged but not integration-tagged, and still need a Linux GOOS to compile.
- [ ] `internal/component/traffic/yang/ze-traffic-control-conf.yang` - the config tree is `traffic / control / backend` and `traffic / control / interface`. No `environment` container exists under it.

**Behavior to preserve:**

- `Apply` keeps its signature, its lock, its connector wait, its 5s `waitConnectedTimeout`, its deferred `ch.Close()`, and its error wrapping with the `traffic-vpp:` prefix.
- `govppOps` keeps its single `ch` field and all eleven method bodies unchanged.
- `applyWithOps` keeps its signature and its behavior. Every test in `apply_test.go` stays green and unedited.
- The traffic `Backend` contract keeps no timeout sentinel. A reply-deadline failure surfaces as the wrapped govpp error, exactly as any other VPP failure does today.
- `backend_other.go` keeps the non-Linux build of the package compiling.

**Behavior to change:**

- The channel that `Apply` sends on carries a bounded reply deadline, so a request
  VPP accepts and never answers returns an error instead of blocking forever.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- An operator commits a `traffic { control { backend vpp; interface ... } }`
  configuration. The traffic plugin's config-apply handler
  (`internal/component/traffic/register.go`) calls `Backend.Apply`, and the reload
  journal calls it again on reload and on rollback.
- A second entry is the process environment: `ze.traffic.vpp.reply-timeout`, read
  once per ops construction.

### Transformation Path

1. `(*backend).Apply` takes `b.mu` and resolves the VPP connector.
2. `Apply` calls `conn.NewChannel()`, which returns a pooled govpp `Channel` whose
   `replyTimeout` is whatever its previous owner left, or 0 for a fresh one.
3. `newGovppOps` reads the env knob, clamps it, calls `SetReplyTimeout` on the
   channel, and returns the ops value. This step is what the spec adds.
4. `applyWithOps` drives the apply through the `vppOps` interface: interface dump,
   policer add or delete, classify table and session, policer-classify binding.
5. Each `govppOps` method calls `ReceiveReply`, which now returns
   `core.ErrReplyTimeout` when VPP stays silent past the deadline.
6. The error travels back through `applyWithOps` and `Apply` to the traffic
   component, which fails the config apply or the reload and runs its rollback.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ VPP dataplane | govpp binary API over the shared socket connection; the deadline bounds one request-reply round trip | Yes -- `TestNewGovppOpsBindsReplyTimeout` asserts the value installed on the `api.Channel` the facade wraps, for every operator input |
| Process environment ↔ plugin | `env.GetDuration` on `ze.traffic.vpp.reply-timeout`, registered by `env.MustRegister` at package init | Yes -- `ze env list` inside the QEMU VM prints `ze.traffic.vpp.reply-timeout duration 10s` beside the firewall key, from a `ze_core ze_vpp` build |
| Traffic component ↔ backend | the `traffic.Backend` interface; an error returns, and no new sentinel type crosses | Yes -- `grep -rn "component/firewall" internal/plugins/traffic/` returns nothing, and no error type is added |

### Integration Points

- `(*backend).Apply` (`backend_linux.go`) - the one call site that constructs the ops value.
- `govppOps` (`ops_linux.go`) - the type the constructor returns; unchanged.
- `env.MustRegister` and `env.GetDuration` (`internal/core/env`) - the knob.
- `ze-qemu-integration-test` (`mk/test-integration.mk`) - the target that must name this package so a darwin checkout still runs its Linux-only tests.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `(*backend).Apply` still opens the channel and still calls `applyWithOps`; the constructor sits between them and adds no layer. `applyWithOps` keeps its signature and body |
| No unintended coupling (components stay isolated) | Yes | `internal/plugins/traffic/vpp` imports `go.fd.io/govpp/api` and `internal/core/env` and nothing else new. `maxReplyTimeout` is stated as `60 * time.Second` rather than imported from `firewall.MaxBackendDeadline`, so no traffic-to-firewall dependency is created |
| No duplicated functionality (extends existing, does not recreate) | Yes | The clamp reuses `env.GetDuration` and Go's `min`/`max`; no timeout helper, no error type and no metric is added. The firewall sibling's file is mirrored in SHAPE, which is the point of D-2, not shared through a new abstraction with two users |
| Zero-copy preserved where applicable (refs, not copies) | N-A | The change is a control-plane call made once per apply. No buffer, no wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The only registration is `env.MustRegister` in the plugin's own file. No core package names the key: `ze env list` and completion reach it through `env.Entries()` and `internal/core/envcatalog` |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `(*Channel).Reset` does not clear `replyTimeout`, so binding once per ops construction is required rather than optional | `vendor/go.fd.io/govpp/core/channel.go` read 2026-08-11: `Reset` drains `reqChan` and `replyChan` and touches no other field | binding at connect time would be enough, and the constructor would be redundant machinery | re-read the vendored `Reset` body during the implementation audit | confirmed 2026-08-23: `(*Channel).Reset` (`vendor/go.fd.io/govpp/core/channel.go`) drains `reqChan` and `replyChan` and writes no other field. `receiveReplyInternal` in the same file reads `ch.replyTimeout` and substitutes `maxInt64` when the value is at or below zero. `DefaultReplyTimeout` is `time.Duration(0)` (`vendor/go.fd.io/govpp/core/connection.go`) and the `channelPool` factory installs it on every freshly allocated channel |
| A-2 | `(*backend).Apply` is the only producer of a `govppOps` in the package | `grep -rn 'govppOps{' internal/plugins/traffic/vpp/` returns one hit in `backend_linux.go`, and `grep -rn 'NewChannel()' internal/plugins/traffic/` returns one hit in the same function | a second call site would keep an unbounded path alive and the constructor would not close the hole | repeat both greps during the implementation audit | confirmed 2026-08-23: `govppOps{` returns one hit and `NewChannel()` returns one hit, both inside `(*backend).Apply` (`internal/plugins/traffic/vpp/backend_linux.go`) |
| A-3 | A clamped duration in the range 1s to 60s is right for traffic as well as for firewall | `docs/architecture/core-design.md` records a 10s default and a 1..60s clamp for both firewall backends; the traffic apply is a smaller message set on the same socket | too low a ceiling would fail a legitimate slow apply on a loaded VPP | the traffic apply's message count is bounded by the interface count; record the observed apply duration from the QEMU run | confirmed 2026-08-23: `docs/architecture/core-design.md` and `docs/guide/firewall.md` both publish 10s with a 1s to 60s range for the firewall VPP knob. The traffic apply's message count is bounded by the interface count, and the ceiling is reachable through the env knob with no rebuild |
| A-4 | No YANG leaf is owed, because this is a safety cap rather than an operator setting | `ai/rules/config.md` decision table, "Is it a safety cap that should never be tuned in production? Env var only"; both firewall knobs are env-only and neither appears in any `.yang` file (grep 2026-08-11) | operators would need a config leaf and the env-only key would be a promotion debt | grep the `.yang` tree for `reply-timeout` during the audit; confirm both firewall knobs are still env-only | confirmed 2026-08-23: `grep -rn "reply-timeout" --include="*.yang" .` returns nothing, so both firewall knobs are still env-only |
| A-5 | `env.MustRegister` inside a `//go:build linux` file registers the key on Linux only, so `ze env list` on darwin does not show it | the firewall knob has the same placement and the same consequence | the key would be invisible on the platform an operator reads the docs on | run `ze env list` on Linux and confirm the key appears; record that darwin does not carry it | confirmed 2026-08-23, and REFINED: the linux tag is only half the gate. `feature-gates.txt` puts `internal/plugins/traffic/vpp` behind `ze_vpp`, so a `ze_core`-only build lists neither this key nor the firewall one. Measured: a VM build with `-tags ze_core` printed 95 env rows and no vpp key at all; a `ze_core ze_vpp` build printed both. `ze_vpp` is default-on (`ZE_FEATURES` derives from every `ze_*` tag in `feature-gates.txt`), so a shipped `ze` carries it. Same placement and same consequence as the `ze.firewall.vpp.reply-timeout` entry in `internal/plugins/firewall/vpp/timeout_linux.go` |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The deadline fires on a legitimately slow apply and turns a working configuration into a failed one | a traffic apply fails with a reply-timeout error on a VPP that later answers | the 60s ceiling is reachable through the env knob without a rebuild; the default matches the firewall backend that has run with it since 2026-08-10 |
| R-2 | The test binds the deadline on a fake and never proves the production path, because `Apply` needs a live VPP connection that a unit test cannot make | the test passes while `Apply` still builds the inline literal | RETIRED 2026-08-23. The planned mitigation was a one-off audit grep, and review round 1 judged that too weak for what the code's own comments were claiming: an audit step nobody repeats is not an invariant. `TestGovppOpsIsBuiltOnlyByItsConstructor` (`internal/plugins/traffic/vpp/ops_construction_test.go`) now parses the package's own sources and fails on a `govppOps` built outside the constructor, and fails again when it finds none at all. The link the unit test could not make is made by a test rather than by a habit |
| R-3 | The QEMU line is edited and the package still does not run there, because the recipe's shell command accepts a package pattern that matches nothing | `make ze-qemu-integration-test` passes without naming a traffic test | read the run output for a line naming `internal/plugins/traffic/vpp`, not only for a zero exit |
| R-4 | The reviewer reads the missing error tagging as an omission rather than a decision | a review finding proposing a traffic equivalent of `firewall.ErrKernelTimeout` | Key Design Decisions records why the sentinel is firewall-only: it exists to drive a metric and a rollback skip, and traffic has neither |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A too-short deadline fails a traffic-control config apply or reload against a slow VPP, and the reload journal then runs its rollback arm, which calls `Apply` again and burns a second deadline. Nothing outside the traffic plugin is affected: `b.mu` is the backend's own lock, unlike the firewall's process-wide `reconcileMu`. **The bound is per ROUND TRIP, not per apply**, which the first draft of this spec did not say. One `Apply` issues the interface dump, the policer dump, a policer add and an output bind for each class, and a classify table, session and bind for each steering interface; the undo list issues more on failure. Against a VPP that accepts every request and answers none, `b.mu` is held for roughly the request count times the deadline, which is minutes for a many-interface configuration rather than the 10 seconds the default reads like, and the caller's `ctx` cannot cut it short because `Channel.ReceiveReply` takes none. What the deadline buys is TERMINATION, not a 10-second ceiling on the apply. |
| How is it reverted? | Single commit revert. No config migration, no wire-visible change, no persisted state. An operator can also raise the value at runtime through `ze.traffic.vpp.reply-timeout` without a rebuild. |
| Who else touches this path? | `plan/spec-finish-vpp-stub.md` (Status `ready`) plans apply-tier traffic coverage against the VPP stub, and its AC-11 drives the same `Apply`. It edits the stub and the `.ci` suite, not this package: its design struck `internal/plugins/traffic/vpp/backend_linux.go` from Files to Modify on 2026-07-10. `plan/spec-fixit-firewall-concurrency-deadlock.md` owns the firewall sibling and stays open for its phase 1. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `(*backend).Apply` obtains a pooled channel and builds the ops facade | → | `newGovppOps` calls `SetReplyTimeout` before it returns | TWO tests, because neither half is the row on its own. `TestNewGovppOpsBindsReplyTimeout` (`internal/plugins/traffic/vpp/timeout_linux_test.go`) proves the constructor installs the deadline, on its own fake channel, which its comment says it does not extend to the production one. `TestGovppOpsIsBuiltOnlyByItsConstructor` (`internal/plugins/traffic/vpp/ops_construction_test.go`) is what connects that to `Apply`: it fails if anything in the package builds a facade some other way. <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| The operator sets `ze.traffic.vpp.reply-timeout` in the process environment | → | `vppReplyTimeout` reads and clamps it | `TestVppReplyTimeoutBounds` in `internal/plugins/traffic/vpp/timeout_linux_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A darwin checkout runs the Linux-only suite | → | `ze-qemu-integration-test` names this package | `TestNewGovppOpsBindsReplyTimeout` runs inside the QEMU VM; the run output names `internal/plugins/traffic/vpp` |

N/A for a `.ci` row: the apply tier of the traffic VPP backend has no functional
test that reaches VPP. See Functional Tests below for the reason and the
dependency.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The traffic VPP backend obtains a govpp channel and builds its ops facade | The channel carries a non-zero reply deadline before any request is sent, so a request VPP accepts and never answers returns `core.ErrReplyTimeout` instead of blocking forever |
| AC-2 | `ze.traffic.vpp.reply-timeout` is unset | The deadline is 10 seconds |
| AC-3 | `ze.traffic.vpp.reply-timeout` is set to zero, to a negative duration, to a value below 1 second, above 60 seconds, or to text that does not parse as a duration | The deadline clamps into the range 1 second to 60 seconds. Zero is never installed, because zero is govpp's spelling of "no deadline" and is the defect being removed |
| AC-4 | `ze env list` runs on Linux | `ze.traffic.vpp.reply-timeout` appears, with its type, its default and a description that says what an unbounded call would cost |
| AC-5 | `make ze-qemu-integration-test` runs from any host | The run output names `internal/plugins/traffic/vpp` and its tests pass inside the VM, so the Linux-only proof of AC-1 is not reachable only from a Linux workstation |
| AC-6 | The `SetReplyTimeout` call is removed from the constructor and the tests are re-run | `TestNewGovppOpsBindsReplyTimeout` fails and names the missing call. The RED output is pasted into the TDD checklist |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits a traffic-control configuration with `backend vpp` against a VPP that stops answering | config apply (`internal/component/traffic/register.go`) → `(*backend).Apply` → `newGovppOps` binds the deadline → `applyWithOps` → `ReceiveReply` returns `core.ErrReplyTimeout` → the config apply fails with a `traffic-vpp:` error instead of hanging | `TestNewGovppOpsBindsReplyTimeout` |
| 2 | raises the deadline for a loaded VPP by setting `ze.traffic.vpp.reply-timeout` to 30s | env read at ops construction → `vppReplyTimeout` clamp → `SetReplyTimeout` | `TestVppReplyTimeoutBounds`, `TestNewGovppOpsBindsReplyTimeout` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNewGovppOpsBindsReplyTimeout` | `internal/plugins/traffic/vpp/timeout_linux_test.go` | AC-1: the constructor calls `SetReplyTimeout` with the clamped value on the channel it was given, and returns an ops value wired to that same channel. Uses a recording fake that implements `api.Channel`, records the duration, and panics on every other method | PASS in the QEMU VM. Written table-driven over four operator inputs rather than one, so the value asserted is the one INSTALLED at every boundary, not only the one computed. Shown RED with the `SetReplyTimeout` call removed |
| `TestVppReplyTimeoutBounds` | `internal/plugins/traffic/vpp/timeout_linux_test.go` | AC-2 and AC-3: the default with the key unset, and the clamp for every out-of-range and unparseable input. Table-driven, one row per boundary below | PASS in the QEMU VM, nine rows covering every cell of the boundary table below |
| `TestGovppOpsIsBuiltOnlyByItsConstructor` | `internal/plugins/traffic/vpp/ops_construction_test.go` | AC-1's precondition: the constructor is the ONLY way a `govppOps` comes into being, so the deadline it installs cannot be skipped. Parses every `.go` file in the package with `go/parser` and reports each composite literal or `new()` of `govppOps`, with its file and enclosing function | Added in review round 1, replacing the audit grep R-2 planned. PASS. Untagged, so it runs on darwin as well as Linux and needs no VM. Shown RED twice: once with the call site reverted to `&govppOps{ch: ch}`, once with the constructor returning `nil` so no site exists at all |

**What the tests pin, stated plainly.** They pin that the ops facade the
production path builds carries a bounded, non-zero deadline on its own channel,
for every operator input. They do NOT pin that a wedged VPP unblocks. The wait
itself is `receiveReplyInternal` inside vendored govpp, which no fake channel can
stand in for, and the traffic package has no harness that reaches a live VPP.
The stronger claim needs a stub that accepts a request and never answers, which
the Functional Tests row below shows exists in no spec today. It is NOT
`plan/spec-finish-vpp-stub.md` AC-11: that asserts `Apply` completes, which a
deadline test cannot be built on.

`TestGovppOpsIsBuiltOnlyByItsConstructor` pins a narrower thing again, and its
bound is deliberate. It reads the three forms that name `govppOps` directly, so
it catches the regression that actually occurred, an inline literal at the call
site. It does not see a facade built as part of another value, and it therefore
does not make an unbounded one unconstructible. The function's own comment
carries the list.

**AC-6 discrimination evidence.** Taken with the `SetReplyTimeout` call deleted
from `newGovppOps`, re-run with `-count=1` inside the QEMU VM. The mutation was
confirmed applied before the run (`diff` against a pristine copy returned the
deleted line; `grep -c "ch.SetReplyTimeout"` returned 0), and the file was
restored afterwards by copying the pristine copy back, verified byte-identical
with `shasum -c`.

```
--- FAIL: TestNewGovppOpsBindsReplyTimeout (0.00s)
    --- FAIL: TestNewGovppOpsBindsReplyTimeout/an_explicit_value_reaches_the_channel (0.00s)
        timeout_linux_test.go:123: SetReplyTimeout was never called: the channel keeps govpp's disabled default
    --- FAIL: TestNewGovppOpsBindsReplyTimeout/unset_installs_the_default (0.00s)
        timeout_linux_test.go:123: SetReplyTimeout was never called: the channel keeps govpp's disabled default
    --- FAIL: TestNewGovppOpsBindsReplyTimeout/zero_is_clamped_before_it_reaches_the_channel (0.00s)
        timeout_linux_test.go:123: SetReplyTimeout was never called: the channel keeps govpp's disabled default
    --- FAIL: TestNewGovppOpsBindsReplyTimeout/above_the_ceiling_is_clamped_before_it_reaches_the_channel (0.00s)
        timeout_linux_test.go:123: SetReplyTimeout was never called: the channel keeps govpp's disabled default
FAIL
FAIL	github.com/ze-software/ze/internal/plugins/traffic/vpp	0.025s
```

The failure names the missing call rather than a compile error, which is what
AC-6 asks for. `TestVppReplyTimeoutBounds` stayed green through the mutation,
which is the right split: the clamp helper was untouched, and only the
installation was broken.

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.traffic.vpp.reply-timeout` | 1s to 60s | 1s and 60s both accepted unchanged | `0s`, `-1s` and `500ms` clamp up to 1s | `61s` and `10m` clamp down to 60s |
| `ze.traffic.vpp.reply-timeout` (unparseable) | N-A | N-A | an empty value and `not-a-duration` fall back to the 10s default | N-A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Not applicable, and the enabler is not written anywhere yet | - | No `.ci` can reach this code path today. The VPP tests in `test/traffic/` stop at the verify tier: `traffic-vpp-not-connected.ci` and the two `traffic-vpp-accept-*` tests prove their point through the ABSENCE of VPP, and `traffic-vpp-accept-dscp-filter.ci` records the reason in its own body, naming `plan/spec-finish-vpp-stub.md` as the blocker. **Correction made during implementation, 2026-08-23: that spec is not the enabler for THIS behavior.** Its AC-11 asserts that `Apply` COMPLETES against the stub, and a stub that answers proves nothing about a reply deadline. What this behavior needs is a stub mode that ACCEPTS a request and never answers it, plus an assertion that the apply returns a reply-timeout error inside roughly the configured deadline instead of hanging. No row in `spec-finish-vpp-stub` asks for that, and neither does this spec, so naming AC-11 as the enabler pointed at something that cannot deliver. Writing that stub mode is stub work and stays outside a deadline fix; what changes here is that the spec stops claiming a route it does not have. The observable proof shipped is the unit tests above, run inside the QEMU VM (AC-5), which is the same proof the firewall sibling shipped | <!-- doc-links: ignore (the enabling row exists in no spec; this cell is what says so) --> |

### Interop Tests (Scope: protocol)

Not applicable with a reason: the VPP binary API is a local control interface
between ze and its own dataplane, not a wire protocol between routing daemons.
There is no third-party peer to interoperate with. This change alters no byte on
any wire; it installs a client-side deadline.

## Files to Modify

- `internal/plugins/traffic/vpp/backend_linux.go` - the last statement of `(*backend).Apply` uses the constructor instead of the inline `&govppOps{ch: ch}` literal. Its `// Design:` annotation points at `docs/architecture/traffic/fw-7-traffic-vpp.md`, which describes the apply path and not the channel lifetime, so that doc needs no edit; the seam doc does.
- `mk/test-integration.mk` - the `ze-qemu-integration-test` recipe names `./internal/plugins/traffic/vpp/...` beside the firewall package, and the comment above `ZE_QEMU_INTEGRATION_PKGS` names both for the same reason.
- `docs/architecture/traffic/fw-7b-backend-hardening.md` - the "The `vppOps` seam" section records that the production implementation is built by a constructor that binds the reply deadline, and why the binding sits there.

## Files to Create

- `internal/plugins/traffic/vpp/timeout_linux.go` - the three constants, the `env.MustRegister` entry, the clamping reader, and the constructor. <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- `internal/plugins/traffic/vpp/timeout_linux_test.go` - the recording channel fake and the two tests. <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- `internal/plugins/traffic/vpp/ops_construction_test.go` - added in review round 1, not planned at design time. The source-scanning guard that makes D-1 an invariant instead of a convention. Deliberately UNTAGGED: the scan is textual, so it runs on every GOOS and catches a violation in the fast darwin unit run rather than only in the VM. <!-- doc-links: ignore (added during implementation; the file is described in the row above it) -->

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `ai/rules/config.md` routes a safety cap that is never tuned in production to an env var. Both firewall deadline knobs are env-only and neither appears in any `.yang` file. A YANG leaf would also have to live under an `environment` container that `ze-traffic-control-conf.yang` does not have |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | No | The knob is read from the environment; `ze env get` and `ze env list` already serve every registered key |
| CLI grammar (keyword before value) | N-A | No CLI command is added |
| Editor autocomplete | Yes, automatic | `internal/core/envcatalog` merges `env.Entries()`, so a registered non-private key reaches shell and CLI completion with no per-key wiring |
| Functional test for new RPC/API | No | No RPC or API is added. See Functional Tests for why no `.ci` can reach this path today |
| Env var registration | Yes | `env.MustRegister` for `ze.traffic.vpp.reply-timeout` in `internal/plugins/traffic/vpp/timeout_linux.go`, type `duration`, default `10s`. There is no YANG leaf to match, so the "YANG leaf under `environment/`" half of the rule does not apply <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| Pipe completeness | N-A | No command output is added |
| Doctor check for runtime dependencies | No | No new file path, socket, service, module, port or certificate. The VPP connection itself already has its doctor coverage |
| Prometheus counters/metrics | No | The firewall counter `ze_firewall_apply_timeout_total` exists because `firewall.ErrKernelTimeout` classifies the failure for `observeApply`. The traffic component has no apply metric and no error classifier, so a counter here would need a sentinel, a classifier and a histogram that nothing reads. That is a separate change with its own justification, not a rider on a deadline fix |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | An internal safety bound, not a feature |
| 2 | Config syntax changed? | No | No YANG leaf, no parser change |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | The traffic plugin's registration, commands and schema are untouched |
| 6 | Has a user guide page? | No | `docs/guide/` has no traffic-VPP page; the knob is a safety cap and not operator-facing |
| 7 | Wire format changed? | No | A client-side deadline changes no byte |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The VPP binary API is governed by no RFC |
| 10 | Test infrastructure changed? | Yes, conditional | `docs/functional-tests.md`, if it spells out the QEMU package list. Grep it for `ze-qemu-integration-test` during the audit and update the list if the packages are named there |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/traffic/fw-7b-backend-hardening.md`, "The `vppOps` seam": the production implementation is now built by a constructor that binds the reply deadline. `docs/architecture/core-design.md` is NOT edited: its deadline table sits under "Firewall reconcile concurrency" and states an obligation of the firewall `Backend` contract, which this backend does not implement |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | See the Integration Checklist row |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing new registers except the env entry, which reaches `ze env list` through the existing registry |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for `source: internal/plugins/traffic/vpp` and for `backend_linux.go` during the audit, and correct any claim about how the ops value is built |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No syntax changed, so no example can be stale |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create the timeout file and its test, and prove the test fails.
   - Tests: `TestNewGovppOpsBindsReplyTimeout`, `TestVppReplyTimeoutBounds`
   - Files: `internal/plugins/traffic/vpp/timeout_linux.go`, `internal/plugins/traffic/vpp/timeout_linux_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
   - Verify: the tests exist and fail because the constructor does not yet bind the deadline. Paste the RED output. This is also the AC-6 discrimination evidence, so capture it before the fix rather than reconstructing it after.
2. **Phase: bind the deadline** -- write the constants, the env entry, the clamping reader and the constructor.
   - Tests: the two above turn green
   - Files: `internal/plugins/traffic/vpp/timeout_linux.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
   - Verify: `make ze-unit-pkg-test PKG=./internal/plugins/traffic/vpp`
3. **Phase: use the constructor** -- replace the inline literal in `(*backend).Apply`.
   - Tests: every test in `apply_test.go` stays green and unedited
   - Files: `internal/plugins/traffic/vpp/backend_linux.go`
   - Verify: `grep -rn 'govppOps{' internal/plugins/traffic/vpp/` returns exactly one hit, inside the constructor. Then `make ze-lint-changed`
4. **Phase: QEMU reach** -- name the package in the QEMU target.
   - Tests: the two unit tests run inside the VM
   - Files: `mk/test-integration.mk`
   - Verify: `make ze-qemu-integration-test`, and read the output for a line naming `internal/plugins/traffic/vpp`. A zero exit alone does not satisfy AC-5 (R-3)
5. **Phase: documentation** -- the seam doc, and any stale source anchor the checklist grep finds.
   - Files: `docs/architecture/traffic/fw-7b-backend-hardening.md`
   - Verify: `make ze-doc-verify`
6. **Full verification** -- `make ze-precommit-verify`, then set Status to `verification`, commit, and stop. Closure belongs to a later Opus 5 session (`Handoff | verify`).

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 to AC-6 each have an implementation or an evidence line naming the file and the symbol |
| Feature completeness | Both user stories run; no call site builds an ops value that skips the constructor |
| Correctness | The clamp refuses zero rather than honoring it; the default matches the registered `Default` string; the reader falls back to the default on unparseable input rather than to zero |
| Naming | The env key is `ze.traffic.vpp.reply-timeout`, matching `ze.firewall.vpp.reply-timeout` segment for segment; the constants are named as in the sibling so the two files read alike |
| Data flow | The deadline is installed on the channel BEFORE the ops value is returned, and no request can be sent between the two |
| Rule: `ai/rules/interop-and-goal-validation.md` | The RED output for AC-6 is pasted, and it names the missing `SetReplyTimeout` rather than a compile error |
| Rule: `ai/rules/simplicity.md` | No error sentinel, no metric, no interface and no option is added beyond the one knob. The change is a constructor, a clamp and one call site |
| Registration over hardcoding | The env entry registers through `env.MustRegister` and reaches `ze env list` and completion through the existing registry; no key is listed in a core package |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The constructor binds the deadline | `grep -n 'SetReplyTimeout' internal/plugins/traffic/vpp/timeout_linux.go` |
| No unbounded construction survives | `grep -rn 'govppOps{' internal/plugins/traffic/vpp/` returns one hit, inside `newGovppOps` |
| The env key is registered | `bin/ze env list` on Linux lists `ze.traffic.vpp.reply-timeout` |
| The package runs in QEMU | `make ze-qemu-integration-test` output names `internal/plugins/traffic/vpp` |
| The test discriminates | The RED output pasted under TDD, taken with the `SetReplyTimeout` call absent |
| The seam doc records the binding | `git diff docs/architecture/traffic/fw-7b-backend-hardening.md` is non-empty |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The env value is operator-supplied text. Every path clamps into the range 1s to 60s, the unparseable one included, and no path can install zero |
| Resource exhaustion | The deadline REMOVES an exhaustion path: an unbounded call held `b.mu` and a pooled channel for the life of the process |
| Error leakage | The returned error carries the govpp timeout text and the configured duration. Neither is a secret |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| The QEMU target passes without naming the package | R-3: the package pattern matched nothing. Fix the pattern; do not accept the zero exit |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A pooled channel makes an unbounded default worse than it reads. Because
  `(*Channel).Reset` leaves `replyTimeout` alone and the pool is shared across every
  plugin on the one `Connection`, the traffic backend's current deadline is
  whichever value the last owner set. That is why the bound belongs in the
  constructor rather than at connect time: the constructor is the only place that
  runs once per use of a channel.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: Bind the deadline inside a constructor for the ops facade | (a) call `SetReplyTimeout` in `Apply` next to `NewChannel`; (b) set it once in `(*Connector).NewChannel` for every caller | (a) is one line further from the type that owns the channel, and a second ops call site would miss it. (b) puts one plugin's policy on every plugin's channel and denies each backend its own value. The sibling chose the constructor and gave the reason: a computed but uninstalled deadline is indistinguishable from having none. **Amended in review rounds 1 and 2:** a constructor only delivers that if it is the only way to build the type, and Go does not give that for an unexported struct in its own package. `TestGovppOpsIsBuiltOnlyByItsConstructor` is what holds it; without the test D-1 buys a convention, which is the weaker half of the fix. Round 2 BOUNDED the claim, after the reviewer ran the guard against mutated copies: it holds against every DIRECT construction (composite literal, `new()`, bare `var`), which is the form the defect actually took, and not against a facade built as part of another value, which would need `go/types` rather than a parse. D-1 is a ratchet on the regression, not a proof that an unbounded facade cannot exist |
| D-2: A new `timeout_linux.go`, not an addition to `ops_linux.go` | put the constructor beside the `govppOps` type it returns | The sibling's layout is the point. Two backends carrying the same obligation should read the same way, and `ops_linux.go` is the message-wrapping topic, one method per VPP request. A file named for the concern also carries the `// Design:` annotation the seam doc needs |
| D-3: Env-only knob, no YANG leaf | a leaf under a new `environment` container in `ze-traffic-control-conf.yang` | `ai/rules/config.md` routes a safety cap that is never tuned in production to an env var, and both firewall deadline knobs are env-only. Adding a container to a config tree that has none, for a value an operator sets once in an emergency, is machinery the problem does not need |
| D-4: No timeout sentinel on the traffic `Backend` contract | mirror `firewall.ErrKernelTimeout` with a traffic equivalent | The firewall sentinel exists to drive two decisions: `ze_firewall_apply_timeout_total` counts wedged reconciles, and ddos-local skips its rollback reconcile on it. Traffic has neither consumer. A sentinel nothing reads is an abstraction with no user |
| D-5: Default 10s, clamp 1s to 60s, zero refused | pick a traffic-specific value | An operator should meet one pair of bounds across every ze dataplane. A third number would need a reason, and the traffic apply is a smaller message set on the same socket, so there is none. **Corrected in review round 1:** `docs/architecture/core-design.md` publishes that pair under "Firewall reconcile concurrency", where "both" means the two FIREWALL backends. This backend is not in that table, so it MATCHES the numbers deliberately rather than inheriting them, and the code comment says so |
| D-7: State `maxReplyTimeout` as `60 * time.Second` rather than import `firewall.MaxBackendDeadline` | import the firewall constant, so the two ceilings can never drift | Recorded in review round 1, which upheld the decision on a better reason than the tier argument first given. That constant's own doc says it exists so THREE firewall things agree: the nft clamp, the vpp clamp, and the last finite bucket of the apply-latency histogram (`internal/component/firewall/metrics.go`). Traffic has none of the three, so the import buys coupling to a component this plugin has no other relationship with, and buys no agreement in return. Drift is a two-line correction; a dependency is permanent |
| D-6: Name the package in `ze-qemu-integration-test` rather than tag the tests `integration && linux` | add the `integration` tag to the new test file | The `integration` tag means the test needs kernel capabilities (`ai/rules/platform-linux.md`). This test uses a fake channel and makes no syscall, so the tag would be a false claim. The firewall package is named explicitly for exactly this reason, and the comment above `ZE_QEMU_INTEGRATION_PKGS` records it |

## Known Limitations

- **FOUR more VPP backends carry the same defect and are OUT OF SCOPE here.** The
  fourth was found during this implementation and was named by nothing before it:
  `newVPPBackend` (`internal/component/ike/dataplane/vpp.go`) takes a channel and
  binds no deadline, and it HOLDS that channel for the backend's lifetime rather
  than per apply, so every SA and SPD request waits on it. It is recorded in
  `plan/journal/guard-added-to-one-half-of-a-pair.md` and belongs with the same
  owner decision as the three below; `plan/spec-vpp-reply-deadline-iface-fib-static.md`
  does not reach it.
- **Three more VPP backends carry the same defect and are OUT OF SCOPE here.**
  `grep -rn 'SetReplyTimeout' --include=*.go internal/` on 2026-08-11 returns one
  production call, the firewall one. `internal/plugins/iface/vpp` (through
  `ensureChannel`), `internal/plugins/fib/vpp` (through its plugin registration)
  and `internal/plugins/static/vpp` all obtain govpp channels and never bind a
  deadline; each already has a test fake whose `SetReplyTimeout` is an empty
  method. This spec fixes the one backend the deferral row named. The wider set is
  recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md` and needs its own
  decision from Thomas, because those three have different callers, different locks
  and different blast radii from traffic's.
- **No functional `.ci` proves the deadline end to end, and the enabler is written
  in no spec.** The apply tier of the traffic VPP backend has no test that reaches
  VPP at all. `plan/spec-finish-vpp-stub.md` AC-11 would give the apply tier a
  test, but not THIS one: it asserts that `Apply` completes against the stub, and
  a stub that answers cannot exercise a reply deadline. Proving this behavior end
  to end needs a stub mode that accepts a request and never answers, plus an
  assertion that the apply fails with a reply-timeout error inside roughly the
  configured deadline. Neither `spec-finish-vpp-stub` nor this spec carries that
  row. Recorded here rather than fixed because writing the stub mode is stub work;
  what this spec owed was to stop naming an enabler that cannot deliver.
- The env key registers inside a Linux-only file, so `ze env list` on darwin does
  not show it. The firewall knob has the same property, so this is the established
  behavior for a Linux-only backend knob rather than a new gap.

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1 to AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: the 2026-08-07 traffic row in `plan/deferrals/fixit-firewall-concurrency-deadlock.md` names this spec and is closed by this work

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior, or the recorded reason none is reachable
- [ ] Interop tests for protocol features (or N-A with a reason)

### Handoff (`Handoff | verify`)

- [ ] Status set to `verification` before the commit
- [ ] ONE commit: code, tests, docs, the deferral shard row, and this spec. No `plan/learned/` file and no spec removal, or `commit_helper.py` reads it as a closure commit
- [ ] `scripts/dev/spec-session.sh release`, report the SHA, then stop. A later Opus 5 session runs the Review Gate over the committed diff and closes
