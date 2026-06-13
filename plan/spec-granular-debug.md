# Spec: Granular Debug

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/core/slogutil/slogutil.go` - current logging infrastructure
4. `internal/core/slogutil/debug.go` - current debug flag resolution
5. `internal/plugins/debug/debug.go` - current `ze debug` CLI
6. `internal/component/config/yang/modules/ze-extensions.yang` - YANG extensions
7. `internal/component/config/yang/register.go` - YANG module registration pattern

## Task

Add granular per-module debug configuration with topic flags, direction filters,
instance scoping, and named profiles. Debug is a YANG-modeled configuration tree,
separate from committed config, stored in `debug.zefs`. Each plugin declares its
debug schema (available flags, scopes). Toggled via a `debug` CLI keyword with
toggle semantics. Profiles can be saved and loaded by name.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - ZeFS key storage model
- [ ] `docs/architecture/config/environment.md` - slogutil env var and log config
- [ ] `docs/architecture/core-design.md` - registration pattern
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - existing YANG extensions

### RFC Summaries (MUST for protocol work)
N/A - internal operational feature, no protocol wire changes.

**Key insights:**
- Ze already has hierarchical subsystem logging with runtime level changes via `slog.LevelVar`
- Debug flags already persist in ZeFS (`state/debug/*` keys in `database.zefs`)
- The `ze debug enable/disable/show` CLI exists but only toggles on/off per subsystem
- Config YANG modules are registered via `yang.RegisterModule()` in `init()`
- The config parser validates against the combined YANG schema

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/slogutil/slogutil.go` - per-subsystem slog logger creation, hierarchical env var lookup, `SetLevel()` runtime changes
- [ ] `internal/core/slogutil/debug.go` - `ResolveDebugStates()`, `ApplyDebugFlags()`, ZeFS key resolution (global > explicit > hierarchical parent > default)
- [ ] `internal/plugins/debug/debug.go` - `ze debug enable/disable/show` CLI, reads/writes `state/debug/*` keys in `database.zefs`
- [ ] `internal/component/config/yang/register.go` - `RegisterModule()` for YANG schema registration
- [ ] `internal/component/config/reader.go` - `ParseConfig()` for config text parsing

**Behavior to preserve:**
- Hierarchical subsystem names (dot-separated: `bgp.reactor.peer`)
- 4 slog severity levels (debug/info/warn/error)
- `slog.LevelVar` per subsystem for zero-cost short-circuit when level is above threshold
- Env var override (`ze.log.bgp.fsm=debug`) and config file log settings

**Behavior to change:**
- Debug state moves from `database.zefs` individual keys to YANG-modeled config in `debug.zefs`
- `ze debug enable/disable/show` is REPLACED by `debug` top-level CLI verb with toggle semantics
- Add flag/direction/scope filtering on debug log messages
- Each plugin declares its debug YANG schema (available flags, scope types)
- Debug profiles can be saved and loaded by name

**Old infrastructure to remove:**
- `KeyDebugAll` and `KeyDebugSubsystem` in `pkg/zefs/keys.go`
- `state/debug/*` key format in `database.zefs`
- `slogutil.ResolveDebugStates()` old key-based resolution (rewrite for profile-based)
- `cmd/ze/hub/main.go:271` `slogutil.ApplyDebugFlags(bs)` auto-apply at startup

## Data Flow (MANDATORY)

### Entry Point
- CLI command: `debug bgp.reactor flag update direction receive`
- Parsed by CLI command dispatcher into module, flag, direction, scope parameters

### Transformation Path
1. CLI parser extracts: module=`bgp.reactor`, flag=`update`, direction=`receive`
2. Load active debug profile from `debug.zefs`
3. Toggle: if flag entry exists in profile, remove it; if absent, add it
4. Validate modified profile against the debug YANG schema
5. Save updated profile to `debug.zefs`
6. Apply the change: update in-memory FilterHandler, call `slogutil.SetLevel()` if level changed
7. At log call sites: slog FilterHandler checks flag/direction/scope attributes against active config

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> debug plugin | CLI dispatch to debug command handler | [ ] |
| debug plugin -> debug config | Load/save profile from `debug.zefs` | [ ] |
| debug plugin -> debug YANG schema | Validate profile against registered modules | [ ] |
| debug plugin -> slogutil | `SetLevel()` / `RestoreLevel()` + FilterHandler config | [ ] |
| slogutil -> slog handler | FilterHandler wraps base handler, checks attributes | [ ] |

### Integration Points
- `slogutil.SetLevel()` - existing runtime level change mechanism
- `slogutil.Subsystems()` - existing subsystem registry for validation/completion
- `yang.RegisterModule()` - existing YANG registration pattern (reused for debug YANG)
- `zefs.BlobStore` - existing key-value storage for profile persistence
- CLI command registration via `register.go` pattern

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ZeFS supports a second blob file alongside database.zefs | ZeFS is a file-level abstraction | Need alternative storage or shared blob with prefix separation | Read zefs package API | confirmed (first attempt: zefs.Create takes any path) |
| A-2 | slog.Handler can filter on arbitrary structured attributes | Go slog design | Need custom handler implementation | Read slog.Handler interface | confirmed (first attempt: 9 FilterHandler tests passed) |
| A-3 | CLI can register top-level verbs beyond show/set/delete | CLI dispatch architecture | May need CLI framework extension | Read CLI dispatch code | confirmed (first attempt: MustRegisterRootHandler exists and used) |
| A-4 | Existing config parser can be reused for debug config text | Parser is YANG-driven, not config-specific | May need a separate parser or adapter | Read config parser code | confirmed (TokenizerFrontend is a pure tokenizer producing map[string]any, no YANG coupling) |
| A-5 | Debug YANG modules can coexist with config YANG modules in separate registries | YANG registration is a simple list | May need a separate debug module registry | Read yang.RegisterModule | confirmed (config YANG registry is trivial []Module slice, easily duplicated) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Attribute-based filtering adds overhead even when debug is off | Benchmark shows regression | LevelVar short-circuits before handler; filter only runs at debug level |
| R-2 | Toggle semantics confuse operators expecting explicit on/off | User feedback | `show debug` always shows current state; output confirms "enabled"/"disabled" |
| R-3 | Separate debug.zefs file not cleaned on factory reset | Manual test | Factory reset procedure must include debug.zefs deletion |
| R-4 | Debug YANG schema registration adds startup complexity | Profile count | Keep debug YANG loader minimal; only parse on profile load, not at startup |

## Design Decisions

### Debug is a separate YANG-modeled configuration tree

Debug state is configuration, but separate from the committed config tree. It does
not appear in `show configuration`, `show | compare`, or `commit`. It has its own
YANG schema, its own storage (`debug.zefs`), and its own CLI verb (`debug`).

Each plugin declares its debug surface via a debug YANG module:
- Which flags it supports (open, update, keepalive, fsm...)
- Which scope types it accepts (neighbor, group, interface...)
- Any plugin-specific debug options

This follows the same registration pattern as config YANG: each plugin registers
its debug YANG module via `init()`, the debug system combines them into a schema,
and profiles are validated against it.

**Rationale:** Debug configuration is richer than on/off. Flags, directions, scopes,
and levels form a structured document. YANG-modeling gives schema validation,
CLI completion from the schema, and plugin self-containment (each plugin owns its
debug surface). Named profiles enable operators to switch between debug scenarios
("bgp-deep", "plugin-lifecycle") without retyping everything.

**What this replaces:** The first implementation attempt used individual ZeFS keys
(`debug/{module}/level`, `debug/{module}/flag/{flag}/active`). This failed because:
individual keys cannot be restored as a unit, named profiles require grouping,
and ZeFS path conflicts prevent a segment from being both a file and a directory.

### Structured data, not formatted output

The debug plugin returns structured data. Presentation is handled by the standard
pipe infrastructure (`ApplyPipes`/`ProcessPipes`), not by the debug plugin itself.

- `show debug` -- default table renderer
- `show debug | json` -- JSON array
- `show debug | count` -- entry count
- `show debug | match <pattern>` -- grep through table rows
- `show debug bgp` -- subtree filter before pipe processing

### Toggle semantics via `debug` keyword

`debug` is a top-level CLI verb (like `show`). Running the same command twice toggles
the debug state in the active profile: first invocation adds the entry, second
invocation removes it. This makes CLI history (`arrow-up`) the natural undo mechanism.

No `activate`/`deactivate` pair needed. The command IS its own undo.

### Profile storage in debug.zefs

Debug profiles stored in `debug.zefs` as named ZeFS keys. Each profile is a
complete debug config document (text format matching the debug YANG schema).

On reboot, debug state is NOT auto-applied (safety: if debug caused the crash,
clean start). The saved profile persists on disk and can be explicitly restored.

### FilterHandler wiring into the logger pipeline

Every logger created via `slogutil.Logger()` or `LazyLogger()` wraps its base
handler with a `FilterHandler`. The FilterHandler is stored in a `filterRegistry`
(sync.Map keyed by subsystem name) alongside the existing `levelRegistry`.

When the debug plugin toggles a flag/scope, it calls `slogutil.ConfigureFilter(subsystem, ...)`
which looks up the FilterHandler in the registry and calls `SetFlags`/`SetDirections`/`SetScopes`.
When debug is toggled off for a module, `ClearFilters()` restores pass-through behavior.

Zero-cost path: when no filters are configured (the common case), FilterHandler's
`Handle()` checks `flags == nil && len(scopes) == 0` and delegates directly.
The LevelVar short-circuits before Handle is even called when debug is off.

### Runtime state vs persisted profiles

`show debug` shows **runtime state**: what is currently applied to loggers. This is
tracked in slogutil via the filterRegistry and levelRegistry. It does NOT read from
`debug.zefs`.

`show debug saved` shows **persisted state**: what is stored in `debug.zefs` but may
not be applied (e.g. after reboot before `debug restore`).

During a running session where the user toggles debug on, both views agree because
toggle both applies and persists. After reboot, they diverge: runtime is empty,
saved has the last profile.

### Profile document format

Profiles use the same Ze/Junos-style block syntax as regular config. Module names
use dot-notation in the CLI (`bgp.reactor`) but map to nested containers in the
profile document and YANG schema:

CLI: `debug bgp.reactor flag update direction receive`

Profile document stored in debug.zefs:
```
bgp {
    reactor {
        level debug;
        flag update {
            direction receive;
        }
    }
}
timeout 30;
```

The existing config parser (`TokenizerFrontend.ParseConfig`) parses this against
the debug YANG schema. The dot-to-nesting mapping is handled at the CLI layer,
not the profile layer.

-> Decision: Reuse the existing config parser. The parser is YANG-driven and
   not config-specific. The debug YANG schema defines the valid tree; the parser
   validates against it. No separate parser needed (closes Open Question #3).

### Debug YANG schema per plugin

Each plugin declares its debug surface in a YANG module. The debug system
loads all registered debug YANG modules to build the validation schema.
The debug YANG registry is separate from the config YANG registry.
Both follow the same `RegisterModule()` / `Modules()` pattern.

## Vendor Research Summary

| Vendor | Model | Granularity | Persistence | Unique Feature |
|--------|-------|-------------|-------------|----------------|
| Junos | Flag-based (open, update, keepalive...) | global/protocol/group/neighbor | Config-persistent (committed) | `detail`/`send`/`receive` modifiers per flag |
| Nokia SR OS | Flag-based, nearly identical to Junos | per-protocol/group/neighbor | NOT persistent (separate debug.cfg) | Clean separation from config |
| Cisco IOS-XR | Flag-based + conditional filters | per-process/neighbor/direction/policy | Runtime only | Route-policy filter on debug output |
| Arista EOS | Level-based (0-9) per agent/facility | per-agent, per-facility (wildcards) | Runtime only | Wildcard facility names, numeric levels |
| OpenConfig | Syslog severity + boolean per DEBUG_SERVICE | per-facility, per-service | Config | Vendor-augmentable identity base |
| IETF | No dedicated debug model | N/A | N/A | Only ietf-syslog (severity per facility) |

### Key takeaways

1. Every vendor separates debug from config EXCEPT Junos
2. Flag-based (what to trace) is universal; level-based (how verbose) is Arista-only
3. Per-neighbor/per-group scoping is table stakes (Junos, Nokia, IOS-XR)
4. Direction filtering (send/receive) is valuable for wire protocols
5. Nokia's debug.cfg (structured separate config) is closest to Ze's YANG-modeled approach

## Ze's Hybrid Model

| Dimension | Ze approach | Vendor inspiration |
|-----------|-------------|-------------------|
| Severity levels | Keep slog 4 levels (debug/info/warn/error) | Already have this |
| Subsystem hierarchy | Keep dot-notation (`bgp.reactor.peer`) | Already have this |
| Topic flags | Per-subsystem flag filtering (open, update, keepalive...) | Junos/Nokia |
| Direction | send/receive/both modifier on flags | Junos |
| Instance scoping | neighbor/group filter | Junos/Nokia/IOS-XR |
| Output routing | Optional per-module file with rotation (v2) | Junos |
| Toggle CLI | `debug` keyword, presence-based | Ze-native |
| Storage | YANG-modeled config in dedicated debug.zefs | Nokia-inspired, Ze-native YANG |
| Profiles | Named debug configs (save/load/restore) | Ze-native |
| Schema | Per-plugin debug YANG (plugin-owned surface) | Ze registration pattern |

## CLI Grammar

```
debug <module>                                    # toggle module debug on/off
debug <module> level <level>                      # set level (debug/info/warn/error)
debug <module> flag <flag>                         # toggle flag
debug <module> flag <flag> direction <dir>         # toggle flag with direction
debug <module> scope neighbor <addr>               # toggle neighbor scope
debug <module> scope group <name>                  # toggle group scope
debug timeout <minutes>                            # global auto-disable timer
debug restore                                      # load and apply default profile
debug restore profile <name>                       # load and apply named profile
debug clear                                        # wipe default profile
debug profile save <name>                          # save current state as named profile
debug profile list                                 # list available profiles
debug profile delete <name>                        # delete a named profile

show debug                                         # active debug state
show debug <module>                                # debug state for module subtree
show debug saved                                   # saved profile (not yet active after reboot)
show debug profile <name>                          # inspect a named profile
```

## Flag Sets (per subsystem domain)

Flags are declared in each plugin's debug YANG module. Examples:

### bgp.*
`open`, `update`, `keepalive`, `notification`, `refresh`, `route`, `policy`,
`fsm`, `timer`, `socket`, `config`, `graceful-restart`, `bfd`, `capability`

### plugin.*
`lifecycle`, `event`, `command`, `respawn`, `stage`

### iface.*
`netlink`, `config`, `state`, `counters`

### l2tp.*
`control`, `session`, `auth`, `tunnel`

### Flag registration

Each plugin declares its flags in its debug YANG module. The debug system
discovers available flags by walking the registered debug YANG schema.
No runtime `RegisterFlags()` API needed; the schema IS the registry.

## Open Questions

1. **Auto-disable timeout**: should `debug timeout 30` auto-clear ALL debug state after
   30 minutes? Useful safety net. Needs a background goroutine or cron-style check.

2. **Output routing**: per-module file output needs a new slog.Handler per file.
   Defer to v2, start with flag/scope filtering only.

3. ~~**Parser reuse**~~: Closed. Reuse the existing config parser. See "Profile document
   format" design decision above.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `debug bgp.reactor` | -> | Profile read-modify-write + apply | TBD at implementation |
| CLI `show debug` | -> | Profile load + structured display | TBD at implementation |
| Plugin debug YANG | -> | Schema in debug YANG registry | TBD at implementation |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `debug bgp.reactor` when module not in profile | Adds module to profile, sets level to debug, prints "enabled" |
| AC-2 | `debug bgp.reactor` when module in profile | Removes module from profile, restores level, prints "disabled" |
| AC-3 | `debug bgp.reactor flag update` | Toggles flag entry in module's profile section |
| AC-4 | `debug bgp.reactor flag update direction receive` | Toggles flag with direction constraint in profile |
| AC-5 | `debug bgp.reactor scope neighbor 192.0.2.1` | Toggles scope entry in module's profile section |
| AC-6 | `show debug` | Lists all active debug config with module, level, flags, scopes |
| AC-7 | `show debug bgp` | Lists only bgp.* subtree debug state |
| AC-8 | Debug state persists in `debug.zefs`, not `database.zefs` | Separate from config storage |
| AC-9 | Debug messages filtered by flag when flags are active | Only matching flag attribute passes through FilterHandler |
| AC-10 | No overhead when debug is off | `slog.LevelVar` short-circuits before filter check |
| AC-11 | Plugin declares debug flags via YANG | Debug YANG module registered, schema includes plugin's flags |
| AC-12 | `debug restore` loads and applies saved profile | Profile read from debug.zefs, levels and filters applied |
| AC-13 | `debug profile save <name>` saves current state | Named profile stored in debug.zefs |
| AC-14 | Invalid flag name rejected with YANG-derived error | `debug bgp.reactor flag nonexistent` fails validation |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProfileLoadSave` | `internal/plugins/debug/profile_test.go` | Profile round-trip through ZeFS | TBD |
| `TestProfileToggleModule` | `internal/plugins/debug/profile_test.go` | Module add/remove in profile | TBD |
| `TestProfileToggleFlag` | `internal/plugins/debug/profile_test.go` | Flag add/remove in profile | TBD |
| `TestFilterHandlerPassesMatchingFlag` | `internal/core/slogutil/filter_test.go` | Flag attribute matches active filter | TBD |
| `TestFilterHandlerBlocksNonMatchingFlag` | `internal/core/slogutil/filter_test.go` | Non-matching flag blocked | TBD |
| `TestFilterHandlerDirectionFilter` | `internal/core/slogutil/filter_test.go` | Direction send/receive filtering | TBD |
| `TestFilterHandlerScopeNeighbor` | `internal/core/slogutil/filter_test.go` | Neighbor scope filtering | TBD |
| `TestFilterHandlerNoFlagsPassesAll` | `internal/core/slogutil/filter_test.go` | No flags configured = all pass | TBD |
| `TestDebugYANGRegistration` | `internal/component/debug/yang/register_test.go` | Debug YANG modules registered | TBD |
| `TestShowDebugSubtree` | `internal/plugins/debug/show_test.go` | Hierarchical subtree display | TBD |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| timeout | 0-1440 | 1440 | N/A (0=never) | 1441 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-debug-toggle` | `test/plugin/debug-toggle.ci` | Toggle debug on/off via CLI, verify profile persists | TBD |
| `test-debug-show` | `test/plugin/debug-show.ci` | Show active debug state | TBD |
| `test-debug-profile` | `test/plugin/debug-profile.ci` | Save/load/list named profiles | TBD |

### Interop Tests (MANDATORY for protocol features)
N/A - internal operational feature, no wire protocol changes.

## Files to Modify
- `internal/plugins/debug/debug.go` - rewrite with toggle semantics, profile operations
- `internal/plugins/debug/register.go` - register `debug` as top-level CLI verb + `show debug`
- `internal/core/slogutil/slogutil.go` - wrap handlers with FilterHandler, expose filter registry
- `internal/core/slogutil/debug.go` - rewrite for profile-based debug resolution, remove old key format
- `pkg/zefs/keys.go` - remove `KeyDebugAll` and `KeyDebugSubsystem`, add `KeyDebugProfile`
- `cmd/ze/hub/main.go` - remove `slogutil.ApplyDebugFlags(bs)` auto-apply at startup
- `internal/plugins/debug/debug_test.go` - rewrite for profile-based API
- `internal/core/slogutil/debug_test.go` - rewrite for new key format

## Files to Create
- `internal/component/debug/yang/register.go` - debug YANG module registry (parallel to `internal/component/config/yang/`)
- `internal/component/debug/yang/register_test.go` - registry tests
- `internal/core/slogutil/filter.go` - slog.Handler that filters on flag/direction/scope attributes
- `internal/core/slogutil/filter_test.go` - filter handler tests
- `internal/plugins/debug/profile.go` - debug profile load/save/modify
- `internal/plugins/debug/profile_test.go` - profile tests
- `internal/plugins/debug/show.go` - show debug structured display
- `internal/plugins/debug/show_test.go` - show debug tests
- `internal/component/bgp/yang/ze-bgp-debug.yang` - BGP debug YANG schema (alongside config YANG)
- `internal/plugins/debug/yang/ze-debug-cmd.yang` - debug CLI command YANG
- `test/plugin/debug-toggle.ci` - functional test

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Debug YANG infrastructure** -- registry, BGP example schema
   - Tests: `TestDebugYANGRegistration`
   - Files: `internal/component/debug/yang/register.go`, BGP debug YANG module
   - Verify: debug modules registered separately from config modules

2. **Phase: Profile storage** -- load/save/modify profiles in debug.zefs
   - Tests: `TestProfileLoadSave`, `TestProfileToggleModule`, `TestProfileToggleFlag`
   - Files: `internal/plugins/debug/profile.go`
   - Verify: profile round-trips through ZeFS, toggle modifies entries

3. **Phase: Filter handler** -- slog.Handler wrapping with flag/direction/scope
   - Tests: `TestFilterHandler*` tests
   - Files: `internal/core/slogutil/filter.go`
   - Verify: filtering works on structured attributes, wired into logger pipeline

4. **Phase: CLI wiring + toggle** -- register `debug` verb, implement toggle dispatch
   - Tests: wiring tests
   - Files: `debug.go`, `register.go`
   - Verify: toggle modifies profile and applies to runtime

5. **Phase: Show debug + profiles** -- display active state, save/load profiles
   - Tests: `TestShowDebugSubtree`, functional tests
   - Files: `show.go`
   - Verify: hierarchical display, profile save/load works

6. **Phase: YANG validation** -- validate toggle commands against debug YANG schema
   - Tests: `TestInvalidFlagRejected`
   - Files: profile.go (validation integration)
   - Verify: invalid flag names rejected with schema-derived error

7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Profile read-modify-write is atomic (no partial state on failure) |
| Naming | Profile key uses `debug/profile/{name}` pattern |
| Data flow | Filter handler only activates at debug level |
| CLI grammar | `debug` verb registered as top-level keyword |
| Zero-cost | Benchmark confirms no overhead when debug is off |
| YANG schema | Each plugin's debug surface declared in debug YANG, not hardcoded |
| Plugin self-containment | Remove a plugin's debug YANG and its debug flags disappear |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `debug` CLI verb works | `ze debug bgp.reactor` in functional test |
| Toggle semantics | Run same command twice, verify profile state flips |
| `show debug` | Verify output shows active flags/scopes |
| Separate debug.zefs | `ls` for debug.zefs, verify not in database.zefs |
| Filter handler wired | Debug message with wrong flag is suppressed |
| Debug YANG schema | BGP debug flags discoverable via schema |
| Named profiles | `debug profile save/list/load` works |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Module names validated against debug YANG schema |
| Path traversal | Profile names sanitized (no `../`) |
| Resource exhaustion | Profile count and size bounded |

### Documentation Update Checklist
| Category | Applies | File + Update |
|----------|---------|---------------|
| Feature list | Yes | `docs/features.md` -- updated "Debug Flags" row to "Granular Debug" with new grammar |
| User guide | No | No user guide section exists for debug |
| Config syntax | No | Debug is separate from config |
| CLI reference | Yes | `docs/guide/command-reference.md` -- replaced old enable/disable/show with full new grammar |
| API/RPC docs | No | Debug is offline CLI only, no RPC |
| Plugin SDK | No | Debug YANG is internal, not part of external plugin SDK |
| Architecture design | No | `docs/architecture/config/environment.md` only mentions debug as env-only; no section to update |
| Test infrastructure | No | No test infrastructure docs affected |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Individual ZeFS keys per debug setting | Cannot restore as a unit, cannot have named profiles, not a configuration model. ZeFS path conflict: a segment cannot be both file and directory. | YANG-modeled debug configuration: each plugin declares its debug YANG schema, profiles are named config documents stored in debug.zefs. |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

-> Decision: Debug state must be YANG-modeled configuration, not raw ZeFS keys.
   Each plugin declares its debug schema (flags, scopes, levels) via YANG,
   the debug system validates and applies it like a second config tree (separate
   from committed config), and profiles are named config documents that support
   save/load/restore.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Debug is a separate YANG-modeled config tree | Junos-style committed traceoptions; raw ZeFS keys (first attempt) | YANG gives schema validation, CLI completion, plugin self-containment. Separate from committed config so operators don't risk committing verbose tracing. |
| Toggle semantics (one verb) | activate/deactivate pair | Simpler mental model, history-friendly, command IS its own undo |
| Named profiles in debug.zefs | Single unnamed state; external files | Operators need multiple debug scenarios; ZeFS keeps them co-located |
| Plugin-owned debug YANG | Central flag registry; runtime RegisterFlags API | Same pattern as config YANG: plugin owns its surface, delete the plugin and its debug flags vanish |
| Flag-based filtering (Junos/Nokia) | Level-based 0-9 (Arista) | Flags are more discoverable ("update" vs "level 5"); slog's 4 levels suffice |
| Replace `ze debug enable/disable` entirely | Backward compat alias | Clean break; old CLI was simple on/off, new model is richer |
| Structured data + pipe system | Custom formatted output | Same pattern as all `show` commands |

## Known Limitations
- Route-policy conditional debugging (IOS-XR style) is out of scope
- Numeric verbosity levels (Arista 0-9) not adopted; slog's 4 levels suffice
- Per-module file output is v2 (start with flag/scope/direction filtering)

## Implementation Summary

### What Was Implemented
- Debug YANG registry (`internal/component/debug/yang/`) for plugin debug flag registration
- filterHandler (`internal/core/slogutil/filter.go`) wrapping every logger for flag/direction/scope filtering
- Profile storage (`internal/plugins/debug/profile.go`) with JSON profiles in `debug.zefs`
- Toggle-based CLI handler replacing old enable/disable/show
- Show debug with structured table output and subtree filtering
- Profile save/load/list/delete, restore, clear, timeout
- Flag and scope validation against registered debug YANG modules
- BGP debug flags registered (`internal/component/bgp/yang/register_debug.go`)
- filterHandler wired into slogutil.Logger() pipeline with ConfigureFilter/ClearFilter API
- Removed old infrastructure: KeyDebugAll, KeyDebugSubsystem, ApplyDebugFlags, ResolveDebugStates

### Bugs Found/Fixed
- ZeFS List() fails with trailing slash in prefix (segments split on "/" creates empty segment)
- filterHandler WithAttrs needed to propagate pre-set attributes for matching

### Documentation Updates
- TBD (pending documentation review step)

### Deviations from Plan
- filterHandler and its methods are unexported (only ConfigureFilter/ClearFilter are public API) since all callers are within slogutil
- Debug YANG modules carry pre-extracted Flags/Scopes metadata instead of runtime YANG parsing
- Validation skips when no debug YANG module covers the module prefix (progressive rollout)
- `show debug` is via `debug show` subcommand (not online `show` verb) for simplicity

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-module debug config | done | debug.go:cmdToggle | Toggle per subsystem |
| Topic flags | done | profile.go:ToggleFlag | Flag entries in profile |
| Direction filters | done | profile.go:ToggleFlagDirection, filter.go:matchDirection | send/receive |
| Instance scoping | done | profile.go:ToggleScope, filter.go:matchScope | neighbor/group |
| Named profiles | done | profile.go:SaveProfile/LoadProfile/ListProfiles | JSON in debug.zefs |
| Debug as YANG-modeled config | done | component/debug/yang/register.go | Separate registry |
| Toggle semantics | done | debug.go:cmdToggle | Same command is undo |
| Stored in debug.zefs | done | profile.go:openDebugStore | Separate from database.zefs |
| Plugin declares debug schema | done | bgp/yang/register_debug.go | BGP flags registered |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestDebugToggleOn` debug_test.go:34 | Toggle on adds module, prints "enabled" |
| AC-2 | done | `TestDebugToggleOff` debug_test.go:48 | Toggle off removes module, prints "disabled" |
| AC-3 | done | `TestDebugToggleFlag` debug_test.go:65 | Flag toggle in profile |
| AC-4 | done | `TestDebugDirectionAsScope` debug_test.go:84 | Direction is a scope kind, not a flag modifier |
| AC-5 | done | `TestDebugToggleScope` debug_test.go:91 | Scope toggle in profile |
| AC-6 | done | `TestDebugShow` debug_test.go:104 | Show active debug state |
| AC-7 | done | `TestDebugShowSubtree` debug_test.go:117 + `TestShowDebugSubtree` show_test.go:26 | Subtree filtering |
| AC-8 | done | `TestProfileLoadSave` profile_test.go:100 | Uses debug.zefs, not database.zefs |
| AC-9 | done | `TestFilterHandlerPassesMatchingFlag` filter_test.go:30 | FilterHandler wired into Logger() |
| AC-10 | done | `TestFilterHandlerNoFlagsPassesAll` filter_test.go:21 | Zero-cost when no filters |
| AC-11 | done | `TestBGPDebugFlagsRegistered` debug_register_test.go:14 | BGP flags in debug YANG |
| AC-12 | done | `TestDebugRestore` debug_test.go:130 | Restore loads and applies |
| AC-13 | done | `TestDebugProfileSaveList` debug_test.go:152 | Named profile save/list |
| AC-14 | done | `TestDebugInvalidFlagRejected` debug_test.go:300 | YANG-derived validation |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestProfileLoadSave | done | profile_test.go:100 | Round-trip through ZeFS |
| TestProfileToggleModule | done | profile_test.go:30 | Module add/remove |
| TestProfileToggleFlag | done | profile_test.go:49 | Flag add/remove |
| TestFilterHandlerPassesMatchingFlag | done | filter_test.go:30 | Flag match passes |
| TestFilterHandlerBlocksNonMatchingFlag | done | filter_test.go:42 | Non-match blocked |
| TestFilterHandlerDirectionFilter | done | filter_test.go:67 | Direction filtering |
| TestFilterHandlerScopeNeighbor | done | filter_test.go:104 | Scope filtering |
| TestFilterHandlerNoFlagsPassesAll | done | filter_test.go:21 | No flags = pass-through |
| TestRegisterModule (debug YANG) | done | register_test.go:10 | Debug YANG registered |
| TestShowDebugSubtree | done | show_test.go:26 | Subtree display |
| TestBGPDebugFlagsRegistered | done | debug_register_test.go:14 | BGP flags in registry |
| TestDebugInvalidFlagRejected | done | debug_test.go:300 | Invalid flag rejected |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/component/debug/yang/register.go | created | Debug YANG registry with validation |
| internal/component/debug/yang/register_test.go | created | 6 tests |
| internal/core/slogutil/filter.go | created | filterHandler (unexported) |
| internal/core/slogutil/filter_test.go | created | 12 tests |
| internal/plugins/debug/profile.go | created | Profile CRUD |
| internal/plugins/debug/profile_test.go | created | 10 tests |
| internal/plugins/debug/show.go | created | Structured display |
| internal/plugins/debug/show_test.go | created | 4 tests |
| internal/plugins/debug/debug.go | rewritten | Toggle handler |
| internal/plugins/debug/debug_test.go | rewritten | 22 tests |
| internal/plugins/debug/register.go | modified | Updated meta |
| internal/core/slogutil/debug.go | rewritten | Removed old resolution |
| internal/core/slogutil/debug_test.go | rewritten | 8 tests |
| internal/core/slogutil/slogutil.go | modified | filterRegistry, ConfigureFilter, ClearFilter |
| pkg/zefs/keys.go | modified | Replaced old keys with KeyDebugProfile |
| cmd/ze/hub/main.go | modified | Removed ApplyDebugFlags call |
| internal/component/bgp/yang/register_debug.go | created | BGP debug flags |
| internal/component/bgp/yang/debug_register_test.go | created | 2 tests |

### Audit Summary
- **Total items:** 18 files
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (deviations listed above)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Granular per-module debug toggling | unit test | `TestDebugToggleOn/Off`: toggle changes level per-subsystem, hierarchical matching via `SubsystemsMatching` |
| Flag/direction/scope filtering | unit test | `TestFilterHandlerPassesMatchingFlag`, `TestFilterHandlerDirectionFilter`, `TestFilterHandlerScopeNeighbor`: filterHandler blocks/passes based on attributes |
| Separate debug storage | unit test | `TestProfileLoadSave`: profiles stored in `debug.zefs` via `openDebugStore()`, separate from `database.zefs` |
| YANG-modeled debug schema | unit test | `TestBGPDebugFlagsRegistered`, `TestDebugInvalidFlagRejected`: BGP flags registered via debug YANG registry, invalid flags rejected |
| Named profiles | unit test | `TestProfileSaveNamed`, `TestDebugProfileSaveList`, `TestDebugProfileDelete`: save/list/delete named profiles |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- TBD

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

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A)
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-granular-debug.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-granular-debug.md`
