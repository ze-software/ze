Topology lab

# netlab Topology Lab

netlab starts three Ze nodes under containerlab from the supplied topology. A live check tests the routing protocols, the kernel routes and the traffic between them, then tears the lab down.

`Daemon`

The reference topology in `contrib/netlab/topology.yml` has three nodes. r1 and r2 share an internal link in AS 65001 and run iBGP, OSPF, IS-IS and BFD. r3, in AS 65002, is an eBGP peer of r1 on its own link, because netlab runs an IGP on internal links only. r1 also carries two static routes.

To start the same topology by hand with netlab, check it, rebuild it and remove it, follow the [netlab guide](https://ze-software.net/guides/netlab/).

- **What this lab tests:** Every declared BGP, OSPF, IS-IS and BFD adjacency, both static routes in the kernel FIB, and sourced loopback pings between every pair of nodes, when `./le test netlab run` exits 0 after teardown
- **Environment:** Three Ze nodes under containerlab on a Docker host; netlab and containerlab run in a tool container
- **Requires:** Docker access to a Ze Docker host, or QEMU for the disposable Ze Docker host
- **Source:** [contrib/netlab/](https://github.com/ze-software/ze/tree/main/contrib/netlab)

```
# live check against a Ze Docker host
$ ./le test netlab run

# the same check in the disposable QEMU Docker host
$ ./le test qemu docker-lab lab "test netlab run"

# render the templates and compare with the golden files;
# this one needs netlab installed on the machine that runs it
$ ./le test netlab check
```

`Prerequisites`

Docker access from a Ze source checkout. For the two live runs, Python, netlab and containerlab run in the tool container. The template check is different: it runs the netlab installed on the machine that invokes it, or the one named by `NETLAB=/path/to/netlab`, and a missing netlab is an error.

- [internal/le/test/netlab/live.go live check](https://github.com/ze-software/ze/blob/main/internal/le/test/netlab/live.go)
- [contrib/netlab/ daemon definition and templates](https://github.com/ze-software/ze/tree/main/contrib/netlab)

## What has been tested.

The render check covers template rendering and parsing. The functional profile check covers one daemon's SSH login. The live command checks adjacencies, kernel routes and traffic across the topology; it must finish successfully before those properties can be claimed for a run.

| Statement | Evidence |
| --- | --- |
| netlab accepts the daemon definition and finds a template for each module | `netlab create` exits 0 on the reference topology, in `./le test netlab check` |
| The render is valid ze configuration | `ze config validate` exits 0 on each file under `contrib/netlab/golden/` |
| A daemon runs one of those renders and answers the show command with JSON | `test/plugin/netlab-lab-profile.ci` |
| Every declared protocol reaches its healthy state, static routes reach the kernel FIB, and sourced loopback pings pass | `./le test netlab run` exits 0 after teardown |
| Each declared feature passes netlab's own upstream integration suite | Not verified: no run here executes netlab's upstream integration suite. This is separate from the native reference-lab test |

The live command passed on 2026-11-14 on arm64, in Ze's QEMU Docker host under HVF, with netlab 26.10 and the committed templates. It reached every BGP, OSPF, IS-IS and BFD adjacency, found both static routes in the kernel FIB, passed all six sourced loopback pings, and tore the lab down with no container left. No amd64 run from the committed templates is recorded.

Ze also sends and receives no LLDP frame, so a netlab validation that reads LLDP data cannot pass. containerlab does not need LLDP, because the topology names the veth links.

## Run the automated live check.

To run the automated live checks from the source checkout against an existing Ze Docker host, run:

```
./le test netlab run
```

The automated run stages the node binary once for the Docker daemon's architecture, then packages it with the `lab` target in `docker/Dockerfile.runtime`. Its node build context contains only that binary and the runtime recipe; it does not compile Go inside Docker or send the source checkout. The manual source-building command in the [netlab guide](https://ze-software.net/guides/netlab/#step-1-build-the-image) remains available.

If Docker currently uses the distribution's kernel, use the disposable QEMU Docker host instead. This boots Ze's own system, forwards its Docker endpoint, and runs the same lab against it:

```
./le test qemu docker-lab lab "test netlab run"
```

The QEMU route needs QEMU and the native-architecture Ze Docker-host image; its first run builds missing image and kernel inputs. Building the Docker-host image from scratch also needs the Docker CLI and a reachable build daemon for its static administration tools. A native kernel build needs Docker or QEMU and network access to its pinned sources. See the [QEMU integration guide](https://github.com/ze-software/ze/blob/main/docs/architecture/testing/qemu-integration.md).

The command builds the local lab image and a tool image with the versions the [netlab guide](https://ze-software.net/guides/netlab/#what-you-need) lists. The tool build prints its Python and tool versions and checks Python dependency compatibility.

The host needs Docker access; Python, netlab and containerlab run in the tool container, and Ansible configures the nodes from it. That container shares the Docker host's network and PID namespaces, which also supports Ze's QEMU Docker host transport. Running host containerlab against a remote Docker socket alone cannot provide those namespaces.

The runner waits for every declared BGP, OSPF, IS-IS and BFD adjacency, verifies both static routes in the kernel FIB, and sends sourced loopback pings between every pair of nodes. It tears the topology down on success and failure, and a cleanup failure fails the command. Artifacts stay in session scratch for diagnosis. Each run binds its nodes to the image tag retained for that build and verifies that tag still resolves to the built image ID before startup. A host-runtime run holds an exclusive directory lease and refuses an existing deployment, so a second invocation cannot tear down another run's topology.

## When a run fails.

**Files saved when a test fails**

Before cleanup, a failed test run saves a private `diagnostics-*` directory inside the retained lab directory. It contains startup output, the topology snapshot, generated node files, Docker inspection, and each node's last 200 log lines. It also saves addresses, kernel routes, BGP and BFD state, OSPF and IS-IS interfaces, neighbors, databases and routes, the system RIB, and plugin state. Address, kernel route and adjacency queries run across every node before the more detailed routing queries. Capture has a two-minute total deadline and a five-second deadline for each command. A failed query leaves an `.error.txt` artifact with its command, failure and process output, so unavailable state cannot appear as an empty successful result. Cleanup still runs after capture. JSON artifacts keep stdout and use private `.stderr.txt` sidecars for stderr. The runner explicitly adds `netlab.snapshot.yml` to netlab's normal outputs before startup. netlab 26.10 writes a pickle snapshot by default; the added YAML inventory supplies node names for live checks, failure capture, and cleanup.

## Keeping the templates from drifting.

```
./le test netlab check
```

It renders the templates with a real netlab, compares the result with `contrib/netlab/golden/`, and runs `ze config validate` on each golden file. A missing netlab is an error exit, never a skip. Use `./le test netlab update` to rewrite the golden files. `test/plugin/netlab-lab-profile.ci` is the other half and needs no netlab: it starts a daemon from a golden file and parses the show command output.
