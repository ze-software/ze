# 689 -- web-5-bgp

## Context

BGP is Ze's primary use case, so the workbench BGP pages needed to deliver the complete operator loop: find a peer, inspect its live state, edit config, commit, verify the change took effect, all without leaving the workspace. The spec built five purpose-built pages (Peers, Groups, Summary, Families, Filters/Policy) on top of the spec-web-3-foundation components and the spec-web-2 related-tool infrastructure.

## Decisions

- **Config-tree-walking page builders** (`collectPeers`, `collectGroups`, `collectFamilies`, `collectPolicies`) over YANG-schema-driven generic list rendering. Purpose-built collectors walk both standalone (`bgp/peer`) and grouped (`bgp/group/*/peer`) locations, joining data from different config tree depths into flat table rows. A generic renderer could not merge grouped and standalone peers into one table.
- **Dual-location peer collection** (standalone + grouped) in `collectPeers` rather than two separate tables. Operators see all peers in one view with a Group column; the `filterGroup` parameter narrows to a single group when arriving from the Groups page's "View Peers" link.
- **Config-only v1 with operational placeholders** over blocking on live reactor data. State, uptime, prefix counts, and message counters show `"--"` placeholders. The `peerFlag` function returns green/grey based on config (configured/disabled); FSM state colors (red for Idle, yellow for Active/Connect) are deferred to a future spec that wires reactor `PeerInfo` into the enrichment pipeline.
- **Detail panel with three tabs** (Config, Status, Actions) built as `template.HTML` from Go string builders, not separate templates. The tabs are peer-specific and tightly coupled to peer fields; separate template files would add indirection without reuse.
- **Row-level tool actions via `hx-post` to `/tools/related/run`** with `tool_id` and `context_path` sent as form values. The server resolves the actual command from YANG `ze:related` annotations, so the browser never constructs command strings.
- **Confirmation dialogs on destructive actions** (Teardown) using `hx-confirm` attribute over custom modal components. Keeps the implementation minimal and consistent with HTMX patterns.
- **Policy page with hardcoded list-name candidates** (`filter`, `community`, `prefix-list`, `as-path`, `route-map`) rather than a generic `ListNames()` method on `config.Tree`. The Tree type does not expose list enumeration at a given level; known YANG list names are checked explicitly.

## Consequences

- All BGP config is viewable and editable from five workbench pages without visiting the Finder config browser.
- The peer detail panel provides Config/Status/Actions tabs, with operational tools (Detail, Capabilities, Statistics, Flush, Teardown) available as both row actions and detail-panel buttons.
- Groups show peer count and link to a filtered peers view, maintaining cross-page navigation within the workbench.
- The families view aggregates family config across all peers and groups into one cross-cutting table.
- Operational data columns are placeholder, ready for future wiring to the BGP reactor.

## Gotchas

- `collectPeers` walks both `bgp/peer` and `bgp/group/*/peer`, so adding a third peer location (e.g., dynamic peers) would require extending this function explicitly.
- The `listNamesFromTree` function in the policy page uses a hardcoded candidate list. If a new policy list type is added via a plugin YANG augment, it must be added to this list or it will not appear.
- `peerFlag` currently treats all non-disabled peers as "Configured" (green). When live state is wired in, the flag logic must change to use FSM state, which will flip most peers from green to their actual state color.
- `buildPeerActionsHTML` embeds `hx-vals` JSON in single-quoted HTML attributes. The `context_path` is HTML-escaped via `template.HTMLEscapeString`, but peer names with single quotes would break the JSON. Peer names are YANG-validated to exclude single quotes, so this is safe in practice.

## Files

- `internal/component/web/page_bgp_peers.go` -- peer collection, table builder, detail panel, actions
- `internal/component/web/page_bgp_groups.go` -- group collection, table builder, "View Peers" action
- `internal/component/web/page_bgp_families.go` -- cross-peer family aggregation table
- `internal/component/web/page_bgp_policy.go` -- policy/filter collection, rule counting
- `internal/component/web/page_bgp_summary.go` -- read-only summary table with operational placeholders
- `internal/component/web/handler_bgp_test.go` -- 35 test functions covering all five pages
