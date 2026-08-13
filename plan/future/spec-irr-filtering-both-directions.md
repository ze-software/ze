# Spec: irr-filtering-both-directions

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Let IRR filtering run on export as well as import.**

Owner request, 2026-08-13: allow filtering for both directions.

`filter_irr` is import-only today. Its own YANG says so: *"as import filters"*
(`internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang`), and every code path
names an import filter (`filter_irr.go`).

So Ze validates what a peer SENDS against an IRR record. It validates nothing about what Ze
ADVERTISES.

## The syntax needs no change, and that is measured

Owner constraint, 2026-08-13: the configuration that lands today MUST keep working when
this lands.

**It does, and this spec adds no new configuration at all.**
`internal/component/bgp/yang/ze-bgp-conf.yang` already declares:

```
container filter {
    leaf-list import { ... }
    leaf-list export { ... }
}
```

That chain accepts ANY registered filter in either direction. So
`export [ bgp-filter-irr:65001 ]` parses today. The work is entirely in the plugin.

**Establish first what Ze does with that configuration NOW.** A chain entry the plugin
ignores is worse than one it refuses: the operator reads a filter in their configuration
and gets none. If Ze accepts the entry and silently drops it, that is a defect, and this
spec inherits it (`ai/rules/completion.md`). If Ze refuses it at parse, the refusal message
becomes wrong the day this lands.

## The design question: which AS-SET

Import and export ask about different parties.

| Direction | The question | The AS-SET |
|-----------|--------------|------------|
| Import | May this peer advertise this prefix to me? | **Theirs** |
| Export | May I advertise this prefix to this peer? | **Ours**, or what the peer's policy accepts |

`session.irr.as-set` names one AS-SET per peer today, and it means "theirs". **A second
leaf is needed for the export side.** It arrives as a SIBLING inside the existing `irr`
container, so every configuration written today keeps parsing:

```
session { irr { as-set AS-THEM; enable true; } }
```

**Do NOT rename `as-set` to `import-as-set`.** That edit breaks every deployed
configuration, and a sibling leaf reaches the same place for nothing.

Name the new leaf so the pair reads correctly to an operator who knows only the old one.

## What must not be lost

**Fail closed** (`ai/rules/evidence.md`). An export filter whose AS-SET never resolved MUST
advertise nothing, not everything. The blast radius is larger than import: a resolver that
fails open on export leaks routes to a peer, and the peer sees a working session.

**State the startup order.** `filter_irr.go` records that a peer with no IRR import never
gates startup. An export filter that gates startup changes that property, and an operator
whose IRR server is unreachable then cannot boot a router.

## Related

`plan/future/spec-blackhole-authorization-from-irr.md` wants the same resolved data for a
different consumer. Both want IRR data reachable outside the import chain. **Read that
spec's design table before choosing a shape here**, and pick the same one if it fits: two
mechanisms for one question is what `ai/rules/no-layering.md` refuses.
