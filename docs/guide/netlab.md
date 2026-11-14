# containerlab with netlab

[netlab](https://netlab.tools) builds a lab from a YAML topology and starts each node
under containerlab. Ze runs as one of those nodes. netlab calls this the daemon tier:
one YAML file and one directory of Jinja2 templates, with no Ansible task lists and no
Vagrant box.

The ze side of that integration lives in `contrib/netlab/`. **`contrib/netlab/README.md`
is the source of truth for the artifacts.** This page explains how to run them.
What the lab proves, and the automated live check that proves it, are on the
[netlab lab page](../../website/labs/netlab/index.html).
To run Ze outside a lab, see
[Choose a deployment path](ze-install.md#choose-a-deployment-path).

## What you need

The steps below run by hand on a Linux Docker host. Install netlab,
containerlab and Ansible on that host, and start each step from a Ze source
checkout. The automated check on the lab page runs the same tools in a tool
container instead.

| Component | Why |
|-----------|-----|
| netlab 26.10 | Renders the topology and calls containerlab |
| containerlab 0.83.0 and Docker on a Linux host | Start the nodes |
| Ansible Core 2.22.0 with Python 3.13 or later | Configures the nodes |
| `netlab/ze:latest` | The lab image that Step 1 builds. `docker/Dockerfile.lab` |
<!-- source: contrib/netlab/Dockerfile.tools -- pinned packages, Python/tool version receipts and dependency check -->
<!-- source: contrib/netlab/README.md -- Running the reference topology -->

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

There are two routes, and `contrib/netlab/README.md` gives both. The main route
exports a complete working directory, including `topology.yml`,
`topology-defaults.yml` and `templates/ze/`, without changing the netlab
installation:

```bash
./le test netlab package directory /tmp/ze-lab
```

The destination must be new. Once the lab image is available in the target
Docker daemon, this directory can be copied to another host and used without
the Ze source checkout.
<!-- source: internal/le/test/netlab/package.go -- Package -->
<!-- source: internal/le/test/netlab/netlab.go -- buildLab -->
<!-- source: internal/le/test/netlab/actions.go -- Answer -->

The other route copies the daemon definition and the templates into the netlab
package. It creates no lab directory, so make one and copy the reference
topology into it:

```bash
cp contrib/netlab/ze.yml  <netlab>/netsim/daemons/ze.yml
cp -R contrib/netlab/ze   <netlab>/netsim/daemons/ze
mkdir /tmp/ze-lab
cp contrib/netlab/topology.yml /tmp/ze-lab/topology.yml
```

Either route leaves `topology.yml` in `/tmp/ze-lab`, which Step 3 starts.
<!-- source: contrib/netlab/README.md -- Installing it into a netlab checkout -->

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
[Stop and remove the lab](#stop-and-remove-the-lab) says what stays behind.

To run the automated live check from the source checkout instead, with
`./le test netlab run`, see
[Run the automated live check](../../website/labs/netlab/index.html#netlab-run-title)
on the lab page.

## Check the lab you started

`./le test netlab run` is the full success check
([what it checks](../../website/labs/netlab/index.html#netlab-run-title)).
To look at a lab you started
by hand, open a shell on a node and ask Ze for its BGP peers:

```bash
netlab connect r1
ze cli -c "show bgp peer list"
```

The CLI inside the node uses the lab login that `ze.yml` puts in the container
environment ([The lab login](#the-lab-login)). On r1, expect the iBGP session to
r2 and the eBGP session to r3.
<!-- source: contrib/netlab/ze.yml -- clab.node.env -->
<!-- source: contrib/netlab/golden/r1.conf -- peer r2, peer r3 -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer list -->

## Update the lab after a rebuild

A running node keeps the image it started from. To run a new build, stop the
lab, rebuild the image, and start the lab again from the same directory:

```bash
cd /tmp/ze-lab
netlab down --cleanup
cd <ze-checkout>
docker build -t netlab/ze:latest -f docker/Dockerfile.lab .
cd /tmp/ze-lab
netlab up topology.yml
```

Run `docker build` from the Ze checkout and the two `netlab` commands from the
lab directory. When the checkout changes `contrib/netlab/`, give netlab the new
daemon definition again: copy it over the netlab package, or export a new
directory, because `package directory` needs a destination that does not exist.
`./le test netlab run` builds its node image from the checkout on each run, so it
needs no manual rebuild. To change a running node's configuration without a
rebuild, see the SIGHUP reload in [How netlab configures a node](#how-netlab-configures-a-node).
Review, rollback and recovery outside a lab are in [Lifecycle](lifecycle.md).
<!-- source: contrib/netlab/ze.yml -- clab.build, clab.image -->
<!-- source: internal/le/test/netlab/actions.go -- Answer -->
<!-- source: internal/le/test/netlab/images.go -- buildLabNode -->

## Stop and remove the lab

From the lab directory, `netlab down --cleanup` stops the lab and removes its
containers and the files netlab generated. The automated run uses the same
command, then fails if a node's container is still listed by
`docker ps --all`. Run the same check by hand: no r1, r2 or r3 container of the
lab may remain.

```bash
netlab down --cleanup
docker ps --all
```

The lab directory, the lab image and any daemon definition you copied into the
netlab package stay. Remove them when you no longer need the lab:

```bash
docker image rm netlab/ze:latest
rm <netlab>/netsim/daemons/ze.yml
rm -r <netlab>/netsim/daemons/ze
```

Then delete the lab directory you exported.
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
addresses exist.
<!-- source: contrib/netlab/ze.yml -- clab.node.cmd, clab.node.config_templates, clab.group_vars.netlab_dp_module -->

The rendered empty `interface {}` block starts interface monitoring so
`connected` can discover the addresses netlab installed. It declares no managed
addresses; netlab keeps ownership of them. Applying the empty block still clears
Ze's IPv6 reverse-path checks and interface IPv6 forwarding default. Each BGP peer's
redistribution attachment receives `state` and can send `update`, allowing a
peer that establishes later to request the current connected prefixes.
<!-- source: contrib/netlab/ze/ze.j2 -- interface, redistribute-orchestrator -->
<!-- source: internal/component/iface/config_apply.go -- applyConfigPass -->
<!-- source: internal/component/iface/config_sysctl.go -- applyIPv6ForwardingSwitch -->
<!-- source: internal/plugins/redistribute/register.go -- runPlugin -->

Each BGP peer attaches four processes. `bgp-rib` stores the peer's routes and
elects the best one, and re-advertises nothing. `bgp-rs` sends what one peer
announced to the node's other peers: without it a node passes on only its own
connected, OSPF and IS-IS prefixes, and a lab with a node between two BGP
sessions cannot reach across it. `bgp-adj-rib-in` is what `bgp-rs` reads to
replay those routes to a peer whose session comes up later, which in a lab is
always one of them. `redistribute-orchestrator` carries the local prefixes
into BGP.

Toward an external peer the render sets `next-hop self`. That is the default
RFC 4271 Section 5.1.3 describes, written out because Ze's own default leaves
a forwarded route's next hop as it arrived, and an external peer refuses a
next hop that is another node's loopback.
<!-- source: contrib/netlab/ze/ze.j2 -- attach process bgp-rib, bgp-rs, bgp-adj-rib-in, redistribute-orchestrator; next-hop self -->

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
