# Architecture Documentation

Architecture documents describe how the current implementation is wired. Prefer source-anchored documents here over historical research notes when checking behavior.

## Core Map

| Topic | Document | Code anchor |
|-------|----------|-------------|
| One-page overview | `../architecture.md` | `internal/component/engine/`, `cmd/ze/hub/` |
| Core design | `core-design.md` | BGP, plugin, config, and engine packages |
| System architecture | `system-architecture.md` | Hub startup and subsystem wiring |
| Plugin manager wiring | `plugin-manager-wiring.md` | `internal/component/plugin/` |
| Subsystem wiring | `subsystem-wiring.md` | Registered components and plugin server wiring |
| Config design | `config/` | `internal/component/config/` and YANG modules |
| API and IPC | `api/` | `pkg/plugin/rpc/`, `pkg/plugin/sdk/`, command schemas |
| BGP wire format | `wire/` | `internal/core/bgp/attribute/`, `message/`, `wireu/` |
| Route and RIB behavior | `route-selection.md`, `route-types.md`, `rib-transition.md` | `internal/core/rib/`, `internal/component/bgp/plugins/rib/` |
| Pools and buffers | `pool-architecture.md`, `buffer-architecture.md` | `internal/component/bgp/attrpool/`, `internal/core/bufpool/` |
| Module tiers and placement | `module-tiers.md` | `internal/le/arch/tier/`, `internal/core/`, `internal/component/`, `internal/plugins/` |
| Web and UI | `web-interface.md`, `web-components.md` | `internal/component/web/` |
| Fleet interface | `fleet-interface.md` | `internal/component/web/fleetui/` |
| Testing architecture | `testing/` | `internal/test/cli/*.go`, `internal/test/`, `test/` |
| Session and signal behavior | `behavior/` | `internal/component/bgp/fsm/`, `internal/plugins/signal/` |
| BGP plugin behavior | `bgp/` | `internal/component/bgp/plugins/`, `internal/component/bgp/reactor/` |
| Local RIB internals | `rib/` | `internal/core/rib/locrib/`, `internal/core/bgp/nlri/nlrisplit/` |
| Protocol edge cases | `edge-cases/` | `internal/component/bgp/` |
| Route metadata keys | `meta/` | `internal/component/bgp/plugins/role/` |
| IGP routing protocols | `isis/`, `ospf/` | `internal/plugins/isis/`, `internal/plugins/ospf/` |
| MPLS label switching | `mpls/`, `ldp/`, `rsvpte/`, `fib/` | `internal/component/mpls/`, `internal/plugins/ldp/`, `internal/plugins/rsvpte/`, `internal/plugins/fib/` |
| First-hop redundancy | `vrrp/` | `internal/plugins/vrrp/` |
| IKE and IPsec | `ike/` | `internal/component/ike/` |
| Subscriber access | `l2tp/` | `internal/component/l2tp/` |
| Interfaces | `iface/` | `internal/component/iface/` |
| Firewall | `firewall/` | `internal/component/firewall/`, `internal/plugins/firewall/` |
| Policy routing | `policyroute/` | `internal/plugins/policyroute/` |
| Traffic control and accounting | `traffic/` | `internal/plugins/traffic/`, `internal/plugins/trafficusage/`, `internal/core/stats/` |
| Flow export | `flowexport/` | `internal/plugins/flowexport/` |
| Anomaly and DDoS response | `anomaly/`, `ddos/` | `internal/plugins/anomaly/`, `internal/plugins/ddos/` |
| DNS services | `dns/` | `internal/core/dnsserver/`, `internal/plugins/as112/`, `internal/plugins/geodns/` |
| Diagnostics and crash capture | `diagnostics/` | `internal/core/crashlog/`, `internal/core/procfs/`, `internal/plugins/diag/` |
| Host hardware | `host/` | `internal/component/host/`, `internal/plugins/host/` |
| Storage health | `storage/` | `internal/component/storage/`, `internal/core/smart/` |
| Appliance, install, and update | `appliance/` | `internal/appliance/`, `internal/install/disk/`, `cmd/ze-installer/` |
| Provisioning services | `provisioning/` | `internal/plugins/dhcpserver/`, `internal/plugins/tftpserver/`, `internal/plugins/imageserver/` |
| Certificate store | `pki/` | `internal/component/pki/` |
| Credential masking | `ssh/` | `internal/component/ssh/`, `internal/core/redact/` |
| MCP server | `mcp/` | `internal/component/mcp/` |
| Plugin boundaries and RIB storage | `plugin/` | `internal/component/plugin/`, `pkg/ze/` |
| Plugin test tools | `debugging/` | `internal/component/plugin/cli/`, `internal/test/plugins/` |
| CLI surface | `cli/` | `cmd/ze/`, `internal/component/command/` |
| Memory lifetime contracts | `memory/` | `internal/core/memguard/` |
| Decisions | `decisions/` | Decision records tied to current implementation |

## Page format

A design page is any page a source file's `// Design:` header names. Thomas's
decisions about the subject such a page documents are written on that page,
in one section, and nowhere else: no separate register of decisions exists.
The header is the link an agent already follows from the file it edits, so a
decision written there is one an agent meets before the edit.

| Element | Rule |
|---------|------|
| Heading | `## Design decisions`, a whole line, at most once per page. A heading shown inside a fenced code block is not the section |
| Table | One table under the heading, its columns `Decision`, `It forbids`, `Source`, `Date`, in that order, with a separator row under them. The section ends at the next heading of level one or two |
| Decision | The decision as Thomas stated it, about this page's subject. Never empty |
| It forbids | What a design must not do because of it: the shape a reviewer checks a diff against. Never empty |
| Source | One or more entries, separated by commas: a path from the repository root of the record holding his words (never under `tmp/`), a commit id of 7 to 40 hex digits, or `ai/META-DECISIONS.md "<section heading>"`. Every entry must resolve |
| Date | The day he made the decision, `YYYY-MM-DD` |
| No decision recorded | The page carries no such section. An empty table is never written: it would claim that no decision binds the page, which nobody can know |
| Generated pages | A page a generator writes (a rendered rule) carries none; its decisions go to the page its generator reads |

A row is added freely, by the main session, in the turn Thomas states the
decision. A row is changed or removed only with him: through an approved
`departs` row of a spec, or through `./le commit create ... decision-change
"<his words>"` run from his terminal, which records
`Decision-changed: <page>: <his words>` in the commit.

`./le doc consistency` judges the section on every page a `// Design:` header
names: a row breaking a rule above, a repeated heading, a section with no row
or the wrong columns are `design-decisions` errors naming the page and the
row, and a header naming a `.md` page that does not exist is a `design-refs`
error naming the file and its header line. Tree-wide these are reds owed at
release. The same judgment runs as a change-set check of `./le commit create`
(`docs/contributing/committing.md`), which refuses a commit for a malformed
row it adds and for a row present at its parent that it changes or removes,
and for nothing the section already held. Adding a row and reordering rows
need nobody.
<!-- source: internal/le/doc/consistency/decisions.go -- ParseDecisions, JudgeDecisionRow, checkDesignDecisions -->
<!-- source: internal/le/doc/consistency/consistency.go -- checkDesignRefs, designHeader -->
<!-- source: internal/le/doc/consistency/decisionschange.go -- judgeDecisionRows, LostDecisions -->

## Reading Order

1. Start with `../architecture.md` for the current component map.
2. Read `core-design.md` for the detailed implementation model.
3. Move into the topic directory that matches the package you are changing.
4. Use source anchors in the document to verify claims against code.
