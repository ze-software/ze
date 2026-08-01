# 746 -- Firewall Global Options (Sysctl Convenience)

## Context

Ze had no user-friendly way to set common network security defaults. Users had
to know raw sysctl keys for settings like ICMP echo control, SYN cookies, or
reverse path filtering. The goal was a YANG-modeled `global-options` container
under `firewall {}` that maps keyword toggles (all-ping, syn-cookies,
source-validation, etc.) to sysctl writes at config apply time.

## Decisions

- Chose EventBus `(sysctl, default)` emission over direct sysctl plugin calls,
  reusing the three-layer priority model (config > transient > default) so explicit
  `sysctl { setting { ... } }` automatically wins without special priority logic.
  Same pattern as fib-kernel forwarding defaults and iface profile defaults.
- Added `RawData` field to `firewallConfig` to carry JSON from verify through apply,
  so `ExtractGlobalOptions()` runs at both stages without re-parsing config sections.
- Used static mapping table (`globalOptionDefs`) over a registry pattern, because
  the keyword set is closed and unlikely to grow.
- `source-validation` is a three-way enum (disable/strict/loose) rather than
  enable/disable, matching the three modes of `rp_filter`.

## Consequences

- Users can set common network security defaults with keyword toggles instead
  of raw sysctl keys.
- The sysctl plugin's default layer receives firewall-sourced defaults, clearable
  via `clear-source-defaults` on reload so removed keywords revert.
- Adding new keywords requires only a table entry in `globalOptionDefs` and a
  YANG leaf, no wiring changes.

## Gotchas

- `all-ping` and `broadcast-ping` have inverted semantics: "enable" sets the
  `*_ignore_*` sysctl to 0. The YANG description must clarify this or users
  will get the opposite effect.
- The spec originally planned to use the sysctl config layer for global-options.
  This would have made global-options override explicit sysctl settings, violating
  AC-6. Corrected to use the default layer.

## Files

- `internal/component/firewall/yang/ze-firewall-conf.yang` (global-options container)
- `internal/component/firewall/config.go` (ExtractGlobalOptions, globalOptionDefs)
- `internal/component/firewall/engine.go` (emitGlobalOptionsSysctlDefaults)
- `internal/component/firewall/config_test.go` (8 unit tests)
- `internal/component/sysctl/events/events.go` (EventClearSourceDefaults)
- `test/plugin/firewall-global-options.ci` (functional test)
- `docs/guide/firewall.md` (Global Options section)
