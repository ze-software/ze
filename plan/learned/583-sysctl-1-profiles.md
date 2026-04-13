# 583 -- sysctl-1-profiles

## Context

Network operators frequently need specific combinations of kernel tunables set on interfaces
(e.g., arp_announce=2 + arp_ignore=1 for DSR, or rp_filter=1 + log_martians=1 for hardening).
Before profiles, each sysctl had to be set individually in the ipv4/ipv6 containers on each
interface unit. There was no way to express intent ("this interface does DSR") rather than
mechanism ("set these three sysctl knobs"). Profiles replace per-sysctl recipe memorization
with named declarations.

## Decisions

- Profiles over individual YANG leaves: groups co-dependent sysctls that must be set together.
  Rejected flat leaf approach because dsr requires arp_announce AND arp_ignore.
- Core registry (`internal/core/sysctl/`) over plugin-local: leaf package with no plugin deps,
  offline CLI works without daemon. Follows the `internal/core/family/` pattern.
- Profiles emit as default layer via EventBus, same as fib-kernel forwarding. Explicit
  `sysctl { setting ... }` config overrides. Three-layer precedence unchanged.
- Same-value overwrites silent, different-value overwrites warn. Avoids log noise for
  redundant profile overlap (e.g., hardened + multihomed both set arp_filter=1).
- `commit force` skips warnings (not errors) for dangling profile references. Works with
  `commit force confirmed <N>` for auto-rollback. Chose warning-that-blocks over hard error
  because the config may be intentionally staged in two steps.
- `clear-profile-defaults` event per interface before re-emitting on reload. Avoids tracking
  state; just clear and re-emit. Simpler than diffing old vs new profile contents.
- Conflict table is per-sysctl-key, not per-profile. User-defined profiles checked too.
  Deduped per apply cycle to avoid repeated warnings on reload.
- YANG-only boundary enforcement (max-elements, length, pattern). No redundant Go validation.

## Consequences

- Interface units can now declare intent via `sysctl-profile [ dsr hardened ]`.
- User-defined profiles enable transparency: operator sees exactly what gets set.
- `commit force` is a general-purpose mechanism usable beyond profiles.
- `DeregisterProfile` enables clean config reload when user profiles are removed.
- Adding a new built-in profile requires only a `ProfileDef` entry in `builtinProfiles` slice.
- Future: `sysctl-global-profile` for system-wide tunables (tcp_syncookies, etc.) is out of scope.

## Gotchas

- `log_martians` was not registered as a per-interface template key. Had to add it as a
  prerequisite before the hardened profile could work.
- `setDefault` same-value optimization must check config/transient layers first: if config
  overrides the key, a same-value default should still return empty (blocked), not short-circuit.
- `clearProfileDefaults` interface matching must use `.conf.`+name+`.` to avoid VLAN substring
  collision (eth0 matching eth0.100).
- Built-in profile tests need explicit re-registration because other tests call `ResetProfiles`
  in cleanup, wiping the init-time state.
- `commit force` in session mode requires a separate guard: `editor.Save()` errors with
  "not allowed with active session" if session mode is active.
- `OnConfigApply` processes `sec.Removed` for profile deregistration, not just Added/Changed.
  Without this, removed user profiles stay in the registry until daemon restart.

## Files

- `internal/core/sysctl/profiles.go` -- ProfileDef, registry, template substitution, built-in definitions
- `internal/core/sysctl/register_profiles.go` -- init-time registration of built-ins and conflicts
- `internal/core/sysctl/conflicts.go` -- ConflictRule, CheckConflicts
- `internal/core/sysctl/profile_test.go` -- 14 profile/conflict tests
- `internal/core/sysctl/known_linux.go` -- log_martians per-interface template
- `internal/plugins/sysctl/sysctl.go` -- setDefault same-value, clearProfileDefaults, checkProfileConflicts, profile CLI
- `internal/plugins/sysctl/register.go` -- profile parsing, event handlers, daemon commands
- `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` -- profile list in sysctl container
- `internal/component/iface/config.go` -- SysctlProfiles field, applySysctlProfiles
- `internal/component/iface/schema/ze-iface-conf.yang` -- sysctl-profile leaf-list
- `internal/component/cli/model_commands.go` -- commit force, commitSaveAndReload
- `internal/component/cli/model_load.go` -- cmdCommitConfirmed force parameter
- `cmd/ze/sysctl/main.go` -- list-profiles, describe-profile offline CLI
