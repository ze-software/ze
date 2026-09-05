# Spec: router-advertisement-options-out-of-scope

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | iface |
| Depends | - (was `spec-router-advertisement`, delivered and closed 2026-09-05) |
| Phase | 0/1 |
| Handoff | - |
| Updated | 2026-09-05 |

## Task

Ze sends Router Advertisements. `BuildRA` (`internal/core/ndp/ra.go`) writes the
Source Link-Layer Address option, the Prefix Information options, and RDNSS.
It writes nothing else, because the delivering spec scoped the rest out on
2026-07-10 and shipped what an operator needs to give a link SLAAC and a
resolver.

This spec holds what that decision left out. It is the home the deferral row
points at, and it is the only live record of the list: `spec-router-advertisement`
closed on 2026-09-05 and its Known Limitations section went with it.

Nothing here is a defect. Each item is an RA option or an RA behavior a peer can
do without, so a host on a Ze link autoconfigures today.

## What is deferred

**The two the deferral row tracks.**

- **DNSSL** (RFC 8106 Section 5.2). Ze advertises resolvers and not the search
  list beside them. `raParseRDNSS` (`internal/component/iface/config_ra.go`)
  reads the `rdnss` container, and there is no `dnssl` container to read.
- **The MTU option** (RFC 4861 Section 4.6.4). A link whose MTU the operator
  wants every host to agree on has to carry it another way.

**The rest of the same decision, recorded so it is not rediscovered.**

- RFC 4191 route information and router preference.
- Unicast RAs to an individual host.
- The VPP backend. The YANG container is `ze:backend "netlink"`, and a VPP path
  would drive VPP's own ip6-ra feature, which is a different design.
- Deriving the advertised prefixes from the unit's configured addresses.
- RA state in `show interface`.
- The RFC 6275 mobile-IP fields.

## Why it was not built with the sender

The sender's goal was a host that autoconfigures, proven on the wire. Each item
above adds a config surface and an encoder branch without moving that goal, and
two of them, the VPP backend and the derived prefixes, are designs rather than
options.

## Open questions a design phase owes an answer to

- **Whether DNSSL and the MTU option travel together.** Both are one more
  option in the same buffer, so one encoder pass and one config container could
  carry both. They answer different operator questions, so a reader may want
  them separately in the schema even if one change lands them.
- **What the MTU option's value is.** The link MTU Ze already knows, the value
  the operator writes, or a refusal when the two disagree. A wrong MTU on a link
  is worse than no MTU option, so the fail-closed answer needs stating.
- **Whether the encoder stays a fixed set of writers.** `BuildRA` calls one
  writer per option today and `RALen` sizes them the same way. A fourth and a
  fifth option is where that shape either holds or asks to become a list.
- **Whether route information needs Ze to have routes to advertise.** RFC 4191
  is the largest item here and it is the one that touches more than the iface
  component.

## Related

- `docs/features/interfaces.md`, the operator page for what RA does carry.
- the retired deferral shard "router-advertisement", the row this spec homes.
- `internal/core/ndp/ra.go` (`BuildRA`, `RALen`, `writeRDNSS`) and
  `internal/component/iface/config_ra.go` (`parseRAConfig`, `raValidate`), the
  two files any of this work edits.

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `router-advertisement.md`, 2026-08-02

Deferred by spec-router-advertisement.

The DNSSL option and the MTU option in Router Advertisements. RDNSS stays in scope
