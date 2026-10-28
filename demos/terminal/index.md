# Terminal Demonstrations

These recordings run real Ze commands against isolated local fixtures. Each checked-in tape file defines every keystroke, pause, and terminal size. A release can therefore regenerate the recordings when Ze changes.

A terminal demo is an asciicast. The page replays terminal events as selectable
text, so you can copy a command. The browser demonstration uses a WebP screenshot
and a transcript, not encoded video. The recordings use no public service, and
the terminal player is served from this site.
<!-- source: internal/le/site/demo.go -- demoKindAssets, renderDemoHTML -->

Each demo also appears beside the documentation for the feature it exercises. The transcript below each player provides the same command sequence without replaying the recording.

## Continuous live configuration showcase

One independently recorded session starts with ExaBGP migration and the live BGP
dashboard. The migration preserves the neighbor's identity, local and remote
AS numbers, and configured hold time. The session then loads, proves, diagnoses
and repairs eight features on the same running Ze daemon. Each topic below uses
the same configuration and demonstration fragments as this continuous recording.
<!-- source: demos/terminal/showcase/demo.tape -- authored chapter order -->
<!-- source: internal/le/site/terminaldemo/validate_showcase_topics.go -- validateShowcase -->

### Demo: One running router, eight live configuration topics

Migrate ExaBGP, operate the BGP dashboard, then configure, prove, diagnose and repair eight features in one continuous session without restarting Ze.

[Download the asciicast recording](../../assets/demos/showcase.cast?v=a7426845c0) · [Plain-text transcript](../../assets/demos/showcase.txt?v=8bf911576a)

Recorded with Ze 26.10.28 in a Linux namespace lab using Ze recorder. Duration: 32 minutes 57 seconds.

```console
One running router, eight live configuration topics

0. Migrate an actual ExaBGP neighbor configuration:
   ze exabgp migrate /src/demos/terminal/showcase/exabgp.conf
   Read the resulting bgp group and peer, remote address and AS numbers.
   This previews migration; it does not replace the lab's active configuration.

1. Connect to the base lab over SSH. Exit configuration mode, open monitor bgp,
   press s to sort by ASN, then q to return to the operational prompt.
   The transit and FRR peers are established before any feature is committed.

2. RPKI: add cache and validation binding live. Compare Valid, NotFound and
   Invalid routes; explain the ROA, stop the cache, diagnose and restore it.
3. IRR: query a local test server, bind AS-TEST and import filtering live.
   Prove accepted and refused routes; diagnose a wrong AS-SET and restore it.
4. BFD: add a profile and opt-in to the established FRR BGP peer. Cut its link,
   show BFD diagnostics and BGP history, restore the link and the sessions.
5. OSPF: add the area and interface. Follow Full adjacency through the router
   database to the loopback route. Diagnose a Hello mismatch and restore it.
6. IPsec: load IKE, ESP, authenticated peer/selectors and XFRM route in one
   candidate. Prove negotiated and kernel SPIs with protected-host traffic.
   Mismatch ESP alone, diagnose NO_PROPOSAL_CHOSEN without calling IKE down,
   restore the matching ESP group, clear the peer SAs and prove traffic again.
7. VRRP: add the gateway and uplink tracking. Observe master, tracked-link
   demotion and reduced priority; stop the companion master to show failover
   while the shown Ze stays running. Restore the companion and tracked link.
8. Traffic usage: attach accounting live, identify the heavy source and port,
   stop the burst, and observe recovery without changing forwarding.
9. Commit confirmed: leave a risky hostname change unconfirmed, observe its
   automatic rollback, then explicitly accept the next change and prove it stays.
10. Recap: the same daemon received each live commit, diagnostic and recovery.

Every topic uses its standalone recording's configure.tape and show.tape.
The showcase is recorded as one continuous session, not assembled from casts.
See each topic transcript for the commands, topology and vendor references.
BGP vendor mapping: FRR show bgp summary displays session state; Ze's monitor bgp
adds the interactive live view used here.
https://docs.frrouting.org/en/latest/bgp.html#clicmd-show-ip-bgp-all-summary-wide-json

Ordered visible commands from the continuous tape and its shared topic fragments:

$ ze-demo card showcase intro
$ ze exabgp migrate /src/demos/terminal/showcase/exabgp.conf
$ ze-demo card showcase bgp
$ sshpass -e ssh ze-demo
ze# exit
ze> monitor bgp
ze> exit

RPKI live configuration, outage diagnosis and recovery:
$ ze-demo card topic-rpki lab
ze# load file absolute merge /src/demos/terminal/topics/rpki/1-cache.conf
ze# load file absolute merge /src/demos/terminal/topics/rpki/2-policy.conf
ze# load file absolute merge /src/demos/terminal/topics/rpki/3-attach.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show bgp rpki status | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | yaml | no-more'
$ ze-demo run showcase-lab control rtr-cache stop
$ ze cli -c 'show log recent component bgp.rpki level warn count 3 | yaml | no-more'
$ ze-demo run showcase-lab control rtr-cache start
$ ze cli -c 'show bgp rpki status | yaml | no-more'

IRR live configuration, wrong AS-SET and repaired route decisions:
$ ze-demo card topic-irr lab
ze# load file absolute merge /src/demos/terminal/topics/irr/1-server.conf
ze# load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf
ze# load file absolute merge /src/demos/terminal/topics/irr/3-import.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
ze# load file absolute merge /src/demos/terminal/topics/irr/4-wrong-as-set.conf
ze# commit now
$ ze cli -c 'show bgp irr prefix customer | yaml | no-more'
ze# load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf
ze# commit now
$ ze cli -c 'update bgp irr as-set AS-TEST | yaml | no-more'
$ ze-demo run showcase-lab control customer reannounce
$ ze cli -c 'show bgp adj-rib-in | no-more'

BFD opt-in, link failure and session recovery:
$ ze-demo card topic-bfd overview
ze# load file absolute merge /src/demos/terminal/topics/bfd/1-profile.conf
ze# load file absolute merge /src/demos/terminal/topics/bfd/2-peer.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show bfd session address 172.29.0.2 | no-more'
$ ze-demo run showcase-lab control frr down
$ ze cli -c 'show bgp peer 172.29.0.2 history | no-more'
$ ze-demo run showcase-lab control frr up
$ ze cli -c 'show bfd session address 172.29.0.2 | no-more'

OSPF adjacency, Hello mismatch diagnosis and recovery:
$ ze-demo card topic-ospf overview
ze# load file absolute merge /src/demos/terminal/topics/ospf/1-area.conf
ze# load file absolute merge /src/demos/terminal/topics/ospf/2-interface.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show ospf neighbor detail | no-more'
$ ze cli -c 'show ospf route | no-more'
ze# load file absolute merge /src/demos/terminal/topics/ospf/3-mismatch.conf
ze# commit now
$ ze cli -c 'show metrics name ze_ospf_packets_dropped_total label interface frr0 label reason hello-interval | no-more'
ze# load file absolute merge /src/demos/terminal/topics/ospf/4-recovery.conf
ze# commit now
$ ze cli -c 'show ospf neighbor detail | no-more'
$ ze cli -c 'show ospf route | no-more'

IPsec live tunnel, proposal mismatch and protected traffic recovery:
$ ze-demo card topic-ipsec lab
ze# load file absolute merge /src/demos/terminal/topics/ipsec/1-ike.conf
ze# load file absolute merge /src/demos/terminal/topics/ipsec/2-esp.conf
ze# load file absolute merge /src/demos/terminal/topics/ipsec/3-peer.conf
ze# load file absolute merge /src/demos/terminal/topics/ipsec/4-xfrm.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show vpn ipsec dataplane sa | no-more'
$ ze-demo run showcase-lab control vpn ping
ze# load file absolute merge /src/demos/terminal/topics/ipsec/5-mismatch.conf
ze# commit now
$ ze cli -c 'show vpn ipsec peer name responder | no-more'
$ ze cli -c 'show log recent level warn | match NO_PROPOSAL_CHOSEN | last 1 | yaml | no-more'
$ ze cli -c 'show vpn ipsec dataplane sa | yaml | no-more'
$ ze cli -c 'show vpn ipsec dataplane drift | yaml | no-more'
ze# load file absolute merge /src/demos/terminal/topics/ipsec/2-esp.conf
ze# commit now
$ ze cli -c 'clear vpn ipsec sa peer responder | no-more'
$ ze-demo run showcase-lab control vpn ping
$ ze cli -c 'show vpn ipsec dataplane sa | no-more'

VRRP tracked preference, reduced-priority takeover and recovery:
$ ze-demo card topic-vrrp overview
ze# load file absolute merge /src/demos/terminal/topics/vrrp/1-group.conf
ze# load file absolute merge /src/demos/terminal/topics/vrrp/2-tracking.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ze-demo run showcase-lab control tracked-uplink down
$ ip link show zetrk1
$ ze-demo run showcase-lab control keepalived stop
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ze-demo run showcase-lab control keepalived start
$ ze-demo run showcase-lab control tracked-uplink up
$ ze cli -c 'show vrrp statistics | no-more'

Traffic accounting, heavier senders and settled counters:
$ ze-demo card topic-traffic overview
ze# load file absolute merge /src/demos/terminal/topics/traffic/1-accounting.conf
ze# load file absolute merge /src/demos/terminal/topics/traffic/2-interface.conf
ze# show | compare
ze# commit now
$ ip netns exec traffic-peer ping -c 3 -W 2 10.77.0.1
$ ze cli -c 'show traffic usage name traffic0 | no-more'
$ ze-demo run showcase-lab control traffic burst
$ ze cli -c 'show metrics name ze_traffic_usage_ingress_bytes_total label interface traffic0 label src_ip 10.77.0.2 | no-more'
$ ze-demo run showcase-lab control traffic stop
$ ze cli -c 'show traffic usage name traffic0 | no-more'

Confirmed commit rollback and explicit acceptance:
$ ze-demo card topic-commit-confirmed lab
ze# load file absolute merge /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf
ze# show | compare
ze# commit confirmed 8
ze# show system host
ze# show system host
ze# show | compare
ze# show | changes
ze# load file absolute merge /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf
ze# show | compare
ze# commit confirmed 15
ze# commit accept
ze# show system host
ze# exit
ze> exit
$ ze cli -c "show config history $ZE_CONFIG_DIR/ze.conf | yaml | no-more"
$ ze-demo card topic-commit-confirmed recap
$ ze-demo card showcase recap
```


## Interactive command launcher

### Demo: Discover Ze commands interactively

Use type-ahead filtering and drill-down navigation in Ze's interactive command launcher.

[Download the asciicast recording](../../assets/demos/launcher.cast?v=34f1e25629) · [Plain-text transcript](../../assets/demos/launcher.txt?v=0399dbc59f)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 58 seconds.

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

[Download the asciicast recording](../../assets/demos/cli-dashboard.cast?v=daa8ddc3ec) · [Plain-text transcript](../../assets/demos/cli-dashboard.txt?v=86542601eb)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 45 seconds.

```console
$ ssh ze-demo
ze# exit
ze> monitor bgp

The dashboard polls three local BGP sessions. Press "s" to sort by the next column, use the arrow keys to select a peer, and press Enter for live session details. Press Escape to return and "q" to leave the dashboard.
```


## ZeFS and SSH configuration

### Demo: Create ZeFS and commit over SSH

Create the ZeFS database, edit the active configuration through Ze's SSH management plane, and verify the committed setting.

[Download the asciicast recording](../../assets/demos/zefs-config.cast?v=bd9dbca818) · [Plain-text transcript](../../assets/demos/zefs-config.txt?v=586e5c6ada)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 3 minutes 7 seconds.

```console
$ ze init
username: admin
password: ***
host [127.0.0.1]: 
port [2222]: 
name [ze]: 
discovered 1 interface(s), wrote initial config
initialized /src/tmp/terminal-demos/state/zefs-config/config/database
$ ze config list
ze.conf
$ ze data check

$ ssh ze-demo show bgp

$ ssh ze-demo
ze# set environment cli format default table
ze# show | compare
ze# commit
Session committed
ze# exit
ze> exit
$ ze cli -c 'show bgp'
$ ze cli -c 'show bgp | text'
$ ze cli -c 'show bgp | display router-id local-as peers-established'
$ ze cli -c 'show bgp | display router-id | fill alpha'
$ ze cli -c 'show bgp | peers'
$ ze cli -c 'show bgp | raw' | head -14
$ ze cli -c 'show bgp | raw' | ze pipe text

`ze init` asks for the operator account, the SSH address and port, and the instance name. The password is not echoed, and Enter accepts the default shown in brackets. The instance name defaults to the hostname, here `ze`, so Ze reads `ze.conf`, the configuration `ze init` writes. `ze init` creates `database.zefs`. The first BGP summary uses the default text format. The SSH editor commits the format setting back to ZeFS, not to a second flat file, and the same operational command immediately uses the committed default. The last commands show the two ways to override that default, and they are different pipes. `show bgp | text` is Ze's own operator, inside the quoted command, and it wins over the committed setting. Then `| raw` on its own shows what every one of these renderings is made from: the payload as the daemon holds it, unrendered. The last command sends that same payload across a real shell pipe, and `ze pipe text` formats it on this side, which is how output captured earlier is formatted later. The command is `ze pipe` rather than `ze format` because the operator language also carries `match`, `count`, `first`, `last` and `resolve`, so `format` would name one clause of it. Every command answers with structured data, so `text`, `table`, `json`, `yaml` and `ndjson` all render the same payload. Three commands in the middle choose WHICH of that payload to read. `display` names the fields wanted, in the order wanted, and shows those alone. `fill` brings back the fields it did not name, and `alpha` orders them by field name. `peers` is an alias, which is a name for a pipe expression, so one word answers the per-peer rows without the totals beside them. Each command declares its own column order, so a table leads with the fields an operator reads first rather than with the alphabet.
```


## Read-only operator access

### Demo: Prove read-only RBAC enforcement

Run an allowed NOC command, then show Ze explicitly refuse a known state-changing command.

[Download the asciicast recording](../../assets/demos/rbac.cast?v=ad89e9aeef) · [Plain-text transcript](../../assets/demos/rbac.txt?v=939addc51a)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 1 minute 13 seconds.

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

[Download the asciicast recording](../../assets/demos/traceroute.cast?v=60ee2d3f6b) · [Plain-text transcript](../../assets/demos/traceroute.txt?v=2b9d95cc69)

Recorded with Ze 26.10.09 in a Linux namespace lab using Ze recorder. Duration: 46 seconds.

```console
$ ssh ze-demo
ze# run show traceroute 192.0.2.53
ze# run monitor traceroute 192.0.2.53

The destination and router live in an isolated Linux network-namespace lab. Ze sends real ICMP probes, then shows the same path as a one-shot trace and as a continuously refreshed loss and latency table. No public DNS or Internet route is used.
```


## Web configuration commit

### Demo: Edit and commit configuration in the browser

Change a YANG-backed setting, review the generated diff, commit the draft, and verify the active value.

[View the WebP screenshot](../../assets/demos/web-config.webp?v=dd251ea02c) · [Plain-text transcript](../../assets/demos/web-config.txt?v=a614767cf2)

Recorded with Ze 26.10.24 on macOS and Linux using Playwright 1.55.0. Duration: 58 seconds.

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

### Demo: Protect live SSH commits with rollback

Load a risky hostname change over SSH, observe automatic rollback, then accept a second change and prove it persists on the same running router.

[Download the asciicast recording](../../assets/demos/commit-confirmed.cast?v=ada9d531b2) · [Plain-text transcript](../../assets/demos/commit-confirmed.txt?v=cc3b9170bf)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 1 minute 38 seconds.

```console
Commit confirmed: rollback and acceptance on the running router

Connect with sshpass -e ssh ze-demo. The SSH editor starts from edge-original.
Load /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf, review
show | compare, then run commit confirmed 8. Ze's timeout is in seconds,
unlike Junos commit confirmed, whose timeout is in minutes.

$ ze-demo card topic-commit-confirmed lab
$ ze-demo card topic-commit-confirmed overview
$ sshpass -e ssh ze-demo
ze# show system host
ze# load file absolute merge /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf
ze# show | compare
ze# commit confirmed 8

Show the active edge-trial value, then deliberately leave the change
unaccepted. Wait for the actual automatic rollback notice. Inspect show
system host for edge-original, the empty show | compare view, and
show | changes for the explicit No pending changes response.

ze# show system host

After the automatic rollback:
ze# show system host
ze# show | compare
ze# show | changes

Load /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf,
compare, commit confirmed 8, then commit accept. Wait beyond the eight-second
deadline and inspect edge-confirmed again. The accepted value must remain.

ze# load file absolute merge /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf
ze# show | compare
ze# commit confirmed 8
ze# commit accept

After waiting beyond the deadline:
ze# show system host

Leave the editor and run:
ze# exit
ze> exit
$ ze cli -c "show config history $ZE_CONFIG_DIR/ze.conf | yaml | no-more"
The explicit config folder selects the running lab's stored revisions.
History is a local read: a bare ze.conf selects the shell's current folder,
not ZE_CONFIG_DIR. Both outcomes use the running daemon's session editor,
never an offline file editor or a router restart.

$ ze-demo card topic-commit-confirmed recap

These are the authored walkthrough steps. Runtime validation and recording
must succeed before this source is published as a recording.
```


## RPKI validation enforcement

### Demo: Enable RPKI live and diagnose a cache outage

Commit the RTR cache and validation binding live, distinguish Valid, NotFound and ineligible Invalid routes, diagnose a stopped cache, and restore synchronization.

[Download the asciicast recording](../../assets/demos/rpki.cast?v=a3e919d818) · [Plain-text transcript](../../assets/demos/rpki.txt?v=8853dceca8)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 2 minutes 35 seconds.

```console
RPKI: live policy, failure diagnosis and recovery

The lab has one running Ze, transit AS 65001 and a local test RTR cache.
Before the change, inspect show bgp rpki status and the retained transit routes.
Transit negotiated Route Refresh at boot; no RPKI topic binding was preloaded.

$ ze-demo card topic-rpki lab
$ ze cli -c 'show bgp rpki status | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

In the SSH editor, load the cache, validation policy and transit binding sections:
$ ze-demo card topic-rpki overview
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/rpki/1-cache.conf
ze# load file absolute merge /src/demos/terminal/topics/rpki/2-policy.conf
ze# load file absolute merge /src/demos/terminal/topics/rpki/3-attach.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit

Inspect show bgp rpki status, show bgp rpki summary and show bgp adj-rib-in.
The expected states are Valid for 9.43.0.0/24, NotFound for 11.43.0.0/24,
and Invalid for 10.43.0.0/24. Invalid remains visible but ineligible.
show bgp rpki roa 10.43.0.0/24 explains the origin mismatch: the VRP names
AS 65099 while transit announces AS 65001.

$ ze-demo card topic-rpki proof
$ ze cli -c 'show bgp rpki status | yaml | no-more'
$ ze cli -c 'show bgp rpki summary | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | yaml | no-more'
$ ze-demo card topic-rpki origin
$ ze cli -c 'show bgp rpki roa 10.43.0.0/24 | yaml | no-more'

Capture this outage's log boundary, then stop only the RTR service with
ze-demo run showcase-lab control rtr-cache stop.
Use show log recent component bgp.rpki, show bgp rpki cache and status to
diagnose a failed poll newer than that boundary. Idle is the state between polls, not
proof of an outage. The last complete VRP set remains valid until expiry.

$ ze-demo card topic-rpki failure
$ ze-demo run showcase-lab control rtr-cache stop
$ ze cli -c 'show log recent component bgp.rpki level warn count 3 | yaml | no-more'
$ ze cli -c 'show bgp rpki cache | yaml | no-more'
$ ze cli -c 'show bgp rpki status | yaml | no-more'

Restart the cache with ze-demo run showcase-lab control rtr-cache start.
Require a sync-complete log strictly newer than this outage's failed poll and
recheck the route states. show bgp peer detail transit exposes its counter;
the external fixture must also have accepted exactly one connection over the
whole chapter, so replacing a Ze Peer cannot hide a reconnect.

$ ze-demo card topic-rpki recovery
$ ze-demo run showcase-lab control rtr-cache start
$ ze cli -c 'show log recent component bgp.rpki count 4 | yaml | no-more'
$ ze cli -c 'show bgp rpki status | yaml | no-more'
$ ze cli -c 'show bgp peer detail transit | yaml | match connections-established | no-more'

These are the authored walkthrough steps. Runtime validation and recording
must succeed before this source is published as a recording.
```


## Live IRR filtering and AS-SET diagnosis

### Demo: Add IRR filtering to an existing BGP peer

Load the local test IRR server, AS-SET and import policy over SSH, prove route acceptance and refusal, then diagnose and repair a wrong AS-SET without restarting Ze.

[Download the asciicast recording](../../assets/demos/irr-filter.cast?v=132c1244b0) · [Plain-text transcript](../../assets/demos/irr-filter.txt?v=46836a5e6e)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 3 minutes 42 seconds.

```console
IRR: live import filtering and repair

The running base router has a customer in AS 65001 and no IRR policy.
The IRR service at 127.0.0.1:4343 is a local test server, not a public registry.
AS-TEST contains 10.0.0.0/24 but not 192.168.0.0/24.

$ ze-demo card topic-irr lab
$ ze cli -c 'show bgp irr | yaml | no-more'

Load three sections in one SSH candidate:
$ ze-demo card topic-irr overview
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/irr/1-server.conf
ze# load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf
ze# load file absolute merge /src/demos/terminal/topics/irr/3-import.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit

The customer announces both disputed routes and two controls. Inspect show bgp
irr, show bgp irr prefix customer and show bgp irr check customer for each prefix.
Adj-RIB-In must contain 10.0.0.0/24 and the final accepted 10.0.1.0/24 control,
but omit both 192.168.0.0/24 and the wrong-set 192.0.2.0/24 control.

$ ze-demo card topic-irr proof
$ ze cli -c 'show bgp irr | yaml | no-more'
$ ze cli -c 'show bgp irr prefix customer | yaml | no-more'
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp irr check customer 192.168.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

Load 4-wrong-as-set.conf in the SSH editor and commit. The local AS-V4ONLY
object is valid, but its 192.0.2.0/24 prefix does not authorize this customer.
Before reannouncing, wait for the old session and its routes to clear. The new
fixture sends AS-V4ONLY's accepted 192.0.2.0/24 control last. Only after that
control arrives can the absence of both 10.0 routes prove refusal, not delay.

$ ze-demo card topic-irr failure
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/irr/4-wrong-as-set.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp irr prefix customer | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

Load 2-as-set.conf again and commit the repair. Run update bgp irr as-set
AS-TEST, then ze-demo run showcase-lab control customer reannounce. Inspect
the accepted decision, 10.0.0.0/24 and its final 10.0.1.0/24 control again.
The wrong-set control and outside route must be absent. Ze stays running.

$ ze-demo card topic-irr recovery
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit
$ ze cli -c 'update bgp irr as-set AS-TEST | yaml | no-more'
$ ze-demo run showcase-lab control customer reannounce
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

These are the authored walkthrough steps. Runtime validation and recording
must succeed before this source is published as a recording.
```


## Route installation from BGP RIB to Linux FIB

### Demo: Follow a route from BGP RIB to Linux FIB

Inject one route, inspect BGP best-path selection, and verify Linux installed it with Ze's route protocol ID. Validation also proves withdrawal removes it.

[Download the asciicast recording](../../assets/demos/rib-fib.cast?v=01fa66b2f1) · [Plain-text transcript](../../assets/demos/rib-fib.txt?v=ca05c09bc8)

Recorded with Ze 26.10.09 in a Linux namespace lab using Ze recorder. Duration: 52 seconds.

```console
$ ze cli -c 'request bgp rib inject 192.0.2.10 ipv4/unicast 198.51.100.0/24 origin igp nexthop 127.0.0.1 med 42'
$ ze cli -c 'show bgp rib best prefix 198.51.100.0/24 | no-more'
198.51.100.0/24
$ ip -details route show exact 198.51.100.0/24
198.51.100.0/24 ... proto 250

The route enters Ze's BGP RIB, wins best-path selection, reaches the protocol-independent system RIB, and is programmed into Linux with Ze's route protocol ID. The validator withdraws it and confirms kernel removal.
```


## Live warnings and retained errors

<!-- The health-reports demo is not embedded while its recording is wrong. Its
     tape drives an SSH session whose commands the CLI answers with a completion
     listing rather than a result, so the recording holds the intro card, a
     config box and the recap, and none of the output this gallery describes.
     Restore this marker when the recording shows the session again.
     Recorded in plan/journal/green-that-could-not-have-been-red.md -->
<!-- terminal-demo-disabled: health-reports -->

## Configuration views and formatter pipes

### Demo: Render one configuration for humans and automation

Show one BGP peer as hierarchical blocks and set commands, round-trip between both with identical canonical output, then compose match and count over Ze's plugin registry.

[Download the asciicast recording](../../assets/demos/config-views.cast?v=74dca8e268) · [Plain-text transcript](../../assets/demos/config-views.txt?v=9f1109a1a7)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 1 minute 28 seconds.

```console
$ ze config show router.conf bgp peer transit-a
attach process bgp-rib {
    receive [ update state refresh ]
    send update
}
connection {
    local ip 192.0.2.1
    remote ip 192.0.2.2
}
session {
    asn {
        local 65000
        remote 65001
    }
    family ipv4/unicast {
        prefix maximum 1000000
    }
}
$ ze config migrate format set router.conf 2>/dev/null | ze pipe match 'bgp peer transit-a'
set bgp peer transit-a attach process bgp-rib receive [ update state refresh ]
set bgp peer transit-a attach process bgp-rib send update
set bgp peer transit-a connection local ip 192.0.2.1
set bgp peer transit-a connection remote ip 192.0.2.2
set bgp peer transit-a session asn local 65000
set bgp peer transit-a session asn remote 65001
set bgp peer transit-a session family ipv4/unicast prefix maximum 1000000
$ cmp -s $STATE/router.set $STATE/roundtrip.set && echo 'canonical output: identical'
canonical output: identical
$ ze show plugin list | ze pipe match flowspec
bgp-nlri-flowspec             FlowSpec NLRI encoding/decoding ...
ddos-flowspec                 DDoS FlowSpec/RTBH responder: upstream mitigation with leak-probe clear ...
flowspec-firewall             Translates BGP FlowSpec routes into nftables firewall rules ...
$ ze show plugin list | ze pipe match flowspec | ze pipe count
{"count":3,"pipe":[{"op":"count"}]}

Hierarchical and set syntax are alternate presentations of the same parsed configuration. Converting to set syntax and back produces identical canonical set commands. The standalone formatter composes the same match and count operators for shell pipelines.
```


## BFD-triggered BGP failover

### Demo: Let BFD protect a live BGP session

Commit BFD onto an established FRR BGP peer, cut the link, inspect diagnostics and peer history, then restore both sessions without restarting Ze.

[Download the asciicast recording](../../assets/demos/bfd-failover.cast?v=a37cfae7d2) · [Plain-text transcript](../../assets/demos/bfd-failover.txt?v=1d2f779b15)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 2 minutes 33 seconds.

```console
BFD live opt-in and link-failure walkthrough

Lab: one running Ze router and its established FRR BGP peer at 172.29.0.2.
$ ze-demo card topic-bfd overview
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze cli -c 'show bfd sessions | no-more'
$ ip netns exec showcase-frr vtysh -c 'show bgp neighbor 172.29.0.1 json'

Load: profile fast and peer frr BFD binding, section by section in one SSH candidate.
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/bfd/1-profile.conf
ze# load file absolute merge /src/demos/terminal/topics/bfd/2-peer.conf
ze# show | compare
ze# commit now

Proof: show bfd sessions/profile/session and BGP detail expose Up, real packet counts and the negotiated hold timer.
$ ze-demo card topic-bfd proof
$ ze cli -c 'show bfd profile name fast | no-more'
$ ze cli -c 'show bfd session address 172.29.0.2 | no-more'
$ ze cli -c 'show bgp peer 172.29.0.2 detail | no-more'
$ ip netns exec showcase-frr vtysh -c 'show bgp neighbor 172.29.0.1 json'

Failure: bring frr0 down; inspect detection-expired transition, BFD Down notification and BGP history before hold expiry.
$ ze-demo card topic-bfd failure
$ ze-demo run showcase-lab control frr down
$ ze cli -c 'show bfd session address 172.29.0.2 | no-more'
$ ze cli -c 'show bgp peer 172.29.0.2 detail | no-more'
$ ze cli -c 'show bgp peer 172.29.0.2 history | no-more'

Fix: observe released-session retirement, restore frr0, require BFD Up and BGP Established, and print the same shown PID.
$ ze-demo card topic-bfd fix
$ ze cli -c 'show bfd sessions | no-more'
$ ze-demo run showcase-lab control frr up
$ ze cli -c 'show bfd session address 172.29.0.2 | no-more'
$ ze cli -c 'show bgp peer 172.29.0.2 detail | no-more'
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze-demo card topic-bfd recap
```


## OSPF adjacency and learned route

### Demo: Enable OSPF and diagnose a Hello mismatch

Commit OSPF live, follow a Full neighbor through the router database to its loopback route, then diagnose and repair mismatched Hello timers.

[Download the asciicast recording](../../assets/demos/ospf-adjacency.cast?v=6b38530780) · [Plain-text transcript](../../assets/demos/ospf-adjacency.txt?v=fc63b795c8)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 5 minutes 57 seconds.

```console
OSPF live adjacency and Hello-mismatch walkthrough

Lab: Ze on frr0, FRR neighbor 172.29.0.2 and loopback 10.255.1.2/32.
$ ze-demo card topic-ospf overview
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze cli -c 'show ospf neighbor | no-more'

Load: area 0 and broadcast interface timers 10/40 in one SSH candidate.
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/ospf/1-area.conf
ze# load file absolute merge /src/demos/terminal/topics/ospf/2-interface.conf
ze# show | compare
ze# commit now

Proof: show ospf neighbor detail, database router and route connect Full to the Router-LSA and loopback next hop.
$ ze-demo card topic-ospf proof
$ ze cli -c 'show ospf neighbor detail | no-more'
$ ze cli -c 'show ospf interface detail | no-more'
$ ze cli -c 'show ospf database router | no-more'
$ ze cli -c 'show ospf route | no-more'

Failure: commit Hello interval 20; inspect current interface timers, lost adjacency/route and the hello-interval packet-drop metric.
$ ze-demo card topic-ospf failure
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/ospf/3-mismatch.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show ospf neighbor detail | no-more'
$ ze cli -c 'show ospf interface detail | no-more'
$ ze cli -c 'show metrics name ze_ospf_packets_dropped_total label interface frr0 label reason hello-interval | no-more'
$ ze cli -c 'show ospf route | no-more'

Fix: commit interval 10, require all three protocol observations again, and print the unchanged shown PID.
$ ze-demo card topic-ospf fix
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/ospf/4-recovery.conf
ze# show | compare
ze# commit now
$ ze cli -c 'show ospf neighbor detail | no-more'
$ ze cli -c 'show ospf database router | no-more'
$ ze cli -c 'show ospf route | no-more'
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze-demo card topic-ospf recap
```


## Native IPsec configuration and proposal diagnosis

### Demo: Build and repair a native IPsec tunnel live

Commit IKE, ESP, selectors and XFRM routing over SSH, prove kernel SPIs with protected traffic, diagnose an ESP mismatch, and restore the tunnel.

[Download the asciicast recording](../../assets/demos/ipsec.cast?v=1aff2bf168) · [Plain-text transcript](../../assets/demos/ipsec.txt?v=f5fa147589)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 2 minutes 40 seconds.

```console
Native IKEv2 and Linux IPsec, configured live

Lab: shown Ze 172.30.0.1 initiates to a hidden Ze responder at 172.30.0.2.
Protected hosts 10.78.1.2 and 10.78.2.2 live behind different routers.
The shown daemon starts with no VPN configuration or XFRM interface.

$ ze-demo card topic-ipsec lab
$ ze-demo card topic-ipsec overview

In one SSH candidate, load topics/ipsec/1-ike.conf, 2-esp.conf, 3-peer.conf,
and 4-xfrm.conf with load file absolute merge. Review show | compare and
commit now. The sections define IKE, ESP, authenticated peer and selectors,
and the route through vpn-xfrm (if-id 42).

$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/ipsec/1-ike.conf
ze# load file absolute merge /src/demos/terminal/topics/ipsec/2-esp.conf
ze# load file absolute merge /src/demos/terminal/topics/ipsec/3-peer.conf
ze# load file absolute merge /src/demos/terminal/topics/ipsec/4-xfrm.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit

$ ze-demo card topic-ipsec proof
$ ze cli -c 'show vpn ipsec status | no-more'
$ ze cli -c 'show vpn ipsec sa | no-more'
$ ze cli -c 'show vpn ipsec dataplane sa | no-more'
$ ze-demo run showcase-lab control vpn ping
$ ze cli -c 'show vpn ipsec sa | no-more'

The kernel SPIs must match the negotiated Child SA. The protected ping
must succeed and the same inbound SA's packets-in must increase.

Failure: load 5-mismatch.conf and commit. It changes only ESP to AES-128;
the far end offers AES-256. IKE can stay established while no Child SA exists.

$ ze-demo card topic-ipsec failure
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/ipsec/5-mismatch.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit

$ ze-demo card topic-ipsec diagnosis
$ ze cli -c 'show vpn ipsec peer name responder | no-more'
$ ze cli -c 'show log recent level warn | match NO_PROPOSAL_CHOSEN | last 1 | yaml | no-more'
$ ze cli -c 'show vpn ipsec dataplane sa | yaml | no-more'
$ ze cli -c 'show vpn ipsec dataplane drift | yaml | no-more'

The same log entry carries peer responder and notify NO_PROPOSAL_CHOSEN
under attributes beside the message that IKE remains established. An empty
drift result means the engine and kernel agree, not that a tunnel carries traffic.
Load 2-esp.conf, review and commit, then clear vpn ipsec sa peer responder.
Verify the new kernel SPIs, successful protected ping and increasing packets-in.
The shown Ze process is never restarted.

$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/ipsec/2-esp.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit
$ ze cli -c 'clear vpn ipsec sa peer responder | no-more'
$ ze-demo run showcase-lab control vpn ping
$ ze cli -c 'show vpn ipsec sa | no-more'
$ ze cli -c 'show vpn ipsec dataplane sa | no-more'
$ ze-demo card topic-ipsec recap

Vendor mapping: Junos show security ike security-associations reads IKE state;
show security ipsec security-associations reads the IPsec SAs and their SPIs.
https://www.juniper.net/documentation/us/en/software/junos/cli-reference/topics/ref/command/show-security-ike-security-associations.html
https://www.juniper.net/documentation/us/en/software/junos/cli-reference/topics/ref/command/show-security-ipsec-security-associations.html
https://www.juniper.net/documentation/us/en/software/junos/cli-reference/topics/ref/statement/security-edit-proposal.html
```


## Live traffic attribution

### Demo: Attribute a live traffic burst

Enable eBPF accounting on the running router, attribute an unexpected ICMP and HTTP burst to its source and port, stop the sender, and observe recovery.

[Download the asciicast recording](../../assets/demos/traffic-anomaly.cast?v=5bfc138dad) · [Plain-text transcript](../../assets/demos/traffic-anomaly.txt?v=8c33b2924c)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 3 minutes 11 seconds.

```console
Traffic usage live attribution walkthrough

Lab: traffic0 at 10.77.0.1, real sender 10.77.0.2 and HTTP server port 8080.
$ ze-demo card topic-traffic overview
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze cli -c 'show traffic usage | no-more'

Load: bounded eBPF accounting and traffic0 attachment in one SSH candidate.
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/traffic/1-accounting.conf
ze# load file absolute merge /src/demos/terminal/topics/traffic/2-interface.conf
ze# show | compare
ze# commit now

Proof: real ping and curl populate source, ICMP and TCP/8080 byte totals.
Long shell commands below quote their first painted-row prefix, not invented output.
$ ze-demo card topic-traffic proof
$ ip netns exec traffic-peer ping -c 3 -W 2 10.77.0.1
$ ip netns exec traffic-peer curl --fail --silent --show-error --max-time 5
$ ze cli -c 'show traffic usage name traffic0 | no-more'

Failure: bounded heavier senders increase those specific totals in successive snapshots; filtered metrics identify the source.
$ ze-demo card topic-traffic failure
$ ze-demo run showcase-lab control traffic burst
$ ze cli -c 'show traffic usage name traffic0 | no-more'
$ ze cli -c 'show traffic usage name traffic0 | no-more'
$ ze cli -c 'show metrics name ze_traffic_usage_ingress_bytes_total label interface traffic0 label src_ip 10.77.0.2 | no-more'
$ ze cli -c 'show metrics name ze_traffic_usage_ingress_port_bytes_total label interface traffic0 label dst_port 8080 label protocol tcp
$ ze cli -c 'show metrics name ze_traffic_usage_ingress_port_bytes_total label interface traffic0 label dst_port 0 label protocol icmp

Fix: stop the senders and require cumulative totals to settle for two seconds within ten seconds, without resetting counters or replacing Ze.
$ ze-demo card topic-traffic fix
$ ze-demo run showcase-lab control traffic stop
$ ze cli -c 'show traffic usage name traffic0 | no-more'
$ ze cli -c 'show traffic usage name traffic0 | no-more'
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze-demo card topic-traffic recap
```


## VRRP uplink tracking and gateway failover

### Demo: Track an uplink and fail over without stopping Ze

Commit VRRP and uplink tracking live, observe priority reduction and backup demotion, then prove master failover and recovery while the shown Ze stays running.

[Download the asciicast recording](../../assets/demos/vrrp-tracking.cast?v=d55609ce3e) · [Plain-text transcript](../../assets/demos/vrrp-tracking.txt?v=d48d9109d5)

Recorded with Ze 26.10.27 in a Linux namespace lab using Ze recorder. Duration: 2 minutes 50 seconds.

```console
VRRP tracking and peer-failover walkthrough

Lab: VIP 203.0.113.1, VRID 12; shown Ze priority 200 and keepalived priority 100.
$ ze-demo card topic-vrrp overview
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze cli -c 'show vrrp | no-more'

Load: gateway group and tracked zetrk1 decrement 150 in one SSH candidate.
$ sshpass -e ssh ze-demo
ze# load file absolute merge /src/demos/terminal/topics/vrrp/1-group.conf
ze# load file absolute merge /src/demos/terminal/topics/vrrp/2-tracking.conf
ze# show | compare
ze# commit now

Proof: show vrrp, kernel VIP ownership, virtual MAC and a real reachability probe.
$ ze-demo card topic-vrrp proof
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ip -br address show
$ ip netns exec showcase-vrrp ping -I eth0 -c 3 -W 2 203.0.113.1

Failure: uplink down makes Ze Backup at 50; stopping only keepalived makes Ze Master at 50.
$ ze-demo card topic-vrrp failure
$ ze-demo run showcase-lab control tracked-uplink down
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ip link show zetrk1
$ ip -n showcase-vrrp -br address show
$ ping -I zeptrk0 -c 3 -W 2 203.0.113.1
$ ze-demo card topic-vrrp takeover
$ ze-demo run showcase-lab control keepalived stop
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ip -br address show
$ ip -n showcase-vrrp -br address show
$ ip netns exec showcase-vrrp ping -I eth0 -c 3 -W 2 203.0.113.1

Fix: restart keepalived (Ze Backup 50), restore uplink (Ze Master 200), inspect statistics and reachability with the same shown PID.
$ ze-demo card topic-vrrp fix
$ ze-demo run showcase-lab control keepalived start
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ip -n showcase-vrrp -br address show
$ ze-demo run showcase-lab control tracked-uplink up
$ ze cli -c 'show vrrp interface name zeptrk0 | no-more'
$ ze cli -c 'show vrrp statistics | no-more'
$ ip -br link show
$ ip netns exec showcase-vrrp ping -I eth0 -c 3 -W 2 203.0.113.1
$ cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid
$ ze-demo card topic-vrrp recap
```


## Offline Linux host inventory

### Demo: Inspect a Linux host before Ze starts

Use Ze's offline command fallback to read the complete kernel, CPU, and memory inventory in human-readable structured output.

[Download the asciicast recording](../../assets/demos/host-inventory.cast?v=914610af44) · [Plain-text transcript](../../assets/demos/host-inventory.txt?v=5b221c4c0f)

Recorded with Ze 26.10.09 in a Linux namespace lab using Ze recorder. Duration: 39 seconds.

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

[Download the asciicast recording](../../assets/demos/config-graph.cast?v=4c952f92ac) · [Plain-text transcript](../../assets/demos/config-graph.txt?v=7a64ac5a0c)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 1 minute 13 seconds.

```console
An operator needs to change the transit group's remote ASN and identify every peer that inherits it before scheduling maintenance.

$ ze config show router.conf bgp group transit
The scoped configuration shows `upstream-a` and `upstream-b` inside the transit group.

$ ze config validate router.conf
configuration valid

$ ze config graph router.conf | ze pipe text | ze pipe match peer/upstream
$ ze config graph router.conf | ze pipe text | ze pipe match group/transit
$ ze config graph router.conf | ze pipe text | ze pipe match inherits
The graph answer holds two lists, `nodes` and `edges`. A row operator such as `match` has no single set of rows there, so Ze refuses it by name instead of picking one list. `| text` renders both lists as aligned rows, one relationship to a line. `| match` then keeps the lines that name the two peers, the group they share, and the two `inherits` relationships.

No reporting helper creates the displayed relationships. The command filters Ze's graph output directly through Ze's format pipeline.
```

