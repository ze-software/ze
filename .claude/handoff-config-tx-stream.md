# Handoff: spec-config-tx-protocol Phase 1 (event-type registration)

Generated 2026-04-07. Session: design-phase only, no implementation.

## RATIONALE (verify this matches what was agreed)

- **Decision: drop the bus, use the stream system for config transactions** -> all EDITs below
  Reason: the bus (`pkg/ze/bus.go`) was found to be premature abstraction in the
  arch-0 design. The existing stream system in `internal/component/plugin/server/dispatch.go`
  (`subscribe-events` / `emit-event` / `deliver-event` RPCs) already provides everything
  config transactions need: pub/sub fan-out, schema validation via the event-type
  registry, DirectBridge zero-copy hot path, and external plugin participation over TLS.
  No concrete consumer requires the bus. See `plan/spec-config-tx-protocol.md`
  "Transport Revision (2026-04-07)" for the full rationale.

- **Decision: arch-0 5-component model becomes 4** -> EDIT 5 (memory.md, already done)
  The Bus component is removed. Engine, ConfigProvider, PluginManager, Subsystem remain.
  Stream system in PluginManager is the pub/sub backbone.

- **Decision: external plugins (Python via TLS) participate for free** -> AC-17 in spec
  No bus-over-wire infrastructure needed. They use the existing `subscribe-events` /
  `emit-event` / `deliver-event` RPCs the same way they receive BGP and RPKI events today.

- **Decision: per-plugin event types over single broadcast** -> EDIT 1
  Engine emits `(config, verify-<plugin>)` per plugin so each plugin sees only its own
  diffs. Acks use broadcast event types `(config, verify-ok)` etc. Alternative single-type
  approach noted in spec; orchestrator implementation chooses.

- **Decision: authorization is per-connection, not per-event-type** (no spec change)
  Once connected via TLS + token, plugin can subscribe to / emit any registered event.
  Matches existing stream behavior. Recorded but not added to spec per user.

- **Decision: reconnect re-runs 5-stage startup, setup must be idempotent** (no spec change)
  No special "reconnect path." Subscribing to the same `(namespace, event-type)` twice
  yields one subscription. Engine cleans up plugin state on disconnect; plugin re-runs
  declare/subscribe on reconnect. The stream system's subscribe-events handler MUST
  dedupe per `(plugin, namespace, event-type)` -- verify this when wiring.

- **Open: orchestrator pub/sub layer rewrite** (Phase 4, NOT in this handoff)
  `internal/component/config/transaction/orchestrator.go` (~550 lines) currently uses
  `bus.Publish` / `bus.Subscribe`. Phase 4 rewrites it to use `emitEvent` / `subscribeEvents`
  while preserving the orchestration logic (state machine, deadline computation, ack collection,
  reverse-tier rollback, broken plugin recovery). This is the next handoff after Phase 1.

- **Open: reload.go wiring** (Phase 7, NOT in this handoff)
  `internal/component/plugin/server/reload.go` currently calls `SendConfigVerify` /
  `SendConfigApply` sequentially. Phase 7 replaces with `TxCoordinator.Execute()`.

- **Open: doc rewrite** (Phase 9, NOT in this handoff)
  `docs/architecture/config/transaction-protocol.md` is bus-based throughout. Needs
  full rewrite in stream-system terms.

If any rationale bullet is wrong, STOP and fix the handoff before applying edits.

## FILES ALREADY HANDLED (do not re-read)

- `plan/spec-config-tx-protocol.md` -- updated with stream-based decision, status `in-progress` phase `1/8`, all bus references either struck through or rewritten, new ACs 17-19 for stream-system specifics, new Phases 9-10 for doc rewrite and bus deletion. THIS IS THE SOURCE OF TRUTH.
- `plan/spec-config-tx-bus-native.md` -- DELETED (was committed in 7c5f8d89, preserved in git history)
- `plan/spec-plugin-bus-access.md` -- DELETED (was uncommitted, never landed)
- `plan/deferrals.md` -- AC-14 / spec-plugin-bus-access entry marked cancelled. New entry for `spec-stream-open-namespace` (future open-namespace work for plugin-to-plugin opaque messaging).
- `.claude/rules/memory.md` -- arch-0 entry updated: 5 components -> 4, bus removed, stream system named as backbone.
- `internal/component/config/transaction/topics.go` -- `TopicRolledBack` constant reverted (was a bus-approach artifact added earlier this session). Otherwise unchanged. NEEDS REWRITE in EDIT 1 below.
- `internal/component/plugin/events.go` -- READ for context. Defines `NamespaceBGP`, `NamespaceRIB`, `RegisterEventType`, `IsValidEvent`. New `NamespaceConfig` and config event constants will be added here per EDIT 1 / EDIT 2.

## What this handoff covers

**Phase 1 only: event-type constants and namespace registration.**

This is the smallest, lowest-risk starting point. After Phase 1 lands and verifies, the next session writes a Phase 4 handoff for the orchestrator pub/sub rewrite.

Phase 1 has zero behavioral change visible to users. It defines the names and registers the namespace. No plugin code yet uses the new constants. No transaction code is rewired. The change is purely additive vocabulary, plus a one-time revert of the bus topic constants in `transaction/topics.go`.

## EDITS

### EDIT 1: `internal/component/plugin/events.go` -- add config namespace and event types

Add a new namespace constant and event type constants alongside the existing BGP and RIB ones.

After line 16 (`NamespaceRIB = "rib"`), add:

```
	NamespaceConfig = "config"
```

After line 37 (`EventRoute = "route"`), add a new const block:

```
// Config transaction event types.
// Engine emits per-plugin verify/apply events. Plugins ack with broadcast events.
// See plan/spec-config-tx-protocol.md and docs/architecture/config/transaction-protocol.md.
const (
	EventConfigVerify       = "verify"        // Engine -> plugin: validate candidate (per-plugin variant: "verify-<plugin>")
	EventConfigApply        = "apply"         // Engine -> plugin: apply changes (per-plugin variant: "apply-<plugin>")
	EventConfigRollback     = "rollback"      // Engine -> plugins: undo changes
	EventConfigCommitted    = "committed"     // Engine -> plugins: discard journals
	EventConfigApplied      = "applied"       // Engine -> observers: transaction committed
	EventConfigRolledBack   = "rolled-back"   // Engine -> observers: transaction rolled back
	EventConfigVerifyAbort  = "verify-abort"  // Engine -> plugins: verify phase aborted

	EventConfigVerifyOK     = "verify-ok"     // Plugin -> engine: verification passed
	EventConfigVerifyFailed = "verify-failed" // Plugin -> engine: verification rejected
	EventConfigApplyOK      = "apply-ok"      // Plugin -> engine: apply succeeded
	EventConfigApplyFailed  = "apply-failed"  // Plugin -> engine: apply failed, trigger rollback
	EventConfigRollbackOK   = "rollback-ok"   // Plugin -> engine: rollback complete
)
```

After line 72 (the closing `}` of `ValidRibEvents`), add:

```

// ValidConfigEvents is the set of valid config transaction event types.
// Per-plugin variants ("verify-<plugin>", "apply-<plugin>") are registered
// dynamically as plugins start, via RegisterEventType(NamespaceConfig, ...).
var ValidConfigEvents = map[string]bool{
	EventConfigVerify:       true,
	EventConfigApply:        true,
	EventConfigRollback:     true,
	EventConfigCommitted:    true,
	EventConfigApplied:      true,
	EventConfigRolledBack:   true,
	EventConfigVerifyAbort:  true,
	EventConfigVerifyOK:     true,
	EventConfigVerifyFailed: true,
	EventConfigApplyOK:      true,
	EventConfigApplyFailed:  true,
	EventConfigRollbackOK:   true,
}
```

In the `ValidEvents` map (currently lines 77-80), add the new namespace:

OLD:
```
var ValidEvents = map[string]map[string]bool{
	NamespaceBGP: ValidBgpEvents,
	NamespaceRIB: ValidRibEvents,
}
```

NEW:
```
var ValidEvents = map[string]map[string]bool{
	NamespaceBGP:    ValidBgpEvents,
	NamespaceRIB:    ValidRibEvents,
	NamespaceConfig: ValidConfigEvents,
}
```

### EDIT 2: `internal/component/plugin/events_test.go` -- add tests for config namespace

Add test function after the existing tests for the new namespace. The pattern matches `TestIsValidEventBGP` (or whatever the existing BGP test is called -- read the file to confirm).

```
func TestIsValidEventConfig(t *testing.T) {
	if !IsValidEvent(NamespaceConfig, EventConfigVerify) {
		t.Errorf("expected (config, verify) to be valid")
	}
	if !IsValidEvent(NamespaceConfig, EventConfigApplyOK) {
		t.Errorf("expected (config, apply-ok) to be valid")
	}
	if IsValidEvent(NamespaceConfig, "nonsense") {
		t.Errorf("expected (config, nonsense) to be invalid")
	}
}

func TestRegisterConfigPerPluginEvent(t *testing.T) {
	// Per-plugin event types like "verify-myplugin" are registered dynamically.
	if err := RegisterEventType(NamespaceConfig, "verify-test"); err != nil {
		t.Fatalf("unexpected error registering verify-test: %v", err)
	}
	defer unregisterEventType(NamespaceConfig, "verify-test")

	if !IsValidEvent(NamespaceConfig, "verify-test") {
		t.Errorf("expected (config, verify-test) to be valid after registration")
	}
}
```

Note: `unregisterEventType` is a test helper that already exists -- check the bottom of `events_test.go` for it. If it's not there, add a small test helper or use `defer delete(ValidConfigEvents, "verify-test")`.

### EDIT 3: `internal/component/config/transaction/topics.go` -- rewrite topic constants as event-type aliases

The file currently defines bus topic strings like `"config/verify/"`. Replace them with re-exports of the constants from `internal/component/plugin/events.go` so the transaction package has its own clean import without reaching across packages everywhere.

Replace the entire body of `topics.go` (lines 10 onwards, after the package declaration) with:

```
import "codeberg.org/thomas-mangin/ze/internal/component/plugin"

// Namespace re-exported for transaction code clarity.
const Namespace = plugin.NamespaceConfig

// Engine -> plugin event type bases. Per-plugin variants are registered
// dynamically at plugin start as "<base>-<plugin>" (e.g. "verify-bgp").
const (
	EventVerify   = plugin.EventConfigVerify
	EventApply    = plugin.EventConfigApply
	EventRollback = plugin.EventConfigRollback
)

// Engine -> plugins broadcast event types.
const (
	EventVerifyAbort = plugin.EventConfigVerifyAbort
	EventCommitted   = plugin.EventConfigCommitted
	EventApplied     = plugin.EventConfigApplied
	EventRolledBack  = plugin.EventConfigRolledBack
)

// Plugin -> engine ack event types.
const (
	EventVerifyOK     = plugin.EventConfigVerifyOK
	EventVerifyFailed = plugin.EventConfigVerifyFailed
	EventApplyOK      = plugin.EventConfigApplyOK
	EventApplyFailed  = plugin.EventConfigApplyFailed
	EventRollbackOK   = plugin.EventConfigRollbackOK
)

// EventVerifyFor returns the per-plugin verify event type for the named plugin.
// Engine registers this dynamically when the plugin starts.
func EventVerifyFor(name string) string {
	return EventVerify + "-" + name
}

// EventApplyFor returns the per-plugin apply event type for the named plugin.
// Engine registers this dynamically when the plugin starts.
func EventApplyFor(name string) string {
	return EventApply + "-" + name
}

// Failure codes for transaction ack events.
// Plugins include a code in their ack to indicate the severity of failure.
const (
	CodeOK        = "ok"        // Success.
	CodeTimeout   = "timeout"   // Plugin did not respond in time (set by engine).
	CodeTransient = "transient" // Temporary failure, retry may succeed.
	CodeError     = "error"     // Permanent failure, rollback needed.
	CodeBroken    = "broken"    // Plugin state is corrupt, restart needed.
)

// MaxBudgetSeconds is the maximum allowed verify or apply budget.
// Budgets exceeding this are capped to this value.
const MaxBudgetSeconds = 600
```

The file's `// Design:` and `// Related:` comment block at the top stays. Update the package doc comment to say "stream-based" instead of "bus-based":

OLD:
```
// Package transaction implements the bus-based config transaction protocol.
// It defines topic constants, event payload types, and the transaction
// orchestrator that coordinates verify/apply/rollback across plugins.
```

NEW:
```
// Package transaction implements the stream-based config transaction protocol.
// It defines event type constants, event payload types, and the transaction
// orchestrator that coordinates verify/apply/rollback across plugins via the
// stream system in internal/component/plugin/server/dispatch.go.
```

### EDIT 4: `internal/component/config/transaction/topics_test.go` -- update test assertions

The existing test `TestConfigTopicConstants` (or whatever it is named -- read the file) checks the old `"config/verify/"` topic strings. Update it to check the new event type constants instead.

Read the file first. The change is mechanical: every assertion that checks a topic string `"config/X"` becomes an assertion that the corresponding `Event*` constant equals the expected event type name (e.g. `"verify"`, `"apply-ok"`).

Add new assertions for the per-plugin helpers:

```
if got := EventVerifyFor("bgp"); got != "verify-bgp" {
	t.Errorf("EventVerifyFor(bgp) = %q, want verify-bgp", got)
}
if got := EventApplyFor("interface"); got != "apply-interface" {
	t.Errorf("EventApplyFor(interface) = %q, want apply-interface", got)
}
```

### EDIT 5: verify nothing else broke

Run from the repo root:

```
go vet ./internal/component/config/transaction/... ./internal/component/plugin/... 2>&1
```

Then run the targeted tests:

```
go test -race ./internal/component/config/transaction/... ./internal/component/plugin/... 2>&1
```

Both should pass. If `orchestrator.go` fails to compile because it references the old bus topic constants, that is **expected**: those constants are gone after EDIT 3. Phase 4 (the next handoff) rewrites `orchestrator.go` to use the new event-type constants and stream emit/subscribe. **Do not fix `orchestrator.go` in this phase.** Instead, if the build break blocks the test run, that signals Phase 1 is complete and the next session needs to start Phase 4.

## After Phase 1

The next session writes a handoff for **Phase 4** (rewrite `orchestrator.go` pub/sub layer from bus to stream). The structure of that handoff:

1. Verify EDIT 5 above showed expected build break in orchestrator.go (good -- means the constants are removed)
2. Change every `o.bus.Publish(...)` call to an `emitEvent`-equivalent in the stream system
3. Change every `o.bus.Subscribe(...)` call to a `subscribeEvents`-equivalent
4. Add reverse-tier rollback ordering (currently TxCoordinator publishes one broadcast rollback)
5. Add dependency-graph-aware deadline (currently simple max budget)
6. Update `orchestrator_test.go` to use the stream system test fakes instead of bus fakes

After Phase 4 lands, Phase 7 (wire reload.go to TxCoordinator) becomes the next handoff. Phases 9 (doc rewrite) and 10 (bus interface deletion) come last.

## Reference

- Source of truth: `plan/spec-config-tx-protocol.md`
- Open deferrals: `plan/deferrals.md` (cancelled spec-plugin-bus-access; open spec-stream-open-namespace)
- Memory updated: `.claude/rules/memory.md` arch-0 entry
- Last commits before this work: `4a4a80dc chore: add pending spec files` and earlier `7c5f8d89 feat: wire 5 system plugins as config transaction consumers`
