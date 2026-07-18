<!-- Design: plan/spec-diag-core.md -- symptom-based diagnostic guide -->

# Production Diagnostics Guide

Symptom-based troubleshooting using Ze's built-in diagnostic commands. `ze doctor`, health checks, warning/error reports, support bundles, crash capture, and runtime probes are part of the product, so operators can start from evidence before reaching for external Linux tools. The commands below also work on gokrazy appliances without a host shell toolbox.

## Quick Reference

| Symptom | First Command |
|---------|--------------|
| BGP session won't establish | `show tcp-check <peer-ip> 179` |
| Path/routing issue | `show traceroute <dest>` or `monitor traceroute <dest>` |
| BGP session flapping | `show system kernel-log level warning count 50` |
| High CPU | `show system profile cpu duration 10s` |
| Memory leak | `show system profile heap` |
| FD exhaustion | `show system file-descriptors summary` |
| Goroutine leak | `show system goroutines summary` |
| DNS failure | `show dns lookup <name> type A` |
| Latency/reachability | `monitor ping <target>` |
| Process killed | `show system kernel-log level err` |
| Route/link/addr changes | `monitor system netlink all` |
| Packet-level debugging | `show capture interface eth0 tcp port 179 count 10 format text` |

## Failure Categories

### 1. BGP Session Won't Establish

**Verify TCP connectivity:**

```
show tcp-check <peer-ip> 179
```

If "refused": peer is not listening. If "timeout": firewall or routing issue.

**Trace the path to the peer:**

```
show traceroute <peer-ip>
```

If hops stop before the peer, there is a routing or firewall issue at that hop.

**Check sockets for existing connections:**

```
show system sockets tcp state ESTABLISHED port 179
```

**Check DNS resolution of peer hostname:**

```
show dns lookup <peer-hostname>
```

**Inspect raw BGP messages:**

```
show capture raw start bgp
# Wait for connection attempt, then:
show capture raw dump bgp
```

### 2. BGP Session Flapping

**Check kernel log for link events:**

```
show system kernel-log level warning count 50
```

**Stream live link/route events to correlate with flaps:**

```
monitor system netlink all
```

**Check socket state churn:**

```
show system sockets tcp port 179
```

**Inspect raw packets during flap:**

```
show capture raw start bgp
show capture raw dump bgp pcap
```

**Live packet capture on the interface (replaces tcpdump):**

```
show capture interface eth0 tcp port 179 count 20 format text
show capture interface eth0 tcp port 179 duration 10s format pcap
```

### 3. BGP Routes Not Received

```
show bgp peer <selector> detail
show capture raw start bgp
```

Check UPDATE messages in the capture for the expected NLRI.

### 4. BGP Routes Not Advertised

```
show bgp peer <selector> detail
```

Check advertised route counts, filter configuration, and export policy.

### 5. High CPU Usage

**Capture CPU profile:**

```
show system profile cpu duration 10s
```

Decode the base64 output with `go tool pprof`.

**Check goroutine distribution:**

```
show system goroutines summary
```

A large count in one state (e.g., "running") suggests a hot loop.

**Check system metrics:**

```
show system cpu
```

### 6. Memory Leak / High Memory

**Capture heap profile:**

```
show system profile heap
```

**Check process memory from the kernel's perspective:**

```
show system memory
```

Compare VmRSS with Go's heap-in-use from `show runtime memory`:

```
show runtime memory
```

If VmRSS is much larger than heap-in-use, memory is held outside Go's heap (cgo, mmap).

**Check goroutines for leaks:**

```
show system goroutines summary
show system goroutines blocked
```

### 7. File Descriptor Exhaustion

**Check FD usage and limits:**

```
show system file-descriptors summary
```

If total is close to soft-limit, the process is near exhaustion.

**Inspect individual FDs:**

```
show system file-descriptors detail
```

Look for unexpected socket or pipe accumulation.

**Cross-reference with sockets:**

```
show system sockets
```

### 8. Goroutine Leak

**Get current count and distribution:**

```
show system goroutines summary
```

Normal count depends on configuration. A steadily increasing count indicates a leak.

**Find blocked goroutines:**

```
show system goroutines blocked
```

Large numbers stuck in "chan receive" or "select" with the same stack suggest leaked goroutines.

**Full stack dump for analysis:**

```
show system goroutines full
```

### 9. Process Killed (OOM / Signal)

**Check kernel log for OOM killer or signal events:**

```
show system kernel-log level err
```

Look for "Out of memory" or "Killed process" messages.

**Check current memory state:**

```
show system memory
```

**Review warnings and errors:**

```
show warnings
show errors
```

### 10. Interface Down / Link Flap

```
show interface
show system kernel-log level warning
monitor system netlink link
```

`monitor system netlink link` streams live link state changes (up/down/create/delete). Press Esc to stop.

### 11. Kernel Route Missing

```
show route
show system sockets
monitor system netlink route
```

`monitor system netlink route` streams live kernel route changes to observe additions and deletions in real time.

### 12. DNS Resolution Failure

**Test resolution directly:**

```
show dns lookup <hostname> type A
```

**Check cache state:**

```
show dns cache stats
show dns cache list
```

High miss rate with low hit rate suggests upstream resolver issues. `list`
shows all cached entries with remaining TTL, useful for spotting stale or
unexpectedly short-lived records.

**Inspect a specific cached name:**

```
show dns cache record <hostname>
```

**Flush and start fresh:**

```
clear dns cache
```

**Verify socket connectivity to resolver:**

```
show tcp-check <dns-server> 53
```

### 13. Config Commit Failure

```
show errors
show config diff
```

### 14. Plugin Crash / Restart Loop

```
show errors count 20
show warnings
show system goroutines summary
show system kernel-log level err count 20
```

### 15. CLI / SSH Unresponsive

**If the daemon is reachable via another path (web, MCP):**

```
show system goroutines blocked
show system sockets
show system file-descriptors summary
```

Look for goroutines stuck in "semacquire" or "IO wait".

### 16. Web UI Unreachable

```
show tcp-check <router-ip> 3443
show system sockets tcp port 3443
show system goroutines summary
show errors
```

### 17. Telemetry / Metrics Gaps

```
show metrics name ze_peer_state
show system sockets
show system profile cpu duration 5s
```

### 18. Latency and Reachability

**Continuous ping to measure latency and loss:**

```
monitor ping <target>
monitor ping <target> interval 500ms
```

Shows live stats: Sent, Recv, Loss%, Last, Min, Avg, Max, StDev. Use `| log` for scrollback output suitable for correlation with other events.

**Continuous traceroute to observe path changes:**

```
monitor traceroute <target>
monitor traceroute <target> | log | origin
```

mtr-style display with per-hop loss and latency statistics. `| log | origin`
appends one line per round and annotates hops with ASN names, useful for
identifying which network a path change occurs in. `| log | resolve` adds
reverse DNS hostnames instead.

### Demo: Trace a live path without external services

Run Ze's live traceroute through a deterministic Linux network-namespace lab.

[Play the WebM recording](../../../assets/demos/traceroute.webm?v=bda9d06121) · [View the poster](../../../assets/demos/traceroute.png?v=ebcd3e247b) · [Plain-text transcript](../../../assets/demos/traceroute.txt?v=2b9d95cc69)

Recorded with Ze 26.07.18 in a Linux namespace lab using VHS 0.11.0. Duration: 1 minute 2 seconds.

```console
$ ssh ze-demo
ze# run show traceroute 192.0.2.53
ze# run monitor traceroute 192.0.2.53

The destination and router live in an isolated Linux network-namespace lab. Ze sends real ICMP probes, then shows the same path as a one-shot trace and as a continuously refreshed loss and latency table. No public DNS or Internet route is used.
```


## Profiling Workflow

### CPU Profile

```
show system profile cpu duration 10s
```

Save the base64 output, decode, and analyze:

```bash
echo '<base64-data>' | base64 -d > cpu.pprof
go tool pprof cpu.pprof
```

### Heap Profile

```
show system profile heap
```

Same workflow: decode base64, analyze with `go tool pprof`.

### Concurrent Profiling

CPU profiling is mutex-protected. A second concurrent request returns an error. This prevents resource contention from overlapping profiles.

## Platform Detection

`show system platform` reports the runtime platform type and capability flags:

```
show system platform
show system platform | json
```

Detected platforms: **gokrazy**, **systemd**, **container**, **plain-linux**, **darwin**.

Capability flags: `read-only-root`, `perm-available`, `systemd-available`, `gokrazy-update-socket`, `gokrazy-ui-available`, `reboot-allowed`, `persistent-storage-writable`, `fd-limit-soft-current`, `fd-limit-hard-max`, `fd-limit-raisable`.

Platform information is also included in `ze doctor` checks (e.g. gokrazy /perm writability) and `ze support` archives (as the `platform` module).

## Platform Notes

Commands that read `/proc` (sockets, kernel-log, file-descriptors, memory-map) are Linux-only. On other platforms they return "not available on this platform". The remaining commands (tcp-check, traceroute, goroutines, dns, profile) work on all platforms but traceroute requires CAP_NET_RAW.
