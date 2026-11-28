---
category: observe
topic: architecture-and-performance
title: How Ze isolates faults and keeps diagnostics alive
date: 2026-11-27
author: Thomas Mangin
description: How Ze pairs task-level panic containment, optional separate plugin processes, lock-free diagnostics and bounded memory rings so a local fault stays contained and observable.

deck: Sharing a Go process by default raises the stakes when something crashes or hangs, so Ze builds fault containment and bounded diagnostics into the same boundaries.

image: assets/blog/fault-isolation-and-live-diagnostics.svg
image-dark: assets/blog/fault-isolation-and-live-diagnostics-dark.svg
image-alt: Three-stage fault isolation architecture showing taskguard and optional separate plugin processes, always-on zero-allocation bad-packet and crash capture, lock-free live plugin introspection, and hard memory, disk, and time bounds.

---

Ze runs a router's protocols inside one Go binary by default. That layout brings low latency, shared packet buffers and simple deployment, yet it also raises an obvious risk. If one malformed packet or panicking handler brings down the process, every peering session on the router drops with it.

This post explains how Ze isolates faults in that architecture and keeps live diagnostics working even when a lock or a separate plugin process is stuck. [Debugging a live router at two in the morning](../debugging-a-live-router/) walks through what those tools look like during an incident.

*This article was co-authored with an AI assistant. The architecture, design decisions and conclusions are mine. The assistant helped organise the material and draft the text.*

## Comparing single-process and multi-daemon designs

FRR splits routing protocols across separate daemons (`bgpd`, `ospfd`, `isisd`, `zebra`), and IOS-XR does the same, so a crash in one protocol stays out of the others. Junos takes a middle path, separating chassis and management daemons while running its routing protocols together inside `rpd`.

Ze's default single-process layout puts more at risk when something goes wrong, because compiled-in plugins share one Go binary by default. Most compiled-in plugins can be moved into their own OS process with `process separate;` (external plugins already run in separate processes), though that is an operator choice rather than the default.

Because sharing a process raises the stakes, we designed fault isolation and live diagnostics together. Recovery boundaries keep a local failure from taking down the rest of the router. At the same time, catching a panic silently or restarting a plugin in the background without clear records hides bugs that keep coming back while the dashboard stays green. Every isolation boundary in Ze therefore feeds directly into read-only diagnostic commands defined in YANG ([Keeping documentation in step with the code](../reference-from-the-system/)).

## Isolating one bad BGP packet, borrowed from ExaBGP

A single BGP peer bouncing is rarely an emergency on its own. Traffic shifts to another transit or peering session, the neighbour reconnects, and most customers never notice.

One malformed BGP UPDATE turns into an outage when a parser bug crashes the whole BGP process and drops every session at once. It also hurts when a peer stays stuck in a reset loop over an attribute nobody captured. Unless debug logging, `traceoptions` or BMP was already turned on, that bad UPDATE is gone by the time you log in.

In Ze, every loop that reads network input (a BGP session, a protocol receiver or a request handler) sits behind a recovery boundary ([`taskguard`](https://github.com/ze-software/ze/blob/main/internal/core/taskguard/guard.go)). That idea comes straight from ExaBGP, where Python's `try`/`except` around each peer isolated one peer's failure into its own failure domain inside a single process. In Go, a panic inside that boundary closes only that single session or request. Every other peer and route keeps working, turning a potential router-wide crash back into a minor single-peer bounce.

That recovery is never silent. Before the session closes, Ze writes an error log line, increments a panic counter and saves a short flight-recorder trace of recent runtime activity. It raises the `doctor-internal-defect` diagnostic code so `show doctor` and `show errors` flag it immediately. It also writes a recovered crash report (`crash-<time>-recovered.log`) in its own rotation pool, governed by `ze.crash.keep` (default 5). Because fatal crash files use a separate pool, a noisy peer cannot overwrite the report of a real daemon crash.

Ze also copies the offending wire bytes into an always-on circular buffer ([`internal/core/badpacket`](https://github.com/ze-software/ze/blob/main/internal/core/badpacket/badpacket.go)). It holds the last 16 bad packets, up to 4 KiB each (64 KiB in total), with zero heap allocation on normal traffic. The same buffer captures BGP RFC 7606 malformed UPDATEs (`treat-as-withdraw`, `attribute-discard`, `session-reset`) and bad BGP headers on both the standard and coalesced read paths. When a read-loop panic happens, Ze copies the frame only if the read goroutine still owns the buffer, never touching a buffer already handed off to forwarding.

## Optional process isolation and safe plugin introspection

When you want OS-level isolation for a third-party or high-risk plugin, `process separate;` moves a compiled-in plugin into its own subprocess over TLS and RPC (external plugins already run that way). Each separate-process plugin declares a `failure-policy` of `restart`, `ignore` or `fatal` ([process protocol](https://github.com/ze-software/ze/blob/main/docs/architecture/api/process-protocol.md)). Its crash or memory leak stays outside the engine.

Separate processes create a well-known debugging gap. When a daemon hangs or leaks memory, operators usually reach for shell tools such as `pprof`, `gcore`, `perf` or `/proc/<pid>/stack`. Otherwise, they take a blind restart that wipes out the state in memory. Sending `SIGQUIT` to dump stacks is even harsher: under both [Linux `signal(7)`](https://man7.org/linux/man-pages/man7/signal.7.html) and [Go's default signal handling](https://pkg.go.dev/os/signal#hdr-Default_behavior_of_signals_in_Go_programs), `SIGQUIT` terminates the process and closes every TCP and BGP session it owns. Shell tools are also absent on locked-down appliances, and a router CLI rarely reaches inside child processes.

In Ze, you can inspect any plugin directly from the CLI or MCP without a shell or `go tool pprof`.

```sh
show system goroutines mode blocked | plugin bgp-ext
show system profile type heap | plugin bgp-ext | summary
show log recent level err | plugin bgp-ext
```

The `| summary` pipe turns a Go profile into a short JSON list of top functions, while bare `show system profile` still returns raw pprof data for `go tool pprof`. The `| plugin <name>` pipe works identically whether the plugin runs inside the daemon or in a separate process. It answers from a dedicated worker inside the plugin, so it still responds even if the plugin's main loop is deadlocked.

If a separate plugin process is frozen, disconnected or written in another language, Ze never sends `SIGQUIT` or kills it to get a trace. It returns `fallback: true` with the reason, Linux `/proc/<pid>` stats (state, wait channel, threads, memory, open file count) and the last 32 lines of stderr.

## Staying reachable when a lock or handler wedges

Diagnostics fail hardest when a configuration lock is stuck. On Ze, core diagnostic commands never wait on the configuration lock. `show system watchdog`, `show system reload status` and `show system reload history` read only lightweight leaf locks and atomics. `show system triage` avoids the configuration lock too, bounding its sub-queries and marking any timed-out section `available: false`.

Operator commands also time out after 60 seconds. If a handler refuses to stop, Ze returns a timeout error carrying its `operation-id`, raises `doctor-command-handler-hung` and saves a trace. Alongside that, an always-on watchdog checks every 2 seconds for stuck config locks (past 45 seconds), hung commands and pauses when the Go runtime stops scheduling work cleanly. When a stall happens, it saves stack dumps and traces into `show system watchdog` and `show system trace id <id>`.

An unrecovered panic or fatal error can still kill the whole daemon, including on a minimal, shell-less gokrazy Linux appliance. To catch that case, Ze keeps a crash file pre-armed through `debug.SetCrashOutput` ([crash capture](https://github.com/ze-software/ze/blob/main/docs/architecture/diagnostics/crash-capture.md)). The Go runtime writes the crash dump straight into that file before exiting, and the next boot moves it into `show crashes` so `show system triage` shows it right away.

## Bounded by design

Every diagnostic buffer and check in Ze has hard size and time limits so debugging can never exhaust memory, fill the disk or slow the router down. Bad packets cap at 16 slots of 4 KiB. Drift checks cap at 64 findings and 5 seconds per subsystem, and firewall traces stop at 16 jumps. Crash files rotate under `ze.crash.keep`. Regular daemon logs go only to `stderr`, `stdout`, `syslog` or `kmsg` (with a 4 MiB non-blocking `stderr` relay), never to unbounded files. Other dataplane engines (`vpp`, `ipsec`, `mpls`) can plug into the same drift callback once their live readback APIs are ready. Live push notifications over MCP are on the roadmap.

A lab can only take a router design so far, because live networks always turn up failure modes nobody thought to simulate. If you run routers in production and spot a blind spot, a missing signal or a limit that feels wrong, please come tell us on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues). Your war stories from real networks are the best help you can give us to make Ze safer and easier to live with.

*Originally written: 27 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/fault-isolation-and-live-diagnostics.md).*
