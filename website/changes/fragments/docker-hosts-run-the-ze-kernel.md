---
spec: docker-hosts-run-the-ze-kernel
topic: Diagnostics
feature: system-readiness
---

## Before

`ze doctor` checked a kernel feature only when the configuration used the
subsystem that needs it, and only for IPsec's XFRM dataplane and MPLS. There
was no way to ask whether a kernel holds everything Ze can use before writing
a configuration for it. A host whose kernel lacks the IPsec state migration
that MOBIKE needs still negotiated tunnels, and the missing feature showed up
later as a tunnel that could not follow an address change. When a security
policy on the host (an AppArmor profile, an SELinux label) stopped a probe from
asking the kernel, the answer read as unknown, with nothing naming the policy.

## After

`ze doctor kernel-capabilities` probes every kernel feature Ze can use,
whatever the configuration says, and prints one row per feature with its
subsystem, the kernel option that provides it, its state and the reason. With
`--json` it answers `{"ready":...,"capabilities":[...]}`. It exits 0 only when
every row is present. The daemon form is `show doctor kernel-capabilities`.

The features checked now include IPsec state migration for MOBIKE, ESP, each
IPsec cipher and integrity transform Ze can install, the IPsec tunnel
interface, WireGuard, L2TP and PPPoE. `ze doctor` reports a missing one for a
configuration that uses it.

A probe that the host's security policy refused reads `denied`, a state of its
own. Its message names the policy, the step it refused and what to grant, and
`ze doctor` reports it as `doctor-kernel-capability-denied`. Ze still starts,
as it does when a probe could not ask.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
