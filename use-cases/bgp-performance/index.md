# BGP performance testing with Ze

A route-server performance test is simple in outline: create a route source, connect it to a device under test, watch a monitor peer, and publish how many routes arrived and how long convergence took.

AMS-IX used ExaBGP as a lightweight BGP route source. bgperf formalised the same idea with tester, target, and monitor containers. Ze can run that setup with `ze-perf`: Ze supplies the sender and receiver, the target is any BGP speaker you want to test, and the output is a repeatable report instead of a stopwatch and screenshots.

This page shows Ze in the tester role. The device under test can be FRR, BIRD, GoBGP, OpenBGPD, a route-server appliance, or Ze itself.

<!-- source: ../main/internal/perf/cli/cmd_run.go -- ze-perf sender, receiver, repeat, JSON, and output flags -->
<!-- source: ../main/internal/perf/result.go -- result fields -->
<!-- source: ../main/internal/test/perfrunner/run.go -- Runner.RunCLI -->

## What a good test answers

A performance test has to answer these questions and publish the evidence for each.

| Question | Evidence to publish |
| --- | --- |
| Can the DUT establish the sessions? | Sender and receiver BGP sessions reach Established |
| Can the DUT ingest the route set? | Expected route count arrives at the receiver |
| Did it lose routes? | `routes-lost` is zero |
| How long did convergence take? | `convergence-ms`, throughput, and latency percentiles from `ze-perf` |
| Does the result repeat? | Several iterations, warmups discarded, standard deviation reported |
| Can someone reproduce it? | DUT config, route count, family, batch size, hardware, runtime, and raw JSON |

A good run proves a complete path: generated routes enter the DUT, pass through the DUT policy and RIB path, leave the DUT, and arrive at a real BGP receiver. A route count alone is wiring. A repeated run with zero loss and published inputs is evidence.

## Topology

The test is three nodes: a sender and a receiver that Ze runs, with the device under test between them.

```
ze-perf sender  --->  DUT under test  --->  ze-perf receiver
AS 65001             AS 65000             AS 65002
```

| Node | AS | Played by | Job |
| --- | --- | --- | --- |
| Sender | 65001 | `ze-perf` | Generate the route set and inject it into the DUT |
| Device under test | 65000 | FRR, BIRD, GoBGP, OpenBGPD, Ze, or another BGP speaker | Receive the routes, apply policy, and re-advertise them |
| Receiver | 65002 | `ze-perf` | Record when each prefix arrives |

`ze-perf` matches the prefixes the sender injected against the ones the receiver saw, so it can report convergence, throughput, latency percentiles, withdrawal time, and lost routes. In bgperf terms, the sender is the tester, the DUT is the target, and the receiver is the monitor.

## Fastest working test

Use the native Docker runner from the main Ze checkout when you want the whole setup built for you.

```text
go build -o bin/ze-perf ./cmd/ze-perf
go run ./cmd/ze-perf-run --build --test frr
bin/ze-perf report test/perf/results/*.json
```

Run a smaller wiring test first when you are adding a new DUT target.

```text
ze-perf run \
  --dut-addr 172.31.0.2 \
  --dut-asn 65000 \
  --dut-name frr \
  --routes 10 \
  --repeat 1 \
  --warmup-runs 0
```

If 10 routes do not converge, fix the BGP sessions or DUT policy before running a scale test.

## A 100k route run

This is the Ze equivalent of using ExaBGP as the injector in the original route-server tests.

```text
ze-perf run \
  --dut-addr 172.31.0.2 \
  --dut-asn 65000 \
  --dut-name frr \
  --sender-addr 172.31.0.10 \
  --sender-asn 65001 \
  --receiver-addr 172.31.0.11 \
  --receiver-asn 65002 \
  --routes 100000 \
  --repeat 10 \
  --warmup-runs 2 \
  --iter-delay 3s \
  --output results/frr-100k.json
```

The sender and receiver addresses must be valid local addresses in the lab where `ze-perf` runs. The Docker runner handles that wiring. In a manual lab, bind those addresses on the host or run `ze-perf` in the right network namespace.

Then render the report.

```text
ze-perf report results/frr-100k.json
```

Use `--family ipv6/unicast` for IPv6. Use `--force-mp` to encode IPv4 NLRI through MP_REACH_NLRI and test the DUT multiprotocol path. Use `--batch-size 1` when you want one prefix per UPDATE instead of packed UPDATEs.

## FRR as the DUT

This FRR config accepts the Ze sender and exports the received routes to the Ze receiver.

```text
frr defaults traditional
hostname dut-frr
log stdout informational
!
route-map ACCEPT permit 10
!
router bgp 65000
 bgp router-id 172.31.0.2
 no bgp ebgp-requires-policy
 neighbor 172.31.0.10 remote-as 65001
 neighbor 172.31.0.11 remote-as 65002
 !
 address-family ipv4 unicast
  neighbor 172.31.0.10 activate
  neighbor 172.31.0.10 route-map ACCEPT in
  neighbor 172.31.0.11 activate
  neighbor 172.31.0.11 route-map ACCEPT out
 exit-address-family
!
```

Useful checks while debugging:

```text
show bgp ipv4 unicast summary
show bgp ipv4 unicast 10.0.0.0/24
```

`PfxRcd` on the sender neighbor should reach the generated route count. The receiver side should receive the same count.

## BIRD as the DUT

This BIRD 2 config imports routes from the Ze sender and exports them to the Ze receiver.

```text
log stderr all;
router id 172.31.0.2;

protocol device {
}

protocol bgp ze_sender {
    local 172.31.0.2 as 65000;
    neighbor 172.31.0.10 as 65001;
    ipv4 {
        import all;
        export none;
    };
}

protocol bgp ze_receiver {
    local 172.31.0.2 as 65000;
    neighbor 172.31.0.11 as 65002;
    ipv4 {
        import none;
        export all;
    };
}
```

Useful checks:

```text
birdc show protocols
birdc show route count
```

## What Ze reports

`ze-perf` reports the fields that are easy to lose when the test is driven by an external injector.

| Field | Why it matters |
| --- | --- |
| `convergence-ms` | Time from the sender writing the first UPDATE to the receiver collecting the expected routes |
| `throughput-rps` | Sustained route propagation rate over the convergence window |
| `latency-p50-ms`, `latency-p90-ms`, `latency-p99-ms`, `latency-max-ms` | Per-route propagation distribution |
| `withdrawal-ms` | Time for the receiver to become idle after withdrawal |
| `routes-lost` | Route loss, which invalidates a throughput claim until fixed |
| `repeat`, `warmup-runs`, `repeat-kept` | How much measurement stability the run has |
| `seed`, `family`, `force-mp`, `batch-size` | Inputs needed to reproduce the route stream |

Use JSON for anything you intend to publish.

```text
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --routes 100000 --output result.json
```

## What to publish with the number

Publish the method with the result.

| Field | Why it matters |
| --- | --- |
| DUT name, version, commit, and config | Policy and defaults change results |
| Hardware or VM shape | CPU, memory, and virtualization dominate control-plane tests |
| Runtime | Bare metal, Docker, Colima, QEMU, and VM networking are different tests |
| Route source | Synthetic routes, MRT replay, and production captures stress different code paths |
| Peer shape | One large peer and many small peers find different bugs |
| Route count and family | IPv4 inline NLRI, IPv4 MP_REACH, and IPv6 MP_REACH exercise different paths |
| Batch size | Packed UPDATEs and one prefix per UPDATE are different workloads |
| Repeats and warmups | One run is a smoke test |
| Lost routes | Loss must be explained before throughput matters |
| Raw JSON | The number can be checked later |

## How close Ze is to the full route-server test

Ze already covers the core of the test: sender, DUT, receiver, route count, convergence, loss, and repeatable output.

The current `ze-perf` setup is one sender session and one receiver session. That is enough to test route propagation and DUT table pressure, but not hundreds of independent BGP client sessions in one run.

For a full AMS-IX or bgperf-style route-server scale test, Ze would need a multi-peer mode:

| Missing shape | Feature to add |
| --- | --- |
| 500 small route-server clients | N sender sessions, each with its own local address, ASN, and route slice |
| MRT replay as route source | Route input from MRT, with `count` and `skip` controls |
| External generator with Ze monitor | Receiver-only mode that records first route, last route, loss, and receive window |
| Per-client reporting | JSON grouped by sender peer and route family |

That would let Ze reproduce the full route-server topology while keeping Ze's timing and JSON reporting.

## Prior art

AMS-IX published route-server performance slides at RIPE64 with 500 IPv4 peers, 128,000 prefixes, BIRD table-mode comparisons, an MTU test with 1012 IPv6 peers, and a note to check out ExaBGP as a lightweight tool to open BGP sessions and inject prefixes: [AMS-IX Route-Server Performance Test](https://ripe64.ripe.net/presentations/49-Follow_Up_AMS-IX_route-server_test_Euro-IX_20th_RIPE64.pdf).

Job Snijders' `bgp-performance` repository is a bgperf2-based performance suite. Its docs describe a target, monitor, and tester topology, and its MRT mode can use ExaBGP as an MRT injector: [job/bgp-performance](https://github.com/job/bgp-performance), [how bgperf works](https://github.com/job/bgp-performance/blob/master/docs/how_bgperf_works.md), and [MRT injection](https://github.com/job/bgp-performance/blob/master/docs/mrt.md).

## Lab evidence

Ze's automated performance evidence already wires sender, DUT, and receiver the same way.

| File | What it proves |
| --- | --- |
| `internal/perf/cli/cmd_run.go` | `ze-perf run` accepts DUT, sender, receiver, repeat, warmup, JSON, output, family, MP, and batch flags |
| `internal/perf/result.go` | JSON results include convergence, throughput, latency percentiles, withdrawal time, lost routes, seed, family, and repeat metadata |
| `internal/test/perfrunner/run.go` | The native Docker runner benchmarks Ze, FRR, BIRD, GoBGP, rustbgpd, RustyBGP, freeRtr, and OpenBGPD |
| `test/perf/configs/` | DUT configs are stored with the test runner rather than hidden in a report |

Run the Ze performance suite from the main checkout:

```text
go run ./cmd/ze-perf-run --build --test frr bird gobgp
bin/ze-perf report test/perf/results/*.json
```

The full benchmarking guide is at [docs/guide/benchmarking](../../guides/benchmarking/).
