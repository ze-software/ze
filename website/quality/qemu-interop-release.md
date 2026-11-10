# QEMU, Interop, and Release Evidence

Some behavior can only be checked with a Linux kernel, another routing daemon, or a service outside Ze. This page explains the QEMU, Docker, live-data, and performance tests used for those cases, and how their results contribute to release verification.

<!-- source: ../main/ai/META-DECISIONS.md -- Test systems and labs -->
<!-- source: ../main/internal/le/qemu/actions.go -- native QEMU actions -->
<!-- source: ../main/internal/le/qemu/run.go -- host QEMU runner -->
<!-- source: ../main/internal/le/qemu/alltests.go -- complete guest suite -->
<!-- source: ../main/internal/le/integration/gates.go -- interop and live actions -->
<!-- source: ../main/internal/le/evidence/actions.go -- release evidence action -->

Day-to-day changes use the tests that exercise the affected behavior. When that behavior depends on Linux or another implementation, the relevant QEMU or interoperability test is part of that work. Full verification runs in GitHub CI on pushed commits and during release checks.

## When QEMU is required

On a non-Linux host, QEMU provides the environment for tests that depend on Linux behavior. That includes netlink, nftables, network namespaces, veth pairs, PPP, L2TP, eBPF, kernel route state, raw sockets, Linux-only build tags, and tests marked `option=needs-linux`.

<table>
<thead><tr><th>Need</th><th>Command</th></tr></thead>
<tbody>
<tr><td>Run curated Linux-only functional files</td><td><code>./le test qemu netns</code></td></tr>
<tr><td>Run the full suite inside a prepared guest</td><td><code>./le test qemu run command '&lt;guest-le&gt; le test qemu all'</code></td></tr>
<tr><td>Rerun one failing command inside the VM</td><td><code>./le test qemu run command '...'</code></td></tr>
<tr><td>Keep a VM alive for manual inspection</td><td><code>./le test qemu run command '...' keep-alive</code></td></tr>
</tbody>
</table>

The QEMU guest uses Ze's kernel and system image. It provides Linux features that Docker Desktop on macOS cannot provide reliably. During a debug run, you can inspect `ip`, `nft`, `dmesg`, temporary files, process state, and generated logs inside the guest.

## Linux-only `.ci` files

A functional file that needs Linux says so explicitly:

```text
option=needs-linux
```

On macOS, the functional runner reports that test as skipped. The same file runs inside the Linux QEMU guest, so its assertions can be checked on either a Linux host or a Mac with QEMU.

## Integration Go tests

Go integration tests that require Linux use the `integration` and `linux` tags. The QEMU integration targets run these tests inside the guest.

```go
//go:build integration && linux
```

This format suits a kernel API wrapper or package integration test. A `.ci` transcript is useful when the test must start a daemon, apply configuration, or run an operator command.

## Docker interop

Docker interop proves protocol behavior against real implementations. Ze runs BGP scenarios against FRR, BIRD, GoBGP, OpenBGPD, RustyBGP, freeRouter, and related peers. Other labs cover IPsec, L2TP, PPPoE, deployment, and live checks where the target behavior depends on an external daemon or service.

<table>
<thead><tr><th>Evidence</th><th>Command</th><th>What it proves</th></tr></thead>
<tbody>
<tr><td>BGP interop</td><td><code>./le test integration interop</code></td><td>Ze exchanges real protocol messages with third-party BGP daemons.</td></tr>
<tr><td>IPsec interop</td><td><code>./le test integration interop-ipsec</code></td><td>strongSwan and Ze agree on the deployed behavior.</td></tr>
<tr><td>L2TP and PPPoE</td><td><code>./le test deployment docker-l2tp-ppp</code>, <code>./le test deployment docker-pppoe-accel</code></td><td>Access protocol behavior works against real peers.</td></tr>
<tr><td>Deployment evidence</td><td><code>./le test deployment l2tp</code>, <code>./le test deployment vpp</code></td><td>Exercises the deployment scripts with the services they configure.</td></tr>
</tbody>
</table>

A `.ci` transcript can isolate a Ze behavior and make its failure easy to reproduce. An interoperability test adds another implementation, which can expose a protocol mistake shared by Ze and its own test peer. Both kinds of test are useful.

## Performance and live evidence

Performance gates are used when a change can regress throughput, convergence, or data-plane behavior. Live evidence is used when the contract includes external data, such as RPKI cache behavior. These checks are not default verify steps because they depend on time, host capacity, Docker, root privileges, or the internet.

```bash
./le perf record
./le test integration live-rpki
```

## Release evidence

Release evidence runs verification over a clean checkout in a container. The
native action checks its prerequisites before it starts the matrix.

```bash
./le verify evidence release-candidate
```

Release evidence supports claims across the system. For a single failure, the useful starting point is the test that reproduces it; a wider environment is needed when the behavior depends on it.
