# 1099 -- iface-resolve-0-umbrella (closure)

## Context

Umbrella for decoupling the Ze logical interface name from the OS device name at
runtime. The data model existed since learned 523 (name = logical key, MAC =
physical binding, hidden os-name leaf), but every consumer resolved configured
names straight against the kernel, forcing name == kernel device. The umbrella
decomposed the work into 7 sub-specs; all delivered and closed individually.
This closure records the umbrella-to-delivery mapping and the AC evidence.

## Decisions

- Resolver lives in the iface component (`Resolve`/`Addresses`/`Subscribe`,
  `internal/component/iface/resolve.go,71,80`) over a standalone service or
  per-consumer translation, because iface already owned interface knowledge and
  the Monitor event source.
- 7 planned sub-specs closed as 5 units: 949 (model: os-name + permaddr in show),
  950 (resolver + IS-IS proof consumer, sub-specs 2+3), 951 (mac/match selector),
  952 (consumers: routing, protocols, peripheral -- sub-specs 4, 6, part of 7),
  953 (dispatch translation + dhcp/traffic + the AC-U1 guard -- sub-specs 5 + 7).
- Map-only binding kept: the kernel device is never renamed.

## Consequences

- AC-U1 is a standing verify gate, not a one-shot audit: `ze-iface-resolution-check`
  (`scripts/checks/iface_resolution.go`; target `Makefile:310`, executed by
  `stagesForMode` in `scripts/status/verify_run.go,138` -- the Makefile
  `_ze-verify-impl` list at :287 is documented dead code) rejects
  new direct kernel name resolution outside its allowlist. New backends (the VPP
  wave's tunnel/mirror/LCP files) had to pass it, proving it load-bearing.
- Consumers needing interface identity import iface (accepted infrastructure
  dependency); the allowlist documents every legitimate direct-resolution site.

## Gotchas

- The umbrella outlived its own delivery: all sub-specs closed but the umbrella
  sat `ready` in plan/, counted as open work. Umbrella closure needs to be part
  of the last child's closure checklist.
- Planned AC-U5 test name (`test/iface/iface-show-mapping.ci`) never existed;
  the evidence landed as `show interface name <x> detail` QEMU coverage in 949.
  Chasing the planned name is a dead end; the learned files hold the real map.

## Files

- `internal/component/iface/resolve.go` (resolver API)
- `scripts/checks/iface_resolution.go` (AC-U1 guard)
- `plan/learned/949...953-iface-resolve-*.md` (per-unit closures)
- `test/isis/isis-logical-name.ci` (AC-U2 proof)
