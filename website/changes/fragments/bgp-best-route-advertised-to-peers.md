---
spec: bgp-best-route-advertised-to-peers
topic: BGP
feature: bgp-protocol
---

## Before

Passing a learned route from one BGP peer to another required a forwarding
plugin such as `bgp-rs`. The `bgp-rib` process stored routes and selected a
best path for the routing table.

## After

Ze now advertises selected routes between ordinary peers through `bgp-rib` on
the message bus. By default `bgp-rib` receives `bgp:update-received`,
`bgp:update-sent`, `bgp:state`, and `bgp:refresh` and sends `bgp:update` across
BGP peers; Ze advertises a learned route when the destination's configured
export policy accepts it. Config validation refuses a `bus` override that
removes a required relationship and names the peer, endpoint, and completing
line.

<!-- source: internal/component/plugin/server/dissemination_validate.go -- validateDisseminationProcess -->
<!-- source: internal/component/bgp/reactor/dissemination_snapshot.go -- ExportKind -->
<!-- source: internal/component/bgp/filterapi/export_kind.go -- UnconfiguredExportAccepts -->

When the selected route changes, Ze replaces or withdraws the advertisement.
The recipient receives the current exportable selection when its session
reaches Established or it requests a route refresh.

<!-- source: internal/component/bgp/plugins/rib/rib_advertise_capture.go -- commitAdvertise, removeAdvertise -->
<!-- source: internal/component/bgp/plugins/rib/rib_ordinary_destination.go -- updateSelectedDestination -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_refresh.go -- requestRefresh -->

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
