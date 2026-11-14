---
spec: deployment-user-journeys
topic: Installer
---

## Before

Getting from a Ze binary to a working, persistent installation took manual
steps that the guides did not fully describe: installing the binary,
initialising its state and setting up the service were separate, and removal
needed hand cleanup. A container lost its login and configuration when it was
recreated unless the operator mounted the right directory. On an arm64 VM the
installer ISO booted and then failed with "ISO media not found". On the
appliance, `request wireguard keypair` failed because it ran the `wg` tool,
which the image does not ship. In the reference netlab topology a Ze node
passed on only its own prefixes, so routers two hops apart could not reach
each other. A route redistributed into BGP from an IGP could carry the IGP
gateway as its next hop to an external peer, which rejected it.

## After

`ze install` installs the running binary, initialises state only when none
exists, and starts the systemd service where there is one; on a host without
systemd it prints the foreground command. Installing again, or installing a
newer build, keeps the configuration, credentials and service settings.
`ze uninstall` stops and removes the service and binary and keeps the state;
`--purge` also removes the selected installation's state, and refuses to touch
configuration a distribution package owns.

The Docker Compose file keeps the store on a named volume, so a recreated
container comes back with the same login, SSH host key and committed
configuration. The installer ISO installs on the selected disk, leaves other
disks untouched, and the installed system boots and accepts SSH, on amd64 and
arm64 virtual machines. An appliance takes a new image and returns with its
configuration and login, with the previous image still bootable; a named
appliance can also take a configuration over SSH, authenticated by the
operator's key and a pinned host key, and refuses a malformed, unauthorised or
wrongly pinned push without changing what runs. `request wireguard keypair`
makes its keys itself. In the netlab topology every node reaches every other
node's loopback. A redistributed route now gets the next hop the receiving
BGP session needs: Ze's own address toward an external peer.

The website starts with a choice of deployment (Linux install, Docker, VM or
dedicated router, lab) and each guide covers prerequisites, a success check,
persistence, upgrade and removal. No public release exists yet: the guides say
so and offer a source build.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
