# fib-vpp route ownership and the stale IP route sweep

How `fib-vpp` tells the IP routes it installed in VPP from every other
owner's, and how a writer that opens after a Ze restart removes the ones a
previous run left. The label table has its own sweep, in
`docs/architecture/mpls/mpls-kernel.md`, "Label routes a previous run left in
VPP".

## The mark: a VPP FIB source of fib-vpp's own

A kernel route carries `rtm_protocol`, and `fib-kernel` owns the routes marked
250 (`internal/core/rtproto`), which is how its `startupSweep` finds them. A
VPP route carries the FIB source that added it instead. VPP's
`ip_route_add_del` (v1) always adds under `FIB_SOURCE_API`, which every
binary-API client shares, Ze's static routes on VPP included, so that source
proves nothing about who installed a route.

`fib-vpp` therefore installs every IP route, labeled pushes included, under a
source of its own named `ze-fib-vpp`:

| Step | VPP API | What VPP does |
|------|---------|---------------|
| Find the source | `fib_source_dump` | Lists every source with its id, priority and name |
| Register it when absent | `fib_source_add` (priority `0x80`) | `fib_source_allocate` gives a new id, even for a name it holds, so the writer looks the name up first: every Ze run against one VPP process shares the source |
| Install or remove a route | `ip_route_add_del_v2` with `src` set | Adds or removes that source's contribution to the entry; `src` 0 would mean `FIB_SOURCE_API` |
| List the source's routes | `ip_table_dump`, then `ip_route_v2_dump` per table with `src` set | Walks the table with `fib_table_walk_w_src`, so only entries the source contributes to are listed |

The priority is `FIB_SOURCE_API`'s (`foreach_fib_source` in VPP's
`fib_source.h`), so fib-vpp's routes keep the preference they had against
interface, DHCP and adjacency routes. A delete names the source, so it takes
away only fib-vpp's contribution: an interface, DHCP, CLI or other API
client's route for the same prefix stays in VPP.

A writer that VPP gives no source does not open: it could mark no route as its
own, so it installs none, and `show fib vpp` answers the cause.

## The sweep

VPP keeps its FIB when Ze stops, and `fib-vpp` does not flush on stop, so the
routes a previous writer installed keep forwarding after a restart. The writer
removes them with `fib-kernel`'s stale-mark-then-sweep:

1. When it opens, before it installs anything or asks sysrib and the label
   sources for a replay, the writer lists every route under its source in
   every IPv4 and IPv6 table. All of them are a previous run's.
2. sysrib and the label sources re-announce what they want. A route this run
   adds or removes, from the replay or otherwise, is this run's from then on,
   and the sweep never touches it.
3. After 30 seconds (`labelSweepDelay`, fib-kernel's `sweepDelay`) the writer
   removes each listed route nobody re-announced.

When a table cannot be read, nothing is marked: a partial list would let the
sweep remove part of what a previous run left and report the rest as gone.

## Doctor

The runtime check `fib-vpp-stale-routes`, answered from `fib-vpp`'s own
process, reports under `doctor-fib-vpp-stale-route` (error):

| Finding | Fix |
|---------|-----|
| A route a previous run left that VPP refused to remove, with VPP's answer, until this run installs or removes the prefix | `ze signal restart` |
| IP tables the writer could not read when it opened | `ze signal restart` |
| A writer that could not open because VPP gave it no FIB source | a VPP release serving `fib_source_add` and `ip_route_add_del_v2`, then `ze signal restart` |

It lists at most 32 routes, one finding each, and counts the rest in one more
finding. While no writer is open for any other reason it answers no finding:
VPP being unreachable is the vpp component's own report.

<!-- source: internal/plugins/fib/vpp/routesource.go -- fibSourceName, fibSourcePriority, openRouteSource, sendRoute, markStale, sweep, sweepStaleRoutes -->
<!-- source: internal/plugins/fib/vpp/register.go -- openFibVPPWriter, answerDoctorCheck -->
<!-- source: internal/plugins/fib/vpp/doctor_route.go -- staleRoutesCheckDecl, staleRouteDoctorFindings -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- run -->
