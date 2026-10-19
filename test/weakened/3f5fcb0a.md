| Test | Reason |
|---|---|
| TestOSPFNSSANonCandidateDoesNotWedge | Pinned the RFC3101-3.1-4 wedge-avoidance deviation the owner reversed on 2026-10-18 (#31). The RFC behaviour it contradicted is now asserted by TestRFC3101ElectionRuleDisablesCandidate (a listed higher-Router-ID border router with the Nt-bit clear disables the candidate). |
| rfc3101_nssa_test | The one assertion that left is TestOSPFNSSANonCandidateDoesNotWedge's, removed for the reason on its own row. |
| TestOSPFv6NSSANonCandidateDoesNotWedge | Not removed: renamed TestOSPFv6NSSAHigherBorderRouterDisablesCandidate with the same fixture and the inverted assertion the reversed deviation requires (RFC 3101 Section 3.1: a higher-Router-ID border router disables the OSPFv3 candidate). |
