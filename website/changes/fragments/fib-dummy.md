---
spec: fib-dummy
topic: Kernel
feature: fib-dummy
---

## Before

- Running a control-plane-only router (such as a route reflector or route
  server) or exercising the full route path (BGP, protocol RIB, system RIB, and
  FIB delivery) without programming routes into the Linux kernel or VPP had no
  supported configuration block; the only way to suppress automatic loading of
  `fib-kernel` beside a route producer was to configure `fib { p4 { } }`, which
  named a switch backend the box did not have, logged every route, and
  described held routes as programmed.
- Adding a `fib { ... }` block by live reload left an engine-added data-plane
  writer running beside it, removing the only `fib { ... }` block while route
  producers remained left those producers with no writer, removing the last
  route producer left an engine-added writer running, and replacing a
  compiled-in FIB `bus > endpoint` by reload left the old writer running.
- Plugins stopped by an accepted reload stayed in the process manager's
  configured set, so `request plugin restart <name>` accepted and restarted a
  plugin the configuration no longer ran.
- When `fib-kernel` started during a reload after label sources were already
  running, it never requested a targeted `(mpls-fib, replay-request)`. BGP EPE
  only replayed labels already marked installed (never retrying an initial
  cleanup refused before acquisition or previously refused labels), RSVP-TE
  replayed entries whose removal had been refused and did not report live
  apply refusals in `rsvp-te-label-entries`, and live label adds during a
  writer-replacement reload window could retain a stale refusal from the
  stopping writer.
- Benchmarking through `./le perf` only exercised the route-server path
  (`bgp-rs`), with no route-path overlay verifying that every announced and
  withdrawn route reached a FIB consumer and no guard stopping an overlay run
  from entering committed route-server history.

## After

- A new `fib { dummy { } }` block loads `fib-dummy` (pulling in `rib`), which
  receives selected system RIB routes, holds them in memory keyed by
  `(table, prefix)`, counts changes, exports `ze_fibdummy_*` Prometheus
  metrics, and programs no forwarding table (kernel or VPP). `show fib dummy`
  lists held routes with `programmed: "no"`, and `show fib dummy counters`
  reports `forwarding: "none"`, held counts, per-verb totals, invalid changes,
  and refused MPLS label entries. `fib dummy` commits under either
  `interface backend kernel` or `interface backend vpp`, refuses to be
  configured beside any other FIB plugin (config block, bus endpoint, or
  `--plugin`) under `doctor-dataplane-mismatch`, leaves the BGP Graceful
  Restart Forwarding State bit clear, and reports its effect in `ze doctor`
  under `doctor-fib-dummy-holds-routes` (warning with route producers, info
  otherwise). Named-table static routes, policy routing, the IPsec
  tunnel-source guard, and interface addresses are outside the system RIB and
  remain untouched.
- `fib-dummy` refuses every addressed MPLS label batch so LDP, RSVP-TE, OSPF
  Segment Routing, and BGP EPE never advertise unforwarded labels and report
  the refusal under `doctor-fib-mpls-no-writer`. RSVP-TE now records live apply
  refusals as well as replay refusals and drops withdrawn entries from replay
  regardless of writer error.
- Live reloads now judge data-plane writers over the whole candidate
  configuration and plugin selection: adding `fib dummy` stops an engine-added
  writer, removing `fib dummy` while producers remain starts the selected
  plane's writer, removing the last producer stops the engine-added writer
  after withdrawing its routes, and replacing a compiled-in FIB bus endpoint
  stops the displaced writer and starts the candidate's choice. Finalized
  reload removals (and rolled-back reload starts) leave the process manager's
  configured set so `request plugin restart` refuses removed plugins.
- `fib-kernel` now requests a targeted `(mpls-fib, replay-request)` on open,
  BGP EPE replays targeted cleanup and desired labels against the opening
  writer alone, and once a reload stops a displaced FIB writer, surviving
  data-plane writers (`fib-kernel`, `fib-vpp`) trigger a post-stop targeted
  replay to clear reload-window refusals.
- `test/perf/configs-route-path/ze.conf` (selected with `PERF_CONFIGS_DIR`)
  exercises `bgp-rib -> rib -> fib-dummy`; the perf runner checks
  `show fib dummy counters` around each pass to confirm every route reached
  the writer, and `./le perf record history` and `./le perf record evidence`
  refuse overlay results.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
