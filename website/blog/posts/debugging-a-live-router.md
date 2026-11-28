---
category: observe
topic: architecture-and-performance
title: Debugging a live router at two in the morning
date: 2026-11-27
author: Thomas Mangin
description: How Ze gives operators three simple debugging commands and lets an AI assistant narrow down an outage over CLI or MCP without guessing through logs.

deck: Three short commands for a tired engineer, and structured drill-down tools so an AI assistant can help narrow an outage down before anyone reboots the box.

image: assets/blog/debugging-a-live-router.svg
image-dark: assets/blog/debugging-a-live-router-dark.svg
image-alt: Three-stage live debugging flow showing three human entry commands (show system triage, show doctor, request support archive), a read-only AI elimination pass over CLI or MCP, and deterministic diagnosis or redacted support archiving without guessing on missing data.

---

When a production router misbehaves and causes an outage, the hardest part is rarely typing the fix. It also rarely happens at eleven in the morning after a good night's sleep. Outages tend to arrive at two in the morning when you are tired, may have been out earlier and are far from your sharpest. Under that pressure, four hundred other BGP sessions are still carrying traffic and colleagues are understandably asking for an ETA before you even know which protocol is unhappy.

Once you have debugged a router that stays up and tells you what broke, going back to an ordinary one is hard. You miss having the evidence already waiting in memory, and you miss having an AI assistant that can help narrow the fault down while you work out the fix.

*This article was co-authored with an AI assistant. The architecture, design decisions and conclusions are mine. The assistant helped organise the material and draft the text.*

## Three commands for you, full depth for an AI

Nobody wants to memorise fifteen debugging commands during an outage. On Ze, you only need to know three.

```sh
show system triage
show doctor
request support archive
```

`show system triage` prints one summary of what is wrong right now: health, errors, crashes, unhealthy plugins, recent reloads, watchdog alerts and bad packets. Every finding includes a `fix` line naming the next command to run. `show doctor` checks configuration, runtime health and kernel drift, then prints clear next steps. `request support archive` is Ze's structured equivalent of `show tech-support`, gathering host details, configuration, logs and live runtime state into one archive with no flags needed.

Behind those three commands sit deeper tools for crash traces, bad wire packets, plugin goroutines, reload timelines, kernel drift and firewall decisions. You never have to memorise them. Tab completion and `fix` hints lead you there if you dig in by hand, and an AI assistant uses them automatically on your behalf.

The `ze create agent skill` command installs a short entry guide (called a skill in tools such as Claude Code or Codex) that points the assistant at `ze show skill ze` and `ze show skill ze-diagnostics`. When an assistant connects over the Model Context Protocol (MCP), Ze steers it straight to the same two places: read `ze://triage` (the live MCP resource for `show system triage`) and load `show skill ze-diagnostics`. Every CLI command below prints readable text in a terminal and returns the same data as JSON when you add `| json` or call it over MCP.

## Starting with one triage screen

Say a peer (`198.51.100.7`) keeps bouncing, or an alert fires after a change. Instead of opening six terminals, you or your AI assistant start with `show system triage` (abridged JSON below).

```json
{
  "version": { "release": "0.1.0", "commit": "e37266f20e" },
  "doctor": {
    "available": true,
    "diagnostics": [
      {
        "code": "doctor-internal-defect",
        "severity": "error",
        "fix": "ze show crashes name crash-20261127-221403.112000000-recovered.log"
      }
    ]
  },
  "crashes": {
    "crashes": [
      { "name": "crash-20261127-221403.112000000-recovered.log", "kind": "recovered" }
    ],
    "count": 1
  },
  "recent-logs": {
    "entries": [
      { "level": "error", "component": "bgp", "message": "recovered panic in bgp read loop" }
    ]
  },
  "runtime": {
    "watchdog-running": true,
    "stall-count": 0,
    "hung-handler-count": 0,
    "bad-packet-count": 1
  }
}
```

The recent logs come from a dedicated warning-and-error ring that routine `info` messages cannot push out. If any part of the router cannot answer, that section returns `available: false` instead of an empty list. Neither a tired human nor an AI can mistake missing data for a healthy box.

Two short commands then pull the crash stack and the exact BGP packet that triggered it, without waiting on `tcpdump`:

```sh
show crashes name crash-20261127-221403.112000000-recovered.log
show system bad-packets | protocol bgp
```

The second command prints the peer IP, message type, capture time, what Ze did with the packet, the reason and the raw hex bytes.

```json
{
  "packets": [
    {
      "id": "pkt-1",
      "captured-at": "2026-11-27T22:14:03Z",
      "protocol": "bgp",
      "peer": "198.51.100.7",
      "message-type": "UPDATE",
      "action": "task-panic",
      "reason": "runtime error: slice bounds out of range",
      "length": 64,
      "hex": "ffffffffffffffffffffffffffffffff004002..."
    }
  ],
  "count": 1,
  "total-recorded": 1,
  "capacity": 16
}
```

## Following one bad configuration reload

Every operator knows the familiar dread of committing a change on a remote router and watching the CLI freeze for three seconds. For a brief moment you wonder whether the reload is slow or you have just cut off your own SSH session and booked a drive to the datacentre. That moment is why network engineers love `commit confirmed` so much ([One configuration engine, many protocols](../one-engine-any-plugin/)).

Even when the prompt returns, a failed or slow reload on an ordinary router leaves clues scattered across syslog, audit logs and error messages. Lining up timestamps by eye while live routing chatter scrolls past wastes time.

Ze tags every command and reload with a short ID, such as `op-42`, and writes that ID into its logs, events, errors, audit trail and reload history. `show system reload history` lists the last 32 reloads, whether each one succeeded or rolled back, how long each plugin took and its `operation-id`. Say `firewall` rejected the reload under `op-42`. Adding `| operation op-42` filters every other view to that one reload.

```sh
show log recent | operation op-42
show event recent | operation op-42
show errors | operation op-42
show audit | operation op-42
```

Over MCP, the assistant sends the same filter as structured data (`pipe: [{"name": "operation", "args": ["op-42"]}]`). Every unrelated peer event disappears, leaving one clean timeline from commit to rollback.

## Checking silent drift and blocked traffic

The most frustrating incidents produce no panics and no error logs. A customer reports that HTTPS to a server is timing out, or routes stop matching policy. Two days earlier, someone flushed an nftables table or deleted a kernel route by hand. Linux offers `nft monitor trace` and `ip route get`, yet both need shell access, and live `nftrace` means adding trace rules on a busy router.

Ze gives you two read-only checks over CLI and MCP.

First, [`show system drift`](https://github.com/ze-software/ze/blob/main/docs/architecture/diagnostics/production-diagnostics.md) asks the firewall, kernel route (`fib-kernel`) and interface plugins to compare what Ze intends against what is programmed in the kernel, without changing a rule or route. `show doctor` runs the same check under `doctor-dataplane-drift`. If someone flushed one of Ze's `ze_`-prefixed nftables tables by hand, one command spots it.

```sh
show system drift | subsystem firewall
```

```json
{
  "status": "drifted",
  "subsystems": [
    {
      "subsystem": "firewall",
      "status": "drifted",
      "checked": 3,
      "drifted": 1,
      "findings": [
        {
          "subject": "inet-edge",
          "kind": "missing",
          "expected": "present",
          "actual": "absent"
        }
      ]
    }
  ],
  "count": 1,
  "drifted-count": 1
}
```

Second, suppose the kernel matches the configuration and you want to know why traffic dropped. `show firewall trace` tests a synthetic packet offline against Ze's applied firewall model (Linux nftables or VPP) without touching the dataplane.

```sh
show firewall trace table inet-edge protocol tcp source-address 198.51.100.10 destination-address 203.0.113.5 destination-port 443
```

The output walks every chain, jump and term tried. It reports the matched term, the verdict (`accept`, `drop`, `reject` or NAT action) and whether the packet fell through to the chain's default policy. Some rules check live kernel state or packet fields the trace command does not accept yet, such as `destination-local`, IPsec transforms or `tcp-flags`. For those steps, Ze marks `unevaluated: true` and sets `incomplete: true` instead of guessing. For BGP, the same workflow works through `show bgp rib best | reason` and `show policy test`.

## When everyone wants an ETA and you are staring at a TAC ticket

Every network engineer has lived through the worst version of an outage. Service is down and you genuinely do not understand why. Colleagues keep asking when customers will be back online, which is fair because customers are asking them too. At that point, opening a vendor TAC case often feels like the only option left.

When you cannot spot the cause, the default move is to dump everything the router can print (`show tech` and several terminal captures). You upload the whole pile to TAC, and then reboot. The needle is buried somewhere in that haystack. When a support engineer picks up the ticket, the first reply often asks for outputs from a log buffer that rolled over twenty minutes earlier or vanished on reboot. Worse, an hour later you sometimes find out the outage came from a silly misconfiguration that nobody spotted.

What you wanted in that moment was a senior colleague with experience sitting beside you. Someone to narrow the problem down by dichotomy, halving the search space step by step until you find the cause. First rule out config versus kernel, then the core daemon versus one plugin, then a missing table versus a shadowed rule.

Ze's diagnostic commands and built-in `ze-diagnostics` skill let an AI assistant run that read-only elimination pass. It starts from `show system triage`, rules out healthy subsystems and drills down only along the branch that failed. When the cause is a silly misconfiguration, `show firewall trace`, `show bgp rib best | reason`, `show policy test` or `show config validation` with `ze config fix --plan` points straight at the shadowed rule, inverted prefix or bad config line. The deterministic commands prove what went wrong, and you review and apply the fix in minutes with no TAC ticket opened.

Sometimes that elimination shows a real bug in Ze or a failure nobody recognises. In that case, the router keeps enough evidence so you can restore service first and help us fix the bug afterwards. Once service is stable, `request support archive` ([`internal/core/supportreg`](https://github.com/ze-software/ze/blob/main/internal/core/supportreg/registry.go)) packages everything we need in one command with no arguments.

```sh
request support archive
```

It produces redacted JSON covering host details, configuration and live runtime state. That includes recent error logs, active warnings, health, watchdog records, bad packets, kernel drift, goroutine stacks, events, plugin state and reload history. Remote callers over MCP, SSH, REST or gRPC have no local path the daemon can write to. Ze saves the archive in its internal store under `support/archive/<name>` and returns an `archive-key` you can fetch with `show data cat <key>`.

## Designing for real operations

[How Ze isolates faults and keeps diagnostics alive](../fault-isolation-and-live-diagnostics/) explains how Ze keeps the router running during a fault and keeps these commands responsive when a lock or plugin hangs.

Tools that feel natural at two in the morning are hard to get right from a lab alone, so we could really use your help. If you have a spare lab instance, try running `ze create agent skill`, breaking a table or route on purpose and asking your AI assistant what changed. If the output confuses you or misses something you look for during an outage, please come tell us on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues). Your day-to-day experience is what makes Ze better.

*Originally written: 27 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/debugging-a-live-router.md).*
