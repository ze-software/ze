# Spec: sysctl-1-profiles

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
3. `internal/core/sysctl/known.go` - existing known-keys registry to extend
4. `internal/plugins/sysctl/sysctl.go` - store, setDefault, conflict checking target
5. `internal/component/iface/config.go` - applySysctl, where profiles are resolved and emitted
6. `internal/component/iface/schema/ze-iface-conf.yang` - unit grouping where sysctl-profile leaf-list goes
7. `plan/learned/581-sysctl-0-plugin.md` - base sysctl plugin decisions

## Task

Add named sysctl profiles: reusable collections of kernel tunables applied to
interface units. Ship 5 built-in profiles for common network operator use cases
(DSR/anycast, router, hardened, multihomed, transparent proxy). Allow users to
define custom profiles in the `sysctl {}` config block. Profiles emit as
defaults via EventBus (same layer as fib-kernel forwarding). Multiple profiles
per unit, composable. Per-sysctl conflict detection with warn logging.

Replaces the current workflow of remembering individual sysctl recipes with
intent-based declarations: the operator says "this interface does DSR" and ze
sets arp_announce=2, arp_ignore=1 automatically.

## Design Decisions

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Approach | Profiles over individual leaves | Groups sysctls that must be set together (arp_announce + arp_ignore for DSR). Matches BGP `role` pattern. |
| 2 | Config location | `sysctl-profile` leaf-list on the interface unit | Per-interface sysctls map to units; same location as existing ipv4/ipv6 sysctl leaves |
| 3 | Extensibility | Built-in + user-definable | Built-in for convenience; user-defined for transparency (user sees exactly what's set) |
| 4 | Built-in set | `dsr`, `router`, `hardened`, `multihomed`, `proxy` | 5 concrete network operator use cases |
| 5 | Precedence | Profiles emit as default layer | Same as fib-kernel forwarding; explicit `sysctl { setting ... }` config overrides |
| 6 | Composition | Multiple profiles per unit (leaf-list, last wins on key conflict) | Matches filter list pattern `[ prefix-list:X community:Y ]`; DSR+hardened is natural |
| 7 | Code location | Core registry (`internal/core/sysctl/`) | Leaf package, no plugin deps; offline `ze sysctl list-profiles` works |
| 8 | Conflict detection | Per-sysctl conflict table in core registry | Catches conflicts in user-defined profiles too, not just built-in pairs |
| 9 | Conflict action | Warn only, apply anyway | Operator may override via explicit sysctl config; consistent with existing override logging |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - sysctl plugin section (added in sysctl-0)
  → Constraint: All sysctl writes flow through EventBus, sysctl plugin is single writer
- [ ] `.claude/rules/plugin-design.md` - import rules, EventBus patterns
  → Constraint: iface MUST NOT import sysctl plugin; communicate via EventBus or core registry
- [ ] `.claude/rules/config-design.md` - YANG structure, grouping vs augment
  → Constraint: sysctl-profile added to iface YANG via grouping (same component), not augment
- [ ] `.claude/rules/design-principles.md` - explicit > implicit, YAGNI
  → Decision: profiles declare their contents explicitly, no hidden sysctl-setting inference
- [ ] `plan/learned/581-sysctl-0-plugin.md` - base plugin architecture
  → Constraint: profiles emit as defaults; three-layer precedence unchanged

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- Profile definitions live in core registry alongside known keys (no plugin dependency)
- iface plugin resolves profile names at config apply, emits `(sysctl, default)` events per key
- Conflict table is per-sysctl-key, not per-profile, so user-defined profiles are checked too
- Built-in profiles are registered at init time; user-defined profiles at config parse time

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/sysctl/known.go` - KeyDef, MustRegister, Lookup, Validate, All
  → Constraint: extend with ProfileDef, MustRegisterProfile, LookupProfile, AllProfiles, RegisterConflict
- [ ] `internal/core/sysctl/known_linux.go` - Linux known keys (16 keys)
  → Decision: built-in profile definitions go in new `profiles.go` (platform-independent; profiles use `<iface>` templates)
- [ ] `internal/plugins/sysctl/sysctl.go` - store, setDefault, appliedJSON
  → Constraint: setDefault already handles key conflicts between layers; profile emission uses same path
- [ ] `internal/plugins/sysctl/register.go` - EventBus handlers, OnExecuteCommand
  → Decision: add `sysctl list-profiles` and `sysctl describe-profile` commands
- [ ] `internal/component/iface/config.go` - applySysctl emits EventBus defaults per ipv4/ipv6 leaf
  → Decision: add applySysctlProfiles that resolves profiles and emits defaults for each key in the profile
- [ ] `internal/component/iface/config.go:108-125` - ipv4Sysctl/ipv6Sysctl structs, unitEntry
  → Decision: add SysctlProfiles []string field to unitEntry
- [ ] `internal/component/iface/schema/ze-iface-conf.yang:68-190` - interface-unit grouping
  → Decision: add `leaf-list sysctl-profile` to the unit grouping
- [ ] `cmd/ze/sysctl/main.go` - offline CLI
  → Decision: add `list-profiles` and `describe-profile` subcommands

**Behavior to preserve:**
- Existing per-interface ipv4/ipv6 sysctl leaves continue to work unchanged
- Existing `sysctl { setting ... }` config continues to override profiles (config > default)
- fib-kernel forwarding defaults unaffected
- Three-layer precedence model unchanged
- `sysctl show` displays source correctly (profile name as source for profile-emitted defaults)

**Behavior to change:**
- Add `sysctl-profile` leaf-list to interface units
- Add `profile` list to `sysctl {}` config block for user-defined profiles
- Add profile registry to `internal/core/sysctl/`
- Add conflict table and warn logging to sysctl store
- Add CLI commands for profile inspection
- iface plugin resolves and emits profile sysctls during config apply

## Data Flow (MANDATORY)

### Entry Point
Three entry points for profile data:
1. **Built-in profiles:** registered at init time in `internal/core/sysctl/profiles.go`
2. **User-defined profiles:** parsed from `sysctl { profile <name> { setting ... } }` in OnConfigure
3. **Profile application:** parsed from `interface { ... unit N { sysctl-profile [ dsr hardened ]; } }` in iface OnConfigure/OnConfigApply

### Transformation Path
1. Built-in profiles registered via `sysctl.MustRegisterProfile(ProfileDef{...})` at init
2. User-defined profiles registered via `sysctl.RegisterProfile(ProfileDef{...})` from sysctl plugin OnConfigure
3. iface config parser reads `sysctl-profile` leaf-list from unit, stores as `[]string` in unitEntry
4. `applySysctlProfiles(osName, profiles)` iterates profiles in order, calls `sysctl.LookupProfile(name)` for each
5. For each profile, iterates settings, substitutes `<iface>` with osName, emits `(sysctl, default)` with `source: "profile:<name>"`
6. sysctl plugin receives defaults, checks conflict table, warns if conflicting keys detected
7. sysctl plugin writes to kernel via backend (existing flow)
8. Last profile wins on key conflict within the same unit (later emit overwrites earlier default)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| core registry ← init | `MustRegisterProfile()` at package init | [ ] |
| sysctl plugin ← config | `RegisterProfile()` from OnConfigure | [ ] |
| iface ← core registry | `LookupProfile()` (read-only, no plugin import) | [ ] |
| iface → sysctl plugin | EventBus `(sysctl, default)` with `source: "profile:<name>"` | [ ] |
| sysctl plugin ← conflict table | `CheckConflicts()` on active defaults per interface | [ ] |
| CLI ← core registry | `AllProfiles()` for `ze sysctl list-profiles` | [ ] |

### Integration Points
- `internal/core/sysctl/` - new ProfileDef type, profile registry, conflict table
- `internal/plugins/sysctl/register.go` - parse user-defined profiles from config, new CLI commands
- `internal/component/iface/config.go` - parse sysctl-profile leaf-list, emit profile defaults
- `internal/component/iface/schema/ze-iface-conf.yang` - sysctl-profile leaf-list in unit grouping
- `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` - profile list in sysctl container
- `cmd/ze/sysctl/main.go` - list-profiles, describe-profile subcommands

### Architectural Verification
- [ ] No bypassed layers (profiles flow through EventBus like all other sysctl defaults)
- [ ] No unintended coupling (iface imports core/sysctl, not plugins/sysctl)
- [ ] No duplicated functionality (profiles use existing setDefault path, not a parallel write mechanism)
- [ ] Zero-copy preserved where applicable (N/A - string operations only)

## Built-in Profile Definitions

| Profile | Key (with `<iface>` template) | Value | Purpose |
|---------|-------------------------------|-------|---------|
| **dsr** | `net.ipv4.conf.<iface>.arp_announce` | `2` | Only announce source IP from interface's subnet |
| | `net.ipv4.conf.<iface>.arp_ignore` | `1` | Only reply if target IP is on receiving interface |
| **router** | `net.ipv4.conf.<iface>.forwarding` | `1` | Enable IPv4 forwarding on this interface |
| | `net.ipv6.conf.<iface>.forwarding` | `1` | Enable IPv6 forwarding on this interface |
| **hardened** | `net.ipv4.conf.<iface>.rp_filter` | `1` | Strict reverse path filtering (anti-spoofing) |
| | `net.ipv4.conf.<iface>.log_martians` | `1` | Log packets with impossible source addresses |
| | `net.ipv4.conf.<iface>.arp_filter` | `1` | Respond to ARP only on correct interface |
| **multihomed** | `net.ipv4.conf.<iface>.arp_filter` | `1` | Prevent ARP flux on multi-NIC hosts |
| **proxy** | `net.ipv4.conf.<iface>.proxy_arp` | `1` | Answer ARP requests for non-local IPs |
| | `net.ipv4.conf.<iface>.arp_accept` | `1` | Accept gratuitous ARP replies |

Note: `hardened` also needs global keys `net.ipv4.tcp_syncookies=1` and `net.ipv4.conf.all.log_martians=1`,
but those are system-wide, not per-interface. The per-interface `log_martians` and `rp_filter` are the
profile's responsibility. The global keys should be set via explicit `sysctl { setting ... }` config
or a future `sysctl-global-profile` feature. This avoids a profile on one interface silently changing
global kernel state.

## Conflict Table

Per-sysctl conflicts: warn when both keys are active on the same interface.

| Key A (value) | Key B (value) | Why incompatible |
|---------------|---------------|-----------------|
| `arp_ignore=1` | `proxy_arp=1` | Ignore ARP for non-local IPs contradicts proxy ARP for them |
| `rp_filter=1` (strict) | `proxy_arp=1` | Strict RPF drops packets for non-local destinations; proxy ARP advertises reachability |
| `arp_announce=2` | `proxy_arp=1` | Best-source-only announcement contradicts answering for others' IPs |

Conflicts are checked per-interface: when a `(sysctl, default)` event is processed for a key
that has a conflict partner already active as a default on the same interface, the sysctl plugin
logs at warn level with both profile names.

## Config Examples

### Basic DSR setup
```
interface {
    loopback {
        unit 0 {
            address [ 10.0.0.1/32 10.0.0.2/32 ]
        }
    }
    ethernet eth0 {
        unit 0 {
            sysctl-profile [ dsr ]
            address [ 192.168.1.10/24 ]
        }
    }
}
```

### Combined DSR + hardening
```
interface {
    ethernet eth0 {
        unit 0 {
            sysctl-profile [ dsr hardened ]
        }
    }
}
```
Result: arp_announce=2, arp_ignore=1, rp_filter=1, log_martians=1, arp_filter=1

### User-defined profile
```
sysctl {
    profile my-edge {
        setting net.ipv4.conf.<iface>.arp_announce { value 1; }
        setting net.ipv4.conf.<iface>.rp_filter { value 2; }
        setting net.ipv4.conf.<iface>.forwarding { value 1; }
    }
}

interface {
    ethernet eth0 {
        unit 0 {
            sysctl-profile [ my-edge ]
        }
    }
}
```

### Override one key from a profile
```
sysctl {
    setting net.ipv4.conf.eth0.arp_announce { value 1; }
}

interface {
    ethernet eth0 {
        unit 0 {
            sysctl-profile [ dsr ]
        }
    }
}
```
Result: arp_announce=1 (config wins), arp_ignore=1 (profile default applies).
Warn log: "net.ipv4.conf.eth0.arp_announce set to 1 (config), overriding profile:dsr default (2)"

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| iface config with `sysctl-profile [ dsr ]` | -> | applySysctlProfiles emits defaults via EventBus | `test/parse/sysctl-profile-dsr.ci` |
| user-defined profile in sysctl config | -> | sysctl plugin registers profile, iface emits its keys | `test/parse/sysctl-profile-custom.ci` |
| two conflicting profiles on same unit | -> | sysctl plugin warns on conflict | `test/plugin/sysctl-profile-conflict.ci` |
| `ze sysctl list-profiles` offline | -> | core registry returns all profiles | `test/parse/sysctl-list-profiles.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Unit with `sysctl-profile [ dsr ]` | arp_announce=2 and arp_ignore=1 emitted as defaults for that interface |
| AC-2 | Unit with `sysctl-profile [ dsr hardened ]` | All 5 keys from both profiles emitted; last-wins on overlap (hardened's arp_filter after dsr) |
| AC-3 | Unit with `sysctl-profile [ dsr ]` + explicit `sysctl { setting <iface>.arp_announce { value 1; } }` | Config (value 1) wins over profile default (value 2). Warn log emitted. |
| AC-4 | Unit with `sysctl-profile [ dsr proxy ]` (conflicting) | Both profiles applied (last wins). Warn log: arp_ignore=1 conflicts with proxy_arp=1. |
| AC-5 | User-defined profile `my-edge` in sysctl config, referenced by interface unit | Profile keys emitted as defaults, `sysctl show` shows source `profile:my-edge` |
| AC-6 | `ze sysctl list-profiles` | Table of all registered profiles (built-in + user-defined) with names and key counts |
| AC-7 | `ze sysctl describe-profile dsr` | JSON detail: profile name, all key/value pairs, description |
| AC-8 | Unknown profile name in `sysctl-profile [ nosuch ]` | Config verify rejects with error: unknown profile "nosuch" |
| AC-9 | `ze sysctl list-profiles` on Darwin | Shows all 5 built-in profiles (they use `<iface>` templates, platform-independent definitions) |
| AC-10 | `sysctl show` after profile applied | Each key shows source as `profile:<name>` not just `interface` |
| AC-11 | User-defined profile with `<iface>` template in key names | Template substituted with actual interface name at apply time |
| AC-12 | Config reload removes `sysctl-profile` from a unit | Profile defaults cleared; keys fall back to transient/default or original |
| AC-13 | User-defined profile overrides built-in with same name | User-defined wins. Warn log at registration time. |
| AC-14 | `sysctl-profile` leaf-list on loopback unit | Profiles applied to loopback interface (lo) |
| AC-15 | `sysctl-profile` on VLAN unit (eth0.100) | Profile keys use `eth0.100` as interface name via `<iface>` substitution |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMustRegisterProfile` | `internal/core/sysctl/profile_test.go` | Profile registration and lookup | |
| `TestDuplicateProfileRegistration` | `internal/core/sysctl/profile_test.go` | Built-in duplicate panics | |
| `TestUserProfileOverridesBuiltin` | `internal/core/sysctl/profile_test.go` | RegisterProfile overwrites built-in with warn | |
| `TestLookupProfileUnknown` | `internal/core/sysctl/profile_test.go` | Unknown name returns false | |
| `TestAllProfiles` | `internal/core/sysctl/profile_test.go` | Returns all registered profiles | |
| `TestBuiltinDSR` | `internal/core/sysctl/profile_test.go` | DSR profile has arp_announce=2, arp_ignore=1 | |
| `TestBuiltinHardened` | `internal/core/sysctl/profile_test.go` | Hardened profile has rp_filter=1, log_martians=1, arp_filter=1 | |
| `TestBuiltinRouter` | `internal/core/sysctl/profile_test.go` | Router profile has ipv4+ipv6 forwarding=1 | |
| `TestBuiltinMultihomed` | `internal/core/sysctl/profile_test.go` | Multihomed profile has arp_filter=1 | |
| `TestBuiltinProxy` | `internal/core/sysctl/profile_test.go` | Proxy profile has proxy_arp=1, arp_accept=1 | |
| `TestConflictRegistration` | `internal/core/sysctl/profile_test.go` | Conflict table registration and lookup | |
| `TestCheckConflicts` | `internal/core/sysctl/profile_test.go` | Detects arp_ignore + proxy_arp conflict | |
| `TestCheckConflictsNoMatch` | `internal/core/sysctl/profile_test.go` | No conflict for dsr + hardened | |
| `TestTemplateSubstitution` | `internal/core/sysctl/profile_test.go` | `<iface>` replaced with interface name | |
| `TestProfileSourceInShow` | `internal/plugins/sysctl/sysctl_test.go` | setDefault with source `profile:dsr` shows in showEntries | |
| `TestParseProfileConfig` | `internal/plugins/sysctl/sysctl_test.go` | User-defined profile parsed from JSON config | |
| `TestApplySysctlProfiles` | `internal/component/iface/config_test.go` | Profiles resolved and emitted on EventBus | |
| `TestMultipleProfilesLastWins` | `internal/component/iface/config_test.go` | Last profile's value used for overlapping keys | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Profile name length | 1-64 | 64 chars | empty string | 65 chars |
| Settings per profile | 1-50 | 50 | 0 (empty) | 51 |
| Profiles per unit | 1-10 | 10 | 0 (empty leaf-list) | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `sysctl-profile-dsr` | `test/parse/sysctl-profile-dsr.ci` | Config with dsr profile validates | |
| `sysctl-profile-custom` | `test/parse/sysctl-profile-custom.ci` | User-defined profile in config validates | |
| `sysctl-profile-conflict` | `test/plugin/sysctl-profile-conflict.ci` | Two conflicting profiles, warn logged | |
| `sysctl-list-profiles` | `test/parse/sysctl-list-profiles.ci` | `ze sysctl list-profiles` shows built-in profiles | |
| `sysctl-profile-unknown` | `test/parse/sysctl-profile-unknown.ci` | Unknown profile name rejected at validation | |
| `sysctl-profile-combined` | `test/parse/sysctl-profile-combined.ci` | `sysctl-profile [ dsr hardened ]` validates | |

### Future (if deferring any tests)
- `sysctl-global-profile`: system-wide profiles (tcp_syncookies, etc.) are out of scope for this spec
- FreeBSD backend (no FreeBSD in CI)

## Files to Modify

- `internal/core/sysctl/known.go` - add ProfileDef type, profile registry functions, conflict table
- `internal/plugins/sysctl/register.go` - parse user-defined profiles from config, add list-profiles/describe-profile commands
- `internal/plugins/sysctl/sysctl.go` - conflict checking in setDefault, `profile:<name>` source tracking
- `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` - add `list profile` to sysctl container
- `internal/component/iface/config.go` - add applySysctlProfiles, parse sysctl-profile leaf-list
- `internal/component/iface/schema/ze-iface-conf.yang` - add sysctl-profile leaf-list to interface-unit
- `cmd/ze/sysctl/main.go` - add list-profiles, describe-profile subcommands
- `cmd/ze/main.go` - update help text to mention profiles

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (sysctl container) | Yes | `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` |
| YANG schema (iface unit) | Yes | `internal/component/iface/schema/ze-iface-conf.yang` |
| CLI commands/flags | Yes | `cmd/ze/sysctl/main.go` |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new feature | Yes | `test/parse/sysctl-profile-*.ci`, `test/plugin/sysctl-profile-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add sysctl profiles |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - profile config examples |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - list-profiles, describe-profile |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - profile support in sysctl plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` - sysctl profile section |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - sysctl profiles unique to ze |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - profile registry |

## Files to Create

### Core registry extensions
- `internal/core/sysctl/profiles.go` - ProfileDef type, MustRegisterProfile, RegisterProfile, LookupProfile, AllProfiles, built-in definitions
- `internal/core/sysctl/conflicts.go` - ConflictRule, RegisterConflict, CheckConflicts
- `internal/core/sysctl/profile_test.go` - profile and conflict tests

### Functional tests
- `test/parse/sysctl-profile-dsr.ci` - DSR profile config validates
- `test/parse/sysctl-profile-custom.ci` - user-defined profile validates
- `test/parse/sysctl-profile-combined.ci` - combined profiles validate
- `test/parse/sysctl-profile-unknown.ci` - unknown profile rejected (expect failure)
- `test/parse/sysctl-list-profiles.ci` - offline list-profiles command
- `test/plugin/sysctl-profile-conflict.ci` - conflicting profiles warn

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

1. **Phase: core profile registry** -- ProfileDef type, MustRegisterProfile, RegisterProfile, LookupProfile, AllProfiles, template substitution
   - Tests: TestMustRegisterProfile, TestDuplicateProfileRegistration, TestUserProfileOverridesBuiltin, TestLookupProfileUnknown, TestAllProfiles, TestTemplateSubstitution
   - Files: internal/core/sysctl/profiles.go, profile_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: built-in profiles** -- register 5 built-in profiles at init
   - Tests: TestBuiltinDSR, TestBuiltinHardened, TestBuiltinRouter, TestBuiltinMultihomed, TestBuiltinProxy
   - Files: internal/core/sysctl/profiles.go (init function)
   - Verify: tests pass

3. **Phase: conflict table** -- ConflictRule, RegisterConflict, CheckConflicts
   - Tests: TestConflictRegistration, TestCheckConflicts, TestCheckConflictsNoMatch
   - Files: internal/core/sysctl/conflicts.go, profile_test.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: sysctl plugin profile parsing** -- parse user-defined profiles from sysctl config, register via RegisterProfile
   - Tests: TestParseProfileConfig
   - Files: internal/plugins/sysctl/sysctl.go (parseProfileConfig), register.go (OnConfigure extension)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: sysctl plugin conflict checking** -- check conflict table on setDefault, warn log
   - Tests: TestProfileSourceInShow
   - Files: internal/plugins/sysctl/sysctl.go (extend setDefault)
   - Verify: tests pass

6. **Phase: sysctl YANG** -- add `list profile` to sysctl container
   - Files: internal/plugins/sysctl/schema/ze-sysctl-conf.yang
   - Verify: `make ze-lint`

7. **Phase: iface YANG + config parsing** -- add sysctl-profile leaf-list, parse into unitEntry, applySysctlProfiles
   - Tests: TestApplySysctlProfiles, TestMultipleProfilesLastWins
   - Files: internal/component/iface/schema/ze-iface-conf.yang, internal/component/iface/config.go
   - Verify: tests fail -> implement -> tests pass

8. **Phase: CLI commands** -- list-profiles, describe-profile in offline CLI; list-profiles, describe-profile in daemon OnExecuteCommand
   - Files: cmd/ze/sysctl/main.go, internal/plugins/sysctl/register.go
   - Verify: `ze sysctl list-profiles` shows 5 profiles

9. **Phase: functional tests** -- all .ci tests
   - Files: test/parse/sysctl-profile-*.ci, test/plugin/sysctl-profile-conflict.ci
   - Verify: all .ci tests pass

10. **Phase: full verification** -- `make ze-verify`
11. **Phase: complete spec** -- audit, docs, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Profile resolution order matches leaf-list order (last wins). Template substitution produces correct kernel key names. |
| Naming | Profile names kebab-case. JSON output uses kebab-case keys. YANG leaf-list named `sysctl-profile`. |
| Data flow | All profile sysctl writes flow through EventBus -> sysctl plugin. No direct /proc/sys writes from iface. |
| Rule: no-layering | No parallel write path for profile sysctls. Profiles use existing setDefault. |
| Rule: explicit > implicit | Profile contents visible via `ze sysctl describe-profile`. No hidden sysctl settings. |
| Conflict detection | arp_ignore+proxy_arp, rp_filter+proxy_arp, arp_announce+proxy_arp all detected |
| Source tracking | `sysctl show` displays `profile:<name>` as source, not generic `interface` |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Profile registry in core | `grep 'MustRegisterProfile' internal/core/sysctl/profiles.go` |
| 5 built-in profiles | `go test -run TestBuiltin -v ./internal/core/sysctl/...` |
| Conflict table | `grep 'RegisterConflict' internal/core/sysctl/conflicts.go` |
| sysctl-profile in iface YANG | `grep 'sysctl-profile' internal/component/iface/schema/ze-iface-conf.yang` |
| profile list in sysctl YANG | `grep 'list profile' internal/plugins/sysctl/schema/ze-sysctl-conf.yang` |
| User-defined profile parsing | `grep 'parseProfileConfig' internal/plugins/sysctl/sysctl.go` |
| Offline list-profiles command | `bin/ze sysctl list-profiles` shows 5 profiles |
| Offline describe-profile command | `bin/ze sysctl describe-profile dsr` shows key/value pairs |
| Daemon list-profiles command | `sysctl list-profiles` in .ci test |
| Config validation accepts profiles | `test/parse/sysctl-profile-dsr.ci` passes |
| Config validation rejects unknown | `test/parse/sysctl-profile-unknown.ci` passes (expect failure) |
| Conflict detection warns | `test/plugin/sysctl-profile-conflict.ci` passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Profile name validation | Names must be non-empty, max 64 chars, alphanumeric + hyphens only |
| Key validation in user profiles | Each key validated via validateKey (length, no path traversal) |
| Value validation in user profiles | Each value validated via known-key registry (type, range) |
| Template injection | `<iface>` substitution must not produce path traversal (interface names validated by iface) |
| Unbounded allocation | Profile count bounded; settings per profile bounded |
| Conflict table size | Fixed at init time from built-in rules; not user-extensible (no OOM vector) |

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
