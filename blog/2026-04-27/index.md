# Week of 2026-04-27

Interface configuration now rolls back cleanly on failure, the web UI got a dedicated CLI page, and traffic-control state survives more edge cases.

## 🔌 Interfaces & routing

Interface configuration apply is now transactional: if any step (address changes, bridge ports, STP, mirrors, MTU, MAC, VLANs, tunnels) fails partway through, everything already applied gets rolled back instead of leaving a half-configured interface. Sysrib now accepts withdrawal batches that don't carry a specific protocol source, needed for a clean full Loc-RIB teardown, and HTB traffic shaping now uses correct kernel defaults.

## 🖥️ Web UI

The bottom CLI bar was replaced with a dedicated `/cli` page that matches the SSH CLI layout, with the same tab completion, history, and prompt behavior. Workbench, Finder, and CLI are all switchable at runtime from the top bar, with the choice remembered, and login now redirects back to the page you originally requested.

## 📶 Traffic control & telemetry

The netlink traffic-control backend now snapshots and restores the original qdisc instead of leaving it permanently replaced. Prometheus telemetry config was restructured: OS collector settings moved into their own container, HTTP Basic Auth is now available for the metrics endpoint, and a timing side-channel in the auth check was closed.

## 🛠️ Under the hood

REST/gRPC API streaming endpoints are now fully wired end to end, with authorization, accounting, and panic recovery. FIB startup no longer clobbers routes owned by static routing. L2TP route redistribution now emits real add/remove events instead of a stub.
