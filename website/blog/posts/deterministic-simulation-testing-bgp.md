---
category: observe
topic: testing-and-verification
title: Deterministic simulation testing for BGP
date: 2026-11-27
author: Thomas Mangin
description: How Ze adapts TigerBeetle VOPR-style swarm fuzzing, wire-level stream faults, white-box reactor checks and automatic shrinking for BGP in Go, and where standard Go differs from a deterministic hypervisor like Antithesis.

deck: Running one chaos seed shows what happened under one fault mix; deterministic simulation lets a single machine explore many combinations and shrink any failure to its smallest cause.

image: assets/blog/deterministic-simulation-testing-bgp.svg
image-dark: assets/blog/deterministic-simulation-testing-bgp-dark.svg
image-alt: Three-stage deterministic BGP simulation pipeline showing swarm seed profiles, sub-header wire stream faults, live white-box reactor invariant checks, and two-level automatic shrinking to a minimal seed reproducer.
---

In [Breaking BGP on purpose with Le Chaos](../le-chaos/), we discussed Le Chaos, our seeded chaos runner that drives real and in-process BGP sessions under seventeen fault types. Choosing one seed and one set of fault weights still tests only the scenario someone thought to configure, whereas the defects that reach production networks usually appear where several protocol features and transport failures collide.

Drawing on TigerBeetle's VOPR simulator and deterministic hypervisors such as Antithesis pushed Ze's testing beyond individual fault runs toward multi-seed swarm exploration and white-box reactor checks, and also showed where standard Go stops short of full determinism.

*This article was drafted with Gemini. The architecture, design decisions and conclusions are mine. Models helped with implementation, testing and drafting the text.*

## Two ways to make distributed systems deterministic

Distributed systems teams have approached deterministic fault injection from opposite ends of the stack.

The first approach lives inside the application. FoundationDB pioneered it with a single-threaded simulation runtime, and [TigerBeetle's VOPR](https://github.com/tigerbeetle/tigerbeetle) (playable in a browser at [sim.tigerbeetle.com](https://sim.tigerbeetle.com)) carries the same idea into a modern financial database. The simulator runs an entire cluster inside one single-threaded process, replacing clocks, network I/O and disk storage with deterministic doubles driven by a pseudorandom seed. Because no real sleeps or OS thread schedules intervene, days of network partitions, packet reordering and storage corruption compress into minutes, and any failure replays from its seed and commit hash.

The second approach puts determinism underneath unmodified binaries. In talks hosted alongside the TigerBeetle community, Will Wilson described [Antithesis](https://antithesis.com/), which adapts FreeBSD's `bhyve` into a deterministic hypervisor so existing multi-process software can run inside a controlled virtual machine with deterministic scheduling, virtual time and injected network or hardware faults. Alex Pshenichkin later detailed the `bhyve` hypervisor work at the FreeBSD Developer Summit. Meta's [Hermit](https://github.com/facebookexperimental/hermit) explored a related boundary at the Linux syscall layer, and Polar Signals tested Go's [FrostDB](https://www.polarsignals.com/blog/posts/2024/05/28/mostly-dst-in-go) by compiling to single-threaded WebAssembly with a patched Go runtime and fake time.

Ze is written in standard Go, with concurrent goroutines for peer sessions, listeners, plugins and the forward pool. We did not want to rewrite that engine into a single-threaded event loop or maintain a patched Go runtime, yet we still needed fast local fault exploration inside `./le`. Because Ze's BGP reactor already accepts an injected clock, dialer and listener factory, we brought the application-level simulation techniques that fit those seams directly into `le chaos` and left the boundary with Go's goroutine scheduler explicit.

## Fault injection belongs below the protocol frame

Protocol-level faults such as route flaps, hold-timer expiry and malformed UPDATEs exercise BGP at the message boundary. In-memory pipes normally deliver complete framed writes from peer to reactor, leaving the byte-stream parser and partial-write paths cleaner than they ever are on a real TCP socket. On a live network, packets split across the 19-byte BGP header boundary, asymmetric links stall in one direction while keepalives keep arriving in the other, and peers drop connections halfway through transmitting an UPDATE.

To exercise those paths deterministically inside in-process runs, a [fault-injecting stream wrapper](https://github.com/ze-software/ze/blob/main/internal/chaos/mocknet/fault_conn.go) sits on both ends of every simulated connection, driven by per-connection PCG random streams derived from the run seed. It splits reads and writes into sub-header fragments of at most 19 bytes, stalls outbound traffic on the virtual clock to reproduce half-open blackholes without real sleeps, and writes truncated frames before abruptly resetting the connection. Once the active fault window ends, the wrapper pauses new write-side faults so recovering peers and in-flight updates can finish converging before observation closes.

## Why one average fault mix hides bugs

Testing with every fault switched on at a modest probability sounds thorough, yet Alex Groce and colleagues showed in [Swarm Testing](https://www.cs.utah.edu/~regehr/papers/swarm12.pdf) why that can hide defects. When every disruption fires at once, aggressive disconnects interrupt quieter state transitions before they finish; turning one fault class off lets another feature reach deeper states. Sweeping many seeds where each seed activates a different subset of features and fault rates explores extreme combinations that an average configuration never reaches.

Ze's [swarm fuzzer](https://github.com/ze-software/ze/blob/main/internal/chaos/fuzz/swarm.go) derives a distinct scenario profile from each seed, varying peer counts, eBGP and iBGP mixes, IPv4 and IPv6 unicast alongside multicast families, RFC 4724 Graceful Restart negotiation, RFC 7606 malformed UPDATEs, route churn and wire-stream fault rates. Campaigns can sweep a fixed number of seeds or run for a time budget across parallel workers, compressing hours of virtual BGP time into seconds on a local machine and turning any failure into a single reproducible seed.

Checking that two independent runs of the same seed reach the same settled state hash immediately exposed wall-clock entropy inside our own peer simulator. Two chaos helpers still mixed system time into their random seeds, and several simulated actions still waited on real wall-clock timers rather than the virtual clock. Replacing those with deterministic per-peer sequence numbers and virtual-clock waits made the simulated peers repeat the same route selections and timing across replays.

## Checking invariants inside the router

External wire checks catch dropped routes and slow convergence, as described in [The proof is the expensive part](../the-proof-is-the-expensive-part/), yet a router can deliver every expected route on the wire while breaking internal invariants. An illegal state-machine jump, an out-of-range negotiated timer or a leaked read buffer after a storm of mid-frame resets will survive a wire-only test untouched.

Ze's [online state checker](https://github.com/ze-software/ze/blob/main/internal/chaos/inprocess/state_checker.go) inspects the live reactor directly on every virtual tick and again at settle. On each sample, it checks every peer's recorded FSM transition history against the legal edges of RFC 4271 section 8.2.2, flagging impossible jumps such as moving straight from `IDLE` to `ESTABLISHED`. For every established peer, it verifies a non-zero remote router ID and legal negotiated hold and keepalive timers under RFC 4271 section 4.2, and it tracks per-peer lifetime counters across ticks so a counter regression fails the run immediately.

At settle, the checker queries the live RIB plugin to confirm it is still running with valid route counts and hashes sorted peer states, negotiated hold times and remote router IDs into a deterministic checkpoint digest. After all workers tear down their reactors, a leak check compares Ze's shared 4K and 64K read-buffer pools against the pre-run baseline so leaked buffer references fail the sweep.

Running the fuzzer early on also revealed a subtle boundary mistake in the harness itself. Stopping chaos injection and cutting off observation the instant a reconnecting peer reached `ESTABLISHED` caught the route server mid-forward, producing false route-consistency failures on healthy code. Adding a bounded settle window with chaos ticks and stream write faults paused lets legitimately in-flight updates converge, while a genuinely lost route still times out and fails the property.

## Reducing a failing run to its smallest cause

Catching a failure on an eight-peer, three-family run with Graceful Restart, malformed UPDATEs and wire resets all active leaves an engineer facing too many moving parts. Diagnosing the bug quickly depends on stripping away everything the failure did not need.

Ze's [automatic shrinker](https://github.com/ze-software/ze/blob/main/internal/chaos/fuzz/shrink.go) minimises failures at two levels. First, it probes progressively simpler scenario profiles against the live reactor, removing stream resets, blackholes, fragmentation, Graceful Restart, individual chaos actions and extra address families one by one, then binary-searching peer and route counts down toward the smallest configuration that still fails. Second, it shrinks the pre-cutoff event trace to the shortest sequence of causally linked events that violates the property. What starts as a noisy multi-peer sweep ends as a minimal reproducer small enough to trace directly in the BGP engine.

## Where standard Go stops short

Controlling the clock, the network pipes, the peer PRNGs and the stream faults goes a long way in plain Go, though it still leaves goroutine scheduling to the Go runtime.

Ze's reactor spawns real Go goroutines for peer loops, listeners, plugin IPC and the forward pool, and Go's scheduler chooses freely among runnable goroutines and ready `select` cases. Replaying a seed therefore verifies deterministic outcomes and settled checkpoint hashes, alongside every FSM transition observed along the way, while a race that depends on one exact goroutine interleaving between a socket read, a hold timer and a configuration reload can still fire on one run and miss on the next.

Reproducing exact goroutine interleavings requires either a replay-only event queue and scheduler seam inside the reactor (tracked as a deferred engineering spec in the repository so production hot paths stay untouched until we decide on its scope) or running the unmodified Ze binary inside a deterministic hypervisor such as Antithesis. Keeping that boundary plain lets us run fast multi-seed swarm campaigns locally today, with the command syntax and flags documented in the [chaos testing guide](../../guides/chaos-testing/).

We did report a bug in the Go runtime recently ([golang/go#83101](https://github.com/golang/go/issues/83101)), alongside a fix, though I do not believe we are ready to submit patches to bring deterministic scheduling into Go itself. That sits well beyond my skills, and if an AI can write it, the Go team can run one themselves.
