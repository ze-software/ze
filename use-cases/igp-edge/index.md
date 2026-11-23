# Campus and IGP edge (OSPF, IS-IS, BFD, and VRRP)

Deploy Ze as a campus or distribution router without BGP: pair OSPFv2 (with single-hop BFD fast failure detection) or dual-stack IS-IS with VRRPv3 first-hop redundancy and uplink interface tracking.

## Role and topology

```text
          Upstream core (FRR: OSPF+BFD or dual-stack IS-IS)
                         │ eth1 (point-to-point)
                         ▼
   Ze Router A (VRRP prio 200) ─── eth0 (VIP 172.30.0.100/24) ─── keepalived / Ze Router B (prio 100)
                                        │
                                  Access VLAN hosts
```

## Ze configuration: OSPFv2 with BFD

Both the top-level `bfd { enabled true; }` container and the per-interface `bfd` block under `ospf` are required so the BFD plugin starts and registers sessions (based on `test/interop/scenarios/ospf-bfd-frr/ze.conf`):

```text
bfd {
    enabled true;
}

ospf {
    router-id 172.30.0.2;
    areas {
        area 0.0.0.0 {
            area-type normal;
        }
    }
    interfaces {
        interface eth1 {
            area 0.0.0.0;
            network-type point-to-point;
            hello-interval 10;
            dead-interval 40;
            cost 10;
            bfd {
                enabled true;
                min-tx 300;
                min-rx 300;
                multiplier 3;
            }
        }
        interface lo {
            area 0.0.0.0;
            passive true;
        }
    }
}
```

Note the unit difference: inline `ospf ... bfd { min-tx; min-rx; }` timers are in **milliseconds**, while named profiles under `bfd { profile <name> { desired-min-tx-us; required-min-rx-us; } }` are in **microseconds**.

## Ze configuration: dual-stack IS-IS alternative

For IS-IS networks carrying IPv4 and IPv6 together on point-to-point links (based on `test/interop/scenarios/isis-dualstack-frr/ze.conf`):

```text
isis {
    net 49.0001.0000.0000.0000.0002.00;
    level l1-l2;
    hostname ze-ds;
    interfaces {
        interface eth1 {
            metric 10;
            level l1-l2;
            circuit-type point-to-point;
            address-family ipv4-unicast {}
            address-family ipv6-unicast {}
        }
        interface lo {
            passive true;
            address-family ipv4-unicast {}
            address-family ipv6-unicast {}
        }
    }
}
```

## Ze configuration: VRRPv3 with uplink tracking

VRRPv3 runs on the access-facing interface and decrements priority by `150` if uplink `eth1` goes down, triggering deterministic failover to the standby router (based on `test/interop/scenarios/vrrp-mastership-keepalived/ze.conf` and `docs/guide/vrrp.md`):

```text
interface {
    backend netlink;
    ethernet eth0 {
        unit 0 {
            ipv4 {
                address [ 172.30.0.2/24 ];
                vrrp {
                    group vip {
                        vrid 10;
                        virtual-address [ 172.30.0.100 ];
                        priority 200;
                        preempt true;
                        checksum rfc5798-compatible;
                        advertise-interval-milliseconds 1000;
                        track {
                            interface eth1 {
                                priority-decrement 150;
                            }
                        }
                    }
                }
            }
        }
    }
}
```

## Adjacent peer configurations

**FRR OSPFv2 + BFD (`test/interop/scenarios/ospf-bfd-frr/frr.conf`):**

```text
interface eth1
 ip ospf network point-to-point
 ip ospf area 0.0.0.0
 ip ospf bfd
!
interface lo
 ip ospf area 0.0.0.0
!
router ospf
 ospf router-id 172.30.0.3
 passive-interface lo
!
bfd
 profile ospf-fast
  transmit-interval 300
  receive-interval 300
  detect-multiplier 3
 !
!
```

**FRR dual-stack IS-IS (`test/interop/scenarios/isis-dualstack-frr/frr.conf`):**

```text
interface eth1
 ip router isis ZE
 ipv6 router isis ZE
 isis circuit-type level-1-2
 isis network point-to-point
!
interface lo
 ip router isis ZE
 ipv6 router isis ZE
 isis passive
!
router isis ZE
 net 49.0001.0000.0000.0000.0003.00
 is-type level-1-2
 metric-style wide
!
```

**keepalived 2.4.3 VRRPv3 standby (`test/interop/scenarios/vrrp-mastership-keepalived/keepalived.conf`):**

```text
global_defs {
    vrrp_garp_master_refresh 0
}

vrrp_instance VI_1 {
    state BACKUP
    interface eth0
    virtual_router_id 10
    priority 100
    advert_int 1
    version 3
    virtual_ipaddress {
        172.30.0.100/24
    }
}
```

## Verification commands

```text
show ospf neighbor detail
show ospf route
show isis neighbor
show isis route
show bfd sessions
show vrrp
show vrrp statistics
```

## Lab evidence

- **OSPF + BFD against FRR:** `INTEROP_SCENARIO=ospf-bfd-frr ./le test integration interop` (also `ospfv3-bfd-frr`, `ospf-multiarea-frr`, `ospf-lfa-frr`, `ospf-gr-frr`)
- **Dual-stack IS-IS against FRR:** `INTEROP_SCENARIO=isis-dualstack-frr ./le test integration interop` (also `isis-p2p-frr`, `isis-lan-dis-frr`, `isis-auth-frr`)
- **BFD against FRR and BIRD:** `INTEROP_SCENARIO=bfd-frr ./le test integration interop` and `bfd-simple-password-bird`
- **VRRPv3 against keepalived:** `INTEROP_SCENARIO=vrrp-mastership-keepalived ./le test integration interop` and QEMU lab `./le test qemu vrrp-keepalived`

## Current limits

- **BFD is not wired to IS-IS yet:** Ze wires BFD session notifications to BGP, OSPF, and static routes; IS-IS does not yet consume BFD events.
- **Experimental status:** OSPF and VRRP are currently marked Experimental.
- **VRRP interoperability details:** VRRP requires Linux `backend netlink` (not VPP), `CAP_NET_RAW`, `CAP_NET_ADMIN`, and `net.ipv4.ip_forward=1`. Against keepalived on IPv4 VRRPv3, set `checksum rfc5798-compatible` in Ze and `version 3` in keepalived. `accept-mode` defaults to `false` (RFC 9568), so non-owner Active routers drop pings to the VIP unless `accept-mode true` is configured. `track` monitors interface operational state only (route and script tracking are not implemented).

## Related guides

- [OSPF guide](../../guides/ospf/)
- [IS-IS guide](../../guides/isis/)
- [BFD guide](../../guides/bfd/)
- [VRRP guide](../../guides/vrrp/)
