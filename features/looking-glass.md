# Looking Glass

## Meta

| Field | Value |
|-------|-------|
| Name | Looking Glass |
| Page | docs/features/looking-glass.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/lg |
| Real-path tests | test/ui/lg-birdwatcher-counts-unavailable.ci, test/ui/lg-peer-table-flat-payload.ci, test/reload/lg-pki-reference-reload.ci |
| Docs | docs/features/looking-glass.md |
| Doc review | 2026-10-07: ze-lg-conf.yang leaf tls carries default true, and token and certificate leaves exist; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming the looking glass |

## Description

Public BGP looking glass with birdwatcher API, AS path graphs, and BMP-monitored route display. Runs on its own configurable port, separate from the web UI. `tls` defaults to **true**. The `certificate` leaf names a `pki { certificate <name> }` entry, and the listener then serves that leaf with every intermediate the store holds. An unset leaf keeps the self-signed certificate. An optional `token` gates every route and is compared in constant time. An unset token leaves the looking glass open, which is the default. Pages render through templ with typed view models, not `html/template`. <!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf tls default true, leaf token, leaf certificate -->
