# Ruling: Scope Docker interop kernel checks to suite and scenario capabilities

- **Date:** 2026-11-23
- **Owner:** Thomas Mangin
- **Context:** `INTEROP_SCENARIO=bgp-llgr-per-family-frr ./le test integration interop` was refused on a `7.3.0-ze` host kernel because `interoplab`'s pre-flight kernel check probed every enrolled `kernelcap` subsystem unconditionally and failed on newly added SRv6 probes (`srv6-forward-ttl`, `srv6-forward-frag`) that BGP does not use.

## Decision

> "the latest feature we added to our kernel are not required for BGP so the current kernel (and even a stock linux kernel) is capable of running this code."
> "only the kernel capabilities a given suite/scenario"

## What this requires

1. `interoplab.Suite` and `interoplab.ScenarioPlan` scope the Docker host kernel pre-flight check to the kernel capabilities that the suite and its selected scenarios actually require (`Suite.KernelCapabilities` unioned with each selected `ScenarioPlan.KernelCapabilities`).
2. `ze doctor [--json] kernel-capabilities` accepts repeatable `subsystem <selector>` filters (exact subsystem name or trailing-wildcard prefix such as `ipsec-esp-*`), probing only the matched capabilities and refusing unknown selectors.
3. A suite/scenario requiring no special kernel capabilities (e.g. pure BGP, RADIUS admin, VPP userspace proofs) skips the kernel-capabilities probe and runs on a stock Linux kernel or an older `ze` kernel.
4. A suite/scenario that requires a kernel capability still fails fast when that capability is absent, unknown, or denied (preserving D-4 for every feature the run actually needs).
5. Full-host checks (`./le setup docker kernel check`, `./le verify evidence release-candidate`, and any suite leaving `Suite.KernelCapabilities` unset) continue to probe every enrolled capability (fail-closed default).
