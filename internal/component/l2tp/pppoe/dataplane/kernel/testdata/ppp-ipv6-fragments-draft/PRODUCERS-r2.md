# Fragment readiness successor

Source-only successor for the two review findings. The original packet and
handoff remain unchanged as composition inputs. No canonical source, old test,
kernel patch, feature, helper program, compiler, formatter, or test was run or
changed. This is not a native result or feature-acceptance claim.

Installation by Main after independent review:

1. Install the original five-root source at its original new canonical target.
2. Install `native_ipv6_fragment_readiness_r2_integration_linux_test.go.txt` as
   `internal/component/l2tp/pppoe/dataplane/kernel/native_ipv6_fragment_readiness_r2_integration_linux_test.go`.
3. Apply `r2-graft.patch` to the new five-root file. It changes no older source
   or test file. The graft is in `apply_patch` input format, not a shell script.

The earlier handoff's address-only and queue-allocation readiness claims are
superseded. Address readiness now polls the exact native local/peer `/128`
pair, rejects DAD failure, waits for tentative state to end, and requires its
local-table local route plus main-table connected-local and peer routes on the
actual device. Independent Ethernet installs the same real local/peer pair
through netlink and passes the same readiness condition before injection.
Original PPP still installs through public `AddAddressP2P` and `Connect`.

Before creating any test device, each root locks its injector OS thread, saves
affinity, chooses one actually permitted CPU, sets and reads back that exact
single-CPU affinity, then sets and reads back zero RPS defaults in its private
network namespace. The RPS sysctl is a prerequisite: an unavailable or refused
read/write fails setup and never counts as ownership RED. Cleanup restores the
private default, restores and reads back the original thread affinity, then
unlocks the thread. A failed affinity restoration keeps the thread locked.

Each owned ingress veth is read back as veth with no attached XDP program or
program ID. Its GRO feature is disabled and read back off. No XDP program,
qdisc, redirect, RFS table or RPS map is installed by these tests. The fresh
device plus zero per-net default and GRO/XDP checks qualify the native path.

After injecting the first fragment, the same pinned injector sends a useful
unfragmented UDP marker through the same endpoint and admitted carrier. The
actual marker must be received before the native request and queue counts are
checked and before rename, retirement, or a crossed-owner fragment is injected.
Thus queue allocation is not used as the insertion acknowledgment. The useful
receive acknowledges the earlier full native IPv6 receive call on the qualified
FIFO path; the following counters require its incomplete queue to remain.
The five ownership assertions, identical mixed-owner UDP bytes/checksum, native
counter assertions, replacement controls and final useful controls remain.

## Primary source pins read in this turn

All files were fetched from `torvalds/linux` at `v7.3` through the GitHub
`fetch_file` reader. Blob SHAs below identify the actual returned files.

| Path | Blob SHA | Deciding producer |
| --- | --- | --- |
| `net/ipv6/addrconf.c` | `c90ee6dd7446cd10064c9fb95484b538b3ffc18c` | `ipv6_add_addr` can expose NODAD addresses before route publication; `inet6_addr_add` publishes the connected prefix; `addrconf_dad_completed` calls `ipv6_ifa_notify`; `__ipv6_ifa_notify` inserts the local and distinct peer routes. |
| `net/core/sysctl_net_core.c` | `eb35da3556f4aa00cecd4582ab94e339d2518506` | `netns_core_table_dup` translates the namespace table's data pointers from `init_net` to the child `net`; `sysctl_core_net_init` registers that table. `rps_default_mask_sysctl` uses this `struct net` and stores `net->core.rps_default_mask`; it does not write the host-global RFS flow table. |
| `net/core/net-sysfs.c` | `352173df757850af75641501dfa1a91585515a5e` | `rx_queue_add_kobject` invokes `rx_queue_default_mask`; that producer applies the new device's own `dev_net` default only when nonempty. Fresh zeroed RX queues with an empty default retain no RPS map or RFS table. |
| `drivers/net/veth.c` | `643b97dc52456df80de10068ccac9af2ada70256` | `veth_setup` selects no-queue transmission. `veth_xmit` selects its NAPI path only with an active receive NAPI pointer. `veth_set_features`, `veth_napi_del_range` and the XDP paths qualify GRO-off/no-XDP as no receive NAPI. Deletion clears the pointer and synchronizes network activity. `veth_forward_skb` then invokes `__netif_rx`. |
| `net/core/dev.c` | `e76762e29360ea87ade0846e4a4f9016dca587d6` | `netif_alloc_rx_queues` zero-allocates the fresh queue array. `get_rps_cpu` returns no steering target when both fresh RX-queue maps are absent. `netif_rx_internal` uses the current CPU in that case. `enqueue_to_backlog` appends at the tail; `process_backlog` drains the process queue from the head and splices later input at the tail. Both veth input and PPP reinjection therefore preserve order on the same CPU. |
| `net/ethtool/ioctl.c` | `4b0bc503f9307880e436d5ee30f75476264a0c6b` | `ethtool_set_one_feature` maps SGRO to the GRO bit, clears wanted GRO and calls feature update; `ethtool_get_one_feature` returns the actual feature bit for GGRO readback. |
| `drivers/net/ppp/pppoe.c` | `bf7414b46a26068d1773ec79055167956dde7183` | `pppoe_rcv` passes the matched real session to `__sk_receive_skb`; bound `pppoe_rcv_core` calls `ppp_input`. |
| `net/core/sock.c` | `e8551df8330ff4b1d662b0d0133e20d1f74eb728` | `__sk_receive_skb` invokes the backlog receiver inline when the socket is not owned by a user syscall. The test has completed PPPoX setup and starts no PPPoX reader, relay or concurrent close before the useful acknowledgment. |
| `drivers/net/ppp/ppp_generic.c` | `1a610a18893ba94b5ff3f69f8e25dcd944ef01a8` | `ppp_input` invokes PPP receive; `ppp_receive_nonmp_frame` places accepted IPv6 on the real PPP device and calls `netif_rx` from that receive call. |
| `net/ipv6/reassembly.c` | `05c51f669754a1a10e944bb7c4433565bcaacb40` | `ipv6_frag_rcv` increments requests before finding/inserting the payload. `ip6_frag_queue` calls `inet_frag_queue_insert`, updates retained meat and memory, then returns incomplete status. The later useful UDP receive must follow that return on the qualified FIFO path. `ip6_frag_reasm` increments completion only after native assembly. |
| `net/ipv6/proc.c` | `813013ca4e750c43ee9b2a437409481d5d0051e6` | `sockstat6_seq_show` exposes hash entries as `FRAG6 inuse`, so that field alone proves allocation, not inserted payload. |
| `net/ipv4/inet_fragment.c` | `b286ee429da8fae9d427057231f3f37cc7b10959` | `inet_frag_alloc` charges queue size and `inet_frag_create` publishes it before the caller inserts payload. This is why the predecessor's queue/request predicate was insufficient. |

The existing Go producer chain was also read: `kernel/factory.go`
`openWithOps` finishes PPPoX setup before exposing the opened session;
`kernel/transport.go` retains that transport only for relay borrowing/close;
the fixture starts no relay and no service reader. The PPP channel and unit,
not the PPPoX transport descriptor, are given to the shared PPP session.

No queue-size or PPP-wrapper allocation cost is guessed. The source-qualified
acknowledgment does not depend on BTF, a debug kernel probe, a test-supplied
owner, feature admission, or reassembly implementation.

Main owes fresh canonical compilation and native execution through its matched
Ze-kernel launcher, after the original traffic control succeeds. The exact
fresh test-binary selector remains:

```sh
/tmp/root-ppp-ipv6.test -test.v -test.count=1 -test.timeout=5m -test.run '^TestNativeIPv6Fragment(Useful|DistinctOwners|RetiredOwner|Rename|OrdinaryEthernet)$'
```

Retained temporary items: this successor companion, graft and producer pins
exist only to compose and review the two readiness fixes with their unchanged
predecessor. Remove the packet once canonical source and Main's retained review
and native evidence supersede it. No other temporary item was created.
