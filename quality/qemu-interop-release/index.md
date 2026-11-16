# QEMU, Interop, and Release Evidence

Some behavior can only be checked with a Linux kernel, another routing daemon, or a service outside Ze. This page explains the QEMU, Docker, live-data, and performance tests used for those cases, and how their results contribute to release verification.

Day-to-day changes use the tests that exercise the affected behavior. When that behavior depends on Linux or another implementation, the relevant QEMU or interoperability test is part of that work. Full verification runs in GitHub CI on pushed commits and during release checks.

## When QEMU is required

On a non-Linux host, QEMU provides the environment for tests that depend on Linux behavior. That includes netlink, nftables, network namespaces, veth pairs, PPP, L2TP, eBPF, kernel route state, raw sockets, Linux-only build tags, and tests marked `option=needs-linux`.

| Need | Command |
| --- | --- |
| Run curated Linux-only functional files | `./le test qemu netns` |
| Run the full suite inside a prepared guest | `./le test qemu run command '<guest-le> le test qemu all'` |
| Rerun one failing command inside the VM | `./le test qemu run command '...'` |
| Keep a VM alive for manual inspection | `./le test qemu run command '...' keep-alive` |

The QEMU guest uses Ze's kernel and system image. It provides Linux features that Docker Desktop on macOS cannot provide reliably. During a debug run, you can inspect `ip`, `nft`, `dmesg`, temporary files, process state, and generated logs inside the guest.

## Linux-only `.ci` files

A functional file that needs Linux says so explicitly:

```
option=needs-linux
```

On macOS, the functional runner reports that test as skipped. The same file runs inside the Linux QEMU guest, so its assertions can be checked on either a Linux host or a Mac with QEMU.

## Integration Go tests

Go integration tests that require Linux use the `integration` and `linux` tags. The QEMU integration targets run these tests inside the guest.

```
//go:build integration && linux
```

This format suits a kernel API wrapper or package integration test. A `.ci` transcript is useful when the test must start a daemon, apply configuration, or run an operator command.

## Docker interop

Docker interop tests protocol behavior against real implementations. Ze runs BGP scenarios against FRR, BIRD, GoBGP, OpenBGPD, RustyBGP, freeRouter, and related peers. Other labs cover IPsec, L2TP, PPPoE, deployment, and live checks where the target behavior depends on an external daemon or service.

| Evidence | Command | What it tests |
| --- | --- | --- |
| BGP interop | `./le test integration interop` | Ze exchanges real protocol messages with third-party BGP daemons. |
| IPsec interop | `./le test integration interop-ipsec` | strongSwan and Ze agree on the deployed behavior. |
| L2TP and PPPoE | `./le test deployment docker-l2tp-ppp`, `./le test deployment docker-pppoe-accel` | Access protocol behavior works against real peers. |
| Deployment evidence | `./le test deployment l2tp`, `./le test deployment vpp` | Exercises the deployment scripts with the services they configure. |

A `.ci` transcript can isolate a Ze behavior and make its failure easy to reproduce. An interoperability test adds another implementation, which can expose a protocol mistake shared by Ze and its own test peer. Both kinds of test are useful.

## Performance and live evidence

Performance checks are used when a change can reduce throughput, slow convergence, or affect packet forwarding. Tests with live services are used when the behavior depends on external data, such as an RPKI cache. These checks are separate from the default verification run because they depend on time, host capacity, Docker, root privileges, or the internet.

```
./le perf record
./le test integration live-rpki
```

## Release evidence

Release evidence runs verification over a clean checkout in a container. The command checks its prerequisites before it starts testing each supported build.

```
./le verify evidence release-candidate
```

Release evidence supports claims across the system. For a single failure, the useful starting point is the test that reproduces it; a wider environment is needed when the behavior depends on it.
