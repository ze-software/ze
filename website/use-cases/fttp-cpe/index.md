# Home and SMB FTTP CPE

Use Ze as a single-box Linux CPE on a PPPoE fiber circuit: terminate PPPoE on the WAN interface, request an IPv6 prefix via DHCPv6-PD, advertise delegated `/64` subnets on the LAN via Router Advertisements, serve DHCPv4 leases, forward LAN DNS queries to upstream resolvers learned over PPPoE, and enforce TCP MSS clamping and source NAT with nftables.

## Role and topology

```text
ISP / BNG (accel-ppp)
       │ PPPoE + IPv6CP + DHCPv6-PD (/56 or /60)
       ▼ eth1 -> pppoe0 (MTU 1492)
   Ze CPE (Linux, kernel backend)
       │ lan0 / br0 (192.168.50.1/24 + delegated /64)
       ▼
  LAN hosts (DHCPv4 + IPv6 SLAAC + local caching DNS)
```

## Ze configuration

WAN PPPoE client with IPv6CP, DHCPv6-PD, and LAN delegated-prefix Router Advertisements (based on `test/interop-topology/scenarios/pppoe-dhcpv6-pd-ze-client/ze-cpe.conf`):

```text
interface {
    ethernet eth1 {
    }
    ethernet lan0 {
        unit 0 {
            ipv6 {
                forwarding true;
                delegated-prefix pppoe0 {
                    subnet-id 1;
                }
                router-advertisement {
                    enabled true;
                    maximum-interval 4;
                    minimum-interval 3;
                    router-lifetime 1800;
                }
            }
        }
    }
    pppoe-client pppoe0 {
        source-interface eth1;
        mtu 1492;
        service-name internet;
        authentication {
            username alice;
            password s3cr3t;
        }
        ipv6 {
            enabled true;
            accept-ra 2;
            autoconf true;
            forwarding true;
            dhcpv6 {
                enabled true;
                pd {
                    length 56;
                }
            }
        }
    }
}
```

Default IPv4 route, nftables TCP MSS clamping (avoiding PMTUD blackholes across the 1492-byte PPPoE MTU), IPv4 NAT masquerade, LAN DHCPv4 server, and LAN caching DNS resolver forwarding to PPPoE-learned upstream resolvers with a static fallback (based on `test/interop-topology/cpe/ze-cpe.conf` and `test/pppoe/dns-resolver-forward-ppp-lease.ci`):

```text
static {
    table default {
        route 0.0.0.0/0 {
            distance 10;
            next {
                interface pppoe0 {
                }
            }
        }
    }
}

firewall {
    backend kernel;
    table wan-mss {
        family ip;
        chain mss {
            type filter;
            hook postrouting;
            priority -300;
            policy accept;
            term pppoe0-mss {
                from { output-interface pppoe0; protocol tcp; }
                then { tcp-mss-set 1400; }
            }
        }
    }
    table nat-source {
        family ip;
        chain postrouting {
            type nat;
            hook postrouting;
            priority 100;
            policy accept;
            term rule-100-a {
                from { output-interface pppoe0; source-address 192.168.50.0/24; }
                then { masquerade; }
            }
        }
    }
}

service {
    dhcp-server {
        enabled true;
        listen-interface br0;
        shared-network LAN {
            subnet 192.168.50.0/24 {
                range 0 {
                    start 192.168.50.100;
                    stop 192.168.50.199;
                }
                lease-time 86400;
                default-router 192.168.50.1;
                dns-source {
                    source self;
                }
                static-mapping res-a {
                    mac-address 02:00:5e:50:00:30;
                    ip-address 192.168.50.30;
                }
            }
        }
    }

    dns-resolver {
        enabled true;
        listener lan {
            ip 192.168.50.1;
            port 53;
        }
        allow-from [ 192.168.50.0/24 127.0.0.0/8 ];
        forward {
            server 192.0.2.53 {
                port 53;
            }
            dns-source {
                source [ received configured ];
                received-interface [ pppoe0 ];
                mode first-usable;
            }
        }
    }
}
```

## Access concentrator configuration (accel-ppp)

Tested against accel-ppp 1.14.0 (`test/interop-topology/scenarios/pppoe-dhcpv6-pd-ze-client/accel-ppp.conf`):

```ini
[modules]
log_file
cli
pppoe
auth_pap
chap-secrets
ippool
ipv6pool
ipv6_nd
ipv6_dhcp

[pppoe]
interface=eth1
ac-name=ze-pd-ac
service-name=internet
verbose=1

[ppp]
mtu=1492
mru=1492
ipv4=require
ipv6=allow
ipv6-intf-id=0:0:0:1
ipv6-accept-peer-intf-id=1
lcp-echo-interval=30
lcp-echo-failure=3
mppe=deny

[ip-pool]
gw-ip-address=10.198.0.1
10.198.0.2-10

[ipv6-pool]
2001:db8:198::/48,64
delegate=2001:db8:2000::/40,60

[ipv6-nd]
AdvAutonomousFlag=1

[ipv6-dhcp]
pref-lifetime=60
valid-lifetime=120
route-via-gw=1
```

## Verification commands

```text
show interface pppoe-client
show interface delegated-prefix
show system dns
show dns resolver
show route
ze doctor
```

- `show interface pppoe-client` reports session state, negotiated addresses, learned WAN DNS servers, and delegated IPv6 prefixes with remaining lifetimes.
- `show interface delegated-prefix` reports each LAN unit's delegated `/64`, interface address, and prefix lifetimes.
- `show system dns` shows learned WAN DNS records alongside the upstream server selected by `dns-resolver` and the local address advertised by `dhcp-server`.

## Lab evidence

- **PPPoE + DHCPv6-PD + RA against accel-ppp:** `./le test integration interop-topology scenario pppoe-dhcpv6-pd-ze-client` (`test/interop-topology/scenarios/pppoe-dhcpv6-pd-ze-client/`)
- **Full CPE over PPPoE -> LAC -> L2TP -> LNS with DHCP, NAT, and MSS:** `./le test integration interop-topology scenario access-up`, `access-reconnect`, and `lan-services` (`test/interop-topology/cpe/ze-cpe.conf`)
- **PPPoE client interop against accel-ppp:** `ZE_PPPOE_INTEROP_SCENARIO=01-pppoe-chap-ipv4 ./le test deployment docker-pppoe-accel` (and `./le test qemu pppoe-accel`)
- **WAN DNS lease forwarding and DHCP self-DNS:** `test/pppoe/dns-resolver-forward-ppp-lease.ci`, `test/dhcp/dhcp-server-dns-source-self.ci`, `test/plugin/show-interface-pppoe-client.ci`, and `test/plugin/show-system-dns.ci`

## Current limits

- **WAN client is PPPoE only:** Ze has `interface { pppoe-client }` for CPE WANs. Ze's L2TP implementation is an RFC 2661 BNG LNS/LAC rather than a CPE L2TP client.
- **DHCPv4 server only:** `service { dhcp-server }` serves IPv4 leases; LAN IPv6 hosts autoconfigure via SLAAC and Router Advertisements. There is no `show dhcp` lease table command yet.
- **Linux kernel backend required:** `pppoe-client` and `router-advertisement` require Linux with `backend kernel`, and Router Advertisements require `ipv6 { forwarding true; }`.
- **Forwarding DNS resolver:** `service { dns-resolver }` operates in forwarding mode over plain UDP/TCP only (no full root recursion, DNSSEC validation, DoT/DoH, or wildcard `0.0.0.0`/`::` listeners).

## Related guides

- [PPPoE guide](../../guides/pppoe/)
- [DNS resolver guide](../../guides/dns-resolver/)
- [Interfaces](../../features/interfaces/)
- [PPPoE interop lab](../../labs/pppoe-interop/)
