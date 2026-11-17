# Container networks

A container that Ze starts has a network namespace with a loopback device. The
container plugin's `network` package gives it more: a bridge per container
network, one link per attachment, and outbound translation. The package runs
in the container plugin's process and reaches the interface and firewall
components only through the engine client, so it behaves the same whether the
plugin runs internal or forked. The container package never imports it: it
fills the plugin's seams from its own `init()`.

This page describes what exists after slice 4a of
`plan/post-release/features/spec-container-3-network.md`. Published ports,
name resolution, the listings, the kernel enrolments and the doctor checks are
not built yet.

## Config

| Node | Under | What the package does with it |
|------|-------|-------------------------------|
| `network NAME` | `container` | A bridge named `zcn-` plus the name. The name holds 1 to 11 lowercase letters, digits and interior hyphens, and a longer one is refused, never shortened. The list carries `ze:backend "netlink"` |
| `prefix` | `container network` | One per family, masked. IPv4 1 to 30, IPv6 1 to 126. Two networks must not overlap |
| `gateway` | `container network` | One per prefix, a host address of it. Absent: the network address plus one. It is on the bridge and is each container's default route |
| `mtu` | `container network` | 68 to 16000, default 1500, at least 1280 with an IPv6 prefix. Set on the bridge and on both ends of every link |
| `masquerade` | `container network` | Default true: outbound traffic is source-translated |
| `network NAME`, `address`, `mac-address` | `container instance` | An attachment, `ordered-by user`: the first entry is `eth0` inside the container |

The attachment's key is a string with the network name's constraints. The
package's rules refuse a name the network list does not hold. It is neither a
`leafref` nor a `ze:reference`: no list key in the tree is either today.

## The rules

`parseSections` is the network sibling's parse (`RegisterConfigSibling`), so
the same rules run at `ze config validate`, at a reload's verify and at the
boot configure. Every refusal is an operator error with the line that fixes it.
The rules are V-4, V-5, V-13 to V-17 and V-19 of the operator surface spec's
table V-1, and one more: a config with a container network and no `interface`
section, or no `firewall` section, is refused naming the section to add,
because the bridge, link and table requests are refused while the component
is not loaded, and its section is what loads it.

V-19 runs the commit-time backend gate (`config.ValidateBackendFeaturesJSON`)
over the container section with the interface backend the `interface` root
selects, so the `ze:backend "netlink"` annotation is the one statement of what
the bridge needs. A translating network also needs the `nft` firewall backend:
each network's outbound chain is a regular chain, which the VPP firewall
verifier refuses.

## Addresses and names

| Fact | Rule |
|------|------|
| Assigned address | The lowest host address of the prefix that is not the gateway, not set by an operator, not stored for another attached instance, and not carried by the link of a container that left the network and has not ended |
| Record | One state key per network, `meta/container/address/{network}`: each attached instance's addresses and slot, written whole, read tolerantly. An instance whose attachment left is dropped from it, which releases its address. A network that left the config keeps its record |
| Slot | The lowest number no other attachment of the network holds, stored with the address. The interface component names the host end `zc-INDEX-SLOT` from the bridge's interface index |
| Hardware address | The `mac-address` value, else six bytes of the SHA-256 of the network and instance names with the locally administered bit set and the multicast bit clear |
| Grant | The package grants the plugin the device name prefix `zc`, which covers the bridges and the host ends, and reserves the kernel tables `ze_container` and `ze_container6` |

## The apply

The sibling's apply runs before the supervisor's, so a bridge exists before a
container of the new config starts. It reads and writes each network's record,
sends the complete set of bridges (`OwnedBridgesReplace`, with each gateway
address) and the complete set of tables (`FirewallTablesReplace`), then keeps
the links each instance's start makes. A config with no network sends nothing
unless an earlier set must be withdrawn, so a container config without a
network never needs the two components. The plugin starts after the interface
and firewall plugins when they are loaded (`StartAfter`), and loads neither.

The tables, one of family ip and one of family ip6, follow the config: per
translating network, a postrouting term sends traffic from its prefix to the
network's own chain, which returns when the output interface is the bridge, so
traffic between two containers of the network keeps its source, and
masquerades otherwise. A family no translating network uses has no table, and
a config with none sends an explicit empty table map.

## The participant

| Call | What it does |
|------|--------------|
| Contribute | Nothing yet: the resolver file is not built |
| Created | For an attached instance: removes any link still recorded under the instance's name, reads the inode of `/proc/PID/ns/net`, then asks for one link per attachment in order (bridge, slot, process id, inode, `ethN`, hardware address, mtu, addresses with the prefix length, gateways). A refusal removes what it made, on a context of its own, and fails the start. An instance with no attachment returns at once |
| Ended | Removes the links of an instance whose created call made them. A refused removal is logged at info: the kernel removes a pair with its namespace and the next start removes the record first. No address is released |

<!-- source: internal/plugins/container/network/register.go -- init, registered -->
<!-- source: internal/plugins/container/network/register_tables.go -- init -->
<!-- source: internal/plugins/container/network/verify.go -- parseSections, Config, Network, Instance, Attachment, checkComponents, checkInterfaceConfig, checkNetworkAttachments -->
<!-- source: internal/plugins/container/network/backend.go -- checkBackends, runBackendGate -->
<!-- source: internal/plugins/container/network/address.go -- assign, lowestFree, addressRecord, decodeRecord -->
<!-- source: internal/plugins/container/network/name.go -- bridgeName, derivedHardwareAddress, containerLinkName -->
<!-- source: internal/plugins/container/network/apply.go -- manager, apply, assignAll, bridgeSet, planLinks -->
<!-- source: internal/plugins/container/network/tables.go -- buildTables, familyTable -->
<!-- source: internal/plugins/container/network/attach.go -- participant, created, ended -->
<!-- source: internal/plugins/container/network/namespace_linux.go -- namespaceInode -->
