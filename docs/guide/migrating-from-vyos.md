# Migrating a Home CPE from VyOS

This page is the procedure a model (Claude Opus 5.5) follows to convert a VyOS
home-CPE configuration to Ze, and the page a person reads to review the result.
No converter program exists. The model reads a redacted VyOS configuration,
writes a Ze configuration with placeholders for the secrets, and writes a
conversion record that maps every VyOS line. The owner fills the secrets on the
router.

The page covers the constructs a home CPE on a PPPoE FTTP line uses: WAN, LAN,
firewall, NAT, DHCPv4, WireGuard, an IKEv2 tunnel, and system services. Other
VyOS features (BGP, OSPF, VRRP, ...) are not mapped. A line that this page does
not map goes to the record as dropped, with the reason "not mapped by the
guide", and a reviewer decides what to do with it.

## Where the VyOS side was read

Ze's YANG proves that a Ze form is accepted. It does not prove that the form
does what the VyOS construct did. So the VyOS side of a row is read in VyOS's
own source, not inferred from its CLI. The rows were read in vyos-1x, branch
`rolling`, `git describe` `vyos/1.5dev0-5530-g611550d23` (commit of
2026-09-01). The MSS and conntrack rows were first read at
`vyos/1.5dev0-5851-gdb29bdc89`, and the files below did not change for them.

| vyos-1x file | Read for |
|--------------|----------|
| `python/vyos/firewall.py` `parse_rule` | `protocol all`, `state`, groups, `inbound-interface` and `outbound-interface` |
| `python/vyos/template.py` `nft_default_rule` | `default-action`, `default-log` |
| `data/templates/firewall/nftables-defines.j2` | groups become nft sets with `flags interval` |
| `python/vyos/ifconfig/interface.py` `set_tcp_ipv4_mss`, `set_tcp_ipv6_mss`; `data/vyos-firewall-init.conf` chain `VYOS_TCP_MSS` | `adjust-mss` |
| `src/conf_mode/system_conntrack.py` `module_map`; `data/templates/conntrack/nftables-helpers.j2` | `system conntrack modules` |
| `data/templates/ssh/sshd_config.j2` | `service ssh listen-address` |
| `src/conf_mode/system_login.py` | `encrypted-password` |
| `src/conf_mode/interfaces_pppoe.py`; `data/templates/pppoe/peer.j2` | `no-default-route` |
| `python/vyos/utils/strip_config.py`; `src/helpers/strip-private.py` | redaction |

A row whose VyOS producer is not in this table carries the class `not read`.
Its Ze form is verified against the YANG, but nobody has compared the behavior
at both producers yet.

## How the model is fed

1. The owner prints the VyOS configuration in `set` form:
   `show configuration commands` in VyOS operational mode.
2. The owner replaces every value on a secret path (the table below) with a
   placeholder, on the VyOS router or on their own machine. The model never
   sees a password, a private key, a PSK or an API key.
3. The model gets this page and the redacted file, and nothing else.

Do not use VyOS's own redaction for step 2. The `| strip-private` pipe runs
`src/helpers/strip-private.py`, which by default also replaces IP addresses,
MAC addresses, user names, host names and DHCP names, so its output can not be
converted. `show configuration --strip-private` runs
`python/vyos/utils/strip_config.py`, whose `__secret_paths` list does not hold
`interfaces pppoe ... authentication password` or
`interfaces wireguard ... peer ... preshared-key`, so those reach the model. It
also replaces the PKI certificates and the SSH public keys, which are public and
which the conversion needs.

### Secret paths

| VyOS path | Placeholder | Ze leaf the model writes it into |
|-----------|-------------|----------------------------------|
| `interfaces pppoe <n> authentication password` | `<<SECRET:pppoe-<n>-password>>` | `interface pppoe-client <n> authentication password` |
| `interfaces wireguard <n> private-key` | `<<SECRET:wg-<n>-private-key>>` | `interface wireguard <n> private-key` |
| `interfaces wireguard <n> peer <p> preshared-key` | `<<SECRET:wg-<n>-<p>-psk>>` | `interface wireguard <n> peer <p> preshared-key` |
| `pki certificate <n> private key` | `<<SECRET:pki-<n>-key>>` | `pki certificate <n> private key` |
| `pki ca <n> private key` | `<<SECRET:pki-ca-<n>-key>>` | none: a Ze `pki ca` entry holds no private key, so the line is dropped |
| `system login user <n> authentication encrypted-password` | `<<SECRET:user-<n>-password>>` | `system authentication user <n> plaintext-password` |
| `vpn ipsec authentication psk <n> secret` | `<<SECRET:psk-<n>>>` | `vpn ipsec site-to-site peer <p> authentication pre-shared-secret` |
| `service https api keys id <n> key` | `<<SECRET:api-<n>-key>>` | none: the line is dropped (see the dropped list) |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- pppoe-client authentication password, wireguard private-key, peer preshared-key -->
<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- certificate private key -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- user plaintext-password -->
<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- peer authentication pre-shared-secret -->

The placeholder form is `<<SECRET:<name>>>`, where the name says which secret
it is. The model copies each placeholder unchanged into the Ze leaf. It never
invents a value for a secret, and it never writes a placeholder anywhere else.

A VyOS password hash is sha512-crypt, and Ze's `password` leaf takes only a
bcrypt hash. The model therefore writes the user's placeholder into
`plaintext-password`. On the router, the owner types the password there, and
the commit hashes it with bcrypt into `password` and removes the plaintext
leaf.
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- password ze:bcrypt, plaintext-password ze:ephemeral -->

A placeholder is a valid string in most leaves (`plaintext-password`, the PPPoE
`password`, a PSK), so the schema does not catch one left unfilled. Before Ze
loads the file on the router, `grep -n '<<SECRET:' <file>` must print nothing.

## The mapping

Ze's tree has a different shape from VyOS's, so the guide maps by construct,
not by line. Each row gives the Ze form, the rewrite the conversion makes, and
the class of the rewrite.

| Class | Meaning |
|-------|---------|
| equivalent | the Ze form does what the VyOS producer does, read at both producers |
| conditional | equivalent only while a stated condition holds; the row names the condition and the child spec that removes it |
| behavior change | the Ze form differs in a way the operator sees; the row says what the operator loses or must do |
| not read | the Ze form is checked against the YANG, but the VyOS producer was not read |

A child number names a spec under `plan/release/fixes/spec-home-cpe-<N>-*.md`,
and a child name names `plan/release/fixes/spec-<name>.md`. When the child
lands, its row changes.

### Interfaces and routes

| VyOS construct | Ze form | Rewrite or note | Class | Child |
|----------------|---------|-----------------|-------|-------|
| `interfaces ethernet <n> hw-id <mac>` | `interface ethernet <n> mac match <mac>` | Ze never renames the kernel device: `mac match` maps the name `<n>` to the device that holds the MAC, and the device keeps its kernel name. A firewall, NAT or MSS rule that names `<n>` matches that device only while the kernel name is `<n>` (see "Interface names in firewall rules") | conditional | iface-resolution-across-the-plugin-boundary |
| `interfaces ethernet <n> offload gro\|gso\|tso` | `interface ethernet <n> offload gro\|gso\|tso true` | each VyOS presence leaf becomes a boolean set to `true`; an absent leaf keeps the OS default | not read | - |
| `interfaces bridge <n> member interface <m>` | `interface bridge <n> member [ <m> ]` | | not read | - |
| `interfaces <kind> <n> address <a>` | `unit 0 ipv4 address [ <a> ]` or `unit 0 ipv6 address [ <a> ]` under the same interface | one leaf-list per family | not read | - |
| `interfaces pppoe <n> source-interface`, `authentication username`, `authentication password`, `mtu` | `interface pppoe-client <n>` with `source-interface`, `authentication username`, `authentication password` (placeholder), `mtu` | keep the VyOS name: Ze creates the kernel device under the configured name | not read | - |
| `interfaces pppoe <n> no-default-route` | `interface pppoe-client <n> no-default-route` | | equivalent | - |
| `interfaces pppoe <n> default-route-distance <d>` | `interface pppoe-client <n> route-priority <d>` | | not read | - |
| `interfaces pppoe <n> no-peer-dns` | none: dropped | Ze always asks for DNS servers in IPCP and ranks the learned servers after the static `system name-server` entries | behavior change | 13 |
| `interfaces pppoe <n> ipv6 address autoconf`, `dhcpv6-options pd <i> length <l>` | `interface pppoe-client <n> ipv6` with `enabled true`, `autoconf true`, `accept-ra 2`, `forwarding true`, `dhcpv6 enabled true` and `dhcpv6 pd length <l>` | Ze negotiates IPv6CP beside IPCP and runs the DHCPv6 client on the session device; `accept-ra 2` keeps the Router Advertisement default route while the box forwards. The delegated prefix is published as a lease on `<n>` and nothing uses it yet | not read | - |
| `interfaces pppoe <n> dhcpv6-options pd <i> interface <lan> address`, `sla-id` | none: dropped | Ze assigns no part of the delegated prefix to a LAN interface and advertises none of it yet | conditional | 8 |
| `protocols static route <p> interface <n> distance <d>` | `static table default route <p>` with `distance <d>` and `next interface <n>` | a route through a `pppoe-client` waits for the session instead of failing | not read | - |
| `protocols static route <p> next-hop <a>` | `static table default route <p> next hop <a>` | | not read | - |
| `interfaces wireguard <n> port`, `private-key` | `interface wireguard <n> listen-port`, `private-key` (placeholder) | | not read | - |
| `interfaces wireguard <n> peer <p> public-key`, `preshared-key`, `allowed-ips`, `persistent-keepalive` | `interface wireguard <n> peer <p>` with the same four leaves | `preshared-key` is a placeholder; `allowed-ips` is a bracket list | not read | - |
| `interfaces vti <n> address <a>` | `interface xfrm <n>` with `if-id <id>` and `unit 0` addresses | choose any `if-id` from 1 up, unique per xfrm interface | not read | - |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- ethernet mac match, offload, bridge member, unit ipv4/ipv6 address, pppoe-client, pppoe-client ipv6, grouping ipv6-host, wireguard, xfrm if-id -->
<!-- source: internal/component/iface/register.go -- reconcileDHCP -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop, startIPv6CP -->
<!-- source: internal/plugins/static/yang/ze-static-conf.yang -- table route distance, next hop, next interface -->
<!-- source: internal/component/config/system/resolv.go -- RegisterResolverOrigin -->

### Firewall and NAT

| VyOS construct | Ze form | Rewrite or note | Class | Child |
|----------------|---------|-----------------|-------|-------|
| `firewall ipv4 input filter` | `firewall table <t> family ip`, `chain input type filter hook input priority 0` | one table per VyOS family; `ipv6` becomes `family ip6` | equivalent | - |
| `firewall ipv4 forward filter`, `output filter` | the same, with `hook forward` or `hook output` | | equivalent | - |
| `rule <N>` | `term rule-<N>`, written in rule-number order | the term list is ordered by the user, and the order is the evaluation order | equivalent | - |
| `rule <N> action accept\|drop\|reject` | `then accept`, `then drop`, `then reject` | | equivalent | - |
| `protocol ipv6-icmp` | `protocol icmpv6` | | equivalent | - |
| `protocol all` | no `protocol` match | VyOS writes no match for `all` | equivalent | - |
| `protocol tcp_udp` | two terms, one with `protocol tcp` and one with `protocol udp` | | equivalent | - |
| `state established`, `state related` | `connection-state established,related` | one comma list | equivalent | - |
| `inbound-interface name <i>`, `outbound-interface name <i>` | `input-interface <i>`, `output-interface <i>` | a PPPoE client is matched by its configured name, as VyOS does. An ethernet is matched only while its kernel name is the configured name | conditional | iface-resolution-across-the-plugin-boundary |
| `default-action accept\|drop` | chain `policy accept\|drop` | | equivalent | - |
| `default-action reject` | a final term `then reject`, chain `policy drop` | Ze's `policy` is accept or drop only | equivalent | - |
| `default-log` | the final term of the chain carries `counter`, `log prefix "[<family>-<chain>-default-<A>]"` and the verdict of `default-action` (`drop`, `accept` or `reject`) in one `then` block | VyOS writes this rule itself; `<A>` is the first letter of the action in upper case. `log` does not decide the packet, so a term holding only `counter` and `log` lets it through to the chain policy, which never rejects | equivalent | - |
| `firewall group address-group <g> address <a>` | `set <g> type ipv4` with one `element <a>` per address, in every table that names the group; the term writes `@<g>` | a set belongs to one table, so declare it in each table that uses it | equivalent | - |
| `address-group` holding a range `<a>-<b>` | one element per address of the range | a set element in Ze config is one address | equivalent | - |
| `firewall group network-group <g> network <p>` | one term per prefix, each with `source-address <p>` or `destination-address <p>` | do not use a set: Ze programs a prefix element as its bare address | conditional | - |
| `firewall group port-group <g> port <x>` | the ports inline in the term: `destination-port 22,80,8000-8080` | a Ze `inet-service` set element is one port, and the inline list also takes ranges | equivalent | - |
| `firewall global-options all-ping`, `broadcast-ping`, `syn-cookies`, `receive-redirects`, `send-redirects`, `source-validation`, `log-martians`, `ipv6-receive-redirects`, `ipv6-src-route` | the same leaf under `firewall global-options` | | not read | - |
| `firewall global-options ip-src-route disable` | `sysctl setting net.ipv4.conf.all.accept_source_route value 0` | Ze has no IPv4 source-route leaf | not read | - |
| `nat source rule <N> outbound-interface name <i>`, `source address <p>`, `translation address masquerade` | `table <t> family ip`, `chain postrouting type nat hook postrouting priority 100 policy accept`, `term rule-<N> from output-interface <i> source-address <p> then masquerade` | `output-interface <i>` matches an ethernet only while its kernel name is the configured name | not read | iface-resolution-across-the-plugin-boundary |
| `interfaces <kind> <n> ip adjust-mss <N>` | a chain `type filter hook postrouting priority -300 policy accept` with `term from output-interface <n> protocol tcp then tcp-mss-set <N>` | Ze writes no MSS comparison and relies on the kernel, which skips a write that would raise the MSS, so both clamp and neither raises; postrouting keeps the router's own SYNs clamped. On an ethernet, `output-interface <n>` matches only while its kernel name is `<n>` | conditional | iface-resolution-across-the-plugin-boundary |
| `ip adjust-mss clamp-mss-to-pmtu` | the same chain with `then tcp-mss-clamp-to-pmtu` | the same condition as `ip adjust-mss <N>` | conditional | iface-resolution-across-the-plugin-boundary |
| `ipv6 adjust-mss ...` | the same in a `family ip6` table; one `family inet` table when the IPv4 and IPv6 values are the same | the same condition as `ip adjust-mss <N>` | conditional | iface-resolution-across-the-plugin-boundary |
| `system conntrack modules ftp\|h323\|pptp\|sip\|tftp` | `system conntrack module <m>`, one line each | Ze builds these helpers into its kernel and assigns only the listed ones, on the same ports as VyOS's `module_map` | equivalent | - |
| `system conntrack modules nfs\|sqlnet` | none: dropped | no kernel module provides them and Ze has no user-space helper, so Ze refuses the names | behavior change | - |

<!-- source: internal/component/firewall/yang/ze-firewall-conf.yang -- table family, chain type/hook/priority/policy, term, from-block, then-block, set, global-options -->
<!-- source: internal/plugins/firewall/nft/lower_linux.go -- encodeSetElementKey -->
<!-- source: internal/component/sysctl/yang/ze-sysctl-conf.yang -- setting value -->
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- conntrack module -->

The network-group row is conditional because of a Ze defect. A Ze set accepts
a prefix as an element and the set schema says it takes prefixes under
`flags-interval`, but `encodeSetElementKey` keeps only the address, so
`element 192.0.2.0/24` matches 192.0.2.0 alone. Until that is fixed, write one
term per prefix.
<!-- source: internal/plugins/firewall/nft/lower_linux.go -- encodeSetElementKey -->

#### Interface names in firewall rules

The rows that write `input-interface`, `output-interface` or a `mac match`
are conditional because of a Ze defect. Ze never renames a kernel device: a
`mac match` or an `os-name` binds the configured name to a device and leaves
the device's kernel name as it is
([logical name resolution](../architecture/iface/logical-name-resolution.md)).
The firewall writes the configured name into the nft rule unchanged, so a rule
that names `eth0` matches the device the kernel calls `eth0`, which is not the
device `mac match` chose when the two names differ. A filter, NAT or MSS rule
on such an ethernet then matches nothing, or matches another device. A
`pppoe-client` is not affected: Ze creates its kernel device under the
configured name. Until `spec-iface-resolution-across-the-plugin-boundary`
lands, check on the router that each converted ethernet name is also its
kernel name (`ip link show <n>`).
<!-- source: internal/plugins/firewall/nft/lower_linux.go -- lowerIfaceMatch -->

The firewall YANG declares no `tcp-flags` leaf in a term's `from` block, so
the conversion never writes one. An MSS clamp needs none: only a SYN carries
the MSS option, and a packet without the option is not changed.
<!-- source: internal/component/firewall/yang/ze-firewall-conf.yang -- tcp-mss-set, tcp-mss-clamp-to-pmtu -->

### Services and VPN

| VyOS construct | Ze form | Rewrite or note | Class | Child |
|----------------|---------|-----------------|-------|-------|
| `service dhcp-server listen-interface <i>` | `service dhcp-server enabled true`, `listen-interface <i>` | | not read | - |
| `service dhcp-server shared-network-name <s> subnet <p>` | `service dhcp-server shared-network <s> subnet <p>` | | not read | - |
| `subnet <p> range <r> start <a> stop <b>` | `subnet <p> range <r> start <a> stop <b>` | the range name is any string, so VyOS's number is kept | not read | - |
| `subnet <p> option default-router`, `option name-server`, `option domain-name`, `lease` | `default-router`, `dns-server`, `domain-name`, `lease-time` | | not read | - |
| `subnet <p> static-mapping <m> mac <x> ip-address <a>` | `subnet <p> static-mapping <m> mac-address <x> ip-address <a>` | | not read | - |
| `subnet <p> option server-identifier <a>` | `subnet <p> server-identifier <a>` | the listen interface must hold `<a>`: while it does not, Ze answers nothing for the subnet and logs a warning | not read | - |
| `subnet <p> subnet-id <n>` | none: dropped | Ze keys a subnet by its prefix | not read | - |
| `pki ca <n> certificate <b64>` | `pki ca <n> certificate <b64>` | copy the body as it is: Ze takes base64 DER as well as PEM | not read | - |
| `pki certificate <n> certificate <b64>`, `private key` | `pki certificate <n> certificate <b64>`, `private key` (placeholder) | | not read | - |
| a certificate with an Ed25519 key, used by IKE | the same entry | IKE signs with it under RFC 7427 and RFC 8420 when the peer announces the Identity hash, as strongSwan with its openssl or curve25519 plugin does; a peer that does not, or that sends no SIGNATURE_HASH_ALGORITHMS notify, needs an ECDSA or RSA certificate | not read | - |
| `vpn ipsec interface <i>` | `vpn ipsec interface <i>` | | not read | - |
| `vpn ipsec ike-group <g> key-exchange ikev2`, `proposal <N> encryption`, `hash`, `dh-group` | `vpn ipsec ike-group <g>` with the same leaves | `aes128gcm128` becomes `aes128gcm`, `aes256gcm128` becomes `aes256gcm` | not read | - |
| `vpn ipsec esp-group <g> proposal <N> encryption`, `pfs` | `vpn ipsec esp-group <g>` with `proposal <N> encryption`, `pfs enable\|disable` | the same encryption renames | not read | - |
| a peer with no ESP group | an explicit `esp-group` with `pfs disable` and the IKE group's encryption, named on the peer | Ze needs the peer's group named, and its `pfs` default is `enable` | not read | - |
| `site-to-site peer <p> remote-address <a>`, `ike-group`, `authentication remote-id` | `site-to-site peer <p>` with the same leaves | | not read | - |
| `site-to-site peer <p> local-address <a>` | no `local-address` | Ze sends from the address of `vpn ipsec interface` and follows it: when the address changes, as after a PPPoE reconnect, Ze binds the new address and starts the peer's session again from it, and while the interface has none the peer waits. A peer that keeps a literal `local-address` stays on that literal | behavior change | - |
| `site-to-site peer <p> authentication mode x509`, `x509 ca-certificate`, `x509 certificate` | `authentication mode x509`, `ca-certificate`, `certificate` directly under `authentication` | the `x509` container is deprecated in Ze | not read | - |
| `site-to-site peer <p> vti bind <v>` | `vti bind <v>`, plus two `traffic-selector` entries, `0.0.0.0/0` to `0.0.0.0/0` and `::/0` to `::/0` | without the IPv6 selector a VTI over IPv4 carries IPv4 only | not read | - |

<!-- source: internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang -- shared-network, subnet, range, static-mapping, server-identifier -->
<!-- source: internal/plugins/dhcpserver/segment.go -- segment.serverID, segment.withheld -->
<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- ca certificate, certificate private key -->
<!-- source: internal/component/ike/engine/register_signature_eddsa.go -- chooseEd25519Signature -->
<!-- source: internal/component/ike/engine/auth_legacy.go -- legacyKeyName -->
<!-- source: internal/component/ike/engine/dynamic_wan.go -- peerPlan, followAddress -->
<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- interface, esp-group pfs, ike-group, site-to-site peer, traffic-selector, vti bind -->

### System

| VyOS construct | Ze form | Rewrite or note | Class | Child |
|----------------|---------|-----------------|-------|-------|
| `service ssh port <x>`, `listen-address 0.0.0.0` and `listen-address ::` | one `environment ssh server main ip :: port <x>`, `environment ssh enabled true` | `::` alone serves both families; Ze refuses `::` beside `0.0.0.0` on one port | behavior change | - |
| `service ssh listen-address <a>` (one address) | `environment ssh server main ip <a> port <x>` | | equivalent | - |
| `system login user <n> authentication encrypted-password` | `system authentication user <n> plaintext-password` (placeholder) | the hash is not carried: the owner types the password again and Ze stores it as bcrypt | behavior change | - |
| `system login user <n> authentication public-keys <k> key`, `type` | `system authentication user <n> public-keys <k> key`, `type` | a key name with `@` becomes a plain name such as `admin-laptop` | not read | - |
| `system host-name`, `domain-name`, `name-server` | `system host`, `domain`, `name-server` | Ze sets the kernel host name on the appliance build | not read | - |
| `service ntp server <s>` | `environment ntp enabled true`, `server <name> address <s>` | | not read | - |
| `system config-management commit-revisions <N>` | `system commit-revisions <N>` | | not read | - |
| `system config-management commit-archive location <url>` | `system archive <name> location <url>` | | not read | - |
| `service https api keys ...` | `environment api-server grpc server <n>` with `ip` and `port`; `tls-cert` and `tls-key` beside it under `grpc` | REST scripts move to gRPC over TLS; the API key line is dropped | behavior change | - |
| `container name <n> ...` used for monitoring | `telemetry prometheus enabled true`, `server <n>`, `netdata enabled true` | Netdata-named metrics replace the container | behavior change | - |

<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- environment ssh server, public-keys -->
<!-- source: internal/component/config/yang/modules/ze-types.yang -- grouping listener ip -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- user password, plaintext-password -->
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- host, domain, name-server, commit-revisions, archive -->
<!-- source: internal/plugins/ntp/yang/ze-ntp-conf.yang -- server address -->
<!-- source: internal/component/api/grpc/yang/ze-grpc-conf.yang -- grpc server, grpc tls-cert, grpc tls-key -->
<!-- source: internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang -- prometheus server, netdata enabled -->

## Dropped constructs

Each of these lines goes to the record as `dropped:` with the reason given.

| VyOS construct | Reason |
|----------------|--------|
| `system conntrack modules nfs`, `sqlnet` | no kernel module provides them, and Ze refuses the names |
| `service dhcp-server ... subnet-id` | Ze keys a subnet by its prefix |
| `interfaces pppoe <n> no-peer-dns` | Ze has no leaf to refuse IPCP DNS (child 13) |
| `interfaces pppoe <n> dhcpv6-options pd <i> interface ...`; `service router-advert ...` that advertises the delegated prefix | Ze uses no delegated prefix on a LAN interface until child 8 |
| `pki ca <n> private key` | a Ze `pki ca` entry holds no private key |
| `service https ...` (REST API, its keys and its listener) | REST automation moves to gRPC over TLS |
| `container ...` (monitoring) | the Prometheus exporter with Netdata metrics replaces it |
| `system update-check ...` | on gokrazy, gokrazy manages the image updates |
| `policy prefix-list ...`, `policy route-map ...` that nothing references | unused BGP policy |
| a line under a feature this page does not map | not mapped by the guide |

<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- conntrack module enum, update-check -->
<!-- source: internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang -- subnet key prefix -->
<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- list ca -->

## The conversion record

Beside the Ze configuration, the model writes a table with one row for every
line of the VyOS input, in input order. A reviewer reads it, and the lab's
`conversion` scenario checks it (child 11).

| Column | Content |
|--------|---------|
| `#` | the line number in the redacted VyOS file |
| `VyOS` | the line, as it is in the file |
| `Ze` | the Ze lines it became, as config paths, or `dropped: <reason>` |
| `Class` | the class of the mapping row the model used |
| `Note` | the rewrite the row names, if any |

Several VyOS lines can map to one Ze line, and one VyOS line can map to several
Ze lines. Every VyOS line has a row, so "every source line mapped or listed as
dropped" is checked by counting rows.

## The check

1. Substitute lab values for the placeholders. A placeholder is not a valid
   value in a leaf with a format (a bcrypt hash, a WireGuard key, a PEM key),
   so the check never runs on the file with placeholders.
2. Run `ze config validate <file>`. It runs the YANG checks and each plugin's
   in-process verifier, but not a plugin's live verifier.
3. Start Ze with the file, so the commit-time verifiers run as well.
4. Run the lab scenarios of child 11 against it.
5. On the router, the owner fills the placeholders, types each user password
   into `plaintext-password`, and confirms that `grep '<<SECRET:'` prints
   nothing before Ze loads the file.

<!-- source: internal/component/config/cli/cmd_validate.go -- cmdValidate -->
<!-- source: internal/component/config/plugin_verify.go -- VerifyPluginConfigContent -->

## Interim rewrites

These rows hold only until a child spec lands. When it does, the child changes
its row here.

| Rewrite | Removed by |
|---------|------------|
| the delegated prefix is requested on the PPPoE WAN and published there, but no LAN interface takes an address or a Router Advertisement from it | child 8 |
| a network-group becomes one term per prefix, not a set | the fix of the set element defect (`encodeSetElementKey`) |
| a firewall, NAT or MSS rule names an ethernet by a name that must also be its kernel name | `spec-iface-resolution-across-the-plugin-boundary` |

## Worked example

All values are invented: RFC 5737 and RFC 3849 addresses, ULA prefixes and
`example.net` names. The certificate bodies are shortened here. The Ze side,
with lab values in place of the placeholders and full certificate bodies, is
`test/parse/vyos-guide-example.ci`, which runs `ze config validate` on it.

### VyOS input (redacted)

```
set interfaces ethernet eth0 hw-id '02:00:5e:10:00:01'
set interfaces ethernet eth0 offload gro
set interfaces ethernet eth0 offload gso
set interfaces ethernet eth1 hw-id '02:00:5e:10:00:02'
set interfaces bridge br0 address '192.0.2.1/24'
set interfaces bridge br0 address 'fd00:db8:10::1/64'
set interfaces bridge br0 member interface eth1
set interfaces pppoe pppoe0 authentication password '<<SECRET:pppoe-pppoe0-password>>'
set interfaces pppoe pppoe0 authentication username 'subscriber@isp.example.net'
set interfaces pppoe pppoe0 ip adjust-mss 'clamp-mss-to-pmtu'
set interfaces pppoe pppoe0 ipv6 adjust-mss 'clamp-mss-to-pmtu'
set interfaces pppoe pppoe0 mtu '1492'
set interfaces pppoe pppoe0 no-default-route
set interfaces pppoe pppoe0 source-interface 'eth0'
set interfaces vti vti0 address '198.51.100.253/30'
set interfaces vti vti0 address 'fd00:db8:20::2/64'
set interfaces wireguard wg0 address '198.51.100.1/24'
set interfaces wireguard wg0 peer phone allowed-ips '198.51.100.2/32'
set interfaces wireguard wg0 peer phone persistent-keepalive '25'
set interfaces wireguard wg0 peer phone preshared-key '<<SECRET:wg-wg0-phone-psk>>'
set interfaces wireguard wg0 peer phone public-key 'ZXhhbXBsZS1waG9uZS13Zy1wdWJsaWMta2V5LWxhYjA='
set interfaces wireguard wg0 port '51820'
set interfaces wireguard wg0 private-key '<<SECRET:wg-wg0-private-key>>'
set protocols static route 0.0.0.0/0 interface pppoe0 distance '10'
set firewall global-options all-ping 'enable'
set firewall global-options ip-src-route 'disable'
set firewall global-options log-martians 'enable'
set firewall global-options syn-cookies 'enable'
set firewall group address-group ADMIN address '192.0.2.10'
set firewall group address-group ADMIN address '192.0.2.11'
set firewall group network-group TRUSTED network '192.0.2.0/24'
set firewall group network-group TRUSTED network '198.51.100.0/24'
set firewall group port-group MGMT port '22'
set firewall group port-group MGMT port '9273'
set firewall ipv4 input filter default-action 'drop'
set firewall ipv4 input filter default-log
set firewall ipv4 input filter rule 10 action 'accept'
set firewall ipv4 input filter rule 10 state 'established'
set firewall ipv4 input filter rule 10 state 'related'
set firewall ipv4 input filter rule 20 action 'accept'
set firewall ipv4 input filter rule 20 destination group port-group 'MGMT'
set firewall ipv4 input filter rule 20 protocol 'tcp'
set firewall ipv4 input filter rule 20 source group network-group 'TRUSTED'
set firewall ipv4 input filter rule 30 action 'accept'
set firewall ipv4 input filter rule 30 destination port '51820'
set firewall ipv4 input filter rule 30 protocol 'udp'
set firewall ipv4 input filter rule 40 action 'accept'
set firewall ipv4 input filter rule 40 protocol 'icmp'
set firewall ipv4 input filter rule 50 action 'accept'
set firewall ipv4 input filter rule 50 protocol 'all'
set firewall ipv4 input filter rule 50 source group address-group 'ADMIN'
set firewall ipv6 input filter default-action 'reject'
set firewall ipv6 input filter rule 10 action 'accept'
set firewall ipv6 input filter rule 10 state 'established'
set firewall ipv6 input filter rule 10 state 'related'
set firewall ipv6 input filter rule 20 action 'accept'
set firewall ipv6 input filter rule 20 protocol 'ipv6-icmp'
set nat source rule 100 outbound-interface name 'pppoe0'
set nat source rule 100 source address '192.0.2.0/24'
set nat source rule 100 translation address 'masquerade'
set service dhcp-server listen-interface 'br0'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 lease '86400'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 option default-router '192.0.2.1'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 option domain-name 'home.example.net'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 option name-server '192.0.2.1'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 range 0 start '192.0.2.100'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 range 0 stop '192.0.2.199'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 static-mapping printer ip-address '192.0.2.20'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 static-mapping printer mac '02:00:5e:10:00:20'
set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 subnet-id '1'
set pki ca HQ-CA certificate 'MIIBgDCCASWg...'
set pki certificate CPE certificate 'MIIBnjCCAUSg...'
set pki certificate CPE private key '<<SECRET:pki-CPE-key>>'
set vpn ipsec esp-group ESP-HQ proposal 1 encryption 'aes128gcm128'
set vpn ipsec ike-group IKE-HQ key-exchange 'ikev2'
set vpn ipsec ike-group IKE-HQ proposal 1 dh-group '19'
set vpn ipsec ike-group IKE-HQ proposal 1 encryption 'aes128gcm128'
set vpn ipsec ike-group IKE-HQ proposal 1 hash 'sha256'
set vpn ipsec interface 'pppoe0'
set vpn ipsec site-to-site peer hq authentication mode 'x509'
set vpn ipsec site-to-site peer hq authentication remote-id 'hq.example.net'
set vpn ipsec site-to-site peer hq authentication x509 ca-certificate 'HQ-CA'
set vpn ipsec site-to-site peer hq authentication x509 certificate 'CPE'
set vpn ipsec site-to-site peer hq ike-group 'IKE-HQ'
set vpn ipsec site-to-site peer hq local-address '203.0.113.2'
set vpn ipsec site-to-site peer hq remote-address 'hq.example.net'
set vpn ipsec site-to-site peer hq vti bind 'vti0'
set service ssh listen-address '0.0.0.0'
set service ssh listen-address '::'
set service ssh port '22'
set service ntp server time.example.net
set service https api keys id automation key '<<SECRET:api-automation-key>>'
set service https listen-address '127.0.0.1'
set container name netdata image 'docker.io/netdata/netdata:stable'
set system conntrack modules ftp
set system conntrack modules nfs
set system conntrack modules sip
set system config-management commit-revisions '100'
set system domain-name 'home.example.net'
set system host-name 'cpe'
set system login user admin authentication encrypted-password '<<SECRET:user-admin-password>>'
set system login user admin authentication public-keys admin@laptop key 'AAAAC3NzaC1lZDI1NTE5AAAAIGV4YW1wbGUtdnlvcy1ndWlkZS1zc2gta2V5LWxhYjA0'
set system login user admin authentication public-keys admin@laptop type 'ssh-ed25519'
set system name-server '192.0.2.53'
set system update-check auto-check
set policy prefix-list UNUSED rule 10 action 'permit'
set policy prefix-list UNUSED rule 10 prefix '0.0.0.0/0'
```

### Ze output

The Ze side is the configuration in `test/parse/vyos-guide-example.ci` with
these differences: each secret holds its placeholder, the user has
`plaintext-password "<<SECRET:user-admin-password>>"` in place of the bcrypt
`password`, and the gRPC `tls-cert` and `tls-key` paths name files the owner
installs. The main blocks:

```
interface {
    pppoe-client pppoe0 {
        source-interface eth0
        mtu 1492
        no-default-route
        authentication {
            username subscriber@isp.example.net
            password "<<SECRET:pppoe-pppoe0-password>>"
        }
    }
}
static {
    table default {
        route 0.0.0.0/0 {
            distance 10
            next { interface pppoe0 { } }
        }
    }
}
firewall {
    table v4-input {
        family ip
        chain input {
            type filter
            hook input
            priority 0
            policy drop
            term default-action {
                then {
                    counter
                    log { prefix "[ipv4-input-default-D]"; }
                    drop
                }
            }
        }
    }
    table v6-input {
        family ip6
        chain input {
            type filter
            hook input
            priority 0
            policy drop
            term default-action {
                then { reject; }
            }
        }
    }
    table wan-mss {
        family inet
        chain mss {
            type filter
            hook postrouting
            priority -300
            policy accept
            term pppoe0-clamp {
                from { output-interface pppoe0; protocol tcp; }
                then { tcp-mss-clamp-to-pmtu; }
            }
        }
    }
}
vpn {
    ipsec {
        interface pppoe0
        esp-group ESP-HQ {
            pfs disable
            proposal 1 { encryption aes128gcm; }
        }
    }
}
system {
    authentication {
        user admin {
            plaintext-password "<<SECRET:user-admin-password>>"
        }
    }
}
```

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- pppoe-client -->
<!-- source: internal/plugins/static/yang/ze-static-conf.yang -- route distance, next interface -->
<!-- source: internal/component/firewall/yang/ze-firewall-conf.yang -- tcp-mss-clamp-to-pmtu -->
<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- esp-group pfs -->

### Conversion record (excerpt)

| # | VyOS | Ze | Class | Note |
|---|------|----|-------|------|
| 10 | `set interfaces pppoe pppoe0 ip adjust-mss 'clamp-mss-to-pmtu'` | `firewall table wan-mss family inet chain mss term pppoe0-clamp then tcp-mss-clamp-to-pmtu` | equivalent | one inet table, because the IPv6 value is the same |
| 11 | `set interfaces pppoe pppoe0 ipv6 adjust-mss 'clamp-mss-to-pmtu'` | the same term as line 10 | equivalent | |
| 31 | `set firewall group network-group TRUSTED network '192.0.2.0/24'` | `firewall table v4-input chain input term rule-20-a from source-address 192.0.2.0/24` | conditional | one term per prefix, not a set |
| 35 | `set firewall ipv4 input filter default-action 'drop'` | `firewall table v4-input chain input policy drop`; `term default-action then drop` | equivalent | |
| 36 | `set firewall ipv4 input filter default-log` | `firewall table v4-input chain input term default-action then { counter; log prefix "[ipv4-input-default-D]"; drop; }` | equivalent | the same term as line 35 |
| 50 | `set firewall ipv4 input filter rule 50 protocol 'all'` | none | equivalent | no `protocol` match |
| 52 | `set firewall ipv6 input filter default-action 'reject'` | `firewall table v6-input chain input policy drop`; `term default-action then reject` | equivalent | no `default-log`, so no `counter` or `log` |
| 70 | `set service dhcp-server shared-network-name LAN subnet 192.0.2.0/24 subnet-id '1'` | dropped: Ze keys a subnet by its prefix | not read | |
| 85 | `set vpn ipsec site-to-site peer hq local-address '203.0.113.2'` | none | behavior change | Ze sends from the address of `vpn ipsec interface pppoe0` and follows it after a reconnect |
| 87 | `set vpn ipsec site-to-site peer hq vti bind 'vti0'` | `vpn ipsec site-to-site peer hq vti bind vti0`; `traffic-selector 1`, `traffic-selector 2` | not read | both families on the VTI |
| 92 | `set service https api keys id automation key '<<SECRET:api-automation-key>>'` | dropped: REST automation moves to gRPC over TLS | behavior change | |
| 96 | `set system conntrack modules nfs` | dropped: no kernel module provides it | behavior change | |
| 105 | `set system update-check auto-check` | dropped: on gokrazy, gokrazy manages the image updates | behavior change | the router no longer checks for updates itself |
| 106 | `set policy prefix-list UNUSED rule 10 action 'permit'` | dropped: unused BGP policy | behavior change | nothing referenced it, so routing does not change; the list is no longer in the configuration |
