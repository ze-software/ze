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

Ze now advertises selected routes between ordinary peers through one explicitly
attached `bgp-rib` process. The source grants `receive [ update-received state ]`.
The destination grants `send [ update ]` and `receive [ state refresh update-sent ]`
to that same process. Ze advertises a learned route when the destination's
configured export policy accepts it. Config validation names the peer and
process and shows the required `attach process` block if grants are incomplete.

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
