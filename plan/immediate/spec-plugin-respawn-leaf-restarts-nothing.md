# Spec: plugin-respawn-leaf-restarts-nothing

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 1/4 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The owner replaced this spec's premise on 2026-09-06.** The original spec found
an operator-facing `respawn` leaf that reached no code and asked whether to build
it or refuse it. The owner answered a third way, in three messages on that day.

> "the plugin should inform us if it wishes to be restarted on failure, us to
> ignore it or cause the whole ze to stop and we should implement it."

> "fatal is for any plugin the user decides to use."

> "no the respawn must stay for exabgp compatibility but if a plugin declare that
> it can not restart then we can not start. exabgp bridge can restart because
> exabgp allows it."

So a plugin DECLARES what its own failure means, in three values, and the
`respawn` config leaf stays as the operator's request inside that declaration.

| Declared policy | On plugin failure | May the plugin be restarted? |
|-----------------|-------------------|------------------------------|
| `restart` | Ze starts the plugin again | Yes |
| `ignore` | Ze logs and carries on without it | No |
| `fatal` | Ze stops | No |

The declaration is a CONSTRAINT and the config leaf is a REQUEST inside it. A
config asking to respawn a plugin that declares it cannot be restarted is a
disagreement Ze cannot honor, so the daemon does not start, and the refusal
names both sides.

`fatal` is open to any plugin, whether Ze ships it or an operator wrote it.
Configuring a plugin is accepting its terms.

**What Ze did before this spec.** `ExtractPluginsFromTree`
(`internal/component/config/loader.go`) read `run`, `use`, `encoder` and
`timeout` over the `external` list and never read `respawn`, so
`PluginConfig.Respawn` was false for every configured plugin.
`ProcessManager.Respawn` (`internal/component/plugin/process/manager.go`) tested
that field and returned `ErrRespawnNotEnabled`, so it refused every plugin in a
running daemon. Its only non-test caller is `(*Server).restartPlugin`
(`internal/component/plugin/server/reload_tx.go`), reached when a config-reload
rollback ack reports `CodeBroken`, so that path could never succeed either. And
nothing watched a plugin process exit, so no code called `Respawn` after a crash
at all. Three dead behaviors: the leaf, the crash restart, and the reload
restart.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the five-stage handshake and
      the Stage-1 declaration set the policy joins.
  -> Decision: Stage 1 (`declare-registration`) is the one declaration surface
     both internal and external plugins use, so the policy needs no second,
     compile-time spelling in `registry.Registration`.
  -> Constraint: the engine learns nothing about a plugin before Stage 1
     completes, so a plugin that fails at or before Stage 1 has declared no
     policy and Ze cannot apply one.
- [ ] `docs/architecture/system-architecture.md` - publishes `respawn true;` in
      two worked configuration blocks, the first annotated "Restart if crashes".
  -> Constraint: the annotation is only half true once the plugin decides, so
     both blocks change in the same work as the code.
- [ ] `docs/architecture/exabgp-bridge.md` - the bridge that runs ExaBGP scripts.
  -> Decision: ExaBGP's own process block carries `respawn`, default true
     (`exabgpRespawn`, `internal/exabgp/migration/migrate.go`, reading
     `src/exabgp/configuration/process/__init__.py`). That lineage is why the
     leaf stays, and why the bridge declares `restart`.

**Key insights:**
- The crash-loop bound already exists and is not new work: `RespawnLimit` 5 per
  `RespawnWindow` 60s, `MaxTotalRespawns` 20 cumulative, then `disabled[name]`,
  a `plugin-down` report warning and a log line (`process/manager.go`).
- `Server.signalShutdownRequested` releases `Server.Wait`, which is what
  `waitForServerDone` (`cmd/ze/hub/main.go`) blocks on, and `shutdownFunc`
  injects the SIGTERM the daemon's own teardown reads. `handleDaemonShutdown`
  (`server/system.go`) already uses exactly that pair, so a daemon stop reuses
  the existing route rather than inventing one.
- A sink error from Stage 1 reaches `proc.SetStartupError` unwrapped
  (`runStartupHandshake` returns it as-is) and `startupFailureError` wraps it
  with `%w`, so a sentinel error survives to the phase that decides what to do.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/loader.go` - `ExtractPluginsFromTree` never read
      the `respawn` leaf.
- [ ] `internal/component/plugin/types.go` - `PluginConfig` carried `Respawn` and
      `RespawnEnabled`, neither of which any producer set.
- [ ] `internal/component/plugin/process/manager.go` - `Respawn` refuses on
      `!cfg.RespawnEnabled && !cfg.Respawn`, raises the `plugin-crash` report,
      and enforces the two restart bounds.
- [ ] `internal/component/plugin/server/startup.go` - `runPluginPhase` step (d)
      started one runtime handler per running process, and that handler's return
      did nothing. `onRegistration` and `registrationFromRPC` carry a plugin's
      Stage-1 declaration onto the process.
- [ ] `internal/component/plugin/server/dispatch.go` -
      `handleSingleProcessCommandsRPC` is the runtime handler, and for a
      bridge-mode plugin it waited only on the server context and the process
      context.
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` is the Stage-1
      declaration, and `SignalsSessionReady` is the precedent for a voluntary
      per-plugin declaration an external process can make.
- [ ] `internal/exabgp/migration/migrate.go` - `exabgpRespawn` and
      `injectBridgePlugin`, which carry an ExaBGP process block's `respawn` into
      the bridge's own config.

**Behavior to preserve:**
- The two restart bounds, their `plugin-down` warning and the
  `ze_plugin_restarts_total` metric.
- `Server.restartPlugin`'s three ordered steps for a config-reload rollback.
- A plugin that declares nothing behaves as it did: it exits and stays down.
- The `respawn` leaf keeps its spelling and its place under
  `plugin { external <name> }`.

**Behavior to change:**
- A plugin declares a failure policy, and Ze acts on it.
- `ExtractPluginsFromTree` reads the `respawn` leaf into `PluginConfig.Respawn`.
- A config whose respawn request the plugin's declaration cannot meet stops the
  daemon at startup with a message naming both sides.
- `ProcessManager.Respawn` no longer refuses on config: the caller owns the
  policy decision, so the config-reload rollback restart can now succeed.
- A plugin's unexpected exit raises the `plugin-crash` report whatever the
  policy, rather than only on the restart path.

## Data Flow (MANDATORY)

### Entry Point
- A plugin's Stage-1 `declare-registration` message, carrying
  `failure-policy: "restart" | "ignore" | "fatal"`.
- The operator's `plugin { external <name> { respawn true; } }`.

### Transformation Path
1. The plugin writes `sdk.Registration{FailurePolicy: ...}` and calls
   `(*Plugin).Run` (`pkg/plugin/sdk/sdk.go`).
2. `PluginConn.sendDeclareRegistration` sends it; the engine's startup driver
   decodes it into `rpc.DeclareRegistrationInput`.
3. `engineStartupSink.onRegistration` (`server/startup.go`) refuses an unknown
   spelling and refuses a respawn request the declaration cannot meet, then
   `registrationFromRPC` copies the policy into
   `plugin.PluginRegistration.FailurePolicy`, which `proc.SetRegistration`
   stores on the `*process.Process`.
4. The plugin's runtime handler returns when the process ends.
   `Server.superviseProcess` then calls `Server.applyFailurePolicy`.
5. `pluginFailurePolicy` resolves the declaration and the config request into one
   of the three concrete outcomes, and the switch restarts, logs, or stops the
   daemon.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> Engine | `plugin { external { respawn } }` -> `PluginConfig.Respawn` | Yes |
| Plugin -> Engine | `declare-registration` JSON, new `failure-policy` key | Yes |
| Engine -> Process manager | `ProcessManager.Respawn(name)` | Yes |
| Engine -> Daemon | `shutdownFunc` + `signalShutdownRequested` | Yes |

### Integration Points
- `rpc.DeclareRegistrationInput` - the declaration joins the existing Stage-1 set.
- `process.ProcessManager.Respawn` - the bounded restart the `restart` policy uses.
- `Server.restartPlugin` - the handshake half of a restart, shared with the
  config-reload rollback path.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the policy travels the same Stage-1 path every other declaration takes; the leaf travels the same loader path as `run` and `encoder` |
| No unintended coupling | Yes | `process` gains no knowledge of the policy; the server decides and calls `Respawn` |
| No duplicated functionality | Yes | the restart bounds, the crash report and the metric are the existing ones |
| Zero-copy preserved where applicable | N-A | control plane, once per plugin generation |
| Registration over hardcoding | Yes | a plugin declares its own policy; no central list names a plugin |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | "cannot restart" is the plugin declaring anything other than `restart`, rather than a second, independent capability field | the owner's three values are mutually exclusive outcomes, and Ze restarts a plugin for exactly one reason, which is failure | a second boolean would be needed beside the policy | read at the producer: `Respawn` has one failure-path caller and one rollback caller, and neither wants "restartable but do not restart" | confirmed |
| A-2 | An in-process plugin's engine ending does not close its process context or the server context, so the exit needs a third channel | `process.go` `engineDone`, `dispatch.go` bridge-mode select | the exit would already be observed and no new channel is needed | `TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart` fails without `EngineDone` | confirmed |
| A-3 | `signalShutdownRequested` alone ends the daemon, because `waitForServerDone` blocks on `Server.Wait` | `waitForServerDone` and `waitLoop` in `cmd/ze/hub/main.go`, `Server.Wait` in `server.go` | `fatal` would need a second mechanism | read at the producer | confirmed |
| A-4 | A plugin that fails before Stage 1 completes has declared nothing | `startup.go` `onRegistration` is Stage 1 | a policy could be applied earlier | read at the producer | confirmed |
| A-5 | A Stage-1 sink error survives to `runPluginPhase` for `errors.Is` | `runStartupHandshake` returns the sink error unwrapped; `startupFailureError` wraps with `%w` | the disagreement would need a field on `Process` rather than a sentinel | read at both producers | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `restart` plugin that crashes on every start cycles the daemon | the `plugin-down` warning and `ze_plugin_restarts_total` climbing | the existing bounds: 5 per 60s, 20 cumulative, then the plugin is disabled and Ze carries on, which is the `ignore` outcome |
| R-2 | A `fatal` plugin an operator cannot keep running takes the router down on every start | the daemon stops naming the plugin in an ERROR line | the operator removes the plugin block, which is the consent the owner named |
| R-3 | An existing config carrying `respawn true;` against a plugin that declares nothing now stops the daemon where it used to load | the startup refusal names the plugin and both sides | intended, and it is the owner's "we can not start". The remedy is in the message: remove the leaf, or run a plugin that declares `restart` |
| R-4 | Removing the `cfg.Respawn` guard makes the reload-rollback restart succeed where it used to be refused | `TestRestartPluginRestartsABrokenPlugin` | intended: that path was dead and the spec's Task names it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a plugin restarts when it should not, stays down when it should not, or stops the daemon when it should not. A config pairing `respawn true` with a plugin that declares otherwise no longer starts |
| How is it reverted? | single commit revert; no on-disk state, no wire format outside the plugin protocol |
| Who else touches this path? | the config-reload transaction rollback (`reload_tx.go`), the ExaBGP migration, and every plugin's Stage-1 declaration |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin's `sdk.Registration{FailurePolicy: "restart"}`, over the real handshake | -> | `Server.applyFailurePolicy` -> `Server.restartPlugin` | `TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart`, and `failure-policy-restart` over a forked process |
| A plugin that declares nothing | -> | `pluginFailurePolicy` -> the ignore branch | `TestPluginThatExitsStaysDownWhenItDeclaredNothing` |
| A plugin's `sdk.Registration{FailurePolicy: "fatal"}` | -> | `Server.stopDaemonForPlugin` | `TestPluginThatExitsStopsTheDaemonWhenItDeclaredFatal` |
| `plugin { external p { respawn true; } }` against a plugin that declares `ignore` | -> | `refuseRespawnDisagreement` -> `Server.stopDaemonForPlugin` | `TestRespawnRequestAgainstAPluginThatCannotRestartStopsTheDaemon` |
| `plugin { external p { respawn true; } }` in a config file | -> | `ExtractPluginsFromTree` -> `PluginConfig.Respawn` | `TestExtractPluginsReadsTheRespawnLeaf` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin declares `failure-policy: restart` and its process later exits | Ze starts it again, the replacement completes the five-stage handshake, and its commands resolve to the replacement process |
| AC-2 | A plugin declares `failure-policy: ignore`, or declares nothing, and its process exits | the plugin stays down, Ze logs it at WARN naming the plugin, and the daemon keeps running |
| AC-3 | A plugin declares `failure-policy: fatal` and its process exits after startup | Ze stops: the shutdown request is signaled and `Server.Wait` returns |
| AC-4 | A plugin declares `failure-policy: fatal` and fails a startup stage after it declared | Ze stops rather than running degraded without it |
| AC-5 | A plugin declares a value that is not one of the three | its registration is refused with an error naming the value and the three legal spellings, and the plugin does not run |
| AC-6 | A `restart` plugin exits more than `RespawnLimit` times inside `RespawnWindow` | the plugin is disabled, the `plugin-down` warning is raised, and Ze carries on, which is the `ignore` outcome |
| AC-7 | `plugin { external p { respawn true; } }` and plugin `p` declares `ignore` or `fatal` | Ze does not start. The message names the plugin, the policy it declared, and the respawn the configuration asked for |
| AC-8 | `plugin { external p { respawn false; } }` and plugin `p` declares `restart` | the plugin is not restarted when it exits: the operator asked for less than the plugin allows, which is inside the constraint |
| AC-9 | A config carries `respawn true;` under `plugin { external <name> }` | `ExtractPluginsFromTree` reads it into `PluginConfig.Respawn`, and a value that is not a boolean is refused by name |
| AC-10 | Any plugin's process exits unexpectedly, whatever the policy | the `plugin-crash` report error is raised for it |
| AC-11 | A config-reload rollback ack reports `CodeBroken` for a plugin that declares `restart` | `Server.restartPlugin` restarts it, rather than failing with "respawn not enabled" |
| AC-12 | The ExaBGP bridge plugin | declares `restart`, because ExaBGP's own process block respawns by default |
| AC-13 | A plugin the engine restarts | receives the post-startup callback, so its `OnAllPluginsReady` handler runs for the replacement as it did for the plugin it replaced |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a plugin that declares `restart`, and the plugin crashes | plugin exit -> `superviseProcess` -> `applyFailurePolicy` -> `restartPlugin` -> handshake -> command registry | `TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart` |
| 2 | configures a plugin that declares `fatal`, and the plugin crashes | plugin exit -> `applyFailurePolicy` -> `stopDaemonForPlugin` -> `Server.Wait` returns | `TestPluginThatExitsStopsTheDaemonWhenItDeclaredFatal` |
| 3 | writes `respawn true;` for a plugin that declares `ignore` | config -> `PluginConfig.Respawn` -> Stage 1 -> `refuseRespawnDisagreement` -> the daemon stops with a message naming both sides | `TestRespawnRequestAgainstAPluginThatCannotRestartStopsTheDaemon` |
| 4 | migrates an ExaBGP config whose process block respawns | `exabgpRespawn` -> the bridge, which declares `restart` | `TestExabgpBridgeDeclaresItCanRestart` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFailurePolicyValidAcceptsOnlyTheThreeSpellings` | `pkg/plugin/rpc/failure_policy_test.go` | AC-5 form check | |
| `TestFailurePolicyAllowsARestartOnlyForRestart` | `pkg/plugin/rpc/failure_policy_test.go` | A-1: one field answers both questions | |
| `TestExtractPluginsReadsTheRespawnLeaf` | `internal/component/config/loader_plugin_test.go` | AC-9 | |
| `TestPluginFailurePolicyReadsAnUndeclaredPolicyAsIgnore` | `internal/component/plugin/server/failure_policy_test.go` | AC-2 default | |
| `TestRegistrationRefusesAnUnknownFailurePolicy` | `internal/component/plugin/server/failure_policy_test.go` | AC-5 | |
| `TestExabgpBridgeDeclaresItCanRestart` | `internal/plugins/exabgp/bridgeplugin/internal_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| respawns per window | 0-`RespawnLimit` | 5 | N/A | 6, which disables the plugin |
| cumulative respawns | 0-`MaxTotalRespawns` | 20 | N/A | 21, which disables the plugin |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart` | `internal/component/plugin/server/failure_policy_test.go` | a crashed plugin is running again with its commands | |
| `TestPluginThatExitsStaysDownWhenItDeclaredNothing` | same | a plugin that declared nothing is left stopped | |
| `TestPluginThatExitsStopsTheDaemonWhenItDeclaredFatal` | same | the daemon stops | |
| `TestStartupFailureOfAFatalPluginStopsTheDaemon` | same | AC-4 | |
| `TestRespawnRequestAgainstAPluginThatCannotRestartStopsTheDaemon` | same | AC-7 | |
| `TestRespawnFalseKeepsAPluginThatCanRestartDown` | same | AC-8 | |
| `failure-policy-restart` | `test/plugin/` | a forked plugin that exits is running again, and says so from its second generation | |
| `failure-policy-fatal` | `test/plugin/` | a forked plugin that declares fatal and exits stops the daemon | |
| `failure-policy-disagreement` | `test/plugin/` | `respawn true` against a plugin that declares `ignore` refuses to start, naming both sides | |
| `cli-completion-plugin-external.ci` | `test/ui/` | completion still offers `respawn` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | The plugin protocol has no third-party peer. The declaration is Ze's own IPC between the engine and a plugin built on Ze's SDK, so there is no other implementation to interoperate with (`ai/rules/interop-and-goal-validation.md`, "tooling with no protocol peer"). The ExaBGP lineage is honored through the config leaf, which the migration suite already covers | N-A |

## Files to Modify
- `pkg/plugin/rpc/types.go` - the `FailurePolicy` type and the declaration field
- `pkg/plugin/sdk/sdk_types.go` - the SDK's names for both
- `internal/component/plugin/registration.go` - `PluginRegistration.FailurePolicy`
- `internal/component/plugin/types.go` - one `Respawn` field, not two
- `internal/component/plugin/process/manager.go` - drop the config check and `ErrRespawnNotEnabled`; move the crash report to `ReportCrash`
- `internal/component/plugin/process/process.go` - `EngineDone`
- `internal/component/plugin/server/dispatch.go` - the third way out of the bridge-mode wait
- `internal/component/plugin/server/restart.go` - `restartHandshake` delivers the post-startup callback to the replacement
- `internal/component/plugin/server/startup.go` - refuse an unknown policy and a respawn disagreement, carry the policy, apply `fatal` to a startup failure, supervise every running process
- `internal/component/config/loader.go` - read the `respawn` leaf
- `internal/component/plugin/yang/ze-plugin-conf.yang` - the leaf's help says what it now means
- `cmd/ze/hub/main.go` - the `Respawn` copy stays and now carries a real value
- `internal/plugins/exabgp/bridgeplugin/internal.go` - declare `restart`
- `internal/component/firewall/plugins/irr/irr.go`, `.../domain/domain.go` - declare `restart`, which their own comments already claim
- `docs/architecture/system-architecture.md` - the two `respawn true;` blocks
- `docs/architecture/api/process-protocol.md` - the Stage-1 declaration set

## Files to Create
- `internal/component/plugin/server/failure_policy.go` - resolve and apply the policy
- `internal/component/plugin/server/failure_policy_test.go`
- `pkg/plugin/rpc/failure_policy_test.go`
- `internal/component/config/loader_plugin_test.go`
- `internal/test/fixture/plugin_fixture_failure_policy.go`, `register_failure_policy.go` - the three forked-process drivers
- `test/plugin/failure-policy-restart.ci`, `failure-policy-fatal.ci`, `failure-policy-disagreement.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/ze-plugin-conf.yang`, whose `respawn` help is rewritten. No leaf is added or removed |
| YANG validation constraints | Yes | the leaf is `type boolean`, which is the maximum native constraint for it |
| YANG custom validators | No | native `boolean` is sufficient; the cross-check against the plugin's declaration cannot run at config-load time because the plugin has not declared yet |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | Yes | automatic for a boolean leaf, unchanged |
| Functional test for new RPC/API | Yes | the six server-level tests drive real plugin engines through the real handshake |
| Pipe completeness | N-A | no command output changes |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | No | no new runtime dependency; the existing `plugin-down` and `plugin-crash` report codes carry the observable state |
| Prometheus counters/metrics | No | `ze_plugin_restarts_total` already exists and already counts what `restart` produces |
| BGP family surface | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/plugins.md` |
| 2 | Config syntax changed? | Yes | `docs/architecture/system-architecture.md` (the two blocks) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/plugins.md` |
| 7 | Wire format changed? | No | the change is a JSON key in the plugin IPC, covered by row 8 |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior? | N-A | no RFC governs the plugin IPC |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `website/compare/nos.md` cites the respawn machinery |
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md` |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | unchanged |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` |
| 16 | Changed source file referenced by doc source anchors? | DERIVED | `./le spec citation anchors`. The four pages a changed file DECLARES with `// Design:` are named below |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/system-architecture.md`, `docs/guide/firewall.md` |

**Pages a changed file DECLARES with its `// Design:` header (row 16, named
rather than derived):**

| Page | Declared by | Verdict |
|------|-------------|---------|
| `docs/architecture/api/ipc_protocol.md` | `pkg/plugin/rpc/types.go` | UPDATE: the Stage-1 declaration set gains `failure-policy` |
| `docs/architecture/config/syntax.md` | `internal/component/config/loader.go` | UNAFFECTED: no grammar changes. The `respawn` leaf keeps its spelling and its container, and only its meaning changes, which `docs/architecture/system-architecture.md` carries |
| `docs/architecture/firewall/firewall-irr.md` | `internal/component/firewall/plugins/irr/irr.go` | UPDATE: the page describes what a crash of the IRR plugin costs, and the plugin now declares that it restarts |
| `docs/architecture/hub-architecture.md` | `internal/component/plugin/server/startup.go` | UPDATE: the plugin startup phases now refuse a respawn disagreement and can stop the daemon |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the declaration and the leaf reach the exit path
   - Tests: the five Wiring Test rows, failing
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go`,
     `internal/component/plugin/registration.go`, `server/startup.go`,
     `server/failure_policy.go`, `process/process.go`, `server/dispatch.go`,
     `config/loader.go`
   - Verify: a plugin's declared policy and the config's request are both
     readable from its `*process.Process` after the handshake, and the exit path
     runs
2. **Phase: The three policies and the disagreement**
   - Tests: `TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart`,
     `TestPluginThatExitsStaysDownWhenItDeclaredNothing`,
     `TestPluginThatExitsStopsTheDaemonWhenItDeclaredFatal`,
     `TestStartupFailureOfAFatalPluginStopsTheDaemon`,
     `TestRespawnRequestAgainstAPluginThatCannotRestartStopsTheDaemon`,
     `TestRespawnFalseKeepsAPluginThatCanRestartDown`
   - Files: `server/failure_policy.go`, `server/startup.go`, `process/manager.go`
   - Verify: each policy produces its own outcome and no other
3. **Phase: The leaf keeps its promise**
   - Tests: `TestExtractPluginsReadsTheRespawnLeaf`, `cli-completion-plugin-external.ci`
   - Files: `config/loader.go`, `yang/ze-plugin-conf.yang`, `plugin/types.go`,
     `cmd/ze/hub/main.go`
   - Verify: the leaf reaches `PluginConfig` and its help says what it now means
4. **Phase: Declarations and documentation**
   - Files: `exabgp/bridgeplugin`, `firewall/plugins/irr`,
     `firewall/plugins/domain`, the doc pages
   - Verify: the field has non-test producers, and no page still teaches the old
     meaning

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Feature completeness | The three policies each reach a distinct outcome through the real handshake, not a unit switch |
| Correctness | The exit path does not fire on a daemon shutdown, on a plugin the manager no longer holds, or on a process another path already replaced |
| Correctness | `fatal` and the disagreement use the SAME daemon-stop route `request shutdown` uses, not a second one |
| Correctness | The disagreement refusal names the plugin, its declared policy, and the config leaf |
| Naming | The JSON key is kebab-case `failure-policy`; the three values are the owner's words |
| Data flow | The `process` package learns nothing about the policy; the server decides |
| Guard | An unknown policy fails CLOSED: the plugin is refused, never silently defaulted |
| Rule: `ai/rules/principles.md` | `FailurePolicyUnspecified` is the zero value and is never an answer: exactly one function turns it into an outcome, and it says why |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| A plugin can declare a failure policy | `grep -n FailurePolicy pkg/plugin/sdk/sdk_types.go` |
| Shipped plugins declare one | `grep -rn "FailurePolicy:" internal/` |
| The leaf reaches the engine | `go test ./internal/component/config/ -run TestExtractPluginsReadsTheRespawnLeaf` |
| Each policy is proven through the real path | `go test ./internal/component/plugin/server/ -run 'Declared|Respawn'` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The policy string arrives over the plugin IPC. It is checked against a closed set at Stage 1 and an unknown value refuses the registration |
| Resource exhaustion | A `restart` crash loop is bounded by the existing per-window and cumulative limits, after which the plugin is disabled |
| Availability | `fatal` and the disagreement stop the daemon by design. The owner settled the trust question: configuring the plugin is the consent |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check the AC |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights
- The engine already had every part of a restart except the trigger: bounded
  respawn, a handshake runner, and a registration release. What was missing was
  a watcher on the plugin's exit, and for an in-process plugin that exit is a
  third channel nothing selected on.
- One field answers both "what happens when this plugin fails" and "may this
  plugin be restarted", because Ze restarts a plugin for exactly one reason.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The plugin declares the policy, the leaf requests inside it | an operator leaf alone; a plugin declaration alone | Owner, 2026-09-06: the leaf stays for ExaBGP compatibility, and "if a plugin declare that it can not restart then we can not start" |
| One field, not a capability plus an outcome | a separate `Restartable` boolean beside the policy | the three values are mutually exclusive outcomes and Ze restarts a plugin for one reason only, so "restartable but do not restart on failure" is a state nothing needs |
| `fatal` is open to any plugin | restrict it to plugins Ze ships | Owner, 2026-09-06: "fatal is for any plugin the user decides to use" |
| An undeclared policy reads as `ignore` | default to `restart` | `ignore` is what Ze did before this spec, so no shipped plugin's behavior changes without its author asking. `restart` would turn every one-shot plugin into a loop |
| `respawn false` against a `restart` plugin leaves it down | refuse that pairing too | the owner named one disagreement only, and it is the one where the config asks for MORE than the plugin allows. Asking for less is inside the constraint, and refusing it would break an ExaBGP config that turned respawning off |
| `restart` is bounded by the existing limits and falls back to staying down | a new backoff | the bound already exists, is observable through `plugin-down` and `ze_plugin_restarts_total`, and a second bound beside it would be a second declaration of the same fact |
| The policy is applied wherever a plugin has DECLARED it, at startup and at runtime | apply it only after startup | a plugin that declared `fatal` at Stage 1 and failed at Stage 3 has said what its failure means. Leaving the daemon degraded there would make `fatal` mean less than it says |
| `ProcessManager.Respawn` stops refusing on config | keep the guard | the guard read a field no producer set, and the caller is now the one that knows the policy. Removing it also revives the config-reload rollback restart |
| A restarted plugin gets the post-startup callback | leave the gap, since it predates this spec | the gap BLOCKS this spec's goal: a restart that leaves the replacement without its `OnAllPluginsReady` handler is not the plugin coming back. The `failure-policy-restart` functional test timed out on exactly that, and the fix is the same call a mid-life config reload already makes |

## Known Limitations
- A plugin that fails at or before Stage 1 has declared no policy, so Ze reads
  it as `ignore`. That is the same abort-the-phase behavior as before, and there
  is nothing else it can be: the daemon has not been told what the plugin wants.
- The policy is a property of the plugin build, so an operator learns it from the
  plugin's source or from the refusal message when it disagrees with their config.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries checkpoints
- [ ] Integration Checklist marks the rows that apply

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated
- [ ] Integration and Documentation checklists answered
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken
- [ ] Every item this spec did not do is a spec of its own

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
