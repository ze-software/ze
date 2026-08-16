# netlab

[netlab](https://netlab.tools) builds a lab from a YAML topology and starts each node
under containerlab. Ze runs as one of those nodes. netlab calls this the daemon tier:
one YAML file and one directory of Jinja2 templates, with no Ansible task lists and no
Vagrant box.

The ze side of that integration lives in `contrib/netlab/`. **`contrib/netlab/README.md`
is the source of truth for the artifacts.** This page explains how to run them.

## What you need

| Component | Why |
|-----------|-----|
| netlab 26.08 | Renders the topology and calls containerlab |
| containerlab and docker | Start the nodes |
| `netlab/ze:latest` | The lab image, built by `make ze-docker-lab` |

The lab image is not the deployment image. See [Docker](https://github.com/ze-software/ze/blob/main/docs/guide/docker.md) for the two images
and what separates them.

## Step 1: build the image

```bash
make ze-docker-lab
```

`contrib/netlab/ze.yml` sets `clab.build: False` and `image: netlab/ze:latest`, so netlab
starts this image and builds nothing. `ZE_LAB_IMAGE` and `ZE_LAB_TAG` change the name.
Change them and change `image:` in `ze.yml` to match.
<!-- source: Makefile -- ze-docker-lab, ZE_LAB_IMAGE, ZE_LAB_TAG -->
<!-- source: contrib/netlab/ze.yml -- clab.build, clab.image -->

## Step 2: give netlab the daemon definition

There are two routes, and `contrib/netlab/README.md` gives both. Copy the daemon
definition and the templates into the netlab package:

```bash
cp contrib/netlab/ze.yml  <netlab>/netsim/daemons/ze.yml
cp -R contrib/netlab/ze   <netlab>/netsim/daemons/ze
```

Or leave the netlab install alone and let a topology carry them, which is what
`make ze-netlab-render-check` does. netlab reads a `topology-defaults.yml` beside the
topology and a `templates/ze/` directory beside it.
<!-- source: contrib/netlab/README.md -- Installing it into a netlab checkout -->
<!-- source: scripts/dev/netlab_render_check.py -- build_lab -->

## Step 3: run the reference topology

```bash
netlab up -t contrib/netlab/topology.yml
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

## How netlab configures a node

netlab renders `contrib/netlab/ze/ze.j2` into one file, `/etc/ze/ze.conf`, and
containerlab bind-mounts it into the node. Ze has no `include` directive, so one file
holds the whole running configuration. Each other module key points at an ignore file
that nothing reads.
<!-- source: contrib/netlab/ze.yml -- daemon_config -->

The node starts with `ze start /etc/ze/ze.conf`. netlab assigns the interface addresses
with `ip` inside the container, so the rendered configuration carries no interface
block.

Ze re-reads its configuration on SIGHUP only. `handleSIGHUPReload` stages the file as a
candidate and reloads it. netlab sends no SIGHUP, and `ze.yml` says so with
`features.initial.reload: false`. A configuration change in a running lab therefore
needs `kill -HUP` on the ze process inside the node, or a node restart.
<!-- source: contrib/netlab/ze.yml -- clab.group_vars.netlab_start_daemon -->
<!-- source: contrib/netlab/ze.yml -- features.initial.reload -->
<!-- source: cmd/ze/hub/main_reload.go -- handleSIGHUPReload, stageSIGHUPCandidate -->

`netlab validate` reads the daemon through the CLI. `ze.yml` declares the show command
as `ze cli -c "show $@ | json compact"`, and netlab runs it with `docker exec`. An
explicit format pipe beats the `--format` flag, so that command emits JSON.
<!-- source: contrib/netlab/ze.yml -- clab.group_vars.netlab_show_command -->
<!-- source: internal/component/cli/client/main.go -- renderCommandOutput -->

## The lab login

The rendered configuration declares one user, `netlab`, with the password `netlab`.
`ze.yml` puts the same pair in the container environment, so the CLI inside the node has
a credential. This is a well-known password in a throwaway container. Nothing in ze
defaults to it and no image carries it.
<!-- source: contrib/netlab/ze/ze.j2 -- netlab_lab_user, netlab_lab_password -->
<!-- source: contrib/netlab/ze.yml -- clab.node.env -->

The template writes the password as `plaintext-password`. Ze hashes that leaf when it
loads the file, and it warns that the file still holds the secret. See
[Authentication](../../guides/authentication/index.md#passwords-in-a-config-file).
<!-- source: internal/component/config/loader.go -- LoadConfig, warnPlaintextOnDisk -->

## What is proven, and what is not

**The lab has never been started on the machine that wrote this integration.** That
machine has no containerlab, so `netlab up` and `netlab validate` were not run. The
declared bgp, ospf, isis, bfd, and routing features are rendered and parsed. They are
not validated against netlab's own integration tests.

| Statement | Evidence |
|-----------|----------|
| netlab accepts the daemon definition and finds a template for each module | `netlab create` exits 0 on the reference topology, in `make ze-netlab-render-check` |
| The render is valid ze configuration | `ze config validate` exits 0 on each file under `contrib/netlab/golden/` |
| A daemon runs one of those renders and answers the show command with JSON | `test/plugin/netlab-lab-profile.ci` |
| Routes reach the FIB of a running lab, and a `ping` validation passes | Not run |
| Each declared feature passes netlab's integration test for it | Not run |
<!-- source: scripts/dev/netlab_render_check.py -- run_netlab_create, compare, validate_golden -->
<!-- source: test/plugin/netlab-lab-profile.ci -- daemon start, SSH login, json compact -->

Ze also sends and receives no LLDP frame, so a netlab validation that reads LLDP data
cannot pass. containerlab does not need LLDP, because the topology names the veth
links.

## Keeping the templates from drifting

```bash
make ze-netlab-render-check
```

It renders the templates with a real netlab, compares the result against
`contrib/netlab/golden/`, and runs `ze config validate` on each golden file. A missing
netlab is an error exit, never a skip. `ARGS=--update` rewrites the golden files.
`test/plugin/netlab-lab-profile.ci` is the other half and needs no netlab: it starts a
daemon from a golden file and parses the show command output.
<!-- source: mk/test-integration.mk -- ze-netlab-render-check -->
