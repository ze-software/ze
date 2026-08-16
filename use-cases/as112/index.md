# AS112 anycast DNS inside a network

Run Ze as the AS112 sink for RFC 1918 and link-local reverse DNS, then learn the four AS112 covering prefixes from Ze over BGP.

This is a local-network deployment shape. If you intend to advertise an AS112 node to public Internet peers, coordinate it first as RFC 7534 expects, and keep the import and export filters below. The software cannot do that human coordination for you.

## Topology

| Item | Value in this example |
| --- | --- |
| Ze node | as112-ze |
| Ze BGP transport, IPv4 | 172.30.0.2 |
| Core router BGP transport, IPv4 | 172.30.0.1 |
| Ze BGP transport, IPv6 | 2001:db8:112::2 |
| Core router BGP transport, IPv6 | 2001:db8:112::1 |
| Ze BGP ASN | 65001 |
| Core router ASN | 65000 |
| AS112 origin ASN carried by the routes | 112 |
| IPv4 covering prefixes to accept | 192.175.48.0/24, 192.31.196.0/24 |
| IPv6 covering prefixes to accept | 2620:4f:8000::/48, 2001:4:112::/48 |
| DNS service addresses hosted by Ze | 192.175.48.1, 192.31.196.1, 2620:4f:8000::1, 2001:4:112::1 |

The prefixes you route are the covering `/24` and `/48` prefixes. The host addresses are the DNS service addresses Ze binds on `lo`; do not import or advertise `/32` or `/128` host routes for AS112.

## Ze config

`import as112` is the trigger. Without it, Ze can answer AS112 DNS locally, but BGP will not receive the AS112 covering prefixes.

```text
service {
    as112 {
        enabled true
        hostname "as112-ldn1.example.net"
        facility "Example London"
        location "London, GB"
        asn 112
        community [ no-export ]
        watchdog true
    }
}

bgp {
    router-id 172.30.0.2

    session {
        asn {
            local 65001
        }
    }

    peer core-v4 {
        connection {
            remote {
                ip 172.30.0.1
            }
            local {
                ip 172.30.0.2
            }
        }
        session {
            asn {
                remote 65000
            }
            family {
                ipv4/unicast {
                    prefix {
                        maximum 1000
                    }
                }
            }
        }
    }

    peer core-v6 {
        connection {
            remote {
                ip 2001:db8:112::1
            }
            local {
                ip 2001:db8:112::2
            }
        }
        session {
            asn {
                remote 65000
            }
            family {
                ipv6/unicast {
                    prefix {
                        maximum 1000
                    }
                }
            }
        }
    }
}

redistribute {
    destination bgp {
        import as112
    }
}
```

What this does:

- Ze answers AS112 DNS authoritatively on the four fixed service addresses.
- Ze registers those service addresses on `lo`; you do not configure them by hand.
- Ze originates the two IPv4 `/24` and two IPv6 `/48` covering prefixes into BGP.
- `watchdog true` keeps the BGP routes withdrawn until the DNS listener is serving.
- The top-level `session` sets Ze's local ASN once; each peer declares only the remote ASN.
- `community [ no-export ]` marks the routes so the network can keep them local. Use `nopeer` instead, or in addition, for a bilateral AS112 peering policy.

## FRR core router

This FRR config accepts only the AS112 covering prefixes from Ze and exports nothing back to the Ze node.

```text
frr defaults traditional
hostname core-frr
log file /tmp/frr.log informational
!
ip prefix-list AS112-V4 seq 10 permit 192.175.48.0/24
ip prefix-list AS112-V4 seq 20 permit 192.31.196.0/24
ipv6 prefix-list AS112-V6 seq 10 permit 2620:4f:8000::/48
ipv6 prefix-list AS112-V6 seq 20 permit 2001:4:112::/48
!
route-map AS112-V4-IN permit 10
 match ip address prefix-list AS112-V4
!
route-map AS112-V4-IN deny 100
!
route-map AS112-V6-IN permit 10
 match ipv6 address prefix-list AS112-V6
!
route-map AS112-V6-IN deny 100
!
route-map DROP-ALL deny 10
!
router bgp 65000
 bgp router-id 172.30.0.1
 no bgp ebgp-requires-policy
 neighbor 172.30.0.2 remote-as 65001
 neighbor 2001:db8:112::2 remote-as 65001
 !
 address-family ipv4 unicast
  neighbor 172.30.0.2 activate
  neighbor 172.30.0.2 route-map AS112-V4-IN in
  neighbor 172.30.0.2 route-map DROP-ALL out
 exit-address-family
 !
 address-family ipv6 unicast
  neighbor 2001:db8:112::2 activate
  neighbor 2001:db8:112::2 route-map AS112-V6-IN in
  neighbor 2001:db8:112::2 route-map DROP-ALL out
 exit-address-family
!
```

Useful checks:

```text
show bgp ipv4 unicast 192.175.48.0/24
show bgp ipv4 unicast 192.31.196.0/24
show bgp ipv6 unicast 2620:4f:8000::/48
show bgp ipv6 unicast 2001:4:112::/48
```

## BIRD core router

This BIRD 2 config uses one IPv4 BGP session and one IPv6 BGP session, both receive-only.

```text
log stderr all;
router id 172.30.0.1;

protocol device {
}

filter as112_v4_in {
    if net = 192.175.48.0/24 then accept;
    if net = 192.31.196.0/24 then accept;
    reject;
}

filter as112_v6_in {
    if net = 2620:4f:8000::/48 then accept;
    if net = 2001:4:112::/48 then accept;
    reject;
}

protocol bgp ze_as112_v4 {
    local 172.30.0.1 as 65000;
    neighbor 172.30.0.2 as 65001;
    ipv4 {
        import filter as112_v4_in;
        export none;
    };
}

protocol bgp ze_as112_v6 {
    local 2001:db8:112::1 as 65000;
    neighbor 2001:db8:112::2 as 65001;
    ipv6 {
        import filter as112_v6_in;
        export none;
    };
}
```

Useful checks:

```text
birdc show protocols
birdc show route for 192.175.48.0/24 all
birdc show route for 2620:4f:8000::/48 all
```

## VyOS core router

This VyOS config receives the AS112 covering prefixes from Ze and applies exact prefix filters on import.

```text
set protocols bgp system-as '65000'
set protocols bgp parameters router-id '172.30.0.1'

set policy prefix-list AS112-V4 rule 10 action 'permit'
set policy prefix-list AS112-V4 rule 10 prefix '192.175.48.0/24'
set policy prefix-list AS112-V4 rule 20 action 'permit'
set policy prefix-list AS112-V4 rule 20 prefix '192.31.196.0/24'

set policy prefix-list6 AS112-V6 rule 10 action 'permit'
set policy prefix-list6 AS112-V6 rule 10 prefix '2620:4f:8000::/48'
set policy prefix-list6 AS112-V6 rule 20 action 'permit'
set policy prefix-list6 AS112-V6 rule 20 prefix '2001:4:112::/48'

set policy route-map AS112-V4-IN rule 10 action 'permit'
set policy route-map AS112-V4-IN rule 10 match ip address prefix-list 'AS112-V4'
set policy route-map AS112-V4-IN rule 100 action 'deny'

set policy route-map AS112-V6-IN rule 10 action 'permit'
set policy route-map AS112-V6-IN rule 10 match ipv6 address prefix-list 'AS112-V6'
set policy route-map AS112-V6-IN rule 100 action 'deny'

set protocols bgp neighbor 172.30.0.2 remote-as '65001'
set protocols bgp neighbor 172.30.0.2 address-family ipv4-unicast route-map import 'AS112-V4-IN'

set protocols bgp neighbor 2001:db8:112::2 remote-as '65001'
set protocols bgp neighbor 2001:db8:112::2 address-family ipv6-unicast route-map import 'AS112-V6-IN'
```

Useful checks:

```text
run show bgp ipv4 unicast 192.175.48.0/24
run show bgp ipv6 unicast 2620:4f:8000::/48
```

## Junos core router

This Junos config uses exact route filters and imports only the four AS112 covering prefixes.

```text
policy-options {
    policy-statement AS112-V4-IN {
        term exact-as112 {
            from {
                route-filter 192.175.48.0/24 exact;
                route-filter 192.31.196.0/24 exact;
            }
            then accept;
        }
        then reject;
    }
    policy-statement AS112-V6-IN {
        term exact-as112 {
            from {
                route-filter 2620:4f:8000::/48 exact;
                route-filter 2001:4:112::/48 exact;
            }
            then accept;
        }
        then reject;
    }
    policy-statement REJECT-ALL {
        then reject;
    }
}
protocols {
    bgp {
        group ZE-AS112-V4 {
            type external;
            local-address 172.30.0.1;
            peer-as 65001;
            family inet {
                unicast;
            }
            import AS112-V4-IN;
            export REJECT-ALL;
            neighbor 172.30.0.2;
        }
        group ZE-AS112-V6 {
            type external;
            local-address 2001:db8:112::1;
            peer-as 65001;
            family inet6 {
                unicast;
            }
            import AS112-V6-IN;
            export REJECT-ALL;
            neighbor 2001:db8:112::2;
        }
    }
}
```

Useful checks:

```text
show route receive-protocol bgp 172.30.0.2 192.175.48.0/24 extensive
show route receive-protocol bgp 2001:db8:112::2 2620:4f:8000::/48 extensive
```

## Cisco IOS XR core router

This IOS XR config receives the four AS112 covering prefixes from Ze and sends no routes back.

```text
prefix-set AS112-V4
  192.175.48.0/24,
  192.31.196.0/24
end-set

prefix-set AS112-V6
  2620:4f:8000::/48,
  2001:4:112::/48
end-set

route-policy AS112-V4-IN
  if destination in AS112-V4 then
    pass
  else
    drop
  endif
end-policy

route-policy AS112-V6-IN
  if destination in AS112-V6 then
    pass
  else
    drop
  endif
end-policy

route-policy DROP-ALL
  drop
end-policy

router bgp 65000
 bgp router-id 172.30.0.1
 neighbor 172.30.0.2
  remote-as 65001
  address-family ipv4 unicast
   route-policy AS112-V4-IN in
   route-policy DROP-ALL out
  !
 !
 neighbor 2001:db8:112::2
  remote-as 65001
  address-family ipv6 unicast
   route-policy AS112-V6-IN in
   route-policy DROP-ALL out
  !
 !
!
```

Useful checks:

```text
show bgp ipv4 unicast 192.175.48.0/24 detail
show bgp ipv6 unicast 2620:4f:8000::/48 detail
```

## Verifying the Ze node

On Ze:

```text
show as112
request as112 healthcheck target 192.175.48.1
request as112 healthcheck target 192.31.196.1
request as112 healthcheck target 2620:4f:8000::1
request as112 healthcheck target 2001:4:112::1
```

On the core router, verify exactly the covering prefixes. The expected eBGP AS path is `65001 112`: the Ze BGP session AS first, then the AS112 origin AS supplied by the `as112` service.

Also check that these routes are absent:

```text
192.175.48.1/32
192.31.196.1/32
2620:4f:8000::1/128
2001:4:112::1/128
```

Those are the service host addresses, not the routable AS112 covering prefixes.

## Lab evidence

The existing interop suite already contains the AS112 proof this page is based on:

| Scenario | What it proves |
| --- | --- |
| test/interop/scenarios/as112-redistribute-lab | Real AS112 DNS serving plus BGP origination to FRR while the watchdog is healthy |
| test/interop/scenarios/as112-redistribute-origin-frr | The AS112 redistribute producer carries origin AS 112; FRR sees the iBGP shape and BIRD sees the eBGP prepend shape |
| test/interop/scenarios/as112-redistribute-community-frr | The AS112 redistribute producer carries the configured well-known community onto the wire |
| test/plugin/redistribute-as112-announce.ci | Ze announces the two `/24` and two `/48` covering prefixes, not the `/32` and `/128` host addresses |

Run the lab from the main Ze checkout:

```text
python3 test/interop/run.py as112-redistribute-lab
python3 test/interop/run.py as112-redistribute-origin-frr
python3 test/interop/run.py as112-redistribute-community-frr
```

The AS112 feature reference remains at [docs/guide/as112](../../guides/as112/) for every configuration leaf, CLI command, and RFC compliance note.