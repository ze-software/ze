# Stale ARP in ze-perf Propagation Benchmark

## Summary

The sender-first (propagation) benchmark for the ze DUT fails reliably on its
warmup iteration with a 20-second TCP dial timeout. The root cause is a stale
ARP cache entry in the ze container for the receiver IP address (172.31.0.11)
after the runner container is destroyed and recreated between the standard and
propagation benchmark runs.

## Symptoms

```
ze-perf run | dut=172.31.0.2:179 (AS 65000) | routes=100000 | ...
iteration 1/4 (warmup)
error: iteration 1/4 (warmup): connecting receiver: dialing 172.31.0.2:1791:
  dial tcp 172.31.0.11:0->172.31.0.2:1791: i/o timeout
```

The error appears in both the 2 GB and 4 GB Colima VM runs. It is
deterministic, not intermittent.

## Background: how ze-perf uses two IPs

The runner container is assigned 172.31.0.10 by Docker (the primary IP). A
second address, 172.31.0.11, is added manually inside the container:

```python
docker exec runner ip addr add 172.31.0.11/24 dev eth0
```

Ze is configured with two passive peers, each on its own port:

| Peer           | Remote IP      | Local port |
|----------------|----------------|------------|
| perf-sender    | 172.31.0.10    | 1790       |
| perf-receiver  | 172.31.0.11    | 1791       |

## Sequence of events

### 1. Standard benchmark (succeeds)

`run_perf(dut)` creates a runner container, adds 172.31.0.11, runs the
benchmark. Both sessions work. Ze's kernel learns an ARP entry:

```
172.31.0.11  ->  ba:35:58:68:e4:67   (runner #1's MAC)
```

The `finally` block calls `docker rm -f runner`, destroying the container and
its veth pair. Ze's ARP cache is not flushed.

### 2. Propagation benchmark (fails)

`run_perf(dut, sender_first=True)` creates a new runner container. Docker
assigns the same primary IP (172.31.0.10) and the script adds 172.31.0.11
again. The new container has a different MAC:

```
172.31.0.10  ->  42:9f:43:b2:4f:be   (runner #2's MAC, Docker-managed)
172.31.0.11  ->  42:9f:43:b2:4f:be   (runner #2's MAC, same NIC)
```

But ze's ARP cache still has:

```
172.31.0.11  ->  ba:35:58:68:e4:67   (runner #1's MAC, stale)
```

Docker updates the bridge forwarding table for 172.31.0.10 (it manages that
IP), but has no knowledge of 172.31.0.11 (added manually inside the
container).

### 3. Sender connects (succeeds)

In sender-first mode, ze-perf dials the sender session first from 172.31.0.10
to ze at port 1790. This works because Docker correctly updated ARP for
172.31.0.10.

### 4. Receiver connects (times out)

After sending routes and waiting, ze-perf dials the receiver session from
172.31.0.11 to ze at port 1791. The TCP SYN reaches ze (the bridge forwards it
based on destination MAC, which is ze's own NIC). Ze's kernel sends the SYN-ACK
back to 172.31.0.11, looks up the ARP cache, and sends it to the stale MAC
`ba:35:58:68:e4:67`. The bridge drops the frame because that MAC no longer
exists on any port. The SYN-ACK never arrives. The dial times out after 20
seconds.

## Reproduction

```bash
docker network create --subnet 172.31.0.0/24 test-net
docker run -d --name runner1 --network test-net --ip 172.31.0.10 \
    --cap-add NET_ADMIN alpine:3.21 sleep 3600
docker exec runner1 ip addr add 172.31.0.11/24 dev eth0
docker run -d --name dut --network test-net --ip 172.31.0.2 \
    --cap-add NET_ADMIN alpine:3.21 sleep 3600

# Populate ARP
docker exec dut ping -c1 -W2 172.31.0.11

# Destroy and recreate runner
docker rm -f runner1
docker run -d --name runner2 --network test-net --ip 172.31.0.10 \
    --cap-add NET_ADMIN alpine:3.21 sleep 3600
docker exec runner2 ip addr add 172.31.0.11/24 dev eth0

# Stale ARP: ping from DUT to the manually-added IP fails
docker exec dut ping -c1 -W2 172.31.0.11
# -> 100% packet loss

# Primary IP works fine
docker exec dut ping -c1 -W2 172.31.0.10
# -> 0% packet loss

# Verify: DUT ARP still has old MAC for .11, correct MAC for .10
docker exec dut cat /proc/net/arp
```

## Why only the ze DUT is affected

Other DUTs (bird, frr, gobgp) use port 179 for both peers and do not have
separate sender/receiver ports. Their `sender_port` and `receiver_port` are 0,
so ze-perf connects both sessions to the same port from the same runner IP.
Ze is the only DUT where the receiver IP (172.31.0.11) is used to connect to a
dedicated port (1791), and only in the propagation benchmark which requires
destroying and recreating the runner container.

## Fix options

**Option A: gratuitous ARP from the new runner.** After `ip addr add`, send a
gratuitous ARP to update all neighbors on the subnet. This is the standard
solution for IP mobility in Layer 2 networks.

```python
docker exec runner arping -c 1 -A -I eth0 172.31.0.11
```

`arping` is not in alpine:3.21 by default but is in `iputils`. Alternatively,
use the `ip` tool:

```python
docker exec runner ip neigh flush dev eth0  # not needed on the runner side
```

Actually, the simplest approach: send a single broadcast ping from 172.31.0.11
before doing anything else. This forces the DUT kernel to refresh its ARP entry
via the normal request/reply cycle.

**Option B: flush DUT's ARP entry.** After destroying the runner, flush the
stale entry from the DUT. This requires knowing the DUT container name:

```python
docker exec dut ip neigh flush 172.31.0.11
```

This is fragile because it requires exec into the DUT, and some DUTs (bird,
frr) may not have the `ip` tool.

**Option C: don't destroy the runner between runs.** Keep the runner container
alive across both `run_perf` calls and re-exec ze-perf instead of creating a
fresh container. This avoids the ARP problem entirely, but requires refactoring
`run_perf` to optionally reuse an existing runner.

**Option A is the simplest.** One `arping` or equivalent after `ip addr add` in
`run_perf`, for all DUTs (not just ze), costs nothing and prevents the class of
problems entirely.
