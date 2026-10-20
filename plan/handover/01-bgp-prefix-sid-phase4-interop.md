# Handover: spec-bgp-prefix-sid-rfc-defects, phase 4 interop

Written 2026-10-20 by the closure agent `prefixsid` (session d8e7dbbf). The spec
is `plan/immediate/spec-bgp-prefix-sid-rfc-defects.md`; its "Phase 4 run,
2026-10-20" table carries the evidence.

## State

| Item | State |
|------|-------|
| Phases 1-3 code, units, `.ci` | landed (`8c1c4013f2`, `8ddd0ba446`, `7d37dcd562`, `4333cbb3b9`, `ea9bb0a1de`); scoped package tests green on Linux at `eeb0e0e9f7` (attribute, message, rib/..., nlrisplit, reactor `-run 'RFC8669\|RFC9252\|PrefixSID\|LabelIndex'`) |
| `bgp-prefix-sid-malformed-frr` | RED at HEAD: adj-rib-in keys labeled and VPN routes on label octets, so the bgp-rs peer-up replay loses routes. Product defect outside the spec, journal rows of 2026-10-04 and 2026-10-20 in `plan/journal/declared-format-contradicts-payload.md` |
| `bgp-srv6-invalid-sid-ineligible-frr` | RED at HEAD: the lab network is IPv4-only (`needsIPv6`, `internal/le/interoplab/bgp/prepare.go`), and FRR refuses an IPv6 VPN session with no local IPv6 address (Cease/Unspecific on OPEN). Scenario defect, the spec's own |
| Forced-red walks | not started: each needs its green first |
| RFC9252-3.4-1 verdict | no longer stale: re-judged 2026-10-20 in `rfc/audit/rfc9252.json` (commit `ead4951e1f`); `./le rfc check` reports no RFC 8669 or RFC 9252 violation (its 3 violations are rfc2328 and rfc1997) |
| A-5 | confirmed (injector sends the five frames in order on one connection) |
| A-4 | still unvalidated; see the spec row for the possible vacuity of the "label-index" absence checks |
| A-1, A-3 | still unvalidated. Lead: `injectWireRoute` (`plugins/rib/rib_inject.go`) runs `ValidateUpdateRFC7606` but acts only on treat-as-withdraw and above, so an attribute-discard verdict is not applied there, and it does not pass `publishBase`, so the D6 discard does not run on that path |
| A-6 | the `.ci` took the fallback (three wire prefixes, presence and absence), per `4333cbb3b9` |
| Closure sections, review gate | not started |

## Next steps

1. Owner decision on the adj-rib-in defect: fix it first (a separate fix, its
   own commit), or change the scenario so FRR is established before the
   injector sends and the live forward rail carries the routes.
2. Make `bgp-srv6-invalid-sid-ineligible-frr` dual-stack (a scenario file
   naming a `fd00:1e:0::` address), then run it green.
3. For each scenario: green, revert the producer (`attribute.ClassifyPrefixSID`
   path for the first, `pool/srv6sid.go` and `isSRv6Ineligible` for the
   second), rebuild by running the suite, confirm red, restore exactly, green.
   Use `./le --name <agent> test integration interop`; `./le` alone runs the
   prebuilt `bin/le`.
4. Resolve A-1, A-3, A-4; append `plan/TEMPLATE-CLOSURE.md`; run the review
   gate; commit A.
