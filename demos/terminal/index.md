# Terminal Demonstrations

These recordings run real Ze commands against isolated local fixtures. The checked-in VHS tapes define every keystroke, pause, and terminal size, so a release can regenerate the videos when Ze changes. The recordings use no public service.

Each demo also appears beside the documentation for the feature it exercises. The transcript below each player provides the same command sequence without requiring video playback.

## Interactive command launcher

### Demo: Discover Ze commands interactively

Use type-ahead filtering and drill-down navigation in Ze's interactive command launcher.

[Play the WebM recording](../../assets/demos/launcher.webm?v=1536c23832) · [View the poster](../../assets/demos/launcher.png?v=7b0e4ac048) · [Plain-text transcript](../../assets/demos/launcher.txt?v=0399dbc59f)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 44 seconds.

```console
$ ze

Type "show" to filter the command launcher, then press Enter to open the show command tree.
Type "traceroute" to find the path diagnostic command.
Press Escape and Left to return, then type "doctor" to find the readiness checker.
Press Escape to move back through the menu and return to the shell.
```


## Live BGP dashboard

### Demo: Operate BGP from the live dashboard

Connect to Ze over SSH, open the live BGP dashboard, sort peers, and inspect one session.

[Play the WebM recording](../../assets/demos/cli-dashboard.webm?v=56620f9bd0) · [View the poster](../../assets/demos/cli-dashboard.png?v=5595195ace) · [Plain-text transcript](../../assets/demos/cli-dashboard.txt?v=86542601eb)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 40 seconds.

```console
$ ssh ze-demo
ze# exit
ze> monitor bgp

The dashboard polls three local BGP sessions. Press "s" to sort by the next column, use the arrow keys to select a peer, and press Enter for live session details. Press Escape to return and "q" to leave the dashboard.
```


## ZeFS and SSH configuration

### Demo: Create ZeFS and commit over SSH

Create the ZeFS database, edit the active configuration through Ze's SSH management plane, and verify the committed setting.

[Play the WebM recording](../../assets/demos/zefs-config.webm?v=9651a7e3df) · [View the poster](../../assets/demos/zefs-config.png?v=9e82166aac) · [Plain-text transcript](../../assets/demos/zefs-config.txt?v=cf435951c8)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 2 minutes 21 seconds.

```console
$ cat "$ZE_INIT_INPUT"
admin
secret123
127.0.0.1
2222
ze-demo
$ ze init < "$ZE_INIT_INPUT"
$ ze config ls
ze.conf
$ ze data check

$ ssh ze-demo show bgp summary

$ ssh ze-demo
ze# set environment cli format default table
ze# show | compare
ze# commit
Session committed
ze# exit
ze# exit
$ ze cli -c 'show bgp summary'
$ ze cli -c 'show bgp summary | text'
$ ze cli -c 'show bgp summary | raw' | head -14
$ ze cli -c 'show bgp summary | raw' | ze pipe text

The five lines answer `ze init`'s prompts in order: username, password, host, port, and name. It reads them from a file here so the recording is reproducible, and it prints nothing when its input is not a terminal, so the file is shown first rather than left as an unexplained redirection. `ze init` creates `database.zefs`. The first BGP summary uses the default text format. The SSH editor commits the format setting back to ZeFS, not to a second flat file, and the same operational command immediately uses the committed default. The last commands show the two ways to override that default, and they are different pipes. `show bgp summary | text` is Ze's own operator, inside the quoted command, and it wins over the committed setting. Then `| raw` on its own shows what every one of these renderings is made from: the payload as the daemon holds it, unrendered. The last command sends that same payload across a real shell pipe, and `ze pipe text` formats it on this side, which is how output captured earlier is formatted later. The command is `ze pipe` rather than `ze format` because the operator language also carries `match`, `count`, `first`, `last` and `resolve`, so `format` would name one clause of it. Every command answers with structured data, so `text`, `table`, `json`, `yaml` and `ndjson` all render the same payload.
```


## Read-only operator access

### Demo: Prove read-only RBAC enforcement

Run an allowed NOC command, then show Ze explicitly refuse a known state-changing command.

[Play the WebM recording](../../assets/demos/rbac.webm?v=189c6c578f) · [View the poster](../../assets/demos/rbac.png?v=83eb6771ce) · [Plain-text transcript](../../assets/demos/rbac.txt?v=939addc51a)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 12 seconds.

```console
$ ze config show rbac.conf system authorization profile read-only
default-action allow
entry 20 {
   action deny
   match clear
}

$ ze config show rbac.conf system authentication user noc profile
profile read-only

$ ze cli --user noc -c 'show version'
version: ze 26.07.18

$ ze cli --user noc -c 'clear interface counters'
error: command restricted by access control

The recording displays the command restriction and the NOC user's profile binding before exercising both paths. The daemon allows `show version`, then rejects the matching `clear` command before execution.
```


## Traceroute in an isolated Linux lab

### Demo: Trace a live path without external services

Run Ze's live traceroute through a deterministic Linux network-namespace lab.

[Play the WebM recording](../../assets/demos/traceroute.webm?v=076e9f586a) · [View the poster](../../assets/demos/traceroute.png?v=8dcfcd9e64) · [Plain-text transcript](../../assets/demos/traceroute.txt?v=2b9d95cc69)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 42 seconds.

```console
$ ssh ze-demo
ze# run show traceroute 192.0.2.53
ze# run monitor traceroute 192.0.2.53

The destination and router live in an isolated Linux network-namespace lab. Ze sends real ICMP probes, then shows the same path as a one-shot trace and as a continuously refreshed loss and latency table. No public DNS or Internet route is used.
```


## Web configuration commit

### Demo: Edit and commit configuration in the browser

Change a YANG-backed setting, review the generated diff, commit the draft, and verify the active value.

[Play the WebM recording](../../assets/demos/web-config.webm?v=a8458dd75a) · [View the poster](../../assets/demos/web-config.png?v=300f2e8e47) · [Plain-text transcript](../../assets/demos/web-config.txt?v=a614767cf2)

Recorded with Ze 26.08.19 on macOS and Linux using Playwright 1.55.0. Duration: 58 seconds.

```console
Ze web configuration demo

1. Open the local Ze HTTPS interface.
2. Sign in as the local administrator.
3. Open System / Identity in configuration mode.
4. Change the hostname from ze-demo to edge-demo.
5. Save the draft and open Review & Commit.
6. Verify the diff contains `host edge-demo`.
7. Confirm the commit.
8. Reload the setting and verify the active hostname is edge-demo.

Expected result: Ze commits the browser user's isolated draft and the active YANG-backed hostname reads `edge-demo`.
```


## Confirmed commit rollback

### Demo: Watch an unconfirmed change roll back

Commit a hostname change in the interactive editor, leave the confirmation window unanswered, and verify Ze restores the previous configuration.

[Play the WebM recording](../../assets/demos/commit-confirmed.webm?v=91983a21e7) · [View the poster](../../assets/demos/commit-confirmed.png?v=39936789b5) · [Plain-text transcript](../../assets/demos/commit-confirmed.txt?v=7dcd8dbbc1)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 10 seconds.

```console
$ ze config edit -f ze.conf
ze# show system host
host edge-original
ze# set system host edge-trial
ze# show | compare
ze# commit confirmed 8
Committed. Confirm within 8s or auto-revert. Use 'confirm' or 'confirm abort'.
ze# show system host
host edge-trial
Timeout: configuration automatically rolled back.
ze# show system host
host edge-original

ze# set system host edge-confirmed
ze# commit confirmed 8
Committed. Confirm within 8s or auto-revert. Use 'confirm' or 'confirm abort'.
ze# confirm
Configuration confirmed and saved permanently.
ze# show system host
host edge-confirmed

The first change is left unconfirmed and rolls back. The second receives `confirm`; after waiting beyond the same deadline, the editor still reports edge-confirmed.
```


## RPKI validation enforcement

### Demo: Accept valid routes and reject RPKI-invalid ones

Feed three local routes through a deterministic RTR cache, then show Valid and NotFound routes installed while the Invalid route is absent.

[Play the WebM recording](../../assets/demos/rpki.webm?v=971e751f4f) · [View the poster](../../assets/demos/rpki.png?v=4e7e9269d8) · [Plain-text transcript](../../assets/demos/rpki.txt?v=bf49f52038)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 34 seconds.

```console
$ ze cli -c 'show bgp rpki status | no-more'
sessions: 1
vrp-count-ipv4: 171
$ ze cli -c 'show bgp adj-rib-in | no-more'
9.43.0.0/24   validation-state: 1
11.43.0.0/24  validation-state: 2

The local RTR cache classifies 9.43.0.0/24 as Valid, 10.43.0.0/24 as Invalid, and 11.43.0.0/24 as NotFound. Policy accepts Valid and NotFound. The Invalid prefix is absent from Adj-RIB-In because Ze rejects it before installation.
```


## Route installation from BGP RIB to Linux FIB

### Demo: Follow a route from BGP RIB to Linux FIB

Inject one route, inspect BGP best-path selection, and verify Linux installed it with Ze's route protocol ID. Validation also proves withdrawal removes it.

[Play the WebM recording](../../assets/demos/rib-fib.webm?v=6b2efa49fe) · [View the poster](../../assets/demos/rib-fib.png?v=7173a6febd) · [Plain-text transcript](../../assets/demos/rib-fib.txt?v=ca05c09bc8)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 49 seconds.

```console
$ ze cli -c 'request bgp rib inject 192.0.2.10 ipv4/unicast 198.51.100.0/24 origin igp nexthop 127.0.0.1 med 42'
$ ze cli -c 'show bgp rib best prefix 198.51.100.0/24 | no-more'
198.51.100.0/24
$ ip -details route show exact 198.51.100.0/24
198.51.100.0/24 ... proto 250

The route enters Ze's BGP RIB, wins best-path selection, reaches the protocol-independent system RIB, and is programmed into Linux with Ze's route protocol ID. The validator withdraws it and confirms kernel removal.
```


## Live warnings and retained errors

### Demo: Separate live warnings from recent errors

Read aggregate component health, follow a live stale-prefix warning from the SSH banner into show warnings, then reset a peer and find the retained event in show errors.

[Play the WebM recording](../../assets/demos/health-reports.webm?v=c95587e1ba) · [View the poster](../../assets/demos/health-reports.png?v=4b94b2625a) · [Plain-text transcript](../../assets/demos/health-reports.txt?v=113557fa9c)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 1 minute.

```console
$ ssh ze-demo
Warning: stale-prefix-data has stale prefix data (updated 2024-01-01)
ze# run show health
ipsec  down  ike engine not running
ze# run show warnings source bgp
bgp  prefix-stale  warning  127.0.0.2  ... 2024-01-01
ze# run request peer 127.0.0.2 teardown 4
ze# run show errors source bgp
bgp  notification-sent  error  127.0.0.2  direction sent  code 6  subcode 4

Health reports aggregate component state. Warnings describe conditions that remain active and disappear when resolved. Errors retain discrete events. The login banner and filtered commands use the same report bus.
```


## Configuration views and formatter pipes

### Demo: Render one configuration for humans and automation

Show one BGP peer as hierarchical blocks and set commands, round-trip between both with identical canonical output, then compose match and count over Ze's plugin registry.

[Play the WebM recording](../../assets/demos/config-views.webm?v=9286711b41) · [View the poster](../../assets/demos/config-views.png?v=ad33316247) · [Plain-text transcript](../../assets/demos/config-views.txt?v=0f968daa34)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 16 seconds.

```console
$ ze config show router.conf bgp peer transit-a
connection {
    local ip 192.0.2.1
    remote ip 192.0.2.2
}
session {
    asn { local 65000; remote 65001; }
    family ipv4/unicast { prefix maximum 1000000; }
}
$ ze config migrate --format set router.conf | ze pipe match 'bgp peer transit-a'
set bgp peer transit-a connection local ip 192.0.2.1
set bgp peer transit-a connection remote ip 192.0.2.2
set bgp peer transit-a session asn local 65000
set bgp peer transit-a session asn remote 65001
...
$ cmp -s router.set roundtrip.set && echo 'canonical output: identical'
canonical output: identical
$ ze --plugins | ze pipe match flowspec
bgp-nlri-flowspec
flowspec-firewall
...
$ ze --plugins | ze pipe match flowspec | ze pipe count
{"count":3,"pipe":{"count":true}}

Hierarchical and set syntax are alternate presentations of the same parsed configuration. Converting to set syntax and back produces identical canonical set commands. The standalone formatter composes the same match and count operators for shell pipelines.
```


## BFD-triggered BGP failover

### Demo: Let BFD protect a live BGP session

Establish BFD and BGP with a local FRR peer, cut the peer link, and verify BFD drives BGP down before protocol timers expire.

[Play the WebM recording](../../assets/demos/bfd-failover.webm?v=b815e46d6a) · [View the poster](../../assets/demos/bfd-failover.png?v=eadc3dfcc7) · [Plain-text transcript](../../assets/demos/bfd-failover.txt?v=ae76cb645e)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 2 minutes 4 seconds.

```console
An operator needs to verify that BFD, not the 300-second BGP hold timer, protects an edge session.

$ ze config show demos/terminal/bfd-failover/ze.conf bfd
$ ze config show demos/terminal/bfd-failover/ze.conf bgp peer edge-peer connection
$ ze config show demos/terminal/bfd-failover/ze.conf bgp peer edge-peer timer
The daemon configuration shows the 300 ms BFD profile, multiplier 3, single-hop binding, and 300-second BGP hold time.

$ ze cli -c "show bfd sessions"
The running control plane shows the complete Up BFD session.

$ date -u +%T; ip link set bfd-p down
$ ze cli -c "show bfd sessions"
$ ze cli -c "show bgp peer list"
Five seconds after the kernel link is cut, the full command output shows no live BFD session and BGP has left Established.

$ ip link set bfd-p up
$ ze cli -c "show bgp peer list"
The same peer returns to Established after the link is restored.

Every protocol result comes directly from `ze cli`; the lab helper is used only to create and reset the isolated FRR peer.
```


## OSPF adjacency and learned route

### Demo: Diagnose a missing OSPF route

Inspect the active OSPF configuration, query the running control plane with Ze's CLI, trace a Full neighbor through the LSDB, and confirm the expected route.

[Play the WebM recording](../../assets/demos/ospf-adjacency.webm?v=0f9d490996) · [View the poster](../../assets/demos/ospf-adjacency.png?v=18f9f5f412) · [Plain-text transcript](../../assets/demos/ospf-adjacency.txt?v=d82925c752)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 54 seconds.

```console
An operator is investigating why 10.255.0.3/32 is missing.

$ ze config show demos/terminal/ospf-adjacency/ze.conf ospf
The daemon configuration shows the OSPF process, area, interface, and router ID used by the recording.

$ ze cli -c "show ospf neighbor detail"
The live FRR neighbor at 172.31.0.3 is Full.

$ ze cli -c "show ospf database router"
The live link-state database contains FRR's Router-LSA.

$ ze cli -c "show ospf route"
The FRR loopback 10.255.0.3/32 is an intra-area route through 172.31.0.3.

The recording uses `ze cli` directly. No output wrapper or synthetic summary sits between the operator and the running control plane.
```


## Live traffic attribution

### Demo: Attribute a live traffic burst

Attach Ze's pure-Go eBPF accounting to a local veth, generate ICMP and HTTP traffic, and inspect source, protocol, port, and byte totals.

[Play the WebM recording](../../assets/demos/traffic-anomaly.webm?v=f4c762cadb) · [View the poster](../../assets/demos/traffic-anomaly.png?v=bd373dfcc3) · [Plain-text transcript](../../assets/demos/traffic-anomaly.txt?v=fe6d21bab0)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 1 minute 27 seconds.

```console
An operator sees an unexpected burst on `traffic0` and needs to identify the source and application without capturing payloads.

$ ze config show demos/terminal/traffic-anomaly/ze.conf traffic usage
The daemon configuration shows eBPF accounting enabled on `traffic0`, with per-IP tracking and bounded maps.

$ ze cli -c "show traffic usage name traffic0"
The complete baseline snapshot is displayed.

$ ip netns exec traffic-peer ping -c 4 10.77.0.1
$ ip netns exec traffic-peer curl -s -o /dev/null http://10.77.0.1:8080/payload.txt
The isolated workload sends ICMP and HTTP traffic.

$ ze cli -c "show traffic usage name traffic0"
The complete live snapshot attributes bytes to source 10.77.0.2, ICMP, TCP destination port 8080, and reports map occupancy. The accounting path observes traffic only and never modifies or drops packets.
```


## VRRP gateway failover

### Demo: Keep the gateway reachable while Ze stops

Inspect the active and live VRRP state, stop the higher-priority Ze router, and prove keepalived takes the same reachable VIP.

[Play the WebM recording](../../assets/demos/vrrp-failover.webm?v=07fd9dcaf7) · [View the poster](../../assets/demos/vrrp-failover.png?v=27c2d1640a) · [Plain-text transcript](../../assets/demos/vrrp-failover.txt?v=0b9dc45f28)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 2 minutes 38 seconds.

```console
An operator needs to stop the active router without changing the default gateway on every host.

$ ze config show demos/terminal/vrrp-failover/ze.conf interface ethernet eth0 unit 0 ipv4 vrrp group gateway
The daemon configuration shows VRID 10, VIP 192.0.2.1, priority 200, and the advertisement interval.

$ grep -E 'interface|virtual_router_id|priority|192.0.2.1' keepalived.conf
The peer is configured as BACKUP on the same interface, VRID, and VIP with a lower priority of 100.

$ ze cli -c "show vrrp"
The complete live state shows Ze is master.

$ ip -n vrrp-ze -o addr show | grep 192.0.2.1
The kernel shows the VIP on Ze's RFC virtual-MAC interface.

$ cat failover-proof.sh
$ bash -x failover-proof.sh
The recording displays and traces the exact commands that kill Ze, remove its namespace, inspect the VIP on keepalived, and send two probes.

The final kernel output shows 192.0.2.1 on keepalived's `vrrp.10` interface, and both probes succeed after failover.
```


## Offline Linux host inventory

### Demo: Inspect a Linux host before Ze starts

Use Ze's offline command fallback to read the complete kernel, CPU, and memory inventory in human-readable structured output.

[Play the WebM recording](../../assets/demos/host-inventory.webm?v=3644670538) · [View the poster](../../assets/demos/host-inventory.png?v=fc7a447153) · [Plain-text transcript](../../assets/demos/host-inventory.txt?v=5b221c4c0f)

Recorded with Ze 26.08.19 in a Linux namespace lab using VHS 0.11.0. Duration: 37 seconds.

```console
An operator needs to inspect an unfamiliar Linux host before starting Ze.

$ ze show host kernel | ze pipe yaml
The complete live kernel inventory is displayed.

$ ze show host cpu | ze pipe yaml
The complete CPU topology, model, core, and thread inventory is displayed.

$ ze show host memory | ze pipe yaml
The complete memory capacity, availability, cache, swap, and ECC inventory is displayed.

The commands work without a running Ze daemon. Every field returned by `ze show host` remains visible and machine-readable.
```


## Configuration dependency impact

### Demo: Find every peer affected by a group change

Inspect and validate a BGP group, then use Ze's dependency graph to prove which peers inherit the value before scheduling maintenance.

[Play the WebM recording](../../assets/demos/config-graph.webm?v=e0904130d3) · [View the poster](../../assets/demos/config-graph.png?v=cf972162a4) · [Plain-text transcript](../../assets/demos/config-graph.txt?v=1708ee2fac)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 31 seconds.

```console
An operator needs to change the transit group's remote ASN and identify every peer that inherits it before scheduling maintenance.

$ ze config show router.conf bgp group transit
The scoped configuration shows `upstream-a` and `upstream-b` inside the transit group.

$ ze config validate router.conf
configuration valid

$ ze config graph router.conf | ze pipe match peer/upstream
$ ze config graph router.conf | ze pipe match group/transit
$ ze config graph router.conf | ze pipe match inherits
The three direct graph views name both peer nodes, their shared group target, and the two `inherits` relationships.

No reporting helper creates the displayed relationships. The command filters Ze's graph output directly through Ze's format pipeline.
```

