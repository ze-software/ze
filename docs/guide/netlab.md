# containerlab with netlab

[netlab](https://netlab.tools) builds a lab from a YAML topology and starts each node
under containerlab. Ze runs as one of those nodes. netlab calls this the daemon tier:
one YAML file and one directory of Jinja2 templates, with no Ansible task lists and no
Vagrant box.

The ze side of that integration lives in `contrib/netlab/`. **`contrib/netlab/README.md`
is the source of truth for the artifacts.** This page explains how to run them.

## What you need

| Component | Why |
|-----------|-----|
| netlab 26.10 | Renders the topology and calls containerlab |
| containerlab 0.83.0 and Docker | Start the nodes |
| `netlab/ze:latest` | The lab image, built from `docker/Dockerfile.lab` |

The lab image is not the deployment image. See [Docker](docker.md) for the two images
and what separates them.

## Step 1: build the image

```bash
docker build -t netlab/ze:latest -f docker/Dockerfile.lab .
```

`contrib/netlab/ze.yml` sets `clab.build: False` and names
`netlab/ze:latest`, so netlab starts this image and builds nothing. Change its
`image:` value when you choose a different Docker tag.
<!-- source: docker/Dockerfile.lab -- lab image -->
<!-- source: contrib/netlab/ze.yml -- clab.build, clab.image -->

## Step 2: give netlab the daemon definition

There are two routes, and `contrib/netlab/README.md` gives both. Copy the daemon
definition and the templates into the netlab package:

```bash
cp contrib/netlab/ze.yml  <netlab>/netsim/daemons/ze.yml
cp -R contrib/netlab/ze   <netlab>/netsim/daemons/ze
```

Or export a complete working directory, including `topology-defaults.yml` and
`templates/ze/`, without changing the netlab installation:

```bash
./le test netlab package directory /tmp/ze-lab
```

The destination must be new. Once the lab image is available in the target
Docker daemon, this directory can be copied to another host and used without
the Ze source checkout.
<!-- source: contrib/netlab/README.md -- Installing it into a netlab checkout -->
<!-- source: internal/le/test/netlab/actions.go -- Answer -->

## Step 3: run the reference topology

```bash
cd /tmp/ze-lab
netlab up topology.yml
```

`contrib/netlab/topology.yml` has three nodes:

```
r1 ---- internal ---- r2      AS 65001, iBGP + OSPF + IS-IS + BFD
 \
  ---- external ---- r3       AS 65002, eBGP only
```

netlab runs an IGP on internal links only, so the eBGP peer sits on its own link. r1
also carries two static routes. The topology declares every module `ze.yml` declares:
bgp, ospf, isis, bfd, and routing.
<!-- source: contrib/netlab/topology.yml -- nodes, links, module -->

Stop the lab from that directory with `netlab down --cleanup`.

To run the automated live checks from the source checkout against an existing
Ze Docker host, run:

```bash
./le test netlab run
```

The automated run stages the node binary once for the Docker daemon's
architecture, then packages it with the `lab` target in
`docker/Dockerfile.runtime`. Its node build context contains only that binary
and the runtime recipe; it does not compile Go inside Docker or send the source
checkout. The manual source-building command above remains available.
<!-- source: internal/le/test/netlab/run.go -- StageBinaries -->
<!-- source: internal/le/test/netlab/images.go -- buildLabNode -->
<!-- source: internal/le/interoplab/runtime_image.go -- StageRuntimeImage -->

If Docker currently uses the distribution's kernel, use the disposable QEMU
Docker host instead. This boots Ze's own system, forwards its Docker endpoint,
and runs the same lab against it:

```bash
./le test qemu docker-lab lab "test netlab run"
```

The QEMU route needs QEMU and the native-architecture Ze Docker-host image;
its first run builds missing image and kernel inputs. Building the Docker-host
image from scratch also needs the Docker CLI and a reachable build daemon for
its static administration tools. A native kernel build needs Docker or QEMU
and network access to its pinned sources. See the
[QEMU integration guide](../architecture/testing/qemu-integration.md).
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLab -->
<!-- source: internal/le/setup/docker/host/build.go -- builder.build -->
<!-- source: internal/le/setup/docker/host/static.go -- errDockerCLIMissing, runBuilderContainer -->

The command builds the local lab image and a tool image with the versions above.
The host needs Docker access; Python, netlab and containerlab run in the tool
container. That container shares the Docker host's network and PID namespaces,
which also supports Ze's QEMU Docker host transport. Running host containerlab
against a remote Docker socket alone cannot provide those namespaces.

The runner waits for every declared BGP, OSPF, IS-IS and BFD adjacency, verifies
both static routes in the kernel FIB, and sends sourced loopback pings between
every pair of nodes. It tears the topology down on success and failure, and a
cleanup failure fails the command. Artifacts stay in session scratch for diagnosis.
Each run binds its nodes to the image tag retained for that build and verifies
that tag still resolves to the built image ID before startup. A host-runtime run
holds an exclusive directory lease and refuses an existing deployment, so a
second invocation cannot tear down another run's topology.
<!-- source: internal/le/test/netlab/live.go -- RunLive, proveNode -->
<!-- source: internal/le/test/netlab/run.go -- runContainerLab -->

Before cleanup, a failed proof saves a private `diagnostics-*` directory inside
the retained lab directory. It contains startup output, the topology snapshot,
generated node files, Docker inspection, and each node's last 200 log lines.
It also saves addresses, kernel routes, BGP and BFD state, OSPF and IS-IS
interfaces, neighbors, databases and routes, the system RIB, and plugin state.
Address, kernel route and adjacency queries run across every node before the
more detailed routing queries. Capture has a two-minute total deadline and a
five-second deadline for each command. A failed query leaves an `.error.txt`
artifact with its command, failure and process output, so unavailable state
cannot appear as an empty successful result. Cleanup still runs after capture.
JSON artifacts keep stdout and use private `.stderr.txt` sidecars for stderr.
The runner explicitly adds `netlab.snapshot.yml` to netlab's normal outputs
before startup. netlab 26.10 writes a pickle snapshot by default; the added YAML
inventory supplies node names for live checks, failure capture, and cleanup.
<!-- source: internal/le/test/netlab/live_diagnostics.go -- keepLiveDiagnostics, keepLiveCommand -->
<!-- source: internal/le/test/netlab/live.go -- RunLive -->

## How netlab configures a node

netlab renders `contrib/netlab/ze/ze.j2` into one file, `/etc/ze/ze.conf`, and
containerlab bind-mounts it into the node. Ze has no `include` directive, so one file
holds the whole running configuration. Each other module key points at an ignore file
that nothing reads.
<!-- source: contrib/netlab/ze.yml -- daemon_config -->

The node starts netlab's `/etc/dataplane-wait.sh` under `tini`. That wrapper waits
for initial interface configuration, then replaces itself with
`ze start /etc/ze/ze.conf`. Ze therefore starts after its configured local
addresses exist. Its rendered configuration carries no interface block.
<!-- source: contrib/netlab/ze.yml -- clab.node.cmd, clab.node.config_templates, clab.group_vars.netlab_dp_module -->

The explicit `clab.node_config.initial` shell method runs Linux interface setup
through `docker exec`. The Ze initial template marks successful completion for
the startup wrapper. The explicit method overrides the Linux parent's
host-namespace method. Setting the default configuration mode alone does not
replace an inherited method.
<!-- source: contrib/netlab/ze.yml -- clab.node_config.initial -->
<!-- source: contrib/netlab/ze/initial.j2 -- Linux interface setup and completion marker -->

After changing that bind-mounted file, send SIGHUP to reload it.
`handleSIGHUPReload` runs `signalReload`,
which stages the file as a candidate, reloads it, and never writes the file back. netlab sends no SIGHUP, and `ze.yml` says so with
`features.initial.reload: false`. A configuration change in a running lab therefore
needs `kill -HUP` on the ze process inside the node, or a node restart.
<!-- source: contrib/netlab/ze.yml -- clab.group_vars.netlab_start_daemon -->
<!-- source: contrib/netlab/ze.yml -- features.initial.reload -->
<!-- source: cmd/ze/hub/main_reload.go -- handleSIGHUPReload, signalReload, stageSIGHUPCandidate -->
<!-- source: cmd/ze/hub/config_source.go -- promoteSignalCandidate -->

`netlab validate` reads the daemon through the CLI. `ze.yml` declares the show command
as `ze cli -c "show $@ | json compact"`, and netlab runs it with `docker exec`. An
explicit format pipe beats the `--format` flag, so that command emits JSON.
<!-- source: contrib/netlab/ze.yml -- clab.group_vars.netlab_show_command -->
<!-- source: internal/component/cli/client/main.go -- commandWithFormat -->

## The lab login

The rendered configuration declares one user, `netlab`, with the password `netlab`.
`ze.yml` puts the same pair in the container environment, so the CLI inside the node has
a credential. This is a well-known password in a throwaway container. Nothing in ze
defaults to it and no image carries it.
<!-- source: contrib/netlab/ze/ze.j2 -- netlab_lab_user, netlab_lab_password -->
<!-- source: contrib/netlab/ze.yml -- clab.node.env -->

The template writes the password as `plaintext-password`. Ze hashes that leaf when it
loads the file, and it warns that the file still holds the secret. See
[Authentication](authentication.md#passwords-in-a-config-file).
<!-- source: internal/component/config/loader.go -- LoadConfig, warnPlaintextOnDisk -->

## What is proven, and what is not

The render check covers template rendering and parsing. The functional profile
check covers one daemon's SSH login. The live command checks adjacencies, kernel
routes and traffic across the topology; it must finish successfully before those
properties can be claimed for a run.
<!-- source: internal/le/test/netlab/actions.go -- render and golden validation -->
<!-- source: test/plugin/netlab-lab-profile.ci -- one daemon and SSH command profile -->

| Statement | Evidence |
|-----------|----------|
| netlab accepts the daemon definition and finds a template for each module | `netlab create` exits 0 on the reference topology, in `./le test netlab check` |
| The render is valid ze configuration | `ze config validate` exits 0 on each file under `contrib/netlab/golden/` |
| A daemon runs one of those renders and answers the show command with JSON | `test/plugin/netlab-lab-profile.ci` |
| Every declared protocol reaches its healthy state, static routes reach the kernel FIB, and sourced loopback pings pass | `./le test netlab run` exits 0 after teardown |
| Each declared feature passes netlab's own upstream integration suite | Separate from the native reference-lab proof |
<!-- source: internal/le/test/netlab/actions.go -- Answer -->
<!-- source: test/plugin/netlab-lab-profile.ci -- daemon start, SSH login, json compact -->

Ze also sends and receives no LLDP frame, so a netlab validation that reads LLDP data
cannot pass. containerlab does not need LLDP, because the topology names the veth
links.

## Keeping the templates from drifting

```bash
./le test netlab check
```

It renders the templates with a real netlab, compares the result with
`contrib/netlab/golden/`, and runs `ze config validate` on each golden file. A
missing netlab is an error exit, never a skip. Use
`./le test netlab update` to rewrite the golden files.
`test/plugin/netlab-lab-profile.ci` is the other half and needs no netlab: it
starts a daemon from a golden file and parses the show command output.
<!-- source: internal/le/test/netlab/actions.go -- Actions -->
