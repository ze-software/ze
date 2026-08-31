# QEMU, Interop, and Release Evidence

Use this page when the proof needs a Linux kernel, a real peer daemon, Docker, QEMU, internet data, performance evidence, or the full release matrix. These checks are slower because they leave the local unit-test world and exercise the environment Ze is meant to run in.

`./le verify current mode full` stays local on purpose. It should be fast enough to run often and safe enough for a normal developer machine. QEMU and interop jobs are the next layer when a local run would skip the real contract.

## When QEMU is required

QEMU is required for code that depends on Linux behavior rather than Go behavior. That includes netlink, nftables, network namespaces, veth pairs, PPP, L2TP, eBPF, kernel route state, raw sockets, Linux-only build tags, and tests marked `option=needs-linux`.

| Need | Command |
| --- | --- |
| Run curated Linux-only functional files | `./le qemu netns-test` |
| Run the full suite inside a prepared guest | `./le qemu run command '<guest-le> le qemu all-tests'` |
| Rerun one failing command inside the VM | `./le qemu run command '...'` |
| Keep a VM alive for manual inspection | `./le qemu run command '...' keep-alive` |

The runner boots Alpine from an ISO, mounts the repository over 9p, installs the needed packages, and runs the requested command inside the VM. The VM has Linux capabilities that Docker Desktop on macOS cannot provide reliably. A debug run is the right place to inspect `ip`, `nft`, `dmesg`, temporary files, process state, and generated logs.

## Linux-only `.ci` files

A functional file that needs Linux says so explicitly:

```
option=needs-linux
```

On Darwin, the functional runner reports that test as skipped. In QEMU, the same file runs. This keeps the normal verify gate honest while still making the Linux proof available on the same source file.

## Integration Go tests

Go tests that require Linux are built with `integration` and `linux` tags. They run in QEMU through the integration targets, not through the normal unit suite.

```
//go:build integration && linux
```

Use this shape when the test is naturally a Go test, such as a kernel API wrapper or package-level integration. Use `.ci` when the proof is a daemon, command, config, or observable operator flow.

## Docker interop

Docker interop proves protocol behavior against real implementations. Ze runs BGP scenarios against FRR, BIRD, GoBGP, OpenBGPD, RustyBGP, freeRouter, and related peers. Other labs cover IPsec, L2TP, PPPoE, deployment, and live checks where the target behavior depends on an external daemon or service.

| Evidence | Command | What it proves |
| --- | --- | --- |
| BGP interop | `./le integration interop` | Ze exchanges real protocol messages with third-party BGP daemons. |
| IPsec interop | `./le integration interop-ipsec` | strongSwan and Ze agree on the deployed behavior. |
| L2TP and PPPoE | `./le deployment docker-l2tp-ppp-test`, `./le deployment docker-pppoe-accel-test` | Access protocol behavior works against real peers. |
| Deployment evidence | `./le deployment l2tp-test`, `./le deployment vpp-test` | Deployment paths are not just unit-tested scripts. |

Interop tests are not a replacement for functional transcripts. A `.ci` test explains a Ze behavior precisely and cheaply. Interop proves that the behavior still works when another implementation interprets the protocol.

## Performance and live evidence

Performance gates are used when a change can regress throughput, convergence, or data-plane behavior. Live evidence is used when the contract includes external data, such as RPKI cache behavior. These checks are not default verify steps because they depend on time, host capacity, Docker, root privileges, or the internet.

```
./le perf-bench record
./le integration live-rpki
```

## Release evidence

Release evidence runs verification over a clean checkout in a container. The native action checks its prerequisites before it starts the matrix.

```
./le evidence release-candidate
```

Use release evidence when claiming broad coverage, not when debugging a single change. For a single failure, start from the narrow target that reproduces it and move outward only when the contract requires a wider environment.
