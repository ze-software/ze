# Handover: spec-bgp-prefix-sid-rfc-defects, phase 4 interop

Written 2026-10-20 by the closure agent `prefixsid` (session d8e7dbbf), updated
the same day after the rerun. The spec is
`plan/immediate/spec-bgp-prefix-sid-rfc-defects.md`; its "Phase 4 run" and
"Phase 4 rerun, 2026-10-20" tables carry the evidence. The agent claim is
released: the spec waits on the two owner decisions below.

## Open owner decisions

| # | Decision | Options | What waits on it |
|---|----------|---------|------------------|
| 1 | `bgp-srv6-invalid-sid-ineligible-frr` cannot go green on this host: the Docker kernel has no VRF ("Unknown device type"; no VRF line in `/proc/config.gz`) | (a) VRF-capable Docker kernel (the owner's reboot); (b) QEMU docker-lab guest; (c) redesign so FRR originates the SRv6 VPN route without a VRF | AC-4a interop, and the D3/D4 forced-red walk (break the RFC 9252 3.2.1 sum check in `parseSIDStructure`, `rib/pool/srv6sid.go`, so its `return false, 0, 0` answers valid) |
| 2 | AC-1b's interop proof column is vacuous: FRR prints nothing for an Originator SRGB, so the SRGB-only check passed with the RFC 8669 4.1 check reverted | (a) drop the interop claim, keep `.ci` case (d) as the proof (scope, the owner's); (b) add a packet-capture assertion | AC-1b's Goal Validation row |

## State

| Item | State |
|------|-------|
| Phases 1-3 code, units, `.ci` | landed (`8c1c4013f2`, `8ddd0ba446`, `7d37dcd562`, `4333cbb3b9`, `ea9bb0a1de`) |
| adj-rib-in key defect, IPv4-only lab | fixed by `a3ed24c523` and `8463adb050` |
| `bgp-prefix-sid-malformed-frr` | GREEN (ze image `sha256:5047411012ad`) after `3ea95435e8`: control (c) rebuilt from (b1), positive control (e) 10.0.5.0/24, constant `Label Index`. Reds recorded in the spec: D1 reverted (assertion 1, FRR resets), D2 reverted (assertion 9). The AC-1b check does not discriminate (decision 2) |
| `bgp-srv6-invalid-sid-ineligible-frr` | session up, but blocked on the host kernel (decision 1). `3ea95435e8` adds the VRF-creating opExec and an FRR-side route wait |
| Journal | `plan/journal/requirement-applied-outside-the-case-it-names.md` (FRR resets on a non-7 Label-Index off labeled unicast), `plan/journal/green-that-could-not-have-been-red.md` (`bgp-srv6-frr` asserts only the session) |
| A-1, A-3, A-4, A-5 | confirmed 2026-10-20 (A-1 and A-4 with a caveat and a correction, see the spec's Assumptions table) |
| A-6 | the `.ci` took the fallback (three wire prefixes, presence and absence), per `4333cbb3b9` |
| Closure sections, review gate | not started |

## Next steps

1. The owner answers decisions 1 and 2.
2. Under decision 1, run `bgp-srv6-invalid-sid-ineligible-frr` green, then the
   D3/D4 red walk (break, rebuild by running the suite, red, restore exactly,
   green). Use `./le --name <agent> test integration interop`; `./le` alone
   runs the prebuilt `bin/le`.
3. Under decision 2, either edit AC-1b's proof column or add the capture check.
4. Append `plan/TEMPLATE-CLOSURE.md`; run the review gate; commit A.
