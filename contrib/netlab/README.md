# Ze as a netlab device

[netlab](https://netlab.tools) builds a lab from a YAML topology and runs each
node under containerlab. This directory holds everything netlab needs to run ze
as one of those nodes. It is also the source the upstream pull request to
`ipspace/netlab` copies from.

It lives in the ze tree, and not only upstream, because the templates emit ze
configuration syntax. Beside that syntax, a ze test renders them and feeds the
result to the ze parser. Upstream, no test has the parser to do it with.

## What is here

| File | What it is |
|------|-----------|
| `ze.yml` | The daemon definition. Mirrors `netsim/daemons/ze.yml` |
| `ze/ze.j2` | The whole running configuration. Mirrors `netsim/daemons/ze/ze.j2` |
| `ze/{bgp,ospf,isis,bfd,routing}.j2` | One stub per declared module, writing an ignore file |
| `ze/Dockerfile.j2` | The image recipe, for the day netlab builds the image itself |
| `topology.yml` | The reference topology, three nodes, every declared module |
| `golden/*.conf` | The committed render of that topology, one file per node |

## One config file, five ignore files

netlab decides which templates a node needs from the `daemon_config` map in
`ze.yml`, one entry per module. A module a node uses with no entry fails
`netlab create` with `Cannot find <module> configuration template`.

Ze has no `include` directive, so it cannot read one file per module. `ze.j2`
therefore renders the WHOLE running configuration into `/etc/ze/ze.conf`, and
every other key points at an `.ignore` file that nothing reads. This is the
dnsmasq pattern (`netsim/daemons/dnsmasq.yml` upstream), and the stub templates
say so where a reader will meet them.

## The image

netlab daemons usually build their own image. Ze does not, and `ze.yml` sets
`clab.build: False` with `image: netlab/ze:latest`. Build that image from the ze
tree first:

    make ze-docker-lab

It is a SECOND image, not the deployment one. `docker/Dockerfile` stays a static
binary on a scratch base. The lab image adds a shell and `iproute2`, because
containerlab and netlab drive a node from the outside and exec `sh` and `ip`
inside it. Both derive their feature tags from `feature-gates.txt`, so a defect
cannot reproduce in a lab and vanish in production.

## Installing it into a netlab checkout

Copy the two artifacts into the netlab package:

    cp contrib/netlab/ze.yml  <netlab>/netsim/daemons/ze.yml
    cp -R contrib/netlab/ze   <netlab>/netsim/daemons/ze

Or keep them out of the install and let a topology carry them. This is what
`make ze-netlab-render-check` does. It needs two files beside the topology:

- `topology-defaults.yml`, holding the contents of `ze.yml` under a `daemons: ze:` key
- a `templates/ze/` directory, holding the templates

netlab searches both by default (`netsim/defaults/paths.yml`,
`netsim/utils/read.py`).

## Running the reference topology

    make ze-docker-lab
    netlab up -t contrib/netlab/topology.yml

The topology is three ze nodes:

    r1 ---- internal ---- r2      AS 65001, iBGP + OSPF + IS-IS + BFD
     \
      ---- external ---- r3       AS 65002, eBGP only

netlab runs an IGP only on internal links, so the eBGP peer sits on its own
link. r3 also covers what `ze.j2` renders for a node whose modules are a subset
of the declared set.

## The lab login

The rendered configuration declares one user, `netlab`, with the password
`netlab`. `ze.yml` puts the same pair in the container environment, so
`netlab validate` reaches the CLI. This is a well-known credential in a
throwaway container. It is not a deployment credential, nothing in ze defaults
to it, and no image ships it. It exists only in a config a lab tool rendered.

Override both with `netlab_lab_user` and `netlab_lab_password` in the topology.
Change them and change `clab.node.env` in `ze.yml` to match, because the CLI
inside the node reads its credential from there.

The configuration carries the password in plain text, under
`plaintext-password`. Ze hashes it when it loads the file, so the running tree
holds a bcrypt hash and the file holds what the template wrote. The daemon warns
once, naming the file, because the secret is still in it.

## Keeping this from drifting

    make ze-netlab-render-check

It renders these templates with a real netlab, compares the result against
`golden/`, and runs `ze config validate` on each golden file. A missing netlab is
an error, never a skip. `ARGS=--update` rewrites the golden files. Review the
diff.

`test/plugin/netlab-lab-profile.ci` is the other half, and it needs no netlab. It
starts a daemon from `golden/r3.conf`, logs in as the user that render declared,
and parses the output of the show command `netlab_show_command` runs. The render
check proves the templates still emit what ze accepts. The functional test proves
ze still RUNS what they emitted.

## Known limitations

- **No lab run.** `netlab up` and `netlab validate` were never run here. The
  machine that wrote this integration has no containerlab. The artifacts are
  rendered and parsed, and no declared feature is validated by netlab's own
  integration suite.
- **No LLDP.** Ze sends and receives no LLDP frame, so a netlab validation that
  reads an LLDP table cannot pass. containerlab does not need LLDP. Its links
  are veths the topology names.
- **No published image.** `make ze-docker-lab` builds it locally. There is
  nothing to pull.
