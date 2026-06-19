# 919 -- mpls-kernel

## Context
Spec `mpls-1-kernel` aimed to add MPLS forwarding to the Linux kernel FIB so BGP
labeled-unicast routes are installed via netlink. A closure audit first concluded
the work was "already fully implemented" (the fibkernel netlink code WAS all
there, with unit + QEMU integration tests). **A deep review then proved that
conclusion wrong: the labels never reached fibkernel.** The default in-process
path routes BGP best-paths through the unified Loc-RIB, whose `Path` had no
`Labels` field, so a labeled-unicast route installed a plain IP route, not an
MPLS push. The code looked done because the unit tests drove fibkernel directly,
bypassing the BGP → Loc-RIB → sysrib production path. F1 added `Labels` to
`locrib.Path` and threaded it through `sysrib.changeToBatch`.

## Decisions
- Kept the existing data-flow: `fibkernel.processEvent` inspects `BestChangeEntry.Labels`
  and routes labeled changes through the rich-route path (`addRichRoute`), which
  imposes the label stack as an `RTA_ENCAP` MPLS encap -- chosen over a separate
  MPLS code path so push/withdraw/relabel reuse the IP-route plumbing.
- MPLS swap/pop (transit) is a distinct `(mpls-fib, entry)` topic handled by
  `handleMPLSEntry` (built for RSVP-TE/LDP), keyed by in-label via `AF_MPLS` routes.
- Left the kernel data-plane verification in the QEMU integration test, not the
  `.ci` harness -- chosen over duplicating kernel assertions in `.ci` so the `.ci`
  runner needs no MPLS-capable kernel.

## Consequences
- BGP labeled-unicast → kernel MPLS works without VPP; `show mpls forwarding`,
  `doctor-mpls-unavailable`, and the `ze_fibkernel_mpls_*` metrics are the operator surface.
- The interface MPLS-enable config lives in the iface YANG (`mpls { enable }` →
  `net.mpls.conf.<iface>.input`), not a standalone `ze-mpls.yang`; the global table
  size is `net.mpls.platform_labels`.

## Gotchas
- **"Unit tests pass" is not "the feature works."** mpls-1/2/3 all looked done because
  their unit tests drove the engine/fibkernel directly, but every one was broken on the
  real production path: BGP-LU labels dropped in the Loc-RIB (F1), LDP/RSVP-TE config
  parsers read the wrong delivered tree shape so the engines never configured, `show`
  commands recursed to a stack overflow, LDP reload was on the wrong SDK callback,
  RSVP-TE link-down matched the wrong interface. The lesson: a protocol feature is not
  done until a test exercises the FULL path from the user/wire entry point, not a
  hand-built internal struct. Closure must require that integration/functional evidence.
- The unified Loc-RIB `Path` is value-typed; adding the `Labels` slice made it
  non-comparable, so `prevBest != newBest` had to become `prevBest.Equal(newBest)` (and
  Equal must compare labels, or a relabel with the same next hop is silently suppressed).
- An MPLS push bypasses sysrib best-path arbitration and shares the FIB with other
  writers, so a first install uses `RouteAdd` (fails `EEXIST` rather than clobbering
  a foreign route for the same prefix); only a genuine relabel of ze's own push uses
  `RouteReplace`. Regression-guarded by `TestMPLSIntegration_PushNoClobberForeignRoute`.
- A `.ci` test that says "verify kernel FIB" but only asserts a BGP wire exchange is
  misleading: the real data-plane check was in the QEMU integration test all along.
- Spec "files to create" named files (`mpls_linux.go`, `ze-mpls.yang`) that shipped
  under different names/locations -- audit by behavior and symbol, not by filename.

## Files
- `internal/plugins/fib/kernel/fibkernel.go`, `mplsentry.go`, `mplsentry_linux.go`, `nexthop_linux.go`
- `internal/component/mpls/show_forwarding.go`; `internal/component/doctor/checks_linux.go`
- `internal/component/iface/config_sysctl.go`, `iface/yang/ze-iface-conf.yang`; `internal/core/sysctl/known_linux.go`
- `test/plugin/mpls-push.ci`, `mpls-withdraw.ci` (comment correction this session)
