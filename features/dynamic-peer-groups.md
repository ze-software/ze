# Dynamic peer groups

## Meta

| Field | Value |
|-------|-------|
| Name | Dynamic peer groups |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/yang, internal/component/bgp/reactor |
| Real-path tests | test/parse/bgp-dynamic-peer-group.ci, test/parse/bgp-dynamic-peer-reject-at-peer.ci, test/plugin/dynamic-peer-applies-group-filters.ci, test/plugin/dynamic-peer-gets-group-role-capability.ci, test/plugin/dynamic-peer-negotiates-configured-families.ci, test/reload/reload-dynamic-peer-survives.ci |
| Interop | bgp/bgp-redist-late-join-dynamic-frr |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; max-peers range 1..100000 default 1000 matches internal/component/bgp/yang/ze-bgp-conf.yang |
| Defect review | 2026-10-07: journal gate-excludes-part-of-its-population rows 2026-08-12 (listener) and 2026-08-13 (name collision) taken as open |

## Description

A group whose `connection/remote/ip` is `dynamic` accepts a session from any address inside its `range` list and opens its own listening socket, so a configuration naming no static peer still accepts members. Each accepted connection becomes one peer that inherits the group's whole resolved settings, its families, its `attach process` blocks and its per-peer plugin config. Overlapping ranges resolve by longest prefix match. `max-peers` bounds the group (1..100000, default 1000). <!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf-list range, leaf max-peers -->
