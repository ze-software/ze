---
spec: iface-mirror-target-reference
topic: Interfaces
feature: interfaces
---

## Before

Six settings that name an interface Ze configures took any name: a port
mirror's `ingress` and `egress` target, a bridge `member`, a tunnel's
`local interface`, a PPPoE client's `source-interface` and a PPPoE server's
access `interface`. A mistyped name, or a target deleted while a mirror still
named it, committed, and the apply then failed or used whatever device carried
that name.

Some names also reached the wrong device. A VLAN unit, written
`<interface>.<vlan-id>`, of an ethernet entry bound to its port by `mac match`
or `os-name` was looked up by its literal name. A mirror toward it copied to
the wrong device or to none. A bridge member enslaved it before the VLAN
existed, so the apply rolled back. A PPPoE server on it never started. A
tunnel's `local interface` naming such an entry got the wrong parent device.

## After

Those six settings must name an interface the config declares: an entry of
any interface kind, the peer end of a veth, or a VLAN unit
`<interface>.<vlan-id>`. `ze config validate`, boot, reload and commit refuse
any other name, give the setting's path, and name the `set interface ...`
command that declares it. Deleting an interface that a mirror still names is
refused the same way. A device a lab or a script creates is declared as a bare
`ethernet <name> { }` entry, which only sets the port administratively up.

Protocol and match settings are unchanged: an OSPF, IS-IS, LDP or BFD
interface, a firewall match and VRRP `track interface` may still name a kernel
device that no `interface` entry declares.

A VLAN unit of an entry bound by `mac match` or `os-name` now resolves to the
VLAN of the device the selector chose, for mirrors, bridge members, PPPoE
clients and servers, and protocols. A tunnel's `local interface` uses the
entry's selected device, and the apply fails, naming the tunnel and the entry,
when that entry selects no device.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
