# Spec: sysctl-0-plugin

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/fibkernel/register.go` - FIB dependency declaration
4. `internal/plugins/ifacenetlink/sysctl_linux.go` - existing sysctl write code
5. `internal/component/iface/config.go:1212-1276` - applySysctl function to migrate
6. `internal/component/iface/dispatch.go:181-250` - sysctl dispatch functions to migrate
7. `internal/component/iface/backend.go` - backend interface sysctl methods

## Task

Create a `sysctl` plugin that centralises all kernel tunable management (system-wide
and per-interface). Moves sysctl writes out of ifacenetlink into a single plugin.
Three value layers (default, transient, config) with clear precedence. Modules
contribute known keys with descriptions, types, and validation. CLI commands for
inspection and transient changes.

## Design Decisions

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Known keys registry location | `internal/core/sysctl/` | Matches `internal/core/family/` pattern. Avoids plugin cross-imports. |
| 2 | Communication mechanism | All EventBus with DirectBridge | Performance. Plugins can subscribe to `(sysctl, applied)` and react. |
| 3 | Key naming | Kernel-native (`net.ipv4.conf.all.forwarding`) | No translation layer. Operators know these names. Unknown keys just work. |
| 4 | YANG config shape | Generic key/value list | Kernel sysctl namespace too large for structured YANG. |
| 5 | Darwin backend | `syscall.Syscall6` + `SYS_SYSCTLBYNAME` | No exec, production-quality, pure Go. |
| 6 | Cleanup on stop | Restore original values. Log at info. | Ze owns what it touches, puts it back when done. |
| 7 | iface dependency | iface declares `Dependencies: ["sysctl"]` | Tier ordering guarantees sysctl ready before iface config apply. |
| 8 | Override logging | Warn when config/transient overrides a plugin default | User should see the conflict with the plugin's required value. |
| 9 | Missing sysctl plugin | Not an issue. Dependency expansion auto-loads. | User can manually load if needed for standalone use. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin registration, dependency ordering
  → Constraint: plugins declare Dependencies; plugin manager starts them in order via TopologicalTiers
- [ ] `.claude/rules/plugin-design.md` - 5-stage protocol, EventBus, ConfigureEventBus
  → Constraint: EventBus delivered via ConfigureEventBus callback before RunEngine
- [ ] `.claude/patterns/plugin.md` - structural template for new plugins
  → Constraint: register.go + init() + blank import in all.go
- [ ] `pkg/ze/eventbus.go` - EventBus interface (Emit/Subscribe)
  → Constraint: (namespace, eventType) pairs must be registered. Payloads are opaque JSON strings.
- [ ] `internal/component/plugin/events.go` - namespace and event type registration
  → Constraint: new namespace "sysctl" must be added with valid event types

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- Plugins declare dependencies; manager resolves ordering via TopologicalTiers automatically
- Tier 0 (no deps) completes full 5-stage handshake before tier 1 starts
- EventBus uses DirectBridge for internal plugins (zero-copy hot path)
- Known keys registry in `internal/core/sysctl/` follows `internal/core/family/` pattern
- All sysctl writes go through EventBus, enabling any plugin to react to `(sysctl, applied)`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ifacenetlink/sysctl_linux.go` - per-interface sysctl writes via os.WriteFile to /proc/sys/
  → Constraint: writeSysctl, boolToSysctl are private to ifacenetlink; tests override sysctlRoot
- [ ] `internal/component/iface/config.go:108-125` - ipv4Sysctl/ipv6Sysctl structs (pointer fields for nil = not configured)
  → Decision: nil pointer means "not configured, leave OS default"
- [ ] `internal/component/iface/config.go:1212-1276` - applySysctl applies per-unit settings via backend
  → Constraint: only non-nil settings applied; per-interface, not global
- [ ] `internal/component/iface/dispatch.go:181-250` - 10 dispatch functions wrapping backend methods
- [ ] `internal/component/iface/backend.go` - Backend interface has 10 sysctl methods
- [ ] `internal/plugins/fibkernel/register.go` - Dependencies: ["rib"], no sysctl awareness
- [ ] `internal/plugins/fibkernel/fibkernel.go` - no forwarding enablement
- [ ] `internal/component/bgp/plugins/rpki/rpki.go:177-196` - OnAllPluginsReady DispatchCommand reference

**Behavior to preserve:**
- Per-interface sysctl semantics: nil = leave OS default, non-nil = set
- YANG `ze:os "linux"` on per-interface ipv4/ipv6 sysctl leaves
- iface config parsing of ipv4Sysctl/ipv6Sysctl structs
- ifacenetlink test pattern: override sysctlRoot for unit tests

**Behavior to change:**
- sysctl writes move from ifacenetlink to new sysctl plugin
- iface/ifacenetlink become consumers: emit EventBus events instead of writing directly
- iface gains sysctl dependency (tier ordering ensures sysctl ready first)
- fibkernel gains sysctl dependency and emits forwarding defaults via EventBus
- Global forwarding (net.ipv4.conf.all.forwarding, net.ipv6.conf.all.forwarding) added (previously missing entirely)
- Original sysctl values saved before first write, restored on clean daemon stop

## Data Flow (MANDATORY)

### Entry Point
Three entry points:
1. **Config commit:** user config parsed, sysctl plugin receives settings via OnConfigApply
2. **Plugin default:** fibkernel/iface emit `(sysctl, default)` on EventBus in OnStarted
3. **CLI transient:** CLI emits `(sysctl, set)` on EventBus

### Transformation Path
1. Config: YANG parsed -> JSON config section -> sysctl plugin OnConfigApply -> mark as config layer -> save original -> write to kernel -> emit `(sysctl, applied)`
2. Default: EventBus `(sysctl, default)` received -> check precedence -> if no config override: save original, write to kernel, emit `(sysctl, applied)`
3. Transient: EventBus `(sysctl, set)` received -> check precedence -> if no config override: save original, write to kernel, emit `(sysctl, applied)`
4. Query: EventBus `(sysctl, show-request)` received -> iterate all tracked keys -> emit `(sysctl, show-result)` with JSON table
5. Stop: iterate all saved originals -> restore each -> log at info level

### EventBus Events

| Namespace | Event Type | Direction | Payload |
|-----------|-----------|-----------|---------|
| `sysctl` | `default` | fib/iface -> sysctl | key, value, source (plugin name) |
| `sysctl` | `set` | CLI -> sysctl | key, value |
| `sysctl` | `show-request` | CLI -> sysctl | empty or filter |
| `sysctl` | `show-result` | sysctl -> requester | JSON table: key, value, source, persistent |
| `sysctl` | `list-request` | CLI -> sysctl | empty |
| `sysctl` | `list-result` | sysctl -> requester | JSON table: key, description, type, current value |
| `sysctl` | `describe-request` | CLI -> sysctl | key |
| `sysctl` | `describe-result` | sysctl -> requester | JSON detail |
| `sysctl` | `applied` | sysctl -> anyone | key, value, source. Any plugin can subscribe and react. |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| fibkernel -> sysctl | EventBus `(sysctl, default)` in OnStarted | [ ] |
| iface -> sysctl | EventBus `(sysctl, default)` during config apply | [ ] |
| CLI -> sysctl | EventBus `(sysctl, set)` and query events | [ ] |
| sysctl -> kernel | os.WriteFile to /proc/sys/ (Linux), syscall.Syscall6 SYS_SYSCTLBYNAME (Darwin) | [ ] |
| sysctl -> any plugin | EventBus `(sysctl, applied)` after each write | [ ] |

### Integration Points
- `registry.Registration.Dependencies` - fibkernel and iface add "sysctl"
- `ConfigureEventBus` - sysctl plugin receives EventBus for subscribe/emit
- `internal/core/sysctl/` - known keys registry (types + MustRegister)
- `internal/component/plugin/events.go` - new "sysctl" namespace + event types
- `internal/component/plugin/all/all.go` - blank import for new plugin

### Architectural Verification
- [ ] No bypassed layers (all sysctl writes go through sysctl plugin)
- [ ] No unintended coupling (contributors register known keys at init, not import sysctl)
- [ ] No duplicated functionality (ifacenetlink sysctl code deleted after migration)
- [ ] Zero-copy preserved where applicable (N/A - string operations only)

## Value Precedence Model

Three layers, strict priority. For any key, the effective value is the first present layer:

| Priority | Layer | Set by | Persists | Survives reboot |
|----------|-------|--------|----------|-----------------|
| 1 (highest) | config | User in YANG config | Yes | Yes |
| 2 | transient | User via CLI / EventBus `(sysctl, set)` | No | No |
| 3 (lowest) | default | Plugins via EventBus `(sysctl, default)` | No | No |

**Defaults are not suggestions.** They are the values a plugin requires to operate correctly
(e.g., fib-kernel requires forwarding=1). The user can override via config or transient to
say "I know what I'm doing," but without an override the default is the correct value.

**Override logging:** when config or transient overrides a plugin default, sysctl logs at
warn level: "<key> set to <override-value> (<source>), overriding <plugin-name> default (<default-value>)".

When config is applied: config values are authoritative. Transient and default values only
take effect for keys NOT present in config.

When a transient value is set: it overrides any default for that key. Does not affect config keys.

When a plugin sets a default: it only takes effect if no config or transient override exists.

**Restore on stop:** before the first write to any key, the sysctl plugin reads and saves
the original kernel value. On clean daemon stop, all saved originals are restored (info
log per key). If ze crashes, values remain as-is (unavoidable).

## Known Keys Registry

Lives in `internal/core/sysctl/` (mirrors `internal/core/family/` pattern). Leaf package
with types + `MustRegister`. No plugin dependencies. Imported by plugin init() functions.

Modules contribute known keys at init time. Known keys provide: kernel-native name,
type, valid range, description, platform availability.

| Contributor | Registered at | Keys |
|-------------|---------------|------|
| sysctl plugin | own init() | Core system keys (net.core.somaxconn, net.ipv4.tcp_syncookies, net.ipv4.conf.all.log_martians) |
| fibkernel | own init() | net.ipv4.conf.all.forwarding, net.ipv6.conf.all.forwarding |
| ifacenetlink | own init() | Per-interface keys (net.ipv4.conf.*.arp_announce, arp_ignore, rp_filter, etc.) |

Unknown keys: accepted by `sysctl set` and config, written as-is. No tab completion,
no validation, no description. User takes responsibility.

Known keys: tab completion in CLI, type/range validation, description in `sysctl list`
and `sysctl describe`.

### Key Naming

Keys use kernel-native names. No ze-specific translation layer. On Linux, the `/proc/sys/`
path with `/` replaced by `.` (matching `sysctl(8)` convention). On Darwin, the MIB name
as-is.

**System-wide keys (Linux):**
| Key | /proc/sys path | Type |
|-----|---------------|------|
| `net.ipv4.conf.all.forwarding` | `net/ipv4/conf/all/forwarding` | bool |
| `net.ipv6.conf.all.forwarding` | `net/ipv6/conf/all/forwarding` | bool |
| `net.ipv4.conf.all.rp_filter` | `net/ipv4/conf/all/rp_filter` | 0-2 |
| `net.ipv4.tcp_syncookies` | `net/ipv4/tcp_syncookies` | bool |
| `net.core.somaxconn` | `net/core/somaxconn` | int |
| `net.ipv4.conf.all.log_martians` | `net/ipv4/conf/all/log_martians` | bool |

**Per-interface keys (Linux):**
| Key pattern | /proc/sys path |
|-------------|---------------|
| `net.ipv4.conf.<iface>.forwarding` | `net/ipv4/conf/<iface>/forwarding` |
| `net.ipv4.conf.<iface>.arp_filter` | `net/ipv4/conf/<iface>/arp_filter` |
| `net.ipv4.conf.<iface>.arp_accept` | `net/ipv4/conf/<iface>/arp_accept` |
| `net.ipv4.conf.<iface>.proxy_arp` | `net/ipv4/conf/<iface>/proxy_arp` |
| `net.ipv4.conf.<iface>.arp_announce` | `net/ipv4/conf/<iface>/arp_announce` |
| `net.ipv4.conf.<iface>.arp_ignore` | `net/ipv4/conf/<iface>/arp_ignore` |
| `net.ipv4.conf.<iface>.rp_filter` | `net/ipv4/conf/<iface>/rp_filter` |
| `net.ipv6.conf.<iface>.autoconf` | `net/ipv6/conf/<iface>/autoconf` |
| `net.ipv6.conf.<iface>.accept_ra` | `net/ipv6/conf/<iface>/accept_ra` |
| `net.ipv6.conf.<iface>.forwarding` | `net/ipv6/conf/<iface>/forwarding` |

**Darwin keys:**
| Key | Type |
|-----|------|
| `net.inet.ip.forwarding` | bool |
| `net.inet6.ip6.forwarding` | bool |

## CLI Commands

| Command | Output | Notes |
|---------|--------|-------|
| `sysctl show` | Table of all active keys: key, value, source (default/config/transient), persistent (yes/no) | Only keys that have been set |
| `sysctl list` | Table of all known keys: key, description, type, current kernel value | Platform-filtered at compile time |
| `sysctl describe <key>` | Detail for one key: description, type, range, current value, source if set | Works for known keys; unknown keys show current value only |
| `sysctl set <key> <value>` | Writes value immediately, marks as transient | Tab-completes known keys; accepts unknown |

## YANG Config

Generic key/value list under `container sysctl`, with `list setting` keyed by `name`
(string) and a `value` leaf (string). Any sysctl the user puts in config is persistent
and wins over transient and default layers.

Example config: `sysctl { setting net.ipv4.conf.all.forwarding { value 0; } }`

Validated on commit: known keys get type/range validation; unknown keys are validated
by attempting the write (fail = reject config).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| fib-kernel config loaded | -> | sysctl plugin enables forwarding via `(sysctl, default)` event | `test/plugin/sysctl-fib-forwarding.ci` |
| CLI emits `(sysctl, set)` | -> | sysctl plugin writes to kernel, shows in `(sysctl, show-result)` | `test/plugin/sysctl-set-show.ci` |
| YANG config with sysctl setting | -> | config layer wins over fib-kernel default | `test/plugin/sysctl-config-override.ci` |
| Per-interface sysctl via iface config | -> | iface emits `(sysctl, default)`, sysctl plugin writes | `test/plugin/sysctl-per-iface.ci` |
| ze daemon clean stop | -> | sysctl plugin restores original values | `test/plugin/sysctl-restore-on-stop.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | fib-kernel loaded, no sysctl config | net.ipv4.conf.all.forwarding and net.ipv6.conf.all.forwarding set to 1 (fib default via EventBus) |
| AC-2 | fib-kernel loaded, config has `sysctl { setting net.ipv6.conf.all.forwarding { value 0; } }` | ipv4 forwarding=1 (fib default), ipv6 forwarding=0 (config wins). Warn log: "overriding fib-kernel default" |
| AC-3 | CLI emits `(sysctl, set)` with key=net.core.somaxconn value=4096 | Written to kernel, shows as transient in show-result |
| AC-4 | Transient net.core.somaxconn=4096 then config with same key value=1024 applied | somaxconn=1024 (config wins over transient) |
| AC-5 | `(sysctl, show-request)` emitted with mix of default/transient/config keys active | `(sysctl, show-result)` contains all active keys with correct source and persistent columns |
| AC-6 | `(sysctl, list-request)` on Linux | `(sysctl, list-result)` shows all known Linux keys with descriptions |
| AC-7 | `(sysctl, list-request)` on Darwin | `(sysctl, list-result)` shows only forwarding keys |
| AC-8 | `(sysctl, describe-request)` for net.ipv4.conf.all.forwarding | `(sysctl, describe-result)` shows description, type (bool), current value, source if set |
| AC-9 | `(sysctl, set)` with unknown key, kernel write succeeds | Value written, tracked as transient, no tab completion |
| AC-10 | Config with unknown key, kernel write fails on commit | Config rejected with error message |
| AC-11 | `(sysctl, set)` with known key and invalid value (net.ipv4.conf.all.rp_filter=5) | Rejected with error: valid range is 0-2 |
| AC-12 | Per-interface sysctl via iface YANG config | iface emits `(sysctl, default)`, sysctl plugin writes per-interface key |
| AC-13 | fibkernel depends on sysctl, sysctl starts first | Plugin manager starts sysctl (tier 0) before fibkernel (tier 1+) |
| AC-14 | iface depends on sysctl, sysctl starts first | Plugin manager starts sysctl before iface config apply |
| AC-15 | Known keys contributed by fibkernel and ifacenetlink at init | `(sysctl, list-result)` shows keys from all contributors |
| AC-16 | Darwin build: `(sysctl, set)` net.inet.ip.forwarding=1 | Written via syscall.Syscall6 + SYS_SYSCTLBYNAME |
| AC-17 | Darwin build: `(sysctl, set)` net.ipv4.conf.all.rp_filter=1 | Rejected: key not available on darwin |
| AC-18 | Clean daemon stop with sysctl values modified | All modified keys restored to original values. Info log per key restored. |
| AC-19 | Plugin emits `(sysctl, default)` overridden by config | Warn log: "<key> set to <config-value> (config), overriding <plugin> default (<default-value>)" |
| AC-20 | Any plugin subscribes to `(sysctl, applied)` | Receives event after each sysctl write with key, value, source |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValuePrecedence` | `internal/plugins/sysctl/sysctl_test.go` | Config > transient > default ordering | |
| `TestConfigOverridesDefault` | `internal/plugins/sysctl/sysctl_test.go` | Config key blocks plugin default, warn logged | |
| `TestTransientOverridesDefault` | `internal/plugins/sysctl/sysctl_test.go` | Transient key wins over default | |
| `TestConfigOverridesTransient` | `internal/plugins/sysctl/sysctl_test.go` | Config key wins over transient | |
| `TestKnownKeyValidation` | `internal/plugins/sysctl/sysctl_test.go` | Bool, int, int-range validation | |
| `TestUnknownKeyAccepted` | `internal/plugins/sysctl/sysctl_test.go` | Unknown key written without validation | |
| `TestKnownKeyRegistration` | `internal/core/sysctl/known_test.go` | MustRegister stores and retrieves metadata | |
| `TestDuplicateRegistration` | `internal/core/sysctl/known_test.go` | Same key re-registered panics | |
| `TestShowResult` | `internal/plugins/sysctl/sysctl_test.go` | show-result formats JSON with source/persistent | |
| `TestListResult` | `internal/plugins/sysctl/sysctl_test.go` | list-result includes all known keys | |
| `TestDescribeKnown` | `internal/plugins/sysctl/sysctl_test.go` | describe-result returns full metadata for known key | |
| `TestDescribeUnknown` | `internal/plugins/sysctl/sysctl_test.go` | describe-result returns current value only for unknown key | |
| `TestBackendLinux` | `internal/plugins/sysctl/backend_linux_test.go` | Writes to overridden /proc/sys path | |
| `TestBackendDarwin` | `internal/plugins/sysctl/backend_darwin_test.go` | Forwarding keys use SYS_SYSCTLBYNAME syscall | |
| `TestRestoreOnStop` | `internal/plugins/sysctl/sysctl_test.go` | Original values saved before write, restored on stop | |
| `TestOverrideWarnLog` | `internal/plugins/sysctl/sysctl_test.go` | Config overriding a default emits warn log with plugin name | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rp-filter | 0-2 | 2 | N/A (0 valid) | 3 |
| arp-announce | 0-2 | 2 | N/A (0 valid) | 3 |
| arp-ignore | 0-2 | 2 | N/A (0 valid) | 3 |
| accept-ra | 0-2 | 2 | N/A (0 valid) | 3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `sysctl-fib-forwarding` | `test/plugin/sysctl-fib-forwarding.ci` | fib-kernel loaded, verify forwarding defaults emitted on bus | |
| `sysctl-set-show` | `test/plugin/sysctl-set-show.ci` | User sets transient value via bus, show-result lists it | |
| `sysctl-config-override` | `test/plugin/sysctl-config-override.ci` | Config value wins over fib-kernel default, warn logged | |
| `sysctl-per-iface` | `test/plugin/sysctl-per-iface.ci` | Per-interface sysctl via iface config emits default on bus | |
| `sysctl-list` | `test/plugin/sysctl-list.ci` | list-result shows known keys with descriptions | |
| `sysctl-restore-on-stop` | `test/plugin/sysctl-restore-on-stop.ci` | Clean stop restores original values | |

### Future (if deferring any tests)
- FreeBSD backend (no FreeBSD in CI)

## Files to Modify

- `internal/plugins/fibkernel/register.go` - add "sysctl" to Dependencies, add ConfigureEventBus
- `internal/plugins/fibkernel/fibkernel.go` - emit `(sysctl, default)` for forwarding in OnStarted
- `internal/component/iface/backend.go` - remove sysctl methods from Backend interface
- `internal/component/iface/dispatch.go` - remove sysctl dispatch functions
- `internal/component/iface/config.go` - applySysctl emits `(sysctl, default)` on EventBus instead of backend calls
- `internal/component/iface/register.go` - add "sysctl" to Dependencies (ifacenetlink registration)
- `internal/plugins/ifacenetlink/sysctl_linux.go` - delete (code moves to sysctl plugin)
- `internal/plugins/ifacenetlink/sysctl_linux_test.go` - delete (tests move to sysctl plugin)
- `internal/plugins/ifacenetlink/register.go` - add "sysctl" to Dependencies
- `internal/component/plugin/all/all.go` - add blank import for sysctl plugin
- `internal/component/plugin/events.go` - add NamespaceSysctl + sysctl event types

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` |
| CLI commands/flags | Yes | YANG-driven (sysctl show, set, list, describe) |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test for new RPC/API | Yes | `test/plugin/sysctl-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add sysctl management |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - sysctl config section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - sysctl commands |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - sysctl RPCs |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - sysctl plugin entry |
| 6 | Has a user guide page? | Yes | `docs/guide/sysctl.md` - new page |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - sysctl management column |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - sysctl plugin section |

## Files to Create

### Core registry (`internal/core/sysctl/`)
- `internal/core/sysctl/known.go` - KnownSysctl type, MustRegister, Lookup, All, Validate
- `internal/core/sysctl/known_test.go` - registry tests

### Plugin (`internal/plugins/sysctl/`)
- `internal/plugins/sysctl/sysctl.go` - plugin logic: value store, precedence, event handlers, restore-on-stop
- `internal/plugins/sysctl/backend.go` - backend interface (read/write/restore)
- `internal/plugins/sysctl/backend_linux.go` - os.WriteFile/ReadFile to /proc/sys/
- `internal/plugins/sysctl/backend_darwin.go` - syscall.Syscall6 + SYS_SYSCTLBYNAME for forwarding
- `internal/plugins/sysctl/backend_other.go` - no-op
- `internal/plugins/sysctl/known_linux.go` - Linux known keys registered at init
- `internal/plugins/sysctl/known_darwin.go` - Darwin known keys (forwarding only) registered at init
- `internal/plugins/sysctl/register.go` - init() registration with ConfigureEventBus
- `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` - YANG schema
- `internal/plugins/sysctl/schema/embed.go` - YANG embed
- `internal/plugins/sysctl/sysctl_test.go` - unit tests (precedence, events, restore, logging)
- `internal/plugins/sysctl/backend_linux_test.go` - Linux backend tests
- `internal/plugins/sysctl/backend_darwin_test.go` - Darwin backend tests

### Functional tests
- `test/plugin/sysctl-fib-forwarding.ci` - fib-kernel forwarding wiring test
- `test/plugin/sysctl-set-show.ci` - set + show functional test
- `test/plugin/sysctl-config-override.ci` - config overrides default test
- `test/plugin/sysctl-per-iface.ci` - per-interface EventBus dispatch test
- `test/plugin/sysctl-list.ci` - list command functional test
- `test/plugin/sysctl-restore-on-stop.ci` - restore originals on clean stop

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: core known-keys registry** -- KnownSysctl type, MustRegister, Lookup, Validate in `internal/core/sysctl/`
   - Tests: TestKnownKeyRegistration, TestDuplicateRegistration
   - Files: internal/core/sysctl/known.go, known_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: event namespace** -- add "sysctl" namespace and event types to events.go
   - Tests: compilation + existing event tests still pass
   - Files: internal/component/plugin/events.go
   - Verify: `make ze-lint`

3. **Phase: backend** -- platform-specific sysctl read/write with original value save/restore
   - Tests: TestBackendLinux, TestBackendDarwin
   - Files: backend.go, backend_linux.go, backend_darwin.go, backend_other.go + tests
   - Verify: tests fail -> implement -> tests pass

4. **Phase: value store and precedence** -- three-layer store with config > transient > default, restore-on-stop
   - Tests: TestValuePrecedence, TestConfigOverridesDefault, TestTransientOverridesDefault, TestConfigOverridesTransient, TestRestoreOnStop, TestOverrideWarnLog
   - Files: sysctl.go, sysctl_test.go
   - Verify: tests fail -> implement -> tests pass

5. **Phase: EventBus handlers** -- subscribe to default/set/show-request/list-request/describe-request, emit results and applied
   - Tests: TestShowResult, TestListResult, TestDescribeKnown, TestDescribeUnknown, TestUnknownKeyAccepted, TestKnownKeyValidation
   - Files: sysctl.go, sysctl_test.go
   - Verify: tests fail -> implement -> tests pass

6. **Phase: plugin registration and YANG** -- register.go, schema, ConfigureEventBus, OnConfigApply, platform known keys
   - Tests: compilation + functional tests
   - Files: register.go, schema/ze-sysctl-conf.yang, schema/embed.go, known_linux.go, known_darwin.go
   - Verify: compiles, `make ze-lint`

7. **Phase: fibkernel integration** -- add sysctl dependency, emit forwarding defaults on EventBus in OnStarted
   - Tests: functional test sysctl-fib-forwarding.ci
   - Files: fibkernel/register.go, fibkernel/fibkernel.go
   - Verify: functional test passes

8. **Phase: iface migration** -- move sysctl writes from ifacenetlink to EventBus emit, add sysctl dependency
   - Tests: functional test sysctl-per-iface.ci, existing iface tests still pass
   - Files: iface/backend.go, iface/dispatch.go, iface/config.go, ifacenetlink/register.go, delete ifacenetlink/sysctl_linux.go
   - Verify: all existing iface tests pass, new functional test passes

9. **Functional tests** -- all .ci tests for end-to-end scenarios including restore-on-stop
10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- audit, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Precedence ordering correct (config > transient > default), kernel-native key paths resolve correctly per platform |
| Naming | Keys use kernel-native names, YANG uses kebab-case, JSON payload keys use kebab-case |
| Data flow | All sysctl writes flow through EventBus -> sysctl plugin, no direct /proc/sys writes remain in iface |
| Rule: no-layering | ifacenetlink sysctl code fully deleted, not kept alongside new code |
| Rule: integration-completeness | Every event reachable from entry point, fib-kernel forwarding proven via .ci test |
| Restore | Original values saved before first write, restored on clean stop, info logged |
| Override logging | Warn emitted when config/transient overrides plugin default, includes plugin name |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Core registry exists | `ls internal/core/sysctl/known.go` |
| sysctl plugin registered | `grep 'sysctl' internal/component/plugin/all/all.go` |
| sysctl namespace in events.go | `grep 'NamespaceSysctl' internal/component/plugin/events.go` |
| fibkernel depends on sysctl | `grep 'sysctl' internal/plugins/fibkernel/register.go` |
| iface depends on sysctl | `grep 'sysctl' internal/plugins/ifacenetlink/register.go` |
| ifacenetlink sysctl code deleted | `ls internal/plugins/ifacenetlink/sysctl_linux.go` returns not found |
| Known keys available on Linux | TestListResult shows Linux keys |
| Known keys available on Darwin | TestListResult shows forwarding keys only |
| EventBus show works | sysctl-set-show.ci passes |
| forwarding enabled by fib | sysctl-fib-forwarding.ci passes |
| config overrides default with warn | sysctl-config-override.ci passes |
| Per-interface EventBus works | sysctl-per-iface.ci passes |
| Restore on stop works | sysctl-restore-on-stop.ci passes |
| Darwin backend uses syscall | `grep 'SYS_SYSCTLBYNAME' internal/plugins/sysctl/backend_darwin.go` |
| All existing iface tests pass | `make ze-verify` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Key names validated (no path traversal in /proc/sys writes), values validated for known keys |
| Path traversal | Linux: key-to-path conversion replaces `.` with `/`, reject keys containing `..` or absolute paths |
| Privilege | sysctl writes require CAP_NET_ADMIN or root; document requirement |
| Unknown key writes | Write failure (permission, nonexistent) returns error, does not crash |
| Darwin syscall safety | SYS_SYSCTLBYNAME called with correct buffer sizes, errno checked, no buffer overflow |
| Restore safety | Restore-on-stop must not panic if backend read fails (log and skip) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation
N/A - no RFC implementation.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/578-sysctl-0-plugin.md`
- [ ] Summary included in commit
