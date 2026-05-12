# 694 -- web-1-identity

## Context

Operators managing multiple Ze routers in separate browser tabs had no persistent visual indicator of which router instance they were connected to. The workbench topbar showed "Ze workbench" with no device-specific identity. This created misconfiguration risk when editing config on the wrong router. The spec also called for a fleet selector to switch between routers without opening separate tabs.

## Decisions

- **Identity resolution priority: system/host > bgp/router-id > "ze"** over a dedicated display-name leaf. The hostname is already configured for operational use and is the most recognizable identifier. Router-id is the fallback for headless deployments. "ze" is the product-name default when nothing is configured. No new YANG leaf needed.
- **Identity in the topbar brand area** (between "Ze" and the breadcrumb) over a separate status bar or page footer. The topbar is always visible and is the first place operators look. A vertical border separator visually groups it with the product name without cluttering the breadcrumb.
- **Direct-link fleet model** over a proxy model. Each fleet peer entry is a simple URL link that opens the remote Ze instance in the browser. This avoids proxy complexity, auth forwarding, and latency. Per-router session state (pending edits, context path) is preserved naturally because each router has its own editor session.
- **Fleet peers from `system/fleet/peer[]` config** over dynamic BGP peer discovery. Static config is explicit, auditable, and does not require BGP to be running. Dynamic discovery can be added as a future enhancement without changing the display layer.
- **`RouterIdentity` and `FleetPeers` on `LayoutData`** over a separate topbar-specific data struct. LayoutData already flows to every template; adding two fields is simpler than threading a new struct through the render pipeline.

## Consequences

- Every workbench page shows the router identity in the topbar. Two tabs to different routers show different identity strings.
- Fleet selector renders only when `system/fleet/peer[]` is configured. Without fleet config, no dropdown appears (zero UI clutter for single-router deployments).
- The fleet selector is CSS hover-activated (same pattern as the portal menu), no new JavaScript required.
- Identity resolution is called once per full-page render, not per HTMX partial request.

## Gotchas

- `ResolveRouterIdentity` reads from the user's draft tree (via `viewTree`), not the committed config. If a user changes the hostname in their draft, the topbar shows the draft hostname before commit. This is consistent with how the rest of the workbench shows draft state.
- `CollectFleetPeers` does not mark any peer as Active in the current implementation. The `Active` field exists on `FleetPeer` for future use (matching current identity against peer names), but comparing identity strings to peer names is unreliable when hostnames differ from fleet entry names.
- The fleet dropdown uses CSS `:hover`/`:focus-within` activation, matching the portal menu pattern. On touch devices without hover, users must tap the toggle button.

## Files

- `internal/component/web/render.go` -- `RouterIdentity` and `FleetPeers` fields on LayoutData, FleetPeer type
- `internal/component/web/page_system.go` -- `ResolveRouterIdentity()`, `CollectFleetPeers()`
- `internal/component/web/handler_workbench.go` -- identity resolution and fleet collection wired into both LayoutData construction paths
- `internal/component/web/templates/component/workbench_topbar.html` -- identity display and fleet dropdown
- `internal/component/web/assets/style.css` -- `.workbench-identity`, `.fleet-menu`, `.fleet-dropdown`, `.fleet-item` styles
- `internal/component/web/page_system_test.go` -- 7 tests for identity resolution and fleet collection
