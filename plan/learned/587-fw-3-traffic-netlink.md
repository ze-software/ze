# 587 -- fw-3 Traffic Control Netlink Backend

## Context

Ze needed a tc backend plugin to program Linux traffic control (qdiscs, classes, filters) from config. The traffic component (fw-1 data model, fw-4 config parser) produces `map[string]InterfaceQoS`. The trafficnetlink plugin translates these to vishvananda/netlink tc API calls.

## Decisions

- Used already-vendored `vishvananda/netlink` tc API, over raw rtnetlink, because it provides typed qdisc/class/filter structs and handles netlink encoding.
- Apply replaces the root qdisc per interface then rebuilds all classes and filters, over incremental diff, because tc class/filter handles are positional and full rebuild is simpler and correct.
- HTB handle scheme: root qdisc is 1:0, classes are 1:1, 1:2, etc. (minor = class index + 1), over user-specified handles, because the config uses names and the handle assignment is an implementation detail.
- Registration as "tc" backend, matching the kernel subsystem name, over "netlink" (too generic) or "trafficnetlink" (too long for config).

## Consequences

- The traffic component can now be wired end-to-end: config parsed, InterfaceQoS built, Apply called, qdiscs/classes/filters programmed.
- All 10 qdisc types from the vendored netlink library are translatable.
- Linux-only `_linux.go` files have potential type mismatches (reported by cross-compilation LSP) that need Linux CI to validate and fix.
- ListQdiscs returns the root qdisc type but not the full class/filter tree. Full read-back is a follow-up.

## Gotchas

- vishvananda/netlink `Htb.Defcls` field type may differ between versions (uint32 vs *uint32). The vendored v1.3.1 needs verification on Linux.
- tc handle math uses `(major << 16) | minor`. Getting this wrong produces silent misrouting of traffic. The `makeHandle` helper centralizes this.
- The `FwFilter` type uses `Mask` for mark matching. Without setting it, all marks would match.

## Files

- `internal/plugins/trafficnetlink/trafficnetlink.go` -- Package doc
- `internal/plugins/trafficnetlink/register.go` -- RegisterBackend("tc")
- `internal/plugins/trafficnetlink/backend_linux.go` -- Apply, ListQdiscs
- `internal/plugins/trafficnetlink/backend_other.go` -- Non-Linux stub
- `internal/plugins/trafficnetlink/translate_linux.go` -- Qdisc/class/filter translation
- `internal/component/plugin/all/all.go` -- Updated by make generate
