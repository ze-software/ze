Benchmarks

# Performance.

Measured, not claimed.

Benchmark numbers need context: these numbers measure one scenario, on one machine, under artificial conditions. They show relative differences between implementations, not absolute performance. If it matters to you, run `le perf` on your own hardware with your own workload.

`BGP benchmark`

`le perf` establishes two BGP sessions with a device under test: a sender injects 100,000 routes, a receiver times their arrival. The same harness runs unmodified against BIRD, FRR, GoBGP, RustyBGP, freeRtr, and (in earlier runs) rustbgpd and OpenBGPD, all in Docker on the same host.

Go carries an estimated 10-15% CPU overhead against C/Rust implementations. That number comes from code-path analysis, not a measured benchmark -- Ze has not been benchmarked at DFZ scale (1M+ prefixes).

- **Convergence:** 62ms to propagate 100,000 routes (2026-06-05 run)
- **Throughput:** 1,612,903 routes/sec sustained during propagation
- **Withdrawal:** 596ms from withdrawal sent to receiver idle
- **Full results:** [All DUTs, all runs, full methodology](https://ze-software.net/performance/bgp/)

```
# build all DUT images, then run
$ ./le perf run

# test specific DUTs only
$ ./le perf run dut "ze bird"

# regenerate docs/performance.md from results
$ ./le perf compare --doc test/perf/results/*.json > docs/performance.md
```

`Prerequisites`

Docker (Colima on macOS). `le perf` works against any BGP implementation, not just Ze -- point it at your own DUT.

- [Benchmarking guide architecture, flags, JSON output](https://ze-software.net/guides/benchmarking/)
- [BGP performance tests with Ze sender, receiver, DUT, JSON](https://ze-software.net/use-cases/bgp-performance/)

## Related interop and dataplane labs.

Functional verification alongside the benchmark harness.

### [BGP Protocol Interop](https://ze-software.net/labs/bgp-interop/)

`Daemon` `Docker`

- Same peer implementations as the benchmark: **FRR, BIRD, GoBGP**
- Correctness first; the perf numbers above measure the same sessions

### [VPP Dataplane Evidence](https://ze-software.net/labs/vpp-dataplane/)

`Daemon` `Docker`

- Ze programs **FIB, traffic, and firewall** into a real VPP daemon
- Verifies control-plane programming (throughput is not benchmarked in this Docker lab)

### [VLAN QoS Packet Tests](https://ze-software.net/labs/vlan-qos/)

`Daemon` `AF_PACKET`

- 802.1p **PCP tagging** verified on captured packets
- Checks wire marking correctness (scheduling and throughput remain untested here)
