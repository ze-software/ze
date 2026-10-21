# Handover: spec-rfc-verdict-fix-ospf (2026-10-21)

Spec: `plan/pre-release/spec-rfc-verdict-fix-ospf.md` (in-progress). Written so the
work survives a machine move (M9). Delete this file in the spec's closing commit.

## Landed

| Commit | What |
|--------|------|
| `90dce367a3` | RFC5443-2-2 positive unit drives effectiveP2PCost (record, judge re-judged enforced); RFC8362-2-1 positive also covers the E-Inter-Area-Prefix-LSA (0xA023); stale LS type comments corrected |
| `b75f0355c6` | AC-C3 rows RFC3101-3.2-3, RFC7770-2.2-2, RFC7166-4.6-7 (+4.6-4 gap), first verdicts by an independent judge; journal row on the umbrella's false RFC 7166 claim |
| `0de9b98e6c` | AC-C3 row RFC5709-3.2-6 (key storage persists), single-polarity, judged enforced |

## Update 2026-10-21 afternoon

`50044040c5` landed the judged record fixes, the GR helper identity fix and the
extPrefixScope fix. Review round 1 (artifact `tmp/review/rfc-verdict-fix-ospf-*.md`)
found 3 BLOCKER and 1 ISSUE; the working tree now holds their fixes, uncommitted:
26 new split rows (RFC2328, 3101, 5250, 5392, 5709, 8665, 8666) with tests and records,
dated corrections in `rfc/corrections/rfc2328.md`, `rfc3630.md`, `rfc7684.md`, GR flush
tests over broadcast/NBMA/P2MP, and three more D-8 fixes: `sr_install.go::forwarding`
(RFC 8665 §5 / RFC 8666 §6 next-hop flags off the best path), `lsdb/origination.go::handleSelfReceived`
(RFC 2328 §13.4 flush of a withdrawn self LSA), and an AuType 2 per-neighbour replay fix
(RFC 2328 §D.4.3) in progress. Still owed: first verdicts for the new rows and re-judges
of the staled ones by a non-author judge, reseal, commit, review round 2, interop
(the QEMU route `./le test qemu docker-lab lab 'test integration interop' env INTEROP_SCENARIO=<name>`;
the plain Docker route refuses on the Mac's Colima kernel), verify, close.
`internal/plugins/ospf/zz_probe_acc3_test.go` is an agent's throwaway probe: delete it, never commit it.

## In the working tree, uncommitted (2026-10-21, Mac, morning; landed in 50044040c5)

Three independent judges read the 118 AC-C2 sweep records of 2026-10-15. Every
record is a whole-body revert, so many proved only reach. Two author agents then
re-targeted records, strengthened units and fixed one product defect:

- `internal/plugins/ospf/ext.go::extPrefixScope`: NSSA-external (route type 7) now maps
  to area scope (RFC 7684 §2.1, RFC 5250 §3); `TestExtPrefixScopeSelection` fixed; docs/guide/ospf.md.
- Record and tag fixes across rfc2328, rfc3101, rfc3623, rfc5187, rfc5250, rfc5340,
  rfc5392, rfc5443, rfc5838, rfc7684, rfc8362, rfc8665, rfc8666.

These leave audit verdicts STALE (list: `./le rfc check`). Owed before commit:
`./le rfc reseal` (check no foreign audit file changes: compare shasums), a fresh
non-author judge re-judging every stale verdict with `audit-stamp ... mode rejudge`.

## Still owed

1. An author is adding the RFC 2328 §16.4 skip row(s) and fixing RFC6138-x-1.
2. An author is fixing the GR helper defect (RFC 3623 §3.1 check 1: `grInspect` fills
   `ifaceAddr`, gr.go never reads it; journal row in `plan/journal/silent-fall-through.md`),
   adding the OSPFv3 cost-out test (RFC5443-2-1/2-2) and judge items on RFC5709-3-5,
   3.1-2, 3.1-3, RFC7474-2-1, 3-1, RFC7770-2.4-2.
3. Re-judge, reseal, commit; then interop: `ospf-p2p-frr`, `ospf-broadcast-frr`,
   `ospf-opaque-frr`, `ospf-ext-prefix-link-frr`, `ospf-te-frr`, `ospf-ri-frr`,
   `ospf-sr-frr`, `ospf-auth-frr`, `ospf-accept-lifetime-frr`, `ospf-gr-frr`,
   `ospf-stub-nssa-frr`, `ospfv3-nssa-redist-frr`, each with
   `INTEROP_SCENARIO=<name> ./le test integration interop`.
4. `/ze-review` gate, one full `./le verify worktree`, then close per `ze-close`.
