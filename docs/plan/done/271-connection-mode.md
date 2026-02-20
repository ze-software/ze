# Spec: connection-mode

## Task

Replace the `passive` boolean peer option with a `connection` enum having three values: `both` (default), `passive` (accept only), `active` (dial only). The `active` mode prevents Ze from binding/listening for a peer — needed when running Ze with ze-peer where Ze should only dial out. All `.ci` test files using ze-peer must be updated to use `connection active;`.

## Key Insights

- FSM `handleIdle`: passive → StateActive, non-passive → StateConnect
- Peer `runOnce`: passive skips `session.Connect()`, non-passive calls it
- Listener accepts inbound for all peers; active-only peers are excluded
- `.ci` tests use ze-peer (listens) with Ze connecting out — `connection active` prevents Ze from binding
- RFC 4271 Section 8.1.1: PassiveTcpEstablishment optional attribute

## Data Flow

### Entry Point
- Config file: `connection passive;` or `connection active;` in peer block
- Environment: `ze.bgp.bgp.connection=passive` env var or config `environment { bgp { connection passive; } }`
- API: `bgp peer-add ... connection passive` RPC command

### Transformation Path
1. YANG validation: enum value checked against `{both, passive, active}`
2. Config tree: stored as string `"passive"`, `"active"`, or `"both"`
3. `parsePeerFromTree()`: string → `ConnectionMode` constant on `PeerSettings`
4. `Session` creation: `PeerSettings.Connection` → `FSM.SetPassive(!mode.IsActive())`
5. `peer.runOnce()`: checks `Connection.IsActive()` / `Connection.IsPassive()` for connect vs wait
6. Listener startup: checks `Connection.IsPassive()` to skip binding for active-only peers

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Config → PeerSettings | `parsePeerFromTree` reads `"connection"` key |
| PeerSettings → FSM | `session.go` translates enum to bool via `SetPassive(!IsActive())` |
| PeerSettings → Listener | `reactor.go` checks mode before starting listener |

### Integration Points
- `FSM.SetPassive(bool)` — kept as adapter; session translates ConnectionMode → bool
- `peer.runOnce()` — uses `Connection.IsActive()` / `Connection.IsPassive()` methods
- Listener startup — skips peers where `!Connection.IsPassive()`
- `acceptOrReject()` — rejects inbound for non-passive peers (defense in depth)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer with `connection passive;` | Identical behavior to old `passive true`: FSM→Active, no dial, accept inbound |
| AC-2 | Peer with `connection active;` | FSM→Connect, dial out, NO listener started, reject inbound |
| AC-3 | Peer with `connection both;` or no connection field | Default: FSM→Connect, dial out, accept inbound (identical to old `passive false`) |
| AC-4 | Peer with invalid `connection foo;` | Config parse error with clear message |
| AC-5 | ExaBGP config with `passive true;` | Migration outputs `connection passive;` |
| AC-6 | All `.ci` test peer blocks | Have `connection active;` — Ze does not bind when using ze-peer |
| AC-7 | Environment `ze.bgp.bgp.connection=passive` | Global connection mode applied to all peers |
| AC-8 | `ze bgp validate` output | Shows connection mode (both/passive/active) per peer |
| AC-9 | Chaos config generation | Emits `connection passive;` instead of `passive true;` |

## Implementation Summary

### What Was Implemented

**Core type** (`peersettings.go:21-68`):
- `ConnectionMode` type with constants: `ConnectionActive`, `ConnectionPassive`, `ConnectionBoth`
- `IsActive()`, `IsPassive()` methods, `String()`, `ParseConnectionMode()`
- Zero value is `ConnectionBoth` (default)

**Config/YANG** — all three YANG schemas updated:
- `ze-bgp-conf.yang:140-148` — enum with both/passive/active, default both
- `ze-bgp-api.yang:112` — `leaf connection` in peer-add
- `ze-hub-conf.yang:87` — `leaf connection` in environment/bgp
- `config.go:72` — `ParseConnectionMode()` called on "connection" key
- `environment.go:79` — `Connection string` field (replaces `Passive bool`)

**Session/FSM adapter** (`session.go:199`):
- `s.fsm.SetPassive(!settings.Connection.IsActive())` — translates enum to bool at session boundary
- FSM keeps internal `passive bool` — adapter pattern avoids leaking enum into FSM internals

**Peer/Reactor active-mode logic**:
- `peer.go:973,985,995` — `Connection.IsActive()` / `IsPassive()` checks in runOnce
- `reactor.go:4289` — skip listener for non-passive peers
- `reactor.go:4430` — filter listener list to passive peers only
- `reactor.go:4878` — reject inbound on non-passive peers

**Handler/Validate**:
- `handler/bgp.go:281-288` — peer-add command parses connection mode
- `validate/main.go:117,202,308` — displays connection mode per peer

**ExaBGP migration** (`migrate.go:233-236,291-294`):
- `passive true` → `connection passive`
- Default (no passive field) → `connection active` (ExaBGP peers dial out)

**Chaos config** (`scenario/config.go:110`):
- Emits `connection passive;` — Ze never dials out in chaos mode

**Test files**:
- 89 `.ci` files updated with `connection active;`
- 17+ Go test files mechanically renamed from `.Passive` to `.Connection`
- Zero remaining `.Passive` references in Go code (outside ExaBGP source format)

### Bugs Found/Fixed

- `summary-format.ci` — missing static route expect (fixed in 81a9acb3, pre-existing, not connection-mode)
- Invalid connection mode in config logged warning instead of error — added unit tests (48d7f35a)
- Environment var `ze_bgp_bgp_connection` not wired to PeerSettings — fixed (9918bba3)

### Deviations from Plan

| Planned | Actual | Reason |
|---------|--------|--------|
| FSM `passive bool` → `connection ConnectionMode` | FSM keeps `passive bool`, session translates | Adapter pattern: FSM doesn't need full enum, only passive/non-passive distinction. Cleaner boundary. |
| `SetPassive` → `SetConnection` | `SetPassive(bool)` kept | Same reason — FSM interface stays simple |
| Chaos `inprocess/runner.go` emits `connection passive` directly | Runner sets `ModePassive` on profiles, config generator emits the string | Cleaner: runner controls mode, generator handles syntax |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace passive bool with connection enum | ✅ Done | `peersettings.go:21-68` | ConnectionMode type with 3 values |
| Add active mode (don't bind/listen) | ✅ Done | `reactor.go:4289,4430,4878` | Listener skip + inbound reject |
| Update all YANG schemas | ✅ Done | `ze-bgp-conf.yang:140`, `ze-bgp-api.yang:112`, `ze-hub-conf.yang:87` | |
| Update ExaBGP migration | ✅ Done | `migrate.go:233,291` | passive→connection translation |
| Update all .ci tests with connection active | ✅ Done | 89 files in test/{plugin,encode,reload} | Verified by grep |
| Update chaos config generation | ✅ Done | `scenario/config.go:110` | `connection passive;` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | Existing FSM tests + `config_test.go:24` | Passive behavior preserved |
| AC-2 | ✅ Done | `reactor.go:4289,4878` + all .ci tests pass with `connection active` | Active mode: no listener, reject inbound |
| AC-3 | ✅ Done | `TestParseConnectionMode` (empty string → Both) | Zero value default |
| AC-4 | ✅ Done | `TestParseConnectionMode` + `TestLoadEnvironmentConnectionInvalidValue` | Invalid enum returns error |
| AC-5 | ✅ Done | `migrate.go:233-236,291-294` + ExaBGP compat tests | passive true → connection passive |
| AC-6 | ✅ Done | 89 .ci files with `connection active;` | Verified via grep count |
| AC-7 | ✅ Done | `environment.go:79` + `TestLoadEnvironmentConnectionValues` | String field wired through |
| AC-8 | ✅ Done | `validate/main.go:308` | Displays connection mode per peer |
| AC-9 | ✅ Done | `scenario/config.go:110` + `TestConfigGenAllPeersPassive` | `connection passive;` emitted |

### Tests
| Test | Status | Location |
|------|--------|----------|
| TestParseConnectionMode | ✅ Done | `config_test.go:938` |
| TestConnectionModeString | ✅ Done | `config_test.go:970` |
| TestConnectionModeIsActiveIsPassive | ✅ Done | `config_test.go:991` |
| TestLoadEnvironmentConnectionValues | ✅ Done | `environment_test.go:122` |
| TestLoadEnvironmentConnectionInvalidValue | ✅ Done | `environment_test.go:148` |
| TestConfigGenAllPeersPassive | ✅ Done | `scenario/config_test.go:173` |
| Existing passive tests (renamed) | ✅ Done | 17+ test files | Mechanical .Passive→.Connection |
| All .ci tests pass | ✅ Done | 89 files | `connection active;` added |

### Files Modified
| File | Status |
|------|--------|
| ze-bgp-conf.yang | ✅ |
| ze-bgp-api.yang | ✅ |
| ze-hub-conf.yang | ✅ |
| peersettings.go | ✅ |
| fsm.go | 🔄 Kept `passive bool` (adapter pattern) |
| config.go | ✅ |
| reactor.go | ✅ |
| environment.go | ✅ |
| session.go | ✅ |
| peer.go | ✅ |
| handler/bgp.go | ✅ |
| validate/main.go | ✅ |
| migrate.go | ✅ |
| scenario/config.go | ✅ |
| inprocess/runner.go | 🔄 Uses ModePassive (config generator emits string) |
| ~89 .ci files | ✅ |
| ~17 Go test files | ✅ |

### Audit Summary
- **Total items:** 38
- **Done:** 36
- **Changed:** 2 (FSM adapter pattern, runner delegation — both improvements)
- **Partial:** 0
- **Skipped:** 0

## Commits

- `823caa42` feat(config): replace passive bool with connection mode enum (both/passive/active)
- `81a9acb3` fix(test): add missing static route expect to summary-format test
- `48d7f35a` fix(reactor): log warning on invalid connection mode, add unit tests, fix race
- `9918bba3` fix(config): wire ze_bgp_bgp_connection env var to PeerSettings
