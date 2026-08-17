# Command Equivalents

392 live Ze commands. 47 have vendor CLI today. 100 have been reviewed for migration intent. Vendor commands are curated migration hints, not exhaustive vendor CLI catalogs.

## Commands with vendor CLI

These rows have at least one listed vendor command.

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `clear interface counters` | Daemon | - | - | - | `clear interfaces ethernet <name> counters` | [details](clear-interface-counters/) |
| `clear interface name <name> counters` | Daemon | - | - | - | `clear interfaces ethernet <name> counters` | [details](clear-interface-name-counters/) |
| `request commit` | Daemon | - | - | - | `commit` | [details](request-commit/) |
| `request halt` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-halt/) |
| `request peer refresh <selector>` | Daemon | `clear bgp neighbor <peer> soft-inbound` | `clear bgp ipv4 unicast <peer> soft in` | - | `reset bgp <peer> soft in` | [details](request-peer-refresh/) |
| `request peer <selector> teardown [cease-subcode]` | Daemon | `clear bgp neighbor <peer>` | `clear bgp ipv4 unicast <peer>` | - | `reset bgp <peer>` | [details](request-peer-teardown/) |
| `request reboot` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-reboot/) |
| `request reload` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-reload/) |
| `request shutdown` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-shutdown/) |
| `resolve dns a <hostname>` | Read-only | - | - | - | `show dns` | [details](resolve-dns-a/) |
| `resolve dns aaaa <hostname>` | Read-only | - | - | - | `show dns` | [details](resolve-dns-aaaa/) |
| `resolve dns ptr <ip-address>` | Read-only | - | - | - | `show dns` | [details](resolve-dns-ptr/) |
| `resolve ping <target> [source <ip>] [count <n>] [size <bytes>]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](resolve-ping/) |
| `resolve traceroute <target> [source <ip>] [max-hops N] [timeout D] [probes N]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](resolve-traceroute/) |
| `show arp` | Read-only | - | - | - | `show arp` | [details](show-arp/) |
| `show bgp peer <selector> rib [scope\|filters\|terminal]` | Read-only | `show route advertising-protocol bgp <peer>`<br>`show route receive-protocol bgp <peer>` | - | - | - | [details](show-bgp-peer-rib/) |
| `show bgp rib best` | Read-only | `show route <prefix> protocol bgp`<br>`show route protocol bgp` | `show bgp ipv4 unicast`<br>`show bgp ipv4 unicast <prefix>` | - | `show ip bgp <prefix>` | [details](show-bgp-rib-best/) |
| `show bgp summary` | Read-only | `show bgp summary` | `show bgp ipv4 unicast summary`<br>`show bgp summary` | `show router bgp summary` | `show bgp ipv4 summary`<br>`show ip bgp summary` | [details](show-bgp-summary/) |
| `show config diff` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-diff/) |
| `show config dump` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-dump/) |
| `show config fmt` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-fmt/) |
| `show config history` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-history/) |
| `show dns cache list` | Read-only | - | - | - | `show dns` | [details](show-dns-cache-list/) |
| `show dns cache record <name>` | Read-only | - | - | - | `show dns` | [details](show-dns-cache-record/) |
| `show dns cache stats` | Read-only | - | - | - | `show dns` | [details](show-dns-cache-stats/) |
| `show dns lookup <hostname> [<type>]` | Read-only | - | - | - | `show dns` | [details](show-dns-lookup/) |
| `show doctor` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-doctor/) |
| `show errors` | Read-only | - | - | - | `show log` | [details](show-errors/) |
| `show health` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-health/) |
| `show interface brief` | Read-only | - | - | - | `show interfaces` | [details](show-interface-brief/) |
| `show interface name <name> detail` | Read-only | - | - | - | `show interfaces <name>` | [details](show-interface-name-detail/) |
| `show log recent [<component>] [<count>] [<level>]` | Read-only | - | - | - | `show log` | [details](show-log-recent/) |
| `show neighbor [<family>]` | Read-only | - | - | - | `show arp` | [details](show-neighbor/) |
| `show ping [<count>] [<dest>] [<size>] [<timeout>]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](show-ping/) |
| `show route [<limit>] [<prefix>]` | Read-only | `show route` | - | - | `show ip route` | [details](show-route/) |
| `show route lookup` | Read-only | `show route <prefix>` | - | - | `show ip route <prefix>` | [details](show-route-lookup/) |
| `show system cpu` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-system-cpu/) |
| `show system date` | Read-only | - | - | - | `show date`<br>`show ntp` | [details](show-system-date/) |
| `show system memory` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-system-memory/) |
| `show system ntp` | Read-only | - | - | - | `show date`<br>`show ntp` | [details](show-system-ntp/) |
| `show system ntp peers` | Read-only | - | - | - | `show date`<br>`show ntp` | [details](show-system-ntp-peers/) |
| `show system platform` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-system-platform/) |
| `show traceroute [<dest>] [<max-hops>] [<probes>] [<timeout>]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](show-traceroute/) |
| `show uptime` | Read-only | - | - | - | `show system uptime` | [details](show-uptime/) |
| `show version` | Read-only | - | - | - | `show system uptime` | [details](show-version/) |
| `show warnings` | Read-only | - | - | - | `show log` | [details](show-warnings/) |
| `validate config` | Offline | - | - | - | `commit` | [details](validate-config/) |

## Full live command catalog

Rows without vendor CLI remain visible so missing coverage is explicit.


### announce

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `announce <unicast\|blackhole\|flowspec> <args> [tag <key> <value>] [for <duration>]` | Daemon | - | - | - | - | [details](announce/) |

### clear

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `clear bgp rib in` | Daemon | - | - | - | - | [details](clear-bgp-rib-in/) |
| `clear bgp rib out` | Daemon | - | - | - | - | [details](clear-bgp-rib-out/) |
| `clear debug` | Offline | - | - | - | - | [details](clear-debug/) |
| `clear dns cache` | Daemon | - | - | - | - | [details](clear-dns-cache/) |
| `clear dns cache record <name> [<type>]` | Daemon | - | - | - | - | [details](clear-dns-cache-record/) |
| `clear dns cache stats` | Daemon | - | - | - | - | [details](clear-dns-cache-stats/) |
| `clear interface counters` | Daemon | - | - | - | `clear interfaces ethernet <name> counters` | [details](clear-interface-counters/) |
| `clear interface name <name> counters` | Daemon | - | - | - | `clear interfaces ethernet <name> counters` | [details](clear-interface-name-counters/) |
| `clear isis adjacency` | Daemon | - | - | - | - | [details](clear-isis-adjacency/) |
| `clear isis counters` | Daemon | - | - | - | - | [details](clear-isis-counters/) |
| `clear l2tp session all` | Daemon | - | - | - | - | [details](clear-l2tp-session-all/) |
| `clear l2tp session id` | Daemon | - | - | - | - | [details](clear-l2tp-session-id/) |
| `clear l2tp tunnel all` | Daemon | - | - | - | - | [details](clear-l2tp-tunnel-all/) |
| `clear l2tp tunnel id` | Daemon | - | - | - | - | [details](clear-l2tp-tunnel-id/) |
| `clear ospf counters` | Daemon | - | - | - | - | [details](clear-ospf-counters/) |
| `clear ospf neighbor` | Daemon | - | - | - | - | [details](clear-ospf-neighbor/) |
| `clear ospf process` | Daemon | - | - | - | - | [details](clear-ospf-process/) |
| `clear vpn ipsec sa` | Daemon | - | - | - | - | [details](clear-vpn-ipsec-sa/) |
| `clear vrrp statistics` | Daemon | - | - | - | - | [details](clear-vrrp-statistics/) |

### create

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `create interface address <name> <prefix>` | Daemon | - | - | - | - | [details](create-interface-address/) |
| `create interface bridge name <name>` | Daemon | - | - | - | - | [details](create-interface-bridge-name/) |
| `create interface bridge name <name> address <prefix>` | Daemon | - | - | - | - | [details](create-interface-bridge-name-address/) |
| `create interface bridge name <name> unit <vid>` | Daemon | - | - | - | - | [details](create-interface-bridge-name-unit/) |
| `create interface dummy name <name>` | Daemon | - | - | - | - | [details](create-interface-dummy-name/) |
| `create interface dummy name <name> address <prefix>` | Daemon | - | - | - | - | [details](create-interface-dummy-name-address/) |
| `create interface dummy name <name> unit <vid>` | Daemon | - | - | - | - | [details](create-interface-dummy-name-unit/) |
| `create interface unit <parent> <vid>` | Daemon | - | - | - | - | [details](create-interface-unit/) |
| `create interface veth name <name> <peer>` | Daemon | - | - | - | - | [details](create-interface-veth-name/) |

### debug

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `debug ip ospf inject opaque scope <link\|area\|as> id <opaque-id>` | Daemon | - | - | - | - | [details](debug-ip-ospf-inject-opaque/) |
| `debug ipv6 ospf inject lsa scope <link\|area\|as> type <ls-type>` | Daemon | - | - | - | - | [details](debug-ipv6-ospf-inject-lsa/) |
| `debug ospf inject disable` | Daemon | - | - | - | - | [details](debug-ospf-inject-disable/) |
| `debug ospf inject enable` | Daemon | - | - | - | - | [details](debug-ospf-inject-enable/) |

### delete

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `delete bgp peer <selector>` | Daemon | - | - | - | - | [details](delete-bgp-peer/) |
| `delete debug module` | Offline | - | - | - | - | [details](delete-debug-module/) |
| `delete debug profile name` | Offline | - | - | - | - | [details](delete-debug-profile-name/) |
| `delete interface name <name>` | Daemon | - | - | - | - | [details](delete-interface-name/) |
| `delete interface name <name> address <prefix>` | Daemon | - | - | - | - | [details](delete-interface-name-address/) |
| `delete interface name <name> unit` | Daemon | - | - | - | - | [details](delete-interface-name-unit/) |

### doctor

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `doctor` | Offline | - | - | - | - | [details](doctor/) |

### explain

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `explain` | Offline | - | - | - | - | [details](explain/) |

### generate

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `generate wireguard keypair` | Offline | - | - | - | - | [details](generate-wireguard-keypair/) |

### help

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `help` | Read-only | - | - | - | - | [details](help/) |
| `help ai` | Offline | - | - | - | - | [details](help-ai/) |
| `help command` | Offline | - | - | - | - | [details](help-command/) |

### monitor

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `monitor bgp` | Read-only | - | - | - | - | [details](monitor-bgp/) |
| `monitor event` | Read-only | - | - | - | - | [details](monitor-event/) |
| `monitor interface rate` | Read-only | - | - | - | - | [details](monitor-interface-rate/) |
| `monitor ping` | Read-only | - | - | - | - | [details](monitor-ping/) |
| `monitor system netlink` | Read-only | - | - | - | - | [details](monitor-system-netlink/) |
| `monitor traceroute` | Read-only | - | - | - | - | [details](monitor-traceroute/) |
| `monitor traffic stat [<name>]` | Read-only | - | - | - | - | [details](monitor-traffic-stat/) |
| `monitor vpn ipsec` | Read-only | - | - | - | - | [details](monitor-vpn-ipsec/) |

### peer

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `peer raw <selector>` | Daemon | - | - | - | - | [details](peer-raw/) |
| `peer update <selector>` | Daemon | - | - | - | - | [details](peer-update/) |

### plugin

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `plugin ack` | Read-only | - | - | - | - | [details](plugin-ack/) |
| `plugin command complete` | Read-only | - | - | - | - | [details](plugin-command-complete/) |
| `plugin command help` | Read-only | - | - | - | - | [details](plugin-command-help/) |
| `plugin command list` | Read-only | - | - | - | - | [details](plugin-command-list/) |
| `plugin encoding` | Read-only | - | - | - | - | [details](plugin-encoding/) |
| `plugin format` | Read-only | - | - | - | - | [details](plugin-format/) |
| `plugin help` | Read-only | - | - | - | - | [details](plugin-help/) |
| `plugin session bye` | Read-only | - | - | - | - | [details](plugin-session-bye/) |
| `plugin session ping` | Read-only | - | - | - | - | [details](plugin-session-ping/) |
| `plugin session ready` | Read-only | - | - | - | - | [details](plugin-session-ready/) |

### request

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `request as112 healthcheck [target <ip>]` | Daemon | - | - | - | - | [details](request-as112-healthcheck/) |
| `request bgp rib inject` | Daemon | - | - | - | - | [details](request-bgp-rib-inject/) |
| `request bgp rib withdraw` | Daemon | - | - | - | - | [details](request-bgp-rib-withdraw/) |
| `request cache expire <id>` | Daemon | - | - | - | - | [details](request-cache-expire/) |
| `request cache forward <id> <selector>` | Daemon | - | - | - | - | [details](request-cache-forward/) |
| `request cache release <id>` | Daemon | - | - | - | - | [details](request-cache-release/) |
| `request cache retain <id>` | Daemon | - | - | - | - | [details](request-cache-retain/) |
| `request commit` | Daemon | - | - | - | `commit` | [details](request-commit/) |
| `request config archive` | Daemon | - | - | - | - | [details](request-config-archive/) |
| `request halt` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-halt/) |
| `request interface <name> down` | Daemon | - | - | - | - | [details](request-interface-down/) |
| `request interface <name> mac <aa:bb:cc:dd:ee:ff>` | Daemon | - | - | - | - | [details](request-interface-mac/) |
| `request interface migrate` | Daemon | - | - | - | - | [details](request-interface-migrate/) |
| `request interface <name> mtu <bytes>` | Daemon | - | - | - | - | [details](request-interface-mtu/) |
| `request interface <name> up` | Daemon | - | - | - | - | [details](request-interface-up/) |
| `request l2tp outgoing-call remote <name> called <number>` | Daemon | - | - | - | - | [details](request-l2tp-outgoing-call-remote-called/) |
| `request log level <logger> <level>` | Daemon | - | - | - | - | [details](request-log-level/) |
| `request ospf graceful-restart` | Daemon | - | - | - | - | [details](request-ospf-graceful-restart/) |
| `request peer borr <selector>` | Daemon | - | - | - | - | [details](request-peer-borr/) |
| `request peer clear soft <selector>` | Daemon | - | - | - | - | [details](request-peer-clear-soft/) |
| `request peer eorr <selector>` | Daemon | - | - | - | - | [details](request-peer-eorr/) |
| `request peer <selector> flush` | Daemon | - | - | - | - | [details](request-peer-flush/) |
| `request peer <selector> pause` | Daemon | - | - | - | - | [details](request-peer-pause/) |
| `request peer <selector> plugin session ready` | Daemon | - | - | - | - | [details](request-peer-plugin-session-ready/) |
| `request peer refresh <selector>` | Daemon | `clear bgp neighbor <peer> soft-inbound` | `clear bgp ipv4 unicast <peer> soft in` | - | `reset bgp <peer> soft in` | [details](request-peer-refresh/) |
| `request peer <selector> resume` | Daemon | - | - | - | - | [details](request-peer-resume/) |
| `request peer <selector> teardown [cease-subcode]` | Daemon | `clear bgp neighbor <peer>` | `clear bgp ipv4 unicast <peer>` | - | `reset bgp <peer>` | [details](request-peer-teardown/) |
| `request quiesce` | Daemon | - | - | - | - | [details](request-quiesce/) |
| `request reboot` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-reboot/) |
| `request reload` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-reload/) |
| `request shutdown` | Daemon | - | - | - | `poweroff`<br>`reboot` | [details](request-shutdown/) |
| `request subscribe` | Daemon | - | - | - | - | [details](request-subscribe/) |
| `request unsubscribe` | Daemon | - | - | - | - | [details](request-unsubscribe/) |

### resolve

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `resolve cymru asn-name <asn>` | Read-only | - | - | - | - | [details](resolve-cymru-asn-name/) |
| `resolve dns a <hostname>` | Read-only | - | - | - | `show dns` | [details](resolve-dns-a/) |
| `resolve dns aaaa <hostname>` | Read-only | - | - | - | `show dns` | [details](resolve-dns-aaaa/) |
| `resolve dns ptr <ip-address>` | Read-only | - | - | - | `show dns` | [details](resolve-dns-ptr/) |
| `resolve dns txt <hostname>` | Read-only | - | - | - | - | [details](resolve-dns-txt/) |
| `resolve irr expand` | Read-only | - | - | - | - | [details](resolve-irr-expand/) |
| `resolve irr prefix` | Read-only | - | - | - | - | [details](resolve-irr-prefix/) |
| `resolve peeringdb as-set <asn>` | Read-only | - | - | - | - | [details](resolve-peeringdb-as-set/) |
| `resolve peeringdb max-prefix <asn>` | Read-only | - | - | - | - | [details](resolve-peeringdb-max-prefix/) |
| `resolve ping <target> [source <ip>] [count <n>] [size <bytes>]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](resolve-ping/) |
| `resolve traceroute <target> [source <ip>] [max-hops N] [timeout D] [probes N]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](resolve-traceroute/) |

### set

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `set debug active name` | Offline | - | - | - | - | [details](set-debug-active-name/) |
| `set debug module` | Offline | - | - | - | - | [details](set-debug-module/) |
| `set debug profile name` | Offline | - | - | - | - | [details](set-debug-profile-name/) |
| `set debug timeout` | Offline | - | - | - | - | [details](set-debug-timeout/) |
| `set system file-descriptors [<limit>]` | Daemon | - | - | - | - | [details](set-system-file-descriptors/) |

### show

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `show aaa accounting` | Read-only | - | - | - | - | [details](show-aaa-accounting/) |
| `show announcements [tag <key>] [selector <pattern>] [family <fam>]` | Read-only | - | - | - | - | [details](show-announcements/) |
| `show anomaly detect` | Read-only | - | - | - | - | [details](show-anomaly-detect/) |
| `show anomaly shape` | Read-only | - | - | - | - | [details](show-anomaly-shape/) |
| `show arp` | Read-only | - | - | - | `show arp` | [details](show-arp/) |
| `show as112` | Read-only | - | - | - | - | [details](show-as112/) |
| `show audit [<action>] [<actor>] [<count>] [<since>] [<surface>] [<until>]` | Read-only | - | - | - | - | [details](show-audit/) |
| `show bfd profile` | Read-only | - | - | - | - | [details](show-bfd-profile/) |
| `show bfd profile <name>` | Read-only | - | - | - | - | [details](show-bfd-profile-name/) |
| `show bfd session <address>` | Read-only | - | - | - | - | [details](show-bfd-session-address/) |
| `show bfd sessions` | Read-only | - | - | - | - | [details](show-bfd-sessions/) |
| `show bgp decode` | Read-only | - | - | - | - | [details](show-bgp-decode/) |
| `show bgp encode` | Read-only | - | - | - | - | [details](show-bgp-encode/) |
| `show bgp health` | Read-only | - | - | - | - | [details](show-bgp-health/) |
| `show bgp irr` | Read-only | - | - | - | - | [details](show-bgp-irr/) |
| `show bgp irr check <peer> <prefix>` | Read-only | - | - | - | - | [details](show-bgp-irr-check/) |
| `show bgp irr prefix <peer>` | Read-only | - | - | - | - | [details](show-bgp-irr-prefix/) |
| `show bgp peer <selector> capabilities` | Read-only | - | - | - | - | [details](show-bgp-peer-capabilities/) |
| `show bgp peer <selector> detail` | Read-only | - | - | - | - | [details](show-bgp-peer-detail/) |
| `show bgp peer <selector> history` | Read-only | - | - | - | - | [details](show-bgp-peer-history/) |
| `show bgp peer list` | Read-only | - | - | - | - | [details](show-bgp-peer-list/) |
| `show bgp peer <selector> rib [scope\|filters\|terminal]` | Read-only | `show route advertising-protocol bgp <peer>`<br>`show route receive-protocol bgp <peer>` | - | - | - | [details](show-bgp-peer-rib/) |
| `show bgp peer <selector> statistics` | Read-only | - | - | - | - | [details](show-bgp-peer-statistics/) |
| `show bgp rib` | Read-only | - | - | - | - | [details](show-bgp-rib/) |
| `show bgp rib best` | Read-only | `show route <prefix> protocol bgp`<br>`show route protocol bgp` | `show bgp ipv4 unicast`<br>`show bgp ipv4 unicast <prefix>` | - | `show ip bgp <prefix>` | [details](show-bgp-rib-best/) |
| `show bgp rib best status` | Read-only | - | - | - | - | [details](show-bgp-rib-best-status/) |
| `show bgp rib rpf` | Read-only | - | - | - | - | [details](show-bgp-rib-rpf/) |
| `show bgp rib status` | Read-only | - | - | - | - | [details](show-bgp-rib-status/) |
| `show bgp summary` | Read-only | `show bgp summary` | `show bgp ipv4 unicast summary`<br>`show bgp summary` | `show router bgp summary` | `show bgp ipv4 summary`<br>`show ip bgp summary` | [details](show-bgp-summary/) |
| `show bmp collectors` | Read-only | - | - | - | - | [details](show-bmp-collectors/) |
| `show bmp peers` | Read-only | - | - | - | - | [details](show-bmp-peers/) |
| `show bmp rib` | Read-only | - | - | - | - | [details](show-bmp-rib/) |
| `show bmp sessions` | Read-only | - | - | - | - | [details](show-bmp-sessions/) |
| `show cache` | Read-only | - | - | - | - | [details](show-cache/) |
| `show capture [<count>] [<peer>] [<protocol>] [<tunnel-id>]` | Read-only | - | - | - | - | [details](show-capture/) |
| `show capture interface [<count>] [<duration>] [<format>] [<iface>] [<protocol>] [<snap-len>]` | Read-only | - | - | - | - | [details](show-capture-interface/) |
| `show capture raw [<action>] [<count>] [<format>] [<protocol>]` | Read-only | - | - | - | - | [details](show-capture-raw/) |
| `show command complete` | Read-only | - | - | - | - | [details](show-command-complete/) |
| `show command help` | Read-only | - | - | - | - | [details](show-command-help/) |
| `show command list` | Read-only | - | - | - | - | [details](show-command-list/) |
| `show config cat <id>` | Read-only | - | - | - | - | [details](show-config-cat/) |
| `show config diff` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-diff/) |
| `show config dump` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-dump/) |
| `show config fmt` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-fmt/) |
| `show config graph` | Offline | - | - | - | - | [details](show-config-graph/) |
| `show config history` | Read-only | - | - | - | `compare`<br>`show configuration` | [details](show-config-history/) |
| `show config ls` | Read-only | - | - | - | - | [details](show-config-ls/) |
| `show crashes [<name>]` | Read-only | - | - | - | - | [details](show-crashes/) |
| `show data cat <key>` | Read-only | - | - | - | - | [details](show-data-cat/) |
| `show data ls` | Read-only | - | - | - | - | [details](show-data-ls/) |
| `show data registered` | Read-only | - | - | - | - | [details](show-data-registered/) |
| `show ddos flowspec` | Read-only | - | - | - | - | [details](show-ddos-flowspec/) |
| `show ddos incidents` | Read-only | - | - | - | - | [details](show-ddos-incidents/) |
| `show ddos local` | Read-only | - | - | - | - | [details](show-ddos-local/) |
| `show ddos status` | Read-only | - | - | - | - | [details](show-ddos-status/) |
| `show debug` | Read-only | - | - | - | - | [details](show-debug/) |
| `show debug profile` | Offline | - | - | - | - | [details](show-debug-profile/) |
| `show dns cache list` | Read-only | - | - | - | `show dns` | [details](show-dns-cache-list/) |
| `show dns cache record <name>` | Read-only | - | - | - | `show dns` | [details](show-dns-cache-record/) |
| `show dns cache stats` | Read-only | - | - | - | `show dns` | [details](show-dns-cache-stats/) |
| `show dns lookup <hostname> [<type>]` | Read-only | - | - | - | `show dns` | [details](show-dns-lookup/) |
| `show doctor` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-doctor/) |
| `show env get <name>` | Read-only | - | - | - | - | [details](show-env-get/) |
| `show env list` | Read-only | - | - | - | - | [details](show-env-list/) |
| `show env registered` | Read-only | - | - | - | - | [details](show-env-registered/) |
| `show errors` | Read-only | - | - | - | `show log` | [details](show-errors/) |
| `show event delivery` | Read-only | - | - | - | - | [details](show-event-delivery/) |
| `show event list` | Read-only | - | - | - | - | [details](show-event-list/) |
| `show event namespaces` | Read-only | - | - | - | - | [details](show-event-namespaces/) |
| `show event recent` | Read-only | - | - | - | - | [details](show-event-recent/) |
| `show firewall group` | Read-only | - | - | - | - | [details](show-firewall-group/) |
| `show firewall irr` | Read-only | - | - | - | - | [details](show-firewall-irr/) |
| `show firewall irr prefix <asn-or-as-set>` | Read-only | - | - | - | - | [details](show-firewall-irr-prefix/) |
| `show firewall ruleset <name>` | Read-only | - | - | - | - | [details](show-firewall-ruleset/) |
| `show flow export [<name>]` | Read-only | - | - | - | - | [details](show-flow-export/) |
| `show flow recent [<dst>]` | Read-only | - | - | - | - | [details](show-flow-recent/) |
| `show geodns` | Read-only | - | - | - | - | [details](show-geodns/) |
| `show gnmi` | Read-only | - | - | - | - | [details](show-gnmi/) |
| `show health` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-health/) |
| `show host` | Read-only | - | - | - | - | [details](show-host/) |
| `show host all` | Read-only | - | - | - | - | [details](show-host-all/) |
| `show host cpu` | Read-only | - | - | - | - | [details](show-host-cpu/) |
| `show host dmi` | Read-only | - | - | - | - | [details](show-host-dmi/) |
| `show host kernel` | Read-only | - | - | - | - | [details](show-host-kernel/) |
| `show host memory` | Read-only | - | - | - | - | [details](show-host-memory/) |
| `show host nic` | Read-only | - | - | - | - | [details](show-host-nic/) |
| `show host platform` | Read-only | - | - | - | - | [details](show-host-platform/) |
| `show host storage` | Read-only | - | - | - | - | [details](show-host-storage/) |
| `show host thermal` | Read-only | - | - | - | - | [details](show-host-thermal/) |
| `show interface` | Read-only | - | - | - | - | [details](show-interface/) |
| `show interface brief` | Read-only | - | - | - | `show interfaces` | [details](show-interface-brief/) |
| `show interface errors` | Read-only | - | - | - | - | [details](show-interface-errors/) |
| `show interface name <name> counters` | Read-only | - | - | - | - | [details](show-interface-name-counters/) |
| `show interface name <name> detail` | Read-only | - | - | - | `show interfaces <name>` | [details](show-interface-name-detail/) |
| `show interface rate` | Read-only | - | - | - | - | [details](show-interface-rate/) |
| `show interface scan` | Read-only | - | - | - | - | [details](show-interface-scan/) |
| `show interface type <type>` | Read-only | - | - | - | - | [details](show-interface-type/) |
| `show isis database` | Read-only | - | - | - | - | [details](show-isis-database/) |
| `show isis database detail` | Read-only | - | - | - | - | [details](show-isis-database-detail/) |
| `show isis hostname` | Read-only | - | - | - | - | [details](show-isis-hostname/) |
| `show isis interface` | Read-only | - | - | - | - | [details](show-isis-interface/) |
| `show isis neighbor` | Read-only | - | - | - | - | [details](show-isis-neighbor/) |
| `show isis route` | Read-only | - | - | - | - | [details](show-isis-route/) |
| `show isis route ipv6` | Read-only | - | - | - | - | [details](show-isis-route-ipv6/) |
| `show isis spf-log` | Read-only | - | - | - | - | [details](show-isis-spf-log/) |
| `show l2tp` | Read-only | - | - | - | - | [details](show-l2tp/) |
| `show l2tp config` | Read-only | - | - | - | - | [details](show-l2tp-config/) |
| `show l2tp cqm` | Read-only | - | - | - | - | [details](show-l2tp-cqm/) |
| `show l2tp echo` | Read-only | - | - | - | - | [details](show-l2tp-echo/) |
| `show l2tp health` | Read-only | - | - | - | - | [details](show-l2tp-health/) |
| `show l2tp listeners` | Read-only | - | - | - | - | [details](show-l2tp-listeners/) |
| `show l2tp observer` | Read-only | - | - | - | - | [details](show-l2tp-observer/) |
| `show l2tp reliable` | Read-only | - | - | - | - | [details](show-l2tp-reliable/) |
| `show l2tp session history` | Read-only | - | - | - | - | [details](show-l2tp-session-history/) |
| `show l2tp session <id>` | Read-only | - | - | - | - | [details](show-l2tp-session-id/) |
| `show l2tp session traffic` | Read-only | - | - | - | - | [details](show-l2tp-session-traffic/) |
| `show l2tp sessions` | Read-only | - | - | - | - | [details](show-l2tp-sessions/) |
| `show l2tp statistics` | Read-only | - | - | - | - | [details](show-l2tp-statistics/) |
| `show l2tp tunnel history` | Read-only | - | - | - | - | [details](show-l2tp-tunnel-history/) |
| `show l2tp tunnel <id>` | Read-only | - | - | - | - | [details](show-l2tp-tunnel-id/) |
| `show l2tp tunnels` | Read-only | - | - | - | - | [details](show-l2tp-tunnels/) |
| `show ldp binding` | Read-only | - | - | - | - | [details](show-ldp-binding/) |
| `show ldp neighbor` | Read-only | - | - | - | - | [details](show-ldp-neighbor/) |
| `show log levels` | Read-only | - | - | - | - | [details](show-log-levels/) |
| `show log recent [<component>] [<count>] [<level>]` | Read-only | - | - | - | `show log` | [details](show-log-recent/) |
| `show metrics list` | Read-only | - | - | - | - | [details](show-metrics-list/) |
| `show metrics name <name> [label=value` | Read-only | - | - | - | - | [details](show-metrics-name/) |
| `show metrics pool` | Read-only | - | - | - | - | [details](show-metrics-pool/) |
| `show metrics values` | Read-only | - | - | - | - | [details](show-metrics-values/) |
| `show mpls forwarding [<limit>]` | Read-only | - | - | - | - | [details](show-mpls-forwarding/) |
| `show neighbor [<family>]` | Read-only | - | - | - | `show arp` | [details](show-neighbor/) |
| `show ospf` | Read-only | - | - | - | - | [details](show-ospf/) |
| `show ospf border-routers` | Read-only | - | - | - | - | [details](show-ospf-border-routers/) |
| `show ospf database` | Read-only | - | - | - | - | [details](show-ospf-database/) |
| `show ospf database asbr-summary` | Read-only | - | - | - | - | [details](show-ospf-database-asbr-summary/) |
| `show ospf database external` | Read-only | - | - | - | - | [details](show-ospf-database-external/) |
| `show ospf database network` | Read-only | - | - | - | - | [details](show-ospf-database-network/) |
| `show ospf database nssa-external` | Read-only | - | - | - | - | [details](show-ospf-database-nssa-external/) |
| `show ospf database opaque-area` | Read-only | - | - | - | - | [details](show-ospf-database-opaque-area/) |
| `show ospf database opaque-area detail` | Read-only | - | - | - | - | [details](show-ospf-database-opaque-area-detail/) |
| `show ospf database opaque-as` | Read-only | - | - | - | - | [details](show-ospf-database-opaque-as/) |
| `show ospf database opaque-as detail` | Read-only | - | - | - | - | [details](show-ospf-database-opaque-as-detail/) |
| `show ospf database opaque-link` | Read-only | - | - | - | - | [details](show-ospf-database-opaque-link/) |
| `show ospf database opaque-link detail` | Read-only | - | - | - | - | [details](show-ospf-database-opaque-link-detail/) |
| `show ospf database router` | Read-only | - | - | - | - | [details](show-ospf-database-router/) |
| `show ospf database router-information` | Read-only | - | - | - | - | [details](show-ospf-database-router-information/) |
| `show ospf database summary` | Read-only | - | - | - | - | [details](show-ospf-database-summary/) |
| `show ospf graceful-restart` | Read-only | - | - | - | - | [details](show-ospf-graceful-restart/) |
| `show ospf instance` | Read-only | - | - | - | - | [details](show-ospf-instance/) |
| `show ospf interface` | Read-only | - | - | - | - | [details](show-ospf-interface/) |
| `show ospf interface detail` | Read-only | - | - | - | - | [details](show-ospf-interface-detail/) |
| `show ospf ipv6` | Read-only | - | - | - | - | [details](show-ospf-ipv6/) |
| `show ospf ipv6 database` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database/) |
| `show ospf ipv6 database detail` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-detail/) |
| `show ospf ipv6 database extended` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-extended/) |
| `show ospf ipv6 database router detail` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-router-detail/) |
| `show ospf ipv6 database router-information` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-router-information/) |
| `show ospf ipv6 database scope area` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-scope-area/) |
| `show ospf ipv6 database scope as` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-scope-as/) |
| `show ospf ipv6 database scope link` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-scope-link/) |
| `show ospf ipv6 database segment-routing` | Read-only | - | - | - | - | [details](show-ospf-ipv6-database-segment-routing/) |
| `show ospf ipv6 graceful-restart` | Read-only | - | - | - | - | [details](show-ospf-ipv6-graceful-restart/) |
| `show ospf ipv6 instance` | Read-only | - | - | - | - | [details](show-ospf-ipv6-instance/) |
| `show ospf ipv6 interface` | Read-only | - | - | - | - | [details](show-ospf-ipv6-interface/) |
| `show ospf ipv6 interface detail` | Read-only | - | - | - | - | [details](show-ospf-ipv6-interface-detail/) |
| `show ospf ipv6 neighbor` | Read-only | - | - | - | - | [details](show-ospf-ipv6-neighbor/) |
| `show ospf ipv6 neighbor detail` | Read-only | - | - | - | - | [details](show-ospf-ipv6-neighbor-detail/) |
| `show ospf ipv6 segment-routing` | Read-only | - | - | - | - | [details](show-ospf-ipv6-segment-routing/) |
| `show ospf ipv6 spf` | Read-only | - | - | - | - | [details](show-ospf-ipv6-spf/) |
| `show ospf ipv6 spf detail` | Read-only | - | - | - | - | [details](show-ospf-ipv6-spf-detail/) |
| `show ospf ldp-sync` | Read-only | - | - | - | - | [details](show-ospf-ldp-sync/) |
| `show ospf neighbor` | Read-only | - | - | - | - | [details](show-ospf-neighbor/) |
| `show ospf neighbor detail` | Read-only | - | - | - | - | [details](show-ospf-neighbor-detail/) |
| `show ospf route` | Read-only | - | - | - | - | [details](show-ospf-route/) |
| `show ospf route fast-reroute` | Read-only | - | - | - | - | [details](show-ospf-route-fast-reroute/) |
| `show ospf segment-routing` | Read-only | - | - | - | - | [details](show-ospf-segment-routing/) |
| `show ospf spf` | Read-only | - | - | - | - | [details](show-ospf-spf/) |
| `show ospf spf detail` | Read-only | - | - | - | - | [details](show-ospf-spf-detail/) |
| `show ospf te-database` | Read-only | - | - | - | - | [details](show-ospf-te-database/) |
| `show ospf virtual-links` | Read-only | - | - | - | - | [details](show-ospf-virtual-links/) |
| `show ping [<count>] [<dest>] [<size>] [<timeout>]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](show-ping/) |
| `show pki certificate name <name> [pem \| bundle pem \| fingerprint` | Read-only | - | - | - | - | [details](show-pki-certificate-name/) |
| `show pki certificates` | Read-only | - | - | - | - | [details](show-pki-certificates/) |
| `show policy chain peer <selector> [import\|export]` | Read-only | - | - | - | - | [details](show-policy-chain-peer/) |
| `show policy list` | Read-only | - | - | - | - | [details](show-policy-list/) |
| `show policy routes` | Read-only | - | - | - | - | [details](show-policy-routes/) |
| `show policy test peer <selector> import\|export [filter <name>]` | Read-only | - | - | - | - | [details](show-policy-test-peer/) |
| `show pppoe` | Read-only | - | - | - | - | [details](show-pppoe/) |
| `show pppoe interfaces` | Read-only | - | - | - | - | [details](show-pppoe-interfaces/) |
| `show pppoe session <id>` | Read-only | - | - | - | - | [details](show-pppoe-session-id/) |
| `show pppoe sessions` | Read-only | - | - | - | - | [details](show-pppoe-sessions/) |
| `show pppoe statistics` | Read-only | - | - | - | - | [details](show-pppoe-statistics/) |
| `show probe-round [<dest>] [<max-hops>] [<probes>] [<timeout>]` | Read-only | - | - | - | - | [details](show-probe-round/) |
| `show reload-status` | Read-only | - | - | - | - | [details](show-reload-status/) |
| `show route [<limit>] [<prefix>]` | Read-only | `show route` | - | - | `show ip route` | [details](show-route/) |
| `show route lookup` | Read-only | `show route <prefix>` | - | - | `show ip route <prefix>` | [details](show-route-lookup/) |
| `show rr peers` | Read-only | - | - | - | - | [details](show-rr-peers/) |
| `show rr status` | Read-only | - | - | - | - | [details](show-rr-status/) |
| `show rsvp-te fast-reroute` | Read-only | - | - | - | - | [details](show-rsvp-te-fast-reroute/) |
| `show rsvp-te interface` | Read-only | - | - | - | - | [details](show-rsvp-te-interface/) |
| `show rsvp-te lsp` | Read-only | - | - | - | - | [details](show-rsvp-te-lsp/) |
| `show rsvp-te tunnel` | Read-only | - | - | - | - | [details](show-rsvp-te-tunnel/) |
| `show runtime memory` | Read-only | - | - | - | - | [details](show-runtime-memory/) |
| `show schema events` | Read-only | - | - | - | - | [details](show-schema-events/) |
| `show schema handlers` | Read-only | - | - | - | - | [details](show-schema-handlers/) |
| `show schema list` | Read-only | - | - | - | - | [details](show-schema-list/) |
| `show schema methods` | Read-only | - | - | - | - | [details](show-schema-methods/) |
| `show schema protocol` | Read-only | - | - | - | - | [details](show-schema-protocol/) |
| `show static` | Read-only | - | - | - | - | [details](show-static/) |
| `show status` | Read-only | - | - | - | - | [details](show-status/) |
| `show storage smart` | Read-only | - | - | - | - | [details](show-storage-smart/) |
| `show subscriber` | Read-only | - | - | - | - | [details](show-subscriber/) |
| `show subscriber <id> detail` | Read-only | - | - | - | - | [details](show-subscriber-id-detail/) |
| `show system conntrack` | Read-only | - | - | - | - | [details](show-system-conntrack/) |
| `show system cpu` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-system-cpu/) |
| `show system date` | Read-only | - | - | - | `show date`<br>`show ntp` | [details](show-system-date/) |
| `show system file-descriptors [<mode>]` | Read-only | - | - | - | - | [details](show-system-file-descriptors/) |
| `show system goroutines [<mode>]` | Read-only | - | - | - | - | [details](show-system-goroutines/) |
| `show system kernel-log [<count>] [<level>]` | Read-only | - | - | - | - | [details](show-system-kernel-log/) |
| `show system memory` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-system-memory/) |
| `show system ntp` | Read-only | - | - | - | `show date`<br>`show ntp` | [details](show-system-ntp/) |
| `show system ntp peers` | Read-only | - | - | - | `show date`<br>`show ntp` | [details](show-system-ntp-peers/) |
| `show system platform` | Read-only | - | - | - | `show hardware cpu`<br>`show system memory` | [details](show-system-platform/) |
| `show system profile [<duration>] [<type>]` | Read-only | - | - | - | - | [details](show-system-profile/) |
| `show system sockets [<port>] [<protocol>] [<state>]` | Read-only | - | - | - | - | [details](show-system-sockets/) |
| `show system subsystem list` | Read-only | - | - | - | - | [details](show-system-subsystem-list/) |
| `show system update` | Read-only | - | - | - | - | [details](show-system-update/) |
| `show system update history` | Read-only | - | - | - | - | [details](show-system-update-history/) |
| `show tcp-check <host> <port> [<source>] [<timeout>]` | Read-only | - | - | - | - | [details](show-tcp-check/) |
| `show traceroute [<dest>] [<max-hops>] [<probes>] [<timeout>]` | Read-only | - | - | `ping <target>` | `ping <target>`<br>`traceroute <target>` | [details](show-traceroute/) |
| `show traffic control` | Read-only | - | - | - | - | [details](show-traffic-control/) |
| `show traffic feature [<name>]` | Read-only | - | - | - | - | [details](show-traffic-feature/) |
| `show traffic stat [<name>]` | Read-only | - | - | - | - | [details](show-traffic-stat/) |
| `show traffic usage [<name>]` | Read-only | - | - | - | - | [details](show-traffic-usage/) |
| `show uptime` | Read-only | - | - | - | `show system uptime` | [details](show-uptime/) |
| `show version` | Read-only | - | - | - | `show system uptime` | [details](show-version/) |
| `show vpn ipsec dataplane drift` | Read-only | - | - | - | - | [details](show-vpn-ipsec-dataplane-drift/) |
| `show vpn ipsec dataplane policy` | Read-only | - | - | - | - | [details](show-vpn-ipsec-dataplane-policy/) |
| `show vpn ipsec dataplane sa [<spi>]` | Read-only | - | - | - | - | [details](show-vpn-ipsec-dataplane-sa/) |
| `show vpn ipsec peer name <name>` | Read-only | - | - | - | - | [details](show-vpn-ipsec-peer-name/) |
| `show vpn ipsec sa` | Read-only | - | - | - | - | [details](show-vpn-ipsec-sa/) |
| `show vpn ipsec status` | Read-only | - | - | - | - | [details](show-vpn-ipsec-status/) |
| `show vpp runtime` | Read-only | - | - | - | - | [details](show-vpp-runtime/) |
| `show vpp trace clear` | Read-only | - | - | - | - | [details](show-vpp-trace-clear/) |
| `show vpp trace show` | Read-only | - | - | - | - | [details](show-vpp-trace-show/) |
| `show vpp trace start` | Read-only | - | - | - | - | [details](show-vpp-trace-start/) |
| `show vrrp` | Read-only | - | - | - | - | [details](show-vrrp/) |
| `show vrrp interface name [<value>]` | Read-only | - | - | - | - | [details](show-vrrp-interface-name/) |
| `show vrrp statistics` | Read-only | - | - | - | - | [details](show-vrrp-statistics/) |
| `show warnings` | Read-only | - | - | - | `show log` | [details](show-warnings/) |
| `show yang completion` | Read-only | - | - | - | - | [details](show-yang-completion/) |
| `show yang doc` | Read-only | - | - | - | - | [details](show-yang-doc/) |
| `show yang tree` | Read-only | - | - | - | - | [details](show-yang-tree/) |

### skills

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `skills` | Offline | - | - | - | - | [details](skills/) |

### support

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `support` | Offline | - | - | - | - | [details](support/) |

### system

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `system command complete` | Read-only | - | - | - | - | [details](system-command-complete/) |
| `system command help` | Read-only | - | - | - | - | [details](system-command-help/) |
| `system command list` | Read-only | - | - | - | - | [details](system-command-list/) |
| `system dispatch` | Read-only | - | - | - | - | [details](system-dispatch/) |
| `system help` | Read-only | - | - | - | - | [details](system-help/) |
| `system subsystem list` | Read-only | - | - | - | - | [details](system-subsystem-list/) |
| `system version api` | Read-only | - | - | - | - | [details](system-version-api/) |
| `system version software` | Read-only | - | - | - | - | [details](system-version-software/) |

### update

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `update bgp irr all` | Daemon | - | - | - | - | [details](update-bgp-irr-all/) |
| `update bgp irr as-set <as-set>` | Daemon | - | - | - | - | [details](update-bgp-irr-as-set/) |
| `update bgp irr asn <asn>` | Daemon | - | - | - | - | [details](update-bgp-irr-asn/) |
| `update bgp peer <selector> prefix` | Daemon | - | - | - | - | [details](update-bgp-peer-prefix/) |
| `update firewall irr all` | Daemon | - | - | - | - | [details](update-firewall-irr-all/) |
| `update firewall irr as-set <as-set>` | Daemon | - | - | - | - | [details](update-firewall-irr-as-set/) |
| `update firewall irr asn <asn>` | Daemon | - | - | - | - | [details](update-firewall-irr-asn/) |
| `update serve` | Offline | - | - | - | - | [details](update-serve/) |
| `update system firmware apply` | Daemon | - | - | - | - | [details](update-system-firmware-apply/) |
| `update system firmware check` | Daemon | - | - | - | - | [details](update-system-firmware-check/) |
| `update system firmware download` | Daemon | - | - | - | - | [details](update-system-firmware-download/) |
| `update system firmware restart` | Daemon | - | - | - | - | [details](update-system-firmware-restart/) |
| `update system firmware rollback` | Daemon | - | - | - | - | [details](update-system-firmware-rollback/) |

### validate

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `validate config` | Offline | - | - | - | `commit` | [details](validate-config/) |

### withdraw

| Ze | Mode | Junos MX | IOS XR | SR OS | VyOS | Details |
| --- | --- | --- | --- | --- | --- | --- |
| `withdraw tag <key> <value\|*> \| withdraw tag * \| withdraw id <N> \| withdraw all` | Daemon | - | - | - | - | [details](withdraw/) |

## Vendor-only gaps

| Intent | Junos MX | IOS XR | SR OS | VyOS | Notes |
| --- | --- | --- | --- | --- | --- |
| LLDP neighbor discovery | - | - | - | - | No Ze LLDP subsystem is present in the live command catalog. |
| NAT translations and pools | - | - | - | - | No Ze NAT command is present in the live command catalog. |
