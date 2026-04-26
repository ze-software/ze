# Ze Web Interface Design: Operator Workbench

## What This Document Is

A comprehensive UI design for the Ze web interface, inspired by RouterOS 7 WebFig
but adapted to Ze's capabilities. This is the blueprint that must be reviewed and
approved before any further implementation work.

Every section below describes what a page shows, what actions are available, and
what happens when the page is empty. Nothing here is a vague category label pointing
at a YANG tree path.

---

## Design Principles

1. **Every page must be useful.** If a page has no data, it shows the empty table with
   column headers and an "Add" button. A blank page with no action is never acceptable.

2. **Configuration follows the network stack.** Interfaces first (L2), then addresses
   (L3), then routing protocols, then policy. The navigation order matches the order
   an operator builds a working router.

3. **Tables are the primary view.** Lists of objects (peers, interfaces, addresses,
   rules) render as tables with sortable columns, not as YANG tree browsers.

4. **Every table row has actions.** View details, edit, enable/disable, delete. Where
   operational commands exist (peer detail, interface counters), they appear as row
   actions.

5. **Status is visible inline.** BGP peer state, interface link status, session uptime
   are shown as table columns, not hidden behind clicks.

6. **Monitor and configure in the same place.** An interface page shows both its
   configuration AND its traffic counters. A BGP peer page shows both its config AND
   its session state.

---

## Navigation Structure

### Top Bar

| Element | Purpose |
|---------|---------|
| Ze logo/name | Link to dashboard |
| Breadcrumb trail | Shows current location (e.g., Routing > BGP > Peers > neighbor-1) |
| Pending changes indicator | Badge showing count of uncommitted changes; click opens diff/commit panel |
| Search | Global search across config objects (interfaces, peers, addresses, rules) |
| Username | Current user identity |
| CLI toggle | Opens/closes the CLI bar at the bottom |

### Left Navigation (Sidebar)

The sidebar has two levels: sections and sub-pages. Sections expand to show their
pages. The order follows the network stack bottom-up, then system, then tools.

```
Dashboard
  Overview
  Health
  Recent Events

Interfaces
  All Interfaces        (table: all interface types)
  Ethernet              (filtered: ethernet only)
  Bridge                (filtered: bridge only)
  VLAN                  (filtered: vlan units only)
  Tunnel                (filtered: GRE, IP-in-IP, WireGuard, etc.)
  Traffic               (live counters for all interfaces)

IP
  Addresses             (table: all IP addresses on all interfaces)
  Routes                (table: routing table, static routes)
  DNS                   (resolver configuration)

Routing
  BGP
    Peers               (table: all BGP peers)
    Groups              (table: peer groups)
    Families            (table: address family config)
    Summary             (operational: peer state overview)
  Redistribute          (route redistribution config)

Policy
  Filters               (table: BGP import/export filter chains)
  Communities           (table: community lists)
  Prefix Lists          (table: prefix filter lists)

Firewall
  Tables                (table: nftables tables)
  Chains                (table: chains within selected table)
  Rules                 (table: rules within selected chain)
  Sets                  (table: named sets)
  Connections           (operational: conntrack table)

L2TP
  Sessions              (table: active L2TP sessions)
  Configuration         (L2TP server settings)
  Health                (operational: session health sorted by echo loss)

Services
  SSH                   (SSH server configuration)
  Web                   (Web server / portal configuration)
  Telemetry             (metrics collection configuration)
  TACACS                (TACACS+ server configuration)
  MCP                   (MCP/OAuth configuration)
  Looking Glass         (looking glass configuration)
  API                   (gRPC API configuration)

System
  Identity              (hostname, router-id)
  Users                 (table: users and access control)
  Resources             (operational: CPU, memory, disk, uptime)
  Host Hardware         (operational: NIC, CPU, storage, thermal, DMI inventory)
  Sysctl Profiles       (table: sysctl profiles)

Tools
  Ping                  (interactive ping tool)
  BGP Decode            (decode hex BGP messages)
  Metrics Query         (Prometheus metrics browser)
  Capture               (packet capture viewer)

Logs
  Live Log              (streaming log viewer with topic filtering)
  Warnings              (active operator warnings)
  Errors                (recent error events)
```

### Navigation Behavior

- Clicking a section label expands/collapses its sub-pages
- Clicking a sub-page navigates to that page
- The active page is highlighted in the sidebar
- Sections with problems (e.g., BGP peers down, interface errors) show a small
  warning indicator next to the section name
- Collapsing the sidebar (on narrow viewports) shows only icons

---

## Page Designs

### Dashboard > Overview

**Purpose:** First thing an operator sees. Quick health assessment.

| Panel | Content | Source |
|-------|---------|--------|
| System | Hostname, uptime, version, CPU load, memory usage | `show version`, `show uptime`, `show system/memory`, `show system/cpu` |
| BGP Summary | Peer count by state (established/active/idle/down), total prefixes | `show bgp-health` |
| Interfaces | Interface count by state (up/down/admin-down), total traffic rate | `show interface counters` |
| Active Warnings | List of current warnings with severity | `show warnings` |
| Recent Errors | Last 10 errors with timestamps | `show errors` |
| Recent Events | Last 10 events with timestamps | `show event/recent` |

**Auto-refresh:** SSE or polling every 5 seconds for counters, 30 seconds for health.

**Empty state:** If Ze is freshly installed with no config, show:
- "No interfaces configured" with link to Interfaces > All Interfaces
- "No BGP peers configured" with link to Routing > BGP > Peers
- "System healthy, no warnings" in the warnings panel

### Dashboard > Health

**Purpose:** Aggregated component health at a glance.

A table with one row per component (BGP, interfaces, L2TP, DNS, etc.):

| Column | Content |
|--------|---------|
| Component | Name (BGP, Interfaces, L2TP, ...) |
| Status | Green/Yellow/Red indicator |
| Summary | Brief text (e.g., "4/5 peers established", "all interfaces up") |
| Last Change | Timestamp of last status change |

**Source:** `show health`, `show bgp-health`, `show l2tp-health`

### Dashboard > Recent Events

**Purpose:** Event log with namespace filtering.

| Column | Content |
|--------|---------|
| Time | Timestamp |
| Namespace | Event source (bgp, iface, config, ...) |
| Message | Event text |

**Source:** `show event/recent`, `show event/namespaces` for filter dropdown.

---

### Interfaces > All Interfaces

**Purpose:** See every network interface, its state, and traffic.

**Table columns:**

| Column | Content |
|--------|---------|
| Flags | R (running), X (disabled), D (dynamic) |
| Name | Interface name (clickable to detail) |
| Type | ethernet, bridge, vlan, tunnel, dummy, veth |
| OS Name | Original OS interface name |
| MTU | Configured MTU |
| MAC | MAC address (L2 types only) |
| Link State | Up/Down (from operational data) |
| TX Rate | Current transmit rate |
| RX Rate | Current receive rate |
| Description | User description |

**Toolbar actions:**
- **Add Interface** (dropdown: ethernet, bridge, dummy, veth, tunnel subtypes)
- **Filter by type** (dropdown)
- **Search** (filter by name/description)

**Row actions:**
- **View** (open detail panel)
- **Edit** (open config form)
- **Enable/Disable** (toggle admin state)
- **Counters** (show detailed TX/RX/error/drop counters)
- **Delete** (with confirmation)

**Empty state:** Table headers visible. "No interfaces configured. Add Interface to begin." with prominent Add button.

**Detail panel (when a row is clicked):**

Split into sections:

*Configuration:*
- Name, type, description, MTU, MAC address, admin state
- All editable inline

*Units (sub-table):*
- Table of logical units on this interface
- Columns: Unit ID, VLAN ID, Addresses, VRF, Description, State
- Add Unit button

*Status (read-only):*
- Link state, actual MTU, running flag

*Traffic Counters (read-only, auto-refresh):*
- RX: bytes, packets, errors, drops
- TX: bytes, packets, errors, drops
- Fast-path counters if available

*Actions:*
- Clear Counters (with confirmation)

### Interfaces > Ethernet / Bridge / VLAN / Tunnel

Same table as All Interfaces but pre-filtered by type. Each has the same actions
and empty states appropriate to the type.

### Interfaces > Traffic

**Purpose:** Real-time traffic monitoring across all interfaces.

A table sorted by traffic rate (descending):

| Column | Content |
|--------|---------|
| Interface | Name |
| TX bps | Current transmit bits/sec |
| RX bps | Current receive bits/sec |
| TX pps | Transmit packets/sec |
| RX pps | Receive packets/sec |
| TX Errors | Error count |
| RX Errors | Error count |
| TX Drops | Drop count |
| RX Drops | Drop count |

**Auto-refresh:** Every 2-5 seconds via SSE or polling.

**Source:** `show interface counters` or per-interface monitor.

**Empty state:** "No interfaces to monitor."

---

### IP > Addresses

**Purpose:** See and manage all IP addresses across all interfaces.

**Table columns:**

| Column | Content |
|--------|---------|
| Flags | X (disabled), D (dynamic) |
| Address | IP/prefix (e.g., 10.0.0.1/24) |
| Network | Network address |
| Interface | Interface.unit where assigned |
| VRF | VRF membership if any |
| Protocol | IPv4 or IPv6 |
| Description | From the unit description |

**Toolbar actions:**
- **Add Address** (form: address, interface selector, VRF)
- **Filter** by interface, protocol (v4/v6), VRF
- **Search** by address or interface

**Row actions:**
- **Edit** (change address, move to different interface)
- **Delete** (with confirmation)
- **Go to Interface** (navigate to the parent interface detail)

**Empty state:** "No IP addresses configured. Add an address to an interface to enable L3 connectivity." with Add Address button.

**Workflow note:** This is where an operator goes after creating interfaces. The
page should make clear which interfaces have no addresses yet.

### IP > Routes

**Purpose:** View the routing table and manage static routes.

**Table columns:**

| Column | Content |
|--------|---------|
| Flags | A (active), S (static), B (BGP), C (connected), D (dynamic) |
| Destination | Prefix |
| Gateway | Next-hop address or interface |
| Distance | Admin distance |
| Metric | Route metric |
| Protocol | Source (connected, static, bgp, ospf) |
| Interface | Outgoing interface |
| Table | Routing table name |

**Toolbar actions:**
- **Add Static Route** (form: destination, gateway, distance, metric, table)
- **Filter** by protocol, table, prefix
- **Search** by destination

**Row actions (static routes only):**
- **Edit**
- **Enable/Disable**
- **Delete**

**Empty state for static routes:** "No static routes. Connected routes appear automatically when interfaces have addresses."

### IP > DNS

**Purpose:** Configure DNS resolver settings.

**Form layout (not a table, since this is a singleton config):**

| Field | Type | Description |
|-------|------|-------------|
| Upstream Servers | List (add/remove) | DNS resolver addresses |
| Cache enabled | Toggle | Enable/disable DNS cache |
| Cache size | Number | Maximum cache entries |

**Empty state:** "No DNS resolvers configured. Add upstream DNS servers for name resolution."

---

### Routing > BGP > Peers

**Purpose:** The primary BGP operational page. Every peer, its config, and its live state.

**Table columns:**

| Column | Content |
|--------|---------|
| Flags | E (established), X (disabled), A (active) |
| Name | Peer name (clickable to detail) |
| Remote IP | Neighbor address |
| Remote AS | Peer ASN |
| Local AS | Local ASN (if overridden) |
| Group | Parent group (if any) |
| State | Session state: Established/Active/Connect/Idle/OpenSent/OpenConfirm |
| Uptime | Time since last state change |
| Prefixes | Received prefix count |
| TX/RX Messages | Message counters |
| Last Error | Last notification/error reason |

**Color coding:**
- **Green row:** Established
- **Red row:** Idle or with errors
- **Grey row:** Disabled
- **Yellow row:** Active/Connect (trying to establish)

**Toolbar actions:**
- **Add Peer** (form: name, remote IP, remote AS, local AS, group, families)
- **BGP Health** (modal: `show bgp-health`)
- **Filter** by state, group, AS
- **Search** by name, IP, AS

**Row actions:**
- **Detail** (drawer: `peer <ip> detail`)
- **Capabilities** (modal: `peer <ip> capabilities`)
- **Statistics** (modal: `peer <ip> statistics`)
- **Edit** (open config form)
- **Enable/Disable**
- **Flush** (soft reset, with confirmation)
- **Teardown** (hard close, with confirmation + danger styling)
- **Delete** (with confirmation)

**Empty state:** "No BGP peers configured." with a prominent "Add Peer" button
and a brief hint: "Define a peer by specifying a neighbor IP address and remote
AS number. You will also need to configure at least one address family."

**Detail panel (when a row is clicked):**

*Tabs within the detail panel:*

**Config tab:**
- All peer configuration fields, editable
- Connection: remote IP, remote port, local IP, local port, MD5, TTL, BFD
- Session: remote AS, local AS, local options, router-id, route-reflector-client, cluster-id, next-hop
- Families sub-table: family name, mode, prefix limits, default-originate
- Filters: import chain, export chain
- Updates: announced routes

**Status tab (read-only, auto-refresh):**
- Session state, uptime, last state change
- Negotiated capabilities (4-byte ASN, add-path, extended nexthop, graceful restart)
- Received prefixes, sent prefixes
- Messages sent/received, bytes transferred
- Hold time, keepalive time
- Last notification message and code
- FSM transition history (last N transitions with timestamps)

**Actions tab:**
- Flush (soft reset)
- Teardown (hard close)
- Route refresh (when available)

### Routing > BGP > Groups

**Purpose:** Manage peer groups that share configuration.

**Table columns:**

| Column | Content |
|--------|---------|
| Name | Group name (clickable to detail) |
| Peer Count | Number of peers in group |
| Remote AS | Default remote AS |
| Families | Configured families |
| Description | Group purpose |

**Toolbar actions:**
- **Add Group**
- **Search**

**Row actions:**
- **View Peers** (navigate to peers filtered by this group)
- **Edit**
- **Delete** (blocked if group has peers, unless force)

**Empty state:** "No peer groups configured. Groups let you share configuration
across multiple peers." with Add Group button.

**Detail panel:**
- Group-level configuration (same fields as peer, but inherited by members)
- Sub-table of member peers

### Routing > BGP > Families

**Purpose:** Manage address family configuration.

**Table columns:**

| Column | Content |
|--------|---------|
| Family | e.g., ipv4/unicast, ipv6/unicast, ipv4/multicast |
| Mode | enable/disable/require/ignore |
| Max Prefixes | Hard limit |
| Warning Threshold | Percentage |
| Teardown on Limit | Yes/No |
| Default Originate | Yes/No |

This shows the family configuration across all peers/groups.

### Routing > BGP > Summary

**Purpose:** Operational overview of all BGP sessions.

**Source:** `show bgp-health`

A read-only table sorted by state (problems first):

| Column | Content |
|--------|---------|
| Peer | Name and IP |
| Remote AS | ASN |
| State | Session state |
| Uptime | Duration |
| Prefixes | Received count |
| Messages In/Out | Counter |
| Last Error | Error text |

This is the quick-glance page for "is BGP healthy?" It links to the full Peers
page for actions.

### Routing > Redistribute

**Purpose:** Configure route redistribution between protocols.

| Column | Content |
|--------|---------|
| Name | Policy name |
| Source | connected/static/bgp/ospf |
| Target | Protocol receiving routes |
| Filter | Applied filter |

**Empty state:** "No redistribution policies configured."

---

### Policy > Filters

**Purpose:** Manage BGP route filters (import/export chains).

**Table columns:**

| Column | Content |
|--------|---------|
| Name | Filter name (clickable to detail) |
| Type | Import/Export |
| Used By | Peers/groups referencing this filter |
| Rule Count | Number of match/action rules |

**Toolbar actions:**
- **Add Filter**
- **Search**

**Detail panel:**
- Ordered list of rules within the filter
- Each rule: match conditions, action (accept/reject/modify)
- Drag-to-reorder or move up/down buttons

**Empty state:** "No route filters configured. Filters control which routes
are accepted from or advertised to peers." with Add Filter button.

### Policy > Communities / Prefix Lists

Similar table-based pages for community lists and prefix lists used in filters.

---

### Firewall > Tables

**Purpose:** Manage nftables tables.

**Table columns:**

| Column | Content |
|--------|---------|
| Name | Table name |
| Family | inet, ip, ip6, arp, bridge, netdev |
| Chains | Count of chains |
| Sets | Count of sets |

**Toolbar actions:**
- **Add Table**

**Row actions:**
- **View Chains** (navigate to chains filtered by this table)
- **Edit**
- **Delete** (with confirmation: "This will remove all chains and rules")

**Empty state:** "No firewall tables configured. Create a table to start defining packet filtering rules." with Add Table button.

### Firewall > Chains

**Table columns:**

| Column | Content |
|--------|---------|
| Table | Parent table |
| Name | Chain name |
| Type | filter, nat, route |
| Hook | input, output, forward, prerouting, postrouting, ingress, egress |
| Priority | Numeric priority |
| Policy | accept, drop |
| Rule Count | Number of rules |
| Packet Count | Total packets matched |
| Byte Count | Total bytes matched |

**Toolbar actions:**
- **Add Chain** (with table selector)
- **Filter** by table, hook, type

**Row actions:**
- **View Rules** (navigate to rules for this chain)
- **Edit**
- **Delete**

**Empty state:** "No chains in this table. Add a chain with a hook point to start filtering traffic."

### Firewall > Rules

**Table columns:**

| Column | Content |
|--------|---------|
| # | Rule number (order matters) |
| Flags | X (disabled) |
| Chain | Parent chain |
| Match | Source, dest, protocol, port summary |
| Action | accept, drop, reject, log, queue |
| Packets | Hit counter |
| Bytes | Byte counter |
| Comment | Rule description |

**Toolbar actions:**
- **Add Rule** (form with match conditions + action)
- **Filter** by chain, action

**Row actions:**
- **Edit**
- **Enable/Disable**
- **Move Up/Down** (reorder)
- **Clone** (duplicate this rule)
- **Delete**

**Empty state:** "No rules in this chain. The default policy ({policy}) applies to all traffic. Add a rule to start filtering."

### Firewall > Sets

**Table columns:**

| Column | Content |
|--------|---------|
| Table | Parent table |
| Name | Set name |
| Type | ipv4, ipv6, ether, inet-service, mark, ifname |
| Flags | constant, interval, timeout |
| Elements | Count of elements |

**Row actions:**
- **View/Edit Elements** (expand to show set members)
- **Add Element**
- **Delete**

**Empty state:** "No named sets. Sets let you group addresses, ports, or other values for use in firewall rules."

### Firewall > Connections

**Purpose:** View active connection tracking entries.

Read-only table (no add/edit, this is operational data):

| Column | Content |
|--------|---------|
| Protocol | TCP/UDP/ICMP/... |
| Source | Source IP:port |
| Destination | Dest IP:port |
| State | NEW/ESTABLISHED/RELATED/... |
| Timeout | Seconds remaining |
| Packets | Packet count |
| Bytes | Byte count |

**Toolbar:**
- **Search** by IP, port, protocol
- **Refresh**

---

### L2TP > Sessions

**Table columns:**

| Column | Content |
|--------|---------|
| Tunnel ID | L2TP tunnel identifier |
| Session ID | Session identifier |
| Peer | Remote endpoint |
| State | Active/Idle |
| Uptime | Session duration |
| TX/RX | Traffic counters |
| Echo Loss | Echo request loss percentage |

**Row actions:**
- **Detail** (full session info)
- **Disconnect** (with confirmation)

**Empty state:** "No active L2TP sessions."

### L2TP > Configuration

Form-based configuration page:

| Field | Type | Description |
|-------|------|-------------|
| Enabled | Toggle | Enable/disable L2TP |
| Max Tunnels | Number | Maximum tunnel count (0 = unlimited) |
| Max Sessions | Number | Maximum session count |
| Shared Secret | Password | Authentication secret |
| Hello Interval | Number (seconds) | Keepalive interval |
| CQM Enabled | Toggle | Connection Quality Monitoring |
| Max Logins | Number | Maximum concurrent logins |
| Server Endpoints | Table | IP:port listener endpoints (add/remove) |

**Empty state for endpoints:** "No L2TP listener endpoints configured. Add a server endpoint to accept L2TP connections."

### L2TP > Health

Read-only operational table sorted by echo loss (worst first):

| Column | Content |
|--------|---------|
| Session | Identifier |
| Peer | Remote endpoint |
| Echo Loss % | Packet loss |
| Latency | Round-trip time |
| State | Health status |

**Source:** `show l2tp-health`

---

### Services

Each service gets a configuration form page. Services are singletons, not lists,
so they use forms rather than tables.

### Services > SSH

| Field | Description |
|-------|-------------|
| Listen endpoints | Table of IP:port pairs |
| Allowed key types | Checkboxes |
| Ciphers | Multi-select |
| Authentication | Password/key/both |

### Services > Web

| Field | Description |
|-------|-------------|
| Listen port (HTTP) | Number |
| Listen port (HTTPS) | Number |
| TLS certificate | Path or auto-generate |
| Portal branding | Portal title, logo |

### Services > Telemetry

| Field | Description |
|-------|-------------|
| Enabled | Toggle |
| Collection interval | Duration |
| Collectors | Checkboxes for Linux kernel, hardware, process metrics |
| Export endpoint | Prometheus endpoint config |

### Services > TACACS

| Field | Description |
|-------|-------------|
| Servers | Table of server addresses |
| Shared secrets | Per-server secrets |
| Timeout | Request timeout |

### Services > MCP

| Field | Description |
|-------|-------------|
| OAuth configuration | Client ID, secret, endpoints |
| JWT settings | Key, algorithm, issuer |
| Bearer tokens | Table of tokens |

### Services > Looking Glass

| Field | Description |
|-------|-------------|
| Enabled | Toggle |
| Public access | Toggle |
| Allowed queries | Checkboxes for peer, route, etc. |

### Services > API (gRPC)

| Field | Description |
|-------|-------------|
| Listen address | IP:port |
| TLS | Toggle + certificate config |

---

### System > Identity

| Field | Description |
|-------|-------------|
| Hostname | System hostname |
| Router ID | IPv4 router identifier |

### System > Users

**Table columns:**

| Column | Content |
|--------|---------|
| Username | User name |
| Groups | Group memberships |
| Permissions | Summary of access level |
| Last Login | Timestamp |

**Toolbar actions:**
- **Add User**
- **Search**

**Row actions:**
- **Edit** (change password, groups, permissions)
- **Disable/Enable**
- **Delete** (with confirmation)

**Empty state:** "No users configured beyond the default admin."

### System > Resources

Read-only operational page (not a table, a property list):

| Property | Value |
|----------|-------|
| Uptime | Duration since start |
| Version | Ze version and build |
| CPU Cores | Count |
| CPU Load | Percentage |
| GOMAXPROCS | Go runtime setting |
| Goroutines | Active goroutine count |
| Memory Allocated | Current allocation |
| Memory Total | Heap size |
| GC Runs | Garbage collection count |
| Current Time | Wall clock |

**Auto-refresh:** Every 5 seconds.

### System > Host Hardware

Read-only hardware inventory organized by subsystem:

**CPU section:**
- Table of cores with vendor, model, frequency, governor

**NIC section:**
- Table of physical NICs with driver, PCI slot, link speed, firmware

**Storage section:**
- Table of block devices with size, model, transport, firmware

**Memory section:**
- Total physical RAM, ECC error count

**Thermal section:**
- Table of temperature sensors with current reading, throttle count

**DMI section:**
- Vendor, board, BIOS version, chassis type

**Source:** `show host/all`

### System > Sysctl Profiles

**Table columns:**

| Column | Content |
|--------|---------|
| Name | Profile name (dsr, router, hardened, multihomed, proxy, custom) |
| Applied To | Interfaces/units using this profile |
| Parameters | Key settings summary |

**Row actions:**
- **View** (show all sysctl values)
- **Edit** (custom profiles only)

---

### Tools > Ping

**Interactive form:**

| Field | Description |
|-------|-------------|
| Destination | IP or hostname |
| Count | Number of pings (default: 5) |
| Packet Size | Bytes |
| Source Interface | Optional interface selector |
| Timeout | Per-packet timeout |

**Results area:** Live results as they arrive, showing per-hop latency, loss
percentage, min/avg/max summary.

**Source:** `show ping <dest> count <n> timeout <t>`

### Tools > BGP Decode

| Field | Description |
|-------|-------------|
| Hex Input | Text area for hex-encoded BGP message |

**Result:** Decoded message fields in structured display.

### Tools > Metrics Query

| Field | Description |
|-------|-------------|
| Metric Name | Searchable dropdown of available metrics |
| Label Filter | Optional label=value pairs |

**Result:** Current metric value(s) in table format.

**Source:** `show metrics-query <name> [label-filter]`

### Tools > Capture

| Field | Description |
|-------|-------------|
| Tunnel ID | Optional filter |
| Peer | Optional filter |
| Count | Max packets to show |

**Result:** Table of captured L2TP control messages.

**Source:** `show capture`

---

### Logs > Live Log

**Purpose:** Real-time log viewer.

**Display:** Scrolling log entries with:

| Column | Content |
|--------|---------|
| Time | Timestamp |
| Level | info/warning/error/debug |
| Namespace | Source component |
| Message | Log text |

**Toolbar:**
- **Filter by namespace** (dropdown with registered namespaces)
- **Filter by level** (checkboxes: error, warning, info, debug)
- **Search** text filter
- **Pause/Resume** streaming
- **Clear** display

**Source:** SSE stream from event bus, or polling `show event/recent`.

**Color coding:**
- Red: errors
- Yellow: warnings
- Normal: info
- Grey: debug

### Logs > Warnings

Read-only table of active warnings:

| Column | Content |
|--------|---------|
| Time | When warning was raised |
| Component | Source |
| Message | Warning text |
| Duration | How long active |

**Source:** `show warnings`

**Empty state:** "No active warnings. All systems operating normally."

### Logs > Errors

Read-only table of recent errors:

| Column | Content |
|--------|---------|
| Time | Timestamp |
| Component | Source |
| Message | Error text |

**Source:** `show errors`

**Empty state:** "No recent errors."

---

## Common UI Patterns

### Table Pattern

Every table in the system follows the same pattern:

```
+-------------------------------------------------------------+
| Section Title                    [Add] [Filter v] [Search___]|
+-------------------------------------------------------------+
| Flags | Name     | Col2    | Col3    | Status   | Actions   |
|-------|----------|---------|---------|----------|-----------|
| R     | eth0     | 1500    | ...     | Up       | [v][e][x] |
| RX    | eth1     | 1500    | ...     | Down     | [v][e][x] |
|-------|----------|---------|---------|----------|-----------|
```

- **[v]** = View/Detail, **[e]** = Edit, **[x]** = Delete
- Row click opens detail panel on the right or below
- Column headers are clickable for sorting
- Color coding: red=problem, grey=disabled, green=healthy

### Empty State Pattern

Every table when empty shows:

```
+-------------------------------------------------------------+
| Section Title                    [Add New]                   |
+-------------------------------------------------------------+
| Flags | Name     | Col2    | Col3    | Status   | Actions   |
|-------|----------|---------|---------|----------|-----------|
|                                                              |
|            No {items} configured.                            |
|            {One sentence explaining what this is for}        |
|                                                              |
|                       [Add {Item}]                           |
|                                                              |
+-------------------------------------------------------------+
```

The Add button appears both in the toolbar AND centered in the empty area.
The empty message is specific to the page, not a generic "no data."

### Detail Panel Pattern

When a table row is clicked, a detail panel opens (either as a right-side drawer
or expanding below the row, depending on viewport width):

```
+----------------------------------+----------------------------+
| Table (narrowed)                 | Detail Panel               |
|                                  |                            |
|                                  | [Config] [Status] [Actions]|
|                                  |                            |
|                                  | Field: value     [edit]    |
|                                  | Field: value     [edit]    |
|                                  | ...                        |
|                                  |                            |
|                                  | Related tools:             |
|                                  | [Detail] [Stats] [Flush]   |
|                                  |                            |
|                                  | [Save] [Discard] [Close]   |
+----------------------------------+----------------------------+
```

### Form Pattern

Configuration forms for singletons (DNS, SSH, etc.):

```
+-------------------------------------------------------------+
| Section Title                                                |
+-------------------------------------------------------------+
|                                                              |
| Field Label          [value_______________]                  |
| Field Label          [value_______________]                  |
| Toggle Label         [x] Enabled                            |
| List Field           [item1] [item2] [item3] [+Add]         |
|                                                              |
| [Save] [Discard]                                             |
+-------------------------------------------------------------+
```

### Overlay Pattern (for operational tool output)

When a row-level tool (e.g., "Peer Detail") is invoked:

```
+-------------------------------------------------------------+
| Peer Detail: neighbor-1 (10.0.0.1)           [Rerun][Close] |
+-------------------------------------------------------------+
| <pre>                                                        |
| BGP Peer: neighbor-1                                        |
|   State: Established                                        |
|   Remote AS: 65001                                          |
|   ...                                                       |
| </pre>                                                      |
+-------------------------------------------------------------+
```

- The overlay appears on top of (but does not replace) the workspace
- Multiple overlays can be pinned (stacked)
- Each has Rerun and Close buttons
- Dangerous actions (Teardown) show a confirmation step first

---

## Operator Workflows

### Workflow 1: Initial Router Setup (L2 -> L3 -> Routing)

1. **Dashboard** shows empty state with hints
2. Operator navigates to **Interfaces > All Interfaces**
3. Clicks **Add Interface** to create a bridge or configure ethernet
4. Opens the new interface, clicks **Add Unit** to create a logical unit
5. Navigates to **IP > Addresses**
6. Clicks **Add Address**, selects the interface.unit, enters IP/prefix
7. Navigates to **Routing > BGP > Peers**
8. Clicks **Add Peer**, fills in neighbor IP, remote AS, families
9. Checks **Routing > BGP > Summary** to see session establishing

### Workflow 2: Debugging a Down BGP Peer

1. **Dashboard** shows warning: "BGP: 1 peer down"
2. Operator clicks warning or navigates to **Routing > BGP > Peers**
3. Sees the red row for the down peer
4. Clicks the row to open detail
5. Checks **Status tab**: sees "Idle" state, last error "Hold timer expired"
6. Clicks **Statistics** tool: sees message counters stopped
7. Checks **Capabilities** tool: verifies capability mismatch if any
8. Edits config if needed (e.g., changes hold timer)
9. Clicks **Flush** or **Teardown** then waits for re-establishment
10. Clicks **Detail** again to verify state is now Established

### Workflow 3: Investigating Packet Drops

1. Navigates to **Interfaces > Traffic**
2. Sorts by RX Drops (descending)
3. Sees interface with high drop count
4. Clicks interface name to go to detail
5. Checks **Traffic Counters**: sees TX queue drops increasing
6. Navigates to **Firewall > Rules** to check if rules are dropping traffic
7. Checks packet/byte counters on each rule to find active drop rules
8. Navigates to **Firewall > Connections** to check conntrack state

### Workflow 4: Adding a Firewall Rule

1. Navigates to **Firewall > Tables**
2. If no tables: clicks **Add Table**, selects family "inet"
3. Navigates to **Firewall > Chains**
4. Clicks **Add Chain**: name="input", type=filter, hook=input, policy=accept
5. Navigates to **Firewall > Rules**
6. Clicks **Add Rule**: chain=input, match (protocol=tcp, dport=22), action=accept
7. Adds another rule: chain=input, match (state=established,related), action=accept
8. Adds final rule: chain=input, action=drop (catch-all)
9. Reviews rule order, uses **Move Up/Down** to adjust

### Workflow 5: Monitoring System Health

1. Opens **Dashboard > Overview**
2. Glances at all panels: CPU, memory, BGP health, warnings, errors
3. If concerned about hardware: checks **System > Host Hardware**
4. For detailed metrics: uses **Tools > Metrics Query**
5. For live event stream: opens **Logs > Live Log**, filters by namespace

### Workflow 6: Editing a BGP Peer (Change-and-Verify Loop)

1. On **Routing > BGP > Peers**, clicks peer row
2. In detail panel **Config tab**, changes a field (e.g., hold timer)
3. Sees pending-change marker on the row
4. Clicks **Peer Detail** tool to see current runtime state (overlay)
5. Clicks **Commit** in top bar (or commit bar)
6. After commit, clicks **Peer Detail** again to verify new state
7. If unhappy: clicks **Diff** to see what changed, or **Discard** to revert

---

## Implementation Phases

This section outlines the order in which pages should be built. Each phase
delivers usable functionality, not partial scaffolding.

### Phase 1: Foundation + Interfaces

**Deliverables:**
- Working left navigation with expand/collapse sections
- Dashboard > Overview (even if some panels show placeholder data)
- Interfaces > All Interfaces (full table with add/edit/delete/enable/disable)
- Interface detail panel with units sub-table
- IP > Addresses (table with add/edit/delete)
- Empty state pattern implemented
- Table pattern implemented (toolbar, search, actions)

**Why first:** Interfaces and addresses are the foundation. Every router needs
them. This proves the table pattern and detail panel pattern work.

### Phase 2: BGP (the critical workflow)

**Deliverables:**
- Routing > BGP > Peers (full table with status columns)
- Peer detail panel with Config/Status/Actions tabs
- Row-level tools: Detail, Capabilities, Statistics, Flush, Teardown
- Tool overlay pattern implemented
- Routing > BGP > Groups (table with member management)
- Routing > BGP > Summary (read-only health view)
- Pending change markers on rows
- Change-and-verify loop working end to end

**Why second:** BGP is the primary use case for Ze. This proves the operational
workflow (edit, commit, verify) and the tool overlay pattern.

### Phase 3: Firewall

**Deliverables:**
- Firewall > Tables, Chains, Rules, Sets (all four pages)
- Ordered rule management (move up/down)
- Rule counters display
- Firewall > Connections (operational view)

**Why third:** Firewall is the second most important feature after routing.
It has unique requirements (ordered rules, counters) that extend the table pattern.

### Phase 4: System + Services + L2TP

**Deliverables:**
- All System pages (Identity, Users, Resources, Hardware, Sysctl)
- All Services pages (SSH, Web, Telemetry, TACACS, MCP, LG, API)
- L2TP pages (Sessions, Configuration, Health)
- Dashboard > Health (aggregated from all components)

### Phase 5: Tools + Logs + Polish

**Deliverables:**
- All Tools pages (Ping, BGP Decode, Metrics Query, Capture)
- All Logs pages (Live Log with SSE, Warnings, Errors)
- Dashboard > Recent Events
- Policy pages (Filters, Communities, Prefix Lists)
- Routing > Redistribute
- Search across all config objects
- Column sorting on all tables

---

## What the Previous Implementation Got Wrong

For the record, so these mistakes are not repeated:

1. **Navigation links to YANG tree paths.** `/show/bgp/` is a schema browser, not
   an operator page. Every nav link must go to a purpose-built page.

2. **No actual pages.** The workbench rendered the same `detail` fragment as the
   Finder. There were no tables, no forms, no empty states, no actions.

3. **Mixed old and new UI.** Some links went to the new workbench shell, others
   went to the old Finder. This is confusing and unusable.

4. **No empty state handling.** A blank page with no data and no way to add data
   is useless to an operator.

5. **No consideration of user workflows.** The navigation was a flat list of
   categories with no thought about what an operator actually does.

6. **No operational data.** BGP peer tables without session state, interface lists
   without link status or traffic, are not useful.

7. **No actions.** Every configurable thing needs an Add button. Every existing
   thing needs Edit and Delete. Operational objects need their relevant commands.

---

## Open Questions for Review

1. **Dashboard widgets:** Should the dashboard panels be customizable/rearrangeable,
   or is a fixed layout sufficient for v1?

2. **Interface types:** Ze supports ethernet, bridge, dummy, veth, tunnel (GRE, IPIP,
   sit, WireGuard). Should each get its own nav entry, or is filtering the All
   Interfaces table sufficient?

3. **Policy editor complexity:** BGP filter/policy editing can be arbitrarily complex
   (match conditions, actions, chaining). How much visual tooling is needed vs. just
   providing a structured form?

4. **Traffic graphs:** RouterOS has `/graphs/` with RRD-style PNG graphs. Should Ze
   have similar historical graphs, or is the real-time counter display sufficient?

5. **Mobile support:** RouterOS WebFig barely works on mobile. Should Ze's workbench
   target tablet at minimum, or is desktop-only acceptable?

6. **Keyboard navigation:** Should the table support keyboard navigation (arrow keys,
   Enter to open, shortcuts for common actions)?

7. **Multi-select operations:** Should tables support selecting multiple rows for
   bulk enable/disable/delete?
