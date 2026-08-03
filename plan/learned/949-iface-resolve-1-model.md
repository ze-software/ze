# 949 -- iface-resolve-1-model

## Context

First implementable unit of the interface logical-name resolution effort
(`spec-iface-resolve-0-umbrella`). Ze interface consumers resolve a configured
name straight against the kernel, forcing the config `name` to equal the OS
device name, and Ze reads only the operational MAC -- never the permanent
(factory) MAC -- so a MAC override erases the only stable physical-device
identity. This sub-spec lays the read-only foundation: expose the permanent MAC
and make the OS device name a first-class, visible field, without yet building
the resolver (sub-spec 2) or migrating any consumer.

## Decisions

- Added two `omitempty` fields to `iface.InterfaceInfo` (`OsName`, `PermanentMAC`) over a new struct or a separate API, because the type is the existing cross-boundary value type and `omitempty` makes the addition non-breaking for every show/web/rate consumer.
- `PermanentMAC` is read from `netlink.LinkAttrs.PermHWAddr` (vishvananda/netlink v1.3.1, `link.go`) over a raw `IFLA_PERM_ADDRESS` netlink parse or `ETHTOOL_GPERMADDR` ioctl, because the wrapper already exposes the field; no extra syscall path is needed.
- Populated both in `linkToInfo` (the single build point shared by `ListInterfaces` and `GetInterface`) over per-call-site reads, so there is one permaddr read path.
- **Narrowed scope mid-implementation (user-approved):** os-name as a *resolution selector* (un-hiding the config `os-name` leaf, default-to-name, binding) was moved to sub-spec 2. The audit proved the config `os-name` leaf is **dormant** -- written by `ze init` (523) but read by no runtime code -- so promoting it before a consumer exists would be a speculative, inert config knob. Sub-spec 1 only makes os-name *visible* in `show interface` (the `OsName` field = kernel device name) and reads the permanent MAC.
- `show interface name <x> detail` needed no formatter change: it returns the whole `InterfaceInfo`, so the new fields surface automatically.

## Consequences

- `show interface name <x> detail` now exposes `os-name` (always) and `permanent-mac-address` (real NICs only). Today `OsName == Name`; once the resolver makes `Name` a logical handle, `OsName` keeps the kernel device and show displays both sides of the mapping.
- Virtual/created kinds (veth/bridge/tunnel/dummy) report empty `PermanentMAC` (no factory address) -- rendered as a missing key via `omitempty`, not an error.
- A `unique "mac/address"` invariant test now guards the MAC physical binding against silent linter reversion (see Gotchas).

## Gotchas

- The MAC-binding model from 523 (`unique` + intended-but-absent `ze:required`) is fragile: 523 documents a linter hook silently reverting YANG `unique`/`ze:required` between sessions. `TestMacBindingUniqueRetained` asserts the `unique "mac/address"` count stays >= 3 and that mac is NOT `ze:required`, so a future revert fails loudly. The *correct* state is unique + optional (a required MAC would break every `interface eth0 {}` config).
- Linux-only code: the netlink unit tests (`//go:build linux`) cannot run on a darwin host. Run them via `make ze-linux-test ZE_LINUX_TEST_PACKAGES=./internal/plugins/iface/netlink/` (Docker) or QEMU; the host can only compile-check (`GOOS=linux go vet`).
- **Two distinct `show interface` surfaces, and a struct field reaches only one of them automatically.** The RPC/interactive path (`show interface name <x> detail`, `internal/component/iface/cmd/show_interface.go`) returns the whole `InterfaceInfo` and serializes new fields for free. The standalone binary command (`ze interface show <name>`, `internal/component/iface/cli/show.go` `showOne`) hand-prints a fixed field list and `--json` marshals the struct -- so a new field is invisible in the *text* output until `showOne` is edited. Adding the struct field was NOT enough; only the QEMU functional test (`ze interface show lo`) caught that os-name never rendered. Compile checks and unit tests on `linkToInfo` all passed while the user-visible output was still missing the field -- run the real command end-to-end.
- The `ze interface show` text formatter fought two hooks: the no-sprintf-alloc pretool hook blocks new `fmt.Printf("...%s...")`, and `fmt.Fprintf(os.Stdout, ...)` then trips errcheck. Resolution: build the detail block with a `textbuf.Buffer` in a pure `formatInterfaceDetail(info) string` helper and `fmt.Fprint(os.Stdout, ...)` once -- which also made the render unit-testable without netlink (the QEMU test on loopback can't cover the permaddr line; the unit test can).
- **Growing a cross-boundary value type past gocritic `rangeValCopy: sizeThreshold` breaks unchanged consumers repo-wide.** `InterfaceInfo` grew 152→184 bytes (two string fields), crossing the 160 threshold in `.golangci.yml`. Every `for _, x := range []InterfaceInfo` loop ANYWHERE then fails gocritic -- adding even one string field (152→168) crosses it. `make ze-lint-changed` only lints changed packages, so it passed while 5 violations sat in the unchanged `web/` package; `/ze-review`'s repo-wide sweep caught them. Fix: range by index (`for i := range xs { ... xs[i] ... }`) or `cfg := &xs[i]`. Lesson: when a shared value type grows, run a full lint, not just changed-package lint.
- This work was done while a concurrent `tiers` refactor relocated whole engines (`isis`, `flowexport`, `ldp`) from `internal/component/` to `internal/plugins/` and rewrote history. Stale LSP `BrokenImport` diagnostics appeared repeatedly; each was a cache artifact, not a real break -- always confirm with `go vet ./internal/component/plugin/all/` before reacting.

## Files

- `internal/component/iface/iface.go` -- `InterfaceInfo.OsName`, `InterfaceInfo.PermanentMAC`
- `internal/plugins/iface/netlink/show_linux.go` -- `linkToInfo` populates both from `Attrs()`
- `internal/plugins/iface/netlink/show_linux_test.go` -- `TestLinkToInfoPermanentMAC`, `TestLinkToInfoNoPermanentMAC`
- `internal/component/iface/cli/show.go` -- `formatInterfaceDetail` renders `OS Name:` / `Perm MAC:` lines (textbuf)
- `internal/component/iface/cli/show_test.go` -- `TestFormatInterfaceDetail` / `…NoPermMAC`
- `internal/component/iface/yang/mac_binding_test.go` -- `TestMacBindingUniqueRetained`
- `internal/component/iface/health.go`, `internal/component/web/page_interfaces.go`, `page_ip_addresses.go`, `page_traffic.go` -- range `[]InterfaceInfo` by index (rangeValCopy cascade from the struct growth)
- `docs/guide/command-reference.md` -- os-name / permanent-MAC note (source-anchored)
- `test/parse/cli-show-interface-osname.ci` -- show-interface os-name visibility (QEMU-verified, PASS)
