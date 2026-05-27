# 797 -- Interop Gap Coverage

## Context

The interop gap spec compared Ze with VyOS config coverage and rustbgpd interop milestones. The original AC list included both supported and unsupported surfaces. The selected implementation scope was the supported subset only: add coverage where Ze already has exact config and runtime behavior, and leave unsupported ACs explicitly open.

## Decisions

- Use existing BGP runtime behavior over adding compatibility shims. Route reflection, policy import/export, BMP sender, and max-prefix teardown already existed and needed interop evidence.
- Use Ze-owned internal plugins with `use bgp-*` in new scenarios. This avoids Python process plugin overhead for Ze-owned modules while preserving canonical plugin names in config.
- Verify runtime behavior through peer daemons. The interop Ze image does not expose live runtime RIB commands in a useful way, while FRR and BIRD give direct evidence of received routes and attributes.
- Start BMP collectors as interop sidecars before Ze starts. A collector implemented as a Ze process plugin races the internal BMP sender and can miss the PeerUp and RouteMonitoring messages.
- Keep scenario sources on the default `172.30.0.x` prefix, then render per-run copies under `tmp/interop-rendered/` with an allocated `/24`. This preserves readable fixtures while allowing concurrent Docker runs.

## Consequences

- New scenarios 38, 39, 43, 44, and 45 cover route reflection, import/export policy, RPKI origin validation, BMP export, and max-prefix session recovery with real peer daemons.
- New parse coverage files cover supported IXP-style peering, large neighbor sets, global RPKI policy config, and BGP/kernel redistribution config.
- Concurrent interop runs no longer fail on Docker subnet overlap. The runner retries across `172.30.N.0/24`, then `172.31.N.0/24`, then `10.254.N.0/24`, and updates helper constants before importing `check.py`.
- Unsupported ACs stay visible in the spec: confederations, EVPN L3VPN with VXLAN/VRF service config, route-map RPKI matching, OSPF redistribution, GTSM, and several deeper protocol interop scenarios still need real product support or dedicated infrastructure.

## Gotchas

- One-line nested config blocks still require semicolons on leaf values. Missing semicolons produced parse errors such as `expected ';' after ip value`.
- The BGP interop runner accepts one scenario filter argument. Passing several scenario names on one command line only runs the first one.
- Helper constructors must not bind `FRR_IP`, `BIRD_IP`, or `GOBGP_IP` as default arguments. The subnet is chosen at scenario setup time, so defaults need to read globals at object construction.
- FRR source to Ze RR to BIRD client proved route reflection reliably. The inverse BIRD-source path did not deliver the route during scenario development.
- The BMP sidecar always uses the run's `.6` address, and the RPKI sidecar always uses `.7`, alongside Ze `.2`, FRR `.3`, BIRD `.4`, and GoBGP `.5`.
- Structured adj-rib-in delivery must preserve legacy IPv4 NEXT_HOP from path attributes. Without that, direct-bridge received UPDATEs parse NLRI but get dropped as unreplayable because `FamilyOperation.NextHop` is empty.
- BMP interop should not force a BGP session clear just to create message coverage. The initial session already produces Initiation, PeerUp, and RouteMonitoring; forced flaps make the lab nondeterministic.

## Files

- `test/interop/interop.py` -- BMP and RPKI sidecar container support
- `test/interop/scenarios/38-route-reflection-frr/` -- RR interop evidence
- `test/interop/scenarios/39-policy-import-export-frr/` -- policy import/export interop evidence
- `test/interop/scenarios/43-rpki-frr/` -- RPKI RTR and route decision interop evidence
- `test/interop/scenarios/44-bmp-frr/` -- BMP sender interop evidence
- `test/interop/scenarios/45-max-prefix-cease-frr/` -- max-prefix teardown and recovery evidence
- `test/parse/coverage-ixp-peering.ci` -- supported IXP-style parse coverage
- `test/parse/coverage-large-scale.ci` -- supported large peer-set parse coverage
- `test/parse/coverage-rpki.ci` -- supported RPKI parse coverage subset
- `test/parse/coverage-redistribution.ci` -- supported redistribution parse coverage subset
- `docs/functional-tests.md` -- documents parse coverage and BGP interop sidecar behavior
