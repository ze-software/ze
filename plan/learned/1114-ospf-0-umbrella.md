# 1114 — ospf-0-umbrella (CLOSURE record)

Umbrella coordinating OSPFv2 delivery across 13 base children (spec-ospf-1..13)
plus ospfv3 and af-unify. Base OSPFv2 (RFC 2328 + 3101 + 5709/7474 + 9129) is
**delivered as a registered, config-driven plugin** — verified first-hand:
`internal/plugins/ospf/register.go` registers `{Name:"ospf",
RunEngine: runOSPFEngine, YANG: ospfyang.ZeOSPFConfYANG, Dependencies:
[interface, fib-kernel, sysctl]}`, live via the generated
`internal/component/plugin/all/all_ze_ospf.go` blank import.

Producers (from the truth-audit, register wiring re-read at closure):
- Adjacency/NSM: `neighbor/table.go` `setStateLocked`, `nsm.go` `shouldAdj`.
- LSA flooding (§13): `lsdb/flooding.go`; SPF: `spf/spf.go` `Compute` (§16
  Dijkstra), `spf/computer.go` `Run`.
- FIB install: `spf/install.go` `insert` -> `:221` `insertPath` emitting a
  `locrib.Path` at admin distance 110.
- Inter-area/external: `spf/interarea.go`, `spf/external.go`; auth:
  `packet/auth`; raw IP proto 89: `transport/`.
- Tests: 40+ `test/ospf/*.ci` + FRR interop `test/interop/scenarios/ospf-*-frr/`.

All children were closed in commit `3b4a57163` with learned summaries (955-967
base, 968-975 ospfv3, 972 af-unify). Follow-on protocol extensions live in
`plan/spec-ospf-ext-0-umbrella.md` (Status: ready) — NOT part of this base umbrella.

## GOTCHAS
- The umbrella described **delivered** behavior in its prose while keeping
  `Status: design` and an Implementation Audit full of `(pending)` — a stark
  self-contradiction. When children are closed individually, put the umbrella's
  own closure on the last child's checklist; otherwise the umbrella lingers as
  false "design / open" work long after everything it indexes has shipped
  (`spec-closure-check.py --list` flagged it only as a weak umbrella signal).

## Files

None recorded.
