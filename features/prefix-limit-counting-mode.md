# Prefix-limit counting mode

## Meta

| Field | Value |
|-------|-------|
| Name | Prefix-limit counting mode |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/yang, internal/component/bgp/reactor |
| Real-path tests | test/plugin/prefix-count-offered.ci, test/plugin/prefix-count-installed.ci, test/plugin/prefix-count-installed-cross-family.ci, test/plugin/prefix-count-installed-reannounce.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; enum offered and installed with default offered in internal/component/bgp/yang/ze-bgp-conf.yang |
| Defect review | 2026-10-07: journal invariant-enforced-by-an-absent-call-site and guard-added-to-one-half-of-a-pair (2026-10-03) rows on installed counting taken as open |

## Description

Per-family `prefix { count offered \| installed; }`, default `offered`. `offered` counts announcement events off the wire; `installed` counts the set of prefixes the family currently holds, so a peer that re-announces the same prefix does not walk toward its limit. <!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf count, enum offered/installed -->
