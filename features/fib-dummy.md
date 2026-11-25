# FIB Dummy Backend

## Meta

| Field | Value |
|-------|-------|
| Name | FIB Dummy Backend |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/fib/dummy |
| Real-path tests | test/plugin/fib-dummy.ci, test/plugin/fib-dummy-forked.ci, test/plugin/fib-dummy-labels-refused.ci, test/plugin/fib-dummy-doctor.ci, test/plugin/perf-route-path-dut.ci, test/parse/fib-dummy-beside-writer-refused.ci, test/parse/fib-dummy-beside-cli-plugin-refused.ci, test/reload/fib-dummy-beside-writer-reload-refused.ci, test/reload/fib-dummy-reload-add.ci, test/reload/fib-dummy-reload-remove.ci, test/reload/static-removed-stops-engine-added-writer.ci, test/reload/fib-internal-endpoint-replaced-by-dummy.ci, test/static/static-fib-dummy-holds-route.ci, test/vpp/fib-dummy-vpp-backend.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-11-24: verified against internal/plugins/fib/dummy, docs/guide/configuration.md, docs/guide/command-reference.md, docs/guide/plugins.md, and docs/plugin-development/metrics.md. All source anchors resolve and match the implemented commands, counters, metrics, doctor codes, and scope and conversion contracts |
| Defect review | 2026-11-24: no open plan/release/fixes spec or journal row names internal/plugins/fib/dummy |

## Description

`fib { dummy { } }` loads the `fib-dummy` plugin.
The plugin declares `ProgramsFIB` and `NoDataPlaneChoice` with no `DataPlane`.
Its YANG container carries no leaf.

Because `fib dummy` is the operator's FIB choice, Ze adds no data-plane writer beside it.
Ze commits it under either `interface backend kernel` or `interface backend vpp`.
Ze refuses it beside any other `ProgramsFIB` plugin (config block, bus endpoint, or `--plugin`) under `doctor-dataplane-mismatch`.

`fib-dummy` subscribes to `(system-rib, best-change)` and requests a system RIB replay at startup.
It holds each selected route in memory keyed by `(table, prefix)`.
It counts installs, replaces, removals, and invalid changes, and writes no route to any forwarding table.

`show fib dummy` answers a table with `table`, `prefix`, `protocol`, `route-type`, `next-hop`, `interface`, `paths`, and `programmed` (`no`).
`show fib dummy counters` answers `forwarding` (`none`), `routes-held`, `installs`, `replaces`, `removals`, `invalid-changes`, `label-entries-refused`, and `since`.

`fib-dummy` subscribes to `(mpls-fib, entry)` and requests a targeted label replay at startup.
It refuses every addressed label batch so LDP, RSVP-TE, OSPF Segment Routing, and BGP EPE never advertise unforwarded labels.
Each label source reports that refusal under `doctor-fib-mpls-no-writer`.

The plugin exports `ze_fibdummy_routes_held`, `ze_fibdummy_route_installs_total`, `ze_fibdummy_route_replaces_total`, `ze_fibdummy_route_removals_total`, and `ze_fibdummy_label_entries_refused_total`.
It registers no `PreservesForwardingState` answer, so BGP Graceful Restart leaves its Forwarding State bit clear.
`ze doctor` reports `doctor-fib-dummy-holds-routes` (warning with a `NeedsDataPlane` producer, info otherwise).
Runtime raises `doctor-fib-dummy-invalid-change` once on an invalid prefix.

This contract covers routes selected by the system RIB only.
Named-table static routes, policy routing, the IPsec tunnel-source guard, and interface addresses write forwarding state by their own paths and stay untouched.
If you replace a running `fib kernel` writer whose `flush-on-stop` is `false`, its kernel routes stay installed until you commit `fib kernel flush-on-stop true` first or flush them manually. <!-- source: internal/plugins/fib/dummy/register.go -- init, commandDecls, SetMetricsRegistry --> <!-- source: internal/plugins/fib/dummy/fibdummy.go -- dummyFIB, applyBatch --> <!-- source: internal/plugins/fib/dummy/show.go -- routeRow, countersSnapshot, routeRows, counters --> <!-- source: internal/plugins/fib/dummy/labels.go -- handleMPLSEntry, subscribeAndRequestLabels --> <!-- source: internal/plugins/fib/dummy/doctor.go -- checkFIBDummyHoldsRoutes --> <!-- source: internal/plugins/fib/dummy/register_doctor_codes.go -- holdsRoutesCodeMeta, invalidChangeCodeMeta --> <!-- source: internal/plugins/fib/dummy/yang/ze-fib-dummy-conf.yang -- dummy container --> <!-- source: internal/component/config/dataplane_verdict.go -- evaluateNoPlaneFIBChoice -->
