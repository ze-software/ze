# 716 -- iface-2-urpf

## Context

Ze exposed reverse path filtering as a raw integer (`rp-filter 0|1|2`) under
the IPv4 container only. Operators had to remember the mapping, and there was
no IPv6 equivalent. The goal was to replace the opaque integer with a named
`rpf-check strict|loose|disable` enumeration in both address families, with
backward-compatible parsing of the legacy syntax.

## Decisions

- Chose YANG `enumeration` type over keeping `uint8 range 0..2` because the
  enum is self-documenting and YANG validates values at schema level before
  the Go parser runs.
- Chose `rpf-check` name (Junos-inspired) over `source-validation` (VyOS)
  or `verify unicast` (Cisco) because it is shorter and matches the Junos
  naming pattern Ze already follows for similar leaves.
- Chose explicit `disable` value over relying on absence because absence
  means "not configured" (nil, leave OS default), while `disable` means
  "explicitly set to 0". This three-state (nil/disable/strict/loose) matches
  the existing pointer-field pattern in `ipv4Settings`.
- Deferred VPP uRPF enforcement over implementing it now because no GoVPP
  uRPF bindings exist in the codebase. The only reference is a FIB flag name
  string `"urpf-exempt"` in `internal/plugins/iface/vpp/fib.go`.
- Removed AC-7 (show interface displays rpf-check) over implementing it
  because `cmd/ze/iface/show.go` reads live OS state via netlink, not parsed
  config or sysctl values. Adding config state display is a separate feature.
- Unexported `rpfMode` type over exporting it because it is only used within
  the `iface` package.

## Consequences

- Legacy `rp-filter N` configs continue to parse but emit a deprecation
  warning. The YANG schema no longer has the `rp-filter` leaf, so the
  config editor will not offer it in completion.
- IPv6 `rpf-check` is accepted in config but only logs a warning on Linux.
  When VPP uRPF bindings are added, the `applySysctl` IPv6 branch needs
  to call GoVPP instead of just warning.
- The `hardened` sysctl profile sets `rp_filter=1`. Current apply order
  (`applySysctl` then `applySysctlProfiles`) means profiles override
  explicit config. This is a pre-existing issue not specific to rpf-check
  but worth fixing when profiles are next touched.

## Gotchas

- The spec originally claimed `show interface` would display rpf-check mode.
  Reading `cmd/ze/iface/show.go` showed it only reads OS state (netlink),
  not config. The AC was removed during spec review. Lesson: verify the
  existing show command's data source before promising display of config
  state.
- The spec's Files to Modify section omitted `config_sysctl.go` (the apply
  path) and named the parser function as `parseIPv4Sysctl` (does not exist;
  actual name is `parseIPv4Settings`). Lesson: grep for the actual field name
  before trusting spec claims about function names.
- The `applySysctl` / `applySysctlProfiles` ordering means "last write wins"
  on the EventBus. This was discovered during spec review, not during
  implementation. The spec now documents it as an investigation item.

## Files

- `internal/component/iface/config.go` -- rpfMode type, parser changes
- `internal/component/iface/config_sysctl.go` -- apply path, IPv6 warning
- `internal/component/iface/schema/ze-iface-conf.yang` -- rpf-check enum
- `internal/component/iface/config_test.go` -- 6 unit tests
- `test/parse/iface-rpf-check.ci` -- functional parse test
- `docs/guide/configuration.md` -- rpf-check section
- `docs/features.md` -- feature description update
