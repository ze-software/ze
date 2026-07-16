# Terminal Demonstrations

These recordings run real Ze commands against isolated local fixtures. The checked-in VHS tapes define every keystroke, pause, and terminal size, so a release can regenerate the videos when Ze changes. The recordings use no public service.

Each demo also appears beside the documentation for the feature it exercises. The transcript below each player provides the same command sequence without requiring video playback.

## Interactive command launcher

### Terminal demo: Discover Ze commands interactively

Use type-ahead filtering and drill-down navigation in Ze's interactive command launcher.

[Play the WebM recording](../../../assets/demos/launcher.webm) · [View the poster](../../../assets/demos/launcher.png) · [Plain-text transcript](../../../assets/demos/launcher.txt)

Recorded with Ze 26.07.16 on macOS and Linux. Duration: 55 seconds.

```console
$ ze

Type "show" to filter the command launcher, then press Enter to open the show command tree.
Type "traceroute" to find the path diagnostic command.
Press Escape and Left to return, then type "doctor" to find the readiness checker.
Press Escape to move back through the menu and return to the shell.
```


## Live BGP dashboard

### Terminal demo: Operate BGP from the live dashboard

Connect to Ze over SSH, open the live BGP dashboard, sort peers, and inspect one session.

[Play the WebM recording](../../../assets/demos/cli-dashboard.webm) · [View the poster](../../../assets/demos/cli-dashboard.png) · [Plain-text transcript](../../../assets/demos/cli-dashboard.txt)

Recorded with Ze 26.07.16 on macOS and Linux. Duration: 50 seconds.

```console
$ ssh ze-demo
ze# exit
ze> monitor bgp

The dashboard polls three local BGP sessions. Press "s" to sort by the next column, use the arrow keys to select a peer, and press Enter for live session details. Press Escape to return and "q" to leave the dashboard.
```


## ZeFS and SSH configuration

### Terminal demo: Create ZeFS and commit over SSH

Create the ZeFS database, edit the active configuration through Ze's SSH management plane, and verify the committed setting.

[Play the WebM recording](../../../assets/demos/zefs-config.webm) · [View the poster](../../../assets/demos/zefs-config.png) · [Plain-text transcript](../../../assets/demos/zefs-config.txt)

Recorded with Ze 26.07.16 on macOS and Linux. Duration: 85 seconds.

```console
$ ze init < "$ZE_INIT_INPUT"
$ ze config ls
ze.conf
$ ze data check

$ ssh ze-demo
ze# set environment cli format default table
ze# show | compare
ze# commit
Session committed
ze# run show bgp summary

`ze init` creates `database.zefs`. The SSH editor then commits the table-format setting back to ZeFS, not to a second flat file. The operational command uses the new default immediately.
```


## Read-only operator access

### Terminal demo: Prove read-only RBAC enforcement

Run an allowed NOC command, then show Ze explicitly refuse a known state-changing command.

[Play the WebM recording](../../../assets/demos/rbac.webm) · [View the poster](../../../assets/demos/rbac.png) · [Plain-text transcript](../../../assets/demos/rbac.txt)

Recorded with Ze 26.07.16 on macOS and Linux. Duration: 45 seconds.

```console
$ ze cli --user noc -c 'show version'
version: ze 26.07.15
$ ze cli --user noc -c 'clear interface counters'
error: command restricted by access control

The read-only NOC profile allows operational show commands and denies every command matching "clear". The daemon resolves the command, finds the profile forbids it, and refuses before running it. The password is supplied outside the recording.
```


## Traceroute in an isolated Linux lab

### Terminal demo: Trace a live path without external services

Run Ze's live traceroute through a deterministic Linux network-namespace lab.

[Play the WebM recording](../../../assets/demos/traceroute.webm) · [View the poster](../../../assets/demos/traceroute.png) · [Plain-text transcript](../../../assets/demos/traceroute.txt)

Recorded with Ze 26.07.16 in a Linux namespace lab. Duration: 55 seconds.

```console
$ ssh ze-demo
ze# run show traceroute 192.0.2.53
ze# run monitor traceroute 192.0.2.53

The destination and router live in an isolated Linux network-namespace lab. Ze sends real ICMP probes, then shows the same path as a one-shot trace and as a continuously refreshed loss and latency table. No public DNS or Internet route is used.
```

