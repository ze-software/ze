# 1185 -- fixit-static-interface-nexthops

## Context

A static route whose next-hop names an interface but carries no address
(`next { interface tun100 { } }`) failed to program on BOTH data planes, for two
unrelated reasons. On linux, `resolveNexthopIndex` -> `iface.Resolve` errored
with "iface: no backend loaded" whenever no iface backend was active, and static
declared no dependency on the iface component so its config apply RACED
`LoadBackend` in the same startup tier. On VPP, `toVPPRoute` rejected
interface-only next-hops outright because no logical-name -> `sw_if_index`
resolver was threaded in. Research overturned three skeleton premises: static
runs in-process (shares `activeBackend`), `iface.Resolve` ALREADY returns the VPP
`sw_if_index` (no `ifacevpp` export needed), and `toFibPath` mis-encoded an
address-less path's proto.

## Decisions

- **One resolver, two data planes.** The VPP leg resolves through the SAME
  `iface.Resolve` the netlink backend uses, not a new resolver or an `ifacevpp`
  export: the VPP iface backend publishes its `sw_if_index` through
  `iface.InterfaceInfo.Index` -> `Binding.Ifindex`. No second resolution path
  (honors `plan/learned/950`), no `ifacevpp` change (removes the only overlap with
  `spec-fixit-vpp-lcp-reachability`).
- **Gate VPP resolution on the active iface backend being vpp (R-7/D-1).** Added
  `iface.ActiveBackendName()` (recorded by `LoadBackend`, cleared by
  `CloseBackend`). `toVPPRoute` errors when it != "vpp" BEFORE resolving, so a
  netlink-backed resolve never programs a kernel ifindex as a VPP `sw_if_index`.
  Fail-closed: a zero/invalid resolved index is rejected, never emitted (index 0
  is VPP local0). Chose a runtime accessor over a config-verify pairing check
  because `LoadBackend` swaps the backend live at runtime.
- **Ordering fix is a declaration, not a workaround (C-1).** Static declares
  `OptionalDependencies: ["interface"]`, which orders it after the iface
  component ONLY when an `interface` stanza is present, and leaves it
  unconstrained otherwise. This is the mechanism's designed purpose
  (`registry.go` optional-dep semantics; precedent `traffic/register.go`).
- **Proto from the route family for address-less paths (C-5).** `toFibPath` now
  takes the route prefix; when the next-hop is unset it derives the proto from
  the route family instead of the zero `netip.Addr` (whose `Is4()` is false and
  defaulted an IPv4 route to `PROTO_IP6`).
- **Validate the reference AND handle runtime failure (D-2 = (a)+(b)).** Widened
  `WantsConfig` to `["static", "interface"]` (Thomas-approved) so config-time
  validation of the interface reference is buildable, kept the actionable runtime
  error (C-2), and added a doctor check
  (`doctor-static-interface-nexthop-no-backend`). None is redundant: an interface
  next-hop may legitimately name an externally-created device, so runtime
  resolution failure is always reachable.
- **Whole-section failure kept and documented (D-3 = (a)).** One unresolvable
  next-hop still fails the whole static section (a deliberate blast-radius
  choice). Per-route isolation is deferred to
  `plan/spec-fixit-static-per-route-isolation.md`; `inject.go` is NOT touched
  here, so the `routesEqual` diff that keeps `WantsConfig`-widening cheap stays
  intact.

## Consequences

- Any future consumer needing a data-plane-correct interface index should reach
  for `iface.Resolve` first, and gate on `iface.ActiveBackendName()` when the
  index will be programmed into VPP -- the two data-plane globals
  (`vpp.GetActiveConnector` vs `iface.activeBackend`) can disagree.
- Static now participates in the reload transaction on every `interface` change
  (the `WantsConfig` widening), but the re-apply is a parse + map diff no-op for
  unchanged routes (`routesEqual` short-circuit), NOT a FIB rewrite.
- The VPP FIB write is NOT proven end-to-end: no `.ci`/QEMU rail exercises a
  VPP-backed static route. C-3/C-5 are proven at the translation seams by unit
  tests (fake `iface.Backend` reporting any index; `toFibPath` proto for a zero
  next-hop). Recorded as a Known Limitation.

## Gotchas

- A zero `netip.Addr` has `Is4() == false` AND `Is6() == false`; code that reads
  proto from a next-hop must handle the address-less case explicitly, or an IPv4
  route silently encodes as IPv6.
- `iface.Resolve` caches per logical name across the process; unit tests loading
  a fake backend must use UNIQUE interface names, or a cached binding from an
  earlier test leaks in.
- A fake `iface.Backend` that embeds the `iface.Backend` interface (nil) must
  override `Close()`, or `CloseBackend()` in test cleanup dispatches to the nil
  embedded interface and panics.
- The VPP static test loads its fake under the exact name "vpp" (the C-4 gate is
  a string compare); nothing else in the static test binary registers that name.

## Known Limitations

- VPP FIB write unproven end-to-end (no rail); interface-only next-hop is proven
  at the translation seams only.
- Whether VPP wants an explicit attached/P2P path TYPE for an address-less path
  is unsettled; C-5 fixes only the proto, which is demonstrably wrong today.
- Functional tests `005`/`006` are `needs-linux` (QEMU) and were NOT run in the
  implementing environment (QEMU forbidden there); they are parked for a QEMU
  validation pass.

## Files

- internal/component/iface/backend.go (activeBackendName + ActiveBackendName accessor; set/clear in LoadBackend/CloseBackend)
- internal/plugins/static/register.go (OptionalDependencies interface; DoctorChecks; WantsConfig widen to static+interface)
- internal/plugins/static/backend_linux.go (C-2 diagnosable no-backend error; back-ref to doctor.go)
- internal/plugins/static/backend_vpp_linux.go (C-3/C-4 resolve via iface.Resolve gated on active vpp backend; reject zero/invalid index)
- internal/plugins/static/vpp/backend.go (C-5 toFibPath proto from route family for address-less paths)
- internal/plugins/static/doctor.go (doctor check for interface-only next-hop with no iface backend)
- internal/core/diagnostic/codes.go (register doctor-static-interface-nexthop-no-backend)
- internal/component/iface/active_backend_name_test.go (accessor load/close coverage)
- internal/plugins/static/backend_vpp_iface_linux_test.go (fake iface backend; AC-4/AC-5/AC-10 + ECMP + zero-index)
- internal/plugins/static/backend_vpp_linux_test.go (rename interface-nexthop rejection to no-backend reason; iface import)
- internal/plugins/static/backend_linux_test.go (AC-2 diagnosable error)
- internal/plugins/static/doctor_test.go (doctor check unit tests)
- internal/plugins/static/register_test.go (AC-8 optional dep; doctor check registration)
- internal/plugins/static/vpp/translate_test.go (toFibPath signature; AC-9 IPv4/IPv6 interface-only proto)
- test/static/005-table-interface.ci (C-6: needs-linux, interface backend stanza, tun100 device)
- test/static/006-interface-nexthop-no-backend.ci (AC-2 functional: diagnosable no-backend error)
- plan/learned/650-static-routes.md (amend: one-resolver-two-dataplanes, ordering, proto, gotchas, blast radius)
