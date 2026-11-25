# Container networks

A container that Ze starts has a network namespace with a loopback device. The
container plugin's `network` package gives it more: a bridge per container
network, one link per attachment, and outbound translation. The package runs
in the container plugin's process and reaches the interface and firewall
components only through the engine client, so it behaves the same whether the
plugin runs internal or forked. The container package never imports it: it
fills the plugin's seams from its own `init()`.

This page describes what exists after slices 4a and 4b of
`plan/post-release/features/spec-container-3-network.md`, with the forwarding
defaults and the `enabled` rule of the tables. Name resolution, the listings,
the conflict check against Ze's own listeners, the kernel enrolments and the
doctor checks are not built yet.

## Config

| Node | Under | What the package does with it |
|------|-------|-------------------------------|
| `network NAME` | `container` | A bridge named `zcn-` plus the name. The name holds 1 to 11 lowercase letters, digits and interior hyphens, and a longer one is refused, never shortened. The list carries `ze:backend "kernel"` |
| `prefix` | `container network` | One per family, masked. IPv4 1 to 30, IPv6 1 to 126. Two networks must not overlap |
| `gateway` | `container network` | One per prefix, a host address of it. Absent: the network address plus one. It is on the bridge and is each container's default route |
| `mtu` | `container network` | 68 to 16000, default 1500, at least 1280 with an IPv6 prefix. Set on the bridge and on both ends of every link |
| `masquerade` | `container network` | Default true: outbound traffic is source-translated |
| `network NAME`, `address`, `mac-address` | `container instance` | An attachment, `ordered-by user`: the first entry is `eth0` inside the container |
| `enabled` | `container instance` | Default true, read as the runtime reads it. A disabled instance keeps its attachments and its addresses, and its ports are judged at verify but not translated |
| `port NAME`: `protocol`, `public-address`, `public-port`, `container-port` | `container instance` | A published port: `protocol` tcp (the default) or udp; `public-address`, at most 16, the router addresses it is published on, none meaning every address in each family the instance's first network has a prefix of; `public-port` and `container-port`, one port or a first-last range from 1 to 65535, mandatory. At most 256 ports in one config |

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

For V-19, the config component's engine-owned `ze:backend` walk judges the
container network list's `ze:backend "kernel"` annotation against the primary
data-plane selector (`interface backend`), so the annotation is the one
statement of what the bridge needs and a network under `interface backend vpp`
is refused naming the network path and `set interface backend kernel`. The
container plugin keeps only its semantic firewall rules in `checkBackends`,
reading the firewall backend through the config component's selector resolution:
a translating network needs the `kernel` firewall backend because each network's
outbound chain is a regular chain, which the VPP firewall verifier refuses, and
a published port needs it too because its terms match `destination-local`, which
the VPP verifier refuses.

The port rules are V-6 and V-18. A port on an instance with no `network` entry
is refused (V-18). Two ranges of different lengths, a range that runs
backward, and two entries that publish one protocol and an overlapping port
range on the same router address, or where either has no public address, are
refused (V-6), across instances and within one. A public address of a family
the instance's first network has no prefix of is refused, because the port is
translated to the instance's address on that network in the public address's
family. A range is accepted only onto the same range: a destination
translation to a port range keeps a connection's port when it lies inside the
range and has no form that shifts it, so `8000-8010` onto `5000-5010` would
not be exact and is refused.

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
masquerades otherwise. Per published port, in each family it is published
in, one destination translation term sits in a nat chain at prerouting, for a
client beyond the router, and the same term in a nat chain at output, for a
connection the router itself opens, both at priority -100. A term matches the
protocol, the public port, `destination-local` and the public address when one
is set, and translates to the instance's assigned address on its first
network and the container port (the address alone for a range, which keeps
the port). Its name is `port_INSTANCE_PORT`, with `_N` for the Nth public
address; no instance or port name holds an underscore, so two names never
collide. Only the first packet of a connection crosses a nat chain, so the
replies of a translated connection are never masqueraded. A family nothing
uses has no table, and a config with none sends an explicit empty table map.

The tables follow the config, not the run state: an enabled container's
terms exist while it is stopped, and a disabled container has none, so a
connection to its port is never translated to an address nothing holds. A
disabled container keeps its address record, so enabling it again gives it
the same address, and its ports still take part in the V-6 rules, so the
config stays valid when it is enabled again.

### The firewall policy and the port-blocked check

Publishing a port never opens the operator's firewall policy. The translation
tables are the plugin's own, and the plugin writes nothing into a table the
operator owns. A connection from beyond the router to a published port is
translated at prerouting and then forwarded to the container, so it crosses
every base chain at the forward hook, and a drop in any of them is final. The
policy must accept, at the forward hook, the protocol to the container port
(the destination after translation) leaving by the network's bridge `zcn-NAME`
or going to the instance's address, and the replies, by connection state
`established,related` or by input interface the bridge. A connection the
router itself opens is translated at the output hook and does not cross the
forward hook.

The doctor check `container-port-blocked` reports a published port the policy
blocks, as a warning under `doctor-container-port-blocked`. It runs in the
plugin's process, internal and forked alike, and makes no engine call: the
parse reads the `firewall` root beside the container root and keeps, in the
network config, each base chain at the forward hook whose policy is drop,
with the output interface and the destination prefix of each of its accepting
terms. The apply records those chains beside the ports it translated, with
the address each port is translated to, and the check judges that record.

| Fact | Behaviour |
|------|-----------|
| What it reads | The committed firewall config, which the firewall component applies at the same commit. It does not read the kernel's ruleset, so a rule written with another tool is not seen |
| When a port is reported | A dropping forward chain whose table carries the family of the port's target (`inet` both, `ip` IPv4, `ip6` IPv6) holds no accepting term that names the bridge as `output-interface` (the name, or a prefix before a trailing asterisk) or a `destination-address` prefix that holds the target |
| What it does not judge | The rest of a term. It reports a port a term accepts by another match, and a destination that names a set; it does not see a drop term ahead of the accepting one, or a chain at the input or output hook |
| The finding | One per port entry and blocking chain: the container, the port entry, the chain and the table, the bridge and the address |
| The fix | The accepting term as `set firewall table T chain C term container_INSTANCE_PORT ...` lines (protocol, destination port the container port, output interface the bridge, `then accept`) followed by `commit now`, to paste into `ze config edit` |
| A policy commit | A commit that changes only the firewall section is delivered to the plugin with the container root, gives a network config that differs, and is applied, so the finding follows the policy |
| A disabled container | Has no translation and no finding |

<!-- source: internal/plugins/container/network/policy.go -- parseForwardChains, ForwardChain, ForwardAccept, portPolicy, publishedPorts, acceptingTerm, answerPortBlocked -->
<!-- source: internal/test/containerprobe/policy.go -- blocked, adopt -->

### The forwarding defaults

A connection from beyond the router to a published port, and a container's
connection to a host beyond the router, are forwarded, and the operator's
policy at the forward hook must accept them. The kernel forwards a family
only while its forwarding setting is on, so the apply claims, through the
engine client's `EmitEvent`, the sysctl plugin's default
`net.ipv4.conf.all.forwarding=1` while a network has an IPv4 prefix and
`net.ipv6.conf.all.forwarding=1` while one has an IPv6 prefix. The keys are
the ones `fib-kernel` claims, so the sysctl plugin sees one key claimed by two
sources and keeps it on until both withdrew.

| Fact | Behaviour |
|------|-----------|
| Source | One per family, `container-network-ipv4` and `container-network-ipv6`. The sysctl plugin withdraws every key of a source at once (`clear-source-defaults`), so a family is withdrawn without touching the other, or the runtime's own claim under the source `container` |
| Override | The operator's `sysctl setting` for the key outranks the default. The sysctl plugin writes the operator's value and logs that the default is overridden |
| Shared claim | A default another source holds stays claimed under that source, and clearing one source hands the key to the next (`internal/component/sysctl/sysctl.go`, `dropDefaults`) |
| Withdrawal | When no network uses a family any more, the apply clears its source, and the sysctl plugin restores the kernel's value once no source claims the key. The daemon's stop restores every original value |
| Idempotence | A claim is emitted once when a family becomes used, and withdrawn once when it stops being used. A second apply of the same config emits nothing, and a config that never had a network emits nothing |

## The participant

| Call | What it does |
|------|--------------|
| Contribute | Nothing yet: the resolver file is not built |
| Created | For an attached instance: removes any link still recorded under the instance's name, reads the inode of `/proc/PID/ns/net`, then asks for one link per attachment in order (bridge, slot, process id, inode, `ethN`, hardware address, mtu, addresses with the prefix length, gateways). A refusal removes what it made, on a context of its own, and fails the start. An instance with no attachment returns at once |
| Ended | Removes the links of an instance whose created call made them. A refused removal is logged at info: the kernel removes a pair with its namespace and the next start removes the record first. No address is released |

<!-- source: internal/plugins/container/network/register.go -- init, registered -->
<!-- source: internal/plugins/container/network/register_tables.go -- init -->
<!-- source: internal/plugins/container/network/verify.go -- parseSections, Config, Network, Instance, Attachment, checkComponents, checkInterfaceConfig, checkNetworkAttachments -->
<!-- source: internal/plugins/container/network/backend.go -- checkBackends, resolveFirewallBackend -->
<!-- source: internal/plugins/container/network/address.go -- assign, lowestFree, addressRecord, decodeRecord -->
<!-- source: internal/plugins/container/network/name.go -- bridgeName, derivedHardwareAddress, containerLinkName -->
<!-- source: internal/plugins/container/network/apply.go -- manager, apply, assignAll, bridgeSet, planLinks -->
<!-- source: internal/plugins/container/network/tables.go -- buildTables, familyTable, publishedTerms, portTerms, translationTarget, publishes -->
<!-- source: internal/plugins/container/network/forwarding.go -- forwardingWanted, forward, claimForwarding, withdrawForwarding -->
<!-- source: internal/plugins/container/network/port.go -- parsePorts, checkRanges, checkPorts, portEndpoints -->
<!-- source: internal/test/containerprobe/serve.go -- serve, published, refused -->
<!-- source: internal/plugins/container/network/attach.go -- participant, created, ended -->
<!-- source: internal/plugins/container/network/namespace_linux.go -- namespaceInode -->
