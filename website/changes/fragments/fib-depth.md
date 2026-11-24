---
spec: fib-depth
topic: Kernel
feature: route-installation
---

## Before

- Running the VPP FIB writer without an initialized persistent daemon store
  blocked plain IP and MPLS route installation because SRv6 ownership restore
  ran unconditionally before programming any route.
- Labeled ECMP routes on VPP dropped their multipath members and programmed
  only a single labeled path.
- MPLS label swap and pop entries from LDP, RSVP-TE, OSPF-SR, and BGP EPE
  reached only the Linux kernel FIB writer, and `show mpls forwarding` read
  only the kernel AF_MPLS table, returning empty output on VPP deployments.
- In the Linux kernel FIB writer, a route refused transiently by the kernel was
  never retried while `fib-sync-failure` stayed active; moving a prefix between
  routing tables left the previous table's route behind; a failed superseded
  route delete left the newly installed route unrecorded; startup sweep counted
  already-absent (`ESRCH`) deletes as removed orphans; and a non-rich backend
  silently ignored nonzero routing table IDs.
- Enabling `connected { }` with BGP without an explicit `interface { }` or
  `static { }` block did not pull the interface monitor, leaving connected
  subnets out of the system RIB and direct BGP next hops unresolved.

## After

- The VPP FIB writer now programs and flushes plain IP routes and MPLS pushes
  even when no persistent daemon store is configured, refusing only SRv6
  service routes and reporting them through `ze show doctor`
  (`fib-vpp-srv6`, code `doctor-fib-vpp-srv6-refused`).
- Labeled and unlabeled ECMP routes on VPP program all multipath members with
  per-path uniform-mode MPLS label stacks under FIB source `ze-fib-vpp`, and
  restarted writers sweep unclaimed stale IP and MPLS routes automatically.
- MPLS swap and pop entries, interface `mpls { enable true; }`, and
  `ze show mpls forwarding` now follow the selected data plane (`kernel` or
  `vpp`), with LDP, RSVP-TE, OSPF, and IS-IS automatically loading the writer
  for the configured data plane.
- The Linux kernel FIB writer retries failed route installs and replacements
  with bounded exponential backoff (up to 10 attempts, 500ms to 30s) and on
  subsequent batch progress, clearing `fib-sync-failure` once a route installs
  or withdraws cleanly; removes old-table routes on table moves; preserves
  ownership and cleans up superseded routes after partial applies; counts only
  actual kernel deletes during startup sweep; and rejects nonzero table IDs on
  backends that only support the main table.
- Enabling `connected { }` now automatically starts the interface plugin as an
  optional dependency ahead of `connected`, so interface addresses populate the
  system RIB without requiring an explicit `interface { }` block.

<!-- source: internal/plugins/fib/vpp/fibvpp.go -- processEvent, isSinglePathMPLS -->
<!-- source: internal/plugins/fib/vpp/backend.go -- addRichRouteInTable, applyPathLabels -->
<!-- source: internal/plugins/fib/vpp/doctor_srv6.go -- srv6CheckDecl, srv6DoctorFindings -->
<!-- source: internal/plugins/fib/vpp/routesource.go -- openRouteSource, sweepStaleRoutes -->
<!-- source: internal/plugins/fib/vpp/mplsentry.go -- handleMPLSEntry -->
<!-- source: internal/plugins/fib/vpp/mplsforwarding.go -- readMPLSForwarding -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- programRouteLocked, ownsSlotLocked, sweepPrefixLocked -->
<!-- source: internal/plugins/fib/kernel/pending_retry.go -- runRetryLoop, retryDuePending, retryPendingAfterProgressLocked -->
<!-- source: internal/plugins/connected/register.go -- init -->

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
