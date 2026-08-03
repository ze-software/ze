# 1113 — fib-depth-4-srv6 (CLOSURE): SRv6 FIB backend programming

Both FIB backends consume the `SRv6SID` field on a best-change entry and program
SRv6 steering/encap:
- **Kernel (SEG6):** `buildSEG6Encap` (`internal/plugins/fib/kernel/nexthop_linux.go`),
  reached from `buildRichRoute` and gated by `SRv6SID.Is6()`.
- **VPP (SR steer):** `fibVPP.processSRv6Change` (`internal/plugins/fib/vpp/srv6.go`)
  dispatches on the route verb — install/replace → `govppSRv6Backend.addSRv6Steer`
  (`srv6.go`, a real `sr.SrSteeringAddDel` with the SID as BSID), remove →
  `delSRv6Steer` (`srv6.go`) — and tracks installed prefixes in `f.srv6Installed`
  so removals are idempotent. A change with no SID is a no-op (the verb switch,
  `srv6.go`).

Tests: `TestKernelSRv6Encap` (`fibkernel_test.go`), `TestSRv6SteerAdd`/
`TestSRv6SteerWithdraw`/`TestSRv6ZeroSIDSkipped` (`apply_test.go/869/895`),
`test/plugin/fib-srv6-kernel.ci`, interop `test/interop/scenarios/35-srv6-frr/`.

## GOTCHAS
- **The `blocked on bgp-nlri-srv6` premise was wrong.** SRv6 SIDs reach the FIB via
  the BGP **Prefix-SID attribute** extraction (`internal/component/bgp/plugins/rib/pool/srv6sid.go`
  → `RIBManager.lookupSRv6SIDForBest` sets `entry.SRv6SID`), NOT a dedicated SRv6 NLRI
  family. This backend's scope only ever needed the `SRv6SID` field to be populated,
  which the Prefix-SID path already does. A `blocked` status can outlive its blocker.
- The spec's "Current Behavior / Files to Create" claimed "no seg6 encap code yet;
  create srv6.go" — stale at closure: both the kernel SEG6 path and `srv6.go` already
  existed and were tested. Re-audit the input→FIB chain before trusting a spec's
  "current behavior" section written months earlier.

## Files

None recorded.
