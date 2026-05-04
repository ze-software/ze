# MikroTik WinBox/WebFig vs Ze Workbench: Comparative Analysis

**Date:** 2026-04-27
**Purpose:** Identify patterns from MikroTik's RouterOS management UI that Ze should adopt, adapt, or deliberately reject. This document feeds future specs.
**Related:** `plan/design/web-interface-design.md` (Ze's existing web UI blueprint)

---

## 1. MikroTik's UI Architecture

### 1.1 WinBox (Native Desktop Client)

WinBox is a native MDI (Multi-Document Interface) application. The interface has three fixed zones:

| Zone | Location | Content |
|------|----------|---------|
| Menu tree | Left sidebar | Hierarchical list of all RouterOS subsystems |
| Toolbar | Top | Info fields (CPU, memory, uptime) configurable by user |
| Work area | Center | Child windows (tables, forms, graphs) that tile and overlap |

**Key properties:**
- The menu tree is a direct mirror of the CLI path hierarchy (`/ip/firewall/filter`, `/interface/ethernet`, `/routing/bgp`). This is deliberate: RouterOS documentation references CLI paths, and WinBox navigation matches those paths exactly.
- Multiple child windows can be open simultaneously. An operator can view firewall rules, an interface detail panel, and a traffic graph side by side.
- Each child window has its own toolbar with New/Enable/Disable/Remove and a Find/Filter bar.
- Object detail opens in a tabbed dialog (General, Ethernet, Status, Traffic, etc.) with Cancel/Apply/OK.
- A contextual "Actions" panel appears on the right when viewing certain objects (interfaces, queues). Actions are operational commands: Torch, Cable Test, Blink, Reset Counters.

### 1.2 WebFig (Browser-Based)

WebFig mirrors WinBox's structure in a browser:
- Same left menu tree, same table views, same detail forms.
- Single-window (no MDI), but uses tab-based sub-navigation within pages.
- "Design Skin" mode lets administrators hide fields, restrict value sets, and build custom status pages per user group.
- Branding support: custom login page, logo, CSS per deployment.

### 1.3 Quick Set

A simplified wizard that appears on first connection. Pre-fills router mode (Home AP, CPE, PTP Bridge, etc.) and exposes only the essential fields for that mode. After setup, users switch to the full WinBox/WebFig interface.

---

## 2. Ze's Current Architecture

### 2.1 Layout

CSS Grid with four rows and two columns:

```
topbar      topbar          (brand + breadcrumb + user + UI toggles)
nav         workspace       (220px sidebar | content area)
nav         notifications   (sidebar continues | notification panel)
commit      commit          (draft change count + Review/Discard)
```

### 2.2 Navigation

Two-level left sidebar with 12 sections, ~45 sub-pages. Sections group by operational concern (Dashboard, Interfaces, IP, Routing, Policy, Firewall, L2TP, Services, System, Tools, Logs, CLI) rather than mirroring YANG paths.

Selection logic in Go maps URL path segments to sections. The active section and sub-page are highlighted. Sections use HTML `<details>` for expand/collapse without JavaScript.

### 2.3 Content Rendering

Purpose-built page handlers (tables for interfaces, peers, routes, firewall rules, etc.) take priority. Unmatched paths fall through to a generic YANG tree detail view. HTMX handles fragment updates. SSE provides live streaming for logs and events.

### 2.4 Modes

Two UI modes selectable by the user:
- **Workbench** (default): Persistent sidebar + topbar, operator-oriented.
- **Finder**: Column-based YANG browser, closer to raw config tree.

---

## 3. Pattern-by-Pattern Comparison

### 3.1 Navigation Model

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Hierarchy | CLI path mirror (3-4 levels deep) | Domain grouping (2 levels) |
| Dynamic entries | Menu hides disabled packages | Static sections, always visible |
| Section count | ~15 top-level (Quick Set, WiFi, Wireless, Interfaces, WireGuard, Bridge, PPP, Switch, Mesh, IP, IPv6, MPLS, Routing, System) | 12 top-level |
| Deep nesting | IP > Firewall > Filter Rules > rule N > General/Advanced/Extra/Action tabs | Firewall > Rules (flat table) |

**Assessment:** Ze's two-level grouping is better for a web UI. MikroTik's deep nesting works in a desktop app with persistent windows, but in a browser it means excessive page loads. Ze's approach of flattening BGP/Policy/Firewall into separate top-level sections with sub-pages is the right call.

**Gap:** Ze's sections are static. MikroTik hides sections when the underlying package is not installed. Ze should consider dimming or hiding sections for components that are not enabled (e.g., hide L2TP section if L2TP is not configured, dim BGP section if no peers exist). This reduces visual noise for deployments that use only a subset of features.

### 3.2 Multi-Context Viewing

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Simultaneous views | MDI: multiple overlapping child windows | Single content pane |
| Cross-referencing | Open firewall rules + interface detail + traffic graph | Navigate away from one to see another |
| Detail inspection | Double-click row opens tabbed dialog over the table | Navigate to detail page, losing table context |

**Assessment:** Full MDI is wrong for a browser (window management UX is poor without native OS chrome). But the underlying need is real: operators troubleshoot across domains. "Why is traffic dropping on eth0?" requires interface counters, firewall rules, and BGP routes simultaneously.

**Recommendation:** Implement a **detail slide-out panel**. Clicking a table row opens a panel to the right (or bottom on narrow screens) showing the object's properties, without replacing the table. The panel uses `hx-target` to load content into a `<aside class="workbench-detail">` area. A close button or clicking another row replaces the panel content. This gives 80% of MDI's value with 20% of its complexity.

### 3.3 Contextual Actions

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Per-object operations | "Actions" panel: Torch, Cable Test, Blink, Reset Counters, Reset MAC | None |
| Per-table operations | New, Enable, Disable, Remove toolbar per table | No table-level action toolbar |
| Inline operations | Right-click context menu on table rows | None |

**Assessment:** This is Ze's biggest gap. MikroTik understands that network management is not just reading config: operators need to *act* on objects. Clear a BGP session. Reset interface counters. Run a ping from a specific source interface. Capture traffic on a port.

**Recommendation:** Add an **actions block** to entity detail views. The actions available depend on the entity type:

| Entity | Actions |
|--------|---------|
| Interface | Reset counters, Ping (source), Capture, Enable/Disable |
| BGP Peer | Clear session, Soft reset (in/out), View received routes |
| Firewall rule | Enable/Disable, Move up/down, Clone |
| L2TP session | Disconnect, Reset echo counters |
| System | Reboot, Save config, Export config |

These map to existing CLI commands dispatched through the `CommandDispatcher`. The UI element is a `<div class="entity-actions">` rendered by the page handler, populated from a registry of actions per entity type.

### 3.4 Status Bar / System Vitals

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Persistent vitals | Bottom bar: hostname, platform, version, CPU%, memory, uptime, date | None (data exists on Dashboard > Overview) |
| Always visible | Yes, across all views | No, only on dashboard page |

**Assessment:** A persistent status bar provides constant situational awareness. When you are deep in firewall rule editing, you still see that CPU is at 95% or that uptime shows the box just rebooted. This context prevents mistakes (e.g., committing a complex change on a router that is already under stress).

**Recommendation:** Add a `<footer class="workbench-statusbar">` as a new grid row:

```
topbar      topbar
nav         workspace
nav         notifications
commit      commit
statusbar   statusbar        <-- new
```

Content: hostname | version | uptime | CPU% | memory free/total

Updated via SSE (reuse the health endpoint data). Lightweight: one line of text, no graphs. Clicking it navigates to Dashboard > Health for full details.

### 3.5 Table Controls

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Column sorting | Click column header to sort | Not implemented |
| Filtering | Find bar, chain dropdown filter | URL query params (?type=ethernet) for some tables |
| Column visibility | Right-click header to show/hide columns | Not implemented |
| Detail mode | Toggle between table view and property-list view | Not implemented |
| Row count | Shown in bottom-left corner of each table | Not shown |

**Assessment:** Ze's tables are functional but static. MikroTik's table controls are polished because tables are the primary workspace, and operators spend hours looking at them.

**Recommendation:** Implement in phases:

1. **Phase 1:** Clickable column headers for client-side sort (vanilla JS, works on all tables). Row count in table footer.
2. **Phase 2:** Filter input above the table that does client-side text search across visible columns. Replaces URL-param filtering for simple cases.
3. **Phase 3 (defer):** Column visibility toggle, persistent column preferences per user.

### 3.6 Real-Time Data

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Inline traffic graphs | Byte graph in interface detail panel (live, embedded) | Separate traffic page (/show/iface/traffic/) |
| Torch | Real-time per-flow traffic analysis on any interface | Not implemented |
| Bandwidth test | Built-in throughput test between MikroTik devices | Not applicable (not peer-to-peer) |
| Counter auto-refresh | Tables auto-refresh counters | SSE for logs/events, not for table data |

**Assessment:** MikroTik's inline graphs are compelling because they eliminate navigation. You see the interface config, its status, and its traffic in one view. Ze has the SSE infrastructure but uses it only for logs and events, not for counter data.

**Recommendation:**

1. **Near-term:** Add a mini traffic sparkline (SVG, no library) to the interface detail view. Data from the same counters endpoint that feeds the traffic page. Updated via SSE or polling.
2. **Medium-term:** Implement Torch-equivalent as a tool accessible from the interface actions panel. Streams per-flow data via SSE. This is a high-value diagnostic tool.
3. **Defer:** Generalized inline graphs for BGP prefix counts, session age, etc.

### 3.7 Configuration Workflow

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Apply model | Immediate (click OK, change is live) | Staged draft with diff and commit |
| Rollback | Manual (`/system/backup`, undo in some cases) | Config diff, discard draft |
| Validation | Minimal (some field-level checks) | YANG-modeled validation |
| Bulk operations | Select multiple rows, apply action | Not implemented |

**Assessment:** Ze's draft/commit model is strictly superior for a network OS. RouterOS's immediate-apply model is a known pain point (typo in a firewall rule can lock you out). Ze should not copy this.

**What to take:** MikroTik's **bulk operations** (select multiple firewall rules, disable them all) are worth adopting. Table rows should support checkbox selection, with a bulk action bar (Enable/Disable/Delete selected).

### 3.8 Branding and Customization

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Custom skins | Per-user-group UI skins via Design Skin tool | Theme toggle (light/dark) |
| Field restriction | Hide fields, limit value sets per group | Not implemented |
| Custom status pages | Drag-and-drop status page builder per group | Not implemented |
| Custom login | Branding package with logo, CSS, login page | Login page exists but not customizable |

**Assessment:** MikroTik's skin system serves ISPs who provide managed CPE to customers. The customer gets a simplified WebFig that shows only what they need. Ze could need this if deployments serve different operator tiers, but it is low priority for now.

**Defer entirely.** Not relevant until Ze has multi-tenant or customer-facing deployments.

### 3.9 Search

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Global search | Not available (navigate menu tree) | Not implemented (design doc mentions it in top bar) |

**Assessment:** Neither has global search. MikroTik gets away with it because operators memorize the CLI path tree. Ze's design document specifies a search field in the top bar. This is worth implementing eventually, but not a gap relative to MikroTik.

### 3.10 Quick Set / Wizards

| Aspect | MikroTik | Ze |
|--------|----------|-----|
| Initial setup wizard | Quick Set (mode-aware, one-page form) | Not implemented |
| Guided workflows | None beyond Quick Set | Not implemented |

**Assessment:** Quick Set solves first-boot. Ze's gokrazy appliance model may make this less critical (the builder pre-configures the image). However, a "first peer" wizard that walks through group + peer + policy creation would reduce the learning curve significantly.

**Defer.** Useful but not urgent. When implemented, it should be a workbench page (not a modal), reachable from Dashboard when no peers are configured.

---

## 4. Recommendations Summary

### Adopt (high value, aligned with Ze's architecture)

| # | Pattern | Priority | Effort | Spec dependency |
|---|---------|----------|--------|-----------------|
| 1 | Persistent status bar | High | Low | None, standalone |
| 2 | Contextual actions panel | High | Medium | Needs action registry per entity type |
| 3 | Table sort (client-side) | High | Low | None, reusable JS component |
| 4 | Table filter bar | Medium | Low | None, reusable JS component |
| 5 | Detail slide-out panel | Medium | Medium | Needs CSS grid change + HTMX wiring |
| 6 | Table row count | Medium | Trivial | None |
| 7 | Bulk row selection + actions | Medium | Medium | Needs table checkbox component |
| 8 | Conditional section visibility | Low | Low | Needs component-enabled query |

### Adapt (take the concept, change the implementation)

| # | Pattern | MikroTik approach | Ze adaptation |
|---|---------|-------------------|---------------|
| 1 | Multi-context viewing | MDI overlapping windows | Detail slide-out, not windows |
| 2 | Inline traffic graphs | Full chart in detail panel | SVG sparkline, minimal footprint |
| 3 | Torch | Separate tool window | Tool launched from interface actions panel, streams via SSE |
| 4 | Quick Set | Mode-aware initial wizard | "First peer" guide on empty dashboard (deferred) |

### Reject (wrong for Ze)

| # | Pattern | Reason |
|---|---------|--------|
| 1 | CLI-mirrored menu tree | Ze groups by operator domain, not data model path. Better for web. |
| 2 | Immediate-apply config | Ze's draft/commit model is safer and more predictable. |
| 3 | MDI window management | Poor UX in browser. Slide-out panel achieves the same goal. |
| 4 | Design Skin / per-group UI | Premature. No multi-tenant requirement yet. |
| 5 | Column visibility toggle | Over-engineering for current table complexity. Defer to Phase 3. |

---

## 5. Implementation Order

The recommendations above group into natural spec-sized chunks:

**Spec candidate 1: Status bar + table controls**
- Persistent status bar (SSE-fed)
- Client-side column sort
- Client-side filter bar
- Row count in table footer
- These are all small, independent, and improve every page.

**Spec candidate 2: Contextual actions**
- Action registry (entity type to available actions mapping)
- Action rendering in detail views
- Command dispatch through existing `CommandDispatcher`
- This is the biggest UX improvement and needs its own spec.

**Spec candidate 3: Detail slide-out panel**
- CSS grid modification to support workspace split
- HTMX wiring for panel content loading
- Table row click behavior change
- Close/resize behavior
- This changes the workbench layout and should be its own spec.

**Spec candidate 4: Inline monitoring**
- SVG sparklines for interface traffic
- SSE subscription for counter data
- Torch-equivalent tool (interface actions integration)
- Depends on spec 2 (actions) for Torch placement.

---

## 6. References

- [WinBox Documentation](https://help.mikrotik.com/docs/spaces/ROS/pages/328129/WinBox)
- [WebFig Documentation](https://help.mikrotik.com/docs/spaces/ROS/pages/328131/WebFig)
- [Quick Set Documentation](https://help.mikrotik.com/docs/spaces/ROS/pages/328060/Quick+Set)
- [Torch Documentation](https://help.mikrotik.com/docs/spaces/ROS/pages/8323150/Torch)
- [Interface stats and monitor-traffic](https://help.mikrotik.com/docs/spaces/ROS/pages/139526175/Interface+stats+and+monitor+traffic)
- [Graphing Documentation](https://help.mikrotik.com/docs/spaces/ROS/pages/22773810/Graphing)
- [RouterOS CLI Documentation](https://help.mikrotik.com/docs/spaces/ROS/pages/328134/Command+Line+Interface)
- [WinBox v4 release thread](https://forum.mikrotik.com/t/winbox-v4-0-1-released/268595)
- Ze design blueprint: `plan/design/web-interface-design.md`
